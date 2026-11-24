import {Button} from "../primitives/button";
import type {ReactNode} from "react";
import {X} from "lucide-react";

export function BatchBar({
  count,
  onClear,
  children,
}: {
  count: number;
  onClear: () => void;
  children: ReactNode;
}) {
  if (count <= 0) return null;
  return (
    <div className="swift-batch-bar" role="region" aria-label="批量操作">
      <span className="swift-batch-count">
        已选 <strong className="weir-digits">{count}</strong> 项
      </span>
      <div className="swift-batch-actions">{children}</div>
      <Button type="button" variant="ghost" size="sm" onClick={onClear} title="清空选择" aria-label="清空选择">
        <X aria-hidden />清空
      </Button>
    </div>
  );
}
