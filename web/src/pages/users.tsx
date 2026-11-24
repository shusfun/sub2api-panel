import { useId,useEffect,useMemo,useState } from "react";
import { Plus,UserPlus } from "lucide-react";
import { z } from "zod";
import {
adminSubscriptionRow,
api,empty,
grantSchema,keyRow,
page,userRow
} from "../api";
import { cents,date,money,relativeUntil,smartMoney } from "../format";
import { useAction,useDebouncedValue,useResource } from "../requests";
import { navigateTo } from "../route";
import {
InlineStack, FormActions,
FormSection,
Badge,
BatchBar,
Button,
ConfirmDialog,DataTable,
EmptyState,
EntityLink,
ErrorLine,
Feedback,
Field,
FilterBar,
FilterChips,
FilterSlot,
Form,
FormGrid,
GroupDialog,
GroupSelect,
PlanIdentity,
PlanSummary,
useGroups,
Input,
InsetGroup,
InsetRow,
Inspector,MultiGroupSelect,
Overlay,PasswordInput,QuickPills,
RefreshButton,
Resource,
SearchInput,
Segmented,
Select,
Status,
StepUpDialog,
Switch,
TagInput,
Toolbar,
UserSelect,
WorkspaceStage,
toast,
type Column,
type FilterChip
} from "../ui";
import { RemoteTable } from "./remote-table";
import { UserQuotas } from "./user-quotas";
import { UserAttributes } from "./user-attributes";

type User = z.infer<typeof userRow>;
type SubRow = z.infer<typeof adminSubscriptionRow>;
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
  const [userRows, setUserRows] = useState<User[]>([]);
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

  const userCols: Column<User>[] = [
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
    {
      id: "group",
      title: "订阅分组",
      render: row => <PlanIdentity item={{id: row.group_id, name: row.group, platform: row.platform}} />,
    },
    {
      id: "usage",
      title: "日 / 周 / 月消耗",
      align: "number",
      render: row => `${smartMoney(row.daily_usage_usd)} / ${smartMoney(row.weekly_usage_usd)} / ${smartMoney(row.monthly_usage_usd)}`,
    },
    {
      id: "exp",
      title: "到期时间",
      render: row => <span className="code-mono text-fine">{date(row.expires_at)} · {relativeUntil(row.expires_at)}</span>,
    },
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
              options={[
                ["users", "用户"],
                ["subscriptions", "订阅"],
              ] as const}
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
              {view === "users" && (
                <Button onClick={() => setCreateUserOpen(true)}><UserPlus aria-hidden />创建用户</Button>
              )}
              {view === "subscriptions" && (
                <Button onClick={() => setBulkSubAssignOpen(true)}><Plus aria-hidden />分配订阅</Button>
              )}
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
                    subscription_ids: selectedSubs,
                    action: "extend",
                    days: 30,
                    operation_id: crypto.randomUUID(),
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
                    subscription_ids: selectedSubs,
                    action: "reset_quota",
                    daily: true,
                    weekly: true,
                    monthly: true,
                    operation_id: crypto.randomUUID(),
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
                    subscription_ids: selectedSubs,
                    action: "revoke",
                    operation_id: crypto.randomUUID(),
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
        onCreated={u => {
          setCreateUserOpen(false);
          refresh();
          setInspectedUserID(u.id);
        }}
      />

      <BatchUserLimitsDialog
        open={batchLimitOpen}
        userIDs={selectedUsers}
        onClose={() => setBatchLimitOpen(false)}
        onSaved={() => {
          setBatchLimitOpen(false);
          setSelectedUsers([]);
          toast("已批量更新用户并发与 RPM 限制");
          refresh();
        }}
      />

      <AssignSubscriptionDialog
        open={bulkSubAssignOpen}
        revision={revision}
        onClose={() => setBulkSubAssignOpen(false)}
        onSaved={() => {
          setBulkSubAssignOpen(false);
          toast("订阅已分配");
          refresh();
        }}
      />
      {renewing && <ExtendSubscription id={renewing.id} onClose={() => setRenewing(undefined)} onSaved={() => {
        setRenewing(undefined);
        toast("订阅已续期");
        refresh();
      }} />}

    </>
  );
}

