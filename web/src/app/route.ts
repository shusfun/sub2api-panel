import {useCallback, useEffect, useMemo, useState} from "react";
import {entityHref, navigatePath, withQuery, type EntityKind, type RouteParams} from "@/lib/navigation";

export {entityHref, navigatePath, type EntityKind};

export type WorkspaceID =
  | "ops"
  | "patrol"
  | "usage"
  | "accounts"
  | "groups"
  | "proxies"
  | "users"
  | "codes"
  | "plugins"
  | "keys"
  | "settings"
  | "announcements"
  | "manage-announcements"
  | "affiliate"
  | "model-plaza"
  | "audit-logs"
  | "mcp"
  | "purchase"
  | "batch-image"
  | "risk-control"
  | "backups"
  | "payment-orders";

export type PublicPageID =
  | "home"
  | "login"
  | "register"
  | "email-verify"
  | "forgot-password"
  | "reset-password"
  | "oauth-callback"
  | "public-patrol"
  | "key-usage"
  | "model-plaza";

const adminPages: readonly WorkspaceID[] = [
  "ops", "patrol", "usage", "accounts", "groups", "proxies", "users", "codes",
  "plugins", "keys", "settings", "announcements", "manage-announcements", "affiliate",
  "model-plaza", "audit-logs", "mcp", "purchase", "batch-image", "risk-control",
  "backups", "payment-orders",
];

const userPages: readonly WorkspaceID[] = [
  "keys", "usage", "patrol", "settings", "announcements", "affiliate",
  "model-plaza", "mcp", "purchase", "batch-image",
];

const publicPaths: Record<PublicPageID, string> = {
  home: "/",
  login: "/login",
  register: "/register",
  "email-verify": "/email-verify",
  "forgot-password": "/forgot-password",
  "reset-password": "/reset-password",
  "oauth-callback": "/auth/callback",
  "public-patrol": "/patrol",
  "key-usage": "/key-usage",
  "model-plaza": "/model-plaza",
};

const workspacePaths: Record<WorkspaceID, string> = {
  ops: "/admin/ops",
  patrol: "/channels/status",
  usage: "/usage",
  accounts: "/admin/accounts",
  groups: "/admin/groups",
  proxies: "/admin/proxies",
  users: "/admin/users",
  codes: "/admin/redeem",
  plugins: "/admin/plugins",
  keys: "/keys",
  settings: "/settings",
  announcements: "/announcements",
  "manage-announcements": "/admin/announcements",
  affiliate: "/affiliate",
  "model-plaza": "/model-plaza",
  "audit-logs": "/admin/audit-logs",
  mcp: "/mcp",
  purchase: "/purchase",
  "batch-image": "/batch-image",
  "risk-control": "/admin/risk-control",
  backups: "/admin/backups",
  "payment-orders": "/admin/orders",
};

const routesWithEntity = new Set<WorkspaceID>(["accounts", "users", "groups", "proxies", "keys"]);
const entityID = /^[1-9][0-9]{0,15}$/;

export function customPageSlug(path = location.pathname) {
  return /^\/custom\/([a-zA-Z0-9][a-zA-Z0-9_-]{0,79})$/.exec(path)?.[1];
}

export function resolvePublicRoute(): {
  page: PublicPageID | "workspace" | "missing" | "mcp-consent" | "payment-result" | "ui-gallery";
  params: URLSearchParams;
} {
  if (location.pathname === "/_ui") return {page: "ui-gallery", params: new URLSearchParams(location.search)};
  if (location.pathname === "/oauth/mcp/authorize") return {page: "mcp-consent", params: new URLSearchParams(location.search)};
  if (location.pathname === "/payment/result") return {page: "payment-result", params: new URLSearchParams(location.search)};
  const page = (Object.keys(publicPaths) as PublicPageID[]).find(id => publicPaths[id] === location.pathname);
  return {page: page ?? ((matchWorkspace(location.pathname) || customPageSlug()) ? "workspace" : "missing"), params: new URLSearchParams(location.search)};
}

export function navigatePublic(page: PublicPageID, params?: RouteParams) {
  navigatePath(withQuery(publicPaths[page], params));
}

function matchWorkspace(path: string): {page: WorkspaceID; id?: string} | undefined {
  for (const page of adminPages) {
    const base = workspacePaths[page];
    if (path === base) return {page};
    if (!routesWithEntity.has(page) || !path.startsWith(base + "/")) continue;
    const id = path.slice(base.length + 1);
    if (entityID.test(id) && Number.isSafeInteger(Number(id))) return {page, id};
  }
  return undefined;
}

function readRoute(url: string, admin: boolean) {
  const {pathname, search} = new URL(url, location.origin);
  const match = matchWorkspace(pathname);
  const allowed = admin ? adminPages : userPages;
  const page = match?.page ?? allowed[0];
  const params = new URLSearchParams(search);
  if (match?.id) params.set(page === "accounts" ? "detail" : "inspect", match.id);
  return {page, params, queryString: params.toString(), forbidden: !allowed.includes(page)};
}

export function buildHref(page: WorkspaceID, params?: RouteParams): string {
  const query = {...params};
  const entityParam = page === "accounts" ? "detail" : "inspect";
  const id = String(query[entityParam] ?? "");
  let path = workspacePaths[page];
  if (routesWithEntity.has(page) && entityID.test(id) && Number.isSafeInteger(Number(id))) {
    path += "/" + id;
    delete query[entityParam];
  }
  return withQuery(path, query);
}

export function navigateTo(page: WorkspaceID, params?: RouteParams, replace = false) {
  navigatePath(buildHref(page, params), replace);
}

export function interceptWorkspaceLinks(event: MouseEvent) {
  if (customPageSlug() || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
  const anchor = event.target instanceof Element ? event.target.closest("a[href]") : null;
  if (!(anchor instanceof HTMLAnchorElement) || anchor.target || anchor.hasAttribute("download")) return;
  const url = new URL(anchor.href);
  if (url.origin !== location.origin || url.hash || url.pathname === "/batch-image" || !matchWorkspace(url.pathname)) return;
  event.preventDefault();
  navigatePath(url.pathname + url.search);
}

export function useRoute(admin: boolean) {
  const [url, setURL] = useState(() => location.pathname + location.search);
  useEffect(() => {
    const sync = () => setURL(location.pathname + location.search);
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, []);
  const state = useMemo(() => readRoute(url, admin), [url, admin]);

  const setParams = useCallback((updates: Record<string, string | number | undefined | null>, replace = true) => {
    const current = readRoute(location.pathname + location.search, admin);
    const merged: Record<string, string> = {};
    current.params.forEach((v, k) => { merged[k] = v; });
    for (const [k, v] of Object.entries(updates)) {
      if (v === undefined || v === null || String(v) === "") {
        delete merged[k];
      } else {
        merged[k] = String(v);
      }
    }
    navigateTo(current.page, merged, replace);
  }, [admin]);

  return {...state, setParams};
}
