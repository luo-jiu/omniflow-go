## 项目说明

`omniflow-go` 是 OmniFlow 的 Go 后端。项目核心目标是：在保持前端、CLI、脚本调用方对外契约稳定的前提下，持续沉淀清晰、可测试、可复用的后端能力。

### 入口

- HTTP 服务入口：`cmd/server/main.go`
- HTTP 路由定义：`internal/transport/http/router/router.go` 与 `internal/transport/http/router/routes_*.go`
- HTTP handler：`internal/transport/http/handler/*.go`
- CLI 入口：`cmd/cli/main.go`
- CLI 命令树：`internal/transport/cli/app.go`
- CLI 命令实现：`internal/transport/cli/command_*.go`
- CLI HTTP 客户端：`internal/transport/cli/client.go`
- 业务编排：`internal/usecase/*.go`
- 领域对象与端口：`internal/domain/**`
- 仓储实现：`internal/repository/**`
- 依赖注入：`internal/bootstrap/wire.go` 与 `internal/bootstrap/wire_gen.go`
- 默认配置：`configs/config.yaml`

### 权威文档

`docs/` 与关键 README 是本项目的**外部长期记忆**。Agent 在修改相关代码前必须优先查阅对应文档，不能只凭代码变量名或直觉推断业务规则。

- 后端总审查标准：`docs/architecture/backend-review-standard.md`
- 文档规范：`docs/architecture/documentation-standard.md`
- 分层与目录规范：`docs/architecture/layered-structure-spec.md`
- internal 目录速览：`docs/architecture/internal-structure-overview.md`
- 日志规范：`docs/logging.md`
- CLI 开发规范：`cmd/cli/README.md`
- CLI 作战手册：`docs/architecture/cli-agent-development-playbook.md`
- CLI 最小使用文档：`docs/architecture/cli-minimal-quickstart.md`
- Go 后端交接：`docs/architecture/go-backend-handoff.md`
- Go API 契约状态：`docs/progress/go-api-contract-status.md`
- CLI 进度台账：`docs/progress/cli-development-plan-status.md`
- dry-run 契约与进度：`docs/progress/cli-dry-run-master-plan.md`

## 项目规范

> 所有 agent 都需要遵守。若本文件与更细的专题文档冲突，以专题文档为准，并在必要时同步修正过期文档。

### Review 必读规范

- 用户要求 review、代码审查、评审、检查改动、找风险时，必须先阅读并严格按 `docs/architecture/backend-review-standard.md` 执行。
- Review 结论必须以 findings 为先，优先关注行为回归、契约漂移、分层泄漏、`dry-run`/CLI 漏洞、日志审计缺口和测试缺口。
- 不得只做总结式 review；没有发现问题时，也必须明确写“未发现问题”，并说明残余风险或未验证项。
- 后端 review 中发现的通用规则缺口，应优先补充到 `docs/architecture/backend-review-standard.md`，不要只写在一次性回复里。

### 开发规范（含知识准备）

- **业务前置要求**：开始编写涉及接口行为、业务流程、权限、状态、写链路、对象存储或 CLI 的代码前，必须先阅读相关文档。
- 修改对外接口时，优先保证 Go 当前契约稳定：
  1. `Method + Path` 不随意变化
  2. query/body 字段名与大小写保持一致
  3. 响应外壳保持 `code/message/data/request_id`
  4. 错误语义保持一致，尤其是 `401/403/404/409`
- 按照项目现有轻量分层开发，严格保持依赖方向：

```text
transport -> usecase -> domain(port) <- repository(impl)
```

- `transport/http` 只做请求绑定、参数校验、响应封装、错误码映射，不写业务规则。
- `transport/cli` 只做命令解析、HTTP 调用、输出格式化，不绕过 HTTP API 直接操作 repository。
- `usecase` 只做业务编排、权限校验、事务边界、审计、`dry-run` 控制，不直接操作 SQL、Redis、MinIO 客户端。
- `domain` 放领域对象、值对象、领域错误、端口接口，不依赖 GORM、Redis、MinIO、Gin 等技术实现。
- `repository` 收敛 SQL、GORM query、缓存 key、TTL、对象存储细节，不反向依赖 transport。
- 新增 PG/Redis/MinIO 实现优先放入 `repository/postgres`、`repository/redis`、`repository/object` 对应目录。
- 优先小步、最小必要改造；不要为了“看起来整齐”扩大改动范围。
- 单文件超过 `400` 行，或单 package 超过 `8` 个文件且职责散乱时，再考虑继续拆分。
- 能复用已有方法就复用；但复用会明显增加数据库查询次数或模糊职责时，可以新增更合适的方法。
- 接口入参尽量定义成结构体，并使用 Gin 绑定与校验能力。
- 善用早返回，不应该继续执行的逻辑及时结束。
- 行内代码尽量不超过 `120` 个字符，超过时按 Go 最佳实践换行。
- `lo` 可以使用，但只在集合转换更清楚时使用，不为了“统一”替换更直白的 Go 原生代码。
- 结构化日志优先使用稳定字段，避免拼接不可检索的大字符串。

