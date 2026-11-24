package system

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

var buildVersion atomic.Value

func init() {
	buildVersion.Store("dev")
}

// SetBuildVersion configures the panel version reported by system tools.
func SetBuildVersion(v string) {
	if v != "" {
		buildVersion.Store(v)
	}
}

func currentBuildVersion() string {
	if v, ok := buildVersion.Load().(string); ok && v != "" {
		return v
	}
	return "dev"
}

// 站点设置里不属于「字段读写」的操作：邮件测试与模板、管理密钥状态、面板连接。
func (h *handler) adminSettingToolRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "POST /api/admin/settings-tools/mail/test-connection", true, h.testMailConnection)
	h.s.Route(mux, "POST /api/admin/settings-tools/mail/send-test", true, h.sendTestMail)
	h.s.Route(mux, "GET /api/admin/settings-tools/mail/templates", true, h.mailTemplates)
	h.s.Route(mux, "GET /api/admin/settings-tools/mail/templates/{event}/{locale}", true, h.mailTemplate)
	h.s.Route(mux, "PUT /api/admin/settings-tools/mail/templates/{event}/{locale}", true, h.saveMailTemplate)
	h.s.Route(mux, "POST /api/admin/settings-tools/mail/templates/{event}/{locale}/preview", true, h.previewMailTemplate)
	h.s.Route(mux, "POST /api/admin/settings-tools/mail/templates/{event}/{locale}/restore", true, h.restoreMailTemplate)

	h.s.Route(mux, "GET /api/admin/settings-tools/system", true, h.systemVersions)
	h.s.Route(mux, "POST /api/admin/settings-tools/system/{action}", true, h.officialSystemAction)
	h.s.Route(mux, "GET /api/admin/settings-tools/admin-key", true, h.adminKeyStatus)
	h.s.Route(mux, "POST /api/admin/settings-tools/admin-key/rotate", true, h.changeLegacyAdminKey)
	h.s.Route(mux, "DELETE /api/admin/settings-tools/admin-key", true, h.changeLegacyAdminKey)
	h.s.Route(mux, "GET /api/admin/settings-tools/access-keys", true, h.scopedAdminKeys)
	h.s.Route(mux, "POST /api/admin/settings-tools/access-keys", true, h.scopedAdminKeys)
	h.s.Route(mux, "PUT /api/admin/settings-tools/access-keys/{id}", true, h.scopedAdminKeys)
	h.s.Route(mux, "DELETE /api/admin/settings-tools/access-keys/{id}", true, h.scopedAdminKeys)
	h.s.Route(mux, "POST /api/admin/settings-tools/access-keys/{id}/{action}", true, h.scopedAdminKeys)
	h.s.Route(mux, "DELETE /api/admin/settings-tools/access-keys/{id}/{action}", true, h.scopedAdminKeys)
	h.s.Route(mux, "GET /api/admin/settings-tools/connection", true, h.connectionInfo)
}

// 未填的发信参数由官方用已保存的配置补齐，所以这里只转发管理员在表单里临时改过的值。
type mailProbe struct {
	Email    string `json:"email,omitempty"`
	Host     string `json:"smtp_host,omitempty"`
	Port     int    `json:"smtp_port,omitempty"`
	Username string `json:"smtp_username,omitempty"`
	Password string `json:"smtp_password,omitempty"`
	From     string `json:"smtp_from_email,omitempty"`
	FromName string `json:"smtp_from_name,omitempty"`
	TLS      *bool  `json:"smtp_use_tls,omitempty"`
}

func readMailProbe(r *http.Request, needEmail bool) (mailProbe, error) {
	input, err := platform.DecodeBody[mailProbe](r)
	if err != nil || len(input.Host) > 255 || input.Port < 0 || input.Port > 65535 || len(input.Username) > 255 ||
		len(input.Password) > 1024 || len(input.From) > 320 || len(input.FromName) > 200 ||
		needEmail != (input.Email != "") || needEmail && !platform.ValidEmail(input.Email) {
		return input, platform.Fail(400, "invalid_input")
	}
	return input, nil
}

