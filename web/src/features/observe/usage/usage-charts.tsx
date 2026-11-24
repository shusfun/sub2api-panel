import { memo,useMemo,useState } from "react";
import {compact, count, rate, smartMoney} from "@/lib/format";
import {DonutChart, EmptyState, MetricTrend, Panel, RankedBarList, Segmented, useChartPalette, type HeroChartTab} from "@/ui";
import type {UsageData, UsageBucket, ModelUsage} from "./index";

const metrics = [["actual_cost", "按扣费"], ["total_tokens", "按 Token"], ["requests", "按请求数"]] as const;
const other = "其他模型";
type Metric = (typeof metrics)[number][0];

const formatCount = (value: number) => compact.format(value);
const formatRate = (value: number) => rate.format(value);

const tokenTotal = (value: UsageData["totals"]) =>
  value.input_tokens + value.output_tokens + value.cache_read_tokens + value.cache_creation_tokens;

const cacheRate = (value: UsageData["totals"]) => {
  const denominator = value.input_tokens + value.cache_read_tokens + value.cache_creation_tokens;
  return denominator > 0 ? value.cache_read_tokens / denominator : null;
};

export function bucketLabel(bucket: UsageBucket, hourly: boolean, includeOffset = false) {
  const label = bucket.start.slice(5, hourly ? 16 : 10).replace("T", " ");
  return includeOffset && hourly ? label + " " + bucket.start.slice(19) : label;
}

export const UsageHeroTrends = memo(function UsageHeroTrends({
  totals,
  buckets,
  hourly,
  group,
  onBucket,
}: {
  totals: UsageData["totals"];
  buckets: UsageBucket[];
  hourly: boolean;
  group: string;
  onBucket: (bucket: string) => void;
}) {
  const palette = useChartPalette();
  const categories = useMemo(() => buckets.map(b => bucketLabel(b, hourly)), [buckets, hourly]);
  const hit = cacheRate(totals);

  const tabs = useMemo<HeroChartTab[]>(() => {
    const clickBucket = (idx: number) => {
      const bucket = buckets[idx];
      if (bucket) onBucket(bucket.start);
    };
    return [
      {
        id: "cost",
        label: "实际扣费",
        value: smartMoney(totals.actual_cost),
        hint: "点击曲线点可下钻对应时段明细",
        accent: palette.c1,
        categories,
        format: smartMoney,
        onPointClick: clickBucket,
        series: [{name: "实际扣费", values: buckets.map(b => (b.pending ? null : b.actual_cost)), color: palette.c1, kind: "area"}],
      },
      {
        id: "requests",
        label: "请求次数",
        value: count.format(totals.requests),
        hint: hourly ? "小时级吞吐脉冲" : "日级调用频次",
        accent: palette.c3,
        categories,
        format: formatCount,
        onPointClick: clickBucket,
        series: [{name: "请求数", values: buckets.map(b => (b.pending ? null : b.requests)), color: palette.c3, kind: "bar"}],
      },
      {
        id: "tokens",
        label: "输入与输出 Token",
        value: compact.format(tokenTotal(totals)),
        hint: `入 ${compact.format(totals.input_tokens)} · 出 ${compact.format(totals.output_tokens)}`,
        accent: palette.c2,
        categories,
        format: formatCount,
        onPointClick: clickBucket,
        series: [
          {name: "输入 Token", values: buckets.map(b => (b.pending ? null : b.input_tokens)), color: palette.c1, kind: "area"},
          {name: "输出 Token", values: buckets.map(b => (b.pending ? null : b.output_tokens)), color: palette.c2, kind: "area"},
        ],
      },
      {
        id: "cache-tokens", label: "缓存读写 Token", value: compact.format(totals.cache_read_tokens + totals.cache_creation_tokens),
        categories, format: formatCount, onPointClick: clickBucket,
        series: [
          {name: "缓存读取", values: buckets.map(b => b.pending ? null : b.cache_read_tokens), color: palette.c3},
          {name: "缓存写入", values: buckets.map(b => b.pending ? null : b.cache_creation_tokens), color: palette.c5},
        ],
      },
      {
        id: "cache",
        label: "缓存命中率",
        value: hit === null ? null : rate.format(hit),
        hint: `缓存读 ${compact.format(totals.cache_read_tokens)} · 写 ${compact.format(totals.cache_creation_tokens)}`,
        accent: palette.c4,
        categories,
        format: formatRate,
        onPointClick: clickBucket,
        series: [{name: "缓存命中率", values: buckets.map(b => (b.pending ? null : cacheRate(b))), color: palette.c4, kind: "area"}],
      },
    ];
  }, [buckets, categories, hit, hourly, onBucket, palette, totals]);

  const emptyTitle = buckets.some(bucket => bucket.pending) ? "所选时段正在同步，暂无可用样本" : undefined;
  return <div className="split-grid-2">{tabs.map((metric, index) => <MetricTrend key={metric.id} metric={metric} primary={index === 0} group={group} emptyTitle={emptyTitle} />)}</div>;
});

