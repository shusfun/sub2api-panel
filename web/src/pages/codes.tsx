import { useId,useMemo,useRef,useState } from "react";
import { Download,Ticket,TimerOff,Trash2 } from "lucide-react";
import { z } from "zod";
import { api,ApiError,createdRedeemCode,empty,issue as issueSchema,redeemCodeRow,type Config } from "../api";
import { cents,count,date,money } from "../format";
import { useAction,useDebouncedValue,useResource } from "../requests";
import { navigateTo } from "../route";
import {
InlineStack, Section,
WorkspaceContent,
Badge,
BatchBar,
Button,
ConfirmDialog,
CopyButton,
CopyValue,
EmptyState,
EntityLink,
ErrorLine,
Feedback,
Field,
FilterBar,
Form,
FormGrid,
GroupSelect,
Input,
InsetRow,
MaskedSecret,
MetricStrip,
Overlay,
Panel,
PlanIdentity,
QuickPills,
RefreshButton,
SearchInput,
Segmented,
Select,
Status,
Switch,
Toolbar,
UserSelect,
WorkspaceStage,
toast,
type Column,
} from "../ui";
import { RemoteTable } from "./remote-table";

type RedeemRow = z.infer<typeof redeemCodeRow>;
type CreatedCode = z.infer<typeof createdRedeemCode>;
type IssueRow = z.infer<typeof issueSchema>;

