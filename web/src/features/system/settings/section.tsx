import {useEffect, useMemo, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Badge, Button, ErrorLine, Field, FilePicker, Form, FormSection, InlineStack, Input, InsetRow, ListInput, Resource, RowsEditor,
  SaveBar, SecretInput, Select, StepUpDialog, Switch, Textarea, toast, type RowColumn,
} from "@/ui";

const option = z.object({value: z.string(), label: z.string()});
const column = z.object({key: z.string(), kind: z.string(), label: z.string(), options: z.array(option).optional()});
const fieldSchema = z.object({
  key: z.string(), kind: z.string(), label: z.string(), hint: z.string().optional(),
  min: z.number().optional(), max: z.number().optional(), options: z.array(option).optional(),
  when: z.string().optional(), read_only: z.boolean().optional(), columns: z.array(column).optional(),
  value: z.unknown().optional(), configured: z.boolean().optional(),
});
const sectionSchema = z.object({
  id: z.string(), title: z.string(), hint: z.string().optional(), step_up: z.boolean(),
  groups: z.array(z.object({title: z.string().optional(), hint: z.string().optional(), when: z.string().optional(), fields: z.array(fieldSchema)})).optional(),
});
type SettingField = z.infer<typeof fieldSchema>;
type Row = Record<string, unknown>;

const quotaColumns: RowColumn[] = [
  {key: "platform", kind: "text", label: "平台"}, {key: "daily", kind: "optfloat", label: "每日"},
  {key: "weekly", kind: "optfloat", label: "每周"}, {key: "monthly", kind: "optfloat", label: "每月"},
];
const numberColumns: RowColumn[] = [{key: "platform", kind: "text", label: "平台"}, {key: "value", kind: "float", label: "数值"}];

const toRows = (value: unknown, single: boolean): Row[] => Object.entries((value ?? {}) as Record<string, unknown>)
  .map(([platform, item]) => single ? {platform, value: item} : {platform, ...(item as Row)});
const fromRows = (rows: Row[], single: boolean) => Object.fromEntries(rows.filter(row => String(row.platform ?? "").trim())
  .map(row => [String(row.platform).trim(), single ? Number(row.value ?? 0) : {daily: row.daily ?? null, weekly: row.weekly ?? null, monthly: row.monthly ?? null}]));

function SiteLogoInput({value, onChange}: {value: string; onChange: (value: string) => void}) {
  const action = useAction();
  const [files, setFiles] = useState<File[]>([]);
  return (
    <div className="stack">
      <InlineStack>
        <img src={value || "/favicon.svg"} width={47} height={47} alt="站点图标预览" />
        <Input
          aria-label="图标来源"
          value={value.startsWith("data:") ? "已上传内联图片" : value}
          onChange={event => onChange(event.target.value)}
        />
        {value && <Button type="button" variant="outline" size="sm" busy={action.busy} onClick={() => { setFiles([]); onChange(""); }}>移除</Button>}
      </InlineStack>
      <FilePicker
        accept="image/png,image/jpeg,image/webp,image/x-icon"
        maxBytes={20 * 1024}
        hint="支持 PNG、JPEG、WebP 或 ICO，最大 20 KiB"
        value={files}
        onChange={picked => {
          setFiles(picked);
          const file = picked[0];
          if (!file) return;
          void action.run(async () => {
            const url = await new Promise<string>((resolve, reject) => {
              const reader = new FileReader();
              reader.onerror = () => reject(new Error("读取图标失败"));
              reader.onload = () => typeof reader.result === "string" ? resolve(reader.result) : reject(new Error("读取图标失败"));
              reader.readAsDataURL(file);
            });
            onChange(url);
          });
        }}
      />
      <ErrorLine value={action.error} />
    </div>
  );
}

