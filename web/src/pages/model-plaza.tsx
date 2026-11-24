import {useEffect, useState} from "react";
import {z} from "zod";
import {api, named, type Config} from "../api";
import {count, preciseMoney} from "../format";
import {useResource} from "../requests";
import {navigatePublic} from "../route";
import {Button, DataTable, EllipsisText, EmptyState, FilterBar, InlineStack, InsetGroup, InsetRow, PlatformBadge, PublicLayout, Resource, SearchInput, WorkspaceContent} from "../ui";

const amount = z.number().nullish();
const interval = z.object({min_tokens: z.number(), max_tokens: amount, tier_label: z.string().optional(), input_price: amount,
  output_price: amount, per_request_price: amount, input_multiplier: amount, output_multiplier: amount});
const price = z.object({billing_mode: z.string().optional(), input_price: amount, output_price: amount,
  cache_read_price: amount, cache_write_price: amount, image_output_price: amount, per_request_price: amount, intervals: z.array(interval).optional()});
const model = z.object({name: z.string(), pricing: price.nullable(), official_pricing: price.nullable(), long_context_basis: z.string().optional(),
  time_pricing: z.object({timezone: z.string(), weekdays_only: z.boolean().optional(), periods: z.array(z.object({start_time: z.string(), end_time: z.string(), multiplier: z.number()}))}).nullish()});
const group = named.extend({description: z.string(), platform: z.string(), subscription_type: z.string(), rate_multiplier: z.number(), user_rate_multiplier: amount,
  peak_rate_enabled: z.boolean(), peak_start: z.string().optional(), peak_end: z.string().optional(), peak_rate_multiplier: z.number(),
  is_exclusive: z.boolean(), image_rate_independent: z.boolean(), image_rate_multiplier: z.number(), video_rate_independent: z.boolean(),
  video_rate_multiplier: z.number(), long_context_pricing_enabled: z.boolean(), models: z.array(model)});
const plaza = z.object({description: z.string(), groups: z.array(group)});
type Group = z.infer<typeof group>;
type Model = z.infer<typeof model>;
const usd = (value: number | null | undefined, scale = 1) => value == null ? "—" : preciseMoney.format(value * scale);

export function PublicModelPlaza({config}: {config: Config}) {
  const site = config.site_name || "Sub2API";
  useEffect(() => { document.title = "模型广场 · " + site; }, [site]);
  return (
    <PublicLayout
      siteName={site}
      siteLogo={config.site_logo || undefined}
      subtitle="模型广场与公开价目"
      contactInfo={config.contact_info}
      docUrl={config.doc_url}
      actions={
        <>
          <Button size="sm" variant="ghost" onClick={() => navigatePublic("home")}>返回首页</Button>
          <Button size="sm" onClick={() => navigatePublic("login")}>登录</Button>
        </>
      }
    >
      {config.model_plaza_enabled ? <ModelPlaza guest /> : <EmptyState title="站点尚未开放模型广场" />}
    </PublicLayout>
  );
}

