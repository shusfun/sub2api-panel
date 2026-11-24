import {useState} from "react";
import {Wrench} from "lucide-react";
import type {z} from "zod";
import {api} from "@/lib/http";
import {date, durationText} from "@/lib/format";
import {useAction} from "@/lib/requests";
import {account, empty, opsErrorRowSchema, opsRequestRowSchema} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  Badge, Button, EntityLink, ErrorLine, Feedback, InlineStack, InsetGroup, InsetRow,
  Inspector, PlatformBadge, Status, Switch, TimingBar, TraceId, type Column,
} from "@/ui";

export type OpsErrorRow = z.infer<typeof opsErrorRowSchema>;
export type OpsRequestRow = z.infer<typeof opsRequestRowSchema>;

export const opsErrorColumns: Column<OpsErrorRow>[] = [
  {id: "time", title: "时间", render: row => <span className="code-mono">{date(row.created_at)}</span>},
  {
    id: "status",
    title: "状态码 / 阶段",
    render: row => (
      <InlineStack density="compact">
        <Badge variant="soft" tone={row.status_code >= 500 ? "bad" : "warn"}>{row.status_code}</Badge>
        {row.upstream_status_code ? <span className="muted code-mono">→ {row.upstream_status_code}</span> : null}
        <span className="muted text-fine">{row.phase || row.type}</span>
      </InlineStack>
    ),
  },
  {id: "model", title: "模型", render: row => <span className="compact-model code-mono" title={row.model || row.requested_model}>{row.model || row.requested_model || "—"}</span>},
  {
    id: "entities",
    title: "关联用户 / 分组 / 上游账号",
    render: row => (
      <InlineStack density="compact" wrap>
        {row.user_id && <EntityLink kind="user" id={row.user_id} label={row.user_email || `#${row.user_id}`} />}
        {row.group_id && <EntityLink kind="group" id={row.group_id} label={row.group_name || `分组 #${row.group_id}`} />}
        {row.account_id && <EntityLink kind="account" id={row.account_id} label={row.account_name || `账号 #${row.account_id}`} />}
      </InlineStack>
    ),
  },
  {id: "trace", title: "请求编号", render: row => <TraceId value={row.request_id} />},
  {id: "state", title: "状态", align: "center", render: row => <Status value={row.resolved ? "resolved" : "error"} />},
];

export const opsRequestColumns: Column<OpsRequestRow>[] = [
  {id: "time", title: "时间", render: row => <span className="code-mono">{date(row.created_at)}</span>},
  {
    id: "kind",
    title: "状态码",
    render: row => <Badge variant="soft" tone={row.kind === "error" ? "bad" : "ok"}>{row.status_code ?? (row.kind === "error" ? "错误" : "200")}</Badge>,
  },
  {id: "platform", title: "平台 / 模型", render: row => <InlineStack density="compact"><PlatformBadge platform={row.platform} /><span className="compact-model code-mono" title={row.model}>{row.model}</span></InlineStack>},
  {
    id: "links",
    title: "路由归属",
    render: row => (
      <InlineStack density="compact" wrap>
        {row.user_id && <EntityLink kind="user" id={row.user_id} />}
        {row.group_id && <EntityLink kind="group" id={row.group_id} />}
        {row.account_id && <EntityLink kind="account" id={row.account_id} />}
      </InlineStack>
    ),
  },
  {id: "ttft", title: "首字 TTFT", align: "number", render: row => durationText(row.first_token_ms)},
  {id: "dur", title: "总耗时", align: "number", render: row => durationText(row.duration_ms)},
  {id: "req", title: "Trace", render: row => <TraceId value={row.request_id} />},
];

