package main

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
	key        int64
	start, end time.Time
	timezone   string
}

func parseUsageFilter(query url.Values, now time.Time, summary bool) (usageFilter, error) {
	filter := usageFilter{timezone: now.Location().String()}
	if query.Has("api_key_id") {
		id, err := strconv.ParseInt(query.Get("api_key_id"), 10, 64)
		if err != nil || id < 1 || len(query["api_key_id"]) != 1 {
			return filter, fail(400, "invalid_input")
		}
		filter.key = id
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
		days := map[string]int{"": 7, "today": 1, "7d": 7, "30d": 30}[period]
		if days == 0 || len(query["period"]) > 1 {
			return filter, fail(400, "invalid_usage_range")
		}
		filter.start, filter.end = today.AddDate(0, 0, 1-days), today
	}
	return filter, nil
}

func (filter usageFilter) query() url.Values {
	query := url.Values{"timezone": {filter.timezone}}
	if filter.key != 0 {
		query.Set("api_key_id", strconv.FormatInt(filter.key, 10))
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
	CacheRead  *int64     `json:"cache_read_tokens"`
	CacheWrite *int64     `json:"cache_creation_tokens"`
	User       int64      `json:"user_id"`
	KeyID      int64      `json:"api_key_id"`
	Key        *namedItem `json:"api_key"`
}

type usageRecord struct {
	usageView
	CacheRead  *int64 `json:"cache_read_tokens"`
	CacheWrite *int64 `json:"cache_creation_tokens"`
	KeyName    string `json:"key_name"`
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

func (p *panel) usage(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	filter, err := parseUsageFilter(r.URL.Query(), time.Now().In(p.usageZone()), false)
	if err != nil {
		return err
	}
	projection := func(row usageRecordWire) (usageRecord, error) {
		if row.ID < 1 || row.User != a.user.ID || row.KeyID < 1 || filter.key != 0 && row.KeyID != filter.key ||
			row.Model == "" || row.Created.IsZero() || row.Input < 0 || row.Output < 0 ||
			row.CacheRead != nil && *row.CacheRead < 0 || row.CacheWrite != nil && *row.CacheWrite < 0 ||
			!filter.start.IsZero() && (row.Created.Before(filter.start) || !row.Created.Before(filter.end.AddDate(0, 0, 1))) {
			return usageRecord{}, fail(502, "invalid_upstream")
		}
		name := "Key #" + strconv.FormatInt(row.KeyID, 10)
		if row.Key != nil && row.Key.ID == row.KeyID && row.Key.Name != "" {
			name = row.Key.Name
		}
		row.Created = row.Created.In(p.usageZone())
		return usageRecord{row.usageView, row.CacheRead, row.CacheWrite, name}, nil
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/usage", page, filter.query(), projection)
	return send(w, 200, rows, err)
}

type usageDay struct {
	Date string `json:"date"`
	usageTokens
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
	Timezone string                 `json:"timezone"`
	Start    string                 `json:"start_date"`
	End      string                 `json:"end_date"`
	Totals   usageTotals            `json:"totals"`
	Days     []usageDay             `json:"days"`
	Models   pageResult[usageModel] `json:"models"`
}

func (p *panel) usageSummary(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	filter, err := parseUsageFilter(r.URL.Query(), time.Now().In(p.usageZone()), true)
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	query := filter.query()
	query.Set("granularity", "day")
	query.Set("include_group_stats", "false")
	snapshot, err := upstream[usageSnapshotWire](r.Context(), p.upstream, "GET", "/usage/dashboard/snapshot-v2?"+query.Encode(), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	result, err := projectUsageSummary(filter, snapshot, page)
	return send(w, 200, result, err)
}

func projectUsageSummary(filter usageFilter, snapshot usageSnapshotWire, page int) (usageSummary, error) {
	result := usageSummary{Timezone: filter.timezone, Start: filter.start.Format(usageDateLayout), End: filter.end.Format(usageDateLayout)}
	if snapshot.Trend == nil || snapshot.Models == nil {
		return result, fail(502, "invalid_upstream")
	}
	byDate := make(map[string]usageTokens, len(snapshot.Trend))
	for _, row := range snapshot.Trend {
		date, err := time.Parse(usageDateLayout, row.Date)
		_, duplicate := byDate[row.Date]
		if err != nil || date.Format(usageDateLayout) != row.Date || row.Date < result.Start || row.Date > result.End || duplicate ||
			row.Requests == nil || *row.Requests < 0 || row.Cost == nil || row.Input == nil || row.Output == nil || row.CacheRead == nil || row.CacheWrite == nil {
			return result, fail(502, "invalid_upstream")
		}
		tokens := usageTokens{*row.Input, *row.Output, *row.CacheRead, *row.CacheWrite}
		if !tokens.valid() {
			return result, fail(502, "invalid_upstream")
		}
		byDate[row.Date] = tokens
		result.Totals.Requests += *row.Requests
		result.Totals.Input += tokens.Input
		result.Totals.Output += tokens.Output
		result.Totals.CacheRead += tokens.CacheRead
		result.Totals.CacheWrite += tokens.CacheWrite
		result.Totals.Cost += *row.Cost
	}
	for date := filter.start; !date.After(filter.end); date = date.AddDate(0, 0, 1) {
		day := date.Format(usageDateLayout)
		result.Days = append(result.Days, usageDay{day, byDate[day]})
	}
	models := make([]usageModel, 0, len(snapshot.Models))
	for _, row := range snapshot.Models {
		if row.Model == "" || row.Requests == nil || *row.Requests < 0 || row.Tokens == nil || *row.Tokens < 0 || row.Cost == nil {
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
	result.Models = pageResult[usageModel]{models[start:end], end < len(models)}
	return result, nil
}
