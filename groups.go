package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	ID                       int64                 `json:"id"`
	Name                     string                `json:"name"`
	Description              *string               `json:"description"`
	Platform                 string                `json:"platform"`
	RateMultiplier           float64               `json:"rate_multiplier"`
	RPMLimit                 int                   `json:"rpm_limit"`
	IsExclusive              bool                  `json:"is_exclusive"`
	Status                   string                `json:"status"`
	SubscriptionType         string                `json:"subscription_type"`
	DailyLimitUSD            *float64              `json:"daily_limit_usd"`
	WeeklyLimitUSD           *float64              `json:"weekly_limit_usd"`
	MonthlyLimitUSD          *float64              `json:"monthly_limit_usd"`
	LongContextPricing       bool                  `json:"long_context_pricing_enabled"`
	PeakRateEnabled          bool                  `json:"peak_rate_enabled"`
	PeakStart                string                `json:"peak_start"`
	PeakEnd                  string                `json:"peak_end"`
	PeakRateMultiplier       float64               `json:"peak_rate_multiplier"`
	FallbackGroupID          *int64                `json:"fallback_group_id"`
	InvalidFallbackGroupID   *int64                `json:"fallback_group_id_on_invalid_request"`
	ClaudeCodeOnly           bool                  `json:"claude_code_only"`
	RequireOAuthOnly         bool                  `json:"require_oauth_only"`
	AllowMessagesDispatch    bool                  `json:"allow_messages_dispatch"`
	DefaultMappedModel       string                `json:"default_mapped_model"`
	MaxReasoningEffort       string                `json:"max_reasoning_effort"`
	ModelAllowlist           *modelAllowlistWire   `json:"model_allowlist"`
	ModelPricing             []channelModelPricing `json:"model_pricing"`
	AccountCount             int64                 `json:"account_count"`
	ActiveAccountCount       int64                 `json:"active_account_count"`
	RateLimitedAccountCount  int64                 `json:"rate_limited_account_count"`
	SortOrder                int                   `json:"sort_order"`
	CreatedAt                string                `json:"created_at"`
	UpdatedAt                string                `json:"updated_at"`
}

type adminGroupView struct {
	ID                       int64                 `json:"id"`
	Name                     string                `json:"name"`
	Description              string                `json:"description"`
	Platform                 string                `json:"platform"`
	RateMultiplier           float64               `json:"rate_multiplier"`
	RPMLimit                 int                   `json:"rpm_limit"`
	IsExclusive              bool                  `json:"is_exclusive"`
	Status                   string                `json:"status"`
	SubscriptionType         string                `json:"subscription_type"`
	DailyLimitUSD            *float64              `json:"daily_limit_usd"`
	WeeklyLimitUSD           *float64              `json:"weekly_limit_usd"`
	MonthlyLimitUSD          *float64              `json:"monthly_limit_usd"`
	LongContextPricing       bool                  `json:"long_context_pricing_enabled"`
	PeakRateEnabled          bool                  `json:"peak_rate_enabled"`
	PeakStart                string                `json:"peak_start"`
	PeakEnd                  string                `json:"peak_end"`
	PeakRateMultiplier       float64               `json:"peak_rate_multiplier"`
	FallbackGroupID          *int64                `json:"fallback_group_id"`
	InvalidFallbackGroupID   *int64                `json:"fallback_group_id_on_invalid_request"`
	ClaudeCodeOnly           bool                  `json:"claude_code_only"`
	RequireOAuthOnly         bool                  `json:"require_oauth_only"`
	AllowMessagesDispatch    bool                  `json:"allow_messages_dispatch"`
	DefaultMappedModel       string                `json:"default_mapped_model"`
	MaxReasoningEffort       string                `json:"max_reasoning_effort"`
	ModelAllowlist           modelAllowlistWire    `json:"model_allowlist"`
	ModelPricing             []channelModelPricing `json:"model_pricing"`
	AccountCount             int64                 `json:"account_count"`
	ActiveAccountCount       int64                 `json:"active_account_count"`
	RateLimitedAccountCount  int64                 `json:"rate_limited_account_count"`
	ErrorAccountCount        int64                 `json:"error_account_count"`
	ConcurrencyUsed          int                   `json:"concurrency_used"`
	ConcurrencyMax           int                   `json:"concurrency_max"`
	RPMUsed                  int                   `json:"rpm_used"`
	RPMMax                   int                   `json:"rpm_max"`
	Health                   string                `json:"health"`
	HealthScore              *float64              `json:"health_score"`
	TodayCost                float64               `json:"today_cost"`
	TotalCost                float64               `json:"total_cost"`
	SortOrder                int                   `json:"sort_order"`
	CreatedAt                string                `json:"created_at"`
	UpdatedAt                string                `json:"updated_at"`
}

