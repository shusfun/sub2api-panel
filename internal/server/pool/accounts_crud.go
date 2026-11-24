package pool

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func RegisterRoutes(mux *http.ServeMux, s *core.Server) {
	h := &handler{s: s}
	h.registerAccountRoutes(mux)
	h.registerGroupRoutes(mux)
	h.registerProxyRoutes(mux)
	h.registerChannelAndPlazaRoutes(mux)
}

type handler struct {
	s *core.Server
}

func (h *handler) registerAccountRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "POST /api/admin/accounts/sync/crs/preview", true, h.syncCRSAccounts)
	h.s.Route(mux, "POST /api/admin/accounts/sync/crs", true, h.syncCRSAccounts)
	h.s.Route(mux, "POST /api/admin/accounts/data/export", true, h.exportAccountData)
	h.s.Route(mux, "POST /api/admin/accounts/data/import", true, h.importAccountData)
	h.s.Route(mux, "GET /api/admin/accounts", true, h.adminAccountsList)
	h.s.Route(mux, "GET /api/admin/accounts/stream", true, h.accountActivityStream)
	h.s.Route(mux, "GET /api/admin/accounts/batch-targets", true, h.filteredAccountTargets)
	h.s.Route(mux, "GET /api/admin/accounts/{id}", true, h.adminAccountDetail)
	h.s.Route(mux, "POST /api/admin/accounts", true, h.createAdminAccount)
	h.s.Route(mux, "PUT /api/admin/accounts/{id}", true, h.updateAdminAccount)
	h.s.Route(mux, "DELETE /api/admin/accounts/{id}", true, h.deleteAdminAccount)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/duplicate", true, h.duplicateAdminAccount)
	h.s.Route(mux, "POST /api/admin/accounts/check-mixed-channel", true, h.checkMixedChannel)
	h.s.Route(mux, "GET /api/admin/accounts/{id}/quota", true, h.accountQuota)
	h.s.Route(mux, "POST /api/admin/accounts/quota/batch", true, h.batchAccountQuota)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/balance", true, h.refreshAccountBalance)
	h.s.Route(mux, "GET /api/admin/accounts/{id}/balance-settings", true, h.accountBalanceSettings)
	h.s.Route(mux, "PUT /api/admin/accounts/{id}/balance-settings", true, h.accountBalanceSettings)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/balance-settings", true, h.accountBalanceSettings)
	h.s.Route(mux, "GET /api/admin/accounts/{id}/stats", true, h.adminAccountStats)
	h.s.Route(mux, "GET /api/admin/accounts/{id}/models", true, h.adminAccountModels)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/models/sync", true, h.syncAdminAccountModels)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/revert-proxy-fallback", true, h.revertAccountProxy)
	h.s.Route(mux, "GET /api/admin/accounts/{id}/scheduled-tests", true, h.scheduledAccountTests)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/scheduled-tests", true, h.editAccountTestPlan)
	h.s.Route(mux, "GET /api/admin/accounts/{id}/scheduled-tests/{plan}/results", true, h.editAccountTestPlan)
	h.s.Route(mux, "PUT /api/admin/accounts/{id}/scheduled-tests/{plan}", true, h.editAccountTestPlan)
	h.s.Route(mux, "DELETE /api/admin/accounts/{id}/scheduled-tests/{plan}", true, h.editAccountTestPlan)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/test", true, h.testAccount)
	h.s.Route(mux, "PUT /api/admin/accounts/{id}/scheduling", true, h.scheduleAccount)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/recover", true, h.accountAction("recover-state"))
	h.s.Route(mux, "POST /api/admin/accounts/{id}/clear-error", true, h.accountAction("clear-error"))
	h.s.Route(mux, "POST /api/admin/accounts/{id}/clear-rate-limit", true, h.accountAction("clear-rate-limit"))
	h.s.Route(mux, "POST /api/admin/accounts/{id}/reset-quota", true, h.accountAction("reset-quota"))
	h.s.Route(mux, "POST /api/admin/accounts/{id}/refresh", true, h.refreshAdminAccount)
	h.s.Route(mux, "DELETE /api/admin/accounts/{id}/temp-unschedulable", true, h.clearTempUnschedulable)
	h.s.Route(mux, "POST /api/admin/accounts/bulk-update", true, h.bulkUpdateAccounts)
	h.s.Route(mux, "POST /api/admin/accounts/batch-delete", true, h.batchDeleteAccounts)
	h.s.Route(mux, "POST /api/admin/accounts/batch-clear-error", true, h.batchAccountIDsAction("/admin/accounts/batch-clear-error"))
	h.s.Route(mux, "POST /api/admin/accounts/batch-refresh", true, h.batchAccountIDsAction("/admin/accounts/batch-refresh"))
	h.s.Route(mux, "POST /api/admin/accounts/oauth/start", true, h.oauthStart)
	h.s.Route(mux, "POST /api/admin/accounts/oauth/complete", true, h.oauthComplete)
	h.s.Route(mux, "POST /api/admin/accounts/import", true, h.importAccounts)
}

