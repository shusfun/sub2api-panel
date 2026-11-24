import { z } from "zod";

export const empty = z.object({});
export const page = <T extends z.ZodTypeAny>(row: T) => z.object({items: z.array(row), more: z.boolean(), pages: z.number().int().nonnegative().optional()});
const id = z.number().int().positive();
export const agreementDocSchema = z.object({id: z.string(), title: z.string(), content_md: z.string()});
export const customMenuItem = z.object({id: z.string(), label: z.string(), url: z.string().url(), page_slug: z.string(), icon_svg: z.string().optional(), sort_order: z.number(), hide_open_button: z.boolean().optional()});
export const configSchema = z.object({
  official: z.string().url(),
  site_name: z.string().optional().default("Sub2API"),
  site_subtitle: z.string().optional().default(""),
  site_logo: z.string().optional().default(""),
  doc_url: z.string().optional().default(""),
  contact_info: z.string().optional().default(""),
  home_content: z.string().optional().default(""),
  compact_home: z.boolean().optional().default(false),
  backend_mode: z.boolean().optional().default(false),
  registration: z.boolean().optional().default(false),
  email_verify: z.boolean().optional().default(false),
  promo_code: z.boolean().optional().default(false),
  password_reset: z.boolean().optional().default(false),
  passkey: z.boolean().optional().default(false),
  email_whitelist: z.array(z.string()).optional().default([]),
  api_base_url: z.string().optional(),
  hide_ccs_import_button: z.boolean().optional().default(false),
  custom_endpoints: z.array(z.object({name: z.string(), endpoint: z.string().url(), description: z.string().optional()})).optional().default([]),
  custom_menu_items: z.array(customMenuItem).optional().default([]),
  agreement: z.boolean(),
  agreement_mode: z.string().optional().default("checkbox"),
  agreement_revision: z.string().optional().default(""),
  agreement_docs: z.array(agreementDocSchema).optional().default([]),
  oauth_providers: z.array(z.string()).optional().default([]),
  oidc_provider_name: z.string().optional().default(""),
  affiliate: z.boolean(),
  model_plaza_enabled: z.boolean().default(false),
  model_plaza_require_auth: z.boolean().default(false),
  mcp_enabled: z.boolean().default(false),
  channel_monitor_enabled: z.boolean().default(false),
  invitation: z.boolean(),
  purchase_enabled: z.boolean().default(false),
  available_channels_enabled: z.boolean().default(false),
  patrol_home_visible: z.boolean().optional().default(true),
  patrol_user_visible: z.boolean().optional().default(true),
  guest_probe_enabled: z.boolean().optional().default(false),
  dev_accounts: z.array(z.object({label: z.string(), email: z.string().email(), password: z.string(), otp: z.string().optional()})).optional(),
  captcha: z.object({provider: z.enum(["turnstile", "tencent", "aliyun"]), key: z.string(), prefix: z.string().optional(),
    region: z.string().optional()}).optional(),
});
export const setupSchema = z.object({required: z.boolean()});
export const setupCandidatesSchema = z.object({candidates: z.array(z.object({url: z.string().url(), version: z.string()}))});
export type Config = z.infer<typeof configSchema>;
export const sessionSchema = z.object({
  admin: z.boolean(),
  csrf: z.string(),
  step_up_enabled: z.boolean().optional().default(false),
  totp_enabled: z.boolean().optional().default(false),
});
export type SessionInfo = z.infer<typeof sessionSchema>;
export const grantSchema = z.object({invites: z.number(), budget_cents: z.number(), amounts_cents: z.array(z.number())});
export const named = z.object({id, name: z.string()});
export const keyGroup = z.object({id, name: z.string(), platform: z.string(), rate_multiplier: z.number(), status: z.string(),
  subscription_type: z.string(), health: z.enum(["healthy", "warning", "critical", "unknown", "unavailable"]), health_score: z.number().nullable()});
export const keyRow = named.extend({masked: z.string(), status: z.string(), group_id: id.nullable(), ip_whitelist: z.array(z.string()), ip_blacklist: z.array(z.string()),
  quota: z.number(), quota_used: z.number(), expires_at: z.string().nullable(), last_used_at: z.string().nullable(), last_used_ip: z.string().nullable(),
  current_concurrency: z.number(), rate_limit_5h: z.number(), rate_limit_1d: z.number(), rate_limit_7d: z.number(), usage_5h: z.number(), usage_1d: z.number(), usage_7d: z.number(),
  today_actual_cost: z.number().nullable(), total_actual_cost: z.number().nullable()});
export const subscriptions = z.object({id, group: z.string(), status: z.string(), expires_at: z.string(), platform: z.string().optional(), is_exclusive: z.boolean().optional()});
export const usage = z.object({id, model: z.string(), created_at: z.string(), input_tokens: z.number(), output_tokens: z.number(), actual_cost: z.number().nullable()});
const tokenCount = z.number().int().nonnegative();
const usageTokens = z.object({input_tokens: tokenCount, output_tokens: tokenCount, cache_read_tokens: tokenCount, cache_creation_tokens: tokenCount});
export const usageRecord = usage.extend({created_at: z.string().datetime({offset: true}), key_name: z.string(),
  input_tokens: tokenCount, output_tokens: tokenCount, cache_read_tokens: tokenCount.nullable(), cache_creation_tokens: tokenCount.nullable(),
  api_key_id: id.optional(), user_id: id.optional(), user_email: z.string().optional(),
  group_id: id.nullable().optional(), group_name: z.string().optional(),
  account_id: id.nullable().optional(), account_name: z.string().optional(),
  duration_ms: z.number().nullable().optional(), first_token_ms: z.number().nullable().optional(), stream: z.boolean().nullable().optional()});
