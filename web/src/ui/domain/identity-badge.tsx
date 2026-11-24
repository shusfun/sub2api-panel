import type {ReactNode} from "react";
import {cn} from "cn";

function monogram(text: string): string {
  const trimmed = text.trim();
  if (!trimmed) return "?";
  const parts = trimmed.split(/[\s@._-]+/).filter(Boolean);
  if (parts.length >= 2 && /^[a-zA-Z0-9]/.test(parts[0]) && /^[a-zA-Z0-9]/.test(parts[1])) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  return Array.from(trimmed).slice(0, 1).join("").toUpperCase();
}

export function IdentityBadge({
  name,
  meta,
  badge,
  tone = "neutral",
  className,
}: {
  name: string;
  meta?: ReactNode;
  badge?: string;
  tone?: "neutral" | "accent" | "ok" | "warn" | "bad";
  className?: string;
}) {
  return (
    <span className={cn("weir-identity-badge", className)}>
      <span className="weir-identity-plate" data-tone={tone} aria-hidden>
        {badge ?? monogram(name)}
      </span>
      <span className="weir-identity-copy">
        <span className="weir-identity-name">{name}</span>
        {meta !== undefined && meta !== "" && <span className="weir-identity-meta">{meta}</span>}
      </span>
    </span>
  );
}
