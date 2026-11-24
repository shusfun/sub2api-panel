import {useEffect, useId, useRef, useState} from "react";
import {Ticket} from "lucide-react";
import {z} from "zod";
import {ApiError, api} from "@/lib/http";
import {cents, date, money} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {grantSchema, issue as issueSchema, keyRow, type Config} from "@/schemas";
import {
  Badge, Button, CollapsibleSection, CopyValue, ErrorLine, Field, Form, FormGrid, FormSection,
  GroupSelect, Input, InsetRow, Overlay, Panel, PasswordInput, RemoteTable, Resource, Select,
  Status, Switch, Textarea, Toolbar, type Column,
} from "@/ui";
import {parseKeyLines} from "./key-inspector";

const keySecretSchema = z.object({key: z.string().min(1)});
const codeResultSchema = z.object({code: z.string().min(1)});
type Issue = z.infer<typeof issueSchema>;

export function CreateKeyDialog({
  open,
  defaultGroupID,
  onClose,
  onCreated,
}: {
  open: boolean;
  defaultGroupID: string;
  onClose: () => void;
  onCreated: (secret: string) => void;
}) {
  const formID = useId();
  const createAction = useAction();
  const [customKey, setCustomKey] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState(defaultGroupID);

  useEffect(() => {
    if (open && !selectedGroup && defaultGroupID) setSelectedGroup(defaultGroupID);
  }, [defaultGroupID, open, selectedGroup]);

  return (
    <Overlay
      open={open}
      title="新建 API Key"
      onClose={onClose}
      busy={createAction.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={createAction.busy}>创建密钥</Button>
        </>
      }
    >
      <Form
        id={formID}
        busy={createAction.busy}
        onSubmit={event => {
          const fd = new FormData(event.currentTarget);
          const name = String(fd.get("name") ?? "").trim();
          const whitelist = parseKeyLines(String(fd.get("ip_whitelist") ?? ""));
          const blacklist = parseKeyLines(String(fd.get("ip_blacklist") ?? ""));
          const expDays = Number(fd.get("expires_in_days") || 0);
          void createAction.run(async () => {
            const res = await api("/keys", keySecretSchema, "POST", {
              name,
              ...(customKey ? {custom_key: String(fd.get("custom_key") ?? "").trim()} : {}),
              ...(selectedGroup ? {group_id: Number(selectedGroup)} : {}),
              ip_whitelist: whitelist,
              ip_blacklist: blacklist,
              quota: Number(fd.get("quota") || 0),
              ...(expDays > 0 ? {expires_in_days: expDays} : {}),
              rate_limit_5h: Number(fd.get("rate_limit_5h") || 0),
              rate_limit_1d: Number(fd.get("rate_limit_1d") || 0),
              rate_limit_7d: Number(fd.get("rate_limit_7d") || 0),
            });
            setCustomKey(false);
            onCreated(res.key);
          });
        }}
      >
        <FormSection title="基础标识与路由">
          <Field label="密钥名称">
            <Input name="name" required maxLength={100} placeholder="例如：本地开发 / Claude Code" />
          </Field>
          <InsetRow label="使用自定义密钥"><Switch label="使用自定义密钥" checked={customKey} onChange={setCustomKey} /></InsetRow>
          {customKey && (
            <Field label="自定义密钥">
              <PasswordInput name="custom_key" toggleLabel="自定义密钥" autoComplete="new-password" required minLength={16} maxLength={256} pattern="[A-Za-z0-9_\-]+" />
            </Field>
          )}
          <Field label="调用分组">
            <GroupSelect admin={false} value={selectedGroup} placeholder="自动匹配默认分组" onChange={setSelectedGroup} />
          </Field>
        </FormSection>

        <CollapsibleSection title="预算限额与来源 IP 限制（可选）" summary="默认不限额">
          <FormGrid>
            <Field label="总额度上限 (USD)"><Input type="number" name="quota" min="0" step="0.01" defaultValue="0" /></Field>
            <Field label="有效天数（0 为永久）"><Input type="number" name="expires_in_days" min="0" defaultValue="0" /></Field>
            <Field label="5 小时限额 (USD)"><Input type="number" name="rate_limit_5h" min="0" step="0.01" defaultValue="0" /></Field>
            <Field label="24 小时限额 (USD)"><Input type="number" name="rate_limit_1d" min="0" step="0.01" defaultValue="0" /></Field>
            <Field label="7 天限额 (USD)"><Input type="number" name="rate_limit_7d" min="0" step="0.01" defaultValue="0" /></Field>
          </FormGrid>
          <FormGrid>
            <Field label="IP 白名单"><Textarea name="ip_whitelist" rows={2} placeholder="每行一个 IP 或 CIDR" /></Field>
            <Field label="IP 黑名单"><Textarea name="ip_blacklist" rows={2} placeholder="每行一个 IP 或 CIDR" /></Field>
          </FormGrid>
        </CollapsibleSection>

        <ErrorLine value={createAction.error} />
      </Form>
    </Overlay>
  );
}

