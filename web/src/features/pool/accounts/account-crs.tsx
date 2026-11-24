import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction} from "@/lib/requests";
import {Button, Checkbox, DataTable, ErrorLine, Feedback, Field, Form, InlineStack, Input, InsetGroup, InsetRow, Overlay, PasswordInput, PlatformBadge, Status, toast} from "@/ui";

const candidate = z.object({crs_account_id: z.string(), name: z.string(), platform: z.string(), type: z.string()});
const previewSchema = z.object({new_accounts: z.array(candidate), existing_accounts: z.array(candidate)});
const outcomeSchema = z.object({created: z.number(), updated: z.number(), skipped: z.number(), failed: z.number(),
  items: z.array(z.object({crs_account_id: z.string(), name: z.string(), action: z.string()}))});
const status: Record<string, string> = {created: "已创建", updated: "已更新", skipped: "已跳过", failed: "失败"};
const base = "/admin/accounts/sync/crs";
type Connection = {base_url: string; username: string; password: string};

export function AccountCRS({onClose, onChanged}: {onClose: () => void; onChanged: () => void}) {
  const [connection, setConnection] = useState<Connection>();
  const [preview, setPreview] = useState<z.infer<typeof previewSchema>>();
  const [outcome, setOutcome] = useState<z.infer<typeof outcomeSchema>>();
  const [selected, select] = useState<string[]>([]);
  const [syncProxies, setSyncProxies] = useState(false);
  const action = useAction();
  return <Overlay open drawer title="同步 CRS 上游账号" onClose={onClose} busy={action.busy}>
    {outcome ? <>
      <Feedback>{["创建 " + outcome.created, "更新 " + outcome.updated, "跳过 " + outcome.skipped, "失败 " + outcome.failed].join(" · ")}</Feedback>
      <DataTable rows={outcome.items} rowID={item => item.crs_account_id} label="CRS 同步结果" columns={[
        {id: "name", title: "账号", render: item => item.name},
        {id: "status", title: "结果", align: "center", render: item => <Status value={item.action === "failed" ? "failed" : item.action === "skipped" ? "skip" : "ok"} label={status[item.action] ?? "未知"} />},
      ]} />
    </> : preview && connection ? <Form busy={action.busy} errors={action.fieldErrors} onSubmit={() => void action.run(async () => {
      const result = await api(base, outcomeSchema, "POST", {...connection, sync_proxies: syncProxies, selected_account_ids: selected});
      setConnection(undefined);
      setOutcome(result);
      toast(`CRS 同步完成：新增 ${result.created}，更新 ${result.updated}`);
      onChanged();
    })}>
      {preview.existing_accounts.length > 0 && <InsetGroup title="将更新现有账号">
        {preview.existing_accounts.map(item => <InsetRow label={item.name} hint={item.type} key={item.crs_account_id}><PlatformBadge platform={item.platform} /></InsetRow>)}
      </InsetGroup>}
      {preview.new_accounts.length > 0 && <InsetGroup title="待导入的新账号">
        {preview.new_accounts.map(item => <InsetRow label={item.name} hint={item.type} key={item.crs_account_id}>
          <PlatformBadge platform={item.platform} />
          <Checkbox label={"导入 " + item.name} checked={selected.includes(item.crs_account_id)}
            onChange={checked => select(current => checked ? [...current, item.crs_account_id] : current.filter(id => id !== item.crs_account_id))} />
        </InsetRow>)}
      </InsetGroup>}
      {!preview.existing_accounts.length && !preview.new_accounts.length && <Feedback>这个 CRS 实例没有可同步的账号。</Feedback>}
      <InsetRow label="同步代理配置"><Checkbox label="同步代理" checked={syncProxies} onChange={setSyncProxies} /></InsetRow>
      <ErrorLine value={action.error} />
      <InlineStack>
        <Button variant="outline" onClick={() => { setPreview(undefined); setConnection(undefined); action.clearError(); }}>返回连接设置</Button>
        <Button type="submit" busy={action.busy} disabled={preview.new_accounts.length > 0 ? !selected.length : !preview.existing_accounts.length}>确认同步</Button>
      </InlineStack>
    </Form> : <Form busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
      const data = new FormData(event.currentTarget);
      const input = {base_url: String(data.get("base_url") ?? ""), username: String(data.get("username") ?? ""), password: String(data.get("password") ?? "")};
      void action.run(async () => {
        const result = await api(base + "/preview", previewSchema, "POST", input);
        select(result.new_accounts.map(item => item.crs_account_id));
        setConnection(input);
        setPreview(result);
      });
    }}>
      <Field label="CRS 地址"><Input type="url" name="base_url" placeholder="https://relay.example.com" required /></Field>
      <Field label="CRS 管理员"><Input name="username" autoComplete="off" required /></Field>
      <Field label="CRS 密码"><PasswordInput name="password" toggleLabel="CRS 密码" autoComplete="off" required /></Field>
      <ErrorLine value={action.error} />
      <Button type="submit" busy={action.busy}>预览账号</Button>
    </Form>}
  </Overlay>;
}