func projectAdminGroup(g adminGroupWire) (adminGroupView, error) {
	if g.ID < 1 || g.Name == "" || g.Platform == "" || g.RateMultiplier < 0 {
		return adminGroupView{}, fail(502, "invalid_upstream")
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
		return nil, fail(400, "invalid_input")
	}
	if in.Name != "" && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100) ||
		in.Platform != "" && len(in.Platform) > 50 ||
		in.Status != "" && in.Status != "active" && in.Status != "inactive" ||
		in.SubscriptionType != "" && in.SubscriptionType != "standard" && in.SubscriptionType != "subscription" ||
		in.RateMultiplier != nil && *in.RateMultiplier < 0 ||
		in.RPMLimit != nil && *in.RPMLimit < 0 ||
		in.PeakRateMultiplier != nil && *in.PeakRateMultiplier < 0 ||
		in.FallbackGroupID != nil && *in.FallbackGroupID < 1 {
		return nil, fail(400, "invalid_input")
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
		return nil, fail(400, "invalid_input")
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
	ID                 int64                         `json:"id"`
	Name               string                        `json:"name"`
	Description        string                        `json:"description"`
	Status             string                        `json:"status"`
	BillingModelSource string                        `json:"billing_model_source"`
	RestrictModels     bool                          `json:"restrict_models"`
	GroupIDs           []int64                       `json:"group_ids"`
	ModelPricing       []channelModelPricing         `json:"model_pricing"`
	ModelMapping       map[string]map[string]string  `json:"model_mapping"`
	ApplyToAccountStat bool                          `json:"apply_pricing_to_account_stats"`
	CreatedAt          string                        `json:"created_at"`
	UpdatedAt          string                        `json:"updated_at"`
}

type channelInput struct {
	Name               string                        `json:"name,omitempty"`
	Description        *string                       `json:"description,omitempty"`
	Status             string                        `json:"status,omitempty"`
	BillingModelSource string                        `json:"billing_model_source,omitempty"`
	RestrictModels     *bool                         `json:"restrict_models,omitempty"`
	GroupIDs           *[]int64                      `json:"group_ids,omitempty"`
	ModelPricing       *[]channelModelPricing        `json:"model_pricing,omitempty"`
	ModelMapping       map[string]map[string]string  `json:"model_mapping,omitempty"`
	ApplyToAccountStat *bool                         `json:"apply_pricing_to_account_stats,omitempty"`
}

func (p *panel) groupRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/admin/groups", true, p.adminGroups)
	p.route(mux, "GET /api/admin/groups/all", true, p.adminAllGroups)
	p.route(mux, "PUT /api/admin/groups/sort-order", true, p.updateGroupSortOrder)
	p.route(mux, "GET /api/admin/groups/{id}", true, p.adminGroupDetail)
	p.route(mux, "POST /api/admin/groups", true, p.createAdminGroup)
	p.route(mux, "PUT /api/admin/groups/{id}", true, p.updateAdminGroup)
	p.route(mux, "DELETE /api/admin/groups/{id}", true, p.deleteAdminGroup)
	p.route(mux, "POST /api/admin/groups/{id}/duplicate", true, p.duplicateAdminGroup)
	p.route(mux, "GET /api/admin/groups/{id}/stats", true, p.adminGroupStats)
	p.route(mux, "GET /api/admin/groups/{id}/keys", true, p.adminGroupKeys)
	p.route(mux, "GET /api/admin/groups/{id}/model-candidates", true, p.adminGroupModelCandidates)
	p.route(mux, "GET /api/admin/groups/{id}/rate-multipliers", true, p.getGroupRateMultipliers)
	p.route(mux, "PUT /api/admin/groups/{id}/rate-multipliers", true, p.setGroupRateMultipliers)
	p.route(mux, "DELETE /api/admin/groups/{id}/rate-multipliers", true, p.clearGroupRateMultipliers)
	p.route(mux, "GET /api/admin/groups/{id}/composite-routes", true, p.listCompositeRoutes)
	p.route(mux, "POST /api/admin/groups/{id}/composite-routes", true, p.createCompositeRoute)
	p.route(mux, "POST /api/admin/groups/{id}/composite-routes/preview", true, p.previewCompositeRoute)
	p.route(mux, "PUT /api/admin/groups/{id}/composite-routes/{route}", true, p.updateCompositeRoute)
	p.route(mux, "DELETE /api/admin/groups/{id}/composite-routes/{route}", true, p.deleteCompositeRoute)
	p.route(mux, "GET /api/admin/channels", true, p.listChannels)
	p.route(mux, "GET /api/admin/channels/model-pricing", true, p.getChannelModelDefaultPricing)
	p.route(mux, "GET /api/admin/channels/sync-models", true, p.syncChannelPricingModels)
	p.route(mux, "GET /api/admin/channels/{id}", true, p.getChannel)
	p.route(mux, "POST /api/admin/channels", true, p.createChannel)
	p.route(mux, "PUT /api/admin/channels/{id}", true, p.updateChannel)
	p.route(mux, "DELETE /api/admin/channels/{id}", true, p.deleteChannel)
}

