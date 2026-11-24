package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type accountBalanceItem struct {
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
}

type accountBalanceView struct {
	Kind      string               `json:"kind,omitempty"`
	Stale     bool                 `json:"stale,omitempty"`
	Currency  string               `json:"currency"`
	Balance   float64              `json:"balance"`
	Available *bool                `json:"available,omitempty"`
	Balances  []accountBalanceItem `json:"balances,omitempty"`
	FetchedAt *int64               `json:"fetched_at,omitempty"`
	Billing   *accountBillingView  `json:"billing,omitempty"`
}

type accountBillingView struct {
	Effective float64   `json:"effective_multiplier"`
	Observed  time.Time `json:"observed_at"`
	Stale     bool      `json:"stale,omitempty"`
}

type accountExtraWire struct {
	Codex5h           *float64             `json:"codex_5h_used_percent"`
	Codex5hReset      *string              `json:"codex_5h_reset_at"`
	Codex7d           *float64             `json:"codex_7d_used_percent"`
	Codex7dReset      *string              `json:"codex_7d_reset_at"`
	Kimi5h            *float64             `json:"kimi_5h_used_percent"`
	Kimi5hReset       *string              `json:"kimi_5h_reset_at"`
	KimiWeek          *float64             `json:"kimi_weekly_used_percent"`
	KimiWeekReset     *string              `json:"kimi_weekly_reset_at"`
	Zhipu5h           *float64             `json:"zhipu_5h_used_percent"`
	Zhipu5hReset      *string              `json:"zhipu_5h_reset_at"`
	ZhipuWeek         *float64             `json:"zhipu_weekly_used_percent"`
	ZhipuWeekReset    *string              `json:"zhipu_weekly_reset_at"`
	MiniMax5h         *float64             `json:"minimax_5h_used_percent"`
	MiniMax5hReset    *string              `json:"minimax_5h_reset_at"`
	MiniMaxWeek       *float64             `json:"minimax_weekly_used_percent"`
	MiniMaxWeekReset  *string              `json:"minimax_weekly_reset_at"`
	DeepseekBalance   *float64             `json:"deepseek_balance"`
	DeepseekCurrency  string               `json:"deepseek_balance_currency"`
	DeepseekAvailable *bool                `json:"deepseek_balance_available"`
	DeepseekBalances  []accountBalanceItem `json:"deepseek_balances"`
	KimiBalance       *float64             `json:"kimi_balance"`
	KimiCurrency      string               `json:"kimi_balance_currency"`
	KimiAvailable     *bool                `json:"kimi_balance_available"`
	KimiBalances      []accountBalanceItem `json:"kimi_balances"`
}

type accountDetailWire struct {
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
	GroupIDs               []int64          `json:"group_ids"`
	Groups                 []namedItem      `json:"groups"`
	Concurrency            int              `json:"concurrency"`
	CurrentConcurrency     int              `json:"current_concurrency"`
	Priority               int              `json:"priority"`
	RateMultiplier         float64          `json:"rate_multiplier"`
	Status                 string           `json:"status"`
	Schedulable            *bool            `json:"schedulable"`
	RateLimitedAt          *string          `json:"rate_limited_at"`
	RateLimitResetAt       *string          `json:"rate_limit_reset_at"`
	OverloadUntil          *string          `json:"overload_until"`
	TempUnschedulableUntil *string          `json:"temp_unschedulable_until"`
	LastUsedAt             *string          `json:"last_used_at"`
	ExpiresAt              *int64           `json:"expires_at"`
	AutoPauseOnExpired     bool             `json:"auto_pause_on_expired"`
	CreatedAt              string           `json:"created_at"`
	UpdatedAt              string           `json:"updated_at"`
	Extra                  accountExtraWire `json:"extra"`
}

