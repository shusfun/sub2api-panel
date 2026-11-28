import {useMemo} from "react";
import {Bell} from "lucide-react";
import type {z} from "zod";
import {compact, count, durationText, smartMoney, uptimeText} from "@/lib/format";
import type {adminOverview, opsConcurrencySchema, opsOverviewSchema} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  Button, DataTable, EmptyState, EntityLink, KpiStrip, MeterBar, MetricTrend, Panel,
  PlatformBadge, RankedBarList, SegmentBar, Stack, Verdict, useChartPalette,
  type HeroChartTab,
} from "@/ui";

export function OpsLiveOverview({
  conc,
  dash,
  ops,
  firingCount,
  badAccounts,
  onOpenTab,
}: {
  conc?: z.infer<typeof opsConcurrencySchema>;
  dash?: z.infer<typeof adminOverview>;
  ops?: z.infer<typeof opsOverviewSchema>;
  firingCount: number;
  badAccounts: number;
  onOpenTab: (tab: "live" | "errors" | "alerts") => void;
}) {
  const palette = useChartPalette();
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
        series: [{name: "QPS", values: pts.map(p => p.qps), color: palette.c1, kind: "area"}],
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
        id: "concurrency",
        label: "并发与排队",
        value: conc?.enabled ? count.format(totalInUse) : null,
        categories: (conc?.platforms ?? []).map(item => item.platform),
        series: [
          {name: "并发占用", values: (conc?.platforms ?? []).map(item => conc?.enabled ? item.current_in_use : null), color: palette.c1, kind: "bar"},
          {name: "排队", values: (conc?.platforms ?? []).map(item => conc?.enabled ? item.waiting_in_queue : null), color: palette.c5, kind: "bar"},
        ],
      },
    ];
  }, [conc, dash, ops, palette, totalInUse, totalWaiting]);

  return (
    <>
      <Verdict
        tone={!ops ? "info" : firingCount > 0 || (sla !== undefined && sla < 95) ? "bad" : attention.length ? "warn" : "ok"}
        title={!ops ? "正在读取运行状态" : attention.length ? `${attention.length} 项需要处理` : "运行正常"}
        detail={attention.length ? attention.join(" · ") : uptime ? `已连续运行 ${uptime}` : undefined}
        actions={
          <>
            {firingCount > 0 && <Button size="sm" variant="outline" onClick={() => onOpenTab("alerts")}><Bell aria-hidden />查看告警</Button>}
            {badAccounts > 0 && <Button size="sm" variant="outline" onClick={() => navigateTo("accounts", {status: "error"})}>处理异常账号</Button>}
            {ops && ops.overview.error_count_total > 0 && <Button size="sm" variant="ghost" onClick={() => onOpenTab("errors")}>排查报错</Button>}
          </>
        }
      />
      <KpiStrip
        items={[
          {id: "qps", label: "当前吞吐 req/s", value: conc ? conc.qps.current.toFixed(1) : ops ? ops.overview.qps.current.toFixed(1) : "—", delta: conc ? `峰值 ${conc.qps.peak.toFixed(1)}` : undefined, spark: (ops?.throughput_points ?? []).map(p => p.qps)},
          {
            id: "sla",
            label: "成功率",
            value: sla !== undefined ? `${sla.toFixed(sla >= 99.95 ? 0 : 1)}%` : "—",
            delta: ops ? `${count.format(ops.overview.error_count_total)} 次报错 · 429×${ops.overview.upstream_429_count}` : undefined,
            tone: sla === undefined ? "neutral" : sla < 95 ? "bad" : sla < 99 ? "warn" : "ok",
            spark: (ops?.error_points ?? []).map(p => p.error_count_sla + p.upstream_429_count),
            onClick: () => onOpenTab("errors"),
            actionLabel: "查看报错",
          },
          {id: "latency", label: "平均响应", value: ops?.overview.duration.avg_ms != null ? durationText(ops.overview.duration.avg_ms) : "—", tone: "cyan"},
          ...(conc && !conc.enabled ? [] : [
            {id: "conc", label: "并发占用", value: conc?.enabled ? count.format(totalInUse) : "—", delta: conc?.enabled ? (totalWaiting > 0 ? `${totalWaiting} 个请求排队中` : "无排队") : undefined, tone: totalWaiting > 0 ? "warn" as const : "neutral" as const},
          ]),
          {id: "cost", label: "今日扣费", value: dash?.traffic ? smartMoney(dash.traffic.today_cost) : "—", delta: dash?.traffic ? `${compact.format(dash.traffic.today_tokens)} Token` : undefined, tone: "violet", spark: (dash?.trend ?? []).map(d => d.actual_cost), onClick: () => navigateTo("usage", {period: "today"}), actionLabel: "查看今日用量"},
        ]}
      />
      <div className="split-grid-2">
        {heroTabs
          .filter(chart => chart.id !== "cost" && (chart.id !== "concurrency" || (conc?.platforms?.length ?? 0) > 1))
          .map(chart => <MetricTrend key={chart.id} metric={chart} primary={chart.id === "throughput"} hideValue switchable={chart.id !== "latency" && chart.id !== "concurrency"} />)}
      </div>
      <h2 className="stage-heading">历史</h2>
      {heroTabs.filter(chart => chart.id === "cost").map(chart => <MetricTrend key={chart.id} metric={chart} />)}

      <div className="split-grid-2">
        <Panel
          title="分组实时并发与健康配比"
          padded={false}
          actions={<Button size="xs" variant="ghost" onClick={() => navigateTo("accounts")}>查看号池 →</Button>}
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
                  <MeterBar size="sm" value={row.max_capacity > 0 ? (row.current_in_use / row.max_capacity) * 100 : 0} label={`${row.current_in_use}/${row.max_capacity || "∞"}`} />
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
          actions={<Button size="xs" variant="ghost" onClick={() => navigateTo("usage")}>完整账单分析 →</Button>}
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
    </>
  );
}
