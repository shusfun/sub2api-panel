import { useId,useEffect,useMemo,useState } from "react";
import {
Activity,Download,
FileUp,
Gauge,Plus,
Trash2
} from "lucide-react";
import { z } from "zod";
import {
api,empty,proxyQuality,
proxyRow,
proxyTest
} from "../api";
import { count } from "../format";
import { useAction,useDebouncedValue,useResource } from "../requests";
import { navigateTo } from "../route";
import {FormActions, 
FormSection,
Badge,
BatchBar,
Button,ConfirmDialog,EmptyState,
EntityLink,
ErrorLine,
Field,
FilePicker,
FilterBar,Form,
FormGrid,Input,
InsetGroup,
InsetRow,
Inspector,Overlay,PasswordInput,ProxySelect,
QuickPills,
RefreshButton,
Resource,
SearchInput,Select,
Status,Switch,
Textarea,toast,type Column,Menu,
MenuItem,MoreActions,RowActions,WorkspaceStage
} from "../ui";
import { RemoteTable } from "./remote-table";
import { statusPills,type ProxyRow } from "./pool-shared";

export function ProxiesWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString), [queryString]);
  const [status, setStatus] = useState("");
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
    const qs = p.toString();
    return "/admin/proxies" + (qs ? "?" + qs : "");
  }, [debouncedSearch, status]);

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

function ProxyInspector({
  id,
  revision,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const detail = useResource(() => api(`/admin/proxies/${id}`, proxyRow), `prx-detail-${id}`, true, revision);
  const linked = useResource(
    () => api(`/admin/proxies/${id}/accounts`, z.array(z.object({id: z.number(), name: z.string(), platform: z.string()}))),
    `prx-acc-${id}`,
    true,
    revision,
  );
  const [confirmDelete, setConfirmDelete] = useState(false);
  const del = useAction();

  return (
    <Resource state={detail}>
      {px => (
        <>
          <ProxyEditForm px={px} revision={revision} onSaved={onChanged} onRequestDelete={() => setConfirmDelete(true)} />
          <InsetGroup title={`挂载在此代理的账号 (${linked?.value?.length ?? 0})`}>
            {(linked?.value ?? []).length ? (
              (linked?.value ?? []).map(a => (
                <InsetRow key={a.id} label={a.name} hint={a.platform}>
                  <EntityLink kind="account" id={a.id} label="查看账号" />
                </InsetRow>
              ))
            ) : (
              <InsetRow label="暂无关联账号" />
            )}
          </InsetGroup>
          <ConfirmDialog
            open={confirmDelete}
            title="删除代理节点"
            description={`确定删除代理「${px.name}」吗？`}
            confirm="删除"
            busy={del.busy}
            error={del.error}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void del.run(async () => {
              await api(`/admin/proxies/${px.id}`, empty, "DELETE");
              setConfirmDelete(false);
              onDeleted();
            })}
          />
        </>
      )}
    </Resource>
  );
}

