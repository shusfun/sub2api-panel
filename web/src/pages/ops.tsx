import { useId,useEffect,useMemo,useState } from "react";
import { Bell,Plus,ShieldAlert,Trash2,Wrench } from "lucide-react";
import { z } from "zod";
import {
account,
adminOverview,
api,
empty,
opsAlertEventSchema,
opsAlertRuleSchema,
opsConcurrencySchema,
opsLiveSchema,
opsErrorRowSchema,
opsOverviewSchema,
opsRequestRowSchema,
opsSettingsBundleSchema,
streamOps,
type UsageStreamStatus,
} from "../api";
import { compact,count,date,durationText,smartMoney,uptimeText } from "../format";
import { useAction,useDebouncedValue,useResource } from "../requests";
import { navigateTo } from "../route";
import { useChartPalette } from "../ui/chart";
import {Stack, FormActions, 
InlineStack,WorkspaceContent,FormSection,
AccountSelect,
Badge,
Button,
CollapsibleSection,
ConfirmDialog,
DataTable,
EmptyState,
EntityLink,
ErrorLine,
Feedback,
Field,
FilterBar,
FilterSlot,
Form,
FormGrid,
GroupSelect,
MetricTrend,
Input,
InsetGroup,
InsetRow,
Inspector,
MeterBar,
MetricStrip,
Overlay,
Panel,
PlatformBadge,
PlatformSelect,
QuickPills,
RankedBarList,
RefreshButton,
Resource,
SearchInput,
SegmentBar,
Segmented,
Select,
Status,
StatusBanner,
StatusDot,
Switch,
TimingBar,
TraceId,
UserSelect,
WorkspaceStage,
type Column,
type HeroChartTab,
} from "../ui";
import { RemoteTable } from "./remote-table";

type OpsTab = "live" | "errors" | "alerts";
type ErrorSourceTab = "request-errors" | "upstream-errors" | "requests";
type OpsErrorRow = z.infer<typeof opsErrorRowSchema>;
type OpsRequestRow = z.infer<typeof opsRequestRowSchema>;
type AlertRule = z.infer<typeof opsAlertRuleSchema>;

const timeRanges = [
  {value: "5m", label: "5m"},
  {value: "30m", label: "30m"},
  {value: "1h", label: "1h"},
  {value: "6h", label: "6h"},
  {value: "24h", label: "24h"},
] as const;

const streamLabel: Record<UsageStreamStatus, string> = {
  connecting: "连接中", live: "实时更新", reconnecting: "正在重连", paused: "后台已暂停", denied: "实时流不可用",
};

const metricTypeOptions = [
  ["error_rate", "客户端请求错误率 (%)"],
  ["upstream_error_rate", "上游渠道错误率 (%)"],
  ["success_rate", "整体可用率 SLA (%)"],
  ["concurrency_queue_depth", "并发排队深度"],
  ["account_error_count", "异常上游账号数"],
  ["account_rate_limited_count", "限流上游账号数"],
  ["overload_account_count", "过载上游账号数"],
] as const;

