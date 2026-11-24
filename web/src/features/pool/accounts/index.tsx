import {useEffect, useMemo, useRef, useState} from "react";
import {Plus} from "lucide-react";
import {z} from "zod";
import {api, describeError, testAccount, type UsageStreamStatus} from "@/lib/http";
import {account, adminOverview, empty, named} from "@/schemas";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {navigateTo} from "@/app/route";
import {
  Button,
  ConfirmDialog,
  DataTable,
  EmptyState,
  EntityGrid,
  Inspector,
  PLATFORM_OPTIONS,
  RemoteTable,
  WorkspaceStage,
  toast,
  type EntityCardResult,
  type ViewMode,
} from "@/ui";
import {ReorderMotion} from "@/ui/primitives/activity-motion";
import {groupBriefSchema, type AccountRow} from "@/features/pool/shared";
import {AddAccountDialog} from "./account-add-dialog";
import {AccountBatch} from "./account-batch";
import {AccountCardItem} from "./account-card";
import {buildAccountColumns} from "./account-columns";
import {AccountWorkspaceHeader, createAccountRowControls} from "./account-controls";
import {AccountCRS} from "./account-crs";
import {AccountData} from "./account-data";
import {
  accountProblem,
  canRefreshBalance,
  isQuotaCapable,
  quotaDisplayKey,
  sortAccounts,
  sortOptions,
  useAccountTelemetry,
  viewKey,
  type AccountSort,
  type CardMode,
  type QuotaDisplay,
  type UsageWindow,
} from "./account-helpers";
import {AccountModelsDialog, AccountSummary, BulkBindAccountsDialog} from "./account-overlays";

export {AccountDetailWorkspace} from "./account-detail";

