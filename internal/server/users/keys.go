package users

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type balanceView struct {
	Balance *float64 `json:"balance"`
}

type subscriptionWire struct {
	ID      int64 `json:"id"`
	GroupID int64 `json:"group_id"`
	Group   *struct {
		platform.NamedItem
		Platform string `json:"platform"`
		Private  *bool  `json:"is_exclusive"`
	} `json:"group"`
	Status  string    `json:"status"`
	Expires time.Time `json:"expires_at"`
}

type subscriptionView struct {
	ID       int64     `json:"id"`
	Group    string    `json:"group"`
	Status   string    `json:"status"`
	Expires  time.Time `json:"expires_at"`
	Platform string    `json:"platform,omitempty"`
	Private  *bool     `json:"is_exclusive,omitempty"`
}

func projectSubscription(s subscriptionWire) (subscriptionView, error) {
	group := fmt.Sprintf("分组 %d", s.GroupID)
	if s.Group != nil && s.Group.Name != "" {
		group = s.Group.Name
	}
	view := subscriptionView{ID: s.ID, Group: group, Status: s.Status, Expires: s.Expires}
	if s.Group != nil {
		view.Platform, view.Private = s.Group.Platform, s.Group.Private
	}
	return view, platform.RequiredID(s.ID)
}

type KeyWire struct {
	platform.NamedItem
	Key        string `json:"key"`
	Status     string `json:"status"`
	Group      *int64 `json:"group_id"`
	BatchGroup *struct {
		Platform string `json:"platform"`
		Enabled  bool   `json:"allow_batch_image_generation"`
	} `json:"group"`
	Owner       int64      `json:"user_id"`
	IPWhitelist []string   `json:"ip_whitelist"`
	IPBlacklist []string   `json:"ip_blacklist"`
	Quota       float64    `json:"quota"`
	QuotaUsed   float64    `json:"quota_used"`
	Expires     *time.Time `json:"expires_at"`
	LastUsed    *time.Time `json:"last_used_at"`
	LastIP      *string    `json:"last_used_ip"`
	Concurrency int64      `json:"current_concurrency"`
	Limit5h     float64    `json:"rate_limit_5h"`
	Limit1d     float64    `json:"rate_limit_1d"`
	Limit7d     float64    `json:"rate_limit_7d"`
	Usage5h     float64    `json:"usage_5h"`
	Usage1d     float64    `json:"usage_1d"`
	Usage7d     float64    `json:"usage_7d"`
	TodayCost   *float64   `json:"today_actual_cost"`
	TotalCost   *float64   `json:"total_actual_cost"`
}

type KeyView struct {
	platform.NamedItem
	Masked      string     `json:"masked"`
	Status      string     `json:"status"`
	GroupID     *int64     `json:"group_id"`
	BatchImage  bool       `json:"batch_image,omitempty"`
	IPWhitelist []string   `json:"ip_whitelist"`
	IPBlacklist []string   `json:"ip_blacklist"`
	Quota       float64    `json:"quota"`
	QuotaUsed   float64    `json:"quota_used"`
	Expires     *time.Time `json:"expires_at"`
	LastUsed    *time.Time `json:"last_used_at"`
	LastIP      *string    `json:"last_used_ip"`
	Concurrency int64      `json:"current_concurrency"`
	Limit5h     float64    `json:"rate_limit_5h"`
	Limit1d     float64    `json:"rate_limit_1d"`
	Limit7d     float64    `json:"rate_limit_7d"`
	Usage5h     float64    `json:"usage_5h"`
	Usage1d     float64    `json:"usage_1d"`
	Usage7d     float64    `json:"usage_7d"`
	TodayCost   *float64   `json:"today_actual_cost"`
	TotalCost   *float64   `json:"total_actual_cost"`
}

