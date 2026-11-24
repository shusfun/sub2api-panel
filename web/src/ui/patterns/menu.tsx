import {useRef, type ReactElement, type ReactNode} from "react";
import {Menu as MenuPrimitive} from "@base-ui/react/menu";
import {Ellipsis} from "lucide-react";
import {Button} from "../primitives/button";

export function Menu({trigger, children, align = "end", onOpenChange}: {
  trigger: ReactElement;
  children: ReactNode;
  align?: "start" | "center" | "end";
  onOpenChange?: (open: boolean) => void;
}) {
  return (
    <MenuPrimitive.Root onOpenChange={onOpenChange}>
      <MenuPrimitive.Trigger render={trigger} />
      <MenuPrimitive.Portal>
        <MenuPrimitive.Positioner className="menu-positioner" align={align} sideOffset={5}>
          <MenuPrimitive.Popup className="menu-popup">{children}</MenuPrimitive.Popup>
        </MenuPrimitive.Positioner>
      </MenuPrimitive.Portal>
    </MenuPrimitive.Root>
  );
}

export function MenuItem({children, onSelect, disabled, tone}: {
  children: ReactNode;
  onSelect: () => void;
  disabled?: boolean;
  tone?: "bad";
}) {
  return <MenuPrimitive.Item className="menu-item" data-tone={tone} disabled={disabled} onClick={onSelect}>{children}</MenuPrimitive.Item>;
}

export function MoreActions({label, busy, children, onOpenChange}: {
  label: string;
  busy?: boolean;
  children: ReactNode;
  onOpenChange?: (open: boolean, trigger: HTMLButtonElement | null) => void;
}) {
  const trigger = useRef<HTMLButtonElement>(null);
  return <Menu onOpenChange={open => onOpenChange?.(open, trigger.current)} trigger={
    <Button ref={trigger} size="icon-sm" variant="ghost" title="更多" aria-label={label} busy={busy}><Ellipsis aria-hidden /></Button>
  }>{children}</Menu>;
}

export function MenuSeparator() {
  return <MenuPrimitive.Separator className="menu-separator" />;
}
