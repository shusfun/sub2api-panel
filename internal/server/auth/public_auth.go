package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
)

var oauthProviderNames = map[string]bool{
	"linuxdo":  true,
	"github":   true,
	"google":   true,
	"wechat":   true,
	"dingtalk": true,
	"oidc":     true,
}

func (h *handler) registerPublicAuthRoutes(mux *http.ServeMux) {
	h.s.Public(mux, "POST /api/session/send-verify-code", h.publicSendVerifyCode)
	h.s.Public(mux, "POST /api/session/validate-invite", h.publicValidateInvite)
	h.s.Public(mux, "POST /api/session/validate-promo", h.publicValidatePromo)
	h.s.Public(mux, "POST /api/session/register", h.publicRegister)
	h.s.Public(mux, "POST /api/session/forgot-password", h.publicForgotPassword)
	h.s.Public(mux, "POST /api/session/reset-password", h.publicResetPassword)
	h.s.Public(mux, "POST /api/session/passkey/begin", h.publicPasskeyBegin)
	h.s.Public(mux, "POST /api/session/passkey/finish", h.publicPasskeyFinish)
	h.s.Public(mux, "POST /api/session/oauth/{provider}/start", h.publicOAuthStart)
	h.s.Public(mux, "POST /api/session/oauth/complete", h.publicOAuthComplete)
	h.s.Public(mux, "POST /api/session/oauth/pending/exchange", h.publicOAuthPendingExchange)
	h.s.Public(mux, "POST /api/session/oauth/pending/send-code", h.publicOAuthPendingSendCode)
	h.s.Public(mux, "POST /api/session/oauth/pending/create", h.publicOAuthPendingCreate)
	h.s.Public(mux, "POST /api/session/oauth/pending/bind", h.publicOAuthPendingBind)
}

func (h *handler) publicSendVerifyCode(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Email   string `json:"email"`
		Captcha string `json:"turnstile_token,omitempty"`
		Ticket  string `json:"tencent_captcha_ticket,omitempty"`
		Rand    string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || !platform.ValidEmail(strings.TrimSpace(input.Email)) {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Countdown int `json:"countdown"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/send-verify-code", "", "", "", map[string]any{
		"email":                   strings.TrimSpace(input.Email),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err != nil {
		return err
	}
	if res.Countdown <= 0 {
		res.Countdown = 60
	}
	platform.WriteJSON(w, 200, res)
	return nil
}

func (h *handler) publicValidateInvite(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Code string `json:"code"`
	}](r)
	if err != nil || strings.TrimSpace(input.Code) == "" || len(input.Code) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Valid     bool   `json:"valid"`
		ErrorCode string `json:"error_code,omitempty"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/validate-invitation-code", "", "", "", map[string]string{
		"code": strings.TrimSpace(input.Code),
	})
	return platform.Send(w, 200, res, err)
}

