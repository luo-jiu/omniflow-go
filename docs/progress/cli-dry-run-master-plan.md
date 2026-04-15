# CLI `--dry-run` 契约与进度摘要

更新时间：2026-04-15  
状态：核心代码已完成，端到端联调与观测收口待持续验证  
适用范围：`omniflow-go` 后端写接口与 `of` CLI 写命令

## 1. 当前结论

`--dry-run` 已成为 OmniFlow 写链路的正式契约，不是临时调试开关。

当前已落地：

- CLI 写命令支持 `--dry-run` 并透传 `dryRun=true`。
- 后端 Node、Library、User、Tag、Auth logout 等写链路已接入 `dryRun`。
- User 头像 dry-run 只做校验与模拟返回，不触发对象存储写入。
- `auth logout` 支持 dry-run，避免 CLI 误触发真实注销。

后续重点不再是“批次改造”，而是新增写链路时持续遵守同一契约，并补齐端到端联调、日志和审计验证。

## 2. 统一契约

### CLI 契约

- 所有会产生持久化副作用的 CLI 写命令默认支持 `--dry-run`。
- `--dry-run` 必须透传为 HTTP query：`dryRun=true`。
- 命令仍执行参数校验、路径解析、本地会话校验。
- 支持机器消费的命令必须同时支持 `--json`。
- CLI 不得自行模拟业务结果，后端是 dry-run 的权威来源。

### HTTP 契约

- 写接口统一接收 `dryRun`。
- 不传 `dryRun` 或 `dryRun=false` 时保持真实执行语义。
- `dryRun=true` 时执行业务校验与计划生成，但不提交持久化副作用。
- 响应外壳仍保持 `code/message/data/request_id`。

建议写接口在 `data` 中返回：

- `dryRun`
- `wouldChange`
- `summary`
- `actions`
- `warnings`
- `blockedBy`
- `codes`

### 后端实现契约

- `dry-run` 与真实执行共用同一套业务校验链路。
- 差异只能出现在事务提交、外部副作用执行或最终落库阶段。
- 事务边界放在 usecase。
- MinIO、Redis、异步任务等外部副作用在 dry-run 下不得真实执行。
- 日志和审计必须能区分 `mode=dry-run` 与 `mode=execute`。

## 3. 覆盖范围

已纳入 dry-run 的写链路类别：

| 类别 | 范围 | 状态 |
|---|---|---|
| Node | 创建、移动、重命名、删除、恢复、彻删、批量归档、排序等 | 已接入 |
| Library | 创建、更新、删除 | 已接入 |
| User | 资料更新、密码更新、头像上传 | 已接入 |
| Tag | 创建、更新、删除 | 已接入 |
| Auth | logout | 已接入 |
| Browser | browser-map、browser-bookmark 写命令 | 已接入 CLI dry-run，后端语义随对应接口维护 |

纯读接口不需要引入 dry-run。

## 4. 验收门禁

新增或修改写链路时，必须满足：

1. CLI 支持 `--dry-run`，并透传 `dryRun=true`。
2. 后端 dry-run 与真实执行校验结果一致，仅副作用不同。
3. dry-run 后数据库、MinIO、Redis、异步任务无真实变更。
4. 响应、日志、审计能明确显示 dry-run 模式。
5. 测试覆盖成功、冲突、权限拒绝、无变更中的关键路径。

## 5. 待验证项

以下事项不是批次开发任务，而是发布或联调前的质量门禁：

- 使用真实会话跑核心写命令：先 dry-run，再真实执行，校验结果一致性。
- 验证副作用隔离：dry-run 后 DB、MinIO、Redis 无变化。
- 抽样检查日志和审计字段是否包含 `mode=dry-run`。
- 补充关键命令的 CLI `--json` 快照或等价契约测试。

## 6. 维护规则

出现以下情况必须更新本文档：

- 新增写接口或写命令。
- dry-run 响应结构、字段语义或错误语义发生变化。
- 外部副作用类型新增，例如消息队列、搜索索引、第三方回调。
- 端到端联调发现 dry-run 与真实执行存在语义分叉。
