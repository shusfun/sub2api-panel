package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func RegisterRoutes(mux *http.ServeMux, s *core.Server) {
	h := &handler{s: s}
	s.Public(mux, "GET /api/config", h.publicConfig)
	s.Public(mux, "GET /api/setup", h.setupStatus)
	s.Public(mux, "POST /api/setup/discover", h.setupDiscover)
	s.Public(mux, "POST /api/setup/confirm", h.setupConfirm)
	s.Public(mux, "POST /api/session/challenge", h.challenge)
	s.Public(mux, "POST /api/session/login", h.login)
	s.Public(mux, "POST /api/session/verify", h.verifyLogin)
	s.Route(mux, "GET /api/session", false, h.sessionInfo)
	// Local logout must work even after the upstream session was revoked.
	s.Public(mux, "POST /api/session/logout", h.logout)
	h.registerPublicAuthRoutes(mux)
}

type handler struct {
	s *core.Server
}

func (h *handler) sessionInfo(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	stepUp := a.User.Role == "admin" && h.s.StepUpEnabled(r.Context(), a.Tokens.Access)
	var totp bool
	if status, err := platform.Upstream[struct {
		Enabled *bool `json:"enabled"`
	}](r.Context(), h.s.Upstream, "GET", "/user/totp/status", a.Tokens.Access, "", "", nil); err == nil && status.Enabled != nil {
		totp = *status.Enabled
	}
	platform.WriteJSON(w, 200, struct {
		Admin         bool   `json:"admin"`
		CSRF          string `json:"csrf"`
		StepUpEnabled bool   `json:"step_up_enabled"`
		TOTPEnabled   bool   `json:"totp_enabled"`
	}{a.User.Role == "admin", a.Local.CSRF, stepUp, totp})
	return nil
}

func (h *handler) setupStatus(w http.ResponseWriter, _ *http.Request) error {
	h.s.RuntimeMu.RLock()
	required := h.s.Setup
	h.s.RuntimeMu.RUnlock()
	platform.WriteJSON(w, http.StatusOK, struct {
		Required bool `json:"required"`
	}{required})
	return nil
}

func (h *handler) setupDiscover(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Address string `json:"address"`
	}](r)
	if err != nil {
		return err
	}
	result := make([]platform.DiscoveredSub2API, 0, 2)
	for _, candidate := range platform.LocalSub2APICandidates(input.Address) {
		found, probeErr := platform.ProbeSub2API(r.Context(), candidate)
		if probeErr == nil {
			result = append(result, found)
		}
	}
	platform.WriteJSON(w, http.StatusOK, struct {
		Candidates []platform.DiscoveredSub2API `json:"candidates"`
	}{result})
	return nil
}

func (h *handler) setupConfirm(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		URL string `json:"url"`
	}](r)
	if err != nil {
		return err
	}
	if h.s.Database == nil {
		return platform.Fail(503, "service_unavailable")
	}
	if _, err := platform.ProbeSub2API(r.Context(), input.URL); err != nil {
		return platform.Fail(400, "invalid_upstream")
	}
	if err := platform.SetRuntimeUpstream(r.Context(), h.s.Database, input.URL); err != nil {
		return err
	}
	h.s.RuntimeMu.Lock()
	h.s.Upstream = platform.NewClient(strings.TrimRight(input.URL, "/"))
	h.s.Setup = false
	h.s.RuntimeMu.Unlock()
	platform.WriteJSON(w, http.StatusOK, struct{}{})
	return nil
}

