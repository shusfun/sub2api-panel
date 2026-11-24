package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// 站点设置里不属于「字段读写」的操作：邮件测试与模板、管理密钥状态、面板连接。
func (p *panel) adminSettingToolRoutes(mux *http.ServeMux) {
	p.route(mux, "POST /api/admin/settings-tools/mail/test-connection", true, p.testMailConnection)
	p.route(mux, "POST /api/admin/settings-tools/mail/send-test", true, p.sendTestMail)
	p.route(mux, "GET /api/admin/settings-tools/mail/templates", true, p.mailTemplates)
	p.route(mux, "GET /api/admin/settings-tools/mail/templates/{event}/{locale}", true, p.mailTemplate)
	p.route(mux, "PUT /api/admin/settings-tools/mail/templates/{event}/{locale}", true, p.saveMailTemplate)
	p.route(mux, "POST /api/admin/settings-tools/mail/templates/{event}/{locale}/preview", true, p.previewMailTemplate)
	p.route(mux, "POST /api/admin/settings-tools/mail/templates/{event}/{locale}/restore", true, p.restoreMailTemplate)

	p.route(mux, "GET /api/admin/settings-tools/system", true, p.systemVersions)
	p.route(mux, "POST /api/admin/settings-tools/system/{action}", true, p.officialSystemAction)
	p.route(mux, "GET /api/admin/settings-tools/admin-key", true, p.adminKeyStatus)
	p.route(mux, "POST /api/admin/settings-tools/admin-key/rotate", true, p.changeLegacyAdminKey)
	p.route(mux, "DELETE /api/admin/settings-tools/admin-key", true, p.changeLegacyAdminKey)
	p.route(mux, "GET /api/admin/settings-tools/access-keys", true, p.scopedAdminKeys)
	p.route(mux, "POST /api/admin/settings-tools/access-keys", true, p.scopedAdminKeys)
	p.route(mux, "PUT /api/admin/settings-tools/access-keys/{id}", true, p.scopedAdminKeys)
	p.route(mux, "DELETE /api/admin/settings-tools/access-keys/{id}", true, p.scopedAdminKeys)
	p.route(mux, "POST /api/admin/settings-tools/access-keys/{id}/{action}", true, p.scopedAdminKeys)
	p.route(mux, "DELETE /api/admin/settings-tools/access-keys/{id}/{action}", true, p.scopedAdminKeys)
	p.route(mux, "GET /api/admin/settings-tools/connection", true, p.connectionInfo)
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
	input, err := decodeBody[mailProbe](r)
	if err != nil || len(input.Host) > 255 || input.Port < 0 || input.Port > 65535 || len(input.Username) > 255 ||
		len(input.Password) > 1024 || len(input.From) > 320 || len(input.FromName) > 200 ||
		needEmail != (input.Email != "") || needEmail && !validEmail(input.Email) {
		return input, fail(400, "invalid_input")
	}
	return input, nil
}

