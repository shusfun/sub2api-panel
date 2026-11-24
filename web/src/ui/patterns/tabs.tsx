import {useRef, type ReactNode} from "react";
import {ArrowLeft} from "lucide-react";
import {cn} from "cn";
import {Button} from "../primitives/button";
import {RefreshMorphIcon} from "../icons/icons";
import {InlineStack} from "../primitives/layout";

export type TabItem<Value extends string> = {
  value: Value;
  label: ReactNode;
  badge?: ReactNode;
};

export function Tabs<Value extends string>({
  label,
  value,
  items,
  className,
  onChange,
}: {
  label: string;
  value: Value;
  items: readonly TabItem<Value>[];
  className?: string;
  onChange: (next: Value) => void;
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  return (
    <div role="tablist" aria-label={label} className={cn("weir-tabs", className)}>
      {items.map((item, index) => {
        const active = item.value === value;
        return (
          <button
            key={item.value}
            ref={el => { refs.current[index] = el; }}
            type="button"
            role="tab"
            aria-selected={active}
            tabIndex={active ? 0 : -1}
            className="weir-tab"
            onClick={() => onChange(item.value)}
            onKeyDown={e => {
              if (!["ArrowRight", "ArrowLeft", "Home", "End"].includes(e.key)) return;
              e.preventDefault();
              const nextIdx =
                e.key === "Home"
                  ? 0
                  : e.key === "End"
                    ? items.length - 1
                    : (index + (e.key === "ArrowRight" ? 1 : -1) + items.length) % items.length;
              const target = items[nextIdx];
              if (target) {
                onChange(target.value);
                refs.current[nextIdx]?.focus();
              }
            }}
          >
            <span>{item.label}</span>
            {item.badge}
          </button>
        );
      })}
    </div>
  );
}

export function DetailHeader({
  backLabel = "返回列表",
  title,
  badges,
  actions,
  onBack,
}: {
  backLabel?: string;
  title: ReactNode;
  badges?: ReactNode;
  actions?: ReactNode;
  onBack: () => void;
}) {
  return (
    <header className="weir-detail-header">
      <div className="weir-detail-title-row">
        <Button type="button" size="sm" variant="ghost" onClick={onBack}>
          <ArrowLeft aria-hidden />{backLabel}
        </Button>
        <h2>{title}</h2>
        {badges}
      </div>
      {actions && <InlineStack wrap>{actions}</InlineStack>}
    </header>
  );
}

export function RefreshButton({
  busy,
  label = "刷新",
  iconOnly = false,
  size = "sm",
  variant = "outline",
  onClick,
}: {
  busy?: boolean;
  label?: string;
  iconOnly?: boolean;
  size?: "default" | "xs" | "sm";
  variant?: "outline" | "ghost";
  onClick: () => void;
}) {
  return (
    <Button
      type="button"
      size={iconOnly ? (size === "default" ? "icon" : `icon-${size}`) : size}
      variant={variant}
      busy={busy}
      title={iconOnly ? label : undefined}
      aria-label={iconOnly ? label : undefined}
      onClick={onClick}
    >
      <RefreshMorphIcon />
      {!iconOnly && <span>{label}</span>}
    </Button>
  );
}
