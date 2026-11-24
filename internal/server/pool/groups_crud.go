package pool

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func (h *handler) registerGroupRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/groups", true, h.adminGroups)
	h.s.Route(mux, "GET /api/admin/groups/all", true, h.adminAllGroups)
	h.s.Route(mux, "PUT /api/admin/groups/sort-order", true, h.updateGroupSortOrder)
	h.s.Route(mux, "GET /api/admin/groups/{id}", true, h.adminGroupDetail)
	h.s.Route(mux, "POST /api/admin/groups", true, h.createAdminGroup)
	h.s.Route(mux, "PUT /api/admin/groups/{id}", true, h.updateAdminGroup)
	h.s.Route(mux, "DELETE /api/admin/groups/{id}", true, h.deleteAdminGroup)
	h.s.Route(mux, "POST /api/admin/groups/{id}/duplicate", true, h.duplicateAdminGroup)
	h.s.Route(mux, "GET /api/admin/groups/{id}/stats", true, h.adminGroupStats)
	h.s.Route(mux, "GET /api/admin/groups/{id}/keys", true, h.adminGroupKeys)
	h.s.Route(mux, "GET /api/admin/groups/{id}/model-candidates", true, h.adminGroupModelCandidates)
	h.s.Route(mux, "GET /api/admin/groups/{id}/rate-multipliers", true, h.getGroupRateMultipliers)
	h.s.Route(mux, "PUT /api/admin/groups/{id}/rate-multipliers", true, h.setGroupRateMultipliers)
	h.s.Route(mux, "DELETE /api/admin/groups/{id}/rate-multipliers", true, h.clearGroupRateMultipliers)
	h.s.Route(mux, "GET /api/admin/groups/{id}/composite-routes", true, h.listCompositeRoutes)
	h.s.Route(mux, "POST /api/admin/groups/{id}/composite-routes", true, h.createCompositeRoute)
	h.s.Route(mux, "POST /api/admin/groups/{id}/composite-routes/preview", true, h.previewCompositeRoute)
	h.s.Route(mux, "PUT /api/admin/groups/{id}/composite-routes/{route}", true, h.updateCompositeRoute)
	h.s.Route(mux, "DELETE /api/admin/groups/{id}/composite-routes/{route}", true, h.deleteCompositeRoute)
	h.s.Route(mux, "GET /api/admin/channels", true, h.listChannels)
	h.s.Route(mux, "GET /api/admin/channels/model-pricing", true, h.getChannelModelDefaultPricing)
	h.s.Route(mux, "GET /api/admin/channels/sync-models", true, h.syncChannelPricingModels)
	h.s.Route(mux, "GET /api/admin/channels/{id}", true, h.getChannel)
	h.s.Route(mux, "POST /api/admin/channels", true, h.createChannel)
	h.s.Route(mux, "PUT /api/admin/channels/{id}", true, h.updateChannel)
	h.s.Route(mux, "DELETE /api/admin/channels/{id}", true, h.deleteChannel)
}

