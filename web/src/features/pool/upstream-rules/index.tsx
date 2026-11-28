import {useState} from "react";
import {Segmented} from "@/ui";
import {PassthroughView} from "./passthrough-view";
import {TlsProfilesView} from "./tls-view";

type RuleView = "passthrough" | "tls";

export function UpstreamRulesWorkspace({queryString = ""}: {queryString?: string}) {
  const [view, setView] = useState<RuleView>(() => new URLSearchParams(queryString).get("view") === "tls" ? "tls" : "passthrough");
  const switcher = (
    <Segmented label="上游规则视图" value={view} options={[["passthrough", "错误透传规则"], ["tls", "TLS 指纹档案"]] as const} onChange={setView} />
  );
  return view === "passthrough" ? <PassthroughView switcher={switcher} /> : <TlsProfilesView switcher={switcher} />;
}