export function CodesWorkspace({config}: {config: Config}) {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [codeType, setCodeType] = useState("");
  const [codePoolMode, setCodePoolMode] = useState<"official" | "issues">("official");
  const [revision, revise] = useState(0);
  const [selectedCodes, setSelectedCodes] = useState<number[]>([]);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [editingCodes, editCodes] = useState(false);
  const [generatedCodes, setGeneratedCodes] = useState<CreatedCode[]>([]);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const batchAction = useAction();

  const codeListPath = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (status) p.set("status", status);
    if (codeType) p.set("type", codeType);
    const qs = p.toString();
    return "/admin/redeem-codes" + (qs ? "?" + qs : "");
  }, [codeType, debouncedSearch, status]);

  const exportMasked = () => void batchAction.run(async () => {
    const rows = await api(codeListPath.replace("redeem-codes", "redeem-codes/export"), z.array(redeemCodeRow));
    const entries = [["编号", "兑换码", "类型", "状态", "到期时间"], ...rows.map(row => [row.id, row.masked, row.type, row.status, row.expires_at])];
    const cell = (value: unknown) => {
      const text = String(value ?? "");
      return '"' + (/^[=+@\-]/.test(text) ? "'" : "") + text.replaceAll('"', '""') + '"';
    };
    const uri = URL.createObjectURL(new Blob(["\ufeff", entries.map(row => row.map(cell).join(",")).join("\r\n")], {type: "text/csv;charset=utf-8"}));
    const link = document.createElement("a");
    link.href = uri;
    link.download = "redeem-history.csv";
    link.click();
    setTimeout(() => URL.revokeObjectURL(uri), 1000);
    toast("已导出 " + rows.length + " 条脱敏兑换记录");
  });

  const codeStats = useResource(
    () => api("/admin/redeem-codes/stats", z.object({total_codes: z.number(), active_codes: z.number(), used_codes: z.number(), expired_codes: z.number()})),
    "redeem-stats",
    true,
    revision,
  );

  const codeCols: Column<RedeemRow>[] = [
    {id: "code", title: "兑换码", render: row => <MaskedSecret masked={row.masked} />},
    {
      id: "type",
      title: "类型 / 面值",
      render: row => {
        const typeNames: Record<string, string> = {balance: "余额码", subscription: "订阅码", concurrency: "并发码", invitation: "邀请码"};
        return (
          <InlineStack density="compact" className="table-label-value">
            <Badge variant="soft" tone="neutral">{typeNames[row.type] ?? row.type}</Badge>
            <strong className="weir-digits">
              {row.type === "balance" ? money(row.value) : row.type === "subscription" ? `${row.validity_days} 天` : row.type === "concurrency" ? `+${row.value}` : "注册资格"}
            </strong>
          </InlineStack>
        );
      },
    },
    {
      id: "group",
      hideWhenNarrow: true,
      title: "关联订阅组",
      render: row => row.group_id ? <PlanIdentity item={{id: row.group_id, name: row.group_name || `分组 #${row.group_id}`}} /> : <span className="muted">—</span>,
    },
    {
      id: "usedBy",
      hideWhenNarrow: true,
      title: "兑换人",
      render: row =>
        row.used_by ? (
          <EntityLink
            kind="user"
            id={row.used_by}
            label={row.used_email || `用户 #${row.used_by}`}
          />
        ) : (
          <span className="muted">未使用</span>
        ),
    },
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {
      id: "ops",
      title: "",
      align: "end",
      render: row =>
        row.status === "unused" || row.status === "active" ? (
          <Button
            size="icon-sm"
            variant="ghost"
            busy={batchAction.busy}
            title="设为过期"
            aria-label="设为过期"
            onClick={e => {
              e.stopPropagation();
              void batchAction.run(async () => {
                await api(`/admin/redeem-codes/${row.id}/expire`, redeemCodeRow, "POST", {});
                refresh();
              });
            }}
          >
            <TimerOff aria-hidden />
          </Button>
        ) : null,
    },
  ];

  function downloadGenerated(ext: "txt" | "csv") {
    const content = ext === "txt"
      ? generatedCodes.map(c => c.code).join("\n")
      : ["id,code,type,value,group_id,validity_days", ...generatedCodes.map(c => `${c.id},${c.code},${c.type},${c.value},${c.group_id ?? ""},${c.validity_days ?? ""}`)].join("\n");
    const blob = new Blob([content], {type: "text/plain;charset=utf-8"});
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `redeem-codes.${ext}`;
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <>
      <WorkspaceStage flush>
        <FilterBar sticky>
          <div className="obs-filter-row">
            {codePoolMode === "official" && <>
              <SearchInput value={search} onChange={setSearch} placeholder="搜索备注或兑换码后缀" ariaLabel="搜索兑换码" />
              <QuickPills
                label="兑换码类型"
                value={codeType}
                options={[
                  {value: "", label: "全部类型"},
                  {value: "balance", label: "余额码"},
                  {value: "subscription", label: "订阅码"},
                  {value: "invitation", label: "邀请码"},
                  {value: "concurrency", label: "并发码"},
                ]}
                onChange={setCodeType}
              />
              <QuickPills
                label="兑换码状态"
                value={status}
                options={[
                  {value: "", label: "全部"},
                  {value: "unused", label: "可用", tone: "ok"},
                  {value: "used", label: "已用"},
                  {value: "expired", label: "已过期", tone: "warn"},
                ]}
                onChange={setStatus}
              />
            </>}
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新兑换码" onClick={refresh} />
              <Button onClick={() => { setGeneratedCodes([]); setGenerateOpen(true); }}><Ticket aria-hidden />生成兑换码</Button>
            </div>
          </div>
          <ErrorLine value={batchAction.error} />
        </FilterBar>
            {codeStats?.value && (
              <MetricStrip
                flush
                items={[
                  {id: "active", label: "可用", value: count.format(codeStats.value.active_codes), tone: "ok"},
                  {id: "used", label: "已兑换", value: count.format(codeStats.value.used_codes), tone: "accent"},
                  {id: "expired", label: "已过期", value: count.format(codeStats.value.expired_codes), tone: "warn"},
                  {id: "total", label: "累计生成", value: count.format(codeStats.value.total_codes)},
                ]}
                trailing={
                  <Segmented
                    label="兑换码来源"
                    value={codePoolMode}
                    options={[
                      ["official", "全部兑换码"],
                      ["issues", "子管理员发码"],
                    ] as const}
                    onChange={setCodePoolMode}
                  />
                }
              />
            )}

            {generatedCodes.length > 0 && (
              <Section className="section-divided">
                <Panel
                  title={`已生成 ${generatedCodes.length} 张兑换码`}
                  subtitle="明文只显示这一次"
                  actions={
                    <>
                      <CopyButton
                        variant="outline"
                        size="xs"
                        label="复制全部兑换码"
                        value={generatedCodes.map(c => c.code).join("\n")}
                        onCopied={() => toast("已复制全部兑换码")}
                      >
                        复制全部
                      </CopyButton>
                      <Button size="xs" variant="outline" onClick={() => downloadGenerated("txt")}><Download aria-hidden />TXT</Button>
                      <Button size="xs" variant="outline" onClick={() => downloadGenerated("csv")}><Download aria-hidden />CSV</Button>
                    </>
                  }
                >
                  <div className="code-issue-list code-mono">
                    {generatedCodes.map(c => <div key={c.id}>{c.code}</div>)}
                  </div>
                </Panel>
              </Section>
            )}

            {codePoolMode === "official" ? (
              <>
                {editingCodes && <EditCodesBatch ids={selectedCodes} onClose={() => editCodes(false)} onSaved={total => {
                  editCodes(false);
                  setSelectedCodes([]);
                  toast("已更新 " + total + " 张兑换码");
                  refresh();
                }} />}
                <Toolbar><Button variant="outline" size="sm" busy={batchAction.busy} onClick={exportMasked}><Download aria-hidden />导出筛选结果</Button></Toolbar>
                <BatchBar count={selectedCodes.length} onClear={() => setSelectedCodes([])}>
                  <Button size="xs" variant="outline" onClick={() => editCodes(true)}>批量编辑</Button>
                  <Button
                    size="xs"
                    variant="destructive"
                    busy={batchAction.busy}
                    onClick={() => void batchAction.run(async () => {
                      await api("/admin/redeem-codes/batch-delete", z.object({deleted: z.number()}), "POST", {ids: selectedCodes});
                      setSelectedCodes([]);
                      toast("已删除所选兑换码");
                      refresh();
                    })}
                  >
                    <Trash2 aria-hidden />批量删除
                  </Button>
                </BatchBar>

                <div className="swift-pane-table">
                  <RemoteTable
                    key={codeListPath}
                    path={codeListPath}
                    schema={redeemCodeRow}
                    columns={codeCols}
                    rowID={row => row.id}
                    label="兑换码"
                    pinActions
                    revision={revision}
                    selection={{selected: selectedCodes, onChange: setSelectedCodes}}
                    empty={
                      <EmptyState
                        title="没有符合条件的兑换码"
                        action={<Button size="sm" onClick={() => setGenerateOpen(true)}><Ticket aria-hidden />生成兑换码</Button>}
                      />
                    }
                  />
                </div>
              </>
            ) : (
              <WorkspaceContent>
                <IssuerAuditSection config={config} revision={revision} onChanged={refresh} />
              </WorkspaceContent>
            )}
      </WorkspaceStage>

      <GenerateRedeemDialog
        open={generateOpen}
        revision={revision}
        onClose={() => setGenerateOpen(false)}
        onGenerated={codes => {
          setGenerateOpen(false);
          setGeneratedCodes(codes);
          refresh();
        }}
        onDirectRedeemed={uid => {
          setGenerateOpen(false);
          refresh();
          navigateTo("users", {inspect: uid});
        }}
      />
    </>
  );
}