func (h *handler) publicConfig(w http.ResponseWriter, r *http.Request) error {
	h.s.RuntimeMu.RLock()
	setup := h.s.Setup
	h.s.RuntimeMu.RUnlock()
	if setup {
		platform.WriteJSON(w, 200, struct {
			Official  string `json:"official"`
			Agreement bool   `json:"agreement"`
			Affiliate bool   `json:"affiliate"`
			Invite    bool   `json:"invitation"`
		}{h.s.Official, false, false, false})
		return nil
	}
	settings, err := h.s.Settings(r.Context())
	if err != nil {
		return err
	}
	_, patrolHome, patrolUser, guestProbe, _ := h.s.PatrolVisibility()
	siteName := strings.TrimSpace(settings.SiteName)
	if siteName == "" {
		siteName = "Sub2API"
	}
	whitelist := settings.EmailWhitelist
	if whitelist == nil {
		whitelist = []string{}
	}
	docs := settings.AgreementDocs
	if docs == nil {
		docs = []core.AgreementDocView{}
	}
	var oauthProviders []string
	if !settings.BackendMode {
		if settings.GitHubOAuth {
			oauthProviders = append(oauthProviders, "github")
		}
		if settings.GoogleOAuth {
			oauthProviders = append(oauthProviders, "google")
		}
		if settings.LinuxDoOAuth {
			oauthProviders = append(oauthProviders, "linuxdo")
		}
		if settings.WeChatOAuth {
			oauthProviders = append(oauthProviders, "wechat")
		}
		if settings.DingTalkOAuth {
			oauthProviders = append(oauthProviders, "dingtalk")
		}
		if settings.OIDCOAuth {
			oauthProviders = append(oauthProviders, "oidc")
		}
	}
	platform.WriteJSON(w, 200, struct {
		HideCCSImport     bool                    `json:"hide_ccs_import_button"`
		CustomEndpoints   []core.CustomEndpoint   `json:"custom_endpoints,omitempty"`
		MenuItems         []core.CustomMenuItem   `json:"custom_menu_items,omitempty"`
		Official          string                  `json:"official"`
		SiteName          string                  `json:"site_name"`
		SiteSubtitle      string                  `json:"site_subtitle,omitempty"`
		SiteLogo          string                  `json:"site_logo,omitempty"`
		DocURL            string                  `json:"doc_url,omitempty"`
		ContactInfo       string                  `json:"contact_info,omitempty"`
		HomeContent       string                  `json:"home_content,omitempty"`
		CompactHome       bool                    `json:"compact_home"`
		BackendMode       bool                    `json:"backend_mode"`
		Registration      bool                    `json:"registration"`
		EmailVerify       bool                    `json:"email_verify"`
		PromoCode         bool                    `json:"promo_code"`
		PasswordReset     bool                    `json:"password_reset"`
		Passkey           bool                    `json:"passkey"`
		EmailWhitelist    []string                `json:"email_whitelist,omitempty"`
		APIBaseURL        string                  `json:"api_base_url,omitempty"`
		Captcha           *core.CaptchaConfig     `json:"captcha,omitempty"`
		Agreement         bool                    `json:"agreement"`
		AgreementMode     string                  `json:"agreement_mode,omitempty"`
		AgreementRevision string                  `json:"agreement_revision,omitempty"`
		AgreementDocs     []core.AgreementDocView `json:"agreement_docs,omitempty"`
		OAuthProviders    []string                `json:"oauth_providers,omitempty"`
		OIDCProviderName  string                  `json:"oidc_provider_name,omitempty"`
		Affiliate         bool                    `json:"affiliate"`
		Invite            bool                    `json:"invitation"`
		PurchaseEnabled   bool                    `json:"purchase_enabled"`
		Channels          bool                    `json:"available_channels_enabled"`
		ModelPlaza        bool                    `json:"model_plaza_enabled"`
		PlazaRequireAuth  bool                    `json:"model_plaza_require_auth"`
		MCPEnabled        bool                    `json:"mcp_enabled"`
		ChannelMonitor    bool                    `json:"channel_monitor_enabled"`
		PatrolHomeVisible bool                    `json:"patrol_home_visible"`
		PatrolUserVisible bool                    `json:"patrol_user_visible"`
		GuestProbeEnabled bool                    `json:"guest_probe_enabled"`
		DevAccounts       []core.DevLoginAccount  `json:"dev_accounts,omitempty"`
	}{
		Official:          h.s.Official,
		HideCCSImport:     settings.HideCCSImport,
		CustomEndpoints:   core.ProjectCustomEndpoints(settings.CustomEndpoints),
		MenuItems:         core.ProjectCustomMenu(settings.MenuItems, "user"),
		SiteName:          siteName,
		SiteSubtitle:      strings.TrimSpace(settings.SiteSubtitle),
		SiteLogo:          strings.TrimSpace(settings.SiteLogo),
		DocURL:            strings.TrimSpace(settings.DocURL),
		ContactInfo:       strings.TrimSpace(settings.ContactInfo),
		HomeContent:       settings.HomeContent,
		CompactHome:       settings.CompactHome,
		BackendMode:       settings.BackendMode,
		Registration:      settings.Registration,
		EmailVerify:       settings.EmailVerify,
		PromoCode:         settings.PromoCode,
		PasswordReset:     settings.PasswordReset,
		Passkey:           settings.Passkey,
		EmailWhitelist:    whitelist,
		APIBaseURL:        strings.TrimRight(settings.APIBaseURL, "/"),
		Captcha:           core.CaptchaSettings(settings),
		Agreement:         settings.Agreement,
		AgreementMode:     settings.AgreementMode,
		AgreementRevision: platform.FirstNonEmpty(settings.AgreementRev, settings.AgreementUpdated),
		AgreementDocs:     docs,
		OAuthProviders:    oauthProviders,
		OIDCProviderName:  strings.TrimSpace(settings.OIDCProviderName),
		Affiliate:         settings.Affiliate,
		Invite:            settings.Invite,
		PurchaseEnabled:   settings.Purchase || settings.Payment,
		Channels:          settings.Channels,
		ModelPlaza:        settings.ModelPlaza,
		PlazaRequireAuth:  settings.PlazaRequireAuth,
		MCPEnabled:        settings.MCPEnabled,
		ChannelMonitor:    settings.ChannelMonitor,
		PatrolHomeVisible: patrolHome,
		PatrolUserVisible: patrolUser,
		GuestProbeEnabled: guestProbe,
		DevAccounts:       h.s.DevAccounts,
	})
	return nil
}

