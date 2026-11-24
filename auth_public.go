package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type agreementDocView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
}

func (p *panel) publicAuthRoutes(mux *http.ServeMux) {
	p.public(mux, "POST /api/session/send-verify-code", p.publicSendVerifyCode)
	p.public(mux, "POST /api/session/validate-invite", p.publicValidateInvite)
	p.public(mux, "POST /api/session/validate-promo", p.publicValidatePromo)
	p.public(mux, "POST /api/session/register", p.publicRegister)
	p.public(mux, "POST /api/session/forgot-password", p.publicForgotPassword)
	p.public(mux, "POST /api/session/reset-password", p.publicResetPassword)
	p.public(mux, "POST /api/session/passkey/begin", p.publicPasskeyBegin)
	p.public(mux, "POST /api/session/passkey/finish", p.publicPasskeyFinish)
	p.public(mux, "POST /api/session/oauth/{provider}/start", p.publicOAuthStart)
	p.public(mux, "POST /api/session/oauth/complete", p.publicOAuthComplete)
	p.public(mux, "POST /api/session/oauth/pending/exchange", p.publicOAuthPendingExchange)
	p.public(mux, "POST /api/session/oauth/pending/send-code", p.publicOAuthPendingSendCode)
	p.public(mux, "POST /api/session/oauth/pending/create", p.publicOAuthPendingCreate)
	p.public(mux, "POST /api/session/oauth/pending/bind", p.publicOAuthPendingBind)
}

func (p *panel) publicSendVerifyCode(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Email   string `json:"email"`
		Captcha string `json:"turnstile_token,omitempty"`
		Ticket  string `json:"tencent_captcha_ticket,omitempty"`
		Rand    string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || !validEmail(strings.TrimSpace(input.Email)) {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Countdown int `json:"countdown"`
	}](r.Context(), p.upstream, "POST", "/auth/send-verify-code", "", "", "", map[string]any{
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
	writeJSON(w, 200, res)
	return nil
}

func (p *panel) publicValidateInvite(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Code string `json:"code"`
	}](r)
	if err != nil || strings.TrimSpace(input.Code) == "" || len(input.Code) > 128 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Valid     bool   `json:"valid"`
		ErrorCode string `json:"error_code,omitempty"`
	}](r.Context(), p.upstream, "POST", "/auth/validate-invitation-code", "", "", "", map[string]string{
		"code": strings.TrimSpace(input.Code),
	})
	return send(w, 200, res, err)
}

func (p *panel) publicValidatePromo(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Code string `json:"code"`
	}](r)
	if err != nil || strings.TrimSpace(input.Code) == "" || len(input.Code) > 128 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Valid       bool    `json:"valid"`
		BonusAmount float64 `json:"bonus_amount,omitempty"`
		ErrorCode   string  `json:"error_code,omitempty"`
	}](r.Context(), p.upstream, "POST", "/auth/validate-promo-code", "", "", "", map[string]string{
		"code": strings.TrimSpace(input.Code),
	})
	return send(w, 200, res, err)
}