func (p *panel) testMailConnection(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := readMailProbe(r, false)
	if err != nil {
		return err
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "POST", "/admin/settings/test-smtp", a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) sendTestMail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := readMailProbe(r, true)
	if err != nil {
		return err
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "POST", "/admin/settings/send-test-email", a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

var templateEventPattern = regexp.MustCompile(`^[a-z0-9_.]{1,80}$`)
var templateLocalePattern = regexp.MustCompile(`^[a-zA-Z-]{2,10}$`)

func templatePath(r *http.Request) (string, error) {
	event, locale := r.PathValue("event"), r.PathValue("locale")
	if !templateEventPattern.MatchString(event) || !templateLocalePattern.MatchString(locale) {
		return "", fail(400, "invalid_input")
	}
	return "/admin/settings/email-templates/" + url.PathEscape(event) + "/" + url.PathEscape(locale), nil
}

func (p *panel) mailTemplates(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	data, err := upstream[struct {
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
	}](r.Context(), p.upstream, "GET", "/admin/settings/email-templates", a.tokens.Access, "", "", nil)
	if err == nil && (len(data.Events) > 200 || len(data.Locales) > 20 || len(data.Templates) > 4000) {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, data, err)
}

type mailTemplateView struct {
	Subject      string   `json:"subject"`
	HTML         string   `json:"html"`
	Placeholders []string `json:"placeholders"`
}

func (p *panel) mailTemplate(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path, err := templatePath(r)
	if err != nil {
		return err
	}
	data, err := upstream[mailTemplateView](r.Context(), p.upstream, "GET", path, a.tokens.Access, "", "", nil)
	return send(w, 200, data, err)
}

type mailTemplateInput struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

func readMailTemplate(r *http.Request) (mailTemplateInput, error) {
	input, err := decodeBody[mailTemplateInput](r)
	if err != nil || strings.TrimSpace(input.Subject) == "" || len(input.Subject) > 500 || input.HTML == "" || len(input.HTML) > settingLongLimit {
		return input, fail(400, "invalid_input")
	}
	return input, nil
}

func (p *panel) saveMailTemplate(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path, err := templatePath(r)
	if err != nil {
		return err
	}
	input, err := readMailTemplate(r)
	if err != nil {
		return err
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "PUT", path, a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) previewMailTemplate(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if _, err := templatePath(r); err != nil {
		return err
	}
	input, err := readMailTemplate(r)
	if err != nil {
		return err
	}
	data, err := upstream[mailTemplateInput](r.Context(), p.upstream, "POST", "/admin/settings/email-template-preview", a.tokens.Access, "", "", struct {
		Event   string `json:"event"`
		Locale  string `json:"locale"`
		Subject string `json:"subject"`
		HTML    string `json:"html"`
	}{r.PathValue("event"), r.PathValue("locale"), input.Subject, input.HTML})
	return send(w, 200, data, err)
}

func (p *panel) restoreMailTemplate(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path, err := templatePath(r)
	if err != nil {
		return err
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "POST", path+"/restore-official", a.tokens.Access, "", "", struct{}{})
	return send(w, 200, struct{}{}, err)
}

func (p *panel) adminKeyStatus(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	legacy, err := upstream[struct {
		Exists bool   `json:"exists"`
		Masked string `json:"masked_key"`
	}](r.Context(), p.upstream, "GET", "/admin/settings/admin-api-key", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	writeJSON(w, 200, struct {
		Legacy bool   `json:"legacy_exists"`
		Masked string `json:"legacy_masked"`
	}{legacy.Exists, legacy.Masked})
	return nil
}

func (p *panel) changeLegacyAdminKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if err := p.stepUp(r, a, input.TOTP); err != nil {
		return err
	}
	const path = "/admin/settings/admin-api-key"
	if r.Method == http.MethodDelete {
		_, err := upstream[struct{}](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", nil)
		return send(w, 200, struct{}{}, err)
	}
	result, err := upstream[struct {
		Key string `json:"key"`
	}](r.Context(), p.upstream, r.Method, path+"/regenerate", a.tokens.Access, "", "", nil)
	if err == nil && result.Key == "" {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, result, err)
}

func (p *panel) connectionInfo(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	p.runtimeMu.RLock()
	base := p.upstream.base
	p.runtimeMu.RUnlock()
	writeJSON(w, 200, struct {
		Upstream string `json:"upstream"`
		Official string `json:"official"`
		Origin   string `json:"origin"`
		Stats    bool   `json:"account_stats"`
	}{base, p.official, p.origin, p.officialDB != nil})
	return nil
}

func (p *panel) systemVersions(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	type officialVersions struct {
		Current   string `json:"current_version"`
		Latest    string `json:"latest_version"`
		HasUpdate bool   `json:"has_update"`
		BuildType string `json:"build_type"`
	}
	data, err := upstream[officialVersions](r.Context(), p.upstream, "GET", "/admin/system/check-updates", a.tokens.Access, "", "", nil)
	writeJSON(w, 200, struct {
		Panel    string            `json:"panel_version"`
		Commit   string            `json:"panel_commit"`
		Upstream *officialVersions `json:"upstream,omitempty"`
	}{version, commit, func() *officialVersions {
		if err != nil {
			return nil
		}
		return &data
	}()})
	return nil
}

func (p *panel) officialSystemAction(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	action := r.PathValue("action")
	switch action {
	case "update", "rollback", "restart":
	default:
		return fail(404, "not_found")
	}
	input, err := decodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if err := p.stepUp(r, a, input.TOTP); err != nil {
		return err
	}
	result, err := upstream[struct {
		Restart bool `json:"need_restart"`
	}](r.Context(), p.upstream, "POST", "/admin/system/"+action, a.tokens.Access, "", "", struct{}{})
	return send(w, 200, result, err)
}
