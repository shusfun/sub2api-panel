package users

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
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

func (h *handler) registerMCPRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/mcp/config", false, h.mcpConfig)
	h.s.Route(mux, "GET /api/mcp/grants", false, h.mcpGrants)
	h.s.Route(mux, "DELETE /api/mcp/grants/{id}", false, h.revokeMCPGrant)
	h.s.Route(mux, "GET /api/mcp/authorize", false, h.mcpConsent)
	h.s.Route(mux, "POST /api/mcp/authorize", false, h.mcpDecide)
}

func (h *handler) mcpConfig(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	result, err := platform.Upstream[struct {
		Resource string `json:"resource"`
	}](r.Context(), h.s.Upstream, http.MethodGet, "/user/mcp/config", a.Tokens.Access, "", "", nil)
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) mcpGrants(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	rows, err := platform.Upstream[[]mcpGrant](r.Context(), h.s.Upstream, http.MethodGet, "/user/mcp/grants", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	rows, err = platform.ProjectList(rows, func(row mcpGrant) (mcpGrant, error) {
		if !mcpGrantID.MatchString(row.ID) {
			return row, platform.Fail(http.StatusBadGateway, "invalid_upstream")
		}
		return row, nil
	})
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) revokeMCPGrant(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id := r.PathValue("id")
	if !mcpGrantID.MatchString(id) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	row, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodDelete, "/user/mcp/grants/"+id, a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) mcpConsent(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	raw, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(r.URL.RawQuery) > 8192 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	query := make(url.Values)
	for _, key := range []string{"response_type", "client_id", "redirect_uri", "scope", "state", "code_challenge", "code_challenge_method", "resource"} {
		if values := raw[key]; len(values) > 1 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		} else if len(values) == 1 {
			query.Set(key, values[0])
		}
	}
	row, err := platform.Upstream[struct {
		Client      string `json:"client_name"`
		Resource    string `json:"resource"`
		RedirectURI string `json:"redirect_uri"`
	}](r.Context(), h.s.Upstream, http.MethodGet, mcpAuthorizationPath+"?"+query.Encode(), a.Tokens.Access, "", "", nil)
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) mcpDecide(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
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
		return platform.FailField("scope", "请选择有效的授权范围")
	}
	if input.Approved && input.Scope == "admin" {
		if a.User.Role != "admin" {
			return platform.Fail(http.StatusForbidden, "not_allowed")
		}
		if err := h.s.StepUp(r, a, input.TOTP); err != nil {
			return err
		}
	}
	row, err := platform.Upstream[struct {
		Redirect string `json:"redirect_url"`
	}](r.Context(), h.s.Upstream, http.MethodPost, mcpAuthorizationPath, a.Tokens.Access, "", "", struct {
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
		return platform.Fail(http.StatusBadGateway, "invalid_upstream")
	}
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, row, nil)
}
