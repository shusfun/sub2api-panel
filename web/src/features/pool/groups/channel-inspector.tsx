import {useRef, useState} from "react";
import {Pencil, Plus, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {channelRowSchema, empty} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {useModelCatalog} from "@/lib/model-catalog";
import {
  Button,
  CollapsibleSection,
  ConfirmDialog,
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
  MultiGroupSelect,
  Resource,
  Select,
  Stack,
  Switch,
} from "@/ui";
import {type ChannelRow, type PricingRule} from "@/features/pool/shared";

export function ChannelInspector({
  id,
  revision,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const detail = useResource(() => api(`/admin/channels/${id}`, channelRowSchema), `chn-detail-${id}`, true, revision);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const del = useAction();

  return (
    <Resource state={detail}>
      {ch => (
        <>
          <ChannelEditForm ch={ch} revision={revision} onSaved={onChanged} onRequestDelete={() => setConfirmDelete(true)} />
          <ConfirmDialog
            open={confirmDelete}
            title="删除调价模板"
            description={`确定删除调价模板「${ch.name}」吗？`}
            confirm="删除"
            busy={del.busy}
            error={del.error}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void del.run(async () => {
              await api(`/admin/channels/${ch.id}`, empty, "DELETE");
              setConfirmDelete(false);
              onDeleted();
            })}
          />
        </>
      )}
    </Resource>
  );
}

function toPerM(v?: number | null) {
  return v != null ? String(Number((v * 1_000_000).toFixed(6))) : "";
}

function fromPerM(s: string) {
  return s.trim() !== "" ? Number(s) / 1_000_000 : undefined;
}

function ChannelEditForm({
  ch,
  revision,
  onSaved,
  onRequestDelete,
}: {
  ch: ChannelRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(ch.name);
  const [desc, setDesc] = useState(ch.description);
  const [active, setActive] = useState(ch.status === "active");
  const [billingSource, setBillingSource] = useState(ch.billing_model_source || "requested");
  const [restrictModels, setRestrictModels] = useState(ch.restrict_models);
  const [applyToStats, setApplyToStats] = useState(ch.apply_pricing_to_account_stats);
  const [groupIDs, setGroupIDs] = useState<number[]>(ch.group_ids);
  const [pricing, setPricing] = useState<PricingRule[]>(ch.model_pricing);
  const [mappingRows, setMappingRows] = useState<Array<{platform: string; from: string; to: string}>>(() => {
    const out: Array<{platform: string; from: string; to: string}> = [];
    for (const [plat, map] of Object.entries(ch.model_mapping ?? {})) {
      for (const [from, to] of Object.entries(map)) out.push({platform: plat, from, to});
    }
    return out;
  });

  const [editIdx, setEditIdx] = useState<number | null>(null);
  const [draftPlatform, setDraftPlatform] = useState("openai");
  const [draftModel, setDraftModel] = useState("");
  const [draftMode, setDraftMode] = useState("token");
  const [draftIn, setDraftIn] = useState("");
  const [draftOut, setDraftOut] = useState("");
  const [draftCacheW, setDraftCacheW] = useState("");
  const [draftCacheR, setDraftCacheR] = useState("");
  const [draftImgIn, setDraftImgIn] = useState("");
  const [draftImgOut, setDraftImgOut] = useState("");
  const [draftPerReq, setDraftPerReq] = useState("");
  const [draftIntervalMin, setDraftIntervalMin] = useState("");
  const [draftIntervalIn, setDraftIntervalIn] = useState("");
  const [draftIntervalOut, setDraftIntervalOut] = useState("");
  const [draftTz, setDraftTz] = useState("");
  const [draftPeriodStart, setDraftPeriodStart] = useState("");
  const [draftPeriodEnd, setDraftPeriodEnd] = useState("");
  const [draftPeriodMult, setDraftPeriodMult] = useState("");

  const pricingModels = useModelCatalog(`/admin/channels/sync-models?platform=${encodeURIComponent(draftPlatform)}`, revision);
  const referencePrice = useAction();
  const priceSelection = useRef(0);

  const resetDraft = () => {
    setEditIdx(null);
    setDraftModel("");
    setDraftMode("token");
    setDraftIn("");
    setDraftOut("");
    setDraftCacheW("");
    setDraftCacheR("");
    setDraftImgIn("");
    setDraftImgOut("");
    setDraftPerReq("");
    setDraftIntervalMin("");
    setDraftIntervalIn("");
    setDraftIntervalOut("");
    setDraftTz("");
    setDraftPeriodStart("");
    setDraftPeriodEnd("");
    setDraftPeriodMult("");
  };

  const loadRuleIntoDraft = (idx: number) => {
    const r = pricing[idx];
    setEditIdx(idx);
    setDraftPlatform(r.platform);
    setDraftModel(r.models.join(", "));
    setDraftMode(r.billing_mode || "token");
    setDraftIn(toPerM(r.input_price));
    setDraftOut(toPerM(r.output_price));
    setDraftCacheW(toPerM(r.cache_write_price));
    setDraftCacheR(toPerM(r.cache_read_price));
    setDraftImgIn(toPerM(r.image_input_price));
    setDraftImgOut(toPerM(r.image_output_price));
    setDraftPerReq(r.per_request_price != null ? String(r.per_request_price) : "");
    const iv = r.intervals?.[0];
    setDraftIntervalMin(iv ? String(iv.min_tokens) : "");
    setDraftIntervalIn(toPerM(iv?.input_price));
    setDraftIntervalOut(toPerM(iv?.output_price));
    setDraftTz(r.time_pricing?.timezone ?? "");
    const tp = r.time_pricing?.periods?.[0];
    setDraftPeriodStart(tp?.start_time ?? "");
    setDraftPeriodEnd(tp?.end_time ?? "");
    setDraftPeriodMult(tp ? String(tp.multiplier) : "");
  };

  async function fillDefaultPrice() {
    if (!draftModel.trim()) return;
    const selection = priceSelection.current;
    const def = await api(
      `/admin/channels/model-pricing?model=${encodeURIComponent(draftModel.split(",")[0].trim())}`,
      z.object({found: z.boolean(), input_price: z.number().optional(), output_price: z.number().optional()}),
    );
    if (selection === priceSelection.current && def.found) {
      if (def.input_price !== undefined) setDraftIn(toPerM(def.input_price));
      if (def.output_price !== undefined) setDraftOut(toPerM(def.output_price));
    }
  }

  function commitRule() {
    const models = draftModel.split(",").map(s => s.trim()).filter(Boolean);
    if (!models.length) return;
    const rule: PricingRule = {
      platform: draftPlatform,
      models,
      billing_mode: draftMode,
      input_price: fromPerM(draftIn),
      output_price: fromPerM(draftOut),
      cache_write_price: fromPerM(draftCacheW),
      cache_read_price: fromPerM(draftCacheR),
      image_input_price: fromPerM(draftImgIn),
      image_output_price: fromPerM(draftImgOut),
      per_request_price: draftPerReq.trim() !== "" ? Number(draftPerReq) : undefined,
      ...(draftIntervalMin.trim() !== "" ? {
        intervals: [{
          min_tokens: Number(draftIntervalMin),
          input_price: fromPerM(draftIntervalIn),
          output_price: fromPerM(draftIntervalOut),
          sort_order: 0,
        }],
      } : {}),
      ...(draftTz.trim() && draftPeriodStart.trim() && draftPeriodEnd.trim() ? {
        time_pricing: {
          timezone: draftTz.trim(),
          periods: [{start_time: draftPeriodStart.trim(), end_time: draftPeriodEnd.trim(), multiplier: Number(draftPeriodMult || 1)}],
        },
      } : {}),
    };
    if (editIdx !== null) {
      setPricing(pricing.map((item, i) => (i === editIdx ? rule : item)));
    } else {
      setPricing([...pricing, rule]);
    }
    resetDraft();
  }

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        const modelMapping: Record<string, Record<string, string>> = {};
        for (const row of mappingRows) {
          if (row.platform.trim() && row.from.trim() && row.to.trim()) {
            modelMapping[row.platform.trim()] ??= {};
            modelMapping[row.platform.trim()][row.from.trim()] = row.to.trim();
          }
        }
        await api(`/admin/channels/${ch.id}`, channelRowSchema, "PUT", {
          name: name.trim(),
          description: desc,
          status: active ? "active" : "disabled",
          billing_model_source: billingSource,
          restrict_models: restrictModels,
          apply_pricing_to_account_stats: applyToStats,
          group_ids: groupIDs,
          model_pricing: pricing,
          model_mapping: modelMapping,
        });
        onSaved();
      })}
    >
      <FormSection title="调价模板范围与策略">
        <InsetRow label="启用模板"><Switch label="启用调价模板" checked={active} onChange={setActive} /></InsetRow>
        <Field label="模板名称"><Input required value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="计费模型名称基准">
          <Select value={billingSource} onChange={e => setBillingSource(e.target.value)}>
            <option value="requested">请求模型名 (requested)</option>
            <option value="upstream">上游模型名 (upstream)</option>
            <option value="response_model">响应模型名 (response_model)</option>
          </Select>
        </Field>
        <InsetRow label="限制仅允许已定价模型"><Switch label="限制模型" checked={restrictModels} onChange={setRestrictModels} /></InsetRow>
        <InsetRow label="将调价同步至账号统计成本"><Switch label="应用到账号统计" checked={applyToStats} onChange={setApplyToStats} /></InsetRow>
        <Field label="作用分组"><MultiGroupSelect value={groupIDs} revision={revision} onChange={setGroupIDs} /></Field>
        <Field label="说明"><Input value={desc} onChange={e => setDesc(e.target.value)} /></Field>
      </FormSection>

      <FormSection title="模型价目规则 (USD / 1M Token)">
        {pricing.map((rule, idx) => (
          <InsetRow
            key={idx}
            label={`${rule.platform}: ${rule.models.join(", ")}`}
            hint={`入 $${((rule.input_price ?? 0) * 1_000_000).toFixed(2)}/M · 出 $${((rule.output_price ?? 0) * 1_000_000).toFixed(2)}/M${rule.per_request_price ? ` · 按次 $${rule.per_request_price}` : ""}`}
          >
            <InlineStack density="compact">
              <Button type="button" size="icon-xs" variant="ghost" aria-label="编辑价目" onClick={() => loadRuleIntoDraft(idx)}><Pencil /></Button>
              <Button type="button" size="icon-xs" variant="ghost" aria-label="移除价目" onClick={() => setPricing(pricing.filter((_, i) => i !== idx))}><Trash2 /></Button>
            </InlineStack>
          </InsetRow>
        ))}

        <FormSection title={editIdx !== null ? "编辑模型价目规则" : "新增模型价目规则"}>
          <Stack>
            <FormGrid cols={3}>
              <Field label="平台">
                <Select value={draftPlatform} onChange={e => { setDraftPlatform(e.target.value); setDraftModel(""); }}>
                  <option value="openai">OpenAI</option>
                  <option value="anthropic">Anthropic</option>
                  <option value="gemini">Gemini</option>
                  <option value="antigravity">Antigravity</option>
                  <option value="grok">Grok</option>
                  <option value="deepseek">DeepSeek</option>
                </Select>
              </Field>
              <Field label="模型">
                <ModelSelect label="定价模型" value={draftModel} onChange={setDraftModel} placeholder="选择或输入模型" catalog={pricingModels} allowCustom />
              </Field>
              <Field label="计费模式">
                <Select value={draftMode} onChange={e => setDraftMode(e.target.value)}>
                  <option value="token">按 Token</option>
                  <option value="per_request">按请求次数</option>
                </Select>
              </Field>
            </FormGrid>
            <InlineStack>
              <Button type="button" size="xs" variant="outline" disabled={!draftModel} busy={referencePrice.busy} onClick={() => void referencePrice.run(fillDefaultPrice)}>读取参考价</Button>
              {editIdx !== null && <Button type="button" size="xs" variant="ghost" onClick={resetDraft}>取消编辑</Button>}
            </InlineStack>
            <ErrorLine value={referencePrice.error} />
            <FormGrid cols={3}>
              <Field label="输入 $/1M"><Input type="number" min="0" step="0.01" value={draftIn} onChange={e => setDraftIn(e.target.value)} /></Field>
              <Field label="输出 $/1M"><Input type="number" min="0" step="0.01" value={draftOut} onChange={e => setDraftOut(e.target.value)} /></Field>
              <Field label="按次单价 $"><Input type="number" min="0" step="0.001" value={draftPerReq} onChange={e => setDraftPerReq(e.target.value)} /></Field>
              <Field label="缓存写 $/1M"><Input type="number" min="0" step="0.01" value={draftCacheW} onChange={e => setDraftCacheW(e.target.value)} /></Field>
              <Field label="缓存读 $/1M"><Input type="number" min="0" step="0.01" value={draftCacheR} onChange={e => setDraftCacheR(e.target.value)} /></Field>
              <Field label="图像入 $/1M"><Input type="number" min="0" step="0.01" value={draftImgIn} onChange={e => setDraftImgIn(e.target.value)} /></Field>
              <Field label="图像出 $/1M"><Input type="number" min="0" step="0.01" value={draftImgOut} onChange={e => setDraftImgOut(e.target.value)} /></Field>
            </FormGrid>
            <CollapsibleSection title="阶梯区间 (intervals) 与分时倍率 (time_pricing)" summary="可选高级定价">
              <FormGrid cols={3}>
                <Field label="阶梯起始 Tokens"><Input type="number" min="0" value={draftIntervalMin} onChange={e => setDraftIntervalMin(e.target.value)} placeholder="如 128000" /></Field>
                <Field label="阶梯输入 $/1M"><Input type="number" min="0" step="0.01" value={draftIntervalIn} onChange={e => setDraftIntervalIn(e.target.value)} /></Field>
                <Field label="阶梯输出 $/1M"><Input type="number" min="0" step="0.01" value={draftIntervalOut} onChange={e => setDraftIntervalOut(e.target.value)} /></Field>
                <Field label="时区"><Input value={draftTz} onChange={e => setDraftTz(e.target.value)} placeholder="Asia/Shanghai" /></Field>
                <Field label="时段 (HH:MM - HH:MM)">
                  <InlineStack density="compact">
                    <Input value={draftPeriodStart} onChange={e => setDraftPeriodStart(e.target.value)} placeholder="00:00" />
                    <Input value={draftPeriodEnd} onChange={e => setDraftPeriodEnd(e.target.value)} placeholder="08:00" />
                  </InlineStack>
                </Field>
                <Field label="时段倍率"><Input type="number" min="0" step="0.01" value={draftPeriodMult} onChange={e => setDraftPeriodMult(e.target.value)} placeholder="0.5" /></Field>
              </FormGrid>
            </CollapsibleSection>
            <Button type="button" size="xs" variant="outline" disabled={!draftModel.trim()} onClick={commitRule}>
              {editIdx !== null ? "更新该规则" : "加入规则列表"}
            </Button>
          </Stack>
        </FormSection>
      </FormSection>

      <CollapsibleSection title="渠道级模型映射 (model_mapping)" summary={`${mappingRows.length} 条`}>
        <Stack>
          {mappingRows.map((row, idx) => (
            <FormGrid key={idx} cols={3}>
              <Input value={row.platform} onChange={e => setMappingRows(mappingRows.map((r, i) => i === idx ? {...r, platform: e.target.value} : r))} placeholder="平台" />
              <Input value={row.from} onChange={e => setMappingRows(mappingRows.map((r, i) => i === idx ? {...r, from: e.target.value} : r))} placeholder="请求模型" />
              <InlineStack density="compact">
                <Input value={row.to} onChange={e => setMappingRows(mappingRows.map((r, i) => i === idx ? {...r, to: e.target.value} : r))} placeholder="目标模型" />
                <Button type="button" size="icon-xs" variant="ghost" aria-label="删除映射" onClick={() => setMappingRows(mappingRows.filter((_, i) => i !== idx))}><Trash2 /></Button>
              </InlineStack>
            </FormGrid>
          ))}
          <Button type="button" size="xs" variant="outline" onClick={() => setMappingRows([...mappingRows, {platform: "openai", from: "", to: ""}])}>
            <Plus aria-hidden />新增映射
          </Button>
        </Stack>
      </CollapsibleSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除模板</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存调价模板</Button>
      </FormActions>
    </Form>
  );
}
