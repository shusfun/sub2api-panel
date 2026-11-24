import {useEffect, useId, useState} from "react";
import {z} from "zod";
import {api, empty} from "../../api";
import {useAction, useResource} from "../../requests";
import {Badge, Button, CopyValue, ErrorLine, Feedback, Field, Form, InsetRow, OtpInput, Overlay, PasswordInput, QrCode, Resource, toast, useTotpStatus} from "../../ui";

const methodSchema = z.object({method: z.enum(["email", "password"])});
const setupSchema = z.object({qr_code_url: z.string().startsWith("otpauth://totp/"), setup_token: z.string().min(1), countdown: z.number().int().positive()});
const path = "/settings/totp";

export {useTotpStatus};

export function TotpRow() {
  const [revision, revise] = useState(0);
  const status = useTotpStatus(revision);
  const [mode, setMode] = useState<"enable" | "disable">();
  return (
    <>
      <Resource state={status}>
        {value => (
          <InsetRow label="两步验证" hint={!value.feature_enabled ? "站点已关闭两步验证" : "登录、发码授权和敏感设置需要验证器的 6 位代码"}>
            <span role="status"><Badge variant="soft" tone={value.enabled ? "ok" : "neutral"}>{value.enabled ? "已绑定" : "未绑定"}</Badge></span>
            {(value.enabled || value.feature_enabled) && (
              <Button size="sm" variant={value.enabled ? "outline" : "default"} onClick={() => setMode(value.enabled ? "disable" : "enable")}>
                {value.enabled ? "停用验证器" : "绑定验证器"}
              </Button>
            )}
          </InsetRow>
        )}
      </Resource>
      {status?.error && (
        <InsetRow label="读不到两步验证状态">
          <Button size="sm" variant="outline" onClick={() => revise(n => n + 1)}>重试</Button>
        </InsetRow>
      )}
      {mode && (
        <TotpDialog
          mode={mode}
          onClose={() => setMode(undefined)}
          onSaved={() => {
            toast(mode === "enable" ? "已绑定两步验证器" : "已停用两步验证器");
            setMode(undefined);
            revise(n => n + 1);
          }}
        />
      )}
    </>
  );
}

function TotpDialog({mode, onClose, onSaved}: {mode: "enable" | "disable"; onClose: () => void; onSaved: () => void}) {
  const [revision, revise] = useState(0);
  const method = useResource(() => api(path + "/verification-method", methodSchema), "totp-proof", true, revision);
  const [setup, setSetup] = useState<{token: string; uri: string; secret: string; expires: number}>();
  const formID = useId();
  const [expired, setExpired] = useState(false);
  const save = useAction();
  const send = useAction();

  useEffect(() => {
    if (!setup) return;
    const timer = window.setTimeout(() => setExpired(true), Math.max(0, setup.expires - Date.now()));
    return () => window.clearTimeout(timer);
  }, [setup]);

  return (
    <Overlay
      open
      title={mode === "enable" ? "绑定验证器" : "停用验证器"}
      onClose={onClose}
      busy={save.busy || send.busy}
      footer={
        <>
          <Button variant="ghost" disabled={save.busy || send.busy} onClick={onClose}>取消</Button>
          {setup && expired ? (
            <Button onClick={() => { setSetup(undefined); setExpired(false); }}>重新开始</Button>
          ) : (
            (setup || method?.value) && (
              <Button type="submit" form={formID} busy={save.busy} disabled={send.busy}>
                {setup ? "确认绑定" : mode === "enable" ? "生成绑定二维码" : "确认停用"}
              </Button>
            )
          )}
        </>
      }
    >
      {setup ? (
        <Form
          id={formID}
          key="confirm"
          busy={save.busy}
          errors={save.fieldErrors}
          onSubmit={event => {
            if (expired || Date.now() >= setup.expires) return;
            const totp = String(new FormData(event.currentTarget).get("totp") ?? "").trim();
            void save.run(async () => {
              await api(path + "/enable", empty, "POST", {totp, setup_token: setup.token});
              onSaved();
            });
          }}
        >
          <QrCode
            value={setup.uri}
            alt="验证器绑定二维码"
            expired={expired}
            onRefresh={() => { setSetup(undefined); setExpired(false); }}
          />
          {!expired && (
            <>
              <CopyValue label="手动输入密钥" value={setup.secret} />
              <Field label="验证器 6 位验证码" name="totp">
                <OtpInput name="totp" autoFocus required />
              </Field>
            </>
          )}
          <ErrorLine value={save.error} />
        </Form>
      ) : (
        <>
          <Resource state={method}>
            {proof => (
              <Form
                id={formID}
                busy={save.busy}
                errors={save.fieldErrors}
                onSubmit={event => {
                  const fields = new FormData(event.currentTarget);
                  const input = proof.method === "email" ? {email_code: fields.get("proof")} : {password: fields.get("proof")};
                  void save.run(async () => {
                    if (mode === "disable") {
                      await api(path + "/disable", empty, "POST", input);
                      onSaved();
                      return;
                    }
                    const result = await api(path + "/setup", setupSchema, "POST", input);
                    const expires = Date.now() + result.countdown * 1000;
                    setExpired(false);
                    setSetup({
                      token: result.setup_token,
                      uri: result.qr_code_url,
                      secret: new URL(result.qr_code_url).searchParams.get("secret") ?? "",
                      expires,
                    });
                  });
                }}
              >
                {proof.method === "email" ? (
                  <>
                    <Field label="邮箱验证码" name="proof">
                      <OtpInput
                        name="proof"
                        sendBusy={send.busy}
                        onSendCode={async () => {
                          await send.run(async () => {
                            await api(path + "/send-code", empty, "POST");
                            toast("邮箱验证码已发送");
                          });
                        }}
                        required
                      />
                    </Field>
                    <ErrorLine value={send.error} />
                  </>
                ) : (
                  <Field label="当前密码" name="proof">
                    <PasswordInput name="proof" autoComplete="current-password" required />
                  </Field>
                )}
                <ErrorLine value={save.error} />
              </Form>
            )}
          </Resource>
          {method?.error && <Button size="sm" variant="outline" onClick={() => revise(n => n + 1)}>重试</Button>}
        </>
      )}
    </Overlay>
  );
}
