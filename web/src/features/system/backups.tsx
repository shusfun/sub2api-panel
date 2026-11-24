import {useEffect, useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {
  Button, Checkbox, ConfirmDialog, DataTable, EllipsisText, EmptyState, ErrorLine, Field, Form, FormGrid, InlineStack, Input,
  InsetGroup, InsetRow, Overlay, PasswordInput, RefreshButton, Resource, SaveBar, Status, StepUpDialog, Switch, TagInput,
  WorkspaceContent, toast, type Column,
} from "@/ui";

const base = "/admin/backups";
const rowSchema = z.object({
  id: z.string(), status: z.string(), file_name: z.string(), size_bytes: z.number(), started_at: z.string(),
  expires_at: z.string(), progress: z.string(), restore_status: z.string(),
});
const storageSchema = z.object({
  endpoint: z.string(), region: z.string(), bucket: z.string(), access_key_id: z.string(), prefix: z.string(), force_path_style: z.boolean(),
});
const archiveSchema = z.object({
  enabled: z.boolean(), days: z.array(z.number()).nullable(), include_month_end: z.boolean(), retain_count: z.number(),
});
const scheduleSchema = z.object({
  enabled: z.boolean(), cron_expr: z.string(), retain_days: z.number(), retain_count: z.number(), monthly_archive: archiveSchema.optional(),
});
const downloadSchema = z.object({
  url: z.string().optional(), parts: z.array(z.object({index: z.number(), size_bytes: z.number(), url: z.string()})).optional(),
});
type Row = z.infer<typeof rowSchema>;
type Dialog = {kind: "create"} | {kind: "restore" | "delete" | "download"; row: Row};
const bytes = new Intl.NumberFormat("zh-CN", {style: "unit", unit: "megabyte", maximumFractionDigits: 2});

export function BackupWorkspace({stepUpEnabled = true}: {stepUpEnabled?: boolean}) {
  const [revision, revise] = useState(0);
  const [dialog, setDialog] = useState<Dialog>();
  const [archived, setArchived] = useState(false);
  const [expireDays, setExpireDays] = useState("14");
  const action = useAction();
  const refresh = () => revise(v => v + 1);
  const list = useResource(() => api(base, z.array(rowSchema)), base, true, revision);
  const storage = useResource(() => api(base + "/s3-config", storageSchema), base + "/s3-config");
  const schedule = useResource(() => api(base + "/schedule", scheduleSchema), base + "/schedule");
  const pending = list?.value?.some(row => row.status === "pending" || row.status === "running" || row.restore_status === "running");

  useEffect(() => {
    if (!pending || dialog) return;
    const timer = window.setInterval(() => { if (!document.hidden) revise(v => v + 1); }, 5000);
    return () => window.clearInterval(timer);
  }, [pending, dialog]);

  const open = (value: Dialog) => { action.clearError(); setArchived(false); setDialog(value); };
  const done = (message: string) => { setDialog(undefined); toast(message); refresh(); };

  const columns: Column<Row>[] = [
    {id: "file", title: "备份文件", render: row => <EllipsisText text={row.file_name || row.id} mono />},
    {id: "status", title: "备份状态", align: "center", render: row => <Status value={row.progress || row.status} />},
    {id: "restore", title: "恢复状态", align: "center", hideWhenCompact: true, render: row => row.restore_status ? <Status value={row.restore_status} /> : <span className="muted">—</span>},
    {id: "size", title: "大小", align: "number", hideWhenCompact: true, render: row => row.size_bytes ? bytes.format(row.size_bytes / 1_000_000) : "—"},
    {id: "created", title: "开始时间", hideWhenNarrow: true, render: row => row.started_at ? date(row.started_at) : "—"},
    {id: "expires", title: "到期时间", hideWhenNarrow: true, render: row => row.expires_at ? date(row.expires_at) : "长期"},
    {id: "operations", title: "操作", align: "end", render: row => <InlineStack>
      {row.status === "completed" && <>
        <Button size="sm" variant="ghost" onClick={() => open({kind: "download", row})}>下载</Button>
        <Button size="sm" variant="destructive" disabled={row.restore_status === "running"} onClick={() => open({kind: "restore", row})}>恢复</Button>
      </>}
      <Button size="sm" variant="ghost" disabled={row.status === "pending" || row.status === "running" || row.restore_status === "running"}
        onClick={() => open({kind: "delete", row})}>删除</Button>
    </InlineStack>},
  ];

  return <WorkspaceContent>
    <InsetGroup
      title="官方数据库备份"
      actions={
        <InlineStack>
          <RefreshButton size="sm" busy={list?.loading} onClick={refresh} />
          <Button size="sm" onClick={() => open({kind: "create"})}>创建备份</Button>
        </InlineStack>
      }
    >
      <Resource state={list}>{rows => rows.length
        ? <DataTable rows={rows} label="备份记录" columns={columns} rowID={row => row.id} />
        : <EmptyState title="暂无备份" description="配置下方对象存储后即可创建第一份数据库备份。" />}</Resource>
    </InsetGroup>

    <Resource state={storage}>{value => <StorageForm initial={value} stepUp={stepUpEnabled} />}</Resource>
    <Resource state={schedule}>{value => <ScheduleForm initial={value} />}</Resource>

    {dialog?.kind === "create" && (
      <StepUpDialog
        title="创建数据库备份"
        requireTotp={stepUpEnabled}
        confirmLabel="开始备份"
        busy={action.busy}
        error={action.error}
        fieldErrors={action.fieldErrors}
        onClose={() => setDialog(undefined)}
        onConfirm={({totp}) =>
          void action.run(async () => {
            await api(base, rowSchema, "POST", {expire_days: Number(expireDays), ...(totp ? {totp} : {})});
            done("官方已开始创建数据库备份");
          })
        }
      >
        <Field label="保留期限" hint="填 0 表示永久保留">
          <Input
            name="expire_days"
            type="number"
            suffix="天"
            min={0}
            max={36500}
            value={expireDays}
            onChange={e => setExpireDays(e.currentTarget.value)}
            required
          />
        </Field>
      </StepUpDialog>
    )}

    {dialog?.kind === "restore" && (
      <StepUpDialog
        title="恢复官方数据库"
        description={`将使用备份文件「${dialog.row.file_name || dialog.row.id}」完整覆盖官方数据库，期间服务会短暂中断。`}
        requireTotp={stepUpEnabled}
        requirePassword
        critical
        confirmLabel="确认覆盖恢复"
        busy={action.busy}
        error={action.error}
        fieldErrors={action.fieldErrors}
        onClose={() => setDialog(undefined)}
        onConfirm={({totp, password}) =>
          void action.run(async () => {
            await api(base + "/" + dialog.row.id + "/restore", rowSchema, "POST", {password, ...(totp ? {totp} : {})});
            done("官方已开始恢复数据库");
          })
        }
      />
    )}

    {dialog?.kind === "delete" && (
      <ConfirmDialog
        open
        title="删除备份记录"
        description={`将删除备份「${dialog.row.file_name || dialog.row.id}」。`}
        confirm="删除备份"
        busy={action.busy}
        error={action.error}
        onClose={() => setDialog(undefined)}
        onConfirm={() =>
          void action.run(async () => {
            await api(base + "/" + dialog.row.id + (archived ? "?delete_archived=true" : ""), empty, "DELETE");
            done("备份已删除");
          })
        }
      >
        <Checkbox label="同时删除月度归档副本" checked={archived} onChange={setArchived}>同时删除月度归档副本</Checkbox>
      </ConfirmDialog>
    )}

    {dialog?.kind === "download" && <BackupDownload row={dialog.row} close={() => setDialog(undefined)} stepUp={stepUpEnabled} />}
  </WorkspaceContent>;
}

function BackupDownload({row, close, stepUp}: {row: Row; close: () => void; stepUp: boolean}) {
  const path = base + "/" + row.id + "/download-url";
  const [links, setLinks] = useState<z.infer<typeof downloadSchema>>();
  const action = useAction();

  if (links) {
    return (
      <Overlay open title="下载备份文件" onClose={close}>
        <InlineStack wrap>
          {links.url && <Button render={<a href={links.url} target="_blank" rel="noopener noreferrer" />}>下载完整备份</Button>}
          {links.parts?.map(part => (
            <Button variant="outline" key={part.index} render={<a href={part.url} target="_blank" rel="noopener noreferrer" />}>
              下载分片 {part.index}（{bytes.format(part.size_bytes / 1_000_000)}）
            </Button>
          ))}
        </InlineStack>
      </Overlay>
    );
  }

  return (
    <StepUpDialog
      title="获取短期下载链接"
      labels={[row.file_name || row.id]}
      requireTotp={stepUp}
      confirmLabel="获取下载链接"
      busy={action.busy}
      error={action.error}
      fieldErrors={action.fieldErrors}
      onClose={close}
      onConfirm={({totp}) =>
        void action.run(async () => {
          setLinks(await api(path, downloadSchema, "POST", totp ? {totp} : {}));
        })
      }
    />
  );
}

const storageFields = [
  ["endpoint", "端点地址"], ["region", "存储区域"], ["bucket", "存储桶"], ["access_key_id", "访问密钥 ID"], ["prefix", "对象前缀"],
] as const;

function StorageForm({initial, stepUp}: {initial: z.infer<typeof storageSchema>; stepUp: boolean}) {
  const formID = useId();
  const [values, setValues] = useState(() => ({...initial, secret_access_key: ""}));
  const [confirming, setConfirming] = useState(false);
  const action = useAction();

  const dirty =
    storageFields.some(([k]) => values[k] !== initial[k]) ||
    values.force_path_style !== initial.force_path_style ||
    values.secret_access_key !== "";

  const payload = () => ({
    endpoint: values.endpoint.trim(),
    region: values.region.trim(),
    bucket: values.bucket.trim(),
    access_key_id: values.access_key_id.trim(),
    prefix: values.prefix.trim(),
    force_path_style: values.force_path_style,
    ...(values.secret_access_key ? {secret_access_key: values.secret_access_key} : {}),
  });

  const save = (totp?: string) =>
    action.run(async () => {
      const updated = await api(base + "/s3-config", storageSchema, "PUT", {...payload(), ...(totp ? {totp} : {})});
      setValues({...updated, secret_access_key: ""});
      setConfirming(false);
      toast("对象存储配置已保存");
    });

  return (
    <InsetGroup title="对象存储（S3 兼容）">
      <InsetRow label="存储连接参数" vertical>
        <Form
          id={formID}
          busy={action.busy}
          errors={action.fieldErrors}
          onSubmit={() => {
            if (stepUp) {
              action.clearError();
              setConfirming(true);
            } else {
              void save();
            }
          }}
        >
          <FormGrid>
            {storageFields.map(([name, label]) => (
              <Field key={name} label={label} name={name}>
                <Input
                  name={name}
                  value={values[name]}
                  onChange={e => setValues(prev => ({...prev, [name]: e.currentTarget.value}))}
                />
              </Field>
            ))}
            <Field label="访问密钥 Secret" name="secret_access_key" hint="留空保持已保存的密钥不变">
              <PasswordInput
                name="secret_access_key"
                toggleLabel="访问密钥"
                autoComplete="new-password"
                value={values.secret_access_key}
                onChange={e => setValues(prev => ({...prev, secret_access_key: e.currentTarget.value}))}
              />
            </Field>
          </FormGrid>
          <InsetRow label="使用路径式寻址（Path-Style）">
            <Switch
              label="使用路径式寻址"
              checked={values.force_path_style}
              onChange={force_path_style => setValues(prev => ({...prev, force_path_style}))}
            />
          </InsetRow>
          <SaveBar
            dirty={dirty}
            busy={action.busy}
            error={!confirming ? action.error : ""}
            submitLabel="保存对象存储"
            onReset={() => { setValues({...initial, secret_access_key: ""}); action.clearError(); }}
            extra={
              <Button
                type="button"
                variant="outline"
                busy={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    const result = await api(base + "/s3-config/test", z.object({ok: z.boolean()}), "POST", payload());
                    if (!result.ok) throw new Error("无法访问对象存储，请核对端点、存储桶和凭据。");
                    toast("对象存储连接测试通过");
                  })
                }
              >
                测试连接
              </Button>
            }
          />
        </Form>
      </InsetRow>
      {confirming && (
        <StepUpDialog
          title="确认保存对象存储配置"
          confirmLabel="验证并保存"
          busy={action.busy}
          error={action.error}
          fieldErrors={action.fieldErrors}
          onClose={() => setConfirming(false)}
          onConfirm={({totp}) => void save(totp)}
        />
      )}
    </InsetGroup>
  );
}

