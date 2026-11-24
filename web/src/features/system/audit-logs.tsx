import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date, durationText} from "@/lib/format";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {
  Badge, Button, EllipsisText, EntityLink, FilterBar, FilterSlot, InsetGroup, InsetRow, Overlay, RefreshButton, Resource,
  SearchInput, Select, StepUpDialog, TraceId, WorkspaceStage, toast, type Column,
} from "@/ui";
import {RemoteTable} from "@/ui";

const base = "/admin/audit-logs";
const entry = z.object({
  id: z.number(), created_at: z.string(), actor_user_id: z.number().optional(), actor_email: z.string(), actor_role: z.string(),
  auth_method: z.string(), action: z.string(), method: z.string(), path: z.string(), request_id: z.string(),
  client_ip: z.string(), status_code: z.number(), latency_ms: z.number(),
});
type Entry = z.infer<typeof entry>;

export function AuditLogs() {
  const [revision, revise] = useState(0);
  const [search, setSearch] = useState("");
  const [result, setResult] = useState("");
  const [method, setMethod] = useState("");
  const [clearing, clear] = useState(false);
  const [selected, select] = useState<number>();
  const keyword = useDebouncedValue(search.trim());
  const action = useAction();
  const detail = useResource(() => api(base + "/" + selected, entry), selected ?? "", selected !== undefined);
  const columns: Column<Entry>[] = [
    {id: "action", title: "操作", render: row => <EllipsisText text={row.action || row.method} />},
    {
      id: "actor",
      title: "操作者",
      hideWhenCompact: true,
      render: row =>
        row.actor_user_id ? (
          <EntityLink kind="user" id={row.actor_user_id} label={row.actor_email || `用户 #${row.actor_user_id}`} />
        ) : (
          row.actor_email || "—"
        ),
    },
    {id: "path", title: "请求路径", hideWhenCompact: true, render: row => <EllipsisText text={row.path} mono />},
    {id: "status", title: "状态码", align: "center", render: row => <Badge variant="soft" tone={row.status_code >= 400 ? "bad" : "ok"}>{row.status_code}</Badge>},
    {id: "duration", title: "耗时", align: "number", hideWhenNarrow: true, render: row => durationText(row.latency_ms)},
    {id: "time", title: "时间", hideWhenNarrow: true, render: row => date(row.created_at)},
    {id: "detail", title: "详情", align: "end", render: row => <Button variant="ghost" size="sm" onClick={() => select(row.id)}>查看</Button>},
  ];
  return (
    <WorkspaceStage flush>
      <FilterBar sticky>
        <div className="obs-filter-row">
          <SearchInput value={search} onChange={setSearch} placeholder="搜索路径、操作或操作者" />
          <FilterSlot>
            <Select aria-label="请求方法" value={method} onChange={event => setMethod(event.target.value)}>
              <option value="">全部方法</option>
              {["GET", "POST", "PUT", "PATCH", "DELETE"].map(value => <option key={value}>{value}</option>)}
            </Select>
          </FilterSlot>
          <FilterSlot>
            <Select aria-label="请求结果" value={result} onChange={event => setResult(event.target.value)}>
              <option value="">全部结果</option>
              <option value="true">成功</option>
              <option value="false">失败</option>
            </Select>
          </FilterSlot>
          <div className="obs-filter-spacer">
            <RefreshButton iconOnly size="default" variant="ghost" label="刷新审计" onClick={() => revise(v => v + 1)} />
            <Button variant="destructive" onClick={() => { action.clearError(); clear(true); }}>清空日志</Button>
          </div>
        </div>
      </FilterBar>
      <div className="swift-pane-table">
        <RemoteTable path={base + "?" + new URLSearchParams({q: keyword, method, success: result})} schema={entry} columns={columns}
          rowID={row => row.id} onRowClick={row => select(row.id)} pinActions label="操作审计" revision={revision} />
      </div>

      {selected !== undefined && (
        <Overlay open drawer title="审计详情" onClose={() => select(undefined)}>
          <Resource state={detail}>{item => item.id === selected && (
            <InsetGroup>
              <InsetRow label="操作" hint={item.action} />
              <InsetRow label="时间" hint={date(item.created_at)} />
              <InsetRow label="请求"><span className="code-mono">{item.method} {item.path}</span></InsetRow>
              <InsetRow label="操作账号">
                {item.actor_user_id ? <EntityLink kind="user" id={item.actor_user_id} label={item.actor_email || `用户 #${item.actor_user_id}`} /> : (item.actor_email || "—")}
              </InsetRow>
              <InsetRow label="认证方式" hint={item.auth_method || "—"} />
              <InsetRow label="来源 IP"><span className="code-mono">{item.client_ip || "—"}</span></InsetRow>
              <InsetRow label="状态码 / 耗时" hint={item.status_code + " / " + durationText(item.latency_ms)} />
              {item.request_id && <InsetRow label="请求追踪号"><TraceId value={item.request_id} linkToOps /></InsetRow>}
            </InsetGroup>
          )}</Resource>
        </Overlay>
      )}

      {clearing && (
        <StepUpDialog
          title="清空全站审计日志"
          description="清空操作会永久移除现有历史审计日志，官方将单独记录本次清空事件。"
          critical
          confirmLabel="确认清空"
          busy={action.busy}
          error={action.error}
          fieldErrors={action.fieldErrors}
          onClose={() => clear(false)}
          onConfirm={({totp}) =>
            void action.run(async () => {
              const response = await api(base + "/clear", z.object({deleted: z.number()}), "POST", {totp});
              clear(false);
              revise(value => value + 1);
              toast(`已清空 ${response.deleted} 条审计记录`);
            })
          }
        />
      )}
    </WorkspaceStage>
  );
}
