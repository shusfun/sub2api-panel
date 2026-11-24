import {useFieldControl} from "./field";
import type {ReactNode} from "react";

export function Checkbox({
  checked,
  onChange,
  label,
  disabled,
  children,
  name,
  required,
  inline = false,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  disabled?: boolean;
  children?: ReactNode;
  name?: string;
  required?: boolean;
  inline?: boolean;
}) {
  const {attributes} = useFieldControl({name, "aria-label": label});
  return (
    <label className={children && !inline ? "plan-option-control" : "weir-checkbox-wrap"} data-inline={inline || undefined}>
      <input
        className="weir-checkbox"
        type="checkbox"
        checked={checked}
        disabled={disabled}
        name={name}
        required={required}
        {...attributes}
        aria-label={label}
        onChange={event => onChange(event.target.checked)}
      />
      {children}
    </label>
  );
}
