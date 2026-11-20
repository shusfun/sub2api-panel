package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureOrigin = "https://panel.example.test"
const fixtureKey = "fixture-issuer-only-key"
const fixtureLeak = "fixture-not-for-client"
const fixturePassword = "fixture-password"
const fixtureTOTPToken = "fixture-totp-token"

func officialSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, 200, map[string]any{"code": 0, "data": data, "message": fixtureLeak})
}

func officialPage(w http.ResponseWriter, item any) {
	officialSuccess(w, map[string]any{"items": []any{item}, "pages": 1, "unused": fixtureLeak})
}

type fixture struct {
	app         http.Handler
	issuer      *issuer
	issues      atomic.Int64
	issueStatus atomic.Int64
	invitation  atomic.Bool
	scheduled   atomic.Bool
	usageRows   []fixtureUsageRow
}

func setupFixture(t testing.TB, static http.Handler, origin ...string) *fixture {
	t.Helper()
	f := new(fixture)
	f.usageRows = fixtureUsageRows()
	f.invitation.Store(true)
	f.scheduled.Store(true)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1")
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		role := "user"
		user := int64(2)
		switch token {
		case "admin-token":
			role, user = "admin", 1
		case "other-token":
			user = 3
		}
		if r.Header.Get("X-API-Key") != "" && (path != "/admin/redeem-codes/generate" || r.Header.Get("X-API-Key") != fixtureKey || token != "") {
			t.Errorf("machine key used for unexpected request %s", path)
			w.WriteHeader(403)
			return
		}
		trace := r.Header.Get("X-Request-ID")
		span := spanHex.FindStringSubmatch(r.Header.Get("traceparent"))
		if !traceHex.MatchString(trace) || len(span) != 3 || span[1] != trace {
			t.Errorf("uncorrelated upstream %s", path)
		}
		switch path {
		case "/settings/public":
			officialSuccess(w, map[string]any{"invitation_code_enabled": f.invitation.Load(), "affiliate_enabled": true, "ignored": fixtureLeak})
		case "/auth/login":
			var input officialLogin
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input.Password != fixturePassword {
				w.WriteHeader(401)
				return
			}
			name, _, _ := strings.Cut(input.Email, "@")
			if name == "admin" {
				officialSuccess(w, map[string]any{"requires_2fa": true, "temp_token": fixtureTOTPToken})
				return
			}
			officialSuccess(w, credentials{name + "-token", name + "-refresh"})
		case "/auth/login/2fa":
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input["temp_token"] != fixtureTOTPToken || input["totp_code"] != "123456" {
				w.WriteHeader(401)
				return
			}
			officialSuccess(w, credentials{"admin-token", "admin-refresh"})
		case "/auth/logout":
			writeJSON(w, 200, map[string]any{"code": 0})
		case "/auth/me":
			if token != "user-token" && token != "other-token" && token != "admin-token" {
				w.WriteHeader(401)
				return
			}
			officialSuccess(w, map[string]any{"id": user, "role": role, "sensitive": fixtureLeak})
		case "/user/profile":
			officialSuccess(w, map[string]any{"balance": 12.50, "sensitive": fixtureLeak})
		case "/user/aff":
			officialSuccess(w, map[string]any{"aff_code": "AFF2", "sensitive": fixtureLeak})
		case "/subscriptions":
			officialSuccess(w, []any{map[string]any{"id": 11, "group": map[string]any{"id": 2, "name": "Demo", "extra": fixtureLeak}, "status": "active", "expires_at": "2030-01-01T00:00:00Z"}})
		case "/usage", "/usage/dashboard/snapshot-v2", "/admin/usage":
			f.serveUsage(w, r, path, user)
		case "/groups/available":
			officialSuccess(w, []any{map[string]any{"id": 8, "name": "Example", "extra": fixtureLeak}})
		case "/keys":
			key := map[string]any{"id": 4, "name": "Demo", "key": fixtureLeak + "abcd", "status": "active", "user_id": user, "unneeded": fixtureLeak}
			if r.Method == "POST" {
				officialSuccess(w, key)
			} else {
				officialSuccess(w, map[string]any{"items": fixtureKeys(user), "pages": 1, "unused": fixtureLeak})
			}
		case "/keys/4", "/keys/9":
			if path == "/keys/9" || user != 2 {
				writeJSON(w, 403, map[string]any{"msg": fixtureLeak})
				return
			}
			if r.Method == "DELETE" {
				writeJSON(w, 200, map[string]int{"code": 0})
				return
			}
			officialSuccess(w, map[string]any{"id": 4, "user_id": user, "key": fixtureLeak})
		case "/redeem":
			officialSuccess(w, map[string]any{"unused": fixtureLeak})
		case "/admin/users", "/admin/users/2", "/admin/users/3":
			if role != "admin" {
				w.WriteHeader(403)
				return
			}
			id := 2
			if path == "/admin/users/3" {
				id = 3
			}
			row := map[string]any{"id": id, "role": "user", "email": "user@example.test", "balance": 12.5, "status": "active", "internal_notes": fixtureLeak}
			if path == "/admin/users" {
				officialPage(w, row)
			} else {
				officialSuccess(w, row)
			}
		case "/user/totp/step-up":
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			officialSuccess(w, map[string]any{"verified": role == "admin" && input["code"] == "123456"})
		case "/admin/redeem-codes/generate":
			var input struct {
				Count int
				Type  string
				Value float64
				Days  int `json:"expires_in_days"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			if r.Header.Get("X-API-Key") != fixtureKey || !operationPattern.MatchString(r.Header.Get("Idempotency-Key")) || input.Count != 1 ||
				(input.Type == "invitation" && input.Days != 7) || (input.Type == "balance" && input.Days != 30) {
				t.Error("invalid request to official issuer")
			}
			id := f.issues.Add(1)
			if status := f.issueStatus.Load(); status != 0 {
				writeJSON(w, int(status), map[string]any{"msg": fixtureLeak})
				return
			}
			officialSuccess(w, []any{map[string]any{"id": id, "code": fmt.Sprintf("fixture-code-%d", id), "type": input.Type, "value": input.Value, "notes": fixtureLeak}})
		case "/admin/accounts", "/admin/accounts/5":
			if role != "admin" {
				w.WriteHeader(403)
				return
			}
			row := map[string]any{"id": 5, "name": "Demo upstream", "platform": "openai", "status": "active", "schedulable": f.scheduled.Load(),
				"credentials": map[string]any{"access_token": fixtureLeak}, "extra": map[string]any{"codex_5h_used_percent": 25, "private": fixtureLeak}}
			if path == "/admin/accounts" {
				officialPage(w, row)
			} else {
				officialSuccess(w, row)
			}
		case "/admin/accounts/5/usage":
			officialSuccess(w, map[string]any{"five_hour": map[string]any{"utilization": 25, "unused": fixtureLeak}, "credentials": fixtureLeak})
		case "/admin/accounts/5/schedulable":
			var input struct {
				On bool `json:"schedulable"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			f.scheduled.Store(input.On)
			officialSuccess(w, map[string]any{"credentials": fixtureLeak})
		case "/admin/accounts/5/test":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"type\":\"content\",\"text\":%q}\n\ndata: {\"type\":\"test_complete\",\"success\":true,\"secret\":%q}\n\n", fixtureLeak, fixtureLeak)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(up.Close)
	db, err := openIssuerDB(filepath.Join(t.TempDir(), "issuer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f.issuer = &issuer{db: db, upstream: newClient(up.URL), secret: randomToken(), adminKey: fixtureKey}
	internal := httptest.NewServer(f.issuer.routes())
	t.Cleanup(internal.Close)
	targetOrigin := fixtureOrigin
	if len(origin) != 0 {
		targetOrigin = origin[0]
	}
	app := &panel{origin: targetOrigin, official: "https://official.example.test", upstream: newClient(up.URL), issuer: newClient(internal.URL), secret: f.issuer.secret, sessions: newSessions()}
	f.app = app.routes(static)
	return f
}

type fixtureBrowser struct {
	t      *testing.T
	app    http.Handler
	key    *ecdsa.PrivateKey
	cookie *http.Cookie
	csrf   string
}

func browserLogin(t *testing.T, app http.Handler, name string) *fixtureBrowser {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b := &fixtureBrowser{t: t, app: app, key: key}
	challenge := b.expect("POST", "/session/challenge", "", 200)
	public := base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), key.X, key.Y))
	result := b.request("POST", "/session/login", fmt.Sprintf(`{"challenge":%q,"browser_key":%q,"email":%q,"password":%q}`, challenge["challenge"], public, name+"@example.test", fixturePassword))
	checkResponse(t, result, 200)
	if name == "admin" {
		pending := object(t, result.Body.Bytes())["pending"]
		result = b.request("POST", "/session/verify", fmt.Sprintf(`{"challenge":%q,"browser_key":%q,"otp":"123456"}`, pending, public))
		checkResponse(t, result, 200)
	}
	cookies := result.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	b.cookie = cookies[0]
	if b.cookie.Name != cookieName || b.cookie.Domain != "" || b.cookie.Path != "/" || !b.cookie.HttpOnly || !b.cookie.Secure ||
		b.cookie.SameSite != http.SameSiteStrictMode || b.cookie.MaxAge != 43200 {
		t.Fatal("unexpected cookie policy")
	}
	self := b.expect("GET", "/session", "", 200)
	b.csrf, _ = self["csrf"].(string)
	if len(self) != 2 || b.csrf == "" || self["admin"] != (name == "admin") {
		t.Fatalf("unexpected session fields: %v", self)
	}
	return b
}

