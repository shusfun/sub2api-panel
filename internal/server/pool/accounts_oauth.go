package pool

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func oauthEndpoints(plat, mode string) (startPath, exchangePath, accountType string, ok bool) {
	switch plat {
	case "openai":
		return "/admin/openai/generate-auth-url", "/admin/openai/exchange-code", "oauth", true
	case "anthropic":
		if mode == "setup-token" {
			return "/admin/accounts/generate-setup-token-url", "/admin/accounts/exchange-setup-token-code", "setup-token", true
		}
		return "/admin/accounts/generate-auth-url", "/admin/accounts/exchange-code", "oauth", true
	case "gemini":
		return "/admin/gemini/oauth/auth-url", "/admin/gemini/oauth/exchange-code", "oauth", true
	case "antigravity":
		return "/admin/antigravity/oauth/auth-url", "/admin/antigravity/oauth/exchange-code", "oauth", true
	case "grok":
		return "/admin/grok/oauth/auth-url", "/admin/grok/oauth/exchange-code", "oauth", true
	default:
		return "", "", "", false
	}
}

func (h *handler) oauthStart(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Platform    string `json:"platform"`
		Mode        string `json:"mode,omitempty"`
		ProxyID     *int64 `json:"proxy_id,omitempty"`
		ProjectID   string `json:"project_id,omitempty"`
		OAuthType   string `json:"oauth_type,omitempty"`
		TierID      string `json:"tier_id,omitempty"`
		RedirectURI string `json:"redirect_uri,omitempty"`
	}](r)
	if err != nil || input.ProxyID != nil && *input.ProxyID < 1 {
		return platform.Fail(400, "invalid_input")
	}
	startPath, _, _, ok := oauthEndpoints(input.Platform, input.Mode)
	if !ok {
		return platform.Fail(400, "invalid_input")
	}
	payload := map[string]any{}
	if input.ProxyID != nil {
		payload["proxy_id"] = *input.ProxyID
	}
	if input.ProjectID != "" {
		payload["project_id"] = input.ProjectID
	}
	if input.OAuthType != "" {
		payload["oauth_type"] = input.OAuthType
	}
	if input.TierID != "" {
		payload["tier_id"] = input.TierID
	}
	if input.RedirectURI != "" {
		payload["redirect_uri"] = input.RedirectURI
	}
	res, err := platform.Upstream[struct {
		AuthURL   string `json:"auth_url"`
		SessionID string `json:"session_id"`
		State     string `json:"state"`
	}](r.Context(), h.s.Upstream, "POST", startPath, a.Tokens.Access, "", "", payload)
	if err != nil {
		return err
	}
	if res.AuthURL == "" || res.SessionID == "" {
		return platform.Fail(502, "invalid_upstream")
	}
	platform.WriteJSON(w, 200, res)
	return nil
}

