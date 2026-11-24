import {
  Children,
  isValidElement,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type ComponentProps,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import {Popover} from "@base-ui/react/popover";
import {Check} from "lucide-react";
import {cn} from "cn";
import {useFieldControl} from "./field";
import {SelectChevronIcon} from "./icons";

type ParsedOption = {
  value: string;
  label: string;
  disabled?: boolean;
};

function textOf(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (isValidElement<{children?: ReactNode}>(node)) return textOf(node.props.children);
  return "";
}

function collectOptions(children: ReactNode, out: ParsedOption[] = []): ParsedOption[] {
  Children.forEach(children, child => {
    if (!isValidElement<{value?: string | number; disabled?: boolean; children?: ReactNode}>(child)) return;
    if (child.type === "option") {
      const label = textOf(child.props.children);
      const val = child.props.value !== undefined ? String(child.props.value) : label;
      out.push({value: val, label, disabled: child.props.disabled});
      return;
    }
    if (child.props.children !== undefined) {
      collectOptions(child.props.children, out);
    }
  });
  return out;
}

export function Select({
  className,
  children,
  value,
  defaultValue,
  onChange,
  disabled,
  required,
  name,
  id,
  "aria-label": ariaLabel,
  ...rest
}: ComponentProps<"select">) {
  const {attributes, clearError} = useFieldControl({id, name, "aria-label": ariaLabel});
  const options = useMemo(() => collectOptions(children), [children]);
  const controlled = value !== undefined;

  const fallbackValue = useMemo(() => {
    if (defaultValue !== undefined) return String(defaultValue);
    return options[0]?.value ?? "";
  }, [defaultValue, options]);

  const [internalValue, setInternalValue] = useState<string>(fallbackValue);
  useEffect(() => {
    if (!controlled && !options.some(o => o.value === internalValue) && options.length > 0) {
      setInternalValue(fallbackValue);
    }
  }, [controlled, fallbackValue, internalValue, options]);

  const currentValue = controlled ? String(value ?? "") : internalValue;
  const currentOption = options.find(opt => opt.value === currentValue) ?? options[0];

  const [open, setOpen] = useState(false);
  const popupRef = useRef<HTMLDivElement>(null);
  const effectiveName = attributes?.name ?? name;

  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => {
      const active =
        popupRef.current?.querySelector<HTMLButtonElement>('[role="option"][aria-selected="true"]:not(:disabled)') ??
        popupRef.current?.querySelector<HTMLButtonElement>('[role="option"]:not(:disabled)');
      active?.focus();
    });
    return () => cancelAnimationFrame(frame);
  }, [open]);

  function selectValue(next: string) {
    if (!controlled) setInternalValue(next);
    clearError?.();
    if (onChange) {
      const target = {value: next, name: effectiveName ?? ""} as EventTarget & HTMLSelectElement;
      const synthetic = {
        target,
        currentTarget: target,
        preventDefault: () => {},
        stopPropagation: () => {},
      } as unknown as ChangeEvent<HTMLSelectElement>;
      onChange(synthetic);
    }
    setOpen(false);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const items = Array.from(popupRef.current?.querySelectorAll<HTMLButtonElement>('[role="option"]:not(:disabled)') ?? []);
    if (!items.length) return;
    const idx = items.findIndex(item => item === document.activeElement);
    if (event.key === "ArrowDown") {
      event.preventDefault();
      items[idx < 0 ? 0 : Math.min(items.length - 1, idx + 1)]?.focus();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      items[idx < 0 ? items.length - 1 : Math.max(0, idx - 1)]?.focus();
    } else if (event.key === "Home") {
      event.preventDefault();
      items[0]?.focus();
    } else if (event.key === "End") {
      event.preventDefault();
      items[items.length - 1]?.focus();
    }
  }

  return (
    <span className="obs-select-wrap">
      {effectiveName && (
        <input
          type="hidden"
          name={effectiveName}
          value={currentValue}
          disabled={disabled}
          required={required}
        />
      )}
      <Popover.Root open={open} onOpenChange={next => { if (!disabled) setOpen(next); }}>
        <Popover.Trigger
          id={attributes?.id ?? id}
          data-slot="select"
          disabled={disabled}
          aria-label={ariaLabel}
          aria-labelledby={attributes?.["aria-labelledby"]}
          aria-describedby={attributes?.["aria-describedby"]}
          aria-invalid={attributes?.["aria-invalid"] ?? rest["aria-invalid"]}
          data-placeholder={currentValue === "" || undefined}
          className={cn("weir-input weir-field-control flex items-center justify-between w-full min-w-0 obs-select-trigger", className)}
        >
          <span className="obs-combobox-value">{currentOption?.label ?? currentValue}</span>
          <span className="obs-select-chevron" aria-hidden><SelectChevronIcon open={open} /></span>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Positioner sideOffset={5} align="start" className="obs-popover-positioner">
            <Popover.Popup
              ref={popupRef}
              className="obs-popover obs-select-popup"
              onKeyDown={handleKeyDown}
            >
              <div className="obs-popover-list" role="listbox" aria-label={ariaLabel}>
                {options.map(opt => {
                  const selected = opt.value === currentValue;
                  return (
                    <button
                      key={opt.value}
                      type="button"
                      role="option"
                      disabled={opt.disabled}
                      aria-selected={selected}
                      className={cn("obs-popover-item", selected && "is-selected")}
                      onClick={() => selectValue(opt.value)}
                    >
                      <span className="obs-popover-item-label">{opt.label}</span>
                      {selected && <Check className="obs-popover-check" aria-hidden />}
                    </button>
                  );
                })}
              </div>
            </Popover.Popup>
          </Popover.Positioner>
        </Popover.Portal>
      </Popover.Root>
    </span>
  );
}