const usageDate = z.string().regex(/^\d{4}-\d{2}-\d{2}$/);
const usageModel = z.object({model: z.string(), requests: tokenCount, total_tokens: tokenCount, actual_cost: z.number().finite()});
const instant = z.string().datetime({offset: true});
const usageBucket = usageTokens.extend({start: instant, end: instant, pending: z.boolean().optional(), requests: tokenCount, actual_cost: z.number().finite()});
export const usageSummary = z.object({
  timezone: z.string(), start_date: usageDate, end_date: usageDate,
  totals: usageTokens.extend({requests: tokenCount, actual_cost: z.number().finite()}),
  days: z.array(usageTokens.extend({date: usageDate, requests: tokenCount, actual_cost: z.number().finite()})).max(90),
  granularity: z.enum(["hour", "day"]), buckets: z.array(usageBucket).max(175),
  ranking: z.array(usageModel),
  model_series: z.object({ready_days: tokenCount, total_days: tokenCount, unavailable: z.boolean().optional(),
    buckets: z.array(z.object({start: instant, models: z.array(usageModel)})).max(175)}),
  models: page(usageModel),
});
export const usageStreamEvent = z.object({
  type: z.enum(["snapshot", "delta", "heartbeat", "unavailable"]),
  cursor: z.string().optional(),
  snapshot: usageSummary.optional(),
  delta: usageSummary.partial().optional(), base: z.string().optional(),
  request_id: z.string().optional(), reason: z.enum(["not_allowed", "upstream_error"]).optional(),
  server_time: instant,
});
export const userRow = z.object({
  id, email: z.string(), username: z.string().optional().default(""), role: z.string().optional().default("user"),
  status: z.string(), balance: z.number().nullable(), concurrency: z.number().optional().default(0),
  current_concurrency: z.number().optional().default(0), rpm_limit: z.number().optional().default(0),
  allowed_groups: z.array(id).optional().default([]), restrict_public_groups: z.boolean().optional().default(false),
  notes: z.string().optional().default(""), last_used_at: z.string().nullable().optional(), created_at: z.string().optional(),
});
export const usageUser = z.object({id, email: z.string()});
export const adminSubscriptionRow = z.object({
  id, user_id: z.number().int().nonnegative(), user_email: z.string().optional().default(""),
  group_id: z.number().int().nonnegative(), group: z.string(), platform: z.string().optional().default(""),
  status: z.string(), expires_at: z.string(), daily_usage_usd: z.number().optional().default(0),
  weekly_usage_usd: z.number().optional().default(0), monthly_usage_usd: z.number().optional().default(0),
  notes: z.string().optional().default(""),
});
export const balanceHistoryRow = z.object({
  id, type: z.string(), value: z.number(), status: z.string(), notes: z.string(),
  used_at: z.string().nullable(), created_at: z.string(), group: named.optional(),
});
export const accountBalanceItem = z.object({currency: z.string(), balance: z.number()});
export const accountBalance = z.object({
  kind: z.enum(["account", "key_quota", "subscription"]).optional(), stale: z.boolean().optional(),
  currency: z.string(),
  balance: z.number(),
  available: z.boolean().optional(),
  balances: z.array(accountBalanceItem).optional(),
  fetched_at: z.number().optional(),
  billing: z.object({effective_multiplier: z.number(), observed_at: z.string(), stale: z.boolean().optional()}).optional(),
});
export const account = named.extend({
  proxy_fallback_origin_id: id.nullish(), base_url: z.string().optional(), model_mapping: z.record(z.string()).optional(),
  notes: z.string().optional().default(""), platform: z.string(), type: z.string().optional().default("apikey"),
  credentials_status: z.record(z.string(), z.boolean()).optional().default({}),
  proxy_id: id.nullable().optional().default(null), proxy_name: z.string().optional().default(""),
  group_ids: z.array(id).optional().default([]), groups: z.array(named).optional().default([]),
  concurrency: z.number().optional().default(0), current_concurrency: z.number().optional().default(0), activity_known: z.boolean().optional(),
  priority: z.number().optional().default(0), rate_multiplier: z.number().optional().default(1),
  status: z.string(), schedulable: z.boolean(),
  five_hours_used: z.number().nullable(), five_hours_reset_at: z.string().nullable().optional(),
  week_used: z.number().nullable(), week_reset_at: z.string().nullable().optional(),
  rate_limit_reset_at: z.string().nullable().optional(), overload_until: z.string().nullable().optional(),
  temp_unschedulable_until: z.string().nullable().optional(), last_used_at: z.string().nullable().optional(),
  expires_at: z.number().nullable().optional(), auto_pause_on_expired: z.boolean().optional().default(false),
  created_at: z.string().optional(), updated_at: z.string().optional(),
  today_requests: z.number().optional(), today_tokens: z.number().optional(),
  today_cost: z.number().optional(), today_errors: z.number().optional(),
  balance: accountBalance.optional(),
});
const quotaWindow = z.object({percent_used: z.number().nullable(), resets_at: z.string().nullable()}).nullable();
export const accountQuota = z.object({
  five_hours: quotaWindow, week: quotaWindow,
  sonnet_week: quotaWindow.optional(), gemini_daily: quotaWindow.optional(),
  gemini_pro_daily: quotaWindow.optional(), gemini_flash_daily: quotaWindow.optional(),
  thirty_day: quotaWindow.optional(),
  subscription_tier: z.string().optional(), needs_reauth: z.boolean().optional(),
  is_banned: z.boolean().optional(), needs_verify: z.boolean().optional(),
});
export const accountQuotaBatch = z.object({
  quotas: z.record(z.string(), accountQuota),
  unavailable: z.array(id),
});
export const accountStatsSummary = z.object({
  summary: z.object({total_requests: z.number(), total_tokens: z.number(), total_cost: z.number()}),
});
export const channelModelPricingSchema = z.object({
  id: z.number().optional(), platform: z.string(), models: z.array(z.string()), billing_mode: z.string(),
  input_price: z.number().nullable().optional(), output_price: z.number().nullable().optional(),
  cache_write_price: z.number().nullable().optional(), cache_read_price: z.number().nullable().optional(),
  image_input_price: z.number().nullable().optional(), image_output_price: z.number().nullable().optional(),
  per_request_price: z.number().nullable().optional(),
});
export const adminGroupRow = z.object({
  id, name: z.string(), description: z.string(), platform: z.string(), rate_multiplier: z.number(),
  rpm_limit: z.number(), is_exclusive: z.boolean(), status: z.string(), subscription_type: z.string(),
  daily_limit_usd: z.number().nullable(), weekly_limit_usd: z.number().nullable(), monthly_limit_usd: z.number().nullable(),
  long_context_pricing_enabled: z.boolean(), peak_rate_enabled: z.boolean(), peak_start: z.string(), peak_end: z.string(),
  peak_rate_multiplier: z.number(), fallback_group_id: id.nullable(), fallback_group_id_on_invalid_request: id.nullable(),
  claude_code_only: z.boolean(), require_oauth_only: z.boolean(), allow_messages_dispatch: z.boolean(),
  default_mapped_model: z.string(), max_reasoning_effort: z.string(),
  model_allowlist: z.object({enabled: z.boolean(), models: z.array(z.string())}),
  model_pricing: z.array(channelModelPricingSchema),
  account_count: z.number(), active_account_count: z.number(), rate_limited_account_count: z.number(),
  error_account_count: z.number(), concurrency_used: z.number(), concurrency_max: z.number(),
  rpm_used: z.number(), rpm_max: z.number(), health: z.string(), health_score: z.number().nullable(),
  today_cost: z.number(), total_cost: z.number(), sort_order: z.number(),
  created_at: z.string(), updated_at: z.string(),
});
export const compositeRouteSchema = z.object({
  id, group_id: id, public_model: z.string(), match_type: z.string(), target_platform: z.string(),
  upstream_model: z.string(), endpoint: z.string(), priority: z.number(), enabled: z.boolean(), notes: z.string(),
});
export const channelRowSchema = z.object({
  id, name: z.string(), description: z.string(), status: z.string(), billing_model_source: z.string(),
  restrict_models: z.boolean(), group_ids: z.array(id), model_pricing: z.array(channelModelPricingSchema),
  model_mapping: z.record(z.string(), z.record(z.string(), z.string())),
  apply_pricing_to_account_stats: z.boolean(), created_at: z.string(), updated_at: z.string(),
});
export const redeemCodeRow = z.object({
  id, masked: z.string(), type: z.string(), value: z.number(), status: z.string(),
  used_by: id.nullable(), used_email: z.string().optional().default(""), used_at: z.string().nullable().optional(),
  created_at: z.string(), expires_at: z.string().nullable().optional(), notes: z.string().optional().default(""),
  group_id: id.nullable().optional(), group_name: z.string().optional().default(""), validity_days: z.number().optional().default(0),
});
export const createdRedeemCode = z.object({
  id, code: z.string(), type: z.string(), value: z.number(), group_id: id.nullable().optional(),
  validity_days: z.number().optional(), expires_at: z.string().nullable().optional(),
});
const opsRateSchema = z.object({current: z.number(), peak: z.number(), avg: z.number()});
const opsPercentilesSchema = z.object({
  p50_ms: z.number().nullable(), p90_ms: z.number().nullable(), p95_ms: z.number().nullable(),
  p99_ms: z.number().nullable(), avg_ms: z.number().nullable(), max_ms: z.number().nullable(),
});
export const opsOverviewSchema = z.object({
  generated_at: z.string(),
  overview: z.object({
    start_time: z.string(), end_time: z.string(), platform: z.string(), group_id: id.nullable(),
    health_score: z.number().nullable(), success_count: z.number(), error_count_total: z.number(),
    business_limited_count: z.number(), error_count_sla: z.number(), request_count_total: z.number(),
    request_count_sla: z.number(), token_consumed: z.number(), sla: z.number(), error_rate: z.number(),
    upstream_error_rate: z.number(), upstream_error_count_excl_429_529: z.number(),
    upstream_429_count: z.number(), upstream_529_count: z.number(),
    qps: opsRateSchema, tps: opsRateSchema, duration: opsPercentilesSchema, ttft: opsPercentilesSchema,
    system_metrics: z.object({
      cpu_usage_percent: z.number().nullable(), memory_used_mb: z.number().nullable(),
      memory_total_mb: z.number().nullable(), memory_usage_percent: z.number().nullable(),
      db_ok: z.boolean().nullable(), redis_ok: z.boolean().nullable(),
      goroutine_count: z.number().nullable(), concurrency_queue_depth: z.number().nullable(),
      account_switch_count: z.number().nullable(),
    }).nullable(),
  }),
  throughput_points: z.array(z.object({
    bucket_start: z.string(), request_count: z.number(), token_consumed: z.number(),
    switch_count: z.number(), qps: z.number(), tps: z.number(),
  })),
  error_points: z.array(z.object({
    bucket_start: z.string(), error_count_total: z.number(), business_limited_count: z.number(),
    error_count_sla: z.number(), upstream_error_count_excl_429_529: z.number(),
    upstream_429_count: z.number(), upstream_529_count: z.number(),
  })),
  latency_buckets: z.array(z.object({range: z.string(), count: z.number()})),
  error_distribution: z.array(z.object({status_code: z.number(), total: z.number(), sla: z.number(), business_limited: z.number()})),
});
export const opsConcurrencySchema = z.object({
  enabled: z.boolean(), timestamp: z.string().optional(),
  qps: opsRateSchema, tps: opsRateSchema,
  platforms: z.array(z.object({
    platform: z.string(), current_in_use: z.number(), max_capacity: z.number(), load_percentage: z.number(),
    waiting_in_queue: z.number(), total_accounts: z.number(), available_count: z.number(),
    rate_limit_count: z.number(), error_count: z.number(),
  })),
  groups: z.array(z.object({
    group_id: id, group_name: z.string(), platform: z.string(), current_in_use: z.number(),
    max_capacity: z.number(), load_percentage: z.number(), waiting_in_queue: z.number(),
    total_accounts: z.number(), available_count: z.number(), rate_limit_count: z.number(), error_count: z.number(),
  })),
  accounts: z.array(z.object({
    account_id: id, account_name: z.string(), platform: z.string(), group_id: z.number(),
    group_name: z.string(), status: z.string(), current_in_use: z.number(), max_capacity: z.number(),
    load_percentage: z.number(), waiting_in_queue: z.number(), is_available: z.boolean(),
    is_rate_limited: z.boolean(), rate_limit_reset_at: z.string().optional(),
    is_overloaded: z.boolean(), overload_until: z.string().optional(), has_error: z.boolean(),
  })),
  users: z.array(z.object({
    user_id: id, user_email: z.string(), username: z.string(), current_in_use: z.number(),
    max_capacity: z.number(), load_percentage: z.number(), waiting_in_queue: z.number(),
  })),
});
export const opsRequestRowSchema = z.object({
  kind: z.string(), created_at: z.string(), request_id: z.string(), platform: z.string(), model: z.string(),
  duration_ms: z.number().nullable(), first_token_ms: z.number().nullable(), status_code: z.number().nullable(),
  error_id: z.number().nullable(), phase: z.string(), severity: z.string(),
  user_id: z.number().nullable(), api_key_id: z.number().nullable(), account_id: z.number().nullable(),
  group_id: z.number().nullable(), stream: z.boolean(),
});
export const opsErrorRowSchema = z.object({
  id, created_at: z.string(), phase: z.string(), type: z.string(), error_owner: z.string(), error_source: z.string(),
  severity: z.string(), status_code: z.number(), upstream_status_code: z.number().nullable().optional(),
  platform: z.string(), model: z.string(), requested_model: z.string().optional().default(""),
  upstream_model: z.string().optional().default(""), inbound_endpoint: z.string().optional().default(""),
  upstream_endpoint: z.string().optional().default(""), resolved: z.boolean(), resolved_at: z.string().nullable().optional(),
  is_business_limited: z.boolean(), client_request_id: z.string().optional().default(""), request_id: z.string(),
  user_id: z.number().nullable(), user_email: z.string(), api_key_id: z.number().nullable(), api_key_name: z.string(),
  account_id: z.number().nullable(), account_name: z.string(), group_id: z.number().nullable(), group_name: z.string(),
  stream: z.boolean(), auth_latency_ms: z.number().nullable(), routing_latency_ms: z.number().nullable(),
  upstream_latency_ms: z.number().nullable(), response_latency_ms: z.number().nullable(),
  time_to_first_token_ms: z.number().nullable(),
});
export const opsAlertRuleSchema = z.object({
  id: z.number().optional(), name: z.string(), description: z.string().optional().default(""),
  enabled: z.boolean(), metric_type: z.string(), operator: z.string(), threshold: z.number(),
  window_minutes: z.number(), sustained_minutes: z.number(), severity: z.string(),
  cooldown_minutes: z.number(), notify_email: z.boolean(), last_triggered_at: z.string().nullable().optional(),
});
export const opsAlertEventSchema = z.object({
  id, rule_id: z.number(), severity: z.string(), status: z.string(), title: z.string(), description: z.string(),
  metric_value: z.number().nullable(), threshold_value: z.number().nullable(),
  fired_at: z.string(), resolved_at: z.string().nullable(), email_sent: z.boolean(), created_at: z.string(),
});
export const opsSettingsBundleSchema = z.object({
  thresholds: z.object({
    sla_percent_min: z.number().nullable(), ttft_p99_ms_max: z.number().nullable(),
    request_error_rate_percent_max: z.number().nullable(), upstream_error_rate_percent_max: z.number().nullable(),
  }),
  email: z.object({
    alert: z.object({
      enabled: z.boolean(), recipients: z.array(z.string()), min_severity: z.string(),
      rate_limit_per_hour: z.number(), batching_window_seconds: z.number(), include_resolved_alerts: z.boolean(),
    }),
    report: z.object({
      enabled: z.boolean(), recipients: z.array(z.string()), daily_summary_enabled: z.boolean(),
      daily_summary_schedule: z.string(), weekly_summary_enabled: z.boolean(), weekly_summary_schedule: z.string(),
      error_digest_enabled: z.boolean(), error_digest_schedule: z.string(), error_digest_min_count: z.number(),
      account_health_enabled: z.boolean(), account_health_schedule: z.string(),
      account_health_error_rate_threshold: z.number(),
    }),
  }),
  logging: z.object({
    level: z.string(), persist_access_logs: z.boolean(), enable_sampling: z.boolean(),
    sampling_initial: z.number(), sampling_thereafter: z.number(), caller: z.boolean(),
    stacktrace_level: z.string(), retention_days: z.number(), request_retention_days: z.number(),
  }),
  advanced: z.object({
    data_retention: z.object({
      cleanup_enabled: z.boolean(), cleanup_schedule: z.string(), error_log_retention_days: z.number(),
      minute_metrics_retention_days: z.number(), hourly_metrics_retention_days: z.number(),
    }),
    aggregation: z.object({aggregation_enabled: z.boolean()}),
    openai_account_quota_auto_pause: z.object({default_threshold_5h: z.number(), default_threshold_7d: z.number()}),
    ignore_count_tokens_errors: z.boolean(), ignore_context_canceled: z.boolean(),
    ignore_no_available_accounts: z.boolean(), ignore_invalid_api_key_errors: z.boolean(),
    ignore_insufficient_balance_errors: z.boolean(), display_openai_token_stats: z.boolean(),
    display_alert_events: z.boolean(), auto_refresh_enabled: z.boolean(), auto_refresh_interval_seconds: z.number(),
  }),
});
export const adminOverview = z.object({
  users: z.object({total: z.number(), new_today: z.number(), active: z.number()}),
  accounts: z.object({total: z.number(), normal: z.number(), error: z.number(), rate_limited: z.number(), overloaded: z.number()}),
  keys: z.object({total: z.number(), active: z.number()}),
  codes: z.object({total: z.number(), active: z.number(), used: z.number(), expired: z.number()}),
  subscriptions: z.object({total: z.number(), active: z.number()}),
  announcements: z.number(), groups: z.number(), proxies: z.object({total: z.number(), active: z.number()}),
  traffic: z.object({
    today_requests: z.number(), total_requests: z.number(), today_tokens: z.number(), total_tokens: z.number(),
    today_cost: z.number(), total_cost: z.number(), average_duration_ms: z.number(), uptime: z.number(),
    rpm: z.number(), tpm: z.number(),
  }).optional(),
  trend: z.array(usageTokens.extend({date: usageDate, requests: tokenCount, actual_cost: z.number()})).optional().default([]),
  top_groups: z.array(z.object({group_id: id, group_name: z.string(), requests: z.number(), total_tokens: z.number(), actual_cost: z.number()})).optional().default([]),
  top_models: z.array(usageModel).optional().default([]),
  top_users: z.array(z.object({user_id: id, email: z.string(), requests: z.number(), tokens: z.number(), actual_cost: z.number()})).optional().default([]),
});
export const proxyRow = z.object({
  id, name: z.string(), protocol: z.enum(["http", "https", "socks5", "socks5h"]), host: z.string(), port: z.number(), has_auth: z.boolean(),
  status: z.string(), account_count: z.number(), latency_ms: z.number().nullable(), latency_status: z.string(), ip_address: z.string(),
  country: z.string(), country_code: z.string(), region: z.string(), city: z.string(), quality_status: z.string(), quality_score: z.number().nullable(),
  quality_grade: z.string(), quality_summary: z.string(), quality_checked: z.number(), expires_at: z.string().nullable(), fallback_mode: z.string(),
  backup_proxy_id: id.nullable(), expiry_warn_days: z.number(), created_at: z.string(), updated_at: z.string(),
});
export const proxyTest = z.object({success: z.boolean(), message: z.string(), latency_ms: z.number().optional(), ip_address: z.string().optional(),
  city: z.string().optional(), region: z.string().optional(), country: z.string().optional(), country_code: z.string().optional()});
