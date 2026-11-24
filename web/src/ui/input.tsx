import {type ComponentProps, type ReactNode} from "react";
import {Input as InputPrimitive} from "@base-ui/react/input";
import {cn} from "cn";
import {useFieldControl} from "./field";

export const fieldClass =
  "weir-input weir-field-control block w-full min-w-0 text-control text-[var(--weir-ink)] transition-colors disabled:cursor-not-allowed disabled:bg-[var(--weir-fill-4)] disabled:opacity-55 aria-invalid:border-destructive";

type InputProps = Omit<ComponentProps<"input">, "prefix"> & {
  prefix?: ReactNode;
  suffix?: ReactNode;
  endAddon?: ReactNode;
};

function Input({className, type, prefix, suffix, endAddon, ...props}: InputProps) {
  const {attributes} = useFieldControl(props);
  if (prefix !== undefined || suffix !== undefined || endAddon !== undefined) {
    return (
      <div className={cn("weir-input-shell", className)} data-slot="input-shell">
        {prefix !== undefined && <span className="weir-input-affix">{prefix}</span>}
        <InputPrimitive
          type={type}
          data-slot="input"
          className="read-only:opacity-80"
          {...props}
          {...attributes}
        />
        {suffix !== undefined && <span className="weir-input-affix">{suffix}</span>}
        {endAddon !== undefined && <div className="weir-input-addon">{endAddon}</div>}
      </div>
    );
  }
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(fieldClass, "placeholder:text-[var(--weir-ink-3)] read-only:bg-[var(--weir-fill-4)]", className)}
      {...props}
      {...attributes}
    />
  );
}

export {Input};
