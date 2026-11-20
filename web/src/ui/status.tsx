import {statusLabel} from "../format";
import {Badge} from "./badge";

export function Status({value}: {value: string}) {
  const tone = value === "active" || value === "issued" ? "ok"
    : value === "pending" || value === "unknown" ? "warn"
    : value === "error" || value === "failed" ? "bad" : "neutral";
  return <Badge variant="soft" tone={tone}>{statusLabel(value)}</Badge>;
}