export const proxyQuality = z.object({proxy_id: id, score: z.number(), grade: z.string(), summary: z.string(), exit_ip: z.string().optional(),
  country: z.string().optional(), country_code: z.string().optional(), passed_count: z.number(), warn_count: z.number(), failed_count: z.number(), message: z.string().optional()});
export const issue = z.object({operation_id: z.string(), actor_id: id.optional(), kind: z.string(), amount_cents: z.number(), state: z.string(),
  code_id: id.nullable(), created_at: z.number(), quota_delta: z.number(), request_id: z.string().optional()});
export const probeCandidateSchema = z.object({
  model: z.string(), display_name: z.string(), family: z.string(), score: z.number(), probability: z.number().nullable().optional(),
});
export const probeReportSchema = z.object({
  verdict: z.enum(["ok", "degraded", "failed"]),
  engine: z.string(),
  model: z.string(),
  effort: z.string().optional(),
  duration_ms: z.number(),
  ttft_ms: z.number(),
  input_tokens: z.number().optional().default(0),
  output_tokens: z.number().optional().default(0),
  reasoning_tokens: z.number().optional().default(0),
  cached_tokens: z.number().optional().default(0),
  estimated_micros: z.number().optional().default(0),
  confidence: z.number().nullable().optional(),
  predicted_model: z.string().optional(),
  summary: z.string(),
  reply: z.string().optional(),
  candidates: z.array(probeCandidateSchema).optional(),
  inspector_score: z.number().nullable().optional(),
  inspector_grade: z.string().optional(),
});
export const ephemeralPassSchema = z.object({
  base_url: z.string(), api_key: z.string(), expires_at: z.number(), quota_usd: z.string(), ikale_url: z.string(), inspector_url: z.string(),
});
const probeStreamFrameSchema = z.object({
  type: z.enum(["status", "delta", "complete", "error"]),
  message: z.string().optional(),
  delta: z.string().optional(),
  report: probeReportSchema.optional(),
});
export const patrolBarSchema = z.object({
  id: z.number(), verdict: z.enum(["ok", "degraded", "failed", "skipped_idle"]),
  duration_ms: z.number(), tokens: z.number(), created_at: z.string(), summary: z.string().optional(), confidence: z.number().nullable().optional(),
});
export const patrolGroupBoardSchema = z.object({
  group_id: id, group_name: z.string(), platform: z.string(), rate_multiplier: z.number(),
  engine: z.string(), model: z.string(), effort: z.string().optional(), enabled: z.boolean(), public_visible: z.boolean(),
  latest_verdict: z.string(), healthy_rate: z.number().nullable(), valid_count: z.number(), healthy_count: z.number(),
  latest_duration_ms: z.number().nullable(), latest_tokens: z.number().nullable(), latest_at: z.string().optional(),
  latest_run_id: z.number().nullable().optional(), next_check_at: z.number(),
  min_active_users: z.number(), window_minutes: z.number(), interval_minutes: z.number(),
  bars: z.array(patrolBarSchema),
});
export const patrolBoardSchema = z.object({
  enabled: z.boolean(), guest_check_enabled: z.boolean(), guest_default_model: z.string().optional(),
  synced_at: z.string(), next_countdown_sec: z.number(),
  group_count: z.number(), healthy_groups: z.number(), degraded_groups: z.number(), idle_groups: z.number(),
  overall_healthy_rate: z.number().nullable(), total_valid_runs: z.number(), total_healthy_runs: z.number(), total_failed_runs: z.number(),
  avg_duration_ms: z.number().nullable(), recent_tokens: z.number(), idle_skipped_today: z.number(), estimated_saved_tokens: z.number(),
  groups: z.array(patrolGroupBoardSchema),
});
export const patrolRunDetailSchema = z.object({
  id: z.number(), group_name: z.string(), trigger: z.string(), verdict: z.string(), engine: z.string(),
  model: z.string(), effort: z.string().optional(), active_users: z.number(), window_requests: z.number(),
  duration_ms: z.number(), ttft_ms: z.number(), input_tokens: z.number(), output_tokens: z.number(),
  reasoning_tokens: z.number(), cached_tokens: z.number(), estimated_micros: z.number(),
  confidence: z.number().nullable().optional(), predicted_model: z.string().optional(),
  summary: z.string(), reply_excerpt: z.string().optional(), created_at: z.string(),
});
export const patrolPolicySchema = z.object({
  group_id: id, group_name: z.string(), platform: z.string(), rate_multiplier: z.number(),
  enabled: z.boolean(), public_visible: z.boolean(), engine: z.string(), template_id: z.number().nullable().optional(),
  model: z.string(), protocol: z.string(), effort: z.string(), inspector_preset: z.string(), has_patrol_key: z.boolean(),
  window_minutes: z.number(), min_active_users: z.number(), min_window_requests: z.number(), peak_active_users: z.number(),
  interval_minutes: z.number(), peak_interval_minutes: z.number(), recheck_minutes: z.number(),
  daily_token_limit: z.number(), daily_budget_cents: z.number(), next_check_at: z.number(),
});
export const probeTemplateSchema = z.object({
  id: z.number(), name: z.string(), protocol: z.string(), system_prompt: z.string(), user_prompt: z.string(),
  effort: z.string(), stream: z.boolean(), max_tokens: z.number(), match_kind: z.string(), expected: z.string(),
  min_reasoning_tokens: z.number(), max_ttft_ms: z.number(),
});
export const builtinPluginSettingSchema = z.object({
  key: z.string(), title: z.string(), description: z.string().optional(), kind: z.enum(["bool", "int", "string"]),
  default: z.unknown(), min: z.number().optional(), max: z.number().optional(), enum: z.array(z.string()).optional(),
});
export const builtinPluginStateSchema = z.object({
  id: z.string(), name: z.string(), description: z.string(), category: z.string(), enabled: z.boolean(),
  status: z.enum(["off", "active", "failed"]), reason: z.string().optional(),
  settings: z.array(builtinPluginSettingSchema), config: z.record(z.string(), z.unknown()),
});
export const extPluginViewSchema = z.object({
  id: z.string(), name: z.string(), publisher: z.string(), summary: z.string(), category: z.string(),
  license: z.string(), homepage: z.string().optional(), repo: z.string().optional(), version: z.string().optional(),
  permissions: z.array(z.string()).default([]),
  option_specs: z.array(z.object({key: z.string(), title: z.string(), placeholder: z.string().optional(), default: z.string().optional()})).default([]),
  probes: z.array(z.object({id: z.string(), title: z.string(), protocol: z.string(), presets: z.array(z.string()).optional()})).default([]),
  page: z.object({title: z.string(), file: z.string().optional(), url: z.string().optional(), mount: z.string()}).nullable().optional(),
  actions: z.array(z.object({id: z.string(), target: z.string(), title: z.string()})).default([]),
  tools: z.array(z.object({name: z.string(), description: z.string()})).default([]),
  official_seed: z.boolean(), installed: z.boolean(), enabled: z.boolean(), ready: z.boolean(),
  status_note: z.string().optional(), source: z.string().optional(), options: z.record(z.string(), z.string()),
});
export const extMarketOverviewSchema = z.object({
  market_url: z.string(), plugins: z.array(extPluginViewSchema),
});
export const adminPluginsBundleSchema = z.object({
  builtin: z.array(builtinPluginStateSchema), market: extMarketOverviewSchema, upstream_enabled: z.boolean(),
});
export const mountedPluginsSchema = z.object({
  items: z.array(z.object({plugin_id: z.string(), title: z.string(), mount: z.string()})),
});
const testEvent = z.object({state: z.enum(["running", "passed", "failed"])});
const errorSchema = z.object({
  code: z.string(),
  hint: z.string(),
  request_id: z.string(),
  fields: z.record(z.string(), z.string()).optional(),
});

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly requestId = "",
    hint?: string,
    readonly fields: Record<string, string> = {},
  ) {
    super(hint ?? (status === 401 ? "会话已结束，请重新登录。" : "服务暂不可用，请稍后再试。"));
  }
}

