package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func authenticatorCode(t *testing.T, key string) string {
	t.Helper()
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.TrimRight(key, "="))
	if err != nil {
		t.Fatal("invalid TOTP setup response")
	}
	window := binary.BigEndian.AppendUint64(nil, uint64(time.Now().Unix()/30))
	mac := hmac.New(sha1.New, secret)
	mac.Write(window)
	value := mac.Sum(nil)
	offset := value[len(value)-1] & 15
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(value[offset:])&0x7fffffff)%1_000_000)
}

// Run against a disposable official Sub2API installation with an administrator
// who has not set up an authenticator yet.
func TestOfficialCompatibility(t *testing.T) {
	origin, config := os.Getenv("PANEL_TEST_OFFICIAL"), os.Getenv("PANEL_TEST_ADMIN_FILE")
	if origin == "" || config == "" {
		t.Skip("configure a disposable official instance to run")
	}
	var admin officialLogin
	data, err := os.ReadFile(config)
	if err != nil || json.Unmarshal(data, &admin) != nil || admin.Email == "" || admin.Password == "" || !checkOrigin(origin, false) {
		t.Fatal("invalid official fixture configuration")
	}
	c := newClient(origin)
	trace := callMeta{Trace: randomHex(16), Span: randomHex(8)}
	ctx := context.WithValue(context.Background(), metaKey{}, trace)
	call := func(method, path, bearer string, payload any, expected int) json.RawMessage {
		t.Helper()
		response, err := c.open(ctx, method, "/api/v1"+path, bearer, "", "", payload)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer response.Body.Close()
		var envelope struct {
			Code int
			Data json.RawMessage
		}
		_ = json.NewDecoder(response.Body).Decode(&envelope)
		if response.StatusCode != expected || (expected == 200 || expected == 201) && envelope.Code != 0 {
			t.Fatalf("%s %s: status=%d code=%d request=%s", method, path, response.StatusCode, envelope.Code, response.Header.Get("X-Request-ID"))
		}
		t.Logf("official %s %s status=%d request=%s", method, path, response.StatusCode, response.Header.Get("X-Request-ID"))
		return envelope.Data
	}
	var initial credentials
	if json.Unmarshal(call("POST", "/auth/login", "", admin, 200), &initial) != nil || initial.Access == "" {
		t.Fatal("fixture needs a fresh official administrator")
	}
	var compliance struct {
		Required bool
		Phrase   string `json:"ack_phrase_en"`
	}
	if json.Unmarshal(call("GET", "/admin/compliance", initial.Access, nil, 200), &compliance) != nil || compliance.Phrase == "" {
		t.Fatal("invalid official compliance fixture status")
	}
	if compliance.Required {
		call("POST", "/admin/compliance/accept", initial.Access, map[string]string{"phrase": compliance.Phrase, "language": "en"}, 200)
	}
	call("PUT", "/admin/settings", initial.Access, map[string]bool{"registration_enabled": true, "totp_enabled": true, "affiliate_enabled": true, "invitation_code_enabled": false}, 200)
	var setup struct {
		Secret string
		Token  string `json:"setup_token"`
	}
	if json.Unmarshal(call("POST", "/user/totp/setup", initial.Access, map[string]string{"password": admin.Password}, 200), &setup) != nil || setup.Secret == "" || setup.Token == "" {
		t.Fatal("invalid TOTP setup")
	}
	call("POST", "/user/totp/enable", initial.Access, map[string]string{"setup_token": setup.Token, "totp_code": authenticatorCode(t, setup.Secret)}, 200)
	var issuerSecret struct{ Key string }
	if json.Unmarshal(call("POST", "/admin/settings/admin-api-keys", initial.Access, map[string]any{
		"name": "panel isolated integration", "enabled": true, "policy": map[string]bool{"full_access": true}, "confirm_full_access": true,
	}, 200), &issuerSecret) != nil || issuerSecret.Key == "" {
		t.Fatal("could not create the official issuer key")
	}
	password := randomToken()
	var registered credentials
	userEmail := "delegate-" + randomHex(5) + "@panel.test"
	if json.Unmarshal(call("POST", "/auth/register", "", officialLogin{Email: userEmail, Password: password}, 200), &registered) != nil {
		t.Fatal("could not register delegate")
	}
	user, err := currentUser(ctx, c, registered.Access)
	if err != nil {
		t.Fatal(err)
	}
	call("PUT", "/admin/settings", initial.Access, map[string]bool{"invitation_code_enabled": true}, 200)
	db, err := openIssuerDB(filepath.Join(t.TempDir(), "issuer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	i := &issuer{db: db, upstream: c, adminKey: issuerSecret.Key, secret: randomToken()}
	internal := httptest.NewServer(i.routes())
	defer internal.Close()
	app := &panel{origin: fixtureOrigin, official: origin, upstream: c, issuer: newClient(internal.URL), secret: i.secret, sessions: newSessions()}
	web := app.routes(http.NotFoundHandler())
	login := func(email, password, totp string) *fixtureBrowser {
		t.Helper()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		b := &fixtureBrowser{t: t, app: web, key: key}
		challenge := b.expect("POST", "/session/challenge", "", 200)["challenge"]
		public := base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), key.X, key.Y))
		response := b.request("POST", "/session/login", fmt.Sprintf(`{"challenge":%q,"browser_key":%q,"email":%q,"password":%q}`, challenge, public, email, password))
		checkResponse(t, response, 200)
		if totp != "" {
			pending := object(t, response.Body.Bytes())["pending"]
			response = b.request("POST", "/session/verify", fmt.Sprintf(`{"challenge":%q,"browser_key":%q,"otp":%q}`, pending, public, authenticatorCode(t, totp)))
			checkResponse(t, response, 200)
		}
		if len(response.Result().Cookies()) != 1 {
			t.Fatal("missing panel session")
		}
		b.cookie = response.Result().Cookies()[0]
		b.csrf, _ = b.expect("GET", "/session", "", 200)["csrf"].(string)
		return b
	}
	a := login(admin.Email, admin.Password, setup.Secret)
	u := login(userEmail, password, "")
	grantPath := fmt.Sprintf("/admin/grants/%d", user.ID)
	a.expect("PUT", grantPath, fmt.Sprintf(`{"invites":1,"budget_cents":500,"amounts_cents":[500],"totp":%q}`, authenticatorCode(t, setup.Secret)), 200)
	newIssue := func(kind string, amount int) string {
		return fmt.Sprintf(`{"operation_id":"%s-%s-4%s-8%s-%s","kind":%q,"amount_cents":%d}`,
			randomHex(4), randomHex(2), randomHex(2)[1:], randomHex(2)[1:], randomHex(6), kind, amount)
	}
	invite := u.expect("POST", "/issues", newIssue("invitation", 0), 201)["code"]
	balanceCode := u.expect("POST", "/issues", newIssue("balance", 500), 201)["code"]
	var recipient credentials
	recipientEmail := "recipient-" + randomHex(5) + "@panel.test"
	if json.Unmarshal(call("POST", "/auth/register", "", map[string]any{"email": recipientEmail, "password": password, "invitation_code": invite}, 200), &recipient) != nil {
		t.Fatal("invitation register response")
	}
	call("POST", "/auth/register", "", map[string]any{"email": "second-" + randomHex(5) + "@panel.test", "password": password, "invitation_code": invite}, 400)
	call("POST", "/redeem", recipient.Access, map[string]any{"code": balanceCode}, 200)
	var profile struct{ Balance float64 }
	if json.Unmarshal(call("GET", "/user/profile", recipient.Access, nil, 200), &profile) != nil || profile.Balance < 5 {
		t.Fatal("official redemption balance")
	}
	u.expect("GET", "/balance", "", 200)
	u.expect("GET", "/usage", "", 200)
	u.expect("GET", "/keys", "", 200)
	var account namedItem
	if json.Unmarshal(call("POST", "/admin/accounts", initial.Access, map[string]any{
		"name": "panel fixture " + randomHex(5), "platform": "openai", "type": "apikey",
		"credentials": map[string]string{"api_key": "panel-fixture-placeholder"},
		"concurrency": 1, "priority": 50,
	}, 200), &account) != nil || account.ID < 1 {
		t.Fatal("could not create disposable upstream account")
	}
	other := login(recipientEmail, password, "")
	testOfficialUsage(t, a, u, other, account.ID)
	a.expect("GET", "/admin/accounts", "", 200)
	schedulePath := fmt.Sprintf("/admin/accounts/%d/scheduling", account.ID)
	a.expect("PUT", schedulePath, `{"schedulable":false}`, 200)
	a.expect("PUT", schedulePath, `{"schedulable":true}`, 200)
	a.expect("GET", "/admin/users", "", 200)
	a.expect("GET", "/issues", "", 200)
	a.expect("DELETE", grantPath, "", 200)
	u.expect("POST", "/issues", newIssue("balance", 500), 403)
	t.Logf("official compatibility finished trace=%s user=%d", trace.Trace, user.ID)
}

