package main

import (
	"bytes"
	"context"
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
	"strconv"
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
const browserFixtureAddr = "127.0.0.1:28741"

func officialSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, 200, map[string]any{"code": 0, "data": data, "message": fixtureLeak})
}

func officialPage(w http.ResponseWriter, item any) {
	officialSuccess(w, map[string]any{"items": []any{item}, "pages": 1, "unused": fixtureLeak})
}

type fixture struct {
	app               http.Handler
	panel             *panel
	issuer            *issuer
	issues            atomic.Int64
	issueStatus       atomic.Int64
	invitation        atomic.Bool
	scheduled         atomic.Bool
	usageRows         []fixtureUsageRow
	previewKeys       map[int64][]map[string]any
	preview           *previewCatalog
	previewNotices    []map[string]any
	previewPromos     []map[string]any
	previewNoticeRead map[int64]map[int64]time.Time
	previewContentID  int64
	previewRebates    map[int64]float64
	previewPayment    map[int64]paymentOrder
	previewOrders     int64
	previewProxies    []map[string]any
	previewRules      map[int64]channelRule
	previewRuleID     int64
	previewOfficial   *officialPlugin
	previewPluginConf map[string]json.RawMessage
	previewRisk       *riskConfigView
	previewRoutes     map[string]previewUsageRoute
	previewMu         sync.Mutex
	previewUserGroups map[int64][]int64
	previewBindings   map[int64]int64
	totpState         map[int64]bool
	profileNames      map[int64]string
	previewSettings   map[string]any
	previewSubSources map[string]map[string]any
	previewAdminKey   string
	previewAccessKeys []map[string]any
	previewAuditLogs  []auditEntry
	previewMCPGrants  []mcpGrant
	previewQuotas     map[int64][]platformQuotaView
	previewAttrs      map[int64][]userAttributeValue
	previewSubscribed atomic.Bool
	totpFeature       atomic.Bool
}

