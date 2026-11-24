import {useEffect, useState, type FormEvent} from "react";
import {ArrowLeft, KeyRound} from "lucide-react";
import {z} from "zod";
import {api, newBrowserKey, saveBrowserKey} from "@/lib/http";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {empty, sessionSchema, type Config, type SessionInfo} from "@/schemas";
import {Brand} from "@/app/appearance";
import {navigatePublic, type PublicPageID} from "@/app/route";
import {
  Button, Checkbox, ErrorLine, Field, Form, FormSection, Input,
  OtpInput, PasswordInput, Select, Stack, StageCard,
} from "@/ui";
import {
  AgreementOverlay, agreementConsentKey, buildCaptchaFields, countdownSchema, inviteCheckSchema, loginSchema,
  oauthLabels, oauthStartSchema, passkeyBeginSchema, pendingExchangeSchema,
  prepareWebAuthnGetOptions, promoCheckSchema, serializeAssertion, Verification,
  type BrowserKey, type DevLoginAccount, type PendingOAuth,
} from "./auth-helpers";
import {OAuthCallbackView, PasswordRecoveryView, RegisterFormView} from "./auth-secondary-flows";

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

  const [inviteCode, setInviteCode] = useState(params?.get("invite") ?? "");
  const [promoCode, setPromoCode] = useState("");
  const [affCode, setAffCode] = useState(initialAff);
  const [verifyCode, setVerifyCode] = useState("");
  const [countdown, setCountdown] = useState(0);
  const [inviteState, setInviteState] = useState<"idle" | "valid" | "invalid">("idle");
  const [promoState, setPromoState] = useState<{status: "idle" | "valid" | "invalid"; bonus?: number}>({status: "idle"});
  const [forgotDone, setForgotDone] = useState(false);
  const [resetDone, setResetDone] = useState(false);

  const [oauthPending, setOAuthPending] = useState<PendingOAuth>();
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

      if (mode === "register" && config.captcha && !proof.token) throw new Error("请先完成人机验证。");
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
        <Checkbox name="agreement" label="阅读并同意服务协议" checked={agreed} onChange={setAgreed} inline>我已阅读并同意</Checkbox>
        {config.agreement_docs.length > 0 ? (
          <Button size="xs" variant="link" onClick={() => setAgreementModal(true)}>《{config.agreement_docs[0].title}》</Button>
        ) : (
          <span>站点服务协议</span>
        )}
      </div>
    </Field>
  );

  const heroTitle = pending ? "两步验证" : mode === "register" ? "创建账号" : mode === "email-verify" ? "验证邮箱" : mode === "forgot-password" ? "找回密码" : mode === "reset-password" ? "设置新密码" : mode === "oauth-callback" ? "第三方登录连接" : "欢迎回来";
  const heroSubtitle = pending ? "输入验证器应用中的 6 位动态验证码" : mode === "register" ? config.site_subtitle || `加入 ${siteName} 开始使用统一网关` : mode === "email-verify" ? `验证码已发送至 ${credentials.email}` : mode === "forgot-password" ? "输入注册邮箱以接收密码重置链接" : mode === "reset-password" ? "验证重置凭证并更新账户密码" : mode === "oauth-callback" ? (oauthPending ? `完成 ${oauthLabels[oauthPending.provider] ?? oauthPending.provider} 账号关联` : "正在安全建立会话…") : config.site_subtitle || `登录 ${siteName} 控制台`;

  return (
    <>
      <StageCard
        title={heroTitle}
        subtitle={heroSubtitle}
        brand={
          <a href="/" className="auth-brand-link" onClick={e => { e.preventDefault(); navigatePublic("home"); }}>
            <Brand name={siteName} logo={config.site_logo || undefined} />
          </a>
        }
        topActions={<Button variant="ghost" size="sm" onClick={() => navigatePublic("home")}><ArrowLeft aria-hidden />首页</Button>}
        footer={
          !pending ? (
            <footer className="login-links">
              {mode === "login" && canSignup && <Button size="xs" variant="link" onClick={() => navigatePublic("register")}>还没有账号？立即注册</Button>}
              {mode !== "login" && <Button size="xs" variant="link" onClick={() => navigatePublic("login")}>已有账号？返回登录</Button>}
              {config.patrol_home_visible && <Button size="xs" variant="link" onClick={() => navigatePublic("public-patrol")}>实时渠道状态</Button>}
            </footer>
          ) : undefined
        }
      >
        {pending ? (
          <Form className="login-form" busy={action.busy} errors={action.fieldErrors} onSubmit={submitLogin}>
            <Field key="otp" label="验证器代码">
              <OtpInput autoFocus name="otp" defaultValue={pending.devOTP} required />
            </Field>
            <ErrorLine value={action.error} />
            <Button type="submit" size="lg" busy={action.busy}>验证并登录</Button>
            <Button variant="link" disabled={action.busy} onClick={() => setPending(undefined)}>返回更换账号</Button>
          </Form>
        ) : mode === "oauth-callback" ? (
          <OAuthCallbackView
            config={config}
            action={action}
            oauthPending={oauthPending}
            oauthBindMode={oauthBindMode}
            setOAuthBindMode={setOAuthBindMode}
            credentials={credentials}
            setCredentials={setCredentials}
            verifyCode={verifyCode}
            setVerifyCode={setVerifyCode}
            countdown={countdown}
            setCountdown={setCountdown}
            affCode={affCode}
            setPending={setPending}
            onLogin={onLogin}
          />
        ) : mode === "forgot-password" || mode === "reset-password" ? (
          <PasswordRecoveryView
            mode={mode}
            config={config}
            action={action}
            credentials={credentials}
            setCredentials={setCredentials}
            confirmPwd={confirmPwd}
            setConfirmPwd={setConfirmPwd}
            showPwd={showPwd}
            setShowPwd={setShowPwd}
            proof={proof}
            setProof={setProof}
            revision={revision}
            resetCaptcha={resetCaptcha}
            resetToken={resetToken}
            forgotDone={forgotDone}
            setForgotDone={setForgotDone}
            resetDone={resetDone}
            setResetDone={setResetDone}
          />
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
          <RegisterFormView
            config={config}
            canSignup={canSignup}
            action={action}
            credentials={credentials}
            setCredentials={setCredentials}
            confirmPwd={confirmPwd}
            setConfirmPwd={setConfirmPwd}
            showPwd={showPwd}
            setShowPwd={setShowPwd}
            inviteCode={inviteCode}
            setInviteCode={setInviteCode}
            inviteState={inviteState}
            affCode={affCode}
            setAffCode={setAffCode}
            promoCode={promoCode}
            setPromoCode={setPromoCode}
            promoState={promoState}
            agreementControl={agreementControl}
            revision={revision}
            setProof={setProof}
            oauthList={oauthList}
            handleOAuthStart={handleOAuthStart}
            onSubmit={submitRegister}
          />
        ) : (
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
                  <Button size="xs" variant="link" onClick={() => navigatePublic("forgot-password")}>忘记密码？</Button>
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

      <AgreementOverlay
        open={agreementModal}
        docs={config.agreement_docs}
        onClose={() => setAgreementModal(false)}
        onAccept={() => {
          setAgreed(true);
          if (config.agreement_revision) localStorage.setItem(agreementConsentKey, config.agreement_revision);
          setAgreementModal(false);
        }}
      />
    </>
  );
}