func (p *panel) publicRegister(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
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
	if _, err := p.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	email := strings.TrimSpace(input.Email)
	if !validEmail(email) || len(input.Password) < 6 || len(input.Password) > 256 {
		return fail(400, "invalid_input")
	}
	settings, err := p.settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.Registration || settings.BackendMode {
		return fail(403, "not_allowed")
	}
	if settings.Agreement && !input.Agreement {
		return fail(400, "agreement_required")
	}
	tokens, err := upstream[credentials](r.Context(), p.upstream, "POST", "/auth/register", "", "", "", map[string]any{
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
	return p.startSession(w, r, input.Browser, tokens)
}

func (p *panel) publicForgotPassword(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Email   string `json:"email"`
		Captcha string `json:"turnstile_token,omitempty"`
		Ticket  string `json:"tencent_captcha_ticket,omitempty"`
		Rand    string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || !validEmail(strings.TrimSpace(input.Email)) {
		return fail(400, "invalid_input")
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "POST", "/auth/forgot-password", "", "", "", map[string]any{
		"email":                   strings.TrimSpace(input.Email),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	return send(w, 200, struct{}{}, err)
}

func (p *panel) publicResetPassword(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Email       string `json:"email,omitempty"`
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}](r)
	if err != nil || strings.TrimSpace(input.Token) == "" || len(input.Token) > 512 || len(input.NewPassword) < 6 || len(input.NewPassword) > 256 {
		return fail(400, "invalid_input")
	}
	_, err = upstream[json.RawMessage](r.Context(), p.upstream, "POST", "/auth/reset-password", "", "", "", map[string]any{
		"email":        strings.TrimSpace(input.Email),
		"token":        strings.TrimSpace(input.Token),
		"new_password": input.NewPassword,
	})
	return send(w, 200, struct{}{}, err)
}

func (p *panel) publicPasskeyBegin(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Email   string `json:"email,omitempty"`
		Captcha string `json:"turnstile_token,omitempty"`
		Ticket  string `json:"tencent_captcha_ticket,omitempty"`
		Rand    string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || len(input.Email) > 320 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Token   string          `json:"session_token"`
		Options json.RawMessage `json:"options"`
	}](r.Context(), p.upstream, "POST", "/auth/passkey/login/begin", "", "", "", map[string]any{
		"email":                   strings.TrimSpace(input.Email),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err == nil && (res.Token == "" || len(res.Token) > 2048 || len(res.Options) < 2 || len(res.Options) > 32<<10) {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, res, err)
}

func (p *panel) publicPasskeyFinish(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		loginProof
		Token      string          `json:"session_token"`
		Credential json.RawMessage `json:"credential"`
		Agreement  bool            `json:"agreement"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := p.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	if input.Token == "" || len(input.Token) > 2048 || len(input.Credential) < 2 || len(input.Credential) > 48<<10 {
		return fail(400, "invalid_input")
	}
	settings, err := p.settings(r.Context())
	if err != nil {
		return err
	}
	if settings.Agreement && !input.Agreement {
		return fail(400, "agreement_required")
	}
	result, err := upstream[struct {
		credentials
		NeedsOTP bool   `json:"requires_2fa"`
		Temp     string `json:"temp_token"`
	}](r.Context(), p.upstream, "POST", "/auth/passkey/login/finish", "", "", "", map[string]any{
		"session_token": input.Token,
		"credential":    input.Credential,
	})
	if err != nil {
		return loginError(err)
	}
	if result.NeedsOTP {
		pending, cErr := p.sessions.challenge(time.Now(), input.Browser, result.Temp)
		if cErr != nil {
			return cErr
		}
		writeJSON(w, 200, struct {
			Pending string `json:"pending"`
		}{pending})
		return nil
	}
	return p.startSession(w, r, input.Browser, result.credentials)
}

func (p *panel) publicOAuthStart(w http.ResponseWriter, r *http.Request) error {
	provider := strings.TrimSpace(r.PathValue("provider"))
	if identityProviders[provider] == "" {
		return fail(400, "invalid_input")
	}
	input, err := decodeBody[struct {
		AffCode   string `json:"aff_code,omitempty"`
		PromoCode string `json:"promo_code,omitempty"`
		Captcha   string `json:"turnstile_token,omitempty"`
		Ticket    string `json:"tencent_captcha_ticket,omitempty"`
		Rand      string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	res, err := upstream[struct {
		URL         string `json:"authorize_url"`
		AuthURL     string `json:"auth_url"`
		RedirectURL string `json:"url"`
	}](r.Context(), p.upstream, "POST", "/auth/oauth/"+provider+"/start", "", "", "", map[string]any{
		"redirect":                p.origin + "/auth/callback",
		"aff_code":                strings.TrimSpace(input.AffCode),
		"promo_code":              strings.TrimSpace(input.PromoCode),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err != nil {
		return err
	}
	target := firstNonEmpty(res.URL, res.AuthURL, res.RedirectURL)
	if strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//") {
		target = p.official + target
	}
	if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "http://127.0.0.1") {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 200, struct {
		URL string `json:"url"`
	}{target})
	return nil
}

func (p *panel) publicOAuthComplete(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		loginProof
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := p.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	if strings.TrimSpace(input.AccessToken) == "" || len(input.AccessToken) > 4096 || len(input.RefreshToken) > 4096 {
		return fail(400, "invalid_input")
	}
	return p.startSession(w, r, input.Browser, credentials{
		Access:  strings.TrimSpace(input.AccessToken),
		Refresh: strings.TrimSpace(input.RefreshToken),
	})
}

func (p *panel) publicOAuthPendingExchange(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		SessionToken string `json:"session_token"`
	}](r)
	if err != nil || strings.TrimSpace(input.SessionToken) == "" || len(input.SessionToken) > 2048 {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		PendingToken string `json:"pending_auth_token"`
		Provider     string `json:"provider"`
		DisplayName  string `json:"display_name,omitempty"`
		Email        string `json:"email,omitempty"`
		RequireEmail bool   `json:"require_email"`
	}](r.Context(), p.upstream, "POST", "/auth/oauth/pending/exchange", "", "", "", map[string]string{
		"session_token": strings.TrimSpace(input.SessionToken),
	})
	return send(w, 200, res, err)
}

func (p *panel) publicOAuthPendingSendCode(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		PendingToken string `json:"pending_auth_token"`
		Email        string `json:"email"`
		Captcha      string `json:"turnstile_token,omitempty"`
		Ticket       string `json:"tencent_captcha_ticket,omitempty"`
		Rand         string `json:"tencent_captcha_randstr,omitempty"`
	}](r)
	if err != nil || strings.TrimSpace(input.PendingToken) == "" || !validEmail(strings.TrimSpace(input.Email)) {
		return fail(400, "invalid_input")
	}
	res, err := upstream[struct {
		Countdown int `json:"countdown"`
	}](r.Context(), p.upstream, "POST", "/auth/oauth/pending/send-verify-code", "", "", "", map[string]any{
		"pending_auth_token":      strings.TrimSpace(input.PendingToken),
		"email":                   strings.TrimSpace(input.Email),
		"turnstile_token":         input.Captcha,
		"tencent_captcha_ticket":  input.Ticket,
		"tencent_captcha_randstr": input.Rand,
	})
	if err == nil && res.Countdown <= 0 {
		res.Countdown = 60
	}
	return send(w, 200, res, err)
}

func (p *panel) publicOAuthPendingCreate(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
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
	if _, err := p.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	tokens, err := upstream[credentials](r.Context(), p.upstream, "POST", "/auth/oauth/pending/create-account", "", "", "", map[string]any{
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
	return p.startSession(w, r, input.Browser, tokens)
}

func (p *panel) publicOAuthPendingBind(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		loginProof
		PendingToken string `json:"pending_auth_token"`
		Email        string `json:"email"`
		Password     string `json:"password"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := p.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	result, err := upstream[struct {
		credentials
		NeedsOTP bool   `json:"requires_2fa"`
		Temp     string `json:"temp_token"`
	}](r.Context(), p.upstream, "POST", "/auth/oauth/pending/bind-login", "", "", "", map[string]any{
		"pending_auth_token": strings.TrimSpace(input.PendingToken),
		"email":              strings.TrimSpace(input.Email),
		"password":           input.Password,
	})
	if err != nil {
		return loginError(err)
	}
	if result.NeedsOTP {
		pending, cErr := p.sessions.challenge(time.Now(), input.Browser, result.Temp)
		if cErr != nil {
			return cErr
		}
		writeJSON(w, 200, struct {
			Pending string `json:"pending"`
		}{pending})
		return nil
	}
	return p.startSession(w, r, input.Browser, result.credentials)
}
