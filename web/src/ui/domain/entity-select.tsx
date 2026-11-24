import {useMemo, useState} from "react";
import {Layers3, Network, Server, UserRound} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {named, page, usageUser} from "@/schemas";
import {useResource} from "@/lib/requests";
import {SearchSelect, type SelectOption} from "../patterns/combobox";
import {platformLabels} from "./entity";

export const PLATFORM_OPTIONS: readonly SelectOption[] = Object.entries(platformLabels).map(([value, label]) => ({value, label}));

const proxyOptionSchema = z.object({
  id: z.number().int().positive(),
  name: z.string(),
  protocol: z.string(),
  host: z.string(),
  port: z.number(),
  status: z.string(),
  latency_ms: z.number().nullable(),
});

const accountOptionSchema = z.object({
  id: z.number().int().positive(),
  name: z.string(),
  platform: z.string(),
  status: z.string(),
});

export function PlatformSelect({
  value,
  onChange,
  includeComposite = false,
  placeholder = "全部平台",
  clearable = true,
}: {
  value: string;
  onChange: (value: string) => void;
  includeComposite?: boolean;
  placeholder?: string;
  clearable?: boolean;
}) {
  const options = useMemo(
    () => includeComposite ? PLATFORM_OPTIONS : PLATFORM_OPTIONS.filter(p => p.value !== "composite"),
    [includeComposite],
  );
  return (
    <SearchSelect
      label="平台"
      value={value}
      options={options}
      placeholder={placeholder}
      searchable={false}
      clearable={clearable}
      showEmptyOption={clearable}
      icon={<Layers3 />}
      onChange={onChange}
    />
  );
}

export function UserSelect({
  value,
  onChange,
  extraOptions = [],
  placeholder = "全部用户",
  clearable = true,
}: {
  value: string;
  onChange: (value: string, label?: string) => void;
  extraOptions?: readonly SelectOption[];
  placeholder?: string;
  clearable?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [cachedLabel, setCachedLabel] = useState("");
  const path = "/admin/usage/users?search=" + encodeURIComponent(search);
  const resource = useResource(() => api(path, page(usageUser)), path);
  const options = useMemo(() => {
    const map = new Map<string, SelectOption>();
    for (const item of extraOptions) map.set(item.value, item);
    for (const row of resource?.value?.items ?? []) {
      map.set(String(row.id), {value: String(row.id), label: row.email, meta: `#${row.id}`});
    }
    if (value && cachedLabel && !map.has(value)) {
      map.set(value, {value, label: cachedLabel});
    }
    return [...map.values()];
  }, [cachedLabel, extraOptions, resource?.value?.items, value]);

  return (
    <SearchSelect
      label="用户"
      value={value}
      options={options}
      placeholder={placeholder}
      searchPlaceholder="搜索用户邮箱…"
      searching={resource?.loading}
      clearable={clearable}
      showEmptyOption={clearable}
      icon={<UserRound />}
      onSearchChange={setSearch}
      onChange={next => {
        const found = options.find(o => o.value === next);
        if (found) setCachedLabel(found.label);
        onChange(next, found?.label);
      }}
    />
  );
}

export function ProxySelect({
  value,
  onChange,
  excludeID,
  placeholder = "直连（不使用代理）",
  revision = 0,
}: {
  value: string;
  onChange: (value: string) => void;
  excludeID?: number;
  placeholder?: string;
  revision?: number;
}) {
  const resource = useResource(() => api("/admin/proxies?page=1", page(proxyOptionSchema)), "proxy-options", true, revision);
  const options = useMemo<SelectOption[]>(() => {
    return (resource?.value?.items ?? [])
      .filter(p => p.id !== excludeID)
      .map(p => ({
        value: String(p.id),
        label: p.name,
        meta: `${p.protocol}://${p.host}:${p.port}${p.latency_ms !== null ? ` · ${Math.round(p.latency_ms)}ms` : ""}`,
      }));
  }, [excludeID, resource?.value?.items]);

  return (
    <SearchSelect
      label="代理节点"
      value={value}
      options={options}
      placeholder={placeholder}
      searchPlaceholder="搜索代理名称或地址…"
      searching={resource?.loading}
      icon={<Network />}
      onChange={onChange}
    />
  );
}

export function AccountSelect({
  value,
  onChange,
  placeholder = "全部上游账号",
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  const [search, setSearch] = useState("");
  const path = "/admin/accounts?page=1" + (search ? "&search=" + encodeURIComponent(search) : "");
  const resource = useResource(() => api(path, page(accountOptionSchema)), path);
  const options = useMemo<SelectOption[]>(
    () => (resource?.value?.items ?? []).map(a => ({value: String(a.id), label: a.name, meta: `${a.platform} · #${a.id}`})),
    [resource?.value?.items],
  );
  return (
    <SearchSelect
      label="上游账号"
      value={value}
      options={options}
      placeholder={placeholder}
      searchPlaceholder="搜索上游账号…"
      searching={resource?.loading}
      icon={<Server />}
      onSearchChange={setSearch}
      onChange={onChange}
    />
  );
}

export function KeySelect({
  path,
  value,
  onChange,
  placeholder = "全部 API Key",
}: {
  path: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  const [pageNumber, setPageNumber] = useState(1);
  const [previous, setPrevious] = useState<SelectOption[]>([]);
  const url = path + (path.includes("?") ? "&" : "?") + "page=" + pageNumber;
  const resource = useResource(() => api(url, page(named)), url);
  const rows = resource?.stale ? [] : resource?.value?.items ?? [];
  const current = rows.map(row => ({value: String(row.id), label: row.name, meta: `#${row.id}`}));
  const options = [...new Map([...previous, ...current].map(row => [row.value, row])).values()];

  return (
    <SearchSelect
      label="API Key"
      value={value}
      options={options}
      placeholder={placeholder}
      searching={resource?.loading}
      emptyText={resource?.error ? "无法加载 Key" : "暂无可选 Key"}
      hasMore={resource?.value?.more}
      loadingMore={resource?.loading}
      onLoadMore={() => {
        if (resource?.loading) return;
        setPrevious(options);
        setPageNumber(pageNumber + 1);
      }}
      onChange={onChange}
    />
  );
}
