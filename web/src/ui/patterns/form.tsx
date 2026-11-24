import { createContext, useContext, useEffect, useId, useMemo, useRef, type ComponentProps, type FormEvent, type ReactNode } from "react";
import { cn } from "cn";

export type FormErrors = Partial<Record<string, ReactNode>>;
const noErrors: FormErrors = {};
type FormContextValue = { id: string; busy: boolean; errors: FormErrors };
const FormContext = createContext<FormContextValue | null>(null);

function useFormContext() {
  return useContext(FormContext);
}

// A nested dialog owns a separate form scope, even when rendered inside a form.
function FormBoundary({ children }: { children: ReactNode }) {
  return <FormContext.Provider value={null}>{children}</FormContext.Provider>;
}

function Form({
  id,
  busy = false,
  errors,
  onSubmit,
  className,
  onInvalidCapture,
  children,
  ref: outerRef,
  ...props
}: Omit<ComponentProps<"form">, "onSubmit"> & {
  busy?: boolean;
  errors?: FormErrors;
  onSubmit?: (e: FormEvent<HTMLFormElement>) => void;
}) {
  const generatedId = useId();
  const formId = id ?? generatedId;
  const value = useMemo(() => ({ id: formId, busy, errors: errors ?? noErrors }), [formId, busy, errors]);
  const ref = useRef<HTMLFormElement>(null);
  const invalidFrame = useRef<number>(undefined);
  const invalidTargets = useRef(new Set<HTMLElement>());
  useEffect(() => () => {
    if (invalidFrame.current !== undefined) cancelAnimationFrame(invalidFrame.current);
  }, []);
  useEffect(() => {
    if (busy || !errors || !Object.values(errors).some(Boolean)) return;
    for (const control of ref.current?.querySelectorAll<HTMLElement>('[aria-invalid="true"]') ?? []) {
      control.focus();
      if (control === document.activeElement) break;
    }
  }, [errors, busy]);
  return (
    <FormContext.Provider value={value}>
      <form
        id={formId}
        ref={(el) => {
          ref.current = el;
          if (typeof outerRef === "function") outerRef(el);
          else if (outerRef) outerRef.current = el;
        }}
        data-slot="form"
        data-tour="form"
        aria-busy={busy || undefined}
        onInvalidCapture={event => {
          onInvalidCapture?.(event);
          if (!(event.target instanceof HTMLElement) || !event.currentTarget.contains(event.target)) return;
          invalidTargets.current.add(event.target);
          if (invalidFrame.current !== undefined) return;
          invalidFrame.current = requestAnimationFrame(() => {
            invalidFrame.current = undefined;
            for (const control of invalidTargets.current) {
              if (!ref.current?.contains(control)) continue;
              control.focus();
              if (control === document.activeElement) break;
            }
            invalidTargets.current.clear();
          });
        }}
        onSubmit={(e) => {
          e.preventDefault();
          if (e.target !== e.currentTarget) return;
          if (!busy) onSubmit?.(e);
        }}
        {...props}
      >
        <fieldset disabled={busy} className={cn("form-fields", className)}>
          {children}
        </fieldset>
      </form>
    </FormContext.Provider>
  );
}

// 根据表单所在容器选列数，弹窗和检查器与主工作区使用同一规则。
function FormGrid({ cols = 2, className, ...props }: ComponentProps<"div"> & { cols?: 2 | 3 }) {
  return (
    <div data-slot="form-grid" className="form-grid-container">
      <div className={cn("form-grid", className)} data-columns={cols} {...props} />
    </div>
  );
}

function FormActions({className, ...props}: ComponentProps<"div">) {
  return <div className={cn("form-actions", className)} {...props} />;
}

// 表单内的分区（稿子 DForm 的分区小标题 + 分组内容）：长弹窗按业务拆成 2–3 段，替代从头堆到底或老式 <fieldset><legend> 线框。
function FormSection({
  title,
  description,
  actions,
  plain,
  className,
  children,
  ...props
}: Omit<ComponentProps<"div">, "title"> & { title?: ReactNode; description?: ReactNode; actions?: ReactNode; plain?: boolean }) {
  return (
    <div
      data-slot="form-section"
      data-plain={plain || undefined}
      className={cn("form-section", className)}
      {...props}
    >
      {(title != null || description != null || actions != null) && (
        <div className="form-section-head">
          <div className="min-w-0">
            {title != null && <div className="text-fine font-semibold tracking-[0.05em] text-[var(--weir-ink-3)]">{title}</div>}
            {description != null && <div className="mt-[var(--weir-gap-3)] text-meta text-[var(--weir-ink-3)]">{description}</div>}
          </div>
          {actions}
        </div>
      )}
      {children}
    </div>
  );
}

export { Form, FormActions, FormGrid, FormSection, FormBoundary, useFormContext };
