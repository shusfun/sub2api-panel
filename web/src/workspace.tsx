import {useEffect, useState} from "react";
import {Activity, ArrowUpRight, KeyRound, LayoutDashboard, LogOut, Server, Ticket, UsersRound} from "lucide-react";
import {api, empty, type Config, type SessionInfo} from "./api";
import {Brand, ThemeControl} from "./appearance";
import {useAction} from "./requests";
import {Button, ErrorLine} from "./ui";
import {MenuToggle} from "./ui/menu-toggle";
import {Accounts} from "./pages/accounts";
import {Codes} from "./pages/codes";
import {Keys} from "./pages/keys";
import {Overview} from "./pages/overview";
import {Users} from "./pages/users";
import {UsageDashboard} from "./pages/usage";

const links = [
  {path: "overview", label: "账户总览", icon: LayoutDashboard, admin: false},
  {path: "keys", label: "API Key", icon: KeyRound, admin: false},
  {path: "usage", label: "用量记录", icon: Activity, admin: false},
  {path: "codes", label: "邀请码与兑换码", icon: Ticket, admin: false},
  {path: "users", label: "用户管理", icon: UsersRound, admin: true},
  {path: "accounts", label: "上游账号", icon: Server, admin: true},
] as const;

export function Workspace({config, session, logout}: {config: Config; session: SessionInfo; logout: () => void}) {
  const [path, setPath] = useState(location.hash.slice(1));
  const [mobile, openMobile] = useState(false);
  const action = useAction();
  const available = links.filter(link => session.admin || !link.admin);
  const [route, query] = path.split("?", 2);
  const active = available.find(link => link.path === route) ?? available[0];
  useEffect(() => {
    const update = () => { setPath(location.hash.slice(1)); openMobile(false); };
    window.addEventListener("hashchange", update);
    return () => window.removeEventListener("hashchange", update);
  }, []);
  useEffect(() => { document.title = active.label + " · Sub2API"; }, [active.label]);
  return <div className="workspace">
    <aside className="rail" data-open={mobile} id="panel-nav">
      <Brand />
      <nav aria-label="主导航">{available.map(({path, label, icon: Icon}) =>
        <a key={path} href={"#" + path} aria-current={active.path === path ? "page" : undefined} onClick={() => openMobile(false)}>
          <Icon aria-hidden /><span>{label}</span>
        </a>)}</nav>
      <a className="official-link" href={config.official + (session.admin ? "/admin" : "/dashboard")}>官方面板 <ArrowUpRight aria-hidden /></a>
    </aside>
    <div className="workspace-main">
      <header className="topbar">
        <Button className="mobile-toggle" variant="ghost" size="icon" title={mobile ? "关闭菜单" : "打开菜单"} aria-label={mobile ? "关闭菜单" : "打开菜单"}
          aria-controls="panel-nav" aria-expanded={mobile} onClick={() => openMobile(value => !value)}><MenuToggle open={mobile} /></Button>
        <h1 className="topbar-title">{active.label}</h1>
        <div className="topbar-actions"><ThemeControl />
          <Button variant="ghost" size="icon" busy={action.busy} title="退出登录" aria-label="退出登录"
            onClick={() => void action.run(async () => { await api("/session/logout", empty, "POST"); logout(); })}><LogOut /></Button>
        </div>
      </header>
      <main className="main-content" aria-label={active.label}>
        <ErrorLine value={action.error} />
        {active.path === "overview" && <Overview config={config} />}
        {active.path === "keys" && <Keys />}
        {active.path === "usage" && <UsageDashboard keyID={new URLSearchParams(query).get("key") ?? ""} />}
        {active.path === "codes" && <Codes config={config} admin={session.admin} />}
        {active.path === "users" && session.admin && <Users />}
        {active.path === "accounts" && session.admin && <Accounts />}
      </main>
    </div>
  </div>;
}
