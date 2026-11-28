import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {
  Field, Form, FormGrid, Input, InsetGroup, InsetRow, MultiGroupSelect, Resource, SaveBar, Switch, toast,
} from "@/ui";

const base = "/admin/prompt-audit";
const endpoint = z.object({
  id: z.string(), name: z.string(), protocol: z.string(), base_url: z.string(), model: z.string(),
  timeout_ms: z.number(), input_limit: z.number(), enabled: z.boolean(), has_token: z.boolean(), token_status: z.string(),
});
const configSchema = z.object({
  enabled: z.boolean(), blocking_enabled: z.boolean(), blocking_latest_turn_only: z.boolean(), store_pass_events: z.boolean(),
  effective_mode: z.string(), strategy: z.string(), worker_count: z.number(), queue_capacity: z.number(),
  scanners: z.array(z.string()), all_groups: z.boolean(), group_ids: z.array(z.number()), endpoints: z.array(endpoint),
  config_version: z.number(), updated_at: z.string(), change_summary: z.string(),
});
const runtimeSchema = z.object({
  process_status: z.string(), effective_mode: z.string(), expected_config_version: z.number(), active_config_version: z.number(),
  worker_total: z.number(), worker_active: z.number(), queue_capacity: z.number(), queue: z.record(z.string(), z.number()),
  processed_total: z.number(), failed_total: z.number(), dropped_total: z.number(),
  last_error_code: z.string().nullish(), database_status: z.string(), redis_status: z.string(),
});
type Config = z.infer<typeof configSchema>;

export function PromptAuditConfig({revision, onSaved}: {revision: number; onSaved: () => void}) {
  const config = useResource(() => api(base + "/config", configSchema), "prompt-config", true, revision);
  const runtime = useResource(() => api(base + "/runtime", runtimeSchema), "prompt-runtime", true, revision);
  return (
    <>
      <Resource state={runtime}>
        {rt => (
          <InsetGroup title="审计运行状态">
            <InsetRow label="进程 / 生效模式">{rt.process_status} · {rt.effective_mode}</InsetRow>
            <InsetRow label="配置版本">{rt.active_config_version} / 期望 {rt.expected_config_version}</InsetRow>
            <InsetRow label="工作线程">{rt.worker_active} / {rt.worker_total} 活跃</InsetRow>
            <InsetRow label="已处理 / 失败 / 丢弃">{rt.processed_total} / {rt.failed_total} / {rt.dropped_total}</InsetRow>
            <InsetRow label="队列">{Object.entries(rt.queue).map(([k, v]) => `${k} ${v}`).join(" · ") || "空"}（容量 {rt.queue_capacity}）</InsetRow>
            <InsetRow label="数据库 / Redis">{rt.database_status} / {rt.redis_status}</InsetRow>
            {rt.last_error_code && <InsetRow label="最近错误码">{rt.last_error_code}</InsetRow>}
          </InsetGroup>
        )}
      </Resource>
      <Resource state={config}>{value => <ConfigForm key={value.config_version} value={value} onSaved={onSaved} />}</Resource>
    </>
  );
}

function ConfigForm({value, onSaved}: {value: Config; onSaved: () => void}) {
  const [enabled, setEnabled] = useState(value.enabled);
  const [blocking, setBlocking] = useState(value.blocking_enabled);
  const [latestOnly, setLatestOnly] = useState(value.blocking_latest_turn_only);
  const [storePass, setStorePass] = useState(value.store_pass_events);
  const [allGroups, setAllGroups] = useState(value.all_groups);
  const [groups, setGroups] = useState(value.group_ids);
  const [dirty, setDirty] = useState(false);
  const action = useAction();
  const mark = <T,>(set: (v: T) => void) => (v: T) => { set(v); setDirty(true); };

  return (
    <Form
      busy={action.busy}
      errors={action.fieldErrors}
      onChange={() => setDirty(true)}
      onSubmit={event => {
        const fields = new FormData(event.currentTarget);
        void action.run(async () => {
          await api(base + "/config", configSchema, "PUT", {
            expected_config_version: value.config_version,
            enabled,
            blocking_enabled: blocking,
            blocking_latest_turn_only: latestOnly,
            store_pass_events: storePass,
            strategy: String(fields.get("strategy") ?? value.strategy).trim(),
            worker_count: Number(fields.get("worker_count")),
            queue_capacity: Number(fields.get("queue_capacity")),
            scanners: value.scanners,
            all_groups: allGroups,
            group_ids: groups,
            // 令牌不回显：保留原端点，不带 token 即沿用已保存的令牌。
            endpoints: value.endpoints.map(e => ({
              id: e.id, name: e.name, protocol: e.protocol, base_url: e.base_url, model: e.model,
              clear_token: false, timeout_ms: e.timeout_ms, input_limit: e.input_limit, enabled: e.enabled,
            })),
          });
          setDirty(false);
          toast("提示词审计配置已保存");
          onSaved();
        });
      }}
    >
      <InsetGroup title="提示词审计配置">
        <InsetRow label="启用提示词审计" hint={`配置版本 ${value.config_version} · 更新于 ${date(value.updated_at)}`}>
          <Switch label="启用提示词审计" checked={enabled} onChange={mark(setEnabled)} />
        </InsetRow>
        <InsetRow label="命中后拦截请求"><Switch label="拦截请求" checked={blocking} onChange={mark(setBlocking)} /></InsetRow>
        <InsetRow label="拦截只看最新一轮"><Switch label="只看最新一轮" checked={latestOnly} onChange={mark(setLatestOnly)} /></InsetRow>
        <InsetRow label="保存放行事件"><Switch label="保存放行事件" checked={storePass} onChange={mark(setStorePass)} /></InsetRow>
        <InsetRow label="覆盖全部调度分组"><Switch label="覆盖全部分组" checked={allGroups} onChange={mark(setAllGroups)} /></InsetRow>
        {!allGroups && <InsetRow label="适用调度分组" vertical><MultiGroupSelect value={groups} onChange={mark(setGroups)} /></InsetRow>}
        <InsetRow label="检测策略与容量" vertical>
          <FormGrid>
            <Field label="策略"><Input name="strategy" defaultValue={value.strategy} maxLength={64} required /></Field>
            <Field label="工作线程"><Input name="worker_count" type="number" min={1} defaultValue={value.worker_count} required /></Field>
            <Field label="队列容量"><Input name="queue_capacity" type="number" min={1} defaultValue={value.queue_capacity} required /></Field>
          </FormGrid>
        </InsetRow>
        <InsetRow label="检测端点">
          {value.endpoints.length ? value.endpoints.map(e => `${e.name}（${e.enabled ? "启用" : "停用"}，令牌${e.has_token ? "已配置" : "未配置"}）`).join("；") : "无"}
        </InsetRow>
      </InsetGroup>
      <SaveBar dirty={dirty && !(enabled && !allGroups && !groups.length)} busy={action.busy} error={action.error} submitLabel="保存审计配置" />
    </Form>
  );
}
