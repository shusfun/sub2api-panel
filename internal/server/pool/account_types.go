package pool

import (
	"encoding/json"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

var validAccountPlatforms = map[string]bool{
	"openai": true, "anthropic": true, "gemini": true, "antigravity": true,
	"grok": true, "deepseek": true, "kimi": true, "zhipu": true,
	"minimax": true, "opencode_go": true, "command_code": true, "cline": true,
}

var validAccountTypes = map[string]bool{
	"oauth": true, "setup-token": true, "apikey": true,
	"upstream": true, "bedrock": true, "service_account": true,
}

func validAccountPlatform(p string) bool { return validAccountPlatforms[p] }
func validAccountType(t string) bool     { return validAccountTypes[t] }

type AccountExtraView struct {
	WindowCostLimit         *float64 `json:"window_cost_limit,omitempty"`
	WindowCostStickyReserve *float64 `json:"window_cost_sticky_reserve,omitempty"`
	DailyQuota              *float64 `json:"daily_quota,omitempty"`
	TotalQuota              *float64 `json:"total_quota,omitempty"`
	MaxSessions             *int     `json:"max_sessions,omitempty"`
	RPMLimit                *int     `json:"rpm_limit,omitempty"`
	CodexCLIOnly            bool     `json:"codex_cli_only,omitempty"`
	TLSFingerprint          bool     `json:"tls_fingerprint,omitempty"`
	InterceptWarmup         bool     `json:"intercept_warmup,omitempty"`
	ModelAllowlist          []string `json:"model_allowlist,omitempty"`
}

type AccountExtraInput struct {
	WindowCostLimit         *float64  `json:"window_cost_limit,omitempty"`
	WindowCostStickyReserve *float64  `json:"window_cost_sticky_reserve,omitempty"`
	DailyQuota              *float64  `json:"daily_quota,omitempty"`
	TotalQuota              *float64  `json:"total_quota,omitempty"`
	MaxSessions             *int      `json:"max_sessions,omitempty"`
	RPMLimit                *int      `json:"rpm_limit,omitempty"`
	CodexCLIOnly            *bool     `json:"codex_cli_only,omitempty"`
	TLSFingerprint          *bool     `json:"tls_fingerprint,omitempty"`
	InterceptWarmup         *bool     `json:"intercept_warmup,omitempty"`
	ModelAllowlist          *[]string `json:"model_allowlist,omitempty"`
}

func (in AccountExtraInput) valid() bool {
	if in.WindowCostLimit != nil && *in.WindowCostLimit < 0 ||
		in.WindowCostStickyReserve != nil && *in.WindowCostStickyReserve < 0 ||
		in.DailyQuota != nil && *in.DailyQuota < 0 ||
		in.TotalQuota != nil && *in.TotalQuota < 0 ||
		in.MaxSessions != nil && *in.MaxSessions < 0 ||
		in.RPMLimit != nil && *in.RPMLimit < 0 {
		return false
	}
	return true
}

func (in AccountExtraInput) applyTo(dst map[string]any) {
	if in.WindowCostLimit != nil {
		dst["window_cost_limit"] = *in.WindowCostLimit
	}
	if in.WindowCostStickyReserve != nil {
		dst["window_cost_sticky_reserve"] = *in.WindowCostStickyReserve
	}
	if in.DailyQuota != nil {
		dst["daily_quota"] = *in.DailyQuota
	}
	if in.TotalQuota != nil {
		dst["total_quota"] = *in.TotalQuota
	}
	if in.MaxSessions != nil {
		dst["max_sessions"] = *in.MaxSessions
	}
	if in.RPMLimit != nil {
		dst["rpm_limit"] = *in.RPMLimit
	}
	if in.CodexCLIOnly != nil {
		dst["codex_cli_only"] = *in.CodexCLIOnly
	}
	if in.TLSFingerprint != nil {
		dst["tls_fingerprint"] = *in.TLSFingerprint
	}
	if in.InterceptWarmup != nil {
		dst["intercept_warmup"] = *in.InterceptWarmup
	}
	if in.ModelAllowlist != nil {
		dst["model_allowlist"] = *in.ModelAllowlist
	}
}

type AccountExtraWire struct {
	Codex5h                 *float64                  `json:"codex_5h_used_percent"`
	Codex5hReset            *string                   `json:"codex_5h_reset_at"`
	Codex7d                 *float64                  `json:"codex_7d_used_percent"`
	Codex7dReset            *string                   `json:"codex_7d_reset_at"`
	Kimi5h                  *float64                  `json:"kimi_5h_used_percent"`
	Kimi5hReset             *string                   `json:"kimi_5h_reset_at"`
	KimiWeek                *float64                  `json:"kimi_weekly_used_percent"`
	KimiWeekReset           *string                   `json:"kimi_weekly_reset_at"`
	Zhipu5h                 *float64                  `json:"zhipu_5h_used_percent"`
	Zhipu5hReset            *string                   `json:"zhipu_5h_reset_at"`
	ZhipuWeek               *float64                  `json:"zhipu_weekly_used_percent"`
	ZhipuWeekReset          *string                   `json:"zhipu_weekly_reset_at"`
	MiniMax5h               *float64                  `json:"minimax_5h_used_percent"`
	MiniMax5hReset          *string                   `json:"minimax_5h_reset_at"`
	MiniMaxWeek             *float64                  `json:"minimax_weekly_used_percent"`
	MiniMaxWeekReset        *string                   `json:"minimax_weekly_reset_at"`
	DeepseekBalance         *float64                  `json:"deepseek_balance"`
	DeepseekCurrency        string                    `json:"deepseek_balance_currency"`
	DeepseekAvailable       *bool                     `json:"deepseek_balance_available"`
	DeepseekBalances        []core.AccountBalanceItem `json:"deepseek_balances"`
	KimiBalance             *float64                  `json:"kimi_balance"`
	KimiCurrency            string                    `json:"kimi_balance_currency"`
	KimiAvailable           *bool                     `json:"kimi_balance_available"`
	KimiBalances            []core.AccountBalanceItem `json:"kimi_balances"`
	WindowCostLimit         *float64                  `json:"window_cost_limit"`
	WindowCostStickyReserve *float64                  `json:"window_cost_sticky_reserve"`
	DailyQuota              *float64                  `json:"daily_quota"`
	TotalQuota              *float64                  `json:"total_quota"`
	MaxSessions             *int                      `json:"max_sessions"`
	RPMLimit                *int                      `json:"rpm_limit"`
	CodexCLIOnly            bool                      `json:"codex_cli_only"`
	TLSFingerprint          bool                      `json:"tls_fingerprint"`
	InterceptWarmup         bool                      `json:"intercept_warmup"`
	ModelAllowlist          []string                  `json:"model_allowlist"`
}

func projectAccountExtra(e AccountExtraWire) *AccountExtraView {
	if e.WindowCostLimit == nil && e.WindowCostStickyReserve == nil &&
		e.DailyQuota == nil && e.TotalQuota == nil &&
		e.MaxSessions == nil && e.RPMLimit == nil &&
		!e.CodexCLIOnly && !e.TLSFingerprint && !e.InterceptWarmup &&
		len(e.ModelAllowlist) == 0 {
		return nil
	}
	return &AccountExtraView{
		WindowCostLimit:         e.WindowCostLimit,
		WindowCostStickyReserve: e.WindowCostStickyReserve,
		DailyQuota:              e.DailyQuota,
		TotalQuota:              e.TotalQuota,
		MaxSessions:             e.MaxSessions,
		RPMLimit:                e.RPMLimit,
		CodexCLIOnly:            e.CodexCLIOnly,
		TLSFingerprint:          e.TLSFingerprint,
		InterceptWarmup:         e.InterceptWarmup,
		ModelAllowlist:          e.ModelAllowlist,
	}
}

type AccountDetailWire struct {
	ProxyFallbackID   *int64                     `json:"proxy_fallback_origin_id"`
	Credentials       map[string]json.RawMessage `json:"credentials"`
	ID                int64                      `json:"id"`
	Name              string                     `json:"name"`
	Notes             *string                    `json:"notes"`
	Platform          string                     `json:"platform"`
	Type              string                     `json:"type"`
	CredentialsStatus map[string]bool            `json:"credentials_status"`
	ProxyID           *int64                     `json:"proxy_id"`
	Proxy             *struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Protocol string `json:"protocol"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
	} `json:"proxy"`
	GroupIDs               []int64              `json:"group_ids"`
	Groups                 []platform.NamedItem `json:"groups"`
	Concurrency            int                  `json:"concurrency"`
	CurrentConcurrency     int                  `json:"current_concurrency"`
	Priority               int                  `json:"priority"`
	RateMultiplier         float64              `json:"rate_multiplier"`
	Status                 string               `json:"status"`
	Schedulable            *bool                `json:"schedulable"`
	RateLimitedAt          *string              `json:"rate_limited_at"`
	RateLimitResetAt       *string              `json:"rate_limit_reset_at"`
	OverloadUntil          *string              `json:"overload_until"`
	TempUnschedulableUntil *string              `json:"temp_unschedulable_until"`
	LastErrorCode          string               `json:"last_error_code"`
	LastErrorKind          string               `json:"last_error_kind"`
	ErrorMessage           *string              `json:"error_message"`
	LastUsedAt             *string              `json:"last_used_at"`
	ExpiresAt              *int64               `json:"expires_at"`
	AutoPauseOnExpired     bool                 `json:"auto_pause_on_expired"`
	CreatedAt              string               `json:"created_at"`
	UpdatedAt              string               `json:"updated_at"`
	Extra                  AccountExtraWire     `json:"extra"`
}

type AccountAdminView struct {
	BaseURL                string                   `json:"base_url,omitempty"`
	ModelMapping           map[string]string        `json:"model_mapping,omitempty"`
	CustomHeaders          map[string]bool          `json:"custom_headers,omitempty"`
	Organization           string                   `json:"organization,omitempty"`
	Project                string                   `json:"project,omitempty"`
	Region                 string                   `json:"region,omitempty"`
	AccessKeyID            string                   `json:"access_key_id,omitempty"`
	ProjectID              string                   `json:"project_id,omitempty"`
	Location               string                   `json:"location,omitempty"`
	ID                     int64                    `json:"id"`
	Name                   string                   `json:"name"`
	Notes                  string                   `json:"notes"`
	Platform               string                   `json:"platform"`
	Type                   string                   `json:"type"`
	CredentialsStatus      map[string]bool          `json:"credentials_status"`
	ProxyFallbackID        *int64                   `json:"proxy_fallback_origin_id,omitempty"`
	ProxyID                *int64                   `json:"proxy_id"`
	ProxyName              string                   `json:"proxy_name,omitempty"`
	GroupIDs               []int64                  `json:"group_ids"`
	Groups                 []platform.NamedItem     `json:"groups"`
	Concurrency            int                      `json:"concurrency"`
	CurrentConcurrency     int                      `json:"current_concurrency"`
	ActivityKnown          *bool                    `json:"activity_known,omitempty"`
	Priority               int                      `json:"priority"`
	RateMultiplier         float64                  `json:"rate_multiplier"`
	Status                 string                   `json:"status"`
	Schedulable            bool                     `json:"schedulable"`
	Five                   *float64                 `json:"five_hours_used"`
	FiveResetAt            *string                  `json:"five_hours_reset_at,omitempty"`
	Week                   *float64                 `json:"week_used"`
	WeekResetAt            *string                  `json:"week_reset_at,omitempty"`
	RateLimitedAt          *string                  `json:"rate_limited_at,omitempty"`
	RateLimitResetAt       *string                  `json:"rate_limit_reset_at,omitempty"`
	OverloadUntil          *string                  `json:"overload_until,omitempty"`
	TempUnschedulableUntil *string                  `json:"temp_unschedulable_until,omitempty"`
	LastErrorCode          string                   `json:"last_error_code,omitempty"`
	LastErrorKind          string                   `json:"last_error_kind,omitempty"`
	LastUsedAt             *string                  `json:"last_used_at,omitempty"`
	ExpiresAt              *int64                   `json:"expires_at,omitempty"`
	AutoPauseOnExpired     bool                     `json:"auto_pause_on_expired"`
	Extra                  *AccountExtraView        `json:"extra,omitempty"`
	CreatedAt              string                   `json:"created_at"`
	UpdatedAt              string                   `json:"updated_at"`
	TodayRequests          *int64                   `json:"today_requests,omitempty"`
	TodayTokens            *int64                   `json:"today_tokens,omitempty"`
	TodayCost              *float64                 `json:"today_cost,omitempty"`
	TodayErrors            *int64                   `json:"today_errors,omitempty"`
	Balance                *core.AccountBalanceView `json:"balance,omitempty"`
}

func projectExtraBalance(plat string, extra AccountExtraWire) *core.AccountBalanceView {
	project := func(bal *float64, cur string, avail *bool, items []core.AccountBalanceItem) *core.AccountBalanceView {
		if bal == nil && len(items) == 0 {
			return nil
		}
		out := &core.AccountBalanceView{Currency: strings.ToUpper(strings.TrimSpace(cur)), Available: avail}
		if out.Currency == "" {
			out.Currency = "CNY"
		}
		if bal != nil {
			out.Balance = *bal
		} else if len(items) > 0 {
			out.Balance = items[0].Balance
			if items[0].Currency != "" {
				out.Currency = strings.ToUpper(strings.TrimSpace(items[0].Currency))
			}
		}
		if len(items) > 0 {
			clean := make([]core.AccountBalanceItem, 0, len(items))
			for _, item := range items {
				c := strings.ToUpper(strings.TrimSpace(item.Currency))
				if c == "" {
					c = out.Currency
				}
				clean = append(clean, core.AccountBalanceItem{Currency: c, Balance: item.Balance})
			}
			out.Balances = clean
		}
		return out
	}
	switch plat {
	case "deepseek":
		return project(extra.DeepseekBalance, extra.DeepseekCurrency, extra.DeepseekAvailable, extra.DeepseekBalances)
	case "kimi":
		return project(extra.KimiBalance, extra.KimiCurrency, extra.KimiAvailable, extra.KimiBalances)
	default:
		return nil
	}
}

func redactAccountError(a AccountDetailWire) (code, kind string) {
	code = strings.TrimSpace(a.LastErrorCode)
	kind = strings.TrimSpace(a.LastErrorKind)
	if code == "" && kind == "" && a.ErrorMessage != nil && strings.TrimSpace(*a.ErrorMessage) != "" {
		msg := strings.ToLower(*a.ErrorMessage)
		switch {
		case strings.Contains(msg, "429") || strings.Contains(msg, "rate limit"):
			code, kind = "429", "rate_limit"
		case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid_api_key"):
			code, kind = "401", "auth_failed"
		case strings.Contains(msg, "403") || strings.Contains(msg, "forbidden"):
			code, kind = "403", "forbidden"
		case strings.Contains(msg, "529") || strings.Contains(msg, "overload"):
			code, kind = "529", "overloaded"
		case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
			code, kind = "504", "timeout"
		default:
			code, kind = "upstream_error", "error"
		}
	}
	return code, kind
}

func ProjectAdminAccount(a AccountDetailWire) (AccountAdminView, error) {
	var baseURL, org, proj, region, accessKeyID, projectID, location string
	var modelMapping map[string]string
	var rawHeaders map[string]string
	_ = json.Unmarshal(a.Credentials["base_url"], &baseURL)
	_ = json.Unmarshal(a.Credentials["model_mapping"], &modelMapping)
	_ = json.Unmarshal(a.Credentials["custom_headers"], &rawHeaders)
	_ = json.Unmarshal(a.Credentials["organization"], &org)
	_ = json.Unmarshal(a.Credentials["project"], &proj)
	_ = json.Unmarshal(a.Credentials["region"], &region)
	_ = json.Unmarshal(a.Credentials["access_key_id"], &accessKeyID)
	_ = json.Unmarshal(a.Credentials["project_id"], &projectID)
	_ = json.Unmarshal(a.Credentials["location"], &location)
	if u, ok := platform.ExternalLink(baseURL); !ok || u.RawQuery != "" || u.Fragment != "" {
		baseURL = ""
	}
	var customHeaders map[string]bool
	if len(rawHeaders) > 0 {
		customHeaders = make(map[string]bool, len(rawHeaders))
		for k := range rawHeaders {
			if strings.TrimSpace(k) != "" {
				customHeaders[k] = true
			}
		}
	}
	if a.ID < 1 || a.Name == "" || a.Schedulable == nil {
		return AccountAdminView{}, platform.Fail(502, "invalid_upstream")
	}
	notes := ""
	if a.Notes != nil {
		notes = *a.Notes
	}
	credStatus := a.CredentialsStatus
	if credStatus == nil {
		credStatus = map[string]bool{}
	}
	groupIDs := a.GroupIDs
	if groupIDs == nil {
		groupIDs = []int64{}
	}
	groups := a.Groups
	if groups == nil {
		groups = []platform.NamedItem{}
	}
	if len(groupIDs) == 0 && len(groups) > 0 {
		for _, g := range groups {
			groupIDs = append(groupIDs, g.ID)
		}
	}
	proxyName := ""
	if a.Proxy != nil {
		proxyName = a.Proxy.Name
	}
	accType := a.Type
	if accType == "" {
		accType = "apikey"
	}
	five := platform.FirstNonNil(a.Extra.Codex5h, a.Extra.Kimi5h, a.Extra.Zhipu5h, a.Extra.MiniMax5h)
	fiveReset := platform.FirstNonNil(a.Extra.Codex5hReset, a.Extra.Kimi5hReset, a.Extra.Zhipu5hReset, a.Extra.MiniMax5hReset)
	week := platform.FirstNonNil(a.Extra.Codex7d, a.Extra.KimiWeek, a.Extra.ZhipuWeek, a.Extra.MiniMaxWeek)
	weekReset := platform.FirstNonNil(a.Extra.Codex7dReset, a.Extra.KimiWeekReset, a.Extra.ZhipuWeekReset, a.Extra.MiniMaxWeekReset)
	errCode, errKind := redactAccountError(a)
	return AccountAdminView{
		ID: a.ID, Name: a.Name, Notes: notes, Platform: a.Platform, Type: accType,
		BaseURL: baseURL, ModelMapping: modelMapping, CustomHeaders: customHeaders,
		Organization: org, Project: proj, Region: region, AccessKeyID: accessKeyID,
		ProjectID: projectID, Location: location,
		CredentialsStatus: credStatus, ProxyFallbackID: a.ProxyFallbackID, ProxyID: a.ProxyID, ProxyName: proxyName,
		GroupIDs: groupIDs, Groups: groups,
		Concurrency: a.Concurrency, CurrentConcurrency: a.CurrentConcurrency,
		Priority: a.Priority, RateMultiplier: a.RateMultiplier,
		Status: a.Status, Schedulable: *a.Schedulable,
		Five: five, FiveResetAt: fiveReset, Week: week, WeekResetAt: weekReset,
		RateLimitedAt: a.RateLimitedAt, RateLimitResetAt: a.RateLimitResetAt,
		OverloadUntil: a.OverloadUntil, TempUnschedulableUntil: a.TempUnschedulableUntil,
		LastErrorCode: errCode, LastErrorKind: errKind,
		LastUsedAt: a.LastUsedAt, ExpiresAt: a.ExpiresAt,
		AutoPauseOnExpired: a.AutoPauseOnExpired,
		Extra:              projectAccountExtra(a.Extra),
		CreatedAt:          a.CreatedAt, UpdatedAt: a.UpdatedAt,
		Balance: projectExtraBalance(a.Platform, a.Extra),
	}, nil
}

type accountWriteInput struct {
	Name               string             `json:"name,omitempty"`
	Notes              *string            `json:"notes,omitempty"`
	Platform           string             `json:"platform,omitempty"`
	Type               string             `json:"type,omitempty"`
	Credentials        map[string]any     `json:"credentials,omitempty"`
	Extra              *AccountExtraInput `json:"extra,omitempty"`
	ProxyID            *int64             `json:"proxy_id,omitempty"`
	ClearProxy         bool               `json:"clear_proxy,omitempty"`
	Concurrency        *int               `json:"concurrency,omitempty"`
	Priority           *int               `json:"priority,omitempty"`
	RateMultiplier     *float64           `json:"rate_multiplier,omitempty"`
	Status             string             `json:"status,omitempty"`
	Schedulable        *bool              `json:"schedulable,omitempty"`
	GroupIDs           *[]int64           `json:"group_ids,omitempty"`
	ExpiresAt          *int64             `json:"expires_at,omitempty"`
	ClearExpires       bool               `json:"clear_expires,omitempty"`
	AutoPauseOnExpired *bool              `json:"auto_pause_on_expired,omitempty"`
	ConfirmMixedRisk   bool               `json:"confirm_mixed_channel_risk,omitempty"`
}

func (in accountWriteInput) toBody(create bool) (map[string]any, error) {
	if create && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100 || !validAccountPlatform(in.Platform) || !validAccountType(in.Type) || len(in.Credentials) == 0) {
		return nil, platform.Fail(400, "invalid_input")
	}
	if in.Name != "" && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100) ||
		in.Platform != "" && !validAccountPlatform(in.Platform) ||
		in.Type != "" && !validAccountType(in.Type) ||
		in.Status != "" && in.Status != "active" && in.Status != "inactive" ||
		in.Concurrency != nil && (*in.Concurrency < 0 || *in.Concurrency > 10000) ||
		in.Priority != nil && (*in.Priority < -1000 || *in.Priority > 10000) ||
		in.RateMultiplier != nil && *in.RateMultiplier < 0 ||
		in.ProxyID != nil && *in.ProxyID < 1 ||
		in.Extra != nil && !in.Extra.valid() {
		return nil, platform.Fail(400, "invalid_input")
	}
	body := map[string]any{}
	if in.Name != "" {
		body["name"] = strings.TrimSpace(in.Name)
	}
	if in.Notes != nil {
		body["notes"] = *in.Notes
	}
	if in.Platform != "" {
		body["platform"] = in.Platform
	}
	if in.Type != "" {
		body["type"] = in.Type
	}
	if len(in.Credentials) > 0 {
		body["credentials"] = in.Credentials
	}
	if in.Extra != nil {
		extraMap := map[string]any{}
		in.Extra.applyTo(extraMap)
		if len(extraMap) > 0 {
			body["extra"] = extraMap
		}
	}
	if in.ClearProxy {
		body["proxy_id"] = nil
	} else if in.ProxyID != nil {
		body["proxy_id"] = *in.ProxyID
	}
	if in.Concurrency != nil {
		body["concurrency"] = *in.Concurrency
	}
	if in.Priority != nil {
		body["priority"] = *in.Priority
	}
	if in.RateMultiplier != nil {
		body["rate_multiplier"] = *in.RateMultiplier
	}
	if in.Status != "" {
		body["status"] = in.Status
	}
	if in.Schedulable != nil {
		body["schedulable"] = *in.Schedulable
	}
	if in.GroupIDs != nil {
		body["group_ids"] = *in.GroupIDs
	}
	if in.ClearExpires {
		body["expires_at"] = nil
	} else if in.ExpiresAt != nil {
		body["expires_at"] = *in.ExpiresAt
	}
	if in.AutoPauseOnExpired != nil {
		body["auto_pause_on_expired"] = *in.AutoPauseOnExpired
	}
	if in.ConfirmMixedRisk {
		body["confirm_mixed_channel_risk"] = true
	}
	if !create && len(body) == 0 {
		return nil, platform.Fail(400, "invalid_input")
	}
	return body, nil
}