func (h *handler) challenge(w http.ResponseWriter, _ *http.Request) error {
	id, err := h.s.Sessions.Challenge(time.Now(), "", "")
	if err != nil {
		return err
	}
	platform.WriteJSON(w, 200, struct {
		Challenge string `json:"challenge"`
	}{id})
	return nil
}

type loginProof struct {
	Challenge string `json:"challenge"`
	Browser   string `json:"browser_key"`
}

type officialLogin struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Captcha  string `json:"turnstile_token,omitempty"`
	Ticket   string `json:"tencent_captcha_ticket,omitempty"`
	Rand     string `json:"tencent_captcha_randstr,omitempty"`
}

func (h *handler) consumeLoginProof(r *http.Request, proof loginProof, is2FA bool) (string, error) {
	key, err := platform.ParseBrowserKey(proof.Browser)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if _, err := platform.VerifySignature(r, h.s.Origin, key, now); err != nil {
		return "", err
	}
	return h.s.Sessions.TakeChallenge(proof.Challenge, now, proof.Browser, is2FA)
}

func readLoginBody(_ http.ResponseWriter, r *http.Request) (io.ReadCloser, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) > platform.MaxBody {
		return nil, platform.Fail(413, "invalid_input")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		loginProof
		officialLogin
		Agreement bool `json:"agreement"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := h.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	if input.Email == "" || input.Password == "" {
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
	}](r.Context(), h.s.Upstream, "POST", "/auth/login", "", "", "", input.officialLogin)
	if err != nil {
		return loginError(err)
	}
	if result.NeedsOTP {
		if result.Temp == "" {
			return platform.Fail(502, "invalid_upstream")
		}
		pending, err := h.s.Sessions.Challenge(time.Now(), input.Browser, result.Temp)
		if err != nil {
			return err
		}
		platform.WriteJSON(w, 200, struct {
			Pending string `json:"pending"`
		}{pending})
		return nil
	}
	return h.startSession(w, r, input.Browser, result.Credentials)
}

func (h *handler) verifyLogin(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		loginProof
		OTP string `json:"otp"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if !platform.OTPPattern.MatchString(input.OTP) {
		return platform.Fail(400, "invalid_input")
	}
	upstreamOTP, err := h.consumeLoginProof(r, input.loginProof, true)
	if err != nil {
		return err
	}
	result, err := platform.Upstream[platform.Credentials](r.Context(), h.s.Upstream, "POST", "/auth/login/2fa", "", "", "", struct {
		Temp string `json:"temp_token"`
		OTP  string `json:"totp_code"`
	}{upstreamOTP, input.OTP})
	if err != nil {
		return loginError(err)
	}
	return h.startSession(w, r, input.Browser, result)
}

func loginError(err error) error {
	var status platform.UpstreamStatus
	if errors.As(err, &status) && status.Status == http.StatusUnauthorized {
		return platform.Fail(http.StatusUnauthorized, "login_rejected")
	}
	return err
}

func (h *handler) startSession(w http.ResponseWriter, r *http.Request, browser string, tokens platform.Credentials) error {
	if tokens.Access == "" {
		return platform.Fail(502, "invalid_upstream")
	}
	key, err := platform.ParseBrowserKey(browser)
	if err != nil {
		return err
	}
	user, err := platform.CurrentUser(r.Context(), h.s.Upstream, tokens.Access)
	if err != nil {
		return err
	}
	id, err := h.s.Sessions.Create(time.Now(), key, user, tokens)
	if err != nil {
		return err
	}
	platform.SetSessionCookie(w, id, 12*60*60)
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) error {
	active, err := h.s.Sessions.Authenticate(r, h.s.Origin, time.Now())
	if err != nil {
		return err
	}
	tokens := h.s.Sessions.Revoke(active)
	platform.SetSessionCookie(w, "", -1)
	if tokens.Refresh != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		_, _ = platform.Upstream[struct{}](ctx, h.s.Upstream, "POST", "/auth/logout", "", "", "", struct {
			Token string `json:"refresh_token"`
		}{tokens.Refresh})
	}
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}
