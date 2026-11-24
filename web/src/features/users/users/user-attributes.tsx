import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {named} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {Checkbox, EmptyState, Field, Form, FormSection, Input, Overlay, Resource, SaveBar, Select, Textarea, toast} from "@/ui";

const definition = named.extend({type: z.string(), description: z.string(), required: z.boolean(),
  options: z.array(z.object({value: z.string(), label: z.string()})).nullable()});
const valuesSchema = z.array(z.object({attribute_id: z.number(), value: z.string()}));
const choices = z.array(z.string());
type Definition = z.infer<typeof definition>;

export function UserAttributes({userID, onClose}: {userID: number; onClose: () => void}) {
  const path = "/admin/users/" + userID + "/attributes";
  const data = useResource(async () => {
    const [definitions, values] = await Promise.all([api("/admin/user-attributes", z.array(definition)), api(path, valuesSchema)]);
    return {definitions, values};
  }, path);
  return <Overlay open title="自定义属性" onClose={onClose}>
    <Resource state={data}>{value => value.definitions.length > 0
      ? <AttributeEditor path={path} definitions={value.definitions} initial={value.values} /> : <EmptyState title="尚未配置用户属性" />}</Resource>
  </Overlay>;
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
  if (item.type === "multi_select") {
    let selected: string[] = [];
    try { selected = choices.safeParse(JSON.parse(value)).data ?? []; } catch { /* Empty or obsolete attribute value. */ }
    return <FormSection title={item.name} description={item.description || undefined}>
      {options.map(option => <Checkbox key={option.value} label={option.label} checked={selected.includes(option.value)} onChange={checked => {
        const next = checked ? [...selected, option.value] : selected.filter(entry => entry !== option.value);
        onChange(JSON.stringify(next));
      }}>{option.label}</Checkbox>)}
    </FormSection>;
  }
  return <Field label={item.name} hint={item.description || undefined}>
    {item.type === "textarea" ? <Textarea value={value} required={item.required} onChange={event => onChange(event.target.value)} /> :
      item.type === "select" ? <Select value={value} required={item.required} onChange={event => onChange(event.target.value)}>
        <option value="">请选择</option>
        {options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
      </Select> : <Input type={["number", "email", "url", "date"].includes(item.type) ? item.type : "text"}
        step={item.type === "number" ? "any" : undefined} value={value} required={item.required} onChange={event => onChange(event.target.value)} />}
  </Field>;
}
