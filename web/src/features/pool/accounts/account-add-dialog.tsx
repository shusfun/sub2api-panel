import {useEffect, useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {account, adminGroupRow, proxyRow} from "@/schemas";
import {useAction} from "@/lib/requests";
import {
  Button,
  ChoiceCards,
  CollapsibleSection,
  ErrorLine,
  Field,
  Form,
  FormSection,
  Input,
  InsetRow,
  MultiGroupSelect,
  Overlay,
  PasswordInput,
  ProxySelect,
  Segmented,
  Select,
  Stack,
  Textarea,
  Toolbar,
} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";

export function AddAccountDialog({
  open,
  revision,
  defaultGroupID,
  defaultProxyID,
  reauthAccount,
  onClose,
  onCreated,
  onInlineResourceCreated,
}: {
  open: boolean;
  revision: number;
  defaultGroupID?: number;
  defaultProxyID?: number;
  reauthAccount?: AccountRow;
  onClose: () => void;
  onCreated: (id?: number) => void;
  onInlineResourceCreated: () => void;
}) {
  const formID = useId();
  const [platform, setPlatform] = useState(reauthAccount?.platform || "openai");
  const [mode, setMode] = useState<"oauth" | "apikey" | "import">(reauthAccount ? "oauth" : "oauth");
  const [name, setName] = useState(reauthAccount?.name || "");
  const [proxyID, setProxyID] = useState(reauthAccount?.proxy_id ? String(reauthAccount.proxy_id) : defaultProxyID ? String(defaultProxyID) : "");
  const [groupIDs, setGroupIDs] = useState<number[]>(reauthAccount?.group_ids ?? (defaultGroupID ? [defaultGroupID] : []));
  const [apiKeyVal, setApiKeyVal] = useState("");
  const [baseUrlVal, setBaseUrlVal] = useState("");
  const [oauthSession, setOauthSession] = useState<{auth_url: string; session_id: string; state: string}>();
  const [oauthCode, setOauthCode] = useState("");
  const [claudeMode, setClaudeMode] = useState<"oauth" | "setup-token">("oauth");
  const [importKind, setImportKind] = useState<"codex-session" | "codex-pat" | "grok-sso">("codex-session");
  const [importLines, setImportLines] = useState("");
  const [inlineRev, setInlineRev] = useState(0);

  const [quickProxyHost, setQuickProxyHost] = useState("");
  const [quickProxyPort, setQuickProxyPort] = useState("8080");
  const [quickGroupName, setQuickGroupName] = useState("");

  const op = useAction();
  const inlineOp = useAction();

  const oauthSupported = ["openai", "anthropic", "gemini", "antigravity", "grok"].includes(platform);

  useEffect(() => {
    if (!oauthSupported && mode === "oauth") setMode("apikey");
  }, [mode, oauthSupported]);

  return (
    <Overlay
      open={open}
      title={reauthAccount ? `重新授权 · ${reauthAccount.name}` : "添加或导入上游账号"}
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" disabled={op.busy} onClick={onClose}>取消</Button>
          {mode === "oauth" && oauthSession && (
            <Button type="submit" form={formID} busy={op.busy} disabled={!oauthCode.trim() || (!reauthAccount && !name.trim())}>
              完成服务端授权
            </Button>
          )}
          {mode === "apikey" && !reauthAccount && (
            <Button type="submit" form={formID} busy={op.busy} disabled={!name.trim() || !apiKeyVal.trim()}>
              保存账号
            </Button>
          )}
          {mode === "import" && !reauthAccount && (
            <Button type="submit" form={formID} busy={op.busy}>
              开始批量导入
            </Button>
          )}
        </>
      }
    >
      {!reauthAccount && (
        <ChoiceCards
          label="选择上游平台"
          value={platform}
          options={[
            {value: "openai", title: "OpenAI", description: "Codex OAuth / Key / PAT"},
            {value: "anthropic", title: "Anthropic", description: "Claude OAuth / Key"},
            {value: "gemini", title: "Gemini", description: "Google OAuth / Key"},
            {value: "antigravity", title: "Antigravity", description: "Companion OAuth"},
            {value: "grok", title: "Grok", description: "xAI OAuth / SSO"},
            {value: "deepseek", title: "DeepSeek", description: "API Key 直连"},
          ]}
          onChange={val => {
            if (val !== platform) setGroupIDs([]);
            setPlatform(val);
            setOauthSession(undefined);
          }}
        />
      )}

      {!reauthAccount && (
        <Segmented
          label="接入方式"
          value={mode}
          options={[
            ...(oauthSupported ? ([["oauth", "OAuth 服务端闭环授权"]] as const) : []),
            ["apikey", "API Key 凭据"],
            ["import", "Session / PAT / SSO 批量导入"],
          ]}
          onChange={v => setMode(v as "oauth" | "apikey" | "import")}
        />
      )}

      {!reauthAccount && (
        <FormSection title="分组与代理">
          {mode !== "import" && (
            <Field label="账号名称">
              <Input value={name} onChange={e => setName(e.target.value)} placeholder={`例如 ${platform}-prod-01`} />
            </Field>
          )}
          <Field label="代理节点">
            <ProxySelect value={proxyID} revision={revision + inlineRev} onChange={setProxyID} />
          </Field>
          <Field label="挂载调度分组">
            <MultiGroupSelect value={groupIDs} platform={platform} revision={revision + inlineRev} onChange={setGroupIDs} />
          </Field>
        </FormSection>
      )}

      {!reauthAccount && (
        <CollapsibleSection title="新建代理或分组">
          <Stack>
            <Toolbar>
              <Field label="代理主机"><Input value={quickProxyHost} onChange={e => setQuickProxyHost(e.target.value)} placeholder="127.0.0.1" /></Field>
              <Field label="端口"><Input type="number" value={quickProxyPort} onChange={e => setQuickProxyPort(e.target.value)} /></Field>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!quickProxyHost.trim()}
                busy={inlineOp.busy}
                onClick={() =>
                  void inlineOp.run(async () => {
                    const px = await api("/admin/proxies", proxyRow, "POST", {
                      name: `${quickProxyHost.trim()}:${quickProxyPort}`,
                      protocol: "http",
                      host: quickProxyHost.trim(),
                      port: Number(quickProxyPort || 8080),
                    });
                    setProxyID(String(px.id));
                    setQuickProxyHost("");
                    setInlineRev(v => v + 1);
                    onInlineResourceCreated();
                  })
                }
              >
                新建并选用代理
              </Button>
            </Toolbar>
            <Toolbar>
              <Field label="分组名称"><Input value={quickGroupName} onChange={e => setQuickGroupName(e.target.value)} placeholder={`新 ${platform} 分组名称`} /></Field>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!quickGroupName.trim()}
                busy={inlineOp.busy}
                onClick={() =>
                  void inlineOp.run(async () => {
                    const grp = await api("/admin/groups", adminGroupRow, "POST", {
                      name: quickGroupName.trim(),
                      platform,
                      rate_multiplier: 1,
                    });
                    setGroupIDs(prev => [...new Set([...prev, grp.id])]);
                    setQuickGroupName("");
                    setInlineRev(v => v + 1);
                    onInlineResourceCreated();
                  })
                }
              >
                新建并勾选分组
              </Button>
            </Toolbar>
          </Stack>
          <ErrorLine value={inlineOp.error} />
        </CollapsibleSection>
      )}

      {mode === "oauth" && (
        <Form
          id={formID}
          busy={op.busy}
          onSubmit={() => {
            if (!oauthSession || !oauthCode.trim() || (!reauthAccount && !name.trim())) return;
            void op.run(async () => {
              const created = await api("/admin/accounts/oauth/complete", account, "POST", {
                platform,
                mode: platform === "anthropic" ? claudeMode : undefined,
                session_id: oauthSession.session_id,
                state: oauthSession.state,
                code: oauthCode.trim(),
                ...(reauthAccount ? {account_id: reauthAccount.id} : {name: name.trim(), group_ids: groupIDs}),
                ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
              });
              onCreated(created.id);
            });
          }}
        >
          <FormSection title="服务端闭环 OAuth 授权" description="授权码在面板 Go 服务端内部换取 Token 并直接写入上游，浏览器全程不接触上游令牌。">
            {platform === "anthropic" && (
              <Field label="Claude 授权方式">
                <Segmented
                  label="Claude 授权方式"
                  value={claudeMode}
                  options={[
                    ["oauth", "标准 OAuth"],
                    ["setup-token", "Setup Token"],
                  ] as const}
                  onChange={setClaudeMode}
                />
              </Field>
            )}
            <InsetRow label="第 1 步：生成官方授权链接">
              <Button
                type="button"
                size="sm"
                variant="outline"
                busy={op.busy}
                onClick={() =>
                  void op.run(async () => {
                    const res = await api(
                      "/admin/accounts/oauth/start",
                      z.object({auth_url: z.string(), session_id: z.string(), state: z.string()}),
                      "POST",
                      {
                        platform,
                        mode: platform === "anthropic" ? claudeMode : undefined,
                        ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
                      },
                    );
                    setOauthSession(res);
                    window.open(res.auth_url, "_blank", "noopener,noreferrer");
                  })
                }
              >
                获取并打开授权链接
              </Button>
            </InsetRow>
            {oauthSession && (
              <Field label="第 2 步：粘贴回调 Code 或回调 URL 参数">
                <Input name="code" value={oauthCode} onChange={e => setOauthCode(e.target.value)} placeholder="粘贴授权完成后的 code" required />
              </Field>
            )}
          </FormSection>
          <ErrorLine value={op.error} />
        </Form>
      )}

      {mode === "apikey" && !reauthAccount && (
        <Form
          id={formID}
          busy={op.busy}
          onSubmit={() =>
            void op.run(async () => {
              const created = await api("/admin/accounts", account, "POST", {
                name: name.trim(),
                platform,
                type: "apikey",
                credentials: {
                  api_key: apiKeyVal.trim(),
                  ...(baseUrlVal.trim() ? {base_url: baseUrlVal.trim()} : {}),
                },
                ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
                group_ids: groupIDs,
              });
              onCreated(created.id);
            })
          }
        >
          <FormSection title="API Key 凭据录入">
            <Field label="上游 API Key">
              <PasswordInput required toggleLabel="API Key" autoComplete="new-password" value={apiKeyVal} onChange={e => setApiKeyVal(e.target.value)} placeholder="sk-..." />
            </Field>
            <Field label="自定义 Base URL（可选）">
              <Input value={baseUrlVal} onChange={e => setBaseUrlVal(e.target.value)} placeholder="留空使用平台官方网关" />
            </Field>
          </FormSection>
          <ErrorLine value={op.error} />
        </Form>
      )}

      {mode === "import" && !reauthAccount && (
        <Form
          id={formID}
          busy={op.busy}
          onSubmit={() =>
            void op.run(async () => {
              const items = importLines
                .split(/\r?\n/)
                .map(s => s.trim())
                .filter(Boolean);
              await api(
                "/admin/accounts/import",
                z.object({created: z.number(), updated: z.number(), skipped: z.number(), failed: z.number()}),
                "POST",
                {
                  kind: importKind,
                  name: name.trim() || undefined,
                  items,
                  ...(proxyID ? {proxy_id: Number(proxyID)} : {}),
                  group_ids: groupIDs,
                  update_existing: true,
                },
              );
              onCreated();
            })
          }
        >
          <FormSection title="批量导入上游会话凭据">
            <Field label="导入格式">
              <Select value={importKind} onChange={e => setImportKind(e.target.value as "codex-session" | "codex-pat" | "grok-sso")}>
                <option value="codex-session">OpenAI Codex Session JSON</option>
                <option value="codex-pat">OpenAI Codex Access Token (PAT)</option>
                <option value="grok-sso">Grok SSO Cookie 批量转换</option>
              </Select>
            </Field>
            <Field label="凭据内容（每行一条）">
              <Textarea rows={5} required value={importLines} onChange={e => setImportLines(e.target.value)} placeholder="每行粘贴一条 Session JSON / Token / SSO Cookie" />
            </Field>
          </FormSection>
          <ErrorLine value={op.error} />
        </Form>
      )}
    </Overlay>
  );
}
