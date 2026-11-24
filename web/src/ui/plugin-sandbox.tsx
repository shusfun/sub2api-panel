import {useEffect, useRef} from "react";

export function PluginSandboxFrame({
  pluginId,
  title,
  permissions = [],
  onReadScope,
}: {
  pluginId: string;
  title: string;
  permissions?: readonly string[];
  onReadScope?: (scope: string, params: Record<string, unknown>) => Promise<unknown>;
}) {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const dark = typeof document !== "undefined" && document.documentElement.classList.contains("dark");
  const src = `/api/plugins/frame/${encodeURIComponent(pluginId)}?theme=${dark ? "dark" : "light"}`;

  useEffect(() => {
    const handleMessage = async (ev: MessageEvent) => {
      if (ev.source !== frameRef.current?.contentWindow) return;
      const payload = ev.data as {panelPluginRead?: string; pluginId?: string; reqId?: string; params?: Record<string, unknown>} | null;
      if (!payload?.panelPluginRead || !payload.reqId || payload.pluginId !== pluginId) return;
      const reply = (body: Record<string, unknown>) => {
        frameRef.current?.contentWindow?.postMessage({panelReply: payload.reqId, ...body}, "*");
      };
      if (!permissions.includes(payload.panelPluginRead)) {
        reply({error: "插件未声明权限：" + payload.panelPluginRead});
        return;
      }
      if (!onReadScope) {
        reply({error: "当前页面不支持该作用域读取"});
        return;
      }
      try {
        const result = await onReadScope(payload.panelPluginRead, payload.params ?? {});
        reply({result});
      } catch (err) {
        reply({error: err instanceof Error ? err.message : "读取失败"});
      }
    };
    window.addEventListener("message", handleMessage);
    return () => window.removeEventListener("message", handleMessage);
  }, [onReadScope, permissions, pluginId]);

  return (
    <div className="swift-plugin-frame-wrap">
      <iframe
        ref={frameRef}
        className="swift-plugin-frame"
        title={title}
        src={src}
        sandbox="allow-scripts"
      />
    </div>
  );
}