type accountAdminView struct {
	BaseURL                string              `json:"base_url,omitempty"`
	ModelMapping           map[string]string   `json:"model_mapping,omitempty"`
	ID                     int64               `json:"id"`
	Name                   string              `json:"name"`
	Notes                  string              `json:"notes"`
	Platform               string              `json:"platform"`
	Type                   string              `json:"type"`
	CredentialsStatus      map[string]bool     `json:"credentials_status"`
	ProxyFallbackID        *int64              `json:"proxy_fallback_origin_id,omitempty"`
	ProxyID                *int64              `json:"proxy_id"`
	ProxyName              string              `json:"proxy_name,omitempty"`
	GroupIDs               []int64             `json:"group_ids"`
	Groups                 []namedItem         `json:"groups"`
	Concurrency            int                 `json:"concurrency"`
	CurrentConcurrency     int                 `json:"current_concurrency"`
	ActivityKnown          *bool               `json:"activity_known,omitempty"`
	Priority               int                 `json:"priority"`
	RateMultiplier         float64             `json:"rate_multiplier"`
	Status                 string              `json:"status"`
	Schedulable            bool                `json:"schedulable"`
	Five                   *float64            `json:"five_hours_used"`
	FiveResetAt            *string             `json:"five_hours_reset_at,omitempty"`
	Week                   *float64            `json:"week_used"`
	WeekResetAt            *string             `json:"week_reset_at,omitempty"`
	RateLimitResetAt       *string             `json:"rate_limit_reset_at,omitempty"`
	OverloadUntil          *string             `json:"overload_until,omitempty"`
	TempUnschedulableUntil *string             `json:"temp_unschedulable_until,omitempty"`
	LastUsedAt             *string             `json:"last_used_at,omitempty"`
	ExpiresAt              *int64              `json:"expires_at,omitempty"`
	AutoPauseOnExpired     bool                `json:"auto_pause_on_expired"`
	CreatedAt              string              `json:"created_at"`
	UpdatedAt              string              `json:"updated_at"`
	TodayRequests          *int64              `json:"today_requests,omitempty"`
	TodayTokens            *int64              `json:"today_tokens,omitempty"`
	TodayCost              *float64            `json:"today_cost,omitempty"`
	TodayErrors            *int64              `json:"today_errors,omitempty"`
	Balance                *accountBalanceView `json:"balance,omitempty"`
}

func firstNonNil[T any](values ...*T) *T {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func projectExtraBalance(platform string, extra accountExtraWire) *accountBalanceView {
	project := func(bal *float64, cur string, avail *bool, items []accountBalanceItem) *accountBalanceView {
		if bal == nil && len(items) == 0 {
			return nil
		}
		out := &accountBalanceView{Currency: strings.ToUpper(strings.TrimSpace(cur)), Available: avail}
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
			clean := make([]accountBalanceItem, 0, len(items))
			for _, item := range items {
				c := strings.ToUpper(strings.TrimSpace(item.Currency))
				if c == "" {
					c = out.Currency
				}
				clean = append(clean, accountBalanceItem{Currency: c, Balance: item.Balance})
			}
			out.Balances = clean
		}
		return out
	}
	switch platform {
	case "deepseek":
		return project(extra.DeepseekBalance, extra.DeepseekCurrency, extra.DeepseekAvailable, extra.DeepseekBalances)
	case "kimi":
		return project(extra.KimiBalance, extra.KimiCurrency, extra.KimiAvailable, extra.KimiBalances)
	default:
		return nil
	}
}

