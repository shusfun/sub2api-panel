import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {themeNames, useTheme} from "@/app/appearance";
import {date, money} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {
  Badge, Button, ConfirmDialog, ErrorLine, Field, Form, FormActions, FormGrid, FormSection, InlineStack, Input, InsetGroup, InsetRow,
  OtpInput, Overlay, PasswordInput, Resource, SaveBar, Segmented, Switch, toast,
} from "@/ui";
import {TotpRow} from "./totp";

const profileSchema = z.object({
  email: z.string(), username: z.string(), role: z.enum(["admin", "user"]), balance: z.number(), concurrency: z.number(), rpm_limit: z.number(),
  notify_available: z.boolean(), notify_enabled: z.boolean(), notify_threshold: z.number().nullable(),
  notify_emails: z.array(z.object({email: z.string(), disabled: z.boolean(), verified: z.boolean()})),
  identities: z.array(z.object({provider: z.string(), label: z.string(), bound: z.boolean(), display: z.string().optional(), can_bind: z.boolean(), can_unbind: z.boolean()})),
});
const passkeysSchema = z.object({enabled: z.boolean(), items: z.array(z.object({id: z.number(), name: z.string(), created_at: z.string(), last_used_at: z.string().nullable().optional()}))});
const passkeyBegin = z.object({session_token: z.string(), options: z.record(z.unknown())});

export const personalSections = [
  {id: "account", title: "账号", hint: "邮箱、余额和用户名"},
  {id: "password", title: "密码与验证", hint: "密码、两步验证、通行密钥"},
  {id: "identities", title: "登录方式", hint: "绑定的第三方账号"},
  {id: "notify", title: "通知", hint: "余额不足提醒"},
  {id: "appearance", title: "外观", hint: "主题"},
] as const;

function useProfile(revision: number) {
  return useResource(() => api("/settings/profile", profileSchema), "settings-profile", true, revision);
}

export function PersonalSection({id}: {id: string}) {
  const [revision, revise] = useState(0);
  const profile = useProfile(revision);
  const refresh = () => revise(n => n + 1);
  if (id === "appearance") return <Appearance />;
  if (id === "password") return <><PasswordGroup /><InsetGroup title="两步验证"><TotpRow /></InsetGroup><Passkeys /></>;
  return <Resource state={profile}>{value => id === "identities" ? <Identities items={value.identities} onChanged={refresh} />
    : id === "notify" ? <Notify profile={value} onChanged={refresh} />
    : <Account profile={value} onChanged={refresh} />}</Resource>;
}

type Profile = z.infer<typeof profileSchema>;

function Account({profile, onChanged}: {profile: Profile; onChanged: () => void}) {
  const [name, setName] = useState(profile.username);
  const save = useAction();
  const dirty = name.trim() !== profile.username;
  return <>
    <InsetGroup>
      <InsetRow label="邮箱">{profile.email}</InsetRow>
      <InsetRow label="角色">{profile.role === "admin" ? "管理员" : "普通用户"}</InsetRow>
      <InsetRow label="可用余额"><strong className="weir-digits">{money(profile.balance)}</strong></InsetRow>
      <InsetRow label="并发 / 每分钟请求"><span className="weir-digits">{profile.concurrency} / {profile.rpm_limit || "不限"}</span></InsetRow>
    </InsetGroup>
    <Form busy={save.busy} errors={save.fieldErrors} onSubmit={() => void save.run(async () => {
      await api("/settings/profile", profileSchema, "PUT", {username: name.trim()});
      toast("个人资料已保存");
      onChanged();
    })}>
      <Field label="用户名" name="username" hint="显示在站点和通知邮件里。">
        <Input aria-label="用户名" maxLength={100} value={name} onChange={event => setName(event.target.value)} />
      </Field>
      <SaveBar dirty={dirty} busy={save.busy} error={save.error} onReset={() => { setName(profile.username); save.clearError(); }} />
    </Form>
  </>;
}

function PasswordGroup() {
  const formID = useId();
  const [open, setOpen] = useState(false);
  const save = useAction();
  return <>
    <InsetGroup title="密码"><InsetRow label="登录密码" hint="修改后所有设备都要重新登录" onClick={() => { save.clearError(); setOpen(true); }} /></InsetGroup>
    <Overlay open={open} title="修改密码" onClose={() => setOpen(false)} busy={save.busy} footer={
      <><Button variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button type="submit" form={formID} busy={save.busy}>修改密码</Button></>
    }>
      <Form id={formID} busy={save.busy} errors={save.fieldErrors} onSubmit={event => {
        const data = new FormData(event.currentTarget);
        void save.run(async () => {
          if (data.get("new_password") !== data.get("repeat")) throw new Error("两次输入的新密码不一致");
          await api("/settings/password", empty, "PUT", {old_password: data.get("old_password"), new_password: data.get("new_password")});
          location.reload();
        });
      }}>
        <Field label="当前密码"><PasswordInput name="old_password" autoComplete="current-password" required /></Field>
        <Field label="新密码"><PasswordInput name="new_password" autoComplete="new-password" minLength={6} maxLength={256} required /></Field>
        <Field label="再输入一次新密码"><PasswordInput name="repeat" autoComplete="new-password" minLength={6} maxLength={256} required /></Field>
        <ErrorLine value={save.error} />
      </Form>
    </Overlay>
  </>;
}

