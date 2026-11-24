import {Tooltip as TooltipPrimitive} from "@base-ui/react/tooltip";
import {useLayoutEffect, useRef, useState, type ReactNode} from "react";
import {cn} from "cn";

function TooltipProvider({
  children,
  delay = 220,
  closeDelay = 80,
}: {
  children: ReactNode;
  delay?: number;
  closeDelay?: number;
}) {
  return (
    <TooltipPrimitive.Provider delay={delay} closeDelay={closeDelay}>
      {children}
    </TooltipPrimitive.Provider>
  );
}

function Tooltip({
  content,
  children,
  side = "top",
  disabled = false,
}: {
  content: ReactNode;
  children: TooltipPrimitive.Trigger.Props["render"];
  side?: "top" | "right" | "bottom" | "left";
  disabled?: boolean;
}) {
  return (
    <TooltipPrimitive.Root disabled={disabled || content === undefined || content === null || content === ""}>
      <TooltipPrimitive.Trigger render={children} />
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Positioner side={side} sideOffset={7} className="weir-tooltip-positioner">
          <TooltipPrimitive.Popup className="weir-tooltip-popup">
            {content}
          </TooltipPrimitive.Popup>
        </TooltipPrimitive.Positioner>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}

// 单行或双行省略文本：仅在内容真实截断或显式传入 tip 时弹出石板浮层提示，避免未截断短词乱弹气泡。
function EllipsisText({
  text,
  tip,
  lines = 1,
  mono = false,
  className,
  onClick,
  tabIndex,
}: {
  text: string;
  tip?: ReactNode;
  lines?: 1 | 2;
  mono?: boolean;
  className?: string;
  onClick?: () => void;
  tabIndex?: number;
}) {
  const [truncated, setTruncated] = useState(false);
  const ref = useRef<HTMLSpanElement>(null);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => setTruncated(el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    document.fonts.addEventListener("loadingdone", measure);
    return () => {
      observer.disconnect();
      document.fonts.removeEventListener("loadingdone", measure);
    };
  }, [text, lines]);

  const node = (
    <span
      ref={ref}
      className={cn(lines === 2 ? "weir-ellipsis-2" : "weir-ellipsis-1", mono && "code-mono", className)}
      onClick={onClick}
      tabIndex={tabIndex}
    >
      {text}
    </span>
  );

  const tooltipContent = tip ?? (truncated ? text : undefined);
  return <Tooltip content={tooltipContent} side="top">{node}</Tooltip>;
}

export {Tooltip, TooltipProvider, EllipsisText};
