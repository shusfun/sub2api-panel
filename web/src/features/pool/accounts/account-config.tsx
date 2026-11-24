import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {account} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Button,
  ErrorLine,
  Field,
  Form,
  FormActions,
  FormSection,
  Input,
  InsetRow,
  MultiGroupSelect,
  PasswordInput,
  ProxySelect,
  Resource,
  Select,
  Switch,
  Textarea,
  Toolbar,
  toast,
} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";
import {balancePlatforms} from "./account-helpers";

const balanceSettingsSchema = z.object({enabled: z.boolean(), provider: z.string(), base_url: z.string(), configured: z.boolean()});

export function AccountBalanceSettings({account: acc, revision, onRefreshed}: {account: AccountRow; revision: number; onRefreshed: () => void}) {
  const path = "/admin/accounts/" + acc.id + "/balance-settings";
  const settings = useResource(() => api(path, balanceSettingsSchema), path, true, revision);
  return <Resource state={settings}>{value => <BalanceSettingsEditor key={revision} path={path} value={value} account={acc} onRefreshed={onRefreshed} />}</Resource>;
}

function BalanceSettingsEditor({value, path, account: acc, onRefreshed}: {value: z.infer<typeof balanceSettingsSchema>; path: string; account: AccountRow; onRefreshed: () => void}) {
  const [enabled, setEnabled] = useState(value.enabled);
  const [provider, setProvider] = useState(value.provider || (balancePlatforms.has(acc.platform) ? acc.platform : "sub2api"));
  const [address, setAddress] = useState(value.base_url);
  const [key, setKey] = useState("");
  const action = useAction();
  return <FormSection title="定时查询上游余额">
    <Form onSubmit={() => void action.run(async () => {
      await api(path, balanceSettingsSchema, "PUT", {enabled, provider, base_url: provider === "sub2api" ? address : "", api_key: key});
      setKey("");
      toast("余额查询配置已保存");
      onRefreshed();
    })}>
      <InsetRow label="每 30 分钟查询"><Switch label="定时查询余额" checked={enabled} onChange={setEnabled} /></InsetRow>
      {enabled && <>
        <Field label="余额接口"><Select value={provider} onChange={event => setProvider(event.target.value)}>
          {balancePlatforms.has(acc.platform) && <option value={acc.platform}>{acc.platform === "kimi" ? "Kimi" : "DeepSeek"}</option>}
          <option value="sub2api">兼容 Sub2API</option>
        </Select></Field>
        {provider === "sub2api" && <Field label="上游 HTTPS 地址"><Input value={address} onChange={event => setAddress(event.target.value)} type="url" required /></Field>}
        <Field label="余额查询密钥"><PasswordInput toggleLabel="余额查询密钥" value={key} onChange={event => setKey(event.target.value)} autoComplete="new-password"
          required={!value.configured || provider !== value.provider || provider === "sub2api" && address !== value.base_url} placeholder={value.configured ? "已配置，留空不改" : "输入已授权的上游密钥"} /></Field>
      </>}
      <Toolbar>
        <Button type="submit" size="sm" busy={action.busy}>保存查询设置</Button>
        {value.configured && <Button type="button" variant="outline" size="sm" busy={action.busy} onClick={() => void action.run(async () => {
          await api(path, balanceSettingsSchema, "POST", {});
          toast("已刷新上游余额");
          onRefreshed();
        })}>立即查询</Button>}
      </Toolbar>
      <ErrorLine value={action.error} />
    </Form>
  </FormSection>;
}

