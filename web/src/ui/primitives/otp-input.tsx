import {useEffect, useState, type ComponentProps} from "react";
import {cn} from "cn";
import {Button} from "./button";
import {Input} from "./input";

export function OtpInput({
  length = 6,
  countdown: externalCountdown,
  sendBusy,
  sendDisabled,
  sendLabel = "发送验证码",
  resendLabel = "重新发送",
  onSendCode,
  className,
  onChange,
  ...props
}: Omit<ComponentProps<typeof Input>, "type" | "maxLength" | "inputMode" | "endAddon"> & {
  length?: number;
  countdown?: number;
  sendBusy?: boolean;
  sendDisabled?: boolean;
  sendLabel?: string;
  resendLabel?: string;
  onSendCode?: () => Promise<number | void> | number | void;
}) {
  const [localCountdown, setLocalCountdown] = useState(0);
  const [sentOnce, setSentOnce] = useState(false);
  const seconds = externalCountdown ?? localCountdown;

  useEffect(() => {
    if (externalCountdown !== undefined || localCountdown <= 0) return;
    const timer = window.setInterval(() => setLocalCountdown(v => Math.max(0, v - 1)), 1000);
    return () => window.clearInterval(timer);
  }, [externalCountdown, localCountdown > 0]);

  return (
    <Input
      {...props}
      inputMode="numeric"
      autoComplete={props.autoComplete ?? "one-time-code"}
      pattern={props.pattern ?? `[0-9]{${length}}`}
      maxLength={length}
      placeholder={props.placeholder ?? "0".repeat(length)}
      className={cn("weir-otp-input", className)}
      onChange={e => {
        const cleaned = e.currentTarget.value.replace(/\D+/g, "").slice(0, length);
        if (cleaned !== e.currentTarget.value) {
          e.currentTarget.value = cleaned;
        }
        onChange?.(e);
      }}
      endAddon={
        onSendCode ? (
          <Button
            type="button"
            size="xs"
            variant="outline"
            busy={sendBusy}
            disabled={props.disabled || sendDisabled || seconds > 0}
            onClick={async () => {
              const next = await onSendCode();
              setSentOnce(true);
              if (typeof next === "number" && next > 0) {
                setLocalCountdown(next);
              } else if (externalCountdown === undefined) {
                setLocalCountdown(60);
              }
            }}
          >
            {seconds > 0 ? `${seconds}s` : sentOnce ? resendLabel : sendLabel}
          </Button>
        ) : undefined
      }
    />
  );
}
