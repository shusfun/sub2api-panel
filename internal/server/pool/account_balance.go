package pool

import (
	"fmt"
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func (h *handler) applyBalanceSnapshots(items []AccountAdminView) {
	if h.s.Balances == nil {
		return
	}
	for i := range items {
		if sample := h.s.Balances.Snapshot(items[i].ID); sample != nil {
			items[i].Balance = sample
		}
	}
}

func (h *handler) accountBalanceSettings(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	if h.s.Database == nil || h.s.Balances == nil {
		return platform.Fail(503, "service_unavailable")
	}
	raw, err := platform.Upstream[AccountDetailWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if raw.ID != id || raw.Type != "apikey" {
		return platform.Fail(400, "invalid_input")
	}
	row, config, err := h.s.FindBalanceConfig(r.Context(), id)
	if err != nil {
		return err
	}
	switch r.Method {
	case "PUT":
		next, err := platform.DecodeBody[core.BalanceConfig](r)
		if err != nil {
			return err
		}
		next.BaseURL = strings.TrimSpace(next.BaseURL)
		next.Key = strings.TrimSpace(next.Key)
		if next.Provider != "sub2api" && (next.Provider != raw.Platform || (next.Provider != "deepseek" && next.Provider != "kimi")) {
			return platform.Fail(400, "invalid_input")
		}
		if next.Enabled || next.BaseURL != "" || config.Key != "" {
			if _, err := core.BalanceEndpoint(next); err != nil {
				return err
			}
		}
		if next.Key == "" && next.Provider == config.Provider && next.BaseURL == config.BaseURL {
			next.Key = config.Key
		}
		if (next.Enabled && next.Key == "") || len(next.Key) > 4096 || strings.ContainsAny(next.Key, "\r\n") {
			return platform.Fail(400, "invalid_input")
		}
		h.s.Balances.Lock()
		defer h.s.Balances.Unlock()
		current, _, err := h.s.FindBalanceConfig(r.Context(), id)
		if err != nil {
			return err
		}
		if (current == nil) != (row == nil) || (current != nil && current.Value != row.Value) {
			return platform.Fail(409, "invalid_input")
		}
		if err := h.s.SaveBalanceConfig(r.Context(), id, row, next); err != nil {
			return err
		}
		h.s.Balances.DeleteStateLocked(id)
		config = next
	case "POST":
		if row == nil || config.Key == "" {
			return platform.Fail(400, "invalid_input")
		}
		completed := h.s.Balances.SampleIfDue(r.Context(), id, row.Value, config, true)
		if completed == nil {
			return platform.Fail(429, "rate_limited")
		}
		select {
		case err := <-completed:
			if err != nil {
				return platform.Fail(502, "balance_unavailable")
			}
		case <-r.Context().Done():
			return r.Context().Err()
		}
	}
	platform.WriteJSON(w, 200, struct {
		Enabled    bool   `json:"enabled"`
		Provider   string `json:"provider"`
		Base       string `json:"base_url"`
		Configured bool   `json:"configured"`
	}{config.Enabled, config.Provider, config.BaseURL, config.Key != ""})
	return nil
}
