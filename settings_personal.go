package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// 个人设置：资料、密码、通行密钥、登录方式和通知邮箱。
// 全部以当前登录者的官方令牌访问上游，只投影页面用到的字段。
func (p *panel) personalSettingsRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/settings/profile", false, p.profile)
	p.route(mux, "PUT /api/settings/profile", false, p.updateProfile)
	p.route(mux, "PUT /api/settings/password", false, p.changePassword)
	p.route(mux, "POST /api/settings/notify-emails/send-code", false, p.notifyEmailAction("POST", "/send-code", false))
	p.route(mux, "POST /api/settings/notify-emails/verify", false, p.notifyEmailAction("POST", "/verify", true))
	p.route(mux, "PUT /api/settings/notify-emails/toggle", false, p.toggleNotifyEmail)
	p.route(mux, "DELETE /api/settings/notify-emails", false, p.notifyEmailAction("DELETE", "", false))
	p.route(mux, "GET /api/settings/passkeys", false, p.passkeys)
	p.route(mux, "POST /api/settings/passkeys/register/begin", false, p.beginPasskey)
	p.route(mux, "POST /api/settings/passkeys/register/finish", false, p.finishPasskey)
	p.route(mux, "PATCH /api/settings/passkeys/{id}", false, p.renamePasskey)
	p.route(mux, "DELETE /api/settings/passkeys/{id}", false, p.deletePasskey)
	p.route(mux, "POST /api/settings/identities/{provider}/bind", false, p.bindIdentity)
	p.route(mux, "DELETE /api/settings/identities/{provider}", false, p.unbindIdentity)
}

var identityOrder = []string{"linuxdo", "github", "google", "wechat", "dingtalk", "oidc"}
var identityProviders = map[string]string{
	"linuxdo": "LinuxDo", "github": "GitHub", "google": "Google", "wechat": "微信", "dingtalk": "钉钉", "oidc": "OIDC",
}

type notifyEmail struct {
	Email    string `json:"email"`
	Disabled bool   `json:"disabled"`
	Verified bool   `json:"verified"`
}

type profileWire struct {
	ID          int64         `json:"id"`
	Email       string        `json:"email"`
	Username    string        `json:"username"`
	Role        string        `json:"role"`
	Balance     *float64      `json:"balance"`
	Concurrency *int          `json:"concurrency"`
	RPM         int           `json:"rpm_limit"`
	Notify      bool          `json:"balance_notify_enabled"`
	Threshold   *float64      `json:"balance_notify_threshold"`
	Extra       []notifyEmail `json:"balance_notify_extra_emails"`
	Identities  map[string]struct {
		Bound   bool   `json:"bound"`
		Display string `json:"display_name"`
		Bind    bool   `json:"can_bind"`
		Unbind  bool   `json:"can_unbind"`
	} `json:"identities"`
}

type profileIdentity struct {
	Provider string `json:"provider"`
	Label    string `json:"label"`
	Bound    bool   `json:"bound"`
	Display  string `json:"display,omitempty"`
	Bind     bool   `json:"can_bind"`
	Unbind   bool   `json:"can_unbind"`
}

type profileView struct {
	Email       string            `json:"email"`
	Username    string            `json:"username"`
	Role        string            `json:"role"`
	Balance     float64           `json:"balance"`
	Concurrency int               `json:"concurrency"`
	RPM         int               `json:"rpm_limit"`
	NotifySite  bool              `json:"notify_available"`
	Notify      bool              `json:"notify_enabled"`
	Threshold   *float64          `json:"notify_threshold"`
	Emails      []notifyEmail     `json:"notify_emails"`
	Identities  []profileIdentity `json:"identities"`
}

func (p *panel) sendProfile(w http.ResponseWriter, r *http.Request, a sessionAuth, wire profileWire) error {
	if wire.ID != a.user.ID || wire.Role != a.user.Role || wire.Balance == nil || wire.Concurrency == nil || len(wire.Email) > 320 {
		return fail(502, "invalid_upstream")
	}
	site, err := upstream[struct {
		Notify bool `json:"balance_low_notify_enabled"`
	}](r.Context(), p.upstream, "GET", "/settings/public", "", "", "", nil)
	if err != nil {
		return err
	}
	view := profileView{
		Email: wire.Email, Username: wire.Username, Role: wire.Role, Balance: *wire.Balance, Concurrency: *wire.Concurrency,
		RPM: wire.RPM, NotifySite: site.Notify, Notify: wire.Notify, Threshold: wire.Threshold,
		Emails: append([]notifyEmail{}, wire.Extra...), Identities: []profileIdentity{},
	}
	for _, provider := range identityOrder {
		if state, ok := wire.Identities[provider]; ok && (state.Bound || state.Bind) {
			view.Identities = append(view.Identities, profileIdentity{provider, identityProviders[provider], state.Bound, state.Display, state.Bind, state.Unbind})
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, view)
	return nil
}

func (p *panel) profile(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	wire, err := upstream[profileWire](r.Context(), p.upstream, "GET", "/user/profile", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	return p.sendProfile(w, r, a, wire)
}

func (p *panel) updateProfile(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Username  *string  `json:"username,omitempty"`
		Notify    *bool    `json:"balance_notify_enabled,omitempty"`
		Threshold *float64 `json:"balance_notify_threshold,omitempty"`
	}](r)
	if err != nil || input.Username == nil && input.Notify == nil && input.Threshold == nil ||
		input.Username != nil && len(strings.TrimSpace(*input.Username)) > 100 ||
		input.Threshold != nil && (*input.Threshold < 0 || *input.Threshold > 1e9) {
		return fail(400, "invalid_input")
	}
	if _, err := upstream[json.RawMessage](r.Context(), p.upstream, "PUT", "/user", a.tokens.Access, "", "", input); err != nil {
		return err
	}
	return p.profile(w, r, a)
}

