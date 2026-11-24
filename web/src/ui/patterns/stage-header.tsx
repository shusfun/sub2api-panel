import type {ReactNode} from "react";
import {EllipsisText} from "../primitives/tooltip";

export function StageHeader({
  title,
  leading,
  meta,
  actions,
}: {
  title: string;
  leading?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="weir-stage-header">
      <div className="weir-stage-header-lead">
        {leading}
        <h1 className="weir-stage-header-title">
          <EllipsisText text={title} tabIndex={0} />
        </h1>
        {meta && <div className="weir-stage-header-meta">{meta}</div>}
      </div>
      {actions && <div className="weir-stage-header-actions">{actions}</div>}
    </header>
  );
}
