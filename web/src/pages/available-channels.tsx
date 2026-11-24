import {z} from "zod";
import {api, named} from "../api";
import {useResource} from "../requests";
import {DataTable, EmptyState, InsetGroup, InsetRow, PlatformBadge, Resource, WorkspaceContent} from "../ui";

const price = z.number().nullish();
const model = z.object({name: z.string(), pricing: z.object({input_price: price, output_price: price, per_request_price: price}).nullish()});
const channelList = z.array(z.object({name: z.string(), description: z.string(), platforms: z.array(z.object({
  platform: z.string(), groups: z.array(named.extend({rate_multiplier: z.number(), subscription_type: z.string()})).nullable(),
  supported_models: z.array(model).nullable(),
})).nullable()}));
const usd = new Intl.NumberFormat("zh-CN", {style: "currency", currency: "USD", maximumFractionDigits: 6});
const formatPrice = (value: number | null | undefined, multiplier = 1) => value == null ? "—" : usd.format(value * multiplier);

export function AvailableChannels() {
  const channels = useResource(() => api("/channels/available", channelList), "available-channels");
  return <WorkspaceContent>
    <Resource state={channels}>{items => items.length ? items.map((channel, index) =>
      <InsetGroup title={channel.name} key={JSON.stringify([channel.name, index])} footer={channel.description || undefined}>
        {channel.platforms?.map(platform => <div key={platform.platform}>
          <InsetRow label={<PlatformBadge platform={platform.platform} />} hint={platform.groups?.map(group =>
            group.name + (group.subscription_type === "subscription" ? "（订阅）" : "") + " ×" + group.rate_multiplier).join(" · ")} />
          {!!platform.supported_models?.length && <DataTable label={channel.name + " · 模型价格"} rows={platform.supported_models}
            rowID={row => row.name} columns={[
              {id: "model", title: "模型", render: row => row.name},
              {id: "in", title: "输入 / 百万 Token", align: "number", render: row => formatPrice(row.pricing?.input_price, 1e6)},
              {id: "out", title: "输出 / 百万 Token", align: "number", render: row => formatPrice(row.pricing?.output_price, 1e6)},
              {id: "request", title: "每次请求", align: "number", render: row => formatPrice(row.pricing?.per_request_price)},
            ]} />}
        </div>)}
      </InsetGroup>) : <EmptyState title="暂无可使用的渠道" />}</Resource>
  </WorkspaceContent>;
}
