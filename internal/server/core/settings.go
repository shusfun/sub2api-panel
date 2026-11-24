package core

import (
	"context"
	"slices"
	"strings"

	"sub2api-panel/internal/platform"
)

type AgreementDocView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
}

type CustomEndpoint struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Description string `json:"description,omitempty"`
}

type CustomMenuItem struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	URL        string `json:"url"`
	Slug       string `json:"page_slug"`
	Icon       string `json:"icon_svg,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Sort       int    `json:"sort_order"`
	HideOpen   bool   `json:"hide_open_button,omitempty"`
}

type SettingsWire struct {
	HideCCSImport    bool               `json:"hide_ccs_import_button"`
	CustomEndpoints  []CustomEndpoint   `json:"custom_endpoints"`
	MenuItems        []CustomMenuItem   `json:"custom_menu_items"`
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
	AgreementDocs    []AgreementDocView `json:"login_agreement_documents"`
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

func (s *Server) Settings(ctx context.Context) (SettingsWire, error) {
	return platform.Upstream[SettingsWire](ctx, s.Upstream, "GET", "/settings/public", "", "", "", nil)
}

type CaptchaConfig struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Prefix   string `json:"prefix,omitempty"`
	Region   string `json:"region,omitempty"`
}

func CaptchaSettings(s SettingsWire) *CaptchaConfig {
	switch {
	case s.Turnstile:
		return &CaptchaConfig{Provider: "turnstile", Key: s.SiteKey}
	case s.Tencent:
		return &CaptchaConfig{Provider: "tencent", Key: s.TencentID}
	case s.Aliyun:
		return &CaptchaConfig{Provider: "aliyun", Key: s.Scene, Prefix: s.Prefix, Region: s.Region}
	default:
		return nil
	}
}

func ProjectCustomEndpoints(items []CustomEndpoint) []CustomEndpoint {
	out := make([]CustomEndpoint, 0)
	for _, item := range items {
		if _, ok := platform.ExternalLink(item.Endpoint); ok && item.Name != "" {
			out = append(out, item)
		}
	}
	return out
}

func ProjectCustomMenu(items []CustomMenuItem, visibility string) []CustomMenuItem {
	out := make([]CustomMenuItem, 0)
	seen := make(map[string]bool)
	for _, item := range items {
		if item.Visibility != visibility || strings.TrimSpace(item.Label) == "" {
			continue
		}
		if item.Slug == "" {
			item.Slug = item.ID
		}
		if _, ok := platform.ExternalLink(item.URL); !ok || !platform.CustomSlugPattern.MatchString(item.Slug) || seen[item.Slug] {
			continue
		}
		item.Visibility = ""
		if len(item.Icon) > 8192 || !strings.HasPrefix(strings.TrimSpace(item.Icon), "<svg") {
			item.Icon = ""
		}
		seen[item.Slug] = true
		out = append(out, item)
	}
	slices.SortStableFunc(out, func(a, b CustomMenuItem) int {
		if a.Sort < b.Sort {
			return -1
		}
		if a.Sort > b.Sort {
			return 1
		}
		return 0
	})
	return out
}