func (p *panel) enrichAdminGroups(r *http.Request, token string, items []adminGroupView) []adminGroupView {
	if len(items) == 0 {
		return items
	}
	capacities, capErr := upstream[[]struct {
		GroupID         int64 `json:"group_id"`
		ConcurrencyUsed int   `json:"concurrency_used"`
		ConcurrencyMax  int   `json:"concurrency_max"`
		RPMUsed         int   `json:"rpm_used"`
		RPMMax          int   `json:"rpm_max"`
	}](r.Context(), p.upstream, "GET", "/admin/groups/capacity-summary", token, "", "", nil)
	capMap := make(map[int64]struct {
		ConcUsed, ConcMax, RPMUsed, RPMMax int
	})
	if capErr == nil {
		for _, c := range capacities {
			capMap[c.GroupID] = struct{ ConcUsed, ConcMax, RPMUsed, RPMMax int }{c.ConcurrencyUsed, c.ConcurrencyMax, c.RPMUsed, c.RPMMax}
		}
	}
	usageRows, usageErr := upstream[[]struct {
		GroupID   int64   `json:"group_id"`
		TodayCost float64 `json:"today_cost"`
		TotalCost float64 `json:"total_cost"`
	}](r.Context(), p.upstream, "GET", "/admin/groups/usage-summary", token, "", "", nil)
	usageMap := make(map[int64]struct{ Today, Total float64 })
	if usageErr == nil {
		for _, u := range usageRows {
			usageMap[u.GroupID] = struct{ Today, Total float64 }{u.TodayCost, u.TotalCost}
		}
	}
	monitor, monErr := upstream[groupHealthWire](r.Context(), p.upstream, "GET", "/channel-monitor-v2/matrix?range=24h&group_by=platform_group", token, "", "", nil)
	healthMap := make(map[int64]struct {
		Health string
		Score  *float64
	})
	if monErr == nil {
		for _, item := range monitor.Items {
			if item.GroupID > 0 {
				healthMap[item.GroupID] = struct {
					Health string
					Score  *float64
				}{item.Health.Overall, item.Health.Score}
			}
		}
	}
	for i := range items {
		id := items[i].ID
		if c, ok := capMap[id]; ok {
			items[i].ConcurrencyUsed, items[i].ConcurrencyMax = c.ConcUsed, c.ConcMax
			items[i].RPMUsed, items[i].RPMMax = c.RPMUsed, c.RPMMax
		}
		if u, ok := usageMap[id]; ok {
			items[i].TodayCost, items[i].TotalCost = u.Today, u.Total
		}
		if h, ok := healthMap[id]; ok && h.Health != "" {
			items[i].Health, items[i].HealthScore = h.Health, h.Score
		}
	}
	return items
}

func (p *panel) adminGroups(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "search", "platform", "status", "is_exclusive", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/groups", page, params, projectAdminGroup)
	if err == nil {
		rows.Items = p.enrichAdminGroups(r, a.tokens.Access, rows.Items)
	}
	return send(w, 200, rows, err)
}

