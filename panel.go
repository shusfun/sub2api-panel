package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/internal/extplugin"
	"sub2api-panel/internal/pluginrt"

	officialent "sub2api-panel/official"
)

type panel struct {
	origin             string
	official           string
	upstream           serviceClient
	issuer             serviceClient
	secret             string
	sessions           *sessions
	usageLocation      *time.Location
	usageSampleVersion string
	usageHub           *usageStreamHub
	opsHub             *opsStreamHub
	accountHub         *opsStreamHub
	balances           *balanceMonitor
	database           *ent.Client
	officialDB         *officialent.Client
	dataDir            string
	pluginRT           *pluginrt.Runtime
	extPlugins         *extplugin.Manager
	devAccounts        []devLoginAccount
	setup              bool
	runtimeMu          sync.RWMutex
}

type sessionAuth struct {
	user   identity
	tokens credentials
	local  *session
}

type task func(http.ResponseWriter, *http.Request, sessionAuth) error

type devLoginAccount struct {
	Label    string `json:"label"`
	Email    string `json:"email"`
	Password string `json:"password"`
	OTP      string `json:"otp,omitempty"`
}

func (p *panel) routes(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", static)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		r, end := traceRequest(r, w, "unknown route")
		defer end()
		writeError(w, r, fail(404, "not_found"))
	})
	p.public(mux, "GET /api/config", p.publicConfig)
	p.route(mux, "GET /api/admin/custom-menu", true, p.adminCustomMenu)
	p.route(mux, "GET /api/admin/compliance", true, p.adminCompliance)
	p.route(mux, "POST /api/admin/compliance/accept", true, p.adminCompliance)
	p.public(mux, "GET /api/setup", p.setupStatus)
	p.public(mux, "POST /api/setup/discover", p.setupDiscover)
	p.public(mux, "POST /api/setup/confirm", p.setupConfirm)
	p.public(mux, "POST /api/session/challenge", p.challenge)
	p.public(mux, "POST /api/session/login", p.login)
	p.public(mux, "POST /api/session/verify", p.verifyLogin)
	p.route(mux, "GET /api/session", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		stepUp := a.user.Role == "admin" && p.stepUpEnabled(r.Context(), a.tokens.Access)
		var totp bool
		if status, err := upstream[struct {
			Enabled *bool `json:"enabled"`
		}](r.Context(), p.upstream, "GET", "/user/totp/status", a.tokens.Access, "", "", nil); err == nil && status.Enabled != nil {
			totp = *status.Enabled
		}
		writeJSON(w, 200, struct {
			Admin         bool   `json:"admin"`
			CSRF          string `json:"csrf"`
			StepUpEnabled bool   `json:"step_up_enabled"`
			TOTPEnabled   bool   `json:"totp_enabled"`
		}{a.user.Role == "admin", a.local.csrf, stepUp, totp})
		return nil
	})
	// Local logout must work even after the upstream session was revoked.
	p.public(mux, "POST /api/session/logout", p.logout)
	p.route(mux, "GET /api/grant", false, issuerCall[grant](p, "GET", fixed("/internal/grant"), nil))
	p.route(mux, "GET /api/issues", false, issuerCall[pageResult[issueEvent]](p, "GET", issuePage, nil))
	p.route(mux, "POST /api/issues", false, issuerCall[issueResult](p, "POST", fixed("/internal/issues"), readInput[issueInput]))
	p.route(mux, "GET /api/admin/grants/{user}", true, issuerCall[grant](p, "GET", grantPath, nil))
	p.route(mux, "PUT /api/admin/grants/{user}", true, issuerCall[struct{}](p, "PUT", grantPath, readInput[grantChange]))
	p.route(mux, "DELETE /api/admin/grants/{user}", true, issuerCall[struct{}](p, "DELETE", grantPath, nil))
	p.route(mux, "POST /api/admin/issues/{operation}/resolve", true, issuerCall[struct{}](p, "POST", resolvePath, readInput[reconciliation]))
	p.route(mux, "GET /api/usage/stream", false, p.usageStream)
	p.publicAuthRoutes(mux)
	p.pluginRoutes(mux)
	p.probeRoutes(mux)
	p.patrolRoutes(mux)
	p.businessRoutes(mux)
	p.settingsRoutes(mux)
	p.siteContentRoutes(mux)
	p.route(mux, "GET /api/admin/audit-logs", true, p.auditLogList)
	p.route(mux, "GET /api/admin/audit-logs/{id}", true, p.auditLogDetail)
	p.route(mux, "POST /api/admin/audit-logs/clear", true, p.auditLogClear)
	p.public(mux, "GET /api/public/model-plaza", func(w http.ResponseWriter, r *http.Request) error {
		return p.modelPlaza(w, r, sessionAuth{})
	})
	p.route(mux, "GET /api/model-plaza", false, p.modelPlaza)
	p.mcpRoutes(mux)
	p.paymentRoutes(mux)
	p.backupRoutes(mux)
	p.riskRoutes(mux)
	p.route(mux, "GET /api/channel-monitor", false, p.channelMonitor)
	p.channelRuleRoutes(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy := "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'none'; form-action 'self'"
		if r.URL.Path == "/verification.html" {
			const vendors = " https://challenges.cloudflare.com https://turing.captcha.qcloud.com https://ca.turing.captcha.qcloud.com https://*.alicdn.com https://*.aliyuncs.com"
			policy = "default-src 'none'; sandbox allow-scripts allow-forms allow-popups; script-src 'self'" + vendors +
				"; style-src 'unsafe-inline'" + vendors + "; img-src data:" + vendors + "; connect-src" + vendors +
				"; frame-src" + vendors + "; frame-ancestors " + p.origin + "; base-uri 'none'; form-action 'none'"
		} else if strings.HasPrefix(r.URL.Path, "/custom/") && isPanelDocumentRoute(r.URL.Path) {
			policy = strings.Replace(policy, "frame-src 'self'", "frame-src https: http:", 1)
		} else if strings.HasPrefix(r.URL.Path, "/api/plugins/frame/") {
			policy = "default-src 'none'; sandbox allow-scripts; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; frame-ancestors " + p.origin + "; base-uri 'none'; form-action 'none'"
		} else if (r.URL.Path == "/key-usage" || r.URL.Path == "/batch-image") && checkOrigin(p.official, false) && !strings.ContainsAny(p.official, ";, ") {
			policy = strings.Replace(policy, "connect-src 'self'", "connect-src 'self' "+p.official, 1)
		}
		// The isolated verification document has an opaque origin and loads one public Vite module.
		if strings.HasPrefix(r.URL.Path, "/assets/verification-") && r.Header.Get("Origin") == "null" {
			w.Header().Set("Access-Control-Allow-Origin", "null")
		}
		w.Header().Set("Content-Security-Policy", policy)
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		mux.ServeHTTP(w, r)
	})
}