export function OpsErrorInspector({
  open,
  errorRow,
  errorSource,
  onClose,
  onPrev,
  onNext,
  onUpdated,
  onRefresh,
}: {
  open: boolean;
  errorRow?: OpsErrorRow;
  errorSource: "request-errors" | "upstream-errors" | "requests";
  onClose: () => void;
  onPrev?: () => void;
  onNext?: () => void;
  onUpdated: (next: OpsErrorRow) => void;
  onRefresh: () => void;
}) {
  const recovery = useAction();
  const [recoveryHint, setRecoveryHint] = useState("");

  return (
    <Inspector
      open={open}
      title={errorRow ? `${errorRow.status_code} · ${errorRow.phase || errorRow.type || "请求异常"}` : "错误诊断"}
      subtitle={errorRow ? date(errorRow.created_at) : undefined}
      badge={errorRow ? <PlatformBadge platform={errorRow.platform} /> : undefined}
      onClose={() => { setRecoveryHint(""); onClose(); }}
      onPrev={onPrev}
      onNext={onNext}
      busy={recovery.busy}
    >
      {errorRow && (
        <>
          <InsetGroup title="阶段耗时瀑布 (Instruments)">
            <InsetRow label="各阶段响应分解" vertical>
              <TimingBar
                phases={[
                  {label: "鉴权校验", ms: errorRow.auth_latency_ms, tone: "cyan"},
                  {label: "路由调度", ms: errorRow.routing_latency_ms, tone: "violet"},
                  {label: "首字 TTFT", ms: errorRow.time_to_first_token_ms, tone: "accent"},
                  {label: "上游总耗时", ms: errorRow.upstream_latency_ms ?? errorRow.response_latency_ms, tone: "warn"},
                ]}
              />
            </InsetRow>
          </InsetGroup>

          <InsetGroup title="结构化诊断摘要">
            <InsetRow label="客户端 / 上游状态码">
              <span className="weir-digits">
                HTTP {errorRow.status_code}
                {errorRow.upstream_status_code ? ` → 上游 ${errorRow.upstream_status_code}` : ""}
              </span>
            </InsetRow>
            <InsetRow label="错误归属与分类">
              <Badge variant="soft" tone={errorRow.is_business_limited ? "warn" : "bad"}>
                {errorRow.error_owner || "upstream"} · {errorRow.type || errorRow.phase}
              </Badge>
            </InsetRow>
            <InsetRow label="请求模型 → 上游模型">
              <span className="code-mono">
                {errorRow.requested_model || errorRow.model || "—"}
                {errorRow.upstream_model && errorRow.upstream_model !== errorRow.requested_model ? ` → ${errorRow.upstream_model}` : ""}
              </span>
            </InsetRow>
            <InsetRow label="请求编号"><TraceId value={errorRow.request_id} /></InsetRow>
            <InsetRow label="标记为已处理">
              <Switch
                label="标记错误已处理"
                checked={errorRow.resolved}
                disabled={recovery.busy}
                onChange={next => void recovery.run(async () => {
                  const target = errorSource === "upstream-errors" ? "upstream-errors" : "request-errors";
                  await api(`/admin/ops/${target}/${errorRow.id}/resolve`, empty, "PUT", {resolved: next});
                  onUpdated({...errorRow, resolved: next});
                })}
              />
            </InsetRow>
          </InsetGroup>

          <InsetGroup title="四维关联实体直达">
            {errorRow.user_id && <InsetRow label="调用用户"><EntityLink kind="user" id={errorRow.user_id} label={errorRow.user_email || `#${errorRow.user_id}`} /></InsetRow>}
            {errorRow.api_key_id && (
              <InsetRow label="API Key">
                <EntityLink kind="key" id={errorRow.api_key_id} label={errorRow.api_key_name || `Key #${errorRow.api_key_id}`} extra={errorRow.user_id ? {user_id: errorRow.user_id} : undefined} />
              </InsetRow>
            )}
            {errorRow.group_id && <InsetRow label="调度分组"><EntityLink kind="group" id={errorRow.group_id} label={errorRow.group_name || `分组 #${errorRow.group_id}`} /></InsetRow>}
            {errorRow.account_id && <InsetRow label="故障上游账号"><EntityLink kind="account" id={errorRow.account_id} label={errorRow.account_name || `账号 #${errorRow.account_id}`} /></InsetRow>}
          </InsetGroup>

          {errorRow.account_id && (
            <InsetGroup title="处置故障账号">
              <InsetRow label="快捷运维动作" vertical>
                <InlineStack wrap>
                  <Button size="xs" variant="outline" busy={recovery.busy} onClick={() => void recovery.run(async () => {
                    setRecoveryHint("");
                    await api(`/admin/accounts/${errorRow.account_id}/recover`, account, "POST", {});
                    setRecoveryHint("已执行一键状态恢复（清除报错与限流）");
                    onRefresh();
                  })}>
                    <Wrench aria-hidden />一键清错并恢复
                  </Button>
                  <Button size="xs" variant="outline" busy={recovery.busy} onClick={() => void recovery.run(async () => {
                    setRecoveryHint("");
                    await api(`/admin/accounts/${errorRow.account_id}/clear-rate-limit`, account, "POST", {});
                    setRecoveryHint("已清除该账号限流窗口");
                    onRefresh();
                  })}>
                    清除限流
                  </Button>
                  <Button size="xs" variant="destructive" busy={recovery.busy} onClick={() => void recovery.run(async () => {
                    setRecoveryHint("");
                    await api(`/admin/accounts/${errorRow.account_id}/scheduling`, empty, "PUT", {schedulable: false});
                    setRecoveryHint("已暂停该账号参与调度");
                    onRefresh();
                  })}>
                    暂停调度
                  </Button>
                  <Button size="xs" variant="ghost" onClick={() => navigateTo("accounts", {inspect: errorRow.account_id!})}>
                    在号池测活 →
                  </Button>
                </InlineStack>
              </InsetRow>
            </InsetGroup>
          )}
          <ErrorLine value={recovery.error} />
          {recoveryHint && <Feedback tone="success">{recoveryHint}</Feedback>}
        </>
      )}
    </Inspector>
  );
}
