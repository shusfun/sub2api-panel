import {useRef, useState} from "react";
import {Plus} from "lucide-react";
import {z} from "zod";
import {api, empty, keyRow, named} from "../api";
import {useAction, useResource} from "../requests";
import {Button, ConfirmDialog, CopyValue, ErrorLine, Field, Form, Input, Overlay, Resource, Section, Select, Status, Toolbar, type Column} from "../ui";
import {RemoteTable} from "./remote-table";

const keySchema = z.object({key: z.string().min(1)});
type Key = z.infer<typeof keyRow>;
type Editor = {type: "create"} | {type: "rename"; row: Key};

export function Keys() {
  const [revision, revise] = useState(0);
  const [editor, edit] = useState<Editor>();
  const [deleting, setDeleting] = useState<Key>();
  const [secret, setSecret] = useState("");
  const createButton = useRef<HTMLButtonElement>(null);
  const editing = useAction();
  const removal = useAction();
  const toggle = useAction();
  const creating = editor?.type === "create";
  const groups = useResource(() => api("/key-groups", z.array(named)), "groups", creating);
  const refresh = () => revise(value => value + 1);
  const columns: Column<Key>[] = [
    {id: "name", title: "名称", render: row => row.name},
    {id: "key", title: "Key", render: row => <code>{row.masked}</code>},
    {id: "status", title: "状态", render: row => <Status value={row.status} />},
    {id: "actions", title: "操作", align: "end", render: row => <div className="row-actions">
      <Button variant="ghost" size="sm" render={<a href={"#usage?key=" + row.id} />}>查看用量</Button>
      <Button variant="ghost" size="sm" onClick={() => { editing.clearError(); edit({type: "rename", row}); }}>改名</Button>
      <Button variant="ghost" size="sm" disabled={toggle.busy} onClick={() => void toggle.run(async () => {
        await api("/keys/" + row.id, empty, "PUT", {status: row.status === "active" ? "inactive" : "active"});
        refresh();
      })}>{row.status === "active" ? "停用" : "启用"}</Button>
      <Button variant="ghost" size="sm" onClick={() => { removal.clearError(); setDeleting(row); }}>删除</Button>
    </div>},
  ];

  return <Section>
    <Toolbar className="section-heading"><h2>密钥列表</h2>
      <Button ref={createButton} onClick={() => { editing.clearError(); setSecret(""); edit({type: "create"}); }}><Plus aria-hidden />创建 API Key</Button>
    </Toolbar>
    {secret && <CopyValue label="新 API Key" value={secret} />}
    <ErrorLine value={toggle.error} />
    <RemoteTable path="/keys" schema={keyRow} columns={columns} rowID={row => row.id} label="API Key" revision={revision} />
    <Overlay open={!!editor} title={creating ? "创建 API Key" : "修改密钥名称"} onClose={() => edit(undefined)} busy={editing.busy}>
      {editor && <Form busy={editing.busy} onSubmit={event => {
        const fields = new FormData(event.currentTarget);
        const name = fields.get("name");
        void editing.run(async () => {
          if (editor.type === "create") {
            const group = fields.get("group");
            const result = await api("/keys", keySchema, "POST", {name, ...(group ? {group_id: Number(group)} : {})});
            setSecret(result.key);
          } else {
            await api("/keys/" + editor.row.id, empty, "PUT", {name});
          }
          edit(undefined);
          refresh();
        });
      }}>
        <Field label="名称"><Input name="name" required maxLength={100} defaultValue={editor.type === "rename" ? editor.row.name : ""} /></Field>
        {creating && <Resource state={groups}>{data => <Field label="调用分组">
          <Select name="group" defaultValue={data[0]?.id ?? ""}><option value="">暂不分组</option>
            {data.map(group => <option key={group.id} value={group.id}>{group.name}</option>)}
          </Select>
        </Field>}</Resource>}
        <ErrorLine value={editing.error} />
        <div className="form-actions">
          <Button variant="outline" onClick={() => edit(undefined)}>取消</Button>
          <Button type="submit" disabled={creating && groups?.value === undefined} busy={editing.busy}>{creating ? "创建" : "保存"}</Button>
        </div>
      </Form>}
    </Overlay>
    <ConfirmDialog open={!!deleting} title="删除 API Key" description={`删除「${deleting?.name ?? ""}」后将无法再用此 Key 发起请求。`}
      confirm="删除" busy={removal.busy} error={removal.error} onClose={() => setDeleting(undefined)}
      finalFocus={createButton} onConfirm={() => {
        if (!deleting) return;
        void removal.run(async () => {
          await api("/keys/" + deleting.id, empty, "DELETE");
          setDeleting(undefined);
          refresh();
        });
      }} />
  </Section>;
}
