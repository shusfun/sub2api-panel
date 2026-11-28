import {z} from "zod";
import {date, money} from "@/lib/format";
import {navigateTo} from "@/app/route";
import {Badge, EntityLink, type Column} from "@/ui";
import {affiliateInviteRow, affiliateRebateRow, affiliateTransferRow, affiliateUserRow} from "@/schemas";

export type AffUser = z.infer<typeof affiliateUserRow>;
export type AffInvite = z.infer<typeof affiliateInviteRow>;
export type AffRebate = z.infer<typeof affiliateRebateRow>;
export type AffTransfer = z.infer<typeof affiliateTransferRow>;

const stamp = (value: string) => <span className="code-mono text-fine">{date(value)}</span>;

function person(id: number, email: string, username: string) {
  return <EntityLink kind="user" id={id} label={email || username || `用户 #${id}`} onClick={() => navigateTo("users", {inspect: id, tab: "affiliate"})} />;
}

export const userColumns: Column<AffUser>[] = [
  {id: "user", title: "专属用户", render: row => person(row.user_id, row.email, row.username)},
  {id: "code", title: "邀请码", render: row => <span className="code-mono">{row.aff_code}</span>},
  {id: "custom", title: "类型", align: "center", hideWhenNarrow: true, render: row => <Badge variant="soft" tone={row.aff_code_custom ? "info" : "neutral"}>{row.aff_code_custom ? "自定义码" : "系统码"}</Badge>},
  {id: "rate", title: "专属返利比例", align: "number", render: row => row.aff_rebate_rate_percent != null ? `${row.aff_rebate_rate_percent}%` : <span className="muted">沿用全局</span>},
  {id: "count", title: "邀请人数", align: "number", render: row => row.aff_count},
];

export const inviteColumns: Column<AffInvite>[] = [
  {id: "inviter", title: "邀请人", render: row => person(row.inviter_id, row.inviter_email, row.inviter_username)},
  {id: "invitee", title: "被邀请人", render: row => person(row.invitee_id, row.invitee_email, row.invitee_username)},
  {id: "code", title: "邀请码", hideWhenNarrow: true, render: row => <span className="code-mono">{row.aff_code}</span>},
  {id: "rebate", title: "累计返利", align: "number", render: row => money(row.total_rebate)},
  {id: "at", title: "绑定时间", hideWhenNarrow: true, render: row => stamp(row.created_at)},
];

export const rebateColumns: Column<AffRebate>[] = [
  {id: "order", title: "订单号", render: row => <span className="code-mono text-fine">{row.out_trade_no || (row.order_id != null ? `#${row.order_id}` : "—")}</span>},
  {id: "inviter", title: "邀请人", render: row => person(row.inviter_id, row.inviter_email, row.inviter_username)},
  {id: "invitee", title: "被邀请人", hideWhenNarrow: true, render: row => row.invitee_id != null ? person(row.invitee_id, row.invitee_email, row.invitee_username) : "—"},
  {id: "pay", title: "实付 / 订单额", align: "number", hideWhenNarrow: true, render: row => `${row.pay_amount != null ? money(row.pay_amount) : "—"} / ${row.order_amount != null ? money(row.order_amount) : "—"}`},
  {id: "rebate", title: "返利", align: "number", render: row => money(row.rebate_amount)},
  {id: "status", title: "订单状态", align: "center", hideWhenCompact: true, render: row => <Badge variant="tag">{row.order_status || "—"}</Badge>},
  {id: "at", title: "时间", hideWhenNarrow: true, render: row => stamp(row.created_at)},
];

export const transferColumns: Column<AffTransfer>[] = [
  {id: "ledger", title: "流水", render: row => <span className="code-mono text-fine">#{row.ledger_id}</span>},
  {id: "user", title: "用户", render: row => person(row.user_id, row.user_email, row.username)},
  {id: "action", title: "动作", align: "center", render: row => <Badge variant="tag">{row.action}</Badge>},
  {id: "amount", title: "金额", align: "number", render: row => money(row.amount)},
  {id: "after", title: "转出后可用", align: "number", hideWhenNarrow: true, render: row => row.snapshot_available && row.available_quota_after != null ? money(row.available_quota_after) : <span className="muted">无快照</span>},
  {id: "at", title: "时间", hideWhenNarrow: true, render: row => stamp(row.created_at)},
];
