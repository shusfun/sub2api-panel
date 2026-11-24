import {z} from "zod";
import {instant, opsLiveSchema, probeReportSchema, usageStreamEvent, usageSummary, type ProbeReport, type SessionInfo} from "@/schemas";

const probeStreamFrameSchema = z.object({
  type: z.enum(["status", "delta", "complete", "error"]),
  message: z.string().optional(),
  delta: z.string().optional(),
  report: probeReportSchema.optional(),
});

const testEvent = z.object({state: z.enum(["running", "passed", "failed"])});
const errorSchema = z.object({
  code: z.string(),
  hint: z.string(),
  request_id: z.string(),
  fields: z.record(z.string(), z.string()).optional(),
});

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly requestId = "",
    hint?: string,
    readonly fields: Record<string, string> = {},
  ) {
    super(hint ?? (status === 401 ? "会话已结束，请重新登录。" : "服务暂不可用，请稍后再试。"));
  }
}

export function describeError(error: unknown) {
  if (error instanceof ApiError) return error.requestId ? error.message + " 请求编号 " + error.requestId : error.message;
  return error instanceof Error ? error.message : "操作未能完成。";
}

let csrf = "";
let privateKey: CryptoKey | undefined;

export function setSession(session?: SessionInfo) {
  csrf = session?.csrf ?? "";
}

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

async function sign(headers: Headers, key: CryptoKey, method: string, url: URL, body?: string | Blob) {
  const bytes = new TextEncoder();
  const input = body instanceof Blob ? await body.arrayBuffer() : bytes.encode(body ?? "");
  const digest = "sha-256=:" + base64(await crypto.subtle.digest("SHA-256", input)) + ":";
  const nonce = [...crypto.getRandomValues(new Uint8Array(16))].map(value => value.toString(16).padStart(2, "0")).join("");
  const params = '("@method" "@target-uri" "content-digest");created=' + Math.floor(Date.now() / 1000) + ';nonce="' + nonce + '"';
  const fields = ['"@method": ' + method, '"@target-uri": ' + url.href, '"content-digest": ' + digest, '"@signature-params": ' + params];
  const signature = await crypto.subtle.sign({name: "ECDSA", hash: "SHA-256"}, key, bytes.encode(fields.join("\n")));
  headers.set("Signature-Input", "panel=" + params);
  headers.set("Signature", "panel=:" + base64(signature) + ":");
  headers.set("Content-Digest", digest);
}

const traceHex = (size: number) => [...crypto.getRandomValues(new Uint8Array(size))].map(value => value.toString(16).padStart(2, "0")).join("");
export const operationTrace = () => traceHex(16);

let currentActionTrace = "";
export async function withActionTrace<T>(trace: string, run: () => Promise<T>): Promise<T> {
  const prev = currentActionTrace;
  currentActionTrace = trace;
  try {
    return await run();
  } finally {
    currentActionTrace = prev;
  }
}

async function request(path: string, method: string, input?: object, proof?: CryptoKey | false, signal?: AbortSignal, trace = currentActionTrace || traceHex(16)) {
  const url = new URL("/api" + path, location.origin);
  const headers = new Headers();
  headers.set("traceparent", "00-" + trace + "-" + traceHex(8) + "-01");
  const body = input instanceof Blob ? input : input === undefined ? undefined : JSON.stringify(input);
  if (body !== undefined) headers.set("Content-Type", body instanceof Blob ? "application/octet-stream" : "application/json");
  if (proof !== false) {
    const key = proof ?? privateKey ?? await storedKey();
    if (!key) throw new ApiError(401, "sign_in_required");
    if (!proof) privateKey = key;
    await sign(headers, key, method, url, body);
  }
  if (csrf && method !== "GET") headers.set("X-CSRF-Token", csrf);
  let response: Response;
  try {
    response = await fetch(url, {method, headers, body, signal, redirect: "error"});
  } catch {
    if (signal?.aborted) throw signal.reason;
    throw new Error("连接失败，请检查网络后重试。");
  }
  if (!response.ok) {
    const problem = errorSchema.safeParse(await response.json().catch(() => null));
    const error = new ApiError(
      response.status,
      problem.data?.code ?? "service_unavailable",
      problem.data?.request_id,
      problem.data?.hint,
      problem.data?.fields ?? {},
    );
    if (csrf && error.status === 401 && ["sign_in_required", "invalid_proof"].includes(error.code)) {
      window.dispatchEvent(new Event("panel-session-ended"));
    }
    throw error;
  }
  return response;
}