func setupFixture(t testing.TB, static http.Handler, origin ...string) *fixture {
	t.Helper()
	databaseURL := os.Getenv("PANEL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("configure PANEL_TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	f := new(fixture)
	f.previewUserGroups = make(map[int64][]int64)
	f.previewRebates = map[int64]float64{2: 3.25}
	f.previewPayment = make(map[int64]paymentOrder)
	f.totpState = map[int64]bool{1: true}
	f.profileNames = map[int64]string{}
	f.previewSettings = map[string]any{
		"site_name": "Sub2API", "site_subtitle": "高性能多渠道统一 AI 网关与智能号池控制台",
		"registration_enabled": true, "promo_code_enabled": true, "password_reset_enabled": true, "passkey_enabled": true,
		"github_oauth_enabled": true, "google_oauth_enabled": true, "linuxdo_oauth_enabled": true, "affiliate_enabled": true, "available_channels_enabled": true,
		"model_plaza_enabled": true, "payment_enabled": true, "channel_monitor_enabled": true, "channel_monitor_mode": "v2",
		"plugin_management_enabled": true, "mcp_enabled": true, "risk_control_enabled": true, "balance_low_notify_enabled": true,
		"table_default_page_size": 20, "table_page_size_options": []int{10, 20, 50, 100},
	}
	f.totpFeature.Store(true)
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
		if f.previewKeys != nil {
			if id := f.previewUserID(strings.TrimSuffix(token, "-token") + "@example.test"); id > 3 && strings.HasSuffix(token, "-token") {
				user = id
			}
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
		if r.Header.Get("X-API-Key") == "" && (f.servePreview(w, r, path, role, user) ||
			f.serveAdminExtended(w, r, path, role) || f.servePreviewExtra(w, r, path, role, user) ||
			f.preview != nil && f.servePreviewPayment(w, r, path, user, token)) {
			return
		}
		if f.serveFixtureTOTP(w, r, path, user) {
			return
		}
		switch path {
		case "/settings/public":
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			public := map[string]any{"invitation_code_enabled": f.invitation.Load(), "ignored": fixtureLeak}
			for _, key := range []string{"site_name", "site_subtitle", "site_logo", "registration_enabled", "promo_code_enabled", "model_plaza_enabled", "model_plaza_require_auth", "payment_enabled", "channel_monitor_enabled", "channel_monitor_mode", "plugin_management_enabled",
				"password_reset_enabled", "passkey_enabled", "github_oauth_enabled", "google_oauth_enabled", "linuxdo_oauth_enabled", "mcp_enabled", "risk_control_enabled", "balance_low_notify_enabled",
				"affiliate_enabled", "custom_menu_items", "custom_endpoints", "hide_ccs_import_button", "purchase_subscription_enabled", "available_channels_enabled"} {
				if value, ok := f.previewSettings[key]; ok {
					public[key] = value
				}
			}
			officialSuccess(w, public)
		case "/admin/settings":
			if role != "admin" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			if r.Method == http.MethodPut {
				var values map[string]any
				if json.NewDecoder(r.Body).Decode(&values) != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				for key, value := range values {
					f.previewSettings[key] = value
				}
			}
			officialSuccess(w, f.previewSettings)
		case "/auth/login":
			var input officialLogin
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input.Password != fixturePassword {
				w.WriteHeader(401)
				return
			}
			if f.previewKeys != nil && f.previewUserID(input.Email) == 0 {
				w.WriteHeader(401)
				return
			}
			name, _, _ := strings.Cut(input.Email, "@")
			id := int64(2)
			if name == "admin" {
				id = 1
			} else if f.previewKeys != nil {
				id = f.previewUserID(input.Email)
			}
			if f.fixtureTOTPEnabled(id) {
				officialSuccess(w, map[string]any{"requires_2fa": true, "temp_token": fixtureTOTPToken + ":" + name})
				return
			}
			officialSuccess(w, credentials{name + "-token", name + "-refresh"})
		case "/auth/login/2fa":
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			name, ok := strings.CutPrefix(input["temp_token"], fixtureTOTPToken+":")
			if !ok || input["totp_code"] != "123456" {
				w.WriteHeader(401)
				return
			}
			officialSuccess(w, credentials{name + "-token", name + "-refresh"})
		case "/auth/logout":
			writeJSON(w, 200, map[string]any{"code": 0})
		case "/auth/send-verify-code":
			officialSuccess(w, map[string]any{"countdown": 60})
		case "/auth/validate-invitation-code":
			officialSuccess(w, map[string]any{"valid": true})
		case "/auth/validate-promo-code":
			officialSuccess(w, map[string]any{"valid": true, "bonus_amount": 5.0})
		case "/auth/register":
			officialSuccess(w, credentials{"user-token", "user-refresh"})
		case "/auth/forgot-password", "/auth/reset-password":
			officialSuccess(w, map[string]any{"message": "ok"})
		case "/auth/me":
			if user <= 3 && token != "user-token" && token != "other-token" && token != "admin-token" {
				w.WriteHeader(401)
				return
			}
			officialSuccess(w, map[string]any{"id": user, "role": role, "sensitive": fixtureLeak})
		case "/admin/compliance":
			officialSuccess(w, map[string]any{"required": false})
		case "/user/profile", "/user":
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			if path == "/user" && r.Method == http.MethodPut {
				var input struct{ Username *string }
				if json.NewDecoder(r.Body).Decode(&input) != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if input.Username != nil {
					f.profileNames[user] = *input.Username
				}
			}
			name := strings.TrimSuffix(token, "-token")
			officialSuccess(w, map[string]any{"id": user, "email": name + "@example.test", "username": f.profileNames[user], "role": role, "concurrency": 5, "balance": 12.50, "sensitive": fixtureLeak})
		case "/user/aff":
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			officialSuccess(w, map[string]any{"aff_code": fmt.Sprintf("AFF%d", user), "aff_quota": f.previewRebates[user],
				"aff_count": 0, "aff_frozen_quota": 0, "aff_history_quota": 3.25, "effective_rebate_rate_percent": 10, "sensitive": fixtureLeak})
		case "/user/aff/transfer":
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			amount := f.previewRebates[user]
			if amount <= 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.previewRebates[user] = 0
			officialSuccess(w, map[string]any{"transferred_quota": amount, "balance": 12.5 + amount})
		case "/model-plaza":
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			if f.previewSettings["model_plaza_enabled"] != true {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if f.previewSettings["model_plaza_require_auth"] == true && token == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			price := 0.000002
			group := modelPlazaGroup{namedItem: namedItem{ID: 8, Name: "预览模型分组"}, Platform: "openai", Subscription: "standard", Rate: 0.5,
				Models: []modelPlazaModel{{Name: "gpt-preview", Pricing: &modelPlazaPrice{Input: &price}, Official: &modelPlazaPrice{Input: &price}}}}
			officialSuccess(w, map[string]any{"description": "预览价目", "groups": []modelPlazaGroup{group}})
		case "/subscriptions":
			officialSuccess(w, []any{map[string]any{"id": 11, "group": map[string]any{"id": 2, "name": "Demo", "extra": fixtureLeak}, "status": "active", "expires_at": "2030-01-01T00:00:00Z"}})
		case "/usage", "/usage/dashboard/snapshot-v2", "/admin/usage", "/admin/dashboard/snapshot-v2":
			f.serveUsage(w, r, path, user)
		case "/groups/available":
			officialSuccess(w, []any{map[string]any{"id": 8, "name": "Example", "platform": "anthropic", "rate_multiplier": 1.0, "status": "active", "subscription_type": "standard", "extra": fixtureLeak}})
		case "/channel-monitor-v2/matrix":
			officialSuccess(w, map[string]any{"items": []any{map[string]any{"group_id": 8, "health": map[string]any{"overall": "healthy", "score": 98}}}})
		case "/usage/dashboard/api-keys-usage", "/admin/dashboard/api-keys-usage":
			var input struct {
				IDs []int64 `json:"api_key_ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			stats := map[string]any{}
			for _, id := range input.IDs {
				today, total := f.fixtureKeyUsage(id, user, strings.HasPrefix(path, "/admin/"))
				stats[strconv.FormatInt(id, 10)] = map[string]any{"api_key_id": id, "today_actual_cost": today, "total_actual_cost": total}
			}
			officialSuccess(w, map[string]any{"stats": stats})
		case "/keys":
			key := map[string]any{"id": 4, "name": "Demo", "key": fixtureLeak + "abcd", "status": "active", "user_id": user, "group_id": 8,
				"ip_whitelist": []string{}, "ip_blacklist": []string{}, "quota": 10.0, "quota_used": 2.0, "current_concurrency": 0,
				"rate_limit_5h": 0, "rate_limit_1d": 0, "rate_limit_7d": 0, "usage_5h": 0, "usage_1d": 0, "usage_7d": 0, "unneeded": fixtureLeak}
			if r.Method == "POST" {
				officialSuccess(w, key)
			} else {
				officialSuccess(w, map[string]any{"items": f.keys(user), "pages": 1, "unused": fixtureLeak})
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
		case "/keys/6", "/keys/8", "/keys/10":
			for _, key := range fixtureKeys(user) {
				if path == fmt.Sprint("/keys/", key["id"]) {
					officialSuccess(w, key)
					return
				}
			}
			w.WriteHeader(403)
		case "/redeem":
			officialSuccess(w, map[string]any{"unused": fixtureLeak})
		case "/redeem/history":
			officialSuccess(w, map[string]any{"items": []any{}, "pages": 0})
		case "/channels/available":
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			if f.previewSettings["available_channels_enabled"] != true {
				officialSuccess(w, []any{})
				return
			}
			officialSuccess(w, []any{map[string]any{"name": "预览渠道", "description": "", "platforms": []any{map[string]any{
				"platform": "openai", "groups": []any{map[string]any{"id": 8, "name": "公开预览分组", "rate_multiplier": 1, "subscription_type": "standard"}},
				"supported_models": []any{map[string]any{"name": "gpt-4.1", "pricing": map[string]any{"input_price": 0.000002, "output_price": 0.000008}}},
			}}}})
		case "/admin/users", "/admin/users/2", "/admin/users/3", "/admin/users/2/api-keys", "/admin/users/3/api-keys":
			if role != "admin" {
				w.WriteHeader(403)
				return
			}
			id := 2
			if strings.HasPrefix(path, "/admin/users/3") {
				id = 3
			}
			row := map[string]any{"id": id, "role": "user", "email": "user@example.test", "balance": 12.5, "status": "active", "internal_notes": fixtureLeak}
			if strings.HasSuffix(path, "/api-keys") {
				officialSuccess(w, map[string]any{"items": f.keys(int64(id)), "pages": 1})
			} else if path == "/admin/users" {
				officialPage(w, row)
			} else {
				officialSuccess(w, row)
			}
		case "/user/totp/step-up":
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			officialSuccess(w, map[string]any{"verified": role == "admin" && f.fixtureTOTPEnabled(user) && input["code"] == "123456"})
		case "/admin/redeem-codes/generate":
			var input struct {
				Count int
				Type  string
				Value float64
				Days  int `json:"expires_in_days"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			if r.Header.Get("X-API-Key") == "" && role == "admin" {
				out := make([]any, 0, input.Count)
				for i := range input.Count {
					id := f.issues.Add(1)
					out = append(out, map[string]any{"id": id, "code": fmt.Sprintf("ADMIN-CODE-%d-%d", id, i+1), "type": input.Type, "value": input.Value, "notes": fixtureLeak})
				}
				officialSuccess(w, out)
				return
			}
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
		case "/admin/dashboard/stats":
			officialSuccess(w, map[string]any{"total_users": 12, "today_new_users": 2, "active_users": 7, "total_accounts": 4, "normal_accounts": 3,
				"error_accounts": 1, "ratelimit_accounts": 0, "overload_accounts": 0, "total_api_keys": 8, "active_api_keys": 6,
				"today_requests": 140, "total_requests": 9200, "today_tokens": 580000, "total_tokens": 41000000,
				"today_actual_cost": 4.85, "total_actual_cost": 312.40, "average_duration_ms": 680, "uptime": 172800, "rpm": 14.5, "tpm": 28400})
		case "/admin/redeem-codes/stats":
			officialSuccess(w, map[string]any{"total_codes": 9, "active_codes": 4, "used_codes": 3, "expired_codes": 2})
		case "/admin/subscriptions", "/admin/announcements", "/admin/groups", "/admin/proxies":
			officialSuccess(w, map[string]any{"items": []any{}, "total": 5, "page": 1, "page_size": 1, "pages": 5})
		default:
			if f.previewKeys != nil && r.Method == http.MethodGet && strings.HasPrefix(path, "/keys/") {
				for _, key := range f.keys(user) {
					if path == fmt.Sprint("/keys/", key["id"]) {
						officialSuccess(w, key)
						return
					}
				}
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(up.Close)
	db, err := openDatabase(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Schema.Create(context.Background()); err != nil {
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
	app := &panel{origin: targetOrigin, official: "https://official.example.test", upstream: newClient(up.URL), issuer: newClient(internal.URL), secret: f.issuer.secret, sessions: newSessions(), database: db}
	app.usageHub = newUsageStreamHub(app.buildUsageStreamSnapshot)
	app.usageHub.commit = app.commitUsageSnapshot
	app.opsHub = newOpsStreamHub(app.buildOpsStreamSnapshot)
	app.accountHub = newOpsStreamHub(app.buildAccountActivitySnapshot)
	app.balances = newBalanceMonitor()
	f.panel = app
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
	if _, exists := u.expect("GET", "/config", "", 200)["dev_accounts"]; exists {
		t.Fatal("development accounts leaked outside the browser fixture")
	}
	if data := u.expect("GET", "/balance", "", 200); len(data) != 1 || data["balance"] != 12.5 {
		t.Fatal(data)
	}
	if u.expect("GET", "/referral", "", 200)["url"] != fixtureOrigin+"/register?aff=AFF2" {
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
	if accounts["extra"] != nil || accounts["credentials"] != nil || accounts["name"] != "Demo upstream" {
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
	configured := os.Getenv("PANEL_TEST_BROWSER_ADDR")
	if configured == "" {
		t.Skip("set PANEL_TEST_BROWSER_ADDR to enable the fixed-port browser fixture")
	}
	if configured != browserFixtureAddr {
		t.Fatalf("the browser fixture is fixed at %s", browserFixtureAddr)
	}
	address := browserFixtureAddr
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
	files := spaFileServer(os.DirFS("web/dist"))
	previewFiles := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPanelDocumentRoute(r.URL.Path) || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	frontend, err := previewVite(previewFiles, os.Getenv("PANEL_TEST_BROWSER_VITE"))
	if err != nil {
		t.Fatal(err)
	}
	f := setupFixture(t, frontend, "http://"+listener.Addr().String())
	if path := os.Getenv("PANEL_TEST_BROWSER_USAGE_FILE"); path != "" {
		if err := f.loadPreviewUsage(path); err != nil {
			t.Fatal(err)
		}
		f.panel.usageHub.build = f.buildPreviewUsageSnapshot
		f.panel.usageSampleVersion = previewSampleVersion
		t.Logf("loaded %d anonymized usage records (%s)", len(f.usageRows), previewSampleVersion)
	}
	if path := os.Getenv("PANEL_TEST_BROWSER_CATALOG_FILE"); path != "" {
		if err := f.loadPreviewCatalog(path); err != nil {
			t.Fatal(err)
		}
		t.Logf("loaded anonymized preview catalog")
	}
	f.panel.devAccounts = []devLoginAccount{
		{Label: "普通用户", Email: "user@example.test", Password: fixturePassword},
		{Label: "管理员", Email: "admin@example.test", Password: fixturePassword, OTP: "123456"},
		{Label: "其他用户", Email: "other@example.test", Password: fixturePassword},
	}
	for id := int64(4); id <= maxPreviewUser; id++ {
		if len(f.previewKeys[id]) > 0 {
			f.panel.devAccounts = append(f.panel.devAccounts, devLoginAccount{
				Label: fmt.Sprintf("样本用户 %02d", id-1), Email: previewEmail(id), Password: fixturePassword,
			})
		}
	}
	seedPreviewPatrol(t, f.panel)
	previewMux := http.NewServeMux()
	previewMux.Handle("/", f.app)
	previewMux.HandleFunc("GET /v1/usage", f.previewPublicKeyUsage)
	server := &http.Server{Handler: previewMux, ReadHeaderTimeout: 5 * time.Second}
	t.Logf("browser fixture ready at http://%s", listener.Addr())
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		t.Fatal(err)
	}
}

func seedPreviewPatrol(t *testing.T, app *panel) {
	t.Helper()
	if app.database == nil {
		return
	}
	ctx := context.Background()
	if n, err := app.database.PatrolPolicy.Query().Count(ctx); err != nil || n > 0 {
		return
	}
	now := time.Now()
	groups := []struct {
		id       int64
		name     string
		platform string
		rate     float64
		engine   string
		model    string
		effort   string
		minUsers int
	}{
		{8, "旗舰满血高并发主力通道 · 生产级粘性会话保障组", "openai", 0.30, "lmfpd", "gpt-5.4", "low", 3},
		{9, "Claude Sonnet 研发编码专用直连池", "anthropic", 0.25, "lmfpd", "claude-sonnet-4-6", "medium", 3},
		{10, "特惠低峰弹性推理通道", "openai", 0.12, "gpt-relay-inspector", "gpt-5.4-mini", "low", 5},
	}
	for _, g := range groups {
		_, _ = app.database.PatrolPolicy.Create().
			SetGroupID(g.id).
			SetGroupName(g.name).
			SetPlatform(g.platform).
			SetRateMultiplier(g.rate).
			SetEnabled(true).
			SetPublicVisible(true).
			SetEngine(g.engine).
			SetModel(g.model).
			SetProtocol(defaultProtocolForPlatform(g.platform)).
			SetEffort(g.effort).
			SetWindowMinutes(15).
			SetMinActiveUsers(g.minUsers).
			SetMinWindowRequests(15).
			SetPeakActiveUsers(15).
			SetIntervalMinutes(30).
			SetPeakIntervalMinutes(15).
			SetRecheckMinutes(5).
			SetDailyTokenLimit(300000).
			SetDailyBudgetCents(200).
			SetNextCheckAt(now.Add(18 * time.Minute).Unix()).
			SetUpdatedAt(now.Unix()).
			Save(ctx)

		for i := 59; i >= 0; i-- {
			ts := now.Add(-time.Duration(i*30) * time.Minute)
			verdict := "ok"
			trigger := "scheduled"
			inTok, outTok, reasonTok, cacheTok, durMs, ttftMs := int64(4280), int64(192), int64(148), int64(3840), int64(14200+(i%7)*950), int64(320+(i%5)*45)
			summary := "数字分布特征与目标模型家族一致，回答命中独立数字 14"
			reply := "Comparing lemon and strawberry candy counts after replacement...\n\n14"
			conf := 0.96
			if i%13 == 5 {
				verdict = "skipped_idle"
				trigger = "idle_skip"
				inTok, outTok, reasonTok, cacheTok, durMs, ttftMs = 0, 0, 0, 0, 0, 0
				summary = "近 15 分钟仅 1 名活跃用户（阈值 3），自动静默免检省 Token"
				reply = ""
			} else if g.id == 10 && i == 11 {
				verdict = "degraded"
				conf = 0.41
				summary = "流式输出特征疑似跨档降级，已触发短周期复检"
			}
			create := app.database.PatrolRun.Create().
				SetGroupID(g.id).
				SetGroupName(g.name).
				SetTrigger(trigger).
				SetVerdict(verdict).
				SetEngine(g.engine).
				SetModel(g.model).
				SetEffort(g.effort).
				SetActiveUsers(max(1, (i%8)+1)).
				SetWindowRequests(int64((i % 19) + 4)).
				SetDurationMs(durMs).
				SetTtftMs(ttftMs).
				SetInputTokens(inTok).
				SetOutputTokens(outTok).
				SetReasoningTokens(reasonTok).
				SetCachedTokens(cacheTok).
				SetEstimatedMicros(inTok*3 + outTok*15).
				SetSummary(summary).
				SetReplyExcerpt(reply).
				SetCreatedAt(ts)
			if verdict != "skipped_idle" {
				create.SetConfidence(conf).SetPredictedModel(g.model)
			}
			_, _ = create.Save(ctx)
		}
	}
}
