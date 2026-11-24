package main

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

type adminUserWire struct {
	ID                 int64    `json:"id"`
	Email              string   `json:"email"`
	Username           string   `json:"username"`
	Role               string   `json:"role"`
	Status             string   `json:"status"`
	Balance            *float64 `json:"balance"`
	Concurrency        int      `json:"concurrency"`
	CurrentConcurrency int      `json:"current_concurrency"`
	RPMLimit           int      `json:"rpm_limit"`
	AllowedGroups      []int64  `json:"allowed_groups"`
	RestrictPublic     bool     `json:"restrict_public_groups"`
	Notes              string   `json:"notes"`
	LastUsedAt         *string  `json:"last_used_at"`
	CreatedAt          string   `json:"created_at"`
}

type adminUserView struct {
	ID                 int64    `json:"id"`
	Email              string   `json:"email"`
	Username           string   `json:"username"`
	Role               string   `json:"role"`
	Status             string   `json:"status"`
	Balance            *float64 `json:"balance"`
	Concurrency        int      `json:"concurrency"`
	CurrentConcurrency int      `json:"current_concurrency"`
	RPMLimit           int      `json:"rpm_limit"`
	AllowedGroups      []int64  `json:"allowed_groups"`
	RestrictPublic     bool     `json:"restrict_public_groups"`
	Notes              string   `json:"notes"`
	LastUsedAt         *string  `json:"last_used_at,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty"`
}

func projectAdminUser(u adminUserWire) (adminUserView, error) {
	if u.ID < 1 || u.Email == "" {
		return adminUserView{}, fail(502, "invalid_upstream")
	}
	role := u.Role
	if role == "" {
		role = "user"
	}
	groups := u.AllowedGroups
	if groups == nil {
		groups = []int64{}
	}
	return adminUserView{
		ID: u.ID, Email: u.Email, Username: u.Username, Role: role,
		Status: u.Status, Balance: u.Balance,
		Concurrency: u.Concurrency, CurrentConcurrency: u.CurrentConcurrency,
		RPMLimit: u.RPMLimit, AllowedGroups: groups, RestrictPublic: u.RestrictPublic,
		Notes: u.Notes, LastUsedAt: u.LastUsedAt, CreatedAt: u.CreatedAt,
	}, nil
}

type adminSubscriptionWire struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	GroupID         int64     `json:"group_id"`
	Status          string    `json:"status"`
	Expires         time.Time `json:"expires_at"`
	CreatedAt       string    `json:"created_at"`
	DailyUsageUSD   float64   `json:"daily_usage_usd"`
	WeeklyUsageUSD  float64   `json:"weekly_usage_usd"`
	MonthlyUsageUSD float64   `json:"monthly_usage_usd"`
	Notes           string    `json:"notes"`
	User            *struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	Group *struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Platform string `json:"platform"`
	} `json:"group"`
}

type adminSubscriptionView struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	UserEmail       string    `json:"user_email"`
	GroupID         int64     `json:"group_id"`
	Group           string    `json:"group"`
	Platform        string    `json:"platform,omitempty"`
	Status          string    `json:"status"`
	Expires         time.Time `json:"expires_at"`
	DailyUsageUSD   float64   `json:"daily_usage_usd"`
	WeeklyUsageUSD  float64   `json:"weekly_usage_usd"`
	MonthlyUsageUSD float64   `json:"monthly_usage_usd"`
	Notes           string    `json:"notes,omitempty"`
}

func projectAdminSubscription(s adminSubscriptionWire) (adminSubscriptionView, error) {
	if s.ID < 1 {
		return adminSubscriptionView{}, fail(502, "invalid_upstream")
	}
	groupName := fmt.Sprintf("分组 %d", s.GroupID)
	platform := ""
	if s.Group != nil {
		if s.Group.ID > 0 && s.GroupID == 0 {
			s.GroupID = s.Group.ID
		}
		if s.Group.Name != "" {
			groupName = s.Group.Name
		}
		platform = s.Group.Platform
	}
	email := ""
	if s.User != nil {
		if s.User.ID > 0 && s.UserID == 0 {
			s.UserID = s.User.ID
		}
		email = s.User.Email
	}
	return adminSubscriptionView{
		ID: s.ID, UserID: s.UserID, UserEmail: email,
		GroupID: s.GroupID, Group: groupName, Platform: platform,
		Status: s.Status, Expires: s.Expires,
		DailyUsageUSD: s.DailyUsageUSD, WeeklyUsageUSD: s.WeeklyUsageUSD, MonthlyUsageUSD: s.MonthlyUsageUSD,
		Notes: s.Notes,
	}, nil
}

