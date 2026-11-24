package testkit

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
	"strings"
	"testing"
	"time"

	"sub2api-panel/internal/issuer"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/router"
	"sub2api-panel/internal/server/system"
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

// RunOfficialCompatibility runs against a disposable official Sub2API installation
// with an administrator who has not set up an authenticator yet.
func RunOfficialCompatibility(t *testing.T) {
	origin, config := os.Getenv("PANEL_TEST_OFFICIAL"), os.Getenv("PANEL_TEST_ADMIN_FILE")
	if origin == "" || config == "" {
		t.Skip("configure a disposable official instance to run")
	}
	var admin mockLogin
	data, err := os.ReadFile(config)
	if err != nil || json.Unmarshal(data, &admin) != nil || admin.Email == "" || admin.Password == "" || !platform.CheckOrigin(origin, false) {
		t.Fatal("invalid official fixture configuration")
	}
	c := platform.NewClient(origin)
	trace := platform.CallMeta{Trace: platform.RandomHex(16), Span: platform.RandomHex(8)}
	ctx := platform.WithMeta(context.Background(), trace)
	call := func(method, path, bearer string, payload any, expected int) json.RawMessage {
		t.Helper()
		response, err := c.Open(ctx, method, "/api/v1"+path, bearer, "", "", payload)
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
	var initial platform.Credentials
	if json.Unmarshal(call("POST", "/auth/login", "", admin, 200), &initial) != nil || initial.Access == "" {
		t.Fatal("fixture needs a fresh official administrator")
	}
	app := &core.Server{Origin: fixtureOrigin, Official: origin, Upstream: c, Sessions: platform.NewSessions()}
	router.WireHubs(app)
	web := router.NewHandler(app, http.NotFoundHandler())
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
	firstAdmin := login(admin.Email, admin.Password, "")
	if firstAdmin.expect("GET", "/admin/compliance", "", 200)["required"] != true {
		t.Fatal("expected a fresh official administrator to require acknowledgement")
	}
	firstAdmin.expect("GET", "/admin/accounts", "", 423)
	firstAdmin.expect("POST", "/admin/compliance/accept", `{"phrase":"incorrect"}`, 400)
	confirmation := firstAdmin.expect("GET", "/admin/compliance", "", 200)
	result := firstAdmin.expect("POST", "/admin/compliance/accept", fmt.Sprintf(`{"phrase":%q}`, confirmation["ack_phrase_zh"]), 200)
	if result["required"] != false {
		t.Fatal("official acknowledgement did not persist")
	}
	call("PUT", "/admin/settings", initial.Access, map[string]any{
		"registration_enabled": true, "totp_enabled": true, "affiliate_enabled": true,
		"affiliate_admin_recharge_enabled": true, "affiliate_rebate_freeze_hours": 0, "affiliate_rebate_rate": 10,
		"promo_code_enabled": true, "invitation_code_enabled": false, "model_plaza_enabled": true,
	}, 200)
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
	password := platform.RandomToken()
	var registered platform.Credentials
	userEmail := "delegate-" + platform.RandomHex(5) + "@panel.test"
	if json.Unmarshal(call("POST", "/auth/register", "", mockLogin{Email: userEmail, Password: password}, 200), &registered) != nil {
		t.Fatal("could not register delegate")
	}
	user, err := platform.CurrentUser(ctx, c, registered.Access)
	if err != nil {
		t.Fatal(err)
	}
	databaseURL := os.Getenv("PANEL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("configure PANEL_TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	db, err := platform.OpenDatabase(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Schema.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	i := &issuer.Service{DB: db, Upstream: c, AdminKey: issuerSecret.Key, Secret: platform.RandomToken()}
	internal := httptest.NewServer(i.Routes())
	defer internal.Close()
	app.Issuer, app.Secret = platform.NewClient(internal.URL), i.Secret
	a := login(admin.Email, admin.Password, setup.Secret)
	u := login(userEmail, password, "")
	u.expect("GET", "/admin/compliance", "", 403)
	u.expect("GET", "/settings/profile", "", 200)
	testOfficialSiteContent(t, a, u, func(code, affiliateCode string) platform.Identity {
		var signup platform.Credentials
		response := call("POST", "/auth/register", "", map[string]any{
			"email": "promo-" + platform.RandomHex(5) + "@panel.test", "password": password, "promo_code": code, "aff_code": affiliateCode,
		}, 200)
		if json.Unmarshal(response, &signup) != nil || signup.Access == "" {
			t.Fatal("could not register a test invitee with a promotion")
		}
		var profile platform.Identity
		if json.Unmarshal(call("GET", "/user/profile", signup.Access, nil, 200), &profile) != nil || profile.ID < 1 {
			t.Fatal("could not read the promotional registration")
		}
		var balance struct{ Balance float64 }
		if json.Unmarshal(call("GET", "/user/profile", signup.Access, nil, 200), &balance) != nil || balance.Balance < 3.5 {
			t.Fatal("promotional balance was not granted at registration")
		}
		return profile
	})
	u.expect("GET", "/model-plaza", "", 200)
	guest := httptest.NewRecorder()
	web.ServeHTTP(guest, httptest.NewRequest("GET", "/api/public/model-plaza", nil))
	checkResponse(t, guest, 200)
	call("PUT", "/admin/settings", initial.Access, map[string]bool{"model_plaza_require_auth": true, "invitation_code_enabled": true}, 200)
	checkResponse(t, u.request("GET", "/model-plaza", ""), 200)
	guest = httptest.NewRecorder()
	web.ServeHTTP(guest, httptest.NewRequest("GET", "/api/public/model-plaza", nil))
	checkResponse(t, guest, 401)
	quotaPath := fmt.Sprintf("/admin/users/%d/platform-quotas", user.ID)
	u.expect("GET", quotaPath, "", 403)
	quota := a.expect("PUT", quotaPath, `{"quotas":[{"platform":"openai","daily_limit_usd":10,"weekly_limit_usd":0,"monthly_limit_usd":null}]}`, 200)["platform_quotas"].([]any)
	if len(quota) != 1 || quota[0].(map[string]any)["weekly_limit_usd"] != float64(0) {
		t.Fatal("could not read back the saved platform quota")
	}
	a.expect("POST", quotaPath+"/reset", `{"platform":"openai","window":"daily"}`, 200)
	attribute := call("POST", "/admin/user-attributes", initial.Access, map[string]any{
		"key": "panel_" + platform.RandomHex(4), "name": "交付验收", "type": "text", "enabled": true,
	}, 200)
	var attributeID struct{ ID int64 }
	if json.Unmarshal(attribute, &attributeID) != nil || attributeID.ID < 1 {
		t.Fatal("could not create official attribute definition")
	}
	attributePath := fmt.Sprintf("/admin/users/%d/attributes", user.ID)
	u.expect("GET", attributePath, "", 403)
	checkResponse(t, a.request("GET", "/admin/user-attributes", ""), 200)
	checkResponse(t, a.request("PUT", attributePath, fmt.Sprintf(`{"values":{"%d":"复验值"}}`, attributeID.ID)), 200)
	attributeResponse := a.request("GET", attributePath, "")
	checkResponse(t, attributeResponse, 200)
	var attributes []mockUserAttributeValue
	if json.Unmarshal(attributeResponse.Body.Bytes(), &attributes) != nil || len(attributes) != 1 || attributes[0].Value != "复验值" {
		t.Fatal("could not read back the saved user attribute")
	}
	grantPath := fmt.Sprintf("/admin/grants/%d", user.ID)
	a.expect("PUT", grantPath, fmt.Sprintf(`{"invites":1,"budget_cents":500,"amounts_cents":[500],"totp":%q}`, authenticatorCode(t, setup.Secret)), 200)
	newIssue := func(kind string, amount int) string {
		return fmt.Sprintf(`{"operation_id":"%s-%s-4%s-8%s-%s","kind":%q,"amount_cents":%d}`,
			platform.RandomHex(4), platform.RandomHex(2), platform.RandomHex(2)[1:], platform.RandomHex(2)[1:], platform.RandomHex(6), kind, amount)
	}
	invite := u.expect("POST", "/issues", newIssue("invitation", 0), 201)["code"]
	balanceCode := u.expect("POST", "/issues", newIssue("balance", 500), 201)["code"]
	var recipient platform.Credentials
	recipientEmail := "recipient-" + platform.RandomHex(5) + "@panel.test"
	if json.Unmarshal(call("POST", "/auth/register", "", map[string]any{"email": recipientEmail, "password": password, "invitation_code": invite}, 200), &recipient) != nil {
		t.Fatal("invitation register response")
	}
	call("POST", "/auth/register", "", map[string]any{"email": "second-" + platform.RandomHex(5) + "@panel.test", "password": password, "invitation_code": invite}, 400)
	call("POST", "/redeem", recipient.Access, map[string]any{"code": balanceCode}, 200)
	var profile struct{ Balance float64 }
	if json.Unmarshal(call("GET", "/user/profile", recipient.Access, nil, 200), &profile) != nil || profile.Balance < 5 {
		t.Fatal("official redemption balance")
	}
	u.expect("GET", "/balance", "", 200)
	u.expect("GET", "/usage", "", 200)
	u.expect("GET", "/keys", "", 200)
	var account platform.NamedItem
	if json.Unmarshal(call("POST", "/admin/accounts", initial.Access, map[string]any{
		"name": "panel fixture " + platform.RandomHex(5), "platform": "openai", "type": "apikey",
		"credentials": map[string]string{"api_key": "panel-fixture-placeholder"},
		"concurrency": 1, "priority": 50,
	}, 200), &account) != nil || account.ID < 1 {
		t.Fatal("could not create disposable upstream account")
	}
	other := login(recipientEmail, password, "")
	history := other.expect("GET", "/redeem/history", "", 200)["items"].([]any)
	if len(history) == 0 || history[0].(map[string]any)["type"] != "balance" || history[0].(map[string]any)["code"] != nil {
		t.Fatal("the recipient history did not show the balance redemption without the secret")
	}
	checkResponse(t, u.request("GET", "/channels/available", ""), 200)
	if got := a.expect("GET", "/admin/settings-tools/admin-key", "", 200); got["legacy_exists"] != false {
		t.Fatal("a fresh official instance should use scoped management keys")
	}
	testOfficialUsage(t, a, u, other, account.ID)
	a.expect("GET", "/admin/accounts", "", 200)
	schedulePath := fmt.Sprintf("/admin/accounts/%d/scheduling", account.ID)
	a.expect("PUT", schedulePath, `{"schedulable":false}`, 200)
	a.expect("PUT", schedulePath, `{"schedulable":true}`, 200)
	a.expect("GET", "/admin/users", "", 200)
	a.expect("GET", "/issues", "", 200)
	a.expect("DELETE", grantPath, "", 200)
	u.expect("POST", "/issues", newIssue("balance", 500), 403)
	u.expect("GET", "/admin/audit-logs", "", 403)
	for _, path := range []string{"/admin/backups/s3-config", "/admin/backups/schedule"} {
		u.expect("GET", path, "", http.StatusForbidden)
		a.expect("GET", path, "", http.StatusOK)
	}
	backups := a.request(http.MethodGet, "/admin/backups", "")
	checkResponse(t, backups, http.StatusOK)
	if strings.TrimSpace(backups.Body.String()) != "[]" {
		t.Fatal("the isolated official fixture should have no backups")
	}
	a.expect("POST", "/admin/backups", `{"expire_days":-1}`, http.StatusBadRequest)
	a.expect("POST", "/admin/backups", `{"totp":"invalid"}`, http.StatusForbidden)
	u.expect("GET", "/admin/plugins/upstream", "", http.StatusForbidden)
	u.expect("GET", adminOrderPath, "", http.StatusForbidden)
	a.expect("GET", adminOrderPath, "", http.StatusOK)
	a.expect("GET", adminOrderPath+"/99999999", "", http.StatusNotFound)
	a.expect("GET", "/admin/audit-logs", "", 200)
	a.expect("POST", "/admin/audit-logs/clear", `{"totp":"invalid"}`, 400)
	a.expect("GET", "/admin/risk-control/config", "", 200)
	a.expect("GET", "/admin/risk-control/logs", "", 200)
	u.expect("GET", "/admin/risk-control/logs", "", 403)
	u.expect("GET", "/payment/orders", "", 200)
	call("PUT", "/admin/settings", initial.Access, map[string]any{"channel_monitor_enabled": true, "channel_monitor_mode": "v1"}, 200)
	if result := u.expect("GET", "/channel-monitor", "", 200); result["mode"] != "v1" {
		t.Fatal("unexpected official channel monitor mode")
	}
	t.Logf("official compatibility finished trace=%s user=%d", trace.Trace, user.ID)
}

func testOfficialSiteContent(t *testing.T, admin, user *fixtureBrowser, register func(string, string) platform.Identity) {
	t.Helper()
	const announcements = "/admin/announcements"
	const promos = "/admin/promo-codes"
	user.expect("GET", announcements, "", 403)
	user.expect("POST", promos, "{}", 403)
	admin.expect("POST", announcements, `{"title":"交付验收","content":"公告正文","status":"active","notify_mode":"popup","targeting":{}}`, 200)
	rows := admin.expect("GET", announcements, "", 200)["items"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["content"] != nil {
		t.Fatal("announcement list should contain the summary")
	}
	id := int64(rows[0].(map[string]any)["id"].(float64))
	detail := fmt.Sprintf("%s/%d", announcements, id)
	if result := admin.expect("GET", detail, "", 200); result["content"] != "公告正文" || result["created_by"] != nil {
		t.Fatal("unexpected announcement detail")
	}
	read := func(path string) []system.UserAnnouncement {
		t.Helper()
		response := user.request("GET", path, "")
		checkResponse(t, response, 200)
		var list []system.UserAnnouncement
		if json.Unmarshal(response.Body.Bytes(), &list) != nil {
			t.Fatal("invalid user announcements")
		}
		return list
	}
	if list := read("/announcements?unread_only=true"); len(list) != 1 || list[0].ID != id || list[0].NotifyMode != "popup" {
		t.Fatal("published announcement was not delivered")
	}
	user.expect("POST", fmt.Sprintf("/announcements/%d/read", id), "", 200)
	if list := read("/announcements"); len(list) != 1 || list[0].ReadAt == nil || len(read("/announcements?unread_only=true")) != 0 {
		t.Fatal("announcement read state did not persist")
	}
	admin.expect("GET", detail+"/read-status", "", 200)
	admin.expect("PUT", detail, `{"title":"交付验收","content":"已归档","status":"archived","notify_mode":"silent","targeting":{}}`, 200)
	if len(read("/announcements")) != 0 {
		t.Fatal("archived announcement is still visible")
	}
	admin.expect("DELETE", detail, "", 200)

	response := admin.request("POST", promos, `{"bonus_amount":3.5,"max_uses":1,"notes":"登记验收"}`)
	checkResponse(t, response, 200)
	code := object(t, response.Body.Bytes())["code"].(string)
	if code == "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("new promo code should be available in a non-cacheable creation response")
	}
	rows = admin.expect("GET", promos, "", 200)["items"].([]any)
	if len(rows) != 1 {
		t.Fatal("expected the created promo code")
	}
	promo := rows[0].(map[string]any)
	if promo["masked"] != system.MaskRedeemCode(code) || promo["code"] != nil || promo["bonus_amount"] != 3.5 {
		t.Fatal("unexpected promo list projection")
	}
	detail = fmt.Sprintf("%s/%d", promos, int64(promo["id"].(float64)))
	referral := user.expect("GET", "/affiliate", "", 200)["aff_code"].(string)
	invitee := register(code, referral)
	rows = admin.expect("GET", detail+"/usages", "", 200)["items"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["user_id"] != float64(invitee.ID) {
		t.Fatal("promo usage did not record the recipient")
	}
	admin.expect("POST", fmt.Sprintf("/admin/users/%d/balance", invitee.ID), `{"balance":30,"operation":"add","notes":"referral verification"}`, 200)
	if result := user.expect("POST", "/affiliate/transfer", "", 200); result["transferred_quota"] != float64(3) {
		t.Fatal("unexpected transferred affiliate amount")
	}
	if result := user.expect("GET", "/affiliate", "", 200); result["aff_quota"] != float64(0) || result["aff_count"] != float64(1) {
		t.Fatal("affiliate detail did not reflect the transfer and invitee")
	}
	admin.expect("PUT", detail, `{"status":"disabled","notes":"已验收"}`, 200)
	admin.expect("GET", detail+"/usages", "", 200)
	admin.expect("DELETE", detail, "", 200)
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
	secretPath := "/keys/" + first + "/secret"
	secretResponse := user.request("GET", secretPath, "")
	checkResponse(t, secretResponse, 200)
	secret := object(t, secretResponse.Body.Bytes())["key"]
	if secret == "" || secret == nil || secretResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("could not retrieve the owner key without caching")
	}
	for _, reader := range []*fixtureBrowser{other, admin} {
		response := reader.request("GET", secretPath, "")
		if (response.Code != http.StatusForbidden && response.Code != http.StatusNotFound) || object(t, response.Body.Bytes())["key"] != nil {
			t.Fatal("foreign session could retrieve owner key")
		}
	}
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
	sql := fmt.Sprintf(`
INSERT INTO usage_logs (user_id, api_key_id, account_id, request_id, model, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, actual_cost, created_at)
SELECT k.user_id, k.id, %d, 'panel-usage-' || k.id || '-' || sample.n, sample.model, sample.input,
       5, 7, 9, 0.02, date_trunc('day', now(), 'Asia/Shanghai') - sample.days * interval '1 day'
FROM (SELECT %s AS key_id, n, 'sample-a' AS model, 20 AS input, CASE WHEN n = %d THEN 45 ELSE n %% 2 END AS days FROM generate_series(0, %d) AS n
      UNION ALL SELECT %s, 0, 'sample-b', 50, 0
      UNION ALL SELECT %s, 0, 'private-sample', 300, 0) AS sample
JOIN api_keys k ON k.id = sample.key_id;`, account, first, platform.PageSize, platform.PageSize, second, foreign)
	seed := exec.Command("docker", "exec", "-i", container, "sh", "-c", `psql -X --set=ON_ERROR_STOP=1 --username="$POSTGRES_USER" --dbname="$POSTGRES_DB"`)
	seed.Stdin = strings.NewReader(sql)
	if output, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("could not seed official usage: %v %s", err, output)
	}
	summary := user.expect("GET", "/usage/summary"+query, "", 200)
	totals := summary["totals"].(map[string]any)
	days := summary["days"].([]any)
	models := summary["models"].(map[string]any)["items"].([]any)
	if totals["requests"] != float64(platform.PageSize) || totals["input_tokens"] != float64(platform.PageSize*20) ||
		totals["output_tokens"] != float64(platform.PageSize*5) || totals["cache_read_tokens"] != float64(platform.PageSize*7) ||
		totals["cache_creation_tokens"] != float64(platform.PageSize*9) || math.Abs(totals["actual_cost"].(float64)-0.02*platform.PageSize) > 1e-9 ||
		days[5].(map[string]any)["input_tokens"] != float64(platform.PageSize*10) || days[6].(map[string]any)["input_tokens"] != float64(platform.PageSize*10) ||
		len(models) != 1 || models[0].(map[string]any)["model"] != "sample-a" || models[0].(map[string]any)["total_tokens"] != float64(platform.PageSize*41) {
		t.Fatal(summary)
	}
	all := user.expect("GET", "/usage/summary", "", 200)
	if all["totals"].(map[string]any)["requests"] != float64(platform.PageSize+1) {
		t.Fatal(all)
	}
	today := user.expect("GET", "/usage"+query+"&period=today", "", 200)
	if items := today["items"].([]any); len(items) != platform.PageSize/2 || items[0].(map[string]any)["cache_creation_tokens"] != float64(9) {
		t.Fatal(today)
	}
	history := user.expect("GET", "/usage"+query+"&page=2", "", 200)
	if items := history["items"].([]any); len(items) != 1 || len(items[0].(map[string]any)) != 9 || history["more"] != false {
		t.Fatal(history)
	}
}
