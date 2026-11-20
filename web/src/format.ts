export const money = (value: number | null) => value === null ? "未知" : new Intl.NumberFormat("zh-CN", {style: "currency", currency: "USD"}).format(value);
export const date = (value: string | number) => new Date(value).toLocaleString("zh-CN", {dateStyle: "short", timeStyle: "short"});
export const statusLabel = (value: string) => ({
  active: "正常", inactive: "已停用", disabled: "已停用", error: "异常", expired: "已过期",
  pending: "等待核对", unknown: "结果待核对", issued: "已发放", failed: "失败",
})[value] ?? "未知";

export function cents(value: string): number {
  if (!/^(0|[1-9][0-9]{0,6})(\.[0-9]{1,2})?$/.test(value)) throw new Error("金额最多保留两位小数。");
  const [integer, fraction = ""] = value.split(".");
  return Number(integer) * 100 + Number(fraction.padEnd(2, "0"));
}

export const percent = (value: number | null) => value === null ? "未知" : value.toFixed(0) + "%";
export const count = new Intl.NumberFormat("zh-CN");
export const preciseMoney = new Intl.NumberFormat("zh-CN", {style: "currency", currency: "USD", maximumFractionDigits: 6});
export const rate = new Intl.NumberFormat("zh-CN", {style: "percent", maximumFractionDigits: 1});