func (h *handler) testMailConnection(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := readMailProbe(r, false)
	if err != nil {
		return err
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "POST", "/admin/settings/test-smtp", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) sendTestMail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := readMailProbe(r, true)
	if err != nil {
		return err
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "POST", "/admin/settings/send-test-email", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

var templateEventPattern = regexp.MustCompile(`^[a-z0-9_.]{1,80}$`)
var templateLocalePattern = regexp.MustCompile(`^[a-zA-Z-]{2,10}$`)

func templatePath(r *http.Request) (string, error) {
	event, locale := r.PathValue("event"), r.PathValue("locale")
	if !templateEventPattern.MatchString(event) || !templateLocalePattern.MatchString(locale) {
		return "", platform.Fail(400, "invalid_input")
	}
	return "/admin/settings/email-templates/" + url.PathEscape(event) + "/" + url.PathEscape(locale), nil
}

func (h *handler) mailTemplates(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	data, err := platform.Upstream[struct {
		Events []struct {
			Value    string `json:"value"`
			Label    string `json:"label"`
			Category string `json:"category"`
		} `json:"events"`
		Locales   []string `json:"locales"`
		Templates []struct {
			Event   string `json:"event"`
			Locale  string `json:"locale"`
			Subject string `json:"subject"`
		} `json:"templates"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/settings/email-templates", a.Tokens.Access, "", "", nil)
	if err == nil && (len(data.Events) > 200 || len(data.Locales) > 20 || len(data.Templates) > 4000) {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, data, err)
}

type mailTemplateView struct {
	Subject      string   `json:"subject"`
	HTML         string   `json:"html"`
	Placeholders []string `json:"placeholders"`
}

func (h *handler) mailTemplate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path, err := templatePath(r)
	if err != nil {
		return err
	}
	data, err := platform.Upstream[mailTemplateView](r.Context(), h.s.Upstream, "GET", path, a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, data, err)
}

type mailTemplateInput struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

func readMailTemplate(r *http.Request) (mailTemplateInput, error) {
	input, err := platform.DecodeBody[mailTemplateInput](r)
	if err != nil || strings.TrimSpace(input.Subject) == "" || len(input.Subject) > 500 || input.HTML == "" || len(input.HTML) > settingLongLimit {
		return input, platform.Fail(400, "invalid_input")
	}
	return input, nil
}

func (h *handler) saveMailTemplate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path, err := templatePath(r)
	if err != nil {
		return err
	}
	input, err := readMailTemplate(r)
	if err != nil {
		return err
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "PUT", path, a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) previewMailTemplate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if _, err := templatePath(r); err != nil {
		return err
	}
	input, err := readMailTemplate(r)
	if err != nil {
		return err
	}
	data, err := platform.Upstream[mailTemplateInput](r.Context(), h.s.Upstream, "POST", "/admin/settings/email-template-preview", a.Tokens.Access, "", "", struct {
		Event   string `json:"event"`
		Locale  string `json:"locale"`
		Subject string `json:"subject"`
		HTML    string `json:"html"`
	}{r.PathValue("event"), r.PathValue("locale"), input.Subject, input.HTML})
	return platform.Send(w, 200, data, err)
}

func (h *handler) restoreMailTemplate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path, err := templatePath(r)
	if err != nil {
		return err
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "POST", path+"/restore-official", a.Tokens.Access, "", "", struct{}{})
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) adminKeyStatus(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	legacy, err := platform.Upstream[struct {
		Exists bool   `json:"exists"`
		Masked string `json:"masked_key"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/settings/admin-api-key", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, 200, struct {
		Legacy bool   `json:"legacy_exists"`
		Masked string `json:"legacy_masked"`
	}{legacy.Exists, legacy.Masked})
	return nil
}

func (h *handler) changeLegacyAdminKey(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if err := h.s.StepUp(r, a, input.TOTP); err != nil {
		return err
	}
	const path = "/admin/settings/admin-api-key"
	if r.Method == http.MethodDelete {
		_, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", nil)
		return platform.Send(w, 200, struct{}{}, err)
	}
	result, err := platform.Upstream[struct {
		Key string `json:"key"`
	}](r.Context(), h.s.Upstream, r.Method, path+"/regenerate", a.Tokens.Access, "", "", nil)
	if err == nil && result.Key == "" {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, result, err)
}

func (h *handler) connectionInfo(w http.ResponseWriter, _ *http.Request, _ core.SessionAuth) error {
	h.s.RuntimeMu.RLock()
	base := h.s.Upstream.Base
	h.s.RuntimeMu.RUnlock()
	platform.WriteJSON(w, 200, struct {
		Upstream string `json:"upstream"`
		Official string `json:"official"`
		Origin   string `json:"origin"`
		Stats    bool   `json:"account_stats"`
	}{base, h.s.Official, h.s.Origin, h.s.OfficialDB != nil})
	return nil
}

func (h *handler) systemVersions(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	type officialVersions struct {
		Current   string `json:"current_version"`
		Latest    string `json:"latest_version"`
		HasUpdate bool   `json:"has_update"`
		BuildType string `json:"build_type"`
	}
	data, err := platform.Upstream[officialVersions](r.Context(), h.s.Upstream, "GET", "/admin/system/check-updates", a.Tokens.Access, "", "", nil)
	platform.WriteJSON(w, 200, struct {
		Panel    string            `json:"panel_version"`
		Commit   string            `json:"panel_commit"`
		Upstream *officialVersions `json:"upstream,omitempty"`
	}{currentBuildVersion(), platform.BuildCommit(), func() *officialVersions {
		if err != nil {
			return nil
		}
		return &data
	}()})
	return nil
}

func (h *handler) officialSystemAction(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	action := r.PathValue("action")
	switch action {
	case "update", "rollback", "restart":
	default:
		return platform.Fail(404, "not_found")
	}
	input, err := platform.DecodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if err := h.s.StepUp(r, a, input.TOTP); err != nil {
		return err
	}
	result, err := platform.Upstream[struct {
		Restart bool `json:"need_restart"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/system/"+action, a.Tokens.Access, "", "", struct{}{})
	return platform.Send(w, 200, result, err)
}
