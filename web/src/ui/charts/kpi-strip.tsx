import type {ReactNode} from "react";
import type {TelemetryTone} from "./telemetry";

export type KpiItem = {
  id: string;
  label: string;
  value: ReactNode;
  delta?: ReactNode;
  tone?: TelemetryTone;
  spark?: readonly number[];
  // 有 onClick 时整块渲染为按钮；actionLabel 说明跳转目标，供读屏使用。
  onClick?: () => void;
  actionLabel?: string;
};

const sparkWidth = 91;
const sparkHeight = 29;
const sparkPad = 3;

// 迷你趋势线：纵轴下限固定为 0（有负值时取最小值），上限至少比下限高 1，
// 避免全是 0 的序列里一次报错把线顶满。
function KpiSpark({values}: {values: readonly number[]}) {
  if (values.length < 2) return null;
  const lower = Math.min(0, ...values);
  const upper = Math.max(lower + 1, ...values);
  const step = (sparkWidth - sparkPad * 2) / (values.length - 1);
  const points = values.map((value, index) => [
    sparkPad + index * step,
    sparkHeight - sparkPad - ((value - lower) / (upper - lower)) * (sparkHeight - sparkPad * 2),
  ] as const);
  const line = points.map(([x, y], index) => `${index === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const area = `${line} L${(sparkWidth - sparkPad).toFixed(1)},${sparkHeight} L${sparkPad},${sparkHeight} Z`;
  return (
    <svg className="weir-kpi-spark" viewBox={`0 0 ${sparkWidth} ${sparkHeight}`} aria-hidden>
      <path d={area} fill="currentColor" fillOpacity="0.13" />
      <path d={line} fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function KpiBody({item}: {item: KpiItem}) {
  return (
    <>
      <span className="weir-kpi-label">{item.label}</span>
      <span className="weir-kpi-value weir-digits">{item.value}</span>
      {item.delta !== undefined && <span className="weir-kpi-delta">{item.delta}</span>}
      {item.spark && <KpiSpark values={item.spark} />}
      {item.onClick && item.actionLabel && <span className="weir-kpi-hint">，{item.actionLabel}</span>}
    </>
  );
}

// 状态页 KPI 条：标签、大数字、变化说明，右侧带迷你趋势线。
// 用 ul/li + span，这样可点的项能合法地渲染成 button。
export function KpiStrip({items}: {items: readonly KpiItem[]}) {
  return (
    <ul className="weir-kpi-strip">
      {items.map(item => (
        <li key={item.id}>
          {item.onClick ? (
            <button type="button" className="weir-kpi" data-tone={item.tone} data-action="" onClick={item.onClick}>
              <KpiBody item={item} />
            </button>
          ) : (
            <div className="weir-kpi" data-tone={item.tone}>
              <KpiBody item={item} />
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}