func (p *panel) public(mux *http.ServeMux, pattern string, run func(http.ResponseWriter, *http.Request) error) {
	p.publicWithLimit(mux, pattern, maxBody, run)
}

func (p *panel) publicWithLimit(mux *http.ServeMux, pattern string, limit int64, run func(http.ResponseWriter, *http.Request) error) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		r, end := traceRequest(r, w, pattern)
		defer end()
		if r.Method != "GET" && r.Header.Get("Origin") != p.origin {
			writeError(w, r, fail(403, "csrf_rejected"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if limit > maxBody {
			defer func() { _ = r.Body.Close() }()
		}
		if err := run(w, r); err != nil {
			writeError(w, r, translateUpstream(err))
		}
	})
}

func (p *panel) route(mux *http.ServeMux, pattern string, admin bool, run task) {
	p.routeWithLimit(mux, pattern, admin, maxBody, run)
}

func (p *panel) routeWithLimit(mux *http.ServeMux, pattern string, admin bool, limit int64, run task) {
	p.publicWithLimit(mux, pattern, limit, func(w http.ResponseWriter, r *http.Request) error {
		p.runtimeMu.RLock()
		setup := p.setup
		p.runtimeMu.RUnlock()
		if setup {
			return fail(503, "setup_required")
		}
		active, err := p.sessions.authenticateWithLimit(r, p.origin, time.Now(), limit)
		if err != nil {
			return err
		}
		user, tokens, err := p.sessions.principal(r.Context(), p.upstream, active)
		if err != nil {
			return err
		}
		if admin && user.Role != "admin" {
			return fail(403, "not_allowed")
		}
		return run(w, r, sessionAuth{user, tokens, active})
	})
}

