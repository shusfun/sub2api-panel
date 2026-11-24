import {useId, useState} from "react";
import {z} from "zod";
import {api, empty} from "../../api";
import {dateTimeInputValue} from "../../format";
import {useAction, useResource} from "../../requests";
import {
  AccessRuleBuilder, Button, ErrorLine, Feedback, Field, Form, FormGrid, Input, InsetGroup, InsetRow,
  MaskedSecret, Overlay, PermissionPicker, Resource, SecretRevealDialog, Status, StepUpDialog, Switch, TagInput, toast,
  type PermissionEntry, type ScopedAccessRule,
} from "../../ui";

const path = "/admin/settings-tools/access-keys";
const legacyPath = "/admin/settings-tools/admin-key";
const legacyStatus = z.object({legacy_exists: z.boolean(), legacy_masked: z.string()});
const rule = z.object({permissions: z.array(z.string()).nullish(), group_ids: z.array(z.number()).nullish(), all_groups: z.boolean().optional(), include_ungrouped: z.boolean().optional(), account_types: z.array(z.string()).nullish(), platforms: z.array(z.string()).nullish()});
const policy = z.object({full_access: z.boolean().optional(), rules: z.array(rule).nullish(), global_permissions: z.array(z.string()).nullish()});
const item = z.object({id: z.string(), name: z.string(), description: z.string(), enabled: z.boolean(), expires_at: z.string().nullable(), revoked_at: z.string().nullable(), allowed_ips: z.array(z.string()).nullish(), key_prefix: z.string(), policy});
const accessKeys = z.object({items: z.array(item), permissions: z.array(z.object({id: z.string(), label: z.string(), category: z.string(), scope: z.string()})).nullish()});
const secretResult = z.object({key: z.string().min(1)});
type Item = z.infer<typeof item>;
type Mutation = {label: string; path: string; method: string; config?: Record<string, unknown>; returnsSecret?: boolean; critical?: boolean};

export function AdminAccessKeys() {
  const [revision, revise] = useState(0);
  const [editor, edit] = useState<Item | "new">();
  const [pending, setPending] = useState<Mutation>();
  const [secret, setSecret] = useState("");
  const action = useAction();
  const list = useResource(() => api(path, accessKeys), path, true, revision);
  const legacy = useResource(() => api(legacyPath, legacyStatus), legacyPath, true, revision);
  const request = (operation: Mutation) => { action.clearError(); setPending(operation); };

  return (
    <InsetGroup title="管理访问密钥" actions={<Button size="sm" onClick={() => edit("new")}>创建访问密钥</Button>}>
      <Resource state={legacy}>{value => (
        <InsetRow label="旧版全局密钥" hint={value.legacy_exists ? undefined : "新密钥通过细粒度权限管理创建"}>
          {value.legacy_exists && <MaskedSecret masked={value.legacy_masked} />}
          {value.legacy_exists && (
            <>
              <Button size="xs" variant="outline" onClick={() => request({label: "重新生成全局密钥，旧密钥将立即失效", path: legacyPath + "/rotate", method: "POST", returnsSecret: true, critical: true})}>重新生成</Button>
              <Button size="xs" variant="destructive" onClick={() => request({label: "删除旧版全局密钥", path: legacyPath, method: "DELETE", critical: true})}>删除</Button>
            </>
          )}
        </InsetRow>
      )}</Resource>
      <Resource state={list}>{value => (
        <>
          {value.items.length === 0 && <InsetRow label="还没有管理访问密钥" />}
          {value.items.map(key => {
            const endpoint = path + "/" + encodeURIComponent(key.id);
            const stateKey = key.revoked_at ? "revoked" : key.enabled ? "enabled" : "disabled";
            return (
              <InsetRow key={key.id} label={key.name} hint={key.description || undefined}>
                <MaskedSecret masked={key.key_prefix} />
                <Status value={stateKey} />
                {key.revoked_at ? (
                  <Button size="xs" variant="destructive" onClick={() => request({label: `删除已吊销密钥「${key.name}」记录`, path: endpoint + "/record", method: "DELETE"})}>删除记录</Button>
                ) : (
                  <>
                    <Button size="xs" variant="outline" onClick={() => edit(key)}>编辑权限</Button>
                    <Button size="xs" variant="outline" onClick={() => request({label: `轮换访问密钥「${key.name}」，旧密钥将立即失效`, path: endpoint + "/rotate", method: "POST", returnsSecret: true, critical: true})}>轮换</Button>
                    <Button size="xs" variant="destructive" onClick={() => request({label: `吊销访问密钥「${key.name}」`, path: endpoint, method: "DELETE", critical: true})}>吊销</Button>
                  </>
                )}
              </InsetRow>
            );
          })}
        </>
      )}</Resource>

      {editor && (
        <AccessKeyEditor
          key={editor === "new" ? "new" : editor.id}
          value={editor === "new" ? undefined : editor}
          permissions={list?.value?.permissions ?? []}
          onClose={() => edit(undefined)}
          onSave={config => {
            const current = editor === "new" ? undefined : editor;
            edit(undefined);
            request({
              label: current ? `保存访问密钥「${current.name}」配置` : "创建管理访问密钥",
              path: current ? path + "/" + encodeURIComponent(current.id) : path,
              method: current ? "PUT" : "POST",
              config,
              returnsSecret: !current,
            });
          }}
        />
      )}

      {pending && (
        <StepUpDialog
          title="验证管理员身份"
          labels={[pending.label]}
          critical={pending.critical}
          confirmLabel={pending.critical ? "确认执行" : "验证并保存"}
          busy={action.busy}
          error={action.error}
          fieldErrors={action.fieldErrors}
          onClose={() => setPending(undefined)}
          onConfirm={({totp}) =>
            void action.run(async () => {
              const result = await api(pending.path, pending.returnsSecret ? secretResult : empty, pending.method, {
                totp,
                ...(pending.config ? {config: pending.config} : {}),
              });
              if ("key" in result && typeof result.key === "string") {
                setSecret(result.key);
              } else {
                toast("访问密钥操作已完成");
              }
              setPending(undefined);
              revise(value => value + 1);
            })
          }
        />
      )}

      <SecretRevealDialog
        open={Boolean(secret)}
        title="复制新的管理访问密钥"
        label="管理访问密钥"
        value={secret}
        onClose={() => setSecret("")}
      />
    </InsetGroup>
  );
}

