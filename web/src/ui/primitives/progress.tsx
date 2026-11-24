import {cn} from "cn";

export function MeterBar({
  value,
  label,
  tone = "auto",
  size = "md",
  className,
}: {
  value: number | null;
  label?: string;
  tone?: "auto" | "accent" | "ok" | "warn" | "bad" | "cyan" | "violet";
  size?: "sm" | "md";
  className?: string;
}) {
  if (value === null || !Number.isFinite(value)) {
    return <span className="muted code-mono text-fine">—</span>;
  }
  const clamped = Math.min(100, Math.max(0, value));
  const resolved = tone === "auto"
    ? clamped >= 85 ? "bad" : clamped >= 60 ? "warn" : "ok"
    : tone;
  return (
    <div className={cn("obs-meter", size === "sm" && "obs-meter-sm", className)} data-tone={resolved}>
      <div className="obs-meter-track" role="progressbar" aria-valuenow={Math.round(clamped)} aria-valuemin={0} aria-valuemax={100}>
        <div className="obs-meter-fill" style={{width: `${clamped}%`}} />
      </div>
      {label !== undefined && <span className="obs-meter-label weir-digits">{label}</span>}
    </div>
  );
}