export function describeError(error: unknown) {
  if (error instanceof ApiError) return error.requestId ? error.message + " 请求编号 " + error.requestId : error.message;
  return error instanceof Error ? error.message : "操作未能完成。";
}

let csrf = "";
let privateKey: CryptoKey | undefined;

export function setSession(session?: SessionInfo) { csrf = session?.csrf ?? ""; }

function openKeys(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open("sub2api-panel", 1);
    request.onupgradeneeded = () => request.result.createObjectStore("keys");
    request.onerror = () => reject(new Error("浏览器无法保存会话密钥。"));
    request.onsuccess = () => resolve(request.result);
  });
}

async function storedKey(value?: CryptoKey) {
  const db = await openKeys();
  try {
    return await new Promise<CryptoKey | undefined>((resolve, reject) => {
      const tx = db.transaction("keys", value ? "readwrite" : "readonly");
      const store = tx.objectStore("keys");
      const request = value ? store.put(value, "session") : store.get("session");
      tx.oncomplete = () => {
        const result: unknown = request.result;
        resolve(result instanceof CryptoKey ? result : undefined);
      };
      tx.onabort = () => reject(new Error("浏览器无法保存会话密钥。"));
    });
  } finally {
    db.close();
  }
}

const base64 = (bytes: ArrayBuffer) => btoa(String.fromCharCode(...new Uint8Array(bytes)));
const base64URL = (bytes: ArrayBuffer) => base64(bytes).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");

