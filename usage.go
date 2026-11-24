package main

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const usageDateLayout = "2006-01-02"
const maxUsageDays = 90

func loadUsageLocation(name string) (*time.Location, error) {
	if name == "" {
		name = "Asia/Shanghai"
	}
	return time.LoadLocation(name)
}

func (p *panel) usageZone() *time.Location {
	if p.usageLocation != nil {
		return p.usageLocation
	}
	location, _ := loadUsageLocation("")
	return location
}

type usageFilter struct {
	key         int64
	group       int64
	account     int64
	granularity string
	period      string
	model       string
	start, end  time.Time
	timezone    string
}

func parseUsageFilter(query url.Values, now time.Time, summary bool) (usageFilter, error) {
	filter := usageFilter{timezone: now.Location().String(), granularity: "day"}
	if query.Has("granularity") {
		filter.granularity = query.Get("granularity")
		if len(query["granularity"]) != 1 || filter.granularity != "hour" && filter.granularity != "day" {
			return filter, fail(400, "invalid_input")
		}
	}
	if query.Has("api_key_id") {
		id, err := strconv.ParseInt(query.Get("api_key_id"), 10, 64)
		if err != nil || id < 1 || len(query["api_key_id"]) != 1 {
			return filter, fail(400, "invalid_input")
		}
		filter.key = id
	}
	if query.Has("group_id") {
		id, err := strconv.ParseInt(query.Get("group_id"), 10, 64)
		if err != nil || id < 1 || len(query["group_id"]) != 1 {
			return filter, fail(400, "invalid_input")
		}
		filter.group = id
	}
	if query.Has("account_id") {
		id, err := strconv.ParseInt(query.Get("account_id"), 10, 64)
		if err != nil || id < 1 || len(query["account_id"]) != 1 {
			return filter, fail(400, "invalid_input")
		}
		filter.account = id
	}
	if query.Has("model") {
		model := strings.TrimSpace(query.Get("model"))
		if summary || model == "" || len(model) > 128 || len(query["model"]) != 1 {
			return filter, fail(400, "invalid_input")
		}
		filter.model = model
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if query.Has("start_date") || query.Has("end_date") {
		start, startErr := time.ParseInLocation(usageDateLayout, query.Get("start_date"), now.Location())
		end, endErr := time.ParseInLocation(usageDateLayout, query.Get("end_date"), now.Location())
		if startErr != nil || endErr != nil || start.Year() < 1 || query.Has("period") || end.Before(start) || end.After(today) ||
			end.After(start.AddDate(0, 0, maxUsageDays-1)) || len(query["start_date"]) != 1 || len(query["end_date"]) != 1 {
			return filter, fail(400, "invalid_usage_range")
		}
		filter.start, filter.end = start, end
	} else if summary || query.Has("period") {
		period := query.Get("period")
		filter.period = period
		if period == "" {
			filter.period = "7d"
		}
		days := map[string]int{"": 7, "today": 1, "7d": 7, "14d": 14, "30d": 30, "90d": 90}[period]
		if days == 0 || len(query["period"]) > 1 {
			return filter, fail(400, "invalid_usage_range")
		}
		filter.start, filter.end = today.AddDate(0, 0, 1-days), today
	}
	if filter.granularity == "hour" && !filter.start.IsZero() && filter.end.After(filter.start.AddDate(0, 0, 6)) {
		return filter, fail(400, "invalid_hour_range")
	}
	return filter, nil
}

func (filter usageFilter) query() url.Values {
	query := url.Values{"timezone": {filter.timezone}}
	if filter.key != 0 {
		query.Set("api_key_id", strconv.FormatInt(filter.key, 10))
	}
	if filter.group != 0 {
		query.Set("group_id", strconv.FormatInt(filter.group, 10))
	}
	if filter.account != 0 {
		query.Set("account_id", strconv.FormatInt(filter.account, 10))
	}
	if filter.model != "" {
		query.Set("model", filter.model)
	}
	if !filter.start.IsZero() {
		query.Set("start_date", filter.start.Format(usageDateLayout))
		query.Set("end_date", filter.end.Format(usageDateLayout))
	}
	return query
}

type usageTokens struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheRead  int64 `json:"cache_read_tokens"`
	CacheWrite int64 `json:"cache_creation_tokens"`
}

