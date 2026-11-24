# Sub2API v0.2.23 兼容性调查

源码基线为正式版 `v0.2.23`，对应提交 `5af9afb588c4b014b3df9db7d4073af1f8ae01b6`。面板可和官方实例同机运行，使用独立域名、会话和发码数据库。官方实例继续负责用户身份、账户余额、订阅、Key、计费记录和上游调度。 [官方版本](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.23)

## 面板依赖的官方接口

以下管理和用户端点均位于 `/api/v1`。普通用户与子管理员使用自己的官方账号。子管理员的发码授权和额度存于面板的 PostgreSQL，主管理操作使用操作者自己的官方管理员会话。管理员首次登录后在新版工作区阅读官方承诺，并通过 `GET /admin/compliance` 和 `POST /admin/compliance/accept` 完成确认。

| 操作 | 官方接口与边界 |
|---|---|
| 登录、注册与多元认证 | 面板原生承载公开首页、登录、注册、邮箱验证码、邀请码/优惠码实时校验、找回与重置密码、Passkey 通行密钥（`/auth/passkey/login/*`）及 6 类第三方 OAuth（`/auth/oauth/*` 与 `/auth/oauth/pending/*`）。开通和补充发码额度使用 `/user/totp/step-up`。面板后端密封官方令牌，浏览器不存明文凭据。 |
| 账户与密钥 | 面板读取本人余额、订阅和返利码，使用 `/keys` 管理本人密钥（含限额、窗口配额与到期设置）。列表显示掩码。本人复制或导入客户端时按需查询官方 `GET /keys/{id}`，并在面板上再次验证所有者。对应响应设为 `Cache-Control: no-store`。管理员查看用户名下密钥时显示掩码，可经官方接口改绑分组。 |
| 兑换记录与可用渠道 | `GET /redeem/history` 按当前用户分页读取官方兑换记录。面板返回类型、金额和兑换时间。`GET /channels/available` 沿用官方的站点开关和当前用户的分组权限，展示渠道、可用分组、模型和基础定价。 |
| 公告、优惠码和返利 | 公告使用 `/announcements` 和 `/admin/announcements`，优惠码使用 `/admin/promo-codes`，返利使用 `/user/aff` 和 `/user/aff/transfer`。管理员列表仅展示掩码，创建后提供一次复制。返利转入由官方事务完成。 |
| 模型广场与渠道监控 | `GET /model-plaza` 按官方设置和用户权限返回分组及定价。渠道监控按站点开关显示主动探测或流量健康。管理员通过 `/admin/channel-monitors` 新增、编辑、删除主动监控规则。 |
| 操作审计和风控 | `/admin/audit-logs` 提供筛选、摘要详情和 TOTP 清空。`/admin/risk-control` 提供审核引擎和模式、拦截反馈、记录保留、分页记录及解封。面板响应剥离请求体、原始提示词和原始错误。 |
| 账号配置与计费探测 | `/admin/accounts/data` 用于账号配置导入和导出。面板剥离导出凭据及可能包含代理认证的标识。CRS 同步调用 `/admin/accounts/sync/crs` 的预览和确认接口。兼容 Sub2API 的余额查询使用 `/v1/usage`，Token 计费倍率使用 `/v1/sub2api/billing`。 |
| 托管支付 | 使用 `/payment/checkout-info` 与 `/payment/orders` 创建充值或订阅订单。后端固定回跳为新版面板的 `/payment/result`。匿名回跳使用官方签名恢复令牌，已登录可按本人的订单编号查阅。 |
| 支付订单管理 | 管理员在 `/admin/orders` 筛选和查看订单。取消、重试入账、发起退款和查询退款状态走官方 `/admin/payment/orders`。官方校验订单状态及支付实例的退款能力。 |
| 备份与恢复 | 管理员在 `/admin/backups` 管理官方数据库备份。启用官方敏感操作二次验证时，创建、下载、恢复及保存对象存储沿用官方二次验证，恢复还校验管理员密码。备份文件下载使用官方按需生成的短期链接。 |
| 官方网关插件 | 管理员使用 `/admin/plugins` 管理官方网关插件。上传支持至 128 MiB 的 `.s2plugin`，安装和敏感写入遵循官方二次验证。面板对插件配置中的敏感字段做掩码投影并在保存时由服务端合并未改动的密钥，非敏感字段直接按类型回显编辑。 |
| MCP 授权与批量生图 | MCP 授权确认和撤销走 `/user/mcp`。批量生图使用本人选中的 Gemini 密钥，在浏览器直接请求官方网关 `/v1/images/batches`，保留官方网关的来源 IP 校验。 |
| 发码与官方卡密池 | 子管理员单张签发经 `issuer` 调用 `POST /admin/redeem-codes/generate`（带 `Idempotency-Key`）；管理员通过 `/admin/redeem-codes` 管理卡密池、批量制卡和一步直充（历史卡密在面板侧强制掩码，明文仅创建响应展示一次）。 |
| 实时运维与排障 (Ops) | 管理员通过 `/admin/ops/*` 与 `/admin/dashboard/*` 读取实时并发、账号可用性、吞吐/错误趋势、请求追踪、分层错误诊断（剥离原始错误载荷）以及告警规则、静默和通知阈值设置；实时并发经面板 5 秒共享采样签名 SSE 下发。 |
| 渠道、分组与上游号池 | 管理员通过 `/admin/groups`（含 `/capacity-summary`、`/composite-routes`、`/rate-multipliers`）、`/admin/channels`、`/admin/accounts` 和 `/admin/proxies` 协同管理分组调价、复合路由、上游账号与代理节点。各平台 OAuth 授权码交换与凭据落库在 `serve` 进程内闭环完成，上游令牌不经手浏览器。 |
| 用户、订阅与全站用量 | 管理员通过 `/admin/users`（含余额调整与流水、名下 Key 改绑）、`/admin/subscriptions`（单人/批量分配、续期、重置、撤销）与 `/admin/usage`（支持全站汇总及按用户、分组、账号、Key、模型组合下钻）完成运营管理。 |

