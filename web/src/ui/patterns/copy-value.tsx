import {useState} from "react";
import {Button} from "../primitives/button";
import {CopyIcon} from "../icons/copy-icon";

export function CopyValue({value, label}: {value: string; label: string}) {
  const [result, setResult] = useState({value: "", feedback: ""});
  const feedback = result.value === value ? result.feedback : "";
  return <div className="copy-result">
    <span>{label}</span><code>{value}</code>
    <Button size="icon" variant="outline" title="复制" aria-label={"复制" + label} onClick={async () => {
      try { await navigator.clipboard.writeText(value); setResult({value, feedback: "已复制"}); }
      catch { setResult({value, feedback: "请手动复制"}); }
    }}><CopyIcon copied={feedback === "已复制"} /></Button>
    {feedback && <small role="status">{feedback}</small>}
  </div>;
}
