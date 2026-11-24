package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type wirePage[T any] struct {
	Items []T    `json:"items"`
	Pages *int   `json:"pages"`
	Total *int64 `json:"total"`
}

func queryPage[T any, V any](ctx context.Context, c serviceClient, token, path string, page int, query url.Values, project func(T) (V, error)) (pageResult[V], error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("page_size", strconv.Itoa(pageSize))
	input, err := upstream[wirePage[T]](ctx, c, "GET", path+"?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return pageResult[V]{}, err
	}
	pages := -1
	if input.Pages != nil {
		pages = *input.Pages
	} else if input.Total != nil && *input.Total >= 0 {
		pages = int((*input.Total + int64(pageSize) - 1) / int64(pageSize))
	}
	if pages < 0 || len(input.Items) > pageSize {
		return pageResult[V]{}, fail(502, "invalid_upstream")
	}
	items, err := projectList(input.Items, project)
	return pageResult[V]{items, page < pages, pages}, err
}

func projectList[T any, V any](input []T, project func(T) (V, error)) ([]V, error) {
	out := make([]V, 0, len(input))
	for _, item := range input {
		value, err := project(item)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func requiredID(id int64) error {
	if id < 1 {
		return fail(502, "invalid_upstream")
	}
	return nil
}

func send[T any](w http.ResponseWriter, status int, data T, err error) error {
	if err == nil {
		writeJSON(w, status, data)
	}
	return err
}

type balanceView struct {
	Balance *float64 `json:"balance"`
}

type namedItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type subscriptionWire struct {
	ID      int64 `json:"id"`
	GroupID int64 `json:"group_id"`
	Group   *struct {
		namedItem
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
	return view, requiredID(s.ID)
}

type usageView struct {
	ID      int64     `json:"id"`
	Model   string    `json:"model"`
	Created time.Time `json:"created_at"`
	Input   int64     `json:"input_tokens"`
	Output  int64     `json:"output_tokens"`
	Cost    *float64  `json:"actual_cost"`
}

type userRow struct {
	ID      int64    `json:"id"`
	Email   string   `json:"email"`
	Status  string   `json:"status"`
	Balance *float64 `json:"balance"`
}

type keyWire struct {
	namedItem
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

type keyView struct {
	namedItem
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

func projectKey(k keyWire, owner int64) (keyView, error) {
	if k.Owner != owner {
		return keyView{}, fail(502, "invalid_upstream")
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
		return keyView{}, fail(502, "invalid_upstream")
	}
	return keyView{
		namedItem: k.namedItem, Masked: masked, Status: k.Status, GroupID: k.Group,
		BatchImage:  k.BatchGroup != nil && k.BatchGroup.Platform == "gemini" && k.BatchGroup.Enabled,
		IPWhitelist: k.IPWhitelist, IPBlacklist: k.IPBlacklist,
		Quota: k.Quota, QuotaUsed: k.QuotaUsed, Expires: k.Expires,
		LastUsed: k.LastUsed, LastIP: k.LastIP, Concurrency: k.Concurrency,
		Limit5h: k.Limit5h, Limit1d: k.Limit1d, Limit7d: k.Limit7d,
		Usage5h: k.Usage5h, Usage1d: k.Usage1d, Usage7d: k.Usage7d,
	}, requiredID(k.ID)
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

type quotaWire struct {
	Usage   *float64   `json:"utilization"`
	Unknown bool       `json:"utilization_unknown"`
	Reset   *time.Time `json:"resets_at"`
}

type quotaView struct {
	Usage *float64   `json:"percent_used"`
	Reset *time.Time `json:"resets_at"`
}

type accountWire struct {
	namedItem
	Platform    string `json:"platform"`
	Status      string `json:"status"`
	Schedulable *bool  `json:"schedulable"`
	Extra       struct {
		Five *float64 `json:"codex_5h_used_percent"`
		Week *float64 `json:"codex_7d_used_percent"`
	} `json:"extra"`
}

type accountRow struct {
	namedItem
	Platform    string   `json:"platform"`
	Status      string   `json:"status"`
	Schedulable bool     `json:"schedulable"`
	Five        *float64 `json:"five_hours_used"`
	Week        *float64 `json:"week_used"`
}

func projectAccount(a accountWire) (accountRow, error) {
	if a.ID < 1 || a.Schedulable == nil {
		return accountRow{}, fail(502, "invalid_upstream")
	}
	return accountRow{a.namedItem, a.Platform, a.Status, *a.Schedulable, a.Extra.Five, a.Extra.Week}, nil
}

func (p *panel) businessRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/balance", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		row, err := upstream[balanceView](r.Context(), p.upstream, "GET", "/user/profile", a.tokens.Access, "", "", nil)
		return send(w, 200, row, err)
	})
	p.route(mux, "GET /api/subscriptions", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		rows, err := upstream[[]subscriptionWire](r.Context(), p.upstream, "GET", "/subscriptions", a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		data, err := projectList(rows, projectSubscription)
		return send(w, 200, data, err)
	})
	p.route(mux, "GET /api/usage", false, p.usage)
	p.route(mux, "GET /api/usage/summary", false, p.usageSummary)
	p.route(mux, "GET /api/usage/keys", false, p.usageKeys)
	p.route(mux, "POST /api/keys/usage", false, p.keyUsageBatch)
	p.route(mux, "GET /api/keys/{id}/secret", false, p.keySecret)
	p.route(mux, "GET /api/keys", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		page, err := requestedPage(r)
		if err != nil {
			return err
		}
		rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/keys", page, nil, func(k keyWire) (keyView, error) {
			return projectKey(k, a.user.ID)
		})
		if err == nil {
			rows.Items = p.withKeyUsage(r.Context(), a.tokens.Access, rows.Items, false)
		}
		return send(w, 200, rows, err)
	})
	p.route(mux, "GET /api/key-groups", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		groups, err := upstream[[]groupWire](r.Context(), p.upstream, "GET", "/groups/available", a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		health := make(map[int64]groupView)
		monitor, monitorErr := upstream[groupHealthWire](r.Context(), p.upstream, "GET", "/channel-monitor-v2/matrix?range=24h&group_by=platform_group", a.tokens.Access, "", "", nil)
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
				return fail(502, "invalid_upstream")
			}
			status := health[group.ID]
			if status.Health == "" {
				status.Health = "unavailable"
			}
			out = append(out, groupView{group.ID, group.Name, group.Platform, group.RateMultiplier, group.Status, group.Subscription, group.IsExclusive, status.Health, status.HealthScore})
		}
		return send(w, 200, out, nil)
	})
	p.route(mux, "POST /api/keys", false, p.createKey)
	p.route(mux, "PUT /api/keys/{id}", false, p.updateKey)
	p.route(mux, "DELETE /api/keys/{id}", false, p.deleteKey)
	p.route(mux, "POST /api/redeem", false, p.redeem)
	p.route(mux, "GET /api/redeem/history", false, p.redeemHistory)
	p.route(mux, "GET /api/channels/available", false, p.availableChannels)
	p.route(mux, "GET /api/referral", false, p.referral)
	p.route(mux, "GET /api/admin/usage/users", true, p.adminUsageUsers)
	p.route(mux, "GET /api/admin/usage", true, p.adminDashboardUsage)
	p.route(mux, "GET /api/admin/usage/keys", true, p.adminDashboardKeys)
	p.route(mux, "GET /api/admin/users/{user}/usage", true, p.adminUsage)
	p.route(mux, "GET /api/admin/users/{user}/keys", true, p.adminUserKeys)
	p.route(mux, "GET /api/admin/usage/summary", true, p.adminUsageSummary)
	p.route(mux, "GET /api/admin/usage/stream", true, p.adminUsageStream)
	p.route(mux, "GET /api/admin/overview", true, p.adminOverview)
	p.route(mux, "GET /api/admin/proxies", true, p.adminProxies)
	p.route(mux, "GET /api/admin/proxies/{id}", true, p.adminProxy)
	p.route(mux, "POST /api/admin/proxies", true, p.createProxy)
	p.route(mux, "PUT /api/admin/proxies/{id}", true, p.updateProxy)
	p.route(mux, "DELETE /api/admin/proxies/{id}", true, p.deleteProxy)
	p.route(mux, "POST /api/admin/proxies/{id}/test", true, p.testProxy)
	p.route(mux, "POST /api/admin/proxies/{id}/quality-check", true, p.checkProxyQuality)
	p.route(mux, "GET /api/admin/proxies/{id}/stats", true, p.proxyStats)
	p.route(mux, "GET /api/admin/proxies/{id}/accounts", true, p.proxyAccounts)
	p.route(mux, "POST /api/admin/proxies/batch-delete", true, p.batchDeleteProxies)
	p.route(mux, "POST /api/admin/proxies/batch", true, p.batchCreateProxies)
	p.route(mux, "POST /api/admin/proxies/import", true, p.importProxies)
	p.route(mux, "GET /api/admin/proxies/export", true, p.exportProxies)
	p.route(mux, "GET /api/admin/users/{user}/subscriptions", true, p.userSubscriptions)
	p.opsRoutes(mux)
	p.groupRoutes(mux)
	p.accountRoutes(mux)
	p.userAdminRoutes(mux)
	p.redeemAdminRoutes(mux)
}

