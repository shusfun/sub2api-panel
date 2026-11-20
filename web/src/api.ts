import { z } from "zod";

export const empty = z.object({});
export const page = <T extends z.ZodTypeAny>(row: T) => z.object({items: z.array(row), more: z.boolean()});
const id = z.number().int().positive();
export const configSchema = z.object({
  official: z.string().url(), agreement: z.boolean(), affiliate: z.boolean(), invitation: z.boolean(),
  captcha: z.object({provider: z.enum(["turnstile", "tencent", "aliyun"]), key: z.string(), prefix: z.string().optional(),
    region: z.string().optional()}).optional(),
});
export type Config = z.infer<typeof configSchema>;
export const sessionSchema = z.object({admin: z.boolean(), csrf: z.string()});
export type SessionInfo = z.infer<typeof sessionSchema>;
export const grantSchema = z.object({invites: z.number(), budget_cents: z.number(), amounts_cents: z.array(z.number())});
export const named = z.object({id, name: z.string()});
export const keyRow = named.extend({masked: z.string(), status: z.string()});
export const subscriptions = z.object({id, group: z.string(), status: z.string(), expires_at: z.string()});
export const usage = z.object({id, model: z.string(), created_at: z.string(), input_tokens: z.number(), output_tokens: z.number(), actual_cost: z.number().nullable()});
const tokenCount = z.number().int().nonnegative();
const usageTokens = z.object({input_tokens: tokenCount, output_tokens: tokenCount, cache_read_tokens: tokenCount, cache_creation_tokens: tokenCount});
export const usageRecord = usage.extend({created_at: z.string().datetime({offset: true}), key_name: z.string(),
  input_tokens: tokenCount, output_tokens: tokenCount, cache_read_tokens: tokenCount.nullable(), cache_creation_tokens: tokenCount.nullable()});
const usageDate = z.string().regex(/^\d{4}-\d{2}-\d{2}$/);
const usageModel = z.object({model: z.string(), requests: tokenCount, total_tokens: tokenCount, actual_cost: z.number().finite()});
export const usageSummary = z.object({
  timezone: z.string(), start_date: usageDate, end_date: usageDate,
  totals: usageTokens.extend({requests: tokenCount, actual_cost: z.number().finite()}),
  days: z.array(usageTokens.extend({date: usageDate})).min(1).max(90),
  models: page(usageModel),
});
export const userRow = z.object({id, email: z.string(), status: z.string(), balance: z.number().nullable()});
export const account = named.extend({platform: z.string(), status: z.string(), schedulable: z.boolean(), five_hours_used: z.number().nullable(), week_used: z.number().nullable()});
const quotaWindow = z.object({percent_used: z.number().nullable(), resets_at: z.string().nullable()}).nullable();
export const accountQuota = z.object({five_hours: quotaWindow, week: quotaWindow});
export const issue = z.object({operation_id: z.string(), actor_id: id.optional(), kind: z.string(), amount_cents: z.number(), state: z.string(),
  code_id: id.nullable(), created_at: z.number(), quota_delta: z.number(), request_id: z.string().optional()});
const testEvent = z.object({state: z.enum(["running", "passed", "failed"])});
const errorSchema = z.object({code: z.string(), hint: z.string(), request_id: z.string()});

export class ApiError extends Error {
  constructor(readonly status: number, readonly code: string, readonly requestId = "", hint?: string) {
    super(hint ?? (status === 401 ? "会话已结束，请重新登录。" : "服务暂不可用，请稍后再试。"));
  }
}

export function describeError(error: unknown) {
  if (error instanceof ApiError) return error.requestId ? error.message + " 请求编号 " + error.requestId : error.message;
  return error instanceof Error ? error.message : "操作未能完成。";
}

let csrf = "";
let privateKey: CryptoKey | undefined;

export function setSession(session?: SessionInfo) { csrf = session?.csrf ?? ""; }

function openKeys(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open("sub2api-panel", 1);
    request.onupgradeneeded = () => request.result.createObjectStore("keys");
    request.onerror = () => reject(new Error("浏览器无法保存会话密钥。"));
    request.onsuccess = () => resolve(request.result);
  });
}

