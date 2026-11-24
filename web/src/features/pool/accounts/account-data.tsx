import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction} from "@/lib/requests";
import {Button, Checkbox, ErrorLine, Feedback, Field, FilePicker, Form, OtpInput, Overlay, toast} from "@/ui";

const base = "/admin/accounts/data";
const account = z.object({name: z.string(), platform: z.string(), type: z.string(), concurrency: z.number(), priority: z.number(),
  credentials: z.record(z.unknown()).optional(), notes: z.string().nullish(), rate_multiplier: z.number().nullish(),
  expires_at: z.number().nullish(), auto_pause_on_expired: z.boolean().nullish()});
const bundle = z.object({type: z.literal("sub2api-data"), version: z.literal(1), exported_at: z.string(),
  accounts: z.array(account).max(1000), skipped_shadows: z.number().optional()});
const outcome = z.object({account_created: z.number(), account_failed: z.number(), errors: z.array(z.object({name: z.string()})).optional()});

export function AccountData({mode, selected, onClose, onChanged}: {
  mode: "import" | "export"; selected: number[]; onClose: () => void; onChanged: () => void;
}) {
  const formID = useId();
  const action = useAction();
  const [bind, setBind] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const exporting = mode === "export";

  return (
    <Overlay
      open
      title={exporting ? "导出账号配置" : "导入账号配置"}
      onClose={onClose}
      busy={action.busy}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={action.busy}>{exporting ? "下载脱敏配置" : "导入账号"}</Button>
        </>
      }
    >
      <Form
        id={formID}
        busy={action.busy}
        errors={action.fieldErrors}
        onSubmit={event => {
          const values = new FormData(event.currentTarget);
          void action.run(async () => {
            if (exporting) {
              const data = await api(base + "/export", bundle, "POST", {ids: selected, totp: values.get("totp")});
              const uri = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], {type: "application/json"}));
              const link = document.createElement("a");
              link.href = uri;
              link.download = "sub2api-account-config.json";
              link.click();
              setTimeout(() => URL.revokeObjectURL(uri), 1000);
              toast("已导出 " + data.accounts.length + " 个账号" + (data.skipped_shadows ? "，跳过 " + data.skipped_shadows + " 个影子账号" : ""));
              onClose();
              return;
            }
            const file = files[0];
            if (!file || !file.size || file.size > 30 * 1024) throw new Error("请选择 30 KiB 以内的账号 JSON 文件");
            const parsed = bundle.safeParse(JSON.parse(await file.text()));
            if (!parsed.success || parsed.data.accounts.length < 1 || parsed.data.accounts.length > 100) throw new Error("文件应包含 1 到 100 个账号");
            const missing = parsed.data.accounts.find(row => !Object.keys(row.credentials ?? {}).some(key => !["base_url", "model_mapping"].includes(key)));
            if (missing) throw new Error("请在文件中为「" + missing.name + "」补录凭据后重试。");
            const data = await api(base + "/import", outcome, "POST", {data: parsed.data, skip_default_group_bind: !bind});
            onChanged();
            const summary = "已创建 " + data.account_created + " 个账号" + (data.account_failed ? `，失败 ${data.account_failed} 个` : "");
            if (data.account_failed) {
              const failed = data.errors?.map(item => item.name).filter(Boolean).join("、");
              throw new Error(failed ? summary + "。失败账号：" + failed : summary);
            }
            toast(summary);
            onClose();
          });
        }}
      >
        {exporting ? (
          <>
            <Feedback>{selected.length ? `将导出选中的 ${selected.length} 个账号。` : "将导出全部账号。"} 导出文件已剥离敏感凭据，重新导入前需补录密钥与代理。</Feedback>
            <Field label="验证器 6 位验证码" name="totp">
              <OtpInput name="totp" autoFocus required />
            </Field>
          </>
        ) : (
          <>
            <Field label="账号 JSON 配置文件" name="file">
              <FilePicker
                name="file"
                accept="application/json,.json"
                maxBytes={30 * 1024}
                hint="导出的 JSON 文件需补录 credentials 后上传，最大 30 KiB"
                value={files}
                onChange={setFiles}
                required
              />
            </Field>
            <Checkbox checked={bind} onChange={setBind} label="自动绑定默认调度分组">自动绑定默认调度分组</Checkbox>
          </>
        )}
        <ErrorLine value={action.error} />
      </Form>
    </Overlay>
  );
}
