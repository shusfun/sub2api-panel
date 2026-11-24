import {useEffect, useMemo, useRef, useState} from "react";
import {Dialog} from "@base-ui/react/dialog";
import {Activity, Gauge, KeyRound, Layers3, Network, Puzzle, Search, Server, ShieldCheck, Ticket, UsersRound, X} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {entityHref, navigatePath} from "@/lib/navigation";
import {useDebouncedValue, useResource} from "@/lib/requests";
import {page, usageUser} from "@/schemas";

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
    const go = (path: string) => {
      navigatePath(path);
      onClose();
    };
    const base: PaletteItem[] = admin
      ? [
          {id: "nav-ops", section: "工作台", label: "实时监控", meta: "QPS · 并发 · 错误追踪 · 告警规则", icon: Gauge, run: () => go("/admin/ops")},
          {id: "nav-patrol", section: "工作台", label: "渠道健康", meta: "巡检看板 · 官方监控 · 规则编排", icon: ShieldCheck, run: () => go("/channels/status")},
          {id: "nav-usage", section: "工作台", label: "用量分析", meta: "全站与多维 Token / 费用趋势", icon: Activity, run: () => go("/usage")},
          {id: "nav-accounts", section: "工作台", label: "上游账号", meta: "水位 · 调度 · 多引擎测活", icon: Server, run: () => go("/admin/accounts")},
          {id: "nav-groups", section: "工作台", label: "调度分组", meta: "分组 · 复合路由 · 模型调价", icon: Layers3, run: () => go("/admin/groups")},
          {id: "nav-proxies", section: "工作台", label: "代理节点", meta: "测速 · 出口质量 · 导入导出", icon: Network, run: () => go("/admin/proxies")},
          {id: "nav-users", section: "工作台", label: "用户", meta: "调余额 · 订阅 · 私有分组", icon: UsersRound, run: () => go("/admin/users")},
          {id: "nav-codes", section: "工作台", label: "卡密与优惠", meta: "兑换码 · 优惠码 · 子管理员发码", icon: Ticket, run: () => go("/admin/redeem")},
          {id: "nav-plugins", section: "工作台", label: "插件生态", meta: "LM 指纹 · 外部市场与沙箱页", icon: Puzzle, run: () => go("/admin/plugins")},
          {id: "nav-keys", section: "工作台", label: "我的密钥", meta: "个人余额 · 兑换 · API Key 导入客户端", icon: KeyRound, run: () => go("/keys")},
        ]
      : [
          {id: "nav-keys", section: "工作台", label: "我的密钥", meta: "可用余额 · 兑换充值 · API Key", icon: KeyRound, run: () => go("/keys")},
          {id: "nav-usage", section: "工作台", label: "用量分析", meta: "实时扣费趋势 · 模型占比 · 逐笔明细", icon: Activity, run: () => go("/usage")},
          {id: "nav-patrol", section: "工作台", label: "渠道健康", meta: "实时分组不降智率与历史巡检记录", icon: ShieldCheck, run: () => go("/channels/status")},
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
        run: () => go(entityHref("group", g.id)),
      });
    }
    for (const a of (accounts?.value?.items ?? []).slice(0, 5)) {
      entityItems.push({
        id: `acc-${a.id}`,
        section: "上游账号",
        label: a.name,
        meta: `${a.platform} · #${a.id}`,
        icon: Server,
        run: () => go(entityHref("account", a.id)),
      });
    }
    for (const u of (users?.value?.items ?? []).slice(0, 5)) {
      entityItems.push({
        id: `user-${u.id}`,
        section: "用户",
        label: u.email,
        meta: `#${u.id}`,
        icon: UsersRound,
        run: () => go(entityHref("user", u.id)),
      });
    }
    for (const p of (proxies?.value?.items ?? []).slice(0, 4)) {
      entityItems.push({
        id: `proxy-${p.id}`,
        section: "代理节点",
        label: p.name,
        meta: `${p.protocol}://${p.host}:${p.port}`,
        icon: Network,
        run: () => go(entityHref("proxy", p.id)),
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