func (h *handler) adminAccountsList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	result, err := h.fetchAccountPage(r, a.Tokens.Access)
	return platform.Send(w, 200, result, err)
}

func (h *handler) fetchAccountPage(r *http.Request, token string) (platform.PageResult[AccountAdminView], error) {
	if r.URL.Query().Get("sort_by") == "activity" {
		return h.accountActivityPage(r, token)
	}
	page, params, err := platform.ListQuery(r, "search", "platform", "type", "status", "group", "sort_by", "sort_order")
	if err != nil {
		return platform.PageResult[AccountAdminView]{}, err
	}
	var proxyFilter int64
	if rawProxy := strings.TrimSpace(r.URL.Query().Get("proxy_id")); rawProxy != "" {
		proxyFilter, err = strconv.ParseInt(rawProxy, 10, 64)
		if err != nil || proxyFilter < 1 {
			return platform.PageResult[AccountAdminView]{}, platform.Fail(400, "invalid_input")
		}
	}
	if proxyFilter > 0 {
		all := make([]AccountAdminView, 0)
		for cursor := 1; cursor <= 50; cursor++ {
			rows, qErr := core.QueryPage(r.Context(), h.s.Upstream, token, "/admin/accounts", cursor, params, ProjectAdminAccount)
			if qErr != nil {
				return platform.PageResult[AccountAdminView]{}, qErr
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
		start := min((page-1)*platform.PageSize, len(all))
		end := min(start+platform.PageSize, len(all))
		pages := (len(all) + platform.PageSize - 1) / platform.PageSize
		h.withDayStats(r.Context(), token, all[start:end])
		h.applyBalanceSnapshots(all[start:end])
		return platform.PageResult[AccountAdminView]{Items: all[start:end], More: end < len(all), Pages: pages}, nil
	}
	result, err := core.QueryPage(r.Context(), h.s.Upstream, token, "/admin/accounts", page, params, ProjectAdminAccount)
	if err == nil {
		h.withDayStats(r.Context(), token, result.Items)
		h.applyBalanceSnapshots(result.Items)
	}
	return result, err
}

func (h *handler) adminAccountDetail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := ProjectAdminAccount(raw)
	views := []AccountAdminView{view}
	h.applyBalanceSnapshots(views)
	return platform.Send(w, 200, views[0], err)
}

func (h *handler) createAdminAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[accountWriteInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(true)
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "POST", "/admin/accounts", a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := ProjectAdminAccount(raw)
	return platform.Send(w, 201, view, err)
}

func (h *handler) updateAdminAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[accountWriteInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(false)
	if err != nil {
		return err
	}
	if len(input.Credentials) > 0 {
		current, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		if current.ID != id {
			return platform.Fail(502, "invalid_upstream")
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
	raw, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", id), a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := ProjectAdminAccount(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) deleteAdminAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/accounts/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) duplicateAdminAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Operation string `json:"operation_id,omitempty"`
	}](r)
	if err != nil || len(input.Operation) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	raw, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/accounts/%d/duplicate", id), a.Tokens.Access, "", input.Operation, struct{}{})
	if err != nil {
		return err
	}
	view, err := ProjectAdminAccount(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) checkMixedChannel(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Platform  string  `json:"platform"`
		GroupIDs  []int64 `json:"group_ids"`
		AccountID *int64  `json:"account_id,omitempty"`
	}](r)
	if err != nil || input.Platform == "" || len(input.GroupIDs) == 0 || len(input.GroupIDs) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		HasRisk bool   `json:"has_risk"`
		Message string `json:"message,omitempty"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/accounts/check-mixed-channel", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, res, err)
}

func (h *handler) accountAction(action string) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		raw, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/accounts/%d/%s", id, action), a.Tokens.Access, "", "", struct{}{})
		if err != nil {
			return err
		}
		view, err := ProjectAdminAccount(raw)
		return platform.Send(w, 200, view, err)
	}
}

func (h *handler) refreshAdminAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[map[string]any](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/accounts/%d/refresh", id), a.Tokens.Access, "", "", struct{}{})
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := ProjectAdminAccount(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) clearTempUnschedulable(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/accounts/%d/temp-unschedulable", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) batchAccountQuota(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		AccountIDs []int64 `json:"account_ids"`
		Force      bool    `json:"force,omitempty"`
	}](r)
	if err != nil || len(input.AccountIDs) == 0 || len(input.AccountIDs) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	for _, id := range input.AccountIDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	raw, err := platform.Upstream[struct {
		Usage  map[string]accountQuotaWire `json:"usage"`
		Errors map[string]string           `json:"errors"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/accounts/usage/batch", a.Tokens.Access, "", "", map[string]any{
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
	platform.WriteJSON(w, 200, struct {
		Quotas      map[string]accountQuotaView `json:"quotas"`
		Unavailable []int64                     `json:"unavailable"`
	}{Quotas: quotas, Unavailable: unavailable})
	return nil
}

