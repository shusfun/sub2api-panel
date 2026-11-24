package testkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/internal/server/observe"
)

const previewSampleVersion = "real-models-v1"

type previewModel struct {
	ID     string `json:"id"`
	Name   string `json:"display_name"`
	sample int
}

var previewModelCatalog = []struct {
	platform string
	name     string
	models   []previewModel
}{
	{"openai", "OpenAI", []previewModel{
		{"gpt-5.2", "GPT-5.2", 1}, {"gpt-5.4", "GPT-5.4", 3}, {"gpt-5.4-mini", "GPT-5.4 mini", 7},
		{"gpt-5.5", "GPT-5.5", 13}, {"gpt-5.2-pro", "GPT-5.2 pro", 14}, {"gpt-5.5-pro", "GPT-5.5 pro", 18},
	}},
	{"anthropic", "Anthropic", []previewModel{
		{"claude-sonnet-4-6", "Claude Sonnet 4.6", 2}, {"claude-opus-4-6", "Claude Opus 4.6", 8},
		{"claude-haiku-4-5-20251001", "Claude Haiku 4.5", 11}, {"claude-sonnet-4-5-20250929", "Claude Sonnet 4.5", 15},
	}},
	{"gemini", "Gemini", []previewModel{
		{"gemini-2.5-pro", "Gemini 2.5 Pro", 12}, {"gemini-2.5-flash", "Gemini 2.5 Flash", 6},
		{"gemini-3-flash-preview", "Gemini 3 Flash Preview", 16}, {"gemini-3.1-flash-lite", "Gemini 3.1 Flash-Lite", 17},
	}},
	{"grok", "Grok", []previewModel{{"grok-4.3", "Grok 4.3", 4}, {"grok-4.7", "Grok 4.7", 10}}},
	{"deepseek", "DeepSeek", []previewModel{{"deepseek-v4-pro", "DeepSeek V4 Pro", 9}, {"deepseek-flash", "DeepSeek Flash", 5}}},
}

func previewModels(platform string) []previewModel {
	models := []previewModel{}
	for _, provider := range previewModelCatalog {
		if platform == provider.platform || platform == "composite" {
			models = append(models, provider.models...)
		}
	}
	return models
}