export async function api<Schema extends z.ZodTypeAny>(path: string, schema: Schema, method = "GET", input?: object, proof?: CryptoKey | false, trace?: string): Promise<z.output<Schema>> {
  const response = await request(path, method, input, proof, undefined, trace);
  const parsed = schema.safeParse(await response.json().catch(() => null));
  if (!parsed.success) throw new ApiError(502, "invalid_response", response.headers.get("X-Request-ID") ?? "");
  return parsed.data;
}

export async function testAccount(id: number, model: string, progress: (state: string) => void, trace?: string) {
  const response = await request("/admin/accounts/" + id + "/test", "POST", model ? {model_id: model} : {}, undefined, undefined, trace);
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

export async function streamProbe(
  path: string,
  payload: Record<string, unknown>,
  callbacks: {
    onStatus?: (msg: string) => void;
    onDelta?: (chunk: string) => void;
  },
): Promise<ProbeReport> {
  const response = await request(path, "POST", payload);
  if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
    throw new Error("探测响应格式错误。");
  }
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
  try {
    let buffered = "";
    for (;;) {
      const {done, value} = await reader.read();
      if (done) break;
      buffered += value;
      let end = buffered.indexOf("\n\n");
      while (end !== -1) {
        const message = buffered.slice(0, end);
        buffered = buffered.slice(end + 2);
        end = buffered.indexOf("\n\n");
        if (!message.startsWith("data: ")) continue;
        const frame = probeStreamFrameSchema.parse(JSON.parse(message.slice(6)));
        if (frame.type === "status" && frame.message) callbacks.onStatus?.(frame.message);
        else if (frame.type === "delta" && frame.delta) callbacks.onDelta?.(frame.delta);
        else if (frame.type === "error") throw new Error(frame.message ?? "探测未完成。");
        else if (frame.type === "complete" && frame.report) return frame.report;
      }
    }
    throw new Error("探测流提前中断。");
  } finally {
    await reader.cancel();
  }
}

export type UsageStreamStatus = "connecting" | "live" | "reconnecting" | "paused" | "denied";
type UsageSnapshot = z.infer<typeof usageSummary>;

function waitForStream(signal: AbortSignal, untilVisible: boolean) {
  return new Promise<void>(resolve => {
    const finish = () => {
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", visible);
      signal.removeEventListener("abort", finish);
      resolve();
    };
    const visible = () => { if (!document.hidden) finish(); };
    const timer = untilVisible ? undefined : window.setTimeout(finish, 1500);
    if (untilVisible) document.addEventListener("visibilitychange", visible);
    signal.addEventListener("abort", finish, {once: true});
    if (signal.aborted || (untilVisible && !document.hidden)) finish();
  });
}

export async function streamUsage(
  path: string,
  onSnapshot: (value: UsageSnapshot, cursor: string, requestID?: string) => void,
  onStatus: (status: UsageStreamStatus) => void,
  signal?: AbortSignal,
) {
  const lifetime = signal ?? new AbortController().signal;
  const trace = traceHex(16);
  let cursor = "";
  let snapshot: UsageSnapshot | undefined;
  while (!lifetime.aborted) {
    if (document.hidden) {
      onStatus("paused");
      await waitForStream(lifetime, true);
      continue;
    }
    onStatus(cursor ? "reconnecting" : "connecting");
    const connection = new AbortController();
    const abort = () => connection.abort();
    const pause = () => { if (document.hidden) abort(); };
    document.addEventListener("visibilitychange", pause);
    lifetime.addEventListener("abort", abort, {once: true});
    let watchdog: number | undefined;
    const renew = () => {
      window.clearTimeout(watchdog);
      watchdog = window.setTimeout(abort, 35000);
    };
    renew();
    try {
      const separator = path.includes("?") ? "&" : "?";
      const response = await request(path + (cursor ? separator + "cursor=" + encodeURIComponent(cursor) : ""), "GET", undefined, undefined, connection.signal, trace);
      if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
        throw new ApiError(502, "invalid_response", response.headers.get("X-Request-ID") ?? "");
      }
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
      let buffered = "";
      let available = false;
      try {
        while (!connection.signal.aborted) {
          const {done, value} = await reader.read();
          if (done) break;
          if (connection.signal.aborted) break;
          buffered += value;
          if (buffered.length > 8 * 1024 * 1024) throw new ApiError(502, "invalid_response");
          let end: number;
          while ((end = buffered.indexOf("\n\n")) !== -1) {
            const data = buffered.slice(0, end).split("\n").find(line => line.startsWith("data: "))?.slice(6);
            buffered = buffered.slice(end + 2);
            if (!data) continue;
            const parsed = usageStreamEvent.parse(JSON.parse(data));
            renew();
            if (parsed.type === "unavailable") {
              available = false;
              if (parsed.reason === "not_allowed") { onStatus("denied"); return; }
              onStatus("reconnecting");
              continue;
            }
            if (parsed.type === "heartbeat") {
              if (parsed.cursor && parsed.cursor === cursor && snapshot) available = true;
              if (available) onStatus("live");
              continue;
            }
            if (!parsed.cursor) throw new ApiError(502, "invalid_response");
            if (parsed.type === "snapshot" && parsed.snapshot) snapshot = parsed.snapshot;
            else if (parsed.type === "delta" && parsed.base === cursor && snapshot && parsed.delta) snapshot = {...snapshot, ...parsed.delta};
            else {
              cursor = "";
              snapshot = undefined;
              throw new ApiError(502, "invalid_response");
            }
            cursor = parsed.cursor;
            available = true;
            onStatus("live");
            onSnapshot(snapshot, cursor, parsed.request_id);
          }
        }
      } finally {
        await reader.cancel().catch(() => {});
      }
    } catch (error) {
      if (lifetime.aborted) return;
      if (error instanceof ApiError && [400, 401, 403, 404].includes(error.status)) {
        onStatus("denied");
        return;
      }
    } finally {
      window.clearTimeout(watchdog);
      document.removeEventListener("visibilitychange", pause);
      lifetime.removeEventListener("abort", abort);
      abort();
    }
    if (!document.hidden && !lifetime.aborted) {
      onStatus("reconnecting");
      await waitForStream(lifetime, false);
    }
  }
}

