# Sub2API 管理面板

这是 Sub2API 管理面板的独立项目。面板提供公开首页、完整注册/登录/Passkey/第三方 OAuth 认证、流量门控智能渠道巡检、类 Magpie 双轨插件生态，以及个人和全站管理工作台，所有官方数据写入均经官方 API 完成。面板包含一个 React 前端和两个 Go 进程，可与官方 Sub2API 同机部署，沿用原有官方站点的升级方式。

## 构建

需要 Go 1.25、Node.js 22 和 pnpm 10。从仓库根目录运行下列命令。Windows 下将 Go 输出文件命名为 `sub2api-panel.exe`。

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go build -tags panel_release -o sub2api-panel .
```

正式二进制内嵌前端。调试构建可省略 `panel_release` 标签，并通过 `PANEL_WEB_DIST` 指向静态文件。会话与加密密钥保存在面板进程内存中，重启后重新登录。发码、审计和运行配置统一保存在 PostgreSQL 中。

`sub2api-panel --help` 查看命令，`sub2api-panel version` 查看版本。下载对应平台与架构的发布包后，先对照发布的 `checksums.txt` 验证压缩包。面板的 `update check` 和 `update install` 使用本地 JSON 清单，其字段为 `version`、`os`、`arch` 和候选二进制的 `sha256`。安装命令需要指定现行二进制路径，恢复时使用 `update rollback --target`。更换前停止服务管理器中的面板进程，更换完成后启动。官方实例版本在「设置 → 系统信息」查看和管理。

## 同机 HTTPS 部署

用独立的 HTTPS 域名访问面板。下面的示例由 Caddy 终止 TLS，只将这个域名转发到面板进程。保留原有官方站点的独立配置。反向代理应保留原始路径、查询、签名请求头和 `Origin`，并让测活事件及时下发。

```caddyfile
panel.example.com {
    reverse_proxy 127.0.0.1:8741 {
        flush_interval -1
    }
}
```

分别向两个进程提供环境变量，再交由系统服务管理器启动。两个监听器只绑定回环地址，反向代理只指向 `serve`。

| 环境变量 | 进程 | 示例或说明 |
|---|---|---|
| `PANEL_DATABASE_URL` | 两个进程 | PostgreSQL 连接串；迁移命令使用管理角色，两个进程使用受限角色 |
| `PANEL_UPSTREAM` | 首次启动 | 可选的本机 Sub2API 地址种子，确认后写入 runtime 配置 |
| `PANEL_ISSUER_ADDR` | 两个进程 | `127.0.0.1:8742` |
| `PANEL_ISSUER_SECRET` | 两个进程 | 两个进程共享的随机通信密钥，可用 `openssl rand -hex 32` 生成 |
| `PANEL_UPSTREAM_ADMIN_KEY` | `issuer` | 为发码进程单独签发的官方管理密钥 |
| `PANEL_ADDR` | `serve` | `127.0.0.1:8741` |
| `PANEL_ORIGIN` | `serve` | `https://panel.example.com`，与浏览器看到的源完全一致 |
| `PANEL_OFFICIAL_ORIGIN` | `serve` | `https://api.example.com`，官方注册和购买所在站点 |
| `PANEL_WEB_DIST` | `serve` | 调试构建的外置前端目录，例如 `/opt/sub2api-panel/web/dist` |
| `PANEL_OFFICIAL_DATABASE_URL` | `serve` | 可选。官方 Sub2API 数据库的只读连接，用于上游账号当日扣费/成败统计及智能巡检的滑动窗口去重活跃用户聚合；未配置时自动回退官方接口 |
| `PANEL_USAGE_TIMEZONE` | `serve` | 用量日期和趋势使用的 IANA 时区，与官方 Sub2API 的 `timezone` 保持一致，默认 `Asia/Shanghai` |
| `PANEL_DATA_DIR` | `serve` | 可选。插件受控配置（`managed/*.toml`）、外部插件包与模型指纹缓存目录，默认 `./data` |
| `PANEL_PLUGIN_MARKET` | `serve` | 可选。外部插件市场 `registry.json` 默认地址种子 |

```sh
./sub2api-panel migrate
./sub2api-panel issuer
./sub2api-panel serve
```

官方 Sub2API `v0.2.23` 的受限管理密钥权限目录未收录发码接口。为 `issuer` 单独签发具备完整访问权限的管理密钥。把这把密钥只交给 `issuer`。`issuer` 用它访问官方发码路由，读取用户、核对码和验证 TOTP 使用操作者自己的官方登录凭据。生产部署可用不同的系统用户及独立环境文件运行两个进程。主管理员登录新版后阅读并提交官方首次确认，再到「设置 → 密码与验证」启用 TOTP，到「用户」开通或追加发码额度。

## 发码和核对

注册邀请码在官方开启邀请码注册后可用，每张有效 7 天。余额码每张有效 30 天。用户获得邀请码张数后可以发邀请码，获得余额码预算和允许面额后可以发余额码。额度独立于用户账户余额。面板将余额按美分预扣，官方确定拒绝时退回。撤权会清除旧授权下尚可使用的额度。

遇到结果待核对时，主管理员从发码记录和官方服务器日志对照请求编号，确认创建结果后填入官方码 ID，或确认未生成后释放预扣。发码页面只在生成成功时展示码本身。面板和发码进程均输出 JSON 日志，响应的 `X-Request-ID` 将面板、发码审计和官方请求关联起来。

## 验证

从仓库根目录运行 `go test ./...`。集成夹具使用 `PANEL_TEST_DATABASE_URL` 指定的临时 PostgreSQL，覆盖面板会话、权限、发码和账号管理。上线前以选定的官方版本复验登录、TOTP、发码、注册、兑换和测活。官方开启人机验证的部署还需使用目标域名验证其配置的服务商。

