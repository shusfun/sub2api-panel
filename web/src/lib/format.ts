export const money = (value: number | null) => value === null ? "未知" : new Intl.NumberFormat("zh-CN", {style: "currency", currency: "USD"}).format(value);
export const date = (value: string | number) => new Date(value).toLocaleString("zh-CN", {dateStyle: "short", timeStyle: "short"});

export function dateTimeInputValue(value: string | null | undefined): string {
  if (!value) return "";
  const local = new Date(value);
  return new Date(local.getTime() - local.getTimezoneOffset() * 60000).toISOString().slice(0, 19);
}

export function dateTimeInputSeconds(value: FormDataEntryValue | null): number | undefined {
  if (typeof value !== "string" || !value) return undefined;
  const seconds = Math.trunc(Date.parse(value) / 1000);
  if (!Number.isFinite(seconds) || seconds < 0) throw new Error("请选择有效日期和时间");
  return seconds;
}

const statusDictionary: Record<string, string> = {
  active: "正常",
  enabled: "已启用",
  valid: "有效",
  ok: "正常",
  succeeded: "已成功",
  completed: "已完成",
  COMPLETED: "已完成",
  PAID: "已付款",
  allow: "放行",
  compatible: "兼容",
  inactive: "已停用",
  disabled: "已停用",
  draft: "草稿",
  archived: "已归档",
  CANCELLED: "已取消",
  cancelled: "已取消",
  output_deleted: "结果已删除",
  skipped_idle: "低峰免检",
  skip: "跳过",
  shadow: "仅记录",
  error: "异常",
  failed: "失败",
  FAILED: "失败",
  REFUND_FAILED: "退款失败",
  incompatible: "不兼容",
  degraded: "降智",
  block: "拦截",
  banned: "已封禁",
  invalidated: "已失效",
  expired: "已过期",
  EXPIRED: "已过期",
  pending: "排队中",
  PENDING: "待支付",
  RECHARGING: "正在入账",
  REFUND_REQUESTED: "已申请退款",
  REFUNDING: "退款中",
  REFUND_PENDING: "等待退款",
  PARTIALLY_REFUNDED: "部分退款",
  REFUNDED: "已退款",
  running: "进行中",
  starting: "启动中",
  queued: "已排队",
  dumping: "正在导出",
  uploading: "正在上传",
  indexing: "正在索引",
  processing_results: "整理结果",
  settling: "结算中",
  untested: "未验证",
  unknown: "结果待核对",
  issued: "已发放",
  used: "已使用",
  unused: "未使用",
  revoked: "已撤销",
  suspended: "已暂停",
  quota_exhausted: "额度耗尽",
  firing: "触发中",
  resolved: "已恢复",
  manual_resolved: "人工恢复",
  healthy: "健康",
  warning: "预警",
  critical: "严重",
  warn: "预警",
};

export const statusLabel = (value: string) => (Object.hasOwn(statusDictionary, value) ? statusDictionary[value] : "未知");

export function cents(value: string): number {
  if (!/^(0|[1-9][0-9]{0,6})(\.[0-9]{1,2})?$/.test(value)) throw new Error("金额最多保留两位小数。");
  const [integer, fraction = ""] = value.split(".");
  return Number(integer) * 100 + Number(fraction.padEnd(2, "0"));
}

export const percent = (value: number | null) => value === null ? "未知" : value.toFixed(0) + "%";
export const count = new Intl.NumberFormat("zh-CN");
export const preciseMoney = new Intl.NumberFormat("zh-CN", {style: "currency", currency: "USD", maximumFractionDigits: 6});
export const rate = new Intl.NumberFormat("zh-CN", {style: "percent", maximumFractionDigits: 1});
export const compact = new Intl.NumberFormat("zh-CN", {notation: "compact", maximumFractionDigits: 1});
export const compactEn = new Intl.NumberFormat("en-US", {notation: "compact", maximumFractionDigits: 1});

export function smartMoney(value: number): string {
  if (!Number.isFinite(value)) return "—";
  if (value === 0) return "$0.00";
  const abs = Math.abs(value);
  if (abs < 0.01) return new Intl.NumberFormat("en-US", {style: "currency", currency: "USD", minimumFractionDigits: 4, maximumFractionDigits: 6}).format(value);
  if (abs < 1) return new Intl.NumberFormat("en-US", {style: "currency", currency: "USD", minimumFractionDigits: 3, maximumFractionDigits: 4}).format(value);
  return new Intl.NumberFormat("en-US", {style: "currency", currency: "USD", minimumFractionDigits: 2, maximumFractionDigits: 2}).format(value);
}

export function currencyMoney(value: number, currency = "USD"): string {
  if (!Number.isFinite(value)) return "—";
  const code = currency.toUpperCase();
  if (code === "USD") return smartMoney(value);
  if (code === "CNY" || code === "RMB") {
    return new Intl.NumberFormat("zh-CN", {style: "currency", currency: "CNY", minimumFractionDigits: 2, maximumFractionDigits: 2}).format(value);
  }
  return `${value.toFixed(2)} ${code}`;
}

export function compactTokens(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return "未知";
  if (value === 0) return "0";
  return compact.format(value);
}

export function shortDay(isoDate: string): string {
  return isoDate.length >= 10 ? isoDate.slice(5, 10) : isoDate;
}

export function relativeUntil(iso: string | null): string {
  if (!iso) return "";
  const diff = new Date(iso).getTime() - Date.now();
  if (!Number.isFinite(diff)) return "";
  if (diff <= 0) return "已到期";
  const minutes = Math.round(diff / 60000);
  if (minutes < 60) return `${minutes} 分钟后`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时 ${minutes % 60} 分后`;
  const days = Math.floor(hours / 24);
  return `${days} 天后`;
}

export function durationText(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || !Number.isFinite(ms)) return "未知";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
}

export function uptimeText(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || !Number.isFinite(seconds) || seconds <= 0) return "未知";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  if (days > 0) return `${days} 天 ${hours} 小时`;
  const mins = Math.floor((seconds % 3600) / 60);
  return hours > 0 ? `${hours} 小时 ${mins} 分` : `${mins} 分钟`;
}