export async function newBrowserKey() {
  const key = await crypto.subtle.generateKey({name: "ECDSA", namedCurve: "P-256"}, false, ["sign", "verify"]);
  return {private: key.privateKey, public: base64URL(await crypto.subtle.exportKey("raw", key.publicKey))};
}

export async function saveBrowserKey(key: CryptoKey) {
  await storedKey(key);
  privateKey = key;
}

async function sign(headers: Headers, key: CryptoKey, method: string, url: URL, body?: string | Blob) {
  const bytes = new TextEncoder();
  const input = body instanceof Blob ? await body.arrayBuffer() : bytes.encode(body ?? "");
  const digest = "sha-256=:" + base64(await crypto.subtle.digest("SHA-256", input)) + ":";
  const nonce = [...crypto.getRandomValues(new Uint8Array(16))].map(value => value.toString(16).padStart(2, "0")).join("");
  const params = '("@method" "@target-uri" "content-digest");created=' + Math.floor(Date.now() / 1000) + ';nonce="' + nonce + '"';
  const fields = ['"@method": ' + method, '"@target-uri": ' + url.href, '"content-digest": ' + digest, '"@signature-params": ' + params];
  const signature = await crypto.subtle.sign({name: "ECDSA", hash: "SHA-256"}, key, bytes.encode(fields.join("\n")));
  headers.set("Signature-Input", "panel=" + params);
  headers.set("Signature", "panel=:" + base64(signature) + ":");
  headers.set("Content-Digest", digest);
}

