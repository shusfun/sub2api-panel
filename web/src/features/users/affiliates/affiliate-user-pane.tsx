import {useId, useState} from "react";
import {api} from "@/lib/http";
import {money} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {affiliateOverview, affiliateWithdrawResult, empty} from "@/schemas";
import {
  Button, ConfirmDialog, ErrorLine, Field, Form, Input, InsetGroup, InsetRow, Overlay, Resource, Toolbar, toast,
} from "@/ui";

type Task = "withdraw" | "code" | "rate" | "clear";

export function AffiliateUserPane({userId}: {userId: number}) {
  const [revision, revise] = useState(0);
  const [task, setTask] = useState<Task>();
  const overview = useResource(() => api(`/admin/affiliates/users/${userId}/overview`, affiliateOverview), `aff-overview-${userId}`, userId > 0, revision);
  const done = (message: string) => { setTask(undefined); toast(message); revise(v => v + 1); };

  return (
    <Resource state={overview}>
      {view => (
        <>
          <InsetGroup title="推广概览">
            <InsetRow label="邀请码" onClick={() => setTask("code")}><span className="code-mono">{view.aff_code}</span></InsetRow>
            <InsetRow label="返利比例" onClick={() => setTask("rate")}>{view.rebate_rate_percent}%</InsetRow>
            <InsetRow label="已邀请 / 产生返利">{view.invited_count} / {view.rebated_invitee_count} 人</InsetRow>
            <InsetRow label="可转出返利"><strong className="weir-digits">{money(view.available_quota)}</strong></InsetRow>
            <InsetRow label="历史累计返利"><span className="weir-digits">{money(view.history_quota)}</span></InsetRow>
          </InsetGroup>
          <Toolbar>
            <Button variant="outline" disabled={view.available_quota <= 0} onClick={() => setTask("withdraw")}>代为转出返利</Button>
            <Button variant="outline" onClick={() => setTask("code")}>修改邀请码</Button>
            <Button variant="outline" onClick={() => setTask("rate")}>设置专属比例</Button>
            <Button variant="destructive" onClick={() => setTask("clear")}>清除推广数据</Button>
          </Toolbar>

          {task === "withdraw" && <WithdrawDialog userId={userId} max={view.available_quota} onClose={() => setTask(undefined)} onDone={done} />}
          {task === "code" && <UpdateDialog kind="code" userId={userId} current={view.aff_code} onClose={() => setTask(undefined)} onDone={done} />}
          {task === "rate" && <UpdateDialog kind="rate" userId={userId} current={String(view.rebate_rate_percent)} onClose={() => setTask(undefined)} onDone={done} />}
          <ClearDialog open={task === "clear"} userId={userId} onClose={() => setTask(undefined)} onDone={done} />
        </>
      )}
    </Resource>
  );
}

function WithdrawDialog({userId, max, onClose, onDone}: {userId: number; max: number; onClose: () => void; onDone: (message: string) => void}) {
  const formID = useId();
  const op = useAction();
  const [operationID] = useState(() => crypto.randomUUID());
  return (
    <Overlay open title="代为转出返利" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>确认转出</Button>
      </>
    }>
      <Form id={formID} busy={op.busy} onSubmit={e => {
        const amount = Number(new FormData(e.currentTarget).get("amount"));
        void op.run(async () => {
          if (!(amount > 0) || amount > max) throw new Error("转出金额需大于 0 且不超过可转出返利");
          await api(`/admin/affiliates/users/${userId}/withdraw`, affiliateWithdrawResult, "POST", {amount, operation_id: operationID});
          onDone("返利已转出");
        });
      }}>
        <Field label={`转出金额（USD，最多 ${money(max)}）`}><Input name="amount" type="number" min="0.01" max={max} step="0.01" defaultValue={max} required /></Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

function UpdateDialog({kind, userId, current, onClose, onDone}: {kind: "code" | "rate"; userId: number; current: string; onClose: () => void; onDone: (message: string) => void}) {
  const formID = useId();
  const op = useAction();
  const isCode = kind === "code";
  const [value, setValue] = useState(current);
  const save = (body: Record<string, unknown>, message: string) => void op.run(async () => {
    await api(`/admin/affiliates/users/${userId}`, empty, "PUT", body);
    onDone(message);
  });
  return (
    <Overlay open title={isCode ? "修改邀请码" : "设置专属返利比例"} onClose={onClose} busy={op.busy} footer={
      <>
        {!isCode && <Button variant="ghost" disabled={op.busy} onClick={() => save({clear_rebate_rate: true}, "已恢复沿用全局比例")}>恢复全局比例</Button>}
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>保存</Button>
      </>
    }>
      <Form id={formID} busy={op.busy} onSubmit={() => {
        if (isCode) save({aff_code: value.trim()}, "邀请码已更新");
        else save({aff_rebate_rate_percent: Number(value)}, "返利比例已更新");
      }}>
        {isCode
          ? <Field label="邀请码（4-32 位）"><Input value={value} minLength={4} maxLength={32} required onChange={e => setValue(e.target.value)} /></Field>
          : <Field label="返利比例（0-100 %）"><Input type="number" min="0" max="100" step="0.1" value={value} required onChange={e => setValue(e.target.value)} /></Field>}
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

function ClearDialog({open, userId, onClose, onDone}: {open: boolean; userId: number; onClose: () => void; onDone: (message: string) => void}) {
  const op = useAction();
  return (
    <ConfirmDialog
      open={open}
      title="清除推广数据"
      description="将清除该用户的专属邀请码、专属返利比例等推广设置。此操作由上游执行，无法在面板内撤销。"
      confirm="清除"
      critical
      busy={op.busy}
      error={op.error}
      onClose={onClose}
      onConfirm={() => void op.run(async () => {
        await api(`/admin/affiliates/users/${userId}`, empty, "DELETE");
        onDone("推广数据已清除");
      })}
    />
  );
}
