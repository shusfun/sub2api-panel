// 来源 codedthemes/uiable button；控制台样式：系统蓝实心主键、白面描边次级键、透明键，按下缩到 0.97 并叠填充色（.weir-key），松手弹簧回弹。
import { Button as ButtonPrimitive } from "@base-ui/react/button";
import { cva, type VariantProps } from "class-variance-authority";
import { LoaderCircle } from "lucide-react";
import { cn } from "cn";
import { Tooltip } from "./tooltip";
import { useFormContext } from "./form";
import type {Ref} from "react";

// 文字与图标按钮共用尺寸档位：默认 29、小号 25、更小 21，触屏至少 45。尺寸与内边距统一由公共样式决定。
const buttonVariants = cva(
  "weir-key weir-button group/button inline-flex shrink-0 items-center justify-center rounded-[var(--weir-radius)] font-normal whitespace-nowrap select-none disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-[var(--weir-button-icon)]",
  {
    variants: {
      variant: {
        // 主按钮：系统蓝实心，一个区域最多一个
        default: "weir-key-solid bg-[var(--weir-accent)] text-[var(--weir-accent-ink)] font-medium hover:bg-[var(--weir-accent-lo)]",
        // 次级按钮：白面加 1px 描边，悬停沉入井面色
        outline: "bg-[var(--weir-raised)] text-[var(--weir-ink)] border-[var(--weir-line)] hover:bg-[var(--weir-well)]",
        // 选中态：分段控件同款白色滑块
        secondary: "is-pressed text-[var(--weir-ink)]",
        ghost: "weir-key-ghost text-[var(--weir-ink-2)] hover:text-[var(--weir-ink)]",
        // 危险操作入口：软红底配红字；常规删除确认用 danger-outline 红色描边；高危确认用 danger 实心红
        destructive: "bg-[var(--weir-bad-soft)] text-[var(--weir-bad-ink)] hover:bg-[var(--weir-bad-soft-hi)]",
        "danger-outline": "bg-[var(--weir-raised)] text-[var(--weir-bad-ink)] border-[var(--weir-bad)] font-medium hover:bg-[var(--weir-bad-soft)]",
        danger: "weir-key-solid bg-[var(--weir-bad)] text-[var(--weir-accent-ink)] font-medium hover:opacity-90",
        // 文字链接：按下变淡（iOS 的 plain 按钮），不缩放、不叠填充
        link: "weir-key-link text-[var(--weir-accent-text)] underline-offset-4 hover:underline",
      },
      size: {
        default: "weir-button-default",
        xs: "weir-button-xs",
        sm: "weir-button-sm",
        lg: "weir-button-lg rounded-[var(--weir-radius-card)] font-medium",
        icon: "weir-button-default weir-button-icon",
        "icon-xs": "weir-button-xs weir-button-icon",
        "icon-sm": "weir-button-sm weir-button-icon",
        "icon-lg": "weir-button-lg weir-button-icon",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

// busy：请求进行中，按钮禁用并把文字换成 busyText（默认「正在处理…」）。两段文字叠在同一格里、
// 宽度取较长的那段，切换时按钮和旁边的按钮都不移动。外层格也要继承 gap，否则内层取到 0、图标贴字。只有图标的按钮（size 为 icon*）没有文字可换：
// 图标原位换成转圈，busyText 作为读屏文字。
function Button({
  className,
  variant = "default",
  size = "default",
  nativeButton,
  busy,
  busyText = "正在处理…",
  disabled,
  title,
  ref: buttonRef,
  children,
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants> & { busy?: boolean; busyText?: string; ref?: Ref<HTMLButtonElement> }) {
  const form = useFormContext();
  const submits = "type" in props && props.type === "submit";
  const formID = "form" in props ? props.form : undefined;
  const iconOnly = typeof size === "string" && size.startsWith("icon");
  const label =
    busy === undefined ? children : iconOnly ? (
      <span className="grid place-items-center [&>*]:col-start-1 [&>*]:row-start-1">
        <span className={cn("inline-flex", busy && "invisible")} aria-hidden={busy || undefined}>{children}</span>
        {busy && <LoaderCircle className="animate-spin motion-reduce:animate-none" aria-label={busyText} />}
      </span>
    ) : (
      <span className="grid items-center justify-items-center gap-[inherit] [&>*]:col-start-1 [&>*]:row-start-1">
        <span className={cn("inline-flex items-center gap-[inherit]", busy && "invisible")} aria-hidden={busy || undefined}>{children}</span>
        <span className={cn(!busy && "invisible")} aria-hidden={!busy || undefined}>{busyText}</span>
      </span>
    );
  const button = (
    <ButtonPrimitive
      data-slot="button"
      ref={buttonRef}
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      // 以链接等非 <button> 元素渲染时（render={<Link/>}）告知 Base UI，避免丢失按钮语义的警告
      nativeButton={nativeButton ?? !props.render}
      disabled={disabled || busy || (submits && form?.busy)}
      aria-busy={busy || undefined}
      {...props}
      form={formID ?? (submits ? form?.id : undefined)}
    >
      {label}
    </ButtonPrimitive>
  );
  return title ? <Tooltip content={title} side="bottom">{button}</Tooltip> : button;
}

export { Button, buttonVariants };
