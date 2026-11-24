import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty, proxyRow} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Button,
  ConfirmDialog,
  EntityLink,
  ErrorLine,
  Field,
  Form,
  FormActions,
  FormGrid,
  FormSection,
  Input,
  InsetGroup,
  InsetRow,
  PasswordInput,
  ProxySelect,
  Resource,
  Select,
  Switch,
} from "@/ui";
import {type ProxyRow} from "@/features/pool/shared";

export function ProxyInspector({
  id,
  revision,
  onChanged,
  onDeleted,
}: {
  id: number;
  revision: number;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const detail = useResource(() => api(`/admin/proxies/${id}`, proxyRow), `prx-detail-${id}`, true, revision);
  const linked = useResource(
    () => api(`/admin/proxies/${id}/accounts`, z.array(z.object({id: z.number(), name: z.string(), platform: z.string()}))),
    `prx-acc-${id}`,
    true,
    revision,
  );
  const [confirmDelete, setConfirmDelete] = useState(false);
  const del = useAction();

  return (
    <Resource state={detail}>
      {px => (
        <>
          <ProxyEditForm px={px} revision={revision} onSaved={onChanged} onRequestDelete={() => setConfirmDelete(true)} />
          <InsetGroup title={`挂载在此代理的账号 (${linked?.value?.length ?? 0})`}>
            {(linked?.value ?? []).length ? (
              (linked?.value ?? []).map(a => (
                <InsetRow key={a.id} label={a.name} hint={a.platform}>
                  <EntityLink kind="account" id={a.id} label="查看账号" />
                </InsetRow>
              ))
            ) : (
              <InsetRow label="暂无关联账号" />
            )}
          </InsetGroup>
          <ConfirmDialog
            open={confirmDelete}
            title="删除代理节点"
            description={`确定删除代理「${px.name}」吗？`}
            confirm="删除"
            busy={del.busy}
            error={del.error}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void del.run(async () => {
              await api(`/admin/proxies/${px.id}`, empty, "DELETE");
              setConfirmDelete(false);
              onDeleted();
            })}
          />
        </>
      )}
    </Resource>
  );
}

function ProxyEditForm({
  px,
  revision,
  onSaved,
  onRequestDelete,
}: {
  px: ProxyRow;
  revision: number;
  onSaved: () => void;
  onRequestDelete: () => void;
}) {
  const save = useAction();
  const [name, setName] = useState(px.name);
  const [protocol, setProtocol] = useState<ProxyRow["protocol"]>(px.protocol);
  const [host, setHost] = useState(px.host);
  const [port, setPort] = useState(String(px.port));
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [fallbackMode, setFallbackMode] = useState(px.fallback_mode || "none");
  const [backupID, setBackupID] = useState(px.backup_proxy_id ? String(px.backup_proxy_id) : "");
  const [active, setActive] = useState(px.status === "active");
  const [clearAuth, setClearAuth] = useState(false);

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/admin/proxies/${px.id}`, proxyRow, "PUT", {
          name: name.trim(),
          protocol,
          host: host.trim(),
          port: Number(port),
          status: active ? "active" : "inactive",
          fallback_mode: fallbackMode,
          ...(fallbackMode === "proxy" && backupID ? {backup_proxy_id: Number(backupID)} : {clear_backup_proxy_id: true}),
          ...(username ? {username} : {}),
          ...(password ? {password} : {}),
          ...(clearAuth ? {clear_auth: true} : {}),
        });
        setUsername("");
        setPassword("");
        setClearAuth(false);
        onSaved();
      })}
    >
      <FormSection title="连接与协议设置">
        <InsetRow label="启用代理"><Switch label="启用代理" checked={active} onChange={setActive} /></InsetRow>
        <Field label="名称"><Input required value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="协议">
          <div className="row-field">
            <Select value={protocol} onChange={e => setProtocol(e.target.value as ProxyRow["protocol"])}>
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="socks5">SOCKS5</option>
              <option value="socks5h">SOCKS5H</option>
            </Select>
          </div>
        </Field>
        <FormSection title="主机与端口">
          <FormGrid cols={3}>
            <div className="col-span-2"><Field label="127.0.0.1"><Input required value={host} onChange={e => setHost(e.target.value)} placeholder="127.0.0.1" /></Field></div>
            <Field label="端口"><Input required type="number" min="1" max="65535" value={port} onChange={e => setPort(e.target.value)} /></Field>
          </FormGrid>
        </FormSection>
      </FormSection>

      <FormSection title="故障回退与认证更新">
        <Field label="故障回退策略">
          <div className="row-field">
            <Select value={fallbackMode} onChange={e => setFallbackMode(e.target.value)}>
              <option value="none">不回退</option>
              <option value="direct">回退直连</option>
              <option value="proxy">切换备用代理</option>
            </Select>
          </div>
        </Field>
        {fallbackMode === "proxy" && (
          <Field label="选择备用代理">
            <ProxySelect value={backupID} excludeID={px.id} placeholder="选择备用代理节点…" revision={revision} onChange={setBackupID} />
          </Field>
        )}
        <FormSection title="新用户名 / 密码（留空保留原认证）">
          <FormGrid>
            <Field label={px.has_auth ? "已配置" : "可选用户名"}><Input value={username} onChange={e => setUsername(e.target.value)} placeholder={px.has_auth ? "已配置" : "可选用户名"} /></Field>
            <Field label="可选密码"><PasswordInput toggleLabel="代理密码" autoComplete="new-password" value={password} onChange={e => setPassword(e.target.value)} placeholder="可选密码" /></Field>
          </FormGrid>
        </FormSection>
        {px.has_auth && (
          <InsetRow label="清除已有代理认证">
            <Switch label="清除代理认证" checked={clearAuth} onChange={setClearAuth} />
          </InsetRow>
        )}
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="button" variant="destructive" size="sm" onClick={onRequestDelete}>删除</Button>
        <Button type="submit" size="sm" busy={save.busy}>保存代理</Button>
      </FormActions>
    </Form>
  );
}