async function storedKey(value?: CryptoKey) {
  const db = await openKeys();
  try {
    return await new Promise<CryptoKey | undefined>((resolve, reject) => {
      const tx = db.transaction("keys", value ? "readwrite" : "readonly");
      const store = tx.objectStore("keys");
      const request = value ? store.put(value, "session") : store.get("session");
      tx.oncomplete = () => {
        const result: unknown = request.result;
        resolve(result instanceof CryptoKey ? result : undefined);
      };
      tx.onabort = () => reject(new Error("浏览器无法保存会话密钥。"));
    });
  } finally {
    db.close();
  }
}

const base64 = (bytes: ArrayBuffer) => btoa(String.fromCharCode(...new Uint8Array(bytes)));
const base64URL = (bytes: ArrayBuffer) => base64(bytes).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");

export async function newBrowserKey() {
  const key = await crypto.subtle.generateKey({name: "ECDSA", namedCurve: "P-256"}, false, ["sign", "verify"]);
  return {private: key.privateKey, public: base64URL(await crypto.subtle.exportKey("raw", key.publicKey))};
}

export async function saveBrowserKey(key: CryptoKey) {
  await storedKey(key);
  privateKey = key;
}

async function sign(headers: Headers, key: CryptoKey, method: string, url: URL, body?: string) {
  const bytes = new TextEncoder();
  const digest = "sha-256=:" + base64(await crypto.subtle.digest("SHA-256", bytes.encode(body ?? ""))) + ":";
  const nonce = [...crypto.getRandomValues(new Uint8Array(16))].map(value => value.toString(16).padStart(2, "0")).join("");
  const params = '("@method" "@target-uri" "content-digest");created=' + Math.floor(Date.now() / 1000) + ';nonce="' + nonce + '"';
  const fields = ['"@method": ' + method, '"@target-uri": ' + url.href, '"content-digest": ' + digest, '"@signature-params": ' + params];
  const signature = await crypto.subtle.sign({name: "ECDSA", hash: "SHA-256"}, key, bytes.encode(fields.join("\n")));
  headers.set("Signature-Input", "panel=" + params);
  headers.set("Signature", "panel=:" + base64(signature) + ":");
  headers.set("Content-Digest", digest);
}

async function request(path: string, method: string, input?: object, proof?: CryptoKey | false) {
  const url = new URL("/api" + path, location.origin);
  const headers = new Headers();
  const body = input === undefined ? undefined : JSON.stringify(input);
  if (body !== undefined) headers.set("Content-Type", "application/json");
  if (proof !== false) {
    const key = proof ?? privateKey ?? await storedKey();
    if (!key) throw new ApiError(401, "sign_in_required");
    if (!proof) privateKey = key;
    await sign(headers, key, method, url, body);
  }
  if (csrf && method !== "GET") headers.set("X-CSRF-Token", csrf);
  let response: Response;
  try {
    response = await fetch(url, {method, headers, body, redirect: "error"});
  } catch {
    throw new Error("连接失败，请检查网络后重试。");
  }
  if (!response.ok) {
    const problem = errorSchema.safeParse(await response.json().catch(() => null));
    const error = new ApiError(response.status, problem.data?.code ?? "service_unavailable", problem.data?.request_id, problem.data?.hint);
    if (csrf && error.status === 401 && ["sign_in_required", "invalid_proof"].includes(error.code)) {
      window.dispatchEvent(new Event("panel-session-ended"));
    }
    throw error;
  }
  return response;
}

export async function api<T>(path: string, schema: z.ZodType<T>, method = "GET", input?: object, proof?: CryptoKey | false) {
  const response = await request(path, method, input, proof);
  const parsed = schema.safeParse(await response.json().catch(() => null));
  if (!parsed.success) throw new ApiError(502, "invalid_response", response.headers.get("X-Request-ID") ?? "");
  return parsed.data;
}

export async function testAccount(id: number, model: string, progress: (state: string) => void) {
  const response = await request("/admin/accounts/" + id + "/test", "POST", model ? {model_id: model} : {});
  if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
    throw new Error("测活结果格式错误。");
  }
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
  try {
    let buffered = "";
    for (;;) {
      const {done, value} = await reader.read();
      if (done) throw new Error("测活连接提前结束。");
      buffered += value;
      let end = buffered.indexOf("\n\n");
      while (end !== -1) {
        const message = buffered.slice(0, end);
        buffered = buffered.slice(end + 2);
        end = buffered.indexOf("\n\n");
        if (!message.startsWith("data: ")) continue;
        const event = testEvent.parse(JSON.parse(message.slice(6)));
        progress(event.state);
        if (event.state !== "running") return;
      }
    }
  } finally {
    await reader.cancel();
  }
}