function EditCodesBatch({ids, onClose, onSaved}: {ids: number[]; onClose: () => void; onSaved: (total: number) => void}) {
  const formID = useId();
  const action = useAction();
  const [expiry, setExpiry] = useState("");
  const [group, setGroup] = useState("");
  const [editGroup, setEditGroup] = useState(false);
  const [editNotes, setEditNotes] = useState(false);
  return <Overlay open title="批量编辑兑换码" onClose={onClose} busy={action.busy} footer={<>
    <Button variant="outline" onClick={onClose}>取消</Button>
    <Button type="submit" form={formID} busy={action.busy}>保存 {ids.length} 张兑换码</Button>
  </>}>
    <Form id={formID} onSubmit={event => {
      const values = new FormData(event.currentTarget);
      void action.run(async () => {
        const fields: Record<string, unknown> = {};
        if (values.get("status")) fields.status = values.get("status");
        if (expiry === "clear") fields.expires_at = null;
        if (expiry === "set") {
          const date = new Date(String(values.get("expires_at") ?? ""));
          if (!Number.isFinite(date.getTime())) throw new Error("请输入有效的到期时间");
          fields.expires_at = date.toISOString();
        }
        if (editGroup) fields.group_id = group ? Number(group) : null;
        if (editNotes) fields.notes = String(values.get("notes") ?? "");
        if (!Object.keys(fields).length) throw new Error("请选择要修改的字段");
        const result = await api("/admin/redeem-codes/batch-update", z.object({updated: z.number()}), "POST", {ids, fields});
        onSaved(result.updated);
      });
    }}>
      <Field label="状态"><Select name="status"><option value="">保持原值</option><option value="unused">启用</option><option value="disabled">停用</option></Select></Field>
      <Field label="到期时间"><Select value={expiry} onChange={event => setExpiry(event.target.value)}><option value="">保持原值</option><option value="set">设置时间</option><option value="clear">清除</option></Select></Field>
      {expiry === "set" && <Field label="到期于"><Input type="datetime-local" name="expires_at" required /></Field>}
      <InsetRow label="更改订阅分组"><Switch label="更改订阅分组" checked={editGroup} onChange={setEditGroup} /></InsetRow>
      {editGroup && <Field label="订阅分组"><GroupSelect subscriptionOnly value={group} onChange={setGroup} placeholder="清除分组" /></Field>}
      <InsetRow label="更改备注"><Switch label="更改备注" checked={editNotes} onChange={setEditNotes} /></InsetRow>
      {editNotes && <Field label="新备注"><Input name="notes" maxLength={255} /></Field>}
      <ErrorLine value={action.error} />
    </Form>
  </Overlay>;
}

