import {useState} from "react";
import {Plus, Trash2, X} from "lucide-react";
import {dateTimeInputSeconds, dateTimeInputValue} from "@/lib/format";
import {ProxySelect, UserSelect} from "../domain/entity-select";
import {GroupSelect} from "../domain/plan-picker";
import {Field} from "./field";
import {Badge} from "../primitives/badge";
import {Button} from "../primitives/button";
import {Input} from "../primitives/input";
import {PasswordInput} from "../primitives/password-input";
import {Select} from "../primitives/select";
import {Switch} from "../primitives/switch";
import {Textarea} from "../primitives/textarea";

// 密钥输入：只写不读。显示是否已配置，留空表示保持原值。
export function SecretInput({label, configured, value, onChange}: {
  label: string;
  configured?: boolean;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="setting-secret">
      {configured !== undefined && <Badge variant="soft" tone={configured ? "ok" : "neutral"}>{configured ? "已配置" : "未配置"}</Badge>}
      <PasswordInput autoComplete="new-password" aria-label={label} toggleLabel={label} value={value}
        placeholder={configured ? "留空不改" : "输入新密钥"} onChange={event => onChange(event.target.value)} />
    </div>
  );
}

// 多值输入：一行一个，保存时去掉空行和首尾空白。
export function ListInput({label, value, onChange, numeric = false}: {
  label: string;
  value: readonly (string | number)[];
  onChange: (value: (string | number)[]) => void;
  numeric?: boolean;
}) {
  return (
    <Textarea aria-label={label} rows={Math.min(9, Math.max(3, value.length + 1))} defaultValue={value.join("\n")}
      placeholder="一行一个" onBlur={event => {
        const lines = event.target.value.split(/[\n,]/).map(line => line.trim()).filter(Boolean);
        onChange(numeric ? lines.map(Number).filter(Number.isFinite) : lines);
      }} />
  );
}

export type RowColumn = {
  key: string;
  kind: string;
  label: string;
  options?: readonly {value: string; label: string}[];
};
type Row = Record<string, unknown>;

function blank(column: RowColumn): unknown {
  switch (column.kind) {
    case "bool": return false;
    case "int": case "float": case "group": return 0;
    case "optint": case "optfloat": case "proxy": case "datetime": return null;
    case "list": case "intlist": case "users": return [];
    case "select": return column.options?.[0]?.value ?? "";
    default: return "";
  }
}

function UsersCellEditor({value, onChange}: {value: number[]; onChange: (next: number[]) => void}) {
  const [labels, setLabels] = useState<Record<number, string>>({});
  return (
    <div className="stack">
      <UserSelect
        value=""
        placeholder="搜索并添加用户…"
        onChange={(next, label) => {
          const id = Number(next);
          if (!Number.isInteger(id) || id < 1) return;
          if (label) setLabels(prev => ({...prev, [id]: label}));
          if (!value.includes(id)) onChange([...value, id]);
        }}
      />
      {value.length > 0 && (
        <div className="row-actions">
          {value.map(id => (
            <Badge key={id} variant="tag" tone="neutral">
              <span>{labels[id] ? `${labels[id]} (#${id})` : `#${id}`}</span>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label={`移除用户 #${id}`}
                onClick={() => onChange(value.filter(item => item !== id))}
              >
                <X />
              </Button>
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}

function unixToDateTimeInput(cell: unknown): string {
  if (typeof cell === "number" && Number.isFinite(cell) && cell > 0) {
    return dateTimeInputValue(new Date(cell * 1000).toISOString());
  }
  if (typeof cell === "string" && cell.trim()) {
    return dateTimeInputValue(cell);
  }
  return "";
}

// 行表格：每行一组字段，可增删。列的类型与设置字段一致。
export function RowsEditor({label, columns, value, onChange}: {
  label: string;
  columns: readonly RowColumn[];
  value: readonly Row[];
  onChange: (value: Row[]) => void;
}) {
  const update = (index: number, key: string, next: unknown) => onChange(value.map((row, i) => i === index ? {...row, [key]: next} : row));
  return (
    <div className="setting-rows" role="group" aria-label={label}>
      {value.map((row, index) => (
        <div key={index} className="setting-row-card">
          <div className="setting-row-fields">
            {columns.map(column => {
              const name = `${label} 第 ${index + 1} 行 ${column.label}`;
              const cell = row[column.key];
              return (
                <Field key={column.key} label={column.label} data-wide={column.kind === "long" || column.kind === "list" || column.kind === "users" || undefined}>
                  {column.kind === "bool" ? <Switch label={name} checked={cell === true} onChange={next => update(index, column.key, next)} />
                    : column.kind === "select" ? (
                      <Select aria-label={name} value={String(cell ?? "")} onChange={event => update(index, column.key, event.target.value)}>
                        {column.options?.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
                      </Select>
                    ) : column.kind === "group" ? (
                      <GroupSelect
                        value={typeof cell === "number" && cell > 0 ? String(cell) : ""}
                        placeholder="选择分组"
                        onChange={next => update(index, column.key, next ? Number(next) : 0)}
                      />
                    ) : column.kind === "proxy" ? (
                      <ProxySelect
                        value={typeof cell === "number" && cell > 0 ? String(cell) : ""}
                        onChange={next => update(index, column.key, next ? Number(next) : null)}
                      />
                    ) : column.kind === "users" ? (
                      <UsersCellEditor
                        value={Array.isArray(cell) ? cell.map(Number).filter(n => Number.isInteger(n) && n > 0) : []}
                        onChange={next => update(index, column.key, next)}
                      />
                    ) : column.kind === "datetime" ? (
                      <Input
                        aria-label={name}
                        type="datetime-local"
                        step={1}
                        value={unixToDateTimeInput(cell)}
                        onChange={event => update(index, column.key, dateTimeInputSeconds(event.target.value) ?? null)}
                      />
                    ) : column.kind === "long" ? <Textarea aria-label={name} rows={3} value={String(cell ?? "")} onChange={event => update(index, column.key, event.target.value)} />
                    : column.kind === "list" || column.kind === "intlist" ? (
                      <ListInput label={name} numeric={column.kind === "intlist"} value={Array.isArray(cell) ? cell as (string | number)[] : []} onChange={next => update(index, column.key, next)} />
                    ) : column.kind === "secret" ? (
                      <SecretInput label={name} configured={row[column.key + "_configured"] === true} value={String(cell ?? "")} onChange={next => update(index, column.key, next)} />
                    ) : column.kind === "text" ? <Input aria-label={name} value={String(cell ?? "")} onChange={event => update(index, column.key, event.target.value)} />
                    : (
                      <Input aria-label={name} type="number" step={column.kind.endsWith("float") ? "any" : "1"} value={cell === null || cell === undefined ? "" : String(cell)}
                        placeholder={column.kind.startsWith("opt") ? "不限" : undefined}
                        onChange={event => update(index, column.key, event.target.value === "" ? (column.kind.startsWith("opt") ? null : 0) : Number(event.target.value))} />
                    )}
                </Field>
              );
            })}
          </div>
          <Button variant="ghost" size="icon-sm" title="删除这一行" aria-label={`删除${label}第 ${index + 1} 行`} onClick={() => onChange(value.filter((_, i) => i !== index))}><Trash2 /></Button>
        </div>
      ))}
      <Button variant="outline" size="sm" onClick={() => onChange([...value, Object.fromEntries(columns.map(column => [column.key, blank(column)]))])}>
        <Plus aria-hidden />添加一行
      </Button>
    </div>
  );
}