func isPanelDocumentRoute(p string) bool {
	switch p {
	case "/", "/index.html", "/_ui", "/login", "/register", "/email-verify", "/forgot-password", "/reset-password", "/auth/callback", "/patrol", "/key-usage", "/model-plaza", "/oauth/mcp/authorize", "/mcp", "/purchase", "/payment/result", "/batch-image",
		"/keys", "/usage", "/settings", "/announcements", "/affiliate", "/channels/status", "/channels/available", "/channels/monitor", "/admin/ops", "/admin/accounts", "/admin/groups", "/admin/proxies", "/admin/users", "/admin/redeem", "/admin/plugins", "/admin/announcements", "/admin/promo-codes", "/admin/audit-logs", "/admin/risk-control", "/admin/backups", "/admin/orders":
		return true
	default:
		parent, id, ok := strings.Cut(strings.TrimPrefix(p, "/"), "/")
		if parent == "admin" {
			parent, id, ok = strings.Cut(id, "/")
			switch parent {
			case "accounts", "groups", "proxies", "users":
			default:
				return false
			}
		} else if parent == "custom" {
			return ok && customSlugPattern.MatchString(id)
		} else if parent != "keys" {
			return false
		}
		if !ok || len(id) == 0 || len(id) > 16 || id[0] == '0' {
			return false
		}
		for _, digit := range id {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		return true
	}
}

func spaFileServer(root fs.FS) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && isPanelDocumentRoute(r.URL.Path) && r.URL.Path != "/" && r.URL.Path != "/index.html" {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, clone)
			return
		}
		files.ServeHTTP(w, r)
	})
}

type settingsWire struct {
	HideCCSImport    bool               `json:"hide_ccs_import_button"`
	CustomEndpoints  []customEndpoint   `json:"custom_endpoints"`
	MenuItems        []customMenuItem   `json:"custom_menu_items"`
	SiteName         string             `json:"site_name"`
	SiteSubtitle     string             `json:"site_subtitle"`
	SiteLogo         string             `json:"site_logo"`
	DocURL           string             `json:"doc_url"`
	ContactInfo      string             `json:"contact_info"`
	HomeContent      string             `json:"home_content"`
	CompactHome      bool               `json:"compact_home_enabled"`
	BackendMode      bool               `json:"backend_mode_enabled"`
	Registration     bool               `json:"registration_enabled"`
	EmailVerify      bool               `json:"email_verify_enabled"`
	PromoCode        bool               `json:"promo_code_enabled"`
	PasswordReset    bool               `json:"password_reset_enabled"`
	Passkey          bool               `json:"passkey_enabled"`
	EmailWhitelist   []string           `json:"registration_email_suffix_whitelist"`
	AgreementMode    string             `json:"login_agreement_mode"`
	AgreementUpdated string             `json:"login_agreement_updated_at"`
	AgreementRev     string             `json:"login_agreement_revision"`
	AgreementDocs    []agreementDocView `json:"login_agreement_documents"`
	LinuxDoOAuth     bool               `json:"linuxdo_oauth_enabled"`
	GitHubOAuth      bool               `json:"github_oauth_enabled"`
	GoogleOAuth      bool               `json:"google_oauth_enabled"`
	WeChatOAuth      bool               `json:"wechat_oauth_enabled"`
	DingTalkOAuth    bool               `json:"dingtalk_oauth_enabled"`
	OIDCOAuth        bool               `json:"oidc_oauth_enabled"`
	OIDCProviderName string             `json:"oidc_oauth_provider_name"`
	PluginManagement bool               `json:"plugin_management_enabled"`
	APIBaseURL       string             `json:"api_base_url"`
	Turnstile        bool               `json:"turnstile_enabled"`
	SiteKey          string             `json:"turnstile_site_key"`
	Tencent          bool               `json:"tencent_captcha_enabled"`
	TencentID        string             `json:"tencent_captcha_app_id"`
	Aliyun           bool               `json:"aliyun_captcha_enabled"`
	Scene            string             `json:"aliyun_captcha_scene_id"`
	Prefix           string             `json:"aliyun_captcha_prefix"`
	Region           string             `json:"aliyun_captcha_region"`
	Agreement        bool               `json:"login_agreement_enabled"`
	Affiliate        bool               `json:"affiliate_enabled"`
	Invite           bool               `json:"invitation_code_enabled"`
	Purchase         bool               `json:"purchase_subscription_enabled"`
	Payment          bool               `json:"payment_enabled"`
	Channels         bool               `json:"available_channels_enabled"`
	ModelPlaza       bool               `json:"model_plaza_enabled"`
	PlazaRequireAuth bool               `json:"model_plaza_require_auth"`
	MCPEnabled       bool               `json:"mcp_enabled"`
	ChannelMonitor   bool               `json:"channel_monitor_enabled"`
	MonitorMode      string             `json:"channel_monitor_mode"`
}

