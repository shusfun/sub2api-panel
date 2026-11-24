import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty, type Config} from "@/schemas";
import {Brand} from "@/app/appearance";
import {date} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {
  Button, ConfirmDialog, CopyValue, DataTable, EllipsisText, EmptyState, ErrorLine, Field, Form, InlineStack, InsetGroup, InsetRow,
  OtpInput, RefreshButton, Resource, Select, StageCard, Status, WorkspaceContent, toast, type Column,
} from "@/ui";

const grant = z.object({
  id: z.string(), client_name: z.string(), scope: z.enum(["admin", "user"]), read_only: z.boolean(), created_at: z.string(),
  expires_at: z.string(), last_used_at: z.string().nullish(), revoked_at: z.string().nullish(), invalidated: z.boolean(),
});
const consent = z.object({client_name: z.string(), redirect_uri: z.string(), resource: z.string()});
const base = "/mcp";
type Grant = z.infer<typeof grant>;
const scopeLabel = (scope: string) => scope === "admin" ? "管理工作区" : "个人工作区";

export function MCPGrants() {
  const [revision, revise] = useState(0);
  const [selected, select] = useState<Grant>();
  const action = useAction();
  const server = useResource(() => api(base + "/config", z.object({resource: z.string()})), "mcp-config");
  const grants = useResource(() => api(base + "/grants", z.array(grant)), "mcp-grants", true, revision);
  const columns: Column<Grant>[] = [
    {id: "client", title: "客户端", render: row => <EllipsisText text={row.client_name} />},
    {id: "scope", title: "权限范围", hideWhenCompact: true, render: row => scopeLabel(row.scope) + (row.read_only ? " · 只读" : " · 可写")},
    {id: "expiry", title: "到期时间", hideWhenNarrow: true, render: row => date(row.expires_at)},
    {id: "last", title: "最近使用", hideWhenNarrow: true, render: row => row.last_used_at ? date(row.last_used_at) : "尚未使用"},
    {
      id: "status",
      title: "状态",
      align: "center",
      render: row => {
        const key = row.revoked_at ? "revoked" : row.invalidated ? "invalidated" : Date.parse(row.expires_at) <= Date.now() ? "expired" : "valid";
        return <Status value={key} />;
      },
    },
    {id: "action", title: "操作", align: "end", render: row => !row.revoked_at &&
      <Button variant="destructive" size="sm" onClick={() => { action.clearError(); select(row); }}>撤销</Button>},
  ];
  return <WorkspaceContent>
    <Resource state={server}>{row => <InsetGroup title="MCP 连接地址"><InsetRow label="服务器资源端点" vertical><CopyValue value={row.resource} label="连接地址" /></InsetRow></InsetGroup>}</Resource>
    <InsetGroup title="已授权客户端" actions={<RefreshButton size="xs" variant="ghost" busy={grants?.loading} onClick={() => revise(v => v + 1)} />}>
      <Resource state={grants}>{rows => rows.length ?
        <DataTable rows={rows} rowID={row => row.id} columns={columns} label="已授权客户端" /> : <EmptyState title="暂无已授权的客户端" />}</Resource>
    </InsetGroup>
    {selected && <ConfirmDialog open title="撤销 MCP 授权" description={selected.client_name + " 将无法继续通过此令牌访问工作区。"} confirm="撤销授权"
      busy={action.busy} error={action.error} onClose={() => select(undefined)} onConfirm={() => void action.run(async () => {
        await api(base + "/grants/" + encodeURIComponent(selected.id), empty, "DELETE");
        revise(value => value + 1);
        select(undefined);
        toast("已撤销客户端授权");
      })} />}
  </WorkspaceContent>;
}

const oauthFields = ["response_type", "client_id", "redirect_uri", "scope", "state", "code_challenge", "code_challenge_method", "resource"];

export function MCPConsent({config, admin, query}: {config: Config; admin: boolean; query: string}) {
  const action = useAction();
  const [scope, chooseScope] = useState("user");
  const [readOnly, chooseReadOnly] = useState(true);
  const params = new URLSearchParams(query);
  const authorization = Object.fromEntries(oauthFields.map(name => [name, params.get(name) ?? ""]));
  const path = base + "/authorize";
  const preview = useResource(() => api(path + "?" + query, consent), query, config.mcp_enabled);

  function decide(approved: boolean, totp = "") {
    void action.run(async () => {
      const result = await api(path, z.object({redirect_url: z.string()}), "POST",
        {authorization, approved, scope, read_only: readOnly, ...(totp ? {totp} : {})});
      location.assign(result.redirect_url);
    });
  }
  return (
    <StageCard
      size="md"
      title="MCP 客户端授权确认"
      brand={<Brand name={config.site_name || "Sub2API"} logo={config.site_logo || undefined} />}
    >
      {config.mcp_enabled ? <Resource state={preview}>{row => <>
        <InsetGroup>
          <InsetRow label="请求客户端" hint={row.client_name} />
          <InsetRow label="访问资源"><span className="code-mono">{row.resource}</span></InsetRow>
          <InsetRow label="回调地址"><span className="code-mono">{row.redirect_uri}</span></InsetRow>
        </InsetGroup>
        <Form busy={action.busy} errors={action.fieldErrors} onSubmit={event => decide(true, String(new FormData(event.currentTarget).get("totp") ?? ""))}>
          {admin && <Field label="授权范围" name="scope"><Select value={scope} onChange={event => chooseScope(event.target.value)}>
            <option value="user">个人工作区</option><option value="admin">管理工作区</option>
          </Select></Field>}
          <Field label="读写权限"><Select value={readOnly ? "read" : "write"} onChange={event => chooseReadOnly(event.target.value === "read")}>
            <option value="read">只读访问</option><option value="write">允许修改</option>
          </Select></Field>
          {scope === "admin" && <Field label="验证器 6 位验证码" name="totp"><OtpInput name="totp" required /></Field>}
          <ErrorLine value={action.error} />
          <InlineStack>
            <Button variant="outline" onClick={() => decide(false)}>拒绝</Button>
            <Button type="submit" busy={action.busy}>允许访问</Button>
          </InlineStack>
        </Form>
      </>}</Resource> : <EmptyState title="站点尚未开放 MCP 授权" />}
    </StageCard>
  );
}
