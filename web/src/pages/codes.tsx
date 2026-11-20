import {useRef, useState} from "react";
import {z} from "zod";
import {api, ApiError, empty, grantSchema, issue, issue as issueSchema, type Config} from "../api";
import {cents, date, money} from "../format";
import {useAction, useResource} from "../requests";
import {Button, ConfirmDialog, CopyValue, ErrorLine, Field, Form, Input, Overlay, Resource, Section, Select, Status, Toolbar, type Column} from "../ui";
import {RemoteTable} from "./remote-table";

type Issue = z.infer<typeof issue>;
const codeSchema = z.object({code: z.string().min(1)});

export function Codes({config, admin}: {config: Config; admin: boolean}) {
  const [revision, revise] = useState(0);
  const [kind, setKind] = useState("");
  const [value, setValue] = useState("");
  const [secret, setSecret] = useState("");
  const [referral, setReferral] = useState("");
  const [resolving, resolve] = useState<Issue>();
  const [resolveCode, setResolveCode] = useState("");
  const [refund, setRefund] = useState(false);
  const pending = useRef<{id: string; input: string} | null>(null);
  const operation = useAction();
  const referralAction = useAction();
  const reconciliation = useAction();
  const refundCheck = useAction();
  const grant = useResource(() => api("/grant", grantSchema), "grant", true, revision);
  const columns: Column<Issue>[] = [
    {id: "created", title: "时间", render: row => date(row.created_at * 1000)},
    ...(admin ? [{id: "actor", title: "操作者", render: (row: Issue) => "#" + row.actor_id} satisfies Column<Issue>[][number]] : []),
    {id: "kind", title: "码种", render: row => row.kind === "balance" ? "余额兑换码" : "注册邀请码"},
    {id: "amount", title: "面额", align: "number", render: row => row.amount_cents ? money(row.amount_cents / 100) : "—"},
    {id: "quota", title: "额度变化", align: "number", render: row => row.kind === "balance" ? money(row.quota_delta / 100) : row.quota_delta + " 张"},
    {id: "state", title: "状态", render: row => <Status value={row.state} />},
    {id: "codeID", title: "码 ID", align: "number", render: row => row.code_id ?? "—"},
    ...(admin ? [{id: "actions", title: "操作", align: "end", render: (row: Issue) => row.state === "pending" || row.state === "unknown"
      ? <Button variant="ghost" size="sm" onClick={() => {
        reconciliation.clearError();
        setResolveCode("");
        resolve(row);
      }}>核对</Button>
      : row.request_id ? <Button variant="ghost" size="sm" onClick={() => void navigator.clipboard.writeText(row.request_id ?? "")}>复制请求编号</Button> : "—"} satisfies Column<Issue>[][number]] : []),
  ];

  return <>
    <Section>
      <div className="section-heading"><div><h2>生成兑换码或邀请码</h2></div></div>
      <Resource state={grant}>{permit => {
        const amounts = permit.amounts_cents.filter(amount => amount <= permit.budget_cents);
        const invite = config.invitation && (admin || permit.invites > 0);
        const balance = admin || amounts.length > 0;
        const selected = kind && (kind === "invitation" ? invite : balance) ? kind : invite ? "invitation" : "balance";
        if (!invite && !balance) return <p className="feedback">当前没有可用发码额度。</p>;
        return <Form busy={operation.busy} onSubmit={() => void operation.run(async () => {
          setSecret("");
          const amount = selected === "invitation" ? 0 : admin ? cents(value) : Number(value || amounts[0]);
          const fingerprint = selected + "/" + amount;
          const operationID = pending.current?.input === fingerprint ? pending.current.id : crypto.randomUUID();
          pending.current = {id: operationID, input: fingerprint};
          try {
            const response = await api("/issues", codeSchema, "POST", {operation_id: operationID, kind: selected, amount_cents: amount});
            setSecret(response.code);
            pending.current = null;
          } catch (error) {
            if (error instanceof ApiError && ["issue_rejected", "grant_exhausted", "invalid_input", "not_allowed"].includes(error.code)) pending.current = null;
            throw error;
          } finally {
            revise(previous => previous + 1);
          }
        })}>
          <Toolbar>
            <Field label="码种"><Select value={selected} onChange={event => {setKind(event.target.value); setValue("");}}>
              {invite && <option value="invitation">注册邀请码</option>}
              {balance && <option value="balance">余额兑换码</option>}
            </Select></Field>
            {selected === "balance" && <Field label="面额（USD）">{admin
              ? <Input inputMode="decimal" required value={value} onChange={event => setValue(event.target.value)} />
              : <Select value={value || amounts[0]} onChange={event => setValue(event.target.value)}>
                {amounts.map(amount => <option value={amount} key={amount}>{money(amount / 100)}</option>)}
              </Select>}</Field>}
            <Button type="submit" busy={operation.busy}>生成一张</Button>
            {selected === "invitation" && !admin && <span className="muted">剩余 {permit.invites} 张</span>}
            {selected === "balance" && !admin && <span className="muted">剩余额度 {money(permit.budget_cents / 100)}</span>}
          </Toolbar>
          <ErrorLine value={operation.error} />
        </Form>;
      }}</Resource>
      {secret && <CopyValue value={secret} label="新生成的码" />}
    </Section>
    {config.affiliate && <Section>
      <Toolbar className="section-heading"><h2>个人返利链接</h2>
        <Button variant="outline" busy={referralAction.busy} onClick={() => void referralAction.run(async () => {
          const data = await api("/referral", z.object({url: z.string().url()}));
          setReferral(data.url);
        })}>显示链接</Button>
      </Toolbar>
      <ErrorLine value={referralAction.error} />
      {referral && <CopyValue label="返利链接" value={referral} />}
    </Section>}
    <Section>
      <h2>发码记录</h2>
      <RemoteTable path="/issues" schema={issueSchema} columns={columns} rowID={row => row.operation_id} label="发码记录" revision={revision} />
    </Section>
    <Overlay open={!!resolving} onClose={() => resolve(undefined)} title="核对发码结果" busy={reconciliation.busy}>
      {resolving && <Form busy={reconciliation.busy} onSubmit={event => {
        void reconciliation.run(async () => {
          await api(`/admin/issues/${resolving.operation_id}/resolve`, empty, "POST", {code_id: Number(resolveCode)});
          resolve(undefined);
          setRefund(false);
          revise(previous => previous + 1);
        });
      }}>
        {resolving.request_id && <CopyValue label="请求编号" value={resolving.request_id} />}
        <Field label="官方码 ID（未生成时用下方核验退款）"><Input type="number" min="1" value={resolveCode} onChange={event => setResolveCode(event.target.value)} /></Field>
        <ErrorLine value={reconciliation.error} />
        <div className="form-actions">
          <Button variant="outline" onClick={() => resolve(undefined)}>取消</Button>
          <Button type="submit" disabled={!resolveCode} busy={reconciliation.busy}>保存核对结果</Button>
          <Button variant="destructive" disabled={reconciliation.busy} onClick={() => { refundCheck.clearError(); setRefund(true); }}>确认未生成并退回额度</Button>
        </div>
      </Form>}
    </Overlay>
    <ConfirmDialog open={refund} title="退回预扣额度" description="已核实官方没有生成该码吗？确认后将把预扣额度退回。"
      confirm="确认未生成" busy={refundCheck.busy} error={refundCheck.error} onClose={() => setRefund(false)} onConfirm={() => {
        if (!resolving) return;
        void refundCheck.run(async () => {
          await api(`/admin/issues/${resolving.operation_id}/resolve`, empty, "POST", {code_id: null});
          setRefund(false);
          resolve(undefined);
          revise(previous => previous + 1);
        });
      }} />
  </>;
}
