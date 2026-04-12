# OmniFlow CLI `--dry-run` 改造进度报告（终极计划）

更新时间：2026-04-09  
当前状态：`M5 联调准备（M1-M4 代码已完成）`  
适用范围：`omniflow-go` 后端 + `of` CLI

当前落地说明：
- CLI 已支持 `--dry-run` 并透传 `dryRun=true`。
- 后端 Node 写链路（P0）已支持 `dryRun=true` 并通过事务回滚避免落库（代码层）。
- 后端 Library 写链路（Create/Update/Delete）已支持 `dryRun=true` 并通过事务回滚避免落库（代码层）。
- 后端 User 写链路（Update/UpdatePassword/UploadAvatar）已支持 `dryRun=true`，其中头像 dry-run 仅做校验与模拟返回，不触发对象存储写入（代码层）。
- 后端 Tag 写链路（Create/Update/Delete）已支持 `dryRun=true` 并通过事务回滚避免落库（代码层）。
- `auth logout` 已提前支持 dry-run，避免 CLI 误触发真实注销。
- 需重启后端进程后再做端到端联调验证。

## 1. 背景与目标

当前 CLI 已可稳定执行读命令，但写命令对“陌生操作者/AI”仍有直接落库风险。  
本次改造目标是建立企业可用的“安全预演”能力：

- CLI 侧增加 `--dry-run`，用于触发和展示。
- 后端写接口支持 `dryRun=true`，走真实校验链路但不提交变更。
- 输出统一的“会发生什么”结果，支持人读和机读。

一句话目标：  
`CLI 负责触发与展示，后端负责权威模拟。`

## 2. 范围定义

### 2.1 本期纳入（必须）

- 所有会产生持久化副作用的 CLI 写命令。
- 对应后端写接口（HTTP mutation）。
- 审计与日志加 `mode=dry-run` 标记。
- JSON 输出统一结构（便于脚本和 AI 消费）。

### 2.2 本期不纳入（明确排除）

- 纯读接口（health/status/list/search/path 等）不改造。
- 非 CLI 调用方的 UI 展示优化（可后续单独做）。
- 新增复杂审批流（本期只做模拟，不做流程引擎）。

## 3. 统一设计（最终定版）

## 3.1 CLI 约定

- 写命令统一支持：`--dry-run`
- 命令仍执行：
  - 参数校验
  - 路径解析
  - 本地会话校验
- 最终请求透传：`dryRun=true`
- `--json` 输出保持结构化，不做破坏式变更

## 3.2 HTTP 约定

- 写接口统一接收 `dryRun`（建议 query 统一：`?dryRun=true`）
- 不传 `dryRun` 或 `dryRun=false`：保持现有行为
- `dryRun=true`：执行业务校验与计划生成，不提交事务

## 3.3 响应结构（标准）

建议在 `data` 内统一返回以下字段（读接口无需引入）：

- `dryRun: true|false`
- `wouldChange: true|false`
- `summary: string`
- `actions: []`
- `warnings: []`
- `blockedBy: []`
- `codes: []`

说明：
- `wouldChange=false` 且 `blockedBy=[]`：表示无变更（例如重复操作 no-op）。
- `blockedBy!=[]`：表示被业务规则拦截（权限、冲突、状态不满足等）。

## 4. 分层改造方案（按现有架构）

### 4.1 transport 层

- 为写路由处理函数增加 `dryRun` 解析与传递。
- 不改已有响应外壳，只扩展 `data`。

### 4.2 usecase 层（核心）

- mutation command 增加 `DryRun bool` 字段。
- 校验、权限、冲突检测与真实写路径共用同一逻辑。
- `DryRun=true` 时：
  - 生成 `actions/warnings/blockedBy`
  - 不触发真实写入提交
  - 不触发外部副作用执行

### 4.3 repository 层

- 保持现有 repository 查询/写入能力，避免大规模侵入。
- 事务边界仍在 usecase，通过 `Transactor.WithinTx` 控制提交/回滚。

### 4.4 外部副作用（对象存储/会话/消息）

- Dry-run 模式禁止真实副作用：
  - MinIO：不真实上传/删除
  - Redis：不真实写会话/缓存
  - 其他异步任务：不入队
- 仅在 `actions` 中描述计划动作。

## 5. 接口改造优先级矩阵

## 5.1 P0（先做，直接保护 CLI 高频写操作）

