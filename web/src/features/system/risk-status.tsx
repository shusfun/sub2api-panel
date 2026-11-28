import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {count, date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {
  Badge, Button, Field, Input, InsetGroup, InsetRow, RefreshButton, Resource, StepUpDialog, Toolbar, toast, useTotpStatus,
} from "@/ui";

const base = "/admin/risk-control";
const keyStatus = z.object({
  index: z.number(), key_hash: z.string(), masked: z.string(), status: z.string(), failure_count: z.number(),
  success_count: z.number(), last_checked_at: z.string().nullish(), frozen_until: z.string().nullish(),
  last_latency_ms: z.number(), last_http_status: z.number(),
});
const statusSchema = z.object({
  engine: z.string(), enabled: z.boolean(), risk_control_enabled: z.boolean(), mode: z.string(), worker_count: z.number(),
  active_workers: z.number(), queue_size: z.number(), queue_length: z.number(), queue_usage_percent: z.number(),
  dropped: z.number(), processed: z.number(), errors: z.number(), pre_block_checked: z.number(), pre_block_blocked: z.number(),
  pre_block_errors: z.number(), pre_block_avg_latency_ms: z.number(), flagged_hash_count: z.number(),
  last_cleanup_at: z.string().nullish(), api_key_statuses: z.array(keyStatus),
});
const testSchema = z.object({
  items: z.array(keyStatus),
  audit_result: z.object({
    flagged: z.boolean(), highest_category: z.string(), highest_score: z.number(), composite_score: z.number(),
    category_scores: z.record(z.string(), z.number()), thresholds: z.record(z.string(), z.number()),
  }).nullish(),
});
const hashResult = z.object({deleted: z.union([z.boolean(), z.number()])});

type KeyStatus = z.infer<typeof keyStatus>;
type Test = z.infer<typeof testSchema>;

export function RiskStatusPanel() {
  const [revision, revise] = useState(0);
  const [tested, setTested] = useState<Test>();
  const [hashDialog, setHashDialog] = useState<"one" | "all">();
  const [hash, setHash] = useState("");
  const status = useResource(() => api(base + "/status", statusSchema), "risk-status", true, revision);
  const totp = useTotpStatus();
  const test = useAction();
  const clear = useAction();

  const runTest = () => void test.run(async () => {
    setTested(await api(base + "/api-keys/test", testSchema, "POST", {}));
    toast("密钥测试完成");
    revise(v => v + 1);
  });

  const closeHash = () => { setHashDialog(undefined); setHash(""); clear.clearError(); };

  return (
    <Resource state={status}>
      {view => (
        <>
          <InsetGroup title="运行状态">
            <InsetRow label="引擎 / 模式">{view.engine || "—"} · {view.mode || "—"}</InsetRow>
            <InsetRow label="审核线程">{view.active_workers} / {view.worker_count} 活跃</InsetRow>
            <InsetRow label="队列">{view.queue_length} / {view.queue_size}（{view.queue_usage_percent.toFixed(0)}%）</InsetRow>
            <InsetRow label="已处理 / 出错 / 丢弃">{count.format(view.processed)} / {count.format(view.errors)} / {count.format(view.dropped)}</InsetRow>
            <InsetRow label="请求前拦截">检查 {count.format(view.pre_block_checked)} · 拦截 {count.format(view.pre_block_blocked)} · 出错 {count.format(view.pre_block_errors)} · 平均 {view.pre_block_avg_latency_ms} ms</InsetRow>
            <InsetRow label="已标记哈希">{count.format(view.flagged_hash_count)}</InsetRow>
            <InsetRow label="上次清理">{view.last_cleanup_at ? date(view.last_cleanup_at) : "—"}</InsetRow>
          </InsetGroup>

          <InsetGroup title="审核密钥">
            {view.api_key_statuses.length === 0 && <InsetRow label="尚未配置审核密钥" />}
            {view.api_key_statuses.map(key => <KeyRow key={key.key_hash || key.index} item={key} />)}
          </InsetGroup>

          {tested && (
            <InsetGroup title="最近一次测试">
              {tested.items.map(key => <KeyRow key={"t-" + (key.key_hash || key.index)} item={key} />)}
              {tested.audit_result && (
                <InsetRow label="样例审核结果">
                  {tested.audit_result.flagged ? "命中" : "未命中"} · {tested.audit_result.highest_category || "—"} · 最高分 {tested.audit_result.highest_score.toFixed(3)}
                </InsetRow>
              )}
            </InsetGroup>
          )}

          <Toolbar>
            <Button variant="outline" busy={test.busy} onClick={runTest}>测试已配置密钥</Button>
            <Button variant="outline" onClick={() => setHashDialog("one")}>删除单个哈希</Button>
            <Button variant="destructive" onClick={() => setHashDialog("all")}>清空全部哈希</Button>
            <RefreshButton iconOnly size="default" variant="ghost" label="刷新风控状态" onClick={() => revise(v => v + 1)} />
          </Toolbar>
          {test.error && <p className="muted">{test.error}</p>}

          {hashDialog && (
            <StepUpDialog
              title={hashDialog === "all" ? "清空全部已标记哈希" : "删除单个已标记哈希"}
              description={hashDialog === "all" ? "清空后，此前被标记的内容会重新参与审核。" : undefined}
              requireTotp={totp?.value?.enabled === true}
              critical={hashDialog === "all"}
              confirmLabel={hashDialog === "all" ? "确认清空" : "确认删除"}
              busy={clear.busy}
              error={clear.error}
              fieldErrors={clear.fieldErrors}
              onClose={closeHash}
              onConfirm={({totp: code}) => void clear.run(async () => {
                const auth = code ? {totp: code} : {};
                if (hashDialog === "all") await api(base + "/hashes/all", hashResult, "DELETE", auth);
                else await api(base + "/hashes", hashResult, "DELETE", {input_hash: hash.trim(), ...auth});
                closeHash();
                toast(hashDialog === "all" ? "已清空全部哈希" : "已删除该哈希");
                revise(v => v + 1);
              })}
            >
              {hashDialog === "one" && (
                <Field label="input_hash"><Input value={hash} maxLength={128} required onChange={e => setHash(e.target.value)} /></Field>
              )}
            </StepUpDialog>
          )}
        </>
      )}
    </Resource>
  );
}

function KeyRow({item}: {item: KeyStatus}) {
  const ok = item.status === "healthy" || item.status === "ok" || item.status === "active";
  return (
    <InsetRow label={item.masked || `密钥 #${item.index}`}>
      <Badge variant="soft" tone={ok ? "ok" : "warn"}>{item.status || "未知"}</Badge>{" "}
      <span className="code-mono text-fine">成功 {item.success_count} · 失败 {item.failure_count} · {item.last_latency_ms} ms · HTTP {item.last_http_status || "—"}{item.frozen_until ? ` · 冻结至 ${date(item.frozen_until)}` : ""}</span>
    </InsetRow>
  );
}
