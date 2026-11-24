import {useId, useState} from "react";
import {z} from "zod";
import {api, empty, named} from "../api";
import {useAction, useResource} from "../requests";
import {
  AccountSelect, Button, ChoiceCards, ConfirmDialog, ErrorLine, Field, Form, FormGrid, InlineStack, Input, InsetGroup,
  InsetRow, Overlay, PasswordInput, PlatformBadge, Resource, Select, Switch, TagInput, toast,
} from "../ui";
import {RemoteTable} from "./remote-table";

const base = "/admin/channel-monitors";
const entry = named.extend({provider: z.string(), primary_model: z.string()});
const rule = entry.extend({api_mode: z.string(), endpoint: z.string(), api_key_masked: z.string(), extra_models: z.array(z.string()).nullable(),
  group_name: z.string(), enabled: z.boolean(), interval_seconds: z.number(), jitter_seconds: z.number(), check_mode: z.string(), account_id: z.number().nullable()});
type Rule = z.infer<typeof rule>;
const providers: Record<string, string> = {openai: "OpenAI", anthropic: "Anthropic", gemini: "Gemini", grok: "Grok", antigravity: "Antigravity",
  kimi: "Kimi", zhipu: "智谱", deepseek: "DeepSeek", minimax: "MiniMax", opencode_go: "OpenCode Go"};

export function ChannelRules() {
  const [revision, revise] = useState(0);
  const [selected, select] = useState<number>();
  const [removing, remove] = useState<z.infer<typeof entry>>();
  const action = useAction();
  const saved = (message: string) => { select(undefined); remove(undefined); toast(message); revise(value => value + 1); };
  return <InsetGroup title="主动监控规则" actions={<Button size="sm" onClick={() => select(0)}>新增监控规则</Button>}>
    <RemoteTable path={base} schema={entry} rowID={row => row.id} label="主动监控规则" revision={revision} columns={[
      {id: "name", title: "规则", render: row => row.name},
      {id: "provider", title: "平台", render: row => <PlatformBadge platform={row.provider} />},
      {id: "model", title: "探测模型", render: row => row.primary_model || "—"},
      {id: "actions", title: "操作", align: "end", render: row => <InlineStack>
        <Button size="sm" variant="ghost" onClick={() => select(row.id)}>编辑</Button>
        <Button size="sm" variant="destructive" onClick={() => { action.clearError(); remove(row); }}>删除</Button>
      </InlineStack>},
    ]} />
    {selected !== undefined && <RuleDialog key={selected} id={selected} close={() => select(undefined)} onSaved={saved} />}
    {removing && <ConfirmDialog open title="删除监控规则" description={"将删除监控规则「" + removing.name + "」。"} confirm="删除规则"
      busy={action.busy} error={action.error} onClose={() => remove(undefined)} onConfirm={() => void action.run(async () => {
        await api(base + "/" + removing.id, empty, "DELETE");
        saved("监控规则已删除");
      })} />}
  </InsetGroup>;
}

function RuleDialog({id, close, onSaved}: {id: number; close: () => void; onSaved: (message: string) => void}) {
  const path = base + "/" + id;
  const data = useResource(() => api(path, rule), path, id > 0);
  const action = useAction();
  const formID = useId();
  return <Overlay open title={id ? "编辑监控规则" : "新增监控规则"} onClose={close} busy={action.busy}
    footer={<>
      <Button variant="ghost" onClick={close}>取消</Button>
      <Button type="submit" form={formID} busy={action.busy} disabled={id > 0 && !data?.value}>保存规则</Button>
    </>}>
    {id ? <Resource state={data}>{value => <RuleForm value={value} formID={formID} action={action} onSaved={onSaved} />}</Resource>
      : <RuleForm formID={formID} action={action} onSaved={onSaved} />}
  </Overlay>;
}

