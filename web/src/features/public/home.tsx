import {ArrowRight, BookOpen, Gauge} from "lucide-react";
import {api} from "@/lib/http";
import {mountedPluginsSchema, type Config, type SessionInfo} from "@/schemas";
import {useResource} from "@/lib/requests";
import {navigatePublic, type PublicPageID} from "@/app/route";
import {Badge, Button, InlineStack, PlatformBadge, PluginSandboxFrame, PublicLayout, Stack} from "@/ui";
import {PublicPatrolSection} from "@/features/observe/patrol";

const homeProviders = ["anthropic", "openai", "gemini", "grok", "deepseek", "kimi", "zhipu", "minimax"] as const;

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

        <div className="public-terminal" aria-label="网关调用示例">
          <div className="public-terminal-head">
            <span className="public-terminal-dots" aria-hidden>
              <i /><i /><i />
            </span>
            <span>gateway-verify</span>
          </div>
          <div className="public-terminal-body">
            <div>$ curl -X POST {endpointDisplay}/v1/responses</div>
            <div className="terminal-comment"># 会话粘性路由 · 实时计费与品质巡检</div>
            <div>
              <span className="terminal-success">200 OK</span>{" "}
              <span>{"{ \"model\": \"gpt-5.4\", \"status\": \"completed\" }"}</span>
            </div>
          </div>
        </div>
      </section>

      {config.patrol_home_visible && <PublicPatrolSection publicMode />}

      {homePlugins.map(plug => (
        <PluginSandboxFrame key={plug.plugin_id} pluginId={plug.plugin_id} title={plug.title} />
      ))}

      <section className="public-feature-grid" aria-label="平台核心特性">
        <article className="public-feature-card">
          <h3>统一原生协议网关</h3>
          <span>同时兼容 OpenAI Responses、Chat Completions 与 Anthropic Messages 流式调用，一行配置平滑接入各类开发客户端。</span>
        </article>
        <article className="public-feature-card">
          <h3>多号池粘性调度与限流自愈</h3>
          <span>按分组权重、实时水位和会话上下文智能路由，自动回避限流冷却节点并在恢复后平滑重入。</span>
        </article>
        <article className="public-feature-card">
          <h3>实时账单与省 Token 品质巡检</h3>
          <span>逐笔记录输入、输出与缓存读写成本，结合活跃用户数自适应触发指纹与降智抽检，公开透明呈现真实通道状态。</span>
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
