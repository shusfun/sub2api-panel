import {z} from "zod";
import {lazy, Suspense, useEffect, useState} from "react";
import {Activity, Bell, Boxes, ClipboardList, Compass, DatabaseBackup, Gauge, Gift, Images, KeyRound, Layers3, LogOut, Megaphone, Network, Plug, Puzzle, Search, Server, Settings, ShieldCheck, Tag, Ticket, UsersRound, Wallet} from "lucide-react";
import {api, customMenuItem, empty, type Config, type SessionInfo} from "./api";
import {Brand, ThemeControl} from "./appearance";
import {useAction, useResource} from "./requests";
import {buildHref, customPageSlug, navigatePath, useRoute, type WorkspaceID} from "./route";
import {Button, CommandPalette, EllipsisText, EmptyState, ErrorLine, Feedback, Field, Form, InsetGroup, InsetRow, Input, WorkspaceContent, WorkspaceErrorBoundary} from "./ui";
import {MenuToggle} from "./ui/menu-toggle";

const customMenuList = z.array(customMenuItem);
const adminCompliance = z.object({required: z.boolean(), version: z.string(), document_url_zh: z.string(), ack_phrase_zh: z.string()});

function AdminConfirmation({status, onAccepted}: {status: z.infer<typeof adminCompliance>; onAccepted: () => void}) {
  const action = useAction();
  const [phrase, setPhrase] = useState("");
  return <WorkspaceContent>
    <InsetGroup title="管理员首次确认">
      <InsetRow label="Sub2API 部署与运营合规承诺" hint={status.version}>
        <Button variant="outline" render={<a href={status.document_url_zh} target="_blank" rel="noopener noreferrer" />}>阅读官方承诺</Button>
      </InsetRow>
      <InsetRow label="确认语句" hint={status.ack_phrase_zh} vertical>
        <Form busy={action.busy} onSubmit={() => void action.run(async () => {
          const result = await api("/admin/compliance/accept", adminCompliance, "POST", {phrase: phrase.trim()});
          if (result.required) throw new Error("官方仍要求完成确认");
          onAccepted();
        })}>
          <Field label="输入确认语句"><Input value={phrase} onChange={event => setPhrase(event.target.value)} maxLength={300} required /></Field>
          <ErrorLine value={action.error} />
          <Button type="submit" busy={action.busy} disabled={phrase.trim() !== status.ack_phrase_zh}>确认并进入管理工作区</Button>
        </Form>
      </InsetRow>
    </InsetGroup>
  </WorkspaceContent>;
}

const OpsWorkspace = lazy(() => import("./pages/ops").then(m => ({default: m.OpsWorkspace})));
const PatrolWorkspace = lazy(() => import("./pages/patrol").then(m => ({default: m.PatrolWorkspace})));
const UsageDashboard = lazy(() => import("./usage").then(m => ({default: m.UsageDashboard})));
const AccountsWorkspace = lazy(() => import("./pages/accounts").then(m => ({default: m.AccountsWorkspace})));
const AccountDetailWorkspace = lazy(() => import("./pages/accounts").then(m => ({default: m.AccountDetailWorkspace})));
const GroupsWorkspace = lazy(() => import("./pages/groups").then(m => ({default: m.GroupsWorkspace})));
const ProxiesWorkspace = lazy(() => import("./pages/proxies").then(m => ({default: m.ProxiesWorkspace})));
const UsersWorkspace = lazy(() => import("./pages/users").then(m => ({default: m.UsersWorkspace})));
const CodesWorkspace = lazy(() => import("./pages/codes").then(m => ({default: m.CodesWorkspace})));
const PluginsWorkspace = lazy(() => import("./pages/plugins").then(m => ({default: m.PluginsWorkspace})));
const KeysWorkspace = lazy(() => import("./pages/keys").then(m => ({default: m.KeysWorkspace})));
const SettingsWorkspace = lazy(() => import("./pages/settings").then(m => ({default: m.SettingsWorkspace})));
const AvailableChannels = lazy(() => import("./pages/available-channels").then(m => ({default: m.AvailableChannels})));
const Announcements = lazy(() => import("./pages/announcements").then(m => ({default: m.Announcements})));
const AnnouncementNotice = lazy(() => import("./pages/announcements").then(m => ({default: m.AnnouncementNotice})));
const ManageAnnouncements = lazy(() => import("./pages/announcements").then(m => ({default: m.ManageAnnouncements})));
const PromoCodes = lazy(() => import("./pages/promo-codes").then(m => ({default: m.PromoCodes})));
const Affiliate = lazy(() => import("./pages/affiliate").then(m => ({default: m.Affiliate})));
const ModelPlaza = lazy(() => import("./pages/model-plaza").then(m => ({default: m.ModelPlaza})));
const AuditLogs = lazy(() => import("./pages/audit-logs").then(m => ({default: m.AuditLogs})));
const MCPGrants = lazy(() => import("./pages/mcp").then(m => ({default: m.MCPGrants})));
const Purchase = lazy(() => import("./pages/payment").then(m => ({default: m.Purchase})));
const PaymentOrders = lazy(() => import("./pages/payment").then(m => ({default: m.PaymentOrders})));
const BatchImages = lazy(() => import("./pages/batch-images").then(m => ({default: m.BatchImages})));
const RiskControl = lazy(() => import("./pages/risk-control").then(m => ({default: m.RiskControl})));
const ChannelMonitor = lazy(() => import("./pages/channel-monitor").then(m => ({default: m.ChannelMonitor})));