export function OpsWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [tab, setTab] = useState<OpsTab>(() => {
    const t = initial.get("tab");
    if (t === "errors" || t === "requests" || initial.has("request_id") || initial.has("account_id") || initial.has("user_id")) return "errors";
    if (t === "alerts") return "alerts";
    return "live";
  });
  const [errorSource, setErrorSource] = useState<ErrorSourceTab>(() =>
    initial.get("tab") === "requests" || initial.has("request_id") ? "requests" : "request-errors",
  );
  const [timeRange, setTimeRange] = useState("1h");
  const [platform, setPlatform] = useState("");
  const [groupID, setGroupID] = useState(() => initial.get("group_id") ?? "");
  const [accountID, setAccountID] = useState(() => initial.get("account_id") ?? "");
  const [userID, setUserID] = useState(() => initial.get("user_id") ?? "");
  const [search, setSearch] = useState(() => initial.get("request_id") ?? "");
  const [revision, revise] = useState(0);
  const [live, setLive] = useState<{scope: string; snapshot: z.infer<typeof opsLiveSchema>}>();
  const [streamState, setStreamState] = useState<UsageStreamStatus>("connecting");
  const [inspectedError, setInspectedError] = useState<OpsErrorRow>();
  const [errorRows, setErrorRows] = useState<OpsErrorRow[]>([]);
  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const palette = useChartPalette();

  useEffect(() => {
    const params = new URLSearchParams(queryString);
    if (params.has("request_id")) {
      setTab("errors");
      setErrorSource("requests");
      setSearch(params.get("request_id") ?? "");
    } else if (params.has("account_id")) {
      setTab("errors");
      setAccountID(params.get("account_id") ?? "");
    } else if (params.has("user_id")) {
      setTab("errors");
      setUserID(params.get("user_id") ?? "");
    }
  }, [queryString]);

  const scopeParams = useMemo(() => {
    const p = new URLSearchParams({time_range: timeRange});
    if (platform) p.set("platform", platform);
    if (groupID) p.set("group_id", groupID);
    return p.toString();
  }, [groupID, platform, timeRange]);

  const overview = useResource(() => api("/admin/overview", adminOverview), "admin-overview", true, revision);
  const opsOverview = useResource(() => api("/admin/ops/overview?" + scopeParams, opsOverviewSchema), "ops-overview:" + scopeParams, true, revision);
  const initialConc = useResource(() => api("/admin/ops/concurrency?" + scopeParams, opsConcurrencySchema), "ops-conc:" + scopeParams, true, revision);
  const events = useResource(() => api("/admin/ops/alert-events?status=firing", z.array(opsAlertEventSchema)), "ops-firing-alerts", true, revision);

  useEffect(() => {
    const controller = new AbortController();
    void streamOps("/admin/ops/stream?" + scopeParams, snapshot => setLive({scope: scopeParams, snapshot}), setStreamState, controller.signal);
    return () => controller.abort();
  }, [scopeParams, revision]);

  const snapshot = live?.scope === scopeParams ? live.snapshot : undefined;
  const conc = snapshot ?? (initialConc?.stale ? undefined : initialConc?.value);
  const dash = overview?.value;
  const ops = snapshot?.charts ?? (opsOverview?.stale ? undefined : opsOverview?.value);
  const firingCount = events?.value?.length ?? 0;
  const badAccounts = (dash?.accounts.error ?? 0) + (dash?.accounts.rate_limited ?? 0) + (dash?.accounts.overloaded ?? 0);
  const totalInUse = (conc?.platforms ?? []).reduce((s, p) => s + p.current_in_use, 0);
  const totalWaiting = (conc?.platforms ?? []).reduce((s, p) => s + p.waiting_in_queue, 0);

  const sla = ops?.overview.sla;
  const uptime = dash?.traffic && dash.traffic.uptime > 0 ? uptimeText(dash.traffic.uptime) : "";
  const attention = [
    firingCount > 0 ? `${firingCount} 条告警正在触发` : "",
    badAccounts > 0 ? `${badAccounts} 个上游账号异常或限流` : "",
    totalWaiting > 0 ? `${totalWaiting} 个请求在排队` : "",
    sla !== undefined && sla < 99 ? `成功率降到 ${sla.toFixed(1)}%` : "",
  ].filter(Boolean);

  const recovery = useAction();
  const [recoveryHint, setRecoveryHint] = useState("");

  const heroTabs = useMemo<HeroChartTab[]>(() => {
    const pts = ops?.throughput_points ?? [];
    const errPts = ops?.error_points ?? [];
    const timeLabels = pts.map(p => p.bucket_start.slice(11, 16) || p.bucket_start.slice(5, 16));
    const trend7d = dash?.trend ?? [];
    const buckets = ops?.latency_buckets ?? [];

    return [
      {
        id: "throughput",
        label: "实时吞吐 QPS",
        value: conc ? `${conc.qps.current.toFixed(1)} req/s` : ops ? `${ops.overview.qps.current.toFixed(1)} req/s` : null,
        hint: conc ? `峰值 ${conc.qps.peak.toFixed(1)} · 并发 ${totalInUse}${totalWaiting > 0 ? ` (排队 ${totalWaiting})` : ""}` : undefined,
        accent: palette.c1,
        categories: timeLabels,
        format: v => compact.format(v),
        series: [
          {name: "QPS", values: pts.map(p => p.qps), color: palette.c1, kind: "area"},
        ],
      },
      {
        id: "sla",
        label: "错误与限流",
        value: ops ? count.format(ops.overview.error_count_total) : null,
        hint: ops ? `上游 429×${ops.overview.upstream_429_count} · 529×${ops.overview.upstream_529_count}` : undefined,
        accent: ops && ops.overview.sla < 98 ? palette.c6 : palette.c4,
        categories: errPts.map(p => p.bucket_start.slice(11, 16) || p.bucket_start.slice(5, 16)),
        format: v => `${Math.round(v)}`,
        series: [
          {name: "上游 429 限流", values: errPts.map(p => p.upstream_429_count), color: palette.c5, kind: "bar"},
          {name: "SLA 故障", values: errPts.map(p => p.error_count_sla), color: palette.c6, kind: "bar"},
        ],
      },
      {
        id: "cost",
        label: "近 7 天实际扣费",
        value: trend7d.length ? smartMoney(trend7d.reduce((sum, d) => sum + d.actual_cost, 0)) : null,
        hint: dash?.traffic ? `累计 ${smartMoney(dash.traffic.total_cost)} · 今日 ${compact.format(dash.traffic.today_tokens)} Tok` : undefined,
        accent: palette.c2,
        categories: trend7d.map(d => d.date.slice(5)),
        format: smartMoney,
        series: [{name: "日实际扣费", values: trend7d.map(d => d.actual_cost), color: palette.c2, kind: "area"}],
      },
      {
        id: "latency",
        label: "响应延迟分布",
        value: ops?.overview.duration.avg_ms != null ? durationText(ops.overview.duration.avg_ms) : null,
        hint: dash ? `在线号池 ${dash.accounts.normal}/${dash.accounts.total} · ${dash.users.active} 活跃用户` : undefined,
        accent: palette.c3,
        categories: buckets.map(b => b.range),
        format: v => count.format(v),
        series: [{name: "请求分布", values: buckets.map(b => b.count), color: palette.c3, kind: "bar"}],
      },
      {
        id: "concurrency", label: "并发与排队", value: conc?.enabled ? count.format(totalInUse) : null,
        categories: (conc?.platforms ?? []).map(item => item.platform),
        series: [
          {name: "并发占用", values: (conc?.platforms ?? []).map(item => conc?.enabled ? item.current_in_use : null), color: palette.c1, kind: "bar"},
          {name: "排队", values: (conc?.platforms ?? []).map(item => conc?.enabled ? item.waiting_in_queue : null), color: palette.c5, kind: "bar"},
        ],
      },
    ];
  }, [conc, dash, ops, palette, totalInUse, totalWaiting]);

  const errorListPath = useMemo(() => {
    const p = new URLSearchParams({time_range: timeRange});
    if (platform) p.set("platform", platform);
    if (groupID) p.set("group_id", groupID);
    if (accountID) p.set("account_id", accountID);
    if (userID) p.set("user_id", userID);
    if (debouncedSearch) {
      if (errorSource === "requests") p.set("request_id", debouncedSearch);
      else p.set("q", debouncedSearch);
    }
    return `/admin/ops/${errorSource}?${p.toString()}`;
  }, [accountID, debouncedSearch, errorSource, groupID, platform, timeRange, userID]);

  const errorColumns: Column<OpsErrorRow>[] = [
    {id: "time", title: "时间", render: row => <span className="code-mono">{date(row.created_at)}</span>},
    {
      id: "status",
      title: "状态码 / 阶段",
      render: row => (
        <InlineStack density="compact">
          <Badge variant="soft" tone={row.status_code >= 500 ? "bad" : "warn"}>{row.status_code}</Badge>
          {row.upstream_status_code ? <span className="muted code-mono">→ {row.upstream_status_code}</span> : null}
          <span className="muted text-fine">{row.phase || row.type}</span>
        </InlineStack>
      ),
    },
    {id: "model", title: "模型", render: row => <span className="compact-model code-mono" title={row.model || row.requested_model}>{row.model || row.requested_model || "—"}</span>},
    {
      id: "entities",
      title: "关联用户 / 分组 / 上游账号",
      render: row => (
        <InlineStack density="compact" wrap>
          {row.user_id && <EntityLink kind="user" id={row.user_id} label={row.user_email || `#${row.user_id}`} />}
          {row.group_id && <EntityLink kind="group" id={row.group_id} label={row.group_name || `分组 #${row.group_id}`} />}
          {row.account_id && <EntityLink kind="account" id={row.account_id} label={row.account_name || `账号 #${row.account_id}`} />}
        </InlineStack>
      ),
    },
    {id: "trace", title: "请求编号", render: row => <TraceId value={row.request_id} />},
    {id: "state", title: "状态", align: "center", render: row => <Status value={row.resolved ? "resolved" : "error"} />},
  ];

  const requestColumns: Column<OpsRequestRow>[] = [
    {id: "time", title: "时间", render: row => <span className="code-mono">{date(row.created_at)}</span>},
    {
      id: "kind",
      title: "状态码",
      render: row => <Badge variant="soft" tone={row.kind === "error" ? "bad" : "ok"}>{row.status_code ?? (row.kind === "error" ? "错误" : "200")}</Badge>,
    },
    {id: "platform", title: "平台 / 模型", render: row => <InlineStack density="compact"><PlatformBadge platform={row.platform} /><span className="compact-model code-mono" title={row.model}>{row.model}</span></InlineStack>},
    {
      id: "links",
      title: "路由归属",
      render: row => (
        <InlineStack density="compact" wrap>
          {row.user_id && <EntityLink kind="user" id={row.user_id} />}
          {row.group_id && <EntityLink kind="group" id={row.group_id} />}
          {row.account_id && <EntityLink kind="account" id={row.account_id} />}
        </InlineStack>
      ),
    },
    {id: "ttft", title: "首字 TTFT", align: "number", render: row => durationText(row.first_token_ms)},
    {id: "dur", title: "总耗时", align: "number", render: row => durationText(row.duration_ms)},
    {id: "req", title: "Trace", render: row => <TraceId value={row.request_id} />},
  ];

  const errIdx = inspectedError ? errorRows.findIndex(r => r.id === inspectedError.id) : -1;
  const inspectorOpen = tab === "errors" && !!inspectedError;

  return (
    <WorkspaceStage
      flush
      inspectorOpen={inspectorOpen}
      inspector={
        <Inspector
          open={inspectorOpen}
          title={inspectedError ? `${inspectedError.status_code} · ${inspectedError.phase || inspectedError.type || "请求异常"}` : "错误诊断"}
          subtitle={inspectedError ? date(inspectedError.created_at) : undefined}
          badge={inspectedError ? <PlatformBadge platform={inspectedError.platform} /> : undefined}
          onClose={() => { setInspectedError(undefined); setRecoveryHint(""); }}
          onPrev={errIdx > 0 ? () => setInspectedError(errorRows[errIdx - 1]) : undefined}
          onNext={errIdx >= 0 && errIdx < errorRows.length - 1 ? () => setInspectedError(errorRows[errIdx + 1]) : undefined}
          busy={recovery.busy}
        >
          {inspectedError && (
            <>
              <InsetGroup title="阶段耗时瀑布 (Instruments)">
                <InsetRow label="各阶段响应分解" vertical>
                  <TimingBar
                    phases={[
                      {label: "鉴权校验", ms: inspectedError.auth_latency_ms, tone: "cyan"},
                      {label: "路由调度", ms: inspectedError.routing_latency_ms, tone: "violet"},
                      {label: "首字 TTFT", ms: inspectedError.time_to_first_token_ms, tone: "accent"},
                      {label: "上游总耗时", ms: inspectedError.upstream_latency_ms ?? inspectedError.response_latency_ms, tone: "warn"},
                    ]}
                  />
                </InsetRow>
              </InsetGroup>

              <InsetGroup title="结构化诊断摘要">
                <InsetRow label="客户端 / 上游状态码">
                  <span className="weir-digits">
                    HTTP {inspectedError.status_code}
                    {inspectedError.upstream_status_code ? ` → 上游 ${inspectedError.upstream_status_code}` : ""}
                  </span>
                </InsetRow>
                <InsetRow label="错误归属与分类">
                  <Badge variant="soft" tone={inspectedError.is_business_limited ? "warn" : "bad"}>
                    {inspectedError.error_owner || "upstream"} · {inspectedError.type || inspectedError.phase}
                  </Badge>
                </InsetRow>
                <InsetRow label="请求模型 → 上游模型">
                  <span className="code-mono">
                    {inspectedError.requested_model || inspectedError.model || "—"}
                    {inspectedError.upstream_model && inspectedError.upstream_model !== inspectedError.requested_model ? ` → ${inspectedError.upstream_model}` : ""}
                  </span>
                </InsetRow>
                <InsetRow label="请求编号"><TraceId value={inspectedError.request_id} /></InsetRow>
                <InsetRow label="标记为已处理">
                  <Switch
                    label="标记错误已处理"
                    checked={inspectedError.resolved}
                    disabled={recovery.busy}
                    onChange={next => void recovery.run(async () => {
                      const target = errorSource === "upstream-errors" ? "upstream-errors" : "request-errors";
                      await api(`/admin/ops/${target}/${inspectedError.id}/resolve`, empty, "PUT", {resolved: next});
                      setInspectedError({...inspectedError, resolved: next});
                      revise(v => v + 1);
                    })}
                  />
                </InsetRow>
              </InsetGroup>

              <InsetGroup title="四维关联实体直达">
                {inspectedError.user_id && (
                  <InsetRow label="调用用户">
                    <EntityLink kind="user" id={inspectedError.user_id} label={inspectedError.user_email || `#${inspectedError.user_id}`} />
                  </InsetRow>
                )}
                {inspectedError.api_key_id && (
                  <InsetRow label="API Key">
                    <EntityLink
                      kind="key"
                      id={inspectedError.api_key_id}
                      label={inspectedError.api_key_name || `Key #${inspectedError.api_key_id}`}
                      extra={inspectedError.user_id ? {user_id: inspectedError.user_id} : undefined}
                    />
                  </InsetRow>
                )}
                {inspectedError.group_id && (
                  <InsetRow label="调度分组">
                    <EntityLink kind="group" id={inspectedError.group_id} label={inspectedError.group_name || `分组 #${inspectedError.group_id}`} />
                  </InsetRow>
                )}
                {inspectedError.account_id && (
                  <InsetRow label="故障上游账号">
                    <EntityLink kind="account" id={inspectedError.account_id} label={inspectedError.account_name || `账号 #${inspectedError.account_id}`} />
                  </InsetRow>
                )}
              </InsetGroup>

              {inspectedError.account_id && (
                <InsetGroup title="处置故障账号">
                  <InsetRow label="快捷运维动作" vertical>
                    <InlineStack wrap>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={recovery.busy}
                        onClick={() => void recovery.run(async () => {
                          setRecoveryHint("");
                          await api(`/admin/accounts/${inspectedError.account_id}/recover`, account, "POST", {});
                          setRecoveryHint("已执行一键状态恢复（清除报错与限流）");
                          revise(v => v + 1);
                        })}
                      >
                        <Wrench aria-hidden />一键清错并恢复
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={recovery.busy}
                        onClick={() => void recovery.run(async () => {
                          setRecoveryHint("");
                          await api(`/admin/accounts/${inspectedError.account_id}/clear-rate-limit`, account, "POST", {});
                          setRecoveryHint("已清除该账号限流窗口");
                          revise(v => v + 1);
                        })}
                      >
                        清除限流
                      </Button>
                      <Button
                        size="xs"
                        variant="destructive"
                        busy={recovery.busy}
                        onClick={() => void recovery.run(async () => {
                          setRecoveryHint("");
                          await api(`/admin/accounts/${inspectedError.account_id}/scheduling`, empty, "PUT", {schedulable: false});
                          setRecoveryHint("已暂停该账号参与调度");
                          revise(v => v + 1);
                        })}
                      >
                        暂停调度
                      </Button>
                      <Button
                        size="xs"
                        variant="ghost"
                        onClick={() => navigateTo("accounts", {inspect: inspectedError.account_id!})}
                      >
                        在号池测活 →
                      </Button>
                    </InlineStack>
                  </InsetRow>
                </InsetGroup>
              )}
              <ErrorLine value={recovery.error} />
              {recoveryHint && <Feedback tone="success">{recoveryHint}</Feedback>}
            </>
          )}
        </Inspector>
      }
    >
      <FilterBar sticky>
        <div className="obs-filter-row">
          <Segmented
            label="监控视图切换"
            value={tab}
            options={[
              ["live", "负载与总览"],
              ["errors", "报错与追踪"],
              ["alerts", "告警与策略"],
            ] as const}
            onChange={next => { setTab(next); setInspectedError(undefined); }}
          />
          {tab === "errors" && (
            <Segmented
              label="日志数据源"
              value={errorSource}
              options={[
                ["request-errors", "客户端报错"],
                ["upstream-errors", "上游报错"],
                ["requests", "全量请求"],
              ] as const}
              onChange={next => { setErrorSource(next); setInspectedError(undefined); }}
            />
          )}
          {tab !== "alerts" && (
            <>
              <QuickPills label="时间窗" value={timeRange} options={timeRanges} onChange={setTimeRange} />
              <FilterSlot><PlatformSelect value={platform} onChange={setPlatform} /></FilterSlot>
              <FilterSlot><GroupSelect value={groupID} onChange={setGroupID} /></FilterSlot>
            </>
          )}
          {tab === "errors" && (
            <>
              <SearchInput value={search} onChange={setSearch} placeholder="搜请求编号或模型…" ariaLabel="搜索请求编号或模型" />
              <FilterSlot><AccountSelect value={accountID} onChange={setAccountID} /></FilterSlot>
              <FilterSlot><UserSelect value={userID} onChange={v => setUserID(v)} /></FilterSlot>
            </>
          )}
          <div className="obs-filter-spacer">
            <InlineStack density="compact" className="text-fine muted" role="status">
              <StatusDot tone={streamState === "live" ? "ok" : streamState === "denied" ? "bad" : "warn"} />
              {streamLabel[streamState]}
            </InlineStack>
            {tab !== "live" && badAccounts > 0 && (
              <Button variant="outline" onClick={() => navigateTo("accounts", {status: "error"})}>
                异常账号 {badAccounts}
              </Button>
            )}
            {tab !== "live" && firingCount > 0 && (
              <Button variant="outline" onClick={() => setTab("alerts")}>
                <Bell aria-hidden />活跃告警 {firingCount}
              </Button>
            )}
            <RefreshButton iconOnly size="default" variant="ghost" label="刷新监控" onClick={() => revise(v => v + 1)} />
          </div>
        </div>
      </FilterBar>

      {tab === "live" && (
        <WorkspaceContent>
          <StatusBanner
            tone={!ops ? "info" : firingCount > 0 || sla !== undefined && sla < 95 ? "bad" : attention.length ? "warn" : "ok"}
            title={!ops ? "正在读取运行状态" : attention.length ? `${attention.length} 项需要处理` : "运行正常"}
            subtitle={attention.length ? attention.join(" · ") : uptime ? `已连续运行 ${uptime}` : undefined}
            actions={<>
              {firingCount > 0 && <Button size="sm" variant="outline" onClick={() => setTab("alerts")}><Bell aria-hidden />查看告警</Button>}
              {badAccounts > 0 && <Button size="sm" variant="outline" onClick={() => navigateTo("accounts", {status: "error"})}>处理异常账号</Button>}
              {ops && ops.overview.error_count_total > 0 && <Button size="sm" variant="ghost" onClick={() => setTab("errors")}>排查报错</Button>}
            </>}
          />
          <MetricStrip
            items={[
              {id: "qps", label: "当前吞吐", value: conc ? conc.qps.current.toFixed(1) : ops ? ops.overview.qps.current.toFixed(1) : null, unit: "req/s", hint: conc ? `峰值 ${conc.qps.peak.toFixed(1)}` : undefined, tone: "accent"},
              {id: "sla", label: "成功率", value: sla !== undefined ? `${sla.toFixed(sla >= 99.95 ? 0 : 1)}%` : null, hint: ops ? `${count.format(ops.overview.error_count_total)} 次报错 · 429×${ops.overview.upstream_429_count}` : undefined,
                tone: sla === undefined ? "neutral" : sla < 95 ? "bad" : sla < 99 ? "warn" : "ok", onClick: () => setTab("errors")},
              {id: "latency", label: "平均响应", value: ops?.overview.duration.avg_ms != null ? durationText(ops.overview.duration.avg_ms) : null, tone: "cyan"},
              // 并发统计未启用时收起整格，不渲染「未知」
              ...(conc && !conc.enabled ? [] : [
                {id: "conc", label: "并发占用", value: conc?.enabled ? count.format(totalInUse) : null, hint: conc?.enabled ? (totalWaiting > 0 ? `${totalWaiting} 个请求排队中` : "无排队") : undefined, tone: totalWaiting > 0 ? "warn" as const : "neutral" as const},
              ]),
              {id: "cost", label: "今日扣费", value: dash?.traffic ? smartMoney(dash.traffic.today_cost) : null, hint: dash?.traffic ? `${compact.format(dash.traffic.today_tokens)} Token` : undefined, tone: "violet", onClick: () => navigateTo("usage", {period: "today"})},
            ]}
          />
          <div className="split-grid-2">{heroTabs.filter(chart => chart.id !== "cost" && (chart.id !== "concurrency" || (conc?.platforms?.length ?? 0) > 1)).map(chart =>
            <MetricTrend key={chart.id} metric={chart} primary={chart.id === "throughput"} hideValue switchable={chart.id !== "latency" && chart.id !== "concurrency"} />)}</div>
          <h2 className="stage-heading">历史</h2>
          {heroTabs.filter(chart => chart.id === "cost").map(chart => <MetricTrend key={chart.id} metric={chart} />)}

          <div className="split-grid-2">
            <Panel
              title="分组实时并发与健康配比"
              padded={false}
              actions={
                <Button size="xs" variant="ghost" onClick={() => navigateTo("accounts")}>
                  查看号池 →
                </Button>
              }
            >
              <DataTable
                label="分组实时水位"
                rows={(conc?.groups ?? []).slice(0, 8)}
                rowID={row => row.group_id}
                onRowClick={row => navigateTo("groups", {inspect: row.group_id})}
                empty={<EmptyState title="暂无活跃分组负载" />}
                columns={[
                  {
                    id: "group",
                    title: "调度分组",
                    render: row => (
                      <Stack>
                        <EntityLink kind="group" id={row.group_id} label={row.group_name || `#${row.group_id}`} />
                        <PlatformBadge platform={row.platform} />
                      </Stack>
                    ),
                  },
                  {
                    id: "conc",
                    title: "并发占用",
                    render: row => (
                      <MeterBar
                        size="sm"
                        value={row.max_capacity > 0 ? (row.current_in_use / row.max_capacity) * 100 : 0}
                        label={`${row.current_in_use}/${row.max_capacity || "∞"}`}
                      />
                    ),
                  },
                  {
                    id: "accounts",
                    title: "可用 / 限流 / 异常",
                    render: row => (
                      <SegmentBar
                        compact
                        segments={[
                          {label: "可用", value: row.available_count, tone: "ok"},
                          {label: "限流", value: row.rate_limit_count, tone: "warn"},
                          {label: "异常", value: row.error_count, tone: "bad"},
                        ]}
                      />
                    ),
                  },
                ]}
              />
            </Panel>

            <Panel
              title="近 7 天消费排行（分组与用户）"
              actions={
                <Button size="xs" variant="ghost" onClick={() => navigateTo("usage")}>
                  完整账单分析 →
                </Button>
              }
            >
              <Stack>
                {(dash?.top_groups ?? []).length > 0 && (
                  <RankedBarList
                    items={(dash?.top_groups ?? []).slice(0, 4).map(g => ({
                      id: `grp-${g.group_id}`,
                      label: g.group_name,
                      sub: `${count.format(g.requests)} 次`,
                      value: g.actual_cost,
                      formatted: smartMoney(g.actual_cost),
                      color: palette.c1,
                      onClick: () => navigateTo("usage", {group: g.group_id}),
                    }))}
                  />
                )}
                <RankedBarList
                  emptyTitle="近 7 天暂无消费记录"
                  items={(dash?.top_users ?? []).slice(0, 5).map(u => ({
                    id: `usr-${u.user_id}`,
                    label: u.email,
                    sub: `${compact.format(u.tokens)} Tok`,
                    value: u.actual_cost,
                    formatted: smartMoney(u.actual_cost),
                    color: palette.c3,
                    onClick: () => navigateTo("users", {inspect: u.user_id}),
                  }))}
                />
              </Stack>
            </Panel>
          </div>
        </WorkspaceContent>
      )}

      {tab === "errors" && (
        <div className="swift-pane-table">
          {errorSource === "requests" ? (
            <RemoteTable
              key={errorListPath}
              path={errorListPath}
              schema={opsRequestRowSchema}
              columns={requestColumns}
              rowID={row => row.request_id || `${row.created_at}-${row.model}`}
              label="请求追踪"
              revision={revision}
            />
          ) : (
            <RemoteTable
              key={errorListPath}
              path={errorListPath}
              schema={opsErrorRowSchema}
              columns={errorColumns}
              rowID={row => row.id}
              label="异常记录"
              revision={revision}
              activeRowID={inspectedError?.id}
              onRowClick={setInspectedError}
              onItemsLoaded={setErrorRows}
            />
          )}
        </div>
      )}

      {tab === "alerts" && (
        <WorkspaceContent>
          <OpsAlertsAndSettings revision={revision} onChanged={() => revise(v => v + 1)} />
        </WorkspaceContent>
      )}
    </WorkspaceStage>
  );
}

