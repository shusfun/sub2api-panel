package observe

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func usageKeys(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, err := platform.RequestedPage(r)
		if err != nil {
			return err
		}
		rows, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, "/keys", page, nil, func(key usageKeyWire) (platform.NamedItem, error) {
			if key.Owner != a.User.ID || key.Name == "" {
				return platform.NamedItem{}, platform.Fail(502, "invalid_upstream")
			}
			return key.NamedItem, platform.RequiredID(key.ID)
		})
		return platform.Send(w, 200, rows, err)
	}
}

func keyUsageBatch(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		input, err := platform.DecodeBody[struct {
			IDs []int64 `json:"api_key_ids"`
		}](r)
		if err != nil || len(input.IDs) == 0 || len(input.IDs) > 100 {
			return platform.Fail(400, "invalid_input")
		}
		seen := make(map[int64]struct{}, len(input.IDs))
		for _, id := range input.IDs {
			if id < 1 {
				return platform.Fail(400, "invalid_input")
			}
			if _, exists := seen[id]; exists {
				return platform.Fail(400, "invalid_input")
			}
			seen[id] = struct{}{}
		}
		result, err := platform.Upstream[struct {
			Stats map[string]KeyUsageStats `json:"stats"`
		}](r.Context(), s.Upstream, "POST", "/usage/dashboard/api-keys-usage", a.Tokens.Access, "", "", input)
		if err != nil {
			return err
		}
		for keyID, stats := range result.Stats {
			parsed, parseErr := strconv.ParseInt(keyID, 10, 64)
			if parseErr != nil || parsed < 1 || stats.Today < 0 || stats.Total < 0 {
				return platform.Fail(502, "invalid_upstream")
			}
			if _, requested := seen[parsed]; !requested {
				return platform.Fail(502, "invalid_upstream")
			}
		}
		if result.Stats == nil {
			result.Stats = map[string]KeyUsageStats{}
		}
		return platform.Send(w, 200, result, nil)
	}
}

func usageRecords(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		return dashboardUsageRecords(s, w, r, a, a.User.ID, false)
	}
}

func usageSummaryHandler(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		if r.URL.Query().Has("user_id") || r.URL.Query().Has("group_id") || r.URL.Query().Has("account_id") {
			return platform.Fail(400, "invalid_input")
		}
		filter, err := ParseUsageFilter(r.URL.Query(), time.Now().In(s.UsageZone()), true)
		if err != nil {
			return err
		}
		page, err := platform.RequestedPage(r)
		if err != nil {
			return err
		}
		result, err := fetchSelfUsageSummary(s, r.Context(), a.Tokens.Access, filter, page)
		if err == nil {
			err = readUsageModelSeries(s, r.Context(), a.User.ID, false, r.URL.Query(), &result)
		}
		return platform.Send(w, 200, result, err)
	}
}