type CredentialApi = typeof PublicKeyCredential & {parseCreationOptionsFromJSON?: (options: unknown) => CredentialCreationOptions["publicKey"]};

function Passkeys() {
  const formID = useId();
  const [revision, revise] = useState(0);
  const list = useResource(() => api("/settings/passkeys", passkeysSchema), "settings-passkeys", true, revision);
  const [task, setTask] = useState<{kind: "add"} | {kind: "rename"; id: number; name: string} | {kind: "delete"; id: number; name: string}>();
  const action = useAction();
  const close = () => setTask(undefined);
  return <>
    <InsetGroup title="通行密钥" footer={list?.value && !list.value.enabled ? "站点没有开启通行密钥登录。" : "用设备的指纹、面容或安全密钥登录，不需要密码。"}>
      <Resource state={list}>{value => <>
        {value.items.map(item => <InsetRow key={item.id} label={item.name} hint={`添加于 ${date(item.created_at)}${item.last_used_at ? `，最近使用 ${date(item.last_used_at)}` : ""}`}>
          <InlineStack density="compact">
            <Button size="sm" variant="outline" onClick={() => { action.clearError(); setTask({kind: "rename", id: item.id, name: item.name}); }}>重命名</Button>
            <Button size="sm" variant="destructive" onClick={() => { action.clearError(); setTask({kind: "delete", id: item.id, name: item.name}); }}>删除</Button>
          </InlineStack>
        </InsetRow>)}
        {value.enabled && <InsetRow label="添加通行密钥" onClick={() => { action.clearError(); setTask({kind: "add"}); }} />}
        {!value.enabled && !value.items.length && <InsetRow label="未启用" />}
      </>}</Resource>
    </InsetGroup>
    {task && <Overlay open title={task.kind === "add" ? "添加通行密钥" : task.kind === "rename" ? "重命名通行密钥" : `删除「${task.name}」`} onClose={close} busy={action.busy} footer={
      <><Button variant="ghost" onClick={close}>取消</Button>
        <Button type="submit" form={formID} variant={task.kind === "delete" ? "danger-outline" : "default"} busy={action.busy}>{task.kind === "add" ? "继续" : task.kind === "rename" ? "保存名称" : "确认删除"}</Button></>
    }>
      <Form id={formID} busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
        const data = new FormData(event.currentTarget);
        const password = data.get("password");
        void action.run(async () => {
          if (task.kind === "rename") {
            const name = String(data.get("name") ?? "").trim();
            if (!name) throw new Error("请输入设备名称");
            await api(`/settings/passkeys/${task.id}`, empty, "PATCH", {name});
            toast("通行密钥已重命名");
          } else if (task.kind === "delete") {
            await api(`/settings/passkeys/${task.id}`, empty, "DELETE", {password});
            toast("通行密钥已删除");
          } else {
            const credentials = window.PublicKeyCredential as CredentialApi | undefined;
            if (!credentials?.parseCreationOptionsFromJSON) throw new Error("当前浏览器不支持在这里添加通行密钥");
            const begin = await api("/settings/passkeys/register/begin", passkeyBegin, "POST", {password});
            const publicKey = credentials.parseCreationOptionsFromJSON(begin.options.publicKey ?? begin.options);
            const credential = await navigator.credentials.create({publicKey}) as (PublicKeyCredential & {toJSON(): unknown}) | null;
            if (!credential) throw new Error("没有创建通行密钥");
            await api("/settings/passkeys/register/finish", empty, "POST", {session_token: begin.session_token, name: String(data.get("name") ?? "").trim(), credential: credential.toJSON()});
            toast("通行密钥已添加");
          }
          close();
          revise(n => n + 1);
        });
      }}>
        {(task.kind === "add" || task.kind === "rename") && (
          <Field label="设备名称">
            <Input name="name" maxLength={100} defaultValue={task.kind === "rename" ? task.name : ""} placeholder="例如：办公电脑" required={task.kind === "rename"} />
          </Field>
        )}
        {task.kind !== "rename" && <Field label="当前密码"><PasswordInput name="password" autoComplete="current-password" required /></Field>}
        <ErrorLine value={action.error} />
      </Form>
    </Overlay>}
  </>;
}