export function streamOps(path: string, onSnapshot: (value: z.infer<typeof opsLiveSchema>) => void, onStatus: (status: UsageStreamStatus) => void, signal?: AbortSignal) {
  return streamSnapshot(path, opsLiveSchema, onSnapshot, onStatus, signal);
}

export async function streamSnapshot<Schema extends z.ZodTypeAny>(
  path: string,
  snapshot: Schema,
  onSnapshot: (value: z.output<Schema>) => void,
  onStatus: (status: UsageStreamStatus) => void,
  signal?: AbortSignal,
) {
  const eventSchema = z.object({
    type: z.enum(["snapshot", "heartbeat", "unavailable"]),
    snapshot: snapshot.optional(),
    reason: z.string().optional(),
    server_time: instant,
  });
  const lifetime = signal ?? new AbortController().signal;
  const trace = traceHex(16);
  while (!lifetime.aborted) {
    if (document.hidden) {
      onStatus("paused");
      await waitForStream(lifetime, true);
      continue;
    }
    onStatus("connecting");
    const connection = new AbortController();
    const abort = () => connection.abort();
    const pause = () => { if (document.hidden) abort(); };
    document.addEventListener("visibilitychange", pause);
    lifetime.addEventListener("abort", abort, {once: true});
    let watchdog: number | undefined;
    const renew = () => {
      window.clearTimeout(watchdog);
      watchdog = window.setTimeout(abort, 35000);
    };
    renew();
    try {
      const response = await request(path, "GET", undefined, undefined, connection.signal, trace);
      if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
        throw new ApiError(502, "invalid_response");
      }
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
      let buffered = "";
      try {
        while (!connection.signal.aborted) {
          const {done, value} = await reader.read();
          if (done || connection.signal.aborted) break;
          buffered += value;
          let end: number;
          while ((end = buffered.indexOf("\n\n")) !== -1) {
            const data = buffered.slice(0, end).split("\n").find(line => line.startsWith("data: "))?.slice(6);
            buffered = buffered.slice(end + 2);
            if (!data) continue;
            const parsed = eventSchema.parse(JSON.parse(data));
            renew();
            if (parsed.type === "unavailable") {
              if (parsed.reason === "not_allowed") { onStatus("denied"); return; }
              onStatus("reconnecting");
              continue;
            }
            if (parsed.type === "snapshot" && parsed.snapshot) {
              onStatus("live");
              onSnapshot(parsed.snapshot);
            }
          }
        }
      } finally {
        await reader.cancel().catch(() => {});
      }
    } catch (error) {
      if (lifetime.aborted) return;
      if (error instanceof ApiError && [400, 401, 403, 404].includes(error.status)) {
        onStatus("denied");
        return;
      }
    } finally {
      window.clearTimeout(watchdog);
      document.removeEventListener("visibilitychange", pause);
      lifetime.removeEventListener("abort", abort);
      abort();
    }
    if (!document.hidden && !lifetime.aborted) {
      onStatus("reconnecting");
      await waitForStream(lifetime, false);
    }
  }
}
