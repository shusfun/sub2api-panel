import {useEffect, useState} from "react";
import {X} from "lucide-react";
import {Button} from "../primitives/button";

export type ToastTone = "success" | "warn" | "error";

type ToastEntry = {
  id: number;
  message: string;
  tone: ToastTone;
};

let seq = 0;
let entries: ToastEntry[] = [];
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

export function dismissToast(id: number) {
  const next = entries.filter(item => item.id !== id);
  if (next.length !== entries.length) {
    entries = next;
    emit();
  }
}

export function toast(message: string, tone: ToastTone = "success") {
  const trimmed = message.trim();
  if (!trimmed) return 0;
  const id = ++seq;
  entries = [...entries.slice(-2), {id, message: trimmed, tone}];
  emit();
  return id;
}

export function ToastViewport() {
  const [items, setItems] = useState<ToastEntry[]>(() => entries);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    const sync = () => setItems(entries);
    listeners.add(sync);
    return () => { listeners.delete(sync); };
  }, []);

  useEffect(() => {
    if (paused || items.length === 0) return;
    const oldest = items[0];
    const timer = window.setTimeout(() => dismissToast(oldest.id), 3200);
    return () => window.clearTimeout(timer);
  }, [items, paused]);

  if (items.length === 0) return null;

  return (
    <div
      className="weir-toast-viewport"
      role="region"
      aria-label="操作反馈"
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      {items.map(item => (
        <div
          key={item.id}
          className="weir-toast"
          data-tone={item.tone}
          role={item.tone === "error" ? "alert" : "status"}
          aria-live={item.tone === "error" ? "assertive" : "polite"}
        >
          <div className="weir-toast-body">
            <i className="weir-toast-dot" aria-hidden />
            <span className="weir-toast-text">{item.message}</span>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            title="关闭提示"
            aria-label="关闭提示"
            onClick={() => dismissToast(item.id)}
          >
            <X aria-hidden />
          </Button>
        </div>
      ))}
    </div>
  );
}
