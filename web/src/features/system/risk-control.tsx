import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {date} from "@/lib/format";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {
  Button, ChoiceCards, ConfirmDialog, EllipsisText, EntityLink, Field, Form, FormGrid, Input, InsetGroup, InsetRow,
  MultiGroupSelect, PasswordInput, RefreshButton, Resource, SaveBar, SearchInput, Select, Status, Switch,
  WorkspaceContent, toast, type Column,
} from "@/ui";
import {RemoteTable} from "@/ui";

const base = "/admin/risk-control";
const config = z.object({enabled: z.boolean(), engine: z.string(), mode: z.string(), model: z.string(), sample_rate: z.number(),
  all_groups: z.boolean(), group_ids: z.array(z.number()), auto_ban_enabled: z.boolean(), ban_threshold: z.number(), api_key_configured: z.boolean(),
  base_url: z.string(), timeout_ms: z.number(), record_non_hits: z.boolean(), block_status: z.number(), block_message: z.string(),
  hit_retention_days: z.number(), non_hit_retention_days: z.number(),
  openai: z.object({base_url: z.string(), model: z.string(), api_key_configured: z.boolean(), timeout_ms: z.number()})});
const log = z.object({id: z.number(), created_at: z.string(), user_id: z.number().nullable(), user_email: z.string(), request_id: z.string(),
  model: z.string(), group_name: z.string(), action: z.string(), flagged: z.boolean(), highest_category: z.string(),
  highest_score: z.number(), auto_banned: z.boolean(), user_status: z.string()});

export function RiskControl() {
  const [revision, revise] = useState(0);
  const [search, setSearch] = useState("");
  const [target, setTarget] = useState<{id: number; label: string}>();
  const keyword = useDebouncedValue(search.trim());
  const action = useAction();
  const data = useResource(() => api(base + "/config", config), "risk-config", true, revision);
  const columns: Column<z.infer<typeof log>>[] = [
    {id: "actor", title: "用户", render: row => row.user_id ? <EntityLink kind="user" id={row.user_id} label={row.user_email || "用户 #" + row.user_id} /> : "—"},
    {id: "model", title: "分组 / 模型", hideWhenCompact: true, render: row => <EllipsisText text={[row.group_name, row.model].filter(Boolean).join(" / ") || "—"} />},
    {id: "action", title: "处理动作", align: "center", render: row => <Status value={row.action} />},
    {id: "reason", title: "命中分类", hideWhenCompact: true, render: row => row.flagged ? <EllipsisText text={row.highest_category} /> : "—"},
    {id: "time", title: "时间", hideWhenNarrow: true, render: row => date(row.created_at)},
    {id: "ops", title: "操作", align: "end", render: row => row.user_id && row.user_status === "banned" &&
      <Button size="sm" variant="ghost" onClick={() => { action.clearError(); setTarget({id: row.user_id!, label: row.user_email || "用户 #" + row.user_id}); }}>解除封禁</Button>},
  ];
  return <WorkspaceContent>
    <Resource state={data}>{value => <RiskForm key={JSON.stringify(value)} value={value} onSaved={() => { revise(v => v + 1); toast("风控配置已保存"); }} />}</Resource>
    <section className="swift-group" aria-label="风控记录">
      <div className="swift-group-head">
        <span className="swift-group-title">风控记录</span>
        <div className="swift-group-actions">
          <SearchInput value={search} onChange={setSearch} placeholder="搜索用户、模型或分类" />
          <RefreshButton size="xs" variant="ghost" onClick={() => revise(v => v + 1)} />
        </div>
      </div>
      <RemoteTable path={base + "/logs?" + new URLSearchParams({search: keyword})} schema={log} columns={columns} rowID={row => row.id} label="风控记录" revision={revision} />
    </section>
    {target && <ConfirmDialog open title="解除封禁" description={"恢复 " + target.label + " 的账号访问。"} confirm="解除封禁"
      busy={action.busy} error={action.error} onClose={() => setTarget(undefined)} onConfirm={() => void action.run(async () => {
        await api(base + "/users/" + target.id + "/unban", empty, "POST");
        setTarget(undefined);
        revise(value => value + 1);
        toast("已解除用户封禁");
      })} />}
  </WorkspaceContent>;
}

