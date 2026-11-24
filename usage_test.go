package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"testing"
	"time"
)

func fixtureKeys(user int64) []map[string]any {
	key := func(id int64, name string) map[string]any {
		return map[string]any{"id": id, "name": name, "key": fixtureLeak + "abcd", "status": "active", "user_id": user, "secret": fixtureLeak}
	}
	switch user {
	case 2:
		return []map[string]any{key(4, "Demo"), key(6, "Automation"), key(8, "Unused")}
	case 3:
		return []map[string]any{key(9, "Other key")}
	default:
		return []map[string]any{key(10, "Admin key")}
	}
}

type fixtureUsageRow struct {
	id, user, key int64
	model         string
	created       time.Time
	usageTokens
	cost float64
}

func fixtureUsageRows() []fixtureUsageRow {
	zone, _ := loadUsageLocation("")
	now := time.Now().In(zone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
	rows := []fixtureUsageRow{
		{1001, 1, 10, "admin-personal-model", today, usageTokens{1000, 100, 40, 10}, 0},
		{1000, 3, 9, "other-private-model", today, usageTokens{9000, 50, 15, 5}, 0},
		{999, 2, 6, "claude-sonnet", today, usageTokens{4000, 500, 2200, 800}, 0},
		{998, 2, 4, "gpt-5.2", today, usageTokens{100, 20, 80, 10}, 0},
	}
	for index := range pageSize {
		day := 1 + index%6
		if index == pageSize-1 {
			day = 45
		}
		model := "claude-sonnet"
		if index%2 == 0 {
			model = "gpt-5.2"
		}
		rows = append(rows, fixtureUsageRow{int64(990 - index), 2, 4, model, today.AddDate(0, 0, -day),
			usageTokens{int64(200 + index*31), int64(40 + index*3), int64(50 + index*25), 15}, 0})
	}
	for i := range rows {
		rows[i].cost = float64(rows[i].Input) / 10000
	}
	sort.SliceStable(rows, func(left, right int) bool { return rows[left].created.After(rows[right].created) })
	return rows
}

func (f *fixture) serveUsage(w http.ResponseWriter, r *http.Request, path string, user int64) {
	query := r.URL.Query()
	if path == "/admin/usage" || path == "/admin/dashboard/snapshot-v2" {
		if user != 1 {
			w.WriteHeader(403)
			return
		}
		user, _ = strconv.ParseInt(query.Get("user_id"), 10, 64)
	}
	key, _ := strconv.ParseInt(query.Get("api_key_id"), 10, 64)
	group, _ := strconv.ParseInt(query.Get("group_id"), 10, 64)
	account, _ := strconv.ParseInt(query.Get("account_id"), 10, 64)
	keyUsers := []int64{user}
	if user == 0 {
		keyUsers = []int64{1, 2, 3}
		if f.previewKeys != nil {
			keyUsers = make([]int64, 0, len(f.previewKeys))
			for id := range f.previewKeys {
				keyUsers = append(keyUsers, id)
			}
		}
	}
	keys := make(map[int64]map[string]any)
	for _, uid := range keyUsers {
		for _, candidate := range f.keys(uid) {
			keys[candidate["id"].(int64)] = candidate
		}
	}
	if key != 0 {
		if _, owned := keys[key]; !owned {
			writeJSON(w, 403, map[string]any{"msg": fixtureLeak})
			return
		}
	}
	isSnapshot := path == "/usage/dashboard/snapshot-v2" || path == "/admin/dashboard/snapshot-v2"
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("page_size"))
	page = max(1, page)
	if size < 1 {
		size = pageSize
	}
	offset, count := (page-1)*size, 0
	items := make([]any, 0)
	trends := map[string]usageTotals{}
	models := map[string]usageModel{}
	groupTotals := map[namedItem]usageModel{}
	modelFilter := query.Get("model")
	startDate, endDate := query.Get("start_date"), query.Get("end_date")
	zone := f.usageRows[0].created.Location()
	rows := f.usageRows
	if end, err := time.ParseInLocation(usageDateLayout, endDate, zone); err == nil {
		exclusive := end.AddDate(0, 0, 1)
		rows = rows[sort.Search(len(rows), func(i int) bool { return rows[i].created.Before(exclusive) }):]
	}
	if start, err := time.ParseInLocation(usageDateLayout, startDate, zone); err == nil {
		rows = rows[:sort.Search(len(rows), func(i int) bool { return rows[i].created.Before(start) })]
	}
	for _, row := range rows {
		if user != 0 && row.user != user || key != 0 && key != row.key || modelFilter != "" && row.model != modelFilter {
			continue
		}
		cost := row.cost
		route := f.previewRoutes[row.model]
		if f.previewRoutes != nil && (group != 0 && route.group.ID != group || account != 0 && route.account.ID != account) {
			continue
		}
		if !isSnapshot {
			index := count
			count++
			if index < offset || len(items) == size {
				continue
			}
			apiKey := keys[row.key]
			var owner map[string]any
			if f.previewKeys != nil && path == "/admin/usage" {
				owner = map[string]any{"id": row.user, "email": previewEmail(row.user)}
			}
			var groupWire, accountWire *namedItem
			if route.group.ID != 0 {
				groupWire, accountWire = &route.group, &route.account
			}
			items = append(items, struct {
				usageView
				CacheRead  int64      `json:"cache_read_tokens"`
				CacheWrite int64      `json:"cache_creation_tokens"`
				UserID     int64      `json:"user_id"`
				KeyID      int64      `json:"api_key_id"`
				Key        any        `json:"api_key"`
				Owner      any        `json:"user,omitempty"`
				GroupID    int64      `json:"group_id,omitempty"`
				AccountID  int64      `json:"account_id,omitempty"`
				Group      *namedItem `json:"group,omitempty"`
				Account    *namedItem `json:"account,omitempty"`
				Extra      string     `json:"account_cost"`
			}{usageView{ID: row.id, Model: row.model, Created: row.created.UTC(), Input: row.Input, Output: row.Output, Cost: &cost},
				row.CacheRead, row.CacheWrite, row.user, row.key, apiKey, owner, route.group.ID, route.account.ID, groupWire, accountWire, fixtureLeak})
			continue
		}
		day := row.created.Format(usageDateLayout)
		if query.Get("granularity") == "hour" {
			day = row.created.Truncate(time.Hour).Format(usageHourLayout)
		}
		trend := trends[day]
		trend.Requests++
		trend.Input += row.Input
		trend.Output += row.Output
		trend.CacheRead += row.CacheRead
		trend.CacheWrite += row.CacheWrite
		trend.Cost += cost
		trends[day] = trend
		model := models[row.model]
		model.Model = row.model
		model.Requests++
		model.Tokens += row.Input + row.Output + row.CacheRead + row.CacheWrite
		model.Cost += cost
		models[row.model] = model
		if route.group.ID != 0 && query.Get("include_group_stats") == "true" {
			total := groupTotals[route.group]
			total.Requests++
			total.Tokens += row.Input + row.Output + row.CacheRead + row.CacheWrite
			total.Cost += cost
			groupTotals[route.group] = total
		}
	}
	if !isSnapshot {
		officialSuccess(w, map[string]any{"items": items, "pages": (count + size - 1) / size, "extra": fixtureLeak})
		return
	}
	trendItems := make([]any, 0, len(trends))
	modelItems := make([]any, 0, len(models))
	for date, trend := range trends {
		trendItems = append(trendItems, struct {
			usageDay
			Extra string `json:"upstream_cost"`
		}{usageDay{date, trend.usageTokens, trend.Requests, trend.Cost}, fixtureLeak})
	}
	for _, model := range models {
		modelItems = append(modelItems, struct {
			usageModel
			Extra string `json:"account_cost"`
		}{model, fixtureLeak})
	}
	if startDate == "" {
		startDate = time.Now().In(f.usageRows[0].created.Location()).Format(usageDateLayout)
	}
	if endDate == "" {
		endDate = time.Now().In(f.usageRows[0].created.Location()).Format(usageDateLayout)
	}
	groups := []any{}
	if f.preview == nil {
		groups = []any{
			map[string]any{"group_id": 8, "group_name": "Example", "requests": 24, "total_tokens": 68000, "actual_cost": 2.15},
			map[string]any{"group_id": 12, "group_name": "Pro-Composite", "requests": 16, "total_tokens": 41000, "actual_cost": 1.38},
		}
	}
	for group, total := range groupTotals {
		groups = append(groups, map[string]any{"group_id": group.ID, "group_name": group.Name,
			"requests": total.Requests, "total_tokens": total.Tokens, "actual_cost": total.Cost})
	}
	officialSuccess(w, map[string]any{
		"start_date": startDate, "end_date": endDate, "trend": trendItems, "models": modelItems,
		"groups": groups,
		"extra":  fixtureLeak,
	})
}

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
	day := date.Format(usageDateLayout)
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
		filtered["totals"].(map[string]any)["requests"] != float64(pageSize) {
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
			user.expect("GET", fmt.Sprintf("%s?start_date=%s&end_date=%s", path, date.AddDate(0, 0, 1-length).Format(usageDateLayout), day), "", status)
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
			officialSuccess(w, identity{ID: 2, Role: "user"})
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unavailable.Close()
	f.panel.upstream = newClient(unavailable.URL)
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
	ctx := context.WithValue(context.Background(), metaKey{}, callMeta{Trace: randomHex(16), Span: randomHex(8)})
	query, _ := url.ParseQuery(filters + "&user_id=1&actor=1")
	query.Set("admin", "true")
	scope := f.panel.dashboardScope(1, query, true)
	first, err := f.panel.buildUsageStreamSnapshot(ctx, "admin-token", query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	var summary usageSummary
	if err := json.Unmarshal(first, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.ModelSeries.Unavailable || summary.ModelSeries.ReadyDays != 1 || summary.ModelSeries.Buckets[0].Models[0].Model != "admin-personal-model" {
		t.Fatalf("observation reconciliation failed: %s", first)
	}
	version, err := f.panel.commitUsageSnapshot(ctx, scope, first)
	if err != nil || version != 1 {
		t.Fatalf("version %d: %v", version, err)
	}
	again, err := f.panel.commitUsageSnapshot(ctx, scope, first)
	if err != nil || again != version {
		t.Fatalf("unchanged snapshot advanced to %d: %v", again, err)
	}
	base, err := f.panel.resumeUsage(ctx, scope, scope+".1")
	if err != nil || base == nil || base.Totals.Requests != 1 {
		t.Fatalf("resume failed: %v", err)
	}
	foreign, err := f.panel.resumeUsage(ctx, "other-scope", scope+".1")
	if err != nil || foreign != nil {
		t.Fatal("foreign cursor accepted")
	}
	bucket := url.QueryEscape(summary.Buckets[0].Start.Format(time.RFC3339))
	page := admin.expect("GET", "/admin/usage?"+filters+"&bucket_start="+bucket, "", 200)
	if len(page["items"].([]any)) != 1 || page["items"].([]any)[0].(map[string]any)["model"] != "admin-personal-model" {
		t.Fatal(page)
	}
	query.Del("user_id")
	if ownScope := f.panel.dashboardScope(1, query, true); ownScope != scope {
		t.Fatal("implicit self must share its explicit scope")
	}
}

func TestHourlyWindowsHaveDistinctDSTOffsets(t *testing.T) {
	zone, err := loadUsageLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct {
		day   string
		count int
	}{{"2026-03-08", 23}, {"2026-11-01", 25}} {
		day, err := time.ParseInLocation(usageDateLayout, example.day, zone)
		if err != nil {
			t.Fatal(err)
		}
		windows := usageWindows(usageFilter{start: day, end: day, granularity: "hour"})
		if len(windows) != example.count {
			t.Fatalf("%s: %d", example.day, len(windows))
		}
		if example.count == 25 && (windows[1].Start.Hour() != windows[2].Start.Hour() || windows[1].Start.Equal(windows[2].Start)) {
			t.Fatal("repeated wall hour was collapsed")
		}
	}
}

func TestRepeatedDSTHourIsRebuiltFromObservations(t *testing.T) {
	zone, err := loadUsageLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	day, err := time.ParseInLocation(usageDateLayout, "2026-11-01", zone)
	if err != nil {
		t.Fatal(err)
	}
	requests, input, output, read, write := int64(1), int64(100), int64(20), int64(30), int64(10)
	cost := .01
	filter := usageFilter{start: day, end: day, granularity: "hour", timezone: zone.String()}
	snapshot := usageSnapshotWire{Trend: []usageTrendWire{
		{Date: "2026-11-01 01:00", Requests: &requests, Input: &input, Output: &output, CacheRead: &read, CacheWrite: &write, Cost: &cost},
		{Date: "2026-11-01 01:00", Requests: &requests, Input: &input, Output: &output, CacheRead: &read, CacheWrite: &write, Cost: &cost},
	}, Models: []struct {
		Model    string   `json:"model"`
		Requests *int64   `json:"requests"`
		Tokens   *int64   `json:"total_tokens"`
		Cost     *float64 `json:"actual_cost"`
	}{}}
	result, err := projectUsageSummary(filter, snapshot, 1)
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
