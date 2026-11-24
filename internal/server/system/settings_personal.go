package system

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

// 个人设置：资料、密码、通行密钥、登录方式和通知邮箱。
// 全部以当前登录者的官方令牌访问上游，只投影页面用到的字段。
func (h *handler) personalSettingsRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/settings/profile", false, h.profile)
	h.s.Route(mux, "PUT /api/settings/profile", false, h.updateProfile)
	h.s.Route(mux, "PUT /api/settings/password", false, h.changePassword)
	h.s.Route(mux, "POST /api/settings/notify-emails/send-code", false, h.notifyEmailAction("POST", "/send-code", false))
	h.s.Route(mux, "POST /api/settings/notify-emails/verify", false, h.notifyEmailAction("POST", "/verify", true))
	h.s.Route(mux, "PUT /api/settings/notify-emails/toggle", false, h.toggleNotifyEmail)
	h.s.Route(mux, "DELETE /api/settings/notify-emails", false, h.notifyEmailAction("DELETE", "", false))
	h.s.Route(mux, "GET /api/settings/passkeys", false, h.passkeys)
	h.s.Route(mux, "POST /api/settings/passkeys/register/begin", false, h.beginPasskey)
	h.s.Route(mux, "POST /api/settings/passkeys/register/finish", false, h.finishPasskey)
	h.s.Route(mux, "PATCH /api/settings/passkeys/{id}", false, h.renamePasskey)
	h.s.Route(mux, "DELETE /api/settings/passkeys/{id}", false, h.deletePasskey)
	h.s.Route(mux, "POST /api/settings/identities/{provider}/bind", false, h.bindIdentity)
	h.s.Route(mux, "DELETE /api/settings/identities/{provider}", false, h.unbindIdentity)
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

func (h *handler) sendProfile(w http.ResponseWriter, r *http.Request, a core.SessionAuth, wire profileWire) error {
	if wire.ID != a.User.ID || wire.Role != a.User.Role || wire.Balance == nil || wire.Concurrency == nil || len(wire.Email) > 320 {
		return platform.Fail(502, "invalid_upstream")
	}
	site, err := platform.Upstream[struct {
		Notify bool `json:"balance_low_notify_enabled"`
	}](r.Context(), h.s.Upstream, "GET", "/settings/public", "", "", "", nil)
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
	platform.WriteJSON(w, 200, view)
	return nil
}

func (h *handler) profile(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	wire, err := platform.Upstream[profileWire](r.Context(), h.s.Upstream, "GET", "/user/profile", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	return h.sendProfile(w, r, a, wire)
}

func (h *handler) updateProfile(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Username  *string  `json:"username,omitempty"`
		Notify    *bool    `json:"balance_notify_enabled,omitempty"`
		Threshold *float64 `json:"balance_notify_threshold,omitempty"`
	}](r)
	if err != nil || input.Username == nil && input.Notify == nil && input.Threshold == nil ||
		input.Username != nil && len(strings.TrimSpace(*input.Username)) > 100 ||
		input.Threshold != nil && (*input.Threshold < 0 || *input.Threshold > 1e9) {
		return platform.Fail(400, "invalid_input")
	}
	if _, err := platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "PUT", "/user", a.Tokens.Access, "", "", input); err != nil {
		return err
	}
	return h.profile(w, r, a)
}

func (h *handler) changePassword(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}](r)
	if err != nil || input.Old == "" || len(input.Old) > 256 || len(input.New) < 6 || len(input.New) > 256 || input.Old == input.New {
		return platform.Fail(400, "invalid_input")
	}
	if _, err := platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "PUT", "/user/password", a.Tokens.Access, "", "", input); err != nil {
		return err
	}
	// 官方改密后会吊销全部令牌：本地会话一并结束，让用户用新密码重新登录。
	if h.s.Sessions != nil && a.Local != nil {
		h.s.Sessions.Revoke(a.Local)
	}
	platform.SetSessionCookie(w, "", -1)
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}

func (h *handler) notifyEmailAction(method, suffix string, withCode bool) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		input, err := platform.DecodeBody[struct {
			Email string `json:"email"`
			Code  string `json:"code,omitempty"`
		}](r)
		if err != nil || !platform.ValidEmail(input.Email) || withCode != (input.Code != "") || withCode && !platform.OTPPattern.MatchString(input.Code) {
			return platform.Fail(400, "invalid_input")
		}
		_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, method, "/user/notify-email"+suffix, a.Tokens.Access, "", "", input)
		return platform.Send(w, 200, struct{}{}, err)
	}
}

