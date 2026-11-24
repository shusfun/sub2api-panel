package main

import (
	"net/http"
	"net/url"
)

type totpIdentityProof struct {
	EmailCode string `json:"email_code,omitempty"`
	Password  string `json:"password,omitempty"`
}

func (p *panel) settingsRoutes(mux *http.ServeMux) {
	p.personalSettingsRoutes(mux)
	p.adminSettingsRoutes(mux)
	p.route(mux, "GET /api/settings/totp", false, p.totpStatus)
	p.route(mux, "GET /api/settings/totp/verification-method", false, p.totpVerificationMethod)
	p.route(mux, "POST /api/settings/totp/send-code", false, p.totpSendCode)
	p.route(mux, "POST /api/settings/totp/setup", false, p.totpSetup)
	p.route(mux, "POST /api/settings/totp/enable", false, p.totpEnable)
	p.route(mux, "POST /api/settings/totp/disable", false, p.totpDisable)
}

func (p *panel) totpStatus(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	w.Header().Set("Cache-Control", "no-store")
	status, err := upstream[struct {
		Enabled *bool `json:"enabled"`
		Feature *bool `json:"feature_enabled"`
	}](r.Context(), p.upstream, "GET", "/user/totp/status", a.tokens.Access, "", "", nil)
	if err == nil && (status.Enabled == nil || status.Feature == nil) {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, status, err)
}

func (p *panel) totpVerificationMethod(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	result, err := upstream[struct {
		Method string `json:"method"`
	}](r.Context(), p.upstream, "GET", "/user/totp/verification-method", a.tokens.Access, "", "", nil)
	if err == nil && result.Method != "email" && result.Method != "password" {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, result, err)
}

func (p *panel) totpSendCode(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	return p.totpAction(w, r, a, "send-code", nil)
}

func readTOTPProof(r *http.Request) (totpIdentityProof, error) {
	proof, err := decodeBody[totpIdentityProof](r)
	if err != nil || (proof.Password == "") == (proof.EmailCode == "") || len(proof.Password) > 256 ||
		proof.EmailCode != "" && !otpPattern.MatchString(proof.EmailCode) {
		return proof, fail(400, "invalid_input")
	}
	return proof, nil
}

func (p *panel) totpSetup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	proof, err := readTOTPProof(r)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := upstream[struct {
		URI       string `json:"qr_code_url"`
		Token     string `json:"setup_token"`
		Countdown int    `json:"countdown"`
	}](r.Context(), p.upstream, "POST", "/user/totp/setup", a.tokens.Access, "", "", proof)
	if err != nil {
		return err
	}
	uri, err := url.Parse(result.URI)
	if err != nil || uri.Scheme != "otpauth" || uri.Host != "totp" || uri.Query().Get("secret") == "" ||
		len(result.URI) > 2048 || len(result.Token) == 0 || len(result.Token) > 512 || result.Countdown <= 0 {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, result, nil)
}

func (p *panel) totpEnable(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		TOTP  string `json:"totp"`
		Token string `json:"setup_token"`
	}](r)
	if err != nil {
		return err
	}
	if !otpPattern.MatchString(input.TOTP) {
		return failField("totp", "请输入 6 位数字验证码")
	}
	if len(input.Token) == 0 || len(input.Token) > 512 {
		return fail(400, "invalid_input")
	}
	return p.totpAction(w, r, a, "enable", struct {
		Code  string `json:"totp_code"`
		Token string `json:"setup_token"`
	}{input.TOTP, input.Token})
}

func (p *panel) totpDisable(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	proof, err := readTOTPProof(r)
	if err != nil {
		return err
	}
	return p.totpAction(w, r, a, "disable", proof)
}

func (p *panel) totpAction(w http.ResponseWriter, r *http.Request, a sessionAuth, action string, input any) error {
	result, err := upstream[struct {
		Success bool `json:"success"`
	}](r.Context(), p.upstream, "POST", "/user/totp/"+action, a.tokens.Access, "", "", input)
	if err == nil && !result.Success {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, struct{}{}, err)
}
