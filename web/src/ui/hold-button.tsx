import {useEffect, useRef, useState, type ReactNode} from "react";
import {cn} from "cn";
import {Button} from "./button";

export function HoldButton({
  onConfirm,
  holdMs = 1000,
  busy = false,
  disabled = false,
  size = "default",
  className,
  children,
}: {
  onConfirm: () => void;
  holdMs?: number;
  busy?: boolean;
  disabled?: boolean;
  size?: "default" | "xs" | "sm" | "lg";
  className?: string;
  children: ReactNode;
}) {
  const [holding, setHolding] = useState(false);
  const timer = useRef<number | undefined>(undefined);

  const stop = () => {
    if (timer.current !== undefined) {
      window.clearTimeout(timer.current);
      timer.current = undefined;
    }
    setHolding(false);
  };

  useEffect(() => stop, []);

  const start = () => {
    if (busy || disabled || timer.current !== undefined) return;
    setHolding(true);
    timer.current = window.setTimeout(() => {
      timer.current = undefined;
      setHolding(false);
      onConfirm();
    }, holdMs);
  };

  return (
    <Button
      type="button"
      variant="danger-outline"
      size={size}
      busy={busy}
      disabled={disabled}
      data-holding={holding || undefined}
      className={cn("weir-hold-button", className)}
      onPointerDown={e => {
        if (e.button !== 0) return;
        start();
      }}
      onPointerUp={stop}
      onPointerLeave={stop}
      onPointerCancel={stop}
      onBlur={stop}
      onKeyDown={e => {
        if ((e.key === " " || e.key === "Enter") && !e.repeat) {
          e.preventDefault();
          start();
        }
      }}
      onKeyUp={e => {
        if (e.key === " " || e.key === "Enter") {
          e.preventDefault();
          stop();
        }
      }}
    >
      <span className="weir-hold-progress" aria-hidden />
      <span className="weir-hold-content">
        <span>{children}</span>
        <small aria-hidden>按住 1 秒</small>
      </span>
    </Button>
  );
}