function UserDetailInspector({userID, tab, onTabChange, revision, onChanged, onDeleted}: {
  userID: number; tab: "account" | "subs" | "keys" | "grant";
  onTabChange: (next: "account" | "subs" | "keys" | "grant") => void;
  revision: number; onChanged: () => void; onDeleted: () => void;
}) {
  const [task, setTask] = useState<"balance" | "limits" | "quotas" | "attributes" | "groups" | "replace-group" | "assign" | "profile" | "delete">();
  const [relatedRevision, reviseRelated] = useState(0);
  const detail = useResource(() => api(`/admin/users/${userID}`, userRow), `usr-detail-${userID}`, true, revision);
  const groups = useGroups({revision});
  const subs = useResource(() => api(`/admin/users/${userID}/subscriptions?page=1`, page(adminSubscriptionRow)), `usr-subs-${userID}`, true, relatedRevision);
  const keys = useResource(() => api(`/admin/users/${userID}/keys?page=1`, page(keyRow)), `usr-keys-${userID}`, true, relatedRevision);
  const op = useAction();
  const listOpen = tab !== "account";
  const finish = (message: string, related = false) => {
    toast(message);
    setTask(undefined);
    if (related) reviseRelated(n => n + 1);
    else onChanged();
  };
  const open = (next: typeof task) => { op.clearError(); setTask(next); };
  return <Resource state={detail}>{usr => <>
    <InsetGroup>
      <InsetRow label="账号状态"><Status value={usr.status} /></InsetRow>
      <InsetRow label="可用余额" onClick={() => open("balance")}><strong className="weir-digits">{money(usr.balance)}</strong></InsetRow>
      <InsetRow label="并发 / RPM" onClick={() => open("limits")}><span className="weir-digits">{usr.concurrency} / {usr.rpm_limit || "不限"}</span></InsetRow>
      <InsetRow label="平台配额" onClick={() => open("quotas")} />
      <InsetRow label="自定义属性" onClick={() => open("attributes")} />
    </InsetGroup>
    <InsetGroup title="授权与分发">
      <InsetRow label="私有分组" hint={usr.restrict_public_groups ? "公开分组受现有规则限制" : "可访问公开分组"} onClick={() => open("groups")}>
        {(groups?.value ?? []).filter(g => g.is_exclusive && usr.allowed_groups.includes(g.id)).length || "未授权"}
      </InsetRow>
      <InsetRow label="替换专属分组" onClick={() => open("replace-group")} />
      <InsetRow label="订阅" onClick={() => onTabChange("subs")}>{subs?.value ? `${subs.value.items.length}${subs.value.more ? "+" : ""}` : "未知"}</InsetRow>
      <InsetRow label="API Key" onClick={() => onTabChange("keys")}>{keys?.value ? `${keys.value.items.length}${keys.value.more ? "+" : ""}` : "未知"}</InsetRow>
      {usr.role !== "admin" && <InsetRow label="发码授权" onClick={() => onTabChange("grant")} />}
    </InsetGroup>
    <Toolbar>
      <Button onClick={() => open("assign")}>分配订阅</Button>
      <Button variant="outline" onClick={() => open("profile")}>编辑账号</Button>
      <Button variant="link" onClick={() => navigateTo("usage", {user: usr.id})}>查看用量</Button>
      <Button variant="link" onClick={() => navigateTo("ops", {tab: "errors", user_id: usr.id})}>查看报错</Button>
    </Toolbar>
    {usr.role !== "admin" && <InsetGroup><InsetRow label="删除用户" tone="bad" onClick={() => open("delete")} /></InsetGroup>}
    {task === "groups" && <GroupDialog title="授权私有分组" source={{privateOnly: true, revision}} value={usr.allowed_groups} multiple allowEmpty activeOnly busy={op.busy} error={op.error}
      onClose={() => open(undefined)} onApply={ids => void op.run(async () => {
        await api(`/admin/users/${usr.id}/private-groups`, userRow, "PUT", {group_ids: ids});
        finish("私有分组已更新");
      })} />}
    {task === "replace-group" && <ReplaceUserGroup user={usr} groups={groups?.value ?? []} onClose={() => open(undefined)} onSaved={message => { finish(message); reviseRelated(n => n + 1); }} />}
    {task === "quotas" && <UserQuotas userID={usr.id} onClose={() => open(undefined)} />}
    {task === "attributes" && <UserAttributes userID={usr.id} onClose={() => open(undefined)} />}
    {task === "assign" && <AssignSubscriptionDialog open revision={revision} fixedUserID={usr.id} onClose={() => open(undefined)} onSaved={() => finish("已开通订阅", true)} />}
    {(task === "balance" || task === "limits" || task === "profile") && <UserChangeDialog task={task} usr={usr} onClose={() => open(undefined)} onSaved={finish} />}
    {listOpen && <Overlay open title={tab === "subs" ? "用户订阅" : tab === "keys" ? "用户名下 API Key" : "编辑发码授权"} onClose={() => onTabChange("account")}>
      {tab === "subs" && <UserSubscriptionsPane userID={usr.id} revision={relatedRevision} onChanged={msg => finish(msg, true)} />}
      {tab === "keys" && <UserKeysPane userID={usr.id} revision={relatedRevision} onChanged={msg => finish(msg, true)} />}
      {tab === "grant" && usr.role !== "admin" && <GrantEditor user={usr.id} />}
    </Overlay>}
    <ConfirmDialog open={task === "delete"} title="删除用户" description={`确定删除用户「${usr.email}」吗？`} confirm="删除用户" busy={op.busy} error={op.error}
      onClose={() => open(undefined)} onConfirm={() => void op.run(async () => {await api(`/admin/users/${usr.id}`, empty, "DELETE"); onDeleted();})} />
  </>}</Resource>;
}