function IssuerAuditSection({
  config,
  revision,
  onChanged,
}: {
  config: Config;
  revision: number;
  onChanged: () => void;
}) {
  const formID = useId();
  const [kind, setKind] = useState<"invitation" | "balance">(config.invitation ? "invitation" : "balance");
  const [value, setValue] = useState("10.00");
  const [secret, setSecret] = useState("");
  const [resolving, setResolving] = useState<IssueRow>();
  const [resolveCode, setResolveCode] = useState("");
  const [refundOpen, setRefundOpen] = useState(false);
  const pending = useRef<{id: string; input: string} | null>(null);
  const op = useAction();
  const reconcile = useAction();
  const refund = useAction();

  const cols: Column<IssueRow>[] = [
    {id: "time", title: "时间", render: r => <span className="code-mono">{date(r.created_at * 1000)}</span>},
    {
      id: "actor",
      title: "签发人",
      render: r => r.actor_id ? <EntityLink kind="user" id={r.actor_id} label={`#${r.actor_id}`} /> : "—",
    },
    {id: "kind", title: "类型", render: r => <Badge variant="tag">{r.kind === "balance" ? "余额码" : "邀请码"}</Badge>},
    {id: "amt", title: "面额", align: "number", render: r => r.amount_cents ? money(r.amount_cents / 100) : "—"},
    {id: "state", title: "状态", align: "center", render: r => <Status value={r.state} />},
    {id: "cid", title: "官方码 ID", align: "number", render: r => r.code_id ?? "—"},
    {
      id: "act",
      title: "核对",
      align: "end",
      render: r =>
        r.state === "pending" || r.state === "unknown" ? (
          <Button size="xs" variant="outline" onClick={() => { reconcile.clearError(); setResolveCode(""); setResolving(r); }}>核对结果</Button>
        ) : r.request_id ? (
          <CopyButton value={r.request_id} label="复制请求编号" size="icon-xs" />
        ) : null,
    },
  ];

  return (
    <>
      <Panel title="签发一张">
        <Form
          busy={op.busy}
          onSubmit={() => void op.run(async () => {
            setSecret("");
            const amount = kind === "invitation" ? 0 : cents(value);
            const fp = `${kind}/${amount}`;
            const opID = pending.current?.input === fp ? pending.current.id : crypto.randomUUID();
            pending.current = {id: opID, input: fp};
            try {
              const res = await api("/issues", z.object({code: z.string()}), "POST", {operation_id: opID, kind, amount_cents: amount});
              setSecret(res.code);
              pending.current = null;
            } catch (err) {
              if (err instanceof ApiError && ["issue_rejected", "grant_exhausted", "invalid_input", "not_allowed"].includes(err.code)) {
                pending.current = null;
              }
              throw err;
            } finally {
              onChanged();
            }
          })}
        >
          <Toolbar>
            <Field label="码种">
              <Select value={kind} onChange={e => setKind(e.target.value as "invitation" | "balance")}>
                {config.invitation && <option value="invitation">注册邀请码</option>}
                <option value="balance">余额兑换码</option>
              </Select>
            </Field>
            {kind === "balance" && (
              <Field label="面额 (USD)">
                <Input inputMode="decimal" required value={value} onChange={e => setValue(e.target.value)} />
              </Field>
            )}
            <Button type="submit" busy={op.busy}><Ticket aria-hidden />签发一张</Button>
          </Toolbar>
          <ErrorLine value={op.error} />
        </Form>
        {secret && <CopyValue label="新签发的码" value={secret} />}
      </Panel>

      <Panel title="发码记录" padded={false}>
        <RemoteTable path="/issues" schema={issueSchema} columns={cols} rowID={r => r.operation_id} label="发码记录" revision={revision} />
      </Panel>

      <Overlay open={!!resolving} title="核对异常发码记录" onClose={() => setResolving(undefined)} busy={reconcile.busy} footer={
        <>
          <Button variant="outline" onClick={() => setResolving(undefined)}>取消</Button>
          <Button type="submit" form={formID} disabled={!resolveCode} busy={reconcile.busy}>确认已生成</Button>
          <Button variant="destructive" disabled={reconcile.busy} onClick={() => { refund.clearError(); setRefundOpen(true); }}>
            确认未生成并退额度
          </Button>
        </>
      }>
        {resolving && (
          <Form id={formID}
            busy={reconcile.busy}
            onSubmit={() => void reconcile.run(async () => {
              await api(`/admin/issues/${resolving.operation_id}/resolve`, empty, "POST", {code_id: Number(resolveCode)});
              setResolving(undefined);
              onChanged();
            })}
          >
            {resolving.request_id && <CopyValue label="关联请求编号" value={resolving.request_id} />}
            <Field label="已确认生成的官方码 ID">
              <Input type="number" min="1" value={resolveCode} onChange={e => setResolveCode(e.target.value)} />
            </Field>
            <ErrorLine value={reconcile.error} />
          </Form>
        )}
      </Overlay>

      <ConfirmDialog
        open={refundOpen}
        title="退回预扣发码额度"
        description="确认官方没有生成这张码后，预扣的额度会退回给签发人。"
        confirm="确认退回"
        busy={refund.busy}
        error={refund.error}
        onClose={() => setRefundOpen(false)}
        onConfirm={() => {
          if (!resolving) return;
          void refund.run(async () => {
            await api(`/admin/issues/${resolving.operation_id}/resolve`, empty, "POST", {code_id: null});
            setRefundOpen(false);
            setResolving(undefined);
            onChanged();
          });
        }}
      />
    </>
  );
}

