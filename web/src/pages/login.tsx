import {useEffect, useRef, useState, type FormEvent} from "react";
import {ArrowUpRight} from "lucide-react";
import {z} from "zod";
import {api, empty, newBrowserKey, saveBrowserKey, sessionSchema, type Config, type SessionInfo} from "../api";
import {Brand, ThemeControl} from "../appearance";
import {useAction} from "../requests";
import {Button, ErrorLine, Field, Form, Input} from "../ui";

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
  return <iframe className="captcha" ref={frame} title="人机验证" sandbox="allow-scripts allow-forms allow-popups"
    src="/verification.html" onLoad={() => frame.current?.contentWindow?.postMessage({channel, options: config}, "*")} />;
}

const loginSchema = z.object({pending: z.string().optional()});
type BrowserKey = Awaited<ReturnType<typeof newBrowserKey>>;

export function Login({config, onLogin}: {config: Config; onLogin: (session: SessionInfo) => void}) {
  const [pending, setPending] = useState<{id: string; keys: BrowserKey}>();
  const [proof, setProof] = useState({token: "", randstr: ""});
  const [revision, revise] = useState(0);
  const action = useAction();
  function submit(event: FormEvent<HTMLFormElement>) {
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
        if (config.captcha && !proof.token) throw new Error("请先完成人机验证。");
        const keys = await newBrowserKey();
        const {challenge} = await api("/session/challenge", z.object({challenge: z.string()}), "POST", undefined, false);
        let result;
        try {
          result = await api("/session/login", loginSchema, "POST", {
            challenge, browser_key: keys.public, email: data.get("email"), password: data.get("password"),
            agreement: data.get("agreement") === "on",
            ...(config.captcha?.provider === "tencent" ? {tencent_captcha_ticket: proof.token, tencent_captcha_randstr: proof.randstr}
              : config.captcha ? {turnstile_token: proof.token} : {}),
          }, keys.private);
        } finally {
          setProof({token: "", randstr: ""});
          revise(value => value + 1);
        }
        if (result.pending) {
          setPending({id: result.pending, keys});
          return;
        }
        await saveBrowserKey(keys.private);
      }
      onLogin(await api("/session", sessionSchema));
    });
  }
  return <div className="login-stage">
    <header className="login-header"><Brand /><ThemeControl /></header>
    <main className="login-card">
      <h1>{pending ? "两步验证" : "登录控制台"}</h1>
      <Form className="login-form" busy={action.busy} onSubmit={submit}>
        {pending ? <Field label="身份验证器代码"><Input key="otp" autoFocus name="otp" autoComplete="one-time-code"
          inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required /></Field> : <>
          <Field label="邮箱"><Input name="email" type="email" autoComplete="username" required /></Field>
          <Field label="密码"><Input name="password" type="password" autoComplete="current-password" required /></Field>
          {config.agreement && <label className="agreement"><input type="checkbox" name="agreement" required />
            <span>我已阅读并同意<a href={config.official + "/login"} target="_blank" rel="noreferrer">官网登录协议</a></span></label>}
          {config.captcha && <Verification key={revision} config={config.captcha} onProof={(token, randstr) => setProof({token, randstr})} />}
        </>}
        <ErrorLine value={action.error} />
        <Button type="submit" busy={action.busy}>{pending ? "验证并登录" : "登录"}</Button>
        {pending && <Button variant="ghost" disabled={action.busy} onClick={() => setPending(undefined)}>返回登录</Button>}
      </Form>
      {!pending && <div className="login-links"><a href={config.official + "/register"} target="_blank" rel="noreferrer">注册账号 <ArrowUpRight aria-hidden /></a>
        <a href={config.official + "/login"}>官方面板 <ArrowUpRight aria-hidden /></a></div>}
    </main>
  </div>;
}
