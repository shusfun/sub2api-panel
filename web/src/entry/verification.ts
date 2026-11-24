export {};
import "./verification.css";

type TencentResult = {ret: number; ticket?: string; randstr?: string};
type TencentCallback = (result: TencentResult) => void;
type TencentWidget = {show(): void};
type TencentConstructor = {
  new (key: string, cb: TencentCallback): TencentWidget;
  new (element: HTMLElement, key: string, cb: TencentCallback): TencentWidget;
};

function readMessage(value: unknown) {
  if (typeof value !== "object" || !value || !("channel" in value) || !("options" in value)) return;
  const {channel, options} = value;
  if (typeof channel !== "string" || !/^[a-f0-9-]{36}$/.test(channel) || typeof options !== "object" || !options ||
      !("provider" in options) || !("key" in options) || typeof options.key !== "string" || !options.key) return;
  const {provider, key} = options;
  if (provider !== "turnstile" && provider !== "tencent" && provider !== "aliyun") return;
  const prefix = "prefix" in options && typeof options.prefix === "string" ? options.prefix : undefined;
  const region = "region" in options && typeof options.region === "string" ? options.region : undefined;
  return {channel, provider, key, prefix, region};
}
declare global {
  interface Window {
    turnstile?: {render(element: string, options: Record<string, unknown>): string};
    TencentCaptcha?: TencentConstructor;
    initAliyunCaptcha?: (options: Record<string, unknown>) => void;
  }
}

const button = document.querySelector<HTMLButtonElement>("#verify");
const message = document.querySelector("#error");
const container = document.querySelector<HTMLElement>("#challenge");
const initialOrigin = new URL(document.referrer || location.href).origin;
let initialized = false;

function loadScript(src: string) {
  return new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = src;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("验证服务无法连接。"));
    document.head.append(script);
  });
}

window.addEventListener("message", async (event: MessageEvent<unknown>) => {
  if (initialized || event.source !== parent || event.origin !== initialOrigin) return;
  const options = readMessage(event.data);
  if (!options || !button || !container) return;
  initialized = true;
  const {channel} = options;
  const send = (token: string, randstr = "") => parent.postMessage({channel, token, randstr}, initialOrigin);
  const fail = () => {
    send("");
    if (message) message.textContent = "验证未完成，请重新打开。";
  };
  try {
    if (options.provider === "turnstile") {
      await loadScript("https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit");
      if (!window.turnstile) return fail();
      window.turnstile.render("#challenge", {
        sitekey: options.key, callback: send, "expired-callback": () => send(""), "error-callback": fail,
      });
      button.hidden = true;
    } else if (options.provider === "tencent") {
      const international = options.region === "intl";
      await loadScript(international ? "https://ca.turing.captcha.qcloud.com/TJNCaptcha-global.js" : "https://turing.captcha.qcloud.com/TJCaptcha.js");
      const TencentCaptcha = window.TencentCaptcha;
      if (!TencentCaptcha) return fail();
      const onResult: TencentCallback = value => {
        if (value.ret === 0 && value.ticket && value.randstr) send(value.ticket, value.randstr);
        else fail();
      };
      if (international) {
        new TencentCaptcha(container, options.key, onResult).show();
        button.hidden = true;
      } else {
        button.onclick = () => new TencentCaptcha(options.key, onResult).show();
      }
    } else {
      await loadScript("https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js");
      if (!window.initAliyunCaptcha) return fail();
      window.initAliyunCaptcha({
        SceneId: options.key, prefix: options.prefix, region: options.region, mode: "popup",
        element: "#challenge", button: "#verify", language: "cn",
        captchaVerifyCallback: (token: string) => {
          send(token);
          return {captchaResult: true};
        },
      });
    }
  } catch {
    fail();
  }
});
