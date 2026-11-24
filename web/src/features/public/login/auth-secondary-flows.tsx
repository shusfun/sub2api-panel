import type {FormEvent, ReactNode} from "react";
import {z} from "zod";
import {api, newBrowserKey, saveBrowserKey} from "@/lib/http";
import {empty, sessionSchema, type Config, type SessionInfo} from "@/schemas";
import {navigatePublic} from "@/app/route";
import {
  Badge, Button, ErrorLine, Feedback, Field, Form, FormSection, Input,
  OtpInput, PasswordInput, Segmented, Stack,
} from "@/ui";
import {
  buildCaptchaFields, countdownSchema, loginSchema, oauthLabels, Verification,
  type BrowserKey, type PendingOAuth,
} from "./auth-helpers";

type ActionState = {
  busy: boolean;
  error: string;
  fieldErrors: Record<string, string>;
  run: (fn: (trace: string) => Promise<void>) => Promise<void>;
};

export function OAuthCallbackView({
  config,
  action,
  oauthPending,
  oauthBindMode,
  setOAuthBindMode,
  credentials,
  setCredentials,
  verifyCode,
  setVerifyCode,
  countdown,
  setCountdown,
  affCode,
  setPending,
  onLogin,
}: {
  config: Config;
  action: ActionState;
  oauthPending?: PendingOAuth;
  oauthBindMode: "create" | "bind";
  setOAuthBindMode: (next: "create" | "bind") => void;
  credentials: {email: string; password: string};
  setCredentials: (fn: (v: {email: string; password: string}) => {email: string; password: string}) => void;
  verifyCode: string;
  setVerifyCode: (v: string) => void;
  countdown: number;
  setCountdown: (v: number) => void;
  affCode: string;
  setPending: (next: {id: string; keys: BrowserKey; devOTP?: string}) => void;
  onLogin: (session: SessionInfo) => void;
}) {
  if (!oauthPending) {
    return (
      <Stack>
        <ErrorLine value={action.error} />
        {!action.error && <Feedback>正在校验授权凭据…</Feedback>}
        <Button variant="outline" onClick={() => navigatePublic("login")}>返回登录</Button>
      </Stack>
    );
  }
  return (
    <Stack>
      <Segmented
        label="关联方式"
        value={oauthBindMode}
        onChange={setOAuthBindMode}
        options={[["create", "创建新账号"], ["bind", "绑定已有账号"]] as const}
      />
      {oauthBindMode === "create" ? (
        <Form
          className="login-form"
          busy={action.busy}
          errors={action.fieldErrors}
          onSubmit={e => {
            e.preventDefault();
            void action.run(async () => {
              const keys = await newBrowserKey();
              const {challenge} = await api("/session/challenge", z.object({challenge: z.string()}), "POST", undefined, false);
              await api("/session/oauth/pending/create", empty, "POST", {
                challenge,
                browser_key: keys.public,
                pending_auth_token: oauthPending.pending_auth_token,
                email: credentials.email.trim() || undefined,
                verify_code: verifyCode.trim() || undefined,
                password: credentials.password || undefined,
                aff_code: affCode.trim() || undefined,
              }, keys.private);
              await saveBrowserKey(keys.private);
              window.history.replaceState(null, "", "/");
              onLogin(await api("/session", sessionSchema));
            });
          }}
        >
          <FormSection plain>
            {(oauthPending.require_email || credentials.email) && (
              <Field label="绑定邮箱">
                <Input
                  type="email"
                  value={credentials.email}
                  onChange={e => setCredentials(v => ({...v, email: e.target.value}))}
                  required={oauthPending.require_email}
                />
              </Field>
            )}
            {config.email_verify && oauthPending.require_email && (
              <Field label="邮箱验证码">
                <OtpInput
                  value={verifyCode}
                  onChange={e => setVerifyCode(e.target.value)}
                  countdown={countdown}
                  sendBusy={action.busy}
                  onSendCode={() =>
                    action.run(async () => {
                      const s = await api("/session/oauth/pending/send-code", countdownSchema, "POST", {
                        pending_auth_token: oauthPending.pending_auth_token,
                        email: credentials.email.trim(),
                      }, false);
                      setCountdown(s.countdown);
                    })
                  }
                  required
                />
              </Field>
            )}
          </FormSection>
          <ErrorLine value={action.error} />
          <Button type="submit" size="lg" busy={action.busy}>完成创建并进入</Button>
        </Form>
      ) : (
        <Form
          className="login-form"
          busy={action.busy}
          errors={action.fieldErrors}
          onSubmit={e => {
            e.preventDefault();
            void action.run(async () => {
              const keys = await newBrowserKey();
              const {challenge} = await api("/session/challenge", z.object({challenge: z.string()}), "POST", undefined, false);
              const res = await api("/session/oauth/pending/bind", loginSchema, "POST", {
                challenge,
                browser_key: keys.public,
                pending_auth_token: oauthPending.pending_auth_token,
                email: credentials.email.trim(),
                password: credentials.password,
              }, keys.private);
              if (res.pending) {
                setPending({id: res.pending, keys});
                return;
              }
              await saveBrowserKey(keys.private);
              window.history.replaceState(null, "", "/");
              onLogin(await api("/session", sessionSchema));
            });
          }}
        >
          <FormSection plain>
            <Field label="已有账号邮箱">
              <Input type="email" value={credentials.email} onChange={e => setCredentials(v => ({...v, email: e.target.value}))} required />
            </Field>
            <Field label="已有账号密码">
              <PasswordInput value={credentials.password} onChange={e => setCredentials(v => ({...v, password: e.target.value}))} required />
            </Field>
          </FormSection>
          <ErrorLine value={action.error} />
          <Button type="submit" size="lg" busy={action.busy}>绑定并登录</Button>
        </Form>
      )}
    </Stack>
  );
}