func (b *fixtureBrowser) signRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, fixtureOrigin+"/api"+path, strings.NewReader(body))
	if method != "GET" {
		r.Header.Set("Origin", fixtureOrigin)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if b.cookie != nil {
		r.AddCookie(b.cookie)
	}
	r.Header.Set("X-CSRF-Token", b.csrf)
	hash := sha256.Sum256([]byte(body))
	digest := "sha-256=:" + base64.StdEncoding.EncodeToString(hash[:]) + ":"
	params := fmt.Sprintf(`("@method" "@target-uri" "content-digest");created=%d;nonce="%s"`, time.Now().Unix(), randomHex(16))
	sigBase := fmt.Sprintf("\"@method\": %s\n\"@target-uri\": %s\n\"content-digest\": %s\n\"@signature-params\": %s", method, r.URL.String(), digest, params)
	message := sha256.Sum256([]byte(sigBase))
	rInt, sInt, err := ecdsa.Sign(rand.Reader, b.key, message[:])
	if err != nil {
		panic(err)
	}
	sig := make([]byte, 64)
	rInt.FillBytes(sig[:32])
	sInt.FillBytes(sig[32:])
	r.Header.Set("Content-Digest", digest)
	r.Header.Set("Signature-Input", "panel="+params)
	r.Header.Set("Signature", "panel=:"+base64.StdEncoding.EncodeToString(sig)+":")
	return r
}

