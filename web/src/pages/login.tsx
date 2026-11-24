import {useEffect, useRef, useState, type FormEvent} from "react";
import {ArrowLeft, KeyRound} from "lucide-react";
import {z} from "zod";
import {api, empty, newBrowserKey, saveBrowserKey, sessionSchema, type Config, type SessionInfo} from "../api";
import {Brand} from "../appearance";
import {useAction, useDebouncedValue} from "../requests";
import {navigatePublic, type PublicPageID} from "../route";
import {Badge, Button, Checkbox, ErrorLine, Feedback, Field, Form, FormSection, InlineStack, Input, InsetGroup, InsetRow, OtpInput, Overlay, PasswordInput, Segmented, Select, Stack, StageCard} from "../ui";

function Verification({config, onProof}: {config: NonNullable<Config["captcha"]>; onProof: (token: string, randstr: string) => void}) {
  const frame = useRef<HTMLIFrameElement>(null);
  const [channel] = useState(() => crypto.randomUUID());
  useEffect(() => {
    const schema = z.object({channel: z.literal(channel), token: z.string().max(12000), randstr: z.string().max(300)});
    const receive = (event: MessageEvent<unknown>) => {
      if (event.source !== frame.current?.contentWindow || event.origin !== "null") return;
      const result = schema.safeParse(event.data);
      if (result.success) onProof(result.data.token, result.data.randstr);
    };
    window.addEventListener("message", receive);
    return () => window.removeEventListener("message", receive);
  }, [channel, onProof]);
  return (
    <iframe
      className="captcha"
      ref={frame}
      title="人机验证"
      sandbox="allow-scripts allow-forms allow-popups"
      src="/verification.html"
      onLoad={() => frame.current?.contentWindow?.postMessage({channel, options: config}, "*")}
    />
  );
}

const loginSchema = z.object({pending: z.string().optional()});
const countdownSchema = z.object({countdown: z.number()});
const inviteCheckSchema = z.object({valid: z.boolean(), error_code: z.string().optional()});
const promoCheckSchema = z.object({valid: z.boolean(), bonus_amount: z.number().optional(), error_code: z.string().optional()});
const passkeyBeginSchema = z.object({session_token: z.string(), options: z.record(z.string(), z.unknown())});
const oauthStartSchema = z.object({url: z.string()});
const pendingExchangeSchema = z.object({
  pending_auth_token: z.string(),
  provider: z.string(),
  display_name: z.string().optional(),
  email: z.string().optional(),
  require_email: z.boolean(),
});

type BrowserKey = Awaited<ReturnType<typeof newBrowserKey>>;
type DevLoginAccount = NonNullable<Config["dev_accounts"]>[number];

const agreementConsentKey = "panel-login-agreement-rev";

const oauthLabels: Record<string, string> = {
  github: "GitHub",
  google: "Google",
  linuxdo: "LinuxDo",
  wechat: "微信",
  dingtalk: "钉钉",
  oidc: "OIDC",
};

