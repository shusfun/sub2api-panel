import {Children, isValidElement, type ReactNode} from "react";
import {LayoutGrid, List, RotateCcw, Search, X} from "lucide-react";
import {cn} from "cn";
import {Tooltip} from "../primitives/tooltip";

export function FilterBar({
  sticky = false,
  className,
  children,
}: {
  sticky?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const hasExplicitRows = Children.toArray(children).some(
    child => isValidElement<{className?: string}>(child) && typeof child.props.className === "string" && child.props.className.includes("obs-filter-row"),
  );
  return (
    <div className={cn("obs-filter-bar", !hasExplicitRows && "obs-filter-bar-inline", sticky && "obs-filter-bar-sticky", className)}>
      {children}
    </div>
  );
}

// 筛选栏里的选择器槽位：统一宽度，页面不写尺寸。
export function FilterSlot({children}: {children: ReactNode}) {
  return <div className="obs-filter-slot">{children}</div>;
}

export type ViewMode = "card" | "list";

// 卡片 / 列表切换：两个图标共用一个分段，当前项 aria-pressed。
export function ViewToggle({value, onChange}: {value: ViewMode; onChange: (next: ViewMode) => void}) {
  return (
    <div className="obs-quick-pills" role="group" aria-label="视图">
      {([["card", "宫格卡片视图", LayoutGrid], ["list", "紧凑列表视图", List]] as const).map(([mode, label, Icon]) => (
        <Tooltip key={mode} content={label} side="bottom">
          <button type="button" className={cn("obs-pill-btn", value === mode && "is-selected")}
            aria-pressed={value === mode} aria-label={label} onClick={() => onChange(mode)}>
            <Icon aria-hidden />
          </button>
        </Tooltip>
      ))}
    </div>
  );
}

export function SearchInput({
  value,
  onChange,
  placeholder = "搜索…",
  ariaLabel = "搜索",
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  ariaLabel?: string;
  className?: string;
}) {
  return (
    <div className={cn("obs-search-input", className)}>
      <Search className="obs-search-icon" aria-hidden />
      <input
        type="search"
        className="weir-input weir-field-control obs-search-control w-full"
        value={value}
        placeholder={placeholder}
        aria-label={ariaLabel}
        maxLength={100}
        onChange={event => onChange(event.target.value)}
      />
      {value && (
        <button
          type="button"
          className="obs-search-clear"
          aria-label="清空搜索"
          title="清空搜索"
          onClick={() => onChange("")}
        >
          <X aria-hidden />
        </button>
      )}
    </div>
  );
}

export type FilterChip = {
  id: string;
  label: string;
  value: string;
  onRemove: () => void;
};

export function FilterChips({
  chips,
  onReset,
}: {
  chips: readonly FilterChip[];
  onReset?: () => void;
}) {
  if (!chips.length) return null;
  return (
    <div className="obs-filter-chips" aria-label="当前筛选条件">
      {chips.map(chip => (
        <span key={chip.id} className="obs-chip">
          <span className="obs-chip-key">{chip.label}</span>
          <strong className="obs-chip-val" title={chip.value}>{chip.value}</strong>
          <button
            type="button"
            className="obs-chip-remove"
            aria-label={`移除筛选 ${chip.label}: ${chip.value}`}
            onClick={chip.onRemove}
          >
            <X aria-hidden />
          </button>
        </span>
      ))}
      {onReset && chips.length > 1 && (
        <button type="button" className="obs-chip-reset" onClick={onReset}>
          <RotateCcw aria-hidden />重置筛选
        </button>
      )}
    </div>
  );
}

export function QuickPills<Value extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: Value;
  options: readonly {value: Value; label: string; tone?: "neutral" | "muted" | "ok" | "warn" | "bad"}[];
  onChange: (value: Value) => void;
}) {
  return (
    <div className="obs-quick-pills" role="group" aria-label={label}>
      {options.map(item => (
        <button
          key={item.value}
          type="button"
          className={cn("obs-pill-btn", value === item.value && "is-selected")}
          data-tone={item.tone ?? "neutral"}
          aria-pressed={value === item.value}
          onClick={() => onChange(item.value)}
        >
          {item.tone && item.tone !== "neutral" && <i className="obs-pill-dot" aria-hidden />}
          <span>{item.label}</span>
        </button>
      ))}
    </div>
  );
}
