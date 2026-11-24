import {useId, useState} from "react";
import {Plus, RotateCcw, ShieldAlert, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {empty, opsAlertEventSchema, opsAlertRuleSchema, opsSettingsBundleSchema} from "@/schemas";
import {
  Badge, Button, CollapsibleSection, ConfirmDialog, DataTable, EmptyState, ErrorLine, Feedback,
  Field, Form, FormActions, FormGrid, FormSection, InlineStack, Input, InsetGroup, InsetRow,
  Overlay, Panel, PlatformSelect, QuickPills, Resource, Select, Status, Switch,
} from "@/ui";

type AlertRule = z.infer<typeof opsAlertRuleSchema>;
type AlertEvent = z.infer<typeof opsAlertEventSchema>;

const metricTypeOptions = [
  ["error_rate", "客户端请求错误率 (%)"],
  ["upstream_error_rate", "上游渠道错误率 (%)"],
  ["success_rate", "整体可用率 SLA (%)"],
  ["concurrency_queue_depth", "并发排队深度"],
  ["account_error_count", "异常上游账号数"],
  ["account_rate_limited_count", "限流上游账号数"],
  ["overload_account_count", "过载上游账号数"],
] as const;

const metricLabelMap: Record<string, string> = {
  error_rate: "客户端请求错误率",
  upstream_error_rate: "上游渠道错误率",
  success_rate: "整体可用率 SLA",
  concurrency_queue_depth: "并发排队深度",
  account_error_count: "异常上游账号数",
  account_rate_limited_count: "限流上游账号数",
  overload_account_count: "过载上游账号数",
};

const operatorLabelMap: Record<string, string> = {
  ">": "大于",
  ">=": "不低于",
  "<": "低于",
  "<=": "不超过",
  "==": "等于",
  "!=": "不等于",
};

const severityLabelMap: Record<string, string> = {
  critical: "严重",
  warning: "预警",
  info: "提示",
};

const silencePresets = [
  {value: "30", label: "30 分钟"},
  {value: "60", label: "1 小时"},
  {value: "240", label: "4 小时"},
  {value: "1440", label: "24 小时"},
] as const;

function formatRuleCondition(row: AlertRule) {
  const metric = metricLabelMap[row.metric_type] ?? row.metric_type;
  const op = operatorLabelMap[row.operator] ?? row.operator;
  const unit = row.metric_type.endsWith("_rate") ? "%" : "";
  return `${metric}${op} ${row.threshold}${unit}（窗口 ${row.window_minutes} 分钟）`;
}

export function OpsAlertsAndSettings({revision, onChanged}: {revision: number; onChanged: () => void}) {
  const formID = useId();
  const silenceFormID = useId();
  const rules = useResource(() => api("/admin/ops/alert-rules", z.array(opsAlertRuleSchema)), "ops-rules", true, revision);
  const events = useResource(() => api("/admin/ops/alert-events", z.array(opsAlertEventSchema)), "ops-events-all", true, revision);
  const settings = useResource(() => api("/admin/ops/settings", opsSettingsBundleSchema), "ops-settings", true, revision);

  const [ruleModal, setRuleModal] = useState<{mode: "create"} | {mode: "edit"; rule: AlertRule}>();
  const [silenceEvent, setSilenceEvent] = useState<AlertEvent>();
  const [silenceMinutes, setSilenceMinutes] = useState("60");
  const [silencePlatform, setSilencePlatform] = useState("");
  const [silenceReason, setSilenceReason] = useState("");
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
                          <Button size="xs" variant="outline" busy={action.busy} onClick={() => void action.run(async () => {
                            await api(`/admin/ops/alert-events/${row.id}/status`, empty, "PUT", {status: "manual_resolved"});
                            onChanged();
                          })}>标记恢复</Button>
                          <Button size="xs" variant="ghost" busy={action.busy} onClick={() => {
                            action.clearError();
                            setSilenceMinutes("60");
                            setSilencePlatform("");
                            setSilenceReason("控制台临时静默");
                            setSilenceEvent(row);
                          }}>静默</Button>
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
          actions={<Button size="xs" onClick={() => { action.clearError(); setNotifyEmail(true); setRuleModal({mode: "create"}); }}><Plus aria-hidden />新建规则</Button>}
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
                        <div className="muted text-fine">{formatRuleCondition(row)}</div>
                      </div>
                    ),
                  },
                  {id: "sev", title: "级别", render: row => <Badge variant="soft" tone={row.severity === "critical" ? "bad" : row.severity === "warning" ? "warn" : "info"}>{severityLabelMap[row.severity] ?? row.severity}</Badge>},
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
                        <Button size="icon-xs" variant="ghost" title="删除规则" aria-label={`删除 ${row.name}`} onClick={() => setDeletingRule(row)}><Trash2 /></Button>
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
        {bundle => <OpsSettingsForm key={revision} bundle={bundle} savedNotice={savedNotice} onSaved={() => { setSavedNotice("运行策略与通知设置已保存"); onChanged(); }} />}
      </Resource>

      <Overlay
        open={!!ruleModal}
        title={ruleModal?.mode === "edit" ? "编辑告警规则" : "新建告警规则"}
        onClose={() => setRuleModal(undefined)}
        busy={action.busy}
        footer={<><Button variant="outline" onClick={() => setRuleModal(undefined)}>取消</Button><Button type="submit" form={formID} busy={action.busy}>保存规则</Button></>}
      >
        {ruleModal && (
          <Form
            id={formID}
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
                if (ruleModal.mode === "edit" && ruleModal.rule.id) await api(`/admin/ops/alert-rules/${ruleModal.rule.id}`, opsAlertRuleSchema, "PUT", payload);
                else await api("/admin/ops/alert-rules", opsAlertRuleSchema, "POST", payload);
                setRuleModal(undefined);
                onChanged();
              });
            }}
          >
            <Field label="规则名称"><Input name="name" required maxLength={100} defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.name : ""} /></Field>
            <FormGrid>
              <Field label="监控指标"><Select name="metric_type" defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.metric_type : "error_rate"}>{metricTypeOptions.map(([val, text]) => <option key={val} value={val}>{text}</option>)}</Select></Field>
              <Field label="严重级别"><Select name="severity" defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.severity : "warning"}><option value="critical">严重 (critical)</option><option value="warning">预警 (warning)</option><option value="info">提示 (info)</option></Select></Field>
              <Field label="比较符"><Select name="operator" defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.operator : ">"}><option value=">">&gt;</option><option value=">=">&gt;=</option><option value="<">&lt;</option><option value="<=">&lt;=</option></Select></Field>
              <Field label="触发阈值"><Input name="threshold" type="number" step="0.1" required defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.threshold : 5} /></Field>
              <Field label="统计窗口（分钟）"><Input name="window_minutes" type="number" min="1" max="1440" required defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.window_minutes : 5} /></Field>
              <Field label="冷却时间（分钟）"><Input name="cooldown_minutes" type="number" min="0" max="10080" required defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.cooldown_minutes : 30} /></Field>
            </FormGrid>
            <Field label="补充说明"><Input name="description" maxLength={200} defaultValue={ruleModal.mode === "edit" ? ruleModal.rule.description : ""} /></Field>
            <InsetGroup><InsetRow label="触发时发送邮件通知"><Switch label="发送邮件通知" checked={notifyEmail} onChange={setNotifyEmail} /></InsetRow></InsetGroup>
            <ErrorLine value={action.error} />
          </Form>
        )}
      </Overlay>

      <Overlay
        open={!!silenceEvent}
        title="配置告警静默"
        onClose={() => setSilenceEvent(undefined)}
        busy={action.busy}
        footer={<><Button variant="outline" onClick={() => setSilenceEvent(undefined)}>取消</Button><Button type="submit" form={silenceFormID} busy={action.busy}>确认静默</Button></>}
      >
        {silenceEvent && (
          <Form
            id={silenceFormID}
            busy={action.busy}
            onSubmit={() => {
              const mins = Math.max(1, Number(silenceMinutes) || 60);
              const until = new Date(Date.now() + mins * 60_000).toISOString();
              void action.run(async () => {
                await api("/admin/ops/alert-silences", empty, "POST", {
                  rule_id: silenceEvent.rule_id,
                  platform: silencePlatform,
                  until,
                  reason: silenceReason.trim() || `控制台静默 ${mins} 分钟`,
                });
                setSilenceEvent(undefined);
                onChanged();
              });
            }}
          >
            <InsetGroup>
              <InsetRow label="目标规则" hint={silenceEvent.title || `规则 #${silenceEvent.rule_id}`} />
            </InsetGroup>
            <Field label="快捷时长">
              <QuickPills
                label="静默时长预设"
                options={silencePresets}
                value={silencePresets.some(p => p.value === silenceMinutes) ? silenceMinutes : ""}
                onChange={setSilenceMinutes}
              />
            </Field>
            <FormGrid>
              <Field label="自定义时长（分钟）">
                <Input type="number" min="1" max="43200" required value={silenceMinutes} onChange={e => setSilenceMinutes(e.target.value)} />
              </Field>
              <Field label="限定平台（可选）">
                <PlatformSelect value={silencePlatform} onChange={setSilencePlatform} />
              </Field>
            </FormGrid>
            <Field label="静默原因">
              <Input maxLength={200} value={silenceReason} onChange={e => setSilenceReason(e.target.value)} placeholder="例如：上游维护期间临时静默" />
            </Field>
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

function OpsSettingsForm({bundle, savedNotice, onSaved}: {bundle: z.infer<typeof opsSettingsBundleSchema>; savedNotice: string; onSaved: () => void}) {
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
          logging: {...bundle.logging, level: logLevel},
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
          <InsetRow label="启用邮件告警通知" hint="达到规则阈值时自动向收件人发送通知"><Switch label="启用邮件告警" checked={emailEnabled} onChange={setEmailEnabled} /></InsetRow>
          <Field label="告警收件邮箱（逗号分隔）"><Input value={recipients} onChange={e => setRecipients(e.target.value)} placeholder="ops@example.com" /></Field>
          <Field label="最低 SLA 红线 (%)"><div className="row-field"><Input type="number" step="0.1" value={slaMin} onChange={e => setSlaMin(e.target.value)} /></div></Field>
          <Field label="TTFT P99 上限 (ms)"><div className="row-field"><Input type="number" value={ttftMax} onChange={e => setTtftMax(e.target.value)} /></div></Field>
          <FormSection title="请求 / 上游错误率红线 (%)">
            <FormGrid>
              <Field label="请求错误率红线"><Input type="number" step="0.1" value={reqErrMax} onChange={e => setReqErrMax(e.target.value)} aria-label="请求错误率红线" /></Field>
              <Field label="上游错误率红线"><Input type="number" step="0.1" value={upErrMax} onChange={e => setUpErrMax(e.target.value)} aria-label="上游错误率红线" /></Field>
            </FormGrid>
          </FormSection>
        </FormSection>

        <FormSection title="运行时日志与噪音过滤">
          <Field label="运行时日志级别">
            <InlineStack density="compact">
              <div className="row-field">
                <Select value={logLevel} onChange={e => setLogLevel(e.target.value)}>
                  <option value="debug">debug</option>
                  <option value="info">info</option>
                  <option value="warn">warn</option>
                  <option value="error">error</option>
                </Select>
              </div>
              <Button
                size="sm"
                variant="outline"
                busy={save.busy}
                onClick={() => void save.run(async () => {
                  const res = await api("/admin/ops/runtime/logging/reset", z.object({level: z.string()}), "POST", {});
                  setLogLevel(res.level || "info");
                  onSaved();
                })}
              >
                <RotateCcw aria-hidden />重置默认
              </Button>
            </InlineStack>
          </Field>
          <InsetRow label="忽略客户端主动断开 (Context Canceled)" hint="不计入 SLA 故障统计"><Switch label="忽略客户端取消" checked={ignoreCanceled} onChange={setIgnoreCanceled} /></InsetRow>
          <InsetRow label="忽略无效 API Key 请求" hint="401 鉴权拒绝不计入网关故障"><Switch label="忽略无效 Key" checked={ignoreInvalidKey} onChange={setIgnoreInvalidKey} /></InsetRow>
          <InsetRow label="忽略用户余额不足拦截" hint="402/403 余额耗尽不触发渠道告警"><Switch label="忽略余额不足" checked={ignoreNoBalance} onChange={setIgnoreNoBalance} /></InsetRow>
        </FormSection>
      </div>

      <CollapsibleSection title="数据保留策略摘要" summary={`错误日志保留 ${bundle.advanced.data_retention.error_log_retention_days} 天`}>
        <div className="muted text-meta">
          分钟级指标保留 {bundle.advanced.data_retention.minute_metrics_retention_days} 天 · 小时级指标保留 {bundle.advanced.data_retention.hourly_metrics_retention_days} 天
        </div>
      </CollapsibleSection>

      <ErrorLine value={save.error} />
      {savedNotice && <Feedback tone="success">{savedNotice}</Feedback>}
      <FormActions><Button type="submit" busy={save.busy}><ShieldAlert aria-hidden />保存监控与告警设置</Button></FormActions>
    </Form>
  );
}
