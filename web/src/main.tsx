import {useEffect, useState} from "react";
import {createRoot} from "react-dom/client";
import {ApiError, api, configSchema, sessionSchema, setSession, type SessionInfo} from "./api";
import {useResource} from "./requests";
import {Button, ErrorLine, Feedback, Resource} from "./ui";
import {Login} from "./pages/login";
import {Workspace} from "./workspace";
import "./styles.css";

function App() {
  const config = useResource(() => api("/config", configSchema, "GET", undefined, false), "config");
  const current = useResource(() => api("/session", sessionSchema).catch(error => {
    if (error instanceof ApiError && error.status === 401) return null;
    throw error;
  }), "session");
  const [changed, setChanged] = useState<SessionInfo | null>();
  const active = changed === undefined ? current?.value : changed;
  useEffect(() => { setSession(active ?? undefined); }, [active]);
  useEffect(() => {
    const end = () => setChanged(null);
    window.addEventListener("panel-session-ended", end);
    return () => window.removeEventListener("panel-session-ended", end);
  }, []);
  return <Resource state={config}>{value => {
    if (current?.error && changed === undefined) return <div className="login-stage">
      <div className="login-card"><ErrorLine value={current.error} /><Button onClick={() => location.reload()}>重新加载</Button></div>
    </div>;
    if (active === undefined) return <Feedback>正在加载…</Feedback>;
    if (active === null) return <Login config={value} onLogin={setChanged} />;
    return <Workspace config={value} session={active} logout={() => setChanged(null)} />;
  }}</Resource>;
}

const root = document.querySelector("#root");
if (root) createRoot(root).render(<App />);
