import {useState} from "react";
import {z} from "zod";
import {api} from "../api";
import {durationText} from "../format";
import {useResource} from "../requests";
import {DataTable, EllipsisText, EmptyState, InsetGroup, PlatformBadge, RefreshButton, Resource, Status, WorkspaceContent} from "../ui";
import {ChannelRules} from "./channel-rules";

const active = z.object({id: z.number(), name: z.string(), provider: z.string(), group_name: z.string(), primary_model: z.string(),
  primary_status: z.string(), primary_latency_ms: z.number().nullable(), availability_7d: z.number()});
const health = z.object({platform: z.string(), group_id: z.number().optional(), group_name: z.string(),
  health: z.object({overall: z.string(), score: z.number().nullable()})});
const monitor = z.object({mode: z.enum(["v1", "v2"]), active: z.array(active), health: z.array(health)});

export function ChannelMonitor({admin}: {admin: boolean}) {
  const [revision, revise] = useState(0);
  const data = useResource(() => api("/channel-monitor", monitor), "channel-monitor", true, revision);
  return <WorkspaceContent><Resource state={data}>{rows => <InsetGroup
    title={rows.mode === "v1" ? "主动探测" : "真实流量健康度"}
    actions={<RefreshButton size="sm" busy={data?.loading} onClick={() => revise(value => value + 1)} />}>
    {rows.mode === "v1" ? rows.active.length ?
      <DataTable label="主动探测" rows={rows.active} rowID={row => row.id} columns={[
        {id: "name", title: "渠道", render: row => <EllipsisText text={row.name} />},
        {id: "platform", title: "平台", hideWhenCompact: true, render: row => <PlatformBadge platform={row.provider} />},
        {id: "model", title: "模型", hideWhenCompact: true, render: row => row.primary_model ? <EllipsisText text={row.primary_model} mono /> : "—"},
        {id: "status", title: "状态", align: "center", render: row => <Status value={row.primary_status} />},
        {id: "latency", title: "延迟", align: "number", hideWhenNarrow: true, render: row => durationText(row.primary_latency_ms)},
        {id: "uptime", title: "7 天可用率", align: "number", render: row => row.availability_7d.toFixed(2) + "%"},
      ]} /> : <EmptyState title="暂无主动探测记录" /> : rows.health.length ?
      <DataTable label="真实流量" rows={rows.health} rowID={row => row.platform + ":" + (row.group_id ?? row.group_name)} columns={[
        {id: "group", title: "分组", render: row => <EllipsisText text={row.group_name || (row.group_id ? "分组 #" + row.group_id : "—")} />},
        {id: "platform", title: "平台", hideWhenCompact: true, render: row => <PlatformBadge platform={row.platform} />},
        {id: "state", title: "健康状态", align: "center", render: row => <Status value={row.health.overall} />},
        {id: "score", title: "健康分", align: "number", render: row => row.health.score?.toFixed(1) ?? "样本不足"},
      ]} /> : <EmptyState title="暂无真实流量样本" />}
  </InsetGroup>}</Resource>{admin && <ChannelRules />}</WorkspaceContent>;
}
