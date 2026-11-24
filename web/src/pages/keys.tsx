import { useId,useEffect,useMemo,useRef,useState } from "react";
import { Activity,Link2,Plus,Shield,Ticket,Trash2,Wallet } from "lucide-react";
import { z } from "zod";
import {
api,
ApiError,
empty,
grantSchema,
issue as issueSchema,
keyGroup,
keyRow,
streamUsage,
subscriptions,
usageSummary,
type Config,
} from "../api";
import { ccSwitchImportLink,magpieImportLink } from "../client-import";
import { cents,compact,count,date,money,relativeUntil,smartMoney } from "../format";
import { useAction,useResource } from "../requests";
import { navigateTo } from "../route";
import {
FormActions,
InlineStack,WorkspaceContent,FormSection,
Badge,
BatchBar,
Button,
CollapsibleSection,
ConfirmDialog,
CopyButton,
CopyValue,
PlanSummary,
PlanIdentity,
EmptyState,ErrorLine,
Feedback,
Field,
FilterBar,
Form,
FormGrid,
GroupSelect,
Input,
InsetGroup,
InsetRow,
Inspector,
MeterBar,
MetricStrip,
Overlay,
Panel,
PasswordInput,
RefreshButton,
Resource,
SecretRevealDialog,
Select,
Status,
Switch,
Textarea,
Toolbar,
WorkspaceStage,
toast,
type Column,
MoreActions,
RowActions,
MenuItem
} from "../ui";
import { RemoteTable } from "./remote-table";

const balanceSchema = z.object({balance: z.number().nullable()});
const keySecretSchema = z.object({key: z.string().min(1)});
const codeResultSchema = z.object({code: z.string().min(1)});
const redemptionRecord = z.object({id: z.number(), type: z.string(), value: z.number(), used_at: z.string().nullable()});
const redemptionTypes: Record<string, string> = {
  balance: "余额", concurrency: "并发额度", subscription: "订阅", invitation: "邀请",
};

type Key = z.infer<typeof keyRow>;
type Group = z.infer<typeof keyGroup>;
type Issue = z.infer<typeof issueSchema>;
type KeyClient = "magpie" | "ccswitch";

function parseLines(value: string) {
  return value.split(/\r?\n/).map(item => item.trim()).filter(Boolean);
}

function healthText(group: Group) {
  const labels: Record<Group["health"], string> = {
    healthy: "健康",
    warning: "预警",
    critical: "异常",
    unknown: "样本不足",
    unavailable: "无观测",
  };
  return labels[group.health] + (group.health_score === null ? "" : ` ${Math.round(group.health_score)}`);
}

