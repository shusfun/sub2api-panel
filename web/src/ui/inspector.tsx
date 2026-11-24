import {useEffect, useState, type ReactNode} from "react";
import {Dialog} from "@base-ui/react/dialog";
import {ChevronDown, ChevronUp, X} from "lucide-react";
import {Button} from "./button";
import {FormBoundary} from "./form";

function useWideDesktop() {
  const [wide, setWide] = useState(() => typeof window === "undefined" || window.innerWidth >= 1201);
  useEffect(() => {
    const query = window.matchMedia("(min-width: 1201px)");
    const sync = () => setWide(query.matches);
    sync();
    query.addEventListener("change", sync);
    return () => query.removeEventListener("change", sync);
  }, []);
  return wide;
}

export function Inspector({
  open,
  title,
  subtitle,
  badge,
  actions,
  onClose,
  onPrev,
  onNext,
  busy,
  children,
}: {
  open: boolean;
  title: ReactNode;
  subtitle?: ReactNode;
  badge?: ReactNode;
  actions?: ReactNode;
  onClose: () => void;
  onPrev?: () => void;
  onNext?: () => void;
  busy?: boolean;
  children: ReactNode;
}) {
  const wide = useWideDesktop();

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy && wide) {
        onClose();
      } else if (event.altKey && event.key === "ArrowUp" && onPrev) {
        event.preventDefault();
        onPrev();
      } else if (event.altKey && event.key === "ArrowDown" && onNext) {
        event.preventDefault();
        onNext();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, busy, wide, onClose, onPrev, onNext]);

  if (!open) return null;

  const header = (
    <header className="swift-inspector-head">
      <div className="swift-inspector-title-wrap">
        <div className="swift-inspector-title-row">
          <h2 className="swift-inspector-title">{title}</h2>
          {badge}
        </div>
        {subtitle !== undefined && <div className="swift-inspector-subtitle">{subtitle}</div>}
      </div>
      <div className="swift-inspector-controls">
        {actions}
        {(onPrev || onNext) && (
          <div className="swift-inspector-step" role="group" aria-label="切换上下条">
            <Button variant="ghost" size="icon-xs" title="上一项 (Alt+↑)" aria-label="上一项" disabled={!onPrev || busy} onClick={onPrev}>
              <ChevronUp />
            </Button>
            <Button variant="ghost" size="icon-xs" title="下一项 (Alt+↓)" aria-label="下一项" disabled={!onNext || busy} onClick={onNext}>
              <ChevronDown />
            </Button>
          </div>
        )}
        <Button variant="ghost" size="icon-xs" title="关闭检查器" aria-label="关闭检查器" disabled={busy} onClick={onClose}>
          <X />
        </Button>
      </div>
    </header>
  );

  if (wide) {
    return (
      <aside className="swift-inspector" aria-label={typeof title === "string" ? `${title} 检查器` : "详情检查器"}>
        {header}
        <div className="swift-inspector-body">
          <FormBoundary>{children}</FormBoundary>
        </div>
      </aside>
    );
  }

  return (
    <Dialog.Root open={open} onOpenChange={next => { if (!next && !busy) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Backdrop className="overlay-backdrop" />
        <Dialog.Popup className="swift-sheet" aria-label={typeof title === "string" ? title : "详情检查器"}>
          <div className="weir-sheet-handle" aria-hidden />
          {header}
          <div className="swift-inspector-body">
            <FormBoundary>{children}</FormBoundary>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
