import {useState, type ComponentProps} from "react";
import {Button} from "./button";
import {EyeMorphIcon} from "./icons";
import {Input} from "./input";

export function PasswordInput({
  visible: controlledVisible,
  onVisibleChange,
  toggleLabel = "密码",
  ...props
}: Omit<ComponentProps<typeof Input>, "type" | "endAddon"> & {
  visible?: boolean;
  onVisibleChange?: (next: boolean) => void;
  toggleLabel?: string;
}) {
  const [internalVisible, setInternalVisible] = useState(false);
  const shown = controlledVisible ?? internalVisible;
  const toggle = () => {
    const next = !shown;
    setInternalVisible(next);
    onVisibleChange?.(next);
  };
  const label = (shown ? "隐藏" : "显示") + toggleLabel;
  return (
    <Input
      {...props}
      type={shown ? "text" : "password"}
      endAddon={
        <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          title={label}
          aria-label={label}
          aria-pressed={shown}
          disabled={props.disabled}
          onClick={toggle}
        >
          <EyeMorphIcon open={shown} />
        </Button>
      }
    />
  );
}
