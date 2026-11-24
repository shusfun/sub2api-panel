import {useState} from "react";
import {Copy as DuplicateIcon} from "lucide-react";
import {api} from "@/lib/http";
import {adminGroupRow, empty} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {useModelCatalog} from "@/lib/model-catalog";
import {
  Button,
  CollapsibleSection,
  ConfirmDialog,
  ErrorLine,
  Feedback,
  Field,
  Form,
  FormActions,
  FormGrid,
  FormSection,
  GroupSelect,
  InlineStack,
  Input,
  InsetRow,
  MultiModelSelect,
  Resource,
  Segmented,
  Switch,
} from "@/ui";
import {type GroupRow} from "@/features/pool/shared";
import {CompositeRoutesEditor, GroupUserRatesEditor} from "./group-routes-rates";

export function GroupInspector({
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
  const [tab, setTab] = useState<"core" | "composite" | "rates">("core");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [hint, setHint] = useState("");
  const detail = useResource(() => api(`/admin/groups/${id}`, adminGroupRow), `grp-detail-${id}`, true, revision);
  const dup = useAction();
  const del = useAction();

  return (
    <Resource state={detail}>
      {grp => (
        <>
          <InlineStack justify="between" wrap>
            <Segmented
              label="分组详情分页"
              value={tab}
              options={[
                ["core", "核心与白名单"],
                ...(grp.platform === "composite" ? ([["composite", "复合路由"]] as const) : []),
                ["rates", "专属倍率"],
              ]}
              onChange={v => setTab(v as "core" | "composite" | "rates")}
            />
            <Button
              size="icon-xs"
              variant="outline"
              title="复制分组"
              aria-label="复制分组"
              busy={dup.busy}
              onClick={() => void dup.run(async () => {
                await api(`/admin/groups/${grp.id}/duplicate`, adminGroupRow, "POST", {operation_id: crypto.randomUUID()});
                setHint("已复制生成新分组");
                onChanged();
              })}
            >
              <DuplicateIcon />
            </Button>
          </InlineStack>

          {hint && <Feedback tone="success">{hint}</Feedback>}
          <ErrorLine value={dup.error} />

          {tab === "core" && (
            <GroupCoreForm
              grp={grp}
              revision={revision}
              onSaved={() => { setHint("分组配置已保存"); onChanged(); }}
              onRequestDelete={() => setConfirmDelete(true)}
            />
          )}

          {tab === "composite" && <CompositeRoutesEditor groupID={grp.id} revision={revision} onChanged={onChanged} />}

          {tab === "rates" && <GroupUserRatesEditor groupID={grp.id} revision={revision} onChanged={onChanged} />}

          <ConfirmDialog
            open={confirmDelete}
            title="删除调度分组"
            description={`确定删除分组「${grp.name}」吗？`}
            confirm="删除"
            busy={del.busy}
            error={del.error}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void del.run(async () => {
              await api(`/admin/groups/${grp.id}`, empty, "DELETE");
              setConfirmDelete(false);
              onDeleted();
            })}
          />
        </>
      )}
    </Resource>
  );
}

