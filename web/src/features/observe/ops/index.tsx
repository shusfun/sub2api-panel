import {useEffect, useMemo, useState} from "react";
import {Bell} from "lucide-react";
import {z} from "zod";
import {api, streamOps, type UsageStreamStatus} from "@/lib/http";
import {useDebouncedValue, useResource} from "@/lib/requests";
import {
  adminOverview, opsAlertEventSchema, opsConcurrencySchema, opsErrorRowSchema,
  opsLiveSchema, opsOverviewSchema, opsRequestRowSchema,
} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  AccountSelect, Button, FilterBar, FilterSlot, GroupSelect, InlineStack, PlatformSelect,
  QuickPills, RefreshButton, RemoteTable, SearchInput, Segmented, StatusDot, UserSelect,
  WorkspaceContent, WorkspaceStage,
} from "@/ui";
import {OpsAlertsAndSettings} from "./ops-alerts";
import {opsErrorColumns, OpsErrorInspector, opsRequestColumns, type OpsErrorRow} from "./ops-inspector";
import {OpsLiveOverview} from "./ops-live";

type OpsTab = "live" | "errors" | "alerts";
type ErrorSourceTab = "request-errors" | "upstream-errors" | "requests";

const timeRanges = [
  {value: "5m", label: "5m"},
  {value: "30m", label: "30m"},
  {value: "1h", label: "1h"},
  {value: "6h", label: "6h"},
  {value: "24h", label: "24h"},
] as const;

const streamLabel: Record<UsageStreamStatus, string> = {
  connecting: "连接中",
  live: "实时更新",
  reconnecting: "正在重连",
  paused: "后台已暂停",
  denied: "实时流不可用",
};

