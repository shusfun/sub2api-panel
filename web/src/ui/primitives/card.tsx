import type {ComponentProps, ReactNode} from "react";
import {cn} from "cn";

export function Panel({
  title,
  subtitle,
  badge,
  actions,
  loading,
  padded = true,
  className,
  children,
  ...props
}: Omit<ComponentProps<"section">, "title"> & {
  title?: ReactNode;
  subtitle?: ReactNode;
  badge?: ReactNode;
  actions?: ReactNode;
  loading?: boolean;
  padded?: boolean;
}) {
  const hasHeader = title !== undefined || subtitle !== undefined || actions !== undefined || badge !== undefined;
  return (
    <section
      data-slot="panel"
      data-loading={loading || undefined}
      className={cn("obs-panel", !padded && "obs-panel-flush", className)}
      {...props}
    >
      {loading && <div className="obs-loading-bar" aria-hidden />}
      {hasHeader && (
        <header className="obs-panel-head">
          <div className="obs-panel-title-wrap">
            {title !== undefined && <div className="obs-panel-title-row"><h2>{title}</h2>{badge}</div>}
            {subtitle !== undefined && <p className="obs-panel-subtitle">{subtitle}</p>}
          </div>
          {actions !== undefined && <div className="obs-panel-actions">{actions}</div>}
        </header>
      )}
      <div className={cn("obs-panel-body", !padded && "obs-panel-body-flush")}>{children}</div>
    </section>
  );
}
