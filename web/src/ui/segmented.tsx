import {useFieldControl} from "./field";
import { ToggleGroup } from "@base-ui/react/toggle-group";
import { Toggle } from "@base-ui/react/toggle";

export function Segmented<Value extends string>({label, value, options, onChange}: {
  label: string;
  value: Value;
  options: readonly (readonly [Value, string])[];
  onChange: (value: Value) => void;
}) {
  const {attributes, clearError} = useFieldControl({"aria-label": label});
  return <ToggleGroup {...attributes} aria-label={label} className="weir-segmented" value={[value]} onValueChange={values => {
    const next = options.find(([option]) => values.includes(option));
    if (next) {clearError?.(); onChange(next[0]);}
  }}>
    {options.map(([option, text]) => <Toggle key={option} value={option}
      className="weir-key segmented-option" data-selected={value === option || undefined}>{text}</Toggle>)}
  </ToggleGroup>;
}
