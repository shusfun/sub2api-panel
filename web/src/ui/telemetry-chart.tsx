import {InlineStack} from "./layout";
import {useId, useMemo, useState, type CSSProperties, type ReactNode} from "react";
import * as echarts from "echarts/core";
import type {EChartsCoreOption} from "echarts/core";
import {Chart, useChartPalette} from "./chart";
import {EmptyState} from "./feedback";
import {Panel} from "./card";
import {Segmented} from "./segmented";
import {EllipsisText} from "./tooltip";

function withAlpha(color: string, alpha: number): string {
  const hex = color.trim();
  if (/^#[0-9a-fA-F]{6}$/.test(hex)) {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  return color;
}

function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, ch => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[ch] ?? ch);
}

export type HeroSeries = {
  name: string;
  values: readonly (number | null)[];
  color?: string;
  kind?: "area" | "line" | "bar";
  stack?: string;
};

export type HeroChartTab = {
  id: string;
  label: ReactNode;
  value: ReactNode | null;
  hint?: ReactNode;
  accent?: string;
  categories: readonly string[];
  series: readonly HeroSeries[];
  format?: (val: number) => string;
  onPointClick?: (index: number, seriesName?: string) => void;
};

export function MetricTrend({metric, group, switchable = true, hideValue, emptyTitle, extra, primary = false}: {
  metric: HeroChartTab; group?: string; switchable?: boolean; hideValue?: boolean; emptyTitle?: string; extra?: ReactNode; primary?: boolean;
}) {
  const [kind, setKind] = useState<"line" | "bar">(metric.series.every(series => series.kind === "bar") ? "bar" : "line");
  const title = typeof metric.label === "string" ? metric.label : metric.id;
  const series = useMemo(() => switchable ? metric.series.map(value => ({...value, kind})) : metric.series, [metric.series, kind, switchable]);
  const chart = useMemo(() => [{...metric, series}], [metric, series]);
  return <Panel title={title} className="metric-trend" data-primary={primary || undefined} actions={<>
    {extra}
    {switchable && <Segmented label={title + "图型"} value={kind} options={[["line", "折线"], ["bar", "柱状"]] as const} onChange={setKind} />}
  </>}>
    {!hideValue && metric.value != null && <strong aria-label={title + "汇总"} className="metric-trend-value weir-digits">{metric.value}</strong>}
    <HeroMetricChart embedded tabs={chart} heightToken={primary ? "--weir-chart-primary-height" : "--weir-chart-secondary-height"} group={group} emptyTitle={emptyTitle} />
  </Panel>;
}

