package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"sub2api-panel/internal/issuer"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/router"
	"sub2api-panel/internal/server/system"
)

func setupFixture(t testing.TB, static http.Handler, origin ...string) *fixture {
	t.Helper()
	databaseURL := os.Getenv("PANEL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("configure PANEL_TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	f := new(fixture)
	f.previewUserGroups = make(map[int64][]int64)
	f.previewRebates = map[int64]float64{2: 3.25}
	f.previewPayment = make(map[int64]mockPaymentOrder)
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
		f.serveUpstream(t, w, r)
	}))
	t.Cleanup(up.Close)
	db, err := platform.OpenDatabase(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Schema.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f.issuer = &issuer.Service{DB: db, Upstream: platform.NewClient(up.URL), Secret: platform.RandomToken(), AdminKey: fixtureKey}
	internal := httptest.NewServer(f.issuer.Routes())
	t.Cleanup(internal.Close)
	targetOrigin := fixtureOrigin
	if len(origin) != 0 {
		targetOrigin = origin[0]
	}
	app := &core.Server{
		Origin:   targetOrigin,
		Official: "https://official.example.test",
		Upstream: platform.NewClient(up.URL),
		Issuer:   platform.NewClient(internal.URL),
		Secret:   f.issuer.Secret,
		Sessions: platform.NewSessions(),
		Database: db,
		Balances: core.NewBalanceMonitor(),
	}
	router.WireHubs(app)
	f.panel = app
	f.app = router.NewHandler(app, static)
	return f
}

func (f *fixture) serveUpstream(t testing.TB, w http.ResponseWriter, r *http.Request) {
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
	span := platform.SpanHex.FindStringSubmatch(r.Header.Get("traceparent"))
	if !platform.TraceHex.MatchString(trace) || len(span) != 3 || span[1] != trace {
		t.Errorf("uncorrelated upstream %s", path)
	}
	if r.Header.Get("X-API-Key") == "" && (f.servePreview(w, r, path, role, user) ||
		f.serveAdminExtended(w, r, path, role) || f.serveCleanupAndImage(w, r, path, role) || f.serveWhitelistSamples(w, r, path, role) || f.servePreviewGaps(w, r, path, role) || f.servePreviewLedger(w, r, path, role) ||
		f.servePreviewExtra(w, r, path, role, user) ||
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
		var input mockLogin
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
		officialSuccess(w, platform.Credentials{Access: name + "-token", Refresh: name + "-refresh"})
	case "/auth/login/2fa":
		var input map[string]string
		_ = json.NewDecoder(r.Body).Decode(&input)
		name, ok := strings.CutPrefix(input["temp_token"], fixtureTOTPToken+":")
		if !ok || input["totp_code"] != "123456" {
			w.WriteHeader(401)
			return
		}
		officialSuccess(w, platform.Credentials{Access: name + "-token", Refresh: name + "-refresh"})
	case "/auth/logout":
		platform.WriteJSON(w, 200, map[string]any{"code": 0})
	case "/auth/send-verify-code":
		officialSuccess(w, map[string]any{"countdown": 60})
	case "/auth/validate-invitation-code":
		officialSuccess(w, map[string]any{"valid": true})
	case "/auth/validate-promo-code":
		officialSuccess(w, map[string]any{"valid": true, "bonus_amount": 5.0})
	case "/auth/register":
		officialSuccess(w, platform.Credentials{Access: "user-token", Refresh: "user-refresh"})
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
		group := mockModelPlazaGroup{NamedItem: platform.NamedItem{ID: 8, Name: "预览模型分组"}, Platform: "openai", Subscription: "standard", Rate: 0.5,
			Models: []mockModelPlazaModel{{Name: "gpt-preview", Pricing: &mockModelPlazaPrice{Input: &price}, Official: &mockModelPlazaPrice{Input: &price}}}}
		officialSuccess(w, map[string]any{"description": "预览价目", "groups": []mockModelPlazaGroup{group}})
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
			platform.WriteJSON(w, 403, map[string]any{"msg": fixtureLeak})
			return
		}
		if r.Method == "DELETE" {
			platform.WriteJSON(w, 200, map[string]int{"code": 0})
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
		if r.Header.Get("X-API-Key") != fixtureKey || !platform.OperationPattern.MatchString(r.Header.Get("Idempotency-Key")) || input.Count != 1 ||
			(input.Type == "invitation" && input.Days != 7) || (input.Type == "balance" && input.Days != 30) {
			t.Error("invalid request to official issuer")
		}
		id := f.issues.Add(1)
		if status := f.issueStatus.Load(); status != 0 {
			platform.WriteJSON(w, int(status), map[string]any{"msg": fixtureLeak})
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
}

func (f *fixture) fixtureTOTPEnabled(user int64) bool {
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	return f.totpState[user]
}

func (f *fixture) serveFixtureTOTP(w http.ResponseWriter, r *http.Request, path string, user int64) bool {
	const setupToken = "local-preview-totp-setup"
	switch path {
	case "/user/totp/status":
		officialSuccess(w, map[string]any{"enabled": f.fixtureTOTPEnabled(user), "feature_enabled": f.totpFeature.Load(), "added_field": fixtureLeak})
	case "/user/totp/verification-method":
		officialSuccess(w, map[string]string{"method": "password"})
	case "/user/totp/setup", "/user/totp/disable":
		var input system.TOTPIdentityProof
		if json.NewDecoder(r.Body).Decode(&input) != nil || input.Password != fixturePassword {
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		if path == "/user/totp/setup" {
			if !f.totpFeature.Load() || f.fixtureTOTPEnabled(user) {
				w.WriteHeader(http.StatusForbidden)
				return true
			}
			officialSuccess(w, map[string]any{
				"qr_code_url": "otpauth://totp/Sub2API%3Apreview?secret=JBSWY3DPEHPK3PXP&issuer=Sub2API",
				"setup_token": setupToken, "countdown": 300, "secret": fixtureLeak,
			})
		} else {
			f.previewMu.Lock()
			f.totpState[user] = false
			f.previewMu.Unlock()
			officialSuccess(w, map[string]bool{"success": true})
		}
	case "/user/totp/enable":
		var values map[string]string
		if json.NewDecoder(r.Body).Decode(&values) != nil || values["setup_token"] != setupToken || values["totp_code"] != "123456" || !f.totpFeature.Load() {
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		f.previewMu.Lock()
		f.totpState[user] = true
		f.previewMu.Unlock()
		officialSuccess(w, map[string]bool{"success": true})
	default:
		return false
	}
	return true
}
