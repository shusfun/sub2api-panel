import {type ReactNode} from "react";
import {compactTokens, count, currencyMoney, date, smartMoney} from "@/lib/format";
import {
  Badge,
  EllipsisText,
  EntityCard,
  EntityLink,
  EntityMeters,
  EntityMetrics,
  MeterBar,
  MiniPills,
  PlanSummary,
  PlatformBadge,
  Status,
  StatusDot,
  Tooltip,
  type EntityCardResult,
} from "@/ui";
import {ActivityWire} from "@/ui/primitives/activity-motion";
import {type AccountRow} from "@/features/pool/shared";
import {
  accountHold,
  accountProblem,
  balanceHint,
  buildQuotaMeters,
  isQuotaCapable,
  type BalanceInfo,
  type CardMode,
  type QuotaDisplay,
  type QuotaInfo,
  type UsageWindow,
  type WindowStat,
} from "./account-helpers";

function runwayText(hours: number | null | undefined) {
  if (hours == null || !Number.isFinite(hours)) return "";
  if (hours < 48) return `按近期速度约可撑 ${Math.max(1, Math.round(hours))} 小时`;
  return `按近期速度约可撑 ${Math.round(hours / 24)} 天`;
}

function signedPercent(value: number) {
  return (value > 0 ? "+" : "") + value.toFixed(0) + "%";
}

