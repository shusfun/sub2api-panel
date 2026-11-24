import {z} from "zod";
import {id, named} from "./common";

export const accountBalanceItem = z.object({currency: z.string(), balance: z.number()});
export const accountBalance = z.object({
  kind: z.enum(["account", "key_quota", "subscription"]).optional(),
  stale: z.boolean().optional(),
  currency: z.string(),
  balance: z.number(),
  available: z.boolean().optional(),
  balances: z.array(accountBalanceItem).optional(),
  fetched_at: z.number().optional(),
  billing: z.object({effective_multiplier: z.number(), observed_at: z.string(), stale: z.boolean().optional()}).optional(),
});

export const account = named.extend({
  proxy_fallback_origin_id: id.nullish(),
  base_url: z.string().optional(),
  model_mapping: z.record(z.string()).optional(),
  notes: z.string().optional().default(""),
  platform: z.string(),
  type: z.string().optional().default("apikey"),
  credentials_status: z.record(z.string(), z.boolean()).optional().default({}),
  proxy_id: id.nullable().optional().default(null),
  proxy_name: z.string().optional().default(""),
  group_ids: z.array(id).optional().default([]),
  groups: z.array(named).optional().default([]),
  concurrency: z.number().optional().default(0),
  current_concurrency: z.number().optional().default(0),
  activity_known: z.boolean().optional(),
  priority: z.number().optional().default(0),
  rate_multiplier: z.number().optional().default(1),
  status: z.string(),
  schedulable: z.boolean(),
  five_hours_used: z.number().nullable(),
  five_hours_reset_at: z.string().nullable().optional(),
  week_used: z.number().nullable(),
  week_reset_at: z.string().nullable().optional(),
  rate_limit_reset_at: z.string().nullable().optional(),
  overload_until: z.string().nullable().optional(),
  temp_unschedulable_until: z.string().nullable().optional(),
  last_used_at: z.string().nullable().optional(),
  expires_at: z.number().nullable().optional(),
  auto_pause_on_expired: z.boolean().optional().default(false),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
  today_requests: z.number().optional(),
  today_tokens: z.number().optional(),
  today_cost: z.number().optional(),
  today_errors: z.number().optional(),
  balance: accountBalance.optional(),
});

const quotaWindow = z.object({percent_used: z.number().nullable(), resets_at: z.string().nullable()}).nullable();
export const accountQuota = z.object({
  five_hours: quotaWindow,
  week: quotaWindow,
  sonnet_week: quotaWindow.optional(),
  gemini_daily: quotaWindow.optional(),
  gemini_pro_daily: quotaWindow.optional(),
  gemini_flash_daily: quotaWindow.optional(),
  thirty_day: quotaWindow.optional(),
  subscription_tier: z.string().optional(),
  needs_reauth: z.boolean().optional(),
  is_banned: z.boolean().optional(),
  needs_verify: z.boolean().optional(),
});

export const accountQuotaBatch = z.object({
  quotas: z.record(z.string(), accountQuota),
  unavailable: z.array(id),
});

export const accountStatsSummary = z.object({
  summary: z.object({total_requests: z.number(), total_tokens: z.number(), total_cost: z.number()}),
});

export const channelModelPricingSchema = z.object({
  id: z.number().optional(),
  platform: z.string(),
  models: z.array(z.string()),
  billing_mode: z.string(),
  input_price: z.number().nullable().optional(),
  output_price: z.number().nullable().optional(),
  cache_write_price: z.number().nullable().optional(),
  cache_read_price: z.number().nullable().optional(),
  image_input_price: z.number().nullable().optional(),
  image_output_price: z.number().nullable().optional(),
  per_request_price: z.number().nullable().optional(),
});

