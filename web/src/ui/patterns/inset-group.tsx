import {useId, useState, type ReactNode} from "react";
import {ChevronRight} from "lucide-react";
import {cn} from "cn";
import {ChevronMorphIcon} from "../icons/icons";

export function InsetGroup({
  title,
  footer,
  actions,
  inset,
  className,
  children,
}: {
  title?: ReactNode;
  footer?: ReactNode;
  actions?: ReactNode;
  inset?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section className={cn("swift-group", className)} data-inset={inset || undefined}>
      {(title !== undefined || actions !== undefined) && (
        <header className="swift-group-head">
          {title !== undefined && <h3 className="swift-group-title">{title}</h3>}
          {actions !== undefined && <div className="swift-group-actions">{actions}</div>}
        </header>
      )}
      <div className="swift-group-surface">{children}</div>
      {footer !== undefined && <div className="swift-group-foot">{footer}</div>}
    </section>
  );
}

export function InsetRow({
  label,
  hint,
  vertical = false,
  onClick,
  active,
  tone,
  className,
  children,
}: {
  label: ReactNode;
  hint?: ReactNode;
  vertical?: boolean;
  onClick?: () => void;
  active?: boolean;
  tone?: "bad";
  className?: string;
  children?: ReactNode;
}) {
  const Tag = onClick ? "button" : "div";
  return (
    <Tag
      type={onClick ? "button" : undefined}
      onClick={onClick}
      aria-current={onClick && active ? "true" : undefined}
      data-active={active || undefined}
      data-tone={tone}
      className={cn(
        "swift-row",
        vertical && "swift-row-vertical",
        onClick && "swift-row-interactive weir-press-fill",
        className,
      )}
    >
      <div className="swift-row-main">
        <span className="swift-row-label">{label}</span>
        {hint !== undefined && <span className="swift-row-hint">{hint}</span>}
      </div>
      {(children !== undefined || onClick) && (
        <div className="swift-row-control">
          {children}
          {onClick && <ChevronRight className="swift-row-chevron" aria-hidden />}
        </div>
      )}
    </Tag>
  );
}

export function CollapsibleSection({
  title,
  summary,
  defaultOpen = false,
  children,
}: {
  title: ReactNode;
  summary?: ReactNode;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const panelID = useId();
  return (
    <div className="swift-disclosure" data-open={open || undefined}>
      <button
        type="button"
        className="swift-disclosure-trigger weir-press-fill"
        aria-expanded={open}
        aria-controls={panelID}
        onClick={() => setOpen(prev => !prev)}
      >
        <span className="swift-disclosure-title">{title}</span>
        <span className="swift-disclosure-meta">
          {summary !== undefined && <span className="swift-disclosure-summary">{summary}</span>}
          <ChevronMorphIcon open={open} />
        </span>
      </button>
      <div id={panelID} className="swift-disclosure-expanse" inert={!open} aria-hidden={!open}>
        <fieldset className="swift-disclosure-clip" disabled={!open}>
          <div className="swift-disclosure-body">{children}</div>
        </fieldset>
      </div>
    </div>
  );
}
