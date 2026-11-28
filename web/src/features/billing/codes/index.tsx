import {useMemo, useState} from "react";
import {Download, Power, Ticket, TimerOff, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {count, money} from "@/lib/format";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {empty, redeemCodeRow, type Config} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  Badge, BatchBar, Button, ConfirmDialog, CopyButton, EmptyState, EntityLink, ErrorLine, FilterBar,
  FilterSlot, InlineStack, MaskedSecret, MetricStrip, Panel, PlanIdentity, QuickPills, RefreshButton,
  RemoteTable, SearchInput, Section, Segmented, Select, Status, WorkspaceContent,
  WorkspaceStage, toast, type Column,
} from "@/ui";
import {EditCodesBatch, GenerateRedeemDialog, type CreatedCode} from "./code-dialogs";
import {IssuerAuditSection} from "./issue-panel";
import {PromoCodes} from "./promo-codes";

type RedeemRow = z.infer<typeof redeemCodeRow>;

export function CodesWorkspace({config}: {config: Config}) {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [codeType, setCodeType] = useState("");
  const [sort, setSort] = useState("");
  const [codePoolMode, setCodePoolMode] = useState<"official" | "promo" | "issues">("official");
  const [revision, revise] = useState(0);
  const [selectedCodes, setSelectedCodes] = useState<number[]>([]);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [editingCodes, editCodes] = useState(false);
  const [deletingCode, setDeletingCode] = useState<RedeemRow>();
  const [generatedCodes, setGeneratedCodes] = useState<CreatedCode[]>([]);

  const debouncedSearch = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const batchAction = useAction();

  const codeListPath = useMemo(() => {
    const p = new URLSearchParams();
    if (debouncedSearch) p.set("search", debouncedSearch);
    if (status) p.set("status", status);
    if (codeType) p.set("type", codeType);
    if (sort) {
      const [by, order] = sort.split(":");
      if (by) p.set("sort_by", by);
      if (order) p.set("sort_order", order);
    }
    const qs = p.toString();
    return "/admin/redeem-codes" + (qs ? "?" + qs : "");
  }, [codeType, debouncedSearch, sort, status]);

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
      grow: true,
      title: "兑换人",
      render: row =>
        row.used_by ? (
          <EntityLink kind="user" id={row.used_by} label={row.used_email || `用户 #${row.used_by}`} />
        ) : (
          <span className="muted">—</span>
        ),
    },
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {
      id: "ops",
      title: "",
      align: "end",
      render: row => (
        <div className="row-actions">
          {(row.status === "unused" || row.status === "active" || row.status === "disabled") && (
            <Button
              size="icon-sm"
              variant="ghost"
              busy={batchAction.busy}
              title={row.status === "disabled" ? "启用兑换码" : "停用兑换码"}
              aria-label={row.status === "disabled" ? "启用兑换码" : "停用兑换码"}
              onClick={e => {
                e.stopPropagation();
                const nextStatus = row.status === "disabled" ? "unused" : "disabled";
                void batchAction.run(async () => {
                  await api("/admin/redeem-codes/batch-update", z.object({updated: z.number()}), "POST", {
                    ids: [row.id],
                    fields: {status: nextStatus},
                  });
                  toast(nextStatus === "disabled" ? "已停用兑换码" : "已启用兑换码");
                  refresh();
                });
              }}
            >
              <Power aria-hidden />
            </Button>
          )}
          {(row.status === "unused" || row.status === "active") && (
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
          )}
          <Button
            size="icon-sm"
            variant="ghost"
            busy={batchAction.busy}
            title="删除兑换码"
            aria-label="删除兑换码"
            onClick={e => {
              e.stopPropagation();
              setDeletingCode(row);
            }}
          >
            <Trash2 aria-hidden />
          </Button>
        </div>
      ),
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
        {codePoolMode !== "promo" && codeStats?.value && (
          <MetricStrip
            flush
            items={[
              {id: "active", label: "未使用", value: count.format(codeStats.value.active_codes), tone: "ok"},
              {id: "used", label: "已使用", value: count.format(codeStats.value.used_codes), tone: "accent"},
              {id: "expired", label: "已过期", value: count.format(codeStats.value.expired_codes), tone: "warn"},
              {id: "total", label: "累计生成", value: count.format(codeStats.value.total_codes)},
            ]}
          />
        )}

        <FilterBar sticky>
          <div className="obs-filter-row">
            <Segmented
              label="卡密视图"
              value={codePoolMode}
              options={[
                ["official", "兑换码"],
                ["promo", "优惠码"],
                ["issues", "子管理员发码"],
              ] as const}
              onChange={setCodePoolMode}
            />
            {codePoolMode === "official" && (
              <>
                <SearchInput value={search} onChange={setSearch} placeholder="搜索备注或兑换码后缀" ariaLabel="搜索兑换码" />
                <FilterSlot>
                  <Select value={codeType} aria-label="兑换码类型" onChange={e => setCodeType(e.target.value)}>
                    <option value="">全部类型</option>
                    <option value="balance">余额码</option>
                    <option value="subscription">订阅码</option>
                    <option value="invitation">邀请码</option>
                    <option value="concurrency">并发码</option>
                  </Select>
                </FilterSlot>
                <QuickPills
                  label="兑换码状态"
                  value={status}
                  options={[
                    {value: "", label: "全部"},
                    {value: "unused", label: "未使用", tone: "ok"},
                    {value: "used", label: "已使用", tone: "muted"},
                    {value: "expired", label: "已过期", tone: "warn"},
                    {value: "disabled", label: "已停用", tone: "bad"},
                  ]}
                  onChange={setStatus}
                />
                <FilterSlot>
                  <Select value={sort} aria-label="排序方式" onChange={e => setSort(e.target.value)}>
                    <option value="">默认排序</option>
                    <option value="created_at:desc">最新生成</option>
                    <option value="created_at:asc">最早生成</option>
                    <option value="value:desc">面值降序</option>
                    <option value="expires_at:asc">即将到期</option>
                  </Select>
                </FilterSlot>
              </>
            )}
            <div className="obs-filter-spacer">
              {codePoolMode === "official" && (
                <Button variant="ghost" size="sm" busy={batchAction.busy} onClick={exportMasked}><Download aria-hidden />导出</Button>
              )}
              {codePoolMode !== "promo" && (
                <>
                  <RefreshButton iconOnly size="default" variant="ghost" label="刷新兑换码" onClick={refresh} />
                  <Button onClick={() => { setGeneratedCodes([]); setGenerateOpen(true); }}><Ticket aria-hidden />生成兑换码</Button>
                </>
              )}
            </div>
          </div>
          <ErrorLine value={batchAction.error} />
        </FilterBar>

        {codePoolMode === "official" && generatedCodes.length > 0 && (
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

        {codePoolMode === "promo" ? (
          <PromoCodes />
        ) : codePoolMode === "official" ? (
          <>
            {editingCodes && (
              <EditCodesBatch
                ids={selectedCodes}
                onClose={() => editCodes(false)}
                onSaved={total => {
                  editCodes(false);
                  setSelectedCodes([]);
                  toast("已更新 " + total + " 张兑换码");
                  refresh();
                }}
              />
            )}
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
      <ConfirmDialog
        open={!!deletingCode}
        title="删除兑换码"
        description={deletingCode ? `确定删除兑换码 ${deletingCode.masked} 吗？` : ""}
        confirm="删除"
        busy={batchAction.busy}
        error={batchAction.error}
        onClose={() => setDeletingCode(undefined)}
        onConfirm={() => deletingCode && void batchAction.run(async () => {
          await api(`/admin/redeem-codes/${deletingCode.id}`, empty, "DELETE");
          setDeletingCode(undefined);
          toast("兑换码已删除");
          refresh();
        })}
      />
    </>
  );
}