function ReplaceUserGroup({user, groups, onClose, onSaved}: {
  user: User;
  groups: readonly {id: number; name: string; status: string; is_exclusive?: boolean; subscription_type: string}[];
  onClose: () => void;
  onSaved: (message: string) => void;
}) {
  const formID = useId();
  const action = useAction();
  const [oldID, setOld] = useState("");
  const [newID, setNew] = useState("");
  const source = groups.filter(group => group.is_exclusive && user.allowed_groups.includes(group.id));
  const destination = groups.filter(group => group.is_exclusive && group.status === "active" && group.subscription_type === "standard" && String(group.id) !== oldID);
  return <Overlay open title="替换专属分组" onClose={onClose} busy={action.busy} footer={<>
    <Button variant="outline" onClick={onClose}>取消</Button>
    <Button type="submit" form={formID} busy={action.busy} disabled={!oldID || !newID || oldID === newID}>迁移密钥</Button>
  </>}>
    <Form id={formID} onSubmit={() => void action.run(async () => {
      const result = await api("/admin/users/" + user.id + "/replace-group", z.object({migrated_keys: z.number().int().nonnegative()}), "POST", {old_group_id: Number(oldID), new_group_id: Number(newID)});
      onSaved("已迁移 " + result.migrated_keys + " 个密钥并更新分组授权");
    })}>
      <Field label="当前分组"><Select value={oldID} onChange={event => { setOld(event.target.value); setNew(""); }}><option value="">选择分组</option>{source.map(group => <option key={group.id} value={group.id}>{group.name}</option>)}</Select></Field>
      <Field label="目标分组"><Select value={newID} onChange={event => setNew(event.target.value)}><option value="">选择分组</option>{destination.map(group => <option key={group.id} value={group.id}>{group.name}</option>)}</Select></Field>
      {!source.length && <Feedback>此用户未获授权访问任何专属分组。</Feedback>}
      <ErrorLine value={action.error} />
    </Form>
  </Overlay>;
}

