import {useEffect, useRef, useState} from "react";
import {type z} from "zod";
import {api} from "@/lib/http";
import {accountBalance, accountQuota, accountQuotaBatch, accountStatsSummary} from "@/schemas";
import {currencyMoney, date, relativeUntil} from "@/lib/format";
import {MeterBar, type ViewMode} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";

export type AccountSort = "priority" | "today_cost" | "five_hours" | "week" | "last_used" | "activity";
export type QuotaInfo = z.infer<typeof accountQuota>;
export type BalanceInfo = z.infer<typeof accountBalance>;
export type CardMode = "quota" | "usage";
export type UsageWindow = "1" | "7" | "30";
export type QuotaDisplay = "used" | "remaining";
export type WindowStat = {requests: number; tokens: number; cost: number};

export const sortOptions = [
  {value: "priority", label: "按优先级"},
  {value: "today_cost", label: "今日消耗高到低"},
  {value: "five_hours", label: "5h 水位高到低"},
  {value: "week", label: "7d 水位高到低"},
  {value: "last_used", label: "最近使用"},
  {value: "activity", label: "按活动排序"},
] as const;

export const viewKey = "panel-accounts-view-grid";
export const quotaDisplayKey = "panel-accounts-quota-mode";
export const balancePlatforms = new Set(["deepseek", "kimi"]);

export function balanceText(balance: BalanceInfo) {
  const label = balance.kind === "key_quota" ? "密钥额度" : balance.kind === "subscription" ? "订阅额度" : "余额";
  return label + " " + currencyMoney(balance.balance, balance.currency) + (balance.stale ? " · 待更新" : "");
}

export function balanceHint(balance: BalanceInfo) {
  const sampled = balance.fetched_at ? "余额更新于 " + date(balance.fetched_at * 1000) : "";
  const billing = balance.billing;
  if (!billing) return sampled || undefined;
  const detail = "上游 Token 计费倍率 ×" + billing.effective_multiplier + " · " + date(billing.observed_at);
  return [sampled, detail + (balance.stale || billing.stale ? " · 待更新" : "")].filter(Boolean).join(" / ");
}

export function sortAccounts(rows: AccountRow[], sort: AccountSort) {
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

export function concurrencyLabel(row: AccountRow) {
  return row.activity_known === false ? "活动未知" : String(row.current_concurrency) + "/" + (row.concurrency || "∞");
}

// 账号当前不能正常调度的原因与恢复时间；正常时返回 undefined。
export function accountProblem(row: AccountRow) {
  if (row.expires_at != null && row.expires_at * 1000 <= Date.now()) return {label: "已到期", tone: "bad", action: "edit"} as const;
  if (row.type === "apikey" && row.credentials_status.has_api_key === false) return {label: "缺少上游密钥", tone: "bad", action: "edit"} as const;
  if (row.proxy_fallback_origin_id) return {label: "代理已回退", tone: "warn", action: "revert-proxy-fallback"} as const;
  if (row.status === "error") return {label: "运行异常", tone: "bad", action: "recover"} as const;
  if (row.rate_limit_reset_at) return {label: "限流 " + relativeUntil(row.rate_limit_reset_at), tone: "warn", action: "clear-rate-limit"} as const;
  if (row.overload_until) return {label: "过载 " + relativeUntil(row.overload_until), tone: "warn", action: "recover"} as const;
  if (row.temp_unschedulable_until) return {label: "冷却 " + relativeUntil(row.temp_unschedulable_until), tone: "warn", action: "temp-unschedulable"} as const;
}

export function accountHold(row: AccountRow) {
  return accountProblem(row)?.label;
}

export function recoveryLabel(row: AccountRow) {
  const problem = accountProblem(row);
  return problem?.action === "edit" ? "查看详情" : problem?.action === "recover" ? "恢复" : "解除异常";
}

export function isQuotaCapable(row: AccountRow, quota?: QuotaInfo) {
  return (
    row.type === "oauth" ||
    row.type === "setup-token" ||
    row.five_hours_used !== null ||
    row.week_used !== null ||
    Boolean(quota?.five_hours || quota?.week || quota?.gemini_daily || quota?.sonnet_week)
  );
}

export function canRefreshBalance(row: AccountRow) {
  return row.type === "apikey" && balancePlatforms.has(row.platform);
}

export function buildQuotaMeters(row: AccountRow, quota: QuotaInfo | undefined, display: QuotaDisplay) {
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

export function useAccountTelemetry({
  rows,
  revision,
  view,
  defaultWindow,
  cardModes,
  cardWindows,
}: {
  rows: AccountRow[];
  revision: number;
  view: ViewMode;
  defaultWindow: UsageWindow;
  cardModes: Record<number, CardMode>;
  cardWindows: Record<number, UsageWindow>;
}) {
  const [quotaMap, setQuotaMap] = useState<Record<number, QuotaInfo>>({});
  const [balanceMap, setBalanceMap] = useState<Record<number, BalanceInfo>>({});
  const [statsCache, setStatsCache] = useState<Record<string, WindowStat>>({});
  const [loadingStats, setLoadingStats] = useState<Record<string, boolean>>({});
  const statsInFlight = useRef(new Set<string>());

  function clearStats() {
    statsInFlight.current.clear();
    setStatsCache({});
  }

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

  async function refreshRowTelemetry(row: AccountRow) {
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
  }

  return {quotaMap, balanceMap, statsCache, loadingStats, clearStats, ensureWindowStats, refreshRowTelemetry};
}
