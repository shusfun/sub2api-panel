import {type MutableRefObject, type RefObject} from "react";
import {Boxes, Copy as DuplicateIcon, PanelRight, Plus, Trash2, Wrench, Zap} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty, type adminOverview} from "@/schemas";
import {count} from "@/lib/format";
import {navigateTo} from "@/app/route";
import {
  BatchBar,
  Button,
  ErrorLine,
  FilterBar,
  FilterSlot,
  GroupSelect,
  MenuItem,
  MenuSeparator,
  MetricStrip,
  MoreActions,
  ProxySelect,
  QuickPills,
  RefreshButton,
  RefreshMorphIcon,
  SearchInput,
  SecondaryTableAction,
  Segmented,
  Select,
  Switch,
  Toolbar,
  ViewToggle,
  toast,
  type ViewMode,
} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";

const accountStatusPills = [
  {value: "", label: "全部"},
  {value: "active", label: "正常"},
  {value: "rate_limited", label: "限流"},
  {value: "overloaded", label: "过载"},
  {value: "unschedulable", label: "不可调度"},
  {value: "expired", label: "已到期"},
  {value: "error", label: "异常"},
  {value: "inactive", label: "已停用"},
];

const accountTypeOptions = [
  {value: "", label: "全部类型"},
  {value: "oauth", label: "OAuth"},
  {value: "setup-token", label: "Setup Token"},
  {value: "apikey", label: "API Key"},
  {value: "upstream", label: "上游中转"},
  {value: "bedrock", label: "AWS Bedrock"},
  {value: "service_account", label: "GCP SA"},
];
import {
  accountProblem,
  canRefreshBalance,
  quotaDisplayKey,
  recoveryLabel,
  sortOptions,
  viewKey,
  type AccountSort,
  type QuotaDisplay,
  type UsageWindow,
} from "./account-helpers";

type OverviewAccounts = z.infer<typeof adminOverview>["accounts"];

