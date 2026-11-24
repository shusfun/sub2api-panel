package pool

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type accountBundleRow struct {
	Name        string         `json:"name"`
	Notes       *string        `json:"notes,omitempty"`
	Platform    string         `json:"platform"`
	Type        string         `json:"type"`
	Credentials map[string]any `json:"credentials,omitempty"`
	ProxyKey    *string        `json:"proxy_key,omitempty"`
	Concurrency int            `json:"concurrency"`
	Priority    int            `json:"priority"`
	Rate        *float64       `json:"rate_multiplier,omitempty"`
	Expires     *int64         `json:"expires_at,omitempty"`
	AutoPause   *bool          `json:"auto_pause_on_expired,omitempty"`
}

type accountBundle struct {
	Type           string             `json:"type"`
	Version        int                `json:"version"`
	ExportedAt     string             `json:"exported_at"`
	Accounts       []accountBundleRow `json:"accounts"`
	SkippedShadows int                `json:"skipped_shadows,omitempty"`
}

func (h *handler) exportAccountData(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		IDs  []int64 `json:"ids"`
		TOTP string  `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if len(input.IDs) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	ids := make([]string, 0, len(input.IDs))
	for _, id := range input.IDs {
		if id <= 0 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	if err := h.s.StepUp(r, a, input.TOTP); err != nil {
		return err
	}
	query := url.Values{"include_proxies": {"false"}}
	if len(ids) > 0 {
		query.Set("ids", strings.Join(ids, ","))
	}
	wire, err := platform.Upstream[struct {
		accountBundle
		Accounts []struct {
			accountBundleRow
			Credentials struct {
				BaseURL      string            `json:"base_url"`
				ModelMapping map[string]string `json:"model_mapping"`
			} `json:"credentials"`
		} `json:"accounts"`
	}](r.Context(), h.s.Upstream, http.MethodGet, "/admin/accounts/data?"+query.Encode(), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	wire.Type, wire.Version = "sub2api-data", 1
	wire.accountBundle.Accounts = make([]accountBundleRow, 0, len(wire.Accounts))
	for _, item := range wire.Accounts {
		row := item.accountBundleRow
		row.ProxyKey = nil
		row.Credentials = make(map[string]any)
		if endpoint, err := url.Parse(item.Credentials.BaseURL); err == nil && endpoint.Hostname() != "" &&
			(endpoint.Scheme == "http" || endpoint.Scheme == "https") && endpoint.User == nil && endpoint.RawQuery == "" && endpoint.Fragment == "" {
			row.Credentials["base_url"] = endpoint.String()
		}
		if len(item.Credentials.ModelMapping) > 0 {
			row.Credentials["model_mapping"] = item.Credentials.ModelMapping
		}
		wire.accountBundle.Accounts = append(wire.accountBundle.Accounts, row)
	}
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, wire.accountBundle, nil)
}

func (h *handler) importAccountData(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Data      accountBundle `json:"data"`
		SkipGroup bool          `json:"skip_default_group_bind"`
	}](r)
	if err != nil {
		return err
	}
	if input.Data.Type != "sub2api-data" || input.Data.Version != 1 || len(input.Data.Accounts) == 0 || len(input.Data.Accounts) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	for _, row := range input.Data.Accounts {
		if _, err := (accountWriteInput{Name: row.Name, Platform: row.Platform, Type: row.Type, Credentials: row.Credentials,
			Concurrency: &row.Concurrency, Priority: &row.Priority, RateMultiplier: row.Rate}).toBody(true); err != nil || row.ProxyKey != nil {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
	}
	payload := map[string]any{"data": map[string]any{
		"type": input.Data.Type, "version": input.Data.Version, "exported_at": input.Data.ExportedAt,
		"accounts": input.Data.Accounts, "proxies": []any{},
	}, "skip_default_group_bind": input.SkipGroup}
	row, err := platform.Upstream[struct {
		Created int `json:"account_created"`
		Failed  int `json:"account_failed"`
		Errors  []struct {
			Name string `json:"name"`
		} `json:"errors,omitempty"`
	}](r.Context(), h.s.Upstream, http.MethodPost, "/admin/accounts/data", a.Tokens.Access, "", "", payload)
	return platform.Send(w, http.StatusOK, row, err)
}
