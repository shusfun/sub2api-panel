package pool

import (
	"fmt"
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const (
	passthroughPath = "/admin/error-passthrough-rules"
	tlsProfilePath  = "/admin/tls-fingerprint-profiles"
)

// 错误透传规则的响应白名单：只列出官方规则里面板界面用到的字段。
type passthroughRule struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Priority        int      `json:"priority"`
	ErrorCodes      []int    `json:"error_codes"`
	Keywords        []string `json:"keywords"`
	MatchMode       string   `json:"match_mode"`
	Platforms       []string `json:"platforms"`
	PassthroughCode bool     `json:"passthrough_code"`
	ResponseCode    *int     `json:"response_code"`
	PassthroughBody bool     `json:"passthrough_body"`
	CustomMessage   *string  `json:"custom_message"`
	SkipMonitoring  bool     `json:"skip_monitoring"`
	Description     *string  `json:"description"`
	UpdatedAt       string   `json:"updated_at"`
}

type passthroughInput struct {
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Priority        int      `json:"priority"`
	ErrorCodes      []int    `json:"error_codes"`
	Keywords        []string `json:"keywords"`
	MatchMode       string   `json:"match_mode"`
	Platforms       []string `json:"platforms"`
	PassthroughCode bool     `json:"passthrough_code"`
	ResponseCode    *int     `json:"response_code"`
	PassthroughBody bool     `json:"passthrough_body"`
	CustomMessage   *string  `json:"custom_message"`
	SkipMonitoring  bool     `json:"skip_monitoring"`
	Description     *string  `json:"description"`
}

// TLS 指纹档案的响应白名单。
type tlsProfile struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Description         *string  `json:"description"`
	EnableGREASE        bool     `json:"enable_grease"`
	CipherSuites        []uint16 `json:"cipher_suites"`
	Curves              []uint16 `json:"curves"`
	PointFormats        []uint16 `json:"point_formats"`
	SignatureAlgorithms []uint16 `json:"signature_algorithms"`
	ALPNProtocols       []string `json:"alpn_protocols"`
	SupportedVersions   []uint16 `json:"supported_versions"`
	KeyShareGroups      []uint16 `json:"key_share_groups"`
	PSKModes            []uint16 `json:"psk_modes"`
	Extensions          []uint16 `json:"extensions"`
	UpdatedAt           string   `json:"updated_at"`
}

type tlsProfileInput struct {
	Name                string   `json:"name"`
	Description         *string  `json:"description"`
	EnableGREASE        bool     `json:"enable_grease"`
	CipherSuites        []uint16 `json:"cipher_suites"`
	Curves              []uint16 `json:"curves"`
	PointFormats        []uint16 `json:"point_formats"`
	SignatureAlgorithms []uint16 `json:"signature_algorithms"`
	ALPNProtocols       []string `json:"alpn_protocols"`
	SupportedVersions   []uint16 `json:"supported_versions"`
	KeyShareGroups      []uint16 `json:"key_share_groups"`
	PSKModes            []uint16 `json:"psk_modes"`
	Extensions          []uint16 `json:"extensions"`
}

func (h *handler) registerRuleRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/error-passthrough-rules", true, h.passthroughList)
	h.s.Route(mux, "GET /api/admin/error-passthrough-rules/{id}", true, h.passthroughOne)
	h.s.Route(mux, "POST /api/admin/error-passthrough-rules", true, h.passthroughSave)
	h.s.Route(mux, "PUT /api/admin/error-passthrough-rules/{id}", true, h.passthroughSave)
	h.s.Route(mux, "DELETE /api/admin/error-passthrough-rules/{id}", true, h.ruleDelete(passthroughPath))
	h.s.Route(mux, "GET /api/admin/tls-fingerprint-profiles", true, h.tlsList)
	h.s.Route(mux, "GET /api/admin/tls-fingerprint-profiles/{id}", true, h.tlsOne)
	h.s.Route(mux, "POST /api/admin/tls-fingerprint-profiles", true, h.tlsSave)
	h.s.Route(mux, "PUT /api/admin/tls-fingerprint-profiles/{id}", true, h.tlsSave)
	h.s.Route(mux, "DELETE /api/admin/tls-fingerprint-profiles/{id}", true, h.ruleDelete(tlsProfilePath))
	h.s.Route(mux, "POST /api/admin/accounts/{id}/set-privacy", true, h.accountPrivacy)
	h.s.Route(mux, "POST /api/admin/accounts/{id}/shadow", true, h.accountShadow)
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

