import {useMemo, useState} from "react";
import {z} from "zod";
import {Plus} from "lucide-react";
import {api} from "@/lib/http";
import {channelRowSchema} from "@/schemas";
import {useDebouncedValue, useResource} from "@/lib/requests";
import {navigateTo} from "@/app/route";
import {
  Badge, Button, EmptyState, EntityLink, FilterBar, InlineStack, Inspector, RefreshButton,
  RemoteTable, SearchInput, Status, WorkspaceStage, type Column,
} from "@/ui";
import {groupBriefSchema, type ChannelRow} from "@/features/pool/shared";
import {ChannelInspector} from "./channel-inspector";
import {CreateChannelDialog} from "./channel-dialogs";

export function ChannelsWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [search, setSearch] = useState("");
  const [revision, revise] = useState(0);
  const [inspectID, setInspectID] = useState<number | undefined>(() => {
    const id = Number(initial.get("inspect") ?? 0);
    return id > 0 ? id : undefined;
  });
  const [createOpen, setCreateOpen] = useState(false);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);

  const allGroups = useResource(() => api("/admin/groups/all", z.array(groupBriefSchema)), "group-names", true, revision);

  const columns: Column<ChannelRow>[] = [
    {
      id: "name",
      title: "调价模板",
      render: row => (
        <div>
          <strong className="compact-text" title={row.name}>{row.name}</strong>
          {row.description && <div className="muted text-fine">{row.description}</div>}
        </div>
      ),
    },
    {
      id: "groups",
      title: "作用分组",
      hideWhenNarrow: true,
      grow: true,
      render: row => (
        <InlineStack density="compact" wrap>
          {row.group_ids.length ? row.group_ids.map(gid => {
            const g = allGroups?.value?.find(item => item.id === gid);
            return <EntityLink key={gid} kind="group" id={gid} label={g?.name ?? `#${gid}`} onClick={() => navigateTo("groups", {inspect: gid})} />;
          }) : <span className="muted">全部分组</span>}
        </InlineStack>
      ),
    },
    {id: "rules", title: "自定义价目", align: "number", render: row => `${row.model_pricing.length} 组`},
    {id: "source", title: "计费模型", render: row => <Badge variant="tag">{row.billing_model_source === "upstream" ? "上游模型" : "请求模型"}</Badge>},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
  ];

  return (
    <>
      <WorkspaceStage
        flush
        inspectorOpen={!!inspectID}
        inspector={
          <Inspector
            open={!!inspectID}
            title="调价模板"
            subtitle={inspectID ? `调价模板 · #${inspectID}` : undefined}
            onClose={() => setInspectID(undefined)}
          >
            {inspectID && (
              <ChannelInspector key={`chn-${inspectID}`} id={inspectID} revision={revision} onChanged={refresh}
                onDeleted={() => { setInspectID(undefined); refresh(); }} />
            )}
          </Inspector>
        }
      >
        <FilterBar sticky>
          <div className="obs-filter-row">
            <SearchInput value={search} onChange={setSearch} placeholder="搜索名称" ariaLabel="搜索调价模板" />
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新" onClick={refresh} />
              <Button onClick={() => setCreateOpen(true)}><Plus aria-hidden />新建调价模板</Button>
            </div>
          </div>
        </FilterBar>

        <div className="swift-pane-table">
          <RemoteTable
            key={debouncedSearch}
            path={"/admin/channels" + (debouncedSearch ? "?search=" + encodeURIComponent(debouncedSearch) : "")}
            schema={channelRowSchema}
            columns={columns}
            rowID={row => row.id}
            label="调价模板"
            revision={revision}
            activeRowID={inspectID}
            onRowClick={row => setInspectID(row.id)}
            empty={
              <EmptyState
                title="没有调价模板"
                description="不建模板时按分组倍率和官方单价计费。"
                action={<Button size="sm" onClick={() => setCreateOpen(true)}><Plus aria-hidden />新建调价模板</Button>}
              />
            }
          />
        </div>
      </WorkspaceStage>

      <CreateChannelDialog
        open={createOpen}
        revision={revision}
        onClose={() => setCreateOpen(false)}
        onCreated={ch => { setCreateOpen(false); refresh(); setInspectID(ch.id); }}
      />
    </>
  );
}