export function HeroMetricChart({
  tabs,
  activeTab,
  onTabChange,
  actions,
  height = 239,
  heightToken,
  group,
  embedded = false,
  emptyTitle = "当前时间窗口暂无曲线样本",
}: {
  tabs: readonly HeroChartTab[];
  activeTab?: string;
  onTabChange?: (id: string) => void;
  actions?: ReactNode;
  height?: number;
  heightToken?: string;
  group?: string;
  embedded?: boolean;
  emptyTitle?: string;
}) {
  const id = useId();
  const [internalTab, setInternalTab] = useState(tabs[0]?.id ?? "");
  const selectedID = activeTab ?? internalTab;
  const current = tabs.find(t => t.id === selectedID) ?? tabs[0];
  const palette = useChartPalette();

  const colors = useMemo(
    () => [palette.c1, palette.c3, palette.c2, palette.c4, palette.c5, palette.c6],
    [palette],
  );
  const seriesColors = useMemo(
    () => current?.series.map((series, index) => series.color ?? (index === 0 ? current.accent : undefined) ?? colors[index % colors.length]) ?? [],
    [colors, current],
  );
  const samples = current?.series.flatMap(series => series.values.filter(value => value !== null && Number.isFinite(value))) ?? [];
  const hasSamples = samples.length > 0;
  const allZero = hasSamples && samples.every(value => value === 0);
  const showTabs = !embedded && tabs.length > 1;

  function selectTab(tabId: string) {
    setInternalTab(tabId);
    onTabChange?.(tabId);
  }

  const option = useMemo<EChartsCoreOption | null>(() => {
    if (!current || current.categories.length === 0 || !hasSamples) return null;
    const fmt = current.format ?? ((n: number) => n.toLocaleString());
    const hasBars = current.series.some(s => s.kind === "bar");
    const onlyLines = current.series.every(s => s.kind !== "bar");

    return {
      animation: false,
      grid: {top: 15, right: 17, bottom: 7, left: 9, containLabel: true},
      legend: {show: false},
      tooltip: {
        trigger: "axis",
        confine: true,
        backgroundColor: palette.raised,
        borderColor: palette.line,
        borderWidth: 1,
        padding: [9, 13],
        textStyle: {color: palette.ink, fontSize: 12},
        extraCssText: "border-radius: 13px; box-shadow: 0 13px 33px rgba(0,0,0,0.28);",
        axisPointer: {
          type: hasBars && !onlyLines ? "shadow" : "line",
          lineStyle: {color: palette.line, width: 1, type: "dashed"},
          shadowStyle: {color: withAlpha(palette.c1, 0.06)},
        },
        formatter: (raw: unknown) => {
          const items = Array.isArray(raw) ? raw : [raw];
          if (!items[0]) return "";
          const first = items[0] as {axisValueLabel?: string; name?: string};
          const head = escapeHtml(String(first.axisValueLabel ?? first.name ?? ""));
          const rows = (items as {seriesName?: string; value?: number | null; color?: unknown}[])
            .filter(it => typeof it.value === "number" && Number.isFinite(it.value))
            .map(it => {
              const dotColor = typeof it.color === "string" ? it.color : palette.c1;
              const valText = typeof it.value === "number" ? fmt(it.value) : String(it.value);
              return `<div style="display:flex;align-items:center;justify-content:space-between;gap:19px;margin-top:5px;">
                <span style="display:inline-flex;align-items:center;gap:7px;color:${palette.ink2};">
                  <i style="width:7px;height:7px;border-radius:99px;background:${dotColor};display:inline-block;"></i>
                  ${escapeHtml(String(it.seriesName ?? ""))}
                </span>
                <strong style="font-family:var(--weir-mono);font-variant-numeric:tabular-nums;color:${palette.ink};">${escapeHtml(valText)}</strong>
              </div>`;
            })
            .join("");
          return `<div style="min-width:149px;">
            <div style="font-family:var(--weir-mono);font-size:11px;color:${palette.axis};margin-bottom:3px;">${head}</div>
            ${rows}
          </div>`;
        },
      },
      xAxis: {
        type: "category",
        boundaryGap: !onlyLines || current.categories.length < 2,
        data: current.categories,
        axisTick: {show: false},
        axisLine: {lineStyle: {color: palette.grid}},
        axisLabel: {
          color: palette.axis,
          fontSize: 11,
          margin: 11,
          hideOverlap: true,
        },
      },
      yAxis: {
        type: "value",
          splitNumber: embedded ? 3 : 4,
        axisLine: {show: false},
        axisTick: {show: false},
        splitLine: {lineStyle: {color: palette.grid, type: "solid"}},
        axisLabel: {
          color: palette.axis,
          fontSize: 11,
          formatter: (val: number) => fmt(val),
        },
      },
      series: current.series.map((s, idx) => {
        const stroke = seriesColors[idx];
        const kind = s.kind ?? "area";
        if (kind === "bar") {
          return {
            name: s.name,
            type: "bar",
            stack: s.stack,
            barMaxWidth: 21,
            data: s.values,
            itemStyle: {
              color: stroke,
              borderRadius: s.stack ? [2, 2, 0, 0] : [5, 5, 0, 0],
            },
          };
        }
        return {
          name: s.name,
          type: "line",
          smooth: 0.35,
          symbol: "circle",
          symbolSize: (_value: unknown, point: {dataIndex: number}) =>
            s.values[point.dataIndex - 1] == null && s.values[point.dataIndex + 1] == null ? 7 : 4,
          showSymbol: Boolean(current.onPointClick) || s.values.some((value, index) =>
            value != null && s.values[index - 1] == null && s.values[index + 1] == null),
          stack: s.stack,
          data: s.values,
          itemStyle: {color: stroke, borderWidth: 2, borderColor: palette.panel},
          lineStyle: {color: stroke, width: 2.2},
          areaStyle:
            kind === "area"
              ? {
                  color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    {offset: 0, color: withAlpha(stroke, s.stack ? 0.32 : 0.26)},
                    {offset: 0.68, color: withAlpha(stroke, 0.06)},
                    {offset: 1, color: withAlpha(stroke, 0)},
                  ]),
                }
              : undefined,
          emphasis: {focus: "series"},
        };
      }),
    };
  }, [current, embedded, hasSamples, palette, seriesColors]);

  if (!current) return null;

  return (
    <div className="swift-hero-chart" data-embedded={embedded || undefined}>
      {showTabs && <div className="swift-hero-tabs" role="tablist" aria-label="图表指标切换">
        {tabs.map((tab, idx) => {
          const active = tab.id === current.id;
          const accent = tab.accent ?? colors[idx % colors.length];
          return (
            <button
              key={tab.id}
              type="button"
              role="tab"
              id={id + "-" + tab.id}
              aria-controls={id}
              aria-selected={active}
              tabIndex={active ? 0 : -1}
              data-active={active || undefined}
              style={{"--tab-accent": accent} as CSSProperties}
              className="swift-hero-tab"
              onClick={() => selectTab(tab.id)}
              onKeyDown={event => {
                const step = event.key === "ArrowLeft" ? -1 : 1;
                let nextIndex;
                switch (event.key) {
                  case "Home": nextIndex = 0; break;
                  case "End": nextIndex = tabs.length - 1; break;
                  case "ArrowLeft":
                  case "ArrowRight": nextIndex = (idx + step + tabs.length) % tabs.length; break;
                  default: return;
                }
                event.preventDefault();
                const next = tabs[nextIndex];
                selectTab(next.id);
                document.getElementById(id + "-" + next.id)?.focus();
              }}
            >
              <span className="swift-hero-tab-label">{tab.label}</span>
              <span className="swift-hero-tab-val">
                {tab.value === null || tab.value === undefined ? "未知" : tab.value}
              </span>
              {tab.hint !== undefined && <span className="swift-hero-tab-hint">{tab.hint}</span>}
            </button>
          );
        })}
      </div>}

      {(current.series.length > 1 || actions) && (
        <div className="swift-hero-toolbar">
          <div className="swift-hero-legend">
            {current.series.length > 1 &&
              current.series.map((s, idx) => (
                <span key={s.name} className="swift-hero-legend-item">
                  <i
                    className="swift-hero-legend-dot"
                    style={{background: seriesColors[idx]}}
                    aria-hidden
                  />
                  <span>{s.name}</span>
                </span>
              ))}
          </div>
          {actions && <InlineStack>{actions}</InlineStack>}
        </div>
      )}

      <div className="swift-hero-plot" id={id} role={showTabs ? "tabpanel" : undefined}
        aria-labelledby={showTabs ? id + "-" + current.id : undefined}>
        {allZero && <p className="swift-hero-zero">当前时段的已知样本均为 0</p>}
        {option ? (
          <Chart
            option={option}
            height={height}
            heightToken={heightToken}
            group={group}
            ariaLabel={typeof current.label === "string" ? current.label : "趋势主图"}
            onPointClick={
              current.onPointClick
                ? payload => {
                    const {dataIndex, seriesName} = payload;
                    if (dataIndex !== undefined && current.series.some(series =>
                      (seriesName === undefined || series.name === seriesName) && series.values[dataIndex] != null)) {
                      current.onPointClick?.(dataIndex, seriesName);
                    }
                  }
                : undefined
            }
          />
        ) : (
          <EmptyState title={emptyTitle} />
        )}
      </div>
    </div>
  );
}