func (h *handler) refreshAccountBalance(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[struct {
		Success   bool                      `json:"success"`
		Balance   float64                   `json:"balance"`
		Currency  string                    `json:"currency"`
		Balances  []core.AccountBalanceItem `json:"balances"`
		Available bool                      `json:"available"`
		FetchedAt int64                     `json:"fetched_at"`
	}](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/cn-providers/accounts/%d/balance", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if !raw.Success {
		return platform.Fail(502, "balance_unavailable")
	}
	currency := strings.ToUpper(strings.TrimSpace(raw.Currency))
	if currency == "" {
		currency = "CNY"
	}
	cleanBalances := make([]core.AccountBalanceItem, 0, len(raw.Balances))
	for _, entry := range raw.Balances {
		c := strings.ToUpper(strings.TrimSpace(entry.Currency))
		if c == "" {
			c = currency
		}
		cleanBalances = append(cleanBalances, core.AccountBalanceItem{Currency: c, Balance: entry.Balance})
	}
	available := raw.Available
	var fetched *int64
	if raw.FetchedAt > 0 {
		fetched = &raw.FetchedAt
	}
	platform.WriteJSON(w, 200, core.AccountBalanceView{
		Currency:  currency,
		Balance:   raw.Balance,
		Available: &available,
		Balances:  cleanBalances,
		FetchedAt: fetched,
	})
	return nil
}

func (h *handler) adminAccountStats(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	days := 7
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		days, err = strconv.Atoi(raw)
		if err != nil || days < 1 || days > 90 {
			return platform.Fail(400, "invalid_input")
		}
	}
	stats, err := platform.Upstream[struct {
		Summary struct {
			TotalRequests int64   `json:"total_requests"`
			TotalTokens   int64   `json:"total_tokens"`
			TotalCost     float64 `json:"total_cost"`
		} `json:"summary"`
	}](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d/stats?days=%d", id, days), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, stats, err)
}

func (h *handler) adminAccountModels(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	models, err := platform.Upstream[[]struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d/models", id), a.Tokens.Access, "", "", nil)
	if err == nil && models == nil {
		models = []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		}{}
	}
	return platform.Send(w, 200, models, err)
}

func (h *handler) syncAdminAccountModels(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	res, err := platform.Upstream[struct {
		Models []string `json:"models"`
	}](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/accounts/%d/models/sync-upstream", id), a.Tokens.Access, "", "", struct{}{})
	if err == nil && res.Models == nil {
		res.Models = []string{}
	}
	return platform.Send(w, 200, res, err)
}

func (h *handler) bulkUpdateAccounts(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
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
		return platform.Fail(400, "invalid_input")
	}
	for _, id := range input.AccountIDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
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
			return platform.Fail(400, "invalid_input")
		}
		body["proxy_id"] = *input.ProxyID
	}
	if input.Concurrency != nil {
		if *input.Concurrency < 0 || *input.Concurrency > 10000 {
			return platform.Fail(400, "invalid_input")
		}
		body["concurrency"] = *input.Concurrency
	}
	if input.Priority != nil {
		body["priority"] = *input.Priority
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return platform.Fail(400, "invalid_input")
		}
		body["rate_multiplier"] = *input.RateMultiplier
	}
	if input.Schedulable != nil {
		body["schedulable"] = *input.Schedulable
	}
	if input.Status != "" {
		if input.Status != "active" && input.Status != "inactive" {
			return platform.Fail(400, "invalid_input")
		}
		body["status"] = input.Status
	}
	if len(body) == 1 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Success int `json:"success"`
		Failed  int `json:"failed"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/accounts/bulk-update", a.Tokens.Access, "", "", body)
	return platform.Send(w, 200, res, err)
}

func (h *handler) batchDeleteAccounts(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 200 {
		return platform.Fail(400, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	res, err := platform.Upstream[struct {
		Deleted int `json:"deleted"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/accounts/batch-delete", a.Tokens.Access, "", "", map[string]any{"account_ids": input.IDs, "ids": input.IDs})
	return platform.Send(w, 200, res, err)
}

func (h *handler) batchAccountIDsAction(path string) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		input, err := platform.DecodeBody[struct {
			IDs []int64 `json:"ids"`
		}](r)
		if err != nil || len(input.IDs) == 0 || len(input.IDs) > 200 {
			return platform.Fail(400, "invalid_input")
		}
		for _, id := range input.IDs {
			if id < 1 {
				return platform.Fail(400, "invalid_input")
			}
		}
		res, err := platform.Upstream[struct {
			Success int `json:"success"`
			Failed  int `json:"failed"`
		}](r.Context(), h.s.Upstream, "POST", path, a.Tokens.Access, "", "", map[string]any{"account_ids": input.IDs, "ids": input.IDs})
		return platform.Send(w, 200, res, err)
	}
}