const traceHex = (size: number) => [...crypto.getRandomValues(new Uint8Array(size))].map(value => value.toString(16).padStart(2, "0")).join("");
export const operationTrace = () => traceHex(16);

let currentActionTrace = "";
export async function withActionTrace<T>(trace: string, run: () => Promise<T>): Promise<T> {
  const prev = currentActionTrace;
  currentActionTrace = trace;
  try {
    return await run();
  } finally {
    currentActionTrace = prev;
  }
}

async function request(path: string, method: string, input?: object, proof?: CryptoKey | false, signal?: AbortSignal, trace = currentActionTrace || traceHex(16)) {
  const url = new URL("/api" + path, location.origin);
  const headers = new Headers();
  headers.set("traceparent", "00-" + trace + "-" + traceHex(8) + "-01");
  const body = input instanceof Blob ? input : input === undefined ? undefined : JSON.stringify(input);
  if (body !== undefined) headers.set("Content-Type", body instanceof Blob ? "application/octet-stream" : "application/json");
  if (proof !== false) {
    const key = proof ?? privateKey ?? await storedKey();
    if (!key) throw new ApiError(401, "sign_in_required");
    if (!proof) privateKey = key;
    await sign(headers, key, method, url, body);
  }
  if (csrf && method !== "GET") headers.set("X-CSRF-Token", csrf);
  let response: Response;
  try {
    response = await fetch(url, {method, headers, body, signal, redirect: "error"});
  } catch {
    if (signal?.aborted) throw signal.reason;
    throw new Error("连接失败，请检查网络后重试。");
  }
  if (!response.ok) {
    const problem = errorSchema.safeParse(await response.json().catch(() => null));
    const error = new ApiError(
      response.status,
      problem.data?.code ?? "service_unavailable",
      problem.data?.request_id,
      problem.data?.hint,
      problem.data?.fields ?? {},
    );
    if (csrf && error.status === 401 && ["sign_in_required", "invalid_proof"].includes(error.code)) {
      window.dispatchEvent(new Event("panel-session-ended"));
    }
    throw error;
  }
  return response;
}

