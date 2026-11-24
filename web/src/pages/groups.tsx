import { useId,useEffect,useMemo,useRef,useState } from "react";
import {
Copy as DuplicateIcon,Plus,
Trash2
} from "lucide-react";
import { z } from "zod";
import {
adminGroupRow,api,channelRowSchema,
compositeRouteSchema,
empty
} from "../api";
import { count,smartMoney } from "../format";
import { useAction,useDebouncedValue,useResource } from "../requests";
import { useModelCatalog } from "../model-catalog";
import { navigateTo } from "../route";
import {FormActions, 
FormSection,Stack,InlineStack,
Badge,Button,
CollapsibleSection,
ConfirmDialog,
DataTable,
EmptyState,
EntityLink,
ErrorLine,
Feedback,
Field,
FilterBar,Form,
FormGrid,
GroupSelect,
Input,
InsetGroup,
InsetRow,
Inspector,ModelSelect,
MultiModelSelect,
MultiGroupSelect,
Overlay,PlatformBadge,PlatformSelect,RefreshButton,Resource,
SearchInput,
SegmentBar,
Segmented,
Select,
Status,Switch,UserSelect,
type Column,FilterSlot,WorkspaceStage
} from "../ui";
import { RemoteTable } from "./remote-table";
import { groupBriefSchema,type ChannelRow,type CompositeRoute,type GroupRow,type PricingRule } from "./pool-shared";

type GroupView = "groups" | "pricing";

