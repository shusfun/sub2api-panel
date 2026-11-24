import {useRef, useState} from "react";
import {Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {channelRowSchema, empty} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {useModelCatalog} from "@/lib/model-catalog";
import {
  Button,
  ConfirmDialog,
  ErrorLine,
  Field,
  Form,
  FormActions,
  FormSection,
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
  const [groupIDs, setGroupIDs] = useState<number[]>(ch.group_ids);
  const [pricing, setPricing] = useState<PricingRule[]>(ch.model_pricing);
  const [draftModel, setDraftModel] = useState("");
  const [draftPlatform, setDraftPlatform] = useState("openai");
  const [draftIn, setDraftIn] = useState("");
  const [draftOut, setDraftOut] = useState("");
  const pricingModels = useModelCatalog(`/admin/channels/sync-models?platform=${encodeURIComponent(draftPlatform)}`, revision);
  const referencePrice = useAction();
  const priceSelection = useRef(0);

  const changePricingModel = (model: string) => {
    priceSelection.current++;
    referencePrice.clearError();
    setDraftModel(model);
    setDraftIn("");
    setDraftOut("");
  };

  async function fillDefaultPrice() {
    if (!draftModel.trim()) return;
    const selection = priceSelection.current;
    const def = await api(
      `/admin/channels/model-pricing?model=${encodeURIComponent(draftModel.trim())}`,
      z.object({found: z.boolean(), input_price: z.number().optional(), output_price: z.number().optional()}),
    );
    if (selection === priceSelection.current && def.found) {
      if (def.input_price !== undefined) setDraftIn(String(def.input_price * 1_000_000));
      if (def.output_price !== undefined) setDraftOut(String(def.output_price * 1_000_000));
    }
  }

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/admin/channels/${ch.id}`, channelRowSchema, "PUT", {
          name: name.trim(),
          description: desc,
          status: active ? "active" : "disabled",
          group_ids: groupIDs,
          model_pricing: pricing,
        });
        onSaved();
      })}
    >
      <FormSection title="调价模板范围">
        <InsetRow label="启用模板"><Switch label="启用调价模板" checked={active} onChange={setActive} /></InsetRow>
        <Field label="模板名称"><Input required value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="作用分组">
          <MultiGroupSelect value={groupIDs} revision={revision} onChange={setGroupIDs} />
        </Field>
        <Field label="说明"><Input value={desc} onChange={e => setDesc(e.target.value)} /></Field>
      </FormSection>

      <FormSection title="按百万 Token 模型单价覆写 (USD / 1M)">
        {pricing.map((rule, idx) => (
          <InsetRow
            key={idx}
            label={rule.models.join(", ")}
            hint={`入 $${((rule.input_price ?? 0) * 1_000_000).toFixed(2)}/M · 出 $${((rule.output_price ?? 0) * 1_000_000).toFixed(2)}/M`}
          >
            <Button size="icon-xs" variant="ghost" title="移除价目" aria-label="移除价目" onClick={() => setPricing(pricing.filter((_, i) => i !== idx))}>
              <Trash2 />
            </Button>
          </InsetRow>
        ))}
        <FormSection title="新增模型价目规则">
          <Stack>
            <div className="field-action-row">
              <Field label="模型平台"><Select value={draftPlatform} onChange={e => { setDraftPlatform(e.target.value); changePricingModel(""); }} aria-label="模型平台">
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="gemini">Gemini</option>
                <option value="grok">Grok</option>
                <option value="deepseek">DeepSeek</option>
              </Select></Field>
              <Field label="选择或输入模型"><ModelSelect label="定价模型" value={draftModel} onChange={changePricingModel} placeholder="选择或输入模型" catalog={pricingModels} allowCustom /></Field>
              <Button type="button" variant="outline" disabled={!draftModel} busy={referencePrice.busy} onClick={() => void referencePrice.run(fillDefaultPrice)}>读取参考价</Button>
            </div>
            <ErrorLine value={referencePrice.error} />
            <div className="field-action-row">
              <Field label="输入 $/1M"><Input type="number" min="0" step="0.01" value={draftIn} onChange={e => setDraftIn(e.target.value)} placeholder="输入 $/1M" /></Field>
              <Field label="输出 $/1M"><Input type="number" min="0" step="0.01" value={draftOut} onChange={e => setDraftOut(e.target.value)} placeholder="输出 $/1M" /></Field>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!draftModel.trim()}
                onClick={() => {
                  setPricing([
                    ...pricing,
                    {
                      platform: draftPlatform,
                      models: [draftModel.trim()],
                      billing_mode: "token",
                      input_price: Number(draftIn || 0) / 1_000_000,
                      output_price: Number(draftOut || 0) / 1_000_000,
                    },
                  ]);
                  changePricingModel("");
                }}
              >
                加入列表
              </Button>
            </div>
          </Stack>
        </FormSection>
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除模板</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存调价模板</Button>
      </FormActions>
    </Form>
  );
}
