import {useEffect, useMemo, useRef, useState} from "react";
import {Activity, Link2, Plus, Shield, Trash2, Wallet} from "lucide-react";
import {z} from "zod";
import {api, streamUsage} from "@/lib/http";
import {ccSwitchImportLink, magpieImportLink} from "@/lib/client-import";
import {compact, count, date, money, relativeUntil, smartMoney} from "@/lib/format";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {empty, grantSchema, keyGroup, keyRow, subscriptions, usageSummary, type Config} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  Badge, BatchBar, Button, ConfirmDialog, CopyButton, CopyValue, EmptyState, ErrorLine, FilterBar,
  FilterSlot, Form, InlineStack, Input, Inspector, MenuItem, MeterBar, MetricStrip, MoreActions,
  Overlay, PlanIdentity, PlanSummary, QuickPills, RefreshButton, RemoteTable, RowActions,
  SearchInput, SecretRevealDialog, Select, Status, Switch, WorkspaceContent, WorkspaceStage, toast,
  type Column,
} from "@/ui";
import {CreateKeyDialog, EditKeyBatch, SubAdminCodeBox} from "./key-dialogs";
import {KeyInspectorForm} from "./key-inspector";

const balanceSchema = z.object({balance: z.number().nullable()});
const keySecretSchema = z.object({key: z.string().min(1)});
const redemptionRecord = z.object({id: z.number(), type: z.string(), value: z.number(), used_at: z.string().nullable()});
const redemptionTypes: Record<string, string> = {balance: "余额", concurrency: "并发额度", subscription: "订阅", invitation: "邀请"};

export type KeyItem = z.infer<typeof keyRow>;
type KeyClient = "magpie" | "ccswitch";

