package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/observe"
)

func TestUsageDashboard(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	user := browserLogin(t, f.app, "user")
	other := browserLogin(t, f.app, "other")
	admin := browserLogin(t, f.app, "admin")
	keys := user.expect("GET", "/usage/keys", "", 200)["items"].([]any)
	if len(keys) != 3 || len(keys[0].(map[string]any)) != 2 {
		t.Fatal("expected the user's Key IDs and names")
	}
	date := f.usageRows[0].created
	day := date.Format(observe.UsageDateLayout)
	summary := user.expect("GET", "/usage/summary?period=today", "", 200)
	totals := summary["totals"].(map[string]any)
	if len(summary) != 10 || len(totals) != 6 || totals["requests"] != float64(2) || totals["input_tokens"] != float64(4100) ||
		totals["output_tokens"] != float64(520) || totals["cache_read_tokens"] != float64(2280) || totals["cache_creation_tokens"] != float64(810) ||
		math.Abs(totals["actual_cost"].(float64)-.41) > 1e-9 {
		t.Fatal(summary)
	}
	filtered := user.expect("GET", "/usage/summary?api_key_id=4", "", 200)
	days := filtered["days"].([]any)
	models := filtered["models"].(map[string]any)["items"].([]any)
	lastDay := days[6].(map[string]any)
	if len(days) != 7 || len(models) != 2 || len(models[0].(map[string]any)) != 4 || filtered["end_date"] != day ||
		lastDay["input_tokens"] != float64(100) || lastDay["requests"] != float64(1) || math.Abs(lastDay["actual_cost"].(float64)-0.01) > 1e-9 ||
		days[5].(map[string]any)["input_tokens"].(float64) <= 0 ||
		filtered["totals"].(map[string]any)["requests"] != float64(platform.PageSize) {
		t.Fatal(filtered)
	}
	if fourteen := user.expect("GET", "/usage/summary?period=14d", "", 200); len(fourteen["days"].([]any)) != 14 {
		t.Fatal(fourteen)
	}
	if ninety := user.expect("GET", "/usage/summary?period=90d", "", 200); len(ninety["days"].([]any)) != 90 {
		t.Fatal(ninety)
	}
	path := "/usage?api_key_id=4"
	page := user.expect("GET", path, "", 200)
	first := page["items"].([]any)[0].(map[string]any)
	if len(first) != 9 || first["key_name"] != "Demo" || first["cache_read_tokens"] != float64(80) || page["more"] != true || page["pages"] != float64(2) {
		t.Fatal(page)
	}
	byModel := user.expect("GET", path+"&model=claude-sonnet", "", 200)
	for _, entry := range byModel["items"].([]any) {
		if entry.(map[string]any)["model"] != "claude-sonnet" {
			t.Fatal(byModel)
		}
	}
	user.expect("GET", "/usage/summary?model=claude-sonnet", "", 400)
	user.expect("GET", "/usage?model=", "", 400)
	next := user.expect("GET", path+"&page=2", "", 200)
	if len(next["items"].([]any)) != 1 || next["more"] != false {
		t.Fatal(next)
	}
	if latest := user.expect("GET", path+"&period=today", "", 200); len(latest["items"].([]any)) != 1 || latest["more"] != false {
		t.Fatal(latest)
	}
	empty := user.expect("GET", "/usage/summary?api_key_id=8", "", 200)
	if empty["totals"].(map[string]any)["requests"] != float64(0) || len(empty["models"].(map[string]any)["items"].([]any)) != 0 {
		t.Fatal(empty)
	}
	for _, route := range []string{"/usage", "/usage/summary"} {
		user.expect("GET", route+"?api_key_id=9", "", 403)
		other.expect("GET", route+"?api_key_id=4", "", 403)
		admin.expect("GET", route+"?api_key_id=4", "", 403)
		user.expect("GET", route+"?api_key_id=0", "", 400)
	}
	for _, path := range []string{"/usage", "/usage/summary"} {
		for _, length := range []int{90, 91} {
			status := 200
			if length == 91 {
				status = 400
			}
			user.expect("GET", fmt.Sprintf("%s?start_date=%s&end_date=%s", path, date.AddDate(0, 0, 1-length).Format(observe.UsageDateLayout), day), "", status)
		}
		user.expect("GET", path+"?start_date=2026-02-29&end_date="+day, "", 400)
	}
	user.expect("GET", "/admin/users/3/usage", "", 403)
	if rows := admin.expect("GET", "/admin/users/3/usage", "", 200)["items"].([]any); len(rows) != 1 || rows[0].(map[string]any)["model"] != "other-private-model" {
		t.Fatal(rows)
	}
}

