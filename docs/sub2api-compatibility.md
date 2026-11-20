# Sub2API v0.2.23 兼容性调查

源码基线为正式版 `v0.2.23`，对应提交 `5af9afb588c4b014b3df9db7d4073af1f8ae01b6`。面板可和官方实例同机运行，使用独立域名、会话和发码数据库。官方实例继续负责用户身份、账户余额、订阅、Key、计费记录和上游调度。 [官方版本](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.23)

## 面板依赖的官方接口

以下管理和用户端点均位于 `/api/v1`。普通用户与子管理员使用自己的官方账号。子管理员的发码授权和额度存于面板的 SQLite，主管理操作使用操作者自己的官方管理员会话。

| 操作 | 官方接口与边界 |
|---|---|
| 登录与二次验证 | 使用 `/auth/login`、`/auth/login/2fa`、`/auth/me` 和 `/auth/refresh`。开通和补充发码额度使用 `/user/totp/step-up`。面板后端保管官方令牌。 |
| 账户与 Key | 面板读取本人余额、订阅和返利码，使用 `/keys` 管理本人 Key。注册、支付和系统设置沿用官方页面。 |
| 发码与兑换 | `POST /admin/redeem-codes/generate` 接受有效期和 `Idempotency-Key`。普通用户通过 `/redeem` 兑换余额码，受邀者在官方注册流程使用邀请码。开启邀请码注册后，官方在同一事务创建用户并占用邀请码。 |
| 管理用户与上游账号 | 管理员通过 `/admin/users` 和 `/admin/usage` 查看用户与用量。账号列表、额度、测活与调度分别使用 `/admin/accounts`、`/admin/accounts/:id/usage`、`/admin/accounts/:id/test` 和 `/admin/accounts/:id/schedulable`。面板映射账号字段与 SSE 事件，测活只返回进度和归一化结果。 |

[认证路由](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/routes/auth.go) · [用户路由](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/routes/user.go) · [管理路由](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/routes/admin.go) · [发码处理](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/handler/admin/redeem_handler.go#L131) · [注册事务](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/service/auth_service.go#L1302)

## 发码密钥

`v0.2.23` 支持细粒度管理密钥，但权限目录还没有 `POST /admin/redeem-codes/generate`。发码进程 `issuer` 因此需要单独签发的全权管理密钥。面板仅由该进程持有密钥，代码只用它请求发码端点。官方权限目录将来加入发码后，可复验只发码密钥并替换。 [权限目录](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/pkg/adminaccess/catalog.go) · [管理密钥校验](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/server/middleware/admin_auth.go#L258)

## Key 用量与历史范围

- 持有有效网关 Key 时，`GET /v1/usage` 可按该 Key 查询用量摘要、每日趋势、模型汇总。用量字段为尽力查询，部分统计失败时仍可能返回余额或配额。逐笔记录由用户接口提供。 [网关用量](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/handler/gateway_handler.go#L1594)
- 用户登录后，`GET /api/v1/usage` 提供分页明细，`GET /api/v1/usage/dashboard/snapshot-v2` 提供每日趋势和按请求模型汇总。两者都可接受数字 `api_key_id`，官方按当前会话校验归属。面板统一传入统计时区，用 `actual_cost` 显示实际扣费，并单列缓存读写 Token。 [用户用量处理](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/backend/internal/handler/usage_handler.go)
- 官方配置 `dashboard_aggregation.retention.usage_logs_days` 默认 `90`。实际可查范围取决于实例配置及已执行的清理。面板的汇总查询最多 90 天，逐笔记录可分页查全部仍保留的记录。 [官方配置](https://github.com/Wei-Shaw/sub2api/blob/v0.2.23/deploy/config.example.yaml#L824)

## 版本切换

升级官方实例时，在一次性实例重跑 `TestOfficialCompatibility`，核对登录与 TOTP、Key 用量、邀请码注册及复用、余额码兑换和账号调度。供应商相关的测活事件和人机验证还需要用部署环境选定的供应商实测。根目录 README 记录测试实例配置方式。
