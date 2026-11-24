package testkit

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"sub2api-panel/internal/issuer"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/observe"
	"sub2api-panel/internal/server/system"
)

const (
	fixtureOrigin      = "https://panel.example.test"
	fixtureKey         = "fixture-issuer-only-key"
	fixtureLeak        = "fixture-not-for-client"
	fixturePassword    = "fixture-password"
	fixtureTOTPToken   = "fixture-totp-token"
	browserFixtureAddr = "127.0.0.1:28741"
	adminOrderPath     = "/admin/payment/orders"
	monitorRulesPath   = "/admin/channel-monitors"
	backupPath         = "/admin/backups"
)

func officialSuccess(w http.ResponseWriter, data any) {
	platform.WriteJSON(w, 200, map[string]any{"code": 0, "data": data, "message": fixtureLeak})
}

func officialPage(w http.ResponseWriter, item any) {
	officialSuccess(w, map[string]any{"items": []any{item}, "pages": 1, "unused": fixtureLeak})
}

type mockLogin struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type mockPaymentOrder struct {
	ID        int64      `json:"id"`
	Amount    float64    `json:"amount"`
	Paid      float64    `json:"pay_amount"`
	Currency  string     `json:"currency"`
	Method    string     `json:"payment_type"`
	Type      string     `json:"order_type"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
}

type mockAdminPaymentOrder struct {
	mockPaymentOrder
	UserID       int64   `json:"user_id"`
	UserEmail    string  `json:"user_email"`
	RefundAmount float64 `json:"refund_amount"`
}

type mockCheckoutPlan struct {
	platform.NamedItem
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Duration    int     `json:"validity_days"`
	Unit        string  `json:"validity_unit"`
}

type mockCheckoutMethod struct {
	Available bool    `json:"available"`
	Name      string  `json:"display_name"`
	Currency  string  `json:"currency"`
	Min       float64 `json:"single_min"`
	Max       float64 `json:"single_max"`
	Fee       float64 `json:"fee_rate"`
}

type mockPaymentCheckout struct {
	Plans           []mockCheckoutPlan            `json:"plans"`
	Methods         map[string]mockCheckoutMethod `json:"methods"`
	BalanceDisabled bool                          `json:"balance_disabled"`
}

type mockChannelRule struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	GroupName    string   `json:"group_name"`
	PrimaryModel string   `json:"primary_model"`
	Status       string   `json:"primary_status"`
	Latency      *int     `json:"primary_latency_ms"`
	Uptime       float64  `json:"availability_7d"`
	APIMode      string   `json:"api_mode"`
	Endpoint     string   `json:"endpoint"`
	Key          string   `json:"api_key_masked"`
	Models       []string `json:"extra_models"`
	Enabled      bool     `json:"enabled"`
	Interval     int      `json:"interval_seconds"`
	Jitter       int      `json:"jitter_seconds"`
	Check        string   `json:"check_mode"`
	Account      *int64   `json:"account_id"`
}

type mockChannelRuleInput struct {
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	APIMode  string   `json:"api_mode,omitempty"`
	Endpoint *string  `json:"endpoint,omitempty"`
	Key      string   `json:"api_key,omitempty"`
	Model    *string  `json:"primary_model,omitempty"`
	Models   []string `json:"extra_models"`
	Group    string   `json:"group_name"`
	Enabled  bool     `json:"enabled"`
	Interval int      `json:"interval_seconds"`
	Jitter   int      `json:"jitter_seconds"`
	Check    string   `json:"check_mode"`
	Account  *int64   `json:"account_id,omitempty"`
}

type mockMCPGrant struct {
	ID       string     `json:"id"`
	Client   string     `json:"client_name"`
	Scope    string     `json:"scope"`
	ReadOnly bool       `json:"read_only"`
	Created  time.Time  `json:"created_at"`
	Expires  time.Time  `json:"expires_at"`
	LastUsed *time.Time `json:"last_used_at"`
	Revoked  *time.Time `json:"revoked_at"`
	Invalid  bool       `json:"invalidated"`
}

type mockPlatformQuotaLimit struct {
	Platform string   `json:"platform"`
	Daily    *float64 `json:"daily_limit_usd"`
	Weekly   *float64 `json:"weekly_limit_usd"`
	Monthly  *float64 `json:"monthly_limit_usd"`
}

type mockPlatformQuota struct {
	mockPlatformQuotaLimit
	DailyUsage   float64 `json:"daily_usage_usd"`
	WeeklyUsage  float64 `json:"weekly_usage_usd"`
	MonthlyUsage float64 `json:"monthly_usage_usd"`
}

type mockUserAttributeValue struct {
	ID    int64  `json:"attribute_id"`
	Value string `json:"value"`
}

type mockCompositeRoute struct {
	ID             int64  `json:"id"`
	GroupID        int64  `json:"group_id"`
	PublicModel    string `json:"public_model"`
	MatchType      string `json:"match_type"`
	TargetPlatform string `json:"target_platform"`
	UpstreamModel  string `json:"upstream_model"`
	Endpoint       string `json:"endpoint"`
	Priority       int    `json:"priority"`
	Enabled        bool   `json:"enabled"`
	Notes          string `json:"notes"`
}

type mockModelPlazaPrice struct {
	Input *float64 `json:"input_price,omitempty"`
}

type mockModelPlazaModel struct {
	Name     string               `json:"name"`
	Pricing  *mockModelPlazaPrice `json:"pricing"`
	Official *mockModelPlazaPrice `json:"official_pricing"`
}

type mockModelPlazaGroup struct {
	platform.NamedItem
	Description  string                `json:"description"`
	Platform     string                `json:"platform"`
	Subscription string                `json:"subscription_type"`
	Rate         float64               `json:"rate_multiplier"`
	Models       []mockModelPlazaModel `json:"models"`
}

type mockAdminDashboardStats struct {
	Users          int64   `json:"total_users"`
	TodayNewUsers  int64   `json:"today_new_users"`
	ActiveUsers    int64   `json:"active_users"`
	Accounts       int64   `json:"total_accounts"`
	NormalAccounts int64   `json:"normal_accounts"`
	ErrorAccounts  int64   `json:"error_accounts"`
	Keys           int64   `json:"total_api_keys"`
	ActiveKeys     int64   `json:"active_api_keys"`
	TodayRequests  int64   `json:"today_requests"`
	TotalRequests  int64   `json:"total_requests"`
	TodayTokens    int64   `json:"today_tokens"`
	TotalTokens    int64   `json:"total_tokens"`
	TodayCost      float64 `json:"today_actual_cost"`
	TotalCost      float64 `json:"total_actual_cost"`
}

type mockOverviewUserRank struct {
	UserID   int64   `json:"user_id"`
	Email    string  `json:"email"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"actual_cost"`
}

type mockBackupStorage struct {
	Endpoint string `json:"endpoint"`
	Region   string `json:"region"`
	Bucket   string `json:"bucket"`
	Prefix   string `json:"prefix"`
}

type mockBackupSchedule struct {
	Enabled    bool   `json:"enabled"`
	Cron       string `json:"cron_expr"`
	RetainDays int    `json:"retain_days"`
	Retain     int    `json:"retain_count"`
}

type mockMailTemplate struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type fixtureUsageRow struct {
	id, user, key int64
	model         string
	created       time.Time
	observe.UsageTokens
	cost float64
}

type fixture struct {
	app               http.Handler
	panel             *core.Server
	issuer            *issuer.Service
	issues            atomic.Int64
	issueStatus       atomic.Int64
	invitation        atomic.Bool
	scheduled         atomic.Bool
	usageRows         []fixtureUsageRow
	previewKeys       map[int64][]map[string]any
	preview           *previewCatalog
	previewNotices    []map[string]any
	previewPromos     []map[string]any
	previewNoticeRead map[int64]map[int64]time.Time
	previewContentID  int64
	previewRebates    map[int64]float64
	previewPayment    map[int64]mockPaymentOrder
	previewOrders     int64
	previewProxies    []map[string]any
	previewRules      map[int64]mockChannelRule
	previewRuleID     int64
	previewOfficial   *system.OfficialPlugin
	previewPluginConf map[string]json.RawMessage
	previewRisk       *system.RiskConfigView
	previewRoutes     map[string]previewUsageRoute
	previewMu         sync.Mutex
	previewUserGroups map[int64][]int64
	previewBindings   map[int64]int64
	totpState         map[int64]bool
	profileNames      map[int64]string
	previewSettings   map[string]any
	previewSubSources map[string]map[string]any
	previewAdminKey   string
	previewAccessKeys []map[string]any
	previewAuditLogs  []system.AuditEntry
	previewMCPGrants  []mockMCPGrant
	previewQuotas     map[int64][]mockPlatformQuota
	previewAttrs      map[int64][]mockUserAttributeValue
	previewSubscribed atomic.Bool
	totpFeature       atomic.Bool
}