function UserChangeDialog({task, usr, onClose, onSaved}: {
  task: "balance" | "limits" | "profile"; usr: User; onClose: () => void; onSaved: (message: string) => void;
}) {
  const formID = useId();
  const op = useAction();
  const [enabled, setEnabled] = useState(usr.status === "active");
  const path = `/admin/users/${usr.id}`;
  const title = {balance: "调整用户余额", limits: "修改用户限流", profile: "编辑用户账号"}[task];
  return <Overlay open title={title} onClose={onClose} busy={op.busy} footer={
          <><Button variant="outline" onClick={onClose}>取消</Button><Button type="submit" form={formID} busy={op.busy}>保存</Button></>
        }>
    <Form id={formID} busy={op.busy} onSubmit={event => {
      const data = new FormData(event.currentTarget);
      const password = String(data.get("password") ?? "");
      void op.run(async () => {
        if (task === "balance") await api(path + "/balance", userRow, "POST", {balance: Number(data.get("balance")), operation: data.get("operation"), notes: data.get("notes")});
        else await api(path, userRow, "PUT", task === "limits" ? {concurrency: Number(data.get("concurrency")), rpm_limit: Number(data.get("rpm"))} :
          {status: enabled ? "active" : "disabled", notes: data.get("notes"), ...(password ? {password} : {})});
        onSaved({balance: "用户余额已更新", limits: "用户限流已更新", profile: "用户账号已更新"}[task]);
      });
    }}>
      {task === "balance" && <>
        <Field label="调整方式"><Select name="operation" defaultValue="add"><option value="add">增加</option><option value="subtract">扣减</option><option value="set">设为</option></Select></Field>
        <Field label="金额"><Input name="balance" type="number" suffix="USD" min="0" step="0.01" required /></Field>
      </>}
      {task === "limits" && <>
        <Field label="最大并发"><Input name="concurrency" type="number" min="0" max="10000" defaultValue={usr.concurrency} required /></Field>
        <Field label="每分钟请求上限"><Input name="rpm" type="number" suffix="RPM" min="0" max="100000" defaultValue={usr.rpm_limit} required /></Field>
      </>}
      {task === "profile" && <>
        <Switch checked={enabled} label="启用该用户" onChange={setEnabled} />
        <Field label="新密码（选填）"><PasswordInput name="password" autoComplete="new-password" minLength={6} maxLength={128} /></Field>
      </>}
      {task !== "limits" && <Field label="备注（选填）"><Input name="notes" defaultValue={task === "profile" ? usr.notes : ""} /></Field>}
      <ErrorLine value={op.error} />
    </Form>
  </Overlay>;
}

