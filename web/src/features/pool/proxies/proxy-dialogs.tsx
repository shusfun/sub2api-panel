import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {proxyRow} from "@/schemas";
import {useAction} from "@/lib/requests";
import {
  Button,
  ErrorLine,
  Field,
  FilePicker,
  Form,
  FormGrid,
  Input,
  Overlay,
  PasswordInput,
  ProxySelect,
  Select,
  Textarea,
} from "@/ui";
import {type ProxyRow} from "@/features/pool/shared";

const proxyImportSchema = z.object({
  proxies: z.array(z.object({
    name: z.string().min(1).max(100),
    protocol: z.enum(["http", "https", "socks5", "socks5h"]),
    host: z.string().min(1).max(255),
    port: z.number().int().min(1).max(65535),
    username: z.string().optional(),
    password: z.string().optional(),
    has_auth: z.boolean().optional(),
  })).min(1).max(100),
});

export function CreateProxyDialog({open, revision, onClose, onCreated}: {open: boolean; revision: number; onClose: () => void; onCreated: (px: ProxyRow) => void}) {
  const formID = useId();
  const [backupID, setBackupID] = useState("");
  const [mode, setMode] = useState("none");
  const op = useAction();

  return (
    <Overlay open={open} title="添加代理出口" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>添加代理</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          const user = String(fd.get("username") ?? "").trim();
          const pass = String(fd.get("password") ?? "");
          void op.run(async () => {
            const expRaw = String(fd.get("expires_date") ?? "").trim();
            const expEpoch = expRaw ? Math.floor(new Date(expRaw + "T23:59:59Z").getTime() / 1000) : undefined;
            const warnDays = Number(fd.get("expiry_warn_days") || 0);
            const created = await api("/admin/proxies", proxyRow, "POST", {
              name: String(fd.get("name") ?? "").trim(),
              protocol: String(fd.get("protocol") ?? "http"),
              host: String(fd.get("host") ?? "").trim(),
              port: Number(fd.get("port") || 8080),
              ...(user ? {username: user} : {}),
              ...(pass ? {password: pass} : {}),
              fallback_mode: mode,
              ...(mode === "proxy" && backupID ? {backup_proxy_id: Number(backupID)} : {}),
              ...(expEpoch ? {expires_at: expEpoch} : {}),
              ...(warnDays > 0 ? {expiry_warn_days: warnDays} : {}),
            });
            onCreated(created);
          });
        }}
      >
        <FormGrid>
          <Field label="节点名称"><Input name="name" required maxLength={100} placeholder="东京住宅出口 01" /></Field>
          <Field label="协议">
            <Select name="protocol" defaultValue="http">
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="socks5">SOCKS5</option>
              <option value="socks5h">SOCKS5H</option>
            </Select>
          </Field>
          <Field label="主机地址"><Input name="host" required maxLength={255} placeholder="127.0.0.1" /></Field>
          <Field label="端口"><Input name="port" type="number" min="1" max="65535" defaultValue="8080" required /></Field>
          <Field label="认证用户名（可选）"><Input name="username" autoComplete="off" /></Field>
          <Field label="认证密码（可选）"><PasswordInput name="password" toggleLabel="代理密码" autoComplete="new-password" /></Field>
          <Field label="故障回退">
            <Select value={mode} onChange={event => setMode(event.target.value)}>
              <option value="none">不回退</option>
              <option value="direct">直连</option>
              <option value="proxy">备用代理</option>
            </Select>
          </Field>
          {mode === "proxy" && <Field label="备用代理"><ProxySelect value={backupID} revision={revision} placeholder="无备用代理" onChange={setBackupID} /></Field>}
          <Field label="到期日期（可选）"><Input name="expires_date" type="date" /></Field>
          <Field label="提前预警天数"><Input name="expiry_warn_days" type="number" min="0" defaultValue="0" /></Field>
        </FormGrid>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