function OpsAlertsAndSettings({revision, onChanged}: {revision: number; onChanged: () => void}) {
  const formID = useId();
  const rules = useResource(() => api("/admin/ops/alert-rules", z.array(opsAlertRuleSchema)), "ops-rules", true, revision);
  const events = useResource(() => api("/admin/ops/alert-events", z.array(opsAlertEventSchema)), "ops-events-all", true, revision);
  const settings = useResource(() => api("/admin/ops/settings", opsSettingsBundleSchema), "ops-settings", true, revision);

  const [ruleModal, setRuleModal] = useState<{mode: "create"} | {mode: "edit"; rule: AlertRule}>();
  const [notifyEmail, setNotifyEmail] = useState(true);
  const [deletingRule, setDeletingRule] = useState<AlertRule>();
  const [savedNotice, setSavedNotice] = useState("");
  const action = useAction();
  const removal = useAction();

  return (
    <>
      <div className="split-grid-2">
        <Panel title="近期告警事件" padded={false}>
          <Resource state={events}>
            {rows => (
              <DataTable
                label="告警事件"
                rows={rows}
                rowID={row => row.id}
                empty={<EmptyState title="近期没有告警触发" description="所有监控指标均处于阈值安全范围内。" />}
                columns={[
                  {id: "time", title: "时间", render: row => <span className="code-mono">{date(row.fired_at)}</span>},
                  {
                    id: "title",
                    title: "事件",
                    render: row => (
                      <div>
                        <strong>{row.title || `规则 #${row.rule_id}`}</strong>
                        {row.description && <div className="muted text-fine">{row.description}</div>}
                      </div>
                    ),
                  },
                  {id: "state", title: "状态", align: "center", render: row => <Status value={row.status} />},
                  {
                    id: "act",
                    title: "",
                    align: "end",
                    render: row =>
                      row.status === "firing" ? (
                        <div className="row-actions">
                          <Button
                            size="xs"
                            variant="outline"
                            busy={action.busy}
                            onClick={() => void action.run(async () => {
                              await api(`/admin/ops/alert-events/${row.id}/status`, empty, "PUT", {status: "manual_resolved"});
                              onChanged();
                            })}
                          >
                            标记恢复
                          </Button>
                          <Button
                            size="xs"
                            variant="ghost"
                            busy={action.busy}
                            onClick={() => void action.run(async () => {
                              const until = new Date(Date.now() + 3600_000).toISOString();
                              await api("/admin/ops/alert-silences", empty, "POST", {rule_id: row.rule_id, platform: "", until, reason: "控制台静默 1 小时"});
                              onChanged();
                            })}
                          >
                            静默 1h
                          </Button>
                        </div>
                      ) : null,
                  },
                ]}
              />
            )}
          </Resource>
        </Panel>

        <Panel
          title="告警规则"
          padded={false}
          actions={
            <Button size="xs" onClick={() => { action.clearError(); setNotifyEmail(true); setRuleModal({mode: "create"}); }}>
              <Plus aria-hidden />新建规则
            </Button>
          }
        >
          <Resource state={rules}>
            {rows => (
              <DataTable
                label="告警规则"
                rows={rows}
                rowID={row => row.id ?? row.name}
                empty={<EmptyState title="尚未配置告警规则" action={<Button size="sm" onClick={() => { setNotifyEmail(true); setRuleModal({mode: "create"}); }}><Plus aria-hidden />创建第一条规则</Button>} />}
                columns={[
                  {
                    id: "name",
                    title: "规则",
                    render: row => (
                      <div>
                        <strong>{row.name}</strong>
                        <div className="muted code-mono text-fine">{row.metric_type} {row.operator} {row.threshold}（{row.window_minutes}m）</div>
                      </div>
                    ),
                  },
                  {id: "sev", title: "级别", render: row => <Badge variant="soft" tone={row.severity === "critical" ? "bad" : row.severity === "warning" ? "warn" : "info"}>{row.severity}</Badge>},
                  {
                    id: "on",
                    title: "启用",
                    render: row => (
                      <Switch
                        label={`启用规则 ${row.name}`}
                        checked={row.enabled}
                        onChange={checked => {
                          if (!row.id) return;
                          void action.run(async () => {
                            await api(`/admin/ops/alert-rules/${row.id}`, opsAlertRuleSchema, "PUT", {...row, enabled: checked});
                            onChanged();
                          });
                        }}
                      />
                    ),
                  },
                  {
                    id: "ops",
                    title: "",
                    align: "end",
                    render: row => (
                      <div className="row-actions">
                        <Button size="xs" variant="ghost" onClick={() => { setNotifyEmail(row.notify_email); setRuleModal({mode: "edit", rule: row}); }}>编辑</Button>
                        <Button size="icon-xs" variant="ghost" title="删除规则" aria-label={`删除 ${row.name}`} onClick={() => setDeletingRule(row)}>
                          <Trash2 />
                        </Button>
                      </div>
                    ),
                  },
                ]}
              />
            )}
          </Resource>
        </Panel>
      </div>

      <Resource state={settings}>
        {bundle => (
          <OpsSettingsForm
            key={revision}
            bundle={bundle}
            savedNotice={savedNotice}
            onSaved={() => {
              setSavedNotice("运行策略与通知设置已保存");
              onChanged();
            }}
          />
        )}
      </Resource>

      <Overlay
        open={!!ruleModal}
        title={ruleModal?.mode === "edit" ? "编辑告警规则" : "新建告警规则"}
        onClose={() => setRuleModal(undefined)}
        busy={action.busy}
        footer={
        <>
          <Button variant="outline" onClick={() => setRuleModal(undefined)}>取消</Button>
          <Button type="submit" form={formID} busy={action.busy}>保存规则</Button>
        </>
      }
      >
        {ruleModal && (
          <Form id={formID}
            busy={action.busy}
            onSubmit={event => {
              const fd = new FormData(event.currentTarget);
              const payload = {
                name: String(fd.get("name") ?? "").trim(),
                description: String(fd.get("description") ?? "").trim(),
                enabled: ruleModal.mode === "edit" ? ruleModal.rule.enabled : true,
                metric_type: String(fd.get("metric_type") ?? "error_rate"),
                operator: String(fd.get("operator") ?? ">"),
                threshold: Number(fd.get("threshold") ?? 5),
                window_minutes: Number(fd.get("window_minutes") ?? 5),
                sustained_minutes: Number(fd.get("sustained_minutes") ?? 1),
                severity: String(fd.get("severity") ?? "warning"),
                cooldown_minutes: Number(fd.get("cooldown_minutes") ?? 30),
                notify_email: notifyEmail,
              };
              void action.run(async () => {
                if (ruleModal.mode === "edit" && ruleModal.rule.id) {
                  await api(`/admin/ops/alert-rules/${ruleModal.rule.id}`, opsAlertRuleSchema, "PUT", payload);
                } else {
                  await api("/admin/ops/alert-rules", opsAlertRuleSchema, "POST", payload);
                }
                setRuleModal(undefined);
                onChanged();
              });
            }}
          >
            <Field label="规则名称">
              <Input name="name" required maxLength={100} defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.name : ""} />
            </Field>
            <FormGrid>
              <Field label="监控指标">
                <Select name="metric_type" defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.metric_type : "error_rate"}>
                  {metricTypeOptions.map(([val, text]) => <option key={val} value={val}>{text}</option>)}
                </Select>
              </Field>
              <Field label="严重级别">
                <Select name="severity" defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.severity : "warning"}>
                  <option value="critical">严重 (critical)</option>
                  <option value="warning">预警 (warning)</option>
                  <option value="info">提示 (info)</option>
                </Select>
              </Field>
              <Field label="比较符">
                <Select name="operator" defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.operator : ">"}>
                  <option value=">">&gt;</option>
                  <option value=">=">&gt;=</option>
                  <option value="<">&lt;</option>
                  <option value="<=">&lt;=</option>
                </Select>
              </Field>
              <Field label="触发阈值">
                <Input name="threshold" type="number" step="0.1" required defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.threshold : 5} />
              </Field>
              <Field label="统计窗口（分钟）">
                <Input name="window_minutes" type="number" min="1" max="1440" required defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.window_minutes : 5} />
              </Field>
              <Field label="冷却时间（分钟）">
                <Input name="cooldown_minutes" type="number" min="0" max="10080" required defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.cooldown_minutes : 30} />
              </Field>
            </FormGrid>
            <Field label="补充说明">
              <Input name="description" maxLength={200} defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.description : ""} />
            </Field>
            <InsetGroup>
              <InsetRow label="触发时发送邮件通知">
                <Switch label="发送邮件通知" checked={notifyEmail} onChange={setNotifyEmail} />
              </InsetRow>
            </InsetGroup>
            <ErrorLine value={action.error} />
          </Form>
        )}
      </Overlay>

      <ConfirmDialog
        open={!!deletingRule}
        title="删除告警规则"
        description={`确定删除规则「${deletingRule?.name ?? ""}」吗？`}
        confirm="删除"
        busy={removal.busy}
        error={removal.error}
        onClose={() => setDeletingRule(undefined)}
        onConfirm={() => {
          if (!deletingRule?.id) return;
          void removal.run(async () => {
            await api(`/admin/ops/alert-rules/${deletingRule.id}`, empty, "DELETE");
            setDeletingRule(undefined);
            onChanged();
          });
        }}
      />
    </>
  );
}