export type RankedBarItem = {
  id: string | number;
  label: ReactNode;
  value: number;
  formatted: string;
  sub?: ReactNode;
  color?: string;
  onClick?: () => void;
};

export function RankedBarList({
  items,
  emptyTitle = "暂无排行数据",
}: {
  items: readonly RankedBarItem[];
  emptyTitle?: string;
}) {
  const palette = useChartPalette();
  const colors = [palette.c1, palette.c3, palette.c2, palette.c4, palette.c5, palette.c6];
  const valid = items.filter(item => Number.isFinite(item.value) && item.value > 0);
  if (!valid.length) {
    return <EmptyState title={emptyTitle} />;
  }
  const total = valid.reduce((sum, item) => sum + item.value, 0);

  return (
    <div className="swift-rank-list">
      {valid.map((item, idx) => {
        const barColor = item.color ?? colors[idx % colors.length];
        const sharePct = (item.value / total) * 100;
        const Tag = item.onClick ? "button" : "div";
        return (
          <Tag
            key={item.id}
            type={item.onClick ? "button" : undefined}
            data-clickable={item.onClick ? "true" : undefined}
            onClick={item.onClick}
            className="swift-rank-row"
          >
            <span className="swift-rank-fill" style={{width: `${sharePct}%`, background: barColor}} aria-hidden />
            <span className="swift-rank-left">
              <i className="swift-rank-dot" style={{background: barColor}} aria-hidden />
              {typeof item.label === "string"
                ? <EllipsisText className="swift-rank-name" text={item.label} />
                : <span className="swift-rank-name">{item.label}</span>}
              {item.sub !== undefined && <span className="swift-rank-sub muted text-fine">{item.sub}</span>}
            </span>
            <span className="swift-rank-right">
              <span className="swift-rank-val">{item.formatted}</span>
              <span className="swift-rank-share">{sharePct >= 10 ? `${Math.round(sharePct)}%` : `${sharePct.toFixed(1)}%`}</span>
            </span>
          </Tag>
        );
      })}
    </div>
  );
}
