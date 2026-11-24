import {useEffect, useId, useMemo, useRef, useState} from "react";
import {
  Boxes,
  Copy as DuplicateIcon,
  PanelRight,
  Plus,
  Trash2,
  Wrench,
  Zap,
} from "lucide-react";
import {z} from "zod";
import {
  account,
  accountBalance,
  accountQuota,
  accountQuotaBatch,
  accountStatsSummary,
  adminGroupRow,
  adminOverview,
  api,
  describeError,
  empty,
  ephemeralPassSchema,
  named,
  proxyRow,
  streamProbe,
  testAccount,
  type ProbeReport,
  type UsageStreamStatus,
} from "../api";
import {compactTokens, count, currencyMoney, date, percent, relativeUntil, smartMoney} from "../format";
import {useModelCatalog} from "../model-catalog";
import {useAction, useDebouncedValue, useResource} from "../requests";
import {navigateTo} from "../route";
import {
  FormActions,
  Badge,
  BatchBar,
  Button,
  ChoiceCards,
  CollapsibleSection,
  ConfirmDialog,
  CopyButton,
  DataTable,
  DetailHeader,
  EllipsisText,
  EmptyState,
  EntityCard,
  EntityMeters,
  EntityMetrics,
  EntityResult,
  EntityGrid,
  EntityLink,
  ErrorLine,
  Feedback,
  Field,
  FilterBar,
  FilterSlot,
  Form,
  FormSection,
  GroupSelect,
  InlineStack,
  Input,
  InsetGroup,
  InsetRow,
  Inspector,
  Menu,
  MenuItem,
  MenuSeparator,
  MoreActions,
  RowActions,
  SecondaryTableAction,
  MeterBar,
  MetricStrip,
  MiniPills,
  ModelSelect,
  MultiGroupSelect,
  Overlay,
  PasswordInput,
  PLATFORM_OPTIONS,
  PlanSummary,
  PlatformBadge,
  ProxySelect,
  QuickPills,
  RefreshButton,
  RefreshMorphIcon,
  Resource,
  SearchInput,
  Segmented,
  Select,
  Stack,
  Status,
  StatusDot,
  Switch,
  Tabs,
  Textarea,
  Toolbar,
  Tooltip,
  ViewToggle,
  WorkspaceStage,
  toast,
  type Column,
  type EntityCardResult,
  type ViewMode,
} from "../ui";
import {groupBriefSchema, statusPills, type AccountRow} from "./pool-shared";
import {ActivityWire, ReorderMotion} from "../ui/activity-motion";
import {RemoteTable} from "./remote-table";
import {AccountBatch} from "./account-batch";
import {AccountCRS} from "./account-crs";
import {AccountData} from "./account-data";

type AccountSort = "priority" | "today_cost" | "five_hours" | "week" | "last_used" | "activity";
type QuotaInfo = z.infer<typeof accountQuota>;
type BalanceInfo = z.infer<typeof accountBalance>;
type CardMode = "quota" | "usage";
type UsageWindow = "1" | "7" | "30";
type QuotaDisplay = "used" | "remaining";
type WindowStat = {requests: number; tokens: number; cost: number};

const sortOptions = [
  {value: "priority", label: "按优先级"},
  {value: "today_cost", label: "今日消耗高到低"},
  {value: "five_hours", label: "5h 水位高到低"},
  {value: "week", label: "7d 水位高到低"},
  {value: "last_used", label: "最近使用"},
  {value: "activity", label: "按活动排序"},
] as const;

const viewKey = "panel-accounts-view";
const quotaDisplayKey = "panel-accounts-quota-mode";
const balancePlatforms = new Set(["deepseek", "kimi"]);

function balanceText(balance: BalanceInfo) {
  const label = balance.kind === "key_quota" ? "密钥额度" : balance.kind === "subscription" ? "订阅额度" : "余额";
  return label + " " + currencyMoney(balance.balance, balance.currency) + (balance.stale ? " · 待更新" : "");
}

function balanceHint(balance: BalanceInfo) {
  const sampled = balance.fetched_at ? "余额更新于 " + date(balance.fetched_at * 1000) : "";
  const billing = balance.billing;
  if (!billing) return sampled || undefined;
  const detail = "上游 Token 计费倍率 ×" + billing.effective_multiplier + " · " + date(billing.observed_at);
  return [sampled, detail + (balance.stale || billing.stale ? " · 待更新" : "")].filter(Boolean).join(" / ");
}

function sortAccounts(rows: AccountRow[], sort: AccountSort) {
  const list = [...rows];
  switch (sort) {
    case "activity":
      return list;
    case "today_cost":
      return list.sort((a, b) => (b.today_cost ?? -1) - (a.today_cost ?? -1) || a.id - b.id);
    case "five_hours":
      return list.sort((a, b) => (b.five_hours_used ?? -1) - (a.five_hours_used ?? -1));
    case "week":
      return list.sort((a, b) => (b.week_used ?? -1) - (a.week_used ?? -1));
    case "last_used":
      return list.sort((a, b) => (Date.parse(b.last_used_at ?? "") || 0) - (Date.parse(a.last_used_at ?? "") || 0));
    default:
      return list.sort((a, b) => a.priority - b.priority || a.id - b.id);
  }
}

// 账号当前不能正常调度的原因与恢复时间；正常时返回 undefined。
function concurrencyLabel(row: AccountRow) {
  return row.activity_known === false ? "活动未知" : String(row.current_concurrency) + "/" + (row.concurrency || "∞");
}

function accountProblem(row: AccountRow) {
  if (row.expires_at != null && row.expires_at * 1000 <= Date.now()) return {label: "已到期", tone: "bad", action: "edit"} as const;
  if (row.type === "apikey" && row.credentials_status.has_api_key === false) return {label: "缺少上游密钥", tone: "bad", action: "edit"} as const;
  if (row.proxy_fallback_origin_id) return {label: "代理已回退", tone: "warn", action: "revert-proxy-fallback"} as const;
  if (row.status === "error") return {label: "运行异常", tone: "bad", action: "recover"} as const;
  if (row.rate_limit_reset_at) return {label: "限流 " + relativeUntil(row.rate_limit_reset_at), tone: "warn", action: "clear-rate-limit"} as const;
  if (row.overload_until) return {label: "过载 " + relativeUntil(row.overload_until), tone: "warn", action: "recover"} as const;
  if (row.temp_unschedulable_until) return {label: "冷却 " + relativeUntil(row.temp_unschedulable_until), tone: "warn", action: "temp-unschedulable"} as const;
}

function accountHold(row: AccountRow) {
  return accountProblem(row)?.label;
}

function isQuotaCapable(row: AccountRow, quota?: QuotaInfo) {
  return (
    row.type === "oauth" ||
    row.type === "setup-token" ||
    row.five_hours_used !== null ||
    row.week_used !== null ||
    Boolean(quota?.five_hours || quota?.week || quota?.gemini_daily || quota?.sonnet_week)
  );
}

function canRefreshBalance(row: AccountRow) {
  return row.type === "apikey" && balancePlatforms.has(row.platform);
}

function buildQuotaMeters(row: AccountRow, quota: QuotaInfo | undefined, display: QuotaDisplay) {
  const rawWindows: {label: string; used: number | null; reset?: string | null}[] = [
    {
      label: "5 小时",
      used: quota?.five_hours ? quota.five_hours.percent_used : row.five_hours_used,
      reset: quota?.five_hours?.resets_at ?? row.five_hours_reset_at,
    },
    {
      label: "7 天",
      used: quota?.week ? quota.week.percent_used : row.week_used,
      reset: quota?.week?.resets_at ?? row.week_reset_at,
    },
    ...(quota?.sonnet_week ? [{label: "Sonnet", used: quota.sonnet_week.percent_used, reset: quota.sonnet_week.resets_at}] : []),
    ...(quota?.gemini_daily ? [{label: "日配额", used: quota.gemini_daily.percent_used, reset: quota.gemini_daily.resets_at}] : []),
    ...(quota?.gemini_pro_daily ? [{label: "Pro 日", used: quota.gemini_pro_daily.percent_used, reset: quota.gemini_pro_daily.resets_at}] : []),
    ...(quota?.thirty_day ? [{label: "30 天", used: quota.thirty_day.percent_used, reset: quota.thirty_day.resets_at}] : []),
  ];
  const active = rawWindows.filter(w => w.used !== null);
  const target = active.length > 0 ? active : rawWindows.slice(0, 2);
  return target.map(w => {
    const used = w.used;
    const shown = used === null ? null : display === "remaining" ? Math.max(0, 100 - used) : used;
    const tone: "ok" | "warn" | "bad" | "auto" = used === null ? "auto" : used >= 85 ? "bad" : used >= 60 ? "warn" : "ok";
    const resetText = w.reset ? relativeUntil(w.reset) : "";
    return {
      label: w.label,
      reset: resetText || undefined,
      bar: <MeterBar size="sm" value={shown} tone={tone} label={shown === null ? "未知" : `${shown.toFixed(0)}%`} />,
    };
  });
}

