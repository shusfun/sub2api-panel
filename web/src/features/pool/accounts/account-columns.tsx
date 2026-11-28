import {type ReactNode} from "react";
import {count, currencyMoney, percent, smartMoney} from "@/lib/format";
import {
  Badge,
  EntityLink,
  EntityResult,
  InlineStack,
  PlanSummary,
  PlatformBadge,
  RowActions,
  SecondaryTableAction,
  Status,
  type Column,
  type EntityCardResult,
} from "@/ui";
import {ActivityWire} from "@/ui/primitives/activity-motion";
import {type AccountRow} from "@/features/pool/shared";
import {
  accountProblem,
  balanceHint,
  balanceText,
  concurrencyLabel,
  type BalanceInfo,
} from "./account-helpers";

function runwayShort(hours: number | null | undefined) {
  if (hours == null || !Number.isFinite(hours)) return "";
  if (hours < 48) return `可撑 ${Math.max(1, Math.round(hours))} 小时`;
  return `可撑 ${Math.round(hours / 24)} 天`;
}

export function buildAccountColumns({
  inspectorOpen = false,
  results,
  balanceMap,
  groupsOf,
  accountRecovery,
  scheduleSwitch,
  rowActions,
  rowMore,
}: {
  inspectorOpen?: boolean;
  results: Record<number, EntityCardResult>;
  balanceMap: Record<number, BalanceInfo>;
  groupsOf: (row: AccountRow) => {id: number; name: string}[];
  accountRecovery: (row: AccountRow) => ReactNode;
  scheduleSwitch: (row: AccountRow) => ReactNode;
  rowActions: (row: AccountRow, card?: boolean) => ReactNode;
  rowMore: (row: AccountRow, card?: boolean) => ReactNode;
}): Column<AccountRow>[] {
  const columns: Column<AccountRow>[] = [
    {
      id: "name",
      title: "上游账号",
      render: row => (
        <div>
          <strong className="compact-text" title={row.name}><span className="acct-plat-dot" data-platform={row.platform} aria-hidden />{row.name}</strong>
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
      id: "balance",
      title: "余额 / 可撑",
      hideWhenNarrow: true,
      align: "number",
      render: row => {
        const ledger = row.ledger;
        const bal = row.balance?.kind ? row.balance : balanceMap[row.id] ?? row.balance;
        const value = ledger?.balance_usd ?? (bal && bal.kind !== "subscription" ? bal.balance : null);
        if (value == null) return <span className="muted">—</span>;
        const currency = ledger?.balance_usd != null ? "USD" : bal?.currency ?? "USD";
        const runway = runwayShort(ledger?.runway_hours);
        return (
          <span className="weir-digits" data-stale={ledger?.stale || bal?.stale || undefined}>
            {currencyMoney(value, currency)}{runway && <span className="muted"> · {runway}</span>}
          </span>
        );
      },
    },
    {
      id: "reconcile",
      title: "今日成本 / 偏差",
      hideWhenNarrow: true,
      align: "number",
      render: row => {
        const ledger = row.ledger;
        if (!ledger?.reconcile) return <span className="muted">—</span>;
        const actual = ledger.actual_today_usd;
        const dev = ledger.deviation_pct;
        return (
          <span className="weir-digits" data-estimated={ledger.estimated || undefined}>
            {actual != null ? smartMoney(actual) : "未知"}
            {dev != null && (
              <Badge variant="soft" tone={Math.abs(dev) >= 10 ? "bad" : "neutral"}>{(dev > 0 ? "+" : "") + dev.toFixed(0)}%</Badge>
            )}
          </span>
        );
      },
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
        return (
          <div className="account-status-cell" onClick={event => event.stopPropagation()}>
            {problem ? (
              <>
                <Badge variant="soft" tone={problem.tone}>{problem.label}</Badge>
                <SecondaryTableAction>{accountRecovery(row)}</SecondaryTableAction>
              </>
            ) : (
              <Status value={row.status} />
            )}
            {row.activity_known !== false && row.current_concurrency > 0 && <ActivityWire />}
          </div>
        );
      },
    },
    {id: "sched", title: "调度", hideWhenNarrow: true, render: row => <span onClick={event => event.stopPropagation()}>{scheduleSwitch(row)}</span>},
    {id: "ops", title: "", align: "end", render: row => <RowActions>{rowActions(row)}{rowMore(row)}</RowActions>},
  ];
  // 检查器打开时表格变窄，收起次要列。
  return inspectorOpen ? columns.filter(column => column.id !== "relations" && column.id !== "conc") : columns;
}
