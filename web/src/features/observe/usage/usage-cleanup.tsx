import {useId, useState} from "react";
import {Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date} from "@/lib/format";
import {useAction} from "@/lib/requests";
import {
  Button, ErrorLine, Feedback, Field, Form, FormGrid, Input, Overlay, RefreshButton, RemoteTable, Status, toast, type Column,
} from "@/ui";

const base = "/admin/usage/cleanup-tasks";
const taskSchema = z.object({
  id: z.number(),
  status: z.string(),
  filters: z.object({
    start_time: z.string(), end_time: z.string(), user_id: z.number().nullish(), api_key_id: z.number().nullish(),
    account_id: z.number().nullish(), group_id: z.number().nullish(), model: z.string().nullish(),
  }),
  deleted_rows: z.number(),
  started_at: z.string().nullish(),
  finished_at: z.string().nullish(),
  created_at: z.string(),
});
const cancelResult = z.object({id: z.number(), status: z.string()});
type Task = z.infer<typeof taskSchema>;

const scopeText = (f: Task["filters"]) => [
  f.user_id ? `用户 #${f.user_id}` : "", f.api_key_id ? `Key #${f.api_key_id}` : "", f.account_id ? `账号 #${f.account_id}` : "",
  f.group_id ? `分组 #${f.group_id}` : "", f.model ?? "",
].filter(Boolean).join(" · ") || "全部记录";

export function UsageCleanupPanel() {
  const [revision, revise] = useState(0);
  const [open, setOpen] = useState(false);
  const action = useAction();
  const refresh = () => revise(v => v + 1);

  const columns: Column<Task>[] = [
    {id: "id", title: "任务", render: row => <span className="code-mono text-fine">#{row.id}</span>},
    {id: "range", title: "清理区间", render: row => <span className="code-mono text-fine">{date(row.filters.start_time)} – {date(row.filters.end_time)}</span>},
    {id: "scope", title: "范围", hideWhenCompact: true, render: row => scopeText(row.filters)},
    {id: "rows", title: "已删除行数", align: "number", render: row => row.deleted_rows},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {id: "at", title: "创建时间", hideWhenNarrow: true, render: row => date(row.created_at)},
    {
      id: "ops", title: "操作", align: "end",
      render: row => (row.status === "pending" || row.status === "running") && (
        <Button size="xs" variant="outline" busy={action.busy} onClick={() => void action.run(async () => {
          await api(`${base}/${row.id}/cancel`, cancelResult, "POST", {});
          toast("已取消清理任务");
          refresh();
        })}>取消</Button>
      ),
    },
  ];

  return (
    <section className="swift-group" aria-label="用量清理任务">
      <div className="swift-group-head">
        <span className="swift-group-title">用量清理任务</span>
        <div className="swift-group-actions">
          <Button size="xs" variant="outline" onClick={() => setOpen(true)}><Trash2 aria-hidden />新建清理任务</Button>
          <RefreshButton size="xs" variant="ghost" onClick={refresh} />
        </div>
      </div>
      <ErrorLine value={action.error} />
      <RemoteTable path={base} schema={taskSchema} columns={columns} rowID={row => row.id} label="用量清理任务" revision={revision} />
      {open && <CreateDialog onClose={() => setOpen(false)} onCreated={() => { setOpen(false); toast("清理任务已创建"); refresh(); }} />}
    </section>
  );
}

function CreateDialog({onClose, onCreated}: {onClose: () => void; onCreated: () => void}) {
  const formID = useId();
  const op = useAction();
  const [operationID] = useState(() => crypto.randomUUID());
  const optionalID = (fd: FormData, name: string) => {
    const raw = String(fd.get(name) ?? "").trim();
    return raw ? {[name]: Number(raw)} : {};
  };
  return (
    <Overlay open title="新建用量清理任务" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} variant="destructive" busy={op.busy}>创建并开始清理</Button>
      </>
    }>
      <Form id={formID} busy={op.busy} onSubmit={e => {
        const fd = new FormData(e.currentTarget);
        const model = String(fd.get("model") ?? "").trim();
        void op.run(async () => {
          await api(base, taskSchema, "POST", {
            start_date: String(fd.get("start_date") ?? ""),
            end_date: String(fd.get("end_date") ?? ""),
            ...optionalID(fd, "user_id"), ...optionalID(fd, "account_id"), ...optionalID(fd, "group_id"), ...optionalID(fd, "api_key_id"),
            ...(model ? {model} : {}),
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            operation_id: operationID,
          });
          onCreated();
        });
      }}>
        <Feedback>清理会永久删除所选区间内的用量记录，无法恢复。上游账本中依赖这些记录的本站侧统计会随之重算，历史金额可能变化。</Feedback>
        <FormGrid>
          <Field label="开始日期"><Input name="start_date" type="date" required /></Field>
          <Field label="结束日期（含）"><Input name="end_date" type="date" required /></Field>
          <Field label="用户 ID（可选）"><Input name="user_id" type="number" min="1" /></Field>
          <Field label="API Key ID（可选）"><Input name="api_key_id" type="number" min="1" /></Field>
          <Field label="上游账号 ID（可选）"><Input name="account_id" type="number" min="1" /></Field>
          <Field label="分组 ID（可选）"><Input name="group_id" type="number" min="1" /></Field>
        </FormGrid>
        <Field label="模型（可选）"><Input name="model" maxLength={200} /></Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
