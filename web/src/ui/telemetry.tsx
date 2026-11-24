import type {ReactNode} from "react";
import {cn} from "cn";
import {durationText} from "../format";

export type TelemetryTone = "neutral" | "accent" | "ok" | "warn" | "bad" | "cyan" | "violet";

const toneColor: Record<TelemetryTone, string> = {
  neutral: "var(--weir-ink-3)",
  accent: "var(--weir-chart-1)",
  violet: "var(--weir-chart-2)",
  cyan: "var(--weir-chart-3)",
  ok: "var(--weir-ok)",
  warn: "var(--weir-chart-5)",
  bad: "var(--weir-bad)",
};

export function StatusDot({tone = "neutral", pulse = false}: {tone?: TelemetryTone; pulse?: boolean}) {
  return <span className={cn("swift-dot", pulse && "swift-dot-live")} data-tone={tone} aria-hidden />;
}

export function StatusBanner({
  tone = "ok",
  title,
  subtitle,
  actions,
}: {
  tone?: "ok" | "warn" | "bad" | "info";
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
}) {
  const dotTone: TelemetryTone = tone === "info" ? "accent" : tone;
  return (
    <div className="swift-banner" data-tone={tone} role="status">
      <div className="swift-banner-main">
        <StatusDot tone={dotTone} pulse={tone === "ok" || tone === "bad"} />
        <div className="swift-banner-text">
          <strong className="swift-banner-title">{title}</strong>
          {subtitle !== undefined && <span className="swift-banner-sub">{subtitle}</span>}
        </div>
      </div>
      {actions !== undefined && <div className="swift-banner-actions">{actions}</div>}
    </div>
  );
}

function MiniAreaSpark({values, color}: {values: readonly number[]; color: string}) {
  if (values.length < 2) return null;
  const max = Math.max(...values);
  const min = Math.min(...values);
  const span = max - min || 1;
  const w = 91;
  const h = 29;
  const pad = 3;
  const step = (w - pad * 2) / (values.length - 1);
  const pts = values.map((v, i) => [pad + i * step, h - pad - ((v - min) / span) * (h - pad * 2)] as const);
  const line = pts.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const area = `${line} L${(w - pad).toFixed(1)},${h} L${pad},${h} Z`;
  return (
    <svg className="swift-spark" viewBox={`0 0 ${w} ${h}`} aria-hidden>
      <path d={area} fill={color} fillOpacity="0.13" />
      <path d={line} fill="none" stroke={color} strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export type MetricStripItem = {
  id: string;
  label: ReactNode;
  value: ReactNode | null;
  unit?: string;
  hint?: ReactNode;
  tone?: TelemetryTone;
  spark?: readonly number[];
  onClick?: () => void;
};

export function MetricStrip({
  items,
  trailing,
  flush = false,
  className,
}: {
  items: readonly MetricStripItem[];
  trailing?: ReactNode;
  flush?: boolean;
  className?: string;
}) {
  return (
    <div className={cn("swift-strip", flush && "swift-strip-flush", className)}>
      <div className="swift-strip-cells">
        {items.map(item => {
          const tone = item.tone ?? "neutral";
          const Tag = item.onClick ? "button" : "div";
          return (
            <Tag
              key={item.id}
              type={item.onClick ? "button" : undefined}
              onClick={item.onClick}
              data-tone={tone}
              className={cn("swift-strip-cell", item.onClick && "swift-strip-clickable weir-press-fill")}
            >
              <div className="swift-strip-head">
                <span className="swift-strip-label">{item.label}</span>
                {item.spark && item.spark.length >= 2 && <MiniAreaSpark values={item.spark} color={toneColor[tone === "neutral" ? "accent" : tone]} />}
              </div>
              <div className="swift-strip-val-row">
                <span className="swift-strip-val weir-digits">
                  {item.value === null || item.value === undefined ? "未知" : item.value}
                </span>
                {item.unit && <span className="swift-strip-unit">{item.unit}</span>}
              </div>
              {item.hint !== undefined && <div className="swift-strip-hint">{item.hint}</div>}
            </Tag>
          );
        })}
      </div>
      {trailing !== undefined && <div className="swift-strip-trailing">{trailing}</div>}
    </div>
  );
}

export type SegmentItem = {
  label: string;
  value: number;
  tone: TelemetryTone;
};

export function SegmentBar({
  segments,
  compact = false,
  showLegend = false,
  className,
}: {
  segments: readonly SegmentItem[];
  compact?: boolean;
  showLegend?: boolean;
  className?: string;
}) {
  const total = segments.reduce((sum, item) => sum + Math.max(0, item.value), 0);
  if (total <= 0) {
    return <span className="muted text-fine">暂无数据</span>;
  }
  return (
    <div className={cn("swift-segbar", compact && "swift-segbar-compact", className)}>
      <div className="swift-segbar-track" role="img" aria-label={segments.map(s => `${s.label} ${s.value}`).join("，")}>
        {segments.filter(s => s.value > 0).map(item => (
          <span
            key={item.label}
            className="swift-segbar-piece"
            style={{width: `${(item.value / total) * 100}%`, background: toneColor[item.tone]}}
            title={`${item.label}: ${item.value}`}
          />
        ))}
      </div>
      {showLegend && (
        <div className="swift-segbar-legend">
          {segments.map(item => (
            <span key={item.label} className="swift-segbar-key">
              <i style={{background: toneColor[item.tone]}} aria-hidden />
              <span>{item.label}</span>
              <strong className="weir-digits">{item.value}</strong>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

export type TimingPhase = {
  label: string;
  ms: number | null | undefined;
  tone: TelemetryTone;
};

export function TimingBar({phases}: {phases: readonly TimingPhase[]}) {
  const valid = phases.filter((p): p is {label: string; ms: number; tone: TelemetryTone} => typeof p.ms === "number" && Number.isFinite(p.ms) && p.ms >= 0);
  if (!valid.length) return <span className="muted text-fine">未记录阶段耗时</span>;
  const total = valid.reduce((sum, p) => sum + Math.max(1, p.ms), 0);
  return (
    <div className="swift-timing">
      <div className="swift-timing-track">
        {valid.map(phase => (
          <span
            key={phase.label}
            className="swift-timing-piece"
            style={{width: `${(Math.max(1, phase.ms) / total) * 100}%`, background: toneColor[phase.tone]}}
            title={`${phase.label}: ${durationText(phase.ms)}`}
          />
        ))}
      </div>
      <div className="swift-timing-list">
        {phases.map(phase => (
          <div key={phase.label} className="swift-timing-item">
            <span className="swift-timing-name">
              <i style={{background: toneColor[phase.tone]}} aria-hidden />
              {phase.label}
            </span>
            <strong className="weir-digits">{durationText(phase.ms)}</strong>
          </div>
        ))}
      </div>
    </div>
  );
}
