import {useEffect, useState} from "react";
import {ArrowLeft} from "lucide-react";
import {z} from "zod";
import type {Config} from "@/schemas";
import {Brand} from "@/app/appearance";
import {count, date, money} from "@/lib/format";
import {useAction} from "@/lib/requests";
import {navigatePublic} from "@/app/route";
import {Button, DataTable, ErrorLine, Field, Form, FormGrid, Input, InsetGroup, InsetRow, PasswordInput, Stack, StageCard, Status, type Column} from "@/ui";

const totals = z.object({requests: z.number(), total_tokens: z.number(), actual_cost: z.number()});
const limit = z.object({used: z.number(), limit: z.number()});
const model = totals.extend({model: z.string()});
const day = totals.extend({date: z.string()});
const usage = z.object({
  mode: z.enum(["quota_limited", "unrestricted"]), status: z.string().optional(), planName: z.string().optional(),
  balance: z.number().optional(), remaining: z.number().optional(), expires_at: z.string().optional(), quota: limit.optional(),
  usage: z.object({today: totals, total: totals, rpm: z.number(), tpm: z.number()}).optional(),
  rate_limits: z.array(limit.extend({window: z.string(), reset_at: z.string().optional()})).optional(),
  subscription: z.object({daily_usage_usd: z.number(), daily_limit_usd: z.number().nullable(),
    weekly_usage_usd: z.number(), weekly_limit_usd: z.number().nullable(), monthly_usage_usd: z.number(),
    monthly_limit_usd: z.number().nullable(), expires_at: z.string().optional()}).optional(),
  daily_usage: z.array(day).optional(), model_stats: z.array(model).optional(),
});
const numbers: Column<z.infer<typeof totals>>[] = [
  {id: "requests", title: "请求数", align: "number", render: row => count.format(row.requests)},
  {id: "tokens", title: "Token", align: "number", render: row => count.format(row.total_tokens)},
  {id: "cost", title: "实扣", align: "number", render: row => money(row.actual_cost)},
];
const windows: Record<string, string> = {"5h": "5 小时", "1d": "1 天", "7d": "7 天"};

export function PublicKeyUsage({config}: {config: Config}) {
  const [result, setResult] = useState<z.infer<typeof usage>>();
  const action = useAction();
  const site = config.site_name || "Sub2API";
  useEffect(() => { document.title = "密钥用量 · " + site; }, [site]);
  return (
    <StageCard
      size={result ? "lg" : "md"}
      title="密钥用量"
      subtitle="使用 API 密钥直接查询剩余额度、窗口速率及最近模型消耗"
      brand={<Brand name={site} logo={config.site_logo || undefined} />}
      topActions={
        <Button variant="ghost" size="sm" onClick={() => navigatePublic("home")}>
          <ArrowLeft aria-hidden />返回首页
        </Button>
      }
    >
      <Form busy={action.busy} onSubmit={event => {
        const fields = new FormData(event.currentTarget);
        setResult(undefined);
        void action.run(async () => {
          const start = String(fields.get("start_date") ?? ""), end = String(fields.get("end_date") ?? "");
          if (Boolean(start) !== Boolean(end) || start > end) throw new Error("请选择完整的模型统计日期范围");
          const query = new URLSearchParams({days: "30", timezone: Intl.DateTimeFormat().resolvedOptions().timeZone});
          if (start && end) {
            if ((Date.parse(end) - Date.parse(start)) / 86400000 >= 90) throw new Error("模型统计日期最多查询 90 天");
            query.set("start_date", start);
            query.set("end_date", end);
          }
          const endpoint = new URL("/v1/usage", config.dev_accounts?.length ? location.origin : config.official);
          endpoint.search = query.toString();
          let response: Response;
          try {
            response = await fetch(endpoint, {credentials: "omit", cache: "no-store", redirect: "error",
              headers: {Authorization: "Bearer " + String(fields.get("key") ?? "").trim()}, signal: AbortSignal.timeout(15000)});
          } catch {
            throw new Error("无法连接官方网关。请检查地址以及网关是否允许当前站点跨域访问。");
          }
          if (response.status === 401 || response.status === 403) throw new Error("密钥无效，或当前来源无权查询。");
          if (!response.ok) throw new Error("查询失败（" + response.status + "）");
          const decoded = usage.safeParse(await response.json());
          if (!decoded.success) throw new Error("官方网关返回了无法识别的用量数据。");
          setResult(decoded.data);
        });
      }}>
        <Field label="API 密钥">
          <PasswordInput name="key" toggleLabel="API 密钥" required maxLength={512} autoComplete="off" />
        </Field>
        <FormGrid>
          <Field label="统计开始日期"><Input type="date" name="start_date" /></Field>
          <Field label="统计结束日期"><Input type="date" name="end_date" /></Field>
        </FormGrid>
        <ErrorLine value={action.error} />
        <Button busy={action.busy} type="submit">查询用量</Button>
      </Form>
      {result && <KeyUsageResult result={result} />}
    </StageCard>
  );
}

