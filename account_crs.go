package main

import (
	"net/http"
	"net/url"
	"strings"
)

type crsAccountInput struct {
	BaseURL string   `json:"base_url"`
	User    string   `json:"username"`
	Pass    string   `json:"password"`
	Proxies *bool    `json:"sync_proxies,omitempty"`
	NewIDs  []string `json:"selected_account_ids,omitempty"`
}

type crsAccountCandidate struct {
	ID       string `json:"crs_account_id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
}

func (p *panel) syncCRSAccounts(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[crsAccountInput](r)
	if err != nil {
		return err
	}
	input.BaseURL, input.User = strings.TrimSpace(input.BaseURL), strings.TrimSpace(input.User)
	address, err := url.Parse(input.BaseURL)
	if err != nil || address.Hostname() == "" || address.User != nil || address.Fragment != "" || address.RawQuery != "" ||
		address.ForceQuery || (address.Scheme != "http" && address.Scheme != "https") || input.User == "" || input.Pass == "" || len(input.NewIDs) > 1000 {
		return fail(http.StatusBadRequest, "invalid_input")
	}
	for _, id := range input.NewIDs {
		if strings.TrimSpace(id) != id || id == "" || len(id) > 200 {
			return fail(http.StatusBadRequest, "invalid_input")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	const base = "/admin/accounts/sync/crs"
	if strings.HasSuffix(r.URL.Path, "/preview") {
		result, err := upstream[struct {
			New      []crsAccountCandidate `json:"new_accounts"`
			Existing []crsAccountCandidate `json:"existing_accounts"`
		}](r.Context(), p.upstream, http.MethodPost, base+"/preview", a.tokens.Access, "", "", input)
		return send(w, http.StatusOK, result, err)
	}
	result, err := upstream[struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
		Skipped int `json:"skipped"`
		Failed  int `json:"failed"`
		Items   []struct {
			ID     string `json:"crs_account_id"`
			Name   string `json:"name"`
			Action string `json:"action"`
		} `json:"items"`
	}](r.Context(), p.upstream, http.MethodPost, base, a.tokens.Access, "", "", input)
	return send(w, http.StatusOK, result, err)
}