export function GroupsWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [view, setView] = useState<GroupView>(() => initial.get("view") === "pricing" ? "pricing" : "groups");
  const [platform, setPlatform] = useState("");
  const [search, setSearch] = useState("");
  const [revision, revise] = useState(0);
  const [inspect, setInspect] = useState<{kind: "group" | "channel"; id: number} | undefined>(() => {
    const id = Number(initial.get("inspect") ?? 0);
    return id > 0 ? {kind: initial.get("view") === "pricing" ? "channel" : "group", id} : undefined;
  });
  const [groupRows, setGroupRows] = useState<GroupRow[]>([]);
  const [createOpen, setCreateOpen] = useState(false);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);

  useEffect(() => {
    const params = new URLSearchParams(queryString);
    const id = Number(params.get("inspect") ?? 0);
    if (id > 0) {
      const pricing = params.get("view") === "pricing";
      setView(pricing ? "pricing" : "groups");
      setInspect({kind: pricing ? "channel" : "group", id});
    }
  }, [queryString]);

  const allGroups = useResource(() => api("/admin/groups/all", z.array(groupBriefSchema)), "group-names", view === "pricing", revision);

  const groupQuery = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (platform) p.set("platform", platform);
    const qs = p.toString();
    return "/admin/groups" + (qs ? "?" + qs : "");
  }, [debouncedSearch, platform]);

  const groupCols: Column<GroupRow>[] = [
    {
      id: "group",
      title: "调度分组",
      render: row => (
        <div>
          <InlineStack density="compact">
            <strong className="compact-text" title={row.name}>{row.name}</strong>
            {row.is_exclusive && <Badge variant="tag">私有</Badge>}
            {row.subscription_type === "subscription" && <Badge variant="soft" tone="info">订阅组</Badge>}
          </InlineStack>
          {row.description && <div className="muted text-fine">{row.description}</div>}
        </div>
      ),
    },
    {id: "platform", title: "平台", hideWhenCompact: true, render: row => <PlatformBadge platform={row.platform} />},
    {id: "rate", title: "倍率", align: "number", render: row => `${row.rate_multiplier}×`},
    {
      id: "capacity",
      title: "账号（可用 / 限流 / 异常）",
      hideWhenNarrow: true,
      render: row => (
        <Button size="xs" variant="ghost" onClick={event => { event.stopPropagation(); navigateTo("accounts", {group: row.id}); }}>
          <SegmentBar
            compact
            segments={[
              {label: "可用", value: row.active_account_count, tone: "ok"},
              {label: "限流", value: row.rate_limited_account_count, tone: "warn"},
              {label: "异常", value: row.error_account_count, tone: "bad"},
            ]}
          />
          {count.format(row.account_count)} 个
        </Button>
      ),
    },
    {
      id: "health",
      title: "24h 健康",
      hideWhenNarrow: true,
      render: row => row.health_score !== null
        ? <Badge variant="soft" tone={row.health === "healthy" ? "ok" : row.health === "critical" ? "bad" : row.health === "warning" ? "warn" : "neutral"}>{Math.round(row.health_score)} 分</Badge>
        : <span className="muted">无样本</span>,
    },
    {id: "cost", title: "今日 / 累计", align: "number", hideWhenCompact: true, render: row => `${smartMoney(row.today_cost)} / ${smartMoney(row.total_cost)}`},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
  ];

  const channelCols: Column<ChannelRow>[] = [
    {
      id: "name",
      title: "调价模板",
      render: row => (
        <div>
          <strong className="compact-text" title={row.name}>{row.name}</strong>
          {row.description && <div className="muted text-fine">{row.description}</div>}
        </div>
      ),
    },
    {
      id: "groups",
      title: "作用分组",
      hideWhenNarrow: true,
      grow: true,
      render: row => (
        <InlineStack density="compact" wrap>
          {row.group_ids.length ? row.group_ids.map(gid => {
            const g = allGroups?.value?.find(item => item.id === gid);
            return <EntityLink key={gid} kind="group" id={gid} label={g?.name ?? `#${gid}`} onClick={() => { setView("groups"); setInspect({kind: "group", id: gid}); }} />;
          }) : <span className="muted">全部分组</span>}
        </InlineStack>
      ),
    },
    {id: "rules", title: "自定义价目", align: "number", render: row => `${row.model_pricing.length} 组`},
    {id: "source", title: "计费模型", render: row => <Badge variant="tag">{row.billing_model_source === "upstream" ? "上游模型" : "请求模型"}</Badge>},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
  ];

  const groupIndex = inspect?.kind === "group" ? groupRows.findIndex(row => row.id === inspect.id) : -1;

  return (
    <>
      <WorkspaceStage
        flush
        inspectorOpen={!!inspect}
        inspector={
          <Inspector
            open={!!inspect}
            title={inspect?.kind === "group" ? groupRows[groupIndex]?.name ?? "调度分组" : "调价模板"}
            subtitle={inspect ? `${inspect.kind === "group" ? "调度分组" : "调价模板"} · #${inspect.id}` : undefined}
            onClose={() => setInspect(undefined)}
            onPrev={groupIndex > 0 ? () => setInspect({kind: "group", id: groupRows[groupIndex - 1].id}) : undefined}
            onNext={groupIndex >= 0 && groupIndex < groupRows.length - 1 ? () => setInspect({kind: "group", id: groupRows[groupIndex + 1].id}) : undefined}
          >
            {inspect?.kind === "group" && (
              <GroupInspector key={`grp-${inspect.id}`} id={inspect.id} revision={revision} onChanged={refresh}
                onDeleted={() => { setInspect(undefined); refresh(); }} />
            )}
            {inspect?.kind === "channel" && (
              <ChannelInspector key={`chn-${inspect.id}`} id={inspect.id} revision={revision} onChanged={refresh}
                onDeleted={() => { setInspect(undefined); refresh(); }} />
            )}
          </Inspector>
        }
      >
        <FilterBar sticky>
          <div className="obs-filter-row">
            <Segmented label="分组视图" value={view} options={[["groups", "分组"], ["pricing", "模型调价"]] as const}
              onChange={next => { setView(next); setInspect(undefined); }} />
            <SearchInput value={search} onChange={setSearch} placeholder="搜索名称" ariaLabel={view === "groups" ? "搜索分组" : "搜索调价模板"} />
            {view === "groups" && <FilterSlot><PlatformSelect value={platform} includeComposite onChange={setPlatform} /></FilterSlot>}
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新" onClick={refresh} />
              <Button onClick={() => setCreateOpen(true)}><Plus aria-hidden />{view === "groups" ? "新建分组" : "新建调价模板"}</Button>
            </div>
          </div>
        </FilterBar>

        <div className="swift-pane-table">
          {view === "groups" ? (
            <RemoteTable
              key={groupQuery}
              path={groupQuery}
              schema={adminGroupRow}
              columns={groupCols}
              rowID={row => row.id}
              label="调度分组"
              revision={revision}
              activeRowID={inspect?.kind === "group" ? inspect.id : undefined}
              onRowClick={row => setInspect({kind: "group", id: row.id})}
              onItemsLoaded={setGroupRows}
              empty={<EmptyState title="还没有调度分组" action={<Button size="sm" onClick={() => setCreateOpen(true)}><Plus aria-hidden />新建分组</Button>} />}
            />
          ) : (
            <RemoteTable
              key={debouncedSearch}
              path={"/admin/channels" + (debouncedSearch ? "?search=" + encodeURIComponent(debouncedSearch) : "")}
              schema={channelRowSchema}
              columns={channelCols}
              rowID={row => row.id}
              label="调价模板"
              revision={revision}
              activeRowID={inspect?.kind === "channel" ? inspect.id : undefined}
              onRowClick={row => setInspect({kind: "channel", id: row.id})}
              empty={
                <EmptyState
                  title="没有调价模板"
                  description="不建模板时按分组倍率和官方单价计费。"
                  action={<Button size="sm" onClick={() => setCreateOpen(true)}><Plus aria-hidden />新建调价模板</Button>}
                />
              }
            />
          )}
        </div>
      </WorkspaceStage>

      <CreateGroupDialog
        open={createOpen && view === "groups"}
        onClose={() => setCreateOpen(false)}
        onCreated={grp => { setCreateOpen(false); refresh(); setInspect({kind: "group", id: grp.id}); }}
      />
      <CreateChannelDialog
        open={createOpen && view === "pricing"}
        revision={revision}
        onClose={() => setCreateOpen(false)}
        onCreated={ch => { setCreateOpen(false); refresh(); setInspect({kind: "channel", id: ch.id}); }}
      />
    </>
  );
}

