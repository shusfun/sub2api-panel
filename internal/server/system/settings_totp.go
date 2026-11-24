package system

import (
	"net/http"
	"net/url"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type TOTPIdentityProof struct {
	EmailCode string `json:"email_code,omitempty"`
	Password  string `json:"password,omitempty"`
}

func (h *handler) settingsRoutes(mux *http.ServeMux) {
	h.personalSettingsRoutes(mux)
	h.adminSettingsRoutes(mux)
	h.s.Route(mux, "GET /api/settings/totp", false, h.totpStatus)
	h.s.Route(mux, "GET /api/settings/totp/verification-method", false, h.totpVerificationMethod)
	h.s.Route(mux, "POST /api/settings/totp/send-code", false, h.totpSendCode)
	h.s.Route(mux, "POST /api/settings/totp/setup", false, h.totpSetup)
	h.s.Route(mux, "POST /api/settings/totp/enable", false, h.totpEnable)
	h.s.Route(mux, "POST /api/settings/totp/disable", false, h.totpDisable)
}

func (h *handler) totpStatus(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	w.Header().Set("Cache-Control", "no-store")
	status, err := platform.Upstream[struct {
		Enabled *bool `json:"enabled"`
		Feature *bool `json:"feature_enabled"`
	}](r.Context(), h.s.Upstream, "GET", "/user/totp/status", a.Tokens.Access, "", "", nil)
	if err == nil && (status.Enabled == nil || status.Feature == nil) {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, status, err)
}

func (h *handler) totpVerificationMethod(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	result, err := platform.Upstream[struct {
		Method string `json:"method"`
	}](r.Context(), h.s.Upstream, "GET", "/user/totp/verification-method", a.Tokens.Access, "", "", nil)
	if err == nil && result.Method != "email" && result.Method != "password" {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, result, err)
}

func (h *handler) totpSendCode(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	return h.totpAction(w, r, a, "send-code", nil)
}

func readTOTPProof(r *http.Request) (TOTPIdentityProof, error) {
	proof, err := platform.DecodeBody[TOTPIdentityProof](r)
	if err != nil || (proof.Password == "") == (proof.EmailCode == "") || len(proof.Password) > 256 ||
		proof.EmailCode != "" && !platform.OTPPattern.MatchString(proof.EmailCode) {
		return proof, platform.Fail(400, "invalid_input")
	}
	return proof, nil
}

func (h *handler) totpSetup(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	proof, err := readTOTPProof(r)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := platform.Upstream[struct {
		URI       string `json:"qr_code_url"`
		Token     string `json:"setup_token"`
		Countdown int    `json:"countdown"`
	}](r.Context(), h.s.Upstream, "POST", "/user/totp/setup", a.Tokens.Access, "", "", proof)
	if err != nil {
		return err
	}
	uri, err := url.Parse(result.URI)
	if err != nil || uri.Scheme != "otpauth" || uri.Host != "totp" || uri.Query().Get("secret") == "" ||
		len(result.URI) > 2048 || len(result.Token) == 0 || len(result.Token) > 512 || result.Countdown <= 0 {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, result, nil)
}

func (h *handler) totpEnable(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		TOTP  string `json:"totp"`
		Token string `json:"setup_token"`
	}](r)
	if err != nil {
		return err
	}
	if !platform.OTPPattern.MatchString(input.TOTP) {
		return platform.FailField("totp", "请输入 6 位数字验证码")
	}
	if len(input.Token) == 0 || len(input.Token) > 512 {
		return platform.Fail(400, "invalid_input")
	}
	return h.totpAction(w, r, a, "enable", struct {
		Code  string `json:"totp_code"`
		Token string `json:"setup_token"`
	}{input.TOTP, input.Token})
}

func (h *handler) totpDisable(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	proof, err := readTOTPProof(r)
	if err != nil {
		return err
	}
	return h.totpAction(w, r, a, "disable", proof)
}

func (h *handler) totpAction(w http.ResponseWriter, r *http.Request, a core.SessionAuth, action string, input any) error {
	result, err := platform.Upstream[struct {
		Success bool `json:"success"`
	}](r.Context(), h.s.Upstream, "POST", "/user/totp/"+action, a.Tokens.Access, "", "", input)
	if err == nil && !result.Success {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, struct{}{}, err)
}