export function KeysWorkspace({config, admin, queryString = ""}: {config: Config; admin: boolean; queryString?: string}) {
  const [historyOpen, setHistoryOpen] = useState(false);
  const formID = useId();
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [revision, revise] = useState(0);
  const [creating, setCreating] = useState(false);
  const [customKey, setCustomKey] = useState(false);
  const [selected, setSelected] = useState<number[]>([]);
  const [editingBatch, setEditingBatch] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState("");
  const [inspected, setInspected] = useState<Key>();
  const [pageRows, setPageRows] = useState<Key[]>([]);
  const [deleting, setDeleting] = useState<Key>();
  const deletionTrigger = useRef<HTMLElement>(null);
  const [secret, setSecret] = useState("");
  const [redeemCode, setRedeemCode] = useState("");
  const [referral, setReferral] = useState("");
  const createButton = useRef<HTMLButtonElement>(null);

  const balance = useResource(() => api("/balance", balanceSchema), "self-balance", true, revision);
  const plans = useResource(() => api("/subscriptions", z.array(subscriptions)), "self-subs", true, revision);
  const groups = useResource(() => api("/key-groups", z.array(keyGroup)), "self-key-groups", true, revision);
  const grant = useResource(() => api("/grant", grantSchema), "self-grant", !admin, revision);
  const initial7d = useResource(() => api("/usage/summary?period=7d&page=1", usageSummary), "self-usage-7d", true, revision);
  const [live7d, setLive7d] = useState<z.infer<typeof usageSummary>>();
  const recent = live7d ?? initial7d?.value;

  useEffect(() => {
    const controller = new AbortController();
    void streamUsage("/usage/stream?period=7d", setLive7d, () => {}, controller.signal);
    return () => controller.abort();
  }, [revision]);

  const redeemAction = useAction();
  const createAction = useAction();
  const toggleAction = useAction();
  const deleteAction = useAction();
  const clientAction = useAction();
  const referralAction = useAction();

  const refresh = () => revise(v => v + 1);

  useEffect(() => {
    if (creating && !selectedGroup && groups?.value?.length) {
      setSelectedGroup(String(groups.value[0].id));
    }
  }, [creating, groups?.value, selectedGroup]);

  useEffect(() => {
    const inspectID = Number(initial.get("inspect") ?? 0);
    if (inspectID > 0 && pageRows.length > 0) {
      const found = pageRows.find(r => r.id === inspectID);
      if (found) setInspected(found);
    }
  }, [initial, pageRows]);

  async function transferKey(row: Key, client: KeyClient) {
    await clientAction.run(async () => {
      const res = await api(`/keys/${row.id}/secret`, keySecretSchema);
      const baseUrl = config.api_base_url || config.official;
      const platform = groups?.value?.find(g => g.id === row.group_id)?.platform ?? "anthropic";
      window.location.assign(
        client === "magpie"
          ? magpieImportLink({name: row.name, baseUrl, platform, key: res.key})
          : ccSwitchImportLink({name: row.name, baseUrl, platform, key: res.key}),
      );
    });
  }

  const columns: Column<Key>[] = [
    {
      id: "name",
      title: "密钥名称",
      render: row => (
        <div>
          <strong>{row.name}</strong>
          <div className="muted code-mono text-fine">{row.masked}</div>
        </div>
      ),
    },
    {
      id: "group",
      title: "调用分组",
      hideWhenCompact: true,
      render: row => {
        const g = groups?.value?.find(item => item.id === row.group_id);
        if (!row.group_id) return <span className="muted">默认路由</span>;
        return <PlanIdentity item={{id: row.group_id, name: g?.name ?? `分组 #${row.group_id}`, platform: g?.platform,
          status: g?.status, rate_multiplier: g?.rate_multiplier, health: g?.health, health_score: g?.health_score}} />;
      },
    },
    {
      id: "usage",
      title: "扣费",
      align: "number",
      render: row => (
        <span className="key-usage-summary">
          <strong>{row.today_actual_cost === null ? "—" : smartMoney(row.today_actual_cost)}</strong>
          <span>{row.total_actual_cost === null ? "—" : smartMoney(row.total_actual_cost)}</span>
        </span>
      ),
    },
    {
      id: "limits",
      title: "额度与窗口限制",
      hideWhenNarrow: true,
      render: row => {
        if (row.quota > 0) {
          return (
            <MeterBar
              size="sm"
              value={(row.quota_used / row.quota) * 100}
              label={`${smartMoney(row.quota_used)}/${smartMoney(row.quota)}`}
            />
          );
        }
        const parts = [
          row.rate_limit_5h > 0 ? `5h ${smartMoney(row.usage_5h)}/${smartMoney(row.rate_limit_5h)}` : "",
          row.rate_limit_1d > 0 ? `1d ${smartMoney(row.usage_1d)}/${smartMoney(row.rate_limit_1d)}` : "",
          row.rate_limit_7d > 0 ? `7d ${smartMoney(row.usage_7d)}/${smartMoney(row.rate_limit_7d)}` : "",
        ].filter(Boolean);
        return parts.length ? <span className="code-mono text-fine">{parts.join(" · ")}</span> : <span className="muted">不限额</span>;
      },
    },
    {
      id: "ip",
      title: "IP 规则",
      hideWhenNarrow: true,
      render: row => {
        const total = row.ip_whitelist.length + row.ip_blacklist.length;
        return total ? <Badge variant="tag"><Shield aria-hidden />{total} 条</Badge> : <span className="muted">不限</span>;
      },
    },
    {
      id: "enabled",
      title: "启用",
      align: "center",
      render: row => (
        <Switch
          label={`${row.status === "active" ? "停用" : "启用"} ${row.name}`}
          checked={row.status === "active"}
          disabled={toggleAction.busy}
          onChange={checked => void toggleAction.run(async () => {
            await api(`/keys/${row.id}`, empty, "PUT", {status: checked ? "active" : "inactive"});
            refresh();
          })}
        />
      ),
    },
    {
      id: "actions",
      title: "操作",
      align: "end",
      render: row => (
        <RowActions>
          <CopyButton
            label={`复制 ${row.name}`}
            getText={async () => (await api(`/keys/${row.id}/secret`, keySecretSchema)).key}
            onCopied={() => toast(`已复制密钥「${row.name}」`)}
          />
          <MoreActions label={row.name + " 的更多操作"} busy={clientAction.busy}
            onOpenChange={(open, trigger) => { if (open) deletionTrigger.current = trigger; }}>
            <MenuItem onSelect={() => void transferKey(row, "magpie")}>导入到 Magpie</MenuItem>
            <MenuItem onSelect={() => void transferKey(row, "ccswitch")}>导入到 CC Switch</MenuItem>
            <MenuItem onSelect={() => navigateTo("usage", {user: admin ? "self" : undefined, key: row.id})}><Activity aria-hidden />查看用量</MenuItem>
            <MenuItem tone="bad" onSelect={() => { deleteAction.clearError(); setDeleting(row); }}><Trash2 aria-hidden />删除密钥</MenuItem>
          </MoreActions>
        </RowActions>
      ),
    },
  ];

  const activePlans = (plans?.value ?? []).filter(p => p.status === "active");
  const total7dTokens = recent
    ? recent.totals.input_tokens + recent.totals.output_tokens + recent.totals.cache_read_tokens + recent.totals.cache_creation_tokens
    : 0;
  const idx = inspected ? pageRows.findIndex(r => r.id === inspected.id) : -1;
  const hasSubAdminGrant = !admin && !!grant?.value && (grant.value.invites > 0 || grant.value.budget_cents > 0);

  return (
    <WorkspaceStage
      flush
      inspectorOpen={!!inspected}
      inspector={
        <Inspector
          open={!!inspected}
          title={inspected?.name ?? "密钥详情"}
          subtitle={inspected?.masked}
          badge={inspected ? <Status value={inspected.status} /> : undefined}
          onClose={() => setInspected(undefined)}
          onPrev={idx > 0 ? () => setInspected(pageRows[idx - 1]) : undefined}
          onNext={idx >= 0 && idx < pageRows.length - 1 ? () => setInspected(pageRows[idx + 1]) : undefined}
        >
          {inspected && (
            <KeyInspectorForm
              key={`key-${inspected.id}`}
              row={inspected}
              admin={admin}
              onSaved={() => {
                refresh();
                toast(`已更新密钥「${inspected.name}」`);
              }}
            />
          )}
        </Inspector>
      }
    >
      <MetricStrip
        flush
        items={[
          {
            id: "balance",
            label: "个人可用余额",
            value: balance?.value ? money(balance.value.balance) : null,
            tone: "ok",
          },
          {
            id: "cost7d",
            label: "近 7 天个人扣费",
            value: recent ? smartMoney(recent.totals.actual_cost) : null,
            spark: (recent?.days ?? []).map(d => d.actual_cost),
            tone: "accent",
            onClick: () => navigateTo("usage", admin ? {user: "self"} : undefined),
          },
          {
            id: "req7d",
            label: "近 7 天请求 / Token",
            value: recent ? count.format(recent.totals.requests) : null,
            hint: recent ? `${compact.format(total7dTokens)} Tokens` : undefined,
          },
          {
            id: "subs",
            label: "我的有效订阅",
            value: plans?.value ? `${activePlans.length} 个` : null,
            hint: activePlans[0] ? `${activePlans[0].group} · ${relativeUntil(activePlans[0].expires_at)}` : "暂无订阅",
          },
        ]}
        trailing={
          <Form
            busy={redeemAction.busy}
            onSubmit={() => void redeemAction.run(async () => {
              await api("/redeem", empty, "POST", {code: redeemCode});
              setRedeemCode("");
              toast("兑换成功，账户余额与订阅状态已刷新");
              refresh();
            })}
          >
            <InlineStack>
              <Input
                required
                maxLength={128}
                placeholder="输入兑换码充值…"
                aria-label="兑换码"
                value={redeemCode}
                onChange={e => setRedeemCode(e.target.value)}
              />
              <Button type="submit" variant="outline" busy={redeemAction.busy}>
                <Wallet aria-hidden />兑换
              </Button>
              <Button type="button" variant="ghost" onClick={() => setHistoryOpen(true)}>兑换历史</Button>
            </InlineStack>
          </Form>
        }
      />

      {historyOpen && <Overlay open title="兑换历史" onClose={() => setHistoryOpen(false)}>
        <RemoteTable path="/redeem/history" schema={redemptionRecord} rowID={row => row.id} label="兑换历史"
          empty={<EmptyState title="暂无兑换记录" />}
          columns={[
            {id: "type", title: "类型", render: row => redemptionTypes[row.type] ?? "其他"},
            {id: "value", title: "入账", align: "number", render: row => row.type === "balance" ? money(row.value) : count.format(row.value)},
            {id: "used_at", title: "兑换时间", render: row => row.used_at ? date(row.used_at) : "未知"},
          ]} />
      </Overlay>}

      <FilterBar sticky>
        <div className="obs-filter-row">
          <div className="obs-filter-spacer">
            {config.affiliate && (
              <Button
                variant="outline"
                busy={referralAction.busy}
                onClick={() => void referralAction.run(async () => {
                  const data = await api("/referral", z.object({url: z.string().url()}));
                  setReferral(data.url);
                })}
              >
                <Link2 aria-hidden />邀请返利链接
              </Button>
            )}
            <RefreshButton iconOnly size="default" variant="ghost" label="刷新密钥" onClick={refresh} />
            <Button
              ref={createButton}
              onClick={() => {
                createAction.clearError();
                setSecret("");
                setSelectedGroup("");
                setCreating(true);
              }}
            >
              <Plus aria-hidden />新建 API Key
            </Button>
          </div>
        </div>
        <ErrorLine value={redeemAction.error || toggleAction.error || clientAction.error || referralAction.error} />
        {config.custom_endpoints.map(endpoint => <CopyValue key={endpoint.endpoint} label={endpoint.name} value={endpoint.endpoint} />)}
        {referral && <CopyValue label="专属邀请注册链接" value={referral} />}
      </FilterBar>

      <SecretRevealDialog
        open={Boolean(secret)}
        title="API Key 已创建"
        label="API Key"
        value={secret}
        onClose={() => setSecret("")}
      />

      <BatchBar count={selected.length} onClear={() => setSelected([])}>
        <Button size="xs" variant="outline" onClick={() => setEditingBatch(true)}>批量编辑</Button>
      </BatchBar>
      {editingBatch && <EditKeyBatch ids={selected} onClose={() => setEditingBatch(false)} onApplied={(failed, succeeded) => {
        setSelected(failed);
        if (succeeded > 0) toast("已更新 " + succeeded + " 个密钥");
        if (!failed.length) setEditingBatch(false);
        refresh();
      }} />}
      <div className="swift-pane-table">
        <RemoteTable
          path="/keys"
          schema={keyRow}
          columns={columns}
          rowID={row => row.id}
          label="API Key"
          pinActions
          revision={revision}
          activeRowID={inspected?.id}
          selection={{selected, onChange: setSelected}}
          onRowClick={setInspected}
          onItemsLoaded={rows => {
            setPageRows(rows);
            if (inspected) {
              const updated = rows.find(r => r.id === inspected.id);
              if (updated) setInspected(updated);
            }
          }}
          empty={
            <EmptyState
              title="还没有 API Key"
              description="创建密钥后可一键导入 Magpie 或 CC Switch，并实时追踪单 Key 消耗。"
              action={<Button size="sm" onClick={() => setCreating(true)}><Plus aria-hidden />创建第一个密钥</Button>}
            />
          }
        />
      </div>

      {(((plans?.value?.length ?? 0) > 0) || ((hasSubAdminGrant || initial.get("tab") === "codes") && !admin)) && (
        <WorkspaceContent divided>
          {(plans?.value?.length ?? 0) > 0 && (
            <PlanSummary label="我的订阅" items={(plans?.value ?? []).map(row => ({...row, name: row.group}))} />
          )}
          {(hasSubAdminGrant || initial.get("tab") === "codes") && !admin && (
            <SubAdminCodeBox config={config} revision={revision} onIssued={refresh} />
          )}
        </WorkspaceContent>
      )}

      <Overlay open={creating} title="新建 API Key" onClose={() => setCreating(false)} busy={createAction.busy} footer={
        <>
          <Button variant="outline" onClick={() => setCreating(false)}>取消</Button>
          <Button type="submit" form={formID} busy={createAction.busy}>创建密钥</Button>
        </>
      }>
        <Form id={formID}
          busy={createAction.busy}
          onSubmit={event => {
            const fd = new FormData(event.currentTarget);
            const name = String(fd.get("name") ?? "").trim();
            const whitelist = parseLines(String(fd.get("ip_whitelist") ?? ""));
            const blacklist = parseLines(String(fd.get("ip_blacklist") ?? ""));
            const expDays = Number(fd.get("expires_in_days") || 0);
            void createAction.run(async () => {
              const res = await api("/keys", keySecretSchema, "POST", {
                name,
                ...(customKey ? {custom_key: String(fd.get("custom_key") ?? "").trim()} : {}),
                ...(selectedGroup ? {group_id: Number(selectedGroup)} : {}),
                ip_whitelist: whitelist,
                ip_blacklist: blacklist,
                quota: Number(fd.get("quota") || 0),
                ...(expDays > 0 ? {expires_in_days: expDays} : {}),
                rate_limit_5h: Number(fd.get("rate_limit_5h") || 0),
                rate_limit_1d: Number(fd.get("rate_limit_1d") || 0),
                rate_limit_7d: Number(fd.get("rate_limit_7d") || 0),
              });
              setSecret(res.key);
              setCreating(false);
              setCustomKey(false);
              refresh();
            });
          }}
        >
          <FormSection title="基础标识与路由">
            <Field label="密钥名称">
              <Input name="name" required maxLength={100} placeholder="例如：本地开发 / Claude Code" />
            </Field>
            <InsetRow label="使用自定义密钥"><Switch label="使用自定义密钥" checked={customKey} onChange={setCustomKey} /></InsetRow>
            {customKey && <Field label="自定义密钥"><PasswordInput name="custom_key" toggleLabel="自定义密钥" autoComplete="new-password" required minLength={16} maxLength={256} pattern="[A-Za-z0-9_\-]+" /></Field>}
            <Field label="调用分组">
              <GroupSelect admin={false} value={selectedGroup} placeholder="自动匹配默认分组" onChange={setSelectedGroup} />
            </Field>
          </FormSection>

          <CollapsibleSection title="预算限额与来源 IP 限制（可选）" summary="默认不限额">
            <FormGrid>
              <Field label="总额度上限 (USD)"><Input type="number" name="quota" min="0" step="0.01" defaultValue="0" /></Field>
              <Field label="有效天数（0 为永久）"><Input type="number" name="expires_in_days" min="0" defaultValue="0" /></Field>
              <Field label="5 小时限额 (USD)"><Input type="number" name="rate_limit_5h" min="0" step="0.01" defaultValue="0" /></Field>
              <Field label="24 小时限额 (USD)"><Input type="number" name="rate_limit_1d" min="0" step="0.01" defaultValue="0" /></Field>
              <Field label="7 天限额 (USD)"><Input type="number" name="rate_limit_7d" min="0" step="0.01" defaultValue="0" /></Field>
            </FormGrid>
            <FormGrid>
              <Field label="IP 白名单"><Textarea name="ip_whitelist" rows={2} placeholder="每行一个 IP 或 CIDR" /></Field>
              <Field label="IP 黑名单"><Textarea name="ip_blacklist" rows={2} placeholder="每行一个 IP 或 CIDR" /></Field>
            </FormGrid>
          </CollapsibleSection>

          <ErrorLine value={createAction.error} />
        </Form>
      </Overlay>

      <ConfirmDialog
        open={!!deleting}
        title="删除 API Key"
        description={`删除「${deleting?.name ?? ""}」后使用此密钥的客户端将立即失效。`}
        confirm="删除"
        busy={deleteAction.busy}
        error={deleteAction.error}
        finalFocus={() => deletionTrigger.current?.isConnected ? deletionTrigger.current : createButton.current}
        onClose={() => setDeleting(undefined)}
        onConfirm={() => {
          if (!deleting) return;
          void deleteAction.run(async () => {
            await api(`/keys/${deleting.id}`, empty, "DELETE");
            if (inspected?.id === deleting.id) setInspected(undefined);
            setDeleting(undefined);
            refresh();
          });
        }}
      />
    </WorkspaceStage>
  );
}

