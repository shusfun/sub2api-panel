import {cn} from "./cn";
import {date} from "../format";

export type SparkBarVerdict = "ok" | "degraded" | "failed" | "skipped_idle" | "running" | "empty";

export type SparkBarSlot = {
  id: number;
  verdict: SparkBarVerdict;
  duration_ms?: number;
  tokens?: number;
  created_at?: string;
  summary?: string;
};

const verdictNames: Record<SparkBarVerdict, string> = {
  ok: "不降智",
  degraded: "降智需关注",
  failed: "检测失败（不计入比率）",
  skipped_idle: "低峰免检（0 Token）",
  running: "检测中",
  empty: "暂无记录",
};

export function QualitySparkBars({
  bars,
  total = 60,
  captionLeft = "最近 60 次检测",
  onSelect,
  className,
}: {
  bars: readonly SparkBarSlot[];
  total?: number;
  captionLeft?: string;
  onSelect?: (id: number) => void;
  className?: string;
}) {
  const trimmed = bars.slice(-total);
  const padCount = Math.max(0, total - trimmed.length);
  const slots: SparkBarSlot[] = [
    ...Array.from({length: padCount}, (_, i) => ({id: -(i + 1), verdict: "empty" as const})),
    ...trimmed,
  ];
  const firstAt = trimmed[0]?.created_at;
  const lastAt = trimmed[trimmed.length - 1]?.created_at;

  return (
    <div className={cn("swift-qbars", className)}>
      <div className="swift-qbars-cap">
        <span>{captionLeft}</span>
        <span className="weir-digits">{trimmed.length} 条记录</span>
      </div>
      <div className="swift-qbars-track" role="list" aria-label={captionLeft}>
        {slots.map(slot => {
          const interactive = slot.id > 0 && slot.verdict !== "empty" && Boolean(onSelect);
          const tipParts = [verdictNames[slot.verdict]];
          if (slot.duration_ms && slot.duration_ms > 0) tipParts.push(`${(slot.duration_ms / 1000).toFixed(1)}s`);
          if (slot.created_at) tipParts.push(date(slot.created_at));
          if (slot.summary) tipParts.push(slot.summary);
          const tip = tipParts.join(" · ");
          if (!interactive) {
            return <span key={slot.id} role="listitem" className="swift-qbar-slot" data-verdict={slot.verdict} title={tip} />;
          }
          return (
            <button
              key={slot.id}
              type="button"
              className="swift-qbar-slot"
              data-verdict={slot.verdict}
              title={tip}
              aria-label={tip}
              onClick={() => onSelect?.(slot.id)}
            />
          );
        })}
      </div>
      {(firstAt || lastAt) && (
        <div className="swift-qbars-axis">
          <span>{firstAt ? date(firstAt) : ""}</span>
          <span>{lastAt ? date(lastAt) : ""}</span>
        </div>
      )}
    </div>
  );
}