export function ModelPlaza({guest = false}: {guest?: boolean}) {
  const path = guest ? "/public/model-plaza" : "/model-plaza";
  const data = useResource(() => api(path, plaza, "GET", undefined, guest ? false : undefined), path);
  const [search, setSearch] = useState("");
  const [open, setOpen] = useState<{group: number; name: string}>();
  const keyword = search.trim().toLowerCase();
  return <WorkspaceContent>
    <FilterBar><div className="obs-filter-row"><SearchInput value={search} onChange={setSearch} placeholder="搜索模型或分组" /></div></FilterBar>
    <Resource state={data}>{result => {
      const visible = result.groups.map(row => ({...row,
        models: !keyword || row.name.toLowerCase().includes(keyword) ? row.models : row.models.filter(item => item.name.toLowerCase().includes(keyword)),
      })).filter(row => row.models.length || !keyword);
      return <>
        {result.description && <p className="preserve-lines">{result.description}</p>}
        {visible.length ? visible.map(row => <InsetGroup key={row.id} footer={row.description} title={row.name}
          actions={<InlineStack><PlatformBadge platform={row.platform} />{row.is_exclusive ? "专属" : row.subscription_type === "subscription" ? "订阅" : null}</InlineStack>}>
          {row.models.length ? <DataTable rows={row.models} rowID={item => item.name} label={row.name + "模型价格"} columns={[
            {id: "name", title: "模型", render: item => <EllipsisText text={item.name} mono />},
            {id: "input", title: "输入 / 百万 Token", align: "number", render: item => usd(item.pricing?.input_price, scale(row, item) * 1e6)},
            {id: "output", title: "输出 / 百万 Token", align: "number", hideWhenCompact: true, render: item => usd(item.pricing?.output_price, scale(row, item) * 1e6)},
            {id: "request", title: "每次请求", align: "number", hideWhenNarrow: true, render: item => usd(item.pricing?.per_request_price, scale(row, item))},
            {id: "detail", title: "价格详情", align: "end", render: item => <Button variant="ghost" size="sm" aria-expanded={open?.group === row.id && open.name === item.name}
              onClick={() => setOpen(open?.group === row.id && open.name === item.name ? undefined : {group: row.id, name: item.name})}>明细</Button>},
          ]} /> : <InsetRow label="暂无定价模型" />}
          {open?.group === row.id && row.models.filter(item => item.name === open.name).map(item => <ModelPriceDetail key={item.name} row={row} item={item} />)}
        </InsetGroup>) : <EmptyState title={keyword ? "没有匹配的模型或分组" : "暂无可见分组"} />}
      </>;
    }}</Resource>
  </WorkspaceContent>;
}

function scale(row: Group, item: Model) {
  if (item.pricing?.billing_mode === "image" && row.image_rate_independent) return row.image_rate_multiplier;
  if (item.pricing?.billing_mode === "video" && row.video_rate_independent) return row.video_rate_multiplier;
  return row.user_rate_multiplier ?? row.rate_multiplier;
}

function ModelPriceDetail({row, item}: {row: Group; item: Model}) {
  const multiplier = scale(row, item);
  const pricing = item.pricing, reference = item.official_pricing;
  return <>
    <InsetRow label={item.name} hint={"生效倍率 ×" + multiplier} />
    {pricing && <>
      <InsetRow label="缓存读写 / 百万 Token" hint={usd(pricing.cache_read_price, multiplier * 1e6) + " / " + usd(pricing.cache_write_price, multiplier * 1e6)} />
      {pricing.image_output_price != null && <InsetRow label="每张输出图" hint={usd(pricing.image_output_price, multiplier)} />}
    </>}
    {reference && <InsetRow label="官方参考输入 / 输出（百万 Token）" hint={usd(reference.input_price, 1e6) + " / " + usd(reference.output_price, 1e6)} />}
    {row.peak_rate_enabled && <InsetRow label="分组高峰倍率" hint={row.peak_start + "–" + row.peak_end + " ×" + row.peak_rate_multiplier} />}
    {item.time_pricing?.periods.map(period => <InsetRow key={period.start_time} label={period.start_time + "–" + period.end_time}
      hint={item.time_pricing?.timezone + (item.time_pricing?.weekdays_only ? " · 工作日" : "") + " · ×" + (multiplier * period.multiplier)} />)}
    {row.long_context_pricing_enabled && pricing?.intervals?.map(interval => <InsetRow key={interval.min_tokens} label={interval.tier_label ?? (count.format(interval.min_tokens) + " Token 起")}
      hint={["输入 " + usd(interval.input_price ?? (pricing.input_price == null || interval.input_multiplier == null ? null : pricing.input_price * interval.input_multiplier), multiplier * 1e6),
        "输出 " + usd(interval.output_price ?? (pricing.output_price == null || interval.output_multiplier == null ? null : pricing.output_price * interval.output_multiplier), multiplier * 1e6),
        item.long_context_basis === "marginal" ? "仅超出部分按档" : "整单按档"].join(" · ")} />)}
  </>;
}
