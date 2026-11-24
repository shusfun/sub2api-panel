import {useEffect, useState} from "react";
import {Play, Plus, Settings2} from "lucide-react";
import type {z} from "zod";
import {api} from "@/lib/http";
import {compactTokens, count, date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {patrolBoardSchema, patrolRunDetailSchema} from "@/schemas";
import {
  Badge, Button, EmptyState, ErrorLine, InlineStack, MetricStrip, PlatformBadge,
  QualitySparkBars, RefreshButton, Resource, Select, Stack, StatusDot,
} from "@/ui";
import {GuestSelfCheckSection, PatrolRunDetailDialog} from "./patrol-details";

type PatrolBoard = z.infer<typeof patrolBoardSchema>;
type PatrolGroupRow = PatrolBoard["groups"][number];

export function verdictBadge(verdict: string) {
  switch (verdict) {
    case "ok":
      return <Badge variant="soft" tone="ok">不降智</Badge>;
    case "degraded":
      return <Badge variant="soft" tone="bad">降智需关注</Badge>;
    case "failed":
      return <Badge variant="soft" tone="warn">检测失败</Badge>;
    case "skipped_idle":
      return <Badge variant="soft" tone="neutral">低峰免检</Badge>;
    default:
      return <Badge variant="soft" tone="neutral">待检测</Badge>;
  }
}

export function engineLabel(engine: string) {
  switch (engine) {
    case "lmfpd":
      return "LM 指纹";
    case "gpt-relay-inspector":
      return "少点套路";
    case "template":
      return "自定义模板";
    default:
      return "糖果快检";
  }
}

function formatCountdown(sec: number) {
  if (sec <= 0) return "即将调度";
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export function PublicPatrolSection({
  publicMode,
  admin = false,
  revision = 0,
  onRefresh,
  onEditPolicy,
  onOpenTemplates,
}: {
  publicMode: boolean;
  admin?: boolean;
  revision?: number;
  onRefresh?: () => void;
  onEditPolicy?: (groupID?: number) => void;
  onOpenTemplates?: () => void;
}) {
  const endpoint = publicMode ? "/patrol/public" : "/patrol";
  const [localRev, bump] = useState(0);
  const mergedRev = revision + localRev;
  const boardState = useResource(
    () => api(endpoint, patrolBoardSchema, "GET", undefined, publicMode ? false : undefined),
    `patrol-board:${endpoint}`,
    true,
    mergedRev,
  );
  const [groupScope, setGroupScope] = useState("all");
  const [detailRunID, setDetailRunID] = useState<number>();
  const [countdown, setCountdown] = useState(0);
  const triggerAct = useAction();

  useEffect(() => {
    setCountdown(boardState?.value?.next_countdown_sec ?? 0);
  }, [boardState?.value?.next_countdown_sec, boardState?.value?.synced_at]);

  useEffect(() => {
    if (countdown <= 0) return;
    const timer = window.setInterval(() => setCountdown(v => Math.max(0, v - 1)), 1000);
    return () => window.clearInterval(timer);
  }, [countdown > 0]);

  const reload = () => {
    bump(v => v + 1);
    onRefresh?.();
  };

  return (
    <Resource state={boardState}>
      {board => {
        const visibleGroups = groupScope === "all"
          ? board.groups
          : board.groups.filter(g => String(g.group_id) === groupScope);

        const stripItems = [
          {
            id: "groups",
            label: "监控分组",
            value: count.format(board.group_count),
            hint: `${board.healthy_groups} 个不降智 · ${board.degraded_groups} 个需关注 · ${board.idle_groups} 个免检或待测`,
          },
          {
            id: "rate",
            label: "综合不降智率",
            value: board.overall_healthy_rate === null ? null : board.overall_healthy_rate.toFixed(1),
            unit: board.overall_healthy_rate === null ? undefined : "%",
            tone: board.overall_healthy_rate !== null && board.overall_healthy_rate < 85 ? ("bad" as const) : ("ok" as const),
            hint: `${board.total_healthy_runs}/${board.total_valid_runs} 次有效检测 · ${board.total_failed_runs} 次失败不计入比率`,
          },
          {
            id: "duration",
            label: "分组平均耗时",
            value: board.avg_duration_ms === null ? null : (board.avg_duration_ms / 1000).toFixed(1),
            unit: board.avg_duration_ms === null ? undefined : "s",
            hint: "各组最近一次已完成有效检测均值",
          },
          {
            id: "tokens",
            label: "近期检测 Token",
            value: compactTokens(board.recent_tokens),
            hint: `今日低峰智能免检 ${board.idle_skipped_today} 轮 · 约省 ${compactTokens(board.estimated_saved_tokens)} Token`,
          },
        ];

        return (
          <Stack>
            <div className="auth-inline-row">
              <InlineStack wrap>
                <StatusDot tone={board.degraded_groups > 0 ? "warn" : "ok"} pulse />
                <strong>{board.enabled ? "智能流量门控巡检运行中" : "巡检已暂停"}</strong>
                <span className="muted text-fine">同步于 {date(board.synced_at)}</span>
              </InlineStack>
              <InlineStack wrap>
                {board.groups.length > 1 && (
                  <Select aria-label="分组范围" value={groupScope} onChange={e => setGroupScope(e.target.value)}>
                    <option value="all">全部分组 ({board.groups.length})</option>
                    {board.groups.map(g => <option key={g.group_id} value={String(g.group_id)}>{g.group_name || `分组 #${g.group_id}`}</option>)}
                  </Select>
                )}
                {admin && onOpenTemplates && (
                  <Button size="sm" variant="outline" onClick={onOpenTemplates}><Settings2 aria-hidden />探针模板</Button>
                )}
                {admin && onEditPolicy && (
                  <Button size="sm" onClick={() => onEditPolicy(undefined)}><Plus aria-hidden />配置巡检分组</Button>
                )}
              </InlineStack>
            </div>

            <MetricStrip items={stripItems} />
            <ErrorLine value={triggerAct.error} />

            <section className="patrol-section-surface" aria-label="分组巡检概览">
              <div className="patrol-section-head">
                <InlineStack wrap>
                  <h2>分组概览</h2>
                  <Badge variant="soft" tone="neutral">下轮调度 · <span className="weir-digits">{formatCountdown(countdown)}</span></Badge>
                </InlineStack>
                <InlineStack wrap><RefreshButton size="sm" onClick={reload} /></InlineStack>
              </div>

              <div className="patrol-legend-bar">
                <div className="patrol-legend-items">
                  <span className="patrol-legend-key"><i className="patrol-legend-swatch" data-tone="ok" aria-hidden />不降智</span>
                  <span className="patrol-legend-key"><i className="patrol-legend-swatch" data-tone="bad" aria-hidden />降智需关注</span>
                  <span className="patrol-legend-key"><i className="patrol-legend-swatch" data-tone="skipped" aria-hidden />低峰智能免检（0 Token）</span>
                  <span className="patrol-legend-key"><i className="patrol-legend-swatch" aria-hidden />请求失败 / 无数据</span>
                </div>
                <span>活跃用户低于门控阈值时自动静默免检；网络失败不参与降智率分母计算。</span>
              </div>

              {visibleGroups.length === 0 ? (
                <div className="stage-body">
                  <EmptyState
                    title="暂无巡检分组"
                    description={admin ? "选择调度分组并设定活跃用户门控阈值即可开启省 Token 巡检。" : "管理员尚未开放公开巡检分组。"}
                    action={admin && onEditPolicy ? <Button size="sm" onClick={() => onEditPolicy(undefined)}>添加首个巡检分组</Button> : undefined}
                  />
                </div>
              ) : (
                visibleGroups.map((row: PatrolGroupRow) => (
                  <div key={row.group_id} className="patrol-group-row">
                    <div className="patrol-group-id">
                      <InlineStack wrap>
                        <PlatformBadge platform={row.platform} />
                        <span className="patrol-group-title" title={row.group_name}>{row.group_name || `分组 #${row.group_id}`}</span>
                      </InlineStack>
                      <div className="patrol-group-tags">
                        <span>{row.model || "默认模型"}</span>
                        {row.effort && <span>· {row.effort}</span>}
                        <span>· 倍率 {row.rate_multiplier}×</span>
                        <span>· {engineLabel(row.engine)}</span>
                        <span>· ≥{row.min_active_users} 活跃用户唤醒</span>
                      </div>
                      <div>{verdictBadge(row.latest_verdict)}</div>
                    </div>

                    <div className="patrol-group-metrics">
                      <div className="patrol-metric-cell">
                        <span>不降智率</span>
                        <strong>{row.healthy_rate === null ? "未知" : `${row.healthy_rate.toFixed(1)}%`}</strong>
                      </div>
                      <div className="patrol-metric-cell">
                        <span>最近耗时</span>
                        <strong>{row.latest_duration_ms === null ? "未知" : `${(row.latest_duration_ms / 1000).toFixed(1)}s`}</strong>
                      </div>
                      <div className="patrol-metric-cell">
                        <span>最近 Token</span>
                        <strong>{row.latest_tokens === null ? "未知" : compactTokens(row.latest_tokens)}</strong>
                      </div>
                    </div>

                    <div className="patrol-group-bars">
                      <QualitySparkBars bars={row.bars} onSelect={id => setDetailRunID(id)} />
                    </div>

                    <div className="patrol-group-tail">
                      <span className="muted text-fine weir-digits">{row.latest_at ? date(row.latest_at) : "尚无记录"}</span>
                      <InlineStack wrap>
                        {row.latest_run_id ? <Button size="xs" variant="ghost" onClick={() => setDetailRunID(row.latest_run_id ?? undefined)}>查看详情</Button> : null}
                        {admin && (
                          <Button
                            size="xs"
                            variant="outline"
                            busy={triggerAct.busy}
                            onClick={() => void triggerAct.run(async () => {
                              const run = await api(`/admin/patrol/policies/${row.group_id}/run`, patrolRunDetailSchema, "POST", {});
                              reload();
                              setDetailRunID(run.id);
                            })}
                          >
                            <Play aria-hidden />测一次
                          </Button>
                        )}
                        {admin && onEditPolicy && <Button size="xs" variant="outline" onClick={() => onEditPolicy(row.group_id)}>策略</Button>}
                      </InlineStack>
                    </div>
                  </div>
                ))
              )}
            </section>

            {publicMode && board.guest_check_enabled && <GuestSelfCheckSection defaultModel={board.guest_default_model || "gpt-5.4"} />}
            <PatrolRunDetailDialog runID={detailRunID} publicMode={publicMode} onClose={() => setDetailRunID(undefined)} />
          </Stack>
        );
      }}
    </Resource>
  );
}
