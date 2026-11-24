import {useEffect, useId, useMemo, useState} from "react";
import {Download, RefreshCw} from "lucide-react";
import {api, streamUsage, type UsageStreamStatus} from "@/lib/http";
import {compact, count, date, durationText, smartMoney} from "@/lib/format";
import {useResource} from "@/lib/requests";
import {usageRecord, usageSummary} from "@/schemas";
import {
  Button, ChartSkeleton, EntityLink, Feedback, FilterSlot, InlineStack, ModelSelect,
  Panel, RemoteTable, Resource, SearchSelect, Segmented, StatusDot, modelOptions, type Column,
} from "@/ui";
import {bucketLabel, UsageHeroTrends, UsageModels} from "./usage-charts";
import type {UsageData, UsageRow} from "./index";

const streamFilterOptions = [
  ["", "全部"],
  ["true", "流式"],
  ["false", "非流式"],
] as const;

function escapeCSV(value: unknown) {
  const s = value == null ? "" : String(value);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

function exportUsageCSV(rows: UsageRow[], admin: boolean) {
  const headers = admin
    ? ["时间", "模型", "流式", "用户", "分组", "上游账号", "API Key", "输入Token", "输出Token", "缓存读取", "缓存创建", "实际扣费", "耗时(ms)"]
    : ["时间", "模型", "流式", "API Key", "输入Token", "输出Token", "缓存读取", "缓存创建", "实际扣费"];
  const lines = [headers.map(escapeCSV).join(",")];
  for (const r of rows) {
    const base = [
      r.created_at,
      r.model,
      r.stream === true ? "流式" : r.stream === false ? "非流式" : "",
    ];
    if (admin) {
      base.push(r.user_email || (r.user_id ? `#${r.user_id}` : ""), r.group_name || (r.group_id ? `#${r.group_id}` : ""), r.account_name || (r.account_id ? `#${r.account_id}` : ""));
    }
    base.push(r.key_name, String(r.input_tokens), String(r.output_tokens), String(r.cache_read_tokens ?? ""), String(r.cache_creation_tokens ?? ""), String(r.actual_cost ?? ""));
    if (admin) base.push(String(r.duration_ms ?? ""));
    lines.push(base.map(escapeCSV).join(","));
  }
  const blob = new Blob(["\ufeff" + lines.join("\n")], {type: "text/csv;charset=utf-8"});
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `usage-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

const liveStatus: Record<UsageStreamStatus, string> = {
  connecting: "连接中",
  live: "实时同步",
  reconnecting: "正在重连",
  paused: "后台已暂停",
  denied: "实时流不可用",
};

export function UsageResults({
  root,
  filter,
  admin,
  time,
  model,
  stream,
  inspectedID,
  setTime,
  setModel,
  setStream,
  onBucket,
  onModel,
  onPoint,
  onInspect,
  onRowsLoaded,
  onFilterGroup,
  onFilterAccount,
  onFilterUser,
}: {
  root: string;
  filter: string;
  admin: boolean;
  time: string;
  model: string;
  stream: "" | "true" | "false";
  inspectedID?: number;
  setTime: (value: string) => void;
  setModel: (value: string) => void;
  setStream: (value: "" | "true" | "false") => void;
  onBucket: (value: string) => void;
  onModel: (value: string) => void;
  onPoint: (model: string, bucket: string) => void;
  onInspect: (row: UsageRow) => void;
  onRowsLoaded: (rows: UsageRow[]) => void;
  onFilterGroup: (id: number) => void;
  onFilterAccount: (id: number) => void;
  onFilterUser: (id: number) => void;
}) {
  const group = useId();
  const [snapshot, setSnapshot] = useState<UsageData>();
  const [status, setStatus] = useState<UsageStreamStatus>("connecting");
  const [updated, setUpdated] = useState<number>();
  const [revision, refresh] = useState(0);
  const [recordRevision, refreshRecords] = useState(0);
  const [currentRows, setCurrentRows] = useState<UsageRow[]>([]);
  const summaryPath = root + "/summary?" + filter;
  const summary = useResource(() => api(summaryPath, usageSummary), summaryPath, true, revision);

  useEffect(() => {
    const controller = new AbortController();
    void streamUsage(
      root + "/stream?" + filter,
      (value, _, requestID) => {
        if (controller.signal.aborted) return;
        setSnapshot(value);
        setUpdated(Date.now());
        refreshRecords(version => version + 1);
        if (requestID) {
          performance.clearMarks("panel-usage-received");
          performance.mark("panel-usage-received", {detail: {requestID}});
        }
      },
      next => { if (!controller.signal.aborted) setStatus(next); },
      controller.signal,
    );
    return () => controller.abort();
  }, [root, filter, revision]);

  const data = status === "denied" ? undefined : snapshot ?? summary?.value;
  const selected = data?.buckets.find(bucket => bucket.start === time);
  const recordQuery = new URLSearchParams(filter);
  if (selected) recordQuery.set("bucket_start", selected.start);
  if (model) recordQuery.set("model", model);
  if (stream) recordQuery.set("stream", stream);
  const recordPath = root + "?" + recordQuery.toString();

  const columns = useMemo<Column<UsageRow>[]>(() => {
    const cols: Column<UsageRow>[] = [
      {id: "time", title: "时间", render: row => <span className="code-mono">{date(row.created_at)}</span>},
      {
        id: "model",
        title: "模型",
        render: row => <EntityLink kind="model" id={row.model} label={row.model} onClick={() => onModel(row.model)} />,
      },
    ];
    if (admin) {
      cols.push({
        id: "entities",
        title: "用户 / 分组 / 账号",
        render: row => (
          <InlineStack density="compact" wrap>
            {row.user_id && (
              <EntityLink kind="user" id={row.user_id} label={row.user_email || `#${row.user_id}`} onClick={() => onFilterUser(row.user_id!)} />
            )}
            {row.group_id && (
              <EntityLink kind="group" id={row.group_id} label={row.group_name || `分组 #${row.group_id}`} onClick={() => onFilterGroup(row.group_id!)} />
            )}
            {row.account_id && (
              <EntityLink kind="account" id={row.account_id} label={row.account_name || `账号 #${row.account_id}`} onClick={() => onFilterAccount(row.account_id!)} />
            )}
          </InlineStack>
        ),
      });
    }
    cols.push(
      {id: "key", title: "API Key", render: row => <span>{row.key_name}</span>},
      {id: "input", title: "输入", align: "number", render: row => count.format(row.input_tokens)},
      {id: "output", title: "输出", align: "number", render: row => count.format(row.output_tokens)},
      {
        id: "cache",
        title: "缓存 读/写",
        align: "number",
        render: row => `${row.cache_read_tokens === null ? "—" : compact.format(row.cache_read_tokens)} / ${row.cache_creation_tokens === null ? "—" : compact.format(row.cache_creation_tokens)}`,
      },
      {id: "cost", title: "实际扣费", align: "number", render: row => row.actual_cost === null ? "未知" : smartMoney(row.actual_cost)},
    );
    if (admin) {
      cols.push({id: "latency", title: "耗时", align: "number", render: row => durationText(row.duration_ms)});
    }
    return cols;
  }, [admin, onFilterAccount, onFilterGroup, onFilterUser, onModel]);

  return (
    <>
      <div className="usage-section-heading">
        <div className="usage-live" role="status">
          <StatusDot tone={status === "live" ? "ok" : status === "denied" ? "bad" : "warn"} />
          <span>{liveStatus[status]}{updated !== undefined && status !== "denied" && " · " + new Date(updated).toLocaleTimeString("zh-CN")}</span>
          {data && <span className="muted">· {data.timezone}</span>}
        </div>
        <Button variant="ghost" size="icon-sm" title="立即同步" aria-label="刷新用量" onClick={() => refresh(v => v + 1)}><RefreshCw /></Button>
      </div>

      {status === "denied" ? (
        <Feedback tone="error">无法读取当前筛选范围的用量。</Feedback>
      ) : (
        <Resource state={data ? {value: data} : summary} fallback={<ChartSkeleton />}>
          {value => (
            <>
              <UsageHeroTrends totals={value.totals} buckets={value.buckets} hourly={value.granularity === "hour"} group={group} onBucket={onBucket} />
              <UsageModels
                ranking={value.ranking}
                analysis={value.model_series}
                buckets={value.buckets}
                hourly={value.granularity === "hour"}
                group={group}
                onModel={onModel}
                onBucket={onBucket}
                onPoint={onPoint}
              />
              <Panel
                title="逐笔计费明细"
                padded={false}
                actions={
                  <InlineStack wrap>
                    <Segmented
                      label="流式筛选"
                      value={stream}
                      options={streamFilterOptions}
                      onChange={setStream}
                    />
                    <FilterSlot>
                      <SearchSelect
                        label="时间桶"
                        value={selected?.start ?? ""}
                        placeholder="全部时间段"
                        options={value.buckets.map(bucket => ({value: bucket.start, label: bucketLabel(bucket, value.granularity === "hour", true)}))}
                        onChange={setTime}
                      />
                    </FilterSlot>
                    <FilterSlot>
                      <ModelSelect
                        label="模型"
                        value={model}
                        placeholder="全部模型"
                        catalog={{options: modelOptions(value.ranking.map(row => row.model))}}
                        onChange={setModel}
                      />
                    </FilterSlot>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={currentRows.length === 0}
                      onClick={() => exportUsageCSV(currentRows, admin)}
                    >
                      <Download aria-hidden />导出 CSV
                    </Button>
                  </InlineStack>
                }
              >
                <RemoteTable
                  key={recordPath}
                  path={recordPath}
                  schema={usageRecord}
                  columns={columns}
                  rowID={row => row.id}
                  label="逐笔用量"
                  revision={recordRevision + revision}
                  activeRowID={inspectedID}
                  onRowClick={onInspect}
                  onItemsLoaded={rows => { setCurrentRows(rows); onRowsLoaded(rows); }}
                />
              </Panel>
            </>
          )}
        </Resource>
      )}
    </>
  );
}
