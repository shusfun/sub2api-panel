import {useEffect, useMemo, useState} from "react";
import {Activity, Download, FileUp, Gauge, Plus, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {proxyQuality, proxyRow, proxyTest} from "@/schemas";
import {count} from "@/lib/format";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {navigateTo} from "@/app/route";
import {
  Badge,
  BatchBar,
  Button,
  ConfirmDialog,
  EmptyState,
  ErrorLine,
  FilterBar,
  GeoExitBadge,
  Inspector,
  Menu,
  MenuItem,
  MoreActions,
  QuickPills,
  RefreshButton,
  RemoteTable,
  RouteChain,
  RowActions,
  SearchInput,
  Select,
  Status,
  WorkspaceStage,
  toast,
  type Column,
} from "@/ui";
import {statusPills, type ProxyRow} from "@/features/pool/shared";
import {CreateProxyDialog, ImportProxiesDialog, PasteProxiesDialog} from "./proxy-dialogs";
import {ProxyInspector} from "./proxy-inspector";

export function ProxiesWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [status, setStatus] = useState("");
  const [protocol, setProtocol] = useState("");
  const [search, setSearch] = useState("");
  const [revision, revise] = useState(0);
  const [selected, setSelected] = useState<number[]>([]);
  const [rows, setRows] = useState<ProxyRow[]>([]);
  const [inspect, setInspect] = useState<number | undefined>(() => {
    const id = Number(initial.get("inspect") ?? 0);
    return id > 0 ? id : undefined;
  });
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [pasteOpen, setPasteOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const action = useAction();
  const batchAction = useAction();

  useEffect(() => {
    const id = Number(new URLSearchParams(queryString).get("inspect") ?? 0);
    if (id > 0) setInspect(id);
  }, [queryString]);

  const query = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (status) p.set("status", status);
    if (protocol) p.set("protocol", protocol);
    const qs = p.toString();
    return "/admin/proxies" + (qs ? "?" + qs : "");
  }, [debouncedSearch, status, protocol]);

  const proxyNameByID = useMemo(() => new Map(rows.map(r => [r.id, r.name])), [rows]);

  const columns: Column<ProxyRow>[] = [
    {
      id: "proxy",
      title: "代理节点",
      render: row => (
        <div>
          <strong>{row.name}</strong>
          <div className="muted code-mono">{row.protocol}://{row.host}:{row.port}</div>
        </div>
      ),
    },
    {
      id: "geo",
      title: "出口位置",
      hideWhenCompact: true,
      render: row => <GeoExitBadge countryCode={row.country_code} country={row.country} city={row.city} ip={row.ip_address} />,
    },
    {
      id: "fallback",
      title: "回退链路",
      hideWhenNarrow: true,
      render: row => {
        if (row.fallback_mode === "direct") return <RouteChain nodes={[row.name, "直连"]} />;
        if (row.fallback_mode === "proxy" && row.backup_proxy_id) {
          return <RouteChain nodes={[row.name, proxyNameByID.get(row.backup_proxy_id) ?? `#${row.backup_proxy_id}`]} />;
        }
        return <span className="muted">不回退</span>;
      },
    },
    {
      id: "accounts",
      title: "挂载账号",
      hideWhenCompact: true,
      align: "number",
      render: row => (
        <Button size="xs" variant="ghost" onClick={event => { event.stopPropagation(); navigateTo("accounts", {proxy: row.id}); }}>
          {count.format(row.account_count)} 个
        </Button>
      ),
    },
    {
      id: "latency",
      title: "延迟",
      align: "number",
      render: row => row.latency_ms === null
        ? <span className="muted">未测速</span>
        : <Badge variant="soft" tone={row.latency_status === "success" ? "ok" : "bad"}>{Math.round(row.latency_ms)} ms</Badge>,
    },
    {
      id: "quality",
      title: "质量",
      hideWhenNarrow: true,
      render: row => row.quality_status
        ? <Badge variant="soft" tone={row.quality_status === "healthy" ? "ok" : row.quality_status === "failed" ? "bad" : "warn"}>{row.quality_grade || row.quality_status}</Badge>
        : <span className="muted">未检查</span>,
    },
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {
      id: "ops",
      title: "",
      align: "end",
      render: row => (
        <RowActions>
          <Button size="icon-sm" variant="ghost" title="测连通" aria-label={`测试 ${row.name}`} busy={action.busy}
            onClick={() => void action.run(async () => {
              const res = await api(`/admin/proxies/${row.id}/test`, proxyTest, "POST", {});
              toast(res.success ? `${row.name} 连通正常（${res.latency_ms ?? 0} ms）` : `${row.name} 连接失败`, res.success ? "success" : "warn");
              refresh();
            })}><Activity /></Button>
          <MoreActions label={row.name + " 的更多操作"}>
            <MenuItem disabled={action.busy} onSelect={() => void action.run(async () => {
              const res = await api(`/admin/proxies/${row.id}/quality-check`, proxyQuality, "POST", {});
              toast(`${row.name} 质量 ${res.grade}（${res.score} 分）`);
              refresh();
            })}><Gauge aria-hidden />检查质量</MenuItem>
          </MoreActions>
        </RowActions>
      ),
    },
  ];

  async function exportProxies() {
    await action.run(async () => {
      const data = await api("/admin/proxies/export", z.object({exported_at: z.string(), proxies: z.array(proxyRow)}));
      const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], {type: "application/json"}));
      const a = document.createElement("a");
      a.href = url;
      a.download = "proxies.json";
      a.click();
      URL.revokeObjectURL(url);
      toast(`已导出 ${data.proxies.length} 条代理配置`);
    });
  }

  const index = inspect ? rows.findIndex(row => row.id === inspect) : -1;

  return (
    <>
      <WorkspaceStage
        flush
        inspectorOpen={!!inspect}
        inspector={
          <Inspector
            open={!!inspect}
            title={rows[index]?.name ?? "代理节点"}
            subtitle={inspect ? `代理节点 · #${inspect}` : undefined}
            onClose={() => setInspect(undefined)}
            onPrev={index > 0 ? () => setInspect(rows[index - 1].id) : undefined}
            onNext={index >= 0 && index < rows.length - 1 ? () => setInspect(rows[index + 1].id) : undefined}
          >
            {inspect && (
              <ProxyInspector key={`prx-${inspect}`} id={inspect} revision={revision} onChanged={refresh}
                onDeleted={() => { setInspect(undefined); refresh(); }} />
            )}
          </Inspector>
        }
      >
        <FilterBar sticky>
          <div className="obs-filter-row">
            <SearchInput value={search} onChange={setSearch} placeholder="搜索名称或地址" ariaLabel="搜索代理" />
            <Select aria-label="按协议筛选" value={protocol} onChange={e => setProtocol(e.target.value)}>
              <option value="">全部协议</option>
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="socks5">SOCKS5</option>
              <option value="socks5h">SOCKS5H</option>
            </Select>
            <QuickPills label="状态筛选" value={status} options={statusPills} onChange={setStatus} />
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新代理" onClick={refresh} />
              <Menu trigger={<Button variant="outline" busy={action.busy}><FileUp aria-hidden />导入 / 导出</Button>}>
                <MenuItem onSelect={() => setImportOpen(true)}><FileUp aria-hidden />导入代理</MenuItem>
                <MenuItem onSelect={() => setPasteOpen(true)}>粘贴代理地址</MenuItem>
                <MenuItem onSelect={() => void exportProxies()}><Download aria-hidden />导出为 JSON</MenuItem>
              </Menu>
              <Button onClick={() => setCreateOpen(true)}><Plus aria-hidden />添加代理</Button>
            </div>
          </div>
          <ErrorLine value={action.error || batchAction.error} />
        </FilterBar>

        <BatchBar count={selected.length} onClear={() => setSelected([])}>
          <Button size="xs" variant="outline" busy={batchAction.busy} onClick={() => void batchAction.run(async () => {
            for (const id of selected) await api(`/admin/proxies/${id}`, proxyRow, "PUT", {status: "active"});
            toast(`已启用 ${selected.length} 个代理`);
            setSelected([]);
            refresh();
          })}>批量启用</Button>
          <Button size="xs" variant="outline" busy={batchAction.busy} onClick={() => void batchAction.run(async () => {
            for (const id of selected) await api(`/admin/proxies/${id}`, proxyRow, "PUT", {status: "inactive"});
            toast(`已停用 ${selected.length} 个代理`);
            setSelected([]);
            refresh();
          })}>批量停用</Button>
          <Button size="xs" variant="outline" busy={batchAction.busy} onClick={() => void batchAction.run(async () => {
            let passed = 0;
            for (const id of selected) {
              if ((await api(`/admin/proxies/${id}/test`, proxyTest, "POST", {})).success) passed++;
            }
            toast(`${passed} / ${selected.length} 个代理连通正常`);
            refresh();
          })}>测连通</Button>
          <Button size="xs" variant="outline" busy={batchAction.busy} onClick={() => void batchAction.run(async () => {
            for (const id of selected) await api(`/admin/proxies/${id}/quality-check`, proxyQuality, "POST", {});
            toast(`已检查 ${selected.length} 个代理的质量`);
            refresh();
          })}>检查质量</Button>
          <Button size="xs" variant="destructive" onClick={() => { batchAction.clearError(); setConfirmDelete(true); }}><Trash2 aria-hidden />删除</Button>
        </BatchBar>

        <div className="swift-pane-table">
          <RemoteTable
            key={query}
            path={query}
            schema={proxyRow}
            columns={columns}
            rowID={row => row.id}
            label="代理"
            pinActions
            revision={revision}
            activeRowID={inspect}
            onRowClick={row => setInspect(row.id)}
            selection={{selected, onChange: setSelected}}
            onItemsLoaded={setRows}
            empty={<EmptyState title="还没有代理" action={<Button size="sm" onClick={() => setCreateOpen(true)}><Plus aria-hidden />添加代理</Button>} />}
          />
        </div>
      </WorkspaceStage>

      <CreateProxyDialog
        open={createOpen}
        revision={revision}
        onClose={() => setCreateOpen(false)}
        onCreated={px => { setCreateOpen(false); refresh(); setInspect(px.id); }}
      />
      <ImportProxiesDialog
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onImported={msg => { setImportOpen(false); toast(msg); refresh(); }}
      />
      {pasteOpen && <PasteProxiesDialog onClose={() => setPasteOpen(false)} onImported={message => { setPasteOpen(false); toast(message); refresh(); }} />}
      <ConfirmDialog
        open={confirmDelete}
        title="删除代理"
        description={`确定删除选中的 ${selected.length} 个代理吗？仍被账号使用的代理会被跳过。`}
        confirm="删除"
        busy={batchAction.busy}
        error={batchAction.error}
        onClose={() => setConfirmDelete(false)}
        onConfirm={() => void batchAction.run(async () => {
          await api("/admin/proxies/batch-delete", z.object({deleted_ids: z.array(z.number()), skipped: z.array(z.any())}), "POST", {ids: selected});
          setSelected([]);
          setConfirmDelete(false);
          refresh();
        })}
      />
    </>
  );
}
