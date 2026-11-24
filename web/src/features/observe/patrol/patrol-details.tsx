import {useState} from "react";
import {api} from "@/lib/http";
import {count, date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {patrolRunDetailSchema, probeReportSchema, type ProbeReport} from "@/schemas";
import {
  Button, CopyButton, EmptyState, ErrorLine, Field, Form, FormGrid, FormSection, InlineStack,
  Input, InsetGroup, InsetRow, Overlay, PasswordInput, Resource, Select, Stack,
} from "@/ui";
import {engineLabel, verdictBadge} from "./patrol-board";

export function PatrolRunDetailDialog({
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
                  <div className="patrol-metric-cell"><span>测试模型</span><strong>{run.model || "默认"}</strong></div>
                  <div className="patrol-metric-cell"><span>检测引擎</span><strong>{engineLabel(run.engine)}</strong></div>
                  <div className="patrol-metric-cell"><span>总耗时 / 首字</span><strong>{totalSec.toFixed(1)}s · {run.ttft_ms > 0 ? `${run.ttft_ms}ms` : "未知"}</strong></div>
                  <div className="patrol-metric-cell"><span>输入 Token</span><strong>{count.format(run.input_tokens)}</strong></div>
                  <div className="patrol-metric-cell"><span>输出 / 推理 Token</span><strong>{count.format(run.output_tokens)} / {count.format(run.reasoning_tokens)}</strong></div>
                  <div className="patrol-metric-cell"><span>缓存输入 Token</span><strong>{count.format(run.cached_tokens)}</strong></div>
                  <div className="patrol-metric-cell"><span>平均 TPS</span><strong>{tps}</strong></div>
                  <div className="patrol-metric-cell"><span>预估费用 · USD</span><strong>${usd}</strong></div>
                  <div className="patrol-metric-cell"><span>窗口活跃用户</span><strong>{count.format(run.active_users)} 人（{count.format(run.window_requests)} 次请求）</strong></div>
                </div>

                {run.predicted_model && (
                  <InsetGroup>
                    <InsetRow label="指纹最接近模型"><span>{run.predicted_model}</span></InsetRow>
                    {run.confidence !== null && run.confidence !== undefined && (
                      <InsetRow label="参考库校准置信度"><span className="weir-digits">{`${(run.confidence * 100).toFixed(1)}%`}</span></InsetRow>
                    )}
                  </InsetGroup>
                )}

                {run.reply_excerpt && <div className="patrol-reply-box">{run.reply_excerpt}</div>}
              </Stack>
            );
          }}
        </Resource>
      )}
    </Overlay>
  );
}

export function GuestSelfCheckSection({defaultModel}: {defaultModel: string}) {
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
                <Field label="网关地址 (HTTPS)"><Input value={baseURL} onChange={e => setBaseURL(e.target.value)} placeholder="https://api.example.com" required /></Field>
                <Field label="模型名称"><Input value={model} onChange={e => setModel(e.target.value)} placeholder="gpt-5.4" required /></Field>
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
