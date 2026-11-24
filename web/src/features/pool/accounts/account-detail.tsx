import {useState} from "react";
import {Wrench} from "lucide-react";
import {api} from "@/lib/http";
import {account, accountBalance, accountQuota, accountStatsSummary, empty} from "@/schemas";
import {compactTokens, count, currencyMoney, percent, relativeUntil, smartMoney} from "@/lib/format";
import {useModelCatalog} from "@/lib/model-catalog";
import {useAction, useResource} from "@/lib/requests";
import {navigateTo} from "@/app/route";
import {
  Button,
  ConfirmDialog,
  DetailHeader,
  ErrorLine,
  InlineStack,
  InsetGroup,
  InsetRow,
  MeterBar,
  MiniPills,
  Resource,
  Tabs,
  toast,
} from "@/ui";
import {AddAccountDialog} from "./account-add-dialog";
import {AccountBalanceSettings, AccountConfigForm} from "./account-config";
import {AccountGuardForm} from "./account-guard";
import {canRefreshBalance, isQuotaCapable} from "./account-helpers";
import {AccountProbeSection, AccountScheduledTests} from "./account-probe";

type AccountDetailTab = "health" | "routing" | "guard";

export function AccountDetailWorkspace({id, queryString}: {id: number; queryString: string}) {
  const [revision, revise] = useState(0);
  const params = Object.fromEntries(new URLSearchParams(queryString));
  const tab: AccountDetailTab = params.tab === "routing" || params.tab === "config" ? "routing" : params.tab === "guard" ? "guard" : "health";
  const back = () => {
    const listParams = {...params};
    delete listParams.tab;
    delete listParams.detail;
    navigateTo("accounts", listParams);
  };
  return (
    <div className="workspace-section">
      <DetailHeader backLabel="返回上游账号" title={`账号 #${id}`} onBack={back} />
      <AccountInspector id={id} revision={revision} subTab={tab} onTabChange={next => navigateTo("accounts", {...params, detail: id, tab: next === "health" ? undefined : next})}
        onChanged={() => revise(n => n + 1)} onDeleted={back} />
    </div>
  );
}

