import type {ReactNode} from "react";
import {Compass, LogOut, type LucideIcon} from "lucide-react";
import {navigatePath} from "@/lib/navigation";
import {Button} from "../primitives/button";
import {IdentityBadge} from "../domain/identity-badge";
import {Menu, MenuItem, MenuSeparator} from "./menu";
import {ThemeControl} from "./appearance";

export type AccountMenuLink = {
  id: string;
  label: string;
  href: string;
  icon?: LucideIcon;
};

// 右上角“关于我自己”：缩略身份触发，菜单依次是身份、个人链接、主题、站点首页、退出。
export function AccountMenu({
  name,
  meta,
  badge,
  tone = "neutral",
  links,
  onOpenPublicHome,
  onLogout,
  logoutBusy,
}: {
  name: string;
  meta?: ReactNode;
  badge?: string;
  tone?: "neutral" | "accent";
  links: readonly AccountMenuLink[];
  onOpenPublicHome?: () => void;
  onLogout: () => void;
  logoutBusy?: boolean;
}) {
  return (
    <Menu trigger={
      <Button className="weir-account-trigger" variant="ghost" size="icon-sm" title={`账户菜单 · ${name}`} aria-label={`账户菜单 · ${name}`} busy={logoutBusy}>
        <IdentityBadge name={name} badge={badge} tone={tone} />
      </Button>
    }>
      <div className="weir-account-menu">
        <div className="weir-account-head">
          <IdentityBadge name={name} meta={meta} badge={badge} tone={tone} />
        </div>
        <MenuSeparator />
        {links.map(link => {
          const Icon = link.icon;
          return (
            <MenuItem key={link.id} onSelect={() => navigatePath(link.href)}>
              {Icon && <Icon aria-hidden />}
              {link.label}
            </MenuItem>
          );
        })}
        <MenuSeparator />
        <div className="weir-account-row">
          <span>主题</span>
          <ThemeControl />
        </div>
        <MenuSeparator />
        {onOpenPublicHome && <MenuItem onSelect={onOpenPublicHome}><Compass aria-hidden />站点首页</MenuItem>}
        <MenuItem onSelect={onLogout}><LogOut aria-hidden />退出登录</MenuItem>
      </div>
    </Menu>
  );
}
