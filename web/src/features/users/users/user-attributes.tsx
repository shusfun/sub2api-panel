import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {userAttributeDefinition} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {Checkbox, EmptyState, Field, Form, FormSection, Input, Overlay, Resource, SaveBar, Select, Switch, Textarea, toast} from "@/ui";

const valuesSchema = z.array(z.object({attribute_id: z.number(), value: z.string()}));
const choices = z.array(z.string());
type Definition = z.infer<typeof userAttributeDefinition>;

export function UserAttributes({userID, onClose, inline}: {userID: number; onClose?: () => void; inline?: boolean}) {
  const path = "/admin/users/" + userID + "/attributes";
  const data = useResource(async () => {
    const [definitions, values] = await Promise.all([api("/admin/user-attributes", z.array(userAttributeDefinition)), api(path, valuesSchema)]);
    return {definitions, values};
  }, path);
  const content = (
    <Resource state={data}>
      {value => value.definitions.length > 0
        ? <AttributeEditor path={path} definitions={value.definitions} initial={value.values} />
        : <EmptyState title="尚未配置用户属性" />}
    </Resource>
  );
  if (inline) return content;
  return <Overlay open title="自定义属性" onClose={() => onClose?.()}>{content}</Overlay>;
}

function AttributeEditor({path, definitions, initial}: {path: string; definitions: Definition[]; initial: z.infer<typeof valuesSchema>}) {
  const [values, setValues] = useState(() => Object.fromEntries(initial.map(item => [item.attribute_id, item.value])));
  const [draft, setDraft] = useState<Record<number, string>>({});
  const action = useAction();
  const dirty = Object.keys(draft).length > 0;
  return <Form busy={action.busy} errors={action.fieldErrors} onSubmit={() => void action.run(async () => {
    const result = await api(path, valuesSchema, "PUT", {values: draft});
    setValues(Object.fromEntries(result.map(item => [item.attribute_id, item.value])));
    setDraft({});
    toast("用户属性已保存");
  })}>
    {definitions.map(item => <AttributeField key={item.id} item={item} value={draft[item.id] ?? values[item.id] ?? ""} onChange={value => {
      setDraft(previous => {
        const next = {...previous};
        if (value === (values[item.id] ?? "")) delete next[item.id];
        else next[item.id] = value;
        return next;
      });
    }} />)}
    <SaveBar dirty={dirty} dirtyCount={Object.keys(draft).length} busy={action.busy} error={action.error} submitLabel="保存属性" onReset={() => { setDraft({}); action.clearError(); }} />
  </Form>;
}

function AttributeField({item, value, onChange}: {item: Definition; value: string; onChange: (value: string) => void}) {
  const options = item.options ?? [];
  const label = item.label || item.name;
  if (item.type === "boolean") {
    return <Switch checked={value === "true"} label={label} onChange={checked => onChange(checked ? "true" : "false")} />;
  }
  if (item.type === "multi_select") {
    let selected: string[] = [];
    try { selected = choices.safeParse(JSON.parse(value)).data ?? []; } catch { /* Empty or obsolete attribute value. */ }
    return <FormSection title={label} description={item.description || undefined}>
      {options.map(option => <Checkbox key={option.value} label={option.label} checked={selected.includes(option.value)} onChange={checked => {
        const next = checked ? [...selected, option.value] : selected.filter(entry => entry !== option.value);
        onChange(JSON.stringify(next));
      }}>{option.label}</Checkbox>)}
    </FormSection>;
  }
  return <Field label={label} hint={item.description || undefined}>
    {item.type === "textarea" ? <Textarea value={value} required={item.required} onChange={event => onChange(event.target.value)} /> :
      item.type === "select" ? <Select value={value} required={item.required} onChange={event => onChange(event.target.value)}>
        <option value="">请选择</option>
        {options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
      </Select> : <Input type={["number", "email", "url", "date"].includes(item.type) ? item.type : "text"}
        step={item.type === "number" ? "any" : undefined} value={value} required={item.required} onChange={event => onChange(event.target.value)} />}
  </Field>;
}