function Identities({items, onChanged}: {items: Profile["identities"]; onChanged: () => void}) {
  const action = useAction();
  const [removing, setRemoving] = useState<Profile["identities"][number]>();
  return <>
    <InsetGroup footer="绑定在授权页完成后自动回到此处。">
      {items.map(item => <InsetRow key={item.provider} label={item.label} hint={item.bound ? item.display || "已绑定" : "未绑定"}>
        {item.bound
          ? item.can_unbind && <Button size="sm" variant="destructive" onClick={() => { action.clearError(); setRemoving(item); }}>解除绑定</Button>
          : item.can_bind && <Button size="sm" variant="outline" busy={action.busy} onClick={() => void action.run(async () => {
            location.assign((await api(`/settings/identities/${item.provider}/bind`, z.object({url: z.string().min(1)}), "POST", {})).url);
          })}>绑定</Button>}
      </InsetRow>)}
      {!items.length && <InsetRow label="站点没有开启第三方登录" />}
    </InsetGroup>
    {!removing && <ErrorLine value={action.error} />}
    <ConfirmDialog open={!!removing} title={`解除 ${removing?.label ?? ""} 绑定`} description="解除后不能再用它登录这个账号。" confirm="解除绑定"
      busy={action.busy} error={action.error} onClose={() => setRemoving(undefined)} onConfirm={() => void action.run(async () => {
        await api(`/settings/identities/${removing!.provider}`, empty, "DELETE");
        toast("已解除第三方账号绑定");
        setRemoving(undefined);
        onChanged();
      })} />
  </>;
}

function Notify({profile, onChanged}: {profile: Profile; onChanged: () => void}) {
  const action = useAction();
  const [threshold, setThreshold] = useState(profile.notify_threshold === null ? "" : String(profile.notify_threshold));
  const [pending, setPending] = useState("");
  const run = (work: () => Promise<unknown>, okMsg?: string) => void action.run(async () => {
    await work();
    if (okMsg) toast(okMsg);
    onChanged();
  });
  if (!profile.notify_available) return <InsetGroup><InsetRow label="站点没有开启余额不足提醒" /></InsetGroup>;
  return <>
    <FormSection title="余额不足提醒" description="余额低于阈值时发邮件到账号邮箱和下面的额外邮箱。">
      <InsetRow label="开启提醒"><Switch label="余额不足提醒" checked={profile.notify_enabled} disabled={action.busy}
        onChange={next => run(() => api("/settings/profile", profileSchema, "PUT", {balance_notify_enabled: next}), next ? "已开启余额提醒" : "已关闭余额提醒")} /></InsetRow>
      {profile.notify_enabled && <Field label="阈值" hint="留空使用站点默认值">
        <InlineStack>
          <div className="row-field"><Input aria-label="提醒阈值" type="number" min="0" step="any" suffix="USD" value={threshold} onChange={event => setThreshold(event.target.value)} /></div>
          <Button size="sm" variant="outline" disabled={threshold === "" || Number(threshold) === profile.notify_threshold} busy={action.busy}
            onClick={() => run(() => api("/settings/profile", profileSchema, "PUT", {balance_notify_threshold: Number(threshold)}), "提醒阈值已保存")}>保存</Button>
        </InlineStack>
      </Field>}
    </FormSection>
    {profile.notify_enabled && <FormSection title="额外通知邮箱">
      {profile.notify_emails.map(item => <InsetRow key={item.email} label={item.email} hint={item.verified ? undefined : "未验证"}>
        {item.verified && <Switch label={`向 ${item.email} 发送提醒`} checked={!item.disabled} disabled={action.busy}
          onChange={next => run(() => api("/settings/notify-emails/toggle", empty, "PUT", {email: item.email, disabled: !next}))} />}
        <Button size="sm" variant="destructive" disabled={action.busy} onClick={() => run(() => api("/settings/notify-emails", empty, "DELETE", {email: item.email}), "已移除通知邮箱")}>移除</Button>
      </InsetRow>)}
      <FormSection title="添加邮箱">
        <Form busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
          const data = new FormData(event.currentTarget);
          const email = String(data.get("email") ?? "").trim();
          if (pending !== email) {
            return void action.run(async () => {
              await api("/settings/notify-emails/send-code", empty, "POST", {email});
              setPending(email);
              toast("验证码已发送至该邮箱");
            });
          }
          run(async () => {
            await api("/settings/notify-emails/verify", empty, "POST", {email, code: data.get("code")});
            setPending("");
          }, "已添加额外通知邮箱");
        }}>
          <FormGrid>
            <Field label="额外通知邮箱"><Input name="email" type="email" aria-label="额外通知邮箱" placeholder="name@example.com" required onChange={() => setPending("")} /></Field>
            {pending && <Field label="邮箱验证码"><OtpInput name="code" aria-label="邮箱验证码" required /></Field>}
          </FormGrid>
          <FormActions><Button type="submit" variant="outline" busy={action.busy}>{pending ? "验证并添加" : "发送验证码"}</Button></FormActions>
        </Form>
      </FormSection>
    </FormSection>}
    <ErrorLine value={action.error} />
  </>;
}

function Appearance() {
  const [theme, setTheme] = useTheme();
  return <FormSection>
    <Field label="主题">
      <Segmented label="主题" value={theme} onChange={setTheme}
        options={[["light", themeNames.light], ["dark", themeNames.dark], ["system", themeNames.system]] as const} />
    </Field>
    <InsetRow label="当前"><Badge variant="tag">{themeNames[theme]}</Badge></InsetRow>
  </FormSection>;
}
