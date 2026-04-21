# CLI 开发计划与进度台账

更新时间：2026-04-15  
状态：M1-M4 已完成，M5 路径体验层进行中

关联文档：

- `cmd/cli/README.md`：CLI 开发规范
- `docs/architecture/cli-agent-development-playbook.md`：Agent 执行手册
- `docs/architecture/cli-minimal-quickstart.md`：CLI 快速使用
- `docs/progress/cli-dry-run-master-plan.md`：dry-run 契约

## 1. 当前结论

`of` CLI 已从最小可用进入持续扩展阶段。当前重点不是重做框架，而是在保持命令契约稳定的前提下补齐路径体验、发布安装和未来知识库命令域。

## 2. 里程碑

| 里程碑 | 内容 | 状态 | 当前结论 |
|---|---|---|---|
| M1 | CLI 基础框架 | 已完成 | 薄入口、命令树、模块拆分已落地 |
| M2 | 最小可用命令 | 已完成 | health/auth/lib/fs/browser/config 可用 |
| M3 | CLI 契约加固 | 已完成 | 退出码、参数严格、help、配置恢复已收口 |
| M4 | 文件系统写操作 | 已完成 | mkdir/rename/mv/rm/recycle 已支持，mkdir 已补 `--conflict-policy` |
| M5 | 路径体验层 | 进行中 | 已有 `fs path resolve`，部分写命令支持 path 输入 |
| M6 | 发布与安装 | 待开始 | version、goreleaser、多平台分发未开始 |
| M7 | RAG 命令域 | 待开始 | `kb ingest/search/reindex` 未开始 |

## 3. 命令域覆盖

| 命令域 | 覆盖情况 |
|---|---|
| `health` | 健康检查 |
| `auth` | login/status/whoami/logout |
| `lib` | 资料库列表 |
| `fs` | mkdir/rename/mv/rm/ls/search/archive/recycle/path resolve（mkdir 支持 `--conflict-policy`） |
| `browser-map` | ls/resolve/create/update/rm |
| `browser-bookmark` | tree/match/import/create/update/move/rm |
| `config` | show |

完整命令示例见 `docs/architecture/cli-minimal-quickstart.md`。

## 4. 保留规则

- CLI 是正式入口，不是临时脚本。
- 命令采用 `of <domain> <action>` 风格。
- flag 使用 kebab-case。
- 禁止隐式位置参数。
- 写命令默认支持 `--dry-run`。
- 支持机器消费的命令必须支持 `--json`。
- CLI 不得私自改写后端 `code/message/data/request_id` 语义。
- `id/path` 双模式必须互斥、必填、错误清晰。

## 5. 下一步任务

按当前优先级：

1. 继续完善路径输入层，评估 `fs cp` / `fs put` 的命令设计。
2. 与后端协作补回收站路径解析能力后，再评估 `recycle --path`。
3. 设计 `of version` 与发布安装流程。
4. RAG 命令域开始前，先补对应后端能力和文档契约。

## 6. 质量门禁

每次 CLI 迭代至少验证：

```bash
go test ./...
go build -o ./bin/of ./cmd/cli
./bin/of --help
```

新增命令还必须覆盖：

- 至少 1 条成功路径。
- 至少 1 条参数失败路径。
- `client_test.go` 中关键 path、query、header、body 契约。
- 相关 README、quickstart、进度台账同步更新。

## 7. 维护规则

- 新增或变更命令后，更新本文档的里程碑、命令域覆盖和下一步任务。
- 已完成的批次不再追加流水账，只保留当前结论。
- 若计划优先级变化，先改本文档再开始编码。
