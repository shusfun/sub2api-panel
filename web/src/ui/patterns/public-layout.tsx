import type {ReactNode} from "react";
import {navigatePath} from "@/lib/navigation";
import {Brand, ThemeControl} from "./appearance";
import {InlineStack} from "../primitives/layout";

export function PublicLayout({
  siteName,
  siteLogo,
  subtitle,
  actions,
  contactInfo,
  docUrl,
  flush = false,
  children,
}: {
  siteName: string;
  siteLogo?: string;
  subtitle?: string;
  actions?: ReactNode;
  contactInfo?: string;
  docUrl?: string;
  flush?: boolean;
  children: ReactNode;
}) {
  return (
    <div className="public-shell">
      <header className="public-topbar">
        <div className="public-topbar-inner">
          <a
            href="/"
            className="public-brand-row"
            onClick={e => {
              e.preventDefault();
              navigatePath("/");
            }}
          >
            <Brand name={siteName} logo={siteLogo || undefined} />
            {subtitle && <span className="public-brand-sub">{subtitle}</span>}
          </a>
          <div className="public-nav-actions">
            {actions}
            <ThemeControl />
          </div>
        </div>
      </header>
      {flush ? children : <main className="public-main">{children}</main>}
      {!flush && (
        <footer className="public-footer">
          <div className="public-footer-inner">
            <span>© {new Date().getFullYear()} {siteName}</span>
            <InlineStack wrap>
              {contactInfo && <span>联系支持：{contactInfo}</span>}
              {docUrl && (
                <a href={docUrl} target="_blank" rel="noopener noreferrer">接口文档</a>
              )}
            </InlineStack>
          </div>
        </footer>
      )}
    </div>
  );
}

export function StageCard({
  title,
  subtitle,
  brand,
  topActions,
  footer,
  size = "sm",
  children,
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  brand?: ReactNode;
  topActions?: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg";
  children: ReactNode;
}) {
  return (
    <div className="stage-shell">
      <header className="stage-top-actions">
        <InlineStack>
          {topActions}
          <ThemeControl />
        </InlineStack>
      </header>
      <div className="stage-stack" data-size={size}>
        <main className="stage-panel">
          <div className="stage-hero">
            {brand}
            <h1>{title}</h1>
            {subtitle && <p>{subtitle}</p>}
          </div>
          {children}
        </main>
        {footer}
      </div>
    </div>
  );
}