func projectAdminAccount(a accountDetailWire) (accountAdminView, error) {
	var baseURL string
	var modelMapping map[string]string
	_ = json.Unmarshal(a.Credentials["base_url"], &baseURL)
	_ = json.Unmarshal(a.Credentials["model_mapping"], &modelMapping)
	if u, ok := externalLink(baseURL); !ok || u.RawQuery != "" || u.Fragment != "" {
		baseURL = ""
	}

	if a.ID < 1 || a.Name == "" || a.Schedulable == nil {
		return accountAdminView{}, fail(502, "invalid_upstream")
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
		groups = []namedItem{}
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
	five := firstNonNil(a.Extra.Codex5h, a.Extra.Kimi5h, a.Extra.Zhipu5h, a.Extra.MiniMax5h)
	fiveReset := firstNonNil(a.Extra.Codex5hReset, a.Extra.Kimi5hReset, a.Extra.Zhipu5hReset, a.Extra.MiniMax5hReset)
	week := firstNonNil(a.Extra.Codex7d, a.Extra.KimiWeek, a.Extra.ZhipuWeek, a.Extra.MiniMaxWeek)
	weekReset := firstNonNil(a.Extra.Codex7dReset, a.Extra.KimiWeekReset, a.Extra.ZhipuWeekReset, a.Extra.MiniMaxWeekReset)
	return accountAdminView{
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
		return nil, fail(400, "invalid_input")
	}
	if in.Name != "" && (strings.TrimSpace(in.Name) == "" || len(in.Name) > 100) ||
		in.Platform != "" && len(in.Platform) > 50 ||
		in.Type != "" && len(in.Type) > 40 ||
		in.Status != "" && in.Status != "active" && in.Status != "inactive" ||
		in.Concurrency != nil && (*in.Concurrency < 0 || *in.Concurrency > 10000) ||
		in.Priority != nil && (*in.Priority < -1000 || *in.Priority > 10000) ||
		in.RateMultiplier != nil && *in.RateMultiplier < 0 ||
		in.ProxyID != nil && *in.ProxyID < 1 {
		return nil, fail(400, "invalid_input")
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
		return nil, fail(400, "invalid_input")
	}
	return body, nil
}

func (p *panel) accountRoutes(mux *http.ServeMux) {
	p.route(mux, "POST /api/admin/accounts/sync/crs/preview", true, p.syncCRSAccounts)
	p.route(mux, "POST /api/admin/accounts/sync/crs", true, p.syncCRSAccounts)
	p.route(mux, "POST /api/admin/accounts/data/export", true, p.exportAccountData)
	p.route(mux, "POST /api/admin/accounts/data/import", true, p.importAccountData)
	p.route(mux, "GET /api/admin/accounts", true, p.adminAccountsList)
	p.route(mux, "GET /api/admin/accounts/stream", true, p.accountActivityStream)
	p.route(mux, "GET /api/admin/accounts/batch-targets", true, p.filteredAccountTargets)
	p.route(mux, "GET /api/admin/accounts/{id}", true, p.adminAccountDetail)
	p.route(mux, "POST /api/admin/accounts", true, p.createAdminAccount)
	p.route(mux, "PUT /api/admin/accounts/{id}", true, p.updateAdminAccount)
	p.route(mux, "DELETE /api/admin/accounts/{id}", true, p.deleteAdminAccount)
	p.route(mux, "POST /api/admin/accounts/{id}/duplicate", true, p.duplicateAdminAccount)
	p.route(mux, "POST /api/admin/accounts/check-mixed-channel", true, p.checkMixedChannel)
	p.route(mux, "GET /api/admin/accounts/{id}/quota", true, p.accountQuota)
	p.route(mux, "POST /api/admin/accounts/quota/batch", true, p.batchAccountQuota)
	p.route(mux, "POST /api/admin/accounts/{id}/balance", true, p.refreshAccountBalance)
	p.route(mux, "GET /api/admin/accounts/{id}/balance-settings", true, p.accountBalanceSettings)
	p.route(mux, "PUT /api/admin/accounts/{id}/balance-settings", true, p.accountBalanceSettings)
	p.route(mux, "POST /api/admin/accounts/{id}/balance-settings", true, p.accountBalanceSettings)
	p.route(mux, "GET /api/admin/accounts/{id}/stats", true, p.adminAccountStats)
	p.route(mux, "GET /api/admin/accounts/{id}/models", true, p.adminAccountModels)
	p.route(mux, "POST /api/admin/accounts/{id}/models/sync", true, p.syncAdminAccountModels)
	p.route(mux, "POST /api/admin/accounts/{id}/revert-proxy-fallback", true, p.revertAccountProxy)
	p.route(mux, "GET /api/admin/accounts/{id}/scheduled-tests", true, p.scheduledAccountTests)
	p.route(mux, "POST /api/admin/accounts/{id}/scheduled-tests", true, p.editAccountTestPlan)
	p.route(mux, "GET /api/admin/accounts/{id}/scheduled-tests/{plan}/results", true, p.editAccountTestPlan)
	p.route(mux, "PUT /api/admin/accounts/{id}/scheduled-tests/{plan}", true, p.editAccountTestPlan)
	p.route(mux, "DELETE /api/admin/accounts/{id}/scheduled-tests/{plan}", true, p.editAccountTestPlan)
	p.route(mux, "POST /api/admin/accounts/{id}/test", true, p.testAccount)
	p.route(mux, "PUT /api/admin/accounts/{id}/scheduling", true, p.scheduleAccount)
	p.route(mux, "POST /api/admin/accounts/{id}/recover", true, p.accountAction("recover-state"))
	p.route(mux, "POST /api/admin/accounts/{id}/clear-error", true, p.accountAction("clear-error"))
	p.route(mux, "POST /api/admin/accounts/{id}/clear-rate-limit", true, p.accountAction("clear-rate-limit"))
	p.route(mux, "POST /api/admin/accounts/{id}/reset-quota", true, p.accountAction("reset-quota"))
	p.route(mux, "POST /api/admin/accounts/{id}/refresh", true, p.refreshAdminAccount)
	p.route(mux, "DELETE /api/admin/accounts/{id}/temp-unschedulable", true, p.clearTempUnschedulable)
	p.route(mux, "POST /api/admin/accounts/bulk-update", true, p.bulkUpdateAccounts)
	p.route(mux, "POST /api/admin/accounts/batch-delete", true, p.batchDeleteAccounts)
	p.route(mux, "POST /api/admin/accounts/batch-clear-error", true, p.batchAccountIDsAction("/admin/accounts/batch-clear-error"))
	p.route(mux, "POST /api/admin/accounts/batch-refresh", true, p.batchAccountIDsAction("/admin/accounts/batch-refresh"))
	p.route(mux, "POST /api/admin/accounts/oauth/start", true, p.oauthStart)
	p.route(mux, "POST /api/admin/accounts/oauth/complete", true, p.oauthComplete)
	p.route(mux, "POST /api/admin/accounts/import", true, p.importAccounts)
}

func (p *panel) adminAccountsList(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	result, err := p.fetchAccountPage(r, a.tokens.Access)
	return send(w, 200, result, err)
}

func (p *panel) fetchAccountPage(r *http.Request, token string) (pageResult[accountAdminView], error) {
	if r.URL.Query().Get("sort_by") == "activity" {
		return p.accountActivityPage(r, token)
	}
	page, params, err := listQuery(r, "search", "platform", "type", "status", "group", "sort_by", "sort_order")
	if err != nil {
		return pageResult[accountAdminView]{}, err
	}
	var proxyFilter int64
	if rawProxy := strings.TrimSpace(r.URL.Query().Get("proxy_id")); rawProxy != "" {
		proxyFilter, err = strconv.ParseInt(rawProxy, 10, 64)
		if err != nil || proxyFilter < 1 {
			return pageResult[accountAdminView]{}, fail(400, "invalid_input")
		}
	}
	if proxyFilter > 0 {
		all := make([]accountAdminView, 0)
		for cursor := 1; cursor <= 50; cursor++ {
			rows, qErr := queryPage(r.Context(), p.upstream, token, "/admin/accounts", cursor, params, projectAdminAccount)
			if qErr != nil {
				return pageResult[accountAdminView]{}, qErr
			}
			for _, item := range rows.Items {
				if item.ProxyID != nil && *item.ProxyID == proxyFilter {
					all = append(all, item)
				}
			}
			if !rows.More {
				break
			}
		}
		start := min((page-1)*pageSize, len(all))
		end := min(start+pageSize, len(all))
		pages := (len(all) + pageSize - 1) / pageSize
		p.withDayStats(r.Context(), token, all[start:end])
		p.applyBalanceSnapshots(all[start:end])
		return pageResult[accountAdminView]{Items: all[start:end], More: end < len(all), Pages: pages}, nil
	}
	result, err := queryPage(r.Context(), p.upstream, token, "/admin/accounts", page, params, projectAdminAccount)
	if err == nil {
		p.withDayStats(r.Context(), token, result.Items)
		p.applyBalanceSnapshots(result.Items)
	}
	return result, err
}

func (p *panel) adminAccountDetail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := upstream[accountDetailWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectAdminAccount(raw)
	views := []accountAdminView{view}
	p.applyBalanceSnapshots(views)
	return send(w, 200, views[0], err)
}

func (p *panel) createAdminAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[accountWriteInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(true)
	if err != nil {
		return err
	}
	raw, err := upstream[accountDetailWire](r.Context(), p.upstream, "POST", "/admin/accounts", a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminAccount(raw)
	return send(w, 201, view, err)
}

func (p *panel) updateAdminAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[accountWriteInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(false)
	if err != nil {
		return err
	}
	if len(input.Credentials) > 0 {
		current, err := upstream[accountDetailWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		if current.ID != id {
			return fail(502, "invalid_upstream")
		}
		merged := make(map[string]any, len(current.Credentials)+len(input.Credentials))
		for key, value := range current.Credentials {
			merged[key] = value
		}
		for key, value := range input.Credentials {
			merged[key] = value
		}
		body["credentials"] = merged
	}
	raw, err := upstream[accountDetailWire](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", id), a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminAccount(raw)
	return send(w, 200, view, err)
}

func (p *panel) deleteAdminAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/accounts/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) duplicateAdminAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
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
	raw, err := upstream[accountDetailWire](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/accounts/%d/duplicate", id), a.tokens.Access, "", input.Operation, struct{}{})
	if err != nil {
		return err
	}
	view, err := projectAdminAccount(raw)
	return send(w, 200, view, err)
}

func (p *panel) checkMixedChannel(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Platform  string  `json:"platform"`
		GroupIDs  []int64 `json:"group_ids"`
		AccountID *int64  `json:"account_id,omitempty"`
	}](r)
	if err != nil || input.Platform == "" || len(input.GroupIDs) == 0 || len(input.GroupIDs) > 100 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		HasRisk bool   `json:"has_risk"`
		Message string `json:"message,omitempty"`
	}](r.Context(), p.upstream, "POST", "/admin/accounts/check-mixed-channel", a.tokens.Access, "", "", input)
	return send(w, 200, res, err)
}

