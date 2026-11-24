import {cn} from "cn";
import {CopyButton} from "../patterns/copy-button";

export function GeoExitBadge({
  countryCode,
  country,
  city,
  ip,
  className,
}: {
  countryCode?: string;
  country?: string;
  city?: string;
  ip?: string;
  className?: string;
}) {
  const code = (countryCode ?? "").trim().toUpperCase();
  const location = [country, city].filter(Boolean).join(" · ");
  if (!code && !location && !ip) return <span className="muted">—</span>;
  return (
    <span className={cn("weir-geo-badge", className)}>
      {code && <span className="weir-geo-code">{code}</span>}
      {location && <span className="weir-geo-loc">{location}</span>}
      {ip && (
        <span className="weir-geo-ip">
          <code className="code-mono">{ip}</code>
          <CopyButton value={ip} label={`复制出口 IP ${ip}`} size="icon-xs" />
        </span>
      )}
    </span>
  );
}