function ProxyEditForm({
  px,
  revision,
  onSaved,
  onRequestDelete,
}: {
  px: ProxyRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(px.name);
  const [protocol, setProtocol] = useState<ProxyRow["protocol"]>(px.protocol);
  const [host, setHost] = useState(px.host);
  const [port, setPort] = useState(String(px.port));
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [fallbackMode, setFallbackMode] = useState(px.fallback_mode || "none");
  const [backupID, setBackupID] = useState(px.backup_proxy_id ? String(px.backup_proxy_id) : "");
  const [active, setActive] = useState(px.status === "active");
  const [clearAuth, setClearAuth] = useState(false);

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/admin/proxies/${px.id}`, proxyRow, "PUT", {
          name: name.trim(),
          protocol,
          host: host.trim(),
          port: Number(port),
          status: active ? "active" : "inactive",
          fallback_mode: fallbackMode,
          ...(fallbackMode === "proxy" && backupID ? {backup_proxy_id: Number(backupID)} : {clear_backup_proxy_id: true}),
          ...(username ? {username} : {}),
          ...(password ? {password} : {}),
          ...(clearAuth ? {clear_auth: true} : {}),
        });
        setUsername("");
        setPassword("");
        setClearAuth(false);
        onSaved();
      })}
    >
      <FormSection title="连接与协议设置">
        <InsetRow label="启用代理"><Switch label="启用代理" checked={active} onChange={setActive} /></InsetRow>
        <Field label="名称"><Input required value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="协议">
          <div className="row-field">
            <Select value={protocol} onChange={e => setProtocol(e.target.value as ProxyRow["protocol"])}>
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="socks5">SOCKS5</option>
              <option value="socks5h">SOCKS5H</option>
            </Select>
          </div>
        </Field>
        <FormSection title="主机与端口">
          <FormGrid cols={3}>
            <div className="col-span-2"><Field label="127.0.0.1"><Input required value={host} onChange={e => setHost(e.target.value)} placeholder="127.0.0.1" /></Field></div>
            <Field label="端口"><Input required type="number" min="1" max="65535" value={port} onChange={e => setPort(e.target.value)} /></Field>
          </FormGrid>
        </FormSection>
      </FormSection>

      <FormSection title="故障回退与认证更新">
        <Field label="故障回退策略">
          <div className="row-field">
            <Select value={fallbackMode} onChange={e => setFallbackMode(e.target.value)}>
              <option value="none">不回退</option>
              <option value="direct">回退直连</option>
              <option value="proxy">切换备用代理</option>
            </Select>
          </div>
        </Field>
        {fallbackMode === "proxy" && (
          <Field label="选择备用代理">
            <ProxySelect value={backupID} excludeID={px.id} placeholder="选择备用代理节点…" revision={revision} onChange={setBackupID} />
          </Field>
        )}
        <FormSection title="新用户名 / 密码（留空保留原认证）">
          <FormGrid>
            <Field label={px.has_auth ? "已配置" : "可选用户名"}><Input value={username} onChange={e => setUsername(e.target.value)} placeholder={px.has_auth ? "已配置" : "可选用户名"} /></Field>
            <Field label="可选密码"><PasswordInput toggleLabel="代理密码" autoComplete="new-password" value={password} onChange={e => setPassword(e.target.value)} placeholder="可选密码" /></Field>
          </FormGrid>
        </FormSection>
        {px.has_auth && (
          <InsetRow label="清除已有代理认证">
            <Switch label="清除代理认证" checked={clearAuth} onChange={setClearAuth} />
          </InsetRow>
        )}
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存代理</Button>
      </FormActions>
    </Form>
  );
}

function CreateProxyDialog({open, revision, onClose, onCreated}: {open: boolean; revision: number; onClose: () => void; onCreated: (px: ProxyRow) => void}) {
  const formID = useId();
  const [backupID, setBackupID] = useState("");
  const [mode, setMode] = useState("none");
  const op = useAction();

  return (
    <Overlay open={open} title="添加代理出口" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>添加代理</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          const user = String(fd.get("username") ?? "").trim();
          const pass = String(fd.get("password") ?? "");
          void op.run(async () => {
            const created = await api("/admin/proxies", proxyRow, "POST", {
              name: String(fd.get("name") ?? "").trim(),
              protocol: String(fd.get("protocol") ?? "http"),
              host: String(fd.get("host") ?? "").trim(),
              port: Number(fd.get("port") || 8080),
              ...(user ? {username: user} : {}),
              ...(pass ? {password: pass} : {}),
              fallback_mode: mode,
              ...(mode === "proxy" && backupID ? {backup_proxy_id: Number(backupID)} : {}),
            });
            onCreated(created);
          });
        }}
      >
        <FormGrid>
          <Field label="节点名称"><Input name="name" required maxLength={100} placeholder="东京住宅出口 01" /></Field>
          <Field label="协议">
            <Select name="protocol" defaultValue="http">
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="socks5">SOCKS5</option>
              <option value="socks5h">SOCKS5H</option>
            </Select>
          </Field>
          <Field label="主机地址"><Input name="host" required maxLength={255} placeholder="127.0.0.1" /></Field>
          <Field label="端口"><Input name="port" type="number" min="1" max="65535" defaultValue="8080" required /></Field>
          <Field label="认证用户名（可选）"><Input name="username" autoComplete="off" /></Field>
          <Field label="认证密码（可选）"><PasswordInput name="password" toggleLabel="代理密码" autoComplete="new-password" /></Field>
          <Field label="故障回退">
            <Select value={mode} onChange={event => setMode(event.target.value)}>
              <option value="none">不回退</option>
              <option value="direct">直连</option>
              <option value="proxy">备用代理</option>
            </Select>
          </Field>
          {mode === "proxy" && <Field label="备用代理"><ProxySelect value={backupID} revision={revision} placeholder="无备用代理" onChange={setBackupID} /></Field>}
        </FormGrid>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

const proxyImportSchema = z.object({
  proxies: z.array(z.object({
    name: z.string().min(1).max(100),
    protocol: z.enum(["http", "https", "socks5", "socks5h"]),
    host: z.string().min(1).max(255),
    port: z.number().int().min(1).max(65535),
    username: z.string().optional(),
    password: z.string().optional(),
    has_auth: z.boolean().optional(),
  })).min(1).max(100),
});

function PasteProxiesDialog({onClose, onImported}: {onClose: () => void; onImported: (message: string) => void}) {
  const formID = useId();
  const action = useAction();
  const [text, setText] = useState("");
  return <Overlay open title="粘贴代理地址" onClose={onClose} busy={action.busy} footer={<>
    <Button variant="outline" onClick={onClose}>取消</Button>
    <Button type="submit" form={formID} busy={action.busy}>创建代理</Button>
  </>}>
    <Form id={formID} onSubmit={() => void action.run(async () => {
      const lines = [...new Set(text.split(/\r?\n/).map(line => line.trim()).filter(Boolean))];
      if (!lines.length || lines.length > 100) throw new Error("请输入 1–100 个代理地址");
      const proxies = lines.map((line, index) => {
        try {
          const url = new URL(line.includes("://") ? line : "http://" + line);
          const protocol = url.protocol.slice(0, -1);
          const host = url.hostname.replace(/^\[|\]$/g, "");
          const port = Number(url.port || (protocol === "http" ? 80 : protocol === "https" ? 443 : 0));
          if (!["http", "https", "socks5", "socks5h"].includes(protocol) || !host || !port || (url.pathname && url.pathname !== "/") || url.search || url.hash) throw new Error();
          return {name: url.host, protocol, host, port, ...(url.username ? {username: decodeURIComponent(url.username)} : {}), ...(url.password ? {password: decodeURIComponent(url.password)} : {})};
        } catch {
          throw new Error("第 " + (index + 1) + " 行代理地址格式有误");
        }
      });
      const result = await api("/admin/proxies/batch", z.object({created: z.number(), skipped: z.number()}), "POST", {proxies});
      onImported("已创建 " + result.created + " 条代理，跳过 " + result.skipped + " 条");
    })}>
      <Field label="代理地址" hint="每行一个地址，IPv6 写成 [::1]:1080。"><Textarea required rows={7} value={text} onChange={event => setText(event.target.value)} placeholder="socks5://127.0.0.1:1080" /></Field>
      <ErrorLine value={action.error} />
    </Form>
  </Overlay>;
}

function ImportProxiesDialog({open, onClose, onImported}: {open: boolean; onClose: () => void; onImported: (msg: string) => void}) {
  const formID = useId();
  const [text, setText] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const op = useAction();

  return (
    <Overlay open={open} title="批量导入代理 JSON" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>导入代理</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={() => void op.run(async () => {
          let json: unknown;
          try {
            json = JSON.parse(text);
          } catch {
            throw new Error("JSON 格式无效，需包含 1–100 条 proxies 数组。");
          }
          const parsed = proxyImportSchema.safeParse(json);
          if (!parsed.success) throw new Error("JSON 格式无效，需包含 1–100 条 proxies 数组。");
          const missing = parsed.data.proxies.filter(proxy => proxy.has_auth && (!proxy.username || proxy.password === undefined));
          if (missing.length) throw new Error("有 " + missing.length + " 条代理缺少认证字段。请补录 username 和 password 后重新导入。");
          const proxies = parsed.data.proxies.map(({has_auth, ...proxy}) => proxy);
          const res = await api("/admin/proxies/import", z.object({created: z.number(), skipped: z.number()}), "POST", {proxies});
          setText("");
          setFiles([]);
          onImported(`导入完成：新增 ${res.created} 条，跳过 ${res.skipped} 条`);
        })}
      >
        <Field label="选择 JSON 文件">
          <FilePicker
            accept="application/json,.json"
            maxBytes={5 * 1024 * 1024}
            hint="选择导出的 proxies.json 自动读取内容"
            value={files}
            onChange={picked => {
              setFiles(picked);
              const f = picked[0];
              if (f) void f.text().then(setText);
            }}
          />
        </Field>
        <Field label="代理 JSON 内容" hint="从本面板导出的配置已脱敏。有认证的代理需要补录用户名和密码。">
          <Textarea rows={6} required value={text} onChange={e => setText(e.target.value)} placeholder='{"proxies":[{"name":"JP-01","protocol":"socks5","host":"127.0.0.1","port":1080}]}' />
        </Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
