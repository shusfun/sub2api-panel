package testkit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/observe"
	"sub2api-panel/internal/server/pool"
	"sub2api-panel/internal/server/users"
)

func TestProjectProxyOmitsCredentials(t *testing.T) {
	username, password := "proxy-user", "proxy-password"
	view, err := pool.ProjectProxy(pool.ProxyWire{
		ID: 7, Name: "edge", Protocol: "http", Host: "127.0.0.1", Port: 8080,
		Username: &username, Password: &password, Status: "active", Fallback: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), username) || strings.Contains(string(encoded), password) || strings.Contains(string(encoded), `"password"`) || strings.Contains(string(encoded), `"username"`) {
		t.Fatalf("proxy credentials leaked through view: %s", encoded)
	}
	if !view.HasAuth {
		t.Fatal("expected the view to indicate configured authentication")
	}
}

func TestProjectKeyEnforcesOwnerAndMasksSecret(t *testing.T) {
	key := users.KeyWire{NamedItem: platform.NamedItem{ID: 4, Name: "Demo"}, Key: "sk-secret-material", Owner: 9, Status: "active"}
	view, err := users.ProjectKey(key, 9)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(view.Masked, key.Key) {
		t.Fatalf("full key appeared in masked field: %s", view.Masked)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), key.Key) || strings.Contains(string(encoded), `"key"`) {
		t.Fatalf("raw key field leaked through the list projection: %s", encoded)
	}
	if _, err := users.ProjectKey(key, 10); err == nil {
		t.Fatal("projectKey accepted a key owned by another user")
	}
}

func TestBuildAdminUsageSummaryScopesSnapshotToSelectedUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/dashboard/snapshot-v2" {
			t.Errorf("unexpected upstream path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("user_id"); got != "42" {
			t.Errorf("snapshot user_id = %q, want 42", got)
		}
		if got := r.URL.Query().Get("model"); got != "model-a" {
			t.Errorf("snapshot model = %q, want model-a", got)
		}
		platform.WriteJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{
			"start_date": "2026-01-01",
			"end_date":   "2026-01-01",
			"trend": []any{map[string]any{
				"date": "2026-01-01", "requests": 2, "input_tokens": 12, "output_tokens": 7,
				"cache_read_tokens": 4, "cache_creation_tokens": 1, "actual_cost": 0.3,
				"account_cost": 999, "internal": fixtureLeak,
			}},
			"models": []any{map[string]any{"model": "model-a", "requests": 2, "total_tokens": 24, "actual_cost": 0.3, "account_cost": 999}},
			"stats":  map[string]any{"total_users": 999},
		}})
	}))
	defer server.Close()
	zone, _ := core.LoadUsageLocation("Asia/Shanghai")
	app := &core.Server{Upstream: platform.NewClient(server.URL), UsageLocation: zone}
	query := url.Values{
		"user_id": {"42"}, "model": {"model-a"},
		"start_date": {"2026-01-01"}, "end_date": {"2026-01-01"},
	}
	result, err := observe.BuildAdminUsageSummary(app, context.Background(), "admin-session", query, 1)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Totals.Requests != 2 || result.Totals.Input != 12 || result.Models.Items[0].Model != "model-a" ||
		strings.Contains(string(encoded), fixtureLeak) || strings.Contains(string(encoded), "account_cost") || strings.Contains(string(encoded), "total_users") {
		t.Fatalf("unexpected projected usage summary %s", encoded)
	}
}

func TestAdminUsageScopeSeparatesActorAndTarget(t *testing.T) {
	query := url.Values{"user_id": {"42"}, "period": {"7d"}}
	first := core.UsageScope(1, "42\x00"+query.Encode())
	second := core.UsageScope(1, "43\x00"+query.Encode())
	otherAdmin := core.UsageScope(2, "42\x00"+query.Encode())
	if first == second || first == otherAdmin {
		t.Fatal("admin usage scopes must separate target users and administrators")
	}
}

