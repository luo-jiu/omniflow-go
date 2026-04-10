# omniflow-go

`omniflow` Java service 的 Go 重构版本骨架。

当前阶段已完成：

- 轻量领域化目录结构
- `gin` HTTP 适配层
- `usecase` 业务动作层
- `gorm` / `redis` / `minio` 基础设施初始化
- `wire` 依赖注入装配
- 为未来 `MCP / agent / skill` 预留的扩展边界
- 可直接启动的健康检查接口

## Architecture

当前结构遵循“外层适配，内层稳定”的原则：

- `internal/domain`
  放核心业务对象定义
- `internal/usecase`
  放可被 HTTP、MCP、Agent 共同复用的业务能力
- `internal/transport/http`
  放 `gin` handler、middleware、router
- `internal/transport/mcp`
  预留未来 MCP tool adapter
- `internal/repository`
  放数据库访问
- `internal/actor` / `internal/authz` / `internal/audit`
  预留未来 Agent 权限、审计和身份能力

这样做的目标不是上很重的 DDD，而是避免把 HTTP 语义写死在业务层里。

## Run

```bash
cd /Users/loyce/personal/omniflow/omniflow-go
go run ./cmd/server -config ./configs/config.yaml
```

启动后可访问：

- `GET /healthz`
- `GET /api/v1/health`

## Logging

项目日志基于 `slog`，支持按配置切换：

- `debug` 场景可使用 `text` 格式（人读友好）
- `release` 场景建议 `json` 格式（便于采集到 ES/Loki）
- `console` 与 `file` 输出可独立开关（`text` 模式支持彩色级别）
- `database.debug_sql=true` 可在本地联调时输出 SQL
- 文件日志使用滚动切割（`max_size_mb / max_backups / max_age_days / compress`）

常见策略：

- 本地调试：`console.enabled=true`，`file.enabled=true`
- 线上部署：`console.enabled=true`，`file.enabled=false`（由平台采集 stdout）

## Wire

项目已提交 `wire_gen.go`，即使本地没有安装 `wire` 也能直接编译运行。

如果后续需要重新生成：

```bash
cd /Users/loyce/personal/omniflow/omniflow-go
wire ./internal/bootstrap
```
