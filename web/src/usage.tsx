import { useCallback,useEffect,useId,useMemo,useState } from "react";
import type { z } from "zod";
import { RefreshCw } from "lucide-react";
import { api,streamUsage,usageRecord,usageSummary,type UsageStreamStatus } from "./api";
import { compact,count,date,durationText,smartMoney } from "./format";
import { useResource } from "./requests";
import { navigateTo } from "./route";
import {
WorkspaceContent,InlineStack,
AccountSelect,
Badge,
Button,
ChartSkeleton,
DateRange,
EntityLink,
Feedback,
FilterBar,
FilterSlot,
FilterChips,
GroupSelect,
InsetGroup,
InsetRow,
Inspector,
KeySelect,
Panel,
QuickPills,
Resource,
SearchSelect,
ModelSelect,
modelOptions,
Segmented,
StatusDot,
UserSelect,
WorkspaceStage,
type Column,
type FilterChip,
} from "./ui";
import { RemoteTable } from "./pages/remote-table";
import { bucketLabel,UsageHeroTrends,UsageModels } from "./usage-charts";

export type UsageData = z.infer<typeof usageSummary>;
export type UsageBucket = UsageData["buckets"][number];
export type ModelUsage = UsageData["ranking"][number];
type UsageRow = z.infer<typeof usageRecord>;
type Range = {start: string; end: string};
type Granularity = UsageData["granularity"];

const grains = [["hour", "按小时"], ["day", "按天"]] as const;
const periodOptions = [
  ["today", "今天"],
  ["7d", "7 天"],
  ["14d", "14 天"],
  ["30d", "30 天"],
  ["90d", "90 天"],
  ["custom", "自定义"],
] as const;
type Period = (typeof periodOptions)[number][0];

const liveStatus: Record<UsageStreamStatus, string> = {
  connecting: "连接中",
  live: "实时同步",
  reconnecting: "正在重连",
  paused: "后台已暂停",
  denied: "实时流不可用",
};

function withinWeek(range: Range) {
  return (Date.parse(range.end) - Date.parse(range.start)) / 86400000 < 7;
}