func TestUsageStreamPreservesTraceOnSharedSample(t *testing.T) {
	trace := platform.CallMeta{Trace: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Span: "bbbbbbbbbbbbbbbb"}
	observed := make(chan platform.CallMeta, 1)
	hub := core.NewUsageStreamHub(func(ctx context.Context, _, _ string) ([]byte, error) {
		observed <- platform.Meta(ctx)
		return []byte(`{"timezone":"Asia/Shanghai","start_date":"2026-01-01","end_date":"2026-01-01","totals":{"input_tokens":0,"output_tokens":0,"cache_read_tokens":0,"cache_creation_tokens":0,"requests":0,"actual_cost":0},"days":[{"date":"2026-01-01","input_tokens":0,"output_tokens":0,"cache_read_tokens":0,"cache_creation_tokens":0,"requests":0,"actual_cost":0}],"models":{"items":[],"more":false}}`), nil
	})
	channel, unsubscribe := hub.Subscribe(context.Background(), "scope", "token", "period=today", 1, trace)
	defer unsubscribe()
	select {
	case got := <-observed:
		if got.Trace != trace.Trace || got.Span != trace.Span {
			t.Fatalf("sample trace metadata = %#v, want %#v", got, trace)
		}
	case <-time.After(time.Second):
		t.Fatal("usage stream did not start its shared sample")
	}
	select {
	case event := <-channel:
		if len(event) == 0 {
			t.Fatal("usage stream sent an empty event")
		}
	case <-time.After(time.Second):
		t.Fatal("usage stream did not deliver the first snapshot")
	}
}

func TestAdminUsageFilterAcceptsModelAndDateWindow(t *testing.T) {
	zone, err := core.LoadUsageLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{
		"user_id": {"42"}, "model": {"claude-sonnet"},
		"start_date": {"2026-01-01"}, "end_date": {"2026-01-02"},
	}
	filter, err := observe.ParseUsageFilter(query, time.Date(2026, 1, 5, 0, 0, 0, 0, zone), false)
	if err != nil {
		t.Fatal(err)
	}
	if filter.Model != "claude-sonnet" || filter.Start.Format(observe.UsageDateLayout) != "2026-01-01" || filter.End.Format(observe.UsageDateLayout) != "2026-01-02" {
		t.Fatalf("unexpected admin usage filter %#v", filter)
	}
}

