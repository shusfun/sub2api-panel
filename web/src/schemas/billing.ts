import {z} from "zod";
import {id} from "./common";

export const grantSchema = z.object({
  invites: z.number(),
  budget_cents: z.number(),
  amounts_cents: z.array(z.number()),
});

export const redeemCodeRow = z.object({
  id,
  masked: z.string(),
  type: z.string(),
  value: z.number(),
  status: z.string(),
  used_by: id.nullable(),
  used_email: z.string().optional().default(""),
  used_at: z.string().nullable().optional(),
  created_at: z.string(),
  expires_at: z.string().nullable().optional(),
  notes: z.string().optional().default(""),
  group_id: id.nullable().optional(),
  group_name: z.string().optional().default(""),
  validity_days: z.number().optional().default(0),
});

export const createdRedeemCode = z.object({
  id,
  code: z.string(),
  type: z.string(),
  value: z.number(),
  group_id: id.nullable().optional(),
  validity_days: z.number().optional(),
  expires_at: z.string().nullable().optional(),
});

export const issue = z.object({
  operation_id: z.string(),
  actor_id: id.optional(),
  kind: z.string(),
  amount_cents: z.number(),
  state: z.string(),
  code_id: id.nullable(),
  created_at: z.number(),
  quota_delta: z.number(),
  request_id: z.string().optional(),
});