export const adminGroupRow = z.object({
  id,
  name: z.string(),
  description: z.string(),
  platform: z.string(),
  rate_multiplier: z.number(),
  rpm_limit: z.number(),
  is_exclusive: z.boolean(),
  status: z.string(),
  subscription_type: z.string(),
  daily_limit_usd: z.number().nullable(),
  weekly_limit_usd: z.number().nullable(),
  monthly_limit_usd: z.number().nullable(),
  long_context_pricing_enabled: z.boolean(),
  peak_rate_enabled: z.boolean(),
  peak_start: z.string(),
  peak_end: z.string(),
  peak_rate_multiplier: z.number(),
  fallback_group_id: id.nullable(),
  fallback_group_id_on_invalid_request: id.nullable(),
  claude_code_only: z.boolean(),
  require_oauth_only: z.boolean(),
  allow_messages_dispatch: z.boolean(),
  default_mapped_model: z.string(),
  max_reasoning_effort: z.string(),
  model_allowlist: z.object({enabled: z.boolean(), models: z.array(z.string())}),
  model_pricing: z.array(channelModelPricingSchema),
  account_count: z.number(),
  active_account_count: z.number(),
  rate_limited_account_count: z.number(),
  error_account_count: z.number(),
  concurrency_used: z.number(),
  concurrency_max: z.number(),
  rpm_used: z.number(),
  rpm_max: z.number(),
  health: z.string(),
  health_score: z.number().nullable(),
  today_cost: z.number(),
  total_cost: z.number(),
  sort_order: z.number(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const compositeRouteSchema = z.object({
  id,
  group_id: id,
  public_model: z.string(),
  match_type: z.string(),
  target_platform: z.string(),
  upstream_model: z.string(),
  endpoint: z.string(),
  priority: z.number(),
  enabled: z.boolean(),
  notes: z.string(),
});

export const channelRowSchema = z.object({
  id,
  name: z.string(),
  description: z.string(),
  status: z.string(),
  billing_model_source: z.string(),
  restrict_models: z.boolean(),
  group_ids: z.array(id),
  model_pricing: z.array(channelModelPricingSchema),
  model_mapping: z.record(z.string(), z.record(z.string(), z.string())),
  apply_pricing_to_account_stats: z.boolean(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const proxyRow = z.object({
  id,
  name: z.string(),
  protocol: z.enum(["http", "https", "socks5", "socks5h"]),
  host: z.string(),
  port: z.number(),
  has_auth: z.boolean(),
  status: z.string(),
  account_count: z.number(),
  latency_ms: z.number().nullable(),
  latency_status: z.string(),
  ip_address: z.string(),
  country: z.string(),
  country_code: z.string(),
  region: z.string(),
  city: z.string(),
  quality_status: z.string(),
  quality_score: z.number().nullable(),
  quality_grade: z.string(),
  quality_summary: z.string(),
  quality_checked: z.number(),
  expires_at: z.string().nullable(),
  fallback_mode: z.string(),
  backup_proxy_id: id.nullable(),
  expiry_warn_days: z.number(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const proxyTest = z.object({
  success: z.boolean(),
  message: z.string(),
  latency_ms: z.number().optional(),
  ip_address: z.string().optional(),
  city: z.string().optional(),
  region: z.string().optional(),
  country: z.string().optional(),
  country_code: z.string().optional(),
});

export const proxyQuality = z.object({
  proxy_id: id,
  score: z.number(),
  grade: z.string(),
  summary: z.string(),
  exit_ip: z.string().optional(),
  country: z.string().optional(),
  country_code: z.string().optional(),
  passed_count: z.number(),
  warn_count: z.number(),
  failed_count: z.number(),
  message: z.string().optional(),
});

export const probeCandidateSchema = z.object({
  model: z.string(),
  display_name: z.string(),
  family: z.string(),
  score: z.number(),
  probability: z.number().nullable().optional(),
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

export type ProbeReport = z.infer<typeof probeReportSchema>;

export const ephemeralPassSchema = z.object({
  base_url: z.string(),
  api_key: z.string(),
  expires_at: z.number(),
  quota_usd: z.string(),
  ikale_url: z.string(),
  inspector_url: z.string(),
});