export function UsageDashboard({keyID, queryString = "", admin = false}: {keyID: string; queryString?: string; admin?: boolean}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const initialUser = initial.get("user") ?? "";
  const [user, setUser] = useState(() => {
    if (!admin) return "self";
    if (initialUser === "self" || /^[1-9]\d*$/.test(initialUser)) return initialUser;
    return "all";
  });
  const [key, setKey] = useState(() => initial.get("key") ?? keyID);
  const [groupID, setGroupID] = useState(() => initial.get("group") ?? "");
  const [accountID, setAccountID] = useState(() => initial.get("account") ?? "");
  const [period, setPeriod] = useState<Period>(() => {
    const p = initial.get("period");
    return p === "today" || p === "7d" || p === "14d" || p === "30d" || p === "90d" ? p : "7d";
  });
  const [customOpen, setCustomOpen] = useState(false);
  const [granularity, setGranularity] = useState<Granularity>(() => initial.get("granularity") === "hour" ? "hour" : "day");
  const [range, setRange] = useState<Range>();
  const [draft, setDraft] = useState<Range>({start: "", end: ""});
  const [time, setTime] = useState("");
  const [model, setModel] = useState(() => initial.get("model") ?? "");
  const [inspected, setInspected] = useState<UsageRow>();
  const [pageRows, setPageRows] = useState<UsageRow[]>([]);

  useEffect(() => {
    const params = new URLSearchParams(queryString);
    if (params.has("key")) setKey(params.get("key") ?? "");
    if (params.has("model")) setModel(params.get("model") ?? "");
    if (admin && params.has("user")) {
      const u = params.get("user") ?? "all";
      setUser(u === "self" || /^[1-9]\d*$/.test(u) ? u : "all");
    }
    if (admin && params.has("group")) setGroupID(params.get("group") ?? "");
    if (admin && params.has("account")) setAccountID(params.get("account") ?? "");
  }, [admin, queryString]);

  const root = admin ? "/admin/usage" : "/usage";
  const criteria = new URLSearchParams({granularity, analysis: "models"});
  if (period === "custom" && range) {
    criteria.set("start_date", range.start);
    criteria.set("end_date", range.end);
  } else {
    criteria.set("period", period === "custom" ? "7d" : period);
  }
  if (admin) {
    if (user === "all") criteria.set("user_id", "all");
    else if (user !== "self") criteria.set("user_id", user);
    if (groupID) criteria.set("group_id", groupID);
    if (accountID) criteria.set("account_id", accountID);
  }
  const keyRoute = root + "/keys" + (admin ? (user === "all" ? "?user_id=all" : user !== "self" ? "?user_id=" + user : "") : "");
  if (key) criteria.set("api_key_id", key);
  const filter = criteria.toString();

  const clearDrill = () => {
    setTime("");
    setModel("");
    setInspected(undefined);
  };
  const changeTime = useCallback((next: string) => setTime(prev => prev === next ? "" : next), []);
  const changeModel = useCallback((next: string) => setModel(prev => prev === next ? "" : next), []);
  const changePoint = useCallback((nextModel: string, nextTime: string) => {
    setTime(nextTime);
    setModel(nextModel);
  }, []);

  const selectPeriod = (next: Period) => {
    setCustomOpen(next === "custom");
    if (next === "custom") return;
    setPeriod(next);
    if (next === "today") setGranularity("hour");
    else if (next !== "7d") setGranularity("day");
    clearDrill();
  };

  const chips: FilterChip[] = [];
  if (admin && user !== "all") chips.push({id: "user", label: "用户范围", value: user === "self" ? "个人用量" : `#${user}`, onRemove: () => { setUser("all"); setKey(""); }});
  if (admin && groupID) chips.push({id: "group", label: "分组", value: `#${groupID}`, onRemove: () => setGroupID("")});
  if (admin && accountID) chips.push({id: "account", label: "上游账号", value: `#${accountID}`, onRemove: () => setAccountID("")});
  if (key) chips.push({id: "key", label: "API Key", value: `#${key}`, onRemove: () => setKey("")});
  if (model) chips.push({id: "model", label: "模型", value: model, onRemove: () => setModel("")});
  if (time) chips.push({id: "time", label: "时间下钻", value: time.slice(5, 16).replace("T", " "), onRemove: () => setTime("")});

  const resetAll = () => {
    if (admin) {
      setUser("all");
      setGroupID("");
      setAccountID("");
    }
    setKey("");
    setModel("");
    setTime("");
    navigateTo("usage", {}, true);
  };

  const idx = inspected ? pageRows.findIndex(r => r.id === inspected.id) : -1;

  return (
    <WorkspaceStage
      flush
      inspectorOpen={!!inspected}
      inspector={
        <Inspector
          open={!!inspected}
          title={inspected?.model ?? "调用详情"}
          subtitle={inspected ? date(inspected.created_at) : undefined}
          badge={inspected?.stream ? <Badge variant="soft" tone="info">流式</Badge> : undefined}
          onClose={() => setInspected(undefined)}
          onPrev={idx > 0 ? () => setInspected(pageRows[idx - 1]) : undefined}
          onNext={idx >= 0 && idx < pageRows.length - 1 ? () => setInspected(pageRows[idx + 1]) : undefined}
        >
          {inspected && (
            <>
              <InsetGroup title="计费与缓存拆解">
                <InsetRow label="实际扣费"><strong className="weir-digits">{inspected.actual_cost === null ? "未知" : smartMoney(inspected.actual_cost)}</strong></InsetRow>
                <InsetRow label="输入 Token"><span className="weir-digits">{count.format(inspected.input_tokens)}</span></InsetRow>
                <InsetRow label="输出 Token"><span className="weir-digits">{count.format(inspected.output_tokens)}</span></InsetRow>
                <InsetRow label="缓存读取"><span className="weir-digits">{inspected.cache_read_tokens === null ? "未知" : count.format(inspected.cache_read_tokens)}</span></InsetRow>
                <InsetRow label="缓存创建"><span className="weir-digits">{inspected.cache_creation_tokens === null ? "未知" : count.format(inspected.cache_creation_tokens)}</span></InsetRow>
              </InsetGroup>
              {admin && (
                <InsetGroup title="链路耗时与关联实体">
                  <InsetRow label="首字响应 (TTFT)"><span className="weir-digits">{durationText(inspected.first_token_ms)}</span></InsetRow>
                  <InsetRow label="总响应耗时"><span className="weir-digits">{durationText(inspected.duration_ms)}</span></InsetRow>
                  {inspected.user_id && (
                    <InsetRow label="调用用户">
                      <EntityLink kind="user" id={inspected.user_id} label={inspected.user_email || `用户 #${inspected.user_id}`} />
                    </InsetRow>
                  )}
                  {inspected.group_id && (
                    <InsetRow label="调度分组">
                      <EntityLink kind="group" id={inspected.group_id} label={inspected.group_name || `分组 #${inspected.group_id}`} />
                    </InsetRow>
                  )}
                  {inspected.account_id && (
                    <InsetRow label="上游账号">
                      <EntityLink kind="account" id={inspected.account_id} label={inspected.account_name || `账号 #${inspected.account_id}`} />
                    </InsetRow>
                  )}
                  <InsetRow label="API Key">
                    <EntityLink
                      kind="key"
                      id={inspected.api_key_id ?? inspected.key_name}
                      label={inspected.key_name}
                      extra={inspected.user_id ? {user_id: inspected.user_id} : undefined}
                    />
                  </InsetRow>
                </InsetGroup>
              )}
            </>
          )}
        </Inspector>
      }
    >
      <FilterBar sticky>
        <div className="obs-filter-row">
          {admin && (
            <FilterSlot>
              <UserSelect
                value={user}
                clearable={false}
                placeholder="全站汇总"
                extraOptions={[
                  {value: "all", label: "全站汇总（所有用户）"},
                  {value: "self", label: "我的个人调用"},
                ]}
                onChange={next => {
                  if (next === user) return;
                  setUser(next || "all");
                  setKey("");
                  clearDrill();
                }}
              />
            </FilterSlot>
          )}
          {admin && (
            <FilterSlot><GroupSelect value={groupID} placeholder="全部分组" onChange={next => { setGroupID(next); clearDrill(); }} /></FilterSlot>
          )}
          {admin && (
            <FilterSlot>
              <AccountSelect value={accountID} placeholder="全部上游账号" onChange={next => { setAccountID(next); clearDrill(); }} />
            </FilterSlot>
          )}
          {(!admin || user !== "all") && (
            <FilterSlot>
              <KeySelect key={keyRoute} path={keyRoute} value={key} onChange={next => { setKey(next); clearDrill(); }} />
            </FilterSlot>
          )}
          <QuickPills
            label="日期范围"
            options={periodOptions.map(([value, label]) => ({value, label}))}
            value={customOpen ? "custom" : period}
            onChange={selectPeriod}
          />
          <Segmented
            label="时间粒度"
            options={grains}
            value={granularity}
            onChange={next => {
              setGranularity(next);
              if (next === "hour" && (period !== "today" && period !== "7d" && period !== "custom" || period === "custom" && range && !withinWeek(range))) {
                setPeriod("today");
              }
              clearDrill();
            }}
          />
        </div>
        {customOpen && (
          <DateRange
            {...draft}
            onChange={setDraft}
            onApply={() => {
              if (granularity === "hour" && !withinWeek(draft)) setGranularity("day");
              setPeriod("custom");
              setRange(draft);
              clearDrill();
            }}
          />
        )}
        <FilterChips chips={chips} onReset={resetAll} />
      </FilterBar>

      <WorkspaceContent>
        <UsageResults
          key={root + filter}
          root={root}
          filter={filter}
          admin={admin}
          time={time}
          model={model}
          inspectedID={inspected?.id}
          setTime={setTime}
          setModel={setModel}
          onBucket={changeTime}
          onModel={changeModel}
          onPoint={changePoint}
          onInspect={setInspected}
          onRowsLoaded={setPageRows}
          onFilterGroup={id => setGroupID(String(id))}
          onFilterAccount={id => setAccountID(String(id))}
          onFilterUser={id => { setUser(String(id)); setKey(""); }}
        />
      </WorkspaceContent>
    </WorkspaceStage>
  );
}

