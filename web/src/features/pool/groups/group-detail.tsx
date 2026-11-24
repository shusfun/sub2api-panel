import {useState} from "react";
import {Copy as DuplicateIcon, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {adminGroupRow, empty} from "@/schemas";
import {count, smartMoney} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {useModelCatalog} from "@/lib/model-catalog";
import {
  Badge,
  Button,
  ConfirmDialog,
  DetailHeader,
  EmptyState,
  EntityLink,
  ErrorLine,
  Feedback,
  Field,
  Form,
  FormActions,
  FormGrid,
  FormSection,
  InlineStack,
  Input,
  InsetRow,
  MetricStrip,
  ModelSelect,
  MultiModelSelect,
  PlatformBadge,
  RemoteTable,
  Resource,
  Stack,
  Status,
  Switch,
  Tabs,
  type Column,
} from "@/ui";
import {type GroupRow, type PricingRule} from "@/features/pool/shared";
import {GroupCoreForm} from "./group-inspector";
import {CompositeRoutesEditor, GroupUserRatesEditor} from "./group-routes-rates";

const groupStatsSchema = z.object({
  total_api_keys: z.number(),
  active_api_keys: z.number(),
  total_requests: z.number(),
  total_cost: z.number(),
});

const groupKeyRowSchema = z.object({
  id: z.number(),
  name: z.string(),
  masked: z.string(),
  status: z.string(),
  user_id: z.number(),
  user_email: z.string().optional().default(""),
  quota: z.number(),
  quota_used: z.number(),
});

type GroupKeyRow = z.infer<typeof groupKeyRowSchema>;

export function GroupDetailWorkspace({
  id,
  revision,
  onBack,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  onBack: () => void;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const [tab, setTab] = useState("core");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [hint, setHint] = useState("");
  const detail = useResource(() => api(`/admin/groups/${id}`, adminGroupRow), `grp-detail-${id}`, true, revision);
  const dup = useAction();
  const del = useAction();

  return (
    <div className="workspace-content">
      <Resource state={detail}>
        {grp => (
          <Stack>
            <DetailHeader
              title={grp.name}
              backLabel="返回分组列表"
              onBack={onBack}
              badges={
                <InlineStack density="compact">
                  <PlatformBadge platform={grp.platform} />
                  <Status value={grp.status} />
                  {grp.is_exclusive && <Badge variant="tag">私有</Badge>}
                </InlineStack>
              }
              actions={
                <Button
                  size="sm"
                  variant="outline"
                  busy={dup.busy}
                  onClick={() => void dup.run(async () => {
                    await api(`/admin/groups/${grp.id}/duplicate`, adminGroupRow, "POST", {operation_id: crypto.randomUUID()});
                    setHint("已复制生成新分组");
                    onChanged();
                  })}
                >
                  <DuplicateIcon aria-hidden />复制分组
                </Button>
              }
            />

            {hint && <Feedback tone="success">{hint}</Feedback>}
            <ErrorLine value={dup.error} />

            <Tabs
              label="分组详情分页"
              value={tab}
              onChange={setTab}
              items={[
                {value: "core", label: "核心与调度策略"},
                {value: "models", label: "白名单与模型定价"},
                {value: "composite", label: "复合路由"},
                {value: "rates", label: "专属倍率与限流"},
                {value: "keys", label: "组内密钥与统计"},
              ]}
            />

            {tab === "core" && (
              <GroupCoreForm
                grp={grp}
                revision={revision}
                onSaved={() => { setHint("分组配置已保存"); onChanged(); }}
                onRequestDelete={() => setConfirmDelete(true)}
              />
            )}

            {tab === "models" && (
              <GroupModelsPricingTab
                grp={grp}
                revision={revision}
                onSaved={() => { setHint("白名单与定价已保存"); onChanged(); }}
              />
            )}

            {tab === "composite" && <CompositeRoutesEditor groupID={grp.id} revision={revision} onChanged={onChanged} />}

            {tab === "rates" && <GroupUserRatesEditor groupID={grp.id} revision={revision} onChanged={onChanged} />}

            {tab === "keys" && <GroupStatsKeysTab groupID={grp.id} revision={revision} />}

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
          </Stack>
        )}
      </Resource>
    </div>
  );
}

function GroupModelsPricingTab({grp, revision, onSaved}: {grp: GroupRow; revision: number; onSaved: () => void}) {
  const save = useAction();
  const [allowlistOn, setAllowlistOn] = useState(grp.model_allowlist.enabled);
  const [models, setModels] = useState<string[]>(grp.model_allowlist.models);
  const [longContext, setLongContext] = useState(grp.long_context_pricing_enabled);
  const [pricing, setPricing] = useState<PricingRule[]>(grp.model_pricing);
  const [draftModel, setDraftModel] = useState("");
  const [draftIn, setDraftIn] = useState("");
  const [draftOut, setDraftOut] = useState("");
  const candidates = useModelCatalog(`/admin/groups/${grp.id}/model-candidates?platform=${encodeURIComponent(grp.platform)}`, revision);

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/admin/groups/${grp.id}`, adminGroupRow, "PUT", {
          model_allowlist: {enabled: allowlistOn, models},
          long_context_pricing_enabled: longContext,
          model_pricing: pricing,
        });
        onSaved();
      })}
    >
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

      <FormSection title="组级模型定价覆写 (USD / 1M)">
        <InsetRow label="启用长上下文阶梯计费">
          <Switch label="长上下文计费" checked={longContext} onChange={setLongContext} />
        </InsetRow>
        {pricing.map((rule, idx) => (
          <InsetRow
            key={idx}
            label={rule.models.join(", ")}
            hint={`入 $${((rule.input_price ?? 0) * 1_000_000).toFixed(2)}/M · 出 $${((rule.output_price ?? 0) * 1_000_000).toFixed(2)}/M`}
          >
            <Button size="icon-xs" variant="ghost" aria-label="移除价目" onClick={() => setPricing(pricing.filter((_, i) => i !== idx))}>
              <Trash2 />
            </Button>
          </InsetRow>
        ))}
        <FormGrid cols={3}>
          <Field label="模型"><ModelSelect label="定价模型" value={draftModel} onChange={setDraftModel} catalog={candidates} allowCustom placeholder="选择或输入模型" /></Field>
          <Field label="输入 $/1M"><Input type="number" min="0" step="0.01" value={draftIn} onChange={e => setDraftIn(e.target.value)} /></Field>
          <Field label="输出 $/1M"><Input type="number" min="0" step="0.01" value={draftOut} onChange={e => setDraftOut(e.target.value)} /></Field>
        </FormGrid>
        <Button
          type="button"
          size="xs"
          variant="outline"
          disabled={!draftModel.trim()}
          onClick={() => {
            setPricing([...pricing, {
              platform: grp.platform,
              models: [draftModel.trim()],
              billing_mode: "token",
              input_price: Number(draftIn || 0) / 1_000_000,
              output_price: Number(draftOut || 0) / 1_000_000,
            }]);
            setDraftModel("");
            setDraftIn("");
            setDraftOut("");
          }}
        >
          添加组级模型定价
        </Button>
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="submit" size="sm" busy={save.busy}>保存白名单与定价</Button>
      </FormActions>
    </Form>
  );
}

function GroupStatsKeysTab({groupID, revision}: {groupID: number; revision: number}) {
  const stats = useResource(() => api(`/admin/groups/${groupID}/stats`, groupStatsSchema), `grp-stats-${groupID}`, true, revision);
  const keyCols: Column<GroupKeyRow>[] = [
    {id: "name", title: "密钥名称", render: r => <div><strong>{r.name}</strong><div className="muted code-mono text-fine">{r.masked}</div></div>},
    {id: "user", title: "归属用户", render: r => <EntityLink kind="user" id={r.user_id} label={r.user_email || `#${r.user_id}`} />},
    {id: "quota", title: "已用 / 额度", align: "number", render: r => `${smartMoney(r.quota_used)} / ${r.quota > 0 ? smartMoney(r.quota) : "不限"}`},
    {id: "status", title: "状态", align: "center", render: r => <Status value={r.status} />},
  ];

  return (
    <Stack>
      {stats?.value && (
        <MetricStrip
          items={[
            {id: "keys", label: "组内密钥数", value: count.format(stats.value.total_api_keys)},
            {id: "active-keys", label: "活跃密钥", value: count.format(stats.value.active_api_keys), tone: "ok"},
            {id: "reqs", label: "累计请求", value: count.format(stats.value.total_requests)},
            {id: "cost", label: "累计消耗", value: smartMoney(stats.value.total_cost)},
          ]}
        />
      )}
      <RemoteTable
        path={`/admin/groups/${groupID}/keys`}
        schema={groupKeyRowSchema}
        columns={keyCols}
        rowID={r => r.id}
        label="组内密钥"
        revision={revision}
        empty={<EmptyState title="该分组下暂无关联密钥" />}
      />
    </Stack>
  );
}
