# Sub2API 管理面板

这是 Sub2API 管理面板的独立项目。面板通过官方 API 使用现有账号。注册、购买和系统设置在官方站点完成。面板包含一个 React 前端和两个 Go 进程，可与官方 Sub2API 同机部署，沿用原有官方站点的升级方式。

## 构建

需要 Go 1.25、Node.js 22 和 pnpm 10。从仓库根目录运行下列命令。Windows 下将 Go 输出文件命名为 `sub2api-panel.exe`。

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go build -o sub2api-panel .
```

将二进制和 `web/dist` 放到服务器。为发码进程准备持久化目录，授予它独占的读写权限。会话与加密密钥保存在面板进程内存中，重启后重新登录。发码额度和发码记录保存在 SQLite 中，重启后继续使用同一数据库。备份时使用 SQLite 的备份接口，避免漏掉 WAL 中的数据。

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
| `PANEL_UPSTREAM` | 两个进程 | `http://127.0.0.1:8080`，同机官方 API 的源地址 |
| `PANEL_ISSUER_ADDR` | 两个进程 | `127.0.0.1:8742` |
| `PANEL_ISSUER_SECRET` | 两个进程 | 两个进程共享的随机通信密钥，可用 `openssl rand -hex 32` 生成 |
| `PANEL_ISSUER_DB` | `issuer` | `/var/lib/sub2api-panel/issuer.db` |
| `PANEL_UPSTREAM_ADMIN_KEY` | `issuer` | 为发码进程单独签发的官方管理密钥 |
| `PANEL_ADDR` | `serve` | `127.0.0.1:8741` |
| `PANEL_ORIGIN` | `serve` | `https://panel.example.com`，与浏览器看到的源完全一致 |
| `PANEL_OFFICIAL_ORIGIN` | `serve` | `https://api.example.com`，官方注册、购买和系统设置所在站点 |
| `PANEL_WEB_DIST` | `serve` | `/opt/sub2api-panel/web/dist` |
| `PANEL_USAGE_TIMEZONE` | `serve` | 用量日期和趋势使用的 IANA 时区，与官方 Sub2API 的 `timezone` 保持一致，默认 `Asia/Shanghai` |

```sh
./sub2api-panel issuer
./sub2api-panel serve
```

官方 Sub2API `v0.2.23` 的受限管理密钥权限目录未收录发码接口。为 `issuer` 单独签发具备完整访问权限的管理密钥。把这把密钥只交给 `issuer`。`issuer` 用它访问官方发码路由，读取用户、核对码和验证 TOTP 使用操作者自己的官方登录凭据。生产部署可用不同的系统用户及独立环境文件运行两个进程。主管理员先登录官方面板，完成该版本要求的管理员确认并在账号中启用 TOTP，再到新面板的用户管理页开通或追加额度。

## 发码和核对

注册邀请码在官方开启邀请码注册后可用，每张有效 7 天。余额码每张有效 30 天。用户获得邀请码张数后可以发邀请码，获得余额码预算和允许面额后可以发余额码。额度独立于用户账户余额。面板将余额按美分预扣，官方确定拒绝时退回。撤权会清除旧授权下尚可使用的额度。

遇到结果待核对时，主管理员从发码记录和官方服务器日志对照请求编号，确认创建结果后填入官方码 ID，或确认未生成后释放预扣。发码页面只在生成成功时展示码本身。面板和发码进程均输出 JSON 日志，响应的 `X-Request-ID` 将面板、发码审计和官方请求关联起来。

## 验证

从仓库根目录运行 `go test ./...`。测试使用 HTTP 夹具和临时 SQLite，覆盖面板会话、权限、发码和账号管理。上线前以选定的官方版本复验登录、TOTP、发码、注册、兑换和测活。官方开启人机验证的部署还需使用目标域名验证其配置的服务商。

可用一次性的官方测试实例运行 `go test -run '^TestOfficialCompatibility$' -v`。通过 `PANEL_TEST_OFFICIAL` 指定 API 源地址，通过 `PANEL_TEST_ADMIN_FILE` 指定仅供测试的 `{ "email": "…", "password": "…" }` 文件。使用已完成官方管理员确认、尚未启用 TOTP 的测试账号。此用例会修改测试实例的注册设置并创建测试用户、管理密钥和账号。

## Windows 本地预览

构建前端后，从仓库根目录运行下列命令。测试夹具使用模拟上游和临时数据库。预览账号为 `admin@example.test`、`user@example.test` 和 `other@example.test`，测试密码均为 `fixture-password`，主管理员的验证码为 `123456`。

```powershell
$env:PANEL_TEST_BROWSER_ADDR = '127.0.0.1:28741'
go test -run '^TestBrowserFixture$' -v -timeout 0
```

## 上游版本与源码来源

当前 API 基线为官方 `v0.2.23`。接口、权限和升级复验项见 [Sub2API 兼容性调查](docs/sub2api-compatibility.md)。

本仓库以原 `panel/` 工作树为初始快照，起点是面板提交 `b3d4879b1`，同时迁入其后的用量与界面改版。根目录保留原 Sub2API 工作树的 LGPL-3.0 许可证。从 Weir 复用的 UI 组件保留 [CodedThemes MIT 声明](web/src/ui/LICENSE)。 [官方 Sub2API](https://github.com/Wei-Shaw/sub2api)