function RuleForm({value, action, formID, onSaved}: {value?: Rule; action: ReturnType<typeof useAction>; formID: string; onSaved: (message: string) => void}) {
  const [mode, setMode] = useState(value?.check_mode || "probe");
  const [provider, setProvider] = useState(value?.provider || "openai");
  const [account, setAccount] = useState(value?.account_id ? String(value.account_id) : "");
  const [extraModels, setExtraModels] = useState<string[]>(() => value?.extra_models ?? []);
  const [enabled, setEnabled] = useState(value?.enabled ?? true);
  const probe = mode !== "quota";
  const quota = mode !== "probe";

  return <Form id={formID} busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
    const fields = new FormData(event.currentTarget);
    const field = (name: string) => String(fields.get(name) ?? "").trim();
    void action.run(async () => {
      if (quota && !account) throw new Error("请选择用于查询配额的上游账号。");
      await api(value ? base + "/" + value.id : base, rule, value ? "PUT" : "POST", {
        name: field("name"), provider, check_mode: mode, enabled, group_name: field("group_name"),
        interval_seconds: Number(fields.get("interval")), jitter_seconds: Number(fields.get("jitter")),
        account_id: quota ? Number(account) : value?.account_id ? 0 : undefined,
        api_mode: provider === "openai" && probe ? field("api_mode") : undefined,
        endpoint: probe ? field("endpoint") : undefined,
        api_key: probe ? field("api_key") || undefined : undefined,
        primary_model: probe ? field("primary_model") : undefined,
        extra_models: probe ? extraModels : undefined,
      });
      onSaved("监控规则已保存");
    });
  }}>
    <Field label="检查模式">
      <ChoiceCards
        label="检查模式"
        value={mode}
        columns={3}
        onChange={setMode}
        options={[
          {value: "probe", title: "主动探测", description: "定时发送请求测量延迟与可用性"},
          {value: "quota", title: "配额查询", description: "轮询关联账号剩余额度"},
          {value: "quota_probe", title: "配额与探测", description: "同时检测调用可用性与额度"},
        ]}
      />
    </Field>
    <FormGrid>
      <Field label="名称"><Input name="name" defaultValue={value?.name} maxLength={100} required /></Field>
      <Field label="显示分组"><Input name="group_name" defaultValue={value?.group_name} maxLength={100} /></Field>
      <Field label="平台"><Select value={provider} onChange={event => setProvider(event.target.value)}>
        {Object.entries(providers).map(([id, title]) => <option key={id} value={id}>{title}</option>)}
      </Select></Field>
      <Field label="检查间隔"><Input name="interval" type="number" suffix="秒" min={15} max={3600} defaultValue={value?.interval_seconds ?? 60} required /></Field>
      <Field label="随机抖动上限"><Input name="jitter" type="number" suffix="秒" min={0} max={3585} defaultValue={value?.jitter_seconds ?? 0} required /></Field>
      {quota && <Field label="关联账号"><AccountSelect value={account} onChange={setAccount} placeholder="选择上游账号" /></Field>}
      {probe && <>
        {provider === "openai" && <Field label="接口协议"><Select name="api_mode" defaultValue={value?.api_mode || "chat_completions"}>
          <option value="chat_completions">Chat Completions</option><option value="responses">Responses</option>
        </Select></Field>}
        <Field label="探测端点"><Input name="endpoint" type="url" maxLength={500} defaultValue={value?.endpoint} required /></Field>
        <Field label="探测密钥" hint={value?.api_key_masked ? "留空保持现有密钥不变" : undefined}>
          <PasswordInput name="api_key" toggleLabel="探测密钥" autoComplete="new-password" maxLength={2000} required={!value} />
        </Field>
        <Field label="主模型"><Input name="primary_model" maxLength={200} defaultValue={value?.primary_model} required /></Field>
      </>}
    </FormGrid>
    {probe && (
      <Field label="附加探测模型">
        <TagInput value={extraModels} placeholder="输入模型 ID 后按回车" onChange={setExtraModels} />
      </Field>
    )}
    <InsetRow label="启用此监控规则"><Switch label="启用监控" checked={enabled} onChange={setEnabled} /></InsetRow>
    <ErrorLine value={action.error} />
  </Form>;
}
