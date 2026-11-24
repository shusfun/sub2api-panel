package pool

import (
	"net/http"
	"strconv"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

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

func (h *handler) bulkUpdateAccounts(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		AccountIDs       []int64  `json:"account_ids"`
		GroupIDs         *[]int64 `json:"group_ids,omitempty"`
		GroupMode        string   `json:"group_mode,omitempty"`
		ProxyID          *int64   `json:"proxy_id,omitempty"`
		ClearProxy       bool     `json:"clear_proxy,omitempty"`
		Concurrency      *int     `json:"concurrency,omitempty"`
		Priority         *int     `json:"priority,omitempty"`
		RateMultiplier   *float64 `json:"rate_multiplier,omitempty"`
		Schedulable      *bool    `json:"schedulable,omitempty"`
		Status           string   `json:"status,omitempty"`
		ConfirmMixedRisk bool     `json:"confirm_mixed_channel_risk,omitempty"`
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
		if input.GroupMode != "" {
			if input.GroupMode != "replace" && input.GroupMode != "append" && input.GroupMode != "remove" {
				return platform.Fail(400, "invalid_input")
			}
			body["group_mode"] = input.GroupMode
		}
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
	if input.ConfirmMixedRisk {
		body["confirm_mixed_channel_risk"] = true
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
