import {z} from "zod";

export const affiliateUserRow = z.object({
  user_id: z.number(),
  email: z.string(),
  username: z.string(),
  aff_code: z.string(),
  aff_code_custom: z.boolean(),
  aff_rebate_rate_percent: z.number().nullish(),
  aff_count: z.number(),
});

export const affiliateInviteRow = z.object({
  inviter_id: z.number(),
  inviter_email: z.string(),
  inviter_username: z.string(),
  invitee_id: z.number(),
  invitee_email: z.string(),
  invitee_username: z.string(),
  aff_code: z.string(),
  total_rebate: z.number(),
  created_at: z.string(),
});

export const affiliateRebateRow = z.object({
  order_id: z.number().nullable(),
  out_trade_no: z.string(),
  inviter_id: z.number(),
  inviter_email: z.string(),
  inviter_username: z.string(),
  invitee_id: z.number().nullable(),
  invitee_email: z.string(),
  invitee_username: z.string(),
  order_amount: z.number().nullable(),
  pay_amount: z.number().nullable(),
  rebate_amount: z.number(),
  payment_type: z.string(),
  order_status: z.string(),
  created_at: z.string(),
});

export const affiliateTransferRow = z.object({
  ledger_id: z.number(),
  action: z.string(),
  user_id: z.number(),
  user_email: z.string(),
  username: z.string(),
  amount: z.number(),
  available_quota_after: z.number().nullish(),
  snapshot_available: z.boolean(),
  created_at: z.string(),
});

export const affiliateOverview = z.object({
  user_id: z.number(),
  email: z.string(),
  username: z.string(),
  aff_code: z.string(),
  rebate_rate_percent: z.number(),
  invited_count: z.number(),
  rebated_invitee_count: z.number(),
  available_quota: z.number(),
  history_quota: z.number(),
});

export const affiliateWithdrawResult = z.object({ledger_id: z.number(), amount: z.number(), available_quota_after: z.number()});
export const affiliateBatchResult = z.object({affected: z.number()});