export function AccountWorkspaceHeader({
  totals,
  platform,
  providerPills,
  status,
  typeFilter = "",
  view,
  defaultWindow,
  quotaDisplay,
  search,
  groupFilter,
  proxyFilter,
  sort,
  selected,
  batchBusy,
  batchError,
  addTrigger,
  deletionTrigger,
  onResetFilters,
  onSetStatus,
  onUpdateQuery,
  onDefaultWindowChange,
  onQuotaDisplayChange,
  onViewChange,
  onRefresh,
  onAddOpen,
  onSearchChange,
  onCRSOpen,
  onAccountData,
  onBatchFiltered,
  onClearSelected,
  onBulkBindOpen,
  onBatchRun,
  onDeleteBatch,
}: {
  totals?: OverviewAccounts;
  platform: string;
  providerPills: {value: string; label: string}[];
  status: string;
  typeFilter?: string;
  view: ViewMode;
  defaultWindow: UsageWindow;
  quotaDisplay: QuotaDisplay;
  search: string;
  groupFilter: string;
  proxyFilter: string;
  sort: AccountSort;
  selected: number[];
  batchBusy: boolean;
  batchError: string;
  addTrigger: RefObject<HTMLButtonElement | null>;
  deletionTrigger: MutableRefObject<HTMLElement | null>;
  onResetFilters: () => void;
  onSetStatus: (next: string) => void;
  onUpdateQuery: (update: Record<string, string>) => void;
  onDefaultWindowChange: (next: UsageWindow) => void;
  onQuotaDisplayChange: (next: QuotaDisplay) => void;
  onViewChange: (next: ViewMode) => void;
  onRefresh: () => void;
  onAddOpen: () => void;
  onSearchChange: (next: string) => void;
  onCRSOpen: () => void;
  onAccountData: (mode: "import" | "export") => void;
  onBatchFiltered: () => void;
  onClearSelected: () => void;
  onBulkBindOpen: () => void;
  onBatchRun: (work: () => Promise<void>) => void;
  onDeleteBatch: () => void;
}) {
  return (
    <>
      <MetricStrip
        flush
        items={[
          {id: "total", label: "账号总数", value: totals ? count.format(totals.total) : null, onClick: onResetFilters},
          {id: "normal", label: "可调度", value: totals ? count.format(totals.normal) : null, tone: "ok", onClick: () => onSetStatus("active")},
          ...(totals && (totals.rate_limited > 0 || totals.error > 0 || totals.overloaded > 0)
            ? [
                {id: "limited", label: "限流中", value: count.format(totals.rate_limited), tone: (totals.rate_limited ? "warn" : "neutral") as "warn" | "neutral", onClick: () => onSetStatus("rate_limited")},
                {id: "error", label: "异常", value: count.format(totals.error), tone: (totals.error ? "bad" : "neutral") as "bad" | "neutral", onClick: () => onSetStatus("error")},
                {id: "overloaded", label: "过载", value: count.format(totals.overloaded), tone: (totals.overloaded ? "warn" : "neutral") as "warn" | "neutral", onClick: () => onSetStatus("overloaded")},
              ]
            : [
                {id: "health", label: "异常与限流", value: totals ? "无异常" : null, hint: totals ? "限流 · 报错 · 过载均为 0" : undefined, tone: "ok" as const},
              ]),
        ]}
      />
      <FilterBar sticky>
        <div className="obs-filter-row">
          <QuickPills label="上游平台" value={platform} options={providerPills} onChange={value => onUpdateQuery({platform: value})} />
          <QuickPills label="状态筛选" value={status} options={accountStatusPills} onChange={value => onUpdateQuery({status: value})} />
          <div className="obs-filter-spacer">
            {view === "card" && (
              <>
                <Segmented
                  label="默认统计周期"
                  value={defaultWindow}
                  options={[["1", "今日"], ["7", "7d"], ["30", "30d"]] as const}
                  onChange={onDefaultWindowChange}
                />
                <Segmented
                  label="配额展示口径"
                  value={quotaDisplay}
                  options={[["used", "已用"], ["remaining", "剩余"]] as const}
                  onChange={next => {
                    onQuotaDisplayChange(next);
                    localStorage.setItem(quotaDisplayKey, next);
                  }}
                />
              </>
            )}
            <ViewToggle
              value={view}
              onChange={next => {
                onViewChange(next);
                localStorage.setItem(viewKey, next);
              }}
            />
            <RefreshButton iconOnly size="default" variant="ghost" label="刷新账号" onClick={onRefresh} />
            <Button ref={addTrigger} onClick={onAddOpen}><Plus aria-hidden />添加账号</Button>
          </div>
        </div>
        <div className="obs-filter-row">
          <SearchInput value={search} onChange={onSearchChange} placeholder="搜索名称或备注" ariaLabel="搜索上游账号" />
          <FilterSlot><GroupSelect value={groupFilter} placeholder="全部分组" onChange={value => onUpdateQuery({group: value})} /></FilterSlot>
          <FilterSlot><ProxySelect value={proxyFilter} placeholder="全部代理" onChange={value => onUpdateQuery({proxy: value})} /></FilterSlot>
          <FilterSlot>
            <Select aria-label="账号类型" value={typeFilter} onChange={event => onUpdateQuery({type: event.target.value})}>
              {accountTypeOptions.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
            </Select>
          </FilterSlot>
          <FilterSlot>
            <Select aria-label="账号排序" value={sort} onChange={event => onUpdateQuery({sort: event.target.value})}>
              {sortOptions.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
            </Select>
          </FilterSlot>
        </div>
        <ErrorLine value={batchError} />
      </FilterBar>

      <Toolbar>
        <Button size="sm" variant="outline" onClick={onCRSOpen}>同步 CRS</Button>
        <Button size="sm" variant="outline" onClick={() => onAccountData("import")}>导入配置</Button>
        <Button size="sm" variant="outline" onClick={() => onAccountData("export")}>导出配置</Button>
        <Button size="sm" variant="outline" busy={batchBusy} onClick={onBatchFiltered}>批量处理筛选结果</Button>
      </Toolbar>

      <BatchBar count={selected.length} onClear={onClearSelected}>
        <Button size="xs" variant="outline" onClick={onBulkBindOpen}>批量参数 / 分组</Button>
        {(
          [
            ["batch-clear-error", "清错", {ids: selected}, "已清错"],
            ["batch-refresh", "刷新凭据", {ids: selected}, "已刷新凭据"],
            ["bulk-update", "暂停调度", {account_ids: selected, schedulable: false}, "已暂停调度"],
            ["bulk-update", "恢复调度", {account_ids: selected, schedulable: true}, "已恢复调度"],
            ["bulk-update", "批量启用", {account_ids: selected, status: "active"}, "已启用"],
            ["bulk-update", "批量停用", {account_ids: selected, status: "inactive"}, "已停用"],
          ] as const
        ).map(([path, label, body, done]) => (
          <Button
            key={label}
            size="xs"
            variant="outline"
            busy={batchBusy}
            onClick={() =>
              onBatchRun(async () => {
                await api(`/admin/accounts/${path}`, z.object({success: z.number(), failed: z.number()}), "POST", body);
                toast(`${selected.length} 个账号${done}`);
                onRefresh();
              })
            }
          >
            {label}
          </Button>
        ))}
        <Button size="xs" variant="destructive" onClick={event => { deletionTrigger.current = event.currentTarget; onDeleteBatch(); }}>
          <Trash2 aria-hidden />删除
        </Button>
      </BatchBar>
    </>
  );
}

export function createAccountRowControls({
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
  onRequestDelete,
}: {
  busyID?: number;
  deletionTrigger: MutableRefObject<HTMLElement | null>;
  setRowMenuOpen: (open: boolean) => void;
  setModelsAccount: (row: AccountRow) => void;
  openAccount: (id: number) => void;
  act: (row: AccountRow, done: string, work: () => Promise<unknown>) => Promise<void>;
  post: (row: AccountRow, action: string) => Promise<unknown>;
  probe: (row: AccountRow) => Promise<void>;
  refreshCardTelemetry: (row: AccountRow) => Promise<void>;
  recoverAccount: (row: AccountRow) => void;
  onRequestDelete: (row: AccountRow) => void;
}) {
  const scheduleSwitch = (row: AccountRow) => (
    <Switch
      label={`${row.schedulable ? "暂停" : "开启"} ${row.name} 调度`}
      checked={row.schedulable}
      disabled={busyID !== undefined}
      onChange={checked =>
        void act(row, checked ? "已开启调度" : "已暂停调度", () =>
          api(`/admin/accounts/${row.id}/scheduling`, empty, "PUT", {schedulable: checked}),
        )
      }
    />
  );

  const accountRecovery = (row: AccountRow) => (
    <Button size="xs" variant="ghost" disabled={busyID !== undefined} onClick={() => recoverAccount(row)}>
      {recoveryLabel(row)}
    </Button>
  );

  const rowActions = (row: AccountRow, card = false) => {
    const busy = busyID === row.id;
    const blocked = busyID !== undefined && !busy;
    return (
      <>
        {card && (
          <Button size="sm" variant="ghost" disabled={blocked} onClick={() => setModelsAccount(row)}>
            <Boxes aria-hidden />模型
          </Button>
        )}
        <Button size="icon-sm" variant="ghost" title="测活" aria-label={`测活 ${row.name}`} busy={busy} disabled={blocked} onClick={() => void probe(row)}>
          <Zap />
        </Button>
        {card && (
          <Button
            size="icon-sm"
            variant="ghost"
            title={canRefreshBalance(row) ? "刷新余额与用量" : "刷新配额与用量"}
            aria-label={`刷新 ${row.name}`}
            disabled={busyID !== undefined}
            onClick={() => void refreshCardTelemetry(row)}
          >
            <RefreshMorphIcon />
          </Button>
        )}
      </>
    );
  };

  const rowMore = (row: AccountRow, card = false) => {
    const problem = accountProblem(row);
    const detail = (
      <Button size="icon-sm" variant="ghost" title="详情" aria-label={`打开 ${row.name} 的详情`} onClick={() => openAccount(row.id)}>
        <PanelRight />
      </Button>
    );
    return (
      <>
        {card ? detail : <SecondaryTableAction>{detail}</SecondaryTableAction>}
        <MoreActions onOpenChange={(open, trigger) => {
          setRowMenuOpen(open);
          if (open) deletionTrigger.current = trigger;
        }} label={`${row.name} 的更多操作`}>
          {!card && <>
            <MenuItem onSelect={() => openAccount(row.id)}><PanelRight aria-hidden />打开详情</MenuItem>
            <MenuItem onSelect={() => setModelsAccount(row)}>查看模型</MenuItem>
            <MenuItem disabled={busyID !== undefined} onSelect={() => void refreshCardTelemetry(row)}>{canRefreshBalance(row) ? "刷新余额与用量" : "刷新配额与用量"}</MenuItem>
          </>}
          <MenuItem disabled={busyID !== undefined} onSelect={() => void act(row, "凭据已刷新", () => post(row, "refresh"))}>刷新凭据</MenuItem>
          {problem && problem.action !== "recover" && problem.action !== "edit" && <MenuItem disabled={busyID !== undefined} onSelect={() => recoverAccount(row)}>{recoveryLabel(row)}</MenuItem>}
          <MenuItem disabled={busyID !== undefined} onSelect={() => void act(row, "已清错并恢复", () => post(row, "recover"))}><Wrench aria-hidden />清错并恢复</MenuItem>
          <MenuSeparator />
          <MenuItem onSelect={() => navigateTo("usage", {account: row.id})}>查看调用明细</MenuItem>
          <MenuItem onSelect={() => void act(row, "已复制为新账号", () => post(row, "duplicate"))}><DuplicateIcon aria-hidden />复制账号</MenuItem>
          <MenuItem onSelect={() => void act(row, "已清除限流", () => post(row, "clear-rate-limit"))}>清除限流</MenuItem>
          <MenuItem onSelect={() => void act(row, "已重置配额", () => post(row, "reset-quota"))}>重置配额</MenuItem>
          <MenuSeparator />
          <MenuItem tone="bad" onSelect={() => onRequestDelete(row)}><Trash2 aria-hidden />删除账号</MenuItem>
        </MoreActions>
      </>
    );
  };

  return {scheduleSwitch, accountRecovery, rowActions, rowMore};
}