可用一次性的官方测试实例运行 `go test -run '^TestOfficialCompatibility$' -v`。通过 `PANEL_TEST_OFFICIAL` 指定 API 源地址，通过 `PANEL_TEST_ADMIN_FILE` 指定仅供测试的 `{ "email": "…", "password": "…" }` 文件。使用尚未完成官方管理员确认、尚未启用 TOTP 的测试账号。此用例会提交官方确认、启用 TOTP、修改注册配置，并创建测试用户、管理密钥和账号。

### 新版面板入口

- 管理员在「站点公告」创建、投放和归档公告，在「卡密与优惠」统一管理发码、兑换码与注册优惠码并核对使用记录。登录用户在「站点公告」查看可见公告并一键全部标为已读。
- 「上游账号」支持全量 12 平台与 6 种账号类型（含 Bedrock、Service Account、OAuth、Cookie 授权、直填 Token）、结构化防封控量与模型映射、账号配置导入、脱敏导出与 CRS 同步。
- 「调度分组」与「用户」提供独立多页签工作舱：分组覆盖路由兜底链、渠道模型定价、复合路由、专属倍率与统计；用户覆盖角色授权、API Key 代管、订阅批量分配、余额流水与自定义用户扩展属性。
- 「邀请返利」遵循官方站点开关。登录用户可复制新版注册邀请链接、查看累计与冻结返利，并将可用返利转入余额。
- 「模型广场」整合模型目录与可用渠道价格对比，沿用官方分组可见性和登录开关。公开「密钥用量」和登录后的「批量生图」由浏览器直接调用官方网关，跨域部署需在官方网关的 CORS 设置中允许新版面板的来源。批量生图支持参考图及任务和结果分页。失败条目可编辑提示词后提交关联任务。
- 「购买与订单」支持返回新版的托管链接及扫码支付流程。支付服务商跳回 `/payment/result`，由官方订单结果决定页面状态。Stripe 和 Airwallex 的专用收银组件、微信内 JSAPI 尚待接入。
- 「支付订单」向管理员提供订单筛选、详情、取消、入账重试和退款状态操作。退款使用官方配置的支付服务商。
- 「备份与恢复」使用可视化周期选择器管理官方数据库的对象存储、定时计划、月度归档、备份记录和下载。启用官方敏感操作二次验证时，保存对象存储、创建备份、获取下载链接和恢复需要验证器。恢复还需当前管理员密码。
- 「插件生态」在官方网关开启插件管理时提供安装、配置、启停、测试和卸载。官方网关配置编辑使用插件所需的完整 JSON。
- 「渠道健康」整合智能巡检 60 格脉搏矩阵与官方主动渠道监控规则。管理员可创建、编辑和删除主动监控规则，按检查模式提供探测端点或关联账号。
- 将官方 Sub2API 的 MCP 授权前端地址指向新版面板后，授权链接会打开新版 `/oauth/mcp/authorize`；MCP 协议和令牌签发继续由官方实例提供。

## Windows 本地预览

构建前端后，从仓库根目录运行下列命令。预览固定监听 `127.0.0.1:28741`，不会自动切换端口。测试夹具使用模拟上游和 `PANEL_TEST_DATABASE_URL` 指定的 PostgreSQL。预览账号为 `admin@example.test`、`user@example.test` 和 `other@example.test`，测试密码均为 `fixture-password`，主管理员的验证码为 `123456`。

```powershell
$env:PANEL_TEST_BROWSER_ADDR = '127.0.0.1:28741'
go test -run '^TestBrowserFixture$' -v -timeout 0
```

开发面板页面时，在宿主机用 `pnpm --dir web dev` 启动 Vite，再给同一个预览夹具设置 `PANEL_TEST_BROWSER_VITE=local`。在 Docker 里的夹具用 `PANEL_TEST_BROWSER_VITE=docker`。仍从 `http://127.0.0.1:28741` 打开面板。夹具转发前端模块和热更新连接，Vite 固定监听宿主机的 `127.0.0.1:28742`。Vite 停止后，重新加载页面即可使用已构建的前端。独立的验证码页面使用构建产物。

可将 `PANEL_TEST_BROWSER_USAGE_FILE` 指向本机脱敏 JSON，用它替换预览用量。文件是最大 32 MiB 的记录数组，每条包含 `id`、`user_id`（1–11 对应测试账号）、`api_key_id`（从 1000 起）、`created_at`（RFC 3339）、`model`、`input_tokens`、`output_tokens`、`cache_read_tokens`、`cache_creation_tokens` 和 `actual_cost`。样本包含的额外测试账号会出现在登录选项中。预览按记录的原始日期查询，可通过自定义区间查看历史样本。

可将 `PANEL_TEST_BROWSER_CATALOG_FILE` 指向脱敏管理样本 JSON。其数组字段为 `keys`、`groups`、`accounts`、`codes`、`announcements`，条目只接收 `status`、`platform`、`type`、`rate`、`priority` 和 `value`。预览生成本地名称和标识。两个样本文件均放在仓库之外。

## 上游版本与源码来源

当前 API 基线为官方 `v0.2.23`。接口、权限和升级复验项见 [Sub2API 兼容性调查](docs/sub2api-compatibility.md)。

本仓库以原 `panel/` 工作树为初始快照，起点是面板提交 `b3d4879b1`，同时迁入其后的用量与界面改版。根目录保留原 Sub2API 工作树的 LGPL-3.0 许可证。从 Weir 复用的 UI 组件保留 [CodedThemes MIT 声明](web/src/ui/LICENSE)。 [官方 Sub2API](https://github.com/Wei-Shaw/sub2api)
