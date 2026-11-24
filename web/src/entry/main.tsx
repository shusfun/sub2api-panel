import {lazy, Suspense, useEffect, useState} from "react";
import {createRoot} from "react-dom/client";
import {ApiError, api, setSession} from "@/lib/http";
import {useResource} from "@/lib/requests";
import {configSchema, empty, sessionSchema, setupCandidatesSchema, setupSchema, type SessionInfo} from "@/schemas";
import {interceptWorkspaceLinks, navigatePath, navigateTo, resolvePublicRoute} from "@/app/route";
import {Workspace} from "@/app/workspace";
import {Button, ErrorLine, Feedback, Field, Form, Input, InsetGroup, InsetRow, Resource, StageCard, ToastViewport, TooltipProvider, WorkspaceErrorBoundary} from "@/ui";
import {PublicHome} from "@/features/public/home";
import {Login} from "@/features/public/login";
import {PublicKeyUsage} from "@/features/public/public-key-usage";
import "../styles.css";

const PublicModelPlaza = lazy(() => import("@/features/workspace/model-plaza").then(module => ({default: module.PublicModelPlaza})));
const MCPConsent = lazy(() => import("@/features/workspace/mcp").then(module => ({default: module.MCPConsent})));
const PaymentResult = lazy(() => import("@/features/billing/payment").then(module => ({default: module.PaymentResult})));
const UIGallery = lazy(() => import("@/features/dev/ui-gallery").then(module => ({default: module.UIGallery})));

function useLocationTick() {
  const [, bump] = useState(0);
  useEffect(() => {
    const sync = () => bump(v => v + 1);
    window.addEventListener("popstate", sync);
    document.addEventListener("click", interceptWorkspaceLinks);
    return () => {
      window.removeEventListener("popstate", sync);
      document.removeEventListener("click", interceptWorkspaceLinks);
    };
  }, []);
}

