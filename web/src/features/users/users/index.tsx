import {useMemo, useState} from "react";
import {SlidersHorizontal, UserPlus} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {count, date, money} from "@/lib/format";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {adminSubscriptionRow, userRow} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  Badge, BatchBar, Button, EmptyState, ErrorLine, FilterBar, FilterChips, FilterSlot,
  GroupSelect, IdentityBadge, MeterBar, MetricStrip, PlanSummary, QuickPills,
  RefreshButton, RemoteTable, SearchInput, Select, Status, WorkspaceStage, toast,
  useGroups, type Column, type FilterChip,
} from "@/ui";
import {
  AssignSubscriptionDialog, BatchUserLimitsDialog, CreateUserDialog,
} from "./user-dialogs";
import {UserAttributeSchemaDialog} from "./user-attribute-schema";
import {UserDetailWorkspace} from "./user-detail-workspace";

export type UserItem = z.infer<typeof userRow>;
export type SubRow = z.infer<typeof adminSubscriptionRow>;

export function UsersWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [role, setRole] = useState("");
  const [groupID, setGroupID] = useState("");
  const [sort, setSort] = useState("");
  const [revision, revise] = useState(0);

  const groupCatalog = useGroups({}, true);
  const [selectedUsers, setSelectedUsers] = useState<number[]>([]);
  const [userTotal, setUserTotal] = useState<number>();
  const standaloneID = Number(initial.get("inspect") ?? 0);

  const [createUserOpen, setCreateUserOpen] = useState(false);
  const [attrSchemaOpen, setAttrSchemaOpen] = useState(false);
  const [batchLimitOpen, setBatchLimitOpen] = useState(false);
  const [bulkSubAssignOpen, setBulkSubAssignOpen] = useState(false);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const batchAction = useAction();

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
    {
      id: "role",
      title: "角色",
      align: "center",
      hideWhenCompact: true,
      render: row => <Badge variant="soft" tone={row.role === "admin" ? "info" : "neutral"}>{row.role === "admin" ? "管理员" : "普通用户"}</Badge>,
    },
    {id: "balance", title: "可用余额", align: "number", render: row => money(row.balance)},
    {
      id: "conc",
      title: "并发 / RPM",
      align: "number",
      render: row => {
        const label = `${row.current_concurrency}/${row.concurrency || "不限"}${row.rpm_limit ? ` · ${row.rpm_limit}rpm` : ""}`;
        // 不限并发时没有占用比例，只显示数字。
        return row.concurrency > 0
          ? <MeterBar size="sm" value={(row.current_concurrency / row.concurrency) * 100} label={label} />
          : <span className="weir-digits text-fine">{label}</span>;
      },
    },
    {
      id: "groups",
      hideWhenNarrow: true,
      title: "私有分组",
      render: row => <PlanSummary items={(groupCatalog?.value ?? []).filter(group => group.is_exclusive && row.allowed_groups.includes(group.id))} label="私有分组" empty="—" />,
    },
    {id: "active", title: "最近调用", hideWhenNarrow: true, render: row => <span className="code-mono text-fine">{row.last_used_at ? date(row.last_used_at) : "—"}</span>},
    {id: "created", title: "注册时间", hideWhenNarrow: true, render: row => <span className="code-mono text-fine">{row.created_at ? date(row.created_at) : "—"}</span>},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
  ];

  const chips: FilterChip[] = [];
  if (debouncedSearch) chips.push({id: "search", label: "搜索", value: debouncedSearch, onRemove: () => setSearch("")});
  if (status) chips.push({id: "status", label: "状态", value: status, onRemove: () => setStatus("")});
  if (role) chips.push({id: "role", label: "角色", value: role, onRemove: () => setRole("")});
  if (groupID) chips.push({id: "group", label: "分组", value: `#${groupID}`, onRemove: () => setGroupID("")});

  const filtered = Boolean(debouncedSearch || status || role || groupID);

  return (
    <>
      <WorkspaceStage flush>
        <MetricStrip
          flush
          items={[
            {id: "u-total", label: filtered ? "筛选结果" : "全部用户", value: userTotal === undefined ? null : count.format(userTotal), unit: "位"},
          ]}
        />
        <FilterBar sticky>
          <div className="obs-filter-row">
            <SearchInput value={search} onChange={setSearch} placeholder="搜索邮箱或用户名" ariaLabel="搜索用户" />
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
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新用户" onClick={refresh} />
              <Button variant="outline" onClick={() => setAttrSchemaOpen(true)}><SlidersHorizontal aria-hidden />属性字段</Button>
              <Button onClick={() => setCreateUserOpen(true)}><UserPlus aria-hidden />创建用户</Button>
            </div>
          </div>
          <FilterChips chips={chips} onReset={() => { setSearch(""); setStatus(""); setRole(""); setGroupID(""); setSort(""); }} />
          <ErrorLine value={batchAction.error} />
        </FilterBar>

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
            onTotalLoaded={setUserTotal}
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
        selectedUserIDs={selectedUsers.length > 0 ? selectedUsers : undefined}
        onClose={() => setBulkSubAssignOpen(false)}
        onSaved={() => { setBulkSubAssignOpen(false); setSelectedUsers([]); toast("订阅已分配"); refresh(); }}
      />
      <UserAttributeSchemaDialog open={attrSchemaOpen} onClose={() => setAttrSchemaOpen(false)} />
    </>
  );
}
