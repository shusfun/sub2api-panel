import type {z} from "zod";
import {subscriptions, usage} from "../api";
import {count, date, money} from "../format";
import {Status, type Column} from "../ui";
import {RemoteTable} from "./remote-table";

export const subscriptionColumns: Column<z.infer<typeof subscriptions>>[] = [
  {id: "group", title: "分组", render: row => row.group},
  {id: "status", title: "状态", render: row => <Status value={row.status} />},
  {id: "expiry", title: "到期时间", render: row => date(row.expires_at)},
];

const usageColumns: Column<z.infer<typeof usage>>[] = [
  {id: "model", title: "模型", render: row => row.model},
  {id: "input", title: "输入 Token", align: "number", render: row => count.format(row.input_tokens)},
  {id: "output", title: "输出 Token", align: "number", render: row => count.format(row.output_tokens)},
  {id: "cost", title: "费用", align: "number", render: row => money(row.actual_cost)},
  {id: "time", title: "时间", render: row => date(row.created_at)},
];

export function UserUsageTable({user}: {user: number}) {
  return <RemoteTable path={`/admin/users/${user}/usage`} schema={usage} columns={usageColumns} rowID={row => row.id} label="用户用量" />;
}
