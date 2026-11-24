package observe

import (
	"context"
	"net/http"
	"net/url"
	"sort"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type opsPlatformConcurrency struct {
	Platform       string  `json:"platform"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
	TotalAccounts  int64   `json:"total_accounts"`
	AvailableCount int64   `json:"available_count"`
	RateLimitCount int64   `json:"rate_limit_count"`
	ErrorCount     int64   `json:"error_count"`
}

type opsGroupConcurrency struct {
	GroupID        int64   `json:"group_id"`
	GroupName      string  `json:"group_name"`
	Platform       string  `json:"platform"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
	TotalAccounts  int64   `json:"total_accounts"`
	AvailableCount int64   `json:"available_count"`
	RateLimitCount int64   `json:"rate_limit_count"`
	ErrorCount     int64   `json:"error_count"`
}

type opsAccountRuntime struct {
	AccountID        int64   `json:"account_id"`
	AccountName      string  `json:"account_name"`
	Platform         string  `json:"platform"`
	GroupID          int64   `json:"group_id"`
	GroupName        string  `json:"group_name"`
	Status           string  `json:"status"`
	CurrentInUse     int64   `json:"current_in_use"`
	MaxCapacity      int64   `json:"max_capacity"`
	LoadPercentage   float64 `json:"load_percentage"`
	WaitingInQueue   int64   `json:"waiting_in_queue"`
	IsAvailable      bool    `json:"is_available"`
	IsRateLimited    bool    `json:"is_rate_limited"`
	RateLimitResetAt string  `json:"rate_limit_reset_at,omitempty"`
	IsOverloaded     bool    `json:"is_overloaded"`
	OverloadUntil    string  `json:"overload_until,omitempty"`
	HasError         bool    `json:"has_error"`
}

type opsUserConcurrency struct {
	UserID         int64   `json:"user_id"`
	UserEmail      string  `json:"user_email"`
	Username       string  `json:"username"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
}

type opsConcurrencyView struct {
	Enabled   bool                     `json:"enabled"`
	Timestamp string                   `json:"timestamp"`
	QPS       opsRateView              `json:"qps"`
	TPS       opsRateView              `json:"tps"`
	Platforms []opsPlatformConcurrency `json:"platforms"`
	Groups    []opsGroupConcurrency    `json:"groups"`
	Accounts  []opsAccountRuntime      `json:"accounts"`
	Users     []opsUserConcurrency     `json:"users"`
}

func opsConcurrency(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		params, err := parseOpsScopeQuery(r.URL.Query())
		if err != nil {
			return err
		}
		result, err := fetchOpsConcurrency(s, r.Context(), a.Tokens.Access, params)
		return platform.Send(w, 200, result, err)
	}
}

func fetchOpsConcurrency(s *core.Server, ctx context.Context, token string, params url.Values) (opsConcurrencyView, error) {
	scope := url.Values{}
	if plat := params.Get("platform"); plat != "" {
		scope.Set("platform", plat)
	}
	if groupID := params.Get("group_id"); groupID != "" {
		scope.Set("group_id", groupID)
	}
	suffix := ""
	if len(scope) > 0 {
		suffix = "?" + scope.Encode()
	}
	concurrency, err := platform.Upstream[struct {
		Enabled  bool `json:"enabled"`
		Platform map[string]struct {
			Platform       string  `json:"platform"`
			CurrentInUse   int64   `json:"current_in_use"`
			MaxCapacity    int64   `json:"max_capacity"`
			LoadPercentage float64 `json:"load_percentage"`
			WaitingInQueue int64   `json:"waiting_in_queue"`
		} `json:"platform"`
		Group map[string]struct {
			GroupID        int64   `json:"group_id"`
			GroupName      string  `json:"group_name"`
			Platform       string  `json:"platform"`
			CurrentInUse   int64   `json:"current_in_use"`
			MaxCapacity    int64   `json:"max_capacity"`
			LoadPercentage float64 `json:"load_percentage"`
			WaitingInQueue int64   `json:"waiting_in_queue"`
		} `json:"group"`
		Account map[string]struct {
			AccountID      int64   `json:"account_id"`
			AccountName    string  `json:"account_name"`
			Platform       string  `json:"platform"`
			GroupID        int64   `json:"group_id"`
			GroupName      string  `json:"group_name"`
			CurrentInUse   int64   `json:"current_in_use"`
			MaxCapacity    int64   `json:"max_capacity"`
			LoadPercentage float64 `json:"load_percentage"`
			WaitingInQueue int64   `json:"waiting_in_queue"`
		} `json:"account"`
		Timestamp string `json:"timestamp"`
	}](ctx, s.Upstream, "GET", "/admin/ops/concurrency"+suffix, token, "", "", nil)
	if err != nil {
		return opsConcurrencyView{}, err
	}

	availability, _ := platform.Upstream[struct {
		Platform map[string]struct {
			TotalAccounts  int64 `json:"total_accounts"`
			AvailableCount int64 `json:"available_count"`
			RateLimitCount int64 `json:"rate_limit_count"`
			ErrorCount     int64 `json:"error_count"`
		} `json:"platform"`
		Group map[string]struct {
			GroupID        int64  `json:"group_id"`
			GroupName      string `json:"group_name"`
			Platform       string `json:"platform"`
			TotalAccounts  int64  `json:"total_accounts"`
			AvailableCount int64  `json:"available_count"`
			RateLimitCount int64  `json:"rate_limit_count"`
			ErrorCount     int64  `json:"error_count"`
		} `json:"group"`
		Account map[string]struct {
			AccountID        int64  `json:"account_id"`
			AccountName      string `json:"account_name"`
			Platform         string `json:"platform"`
			GroupID          int64  `json:"group_id"`
			GroupName        string `json:"group_name"`
			Status           string `json:"status"`
			IsAvailable      bool   `json:"is_available"`
			IsRateLimited    bool   `json:"is_rate_limited"`
			RateLimitResetAt string `json:"rate_limit_reset_at"`
			IsOverloaded     bool   `json:"is_overloaded"`
			OverloadUntil    string `json:"overload_until"`
			HasError         bool   `json:"has_error"`
		} `json:"account"`
	}](ctx, s.Upstream, "GET", "/admin/ops/account-availability"+suffix, token, "", "", nil)

	userConc, _ := platform.Upstream[struct {
		User map[string]opsUserConcurrency `json:"user"`
	}](ctx, s.Upstream, "GET", "/admin/ops/user-concurrency", token, "", "", nil)

	trafficParams := url.Values{"window": {"5m"}}
	for k, v := range scope {
		trafficParams[k] = v
	}
	traffic, _ := platform.Upstream[struct {
		Summary *struct {
			QPS opsRateView `json:"qps"`
			TPS opsRateView `json:"tps"`
		} `json:"summary"`
	}](ctx, s.Upstream, "GET", "/admin/ops/realtime-traffic?"+trafficParams.Encode(), token, "", "", nil)

	result := opsConcurrencyView{
		Enabled:   concurrency.Enabled,
		Timestamp: concurrency.Timestamp,
		Platforms: []opsPlatformConcurrency{},
		Groups:    []opsGroupConcurrency{},
		Accounts:  []opsAccountRuntime{},
		Users:     []opsUserConcurrency{},
	}
	if traffic.Summary != nil {
		result.QPS = traffic.Summary.QPS
		result.TPS = traffic.Summary.TPS
	}

	platformMap := make(map[string]opsPlatformConcurrency)
	for key, item := range concurrency.Platform {
		name := item.Platform
		if name == "" {
			name = key
		}
		platformMap[name] = opsPlatformConcurrency{
			Platform:       name,
			CurrentInUse:   item.CurrentInUse,
			MaxCapacity:    item.MaxCapacity,
			LoadPercentage: item.LoadPercentage,
			WaitingInQueue: item.WaitingInQueue,
		}
	}
	for key, avail := range availability.Platform {
		row := platformMap[key]
		row.Platform = key
		row.TotalAccounts = avail.TotalAccounts
		row.AvailableCount = avail.AvailableCount
		row.RateLimitCount = avail.RateLimitCount
		row.ErrorCount = avail.ErrorCount
		platformMap[key] = row
	}
	for _, item := range platformMap {
		result.Platforms = append(result.Platforms, item)
	}
	sort.Slice(result.Platforms, func(i, j int) bool { return result.Platforms[i].Platform < result.Platforms[j].Platform })

	groupMap := make(map[int64]opsGroupConcurrency)
	for _, item := range concurrency.Group {
		if item.GroupID > 0 {
			groupMap[item.GroupID] = opsGroupConcurrency{
				GroupID:        item.GroupID,
				GroupName:      item.GroupName,
				Platform:       item.Platform,
				CurrentInUse:   item.CurrentInUse,
				MaxCapacity:    item.MaxCapacity,
				LoadPercentage: item.LoadPercentage,
				WaitingInQueue: item.WaitingInQueue,
			}
		}
	}
	for _, avail := range availability.Group {
		if avail.GroupID > 0 {
			row := groupMap[avail.GroupID]
			row.GroupID = avail.GroupID
			if row.GroupName == "" {
				row.GroupName = avail.GroupName
			}
			if row.Platform == "" {
				row.Platform = avail.Platform
			}
			row.TotalAccounts = avail.TotalAccounts
			row.AvailableCount = avail.AvailableCount
			row.RateLimitCount = avail.RateLimitCount
			row.ErrorCount = avail.ErrorCount
			groupMap[avail.GroupID] = row
		}
	}
	for _, item := range groupMap {
		result.Groups = append(result.Groups, item)
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].GroupID < result.Groups[j].GroupID })

	accountMap := make(map[int64]opsAccountRuntime)
	for _, item := range concurrency.Account {
		if item.AccountID > 0 {
			accountMap[item.AccountID] = opsAccountRuntime{
				AccountID:      item.AccountID,
				AccountName:    item.AccountName,
				Platform:       item.Platform,
				GroupID:        item.GroupID,
				GroupName:      item.GroupName,
				Status:         "active",
				CurrentInUse:   item.CurrentInUse,
				MaxCapacity:    item.MaxCapacity,
				LoadPercentage: item.LoadPercentage,
				WaitingInQueue: item.WaitingInQueue,
				IsAvailable:    true,
			}
		}
	}
	for _, avail := range availability.Account {
		if avail.AccountID > 0 {
			row := accountMap[avail.AccountID]
			row.AccountID = avail.AccountID
			if row.AccountName == "" {
				row.AccountName = avail.AccountName
			}
			if row.Platform == "" {
				row.Platform = avail.Platform
			}
			if row.GroupID == 0 {
				row.GroupID = avail.GroupID
			}
			if row.GroupName == "" {
				row.GroupName = avail.GroupName
			}
			row.Status = avail.Status
			row.IsAvailable = avail.IsAvailable
			row.IsRateLimited = avail.IsRateLimited
			row.RateLimitResetAt = avail.RateLimitResetAt
			row.IsOverloaded = avail.IsOverloaded
			row.OverloadUntil = avail.OverloadUntil
			row.HasError = avail.HasError
			accountMap[avail.AccountID] = row
		}
	}
	for _, item := range accountMap {
		result.Accounts = append(result.Accounts, item)
	}
	sort.Slice(result.Accounts, func(i, j int) bool {
		if result.Accounts[i].HasError != result.Accounts[j].HasError {
			return result.Accounts[i].HasError
		}
		if result.Accounts[i].IsRateLimited != result.Accounts[j].IsRateLimited {
			return result.Accounts[i].IsRateLimited
		}
		return result.Accounts[i].AccountID < result.Accounts[j].AccountID
	})

	for _, item := range userConc.User {
		if item.UserID > 0 {
			result.Users = append(result.Users, item)
		}
	}
	sort.Slice(result.Users, func(i, j int) bool { return result.Users[i].CurrentInUse > result.Users[j].CurrentInUse })

	return result, nil
}
