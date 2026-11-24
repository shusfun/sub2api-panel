import {useEffect, useMemo, useState} from "react";
import {KeyRound, Play, Plus, Settings2, Trash2} from "lucide-react";
import {z} from "zod";
import {
  api,
  empty,
  patrolBoardSchema,
  patrolPolicySchema,
  patrolRunDetailSchema,
  probeReportSchema,
  probeTemplateSchema,
  type ProbeReport,
} from "../api";
import {compactTokens, count, date} from "../format";
import {useAction, useResource} from "../requests";
import {
  Badge,
  Button,
  CopyButton,
  EmptyState,
  ErrorLine,
  Feedback,
  Field,
  Form,
  FormGrid,
  FormSection,
  GroupSelect,
  InlineStack,
  Input,
  InsetGroup,
  InsetRow,
  MetricStrip,
  Overlay,
  PasswordInput,
  PlatformBadge,
  QualitySparkBars,
  RefreshButton,
  Resource,
  Segmented,
  Select,
  Stack,
  StatusDot,
  Switch,
  Textarea,
  WorkspaceContent,
  WorkspaceStage,
  toast,
} from "../ui";
import {groupBriefSchema} from "./pool-shared";

type PatrolBoard = z.infer<typeof patrolBoardSchema>;
type PatrolGroupRow = PatrolBoard["groups"][number];
type PatrolPolicy = z.infer<typeof patrolPolicySchema>;
type ProbeTemplate = z.infer<typeof probeTemplateSchema>;

function verdictBadge(verdict: string) {
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

function engineLabel(engine: string) {
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
    const timer = window.setInterval(() => {
      setCountdown(v => Math.max(0, v - 1));
    }, 1000);
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
                    {board.groups.map(g => (
                      <option key={g.group_id} value={String(g.group_id)}>{g.group_name || `分组 #${g.group_id}`}</option>
                    ))}
                  </Select>
                )}
                {admin && onOpenTemplates && (
                  <Button size="sm" variant="outline" onClick={onOpenTemplates}>
                    <Settings2 aria-hidden />探针模板
                  </Button>
                )}
                {admin && onEditPolicy && (
                  <Button size="sm" onClick={() => onEditPolicy(undefined)}>
                    <Plus aria-hidden />配置巡检分组
                  </Button>
                )}
              </InlineStack>
            </div>

            <MetricStrip items={stripItems} />

            <ErrorLine value={triggerAct.error} />

            <section className="patrol-section-surface" aria-label="分组巡检概览">
              <div className="patrol-section-head">
                <InlineStack wrap>
                  <h2>分组概览</h2>
                  <Badge variant="soft" tone="neutral">
                    下轮调度 · <span className="weir-digits">{formatCountdown(countdown)}</span>
                  </Badge>
                </InlineStack>
                <InlineStack wrap>
                  <RefreshButton size="sm" onClick={reload} />
                </InlineStack>
              </div>

              <div className="patrol-legend-bar">
                <div className="patrol-legend-items">
                  <span className="patrol-legend-key">
                    <i className="patrol-legend-swatch" data-tone="ok" aria-hidden />不降智
                  </span>
                  <span className="patrol-legend-key">
                    <i className="patrol-legend-swatch" data-tone="bad" aria-hidden />降智需关注
                  </span>
                  <span className="patrol-legend-key">
                    <i className="patrol-legend-swatch" data-tone="skipped" aria-hidden />低峰智能免检（0 Token）
                  </span>
                  <span className="patrol-legend-key">
                    <i className="patrol-legend-swatch" aria-hidden />请求失败 / 无数据
                  </span>
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
                        {row.latest_run_id ? (
                          <Button size="xs" variant="ghost" onClick={() => setDetailRunID(row.latest_run_id ?? undefined)}>
                            查看详情
                          </Button>
                        ) : null}
                        {admin && (
                          <Button
                            size="xs"
                            variant="outline"
                            busy={triggerAct.busy}
                            onClick={() =>
                              void triggerAct.run(async () => {
                                const run = await api(`/admin/patrol/policies/${row.group_id}/run`, patrolRunDetailSchema, "POST", {});
                                reload();
                                setDetailRunID(run.id);
                              })
                            }
                          >
                            <Play aria-hidden />测一次
                          </Button>
                        )}
                        {admin && onEditPolicy && (
                          <Button size="xs" variant="outline" onClick={() => onEditPolicy(row.group_id)}>
                            策略
                          </Button>
                        )}
                      </InlineStack>
                    </div>
                  </div>
                ))
              )}
            </section>

            {publicMode && board.guest_check_enabled && (
              <GuestSelfCheckSection defaultModel={board.guest_default_model || "gpt-5.4"} />
            )}

            <PatrolRunDetailDialog
              runID={detailRunID}
              publicMode={publicMode}
              onClose={() => setDetailRunID(undefined)}
            />
          </Stack>
        );
      }}
    </Resource>
  );
}

