import {useState} from "react";
import {api} from "@/lib/http";
import {date, smartMoney} from "@/lib/format";
import {useAction} from "@/lib/requests";
import {empty} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  Button, CollapsibleSection, ErrorLine, Field, Form, FormActions, FormSection,
  GroupSelect, Input, InsetGroup, InsetRow, Switch, Textarea,
} from "@/ui";
import type {KeyItem} from "./index";

export function parseKeyLines(value: string) {
  return value.split(/\r?\n/).map(item => item.trim()).filter(Boolean);
}

export function KeyInspectorForm({row, admin, onSaved}: {row: KeyItem; admin: boolean; onSaved: () => void}) {
  const save = useAction();
  const [name, setName] = useState(row.name);
  const [groupID, setGroupID] = useState(row.group_id ? String(row.group_id) : "");
  const [quota, setQuota] = useState(String(row.quota));
  const [limit5h, setLimit5h] = useState(String(row.rate_limit_5h));
  const [limit1d, setLimit1d] = useState(String(row.rate_limit_1d));
  const [limit7d, setLimit7d] = useState(String(row.rate_limit_7d));
  const [expiresAt, setExpiresAt] = useState(row.expires_at ? row.expires_at.slice(0, 10) : "");
  const [resetQuota, setResetQuota] = useState(false);
  const [resetWindows, setResetWindows] = useState(false);
  const [whitelist, setWhitelist] = useState(row.ip_whitelist.join("\n"));
  const [blacklist, setBlacklist] = useState(row.ip_blacklist.join("\n"));

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        await api(`/keys/${row.id}`, empty, "PUT", {
          name: name.trim(),
          ...(groupID ? {group_id: Number(groupID)} : {clear_group: true}),
          quota: Number(quota || 0),
          rate_limit_5h: Number(limit5h || 0),
          rate_limit_1d: Number(limit1d || 0),
          rate_limit_7d: Number(limit7d || 0),
          ...(expiresAt ? {expires_at: `${expiresAt}T23:59:59Z`} : {clear_expires: true}),
          ...(resetQuota ? {reset_quota: true} : {}),
          ...(resetWindows ? {reset_rate_limit_usage: true} : {}),
          ip_whitelist: parseKeyLines(whitelist),
          ip_blacklist: parseKeyLines(blacklist),
        });
        setResetQuota(false);
        setResetWindows(false);
        onSaved();
      })}
    >
      <InsetGroup title="实时运行状态">
        <InsetRow label="今日 / 累计实际扣费">
          <span className="weir-digits">
            {row.today_actual_cost === null ? "—" : smartMoney(row.today_actual_cost)} / {row.total_actual_cost === null ? "—" : smartMoney(row.total_actual_cost)}
          </span>
        </InsetRow>
        <InsetRow label="当前并发连接"><span className="weir-digits">{row.current_concurrency}</span></InsetRow>
        <InsetRow label="最近调用">
          <span className="code-mono text-fine">{row.last_used_at ? date(row.last_used_at) : "尚未使用"}{row.last_used_ip ? ` · ${row.last_used_ip}` : ""}</span>
        </InsetRow>
        <InsetRow label="账单下钻">
          <Button size="xs" variant="outline" onClick={() => navigateTo("usage", {user: admin ? "self" : undefined, key: row.id})}>
            查看该 Key 趋势与明细 →
          </Button>
        </InsetRow>
      </InsetGroup>

      <FormSection title="名称与分组绑定">
        <Field label="名称">
          <Input required maxLength={100} value={name} onChange={e => setName(e.target.value)} />
        </Field>
        <Field label="调用分组">
          <GroupSelect admin={false} value={groupID} placeholder="不绑定特定分组" onChange={setGroupID} />
        </Field>
      </FormSection>

      <FormSection title="额度与窗口限流（0 表示不限）">
        <Field label="总额度 (USD)" hint={`已用 ${smartMoney(row.quota_used)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={quota} onChange={e => setQuota(e.target.value)} /></div>
        </Field>
        <Field label="5 小时限额 (USD)" hint={`当前 ${smartMoney(row.usage_5h)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={limit5h} onChange={e => setLimit5h(e.target.value)} /></div>
        </Field>
        <Field label="24 小时限额 (USD)" hint={`当前 ${smartMoney(row.usage_1d)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={limit1d} onChange={e => setLimit1d(e.target.value)} /></div>
        </Field>
        <Field label="7 天限额 (USD)" hint={`当前 ${smartMoney(row.usage_7d)}`}>
          <div className="row-field"><Input type="number" min="0" step="0.01" value={limit7d} onChange={e => setLimit7d(e.target.value)} /></div>
        </Field>
        <Field label="到期日期（留空永不过期）">
          <div className="row-field"><Input type="date" value={expiresAt} onChange={e => setExpiresAt(e.target.value)} /></div>
        </Field>
        <InsetRow label="保存时清零累计已用额度">
          <Switch label="重置总额度计数" checked={resetQuota} onChange={setResetQuota} />
        </InsetRow>
        <InsetRow label="保存时清零 5h/1d/7d 窗口计数">
          <Switch label="重置窗口计数" checked={resetWindows} onChange={setResetWindows} />
        </InsetRow>
      </FormSection>

      <CollapsibleSection title="来源 IP 黑白名单" summary={`${row.ip_whitelist.length + row.ip_blacklist.length} 条规则`}>
        <Field label="白名单（每行一个 IP 或 CIDR）">
          <Textarea rows={2} value={whitelist} onChange={e => setWhitelist(e.target.value)} />
        </Field>
        <Field label="黑名单（每行一个 IP 或 CIDR）">
          <Textarea rows={2} value={blacklist} onChange={e => setBlacklist(e.target.value)} />
        </Field>
      </CollapsibleSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="submit" busy={save.busy}>保存修改</Button>
      </FormActions>
    </Form>
  );
}