func projectRule(r passthroughRule) (passthroughRule, error) {
	r.ErrorCodes, r.Keywords, r.Platforms = nonNil(r.ErrorCodes), nonNil(r.Keywords), nonNil(r.Platforms)
	return r, platform.RequiredID(r.ID)
}

func projectTLS(p tlsProfile) (tlsProfile, error) {
	p.CipherSuites, p.Curves, p.PointFormats = nonNil(p.CipherSuites), nonNil(p.Curves), nonNil(p.PointFormats)
	p.SignatureAlgorithms, p.ALPNProtocols, p.SupportedVersions = nonNil(p.SignatureAlgorithms), nonNil(p.ALPNProtocols), nonNil(p.SupportedVersions)
	p.KeyShareGroups, p.PSKModes, p.Extensions = nonNil(p.KeyShareGroups), nonNil(p.PSKModes), nonNil(p.Extensions)
	return p, platform.RequiredID(p.ID)
}

func (h *handler) passthroughList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	rows, err := platform.Upstream[[]passthroughRule](r.Context(), h.s.Upstream, http.MethodGet, passthroughPath, a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	items, err := platform.ProjectList(rows, projectRule)
	return platform.Send(w, http.StatusOK, platform.PageResult[passthroughRule]{Items: items, Pages: 1}, err)
}

func (h *handler) passthroughOne(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[passthroughRule](r.Context(), h.s.Upstream, http.MethodGet, fmt.Sprintf("%s/%d", passthroughPath, id), a.Tokens.Access, "", "", nil)
	if err == nil {
		row, err = projectRule(row)
	}
	return platform.Send(w, http.StatusOK, row, err)
}

func validRuleInput(in passthroughInput) bool {
	if n := len(strings.TrimSpace(in.Name)); n == 0 || n > 100 {
		return false
	}
	if in.MatchMode != "any" && in.MatchMode != "all" || in.Priority < 0 || in.Priority > 100000 ||
		len(in.ErrorCodes) > 50 || len(in.Keywords) > 50 || len(in.Platforms) > 20 {
		return false
	}
	for _, code := range in.ErrorCodes {
		if code < 100 || code > 599 {
			return false
		}
	}
	for _, word := range in.Keywords {
		if strings.TrimSpace(word) == "" || len(word) > 200 {
			return false
		}
	}
	if in.ResponseCode != nil && (*in.ResponseCode < 100 || *in.ResponseCode > 599) {
		return false
	}
	if in.CustomMessage != nil && len(*in.CustomMessage) > 2000 || in.Description != nil && len(*in.Description) > 500 {
		return false
	}
	// 至少要有一个匹配条件，否则规则会命中全部错误。
	return len(in.ErrorCodes) > 0 || len(in.Keywords) > 0
}

func (h *handler) passthroughSave(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[passthroughInput](r)
	if err != nil {
		return err
	}
	input.ErrorCodes, input.Keywords, input.Platforms = nonNil(input.ErrorCodes), nonNil(input.Keywords), nonNil(input.Platforms)
	if !validRuleInput(input) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	path, status := passthroughPath, http.StatusCreated
	if r.Method == http.MethodPut {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		path, status = fmt.Sprintf("%s/%d", path, id), http.StatusOK
	}
	row, err := platform.Upstream[passthroughRule](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", input)
	if err == nil {
		row, err = projectRule(row)
	}
	return platform.Send(w, status, row, err)
}

func (h *handler) ruleDelete(base string) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		row, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodDelete, fmt.Sprintf("%s/%d", base, id), a.Tokens.Access, "", "", nil)
		return platform.Send(w, http.StatusOK, row, err)
	}
}