function AccountInspector({
  id,
  revision,
  subTab,
  onTabChange,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  subTab: AccountDetailTab;
  onTabChange: (next: AccountDetailTab) => void;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const [quotaSource, setQuotaSource] = useState<"passive" | "active">("passive");
  const [statsDays, setStatsDays] = useState<"1" | "7" | "30">("7");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [reauthOpen, setReauthOpen] = useState(false);

  const detail = useResource(() => api(`/admin/accounts/${id}`, account), `acc-detail-${id}`, true, revision);
  const quota = useResource(() => api(`/admin/accounts/${id}/quota?source=${quotaSource}`, accountQuota), `acc-quota-${id}-${quotaSource}`, true, revision);
  const models = useModelCatalog(`/admin/accounts/${id}/models`, revision, subTab === "health", true);
  const stats = useResource(
    () => api(`/admin/accounts/${id}/stats?days=${statsDays}`, accountStatsSummary),
    `acc-stats-${id}-${statsDays}`,
    true,
    revision,
  );
  const op = useAction();
  const del = useAction();

  return (
    <Resource state={detail}>
      {acc => {
        const windows = [
          {label: "5 小时窗口", win: quota?.value?.five_hours ?? null},
          {label: "7 天窗口", win: quota?.value?.week ?? null},
          ...(quota?.value?.sonnet_week ? [{label: "Sonnet 7d 窗口", win: quota.value.sonnet_week}] : []),
          ...(quota?.value?.gemini_daily ? [{label: "Gemini 每日配额", win: quota.value.gemini_daily}] : []),
          ...(quota?.value?.gemini_pro_daily ? [{label: "Gemini Pro 日配额", win: quota.value.gemini_pro_daily}] : []),
          ...(quota?.value?.thirty_day ? [{label: "30 天账单配额", win: quota.value.thirty_day}] : []),
        ];
        return (
          <>
            <h2 className="stage-heading">{acc.name}</h2>
            <Tabs
              label="账号详情分页"
              value={subTab}
              items={[
                {value: "health", label: "健康、配额与验真探测"},
                {value: "routing", label: "路由、凭据与模型映射"},
                {value: "guard", label: "防封控量与生命周期"},
              ]}
              onChange={v => onTabChange(v as AccountDetailTab)}
            />

            {subTab === "health" ? (
              <>
                <InsetGroup
                  title="调用消耗与配额水位"
                  actions={
                    <InlineStack>
                      <MiniPills
                        label="统计天数"
                        value={statsDays}
                        options={[
                          ["1", "今日"],
                          ["7", "7d"],
                          ["30", "30d"],
                        ] as const}
                        onChange={setStatsDays}
                      />
                      <Button size="xs" variant="ghost" onClick={() => setQuotaSource(prev => (prev === "passive" ? "active" : "passive"))}>
                        {quotaSource === "passive" ? "主动探测配额" : "切回被动快照"}
                      </Button>
                    </InlineStack>
                  }
                >
                  {stats?.value && (
                    <InsetRow label={statsDays === "1" ? "今日承载调用" : `近 ${statsDays} 天承载调用`}>
                      <span className="weir-digits">
                        {count.format(stats.value.summary.total_requests)} 次 · {compactTokens(stats.value.summary.total_tokens)} Token · {smartMoney(stats.value.summary.total_cost)}
                      </span>
                    </InsetRow>
                  )}
                  {acc.balance && (
                    <InsetRow label="上游渠道余额">
                      <span className="weir-digits">
                        {acc.balance.balances && acc.balance.balances.length > 1
                          ? acc.balance.balances.map(b => currencyMoney(b.balance, b.currency)).join(" · ")
                          : currencyMoney(acc.balance.balance, acc.balance.currency)}
                      </span>
                    </InsetRow>
                  )}
                  {isQuotaCapable(acc, quota?.value) &&
                    windows.map(w => (
                      <InsetRow key={w.label} label={w.label} hint={w.win?.resets_at ? `重置于 ${relativeUntil(w.win.resets_at)}` : undefined}>
                        <MeterBar size="sm" value={w.win?.percent_used ?? null} label={percent(w.win?.percent_used ?? null)} />
                      </InsetRow>
                    ))}
                  <InsetRow label="跨工作台联动">
                    <InlineStack>
                      <Button size="xs" variant="outline" onClick={() => navigateTo("usage", {account: acc.id})}>查账单</Button>
                      <Button size="xs" variant="outline" onClick={() => navigateTo("ops", {tab: "errors", account_id: acc.id})}>查报错</Button>
                    </InlineStack>
                  </InsetRow>
                </InsetGroup>

                <InsetGroup title="一键状态恢复与凭据续期">
                  <InsetRow label="运行恢复动作" vertical>
                    <InlineStack wrap>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/recover`, account, "POST", {});
                            toast("已恢复账号运行状态");
                            onChanged();
                          })
                        }
                      >
                        <Wrench aria-hidden />一键恢复状态
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/clear-error`, account, "POST", {});
                            toast("已清除错误标记");
                            onChanged();
                          })
                        }
                      >
                        清除异常
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/clear-rate-limit`, account, "POST", {});
                            toast("已清除限流状态");
                            onChanged();
                          })
                        }
                      >
                        清除限流
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/temp-unschedulable`, empty, "DELETE");
                            toast("已解除临时冷却");
                            onChanged();
                          })
                        }
                      >
                        解除冷却
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        busy={op.busy}
                        onClick={() =>
                          void op.run(async () => {
                            await api(`/admin/accounts/${acc.id}/refresh`, account, "POST", {});
                            toast("已在服务端刷新上游凭据");
                            onChanged();
                          })
                        }
                      >
                        刷新 Token
                      </Button>
                      {canRefreshBalance(acc) && (
                        <Button
                          size="xs"
                          variant="outline"
                          busy={op.busy}
                          onClick={() =>
                            void op.run(async () => {
                              await api(`/admin/accounts/${acc.id}/balance`, accountBalance, "POST", {});
                              toast("已探测刷新上游渠道余额");
                              onChanged();
                            })
                          }
                        >
                          刷新渠道余额
                        </Button>
                      )}
                      {(acc.type === "oauth" || acc.type === "setup-token") && (
                        <Button size="xs" variant="outline" onClick={() => setReauthOpen(true)}>重新 OAuth 授权</Button>
                      )}
                    </InlineStack>
                  </InsetRow>
                </InsetGroup>

                <AccountProbeSection accountID={acc.id} models={models} busy={op.busy} runAction={op.run} />
                <ErrorLine value={op.error} />
              </>
            ) : subTab === "routing" ? (
              <>
                {acc.type === "apikey" && <AccountBalanceSettings account={acc} revision={revision} onRefreshed={onChanged} />}
                <AccountScheduledTests accountID={acc.id} />
                <AccountConfigForm
                  acc={acc}
                  revision={revision}
                  onSaved={() => {
                    toast("账号配置已更新");
                    onChanged();
                  }}
                  onRequestDelete={() => setConfirmDelete(true)}
                />
              </>
            ) : (
              <AccountGuardForm
                acc={acc}
                onSaved={() => {
                  toast("防封控量与生命周期已更新");
                  onChanged();
                }}
              />
            )}

            <AddAccountDialog
              open={reauthOpen}
              revision={revision}
              reauthAccount={acc}
              onClose={() => setReauthOpen(false)}
              onCreated={() => {
                setReauthOpen(false);
                toast("OAuth 重新授权成功，凭据已在服务端更新");
                onChanged();
              }}
              onInlineResourceCreated={onChanged}
            />

            <ConfirmDialog
              open={confirmDelete}
              title="删除上游账号"
              description={`确定删除账号「${acc.name}」吗？`}
              confirm="删除"
              busy={del.busy}
              error={del.error}
              onClose={() => setConfirmDelete(false)}
              onConfirm={() =>
                void del.run(async () => {
                  await api(`/admin/accounts/${acc.id}`, empty, "DELETE");
                  setConfirmDelete(false);
                  onDeleted();
                })
              }
            />
          </>
        );
      }}
    </Resource>
  );
}