func (p *panel) accountAction(action string) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		raw, err := upstream[accountDetailWire](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/accounts/%d/%s", id, action), a.tokens.Access, "", "", struct{}{})
		if err != nil {
			return err
		}
		view, err := projectAdminAccount(raw)
		return send(w, 200, view, err)
	}
}

func (p *panel) refreshAdminAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[map[string]any](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/accounts/%d/refresh", id), a.tokens.Access, "", "", struct{}{})
	if err != nil {
		return err
	}
	raw, err := upstream[accountDetailWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectAdminAccount(raw)
	return send(w, 200, view, err)
}

func (p *panel) clearTempUnschedulable(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/accounts/%d/temp-unschedulable", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) batchAccountQuota(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		AccountIDs []int64 `json:"account_ids"`
		Force      bool    `json:"force,omitempty"`
	}](r)
	if err != nil || len(input.AccountIDs) == 0 || len(input.AccountIDs) > 100 {
		return fail(400, "invalid_input")
	}
	for _, id := range input.AccountIDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	raw, err := upstream[struct {
		Usage  map[string]accountQuotaWire `json:"usage"`
		Errors map[string]string           `json:"errors"`
	}](r.Context(), p.upstream, "POST", "/admin/accounts/usage/batch", a.tokens.Access, "", "", map[string]any{
		"account_ids": input.AccountIDs,
		"force":       input.Force,
	})
	if err != nil {
		return err
	}
	quotas := make(map[string]accountQuotaView, len(raw.Usage))
	for id, item := range raw.Usage {
		if parsed, pErr := strconv.ParseInt(id, 10, 64); pErr == nil && parsed > 0 {
			quotas[id] = projectQuotaInfo(item)
		}
	}
	unavailable := make([]int64, 0, len(raw.Errors))
	for id := range raw.Errors {
		if parsed, pErr := strconv.ParseInt(id, 10, 64); pErr == nil && parsed > 0 {
			unavailable = append(unavailable, parsed)
		}
	}
	writeJSON(w, 200, struct {
		Quotas      map[string]accountQuotaView `json:"quotas"`
		Unavailable []int64                     `json:"unavailable"`
	}{Quotas: quotas, Unavailable: unavailable})
	return nil
}