const BackupWorkspace = lazy(() => import("./pages/backups").then(m => ({default: m.BackupWorkspace})));

type NavItem = {
  id: WorkspaceID;
  label: string;
  icon: typeof Activity;
  group: string;
};

const channelsNav: NavItem = {id: "available-channels", label: "可用渠道", icon: Network, group: "个人"};
const announcementsNav: NavItem = {id: "announcements", label: "站点公告", icon: Bell, group: "个人"};
const affiliateNav: NavItem = {id: "affiliate", label: "邀请返利", icon: Gift, group: "个人"};
const plazaNav: NavItem = {id: "model-plaza", label: "模型广场", icon: Boxes, group: "个人"};
const mcpNav: NavItem = {id: "mcp", label: "MCP 授权", icon: Plug, group: "个人"};
const purchaseNav: NavItem = {id: "purchase", label: "购买与订单", icon: Wallet, group: "个人"};
const orderNav: NavItem = {id: "payment-orders", label: "支付订单", icon: Wallet, group: "用户"};
const monitorNav: NavItem = {id: "official-monitor", label: "渠道监控", icon: Activity, group: "观测"};

const adminNav: readonly NavItem[] = [
  {id: "ops", label: "实时监控", icon: Gauge, group: "观测"},
  {id: "patrol", label: "渠道巡检", icon: ShieldCheck, group: "观测"},
  {id: "usage", label: "用量分析", icon: Activity, group: "观测"},
  {id: "accounts", label: "上游账号", icon: Server, group: "号池"},
  {id: "groups", label: "调度分组", icon: Layers3, group: "号池"},
  {id: "proxies", label: "代理", icon: Network, group: "号池"},
  {id: "users", label: "用户", icon: UsersRound, group: "用户"},
  {id: "codes", label: "兑换码", icon: Ticket, group: "用户"},
  {id: "promo-codes", label: "优惠码", icon: Tag, group: "用户"},
  {id: "manage-announcements", label: "公告管理", icon: Megaphone, group: "系统"},
  {id: "audit-logs", label: "操作审计", icon: ClipboardList, group: "系统"},
  {id: "risk-control", label: "风控", icon: ShieldCheck, group: "系统"},
  {id: "backups", label: "备份与恢复", icon: DatabaseBackup, group: "系统"},
  {id: "plugins", label: "插件生态", icon: Puzzle, group: "系统"},
  {id: "keys", label: "我的密钥", icon: KeyRound, group: "个人"},
  {id: "batch-image", label: "批量生图", icon: Images, group: "个人"},
  {id: "settings", label: "设置", icon: Settings, group: "个人"},
];