func (h *handler) tlsList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	rows, err := platform.Upstream[[]tlsProfile](r.Context(), h.s.Upstream, http.MethodGet, tlsProfilePath, a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	items, err := platform.ProjectList(rows, projectTLS)
	return platform.Send(w, http.StatusOK, platform.PageResult[tlsProfile]{Items: items, Pages: 1}, err)
}

func (h *handler) tlsOne(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[tlsProfile](r.Context(), h.s.Upstream, http.MethodGet, fmt.Sprintf("%s/%d", tlsProfilePath, id), a.Tokens.Access, "", "", nil)
	if err == nil {
		row, err = projectTLS(row)
	}
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) tlsSave(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[tlsProfileInput](r)
	if err != nil {
		return err
	}
	if n := len(strings.TrimSpace(input.Name)); n == 0 || n > 100 || input.Description != nil && len(*input.Description) > 500 ||
		len(input.CipherSuites) > 100 || len(input.Curves) > 50 || len(input.Extensions) > 100 || len(input.ALPNProtocols) > 10 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	input.CipherSuites, input.Curves, input.PointFormats = nonNil(input.CipherSuites), nonNil(input.Curves), nonNil(input.PointFormats)
	input.SignatureAlgorithms, input.ALPNProtocols, input.SupportedVersions = nonNil(input.SignatureAlgorithms), nonNil(input.ALPNProtocols), nonNil(input.SupportedVersions)
	input.KeyShareGroups, input.PSKModes, input.Extensions = nonNil(input.KeyShareGroups), nonNil(input.PSKModes), nonNil(input.Extensions)
	path, status := tlsProfilePath, http.StatusCreated
	if r.Method == http.MethodPut {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		path, status = fmt.Sprintf("%s/%d", path, id), http.StatusOK
	}
	row, err := platform.Upstream[tlsProfile](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", input)
	if err == nil {
		row, err = projectTLS(row)
	}
	return platform.Send(w, status, row, err)
}

// 账号隐私设置：只回传最终的 privacy_mode，不下发账号凭证和其余 extra。
func (h *handler) accountPrivacy(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[struct {
		Extra map[string]any `json:"extra"`
	}](r.Context(), h.s.Upstream, http.MethodPost, fmt.Sprintf("/admin/accounts/%d/set-privacy", id), a.Tokens.Access, "", "", struct{}{})
	mode, _ := raw.Extra["privacy_mode"].(string)
	return platform.Send(w, http.StatusOK, struct {
		Mode string `json:"privacy_mode"`
	}{mode}, err)
}

// Spark 影子账号：只回传新账号的基本标识。
func (h *handler) accountShadow(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Name        string  `json:"name"`
		Priority    int     `json:"priority"`
		Concurrency int     `json:"concurrency"`
		GroupIDs    []int64 `json:"group_ids"`
	}](r)
	if err != nil {
		return err
	}
	if len(input.Name) > 100 || input.Priority < 0 || input.Priority > 100000 || input.Concurrency < 0 || input.Concurrency > 100000 || len(input.GroupIDs) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	for _, gid := range input.GroupIDs {
		if gid < 1 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
	}
	input.GroupIDs = nonNil(input.GroupIDs)
	row, err := platform.Upstream[struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Platform string `json:"platform"`
		Type     string `json:"type"`
	}](r.Context(), h.s.Upstream, http.MethodPost, fmt.Sprintf("/admin/accounts/%d/shadow", id), a.Tokens.Access, "", "", input)
	if err == nil {
		err = platform.RequiredID(row.ID)
	}
	return platform.Send(w, http.StatusCreated, row, err)
}
