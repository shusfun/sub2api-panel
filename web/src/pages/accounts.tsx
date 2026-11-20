import {useState} from "react";
import type {z} from "zod";
import {account, accountQuota, api, empty, testAccount} from "../api";
import {date, percent} from "../format";
import {useAction, useResource} from "../requests";
import {Button, DataTable, ErrorLine, Feedback, Field, Form, Input, Overlay, Resource, Section, Select, Status, Toolbar, type Column} from "../ui";
import {RemoteTable} from "./remote-table";

type Account = z.infer<typeof account>;
type QuotaWindow = {label: string; window: z.infer<typeof accountQuota>["five_hours"]};
const quotaColumns: Column<QuotaWindow>[] = [
  {id: "window", title: "窗口", render: row => row.label},
  {id: "used", title: "已用", align: "number", render: row => percent(row.window?.percent_used ?? null)},
  {id: "reset", title: "重置时间", render: row => row.window?.resets_at ? date(row.window.resets_at) : "未知"},
];

export function Accounts() {
  const [query, setQuery] = useState("");
  const [revision, revise] = useState(0);
  const [selected, select] = useState<Account>();
  const columns: Column<Account>[] = [
    {id: "account", title: "账号", render: row => row.name},
    {id: "platform", title: "平台", render: row => row.platform},
    {id: "status", title: "状态", render: row => <Status value={row.status} />},
    {id: "fiveHours", title: "5 小时用量", align: "number", render: row => percent(row.five_hours_used)},
    {id: "week", title: "7 天用量", align: "number", render: row => percent(row.week_used)},
    {id: "scheduling", title: "调度", render: row => <Scheduling row={row} onChange={() => revise(previous => previous + 1)} />},
    {id: "detail", title: "操作", align: "end", render: row => <Button variant="ghost" size="sm" onClick={() => select(row)}>额度与测活</Button>},
  ];
  return <Section>
    <Form onSubmit={event => {
      const params = new URLSearchParams();
      new FormData(event.currentTarget).forEach((value, key) => { if (typeof value === "string" && value) params.set(key, value); });
      setQuery(params.toString());
    }}>
      <Toolbar className="filter-toolbar">
        <Field label="账号名称"><Input type="search" name="search" maxLength={100} /></Field>
        <Field label="平台"><Select name="platform"><option value="">全部平台</option><option value="openai">OpenAI</option>
          <option value="anthropic">Anthropic</option><option value="gemini">Gemini</option><option value="grok">Grok</option></Select></Field>
        <Field label="状态"><Select name="status"><option value="">全部状态</option><option value="active">正常</option>
          <option value="inactive">停用</option><option value="error">异常</option></Select></Field>
        <Button type="submit">筛选</Button>
      </Toolbar>
    </Form>
    <RemoteTable path={"/admin/accounts?" + query} schema={account} columns={columns} rowID={row => row.id} label="上游账号" revision={revision} />
    <Overlay open={!!selected} onClose={() => select(undefined)} title={selected?.name ?? "账号详情"} drawer>
      {selected && <AccountDetail key={selected.id} id={selected.id} />}
    </Overlay>
  </Section>;
}

function Scheduling({row, onChange}: {row: Account; onChange: () => void}) {
  const action = useAction();
  const [expected, expect] = useState<boolean>();
  const awaitingReadback = expected !== undefined && expected !== row.schedulable;
  return <div className="row-feedback">
    <Button variant="outline" size="sm" busy={action.busy || awaitingReadback} busyText="正在更新…" aria-pressed={row.schedulable}
      onClick={() => void action.run(async () => {
        await api(`/admin/accounts/${row.id}/scheduling`, empty, "PUT", {schedulable: !row.schedulable});
        expect(!row.schedulable);
        onChange();
      })}>{row.schedulable ? "调度中" : "已停调"}</Button>
    <ErrorLine value={action.error} />
    {expected !== undefined && !awaitingReadback && !action.busy && !action.error && <Feedback tone="success">已更新</Feedback>}
  </div>;
}

function AccountDetail({id}: {id: number}) {
  const quota = useResource(() => api(`/admin/accounts/${id}/quota`, accountQuota), id);
  const action = useAction();
  const [progress, setProgress] = useState("");
  return <>
    <Resource state={quota}>{data => <DataTable label="额度窗口" columns={quotaColumns} rowID={row => row.label}
      rows={[{label: "5 小时", window: data.five_hours}, {label: "7 天", window: data.week}]} />}</Resource>
    <Form busy={action.busy} onSubmit={event => {
      const model = String(new FormData(event.currentTarget).get("model") ?? "").trim();
      void action.run(async () => {
        setProgress("");
        await testAccount(id, model, setProgress);
      });
    }}>
      <Toolbar><Field label="测试模型（可选）"><Input name="model" maxLength={128} /></Field>
        <Button type="submit" busy={action.busy}>测活</Button></Toolbar>
      <ErrorLine value={action.error} />
      {progress === "running" && <Feedback>正在测试…</Feedback>}
      {progress === "passed" && !action.error && <Feedback tone="success">测活通过</Feedback>}
      {progress === "failed" && !action.error && <Feedback tone="error">测活失败</Feedback>}
    </Form>
  </>;
}
