import {useState, type CSSProperties, type ReactNode} from "react";
import {Popover} from "@base-ui/react/popover";
import {Check, X} from "lucide-react";
import {Button} from "../primitives/button";
import {EmptyState, ErrorLine, Skeleton} from "./feedback";

export type NotificationItem = {
  id: string;
  title: ReactNode;
  meta?: ReactNode;
  tone?: "neutral" | "warn" | "bad";
  read: boolean;
};

// 铃铛线条形变：同一条钟身，无未读时铃舌居中收短、角标线收成一点；有未读时铃舌伸长、角标线伸出。
const bellPaths = {
  body: "M 6 17 L 6 11 C 6 7.5 8.5 5 12 5 C 15.5 5 18 7.5 18 11 L 18 17 L 20 17",
  rim: "M 4 17 L 6 17",
  clapper: {idle: "M 11 20 L 13 20", unread: "M 10 20 L 14 20"},
  badge: {idle: "M 19 4 L 19 4", unread: "M 19 3 L 19 6"},
};

function BellMorphIcon({unread}: {unread: boolean}) {
  const clapper = unread ? bellPaths.clapper.unread : bellPaths.clapper.idle;
  const badge = unread ? bellPaths.badge.unread : bellPaths.badge.idle;
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"
      aria-hidden data-unread={unread || undefined} className="weir-morph-icon weir-bell-icon">
      <g className="weir-bell-swing">
        <path d={bellPaths.body} />
        <path d={bellPaths.rim} />
        <path d={clapper} style={{d: `path('${clapper}')`} as CSSProperties} />
      </g>
      <path className="weir-bell-badge" d={badge} style={{d: `path('${badge}')`} as CSSProperties} />
    </svg>
  );
}

export function NotificationBell({
  unread,
  items,
  loading,
  error,
  onOpenChange,
  onSelect,
  onRead,
  onDismiss,
  onReadAll,
}: {
  unread: number;
  items: readonly NotificationItem[];
  loading?: boolean;
  error?: ReactNode;
  onOpenChange?: (open: boolean) => void;
  onSelect: (id: string) => void;
  onRead: (id: string) => void;
  onDismiss: (id: string) => void;
  onReadAll: () => void;
}) {
  const [open, setOpen] = useState(false);
  const label = unread > 0 ? `告警，${unread} 条未读` : "告警";
  return (
    <Popover.Root open={open} onOpenChange={next => { setOpen(next); onOpenChange?.(next); }}>
      <Popover.Trigger render={<Button variant="ghost" size="icon-sm" title={label} aria-label={label} className="weir-bell-trigger" />}>
        <BellMorphIcon unread={unread > 0} />
        {unread > 0 && <span className="weir-bell-count weir-digits" aria-hidden>{unread > 99 ? "99+" : unread}</span>}
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Positioner className="weir-bell-positioner" align="end" sideOffset={5}>
          <Popover.Popup className="weir-bell-popup" aria-label="告警列表">
            <header className="weir-bell-head">
              <strong>告警</strong>
              <Button type="button" size="xs" variant="ghost" disabled={unread === 0} onClick={onReadAll}>全部已读</Button>
            </header>
            {error ? <ErrorLine value={typeof error === "string" ? error : "告警列表加载失败"} /> : null}
            <div className="weir-bell-list" role="list">
              {loading && items.length === 0 ? <Skeleton className="weir-bell-skeleton" /> : null}
              {!loading && !error && items.length === 0 ? <EmptyState title="没有告警" /> : null}
              {items.map(item => (
                <div key={item.id} role="listitem" className="weir-bell-item" data-read={item.read || undefined} data-tone={item.tone ?? "neutral"}>
                  <button type="button" className="weir-bell-main" onClick={() => { onSelect(item.id); setOpen(false); onOpenChange?.(false); }}>
                    <span className="weir-bell-dot" aria-hidden />
                    <span className="weir-bell-text">
                      <span className="weir-bell-title">{item.title}</span>
                      {item.meta !== undefined && <span className="weir-bell-meta">{item.meta}</span>}
                    </span>
                  </button>
                  <span className="weir-bell-ops">
                    {!item.read && (
                      <Button type="button" size="icon-sm" variant="ghost" title="标为已读" aria-label="标为已读" onClick={() => onRead(item.id)}><Check aria-hidden /></Button>
                    )}
                    <Button type="button" size="icon-sm" variant="ghost" title="忽略" aria-label="忽略" onClick={() => onDismiss(item.id)}><X aria-hidden /></Button>
                  </span>
                </div>
              ))}
            </div>
          </Popover.Popup>
        </Popover.Positioner>
      </Popover.Portal>
    </Popover.Root>
  );
}