export function AccountsWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [groupFilter, setGroupFilter] = useState(() => initial.get("group") ?? "");
  const [proxyFilter, setProxyFilter] = useState(() => initial.get("proxy") ?? "");
  const [platform, setPlatform] = useState(() => initial.get("platform") ?? "");
  const [status, setStatus] = useState(() => initial.get("status") ?? "");
  const [typeFilter, setTypeFilter] = useState(() => initial.get("type") ?? "");
  const [search, setSearch] = useState(() => initial.get("search") ?? "");
  const [sort, setSort] = useState<AccountSort>(() => sortOptions.find(option => option.value === initial.get("sort"))?.value ?? "priority");
  const [rowInteraction, setRowInteraction] = useState(false);
  const [rowMenuOpen, setRowMenuOpen] = useState(false);
  const [liveStatus, setLiveStatus] = useState<UsageStreamStatus>("connecting");
  const [view, setView] = useState<ViewMode>(() => (localStorage.getItem(viewKey) === "list" ? "list" : "card"));
  const [quotaDisplay, setQuotaDisplay] = useState<QuotaDisplay>(() =>
    localStorage.getItem(quotaDisplayKey) === "remaining" ? "remaining" : "used",
  );
  const [defaultWindow, setDefaultWindow] = useState<UsageWindow>("1");
  const [cardModes, setCardModes] = useState<Record<number, CardMode>>({});
  const [cardWindows, setCardWindows] = useState<Record<number, UsageWindow>>({});

  const [revision, revise] = useState(0);
  const [selected, setSelected] = useState<number[]>([]);
  const [rows, setRows] = useState<AccountRow[]>([]);
  const {quotaMap, balanceMap, statsCache, loadingStats, clearStats, ensureWindowStats, refreshRowTelemetry} = useAccountTelemetry({
    rows,
    revision,
    view,
    defaultWindow,
    cardModes,
    cardWindows,
  });
  const [platformCounts, setPlatformCounts] = useState<Record<string, number>>({});
  const [inspect, setInspect] = useState<number | undefined>(() => {
    const id = Number(initial.get("inspect") ?? 0);
    return id > 0 ? id : undefined;
  });
  const [modelsAccount, setModelsAccount] = useState<AccountRow>();
  const [results, setResults] = useState<Record<number, EntityCardResult>>({});
  const [busyID, setBusyID] = useState<number>();
  const [addOpen, setAddOpen] = useState(false);
  const [bulkBindOpen, setBulkBindOpen] = useState(false);
  const [crsOpen, setCRSOpen] = useState(false);
  const [accountData, setAccountData] = useState<"import" | "export">();
  const [batchTargets, setBatchTargets] = useState<z.infer<typeof named>[]>();
  const [deleting, setDeleting] = useState<AccountRow | "batch">();
  const deletionTrigger = useRef<HTMLElement>(null);
  const addTrigger = useRef<HTMLButtonElement>(null);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => {
    clearStats();
    revise(v => v + 1);
  };
  const batchAction = useAction();
  const removeAction = useAction();

  useEffect(() => {
    const params = new URLSearchParams(queryString);
    setGroupFilter(params.get("group") ?? "");
    setProxyFilter(params.get("proxy") ?? "");
    setStatus(params.get("status") ?? "");
    setPlatform(params.get("platform") ?? "");
    setTypeFilter(params.get("type") ?? "");
    setSearch(params.get("search") ?? "");
    setSort(sortOptions.find(option => option.value === params.get("sort"))?.value ?? "priority");
    const id = Number(params.get("inspect") ?? 0);
    setInspect(id > 0 ? id : undefined);
  }, [queryString]);

  const groups = useResource(() => api("/admin/groups/all", z.array(groupBriefSchema)), "account-groups", true, revision);
  const overview = useResource(() => api("/admin/overview", adminOverview), "admin-overview", true, revision);
  const totals = overview?.value?.accounts;

  const query = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (platform) p.set("platform", platform);
    if (status) p.set("status", status);
    if (typeFilter) p.set("type", typeFilter);
    if (groupFilter) p.set("group", groupFilter);
    if (proxyFilter) p.set("proxy_id", proxyFilter);
    if (sort === "activity") p.set("sort_by", "activity");
    const qs = p.toString();
    return "/admin/accounts" + (qs ? "?" + qs : "");
  }, [debouncedSearch, groupFilter, platform, proxyFilter, status, typeFilter, sort]);

  const unfiltered = !debouncedSearch && !platform && !status && !typeFilter && !groupFilter && !proxyFilter;
  const filtered = !unfiltered;
  function updateAccountQuery(update: Record<string, string>) {
    navigateTo("accounts", {...Object.fromEntries(new URLSearchParams(location.search)), ...update}, true);
  }
  function openAccount(id: number) {
    const params = Object.fromEntries(new URLSearchParams(location.search));
    delete params.inspect;
    navigateTo("accounts", {...params, detail: id});
  }
  useEffect(() => {
    if (search.trim() !== debouncedSearch) return;
    const urlSearch = new URLSearchParams(location.search).get("search") ?? "";
    if (urlSearch !== debouncedSearch) {
      navigateTo("accounts", {...Object.fromEntries(new URLSearchParams(location.search)), search: debouncedSearch}, true);
    }
  }, [debouncedSearch, search]);
  const resetFilters = () => {
    updateAccountQuery({group: "", proxy: "", platform: "", status: "", type: "", search: ""});
  };

  useEffect(() => {
    if (!unfiltered || rows.length === 0) return;
    const next: Record<string, number> = {};
    for (const r of rows) next[r.platform] = (next[r.platform] ?? 0) + 1;
    setPlatformCounts(next);
  }, [rows, unfiltered]);

  // 单个账号的操作：结果写回它自己的卡片或行，不占用页面级提示。
  async function act(row: AccountRow, done: string, work: () => Promise<unknown>) {
    if (busyID !== undefined) return;
    setBusyID(row.id);
    setResults(prev => {
      const next = {...prev};
      delete next[row.id];
      return next;
    });
    try {
      await work();
      setResults(prev => ({...prev, [row.id]: {tone: "ok", text: done}}));
      refresh();
    } catch (failure) {
      setResults(prev => ({...prev, [row.id]: {tone: "bad", text: describeError(failure)}}));
    } finally {
      setBusyID(undefined);
    }
  }

  const post = (row: AccountRow, action: string) => api(`/admin/accounts/${row.id}/${action}`, account, "POST", {});
  const probe = (row: AccountRow) =>
    act(row, "测活通过", async () => {
      let last = "";
      await testAccount(row.id, "", state => {
        last = state;
      });
      if (last !== "passed") throw new Error("测活失败");
    });

  const refreshCardTelemetry = (row: AccountRow) =>
    act(
      row,
      canRefreshBalance(row) ? "余额与用量已刷新" : isQuotaCapable(row, quotaMap[row.id]) ? "配额已刷新" : "用量已刷新",
      () => refreshRowTelemetry(row),
    );

  const groupsOf = (row: AccountRow) =>
    (row.groups.length ? row.groups : row.group_ids.map(id => ({id, name: `分组 #${id}`})))
      .map(group => groups?.value?.find(item => item.id === group.id) ?? group);

  function recoverAccount(row: AccountRow) {
    const problem = accountProblem(row);
    if (!problem) return;
    if (problem.action === "edit") { openAccount(row.id); return; }
    void act(row, "已恢复运行状态", () =>
      problem.action === "revert-proxy-fallback" ? api("/admin/accounts/" + row.id + "/revert-proxy-fallback", empty, "POST", {}) :
      problem.action === "temp-unschedulable" ? api("/admin/accounts/" + row.id + "/temp-unschedulable", empty, "DELETE") : post(row, problem.action),
    );
  }

  const {scheduleSwitch, accountRecovery, rowActions, rowMore} = createAccountRowControls({
    busyID,
    deletionTrigger,
    setRowMenuOpen,
    setModelsAccount,
    openAccount,
    act,
    post,
    probe,
    refreshCardTelemetry,
    recoverAccount,
    onRequestDelete: row => { removeAction.clearError(); setDeleting(row); },
  });

  const providerPills = useMemo(() => {
    const known = PLATFORM_OPTIONS.filter(p => p.value !== "composite" && ((platformCounts[p.value] ?? 0) > 0 || p.value === platform));
    return [
      {value: "", label: totals ? `全部 ${totals.total}` : "全部"},
      ...known.map(p => {
        const c = platformCounts[p.value];
        return {value: p.value, label: c !== undefined ? `${p.label} ${c}` : p.label};
      }),
    ];
  }, [platform, platformCounts, totals]);

  const columns = buildAccountColumns({results, balanceMap, groupsOf, accountRecovery, scheduleSwitch, rowActions, rowMore});
  const index = inspect ? rows.findIndex(row => row.id === inspect) : -1;
  const emptyState = (
    <EmptyState
      title={filtered ? "没有符合筛选条件的账号" : "还没有上游账号"}
      action={
        filtered
          ? <Button size="sm" variant="outline" onClick={resetFilters}>清除筛选</Button>
          : <Button size="sm" onClick={() => setAddOpen(true)}><Plus aria-hidden />添加账号</Button>
      }
    />
  );

  function renderCard(row: AccountRow) {
    const quota = quotaMap[row.id];
    const hasQuota = isQuotaCapable(row, quota);
    const mode: CardMode = cardModes[row.id] ?? (hasQuota ? "quota" : "usage");
    const win: UsageWindow = cardWindows[row.id] ?? defaultWindow;
    const bal = row.balance?.kind ? row.balance : balanceMap[row.id] ?? row.balance;
    const statKey = `${row.id}:${win}`;
    const cachedWin = win === "1"
      ? row.today_cost !== undefined || row.today_requests !== undefined
        ? {requests: row.today_requests ?? 0, tokens: row.today_tokens ?? 0, cost: row.today_cost ?? 0}
        : undefined
      : statsCache[statKey];

    return (
      <AccountCardItem
        key={row.id}
        row={row}
        quota={quota}
        balance={bal}
        mode={mode}
        win={win}
        quotaDisplay={quotaDisplay}
        cachedWin={cachedWin}
        statBusy={win !== "1" && Boolean(loadingStats[statKey])}
        selected={selected.includes(row.id)}
        active={inspect === row.id}
        groups={groupsOf(row)}
        result={results[row.id]}
        onSelect={next => setSelected(next ? [...selected, row.id] : selected.filter(id => id !== row.id))}
        onOpen={() => setInspect(row.id)}
        onModeChange={next => {
          setCardModes(prev => ({...prev, [row.id]: next}));
          if (next === "usage" && (win === "7" || win === "30")) void ensureWindowStats(row.id, win);
        }}
        onWindowChange={next => {
          setCardWindows(prev => ({...prev, [row.id]: next}));
          if (next === "7" || next === "30") void ensureWindowStats(row.id, next);
        }}
        scheduleSwitch={scheduleSwitch(row)}
        recoveryAction={accountRecovery(row)}
        actions={rowActions(row, true)}
        more={rowMore(row, true)}
      />
    );
  }

  return (
    <>
      <WorkspaceStage
        flush
        inspectorOpen={!!inspect}
        inspector={
          <Inspector
            open={!!inspect}
            title={rows[index]?.name ?? "上游账号"}
            subtitle={inspect ? `上游账号 · #${inspect}` : undefined}
            onClose={() => setInspect(undefined)}
            onPrev={index > 0 ? () => setInspect(rows[index - 1].id) : undefined}
            onNext={index >= 0 && index < rows.length - 1 ? () => setInspect(rows[index + 1].id) : undefined}
          >
            {inspect && <AccountSummary id={inspect} row={rows[index]} revision={revision} onShowModels={acc => setModelsAccount(acc)} />}
          </Inspector>
        }
      >
        <AccountWorkspaceHeader
          totals={totals}
          platform={platform}
          providerPills={providerPills}
          status={status}
          typeFilter={typeFilter}
          view={view}
          defaultWindow={defaultWindow}
          quotaDisplay={quotaDisplay}
          search={search}
          groupFilter={groupFilter}
          proxyFilter={proxyFilter}
          sort={sort}
          selected={selected}
          batchBusy={batchAction.busy}
          batchError={batchAction.error}
          addTrigger={addTrigger}
          deletionTrigger={deletionTrigger}
          onResetFilters={resetFilters}
          onSetStatus={next => updateAccountQuery({status: next})}
          onUpdateQuery={updateAccountQuery}
          onDefaultWindowChange={next => { setDefaultWindow(next); setCardWindows({}); }}
          onQuotaDisplayChange={setQuotaDisplay}
          onViewChange={setView}
          onRefresh={refresh}
          onAddOpen={() => setAddOpen(true)}
          onSearchChange={setSearch}
          onCRSOpen={() => setCRSOpen(true)}
          onAccountData={setAccountData}
          onBatchFiltered={() => void batchAction.run(async () => {
            const matching = await api(query.replace("/admin/accounts", "/admin/accounts/batch-targets"), z.array(named));
            if (matching.length === 0) throw new Error("没有符合当前筛选条件的账号");
            setBatchTargets(matching);
          })}
          onClearSelected={() => setSelected([])}
          onBulkBindOpen={() => setBulkBindOpen(true)}
          onBatchRun={work => void batchAction.run(work)}
          onDeleteBatch={() => { removeAction.clearError(); setDeleting("batch"); }}
        />

        <div className={view === "list" ? "swift-pane-table account-results" : "account-results"}
          onFocusCapture={() => setRowInteraction(true)}
          onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setRowInteraction(false); }}
          onPointerDownCapture={() => setRowInteraction(true)}
          onPointerLeave={() => { if (!document.activeElement?.closest(".account-results")) setRowInteraction(false); }}>
          <RemoteTable
            path={query}
            schema={account}
            columns={columns}
            rowID={row => row.id}
            label="上游账号"
            revision={revision}
            onItemsLoaded={setRows}
            livePath="/admin/accounts/stream"
            onLiveStatus={setLiveStatus}
            holdLiveUpdates={rowInteraction || rowMenuOpen || selected.length > 0 || addOpen || bulkBindOpen || crsOpen || !!accountData || !!batchTargets || !!modelsAccount || !!deleting || busyID !== undefined || batchAction.busy}
            empty={emptyState}
            renderItems={(items, stale) => {
              const activityAvailable = liveStatus !== "reconnecting" && liveStatus !== "denied";
              const ordered = sortAccounts(activityAvailable ? items : items.map(row => ({...row, activity_known: false})), sort);
              return <ReorderMotion key={view} order={ordered.map(item => item.id).join(",")}>
                {view === "list" ? (
                  <DataTable label="上游账号" rows={ordered} columns={columns} rowID={row => row.id}
                    activeRowID={inspect} onRowClick={row => setInspect(row.id)} selection={{selected, onChange: setSelected}}
                    stale={stale} motion pinActions />
                ) : (
                  <EntityGrid label="上游账号">{ordered.map(renderCard)}</EntityGrid>
                )}
              </ReorderMotion>;
            }}
          />
        </div>
      </WorkspaceStage>

      <AccountModelsDialog accountRow={modelsAccount} onClose={() => setModelsAccount(undefined)} />
      {batchTargets && <AccountBatch targets={batchTargets} onClose={() => setBatchTargets(undefined)} onChanged={refresh} />}
      {crsOpen && <AccountCRS onClose={() => setCRSOpen(false)} onChanged={refresh} />}
      {accountData && <AccountData mode={accountData} selected={selected} onClose={() => setAccountData(undefined)} onChanged={refresh} />}

      <AddAccountDialog
        open={addOpen}
        revision={revision}
        defaultGroupID={groupFilter ? Number(groupFilter) : undefined}
        defaultProxyID={proxyFilter ? Number(proxyFilter) : undefined}
        onClose={() => setAddOpen(false)}
        onCreated={createdID => { setAddOpen(false); refresh(); if (createdID) setInspect(createdID); }}
        onInlineResourceCreated={refresh}
      />

      <BulkBindAccountsDialog
        open={bulkBindOpen}
        ids={selected}
        revision={revision}
        onClose={() => setBulkBindOpen(false)}
        onSaved={() => { setBulkBindOpen(false); setSelected([]); toast("已更新所选账号的分组与代理"); refresh(); }}
      />

      <ConfirmDialog
        open={!!deleting}
        title="删除上游账号"
        description={
          deleting === "batch"
            ? `确定删除选中的 ${selected.length} 个上游账号吗？删除后无法恢复。`
            : `确定删除「${deleting?.name ?? ""}」吗？删除后无法恢复。`
        }
        confirm="删除"
        busy={removeAction.busy}
        error={removeAction.error}
        finalFocus={() => deletionTrigger.current?.isConnected ? deletionTrigger.current : addTrigger.current}
        onClose={() => setDeleting(undefined)}
        onConfirm={() =>
          void removeAction.run(async () => {
            if (deleting === "batch") {
              await api("/admin/accounts/batch-delete", z.object({deleted: z.number()}), "POST", {ids: selected});
              setSelected([]);
            } else if (deleting) {
              await api(`/admin/accounts/${deleting.id}`, empty, "DELETE");
              if (inspect === deleting.id) setInspect(undefined);
            }
            setDeleting(undefined);
            refresh();
          })
        }
      />
    </>
  );
}