type groupOptionView struct {
	ID               int64    `json:"id"`
	Name             string   `json:"name"`
	Platform         string   `json:"platform"`
	RateMultiplier   float64  `json:"rate_multiplier"`
	Status           string   `json:"status"`
	SubscriptionType string   `json:"subscription_type"`
	IsExclusive      bool     `json:"is_exclusive"`
	AccountCount     int64    `json:"account_count"`
	ActiveAccounts   int64    `json:"active_account_count"`
	RateLimitedCount int64    `json:"rate_limited_account_count"`
	Health           string   `json:"health"`
	HealthScore      *float64 `json:"health_score"`
}

func (p *panel) adminAllGroups(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	query := url.Values{"include_inactive": {"true"}}
	if platform := strings.TrimSpace(r.URL.Query().Get("platform")); platform != "" {
		if len(platform) > 50 {
			return fail(400, "invalid_input")
		}
		query.Set("platform", platform)
	}
	rows, err := upstream[[]adminGroupWire](r.Context(), p.upstream, "GET", "/admin/groups/all?"+query.Encode(), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	projected, err := projectList(rows, projectAdminGroup)
	if err != nil {
		return err
	}
	projected = p.enrichAdminGroups(r, a.tokens.Access, projected)
	out := make([]groupOptionView, 0, len(projected))
	for _, g := range projected {
		out = append(out, groupOptionView{
			ID: g.ID, Name: g.Name, Platform: g.Platform, RateMultiplier: g.RateMultiplier,
			Status: g.Status, SubscriptionType: g.SubscriptionType, IsExclusive: g.IsExclusive,
			AccountCount: g.AccountCount, ActiveAccounts: g.ActiveAccountCount,
			RateLimitedCount: g.RateLimitedAccountCount, Health: g.Health, HealthScore: g.HealthScore,
		})
	}
	writeJSON(w, 200, out)
	return nil
}

func (p *panel) adminGroupDetail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := upstream[adminGroupWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/groups/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(raw)
	if err != nil {
		return err
	}
	enriched := p.enrichAdminGroups(r, a.tokens.Access, []adminGroupView{view})
	writeJSON(w, 200, enriched[0])
	return nil
}

func (p *panel) createAdminGroup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[groupInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(true)
	if err != nil {
		return err
	}
	created, err := upstream[adminGroupWire](r.Context(), p.upstream, "POST", "/admin/groups", a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(created)
	return send(w, 201, view, err)
}

func (p *panel) updateAdminGroup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[groupInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(false)
	if err != nil {
		return err
	}
	updated, err := upstream[adminGroupWire](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/groups/%d", id), a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(updated)
	return send(w, 200, view, err)
}

func (p *panel) deleteAdminGroup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) duplicateAdminGroup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Operation string `json:"operation_id,omitempty"`
	}](r)
	if err != nil || len(input.Operation) > 128 {
		return fail(400, "invalid_input")
	}
	created, err := upstream[adminGroupWire](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/groups/%d/duplicate", id), a.tokens.Access, "", input.Operation, struct{}{})
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(created)
	return send(w, 200, view, err)
}