function App() {
  useLocationTick();
  const [configRevision, refreshConfig] = useState(0);
  const config = useResource(() => api("/config", configSchema, "GET", undefined, false), "config", true, configRevision);
  useEffect(() => {
    const refresh = () => refreshConfig(value => value + 1);
    window.addEventListener("panel-site-settings-saved", refresh);
    return () => window.removeEventListener("panel-site-settings-saved", refresh);
  }, []);
  const logo = config?.value?.site_logo;
  useEffect(() => {
    const icon = document.querySelector<HTMLLinkElement>("link[data-site-icon]");
    if (icon) icon.href = logo || "/favicon.svg";
  }, [logo]);
  const setup = useResource(() => api("/setup", setupSchema, "GET", undefined, false), "setup");
  const current = useResource(() => api("/session", sessionSchema).catch(error => {
    if (error instanceof ApiError && error.status === 401) return null;
    throw error;
  }), "session");
  const [changed, setChanged] = useState<SessionInfo | null>();
  const [browsingPublic, setBrowsingPublic] = useState(false);
  const active = changed === undefined ? current?.value : changed;

  useEffect(() => {
    setSession(active ?? undefined);
  }, [active]);

  useEffect(() => {
    const end = () => setChanged(null);
    window.addEventListener("panel-session-ended", end);
    return () => window.removeEventListener("panel-session-ended", end);
  }, []);

  const pub = resolvePublicRoute();

  return (
    <TooltipProvider>
      <ToastViewport />
      <WorkspaceErrorBoundary resetKey={location.pathname + location.search}>
        <Resource state={config}>
          {value => {
            const allowGallery = import.meta.env.DEV || Boolean(value.dev_accounts?.length);
            if (pub.page === "ui-gallery") {
              return allowGallery ? (
                <Suspense fallback={<Feedback>正在加载组件画廊…</Feedback>}>
                  <UIGallery siteName={value.site_name} siteLogo={value.site_logo} />
                </Suspense>
              ) : (
                <Feedback>页面不存在</Feedback>
              );
            }
            if (setup?.value?.required) return <SetupWizard />;
            if (pub.page === "key-usage") return <PublicKeyUsage config={value} />;
            if (pub.page === "payment-result") {
              return (
                <Suspense fallback={<Feedback>正在读取订单…</Feedback>}>
                  <PaymentResult config={value} session={active} params={pub.params} />
                </Suspense>
              );
            }
            if (current?.error && changed === undefined) {
              return (
                <StageCard title="读不到登录状态">
                  <ErrorLine value={current.error} />
                  <Button size="lg" onClick={() => location.reload()}>重新加载</Button>
                </StageCard>
              );
            }
            if (active === undefined) return <Feedback>正在加载…</Feedback>;
            if (pub.page === "missing") return <Feedback>页面不存在</Feedback>;
            if (pub.page === "mcp-consent" && active) {
              return <Suspense fallback={<Feedback>正在加载…</Feedback>}><MCPConsent config={value} admin={active.admin} query={pub.params.toString()} /></Suspense>;
            }
            if (active === null) {
              if (pub.page === "model-plaza" && (!value.model_plaza_enabled || !value.model_plaza_require_auth)) {
                return <Suspense fallback={<Feedback>正在加载…</Feedback>}><PublicModelPlaza config={value} /></Suspense>;
              }
              if (pub.page === "home" || pub.page === "public-patrol") {
                return <PublicHome config={value} session={null} page={pub.page} onEnterWorkspace={() => {}} />;
              }
              return (
                <Login
                  config={value}
                  mode={pub.page === "workspace" || pub.page === "model-plaza" || pub.page === "mcp-consent" ? "login" : pub.page}
                  params={pub.params}
                  onLogin={next => {
                    setBrowsingPublic(false);
                    setChanged(next);
                    if (pub.page !== "workspace" && pub.page !== "model-plaza" && pub.page !== "mcp-consent") navigateTo(next.admin ? "ops" : "keys", undefined, true);
                  }}
                />
              );
            }
            if ((browsingPublic && pub.page !== "model-plaza") || pub.page === "public-patrol") {
              return (
                <PublicHome
                  config={value}
                  session={active}
                  page={pub.page === "public-patrol" ? "public-patrol" : "home"}
                  onEnterWorkspace={() => {
                    setBrowsingPublic(false);
                    navigateTo(active.admin ? "ops" : "keys");
                  }}
                />
              );
            }
            return (
              <Workspace
                config={value}
                session={active}
                onOpenPublicHome={() => {
                  setBrowsingPublic(true);
                  navigatePath("/");
                }}
                logout={() => setChanged(null)}
              />
            );
          }}
        </Resource>
      </WorkspaceErrorBoundary>
    </TooltipProvider>
  );
}

function SetupWizard() {
  const [address, setAddress] = useState("");
  const [candidates, setCandidates] = useState<{url: string; version: string}[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function discover() {
    setBusy(true);
    setError("");
    try {
      const result = await api("/setup/discover", setupCandidatesSchema, "POST", {address}, false);
      setCandidates(result.candidates);
      if (result.candidates.length === 0) setError("没有找到可验证的本机 Sub2API，请输入地址后重试。");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "发现失败");
    } finally {
      setBusy(false);
    }
  }
  async function confirm(url: string) {
    setBusy(true);
    setError("");
    try {
      await api("/setup/confirm", empty, "POST", {url}, false);
      location.reload();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "连接失败");
      setBusy(false);
    }
  }
  return (
    <StageCard title="连接 Sub2API" subtitle="只验证本机回环或你填写的地址">
      <Form busy={busy} id="setup-discover" onSubmit={() => void discover()}>
        <Field label="Sub2API 地址">
          <Input name="address" value={address} onChange={event => setAddress(event.target.value)} placeholder="http://127.0.0.1:3000" />
        </Field>
      </Form>
      {candidates.length > 0 && (
        <InsetGroup title="找到的实例">
          {candidates.map(candidate => (
            <InsetRow key={candidate.url} label={candidate.url} hint={candidate.version || undefined} onClick={busy ? undefined : () => void confirm(candidate.url)} />
          ))}
        </InsetGroup>
      )}
      <ErrorLine value={error} />
      <Button size="lg" type="submit" form="setup-discover" busy={busy}>查找并验证</Button>
    </StageCard>
  );
}

const root = document.querySelector("#root");
if (root) createRoot(root).render(<App />);