export async function api<Schema extends z.ZodTypeAny>(path: string, schema: Schema, method = "GET", input?: object, proof?: CryptoKey | false, trace?: string): Promise<z.output<Schema>> {
  const response = await request(path, method, input, proof, undefined, trace);
  const parsed = schema.safeParse(await response.json().catch(() => null));
  if (!parsed.success) throw new ApiError(502, "invalid_response", response.headers.get("X-Request-ID") ?? "");
  return parsed.data;
}

export async function testAccount(id: number, model: string, progress: (state: string) => void, trace?: string) {
  const response = await request("/admin/accounts/" + id + "/test", "POST", model ? {model_id: model} : {}, undefined, undefined, trace);
  if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
    throw new Error("测活结果格式错误。");
  }
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
  try {
    let buffered = "";
    for (;;) {
      const {done, value} = await reader.read();
      if (done) throw new Error("测活连接提前结束。");
      buffered += value;
      let end = buffered.indexOf("\n\n");
      while (end !== -1) {
        const message = buffered.slice(0, end);
        buffered = buffered.slice(end + 2);
        end = buffered.indexOf("\n\n");
        if (!message.startsWith("data: ")) continue;
        const event = testEvent.parse(JSON.parse(message.slice(6)));
        progress(event.state);
        if (event.state !== "running") return;
      }
    }
  } finally {
    await reader.cancel();
  }
}

export type ProbeReport = z.infer<typeof probeReportSchema>;

export async function streamProbe(
  path: string,
  payload: Record<string, unknown>,
  callbacks: {
    onStatus?: (msg: string) => void;
    onDelta?: (chunk: string) => void;
  },
): Promise<ProbeReport> {
  const response = await request(path, "POST", payload);
  if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
    throw new Error("探测响应格式错误。");
  }
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
  try {
    let buffered = "";
    for (;;) {
      const {done, value} = await reader.read();
      if (done) break;
      buffered += value;
      let end = buffered.indexOf("\n\n");
      while (end !== -1) {
        const message = buffered.slice(0, end);
        buffered = buffered.slice(end + 2);
        end = buffered.indexOf("\n\n");
        if (!message.startsWith("data: ")) continue;
        const frame = probeStreamFrameSchema.parse(JSON.parse(message.slice(6)));
        if (frame.type === "status" && frame.message) callbacks.onStatus?.(frame.message);
        else if (frame.type === "delta" && frame.delta) callbacks.onDelta?.(frame.delta);
        else if (frame.type === "error") throw new Error(frame.message ?? "探测未完成。");
        else if (frame.type === "complete" && frame.report) return frame.report;
      }
    }
    throw new Error("探测流提前中断。");
  } finally {
    await reader.cancel();
  }
}

export type UsageStreamStatus = "connecting" | "live" | "reconnecting" | "paused" | "denied";
type UsageSnapshot = z.infer<typeof usageSummary>;

function waitForStream(signal: AbortSignal, untilVisible: boolean) {
  return new Promise<void>(resolve => {
    const finish = () => {
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", visible);
      signal.removeEventListener("abort", finish);
      resolve();
    };
    const visible = () => { if (!document.hidden) finish(); };
    const timer = untilVisible ? undefined : window.setTimeout(finish, 1500);
    if (untilVisible) document.addEventListener("visibilitychange", visible);
    signal.addEventListener("abort", finish, {once: true});
    if (signal.aborted || (untilVisible && !document.hidden)) finish();
  });
}

