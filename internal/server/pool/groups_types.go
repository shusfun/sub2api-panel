package pool

import (
	"strings"

	"sub2api-panel/internal/platform"
)

type modelAllowlistWire struct {
	Enabled bool     `json:"enabled"`
	Models  []string `json:"models"`
}

type channelPricingInterval struct {
	ID                   int64    `json:"id,omitempty"`
	MinTokens            int      `json:"min_tokens"`
	MaxTokens            *int     `json:"max_tokens,omitempty"`
	TierLabel            string   `json:"tier_label,omitempty"`
	InputPrice           *float64 `json:"input_price,omitempty"`
	OutputPrice          *float64 `json:"output_price,omitempty"`
	CacheWritePrice      *float64 `json:"cache_write_price,omitempty"`
	CacheReadPrice       *float64 `json:"cache_read_price,omitempty"`
	InputMultiplier      *float64 `json:"input_multiplier,omitempty"`
	OutputMultiplier     *float64 `json:"output_multiplier,omitempty"`
	CacheWriteMultiplier *float64 `json:"cache_write_multiplier,omitempty"`
	CacheReadMultiplier  *float64 `json:"cache_read_multiplier,omitempty"`
	PerRequestPrice      *float64 `json:"per_request_price,omitempty"`
	SortOrder            int      `json:"sort_order"`
}

type channelTimePeriod struct {
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Multiplier float64 `json:"multiplier"`
}

type channelTimePricing struct {
	Timezone     string              `json:"timezone"`
	WeekdaysOnly bool                `json:"weekdays_only,omitempty"`
	Periods      []channelTimePeriod `json:"periods"`
}

type channelModelPricing struct {
	ID              int64                    `json:"id,omitempty"`
	Platform        string                   `json:"platform"`
	Models          []string                 `json:"models"`
	BillingMode     string                   `json:"billing_mode"`
	InputPrice      *float64                 `json:"input_price,omitempty"`
	OutputPrice     *float64                 `json:"output_price,omitempty"`
	CacheWritePrice *float64                 `json:"cache_write_price,omitempty"`
	CacheReadPrice  *float64                 `json:"cache_read_price,omitempty"`
	ImageInputPrice *float64                 `json:"image_input_price,omitempty"`
	ImageOutPrice   *float64                 `json:"image_output_price,omitempty"`
	PerRequestPrice *float64                 `json:"per_request_price,omitempty"`
	Intervals       []channelPricingInterval `json:"intervals,omitempty"`
	TimePricing     *channelTimePricing      `json:"time_pricing,omitempty"`
}

type adminGroupWire struct {
	ID                      int64                 `json:"id"`
	Name                    string                `json:"name"`
	Description             *string               `json:"description"`
	Platform                string                `json:"platform"`
	RateMultiplier          float64               `json:"rate_multiplier"`
	RPMLimit                int                   `json:"rpm_limit"`
	IsExclusive             bool                  `json:"is_exclusive"`
	Status                  string                `json:"status"`
	SubscriptionType        string                `json:"subscription_type"`
	DailyLimitUSD           *float64              `json:"daily_limit_usd"`
	WeeklyLimitUSD          *float64              `json:"weekly_limit_usd"`
	MonthlyLimitUSD         *float64              `json:"monthly_limit_usd"`
	LongContextPricing      bool                  `json:"long_context_pricing_enabled"`
	PeakRateEnabled         bool                  `json:"peak_rate_enabled"`
	PeakStart               string                `json:"peak_start"`
	PeakEnd                 string                `json:"peak_end"`
	PeakRateMultiplier      float64               `json:"peak_rate_multiplier"`
	FallbackGroupID         *int64                `json:"fallback_group_id"`
	InvalidFallbackGroupID  *int64                `json:"fallback_group_id_on_invalid_request"`
	ClaudeCodeOnly          bool                  `json:"claude_code_only"`
	RequireOAuthOnly        bool                  `json:"require_oauth_only"`
	AllowMessagesDispatch   bool                  `json:"allow_messages_dispatch"`
	DefaultMappedModel      string                `json:"default_mapped_model"`
	MaxReasoningEffort      string                `json:"max_reasoning_effort"`
	ModelAllowlist          *modelAllowlistWire   `json:"model_allowlist"`
	ModelPricing            []channelModelPricing `json:"model_pricing"`
	AccountCount            int64                 `json:"account_count"`
	ActiveAccountCount      int64                 `json:"active_account_count"`
	RateLimitedAccountCount int64                 `json:"rate_limited_account_count"`
	SortOrder               int                   `json:"sort_order"`
	CreatedAt               string                `json:"created_at"`
	UpdatedAt               string                `json:"updated_at"`
}

