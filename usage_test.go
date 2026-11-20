package main

import (
	"fmt"
	"math"
	"net/http"
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
}

func fixtureUsageRows() []fixtureUsageRow {
	zone, _ := loadUsageLocation("")
	now := time.Now().In(zone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
	rows := []fixtureUsageRow{
		{1000, 3, 9, "other-private-model", today, usageTokens{9000, 50, 15, 5}},
		{999, 2, 6, "claude-sonnet", today, usageTokens{4000, 500, 2200, 800}},
		{998, 2, 4, "gpt-5.2", today, usageTokens{100, 20, 80, 10}},
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
			usageTokens{int64(200 + index*31), int64(40 + index*3), int64(50 + index*25), 15}})
	}
	sort.SliceStable(rows, func(left, right int) bool { return rows[left].created.After(rows[right].created) })
	return rows
}

func (f *fixture) serveUsage(w http.ResponseWriter, r *http.Request, path string, user int64) {
	query := r.URL.Query()
	if path == "/admin/usage" {
		if user != 1 {
			w.WriteHeader(403)
			return
		}
		user, _ = strconv.ParseInt(query.Get("user_id"), 10, 64)
	}
	key, _ := strconv.ParseInt(query.Get("api_key_id"), 10, 64)
	if key != 0 {
		owned := false
		for _, candidate := range fixtureKeys(user) {
			owned = owned || candidate["id"] == key
		}
		if !owned {
			writeJSON(w, 403, map[string]any{"msg": fixtureLeak})
			return
		}
	}
	items := make([]any, 0)
	trends := map[string]usageTotals{}
	models := map[string]usageModel{}
	for _, row := range f.usageRows {
		day := row.created.Format(usageDateLayout)
		if row.user != user || key != 0 && key != row.key || query.Has("start_date") && day < query.Get("start_date") || query.Has("end_date") && day > query.Get("end_date") {
			continue
		}
		cost := float64(row.Input) / 10000
		if path != "/usage/dashboard/snapshot-v2" {
			var apiKey any
			for _, candidate := range fixtureKeys(user) {
				if candidate["id"] == row.key {
					apiKey = candidate
				}
			}
			items = append(items, struct {
				usageView
				CacheRead  int64  `json:"cache_read_tokens"`
				CacheWrite int64  `json:"cache_creation_tokens"`
				UserID     int64  `json:"user_id"`
				KeyID      int64  `json:"api_key_id"`
				Key        any    `json:"api_key"`
				Extra      string `json:"account_cost"`
			}{usageView{ID: row.id, Model: row.model, Created: row.created.UTC(), Input: row.Input, Output: row.Output, Cost: &cost},
				row.CacheRead, row.CacheWrite, row.user, row.key, apiKey, fixtureLeak})
			continue
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
	}
	if path != "/usage/dashboard/snapshot-v2" {
		page, _ := strconv.Atoi(query.Get("page"))
		start := min((page-1)*pageSize, len(items))
		end := min(start+pageSize, len(items))
		officialSuccess(w, map[string]any{"items": items[start:end], "pages": (len(items) + pageSize - 1) / pageSize, "extra": fixtureLeak})
		return
	}
	trendItems := make([]any, 0, len(trends))
	modelItems := make([]any, 0, len(models))
	for date, trend := range trends {
		trendItems = append(trendItems, struct {
			usageDay
			Requests int64   `json:"requests"`
			Cost     float64 `json:"actual_cost"`
			Extra    string  `json:"upstream_cost"`
		}{usageDay{date, trend.usageTokens}, trend.Requests, trend.Cost, fixtureLeak})
	}
	for _, model := range models {
		modelItems = append(modelItems, struct {
			usageModel
			Extra string `json:"account_cost"`
		}{model, fixtureLeak})
	}
	officialSuccess(w, map[string]any{"trend": trendItems, "models": modelItems, "extra": fixtureLeak})
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
	if len(summary) != 6 || len(totals) != 6 || totals["requests"] != float64(2) || totals["input_tokens"] != float64(4100) ||
		totals["output_tokens"] != float64(520) || totals["cache_read_tokens"] != float64(2280) || totals["cache_creation_tokens"] != float64(810) ||
		math.Abs(totals["actual_cost"].(float64)-.41) > 1e-9 {
		t.Fatal(summary)
	}
	filtered := user.expect("GET", "/usage/summary?api_key_id=4", "", 200)
	days := filtered["days"].([]any)
	models := filtered["models"].(map[string]any)["items"].([]any)
	if len(days) != 7 || len(models) != 2 || len(models[0].(map[string]any)) != 4 || filtered["end_date"] != day ||
		days[6].(map[string]any)["input_tokens"] != float64(100) || days[5].(map[string]any)["input_tokens"].(float64) <= 0 ||
		filtered["totals"].(map[string]any)["requests"] != float64(pageSize) {
		t.Fatal(filtered)
	}
	path := "/usage?api_key_id=4"
	page := user.expect("GET", path, "", 200)
	first := page["items"].([]any)[0].(map[string]any)
	if len(first) != 9 || first["key_name"] != "Demo" || first["cache_read_tokens"] != float64(80) || page["more"] != true {
		t.Fatal(page)
	}
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
