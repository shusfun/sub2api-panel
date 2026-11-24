import {useEffect, useMemo, useState} from "react";
import {KeyRound, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction, useResource} from "@/lib/requests";
import {empty, patrolPolicySchema, probeTemplateSchema} from "@/schemas";
import {
  Button, EmptyState, ErrorLine, Feedback, Field, Form, FormGrid, FormSection, GroupSelect,
  InlineStack, Input, InsetGroup, InsetRow, Overlay, PasswordInput, Resource, Segmented, Select,
  Stack, Switch, Textarea,
} from "@/ui";
import {groupBriefSchema} from "@/features/pool/shared";

type PatrolPolicy = z.infer<typeof patrolPolicySchema>;
type ProbeTemplate = z.infer<typeof probeTemplateSchema>;

export function PatrolPolicyDialog({
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
              onClick={() => void delAct.run(async () => {
                await api(`/admin/patrol/policies/${target}`, empty, "DELETE");
                onSaved();
              })}
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
            <Field label="调度分组"><GroupSelect value={groupID} onChange={setGroupID} clearable={false} /></Field>
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
                  {(templates?.value ?? []).map((t: ProbeTemplate) => <option key={t.id} value={String(t.id)}>{t.name}</option>)}
                </Select>
              </Field>
            )}
            <FormGrid>
              <Field label="检测模型"><Input value={model} onChange={e => setModel(e.target.value)} placeholder="gpt-5.4" required /></Field>
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
              <Field label="流量观测窗口（分钟）"><Input type="number" min={1} max={1440} value={windowMin} onChange={e => setWindowMin(e.target.value)} /></Field>
              <Field label="唤醒所需最少活跃用户数"><Input type="number" min={0} max={10000} value={minUsers} onChange={e => setMinUsers(e.target.value)} /></Field>
            </FormGrid>
            <FormGrid>
              <Field label="或最少窗口请求数"><Input type="number" min={0} max={1000000} value={minReqs} onChange={e => setMinReqs(e.target.value)} /></Field>
              <Field label="常态巡检间隔（分钟）"><Input type="number" min={1} max={1440} value={intervalMin} onChange={e => setIntervalMin(e.target.value)} /></Field>
            </FormGrid>
            <FormGrid>
              <Field label="高峰用户数 / 高峰间隔（分钟）">
                <InlineStack>
                  <Input type="number" min={1} value={peakUsers} onChange={e => setPeakUsers(e.target.value)} aria-label="高峰活跃用户阈值" />
                  <Input type="number" min={1} value={peakIntervalMin} onChange={e => setPeakIntervalMin(e.target.value)} aria-label="高峰间隔分钟" />
                </InlineStack>
              </Field>
              <Field label="异常快速复核间隔（分钟）"><Input type="number" min={1} max={240} value={recheckMin} onChange={e => setRecheckMin(e.target.value)} /></Field>
            </FormGrid>
            <FormGrid>
              <Field label="单日最高 Token 熔断"><Input type="number" min={0} value={dailyTokens} onChange={e => setDailyTokens(e.target.value)} /></Field>
              <Field label="单日最高预算（USD）"><Input type="number" min={0} step="0.1" value={dailyUSD} onChange={e => setDailyUSD(e.target.value)} /></Field>
            </FormGrid>
          </FormSection>

          <InsetGroup title="无人值守巡检凭据与可见性">
            <InsetRow label="启用定时巡检"><Switch label="启用定时巡检" checked={enabled} onChange={setEnabled} /></InsetRow>
            <InsetRow label="在公开首页及用户端展示此分组"><Switch label="公开展示此分组" checked={publicVisible} onChange={setPublicVisible} /></InsetRow>
            <InsetRow label="巡检专用受限 Key" hint={existing?.has_patrol_key ? "已绑定本组限额巡检密钥" : "保存策略后可一键签发绑定本组且带日限额的专用密钥"}>
              {typeof target === "number" && (
                <Button
                  size="xs"
                  variant="outline"
                  busy={mintAct.busy}
                  onClick={() => void mintAct.run(async () => {
                    await api(`/admin/patrol/policies/${target}/mint-key`, patrolPolicySchema, "POST", {});
                    setMintNotice("已自动创建并绑定本组专用日限额巡检 Key");
                  })}
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

export function ProbeTemplatesDialog({open, onClose}: {open: boolean; onClose: () => void}) {
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
                      onClick={() => void act.run(async () => {
                        await api(`/admin/patrol/templates/${item.id}`, empty, "DELETE");
                        bump(v => v + 1);
                      })}
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
              <Field label="模板名称"><Input value={name} onChange={e => setName(e.target.value)} placeholder="如：多步逻辑推理题 v2" required /></Field>
              <Field label="协议">
                <Select value={protocol} onChange={e => setProtocol(e.target.value)}>
                  <option value="chat_completions">Chat Completions</option>
                  <option value="responses">Responses</option>
                  <option value="messages">Messages</option>
                </Select>
              </Field>
            </FormGrid>
            <Field label="System 提示词（可选）"><Input value={sysPrompt} onChange={e => setSysPrompt(e.target.value)} placeholder="仅输出最终结果数字，不要解释" /></Field>
            <Field label="测试题目 / User Prompt"><Textarea value={userPrompt} onChange={e => setUserPrompt(e.target.value)} required /></Field>
            <FormGrid>
              <Field label="判定方式">
                <Select value={matchKind} onChange={e => setMatchKind(e.target.value)}>
                  <option value="exact_number">匹配独立数字</option>
                  <option value="contains">包含指定文本</option>
                  <option value="regex">正则表达式匹配</option>
                  <option value="status_ok">仅校验流式正常返回</option>
                </Select>
              </Field>
              <Field label="期望命中值"><Input value={expected} onChange={e => setExpected(e.target.value)} placeholder="例如：21" /></Field>
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