export const UsageModels = memo(function UsageModels({
  ranking,
  analysis,
  buckets,
  hourly,
  group,
  onModel,
  onBucket,
  onPoint,
}: {
  ranking: ModelUsage[];
  analysis: UsageData["model_series"];
  buckets: UsageBucket[];
  hourly: boolean;
  group: string;
  onModel: (model: string) => void;
  onBucket: (bucket: string) => void;
  onPoint: (model: string, bucket: string) => void;
}) {
  const [metric, setMetric] = useState<Metric>("actual_cost");
  const palette = useChartPalette();
  const format = metric === "actual_cost" ? smartMoney : formatCount;

  const ranked = useMemo(
    () => [...ranking].sort((a, b) => b[metric] - a[metric] || a.model.localeCompare(b.model)),
    [metric, ranking],
  );
  const first = ranked.slice(0, 6);
  const rest = ranked.slice(6).reduce((sum, row) => sum + row[metric], 0);
  const colors = [palette.c1, palette.c3, palette.c6, palette.c4, palette.c2, palette.c5];
  const colorIndex = new Map([...ranking].sort((a, b) => a.model.localeCompare(b.model)).map((row, index) => [row.model, index]));
  const slices = first.map(row => ({
    name: row.model,
    value: row[metric],
    requests: row.requests,
    color: colors[(colorIndex.get(row.model) ?? 0) % colors.length],
  }));
  if (rest > 0) slices.push({name: other, value: rest, requests: 0, color: palette.axis});
  const top = new Set(first.map(row => row.model));
  const perBucket = new Map(analysis.buckets.map(row => [row.start, row.models]));

  const categories = useMemo(() => buckets.map(b => bucketLabel(b, hourly)), [buckets, hourly]);
  const modelSeries = slices.map(({name, color}) => ({
    name,
    color,
    kind: "bar" as const,
    stack: "models",
    values: buckets.map(window => {
      const rows = perBucket.get(window.start);
      if (!rows) return null;
      return name === other
        ? rows.reduce((sum, row) => sum + (top.has(row.model) ? 0 : row[metric]), 0)
        : rows.find(row => row.model === name)?.[metric] ?? 0;
    }),
  }));

  const syncing = !analysis.unavailable && analysis.ready_days < analysis.total_days;
  const modelTrend: HeroChartTab = {
    id: "model-trend", label: "模型时间趋势", value: null, categories, format, series: modelSeries,
    onPointClick: (index, name) => {
      const bucket = buckets[index];
      if (!bucket) return;
      if (name && name !== other) onPoint(name, bucket.start);
      else onBucket(bucket.start);
    },
  };

  return (
    <div className="split-grid-2">
      <Panel
        title="模型排行"
        actions={<Segmented label="模型排行口径" options={metrics} value={metric} onChange={setMetric} />}
      >
        <RankedBarList
          emptyTitle={syncing ? "正在同步模型消耗，暂无排行" : "暂无非零模型消耗"}
          items={slices.map(item => ({
            id: item.name,
            label: item.name,
            value: item.value,
            formatted: format(item.value),
            sub: item.requests > 0 ? `${count.format(item.requests)} 次` : undefined,
            color: item.color,
            onClick: item.name !== other ? () => onModel(item.name) : undefined,
          }))}
        />
      </Panel>

      <Panel title="模型占比">
        {slices.some(item => item.value > 0) ? <DonutChart ariaLabel="模型占比" formatShare={formatRate}
          slices={slices.map(item => ({
            label: item.name, value: item.value, color: item.color,
            onSelect: item.name !== other ? () => onModel(item.name) : undefined,
          }))} /> : <EmptyState title="暂无模型占比" />}
      </Panel>
      {analysis.unavailable ? <Panel title="模型时间趋势"><EmptyState title="暂时读不到分模型时段分布" /></Panel> :
        <MetricTrend metric={modelTrend} group={group} emptyTitle={syncing ? "正在同步模型样本" : "当前时间窗口暂无分模型样本"}
          extra={syncing && <span className="muted text-fine">同步中 {analysis.ready_days}/{analysis.total_days} 天</span>} />}
    </div>
  );
});
