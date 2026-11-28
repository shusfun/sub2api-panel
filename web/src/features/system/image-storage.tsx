import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction, useResource} from "@/lib/requests";
import {
  Button, Field, Form, FormGrid, Input, InsetGroup, InsetRow, PasswordInput, Resource, SaveBar, StepUpDialog, Switch, toast,
} from "@/ui";

const path = "/admin/backups/image-storage";
const configSchema = z.object({
  enabled: z.boolean(), reuse_backup_s3: z.boolean(), bucket: z.string(), prefix: z.string(), public_base_url: z.string(),
  presign_expiry_hours: z.number(), max_download_bytes: z.number(), endpoint: z.string(), region: z.string(),
  access_key_id: z.string(), force_path_style: z.boolean(),
});
const stateSchema = z.object({config: configSchema, secret_configured: z.boolean()});
const testSchema = z.object({ok: z.boolean(), message: z.string().optional()});
type Config = z.infer<typeof configSchema>;
type Values = Config & {secret_access_key: string};

const textFields = [
  ["endpoint", "端点地址"], ["region", "存储区域"], ["bucket", "存储桶"], ["access_key_id", "访问密钥 ID"],
  ["prefix", "对象前缀"], ["public_base_url", "公开访问地址（可选）"],
] as const;

export function ImageStorageSection({stepUp}: {stepUp: boolean}) {
  const state = useResource(() => api(path, stateSchema), path);
  return <Resource state={state}>{value => <ImageStorageForm initial={value.config} secretConfigured={value.secret_configured} stepUp={stepUp} />}</Resource>;
}

function ImageStorageForm({initial, secretConfigured, stepUp}: {initial: Config; secretConfigured: boolean; stepUp: boolean}) {
  const formID = useId();
  const [saved, setSaved] = useState(initial);
  const [hasSecret, setHasSecret] = useState(secretConfigured);
  const [values, setValues] = useState<Values>(() => ({...initial, secret_access_key: ""}));
  const [confirming, setConfirming] = useState(false);
  const action = useAction();
  const set = (patch: Partial<Values>) => setValues(prev => ({...prev, ...patch}));

  const dirty = values.secret_access_key !== "" || (Object.keys(saved) as (keyof Config)[]).some(k => values[k] !== saved[k]);

  const payload = () => ({
    enabled: values.enabled,
    reuse_backup_s3: values.reuse_backup_s3,
    bucket: values.bucket.trim(),
    prefix: values.prefix.trim(),
    public_base_url: values.public_base_url.trim(),
    presign_expiry_hours: Math.max(0, Number(values.presign_expiry_hours) || 0),
    max_download_bytes: Math.max(0, Number(values.max_download_bytes) || 0),
    endpoint: values.endpoint.trim(),
    region: values.region.trim(),
    access_key_id: values.access_key_id.trim(),
    force_path_style: values.force_path_style,
    ...(values.secret_access_key ? {secret_access_key: values.secret_access_key} : {}),
  });

  const save = (totp?: string) =>
    action.run(async () => {
      const updated = await api(path, configSchema, "PUT", {...payload(), ...(totp ? {totp} : {})});
      if (values.secret_access_key) setHasSecret(true);
      setSaved(updated);
      setValues({...updated, secret_access_key: ""});
      setConfirming(false);
      toast("生图对象存储配置已保存");
    });

  const own = !values.reuse_backup_s3;

  return (
    <InsetGroup title="生图对象存储">
      <InsetRow label="生成图片的存放位置" vertical>
        <Form
          id={formID}
          busy={action.busy}
          errors={action.fieldErrors}
          onSubmit={() => {
            if (stepUp) { action.clearError(); setConfirming(true); } else void save();
          }}
        >
          <InsetRow label="启用生图对象存储"><Switch label="启用生图对象存储" checked={values.enabled} onChange={enabled => set({enabled})} /></InsetRow>
          <InsetRow label="复用备份的对象存储连接"><Switch label="复用备份存储" checked={values.reuse_backup_s3} onChange={reuse_backup_s3 => set({reuse_backup_s3})} /></InsetRow>
          <FormGrid>
            {textFields.filter(([name]) => own || name === "prefix" || name === "public_base_url").map(([name, label]) => (
              <Field key={name} label={label} name={name}>
                <Input name={name} value={values[name]} onChange={e => set({[name]: e.currentTarget.value})} />
              </Field>
            ))}
            {own && (
              <Field label="访问密钥 Secret" name="secret_access_key" hint={hasSecret ? "已配置，留空保持不变" : "尚未配置"}>
                <PasswordInput
                  name="secret_access_key"
                  toggleLabel="访问密钥"
                  autoComplete="new-password"
                  value={values.secret_access_key}
                  onChange={e => set({secret_access_key: e.currentTarget.value})}
                />
              </Field>
            )}
            <Field label="预签名链接有效期" name="presign_expiry_hours" hint="0 表示使用官方默认值">
              <Input name="presign_expiry_hours" type="number" min={0} suffix="小时" value={String(values.presign_expiry_hours)} onChange={e => set({presign_expiry_hours: Number(e.currentTarget.value)})} />
            </Field>
            <Field label="单次下载上限" name="max_download_bytes" hint="单位字节，0 表示使用官方默认值">
              <Input name="max_download_bytes" type="number" min={0} suffix="字节" value={String(values.max_download_bytes)} onChange={e => set({max_download_bytes: Number(e.currentTarget.value)})} />
            </Field>
          </FormGrid>
          {own && (
            <InsetRow label="使用路径式寻址（Path-Style）">
              <Switch label="使用路径式寻址" checked={values.force_path_style} onChange={force_path_style => set({force_path_style})} />
            </InsetRow>
          )}
          <SaveBar
            dirty={dirty}
            busy={action.busy}
            error={!confirming ? action.error : ""}
            submitLabel="保存生图存储"
            onReset={() => { setValues({...saved, secret_access_key: ""}); action.clearError(); }}
            extra={
              <Button
                type="button"
                variant="outline"
                busy={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    const result = await api(path + "/test", testSchema, "POST", payload());
                    if (!result.ok) throw new Error(result.message || "无法访问对象存储，请核对配置。");
                    toast("生图对象存储连接测试通过");
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
          title="确认保存生图对象存储配置"
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
