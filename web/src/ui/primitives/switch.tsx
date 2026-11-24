import {useFieldControl} from "../patterns/field";
import {Switch as SwitchPrimitive} from "@base-ui/react/switch";
import {cn} from "cn";

export function Switch({
  checked,
  onChange,
  label,
  size = "sm",
  disabled,
  className,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  size?: "sm" | "md";
  disabled?: boolean;
  className?: string;
}) {
  const {attributes, clearError} = useFieldControl({"aria-label": label});
  return (
    <SwitchPrimitive.Root
      {...attributes}
      checked={checked}
      disabled={disabled}
      aria-label={label}
      data-size={size === "sm" ? "sm" : undefined}
      onCheckedChange={next => {clearError?.(); onChange(next);}}
      onClick={event => event.stopPropagation()}
      className={cn("weir-switch weir-hit", className)}
    >
      <SwitchPrimitive.Thumb className="weir-switch-thumb" />
    </SwitchPrimitive.Root>
  );
}