const batchKeyLimits = [["quota", "总额度"], ["rate_limit_5h", "5 小时限额"], ["rate_limit_1d", "24 小时限额"], ["rate_limit_7d", "7 天限额"]] as const;
const batchKeyIP = [["ip_whitelist", "IP 白名单"], ["ip_blacklist", "IP 黑名单"]] as const;

function EditKeyBatch({ids, onClose, onApplied}: {ids: number[]; onClose: () => void; onApplied: (failed: number[], succeeded: number) => void}) {
  const formID = useId();
  const action = useAction();
  const [group, setGroup] = useState("");
  const [expiry, setExpiry] = useState("");
  const [enabled, setEnabled] = useState<string[]>([]);
  const [failures, setFailures] = useState<string[]>([]);
  const enable = (name: string, label: string) => <InsetRow label={label}><Switch label={label} checked={enabled.includes(name)} onChange={next => setEnabled(values => next ? [...values, name] : values.filter(value => value !== name))} /></InsetRow>;
  return <Overlay open title="批量编辑密钥" onClose={onClose} busy={action.busy} footer={<>
    <Button variant="outline" onClick={onClose}>关闭</Button>
    <Button type="submit" form={formID} busy={action.busy} disabled={!enabled.length || !ids.length}>更新 {ids.length} 个密钥</Button>
  </>}>
    <Form id={formID} onSubmit={event => {
      const form = new FormData(event.currentTarget);
      void action.run(async () => {
        const updates: Record<string, unknown> = {};
        if (enabled.includes("group_id")) {
          if (!group) throw new Error("请选择新分组");
          updates.group_id = Number(group);
        }
        if (enabled.includes("status")) updates.status = form.get("status");
        if (enabled.includes("expires_at")) {
          if (expiry === "clear") updates.expires_at = "";
          else {
            const date = new Date(String(form.get("expires_at") ?? ""));
            if (!Number.isFinite(date.getTime())) throw new Error("请输入有效到期时间");
            updates.expires_at = date.toISOString();
          }
        }
        for (const [key] of batchKeyLimits) if (enabled.includes(key)) updates[key] = Number(form.get(key));
        for (const [key] of batchKeyIP) if (enabled.includes(key)) updates[key] = parseLines(String(form.get(key) ?? ""));
        const failed: number[] = [];
        const messages: string[] = [];
        for (let start = 0; start < ids.length; start += 5) {
          const batch = ids.slice(start, start + 5);
          const outcomes = await Promise.allSettled(batch.map(id => api("/keys/" + id, keyRow, "PUT", updates)));
          outcomes.forEach((result, index) => {
            if (result.status === "rejected") {
              failed.push(batch[index]);
              messages.push("#" + batch[index] + " · " + (result.reason instanceof Error ? result.reason.message : "更新失败"));
            }
          });
        }
        setFailures(messages);
        onApplied(failed, ids.length - failed.length);
      });
    }}>
      {enable("group_id", "修改分组")}
      {enabled.includes("group_id") && <Field label="目标分组"><GroupSelect admin={false} value={group} onChange={setGroup} clearable={false} /></Field>}
      {enable("status", "修改状态")}
      {enabled.includes("status") && <Field label="状态"><Select name="status"><option value="active">启用</option><option value="inactive">停用</option></Select></Field>}
      {enable("expires_at", "修改到期时间")}
      {enabled.includes("expires_at") && <>
        <Field label="有效期"><Select value={expiry} onChange={event => setExpiry(event.target.value)}><option value="">指定时间</option><option value="clear">永久有效</option></Select></Field>
        {expiry !== "clear" && <Field label="到期时间"><Input name="expires_at" type="datetime-local" required /></Field>}
      </>}
      {batchKeyLimits.map(([key, label]) => <FormSection key={key}>{enable(key, "修改" + label)}{enabled.includes(key) && <Field label={label}><Input name={key} type="number" min="0" step="any" required /></Field>}</FormSection>)}
      {batchKeyIP.map(([key, label]) => <FormSection key={key}>{enable(key, "修改" + label)}{enabled.includes(key) && <Field label={label}><Textarea name={key} rows={2} /></Field>}</FormSection>)}
      <ErrorLine value={action.error} />
      {failures.map(message => <ErrorLine key={message} value={message} />)}
    </Form>
  </Overlay>;
}