function decodeBase64URL(input: string): Uint8Array {
  const pad = "=".repeat((4 - (input.length % 4)) % 4);
  const bin = atob(input.replace(/-/g, "+").replace(/_/g, "/") + pad);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function encodeBase64URL(buf: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(buf)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

function prepareWebAuthnGetOptions(raw: Record<string, unknown>): PublicKeyCredentialRequestOptions {
  const inner = (raw.publicKey as Record<string, unknown> | undefined) ?? raw;
  const allowRaw = Array.isArray(inner.allowCredentials) ? inner.allowCredentials : [];
  return {
    challenge: decodeBase64URL(String(inner.challenge ?? "")) as BufferSource,
    timeout: typeof inner.timeout === "number" ? inner.timeout : 60000,
    rpId: typeof inner.rpId === "string" ? inner.rpId : undefined,
    userVerification: (inner.userVerification as UserVerificationRequirement | undefined) ?? "preferred",
    allowCredentials: allowRaw.map((item: Record<string, unknown>) => ({
      type: "public-key" as const,
      id: decodeBase64URL(String(item.id ?? "")) as BufferSource,
      transports: Array.isArray(item.transports) ? (item.transports as AuthenticatorTransport[]) : undefined,
    })),
  };
}

function serializeAssertion(cred: PublicKeyCredential) {
  const resp = cred.response as AuthenticatorAssertionResponse;
  return {
    id: cred.id,
    rawId: encodeBase64URL(cred.rawId),
    type: cred.type,
    response: {
      authenticatorData: encodeBase64URL(resp.authenticatorData),
      clientDataJSON: encodeBase64URL(resp.clientDataJSON),
      signature: encodeBase64URL(resp.signature),
      userHandle: resp.userHandle ? encodeBase64URL(resp.userHandle) : null,
    },
  };
}

function buildCaptchaFields(config: Config, proof: {token: string; randstr: string}) {
  if (!config.captcha || !proof.token) return {};
  if (config.captcha.provider === "tencent") {
    return {tencent_captcha_ticket: proof.token, tencent_captcha_randstr: proof.randstr};
  }
  return {turnstile_token: proof.token};
}

export function Login({
  config,
  mode = "login",
  params,
  onLogin,
}: {
  config: Config;
  mode?: Exclude<PublicPageID, "home" | "public-patrol">;
  params?: URLSearchParams;
  onLogin: (session: SessionInfo) => void;
}) {
  const siteName = config.site_name || "Sub2API";
  const initialAff = params?.get("aff") ?? params?.get("aff_code") ?? "";
  const resetToken = params?.get("token") ?? "";
  const resetEmailInit = params?.get("email") ?? "";

  const [agreed, setAgreed] = useState(() => {
    if (!config.agreement) return true;
    if (!config.agreement_revision) return false;
    return localStorage.getItem(agreementConsentKey) === config.agreement_revision;
  });
  const [agreementModal, setAgreementModal] = useState(false);

  const [pending, setPending] = useState<{id: string; keys: BrowserKey; devOTP?: string}>();
  const [credentials, setCredentials] = useState({email: resetEmailInit, password: ""});
  const [confirmPwd, setConfirmPwd] = useState("");
  const [showPwd, setShowPwd] = useState(false);
  const [selectedAccount, setSelectedAccount] = useState<DevLoginAccount>();
  const [proof, setProof] = useState({token: "", randstr: ""});
  const [revision, revise] = useState(0);

  // 注册、邀请码、优惠码与邮箱验证码状态
  const [inviteCode, setInviteCode] = useState(params?.get("invite") ?? "");
  const [promoCode, setPromoCode] = useState("");
  const [affCode, setAffCode] = useState(initialAff);
  const [verifyCode, setVerifyCode] = useState("");
  const [countdown, setCountdown] = useState(0);
  const [inviteState, setInviteState] = useState<"idle" | "valid" | "invalid">("idle");
  const [promoState, setPromoState] = useState<{status: "idle" | "valid" | "invalid"; bonus?: number}>({status: "idle"});
  const [forgotDone, setForgotDone] = useState(false);
  const [resetDone, setResetDone] = useState(false);

  // 第三方 OAuth 首次补全或绑定状态
  const [oauthPending, setOAuthPending] = useState<z.infer<typeof pendingExchangeSchema>>();
  const [oauthBindMode, setOAuthBindMode] = useState<"create" | "bind">("create");

  const debouncedInvite = useDebouncedValue(inviteCode.trim(), 320);
  const debouncedPromo = useDebouncedValue(promoCode.trim(), 320);
  const action = useAction();

  useEffect(() => {
    if (countdown <= 0) return;
    const t = window.setInterval(() => setCountdown(v => Math.max(0, v - 1)), 1000);
    return () => window.clearInterval(t);
  }, [countdown > 0]);

  useEffect(() => {
    if (mode !== "register" || !config.invitation || !debouncedInvite) {
      setInviteState("idle");
      return;
    }
    let cancelled = false;
    void api("/session/validate-invite", inviteCheckSchema, "POST", {code: debouncedInvite}, false)
      .then(res => { if (!cancelled) setInviteState(res.valid ? "valid" : "invalid"); })
      .catch(() => { if (!cancelled) setInviteState("invalid"); });
    return () => { cancelled = true; };
  }, [config.invitation, debouncedInvite, mode]);

  useEffect(() => {
    if (mode !== "register" || !config.promo_code || !debouncedPromo) {
      setPromoState({status: "idle"});
      return;
    }
    let cancelled = false;
    void api("/session/validate-promo", promoCheckSchema, "POST", {code: debouncedPromo}, false)
      .then(res => { if (!cancelled) setPromoState({status: res.valid ? "valid" : "invalid", bonus: res.bonus_amount}); })
      .catch(() => { if (!cancelled) setPromoState({status: "invalid"}); });
    return () => { cancelled = true; };
  }, [config.promo_code, debouncedPromo, mode]);

  // 处理第三方 OAuth 回调：立即清理地址栏令牌片段并密封至服务端内存会话
  useEffect(() => {
    if (mode !== "oauth-callback") return;
    const hashParams = new URLSearchParams(location.hash.replace(/^#/, ""));
    const accessToken = params?.get("access_token") || hashParams.get("access_token") || "";
    const refreshToken = params?.get("refresh_token") || hashParams.get("refresh_token") || "";
    const sessionToken = params?.get("session_token") || params?.get("pending_session_token") || hashParams.get("session_token") || "";
    window.history.replaceState(null, "", "/auth/callback");

    if (accessToken) {
      void action.run(async () => {
        const keys = await newBrowserKey();
        const {challenge} = await api("/session/challenge", z.object({challenge: z.string()}), "POST", undefined, false);
        await api("/session/oauth/complete", empty, "POST", {
          challenge,
          browser_key: keys.public,
          access_token: accessToken,
          refresh_token: refreshToken,
        }, keys.private);
        await saveBrowserKey(keys.private);
        window.history.replaceState(null, "", "/");
        onLogin(await api("/session", sessionSchema));
      });
    } else if (sessionToken) {
      void action.run(async () => {
        const exchanged = await api("/session/oauth/pending/exchange", pendingExchangeSchema, "POST", {session_token: sessionToken}, false);
        setOAuthPending(exchanged);
        if (exchanged.email) setCredentials(prev => ({...prev, email: exchanged.email ?? ""}));
      });
    }
  }, [mode]);

  function resetCaptcha() {
    setProof({token: "", randstr: ""});
    revise(v => v + 1);
  }

  function checkEmailAllowed(email: string): string | undefined {
    const list = config.email_whitelist ?? [];
    if (list.length === 0) return undefined;
    const lower = email.trim().toLowerCase();
    const ok = list.some(suffix => {
      const s = suffix.trim().toLowerCase();
      return s && lower.endsWith(s.startsWith("@") ? s : "@" + s);
    });
    return ok ? undefined : `仅允许以下邮箱后缀注册：${list.join("、")}`;
  }

  function submitLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    void action.run(async () => {
      if (pending) {
        try {
          await api("/session/verify", empty, "POST", {challenge: pending.id, browser_key: pending.keys.public, otp: data.get("otp")}, pending.keys.private);
          await saveBrowserKey(pending.keys.private);
        } catch (error) {
          setPending(undefined);
          throw error;
        }
      } else {
        if (config.agreement && !agreed) throw new Error("请先阅读并同意站点服务协议。");
        if (config.captcha && !proof.token) throw new Error("请先完成人机验证。");
        const keys = await newBrowserKey();
        const {challenge} = await api("/session/challenge", z.object({challenge: z.string()}), "POST", undefined, false);
        let result;
        try {
          result = await api("/session/login", loginSchema, "POST", {
            challenge,
            browser_key: keys.public,
            email: credentials.email.trim(),
            password: credentials.password,
            agreement: agreed,
            ...buildCaptchaFields(config, proof),
          }, keys.private);
        } finally {
          resetCaptcha();
        }
        if (config.agreement && config.agreement_revision) {
          localStorage.setItem(agreementConsentKey, config.agreement_revision);
        }
        if (result.pending) {
          setPending({id: result.pending, keys, devOTP: selectedAccount?.otp});
          return;
        }
        await saveBrowserKey(keys.private);
      }
      window.history.replaceState(null, "", "/");
      onLogin(await api("/session", sessionSchema));
    });
  }

  function handlePasskeyLogin() {
    void action.run(async () => {
      if (config.agreement && !agreed) throw new Error("请先阅读并同意站点服务协议。");
      if (typeof window.PublicKeyCredential === "undefined") throw new Error("当前浏览器不支持通行密钥。");
      const begin = await api("/session/passkey/begin", passkeyBeginSchema, "POST", {
        email: credentials.email.trim() || undefined,
        ...buildCaptchaFields(config, proof),
      }, false);
      const cred = await navigator.credentials.get({publicKey: prepareWebAuthnGetOptions(begin.options)});
      if (!(cred instanceof PublicKeyCredential)) throw new Error("未获取到有效的通行密钥凭据。");
      const keys = await newBrowserKey();
      const {challenge} = await api("/session/challenge", z.object({challenge: z.string()}), "POST", undefined, false);
      const res = await api("/session/passkey/finish", loginSchema, "POST", {
        challenge,
        browser_key: keys.public,
        session_token: begin.session_token,
        credential: serializeAssertion(cred),
        agreement: agreed,
      }, keys.private);
      if (res.pending) {
        setPending({id: res.pending, keys});
        return;
      }
      await saveBrowserKey(keys.private);
      window.history.replaceState(null, "", "/");
      onLogin(await api("/session", sessionSchema));
    });
  }

  function handleOAuthStart(provider: string) {
    void action.run(async () => {
      if (config.agreement && !agreed) throw new Error("请先阅读并同意站点服务协议。");
      const res = await api(`/session/oauth/${provider}/start`, oauthStartSchema, "POST", {
        aff_code: affCode.trim() || undefined,
        promo_code: promoCode.trim() || undefined,
        ...buildCaptchaFields(config, proof),
      }, false);
      location.href = res.url;
    });
  }

  function submitRegister(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void action.run(async () => {
      const suffixErr = checkEmailAllowed(credentials.email);
      if (suffixErr) throw new Error(suffixErr);
      if (credentials.password.length < 6) throw new Error("密码长度至少为 6 位。");
      if (credentials.password !== confirmPwd) throw new Error("两次输入的密码不一致。");
      if (config.agreement && !agreed) throw new Error("请先阅读并同意站点服务协议。");

      if (mode === "register" && config.email_verify) {
        if (config.captcha && !proof.token) throw new Error("请先完成人机验证以发送验证码。");
        const sent = await api("/session/send-verify-code", countdownSchema, "POST", {
          email: credentials.email.trim(),
          ...buildCaptchaFields(config, proof),
        }, false);
        resetCaptcha();
        setCountdown(sent.countdown);
        navigatePublic("email-verify");
        return;
      }

      if (mode === "register" && config.captcha && !proof.token) {
        throw new Error("请先完成人机验证。");
      }
      const keys = await newBrowserKey();
      const {challenge} = await api("/session/challenge", z.object({challenge: z.string()}), "POST", undefined, false);
      try {
        await api("/session/register", empty, "POST", {
          challenge,
          browser_key: keys.public,
          email: credentials.email.trim(),
          password: credentials.password,
          verify_code: verifyCode.trim() || undefined,
          invitation_code: inviteCode.trim() || undefined,
          promo_code: promoCode.trim() || undefined,
          aff_code: affCode.trim() || undefined,
          agreement: agreed,
          ...buildCaptchaFields(config, proof),
        }, keys.private);
      } finally {
        resetCaptcha();
      }
      await saveBrowserKey(keys.private);
      window.history.replaceState(null, "", "/");
      onLogin(await api("/session", sessionSchema));
    });
  }

  const canSignup = config.registration && !config.backend_mode;
  const oauthList = config.backend_mode ? [] : (config.oauth_providers ?? []);
  const passkeyAvailable = config.passkey && typeof window !== "undefined" && typeof window.PublicKeyCredential !== "undefined";

  const agreementControl = config.agreement && (
    <Field label={<span className="sr-only">服务协议</span>}>
      <div className="agreement">
        <Checkbox name="agreement" label="阅读并同意服务协议" checked={agreed} onChange={setAgreed} inline>
          我已阅读并同意
        </Checkbox>
        {config.agreement_docs.length > 0 ? (
          <Button size="xs" variant="link" onClick={() => setAgreementModal(true)}>
            《{config.agreement_docs[0].title}》
          </Button>
        ) : (
          <span>站点服务协议</span>
        )}
      </div>
    </Field>
  );

  const heroTitle = pending
    ? "两步验证"
    : mode === "register"
      ? "创建账号"
      : mode === "email-verify"
        ? "验证邮箱"
        : mode === "forgot-password"
          ? "找回密码"
          : mode === "reset-password"
            ? "设置新密码"
            : mode === "oauth-callback"
              ? "第三方登录连接"
              : "欢迎回来";

  const heroSubtitle = pending
    ? "输入验证器应用中的 6 位动态验证码"
    : mode === "register"
      ? config.site_subtitle || `加入 ${siteName} 开始使用统一网关`
      : mode === "email-verify"
        ? `验证码已发送至 ${credentials.email}`
        : mode === "forgot-password"
          ? "输入注册邮箱以接收密码重置链接"
          : mode === "reset-password"
            ? "验证重置凭证并更新账户密码"
            : mode === "oauth-callback"
              ? (oauthPending ? `完成 ${oauthLabels[oauthPending.provider] ?? oauthPending.provider} 账号关联` : "正在安全建立会话…")
              : config.site_subtitle || `登录 ${siteName} 控制台`;

  return (
    <>
      <StageCard
        title={heroTitle}
        subtitle={heroSubtitle}
        brand={
          <a
            href="/"
            className="auth-brand-link"
            onClick={e => {
              e.preventDefault();
              navigatePublic("home");
            }}
          >
            <Brand name={siteName} logo={config.site_logo || undefined} />
          </a>
        }
        topActions={
          <Button variant="ghost" size="sm" onClick={() => navigatePublic("home")}>
            <ArrowLeft aria-hidden />首页
          </Button>
        }
        footer={
          !pending ? (
            <footer className="login-links">
              {mode === "login" && canSignup && (
                <Button size="xs" variant="link" onClick={() => navigatePublic("register")}>
                  还没有账号？立即注册
                </Button>
              )}
              {mode !== "login" && (
                <Button size="xs" variant="link" onClick={() => navigatePublic("login")}>
                  已有账号？返回登录
                </Button>
              )}
              {config.patrol_home_visible && (
                <Button size="xs" variant="link" onClick={() => navigatePublic("public-patrol")}>
                  实时渠道状态
                </Button>
              )}
            </footer>
          ) : undefined
        }
      >
          {/* 1. 两步验证 */}
          {pending ? (
            <Form className="login-form" busy={action.busy} errors={action.fieldErrors} onSubmit={submitLogin}>
              <Field key="otp" label="验证器代码">
                <OtpInput
                  autoFocus
                  name="otp"
                  defaultValue={pending.devOTP}
                  required
                />
              </Field>
              <ErrorLine value={action.error} />
              <Button type="submit" size="lg" busy={action.busy}>验证并登录</Button>
              <Button variant="link" disabled={action.busy} onClick={() => setPending(undefined)}>返回更换账号</Button>
            </Form>
          ) : mode === "oauth-callback" ? (
            oauthPending ? (
              <Stack>
                <Segmented
                  label="关联方式"
                  value={oauthBindMode}
                  onChange={setOAuthBindMode}
                  options={[
                    ["create", "创建新账号"],
                    ["bind", "绑定已有账号"],
                  ] as const}
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
            ) : (
              <Stack>
                <ErrorLine value={action.error} />
                {!action.error && <Feedback>正在校验授权凭据…</Feedback>}
                <Button variant="outline" onClick={() => navigatePublic("login")}>返回登录</Button>
              </Stack>
            )
          ) : mode === "forgot-password" ? (
            forgotDone ? (
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
                    <Input
                      type="email"
                      autoComplete="email"
                      value={credentials.email}
                      onChange={e => setCredentials(v => ({...v, email: e.target.value}))}
                      required
                    />
                  </Field>
                </FormSection>
                {config.captcha && <Verification key={revision} config={config.captcha} onProof={(token, randstr) => setProof({token, randstr})} />}
                <ErrorLine value={action.error} />
                <Button type="submit" size="lg" busy={action.busy}>发送重置邮件</Button>
              </Form>
            )
          ) : mode === "reset-password" ? (
            resetDone ? (
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
            )
          ) : mode === "email-verify" ? (
            <Form className="login-form" busy={action.busy} errors={action.fieldErrors} onSubmit={submitRegister}>
              <Field label="6 位邮箱验证码">
                <OtpInput
                  autoFocus
                  value={verifyCode}
                  onChange={e => setVerifyCode(e.target.value)}
                  countdown={countdown}
                  sendBusy={action.busy}
                  onSendCode={() =>
                    action.run(async () => {
                      const sent = await api("/session/send-verify-code", countdownSchema, "POST", {
                        email: credentials.email.trim(),
                        ...buildCaptchaFields(config, proof),
                      }, false);
                      setCountdown(sent.countdown);
                    })
                  }
                  required
                />
              </Field>
              <ErrorLine value={action.error} />
              <Button type="submit" size="lg" busy={action.busy}>验证并创建账号</Button>
              <Button variant="link" disabled={action.busy} onClick={() => navigatePublic("register")}>返回修改注册资料</Button>
            </Form>
          ) : mode === "register" ? (
            !canSignup ? (
              <Stack>
                <Feedback tone="neutral">当前站点已关闭公开注册，请联系管理员获取账号。</Feedback>
                <Button size="lg" variant="outline" onClick={() => navigatePublic("login")}>返回登录</Button>
              </Stack>
            ) : (
              <Form className="login-form" busy={action.busy} errors={action.fieldErrors} onSubmit={submitRegister}>
                <FormSection plain>
                  <Field label="邮箱">
                    <Input
                      type="email"
                      autoComplete="email"
                      value={credentials.email}
                      onChange={e => setCredentials(v => ({...v, email: e.target.value}))}
                      required
                    />
                  </Field>
                  <Field label="密码（至少 6 位）">
                    <PasswordInput
                      visible={showPwd}
                      onVisibleChange={setShowPwd}
                      autoComplete="new-password"
                      value={credentials.password}
                      onChange={e => setCredentials(v => ({...v, password: e.target.value}))}
                      required
                    />
                  </Field>
                  <Field label="确认密码">
                    <PasswordInput
                      visible={showPwd}
                      onVisibleChange={setShowPwd}
                      autoComplete="new-password"
                      value={confirmPwd}
                      onChange={e => setConfirmPwd(e.target.value)}
                      required
                    />
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
            )
          ) : (
            /* 默认登录模式 */
            <Form className="login-form" busy={action.busy} errors={action.fieldErrors} onSubmit={submitLogin}>
              <FormSection plain>
                <Field label="邮箱">
                  <Input
                    name="email"
                    type="email"
                    autoComplete="username"
                    value={credentials.email}
                    onChange={event => {
                      setCredentials(value => ({...value, email: event.target.value}));
                      if (selectedAccount && selectedAccount.email !== event.target.value) setSelectedAccount(undefined);
                    }}
                    required
                  />
                </Field>
                <Field label="密码">
                  <PasswordInput
                    name="password"
                    visible={showPwd}
                    onVisibleChange={setShowPwd}
                    autoComplete="current-password"
                    value={credentials.password}
                    onChange={event => {
                      setCredentials(value => ({...value, password: event.target.value}));
                      if (selectedAccount && selectedAccount.password !== event.target.value) setSelectedAccount(undefined);
                    }}
                    required
                  />
                </Field>
                {config.password_reset && !config.backend_mode && (
                  <div className="auth-inline-row">
                    <span />
                    <Button size="xs" variant="link" onClick={() => navigatePublic("forgot-password")}>
                      忘记密码？
                    </Button>
                  </div>
                )}
              </FormSection>

              {config.dev_accounts && config.dev_accounts.length > 0 && (
                <div className="login-dev">
                  <span className="login-dev-label">开发预览 · 选择测试账号自动填充，不会直接登录</span>
                  <Select
                    aria-label="测试账号"
                    value={selectedAccount?.email ?? ""}
                    onChange={event => {
                      const account = config.dev_accounts?.find(item => item.email === event.target.value);
                      setSelectedAccount(account);
                      if (account) setCredentials({email: account.email, password: account.password});
                    }}
                  >
                    <option value="">选择测试账号</option>
                    {config.dev_accounts.map(account => (
                      <option key={account.email} value={account.email}>{account.label}</option>
                    ))}
                  </Select>
                </div>
              )}

              {agreementControl}
              {config.captcha && <Verification key={revision} config={config.captcha} onProof={(token, randstr) => setProof({token, randstr})} />}
              <ErrorLine value={action.error} />
              <Button type="submit" size="lg" busy={action.busy}>登录</Button>

              {(passkeyAvailable || oauthList.length > 0) && (
                <Stack>
                  <div className="auth-divider">其他登录方式</div>
                  {passkeyAvailable && (
                    <Button size="sm" variant="outline" disabled={action.busy} onClick={handlePasskeyLogin}>
                      <KeyRound aria-hidden />使用通行密钥 (Passkey) 登录
                    </Button>
                  )}
                  {oauthList.length > 0 && (
                    <div className="auth-oauth-grid">
                      {oauthList.map(provider => (
                        <Button key={provider} size="sm" variant="outline" disabled={action.busy} onClick={() => handleOAuthStart(provider)}>
                          {provider === "oidc" && config.oidc_provider_name ? config.oidc_provider_name : (oauthLabels[provider] ?? provider)}
                        </Button>
                      ))}
                    </div>
                  )}
                </Stack>
              )}
            </Form>
          )}
      </StageCard>

      <Overlay
        open={agreementModal}
        onClose={() => setAgreementModal(false)}
        title="站点服务协议"
        footer={
          <InlineStack>
            <Button variant="outline" onClick={() => setAgreementModal(false)}>关闭</Button>
            <Button
              onClick={() => {
                setAgreed(true);
                if (config.agreement_revision) localStorage.setItem(agreementConsentKey, config.agreement_revision);
                setAgreementModal(false);
              }}
            >
              同意并继续
            </Button>
          </InlineStack>
        }
      >
        <Stack>
          {config.agreement_docs.map(doc => (
            <InsetGroup inset key={doc.id} title={doc.title}>
              <InsetRow label="正文">
                <div className="patrol-reply-box">{doc.content_md}</div>
              </InsetRow>
            </InsetGroup>
          ))}
        </Stack>
      </Overlay>
    </>
  );
}
