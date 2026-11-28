# 面板发布与生产升级

## 版本和发布

预发布标签也受前两段的约束。正式版发布前沿用 `0.1` 系列。之后以最近正式版的前两段为准，切换 RC 系列同样需由项目所有者明确指定完整目标版本。

正式版展示 `M.N.PPPP`。第一段至多一位，第二段至多两位，第三段固定四位且从 `0001` 开始。首个正式版固定为 `0.1.0001`，标签为 `v0.1.1`。常规版本只递增末段一次。例如 `0.1.0009` → `0.1.0010`，标签 `v0.1.9` → `v0.1.10`。末段达到 `9999` 后等待项目所有者指定下一版本。前两段变更时同样由项目所有者明确指定完整目标版本，正式发布流程中的 `approved_version_target` 填这个目标版本。现阶段继续使用 `v0.1.0-rc.N`，正式标签、发布和切生产需单独授权。

发布前在 GitHub 仓库启用 [Release immutability](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository)，为 `panel-release` 环境配置发布审核。获得发布授权后创建符合规则的标签，对同一 commit 手动运行 **Prepare release draft**，填写已有 Git 标签和 JSON 清单。工作流核对已公开的正式发布及 tag 所指 commit，构建 GoReleaser 产物并上传到草稿。发布人核对平台包、`checksums.txt`、`update-manifest.json` 和发布说明后单独发布。已发布产物由 GitHub 生成不可变发布证明。先用本地快照开展构建验证，快照无需正式标签。

升级清单字段如下，所列版本只是演练示例。

```json
{
  "version": "0.1.0001",
  "tag": "v0.1.1",
  "upgrade_from": ["0.1.0-rc.3"],
  "migration": "apply",
  "binary_rollback": true
}
```

`upgrade_from` 只列经过回归的来源构建。若需要从较老版本升级，单独验证该来源并随 Release 声明。`none` 表示数据库结构与权限无变化，`apply` 由执行器运行迁移和授权，`manual` 要求运维介入。只有清单中的所有来源版本都已在完成目标迁移的数据库上验证，才可声明 `binary_rollback: true`。对迁移状态未知的故障，一律交给运维确认数据库状态。

## 首次安装 Linux systemd

需要 Linux amd64/arm64、systemd、Caddy、与数据库版本匹配的 PostgreSQL 客户端，以及支持 `gh release verify-asset` 的 GitHub CLI。部署前校验对应平台压缩包的发布证明及 SHA-256。首次安装和替换更新执行器使用同一个已验证的面板构建。执行器独立安装，常规面板更新只切换应用目录。

1. 创建系统组 `panel-update-client` 和独立系统用户/组 `panel-serve`、`panel-issuer`、`panel-migrate`。更新 socket 归 `root:panel-update-client`，只有 `serve` 才通过 unit 加入该组。将 `deploy/systemd/` 中的 unit 以 0644 安装到 `/etc/systemd/system/`。文件和应用目录由 root 拥有，应用用户具备读取和执行权限。
2. 把同一安装包中的 `sub2api-panel` 和 `postgres-grants.sql` 安装到 `/opt/sub2api-panel/releases/<产品版本>/`，让相对符号链接 `current` 和 `pending` 指向 `releases/<产品版本>`。后续迁移使用目标版本自带的授权文件。将执行器二进制安装为 `/opt/sub2api-panel/executor/sub2api-panel`。创建面板数据目录 `/var/lib/sub2api-panel`，所有者为 `panel-serve`。维护标记目录 `/var/lib/sub2api-panel-maintenance` 设为 root 拥有、0755。已有 RC 实例先备份数据库及当前部署配置，并在换服务管理方式前停止旧面板的两个进程。初始化后，两进程使用同一面板数据库。
3. 在 `/etc/sub2api-panel/` 中创建下列 root 所有、0600 的环境文件。迁移和备份使用 libpq 环境参数，凭据文件权限限制到对应服务用户，连接到同一个面板数据库。数据库运行角色和 DDL 所有者事先按 [角色边界](postgres-roles.md) 配置。

| 文件 | 内容 |
| --- | --- |
| `serve.env` | README 的 `serve` 变量。使用 `panel_serve` 连接，`PANEL_DATA_DIR=/var/lib/sub2api-panel` |
| `issuer.env` | README 的 `issuer` 变量。使用 `panel_issuer` 连接，官方管理 Key 只在本文件 |
| `updater.env` | 与两个应用服务相同的 `PANEL_ADDR`、`PANEL_ISSUER_ADDR`。生产默认 `PANEL_UPDATE_CHANNEL=stable`，预发布可设 `rc`。按 GitHub CLI 要求提供仅有仓库读取权限的 `GH_TOKEN` |
| `migrate.env` | 同一个迁移角色的 `PANEL_DATABASE_URL` 和供 psql 使用的 `PGHOST`、`PGPORT`、`PGDATABASE`、`PGUSER`、`PGPASSFILE`，按数据库要求配置 TLS |
| `backup.env` | 面板库备份角色的 libpq 连接参数。该账号可读整座面板数据库，并能完整导出 schema、数据和对象权限 |

4. 执行 `systemctl daemon-reload`，为新安装或首次切换创建数据库备份后运行 `systemctl start sub2api-panel-migrate.service`，并确认迁移和授权都成功。启动面板。全新数据库先启动 `serve`，通过原有首次设置确认官方网关，再启动 `issuer`。验收两个应用进程后启用 `sub2api-panel-serve.service`、`sub2api-panel-issuer.service` 和 `sub2api-panel-updater.service`，并启用 `sub2api-panel-updater.socket`。如果原地由已发布的 RC 迁移，部署侧负责这次安装，已有稳定版以后可从面板发起。
5. 按 `deploy/Caddyfile.example` 配置面板独立 HTTPS 域名。将 `/internal/*` 留在回环地址，维护窗口回 503。确认登录、TOTP、发码和用量，并从两个回环入口检查运行版本。官方 Sub2API 的反代、数据库和服务由现有部署维护。

## 更新与恢复

日常在「设置 → 系统信息 → 面板版本」检查发布并确认升级。执行器先验包，再启用维护标记，协调停服、备份、迁移、版本切换和同版本就绪检查。数据库备份在 `/var/lib/sub2api-panel-backup/<任务 ID>.dump`。成功后刷新页面并重新登录，查看最近一次任务结果。将备份纳入既有备份保留计划，并定期演练 `pg_restore` 到隔离的 PostgreSQL。

失败时优先查看 `journalctl -u sub2api-panel-updater`，再查看备份或迁移 unit 的日志。最近一次任务的状态位于 `/var/lib/sub2api-panel-updater/latest-job.json`。若标为 `intervention`，先停执行器和 socket，保留任务记录、数据库备份和当前版本。核对任务阶段与 PostgreSQL 当前状态。选择完成目标版本的迁移后启动，或把升级前备份恢复到隔离数据库，确认后按现有恢复流程切到旧版和恢复后的数据库。恢复完成后，归档任务记录，移除维护标记并恢复应用，确认两个进程健康后重启更新执行器与 socket。保留仍需审计的版本和备份。

每次更新部署能力时，在与生产相同的 systemd 环境演练 RC.3 → 首版 → 下一常规版本。覆盖下载校验、角色权限、数据库恢复、服务失败和执行器中断。生产正式发布前，用带目标迁移与真实反代的环境完成一次全流程回归。
