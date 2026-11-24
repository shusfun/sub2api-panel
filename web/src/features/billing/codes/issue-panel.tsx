import {useId, useRef, useState} from "react";
import {Ticket} from "lucide-react";
import {z} from "zod";
import {ApiError, api} from "@/lib/http";
import {cents, date, money} from "@/lib/format";
import {useAction} from "@/lib/requests";
import {empty, issue as issueSchema, type Config} from "@/schemas";
import {
  Badge, Button, ConfirmDialog, CopyButton, CopyValue, EntityLink, ErrorLine, Field, Form,
  Input, Overlay, Panel, RemoteTable, Select, Status, Toolbar, type Column,
} from "@/ui";

type IssueRow = z.infer<typeof issueSchema>;

export function IssuerAuditSection({
  config,
  revision,
  onChanged,
}: {
  config: Config;
  revision: number;
  onChanged: () => void;
}) {
  const formID = useId();
  const [kind, setKind] = useState<"invitation" | "balance">(config.invitation ? "invitation" : "balance");
  const [value, setValue] = useState("10.00");
  const [secret, setSecret] = useState("");
  const [resolving, setResolving] = useState<IssueRow>();
  const [resolveCode, setResolveCode] = useState("");
  const [refundOpen, setRefundOpen] = useState(false);
  const pending = useRef<{id: string; input: string} | null>(null);
  const op = useAction();
  const reconcile = useAction();
  const refund = useAction();

  const cols: Column<IssueRow>[] = [
    {id: "time", title: "时间", render: r => <span className="code-mono">{date(r.created_at * 1000)}</span>},
    {
      id: "actor",
      title: "签发人",
      render: r => r.actor_id ? <EntityLink kind="user" id={r.actor_id} label={`#${r.actor_id}`} /> : "—",
    },
    {id: "kind", title: "类型", render: r => <Badge variant="tag">{r.kind === "balance" ? "余额码" : "邀请码"}</Badge>},
    {id: "amt", title: "面额", align: "number", render: r => r.amount_cents ? money(r.amount_cents / 100) : "—"},
    {id: "state", title: "状态", align: "center", render: r => <Status value={r.state} />},
    {id: "cid", title: "官方码 ID", align: "number", render: r => r.code_id ?? "—"},
    {
      id: "act",
      title: "核对",
      align: "end",
      render: r =>
        r.state === "pending" || r.state === "unknown" ? (
          <Button size="xs" variant="outline" onClick={() => { reconcile.clearError(); setResolveCode(""); setResolving(r); }}>核对结果</Button>
        ) : r.request_id ? (
          <CopyButton value={r.request_id} label="复制请求编号" size="icon-xs" />
        ) : null,
    },
  ];

  return (
    <>
      <Panel title="签发一张">
        <Form
          busy={op.busy}
          onSubmit={() => void op.run(async () => {
            setSecret("");
            const amount = kind === "invitation" ? 0 : cents(value);
            const fp = `${kind}/${amount}`;
            const opID = pending.current?.input === fp ? pending.current.id : crypto.randomUUID();
            pending.current = {id: opID, input: fp};
            try {
              const res = await api("/issues", z.object({code: z.string()}), "POST", {operation_id: opID, kind, amount_cents: amount});
              setSecret(res.code);
              pending.current = null;
            } catch (err) {
              if (err instanceof ApiError && ["issue_rejected", "grant_exhausted", "invalid_input", "not_allowed"].includes(err.code)) {
                pending.current = null;
              }
              throw err;
            } finally {
              onChanged();
            }
          })}
        >
          <Toolbar>
            <Field label="码种">
              <Select value={kind} onChange={e => setKind(e.target.value as "invitation" | "balance")}>
                {config.invitation && <option value="invitation">注册邀请码</option>}
                <option value="balance">余额兑换码</option>
              </Select>
            </Field>
            {kind === "balance" && (
              <Field label="面额 (USD)">
                <Input inputMode="decimal" required value={value} onChange={e => setValue(e.target.value)} />
              </Field>
            )}
            <Button type="submit" busy={op.busy}><Ticket aria-hidden />签发一张</Button>
          </Toolbar>
          <ErrorLine value={op.error} />
        </Form>
        {secret && <CopyValue label="新签发的码" value={secret} />}
      </Panel>

      <Panel title="发码记录" padded={false}>
        <RemoteTable path="/issues" schema={issueSchema} columns={cols} rowID={r => r.operation_id} label="发码记录" revision={revision} />
      </Panel>

      <Overlay
        open={!!resolving}
        title="核对异常发码记录"
        onClose={() => setResolving(undefined)}
        busy={reconcile.busy}
        footer={
          <>
            <Button variant="outline" onClick={() => setResolving(undefined)}>取消</Button>
            <Button type="submit" form={formID} disabled={!resolveCode} busy={reconcile.busy}>确认已生成</Button>
            <Button variant="destructive" disabled={reconcile.busy} onClick={() => { refund.clearError(); setRefundOpen(true); }}>
              确认未生成并退额度
            </Button>
          </>
        }
      >
        {resolving && (
          <Form
            id={formID}
            busy={reconcile.busy}
            onSubmit={() => void reconcile.run(async () => {
              await api(`/admin/issues/${resolving.operation_id}/resolve`, empty, "POST", {code_id: Number(resolveCode)});
              setResolving(undefined);
              onChanged();
            })}
          >
            {resolving.request_id && <CopyValue label="关联请求编号" value={resolving.request_id} />}
            <Field label="已确认生成的官方码 ID">
              <Input type="number" min="1" value={resolveCode} onChange={e => setResolveCode(e.target.value)} />
            </Field>
            <ErrorLine value={reconcile.error} />
          </Form>
        )}
      </Overlay>

      <ConfirmDialog
        open={refundOpen}
        title="退回预扣发码额度"
        description="确认官方没有生成这张码后，预扣的额度会退回给签发人。"
        confirm="确认退回"
        busy={refund.busy}
        error={refund.error}
        onClose={() => setRefundOpen(false)}
        onConfirm={() => {
          if (!resolving) return;
          void refund.run(async () => {
            await api(`/admin/issues/${resolving.operation_id}/resolve`, empty, "POST", {code_id: null});
            setRefundOpen(false);
            setResolving(undefined);
            onChanged();
          });
        }}
      />
    </>
  );
}