func (p *panel) updateGroupSortOrder(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Updates []struct {
			ID        int64 `json:"id"`
			SortOrder int   `json:"sort_order"`
		} `json:"updates"`
	}](r)
	if err != nil || len(input.Updates) == 0 || len(input.Updates) > 200 {
		return fail(400, "invalid_input")
	}
	for _, item := range input.Updates {
		if item.ID < 1 {
			return fail(400, "invalid_input")
		}
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "PUT", "/admin/groups/sort-order", a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) adminGroupStats(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	stats, err := upstream[struct {
		TotalKeys     int64   `json:"total_api_keys"`
		ActiveKeys    int64   `json:"active_api_keys"`
		TotalRequests int64   `json:"total_requests"`
		TotalCost     float64 `json:"total_cost"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/groups/%d/stats", id), a.tokens.Access, "", "", nil)
	return send(w, 200, stats, err)
}

type adminGroupKeyView struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Masked    string   `json:"masked"`
	Status    string   `json:"status"`
	UserID    int64    `json:"user_id"`
	UserEmail string   `json:"user_email,omitempty"`
	Quota     float64  `json:"quota"`
	QuotaUsed float64  `json:"quota_used"`
	TodayCost *float64 `json:"today_actual_cost,omitempty"`
	TotalCost *float64 `json:"total_actual_cost,omitempty"`
}

func (p *panel) adminGroupKeys(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, fmt.Sprintf("/admin/groups/%d/api-keys", id), page, nil, func(k struct {
		keyWire
		User *struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}) (adminGroupKeyView, error) {
		if k.ID < 1 || k.Owner < 1 || k.Name == "" {
			return adminGroupKeyView{}, fail(502, "invalid_upstream")
		}
		masked := "••••••••"
		if len(k.Key) > 12 {
			masked += k.Key[len(k.Key)-4:]
		}
		email := ""
		if k.User != nil {
			email = k.User.Email
		}
		return adminGroupKeyView{
			ID: k.ID, Name: k.Name, Masked: masked, Status: k.Status,
			UserID: k.Owner, UserEmail: email, Quota: k.Quota, QuotaUsed: k.QuotaUsed,
		}, nil
	})
	return send(w, 200, rows, err)
}

func (p *panel) adminGroupModelCandidates(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	rawID := r.PathValue("id")
	id, err := parseNonNegativeID(rawID)
	if err != nil {
		return err
	}
	query := url.Values{}
	if platform := strings.TrimSpace(r.URL.Query().Get("platform")); platform != "" {
		if len(platform) > 50 {
			return fail(400, "invalid_input")
		}
		query.Set("platform", platform)
	}
	suffix := ""
	if len(query) > 0 {
		suffix = "?" + query.Encode()
	}
	res, err := upstream[struct {
		Models []string `json:"models"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/groups/%d/model-allowlist-candidates%s", id, suffix), a.tokens.Access, "", "", nil)
	if err == nil && res.Models == nil {
		res.Models = []string{}
	}
	return send(w, 200, res, err)
}

func parseNonNegativeID(raw string) (int64, error) {
	if raw == "0" {
		return 0, nil
	}
	var id int64
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, fail(400, "invalid_input")
		}
		id = id*10 + int64(ch-'0')
		if id > 1_000_000_000_000 {
			return 0, fail(400, "invalid_input")
		}
	}
	if raw == "" {
		return 0, fail(400, "invalid_input")
	}
	return id, nil
}