func testOfficialUsage(t *testing.T, admin, user, other *fixtureBrowser, account int64) {
	t.Helper()
	addKey := func(browser *fixtureBrowser, name string) string {
		t.Helper()
		browser.expect("POST", "/keys", fmt.Sprintf(`{"name":%q}`, name), 201)
		result := browser.expect("GET", "/usage/keys", "", 200)["items"].([]any)
		for _, item := range result {
			key := item.(map[string]any)
			if key["name"] == name && len(key) == 2 {
				return fmt.Sprintf("%.0f", key["id"].(float64))
			}
		}
		t.Fatal("created Key missing from the usage picker")
		return ""
	}
	first, second, foreign := addKey(user, "Usage A"), addKey(user, "Usage B"), addKey(other, "Usage other")
	query := "?api_key_id=" + first
	if summary := user.expect("GET", "/usage/summary"+query, "", 200); summary["totals"].(map[string]any)["requests"] != float64(0) {
		t.Fatal(summary)
	}
	for _, route := range []string{"/usage", "/usage/summary"} {
		user.expect("GET", route+"?api_key_id="+foreign, "", 403)
		other.expect("GET", route+query, "", 403)
		admin.expect("GET", route+query, "", 403)
	}
	container := os.Getenv("PANEL_TEST_POSTGRES_CONTAINER")
	if container == "" {
		t.Log("non-empty official usage checks require the disposable PostgreSQL container")
		return
	}
	// Seed only the disposable instance so accounting and date boundaries can be checked without a live provider.
	sql := fmt.Sprintf(`
INSERT INTO usage_logs (user_id, api_key_id, account_id, request_id, model, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, actual_cost, created_at)
SELECT k.user_id, k.id, %d, 'panel-usage-' || k.id || '-' || sample.n, sample.model, sample.input,
       5, 7, 9, 0.02, date_trunc('day', now(), 'Asia/Shanghai') - sample.days * interval '1 day'
FROM (SELECT %s AS key_id, n, 'sample-a' AS model, 20 AS input, CASE WHEN n = %d THEN 45 ELSE n %% 2 END AS days FROM generate_series(0, %d) AS n
      UNION ALL SELECT %s, 0, 'sample-b', 50, 0
      UNION ALL SELECT %s, 0, 'private-sample', 300, 0) AS sample
JOIN api_keys k ON k.id = sample.key_id;`, account, first, pageSize, pageSize, second, foreign)
	seed := exec.Command("docker", "exec", "-i", container, "sh", "-c", `psql -X --set=ON_ERROR_STOP=1 --username="$POSTGRES_USER" --dbname="$POSTGRES_DB"`)
	seed.Stdin = strings.NewReader(sql)
	if output, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("could not seed official usage: %v %s", err, output)
	}
	summary := user.expect("GET", "/usage/summary"+query, "", 200)
	totals := summary["totals"].(map[string]any)
	days := summary["days"].([]any)
	models := summary["models"].(map[string]any)["items"].([]any)
	if totals["requests"] != float64(pageSize) || totals["input_tokens"] != float64(pageSize*20) ||
		totals["output_tokens"] != float64(pageSize*5) || totals["cache_read_tokens"] != float64(pageSize*7) ||
		totals["cache_creation_tokens"] != float64(pageSize*9) || math.Abs(totals["actual_cost"].(float64)-0.02*pageSize) > 1e-9 ||
		days[5].(map[string]any)["input_tokens"] != float64(pageSize*10) || days[6].(map[string]any)["input_tokens"] != float64(pageSize*10) ||
		len(models) != 1 || models[0].(map[string]any)["model"] != "sample-a" || models[0].(map[string]any)["total_tokens"] != float64(pageSize*41) {
		t.Fatal(summary)
	}
	all := user.expect("GET", "/usage/summary", "", 200)
	if all["totals"].(map[string]any)["requests"] != float64(pageSize+1) {
		t.Fatal(all)
	}
	today := user.expect("GET", "/usage"+query+"&period=today", "", 200)
	if items := today["items"].([]any); len(items) != pageSize/2 || items[0].(map[string]any)["cache_creation_tokens"] != float64(9) {
		t.Fatal(today)
	}
	history := user.expect("GET", "/usage"+query+"&page=2", "", 200)
	if items := history["items"].([]any); len(items) != 1 || len(items[0].(map[string]any)) != 9 || history["more"] != false {
		t.Fatal(history)
	}
}
