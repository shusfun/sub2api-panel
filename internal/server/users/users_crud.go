package users

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func RegisterRoutes(mux *http.ServeMux, s *core.Server) {
	h := &handler{s: s}
	h.registerKeyRoutes(mux)
	h.registerMCPRoutes(mux)
	h.registerUserAdminRoutes(mux)
}

type handler struct {
	s *core.Server
}

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

type adminGroupFlag struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	IsExclusive bool   `json:"is_exclusive"`
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

func projectAdminUser(u adminUserWire) (adminUserView, error) {
	if u.ID < 1 || u.Email == "" {
		return adminUserView{}, platform.Fail(502, "invalid_upstream")
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

func (h *handler) registerUserAdminRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/users", true, h.adminUsersList)
	h.s.Route(mux, "GET /api/admin/users/{user}", true, h.adminUserDetail)
	h.s.Route(mux, "POST /api/admin/users", true, h.createAdminUser)
	h.s.Route(mux, "PUT /api/admin/users/{user}", true, h.updateAdminUser)
	h.s.Route(mux, "PUT /api/admin/users/{user}/private-groups", true, h.updatePrivateUserGroups)
	h.s.Route(mux, "POST /api/admin/users/{user}/replace-group", true, h.replaceUserGroup)
	h.s.Route(mux, "GET /api/admin/users/{user}/platform-quotas", true, h.userPlatformQuotas)
	h.s.Route(mux, "PUT /api/admin/users/{user}/platform-quotas", true, h.userPlatformQuotas)
	h.s.Route(mux, "POST /api/admin/users/{user}/platform-quotas/reset", true, h.userPlatformQuotas)
	h.s.Route(mux, "GET /api/admin/user-attributes", true, h.userAttributeDefinitions)
	h.s.Route(mux, "GET /api/admin/user-attributes/all", true, h.allUserAttributeDefinitions)
	h.s.Route(mux, "POST /api/admin/user-attributes", true, h.createUserAttributeDefinition)
	h.s.Route(mux, "PUT /api/admin/user-attributes/reorder", true, h.reorderUserAttributeDefinitions)
	h.s.Route(mux, "PUT /api/admin/user-attributes/{id}", true, h.updateUserAttributeDefinition)
	h.s.Route(mux, "DELETE /api/admin/user-attributes/{id}", true, h.deleteUserAttributeDefinition)
	h.s.Route(mux, "GET /api/admin/users/{user}/attributes", true, h.userAttributes)
	h.s.Route(mux, "PUT /api/admin/users/{user}/attributes", true, h.userAttributes)
	h.s.Route(mux, "DELETE /api/admin/users/{user}", true, h.deleteAdminUser)
	h.s.Route(mux, "POST /api/admin/users/{user}/balance", true, h.adjustAdminUserBalance)
	h.s.Route(mux, "GET /api/admin/users/{user}/balance-history", true, h.adminUserBalanceHistory)
	h.s.Route(mux, "POST /api/admin/users/batch-limits", true, h.batchUpdateUserLimits)
	h.s.Route(mux, "POST /api/admin/users/batch-disable", true, h.batchDisableUsers)
	h.s.Route(mux, "POST /api/admin/users/batch-enable", true, h.batchEnableUsers)
	h.s.Route(mux, "PUT /api/admin/api-keys/{id}", true, h.adminUpdateAPIKey)
	h.s.Route(mux, "DELETE /api/admin/api-keys/{id}", true, h.adminDeleteAPIKey)
	h.s.Route(mux, "GET /api/admin/users/{user}/subscriptions", true, h.userSubscriptions)
	h.s.Route(mux, "GET /api/admin/subscriptions", true, h.adminSubscriptionsList)
	h.s.Route(mux, "POST /api/admin/subscriptions/assign", true, h.assignAdminSubscription)
	h.s.Route(mux, "POST /api/admin/subscriptions/bulk-assign", true, h.bulkAssignAdminSubscriptions)
	h.s.Route(mux, "POST /api/admin/subscriptions/bulk-action", true, h.bulkActionAdminSubscriptions)
	h.s.Route(mux, "POST /api/admin/subscriptions/{id}/extend", true, h.extendAdminSubscription)
	h.s.Route(mux, "POST /api/admin/subscriptions/{id}/reset-quota", true, h.subAction("reset-quota"))
	h.s.Route(mux, "POST /api/admin/subscriptions/{id}/revoke", true, h.subAction("revoke"))
	h.s.Route(mux, "POST /api/admin/subscriptions/{id}/restore", true, h.subAction("restore"))
}

func (h *handler) adminUsersList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, params, err := platform.ListQuery(r, "search", "status", "role", "group_name", "api_key_group_id", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/users", page, params, projectAdminUser)
	return platform.Send(w, 200, rows, err)
}

func (h *handler) adminUserDetail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[adminUserWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/users/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) createAdminUser(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
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
		return platform.Fail(400, "invalid_input")
	}
	if len(input.AllowedGroups) > 0 {
		var groups map[int64]adminGroupFlag
		groups, input.AllowedGroups, err = h.privateGroupSelection(r, a, input.AllowedGroups)
		if err != nil {
			return err
		}
		for _, groupID := range input.AllowedGroups {
			if groups[groupID].Status != "active" {
				return platform.Fail(400, "invalid_input")
			}
		}
	}
	raw, err := platform.Upstream[adminUserWire](r.Context(), h.s.Upstream, "POST", "/admin/users", a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return platform.Send(w, 201, view, err)
}

func (h *handler) privateGroupSelection(r *http.Request, a core.SessionAuth, selected []int64) (map[int64]adminGroupFlag, []int64, error) {
	groups, err := platform.Upstream[[]adminGroupFlag](r.Context(), h.s.Upstream, "GET", "/admin/groups/all?include_inactive=true", a.Tokens.Access, "", "", nil)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[int64]adminGroupFlag, len(groups))
	for _, group := range groups {
		if group.ID < 1 || byID[group.ID].ID != 0 {
			return nil, nil, platform.Fail(502, "invalid_upstream")
		}
		byID[group.ID] = group
	}
	ids := make([]int64, 0, len(selected))
	for _, id := range selected {
		if !byID[id].IsExclusive {
			return nil, nil, platform.Fail(400, "invalid_input")
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return byID, slices.Compact(ids), nil
}

func (h *handler) updatePrivateUserGroups(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		IDs []int64 `json:"group_ids"`
	}](r)
	if err != nil || input.IDs == nil {
		return platform.Fail(400, "invalid_input")
	}
	groups, selected, err := h.privateGroupSelection(r, a, input.IDs)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/admin/users/%d", id)
	current, err := platform.Upstream[adminUserWire](r.Context(), h.s.Upstream, "GET", path, a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if current.ID != id {
		return platform.Fail(502, "invalid_upstream")
	}
	for _, groupID := range selected {
		if groups[groupID].Status != "active" && !slices.Contains(current.AllowedGroups, groupID) {
			return platform.Fail(400, "invalid_input")
		}
	}
	for _, groupID := range current.AllowedGroups {
		if !groups[groupID].IsExclusive {
			selected = append(selected, groupID)
		}
	}
	updated, err := platform.Upstream[adminUserWire](r.Context(), h.s.Upstream, "PUT", path, a.Tokens.Access, "", "", map[string][]int64{"allowed_groups": selected})
	if err != nil {
		return err
	}
	view, err := projectAdminUser(updated)
	return platform.Send(w, 200, view, err)
}

func (h *handler) updateAdminUser(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Username       *string `json:"username,omitempty"`
		Password       *string `json:"password,omitempty"`
		Notes          *string `json:"notes,omitempty"`
		Role           string  `json:"role,omitempty"`
		Status         string  `json:"status,omitempty"`
		RestrictPublic *bool   `json:"restrict_public_groups,omitempty"`
		Concurrency    *int    `json:"concurrency,omitempty"`
		RPMLimit       *int    `json:"rpm_limit,omitempty"`
		TOTP           string  `json:"totp,omitempty"`
	}](r)
	if err != nil ||
		input.Role != "" && input.Role != "user" && input.Role != "admin" ||
		input.Status != "" && input.Status != "active" && input.Status != "disabled" ||
		input.Password != nil && (*input.Password != "" && (len(*input.Password) < 6 || len(*input.Password) > 128)) ||
		input.Concurrency != nil && (*input.Concurrency < 0 || *input.Concurrency > 10000) ||
		input.RPMLimit != nil && (*input.RPMLimit < 0 || *input.RPMLimit > 100000) {
		return platform.Fail(400, "invalid_input")
	}
	if input.Role != "" {
		if err := h.s.RequireOptionalStepUp(r, a, input.TOTP); err != nil {
			return err
		}
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
	if input.Role != "" {
		body["role"] = input.Role
	}
	if input.Status != "" {
		body["status"] = input.Status
	}
	if input.RestrictPublic != nil {
		body["restrict_public_groups"] = *input.RestrictPublic
	}
	if input.Concurrency != nil {
		body["concurrency"] = *input.Concurrency
	}
	if input.RPMLimit != nil {
		body["rpm_limit"] = *input.RPMLimit
	}
	if len(body) == 0 {
		return platform.Fail(400, "invalid_input")
	}
	raw, err := platform.Upstream[adminUserWire](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/users/%d", id), a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) deleteAdminUser(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	if id == a.User.ID {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/users/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) adjustAdminUserBalance(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Balance   float64 `json:"balance"`
		Operation string  `json:"operation"`
		Notes     string  `json:"notes,omitempty"`
	}](r)
	if err != nil || input.Balance < 0 || input.Balance > 1_000_000 || len(input.Notes) > 255 ||
		(input.Operation != "add" && input.Operation != "subtract" && input.Operation != "set") ||
		(input.Operation != "set" && input.Balance == 0) {
		return platform.Fail(400, "invalid_input")
	}
	raw, err := platform.Upstream[adminUserWire](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/users/%d/balance", id), a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminUser(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) adminUserBalanceHistory(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	page, params, err := platform.ListQuery(r, "type")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, fmt.Sprintf("/admin/users/%d/balance-history", id), page, params, func(item balanceHistoryRow) (balanceHistoryRow, error) {
		if item.ID < 1 {
			return balanceHistoryRow{}, platform.Fail(502, "invalid_upstream")
		}
		return item, nil
	})
	return platform.Send(w, 200, rows, err)
}