func (p *panel) getGroupRateMultipliers(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	rows, err := upstream[[]groupRateMultiplierRow](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/groups/%d/rate-multipliers", id), a.tokens.Access, "", "", nil)
	if err == nil && rows == nil {
		rows = []groupRateMultiplierRow{}
	}
	return send(w, 200, rows, err)
}

func (p *panel) setGroupRateMultipliers(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Entries []struct {
			UserID         int64   `json:"user_id"`
			RateMultiplier float64 `json:"rate_multiplier"`
		} `json:"entries"`
	}](r)
	if err != nil || len(input.Entries) == 0 || len(input.Entries) > 200 {
		return fail(400, "invalid_input")
	}
	for _, item := range input.Entries {
		if item.UserID < 1 || item.RateMultiplier < 0 {
			return fail(400, "invalid_input")
		}
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/groups/%d/rate-multipliers", id), a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) clearGroupRateMultipliers(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/groups/%d/rate-multipliers", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func validCompositeRouteInput(in compositeRouteInput) bool {
	return strings.TrimSpace(in.PublicModel) != "" && len(in.PublicModel) <= 128 &&
		(in.MatchType == "exact" || in.MatchType == "prefix") &&
		in.TargetPlatform != "" && in.TargetPlatform != "composite" && len(in.TargetPlatform) <= 50 &&
		len(in.UpstreamModel) <= 128 && len(in.Notes) <= 255 &&
		(in.Endpoint == "any" || in.Endpoint == "messages" || in.Endpoint == "count_tokens" || in.Endpoint == "responses" || in.Endpoint == "chat_completions" || in.Endpoint == "embeddings" || in.Endpoint == "images" || in.Endpoint == "gemini")
}

func (p *panel) listCompositeRoutes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	rows, err := upstream[[]compositeRouteView](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/groups/%d/composite-routes", id), a.tokens.Access, "", "", nil)
	if err == nil && rows == nil {
		rows = []compositeRouteView{}
	}
	return send(w, 200, rows, err)
}

func (p *panel) createCompositeRoute(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[compositeRouteInput](r)
	if err != nil || !validCompositeRouteInput(input) {
		return fail(400, "invalid_input")
	}
	created, err := upstream[compositeRouteView](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/groups/%d/composite-routes", id), a.tokens.Access, "", "", input)
	return send(w, 201, created, err)
}

func (p *panel) updateCompositeRoute(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	routeID, err := pathID(r, "route")
	if err != nil {
		return err
	}
	input, err := decodeBody[compositeRouteInput](r)
	if err != nil || !validCompositeRouteInput(input) {
		return fail(400, "invalid_input")
	}
	updated, err := upstream[compositeRouteView](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/groups/%d/composite-routes/%d", id, routeID), a.tokens.Access, "", "", input)
	return send(w, 200, updated, err)
}

func (p *panel) deleteCompositeRoute(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	routeID, err := pathID(r, "route")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/groups/%d/composite-routes/%d", id, routeID), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) previewCompositeRoute(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Model    string `json:"model"`
		Endpoint string `json:"endpoint"`
	}](r)
	if err != nil || strings.TrimSpace(input.Model) == "" || len(input.Model) > 128 {
		return fail(400, "invalid_input")
	}
	if input.Endpoint == "" {
		input.Endpoint = "any"
	}
	decision, err := upstream[struct {
		Matched        bool                `json:"matched"`
		Source         string              `json:"source"`
		GroupID        int64               `json:"group_id"`
		PublicModel    string              `json:"public_model"`
		TargetPlatform string              `json:"target_platform"`
		UpstreamModel  string              `json:"upstream_model"`
		Endpoint       string              `json:"endpoint"`
		Route          *compositeRouteView `json:"route,omitempty"`
		Reason         string              `json:"reason,omitempty"`
	}](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/groups/%d/composite-routes/preview", id), a.tokens.Access, "", "", input)
	return send(w, 200, decision, err)
}

func projectChannel(ch channelView) (channelView, error) {
	if ch.ID < 1 || ch.Name == "" {
		return channelView{}, fail(502, "invalid_upstream")
	}
	if ch.GroupIDs == nil {
		ch.GroupIDs = []int64{}
	}
	if ch.ModelPricing == nil {
		ch.ModelPricing = []channelModelPricing{}
	}
	if ch.ModelMapping == nil {
		ch.ModelMapping = map[string]map[string]string{}
	}
	return ch, nil
}

func (p *panel) listChannels(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "search", "status", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/channels", page, params, projectChannel)
	return send(w, 200, rows, err)
}

func (p *panel) getChannel(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	ch, err := upstream[channelView](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/channels/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectChannel(ch)
	return send(w, 200, view, err)
}

func (p *panel) createChannel(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[channelInput](r)
	if err != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 {
		return fail(400, "invalid_input")
	}
	ch, err := upstream[channelView](r.Context(), p.upstream, "POST", "/admin/channels", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectChannel(ch)
	return send(w, 201, view, err)
}

func (p *panel) updateChannel(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[channelInput](r)
	if err != nil || input.Name != "" && (strings.TrimSpace(input.Name) == "" || len(input.Name) > 100) ||
		input.Status != "" && input.Status != "active" && input.Status != "disabled" {
		return fail(400, "invalid_input")
	}
	ch, err := upstream[channelView](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/channels/%d", id), a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectChannel(ch)
	return send(w, 200, view, err)
}

func (p *panel) deleteChannel(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/channels/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) getChannelModelDefaultPricing(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	if model == "" || len(model) > 128 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Found           bool     `json:"found"`
		InputPrice      *float64 `json:"input_price,omitempty"`
		OutputPrice     *float64 `json:"output_price,omitempty"`
		CacheWritePrice *float64 `json:"cache_write_price,omitempty"`
		CacheReadPrice  *float64 `json:"cache_read_price,omitempty"`
	}](r.Context(), p.upstream, "GET", "/admin/channels/model-pricing?model="+url.QueryEscape(model), a.tokens.Access, "", "", nil)
	return send(w, 200, res, err)
}

func (p *panel) syncChannelPricingModels(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	if platform == "" || len(platform) > 50 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Models []string `json:"models"`
	}](r.Context(), p.upstream, "GET", "/admin/channels/pricing/sync-models?platform="+url.QueryEscape(platform), a.tokens.Access, "", "", nil)
	if err == nil && res.Models == nil {
		res.Models = []string{}
	}
	return send(w, 200, res, err)
}
