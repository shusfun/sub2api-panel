// 来源 codedthemes/uiable input；本地改为白底细边框（表单参照 Element Plus），聚焦时主色 2px 描边。
// 与下拉和表单按钮共享高度；字号用 control 角色。
import { ComponentProps } from "react";
import { Input as InputPrimitive } from "@base-ui/react/input";
import { cn } from "cn";

// 鼠标点进输入框也画焦点描边（:focus 而不只 :focus-visible），与 iOS / macOS 输入框一致
export const fieldClass =
  "weir-input weir-field-control block w-full min-w-0 text-control text-[var(--weir-ink)] transition-colors focus:outline-2 focus:outline-offset-0 focus:outline-[var(--weir-focus)] disabled:cursor-not-allowed disabled:bg-[var(--weir-fill-4)] disabled:opacity-55 aria-invalid:border-destructive";

function Input({ className, type, ...props }: ComponentProps<"input">) {
  return <InputPrimitive type={type} data-slot="input"
    className={cn(fieldClass, "placeholder:text-[var(--weir-ink-3)] read-only:bg-[var(--weir-fill-4)]", className)} {...props} />;
}

export { Input };