export function KeysWorkspace({config, admin, queryString = ""}: {config: Config; admin: boolean; queryString?: string}) {
  const [historyOpen, setHistoryOpen] = useState(false);
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [revision, revise] = useState(0);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [groupFilter, setGroupFilter] = useState("");
  const [creating, setCreating] = useState(false);
  const [selected, setSelected] = useState<number[]>([]);
  const [editingBatch, setEditingBatch] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [inspected, setInspected] = useState<KeyItem>();
  const [pageRows, setPageRows] = useState<KeyItem[]>([]);
  const [deleting, setDeleting] = useState<KeyItem>();
  const deletionTrigger = useRef<HTMLElement>(null);
  const [secret, setSecret] = useState("");
  const [redeemCode, setRedeemCode] = useState("");
  const [referral, setReferral] = useState("");
  const createButton = useRef<HTMLButtonElement>(null);
  const debouncedSearch = useDebouncedValue(search.trim(), 240);

  const keysPath = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (statusFilter) p.set("status", statusFilter);
    if (groupFilter) p.set("group_id", groupFilter);
    const qs = p.toString();
    return "/keys" + (qs ? "?" + qs : "");
  }, [debouncedSearch, groupFilter, statusFilter]);

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
  const toggleAction = useAction();
  const deleteAction = useAction();
  const clientAction = useAction();
  const referralAction = useAction();
  const refresh = () => revise(v => v + 1);

  useEffect(() => {
    const inspectID = Number(initial.get("inspect") ?? 0);
    if (inspectID > 0 && pageRows.length > 0) {
      const found = pageRows.find(r => r.id === inspectID);
      if (found) setInspected(found);
    }
  }, [initial, pageRows]);

  async function transferKey(row: KeyItem, client: KeyClient) {
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

  const columns: Column<KeyItem>[] = [
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
        return (
          <PlanIdentity
            item={{
              id: row.group_id,
              name: g?.name ?? `分组 #${row.group_id}`,
              platform: g?.platform,
              status: g?.status,
              rate_multiplier: g?.rate_multiplier,
              health: g?.health,
              health_score: g?.health_score,
            }}
          />
        );
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
          return <MeterBar size="sm" value={(row.quota_used / row.quota) * 100} label={`${smartMoney(row.quota_used)}/${smartMoney(row.quota)}`} />;
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
          <MoreActions
            label={row.name + " 的更多操作"}
            busy={clientAction.busy}
            onOpenChange={(open, trigger) => { if (open) deletionTrigger.current = trigger; }}
          >
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
          {id: "balance", label: "个人可用余额", value: balance?.value ? money(balance.value.balance) : null, tone: "ok"},
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
              <Input required maxLength={128} placeholder="输入兑换码充值…" aria-label="兑换码" value={redeemCode} onChange={e => setRedeemCode(e.target.value)} />
              <Button type="submit" variant="outline" busy={redeemAction.busy}><Wallet aria-hidden />兑换</Button>
              <Button type="button" variant="ghost" onClick={() => setHistoryOpen(true)}>兑换历史</Button>
            </InlineStack>
          </Form>
        }
      />

      {historyOpen && (
        <Overlay open title="兑换历史" onClose={() => setHistoryOpen(false)}>
          <RemoteTable
            path="/redeem/history"
            schema={redemptionRecord}
            rowID={row => row.id}
            label="兑换历史"
            empty={<EmptyState title="暂无兑换记录" />}
            columns={[
              {id: "type", title: "类型", render: row => redemptionTypes[row.type] ?? "其他"},
              {id: "value", title: "入账", align: "number", render: row => row.type === "balance" ? money(row.value) : count.format(row.value)},
              {id: "used_at", title: "兑换时间", render: row => row.used_at ? date(row.used_at) : "未知"},
            ]}
          />
        </Overlay>
      )}

      <FilterBar sticky>
        <div className="obs-filter-row">
          <SearchInput value={search} onChange={setSearch} placeholder="按密钥名称搜索" ariaLabel="搜索密钥名称" />
          <QuickPills
            label="密钥状态"
            value={statusFilter}
            options={[
              {value: "", label: "全部"},
              {value: "active", label: "启用", tone: "ok"},
              {value: "inactive", label: "停用", tone: "bad"},
            ]}
            onChange={setStatusFilter}
          />
          <FilterSlot>
            <Select value={groupFilter} aria-label="按分组筛选" onChange={e => setGroupFilter(e.target.value)}>
              <option value="">全部分组</option>
              {(groups?.value ?? []).map(g => <option key={g.id} value={g.id}>{g.name}</option>)}
            </Select>
          </FilterSlot>
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
            <Button ref={createButton} onClick={() => { setSecret(""); setCreating(true); }}>
              <Plus aria-hidden />新建 API Key
            </Button>
          </div>
        </div>
        <ErrorLine value={redeemAction.error || toggleAction.error || clientAction.error || referralAction.error || deleteAction.error} />
        {config.custom_endpoints.map(endpoint => <CopyValue key={endpoint.endpoint} label={endpoint.name} value={endpoint.endpoint} />)}
        {referral && <CopyValue label="专属邀请注册链接" value={referral} />}
      </FilterBar>

      <SecretRevealDialog open={Boolean(secret)} title="API Key 已创建" label="API Key" value={secret} onClose={() => setSecret("")} />

      <BatchBar count={selected.length} onClear={() => setSelected([])}>
        <Button size="xs" variant="outline" onClick={() => setEditingBatch(true)}>批量编辑</Button>
        <Button
          size="xs"
          variant="outline"
          busy={toggleAction.busy}
          onClick={() => void toggleAction.run(async () => {
            await Promise.all(selected.map(id => api(`/keys/${id}`, empty, "PUT", {status: "active"})));
            setSelected([]);
            toast("已批量启用所选密钥");
            refresh();
          })}
        >
          批量启用
        </Button>
        <Button
          size="xs"
          variant="outline"
          busy={toggleAction.busy}
          onClick={() => void toggleAction.run(async () => {
            await Promise.all(selected.map(id => api(`/keys/${id}`, empty, "PUT", {status: "inactive"})));
            setSelected([]);
            toast("已批量停用所选密钥");
            refresh();
          })}
        >
          批量停用
        </Button>
        <Button size="xs" variant="destructive" busy={deleteAction.busy} onClick={() => setBatchDeleting(true)}>
          <Trash2 aria-hidden />批量删除
        </Button>
      </BatchBar>
      {editingBatch && (
        <EditKeyBatch
          ids={selected}
          onClose={() => setEditingBatch(false)}
          onApplied={(failed, succeeded) => {
            setSelected(failed);
            if (succeeded > 0) toast("已更新 " + succeeded + " 个密钥");
            if (!failed.length) setEditingBatch(false);
            refresh();
          }}
        />
      )}
      <div className="swift-pane-table">
        <RemoteTable
          key={keysPath}
          path={keysPath}
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

      <CreateKeyDialog
        open={creating}
        defaultGroupID={groups?.value?.[0]?.id ? String(groups.value[0].id) : ""}
        onClose={() => setCreating(false)}
        onCreated={createdSecret => {
          setSecret(createdSecret);
          setCreating(false);
          refresh();
        }}
      />

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
      <ConfirmDialog
        open={batchDeleting}
        title="批量删除 API Key"
        description={`确定删除已选的 ${selected.length} 个 API Key 吗？删除后使用这些密钥的客户端将立即失效。`}
        confirm="批量删除"
        busy={deleteAction.busy}
        error={deleteAction.error}
        onClose={() => setBatchDeleting(false)}
        onConfirm={() => void deleteAction.run(async () => {
          await Promise.all(selected.map(id => api(`/keys/${id}`, empty, "DELETE")));
          if (inspected && selected.includes(inspected.id)) setInspected(undefined);
          setSelected([]);
          setBatchDeleting(false);
          toast("已批量删除所选密钥");
          refresh();
        })}
      />
    </WorkspaceStage>
  );
}

export {KeysWorkspace as Keys};