func (p *panel) adminUsage(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	filter, err := parseUsageFilter(r.URL.Query(), time.Now().In(p.usageZone()), false)
	if err != nil {
		return err
	}
	query := filter.query()
	query.Set("user_id", strconv.FormatInt(id, 10))
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/usage", page, query, func(row struct {
		usageView
		UserID     int64      `json:"user_id"`
		KeyID      int64      `json:"api_key_id"`
		Key        *namedItem `json:"api_key"`
		CacheRead  *int64     `json:"cache_read_tokens"`
		CacheWrite *int64     `json:"cache_creation_tokens"`
	}) (usageRecord, error) {
		if row.UserID != id || row.KeyID < 1 || filter.key != 0 && row.KeyID != filter.key ||
			filter.model != "" && row.Model != filter.model || !filter.start.IsZero() && (row.Created.Before(filter.start) || !row.Created.Before(filter.end.AddDate(0, 0, 1))) {
			return usageRecord{}, fail(502, "invalid_upstream")
		}
		if row.CacheRead != nil && *row.CacheRead < 0 || row.CacheWrite != nil && *row.CacheWrite < 0 {
			return usageRecord{}, fail(502, "invalid_upstream")
		}
		name := fmt.Sprintf("Key #%d", row.KeyID)
		if row.Key != nil && row.Key.ID == row.KeyID && row.Key.Name != "" {
			name = row.Key.Name
		}
		row.Created = row.Created.In(p.usageZone())
		return usageRecord{usageView: row.usageView, CacheRead: row.CacheRead, CacheWrite: row.CacheWrite, KeyName: name}, requiredID(row.ID)
	})
	return send(w, 200, rows, err)
}

