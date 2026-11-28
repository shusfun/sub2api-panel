/*
 * 面板改版原型 · 外壳与公共交互
 * 页面只写 #content（和可选的 #inspector），调用 Proto.mount() 后由这里包上侧栏、顶栏、账户菜单、告警、⌘K 和窄屏底栏。
 * 示例数据全部是示意，不连接任何后端。
 */
(() => {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => [...r.querySelectorAll(s)];
  const store = {
    get: (k, d) => localStorage.getItem("proto:" + k) ?? d,
    set: (k, v) => localStorage.setItem("proto:" + k, v),
  };
  const isMac = /Mac|iPhone|iPad/.test(navigator.platform);

  /* ---------- 图标（24 网格，线条风格与 lucide 一致） ---------- */
  const shieldPath = '<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/>';
  const P = {
    gauge: '<path d="m12 14 4-4"/><path d="M3.34 19a10 10 0 1 1 17.32 0"/>',
    shield: shieldPath + '<path d="m9 12 2 2 4-4"/>',
    shieldAlert: shieldPath + '<path d="M12 8v4M12 16h.01"/>',
    activity: '<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>',
    server: '<rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><path d="M6 6h.01M6 18h.01"/>',
    layers: '<path d="M12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>',
    tag: '<path d="M12.59 2.59A2 2 0 0 0 11.17 2H4a2 2 0 0 0-2 2v7.17a2 2 0 0 0 .59 1.42l8.7 8.7a2.43 2.43 0 0 0 3.42 0l6.58-6.58a2.43 2.43 0 0 0 0-3.42z"/><circle cx="7.5" cy="7.5" r=".5"/>',
    network: '<rect x="16" y="16" width="6" height="6" rx="1"/><rect x="2" y="16" width="6" height="6" rx="1"/><rect x="9" y="2" width="6" height="6" rx="1"/><path d="M5 16v-3a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v3M12 12V8"/>',
    route: '<circle cx="6" cy="19" r="3"/><path d="M9 19h8.5a3.5 3.5 0 0 0 0-7h-11a3.5 3.5 0 0 1 0-7H15"/><circle cx="18" cy="5" r="3"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
    repeat: '<path d="m17 2 4 4-4 4"/><path d="M3 11v-1a4 4 0 0 1 4-4h14"/><path d="m7 22-4-4 4-4"/><path d="M21 13v1a4 4 0 0 1-4 4H3"/>',
    ticket: '<path d="M2 9a3 3 0 0 1 0 6v2a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-2a3 3 0 0 1 0-6V7a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2Z"/><path d="M13 5v2M13 17v2M13 11v2"/>',
    gift: '<rect x="3" y="8" width="18" height="4" rx="1"/><path d="M12 8v13M19 12v7a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-7"/><path d="M7.5 8a2.5 2.5 0 0 1 0-5C10 3 12 8 12 8s2-5 4.5-5a2.5 2.5 0 0 1 0 5"/>',
    wallet: '<path d="M19 7V4a1 1 0 0 0-1-1H5a2 2 0 0 0 0 4h15a1 1 0 0 1 1 1v4h-3a2 2 0 0 0 0 4h3a1 1 0 0 0 1-1v-2a1 1 0 0 0-1-1"/><path d="M3 5v14a2 2 0 0 0 2 2h15a1 1 0 0 0 1-1v-4"/>',
    megaphone: '<path d="m3 11 18-5v12L3 14v-3z"/><path d="M11.6 16.8a3 3 0 1 1-5.8-1.6"/>',
    clipboard: '<rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2M12 11h4M12 16h4M8 11h.01M8 16h.01"/>',
    database: '<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/>',
    puzzle: '<path d="M15.39 4.39a1 1 0 0 0 1.68-.47 2.5 2.5 0 1 1 3.01 3.01 1 1 0 0 0-.47 1.68l1.68 1.68a2.41 2.41 0 0 1 0 3.42l-1.61 1.61a.98.98 0 0 1-.84.28c-.47-.07-.8-.48-.97-.92a2.5 2.5 0 1 0-3.21 3.21c.44.17.85.5.92.97a.98.98 0 0 1-.28.84l-1.61 1.61a2.41 2.41 0 0 1-3.41 0l-1.68-1.68a1 1 0 0 0-1.68.47 2.5 2.5 0 1 1-3.01-3.01 1 1 0 0 0 .47-1.68l-1.68-1.68a2.41 2.41 0 0 1 0-3.42L4.32 8.68a.98.98 0 0 1 .84-.28c.47.07.8.48.97.92a2.5 2.5 0 1 0 3.21-3.21c-.44-.17-.85-.5-.92-.97a.98.98 0 0 1 .28-.84l1.61-1.61a2.41 2.41 0 0 1 3.41 0z"/>',
    settings: '<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>',
    key: '<path d="m15.5 7.5 2.3 2.3a1 1 0 0 0 1.4 0l2.1-2.1a1 1 0 0 0 0-1.4L19 4"/><path d="m21 2-9.6 9.6"/><circle cx="7.5" cy="15.5" r="5.5"/>',
    images: '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.09-3.09a2 2 0 0 0-2.82 0L6 21"/>',
    boxes: '<path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5M12 22V12"/>',
    plug: '<path d="M12 22v-5M9 8V2M15 8V2M18 8v5a4 4 0 0 1-4 4h-4a4 4 0 0 1-4-4V8Z"/>',
    bell: '<path d="M10.27 21a2 2 0 0 0 3.46 0"/><path d="M3.26 15.33A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.67C19.41 13.96 18 12.5 18 8A6 6 0 0 0 6 8c0 4.5-1.41 5.96-2.74 7.33"/>',
    bellOff: '<path d="M10.27 21a2 2 0 0 0 3.46 0M17 17H4a1 1 0 0 1-.74-1.67C4.59 13.96 6 12.5 6 8a6 6 0 0 1 .26-1.74M8.67 3.13A6 6 0 0 1 18 8c0 1.5.16 2.7.44 3.7M2 2l20 20"/>',
    search: '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
    out: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="m16 17 5-5-5-5M21 12H9"/>',
    compass: '<circle cx="12" cy="12" r="10"/><path d="m16.24 7.76-1.8 5.4a2 2 0 0 1-1.27 1.27l-5.41 1.8 1.8-5.4a2 2 0 0 1 1.27-1.27z"/>',
    plus: '<path d="M5 12h14M12 5v14"/>',
    x: '<path d="M18 6 6 18M6 6l12 12"/>',
    dots: '<path d="M5 12h.01M12 12h.01M19 12h.01"/>',
    zap: '<path d="M4 14a1 1 0 0 1-.78-1.63l9.9-10.2a.5.5 0 0 1 .86.46l-1.92 6.02A1 1 0 0 0 13 10h7a1 1 0 0 1 .78 1.63l-9.9 10.2a.5.5 0 0 1-.86-.46l1.92-6.02A1 1 0 0 0 11 14z"/>',
    refresh: '<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>',
    chevDown: '<path d="m6 9 6 6 6-6"/>',
    upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12"/>',
    alert: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4M12 17h.01"/>',
    check: '<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>',
    arrow: '<path d="M5 12h14M12 5l7 7-7 7"/>',
    copy: '<rect x="8" y="8" width="14" height="14" rx="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>',
    trash: '<path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>',
    lock: '<rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>',
    ghost: '<path d="M9 10h.01M15 10h.01M12 2a8 8 0 0 0-8 8v12l3-3 2.5 2.5L12 19l2.5 2.5L17 19l3 3V10a8 8 0 0 0-8-8z"/>',
    pencil: '<path d="M21.17 6.81a1 1 0 0 0-3.99-3.99L3.84 16.17a2 2 0 0 0-.5.83l-1.32 4.35a.5.5 0 0 0 .62.62l4.35-1.32a2 2 0 0 0 .83-.5z"/>',
    external: '<path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
    eraser: '<path d="m7 21-4.3-4.3c-1-1-1-2.5 0-3.4l9.6-9.6c1-1 2.5-1 3.4 0l5.6 5.6c1 1 1 2.5 0 3.4L13 21M22 21H7M5 11l9 9"/>',
  };
  let maskSeq = 0;
  function icon(name, cls = "") {
    if (name === "theme") {
      const id = "theme-mask-" + (++maskSeq);
      return `<svg class="ic m-theme ${cls}" viewBox="0 0 24 24" aria-hidden="true" data-state="${themeState()}"><mask id="${id}"><rect width="24" height="24" fill="#fff" stroke="none"/><circle class="bite" fill="#000" stroke="none"/></mask><circle class="core" cx="12" cy="12" fill="currentColor" stroke="none" mask="url(#${id})"/><g class="rays"><path d="M12 1.5v2M12 20.5v2M4.6 4.6 6 6M18 18l1.4 1.4M1.5 12h2M20.5 12h2M4.6 19.4 6 18M18 6l1.4-1.4"/></g></svg>`;
    }
    if (name === "menu") return `<svg class="ic m-menu ${cls}" viewBox="0 0 24 24" aria-hidden="true"><path class="l1" d="M4 7h16"/><path class="l2" d="M4 12h16"/><path class="l3" d="M4 17h16"/></svg>`;
    if (name === "rail") return `<svg class="ic m-rail ${cls}" viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="3"/><path d="M9 3v18"/><path class="chev" d="m16 9-3 3 3 3"/></svg>`;
    return `<svg class="ic i-${name} ${name === "dots" ? "ic-dots" : ""} ${cls}" viewBox="0 0 24 24" aria-hidden="true">${P[name] || ""}</svg>`;
  }

  /* ---------- 主题 ---------- */
  const darkQuery = matchMedia("(prefers-color-scheme: dark)");
  const themeNames = {light: "明亮", dark: "暗色", system: "跟随系统"};
  function themeState() { return store.get("theme", "system"); }
  function applyTheme() {
    const t = themeState();
    const root = document.documentElement;
    root.classList.toggle("dark", t === "dark" || (t === "system" && darkQuery.matches));
    root.dataset.palette = store.get("palette-v2", "ember");
    $$(".m-theme").forEach(el => { el.dataset.state = t; });
    $$("[data-theme-label]").forEach(el => { el.textContent = themeNames[t]; });
    $$("[data-palette-choice]").forEach(el => el.setAttribute("aria-pressed", String(el.dataset.paletteChoice === root.dataset.palette)));
  }
  function cycleTheme() {
    const order = ["light", "dark", "system"];
    store.set("theme", order[(order.indexOf(themeState()) + 1) % order.length]);
    applyTheme();
  }
  darkQuery.addEventListener("change", applyTheme);
  applyTheme();

  /* ---------- 导航数据 ---------- */
  const soon = name => `「${name}」的稿子在下一批出`;
  const ADMIN_RAIL = [
    {g: "观测", items: [
      {id: "ops", label: "实时监控", icon: "gauge", href: "ops.html", kw: "告警 吞吐 报错 并发"},
      {id: "patrol", label: "渠道健康", icon: "shield", kw: "巡检 渠道监控 监控模板"},
      {id: "usage", label: "用量分析", icon: "activity", kw: "账单 扣费 清理任务"},
    ]},
    {g: "号池", items: [
      {id: "accounts", label: "上游账号", icon: "server", href: "accounts.html", count: 3, kw: "测活 配额 OAuth 隐私 影子"},
      {id: "groups", label: "调度分组", icon: "layers", kw: "倍率 兜底 复合路由"},
      {id: "channels", label: "渠道定价", icon: "tag", kw: "渠道 定价 调价 模型价格"},
      {id: "proxies", label: "代理节点", icon: "network", kw: "出口 测速"},
      {id: "rules", label: "上游规则", icon: "route", kw: "错误透传 TLS 指纹"},
    ]},
    {g: "用户", items: [
      {id: "users", label: "用户", icon: "users", kw: "余额 并发"},
      {id: "subs", label: "订阅", icon: "repeat", kw: "订阅 续期 分配 套餐"},
      {id: "codes", label: "卡密与优惠", icon: "ticket", kw: "兑换码 优惠码 发码"},
      {id: "aff", label: "推广返利", icon: "gift", kw: "推广 邀请 返利 提取"},
      {id: "orders", label: "支付订单", icon: "wallet", kw: "退款 订单"},
    ]},
    {g: "系统", items: [
      {id: "ann", label: "站点公告", icon: "megaphone", kw: "公告"},
      {id: "risk", label: "风控", icon: "shieldAlert", kw: "风控 提示词审计 解封"},
      {id: "audit", label: "操作审计", icon: "clipboard", kw: "审计日志"},
      {id: "backups", label: "备份与恢复", icon: "database", kw: "备份 对象存储"},
      {id: "plugins", label: "插件生态", icon: "puzzle", kw: "插件 市场"},
      {id: "site", label: "站点设置", icon: "settings", kw: "注册 登录 支付 邮件 调度 功能开关"},
    ]},
  ];
  const USER_RAIL = [
    {g: "我的", items: [
      {id: "keys", label: "我的密钥", icon: "key", kw: "API Key 兑换"},
      {id: "usage", label: "用量分析", icon: "activity", kw: "扣费 明细"},
      {id: "patrol", label: "渠道健康", icon: "shield", kw: "渠道状态"},
      {id: "batch", label: "批量生图", icon: "images", kw: "生图"},
      {id: "plaza", label: "模型广场", icon: "boxes", kw: "模型 价格"},
      {id: "purchase", label: "购买与订单", icon: "wallet", kw: "充值 订阅"},
    ]},
  ];
  const ACCOUNT_LINKS = {
    admin: [
      {id: "keys", label: "我的密钥", icon: "key"},
      {id: "batch", label: "批量生图", icon: "images"},
      {id: "purchase", label: "购买与订单", icon: "wallet"},
      {id: "plaza", label: "模型广场", icon: "boxes"},
      {id: "my-aff", label: "邀请返利", icon: "gift"},
      {id: "mcp", label: "MCP 授权", icon: "plug"},
      {id: "my-ann", label: "站点公告", icon: "megaphone", tail: "2 未读"},
    ],
    user: [
      {id: "my-aff", label: "邀请返利", icon: "gift"},
      {id: "mcp", label: "MCP 授权", icon: "plug"},
      {id: "my-ann", label: "站点公告", icon: "megaphone", tail: "2 未读"},
    ],
  };
  const IDENTITY = {
    admin: {badge: "AD", name: "管理员", mail: "admin@example.test"},
    user: {badge: "US", name: "普通用户", mail: "user@example.test"},
  };

  const linkAttrs = item => item.href ? `href="${item.href}"` : `href="#" data-toast="${soon(item.label)}"`;

  /* ---------- 外壳 ---------- */
  let ctx = {page: "", title: "", group: "", alerts: 0, role: "admin"};

  function railHTML() {
    const groups = ctx.role === "admin" ? ADMIN_RAIL : USER_RAIL;
    return `
      <div class="rail-brand"><span class="brand-mark">S</span><span class="rail-brand-name">Sub2API</span></div>
      <div class="rail-groups">
        ${groups.map(group => `
          <div class="rail-group">
            <div class="rail-group-title">${group.g}</div>
            <nav aria-label="${group.g}">
              ${group.items.map(item => `
                <a ${linkAttrs(item)} ${item.id === ctx.page ? 'aria-current="page"' : ""} title="${item.label}">
                  ${icon(item.icon)}<span class="rail-label">${item.label}</span>
                  ${item.count ? `<span class="rail-count" aria-label="${item.count} 个异常">${item.count}</span>` : ""}
                </a>`).join("")}
            </nav>
          </div>`).join("")}
      </div>`;
  }

  function accountPopHTML() {
    const who = IDENTITY[ctx.role];
    return `
      <div class="pop-head"><span class="avatar lg">${who.badge}</span><div><b>${who.name}</b><span class="mono">${who.mail}</span></div></div>
      ${ACCOUNT_LINKS[ctx.role].map(item => `<button class="pop-item" role="menuitem" data-toast="${soon(item.label)}">${icon(item.icon)}${item.label}${item.tail ? `<span class="tail">${item.tail}</span>` : ""}</button>`).join("")}
      <div class="pop-sep"></div>
      <button class="pop-item" role="menuitem" data-toast="${soon("账户设置")}">${icon("settings")}账户设置</button>
      <button class="pop-item" role="menuitem" data-action="theme" data-keep>${icon("theme")}主题<span class="tail" data-theme-label>${themeNames[themeState()]}</span></button>
      <button class="pop-item" role="menuitem" data-href="index.html">${icon("compass")}站点首页</button>
      <div class="pop-sep"></div>
      <button class="pop-item bad" role="menuitem" data-toast="原型里不会真的退出">${icon("out")}退出登录</button>`;
  }

  function alertsPopHTML() {
    const firing = ctx.alerts > 0 ? `
      <div class="pop-title">触发中</div>
      <button class="pop-item alert-item" role="menuitem" data-href="ops.html?state=attention">
        <span class="t-warn">${icon("alert")}</span>
        <span><b>上游 429 限流</b><small>OpenAI 分组 01 · 5 分钟内 7 次 · 4 分钟前</small></span>
      </button>` : `
      <div class="pop-head"><span class="t-ok">${icon("check")}</span><div><b>没有触发中的告警</b><span>规则 12 条，全部在监控中</span></div></div>`;
    return `${firing}
      <div class="pop-sep"></div>
      <div class="pop-title">最近恢复</div>
      <div class="pop-item alert-item" role="menuitem" tabindex="0"><span class="t-ok">${icon("check")}</span><span>代理 hk-02 延迟恢复<small>持续 11 分钟 · 2 小时前</small></span></div>
      <div class="pop-sep"></div>
      <button class="pop-item" role="menuitem" data-href="ops.html">${icon("arrow")}告警与策略</button>`;
  }

  function dockHTML() {
    const items = ctx.role === "admin"
      ? [["ops", "监控", "gauge", "ops.html"], ["accounts", "账号", "server", "accounts.html"], ["users", "用户", "users"]]
      : [["keys", "密钥", "key"], ["usage", "用量", "activity"], ["patrol", "渠道", "shield"]];
    return items.map(([id, label, ic, href]) => `<a ${href ? `href="${href}"` : `href="#" data-toast="${soon(label)}"`} ${id === ctx.page ? 'aria-current="page"' : ""}>${icon(ic)}${label}</a>`).join("")
      + `<button data-palette-open aria-label="搜索或跳转">${icon("search")}搜索</button>`
      + `<button data-pop="pop-account" aria-haspopup="menu" aria-expanded="false" aria-label="账户菜单"><span class="avatar">${IDENTITY[ctx.role].badge}</span>我</button>`;
  }

  function paletteItems() {
    const rail = (ctx.role === "admin" ? ADMIN_RAIL : USER_RAIL).flatMap(g => g.items.map(i => ({...i, section: "跳转", meta: g.g})));
    const mine = ACCOUNT_LINKS[ctx.role].map(i => ({...i, section: "跳转", meta: "账户菜单"}));
    const entities = ctx.role === "admin" ? [
      {label: "OpenAI 账号 03", icon: "server", section: "上游账号", meta: "OAuth · 限流中", href: "accounts.html?inspect=3"},
      {label: "Anthropic 账号 07", icon: "server", section: "上游账号", meta: "Setup Token · 正常", href: "accounts.html?inspect=7"},
      {label: "OpenAI 分组 01", icon: "layers", section: "调度分组", meta: "倍率 1.0 · 12 个账号"},
      {label: "sample04@example.test", icon: "users", section: "用户", meta: "余额 $12.40"},
    ] : [];
    const actions = [
      ...(ctx.role === "admin" ? [{label: "添加上游账号", icon: "plus", section: "操作", href: "accounts.html?add=1"}, {label: "生成兑换码", icon: "ticket", section: "操作"}] : [{label: "创建 API Key", icon: "plus", section: "操作"}]),
      {label: "切换主题", icon: "theme", section: "操作", action: "theme", meta: themeNames[themeState()]},
    ];
    return [...rail, ...mine, ...entities, ...actions];
  }

  function mount(options) {
    ctx = {...ctx, ...options, role: options.role ?? store.get("role", "admin")};
    const content = $("#content");
    const inspector = $("#inspector");
    const app = document.createElement("div");
    app.className = "app";
    app.id = "app";
    if (store.get("rail", "open") === "mini") app.dataset.collapsed = "";
    app.innerHTML = `
      <aside class="rail" id="rail" aria-label="主导航"></aside>
      <div class="scrim" id="sheet-scrim"></div>
      <section class="stage">
        <header class="topbar">
          <button class="icon-btn rail-toggle" data-action="rail" aria-label="折叠侧栏" title="折叠侧栏（[）">${icon("rail")}</button>
          <button class="icon-btn mobile-toggle" data-action="sheet" aria-label="打开导航" aria-expanded="false" aria-controls="rail">${icon("menu")}</button>
          <div class="crumb"><span>${ctx.group}</span><span class="crumb-sep">/</span><b>${ctx.title}</b></div>
          <div class="topbar-actions">
            <button class="search-trigger" data-palette-open aria-label="搜索或跳转">${icon("search")}<span class="search-label">搜索或跳转…</span><kbd>${isMac ? "⌘K" : "Ctrl K"}</kbd></button>
            ${ctx.role === "admin" ? `<button class="icon-btn" id="bell" data-pop="pop-alerts" aria-haspopup="menu" aria-expanded="false"></button>` : ""}
            <button class="icon-btn avatar-btn" data-pop="pop-account" aria-haspopup="menu" aria-expanded="false" aria-label="账户菜单" title="账户菜单"><span class="avatar">${IDENTITY[ctx.role].badge}</span></button>
          </div>
        </header>
        <div class="stage-body"><div class="scroll" id="scroll"></div></div>
      </section>`;
    document.body.prepend(app);
    $("#rail").innerHTML = railHTML();
    $("#scroll").append(content);
    if (inspector) $(".stage-body").append(inspector);

    document.body.insertAdjacentHTML("beforeend", `
      <div class="pop" id="pop-account" role="menu" aria-label="账户菜单"></div>
      <div class="pop" id="pop-alerts" role="menu" aria-label="告警"></div>
      <nav class="dock" aria-label="快捷导航">${dockHTML()}</nav>
      <div class="scrim" id="palette-scrim"></div>
      <div class="palette" id="palette" role="dialog" aria-modal="true" aria-label="搜索或跳转">
        <label class="palette-input">${icon("search")}<input id="palette-input" placeholder="跳转到页面、账号、用户，或执行操作" autocomplete="off" role="combobox" aria-controls="palette-list" aria-expanded="true"></label>
        <div class="palette-list" id="palette-list" role="listbox"></div>
        <div class="palette-foot"><span><kbd>↑</kbd><kbd>↓</kbd>选择</span><span><kbd>Enter</kbd>打开</span><span><kbd>Esc</kbd>关闭</span></div>
      </div>
      <div class="toast" id="toast" role="status" aria-live="polite"></div>
      <div class="proto-bar" id="proto-bar">
        <button class="btn sm ghost proto-toggle" data-action="proto-min" title="收起或展开原型控制条">原型</button>
        <span class="seg" role="group" aria-label="暗色色板">
          <button data-palette-choice="ember">琥珀点色</button><button data-palette-choice="warm">暖石墨</button><button data-palette-choice="current">现有暗色</button>
        </span>
        <button class="btn sm" data-action="theme">${icon("theme")}<span data-theme-label>${themeNames[themeState()]}</span></button>
        ${options.controls ?? ""}
        <a class="btn sm ghost" href="index.html">画板索引</a>
      </div>`);
    $("#pop-account").innerHTML = accountPopHTML();
    setAlerts(ctx.alerts);
    applyTheme();
    if (store.get("proto-bar", "open") === "min") $("#proto-bar").dataset.min = "";
    document.title = `${ctx.title} · Sub2API 原型`;
  }

  function setAlerts(n) {
    ctx.alerts = n;
    const bell = $("#bell");
    if (!bell) return;
    bell.setAttribute("aria-label", n ? `告警，${n} 条触发中` : "告警");
    bell.title = n ? `${n} 条告警触发中` : "告警";
    bell.innerHTML = icon("bell") + (n ? `<span class="bell-count">${n}</span>` : "");
    $("#pop-alerts").innerHTML = alertsPopHTML();
  }

  /* ---------- 浮层 ---------- */
  let open = null;
  function place(pop, trigger) {
    const r = trigger.getBoundingClientRect();
    const w = pop.offsetWidth;
    const h = pop.offsetHeight;
    const left = Math.max(7, Math.min(r.right - w, innerWidth - w - 7));
    let top = r.bottom + 5;
    if (top + h > innerHeight - 7) top = Math.max(7, r.top - 5 - h);
    pop.style.left = left + "px";
    pop.style.top = top + "px";
  }
  function showPop(pop, trigger) {
    closePop();
    pop.dispatchEvent(new CustomEvent("proto:open", {detail: trigger}));
    place(pop, trigger);
    pop.dataset.open = "";
    trigger.setAttribute("aria-expanded", "true");
    open = {pop, trigger};
    (pop.querySelector(".pop-item") || pop).focus?.({preventScroll: true});
  }
  function closePop(restore = false) {
    if (!open) return;
    delete open.pop.dataset.open;
    open.trigger.setAttribute("aria-expanded", "false");
    if (restore) open.trigger.focus();
    open = null;
  }

  /* ---------- ⌘K ---------- */
  let paletteIndex = 0;
  let paletteRows = [];
  function renderPalette() {
    const q = $("#palette-input").value.trim().toLowerCase();
    paletteRows = paletteItems().filter(i => !q || `${i.label} ${i.kw ?? ""} ${i.meta ?? ""}`.toLowerCase().includes(q));
    paletteIndex = Math.min(paletteIndex, Math.max(0, paletteRows.length - 1));
    let last = "";
    $("#palette-list").innerHTML = paletteRows.length ? paletteRows.map((item, i) => {
      const head = item.section !== last ? `<div class="pop-title">${item.section}</div>` : "";
      last = item.section;
      return `${head}<div class="pop-item" role="option" data-index="${i}" ${i === paletteIndex ? "data-active aria-selected=\"true\"" : ""}>${icon(item.icon)}${item.label}${item.meta ? `<span class="tail">${item.meta}</span>` : ""}</div>`;
    }).join("") : `<div class="pop-head"><span class="muted">${icon("search")}</span><div><b>没有匹配的页面或对象</b><span>换个关键词，比如“订阅”“透传”“返利”</span></div></div>`;
    $("#palette-list [data-active]")?.scrollIntoView({block: "nearest"});
  }
  function openPalette() {
    closePop();
    $("#palette-input").value = "";
    paletteIndex = 0;
    renderPalette();
    $("#palette").dataset.open = "";
    $("#palette-scrim").dataset.open = "";
    setTimeout(() => $("#palette-input").focus(), 0);
  }
  function closePalette() {
    delete $("#palette").dataset.open;
    delete $("#palette-scrim").dataset.open;
  }
  function runPaletteItem(item) {
    if (!item) return;
    if (item.action === "theme") { cycleTheme(); renderPalette(); return; }
    closePalette();
    if (item.href) location.href = item.href;
    else toast(soon(item.label));
  }

  /* ---------- 杂项 ---------- */
  let toastTimer = 0;
  function toast(message) {
    const el = $("#toast");
    el.textContent = message;
    el.dataset.open = "";
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => delete el.dataset.open, 1900);
  }
  function setSheet(on) {
    const app = $("#app");
    if (on) { app.dataset.sheet = ""; $("#sheet-scrim").dataset.open = ""; }
    else { delete app.dataset.sheet; delete $("#sheet-scrim").dataset.open; }
    $(".mobile-toggle").setAttribute("aria-expanded", String(on));
  }
  function toggleRail() {
    const app = $("#app");
    if (!app) return;
    const mini = !app.hasAttribute("data-collapsed");
    app.toggleAttribute("data-collapsed", mini);
    store.set("rail", mini ? "mini" : "open");
    const btn = $(".rail-toggle");
    btn.setAttribute("aria-label", mini ? "展开侧栏" : "折叠侧栏");
    btn.title = (mini ? "展开侧栏" : "折叠侧栏") + "（[）";
  }
  function act(el) {
    if (el.dataset.href) { location.href = el.dataset.href; return; }
    if (el.dataset.toast) { toast(el.dataset.toast); return; }
    if (el.dataset.paletteChoice) { store.set("palette-v2", el.dataset.paletteChoice); applyTheme(); return; }
    switch (el.dataset.action) {
      case "theme": cycleTheme(); break;
      case "rail": toggleRail(); break;
      case "sheet": setSheet(!$("#app").hasAttribute("data-sheet")); break;
      case "proto-min": {
        const bar = $("#proto-bar");
        bar.toggleAttribute("data-min");
        store.set("proto-bar", bar.hasAttribute("data-min") ? "min" : "open");
        break;
      }
      case "role": store.set("role", el.dataset.role); location.reload(); break;
    }
  }

  document.addEventListener("click", event => {
    const target = event.target;
    const trigger = target.closest("[data-pop]");
    if (trigger) {
      event.preventDefault();
      const pop = document.getElementById(trigger.dataset.pop);
      if (open && open.pop === pop && open.trigger === trigger) closePop();
      else showPop(pop, trigger);
      return;
    }
    if (target.closest("[data-palette-open]")) { openPalette(); return; }
    if (target.closest("#palette-scrim")) { closePalette(); return; }
    if (target.closest("#sheet-scrim")) { setSheet(false); return; }
    const option = target.closest("#palette-list [data-index]");
    if (option) { runPaletteItem(paletteRows[Number(option.dataset.index)]); return; }
    const actionable = target.closest("[data-href],[data-toast],[data-action],[data-palette-choice]");
    if (actionable) {
      if (actionable.tagName === "A") event.preventDefault();
      act(actionable);
    }
    if (open && (!open.pop.contains(target) || (actionable && !actionable.hasAttribute("data-keep")))) closePop();
  });

  document.addEventListener("keydown", event => {
    const typing = /INPUT|TEXTAREA|SELECT/.test(document.activeElement?.tagName ?? "");
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
      if (!$("#palette")) return;
      event.preventDefault();
      $("#palette").hasAttribute("data-open") ? closePalette() : openPalette();
      return;
    }
    if ($("#palette")?.hasAttribute("data-open")) {
      if (event.key === "Escape") closePalette();
      else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const n = paletteRows.length || 1;
        paletteIndex = (paletteIndex + (event.key === "ArrowDown" ? 1 : n - 1)) % n;
        renderPalette();
      } else if (event.key === "Enter") { event.preventDefault(); runPaletteItem(paletteRows[paletteIndex]); }
      return;
    }
    if (open) {
      const items = $$(".pop-item", open.pop);
      const at = items.indexOf(document.activeElement);
      if (event.key === "Escape") { event.preventDefault(); closePop(true); }
      else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const step = event.key === "ArrowDown" ? 1 : items.length - 1;
        items[(at + step) % items.length]?.focus();
      }
      return;
    }
    if (event.key === "Escape" && $("#app")?.hasAttribute("data-sheet")) { setSheet(false); return; }
    if (!typing && event.key === "[") toggleRail();
  });
  document.addEventListener("input", event => {
    if (event.target.id === "palette-input") { paletteIndex = 0; renderPalette(); }
  });
  addEventListener("resize", () => closePop());

  /* ---------- 示意图表（纯 SVG，无依赖） ---------- */
  function linePath(values, w, h, pad, fromZero) {
    const max = Math.max(...values);
    const min = fromZero ? 0 : Math.min(...values);
    const span = max - min || 1;
    return values.map((v, i) => `${i ? "L" : "M"}${(i * w / (values.length - 1)).toFixed(1)} ${(h - pad - (v - min) / span * (h - pad * 2)).toFixed(1)}`).join(" ");
  }
  function spark(values, tone = "") {
    const d = linePath(values, 77, 23, 3, false);
    return `<svg class="spark ${tone}" viewBox="0 0 77 23" preserveAspectRatio="none" aria-hidden="true"><path class="area" d="${d} L77 23 L0 23 Z"/><path d="${d}"/></svg>`;
  }
  function gridLines(w, h) {
    return `<g class="grid">${[0.25, 0.5, 0.75].map(f => `<line x1="0" x2="${w}" y1="${(h * f).toFixed(1)}" y2="${(h * f).toFixed(1)}"/>`).join("")}</g>`;
  }
  function lineChart(values, label) {
    const w = 601, h = 161;
    const d = linePath(values, w, h, 9, true);
    return `<svg class="chart" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" role="img" aria-label="${label}">${gridLines(w, h)}<path class="area" d="${d} L${w} ${h} L0 ${h} Z"/><path class="line" d="${d}"/></svg>`;
  }
  function barChart(bars, label) {
    const w = 601, h = 161;
    // 纵轴至少按 5 次/分钟画，零星一两次报错不会顶满整张图
    const max = Math.max(5, ...bars.map(b => b.v));
    const step = w / bars.length;
    const bw = Math.max(1, step - 3);
    const rects = bars.map((b, i) => {
      const bh = Math.max(b.v ? 3 : 1, b.v / max * (h - 9));
      return `<rect class="bar ${b.tone ?? ""}" x="${(i * step).toFixed(1)}" y="${(h - bh).toFixed(1)}" width="${bw.toFixed(1)}" height="${bh.toFixed(1)}" rx="1"/>`;
    }).join("");
    return `<svg class="chart" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" role="img" aria-label="${label}">${gridLines(w, h)}${rects}</svg>`;
  }
  /* 页面里写 <span data-ic="名字"></span> 即可放图标 */
  function paintIcons(root = document) {
    $$("[data-ic]", root).forEach(el => { el.insertAdjacentHTML("afterbegin", icon(el.dataset.ic)); el.removeAttribute("data-ic"); });
  }

  window.Proto = {mount, icon, toast, setAlerts, applyTheme, closePop, store, spark, lineChart, barChart, paintIcons};
})();