func (h *handler) oauthComplete(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Platform       string   `json:"platform"`
		Mode           string   `json:"mode,omitempty"`
		SessionID      string   `json:"session_id"`
		State          string   `json:"state,omitempty"`
		Code           string   `json:"code"`
		RedirectURI    string   `json:"redirect_uri,omitempty"`
		ProjectID      string   `json:"project_id,omitempty"`
		OAuthType      string   `json:"oauth_type,omitempty"`
		TierID         string   `json:"tier_id,omitempty"`
		AccountID      *int64   `json:"account_id,omitempty"`
		Name           string   `json:"name,omitempty"`
		Notes          *string  `json:"notes,omitempty"`
		ProxyID        *int64   `json:"proxy_id,omitempty"`
		GroupIDs       []int64  `json:"group_ids,omitempty"`
		Concurrency    *int     `json:"concurrency,omitempty"`
		Priority       *int     `json:"priority,omitempty"`
		RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
	}](r)
	if err != nil || strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.Code) == "" ||
		input.AccountID == nil && strings.TrimSpace(input.Name) == "" {
		return platform.Fail(400, "invalid_input")
	}
	_, exchangePath, accountType, ok := oauthEndpoints(input.Platform, input.Mode)
	if !ok {
		return platform.Fail(400, "invalid_input")
	}
	exchangeBody := map[string]any{
		"session_id": strings.TrimSpace(input.SessionID),
		"code":       strings.TrimSpace(input.Code),
	}
	if input.State != "" {
		exchangeBody["state"] = input.State
	}
	if input.ProxyID != nil && *input.ProxyID > 0 {
		exchangeBody["proxy_id"] = *input.ProxyID
	}
	if input.RedirectURI != "" {
		exchangeBody["redirect_uri"] = input.RedirectURI
	}
	if input.ProjectID != "" {
		exchangeBody["project_id"] = input.ProjectID
	}
	if input.OAuthType != "" {
		exchangeBody["oauth_type"] = input.OAuthType
	}
	if input.TierID != "" {
		exchangeBody["tier_id"] = input.TierID
	}
	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()
	tokenData, err := platform.Upstream[map[string]any](ctx, h.s.Upstream, "POST", exchangePath, a.Tokens.Access, "", "", exchangeBody)
	if err != nil {
		return err
	}
	if len(tokenData) == 0 {
		return platform.Fail(502, "invalid_upstream")
	}
	if input.AccountID != nil && *input.AccountID > 0 {
		raw, err := platform.Upstream[AccountDetailWire](ctx, h.s.Upstream, "POST", fmt.Sprintf("/admin/accounts/%d/apply-oauth-credentials", *input.AccountID), a.Tokens.Access, "", "", map[string]any{
			"type":        accountType,
			"credentials": tokenData,
		})
		if err != nil {
			return err
		}
		view, err := ProjectAdminAccount(raw)
		return platform.Send(w, 200, view, err)
	}
	createBody := map[string]any{
		"name":        strings.TrimSpace(input.Name),
		"platform":    input.Platform,
		"type":        accountType,
		"credentials": tokenData,
	}
	if input.Notes != nil {
		createBody["notes"] = *input.Notes
	}
	if input.ProxyID != nil && *input.ProxyID > 0 {
		createBody["proxy_id"] = *input.ProxyID
	}
	if len(input.GroupIDs) > 0 {
		createBody["group_ids"] = input.GroupIDs
	}
	if input.Concurrency != nil {
		createBody["concurrency"] = *input.Concurrency
	}
	if input.Priority != nil {
		createBody["priority"] = *input.Priority
	}
	if input.RateMultiplier != nil {
		createBody["rate_multiplier"] = *input.RateMultiplier
	}
	raw, err := platform.Upstream[AccountDetailWire](ctx, h.s.Upstream, "POST", "/admin/accounts", a.Tokens.Access, "", "", createBody)
	if err != nil {
		return err
	}
	view, err := ProjectAdminAccount(raw)
	return platform.Send(w, 201, view, err)
}

func (h *handler) importAccounts(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Kind           string   `json:"kind"`
		Name           string   `json:"name,omitempty"`
		Notes          *string  `json:"notes,omitempty"`
		Items          []string `json:"items"`
		ProxyID        *int64   `json:"proxy_id,omitempty"`
		GroupIDs       []int64  `json:"group_ids,omitempty"`
		Concurrency    *int     `json:"concurrency,omitempty"`
		Priority       *int     `json:"priority,omitempty"`
		RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
		UpdateExisting bool     `json:"update_existing,omitempty"`
	}](r)
	if err != nil || len(input.Items) == 0 || len(input.Items) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 80*time.Second)
	defer cancel()
	switch input.Kind {
	case "codex-session":
		body := map[string]any{"contents": input.Items, "update_existing": input.UpdateExisting}
		if input.Name != "" {
			body["name"] = input.Name
		}
		if input.Notes != nil {
			body["notes"] = *input.Notes
		}
		if input.ProxyID != nil && *input.ProxyID > 0 {
			body["proxy_id"] = *input.ProxyID
		}
		if len(input.GroupIDs) > 0 {
			body["group_ids"] = input.GroupIDs
		}
		if input.Concurrency != nil {
			body["concurrency"] = *input.Concurrency
		}
		if input.Priority != nil {
			body["priority"] = *input.Priority
		}
		if input.RateMultiplier != nil {
			body["rate_multiplier"] = *input.RateMultiplier
		}
		res, err := platform.Upstream[struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		}](ctx, h.s.Upstream, "POST", "/admin/accounts/import/codex-session", a.Tokens.Access, "", "", body)
		return platform.Send(w, 200, res, err)
	case "codex-pat":
		created, failed := 0, 0
		for i, pat := range input.Items {
			pat = strings.TrimSpace(pat)
			if pat == "" {
				continue
			}
			name := strings.TrimSpace(input.Name)
			if name == "" {
				name = fmt.Sprintf("Codex PAT #%d", i+1)
			} else if len(input.Items) > 1 {
				name = fmt.Sprintf("%s #%d", name, i+1)
			}
			body := map[string]any{"access_token": pat, "name": name}
			if input.ProxyID != nil && *input.ProxyID > 0 {
				body["proxy_id"] = *input.ProxyID
			}
			if len(input.GroupIDs) > 0 {
				body["group_ids"] = input.GroupIDs
			}
			if _, cErr := platform.Upstream[map[string]any](ctx, h.s.Upstream, "POST", "/admin/openai/create-from-codex-pat", a.Tokens.Access, "", "", body); cErr != nil {
				failed++
			} else {
				created++
			}
		}
		platform.WriteJSON(w, 200, struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		}{Created: created, Failed: failed})
		return nil
	case "grok-sso":
		body := map[string]any{"sso_tokens": input.Items}
		if input.Name != "" {
			body["name"] = input.Name
		}
		if input.ProxyID != nil && *input.ProxyID > 0 {
			body["proxy_id"] = *input.ProxyID
		}
		if len(input.GroupIDs) > 0 {
			body["group_ids"] = input.GroupIDs
		}
		res, err := platform.Upstream[struct {
			Created []any `json:"created"`
			Failed  []any `json:"failed"`
		}](ctx, h.s.Upstream, "POST", "/admin/grok/sso-to-oauth", a.Tokens.Access, "", "", body)
		if err != nil {
			return err
		}
		platform.WriteJSON(w, 200, struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
		}{Created: len(res.Created), Failed: len(res.Failed)})
		return nil
	default:
		return platform.Fail(400, "invalid_input")
	}
}