func (b *fixtureBrowser) send(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	b.app.ServeHTTP(w, r)
	return w
}

func (b *fixtureBrowser) request(method, path, body string) *httptest.ResponseRecorder {
	return b.send(b.signRequest(method, path, body))
}

func (b *fixtureBrowser) expect(method, path, body string, status int) map[string]any {
	b.t.Helper()
	res := b.request(method, path, body)
	checkResponse(b.t, res, status)
	return object(b.t, res.Body.Bytes())
}

func checkResponse(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status || !traceHex.MatchString(w.Header().Get("X-Request-ID")) {
		t.Fatalf("status %d, want %d, body %s", w.Code, status, w.Body.String())
	}
	if strings.Contains(w.Body.String(), fixtureLeak) && !strings.Contains(w.Body.String(), `{"key":"`+fixtureLeak+`abcd"}`) {
		t.Fatal("unexpected upstream field reached the client")
	}
	if w.Code >= 400 {
		problem := object(t, w.Body.Bytes())
		if len(problem) != 3 || problem["hint"] == nil || problem["code"] == nil || problem["request_id"] != w.Header().Get("X-Request-ID") {
			t.Fatal("invalid error projection")
		}
	}
}

func object(t *testing.T, value []byte) map[string]any {
	t.Helper()
	var data map[string]any
	if json.Unmarshal(value, &data) != nil || data == nil {
		t.Fatalf("invalid JSON: %s", value)
	}
	return data
}