type adminGroupView struct {
	ID                      int64                 `json:"id"`
	Name                    string                `json:"name"`
	Description             string                `json:"description"`
	Platform                string                `json:"platform"`
	RateMultiplier          float64               `json:"rate_multiplier"`
	RPMLimit                int                   `json:"rpm_limit"`
	IsExclusive             bool                  `json:"is_exclusive"`
	Status                  string                `json:"status"`
	SubscriptionType        string                `json:"subscription_type"`
	DailyLimitUSD           *float64              `json:"daily_limit_usd"`
	WeeklyLimitUSD          *float64              `json:"weekly_limit_usd"`
	MonthlyLimitUSD         *float64              `json:"monthly_limit_usd"`
	LongContextPricing      bool                  `json:"long_context_pricing_enabled"`
	PeakRateEnabled         bool                  `json:"peak_rate_enabled"`
	PeakStart               string                `json:"peak_start"`
	PeakEnd                 string                `json:"peak_end"`
	PeakRateMultiplier      float64               `json:"peak_rate_multiplier"`
	FallbackGroupID         *int64                `json:"fallback_group_id"`
	InvalidFallbackGroupID  *int64                `json:"fallback_group_id_on_invalid_request"`
	ClaudeCodeOnly          bool                  `json:"claude_code_only"`
	RequireOAuthOnly        bool                  `json:"require_oauth_only"`
	AllowMessagesDispatch   bool                  `json:"allow_messages_dispatch"`
	DefaultMappedModel      string                `json:"default_mapped_model"`
	MaxReasoningEffort      string                `json:"max_reasoning_effort"`
	ModelAllowlist          modelAllowlistWire    `json:"model_allowlist"`
	ModelPricing            []channelModelPricing `json:"model_pricing"`
	AccountCount            int64                 `json:"account_count"`
	ActiveAccountCount      int64                 `json:"active_account_count"`
	RateLimitedAccountCount int64                 `json:"rate_limited_account_count"`
	ErrorAccountCount       int64                 `json:"error_account_count"`
	ConcurrencyUsed         int                   `json:"concurrency_used"`
	ConcurrencyMax          int                   `json:"concurrency_max"`
	RPMUsed                 int                   `json:"rpm_used"`
	RPMMax                  int                   `json:"rpm_max"`
	Health                  string                `json:"health"`
	HealthScore             *float64              `json:"health_score"`
	TodayCost               float64               `json:"today_cost"`
	TotalCost               float64               `json:"total_cost"`
	SortOrder               int                   `json:"sort_order"`
	CreatedAt               string                `json:"created_at"`
	UpdatedAt               string                `json:"updated_at"`
}

func projectAdminGroup(g adminGroupWire) (adminGroupView, error) {
	if g.ID < 1 || g.Name == "" || g.Platform == "" || g.RateMultiplier < 0 {
		return adminGroupView{}, platform.Fail(502, "invalid_upstream")
	}
	desc := ""
	if g.Description != nil {
		desc = *g.Description
	}
	allowlist := modelAllowlistWire{Models: []string{}}
	if g.ModelAllowlist != nil {
		allowlist.Enabled = g.ModelAllowlist.Enabled
		if g.ModelAllowlist.Models != nil {
			allowlist.Models = g.ModelAllowlist.Models
		}
	}
	pricing := g.ModelPricing
	if pricing == nil {
		pricing = []channelModelPricing{}
	}
	subType := g.SubscriptionType
	if subType == "" {
		subType = "standard"
	}
	errorCount := g.AccountCount - g.ActiveAccountCount - g.RateLimitedAccountCount
	if errorCount < 0 {
		errorCount = 0
	}
	return adminGroupView{
		ID: g.ID, Name: g.Name, Description: desc, Platform: g.Platform,
		RateMultiplier: g.RateMultiplier, RPMLimit: g.RPMLimit, IsExclusive: g.IsExclusive,
		Status: g.Status, SubscriptionType: subType,
		DailyLimitUSD: g.DailyLimitUSD, WeeklyLimitUSD: g.WeeklyLimitUSD, MonthlyLimitUSD: g.MonthlyLimitUSD,
		LongContextPricing: g.LongContextPricing,
		PeakRateEnabled: g.PeakRateEnabled, PeakStart: g.PeakStart, PeakEnd: g.PeakEnd, PeakRateMultiplier: g.PeakRateMultiplier,
		FallbackGroupID: g.FallbackGroupID, InvalidFallbackGroupID: g.InvalidFallbackGroupID,
		ClaudeCodeOnly: g.ClaudeCodeOnly, RequireOAuthOnly: g.RequireOAuthOnly,
		AllowMessagesDispatch: g.AllowMessagesDispatch, DefaultMappedModel: g.DefaultMappedModel,
		MaxReasoningEffort: g.MaxReasoningEffort, ModelAllowlist: allowlist, ModelPricing: pricing,
		AccountCount: g.AccountCount, ActiveAccountCount: g.ActiveAccountCount,
		RateLimitedAccountCount: g.RateLimitedAccountCount, ErrorAccountCount: errorCount,
		Health: "unavailable", SortOrder: g.SortOrder, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}, nil
}