func TestConsolidatedWorkspacesAndSecurityBoundaries(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	user := browserLogin(t, f.app, "user")
	admin := browserLogin(t, f.app, "admin")

	for _, adminPath := range []string{
		"/admin/ops/overview",
		"/admin/ops/concurrency",
		"/admin/ops/request-errors",
		"/admin/groups",
		"/admin/channels",
		"/admin/accounts",
		"/admin/subscriptions",
		"/admin/redeem-codes",
	} {
		user.expect("GET", adminPath, "", 403)
	}

	overview := admin.expect("GET", "/admin/overview", "", 200)
	if overview["traffic"] == nil || overview["top_users"] == nil {
		t.Fatalf("expected enriched admin overview: %#v", overview)
	}
	opsView := admin.expect("GET", "/admin/ops/overview?time_range=1h", "", 200)
	if len(opsView["throughput_points"].([]any)) != 12 || len(opsView["latency_buckets"].([]any)) != 5 {
		t.Fatalf("unexpected ops overview: %#v", opsView)
	}
	opsConc := admin.expect("GET", "/admin/ops/concurrency?time_range=1h", "", 200)
	if len(opsConc["groups"].([]any)) != 1 || len(opsConc["accounts"].([]any)) != 1 {
		t.Fatalf("unexpected ops concurrency: %#v", opsConc)
	}
	errPage := admin.expect("GET", "/admin/ops/request-errors?time_range=1h", "", 200)
	errItem := errPage["items"].([]any)[0].(map[string]any)
	if errItem["error_body"] != nil || errItem["message"] != nil || errItem["upstream_status_code"] != float64(429) {
		t.Fatalf("ops error projection leaked raw payload or missed status: %#v", errItem)
	}
	admin.expect("PUT", "/admin/ops/request-errors/77/resolve", `{"resolved":true}`, 200)
	checkResponse(t, admin.request("GET", "/admin/ops/alert-rules", ""), 200)
	admin.expect("GET", "/admin/ops/settings", "", 200)

	groups := admin.expect("GET", "/admin/groups", "", 200)["items"].([]any)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	preview := admin.expect("POST", "/admin/groups/12/composite-routes/preview", `{"model":"claude-sonnet-4-5","endpoint":"any"}`, 200)
	if preview["matched"] != true || preview["target_platform"] != "anthropic" {
		t.Fatalf("unexpected composite route preview: %#v", preview)
	}
	admin.expect("GET", "/admin/channels", "", 200)

	startOAuth := admin.expect("POST", "/admin/accounts/oauth/start", `{"platform":"openai"}`, 200)
	if startOAuth["session_id"] != "sess-123" {
		t.Fatalf("unexpected oauth start: %#v", startOAuth)
	}
	completed := admin.expect("POST", "/admin/accounts/oauth/complete", `{"platform":"openai","session_id":"sess-123","state":"state-456","code":"auth-code-xyz","name":"ClosedLoop OAuth","group_ids":[8]}`, 201)
	if completed["credentials"] != nil || completed["name"] != "ClosedLoop OAuth" {
		t.Fatalf("oauth complete leaked credentials or wrong name: %#v", completed)
	}
	admin.expect("POST", "/admin/accounts/5/recover", `{}`, 200)

	admin.expect("POST", "/admin/users/2/balance", `{"balance":50,"operation":"add","notes":"topup"}`, 200)
	admin.expect("GET", "/admin/users/2/balance-history", "", 200)
	admin.expect("POST", "/admin/subscriptions/assign", `{"user_id":2,"group_id":12,"validity_days":30}`, 201)
	admin.expect("POST", "/admin/subscriptions/11/extend", `{"days":30}`, 200)

	codeList := admin.expect("GET", "/admin/redeem-codes", "", 200)["items"].([]any)[0].(map[string]any)
	if codeList["code"] != nil || !strings.HasPrefix(codeList["masked"].(string), "••••") {
		t.Fatalf("redeem code list failed to mask secret: %#v", codeList)
	}
	generated := admin.expect("POST", "/admin/redeem-codes/generate", `{"count":2,"type":"balance","value":10,"expires_in_days":30}`, 201)
	if len(generated["codes"].([]any)) != 2 {
		t.Fatalf("expected 2 generated codes: %#v", generated)
	}
	admin.expect("POST", "/admin/redeem-codes/create-and-redeem", `{"user_id":2,"type":"balance","value":15}`, 200)

	allSite := admin.expect("GET", "/admin/usage/summary?user_id=all&period=today", "", 200)
	if allSite["totals"].(map[string]any)["requests"].(float64) < 3 {
		t.Fatalf("expected all-site admin usage summary across users: %#v", allSite)
	}
	allSiteRecords := admin.expect("GET", "/admin/usage?user_id=all&period=today", "", 200)["items"].([]any)
	if len(allSiteRecords) < 3 || allSiteRecords[0].(map[string]any)["user_id"] == nil {
		t.Fatalf("expected admin usage records with entity IDs: %#v", allSiteRecords)
	}

	accPage := admin.expect("GET", "/admin/accounts", "", 200)["items"].([]any)
	accRow := accPage[0].(map[string]any)
	if accRow["today_requests"] != float64(19) || accRow["today_tokens"] != float64(48000) || accRow["today_cost"] != float64(1.45) {
		t.Fatalf("expected today-stats/batch fallback on account list: %#v", accRow)
	}
	batchQuota := admin.expect("POST", "/admin/accounts/quota/batch", `{"account_ids":[5,9],"force":false}`, 200)
	rawQuotaBytes, _ := json.Marshal(batchQuota)
	if strings.Contains(string(rawQuotaBytes), fixtureLeak) {
		t.Fatalf("batch quota leaked upstream error or secret: %s", rawQuotaBytes)
	}
	q5 := batchQuota["quotas"].(map[string]any)["5"].(map[string]any)
	if q5["five_hours"].(map[string]any)["percent_used"] != float64(32) || len(batchQuota["unavailable"].([]any)) != 1 {
		t.Fatalf("unexpected batch quota projection: %#v", batchQuota)
	}
	bal := admin.expect("POST", "/admin/accounts/5/balance", `{}`, 200)
	rawBalBytes, _ := json.Marshal(bal)
	if strings.Contains(string(rawBalBytes), fixtureLeak) || bal["balance"] != float64(88.5) || bal["currency"] != "CNY" {
		t.Fatalf("unexpected balance projection: %s", rawBalBytes)
	}
}