function toScopedRules(raw: Item["policy"]["rules"]): ScopedAccessRule[] {
  return (raw ?? []).map(r => ({
    permissions: r.permissions ?? [],
    group_ids: r.group_ids ?? [],
    all_groups: r.all_groups ?? false,
    include_ungrouped: r.include_ungrouped ?? false,
    account_types: r.account_types ?? [],
    platforms: r.platforms ?? [],
  }));
}

function AccessKeyEditor({
  value,
  permissions,
  onClose,
  onSave,
}: {
  value?: Item;
  permissions: readonly PermissionEntry[];
  onClose: () => void;
  onSave: (config: Record<string, unknown>) => void;
}) {
  const formID = useId();
  const [enabled, setEnabled] = useState(value?.enabled ?? true);
  const [full, setFull] = useState(value?.policy.full_access ?? false);
  const [ips, setIps] = useState<string[]>(() => value?.allowed_ips ?? []);
  const [globals, setGlobals] = useState<string[]>(() => value?.policy.global_permissions ?? []);
  const [rules, setRules] = useState<ScopedAccessRule[]>(() => toScopedRules(value?.policy.rules));
  const [error, setError] = useState("");

  return (
    <Overlay
      open
      drawer
      title={value ? "编辑管理访问密钥" : "创建管理访问密钥"}
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID}>继续验证</Button>
        </>
      }
    >
      <Form
        id={formID}
        onSubmit={event => {
          const body = new FormData(event.currentTarget);
          const name = String(body.get("name") ?? "").trim();
          if (!name) return setError("请填写密钥名称");
          const expiration = String(body.get("expires_at") ?? "").trim();
          const timestamp = expiration ? new Date(expiration) : undefined;
          if (timestamp && !Number.isFinite(timestamp.getTime())) return setError("请选择有效的到期时间");
          if (!full && globals.length === 0 && rules.every(r => r.permissions.length === 0)) {
            return setError("请至少勾选一项全局权限或添加一条范围规则");
          }
          setError("");
          onSave({
            name,
            description: String(body.get("description") ?? ""),
            enabled,
            expires_at: timestamp ? timestamp.toISOString() : null,
            allowed_ips: ips,
            confirm_full_access: full,
            policy: {
              full_access: full,
              rules: full ? [] : rules,
              global_permissions: full ? [] : globals,
            },
          });
        }}
      >
        <FormGrid>
          <Field label="名称" name="name"><Input name="name" required maxLength={100} defaultValue={value?.name} /></Field>
          <Field label="到期时间" name="expires_at" hint="留空长期有效">
            <Input type="datetime-local" step={1} name="expires_at" defaultValue={dateTimeInputValue(value?.expires_at)} />
          </Field>
        </FormGrid>
        <Field label="备注" name="description"><Input name="description" maxLength={255} defaultValue={value?.description} /></Field>
        <Field label="来源 IP 白名单" name="allowed_ips" hint="输入 IPv4 / IPv6 或 CIDR 后按回车，留空不限来源">
          <TagInput
            name="allowed_ips"
            value={ips}
            placeholder="例如 203.0.113.10 或 10.0.0.0/8"
            onChange={setIps}
          />
        </Field>
        <InsetRow label="启用访问密钥"><Switch label="启用管理访问密钥" checked={enabled} onChange={setEnabled} /></InsetRow>
        <InsetRow label="授予全部管理权限"><Switch label="授予全部管理权限" checked={full} onChange={setFull} /></InsetRow>
        {full ? (
          <Feedback>该密钥拥有整个官方实例的完整管理权限（适合发码进程等受控组件）。</Feedback>
        ) : (
          <>
            <Field label="全局管理权限">
              <PermissionPicker
                permissions={permissions}
                scopeFilter="global"
                value={globals}
                onChange={setGlobals}
              />
            </Field>
            <Field label="分组与平台范围规则">
              <AccessRuleBuilder
                permissions={permissions}
                value={rules}
                onChange={setRules}
              />
            </Field>
          </>
        )}
        <ErrorLine value={error} />
      </Form>
    </Overlay>
  );
}