func (h *handler) toggleNotifyEmail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Email    string `json:"email"`
		Disabled *bool  `json:"disabled"`
	}](r)
	if err != nil || !platform.ValidEmail(input.Email) || input.Disabled == nil {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "PUT", "/user/notify-email/toggle", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

type passkeyView struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Created  string  `json:"created_at"`
	LastUsed *string `json:"last_used_at"`
}

func (h *handler) passkeys(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	site, err := platform.Upstream[struct {
		Enabled bool `json:"passkey_enabled"`
	}](r.Context(), h.s.Upstream, "GET", "/settings/public", "", "", "", nil)
	if err != nil {
		return err
	}
	result := struct {
		Enabled bool          `json:"enabled"`
		Items   []passkeyView `json:"items"`
	}{site.Enabled, []passkeyView{}}
	if site.Enabled {
		rows, err := platform.Upstream[[]passkeyView](r.Context(), h.s.Upstream, "GET", "/user/passkeys", a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.ID < 1 || len(row.Name) > 200 {
				return platform.Fail(502, "invalid_upstream")
			}
			result.Items = append(result.Items, row)
		}
	}
	platform.WriteJSON(w, 200, result)
	return nil
}

// 通行密钥的创建参数与凭据是 WebAuthn 标准结构，由浏览器生成和消费；面板只限制大小并原样转交。
func (h *handler) beginPasskey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Password string `json:"password"`
	}](r)
	if err != nil || input.Password == "" || len(input.Password) > 256 {
		return platform.Fail(400, "invalid_input")
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := platform.Upstream[struct {
		Token   string          `json:"session_token"`
		Options json.RawMessage `json:"options"`
	}](r.Context(), h.s.Upstream, "POST", "/user/passkeys/register/begin", a.Tokens.Access, "", "", input)
	if err == nil && (result.Token == "" || len(result.Token) > 2048 || len(result.Options) < 2 || len(result.Options) > 32<<10) {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, result, err)
}

func (h *handler) finishPasskey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Token      string          `json:"session_token"`
		Name       string          `json:"name,omitempty"`
		Credential json.RawMessage `json:"credential"`
	}](r)
	if err != nil || input.Token == "" || len(input.Token) > 2048 || len(input.Name) > 100 || len(input.Credential) < 2 || len(input.Credential) > 48<<10 {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "POST", "/user/passkeys/register/finish", a.Tokens.Access, "", "", input)
	return platform.Send(w, 201, struct{}{}, err)
}

func (h *handler) renamePasskey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Name string `json:"name"`
	}](r)
	if err != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "PATCH", fmt.Sprintf("/user/passkeys/%d", id), a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) deletePasskey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Password string `json:"password"`
	}](r)
	if err != nil || input.Password == "" || len(input.Password) > 256 {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/user/passkeys/%d", id), a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

// 第三方绑定：向官方要授权地址，完成后回到面板的设置页。授权回调由官方站处理。
func (h *handler) bindIdentity(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	provider := r.PathValue("provider")
	if identityProviders[provider] == "" {
		return platform.Fail(400, "invalid_input")
	}
	result, err := platform.Upstream[struct {
		URL string `json:"authorize_url"`
	}](r.Context(), h.s.Upstream, "POST", "/user/auth-identities/bind/start", a.Tokens.Access, "", "", struct {
		Provider string `json:"provider"`
		Redirect string `json:"redirect_to"`
	}{provider, h.s.Origin + "/#settings?section=identities"})
	if err != nil {
		return err
	}
	target := result.URL
	if strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//") {
		target = h.s.Official + target
	}
	if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, h.s.Official+"/") {
		return platform.Fail(502, "invalid_upstream")
	}
	platform.WriteJSON(w, 200, struct {
		URL string `json:"url"`
	}{target})
	return nil
}

func (h *handler) unbindIdentity(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	provider := r.PathValue("provider")
	if identityProviders[provider] == "" {
		return platform.Fail(400, "invalid_input")
	}
	_, err := platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "DELETE", "/user/account-bindings/"+provider, a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}
