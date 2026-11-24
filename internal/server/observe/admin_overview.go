package observe

import (
	"net/http"
	"net/url"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type countPage struct {
	Total *int64 `json:"total"`
}

func upstreamCount(s *core.Server, r *http.Request, token, path string, filters url.Values) (int64, error) {
	query := make(url.Values, len(filters)+2)
	for key, values := range filters {
		query[key] = append([]string(nil), values...)
	}
	query.Set("page", "1")
	query.Set("page_size", "1")
	result, err := platform.Upstream[countPage](r.Context(), s.Upstream, "GET", path+"?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return 0, err
	}
	if result.Total == nil || *result.Total < 0 {
		return 0, platform.Fail(502, "invalid_upstream")
	}
	return *result.Total, nil
}

type adminDashboardStatsWire struct {
	Users          int64   `json:"total_users"`
	NewUsers       int64   `json:"today_new_users"`
	ActiveUsers    int64   `json:"active_users"`
	Accounts       int64   `json:"total_accounts"`
	NormalAccounts int64   `json:"normal_accounts"`
	ErrorAccounts  int64   `json:"error_accounts"`
	RateLimited    int64   `json:"ratelimit_accounts"`
	Overloaded     int64   `json:"overload_accounts"`
	Keys           int64   `json:"total_api_keys"`
	ActiveKeys     int64   `json:"active_api_keys"`
	TodayRequests  int64   `json:"today_requests"`
	TotalRequests  int64   `json:"total_requests"`
	TodayTokens    int64   `json:"today_tokens"`
	TotalTokens    int64   `json:"total_tokens"`
	TodayCost      float64 `json:"today_actual_cost"`
	TotalCost      float64 `json:"total_actual_cost"`
	AvgDurationMs  float64 `json:"average_duration_ms"`
	Uptime         int64   `json:"uptime"`
	RPM            float64 `json:"rpm"`
	TPM            float64 `json:"tpm"`
}

type overviewGroupStat struct {
	GroupID  int64   `json:"group_id"`
	Name     string  `json:"group_name"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"total_tokens"`
	Cost     float64 `json:"actual_cost"`
}

type overviewUserRank struct {
	UserID   int64   `json:"user_id"`
	Email    string  `json:"email"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"actual_cost"`
}

type adminOverviewView struct {
	Users struct {
		Total    int64 `json:"total"`
		NewToday int64 `json:"new_today"`
		Active   int64 `json:"active"`
	} `json:"users"`
	Accounts struct {
		Total       int64 `json:"total"`
		Normal      int64 `json:"normal"`
		Error       int64 `json:"error"`
		RateLimited int64 `json:"rate_limited"`
		Overloaded  int64 `json:"overloaded"`
	} `json:"accounts"`
	Keys struct {
		Total  int64 `json:"total"`
		Active int64 `json:"active"`
	} `json:"keys"`
	Codes struct {
		Total   int64 `json:"total"`
		Active  int64 `json:"active"`
		Used    int64 `json:"used"`
		Expired int64 `json:"expired"`
	} `json:"codes"`
	Subscriptions struct {
		Total  int64 `json:"total"`
		Active int64 `json:"active"`
	} `json:"subscriptions"`
	Announcements int64 `json:"announcements"`
	Groups        int64 `json:"groups"`
	Proxies       struct {
		Total  int64 `json:"total"`
		Active int64 `json:"active"`
	} `json:"proxies"`
	Traffic struct {
		TodayRequests int64   `json:"today_requests"`
		TotalRequests int64   `json:"total_requests"`
		TodayTokens   int64   `json:"today_tokens"`
		TotalTokens   int64   `json:"total_tokens"`
		TodayCost     float64 `json:"today_cost"`
		TotalCost     float64 `json:"total_cost"`
		AvgDurationMs float64 `json:"average_duration_ms"`
		Uptime        int64   `json:"uptime"`
		RPM           float64 `json:"rpm"`
		TPM           float64 `json:"tpm"`
	} `json:"traffic"`
	Trend     []UsageDay          `json:"trend"`
	TopGroups []overviewGroupStat `json:"top_groups"`
	TopModels []UsageModel        `json:"top_models"`
	TopUsers  []overviewUserRank  `json:"top_users"`
}

func adminOverview(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		stats, err := platform.Upstream[adminDashboardStatsWire](r.Context(), s.Upstream, "GET", "/admin/dashboard/stats", a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		if stats.Users < 0 || stats.NewUsers < 0 || stats.ActiveUsers < 0 || stats.Accounts < 0 || stats.NormalAccounts < 0 ||
			stats.ErrorAccounts < 0 || stats.RateLimited < 0 || stats.Overloaded < 0 || stats.Keys < 0 || stats.ActiveKeys < 0 ||
			stats.TodayRequests < 0 || stats.TotalRequests < 0 || stats.TodayTokens < 0 || stats.TotalTokens < 0 ||
			stats.TodayCost < 0 || stats.TotalCost < 0 || stats.AvgDurationMs < 0 || stats.Uptime < 0 || stats.RPM < 0 || stats.TPM < 0 {
			return platform.Fail(502, "invalid_upstream")
		}
		result := adminOverviewView{
			Trend:     []UsageDay{},
			TopGroups: []overviewGroupStat{},
			TopModels: []UsageModel{},
			TopUsers:  []overviewUserRank{},
		}
		result.Users.Total, result.Users.NewToday, result.Users.Active = stats.Users, stats.NewUsers, stats.ActiveUsers
		result.Accounts.Total, result.Accounts.Normal, result.Accounts.Error = stats.Accounts, stats.NormalAccounts, stats.ErrorAccounts
		result.Accounts.RateLimited, result.Accounts.Overloaded = stats.RateLimited, stats.Overloaded
		result.Keys.Total, result.Keys.Active = stats.Keys, stats.ActiveKeys
		result.Traffic.TodayRequests, result.Traffic.TotalRequests = stats.TodayRequests, stats.TotalRequests
		result.Traffic.TodayTokens, result.Traffic.TotalTokens = stats.TodayTokens, stats.TotalTokens
		result.Traffic.TodayCost, result.Traffic.TotalCost = stats.TodayCost, stats.TotalCost
		result.Traffic.AvgDurationMs, result.Traffic.Uptime = stats.AvgDurationMs, stats.Uptime
		result.Traffic.RPM, result.Traffic.TPM = stats.RPM, stats.TPM

		codeStats, err := platform.Upstream[struct {
			Total   int64 `json:"total_codes"`
			Active  int64 `json:"active_codes"`
			Used    int64 `json:"used_codes"`
			Expired int64 `json:"expired_codes"`
		}](r.Context(), s.Upstream, "GET", "/admin/redeem-codes/stats", a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		result.Codes.Total, result.Codes.Active, result.Codes.Used, result.Codes.Expired = codeStats.Total, codeStats.Active, codeStats.Used, codeStats.Expired
		for _, count := range []int64{codeStats.Total, codeStats.Active, codeStats.Used, codeStats.Expired} {
			if count < 0 {
				return platform.Fail(502, "invalid_upstream")
			}
		}

		if result.Subscriptions.Total, err = upstreamCount(s, r, a.Tokens.Access, "/admin/subscriptions", nil); err != nil {
			return err
		}
		if result.Subscriptions.Active, err = upstreamCount(s, r, a.Tokens.Access, "/admin/subscriptions", url.Values{"status": {"active"}}); err != nil {
			return err
		}
		if result.Announcements, err = upstreamCount(s, r, a.Tokens.Access, "/admin/announcements", nil); err != nil {
			return err
		}
		if result.Groups, err = upstreamCount(s, r, a.Tokens.Access, "/admin/groups", nil); err != nil {
			return err
		}
		if result.Proxies.Total, err = upstreamCount(s, r, a.Tokens.Access, "/admin/proxies", nil); err != nil {
			return err
		}
		if result.Proxies.Active, err = upstreamCount(s, r, a.Tokens.Access, "/admin/proxies", url.Values{"status": {"active"}}); err != nil {
			return err
		}

		now := time.Now().In(s.UsageZone())
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		start := today.AddDate(0, 0, -6).Format(UsageDateLayout)
		end := today.Format(UsageDateLayout)
		snapQuery := url.Values{
			"start_date":          {start},
			"end_date":            {end},
			"timezone":            {now.Location().String()},
			"granularity":         {"day"},
			"include_stats":       {"false"},
			"include_trend":       {"true"},
			"include_model_stats": {"true"},
			"include_group_stats": {"true"},
		}
		if snap, snapErr := platform.Upstream[struct {
			Trend []struct {
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
			Groups []overviewGroupStat `json:"groups"`
		}](r.Context(), s.Upstream, "GET", "/admin/dashboard/snapshot-v2?"+snapQuery.Encode(), a.Tokens.Access, "", "", nil); snapErr == nil {
			for _, item := range snap.Trend {
				if item.Date != "" && item.Requests >= 0 && item.ActualCost >= 0 {
					result.Trend = append(result.Trend, UsageDay{
						Date:        item.Date,
						UsageTokens: UsageTokens{item.Input, item.Output, item.CacheRead, item.CacheWrite},
						Requests:    item.Requests,
						Cost:        item.ActualCost,
					})
				}
			}
			for _, item := range snap.Models {
				if item.Model != "" && item.Requests >= 0 && item.ActualCost >= 0 {
					result.TopModels = append(result.TopModels, UsageModel{item.Model, item.Requests, item.Tokens, item.ActualCost})
				}
			}
			for _, item := range snap.Groups {
				if item.GroupID > 0 && item.Requests >= 0 && item.Cost >= 0 {
					result.TopGroups = append(result.TopGroups, item)
				}
			}
		}
		rankQuery := url.Values{"start_date": {start}, "end_date": {end}, "limit": {"8"}}
		if ranking, rankErr := platform.Upstream[struct {
			Ranking []overviewUserRank `json:"ranking"`
		}](r.Context(), s.Upstream, "GET", "/admin/dashboard/users-ranking?"+rankQuery.Encode(), a.Tokens.Access, "", "", nil); rankErr == nil {
			for _, item := range ranking.Ranking {
				if item.UserID > 0 && item.Email != "" && item.Requests >= 0 && item.Cost >= 0 {
					result.TopUsers = append(result.TopUsers, item)
				}
			}
		}

		platform.WriteJSON(w, 200, result)
		return nil
	}
}
