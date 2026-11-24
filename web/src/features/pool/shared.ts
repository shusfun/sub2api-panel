import {z} from "zod";
import {account, adminGroupRow, channelModelPricingSchema, channelRowSchema, compositeRouteSchema, proxyRow} from "@/schemas";

export type AccountRow = z.infer<typeof account>;
export type GroupRow = z.infer<typeof adminGroupRow>;
export type ProxyRow = z.infer<typeof proxyRow>;
export type ChannelRow = z.infer<typeof channelRowSchema>;
export type CompositeRoute = z.infer<typeof compositeRouteSchema>;
export type PricingRule = z.infer<typeof channelModelPricingSchema>;

export const groupBriefSchema = z.object({
  id: z.number().int().positive(),
  name: z.string(),
  platform: z.string(),
  rate_multiplier: z.number(),
  status: z.string(),
  subscription_type: z.string(),
  is_exclusive: z.boolean(),
  account_count: z.number(),
  active_account_count: z.number(),
  rate_limited_account_count: z.number(),
  health: z.string(),
  health_score: z.number().nullable(),
});

export const statusPills = [
  {value: "", label: "全部", tone: "neutral"},
  {value: "active", label: "正常", tone: "ok"},
  {value: "error", label: "异常", tone: "bad"},
  {value: "inactive", label: "已停用", tone: "warn"},
] as const;
