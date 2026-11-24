import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Badge, Button, ErrorLine, Field, Form, FormActions, FormSection, InlineStack, Input, InsetGroup, InsetRow,
  Overlay, RefreshButton, Resource, Select, StepUpDialog, Textarea, toast,
} from "@/ui";
import {AdminAccessKeys} from "./access-keys";

const base = "/admin/settings-tools";
const templatesSchema = z.object({
  events: z.array(z.object({value: z.string(), label: z.string(), category: z.string()})),
  locales: z.array(z.string()),
  templates: z.array(z.object({event: z.string(), locale: z.string(), subject: z.string()})),
});
const templateSchema = z.object({subject: z.string(), html: z.string(), placeholders: z.array(z.string()).nullable().optional()});
const previewSchema = z.object({subject: z.string(), html: z.string()});
const connectionSchema = z.object({upstream: z.string(), official: z.string(), origin: z.string(), account_stats: z.boolean()});

export function SectionTools({id, draft = {}}: {id: string; draft?: Record<string, unknown>}) {
  if (id === "mail") return <MailTools draft={draft} />;
  if (id === "access-keys") return <AdminAccessKeys />;
  if (id === "connection") return <><Connection /><SystemTools /></>;
  return null;
}

const smtpDraftKeys = ["smtp_host", "smtp_port", "smtp_username", "smtp_password", "smtp_from_email", "smtp_from_name", "smtp_use_tls"] as const;

function pickSmtpDraft(draft: Record<string, unknown>) {
  const out: Record<string, unknown> = {};
  for (const key of smtpDraftKeys) {
    if (key in draft && draft[key] !== "" && draft[key] !== undefined) {
      out[key] = draft[key];
    }
  }
  return out;
}

function MailTools({draft}: {draft: Record<string, unknown>}) {
  const action = useAction();
  const [editing, setEditing] = useState<{event: string; locale: string}>();
  const [locale, setLocale] = useState("zh");
  const templates = useResource(() => api(base + "/mail/templates", templatesSchema), "mail-templates", true);
  const smtpOverrides = pickSmtpDraft(draft);
  const hasDraftOverrides = Object.keys(smtpOverrides).length > 0;
  return <>
    <FormSection
      title="发信连通性"
      description={hasDraftOverrides ? "测试将自动附带上方未保存的发信参数草稿。" : "使用已保存的发信配置；修改上方参数后可直接测试。"}
    >
      <InsetRow label="测试服务器连接"><Button variant="outline" size="sm" busy={action.busy} onClick={() => void action.run(async () => {
        await api(base + "/mail/test-connection", empty, "POST", smtpOverrides);
        toast("发信服务器连接正常");
      })}>测试连接</Button></InsetRow>
      <FormSection title="发送测试邮件">
        <Form busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
          const email = new FormData(event.currentTarget).get("email");
          void action.run(async () => {
            await api(base + "/mail/send-test", empty, "POST", {...smtpOverrides, email});
            toast(`测试邮件已发往 ${String(email)}`);
          });
        }}>
          <Field label="收件邮箱">
            <Input
              name="email"
              type="email"
              placeholder="name@example.com"
              required
              endAddon={<Button type="submit" size="xs" variant="outline" busy={action.busy}>发送</Button>}
            />
          </Field>
        </Form>
      </FormSection>
    </FormSection>
    <ErrorLine value={action.error} />
    <Resource state={templates}>{value => (
      <InsetGroup title="邮件模板" actions={value.locales.length > 1 && (
        <Select aria-label="模板语言" value={locale} onChange={event => setLocale(event.target.value)}>
          {value.locales.map(item => <option key={item} value={item}>{item}</option>)}
        </Select>
      )}>
        {value.events.map(event => <InsetRow key={event.value} label={event.label}
          hint={value.templates.find(item => item.event === event.value && item.locale === locale)?.subject}
          onClick={() => setEditing({event: event.value, locale})} />)}
      </InsetGroup>
    )}</Resource>
    {editing && <TemplateDialog key={editing.event + editing.locale} {...editing} onClose={() => setEditing(undefined)} />}
  </>;
}

