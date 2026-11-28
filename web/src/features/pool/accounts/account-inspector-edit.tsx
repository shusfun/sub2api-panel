import {useState} from "react";
import {api} from "@/lib/http";
import {account} from "@/schemas";
import {ledgerAccountSeries} from "@/schemas/ledger";
import {useAction, useResource} from "@/lib/requests";
import {date, smartMoney} from "@/lib/format";
import {Button, ErrorLine, Field, Form, FormActions, FormGrid, FormSection, Input, InsetGroup, InsetRow, Resource} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";

const eventLabels: Record<string, string> = {
  recharge: "充值",
  recharge_inferred: "推断充值",
  manual_adjust: "手动调整",
  refund: "退款",
  quota_raised: "额度上调",
  baseline_reset: "基线重置",
};

function text(value: number | null | undefined) {
  return value != null ? String(value) : "";
}

// 检查器内集中编辑：限额、倍率、并发、RPM、会话上限。
// 字段与接口：PUT /admin/accounts/{id}
//   窗口成本上限 -> extra.window_cost_limit
//   每日配额上限 -> extra.daily_quota
//   倍率         -> rate_multiplier
//   并发         -> concurrency
//   RPM          -> extra.rpm_limit
//   会话上限     -> extra.max_sessions
// 请求体只含这 6 个字段，extra 里留空的键为 null（表示删除），例如：
//   {"rate_multiplier":1.2,"concurrency":4,"extra":{"window_cost_limit":5,"daily_quota":null,"rpm_limit":60,"max_sessions":null}}
export function AccountInspectorEdit({acc, revision, onSaved}: {acc: AccountRow; revision: number; onSaved: () => void}) {
  const save = useAction();
  const extra = acc.extra;
  const [windowCost, setWindowCost] = useState(text(extra?.window_cost_limit));
  const [dailyQuota, setDailyQuota] = useState(text(extra?.daily_quota));
  const [multiplier, setMultiplier] = useState(String(acc.rate_multiplier ?? 1));
  const [concurrency, setConcurrency] = useState(String(acc.concurrency ?? 0));
  const [rpm, setRpm] = useState(text(extra?.rpm_limit));
  const [sessions, setSessions] = useState(text(extra?.max_sessions));
  const series = useResource(
    () => api(`/admin/ledger/accounts/${acc.id}?granularity=day`, ledgerAccountSeries),
    `acc-ledger-${acc.id}`,
    true,
    revision,
  );

  async function submit() {
    // 只发 4 个 extra 键；清空的键发 null，后端据此删除。
    const next: Record<string, number | null> = {};
    const fields: [string, string][] = [
      ["window_cost_limit", windowCost],
      ["daily_quota", dailyQuota],
      ["rpm_limit", rpm],
      ["max_sessions", sessions],
    ];
    for (const [key, raw] of fields) {
      if (raw.trim() === "") next[key] = null;
      else {
        const value = Number(raw);
        if (!Number.isFinite(value) || value < 0) throw new Error("限额、RPM 和会话上限需要填写不小于 0 的数字");
        next[key] = value;
      }
    }
    const mult = Number(multiplier || 1);
    if (!Number.isFinite(mult) || mult < 0) throw new Error("倍率需要填写不小于 0 的数字");
    await api(`/admin/accounts/${acc.id}`, account, "PUT", {
      rate_multiplier: mult,
      concurrency: Number(concurrency || 0),
      extra: next,
    });
    onSaved();
  }

  return (
    <>
      <Form busy={save.busy} onSubmit={() => void save.run(submit)}>
        <FormSection title="限额与并发">
          <FormGrid>
            <Field label="窗口成本上限 (USD)"><Input type="number" min="0" step="0.01" value={windowCost} onChange={e => setWindowCost(e.target.value)} placeholder="不限" /></Field>
            <Field label="每日配额上限 (USD)"><Input type="number" min="0" step="0.01" value={dailyQuota} onChange={e => setDailyQuota(e.target.value)} placeholder="不限" /></Field>
            <Field label="账号成本倍率"><Input type="number" min="0" step="0.01" value={multiplier} onChange={e => setMultiplier(e.target.value)} /></Field>
            <Field label="最大并发连接"><Input type="number" min="0" value={concurrency} onChange={e => setConcurrency(e.target.value)} /></Field>
            <Field label="单号 RPM 限制"><Input type="number" min="0" value={rpm} onChange={e => setRpm(e.target.value)} placeholder="不限" /></Field>
            <Field label="会话上限"><Input type="number" min="0" value={sessions} onChange={e => setSessions(e.target.value)} placeholder="不限" /></Field>
          </FormGrid>
        </FormSection>
        <ErrorLine value={save.error} />
        <FormActions>
          <Button type="submit" size="sm" busy={save.busy}>保存限额与并发</Button>
        </FormActions>
      </Form>
      <Resource state={series}>
        {value => (
          <InsetGroup title="账本摘要">
            <InsetRow label="今日实际 / 预计">
              <span className="weir-digits">
                {value.summary.actual_today_usd != null ? smartMoney(value.summary.actual_today_usd) : "未知"} / {smartMoney(value.summary.expected_today_usd)}
              </span>
            </InsetRow>
            {value.summary.deviation_pct != null && (
              <InsetRow label="偏差"><span className="weir-digits">{value.summary.deviation_pct.toFixed(1)}%</span></InsetRow>
            )}
            {value.show_profit && value.summary.profit_today_usd != null && (
              <InsetRow label="今日毛利"><span className="weir-digits">{smartMoney(value.summary.profit_today_usd)}</span></InsetRow>
            )}
            {value.events.slice(0, 3).map(event => (
              <InsetRow key={event.id} label={`${eventLabels[event.kind] ?? event.kind} · ${date(event.occurred_at)}`}>
                <span className="weir-digits">{smartMoney(event.amount_usd)}{event.status !== "confirmed" ? ` · ${event.status === "pending" ? "待确认" : "已作废"}` : ""}</span>
              </InsetRow>
            ))}
            {value.events.length === 0 && <InsetRow label="最近事件"><span className="muted">暂无</span></InsetRow>}
          </InsetGroup>
        )}
      </Resource>
    </>
  );
}
