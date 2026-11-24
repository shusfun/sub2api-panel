package testkit

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/observe"
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

func fixtureUsageRows() []fixtureUsageRow {
	zone, _ := core.LoadUsageLocation("")
	now := time.Now().In(zone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
	rows := []fixtureUsageRow{
		{1001, 1, 10, "admin-personal-model", today, observe.UsageTokens{Input: 1000, Output: 100, CacheRead: 40, CacheWrite: 10}, 0},
		{1000, 3, 9, "other-private-model", today, observe.UsageTokens{Input: 9000, Output: 50, CacheRead: 15, CacheWrite: 5}, 0},
		{999, 2, 6, "claude-sonnet", today, observe.UsageTokens{Input: 4000, Output: 500, CacheRead: 2200, CacheWrite: 800}, 0},
		{998, 2, 4, "gpt-5.2", today, observe.UsageTokens{Input: 100, Output: 20, CacheRead: 80, CacheWrite: 10}, 0},
	}
	for index := range platform.PageSize {
		day := 1 + index%6
		if index == platform.PageSize-1 {
			day = 45
		}
		model := "claude-sonnet"
		if index%2 == 0 {
			model = "gpt-5.2"
		}
		rows = append(rows, fixtureUsageRow{int64(990 - index), 2, 4, model, today.AddDate(0, 0, -day),
			observe.UsageTokens{Input: int64(200 + index*31), Output: int64(40 + index*3), CacheRead: int64(50 + index*25), CacheWrite: 15}, 0})
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
			platform.WriteJSON(w, 403, map[string]any{"msg": fixtureLeak})
			return
		}
	}
	isSnapshot := path == "/usage/dashboard/snapshot-v2" || path == "/admin/dashboard/snapshot-v2"
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("page_size"))
	page = max(1, page)
	if size < 1 {
		size = platform.PageSize
	}
	offset, count := (page-1)*size, 0
	items := make([]any, 0)
	trends := map[string]observe.UsageTotals{}
	models := map[string]observe.UsageModel{}
	groupTotals := map[platform.NamedItem]observe.UsageModel{}
	modelFilter := query.Get("model")
	startDate, endDate := query.Get("start_date"), query.Get("end_date")
	zone := f.usageRows[0].created.Location()
	rows := f.usageRows
	if end, err := time.ParseInLocation(observe.UsageDateLayout, endDate, zone); err == nil {
		exclusive := end.AddDate(0, 0, 1)
		rows = rows[sort.Search(len(rows), func(i int) bool { return rows[i].created.Before(exclusive) }):]
	}
	if start, err := time.ParseInLocation(observe.UsageDateLayout, startDate, zone); err == nil {
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
			var groupWire, accountWire *platform.NamedItem
			if route.group.ID != 0 {
				groupWire, accountWire = &route.group, &route.account
			}
			items = append(items, struct {
				observe.UsageView
				CacheRead  int64               `json:"cache_read_tokens"`
				CacheWrite int64               `json:"cache_creation_tokens"`
				UserID     int64               `json:"user_id"`
				KeyID      int64               `json:"api_key_id"`
				Key        any                 `json:"api_key"`
				Owner      any                 `json:"user,omitempty"`
				GroupID    int64               `json:"group_id,omitempty"`
				AccountID  int64               `json:"account_id,omitempty"`
				Group      *platform.NamedItem `json:"group,omitempty"`
				Account    *platform.NamedItem `json:"account,omitempty"`
				Extra      string              `json:"account_cost"`
			}{observe.UsageView{ID: row.id, Model: row.model, Created: row.created.UTC(), Input: row.Input, Output: row.Output, Cost: &cost},
				row.CacheRead, row.CacheWrite, row.user, row.key, apiKey, owner, route.group.ID, route.account.ID, groupWire, accountWire, fixtureLeak})
			continue
		}
		day := row.created.Format(observe.UsageDateLayout)
		if query.Get("granularity") == "hour" {
			day = row.created.Truncate(time.Hour).Format(observe.UsageHourLayout)
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
			observe.UsageDay
			Extra string `json:"upstream_cost"`
		}{observe.UsageDay{Date: date, UsageTokens: trend.UsageTokens, Requests: trend.Requests, Cost: trend.Cost}, fixtureLeak})
	}
	for _, model := range models {
		modelItems = append(modelItems, struct {
			observe.UsageModel
			Extra string `json:"account_cost"`
		}{model, fixtureLeak})
	}
	if startDate == "" {
		startDate = time.Now().In(f.usageRows[0].created.Location()).Format(observe.UsageDateLayout)
	}
	if endDate == "" {
		endDate = time.Now().In(f.usageRows[0].created.Location()).Format(observe.UsageDateLayout)
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
