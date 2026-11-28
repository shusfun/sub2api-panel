import {useCallback, useEffect, useState} from "react";
import {api, describeError} from "@/lib/http";
import {empty} from "@/schemas";
import {ledgerAlertList, ledgerUnreadCount, type LedgerAlert} from "@/schemas/ledger";
import {navigateTo} from "@/app/route";
import {NotificationBell, type NotificationItem} from "@/ui";
import {date} from "@/lib/format";

const unreadRefreshMs = 61000;

function toItem(alert: LedgerAlert): NotificationItem {
  const meta = [alert.account_name, date(alert.opened_at), alert.status === "resolved" ? "已恢复" : ""].filter(Boolean).join(" · ");
  return {
    id: String(alert.id),
    title: alert.title,
    meta,
    tone: alert.severity === "bad" ? "bad" : alert.severity === "warn" ? "warn" : "neutral",
    read: alert.read,
  };
}

// 告警铃数据容器：未读数定时刷新，打开时拉列表；已读、忽略、全部已读后重新拉取。
export function LedgerAlertBell() {
  const [unread, setUnread] = useState(0);
  const [alerts, setAlerts] = useState<LedgerAlert[]>([]);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();

  const loadUnread = useCallback(async () => {
    try {
      setUnread((await api("/admin/ledger/alerts/unread-count", ledgerUnreadCount)).unread);
    } catch {
      // 未读数拉取失败时保留上一次的值，不打断页面
    }
  }, []);

  const loadList = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api("/admin/ledger/alerts", ledgerAlertList);
      setAlerts(res.items ?? []);
      setError(undefined);
    } catch (failure) {
      setError(describeError(failure));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadUnread();
    const timer = window.setInterval(() => { if (!document.hidden) void loadUnread(); }, unreadRefreshMs);
    return () => window.clearInterval(timer);
  }, [loadUnread]);

  useEffect(() => {
    if (open) void loadList();
  }, [open, loadList]);

  async function mutate(path: string) {
    try {
      await api(path, empty, "POST", {});
      setError(undefined);
    } catch (failure) {
      setError(describeError(failure));
    }
    await Promise.all([loadUnread(), loadList()]);
  }

  return (
    <NotificationBell
      unread={unread}
      items={alerts.map(toItem)}
      loading={loading}
      error={error}
      onOpenChange={setOpen}
      onSelect={id => {
        const target = alerts.find(item => String(item.id) === id);
        if (!target) return;
        if (!target.read) void mutate(`/admin/ledger/alerts/${target.id}/read`);
        if (target.account_id != null) navigateTo("accounts", {detail: target.account_id});
      }}
      onRead={id => void mutate(`/admin/ledger/alerts/${id}/read`)}
      onDismiss={id => void mutate(`/admin/ledger/alerts/${id}/dismiss`)}
      onReadAll={() => void mutate("/admin/ledger/alerts/read-all")}
    />
  );
}
