# 条件批量重命名与提交回执

更新时间：2026-09-22

## 当前结论

HTTP、CLI 与插件宿主共用条件批量改名能力。一次最多 50 个同库普通文件，仅改变主体名并保留扩展名、节点身份和存储对象。全批原子提交，任何一项无效、陈旧或冲突都会回滚；不支持目录、归档虚拟成员、交换名称、覆盖或自动追加序号。

这实现长期方针中的写入可核对原则：取消或超时不代表没有发生，成功修改与持久回执在同一 PostgreSQL 事务提交。回执不自动过期；不复用上传 session，也不依赖日志推测执行结果。

## HTTP 契约

- `POST /api/v1/nodes/rename/batch/conditional?dryRun=true|false`
- `GET /api/v1/nodes/rename/batch/status?libraryId=3&operationId=<uuid>`

请求 body 为 `{libraryId, operationId, items:[{nodeId,name,expected:{name,ext,parentId,updatedAt}}]}`。`name` 是新主体名；`expected.name` 是旧主体名。`ext` 必须显式提供，允许空字符串。`updatedAt` 来自节点详情或 metadata 查询，必须原样保留精度。节点 ID 唯一且属于指定资料库；operation ID 必须为标准小写非零 UUID。body 最大 128 KiB，拒绝未知字段、遗漏 expected 和尾随 JSON。名称不允许空白边界、路径分隔符、控制字符、`.`、`..`，最多 255 个字符。

成功 `data` 为 `{operationId,libraryId,atomic:true,state:"committed",replayed:false,items,affectedParentIds}`。每项为 `{nodeId,parentId,previous:{name,ext,updatedAt},current:{name,ext,updatedAt},status:"committed"}`，按 nodeId 排序。

`dryRun=true` 使用既有包装 `data:{dryRun:true,result:<结果>}` 和 `X-Omniflow-Dry-Run:true`。结果 state/item.status 为 `validated`，current.updatedAt 保留旧版本，不能用回滚时的临时版本继续写入。校验、节点写 SQL、回执写 SQL 与执行共用链路，最终整个事务回滚；不修改对象存储、Redis 或触发后台任务。

状态查询返回 `{operationId,libraryId,state:"committed",result:<原提交结果>}` 或 `{operationId,libraryId,state:"not_found"}`。`not_found` 只表示查询时无可见已提交回执，不排除正在进行中的请求，不能向用户显示为“已取消且未执行”。查询需要当前 actor 的资料库写权限，不能读取其他身份或其他库的回执。

错误沿用 `code/message/data/request_id`：400 参数、文件类型不支持；401 未登录；403 无库写权限；404 节点不存在、跨库或已删除；409 旧值漂移、名称冲突、同 operation ID 不同请求，以及对已提交 operation 再 dry-run；其他执行或提交错误为 500。明确失败不返回部分成功列表。commit 错误消息带 operationId 并明确结果未知；客户端应查询 status。

## 事务与重放

执行前先授权。事务内按 actor kind/ID 与 operation ID 取得 advisory lock，读取回执，再按 nodeId 固定顺序锁定全部节点 `FOR UPDATE`。比对 name/ext/parentId/updatedAt，检查所有目标名称，执行更新并读回触发器维护的真实 updatedAt，然后写回执。

请求摘要为 libraryId 和按 nodeId 排序的完整条目 JSON 的 SHA-256，时间统一 UTC 而不截断精度。相同 actor/operation ID 与摘要重放原始结果，`replayed:true`，不重新修改节点；不同摘要返回 409。重放前仍检查当前权限。查询和重放返回提交时事实，之后其他用户再次改名不会覆写旧回执。

事务为全有或全无；没有逐项部分提交。请求有 30 秒服务端时限；客户端取消可能与 COMMIT 并发，取消本身不撤销已提交结果。网络失败后的重试只能使用相同 operation ID 和同一快照，不得生成新 operation ID 盲目重试。