function UserSubscriptionsPane({userID, revision, onChanged}: {userID: number; revision: number; onChanged: (msg: string) => void}) {
  const [renew, setRenew] = useState<SubRow>();
  const subs = useResource(() => api(`/admin/users/${userID}/subscriptions?page=1`, page(adminSubscriptionRow)), `usr-subs-${userID}`, true, revision);
  const op = useAction();

  return (
    <>
      <Resource state={subs}>
        {data => (
          <DataTable
            label="用户订阅"
            rows={data.items}
            rowID={r => r.id}
            empty={<EmptyState title="该用户暂无订阅套餐" />}
            columns={[
              {
                id: "group",
                title: "订阅组 / 到期",
                render: r => <PlanIdentity item={{id: r.group_id, name: r.group, platform: r.platform, expires_at: r.expires_at}} />,
              },
              {id: "status", title: "状态", align: "center", render: r => <Status value={r.status} />},
              {
                id: "act",
                title: "操作",
                align: "end",
                render: r => (
                  <div className="row-actions">
                    <Button size="xs" variant="outline" onClick={() => setRenew(r)}>续期</Button>
                    <Button
                      size="xs"
                      variant="ghost"
                      busy={op.busy}
                      onClick={() => void op.run(async () => {
                        await api(`/admin/subscriptions/${r.id}/reset-quota`, empty, "POST", {});
                        onChanged("已重置订阅周期用量");
                      })}
                    >
                      清零配额
                    </Button>
                    {r.status === "active" ? (
                      <Button
                        size="xs"
                        variant="destructive"
                        busy={op.busy}
                        onClick={() => void op.run(async () => {
                          await api(`/admin/subscriptions/${r.id}/revoke`, empty, "POST", {});
                          onChanged("已撤销该订阅");
                        })}
                      >
                        撤销
                      </Button>
                    ) : (
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() => void op.run(async () => {
                          await api(`/admin/subscriptions/${r.id}/restore`, empty, "POST", {});
                          onChanged("已恢复该订阅");
                        })}
                      >
                        恢复
                      </Button>
                    )}
                  </div>
                ),
              },
            ]}
          />
        )}
      </Resource>
      {renew && <ExtendSubscription id={renew.id} onClose={() => setRenew(undefined)} onSaved={() => {setRenew(undefined); onChanged("订阅已续期");}} />}
      <ErrorLine value={op.error} />
    </>
  );
}

function UserKeysPane({userID, revision, onChanged}: {userID: number; revision: number; onChanged: (msg: string) => void}) {
  const op = useAction();

  return (
    <>
          <RemoteTable
            path={"/admin/users/" + userID + "/keys"} schema={keyRow} revision={revision}
            label="用户名下 API Key"
            rowID={r => r.id}
            empty={<EmptyState title="该用户尚未创建 API Key" />}
            columns={[
              {
                id: "key",
                title: "Key / 消费",
                render: r => (
                  <div>
                    <strong>{r.name}</strong> <span className="muted code-mono text-fine">{r.masked}</span>
                    <div className="muted text-fine">
                      今日 {r.today_actual_cost === null ? "—" : smartMoney(r.today_actual_cost)} · 累计 {r.total_actual_cost === null ? "—" : smartMoney(r.total_actual_cost)}
                    </div>
                  </div>
                ),
              },
              {
                id: "group",
                title: "绑定分组",
                render: r => (
                  <GroupSelect
                      value={r.group_id ? String(r.group_id) : ""}
                      placeholder="未绑定分组"
                      onChange={next => void op.run(async () => {
                        const result = await api(`/admin/api-keys/${r.id}`, z.object({auto_granted_group_access: z.boolean(), granted_group_name: z.string().optional()}), "PUT", {group_id: next ? Number(next) : 0});
                        onChanged(`已更新密钥「${r.name}」的绑定分组` + (result.auto_granted_group_access ? "，并授予专属分组访问权限" : ""));
                      })}
                    />
                ),
              },
            ]}
          />
          <ErrorLine value={op.error} />
    </>
  );
}