function GroupInspector({
  id,
  revision,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const [tab, setTab] = useState<"core" | "composite" | "rates">("core");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [hint, setHint] = useState("");
  const detail = useResource(() => api(`/admin/groups/${id}`, adminGroupRow), `grp-detail-${id}`, true, revision);
  const dup = useAction();
  const del = useAction();

  return (
    <Resource state={detail}>
      {grp => (
        <>
          <InlineStack justify="between" wrap>
            <Segmented
              label="分组详情分页"
              value={tab}
              options={[
                ["core", "核心与白名单"],
                ...(grp.platform === "composite" ? ([["composite", "复合路由"]] as const) : []),
                ["rates", "专属倍率"],
              ]}
              onChange={v => setTab(v as "core" | "composite" | "rates")}
            />
            <Button
              size="icon-xs"
              variant="outline"
              title="复制分组"
              aria-label="复制分组"
              busy={dup.busy}
              onClick={() => void dup.run(async () => {
                await api(`/admin/groups/${grp.id}/duplicate`, adminGroupRow, "POST", {operation_id: crypto.randomUUID()});
                setHint("已复制生成新分组");
                onChanged();
              })}
            >
              <DuplicateIcon />
            </Button>
          </InlineStack>

          {hint && <Feedback tone="success">{hint}</Feedback>}
          <ErrorLine value={dup.error} />

          {tab === "core" && (
            <GroupCoreForm
              grp={grp}
              revision={revision}
              onSaved={() => { setHint("分组配置已保存"); onChanged(); }}
              onRequestDelete={() => setConfirmDelete(true)}
            />
          )}

          {tab === "composite" && <CompositeRoutesEditor groupID={grp.id} revision={revision} onChanged={onChanged} />}

          {tab === "rates" && <GroupUserRatesEditor groupID={grp.id} revision={revision} onChanged={onChanged} />}

          <ConfirmDialog
            open={confirmDelete}
            title="删除调度分组"
            description={`确定删除分组「${grp.name}」吗？`}
            confirm="删除"
            busy={del.busy}
            error={del.error}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void del.run(async () => {
              await api(`/admin/groups/${grp.id}`, empty, "DELETE");
              setConfirmDelete(false);
              onDeleted();
            })}
          />
        </>
      )}
    </Resource>
  );
}

