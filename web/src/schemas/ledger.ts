import {z} from "zod";

// 上游账本契约（第 0 步定死）：与 internal/server/billing/ledger_types.go 一一对应。
// 收入与毛利字段只在 show_profit 开启时由后端返回，所以都是可选。

export const ledgerAlertKind = z.enum([
  "deviation", "multiplier_change", "runway", "sample_failure", "shared_wallet_suspect", "low_balance", "billing_missing",
]);
export const ledgerSeverity = z.enum(["info", "warn", "bad"]);
export const ledgerEventKind = z.enum(["recharge", "recharge_inferred", "manual_adjust", "refund", "quota_raised", "baseline_reset"]);
// 与 internal/server/billing/ledger_rollup.go 的 ledgerSourceKeyDelta / ledgerSourceBalance 一致。
export const ledgerActualSource = z.enum(["usage_delta", "balance_delta"]);
export const ledgerProvider = z.enum(["sub2api", "newapi", "oneapi", "deepseek", "kimi"]);

export const ledgerAlertBadge = z.object({kind: ledgerAlertKind, severity: ledgerSeverity, label: z.string()});

// 账号卡片 / 检查器用的摘要，合并进账号列表的 ledger 字段。
export const ledgerCardSummary = z.object({
  provider: ledgerProvider.nullable(),
  currency: z.string(),
  reconcile: z.boolean(),
  granularity: z.enum(["hour", "day"]),
  balance_usd: z.number().nullable(),
  runway_hours: z.number().nullable(),
  multiplier: z.number().nullable(),
  local_multiplier: z.number().nullable(),
  previous_multiplier: z.number().nullable(),
  actual_today_usd: z.number().nullable(),
  expected_today_usd: z.number(),
  deviation_usd: z.number().nullable(),
  deviation_pct: z.number().nullable(),
  revenue_today_usd: z.number().optional(),
  profit_today_usd: z.number().optional(),
  estimated: z.boolean(),
  sampled_at: z.string().nullable(),
  stale: z.boolean(),
  alerts: z.array(ledgerAlertBadge),
});

export const ledgerSummary = z.object({
  show_profit: z.boolean(),
  totals: z.object({
    balance_usd: z.number(),
    upstream_cost_today_usd: z.number(),
    revenue_today_usd: z.number().optional(),
    profit_today_usd: z.number().optional(),
    attention_accounts: z.number(),
  }),
  accounts: z.array(ledgerCardSummary.extend({account_id: z.number()})),
});

export const ledgerEvent = z.object({
  id: z.number(),
  kind: ledgerEventKind,
  amount_usd: z.number(),
  occurred_at: z.string(),
  source: z.enum(["log", "inferred", "manual"]),
  status: z.enum(["confirmed", "pending", "void"]),
  note: z.string(),
});

export const ledgerPoint = z.object({
  bucket: z.string(),
  requests: z.number(),
  revenue_usd: z.number().optional(),
  expected_balance_cost_usd: z.number(),
  expected_subscription_cost_usd: z.number(),
  actual_cost_usd: z.number().nullable(),
  actual_source: ledgerActualSource.nullable(),
});

export const ledgerAccountSeries = z.object({
  account_id: z.number(),
  tracked: z.boolean(),
  granularity: z.enum(["hour", "day"]),
  show_profit: z.boolean(),
  summary: ledgerCardSummary,
  points: z.array(ledgerPoint),
  events: z.array(ledgerEvent),
  connection: z.object({
    provider: ledgerProvider.nullable(),
    base_url: z.string(),
    api_key_configured: z.boolean(),
    access_token_configured: z.boolean(),
    api_key_from_account: z.boolean(),
  }),
});

export const ledgerAlert = z.object({
  id: z.number(),
  account_id: z.number().nullable(),
  account_name: z.string(),
  kind: ledgerAlertKind,
  severity: ledgerSeverity,
  status: z.enum(["open", "resolved"]),
  title: z.string(),
  opened_at: z.string(),
  resolved_at: z.string().nullable(),
  read: z.boolean(),
  value: z.number(),
});

export const ledgerAlertList = z.object({items: z.array(ledgerAlert), total: z.number()});
export const ledgerUnreadCount = z.object({unread: z.number()});

export const ledgerSettings = z.object({
  show_profit: z.boolean(),
  deviation_pct: z.number(),
  deviation_min_usd: z.number(),
  runway_hours: z.number(),
  failure_threshold: z.number(),
  hour_retention_days: z.number(),
  sample_retention_days: z.number(),
});

export const ledgerProbeResult = z.object({
  type: z.enum(["sub2api", "newapi", "oneapi", "unknown"]),
  origin: z.string(),
  quota_per_unit: z.number().nullable(),
  token_page_url: z.string(),
  scopes: z.array(z.string()),
});

export type LedgerCardSummary = z.infer<typeof ledgerCardSummary>;
export type LedgerSummary = z.infer<typeof ledgerSummary>;
export type LedgerEvent = z.infer<typeof ledgerEvent>;
export type LedgerPoint = z.infer<typeof ledgerPoint>;
export type LedgerAccountSeries = z.infer<typeof ledgerAccountSeries>;
export type LedgerAlert = z.infer<typeof ledgerAlert>;
export type LedgerSettings = z.infer<typeof ledgerSettings>;
export type LedgerProbeResult = z.infer<typeof ledgerProbeResult>;
