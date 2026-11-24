package testkit

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sub2api-panel/internal/platform"
)

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
	if b.cookie.Name != platform.CookieName || b.cookie.Domain != "" || b.cookie.Path != "/" || !b.cookie.HttpOnly || !b.cookie.Secure ||
		b.cookie.SameSite != http.SameSiteStrictMode || b.cookie.MaxAge != 43200 {
		t.Fatal("unexpected cookie policy")
	}
	self := b.expect("GET", "/session", "", 200)
	b.csrf, _ = self["csrf"].(string)
	if len(self) != 4 || b.csrf == "" || self["admin"] != (name == "admin") {
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
	params := fmt.Sprintf(`("@method" "@target-uri" "content-digest");created=%d;nonce="%s"`, time.Now().Unix(), platform.RandomHex(16))
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
	if w.Code != status || !platform.TraceHex.MatchString(w.Header().Get("X-Request-ID")) {
		t.Fatalf("status %d, want %d, body %s", w.Code, status, w.Body.String())
	}
	if strings.Contains(w.Body.String(), fixtureLeak) && !strings.Contains(w.Body.String(), `{"key":"`+fixtureLeak+`abcd"}`) {
		t.Fatal("unexpected upstream field reached the client")
	}
	if w.Code >= 400 {
		prob := object(t, w.Body.Bytes())
		if len(prob) != 3 || prob["hint"] == nil || prob["code"] == nil || prob["request_id"] != w.Header().Get("X-Request-ID") {
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