function ScheduleForm({initial}: {initial: z.infer<typeof scheduleSchema>}) {
  const [saved, setSaved] = useState(initial);
  const [enabled, setEnabled] = useState(initial.enabled);
  const [cronExpr, setCronExpr] = useState(initial.cron_expr || "0 2 * * *");
  const [retainDays, setRetainDays] = useState(String(initial.retain_days));
  const [retainCount, setRetainCount] = useState(String(initial.retain_count));
  const [archive, setArchive] = useState(initial.monthly_archive?.enabled ?? false);
  const [days, setDays] = useState<string[]>(() => (initial.monthly_archive?.days ?? []).map(String));
  const [monthEnd, setMonthEnd] = useState(initial.monthly_archive?.include_month_end ?? false);
  const [archiveRetain, setArchiveRetain] = useState(String(initial.monthly_archive?.retain_count ?? 0));
  const action = useAction();

  const dirty =
    enabled !== saved.enabled ||
    cronExpr !== saved.cron_expr ||
    Number(retainDays) !== saved.retain_days ||
    Number(retainCount) !== saved.retain_count ||
    archive !== (saved.monthly_archive?.enabled ?? false) ||
    monthEnd !== (saved.monthly_archive?.include_month_end ?? false) ||
    Number(archiveRetain) !== (saved.monthly_archive?.retain_count ?? 0) ||
    days.join(",") !== (saved.monthly_archive?.days ?? []).join(",");

  return (
    <InsetGroup title="定时自动备份">
      <InsetRow label="备份周期与归档规则" vertical>
        <Form
          busy={action.busy}
          errors={action.fieldErrors}
          onSubmit={() => {
            void action.run(async () => {
              const parsedDays = days.map(Number);
              if (archive && parsedDays.some(day => !Number.isInteger(day) || day < 1 || day > 31)) {
                throw new Error("归档日期须为 1 到 31 的整数。");
              }
              const plan = enabled
                ? {
                    enabled,
                    cron_expr: cronExpr.trim(),
                    retain_days: Number(retainDays),
                    retain_count: Number(retainCount),
                    monthly_archive: archive
                      ? {enabled: true, days: [...new Set(parsedDays)], include_month_end: monthEnd, retain_count: Number(archiveRetain)}
                      : {...saved.monthly_archive, enabled: false},
                  }
                : {...saved, enabled};
              const updated = await api(base + "/schedule", scheduleSchema, "PUT", plan);
              setSaved(updated);
              toast("定时备份计划已保存");
            });
          }}
        >
          <InsetRow label="启用定时备份"><Switch label="启用定时备份" checked={enabled} onChange={setEnabled} /></InsetRow>
          {enabled && (
            <>
              <FormGrid cols={3}>
                <Field label="Cron 表达式" hint="例如每天凌晨 2 点：0 2 * * *">
                  <Input name="cron_expr" value={cronExpr} onChange={e => setCronExpr(e.currentTarget.value)} required />
                </Field>
                <Field label="保留天数" hint="0 表示不按天清理">
                  <Input name="retain_days" type="number" min={0} suffix="天" value={retainDays} onChange={e => setRetainDays(e.currentTarget.value)} required />
                </Field>
                <Field label="保留份数" hint="0 表示不限份数">
                  <Input name="retain_count" type="number" min={0} suffix="份" value={retainCount} onChange={e => setRetainCount(e.currentTarget.value)} required />
                </Field>
              </FormGrid>
              <InsetRow label="启用月度归档"><Switch label="启用月度归档" checked={archive} onChange={setArchive} /></InsetRow>
              {archive && (
                <>
                  <FormGrid>
                    <Field label="每月归档日（1–31）">
                      <TagInput
                        value={days}
                        placeholder="输入 1–31 按回车"
                        validate={v => {
                          const n = Number(v);
                          return Number.isInteger(n) && n >= 1 && n <= 31 ? undefined : "请输入 1 到 31 之间的整数日期";
                        }}
                        onChange={setDays}
                      />
                    </Field>
                    <Field label="保留归档份数" hint="0 表示永久保留">
                      <Input name="archive_retain" type="number" min={0} suffix="份" value={archiveRetain} onChange={e => setArchiveRetain(e.currentTarget.value)} required />
                    </Field>
                  </FormGrid>
                  <InsetRow label="每月最后一天自动归档"><Switch label="月底归档" checked={monthEnd} onChange={setMonthEnd} /></InsetRow>
                </>
              )}
            </>
          )}
          <SaveBar
            dirty={dirty}
            busy={action.busy}
            error={action.error}
            submitLabel="保存定时设置"
            onReset={() => {
              setEnabled(saved.enabled);
              setCronExpr(saved.cron_expr || "0 2 * * *");
              setRetainDays(String(saved.retain_days));
              setRetainCount(String(saved.retain_count));
              setArchive(saved.monthly_archive?.enabled ?? false);
              setDays((saved.monthly_archive?.days ?? []).map(String));
              setMonthEnd(saved.monthly_archive?.include_month_end ?? false);
              setArchiveRetain(String(saved.monthly_archive?.retain_count ?? 0));
              action.clearError();
            }}
          />
        </Form>
      </InsetRow>
    </InsetGroup>
  );
}
