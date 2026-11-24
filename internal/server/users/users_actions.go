package users

import (
	"fmt"
	"net/http"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func (h *handler) batchUpdateUserLimits(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		UserIDs     []int64 `json:"user_ids"`
		Concurrency *int    `json:"concurrency,omitempty"`
		RPMLimit    *int    `json:"rpm_limit,omitempty"`
	}](r)
	if err != nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 200 ||
		input.Concurrency == nil && input.RPMLimit == nil ||
		input.Concurrency != nil && (*input.Concurrency < 0 || *input.Concurrency > 10000) ||
		input.RPMLimit != nil && (*input.RPMLimit < 0 || *input.RPMLimit > 100000) {
		return platform.Fail(400, "invalid_input")
	}
	for _, id := range input.UserIDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	res, err := platform.Upstream[struct {
		Affected int `json:"affected"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/users/batch-limits", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, res, err)
}

func (h *handler) batchDisableUsers(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	return h.batchUserStatus(w, r, a, "/admin/users/batch-disable")
}

func (h *handler) batchEnableUsers(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	return h.batchUserStatus(w, r, a, "/admin/users/batch-enable")
}

func (h *handler) batchUserStatus(w http.ResponseWriter, r *http.Request, a core.SessionAuth, upstreamPath string) error {
	input, err := platform.DecodeBody[struct {
		UserIDs []int64 `json:"user_ids"`
	}](r)
	if err != nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 200 {
		return platform.Fail(400, "invalid_input")
	}
	for _, id := range input.UserIDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	res, err := platform.Upstream[struct {
		Affected int `json:"affected"`
	}](r.Context(), h.s.Upstream, "POST", upstreamPath, a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, res, err)
}

func (h *handler) adminUpdateAPIKey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		GroupID        *int64   `json:"group_id,omitempty"`
		Status         string   `json:"status,omitempty"`
		Quota          *float64 `json:"quota,omitempty"`
		ResetUsage     bool     `json:"reset_usage,omitempty"`
		RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
		AllowedGroups  []int64  `json:"allowed_groups,omitempty"`
		IPWhitelist    []string `json:"ip_whitelist,omitempty"`
		IPBlacklist    []string `json:"ip_blacklist,omitempty"`
	}](r)
	if err != nil {
		return platform.Fail(400, "invalid_input")
	}
	if input.GroupID != nil && *input.GroupID < 0 ||
		input.Status != "" && input.Status != "active" && input.Status != "disabled" ||
		input.Quota != nil && (*input.Quota < 0 || *input.Quota > 1_000_000) ||
		input.RateMultiplier != nil && (*input.RateMultiplier <= 0 || *input.RateMultiplier > 100) ||
		len(input.AllowedGroups) > 200 || len(input.IPWhitelist) > 200 || len(input.IPBlacklist) > 200 {
		return platform.Fail(400, "invalid_input")
	}
	body := map[string]any{}
	if input.GroupID != nil {
		body["group_id"] = *input.GroupID
	}
	if input.Status != "" {
		body["status"] = input.Status
	}
	if input.Quota != nil {
		body["quota"] = *input.Quota
	}
	if input.ResetUsage {
		body["reset_usage"] = true
	}
	if input.RateMultiplier != nil {
		body["rate_multiplier"] = *input.RateMultiplier
	}
	if input.AllowedGroups != nil {
		body["allowed_groups"] = input.AllowedGroups
	}
	if input.IPWhitelist != nil {
		body["ip_whitelist"] = input.IPWhitelist
	}
	if input.IPBlacklist != nil {
		body["ip_blacklist"] = input.IPBlacklist
	}
	if len(body) == 0 {
		return platform.Fail(400, "invalid_input")
	}
	type resultView struct {
		AutoGranted bool   `json:"auto_granted_group_access"`
		GroupName   string `json:"granted_group_name,omitempty"`
	}
	result, err := platform.Upstream[resultView](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/api-keys/%d", id), a.Tokens.Access, "", "", body)
	return platform.Send(w, 200, result, err)
}

func (h *handler) adminDeleteAPIKey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/api-keys/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) replaceUserGroup(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		OldID int64 `json:"old_group_id"`
		NewID int64 `json:"new_group_id"`
	}](r)
	if err != nil || input.OldID < 1 || input.NewID < 1 || input.OldID == input.NewID {
		return platform.Fail(400, "invalid_input")
	}
	result, err := platform.Upstream[struct {
		Migrated int `json:"migrated_keys"`
	}](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/users/%d/replace-group", id), a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, result, err)
}