func (p *panel) changePassword(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}](r)
	if err != nil || input.Old == "" || len(input.Old) > 256 || len(input.New) < 6 || len(input.New) > 256 || input.Old == input.New {
		return fail(400, "invalid_input")
	}
	if _, err := upstream[json.RawMessage](r.Context(), p.upstream, "PUT", "/user/password", a.tokens.Access, "", "", input); err != nil {
		return err
	}
	// 官方改密后会吊销全部令牌：本地会话一并结束，让用户用新密码重新登录。
	a.local.mu.Lock()
	a.local.revoked = true
	a.local.encrypted = nil
	a.local.mu.Unlock()
	setSessionCookie(w, "", -1)
	writeJSON(w, 200, struct{}{})
	return nil
}

func validEmail(value string) bool {
	at := strings.LastIndex(value, "@")
	return at > 0 && at < len(value)-3 && len(value) <= 320 && !strings.ContainsAny(value, " \t\r\n")
}

func (p *panel) notifyEmailAction(method, suffix string, withCode bool) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		input, err := decodeBody[struct {
			Email string `json:"email"`
			Code  string `json:"code,omitempty"`
		}](r)
		if err != nil || !validEmail(input.Email) || withCode != (input.Code != "") || withCode && !otpPattern.MatchString(input.Code) {
			return fail(400, "invalid_input")
		}
		_, err = upstream[json.RawMessage](r.Context(), p.upstream, method, "/user/notify-email"+suffix, a.tokens.Access, "", "", input)
		return send(w, 200, struct{}{}, err)
	}
}

func (p *panel) toggleNotifyEmail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Email    string `json:"email"`
		Disabled *bool  `json:"disabled"`
	}](r)
	if err != nil || !validEmail(input.Email) || input.Disabled == nil {
		return fail(400, "invalid_input")
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "PUT", "/user/notify-email/toggle", a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

type passkeyView struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Created  string  `json:"created_at"`
	LastUsed *string `json:"last_used_at"`
}

func (p *panel) passkeys(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	site, err := upstream[struct {
		Enabled bool `json:"passkey_enabled"`
	}](r.Context(), p.upstream, "GET", "/settings/public", "", "", "", nil)
	if err != nil {
		return err
	}
	result := struct {
		Enabled bool          `json:"enabled"`
		Items   []passkeyView `json:"items"`
	}{site.Enabled, []passkeyView{}}
	if site.Enabled {
		rows, err := upstream[[]passkeyView](r.Context(), p.upstream, "GET", "/user/passkeys", a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.ID < 1 || len(row.Name) > 200 {
				return fail(502, "invalid_upstream")
			}
			result.Items = append(result.Items, row)
		}
	}
	writeJSON(w, 200, result)
	return nil
}

// 通行密钥的创建参数与凭据是 WebAuthn 标准结构，由浏览器生成和消费；面板只限制大小并原样转交。
func (p *panel) beginPasskey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Password string `json:"password"`
	}](r)
	if err != nil || input.Password == "" || len(input.Password) > 256 {
		return fail(400, "invalid_input")
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := upstream[struct {
		Token   string          `json:"session_token"`
		Options json.RawMessage `json:"options"`
	}](r.Context(), p.upstream, "POST", "/user/passkeys/register/begin", a.tokens.Access, "", "", input)
	if err == nil && (result.Token == "" || len(result.Token) > 2048 || len(result.Options) < 2 || len(result.Options) > 32<<10) {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, result, err)
}

func (p *panel) finishPasskey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Token      string          `json:"session_token"`
		Name       string          `json:"name,omitempty"`
		Credential json.RawMessage `json:"credential"`
	}](r)
	if err != nil || input.Token == "" || len(input.Token) > 2048 || len(input.Name) > 100 || len(input.Credential) < 2 || len(input.Credential) > 48<<10 {
		return fail(400, "invalid_input")
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "POST", "/user/passkeys/register/finish", a.tokens.Access, "", "", input)
	return send(w, 201, struct{}{}, err)
}

func (p *panel) renamePasskey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Name string `json:"name"`
	}](r)
	if err != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 {
		return fail(400, "invalid_input")
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "PATCH", fmt.Sprintf("/user/passkeys/%d", id), a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) deletePasskey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Password string `json:"password"`
	}](r)
	if err != nil || input.Password == "" || len(input.Password) > 256 {
		return fail(400, "invalid_input")
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/user/passkeys/%d", id), a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

// 第三方绑定：向官方要授权地址，完成后回到面板的设置页。授权回调由官方站处理。
func (p *panel) bindIdentity(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	provider := r.PathValue("provider")
	if identityProviders[provider] == "" {
		return fail(400, "invalid_input")
	}
	result, err := upstream[struct {
		URL string `json:"authorize_url"`
	}](r.Context(), p.upstream, "POST", "/user/auth-identities/bind/start", a.tokens.Access, "", "", struct {
		Provider string `json:"provider"`
		Redirect string `json:"redirect_to"`
	}{provider, p.origin + "/#settings?section=identities"})
	if err != nil {
		return err
	}
	target := result.URL
	if strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//") {
		target = p.official + target
	}
	if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, p.official+"/") {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 200, struct {
		URL string `json:"url"`
	}{target})
	return nil
}

func (p *panel) unbindIdentity(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	provider := r.PathValue("provider")
	if identityProviders[provider] == "" {
		return fail(400, "invalid_input")
	}
	_, err := upstream[json.RawMessage](r.Context(), p.upstream, "DELETE", "/user/account-bindings/"+provider, a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}