function UsageResults({
  root,
  filter,
  admin,
  time,
  model,
  inspectedID,
  setTime,
  setModel,
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
  inspectedID?: number;
  setTime: (value: string) => void;
  setModel: (value: string) => void;
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
              <EntityLink
                kind="user"
                id={row.user_id}
                label={row.user_email || `#${row.user_id}`}
                onClick={() => onFilterUser(row.user_id!)}
              />
            )}
            {row.group_id && (
              <EntityLink
                kind="group"
                id={row.group_id}
                label={row.group_name || `分组 #${row.group_id}`}
                onClick={() => onFilterGroup(row.group_id!)}
              />
            )}
            {row.account_id && (
              <EntityLink
                kind="account"
                id={row.account_id}
                label={row.account_name || `账号 #${row.account_id}`}
                onClick={() => onFilterAccount(row.account_id!)}
              />
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
      cols.push({
        id: "latency",
        title: "耗时",
        align: "number",
        render: row => durationText(row.duration_ms),
      });
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
              <UsageHeroTrends
                totals={value.totals}
                buckets={value.buckets}
                hourly={value.granularity === "hour"}
                group={group}
                onBucket={onBucket}
              />

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
                  onItemsLoaded={onRowsLoaded}
                />
              </Panel>
            </>
          )}
        </Resource>
      )}
    </>
  );
}
