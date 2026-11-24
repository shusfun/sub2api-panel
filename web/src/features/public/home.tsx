import {ArrowRight, BookOpen, Gauge} from "lucide-react";
import {api} from "@/lib/http";
import {mountedPluginsSchema, type Config, type SessionInfo} from "@/schemas";
import {useResource} from "@/lib/requests";
import {navigatePublic, type PublicPageID} from "@/app/route";
import {Badge, Button, InlineStack, PlatformBadge, PluginSandboxFrame, PublicLayout, Stack, StatusDot} from "@/ui";
import {PublicPatrolSection} from "@/features/observe/patrol";

const homeProviders = ["anthropic", "openai", "gemini", "grok", "deepseek", "kimi", "zhipu", "minimax"] as const;

function GatewayTopologyDiagram({endpoint}: {endpoint: string}) {
  return (
    <div className="public-terminal" aria-label="多协议网关路由拓扑">
      <div className="public-terminal-head">
        <InlineStack density="compact">
          <StatusDot tone="ok" />
          <span>多协议路由拓扑</span>
        </InlineStack>
        <span className="code-mono text-fine muted">{endpoint}</span>
      </div>
      <div className="public-terminal-body">
        <svg viewBox="0 0 640 208" width="100%" role="img" aria-label="Client SDKs -> Panel Guard -> Sub2API Pool -> OpenAI / Anthropic / Gemini / Bedrock">
          <defs>
            <marker id="gw-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto">
              <path d="M 0 1 L 7 4 L 0 7 z" fill="var(--weir-focus)" />
            </marker>
          </defs>
          {/* Client SDKs */}
          <rect x="12" y="64" width="124" height="80" rx="7" fill="var(--weir-raised)" stroke="var(--weir-line)" strokeWidth="1.2" />
          <text x="74" y="92" textAnchor="middle" fill="var(--weir-ink)" fontSize="13" fontWeight="600">Client SDKs</text>
          <text x="74" y="112" textAnchor="middle" fill="var(--weir-ink-2)" fontSize="11">OpenAI / Claude</text>
          <text x="74" y="128" textAnchor="middle" fill="var(--weir-ink-3)" fontSize="11">Gemini / CLI</text>

          {/* Arrow 1 */}
          <line x1="136" y1="104" x2="174" y2="104" stroke="var(--weir-focus)" strokeWidth="1.5" markerEnd="url(#gw-arrow)" />

          {/* Panel Guard */}
          <rect x="178" y="64" width="128" height="80" rx="7" fill="var(--weir-raised)" stroke="var(--weir-focus)" strokeWidth="1.4" />
          <text x="242" y="92" textAnchor="middle" fill="var(--weir-ink)" fontSize="13" fontWeight="600">Panel Guard</text>
          <text x="242" y="112" textAnchor="middle" fill="var(--weir-ink-2)" fontSize="11">鉴权 · 限额 · 计费</text>
          <text x="242" y="128" textAnchor="middle" fill="var(--weir-ink-3)" fontSize="11">审计 · 巡检门控</text>

          {/* Arrow 2 */}
          <line x1="306" y1="104" x2="344" y2="104" stroke="var(--weir-focus)" strokeWidth="1.5" markerEnd="url(#gw-arrow)" />

          {/* Sub2API Pool */}
          <rect x="348" y="64" width="128" height="80" rx="7" fill="var(--weir-raised)" stroke="var(--weir-ok)" strokeWidth="1.3" />
          <text x="412" y="92" textAnchor="middle" fill="var(--weir-ink)" fontSize="13" fontWeight="600">Sub2API Pool</text>
          <text x="412" y="112" textAnchor="middle" fill="var(--weir-ink-2)" fontSize="11">粘性调度 · 权重</text>
          <text x="412" y="128" textAnchor="middle" fill="var(--weir-ink-3)" fontSize="11">限流自愈 · 重试</text>

          {/* Fan-out arrows to Upstream Providers */}
          <path d="M 476 84 C 498 84, 498 32, 518 32" fill="none" stroke="var(--weir-focus)" strokeWidth="1.3" markerEnd="url(#gw-arrow)" />
          <path d="M 476 96 C 498 96, 498 80, 518 80" fill="none" stroke="var(--weir-focus)" strokeWidth="1.3" markerEnd="url(#gw-arrow)" />
          <path d="M 476 112 C 498 112, 498 128, 518 128" fill="none" stroke="var(--weir-focus)" strokeWidth="1.3" markerEnd="url(#gw-arrow)" />
          <path d="M 476 124 C 498 124, 498 176, 518 176" fill="none" stroke="var(--weir-focus)" strokeWidth="1.3" markerEnd="url(#gw-arrow)" />

          {/* Upstreams */}
          <rect x="522" y="14" width="106" height="36" rx="7" fill="var(--weir-raised)" stroke="var(--weir-line)" strokeWidth="1.2" />
          <text x="575" y="36" textAnchor="middle" fill="var(--weir-ink)" fontSize="12" fontWeight="500">OpenAI</text>

          <rect x="522" y="62" width="106" height="36" rx="7" fill="var(--weir-raised)" stroke="var(--weir-line)" strokeWidth="1.2" />
          <text x="575" y="84" textAnchor="middle" fill="var(--weir-ink)" fontSize="12" fontWeight="500">Anthropic</text>

          <rect x="522" y="110" width="106" height="36" rx="7" fill="var(--weir-raised)" stroke="var(--weir-line)" strokeWidth="1.2" />
          <text x="575" y="132" textAnchor="middle" fill="var(--weir-ink)" fontSize="12" fontWeight="500">Gemini</text>

          <rect x="522" y="158" width="106" height="36" rx="7" fill="var(--weir-raised)" stroke="var(--weir-line)" strokeWidth="1.2" />
          <text x="575" y="180" textAnchor="middle" fill="var(--weir-ink)" fontSize="12" fontWeight="500">Bedrock</text>
        </svg>
      </div>
    </div>
  );
}