type groupInput struct {
	Name                     string                 `json:"name,omitempty"`
	Description              *string                `json:"description,omitempty"`
	Platform                 string                 `json:"platform,omitempty"`
	RateMultiplier           *float64               `json:"rate_multiplier,omitempty"`
	RPMLimit                 *int                   `json:"rpm_limit,omitempty"`
	IsExclusive              *bool                  `json:"is_exclusive,omitempty"`
	Status                   string                 `json:"status,omitempty"`
	SubscriptionType         string                 `json:"subscription_type,omitempty"`
	DailyLimitUSD            *float64               `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD           *float64               `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD          *float64               `json:"monthly_limit_usd,omitempty"`
	ClearDailyLimit          bool                   `json:"clear_daily_limit,omitempty"`
	ClearWeeklyLimit         bool                   `json:"clear_weekly_limit,omitempty"`
	ClearMonthlyLimit        bool                   `json:"clear_monthly_limit,omitempty"`
	LongContextPricing       *bool                  `json:"long_context_pricing_enabled,omitempty"`
	PeakRateEnabled          *bool                  `json:"peak_rate_enabled,omitempty"`
	PeakStart                string                 `json:"peak_start,omitempty"`
	PeakEnd                  string                 `json:"peak_end,omitempty"`
	PeakRateMultiplier       *float64               `json:"peak_rate_multiplier,omitempty"`
	FallbackGroupID          *int64                 `json:"fallback_group_id,omitempty"`
	ClearFallbackGroup       bool                   `json:"clear_fallback_group,omitempty"`
	ClaudeCodeOnly           *bool                  `json:"claude_code_only,omitempty"`
	RequireOAuthOnly         *bool                  `json:"require_oauth_only,omitempty"`
	AllowMessagesDispatch    *bool                  `json:"allow_messages_dispatch,omitempty"`
	DefaultMappedModel       *string                `json:"default_mapped_model,omitempty"`
	MaxReasoningEffort       *string                `json:"max_reasoning_effort,omitempty"`
	ModelAllowlist           *modelAllowlistWire    `json:"model_allowlist,omitempty"`
	ModelPricing             *[]channelModelPricing `json:"model_pricing,omitempty"`
	CopyAccountsFromGroupIDs []int64                `json:"copy_accounts_from_group_ids,omitempty"`
}

func (in groupInput) toBody(create bool) (map[string]any, error) {
	if create && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100 || strings.TrimSpace(in.Platform) == "") {
		return nil, platform.Fail(400, "invalid_input")
	}
	if in.Name != "" && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100) ||
		in.Platform != "" && len(in.Platform) > 50 ||
		in.Status != "" && in.Status != "active" && in.Status != "inactive" ||
		in.SubscriptionType != "" && in.SubscriptionType != "standard" && in.SubscriptionType != "subscription" ||
		in.RateMultiplier != nil && *in.RateMultiplier < 0 ||
		in.RPMLimit != nil && *in.RPMLimit < 0 ||
		in.PeakRateMultiplier != nil && *in.PeakRateMultiplier < 0 ||
		in.FallbackGroupID != nil && *in.FallbackGroupID < 1 {
		return nil, platform.Fail(400, "invalid_input")
	}
	body := map[string]any{}
	if in.Name != "" {
		body["name"] = strings.TrimSpace(in.Name)
	}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	if in.Platform != "" {
		body["platform"] = in.Platform
	}
	if in.RateMultiplier != nil {
		body["rate_multiplier"] = *in.RateMultiplier
	}
	if in.RPMLimit != nil {
		body["rpm_limit"] = *in.RPMLimit
	}
	if in.IsExclusive != nil {
		body["is_exclusive"] = *in.IsExclusive
	}
	if in.Status != "" {
		body["status"] = in.Status
	}
	if in.SubscriptionType != "" {
		body["subscription_type"] = in.SubscriptionType
	}
	if in.ClearDailyLimit {
		body["daily_limit_usd"] = nil
	} else if in.DailyLimitUSD != nil {
		body["daily_limit_usd"] = *in.DailyLimitUSD
	}
	if in.ClearWeeklyLimit {
		body["weekly_limit_usd"] = nil
	} else if in.WeeklyLimitUSD != nil {
		body["weekly_limit_usd"] = *in.WeeklyLimitUSD
	}
	if in.ClearMonthlyLimit {
		body["monthly_limit_usd"] = nil
	} else if in.MonthlyLimitUSD != nil {
		body["monthly_limit_usd"] = *in.MonthlyLimitUSD
	}
	if in.LongContextPricing != nil {
		body["long_context_pricing_enabled"] = *in.LongContextPricing
	}
	if in.PeakRateEnabled != nil {
		body["peak_rate_enabled"] = *in.PeakRateEnabled
	}
	if in.PeakStart != "" {
		body["peak_start"] = in.PeakStart
	}
	if in.PeakEnd != "" {
		body["peak_end"] = in.PeakEnd
	}
	if in.PeakRateMultiplier != nil {
		body["peak_rate_multiplier"] = *in.PeakRateMultiplier
	}
	if in.ClearFallbackGroup {
		body["fallback_group_id"] = nil
	} else if in.FallbackGroupID != nil {
		body["fallback_group_id"] = *in.FallbackGroupID
	}
	if in.ClaudeCodeOnly != nil {
		body["claude_code_only"] = *in.ClaudeCodeOnly
	}
	if in.RequireOAuthOnly != nil {
		body["require_oauth_only"] = *in.RequireOAuthOnly
	}
	if in.AllowMessagesDispatch != nil {
		body["allow_messages_dispatch"] = *in.AllowMessagesDispatch
	}
	if in.DefaultMappedModel != nil {
		body["default_mapped_model"] = *in.DefaultMappedModel
	}
	if in.MaxReasoningEffort != nil {
		body["max_reasoning_effort"] = *in.MaxReasoningEffort
	}
	if in.ModelAllowlist != nil {
		body["model_allowlist"] = in.ModelAllowlist
	}
	if in.ModelPricing != nil {
		body["model_pricing"] = *in.ModelPricing
	}
	if len(in.CopyAccountsFromGroupIDs) > 0 {
		body["copy_accounts_from_group_ids"] = in.CopyAccountsFromGroupIDs
	}
	if !create && len(body) == 0 {
		return nil, platform.Fail(400, "invalid_input")
	}
	return body, nil
}

