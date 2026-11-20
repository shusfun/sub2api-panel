import type { ComponentProps } from "react";
import { cn } from "cn";
import { fieldClass } from "./input";

export function Select({ className, ...props }: ComponentProps<"select">) {
  return <select data-slot="select" className={cn(fieldClass, className)} {...props} />;
}