function TemplateDialog({event, locale, onClose}: {event: string; locale: string; onClose: () => void}) {
  const formID = useId();
  const path = `${base}/mail/templates/${encodeURIComponent(event)}/${encodeURIComponent(locale)}`;
  const [revision, revise] = useState(0);
  const template = useResource(() => api(path, templateSchema), "mail-template:" + path, true, revision);
  const [preview, setPreview] = useState<z.infer<typeof previewSchema>>();
  const action = useAction();
  const read = (form: HTMLFormElement) => { const data = new FormData(form); return {subject: data.get("subject"), html: data.get("html")}; };
  return <Overlay open title="编辑邮件模板" onClose={onClose} busy={action.busy} footer={template?.value &&
    <FormActions>
      <Button variant="outline" busy={action.busy} onClick={() => void action.run(async () => {
        await api(path + "/restore", empty, "POST", {});
        setPreview(undefined);
        toast("已恢复官方默认邮件模板");
        revise(n => n + 1);
      })}>恢复官方模板</Button>
      <Button variant="outline" busy={action.busy} onClick={() => {
        const form = document.getElementById(formID);
        if (form instanceof HTMLFormElement) void action.run(async () => setPreview(await api(path + "/preview", previewSchema, "POST", read(form))));
      }}>预览</Button>
      <Button type="submit" form={formID} busy={action.busy}>保存</Button>
    </FormActions>
  }>
    <Resource state={template}>{value => (
      <Form id={formID} key={revision} busy={action.busy} errors={action.fieldErrors} onSubmit={submit => void action.run(async () => {
        await api(path, empty, "PUT", read(submit.currentTarget));
        toast("邮件模板已保存");
        onClose();
      })}>
        <Field label="邮件标题"><Input name="subject" defaultValue={value.subject} maxLength={500} required /></Field>
        <Field label="正文（HTML）"><Textarea name="html" rows={13} defaultValue={value.html} required /></Field>
        {!!value.placeholders?.length && <div className="muted text-fine">可用占位符：{value.placeholders.map(name => `{{${name}}}`).join("　")}</div>}
        {preview && <InsetGroup title={"预览：" + preview.subject}>
          <iframe className="setting-preview" title="邮件预览" sandbox="" srcDoc={preview.html} />
        </InsetGroup>}
        <ErrorLine value={action.error} />
      </Form>
    )}</Resource>
  </Overlay>;
}

function Connection() {
  const info = useResource(() => api(base + "/connection", connectionSchema), "setting-connection", true);
  return <Resource state={info}>{value => (
    <InsetGroup footer="运行期连接地址由面板启动配置只读固定。">
      <InsetRow label="上游接口"><span className="code-mono">{value.upstream}</span></InsetRow>
      <InsetRow label="官方站地址"><span className="code-mono">{value.official}</span></InsetRow>
      <InsetRow label="面板地址"><span className="code-mono">{value.origin}</span></InsetRow>
      <InsetRow label="同机只读官方库" hint="用于账号当日统计、智能巡检活跃流量及模型分时趋势加速">
        <Badge variant="soft" tone={value.account_stats ? "ok" : "neutral"}>{value.account_stats ? "已直连" : "未配置"}</Badge>
      </InsetRow>
    </InsetGroup>
  )}</Resource>;
}

const versionSchema = z.object({panel_version: z.string(), panel_commit: z.string(), upstream: z.object({current_version: z.string(), latest_version: z.string(), has_update: z.boolean(), build_type: z.string()}).optional()});
const systemActions = {update: "更新官方上游", rollback: "回滚官方上游", restart: "重启官方上游"} as const;

function SystemTools() {
  const [revision, revise] = useState(0);
  const state = useResource(() => api(base + "/system", versionSchema), "system-versions", true, revision);
  const [task, setTask] = useState<keyof typeof systemActions>();
  const action = useAction();
  return <InsetGroup title="系统信息" actions={<RefreshButton size="xs" variant="ghost" busy={state?.loading} label="检查版本" onClick={() => revise(n => n + 1)} />}>
    <Resource state={state}>{value => <>
      <InsetRow label="面板版本" hint={value.panel_commit}>{value.panel_version}</InsetRow>
      <InsetRow label="官方上游版本" hint={value.upstream?.latest_version ? "可用版本 " + value.upstream.latest_version : undefined}>{value.upstream?.current_version || "暂不可用"}</InsetRow>
      {value.upstream && <InsetRow label="上游生命周期维护"><InlineStack wrap>
        {(Object.entries(systemActions) as [keyof typeof systemActions, string][]).map(([id, label]) => (
          <Button
            size="sm"
            variant={id === "restart" || id === "rollback" ? "destructive" : "outline"}
            key={id}
            disabled={id === "update" && !value.upstream?.has_update}
            onClick={() => { action.clearError(); setTask(id); }}
          >
            {label}
          </Button>
        ))}
      </InlineStack></InsetRow>}
    </>}</Resource>
    {task && (
      <StepUpDialog
        title={systemActions[task]}
        labels={[systemActions[task] + "（执行期间上游网关会短暂中断请求）"]}
        critical={task === "restart" || task === "rollback"}
        confirmLabel={systemActions[task]}
        busy={action.busy}
        error={action.error}
        fieldErrors={action.fieldErrors}
        onClose={() => setTask(undefined)}
        onConfirm={({totp}) => void action.run(async () => {
          const result = await api(base + "/system/" + task, z.object({need_restart: z.boolean().optional()}), "POST", {totp});
          toast(systemActions[task] + "已完成" + (result.need_restart ? "，重启后生效" : ""));
          setTask(undefined);
          revise(n => n + 1);
        })}
      />
    )}
  </InsetGroup>;
}