type compositeRouteView struct {
	ID             int64  `json:"id"`
	GroupID        int64  `json:"group_id"`
	PublicModel    string `json:"public_model"`
	MatchType      string `json:"match_type"`
	TargetPlatform string `json:"target_platform"`
	UpstreamModel  string `json:"upstream_model"`
	Endpoint       string `json:"endpoint"`
	Priority       int    `json:"priority"`
	Enabled        bool   `json:"enabled"`
	Notes          string `json:"notes"`
}

type compositeRouteInput struct {
	PublicModel    string `json:"public_model"`
	MatchType      string `json:"match_type"`
	TargetPlatform string `json:"target_platform"`
	UpstreamModel  string `json:"upstream_model,omitempty"`
	Endpoint       string `json:"endpoint"`
	Priority       int    `json:"priority"`
	Enabled        bool   `json:"enabled"`
	Notes          string `json:"notes,omitempty"`
}

type groupRateMultiplierRow struct {
	UserID         int64    `json:"user_id"`
	UserName       string   `json:"user_name"`
	UserEmail      string   `json:"user_email"`
	UserStatus     string   `json:"user_status"`
	RateMultiplier *float64 `json:"rate_multiplier"`
	RPMOverride    *int     `json:"rpm_override"`
}

type channelView struct {
	ID                 int64                        `json:"id"`
	Name               string                       `json:"name"`
	Description        string                       `json:"description"`
	Status             string                       `json:"status"`
	BillingModelSource string                       `json:"billing_model_source"`
	RestrictModels     bool                         `json:"restrict_models"`
	GroupIDs           []int64                      `json:"group_ids"`
	ModelPricing       []channelModelPricing        `json:"model_pricing"`
	ModelMapping       map[string]map[string]string `json:"model_mapping"`
	ApplyToAccountStat bool                         `json:"apply_pricing_to_account_stats"`
	CreatedAt          string                       `json:"created_at"`
	UpdatedAt          string                       `json:"updated_at"`
}

type channelInput struct {
	Name               string                       `json:"name,omitempty"`
	Description        *string                      `json:"description,omitempty"`
	Status             string                       `json:"status,omitempty"`
	BillingModelSource string                       `json:"billing_model_source,omitempty"`
	RestrictModels     *bool                        `json:"restrict_models,omitempty"`
	GroupIDs           *[]int64                     `json:"group_ids,omitempty"`
	ModelPricing       *[]channelModelPricing       `json:"model_pricing,omitempty"`
	ModelMapping       map[string]map[string]string `json:"model_mapping,omitempty"`
	ApplyToAccountStat *bool                        `json:"apply_pricing_to_account_stats,omitempty"`
}

type groupHealthWire struct {
	Items []struct {
		GroupID int64 `json:"group_id"`
		Health  struct {
			Overall string   `json:"overall"`
			Score   *float64 `json:"score"`
		} `json:"health"`
	} `json:"items"`
}
