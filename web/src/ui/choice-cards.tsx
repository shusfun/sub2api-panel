import {useRef, type ReactNode} from "react";
import {cn} from "cn";
import {FieldBoundary} from "./field";
import {EllipsisText} from "./tooltip";

export type ChoiceOption<Value extends string> = {
  value: Value;
  title: ReactNode;
  description?: ReactNode;
  meta?: ReactNode;
  disabled?: boolean;
};

export function ChoiceCards<Value extends string>({
  label,
  value,
  options,
  columns,
  className,
  onChange,
}: {
  label: string;
  value: Value;
  options: readonly ChoiceOption<Value>[];
  columns?: 2 | 3;
  className?: string;
  onChange: (next: Value) => void;
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const enabledIndices = options.map((opt, idx) => (opt.disabled ? -1 : idx)).filter(idx => idx >= 0);

  return (
    <FieldBoundary>
      <div
        role="radiogroup"
        aria-label={label}
        data-cols={columns}
        className={cn("swift-choice-grid", className)}
      >
        {options.map((option, index) => {
          const selected = option.value === value;
          const focusable = selected || (!options.some(o => o.value === value && !o.disabled) && index === enabledIndices[0]);
          return (
            <button
              key={option.value}
              ref={el => { refs.current[index] = el; }}
              type="button"
              role="radio"
              aria-checked={selected}
              tabIndex={option.disabled ? -1 : focusable ? 0 : -1}
              disabled={option.disabled}
              data-selected={selected || undefined}
              className="swift-choice-card weir-press-fill"
              onClick={() => {
                if (!option.disabled) onChange(option.value);
              }}
              onKeyDown={e => {
                if (!["ArrowRight", "ArrowDown", "ArrowLeft", "ArrowUp"].includes(e.key) || enabledIndices.length === 0) return;
                e.preventDefault();
                const pos = enabledIndices.indexOf(index);
                const step = e.key === "ArrowRight" || e.key === "ArrowDown" ? 1 : -1;
                const nextIdx = enabledIndices[(pos + step + enabledIndices.length) % enabledIndices.length];
                const nextOpt = options[nextIdx];
                if (nextOpt) {
                  onChange(nextOpt.value);
                  refs.current[nextIdx]?.focus();
                }
              }}
            >
              <div className="swift-choice-head">
                <strong>
                  {typeof option.title === "string" ? <EllipsisText text={option.title} /> : option.title}
                </strong>
                {option.meta !== undefined && <span className="swift-choice-meta">{option.meta}</span>}
              </div>
              {option.description !== undefined && (
                typeof option.description === "string" ? (
                  <EllipsisText text={option.description} lines={2} className="swift-choice-desc" />
                ) : (
                  <span className="swift-choice-desc">{option.description}</span>
                )
              )}
            </button>
          );
        })}
      </div>
    </FieldBoundary>
  );
}