### dry-run 规范

所有会产生持久化副作用的写链路，都必须按“真实校验、禁止提交”的原则实现和 review：

- handler 正确解析 `dryRun` 或等价参数。
- usecase command 显式携带 `DryRun` 或 `MutationMode`。
- `dry-run` 与真实执行共用同一套业务校验链路。
- 差异只能出现在提交事务、执行外部副作用或最终落库阶段。
- 日志、审计、响应能区分 `mode=dry-run` 与 `mode=execute`。
- CLI 写命令默认支持 `--dry-run`，并透传给后端。

禁止：

- `dry-run` 走简化逻辑，真实执行走另一套逻辑。
- 只回滚数据库，但仍真实写入 MinIO、Redis、异步任务或外部系统。
- HTTP 支持 `dry-run`，CLI help、README 或命令实现没有同步。

### CLI 规范

CLI 是后端能力的正式入口，不是临时脚本。新增或变更后端可操作能力时，必须检查 CLI 是否同步覆盖：

- `internal/transport/cli/app.go` 命令树、usage、flags、examples
- `internal/transport/cli/command_*.go` 命令实现
- `internal/transport/cli/client.go` HTTP 封装
- `internal/transport/cli/app_test.go`
- `internal/transport/cli/client_test.go`
- `cmd/cli/README.md`
- `docs/architecture/cli-minimal-quickstart.md`
- `docs/progress/cli-development-plan-status.md`

CLI 硬规则：

- 命令采用 `of <domain> <action>` 风格。
- flag 使用 kebab-case，例如 `--library-id`、`--base-url`。
- 禁止隐式位置参数，未声明的多余参数必须失败。
- 写命令默认支持 `--dry-run`。
- 支持机器消费的命令必须支持 `--json`。
- `id/path` 双模式必须互斥、必填、错误清晰。
- CLI 不得私自改写后端 `code/message/data/request_id` 语义。

### 注释规范

- 用中文写业务注释。
- 符合 Go 最佳实践：导出的类型、函数、方法必须有注释。
- 中文和英文、数字混合时，英文和数字两侧保留空格。
- 函数内复杂业务逻辑可以用 `1. 2.` 这样的序列写关键步骤注释。
- 不写“把值赋给变量”这类无信息量注释。

### 数据访问规范

- 常规 CRUD 优先使用 GORM 链式查询或 gorm/gen 生成 query。
- Raw SQL 只用于递归、复杂聚合等必要场景，并集中收口。
- Raw SQL 必须使用参数占位符，禁止字符串拼接构造条件。
- 事务边界统一放在 `usecase`，普通仓储函数不隐式开启事务。
- 仓储函数必须接收 `context.Context`，并通过上下文复用事务。
- `internal/repository/postgres/model/*.gen.go` 与 `internal/repository/postgres/query/*.gen.go` 是生成结果，不手工编辑。
- 需要连接真实数据库验证时，优先使用本地工具 `dbgate`；本地 PostgreSQL 默认使用 `dbgate pg -k local "SQL"`，连接数据库的 `dbgate` 命令需要提权执行，写操作必须获得明确许可。

### 构建与测试规范

- 提交前必须执行 `gofmt`。
- 常规后端改动至少执行：

```bash
go test ./...
```

- CLI 改动额外执行：

```bash
go build -o ./bin/of ./cmd/cli
```

- 新增能力至少覆盖一条成功路径和一条失败路径。
- 写链路必须覆盖 `dry-run` 与真实执行的关键差异。
- handler 测试关注参数绑定、错误码映射、响应外壳。
- usecase 测试关注业务规则、权限、事务和副作用控制。
- CLI client 测试必须覆盖关键 path、query、header、body 契约。
- 如果没有特别说明，不运行会真实增删改数据库、Redis、MinIO 或外部系统的测试。
- 自己新增的测试优先使用 fake、mock、httptest、临时目录或事务回滚。

## 文档规范

