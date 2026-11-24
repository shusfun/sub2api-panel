import {useFieldControl} from "./field";
import type {ComponentProps} from "react";
import {cn} from "cn";
import {fieldClass} from "./input";

export function Textarea({className, ...props}: ComponentProps<"textarea">) {
  const {attributes} = useFieldControl(props);
  return <textarea data-slot="textarea" className={cn(fieldClass, "weir-textarea", className)} {...props} {...attributes} />;
}
