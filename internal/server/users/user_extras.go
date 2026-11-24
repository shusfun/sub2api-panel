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

type userAttributeDefinition struct {
	platform.NamedItem
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Options     []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"options"`
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