func ProjectKey(k KeyWire, owner int64) (KeyView, error) {
	if k.Owner != owner {
		return KeyView{}, platform.Fail(502, "invalid_upstream")
	}
	if k.IPWhitelist == nil {
		k.IPWhitelist = []string{}
	}
	if k.IPBlacklist == nil {
		k.IPBlacklist = []string{}
	}
	masked := "••••••••"
	if len(k.Key) > 12 {
		masked += k.Key[len(k.Key)-4:]
	}
	if k.Name == "" || k.Quota < 0 || k.QuotaUsed < 0 || k.Concurrency < 0 ||
		k.Limit5h < 0 || k.Limit1d < 0 || k.Limit7d < 0 || k.Usage5h < 0 || k.Usage1d < 0 || k.Usage7d < 0 {
		return KeyView{}, platform.Fail(502, "invalid_upstream")
	}
	return KeyView{
		NamedItem: k.NamedItem, Masked: masked, Status: k.Status, GroupID: k.Group,
		BatchImage:  k.BatchGroup != nil && k.BatchGroup.Platform == "gemini" && k.BatchGroup.Enabled,
		IPWhitelist: k.IPWhitelist, IPBlacklist: k.IPBlacklist,
		Quota: k.Quota, QuotaUsed: k.QuotaUsed, Expires: k.Expires,
		LastUsed: k.LastUsed, LastIP: k.LastIP, Concurrency: k.Concurrency,
		Limit5h: k.Limit5h, Limit1d: k.Limit1d, Limit7d: k.Limit7d,
		Usage5h: k.Usage5h, Usage1d: k.Usage1d, Usage7d: k.Usage7d,
	}, platform.RequiredID(k.ID)
}

type groupWire struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Platform       string  `json:"platform"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Status         string  `json:"status"`
	Subscription   string  `json:"subscription_type"`
	IsExclusive    bool    `json:"is_exclusive"`
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

type groupView struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform"`
	RateMultiplier float64  `json:"rate_multiplier"`
	Status         string   `json:"status"`
	Subscription   string   `json:"subscription_type"`
	IsExclusive    bool     `json:"is_exclusive"`
	Health         string   `json:"health"`
	HealthScore    *float64 `json:"health_score"`
}

type keyUsageStats struct {
	Today float64 `json:"today_actual_cost"`
	Total float64 `json:"total_actual_cost"`
}

func (h *handler) registerKeyRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/balance", false, func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		row, err := platform.Upstream[balanceView](r.Context(), h.s.Upstream, "GET", "/user/profile", a.Tokens.Access, "", "", nil)
		return platform.Send(w, 200, row, err)
	})
	h.s.Route(mux, "GET /api/subscriptions", false, func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		rows, err := platform.Upstream[[]subscriptionWire](r.Context(), h.s.Upstream, "GET", "/subscriptions", a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		data, err := platform.ProjectList(rows, projectSubscription)
		return platform.Send(w, 200, data, err)
	})
	h.s.Route(mux, "GET /api/keys/{id}/secret", false, h.keySecret)
	h.s.Route(mux, "GET /api/keys", false, func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, err := platform.RequestedPage(r)
		if err != nil {
			return err
		}
		rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/keys", page, nil, func(k KeyWire) (KeyView, error) {
			return ProjectKey(k, a.User.ID)
		})
		if err == nil {
			rows.Items = h.withKeyUsage(r.Context(), a.Tokens.Access, rows.Items, false)
		}
		return platform.Send(w, 200, rows, err)
	})
	h.s.Route(mux, "GET /api/key-groups", false, h.keyGroups)
	h.s.Route(mux, "POST /api/keys", false, h.createKey)
	h.s.Route(mux, "PUT /api/keys/{id}", false, h.updateKey)
	h.s.Route(mux, "DELETE /api/keys/{id}", false, h.deleteKey)
	h.s.Route(mux, "GET /api/admin/users/{user}/keys", true, h.adminUserKeys)
}

func (h *handler) withKeyUsage(ctx context.Context, token string, keys []KeyView, admin bool) []KeyView {
	if len(keys) == 0 {
		return keys
	}
	ids := make([]int64, 0, len(keys))
	for _, key := range keys {
		ids = append(ids, key.ID)
	}
	path := "/usage/dashboard/api-keys-usage"
	if admin {
		path = "/admin/dashboard/api-keys-usage"
	}
	result, err := platform.Upstream[struct {
		Stats map[string]keyUsageStats `json:"stats"`
	}](ctx, h.s.Upstream, "POST", path, token, "", "", struct {
		IDs []int64 `json:"api_key_ids"`
	}{ids})
	if err != nil {
		return keys
	}
	for index := range keys {
		if stats, ok := result.Stats[strconv.FormatInt(keys[index].ID, 10)]; ok && stats.Today >= 0 && stats.Total >= 0 {
			today, total := stats.Today, stats.Total
			keys[index].TodayCost, keys[index].TotalCost = &today, &total
		}
	}
	return keys
}

