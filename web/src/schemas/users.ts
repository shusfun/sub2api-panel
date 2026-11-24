import {z} from "zod";
import {id, named} from "./common";

export const keyGroup = z.object({
  id,
  name: z.string(),
  platform: z.string(),
  rate_multiplier: z.number(),
  status: z.string(),
  subscription_type: z.string(),
  health: z.enum(["healthy", "warning", "critical", "unknown", "unavailable"]),
  health_score: z.number().nullable(),
});

export const keyRow = named.extend({
  masked: z.string(),
  status: z.string(),
  group_id: id.nullable(),
  ip_whitelist: z.array(z.string()),
  ip_blacklist: z.array(z.string()),
  quota: z.number(),
  quota_used: z.number(),
  expires_at: z.string().nullable(),
  last_used_at: z.string().nullable(),
  last_used_ip: z.string().nullable(),
  current_concurrency: z.number(),
  rate_limit_5h: z.number(),
  rate_limit_1d: z.number(),
  rate_limit_7d: z.number(),
  usage_5h: z.number(),
  usage_1d: z.number(),
  usage_7d: z.number(),
  today_actual_cost: z.number().nullable(),
  total_actual_cost: z.number().nullable(),
});

export const subscriptions = z.object({
  id,
  group: z.string(),
  status: z.string(),
  expires_at: z.string(),
  platform: z.string().optional(),
  is_exclusive: z.boolean().optional(),
});

export const userRow = z.object({
  id,
  email: z.string(),
  username: z.string().optional().default(""),
  role: z.string().optional().default("user"),
  status: z.string(),
  balance: z.number().nullable(),
  concurrency: z.number().optional().default(0),
  current_concurrency: z.number().optional().default(0),
  rpm_limit: z.number().optional().default(0),
  allowed_groups: z.array(id).optional().default([]),
  restrict_public_groups: z.boolean().optional().default(false),
  notes: z.string().optional().default(""),
  last_used_at: z.string().nullable().optional(),
  created_at: z.string().optional(),
});

export const usageUser = z.object({id, email: z.string()});

export const adminSubscriptionRow = z.object({
  id,
  user_id: z.number().int().nonnegative(),
  user_email: z.string().optional().default(""),
  group_id: z.number().int().nonnegative(),
  group: z.string(),
  platform: z.string().optional().default(""),
  status: z.string(),
  expires_at: z.string(),
  daily_usage_usd: z.number().optional().default(0),
  weekly_usage_usd: z.number().optional().default(0),
  monthly_usage_usd: z.number().optional().default(0),
  notes: z.string().optional().default(""),
});

export const balanceHistoryRow = z.object({
  id,
  type: z.string(),
  value: z.number(),
  status: z.string(),
  notes: z.string(),
  used_at: z.string().nullable(),
  created_at: z.string(),
  group: named.optional(),
});

export const userAttributeOption = z.object({
  value: z.string(),
  label: z.string(),
});

export const userAttributeDefinition = named.extend({
  key: z.string().optional().default(""),
  label: z.string().optional().default(""),
  type: z.string(),
  description: z.string().optional().default(""),
  required: z.boolean().optional().default(false),
  enabled: z.boolean().optional().default(true),
  sort_order: z.number().optional().default(0),
  options: z.array(userAttributeOption).nullable().optional(),
});
