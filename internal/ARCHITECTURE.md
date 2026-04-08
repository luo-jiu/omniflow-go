# Architecture

`omniflow-go` 使用轻量领域化分层，而不是把 HTTP handler 当作业务核心。

## Layers

- `internal/domain`
  领域对象与稳定概念，例如 `user`、`library`、`node`
- `internal/usecase`
  业务动作与查询入口，是未来 HTTP、MCP、Agent 共用的能力边界
- `internal/transport/http`
  `gin` 适配层，只负责协议转换
- `internal/transport/mcp`
  未来 MCP tool adapter 的保留位置
- `internal/repository`
  数据持久化
- `internal/storage`
  对象存储等外部能力
- `internal/actor`
  执行者模型，支持用户、系统、Agent、集成身份
- `internal/authz`
  授权模型
- `internal/audit`
  审计能力

## Why

这样可以让以下几种入口共享同一套业务能力：

- 前端调用 HTTP API
- MCP tools
- Skills 编排后的 Agent 调用
- 后台任务或系统脚本

统一原则：

- `transport` 不能直接实现业务规则
- `usecase` 不依赖 `gin.Context`
- Agent/MCP 调用 `usecase`，不直接操作 repository