function KeyUsageResult({result}: {result: z.infer<typeof usage>}) {
  const subscription = result.subscription;
  const periods = subscription ? [
    {name: "每日", used: subscription.daily_usage_usd, limit: subscription.daily_limit_usd},
    {name: "每周", used: subscription.weekly_usage_usd, limit: subscription.weekly_limit_usd},
    {name: "每月", used: subscription.monthly_usage_usd, limit: subscription.monthly_limit_usd},
  ] : [];
  return <Stack>
    <InsetGroup title="额度与状态">
      {result.status && <InsetRow label="密钥状态"><Status value={result.status} /></InsetRow>}
      {result.planName && <InsetRow label="计费方式" hint={result.planName} />}
      {result.balance != null && <InsetRow label="账户余额" hint={money(result.balance)} />}
      {result.quota && <InsetRow label="密钥总额度" hint={money(result.quota.used) + " / " + money(result.quota.limit)} />}
      {result.balance == null && result.remaining != null && <InsetRow label="剩余额度" hint={money(result.remaining)} />}
      {result.expires_at && <InsetRow label="密钥到期" hint={date(result.expires_at)} />}
      {subscription?.expires_at && <InsetRow label="订阅到期" hint={date(subscription.expires_at)} />}
      {periods.filter(row => row.limit != null && row.limit > 0).map(row => <InsetRow key={row.name} label={row.name + "订阅额度"}
        hint={money(row.used) + " / " + money(row.limit)} />)}
      {result.rate_limits?.map(window => <InsetRow key={window.window} label={(windows[window.window] ?? window.window) + "窗口"}
        hint={money(window.used) + " / " + money(window.limit)}>{window.reset_at && <span>重置于 {date(window.reset_at)}</span>}</InsetRow>)}
    </InsetGroup>
    {result.usage && <>
      <DataTable label="用量合计" rows={[{period: "今日", ...result.usage.today}, {period: "累计", ...result.usage.total}]}
        rowID={row => row.period} columns={[{id: "period", title: "范围", render: row => row.period}, ...numbers]} />
      <InsetGroup><InsetRow label="RPM / TPM" hint={count.format(result.usage.rpm) + " / " + count.format(result.usage.tpm)} /></InsetGroup>
    </>}
    {!!result.daily_usage?.length && <InsetGroup title="近 30 天用量"><DataTable label="按日用量" rows={result.daily_usage} rowID={row => row.date}
      columns={[{id: "date", title: "日期", render: row => row.date}, ...numbers]} /></InsetGroup>}
    {!!result.model_stats?.length && <InsetGroup title="模型统计"><DataTable label="按模型统计" rows={result.model_stats} rowID={row => row.model}
      columns={[{id: "model", title: "模型", render: row => row.model}, ...numbers]} /></InsetGroup>}
  </Stack>;
}