func (p *panel) settings(ctx context.Context) (settingsWire, error) {
	return upstream[settingsWire](ctx, p.upstream, "GET", "/settings/public", "", "", "", nil)
}

type captchaConfig struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Prefix   string `json:"prefix,omitempty"`
	Region   string `json:"region,omitempty"`
}

func captchaSettings(s settingsWire) *captchaConfig {
	switch {
	case s.Turnstile:
		return &captchaConfig{Provider: "turnstile", Key: s.SiteKey}
	case s.Tencent:
		return &captchaConfig{Provider: "tencent", Key: s.TencentID}
	case s.Aliyun:
		return &captchaConfig{Provider: "aliyun", Key: s.Scene, Prefix: s.Prefix, Region: s.Region}
	default:
		return nil
	}
}

func (p *panel) publicConfig(w http.ResponseWriter, r *http.Request) error {
	p.runtimeMu.RLock()
	setup := p.setup
	p.runtimeMu.RUnlock()
	if setup {
		writeJSON(w, 200, struct {
			Official  string `json:"official"`
			Agreement bool   `json:"agreement"`
			Affiliate bool   `json:"affiliate"`
			Invite    bool   `json:"invitation"`
		}{p.official, false, false, false})
		return nil
	}
	settings, err := p.settings(r.Context())
	if err != nil {
		return err
	}
	_, patrolHome, patrolUser, guestProbe, _ := p.patrolVisibility()
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
		docs = []agreementDocView{}
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
	writeJSON(w, 200, struct {
		HideCCSImport     bool               `json:"hide_ccs_import_button"`
		CustomEndpoints   []customEndpoint   `json:"custom_endpoints,omitempty"`
		MenuItems         []customMenuItem   `json:"custom_menu_items,omitempty"`
		Official          string             `json:"official"`
		SiteName          string             `json:"site_name"`
		SiteSubtitle      string             `json:"site_subtitle,omitempty"`
		SiteLogo          string             `json:"site_logo,omitempty"`
		DocURL            string             `json:"doc_url,omitempty"`
		ContactInfo       string             `json:"contact_info,omitempty"`
		HomeContent       string             `json:"home_content,omitempty"`
		CompactHome       bool               `json:"compact_home"`
		BackendMode       bool               `json:"backend_mode"`
		Registration      bool               `json:"registration"`
		EmailVerify       bool               `json:"email_verify"`
		PromoCode         bool               `json:"promo_code"`
		PasswordReset     bool               `json:"password_reset"`
		Passkey           bool               `json:"passkey"`
		EmailWhitelist    []string           `json:"email_whitelist,omitempty"`
		APIBaseURL        string             `json:"api_base_url,omitempty"`
		Captcha           *captchaConfig     `json:"captcha,omitempty"`
		Agreement         bool               `json:"agreement"`
		AgreementMode     string             `json:"agreement_mode,omitempty"`
		AgreementRevision string             `json:"agreement_revision,omitempty"`
		AgreementDocs     []agreementDocView `json:"agreement_docs,omitempty"`
		OAuthProviders    []string           `json:"oauth_providers,omitempty"`
		OIDCProviderName  string             `json:"oidc_provider_name,omitempty"`
		Affiliate         bool               `json:"affiliate"`
		Invite            bool               `json:"invitation"`
		PurchaseEnabled   bool               `json:"purchase_enabled"`
		Channels          bool               `json:"available_channels_enabled"`
		ModelPlaza        bool               `json:"model_plaza_enabled"`
		PlazaRequireAuth  bool               `json:"model_plaza_require_auth"`
		MCPEnabled        bool               `json:"mcp_enabled"`
		ChannelMonitor    bool               `json:"channel_monitor_enabled"`
		PatrolHomeVisible bool               `json:"patrol_home_visible"`
		PatrolUserVisible bool               `json:"patrol_user_visible"`
		GuestProbeEnabled bool               `json:"guest_probe_enabled"`
		DevAccounts       []devLoginAccount  `json:"dev_accounts,omitempty"`
	}{
		Official:          p.official,
		HideCCSImport:     settings.HideCCSImport,
		CustomEndpoints:   projectCustomEndpoints(settings.CustomEndpoints),
		MenuItems:         projectCustomMenu(settings.MenuItems, "user"),
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
		Captcha:           captchaSettings(settings),
		Agreement:         settings.Agreement,
		AgreementMode:     settings.AgreementMode,
		AgreementRevision: firstNonEmpty(settings.AgreementRev, settings.AgreementUpdated),
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
		DevAccounts:       p.devAccounts,
	})
	return nil
}