function PatrolRunDetailDialog({
  runID,
  publicMode,
  onClose,
}: {
  runID: number | undefined;
  publicMode: boolean;
  onClose: () => void;
}) {
  const path = runID ? (publicMode ? `/patrol/public/runs/${runID}` : `/patrol/runs/${runID}`) : "";
  const detail = useResource(
    () => api(path, patrolRunDetailSchema, "GET", undefined, publicMode ? false : undefined),
    `patrol-run:${path}`,
    Boolean(runID),
  );

  return (
    <Overlay
      open={Boolean(runID)}
      onClose={onClose}
      title="巡检请求详情"
      footer={
        <InlineStack>
          {detail?.value?.reply_excerpt ? <CopyButton value={detail.value.reply_excerpt} label="复制完整答复" /> : null}
          <Button variant="outline" onClick={onClose}>关闭</Button>
        </InlineStack>
      }
    >
      {runID && (
        <Resource state={detail}>
          {run => {
            const totalSec = run.duration_ms / 1000;
            const tps = totalSec > 0 && run.output_tokens > 0 ? (run.output_tokens / totalSec).toFixed(1) : "未知";
            const usd = (run.estimated_micros / 1_000_000).toFixed(5);
            return (
              <Stack>
                <div className="auth-inline-row">
                  <InlineStack wrap>
                    <strong>{run.group_name}</strong>
                    {verdictBadge(run.verdict)}
                  </InlineStack>
                  <span className="muted text-fine weir-digits">{date(run.created_at)}</span>
                </div>
                <div className="text-body">{run.summary}</div>

                <div className="patrol-detail-grid">
                  <div className="patrol-metric-cell">
                    <span>测试模型</span>
                    <strong>{run.model || "默认"}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>检测引擎</span>
                    <strong>{engineLabel(run.engine)}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>总耗时 / 首字</span>
                    <strong>{totalSec.toFixed(1)}s · {run.ttft_ms > 0 ? `${run.ttft_ms}ms` : "未知"}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>输入 Token</span>
                    <strong>{count.format(run.input_tokens)}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>输出 / 推理 Token</span>
                    <strong>{count.format(run.output_tokens)} / {count.format(run.reasoning_tokens)}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>缓存输入 Token</span>
                    <strong>{count.format(run.cached_tokens)}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>平均 TPS</span>
                    <strong>{tps}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>预估费用 · USD</span>
                    <strong>${usd}</strong>
                  </div>
                  <div className="patrol-metric-cell">
                    <span>窗口活跃用户</span>
                    <strong>{count.format(run.active_users)} 人（{count.format(run.window_requests)} 次请求）</strong>
                  </div>
                </div>

                {run.predicted_model && (
                  <InsetGroup>
                    <InsetRow label="指纹最接近模型"><span>{run.predicted_model}</span></InsetRow>
                    {run.confidence !== null && run.confidence !== undefined && (
                      <InsetRow label="参考库校准置信度"><span className="weir-digits">{`${(run.confidence * 100).toFixed(1)}%`}</span></InsetRow>
                    )}
                  </InsetGroup>
                )}

                {run.reply_excerpt && (
                  <div className="patrol-reply-box">{run.reply_excerpt}</div>
                )}
              </Stack>
            );
          }}
        </Resource>
      )}
    </Overlay>
  );
}

