import type {ReactNode} from "react";
import {cn} from "cn";

export function RouteChain({
  nodes,
  className,
}: {
  nodes: readonly (string | ReactNode)[];
  className?: string;
}) {
  const clean = nodes.filter(node => node !== null && node !== undefined && node !== "");
  if (!clean.length) return <span className="muted">—</span>;
  return (
    <span className={cn("weir-route-chain", className)}>
      {clean.map((node, index) => (
        <span key={index} className="weir-route-chain-step">
          {index > 0 && (
            <svg width="13" height="11" viewBox="0 0 16 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden className="weir-route-chain-arrow">
              <path d="M 1 6 L 4 6 M 6.5 6 L 9.5 6 M 11 3 L 14.5 6 L 11 9" />
            </svg>
          )}
          <span className="weir-route-chain-node">{node}</span>
        </span>
      ))}
    </span>
  );
}
