import {useEffect, useRef, useState} from "react";
import {z} from "zod";
import type {newBrowserKey} from "@/lib/http";
import type {Config} from "@/schemas";
import {Button, InlineStack, InsetGroup, InsetRow, Overlay, Stack} from "@/ui";

export function AgreementOverlay({
  open,
  docs,
  onClose,
  onAccept,
}: {
  open: boolean;
  docs: Config["agreement_docs"];
  onClose: () => void;
  onAccept: () => void;
}) {
  return (
    <Overlay
      open={open}
      onClose={onClose}
      title="站点服务协议"
      footer={
        <InlineStack>
          <Button variant="outline" onClick={onClose}>关闭</Button>
          <Button onClick={onAccept}>同意并继续</Button>
        </InlineStack>
      }
    >
      <Stack>
        {docs.map(doc => (
          <InsetGroup inset key={doc.id} title={doc.title}>
            <InsetRow label="正文">
              <div className="patrol-reply-box">{doc.content_md}</div>
            </InsetRow>
          </InsetGroup>
        ))}
      </Stack>
    </Overlay>
  );
}

export function Verification({config, onProof}: {config: NonNullable<Config["captcha"]>; onProof: (token: string, randstr: string) => void}) {
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

export const loginSchema = z.object({pending: z.string().optional()});
export const countdownSchema = z.object({countdown: z.number()});
export const inviteCheckSchema = z.object({valid: z.boolean(), error_code: z.string().optional()});
export const promoCheckSchema = z.object({valid: z.boolean(), bonus_amount: z.number().optional(), error_code: z.string().optional()});
export const passkeyBeginSchema = z.object({session_token: z.string(), options: z.record(z.string(), z.unknown())});
export const oauthStartSchema = z.object({url: z.string()});
export const pendingExchangeSchema = z.object({
  pending_auth_token: z.string(),
  provider: z.string(),
  display_name: z.string().optional(),
  email: z.string().optional(),
  require_email: z.boolean(),
});

export type BrowserKey = Awaited<ReturnType<typeof newBrowserKey>>;
export type DevLoginAccount = NonNullable<Config["dev_accounts"]>[number];
export type PendingOAuth = z.infer<typeof pendingExchangeSchema>;

export const agreementConsentKey = "panel-login-agreement-rev";

export const oauthLabels: Record<string, string> = {
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

export function prepareWebAuthnGetOptions(raw: Record<string, unknown>): PublicKeyCredentialRequestOptions {
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

export function serializeAssertion(cred: PublicKeyCredential) {
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

export function buildCaptchaFields(config: Config, proof: {token: string; randstr: string}) {
  if (!config.captcha || !proof.token) return {};
  if (config.captcha.provider === "tencent") {
    return {tencent_captcha_ticket: proof.token, tencent_captcha_randstr: proof.randstr};
  }
  return {turnstile_token: proof.token};
}