func (h *handler) publicValidatePromo(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Code string `json:"code"`
	}](r)
	if err != nil || strings.TrimSpace(input.Code) == "" || len(input.Code) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Valid       bool    `json:"valid"`
		BonusAmount float64 `json:"bonus_amount,omitempty"`
		ErrorCode   string  `json:"error_code,omitempty"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/validate-promo-code", "", "", "", map[string]string{
		"code": strings.TrimSpace(input.Code),
	})
	return platform.Send(w, 200, res, err)
}

func (h *handler) publicRegister(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		loginProof
		Email      string `json:"email"`
		Password   string `json:"password"`
		VerifyCode string `json:"verify_code,omitempty"`
		InviteCode string `json:"invitation_code,omitempty"`
		PromoCode  string `json:"promo_code,omitempty"`
		AffCode    string `json:"aff_code,omitempty"`
		Captcha    string `json:"turnstile_token,omitempty"`
		Ticket     string `json:"tencent_captcha_ticket,omitempty"`
		Rand       string `json:"tencent_captcha_randstr,omitempty"`
		Agreement  bool   `json:"agreement"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := h.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	email := strings.TrimSpace(input.Email)
	if !platform.ValidEmail(email) || len(input.Password) < 6 || len(input.Password) > 256 {
		return platform.Fail(400, "invalid_input")
	}
	settings, err := h.s.Settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.Registration || settings.BackendMode {
		return platform.Fail(403, "not_allowed")
	}
	if settings.Agreement && !input.Agreement {
		return platform.Fail(400, "agreement_required")
	}
	tokens, err := platform.Upstream[platform.Credentials](r.Context(), h.s.Upstream, "POST", "/auth/register", "", "", "", map[string]any{
		"email":                   email,
		"password":                input.Password,
		"verify_code":             strings.TrimSpace(input.VerifyCode),
		"invitation_code":         strings.TrimSpace(input.InviteCode),
		"promo_code":              strings.TrimSpace(input.PromoCode),
		"aff_code":                strings.TrimSpace(input.AffCode),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err != nil {
		return err
	}
	return h.startSession(w, r, input.Browser, tokens)
}

func (h *handler) publicForgotPassword(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Email   string `json:"email"`
		Captcha string `json:"turnstile_token,omitempty"`
		Ticket  string `json:"tencent_captcha_ticket,omitempty"`
		Rand    string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || !platform.ValidEmail(strings.TrimSpace(input.Email)) {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "POST", "/auth/forgot-password", "", "", "", map[string]any{
		"email":                   strings.TrimSpace(input.Email),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) publicResetPassword(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Email       string `json:"email,omitempty"`
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}](r)
	if err != nil || strings.TrimSpace(input.Token) == "" || len(input.Token) > 512 || len(input.NewPassword) < 6 || len(input.NewPassword) > 256 {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "POST", "/auth/reset-password", "", "", "", map[string]any{
		"email":        strings.TrimSpace(input.Email),
		"token":        strings.TrimSpace(input.Token),
		"new_password": input.NewPassword,
	})
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) publicPasskeyBegin(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Email   string `json:"email,omitempty"`
		Captcha string `json:"turnstile_token,omitempty"`
		Ticket  string `json:"tencent_captcha_ticket,omitempty"`
		Rand    string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || len(input.Email) > 320 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Token   string          `json:"session_token"`
		Options json.RawMessage `json:"options"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/passkey/login/begin", "", "", "", map[string]any{
		"email":                   strings.TrimSpace(input.Email),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err == nil && (res.Token == "" || len(res.Token) > 2048 || len(res.Options) < 2 || len(res.Options) > 32<<10) {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, res, err)
}

func (h *handler) publicPasskeyFinish(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		loginProof
		Token      string          `json:"session_token"`
		Credential json.RawMessage `json:"credential"`
		Agreement  bool            `json:"agreement"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := h.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	if input.Token == "" || len(input.Token) > 2048 || len(input.Credential) < 2 || len(input.Credential) > 48<<10 {
		return platform.Fail(400, "invalid_input")
	}
	settings, err := h.s.Settings(r.Context())
	if err != nil {
		return err
	}
	if settings.Agreement && !input.Agreement {
		return platform.Fail(400, "agreement_required")
	}
	result, err := platform.Upstream[struct {
		platform.Credentials
		NeedsOTP bool   `json:"requires_2fa"`
		Temp     string `json:"temp_token"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/passkey/login/finish", "", "", "", map[string]any{
		"session_token": input.Token,
		"credential":    input.Credential,
	})
	if err != nil {
		return loginError(err)
	}
	if result.NeedsOTP {
		pending, cErr := h.s.Sessions.Challenge(time.Now(), input.Browser, result.Temp)
		if cErr != nil {
			return cErr
		}
		platform.WriteJSON(w, 200, struct {
			Pending string `json:"pending"`
		}{pending})
		return nil
	}
	return h.startSession(w, r, input.Browser, result.Credentials)
}

func (h *handler) publicOAuthStart(w http.ResponseWriter, r *http.Request) error {
	provider := strings.TrimSpace(r.PathValue("provider"))
	if !oauthProviderNames[provider] {
		return platform.Fail(400, "invalid_input")
	}
	input, err := platform.DecodeBody[struct {
		AffCode   string `json:"aff_code,omitempty"`
		PromoCode string `json:"promo_code,omitempty"`
		Captcha   string `json:"turnstile_token,omitempty"`
		Ticket    string `json:"tencent_captcha_ticket,omitempty"`
		Rand      string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	res, err := platform.Upstream[struct {
		URL         string `json:"authorize_url"`
		AuthURL     string `json:"auth_url"`
		RedirectURL string `json:"url"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/oauth/"+provider+"/start", "", "", "", map[string]any{
		"redirect":                h.s.Origin + "/auth/callback",
		"aff_code":                strings.TrimSpace(input.AffCode),
		"promo_code":              strings.TrimSpace(input.PromoCode),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err != nil {
		return err
	}
	target := platform.FirstNonEmpty(res.URL, res.AuthURL, res.RedirectURL)
	if strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//") {
		target = h.s.Official + target
	}
	if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "http://127.0.0.1") {
		return platform.Fail(502, "invalid_upstream")
	}
	platform.WriteJSON(w, 200, struct {
		URL string `json:"url"`
	}{target})
	return nil
}

func (h *handler) publicOAuthComplete(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		loginProof
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := h.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	if strings.TrimSpace(input.AccessToken) == "" || len(input.AccessToken) > 4096 || len(input.RefreshToken) > 4096 {
		return platform.Fail(400, "invalid_input")
	}
	return h.startSession(w, r, input.Browser, platform.Credentials{
		Access:  strings.TrimSpace(input.AccessToken),
		Refresh: strings.TrimSpace(input.RefreshToken),
	})
}

func (h *handler) publicOAuthPendingExchange(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		SessionToken string `json:"session_token"`
	}](r)
	if err != nil || strings.TrimSpace(input.SessionToken) == "" || len(input.SessionToken) > 2048 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		PendingToken string `json:"pending_auth_token"`
		Provider     string `json:"provider"`
		DisplayName  string `json:"display_name,omitempty"`
		Email        string `json:"email,omitempty"`
		RequireEmail bool   `json:"require_email"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/oauth/pending/exchange", "", "", "", map[string]string{
		"session_token": strings.TrimSpace(input.SessionToken),
	})
	return platform.Send(w, 200, res, err)
}

func (h *handler) publicOAuthPendingSendCode(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		PendingToken string `json:"pending_auth_token"`
		Email        string `json:"email"`
		Captcha      string `json:"turnstile_token,omitempty"`
		Ticket       string `json:"tencent_captcha_ticket,omitempty"`
		Rand         string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || strings.TrimSpace(input.PendingToken) == "" || !platform.ValidEmail(strings.TrimSpace(input.Email)) {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Countdown int `json:"countdown"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/oauth/pending/send-verify-code", "", "", "", map[string]any{
		"pending_auth_token":      strings.TrimSpace(input.PendingToken),
		"email":                   strings.TrimSpace(input.Email),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err == nil && res.Countdown <= 0 {
		res.Countdown = 60
	}
	return platform.Send(w, 200, res, err)
}

