import {useState} from "react";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {type LedgerEvent} from "@/schemas/ledger";
import {useAction} from "@/lib/requests";
import {date, smartMoney} from "@/lib/format";
import {Badge, Button, ConfirmDialog, ErrorLine, Field, Form, FormActions, FormGrid, FormSection, Input, InsetGroup, InsetRow, Select, toast} from "@/ui";

export const eventKindLabels: Record<string, string> = {
  recharge: "充值",
  recharge_inferred: "推断充值",
  manual_adjust: "手动调整",
  refund: "退款",
  quota_raised: "额度上调",
  baseline_reset: "基线重置",
};

const manualKinds = ["recharge", "manual_adjust", "refund", "baseline_reset"] as const;

function localInputValue(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// 事件接口（请求体字段以后端 LedgerEventInput 为准：kind、amount_usd、occurred_at、status、note）：
//   POST   /admin/ledger/accounts/{id}/events           补录
//   PATCH  /admin/ledger/accounts/{id}/events/{event}   修正金额与备注、确认待确认事件
//   DELETE /admin/ledger/accounts/{id}/events/{event}   作废
export function AccountLedgerEvents({accountID, events, onChanged}: {accountID: number; events: LedgerEvent[]; onChanged: () => void}) {
  const action = useAction();
  const voidAction = useAction();
  const [kind, setKind] = useState<(typeof manualKinds)[number]>("recharge");
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const [when, setWhen] = useState(() => localInputValue(new Date()));
  const [editing, setEditing] = useState<number>();
  const [editAmount, setEditAmount] = useState("");
  const [editNote, setEditNote] = useState("");
  const [voiding, setVoiding] = useState<LedgerEvent>();
  const base = `/admin/ledger/accounts/${accountID}/events`;

  // 只发改动的字段：后端对缺省字段保持原值；kind 只接受手工类型，回传 recharge_inferred 等推断类型会被拒绝。
  function patch(event: LedgerEvent, changes: {amount_usd?: number; note?: string; status?: "confirmed"}) {
    return api(`${base}/${event.id}`, empty, "PATCH", changes);
  }

  function startEdit(event: LedgerEvent) {
    setEditing(event.id);
    setEditAmount(String(event.amount_usd));
    // 原备注已显示在行提示里；输入框从空白开始，留空即不修改。
    setEditNote("");
  }

  return (
    <>
      <InsetGroup title="账本事件">
        {events.length === 0 && <InsetRow label="事件"><span className="muted">暂无</span></InsetRow>}
        {events.map(event => (
          <InsetRow key={event.id} label={`${eventKindLabels[event.kind] ?? event.kind} · ${date(event.occurred_at)}`} hint={event.note || undefined}>
            {editing === event.id ? (
              <span className="inline-stack">
                <Input type="number" step="0.01" aria-label="修正金额 (USD)" value={editAmount} onChange={e => setEditAmount(e.target.value)} />
                <Input aria-label="修正备注（留空不修改）" placeholder="备注（留空不修改）" value={editNote} onChange={e => setEditNote(e.target.value)} />
                <span className="muted text-fine">备注留空不修改</span>
                <Button size="xs" busy={action.busy} onClick={() => void action.run(async () => {
                  const value = Number(editAmount);
                  if (editAmount.trim() === "" || !Number.isFinite(value)) throw new Error("请填写金额（USD）");
                  // 只发与原值不同的字段；备注留空表示不修改（后端对空备注同样保持原值）。
                  const changes: {amount_usd?: number; note?: string} = {};
                  if (value !== event.amount_usd) changes.amount_usd = value;
                  const nextNote = editNote.trim();
                  if (nextNote !== "" && nextNote !== event.note) changes.note = nextNote;
                  if (changes.amount_usd === undefined && changes.note === undefined) {
                    setEditing(undefined);
                    return;
                  }
                  await patch(event, changes);
                  setEditing(undefined);
                  toast("事件已修正");
                  onChanged();
                })}>保存</Button>
                <Button size="xs" variant="ghost" onClick={() => setEditing(undefined)}>取消</Button>
              </span>
            ) : (
              <span className="weir-digits">
                {smartMoney(event.amount_usd)}
                {event.status !== "confirmed" && <Badge variant="soft" tone={event.status === "pending" ? "warn" : "neutral"}>{event.status === "pending" ? "待确认" : "已作废"}</Badge>}
                {event.status === "pending" && event.kind === "recharge_inferred" && (
                  <Button size="xs" variant="outline" busy={action.busy} onClick={() => void action.run(async () => {
                    await patch(event, {status: "confirmed"});
                    toast("已确认充值");
                    onChanged();
                  })}>确认</Button>
                )}
                {event.status !== "void" && <Button size="xs" variant="ghost" onClick={() => startEdit(event)}>修正</Button>}
                {event.status !== "void" && <Button size="xs" variant="ghost" onClick={() => { voidAction.clearError(); setVoiding(event); }}>作废</Button>}
              </span>
            )}
          </InsetRow>
        ))}
      </InsetGroup>
      <ErrorLine value={action.error} />
      <FormSection title="记一笔">
        <Form busy={action.busy} onSubmit={() => void action.run(async () => {
          const value = Number(amount);
          if (amount.trim() === "" || !Number.isFinite(value)) throw new Error("请填写金额（USD）");
          const at = new Date(when);
          if (Number.isNaN(at.getTime())) throw new Error("请填写有效的发生时间");
          await api(base, empty, "POST", {kind, amount_usd: value, occurred_at: at.toISOString(), status: "confirmed", note: note.trim()});
          setAmount("");
          setNote("");
          setWhen(localInputValue(new Date()));
          toast("已记入账本");
          onChanged();
        })}>
          <FormGrid>
            <Field label="类型">
              <Select value={kind} onChange={event => setKind(event.target.value as typeof kind)}>
                {manualKinds.map(item => <option key={item} value={item}>{eventKindLabels[item]}</option>)}
              </Select>
            </Field>
            <Field label="金额 (USD)"><Input type="number" step="0.01" value={amount} onChange={event => setAmount(event.target.value)} /></Field>
            <Field label="发生时间"><Input type="datetime-local" value={when} onChange={event => setWhen(event.target.value)} /></Field>
            <Field label="备注"><Input value={note} onChange={event => setNote(event.target.value)} /></Field>
          </FormGrid>
          <FormActions><Button type="submit" size="sm" busy={action.busy}>记入账本</Button></FormActions>
        </Form>
      </FormSection>
      <ConfirmDialog
        open={!!voiding}
        critical
        title="作废账本事件"
        description={voiding ? `确定作废「${eventKindLabels[voiding.kind] ?? voiding.kind}」${smartMoney(voiding.amount_usd)} 吗？作废后不再计入对账。` : ""}
        confirm="作废"
        busy={voidAction.busy}
        error={voidAction.error}
        onClose={() => setVoiding(undefined)}
        onConfirm={() => {
          const target = voiding;
          if (!target) return;
          void voidAction.run(async () => {
            await api(`${base}/${target.id}`, empty, "DELETE");
            setVoiding(undefined);
            toast("事件已作废");
            onChanged();
          });
        }}
      />
    </>
  );
}
