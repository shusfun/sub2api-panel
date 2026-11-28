import {useMemo, useState} from "react";
import {ArrowDown, ArrowUp, Plus} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {adminGroupRow, empty} from "@/schemas";
import {count, smartMoney} from "@/lib/format";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {navigateTo} from "@/app/route";
import {
  Badge,
  Button,
  EmptyState,
  FilterBar,
  FilterSlot,
  InlineStack,
  MeterBar,
  PlatformBadge,
  PlatformSelect,
  RefreshButton,
  RemoteTable,
  RouteChain,
  RowActions,
  SearchInput,
  SegmentBar,
  Status,
  WorkspaceStage,
  toast,
  type Column,
} from "@/ui";
import {groupBriefSchema, type GroupRow} from "@/features/pool/shared";
import {CreateGroupDialog} from "./group-dialogs";
import {GroupDetailWorkspace} from "./group-detail";

export function GroupsWorkspace({queryString = ""}: {queryString?: string}) {
  const [platform, setPlatform] = useState("");
  const [search, setSearch] = useState("");
  const [revision, revise] = useState(0);
  const [groupRows, setGroupRows] = useState<GroupRow[]>([]);
  const [createOpen, setCreateOpen] = useState(false);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const reorderOp = useAction();

  const inspectGroupFromURL = useMemo(() => {
    const id = Number(new URLSearchParams(queryString).get("inspect") ?? 0);
    return id > 0 ? id : 0;
  }, [queryString]);

  const allGroups = useResource(() => api("/admin/groups/all", z.array(groupBriefSchema)), "group-names", true, revision);
  const groupNameByID = useMemo(() => new Map((allGroups?.value ?? []).map(g => [g.id, g.name])), [allGroups?.value]);

  async function moveGroup(rowID: number, dir: -1 | 1) {
    const idx = groupRows.findIndex(r => r.id === rowID);
    const swapIdx = idx + dir;
    if (idx < 0 || swapIdx < 0 || swapIdx >= groupRows.length) return;
    const next = [...groupRows];
    const [moved] = next.splice(idx, 1);
    next.splice(swapIdx, 0, moved);
    await reorderOp.run(async () => {
      await api("/admin/groups/sort-order", empty, "PUT", {
        updates: next.map((g, i) => ({id: g.id, sort_order: i + 1})),
      });
      toast("分组排序已更新");
      refresh();
    });
  }

  const groupQuery = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (platform) p.set("platform", platform);
    const qs = p.toString();
    return "/admin/groups" + (qs ? "?" + qs : "");
  }, [debouncedSearch, platform]);

  if (inspectGroupFromURL > 0) {
    return (
      <GroupDetailWorkspace
        key={`grp-page-${inspectGroupFromURL}`}
        id={inspectGroupFromURL}
        revision={revision}
        onBack={() => navigateTo("groups")}
        onChanged={refresh}
        onDeleted={() => { navigateTo("groups"); refresh(); }}
      />
    );
  }

  const groupCols: Column<GroupRow>[] = [
    {
      id: "group",
      title: "调度分组",
      render: row => (
        <div>
          <InlineStack density="compact">
            <strong className="compact-text" title={row.name}>{row.name}</strong>
            {row.is_exclusive && <Badge variant="tag">私有</Badge>}
            {row.subscription_type === "subscription" && <Badge variant="soft" tone="info">订阅组</Badge>}
          </InlineStack>
          {row.description && <div className="muted text-fine">{row.description}</div>}
        </div>
      ),
    },
    {id: "platform", title: "平台", hideWhenCompact: true, render: row => <PlatformBadge platform={row.platform} />},
    {id: "rate", title: "倍率", align: "number", render: row => `${row.rate_multiplier}×`},
    {
      id: "capacity",
      title: "账号（可用 / 限流 / 异常）",
      hideWhenNarrow: true,
      render: row => (
        <Button size="xs" variant="ghost" onClick={event => { event.stopPropagation(); navigateTo("accounts", {group: row.id}); }}>
          <SegmentBar
            compact
            segments={[
              {label: "可用", value: row.active_account_count, tone: "ok"},
              {label: "限流", value: row.rate_limited_account_count, tone: "warn"},
              {label: "异常", value: row.error_account_count, tone: "bad"},
            ]}
          />
          {count.format(row.account_count)} 个
        </Button>
      ),
    },
    {
      id: "health",
      title: "24h 健康",
      hideWhenNarrow: true,
      render: row => row.health_score !== null
        ? <Badge variant="soft" tone={row.health === "healthy" ? "ok" : row.health === "critical" ? "bad" : row.health === "warning" ? "warn" : "neutral"}>{Math.round(row.health_score)} 分</Badge>
        : <span className="muted">无样本</span>,
    },
    {
      id: "route",
      title: "调度兜底链",
      hideWhenNarrow: true,
      render: row => {
        const nodes: string[] = [row.name];
        if (row.fallback_group_id) nodes.push(groupNameByID.get(row.fallback_group_id) ?? `#${row.fallback_group_id}`);
        if (row.fallback_group_id_on_invalid_request) nodes.push(`无效:${groupNameByID.get(row.fallback_group_id_on_invalid_request) ?? `#${row.fallback_group_id_on_invalid_request}`}`);
        return nodes.length > 1 ? <RouteChain nodes={nodes} /> : <span className="muted">直出</span>;
      },
    },
    {
      id: "load",
      title: "并发 / RPM",
      hideWhenCompact: true,
      render: row => (
        <div className="meter-col">
          <MeterBar label={`并发 ${row.concurrency_used}/${row.concurrency_max}`} value={row.concurrency_max > 0 ? Math.min(100, Math.round((row.concurrency_used / row.concurrency_max) * 100)) : 0} />
          {row.rpm_max > 0 && <MeterBar label={`RPM ${row.rpm_used}/${row.rpm_max}`} value={Math.min(100, Math.round((row.rpm_used / row.rpm_max) * 100))} />}
        </div>
      ),
    },
    {id: "cost", title: "今日 / 累计", align: "number", hideWhenCompact: true, render: row => `${smartMoney(row.today_cost)} / ${smartMoney(row.total_cost)}`},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {
      id: "sort",
      title: "排序",
      align: "end",
      render: row => (
        <RowActions>
          <Button size="icon-xs" variant="ghost" title="上移" aria-label={`上移 ${row.name}`} busy={reorderOp.busy} onClick={() => void moveGroup(row.id, -1)}><ArrowUp /></Button>
          <Button size="icon-xs" variant="ghost" title="下移" aria-label={`下移 ${row.name}`} busy={reorderOp.busy} onClick={() => void moveGroup(row.id, 1)}><ArrowDown /></Button>
        </RowActions>
      ),
    },
  ];

  return (
    <>
      <WorkspaceStage flush>
        <FilterBar sticky>
          <div className="obs-filter-row">
            <SearchInput value={search} onChange={setSearch} placeholder="搜索名称" ariaLabel="搜索分组" />
            <FilterSlot><PlatformSelect value={platform} includeComposite onChange={setPlatform} /></FilterSlot>
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新" onClick={refresh} />
              <Button onClick={() => setCreateOpen(true)}><Plus aria-hidden />新建分组</Button>
            </div>
          </div>
        </FilterBar>

        <div className="swift-pane-table">
          <RemoteTable
            key={groupQuery}
            path={groupQuery}
            schema={adminGroupRow}
            columns={groupCols}
            rowID={row => row.id}
            label="调度分组"
            revision={revision}
            onRowClick={row => navigateTo("groups", {inspect: row.id})}
            onItemsLoaded={setGroupRows}
            empty={<EmptyState title="还没有调度分组" action={<Button size="sm" onClick={() => setCreateOpen(true)}><Plus aria-hidden />新建分组</Button>} />}
          />
        </div>
      </WorkspaceStage>

      <CreateGroupDialog
        open={createOpen}
        revision={revision}
        onClose={() => setCreateOpen(false)}
        onCreated={grp => { setCreateOpen(false); refresh(); navigateTo("groups", {inspect: grp.id}); }}
      />
    </>
  );
}