type balanceHistoryRow struct {
	ID        int64   `json:"id"`
	Type      string  `json:"type"`
	Value     float64 `json:"value"`
	Status    string  `json:"status"`
	Notes     string  `json:"notes"`
	UsedAt    *string `json:"used_at"`
	CreatedAt string  `json:"created_at"`
	Group     *struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"group,omitempty"`
}

func (p *panel) userAdminRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/admin/users", true, p.adminUsersList)
	p.route(mux, "GET /api/admin/users/{user}", true, p.adminUserDetail)
	p.route(mux, "POST /api/admin/users", true, p.createAdminUser)
	p.route(mux, "PUT /api/admin/users/{user}", true, p.updateAdminUser)
	p.route(mux, "PUT /api/admin/users/{user}/private-groups", true, p.updatePrivateUserGroups)
	p.route(mux, "POST /api/admin/users/{user}/replace-group", true, p.replaceUserGroup)
	p.route(mux, "GET /api/admin/users/{user}/platform-quotas", true, p.userPlatformQuotas)
	p.route(mux, "PUT /api/admin/users/{user}/platform-quotas", true, p.userPlatformQuotas)
	p.route(mux, "POST /api/admin/users/{user}/platform-quotas/reset", true, p.userPlatformQuotas)
	p.route(mux, "GET /api/admin/user-attributes", true, p.userAttributeDefinitions)
	p.route(mux, "GET /api/admin/users/{user}/attributes", true, p.userAttributes)
	p.route(mux, "PUT /api/admin/users/{user}/attributes", true, p.userAttributes)
	p.route(mux, "DELETE /api/admin/users/{user}", true, p.deleteAdminUser)
	p.route(mux, "POST /api/admin/users/{user}/balance", true, p.adjustAdminUserBalance)
	p.route(mux, "GET /api/admin/users/{user}/balance-history", true, p.adminUserBalanceHistory)
	p.route(mux, "POST /api/admin/users/batch-limits", true, p.batchUpdateUserLimits)
	p.route(mux, "POST /api/admin/users/batch-disable", true, p.batchDisableUsers)
	p.route(mux, "PUT /api/admin/api-keys/{id}", true, p.adminUpdateKeyGroup)
	p.route(mux, "GET /api/admin/subscriptions", true, p.adminSubscriptionsList)
	p.route(mux, "POST /api/admin/subscriptions/assign", true, p.assignAdminSubscription)
	p.route(mux, "POST /api/admin/subscriptions/bulk-assign", true, p.bulkAssignAdminSubscriptions)
	p.route(mux, "POST /api/admin/subscriptions/bulk-action", true, p.bulkActionAdminSubscriptions)
	p.route(mux, "POST /api/admin/subscriptions/{id}/extend", true, p.extendAdminSubscription)
	p.route(mux, "POST /api/admin/subscriptions/{id}/reset-quota", true, p.subAction("reset-quota"))
	p.route(mux, "POST /api/admin/subscriptions/{id}/revoke", true, p.subAction("revoke"))
	p.route(mux, "POST /api/admin/subscriptions/{id}/restore", true, p.subAction("restore"))
}

func (p *panel) adminUsersList(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "search", "status", "role", "group_name", "api_key_group_id", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/users", page, params, projectAdminUser)
	return send(w, 200, rows, err)
}

func (p *panel) adminUserDetail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	raw, err := upstream[adminUserWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/users/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return send(w, 200, view, err)
}

func (p *panel) createAdminUser(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Email         string   `json:"email"`
		Password      string   `json:"password"`
		Username      string   `json:"username,omitempty"`
		Notes         string   `json:"notes,omitempty"`
		Role          string   `json:"role,omitempty"`
		Balance       *float64 `json:"balance,omitempty"`
		Concurrency   *int     `json:"concurrency,omitempty"`
		RPMLimit      *int     `json:"rpm_limit,omitempty"`
		AllowedGroups []int64  `json:"allowed_groups,omitempty"`
	}](r)
	if err != nil || strings.TrimSpace(input.Email) == "" || len(input.Email) > 120 ||
		len(input.Password) < 6 || len(input.Password) > 128 ||
		input.Role != "" && input.Role != "user" && input.Role != "admin" ||
		input.Balance != nil && *input.Balance < 0 ||
		input.Concurrency != nil && (*input.Concurrency < 0 || *input.Concurrency > 10000) ||
		input.RPMLimit != nil && (*input.RPMLimit < 0 || *input.RPMLimit > 100000) {
		return fail(400, "invalid_input")
	}
	if len(input.AllowedGroups) > 0 {
		var groups map[int64]adminGroupWire
		groups, input.AllowedGroups, err = p.privateGroupSelection(r, a, input.AllowedGroups)
		if err != nil {
			return err
		}
		for _, groupID := range input.AllowedGroups {
			if groups[groupID].Status != "active" {
				return fail(400, "invalid_input")
			}
		}
	}
	raw, err := upstream[adminUserWire](r.Context(), p.upstream, "POST", "/admin/users", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return send(w, 201, view, err)
}

func (p *panel) privateGroupSelection(r *http.Request, a sessionAuth, selected []int64) (map[int64]adminGroupWire, []int64, error) {
	groups, err := upstream[[]adminGroupWire](r.Context(), p.upstream, "GET", "/admin/groups/all?include_inactive=true", a.tokens.Access, "", "", nil)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[int64]adminGroupWire, len(groups))
	for _, group := range groups {
		if group.ID < 1 || byID[group.ID].ID != 0 {
			return nil, nil, fail(502, "invalid_upstream")
		}
		byID[group.ID] = group
	}
	ids := make([]int64, 0, len(selected))
	for _, id := range selected {
		if !byID[id].IsExclusive {
			return nil, nil, fail(400, "invalid_input")
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return byID, slices.Compact(ids), nil
}

func (p *panel) updatePrivateUserGroups(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		IDs []int64 `json:"group_ids"`
	}](r)
	if err != nil || input.IDs == nil {
		return fail(400, "invalid_input")
	}
	groups, selected, err := p.privateGroupSelection(r, a, input.IDs)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/admin/users/%d", id)
	current, err := upstream[adminUserWire](r.Context(), p.upstream, "GET", path, a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if current.ID != id {
		return fail(502, "invalid_upstream")
	}
	for _, groupID := range selected {
		if groups[groupID].Status != "active" && !slices.Contains(current.AllowedGroups, groupID) {
			return fail(400, "invalid_input")
		}
	}
	for _, groupID := range current.AllowedGroups {
		if !groups[groupID].IsExclusive {
			selected = append(selected, groupID)
		}
	}
	updated, err := upstream[adminUserWire](r.Context(), p.upstream, "PUT", path, a.tokens.Access, "", "", map[string][]int64{"allowed_groups": selected})
	if err != nil {
		return err
	}
	view, err := projectAdminUser(updated)
	return send(w, 200, view, err)
}

func (p *panel) updateAdminUser(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Username    *string `json:"username,omitempty"`
		Password    *string `json:"password,omitempty"`
		Notes       *string `json:"notes,omitempty"`
		Status      string  `json:"status,omitempty"`
		Concurrency *int    `json:"concurrency,omitempty"`
		RPMLimit    *int    `json:"rpm_limit,omitempty"`
	}](r)
	if err != nil ||
		input.Status != "" && input.Status != "active" && input.Status != "disabled" ||
		input.Password != nil && (*input.Password != "" && (len(*input.Password) < 6 || len(*input.Password) > 128)) ||
		input.Concurrency != nil && (*input.Concurrency < 0 || *input.Concurrency > 10000) ||
		input.RPMLimit != nil && (*input.RPMLimit < 0 || *input.RPMLimit > 100000) {
		return fail(400, "invalid_input")
	}
	body := map[string]any{}
	if input.Username != nil {
		body["username"] = *input.Username
	}
	if input.Password != nil && *input.Password != "" {
		body["password"] = *input.Password
	}
	if input.Notes != nil {
		body["notes"] = *input.Notes
	}
	if input.Status != "" {
		body["status"] = input.Status
	}
	if input.Concurrency != nil {
		body["concurrency"] = *input.Concurrency
	}
	if input.RPMLimit != nil {
		body["rpm_limit"] = *input.RPMLimit
	}
	if len(body) == 0 {
		return fail(400, "invalid_input")
	}
	raw, err := upstream[adminUserWire](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/users/%d", id), a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return send(w, 200, view, err)
}

func (p *panel) deleteAdminUser(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	if id == a.user.ID {
		return fail(400, "invalid_input")
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/users/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) adjustAdminUserBalance(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Balance   float64 `json:"balance"`
		Operation string  `json:"operation"`
		Notes     string  `json:"notes,omitempty"`
	}](r)
	if err != nil || input.Balance < 0 || input.Balance > 1_000_000 || len(input.Notes) > 255 ||
		(input.Operation != "add" && input.Operation != "subtract" && input.Operation != "set") ||
		(input.Operation != "set" && input.Balance == 0) {
		return fail(400, "invalid_input")
	}
	raw, err := upstream[adminUserWire](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/users/%d/balance", id), a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return send(w, 200, view, err)
}

func (p *panel) adminUserBalanceHistory(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	page, params, err := listQuery(r, "type")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, fmt.Sprintf("/admin/users/%d/balance-history", id), page, params, func(item balanceHistoryRow) (balanceHistoryRow, error) {
		if item.ID < 1 {
			return balanceHistoryRow{}, fail(502, "invalid_upstream")
		}
		return item, nil
	})
	return send(w, 200, rows, err)
}

func (p *panel) batchUpdateUserLimits(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		UserIDs     []int64 `json:"user_ids"`
		Concurrency *int    `json:"concurrency,omitempty"`
		RPMLimit    *int    `json:"rpm_limit,omitempty"`
	}](r)
	if err != nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 200 ||
		input.Concurrency == nil && input.RPMLimit == nil ||
		input.Concurrency != nil && (*input.Concurrency < 0 || *input.Concurrency > 10000) ||
		input.RPMLimit != nil && (*input.RPMLimit < 0 || *input.RPMLimit > 100000) {
		return fail(400, "invalid_input")
	}
	for _, id := range input.UserIDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	res, err := upstream[struct {
		Affected int `json:"affected"`
	}](r.Context(), p.upstream, "POST", "/admin/users/batch-limits", a.tokens.Access, "", "", input)
	return send(w, 200, res, err)
}

func (p *panel) batchDisableUsers(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		UserIDs []int64 `json:"user_ids"`
	}](r)
	if err != nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 200 {
		return fail(400, "invalid_input")
	}
	for _, id := range input.UserIDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	res, err := upstream[struct {
		Affected int `json:"affected"`
	}](r.Context(), p.upstream, "POST", "/admin/users/batch-disable", a.tokens.Access, "", "", input)
	return send(w, 200, res, err)
}

func (p *panel) adminUpdateKeyGroup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		GroupID *int64 `json:"group_id"`
	}](r)
	if err != nil {
		return err
	}
	groupID := int64(0)
	if input.GroupID != nil {
		if *input.GroupID < 0 {
			return fail(400, "invalid_input")
		}
		groupID = *input.GroupID
	}
	type resultView struct {
		AutoGranted bool   `json:"auto_granted_group_access"`
		GroupName   string `json:"granted_group_name,omitempty"`
	}
	result, err := upstream[resultView](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/api-keys/%d", id), a.tokens.Access, "", "", map[string]any{"group_id": groupID})
	return send(w, 200, result, err)
}

func (p *panel) replaceUserGroup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		OldID int64 `json:"old_group_id"`
		NewID int64 `json:"new_group_id"`
	}](r)
	if err != nil || input.OldID < 1 || input.NewID < 1 || input.OldID == input.NewID {
		return fail(400, "invalid_input")
	}
	result, err := upstream[struct {
		Migrated int `json:"migrated_keys"`
	}](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/users/%d/replace-group", id), a.tokens.Access, "", "", input)
	return send(w, 200, result, err)
}

func (p *panel) adminSubscriptionsList(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "status", "user_id", "group_id", "platform", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	for _, field := range []string{"user_id", "group_id"} {
		if raw := params.Get(field); raw != "" {
			if val, pErr := strconv.ParseInt(raw, 10, 64); pErr != nil || val < 1 {
				return fail(400, "invalid_input")
			}
		}
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/subscriptions", page, params, projectAdminSubscription)
	return send(w, 200, rows, err)
}

func (p *panel) userSubscriptions(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/subscriptions", page,
		url.Values{"user_id": {strconv.FormatInt(id, 10)}}, projectAdminSubscription)
	return send(w, 200, rows, err)
}

func (p *panel) assignAdminSubscription(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		UserID       int64  `json:"user_id"`
		GroupID      int64  `json:"group_id"`
		ValidityDays int    `json:"validity_days"`
		Notes        string `json:"notes,omitempty"`
	}](r)
	if err != nil || input.UserID < 1 || input.GroupID < 1 || input.ValidityDays < 1 || input.ValidityDays > 36500 || len(input.Notes) > 255 {
		return fail(400, "invalid_input")
	}
	raw, err := upstream[adminSubscriptionWire](r.Context(), p.upstream, "POST", "/admin/subscriptions/assign", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminSubscription(raw)
	return send(w, 201, view, err)
}

func (p *panel) bulkAssignAdminSubscriptions(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		UserIDs      []int64 `json:"user_ids"`
		GroupID      int64   `json:"group_id"`
		ValidityDays int     `json:"validity_days"`
		Notes        string  `json:"notes,omitempty"`
	}](r)
	if err != nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 100 || input.GroupID < 1 || input.ValidityDays < 1 || input.ValidityDays > 36500 {
		return fail(400, "invalid_input")
	}
	for _, uid := range input.UserIDs {
		if uid < 1 {
			return fail(400, "invalid_input")
		}
	}
	res, err := upstream[struct {
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}](r.Context(), p.upstream, "POST", "/admin/subscriptions/bulk-assign", a.tokens.Access, "", "", input)
	return send(w, 200, res, err)
}

func (p *panel) bulkActionAdminSubscriptions(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		SubscriptionIDs []int64 `json:"subscription_ids"`
		Action          string  `json:"action"`
		Days            int     `json:"days,omitempty"`
		Daily           bool    `json:"daily,omitempty"`
		Weekly          bool    `json:"weekly,omitempty"`
		Monthly         bool    `json:"monthly,omitempty"`
		Operation       string  `json:"operation_id,omitempty"`
	}](r)
	if err != nil || len(input.SubscriptionIDs) == 0 || len(input.SubscriptionIDs) > 200 ||
		(input.Action != "extend" && input.Action != "reset_quota" && input.Action != "revoke" && input.Action != "restore") ||
		input.Action == "extend" && (input.Days == 0 || input.Days < -36500 || input.Days > 36500) {
		return fail(400, "invalid_input")
	}
	for _, sid := range input.SubscriptionIDs {
		if sid < 1 {
			return fail(400, "invalid_input")
		}
	}
	res, err := upstream[struct {
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}](r.Context(), p.upstream, "POST", "/admin/subscriptions/bulk-action", a.tokens.Access, "", input.Operation, map[string]any{
		"subscription_ids": input.SubscriptionIDs,
		"action":           input.Action,
		"days":             input.Days,
		"daily":            input.Daily,
		"weekly":           input.Weekly,
		"monthly":          input.Monthly,
	})
	return send(w, 200, res, err)
}

func (p *panel) extendAdminSubscription(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Days int `json:"days"`
	}](r)
	if err != nil || input.Days == 0 || input.Days < -36500 || input.Days > 36500 {
		return fail(400, "invalid_input")
	}
	raw, err := upstream[adminSubscriptionWire](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/subscriptions/%d/extend", id), a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminSubscription(raw)
	return send(w, 200, view, err)
}

func (p *panel) subAction(action string) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		_, err = upstream[map[string]any](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/subscriptions/%d/%s", id, action), a.tokens.Access, "", "", struct{}{})
		return send(w, 200, struct{}{}, err)
	}
}
