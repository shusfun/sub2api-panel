package observe

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const UsageDateLayout = "2006-01-02"
const UsageHourLayout = "2006-01-02 15:04"
const maxUsageDays = 90

type UsageFilter struct {
	Key         int64
	Group       int64
	Account     int64
	Granularity string
	Period      string
	Model       string
	Stream      string
	Start       time.Time
	End         time.Time
	Timezone    string
}

func ParseUsageFilter(query url.Values, now time.Time, summary bool) (UsageFilter, error) {
	filter := UsageFilter{Timezone: now.Location().String(), Granularity: "day"}
	if query.Has("granularity") {
		filter.Granularity = query.Get("granularity")
		if len(query["granularity"]) != 1 || filter.Granularity != "hour" && filter.Granularity != "day" {
			return filter, platform.Fail(400, "invalid_input")
		}
	}
	if query.Has("api_key_id") {
		id, err := strconv.ParseInt(query.Get("api_key_id"), 10, 64)
		if err != nil || id < 1 || len(query["api_key_id"]) != 1 {
			return filter, platform.Fail(400, "invalid_input")
		}
		filter.Key = id
	}
	if query.Has("group_id") {
		id, err := strconv.ParseInt(query.Get("group_id"), 10, 64)
		if err != nil || id < 1 || len(query["group_id"]) != 1 {
			return filter, platform.Fail(400, "invalid_input")
		}
		filter.Group = id
	}
	if query.Has("account_id") {
		id, err := strconv.ParseInt(query.Get("account_id"), 10, 64)
		if err != nil || id < 1 || len(query["account_id"]) != 1 {
			return filter, platform.Fail(400, "invalid_input")
		}
		filter.Account = id
	}
	if query.Has("model") {
		model := strings.TrimSpace(query.Get("model"))
		if summary || model == "" || len(model) > 128 || len(query["model"]) != 1 {
			return filter, platform.Fail(400, "invalid_input")
		}
		filter.Model = model
	}
	if query.Has("stream") {
		stream := strings.TrimSpace(query.Get("stream"))
		if summary || (stream != "true" && stream != "false") || len(query["stream"]) != 1 {
			return filter, platform.Fail(400, "invalid_input")
		}
		filter.Stream = stream
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if query.Has("start_date") || query.Has("end_date") {
		start, startErr := time.ParseInLocation(UsageDateLayout, query.Get("start_date"), now.Location())
		end, endErr := time.ParseInLocation(UsageDateLayout, query.Get("end_date"), now.Location())
		if startErr != nil || endErr != nil || start.Year() < 1 || query.Has("period") || end.Before(start) || end.After(today) ||
			end.After(start.AddDate(0, 0, maxUsageDays-1)) || len(query["start_date"]) != 1 || len(query["end_date"]) != 1 {
			return filter, platform.Fail(400, "invalid_usage_range")
		}
		filter.Start, filter.End = start, end
	} else if summary || query.Has("period") {
		period := query.Get("period")
		filter.Period = period
		if period == "" {
			filter.Period = "7d"
		}
		days := map[string]int{"": 7, "today": 1, "7d": 7, "14d": 14, "30d": 30, "90d": 90}[period]
		if days == 0 || len(query["period"]) > 1 {
			return filter, platform.Fail(400, "invalid_usage_range")
		}
		filter.Start, filter.End = today.AddDate(0, 0, 1-days), today
	}
	if filter.Granularity == "hour" && !filter.Start.IsZero() && filter.End.After(filter.Start.AddDate(0, 0, 6)) {
		return filter, platform.Fail(400, "invalid_hour_range")
	}
	return filter, nil
}

func (filter UsageFilter) Query() url.Values {
	query := url.Values{"timezone": {filter.Timezone}}
	if filter.Key != 0 {
		query.Set("api_key_id", strconv.FormatInt(filter.Key, 10))
	}
	if filter.Group != 0 {
		query.Set("group_id", strconv.FormatInt(filter.Group, 10))
	}
	if filter.Account != 0 {
		query.Set("account_id", strconv.FormatInt(filter.Account, 10))
	}
	if filter.Model != "" {
		query.Set("model", filter.Model)
	}
	if filter.Stream != "" {
		query.Set("stream", filter.Stream)
	}
	if !filter.Start.IsZero() {
		query.Set("start_date", filter.Start.Format(UsageDateLayout))
		query.Set("end_date", filter.End.Format(UsageDateLayout))
	}
	return query
}

type UsageView struct {
	ID      int64     `json:"id"`
	Model   string    `json:"model"`
	Created time.Time `json:"created_at"`
	Input   int64     `json:"input_tokens"`
	Output  int64     `json:"output_tokens"`
	Cost    *float64  `json:"actual_cost"`
}

type UsageTokens struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheRead  int64 `json:"cache_read_tokens"`
	CacheWrite int64 `json:"cache_creation_tokens"`
}