func (h *handler) keyGroups(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	groups, err := platform.Upstream[[]groupWire](r.Context(), h.s.Upstream, "GET", "/groups/available", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	health := make(map[int64]groupView)
	monitor, monitorErr := platform.Upstream[groupHealthWire](r.Context(), h.s.Upstream, "GET", "/channel-monitor-v2/matrix?range=24h&group_by=platform_group", a.Tokens.Access, "", "", nil)
	if monitorErr == nil {
		for _, item := range monitor.Items {
			if item.GroupID > 0 && (item.Health.Overall == "healthy" || item.Health.Overall == "warning" || item.Health.Overall == "critical" || item.Health.Overall == "unknown") {
				health[item.GroupID] = groupView{Health: item.Health.Overall, HealthScore: item.Health.Score}
			}
		}
	}
	out := make([]groupView, 0, len(groups))
	for _, group := range groups {
		if group.ID < 1 || group.Name == "" || group.Status != "active" || group.RateMultiplier < 0 {
			return platform.Fail(502, "invalid_upstream")
		}
		status := health[group.ID]
		if status.Health == "" {
			status.Health = "unavailable"
		}
		out = append(out, groupView{group.ID, group.Name, group.Platform, group.RateMultiplier, group.Status, group.Subscription, group.IsExclusive, status.Health, status.HealthScore})
	}
	return platform.Send(w, 200, out, nil)
}

func (h *handler) adminUserKeys(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	userID, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	page, err := platform.RequestedPage(r)
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, fmt.Sprintf("/admin/users/%d/api-keys", userID), page, nil,
		func(key KeyWire) (KeyView, error) { return ProjectKey(key, userID) })
	if err == nil {
		rows.Items = h.withKeyUsage(r.Context(), a.Tokens.Access, rows.Items, true)
	}
	return platform.Send(w, 200, rows, err)
}

type keyInput struct {
	CustomKey           string    `json:"custom_key,omitempty"`
	Name                string    `json:"name,omitempty"`
	Group               *int64    `json:"group_id,omitempty"`
	ClearGroup          bool      `json:"clear_group,omitempty"`
	Status              string    `json:"status,omitempty"`
	IPWhitelist         *[]string `json:"ip_whitelist,omitempty"`
	IPBlacklist         *[]string `json:"ip_blacklist,omitempty"`
	Quota               *float64  `json:"quota,omitempty"`
	ExpiresInDays       *int64    `json:"expires_in_days,omitempty"`
	ExpiresAt           *string   `json:"expires_at,omitempty"`
	ClearExpires        bool      `json:"clear_expires,omitempty"`
	ResetQuota          bool      `json:"reset_quota,omitempty"`
	Limit5h             *float64  `json:"rate_limit_5h,omitempty"`
	Limit1d             *float64  `json:"rate_limit_1d,omitempty"`
	Limit7d             *float64  `json:"rate_limit_7d,omitempty"`
	ResetRateLimitUsage bool      `json:"reset_rate_limit_usage,omitempty"`
}

func validKeyIPRules(rules *[]string) bool {
	if rules == nil {
		return true
	}
	if len(*rules) > 100 {
		return false
	}
	for _, rule := range *rules {
		if len(rule) > 64 {
			return false
		}
		if _, err := netip.ParsePrefix(rule); err != nil {
			if _, addressErr := netip.ParseAddr(rule); addressErr != nil {
				return false
			}
		}
	}
	return true
}