function GroupCoreForm({
  grp,
  revision,
  onSaved,
  onRequestDelete,
}: {
  grp: GroupRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(grp.name);
  const [desc, setDesc] = useState(grp.description);
  const [rate, setRate] = useState(String(grp.rate_multiplier));
  const [rpm, setRpm] = useState(String(grp.rpm_limit));
  const [subType, setSubType] = useState(grp.subscription_type);
  const [exclusive, setExclusive] = useState(grp.is_exclusive);
  const [active, setActive] = useState(grp.status === "active");
  const [fallbackID, setFallbackID] = useState(grp.fallback_group_id ? String(grp.fallback_group_id) : "");
  const [allowlistOn, setAllowlistOn] = useState(grp.model_allowlist.enabled);
  const [models, setModels] = useState<string[]>(grp.model_allowlist.models);
  const [dailyLimit, setDailyLimit] = useState(grp.daily_limit_usd !== null ? String(grp.daily_limit_usd) : "");
  const [weeklyLimit, setWeeklyLimit] = useState(grp.weekly_limit_usd !== null ? String(grp.weekly_limit_usd) : "");
  const [monthlyLimit, setMonthlyLimit] = useState(grp.monthly_limit_usd !== null ? String(grp.monthly_limit_usd) : "");
  const [peakOn, setPeakOn] = useState(grp.peak_rate_enabled);
  const [peakStart, setPeakStart] = useState(grp.peak_start || "09:00");
  const [peakEnd, setPeakEnd] = useState(grp.peak_end || "23:00");
  const [peakRate, setPeakRate] = useState(String(grp.peak_rate_multiplier || 1));

  const candidates = useModelCatalog(`/admin/groups/${grp.id}/model-candidates?platform=${encodeURIComponent(grp.platform)}`, revision, allowlistOn);

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/admin/groups/${grp.id}`, adminGroupRow, "PUT", {
          name: name.trim(),
          description: desc,
          status: active ? "active" : "inactive",
          is_exclusive: exclusive,
          subscription_type: subType,
          rate_multiplier: Number(rate || 1),
          rpm_limit: Number(rpm || 0),
          ...(fallbackID ? {fallback_group_id: Number(fallbackID)} : {clear_fallback_group: true}),
          model_allowlist: {enabled: allowlistOn, models},
          ...(subType === "subscription" ? {
            ...(dailyLimit ? {daily_limit_usd: Number(dailyLimit)} : {clear_daily_limit: true}),
            ...(weeklyLimit ? {weekly_limit_usd: Number(weeklyLimit)} : {clear_weekly_limit: true}),
            ...(monthlyLimit ? {monthly_limit_usd: Number(monthlyLimit)} : {clear_monthly_limit: true}),
          } : {}),
          peak_rate_enabled: peakOn,
          ...(peakOn ? {peak_start: peakStart, peak_end: peakEnd, peak_rate_multiplier: Number(peakRate || 1)} : {}),
        });
        onSaved();
      })}
    >
      <FormSection title="核心计费与准入">
        <InsetRow label="启用分组调度"><Switch label="启用分组" checked={active} onChange={setActive} /></InsetRow>
        <Field label="分组名称"><Input required value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="计费模式">
          <Segmented
            label="计费模式"
            value={subType === "subscription" ? "subscription" : "standard"}
            options={[["standard", "按量余额"], ["subscription", "套餐订阅"]] as const}
            onChange={setSubType}
          />
        </Field>
        <Field label="基础计费倍率">
          <div className="row-field"><Input type="number" min="0" step="0.01" value={rate} onChange={e => setRate(e.target.value)} /></div>
        </Field>
        <InsetRow label="专属分组（仅授权用户可见）">
          <Switch label="专属分组" checked={exclusive} onChange={setExclusive} />
        </InsetRow>
        <Field label="故障兜底分组">
          <GroupSelect value={fallbackID} placeholder="不配置兜底" onChange={setFallbackID} />
        </Field>
      </FormSection>

      <FormSection title="模型访问白名单">
        <InsetRow label="启用模型白名单过滤">
          <Switch label="启用模型白名单" checked={allowlistOn} onChange={setAllowlistOn} />
        </InsetRow>
        {allowlistOn && (
          <Field label="允许调用的模型列表">
            <MultiModelSelect label="模型白名单" placeholder="添加模型" value={models} catalog={candidates} onChange={setModels} allowCustom />
          </Field>
        )}
      </FormSection>

      <CollapsibleSection title="订阅额度、RPM 与错峰倍率策略" summary={subType === "subscription" ? "订阅限额生效中" : "高级策略"}>
        <FormGrid>
          <Field label="组级 RPM 限制"><Input type="number" min="0" value={rpm} onChange={e => setRpm(e.target.value)} /></Field>
          {subType === "subscription" && <>
          <Field label="日限额 USD"><Input type="number" min="0" step="0.01" value={dailyLimit} onChange={e => setDailyLimit(e.target.value)} placeholder="不限" /></Field>
          <Field label="周限额 USD"><Input type="number" min="0" step="0.01" value={weeklyLimit} onChange={e => setWeeklyLimit(e.target.value)} placeholder="不限" /></Field>
          <Field label="月限额 USD"><Input type="number" min="0" step="0.01" value={monthlyLimit} onChange={e => setMonthlyLimit(e.target.value)} placeholder="不限" /></Field>
          </>}
        </FormGrid>
        <InlineStack justify="between" wrap>
          <span className="text-meta">启用高峰时段动态倍率</span>
          <Switch label="启用高峰时段倍率" checked={peakOn} onChange={setPeakOn} />
        </InlineStack>
        {peakOn && (
          <FormGrid cols={3}>
            <Field label="开始 (HH:MM)"><Input value={peakStart} onChange={e => setPeakStart(e.target.value)} /></Field>
            <Field label="结束 (HH:MM)"><Input value={peakEnd} onChange={e => setPeakEnd(e.target.value)} /></Field>
            <Field label="高峰倍率"><Input type="number" min="0" step="0.01" value={peakRate} onChange={e => setPeakRate(e.target.value)} /></Field>
          </FormGrid>
        )}
        <Field label="分组说明"><Input value={desc} onChange={e => setDesc(e.target.value)} /></Field>
      </CollapsibleSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除分组</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存分组</Button>
      </FormActions>
    </Form>
  );
}

function CompositeRoutesEditor({groupID, revision, onChanged}: {groupID: number; revision: number; onChanged: () => void}) {
  const candidates = useModelCatalog(`/admin/groups/${groupID}/model-candidates`, revision);
  const [localRev, setLocalRev] = useState(0);
  const [previewModel, setPreviewModel] = useState("");
  const [previewResult, setPreviewResult] = useState<string>("");
  const routes = useResource(
    () => api(`/admin/groups/${groupID}/composite-routes`, z.array(compositeRouteSchema)),
    `comp-routes-${groupID}`,
    true,
    revision + localRev,
  );
  const op = useAction();

  return (
    <>
      <FormSection title="路由规则在线命中预览">
        <Field label="输入模型名称实时测试路由决策">
          <div className="field-action-row">
            <ModelSelect label="路由预览模型" value={previewModel} onChange={setPreviewModel} placeholder="选择或输入模型" catalog={candidates} allowCustom />
            <Button
              size="sm"
              variant="outline"
              disabled={!previewModel.trim()}
              busy={op.busy}
              onClick={() => void op.run(async () => {
                const res = await api(
                  `/admin/groups/${groupID}/composite-routes/preview`,
                  z.object({matched: z.boolean(), target_platform: z.string(), upstream_model: z.string(), source: z.string(), reason: z.string().optional()}),
                  "POST",
                  {model: previewModel.trim(), endpoint: "any"},
                );
                setPreviewResult(
                  res.matched
                    ? `命中 ${res.target_platform} → ${res.upstream_model || previewModel.trim()}（来源：${res.source}）`
                    : `未命中路由${res.reason ? `：${res.reason}` : ""}`,
                );
              })}
            >
              测试命中
            </Button>
          </div>
        </Field>
        {previewResult && <InsetRow label="决策结果"><span className="code-mono text-fine">{previewResult}</span></InsetRow>}
      </FormSection>

      <FormSection title="新增模型转发规则">
        <FormSection title="规则参数">
          <Form
            busy={op.busy}
            onSubmit={e => {
              const fd = new FormData(e.currentTarget);
              const formEl = e.currentTarget;
              void op.run(async () => {
                await api(`/admin/groups/${groupID}/composite-routes`, compositeRouteSchema, "POST", {
                  public_model: String(fd.get("public_model") ?? "").trim(),
                  match_type: String(fd.get("match_type") ?? "prefix"),
                  target_platform: String(fd.get("target_platform") ?? "anthropic"),
                  upstream_model: String(fd.get("upstream_model") ?? "").trim(),
                  endpoint: String(fd.get("endpoint") ?? "any"),
                  priority: Number(fd.get("priority") || 10),
                  enabled: true,
                });
                formEl.reset();
                setLocalRev(v => v + 1);
                onChanged();
              });
            }}
          >
            <FormGrid>
              <Field label="匹配模型 / 前缀"><Input name="public_model" required placeholder="claude-" /></Field>
              <Field label="匹配方式">
                <Select name="match_type" defaultValue="prefix">
                  <option value="prefix">前缀匹配 (prefix)</option>
                  <option value="exact">精确匹配 (exact)</option>
                </Select>
              </Field>
              <Field label="目标上游平台">
                <Select name="target_platform" defaultValue="anthropic">
                  <option value="anthropic">Anthropic</option>
                  <option value="openai">OpenAI</option>
                  <option value="gemini">Gemini</option>
                  <option value="antigravity">Antigravity</option>
                  <option value="grok">Grok</option>
                  <option value="deepseek">DeepSeek</option>
                </Select>
              </Field>
              <Field label="改写上游模型（可选）"><Input name="upstream_model" placeholder="留空透传原名" /></Field>
              <Field label="适用端点">
                <Select name="endpoint" defaultValue="any">
                  <option value="any">全部端点 (any)</option>
                  <option value="messages">messages</option>
                  <option value="responses">responses</option>
                  <option value="chat_completions">chat_completions</option>
                </Select>
              </Field>
              <Field label="优先级"><Input name="priority" type="number" defaultValue="10" /></Field>
            </FormGrid>
            <FormActions>
              <Button type="submit" size="xs" busy={op.busy}><Plus aria-hidden />添加路由规则</Button>
            </FormActions>
          </Form>
        </FormSection>
      </FormSection>

      <Resource state={routes}>
        {rows => (
          <DataTable
            label="复合路由规则"
            rows={rows}
            rowID={r => r.id}
            empty={<EmptyState title="尚未添加复合路由规则" description="未匹配规则时将按模型名前缀自动推断目标平台。" />}
            columns={[
              {
                id: "rule",
                title: "匹配规则 → 目标平台",
                render: (r: CompositeRoute) => (
                  <div className="code-mono text-fine">
                    <strong>{r.public_model}</strong> ({r.match_type}) → {r.target_platform}
                    {r.upstream_model ? ` / ${r.upstream_model}` : ""}
                  </div>
                ),
              },
              {id: "prio", title: "优先级", align: "number", render: r => r.priority},
              {
                id: "del",
                title: "",
                align: "end",
                render: r => (
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    title="删除路由"
                    aria-label="删除路由"
                    onClick={() => void op.run(async () => {
                      await api(`/admin/groups/${groupID}/composite-routes/${r.id}`, empty, "DELETE");
                      setLocalRev(v => v + 1);
                    })}
                  >
                    <Trash2 />
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Resource>
      <ErrorLine value={op.error} />
    </>
  );
}

const groupRateRowSchema = z.object({
  user_id: z.number(),
  user_email: z.string(),
  rate_multiplier: z.number().nullable(),
});

function GroupUserRatesEditor({groupID, revision, onChanged}: {groupID: number; revision: number; onChanged: () => void}) {
  const [localRev, setLocalRev] = useState(0);
  const [targetUser, setTargetUser] = useState("");
  const [rateVal, setRateVal] = useState("1");
  const rows = useResource(() => api(`/admin/groups/${groupID}/rate-multipliers`, z.array(groupRateRowSchema)), `grp-rates-${groupID}`, true, revision + localRev);
  const op = useAction();

  return (
    <>
      <FormSection title="设置单用户专属分组倍率">
        <FormSection title="选择用户与覆写倍率">
          <Stack>
            <Field label="搜索并选择用户…"><UserSelect value={targetUser} placeholder="搜索并选择用户…" onChange={v => setTargetUser(v)} /></Field>
            <InlineStack>
              <Field label="专属倍率"><Input type="number" min="0" step="0.01" value={rateVal} onChange={e => setRateVal(e.target.value)} aria-label="专属倍率" /></Field>
              <Button
                size="sm"
                disabled={!targetUser}
                busy={op.busy}
                onClick={() => void op.run(async () => {
                  await api(`/admin/groups/${groupID}/rate-multipliers`, empty, "PUT", {
                    entries: [{user_id: Number(targetUser), rate_multiplier: Number(rateVal || 1)}],
                  });
                  setTargetUser("");
                  setLocalRev(v => v + 1);
                  onChanged();
                })}
              >
                应用倍率
              </Button>
            </InlineStack>
          </Stack>
        </FormSection>
      </FormSection>

      <Resource state={rows}>
        {items => (
          <DataTable
            label="组内专属倍率"
            rows={items}
            rowID={r => r.user_id}
            empty={<EmptyState title="暂无专属用户倍率" />}
            columns={[
              {id: "user", title: "用户", render: r => <EntityLink kind="user" id={r.user_id} label={r.user_email || `#${r.user_id}`} />},
              {id: "rate", title: "专属倍率", align: "number", render: r => r.rate_multiplier !== null ? `${r.rate_multiplier}×` : "默认"},
            ]}
          />
        )}
      </Resource>
      <ErrorLine value={op.error} />
    </>
  );
}

