import type {ComponentProps} from "react";
import {cn} from "cn";

export function Section({className, ...props}: ComponentProps<"section">) {
  return <section className={cn("section", className)} {...props} />;
}

export function Toolbar({className, ...props}: ComponentProps<"div">) {
  return <div className={cn("toolbar", className)} {...props} />;
}