export function PasswordRecoveryView({
  mode,
  config,
  action,
  credentials,
  setCredentials,
  confirmPwd,
  setConfirmPwd,
  showPwd,
  setShowPwd,
  proof,
  setProof,
  revision,
  resetCaptcha,
  resetToken,
  forgotDone,
  setForgotDone,
  resetDone,
  setResetDone,
}: {
  mode: "forgot-password" | "reset-password";
  config: Config;
  action: ActionState;
  credentials: {email: string; password: string};
  setCredentials: (fn: (v: {email: string; password: string}) => {email: string; password: string}) => void;
  confirmPwd: string;
  setConfirmPwd: (v: string) => void;
  showPwd: boolean;
  setShowPwd: (v: boolean) => void;
  proof: {token: string; randstr: string};
  setProof: (v: {token: string; randstr: string}) => void;
  revision: number;
  resetCaptcha: () => void;
  resetToken: string;
  forgotDone: boolean;
  setForgotDone: (v: boolean) => void;
  resetDone: boolean;
  setResetDone: (v: boolean) => void;
}) {
  if (mode === "forgot-password") {
    return forgotDone ? (
      <Stack>
        <Feedback tone="success">如果该邮箱已注册，密码重置链接已发出，请查收邮件。</Feedback>
        <Button size="lg" variant="outline" onClick={() => navigatePublic("login")}>返回登录</Button>
      </Stack>
    ) : (
      <Form
        className="login-form"
        busy={action.busy}
        errors={action.fieldErrors}
        onSubmit={e => {
          e.preventDefault();
          void action.run(async () => {
            if (config.captcha && !proof.token) throw new Error("请先完成人机验证。");
            await api("/session/forgot-password", empty, "POST", {
              email: credentials.email.trim(),
              ...buildCaptchaFields(config, proof),
            }, false);
            resetCaptcha();
            setForgotDone(true);
          });
        }}
      >
        <FormSection plain>
          <Field label="注册邮箱">
            <Input type="email" autoComplete="email" value={credentials.email} onChange={e => setCredentials(v => ({...v, email: e.target.value}))} required />
          </Field>
        </FormSection>
        {config.captcha && <Verification key={revision} config={config.captcha} onProof={(token, randstr) => setProof({token, randstr})} />}
        <ErrorLine value={action.error} />
        <Button type="submit" size="lg" busy={action.busy}>发送重置邮件</Button>
      </Form>
    );
  }
  return resetDone ? (
    <Stack>
      <Feedback tone="success">密码已更新，请使用新密码登录。</Feedback>
      <Button size="lg" onClick={() => navigatePublic("login")}>前往登录</Button>
    </Stack>
  ) : (
    <Form
      className="login-form"
      busy={action.busy}
      errors={action.fieldErrors}
      onSubmit={e => {
        e.preventDefault();
        void action.run(async () => {
          if (credentials.password.length < 6) throw new Error("新密码至少需要 6 位。");
          if (credentials.password !== confirmPwd) throw new Error("两次输入的密码不一致。");
          await api("/session/reset-password", empty, "POST", {
            email: credentials.email.trim() || undefined,
            token: resetToken,
            new_password: credentials.password,
          }, false);
          setResetDone(true);
        });
      }}
    >
      <FormSection plain>
        <Field label="新密码">
          <PasswordInput visible={showPwd} onVisibleChange={setShowPwd} autoComplete="new-password" value={credentials.password} onChange={e => setCredentials(v => ({...v, password: e.target.value}))} required />
        </Field>
        <Field label="确认新密码">
          <PasswordInput visible={showPwd} onVisibleChange={setShowPwd} autoComplete="new-password" value={confirmPwd} onChange={e => setConfirmPwd(e.target.value)} required />
        </Field>
      </FormSection>
      <ErrorLine value={action.error} />
      <Button type="submit" size="lg" busy={action.busy}>更新密码</Button>
    </Form>
  );
}