func TestUsageSummaryUpstreamUnavailableIncludesRequestID(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	user := browserLogin(t, f.app, "user")
	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/me" {
			officialSuccess(w, platform.Identity{ID: 2, Role: "user"})
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unavailable.Close()
	f.panel.Upstream = platform.NewClient(unavailable.URL)
	response := user.request("GET", "/usage/summary?period=7d", "")
	checkResponse(t, response, http.StatusBadGateway)
	body := object(t, response.Body.Bytes())
	if body["code"] != "service_unavailable" || body["hint"] != "服务暂不可用，请稍后再试。" ||
		body["request_id"] != response.Header().Get("X-Request-ID") {
		t.Fatalf("upstream failure was not projected with its request ID: %#v", body)
	}
}

func TestHourlyUsageAndPostgresResume(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	f.usageRows[0].created = f.usageRows[0].created.Add(17 * time.Minute)
	admin := browserLogin(t, f.app, "admin")
	user := browserLogin(t, f.app, "user")
	const filters = "period=today&granularity=hour&analysis=models"
	own := admin.expect("GET", "/admin/usage/summary?"+filters, "", 200)
	if own["totals"].(map[string]any)["requests"] != float64(1) || len(own["buckets"].([]any)) != 24 ||
		own["ranking"].([]any)[0].(map[string]any)["model"] != "admin-personal-model" {
		t.Fatal(own)
	}
	if first := own["buckets"].([]any)[0].(map[string]any); first["requests"] != float64(1) {
		t.Fatalf("request at minute 17 was not counted in the first hourly bucket: %v", first)
	}
	keys := admin.expect("GET", "/admin/usage/keys", "", 200)["items"].([]any)
	if len(keys) != 1 || keys[0].(map[string]any)["id"] != float64(10) {
		t.Fatal(keys)
	}
	admin.expect("GET", "/admin/usage/summary?"+filters+"&user_id=2&api_key_id=10", "", 403)
	admin.expect("GET", "/admin/usage?user_id=2&api_key_id=10", "", 403)
	if rows := admin.expect("GET", "/admin/usage?api_key_id=10", "", 200)["items"].([]any); len(rows) != 1 {
		t.Fatalf("expected the administrator's own Key record, got %d", len(rows))
	}
	user.expect("GET", "/usage/summary?"+filters+"&user_id=3", "", 400)
	user.expect("GET", "/usage/summary?granularity=hour&period=14d", "", 400)
	ctx := platform.WithMeta(context.Background(), platform.CallMeta{Trace: platform.RandomHex(16), Span: platform.RandomHex(8)})
	query, _ := url.ParseQuery(filters + "&user_id=1&actor=1")
	query.Set("admin", "true")
	scope := observe.DashboardScope(f.panel, 1, query, true)
	build := observe.BuildUsageStreamSnapshot(f.panel)
	first, err := build(ctx, "admin-token", query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	var summary observe.UsageSummary
	if err := json.Unmarshal(first, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.ModelSeries.Unavailable || summary.ModelSeries.ReadyDays != 1 || summary.ModelSeries.Buckets[0].Models[0].Model != "admin-personal-model" {
		t.Fatalf("observation reconciliation failed: %s", first)
	}
	commit := observe.CommitUsageSnapshot(f.panel)
	version, err := commit(ctx, scope, first)
	if err != nil || version != 1 {
		t.Fatalf("version %d: %v", version, err)
	}
	again, err := commit(ctx, scope, first)
	if err != nil || again != version {
		t.Fatalf("unchanged snapshot advanced to %d: %v", again, err)
	}
	base, err := observe.ResumeUsage(ctx, f.panel, scope, scope+".1")
	if err != nil || base == nil || base.Totals.Requests != 1 {
		t.Fatalf("resume failed: %v", err)
	}
	foreign, err := observe.ResumeUsage(ctx, f.panel, "other-scope", scope+".1")
	if err != nil || foreign != nil {
		t.Fatal("foreign cursor accepted")
	}
	bucket := url.QueryEscape(summary.Buckets[0].Start.Format(time.RFC3339))
	page := admin.expect("GET", "/admin/usage?"+filters+"&bucket_start="+bucket, "", 200)
	if len(page["items"].([]any)) != 1 || page["items"].([]any)[0].(map[string]any)["model"] != "admin-personal-model" {
		t.Fatal(page)
	}
	query.Del("user_id")
	if ownScope := observe.DashboardScope(f.panel, 1, query, true); ownScope != scope {
		t.Fatal("implicit self must share its explicit scope")
	}
}

func TestHourlyWindowsHaveDistinctDSTOffsets(t *testing.T) {
	zone, err := core.LoadUsageLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct {
		day   string
		count int
	}{{"2026-03-08", 23}, {"2026-11-01", 25}} {
		day, err := time.ParseInLocation(observe.UsageDateLayout, example.day, zone)
		if err != nil {
			t.Fatal(err)
		}
		windows := observe.UsageWindows(observe.UsageFilter{Start: day, End: day, Granularity: "hour"})
		if len(windows) != example.count {
			t.Fatalf("%s: %d", example.day, len(windows))
		}
		if example.count == 25 && (windows[1].Start.Hour() != windows[2].Start.Hour() || windows[1].Start.Equal(windows[2].Start)) {
			t.Fatal("repeated wall hour was collapsed")
		}
	}
}

func TestRepeatedDSTHourIsRebuiltFromObservations(t *testing.T) {
	zone, err := core.LoadUsageLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	day, err := time.ParseInLocation(observe.UsageDateLayout, "2026-11-01", zone)
	if err != nil {
		t.Fatal(err)
	}
	requests, input, output, read, write := int64(1), int64(100), int64(20), int64(30), int64(10)
	cost := .01
	filter := observe.UsageFilter{Start: day, End: day, Granularity: "hour", Timezone: zone.String()}
	snapshot := observe.UsageSnapshotWire{
		Trend: []observe.UsageTrendWire{
			{Date: "2026-11-01 01:00", Requests: &requests, Input: &input, Output: &output, CacheRead: &read, CacheWrite: &write, Cost: &cost},
			{Date: "2026-11-01 01:00", Requests: &requests, Input: &input, Output: &output, CacheRead: &read, CacheWrite: &write, Cost: &cost},
		},
		Models: []observe.UsageModelWire{},
	}
	result, err := observe.ProjectUsageSummary(filter, snapshot, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Totals.Requests != 2 {
		t.Fatalf("folded hour totals were lost: %+v", result.Totals)
	}
	indices := []int{}
	for i, bucket := range result.Buckets {
		if bucket.Start.Hour() == 1 {
			indices = append(indices, i)
			if !bucket.Pending {
				t.Fatal("repeated wall hour needs observation reconstruction")
			}
		}
	}
	if len(indices) != 2 || result.Buckets[indices[0]].Start.Equal(result.Buckets[indices[1]].Start) {
		t.Fatal("the repeated DST hour did not keep two distinct instants")
	}
}
