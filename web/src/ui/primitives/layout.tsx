import type {ComponentProps} from "react";
import {cn} from "cn";

export function Section({className, ...props}: ComponentProps<"section">) {
  return <section className={cn("section", className)} {...props} />;
}

export function Toolbar({className, ...props}: ComponentProps<"div">) {
  return <div className={cn("toolbar", className)} {...props} />;
}

export function WorkspaceContent({className, divided, ...props}: ComponentProps<"div"> & {divided?: boolean}) {
  return <div className={cn("workspace-content-grid", className)} data-divided={divided || undefined} {...props} />;
}

export function InlineStack({className, wrap, density, justify, ...props}: ComponentProps<"div"> & {
  wrap?: boolean;
  density?: "compact";
  justify?: "between" | "center";
}) {
  return <div className={cn("inline-stack", className)} data-wrap={wrap || undefined} data-density={density} data-justify={justify} {...props} />;
}

export function Stack({className, ...props}: ComponentProps<"div">) {
  return <div className={cn("stack", className)} {...props} />;
}

export function RowActions({className, ...props}: ComponentProps<"div">) {
  return <div className={cn("row-actions", className)} onClick={event => event.stopPropagation()} {...props} />;
}

export function SecondaryTableAction(props: ComponentProps<"span">) {
  return <span className="table-secondary-action" {...props} />;
}