[认证路由](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/routes/auth.go) · [用户路由](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/routes/user.go) · [管理路由](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/routes/admin.go) · [发码处理](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/handler/admin/redeem_handler.go#L131) · [注册事务](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/service/auth_service.go#L1302)

## 发码密钥

`v0.2.23` 支持细粒度管理密钥，但权限目录还没有 `POST /admin/redeem-codes/generate`。发码进程 `issuer` 因此需要单独签发的全权管理密钥。面板仅由该进程持有密钥，代码只用它请求发码端点。官方权限目录将来加入发码后，可复验只发码密钥并替换。 [权限目录](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/pkg/adminaccess/catalog.go) · [管理密钥校验](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/middleware/admin_auth.go#L258)

此版本的旧版全局密钥仅能重新生成或删除已有记录。新全权密钥通过「管理访问密钥」创建并勾选全部管理权限。轮换发码进程专用密钥时，先签发新密钥，再更新 `issuer` 的环境变量并重启该进程。验证发码成功后再吊销旧密钥。 [官方旧版密钥处理](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/handler/admin/setting_handler_runtime.go)

## Key 用量与历史范围

- 持有有效网关 Key 时，可在新版公开入口 `/key-usage` 查询用量摘要、每日趋势和模型汇总。浏览器用该 Key 直连官方网关的 `GET /v1/usage`，官方仍按浏览器来源校验 IP。跨域部署应允许面板来源。用量统计部分失败时仍可能返回余额或配额。逐笔记录由用户接口提供。 [网关用量](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/handler/gateway_handler.go#L1594)
- 用户登录后，`GET /api/v1/usage` 提供分页明细，`GET /api/v1/usage/dashboard/snapshot-v2` 提供每日趋势和按请求模型汇总。两者都可接受数字 `api_key_id`，官方按当前会话校验归属。面板统一传入统计时区，用 `actual_cost` 显示实际扣费，并单列缓存读写 Token。 [用户用量处理](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/handler/usage_handler.go)
- 官方配置 `dashboard_aggregation.retention.usage_logs_days` 默认 `90`。实际可查范围取决于实例配置及已执行的清理。面板的汇总查询最多 90 天，逐笔记录可分页查全部仍保留的记录。 [官方配置](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/deploy/config.example.yaml#L824)

## 版本切换

升级官方实例时，在一次性实例重跑 `TestOfficialCompatibility`，核对登录与 TOTP、Key 用量、邀请码注册及复用、余额码兑换和账号调度。供应商相关的测活事件和人机验证还需要用部署环境选定的供应商实测。根目录 README 记录测试实例配置方式。
