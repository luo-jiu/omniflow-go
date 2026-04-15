# Internal 目录速览

更新时间：2026-04-15

适用范围：`internal/` 下各目录职责快速定位。  
完整分层规则见：

- `docs/architecture/layered-structure-spec.md`
- `docs/architecture/go-backend-handoff.md`
- `docs/architecture/backend-review-standard.md`

## 1. 核心原则

`internal/` 使用轻量领域化分层。所有后端能力必须保持单向依赖：

```text
transport -> usecase -> domain(port) <- repository(impl)
```

基本约束：

- `transport` 只做协议适配，不写业务规则。
- `usecase` 负责业务编排、权限、事务、审计和 `dry-run`。
- `domain` 放稳定领域对象、领域错误和端口接口，不依赖基础设施。
- `repository` 封装 PostgreSQL、Redis、对象存储等实现细节。
- Agent、CLI、HTTP、后台任务等入口都应复用 `usecase` 能力，不直接操作 repository。

## 2. 目录职责

| 目录 | 职责 |
|---|---|
| `actor/` | 执行者模型，描述用户、系统、Agent、集成身份等调用主体 |
| `app/` | 应用级装配入口或顶层运行对象 |
| `audit/` | 审计能力，记录关键写链路和安全相关行为 |
| `authz/` | 授权模型与权限判断 |
| `bootstrap/` | 启动装配，包含配置加载、数据库、Redis、日志、wire 注入 |
| `config/` | 配置结构、默认值、校验和解析 |
| `domain/` | 领域对象、值对象、领域错误、端口接口 |
| `repository/` | 数据库、缓存、对象存储等端口实现 |
| `server/` | HTTP server 启动与宿主层封装 |
| `storage/` | 对象存储等外部存储能力抽象或辅助 |
| `transport/` | HTTP、CLI、MCP 等入口适配层 |
| `usecase/` | 业务动作与查询入口，承载事务、权限、审计、`dry-run` 编排 |

## 3. 入口关系

- HTTP：`transport/http -> usecase`
- CLI：`transport/cli -> HTTP API`
- MCP：`transport/mcp -> usecase`（预留入口）
- Repository：只实现 domain/usecase 所需端口，不反向依赖 transport。

新增入口或调用方式时，应先确认能否复用现有 `usecase`。除非有明确架构决策，不新增绕过 usecase 的业务通道。

## 4. 维护规则

出现以下情况时更新本文档：

- `internal/` 新增或删除一级目录。
- 某个目录职责发生变化。
- 新增入口类型，例如新的 MCP、Agent、job、worker 适配层。
- 分层规范发生变化。

本文档只做速览，不记录具体业务流程、接口契约或阶段计划；这些内容应放入 `docs/architecture/` 或 `docs/progress/`。