func previewModelIDs(platform string) []string {
	models := previewModels(platform)
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func previewCatalogName(platform, kind string, index int) string {
	for _, provider := range previewModelCatalog {
		if platform == provider.platform {
			return fmt.Sprintf("%s %s %02d", provider.name, kind, index+1)
		}
	}
	if platform == "composite" {
		return fmt.Sprintf("复合路由 %02d", index+1)
	}
	return fmt.Sprintf("样本%s %02d", kind, index+1)
}

func (f *fixture) servePreviewModels(w http.ResponseWriter, r *http.Request, path string) bool {
	platformName := r.URL.Query().Get("platform")
	if path == "/admin/channels/pricing/sync-models" && r.Method == http.MethodGet {
		officialSuccess(w, map[string]any{"models": previewModelIDs(platformName)})
		return true
	}
	parent, remainder, _ := strings.Cut(strings.TrimPrefix(path, "/admin/"), "/")
	idText, endpoint, _ := strings.Cut(remainder, "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return false
	}
	switch parent {
	case "groups":
		index, exists := previewEntryIndex(id, previewGroupBase, f.preview.Groups)
		if endpoint == "model-allowlist-candidates" && r.Method == http.MethodGet {
			if !exists && id != 0 {
				w.WriteHeader(http.StatusNotFound)
				return true
			}
			if exists && platformName == "" {
				platformName = f.preview.Groups[index].Platform
			}
			officialSuccess(w, map[string]any{"models": previewModelIDs(platformName)})
			return true
		}
		if exists && f.preview.Groups[index].Platform == "composite" {
			return servePreviewComposite(w, r, id, endpoint)
		}
	case "accounts":
		if endpoint != "models" && endpoint != "models/sync-upstream" && endpoint != "test" {
			return false
		}
		index, ok := previewEntryIndex(id, previewAccountBase, f.preview.Accounts)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		platformName = f.preview.Accounts[index].Platform
		switch {
		case endpoint == "models" && r.Method == http.MethodGet:
			officialSuccess(w, previewModels(platformName))
			return true
		case endpoint == "models/sync-upstream" && r.Method == http.MethodPost:
			officialSuccess(w, map[string]any{"models": previewModelIDs(platformName)})
			return true
		case endpoint == "test" && r.Method == http.MethodPost:
			return servePreviewAccountTest(w, r, platformName)
		}
	}
	return false
}

func servePreviewAccountTest(w http.ResponseWriter, r *http.Request, platformName string) bool {
	var input struct {
		Model string `json:"model_id"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	if input.Model == "" {
		if models := previewModels(platformName); len(models) > 0 {
			input.Model = models[0].ID
		}
	}
	content, _ := json.Marshal(map[string]string{"type": "content", "text": "模拟上游响应 · " + input.Model})
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprintf(w, "data: %s\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n", content)
	return true
}

func servePreviewComposite(w http.ResponseWriter, r *http.Request, id int64, endpoint string) bool {
	if endpoint != "composite-routes" && endpoint != "composite-routes/preview" {
		return false
	}
	routes := make([]mockCompositeRoute, 0)
	for _, provider := range previewModelCatalog {
		for _, model := range provider.models {
			routes = append(routes, mockCompositeRoute{
				ID: int64(model.sample), GroupID: id, PublicModel: model.ID, MatchType: "exact",
				TargetPlatform: provider.platform, UpstreamModel: model.ID, Endpoint: "any", Priority: 10, Enabled: true,
			})
		}
	}
	if endpoint == "composite-routes" && r.Method == http.MethodGet {
		officialSuccess(w, routes)
		return true
	}
	if endpoint != "composite-routes/preview" || r.Method != http.MethodPost {
		return false
	}
	var input struct {
		Model    string `json:"model"`
		Endpoint string `json:"endpoint"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	for _, route := range routes {
		if route.PublicModel == input.Model {
			officialSuccess(w, map[string]any{
				"matched": true, "group_id": id, "public_model": input.Model, "endpoint": input.Endpoint,
				"source": "route", "target_platform": route.TargetPlatform, "upstream_model": route.UpstreamModel, "route": route,
			})
			return true
		}
	}
	officialSuccess(w, map[string]any{
		"matched": false, "group_id": id, "public_model": input.Model, "endpoint": input.Endpoint, "reason": "模拟目录没有该模型",
	})
	return true
}

func (f *fixture) previewAccountWindowStats(accountID int64, days int) (requests, tokens int64, cost float64) {
	index, ok := previewEntryIndex(accountID, previewAccountBase, f.preview.Accounts)
	if !ok || len(f.usageRows) == 0 {
		return 0, 0, 0
	}
	platformName := f.preview.Accounts[index].Platform
	samePlatform := make([]int64, 0, 4)
	for i, acc := range f.preview.Accounts {
		if acc.Platform == platformName {
			samePlatform = append(samePlatform, int64(previewAccountBase+i))
		}
	}
	slot := slices.Index(samePlatform, accountID)
	if slot < 0 {
		return 0, 0, 0
	}
	loc := f.usageRows[0].created.Location()
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(days - 1))
	models := previewModelIDs(platformName)
	for _, row := range f.usageRows {
		if row.created.Before(start) || !slices.Contains(models, row.model) {
			continue
		}
		if int(row.id)%len(samePlatform) != slot {
			continue
		}
		requests++
		tokens += row.Input + row.Output + row.CacheRead + row.CacheWrite
		cost += row.cost
	}
	return requests, tokens, cost
}

func (f *fixture) servePreviewAccountTelemetry(w http.ResponseWriter, r *http.Request, path string) bool {
	if path == "/admin/accounts/today-stats/batch" && r.Method == http.MethodPost {
		var in struct {
			AccountIDs []int64 `json:"account_ids"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		stats := make(map[string]any, len(in.AccountIDs))
		for _, id := range in.AccountIDs {
			req, tok, cost := f.previewAccountWindowStats(id, 1)
			stats[strconv.FormatInt(id, 10)] = map[string]any{
				"requests": req, "tokens": tok, "cost": cost, "user_cost": cost,
			}
		}
		officialSuccess(w, map[string]any{"stats": stats})
		return true
	}
	if path == "/admin/accounts/usage/batch" && r.Method == http.MethodPost {
		var in struct {
			AccountIDs []int64 `json:"account_ids"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		usage := map[string]any{}
		for _, id := range in.AccountIDs {
			idx, ok := previewEntryIndex(id, previewAccountBase, f.preview.Accounts)
			if !ok {
				continue
			}
			entry := f.preview.Accounts[idx]
			kind := previewAccountKind(entry, idx)
			if kind == "oauth" || kind == "setup-token" {
				usage[strconv.FormatInt(id, 10)] = map[string]any{
					"five_hour": map[string]any{"utilization": float64((idx*17 + 23) % 85), "resets_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)},
					"seven_day": map[string]any{"utilization": float64((idx*29 + 31) % 90), "resets_at": time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339)},
				}
			}
		}
		officialSuccess(w, map[string]any{"usage": usage, "errors": map[string]string{}})
		return true
	}
	if strings.HasPrefix(path, "/admin/cn-providers/accounts/") && strings.HasSuffix(path, "/balance") && r.Method == http.MethodGet {
		idText := strings.TrimSuffix(strings.TrimPrefix(path, "/admin/cn-providers/accounts/"), "/balance")
		id, err := strconv.ParseInt(idText, 10, 64)
		idx, ok := previewEntryIndex(id, previewAccountBase, f.preview.Accounts)
		if err != nil || !ok {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		if f.preview.Accounts[idx].Platform != "deepseek" {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		officialSuccess(w, map[string]any{
			"provider": "deepseek", "success": true, "balance": 128.50, "currency": "CNY", "available": true,
			"fetched_at": time.Now().UTC().Unix(),
			"balances": []map[string]any{
				{"currency": "CNY", "balance": 128.50},
				{"currency": "USD", "balance": 15.00},
			},
		})
		return true
	}
	if strings.HasPrefix(path, "/admin/accounts/") && r.Method == http.MethodGet {
		rest := strings.TrimPrefix(path, "/admin/accounts/")
		idText, action, hasSub := strings.Cut(rest, "/")
		id, err := strconv.ParseInt(idText, 10, 64)
		idx, ok := previewEntryIndex(id, previewAccountBase, f.preview.Accounts)
		if !hasSub || err != nil || !ok {
			return false
		}
		switch action {
		case "stats":
			days := 7
			if d, dErr := strconv.Atoi(r.URL.Query().Get("days")); dErr == nil && d >= 1 && d <= 90 {
				days = d
			}
			req, tok, cost := f.previewAccountWindowStats(id, days)
			officialSuccess(w, map[string]any{
				"summary": map[string]any{"total_requests": req, "total_tokens": tok, "total_cost": cost},
			})
			return true
		case "usage":
			entry := f.preview.Accounts[idx]
			kind := previewAccountKind(entry, idx)
			if kind == "oauth" || kind == "setup-token" {
				officialSuccess(w, map[string]any{
					"five_hour": map[string]any{"utilization": float64((idx*17 + 23) % 85), "resets_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)},
					"seven_day": map[string]any{"utilization": float64((idx*29 + 31) % 90), "resets_at": time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339)},
				})
			} else {
				officialSuccess(w, map[string]any{})
			}
			return true
		}
	}
	return false
}

func (f *fixture) previewCodes() []map[string]any {
	rows := make([]map[string]any, 0, len(f.preview.Codes))
	for i, entry := range f.preview.Codes {
		row := map[string]any{
			"id": int64(51 + i), "code": fmt.Sprintf("preview-code-%04d", i+1), "type": entry.Type, "status": entry.Status,
			"created_at": "2026-11-01T00:00:00Z", "value": 0,
		}
		if entry.Value != nil {
			row["value"] = *entry.Value
		}
		rows = append(rows, row)
	}
	return rows
}

func (f *fixture) previewDashboard() mockAdminDashboardStats {
	result := mockAdminDashboardStats{
		Users: int64(len(f.previewKeys)), ActiveUsers: int64(len(f.previewKeys)), Accounts: int64(len(f.preview.Accounts)),
	}
	for _, keys := range f.previewKeys {
		result.Keys += int64(len(keys))
		for _, key := range keys {
			if key["status"] == "active" {
				result.ActiveKeys++
			}
		}
	}
	for _, account := range f.preview.Accounts {
		if account.Status == "active" {
			result.NormalAccounts++
		}
		if account.Status == "error" {
			result.ErrorAccounts++
		}
	}
	today := time.Now().In(f.usageRows[0].created.Location()).Format(observe.UsageDateLayout)
	for _, row := range f.usageRows {
		tokens := row.Input + row.Output + row.CacheRead + row.CacheWrite
		result.TotalRequests++
		result.TotalTokens += tokens
		result.TotalCost += row.cost
		if row.created.Format(observe.UsageDateLayout) == today {
			result.TodayRequests++
			result.TodayTokens += tokens
			result.TodayCost += row.cost
		}
	}
	return result
}

func (f *fixture) previewRanking(r *http.Request) []mockOverviewUserRank {
	start, end := r.URL.Query().Get("start_date"), r.URL.Query().Get("end_date")
	ranked := make(map[int64]mockOverviewUserRank)
	for _, row := range f.usageRows {
		date := row.created.Format(observe.UsageDateLayout)
		if date < start || end != "" && date > end {
			continue
		}
		rank := ranked[row.user]
		rank.UserID, rank.Email = row.user, previewEmail(row.user)
		rank.Requests++
		rank.Tokens += row.Input + row.Output + row.CacheRead + row.CacheWrite
		rank.Cost += row.cost
		ranked[row.user] = rank
	}
	rows := make([]mockOverviewUserRank, 0, len(ranked))
	for _, row := range ranked {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cost == rows[j].Cost {
			return rows[i].UserID < rows[j].UserID
		}
		return rows[i].Cost > rows[j].Cost
	})
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}
