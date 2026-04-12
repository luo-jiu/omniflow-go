# OmniFlow Go Backend Review Standard

更新时间：2026-04-12

适用范围：`omniflow-go` 后端代码评审与改造评估。  
目标：用一份短文统一 API、分层、CLI、dry-run、日志与文档门禁，减少“各文档都对，但落地判断不一致”。

来源基线：
- `docs/architecture/layered-structure-spec.md`
- `docs/logging.md`
- `cmd/cli/README.md`
- `docs/architecture/cli-agent-development-playbook.md`
- `docs/architecture/cli-minimal-quickstart.md`
- `docs/progress/cli-development-plan-status.md`
- `docs/progress/cli-dry-run-master-plan.md`

## 1. Review 目标顺序

每次 review 按这个顺序判断，前一层不过，后一层不算过：

1. 契约是否保持稳定
2. 分层与职责是否干净
3. 写链路与 `dry-run` 语义是否一致
4. CLI 是否与后端能力同步
5. 可观测性、测试、文档是否收口

## 2. 契约稳定性

必须保持前端/调用方黑盒契约稳定，除非需求明确要求变更：

- `Method + Path` 不随意变化
- `query/body/data` 字段名与大小写保持一致
- 响应外壳保持 `code/message/data/request_id`
- 错误语义保持一致，尤其是 `401/403/404/409`
- 不允许把“暂不可用能力”先写进 help、文档或 CLI

如果一个改动让 UI、CLI、脚本三者中的任意一个行为漂移，优先判为高风险。

## 3. 分层与职责边界

必须满足单向依赖：

`transport -> usecase -> domain(port) <- repository(impl)`

Review 时重点检查：

- `transport`
  - 只做参数绑定、响应封装、错误码映射
  - 不写业务规则，不拼接数据库逻辑
- `usecase`
  - 只做业务编排、权限校验、审计、事务边界
  - 不直接操作 SQL、Redis、MinIO 客户端
- `repository`
  - 收敛 SQL、缓存 key、TTL、对象存储细节
  - 不反向依赖 `transport`
- `domain`
  - 不出现 GORM、Redis、MinIO 等技术实现依赖

出现以下情况，默认视为分层泄漏：

- handler 里出现业务分支或 repository 细节
- usecase 直接拼 SQL / 操作 Redis / 操作对象存储 SDK
- repository 开始承担业务编排或权限判断

## 4. 结构与拆分标准

优先小步、最小必要改造，不为“看起来整齐”而拆。

建议标准：

- 单文件超过 `400` 行，或单 package 超过 `8` 个文件且职责散乱，才考虑继续拆
- package 内文件数建议控制在 `3-7` 个核心文件
- 事务边界统一在 `usecase`
- Raw SQL 只用于递归、复杂聚合等必要场景，并集中收口

不要为了抽象而抽象。  
新增抽象只有在以下情况才合理：

- 消除重复业务规则
- 避免跨层实现泄漏
- 明确收紧 mutation/query 边界

## 5. 写链路与 `dry-run`

所有会产生持久化副作用的后端写链路，都要按“真实校验、禁止提交”的原则 review：

- handler 正确解析 `dryRun`
- usecase command 带 `DryRun`
- `dry-run` 与真实执行共用同一业务校验链路
- 差异只能出现在“提交事务/执行外部副作用”阶段
- 响应头、日志、审计能区分 `dry-run` 与真实执行

常见高风险问题：

- `dry-run` 走了另一套简化逻辑
- 只回滚数据库，但仍真实写了 MinIO / Redis / 异步任务
- HTTP 支持了 `dry-run`，CLI 却没透传或 help 没同步

## 6. CLI 一致性标准

CLI 是后端能力的正式入口，不是临时脚本。

凡是新增或变更后端可操作能力，review 时必须检查 CLI 是否同步覆盖：

- `app.go` 命令树、usage、flags、examples
- `command_*.go` 命令实现
- `client.go` HTTP 封装
- `app_test.go` 至少一条成功/可替代验证和一条参数失败路径
- `client_test.go` 至少覆盖关键 query/header/路径契约
- `cmd/cli/README.md`
- `docs/architecture/cli-minimal-quickstart.md`
- `docs/progress/cli-development-plan-status.md`

CLI 硬规则：

- 写命令默认支持 `--dry-run`
- 支持机器消费的命令必须支持 `--json`
- 禁止隐式位置参数
- `id/path` 双模式必须做到互斥、必填、错误清晰
- CLI 不得私自改写后端错误语义

如果 API 已有能力但 CLI 不可达，这不算完整交付。

## 7. `lo` 使用标准

`lo` 可以用，但只能用在“更清楚”，不能用在“更花”。

推荐使用场景：

- `Map` / `Filter` / `Uniq` / `SliceToMap` 这类明确的集合转换
- 替代样板化、无业务含义的循环

不推荐使用场景：

- 隐藏副作用
- 把复杂业务判断塞进匿名函数导致可读性下降
- 只是为了“全项目统一 lo”而替换本来更直白的 Go 原生代码

判断标准只有一个：  
同事第一次读到这里，是否更快看懂真实业务意图。

## 8. 日志、审计与错误

后端改动必须保证可观测性不倒退：

- 请求日志保留 `actor/error/dry-run`
- usecase 写链路补业务日志
- 审计日志区分 `mode=dry-run` 与 `mode=execute`
- 不因为本地调试方便而把默认日志策略改成长期不合适的状态

日志配置要符合：

- `debug` 本地可读
- `release` 默认 JSON 到 stdout
- SQL debug 只在明确需要时开启

任何与本次需求无关的全局日志配置改动，默认都应谨慎对待。

## 9. 测试与文档门禁

后端 review 不只看“能跑”，还要看“以后还能不能放心改”。

最低门禁：

1. `gofmt`
2. `go test ./...`
3. CLI 改动时：`go build -o ./bin/of ./cmd/cli`
4. 至少一条新增能力的失败路径验证
5. 相关文档同步更新

文档只更新真正受影响的文件，不做无关台账 churn。

## 10. Review 输出格式

Review 结论按这个顺序写：

1. Findings，按严重度排序
2. Open questions / assumptions
3. Change summary

Findings 优先关注：

- 行为回归
- 边界不清
- 跨层泄漏
- `dry-run`/CLI 漏洞
- 测试缺口

如果没有问题，要明确写“未发现问题”，并补一句残余风险或未验证项。

## 11. 一票否决项

出现以下任一情况，默认不建议合并：

- 破坏黑盒契约且未明确说明
- `dry-run` 与真实逻辑分叉
- HTTP 能力新增但 CLI 与文档完全未同步
- usecase / handler / repository 明显跨层
- 用配置或日志临时改动掩盖真实问题

这份文档追求的是“长期可维护的判断标准”，不是一次性 checklist。  
结论优先服务于稳定边界、最小必要改造和后续可持续演进。