## 数据迁移

`docs/schema/2026-09-22-node-mutation-receipts.sql` 新建 `node_mutation_receipts`，主键 `(actor_id,operation_id)`，保存 libraryId、kind、requestHash、完整结果及提交时间；同一事务重新建立可见名称唯一索引，禁止文件与同名目录并存。迁移遇到历史重复名称会整体失败，不自动修改用户资料。

2026-09-22 已对本机开发库先查重、执行迁移，再运行 `GOCACHE=/tmp/go-build tools/gen_postgres.sh`。本机 dbgate 的 local profile 默认库不同，应显式 `dbgate pg shell -k local -- -d omniflow -v ON_ERROR_STOP=1 -f <migration>`。其他环境仍需迁移后部署代码。回退旧代码不需要删除回执或放宽名称索引。

## CLI 与验证

```bash
of fs rename-batch --library-id 3 --operation-id 12345678-1234-4234-8234-123456789abc --file proposal.json --dry-run --json
of fs rename-status --library-id 3 --operation-id 12345678-1234-4234-8234-123456789abc --json
```

proposal.json 是 items 数组，CLI 不补齐缺失旧值，不自动重试写请求；去掉 dry-run 执行同一请求。CLI 沿用公共客户端的 dry-run result 解包规则，`--json` 直接输出 state 为 validated 的结果。`not_found` 不是撤销凭证。当前提供显式 ID，不增加路径模式。

sqlmock 覆盖事务提交/回滚、第二项失败全部回滚、快照精度、同名/并发唯一约束、回执写失败、commit unknown、重放、身份和权限；HTTP 覆盖字段、大小、dry-run、错误外壳；CLI 覆盖路径、查询、身份、请求体和 JSON 输出。

2026-09-22 已运行 opt-in PostgreSQL 验收 `OMNIFLOW_RENAME_POSTGRES_TEST=1 GOCACHE=/tmp/go-build go test ./internal/usecase -run '^TestRenameBatchPostgres$' -count=1 -v`：单连接外层事务中创建 `pg_temp.nodes` 和 `pg_temp.node_mutation_receipts` 影子表，确认未限定表名指向临时命名空间，再以显式 ID 创建 libraryId=3 的临时夹具。真实 usecase/GORM SQL 通过嵌套 savepoint 验证成功、触发器权威微秒时间读回、dry-run 全部回滚、第二项陈旧整批拒绝、文件与目录可见名称冲突、相同操作重放、异摘要 409 及 status 查询。直接仓储更新也被真实唯一索引拒绝。

测试最后回滚外层事务并关闭连接，不修改用户节点、持久回执、公共 schema 或任何共享 sequence。默认 `go test ./...` 跳过此测试；显式启用只允许 loopback PostgreSQL，加载既有配置且不打印连接秘密。临时触发器故意产生固定的测试时间以验证读回，未声称覆盖用户表其他触发器或跨连接提交崩溃。本轮验证不代表用户 8850 运行服务已加载新路由，也不是实际资料库 UI 验收或数据库并发压测。

同日已另外完成真实新版 8850 与桌面 Agent 的限定 macOS 联调：Win（libraryId=3）新增合成节点 18519/18520，经范围及提案批准后原子添加名称前缀，节点 ID、parentId 和 ext 保持。唯一匹配回执 `2128c82f-4de4-8bb0-a8db-4f28f5d7232e` 为 committed、atomic=true、两项成功；节点与回执时间均为 `2026-09-22T06:42:43.370915Z`。客户端 SQLite 保存同一操作与插件来源，目录/原打开预览名称同步。详情及未覆盖范围见 [W0～W2 桌面验收](../../../omniflow-app/docs/plugin-system-w0-w2-rename.md)，不将临时表测试当成这次实际联调的替代。

变更权限、摘要、批量上限、回执保留或迁移时，应同步维护本文及 HTTP/CLI 测试。