func (p *panel) refreshAccountBalance(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := upstream[struct {
		Success   bool                 `json:"success"`
		Balance   float64              `json:"balance"`
		Currency  string               `json:"currency"`
		Balances  []accountBalanceItem `json:"balances"`
		Available bool                 `json:"available"`
		FetchedAt int64                `json:"fetched_at"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/cn-providers/accounts/%d/balance", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if !raw.Success {
		return fail(502, "balance_unavailable")
	}
	currency := strings.ToUpper(strings.TrimSpace(raw.Currency))
	if currency == "" {
		currency = "CNY"
	}
	cleanBalances := make([]accountBalanceItem, 0, len(raw.Balances))
	for _, entry := range raw.Balances {
		c := strings.ToUpper(strings.TrimSpace(entry.Currency))
		if c == "" {
			c = currency
		}
		cleanBalances = append(cleanBalances, accountBalanceItem{Currency: c, Balance: entry.Balance})
	}
	available := raw.Available
	var fetched *int64
	if raw.FetchedAt > 0 {
		fetched = &raw.FetchedAt
	}
	writeJSON(w, 200, accountBalanceView{
		Currency:  currency,
		Balance:   raw.Balance,
		Available: &available,
		Balances:  cleanBalances,
		FetchedAt: fetched,
	})
	return nil
}

func (p *panel) adminAccountStats(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	days := 7
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		days, err = strconv.Atoi(raw)
		if err != nil || days < 1 || days > 90 {
			return fail(400, "invalid_input")
		}
	}
	stats, err := upstream[struct {
		Summary struct {
			TotalRequests int64   `json:"total_requests"`
			TotalTokens   int64   `json:"total_tokens"`
			TotalCost     float64 `json:"total_cost"`
		} `json:"summary"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d/stats?days=%d", id, days), a.tokens.Access, "", "", nil)
	return send(w, 200, stats, err)
}

func (p *panel) adminAccountModels(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	models, err := upstream[[]struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d/models", id), a.tokens.Access, "", "", nil)
	if err == nil && models == nil {
		models = []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		}{}
	}
	return send(w, 200, models, err)
}

func (p *panel) syncAdminAccountModels(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	res, err := upstream[struct {
		Models []string `json:"models"`
	}](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/accounts/%d/models/sync-upstream", id), a.tokens.Access, "", "", struct{}{})
	if err == nil && res.Models == nil {
		res.Models = []string{}
	}
	return send(w, 200, res, err)
}

func (p *panel) bulkUpdateAccounts(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		AccountIDs     []int64  `json:"account_ids"`
		GroupIDs       *[]int64 `json:"group_ids,omitempty"`
		ProxyID        *int64   `json:"proxy_id,omitempty"`
		ClearProxy     bool     `json:"clear_proxy,omitempty"`
		Concurrency    *int     `json:"concurrency,omitempty"`
		Priority       *int     `json:"priority,omitempty"`
		RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
		Schedulable    *bool    `json:"schedulable,omitempty"`
		Status         string   `json:"status,omitempty"`
	}](r)
	if err != nil || len(input.AccountIDs) == 0 || len(input.AccountIDs) > 200 {
		return fail(400, "invalid_input")
	}
	for _, id := range input.AccountIDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	body := map[string]any{"account_ids": input.AccountIDs}
	if input.GroupIDs != nil {
		body["group_ids"] = *input.GroupIDs
	}
	if input.ClearProxy {
		body["proxy_id"] = nil
	} else if input.ProxyID != nil {
		if *input.ProxyID < 1 {
			return fail(400, "invalid_input")
		}
		body["proxy_id"] = *input.ProxyID
	}
	if input.Concurrency != nil {
		if *input.Concurrency < 0 || *input.Concurrency > 10000 {
			return fail(400, "invalid_input")
		}
		body["concurrency"] = *input.Concurrency
	}
	if input.Priority != nil {
		body["priority"] = *input.Priority
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return fail(400, "invalid_input")
		}
		body["rate_multiplier"] = *input.RateMultiplier
	}
	if input.Schedulable != nil {
		body["schedulable"] = *input.Schedulable
	}
	if input.Status != "" {
		if input.Status != "active" && input.Status != "inactive" {
			return fail(400, "invalid_input")
		}
		body["status"] = input.Status
	}
	if len(body) == 1 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Success int `json:"success"`
		Failed  int `json:"failed"`
	}](r.Context(), p.upstream, "POST", "/admin/accounts/bulk-update", a.tokens.Access, "", "", body)
	return send(w, 200, res, err)
}