function ChannelInspector({
  id,
  revision,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const detail = useResource(() => api(`/admin/channels/${id}`, channelRowSchema), `chn-detail-${id}`, true, revision);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const del = useAction();

  return (
    <Resource state={detail}>
      {ch => (
        <>
          <ChannelEditForm ch={ch} revision={revision} onSaved={onChanged} onRequestDelete={() => setConfirmDelete(true)} />
          <ConfirmDialog
            open={confirmDelete}
            title="删除调价模板"
            description={`确定删除调价模板「${ch.name}」吗？`}
            confirm="删除"
            busy={del.busy}
            error={del.error}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void del.run(async () => {
              await api(`/admin/channels/${ch.id}`, empty, "DELETE");
              setConfirmDelete(false);
              onDeleted();
            })}
          />
        </>
      )}
    </Resource>
  );
}

function ChannelEditForm({
  ch,
  revision,
  onSaved,
  onRequestDelete,
}: {
  ch: ChannelRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(ch.name);
  const [desc, setDesc] = useState(ch.description);
  const [active, setActive] = useState(ch.status === "active");
  const [groupIDs, setGroupIDs] = useState<number[]>(ch.group_ids);
  const [pricing, setPricing] = useState<PricingRule[]>(ch.model_pricing);
  const [draftModel, setDraftModel] = useState("");
  const [draftPlatform, setDraftPlatform] = useState("openai");
  const [draftIn, setDraftIn] = useState("");
  const [draftOut, setDraftOut] = useState("");
  const pricingModels = useModelCatalog(`/admin/channels/sync-models?platform=${encodeURIComponent(draftPlatform)}`, revision);
  const referencePrice = useAction();
  const priceSelection = useRef(0);

  const changePricingModel = (model: string) => {
    priceSelection.current++;
    referencePrice.clearError();
    setDraftModel(model);
    setDraftIn("");
    setDraftOut("");
  };

  async function fillDefaultPrice() {
    if (!draftModel.trim()) return;
    const selection = priceSelection.current;
    const def = await api(
      `/admin/channels/model-pricing?model=${encodeURIComponent(draftModel.trim())}`,
      z.object({found: z.boolean(), input_price: z.number().optional(), output_price: z.number().optional()}),
    );
    if (selection === priceSelection.current && def.found) {
      if (def.input_price !== undefined) setDraftIn(String(def.input_price * 1_000_000));
      if (def.output_price !== undefined) setDraftOut(String(def.output_price * 1_000_000));
    }
  }

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/admin/channels/${ch.id}`, channelRowSchema, "PUT", {
          name: name.trim(),
          description: desc,
          status: active ? "active" : "disabled",
          group_ids: groupIDs,
          model_pricing: pricing,
        });
        onSaved();
      })}
    >
      <FormSection title="调价模板范围">
        <InsetRow label="启用模板"><Switch label="启用调价模板" checked={active} onChange={setActive} /></InsetRow>
        <Field label="模板名称"><Input required value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="作用分组">
          <MultiGroupSelect value={groupIDs} revision={revision} onChange={setGroupIDs} />
        </Field>
        <Field label="说明"><Input value={desc} onChange={e => setDesc(e.target.value)} /></Field>
      </FormSection>

      <FormSection title="按百万 Token 模型单价覆写 (USD / 1M)">
        {pricing.map((rule, idx) => (
          <InsetRow
            key={idx}
            label={rule.models.join(", ")}
            hint={`入 $${((rule.input_price ?? 0) * 1_000_000).toFixed(2)}/M · 出 $${((rule.output_price ?? 0) * 1_000_000).toFixed(2)}/M`}
          >
            <Button size="icon-xs" variant="ghost" title="移除价目" aria-label="移除价目" onClick={() => setPricing(pricing.filter((_, i) => i !== idx))}>
              <Trash2 />
            </Button>
          </InsetRow>
        ))}
        <FormSection title="新增模型价目规则">
          <Stack>
            <div className="field-action-row">
              <Field label="模型平台"><Select value={draftPlatform} onChange={e => { setDraftPlatform(e.target.value); changePricingModel(""); }} aria-label="模型平台">
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="gemini">Gemini</option>
                <option value="grok">Grok</option>
                <option value="deepseek">DeepSeek</option>
              </Select></Field>
              <Field label="选择或输入模型"><ModelSelect label="定价模型" value={draftModel} onChange={changePricingModel} placeholder="选择或输入模型" catalog={pricingModels} allowCustom /></Field>
              <Button type="button" variant="outline" disabled={!draftModel} busy={referencePrice.busy} onClick={() => void referencePrice.run(fillDefaultPrice)}>读取参考价</Button>
            </div>
            <ErrorLine value={referencePrice.error} />
            <div className="field-action-row">
              <Field label="输入 $/1M"><Input type="number" min="0" step="0.01" value={draftIn} onChange={e => setDraftIn(e.target.value)} placeholder="输入 $/1M" /></Field>
              <Field label="输出 $/1M"><Input type="number" min="0" step="0.01" value={draftOut} onChange={e => setDraftOut(e.target.value)} placeholder="输出 $/1M" /></Field>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!draftModel.trim()}
                onClick={() => {
                  setPricing([
                    ...pricing,
                    {
                      platform: draftPlatform,
                      models: [draftModel.trim()],
                      billing_mode: "token",
                      input_price: Number(draftIn || 0) / 1_000_000,
                      output_price: Number(draftOut || 0) / 1_000_000,
                    },
                  ]);
                  changePricingModel("");
                }}
              >
                加入列表
              </Button>
            </div>
          </Stack>
        </FormSection>
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除模板</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存调价模板</Button>
      </FormActions>
    </Form>
  );
}

function CreateGroupDialog({open, onClose, onCreated}: {open: boolean; onClose: () => void; onCreated: (grp: GroupRow) => void}) {
  const formID = useId();
  const [platform, setPlatform] = useState("openai");
  const [subType, setSubType] = useState("standard");
  const [exclusive, setExclusive] = useState(false);
  const op = useAction();

  return (
    <Overlay open={open} title="新建调度分组" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>创建分组</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            const created = await api("/admin/groups", adminGroupRow, "POST", {
              name: String(fd.get("name") ?? "").trim(),
              description: String(fd.get("description") ?? "").trim(),
              platform,
              subscription_type: subType,
              rate_multiplier: Number(fd.get("rate_multiplier") || 1),
              is_exclusive: exclusive,
            });
            onCreated(created);
          });
        }}
      >
        <Field label="分组名称"><Input name="name" required maxLength={100} placeholder="例如 Claude-Pro-主力组" /></Field>
        <FormGrid>
          <Field label="所属平台">
            <PlatformSelect value={platform} includeComposite clearable={false} onChange={setPlatform} />
          </Field>
          <Field label="基础计费倍率"><Input name="rate_multiplier" type="number" min="0" step="0.01" defaultValue="1" required /></Field>
        </FormGrid>
        <Field label="计费类型">
          <Segmented label="计费类型" value={subType as "standard" | "subscription"} options={[["standard", "按量计费"], ["subscription", "套餐订阅"]] as const} onChange={setSubType} />
        </Field>
        <Field label="分组说明（可选）"><Input name="description" maxLength={200} /></Field>
        <InsetGroup>
          <InsetRow label="设为专属分组" hint="仅授权给指定用户或开通订阅后可见">
            <Switch label="专属分组" checked={exclusive} onChange={setExclusive} />
          </InsetRow>
        </InsetGroup>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

function CreateChannelDialog({open, revision, onClose, onCreated}: {open: boolean; revision: number; onClose: () => void; onCreated: (ch: ChannelRow) => void}) {
  const formID = useId();
  const [groupIDs, setGroupIDs] = useState<number[]>([]);
  const op = useAction();

  return (
    <Overlay open={open} title="新建渠道调价模板" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>创建并配置价目</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            const created = await api("/admin/channels", channelRowSchema, "POST", {
              name: String(fd.get("name") ?? "").trim(),
              description: String(fd.get("description") ?? "").trim(),
              billing_model_source: String(fd.get("billing_model_source") ?? "requested"),
              group_ids: groupIDs,
            });
            onCreated(created);
          });
        }}
      >
        <Field label="模板名称"><Input name="name" required maxLength={100} placeholder="企业专属阶梯价目" /></Field>
        <Field label="计费模型名称基准">
          <Select name="billing_model_source" defaultValue="requested">
            <option value="requested">按客户端请求模型名 (requested)</option>
            <option value="upstream">按实际发往上游模型名 (upstream)</option>
            <option value="response_model">按上游响应返回模型名 (response_model)</option>
          </Select>
        </Field>
        <Field label="绑定调度分组"><MultiGroupSelect value={groupIDs} revision={revision} onChange={setGroupIDs} /></Field>
        <Field label="说明（可选）"><Input name="description" maxLength={200} /></Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
