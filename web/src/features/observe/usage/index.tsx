import {useCallback, useEffect, useMemo, useState} from "react";
import type {z} from "zod";
import {count, date, durationText, smartMoney} from "@/lib/format";
import {usageRecord, usageSummary} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  AccountSelect, Badge, DateRange, EntityLink, FilterBar, FilterChips, FilterSlot, GroupSelect,
  InsetGroup, InsetRow, Inspector, KeySelect, QuickPills, Segmented, UserSelect, WorkspaceContent,
  WorkspaceStage, type FilterChip,
} from "@/ui";
import {UsageResults} from "./usage-results";

export type UsageData = z.infer<typeof usageSummary>;
export type UsageBucket = UsageData["buckets"][number];
export type ModelUsage = UsageData["ranking"][number];
export type UsageRow = z.infer<typeof usageRecord>;
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

function withinWeek(range: Range) {
  return (Date.parse(range.end) - Date.parse(range.start)) / 86400000 < 7;
}

import {UsageCleanupPanel} from "./usage-cleanup";

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
  const [stream, setStream] = useState<"" | "true" | "false">(() => {
    const s = initial.get("stream");
    return s === "true" || s === "false" ? s : "";
  });
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
  if (stream) chips.push({id: "stream", label: "调用方式", value: stream === "true" ? "流式" : "非流式", onRemove: () => setStream("")});
  if (time) chips.push({id: "time", label: "时间下钻", value: time.slice(5, 16).replace("T", " "), onRemove: () => setTime("")});

  const resetAll = () => {
    if (admin) {
      setUser("all");
      setGroupID("");
      setAccountID("");
    }
    setKey("");
    setModel("");
    setStream("");
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
                  {inspected.user_id && <InsetRow label="调用用户"><EntityLink kind="user" id={inspected.user_id} label={inspected.user_email || `用户 #${inspected.user_id}`} /></InsetRow>}
                  {inspected.group_id && <InsetRow label="调度分组"><EntityLink kind="group" id={inspected.group_id} label={inspected.group_name || `分组 #${inspected.group_id}`} /></InsetRow>}
                  {inspected.account_id && <InsetRow label="上游账号"><EntityLink kind="account" id={inspected.account_id} label={inspected.account_name || `账号 #${inspected.account_id}`} /></InsetRow>}
                  <InsetRow label="API Key">
                    <EntityLink kind="key" id={inspected.api_key_id ?? inspected.key_name} label={inspected.key_name} extra={inspected.user_id ? {user_id: inspected.user_id} : undefined} />
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
          {admin && <FilterSlot><GroupSelect value={groupID} placeholder="全部分组" onChange={next => { setGroupID(next); clearDrill(); }} /></FilterSlot>}
          {admin && <FilterSlot><AccountSelect value={accountID} placeholder="全部上游账号" onChange={next => { setAccountID(next); clearDrill(); }} /></FilterSlot>}
          {(!admin || user !== "all") && <FilterSlot><KeySelect key={keyRoute} path={keyRoute} value={key} onChange={next => { setKey(next); clearDrill(); }} /></FilterSlot>}
          <QuickPills label="日期范围" options={periodOptions.map(([value, label]) => ({value, label}))} value={customOpen ? "custom" : period} onChange={selectPeriod} />
          <Segmented
            label="时间粒度"
            options={grains}
            value={granularity}
            onChange={next => {
              setGranularity(next);
              if (next === "hour" && ((period !== "today" && period !== "7d" && period !== "custom") || (period === "custom" && range && !withinWeek(range)))) {
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
          stream={stream}
          inspectedID={inspected?.id}
          setTime={setTime}
          setModel={setModel}
          setStream={setStream}
          onBucket={changeTime}
          onModel={changeModel}
          onPoint={changePoint}
          onInspect={setInspected}
          onRowsLoaded={setPageRows}
          onFilterGroup={id => setGroupID(String(id))}
          onFilterAccount={id => setAccountID(String(id))}
          onFilterUser={id => { setUser(String(id)); setKey(""); }}
        />
        {admin && <UsageCleanupPanel />}
      </WorkspaceContent>
    </WorkspaceStage>
  );
}
