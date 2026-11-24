import {useId, useState} from "react";
import {z} from "zod";
import {api, streamProbe} from "@/lib/http";
import {empty, ephemeralPassSchema, type ProbeReport} from "@/schemas";
import {date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {
  Badge,
  Button,
  ConfirmDialog,
  CopyButton,
  CronField,
  ErrorLine,
  Feedback,
  Field,
  Form,
  FormSection,
  InlineStack,
  Input,
  InsetGroup,
  InsetRow,
  ModelSelect,
  Overlay,
  Resource,
  Segmented,
  Select,
  Status,
  Switch,
  Textarea,
  type ModelCatalog,
} from "@/ui";

export function AccountProbeSection({
  accountID,
  models,
  busy,
  runAction,
}: {
  accountID: number;
  models: ModelCatalog;
  busy: boolean;
  runAction: (work: (trace: string) => Promise<void>) => Promise<void>;
}) {
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

  return (
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
          busy={busy}
          onClick={() =>
            void runAction(async () => {
              setProbeReport(undefined);
              setProbeStreamText("");
              setProbeStatus("正在发起探测…");
              const rep = await streamProbe(
                `/admin/accounts/${accountID}/probe`,
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
          busy={busy}
          onClick={() =>
            void runAction(async () => {
              const pass = await api(`/admin/accounts/${accountID}/probe-pass`, ephemeralPassSchema, "POST", {});
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
  );
}

const scheduledTest = z.object({id: z.number(), model_id: z.string(), cron_expression: z.string(), enabled: z.boolean(), auto_recover: z.boolean(), next_run_at: z.string().nullable()});
const scheduledResults = z.array(z.object({id: z.number(), status: z.string(), latency_ms: z.number(), started_at: z.string()}));

export function AccountScheduledTests({accountID}: {accountID: number}) {
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
  const [cronExpr, setCronExpr] = useState("0 */6 * * *");
  const [planEnabled, setPlanEnabled] = useState(true);
  const [autoRecover, setAutoRecover] = useState(false);
  const openEditor = (plan?: z.infer<typeof scheduledTest>) => {
    action.clearError();
    setModel(plan?.model_id ?? "");
    setCronExpr(plan?.cron_expression ?? "0 */6 * * *");
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
      <Form id={formID} onSubmit={() => {
        void action.run(async () => {
          await api(editing.id ? path + "/" + editing.id : path, scheduledTest, editing.id ? "PUT" : "POST", {
            model_id: model, cron_expression: cronExpr.trim(), enabled: planEnabled, auto_recover: autoRecover,
          });
          setEditing(undefined);
          revise(value => value + 1);
        });
      }}>
        <Field label="测活模型"><Input value={model} onChange={event => setModel(event.target.value)} maxLength={200} placeholder="留空使用默认模型" /></Field>
        <Field label="执行周期 (Cron)"><CronField value={cronExpr} onChange={setCronExpr} /></Field>
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
