package core

// 账号卡片上的上游账本摘要。类型放在 core 里，是为了让 pool 不必引用 billing：
// billing.WireLedger 把取数函数注入 Server.LedgerCards，pool 只经由 core 读取。
// 字段白名单与 web/src/schemas/ledger.ts 一一对应。

type LedgerAlertBadge struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Label    string `json:"label"`
}

type LedgerCardSummary struct {
	Provider           *string            `json:"provider"`
	Currency           string             `json:"currency"`
	Reconcile          bool               `json:"reconcile"`
	Granularity        string             `json:"granularity"`
	BalanceUSD         *float64           `json:"balance_usd"`
	RunwayHours        *float64           `json:"runway_hours"`
	Multiplier         *float64           `json:"multiplier"`
	LocalMultiplier    *float64           `json:"local_multiplier"`
	PreviousMultiplier *float64           `json:"previous_multiplier"`
	ActualTodayUSD     *float64           `json:"actual_today_usd"`
	ExpectedTodayUSD   float64            `json:"expected_today_usd"`
	DeviationUSD       *float64           `json:"deviation_usd"`
	DeviationPct       *float64           `json:"deviation_pct"`
	RevenueTodayUSD    *float64           `json:"revenue_today_usd,omitempty"`
	ProfitTodayUSD     *float64           `json:"profit_today_usd,omitempty"`
	Estimated          bool               `json:"estimated"`
	SampledAt          *string            `json:"sampled_at"`
	Stale              bool               `json:"stale"`
	Alerts             []LedgerAlertBadge `json:"alerts"`
}
