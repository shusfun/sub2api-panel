# PostgreSQL 角色边界

迁移使用独立管理连接执行一次：

```sh
sub2api-panel migrate
```

生产环境把 `serve` 与 `issuer` 配置成同一数据库的不同角色 URL：

- `panel_serve`：`runtime_settings`、`usage_observations`、`usage_snapshots`、`usage_versions`、`usage_cursors`、`patrol_policies`、`probe_templates`、`patrol_runs` 及七张账本表的读写权限，具体清单见 [PostgreSQL 部署授权](../deploy/postgres-grants.sql)。不得读取 `grants`、`issues`、`grant_audits`。
- `panel_issuer`：`grants`、`issues`、`grant_audits` 和 `runtime_settings` 的读写权限；不得读取用量观测表，也不读取浏览器会话数据。
- 迁移角色：拥有 schema migration 所需的 DDL 权限，不用于运行进程。

角色权限应通过部署脚本或 DBA 变更管理维护，不在应用启动时自动授予。备份使用 PostgreSQL 原生逻辑或物理备份，恢复后先运行 `panel migrate` 校验 schema，再启动两个进程。RC 不提供 SQLite 导入、回退或双写路径。
