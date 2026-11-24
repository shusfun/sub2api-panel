import {useEffect, useState} from "react";
import {ExternalLink, FolderOpen, Puzzle, Trash2} from "lucide-react";
import {api} from "@/lib/http";
import {adminPluginsBundleSchema, builtinPluginStateSchema, extMarketOverviewSchema} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Badge,
  Button,
  EllipsisText,
  EmptyState,
  ErrorLine,
  Field,
  Form,
  InlineStack,
  Input,
  InsetGroup,
  InsetRow,
  Overlay,
  PluginSandboxFrame,
  RefreshMorphIcon,
  Resource,
  Select,
  Stack,
  Switch,
  Tooltip,
  WorkspaceContent,
  WorkspaceStage,
  toast,
} from "@/ui";
import {z} from "zod";
import {OfficialPlugins} from "./official-plugins";

type ExtPlugin = z.infer<typeof extMarketOverviewSchema>["plugins"][number];

export function PluginsWorkspace({stepUpEnabled = false}: {stepUpEnabled?: boolean}) {
  const [revision, revise] = useState(0);
  const bundle = useResource(() => api("/admin/plugins", adminPluginsBundleSchema), "admin-plugins", true, revision);
  const [marketURL, setMarketURL] = useState("");
  const [folderOpen, setFolderOpen] = useState(false);
  const [folderPath, setFolderPath] = useState("");
  const [optionDrafts, setOptionDrafts] = useState<Record<string, Record<string, string>>>({});
  const [previewPlugin, setPreviewPlugin] = useState<ExtPlugin>();

  const act = useAction();

  useEffect(() => {
    if (bundle?.value?.market.market_url !== undefined) {
      setMarketURL(bundle.value.market.market_url);
    }
  }, [bundle?.value?.market.market_url]);

  return (
    <WorkspaceStage>
      <WorkspaceContent>
        <Resource state={bundle}>
          {data => (
            <Stack>
              <div className="auth-inline-row">
                <InlineStack wrap>
                  <h2>双轨扩展生态</h2>
                  <Badge variant="soft" tone="neutral">TOML 受控配置 · 凭据测完即焚</Badge>
                </InlineStack>
                <InlineStack wrap>
                  <Button size="sm" variant="outline" onClick={() => setFolderOpen(true)}>
                    <FolderOpen aria-hidden />挂载本地插件目录
                  </Button>
                </InlineStack>
              </div>

              <ErrorLine value={act.error} />
              {data.upstream_enabled && <OfficialPlugins stepUpEnabled={stepUpEnabled} />}

              <InsetGroup title="外部插件市场源（registry.json）">
                <InsetRow label="自定义市场清单地址" hint="留空使用面板内置精选种子；支持 https:// 或本机回环地址" vertical>
                  <Input
                    aria-label="插件市场地址"
                    value={marketURL}
                    onChange={e => setMarketURL(e.target.value)}
                    placeholder="https://example.com/registry.json"
                    endAddon={
                      <Button
                        size="xs"
                        variant="outline"
                        busy={act.busy}
                        onClick={() =>
                          void act.run(async () => {
                            await api("/admin/plugins/market/refresh", extMarketOverviewSchema, "POST", {
                              market_url: marketURL.trim() || "default",
                            });
                            toast("已同步外部插件市场清单");
                            revise(v => v + 1);
                          })
                        }
                      >
                        <RefreshMorphIcon />同步市场源
                      </Button>
                    }
                  />
                </InsetRow>
              </InsetGroup>

              {data.market.plugins.length === 0 ? (
                <EmptyState title="暂无外部插件" description="可通过上方市场清单同步或直接挂载本地插件包。" />
              ) : (
                <div className="plugin-market-grid">
                  {data.market.plugins.map(plug => {
                    const drafts = optionDrafts[plug.id] ?? plug.options;
                    return (
                      <article key={plug.id} className="plugin-card">
                        <div className="plugin-card-head">
                          <div className="plugin-card-title-wrap">
                            <InlineStack>
                              <Puzzle className="rail-icon" aria-hidden />
                              <EllipsisText text={plug.name} className="font-semibold" />
                              {plug.license && <Badge variant="soft" tone="neutral">{plug.license}</Badge>}
                              {plug.version && <span className="muted text-fine">v{plug.version}</span>}
                            </InlineStack>
                            <EllipsisText text={`${plug.publisher}${plug.repo ? ` · ${plug.repo}` : ""}`} className="muted text-fine" />
                          </div>
                          <Switch
                            label={`启用 ${plug.name}`}
                            checked={plug.enabled}
                            disabled={act.busy}
                            onChange={next =>
                              void act.run(async () => {
                                await api(`/admin/plugins/external/${encodeURIComponent(plug.id)}`, extMarketOverviewSchema, "PUT", {
                                  enabled: next,
                                });
                                toast(`${next ? "已启用" : "已停用"} ${plug.name}`);
                                revise(v => v + 1);
                              })
                            }
                          />
                        </div>

                        <EllipsisText text={plug.summary} lines={2} className="plugin-card-summary text-body" />

                        {plug.permissions.length > 0 && (
                          <InlineStack wrap>
                            {plug.permissions.slice(0, 4).map(perm => (
                              <Badge key={perm} variant="soft" tone="info">{perm}</Badge>
                            ))}
                            {plug.permissions.length > 4 && (
                              <Tooltip content={plug.permissions.slice(4).join(" · ")} side="top">
                                <Badge variant="tag">+{plug.permissions.length - 4}</Badge>
                              </Tooltip>
                            )}
                          </InlineStack>
                        )}

                        {plug.enabled && plug.option_specs.length > 0 && (
                          <Stack>
                            {plug.option_specs.map(spec => (
                              <Field key={spec.key} label={spec.title}>
                                <Input
                                  value={drafts[spec.key] ?? ""}
                                  placeholder={spec.placeholder || spec.default}
                                  onChange={e =>
                                    setOptionDrafts(prev => ({
                                      ...prev,
                                      [plug.id]: {...drafts, [spec.key]: e.target.value},
                                    }))
                                  }
                                />
                              </Field>
                            ))}
                            <InlineStack>
                              <Button
                                size="xs"
                                variant="outline"
                                busy={act.busy}
                                onClick={() =>
                                  void act.run(async () => {
                                    await api(`/admin/plugins/external/${encodeURIComponent(plug.id)}`, extMarketOverviewSchema, "PUT", {
                                      enabled: true,
                                      options: drafts,
                                    });
                                    toast(`已更新 ${plug.name} 配置`);
                                    revise(v => v + 1);
                                  })
                                }
                              >
                                保存端点配置
                              </Button>
                            </InlineStack>
                          </Stack>
                        )}

                        <div className="auth-inline-row">
                          <Badge variant="soft" tone={plug.ready ? "ok" : plug.enabled ? "warn" : "neutral"}>
                            {plug.status_note || (plug.ready ? "已就绪" : "未启用")}
                          </Badge>
                          <InlineStack>
                            {plug.homepage && (
                              <Button
                                size="xs"
                                variant="ghost"
                                onClick={() => window.open(plug.homepage, "_blank", "noopener,noreferrer")}
                              >
                                主页<ExternalLink aria-hidden />
                              </Button>
                            )}
                            {plug.page && plug.enabled && plug.ready && (
                              <Button size="xs" variant="outline" onClick={() => setPreviewPlugin(plug)}>
                                打开沙箱视图
                              </Button>
                            )}
                            {!plug.official_seed && plug.installed && (
                              <Button
                                size="xs"
                                variant="destructive"
                                busy={act.busy}
                                onClick={() =>
                                  void act.run(async () => {
                                    await api(`/admin/plugins/external/${encodeURIComponent(plug.id)}`, extMarketOverviewSchema, "DELETE");
                                    revise(v => v + 1);
                                  })
                                }
                              >
                                <Trash2 aria-hidden />移除
                              </Button>
                            )}
                          </InlineStack>
                        </div>
                      </article>
                    );
                  })}
                </div>
              )}

              <InsetGroup title="编译期内置插件（关闭时零内存与后台开销）">
                {data.builtin.map(plugin => (
                  <div key={plugin.id}>
                    <InsetRow
                      label={plugin.name}
                      hint={plugin.description + (plugin.reason ? ` · ${plugin.reason}` : "")}
                    >
                      <InlineStack>
                        <Badge variant="soft" tone={plugin.status === "active" ? "ok" : plugin.status === "failed" ? "bad" : "neutral"}>
                          {plugin.status === "active" ? "运行中" : plugin.status === "failed" ? "异常" : "已关闭"}
                        </Badge>
                        <Switch
                          label={`启用 ${plugin.name}`}
                          checked={plugin.enabled}
                          disabled={act.busy}
                          onChange={next =>
                            void act.run(async () => {
                              await api(`/admin/plugins/builtin/${plugin.id}`, z.object({builtin: z.array(builtinPluginStateSchema)}), "PUT", {
                                enabled: next,
                              });
                              revise(v => v + 1);
                            })
                          }
                        />
                      </InlineStack>
                    </InsetRow>
                    {plugin.enabled &&
                      plugin.settings.map(setting => (
                        <InsetRow key={`${plugin.id}:${setting.key}`} label={`└ ${setting.title}`} hint={setting.description}>
                          {setting.kind === "bool" ? (
                            <Switch
                              label={setting.title}
                              checked={Boolean(plugin.config[setting.key])}
                              disabled={act.busy}
                              onChange={next =>
                                void act.run(async () => {
                                  await api(`/admin/plugins/builtin/${plugin.id}`, z.object({builtin: z.array(builtinPluginStateSchema)}), "PUT", {
                                    config: {...plugin.config, [setting.key]: next},
                                  });
                                  revise(v => v + 1);
                                })
                              }
                            />
                          ) : setting.enum && setting.enum.length > 0 ? (
                            <Select
                              aria-label={setting.title}
                              value={String(plugin.config[setting.key] ?? "")}
                              onChange={e =>
                                void act.run(async () => {
                                  await api(`/admin/plugins/builtin/${plugin.id}`, z.object({builtin: z.array(builtinPluginStateSchema)}), "PUT", {
                                    config: {...plugin.config, [setting.key]: e.target.value},
                                  });
                                  revise(v => v + 1);
                                })
                              }
                            >
                              {setting.enum.map(opt => (
                                <option key={opt} value={opt}>{opt}</option>
                              ))}
                            </Select>
                          ) : (
                            <Input
                              aria-label={setting.title}
                              type={setting.kind === "int" ? "number" : "text"}
                              min={setting.min}
                              max={setting.max}
                              defaultValue={String(plugin.config[setting.key] ?? "")}
                              onBlur={e => {
                                const nextVal = setting.kind === "int" ? Number(e.target.value) : e.target.value;
                                if (String(nextVal) === String(plugin.config[setting.key] ?? "")) return;
                                void act.run(async () => {
                                  await api(`/admin/plugins/builtin/${plugin.id}`, z.object({builtin: z.array(builtinPluginStateSchema)}), "PUT", {
                                    config: {...plugin.config, [setting.key]: nextVal},
                                  });
                                  revise(v => v + 1);
                                });
                              }}
                            />
                          )}
                        </InsetRow>
                      ))}
                  </div>
                ))}
              </InsetGroup>
            </Stack>
          )}
        </Resource>
      </WorkspaceContent>

      <Overlay
        open={folderOpen}
        onClose={() => setFolderOpen(false)}
        title="挂载受控本地插件目录"
        footer={
          <InlineStack>
            <Button variant="outline" onClick={() => setFolderOpen(false)}>取消</Button>
            <Button type="submit" form="mount-plugin-folder" busy={act.busy}>校验并挂载</Button>
          </InlineStack>
        }
      >
        <Form
          id="mount-plugin-folder"
          busy={act.busy}
          onSubmit={e => {
            e.preventDefault();
            void act.run(async () => {
              await api("/admin/plugins/external/folder", extMarketOverviewSchema, "POST", {folder: folderPath.trim()});
              setFolderPath("");
              setFolderOpen(false);
              toast("已挂载本地插件包");
              revise(v => v + 1);
            });
          }}
        >
          <Field label="服务器本地插件目录路径（需含 panel-plugin.json 或 package.json）">
            <Input value={folderPath} onChange={e => setFolderPath(e.target.value)} placeholder="./data/ext-plugins/my-plugin" required />
          </Field>
          <ErrorLine value={act.error} />
        </Form>
      </Overlay>

      <Overlay
        open={Boolean(previewPlugin)}
        onClose={() => setPreviewPlugin(undefined)}
        title={previewPlugin?.page?.title ?? previewPlugin?.name ?? "插件沙箱视图"}
        footer={<Button variant="outline" onClick={() => setPreviewPlugin(undefined)}>关闭</Button>}
      >
        {previewPlugin && (
          <PluginSandboxFrame
            pluginId={previewPlugin.id}
            title={previewPlugin.name}
            permissions={previewPlugin.permissions}
          />
        )}
      </Overlay>
    </WorkspaceStage>
  );
}