func (h *handler) keySecret(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	key, err := platform.Upstream[KeyWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/keys/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if key.ID != id || key.Owner != a.User.ID || key.Key == "" {
		return platform.Fail(404, "not_found")
	}
	platform.WriteJSON(w, 200, struct {
		Key string `json:"key"`
	}{key.Key})
	return nil
}

func validCustomKey(value string) bool {
	if len(value) < 16 || len(value) > 256 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

func (h *handler) createKey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[keyInput](r)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 || input.Status != "" || input.Group != nil && *input.Group < 1 ||
		input.Quota != nil && *input.Quota < 0 || input.ExpiresInDays != nil && *input.ExpiresInDays < 0 ||
		input.Limit5h != nil && *input.Limit5h < 0 || input.Limit1d != nil && *input.Limit1d < 0 || input.Limit7d != nil && *input.Limit7d < 0 ||
		input.CustomKey != "" && !validCustomKey(input.CustomKey) || !validKeyIPRules(input.IPWhitelist) || !validKeyIPRules(input.IPBlacklist) {
		return platform.Fail(400, "invalid_input")
	}
	result, err := platform.Upstream[KeyWire](r.Context(), h.s.Upstream, "POST", "/keys", a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	if result.ID < 1 || result.Owner != a.User.ID || result.Key == "" {
		return platform.Fail(502, "invalid_upstream")
	}
	platform.WriteJSON(w, 201, struct {
		Key string `json:"key"`
	}{result.Key})
	return nil
}

func (h *handler) updateKey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[keyInput](r)
	if err != nil {
		return err
	}
	if input.CustomKey != "" || len(input.Name) > 100 || input.Name != "" && strings.TrimSpace(input.Name) == "" || input.Group != nil && *input.Group < 0 ||
		input.Status != "" && input.Status != "active" && input.Status != "inactive" ||
		input.Quota != nil && *input.Quota < 0 || input.ExpiresInDays != nil && *input.ExpiresInDays < 0 ||
		input.Limit5h != nil && *input.Limit5h < 0 || input.Limit1d != nil && *input.Limit1d < 0 || input.Limit7d != nil && *input.Limit7d < 0 ||
		!validKeyIPRules(input.IPWhitelist) || !validKeyIPRules(input.IPBlacklist) {
		return platform.Fail(400, "invalid_input")
	}
	if input.ExpiresAt != nil && *input.ExpiresAt != "" {
		if _, parseErr := time.Parse(time.RFC3339, *input.ExpiresAt); parseErr != nil {
			return platform.Fail(400, "invalid_input")
		}
	}
	body := map[string]any{}
	if input.Name != "" {
		body["name"] = strings.TrimSpace(input.Name)
	}
	if input.ClearGroup {
		body["group_id"] = nil
	} else if input.Group != nil {
		if *input.Group == 0 {
			body["group_id"] = nil
		} else {
			body["group_id"] = *input.Group
		}
	}
	if input.Status != "" {
		body["status"] = input.Status
	}
	if input.IPWhitelist != nil {
		body["ip_whitelist"] = *input.IPWhitelist
	}
	if input.IPBlacklist != nil {
		body["ip_blacklist"] = *input.IPBlacklist
	}
	if input.Quota != nil {
		body["quota"] = *input.Quota
	}
	if input.ClearExpires || input.ExpiresAt != nil && *input.ExpiresAt == "" {
		body["expires_at"] = ""
	} else if input.ExpiresAt != nil && *input.ExpiresAt != "" {
		body["expires_at"] = *input.ExpiresAt
	} else if input.ExpiresInDays != nil && *input.ExpiresInDays > 0 {
		body["expires_at"] = time.Now().UTC().AddDate(0, 0, int(*input.ExpiresInDays)).Format(time.RFC3339)
	}
	if input.ResetQuota {
		body["reset_quota"] = true
	}
	if input.Limit5h != nil {
		body["rate_limit_5h"] = *input.Limit5h
	}
	if input.Limit1d != nil {
		body["rate_limit_1d"] = *input.Limit1d
	}
	if input.Limit7d != nil {
		body["rate_limit_7d"] = *input.Limit7d
	}
	if input.ResetRateLimitUsage {
		body["reset_rate_limit_usage"] = true
	}
	if len(body) == 0 {
		return platform.Fail(400, "invalid_input")
	}
	data, err := platform.Upstream[KeyWire](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/keys/%d", id), a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	if data.ID != id || data.Owner != a.User.ID {
		return platform.Fail(502, "invalid_upstream")
	}
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}

func (h *handler) deleteKey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/keys/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}
