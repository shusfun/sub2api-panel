package users

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const userAttributeTextLimit = 4000

var quotaPlatformName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type userAttributeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type userAttributeDefinition struct {
	platform.NamedItem
	Key         string                `json:"key,omitempty"`
	Label       string                `json:"label,omitempty"`
	Type        string                `json:"type"`
	Description string                `json:"description"`
	Required    bool                  `json:"required"`
	Enabled     bool                  `json:"enabled"`
	SortOrder   int                   `json:"sort_order"`
	Options     []userAttributeOption `json:"options"`
}

type userAttributeUpsertInput struct {
	Key         string                `json:"key"`
	Label       string                `json:"label"`
	Name        string                `json:"name,omitempty"`
	Type        string                `json:"type"`
	Description string                `json:"description,omitempty"`
	Required    bool                  `json:"required"`
	Enabled     *bool                 `json:"enabled,omitempty"`
	Options     []userAttributeOption `json:"options,omitempty"`
}

func validAttributeType(kind string) bool {
	return kind == "text" || kind == "number" || kind == "select" || kind == "boolean"
}

type userAttributeValue struct {
	ID    int64  `json:"attribute_id"`
	Value string `json:"value"`
}

func (h *handler) userAttributeDefinitions(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	result, err := platform.Upstream[[]userAttributeDefinition](r.Context(), h.s.Upstream, http.MethodGet, "/admin/user-attributes?enabled=true", a.Tokens.Access, "", "", nil)
	if result == nil {
		result = []userAttributeDefinition{}
	}
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) allUserAttributeDefinitions(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	result, err := platform.Upstream[[]userAttributeDefinition](r.Context(), h.s.Upstream, http.MethodGet, "/admin/user-attributes", a.Tokens.Access, "", "", nil)
	if result == nil {
		result = []userAttributeDefinition{}
	}
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) createUserAttributeDefinition(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[userAttributeUpsertInput](r)
	if err != nil || len(input.Key) == 0 || len(input.Key) > 64 || !validAttributeType(input.Type) ||
		(len(input.Label) == 0 && len(input.Name) == 0) || len(input.Label) > 120 || len(input.Name) > 120 || len(input.Description) > 500 || len(input.Options) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	if input.Name == "" {
		input.Name = input.Label
	}
	if input.Label == "" {
		input.Label = input.Name
	}
	result, err := platform.Upstream[userAttributeDefinition](r.Context(), h.s.Upstream, http.MethodPost, "/admin/user-attributes", a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusCreated, result, err)
}

func (h *handler) updateUserAttributeDefinition(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[userAttributeUpsertInput](r)
	if err != nil || len(input.Key) > 64 || (input.Type != "" && !validAttributeType(input.Type)) ||
		len(input.Label) > 120 || len(input.Name) > 120 || len(input.Description) > 500 || len(input.Options) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	if input.Name == "" && input.Label != "" {
		input.Name = input.Label
	}
	result, err := platform.Upstream[userAttributeDefinition](r.Context(), h.s.Upstream, http.MethodPut, fmt.Sprintf("/admin/user-attributes/%d", id), a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) deleteUserAttributeDefinition(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodDelete, fmt.Sprintf("/admin/user-attributes/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, struct{}{}, err)
}

func (h *handler) reorderUserAttributeDefinitions(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodPut, "/admin/user-attributes/reorder", a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, struct{}{}, err)
}

func (h *handler) userAttributes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	var payload any
	if r.Method == http.MethodPut {
		input, err := platform.DecodeBody[struct {
			Values map[int64]string `json:"values"`
		}](r)
		if err != nil || len(input.Values) == 0 || len(input.Values) > 100 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		for attrID, value := range input.Values {
			if attrID < 1 || len(value) > userAttributeTextLimit {
				return platform.Fail(http.StatusBadRequest, "invalid_input")
			}
		}
		payload = input
	}
	result, err := platform.Upstream[[]userAttributeValue](r.Context(), h.s.Upstream, r.Method, fmt.Sprintf("/admin/users/%d/attributes", id), a.Tokens.Access, "", "", payload)
	if result == nil {
		result = []userAttributeValue{}
	}
	return platform.Send(w, http.StatusOK, result, err)
}

type platformQuotaLimit struct {
	Platform string   `json:"platform"`
	Daily    *float64 `json:"daily_limit_usd"`
	Weekly   *float64 `json:"weekly_limit_usd"`
	Monthly  *float64 `json:"monthly_limit_usd"`
}

type platformQuotaView struct {
	platformQuotaLimit
	DailyUsage   float64 `json:"daily_usage_usd"`
	WeeklyUsage  float64 `json:"weekly_usage_usd"`
	MonthlyUsage float64 `json:"monthly_usage_usd"`
}

func (h *handler) userPlatformQuotas(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/admin/users/%d/platform-quotas", id)
	var body any
	switch r.Method {
	case http.MethodPut:
		input, err := platform.DecodeBody[struct {
			Quotas []platformQuotaLimit `json:"quotas"`
		}](r)
		if err != nil || input.Quotas == nil || len(input.Quotas) > 64 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		seen := make(map[string]bool, len(input.Quotas))
		for _, quota := range input.Quotas {
			if !quotaPlatformName.MatchString(quota.Platform) || seen[quota.Platform] {
				return platform.Fail(http.StatusBadRequest, "invalid_input")
			}
			seen[quota.Platform] = true
			for _, lim := range []*float64{quota.Daily, quota.Weekly, quota.Monthly} {
				if lim != nil && *lim < 0 {
					return platform.Fail(http.StatusBadRequest, "invalid_input")
				}
			}
		}
		body = input
	case http.MethodPost:
		input, err := platform.DecodeBody[struct {
			Platform string `json:"platform"`
			Window   string `json:"window"`
		}](r)
		if err != nil || !quotaPlatformName.MatchString(input.Platform) || !slices.Contains([]string{"daily", "weekly", "monthly"}, input.Window) {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		path += "/reset"
		body = input
	}
	result, err := platform.Upstream[struct {
		Quotas []platformQuotaView `json:"platform_quotas"`
	}](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", body)
	if result.Quotas == nil {
		result.Quotas = []platformQuotaView{}
	}
	return platform.Send(w, http.StatusOK, result, err)
}
