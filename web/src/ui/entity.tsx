import type {CSSProperties, ReactNode} from "react";
import {Cpu, Hash, KeyRound, Layers3, Network, Server, UserRound, Workflow} from "lucide-react";
import {cn} from "cn";
import {entityHref, type EntityKind} from "../route";
import {Badge} from "./badge";
import {CopyButton} from "./copy-button";
import {Tooltip} from "./tooltip";
import typesafeLogo from "./platform-logos/typesafe.png";

export const platformLabels: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  gemini: "Gemini",
  antigravity: "Antigravity",
  grok: "Grok",
  composite: "复合路由",
  deepseek: "DeepSeek",
  kimi: "Kimi",
  zhipu: "智谱",
  minimax: "MiniMax",
  opencode_go: "OpenCode Go",
  typesafe: "TypeSafe",
  command_code: "Command Code",
  cline: "Cline",
};

const platformLogos = import.meta.glob<string>("./platform-logos/*.svg", {query: "?url", import: "default", eager: true});

export function platformLabel(platform: string) {
  return Object.hasOwn(platformLabels, platform) ? platformLabels[platform] : platform;
}

const accountTypes: Record<string, string> = {
  oauth: "OAuth", apikey: "API Key", "setup-token": "Setup Token", upstream: "上游转发",
  bedrock: "Bedrock", service_account: "服务账号",
};

export function PlatformBadge({platform, type}: {platform: string; type?: string}) {
  if (!platform) return <span className="muted">—</span>;
  const tone = platform === "composite" ? "info" : "neutral";
  const logo = platformLogos[`./platform-logos/${platform}.svg`];
  return (
    <Badge variant="soft" tone={tone} className="swift-platform-badge">
      {platform === "typesafe" ? <img src={typesafeLogo} className="platform-mark" alt="" /> : platform === "composite" ? <Workflow aria-hidden /> : logo ?
        <span aria-hidden className="platform-logo" style={{"--platform-logo": `url("${logo}")`} as CSSProperties} /> : <Cpu aria-hidden />}
      {platformLabel(platform)}
      {type && <span className="muted"> · {Object.hasOwn(accountTypes, type) ? accountTypes[type] : type}</span>}
    </Badge>
  );
}

const entityIcons: Record<EntityKind, typeof UserRound> = {
  user: UserRound,
  group: Layers3,
  account: Server,
  proxy: Network,
  key: KeyRound,
  model: Cpu,
  request: Hash,
};

export function EntityLink({
  kind,
  id,
  label,
  meta,
  extra,
  onClick,
  className,
}: {
  kind: EntityKind;
  id: string | number;
  label?: ReactNode;
  meta?: ReactNode;
  extra?: Record<string, string | number | undefined>;
  onClick?: () => void;
  className?: string;
}) {
  const Icon = entityIcons[kind];
  const display = label ?? `#${id}`;
  const href = entityHref(kind, id, extra);

  const tip = typeof display === "string" && display.length > 14 ? display : undefined;

  if (onClick) {
    const el = (
      <button
        type="button"
        onClick={event => {
          event.stopPropagation();
          onClick();
        }}
        className={cn("swift-entity weir-press-fill", className)}
      >
        <Icon aria-hidden />
        <span className="swift-entity-label">{display}</span>
        {meta !== undefined && <span className="swift-entity-meta">{meta}</span>}
      </button>
    );
    return tip ? <Tooltip content={tip} side="top">{el}</Tooltip> : el;
  }

  const link = (
    <a
      href={href}
      onClick={event => event.stopPropagation()}
      className={cn("swift-entity weir-press-fill", className)}
    >
      <Icon aria-hidden />
      <span className="swift-entity-label">{display}</span>
      {meta !== undefined && <span className="swift-entity-meta">{meta}</span>}
    </a>
  );
  return tip ? <Tooltip content={tip} side="top">{link}</Tooltip> : link;
}

export function TraceId({value, linkToOps = false}: {value: string; linkToOps?: boolean}) {
  if (!value) return <span className="muted">—</span>;
  const short = value.length > 12 ? `${value.slice(0, 8)}…` : value;
  const codeNode = linkToOps ? (
    <a href={entityHref("request", value)} onClick={e => e.stopPropagation()} className="code-mono">
      {short}
    </a>
  ) : (
    <code className="code-mono">{short}</code>
  );
  return (
    <span className="swift-trace">
      {value.length > 12 ? <Tooltip content={value} side="top">{codeNode}</Tooltip> : codeNode}
      <CopyButton value={value} label="复制请求编号" size="icon-xs" />
    </span>
  );
}
