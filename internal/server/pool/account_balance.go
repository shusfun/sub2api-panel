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
		next.AccessToken = strings.TrimSpace(next.AccessToken)
		hosted := next.Provider == "sub2api" || next.Provider == "newapi" || next.Provider == "oneapi"
		if !hosted && (next.Provider != raw.Platform || (next.Provider != "deepseek" && next.Provider != "kimi")) {
			return platform.Fail(400, "invalid_input")
		}
		// 留空表示沿用旧值：只在站点类型与地址都没变时沿用，换站点必须重新填。
		if next.Provider == config.Provider && next.BaseURL == config.BaseURL {
			if next.Key == "" {
				next.Key = config.Key
			}
			if next.AccessToken == "" {
				next.AccessToken = config.AccessToken
				if next.UpstreamUserID == 0 {
					next.UpstreamUserID = config.UpstreamUserID
				}
			}
		}
		if !hosted {
			next.AccessToken, next.UpstreamUserID = "", 0
		}
		if next.Enabled || next.BaseURL != "" || config.Key != "" {
			if _, err := core.BalanceEndpoint(next); err != nil {
				return err
			}
		}
		if (next.Enabled && next.Key == "" && next.AccessToken == "") ||
			len(next.Key) > 4096 || len(next.AccessToken) > 4096 ||
			strings.ContainsAny(next.Key+next.AccessToken, "\r\n") ||
			next.UpstreamUserID < 0 ||
			(next.Provider == "newapi" && next.AccessToken != "" && next.UpstreamUserID < 1) {
			return platform.Fail(400, "invalid_input")
		}
		// 保存前先试采一次，失败不保存；试采不持有监控锁，避免网络等待阻塞其他账号。
		if next.Enabled {
			if _, err := h.s.Balances.TrialFetch(r.Context(), next); err != nil {
				return platform.Fail(502, "balance_unavailable")
			}
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
		if row == nil || (config.Key == "" && config.AccessToken == "") {
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
	}{config.Enabled, config.Provider, config.BaseURL, config.Key != "" || config.AccessToken != ""})
	return nil
}
