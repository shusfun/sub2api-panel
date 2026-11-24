package pool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type AccountExtraWire struct {
	Codex5h           *float64                  `json:"codex_5h_used_percent"`
	Codex5hReset      *string                   `json:"codex_5h_reset_at"`
	Codex7d           *float64                  `json:"codex_7d_used_percent"`
	Codex7dReset      *string                   `json:"codex_7d_reset_at"`
	Kimi5h            *float64                  `json:"kimi_5h_used_percent"`
	Kimi5hReset       *string                   `json:"kimi_5h_reset_at"`
	KimiWeek          *float64                  `json:"kimi_weekly_used_percent"`
	KimiWeekReset     *string                   `json:"kimi_weekly_reset_at"`
	Zhipu5h           *float64                  `json:"zhipu_5h_used_percent"`
	Zhipu5hReset      *string                   `json:"zhipu_5h_reset_at"`
	ZhipuWeek         *float64                  `json:"zhipu_weekly_used_percent"`
	ZhipuWeekReset    *string                   `json:"zhipu_weekly_reset_at"`
	MiniMax5h         *float64                  `json:"minimax_5h_used_percent"`
	MiniMax5hReset    *string                   `json:"minimax_5h_reset_at"`
	MiniMaxWeek       *float64                  `json:"minimax_weekly_used_percent"`
	MiniMaxWeekReset  *string                   `json:"minimax_weekly_reset_at"`
	DeepseekBalance   *float64                  `json:"deepseek_balance"`
	DeepseekCurrency  string                    `json:"deepseek_balance_currency"`
	DeepseekAvailable *bool                     `json:"deepseek_balance_available"`
	DeepseekBalances  []core.AccountBalanceItem `json:"deepseek_balances"`
	KimiBalance       *float64                  `json:"kimi_balance"`
	KimiCurrency      string                    `json:"kimi_balance_currency"`
	KimiAvailable     *bool                     `json:"kimi_balance_available"`
	KimiBalances      []core.AccountBalanceItem `json:"kimi_balances"`
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
	RateLimitResetAt       *string                  `json:"rate_limit_reset_at,omitempty"`
	OverloadUntil          *string                  `json:"overload_until,omitempty"`
	TempUnschedulableUntil *string                  `json:"temp_unschedulable_until,omitempty"`
	LastUsedAt             *string                  `json:"last_used_at,omitempty"`
	ExpiresAt              *int64                   `json:"expires_at,omitempty"`
	AutoPauseOnExpired     bool                     `json:"auto_pause_on_expired"`
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

func ProjectAdminAccount(a AccountDetailWire) (AccountAdminView, error) {
	var baseURL string
	var modelMapping map[string]string
	_ = json.Unmarshal(a.Credentials["base_url"], &baseURL)
	_ = json.Unmarshal(a.Credentials["model_mapping"], &modelMapping)
	if u, ok := platform.ExternalLink(baseURL); !ok || u.RawQuery != "" || u.Fragment != "" {
		baseURL = ""
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
	return AccountAdminView{
		ID: a.ID, Name: a.Name, Notes: notes, Platform: a.Platform, Type: accType, BaseURL: baseURL, ModelMapping: modelMapping,
		CredentialsStatus: credStatus, ProxyFallbackID: a.ProxyFallbackID, ProxyID: a.ProxyID, ProxyName: proxyName,
		GroupIDs: groupIDs, Groups: groups,
		Concurrency: a.Concurrency, CurrentConcurrency: a.CurrentConcurrency,
		Priority: a.Priority, RateMultiplier: a.RateMultiplier,
		Status: a.Status, Schedulable: *a.Schedulable,
		Five: five, FiveResetAt: fiveReset, Week: week, WeekResetAt: weekReset,
		RateLimitResetAt: a.RateLimitResetAt, OverloadUntil: a.OverloadUntil,
		TempUnschedulableUntil: a.TempUnschedulableUntil,
		LastUsedAt:             a.LastUsedAt, ExpiresAt: a.ExpiresAt,
		AutoPauseOnExpired: a.AutoPauseOnExpired,
		CreatedAt:          a.CreatedAt, UpdatedAt: a.UpdatedAt,
		Balance: projectExtraBalance(a.Platform, a.Extra),
	}, nil
}

type accountWriteInput struct {
	Name               string         `json:"name,omitempty"`
	Notes              *string        `json:"notes,omitempty"`
	Platform           string         `json:"platform,omitempty"`
	Type               string         `json:"type,omitempty"`
	Credentials        map[string]any `json:"credentials,omitempty"`
	ProxyID            *int64         `json:"proxy_id,omitempty"`
	ClearProxy         bool           `json:"clear_proxy,omitempty"`
	Concurrency        *int           `json:"concurrency,omitempty"`
	Priority           *int           `json:"priority,omitempty"`
	RateMultiplier     *float64       `json:"rate_multiplier,omitempty"`
	Status             string         `json:"status,omitempty"`
	Schedulable        *bool          `json:"schedulable,omitempty"`
	GroupIDs           *[]int64       `json:"group_ids,omitempty"`
	ExpiresAt          *int64         `json:"expires_at,omitempty"`
	ClearExpires       bool           `json:"clear_expires,omitempty"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired,omitempty"`
	ConfirmMixedRisk   bool           `json:"confirm_mixed_channel_risk,omitempty"`
}

func (in accountWriteInput) toBody(create bool) (map[string]any, error) {
	if create && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100 || strings.TrimSpace(in.Platform) == "" || strings.TrimSpace(in.Type) == "" || len(in.Credentials) == 0) {
		return nil, platform.Fail(400, "invalid_input")
	}
	if in.Name != "" && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100) ||
		in.Platform != "" && len(in.Platform) > 50 ||
		in.Type != "" && len(in.Type) > 40 ||
		in.Status != "" && in.Status != "active" && in.Status != "inactive" ||
		in.Concurrency != nil && (*in.Concurrency < 0 || *in.Concurrency > 10000) ||
		in.Priority != nil && (*in.Priority < -1000 || *in.Priority > 10000) ||
		in.RateMultiplier != nil && *in.RateMultiplier < 0 ||
		in.ProxyID != nil && *in.ProxyID < 1 {
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

type quotaWire struct {
	Usage   *float64   `json:"utilization"`
	Unknown bool       `json:"utilization_unknown"`
	Reset   *time.Time `json:"resets_at"`
}

type quotaView struct {
	Usage *float64   `json:"percent_used"`
	Reset *time.Time `json:"resets_at"`
}

type accountQuotaWire struct {
	Five             *quotaWire `json:"five_hour"`
	Week             *quotaWire `json:"seven_day"`
	SonnetWeek       *quotaWire `json:"seven_day_sonnet"`
	GeminiDaily      *quotaWire `json:"gemini_shared_daily"`
	GeminiProDaily   *quotaWire `json:"gemini_pro_daily"`
	GeminiFlashDaily *quotaWire `json:"gemini_flash_daily"`
	ThirtyDay        *quotaWire `json:"thirty_day"`
	SubscriptionTier string     `json:"subscription_tier"`
	NeedsReauth      bool       `json:"needs_reauth"`
	IsBanned         bool       `json:"is_banned"`
	NeedsVerify      bool       `json:"needs_verify"`
}

type accountQuotaView struct {
	Five             *quotaView `json:"five_hours"`
	Week             *quotaView `json:"week"`
	SonnetWeek       *quotaView `json:"sonnet_week,omitempty"`
	GeminiDaily      *quotaView `json:"gemini_daily,omitempty"`
	GeminiProDaily   *quotaView `json:"gemini_pro_daily,omitempty"`
	GeminiFlashDaily *quotaView `json:"gemini_flash_daily,omitempty"`
	ThirtyDay        *quotaView `json:"thirty_day,omitempty"`
	SubscriptionTier string     `json:"subscription_tier,omitempty"`
	NeedsReauth      bool       `json:"needs_reauth,omitempty"`
	IsBanned         bool       `json:"is_banned,omitempty"`
	NeedsVerify      bool       `json:"needs_verify,omitempty"`
}

type accountWire struct {
	platform.NamedItem
	Platform    string `json:"platform"`
	Status      string `json:"status"`
	Schedulable *bool  `json:"schedulable"`
}

func projectQuotaInfo(raw accountQuotaWire) accountQuotaView {
	project := func(window *quotaWire) *quotaView {
		if window == nil {
			return nil
		}
		usage := window.Usage
		if window.Unknown {
			usage = nil
		}
		return &quotaView{Usage: usage, Reset: window.Reset}
	}
	return accountQuotaView{
		Five: project(raw.Five), Week: project(raw.Week),
		SonnetWeek: project(raw.SonnetWeek), GeminiDaily: project(raw.GeminiDaily),
		GeminiProDaily: project(raw.GeminiProDaily), GeminiFlashDaily: project(raw.GeminiFlashDaily),
		ThirtyDay:        project(raw.ThirtyDay),
		SubscriptionTier: raw.SubscriptionTier, NeedsReauth: raw.NeedsReauth,
		IsBanned: raw.IsBanned, NeedsVerify: raw.NeedsVerify,
	}
}

func (h *handler) accountQuota(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	source := "passive"
	if r.URL.Query().Get("source") == "active" {
		source = "active"
	}
	raw, err := platform.Upstream[accountQuotaWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d/usage?source=%s", id, source), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, 200, projectQuotaInfo(raw))
	return nil
}

func (h *handler) scheduleAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		On *bool `json:"schedulable"`
	}](r)
	if err != nil {
		return err
	}
	if input.On == nil {
		return platform.Fail(400, "invalid_input")
	}
	path := fmt.Sprintf("/admin/accounts/%d", id)
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "POST", path+"/schedulable", a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	data, err := platform.Upstream[accountWire](r.Context(), h.s.Upstream, "GET", path, a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if data.ID != id || data.Schedulable == nil {
		return platform.Fail(502, "invalid_upstream")
	}
	if *data.Schedulable != *input.On {
		return platform.Fail(409, "schedule_changed")
	}
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}

func (h *handler) testAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Model string `json:"model_id,omitempty"`
	}](r)
	if err != nil || len(input.Model) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	stream := h.s.Upstream.WithTimeout(70 * time.Second)
	res, err := stream.Open(r.Context(), "POST", fmt.Sprintf("/api/v1/admin/accounts/%d/test", id), a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return platform.UpstreamStatus{Status: res.StatusCode}
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		return platform.Fail(502, "invalid_upstream")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 2048), 256<<10)
	start := time.Now()
	emitAccountTest(w, "running")
	for scanner.Scan() {
		frame, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		var evt struct {
			Type    string `json:"type"`
			Success bool   `json:"success"`
		}
		if json.Unmarshal([]byte(frame), &evt) == nil {
			if evt.Type == "test_complete" && evt.Success {
				emitAccountTest(w, "passed")
				slogAccountResult(r.Context(), id, true, start)
				return nil
			}
			if evt.Type == "error" || evt.Type == "test_complete" {
				break
			}
		}
	}
	emitAccountTest(w, "failed")
	slogAccountResult(r.Context(), id, false, start)
	return nil
}

func emitAccountTest(w http.ResponseWriter, state string) {
	fmt.Fprintf(w, "data: {\"state\":\"%s\"}\n\n", state)
	_ = http.NewResponseController(w).Flush()
}

func slogAccountResult(ctx context.Context, id int64, passed bool, start time.Time) {
	slog.Info("account test finished", "trace_id", platform.Meta(ctx).Trace, "account_id", id,
		"passed", passed, "elapsed_ms", time.Since(start).Milliseconds())
}
