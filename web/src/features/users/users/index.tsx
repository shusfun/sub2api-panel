import {useEffect, useMemo, useState} from "react";
import {Plus, UserPlus} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date, money, relativeUntil, smartMoney} from "@/lib/format";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {adminSubscriptionRow, userRow} from "@/schemas";
import {
  Badge, BatchBar, Button, EmptyState, EntityLink, ErrorLine, FilterBar, FilterChips, FilterSlot,
  GroupSelect, InlineStack, Inspector, PlanIdentity, PlanSummary, QuickPills, RefreshButton,
  RemoteTable, SearchInput, Segmented, Status, WorkspaceStage, toast, useGroups,
  type Column, type FilterChip,
} from "@/ui";
import {
  AssignSubscriptionDialog, BatchUserLimitsDialog, CreateUserDialog, ExtendSubscription,
} from "./user-dialogs";
import {UserDetailInspector} from "./user-inspector";

export type UserItem = z.infer<typeof userRow>;
export type SubRow = z.infer<typeof adminSubscriptionRow>;
type TopView = "users" | "subscriptions";

export function UsersWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [view, setView] = useState<TopView>(() => {
    const v = initial.get("view");
    return v === "subscriptions" ? v : "users";
  });
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [role, setRole] = useState("");
  const [groupID, setGroupID] = useState("");
  const [revision, revise] = useState(0);

  const groupCatalog = useGroups({}, view === "users");
  const [selectedUsers, setSelectedUsers] = useState<number[]>([]);
  const [selectedSubs, setSelectedSubs] = useState<number[]>([]);
  const [userRows, setUserRows] = useState<UserItem[]>([]);
  const [inspectedUserID, setInspectedUserID] = useState<number | undefined>(() => {
    const id = Number(initial.get("inspect") ?? 0);
    return id > 0 ? id : undefined;
  });
  const [inspectorTab, setInspectorTab] = useState<"account" | "subs" | "keys" | "grant">(() => {
    const t = initial.get("tab");
    return t === "keys" || t === "subs" || t === "grant" ? t : "account";
  });

  const [createUserOpen, setCreateUserOpen] = useState(false);
  const [batchLimitOpen, setBatchLimitOpen] = useState(false);
  const [bulkSubAssignOpen, setBulkSubAssignOpen] = useState(false);
  const [renewing, setRenewing] = useState<SubRow>();

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const batchAction = useAction();

  useEffect(() => {
    const params = new URLSearchParams(queryString);
    const v = params.get("view");
    if (v === "users" || v === "subscriptions") setView(v);
    const id = Number(params.get("inspect") ?? 0);
    if (id > 0) setInspectedUserID(id);
    const t = params.get("tab");
    if (t === "keys" || t === "subs" || t === "grant" || t === "account") setInspectorTab(t);
  }, [queryString]);

  const userListPath = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (status) p.set("status", status);
    if (role) p.set("role", role);
    if (groupID) p.set("api_key_group_id", groupID);
    const qs = p.toString();
    return "/admin/users" + (qs ? "?" + qs : "");
  }, [debouncedSearch, groupID, role, status]);

  const subListPath = useMemo(() => {
    const p = new URLSearchParams();
    if (status) p.set("status", status);
    if (groupID) p.set("group_id", groupID);
    const qs = p.toString();
    return "/admin/subscriptions" + (qs ? "?" + qs : "");
  }, [groupID, status]);

  const userCols: Column<UserItem>[] = [
    {
      id: "email",
      title: "用户账号",
      render: row => (
        <div>
          <InlineStack density="compact">
            <strong className="compact-text" title={row.email}>{row.email}</strong>
            {row.role === "admin" && <Badge variant="soft" tone="info">管理员</Badge>}
          </InlineStack>
          {(row.username || row.notes) && <div className="muted text-fine">{[row.username, row.notes].filter(Boolean).join(" · ")}</div>}
        </div>
      ),
    },
    {id: "balance", title: "可用余额", align: "number", render: row => money(row.balance)},
    {
      id: "conc",
      title: "并发 / RPM",
      align: "number",
      render: row => `${row.current_concurrency}/${row.concurrency || "∞"}${row.rpm_limit ? ` · ${row.rpm_limit}rpm` : ""}`,
    },
    {
      id: "groups",
      hideWhenNarrow: true,
      title: "私有分组",
      render: row => <PlanSummary items={(groupCatalog?.value ?? []).filter(group => group.is_exclusive && row.allowed_groups.includes(group.id))} label="私有分组" empty="—" />,
    },
    {id: "active", title: "最近调用", hideWhenNarrow: true, render: row => <span className="code-mono text-fine">{row.last_used_at ? date(row.last_used_at) : "—"}</span>},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
  ];

  const subCols: Column<SubRow>[] = [
    {
      id: "user",
      title: "订阅用户",
      render: row => (
        <EntityLink
          kind="user"
          id={row.user_id}
          label={row.user_email || `用户 #${row.user_id}`}
          onClick={() => { setInspectedUserID(row.user_id); setInspectorTab("subs"); }}
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
    {id: "renew", title: "操作", align: "end", render: row => <Button size="xs" variant="outline" onClick={() => setRenewing(row)}>续期</Button>},
  ];

  const chips: FilterChip[] = [];
  if (debouncedSearch) chips.push({id: "search", label: "搜索", value: debouncedSearch, onRemove: () => setSearch("")});
  if (status) chips.push({id: "status", label: "状态", value: status, onRemove: () => setStatus("")});
  if (role && view === "users") chips.push({id: "role", label: "角色", value: role, onRemove: () => setRole("")});
  if (groupID) chips.push({id: "group", label: "分组", value: `#${groupID}`, onRemove: () => setGroupID("")});

  const userIdx = inspectedUserID ? userRows.findIndex(u => u.id === inspectedUserID) : -1;

  return (
    <>
      <WorkspaceStage
        flush
        inspectorOpen={!!inspectedUserID}
        inspector={
          <Inspector
            open={!!inspectedUserID}
            title={userRows[userIdx]?.email ?? "用户"}
            subtitle={inspectedUserID ? `用户 #${inspectedUserID}` : undefined}
            badge={userRows[userIdx]?.role === "admin" ? <Badge variant="soft" tone="info">管理员</Badge> : undefined}
            onClose={() => setInspectedUserID(undefined)}
            onPrev={userIdx > 0 ? () => setInspectedUserID(userRows[userIdx - 1].id) : undefined}
            onNext={userIdx >= 0 && userIdx < userRows.length - 1 ? () => setInspectedUserID(userRows[userIdx + 1].id) : undefined}
          >
            {inspectedUserID && (
              <UserDetailInspector
                key={`usr-${inspectedUserID}`}
                userID={inspectedUserID}
                tab={inspectorTab}
                onTabChange={setInspectorTab}
                revision={revision}
                onChanged={refresh}
                onDeleted={() => { setInspectedUserID(undefined); refresh(); }}
              />
            )}
          </Inspector>
        }
      >
        <FilterBar sticky>
          <div className="obs-filter-row">
            <Segmented
              label="视图"
              value={view}
              options={[["users", "用户"], ["subscriptions", "订阅"]] as const}
              onChange={next => { setView(next); setStatus(""); }}
            />
            {view === "users" && <SearchInput value={search} onChange={setSearch} placeholder="搜索邮箱或用户名" ariaLabel="搜索用户" />}
            {view === "users" && (
              <>
                <QuickPills
                  label="用户状态"
                  value={status}
                  options={[
                    {value: "", label: "全部", tone: "neutral"},
                    {value: "active", label: "正常", tone: "ok"},
                    {value: "disabled", label: "已停用", tone: "bad"},
                  ]}
                  onChange={setStatus}
                />
                <FilterSlot><GroupSelect value={groupID} placeholder="按 Key 绑定分组" onChange={setGroupID} /></FilterSlot>
              </>
            )}
            {view === "subscriptions" && (
              <>
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
              </>
            )}
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新用户" onClick={refresh} />
              {view === "users" && <Button onClick={() => setCreateUserOpen(true)}><UserPlus aria-hidden />创建用户</Button>}
              {view === "subscriptions" && <Button onClick={() => setBulkSubAssignOpen(true)}><Plus aria-hidden />分配订阅</Button>}
            </div>
          </div>
          <FilterChips chips={chips} onReset={() => { setSearch(""); setStatus(""); setRole(""); setGroupID(""); }} />
          <ErrorLine value={batchAction.error} />
        </FilterBar>

        {view === "users" && (
          <>
            <BatchBar count={selectedUsers.length} onClear={() => setSelectedUsers([])}>
              <Button size="xs" variant="outline" onClick={() => setBatchLimitOpen(true)}>批量设并发 / RPM</Button>
              <Button
                size="xs"
                variant="destructive"
                busy={batchAction.busy}
                onClick={() => void batchAction.run(async () => {
                  const res = await api("/admin/users/batch-disable", z.object({affected: z.number()}), "POST", {user_ids: selectedUsers});
                  setSelectedUsers([]);
                  toast(`已批量停用 ${res.affected} 位用户`);
                  refresh();
                })}
              >
                批量停用
              </Button>
            </BatchBar>

            <div className="swift-pane-table">
              <RemoteTable
                key={userListPath}
                path={userListPath}
                schema={userRow}
                columns={userCols}
                rowID={row => row.id}
                label="用户列表"
                revision={revision}
                activeRowID={inspectedUserID}
                onRowClick={row => setInspectedUserID(row.id)}
                selection={{selected: selectedUsers, onChange: setSelectedUsers}}
                onItemsLoaded={setUserRows}
                empty={
                  <EmptyState
                    title="未找到匹配用户"
                    action={<Button size="sm" onClick={() => setCreateUserOpen(true)}><UserPlus aria-hidden />创建新用户</Button>}
                  />
                }
              />
            </div>
          </>
        )}

        {view === "subscriptions" && (
          <>
            <BatchBar count={selectedSubs.length} onClear={() => setSelectedSubs([])}>
              <Button
                size="xs"
                variant="outline"
                busy={batchAction.busy}
                onClick={() => void batchAction.run(async () => {
                  await api("/admin/subscriptions/bulk-action", z.object({success_count: z.number(), failed_count: z.number()}), "POST", {
                    subscription_ids: selectedSubs, action: "extend", days: 30, operation_id: crypto.randomUUID(),
                  });
                  toast(`已为 ${selectedSubs.length} 条订阅批量续期 30 天`);
                  setSelectedSubs([]);
                  refresh();
                })}
              >
                批量续期 +30 天
              </Button>
              <Button
                size="xs"
                variant="outline"
                busy={batchAction.busy}
                onClick={() => void batchAction.run(async () => {
                  await api("/admin/subscriptions/bulk-action", z.object({success_count: z.number(), failed_count: z.number()}), "POST", {
                    subscription_ids: selectedSubs, action: "reset_quota", daily: true, weekly: true, monthly: true, operation_id: crypto.randomUUID(),
                  });
                  toast("已批量重置所选订阅的日/周/月消耗进度");
                  setSelectedSubs([]);
                  refresh();
                })}
              >
                批量重置周期配额
              </Button>
              <Button
                size="xs"
                variant="destructive"
                busy={batchAction.busy}
                onClick={() => void batchAction.run(async () => {
                  await api("/admin/subscriptions/bulk-action", z.object({success_count: z.number(), failed_count: z.number()}), "POST", {
                    subscription_ids: selectedSubs, action: "revoke", operation_id: crypto.randomUUID(),
                  });
                  toast("已批量撤销所选订阅");
                  setSelectedSubs([]);
                  refresh();
                })}
              >
                批量撤销
              </Button>
            </BatchBar>

            <div className="swift-pane-table">
              <RemoteTable
                key={subListPath}
                path={subListPath}
                schema={adminSubscriptionRow}
                columns={subCols}
                rowID={row => row.id}
                label="订阅总表"
                revision={revision}
                selection={{selected: selectedSubs, onChange: setSelectedSubs}}
                empty={<EmptyState title="暂无订阅记录" action={<Button size="sm" onClick={() => setBulkSubAssignOpen(true)}><Plus aria-hidden />分配订阅</Button>} />}
              />
            </div>
          </>
        )}
      </WorkspaceStage>

      <CreateUserDialog
        open={createUserOpen}
        revision={revision}
        onClose={() => setCreateUserOpen(false)}
        onCreated={u => { setCreateUserOpen(false); refresh(); setInspectedUserID(u.id); }}
      />
      <BatchUserLimitsDialog
        open={batchLimitOpen}
        userIDs={selectedUsers}
        onClose={() => setBatchLimitOpen(false)}
        onSaved={() => { setBatchLimitOpen(false); setSelectedUsers([]); toast("已批量更新用户并发与 RPM 限制"); refresh(); }}
      />
      <AssignSubscriptionDialog
        open={bulkSubAssignOpen}
        revision={revision}
        onClose={() => setBulkSubAssignOpen(false)}
        onSaved={() => { setBulkSubAssignOpen(false); toast("订阅已分配"); refresh(); }}
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
