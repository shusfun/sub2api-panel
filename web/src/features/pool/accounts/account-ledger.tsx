import {useState} from "react";
import {api} from "@/lib/http";
import {ledgerAccountSeries, type LedgerAccountSeries} from "@/schemas/ledger";
import {useResource} from "@/lib/requests";
import {date, smartMoney} from "@/lib/format";
import {Badge, HeroMetricChart, InsetGroup, InsetRow, Resource, Segmented} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";
import {AccountLedgerConnect} from "./account-ledger-connect";
import {AccountLedgerEvents} from "./account-ledger-events";

type Granularity = "hour" | "day";

function bucketLabel(bucket: string, granularity: Granularity) {
  return granularity === "hour" ? bucket.slice(5, 16).replace("T", " ") : bucket.slice(5, 10);
}

// 曲线：预计花费、实际花费、收入（收入只在 show_profit 为真时出现）。
function LedgerCurves({value, granularity}: {value: LedgerAccountSeries; granularity: Granularity}) {
  const estimated = value.summary.estimated;
  const series = [
    {name: "预计花费", kind: "line" as const, values: value.points.map(point => point.expected_balance_cost_usd + point.expected_subscription_cost_usd)},
    {name: estimated ? "实际花费（估算）" : "实际花费", kind: "line" as const, values: value.points.map(point => point.actual_cost_usd)},
    ...(value.show_profit ? [{name: "收入", kind: "line" as const, values: value.points.map(point => point.revenue_usd ?? null)}] : []),
  ];
  return (
    <>
      {estimated && <p className="muted text-fine">部分或全部实际花费为估算值，仅供参考。</p>}
      <HeroMetricChart
        tabs={[{
          id: "ledger",
          label: "花费曲线",
          value: null,
          categories: value.points.map(point => bucketLabel(point.bucket, granularity)),
          series,
          format: smartMoney,
        }]}
        emptyTitle="当前时间窗口暂无账本样本"
      />
    </>
  );
}

// 详情「账本」标签：GET /admin/ledger/accounts/{id}?granularity=hour|day。
export function AccountLedger({account: acc, revision, onChanged}: {account: AccountRow; revision: number; onChanged: () => void}) {
  const [granularity, setGranularity] = useState<Granularity>("day");
  const series = useResource(
    () => api(`/admin/ledger/accounts/${acc.id}?granularity=${granularity}`, ledgerAccountSeries),
    `acc-ledger-${acc.id}-${granularity}`,
    true,
    revision,
  );
  return (
    <Resource state={series}>
      {value => {
        const s = value.summary;
        return (
          <>
            <InsetGroup title="对账概览">
              {!value.tracked && <InsetRow label="状态"><span className="muted">尚未接入上游账本，接入并采样成功后开始记录余额与实际花费。</span></InsetRow>}
              <InsetRow label="余额 / 可撑"><span className="weir-digits">{s.balance_usd != null ? smartMoney(s.balance_usd) : "未知"}{s.runway_hours != null ? ` · 约 ${Math.round(s.runway_hours)} 小时` : ""}</span></InsetRow>
              <InsetRow label="今日实际 / 预计"><span className="weir-digits">{s.actual_today_usd != null ? smartMoney(s.actual_today_usd) : "未知"} / {smartMoney(s.expected_today_usd)}{s.estimated ? " · 估算" : ""}</span></InsetRow>
              {s.deviation_pct != null && <InsetRow label="偏差"><span className="weir-digits">{s.deviation_pct.toFixed(1)}%</span></InsetRow>}
              {s.multiplier != null && <InsetRow label="上游 / 本地倍率"><span className="weir-digits">×{s.multiplier} / {s.local_multiplier != null ? `×${s.local_multiplier}` : "—"}</span></InsetRow>}
              {value.show_profit && s.revenue_today_usd != null && s.profit_today_usd != null && (
                <InsetRow label="今日收入 / 毛利"><span className="weir-digits">{smartMoney(s.revenue_today_usd)} / {smartMoney(s.profit_today_usd)}</span></InsetRow>
              )}
              {s.sampled_at && <InsetRow label="最近采样"><span className="weir-digits">{date(s.sampled_at)}</span></InsetRow>}
              {s.alerts.length > 0 && (
                <InsetRow label="告警">
                  {s.alerts.map(alert => <Badge key={alert.kind} variant="soft" tone={alert.severity === "bad" ? "bad" : alert.severity === "warn" ? "warn" : "neutral"}>{alert.label}</Badge>)}
                </InsetRow>
              )}
            </InsetGroup>
            <InsetGroup title="花费曲线">
              <InsetRow label="粒度">
                <Segmented label="账本粒度" value={granularity} options={[["hour", "小时"], ["day", "日"]] as const} onChange={setGranularity} />
              </InsetRow>
            </InsetGroup>
            <LedgerCurves value={value} granularity={granularity} />
            {value.tracked && <AccountLedgerEvents accountID={acc.id} events={value.events} onChanged={onChanged} />}
            {acc.type === "apikey" ? (
              <AccountLedgerConnect account={acc} connection={value.connection} revision={revision} onChanged={onChanged} />
            ) : (
              <InsetGroup title="接入上游账本">
                <InsetRow label="暂不可接入"><span className="muted">仅 API Key 类型的账号可以接入余额查询，OAuth、Setup Token 等类型没有可查询的上游余额接口。</span></InsetRow>
              </InsetGroup>
            )}
          </>
        );
      }}
    </Resource>
  );
}
