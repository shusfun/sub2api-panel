package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const mcpAuthorizationPath = "/user/mcp/authorize"

var mcpGrantID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type mcpAuthorization struct {
	ResponseType        string `json:"response_type"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Resource            string `json:"resource"`
}

type mcpGrant struct {
	ID       string     `json:"id"`
	Client   string     `json:"client_name"`
	Scope    string     `json:"scope"`
	ReadOnly bool       `json:"read_only"`
	Created  time.Time  `json:"created_at"`
	Expires  time.Time  `json:"expires_at"`
	LastUsed *time.Time `json:"last_used_at"`
	Revoked  *time.Time `json:"revoked_at"`
	Invalid  bool       `json:"invalidated"`
}

func (p *panel) mcpRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/mcp/config", false, p.mcpConfig)
	p.route(mux, "GET /api/mcp/grants", false, p.mcpGrants)
	p.route(mux, "DELETE /api/mcp/grants/{id}", false, p.revokeMCPGrant)
	p.route(mux, "GET /api/mcp/authorize", false, p.mcpConsent)
	p.route(mux, "POST /api/mcp/authorize", false, p.mcpDecide)
}

func (p *panel) mcpConfig(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	result, err := upstream[struct {
		Resource string `json:"resource"`
	}](r.Context(), p.upstream, http.MethodGet, "/user/mcp/config", a.tokens.Access, "", "", nil)
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, result, err)
}

func (p *panel) mcpGrants(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	rows, err := upstream[[]mcpGrant](r.Context(), p.upstream, http.MethodGet, "/user/mcp/grants", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	rows, err = projectList(rows, func(row mcpGrant) (mcpGrant, error) {
		if !mcpGrantID.MatchString(row.ID) {
			return row, fail(http.StatusBadGateway, "invalid_upstream")
		}
		return row, nil
	})
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) revokeMCPGrant(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id := r.PathValue("id")
	if !mcpGrantID.MatchString(id) {
		return fail(http.StatusBadRequest, "invalid_input")
	}
	row, err := upstream[struct{}](r.Context(), p.upstream, http.MethodDelete, "/user/mcp/grants/"+id, a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, row, err)
}

func (p *panel) mcpConsent(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	raw, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(r.URL.RawQuery) > 8192 {
		return fail(http.StatusBadRequest, "invalid_input")
	}
	query := make(url.Values)
	for _, key := range []string{"response_type", "client_id", "redirect_uri", "scope", "state", "code_challenge", "code_challenge_method", "resource"} {
		if values := raw[key]; len(values) > 1 {
			return fail(http.StatusBadRequest, "invalid_input")
		} else if len(values) == 1 {
			query.Set(key, values[0])
		}
	}
	row, err := upstream[struct {
		Client      string `json:"client_name"`
		Resource    string `json:"resource"`
		RedirectURI string `json:"redirect_uri"`
	}](r.Context(), p.upstream, http.MethodGet, mcpAuthorizationPath+"?"+query.Encode(), a.tokens.Access, "", "", nil)
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, row, err)
}

func (p *panel) mcpDecide(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Authorization mcpAuthorization `json:"authorization"`
		Approved      bool             `json:"approved"`
		Scope         string           `json:"scope"`
		ReadOnly      bool             `json:"read_only"`
		TOTP          string           `json:"totp,omitzero"`
	}](r)
	if err != nil {
		return err
	}
	if input.Scope != "user" && input.Scope != "admin" {
		return failField("scope", "请选择有效的授权范围")
	}
	if input.Approved && input.Scope == "admin" {
		if a.user.Role != "admin" {
			return fail(http.StatusForbidden, "not_allowed")
		}
		if err := p.stepUp(r, a, input.TOTP); err != nil {
			return err
		}
	}
	row, err := upstream[struct {
		Redirect string `json:"redirect_url"`
	}](r.Context(), p.upstream, http.MethodPost, mcpAuthorizationPath, a.tokens.Access, "", "", struct {
		Authorization mcpAuthorization `json:"authorization"`
		Approved      bool             `json:"approved"`
		Scope         string           `json:"scope"`
		ReadOnly      bool             `json:"read_only"`
	}{input.Authorization, input.Approved, input.Scope, input.ReadOnly})
	if err != nil {
		return err
	}
	redirect, redirectErr := url.Parse(row.Redirect)
	callback, callbackErr := url.Parse(input.Authorization.RedirectURI)
	if redirectErr != nil || callbackErr != nil || redirect.User != nil || redirect.Hostname() == "" ||
		(redirect.Scheme != "http" && redirect.Scheme != "https") || redirect.Scheme != callback.Scheme ||
		!strings.EqualFold(redirect.Host, callback.Host) || redirect.EscapedPath() != callback.EscapedPath() || redirect.Fragment != "" {
		return fail(http.StatusBadGateway, "invalid_upstream")
	}
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, row, nil)
}
