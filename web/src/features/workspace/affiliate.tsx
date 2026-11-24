import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {count, date, money, rate} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {Button, CopyValue, DataTable, EllipsisText, EmptyState, ErrorLine, InsetGroup, InsetRow, RefreshButton, Resource, WorkspaceContent, toast, type Column} from "@/ui";

const invitee = z.object({user_id: z.number(), email: z.string(), username: z.string(), created_at: z.string().nullish(), total_rebate: z.number()});
const affiliate = z.object({
  aff_code: z.string(), aff_count: z.number(), aff_quota: z.number(), aff_frozen_quota: z.number(), aff_history_quota: z.number(),
  effective_rebate_rate_percent: z.number(), invitees: z.array(invitee),
});
const transfer = z.object({transferred_quota: z.number(), balance: z.number()});
const invitees: Column<z.infer<typeof invitee>>[] = [
  {id: "name", title: "受邀用户", render: row => <EllipsisText text={row.username || row.email || "用户 #" + row.user_id} />},
  {id: "date", title: "注册时间", hideWhenCompact: true, render: row => row.created_at ? date(row.created_at) : "—"},
  {id: "rebate", title: "累计返利", align: "number", render: row => money(row.total_rebate)},
];

export function Affiliate() {
  const [revision, refresh] = useState(0);
  const action = useAction();
  const detail = useResource(() => api("/affiliate", affiliate), "affiliate", true, revision);

  return <WorkspaceContent>
    <Resource state={detail}>{row => {
      const inviteURL = new URL("/register", location.origin);
      inviteURL.searchParams.set("aff", row.aff_code);
      return <>
        <InsetGroup title="邀请返利" actions={<RefreshButton size="xs" variant="ghost" busy={detail?.loading} onClick={() => refresh(v => v + 1)} />}>
          <InsetRow label="专属邀请链接" vertical><CopyValue value={inviteURL.href} label="邀请链接" /></InsetRow>
          <InsetRow label="实际返利比例" hint={rate.format(row.effective_rebate_rate_percent / 100)} />
          <InsetRow label="已邀请人数" hint={count.format(row.aff_count) + " 人"} />
          <InsetRow label="累计返利" hint={money(row.aff_history_quota)} />
          <InsetRow label="冻结中返利" hint={money(row.aff_frozen_quota)} />
          <InsetRow label="可转入余额" hint={money(row.aff_quota)}>
            <Button size="sm" busy={action.busy} disabled={detail?.loading || row.aff_quota <= 0} onClick={() => void action.run(async () => {
              const next = await api("/affiliate/transfer", transfer, "POST");
              toast("已转入 " + money(next.transferred_quota) + "，当前余额 " + money(next.balance));
              refresh(value => value + 1);
            })}>转入钱包余额</Button>
          </InsetRow>
          <ErrorLine value={action.error} />
        </InsetGroup>
        {row.invitees.length ? <DataTable rows={row.invitees} rowID={item => item.user_id} columns={invitees} label="邀请记录" /> :
          <EmptyState title="暂无邀请记录" />}
      </>;
    }}</Resource>
  </WorkspaceContent>;
}