function GrantEditor({user}: {user: number}) {
  const path = "/admin/grants/" + user;
  const [revision, revise] = useState(0);
  const [confirming, confirm] = useState(false);
  const [pending, setPending] = useState<{invites: number; budget_cents: number; amounts_cents: number[]}>();
  const [amountTags, setAmountTags] = useState<string[]>();
  const current = useResource(() => api(path, grantSchema), path, true, revision);
  const save = useAction();
  const revoke = useAction();
  const prepare = useAction();

  return (
    <>
      <Resource state={current}>
        {record => {
          const tags = amountTags ?? record.amounts_cents.map(a => String(a / 100));
          return (
            <Form
              key={revision}
              busy={save.busy}
              onSubmit={event => {
                const fields = new FormData(event.currentTarget);
                void prepare.run(async () => {
                  save.clearError();
                  setPending({
                    invites: Number(fields.get("invites")),
                    budget_cents: cents(String(fields.get("budget") || "0")),
                    amounts_cents: tags.map(cents),
                  });
                });
              }}
            >
              <FormSection title={`子管理员发码预算 · 剩 ${record.invites} 张邀请 · ${money(record.budget_cents / 100)}`}>
                <Field label="设定可用邀请码张数">
                  <div className="row-field"><Input type="number" suffix="张" name="invites" min="0" max="10000" defaultValue={record.invites} required /></div>
                </Field>
                <Field label="设定余额码总预算">
                  <div className="row-field"><Input name="budget" suffix="USD" inputMode="decimal" defaultValue={String(record.budget_cents / 100)} required /></div>
                </Field>
                <Field label="允许签发的单张面额（USD）">
                  <TagInput
                    value={tags}
                    placeholder="输入面额数字后按回车，如 5、10、20"
                    validate={v => {
                      try { cents(v); return undefined; } catch (e) { return e instanceof Error ? e.message : "面额格式有误"; }
                    }}
                    onChange={setAmountTags}
                  />
                </Field>
              </FormSection>
              <ErrorLine value={prepare.error} />
              <FormActions>
                <Button variant="destructive" size="sm" disabled={save.busy} onClick={() => { revoke.clearError(); confirm(true); }}>撤销授权</Button>
                <Button type="submit" size="sm" busy={save.busy}>保存授权</Button>
              </FormActions>
            </Form>
          );
        }}
      </Resource>
      {pending && (
        <StepUpDialog
          title="验证发码授权"
          labels={[`邀请码 ${pending.invites} 张`, `余额码预算 ${money(pending.budget_cents / 100)}`]}
          confirmLabel="验证并保存"
          busy={save.busy}
          error={save.error}
          fieldErrors={save.fieldErrors}
          onClose={() => setPending(undefined)}
          onConfirm={({totp}) =>
            void save.run(async () => {
              await api(path, empty, "PUT", {...pending, totp});
              setPending(undefined);
              setAmountTags(undefined);
              revise(n => n + 1);
              toast("已保存发码额度授权");
            })
          }
        />
      )}
      <ConfirmDialog
        open={confirming}
        title="撤销发码授权"
        description="该用户的发码授权与剩余预算将立即清零。"
        confirm="确认撤销"
        busy={revoke.busy}
        error={revoke.error}
        onClose={() => confirm(false)}
        onConfirm={() => void revoke.run(async () => {
          await api(path, empty, "DELETE");
          confirm(false);
          setAmountTags(undefined);
          revise(v => v + 1);
          toast("已撤销发码授权");
        })}
      />
    </>
  );
}

function CreateUserDialog({open, revision, onClose, onCreated}: {open: boolean; revision: number; onClose: () => void; onCreated: (u: User) => void}) {
  const formID = useId();
  const [allowedGroups, setAllowedGroups] = useState<number[]>([]);
  const op = useAction();

  return (
    <Overlay open={open} title="创建新用户" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>创建用户</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            const created = await api("/admin/users", userRow, "POST", {
              email: String(fd.get("email") ?? "").trim(),
              password: String(fd.get("password") ?? ""),
              username: String(fd.get("username") ?? "").trim(),
              balance: Number(fd.get("balance") || 0),
              concurrency: Number(fd.get("concurrency") || 5),
              notes: String(fd.get("notes") ?? "").trim(),
              ...(allowedGroups.length ? {allowed_groups: allowedGroups} : {}),
            });
            onCreated(created);
          });
        }}
      >
        <FormGrid>
          <Field label="登录邮箱"><Input name="email" type="email" required /></Field>
          <Field label="初始密码（至少 6 位）"><PasswordInput name="password" minLength={6} required /></Field>
          <Field label="显示昵称（可选）"><Input name="username" /></Field>
          <Field label="初始余额"><Input name="balance" type="number" suffix="USD" min="0" step="0.01" defaultValue="0" /></Field>
          <Field label="最大并发数"><Input name="concurrency" type="number" min="0" defaultValue="5" /></Field>
          <Field label="备注"><Input name="notes" /></Field>
        </FormGrid>
        <Field label="授权专属分组（可选）">
          <MultiGroupSelect privateOnly value={allowedGroups} revision={revision} onChange={setAllowedGroups} />
        </Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

