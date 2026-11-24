import type {ReactNode} from "react";
import {Button} from "../primitives/button";
import {ErrorLine} from "./feedback";
import {FormActions} from "./form";

export function SaveBar({
  dirty,
  dirtyCount,
  busy = false,
  error = "",
  submitLabel = "保存",
  resetLabel = "撤销改动",
  form,
  extra,
  onReset,
}: {
  dirty: boolean;
  dirtyCount?: number;
  busy?: boolean;
  error?: string;
  submitLabel?: string;
  resetLabel?: string;
  form?: string;
  extra?: ReactNode;
  onReset?: () => void;
}) {
  return (
    <div className="weir-savebar">
      <div className="min-w-0">
        {error ? (
          <ErrorLine value={error} />
        ) : (
          <span className="weir-savebar-state" role="status">
            {busy ? "正在保存…" : dirty ? (dirtyCount !== undefined ? `${dirtyCount} 项改动待保存` : "有未保存的改动") : "当前已是最新配置"}
          </span>
        )}
      </div>
      <FormActions>
        {extra}
        {onReset && (
          <Button type="button" variant="ghost" disabled={!dirty || busy} onClick={onReset}>
            {resetLabel}
          </Button>
        )}
        <Button type="submit" form={form} disabled={!dirty} busy={busy}>
          {submitLabel}
        </Button>
      </FormActions>
    </div>
  );
}