export function AccountCardItem({
  row,
  quota,
  balance,
  mode,
  win,
  quotaDisplay,
  cachedWin,
  statBusy,
  selected,
  active,
  groups,
  result,
  onSelect,
  onOpen,
  onModeChange,
  onWindowChange,
  scheduleSwitch,
  recoveryAction,
  actions,
  more,
}: {
  row: AccountRow;
  quota?: QuotaInfo;
  balance?: BalanceInfo;
  mode: CardMode;
  win: UsageWindow;
  quotaDisplay: QuotaDisplay;
  cachedWin?: WindowStat;
  statBusy: boolean;
  selected: boolean;
  active: boolean;
  groups: {id: number; name: string}[];
  result?: EntityCardResult;
  onSelect: (next: boolean) => void;
  onOpen: () => void;
  onModeChange: (next: CardMode) => void;
  onWindowChange: (next: UsageWindow) => void;
  scheduleSwitch: ReactNode;
  recoveryAction: ReactNode;
  actions: ReactNode;
  more: ReactNode;
}) {
  const ledger = row.ledger;
  const hasQuota = isQuotaCapable(row, quota);
  const problem = accountProblem(row);
  const hold = accountHold(row);
  // 有余额的账号：账本给出美元余额，或旧的余额探测返回了账户 / 密钥余额。
  const walletBalance = ledger?.balance_usd ?? (balance && balance.kind !== "subscription" ? balance.balance : null);
  const walletCurrency = ledger?.balance_usd != null ? "USD" : balance?.currency ?? "USD";
  const hasWallet = !hasQuota && walletBalance != null;
  const dailyQuota = row.extra?.daily_quota ?? null;
  const windowCostLimit = row.extra?.window_cost_limit ?? null;
  const stale = Boolean(ledger?.stale || balance?.stale);

  const banner = (
    <>
      <EllipsisText text={problem?.label ?? (row.schedulable ? "可调度" : "已暂停调度")} className="entity-card-banner-label" />
      {problem && recoveryAction}
    </>
  );

  const requests = statBusy ? "…" : cachedWin ? count.format(cachedWin.requests) : "未知";
  const errors = win === "1" && (row.today_errors ?? 0) > 0 ? ` · 错 ${row.today_errors}` : "";

  // 主区域一：有余额的账号，大字余额 + 可撑时长 + 倍率变化。
  const walletBody = hasWallet ? (
    <div className="acct-wallet">
      <strong className="acct-wallet-value" data-stale={stale || undefined}>{currencyMoney(walletBalance, walletCurrency)}</strong>
      <span className="acct-wallet-sub">{runwayText(ledger?.runway_hours) || (balance ? "余额" : "暂无消耗速度数据")}</span>
      {ledger?.multiplier != null && (
        <span className="acct-multiplier" data-drift={ledger.local_multiplier != null && ledger.local_multiplier !== ledger.multiplier || undefined}>
          上游 ×{ledger.multiplier}
          {ledger.local_multiplier != null && ledger.local_multiplier !== ledger.multiplier && ` · 本地 ×${ledger.local_multiplier}`}
          {ledger.previous_multiplier != null && ledger.previous_multiplier !== ledger.multiplier &&
            ` · ${ledger.multiplier > ledger.previous_multiplier ? "↑" : "↓"} 此前 ×${ledger.previous_multiplier}`}
        </span>
      )}
    </div>
  ) : null;

  // 主区域二：OAuth / Setup Token，配额条与用量切换。
  const quotaBody = hasQuota ? (
    mode === "quota" ? (
      <>
        <EntityMeters items={buildQuotaMeters(row, quota, quotaDisplay)} />
        {windowCostLimit != null && windowCostLimit > 0 && (
          <MeterBar
            size="sm"
            value={Math.min(100, Math.round(((row.today_cost ?? 0) / windowCostLimit) * 100))}
            label={`窗口限额 ${smartMoney(row.today_cost ?? 0)}/${smartMoney(windowCostLimit)}`}
          />
        )}
      </>
    ) : (
      <EntityMetrics
        items={[
          {label: win === "1" ? "今日请求" : `近 ${win} 天请求`, value: requests + errors},
          {label: "Token", value: statBusy ? "…" : cachedWin ? compactTokens(cachedWin.tokens) : "未知"},
          {label: "实际消耗", value: statBusy ? "…" : cachedWin ? smartMoney(cachedWin.cost) : "未知"},
        ]}
      />
    )
  ) : null;

  // 主区域三：拿不到余额的官方 Key，今日已用 / 额度上限。
  const spent = row.today_cost ?? 0;
  const keyBody = !hasWallet && !hasQuota ? (
    <div className="acct-wallet">
      <strong className="acct-wallet-value">{smartMoney(spent)}</strong>
      <span className="acct-wallet-sub">
        {dailyQuota != null && dailyQuota > 0 ? `今日已用 / 额度上限 ${smartMoney(dailyQuota)}` : "今日消耗，未设上限"}
      </span>
      {dailyQuota != null && dailyQuota > 0 && (
        <MeterBar size="sm" value={Math.min(100, Math.round((spent / dailyQuota) * 100))} label={`${Math.min(100, Math.round((spent / dailyQuota) * 100))}%`} />
      )}
    </div>
  ) : null;

  // 对账行：仅美元记账且开启对账的上游。
  const reconcileRow = ledger?.reconcile ? (
    <div className="acct-reconcile" data-estimated={ledger.estimated || undefined}>
      <span>
        今日实际 {ledger.actual_today_usd != null ? smartMoney(ledger.actual_today_usd) : "未知"} / 预计 {smartMoney(ledger.expected_today_usd)}
        {ledger.estimated && " · 估算"}
      </span>
      {ledger.deviation_pct != null && (
        <Badge variant="soft" tone={Math.abs(ledger.deviation_pct) >= 10 ? "bad" : "neutral"}>偏差 {signedPercent(ledger.deviation_pct)}</Badge>
      )}
      {ledger.revenue_today_usd != null && ledger.profit_today_usd != null && (
        <span className="acct-profit">收入 {smartMoney(ledger.revenue_today_usd)} · 毛利 {smartMoney(ledger.profit_today_usd)}</span>
      )}
    </div>
  ) : null;

  const panelHead = hasQuota ? (
    <>
      <MiniPills
        label={`${row.name} 展示维度`}
        value={mode}
        options={[["quota", "配额"], ["usage", "用量"]] as const}
        onChange={onModeChange}
      />
      {mode === "quota" ? (
        <span>{quota?.subscription_tier ? quota.subscription_tier : quotaDisplay === "remaining" ? "剩余" : "已用"}</span>
      ) : (
        <MiniPills
          label={`${row.name} 用量周期`}
          value={win}
          options={[["1", "今日"], ["7", "7d"], ["30", "30d"]] as const}
          onChange={onWindowChange}
        />
      )}
    </>
  ) : (
    <>
      <span>{hasWallet ? "余额" : "今日用量"}</span>
      {balance && <EllipsisText text={balance.kind === "key_quota" ? "密钥额度" : "账户余额"} tip={balanceHint(balance)} />}
    </>
  );

  const panelBody = (
    <>
      {walletBody}
      {quotaBody}
      {keyBody}
      {reconcileRow}
    </>
  );

  const rpm = row.rpm_5m;
  const rpmLimit = row.extra?.rpm_limit;
  const metrics = [
    {label: win === "1" ? "今日请求" : `近 ${win} 天请求`, value: requests + errors},
    {label: "并发", value: row.activity_known === false ? "未知" : `${row.current_concurrency}/${row.concurrency || "∞"}`},
    ...(rpm !== undefined ? [{label: "RPM", value: `${Math.round(rpm)}/${rpmLimit ? rpmLimit : "∞"}`}] : []),
  ];

  const sampled = ledger?.sampled_at ?? (balance?.fetched_at ? new Date(balance.fetched_at * 1000).toISOString() : null);
  const metaItems = [
    ...(row.avg_latency_ms !== undefined ? [`延迟 ${Math.round(row.avg_latency_ms)} ms`] : []),
    ...(sampled ? [<span key="sampled" data-stale={stale || undefined}>余额更新 {date(sampled)}</span>] : []),
    `优先级 ${row.priority}`,
    <PlanSummary key="groups" label="分组" empty="未绑组" items={groups} />,
    row.proxy_id ? <EntityLink key="proxy" kind="proxy" id={row.proxy_id} label={row.proxy_name || `代理 #${row.proxy_id}`} /> : "直连",
  ];

  return (
    <div role="listitem" data-motion-key={row.id}>
      <EntityCard
        name={row.name}
        platform={row.platform}
        hold={problem ? (problem.tone === "bad" ? "bad" : "warn") : undefined}
        lead={<StatusDot tone={row.status === "active" ? (hold ? "warn" : "ok") : row.status === "error" ? "bad" : "neutral"} />}
        toggle={<>{row.activity_known !== false && row.current_concurrency > 0 && <ActivityWire />}{scheduleSwitch}</>}
        selected={selected}
        onSelect={onSelect}
        active={active}
        dim={!row.schedulable}
        onOpen={onOpen}
        badges={
          <>
            <PlatformBadge platform={row.platform} type={row.type} />
            {row.status !== "active" && <Status value={row.status} />}
            {(row.last_error_code || row.last_error_kind) && (
              <Badge variant="soft" tone="bad">{row.last_error_code || row.last_error_kind}</Badge>
            )}
            {ledger?.alerts.map(alert => (
              <Badge key={alert.kind} variant="soft" tone={alert.severity === "bad" ? "bad" : alert.severity === "warn" ? "warn" : "neutral"}>{alert.label}</Badge>
            ))}
            {balance && balance.available === false && <Badge variant="soft" tone="bad">余额不可用</Badge>}
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
        metrics={metrics}
        meta={metaItems}
        result={result}
        reserveResult
        actions={actions}
        more={more}
      />
    </div>
  );
}
