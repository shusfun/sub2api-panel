import {useId, useState} from "react";
import {ArrowDown, ArrowUp, Pencil, Plus, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction, useResource} from "@/lib/requests";
import {empty, userAttributeDefinition} from "@/schemas";
import {
  Badge, Button, ConfirmDialog, DataTable, EmptyState, ErrorLine, Field, Form, FormGrid,
  Input, Overlay, Resource, Select, Switch, TagInput, Toolbar, toast,
} from "@/ui";

type AttrDef = z.infer<typeof userAttributeDefinition>;

const TYPE_LABELS: Record<string, string> = {
  text: "文本",
  number: "数字",
  select: "下拉选项",
  boolean: "布尔开关",
};

export function UserAttributeSchemaDialog({open, onClose}: {open: boolean; onClose: () => void}) {
  const [revision, revise] = useState(0);
  const [editing, setEditing] = useState<AttrDef | "new">();
  const [deleting, setDeleting] = useState<AttrDef>();
  const list = useResource(() => api("/admin/user-attributes/all", z.array(userAttributeDefinition)), "user-attr-schema-all", open, revision);
  const op = useAction();

  const reload = (msg?: string) => {
    if (msg) toast(msg);
    revise(n => n + 1);
  };

  const move = (items: readonly AttrDef[], index: number, delta: -1 | 1) => {
    const target = index + delta;
    if (target < 0 || target >= items.length) return;
    const next = [...items];
    const [moved] = next.splice(index, 1);
    next.splice(target, 0, moved);
    void op.run(async () => {
      await api("/admin/user-attributes/reorder", empty, "PUT", {ids: next.map(i => i.id)});
      reload("属性排序已更新");
    });
  };

  return (
    <Overlay open={open} drawer title="自定义用户属性字段" onClose={onClose}>
      <Toolbar>
        <Button size="sm" onClick={() => setEditing("new")}><Plus aria-hidden />新增字段</Button>
      </Toolbar>
      <ErrorLine value={op.error} />
      <Resource state={list}>
        {items => (
          <DataTable
            label="自定义属性字段"
            rows={items}
            rowID={r => r.id}
            empty={<EmptyState title="尚未配置自定义属性字段" action={<Button size="sm" onClick={() => setEditing("new")}><Plus aria-hidden />新增字段</Button>} />}
            columns={[
              {
                id: "label",
                title: "字段名 / Key",
                render: r => (
                  <div>
                    <strong>{r.label || r.name}</strong>
                    {r.key && <div className="muted code-mono text-fine">{r.key}</div>}
                  </div>
                ),
              },
              {id: "type", title: "类型", render: r => <Badge variant="soft" tone="neutral">{TYPE_LABELS[r.type] ?? r.type}</Badge>},
              {id: "req", title: "必填", align: "center", render: r => r.required ? <Badge variant="soft" tone="warn">必填</Badge> : <span className="muted">—</span>},
              {
                id: "enabled",
                title: "启用",
                align: "center",
                render: r => (
                  <Switch
                    checked={r.enabled}
                    label={`启用 ${r.label || r.name}`}
                    onChange={checked => void op.run(async () => {
                      await api(`/admin/user-attributes/${r.id}`, userAttributeDefinition, "PUT", {
                        key: r.key || r.name,
                        label: r.label || r.name,
                        type: r.type,
                        description: r.description,
                        required: r.required,
                        enabled: checked,
                        options: r.options ?? [],
                      });
                      reload(checked ? "已启用字段" : "已停用字段");
                    })}
                  />
                ),
              },
              {
                id: "act",
                title: "操作",
                align: "end",
                render: r => {
                  const idx = items.findIndex(i => i.id === r.id);
                  return (
                    <div className="row-actions">
                      <Button size="icon-sm" variant="ghost" title="上移" aria-label="上移" disabled={idx <= 0 || op.busy} onClick={() => move(items, idx, -1)}><ArrowUp aria-hidden /></Button>
                      <Button size="icon-sm" variant="ghost" title="下移" aria-label="下移" disabled={idx < 0 || idx === items.length - 1 || op.busy} onClick={() => move(items, idx, 1)}><ArrowDown aria-hidden /></Button>
                      <Button size="icon-sm" variant="ghost" title="编辑" aria-label="编辑" onClick={() => setEditing(r)}><Pencil aria-hidden /></Button>
                      <Button size="icon-sm" variant="ghost" title="删除" aria-label="删除" onClick={() => setDeleting(r)}><Trash2 aria-hidden /></Button>
                    </div>
                  );
                },
              },
            ]}
          />
        )}
      </Resource>
      {editing && (
        <AttrFormDialog
          initial={editing === "new" ? undefined : editing}
          onClose={() => setEditing(undefined)}
          onSaved={() => { setEditing(undefined); reload(editing === "new" ? "已创建属性字段" : "已更新属性字段"); }}
        />
      )}
      <ConfirmDialog
        open={!!deleting}
        title="删除属性字段"
        description={deleting ? `确定删除自定义属性「${deleting.label || deleting.name}」吗？` : ""}
        confirm="删除"
        busy={op.busy}
        error={op.error}
        onClose={() => setDeleting(undefined)}
        onConfirm={() => deleting && void op.run(async () => {
          await api(`/admin/user-attributes/${deleting.id}`, empty, "DELETE");
          setDeleting(undefined);
          reload("属性字段已删除");
        })}
      />
    </Overlay>
  );
}

