package main

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
)

var quotaPlatformName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

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

func (p *panel) userPlatformQuotas(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/admin/users/%d/platform-quotas", id)
	var body any
	switch r.Method {
	case http.MethodPut:
		input, err := decodeBody[struct {
			Quotas []platformQuotaLimit `json:"quotas"`
		}](r)
		if err != nil || input.Quotas == nil || len(input.Quotas) > 64 {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		seen := make(map[string]bool, len(input.Quotas))
		for _, quota := range input.Quotas {
			if !quotaPlatformName.MatchString(quota.Platform) || seen[quota.Platform] {
				return fail(http.StatusBadRequest, "invalid_input")
			}
			seen[quota.Platform] = true
			for _, limit := range []*float64{quota.Daily, quota.Weekly, quota.Monthly} {
				if limit != nil && *limit < 0 {
					return fail(http.StatusBadRequest, "invalid_input")
				}
			}
		}
		body = input
	case http.MethodPost:
		input, err := decodeBody[struct {
			Platform string `json:"platform"`
			Window   string `json:"window"`
		}](r)
		if err != nil || !quotaPlatformName.MatchString(input.Platform) || !slices.Contains([]string{"daily", "weekly", "monthly"}, input.Window) {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		path += "/reset"
		body = input
	}
	result, err := upstream[struct {
		Quotas []platformQuotaView `json:"platform_quotas"`
	}](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", body)
	if result.Quotas == nil {
		result.Quotas = []platformQuotaView{}
	}
	return send(w, http.StatusOK, result, err)
}
