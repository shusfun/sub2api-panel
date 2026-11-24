import {ChevronsUpDown} from "lucide-react";
import {cn} from "cn";
import {selectTriggerClass} from "./combobox";
import {useFieldControl} from "./field";
import {useId, useRef, useState, type ReactNode, type RefObject} from "react";
import {z} from "zod";
import {api} from "../api";
import {date, statusLabel} from "../format";
import {useResource} from "../requests";
import {Badge} from "./badge";
import {Button} from "./button";
import {Checkbox} from "./checkbox";
import {PlatformBadge, platformLabel} from "./entity";
import {EmptyState, ErrorLine, Resource} from "./feedback";
import {Form} from "./form";
import {Input} from "./input";
import {Overlay} from "./overlay";
import {Select} from "./select";
import {Status} from "./status";
import {Tooltip} from "./tooltip";

const groupOption = z.object({
  id: z.number().int().positive(), name: z.string(), platform: z.string(), status: z.string(),
  is_exclusive: z.boolean().optional(), subscription_type: z.string(), rate_multiplier: z.number(),
  health: z.string().optional(), health_score: z.number().nullable().optional(),
});

type Choice = {
  id: number; name: string; platform?: string; status?: string; is_exclusive?: boolean;
  expires_at?: string; rate_multiplier?: number; health?: string; health_score?: number | null;
};
type GroupSource = {
  admin?: boolean; platform?: string; subscriptionOnly?: boolean; privateOnly?: boolean; revision?: number;
};

export function useGroups({admin = true, revision}: GroupSource, enabled = true) {
  const path = admin ? "/admin/groups/all" : "/key-groups";
  return useResource(() => api(path, z.array(groupOption)), path, enabled, revision);
}

export function PlanIdentity({item}: {item: Choice}) {
  return <span className="plan-identity">
    <strong>{item.name}</strong>
    {item.platform && <PlatformBadge platform={item.platform} />}
    {item.is_exclusive !== undefined && <Badge variant="tag">{item.is_exclusive ? "私有" : "公开"}</Badge>}
    {item.status && <Status value={item.status} />}
    {item.expires_at && <span>{date(item.expires_at)} 到期</span>}
    {item.rate_multiplier !== undefined && <span>{item.rate_multiplier}×</span>}
    {item.health && item.health !== "unknown" && item.health !== "unavailable" && <Status value={item.health} />}
    {item.health_score != null && <span>{item.health_score} 分</span>}
  </span>;
}