func (tokens UsageTokens) valid() bool {
	return tokens.Input >= 0 && tokens.Output >= 0 && tokens.CacheRead >= 0 && tokens.CacheWrite >= 0
}

type usageRecordWire struct {
	UsageView
	CacheRead    *int64              `json:"cache_read_tokens"`
	CacheWrite   *int64              `json:"cache_creation_tokens"`
	User         int64               `json:"user_id"`
	KeyID        int64               `json:"api_key_id"`
	GroupID      *int64              `json:"group_id"`
	AccountID    *int64              `json:"account_id"`
	DurationMs   *int64              `json:"duration_ms"`
	FirstTokenMs *int64              `json:"first_token_ms"`
	Stream       *bool               `json:"stream"`
	Key          *platform.NamedItem `json:"api_key"`
	Group        *platform.NamedItem `json:"group"`
	Account      *platform.NamedItem `json:"account"`
	UserObj      *struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

type UsageRecord struct {
	UsageView
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

type KeyUsageStats struct {
	Today float64 `json:"today_actual_cost"`
	Total float64 `json:"total_actual_cost"`
}

type UsageDay struct {
	Date string `json:"date"`
	UsageTokens
	Requests int64   `json:"requests"`
	Cost     float64 `json:"actual_cost"`
}

type UsageBucket struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Pending bool      `json:"pending,omitempty"`
	UsageTotals
}

type UsageTotals struct {
	UsageTokens
	Requests int64   `json:"requests"`
	Cost     float64 `json:"actual_cost"`
}

func (total *UsageTotals) add(row UsageTotals) {
	total.Requests += row.Requests
	total.Cost += row.Cost
	total.Input += row.Input
	total.Output += row.Output
	total.CacheRead += row.CacheRead
	total.CacheWrite += row.CacheWrite
}

type UsageTrendWire struct {
	Date       string   `json:"date"`
	Input      *int64   `json:"input_tokens"`
	Output     *int64   `json:"output_tokens"`
	CacheRead  *int64   `json:"cache_read_tokens"`
	CacheWrite *int64   `json:"cache_creation_tokens"`
	Requests   *int64   `json:"requests"`
	Cost       *float64 `json:"actual_cost"`
}

type UsageModelWire struct {
	Model    string   `json:"model"`
	Requests *int64   `json:"requests"`
	Tokens   *int64   `json:"total_tokens"`
	Cost     *float64 `json:"actual_cost"`
}

type UsageModel struct {
	Model    string  `json:"model"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"total_tokens"`
	Cost     float64 `json:"actual_cost"`
}

type UsageModelBucket struct {
	Start  time.Time    `json:"start"`
	Models []UsageModel `json:"models"`
}

type UsageModelSeries struct {
	ReadyDays   int                `json:"ready_days"`
	TotalDays   int                `json:"total_days"`
	Buckets     []UsageModelBucket `json:"buckets"`
	Unavailable bool               `json:"unavailable,omitempty"`
}

type UsageSnapshotWire struct {
	Trend  []UsageTrendWire `json:"trend"`
	Models []UsageModelWire `json:"models"`
}

type UsageSummary struct {
	Timezone    string                          `json:"timezone"`
	Start       string                          `json:"start_date"`
	End         string                          `json:"end_date"`
	Totals      UsageTotals                     `json:"totals"`
	Days        []UsageDay                      `json:"days"`
	Models      platform.PageResult[UsageModel] `json:"models"`
	Granularity string                          `json:"granularity"`
	Buckets     []UsageBucket                   `json:"buckets"`
	Ranking     []UsageModel                    `json:"ranking"`
	ModelSeries UsageModelSeries                `json:"model_series"`
	expected    map[string]UsageTotals
}

func UsageWindows(filter UsageFilter) []UsageBucket {
	windows := []UsageBucket{}
	for day := filter.Start; !day.IsZero() && !day.After(filter.End); day = day.AddDate(0, 0, 1) {
		end := day.AddDate(0, 0, 1)
		for start := day; start.Before(end); {
			next := end
			if filter.Granularity == "hour" && start.Add(time.Hour).Before(end) {
				next = start.Add(time.Hour)
			}
			windows = append(windows, UsageBucket{Start: start, End: next})
			start = next
		}
	}
	return windows
}

func HourlyWindows(filter UsageFilter) []UsageBucket {
	filter.Granularity = "hour"
	return UsageWindows(filter)
}

func shiftedUsageDay(at time.Time) bool {
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
	return day.AddDate(0, 0, 1).Sub(day) != 24*time.Hour
}

func usageTarget(query url.Values, actor int64) (int64, error) {
	if !query.Has("user_id") {
		return actor, nil
	}
	if len(query["user_id"]) != 1 {
		return 0, platform.Fail(400, "invalid_input")
	}
	if query.Get("user_id") == "all" {
		return 0, nil
	}
	id, err := strconv.ParseInt(query.Get("user_id"), 10, 64)
	if err != nil || id < 1 {
		return 0, platform.Fail(400, "invalid_input")
	}
	return id, nil
}

func timezoneLocation(name string) *time.Location {
	zone, err := core.LoadUsageLocation(name)
	if err != nil {
		return time.UTC
	}
	return zone
}

func ProjectUsageSummary(filter UsageFilter, snapshot UsageSnapshotWire, page int) (UsageSummary, error) {
	result := UsageSummary{
		Timezone:    filter.Timezone,
		Start:       filter.Start.Format(UsageDateLayout),
		End:         filter.End.Format(UsageDateLayout),
		Granularity: filter.Granularity,
		Buckets:     UsageWindows(filter),
		Days:        []UsageDay{},
		ModelSeries: UsageModelSeries{Buckets: []UsageModelBucket{}},
	}
	if snapshot.Trend == nil || snapshot.Models == nil {
		return result, platform.Fail(502, "invalid_upstream")
	}
	byDate := make(map[string]UsageTotals, len(snapshot.Trend))
	result.expected = make(map[string]UsageTotals)
	layout := UsageDateLayout
	if filter.Granularity == "hour" {
		layout = UsageHourLayout
	}
	for _, row := range snapshot.Trend {
		date, err := time.ParseInLocation(layout, row.Date, filter.Start.Location())
		previous, duplicate := byDate[row.Date]
		foldedHour := filter.Granularity == "hour" && shiftedUsageDay(date)
		if err != nil || date.Format(layout) != row.Date || date.Format(UsageDateLayout) < result.Start || date.Format(UsageDateLayout) > result.End || duplicate && !foldedHour ||
			row.Requests == nil || *row.Requests < 0 || row.Cost == nil || *row.Cost < 0 || row.Input == nil || row.Output == nil || row.CacheRead == nil || row.CacheWrite == nil {
			return result, platform.Fail(502, "invalid_upstream")
		}
		tokens := UsageTokens{*row.Input, *row.Output, *row.CacheRead, *row.CacheWrite}
		if !tokens.valid() {
			return result, platform.Fail(502, "invalid_upstream")
		}
		point := UsageTotals{tokens, *row.Requests, *row.Cost}
		if duplicate {
			previous.add(point)
			point = previous
		}
		byDate[row.Date] = point
		day := date.Format(UsageDateLayout)
		daily := result.expected[day]
		daily.add(UsageTotals{tokens, *row.Requests, *row.Cost})
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
		if filter.Granularity != "hour" {
			result.Days = append(result.Days, UsageDay{label, entry.UsageTokens, entry.Requests, entry.Cost})
		} else if shiftedUsageDay(bucket.Start) {
			result.Buckets[i].Pending = true
			continue
		}
		result.Buckets[i].UsageTotals = entry
	}
	models := make([]UsageModel, 0, len(snapshot.Models))
	for _, row := range snapshot.Models {
		if row.Model == "" || row.Requests == nil || *row.Requests < 0 || row.Tokens == nil || *row.Tokens < 0 || row.Cost == nil || *row.Cost < 0 {
			return result, platform.Fail(502, "invalid_upstream")
		}
		models = append(models, UsageModel{row.Model, *row.Requests, *row.Tokens, *row.Cost})
	}
	sort.Slice(models, func(left, right int) bool {
		first, second := models[left], models[right]
		if first.Tokens == second.Tokens {
			return first.Model < second.Model
		}
		return first.Tokens > second.Tokens
	})
	start := min((page-1)*platform.PageSize, len(models))
	end := min(start+platform.PageSize, len(models))
	totalPages := (len(models) + platform.PageSize - 1) / platform.PageSize
	result.Models = platform.PageResult[UsageModel]{Items: models[start:end], More: end < len(models), Pages: totalPages}
	result.Ranking = models
	return result, nil
}
