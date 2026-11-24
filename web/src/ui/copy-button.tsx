import {useEffect, useRef, useState, type ReactNode} from "react";
import {Button} from "./button";
import {CopyIcon} from "./copy-icon";

export function CopyButton({
  value,
  getText,
  label = "复制",
  variant = "ghost",
  size = "icon-sm",
  children,
  onCopied,
  className,
}: {
  value?: string;
  getText?: () => Promise<string> | string;
  label?: string;
  variant?: "default" | "outline" | "ghost" | "secondary";
  size?: "default" | "xs" | "sm" | "icon" | "icon-xs" | "icon-sm";
  children?: ReactNode;
  onCopied?: () => void;
  className?: string;
}) {
  const [feedback, setFeedback] = useState<"已复制" | "复制失败">();
  const [busy, setBusy] = useState(false);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(timer.current), []);

  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      busy={busy}
      className={className}
      title={feedback ?? label}
      aria-label={feedback ? `${label}（${feedback}）` : label}
      onClick={event => {
        event.stopPropagation();
        void (async () => {
          setBusy(true);
          window.clearTimeout(timer.current);
          try {
            if (getText && typeof ClipboardItem === "function" && navigator.clipboard.write) {
              const data = Promise.resolve(getText()).then(text => {
                if (!text) throw new Error("复制内容为空");
                return new Blob([text], {type: "text/plain"});
              });
              // Initiate the write during the click while the protected value is fetched.
              await Promise.all([data, navigator.clipboard.write([new ClipboardItem({"text/plain": data})])]);
            } else {
              const text = getText ? await getText() : value;
              if (!text) return;
              await navigator.clipboard.writeText(text);
            }
            setFeedback("已复制");
            onCopied?.();
          } catch {
            setFeedback("复制失败");
          } finally {
            timer.current = window.setTimeout(() => setFeedback(undefined), 1800);
            setBusy(false);
          }
        })();
      }}
    >
      <CopyIcon copied={feedback === "已复制"} />
      {children}
    </Button>
  );
}