export function AccountsWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [groupFilter, setGroupFilter] = useState(() => initial.get("group") ?? "");
  const [proxyFilter, setProxyFilter] = useState(() => initial.get("proxy") ?? "");
  const [platform, setPlatform] = useState(() => initial.get("platform") ?? "");
  const [status, setStatus] = useState(() => initial.get("status") ?? "");
  const [search, setSearch] = useState(() => initial.get("search") ?? "");
  const [sort, setSort] = useState<AccountSort>(() => sortOptions.find(option => option.value === initial.get("sort"))?.value ?? "priority");
  const [rowInteraction, setRowInteraction] = useState(false);
  const [rowMenuOpen, setRowMenuOpen] = useState(false);
  const [liveStatus, setLiveStatus] = useState<UsageStreamStatus>("connecting");
  const [view, setView] = useState<ViewMode>(() => (localStorage.getItem(viewKey) === "list" ? "list" : "card"));
  const [quotaDisplay, setQuotaDisplay] = useState<QuotaDisplay>(() =>
    localStorage.getItem(quotaDisplayKey) === "remaining" ? "remaining" : "used",
  );
  const [defaultWindow, setDefaultWindow] = useState<UsageWindow>("1");
  const [cardModes, setCardModes] = useState<Record<number, CardMode>>({});
  const [cardWindows, setCardWindows] = useState<Record<number, UsageWindow>>({});
  const [quotaMap, setQuotaMap] = useState<Record<number, QuotaInfo>>({});
  const [balanceMap, setBalanceMap] = useState<Record<number, BalanceInfo>>({});
  const [statsCache, setStatsCache] = useState<Record<string, WindowStat>>({});
  const [loadingStats, setLoadingStats] = useState<Record<string, boolean>>({});
  const statsInFlight = useRef(new Set<string>());

  const [revision, revise] = useState(0);
  const [selected, setSelected] = useState<number[]>([]);
  const [rows, setRows] = useState<AccountRow[]>([]);
  const [platformCounts, setPlatformCounts] = useState<Record<string, number>>({});
  const [inspect, setInspect] = useState<number | undefined>(() => {
    const id = Number(initial.get("inspect") ?? 0);
    return id > 0 ? id : undefined;
  });
  const [modelsAccount, setModelsAccount] = useState<AccountRow>();
  const [results, setResults] = useState<Record<number, EntityCardResult>>({});
  const [busyID, setBusyID] = useState<number>();
  const [addOpen, setAddOpen] = useState(false);
  const [bulkBindOpen, setBulkBindOpen] = useState(false);
  const [crsOpen, setCRSOpen] = useState(false);
  const [accountData, setAccountData] = useState<"import" | "export">();
  const [batchTargets, setBatchTargets] = useState<z.infer<typeof named>[]>();
  const [deleting, setDeleting] = useState<AccountRow | "batch">();
  const deletionTrigger = useRef<HTMLElement>(null);
  const addTrigger = useRef<HTMLButtonElement>(null);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => {
    statsInFlight.current.clear();
    setStatsCache({});
    revise(v => v + 1);
  };
  const batchAction = useAction();
  const removeAction = useAction();

  useEffect(() => {
    const params = new URLSearchParams(queryString);
    setGroupFilter(params.get("group") ?? "");
    setProxyFilter(params.get("proxy") ?? "");
    setStatus(params.get("status") ?? "");
    setPlatform(params.get("platform") ?? "");
    setSearch(params.get("search") ?? "");
    setSort(sortOptions.find(option => option.value === params.get("sort"))?.value ?? "priority");
    const id = Number(params.get("inspect") ?? 0);
    setInspect(id > 0 ? id : undefined);
  }, [queryString]);

  const groups = useResource(() => api("/admin/groups/all", z.array(groupBriefSchema)), "account-groups", true, revision);
  const overview = useResource(() => api("/admin/overview", adminOverview), "admin-overview", true, revision);
  const totals = overview?.value?.accounts;

  const query = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (platform) p.set("platform", platform);
    if (status) p.set("status", status);
    if (groupFilter) p.set("group", groupFilter);
    if (proxyFilter) p.set("proxy_id", proxyFilter);
    if (sort === "activity") p.set("sort_by", "activity");
    const qs = p.toString();
    return "/admin/accounts" + (qs ? "?" + qs : "");
  }, [debouncedSearch, groupFilter, platform, proxyFilter, status, sort]);

  const unfiltered = !debouncedSearch && !platform && !status && !groupFilter && !proxyFilter;
  const filtered = !unfiltered;
  function updateAccountQuery(update: Record<string, string>) {
    navigateTo("accounts", {...Object.fromEntries(new URLSearchParams(location.search)), ...update}, true);
  }
  function openAccount(id: number) {
    const params = Object.fromEntries(new URLSearchParams(location.search));
    delete params.inspect;
    navigateTo("accounts", {...params, detail: id});
  }
  useEffect(() => {
    if (search.trim() !== debouncedSearch) return;
    const urlSearch = new URLSearchParams(location.search).get("search") ?? "";
    if (urlSearch !== debouncedSearch) {
      navigateTo("accounts", {...Object.fromEntries(new URLSearchParams(location.search)), search: debouncedSearch}, true);
    }
  }, [debouncedSearch, search]);
  const resetFilters = () => {
    updateAccountQuery({group: "", proxy: "", platform: "", status: "", search: ""});
  };

  useEffect(() => {
    if (!unfiltered || rows.length === 0) return;
    const next: Record<string, number> = {};
    for (const r of rows) next[r.platform] = (next[r.platform] ?? 0) + 1;
    setPlatformCounts(next);
  }, [rows, unfiltered]);

  // 当页含 OAuth 账号时批量预拉配额窗口，避免逐卡 N+1 请求。
  useEffect(() => {
    const oauthIDs = rows.filter(r => r.type === "oauth" || r.type === "setup-token").map(r => r.id);
    if (oauthIDs.length === 0) return;
    let cancelled = false;
    void api("/admin/accounts/quota/batch", accountQuotaBatch, "POST", {account_ids: oauthIDs, force: false})
      .then(res => {
        if (cancelled) return;
        setQuotaMap(prev => {
          const next = {...prev};
          for (const [k, v] of Object.entries(res.quotas)) next[Number(k)] = v;
          return next;
        });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [rows, revision]);

  async function ensureWindowStats(accountID: number, days: "7" | "30") {
    const key = `${accountID}:${days}`;
    if (statsCache[key] || statsInFlight.current.has(key)) return;
    statsInFlight.current.add(key);
    setLoadingStats(prev => ({...prev, [key]: true}));
    try {
      const res = await api(`/admin/accounts/${accountID}/stats?days=${days}`, accountStatsSummary);
      setStatsCache(prev => ({
        ...prev,
        [key]: {
          requests: res.summary.total_requests,
          tokens: res.summary.total_tokens,
          cost: res.summary.total_cost,
        },
      }));
    } catch {
      // 单卡统计拉取失败时保留「未知」，不打断整页浏览
    } finally {
      statsInFlight.current.delete(key);
      setLoadingStats(prev => {
        const next = {...prev};
        delete next[key];
        return next;
      });
    }
  }

  // 全局周期切到 7d / 30d 时，为当前处于用量视图的卡片按需补拉窗口统计。
  useEffect(() => {
    if (view !== "card" || defaultWindow === "1") return;
    for (const row of rows) {
      const mode = cardModes[row.id] ?? (isQuotaCapable(row, quotaMap[row.id]) ? "quota" : "usage");
      const win = cardWindows[row.id] ?? defaultWindow;
      if (mode === "usage" && (win === "7" || win === "30")) {
        void ensureWindowStats(row.id, win);
      }
    }
  }, [view, defaultWindow, rows, cardModes, cardWindows, quotaMap]);

  // 单个账号的操作：结果写回它自己的卡片或行，不占用页面级提示。
  async function act(row: AccountRow, done: string, work: () => Promise<unknown>) {
    if (busyID !== undefined) return;
    setBusyID(row.id);
    setResults(prev => {
      const next = {...prev};
      delete next[row.id];
      return next;
    });
    try {
      await work();
      setResults(prev => ({...prev, [row.id]: {tone: "ok", text: done}}));
      refresh();
    } catch (failure) {
      setResults(prev => ({...prev, [row.id]: {tone: "bad", text: describeError(failure)}}));
    } finally {
      setBusyID(undefined);
    }
  }

  const post = (row: AccountRow, action: string) => api(`/admin/accounts/${row.id}/${action}`, account, "POST", {});
  const probe = (row: AccountRow) =>
    act(row, "测活通过", async () => {
      let last = "";
      await testAccount(row.id, "", state => {
        last = state;
      });
      if (last !== "passed") throw new Error("测活失败");
    });

  const refreshCardTelemetry = (row: AccountRow) =>
    act(row, canRefreshBalance(row) ? "余额与用量已刷新" : isQuotaCapable(row, quotaMap[row.id]) ? "配额已刷新" : "用量已刷新", async () => {
      if (canRefreshBalance(row)) {
        const bal = await api(`/admin/accounts/${row.id}/balance`, accountBalance, "POST", {});
        setBalanceMap(prev => ({...prev, [row.id]: bal}));
      } else if (isQuotaCapable(row, quotaMap[row.id])) {
        const q = await api(`/admin/accounts/${row.id}/quota?source=active`, accountQuota);
        setQuotaMap(prev => ({...prev, [row.id]: q}));
      }
      const win = cardWindows[row.id] ?? defaultWindow;
      if (win === "7" || win === "30") {
        const res = await api(`/admin/accounts/${row.id}/stats?days=${win}`, accountStatsSummary);
        setStatsCache(prev => ({
          ...prev,
          [`${row.id}:${win}`]: {
            requests: res.summary.total_requests,
            tokens: res.summary.total_tokens,
            cost: res.summary.total_cost,
          },
        }));
      }
    });

  const groupsOf = (row: AccountRow) =>
    (row.groups.length ? row.groups : row.group_ids.map(id => ({id, name: `分组 #${id}`})))
      .map(group => groups?.value?.find(item => item.id === group.id) ?? group);

  const scheduleSwitch = (row: AccountRow) => (
    <Switch
      label={`${row.schedulable ? "暂停" : "开启"} ${row.name} 调度`}
      checked={row.schedulable}
      disabled={busyID !== undefined}
      onChange={checked =>
        void act(row, checked ? "已开启调度" : "已暂停调度", () =>
          api(`/admin/accounts/${row.id}/scheduling`, empty, "PUT", {schedulable: checked}),
        )
      }
    />
  );

  const rowActions = (row: AccountRow, card = false) => {
    const busy = busyID === row.id;
    const blocked = busyID !== undefined && !busy;
    return (
      <>
        {card && (
          <Button size="sm" variant="ghost" disabled={blocked} onClick={() => setModelsAccount(row)}>
            <Boxes aria-hidden />模型
          </Button>
        )}
        <Button size="icon-sm" variant="ghost" title="测活" aria-label={`测活 ${row.name}`} busy={busy} disabled={blocked} onClick={() => void probe(row)}>
          <Zap />
        </Button>
        {card && <Button
          size="icon-sm"
          variant="ghost"
          title={canRefreshBalance(row) ? "刷新余额与用量" : "刷新配额与用量"}
          aria-label={`刷新 ${row.name}`}
          disabled={busyID !== undefined}
          onClick={() => void refreshCardTelemetry(row)}
        >
          <RefreshMorphIcon />
        </Button>}
      </>
    );
  };

  const rowMore = (row: AccountRow, card = false) => {
    const problem = accountProblem(row);
    const detail = <Button size="icon-sm" variant="ghost" title="详情" aria-label={`打开 ${row.name} 的详情`} onClick={() => openAccount(row.id)}>
      <PanelRight />
    </Button>;
    return (
    <>
      {card ? detail : <SecondaryTableAction>{detail}</SecondaryTableAction>}
      <MoreActions onOpenChange={(open, trigger) => {
        setRowMenuOpen(open);
        if (open) deletionTrigger.current = trigger;
      }} label={`${row.name} 的更多操作`}>
        {!card && <>
          <MenuItem onSelect={() => openAccount(row.id)}><PanelRight aria-hidden />打开详情</MenuItem>
          <MenuItem onSelect={() => setModelsAccount(row)}>查看模型</MenuItem>
          <MenuItem disabled={busyID !== undefined} onSelect={() => void refreshCardTelemetry(row)}>{canRefreshBalance(row) ? "刷新余额与用量" : "刷新配额与用量"}</MenuItem>
        </>}
        <MenuItem disabled={busyID !== undefined} onSelect={() => void act(row, "凭据已刷新", () => post(row, "refresh"))}>刷新凭据</MenuItem>
        {problem && problem.action !== "recover" && problem.action !== "edit" && <MenuItem disabled={busyID !== undefined} onSelect={() => recoverAccount(row)}>{recoveryLabel(row)}</MenuItem>}
        <MenuItem disabled={busyID !== undefined} onSelect={() => void act(row, "已清错并恢复", () => post(row, "recover"))}><Wrench aria-hidden />清错并恢复</MenuItem>
        <MenuSeparator />
        <MenuItem onSelect={() => navigateTo("usage", {account: row.id})}>查看调用明细</MenuItem>
        <MenuItem onSelect={() => void act(row, "已复制为新账号", () => post(row, "duplicate"))}><DuplicateIcon aria-hidden />复制账号</MenuItem>
        <MenuItem onSelect={() => void act(row, "已清除限流", () => post(row, "clear-rate-limit"))}>清除限流</MenuItem>
        <MenuItem onSelect={() => void act(row, "已重置配额", () => post(row, "reset-quota"))}>重置配额</MenuItem>
        <MenuSeparator />
        <MenuItem tone="bad" onSelect={() => { removeAction.clearError(); setDeleting(row); }}><Trash2 aria-hidden />删除账号</MenuItem>
      </MoreActions>
    </>
    );
  };

  const providerPills = useMemo(() => {
    const known = PLATFORM_OPTIONS.filter(p => p.value !== "composite" && ((platformCounts[p.value] ?? 0) > 0 || p.value === platform));
    return [
      {value: "", label: totals ? `全部 ${totals.total}` : "全部"},
      ...known.map(p => {
        const c = platformCounts[p.value];
        return {value: p.value, label: c !== undefined ? `${p.label} ${c}` : p.label};
      }),
    ];
  }, [platform, platformCounts, totals]);

  function recoveryLabel(row: AccountRow) {
    const problem = accountProblem(row);
    return problem?.action === "edit" ? "查看详情" : problem?.action === "recover" ? "恢复" : "解除异常";
  }

  function recoverAccount(row: AccountRow) {
    const problem = accountProblem(row);
    if (!problem) return;
    if (problem.action === "edit") { openAccount(row.id); return; }
    void act(row, "已恢复运行状态", () =>
      problem.action === "revert-proxy-fallback" ? api("/admin/accounts/" + row.id + "/revert-proxy-fallback", empty, "POST", {}) :
      problem.action === "temp-unschedulable" ? api("/admin/accounts/" + row.id + "/temp-unschedulable", empty, "DELETE") : post(row, problem.action),
    );
  }

  function accountRecovery(row: AccountRow) {
    return <Button size="xs" variant="ghost" disabled={busyID !== undefined} onClick={() => recoverAccount(row)}>{recoveryLabel(row)}</Button>;
  }

  const columns: Column<AccountRow>[] = [
    {
      id: "name",
      title: "上游账号",
      render: row => (
        <div>
          <strong className="compact-text" title={row.name}>{row.name}</strong>
          <EntityResult result={results[row.id]} fallback={row.notes} />
        </div>
      ),
    },
    {
      id: "platform",
      title: "平台与接入",
      hideWhenCompact: true,
      render: row => {
        const bal = row.balance?.kind ? row.balance : balanceMap[row.id] ?? row.balance;
        return (
          <div className="inline-stack">
            <PlatformBadge platform={row.platform} type={row.type} />
            {bal && <Badge variant="tag" title={balanceHint(bal)}><span className="compact-model">{balanceText(bal)}</span></Badge>}
          </div>
        );
      },
    },
    {
      id: "relations",
      title: "分组 / 代理",
      hideWhenNarrow: true,
      render: row => (
        <InlineStack density="compact" className="table-relations" onClick={event => event.stopPropagation()}>
          <PlanSummary label="分组" empty="未绑组" items={groupsOf(row)} />
          {row.proxy_id ? <EntityLink kind="proxy" id={row.proxy_id} label={row.proxy_name || `代理 #${row.proxy_id}`} /> : null}
        </InlineStack>
      ),
    },
    {
      id: "today",
      title: "今日请求 / 消耗",
      hideWhenNarrow: true,
      align: "number",
      render: row =>
        row.today_cost === undefined ? (
          <span className="muted">—</span>
        ) : (
          <span className="weir-digits">
            {count.format(row.today_requests ?? 0)} 次 · {smartMoney(row.today_cost)}
          </span>
        ),
    },
    {
      id: "watermark",
      title: "配额水位 (5h / 7d)",
      hideWhenNarrow: true,
      render: row =>
        row.five_hours_used === null && row.week_used === null ? (
          <span className="muted">—</span>
        ) : (
          <span className="code-mono text-fine weir-digits">
            {percent(row.five_hours_used)} / {percent(row.week_used)}
          </span>
        ),
    },
    {id: "conc", title: "并发", align: "number", render: row => concurrencyLabel(row)},
    {
      id: "status",
      title: "状态",
      align: "center",
      render: row => {
        const problem = accountProblem(row);
        return <div className="account-status-cell" onClick={event => event.stopPropagation()}>
          {problem ? <><Badge variant="soft" tone={problem.tone}>{problem.label}</Badge><SecondaryTableAction>{accountRecovery(row)}</SecondaryTableAction></> : <Status value={row.status} />}
          {row.activity_known !== false && row.current_concurrency > 0 && <ActivityWire />}
        </div>;
      },
    },
    {id: "sched", title: "调度", hideWhenNarrow: true, render: row => <span onClick={event => event.stopPropagation()}>{scheduleSwitch(row)}</span>},
    {id: "ops", title: "", align: "end", render: row => <RowActions>{rowActions(row)}{rowMore(row)}</RowActions>},
  ];

  const index = inspect ? rows.findIndex(row => row.id === inspect) : -1;
  const emptyState = (
    <EmptyState
      title={filtered ? "没有符合筛选条件的账号" : "还没有上游账号"}
      action={
        filtered
          ? <Button size="sm" variant="outline" onClick={resetFilters}>清除筛选</Button>
          : <Button size="sm" onClick={() => setAddOpen(true)}><Plus aria-hidden />添加账号</Button>
      }
    />
  );

  function renderCard(row: AccountRow) {
    const quota = quotaMap[row.id];
    const hasQuota = isQuotaCapable(row, quota);
    const mode: CardMode = cardModes[row.id] ?? (hasQuota ? "quota" : "usage");
    const win: UsageWindow = cardWindows[row.id] ?? defaultWindow;
    const bal = row.balance?.kind ? row.balance : balanceMap[row.id] ?? row.balance;
    const hold = accountHold(row);

    const problem = accountProblem(row);
    const banner = <>
      <EllipsisText text={problem?.label ?? (row.schedulable ? "可调度" : "已暂停调度")} className="entity-card-banner-label" />
      {problem && accountRecovery(row)}
    </>;

    const modeSwitch = hasQuota ? (
      <MiniPills
        label={`${row.name} 展示维度`}
        value={mode}
        options={[
          ["quota", "配额"],
          ["usage", "用量"],
        ] as const}
        onChange={next => {
          setCardModes(prev => ({...prev, [row.id]: next}));
          if (next === "usage" && (win === "7" || win === "30")) {
            void ensureWindowStats(row.id, win);
          }
        }}
      />
    ) : (
      <EllipsisText text={bal ? `调用用量 · ${balanceText(bal)}` : "调用用量"} tip={bal ? balanceHint(bal) : undefined} />
    );

    const panelHead =
      mode === "quota" ? (
        <>
          {modeSwitch}
          <span>{quota?.subscription_tier ? quota.subscription_tier : quotaDisplay === "remaining" ? "剩余" : "已用"}</span>
        </>
      ) : (
        <>
          {modeSwitch}
          <MiniPills
            label={`${row.name} 用量周期`}
            value={win}
            options={[
              ["1", "今日"],
              ["7", "7d"],
              ["30", "30d"],
            ] as const}
            onChange={next => {
              setCardWindows(prev => ({...prev, [row.id]: next}));
              if (next === "7" || next === "30") {
                void ensureWindowStats(row.id, next);
              }
            }}
          />
        </>
      );

    const statKey = `${row.id}:${win}`;
    const cachedWin = win === "1"
      ? row.today_cost !== undefined || row.today_requests !== undefined
        ? {requests: row.today_requests ?? 0, tokens: row.today_tokens ?? 0, cost: row.today_cost ?? 0}
        : undefined
      : statsCache[statKey];
    const statBusy = win !== "1" && loadingStats[statKey];

    const requests = statBusy ? "…" : cachedWin ? count.format(cachedWin.requests) : "未知";
    const errors = win === "1" && (row.today_errors ?? 0) > 0 ? " · 错 " + row.today_errors : "";
    const panelBody = mode === "quota"
      ? <EntityMeters items={buildQuotaMeters(row, quota, quotaDisplay)} />
      : <EntityMetrics items={[
        {label: win === "1" ? "今日请求" : "近 " + win + " 天请求", value: requests + errors},
        {label: "Token", value: statBusy ? "…" : cachedWin ? compactTokens(cachedWin.tokens) : "未知"},
        {label: "实际消耗", value: statBusy ? "…" : cachedWin ? smartMoney(cachedWin.cost) : "未知"},
      ]} />;

    const metaItems = [
      `优先级 ${row.priority}`,
      ...(row.rate_multiplier !== 1 ? [`${row.rate_multiplier}×`] : []),
      `并发 ${row.current_concurrency}/${row.concurrency || "∞"}`,
      <PlanSummary key="groups" label="分组" empty="未绑组" items={groupsOf(row)} />,
      row.proxy_id
        ? <EntityLink key="proxy" kind="proxy" id={row.proxy_id} label={row.proxy_name || `代理 #${row.proxy_id}`} />
        : "直连",
      ...(row.last_used_at ? [`最近 ${date(row.last_used_at)}`] : []),
    ];

    return (
      <div role="listitem" key={row.id} data-motion-key={row.id}>
        <EntityCard
          name={row.name}
          lead={<StatusDot tone={row.status === "active" ? (hold ? "warn" : "ok") : row.status === "error" ? "bad" : "neutral"} />}
          toggle={<>{row.activity_known !== false && row.current_concurrency > 0 && <ActivityWire />}{scheduleSwitch(row)}</>}
          selected={selected.includes(row.id)}
          onSelect={next => setSelected(next ? [...selected, row.id] : selected.filter(id => id !== row.id))}
          active={inspect === row.id}
          dim={!row.schedulable}
          onOpen={() => setInspect(row.id)}
          badges={
            <>
              <PlatformBadge platform={row.platform} type={row.type} />
              {row.status !== "active" && <Status value={row.status} />}
              {bal && hasQuota && (
                <Tooltip content={balanceHint(bal)} side="top">
                  <Badge variant="tag">{balanceText(bal)}</Badge>
                </Tooltip>
              )}
              {bal && bal.available === false && <Badge variant="soft" tone="bad">余额不可用</Badge>}
              {row.expires_at != null && row.expires_at * 1000 - Date.now() < 7 * 86400000 && (
                <Badge variant="soft" tone={row.expires_at * 1000 < Date.now() ? "bad" : "warn"}>
                  {row.expires_at * 1000 < Date.now() ? "已到期" : "即将到期"}
                </Badge>
              )}
            </>
          }
          banner={banner}
          bannerTone={problem?.tone}
          panelHead={panelHead}
          panelBody={panelBody}
          panelSize={hasQuota ? "quota" : "summary"}
          meta={metaItems}
          result={results[row.id]}
          reserveResult
          actions={rowActions(row, true)}
          more={rowMore(row, true)}
        />
      </div>
    );
  }

  return (
    <>
      <WorkspaceStage
        flush
        inspectorOpen={!!inspect}
        inspector={
          <Inspector
            open={!!inspect}
            title={rows[index]?.name ?? "上游账号"}
            subtitle={inspect ? `上游账号 · #${inspect}` : undefined}
            onClose={() => setInspect(undefined)}
            onPrev={index > 0 ? () => setInspect(rows[index - 1].id) : undefined}
            onNext={index >= 0 && index < rows.length - 1 ? () => setInspect(rows[index + 1].id) : undefined}
          >
            {inspect && <AccountSummary id={inspect} row={rows[index]} revision={revision} onShowModels={acc => setModelsAccount(acc)} />}
          </Inspector>
        }
      >
        <MetricStrip
          flush
          items={[
            {id: "total", label: "账号总数", value: totals ? count.format(totals.total) : null, onClick: resetFilters},
            {id: "normal", label: "可调度", value: totals ? count.format(totals.normal) : null, tone: "ok", onClick: () => setStatus("active")},
            ...(totals && (totals.rate_limited > 0 || totals.error > 0 || totals.overloaded > 0)
              ? [
                  {id: "limited", label: "限流中", value: count.format(totals.rate_limited), tone: (totals.rate_limited ? "warn" : "neutral") as "warn" | "neutral", onClick: () => setStatus("rate_limited")},
                  {id: "error", label: "异常", value: count.format(totals.error), tone: (totals.error ? "bad" : "neutral") as "bad" | "neutral", onClick: () => setStatus("error")},
                  {id: "overloaded", label: "过载", value: count.format(totals.overloaded), tone: (totals.overloaded ? "warn" : "neutral") as "warn" | "neutral"},
                ]
              : [
                  {id: "health", label: "异常与限流", value: totals ? "无异常" : null, hint: totals ? "限流 · 报错 · 过载均为 0" : undefined, tone: "ok" as const},
                ]),
          ]}
        />
        <FilterBar sticky>
          <div className="obs-filter-row">
            <QuickPills label="上游平台" value={platform} options={providerPills} onChange={value => updateAccountQuery({platform: value})} />
            <QuickPills label="状态筛选" value={status} options={statusPills} onChange={value => updateAccountQuery({status: value})} />
            <div className="obs-filter-spacer">
              {view === "card" && (
                <>
                  <Segmented
                    label="默认统计周期"
                    value={defaultWindow}
                    options={[
                      ["1", "今日"],
                      ["7", "7d"],
                      ["30", "30d"],
                    ] as const}
                    onChange={next => {
                      setDefaultWindow(next);
                      setCardWindows({});
                    }}
                  />
                  <Segmented
                    label="配额展示口径"
                    value={quotaDisplay}
                    options={[
                      ["used", "已用"],
                      ["remaining", "剩余"],
                    ] as const}
                    onChange={next => {
                      setQuotaDisplay(next);
                      localStorage.setItem(quotaDisplayKey, next);
                    }}
                  />
                </>
              )}
              <ViewToggle
                value={view}
                onChange={next => {
                  setView(next);
                  localStorage.setItem(viewKey, next);
                }}
              />
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新账号" onClick={refresh} />
              <Button ref={addTrigger} onClick={() => setAddOpen(true)}>
                <Plus aria-hidden />添加账号
              </Button>
            </div>
          </div>
          <div className="obs-filter-row">
            <SearchInput value={search} onChange={setSearch} placeholder="搜索名称或备注" ariaLabel="搜索上游账号" />
            <FilterSlot><GroupSelect value={groupFilter} placeholder="全部分组" onChange={value => updateAccountQuery({group: value})} /></FilterSlot>
            <FilterSlot><ProxySelect value={proxyFilter} placeholder="全部代理" onChange={value => updateAccountQuery({proxy: value})} /></FilterSlot>
            <FilterSlot>
              <Select aria-label="账号排序" value={sort} onChange={event => updateAccountQuery({sort: event.target.value})}>
                {sortOptions.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
              </Select>
            </FilterSlot>
          </div>
          <ErrorLine value={batchAction.error} />
        </FilterBar>

        <Toolbar>
          <Button size="sm" variant="outline" onClick={() => setCRSOpen(true)}>同步 CRS</Button>
          <Button size="sm" variant="outline" onClick={() => setAccountData("import")}>导入配置</Button>
          <Button size="sm" variant="outline" onClick={() => setAccountData("export")}>导出配置</Button>
          <Button size="sm" variant="outline" busy={batchAction.busy} onClick={() => void batchAction.run(async () => {
            const matching = await api(query.replace("/admin/accounts", "/admin/accounts/batch-targets"), z.array(named));
            if (matching.length === 0) throw new Error("没有符合当前筛选条件的账号");
            setBatchTargets(matching);
          })}>批量处理筛选结果</Button>
        </Toolbar>

        <BatchBar count={selected.length} onClear={() => setSelected([])}>
          <Button size="xs" variant="outline" onClick={() => setBulkBindOpen(true)}>改分组 / 代理</Button>
          {(
            [
              ["batch-clear-error", "清错", {ids: selected}, "已清错"],
              ["batch-refresh", "刷新凭据", {ids: selected}, "已刷新凭据"],
              ["bulk-update", "暂停调度", {account_ids: selected, schedulable: false}, "已暂停调度"],
            ] as const
          ).map(([path, label, body, done]) => (
            <Button
              key={path}
              size="xs"
              variant="outline"
              busy={batchAction.busy}
              onClick={() =>
                void batchAction.run(async () => {
                  await api(`/admin/accounts/${path}`, z.object({success: z.number(), failed: z.number()}), "POST", body);
                  toast(`${selected.length} 个账号${done}`);
                  refresh();
                })
              }
            >
              {label}
            </Button>
          ))}
          <Button size="xs" variant="destructive" onClick={event => { deletionTrigger.current = event.currentTarget; removeAction.clearError(); setDeleting("batch"); }}>
            <Trash2 aria-hidden />删除
          </Button>
        </BatchBar>

        <div className={view === "list" ? "swift-pane-table account-results" : "account-results"}
          onFocusCapture={() => setRowInteraction(true)}
          onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setRowInteraction(false); }}
          onPointerDownCapture={() => setRowInteraction(true)}
          onPointerLeave={() => { if (!document.activeElement?.closest(".account-results")) setRowInteraction(false); }}>

          <RemoteTable
            path={query}
            schema={account}
            columns={columns}
            rowID={row => row.id}
            label="上游账号"
            revision={revision}
            onItemsLoaded={setRows}
            livePath="/admin/accounts/stream"
            onLiveStatus={setLiveStatus}
            holdLiveUpdates={rowInteraction || rowMenuOpen || selected.length > 0 || addOpen || bulkBindOpen || crsOpen || !!accountData || !!batchTargets || !!modelsAccount || !!deleting || busyID !== undefined || batchAction.busy}
            empty={emptyState}
            renderItems={(items, stale) => {
              const activityAvailable = liveStatus !== "reconnecting" && liveStatus !== "denied";
              const ordered = sortAccounts(activityAvailable ? items : items.map(row => ({...row, activity_known: false})), sort);
              return <ReorderMotion key={view} order={ordered.map(item => item.id).join(",")}>
                {view === "list" ? (
                  <DataTable label="上游账号" rows={ordered} columns={columns} rowID={row => row.id}
                    activeRowID={inspect} onRowClick={row => setInspect(row.id)} selection={{selected, onChange: setSelected}}
                    stale={stale} motion pinActions />
                ) : (
                  <EntityGrid label="上游账号">{ordered.map(renderCard)}</EntityGrid>
                )}
              </ReorderMotion>;
            }}
          />
        </div>
      </WorkspaceStage>

      <AccountModelsDialog accountRow={modelsAccount} onClose={() => setModelsAccount(undefined)} />
      {batchTargets && <AccountBatch targets={batchTargets} onClose={() => setBatchTargets(undefined)} onChanged={refresh} />}
      {crsOpen && <AccountCRS onClose={() => setCRSOpen(false)} onChanged={refresh} />}
      {accountData && <AccountData mode={accountData} selected={selected} onClose={() => setAccountData(undefined)} onChanged={refresh} />}

      <AddAccountDialog
        open={addOpen}
        revision={revision}
        defaultGroupID={groupFilter ? Number(groupFilter) : undefined}
        defaultProxyID={proxyFilter ? Number(proxyFilter) : undefined}
        onClose={() => setAddOpen(false)}
        onCreated={createdID => {
          setAddOpen(false);
          refresh();
          if (createdID) setInspect(createdID);
        }}
        onInlineResourceCreated={refresh}
      />

      <BulkBindAccountsDialog
        open={bulkBindOpen}
        ids={selected}
        revision={revision}
        onClose={() => setBulkBindOpen(false)}
        onSaved={() => {
          setBulkBindOpen(false);
          setSelected([]);
          toast("已更新所选账号的分组与代理");
          refresh();
        }}
      />

      <ConfirmDialog
        open={!!deleting}
        title="删除上游账号"
        description={
          deleting === "batch"
            ? `确定删除选中的 ${selected.length} 个上游账号吗？删除后无法恢复。`
            : `确定删除「${deleting?.name ?? ""}」吗？删除后无法恢复。`
        }
        confirm="删除"
        busy={removeAction.busy}
        error={removeAction.error}
        finalFocus={() => deletionTrigger.current?.isConnected ? deletionTrigger.current : addTrigger.current}
        onClose={() => setDeleting(undefined)}
        onConfirm={() =>
          void removeAction.run(async () => {
            if (deleting === "batch") {
              await api("/admin/accounts/batch-delete", z.object({deleted: z.number()}), "POST", {ids: selected});
              setSelected([]);
            } else if (deleting) {
              await api(`/admin/accounts/${deleting.id}`, empty, "DELETE");
              if (inspect === deleting.id) setInspect(undefined);
            }
            setDeleting(undefined);
            refresh();
          })
        }
      />
    </>
  );
}

function AccountModelsDialog({accountRow, onClose}: {accountRow?: AccountRow; onClose: () => void}) {
  const [keyword, setKeyword] = useState("");
  const [syncRevision, setSyncRevision] = useState(0);
  const syncOp = useAction();

  useEffect(() => {
    setKeyword("");
  }, [accountRow?.id]);

  const models = useResource(
    () => (accountRow ? api(`/admin/accounts/${accountRow.id}/models`, z.array(z.object({id: z.string(), display_name: z.string()}))) : Promise.resolve([])),
    accountRow ? `acc-models-modal-${accountRow.id}` : "acc-models-none",
    !!accountRow,
    syncRevision,
  );

  const needle = keyword.trim().toLocaleLowerCase();
  const list = (models?.value ?? []).filter(
    m => !needle || m.id.toLocaleLowerCase().includes(needle) || m.display_name.toLocaleLowerCase().includes(needle),
  );

  return (
    <Overlay
      open={!!accountRow}
      title={accountRow ? `可用模型 · ${accountRow.name}` : "可用模型"}
      onClose={onClose}
      busy={syncOp.busy}
      footer={
        <>
          <Button variant="outline" disabled={syncOp.busy} onClick={onClose}>关闭</Button>
          {accountRow && (
            <Button
              busy={syncOp.busy}
              onClick={() =>
                void syncOp.run(async () => {
                  const res = await api(`/admin/accounts/${accountRow.id}/models/sync`, z.object({models: z.array(z.string())}), "POST", {});
                  toast(`已同步 ${res.models.length} 个上游模型`);
                  setSyncRevision(v => v + 1);
                })
              }
            >
              <RefreshMorphIcon />同步上游模型
            </Button>
          )}
        </>
      }
    >
      <SearchInput value={keyword} onChange={setKeyword} placeholder="搜索模型名称或 ID" ariaLabel="搜索可用模型" />
      <ErrorLine value={syncOp.error} />
      <Resource state={models}>
        {all =>
          list.length === 0 ? (
            <EmptyState title={all.length === 0 ? "暂无可用模型" : "没有匹配的模型"} />
          ) : (
            <InsetGroup title={`共 ${list.length} 个模型`}>
              {list.map(item => (
                <InsetRow key={item.id} label={item.display_name || item.id} hint={item.display_name && item.display_name !== item.id ? item.id : undefined}>
                  <CopyButton label={`复制模型 ${item.id}`} getText={async () => item.id} />
                </InsetRow>
              ))}
            </InsetGroup>
          )
        }
      </Resource>
    </Overlay>
  );
}

function AccountSummary({
  id,
  row,
  revision,
  onShowModels,
}: {
  id: number;
  row?: AccountRow;
  revision: number;
  onShowModels: (acc: AccountRow) => void;
}) {
  const detail = useResource(() => api(`/admin/accounts/${id}`, account), `acc-detail-${id}`, true, revision);
  const stats7d = useResource(() => api(`/admin/accounts/${id}/stats?days=7`, accountStatsSummary), `acc-sum-7d-${id}`, true, revision);
  return (
    <Resource state={detail}>
      {value => {
        const bal = value.balance ?? row?.balance;
        return (
          <>
            <InsetGroup title="身份与路由">
              <InsetRow label="平台与接入方式"><PlatformBadge platform={value.platform} type={value.type} /></InsetRow>
              <InsetRow label="状态"><Status value={value.status} /></InsetRow>
              <InsetRow label="并发"><span className="weir-digits">{value.current_concurrency} / {value.concurrency || "不限"}</span></InsetRow>
              <InsetRow label="优先级 / 倍率"><span className="weir-digits">{value.priority} · {value.rate_multiplier}×</span></InsetRow>
              <InsetRow label="所属分组"><PlanSummary label="分组" items={value.groups} empty="未绑组" /></InsetRow>
              {bal && (
                <InsetRow label="渠道余额">
                  <span className="weir-digits">
                    {bal.balances && bal.balances.length > 1
                      ? bal.balances.map(b => currencyMoney(b.balance, b.currency)).join(" · ")
                      : currencyMoney(bal.balance, bal.currency)}
                  </span>
                </InsetRow>
              )}
            </InsetGroup>
            <InsetGroup title="消耗摘要">
              {row?.today_cost !== undefined && (
                <InsetRow label="今日调用">
                  <span className="weir-digits">
                    {count.format(row.today_requests ?? 0)} 次 · {compactTokens(row.today_tokens)} Token · {smartMoney(row.today_cost)}
                  </span>
                </InsetRow>
              )}
              {stats7d?.value && (
                <InsetRow label="近 7 天调用">
                  <span className="weir-digits">
                    {count.format(stats7d.value.summary.total_requests)} 次 · {compactTokens(stats7d.value.summary.total_tokens)} Token · {smartMoney(stats7d.value.summary.total_cost)}
                  </span>
                </InsetRow>
              )}
            </InsetGroup>
            <Toolbar>
              <Button size="sm" onClick={() => navigateTo("accounts", {detail: id})}><Zap aria-hidden />深测与高级配置</Button>
              <Button size="sm" variant="outline" onClick={() => onShowModels(value)}><Boxes aria-hidden />可用模型</Button>
              <Button variant="link" onClick={() => navigateTo("usage", {account: id})}>查看用量</Button>
              <Button variant="link" onClick={() => navigateTo("ops", {tab: "errors", account_id: id})}>排查报错</Button>
            </Toolbar>
          </>
        );
      }}
    </Resource>
  );
}

export function AccountDetailWorkspace({id, queryString}: {id: number; queryString: string}) {
  const [revision, revise] = useState(0);
  const params = Object.fromEntries(new URLSearchParams(queryString));
  const tab = params.tab === "config" ? "config" : "health";
  const back = () => {
    const listParams = {...params};
    delete listParams.tab;
    delete listParams.detail;
    navigateTo("accounts", listParams);
  };
  return (
    <div className="workspace-section">
      <DetailHeader backLabel="返回上游账号" title={`账号 #${id}`} onBack={back} />
      <AccountInspector id={id} revision={revision} subTab={tab} onTabChange={next => navigateTo("accounts", {...params, detail: id, tab: next === "health" ? undefined : next})}
        onChanged={() => revise(n => n + 1)} onDeleted={back} />
    </div>
  );
}

function AccountInspector({
  id,
  revision,
  subTab,
  onTabChange,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  subTab: "health" | "config";
  onTabChange: (next: "health" | "config") => void;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const [quotaSource, setQuotaSource] = useState<"passive" | "active">("passive");
  const [statsDays, setStatsDays] = useState<"1" | "7" | "30">("7");
  const [testModel, setTestModel] = useState("");
  const [testEngine, setTestEngine] = useState<"official" | "custom" | "lmfpd" | "gpt-relay-inspector">("official");
  const [testProtocol, setTestProtocol] = useState("responses");
  const [testEffort, setTestEffort] = useState("");
  const [testPreset, setTestPreset] = useState("fast");
  const [testSysPrompt, setTestSysPrompt] = useState("");
  const [testPrompt, setTestPrompt] = useState("");
  const [probeStatus, setProbeStatus] = useState("");
  const [probeStreamText, setProbeStreamText] = useState("");
  const [probeReport, setProbeReport] = useState<ProbeReport>();
  const [probePass, setProbePass] = useState<z.infer<typeof ephemeralPassSchema>>();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [reauthOpen, setReauthOpen] = useState(false);

  const detail = useResource(() => api(`/admin/accounts/${id}`, account), `acc-detail-${id}`, true, revision);
  const quota = useResource(() => api(`/admin/accounts/${id}/quota?source=${quotaSource}`, accountQuota), `acc-quota-${id}-${quotaSource}`, true, revision);
  const models = useModelCatalog(`/admin/accounts/${id}/models`, revision, subTab === "health", true);
  const stats = useResource(
    () => api(`/admin/accounts/${id}/stats?days=${statsDays}`, accountStatsSummary),
    `acc-stats-${id}-${statsDays}`,
    true,
    revision,
  );
  const op = useAction();
  const del = useAction();

  return (
    <Resource state={detail}>
      {acc => {
        const windows = [
          {label: "5 小时窗口", win: quota?.value?.five_hours ?? null},
          {label: "7 天窗口", win: quota?.value?.week ?? null},
          ...(quota?.value?.sonnet_week ? [{label: "Sonnet 7d 窗口", win: quota.value.sonnet_week}] : []),
          ...(quota?.value?.gemini_daily ? [{label: "Gemini 每日配额", win: quota.value.gemini_daily}] : []),
          ...(quota?.value?.gemini_pro_daily ? [{label: "Gemini Pro 日配额", win: quota.value.gemini_pro_daily}] : []),
          ...(quota?.value?.thirty_day ? [{label: "30 天账单配额", win: quota.value.thirty_day}] : []),
        ];
        return (
          <>
            <h2 className="stage-heading">{acc.name}</h2>
            <Tabs
              label="账号详情分页"
              value={subTab}
              items={[
                {value: "health", label: "水位、用量与恢复"},
                {value: "config", label: "归属与参数"},
              ]}
              onChange={onTabChange}
            />

            {subTab === "health" ? (
              <>
                <InsetGroup
                  title="调用消耗与配额水位"
                  actions={
                    <InlineStack>
                      <MiniPills
                        label="统计天数"
                        value={statsDays}
                        options={[
                          ["1", "今日"],
                          ["7", "7d"],
                          ["30", "30d"],
                        ] as const}
                        onChange={setStatsDays}
                      />
                      <Button size="xs" variant="ghost" onClick={() => setQuotaSource(prev => (prev === "passive" ? "active" : "passive"))}>
                        {quotaSource === "passive" ? "主动探测配额" : "切回被动快照"}
                      </Button>
                    </InlineStack>
                  }
                >
                  {stats?.value && (
                    <InsetRow label={statsDays === "1" ? "今日承载调用" : `近 ${statsDays} 天承载调用`}>
                      <span className="weir-digits">
                        {count.format(stats.value.summary.total_requests)} 次 · {compactTokens(stats.value.summary.total_tokens)} Token · {smartMoney(stats.value.summary.total_cost)}
                      </span>
                    </InsetRow>
                  )}
                  {acc.balance && (
                    <InsetRow label="上游渠道余额">
                      <span className="weir-digits">
                        {acc.balance.balances && acc.balance.balances.length > 1
                          ? acc.balance.balances.map(b => currencyMoney(b.balance, b.currency)).join(" · ")
                          : currencyMoney(acc.balance.balance, acc.balance.currency)}
                      </span>
                    </InsetRow>
                  )}
                  {isQuotaCapable(acc, quota?.value) &&
                    windows.map(w => (
                      <InsetRow key={w.label} label={w.label} hint={w.win?.resets_at ? `重置于 ${relativeUntil(w.win.resets_at)}` : undefined}>
                        <MeterBar size="sm" value={w.win?.percent_used ?? null} label={percent(w.win?.percent_used ?? null)} />
                      </InsetRow>
                    ))}
                  <InsetRow label="跨工作台联动">
                    <InlineStack>
                      <Button size="xs" variant="outline" onClick={() => navigateTo("usage", {account: acc.id})}>查账单</Button>
                      <Button size="xs" variant="outline" onClick={() => navigateTo("ops", {tab: "errors", account_id: acc.id})}>查报错</Button>
                    </InlineStack>
                  </InsetRow>
                </InsetGroup>

                <InsetGroup title="一键状态恢复与凭据续期">
                  <InsetRow label="运行恢复动作" vertical>
                    <InlineStack wrap>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/recover`, account, "POST", {});
                            toast("已恢复账号运行状态");
                            onChanged();
                          })
                        }
                      >
                        <Wrench aria-hidden />一键恢复状态
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/clear-error`, account, "POST", {});
                            toast("已清除错误标记");
                            onChanged();
                          })
                        }
                      >
                        清除异常
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/clear-rate-limit`, account, "POST", {});
                            toast("已清除限流状态");
                            onChanged();
                          })
                        }
                      >
                        清除限流
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/temp-unschedulable`, empty, "DELETE");
                            toast("已解除临时冷却");
                            onChanged();
                          })
                        }
                      >
                        解除冷却
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/refresh`, account, "POST", {});
                            toast("已在服务端刷新上游凭据");
                            onChanged();
                          })
                        }
                      >
                        刷新 Token
                      </Button>
                      {canRefreshBalance(acc) && (
                        <Button
                          size="xs"
                          variant="outline"
                          busy={op.busy}
                          onClick={() =>
                            void op.run(async () => {
                              await api(`/admin/accounts/${acc.id}/balance`, accountBalance, "POST", {});
                              toast("已探测刷新上游渠道余额");
                              onChanged();
                            })
                          }
                        >
                          刷新渠道余额
                        </Button>
                      )}
                      {(acc.type === "oauth" || acc.type === "setup-token") && (
                        <Button size="xs" variant="outline" onClick={() => setReauthOpen(true)}>重新 OAuth 授权</Button>
                      )}
                    </InlineStack>
                  </InsetRow>
                </InsetGroup>

                <AccountScheduledTests accountID={acc.id} />
                <FormSection title="多引擎连通性与验真探测（沙箱隔离 · 测完即焚）">
                  <Field label="检测引擎">
                    <Segmented
                      label="检测引擎"
                      value={testEngine}
                      onChange={setTestEngine}
                      options={[
                        ["official", "原生快检"],
                        ["custom", "自定义输入"],
                        ["lmfpd", "LM 指纹"],
                        ["gpt-relay-inspector", "少点套路"],
                      ] as const}
                    />
                  </Field>

                  <Field label="测试模型">
                    <ModelSelect label="测试模型" value={testModel} onChange={setTestModel} placeholder="留空使用平台默认模型" catalog={models} allowCustom />
                  </Field>

                  {testEngine !== "official" && (
                    <InlineStack wrap>
                      <Field label="网关协议">
                        <Select value={testProtocol} onChange={e => setTestProtocol(e.target.value)}>
                          <option value="responses">Responses</option>
                          <option value="chat_completions">Chat Completions</option>
                          <option value="messages">Anthropic Messages</option>
                        </Select>
                      </Field>
                      <Field label="推理强度">
                        <Select value={testEffort} onChange={e => setTestEffort(e.target.value)}>
                          <option value="">默认</option>
                          <option value="low">Low</option>
                          <option value="medium">Medium</option>
                          <option value="high">High</option>
                        </Select>
                      </Field>
                      {testEngine === "gpt-relay-inspector" && (
                        <Field label="取证档位">
                          <Select value={testPreset} onChange={e => setTestPreset(e.target.value)}>
                            <option value="fast">极速快检</option>
                            <option value="full">19 针深测</option>
                          </Select>
                        </Field>
                      )}
                    </InlineStack>
                  )}

                  {testEngine === "custom" && (
                    <>
                      <Field label="System 提示词（可选）">
                        <Input value={testSysPrompt} onChange={e => setTestSysPrompt(e.target.value)} placeholder="如：只回答最终数字，不要输出多余文字" />
                      </Field>
                      <Field label="自定义输入内容（留空默认跑糖果推理题）">
                        <Textarea value={testPrompt} onChange={e => setTestPrompt(e.target.value)} placeholder="突破官方固定测试词，经单号临时隔离沙箱实测真实响应…" />
                      </Field>
                    </>
                  )}

                  <InlineStack wrap>
                    <Button
                      size="sm"
                      busy={op.busy}
                      onClick={() =>
                        void op.run(async () => {
                          setProbeReport(undefined);
                          setProbeStreamText("");
                          setProbeStatus("正在发起探测…");
                          const rep = await streamProbe(
                            `/admin/accounts/${acc.id}/probe`,
                            {
                              engine: testEngine,
                              model: testModel.trim(),
                              protocol: testProtocol,
                              effort: testEffort,
                              preset: testPreset,
                              system_prompt: testSysPrompt,
                              prompt: testPrompt,
                            },
                            {
                              onStatus: setProbeStatus,
                              onDelta: chunk => setProbeStreamText(prev => (prev.length < 2400 ? prev + chunk : prev)),
                            },
                          );
                          setProbeStatus("");
                          setProbeReport(rep);
                        })
                      }
                    >
                      开始探测
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      busy={op.busy}
                      onClick={() =>
                        void op.run(async () => {
                          const pass = await api(`/admin/accounts/${acc.id}/probe-pass`, ephemeralPassSchema, "POST", {});
                          setProbePass(pass);
                        })
                      }
                    >
                      签发 10 分钟外部验号通行证
                    </Button>
                  </InlineStack>

                  {probeStatus && <Feedback>{probeStatus}</Feedback>}

                  {probeReport && (
                    <InsetGroup title="探测结论">
                      <InsetRow label="判定状态">
                        <Badge variant="soft" tone={probeReport.verdict === "ok" ? "ok" : probeReport.verdict === "degraded" ? "warn" : "bad"}>
                          {probeReport.verdict === "ok" ? "正常通过" : probeReport.verdict === "degraded" ? "降智或特征偏差" : "探测失败"}
                        </Badge>
                      </InsetRow>
                      <InsetRow label="耗时与首字">
                        <span className="weir-digits">{`${(probeReport.duration_ms / 1000).toFixed(2)}s · 首字 ${probeReport.ttft_ms}ms`}</span>
                      </InsetRow>
                      {(probeReport.input_tokens > 0 || probeReport.output_tokens > 0) && (
                        <InsetRow label="Token 消耗">
                          <span className="weir-digits">{`输入 ${probeReport.input_tokens} · 输出 ${probeReport.output_tokens}${probeReport.reasoning_tokens > 0 ? `（推理 ${probeReport.reasoning_tokens}）` : ""}`}</span>
                        </InsetRow>
                      )}
                      {probeReport.predicted_model && (
                        <InsetRow label="指纹识别模型">
                          <span>{`${probeReport.predicted_model}${probeReport.confidence ? `（${(probeReport.confidence * 100).toFixed(1)}%）` : ""}`}</span>
                        </InsetRow>
                      )}
                      <InsetRow label="结论摘要">
                        <span>{probeReport.summary}</span>
                      </InsetRow>
                    </InsetGroup>
                  )}

                  {(probeReport?.reply || probeStreamText) && (
                    <div className="patrol-reply-box">{probeReport?.reply || probeStreamText}</div>
                  )}

                  {probePass && (
                    <InsetGroup title={`一次性外部验号通行证（限额 ${probePass.quota_usd} · 到期自动销毁）`}>
                      <InsetRow label="网关 Base URL" hint={probePass.base_url}>
                        <CopyButton value={probePass.base_url} label="复制地址" />
                      </InsetRow>
                      <InsetRow label="短效探测 Key">
                        <CopyButton value={probePass.api_key} label="复制临时密钥" />
                      </InsetRow>
                      <InsetRow label="外部公信力复核">
                        <InlineStack wrap>
                          <Button size="xs" variant="outline" onClick={() => window.open(probePass.inspector_url, "_blank", "noopener,noreferrer")}>
                            打开少点套路
                          </Button>
                          <Button size="xs" variant="outline" onClick={() => window.open(probePass.ikale_url, "_blank", "noopener,noreferrer")}>
                            打开 LM Detector
                          </Button>
                        </InlineStack>
                      </InsetRow>
                    </InsetGroup>
                  )}
                </FormSection>

                <ErrorLine value={op.error} />
              </>
            ) : (
              <>
              {acc.type === "apikey" && <AccountBalanceSettings account={acc} revision={revision} onRefreshed={onChanged} />}
              <AccountConfigForm
                acc={acc}
                revision={revision}
                onSaved={() => {
                  toast("账号配置已更新");
                  onChanged();
                }}
                onRequestDelete={() => setConfirmDelete(true)}
              />
              </>
            )}

            <AddAccountDialog
              open={reauthOpen}
              revision={revision}
              reauthAccount={acc}
              onClose={() => setReauthOpen(false)}
              onCreated={() => {
                setReauthOpen(false);
                toast("OAuth 重新授权成功，凭据已在服务端更新");
                onChanged();
              }}
              onInlineResourceCreated={onChanged}
            />

            <ConfirmDialog
              open={confirmDelete}
              title="删除上游账号"
              description={`确定删除账号「${acc.name}」吗？`}
              confirm="删除"
              busy={del.busy}
              error={del.error}
              onClose={() => setConfirmDelete(false)}
              onConfirm={() =>
                void del.run(async () => {
                  await api(`/admin/accounts/${acc.id}`, empty, "DELETE");
                  setConfirmDelete(false);
                  onDeleted();
                })
              }
            />
          </>
        );
      }}
    </Resource>
  );
}

const scheduledTest = z.object({id: z.number(), model_id: z.string(), cron_expression: z.string(), enabled: z.boolean(), auto_recover: z.boolean(), next_run_at: z.string().nullable()});
const scheduledResults = z.array(z.object({id: z.number(), status: z.string(), latency_ms: z.number(), started_at: z.string()}));

function AccountScheduledTests({accountID}: {accountID: number}) {
  const path = "/admin/accounts/" + accountID + "/scheduled-tests";
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<z.infer<typeof scheduledTest>>();
  const [deleting, setDeleting] = useState<number>();
  const [resultsID, setResults] = useState<number>();
  const [revision, revise] = useState(0);
  const formID = useId();
  const plans = useResource(() => api(path, z.array(scheduledTest)), path, open, revision);
  const results = useResource(() => api(path + "/" + resultsID + "/results", scheduledResults), path + "/" + resultsID, !!resultsID);
  const action = useAction();
  const [model, setModel] = useState("");
  const [planEnabled, setPlanEnabled] = useState(true);
  const [autoRecover, setAutoRecover] = useState(false);
  const openEditor = (plan?: z.infer<typeof scheduledTest>) => {
    action.clearError();
    setModel(plan?.model_id ?? "");
    setPlanEnabled(plan?.enabled ?? true);
    setAutoRecover(plan?.auto_recover ?? false);
    setEditing(plan ?? {id: 0, model_id: "", cron_expression: "0 */6 * * *", enabled: true, auto_recover: false, next_run_at: null});
  };
  return <>
    <InsetGroup title="定时测活" actions={<Button size="xs" variant="outline" onClick={() => setOpen(value => !value)}>{open ? "收起" : "查看计划"}</Button>}>
      {open && <Resource state={plans}>{items => <>
        <InsetRow label="计划" onClick={() => openEditor()}>新建计划</InsetRow>
        {items.map(plan => <InsetRow key={plan.id} label={plan.model_id || "默认模型"} hint={plan.cron_expression + (plan.next_run_at ? " · 下次 " + date(plan.next_run_at) : "")}>
          <InlineStack>
            <Badge variant="soft" tone={plan.enabled ? "ok" : "neutral"}>{plan.enabled ? "已启用" : "已暂停"}</Badge>
            <Button size="xs" variant="ghost" onClick={() => setResults(plan.id)}>结果</Button>
            <Button size="xs" variant="ghost" onClick={() => openEditor(plan)}>编辑</Button>
            <Button size="xs" variant="destructive" onClick={() => { action.clearError(); setDeleting(plan.id); }}>删除</Button>
          </InlineStack>
        </InsetRow>)}
      </>}</Resource>}
    </InsetGroup>
    {editing && <Overlay open title={editing.id ? "编辑定时计划" : "新建定时计划"} onClose={() => setEditing(undefined)} busy={action.busy} footer={<>
      <Button variant="outline" onClick={() => setEditing(undefined)}>取消</Button>
      <Button type="submit" form={formID} busy={action.busy}>保存计划</Button>
    </>}>
      <Form id={formID} onSubmit={event => {
        const values = new FormData(event.currentTarget);
        void action.run(async () => {
          await api(editing.id ? path + "/" + editing.id : path, scheduledTest, editing.id ? "PUT" : "POST", {
            model_id: model, cron_expression: String(values.get("cron")), enabled: planEnabled, auto_recover: autoRecover,
          });
          setEditing(undefined);
          revise(value => value + 1);
        });
      }}>
        <Field label="测活模型"><Input value={model} onChange={event => setModel(event.target.value)} maxLength={200} placeholder="留空使用默认模型" /></Field>
        <Field label="Cron 表达式"><Input name="cron" defaultValue={editing.cron_expression} required maxLength={100} /></Field>
        <InsetRow label="启用计划"><Switch label="启用计划" checked={planEnabled} onChange={setPlanEnabled} /></InsetRow>
        <InsetRow label="成功后自动恢复"><Switch label="成功后自动恢复" checked={autoRecover} onChange={setAutoRecover} /></InsetRow>
        <ErrorLine value={action.error} />
      </Form>
    </Overlay>}
    {resultsID && <Overlay open title="定时测活结果" onClose={() => setResults(undefined)}><Resource state={results}>{rows => <InsetGroup>
      {!rows.length && <InsetRow label="暂无执行记录" />}
      {rows.map(row => <InsetRow key={row.id} label={date(row.started_at)} hint={row.latency_ms + " ms"}><Status value={row.status} /></InsetRow>)}
    </InsetGroup>}</Resource></Overlay>}
    <ConfirmDialog open={!!deleting} title="删除定时计划" description="删除后此计划将停止执行。" confirm="删除" busy={action.busy} error={action.error}
      onClose={() => setDeleting(undefined)} onConfirm={() => void action.run(async () => {
        await api(path + "/" + deleting, empty, "DELETE");
        setDeleting(undefined);
        revise(value => value + 1);
      })} />
  </>;
}

const balanceSettingsSchema = z.object({enabled: z.boolean(), provider: z.string(), base_url: z.string(), configured: z.boolean()});

function AccountBalanceSettings({account, revision, onRefreshed}: {account: AccountRow; revision: number; onRefreshed: () => void}) {
  const path = "/admin/accounts/" + account.id + "/balance-settings";
  const settings = useResource(() => api(path, balanceSettingsSchema), path, true, revision);
  return <Resource state={settings}>{value => <BalanceSettingsEditor key={revision} path={path} value={value} account={account} onRefreshed={onRefreshed} />}</Resource>;
}

function BalanceSettingsEditor({value, path, account, onRefreshed}: {value: z.infer<typeof balanceSettingsSchema>; path: string; account: AccountRow; onRefreshed: () => void}) {
  const [enabled, setEnabled] = useState(value.enabled);
  const [provider, setProvider] = useState(value.provider || (balancePlatforms.has(account.platform) ? account.platform : "sub2api"));
  const [address, setAddress] = useState(value.base_url);
  const [key, setKey] = useState("");
  const action = useAction();
  return <FormSection title="定时查询上游余额">
    <Form onSubmit={() => void action.run(async () => {
      await api(path, balanceSettingsSchema, "PUT", {enabled, provider, base_url: provider === "sub2api" ? address : "", api_key: key});
      setKey("");
      toast("余额查询配置已保存");
      onRefreshed();
    })}>
      <InsetRow label="每 30 分钟查询"><Switch label="定时查询余额" checked={enabled} onChange={setEnabled} /></InsetRow>
      {enabled && <>
        <Field label="余额接口"><Select value={provider} onChange={event => setProvider(event.target.value)}>
          {balancePlatforms.has(account.platform) && <option value={account.platform}>{account.platform === "kimi" ? "Kimi" : "DeepSeek"}</option>}
          <option value="sub2api">兼容 Sub2API</option>
        </Select></Field>
        {provider === "sub2api" && <Field label="上游 HTTPS 地址"><Input value={address} onChange={event => setAddress(event.target.value)} type="url" required /></Field>}
        <Field label="余额查询密钥"><PasswordInput toggleLabel="余额查询密钥" value={key} onChange={event => setKey(event.target.value)} autoComplete="new-password"
          required={!value.configured || provider !== value.provider || provider === "sub2api" && address !== value.base_url} placeholder={value.configured ? "已配置，留空不改" : "输入已授权的上游密钥"} /></Field>
      </>}
      <Toolbar>
        <Button type="submit" size="sm" busy={action.busy}>保存查询设置</Button>
        {value.configured && <Button type="button" variant="outline" size="sm" busy={action.busy} onClick={() => void action.run(async () => {
          await api(path, balanceSettingsSchema, "POST", {});
          toast("已刷新上游余额");
          onRefreshed();
        })}>立即查询</Button>}
      </Toolbar>
      <ErrorLine value={action.error} />
    </Form>
  </FormSection>;
}

function AccountConfigForm({
  acc,
  revision,
  onSaved,
  onRequestDelete,
}: {
  acc: AccountRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(acc.name);
  const [notes, setNotes] = useState(acc.notes);
  const [proxyID, setProxyID] = useState(acc.proxy_id ? String(acc.proxy_id) : "");
  const [groupIDs, setGroupIDs] = useState<number[]>(acc.group_ids);
  const [concurrency, setConcurrency] = useState(String(acc.concurrency));
  const [priority, setPriority] = useState(String(acc.priority));
  const [multiplier, setMultiplier] = useState(String(acc.rate_multiplier));
  const [enabled, setEnabled] = useState(acc.status === "active");
  const [newApiKey, setNewApiKey] = useState("");
  const [baseURL, setBaseURL] = useState(acc.base_url ?? "");
  const [mapping, setMapping] = useState(Object.entries(acc.model_mapping ?? {}).map(([from,to]) => from + " = " + to).join("\n"));
  const mappingText = Object.entries(acc.model_mapping ?? {}).map(([from,to]) => from + " = " + to).join("\n");

  return (
    <Form
      busy={save.busy}
      onSubmit={() =>
        void save.run(async () => {
          const credentials: Record<string, unknown> = {};
        if (newApiKey.trim()) credentials.api_key = newApiKey.trim();
        if (baseURL.trim() !== (acc.base_url ?? "")) credentials.base_url = baseURL.trim();
        if (mapping !== mappingText) {
          const entries = mapping.split(/\r?\n/).map(line => line.trim()).filter(Boolean).map(line => line.split("=").map(value => value.trim()));
          if (entries.some(pair => pair.length !== 2 || !pair[0] || !pair[1]) || new Set(entries.map(pair => pair[0])).size !== entries.length) throw new Error("每行填写一个不重复的模型映射，格式为「请求模型 = 上游模型」");
          credentials.model_mapping = Object.fromEntries(entries);
        }
        await api(`/admin/accounts/${acc.id}`, account, "PUT", {
            name: name.trim(),
            notes,
            status: enabled ? "active" : "inactive",
            ...(proxyID ? {proxy_id: Number(proxyID)} : {clear_proxy: true}),
            group_ids: groupIDs,
            concurrency: Number(concurrency || 0),
            priority: Number(priority || 0),
            rate_multiplier: Number(multiplier || 1),
            ...(Object.keys(credentials).length ? {credentials} : {}),
          });
          setNewApiKey("");
          onSaved();
        })
      }
    >
      <FormSection title="基本参数与归属">
        <InsetRow label="账号启用状态">
          <Switch label="账号启用" checked={enabled} onChange={setEnabled} />
        </InsetRow>
        <Field label="账号名称">
          <Input required value={name} onChange={e => setName(e.target.value)} />
        </Field>
        <Field label="挂载代理出口">
          <ProxySelect value={proxyID} revision={revision} onChange={setProxyID} />
        </Field>
        <Field label="所属调度分组">
          <MultiGroupSelect value={groupIDs} platform={acc.platform} revision={revision} onChange={setGroupIDs} />
        </Field>
      </FormSection>

      <FormSection title="并发、权重与轮换凭据">
        <Field label="最大并发连接">
          <div className="row-field"><Input type="number" min="0" value={concurrency} onChange={e => setConcurrency(e.target.value)} /></div>
        </Field>
        <Field label="调度优先级">
          <div className="row-field"><Input type="number" value={priority} onChange={e => setPriority(e.target.value)} /></div>
        </Field>
        <Field label="账号成本倍率">
          <div className="row-field"><Input type="number" min="0" step="0.01" value={multiplier} onChange={e => setMultiplier(e.target.value)} /></div>
        </Field>
        {acc.type === "apikey" && <>
          <Field label="上游地址"><Input type="url" value={baseURL} onChange={event => setBaseURL(event.target.value)} /></Field>
          <Field label="模型映射" hint="每行一个，请求模型 = 上游模型"><Textarea rows={4} value={mapping} onChange={event => setMapping(event.target.value)} /></Field>
          <Field label="更新上游 API Key（留空不改）">
            <PasswordInput toggleLabel="API Key" autoComplete="new-password" value={newApiKey} onChange={e => setNewApiKey(e.target.value)} placeholder="已加密保存，仅在需要轮换时输入" />
          </Field>
        </>}
        <Field label="运维备注">
          <Input value={notes} onChange={e => setNotes(e.target.value)} placeholder="选填备注" />
        </Field>
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除账号</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存账号配置</Button>
      </FormActions>
    </Form>
  );
}

function AddAccountDialog({
  open,
  revision,
  defaultGroupID,
  defaultProxyID,
  reauthAccount,
  onClose,
  onCreated,
  onInlineResourceCreated,
}: {
  open: boolean;
  revision: number;
  defaultGroupID?: number;
  defaultProxyID?: number;
  reauthAccount?: AccountRow;
  onClose: () => void;
  onCreated: (id?: number) => void;
  onInlineResourceCreated: () => void;
}) {
  const formID = useId();
  const [platform, setPlatform] = useState(reauthAccount?.platform || "openai");
  const [mode, setMode] = useState<"oauth" | "apikey" | "import">(reauthAccount ? "oauth" : "oauth");
  const [name, setName] = useState(reauthAccount?.name || "");
  const [proxyID, setProxyID] = useState(reauthAccount?.proxy_id ? String(reauthAccount.proxy_id) : defaultProxyID ? String(defaultProxyID) : "");
  const [groupIDs, setGroupIDs] = useState<number[]>(reauthAccount?.group_ids ?? (defaultGroupID ? [defaultGroupID] : []));
  const [apiKeyVal, setApiKeyVal] = useState("");
  const [baseUrlVal, setBaseUrlVal] = useState("");
  const [oauthSession, setOauthSession] = useState<{auth_url: string; session_id: string; state: string}>();
  const [oauthCode, setOauthCode] = useState("");
  const [claudeMode, setClaudeMode] = useState<"oauth" | "setup-token">("oauth");
  const [importKind, setImportKind] = useState<"codex-session" | "codex-pat" | "grok-sso">("codex-session");
  const [importLines, setImportLines] = useState("");
  const [inlineRev, setInlineRev] = useState(0);

  const [quickProxyHost, setQuickProxyHost] = useState("");
  const [quickProxyPort, setQuickProxyPort] = useState("8080");
  const [quickGroupName, setQuickGroupName] = useState("");

  const op = useAction();
  const inlineOp = useAction();

  const oauthSupported = ["openai", "anthropic", "gemini", "antigravity", "grok"].includes(platform);

  useEffect(() => {
    if (!oauthSupported && mode === "oauth") setMode("apikey");
  }, [mode, oauthSupported]);

  return (
    <Overlay
      open={open}
      title={reauthAccount ? `重新授权 · ${reauthAccount.name}` : "添加或导入上游账号"}
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" disabled={op.busy} onClick={onClose}>取消</Button>
          {mode === "oauth" && oauthSession && (
            <Button type="submit" form={formID} busy={op.busy} disabled={!oauthCode.trim() || (!reauthAccount && !name.trim())}>
              完成服务端授权
            </Button>
          )}
          {mode === "apikey" && !reauthAccount && (
            <Button type="submit" form={formID} busy={op.busy} disabled={!name.trim() || !apiKeyVal.trim()}>
              保存账号
            </Button>
          )}
          {mode === "import" && !reauthAccount && (
            <Button type="submit" form={formID} busy={op.busy}>
              开始批量导入
            </Button>
          )}
        </>
      }
    >
      {!reauthAccount && (
        <ChoiceCards
          label="选择上游平台"
          value={platform}
          options={[
            {value: "openai", title: "OpenAI", description: "Codex OAuth / Key / PAT"},
            {value: "anthropic", title: "Anthropic", description: "Claude OAuth / Key"},
            {value: "gemini", title: "Gemini", description: "Google OAuth / Key"},
            {value: "antigravity", title: "Antigravity", description: "Companion OAuth"},
            {value: "grok", title: "Grok", description: "xAI OAuth / SSO"},
            {value: "deepseek", title: "DeepSeek", description: "API Key 直连"},
          ]}
          onChange={val => {
            if (val !== platform) setGroupIDs([]);
            setPlatform(val);
            setOauthSession(undefined);
          }}
        />
      )}

      {!reauthAccount && (
        <Segmented
          label="接入方式"
          value={mode}
          options={[
            ...(oauthSupported ? ([["oauth", "OAuth 服务端闭环授权"]] as const) : []),
            ["apikey", "API Key 凭据"],
            ["import", "Session / PAT / SSO 批量导入"],
          ]}
          onChange={v => setMode(v as "oauth" | "apikey" | "import")}
        />
      )}

      {!reauthAccount && (
        <FormSection title="分组与代理">
          {mode !== "import" && (
            <Field label="账号名称">
              <Input value={name} onChange={e => setName(e.target.value)} placeholder={`例如 ${platform}-prod-01`} />
            </Field>
          )}
          <Field label="代理节点">
            <ProxySelect value={proxyID} revision={revision + inlineRev} onChange={setProxyID} />
          </Field>
          <Field label="挂载调度分组">
            <MultiGroupSelect value={groupIDs} platform={platform} revision={revision + inlineRev} onChange={setGroupIDs} />
          </Field>
        </FormSection>
      )}

      {!reauthAccount && (
        <CollapsibleSection title="新建代理或分组">
          <Stack>
            <Toolbar>
              <Field label="代理主机"><Input value={quickProxyHost} onChange={e => setQuickProxyHost(e.target.value)} placeholder="127.0.0.1" /></Field>
              <Field label="端口"><Input type="number" value={quickProxyPort} onChange={e => setQuickProxyPort(e.target.value)} /></Field>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!quickProxyHost.trim()}
                busy={inlineOp.busy}
                onClick={() =>
                  void inlineOp.run(async () => {
                    const px = await api("/admin/proxies", proxyRow, "POST", {
                      name: `${quickProxyHost.trim()}:${quickProxyPort}`,
                      protocol: "http",
                      host: quickProxyHost.trim(),
                      port: Number(quickProxyPort || 8080),
                    });
                    setProxyID(String(px.id));
                    setQuickProxyHost("");
                    setInlineRev(v => v + 1);
                    onInlineResourceCreated();
                  })
                }
              >
                新建并选用代理
              </Button>
            </Toolbar>
            <Toolbar>
              <Field label="分组名称"><Input value={quickGroupName} onChange={e => setQuickGroupName(e.target.value)} placeholder={`新 ${platform} 分组名称`} /></Field>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!quickGroupName.trim()}
                busy={inlineOp.busy}
                onClick={() =>
                  void inlineOp.run(async () => {
                    const grp = await api("/admin/groups", adminGroupRow, "POST", {
                      name: quickGroupName.trim(),
                      platform,
                      rate_multiplier: 1,
                    });
                    setGroupIDs(prev => [...new Set([...prev, grp.id])]);
                    setQuickGroupName("");
                    setInlineRev(v => v + 1);
                    onInlineResourceCreated();
                  })
                }
              >
                新建并勾选分组
              </Button>
            </Toolbar>
          </Stack>
          <ErrorLine value={inlineOp.error} />
        </CollapsibleSection>
      )}

      {mode === "oauth" && (
        <Form
          id={formID}
          busy={op.busy}
          onSubmit={() => {
            if (!oauthSession || !oauthCode.trim() || (!reauthAccount && !name.trim())) return;
            void op.run(async () => {
              const created = await api("/admin/accounts/oauth/complete", account, "POST", {
                platform,
                mode: platform === "anthropic" ? claudeMode : undefined,
                session_id: oauthSession.session_id,
                state: oauthSession.state,
                code: oauthCode.trim(),
                ...(reauthAccount ? {account_id: reauthAccount.id} : {name: name.trim(), group_ids: groupIDs}),
                ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
              });
              onCreated(created.id);
            });
          }}
        >
          <FormSection title="服务端闭环 OAuth 授权" description="授权码在面板 Go 服务端内部换取 Token 并直接写入上游，浏览器全程不接触上游令牌。">
            {platform === "anthropic" && (
              <Field label="Claude 授权方式">
                <Segmented
                  label="Claude 授权方式"
                  value={claudeMode}
                  options={[
                    ["oauth", "标准 OAuth"],
                    ["setup-token", "Setup Token"],
                  ] as const}
                  onChange={setClaudeMode}
                />
              </Field>
            )}
            <InsetRow label="第 1 步：生成官方授权链接">
              <Button
                type="button"
                size="sm"
                variant="outline"
                busy={op.busy}
                onClick={() =>
                  void op.run(async () => {
                    const res = await api(
                      "/admin/accounts/oauth/start",
                      z.object({auth_url: z.string(), session_id: z.string(), state: z.string()}),
                      "POST",
                      {
                        platform,
                        mode: platform === "anthropic" ? claudeMode : undefined,
                        ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
                      },
                    );
                    setOauthSession(res);
                    window.open(res.auth_url, "_blank", "noopener,noreferrer");
                  })
                }
              >
                获取并打开授权链接
              </Button>
            </InsetRow>
            {oauthSession && (
              <Field label="第 2 步：粘贴回调 Code 或回调 URL 参数">
                <Input name="code" value={oauthCode} onChange={e => setOauthCode(e.target.value)} placeholder="粘贴授权完成后的 code" required />
              </Field>
            )}
          </FormSection>
          <ErrorLine value={op.error} />
        </Form>
      )}

      {mode === "apikey" && !reauthAccount && (
        <Form
          id={formID}
          busy={op.busy}
          onSubmit={() =>
            void op.run(async () => {
              const created = await api("/admin/accounts", account, "POST", {
                name: name.trim(),
                platform,
                type: "apikey",
                credentials: {
                  api_key: apiKeyVal.trim(),
                  ...(baseUrlVal.trim() ? {base_url: baseUrlVal.trim()} : {}),
                },
                ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
                group_ids: groupIDs,
              });
              onCreated(created.id);
            })
          }
        >
          <FormSection title="API Key 凭据录入">
            <Field label="上游 API Key">
              <PasswordInput required toggleLabel="API Key" autoComplete="new-password" value={apiKeyVal} onChange={e => setApiKeyVal(e.target.value)} placeholder="sk-..." />
            </Field>
            <Field label="自定义 Base URL（可选）">
              <Input value={baseUrlVal} onChange={e => setBaseUrlVal(e.target.value)} placeholder="留空使用平台官方网关" />
            </Field>
          </FormSection>
          <ErrorLine value={op.error} />
        </Form>
      )}

      {mode === "import" && !reauthAccount && (
        <Form
          id={formID}
          busy={op.busy}
          onSubmit={() =>
            void op.run(async () => {
              const items = importLines
                .split(/\r?\n/)
                .map(s => s.trim())
                .filter(Boolean);
              await api(
                "/admin/accounts/import",
                z.object({created: z.number(), updated: z.number(), skipped: z.number(), failed: z.number()}),
                "POST",
                {
                  kind: importKind,
                  name: name.trim() || undefined,
                  items,
                  ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
                  group_ids: groupIDs,
                  update_existing: true,
                },
              );
              onCreated();
            })
          }
        >
          <FormSection title="批量导入上游会话凭据">
            <Field label="导入格式">
              <Select value={importKind} onChange={e => setImportKind(e.target.value as "codex-session" | "codex-pat" | "grok-sso")}>
                <option value="codex-session">OpenAI Codex Session JSON</option>
                <option value="codex-pat">OpenAI Codex Access Token (PAT)</option>
                <option value="grok-sso">Grok SSO Cookie 批量转换</option>
              </Select>
            </Field>
            <Field label="凭据内容（每行一条）">
              <Textarea rows={5} required value={importLines} onChange={e => setImportLines(e.target.value)} placeholder="每行粘贴一条 Session JSON / Token / SSO Cookie" />
            </Field>
          </FormSection>
          <ErrorLine value={op.error} />
        </Form>
      )}
    </Overlay>
  );
}

function BulkBindAccountsDialog({
  open,
  ids,
  revision,
  onClose,
  onSaved,
}: {
  open: boolean;
  ids: readonly number[];
  revision: number;
  onClose: () => void;
  onSaved: () => void;
}) {
  const formID = useId();
  const [applyGroups, setApplyGroups] = useState(true);
  const [groupIDs, setGroupIDs] = useState<number[]>([]);
  const [applyProxy, setApplyProxy] = useState(false);
  const [proxyID, setProxyID] = useState("");
  const op = useAction();

  return (
    <Overlay
      open={open}
      title={`批量调整 ${ids.length} 个账号归属`}
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} disabled={!applyGroups && !applyProxy} busy={op.busy}>执行批量更新</Button>
        </>
      }
    >
      <Form
        id={formID}
        busy={op.busy}
        onSubmit={() =>
          void op.run(async () => {
            await api("/admin/accounts/bulk-update", z.object({success: z.number(), failed: z.number()}), "POST", {
              account_ids: ids,
              ...(applyGroups ? {group_ids: groupIDs} : {}),
              ...(applyProxy ? (proxyID ? {proxy_id: Number(proxyID)} : {clear_proxy: true}) : {}),
            });
            onSaved();
          })
        }
      >
        <FormSection title="批量覆写范围">
          <InsetRow label="同步修改所属调度分组"><Switch label="修改所属分组" checked={applyGroups} onChange={setApplyGroups} /></InsetRow>
          {applyGroups && <Field label="目标分组列表"><MultiGroupSelect value={groupIDs} revision={revision} onChange={setGroupIDs} /></Field>}
          <InsetRow label="同步切换代理节点"><Switch label="修改代理节点" checked={applyProxy} onChange={setApplyProxy} /></InsetRow>
          {applyProxy && <Field label="目标代理（清空即改回直连）"><ProxySelect value={proxyID} revision={revision} onChange={setProxyID} /></Field>}
        </FormSection>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