function RiskForm({value, onSaved}: {value: z.infer<typeof config>; onSaved: () => void}) {
  const [enabled, setEnabled] = useState(value.enabled);
  const [allGroups, setAllGroups] = useState(value.all_groups);
  const [groups, setGroups] = useState(value.group_ids);
  const [autoBan, setAutoBan] = useState(value.auto_ban_enabled);
  const [engine, setEngine] = useState(value.engine || "openai");
  const [mode, setMode] = useState(value.mode || "observe");
  const [record, setRecord] = useState(value.record_non_hits);
  const [dirtyForm, setDirtyForm] = useState(false);
  const action = useAction();

  const dirty =
    dirtyForm ||
    enabled !== value.enabled ||
    allGroups !== value.all_groups ||
    autoBan !== value.auto_ban_enabled ||
    engine !== (value.engine || "openai") ||
    mode !== (value.mode || "observe") ||
    record !== value.record_non_hits ||
    groups.join(",") !== value.group_ids.join(",");

  return (
    <Form
      busy={action.busy}
      errors={action.fieldErrors}
      onChange={() => setDirtyForm(true)}
      onSubmit={event => {
        const fields = new FormData(event.currentTarget);
        const text = (name: string) => String(fields.get(name) ?? "").trim();
        void action.run(async () => {
          await api(base + "/config", config, "PUT", {
            enabled,
            ...(enabled
              ? {
                  sample_rate: Number(fields.get("sample_rate")),
                  all_groups: allGroups,
                  group_ids: groups,
                  auto_ban_enabled: autoBan,
                  ban_threshold: autoBan ? Number(fields.get("ban_threshold")) : undefined,
                  engine,
                  mode,
                  record_non_hits: record,
                  hit_retention_days: Number(fields.get("hit_retention_days")),
                  non_hit_retention_days: record ? Number(fields.get("non_hit_retention_days")) : undefined,
                  ...(engine === "openai"
                    ? {
                        base_url: text("base_url"),
                        model: text("model"),
                        api_key: text("api_key") || undefined,
                        timeout_ms: Number(fields.get("timeout_ms")),
                      }
                    : {}),
                  ...(mode === "pre_block"
                    ? {
                        block_status: Number(fields.get("block_status")),
                        block_message: text("block_message"),
                      }
                    : {}),
                }
              : {}),
          });
          onSaved();
        });
      }}
    >
      <InsetGroup title="风控策略配置">
        <InsetRow label="启用内容风控" hint="按抽样比例对请求执行审核与拦截">
          <Switch checked={enabled} onChange={setEnabled} label="启用内容风控" />
        </InsetRow>
        {enabled && (
          <>
            <InsetRow label="处理模式" vertical>
              <ChoiceCards
                label="处理模式"
                value={mode}
                columns={3}
                onChange={setMode}
                options={[
                  {value: "observe", title: "仅观察记录", description: "记录命中分类但正常转发上游请求"},
                  {value: "pre_block", title: "请求前拦截", description: "命中高危分类时直接拒绝并返回状态码"},
                  {value: "off", title: "暂停审核", description: "保留配置但暂停采样检测"},
                ]}
              />
            </InsetRow>
            <InsetRow label="审核引擎与参数" vertical>
              <FormGrid>
                <Field label="审核引擎">
                  <Select value={engine} onChange={event => setEngine(event.target.value)}>
                    <option value="openai">OpenAI Moderation</option>
                    <option value="typesafe">TypeSafe</option>
                  </Select>
                </Field>
                <Field label="请求抽样比例">
                  <Input name="sample_rate" type="number" suffix="%" min={0} max={100} required defaultValue={value.sample_rate} />
                </Field>
                {engine === "openai" && (
                  <>
                    <Field label="审核接口地址"><Input name="base_url" type="url" defaultValue={value.openai.base_url} maxLength={500} /></Field>
                    <Field label="审核模型"><Input name="model" defaultValue={value.openai.model} maxLength={200} /></Field>
                    <Field label="审核密钥" hint={value.openai.api_key_configured ? "已配置，留空保持原密钥" : undefined}>
                      <PasswordInput name="api_key" toggleLabel="审核密钥" autoComplete="new-password" maxLength={2000} />
                    </Field>
                    <Field label="请求超时">
                      <Input name="timeout_ms" type="number" suffix="ms" min={1} defaultValue={value.openai.timeout_ms || 5000} required />
                    </Field>
                  </>
                )}
                {mode === "pre_block" && (
                  <>
                    <Field label="拦截 HTTP 状态码"><Input name="block_status" type="number" min={400} max={599} defaultValue={value.block_status || 403} required /></Field>
                    <Field label="拦截响应提示"><Input name="block_message" defaultValue={value.block_message} maxLength={500} /></Field>
                  </>
                )}
                <Field label="命中记录保留期"><Input name="hit_retention_days" type="number" suffix="天" min={0} defaultValue={value.hit_retention_days} required /></Field>
                {record && <Field label="未命中记录保留期"><Input name="non_hit_retention_days" type="number" suffix="天" min={0} defaultValue={value.non_hit_retention_days} required /></Field>}
              </FormGrid>
            </InsetRow>
            <InsetRow label="同时保留未命中记录"><Switch label="保留未命中记录" checked={record} onChange={setRecord} /></InsetRow>
            <InsetRow label="覆盖全部调度分组"><Switch checked={allGroups} onChange={setAllGroups} label="覆盖全部分组" /></InsetRow>
            {!allGroups && (
              <InsetRow label="适用调度分组" vertical>
                <MultiGroupSelect value={groups} onChange={setGroups} />
              </InsetRow>
            )}
            <InsetRow label="连续违规自动封禁账号">
              <Switch checked={autoBan} onChange={setAutoBan} label="自动封禁" />
            </InsetRow>
            {autoBan && (
              <InsetRow label="触发封禁命中次数">
                <div className="row-field">
                  <Input name="ban_threshold" type="number" suffix="次" min={1} required defaultValue={value.ban_threshold} />
                </div>
              </InsetRow>
            )}
          </>
        )}
      </InsetGroup>
      <SaveBar
        dirty={dirty && !(enabled && !allGroups && !groups.length)}
        busy={action.busy}
        error={action.error}
        submitLabel="保存风控配置"
      />
    </Form>
  );
}