const batchKeyLimits = [["quota", "总额度"], ["rate_limit_5h", "5 小时限额"], ["rate_limit_1d", "24 小时限额"], ["rate_limit_7d", "7 天限额"]] as const;
const batchKeyIP = [["ip_whitelist", "IP 白名单"], ["ip_blacklist", "IP 黑名单"]] as const;

export function EditKeyBatch({ids, onClose, onApplied}: {ids: number[]; onClose: () => void; onApplied: (failed: number[], succeeded: number) => void}) {
  const formID = useId();
  const action = useAction();
  const [group, setGroup] = useState("");
  const [expiry, setExpiry] = useState("");
  const [enabled, setEnabled] = useState<string[]>([]);
  const [failures, setFailures] = useState<string[]>([]);
  const enable = (name: string, label: string) => (
    <InsetRow label={label}>
      <Switch label={label} checked={enabled.includes(name)} onChange={next => setEnabled(values => next ? [...values, name] : values.filter(value => value !== name))} />
    </InsetRow>
  );
  return (
    <Overlay
      open
      title="批量编辑密钥"
      onClose={onClose}
      busy={action.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>关闭</Button>
          <Button type="submit" form={formID} busy={action.busy} disabled={!enabled.length || !ids.length}>更新 {ids.length} 个密钥</Button>
        </>
      }
    >
      <Form
        id={formID}
        onSubmit={event => {
          const form = new FormData(event.currentTarget);
          void action.run(async () => {
            const updates: Record<string, unknown> = {};
            if (enabled.includes("group_id")) {
              if (!group) throw new Error("请选择新分组");
              updates.group_id = Number(group);
            }
            if (enabled.includes("status")) updates.status = form.get("status");
            if (enabled.includes("expires_at")) {
              if (expiry === "clear") updates.expires_at = "";
              else {
                const dt = new Date(String(form.get("expires_at") ?? ""));
                if (!Number.isFinite(dt.getTime())) throw new Error("请输入有效到期时间");
                updates.expires_at = dt.toISOString();
              }
            }
            for (const [key] of batchKeyLimits) if (enabled.includes(key)) updates[key] = Number(form.get(key));
            for (const [key] of batchKeyIP) if (enabled.includes(key)) updates[key] = parseKeyLines(String(form.get(key) ?? ""));
            const failed: number[] = [];
            const messages: string[] = [];
            for (let start = 0; start < ids.length; start += 5) {
              const batch = ids.slice(start, start + 5);
              const outcomes = await Promise.allSettled(batch.map(id => api("/keys/" + id, keyRow, "PUT", updates)));
              outcomes.forEach((result, index) => {
                if (result.status === "rejected") {
                  failed.push(batch[index]);
                  messages.push("#" + batch[index] + " · " + (result.reason instanceof Error ? result.reason.message : "更新失败"));
                }
              });
            }
            setFailures(messages);
            onApplied(failed, ids.length - failed.length);
          });
        }}
      >
        {enable("group_id", "修改分组")}
        {enabled.includes("group_id") && <Field label="目标分组"><GroupSelect admin={false} value={group} onChange={setGroup} clearable={false} /></Field>}
        {enable("status", "修改状态")}
        {enabled.includes("status") && <Field label="状态"><Select name="status"><option value="active">启用</option><option value="inactive">停用</option></Select></Field>}
        {enable("expires_at", "修改到期时间")}
        {enabled.includes("expires_at") && (
          <>
            <Field label="有效期"><Select value={expiry} onChange={event => setExpiry(event.target.value)}><option value="">指定时间</option><option value="clear">永久有效</option></Select></Field>
            {expiry !== "clear" && <Field label="到期时间"><Input name="expires_at" type="datetime-local" required /></Field>}
          </>
        )}
        {batchKeyLimits.map(([key, label]) => (
          <FormSection key={key}>
            {enable(key, "修改" + label)}
            {enabled.includes(key) && <Field label={label}><Input name={key} type="number" min="0" step="any" required /></Field>}
          </FormSection>
        ))}
        {batchKeyIP.map(([key, label]) => (
          <FormSection key={key}>
            {enable(key, "修改" + label)}
            {enabled.includes(key) && <Field label={label}><Textarea name={key} rows={2} /></Field>}
          </FormSection>
        ))}
        <ErrorLine value={action.error} />
        {failures.map(message => <ErrorLine key={message} value={message} />)}
      </Form>
    </Overlay>
  );
}