export async function streamUsage(path: string,
  onSnapshot: (value: UsageSnapshot, cursor: string, requestID?: string) => void,
  onStatus: (status: UsageStreamStatus) => void, signal?: AbortSignal) {
  const lifetime = signal ?? new AbortController().signal;
  const trace = traceHex(16);
  let cursor = "";
  let snapshot: UsageSnapshot | undefined;
  while (!lifetime.aborted) {
    if (document.hidden) {
      onStatus("paused");
      await waitForStream(lifetime, true);
      continue;
    }
    onStatus(cursor ? "reconnecting" : "connecting");
    const connection = new AbortController();
    const abort = () => connection.abort();
    const pause = () => { if (document.hidden) abort(); };
    document.addEventListener("visibilitychange", pause);
    lifetime.addEventListener("abort", abort, {once: true});
    let watchdog: number | undefined;
    const renew = () => {
      window.clearTimeout(watchdog);
      watchdog = window.setTimeout(abort, 35000);
    };
    renew();
    try {
      const separator = path.includes("?") ? "&" : "?";
      const response = await request(path + (cursor ? separator + "cursor=" + encodeURIComponent(cursor) : ""), "GET", undefined, undefined, connection.signal, trace);
      if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
        throw new ApiError(502, "invalid_response", response.headers.get("X-Request-ID") ?? "");
      }
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
      let buffered = "";
      let available = false;
      try {
        while (!connection.signal.aborted) {
          const {done, value} = await reader.read();
          if (done) break;
          if (connection.signal.aborted) break;
          buffered += value;
          if (buffered.length > 8 * 1024 * 1024) throw new ApiError(502, "invalid_response");
          let end: number;
          while ((end = buffered.indexOf("\n\n")) !== -1) {
            const data = buffered.slice(0, end).split("\n").find(line => line.startsWith("data: "))?.slice(6);
            buffered = buffered.slice(end + 2);
            if (!data) continue;
            const parsed = usageStreamEvent.parse(JSON.parse(data));
            renew();
            if (parsed.type === "unavailable") {
              available = false;
              if (parsed.reason === "not_allowed") { onStatus("denied"); return; }
              onStatus("reconnecting");
              continue;
            }
            if (parsed.type === "heartbeat") {
              if (parsed.cursor && parsed.cursor === cursor && snapshot) available = true;
              if (available) onStatus("live");
              continue;
            }
            if (!parsed.cursor) throw new ApiError(502, "invalid_response");
            if (parsed.type === "snapshot" && parsed.snapshot) snapshot = parsed.snapshot;
            else if (parsed.type === "delta" && parsed.base === cursor && snapshot && parsed.delta) snapshot = {...snapshot, ...parsed.delta};
            else {
              cursor = "";
              snapshot = undefined;
              throw new ApiError(502, "invalid_response");
            }
            cursor = parsed.cursor;
            available = true;
            onStatus("live");
            onSnapshot(snapshot, cursor, parsed.request_id);
          }
        }
      } finally {
        await reader.cancel().catch(() => {});
      }
    } catch (error) {
      if (lifetime.aborted) return;
      if (error instanceof ApiError && [400, 401, 403, 404].includes(error.status)) {
        onStatus("denied");
        return;
      }
    } finally {
      window.clearTimeout(watchdog);
      document.removeEventListener("visibilitychange", pause);
      lifetime.removeEventListener("abort", abort);
      abort();
    }
    if (!document.hidden && !lifetime.aborted) {
      onStatus("reconnecting");
      await waitForStream(lifetime, false);
    }
  }
}

export const opsLiveSchema = opsConcurrencySchema.extend({charts: opsOverviewSchema});

export function streamOps(path: string, onSnapshot: (value: z.infer<typeof opsLiveSchema>) => void, onStatus: (status: UsageStreamStatus) => void, signal?: AbortSignal) {
  return streamSnapshot(path, opsLiveSchema, onSnapshot, onStatus, signal);
}

export async function streamSnapshot<Schema extends z.ZodTypeAny>(
  path: string,
  snapshot: Schema,
  onSnapshot: (value: z.output<Schema>) => void,
  onStatus: (status: UsageStreamStatus) => void,
  signal?: AbortSignal,
) {
  const eventSchema = z.object({
    type: z.enum(["snapshot", "heartbeat", "unavailable"]),
    snapshot: snapshot.optional(),
    reason: z.string().optional(),
    server_time: instant,
  });
  const lifetime = signal ?? new AbortController().signal;
  const trace = traceHex(16);
  while (!lifetime.aborted) {
    if (document.hidden) {
      onStatus("paused");
      await waitForStream(lifetime, true);
      continue;
    }
    onStatus("connecting");
    const connection = new AbortController();
    const abort = () => connection.abort();
    const pause = () => { if (document.hidden) abort(); };
    document.addEventListener("visibilitychange", pause);
    lifetime.addEventListener("abort", abort, {once: true});
    let watchdog: number | undefined;
    const renew = () => {
      window.clearTimeout(watchdog);
      watchdog = window.setTimeout(abort, 35000);
    };
    renew();
    try {
      const response = await request(path, "GET", undefined, undefined, connection.signal, trace);
      if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
        throw new ApiError(502, "invalid_response");
      }
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
      let buffered = "";
      try {
        while (!connection.signal.aborted) {
          const {done, value} = await reader.read();
          if (done || connection.signal.aborted) break;
          buffered += value;
          let end: number;
          while ((end = buffered.indexOf("\n\n")) !== -1) {
            const data = buffered.slice(0, end).split("\n").find(line => line.startsWith("data: "))?.slice(6);
            buffered = buffered.slice(end + 2);
            if (!data) continue;
            const parsed = eventSchema.parse(JSON.parse(data));
            renew();
            if (parsed.type === "unavailable") {
              if (parsed.reason === "not_allowed") { onStatus("denied"); return; }
              onStatus("reconnecting");
              continue;
            }
            if (parsed.type === "snapshot" && parsed.snapshot) {
              onStatus("live");
              onSnapshot(parsed.snapshot);
            }
          }
        }
      } finally {
        await reader.cancel().catch(() => {});
      }
    } catch (error) {
      if (lifetime.aborted) return;
      if (error instanceof ApiError && [400, 401, 403, 404].includes(error.status)) {
        onStatus("denied");
        return;
      }
    } finally {
      window.clearTimeout(watchdog);
      document.removeEventListener("visibilitychange", pause);
      lifetime.removeEventListener("abort", abort);
      abort();
    }
    if (!document.hidden && !lifetime.aborted) {
      onStatus("reconnecting");
      await waitForStream(lifetime, false);
    }
  }
}