func (h *handler) oauthCookieAuth(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		SessionKey     string   `json:"session_key"`
		Mode           string   `json:"mode,omitempty"`
		Name           string   `json:"name"`
		Notes          *string  `json:"notes,omitempty"`
		ProxyID        *int64   `json:"proxy_id,omitempty"`
		GroupIDs       []int64  `json:"group_ids,omitempty"`
		Concurrency    *int     `json:"concurrency,omitempty"`
		Priority       *int     `json:"priority,omitempty"`
		RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
	}](r)
	if err != nil || strings.TrimSpace(input.SessionKey) == "" || strings.TrimSpace(input.Name) == "" {
		return platform.Fail(400, "invalid_input")
	}
	authBody := map[string]any{"session_key": strings.TrimSpace(input.SessionKey)}
	if input.ProxyID != nil && *input.ProxyID > 0 {
		authBody["proxy_id"] = *input.ProxyID
	}
	endpoint, accType := "/admin/accounts/cookie-auth", "oauth"
	if input.Mode == "setup-token" {
		endpoint, accType = "/admin/accounts/setup-token-cookie-auth", "setup-token"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()
	tokenData, err := platform.Upstream[map[string]any](ctx, h.s.Upstream, "POST", endpoint, a.Tokens.Access, "", "", authBody)
	if err != nil {
		return err
	}
	if len(tokenData) == 0 {
		return platform.Fail(502, "invalid_upstream")
	}
	createBody := map[string]any{
		"name":        strings.TrimSpace(input.Name),
		"platform":    "anthropic",
		"type":        accType,
		"credentials": tokenData,
	}
	if input.Notes != nil {
		createBody["notes"] = *input.Notes
	}
	if input.ProxyID != nil && *input.ProxyID > 0 {
		createBody["proxy_id"] = *input.ProxyID
	}
	if len(input.GroupIDs) > 0 {
		createBody["group_ids"] = input.GroupIDs
	}
	if input.Concurrency != nil {
		createBody["concurrency"] = *input.Concurrency
	}
	if input.Priority != nil {
		createBody["priority"] = *input.Priority
	}
	if input.RateMultiplier != nil {
		createBody["rate_multiplier"] = *input.RateMultiplier
	}
	raw, err := platform.Upstream[AccountDetailWire](ctx, h.s.Upstream, "POST", "/admin/accounts", a.Tokens.Access, "", "", createBody)
	if err != nil {
		return err
	}
	view, err := ProjectAdminAccount(raw)
	return platform.Send(w, 201, view, err)
}