func operation(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func issueBody(n int, kind string, cents int) string {
	return fmt.Sprintf(`{"operation_id":%q,"kind":%q,"amount_cents":%d}`, operation(n), kind, cents)
}

func TestPanelSmoke(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	u := browserLogin(t, f.app, "user")
	a := browserLogin(t, f.app, "admin")
	if data := u.expect("GET", "/balance", "", 200); len(data) != 1 || data["balance"] != 12.5 {
		t.Fatal(data)
	}
	if u.expect("GET", "/referral", "", 200)["url"] != "https://official.example.test/register?aff=AFF2" {
		t.Fatal("referral link")
	}
	res := u.request("GET", "/subscriptions", "")
	checkResponse(t, res, 200)
	u.expect("GET", "/usage", "", 200)
	u.expect("POST", "/keys", `{"name":"Example"}`, 201)
	u.expect("GET", "/keys", "", 200)
	u.expect("PUT", "/keys/4", `{"name":"Renamed"}`, 200)
	u.expect("DELETE", "/keys/4", "", 200)
	a.expect("GET", "/admin/users", "", 200)
	a.expect("PUT", "/admin/grants/2", `{"invites":1,"budget_cents":500,"amounts_cents":[500],"totp":"123456"}`, 200)
	if result := u.expect("POST", "/issues", issueBody(1, "invitation", 0), 201); len(result) != 1 || result["code"] == nil {
		t.Fatal(result)
	}
	if result := u.expect("POST", "/issues", issueBody(2, "balance", 500), 201); len(result) != 1 || result["code"] == nil {
		t.Fatal(result)
	}
	u.expect("POST", "/redeem", `{"code":"fixture-code-2"}`, 200)
	userHistory := u.expect("GET", "/issues", "", 200)["items"].([]any)[0].(map[string]any)
	adminHistory := a.expect("GET", "/issues", "", 200)["items"].([]any)[0].(map[string]any)
	if userHistory["actor_id"] != nil || userHistory["code"] != nil || adminHistory["actor_id"] != float64(2) {
		t.Fatal("wrong issue history projection")
	}
	accounts := a.expect("GET", "/admin/accounts?platform=openai", "", 200)["items"].([]any)[0].(map[string]any)
	if len(accounts) != 7 || accounts["extra"] != nil || accounts["credentials"] != nil {
		t.Fatal(accounts)
	}
	quota := a.expect("GET", "/admin/accounts/5/quota", "", 200)["five_hours"].(map[string]any)
	if len(quota) != 2 || quota["percent_used"] != float64(25) {
		t.Fatal(quota)
	}
	res = a.request("POST", "/admin/accounts/5/test", `{}`)
	checkResponse(t, res, 200)
	if res.Body.String() != "data: {\"state\":\"running\"}\n\ndata: {\"state\":\"passed\"}\n\n" {
		t.Fatal(res.Body.String())
	}
	a.expect("PUT", "/admin/accounts/5/scheduling", `{"schedulable":false}`, 200)
	if f.scheduled.Load() {
		t.Fatal("schedule did not change")
	}
}

func TestSessionAndRoleBoundaries(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	u := browserLogin(t, f.app, "user")
	a := browserLogin(t, f.app, "admin")
	for _, path := range []string{"/session", "/balance", "/issues"} {
		r := httptest.NewRequest("GET", fixtureOrigin+"/api"+path, nil)
		r.AddCookie(u.cookie)
		checkResponse(t, u.send(r), 401)
	}
	for _, path := range []string{"/redeem", "/session/logout"} {
		r := httptest.NewRequest("POST", fixtureOrigin+"/api"+path, nil)
		r.AddCookie(u.cookie)
		r.Header.Set("Origin", fixtureOrigin)
		checkResponse(t, u.send(r), 401)
	}
	other := browserLogin(t, f.app, "other")
	other.cookie = u.cookie
	other.expect("GET", "/session", "", 401)
	r := u.signRequest("GET", "/balance", "")
	checkResponse(t, u.send(r), 200)
	checkResponse(t, u.send(r), 401)
	r = u.signRequest("POST", "/redeem", `{"code":"expected"}`)
	r.Body = io.NopCloser(bytes.NewBufferString(`{"code":"modified"}`))
	checkResponse(t, u.send(r), 401)
	r = u.signRequest("POST", "/redeem", `{"code":"expected"}`)
	r.Header.Set("Origin", "https://other.example.test")
	checkResponse(t, u.send(r), 403)
	r = u.signRequest("POST", "/redeem", `{"code":"expected"}`)
	r.Header.Del("X-CSRF-Token")
	checkResponse(t, u.send(r), 403)
	u.expect("GET", "/admin/users", "", 403)
	u.expect("PUT", "/admin/grants/2", `{}`, 403)
	u.expect("DELETE", "/keys/9", "", 403)
	u.expect("POST", "/issues", issueBody(10, "invitation", 0), 403)
	a.expect("PUT", "/admin/grants/2", `{"invites":1,"budget_cents":500,"amounts_cents":[500],"totp":"000000"}`, 403)
	a.expect("PUT", "/admin/grants/2", `{"invites":1,"budget_cents":500,"amounts_cents":[500],"totp":"123456"}`, 200)
	u.expect("POST", "/issues", issueBody(11, "balance", 499), 403)
	f.invitation.Store(false)
	u.expect("POST", "/issues", issueBody(12, "invitation", 0), 403)
	u.expect("POST", "/issues", issueBody(13, "balance", 500), 201)
	a.expect("DELETE", "/admin/grants/2", "", 200)
	f.invitation.Store(true)
	u.expect("POST", "/issues", issueBody(14, "invitation", 0), 403)
	u.expect("POST", "/session/logout", "", 200)
	u.expect("GET", "/session", "", 401)
	u.expect("POST", "/session/logout", "", 401)
}

func TestIssueReservations(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	u := browserLogin(t, f.app, "user")
	a := browserLogin(t, f.app, "admin")
	a.expect("PUT", "/admin/grants/2", `{"invites":2,"budget_cents":0,"amounts_cents":[],"totp":"123456"}`, 200)
	f.issueStatus.Store(400)
	u.expect("POST", "/issues", issueBody(20, "invitation", 0), 502)
	if u.expect("GET", "/grant", "", 200)["invites"] != float64(2) {
		t.Fatal("rejected issue was not refunded")
	}
	f.issueStatus.Store(500)
	u.expect("POST", "/issues", issueBody(21, "invitation", 0), 503)
	u.expect("POST", "/issues", issueBody(21, "invitation", 0), 409)
	if u.expect("GET", "/grant", "", 200)["invites"] != float64(1) {
		t.Fatal("unknown issue did not hold reservation")
	}
	f.issueStatus.Store(0)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, n := range []int{22, 23} {
		wg.Go(func() { statuses <- u.request("POST", "/issues", issueBody(n, "invitation", 0)).Code })
	}
	wg.Wait()
	close(statuses)
	tally := map[int]int{}
	for status := range statuses {
		tally[status]++
	}
	if tally[201] != 1 || tally[403] != 1 || f.issues.Load() != 3 {
		t.Fatalf("unexpected issuance counts %v", tally)
	}
	a.expect("DELETE", "/admin/grants/2", "", 200)
	a.expect("PUT", "/admin/grants/2", `{"invites":10,"budget_cents":0,"amounts_cents":[],"totp":"123456"}`, 200)
	a.expect("POST", "/admin/issues/"+operation(21)+"/resolve", `{"code_id":null}`, 200)
	if u.expect("GET", "/grant", "", 200)["invites"] != float64(10) {
		t.Fatal("old reservation changed a new grant")
	}
}

// Serve only the disposable browser fixture, on loopback, when explicitly requested.
func TestBrowserFixture(t *testing.T) {
	address := os.Getenv("PANEL_TEST_BROWSER_ADDR")
	if address == "" {
		t.Skip("configure the loopback browser address to run")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("the browser fixture must bind to IPv4 loopback")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	info, err := os.Stat("web/dist/index.html")
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("build the web bundle first")
	}
	f := setupFixture(t, http.FileServerFS(os.DirFS("web/dist")), "http://"+listener.Addr().String())
	server := &http.Server{Handler: f.app, ReadHeaderTimeout: 5 * time.Second}
	t.Logf("browser fixture ready at http://%s", listener.Addr())
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