func fetchSelfUsageSummary(s *core.Server, ctx context.Context, token string, filter UsageFilter, page int) (UsageSummary, error) {
	query := filter.Query()
	query.Set("granularity", filter.Granularity)
	query.Set("include_group_stats", "false")
	snapshot, err := platform.Upstream[UsageSnapshotWire](ctx, s.Upstream, "GET", "/usage/dashboard/snapshot-v2?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return UsageSummary{}, err
	}
	return ProjectUsageSummary(filter, snapshot, page)
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

func BuildAdminUsageSummary(s *core.Server, ctx context.Context, token string, query url.Values, page int) (UsageSummary, error) {
	filter, err := ParseUsageFilter(query, time.Now().In(s.UsageZone()), !query.Has("model") || !query.Has("start_date") && !query.Has("period"))
	if err != nil {
		return UsageSummary{}, err
	}
	var userID int64
	if rawUser := query.Get("user_id"); rawUser != "all" {
		userID, err = strconv.ParseInt(rawUser, 10, 64)
		if err != nil || userID < 1 || len(query["user_id"]) != 1 {
			return UsageSummary{}, platform.Fail(400, "invalid_input")
		}
	} else if len(query["user_id"]) != 1 {
		return UsageSummary{}, platform.Fail(400, "invalid_input")
	}
	upstreamQuery := filter.Query()
	if userID > 0 {
		upstreamQuery.Set("user_id", strconv.FormatInt(userID, 10))
	}
	upstreamQuery.Set("granularity", filter.Granularity)
	upstreamQuery.Set("include_stats", "false")
	upstreamQuery.Set("include_trend", "true")
	upstreamQuery.Set("include_model_stats", "true")
	upstreamQuery.Set("include_group_stats", "false")
	snapshot, err := platform.Upstream[adminUsageSnapshotWire](ctx, s.Upstream, "GET", "/admin/dashboard/snapshot-v2?"+upstreamQuery.Encode(), token, "", "", nil)
	if err != nil {
		return UsageSummary{}, err
	}
	end := filter.End.Format(UsageDateLayout)
	// Sub2API v0.2.23 subtracts 24 hours from the exclusive end when echoing the date.
	echoedEnd := filter.End.AddDate(0, 0, 1).Add(-24 * time.Hour).Format(UsageDateLayout)
	if snapshot.StartDate != filter.Start.Format(UsageDateLayout) || snapshot.EndDate != end && snapshot.EndDate != echoedEnd || snapshot.Trend == nil || snapshot.Models == nil {
		return UsageSummary{}, platform.Fail(502, "invalid_upstream")
	}
	converted := UsageSnapshotWire{
		Trend:  make([]UsageTrendWire, 0, len(snapshot.Trend)),
		Models: make([]UsageModelWire, 0, len(snapshot.Models)),
	}
	for _, row := range snapshot.Trend {
		requests, input, output, cacheRead, cacheWrite, cost := row.Requests, row.Input, row.Output, row.CacheRead, row.CacheWrite, row.ActualCost
		converted.Trend = append(converted.Trend, UsageTrendWire{Date: row.Date, Input: &input, Output: &output, CacheRead: &cacheRead, CacheWrite: &cacheWrite, Requests: &requests, Cost: &cost})
	}
	for _, row := range snapshot.Models {
		requests, tokens, cost := row.Requests, row.Tokens, row.ActualCost
		converted.Models = append(converted.Models, UsageModelWire{Model: row.Model, Requests: &requests, Tokens: &tokens, Cost: &cost})
	}
	return ProjectUsageSummary(filter, converted, page)
}

func adminUsageSummaryHandler(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, err := platform.RequestedPage(r)
		if err != nil {
			return err
		}
		query, err := AdminUsageQuery(s, r.Context(), a, r.URL.Query())
		if err != nil {
			return err
		}
		result, err := BuildAdminUsageSummary(s, r.Context(), a.Tokens.Access, query, page)
		if err == nil {
			err = readUsageModelSeries(s, r.Context(), a.User.ID, true, query, &result)
		}
		return platform.Send(w, 200, result, err)
	}
}

func adminUsage(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, err := platform.RequestedPage(r)
		if err != nil {
			return err
		}
		id, err := platform.PathID(r, "user")
		if err != nil {
			return err
		}
		filter, err := ParseUsageFilter(r.URL.Query(), time.Now().In(s.UsageZone()), false)
		if err != nil {
			return err
		}
		query := filter.Query()
		query.Set("user_id", strconv.FormatInt(id, 10))
		rows, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, "/admin/usage", page, query, func(row struct {
			UsageView
			UserID     int64               `json:"user_id"`
			KeyID      int64               `json:"api_key_id"`
			Key        *platform.NamedItem `json:"api_key"`
			CacheRead  *int64              `json:"cache_read_tokens"`
			CacheWrite *int64              `json:"cache_creation_tokens"`
		}) (UsageRecord, error) {
			if row.UserID != id || row.KeyID < 1 || filter.Key != 0 && row.KeyID != filter.Key ||
				filter.Model != "" && row.Model != filter.Model || !filter.Start.IsZero() && (row.Created.Before(filter.Start) || !row.Created.Before(filter.End.AddDate(0, 0, 1))) {
				return UsageRecord{}, platform.Fail(502, "invalid_upstream")
			}
			if row.CacheRead != nil && *row.CacheRead < 0 || row.CacheWrite != nil && *row.CacheWrite < 0 {
				return UsageRecord{}, platform.Fail(502, "invalid_upstream")
			}
			name := fmt.Sprintf("Key #%d", row.KeyID)
			if row.Key != nil && row.Key.ID == row.KeyID && row.Key.Name != "" {
				name = row.Key.Name
			}
			row.Created = row.Created.In(s.UsageZone())
			return UsageRecord{UsageView: row.UsageView, CacheRead: row.CacheRead, CacheWrite: row.CacheWrite, KeyName: name}, platform.RequiredID(row.ID)
		})
		return platform.Send(w, 200, rows, err)
	}
}

type usageUserOption struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func adminUsageUsers(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, params, err := platform.ListQuery(r, "search")
		if err != nil {
			return err
		}
		rows, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, "/admin/users", page, params, func(user struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		}) (usageUserOption, error) {
			if user.Email == "" {
				return usageUserOption{}, platform.Fail(502, "invalid_upstream")
			}
			return usageUserOption{user.ID, user.Email}, platform.RequiredID(user.ID)
		})
		return platform.Send(w, 200, rows, err)
	}
}
