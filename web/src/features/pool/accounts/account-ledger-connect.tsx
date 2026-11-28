import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {ledgerProbeResult, type LedgerAccountSeries, type LedgerProbeResult} from "@/schemas/ledger";
import {useAction, useResource} from "@/lib/requests";
import {Button, ErrorLine, Field, Form, FormSection, Input, InsetRow, PasswordInput, Resource, Select, Switch, Toolbar, toast} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";
import {balancePlatforms} from "./account-helpers";

const balanceSettingsSchema = z.object({enabled: z.boolean(), provider: z.string(), base_url: z.string(), configured: z.boolean()});

const hostedProviders = new Set(["sub2api", "newapi", "oneapi"]);
const newApiScopes = ["profile:read", "usage:read", "wallet:read"];

// 接入引导（取代旧的 AccountBalanceSettings）：
//   1. POST /admin/ledger/probe {url}：探测上游类型，按结果预选 provider 并带出令牌页；
//   2. PUT  /admin/accounts/{id}/balance-settings {enabled, provider, base_url, api_key, access_token, upstream_user_id}：
//      后端先试采一次，失败返回 502 且不保存；provider 与地址未变时，密钥和访问令牌留空沿用旧值；
//   3. POST /admin/accounts/{id}/balance-settings：立即查询。
export function AccountLedgerConnect({account: acc, connection, revision, onChanged}: {account: AccountRow; connection: LedgerAccountSeries["connection"]; revision: number; onChanged: () => void}) {
  const path = `/admin/accounts/${acc.id}/balance-settings`;
  const settings = useResource(() => api(path, balanceSettingsSchema), path, true, revision);
  return <Resource state={settings}>{value => <ConnectEditor key={revision} path={path} value={value} account={acc} connection={connection} onChanged={onChanged} />}</Resource>;
}

function ConnectEditor({value, path, account: acc, connection, onChanged}: {value: z.infer<typeof balanceSettingsSchema>; path: string; account: AccountRow; connection: LedgerAccountSeries["connection"]; onChanged: () => void}) {
  const [enabled, setEnabled] = useState(value.enabled);
  const [provider, setProvider] = useState(value.provider || connection.provider || (balancePlatforms.has(acc.platform) ? acc.platform : "sub2api"));
  const [address, setAddress] = useState(value.base_url || connection.base_url);
  const [key, setKey] = useState("");
  const [accessToken, setAccessToken] = useState("");
  const [userID, setUserID] = useState("");
  const [probed, setProbed] = useState<LedgerProbeResult>();
  const save = useAction();
  const probe = useAction();

  const hosted = hostedProviders.has(provider);
  const newApi = provider === "newapi";
  const unchanged = provider === value.provider && address === value.base_url;
  const tokenPage = probed?.token_page_url.startsWith("https://") ? probed.token_page_url : "";

  function submitBody() {
    const uid = Number(userID);
    if (newApi && accessToken.trim() !== "" && (!Number.isInteger(uid) || uid < 1)) throw new Error("New API 使用访问令牌时需要填写上游用户 ID（不小于 1 的整数）");
    return {
      enabled,
      provider,
      base_url: hosted ? address : "",
      api_key: key,
      ...(hosted && accessToken.trim() !== "" ? {access_token: accessToken.trim()} : {}),
      ...(newApi && userID.trim() !== "" ? {upstream_user_id: uid} : {}),
    };
  }

  return (
    <FormSection title="接入上游账本">
      <Form onSubmit={() => void save.run(async () => {
        await api(path, balanceSettingsSchema, "PUT", submitBody());
        setKey("");
        setAccessToken("");
        setUserID("");
        toast("余额查询配置已保存");
        onChanged();
      })}>
        <InsetRow label="定时查询余额"><Switch label="定时查询余额" checked={enabled} onChange={setEnabled} /></InsetRow>
        {enabled && (
          <>
            <Field label="余额接口">
              <Select value={provider} onChange={event => setProvider(event.target.value)}>
                {balancePlatforms.has(acc.platform) && <option value={acc.platform}>{acc.platform === "kimi" ? "Kimi" : "DeepSeek"}</option>}
                <option value="sub2api">Sub2API</option>
                <option value="newapi">New API</option>
                <option value="oneapi">One API</option>
              </Select>
            </Field>
            {hosted && (
              <Field label="上游 HTTPS 地址" hint={probed ? `探测结果：${probed.type}` : undefined}>
                <Input value={address} onChange={event => setAddress(event.target.value)} type="url" required />
              </Field>
            )}
            {newApi && (
              <>
                {tokenPage && (
                  <ol className="muted text-fine">
                    <li><a href={tokenPage} target="_blank" rel="noopener noreferrer">在新标签页打开令牌页</a></li>
                    <li>创建访问令牌，并勾选权限：{newApiScopes.map(scope => <code key={scope} className="code-mono"> {scope}</code>)}</li>
                    <li>把令牌和你在上游的用户 ID 填到下面</li>
                  </ol>
                )}
                <Field label="访问令牌" hint={connection.access_token_configured && unchanged ? "已配置，留空沿用旧值" : undefined}>
                  <PasswordInput toggleLabel="访问令牌" value={accessToken} onChange={event => setAccessToken(event.target.value)} autoComplete="new-password"
                    placeholder={connection.access_token_configured && unchanged ? "已配置，留空不改" : "New API 访问令牌"} />
                </Field>
                <Field label="上游用户 ID" hint={connection.access_token_configured && unchanged ? "留空沿用旧值" : "使用访问令牌时必填"}>
                  <Input type="number" min="1" step="1" value={userID} onChange={event => setUserID(event.target.value)} />
                </Field>
              </>
            )}
            <Field label={newApi ? "Key（可选，用于读取 Key 已用额度）" : "余额查询密钥"}>
              <PasswordInput toggleLabel="余额查询密钥" value={key} onChange={event => setKey(event.target.value)} autoComplete="new-password"
                required={!newApi && (!value.configured || !unchanged) && accessToken.trim() === ""}
                placeholder={value.configured && unchanged ? "已配置，留空不改" : "输入已授权的上游密钥"} />
            </Field>
          </>
        )}
        <Toolbar>
          <Button type="submit" size="sm" busy={save.busy}>保存并试采</Button>
          {enabled && hosted && (
            <Button type="button" size="sm" variant="outline" busy={probe.busy} onClick={() => void probe.run(async () => {
              const result = await api("/admin/ledger/probe", ledgerProbeResult, "POST", {url: address});
              setProbed(result);
              if (result.type !== "unknown") setProvider(result.type);
              if (result.origin) setAddress(result.origin);
            })}>探测上游</Button>
          )}
          {value.configured && (
            <Button type="button" size="sm" variant="outline" busy={save.busy} onClick={() => void save.run(async () => {
              await api(path, balanceSettingsSchema, "POST", {});
              toast("已刷新上游余额");
              onChanged();
            })}>立即查询</Button>
          )}
        </Toolbar>
        {save.error && <p className="muted text-fine">保存前会先向上游试采一次，失败则不保存。请核对地址、密钥或令牌后重试。</p>}
        <ErrorLine value={save.error || probe.error} />
      </Form>
    </FormSection>
  );
}