export function AccountConfigForm({
  acc,
  revision,
  onSaved,
  onRequestDelete,
}: {
  acc: AccountRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(acc.name);
  const [notes, setNotes] = useState(acc.notes);
  const [proxyID, setProxyID] = useState(acc.proxy_id ? String(acc.proxy_id) : "");
  const [groupIDs, setGroupIDs] = useState<number[]>(acc.group_ids);
  const [concurrency, setConcurrency] = useState(String(acc.concurrency));
  const [priority, setPriority] = useState(String(acc.priority));
  const [multiplier, setMultiplier] = useState(String(acc.rate_multiplier));
  const [enabled, setEnabled] = useState(acc.status === "active");
  const [newApiKey, setNewApiKey] = useState("");
  const [baseURL, setBaseURL] = useState(acc.base_url ?? "");
  const [mapping, setMapping] = useState(Object.entries(acc.model_mapping ?? {}).map(([from, to]) => from + " = " + to).join("\n"));
  const mappingText = Object.entries(acc.model_mapping ?? {}).map(([from, to]) => from + " = " + to).join("\n");

  return (
    <Form
      busy={save.busy}
      onSubmit={() =>
        void save.run(async () => {
          const credentials: Record<string, unknown> = {};
          if (newApiKey.trim()) credentials.api_key = newApiKey.trim();
          if (baseURL.trim() !== (acc.base_url ?? "")) credentials.base_url = baseURL.trim();
          if (mapping !== mappingText) {
            const entries = mapping.split(/\r?\n/).map(line => line.trim()).filter(Boolean).map(line => line.split("=").map(value => value.trim()));
            if (entries.some(pair => pair.length !== 2 || !pair[0] || !pair[1]) || new Set(entries.map(pair => pair[0])).size !== entries.length) throw new Error("每行填写一个不重复的模型映射，格式为「请求模型 = 上游模型」");
            credentials.model_mapping = Object.fromEntries(entries);
          }
          await api(`/admin/accounts/${acc.id}`, account, "PUT", {
            name: name.trim(),
            notes,
            status: enabled ? "active" : "inactive",
            ...(proxyID ? {proxy_id: Number(proxyID)} : {clear_proxy: true}),
            group_ids: groupIDs,
            concurrency: Number(concurrency || 0),
            priority: Number(priority || 0),
            rate_multiplier: Number(multiplier || 1),
            ...(Object.keys(credentials).length ? {credentials} : {}),
          });
          setNewApiKey("");
          onSaved();
        })
      }
    >
      <FormSection title="基本参数与归属">
        <InsetRow label="账号启用状态">
          <Switch label="账号启用" checked={enabled} onChange={setEnabled} />
        </InsetRow>
        <Field label="账号名称">
          <Input required value={name} onChange={e => setName(e.target.value)} />
        </Field>
        <Field label="挂载代理出口">
          <ProxySelect value={proxyID} revision={revision} onChange={setProxyID} />
        </Field>
        <Field label="所属调度分组">
          <MultiGroupSelect value={groupIDs} platform={acc.platform} revision={revision} onChange={setGroupIDs} />
        </Field>
      </FormSection>

      <FormSection title="并发、权重与轮换凭据">
        <Field label="最大并发连接">
          <div className="row-field"><Input type="number" min="0" value={concurrency} onChange={e => setConcurrency(e.target.value)} /></div>
        </Field>
        <Field label="调度优先级">
          <div className="row-field"><Input type="number" value={priority} onChange={e => setPriority(e.target.value)} /></div>
        </Field>
        <Field label="账号成本倍率">
          <div className="row-field"><Input type="number" min="0" step="0.01" value={multiplier} onChange={e => setMultiplier(e.target.value)} /></div>
        </Field>
        {acc.type === "apikey" && <>
          <Field label="上游地址"><Input type="url" value={baseURL} onChange={event => setBaseURL(event.target.value)} /></Field>
          <Field label="模型映射" hint="每行一个，请求模型 = 上游模型"><Textarea rows={4} value={mapping} onChange={event => setMapping(event.target.value)} /></Field>
          <Field label="更新上游 API Key（留空不改）">
            <PasswordInput toggleLabel="API Key" autoComplete="new-password" value={newApiKey} onChange={e => setNewApiKey(e.target.value)} placeholder="已加密保存，仅在需要轮换时输入" />
          </Field>
        </>}
        <Field label="运维备注">
          <Input value={notes} onChange={e => setNotes(e.target.value)} placeholder="选填备注" />
        </Field>
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除账号</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存账号配置</Button>
      </FormActions>
    </Form>
  );
}