func (p *panel) batchDeleteAccounts(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 200 {
		return fail(400, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	res, err := upstream[struct {
		Deleted int `json:"deleted"`
	}](r.Context(), p.upstream, "POST", "/admin/accounts/batch-delete", a.tokens.Access, "", "", map[string]any{"account_ids": input.IDs, "ids": input.IDs})
	return send(w, 200, res, err)
}

func (p *panel) batchAccountIDsAction(path string) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		input, err := decodeBody[struct {
			IDs []int64 `json:"ids"`
		}](r)
		if err != nil || len(input.IDs) == 0 || len(input.IDs) > 200 {
			return fail(400, "invalid_input")
		}
		for _, id := range input.IDs {
			if id < 1 {
				return fail(400, "invalid_input")
			}
		}
		res, err := upstream[struct {
			Success int `json:"success"`
			Failed  int `json:"failed"`
		}](r.Context(), p.upstream, "POST", path, a.tokens.Access, "", "", map[string]any{"account_ids": input.IDs, "ids": input.IDs})
		return send(w, 200, res, err)
	}
}

func oauthEndpoints(platform, mode string) (startPath, exchangePath, accountType string, ok bool) {
	switch platform {
	case "openai":
		return "/admin/openai/generate-auth-url", "/admin/openai/exchange-code", "oauth", true
	case "anthropic":
		if mode == "setup-token" {
			return "/admin/accounts/generate-setup-token-url", "/admin/accounts/exchange-setup-token-code", "setup-token", true
		}
		return "/admin/accounts/generate-auth-url", "/admin/accounts/exchange-code", "oauth", true
	case "gemini":
		return "/admin/gemini/oauth/auth-url", "/admin/gemini/oauth/exchange-code", "oauth", true
	case "antigravity":
		return "/admin/antigravity/oauth/auth-url", "/admin/antigravity/oauth/exchange-code", "oauth", true
	case "grok":
		return "/admin/grok/oauth/auth-url", "/admin/grok/oauth/exchange-code", "oauth", true
	default:
		return "", "", "", false
	}
}