function FieldControl({field, value, onChange}: {field: SettingField; value: unknown; onChange: (value: unknown) => void}) {
  if (field.key === "site_logo") return <SiteLogoInput value={String(value ?? "")} onChange={onChange} />;
  switch (field.kind) {
    case "bool":
      return <Switch label={field.label} checked={value === true} onChange={onChange} />;
    case "int": case "float":
      return <div className="row-field"><Input aria-label={field.label} type="number" step={field.kind === "float" ? "any" : "1"} min={field.min} max={field.max}
        value={String(value ?? 0)} onChange={event => onChange(event.target.value === "" ? 0 : Number(event.target.value))} /></div>;
    case "select":
      return <div className="row-field"><Select aria-label={field.label} value={String(value ?? "")} onChange={event => onChange(event.target.value)}>
        {!field.options?.some(item => item.value === String(value ?? "")) && <option value={String(value ?? "")}>{String(value ?? "")}</option>}
        {field.options?.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
      </Select></div>;
    case "long":
      return <Textarea aria-label={field.label} rows={5} value={String(value ?? "")} onChange={event => onChange(event.target.value)} />;
    case "secret":
      return <SecretInput label={field.label} configured={field.configured} value={String(value ?? "")} onChange={onChange} />;
    case "list": case "intlist":
      return <ListInput label={field.label} numeric={field.kind === "intlist"} value={Array.isArray(value) ? value as (string | number)[] : []} onChange={onChange} />;
    case "rows":
      return <RowsEditor label={field.label} columns={field.columns ?? []} value={Array.isArray(value) ? value as Row[] : []} onChange={onChange} />;
    case "quotas": case "numbers": {
      const single = field.kind === "numbers";
      return <RowsEditor label={field.label} columns={single ? numberColumns : quotaColumns} value={toRows(value, single)} onChange={rows => onChange(fromRows(rows, single))} />;
    }
    case "info":
      return typeof value === "boolean"
        ? <Badge variant="soft" tone={value ? "ok" : "neutral"}>{value ? "已配置" : "未配置"}</Badge>
        : <span className="code-mono">{String(value ?? "") || "—"}</span>;
    default:
      return <Input aria-label={field.label} value={String(value ?? "")} onChange={event => onChange(event.target.value)} />;
  }
}

export function SettingSection({id, onDirty}: {id: string; onDirty: (dirty: boolean) => void}) {
  const [revision, revise] = useState(0);
  const section = useResource(() => api(`/admin/settings/${id}`, sectionSchema), `setting-section:${id}`, true, revision);
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [confirming, setConfirming] = useState(false);
  const save = useAction();
  const dirty = Object.keys(draft).length > 0;
  useEffect(() => { onDirty(dirty); }, [dirty, onDirty]);
  useEffect(() => () => onDirty(false), [onDirty]);

  const fields = useMemo(() => section?.value?.groups?.flatMap(group => group.fields) ?? [], [section?.value]);
  const current = (key: string) => key in draft ? draft[key] : fields.find(field => field.key === key)?.value;
  const shown = (when?: string) => !when || current(when) === true;
  const change = (field: SettingField, value: unknown) => {
    setDraft(previous => {
      const next = {...previous, [field.key]: value};
      const same = field.kind === "secret" ? value === "" : JSON.stringify(value) === JSON.stringify(field.value);
      if (same) delete next[field.key];
      return next;
    });
  };
  const submit = (totp?: string) => save.run(async () => {
    await api(`/admin/settings/${id}`, empty, "PUT", {values: draft, ...(totp ? {totp} : {})});
    setDraft({});
    setConfirming(false);
    toast("站点设置已保存");
    if (["site", "signup", "security", "oauth", "features", "payment", "agreement"].includes(id)) {
      window.dispatchEvent(new Event("panel-site-settings-saved"));
    }
    revise(n => n + 1);
  });

  return <Resource state={section}>{value => <>
    <Form busy={save.busy} errors={save.fieldErrors} className="setting-fields" noValidate onSubmit={() => {
      if (!dirty) return;
      if (value.step_up) { save.clearError(); setConfirming(true); }
      else void submit();
    }}>
      {value.groups?.filter(group => shown(group.when)).map((group, index) => (
        <FormSection key={group.title ?? index} title={group.title} description={group.hint}>
          {group.fields.filter(field => shown(field.when)).map(field => {
            const control = <FieldControl field={field} value={current(field.key)} onChange={next => change(field, next)} />;
            if (field.kind === "bool" || field.kind === "info") {
              return <InsetRow key={field.key} label={field.label} hint={field.hint}>{control}</InsetRow>;
            }
            if (["rows", "quotas", "numbers"].includes(field.kind)) {
              return <FormSection key={field.key} title={field.label} description={field.hint}>{control}</FormSection>;
            }
            return <Field key={field.key} label={field.label} hint={field.hint} name={field.key}>{control}</Field>;
          })}
        </FormSection>
      ))}
      {!!value.groups?.length && (
        <SaveBar
          dirty={dirty}
          dirtyCount={Object.keys(draft).length}
          busy={save.busy}
          error={!confirming ? save.error : ""}
          onReset={() => { setDraft({}); save.clearError(); }}
        />
      )}
    </Form>
    {confirming && (
      <StepUpDialog
        title="确认保存敏感设置"
        labels={Object.keys(draft).map(key => fields.find(field => field.key === key)?.label ?? key)}
        confirmLabel="验证并保存"
        busy={save.busy}
        error={save.error}
        fieldErrors={save.fieldErrors}
        onClose={() => setConfirming(false)}
        onConfirm={({totp}) => void submit(totp)}
      />
    )}
  </>}</Resource>;
}
