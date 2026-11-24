import {useEffect, useMemo, useRef, useState} from "react";
import {Dialog} from "@base-ui/react/dialog";
import {Activity, Gauge, KeyRound, Layers3, Network, Puzzle, Search, Server, ShieldCheck, UsersRound, X} from "lucide-react";
import {z} from "zod";
import {api, page, usageUser} from "../api";
import {useDebouncedValue, useResource} from "../requests";
import {navigateTo, type WorkspaceID} from "../route";

type PaletteItem = {
  id: string;
  section: string;
  label: string;
  meta?: string;
  icon: typeof Activity;
  run: () => void;
};

const groupSearchSchema = z.array(z.object({id: z.number(), name: z.string(), platform: z.string()}));
const accountSearchSchema = page(z.object({id: z.number(), name: z.string(), platform: z.string()}));
const proxySearchSchema = page(z.object({id: z.number(), name: z.string(), protocol: z.string(), host: z.string(), port: z.number()}));

export function CommandPalette({
  open,
  onClose,
  admin,
}: {
  open: boolean;
  onClose: () => void;
  admin: boolean;
}) {
  const [query, setQuery] = useState("");
  const [index, setIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounced = useDebouncedValue(query.trim(), 180);

  useEffect(() => {
    if (open) {
      setQuery("");
      setIndex(0);
      const frame = requestAnimationFrame(() => inputRef.current?.focus());
      return () => cancelAnimationFrame(frame);
    }
  }, [open]);

  const groups = useResource(() => api("/admin/groups/all", groupSearchSchema), "palette-groups", open && admin);
  const accounts = useResource(
    () => api("/admin/accounts?page=1" + (debounced ? "&search=" + encodeURIComponent(debounced) : ""), accountSearchSchema),
    "palette-accounts:" + debounced,
    open && admin,
  );
  const users = useResource(
    () => api("/admin/usage/users?search=" + encodeURIComponent(debounced), page(usageUser)),
    "palette-users:" + debounced,
    open && admin,
  );
  const proxies = useResource(
    () => api("/admin/proxies?page=1" + (debounced ? "&search=" + encodeURIComponent(debounced) : ""), proxySearchSchema),
    "palette-proxies:" + debounced,
    open && admin,
  );

  const items = useMemo<PaletteItem[]>(() => {
    const go = (ws: WorkspaceID, params?: Record<string, string | number>) => {
      navigateTo(ws, params);
      onClose();
    };
    const base: PaletteItem[] = admin
      ? [
          {id: "nav-ops", section: "工作台", label: "实时监控与排障", meta: "QPS · 并发 · 错误追踪 · 告警规则", icon: Gauge, run: () => go("ops")},
          {id: "nav-patrol", section: "工作台", label: "智能渠道巡检", meta: "活跃用户门控 · 不降智率 · 60 格健康柱", icon: ShieldCheck, run: () => go("patrol")},
          {id: "nav-usage", section: "工作台", label: "用量与账单分析", meta: "全站与多维 Token / 费用趋势", icon: Activity, run: () => go("usage")},
          {id: "nav-accounts", section: "工作台", label: "上游账号", meta: "水位 · 调度 · 多引擎测活", icon: Server, run: () => go("accounts")},
          {id: "nav-groups", section: "工作台", label: "调度分组", meta: "分组 · 复合路由 · 模型调价", icon: Layers3, run: () => go("groups")},
          {id: "nav-proxies", section: "工作台", label: "代理", meta: "测速 · 质量 · 导入导出", icon: Network, run: () => go("proxies")},
          {id: "nav-users", section: "工作台", label: "用户", meta: "调余额 · 订阅 · 私有分组", icon: UsersRound, run: () => go("users")},
          {id: "nav-codes", section: "工作台", label: "兑换码", meta: "批量生成 · 直接充值 · 发码记录", icon: KeyRound, run: () => go("codes")},
          {id: "nav-plugins", section: "工作台", label: "插件生态", meta: "LM 指纹 · 少点套路 · 外部市场与沙箱页", icon: Puzzle, run: () => go("plugins")},
          {id: "nav-keys", section: "工作台", label: "我的密钥与账户", meta: "个人余额 · 兑换 · API Key 导入客户端", icon: KeyRound, run: () => go("keys")},
        ]
      : [
          {id: "nav-keys", section: "工作台", label: "我的密钥与账户", meta: "可用余额 · 兑换充值 · API Key", icon: KeyRound, run: () => go("keys")},
          {id: "nav-usage", section: "工作台", label: "用量分析", meta: "实时扣费趋势 · 模型占比 · 逐笔明细", icon: Activity, run: () => go("usage")},
          {id: "nav-patrol", section: "工作台", label: "渠道状态", meta: "实时分组不降智率与历史巡检记录", icon: ShieldCheck, run: () => go("patrol")},
        ];

    const kw = query.trim().toLowerCase();
    const filteredNav = kw
      ? base.filter(item => item.label.toLowerCase().includes(kw) || (item.meta?.toLowerCase().includes(kw) ?? false))
      : base;

    if (!admin) return filteredNav;

    const entityItems: PaletteItem[] = [];
    for (const g of (groups?.value ?? []).filter(row => !kw || row.name.toLowerCase().includes(kw) || row.platform.toLowerCase().includes(kw)).slice(0, 5)) {
      entityItems.push({
        id: `group-${g.id}`,
        section: "调度分组",
        label: g.name,
        meta: `${g.platform} · #${g.id}`,
        icon: Layers3,
        run: () => go("groups", {inspect: g.id}),
      });
    }
    for (const a of (accounts?.value?.items ?? []).slice(0, 5)) {
      entityItems.push({
        id: `acc-${a.id}`,
        section: "上游账号",
        label: a.name,
        meta: `${a.platform} · #${a.id}`,
        icon: Server,
        run: () => go("accounts", {inspect: a.id}),
      });
    }
    for (const u of (users?.value?.items ?? []).slice(0, 5)) {
      entityItems.push({
        id: `user-${u.id}`,
        section: "用户",
        label: u.email,
        meta: `#${u.id}`,
        icon: UsersRound,
        run: () => go("users", {inspect: u.id}),
      });
    }
    for (const p of (proxies?.value?.items ?? []).slice(0, 4)) {
      entityItems.push({
        id: `proxy-${p.id}`,
        section: "代理节点",
        label: p.name,
        meta: `${p.protocol}://${p.host}:${p.port}`,
        icon: Network,
        run: () => go("proxies", {inspect: p.id}),
      });
    }

    return [...filteredNav, ...entityItems];
  }, [accounts?.value?.items, admin, groups?.value, onClose, proxies?.value?.items, query, users?.value?.items]);

  useEffect(() => {
    setIndex(0);
  }, [items.length]);

  return (
    <Dialog.Root open={open} onOpenChange={next => { if (!next) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Backdrop className="overlay-backdrop" />
        <Dialog.Popup className="swift-palette" aria-label="快速跳转">
          <div className="swift-palette-search">
            <Search aria-hidden />
            <input
              ref={inputRef}
              type="text"
              value={query}
              placeholder={admin ? "跳转工作台，或搜索用户、分组、账号、代理…" : "搜索工作台…"}
              aria-label="搜索命令或实体"
              onChange={e => setQuery(e.target.value)}
              onKeyDown={e => {
                if (e.key === "ArrowDown") {
                  e.preventDefault();
                  setIndex(prev => (items.length ? (prev + 1) % items.length : 0));
                } else if (e.key === "ArrowUp") {
                  e.preventDefault();
                  setIndex(prev => (items.length ? (prev - 1 + items.length) % items.length : 0));
                } else if (e.key === "Enter" && items[index]) {
                  e.preventDefault();
                  items[index].run();
                }
              }}
            />
            {query && (
              <button type="button" className="obs-popover-search-clear" onClick={() => setQuery("")} aria-label="清空">
                <X aria-hidden />
              </button>
            )}
          </div>
          <div className="swift-palette-list" role="listbox" aria-label="跳转结果">
            {items.map((item, idx) => {
              const Icon = item.icon;
              const prevSection = idx > 0 ? items[idx - 1].section : "";
              return (
                <div key={item.id}>
                  {item.section !== prevSection && <div className="swift-palette-section">{item.section}</div>}
                  <button
                    type="button"
                    role="option"
                    aria-selected={idx === index}
                    data-active={idx === index || undefined}
                    className="swift-palette-item"
                    onMouseEnter={() => setIndex(idx)}
                    onClick={item.run}
                  >
                    <Icon aria-hidden />
                    <span className="swift-palette-label">{item.label}</span>
                    {item.meta && <span className="swift-palette-meta">{item.meta}</span>}
                  </button>
                </div>
              );
            })}
            {!items.length && <div className="obs-popover-empty">没有匹配的工作台或对象</div>}
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
