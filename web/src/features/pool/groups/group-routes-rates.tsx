import {useState} from "react";
import {Plus, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {compositeRouteSchema, empty} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {useModelCatalog} from "@/lib/model-catalog";
import {
  Button,
  DataTable,
  EmptyState,
  EntityLink,
  ErrorLine,
  Field,
  Form,
  FormActions,
  FormGrid,
  FormSection,
  InlineStack,
  Input,
  InsetRow,
  ModelSelect,
  Resource,
  Select,
  Stack,
  Switch,
  UserSelect,
} from "@/ui";
import {type CompositeRoute} from "@/features/pool/shared";

export function CompositeRoutesEditor({groupID, revision, onChanged}: {groupID: number; revision: number; onChanged: () => void}) {
  const candidates = useModelCatalog(`/admin/groups/${groupID}/model-candidates`, revision);
  const [localRev, setLocalRev] = useState(0);
  const [previewModel, setPreviewModel] = useState("");
  const [previewResult, setPreviewResult] = useState<string>("");
  const routes = useResource(
    () => api(`/admin/groups/${groupID}/composite-routes`, z.array(compositeRouteSchema)),
    `comp-routes-${groupID}`,
    true,
    revision + localRev,
  );
  const op = useAction();

  return (
    <>
      <FormSection title="路由规则在线命中预览">
        <Field label="输入模型名称实时测试路由决策">
          <div className="field-action-row">
            <ModelSelect label="路由预览模型" value={previewModel} onChange={setPreviewModel} placeholder="选择或输入模型" catalog={candidates} allowCustom />
            <Button
              size="sm"
              variant="outline"
              disabled={!previewModel.trim()}
              busy={op.busy}
              onClick={() => void op.run(async () => {
                const res = await api(
                  `/admin/groups/${groupID}/composite-routes/preview`,
                  z.object({matched: z.boolean(), target_platform: z.string(), upstream_model: z.string(), source: z.string(), reason: z.string().optional()}),
                  "POST",
                  {model: previewModel.trim(), endpoint: "any"},
                );
                setPreviewResult(
                  res.matched
                    ? `命中 ${res.target_platform} → ${res.upstream_model || previewModel.trim()}（来源：${res.source}）`
                    : `未命中路由${res.reason ? `：${res.reason}` : ""}`,
                );
              })}
            >
              测试命中
            </Button>
          </div>
        </Field>
        {previewResult && <InsetRow label="决策结果"><span className="code-mono text-fine">{previewResult}</span></InsetRow>}
      </FormSection>

      <FormSection title="新增模型转发规则">
        <FormSection title="规则参数">
          <Form
            busy={op.busy}
            onSubmit={e => {
              const fd = new FormData(e.currentTarget);
              const formEl = e.currentTarget;
              void op.run(async () => {
                await api(`/admin/groups/${groupID}/composite-routes`, compositeRouteSchema, "POST", {
                  public_model: String(fd.get("public_model") ?? "").trim(),
                  match_type: String(fd.get("match_type") ?? "prefix"),
                  target_platform: String(fd.get("target_platform") ?? "anthropic"),
                  upstream_model: String(fd.get("upstream_model") ?? "").trim(),
                  endpoint: String(fd.get("endpoint") ?? "any"),
                  priority: Number(fd.get("priority") || 10),
                  enabled: true,
                });
                formEl.reset();
                setLocalRev(v => v + 1);
                onChanged();
              });
            }}
          >
            <FormGrid>
              <Field label="匹配模型 / 前缀"><Input name="public_model" required placeholder="claude-" /></Field>
              <Field label="匹配方式">
                <Select name="match_type" defaultValue="prefix">
                  <option value="prefix">前缀匹配 (prefix)</option>
                  <option value="exact">精确匹配 (exact)</option>
                </Select>
              </Field>
              <Field label="目标上游平台">
                <Select name="target_platform" defaultValue="anthropic">
                  <option value="anthropic">Anthropic</option>
                  <option value="openai">OpenAI</option>
                  <option value="gemini">Gemini</option>
                  <option value="antigravity">Antigravity</option>
                  <option value="grok">Grok</option>
                  <option value="deepseek">DeepSeek</option>
                </Select>
              </Field>
              <Field label="改写上游模型（可选）"><Input name="upstream_model" placeholder="留空透传原名" /></Field>
              <Field label="适用端点">
                <Select name="endpoint" defaultValue="any">
                  <option value="any">全部端点 (any)</option>
                  <option value="messages">messages</option>
                  <option value="responses">responses</option>
                  <option value="chat_completions">chat_completions</option>
                </Select>
              </Field>
              <Field label="优先级"><Input name="priority" type="number" defaultValue="10" /></Field>
            </FormGrid>
            <FormActions>
              <Button type="submit" size="xs" busy={op.busy}><Plus aria-hidden />添加路由规则</Button>
            </FormActions>
          </Form>
        </FormSection>
      </FormSection>

      <Resource state={routes}>
        {rows => (
          <DataTable
            label="复合路由规则"
            rows={rows}
            rowID={r => r.id}
            empty={<EmptyState title="尚未添加复合路由规则" description="未匹配规则时将按模型名前缀自动推断目标平台。" />}
            columns={[
              {
                id: "rule",
                title: "匹配规则 → 目标平台",
                render: (r: CompositeRoute) => (
                  <div className="code-mono text-fine">
                    <strong>{r.public_model}</strong> ({r.match_type}) → {r.target_platform}
                    {r.upstream_model ? ` / ${r.upstream_model}` : ""}
                  </div>
                ),
              },
              {id: "prio", title: "优先级", align: "number", render: r => r.priority},
              {
                id: "enabled",
                title: "启用",
                align: "center",
                render: r => (
                  <Switch
                    label={`启用路由 ${r.public_model}`}
                    checked={r.enabled}
                    onChange={next => void op.run(async () => {
                      await api(`/admin/groups/${groupID}/composite-routes/${r.id}`, compositeRouteSchema, "PUT", {
                        public_model: r.public_model,
                        match_type: r.match_type,
                        target_platform: r.target_platform,
                        upstream_model: r.upstream_model,
                        endpoint: r.endpoint,
                        priority: r.priority,
                        enabled: next,
                        notes: r.notes,
                      });
                      setLocalRev(v => v + 1);
                      onChanged();
                    })}
                  />
                ),
              },
              {
                id: "del",
                title: "",
                align: "end",
                render: r => (
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    title="删除路由"
                    aria-label="删除路由"
                    onClick={() => void op.run(async () => {
                      await api(`/admin/groups/${groupID}/composite-routes/${r.id}`, empty, "DELETE");
                      setLocalRev(v => v + 1);
                    })}
                  >
                    <Trash2 />
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Resource>
      <ErrorLine value={op.error} />
    </>
  );
}

const groupRateRowSchema = z.object({
  user_id: z.number(),
  user_email: z.string(),
  rate_multiplier: z.number().nullable(),
  rpm_override: z.number().nullable().optional(),
});

export function GroupUserRatesEditor({groupID, revision, onChanged}: {groupID: number; revision: number; onChanged: () => void}) {
  const [localRev, setLocalRev] = useState(0);
  const [targetUser, setTargetUser] = useState("");
  const [rateVal, setRateVal] = useState("1");
  const [rpmVal, setRpmVal] = useState("");
  const rows = useResource(() => api(`/admin/groups/${groupID}/rate-multipliers`, z.array(groupRateRowSchema)), `grp-rates-${groupID}`, true, revision + localRev);
  const op = useAction();

  return (
    <>
      <FormSection title="设置单用户专属分组倍率与限流">
        <FormSection title="选择用户与覆写参数">
          <Stack>
            <Field label="搜索并选择用户…"><UserSelect value={targetUser} placeholder="搜索并选择用户…" onChange={v => setTargetUser(v)} /></Field>
            <InlineStack wrap>
              <Field label="专属倍率"><Input type="number" min="0" step="0.01" value={rateVal} onChange={e => setRateVal(e.target.value)} aria-label="专属倍率" /></Field>
              <Field label="RPM 覆写（可选）"><Input type="number" min="0" value={rpmVal} onChange={e => setRpmVal(e.target.value)} placeholder="留空沿用组配置" aria-label="RPM 覆写" /></Field>
              <Button
                size="sm"
                disabled={!targetUser}
                busy={op.busy}
                onClick={() => void op.run(async () => {
                  await api(`/admin/groups/${groupID}/rate-multipliers`, empty, "PUT", {
                    entries: [{
                      user_id: Number(targetUser),
                      rate_multiplier: Number(rateVal || 1),
                      ...(rpmVal.trim() !== "" ? {rpm_override: Number(rpmVal)} : {}),
                    }],
                  });
                  setTargetUser("");
                  setRpmVal("");
                  setLocalRev(v => v + 1);
                  onChanged();
                })}
              >
                应用倍率
              </Button>
              <Button
                size="sm"
                variant="outline"
                busy={op.busy}
                onClick={() => void op.run(async () => {
                  await api(`/admin/groups/${groupID}/rate-multipliers`, empty, "DELETE");
                  setLocalRev(v => v + 1);
                  onChanged();
                })}
              >
                清空全部覆写
              </Button>
            </InlineStack>
          </Stack>
        </FormSection>
      </FormSection>

      <Resource state={rows}>
        {items => (
          <DataTable
            label="组内专属倍率"
            rows={items}
            rowID={r => r.user_id}
            empty={<EmptyState title="暂无专属用户倍率" />}
            columns={[
              {id: "user", title: "用户", render: r => <EntityLink kind="user" id={r.user_id} label={r.user_email || `#${r.user_id}`} />},
              {id: "rate", title: "专属倍率", align: "number", render: r => r.rate_multiplier !== null ? `${r.rate_multiplier}×` : "默认"},
              {id: "rpm", title: "RPM 覆写", align: "number", render: r => r.rpm_override != null ? `${r.rpm_override}` : "默认"},
            ]}
          />
        )}
      </Resource>
      <ErrorLine value={op.error} />
    </>
  );
}