export function SubAdminCodeBox({config, revision, onIssued}: {config: Config; revision: number; onIssued: () => void}) {
  const [kind, setKind] = useState("");
  const [value, setValue] = useState("");
  const [secret, setSecret] = useState("");
  const pending = useRef<{id: string; input: string} | null>(null);
  const operation = useAction();
  const grant = useResource(() => api("/grant", grantSchema), "subadmin-grant", true, revision);

  const issueCols: Column<Issue>[] = [
    {id: "created", title: "时间", render: row => <span className="code-mono">{date(row.created_at * 1000)}</span>},
    {id: "kind", title: "类型", render: row => <Badge variant="tag">{row.kind === "balance" ? "余额码" : "注册邀请码"}</Badge>},
    {id: "amount", title: "面额", align: "number", render: row => row.amount_cents ? money(row.amount_cents / 100) : "—"},
    {id: "state", title: "状态", align: "center", render: row => <Status value={row.state} />},
  ];

  return (
    <Panel title="授权发码工作区" subtitle="使用管理员授予你的独立预算签发邀请码或余额码">
      <Resource state={grant}>
        {permit => {
          const amounts = permit.amounts_cents.filter(a => a <= permit.budget_cents);
          const canInvite = config.invitation && permit.invites > 0;
          const canBalance = amounts.length > 0;
          const selected = kind && (kind === "invitation" ? canInvite : canBalance) ? kind : canInvite ? "invitation" : "balance";
          if (!canInvite && !canBalance) return <span className="muted">当前暂无可用发码授权额度。</span>;
          return (
            <Form
              busy={operation.busy}
              onSubmit={() => void operation.run(async () => {
                setSecret("");
                const amount = selected === "invitation" ? 0 : Number(value || amounts[0]);
                const fp = `${selected}/${amount}`;
                const opID = pending.current?.input === fp ? pending.current.id : crypto.randomUUID();
                pending.current = {id: opID, input: fp};
                try {
                  const res = await api("/issues", codeResultSchema, "POST", {operation_id: opID, kind: selected, amount_cents: cents(String(amount / 100))});
                  setSecret(res.code);
                  pending.current = null;
                } catch (err) {
                  if (err instanceof ApiError && ["issue_rejected", "grant_exhausted", "invalid_input", "not_allowed"].includes(err.code)) {
                    pending.current = null;
                  }
                  throw err;
                } finally {
                  onIssued();
                }
              })}
            >
              <Toolbar>
                <Field label="码种">
                  <Select value={selected} onChange={e => { setKind(e.target.value); setValue(""); }}>
                    {canInvite && <option value="invitation">注册邀请码（剩 {permit.invites} 张）</option>}
                    {canBalance && <option value="balance">余额兑换码（剩 {money(permit.budget_cents / 100)}）</option>}
                  </Select>
                </Field>
                {selected === "balance" && (
                  <Field label="允许面额">
                    <Select value={value || amounts[0]} onChange={e => setValue(e.target.value)}>
                      {amounts.map(a => <option key={a} value={a}>{money(a / 100)}</option>)}
                    </Select>
                  </Field>
                )}
                <Button type="submit" busy={operation.busy}><Ticket aria-hidden />签发一张</Button>
              </Toolbar>
              <ErrorLine value={operation.error} />
            </Form>
          );
        }}
      </Resource>
      {secret && <CopyValue label="新签发的码" value={secret} />}
      <RemoteTable path="/issues" schema={issueSchema} columns={issueCols} rowID={r => r.operation_id} label="我的签发流水" revision={revision} />
    </Panel>
  );
}