function GroupCoreForm({
  grp,
  revision,
  onSaved,
  onRequestDelete,
}: {
  grp: GroupRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(grp.name);
  const [desc, setDesc] = useState(grp.description);
  const [rate, setRate] = useState(String(grp.rate_multiplier));
  const [rpm, setRpm] = useState(String(grp.rpm_limit));
  const [subType, setSubType] = useState(grp.subscription_type);
  const [exclusive, setExclusive] = useState(grp.is_exclusive);
  const [active, setActive] = useState(grp.status === "active");
  const [fallbackID, setFallbackID] = useState(grp.fallback_group_id ? String(grp.fallback_group_id) : "");
  const [allowlistOn, setAllowlistOn] = useState(grp.model_allowlist.enabled);
  const [models, setModels] = useState<string[]>(grp.model_allowlist.models);
  const [dailyLimit, setDailyLimit] = useState(grp.daily_limit_usd !== null ? String(grp.daily_limit_usd) : "");
  const [weeklyLimit, setWeeklyLimit] = useState(grp.weekly_limit_usd !== null ? String(grp.weekly_limit_usd) : "");
  const [monthlyLimit, setMonthlyLimit] = useState(grp.monthly_limit_usd !== null ? String(grp.monthly_limit_usd) : "");
  const [peakOn, setPeakOn] = useState(grp.peak_rate_enabled);
  const [peakStart, setPeakStart] = useState(grp.peak_start || "09:00");
  const [peakEnd, setPeakEnd] = useState(grp.peak_end || "23:00");
  const [peakRate, setPeakRate] = useState(String(grp.peak_rate_multiplier || 1));

  const candidates = useModelCatalog(`/admin/groups/${grp.id}/model-candidates?platform=${encodeURIComponent(grp.platform)}`, revision, allowlistOn);

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/admin/groups/${grp.id}`, adminGroupRow, "PUT", {
          name: name.trim(),
          description: desc,
          status: active ? "active" : "inactive",
          is_exclusive: exclusive,
          subscription_type: subType,
          rate_multiplier: Number(rate || 1),
          rpm_limit: Number(rpm || 0),
          ...(fallbackID ? {fallback_group_id: Number(fallbackID)} : {clear_fallback_group: true}),
          model_allowlist: {enabled: allowlistOn, models},
          ...(subType === "subscription" ? {
            ...(dailyLimit ? {daily_limit_usd: Number(dailyLimit)} : {clear_daily_limit: true}),
            ...(weeklyLimit ? {weekly_limit_usd: Number(weeklyLimit)} : {clear_weekly_limit: true}),
            ...(monthlyLimit ? {monthly_limit_usd: Number(monthlyLimit)} : {clear_monthly_limit: true}),
          } : {}),
          peak_rate_enabled: peakOn,
          ...(peakOn ? {peak_start: peakStart, peak_end: peakEnd, peak_rate_multiplier: Number(peakRate || 1)} : {}),
        });
        onSaved();
      })}
    >
      <FormSection title="核心计费与准入">
        <InsetRow label="启用分组调度"><Switch label="启用分组" checked={active} onChange={setActive} /></InsetRow>
        <Field label="分组名称"><Input required value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="计费模式">
          <Segmented
            label="计费模式"
            value={subType === "subscription" ? "subscription" : "standard"}
            options={[["standard", "按量余额"], ["subscription", "套餐订阅"]] as const}
            onChange={setSubType}
          />
        </Field>
        <Field label="基础计费倍率">
          <div className="row-field"><Input type="number" min="0" step="0.01" value={rate} onChange={e => setRate(e.target.value)} /></div>
        </Field>
        <InsetRow label="专属分组（仅授权用户可见）">
          <Switch label="专属分组" checked={exclusive} onChange={setExclusive} />
        </InsetRow>
        <Field label="故障兜底分组">
          <GroupSelect value={fallbackID} placeholder="不配置兜底" onChange={setFallbackID} />
        </Field>
      </FormSection>

      <FormSection title="模型访问白名单">
        <InsetRow label="启用模型白名单过滤">
          <Switch label="启用模型白名单" checked={allowlistOn} onChange={setAllowlistOn} />
        </InsetRow>
        {allowlistOn && (
          <Field label="允许调用的模型列表">
            <MultiModelSelect label="模型白名单" placeholder="添加模型" value={models} catalog={candidates} onChange={setModels} allowCustom />
          </Field>
        )}
      </FormSection>

      <CollapsibleSection title="订阅额度、RPM 与错峰倍率策略" summary={subType === "subscription" ? "订阅限额生效中" : "高级策略"}>
        <FormGrid>
          <Field label="组级 RPM 限制"><Input type="number" min="0" value={rpm} onChange={e => setRpm(e.target.value)} /></Field>
          {subType === "subscription" && <>
            <Field label="日限额 USD"><Input type="number" min="0" step="0.01" value={dailyLimit} onChange={e => setDailyLimit(e.target.value)} placeholder="不限" /></Field>
            <Field label="周限额 USD"><Input type="number" min="0" step="0.01" value={weeklyLimit} onChange={e => setWeeklyLimit(e.target.value)} placeholder="不限" /></Field>
            <Field label="月限额 USD"><Input type="number" min="0" step="0.01" value={monthlyLimit} onChange={e => setMonthlyLimit(e.target.value)} placeholder="不限" /></Field>
          </>}
        </FormGrid>
        <InlineStack justify="between" wrap>
          <span className="text-meta">启用高峰时段动态倍率</span>
          <Switch label="启用高峰时段倍率" checked={peakOn} onChange={setPeakOn} />
        </InlineStack>
        {peakOn && (
          <FormGrid cols={3}>
            <Field label="开始 (HH:MM)"><Input value={peakStart} onChange={e => setPeakStart(e.target.value)} /></Field>
            <Field label="结束 (HH:MM)"><Input value={peakEnd} onChange={e => setPeakEnd(e.target.value)} /></Field>
            <Field label="高峰倍率"><Input type="number" min="0" step="0.01" value={peakRate} onChange={e => setPeakRate(e.target.value)} /></Field>
          </FormGrid>
        )}
        <Field label="分组说明"><Input value={desc} onChange={e => setDesc(e.target.value)} /></Field>
      </CollapsibleSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除分组</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存分组</Button>
      </FormActions>
    </Form>
  );
}