export function Workspace({
  config,
  session,
  onOpenPublicHome,
  logout,
}: {
  config: Config;
  session: SessionInfo;
  onOpenPublicHome?: () => void;
  logout: () => void;
}) {
  const route = useRoute(session.admin);
  const [mobile, openMobile] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const action = useAction();
  const [confirmationRevision, refreshConfirmation] = useState(0);
  const confirmation = useResource(() => api("/admin/compliance", adminCompliance), "admin-compliance", session.admin, confirmationRevision);
  const needsConfirmation = confirmation?.value?.required === true;

  const [menuRevision, refreshMenu] = useState(0);
  const [announcementRevision, refreshAnnouncements] = useState(0);
  useEffect(() => {
    const refresh = () => refreshMenu(value => value + 1);
    window.addEventListener("panel-site-settings-saved", refresh);
    return () => window.removeEventListener("panel-site-settings-saved", refresh);
  }, []);
  const administratorMenus = useResource(() => api("/admin/custom-menu", customMenuList), "admin-custom-menu", confirmation?.value?.required === false, menuRevision);
  const menu = [...config.custom_menu_items, ...(administratorMenus?.value ?? [])].sort((a, b) => a.sort_order - b.sort_order);
  const slug = customPageSlug();
  const custom = menu.find(item => item.page_slug === slug);
  const userNav: readonly NavItem[] = [
    {id: "keys", label: "我的密钥", icon: KeyRound, group: "个人"},
    {id: "batch-image", label: "批量生图", icon: Images, group: "个人"},
    {id: "usage", label: "用量分析", icon: Activity, group: "个人"},
    ...(config.patrol_user_visible ? [{id: "patrol" as const, label: "渠道状态", icon: ShieldCheck, group: "个人"}] : []),
    {id: "settings", label: "设置", icon: Settings, group: "个人"},
  ];

  const ordered = [...(session.admin ? adminNav : userNav), announcementsNav,
    ...(config.affiliate ? [affiliateNav] : []), ...(config.available_channels_enabled ? [channelsNav] : []),
    ...(config.model_plaza_enabled ? [plazaNav] : []), ...(config.mcp_enabled ? [mcpNav] : []),
    ...(config.channel_monitor_enabled ? [monitorNav] : [])];
  const active = [...ordered, channelsNav, affiliateNav, plazaNav, mcpNav, purchaseNav, orderNav, monitorNav].find(item => item.id === route.page) ?? ordered[0];
  const accountDetail = route.page === "accounts" ? route.params.get("detail") : null;
  const groups = [...new Set(ordered.map(item => item.group))];
  const siteName = config.site_name || "Sub2API";

  useEffect(() => {
    document.title = (custom?.label ?? active.label) + " · " + siteName;
    openMobile(false);
  }, [active.label, custom?.label, route.page, route.queryString, siteName]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen(prev => !prev);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  if (route.forbidden) return <main className="main-content" role="status">无权访问该页面 <a href={buildHref("keys")}>返回我的密钥</a></main>;

  return (
    <div className="workspace">
      <aside className="rail" data-open={mobile} id="panel-nav">
        <Brand name={siteName} logo={config.site_logo || undefined} />
        <div className="rail-groups">
          {groups.map(group => (
            <div key={group} className="rail-group">
              <span className="rail-group-title">{group}</span>
              <nav aria-label={group}>
                {ordered.filter(item => item.group === group).map(({id, label, icon: Icon}) => (
                  <a
                    key={id}
                    href={buildHref(id)}
                    aria-current={!slug && active.id === id ? "page" : undefined}
                    onClick={() => openMobile(false)}
                  >
                    <Icon className="rail-icon" aria-hidden />
                    <span>{label}</span>
                  </a>
                ))}
              </nav>
            </div>
          ))}
          {!!menu.length && <div className="rail-group">
            <span className="rail-group-title">自定义菜单</span>
            <nav aria-label="自定义菜单">
              {menu.map(item => <a key={item.id} href={"/custom/" + item.page_slug} aria-current={slug === item.page_slug ? "page" : undefined}>
                {item.icon_svg ? <img className="rail-icon" alt="" src={"data:image/svg+xml," + encodeURIComponent(item.icon_svg)} /> : <Compass className="rail-icon" aria-hidden="true" />}
                <span>{item.label}</span>
              </a>)}
            </nav>
          </div>}
        </div>
        <div className="rail-footer">
          <div className="rail-footer-meta">
            <span>{session.admin ? "管理员" : "普通用户"}</span>
            {onOpenPublicHome && (
              <a
                className="official-link"
                href="/"
                onClick={e => {
                  e.preventDefault();
                  onOpenPublicHome();
                }}
              >
                <Compass aria-hidden />站点首页
              </a>
            )}
          </div>
        </div>
      </aside>

      <div className="workspace-main">
        <header className="topbar">
          <Button
            className="mobile-toggle"
            variant="ghost"
            size="icon"
            title={mobile ? "关闭菜单" : "打开菜单"}
            aria-label={mobile ? "关闭菜单" : "打开菜单"}
            aria-controls="panel-nav"
            aria-expanded={mobile}
            onClick={() => openMobile(v => !v)}
          >
            <MenuToggle open={mobile} />
          </Button>
          <h1 className="topbar-title"><EllipsisText text={custom?.label ?? (slug ? "自定义页面" : active.label)} tabIndex={0} /></h1>
          <div className="topbar-actions">
            <Button className="workspace-search" variant="ghost" title="快速搜索跳转 (Ctrl/⌘+K)" aria-label="快速搜索跳转" onClick={() => setPaletteOpen(true)}>
              <Search aria-hidden /><span>快速跳转</span><kbd>Ctrl/⌘ K</kbd>
            </Button>
            <ThemeControl />
            <Button
              variant="ghost"
              size="icon"
              busy={action.busy}
              title="退出登录"
              aria-label="退出登录"
              onClick={() => void action.run(async () => {
                await api("/session/logout", empty, "POST");
                navigatePath("/", true);
                logout();
              })}
            >
              <LogOut />
            </Button>
          </div>
        </header>

        <main className="main-content" aria-label={custom?.label ?? active.label}>
          <ErrorLine value={action.error} />
          {confirmation?.loading ? <Feedback>正在读取管理员确认状态…</Feedback> : confirmation?.error ? <WorkspaceContent>
            <ErrorLine value={confirmation.error} />
            <Button variant="outline" onClick={() => refreshConfirmation(value => value + 1)}>重试</Button>
          </WorkspaceContent> : needsConfirmation && confirmation.value ? <AdminConfirmation status={confirmation.value}
            onAccepted={() => refreshConfirmation(value => value + 1)} /> : slug ? custom ? <section className="custom-workspace">
            {!custom.hide_open_button && <Button variant="outline" size="sm" render={<a href={custom.url} target="_blank" rel="noopener noreferrer" />}>在新窗口打开</Button>}
            <iframe title={custom.label} src={custom.url} sandbox="allow-scripts allow-forms allow-popups" referrerPolicy="no-referrer" />
          </section> : administratorMenus?.loading ? <Feedback>正在加载…</Feedback> : <EmptyState title="此页面不可用" /> : <WorkspaceErrorBoundary resetKey={`${active.id}:${accountDetail}:${route.queryString}`}><Suspense fallback={<div className="page-loading" role="status">正在加载工作台…</div>}>
            {active.id === "ops" && session.admin && <OpsWorkspace queryString={route.queryString} />}
            {active.id === "patrol" && <PatrolWorkspace admin={session.admin} />}
            {active.id === "usage" && (
              <UsageDashboard
                keyID={route.params.get("key") ?? ""}
                queryString={route.queryString}
                admin={session.admin}
              />
            )}
            {active.id === "accounts" && session.admin && (accountDetail && /^[1-9][0-9]*$/.test(accountDetail) && Number.isSafeInteger(Number(accountDetail))
              ? <AccountDetailWorkspace key={accountDetail} id={Number(accountDetail)} queryString={route.queryString} /> : <AccountsWorkspace queryString={route.queryString} />)}
            {active.id === "groups" && session.admin && <GroupsWorkspace queryString={route.queryString} />}
            {active.id === "proxies" && session.admin && <ProxiesWorkspace queryString={route.queryString} />}
            {active.id === "users" && session.admin && <UsersWorkspace queryString={route.queryString} />}
            {active.id === "codes" && session.admin && <CodesWorkspace config={config} />}
            {active.id === "plugins" && session.admin && <PluginsWorkspace stepUpEnabled={session.step_up_enabled} />}
            {active.id === "available-channels" && (config.available_channels_enabled ? <AvailableChannels /> : <EmptyState title="站点尚未开放可用渠道" />)}
            {active.id === "announcements" && <Announcements />}
            {active.id === "affiliate" && (config.affiliate ? <Affiliate /> : <EmptyState title="站点尚未开放邀请返利" />)}
            {active.id === "model-plaza" && (config.model_plaza_enabled ? <ModelPlaza /> : <EmptyState title="站点尚未开放模型广场" />)}
            {active.id === "audit-logs" && session.admin && <AuditLogs />}
            {active.id === "mcp" && (config.mcp_enabled ? <MCPGrants /> : <EmptyState title="站点尚未开放 MCP 授权" />)}
            {active.id === "backups" && <BackupWorkspace stepUpEnabled={session.step_up_enabled} />}
            {active.id === "payment-orders" && (config.purchase_enabled ? <PaymentOrders /> : <EmptyState title="站点暂未开启支付" />)}
            {active.id === "purchase" && <Purchase enabled={config.purchase_enabled} />}
            {active.id === "batch-image" && <BatchImages gateway={config.official} />}
            {active.id === "risk-control" && session.admin && <RiskControl />}
            {active.id === "official-monitor" && (config.channel_monitor_enabled ? <ChannelMonitor admin={session.admin} /> : <EmptyState title="站点尚未开启渠道监控" />)}
            {active.id === "manage-announcements" && session.admin && <ManageAnnouncements onChanged={() => refreshAnnouncements(value => value + 1)} />}
            {active.id === "promo-codes" && session.admin && <PromoCodes />}
            {active.id === "keys" && <KeysWorkspace config={config} admin={session.admin} queryString={route.queryString} />}
            {active.id === "settings" && <SettingsWorkspace admin={session.admin} section={route.params.get("section") ?? ""} onSection={section => route.setParams({section}, false)} />}
          </Suspense></WorkspaceErrorBoundary>}
        </main>
      </div>

      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} admin={session.admin} />
      {(!session.admin || confirmation?.value?.required === false) && !route.forbidden && !slug && active.id !== "announcements" &&
        <Suspense fallback={null}><AnnouncementNotice revision={announcementRevision} /></Suspense>}
    </div>
  );
}
