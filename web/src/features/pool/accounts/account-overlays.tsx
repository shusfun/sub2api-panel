import {useEffect, useId, useState} from "react";
import {Boxes, Zap} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {account, accountStatsSummary} from "@/schemas";
import {compactTokens, count, currencyMoney, smartMoney} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {navigateTo} from "@/app/route";
import {
  Button,
  ConfirmDialog,
  CopyButton,
  EmptyState,
  ErrorLine,
  Field,
  Form,
  FormGrid,
  FormSection,
  Input,
  InsetGroup,
  InsetRow,
  MultiGroupSelect,
  Overlay,
  PlanSummary,
  PlatformBadge,
  ProxySelect,
  RefreshMorphIcon,
  Resource,
  SearchInput,
  Segmented,
  Status,
  Switch,
  Toolbar,
  toast,
} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";
import {AccountInspectorEdit} from "./account-inspector-edit";

export function AccountModelsDialog({accountRow, onClose}: {accountRow?: AccountRow; onClose: () => void}) {
  const [keyword, setKeyword] = useState("");
  const [syncRevision, setSyncRevision] = useState(0);
  const syncOp = useAction();

  useEffect(() => {
    setKeyword("");
  }, [accountRow?.id]);

  const models = useResource(
    () => (accountRow ? api(`/admin/accounts/${accountRow.id}/models`, z.array(z.object({id: z.string(), display_name: z.string()}))) : Promise.resolve([])),
    accountRow ? `acc-models-modal-${accountRow.id}` : "acc-models-none",
    !!accountRow,
    syncRevision,
  );

  const needle = keyword.trim().toLocaleLowerCase();
  const list = (models?.value ?? []).filter(
    m => !needle || m.id.toLocaleLowerCase().includes(needle) || m.display_name.toLocaleLowerCase().includes(needle),
  );

  return (
    <Overlay
      open={!!accountRow}
      title={accountRow ? `可用模型 · ${accountRow.name}` : "可用模型"}
      onClose={onClose}
      busy={syncOp.busy}
      footer={
        <>
          <Button variant="outline" disabled={syncOp.busy} onClick={onClose}>关闭</Button>
          {accountRow && (
            <Button
              busy={syncOp.busy}
              onClick={() =>
                void syncOp.run(async () => {
                  const res = await api(`/admin/accounts/${accountRow.id}/models/sync`, z.object({models: z.array(z.string())}), "POST", {});
                  toast(`已同步 ${res.models.length} 个上游模型`);
                  setSyncRevision(v => v + 1);
                })
              }
            >
              <RefreshMorphIcon />同步上游模型
            </Button>
          )}
        </>
      }
    >
      <SearchInput value={keyword} onChange={setKeyword} placeholder="搜索模型名称或 ID" ariaLabel="搜索可用模型" />
      <ErrorLine value={syncOp.error} />
      <Resource state={models}>
        {all =>
          list.length === 0 ? (
            <EmptyState title={all.length === 0 ? "暂无可用模型" : "没有匹配的模型"} />
          ) : (
            <InsetGroup title={`共 ${list.length} 个模型`}>
              {list.map(item => (
                <InsetRow key={item.id} label={item.display_name || item.id} hint={item.display_name && item.display_name !== item.id ? item.id : undefined}>
                  <CopyButton label={`复制模型 ${item.id}`} getText={async () => item.id} />
                </InsetRow>
              ))}
            </InsetGroup>
          )
        }
      </Resource>
    </Overlay>
  );
}

export function AccountSummary({
  id,
  row,
  revision,
  onShowModels,
  onSaved,
}: {
  id: number;
  row?: AccountRow;
  revision: number;
  onShowModels: (acc: AccountRow) => void;
  onSaved: () => void;
}) {
  const detail = useResource(() => api(`/admin/accounts/${id}`, account), `acc-detail-${id}`, true, revision);
  const stats7d = useResource(() => api(`/admin/accounts/${id}/stats?days=7`, accountStatsSummary), `acc-sum-7d-${id}`, true, revision);
  return (
    <Resource state={detail}>
      {value => {
        const bal = value.balance ?? row?.balance;
        return (
          <>
            <InsetGroup title="身份与路由">
              <InsetRow label="平台与接入方式"><PlatformBadge platform={value.platform} type={value.type} /></InsetRow>
              <InsetRow label="状态"><Status value={value.status} /></InsetRow>
              <InsetRow label="并发"><span className="weir-digits">{value.current_concurrency} / {value.concurrency || "不限"}</span></InsetRow>
              <InsetRow label="优先级 / 倍率"><span className="weir-digits">{value.priority} · {value.rate_multiplier}×</span></InsetRow>
              <InsetRow label="所属分组"><PlanSummary label="分组" items={value.groups} empty="未绑组" /></InsetRow>
              {bal && (
                <InsetRow label="渠道余额">
                  <span className="weir-digits">
                    {bal.balances && bal.balances.length > 1
                      ? bal.balances.map(b => currencyMoney(b.balance, b.currency)).join(" · ")
                      : currencyMoney(bal.balance, bal.currency)}
                  </span>
                </InsetRow>
              )}
            </InsetGroup>
            <InsetGroup title="消耗摘要">
              {row?.today_cost !== undefined && (
                <InsetRow label="今日调用">
                  <span className="weir-digits">
                    {count.format(row.today_requests ?? 0)} 次 · {compactTokens(row.today_tokens)} Token · {smartMoney(row.today_cost)}
                  </span>
                </InsetRow>
              )}
              {stats7d?.value && (
                <InsetRow label="近 7 天调用">
                  <span className="weir-digits">
                    {count.format(stats7d.value.summary.total_requests)} 次 · {compactTokens(stats7d.value.summary.total_tokens)} Token · {smartMoney(stats7d.value.summary.total_cost)}
                  </span>
                </InsetRow>
              )}
            </InsetGroup>
            <AccountInspectorEdit acc={value} revision={revision} onSaved={onSaved} />
            <Toolbar>
              <Button size="sm" onClick={() => navigateTo("accounts", {detail: id})}><Zap aria-hidden />深测与高级配置</Button>
              <Button size="sm" variant="outline" onClick={() => onShowModels(value)}><Boxes aria-hidden />可用模型</Button>
              <Button variant="link" onClick={() => navigateTo("usage", {account: id})}>查看用量</Button>
              <Button variant="link" onClick={() => navigateTo("ops", {tab: "errors", account_id: id})}>排查报错</Button>
            </Toolbar>
          </>
        );
      }}
    </Resource>
  );
}