function AttrFormDialog({initial, onClose, onSaved}: {initial?: AttrDef; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const op = useAction();
  const [type, setType] = useState(initial?.type || "text");
  const [required, setRequired] = useState(initial?.required ?? false);
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [options, setOptions] = useState<string[]>(() => (initial?.options ?? []).map(o => o.value));

  return (
    <Overlay
      open
      title={initial ? "编辑属性字段" : "新增属性字段"}
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={op.busy}>保存</Button>
        </>
      }
    >
      <Form
        id={formID}
        busy={op.busy}
        onSubmit={event => {
          const fd = new FormData(event.currentTarget);
          const payload = {
            key: String(fd.get("key") ?? "").trim(),
            label: String(fd.get("label") ?? "").trim(),
            type,
            description: String(fd.get("description") ?? "").trim(),
            required,
            enabled,
            options: type === "select" ? options.map(v => ({value: v, label: v})) : [],
          };
          void op.run(async () => {
            if (initial) {
              await api(`/admin/user-attributes/${initial.id}`, userAttributeDefinition, "PUT", payload);
            } else {
              await api("/admin/user-attributes", userAttributeDefinition, "POST", payload);
            }
            onSaved();
          });
        }}
      >
        <FormGrid>
          <Field label="字段标识（Key）"><Input name="key" defaultValue={initial?.key ?? ""} placeholder="例如 company_name" required /></Field>
          <Field label="显示名称（Label）"><Input name="label" defaultValue={initial?.label || initial?.name || ""} placeholder="例如 所属公司" required /></Field>
          <Field label="字段类型">
            <Select value={type} onChange={e => setType(e.target.value)}>
              <option value="text">文本（text）</option>
              <option value="number">数字（number）</option>
              <option value="select">下拉选项（select）</option>
              <option value="boolean">布尔开关（boolean）</option>
            </Select>
          </Field>
          <Field label="说明（选填）"><Input name="description" defaultValue={initial?.description ?? ""} /></Field>
        </FormGrid>
        {type === "select" && (
          <Field label="可选项（回车添加）">
            <TagInput value={options} placeholder="输入选项值后回车" onChange={setOptions} />
          </Field>
        )}
        <FormGrid>
          <Switch checked={required} label="设为必填项" onChange={setRequired} />
          <Switch checked={enabled} label="启用该字段" onChange={setEnabled} />
        </FormGrid>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
