import type {ReactNode} from "react";

// 状态页首屏的大字结论；需要处理时，处理按钮直接放在结论下面。
export function Verdict({
  tone,
  title,
  detail,
  actions,
}: {
  tone: "ok" | "warn" | "bad" | "info";
  title: ReactNode;
  detail?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <section className="weir-verdict" data-tone={tone} role="status">
      <span className="weir-verdict-mark" aria-hidden />
      <strong className="weir-verdict-title">{title}</strong>
      {detail !== undefined && <span className="weir-verdict-detail">{detail}</span>}
      {actions !== undefined && <div className="weir-verdict-actions">{actions}</div>}
    </section>
  );
}
