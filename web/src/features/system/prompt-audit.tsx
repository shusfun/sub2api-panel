import {useMemo, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date} from "@/lib/format";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {
  BatchBar, Badge, Button, ConfirmDialog, EllipsisText, EntityLink, FilterSlot, RefreshButton, RemoteTable, SearchInput, Select, toast,
  type Column,
} from "@/ui";
import {PromptAuditConfig} from "./prompt-audit-config";

const base = "/admin/prompt-audit";
// 事件只含哈希、长度、计数和判定结果，不含提示词原文与扫描证据。
const eventRow = z.object({
  id: z.number(),
  snapshot: z.object({
    request_id: z.string(), user_id: z.number(), user_email: z.string(), api_key_name: z.string(), group_name: z.string(),
    endpoint: z.string(), model: z.string(), prompt_hash: z.string(), prompt_length: z.number(), message_count: z.number(), stage: z.string(),
  }),
  decision: z.string(), risk_level: z.string(), action: z.string(), categories: z.array(z.string()),
  matched_scanners: z.array(z.string()), latency_ms: z.number(), created_at: z.string(),
});
const deleted = z.object({deleted_events: z.number(), deleted_jobs: z.number()});
const preview = z.object({
  matched_count: z.number(), snapshot_max_id: z.number(), filter_hash: z.string(), confirmation_token: z.string().optional(), expires_at: z.string(),
});
type Event = z.infer<typeof eventRow>;

const riskTone = (level: string) => level === "high" || level === "critical" ? "bad" : level === "medium" ? "warn" : "neutral";

export function PromptAudit() {
  const [revision, revise] = useState(0);
  const [search, setSearch] = useState("");
  const [decision, setDecision] = useState("");
  const [risk, setRisk] = useState("");
  const [selected, setSelected] = useState<number[]>([]);
  const [confirm, setConfirm] = useState<"batch" | "filter">();
  const [ticket, setTicket] = useState<z.infer<typeof preview>>();
  const keyword = useDebouncedValue(search.trim(), 240);
  const action = useAction();
  const refresh = () => revise(v => v + 1);

  const filter = useMemo(() => ({
    ...(decision ? {decision} : {}), ...(risk ? {risk_level: risk} : {}), ...(keyword ? {keyword} : {}),
  }), [decision, risk, keyword]);
  const path = base + "/events" + (Object.keys(filter).length ? "?" + new URLSearchParams(filter) : "");

  const columns: Column<Event>[] = [
    {id: "user", title: "用户", render: row => row.snapshot.user_id ? <EntityLink kind="user" id={row.snapshot.user_id} label={row.snapshot.user_email || `用户 #${row.snapshot.user_id}`} /> : "—"},
    {id: "model", title: "分组 / 模型", hideWhenCompact: true, render: row => <EllipsisText text={[row.snapshot.group_name, row.snapshot.model].filter(Boolean).join(" / ") || "—"} />},
    {id: "hash", title: "提示词哈希 / 长度", hideWhenNarrow: true, render: row => <span className="code-mono text-fine">{row.snapshot.prompt_hash.slice(0, 12) || "—"} · {row.snapshot.prompt_length} 字符 · {row.snapshot.message_count} 条</span>},
    {id: "cats", title: "命中分类", hideWhenCompact: true, render: row => row.categories.length ? <EllipsisText text={row.categories.join("、")} /> : "—"},
    {id: "risk", title: "风险", align: "center", render: row => <Badge variant="soft" tone={riskTone(row.risk_level)}>{row.risk_level || "—"}</Badge>},
    {id: "decision", title: "判定 / 动作", align: "center", render: row => `${row.decision || "—"} / ${row.action || "—"}`},
    {id: "time", title: "时间", hideWhenNarrow: true, render: row => date(row.created_at)},
  ];

  const startFilterDelete = () => void action.run(async () => {
    setTicket(await api(base + "/events/delete-preview", preview, "POST", filter));
    setConfirm("filter");
  });
  const close = () => { setConfirm(undefined); setTicket(undefined); action.clearError(); };

  return (
    <>
      <PromptAuditConfig revision={revision} onSaved={refresh} />
      <section className="swift-group" aria-label="审计事件">
        <div className="swift-group-head">
          <span className="swift-group-title">审计事件</span>
          <div className="swift-group-actions">
            <SearchInput value={search} onChange={setSearch} placeholder="搜索请求号、哈希或关键词" />
            <FilterSlot>
              <Select value={decision} aria-label="按判定筛选" onChange={e => setDecision(e.target.value)}>
                <option value="">全部判定</option><option value="pass">放行</option><option value="flagged">命中</option><option value="blocked">拦截</option>
              </Select>
            </FilterSlot>
            <FilterSlot>
              <Select value={risk} aria-label="按风险筛选" onChange={e => setRisk(e.target.value)}>
                <option value="">全部风险</option><option value="low">低</option><option value="medium">中</option><option value="high">高</option>
              </Select>
            </FilterSlot>
            <Button size="xs" variant="outline" busy={action.busy} onClick={startFilterDelete}>按当前筛选清理</Button>
            <RefreshButton size="xs" variant="ghost" onClick={refresh} />
          </div>
        </div>
        <BatchBar count={selected.length} onClear={() => setSelected([])}>
          <Button size="xs" variant="destructive" onClick={() => setConfirm("batch")}>删除所选事件</Button>
        </BatchBar>
        <RemoteTable key={path} path={path} schema={eventRow} columns={columns} rowID={row => row.id} label="提示词审计事件" revision={revision} selection={{selected, onChange: setSelected}} />
      </section>

      <ConfirmDialog
        open={confirm === "batch"}
        title="删除审计事件"
        description={`确定删除所选 ${selected.length} 条审计事件吗？`}
        confirm="删除"
        critical
        busy={action.busy}
        error={action.error}
        onClose={close}
        onConfirm={() => void action.run(async () => {
          const res = await api(base + "/events/batch-delete", deleted, "POST", {ids: selected});
          close(); setSelected([]); toast(`已删除 ${res.deleted_events} 条事件`); refresh();
        })}
      />
      <ConfirmDialog
        open={confirm === "filter" && !!ticket}
        title="按筛选清理审计事件"
        description={`当前筛选共匹配 ${ticket?.matched_count ?? 0} 条事件，确认后将全部删除，无法恢复。`}
        confirm="确认清理"
        critical
        busy={action.busy}
        error={action.error}
        onClose={close}
        onConfirm={() => void action.run(async () => {
          if (!ticket?.confirmation_token) throw new Error("预览已失效，请重新发起清理");
          const res = await api(base + "/events/delete-by-filter", deleted, "POST", {
            filter, snapshot_max_id: ticket.snapshot_max_id, filter_hash: ticket.filter_hash, confirmation_token: ticket.confirmation_token, confirm: true,
          });
          close(); toast(`已清理 ${res.deleted_events} 条事件`); refresh();
        })}
      />
    </>
  );
}
