import {useFieldControl} from "./field";
import {useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode} from "react";
import {Popover} from "@base-ui/react/popover";
import {Check, ChevronsUpDown, Search, X} from "lucide-react";
import {cn} from "cn";
import {Button} from "../primitives/button";

const COMBOBOX_SIDE_OFFSET = 5;

export const selectTriggerClass = "weir-input weir-field-control obs-combobox-trigger";

export type SelectOption = {
  value: string;
  label: string;
  meta?: string;
};

export function SearchSelect({
  label,
  value,
  options,
  onChange,
  placeholder = "全部",
  searchPlaceholder = "搜索选项…",
  emptyText = "无匹配项",
  searchable = true,
  clearable = true,
  icon,
  hasMore,
  loadingMore,
  onLoadMore,
  onSearchChange,
  searching = false,
  className,
  showEmptyOption = true,
  displayValue,
  customOption,
  status,
}: {
  label: string;
  value: string;
  options: readonly SelectOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  searchable?: boolean;
  clearable?: boolean;
  icon?: ReactNode;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  onSearchChange?: (query: string) => void;
  searching?: boolean;
  className?: string;
  showEmptyOption?: boolean;
  displayValue?: string;
  customOption?: (query: string) => string | undefined;
  status?: string;
}) {
  const {attributes, clearError} = useFieldControl({"aria-label": label});
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popupRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open) {
      setQuery("");
      if (searchable) {
        const frame = requestAnimationFrame(() => inputRef.current?.focus());
        return () => cancelAnimationFrame(frame);
      }
    }
  }, [open, searchable]);

  useEffect(() => {
    if (!onSearchChange) return;
    const timer = window.setTimeout(() => onSearchChange(query.trim()), 220);
    return () => window.clearTimeout(timer);
  }, [onSearchChange, query]);

  const selected = useMemo(
    () => options.find(item => item.value === value),
    [options, value],
  );

  const filtered = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword || onSearchChange) return options;
    return options.filter(item =>
      item.label.toLowerCase().includes(keyword) ||
      item.value.toLowerCase().includes(keyword) ||
      (item.meta?.toLowerCase().includes(keyword) ?? false),
    );
  }, [onSearchChange, options, query]);

  const keyword = query.trim();
  const isExact = (item: SelectOption) => item.value === keyword || item.label.toLowerCase() === keyword.toLowerCase();
  const custom = keyword && !options.some(isExact) ? customOption?.(keyword) : undefined;
  const choose = (next: string) => {
    clearError?.();
    onChange(next);
    setOpen(false);
  };
  const moveFocus = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.nativeEvent.isComposing) return;
    const items = Array.from(popupRef.current?.querySelectorAll<HTMLButtonElement>('[role="option"]') ?? []);
    if (!items.length) return;
    if (event.key === "Enter" && event.target === inputRef.current && query.trim()) {
      const item = items.find(option => option.dataset.exact !== undefined) ?? items[0];
      event.preventDefault();
      item.click();
    }
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    event.preventDefault();
    const index = items.findIndex(item => item === document.activeElement);
    if (event.key === "ArrowUp" && index === 0 && inputRef.current) inputRef.current.focus();
    else if (index < 0) items[event.key === "ArrowUp" ? items.length - 1 : 0].focus();
    else items[Math.max(0, Math.min(items.length - 1, index + (event.key === "ArrowDown" ? 1 : -1)))].focus();
  };
  const currentLabel = selected?.label ?? (value ? displayValue ?? "#" + value : placeholder);

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <div className={cn("obs-combobox", className)}>
        <Popover.Trigger
          ref={triggerRef}
          {...attributes}
          aria-label={label}
          title={value ? displayValue ?? selected?.label : undefined}
          className={cn(selectTriggerClass, value && "is-active")}
        >
          {icon && <span className="obs-combobox-icon" aria-hidden>{icon}</span>}
          <span className="obs-combobox-value">
            {currentLabel}
          </span>
          <ChevronsUpDown className="obs-combobox-chevron" aria-hidden />
        </Popover.Trigger>
        {clearable && value && (
          <button
            type="button"
            className="obs-combobox-clear"
            aria-label={`清除${label}`}
            title={`清除${label}`}
            onClick={event => {
              event.stopPropagation();
              triggerRef.current?.focus();
              onChange("");
            }}
          >
            <X aria-hidden />
          </button>
        )}
      </div>
      <Popover.Portal>
        <Popover.Positioner sideOffset={COMBOBOX_SIDE_OFFSET} align="start" className="obs-popover-positioner">
          <Popover.Popup ref={popupRef} className="obs-popover" onKeyDown={moveFocus}>
            {searchable && (
              <div className="obs-popover-search">
                <Search aria-hidden />
                <input
                  ref={inputRef}
                  type="text"
                  value={query}
                  onChange={event => setQuery(event.target.value)}
                  placeholder={searchPlaceholder}
                  aria-label={searchPlaceholder}
                />
                {query && (
                  <button type="button" className="obs-popover-search-clear" onClick={() => setQuery("")} aria-label="清空搜索">
                    <X aria-hidden />
                  </button>
                )}
              </div>
            )}
            <div className="obs-popover-list" role="listbox" aria-label={label}>
              {showEmptyOption && !query.trim() && (
                <button
                  type="button"
                  role="option"
                  aria-selected={value === ""}
                  className={cn("obs-popover-item", value === "" && "is-selected")}
                  onClick={() => choose("")}
                >
                  <span className="obs-popover-item-label">{placeholder}</span>
                  {value === "" && <Check className="obs-popover-check" aria-hidden />}
                </button>
              )}
              {filtered.map(item => (
                <button
                  key={item.value}
                  type="button"
                  role="option"
                  aria-selected={value === item.value}
                  data-exact={isExact(item) || undefined}
                  className={cn("obs-popover-item", value === item.value && "is-selected")}
                  onClick={() => choose(item.value)}
                >
                  <span className="obs-popover-item-main">
                    <span className="obs-popover-item-label">{item.label}</span>
                    {item.meta && <span className="obs-popover-item-meta">{item.meta}</span>}
                  </span>
                  {value === item.value && <Check className="obs-popover-check" aria-hidden />}
                </button>
              ))}
              {custom && <button type="button" role="option" aria-selected={false} data-exact className="obs-popover-item" onClick={() => choose(query.trim())}>
                <span className="obs-popover-item-label">{custom}</span>
              </button>}
            </div>
            {(status || !filtered.length) && <div className="obs-popover-empty" role="status">{status || (searching ? "搜索中…" : emptyText)}</div>}
            {hasMore && onLoadMore && (
              <div className="obs-popover-footer">
                <Button size="xs" variant="ghost" busy={loadingMore} onClick={onLoadMore} className="w-full">
                  加载更多选项
                </Button>
              </div>
            )}
          </Popover.Popup>
        </Popover.Positioner>
      </Popover.Portal>
    </Popover.Root>
  );
}
