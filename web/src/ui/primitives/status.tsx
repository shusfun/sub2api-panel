import {statusLabel} from "@/lib/format";

const okSet = new Set([
  "active", "enabled", "valid", "ok", "succeeded", "completed", "COMPLETED", "PAID",
  "issued", "healthy", "resolved", "manual_resolved", "unused", "allow", "compatible",
]);

const warnSet = new Set([
  "pending", "PENDING", "RECHARGING", "REFUND_REQUESTED", "REFUNDING", "REFUND_PENDING", "PARTIALLY_REFUNDED",
  "running", "starting", "queued", "dumping", "uploading", "indexing", "processing_results", "settling",
  "unknown", "warning", "warn", "expired", "EXPIRED", "suspended", "degraded", "untested", "shadow",
]);

const badSet = new Set([
  "error", "failed", "FAILED", "REFUND_FAILED", "critical", "firing", "revoked",
  "quota_exhausted", "incompatible", "block", "banned", "invalidated",
]);

export function Status({value, label}: {value: string; label?: string}) {
  const tone = okSet.has(value) ? "ok" : warnSet.has(value) ? "warn" : badSet.has(value) ? "bad" : "neutral";
  return (
    <span className="swift-status" data-tone={tone}>
      <i aria-hidden />
      <span>{label ?? statusLabel(value)}</span>
    </span>
  );
}
