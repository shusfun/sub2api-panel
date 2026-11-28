import {useMemo, useState} from "react";
import {Plus} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {count, date, relativeUntil, smartMoney} from "@/lib/format";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {adminSubscriptionRow, empty} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  BatchBar, Button, EmptyState, EntityLink, ErrorLine, FilterBar, FilterChips, FilterSlot,
  GroupSelect, MetricStrip, PlanIdentity, QuickPills, RefreshButton, RemoteTable, SearchInput,
  Status, WorkspaceStage, toast, type Column, type FilterChip,
} from "@/ui";
import {AssignSubscriptionDialog, ExtendSubscription} from "../users/user-dialogs";

type SubRow = z.infer<typeof adminSubscriptionRow>;

const bulkResult = z.object({success_count: z.number(), failed_count: z.number()});

export function SubscriptionsWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [search, setSearch] = useState(initial.get("search") ?? "");
  const [status, setStatus] = useState(initial.get("status") ?? "");
  const [groupID, setGroupID] = useState(initial.get("group_id") ?? "");
  const [revision, revise] = useState(0);
  const [selected, setSelected] = useState<number[]>([]);
  const [subTotal, setSubTotal] = useState<number>();
  const [assignOpen, setAssignOpen] = useState(false);
  const [renewing, setRenewing] = useState<SubRow>();

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const action = useAction();

  const listPath = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (status) p.set("status", status);
    if (groupID) p.set("group_id", groupID);
    const qs = p.toString();
    return "/admin/subscriptions" + (qs ? "?" + qs : "");
  }, [debouncedSearch, groupID, status]);

  const rowAction = (path: string, message: string) => () => void action.run(async () => {
    await api(path, empty, "POST", {});
    toast(message);
    refresh();
  });

  const bulk = (body: Record<string, unknown>, message: string) => () => void action.run(async () => {
    await api("/admin/subscriptions/bulk-action", bulkResult, "POST", {
      subscription_ids: selected, ...body, operation_id: crypto.randomUUID(),
    });
    toast(message);
    setSelected([]);
    refresh();
  });

  const columns: Column<SubRow>[] = [
    {
      id: "user",
      title: "订阅用户",
      render: row => (
        <EntityLink
          kind="user"
          id={row.user_id}
          label={row.user_email || `用户 #${row.user_id}`}
          onClick={() => navigateTo("users", {inspect: row.user_id, tab: "subs"})}
        />
      ),
    },
    {id: "group", title: "订阅分组", render: row => <PlanIdentity item={{id: row.group_id, name: row.group, platform: row.platform}} />},
    {
      id: "usage",
      title: "日 / 周 / 月消耗",
      align: "number",
      render: row => `${smartMoney(row.daily_usage_usd)} / ${smartMoney(row.weekly_usage_usd)} / ${smartMoney(row.monthly_usage_usd)}`,
    },
    {id: "exp", title: "到期时间", render: row => <span className="code-mono text-fine">{date(row.expires_at)} · {relativeUntil(row.expires_at)}</span>},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {
      id: "actions",
      title: "操作",
      align: "end",
      render: row => (
        <div className="row-actions">
          <Button size="xs" variant="outline" onClick={() => setRenewing(row)}>续期</Button>
          <Button size="xs" variant="ghost" busy={action.busy} onClick={rowAction(`/admin/subscriptions/${row.id}/reset-quota`, "已清零订阅周期用量")}>清零</Button>
          {row.status === "active" ? (
            <Button size="xs" variant="destructive" busy={action.busy} onClick={rowAction(`/admin/subscriptions/${row.id}/revoke`, "已撤销该订阅")}>撤销</Button>
          ) : (
            <Button size="xs" variant="outline" busy={action.busy} onClick={rowAction(`/admin/subscriptions/${row.id}/restore`, "已恢复该订阅")}>恢复</Button>
          )}
        </div>
      ),
    },
  ];

  const chips: FilterChip[] = [];
  if (debouncedSearch) chips.push({id: "search", label: "搜索", value: debouncedSearch, onRemove: () => setSearch("")});
  if (status) chips.push({id: "status", label: "状态", value: status, onRemove: () => setStatus("")});
  if (groupID) chips.push({id: "group", label: "分组", value: `#${groupID}`, onRemove: () => setGroupID("")});

  const filtered = Boolean(debouncedSearch || status || groupID);

  return (
    <>
      <WorkspaceStage flush>
        <MetricStrip
          flush
          items={[
            {id: "s-total", label: filtered ? "筛选结果" : "全部订阅", value: subTotal === undefined ? null : count.format(subTotal), unit: "份"},
          ]}
        />
        <FilterBar sticky>
          <div className="obs-filter-row">
            <SearchInput value={search} onChange={setSearch} placeholder="搜索用户邮箱" ariaLabel="搜索订阅用户" />
            <QuickPills
              label="订阅状态"
              value={status}
              options={[
                {value: "", label: "全部", tone: "neutral"},
                {value: "active", label: "有效", tone: "ok"},
                {value: "expired", label: "已过期", tone: "warn"},
                {value: "revoked", label: "已撤销", tone: "bad"},
              ]}
              onChange={setStatus}
            />
            <FilterSlot><GroupSelect value={groupID} subscriptionOnly placeholder="全部订阅组" onChange={setGroupID} /></FilterSlot>
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新订阅" onClick={refresh} />
              <Button onClick={() => setAssignOpen(true)}><Plus aria-hidden />分配订阅</Button>
            </div>
          </div>
          <FilterChips chips={chips} onReset={() => { setSearch(""); setStatus(""); setGroupID(""); }} />
          <ErrorLine value={action.error} />
        </FilterBar>

        <BatchBar count={selected.length} onClear={() => setSelected([])}>
          <Button size="xs" variant="outline" busy={action.busy} onClick={bulk({action: "extend", days: 30}, `已为 ${selected.length} 条订阅批量续期 30 天`)}>批量续期 +30 天</Button>
          <Button size="xs" variant="outline" busy={action.busy} onClick={bulk({action: "reset_quota", daily: true, weekly: true, monthly: true}, "已批量重置所选订阅的日/周/月消耗进度")}>批量重置周期配额</Button>
          <Button size="xs" variant="destructive" busy={action.busy} onClick={bulk({action: "revoke"}, "已批量撤销所选订阅")}>批量撤销</Button>
        </BatchBar>

        <div className="swift-pane-table">
          <RemoteTable
            key={listPath}
            path={listPath}
            schema={adminSubscriptionRow}
            columns={columns}
            rowID={row => row.id}
            label="订阅总表"
            revision={revision}
            onTotalLoaded={setSubTotal}
            selection={{selected, onChange: setSelected}}
            empty={<EmptyState title="暂无订阅记录" action={<Button size="sm" variant="outline" onClick={() => setAssignOpen(true)}><Plus aria-hidden />分配订阅</Button>} />}
          />
        </div>
      </WorkspaceStage>

      <AssignSubscriptionDialog
        open={assignOpen}
        revision={revision}
        onClose={() => setAssignOpen(false)}
        onSaved={() => { setAssignOpen(false); toast("订阅已分配"); refresh(); }}
      />
      {renewing && (
        <ExtendSubscription
          id={renewing.id}
          onClose={() => setRenewing(undefined)}
          onSaved={() => { setRenewing(undefined); toast("订阅已续期"); refresh(); }}
        />
      )}
    </>
  );
}
