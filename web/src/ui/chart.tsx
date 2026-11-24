import {useEffect, useRef, useState} from "react";
import * as echarts from "echarts/core";
import type {EChartsCoreOption} from "echarts/core";
import {BarChart, LineChart, PieChart} from "echarts/charts";
import {
  AxisPointerComponent,
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  MarkLineComponent,
  TooltipComponent,
} from "echarts/components";
import {CanvasRenderer} from "echarts/renderers";
import {cn} from "cn";

echarts.use([
  BarChart,
  LineChart,
  PieChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DataZoomComponent,
  MarkLineComponent,
  AxisPointerComponent,
  CanvasRenderer,
]);

export type ChartPalette = {
  dark: boolean;
  c1: string;
  c2: string;
  c3: string;
  c4: string;
  c5: string;
  c6: string;
  ink: string;
  ink2: string;
  axis: string;
  grid: string;
  panel: string;
  raised: string;
  line: string;
  well: string;
};

function readPalette(): ChartPalette {
  const root = document.documentElement;
  const css = getComputedStyle(root);
  const pick = (name: string, fallback: string) => css.getPropertyValue(name).trim() || fallback;
  return {
    dark: root.classList.contains("dark"),
    c1: pick("--weir-chart-1", "#4f5bd5"),
    c2: pick("--weir-chart-2", "#7c3aed"),
    c3: pick("--weir-chart-3", "#0891b2"),
    c4: pick("--weir-chart-4", "#16a34a"),
    c5: pick("--weir-chart-5", "#d97706"),
    c6: pick("--weir-chart-6", "#e11d48"),
    ink: pick("--weir-ink", "#1b1c1f"),
    ink2: pick("--weir-ink-2", "#4f5259"),
    axis: pick("--weir-chart-axis", "#606369"),
    grid: pick("--weir-chart-grid", "rgba(27,28,31,0.08)"),
    panel: pick("--weir-panel", "#f5f5f3"),
    raised: pick("--weir-raised", "#f9f9f7"),
    line: pick("--weir-line", "rgba(27,28,31,0.15)"),
    well: pick("--weir-well", "#efefec"),
  };
}

export function useChartPalette(): ChartPalette {
  const [palette, setPalette] = useState<ChartPalette>(() => readPalette());
  useEffect(() => {
    const update = () => setPalette(readPalette());
    update();
    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, {attributes: true, attributeFilter: ["class"]});
    return () => observer.disconnect();
  }, []);
  return palette;
}

export type ChartClickPayload = {
  name?: string;
  seriesName?: string;
  dataIndex?: number;
};

const chartGroups = new Map<string, number>();

export function Chart({
  option,
  height = 320,
  className,
  onPointClick,
  ariaLabel,
  group,
  heightToken,
}: {
  option: EChartsCoreOption;
  height?: number;
  className?: string;
  onPointClick?: (payload: ChartClickPayload) => void;
  ariaLabel?: string;
  group?: string;
  heightToken?: string;
}) {
  const container = useRef<HTMLDivElement>(null);
  const instance = useRef<echarts.ECharts | null>(null);
  const clickRef = useRef(onPointClick);
  clickRef.current = onPointClick;

  useEffect(() => {
    const node = container.current;
    if (!node) return;
    const chart = echarts.init(node, undefined, {renderer: "canvas"});
    if (group) {
      chart.group = group;
      chartGroups.set(group, (chartGroups.get(group) ?? 0) + 1);
      echarts.connect(group);
    }
    instance.current = chart;
    const handleClick = (params: {name?: string; seriesName?: string; dataIndex?: number}) => {
      clickRef.current?.({name: params.name, seriesName: params.seriesName, dataIndex: params.dataIndex});
    };
    chart.on("click", handleClick);
    const resizeObserver = new ResizeObserver(() => {
      chart.resize();
    });
    resizeObserver.observe(node);
    return () => {
      resizeObserver.disconnect();
      chart.off("click", handleClick);
      chart.dispose();
      if (group) {
        const count = (chartGroups.get(group) ?? 1) - 1;
        if (count > 0) chartGroups.set(group, count);
        else {
          chartGroups.delete(group);
          echarts.disconnect(group);
        }
      }
      instance.current = null;
    };
  }, [group]);

  useEffect(() => {
    instance.current?.setOption(option, {replaceMerge: ["series"], lazyUpdate: true});
  }, [option]);

  return (
    <div
      ref={container}
      role="img"
      aria-label={ariaLabel}
      className={cn("obs-chart", onPointClick && "obs-chart-clickable", className)}
      style={{height: heightToken ? "var(" + heightToken + ")" : height}}
    />
  );
}