export function PasteProxiesDialog({onClose, onImported}: {onClose: () => void; onImported: (message: string) => void}) {
  const formID = useId();
  const action = useAction();
  const [text, setText] = useState("");
  return <Overlay open title="粘贴代理地址" onClose={onClose} busy={action.busy} footer={<>
    <Button variant="outline" onClick={onClose}>取消</Button>
    <Button type="submit" form={formID} busy={action.busy}>创建代理</Button>
  </>}>
    <Form id={formID} onSubmit={() => void action.run(async () => {
      const lines = [...new Set(text.split(/\r?\n/).map(line => line.trim()).filter(Boolean))];
      if (!lines.length || lines.length > 100) throw new Error("请输入 1–100 个代理地址");
      const proxies = lines.map((line, index) => {
        try {
          const url = new URL(line.includes("://") ? line : "http://" + line);
          const protocol = url.protocol.slice(0, -1);
          const host = url.hostname.replace(/^\[|\]$/g, "");
          const port = Number(url.port || (protocol === "http" ? 80 : protocol === "https" ? 443 : 0));
          if (!["http", "https", "socks5", "socks5h"].includes(protocol) || !host || !port || (url.pathname && url.pathname !== "/") || url.search || url.hash) throw new Error();
          return {name: url.host, protocol, host, port, ...(url.username ? {username: decodeURIComponent(url.username)} : {}), ...(url.password ? {password: decodeURIComponent(url.password)} : {})};
        } catch {
          throw new Error("第 " + (index + 1) + " 行代理地址格式有误");
        }
      });
      const result = await api("/admin/proxies/batch", z.object({created: z.number(), skipped: z.number()}), "POST", {proxies});
      onImported("已创建 " + result.created + " 条代理，跳过 " + result.skipped + " 条");
    })}>
      <Field label="代理地址" hint="每行一个地址，IPv6 写成 [::1]:1080。"><Textarea required rows={7} value={text} onChange={event => setText(event.target.value)} placeholder="socks5://127.0.0.1:1080" /></Field>
      <ErrorLine value={action.error} />
    </Form>
  </Overlay>;
}

export function ImportProxiesDialog({open, onClose, onImported}: {open: boolean; onClose: () => void; onImported: (msg: string) => void}) {
  const formID = useId();
  const [text, setText] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const op = useAction();

  return (
    <Overlay open={open} title="批量导入代理 JSON" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>导入代理</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={() => void op.run(async () => {
          let json: unknown;
          try {
            json = JSON.parse(text);
          } catch {
            throw new Error("JSON 格式无效，需包含 1–100 条 proxies 数组。");
          }
          const parsed = proxyImportSchema.safeParse(json);
          if (!parsed.success) throw new Error("JSON 格式无效，需包含 1–100 条 proxies 数组。");
          const missing = parsed.data.proxies.filter(proxy => proxy.has_auth && (!proxy.username || proxy.password === undefined));
          if (missing.length) throw new Error("有 " + missing.length + " 条代理缺少认证字段。请补录 username 和 password 后重新导入。");
          const proxies = parsed.data.proxies.map(({has_auth, ...proxy}) => proxy);
          const res = await api("/admin/proxies/import", z.object({created: z.number(), skipped: z.number()}), "POST", {proxies});
          setText("");
          setFiles([]);
          onImported(`导入完成：新增 ${res.created} 条，跳过 ${res.skipped} 条`);
        })}
      >
        <Field label="选择 JSON 文件">
          <FilePicker
            accept="application/json,.json"
            maxBytes={5 * 1024 * 1024}
            hint="选择导出的 proxies.json 自动读取内容"
            value={files}
            onChange={picked => {
              setFiles(picked);
              const f = picked[0];
              if (f) void f.text().then(setText);
            }}
          />
        </Field>
        <Field label="代理 JSON 内容" hint="从本面板导出的配置已脱敏。有认证的代理需要补录用户名和密码。">
          <Textarea rows={6} required value={text} onChange={e => setText(e.target.value)} placeholder='{"proxies":[{"name":"JP-01","protocol":"socks5","host":"127.0.0.1","port":1080}]}' />
        </Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