function PlanOptions({items, draft, onDraft, multiple, readOnly, activeOnly, allowEmpty}: {
  items: readonly Choice[]; draft: number[]; onDraft: (ids: number[]) => void;
  multiple?: boolean; readOnly?: boolean; activeOnly?: boolean; allowEmpty?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [platform, setPlatform] = useState("");
  const [status, setStatus] = useState("");
  const needle = search.trim().toLocaleLowerCase();
  const filtered = items.filter(item => (!platform || item.platform === platform) && (!status || item.status === status) &&
    (!needle || item.name.toLocaleLowerCase().includes(needle) || String(item.id).includes(needle)));
  const selectable = filtered.filter(item => !activeOnly || !item.status || item.status === "active");
  const allSelected = selectable.length > 0 && selectable.every(item => draft.includes(item.id));
  return <>
    <div className="plan-filters">
      <Input type="search" aria-label="搜索名称或 ID" placeholder="搜索名称或 ID" value={search} onChange={e => setSearch(e.target.value)} autoFocus />
      <Select aria-label="按平台筛选" value={platform} onChange={e => setPlatform(e.target.value)}>
        <option value="">全部平台</option>
        {[...new Set(items.flatMap(item => item.platform ? [item.platform] : []))].sort().map(value => <option key={value} value={value}>{platformLabel(value)}</option>)}
      </Select>
      <Select aria-label="按状态筛选" value={status} onChange={e => setStatus(e.target.value)}>
        <option value="">全部状态</option>
        {[...new Set(items.flatMap(item => item.status ? [item.status] : []))].sort().map(value => <option key={value} value={value}>{statusLabel(value)}</option>)}
      </Select>
    </div>
    {multiple && !readOnly && <Button size="sm" variant="outline" disabled={!selectable.length} onClick={() => {
      const ids = new Set(selectable.map(item => item.id));
      onDraft(allSelected ? draft.filter(id => !ids.has(id)) : [...new Set([...draft, ...ids])]);
    }}>{allSelected ? "取消选中当前结果" : "全选当前结果"}</Button>}
    <div className="plan-options" role="group" aria-label="匹配项">
      {!readOnly && allowEmpty && !multiple && <Button variant={draft.length ? "ghost" : "secondary"} aria-pressed={!draft.length} onClick={() => onDraft([])}>清除选择</Button>}
      {!filtered.length && <EmptyState title="没有匹配项" />}
      {filtered.map(item => {
        const checked = draft.includes(item.id);
        const disabled = activeOnly && !!item.status && item.status !== "active" && !checked;
        return <div className="plan-option" key={item.id}>
          {readOnly ? <PlanIdentity item={item} /> : multiple ? (
            <Checkbox checked={checked} disabled={disabled} label={item.name} onChange={next => onDraft(next ? [...draft, item.id] : draft.filter(id => id !== item.id))}>
              <PlanIdentity item={item} />
            </Checkbox>
          ) : <Button className="plan-option-control" variant={checked ? "secondary" : "ghost"} aria-pressed={checked} disabled={disabled} onClick={() => onDraft([item.id])}><PlanIdentity item={item} /></Button>}
        </div>;
      })}
    </div>
  </>;
}

export function GroupDialog({title, onClose, onApply, value = [], multiple, allowEmpty, activeOnly, source, busy, error, children, finalFocus, applyLabel = "应用", applyDisabled}: {
  title: string; onClose: () => void; onApply: (ids: number[], fields: FormData) => void; value?: readonly number[];
  multiple?: boolean; allowEmpty?: boolean; activeOnly?: boolean; source: GroupSource; busy?: boolean; error?: string;
  children?: ReactNode; finalFocus?: RefObject<HTMLElement | null>; applyLabel?: string;
  applyDisabled?: boolean;
}) {
  const formID = useId();
  const resource = useGroups(source);
  const [draft, setDraft] = useState<number[]>();
  const items = (resource?.value ?? []).filter(group => (!source.privateOnly || group.is_exclusive) &&
    (!source.platform || group.platform === source.platform || group.platform === "composite") &&
    (!source.subscriptionOnly || group.subscription_type === "subscription") &&
    (!activeOnly || group.status === "active" || value.includes(group.id)));
  const selected = draft ?? value.filter(id => items.some(item => item.id === id) || !source.privateOnly);
  return <Overlay open title={title} onClose={onClose} busy={busy} finalFocus={finalFocus} footer={
          <div className="form-actions">
            <Button variant="outline" onClick={onClose}>取消</Button>
            <Button type="submit" form={formID} busy={busy} disabled={applyDisabled || !resource?.value || !!resource.error || !allowEmpty && !selected.length}>{applyLabel}</Button>
          </div>
        }>
    <Form id={formID} busy={busy} onSubmit={event => onApply(selected, new FormData(event.currentTarget))}>
      <Resource state={resource}>
        {() => <PlanOptions items={items} draft={selected} onDraft={setDraft} multiple={multiple} allowEmpty={allowEmpty} activeOnly={activeOnly} />}
      </Resource>
      {children}
      <ErrorLine value={error ?? ""} />
    </Form>
  </Overlay>;
}

export function GroupSelect({value, onChange, placeholder = "选择分组", clearable = true, ...source}: GroupSource & {
  value: string; onChange: (value: string) => void; placeholder?: string; clearable?: boolean;
}) {
  return <GroupTrigger value={value ? [Number(value)] : []} onChange={ids => onChange(ids.length ? String(ids[0]) : "")} placeholder={placeholder} source={source} clearable={clearable} />;
}

export function MultiGroupSelect({value, onChange, ...source}: GroupSource & {value: readonly number[]; onChange: (ids: number[]) => void}) {
  return <GroupTrigger value={value} onChange={onChange} placeholder={source.privateOnly ? "选择私有分组" : "选择分组"} source={source} clearable multiple />;
}

function GroupTrigger({value, onChange, placeholder, source, clearable, multiple}: {
  value: readonly number[]; onChange: (ids: number[]) => void; placeholder: string; source: GroupSource; clearable: boolean; multiple?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const {attributes, clearError} = useFieldControl();
  const options = useGroups(source, value.length === 1);
  const item = options?.value?.find(group => group.id === value[0]);
  return <>
    <div className="obs-combobox">
      <button ref={trigger} {...attributes} type="button" className={cn(selectTriggerClass, "obs-group-trigger", value.length > 0 && "is-active")}
        aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen(true)}>
        <span className="obs-combobox-value">
      {value.length > 1 ? `已选 ${value.length} 个分组` : item ? <PlanIdentity item={{id: item.id, name: item.name, platform: item.platform}} /> : value.length ? `分组 #${value[0]}` : placeholder}
        </span>
        <ChevronsUpDown className="obs-combobox-chevron" aria-hidden />
      </button>
    </div>
    {open && <GroupDialog title={source.subscriptionOnly ? "选择订阅分组" : source.privateOnly ? "选择私有分组" : "选择分组"} source={source} value={value}
      allowEmpty={clearable} multiple={multiple} activeOnly={multiple || source.admin === false || source.subscriptionOnly && !clearable} finalFocus={trigger}
      onClose={() => setOpen(false)} onApply={ids => {clearError?.(); onChange(ids); setOpen(false);}} />}
  </>;
}

export function PlanSummary({items, label, empty = "未关联"}: {items: readonly Choice[]; label: string; empty?: string}) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  if (!items.length) return <span className="muted">{empty}</span>;
  const previewNames = items.slice(0, 6).map(i => i.name).join(" · ") + (items.length > 6 ? ` 等 ${items.length} 个` : "");
  return <>
    <Tooltip content={previewNames} side="top">
      <Button ref={trigger} size="xs" variant="ghost" aria-haspopup="dialog" onClick={() => setOpen(true)}>{items.length} 个{label}</Button>
    </Tooltip>
    {open && <Overlay open title={label} onClose={() => setOpen(false)} finalFocus={trigger}>
      <PlanOptions items={items} draft={[]} onDraft={() => {}} readOnly />
    </Overlay>}
  </>;
}