func (tokens usageTokens) valid() bool {
	return tokens.Input >= 0 && tokens.Output >= 0 && tokens.CacheRead >= 0 && tokens.CacheWrite >= 0
}

type usageRecordWire struct {
	usageView
	CacheRead    *int64     `json:"cache_read_tokens"`
	CacheWrite   *int64     `json:"cache_creation_tokens"`
	User         int64      `json:"user_id"`
	KeyID        int64      `json:"api_key_id"`
	GroupID      *int64     `json:"group_id"`
	AccountID    *int64     `json:"account_id"`
	DurationMs   *int64     `json:"duration_ms"`
	FirstTokenMs *int64     `json:"first_token_ms"`
	Stream       *bool      `json:"stream"`
	Key          *namedItem `json:"api_key"`
	Group        *namedItem `json:"group"`
	Account      *namedItem `json:"account"`
	UserObj      *struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

type usageRecord struct {
	usageView
	CacheRead    *int64 `json:"cache_read_tokens"`
	CacheWrite   *int64 `json:"cache_creation_tokens"`
	KeyName      string `json:"key_name"`
	KeyID        int64  `json:"api_key_id,omitempty"`
	UserID       int64  `json:"user_id,omitempty"`
	UserEmail    string `json:"user_email,omitempty"`
	GroupID      *int64 `json:"group_id,omitempty"`
	GroupName    string `json:"group_name,omitempty"`
	AccountID    *int64 `json:"account_id,omitempty"`
	AccountName  string `json:"account_name,omitempty"`
	DurationMs   *int64 `json:"duration_ms,omitempty"`
	FirstTokenMs *int64 `json:"first_token_ms,omitempty"`
	Stream       *bool  `json:"stream,omitempty"`
}

func (p *panel) usageKeys(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/keys", page, nil, func(key keyWire) (namedItem, error) {
		if key.Owner != a.user.ID || key.Name == "" {
			return namedItem{}, fail(502, "invalid_upstream")
		}
		return key.namedItem, requiredID(key.ID)
	})
	return send(w, 200, rows, err)
}

type keyUsageStats struct {
	Today float64 `json:"today_actual_cost"`
	Total float64 `json:"total_actual_cost"`
}

func (p *panel) withKeyUsage(ctx context.Context, token string, keys []keyView, admin bool) []keyView {
	if len(keys) == 0 {
		return keys
	}
	ids := make([]int64, 0, len(keys))
	for _, key := range keys {
		ids = append(ids, key.ID)
	}
	path := "/usage/dashboard/api-keys-usage"
	if admin {
		path = "/admin/dashboard/api-keys-usage"
	}
	result, err := upstream[struct {
		Stats map[string]keyUsageStats `json:"stats"`
	}](ctx, p.upstream, "POST", path, token, "", "", struct {
		IDs []int64 `json:"api_key_ids"`
	}{ids})
	if err != nil {
		return keys
	}
	for index := range keys {
		if stats, ok := result.Stats[strconv.FormatInt(keys[index].ID, 10)]; ok && stats.Today >= 0 && stats.Total >= 0 {
			today, total := stats.Today, stats.Total
			keys[index].TodayCost, keys[index].TotalCost = &today, &total
		}
	}
	return keys
}