export function OpsWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [tab, setTab] = useState<OpsTab>(() => {
    const t = initial.get("tab");
    if (t === "errors" || t === "requests" || initial.has("request_id") || initial.has("account_id") || initial.has("user_id")) return "errors";
    if (t === "alerts") return "alerts";
    return "live";
  });
  const [errorSource, setErrorSource] = useState<ErrorSourceTab>(() =>
    initial.get("tab") === "requests" || initial.has("request_id") ? "requests" : "request-errors",
  );
  const [timeRange, setTimeRange] = useState("1h");
  const [platform, setPlatform] = useState("");
  const [groupID, setGroupID] = useState(() => initial.get("group_id") ?? "");
  const [accountID, setAccountID] = useState(() => initial.get("account_id") ?? "");
  const [userID, setUserID] = useState(() => initial.get("user_id") ?? "");
  const [search, setSearch] = useState(() => initial.get("request_id") ?? "");
  const [revision, revise] = useState(0);
  const [live, setLive] = useState<{scope: string; snapshot: z.infer<typeof opsLiveSchema>}>();
  const [streamState, setStreamState] = useState<UsageStreamStatus>("connecting");
  const [inspectedError, setInspectedError] = useState<OpsErrorRow>();
  const [errorRows, setErrorRows] = useState<OpsErrorRow[]>([]);
  const debouncedSearch = useDebouncedValue(search.trim(), 240);

  useEffect(() => {
    const params = new URLSearchParams(queryString);
    if (params.has("request_id")) {
      setTab("errors");
      setErrorSource("requests");
      setSearch(params.get("request_id") ?? "");
    } else if (params.has("account_id")) {
      setTab("errors");
      setAccountID(params.get("account_id") ?? "");
    } else if (params.has("user_id")) {
      setTab("errors");
      setUserID(params.get("user_id") ?? "");
    }
  }, [queryString]);

  const scopeParams = useMemo(() => {
    const p = new URLSearchParams({time_range: timeRange});
    if (platform) p.set("platform", platform);
    if (groupID) p.set("group_id", groupID);
    return p.toString();
  }, [groupID, platform, timeRange]);

  const overview = useResource(() => api("/admin/overview", adminOverview), "admin-overview", true, revision);
  const opsOverview = useResource(() => api("/admin/ops/overview?" + scopeParams, opsOverviewSchema), "ops-overview:" + scopeParams, true, revision);
  const initialConc = useResource(() => api("/admin/ops/concurrency?" + scopeParams, opsConcurrencySchema), "ops-conc:" + scopeParams, true, revision);
  const events = useResource(() => api("/admin/ops/alert-events?status=firing", z.array(opsAlertEventSchema)), "ops-firing-alerts", true, revision);

  useEffect(() => {
    const controller = new AbortController();
    void streamOps("/admin/ops/stream?" + scopeParams, snapshot => setLive({scope: scopeParams, snapshot}), setStreamState, controller.signal);
    return () => controller.abort();
  }, [scopeParams, revision]);

  const snapshot = live?.scope === scopeParams ? live.snapshot : undefined;
  const conc = snapshot ?? (initialConc?.stale ? undefined : initialConc?.value);
  const dash = overview?.value;
  const ops = snapshot?.charts ?? (opsOverview?.stale ? undefined : opsOverview?.value);
  const firingCount = events?.value?.length ?? 0;
  const badAccounts = (dash?.accounts.error ?? 0) + (dash?.accounts.rate_limited ?? 0) + (dash?.accounts.overloaded ?? 0);

  const errorListPath = useMemo(() => {
    const p = new URLSearchParams({time_range: timeRange});
    if (platform) p.set("platform", platform);
    if (groupID) p.set("group_id", groupID);
    if (accountID) p.set("account_id", accountID);
    if (userID) p.set("user_id", userID);
    if (debouncedSearch) {
      if (errorSource === "requests") p.set("request_id", debouncedSearch);
      else p.set("q", debouncedSearch);
    }
    return `/admin/ops/${errorSource}?${p.toString()}`;
  }, [accountID, debouncedSearch, errorSource, groupID, platform, timeRange, userID]);

  const errIdx = inspectedError ? errorRows.findIndex(r => r.id === inspectedError.id) : -1;
  const inspectorOpen = tab === "errors" && !!inspectedError;

  return (
    <WorkspaceStage
      flush
      inspectorOpen={inspectorOpen}
      inspector={
        <OpsErrorInspector
          open={inspectorOpen}
          errorRow={inspectedError}
          errorSource={errorSource}
          onClose={() => setInspectedError(undefined)}
          onPrev={errIdx > 0 ? () => setInspectedError(errorRows[errIdx - 1]) : undefined}
          onNext={errIdx >= 0 && errIdx < errorRows.length - 1 ? () => setInspectedError(errorRows[errIdx + 1]) : undefined}
          onUpdated={next => { setInspectedError(next); revise(v => v + 1); }}
          onRefresh={() => revise(v => v + 1)}
        />
      }
    >
      <FilterBar sticky>
        <div className="obs-filter-row">
          <Segmented
            label="监控视图切换"
            value={tab}
            options={[["live", "负载与总览"], ["errors", "报错与追踪"], ["alerts", "告警与策略"]] as const}
            onChange={next => { setTab(next); setInspectedError(undefined); }}
          />
          {tab === "errors" && (
            <Segmented
              label="日志数据源"
              value={errorSource}
              options={[["request-errors", "客户端报错"], ["upstream-errors", "上游报错"], ["requests", "全量请求"]] as const}
              onChange={next => { setErrorSource(next); setInspectedError(undefined); }}
            />
          )}
          {tab !== "alerts" && (
            <>
              <QuickPills label="时间窗" value={timeRange} options={timeRanges} onChange={setTimeRange} />
              <FilterSlot><PlatformSelect value={platform} onChange={setPlatform} /></FilterSlot>
              <FilterSlot><GroupSelect value={groupID} onChange={setGroupID} /></FilterSlot>
            </>
          )}
          {tab === "errors" && (
            <>
              <SearchInput value={search} onChange={setSearch} placeholder="搜请求编号或模型…" ariaLabel="搜索请求编号或模型" />
              <FilterSlot><AccountSelect value={accountID} onChange={setAccountID} /></FilterSlot>
              <FilterSlot><UserSelect value={userID} onChange={v => setUserID(v)} /></FilterSlot>
            </>
          )}
          <div className="obs-filter-spacer">
            <InlineStack density="compact" className="text-fine muted" role="status">
              <StatusDot tone={streamState === "live" ? "ok" : streamState === "denied" ? "bad" : "warn"} />
              {streamLabel[streamState]}
            </InlineStack>
            {tab !== "live" && badAccounts > 0 && (
              <Button variant="outline" onClick={() => navigateTo("accounts", {status: "error"})}>异常账号 {badAccounts}</Button>
            )}
            {tab !== "live" && firingCount > 0 && (
              <Button variant="outline" onClick={() => setTab("alerts")}><Bell aria-hidden />活跃告警 {firingCount}</Button>
            )}
            <RefreshButton iconOnly size="default" variant="ghost" label="刷新监控" onClick={() => revise(v => v + 1)} />
          </div>
        </div>
      </FilterBar>

      {tab === "live" && (
        <WorkspaceContent>
          <OpsLiveOverview
            conc={conc}
            dash={dash}
            ops={ops}
            firingCount={firingCount}
            badAccounts={badAccounts}
            onOpenTab={setTab}
          />
        </WorkspaceContent>
      )}

      {tab === "errors" && (
        <div className="swift-pane-table">
          {errorSource === "requests" ? (
            <RemoteTable
              key={errorListPath}
              path={errorListPath}
              schema={opsRequestRowSchema}
              columns={opsRequestColumns}
              rowID={row => row.request_id || `${row.created_at}-${row.model}`}
              label="请求追踪"
              revision={revision}
            />
          ) : (
            <RemoteTable
              key={errorListPath}
              path={errorListPath}
              schema={opsErrorRowSchema}
              columns={opsErrorColumns}
              rowID={row => row.id}
              label="异常记录"
              revision={revision}
              activeRowID={inspectedError?.id}
              onRowClick={setInspectedError}
              onItemsLoaded={setErrorRows}
            />
          )}
        </div>
      )}

      {tab === "alerts" && (
        <WorkspaceContent>
          <OpsAlertsAndSettings revision={revision} onChanged={() => revise(v => v + 1)} />
        </WorkspaceContent>
      )}
    </WorkspaceStage>
  );
}