func (p *panel) challenge(w http.ResponseWriter, _ *http.Request) error {
	id, err := p.sessions.challenge(time.Now(), "", "")
	if err != nil {
		return err
	}
	writeJSON(w, 200, struct {
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

func (p *panel) consumeLoginProof(r *http.Request, proof loginProof, is2FA bool) (string, error) {
	key, err := parseBrowserKey(proof.Browser)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if _, err := verifySignature(r, p.origin, key, now); err != nil {
		return "", err
	}
	return p.sessions.takeChallenge(proof.Challenge, now, proof.Browser, is2FA)
}

func readLoginBody(_ http.ResponseWriter, r *http.Request) (io.ReadCloser, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) > maxBody {
		return nil, fail(413, "invalid_input")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (p *panel) login(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		loginProof
		officialLogin
		Agreement bool `json:"agreement"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if _, err := p.consumeLoginProof(r, input.loginProof, false); err != nil {
		return err
	}
	if input.Email == "" || input.Password == "" {
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
	}](r.Context(), p.upstream, "POST", "/auth/login", "", "", "", input.officialLogin)
	if err != nil {
		return loginError(err)
	}
	if result.NeedsOTP {
		if result.Temp == "" {
			return fail(502, "invalid_upstream")
		}
		pending, err := p.sessions.challenge(time.Now(), input.Browser, result.Temp)
		if err != nil {
			return err
		}
		writeJSON(w, 200, struct {
			Pending string `json:"pending"`
		}{pending})
		return nil
	}
	return p.startSession(w, r, input.Browser, result.credentials)
}

func (p *panel) verifyLogin(w http.ResponseWriter, r *http.Request) error {
	raw, err := readLoginBody(w, r)
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		loginProof
		OTP string `json:"otp"`
	}](r)
	if err != nil {
		return err
	}
	r.Body = raw
	if !otpPattern.MatchString(input.OTP) {
		return fail(400, "invalid_input")
	}
	upstreamOTP, err := p.consumeLoginProof(r, input.loginProof, true)
	if err != nil {
		return err
	}
	result, err := upstream[credentials](r.Context(), p.upstream, "POST", "/auth/login/2fa", "", "", "", struct {
		Temp string `json:"temp_token"`
		OTP  string `json:"totp_code"`
	}{upstreamOTP, input.OTP})
	if err != nil {
		return loginError(err)
	}
	return p.startSession(w, r, input.Browser, result)
}

func loginError(err error) error {
	var status upstreamStatus
	if errors.As(err, &status) && status.status == http.StatusUnauthorized {
		return fail(http.StatusUnauthorized, "login_rejected")
	}
	return err
}

func (p *panel) startSession(w http.ResponseWriter, r *http.Request, browser string, tokens credentials) error {
	if tokens.Access == "" {
		return fail(502, "invalid_upstream")
	}
	key, err := parseBrowserKey(browser)
	if err != nil {
		return err
	}
	user, err := currentUser(r.Context(), p.upstream, tokens.Access)
	if err != nil {
		return err
	}
	id, err := p.sessions.create(time.Now(), key, user, tokens)
	if err != nil {
		return err
	}
	setSessionCookie(w, id, 12*60*60)
	writeJSON(w, 200, struct{}{})
	return nil
}

func setSessionCookie(w http.ResponseWriter, id string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/", Secure: true,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge})
}

func (p *panel) logout(w http.ResponseWriter, r *http.Request) error {
	active, err := p.sessions.authenticate(r, p.origin, time.Now())
	if err != nil {
		return err
	}
	active.mu.Lock()
	tokens, _ := p.sessions.open(active.encrypted)
	active.revoked = true
	active.encrypted = nil
	active.mu.Unlock()
	setSessionCookie(w, "", -1)
	if tokens.Refresh != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		_, _ = upstream[struct{}](ctx, p.upstream, "POST", "/auth/logout", "", "", "", struct {
			Token string `json:"refresh_token"`
		}{tokens.Refresh})
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

const pageSize = 20

type pageResult[T any] struct {
	Items []T  `json:"items"`
	More  bool `json:"more"`
	Pages int  `json:"pages,omitempty"`
}

func windowPage[T any](items []T) pageResult[T] {
	if len(items) > pageSize {
		return pageResult[T]{Items: items[:pageSize], More: true}
	}
	return pageResult[T]{Items: items}
}

func requestedPage(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 || page > 10000 {
		return 0, fail(400, "invalid_input")
	}
	return page, nil
}

func fixed(path string) func(*http.Request) (string, error) {
	return func(*http.Request) (string, error) { return path, nil }
}

func issuePage(r *http.Request) (string, error) {
	page, err := requestedPage(r)
	return fmt.Sprintf("/internal/issues?page=%d", page), err
}

func grantPath(r *http.Request) (string, error) {
	user, err := pathID(r, "user")
	return fmt.Sprintf("/internal/grants/%d", user), err
}

func resolvePath(r *http.Request) (string, error) {
	id := r.PathValue("operation")
	if !operationPattern.MatchString(id) {
		return "", fail(400, "invalid_input")
	}
	return "/internal/issues/" + id + "/resolve", nil
}

func readInput[T any](r *http.Request) (any, error) { return decodeBody[T](r) }

func issuerCall[T any](p *panel, method string, path func(*http.Request) (string, error),
	input func(*http.Request) (any, error)) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		route, err := path(r)
		if err != nil {
			return err
		}
		var body any
		if input != nil {
			body, err = input(r)
			if err != nil {
				return err
			}
		}
		res, err := p.issuer.open(r.Context(), method, route, a.tokens.Access, p.secret, "", body)
		if err != nil {
			return fail(503, "issue_service_unavailable")
		}
		defer res.Body.Close()
		dec := json.NewDecoder(http.MaxBytesReader(w, res.Body, 128<<10))
		if res.StatusCode >= 400 {
			var out struct {
				Code string `json:"code"`
			}
			if dec.Decode(&out) != nil || !strings.Contains("|invalid_input|invalid_grant|invalid_upstream|sign_in_required|not_allowed|official_confirmation_required|not_found|grant_exhausted|duplicate_operation|issue_unknown|issue_rejected|operation_unavailable|code_mismatch|totp_required|operation_rejected|rate_limited|service_unavailable|", "|"+out.Code+"|") {
				return fail(502, "issuer_error")
			}
			return fail(res.StatusCode, out.Code)
		}
		var out T
		if dec.Decode(&out) != nil {
			return fail(502, "issuer_error")
		}
		writeJSON(w, res.StatusCode, out)
		return nil
	}
}