function OpsSettingsForm({
  bundle,
  savedNotice,
  onSaved,
}: {
  bundle: z.infer<typeof opsSettingsBundleSchema>;
  savedNotice: string;
  onSaved: () => void;
}) {
  const save = useAction();
  const [emailEnabled, setEmailEnabled] = useState(bundle.email.alert.enabled);
  const [recipients, setRecipients] = useState(bundle.email.alert.recipients.join(", "));
  const [slaMin, setSlaMin] = useState(String(bundle.thresholds.sla_percent_min ?? 99));
  const [ttftMax, setTtftMax] = useState(String(bundle.thresholds.ttft_p99_ms_max ?? 5000));
  const [reqErrMax, setReqErrMax] = useState(String(bundle.thresholds.request_error_rate_percent_max ?? 5));
  const [upErrMax, setUpErrMax] = useState(String(bundle.thresholds.upstream_error_rate_percent_max ?? 5));
  const [logLevel, setLogLevel] = useState(bundle.logging.level || "info");
  const [ignoreCanceled, setIgnoreCanceled] = useState(bundle.advanced.ignore_context_canceled);
  const [ignoreInvalidKey, setIgnoreInvalidKey] = useState(bundle.advanced.ignore_invalid_api_key_errors);
  const [ignoreNoBalance, setIgnoreNoBalance] = useState(bundle.advanced.ignore_insufficient_balance_errors);

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api("/admin/ops/settings", empty, "PUT", {
          thresholds: {
            sla_percent_min: slaMin ? Number(slaMin) : null,
            ttft_p99_ms_max: ttftMax ? Number(ttftMax) : null,
            request_error_rate_percent_max: reqErrMax ? Number(reqErrMax) : null,
            upstream_error_rate_percent_max: upErrMax ? Number(upErrMax) : null,
          },
          email: {
            ...bundle.email,
            alert: {
              ...bundle.email.alert,
              enabled: emailEnabled,
              recipients: recipients.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean),
            },
          },
          logging: {
            ...bundle.logging,
            level: logLevel,
          },
          advanced: {
            ...bundle.advanced,
            ignore_context_canceled: ignoreCanceled,
            ignore_invalid_api_key_errors: ignoreInvalidKey,
            ignore_insufficient_balance_errors: ignoreNoBalance,
          },
        });
        onSaved();
      })}
    >
      <div className="split-grid-2">
        <FormSection title="监控红线阈值与邮件告警">
          <InsetRow label="启用邮件告警通知" hint="达到规则阈值时自动向收件人发送通知">
            <Switch label="启用邮件告警" checked={emailEnabled} onChange={setEmailEnabled} />
          </InsetRow>
          <Field label="告警收件邮箱（逗号分隔）">
            <Input value={recipients} onChange={e => setRecipients(e.target.value)} placeholder="ops@example.com" />
          </Field>
          <Field label="最低 SLA 红线 (%)">
            <div className="row-field"><Input type="number" step="0.1" value={slaMin} onChange={e => setSlaMin(e.target.value)} /></div>
          </Field>
          <Field label="TTFT P99 上限 (ms)">
            <div className="row-field"><Input type="number" value={ttftMax} onChange={e => setTtftMax(e.target.value)} /></div>
          </Field>
          <FormSection title="请求 / 上游错误率红线 (%)">
            <FormGrid>
              <Field label="请求错误率红线"><Input type="number" step="0.1" value={reqErrMax} onChange={e => setReqErrMax(e.target.value)} aria-label="请求错误率红线" /></Field>
              <Field label="上游错误率红线"><Input type="number" step="0.1" value={upErrMax} onChange={e => setUpErrMax(e.target.value)} aria-label="上游错误率红线" /></Field>
            </FormGrid>
          </FormSection>
        </FormSection>

        <FormSection title="运行时日志与噪音过滤">
          <Field label="运行时日志级别">
            <div className="row-field">
              <Select value={logLevel} onChange={e => setLogLevel(e.target.value)}>
                <option value="debug">debug</option>
                <option value="info">info</option>
                <option value="warn">warn</option>
                <option value="error">error</option>
              </Select>
            </div>
          </Field>
          <InsetRow label="忽略客户端主动断开 (Context Canceled)" hint="不计入 SLA 故障统计">
            <Switch label="忽略客户端取消" checked={ignoreCanceled} onChange={setIgnoreCanceled} />
          </InsetRow>
          <InsetRow label="忽略无效 API Key 请求" hint="401 鉴权拒绝不计入网关故障">
            <Switch label="忽略无效 Key" checked={ignoreInvalidKey} onChange={setIgnoreInvalidKey} />
          </InsetRow>
          <InsetRow label="忽略用户余额不足拦截" hint="402/403 余额耗尽不触发渠道告警">
            <Switch label="忽略余额不足" checked={ignoreNoBalance} onChange={setIgnoreNoBalance} />
          </InsetRow>
        </FormSection>
      </div>

      <CollapsibleSection title="数据保留策略摘要" summary={`错误日志保留 ${bundle.advanced.data_retention.error_log_retention_days} 天`}>
        <div className="muted text-meta">
          分钟级指标保留 {bundle.advanced.data_retention.minute_metrics_retention_days} 天 · 小时级指标保留 {bundle.advanced.data_retention.hourly_metrics_retention_days} 天
        </div>
      </CollapsibleSection>

      <ErrorLine value={save.error} />
      {savedNotice && <Feedback tone="success">{savedNotice}</Feedback>}
      <FormActions>
        <Button type="submit" busy={save.busy}>
          <ShieldAlert aria-hidden />保存监控与告警设置
        </Button>
      </FormActions>
    </Form>
  );
}
