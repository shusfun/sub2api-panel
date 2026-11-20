import {useState} from "react";
import type {z} from "zod";
import {api, empty, grantSchema, subscriptions, userRow} from "../api";
import {cents, money} from "../format";
import {useAction, useResource} from "../requests";
import {Button, ConfirmDialog, ErrorLine, Feedback, Field, Form, FormGrid, Input, Overlay, Resource, Section, Segmented, Status, Toolbar, type Column} from "../ui";
import {subscriptionColumns, UserUsageTable} from "./records";
import {RemoteTable} from "./remote-table";

type User = z.infer<typeof userRow>;
const tabs = [["grant", "发码授权"], ["subscriptions", "订阅"], ["usage", "用量"]] as const;

export function Users() {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<User>();
  const [view, setView] = useState<(typeof tabs)[number][0]>("grant");
  const columns: Column<User>[] = [
    {id: "user", title: "用户", render: row => row.email},
    {id: "status", title: "状态", render: row => <Status value={row.status} />},
    {id: "balance", title: "余额", align: "number", render: row => money(row.balance)},
    {id: "details", title: "操作", align: "end", render: row => <Button size="sm" variant="ghost" onClick={() => {
      setSelected(row);
      setView("grant");
    }}>查看</Button>},
  ];
  return <Section>
    <Form onSubmit={event => setSearch(String(new FormData(event.currentTarget).get("search") ?? "").trim())}>
      <Toolbar><Field label="邮箱或用户名"><Input name="search" type="search" maxLength={100} /></Field>
        <Button type="submit">搜索</Button></Toolbar>
    </Form>
    <RemoteTable path={"/admin/users?search=" + encodeURIComponent(search)} schema={userRow} columns={columns} rowID={row => row.id} label="用户列表" />
    <Overlay open={!!selected} onClose={() => setSelected(undefined)} title={selected?.email ?? "用户详情"} drawer>
      {selected && <>
        <Segmented label="用户详情" value={view} options={tabs} onChange={setView} />
        {view === "grant" && <GrantEditor key={selected.id} user={selected.id} />}
        {view === "usage" && <UserUsageTable user={selected.id} />}
        {view === "subscriptions" && <RemoteTable path={`/admin/users/${selected.id}/subscriptions`} schema={subscriptions}
          columns={subscriptionColumns} rowID={row => row.id} label="用户订阅" />}
      </>}
    </Overlay>
  </Section>;
}

function GrantEditor({user}: {user: number}) {
  const path = "/admin/grants/" + user;
  const [revision, revise] = useState(0);
  const [confirming, confirm] = useState(false);
  const [result, setResult] = useState("");
  const current = useResource(() => api(path, grantSchema), path, true, revision);
  const save = useAction();
  const revoke = useAction();
  return <>
    <Resource state={current}>{record => <>
      <p className="muted">剩余 {record.invites} 张邀请码 · 余额码额度 {money(record.budget_cents / 100)}</p>
      <Form key={revision} busy={save.busy} onSubmit={event => {
        const fields = new FormData(event.currentTarget);
        void save.run(async () => {
          setResult("");
          const entries = String(fields.get("amounts") || "").split(/[,，]/).map(text => text.trim()).filter(Boolean);
          await api(path, empty, "PUT", {invites: Number(fields.get("invites")), budget_cents: cents(String(fields.get("budget") || "0")),
            amounts_cents: entries.map(cents), totp: fields.get("totp")});
          revise(previous => previous + 1);
          setResult("已保存授权");
        });
      }}>
        <FormGrid>
          <Field label="增加邀请码"><Input type="number" name="invites" min="0" max="10000" defaultValue={0} required /></Field>
          <Field label="增加余额码额度（USD）"><Input name="budget" inputMode="decimal" defaultValue="0" required /></Field>
          <Field label="允许面额（逗号分隔）"><Input name="amounts" key={record.amounts_cents.join(",")}
            defaultValue={record.amounts_cents.map(amount => amount / 100).join(", ")} /></Field>
          <Field label="管理员验证码"><Input name="totp" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required /></Field>
        </FormGrid>
        <ErrorLine value={save.error} />
        {result && <Feedback tone="success">{result}</Feedback>}
        <div className="form-actions">
          <Button variant="outline" disabled={save.busy} onClick={() => { revoke.clearError(); confirm(true); }}>撤销授权</Button>
          <Button type="submit" busy={save.busy}>保存授权</Button>
        </div>
      </Form>
    </>}</Resource>
    <ConfirmDialog open={confirming} title="撤销发码授权" description="此用户的发码授权和可用额度都将清除。"
      confirm="撤销授权" busy={revoke.busy} error={revoke.error} onClose={() => confirm(false)} onConfirm={() => void revoke.run(async () => {
        save.clearError();
        setResult("");
        await api(path, empty, "DELETE");
        confirm(false);
        revise(previous => previous + 1);
        setResult("已撤销授权");
      })} />
  </>;
}