export function BulkBindAccountsDialog({
  open,
  ids,
  platform,
  revision,
  onClose,
  onSaved,
}: {
  open: boolean;
  ids: readonly number[];
  platform?: string;
  revision: number;
  onClose: () => void;
  onSaved: () => void;
}) {
  const formID = useId();
  const [applyGroups, setApplyGroups] = useState(true);
  const [groupMode, setGroupMode] = useState<"replace" | "append" | "remove">("replace");
  const [groupIDs, setGroupIDs] = useState<number[]>([]);
  const [applyProxy, setApplyProxy] = useState(false);
  const [proxyID, setProxyID] = useState("");
  const [priority, setPriority] = useState("");
  const [concurrency, setConcurrency] = useState("");
  const [rateMultiplier, setRateMultiplier] = useState("");
  const [mixedRiskMsg, setMixedRiskMsg] = useState<string>();
  const op = useAction();

  const hasChanges = applyGroups || applyProxy || priority.trim() !== "" || concurrency.trim() !== "" || rateMultiplier.trim() !== "";

  async function submitBulk(confirmMixedRisk = false) {
    if (!confirmMixedRisk && applyGroups && groupMode !== "remove" && groupIDs.length > 0 && platform) {
      const check = await api("/admin/accounts/check-mixed-channel", z.object({has_risk: z.boolean(), message: z.string().optional()}), "POST", {
        platform,
        group_ids: groupIDs,
      });
      if (check.has_risk) {
        setMixedRiskMsg(check.message || "所选分组存在混渠道风险，确定继续批量绑定吗？");
        return;
      }
    }
    await api("/admin/accounts/bulk-update", z.object({success: z.number(), failed: z.number()}), "POST", {
      account_ids: ids,
      ...(applyGroups ? {group_ids: groupIDs, group_mode: groupMode} : {}),
      ...(applyProxy ? (proxyID ? {proxy_id: Number(proxyID)} : {clear_proxy: true}) : {}),
      ...(priority.trim() !== "" ? {priority: Number(priority)} : {}),
      ...(concurrency.trim() !== "" ? {concurrency: Number(concurrency)} : {}),
      ...(rateMultiplier.trim() !== "" ? {rate_multiplier: Number(rateMultiplier)} : {}),
      ...(confirmMixedRisk ? {confirm_mixed_channel_risk: true} : {}),
    });
    setMixedRiskMsg(undefined);
    onSaved();
  }

  return (
    <>
      <Overlay
        open={open}
        title={`批量调整 ${ids.length} 个账号参数与归属`}
        onClose={onClose}
        busy={op.busy}
        footer={
          <>
            <Button variant="outline" onClick={onClose}>取消</Button>
            <Button type="submit" form={formID} disabled={!hasChanges} busy={op.busy}>执行批量更新</Button>
          </>
        }
      >
        <Form id={formID} busy={op.busy} onSubmit={() => void op.run(() => submitBulk(false))}>
          <FormSection title="分组与代理归属">
            <InsetRow label="同步修改所属调度分组"><Switch label="修改所属分组" checked={applyGroups} onChange={setApplyGroups} /></InsetRow>
            {applyGroups && (
              <>
                <Field label="分组更新方式">
                  <Segmented
                    label="分组更新方式"
                    value={groupMode}
                    options={[["replace", "替换"], ["append", "追加"], ["remove", "移除"]] as const}
                    onChange={setGroupMode}
                  />
                </Field>
                <Field label="目标分组列表"><MultiGroupSelect value={groupIDs} revision={revision} onChange={setGroupIDs} /></Field>
              </>
            )}
            <InsetRow label="同步切换代理节点"><Switch label="修改代理节点" checked={applyProxy} onChange={setApplyProxy} /></InsetRow>
            {applyProxy && <Field label="目标代理（清空即改回直连）"><ProxySelect value={proxyID} revision={revision} onChange={setProxyID} /></Field>}
          </FormSection>
          <FormSection title="调度参数覆写（留空不改）">
            <FormGrid cols={3}>
              <Field label="优先级"><Input type="number" value={priority} onChange={e => setPriority(e.target.value)} placeholder="留空" /></Field>
              <Field label="最大并发"><Input type="number" min="0" value={concurrency} onChange={e => setConcurrency(e.target.value)} placeholder="留空" /></Field>
              <Field label="成本倍率"><Input type="number" min="0" step="0.01" value={rateMultiplier} onChange={e => setRateMultiplier(e.target.value)} placeholder="留空" /></Field>
            </FormGrid>
          </FormSection>
          <ErrorLine value={op.error} />
        </Form>
      </Overlay>
      <ConfirmDialog
        open={!!mixedRiskMsg}
        title="混渠道批量绑定确认"
        description={mixedRiskMsg ?? ""}
        confirm="确认继续绑定"
        busy={op.busy}
        error={op.error}
        onClose={() => setMixedRiskMsg(undefined)}
        onConfirm={() => void op.run(() => submitBulk(true))}
      />
    </>
  );
}