func (h *handler) enrichAdminGroups(r *http.Request, token string, items []adminGroupView) []adminGroupView {
	if len(items) == 0 {
		return items
	}
	capacities, capErr := platform.Upstream[[]struct {
		GroupID         int64 `json:"group_id"`
		ConcurrencyUsed int   `json:"concurrency_used"`
		ConcurrencyMax  int   `json:"concurrency_max"`
		RPMUsed         int   `json:"rpm_used"`
		RPMMax          int   `json:"rpm_max"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/groups/capacity-summary", token, "", "", nil)
	capMap := make(map[int64]struct {
		ConcUsed, ConcMax, RPMUsed, RPMMax int
	})
	if capErr == nil {
		for _, c := range capacities {
			capMap[c.GroupID] = struct{ ConcUsed, ConcMax, RPMUsed, RPMMax int }{c.ConcurrencyUsed, c.ConcurrencyMax, c.RPMUsed, c.RPMMax}
		}
	}
	usageRows, usageErr := platform.Upstream[[]struct {
		GroupID   int64   `json:"group_id"`
		TodayCost float64 `json:"today_cost"`
		TotalCost float64 `json:"total_cost"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/groups/usage-summary", token, "", "", nil)
	usageMap := make(map[int64]struct{ Today, Total float64 })
	if usageErr == nil {
		for _, u := range usageRows {
			usageMap[u.GroupID] = struct{ Today, Total float64 }{u.TodayCost, u.TotalCost}
		}
	}
	monitor, monErr := platform.Upstream[groupHealthWire](r.Context(), h.s.Upstream, "GET", "/channel-monitor-v2/matrix?range=24h&group_by=platform_group", token, "", "", nil)
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
		if hv, ok := healthMap[id]; ok && hv.Health != "" {
			items[i].Health, items[i].HealthScore = hv.Health, hv.Score
		}
	}
	return items
}

func (h *handler) adminGroups(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, params, err := platform.ListQuery(r, "search", "platform", "status", "is_exclusive", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/groups", page, params, projectAdminGroup)
	if err == nil {
		rows.Items = h.enrichAdminGroups(r, a.Tokens.Access, rows.Items)
	}
	return platform.Send(w, 200, rows, err)
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

func (h *handler) adminAllGroups(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	query := url.Values{"include_inactive": {"true"}}
	if plat := strings.TrimSpace(r.URL.Query().Get("platform")); plat != "" {
		if len(plat) > 50 {
			return platform.Fail(400, "invalid_input")
		}
		query.Set("platform", plat)
	}
	rows, err := platform.Upstream[[]adminGroupWire](r.Context(), h.s.Upstream, "GET", "/admin/groups/all?"+query.Encode(), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	projected, err := platform.ProjectList(rows, projectAdminGroup)
	if err != nil {
		return err
	}
	projected = h.enrichAdminGroups(r, a.Tokens.Access, projected)
	out := make([]groupOptionView, 0, len(projected))
	for _, g := range projected {
		out = append(out, groupOptionView{
			ID: g.ID, Name: g.Name, Platform: g.Platform, RateMultiplier: g.RateMultiplier,
			Status: g.Status, SubscriptionType: g.SubscriptionType, IsExclusive: g.IsExclusive,
			AccountCount: g.AccountCount, ActiveAccounts: g.ActiveAccountCount,
			RateLimitedCount: g.RateLimitedAccountCount, Health: g.Health, HealthScore: g.HealthScore,
		})
	}
	platform.WriteJSON(w, 200, out)
	return nil
}

func (h *handler) adminGroupDetail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[adminGroupWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/groups/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(raw)
	if err != nil {
		return err
	}
	enriched := h.enrichAdminGroups(r, a.Tokens.Access, []adminGroupView{view})
	platform.WriteJSON(w, 200, enriched[0])
	return nil
}

func (h *handler) createAdminGroup(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[groupInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(true)
	if err != nil {
		return err
	}
	created, err := platform.Upstream[adminGroupWire](r.Context(), h.s.Upstream, "POST", "/admin/groups", a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(created)
	return platform.Send(w, 201, view, err)
}

func (h *handler) updateAdminGroup(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[groupInput](r)
	if err != nil {
		return err
	}
	body, err := input.toBody(false)
	if err != nil {
		return err
	}
	updated, err := platform.Upstream[adminGroupWire](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/groups/%d", id), a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(updated)
	return platform.Send(w, 200, view, err)
}

func (h *handler) deleteAdminGroup(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) duplicateAdminGroup(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
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
	created, err := platform.Upstream[adminGroupWire](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/groups/%d/duplicate", id), a.Tokens.Access, "", input.Operation, struct{}{})
	if err != nil {
		return err
	}
	view, err := projectAdminGroup(created)
	return platform.Send(w, 200, view, err)
}

func (h *handler) updateGroupSortOrder(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Updates []struct {
			ID        int64 `json:"id"`
			SortOrder int   `json:"sort_order"`
		} `json:"updates"`
	}](r)
	if err != nil || len(input.Updates) == 0 || len(input.Updates) > 200 {
		return platform.Fail(400, "invalid_input")
	}
	for _, item := range input.Updates {
		if item.ID < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "PUT", "/admin/groups/sort-order", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) adminGroupStats(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	stats, err := platform.Upstream[struct {
		TotalKeys     int64   `json:"total_api_keys"`
		ActiveKeys    int64   `json:"active_api_keys"`
		TotalRequests int64   `json:"total_requests"`
		TotalCost     float64 `json:"total_cost"`
	}](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/groups/%d/stats", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, stats, err)
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

func (h *handler) adminGroupKeys(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	page, err := platform.RequestedPage(r)
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, fmt.Sprintf("/admin/groups/%d/api-keys", id), page, nil, func(k struct {
		ID        int64   `json:"id"`
		Name      string  `json:"name"`
		Key       string  `json:"key"`
		Status    string  `json:"status"`
		Owner     int64   `json:"user_id"`
		Quota     float64 `json:"quota"`
		QuotaUsed float64 `json:"quota_used"`
		User      *struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}) (adminGroupKeyView, error) {
		if k.ID < 1 || k.Owner < 1 || k.Name == "" {
			return adminGroupKeyView{}, platform.Fail(502, "invalid_upstream")
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
	return platform.Send(w, 200, rows, err)
}

func (h *handler) adminGroupModelCandidates(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	rawID := r.PathValue("id")
	id, err := platform.ParseNonNegativeID(rawID)
	if err != nil {
		return err
	}
	query := url.Values{}
	if plat := strings.TrimSpace(r.URL.Query().Get("platform")); plat != "" {
		if len(plat) > 50 {
			return platform.Fail(400, "invalid_input")
		}
		query.Set("platform", plat)
	}
	suffix := ""
	if len(query) > 0 {
		suffix = "?" + query.Encode()
	}
	res, err := platform.Upstream[struct {
		Models []string `json:"models"`
	}](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/groups/%d/model-allowlist-candidates%s", id, suffix), a.Tokens.Access, "", "", nil)
	if err == nil && res.Models == nil {
		res.Models = []string{}
	}
	return platform.Send(w, 200, res, err)
}

func (h *handler) getGroupRateMultipliers(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	rows, err := platform.Upstream[[]groupRateMultiplierRow](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/groups/%d/rate-multipliers", id), a.Tokens.Access, "", "", nil)
	if err == nil && rows == nil {
		rows = []groupRateMultiplierRow{}
	}
	return platform.Send(w, 200, rows, err)
}

func (h *handler) setGroupRateMultipliers(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Entries []struct {
			UserID         int64   `json:"user_id"`
			RateMultiplier float64 `json:"rate_multiplier"`
		} `json:"entries"`
	}](r)
	if err != nil || len(input.Entries) == 0 || len(input.Entries) > 200 {
		return platform.Fail(400, "invalid_input")
	}
	for _, item := range input.Entries {
		if item.UserID < 1 || item.RateMultiplier < 0 {
			return platform.Fail(400, "invalid_input")
		}
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/groups/%d/rate-multipliers", id), a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) clearGroupRateMultipliers(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/groups/%d/rate-multipliers", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}
