import {X} from "lucide-react";
import {useMemo} from "react";
import {Button} from "../primitives/button";
import {SearchSelect, type SelectOption} from "../patterns/combobox";

export type ModelCatalog = {
  options?: readonly SelectOption[];
  loading?: boolean;
  error?: string;
};

export function modelOptions(ids: readonly string[]): SelectOption[] {
  return ids.map(id => ({value: id, label: id}));
}

type ModelPickerProps = {
  label: string;
  placeholder: string;
  catalog: ModelCatalog;
  allowCustom?: boolean;
};

function ModelPicker({label, placeholder, catalog, allowCustom, value, onChange, excluded}: ModelPickerProps & {
  value: string;
  onChange: (id: string) => void;
  excluded?: readonly string[];
}) {
  const options = useMemo(() => {
    const seen = new Set(excluded);
    const options = (catalog.options ?? []).filter(item => {
      if (!item.value.trim() || seen.has(item.value)) return false;
      seen.add(item.value);
      return true;
    });
    if (value && !seen.has(value)) options.unshift({value, label: value});
    return options;
  }, [catalog.options, excluded, value]);
  const status = catalog.loading ? "正在读取候选模型…" : catalog.error;
  return <SearchSelect
    label={label} placeholder={placeholder} value={value} displayValue={value} options={options} onChange={onChange}
    searchPlaceholder="搜索模型名称或 ID" emptyText="暂无匹配模型" status={status}
    clearable={!excluded} showEmptyOption={!excluded}
    customOption={allowCustom ? id => excluded?.includes(id) ? undefined : "使用模型 " + id : undefined}
  />;
}

export function ModelSelect(props: ModelPickerProps & {value: string; onChange: (id: string) => void}) {
  return <ModelPicker {...props} />;
}

export function MultiModelSelect({value, onChange, ...props}: ModelPickerProps & {
  value: readonly string[];
  onChange: (ids: string[]) => void;
}) {
  return <div className="model-multi" role="group" aria-label={props.label}>
    {value.length > 0 && <div className="model-chips">
      {value.map(id => <div className="model-chip" key={id}>
        <span className="model-chip-label code-mono">{id}</span>
        <Button size="icon-xs" variant="ghost" title="移除模型" aria-label={"移除模型 " + id}
          onClick={() => onChange(value.filter(model => model !== id))}><X aria-hidden /></Button>
      </div>)}
    </div>}
    <ModelPicker {...props} value="" excluded={value} onChange={id => onChange([...value, id])} />
  </div>;
}