func (p *panel) oauthStart(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Platform    string `json:"platform"`
		Mode        string `json:"mode,omitempty"`
		ProxyID     *int64 `json:"proxy_id,omitempty"`
		ProjectID   string `json:"project_id,omitempty"`
		OAuthType   string `json:"oauth_type,omitempty"`
		TierID      string `json:"tier_id,omitempty"`
		RedirectURI string `json:"redirect_uri,omitempty"`
	}](r)
	if err != nil || input.ProxyID != nil && *input.ProxyID < 1 {
		return fail(400, "invalid_input")
	}
	startPath, _, _, ok := oauthEndpoints(input.Platform, input.Mode)
	if !ok {
		return fail(400, "invalid_input")
	}
	payload := map[string]any{}
	if input.ProxyID != nil {
		payload["proxy_id"] = *input.ProxyID
	}
	if input.ProjectID != "" {
		payload["project_id"] = input.ProjectID
	}
	if input.OAuthType != "" {
		payload["oauth_type"] = input.OAuthType
	}
	if input.TierID != "" {
		payload["tier_id"] = input.TierID
	}
	if input.RedirectURI != "" {
		payload["redirect_uri"] = input.RedirectURI
	}
	res, err := upstream[struct {
		AuthURL   string `json:"auth_url"`
		SessionID string `json:"session_id"`
		State     string `json:"state"`
	}](r.Context(), p.upstream, "POST", startPath, a.tokens.Access, "", "", payload)
	if err != nil {
		return err
	}
	if res.AuthURL == "" || res.SessionID == "" {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 200, res)
	return nil
}

func (p *panel) oauthComplete(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Platform       string   `json:"platform"`
		Mode           string   `json:"mode,omitempty"`
		SessionID      string   `json:"session_id"`
		State          string   `json:"state,omitempty"`
		Code           string   `json:"code"`
		RedirectURI    string   `json:"redirect_uri,omitempty"`
		OAuthType      string   `json:"oauth_type,omitempty"`
		TierID         string   `json:"tier_id,omitempty"`
		AccountID      *int64   `json:"account_id,omitempty"`
		Name           string   `json:"name,omitempty"`
		Notes          *string  `json:"notes,omitempty"`
		ProxyID        *int64   `json:"proxy_id,omitempty"`
		GroupIDs       []int64  `json:"group_ids,omitempty"`
		Concurrency    *int     `json:"concurrency,omitempty"`
		Priority       *int     `json:"priority,omitempty"`
		RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
	}](r)
	if err != nil || strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.Code) == "" ||
		input.AccountID == nil && strings.TrimSpace(input.Name) == "" {
		return fail(400, "invalid_input")
	}
	_, exchangePath, accountType, ok := oauthEndpoints(input.Platform, input.Mode)
	if !ok {
		return fail(400, "invalid_input")
	}
	exchangeBody := map[string]any{
		"session_id": strings.TrimSpace(input.SessionID),
		"code":       strings.TrimSpace(input.Code),
	}
	if input.State != "" {
		exchangeBody["state"] = input.State
	}
	if input.ProxyID != nil && *input.ProxyID > 0 {
		exchangeBody["proxy_id"] = *input.ProxyID
	}
	if input.RedirectURI != "" {
		exchangeBody["redirect_uri"] = input.RedirectURI
	}
	if input.OAuthType != "" {
		exchangeBody["oauth_type"] = input.OAuthType
	}
	if input.TierID != "" {
		exchangeBody["tier_id"] = input.TierID
	}
	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()
	tokenData, err := upstream[map[string]any](ctx, p.upstream, "POST", exchangePath, a.tokens.Access, "", "", exchangeBody)
	if err != nil {
		return err
	}
	if len(tokenData) == 0 {
		return fail(502, "invalid_upstream")
	}
	if input.AccountID != nil && *input.AccountID > 0 {
		raw, err := upstream[accountDetailWire](ctx, p.upstream, "POST", fmt.Sprintf("/admin/accounts/%d/apply-oauth-credentials", *input.AccountID), a.tokens.Access, "", "", map[string]any{
			"type":        accountType,
			"credentials": tokenData,
		})
		if err != nil {
			return err
		}
		view, err := projectAdminAccount(raw)
		return send(w, 200, view, err)
	}
	createBody := map[string]any{
		"name":        strings.TrimSpace(input.Name),
		"platform":    input.Platform,
		"type":        accountType,
		"credentials": tokenData,
	}
	if input.Notes != nil {
		createBody["notes"] = *input.Notes
	}
	if input.ProxyID != nil && *input.ProxyID > 0 {
		createBody["proxy_id"] = *input.ProxyID
	}
	if len(input.GroupIDs) > 0 {
		createBody["group_ids"] = input.GroupIDs
	}
	if input.Concurrency != nil {
		createBody["concurrency"] = *input.Concurrency
	}
	if input.Priority != nil {
		createBody["priority"] = *input.Priority
	}
	if input.RateMultiplier != nil {
		createBody["rate_multiplier"] = *input.RateMultiplier
	}
	raw, err := upstream[accountDetailWire](ctx, p.upstream, "POST", "/admin/accounts", a.tokens.Access, "", "", createBody)
	if err != nil {
		return err
	}
	view, err := projectAdminAccount(raw)
	return send(w, 201, view, err)
}