func (h *handler) publicOAuthPendingCreate(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		loginProof
		PendingToken string `json:"pending_auth_token"`
		Email        string `json:"email,omitempty"`
		VerifyCode   string `json:"verify_code,omitempty"`
		Password     string `json:"password,omitempty"`
		InviteCode   string `json:"invitation_code,omitempty"`
		PromoCode    string `json:"promo_code,omitempty"`
		AffCode      string `json:"aff_code,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := h.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	tokens, err := platform.Upstream[platform.Credentials](r.Context(), h.s.Upstream, "POST", "/auth/oauth/pending/create-account", "", "", "", map[string]any{
		"pending_auth_token": strings.TrimSpace(input.PendingToken),
		"email":              strings.TrimSpace(input.Email),
		"verify_code":        strings.TrimSpace(input.VerifyCode),
		"password":           input.Password,
		"invitation_code":    strings.TrimSpace(input.InviteCode),
		"promo_code":         strings.TrimSpace(input.PromoCode),
		"aff_code":           strings.TrimSpace(input.AffCode),
	})
	if err != nil {
		return err
	}
	return h.startSession(w, r, input.Browser, tokens)
}

func (h *handler) publicOAuthPendingBind(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		loginProof
		PendingToken string `json:"pending_auth_token"`
		Email        string `json:"email"`
		Password     string `json:"password"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := h.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	result, err := platform.Upstream[struct {
		platform.Credentials
		NeedsOTP bool   `json:"requires_2fa"`
		Temp     string `json:"temp_token"`
	}](r.Context(), h.s.Upstream, "POST", "/auth/oauth/pending/bind-login", "", "", "", map[string]any{
		"pending_auth_token": strings.TrimSpace(input.PendingToken),
		"email":              strings.TrimSpace(input.Email),
		"password":           input.Password,
	})
	if err != nil {
		return loginError(err)
	}
	if result.NeedsOTP {
		pending, cErr := h.s.Sessions.Challenge(time.Now(), input.Browser, result.Temp)
		if cErr != nil {
			return cErr
		}
		platform.WriteJSON(w, 200, struct {
			Pending string `json:"pending"`
		}{pending})
		return nil
	}
	return h.startSession(w, r, input.Browser, result.Credentials)
}