function GenerateRedeemDialog({
  open,
  revision,
  onClose,
  onGenerated,
  onDirectRedeemed,
}: {
  open: boolean;
  revision: number;
  onClose: () => void;
  onGenerated: (codes: CreatedCode[]) => void;
  onDirectRedeemed: (userID: number) => void;
}) {
  const formID = useId();
  const [mode, setMode] = useState<"batch" | "direct">("batch");
  const [kind, setKind] = useState<"balance" | "subscription" | "concurrency" | "invitation">("balance");
  const [groupID, setGroupID] = useState("");
  const [targetUser, setTargetUser] = useState("");
  const op = useAction();

  return (
    <Overlay open={open} title="生成兑换码" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} disabled={(mode === "direct" && !targetUser) || (kind === "subscription" && !groupID)} busy={op.busy}>
          {mode === "direct" ? "充值" : "生成"}
        </Button>
      </>
    }>
      <Segmented
        label="发卡模式"
        value={mode}
        options={[
          ["batch", "批量生成"],
          ["direct", "直接充给用户"],
        ] as const}
        onChange={next => {
          setMode(next);
          if (next === "direct" && kind === "invitation") setKind("balance");
        }}
      />
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            if (mode === "direct") {
              const uid = Number(targetUser);
              await api("/admin/redeem-codes/create-and-redeem", empty, "POST", {
                user_id: uid,
                type: kind,
                value: kind === "subscription" ? 1 : Number(fd.get("value") || 10),
                ...(kind === "subscription" ? {group_id: Number(groupID), validity_days: Number(fd.get("validity_days") || 30)} : {}),
                notes: String(fd.get("notes") ?? "").trim(),
                operation_id: crypto.randomUUID(),
              });
              onDirectRedeemed(uid);
              return;
            }
            const res = await api("/admin/redeem-codes/generate", z.object({codes: z.array(createdRedeemCode)}), "POST", {
              count: Number(fd.get("count") || 1),
              type: kind,
              value: kind === "invitation" ? 0 : kind === "subscription" ? 1 : Number(fd.get("value") || 10),
              ...(kind === "subscription" ? {group_id: Number(groupID), validity_days: Number(fd.get("validity_days") || 30)} : {}),
              expires_in_days: Number(fd.get("expires_in_days") || 30),
              operation_id: crypto.randomUUID(),
            });
            onGenerated(res.codes);
          });
        }}
      >
        {mode === "direct" && (
          <Field label="目标用户"><UserSelect value={targetUser} clearable={false} placeholder="搜索用户邮箱…" onChange={v => setTargetUser(v)} /></Field>
        )}
        <FormGrid>
          <Field label="类型">
            <Select value={kind} onChange={e => setKind(e.target.value as "balance" | "subscription" | "concurrency" | "invitation")}>
              <option value="balance">余额充值码 (USD)</option>
              <option value="subscription">套餐订阅码</option>
              <option value="concurrency">并发扩容码</option>
              {mode === "batch" && <option value="invitation">注册邀请码</option>}
            </Select>
          </Field>
          {mode === "batch" && (
            <Field label="生成张数 (1–100)"><Input name="count" type="number" min="1" max="100" defaultValue="5" required /></Field>
          )}
          {(kind === "balance" || kind === "concurrency") && (
            <Field label={kind === "balance" ? "单张金额 (USD)" : "增加并发数"}>
              <Input name="value" type="number" min="0.01" step={kind === "balance" ? "0.01" : "1"} defaultValue="10" required />
            </Field>
          )}
          {mode === "batch" && (
            <Field label="兑换码有效天数"><Input name="expires_in_days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
          )}
        </FormGrid>
        {kind === "subscription" && (
          <FormGrid>
            <Field label="订阅分组"><GroupSelect value={groupID} subscriptionOnly revision={revision} clearable={false} placeholder="选择订阅组…" onChange={setGroupID} /></Field>
            <Field label="开通有效天数"><Input name="validity_days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
          </FormGrid>
        )}
        {mode === "direct" && <Field label="备注（选填）"><Input name="notes" maxLength={200} /></Field>}
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
