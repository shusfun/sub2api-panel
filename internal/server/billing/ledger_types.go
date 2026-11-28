package billing

import "sub2api-panel/internal/server/core"

// 卡片摘要类型定义在 core（账号列表经 core 取用，pool 不引用 billing），这里用别名保持 JSON 契约不变。
type (
	LedgerAlertBadge  = core.LedgerAlertBadge
	LedgerCardSummary = core.LedgerCardSummary
)

// 上游账本接口的响应结构（第 0 步定死，即字段白名单）。与 web/src/schemas/ledger.ts 一一对应。
// 收入与毛利字段使用指针 + omitempty：show_profit 关闭时不下发。凭据只回 configured 标记，不回明文。

type LedgerAccountCard struct {
	AccountID int64 `json:"account_id"`
	LedgerCardSummary
}

type LedgerTotals struct {
	BalanceUSD           float64  `json:"balance_usd"`
	UpstreamCostTodayUSD float64  `json:"upstream_cost_today_usd"`
	RevenueTodayUSD      *float64 `json:"revenue_today_usd,omitempty"`
	ProfitTodayUSD       *float64 `json:"profit_today_usd,omitempty"`
	AttentionAccounts    int      `json:"attention_accounts"`
}

// GET /api/admin/ledger/summary
type LedgerSummaryView struct {
	ShowProfit bool                `json:"show_profit"`
	Totals     LedgerTotals        `json:"totals"`
	Accounts   []LedgerAccountCard `json:"accounts"`
}

type LedgerEventView struct {
	ID         int64   `json:"id"`
	Kind       string  `json:"kind"`
	AmountUSD  float64 `json:"amount_usd"`
	OccurredAt string  `json:"occurred_at"`
	Source     string  `json:"source"`
	Status     string  `json:"status"`
	Note       string  `json:"note"`
}

type LedgerPointView struct {
	Bucket                      string   `json:"bucket"`
	Requests                    int64    `json:"requests"`
	RevenueUSD                  *float64 `json:"revenue_usd,omitempty"`
	ExpectedBalanceCostUSD      float64  `json:"expected_balance_cost_usd"`
	ExpectedSubscriptionCostUSD float64  `json:"expected_subscription_cost_usd"`
	ActualCostUSD               *float64 `json:"actual_cost_usd"`
	ActualSource                *string  `json:"actual_source"`
}

type LedgerConnectionView struct {
	Provider              *string `json:"provider"`
	BaseURL               string  `json:"base_url"`
	APIKeyConfigured      bool    `json:"api_key_configured"`
	AccessTokenConfigured bool    `json:"access_token_configured"`
	APIKeyFromAccount     bool    `json:"api_key_from_account"`
}

// GET /api/admin/ledger/accounts/{id}?granularity=hour|day&start&end
type LedgerAccountSeriesView struct {
	AccountID   int64                `json:"account_id"`
	Tracked     bool                 `json:"tracked"` // false：尚无账本记录（未接入或从未采样成功），events 恒为空
	Granularity string               `json:"granularity"`
	ShowProfit  bool                 `json:"show_profit"`
	Summary     LedgerCardSummary    `json:"summary"`
	Points      []LedgerPointView    `json:"points"`
	Events      []LedgerEventView    `json:"events"`
	Connection  LedgerConnectionView `json:"connection"`
}

// POST|PATCH /api/admin/ledger/accounts/{id}/events 的请求体；DELETE 作废不需要请求体。
type LedgerEventInput struct {
	Kind       string   `json:"kind"`
	AmountUSD  *float64 `json:"amount_usd"`
	OccurredAt string   `json:"occurred_at"`
	Status     string   `json:"status"`
	Note       string   `json:"note"`
}

// Kind 取值同 ledger_alert.kind；Severity 只用 info / warn / bad（与前端 ledgerSeverity 一致）。
// Value 是触发告警的数值：low_balance 为余额 USD，deviation 为偏差百分比，runway 为可撑小时。
type LedgerAlertView struct {
	ID          int64   `json:"id"`
	AccountID   *int64  `json:"account_id"`
	AccountName string  `json:"account_name"`
	Kind        string  `json:"kind"`
	Severity    string  `json:"severity"`
	Status      string  `json:"status"`
	Title       string  `json:"title"`
	OpenedAt    string  `json:"opened_at"`
	ResolvedAt  *string `json:"resolved_at"`
	Read        bool    `json:"read"`
	Value       float64 `json:"value"`
}

// GET /api/admin/ledger/alerts
type LedgerAlertListView struct {
	Items []LedgerAlertView `json:"items"`
	Total int               `json:"total"`
}

// GET /api/admin/ledger/alerts/unread-count
type LedgerUnreadView struct {
	Unread int `json:"unread"`
}

// GET|PUT /api/admin/ledger/settings；存于 runtime_settings 的 ledger_settings 键。
type LedgerSettings struct {
	ShowProfit          bool    `json:"show_profit"`
	DeviationPct        float64 `json:"deviation_pct"`
	DeviationMinUSD     float64 `json:"deviation_min_usd"`
	RunwayHours         float64 `json:"runway_hours"`
	FailureThreshold    int     `json:"failure_threshold"`
	HourRetentionDays   int     `json:"hour_retention_days"`
	SampleRetentionDays int     `json:"sample_retention_days"`
}

// DefaultLedgerSettings 是 ledger_settings 键缺失时的默认值。
func DefaultLedgerSettings() LedgerSettings {
	return LedgerSettings{DeviationPct: 10, DeviationMinUSD: 1, RunwayHours: 24, FailureThreshold: 3, HourRetentionDays: 365, SampleRetentionDays: 30}
}

// POST /api/admin/ledger/probe 的请求体与响应。
type LedgerProbeInput struct {
	URL string `json:"url"`
}

type LedgerProbeView struct {
	Type         string   `json:"type"`
	Origin       string   `json:"origin"`
	QuotaPerUnit *float64 `json:"quota_per_unit"`
	TokenPageURL string   `json:"token_page_url"`
	Scopes       []string `json:"scopes"`
}
