# OmniFlow CLI 开发计划与进度台账

更新时间：2026-04-09  
维护方式：每次 CLI 开发完成后，必须同步更新本文件（作为唯一进度基线）。

关联文档：
- `cmd/cli/README.md`（CLI 开发规范）
- `docs/architecture/cli-agent-development-playbook.md`（Agent 执行手册）
- `docs/architecture/cli-minimal-quickstart.md`（CLI 快速使用）
- `docs/architecture/layered-structure-spec.md`（分层约束）

## 1. 目标

- 打造可长期维护的 CLI，不做一次性脚本集合。
- 保持与现有 HTTP 契约一致，优先复用后端稳定能力。
- 满足三类场景：
  - 开发者本地操作
  - 自动化脚本调用（可机器读取）
  - 后续 AI 工具调用（命令稳定、可组合）

## 2. 里程碑计划

| 里程碑 | 内容 | 状态 | 说明 |
|---|---|---|---|
| M1 | CLI 基础框架（薄入口 + 命令树 + 模块拆分） | 已完成 | `cmd/cli` + `internal/transport/cli` 已落地。 |
| M2 | 最小可用命令（health/auth/lib/fs/config） | 已完成 | 支持 `--json`，可登录并执行查询。 |
| M3 | CLI 契约加固（退出码/参数严格/帮助输出/配置恢复） | 已完成 | 已修复 review 发现项并补测试。 |
| M4 | 文件系统写操作（mkdir/rename/mv/rm/recycle） | 已完成 | 已支持删除到回收站、回收站查看、恢复、彻删。 |
| M5 | 路径体验层（path -> nodeId） | 进行中 | 已新增 `of fs path resolve`，后续扩展路径化操作。 |
| M6 | 发布与安装（version/goreleaser/多平台） | 待开始 | 提升团队可用性与分发效率。 |
| M7 | RAG 命令域（kb ingest/search/reindex） | 待开始 | 服务知识库运维与 AI 集成。 |

## 3. 当前命令覆盖

### 3.1 已实现

- `of health`
- `of auth login`
- `of auth status`
- `of auth whoami`
- `of auth logout`
- `of lib ls`
- `of fs mkdir`
- `of fs rename`
- `of fs mv`
- `of fs rm`
- `of fs ls`
- `of fs search`
- `of fs archive batch-set-built-in-type`
- `of fs recycle ls`
- `of fs recycle restore`
- `of fs recycle hard`
- `of fs path resolve`
- `of config show`

### 3.2 计划中（优先级顺序）

1. `of fs cp` / `of fs put`（为自动化与 AI 场景准备）
2. 回收站路径语义设计（需要后端补充 deleted 节点路径解析能力）

说明：`fs mkdir`、`fs rename`、`fs mv`、`fs rm`、`fs recycle` 已完成，下一阶段进入路径体验层。

## 4. 质量门禁（每次迭代必过）

1. 通过：`go test ./...`
2. 构建：`go build -o ./bin/of ./cmd/cli`
3. 手工验证最小链路：
   - `./bin/of --help`
   - `./bin/of auth status`
   - 至少 1 条新增命令的成功/失败路径
4. 同步更新本台账和相关命令文档。

## 5. 本轮已完成事项（2026-04-09）

- 完成 CLI 架构重构：
  - 入口薄化：`cmd/cli/main.go`
  - 命令模块化：`internal/transport/cli/command_*.go`
  - 路由与帮助：`internal/transport/cli/app.go`
- 完成评审问题修复：
  - `auth status` 未登录返回非零退出码。
  - 叶子命令拒绝多余位置参数。
  - help 成功输出走 stdout。
  - 配置损坏时回落默认配置，支持自恢复。
- 完成 M4 第一项：
  - 新增 `of fs mkdir`（调用 `POST /api/v1/nodes`，目录类型）。
- 完成 M4 第二项：
  - 新增 `of fs rename`（调用 `PATCH /api/v1/nodes/{nodeId}/rename`）。
- 完成 M4 第三项：
  - 新增 `of fs mv`（调用 `PATCH /api/v1/nodes/{nodeId}/move`）。
- 完成 M4 第四项：
  - 新增 `of fs rm`（调用 `DELETE /api/v1/nodes/{nodeId}/library/{libraryId}`）。
- 完成 M4 第五项：
  - 新增 `of fs recycle ls|restore|hard`（调用回收站查询/恢复/彻删 API）。
- 完成 M5 第一项：
  - 新增 `of fs path resolve`（通过根节点+逐层 children 解析路径到 nodeId）。
- 完成 M5 第二项（阶段 1）：
  - `fs mkdir/mv/rm` 已支持可选路径参数（兼容既有 id 参数）。
- 完成 fs 归档批量能力同步：
  - 新增 `of fs archive batch-set-built-in-type`（调用 `PATCH /api/v1/nodes/{nodeId}/archive/built-in-type/batch-set`，支持 `--dry-run`、`--json`）。
- M5 约束修正：
  - `fs recycle restore/hard` 暂不支持路径参数，避免与回收站语义冲突（仅支持 `--node-id`）。
- help 渐进式披露升级：
  - 单命令 help 默认显示 usage + flags。
  - 可通过 `--examples` 查看命令示例。
- 补充单测：
  - `app_test.go`
  - `helpers_test.go`
  - `config_test.go`

## 6. 下一步开发任务（建议直接执行）

1. 为 `cp/put` 设计可复用路径输入层
2. 与后端协作补回收站路径解析 API 后，再评估 `recycle --path`

## 7. 更新规则（团队协作约定）

- 规则 1：新增/变更命令时，先改代码再更新本台账。
- 规则 2：`里程碑状态`、`当前命令覆盖`、`下一步任务` 三块必须同步。
- 规则 3：若计划优先级变化，先改本文件再开始编码。
- 规则 4：所有“完成”项需可被命令和测试证明。