func (p *panel) importAccounts(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Kind           string   `json:"kind"`
		Name           string   `json:"name,omitempty"`
		Notes          *string  `json:"notes,omitempty"`
		Items          []string `json:"items"`
		ProxyID        *int64   `json:"proxy_id,omitempty"`
		GroupIDs       []int64  `json:"group_ids,omitempty"`
		Concurrency    *int     `json:"concurrency,omitempty"`
		Priority       *int     `json:"priority,omitempty"`
		RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
		UpdateExisting bool     `json:"update_existing,omitempty"`
	}](r)
	if err != nil || len(input.Items) == 0 || len(input.Items) > 100 {
		return fail(400, "invalid_input")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 80*time.Second)
	defer cancel()
	switch input.Kind {
	case "codex-session":
		body := map[string]any{"contents": input.Items, "update_existing": input.UpdateExisting}
		if input.Name != "" {
			body["name"] = input.Name
		}
		if input.Notes != nil {
			body["notes"] = *input.Notes
		}
		if input.ProxyID != nil && *input.ProxyID > 0 {
			body["proxy_id"] = *input.ProxyID
		}
		if len(input.GroupIDs) > 0 {
			body["group_ids"] = input.GroupIDs
		}
		if input.Concurrency != nil {
			body["concurrency"] = *input.Concurrency
		}
		if input.Priority != nil {
			body["priority"] = *input.Priority
		}
		if input.RateMultiplier != nil {
			body["rate_multiplier"] = *input.RateMultiplier
		}
		res, err := upstream[struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		}](ctx, p.upstream, "POST", "/admin/accounts/import/codex-session", a.tokens.Access, "", "", body)
		return send(w, 200, res, err)
	case "codex-pat":
		created, failed := 0, 0
		for i, pat := range input.Items {
			pat = strings.TrimSpace(pat)
			if pat == "" {
				continue
			}
			name := strings.TrimSpace(input.Name)
			if name == "" {
				name = fmt.Sprintf("Codex PAT #%d", i+1)
			} else if len(input.Items) > 1 {
				name = fmt.Sprintf("%s #%d", name, i+1)
			}
			body := map[string]any{"access_token": pat, "name": name}
			if input.ProxyID != nil && *input.ProxyID > 0 {
				body["proxy_id"] = *input.ProxyID
			}
			if len(input.GroupIDs) > 0 {
				body["group_ids"] = input.GroupIDs
			}
			if _, cErr := upstream[map[string]any](ctx, p.upstream, "POST", "/admin/openai/create-from-codex-pat", a.tokens.Access, "", "", body); cErr != nil {
				failed++
			} else {
				created++
			}
		}
		writeJSON(w, 200, struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		}{Created: created, Failed: failed})
		return nil
	case "grok-sso":
		body := map[string]any{"sso_tokens": input.Items}
		if input.Name != "" {
			body["name"] = input.Name
		}
		if input.ProxyID != nil && *input.ProxyID > 0 {
			body["proxy_id"] = *input.ProxyID
		}
		if len(input.GroupIDs) > 0 {
			body["group_ids"] = input.GroupIDs
		}
		res, err := upstream[struct {
			Created []any `json:"created"`
			Failed  []any `json:"failed"`
		}](ctx, p.upstream, "POST", "/admin/grok/sso-to-oauth", a.tokens.Access, "", "", body)
		if err != nil {
			return err
		}
		writeJSON(w, 200, struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		}{Created: len(res.Created), Failed: len(res.Failed)})
		return nil
	default:
		return fail(400, "invalid_input")
	}
}
