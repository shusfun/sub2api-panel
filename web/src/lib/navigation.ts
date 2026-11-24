export type RouteParams = Record<string, string | number | undefined | null>;
export type EntityKind = "user" | "group" | "account" | "proxy" | "key" | "model" | "request";

const entityID = /^[1-9][0-9]{0,15}$/;

export function withQuery(path: string, params?: RouteParams) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params ?? {})) {
    if (value != null && String(value) !== "") query.set(key, String(value));
  }
  const search = query.toString();
  return path + (search ? "?" + search : "");
}

export function navigatePath(next: string, replace = false) {
  if (location.pathname + location.search === next && !location.hash) return;
  if (replace) window.history.replaceState(null, "", next);
  else window.history.pushState(null, "", next);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

function entityPath(base: string, id: string | number, extra?: RouteParams) {
  const raw = String(id);
  if (entityID.test(raw) && Number.isSafeInteger(Number(raw))) {
    return withQuery(`${base}/${raw}`, extra);
  }
  return withQuery(base, {...extra, inspect: id});
}

export function entityHref(kind: EntityKind, id: string | number, extra?: Record<string, string | number | undefined>): string {
  switch (kind) {
    case "user":
      return entityPath("/admin/users", id, extra);
    case "group":
      return entityPath("/admin/groups", id, extra);
    case "account": {
      const raw = String(id);
      if (entityID.test(raw) && Number.isSafeInteger(Number(raw))) {
        return withQuery(`/admin/accounts/${raw}`, extra);
      }
      return withQuery("/admin/accounts", {detail: id, ...extra});
    }
    case "proxy":
      return entityPath("/admin/proxies", id, extra);
    case "key":
      if (extra?.user_id) {
        return entityPath("/admin/users", extra.user_id, {tab: "keys"});
      }
      return entityPath("/keys", id, extra);
    case "model":
      return withQuery("/usage", {model: id, ...extra});
    case "request":
      return withQuery("/admin/ops", {tab: "requests", request_id: id, ...extra});
  }
}