function BatchUserLimitsDialog({open, userIDs, onClose, onSaved}: {open: boolean; userIDs: readonly number[]; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const op = useAction();
  return (
    <Overlay open={open} title={`批量设置 ${userIDs.length} 位用户限流`} onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>应用限流</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          const c = String(fd.get("concurrency") ?? "");
          const r = String(fd.get("rpm_limit") ?? "");
          void op.run(async () => {
            await api("/admin/users/batch-limits", z.object({affected: z.number()}), "POST", {
              user_ids: userIDs,
              ...(c !== "" ? {concurrency: Number(c)} : {}),
              ...(r !== "" ? {rpm_limit: Number(r)} : {}),
            });
            onSaved();
          });
        }}
      >
        <FormGrid>
          <Field label="覆写并发上限（留空不改）"><Input name="concurrency" type="number" min="0" /></Field>
          <Field label="覆写 RPM 上限（留空不改）"><Input name="rpm_limit" type="number" min="0" /></Field>
        </FormGrid>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

function AssignSubscriptionDialog({open, revision, onClose, onSaved, fixedUserID}: {open: boolean; revision: number; onClose: () => void; onSaved: () => void; fixedUserID?: number}) {
  return open ? <AssignSubscriptionForm revision={revision} fixedUserID={fixedUserID} onClose={onClose} onSaved={onSaved} /> : null;
}

function AssignSubscriptionForm({revision, onClose, onSaved, fixedUserID}: {revision: number; onClose: () => void; onSaved: () => void; fixedUserID?: number}) {
  const [userID, setUserID] = useState("");
  const op = useAction();
  return <GroupDialog title="分配订阅" source={{subscriptionOnly: true, revision}} activeOnly onClose={onClose} applyLabel="开通订阅" applyDisabled={!fixedUserID && !userID} busy={op.busy} error={op.error}
    onApply={([groupID], fields) => void op.run(async () => {
      await api("/admin/subscriptions/assign", adminSubscriptionRow, "POST", {
        user_id: fixedUserID ?? Number(userID), group_id: groupID, validity_days: Number(fields.get("validity_days")), notes: String(fields.get("notes") ?? "").trim(),
      });
      onSaved();
    })}>
    {!fixedUserID && <Field label="目标用户"><UserSelect value={userID} placeholder="搜索用户邮箱…" clearable={false} onChange={setUserID} /></Field>}
    <FormGrid>
      <Field label="有效天数"><Input name="validity_days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
      <Field label="备注（选填）"><Input name="notes" maxLength={200} /></Field>
    </FormGrid>
  </GroupDialog>;
}

function ExtendSubscription({id, onClose, onSaved}: {id: number; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const op = useAction();
  return <Overlay open title="订阅续期" onClose={onClose} busy={op.busy} footer={
          <><Button variant="outline" onClick={onClose}>取消</Button><Button type="submit" form={formID} busy={op.busy}>确认续期</Button></>
        }>
    <Form id={formID} busy={op.busy} onSubmit={event => {
      const days = Number(new FormData(event.currentTarget).get("days"));
      void op.run(async () => {await api(`/admin/subscriptions/${id}/extend`, adminSubscriptionRow, "POST", {days}); onSaved();});
    }}>
      <Field label="续期天数"><Input name="days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
      <ErrorLine value={op.error} />
    </Form>
  </Overlay>;
}
