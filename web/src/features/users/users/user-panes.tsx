import {useId, useState} from "react";
import {Pencil, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {cents, money, smartMoney} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {adminSubscriptionRow, empty, grantSchema, keyRow, page, userRow} from "@/schemas";
import {
  Button, ConfirmDialog, DataTable, EmptyState, ErrorLine, Feedback, Field, Form, FormActions,
  FormSection, GroupSelect, Input, Overlay, PasswordInput,
  PlanIdentity, RemoteTable, Resource, Select, Status, StepUpDialog, Switch, TagInput,
  toast,
} from "@/ui";
import {ExtendSubscription} from "./user-dialogs";
import {AdminEditKeyDialog} from "./user-key-edit";
import type {SubRow, UserItem} from "./index";

export function ReplaceUserGroup({
  user,
  groups,
  onClose,
  onSaved,
}: {
  user: UserItem;
  groups: readonly {id: number; name: string; status: string; is_exclusive?: boolean; subscription_type: string}[];
  onClose: () => void;
  onSaved: (message: string) => void;
}) {
  const formID = useId();
  const action = useAction();
  const [oldID, setOld] = useState("");
  const [newID, setNew] = useState("");
  const source = groups.filter(group => group.is_exclusive && user.allowed_groups.includes(group.id));
  const destination = groups.filter(group => group.is_exclusive && group.status === "active" && group.subscription_type === "standard" && String(group.id) !== oldID);
  return (
    <Overlay
      open
      title="替换专属分组"
      onClose={onClose}
      busy={action.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={action.busy} disabled={!oldID || !newID || oldID === newID}>迁移密钥</Button>
        </>
      }
    >
      <Form
        id={formID}
        onSubmit={() => void action.run(async () => {
          const result = await api("/admin/users/" + user.id + "/replace-group", z.object({migrated_keys: z.number().int().nonnegative()}), "POST", {old_group_id: Number(oldID), new_group_id: Number(newID)});
          onSaved("已迁移 " + result.migrated_keys + " 个密钥并更新分组授权");
        })}
      >
        <Field label="当前分组"><Select value={oldID} onChange={event => { setOld(event.target.value); setNew(""); }}><option value="">选择分组</option>{source.map(group => <option key={group.id} value={group.id}>{group.name}</option>)}</Select></Field>
        <Field label="目标分组"><Select value={newID} onChange={event => setNew(event.target.value)}><option value="">选择分组</option>{destination.map(group => <option key={group.id} value={group.id}>{group.name}</option>)}</Select></Field>
        {!source.length && <Feedback>此用户未获授权访问任何专属分组。</Feedback>}
        <ErrorLine value={action.error} />
      </Form>
    </Overlay>
  );
}

export function UserChangeDialog({task, usr, onClose, onSaved}: {task: "balance" | "limits" | "profile"; usr: UserItem; onClose: () => void; onSaved: (message: string) => void}) {
  const formID = useId();
  const op = useAction();
  const [enabled, setEnabled] = useState(usr.status === "active");
  const [role, setRole] = useState(usr.role || "user");
  const [restrictPublic, setRestrictPublic] = useState(usr.restrict_public_groups);
  const [pendingRoleBody, setPendingRoleBody] = useState<Record<string, unknown>>();
  const path = `/admin/users/${usr.id}`;
  const title = {balance: "调整用户余额", limits: "修改用户限流", profile: "编辑用户账号"}[task];
  return (
    <>
      <Overlay
        open
        title={title}
        onClose={onClose}
        busy={op.busy}
        footer={<><Button variant="outline" onClick={onClose}>取消</Button><Button type="submit" form={formID} busy={op.busy}>保存</Button></>}
      >
        <Form
          id={formID}
          busy={op.busy}
          onSubmit={event => {
            const data = new FormData(event.currentTarget);
            const password = String(data.get("password") ?? "");
            if (task === "profile") {
              const body: Record<string, unknown> = {
                username: String(data.get("username") ?? "").trim(),
                status: enabled ? "active" : "disabled",
                restrict_public_groups: restrictPublic,
                notes: String(data.get("notes") ?? ""),
                ...(password ? {password} : {}),
                ...(role !== usr.role ? {role} : {}),
              };
              if (role !== usr.role && (role === "admin" || usr.role === "admin")) {
                op.clearError();
                setPendingRoleBody(body);
                return;
              }
              void op.run(async () => {
                await api(path, userRow, "PUT", body);
                onSaved("用户账号已更新");
              });
              return;
            }
            void op.run(async () => {
              if (task === "balance") await api(path + "/balance", userRow, "POST", {balance: Number(data.get("balance")), operation: data.get("operation"), notes: data.get("notes")});
              else await api(path, userRow, "PUT", {concurrency: Number(data.get("concurrency")), rpm_limit: Number(data.get("rpm"))});
              onSaved({balance: "用户余额已更新", limits: "用户限流已更新"}[task]);
            });
          }}
        >
          {task === "balance" && (
            <>
              <Field label="调整方式"><Select name="operation" defaultValue="add"><option value="add">增加</option><option value="subtract">扣减</option><option value="set">设为</option></Select></Field>
              <Field label="金额"><Input name="balance" type="number" suffix="USD" min="0" step="0.01" required /></Field>
            </>
          )}
          {task === "limits" && (
            <>
              <Field label="最大并发"><Input name="concurrency" type="number" min="0" max="10000" defaultValue={usr.concurrency} required /></Field>
              <Field label="每分钟请求上限"><Input name="rpm" type="number" suffix="RPM" min="0" max="100000" defaultValue={usr.rpm_limit} required /></Field>
            </>
          )}
          {task === "profile" && (
            <>
              <Field label="显示昵称"><Input name="username" defaultValue={usr.username} /></Field>
              <Field label="账号角色">
                <Select value={role} onChange={e => setRole(e.target.value)}>
                  <option value="user">普通用户</option>
                  <option value="admin">管理员</option>
                </Select>
              </Field>
              <Switch checked={enabled} label="启用该用户" onChange={setEnabled} />
              <Switch checked={restrictPublic} label="限制访问公开分组（仅限专属分组）" onChange={setRestrictPublic} />
              <Field label="新密码（选填）"><PasswordInput name="password" autoComplete="new-password" minLength={6} maxLength={128} /></Field>
            </>
          )}
          {task !== "limits" && <Field label="备注（选填）"><Input name="notes" defaultValue={task === "profile" ? usr.notes : ""} /></Field>}
          <ErrorLine value={op.error} />
        </Form>
      </Overlay>
      {pendingRoleBody && (
        <StepUpDialog
          title="验证管理员角色变更"
          labels={[usr.email, `角色变更：${usr.role} → ${role}`]}
          confirmLabel="验证并保存"
          busy={op.busy}
          error={op.error}
          fieldErrors={op.fieldErrors}
          onClose={() => setPendingRoleBody(undefined)}
          onConfirm={({totp}) => void op.run(async () => {
            await api(path, userRow, "PUT", {...pendingRoleBody, totp});
            setPendingRoleBody(undefined);
            onSaved("用户账号与角色已更新");
          })}
        />
      )}
    </>
  );
}

export function UserSubscriptionsPane({userID, revision, onChanged}: {userID: number; revision: number; onChanged: (msg: string) => void}) {
  const [renew, setRenew] = useState<SubRow>();
  const subs = useResource(() => api(`/admin/users/${userID}/subscriptions?page=1`, page(adminSubscriptionRow)), `usr-subs-${userID}`, true, revision);
  const op = useAction();
  return (
    <>
      <Resource state={subs}>
        {data => (
          <DataTable
            label="用户订阅"
            rows={data.items}
            rowID={r => r.id}
            empty={<EmptyState title="该用户暂无订阅套餐" />}
            columns={[
              {id: "group", title: "订阅组 / 到期", render: r => <PlanIdentity item={{id: r.group_id, name: r.group, platform: r.platform, expires_at: r.expires_at}} />},
              {id: "status", title: "状态", align: "center", render: r => <Status value={r.status} />},
              {
                id: "act",
                title: "操作",
                align: "end",
                render: r => (
                  <div className="row-actions">
                    <Button size="xs" variant="outline" onClick={() => setRenew(r)}>续期</Button>
                    <Button size="xs" variant="ghost" busy={op.busy} onClick={() => void op.run(async () => { await api(`/admin/subscriptions/${r.id}/reset-quota`, empty, "POST", {}); onChanged("已重置订阅周期用量"); })}>清零配额</Button>
                    {r.status === "active" ? (
                      <Button size="xs" variant="destructive" busy={op.busy} onClick={() => void op.run(async () => { await api(`/admin/subscriptions/${r.id}/revoke`, empty, "POST", {}); onChanged("已撤销该订阅"); })}>撤销</Button>
                    ) : (
                      <Button size="xs" variant="outline" busy={op.busy} onClick={() => void op.run(async () => { await api(`/admin/subscriptions/${r.id}/restore`, empty, "POST", {}); onChanged("已恢复该订阅"); })}>恢复</Button>
                    )}
                  </div>
                ),
              },
            ]}
          />
        )}
      </Resource>
      {renew && <ExtendSubscription id={renew.id} onClose={() => setRenew(undefined)} onSaved={() => { setRenew(undefined); onChanged("订阅已续期"); }} />}
      <ErrorLine value={op.error} />
    </>
  );
}

export function UserKeysPane({userID, revision, onChanged}: {userID: number; revision: number; onChanged: (msg: string) => void}) {
  const op = useAction();
  const [editing, setEditing] = useState<z.infer<typeof keyRow>>();
  const [deleting, setDeleting] = useState<z.infer<typeof keyRow>>();
  return (
    <>
      <RemoteTable
        path={"/admin/users/" + userID + "/keys"}
        schema={keyRow}
        revision={revision}
        label="用户名下 API Key"
        rowID={r => r.id}
        empty={<EmptyState title="该用户尚未创建 API Key" />}
        columns={[
          {
            id: "key",
            title: "Key / 消费",
            render: r => (
              <div>
                <strong>{r.name}</strong> <span className="muted code-mono text-fine">{r.masked}</span>
                <div className="muted text-fine">
                  今日 {r.today_actual_cost === null ? "—" : smartMoney(r.today_actual_cost)} · 累计 {r.total_actual_cost === null ? "—" : smartMoney(r.total_actual_cost)}
                </div>
              </div>
            ),
          },
          {id: "status", title: "状态", align: "center", render: r => <Status value={r.status} />},
          {
            id: "group",
            title: "绑定分组",
            render: r => (
              <GroupSelect
                value={r.group_id ? String(r.group_id) : ""}
                placeholder="未绑定分组"
                onChange={next => void op.run(async () => {
                  const result = await api(`/admin/api-keys/${r.id}`, z.object({auto_granted_group_access: z.boolean(), granted_group_name: z.string().optional()}), "PUT", {group_id: next ? Number(next) : 0});
                  onChanged(`已更新密钥「${r.name}」的绑定分组` + (result.auto_granted_group_access ? "，并授予专属分组访问权限" : ""));
                })}
              />
            ),
          },
          {
            id: "act",
            title: "操作",
            align: "end",
            render: r => (
              <div className="row-actions">
                <Button size="icon-sm" variant="ghost" title="编辑密钥" aria-label="编辑密钥" onClick={() => setEditing(r)}><Pencil aria-hidden /></Button>
                <Button size="icon-sm" variant="ghost" title="删除密钥" aria-label="删除密钥" onClick={() => setDeleting(r)}><Trash2 aria-hidden /></Button>
              </div>
            ),
          },
        ]}
      />
      <ErrorLine value={op.error} />
      {editing && (
        <AdminEditKeyDialog
          item={editing}
          revision={revision}
          onClose={() => setEditing(undefined)}
          onSaved={msg => { setEditing(undefined); onChanged(msg); }}
        />
      )}
      <ConfirmDialog
        open={!!deleting}
        title="删除代管密钥"
        description={deleting ? `确定删除密钥「${deleting.name}」吗？` : ""}
        confirm="删除"
        busy={op.busy}
        error={op.error}
        onClose={() => setDeleting(undefined)}
        onConfirm={() => deleting && void op.run(async () => {
          await api(`/admin/api-keys/${deleting.id}`, empty, "DELETE");
          setDeleting(undefined);
          onChanged(`已删除密钥「${deleting.name}」`);
        })}
      />
    </>
  );
}

export function GrantEditor({user}: {user: number}) {
  const path = "/admin/grants/" + user;
  const [revision, revise] = useState(0);
  const [confirming, confirm] = useState(false);
  const [pending, setPending] = useState<{invites: number; budget_cents: number; amounts_cents: number[]}>();
  const [amountTags, setAmountTags] = useState<string[]>();
  const current = useResource(() => api(path, grantSchema), path, true, revision);
  const save = useAction();
  const revoke = useAction();
  const prepare = useAction();

  return (
    <>
      <Resource state={current}>
        {record => {
          const tags = amountTags ?? record.amounts_cents.map(a => String(a / 100));
          return (
            <Form
              key={revision}
              busy={save.busy}
              onSubmit={event => {
                const fields = new FormData(event.currentTarget);
                void prepare.run(async () => {
                  save.clearError();
                  setPending({
                    invites: Number(fields.get("invites")),
                    budget_cents: cents(String(fields.get("budget") || "0")),
                    amounts_cents: tags.map(cents),
                  });
                });
              }}
            >
              <FormSection title={`子管理员发码预算 · 剩 ${record.invites} 张邀请 · ${money(record.budget_cents / 100)}`}>
                <Field label="设定可用邀请码张数">
                  <div className="row-field"><Input type="number" suffix="张" name="invites" min="0" max="10000" defaultValue={record.invites} required /></div>
                </Field>
                <Field label="设定余额码总预算">
                  <div className="row-field"><Input name="budget" suffix="USD" inputMode="decimal" defaultValue={String(record.budget_cents / 100)} required /></div>
                </Field>
                <Field label="允许签发的单张面额（USD）">
                  <TagInput
                    value={tags}
                    placeholder="输入面额数字后按回车，如 5、10、20"
                    validate={v => {
                      try { cents(v); return undefined; } catch (e) { return e instanceof Error ? e.message : "面额格式有误"; }
                    }}
                    onChange={setAmountTags}
                  />
                </Field>
              </FormSection>
              <ErrorLine value={prepare.error} />
              <FormActions>
                <Button variant="destructive" size="sm" disabled={save.busy} onClick={() => { revoke.clearError(); confirm(true); }}>撤销授权</Button>
                <Button type="submit" size="sm" busy={save.busy}>保存授权</Button>
              </FormActions>
            </Form>
          );
        }}
      </Resource>
      {pending && (
        <StepUpDialog
          title="验证发码授权"
          labels={[`邀请码 ${pending.invites} 张`, `余额码预算 ${money(pending.budget_cents / 100)}`]}
          confirmLabel="验证并保存"
          busy={save.busy}
          error={save.error}
          fieldErrors={save.fieldErrors}
          onClose={() => setPending(undefined)}
          onConfirm={({totp}) => void save.run(async () => {
            await api(path, empty, "PUT", {...pending, totp});
            setPending(undefined);
            setAmountTags(undefined);
            revise(n => n + 1);
            toast("已保存发码额度授权");
          })}
        />
      )}
      <ConfirmDialog
        open={confirming}
        title="撤销发码授权"
        description="该用户的发码授权与剩余预算将立即清零。"
        confirm="确认撤销"
        busy={revoke.busy}
        error={revoke.error}
        onClose={() => confirm(false)}
        onConfirm={() => void revoke.run(async () => {
          await api(path, empty, "DELETE");
          confirm(false);
          setAmountTags(undefined);
          revise(v => v + 1);
          toast("已撤销发码授权");
        })}
      />
    </>
  );
}