type usageUserOption struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func (p *panel) adminUsageUsers(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "search")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/users", page, params, func(user userRow) (usageUserOption, error) {
		if user.Email == "" {
			return usageUserOption{}, fail(502, "invalid_upstream")
		}
		return usageUserOption{user.ID, user.Email}, requiredID(user.ID)
	})
	return send(w, 200, rows, err)
}

func (p *panel) adminUserKeys(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	userID, err := pathID(r, "user")
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, fmt.Sprintf("/admin/users/%d/api-keys", userID), page, nil,
		func(key keyWire) (keyView, error) { return projectKey(key, userID) })
	if err == nil {
		rows.Items = p.withKeyUsage(r.Context(), a.tokens.Access, rows.Items, true)
	}
	return send(w, 200, rows, err)
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

func (p *panel) keySecret(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	key, err := upstream[keyWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/keys/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if key.ID != id || key.Owner != a.user.ID || key.Key == "" {
		return fail(404, "not_found")
	}
	writeJSON(w, 200, struct {
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

func (p *panel) createKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[keyInput](r)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 || input.Status != "" || input.Group != nil && *input.Group < 1 ||
		input.Quota != nil && *input.Quota < 0 || input.ExpiresInDays != nil && *input.ExpiresInDays < 0 ||
		input.Limit5h != nil && *input.Limit5h < 0 || input.Limit1d != nil && *input.Limit1d < 0 || input.Limit7d != nil && *input.Limit7d < 0 ||
		input.CustomKey != "" && !validCustomKey(input.CustomKey) || !validKeyIPRules(input.IPWhitelist) || !validKeyIPRules(input.IPBlacklist) {
		return fail(400, "invalid_input")
	}
	result, err := upstream[keyWire](r.Context(), p.upstream, "POST", "/keys", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	if result.ID < 1 || result.Owner != a.user.ID || result.Key == "" {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 201, struct {
		Key string `json:"key"`
	}{result.Key})
	return nil
}

func (p *panel) updateKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[keyInput](r)
	if err != nil {
		return err
	}
	if input.CustomKey != "" || len(input.Name) > 100 || input.Name != "" && strings.TrimSpace(input.Name) == "" || input.Group != nil && *input.Group < 0 ||
		input.Status != "" && input.Status != "active" && input.Status != "inactive" ||
		input.Quota != nil && *input.Quota < 0 || input.ExpiresInDays != nil && *input.ExpiresInDays < 0 ||
		input.Limit5h != nil && *input.Limit5h < 0 || input.Limit1d != nil && *input.Limit1d < 0 || input.Limit7d != nil && *input.Limit7d < 0 ||
		!validKeyIPRules(input.IPWhitelist) || !validKeyIPRules(input.IPBlacklist) {
		return fail(400, "invalid_input")
	}
	if input.ExpiresAt != nil && *input.ExpiresAt != "" {
		if _, parseErr := time.Parse(time.RFC3339, *input.ExpiresAt); parseErr != nil {
			return fail(400, "invalid_input")
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
		return fail(400, "invalid_input")
	}
	data, err := upstream[keyWire](r.Context(), p.upstream, "PUT", fmt.Sprintf("/keys/%d", id), a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	if data.ID != id || data.Owner != a.user.ID {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func (p *panel) deleteKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/keys/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

type availableChannel struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Platforms   []struct {
		Platform string `json:"platform"`
		Groups   []struct {
			namedItem
			Rate         float64 `json:"rate_multiplier"`
			Subscription string  `json:"subscription_type"`
		} `json:"groups"`
		Models []struct {
			Name    string `json:"name"`
			Pricing *struct {
				Input   *float64 `json:"input_price"`
				Output  *float64 `json:"output_price"`
				Request *float64 `json:"per_request_price"`
			} `json:"pricing"`
		} `json:"supported_models"`
	} `json:"platforms"`
}

func (p *panel) availableChannels(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	channels, err := upstream[[]availableChannel](r.Context(), p.upstream, "GET", "/channels/available", a.tokens.Access, "", "", nil)
	if channels == nil {
		channels = []availableChannel{}
	}
	return send(w, http.StatusOK, channels, err)
}

type redemptionRecord struct {
	ID     int64      `json:"id"`
	Type   string     `json:"type"`
	Value  float64    `json:"value"`
	UsedAt *time.Time `json:"used_at"`
}

func (p *panel) redeemHistory(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, _, err := listQuery(r)
	if err != nil {
		return err
	}
	result, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/redeem/history", page, nil,
		func(row redemptionRecord) (redemptionRecord, error) { return row, requiredID(row.ID) })
	return send(w, http.StatusOK, result, err)
}

func (p *panel) redeem(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Code string `json:"code"`
	}](r)
	if err != nil {
		return err
	}
	input.Code = strings.TrimSpace(input.Code)
	if len(input.Code) < 3 || len(input.Code) > 128 {
		return fail(400, "invalid_input")
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "POST", "/redeem", a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) referral(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	settings, err := p.settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.Affiliate {
		return fail(403, "not_allowed")
	}
	data, err := upstream[struct {
		Code string `json:"aff_code"`
	}](r.Context(), p.upstream, "GET", "/user/aff", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if data.Code == "" {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 200, struct {
		URL string `json:"url"`
	}{p.origin + "/register?aff=" + url.QueryEscape(data.Code)})
	return nil
}

func listQuery(r *http.Request, names ...string) (int, url.Values, error) {
	page, err := requestedPage(r)
	if err != nil {
		return 0, nil, err
	}
	params := url.Values{}
	for _, name := range names {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if len(value) > 100 {
			return 0, nil, fail(400, "invalid_input")
		}
		if value != "" {
			params.Set(name, value)
		}
	}
	return page, params, nil
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

func projectQuotaInfo(raw accountQuotaWire) accountQuotaView {
	project := func(window *quotaWire) *quotaView {
		if window == nil {
			return nil
		}
		usage := window.Usage
		if window.Unknown {
			usage = nil
		}
		return &quotaView{usage, window.Reset}
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

func (p *panel) accountQuota(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	source := "passive"
	if r.URL.Query().Get("source") == "active" {
		source = "active"
	}
	raw, err := upstream[accountQuotaWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d/usage?source=%s", id, source), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	writeJSON(w, 200, projectQuotaInfo(raw))
	return nil
}

func (p *panel) scheduleAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		On *bool `json:"schedulable"`
	}](r)
	if err != nil {
		return err
	}
	if input.On == nil {
		return fail(400, "invalid_input")
	}
	path := fmt.Sprintf("/admin/accounts/%d", id)
	_, err = upstream[struct{}](r.Context(), p.upstream, "POST", path+"/schedulable", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	data, err := upstream[accountWire](r.Context(), p.upstream, "GET", path, a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if data.ID != id || data.Schedulable == nil {
		return fail(502, "invalid_upstream")
	}
	if *data.Schedulable != *input.On {
		return fail(409, "schedule_changed")
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func (p *panel) testAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Model string `json:"model_id,omitempty"`
	}](r)
	if err != nil || len(input.Model) > 128 {
		return fail(400, "invalid_input")
	}
	stream := p.upstream
	client := *stream.client
	client.Timeout = 70 * time.Second
	stream.client = &client
	res, err := stream.open(r.Context(), "POST", fmt.Sprintf("/api/v1/admin/accounts/%d/test", id), a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return upstreamStatus{res.StatusCode}
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		return fail(502, "invalid_upstream")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 2048), 256<<10)
	start := time.Now()
	emit(w, "running")
	for scanner.Scan() {
		frame, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		var input struct {
			Type    string `json:"type"`
			Success bool   `json:"success"`
		}
		if json.Unmarshal([]byte(frame), &input) == nil {
			if input.Type == "test_complete" && input.Success {
				emit(w, "passed")
				slogAccountResult(r.Context(), id, true, start)
				return nil
			}
			if input.Type == "error" || input.Type == "test_complete" {
				break
			}
		}
	}
	emit(w, "failed")
	slogAccountResult(r.Context(), id, false, start)
	return nil
}

func emit(w http.ResponseWriter, state string) {
	fmt.Fprintf(w, "data: {\"state\":\"%s\"}\n\n", state)
	_ = http.NewResponseController(w).Flush()
}

func slogAccountResult(ctx context.Context, id int64, passed bool, start time.Time) {
	slog.Info("account test finished", "trace_id", meta(ctx).Trace, "account_id", id,
		"passed", passed, "elapsed_ms", time.Since(start).Milliseconds())
}
