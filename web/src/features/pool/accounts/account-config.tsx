import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {account} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Button,
  ConfirmDialog,
  ErrorLine,
  Field,
  Form,
  FormActions,
  FormGrid,
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
  const [org, setOrg] = useState(acc.organization ?? "");
  const [project, setProject] = useState(acc.project ?? "");
  const [region, setRegion] = useState(acc.region ?? "");
  const [accessKeyID, setAccessKeyID] = useState(acc.access_key_id ?? "");
  const [secretAccessKey, setSecretAccessKey] = useState("");
  const [sessionToken, setSessionToken] = useState("");
  const [gcpProjectID, setGcpProjectID] = useState(acc.project_id ?? "");
  const [gcpLocation, setGcpLocation] = useState(acc.location ?? "");
  const [saJSON, setSaJSON] = useState("");
  const [customHeadersText, setCustomHeadersText] = useState(
    Object.keys(acc.custom_headers ?? {}).map(k => `${k} = `).join("\n"),
  );
  const initialHeadersText = Object.keys(acc.custom_headers ?? {}).map(k => `${k} = `).join("\n");
  const [mapping, setMapping] = useState(Object.entries(acc.model_mapping ?? {}).map(([from, to]) => from + " = " + to).join("\n"));
  const mappingText = Object.entries(acc.model_mapping ?? {}).map(([from, to]) => from + " = " + to).join("\n");
  const [mixedRiskMsg, setMixedRiskMsg] = useState<string>();

  async function submitAccount(confirmMixedRisk = false) {
    const credentials: Record<string, unknown> = {};
    if (newApiKey.trim()) credentials.api_key = newApiKey.trim();
    if (baseURL.trim() !== (acc.base_url ?? "")) credentials.base_url = baseURL.trim();
    if (org.trim() !== (acc.organization ?? "")) credentials.organization = org.trim();
    if (project.trim() !== (acc.project ?? "")) credentials.project = project.trim();
    if (region.trim() !== (acc.region ?? "")) credentials.region = region.trim();
    if (accessKeyID.trim() !== (acc.access_key_id ?? "")) credentials.access_key_id = accessKeyID.trim();
    if (secretAccessKey.trim()) credentials.secret_access_key = secretAccessKey.trim();
    if (sessionToken.trim()) credentials.session_token = sessionToken.trim();
    if (gcpProjectID.trim() !== (acc.project_id ?? "")) credentials.project_id = gcpProjectID.trim();
    if (gcpLocation.trim() !== (acc.location ?? "")) credentials.location = gcpLocation.trim();
    if (saJSON.trim()) credentials.service_account_json = saJSON.trim();
    if (customHeadersText !== initialHeadersText) {
      const headers: Record<string, string> = {};
      for (const line of customHeadersText.split(/\r?\n/).map(l => l.trim()).filter(Boolean)) {
        const idx = line.indexOf("=");
        if (idx > 0) headers[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
      }
      credentials.custom_headers = headers;
    }
    if (mapping !== mappingText) {
      const entries = mapping.split(/\r?\n/).map(line => line.trim()).filter(Boolean).map(line => line.split("=").map(value => value.trim()));
      if (entries.some(pair => pair.length !== 2 || !pair[0] || !pair[1]) || new Set(entries.map(pair => pair[0])).size !== entries.length) throw new Error("每行填写一个不重复的模型映射，格式为「请求模型 = 上游模型」");
      credentials.model_mapping = Object.fromEntries(entries);
    }
    if (!confirmMixedRisk && groupIDs.length > 0 && groupIDs.join(",") !== acc.group_ids.join(",")) {
      const check = await api("/admin/accounts/check-mixed-channel", z.object({has_risk: z.boolean(), message: z.string().optional()}), "POST", {
        platform: acc.platform,
        group_ids: groupIDs,
        account_id: acc.id,
      });
      if (check.has_risk) {
        setMixedRiskMsg(check.message || "所选分组存在混渠道风险，确定继续绑定吗？");
        return;
      }
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
      ...(confirmMixedRisk ? {confirm_mixed_channel_risk: true} : {}),
    });
    setNewApiKey("");
    setSecretAccessKey("");
    setSessionToken("");
    setSaJSON("");
    setMixedRiskMsg(undefined);
    onSaved();
  }

  return (
    <>
      <Form
        busy={save.busy}
        onSubmit={() => void save.run(() => submitAccount(false))}
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
        {(acc.type === "apikey" || acc.type === "upstream") && <>
          <Field label="上游地址 (Base URL)"><Input type="url" value={baseURL} onChange={event => setBaseURL(event.target.value)} /></Field>
          <Field label="更新上游 API Key（留空不改）">
            <PasswordInput toggleLabel="API Key" autoComplete="new-password" value={newApiKey} onChange={e => setNewApiKey(e.target.value)} placeholder="已加密保存，仅在需要轮换时输入" />
          </Field>
        </>}
        {acc.platform === "openai" && (
          <FormGrid>
            <Field label="OpenAI Organization"><Input value={org} onChange={e => setOrg(e.target.value)} placeholder="org-..." /></Field>
            <Field label="OpenAI Project"><Input value={project} onChange={e => setProject(e.target.value)} placeholder="proj_..." /></Field>
          </FormGrid>
        )}
        {acc.type === "bedrock" && (
          <FormGrid>
            <Field label="AWS Region"><Input value={region} onChange={e => setRegion(e.target.value)} placeholder="us-east-1" /></Field>
            <Field label="Access Key ID"><Input value={accessKeyID} onChange={e => setAccessKeyID(e.target.value)} placeholder="AKIA..." /></Field>
            <Field label="Secret Access Key（留空不改）"><PasswordInput toggleLabel="Secret Access Key" autoComplete="new-password" value={secretAccessKey} onChange={e => setSecretAccessKey(e.target.value)} placeholder="已加密保存" /></Field>
            <Field label="Session Token（可选，留空不改）"><PasswordInput toggleLabel="Session Token" autoComplete="new-password" value={sessionToken} onChange={e => setSessionToken(e.target.value)} placeholder="可选临时令牌" /></Field>
          </FormGrid>
        )}
        {acc.type === "service_account" && (
          <FormGrid>
            <Field label="GCP Project ID"><Input value={gcpProjectID} onChange={e => setGcpProjectID(e.target.value)} placeholder="my-gcp-project" /></Field>
            <Field label="GCP Location"><Input value={gcpLocation} onChange={e => setGcpLocation(e.target.value)} placeholder="us-central1" /></Field>
            <Field label="Service Account JSON（留空不改）"><Textarea rows={3} value={saJSON} onChange={e => setSaJSON(e.target.value)} placeholder="仅在需要更新密钥时粘贴 JSON" /></Field>
          </FormGrid>
        )}
        <Field label="模型映射" hint="每行一个，请求模型 = 上游模型"><Textarea rows={4} value={mapping} onChange={event => setMapping(event.target.value)} /></Field>
        <Field label="自定义请求头 (custom_headers)" hint="每行一个 HeaderName = Value，值留空保留原密文"><Textarea rows={3} value={customHeadersText} onChange={event => setCustomHeadersText(event.target.value)} placeholder="X-Custom-Auth = " /></Field>
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
    <ConfirmDialog
      open={!!mixedRiskMsg}
      title="混渠道绑定风险确认"
      description={mixedRiskMsg ?? ""}
      confirm="确认继续绑定"
      busy={save.busy}
      error={save.error}
      onClose={() => setMixedRiskMsg(undefined)}
      onConfirm={() => void save.run(() => submitAccount(true))}
    />
    </>
  );
}
