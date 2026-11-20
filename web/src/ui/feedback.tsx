import type {ReactNode} from "react";

export function Feedback({children, tone = "neutral"}: {children: ReactNode; tone?: "neutral" | "error" | "success"}) {
  return <p className="feedback" data-tone={tone} role={tone === "error" ? "alert" : "status"}>{children}</p>;
}

export function ErrorLine({value}: {value: string}) {
  return value ? <Feedback tone="error">{value}</Feedback> : null;
}

export function Resource<T>({state, children}: {
  state?: {value?: T; error?: string}; children: (value: T) => ReactNode;
}) {
  if (state?.error) return <ErrorLine value={state.error} />;
  if (state?.value === undefined) return <Feedback>正在加载…</Feedback>;
  return children(state.value);
}