export function RegisterFormView({
  config,
  canSignup,
  action,
  credentials,
  setCredentials,
  confirmPwd,
  setConfirmPwd,
  showPwd,
  setShowPwd,
  inviteCode,
  setInviteCode,
  inviteState,
  affCode,
  setAffCode,
  promoCode,
  setPromoCode,
  promoState,
  agreementControl,
  revision,
  setProof,
  oauthList,
  handleOAuthStart,
  onSubmit,
}: {
  config: Config;
  canSignup: boolean;
  action: ActionState;
  credentials: {email: string; password: string};
  setCredentials: (fn: (v: {email: string; password: string}) => {email: string; password: string}) => void;
  confirmPwd: string;
  setConfirmPwd: (v: string) => void;
  showPwd: boolean;
  setShowPwd: (v: boolean) => void;
  inviteCode: string;
  setInviteCode: (v: string) => void;
  inviteState: "idle" | "valid" | "invalid";
  affCode: string;
  setAffCode: (v: string) => void;
  promoCode: string;
  setPromoCode: (v: string) => void;
  promoState: {status: "idle" | "valid" | "invalid"; bonus?: number};
  agreementControl: ReactNode;
  revision: number;
  setProof: (v: {token: string; randstr: string}) => void;
  oauthList: readonly string[];
  handleOAuthStart: (provider: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  if (!canSignup) {
    return (
      <Stack>
        <Feedback tone="neutral">当前站点已关闭公开注册，请联系管理员获取账号。</Feedback>
        <Button size="lg" variant="outline" onClick={() => navigatePublic("login")}>返回登录</Button>
      </Stack>
    );
  }
  return (
    <Form className="login-form" busy={action.busy} errors={action.fieldErrors} onSubmit={onSubmit}>
      <FormSection plain>
        <Field label="邮箱">
          <Input type="email" autoComplete="email" value={credentials.email} onChange={e => setCredentials(v => ({...v, email: e.target.value}))} required />
        </Field>
        <Field label="密码（至少 6 位）">
          <PasswordInput visible={showPwd} onVisibleChange={setShowPwd} autoComplete="new-password" value={credentials.password} onChange={e => setCredentials(v => ({...v, password: e.target.value}))} required />
        </Field>
        <Field label="确认密码">
          <PasswordInput visible={showPwd} onVisibleChange={setShowPwd} autoComplete="new-password" value={confirmPwd} onChange={e => setConfirmPwd(e.target.value)} required />
        </Field>
        {config.invitation ? (
          <Field label="注册邀请码">
            <Input value={inviteCode} onChange={e => setInviteCode(e.target.value)} required />
          </Field>
        ) : config.affiliate ? (
          <Field label="邀请码（选填）">
            <Input value={affCode} onChange={e => setAffCode(e.target.value)} />
          </Field>
        ) : null}
        {inviteState === "valid" && <Badge variant="soft" tone="ok">邀请码有效</Badge>}
        {inviteState === "invalid" && <Badge variant="soft" tone="bad">邀请码无效或已被使用</Badge>}

        {config.promo_code && (
          <Field label="注册优惠码（选填）">
            <Input value={promoCode} onChange={e => setPromoCode(e.target.value)} />
          </Field>
        )}
        {promoState.status === "valid" && (
          <Badge variant="soft" tone="ok">
            优惠码有效{promoState.bonus ? ` · 赠送 $${promoState.bonus.toFixed(2)}` : ""}
          </Badge>
        )}
        {promoState.status === "invalid" && <Badge variant="soft" tone="bad">优惠码无效或已过期</Badge>}
      </FormSection>

      {agreementControl}
      {config.captcha && <Verification key={revision} config={config.captcha} onProof={(token, randstr) => setProof({token, randstr})} />}
      <ErrorLine value={action.error} />
      <Button type="submit" size="lg" busy={action.busy}>
        {config.email_verify ? "发送验证码并继续" : "立即注册"}
      </Button>

      {oauthList.length > 0 && (
        <Stack>
          <div className="auth-divider">或通过第三方直接注册</div>
          <div className="auth-oauth-grid">
            {oauthList.map(provider => (
              <Button key={provider} size="sm" variant="outline" disabled={action.busy} onClick={() => handleOAuthStart(provider)}>
                {provider === "oidc" && config.oidc_provider_name ? config.oidc_provider_name : (oauthLabels[provider] ?? provider)}
              </Button>
            ))}
          </div>
        </Stack>
      )}
    </Form>
  );
}
