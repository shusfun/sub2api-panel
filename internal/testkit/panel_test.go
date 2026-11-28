package testkit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/system"
)

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
	u.expect("GET", "/admin/settings-tools/panel-update", "", 403)
	u.expect("POST", "/admin/settings-tools/panel-update", `{"version":"0.1.0001","totp":"123456"}`, 403)
	a.expect("POST", "/admin/settings-tools/panel-update", `{"version":"0.1.0001","totp":"000000"}`, 403)
	a.expect("POST", "/admin/settings-tools/panel-update", `{"version":"0.1.0001","totp":"123456"}`, 503)
	if status := a.expect("GET", "/admin/settings-tools/panel-update", "", 200); status["configured"] != false {
		t.Fatal("unmanaged panel should report online updates disabled")
	}
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

func TestPanelUpdateAdminFlow(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	setupUpdateFixture(t, f)
	admin := browserLogin(t, f.app, "admin")
	const path = "/admin/settings-tools/panel-update"
	admin.expect("GET", path, "", 200)
	admin.expect("GET", path+"/release", "", 200)
	admin.expect("POST", path, `{"version":"0.1.0001","totp":"000000"}`, 403)
	job := admin.expect("POST", path, `{"version":"0.1.0001","totp":"123456"}`, 202)
	if job["phase"] != "queued" || job["to"] != "0.1.0001" {
		t.Fatalf("unexpected queued job: %v", job)
	}
	admin.expect("POST", path, `{"version":"0.1.0001","totp":"123456"}`, 409)
	admin.expect("POST", "/session/logout", "", 200)
	admin = browserLogin(t, f.app, "admin")
	status := admin.expect("GET", path, "", 200)
	if stored, ok := status["job"].(map[string]any); !ok || stored["id"] != job["id"] {
		t.Fatalf("re-login lost latest update job: %v", status)
	}
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

func TestFieldErrorAndConflictContract(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rec := httptest.NewRecorder()
	req, end := platform.TraceRequest(req, rec, "POST /api/test")
	end()
	platform.WriteError(rec, req, platform.FailFields(map[string]string{"name": "名称不能为空", "allowed_ips": "CIDR 格式有误"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var out platform.ProblemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != "invalid_input" || out.Fields["name"] != "名称不能为空" || out.Fields["allowed_ips"] != "CIDR 格式有误" || out.ID == "" {
		t.Fatalf("unexpected field error payload: %+v", out)
	}

	conflictErr := platform.TranslateUpstream(platform.UpstreamStatus{Status: http.StatusConflict})
	var prob *platform.Problem
	if conflictErr == nil || conflictErr.Error() != "conflict" {
		t.Fatalf("expected 409 upstream status to map to conflict, got %v", conflictErr)
	}
	_ = prob
}

func TestOfficialPluginConfigRedactionAndMerge(t *testing.T) {
	stored := map[string]json.RawMessage{
		"api_key":     json.RawMessage(`"sk-super-secret"`),
		"enabled":     json.RawMessage(`true`),
		"max_retries": json.RawMessage(`3`),
		"endpoint":    json.RawMessage(`"https://relay.example.test"`),
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/settings":
			officialSuccess(w, map[string]any{"step_up_enabled": false})
		case "/api/v1/admin/plugins/1/config":
			if r.Method == http.MethodPut {
				var next map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				stored = next
			}
			platform.WriteJSON(w, http.StatusOK, stored)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer up.Close()

	p := &core.Server{Origin: fixtureOrigin, Upstream: platform.NewClient(up.URL)}
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/plugins/upstream/1/config", nil)
	getReq.SetPathValue("id", "1")
	getRec := httptest.NewRecorder()
	if err := system.OfficialPluginConfig(p, getRec, getReq, core.SessionAuth{User: platform.Identity{ID: 1, Role: "admin"}, Tokens: platform.Credentials{Access: "op"}}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(getRec.Body.Bytes(), []byte("sk-super-secret")) {
		t.Fatalf("plugin secret leaked in GET response: %s", getRec.Body.String())
	}
	var view system.OfficialPluginConfigView
	if err := json.Unmarshal(getRec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Fields) != 4 || view.Fields[0].Key != "api_key" || view.Fields[0].Kind != "secret" || !view.Fields[0].Configured {
		t.Fatalf("unexpected classified plugin fields: %+v", view.Fields)
	}

	putBody := `{"totp":"","config":{"enabled":false,"max_retries":5,"endpoint":"https://new.example.test"},"preserve_secrets":["api_key"]}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/admin/plugins/upstream/1/config", strings.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.SetPathValue("id", "1")
	putRec := httptest.NewRecorder()
	if err := system.OfficialPluginConfig(p, putRec, putReq, core.SessionAuth{User: platform.Identity{ID: 1, Role: "admin"}, Tokens: platform.Credentials{Access: "op"}}); err != nil {
		t.Fatal(err)
	}
	if string(stored["api_key"]) != `"sk-super-secret"` || string(stored["max_retries"]) != "5" {
		t.Fatalf("expected secret to be preserved and max_retries updated, got %s", stored)
	}
}
