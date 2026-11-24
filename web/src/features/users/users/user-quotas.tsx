import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {money} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {Button, ConfirmDialog, ErrorLine, Field, Form, InlineStack, Input, InsetGroup, InsetRow, Overlay, PLATFORM_OPTIONS, Resource, Select, platformLabel, toast} from "@/ui";

const quota = z.object({platform: z.string(), daily_limit_usd: z.number().nullable(), weekly_limit_usd: z.number().nullable(), monthly_limit_usd: z.number().nullable(),
  daily_usage_usd: z.number(), weekly_usage_usd: z.number(), monthly_usage_usd: z.number()});
const schema = z.object({platform_quotas: z.array(quota)});
const windows = [["daily", "每日"], ["weekly", "每周"], ["monthly", "每月"]] as const;
type Quota = z.infer<typeof quota>;
type WindowName = typeof windows[number][0];
type Reset = {platform: string; window: WindowName; label: string};

export function UserQuotas({userID, onClose}: {userID: number; onClose: () => void}) {
  const path = "/admin/users/" + userID + "/platform-quotas";
  const [revision, refresh] = useState(0);
  const data = useResource(() => api(path, schema), path, true, revision);
  return <Overlay open title="平台配额" onClose={onClose}>
    <Resource state={data}>{value => <QuotaEditor key={revision} path={path} quotas={value.platform_quotas} onChanged={() => refresh(n => n + 1)} />}</Resource>
  </Overlay>;
}

function QuotaEditor({path, quotas, onChanged}: {path: string; quotas: Quota[]; onChanged: () => void}) {
  const [draft, setDraft] = useState(quotas);
  const [toAdd, setToAdd] = useState("");
  const [reset, setReset] = useState<Reset>();
  const form = useId();
  const save = useAction();
  const resetting = useAction();
  const choices = PLATFORM_OPTIONS.filter(option => option.value !== "" && option.value !== "composite" && !draft.some(row => row.platform === option.value));
  return <>
    <Form id={form} busy={save.busy} errors={save.fieldErrors} onSubmit={() => void save.run(async () => {
      await api(path, schema, "PUT", {quotas: draft.map(({platform, daily_limit_usd, weekly_limit_usd, monthly_limit_usd}) =>
        ({platform, daily_limit_usd, weekly_limit_usd, monthly_limit_usd}))});
      toast("平台配额已保存");
      onChanged();
    })}>
      {draft.map(row => <InsetGroup key={row.platform} title={platformLabel(row.platform)} actions={<Button size="xs" variant="ghost"
        onClick={() => setDraft(items => items.filter(item => item.platform !== row.platform))}>移除配额</Button>}>
        {windows.map(([window, label]) => {
          const key = `${window}_limit_usd` as const;
          const existing = quotas.find(item => item.platform === row.platform);
          return <InsetRow key={window} label={label} hint={existing ? "已用 " + money(existing[`${window}_usage_usd`]) : undefined}>
            <div className="row-field">
              <Input aria-label={platformLabel(row.platform) + label + "额度"} type="number" suffix="USD" min={0} step="any" value={row[key] ?? ""} placeholder="不限"
                onChange={event => {
                  const amount = event.target.value === "" ? null : Number(event.target.value);
                  setDraft(items => items.map(item => item.platform === row.platform ? {...item, [key]: amount} : item));
                }} />
            </div>
            {existing && <Button size="xs" variant="outline" disabled={draft !== quotas} onClick={() => {
              resetting.clearError();
              setReset({platform: row.platform, window, label});
            }}>重置用量</Button>}
          </InsetRow>;
        })}
      </InsetGroup>)}
      {choices.length > 0 && <Field label="新增平台配额">
        <InlineStack>
          <Select aria-label="选择平台" value={toAdd} onChange={event => setToAdd(event.target.value)}>
            <option value="">选择平台</option>
            {choices.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
          </Select>
          <Button type="button" variant="outline" disabled={!toAdd} onClick={() => {
            setDraft(rows => [...rows, {platform: toAdd, daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null,
              daily_usage_usd: 0, weekly_usage_usd: 0, monthly_usage_usd: 0}]);
            setToAdd("");
          }}>添加</Button>
        </InlineStack>
      </Field>}
      <ErrorLine value={save.error} />
      <Button type="submit" busy={save.busy} disabled={draft === quotas}>保存配额</Button>
    </Form>
    {reset && <ConfirmDialog open title="重置平台用量" description={`将 ${platformLabel(reset.platform)} 的${reset.label}累计用量清零。`}
      confirm="重置用量" busy={resetting.busy} error={resetting.error} onClose={() => setReset(undefined)}
      onConfirm={() => void resetting.run(async () => {
        await api(path + "/reset", schema, "POST", {platform: reset.platform, window: reset.window});
        toast("平台窗口用量已重置");
        setReset(undefined);
        onChanged();
      })} />}
  </>;
}