> **重要提醒**：`docs/` 是 Coding Agent 的**外部长期记忆**。  
> 1. **开发前（Read）**：必须阅读相关文档，理解业务上下文、接口契约、当前状态和隐式约束。  
> 2. **开发后（Write）**：必须评估是否需要创建或更新文档，让下一位开发者和下一次 Agent 调用拥有最新事实。
>
> 详细写法、目录归属和归档规则见：`docs/architecture/documentation-standard.md`。

### 何时必须更新文档

出现以下任一情况时，必须补充或更新对应文档：

- 新增或修改对外 HTTP API（路由、请求参数、响应结构、错误语义）。
- 新增或修改 CLI 命令（命令名、flag、输出结构、错误语义、示例）。
- 新增或修改写链路、`dry-run` 语义、事务边界或外部副作用。
- 新增或修改核心业务流程、状态流转、权限规则。
- 新增或修改日志、审计、安全、配置策略。
- 新增或修改数据库表结构、字段含义、枚举值、索引或数据演进策略。
- 新增领域概念，或修改已有术语定义。
- 修复了因业务理解偏差导致的缺陷，需要澄清原有文档描述。
- 调整分层、目录结构、生成代码流程或交付计划。

### 文档位置建议

- 架构、分层、工程规则：`docs/architecture/`
- 阶段计划、交付状态、完成度台账：`docs/progress/`
- CLI 使用与开发规则：`cmd/cli/README.md` 与 `docs/architecture/cli-*.md`
- 日志配置与观测规则：`docs/logging.md`
- internal 目录职责说明：`docs/architecture/internal-structure-overview.md`

如果现有文档没有合适位置，可以在 `docs/architecture/` 或 `docs/progress/` 下创建新文档。文件名使用小写英文短横线连接，例如 `node-dry-run-contract.md`。

### 文档结构要求

新增长期文档优先包含：

1. **概述**：模块职责、边界和当前结论。
2. **背景**：为什么需要该能力，面向哪些调用方或场景。
3. **核心概念**：状态、枚举、术语、关键常量。
4. **契约**：HTTP、CLI、数据模型或 `dry-run` 契约。
5. **实现约束**：分层、事务、副作用、日志、审计、测试要求。
6. **验证方式**：自动化测试、手工验证方式和未覆盖风险。

阶段性进度文档完成后，必须改成“当前结论 + 保留规则 + 未完成或待验证 + 维护规则”的归档摘要，不长期保留大段已完成流水账。

### 文档格式与语言

- 全部使用中文撰写，专业术语可保留英文。
- 使用 Markdown，层级清晰。
- 表格对齐，代码块标明语言类型。
- 中文与英文、数字之间保留空格。
- 涉及密钥、token、密码、内网地址等敏感信息必须使用 `***` 脱敏。
- 不把大段代码直接贴入文档，用关键规则和流程描述代替。
- 不用截图替代可搜索文本。
- 不做无关台账 churn，只更新与本次变更直接相关的文档。

### Agent 开发后自检

Agent 完成一轮代码修改和测试后，必须执行以下自检：

1. 本次变更是否影响已有文档的准确性？
   - 若影响，修改相关文档以反映最新事实。
2. 本次变更是否引入了需要其他开发者、前端、CLI 使用者或后续 Agent 知道的新知识？
   - 若需要，创建新文档或补充到现有文档的合适章节。
3. 本次变更是否涉及对外 HTTP API、CLI 命令、`dry-run` 或进度台账？
   - 若是，必须更新对应接口说明、CLI 文档或进度文档。
4. 如果判断不需要更新文档，最终回复中要简短说明原因。

## Review 输出规范

用户要求 review 时，必须先阅读并遵守 `docs/architecture/backend-review-standard.md`。默认采用代码审查口径，优先输出问题而不是总结：

1. Findings，按严重度排序，包含文件与行号。
2. Open questions / assumptions。
3. Change summary。

Findings 优先关注：

- 行为回归
- 契约漂移
- 分层泄漏
- `dry-run` 或 CLI 漏洞
- 权限、审计、日志缺口
- 测试缺口

如果没有发现问题，要明确写“未发现问题”，并说明残余风险或未验证项。

## 禁止事项

- 破坏对外契约且未明确说明。
- `dry-run` 与真实执行逻辑分叉。
- HTTP 能力新增但 CLI 与文档完全未同步。
- handler、usecase、repository 明显跨层。
- usecase 直接操作 SQL、Redis、MinIO 等基础设施客户端。
- 新增写链路没有权限、审计或失败路径测试。
- 未获授权批量修改真实数据库、Redis、MinIO 或外部系统数据。
- 在日志、文档、测试快照或示例中泄露密钥、token、密码、内部地址。
