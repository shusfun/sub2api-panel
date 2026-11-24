import type {ReactNode} from "react";
import {cn} from "cn";

export function WorkspaceStage({
  inspectorOpen = false,
  inspector,
  flush = false,
  className,
  children,
}: {
  inspectorOpen?: boolean;
  inspector?: ReactNode;
  flush?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("swift-stage", inspectorOpen && "has-inspector", className)}>
      <div className={cn("swift-stage-main", flush && "swift-stage-flush")}>{children}</div>
      {inspector}
    </div>
  );
}
