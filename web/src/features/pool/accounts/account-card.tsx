import {type ReactNode} from "react";
import {compactTokens, count, date, smartMoney} from "@/lib/format";
import {
  Badge,
  EllipsisText,
  EntityCard,
  EntityLink,
  EntityMeters,
  EntityMetrics,
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
  balanceText,
  buildQuotaMeters,
  isQuotaCapable,
  type BalanceInfo,
  type CardMode,
  type QuotaDisplay,
  type QuotaInfo,
  type UsageWindow,
  type WindowStat,
} from "./account-helpers";

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
  const hasQuota = isQuotaCapable(row, quota);
  const hold = accountHold(row);
  const problem = accountProblem(row);

  const banner = (
    <>
      <EllipsisText text={problem?.label ?? (row.schedulable ? "可调度" : "已暂停调度")} className="entity-card-banner-label" />
      {problem && recoveryAction}
    </>
  );

  const modeSwitch = hasQuota ? (
    <MiniPills
      label={`${row.name} 展示维度`}
      value={mode}
      options={[
        ["quota", "配额"],
        ["usage", "用量"],
      ] as const}
      onChange={onModeChange}
    />
  ) : (
    <EllipsisText text={balance ? `调用用量 · ${balanceText(balance)}` : "调用用量"} tip={balance ? balanceHint(balance) : undefined} />
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
          onChange={onWindowChange}
        />
      </>
    );

  const requests = statBusy ? "…" : cachedWin ? count.format(cachedWin.requests) : "未知";
  const errors = win === "1" && (row.today_errors ?? 0) > 0 ? " · 错 " + row.today_errors : "";
  const panelBody =
    mode === "quota" ? (
      <EntityMeters items={buildQuotaMeters(row, quota, quotaDisplay)} />
    ) : (
      <EntityMetrics
        items={[
          {label: win === "1" ? "今日请求" : "近 " + win + " 天请求", value: requests + errors},
          {label: "Token", value: statBusy ? "…" : cachedWin ? compactTokens(cachedWin.tokens) : "未知"},
          {label: "实际消耗", value: statBusy ? "…" : cachedWin ? smartMoney(cachedWin.cost) : "未知"},
        ]}
      />
    );

  const metaItems = [
    `优先级 ${row.priority}`,
    ...(row.rate_multiplier !== 1 ? [`${row.rate_multiplier}×`] : []),
    `并发 ${row.current_concurrency}/${row.concurrency || "∞"}`,
    <PlanSummary key="groups" label="分组" empty="未绑组" items={groups} />,
    row.proxy_id ? <EntityLink key="proxy" kind="proxy" id={row.proxy_id} label={row.proxy_name || `代理 #${row.proxy_id}`} /> : "直连",
    ...(row.last_used_at ? [`最近 ${date(row.last_used_at)}`] : []),
  ];

  return (
    <div role="listitem" data-motion-key={row.id}>
      <EntityCard
        name={row.name}
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
            {balance && hasQuota && (
              <Tooltip content={balanceHint(balance)} side="top">
                <Badge variant="tag">{balanceText(balance)}</Badge>
              </Tooltip>
            )}
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
        meta={metaItems}
        result={result}
        reserveResult
        actions={actions}
        more={more}
      />
    </div>
  );
}