function KeyInspectorForm({row, admin, onSaved}: {row: Key; admin: boolean; onSaved: () => void}) {
  const save = useAction();
  const [name, setName] = useState(row.name);
  const [groupID, setGroupID] = useState(row.group_id ? String(row.group_id) : "");
  const [quota, setQuota] = useState(String(row.quota));
  const [limit5h, setLimit5h] = useState(String(row.rate_limit_5h));
  const [limit1d, setLimit1d] = useState(String(row.rate_limit_1d));
  const [limit7d, setLimit7d] = useState(String(row.rate_limit_7d));
  const [expiresAt, setExpiresAt] = useState(row.expires_at ? row.expires_at.slice(0, 10) : "");
  const [resetQuota, setResetQuota] = useState(false);
  const [resetWindows, setResetWindows] = useState(false);
  const [whitelist, setWhitelist] = useState(row.ip_whitelist.join("\n"));
  const [blacklist, setBlacklist] = useState(row.ip_blacklist.join("\n"));

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/keys/${row.id}`, empty, "PUT", {
          name: name.trim(),
          ...(groupID ? {group_id: Number(groupID)} : {clear_group: true}),
          quota: Number(quota || 0),
          rate_limit_5h: Number(limit5h || 0),
          rate_limit_1d: Number(limit1d || 0),
          rate_limit_7d: Number(limit7d || 0),
          ...(expiresAt ? {expires_at: `${expiresAt}T23:59:59Z`} : {clear_expires: true}),
          ...(resetQuota ? {reset_quota: true} : {}),
          ...(resetWindows ? {reset_rate_limit_usage: true} : {}),
          ip_whitelist: parseLines(whitelist),
          ip_blacklist: parseLines(blacklist),
        });
        setResetQuota(false);
        setResetWindows(false);
        onSaved();
      })}
    >
      <InsetGroup title="实时运行状态">
        <InsetRow label="今日 / 累计实际扣费">
          <span className="weir-digits">
            {row.today_actual_cost === null ? "—" : smartMoney(row.today_actual_cost)} / {row.total_actual_cost === null ? "—" : smartMoney(row.total_actual_cost)}
          </span>
        </InsetRow>
        <InsetRow label="当前并发连接"><span className="weir-digits">{row.current_concurrency}</span></InsetRow>
        <InsetRow label="最近调用">
          <span className="code-mono text-fine">{row.last_used_at ? date(row.last_used_at) : "尚未使用"}{row.last_used_ip ? ` · ${row.last_used_ip}` : ""}</span>
        </InsetRow>
        <InsetRow label="账单下钻">
          <Button size="xs" variant="outline" onClick={() => navigateTo("usage", {user: admin ? "self" : undefined, key: row.id})}>
            查看该 Key 趋势与明细 →
          </Button>
        </InsetRow>
      </InsetGroup>

      <FormSection title="名称与分组绑定">
        <Field label="名称">
          <Input required maxLength={100} value={name} onChange={e => setName(e.target.value)} />
        </Field>
        <Field label="调用分组">
          <GroupSelect admin={false} value={groupID} placeholder="不绑定特定分组" onChange={setGroupID} />
        </Field>
      </FormSection>

      <FormSection title="额度与窗口限流（0 表示不限）">
        <Field label="总额度 (USD)" hint={`已用 ${smartMoney(row.quota_used)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={quota} onChange={e => setQuota(e.target.value)} /></div>
        </Field>
        <Field label="5 小时限额 (USD)" hint={`当前 ${smartMoney(row.usage_5h)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={limit5h} onChange={e => setLimit5h(e.target.value)} /></div>
        </Field>
        <Field label="24 小时限额 (USD)" hint={`当前 ${smartMoney(row.usage_1d)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={limit1d} onChange={e => setLimit1d(e.target.value)} /></div>
        </Field>
        <Field label="7 天限额 (USD)" hint={`当前 ${smartMoney(row.usage_7d)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={limit7d} onChange={e => setLimit7d(e.target.value)} /></div>
        </Field>
        <Field label="到期日期（留空永不过期）">
          <div className="row-field"><Input type="date" value={expiresAt} onChange={e => setExpiresAt(e.target.value)} /></div>
        </Field>
        <InsetRow label="保存时清零累计已用额度">
          <Switch label="重置总额度计数" checked={resetQuota} onChange={setResetQuota} />
        </InsetRow>
        <InsetRow label="保存时清零 5h/1d/7d 窗口计数">
          <Switch label="重置窗口计数" checked={resetWindows} onChange={setResetWindows} />
        </InsetRow>
      </FormSection>

      <CollapsibleSection title="来源 IP 黑白名单" summary={`${row.ip_whitelist.length + row.ip_blacklist.length} 条规则`}>
        <Field label="白名单（每行一个 IP 或 CIDR）">
          <Textarea rows={2} value={whitelist} onChange={e => setWhitelist(e.target.value)} />
        </Field>
        <Field label="黑名单（每行一个 IP 或 CIDR）">
          <Textarea rows={2} value={blacklist} onChange={e => setBlacklist(e.target.value)} />
        </Field>
      </CollapsibleSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="submit" busy={save.busy}>保存修改</Button>
      </FormActions>
    </Form>
  );
}

function SubAdminCodeBox({config, revision, onIssued}: {config: Config; revision: number; onIssued: () => void}) {
  const [kind, setKind] = useState("");
  const [value, setValue] = useState("");
  const [secret, setSecret] = useState("");
  const pending = useRef<{id: string; input: string} | null>(null);
  const operation = useAction();
  const grant = useResource(() => api("/grant", grantSchema), "subadmin-grant", true, revision);

  const issueCols: Column<Issue>[] = [
    {id: "created", title: "时间", render: row => <span className="code-mono">{date(row.created_at * 1000)}</span>},
    {id: "kind", title: "类型", render: row => <Badge variant="tag">{row.kind === "balance" ? "余额码" : "注册邀请码"}</Badge>},
    {id: "amount", title: "面额", align: "number", render: row => row.amount_cents ? money(row.amount_cents / 100) : "—"},
    {id: "state", title: "状态", align: "center", render: row => <Status value={row.state} />},
  ];

  return (
    <Panel title="授权发码工作区" subtitle="使用管理员授予你的独立预算签发邀请码或余额码">
      <Resource state={grant}>
        {permit => {
          const amounts = permit.amounts_cents.filter(a => a <= permit.budget_cents);
          const canInvite = config.invitation && permit.invites > 0;
          const canBalance = amounts.length > 0;
          const selected = kind && (kind === "invitation" ? canInvite : canBalance) ? kind : canInvite ? "invitation" : "balance";
          if (!canInvite && !canBalance) return <span className="muted">当前暂无可用发码授权额度。</span>;
          return (
            <Form
              busy={operation.busy}
              onSubmit={() => void operation.run(async () => {
                setSecret("");
                const amount = selected === "invitation" ? 0 : Number(value || amounts[0]);
                const fp = `${selected}/${amount}`;
                const opID = pending.current?.input === fp ? pending.current.id : crypto.randomUUID();
                pending.current = {id: opID, input: fp};
                try {
                  const res = await api("/issues", codeResultSchema, "POST", {operation_id: opID, kind: selected, amount_cents: cents(String(amount / 100))});
                  setSecret(res.code);
                  pending.current = null;
                } catch (err) {
                  if (err instanceof ApiError && ["issue_rejected", "grant_exhausted", "invalid_input", "not_allowed"].includes(err.code)) {
                    pending.current = null;
                  }
                  throw err;
                } finally {
                  onIssued();
                }
              })}
            >
              <Toolbar>
                <Field label="码种">
                  <Select value={selected} onChange={e => { setKind(e.target.value); setValue(""); }}>
                    {canInvite && <option value="invitation">注册邀请码（剩 {permit.invites} 张）</option>}
                    {canBalance && <option value="balance">余额兑换码（剩 {money(permit.budget_cents / 100)}）</option>}
                  </Select>
                </Field>
                {selected === "balance" && (
                  <Field label="允许面额">
                    <Select value={value || amounts[0]} onChange={e => setValue(e.target.value)}>
                      {amounts.map(a => <option key={a} value={a}>{money(a / 100)}</option>)}
                    </Select>
                  </Field>
                )}
                <Button type="submit" busy={operation.busy}><Ticket aria-hidden />签发一张</Button>
              </Toolbar>
              <ErrorLine value={operation.error} />
            </Form>
          );
        }}
      </Resource>
      {secret && <CopyValue label="新签发的码" value={secret} />}
      <RemoteTable path="/issues" schema={issueSchema} columns={issueCols} rowID={r => r.operation_id} label="我的签发流水" revision={revision} />
    </Panel>
  );
}

export {KeysWorkspace as Keys};