export function PublicHome({
  config,
  session,
  page,
  onEnterWorkspace,
}: {
  config: Config;
  session: SessionInfo | null | undefined;
  page: PublicPageID;
  onEnterWorkspace: () => void;
}) {
  const mounted = useResource(
    () => api("/plugins/mounted", mountedPluginsSchema, "GET", undefined, false),
    "public-mounted-plugins",
    true,
  );

  const siteName = config.site_name || "Sub2API";
  const subtitle = config.site_subtitle || "统一多渠道 AI API 网关、会话粘性调度与实时质量监控平台";
  const customContent = (config.home_content || "").trim();
  const isCustomURL = customContent.startsWith("https://") || customContent.startsWith("http://127.0.0.1");
  const canSignup = config.registration && !config.backend_mode;
  const homePlugins = (mounted?.value?.items ?? []).filter(p => p.mount === "home");
  const patrolPlugins = (mounted?.value?.items ?? []).filter(p => p.mount === "patrol");

  const navActions = (
    <>
      <Button size="sm" variant="ghost" render={<a href="/key-usage" />}>密钥用量</Button>
      {config.model_plaza_enabled && <Button size="sm" variant="ghost" onClick={() => navigatePublic("model-plaza")}>模型广场</Button>}
      {config.patrol_home_visible && (
        <Button
          size="sm"
          variant={page === "public-patrol" ? "secondary" : "ghost"}
          onClick={() => navigatePublic(page === "public-patrol" ? "home" : "public-patrol")}
        >
          <Gauge aria-hidden />
          <span>{page === "public-patrol" ? "返回首页" : "渠道状态"}</span>
        </Button>
      )}
      {config.doc_url && (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => window.open(config.doc_url, "_blank", "noopener,noreferrer")}
        >
          <BookOpen aria-hidden />
          <span>文档</span>
        </Button>
      )}
      {session ? (
        <Button size="sm" onClick={onEnterWorkspace}>
          <span>进入工作台</span>
          <ArrowRight aria-hidden />
        </Button>
      ) : (
        <>
          {canSignup && (
            <Button size="sm" variant="outline" onClick={() => navigatePublic("register")}>
              注册
            </Button>
          )}
          <Button size="sm" onClick={() => navigatePublic("login")}>
            登录
          </Button>
        </>
      )}
    </>
  );

  const layoutProps = {
    siteName,
    siteLogo: config.site_logo || undefined,
    subtitle: page === "public-patrol" ? "渠道巡检看板" : "网关与状态中心",
    actions: navActions,
    contactInfo: config.contact_info,
    docUrl: config.doc_url,
  };

  if (page === "public-patrol") {
    return (
      <PublicLayout {...layoutProps}>
        <PublicPatrolSection publicMode />
        {patrolPlugins.map(plug => (
          <PluginSandboxFrame key={plug.plugin_id} pluginId={plug.plugin_id} title={plug.title} />
        ))}
      </PublicLayout>
    );
  }

  if (isCustomURL) {
    return (
      <PublicLayout {...layoutProps} flush>
        <iframe className="public-content-frame" src={customContent} title={siteName} />
      </PublicLayout>
    );
  }

  if (config.compact_home) {
    return (
      <PublicLayout {...layoutProps}>
        <section className="public-feature-card">
          <Stack>
            <h1 className="public-hero-title">{siteName}</h1>
            <div className="public-hero-sub">{subtitle}</div>
            <InlineStack wrap>
              {session ? (
                <Button size="lg" onClick={onEnterWorkspace}>
                  进入工作台<ArrowRight aria-hidden />
                </Button>
              ) : (
                <>
                  <Button size="lg" onClick={() => navigatePublic("login")}>
                    立即登录<ArrowRight aria-hidden />
                  </Button>
                  {canSignup && (
                    <Button size="lg" variant="outline" onClick={() => navigatePublic("register")}>
                      创建账号
                    </Button>
                  )}
                </>
              )}
            </InlineStack>
          </Stack>
        </section>
        {config.patrol_home_visible && <PublicPatrolSection publicMode />}
        {homePlugins.map(plug => (
          <PluginSandboxFrame key={plug.plugin_id} pluginId={plug.plugin_id} title={plug.title} />
        ))}
      </PublicLayout>
    );
  }

  const endpointDisplay = config.api_base_url || location.origin;

  return (
    <PublicLayout {...layoutProps}>
      <section className="public-hero" aria-label="站点概览">
        <div className="public-hero-copy">
          <InlineStack wrap>
            <Badge variant="soft" tone="ok">OpenAI · Anthropic · Gemini 兼容</Badge>
            <Badge variant="soft" tone="neutral">智能流量门控验真</Badge>
          </InlineStack>
          <h1 className="public-hero-title">{siteName}</h1>
          <div className="public-hero-sub">{customContent || subtitle}</div>
          <InlineStack wrap>
            {session ? (
              <Button size="lg" onClick={onEnterWorkspace}>
                <span>进入控制台</span>
                <ArrowRight aria-hidden />
              </Button>
            ) : (
              <>
                <Button size="lg" onClick={() => navigatePublic("login")}>
                  <span>开始使用</span>
                  <ArrowRight aria-hidden />
                </Button>
                {canSignup && (
                  <Button size="lg" variant="outline" onClick={() => navigatePublic("register")}>
                    注册新账号
                  </Button>
                )}
              </>
            )}
            {config.patrol_home_visible && (
              <Button size="lg" variant="ghost" onClick={() => navigatePublic("public-patrol")}>
                查看独立状态页
              </Button>
            )}
          </InlineStack>
        </div>

        <GatewayTopologyDiagram endpoint={endpointDisplay} />
      </section>

      {config.patrol_home_visible && <PublicPatrolSection publicMode />}

      {homePlugins.map(plug => (
        <PluginSandboxFrame key={plug.plugin_id} pluginId={plug.plugin_id} title={plug.title} />
      ))}

      <section className="public-feature-grid" aria-label="实时运行摘要">
        <article className="public-feature-card">
          <InlineStack density="compact">
            <StatusDot tone="ok" />
            <h3>网关接入点</h3>
          </InlineStack>
          <span className="code-mono text-fine">{endpointDisplay}</span>
          <span className="muted text-fine">原生兼容 OpenAI Responses / Chat Completions、Anthropic Messages 与 Gemini 协议。</span>
        </article>
        <article className="public-feature-card">
          <InlineStack density="compact">
            <Badge variant="soft" tone={canSignup ? "ok" : "neutral"}>{canSignup ? "开放注册" : "受邀模式"}</Badge>
            <h3>账号与接入策略</h3>
          </InlineStack>
          <span className="muted text-fine">
            {config.model_plaza_enabled ? "已开放公开模型广场与价目查询" : "登录后可在工作台查看可用模型与价目"}
            {config.patrol_home_visible ? " · 已开启渠道健康巡检" : ""}
          </span>
        </article>
        <article className="public-feature-card">
          <InlineStack density="compact">
            <Badge variant="soft" tone="info">{homeProviders.length} 大协议家族</Badge>
            <h3>多号池粘性调度</h3>
          </InlineStack>
          <span className="muted text-fine">支持按分组权重、会话粘性与限流冷却自动切换上游节点，逐笔记录 Token 与缓存成本。</span>
        </article>
      </section>

      <section className="public-feature-card" aria-label="支持的上游家族">
        <div className="auth-inline-row">
          <h3>支持的主流模型家族</h3>
          <div className="public-provider-row">
            {homeProviders.map(p => (
              <PlatformBadge key={p} platform={p} />
            ))}
          </div>
        </div>
      </section>
    </PublicLayout>
  );
}
