import {useEffect, useMemo, useState} from "react";
import {Plus, SlidersHorizontal, UserPlus} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date, money, relativeUntil, smartMoney} from "@/lib/format";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {adminSubscriptionRow, empty, userRow} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  BatchBar, Button, EmptyState, EntityLink, ErrorLine, FilterBar, FilterChips, FilterSlot,
  GroupSelect, IdentityBadge, MeterBar, PlanIdentity, PlanSummary, QuickPills,
  RefreshButton, RemoteTable, SearchInput, Segmented, Select, Status, WorkspaceStage, toast,
  useGroups, type Column, type FilterChip,
} from "@/ui";
import {
  AssignSubscriptionDialog, BatchUserLimitsDialog, CreateUserDialog, ExtendSubscription,
} from "./user-dialogs";
import {UserAttributeSchemaDialog} from "./user-attribute-schema";
import {UserDetailWorkspace} from "./user-detail-workspace";

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
  const [sort, setSort] = useState("");
  const [revision, revise] = useState(0);

  const groupCatalog = useGroups({}, view === "users");
  const [selectedUsers, setSelectedUsers] = useState<number[]>([]);
  const [selectedSubs, setSelectedSubs] = useState<number[]>([]);
  const standaloneID = Number(initial.get("inspect") ?? 0);

  const [createUserOpen, setCreateUserOpen] = useState(false);
  const [attrSchemaOpen, setAttrSchemaOpen] = useState(false);
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
  }, [queryString]);

  const userListPath = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (status) p.set("status", status);
    if (role) p.set("role", role);
    if (groupID) p.set("api_key_group_id", groupID);
    if (sort) {
      const [by, order] = sort.split(":");
      if (by) p.set("sort_by", by);
      if (order) p.set("sort_order", order);
    }
    const qs = p.toString();
    return "/admin/users" + (qs ? "?" + qs : "");
  }, [debouncedSearch, groupID, role, sort, status]);

  const subListPath = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (status) p.set("status", status);
    if (groupID) p.set("group_id", groupID);
    const qs = p.toString();
    return "/admin/subscriptions" + (qs ? "?" + qs : "");
  }, [debouncedSearch, groupID, status]);

  if (standaloneID > 0) {
    return (
      <UserDetailWorkspace
        userID={standaloneID}
        initialTab={initial.get("tab") ?? undefined}
        onBack={() => navigateTo("users")}
      />
    );
  }

  const userCols: Column<UserItem>[] = [
    {
      id: "email",
      title: "用户账号",
      render: row => (
        <IdentityBadge
          name={row.email}
          meta={[row.username, row.notes].filter(Boolean).join(" · ") || (row.role === "admin" ? "管理员" : undefined)}
          tone={row.role === "admin" ? "accent" : row.status === "active" ? "ok" : "bad"}
        />
      ),
    },
    {id: "balance", title: "可用余额", align: "number", render: row => money(row.balance)},
    {
      id: "conc",
      title: "并发 / RPM",
      align: "number",
      render: row => (
        <MeterBar
          size="sm"
          value={row.concurrency > 0 ? (row.current_concurrency / row.concurrency) * 100 : 0}
          label={`${row.current_concurrency}/${row.concurrency || "∞"}${row.rpm_limit ? ` · ${row.rpm_limit}rpm` : ""}`}
        />
      ),
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
      id: "renew",
      title: "操作",
      align: "end",
      render: row => (
        <div className="row-actions">
          <Button size="xs" variant="outline" onClick={() => setRenewing(row)}>续期</Button>
          <Button size="xs" variant="ghost" busy={batchAction.busy} onClick={() => void batchAction.run(async () => { await api(`/admin/subscriptions/${row.id}/reset-quota`, empty, "POST", {}); toast("已清零订阅周期用量"); refresh(); })}>清零</Button>
          {row.status === "active" ? (
            <Button size="xs" variant="destructive" busy={batchAction.busy} onClick={() => void batchAction.run(async () => { await api(`/admin/subscriptions/${row.id}/revoke`, empty, "POST", {}); toast("已撤销该订阅"); refresh(); })}>撤销</Button>
          ) : (
            <Button size="xs" variant="outline" busy={batchAction.busy} onClick={() => void batchAction.run(async () => { await api(`/admin/subscriptions/${row.id}/restore`, empty, "POST", {}); toast("已恢复该订阅"); refresh(); })}>恢复</Button>
          )}
        </div>
      ),
    },
  ];

  const chips: FilterChip[] = [];
  if (debouncedSearch) chips.push({id: "search", label: "搜索", value: debouncedSearch, onRemove: () => setSearch("")});
  if (status) chips.push({id: "status", label: "状态", value: status, onRemove: () => setStatus("")});
  if (role && view === "users") chips.push({id: "role", label: "角色", value: role, onRemove: () => setRole("")});
  if (groupID) chips.push({id: "group", label: "分组", value: `#${groupID}`, onRemove: () => setGroupID("")});

  return (
    <>
      <WorkspaceStage flush>
        <FilterBar sticky>
          <div className="obs-filter-row">
            <Segmented
              label="视图"
              value={view}
              options={[["users", "用户"], ["subscriptions", "订阅"]] as const}
              onChange={next => { setView(next); setStatus(""); }}
            />
            <SearchInput value={search} onChange={setSearch} placeholder={view === "users" ? "搜索邮箱或用户名" : "搜索用户邮箱"} ariaLabel={view === "users" ? "搜索用户" : "搜索订阅用户"} />
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
                <FilterSlot>
                  <Select value={role} aria-label="按角色筛选" onChange={e => setRole(e.target.value)}>
                    <option value="">全部角色</option>
                    <option value="user">普通用户</option>
                    <option value="admin">管理员</option>
                  </Select>
                </FilterSlot>
                <FilterSlot>
                  <Select value={sort} aria-label="排序方式" onChange={e => setSort(e.target.value)}>
                    <option value="">默认排序</option>
                    <option value="balance:desc">余额降序</option>
                    <option value="balance:asc">余额升序</option>
                    <option value="last_used_at:desc">最近活跃</option>
                    <option value="created_at:desc">最新注册</option>
                  </Select>
                </FilterSlot>
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
              {view === "users" && (
                <>
                  <Button variant="outline" onClick={() => setAttrSchemaOpen(true)}><SlidersHorizontal aria-hidden />属性字段</Button>
                  <Button onClick={() => setCreateUserOpen(true)}><UserPlus aria-hidden />创建用户</Button>
                </>
              )}
              {view === "subscriptions" && <Button onClick={() => setBulkSubAssignOpen(true)}><Plus aria-hidden />分配订阅</Button>}
            </div>
          </div>
          <FilterChips chips={chips} onReset={() => { setSearch(""); setStatus(""); setRole(""); setGroupID(""); setSort(""); }} />
          <ErrorLine value={batchAction.error} />
        </FilterBar>

        {view === "users" && (
          <>
            <BatchBar count={selectedUsers.length} onClear={() => setSelectedUsers([])}>
              <Button size="xs" variant="outline" onClick={() => setBatchLimitOpen(true)}>批量设并发 / RPM</Button>
              <Button size="xs" variant="outline" onClick={() => setBulkSubAssignOpen(true)}>批量分配订阅</Button>
              <Button
                size="xs"
                variant="outline"
                busy={batchAction.busy}
                onClick={() => void batchAction.run(async () => {
                  const res = await api("/admin/users/batch-enable", z.object({affected: z.number()}), "POST", {user_ids: selectedUsers});
                  setSelectedUsers([]);
                  toast(`已批量启用 ${res.affected} 位用户`);
                  refresh();
                })}
              >
                批量启用
              </Button>
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
                onRowClick={row => navigateTo("users", {inspect: row.id})}
                selection={{selected: selectedUsers, onChange: setSelectedUsers}}
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
        onCreated={u => { setCreateUserOpen(false); refresh(); navigateTo("users", {inspect: u.id}); }}
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
        selectedUserIDs={view === "users" && selectedUsers.length > 0 ? selectedUsers : undefined}
        onClose={() => setBulkSubAssignOpen(false)}
        onSaved={() => { setBulkSubAssignOpen(false); setSelectedUsers([]); toast("订阅已分配"); refresh(); }}
      />
      <UserAttributeSchemaDialog open={attrSchemaOpen} onClose={() => setAttrSchemaOpen(false)} />
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