func (p *panel) keyUsageBatch(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		IDs []int64 `json:"api_key_ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 100 {
		return fail(400, "invalid_input")
	}
	seen := make(map[int64]struct{}, len(input.IDs))
	for _, id := range input.IDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
		if _, exists := seen[id]; exists {
			return fail(400, "invalid_input")
		}
		seen[id] = struct{}{}
	}
	result, err := upstream[struct {
		Stats map[string]keyUsageStats `json:"stats"`
	}](r.Context(), p.upstream, "POST", "/usage/dashboard/api-keys-usage", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	for keyID, stats := range result.Stats {
		parsed, parseErr := strconv.ParseInt(keyID, 10, 64)
		if parseErr != nil || parsed < 1 || stats.Today < 0 || stats.Total < 0 {
			return fail(502, "invalid_upstream")
		}
		if _, requested := seen[parsed]; !requested {
			return fail(502, "invalid_upstream")
		}
	}
	if result.Stats == nil {
		result.Stats = map[string]keyUsageStats{}
	}
	return send(w, 200, result, nil)
}

func (p *panel) usage(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	return p.dashboardUsageRecords(w, r, a, a.user.ID, false)
}

type usageDay struct {
	Date string `json:"date"`
	usageTokens
	Requests int64   `json:"requests"`
	Cost     float64 `json:"actual_cost"`
}

type usageBucket struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Pending bool      `json:"pending,omitempty"`
	usageTotals
}

type usageTotals struct {
	usageTokens
	Requests int64   `json:"requests"`
	Cost     float64 `json:"actual_cost"`
}

type usageTrendWire struct {
	Date       string   `json:"date"`
	Input      *int64   `json:"input_tokens"`
	Output     *int64   `json:"output_tokens"`
	CacheRead  *int64   `json:"cache_read_tokens"`
	CacheWrite *int64   `json:"cache_creation_tokens"`
	Requests   *int64   `json:"requests"`
	Cost       *float64 `json:"actual_cost"`
}

type usageModel struct {
	Model    string  `json:"model"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"total_tokens"`
	Cost     float64 `json:"actual_cost"`
}

type usageSnapshotWire struct {
	Trend  []usageTrendWire `json:"trend"`
	Models []struct {
		Model    string   `json:"model"`
		Requests *int64   `json:"requests"`
		Tokens   *int64   `json:"total_tokens"`
		Cost     *float64 `json:"actual_cost"`
	} `json:"models"`
}

type usageSummary struct {
	Timezone    string                 `json:"timezone"`
	Start       string                 `json:"start_date"`
	End         string                 `json:"end_date"`
	Totals      usageTotals            `json:"totals"`
	Days        []usageDay             `json:"days"`
	Models      pageResult[usageModel] `json:"models"`
	Granularity string                 `json:"granularity"`
	Buckets     []usageBucket          `json:"buckets"`
	Ranking     []usageModel           `json:"ranking"`
	ModelSeries usageModelSeries       `json:"model_series"`
	expected    map[string]usageTotals
}

func (p *panel) usageSummary(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if r.URL.Query().Has("user_id") || r.URL.Query().Has("group_id") || r.URL.Query().Has("account_id") {
		return fail(400, "invalid_input")
	}
	filter, err := parseUsageFilter(r.URL.Query(), time.Now().In(p.usageZone()), true)
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	result, err := p.fetchSelfUsageSummary(r.Context(), a.tokens.Access, filter, page)
	if err == nil {
		err = p.readUsageModelSeries(r.Context(), a.user.ID, false, r.URL.Query(), &result)
	}
	return send(w, 200, result, err)
}

func (p *panel) fetchSelfUsageSummary(ctx context.Context, token string, filter usageFilter, page int) (usageSummary, error) {
	query := filter.query()
	query.Set("granularity", filter.granularity)
	query.Set("include_group_stats", "false")
	snapshot, err := upstream[usageSnapshotWire](ctx, p.upstream, "GET", "/usage/dashboard/snapshot-v2?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return usageSummary{}, err
	}
	return projectUsageSummary(filter, snapshot, page)
}

type adminUsageSnapshotWire struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Trend     []struct {
		Date       string  `json:"date"`
		Requests   int64   `json:"requests"`
		Input      int64   `json:"input_tokens"`
		Output     int64   `json:"output_tokens"`
		CacheRead  int64   `json:"cache_read_tokens"`
		CacheWrite int64   `json:"cache_creation_tokens"`
		ActualCost float64 `json:"actual_cost"`
	} `json:"trend"`
	Models []struct {
		Model      string  `json:"model"`
		Requests   int64   `json:"requests"`
		Tokens     int64   `json:"total_tokens"`
		ActualCost float64 `json:"actual_cost"`
	} `json:"models"`
}

func (p *panel) buildAdminUsageSummary(ctx context.Context, token string, query url.Values, page int) (usageSummary, error) {
	filter, err := parseUsageFilter(query, time.Now().In(p.usageZone()), !query.Has("model") || !query.Has("start_date") && !query.Has("period"))
	if err != nil {
		return usageSummary{}, err
	}
	var userID int64
	if rawUser := query.Get("user_id"); rawUser != "all" {
		userID, err = strconv.ParseInt(rawUser, 10, 64)
		if err != nil || userID < 1 || len(query["user_id"]) != 1 {
			return usageSummary{}, fail(400, "invalid_input")
		}
	} else if len(query["user_id"]) != 1 {
		return usageSummary{}, fail(400, "invalid_input")
	}
	upstreamQuery := filter.query()
	if userID > 0 {
		upstreamQuery.Set("user_id", strconv.FormatInt(userID, 10))
	}
	upstreamQuery.Set("granularity", filter.granularity)
	upstreamQuery.Set("include_stats", "false")
	upstreamQuery.Set("include_trend", "true")
	upstreamQuery.Set("include_model_stats", "true")
	upstreamQuery.Set("include_group_stats", "false")
	snapshot, err := upstream[adminUsageSnapshotWire](ctx, p.upstream, "GET", "/admin/dashboard/snapshot-v2?"+upstreamQuery.Encode(), token, "", "", nil)
	if err != nil {
		return usageSummary{}, err
	}
	end := filter.end.Format(usageDateLayout)
	// Sub2API v0.2.23 subtracts 24 hours from the exclusive end when echoing the date.
	echoedEnd := filter.end.AddDate(0, 0, 1).Add(-24 * time.Hour).Format(usageDateLayout)
	if snapshot.StartDate != filter.start.Format(usageDateLayout) || snapshot.EndDate != end && snapshot.EndDate != echoedEnd || snapshot.Trend == nil || snapshot.Models == nil {
		return usageSummary{}, fail(502, "invalid_upstream")
	}
	converted := usageSnapshotWire{Trend: make([]usageTrendWire, 0, len(snapshot.Trend)), Models: make([]struct {
		Model    string   `json:"model"`
		Requests *int64   `json:"requests"`
		Tokens   *int64   `json:"total_tokens"`
		Cost     *float64 `json:"actual_cost"`
	}, 0, len(snapshot.Models))}
	for _, row := range snapshot.Trend {
		requests, input, output, cacheRead, cacheWrite, cost := row.Requests, row.Input, row.Output, row.CacheRead, row.CacheWrite, row.ActualCost
		converted.Trend = append(converted.Trend, usageTrendWire{Date: row.Date, Input: &input, Output: &output, CacheRead: &cacheRead, CacheWrite: &cacheWrite, Requests: &requests, Cost: &cost})
	}
	for _, row := range snapshot.Models {
		requests, tokens, cost := row.Requests, row.Tokens, row.ActualCost
		converted.Models = append(converted.Models, struct {
			Model    string   `json:"model"`
			Requests *int64   `json:"requests"`
			Tokens   *int64   `json:"total_tokens"`
			Cost     *float64 `json:"actual_cost"`
		}{Model: row.Model, Requests: &requests, Tokens: &tokens, Cost: &cost})
	}
	return projectUsageSummary(filter, converted, page)
}

func (p *panel) adminUsageSummary(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	query, err := p.adminUsageQuery(r.Context(), a, r.URL.Query())
	if err != nil {
		return err
	}
	result, err := p.buildAdminUsageSummary(r.Context(), a.tokens.Access, query, page)
	if err == nil {
		err = p.readUsageModelSeries(r.Context(), a.user.ID, true, query, &result)
	}
	return send(w, 200, result, err)
}

func projectUsageSummary(filter usageFilter, snapshot usageSnapshotWire, page int) (usageSummary, error) {
	result := usageSummary{Timezone: filter.timezone, Start: filter.start.Format(usageDateLayout), End: filter.end.Format(usageDateLayout),
		Granularity: filter.granularity, Buckets: usageWindows(filter), Days: []usageDay{}, ModelSeries: usageModelSeries{Buckets: []usageModelBucket{}}}
	if snapshot.Trend == nil || snapshot.Models == nil {
		return result, fail(502, "invalid_upstream")
	}
	byDate := make(map[string]usageTotals, len(snapshot.Trend))
	result.expected = make(map[string]usageTotals)
	layout := usageDateLayout
	if filter.granularity == "hour" {
		layout = usageHourLayout
	}
	for _, row := range snapshot.Trend {
		date, err := time.ParseInLocation(layout, row.Date, filter.start.Location())
		previous, duplicate := byDate[row.Date]
		foldedHour := filter.granularity == "hour" && shiftedUsageDay(date)
		if err != nil || date.Format(layout) != row.Date || date.Format(usageDateLayout) < result.Start || date.Format(usageDateLayout) > result.End || duplicate && !foldedHour ||
			row.Requests == nil || *row.Requests < 0 || row.Cost == nil || *row.Cost < 0 || row.Input == nil || row.Output == nil || row.CacheRead == nil || row.CacheWrite == nil {
			return result, fail(502, "invalid_upstream")
		}
		tokens := usageTokens{*row.Input, *row.Output, *row.CacheRead, *row.CacheWrite}
		if !tokens.valid() {
			return result, fail(502, "invalid_upstream")
		}
		point := usageTotals{tokens, *row.Requests, *row.Cost}
		if duplicate {
			previous.add(point)
			point = previous
		}
		byDate[row.Date] = point
		day := date.Format(usageDateLayout)
		daily := result.expected[day]
		daily.add(usageTotals{tokens, *row.Requests, *row.Cost})
		result.expected[day] = daily
		result.Totals.Requests += *row.Requests
		result.Totals.Input += tokens.Input
		result.Totals.Output += tokens.Output
		result.Totals.CacheRead += tokens.CacheRead
		result.Totals.CacheWrite += tokens.CacheWrite
		result.Totals.Cost += *row.Cost
	}
	for i, bucket := range result.Buckets {
		label := bucket.Start.Format(layout)
		entry := byDate[label]
		if filter.granularity != "hour" {
			result.Days = append(result.Days, usageDay{label, entry.usageTokens, entry.Requests, entry.Cost})
		} else if shiftedUsageDay(bucket.Start) {
			result.Buckets[i].Pending = true
			continue
		}
		result.Buckets[i].usageTotals = entry
	}
	models := make([]usageModel, 0, len(snapshot.Models))
	for _, row := range snapshot.Models {
		if row.Model == "" || row.Requests == nil || *row.Requests < 0 || row.Tokens == nil || *row.Tokens < 0 || row.Cost == nil || *row.Cost < 0 {
			return result, fail(502, "invalid_upstream")
		}
		models = append(models, usageModel{row.Model, *row.Requests, *row.Tokens, *row.Cost})
	}
	sort.Slice(models, func(left, right int) bool {
		first, second := models[left], models[right]
		if first.Tokens == second.Tokens {
			return first.Model < second.Model
		}
		return first.Tokens > second.Tokens
	})
	start := min((page-1)*pageSize, len(models))
	end := min(start+pageSize, len(models))
	totalPages := (len(models) + pageSize - 1) / pageSize
	result.Models = pageResult[usageModel]{models[start:end], end < len(models), totalPages}
	result.Ranking = models
	return result, nil
}