- `POST /api/v1/nodes`（mkdir）
- `PATCH /api/v1/nodes/{nodeId}/move`（mv）
- `PATCH /api/v1/nodes/{nodeId}/rename`（rename）
- `DELETE /api/v1/nodes/{ancestorId}/library/{libraryId}`（rm）
- `DELETE /api/v1/nodes/recycle/library/{libraryId}/clear`（recycle clear）
- `PATCH /api/v1/nodes/{ancestorId}/library/{libraryId}/restore`（recycle restore）
- `DELETE /api/v1/nodes/{ancestorId}/library/{libraryId}/hard`（recycle hard）

## 5.2 P1（第二批，补齐主要写路径）

- `POST /api/v1/libraries`
- `PUT /api/v1/libraries/{id}`
- `DELETE /api/v1/libraries/{id}`
- `PUT /api/v1/user/{id}`
- `PUT /api/v1/user/me`
- `PUT /api/v1/user/me/password`
- `POST /api/v1/user/me/avatar`

## 5.3 P2（第三批，低频但应一致）

- `POST /api/v1/tags`
- `PUT /api/v1/tags/{tagId}`
- `DELETE /api/v1/tags/{tagId}`
- `DELETE /api/v1/auth/logout`

## 6. 里程碑计划（稳扎稳打）

| 里程碑 | 目标 | 产出 | 状态 |
|---|---|---|---|
| M0 | 冻结协议与范围 | 本报告定稿 | 已完成 |
| M1 | CLI 框架接入 | 写命令支持 `--dry-run`，透传参数 | 已完成 |
| M2 | 后端 P0 支持 | 节点写链路 dry-run 完整可用 | 已完成（待联调验证） |
| M3 | 后端 P1 支持 | 库/用户写链路 dry-run 可用 | 已完成（待联调验证） |
| M4 | 后端 P2 支持 | tag/auth 写链路 dry-run 可用 | 已完成（待联调验证） |
| M5 | 联调与回归 | CLI + 后端端到端验收报告 | 未开始 |
| M6 | 收口与默认策略 | 文档、审计、监控、开关策略完成 | 未开始 |

## 7. 验收标准（Definition of Done）

满足以下条件才算完成：

1. 所有纳入的写命令支持 `--dry-run`，且无真实落库。  
2. 同一请求在 dry-run 与真实执行下，校验结果一致（仅副作用不同）。  
3. 后端返回结构统一，CLI 文本与 `--json` 都可稳定展示。  
4. 审计日志可区分 `mode=dry-run` 与 `mode=execute`。  
5. 回归测试覆盖成功、冲突、权限拒绝、无变更四类路径。  

## 8. 测试与质量门禁

### 8.1 自动化测试

- 单测：usecase dry-run 分支与计划生成逻辑
- 集成：HTTP 写接口 `dryRun=true/false` 对照测试
- CLI：参数透传与 JSON 结构快照测试

### 8.2 人工联调

- 使用真实会话跑 P0 全链路：先 dry-run，再真实执行，校验结果一致性
- 验证“副作用隔离”：dry-run 后 DB/MinIO/Redis 无变化

## 9. 风险与防护

主要风险与对应防护：

- 风险：dry-run 与真实逻辑分叉，结果不可信  
  防护：共用同一业务函数，差异仅在“提交与副作用执行点”

- 风险：遗漏某个写接口，导致行为不一致  
  防护：按接口矩阵逐项打勾，发布前跑全量清单

- 风险：误把 dry-run 当真实执行  
  防护：响应与日志强制输出 `dryRun=true`，CLI 明显提示“模拟模式”

## 10. 进度跟踪看板（执行中持续更新）

| 模块 | 子项 | 负责人 | 状态 | 备注 |
|---|---|---|---|---|
| 协议 | `dryRun` 入参与响应结构冻结 | AI + Owner | 已完成 | 本文已冻结基线 |
| CLI | 写命令统一 `--dry-run` | AI | 已完成 | 已透传 query `dryRun=true` |
| Node | P0 接口 dry-run | AI | 已完成（待联调） | 已接入 query 解析 + 事务回滚 |
| Library/User | P1 接口 dry-run | AI | 已完成（待联调） | library/user 已接入 |
| Tag/Auth | P2 接口 dry-run | AI | 已完成（待联调） | tag/auth 已接入 |
| Observability | 审计/日志模式标记 | AI | 未开始 | 必须项 |
| QA | 端到端回归报告 | AI + Owner | 未开始 | 发布前门禁 |

## 11. 执行策略（推荐）

- 采用“小批次提交”：一个接口或一组同类接口一个 commit。
- 每个批次顺序固定：`usecase -> handler -> CLI -> test -> 文档`。
- 每完成一个里程碑，更新本报告状态，避免隐性偏航。

---

最终结论：  
本方案属于企业常见的“后端权威 dry-run”实现，不是全量侵入式改造；  
仅对写链路做改造即可获得高可靠、低风险、可审计的预演能力。
