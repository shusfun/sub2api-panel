import {useState, type CSSProperties, type FormEvent, type KeyboardEvent} from "react";
import type {z} from "zod";
import {api, named, page, usageRecord, usageSummary} from "./api";
import {count, preciseMoney as dollars, rate as percent} from "./format";
import {useAction, useResource} from "./requests";
import {Button, DataTable, ErrorLine, Field, Feedback, Input, Pagination, Resource, Section, Select, Segmented, Toolbar} from "./ui";
import {RemoteTable} from "./pages/remote-table";
import "./usage.css";

const periods = [["today", "今天"], ["7d", "近 7 天"], ["30d", "近 30 天"], ["custom", "自定义"]] as const;
type Period = (typeof periods)[number][0];
type Summary = z.infer<typeof usageSummary>;
type UsageDay = Summary["days"][number];
type ModelPage = Summary["models"];
const keyPage = page(named);
const compact = new Intl.NumberFormat("zh-CN", {notation: "compact", maximumFractionDigits: 1});
const series = [
  ["input_tokens", "输入"], ["output_tokens", "输出"],
  ["cache_read_tokens", "缓存读取"], ["cache_creation_tokens", "缓存写入"],
] as const;

function UsageKeyPicker({keyID, onChange}: {keyID: string; onChange: (keyID: string) => void}) {
  const first = useResource(() => api("/usage/keys?page=1", keyPage), 1);
  const [morePages, setMorePages] = useState<z.infer<typeof keyPage>[]>([]);
  const action = useAction();
  const options = new Map([...(first?.value?.items ?? []), ...morePages.flatMap(result => result.items)].map(key => [String(key.id), key.name]));
  const more = (morePages.at(-1) ?? first?.value)?.more;
  return <div className="usage-key-picker">
    <Field label="API Key">
      <Select value={keyID} onChange={event => onChange(event.target.value)}>
        <option value="">全部自己的 Key</option>
        {keyID && !options.has(keyID) && <option value={keyID}>Key #{keyID}</option>}
        {[...options].map(([id, name]) => <option key={id} value={id}>{name}</option>)}
      </Select>
    </Field>
    {more && <Button size="sm" variant="ghost" busy={action.busy} onClick={() => void action.run(async () => {
      const result = await api("/usage/keys?page=" + (morePages.length + 2), keyPage);
      setMorePages(previous => [...previous, result]);
    })}>加载更多 Key</Button>}
    <ErrorLine value={action.error || first?.error || ""} />
  </div>;
}

function Stats({totals}: {totals: Summary["totals"]}) {
  const fresh = totals.input_tokens;
  const read = totals.cache_read_tokens;
  const write = totals.cache_creation_tokens;
  const potentialHits = fresh + read + write;
  const metrics = [
    ["实际扣费", dollars.format(totals.actual_cost), ""],
    ["请求数", count.format(totals.requests), ""],
    ["输入／输出 Token", `${compact.format(fresh)} / ${compact.format(totals.output_tokens)}`, `${count.format(fresh)} / ${count.format(totals.output_tokens)}`],
    ["缓存读／写 Token", `${compact.format(read)} / ${compact.format(write)}`, `${count.format(read)} / ${count.format(write)}`],
    ["缓存命中率", potentialHits ? percent.format(read / potentialHits) : "—", ""],
  ];
  return <dl className="usage-stats">
    {metrics.map(([label, value, exact]) => <div key={label}>
      <dt>{label}</dt><dd title={exact || undefined}>{value}</dd>
    </div>)}
  </dl>;
}

function Trend({days}: {days: Summary["days"]}) {
  const [picked, pick] = useState(days.length - 1);
  const selected = days[picked] ?? days[days.length - 1];
  const peak = Math.max(...days.map(day => series.reduce((total, [metric]) => total + day[metric], 0)));
  function navigate(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next = {ArrowLeft: index - 1, ArrowRight: index + 1, Home: 0, End: days.length - 1}[event.key];
    if (next === undefined) return;
    event.preventDefault();
    const buttons = event.currentTarget.parentElement?.querySelectorAll("button");
    buttons?.[Math.min(days.length - 1, Math.max(0, next))]?.focus();
  }
  const describeDay = (day: UsageDay) => [day.date, ...series.map(([metric, label]) => `${label} ${count.format(day[metric])}`)].join("，");
  return <Section className="usage-trend">
    <div className="section-heading"><h2>每日 Token 用量</h2><span className="muted">峰值 {count.format(peak)}</span></div>
    <div className="usage-chart-scroll">
      <div className="usage-bars" role="group" aria-label="按日查看用量，可用左右方向键切换"
        style={{"--usage-day-count": days.length} as CSSProperties}>
        {days.map((day, index) => <button key={day.date} type="button" className="usage-bar" tabIndex={day === selected ? 0 : -1}
          aria-label={describeDay(day)} aria-pressed={day === selected} onFocus={() => pick(index)} onClick={() => pick(index)}
          onKeyDown={event => navigate(event, index)}>
          {series.map(([metric]) => <span key={metric} aria-hidden data-metric={metric} style={{height: `${day[metric] / (peak || 1) * 100}%`}} />)}
        </button>)}
      </div>
    </div>
    <div className="usage-axis muted" aria-hidden><span>{days[0].date}</span>{days.length > 1 && <span>{days[days.length - 1].date}</span>}</div>
    <div className="usage-day" aria-live="polite">
      <time dateTime={selected.date}>{selected.date}</time>
      {series.map(([metric, label]) => <span key={metric}><i aria-hidden data-metric={metric} />{label} <strong>{count.format(selected[metric])}</strong></span>)}
    </div>
  </Section>;
}

export function UsageDashboard({keyID}: {keyID: string}) {
  const [period, setPeriod] = useState<Period>("7d");
  const [dates, setDates] = useState<{start: string; end: string}>();
  const [dateInputs, setDateInputs] = useState<{start: string; end: string}>();
  const [allRecords, setAllRecords] = useState(false);
  const [modelPage, setModelPage] = useState({filter: "", number: 1});
  const query = new URLSearchParams(keyID ? {api_key_id: keyID} : {});
  const keyQuery = new URLSearchParams(query);
  if (period === "custom" && dates) {
    query.set("start_date", dates.start);
    query.set("end_date", dates.end);
  } else {
    query.set("period", period === "custom" ? "7d" : period);
  }
  const filter = query.toString();
  const modelNumber = modelPage.filter === filter ? modelPage.number : 1;
  const summaryPath = "/usage/summary?" + filter + "&page=1";
  const summary = useResource(() => api(summaryPath, usageSummary), summaryPath);
  const modelPath = "/usage/summary?" + filter + "&page=" + modelNumber;
  const modelSummary = useResource(() => api(modelPath, usageSummary), modelPath, modelNumber > 1);
  const modelState: {value?: ModelPage; error?: string} | undefined = modelNumber === 1
    ? summary?.value ? {value: summary.value.models} : summary?.error ? {error: summary.error} : undefined
    : modelSummary?.value ? {value: modelSummary.value.models} : modelSummary?.error ? {error: modelSummary.error} : undefined;
  const data = summary?.value;
  function applyDates(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const next = {start: String(fields.get("start")), end: String(fields.get("end"))};
    setDateInputs(next);
    setDates(next);
  }
  if (data && !allRecords) {
    keyQuery.set("start_date", data.start_date);
    keyQuery.set("end_date", data.end_date);
  }
  const recordPath = "/usage?" + keyQuery;
  const showSelectedRecords = data !== undefined || allRecords;
  return <>
    <Section className="usage-filters">
      <div className="section-heading usage-toolbar">
        <Segmented label="用量日期范围" value={period} options={periods} onChange={next => {
          setPeriod(next);
          if (next === "custom") setDateInputs(dates ?? {start: data?.start_date ?? "", end: data?.end_date ?? ""});
        }} />
        <UsageKeyPicker keyID={keyID} onChange={key => {location.hash = key ? "usage?key=" + encodeURIComponent(key) : "usage";}} />
      </div>
      {period === "custom" && <form className="toolbar" onSubmit={applyDates}>
        <Field label="开始日期"><Input type="date" name="start" value={dateInputs?.start ?? dates?.start ?? data?.start_date ?? ""}
          onChange={event => setDateInputs(previous => ({start: event.target.value, end: previous?.end ?? dates?.end ?? data?.end_date ?? ""}))} required /></Field>
        <Field label="结束日期"><Input type="date" name="end" value={dateInputs?.end ?? dates?.end ?? data?.end_date ?? ""}
          onChange={event => setDateInputs(previous => ({start: previous?.start ?? dates?.start ?? data?.start_date ?? "", end: event.target.value}))} required /></Field>
        <Button type="submit">应用日期</Button>
      </form>}
    </Section>
    <Resource state={summary}>{result => <>
      <Stats key={filter} totals={result.totals} />
      <Trend key={filter} days={result.days} />
      <Section>
        <h2>按模型汇总</h2>
        <Resource state={modelState}>
          {models => <DataTable label="模型用量" columns={[
            {id: "model", title: "模型", render: (row: Summary["models"]["items"][number]) => row.model},
            {id: "requests", title: "请求数", align: "number", render: (row: Summary["models"]["items"][number]) => count.format(row.requests)},
            {id: "tokens", title: "Token 总量", align: "number", render: (row: Summary["models"]["items"][number]) => count.format(row.total_tokens)},
            {id: "cost", title: "实际扣费", align: "number", render: (row: Summary["models"]["items"][number]) => dollars.format(row.actual_cost)},
          ]} rowID={(row: Summary["models"]["items"][number]) => row.model} rows={models.items} />}
        </Resource>
        <Pagination number={modelNumber} more={modelNumber === 1 ? result.models.more : modelSummary?.value?.models.more ?? false}
          onChange={number => setModelPage({filter, number})} label="模型汇总翻页" />
      </Section>
    </>}</Resource>
    <Section>
      <div className="section-heading"><div className="usage-record-heading"><h2>逐笔记录</h2>{data && <span className="muted">{data.timezone}</span>}</div>
        <Segmented label="逐笔记录范围" value={allRecords ? "all" : "filtered"}
          options={[["filtered", "所选日期"], ["all", "全部保留记录"]]} onChange={scope => setAllRecords(scope === "all")} />
      </div>
      {showSelectedRecords && <RemoteTable path={recordPath} schema={usageRecord} label="逐笔用量记录" rowID={row => row.id} columns={[
        {id: "time", title: "时间", render: row => <time dateTime={row.created_at}>{row.created_at.slice(0, 16).replace("T", " ")}</time>},
        {id: "key", title: "API Key", render: row => row.key_name},
        {id: "model", title: "模型", render: row => row.model},
        {id: "input", title: "输入 Token", align: "number", render: row => count.format(row.input_tokens)},
        {id: "output", title: "输出 Token", align: "number", render: row => count.format(row.output_tokens)},
        {id: "cacheRead", title: "缓存读取", align: "number", render: row => row.cache_read_tokens === null ? "—" : count.format(row.cache_read_tokens)},
        {id: "cacheWrite", title: "缓存写入", align: "number", render: row => row.cache_creation_tokens === null ? "—" : count.format(row.cache_creation_tokens)},
        {id: "cost", title: "实际扣费", align: "number", render: row => row.actual_cost === null ? "—" : dollars.format(row.actual_cost)},
      ]} />}
    </Section>
  </>;
}