function GuestSelfCheckSection({defaultModel}: {defaultModel: string}) {
  const [baseURL, setBaseURL] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [model, setModel] = useState(defaultModel);
  const [protocol, setProtocol] = useState("chat_completions");
  const [effort, setEffort] = useState("medium");
  const [result, setResult] = useState<ProbeReport>();
  const act = useAction();

  return (
    <section className="patrol-section-surface" aria-label="自助检测">
      <div className="stage-body">
        <div className="auth-inline-row">
          <h2>访客自带接口自助验证</h2>
          <span className="muted text-fine">密钥仅当次内存直发，服务端零日志零留存</span>
        </div>
        <div className="split-grid-2">
          <Form
            id="guest-self-probe"
            busy={act.busy}
            onSubmit={e => {
              e.preventDefault();
              void act.run(async () => {
                const rep = await api("/patrol/guest-probe", probeReportSchema, "POST", {
                  base_url: baseURL,
                  api_key: apiKey,
                  model,
                  protocol,
                  effort,
                }, false);
                setAPIKey("");
                setResult(rep);
              });
            }}
          >
            <FormSection>
              <FormGrid>
                <Field label="网关地址 (HTTPS)">
                  <Input value={baseURL} onChange={e => setBaseURL(e.target.value)} placeholder="https://api.example.com" required />
                </Field>
                <Field label="模型名称">
                  <Input value={model} onChange={e => setModel(e.target.value)} placeholder="gpt-5.4" required />
                </Field>
              </FormGrid>
              <Field label="一次性 API Key（测完自动清空）">
                <PasswordInput toggleLabel="探测密钥" autoComplete="off" value={apiKey} onChange={e => setAPIKey(e.target.value)} placeholder="sk-..." required />
              </Field>
              <FormGrid>
                <Field label="协议">
                  <Select value={protocol} onChange={e => setProtocol(e.target.value)}>
                    <option value="chat_completions">Chat Completions</option>
                    <option value="responses">Responses</option>
                    <option value="messages">Anthropic Messages</option>
                  </Select>
                </Field>
                <Field label="推理强度">
                  <Select value={effort} onChange={e => setEffort(e.target.value)}>
                    <option value="">默认</option>
                    <option value="low">Low</option>
                    <option value="medium">Medium</option>
                    <option value="high">High</option>
                  </Select>
                </Field>
              </FormGrid>
              <ErrorLine value={act.error} />
              <Button type="submit" busy={act.busy}>发起一次性验证</Button>
            </FormSection>
          </Form>

          <div>
            {result ? (
              <InsetGroup title="本次验证结果">
                <InsetRow label="判定结论">{verdictBadge(result.verdict)}</InsetRow>
                <InsetRow label="模型与耗时"><span className="weir-digits">{`${result.model} · ${(result.duration_ms / 1000).toFixed(1)}s（首字 ${result.ttft_ms}ms）`}</span></InsetRow>
                <InsetRow label="Token 用量"><span className="weir-digits">{`输入 ${count.format(result.input_tokens)} · 输出 ${count.format(result.output_tokens)}`}</span></InsetRow>
                <InsetRow label="说明"><span>{result.summary}</span></InsetRow>
              </InsetGroup>
            ) : (
              <EmptyState title="等待发起验证" description="在左侧填写外部网关 HTTPS 地址与临时密钥即可实测糖果推理判定。" />
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

export function PatrolWorkspace({admin}: {admin: boolean}) {
  const [revision, revise] = useState(0);
  const [editingGroup, setEditingGroup] = useState<number | "new">();
  const [templatesOpen, setTemplatesOpen] = useState(false);

  return (
    <WorkspaceStage>
      <WorkspaceContent>
        <PublicPatrolSection
          publicMode={false}
          admin={admin}
          revision={revision}
          onEditPolicy={admin ? gid => setEditingGroup(gid ?? "new") : undefined}
          onOpenTemplates={admin ? () => setTemplatesOpen(true) : undefined}
        />
      </WorkspaceContent>

      {admin && (
        <>
          <PatrolPolicyDialog
            target={editingGroup}
            revision={revision}
            onClose={() => setEditingGroup(undefined)}
            onSaved={() => {
              setEditingGroup(undefined);
              revise(v => v + 1);
            }}
          />
          <ProbeTemplatesDialog
            open={templatesOpen}
            onClose={() => setTemplatesOpen(false)}
          />
        </>
      )}
    </WorkspaceStage>
  );
}

function PatrolPolicyDialog({
  target,
  revision,
  onClose,
  onSaved,
}: {
  target: number | "new" | undefined;
  revision: number;
  onClose: () => void;
  onSaved: () => void;
}) {
  const open = target !== undefined;
  const groups = useResource(() => api("/admin/groups/all", z.array(groupBriefSchema)), "patrol-groups", open, revision);
  const policies = useResource(() => api("/admin/patrol/policies", z.array(patrolPolicySchema)), "patrol-policies", open, revision);
  const templates = useResource(() => api("/admin/patrol/templates", z.array(probeTemplateSchema)), "patrol-tpls", open, revision);

  const existing: PatrolPolicy | undefined = useMemo(() => {
    if (typeof target !== "number") return undefined;
    return policies?.value?.find(p => p.group_id === target);
  }, [policies?.value, target]);

  const [groupID, setGroupID] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [publicVisible, setPublicVisible] = useState(true);
  const [engine, setEngine] = useState("lmfpd");
  const [templateID, setTemplateID] = useState("");
  const [model, setModel] = useState("gpt-5.4");
  const [protocol, setProtocol] = useState("responses");
  const [effort, setEffort] = useState("low");
  const [preset, setPreset] = useState("fast");
  const [patrolKey, setPatrolKey] = useState("");
  const [windowMin, setWindowMin] = useState("15");
  const [minUsers, setMinUsers] = useState("3");
  const [minReqs, setMinReqs] = useState("15");
  const [peakUsers, setPeakUsers] = useState("15");
  const [intervalMin, setIntervalMin] = useState("30");
  const [peakIntervalMin, setPeakIntervalMin] = useState("15");
  const [recheckMin, setRecheckMin] = useState("5");
  const [dailyTokens, setDailyTokens] = useState("300000");
  const [dailyUSD, setDailyUSD] = useState("2.00");
  const [mintNotice, setMintNotice] = useState("");

  const saveAct = useAction();
  const mintAct = useAction();
  const delAct = useAction();

  useEffect(() => {
    setMintNotice("");
    if (existing) {
      setGroupID(String(existing.group_id));
      setEnabled(existing.enabled);
      setPublicVisible(existing.public_visible);
      setEngine(existing.engine);
      setTemplateID(existing.template_id ? String(existing.template_id) : "");
      setModel(existing.model);
      setProtocol(existing.protocol);
      setEffort(existing.effort);
      setPreset(existing.inspector_preset || "fast");
      setPatrolKey("");
      setWindowMin(String(existing.window_minutes));
      setMinUsers(String(existing.min_active_users));
      setMinReqs(String(existing.min_window_requests));
      setPeakUsers(String(existing.peak_active_users));
      setIntervalMin(String(existing.interval_minutes));
      setPeakIntervalMin(String(existing.peak_interval_minutes));
      setRecheckMin(String(existing.recheck_minutes));
      setDailyTokens(String(existing.daily_token_limit));
      setDailyUSD((existing.daily_budget_cents / 100).toFixed(2));
    } else if (target === "new") {
      const firstGroup = groups?.value?.[0];
      if (firstGroup && !groupID) setGroupID(String(firstGroup.id));
    }
  }, [existing, groups?.value, target]);

  const selectedGroup = groups?.value?.find(g => String(g.id) === groupID);

  return (
    <Overlay
      open={open}
      onClose={onClose}
      title={typeof target === "number" ? "编辑分组智能巡检策略" : "新增巡检分组"}
      footer={
        <InlineStack>
          {typeof target === "number" && (
            <Button
              variant="destructive"
              busy={delAct.busy}
              onClick={() =>
                void delAct.run(async () => {
                  await api(`/admin/patrol/policies/${target}`, empty, "DELETE");
                  onSaved();
                })
              }
            >
              <Trash2 aria-hidden />移除监控
            </Button>
          )}
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form="patrol-policy-form" busy={saveAct.busy}>保存策略</Button>
        </InlineStack>
      }
    >
      <Form
        id="patrol-policy-form"
        busy={saveAct.busy}
        onSubmit={e => {
          e.preventDefault();
          const gid = Number(groupID);
          if (!gid) return;
          void saveAct.run(async () => {
            await api(`/admin/patrol/policies/${gid}`, patrolPolicySchema, "PUT", {
              group_name: selectedGroup?.name ?? existing?.group_name ?? `分组 #${gid}`,
              platform: selectedGroup?.platform ?? existing?.platform ?? "openai",
              rate_multiplier: selectedGroup?.rate_multiplier ?? existing?.rate_multiplier ?? 1,
              enabled,
              public_visible: publicVisible,
              engine,
              ...(engine === "template" && Number(templateID) > 0 ? {template_id: Number(templateID)} : {}),
              model: model.trim(),
              protocol,
              effort,
              inspector_preset: preset,
              ...(patrolKey.trim() ? {patrol_api_key: patrolKey.trim()} : {}),
              window_minutes: Number(windowMin) || 15,
              min_active_users: Number(minUsers) || 0,
              min_window_requests: Number(minReqs) || 0,
              peak_active_users: Number(peakUsers) || 15,
              interval_minutes: Number(intervalMin) || 30,
              peak_interval_minutes: Number(peakIntervalMin) || 15,
              recheck_minutes: Number(recheckMin) || 5,
              daily_token_limit: Number(dailyTokens) || 0,
              daily_budget_cents: Math.round((Number(dailyUSD) || 0) * 100),
            });
            onSaved();
          });
        }}
      >
        <Stack>
          <FormSection title="检测目标与引擎">
            <Field label="调度分组">
              <GroupSelect value={groupID} onChange={setGroupID} clearable={false} />
            </Field>
            <Field label="巡检引擎">
              <Segmented
                label="巡检引擎"
                value={engine}
                onChange={setEngine}
                options={[
                  ["lmfpd", "LM 指纹"],
                  ["gpt-relay-inspector", "少点套路"],
                  ["custom", "内置糖果题"],
                  ["template", "自定义模板"],
                ] as const}
              />
            </Field>
            {engine === "template" && (
              <Field label="自定义探针模板">
                <Select value={templateID} onChange={e => setTemplateID(e.target.value)}>
                  <option value="">选择模板</option>
                  {(templates?.value ?? []).map((t: ProbeTemplate) => (
                    <option key={t.id} value={String(t.id)}>{t.name}</option>
                  ))}
                </Select>
              </Field>
            )}
            <FormGrid>
              <Field label="检测模型">
                <Input value={model} onChange={e => setModel(e.target.value)} placeholder="gpt-5.4" required />
              </Field>
              <Field label="网关协议">
                <Select value={protocol} onChange={e => setProtocol(e.target.value)}>
                  <option value="responses">OpenAI Responses</option>
                  <option value="chat_completions">Chat Completions</option>
                  <option value="messages">Anthropic Messages</option>
                </Select>
              </Field>
            </FormGrid>
            <FormGrid>
              <Field label="推理强度">
                <Select value={effort} onChange={e => setEffort(e.target.value)}>
                  <option value="">默认</option>
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                </Select>
              </Field>
              {engine === "gpt-relay-inspector" && (
                <Field label="取证编排档位">
                  <Select value={preset} onChange={e => setPreset(e.target.value)}>
                    <option value="fast">极速快检</option>
                    <option value="full">19 针全维深测</option>
                  </Select>
                </Field>
              )}
            </FormGrid>
          </FormSection>

          <FormSection title="活跃用户自适应门控（省 Token 核心）">
            <FormGrid>
              <Field label="流量观测窗口（分钟）">
                <Input type="number" min={1} max={1440} value={windowMin} onChange={e => setWindowMin(e.target.value)} />
              </Field>
              <Field label="唤醒所需最少活跃用户数">
                <Input type="number" min={0} max={10000} value={minUsers} onChange={e => setMinUsers(e.target.value)} />
              </Field>
            </FormGrid>
            <FormGrid>
              <Field label="或最少窗口请求数">
                <Input type="number" min={0} max={1000000} value={minReqs} onChange={e => setMinReqs(e.target.value)} />
              </Field>
              <Field label="常态巡检间隔（分钟）">
                <Input type="number" min={1} max={1440} value={intervalMin} onChange={e => setIntervalMin(e.target.value)} />
              </Field>
            </FormGrid>
            <FormGrid>
              <Field label="高峰用户数 / 高峰间隔（分钟）">
                <InlineStack>
                  <Input type="number" min={1} value={peakUsers} onChange={e => setPeakUsers(e.target.value)} aria-label="高峰活跃用户阈值" />
                  <Input type="number" min={1} value={peakIntervalMin} onChange={e => setPeakIntervalMin(e.target.value)} aria-label="高峰间隔分钟" />
                </InlineStack>
              </Field>
              <Field label="异常快速复核间隔（分钟）">
                <Input type="number" min={1} max={240} value={recheckMin} onChange={e => setRecheckMin(e.target.value)} />
              </Field>
            </FormGrid>
            <FormGrid>
              <Field label="单日最高 Token 熔断">
                <Input type="number" min={0} value={dailyTokens} onChange={e => setDailyTokens(e.target.value)} />
              </Field>
              <Field label="单日最高预算（USD）">
                <Input type="number" min={0} step="0.1" value={dailyUSD} onChange={e => setDailyUSD(e.target.value)} />
              </Field>
            </FormGrid>
          </FormSection>

          <InsetGroup title="无人值守巡检凭据与可见性">
            <InsetRow label="启用定时巡检">
              <Switch label="启用定时巡检" checked={enabled} onChange={setEnabled} />
            </InsetRow>
            <InsetRow label="在公开首页及用户端展示此分组">
              <Switch label="公开展示此分组" checked={publicVisible} onChange={setPublicVisible} />
            </InsetRow>
            <InsetRow label="巡检专用受限 Key" hint={existing?.has_patrol_key ? "已绑定本组限额巡检密钥" : "保存策略后可一键签发绑定本组且带日限额的专用密钥"}>
              {typeof target === "number" && (
                <Button
                  size="xs"
                  variant="outline"
                  busy={mintAct.busy}
                  onClick={() =>
                    void mintAct.run(async () => {
                      await api(`/admin/patrol/policies/${target}/mint-key`, patrolPolicySchema, "POST", {});
                      setMintNotice("已自动创建并绑定本组专用日限额巡检 Key");
                    })
                  }
                >
                  <KeyRound aria-hidden />一键签发绑定
                </Button>
              )}
            </InsetRow>
          </InsetGroup>

          <Field label="或手动填入现有分组 Key（留空保持不变）">
            <PasswordInput toggleLabel="分组巡检密钥" autoComplete="off" value={patrolKey} onChange={e => setPatrolKey(e.target.value)} placeholder="sk-..." />
          </Field>

          {mintNotice && <Feedback tone="success">{mintNotice}</Feedback>}
          <ErrorLine value={saveAct.error ?? mintAct.error ?? delAct.error} />
        </Stack>
      </Form>
    </Overlay>
  );
}

function ProbeTemplatesDialog({open, onClose}: {open: boolean; onClose: () => void}) {
  const [rev, bump] = useState(0);
  const list = useResource(() => api("/admin/patrol/templates", z.array(probeTemplateSchema)), "probe-templates", open, rev);
  const [name, setName] = useState("");
  const [protocol, setProtocol] = useState("chat_completions");
  const [sysPrompt, setSysPrompt] = useState("");
  const [userPrompt, setUserPrompt] = useState("");
  const [effort, setEffort] = useState("");
  const [matchKind, setMatchKind] = useState("exact_number");
  const [expected, setExpected] = useState("");
  const act = useAction();

  return (
    <Overlay open={open} onClose={onClose} title="自定义探针模板管理" footer={<Button variant="outline" onClick={onClose}>关闭</Button>}>
      <Stack>
        <Resource state={list}>
          {items =>
            items.length === 0 ? (
              <EmptyState title="暂无自定义模板" description="可在下方录入私有糖果题、金丝雀指令或正则校验规则。" />
            ) : (
              <InsetGroup title="已有模板">
                {items.map(item => (
                  <InsetRow key={item.id} label={item.name} hint={`${item.protocol} · ${item.match_kind}${item.expected ? `: ${item.expected}` : ""}`}>
                    <Button
                      size="xs"
                      variant="destructive"
                      busy={act.busy}
                      onClick={() =>
                        void act.run(async () => {
                          await api(`/admin/patrol/templates/${item.id}`, empty, "DELETE");
                          bump(v => v + 1);
                        })
                      }
                    >
                      删除
                    </Button>
                  </InsetRow>
                ))}
              </InsetGroup>
            )
          }
        </Resource>

        <Form
          id="new-probe-tpl"
          busy={act.busy}
          onSubmit={e => {
            e.preventDefault();
            void act.run(async () => {
              await api("/admin/patrol/templates", probeTemplateSchema, "POST", {
                id: 0,
                name: name.trim(),
                protocol,
                system_prompt: sysPrompt,
                user_prompt: userPrompt,
                effort,
                stream: true,
                max_tokens: 2048,
                match_kind: matchKind,
                expected: expected.trim(),
                min_reasoning_tokens: 0,
                max_ttft_ms: 0,
              });
              setName("");
              setUserPrompt("");
              setExpected("");
              bump(v => v + 1);
            });
          }}
        >
          <FormSection title="新建自定义探针模板">
            <FormGrid>
              <Field label="模板名称">
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="如：多步逻辑推理题 v2" required />
              </Field>
              <Field label="协议">
                <Select value={protocol} onChange={e => setProtocol(e.target.value)}>
                  <option value="chat_completions">Chat Completions</option>
                  <option value="responses">Responses</option>
                  <option value="messages">Messages</option>
                </Select>
              </Field>
            </FormGrid>
            <Field label="System 提示词（可选）">
              <Input value={sysPrompt} onChange={e => setSysPrompt(e.target.value)} placeholder="仅输出最终结果数字，不要解释" />
            </Field>
            <Field label="测试题目 / User Prompt">
              <Textarea value={userPrompt} onChange={e => setUserPrompt(e.target.value)} required />
            </Field>
            <FormGrid>
              <Field label="判定方式">
                <Select value={matchKind} onChange={e => setMatchKind(e.target.value)}>
                  <option value="exact_number">匹配独立数字</option>
                  <option value="contains">包含指定文本</option>
                  <option value="regex">正则表达式匹配</option>
                  <option value="status_ok">仅校验流式正常返回</option>
                </Select>
              </Field>
              <Field label="期望命中值">
                <Input value={expected} onChange={e => setExpected(e.target.value)} placeholder="例如：21" />
              </Field>
            </FormGrid>
            <Field label="默认推理强度">
              <Select value={effort} onChange={e => setEffort(e.target.value)}>
                <option value="">不指定</option>
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
              </Select>
            </Field>
            <ErrorLine value={act.error} />
            <Button type="submit" busy={act.busy}>添加模板</Button>
          </FormSection>
        </Form>
      </Stack>
    </Overlay>
  );
}
