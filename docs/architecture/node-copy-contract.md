# 内部节点复制契约

更新时间：2026-09-10

## 概述

复制面向 HTTP、CLI 和 agent，支持同库文件和显式递归目录。不增加表结构，不复用源对象 key。副本后续编辑不会影响源文件，反之亦然。

## HTTP

`POST /api/v1/nodes/:nodeId/copy`，挂载组内为 `/nodes/:nodeId/copy`。

请求体：

```json
{"libraryId":3,"parentId":10,"name":"report-copy.xlsx","recursive":false,"conflictPolicy":"error"}
```

- `libraryId`、`parentId` 必填正整数；`parentId` 必须是本库现存目录 ID，不能用 `0` 代指根目录。
- `name` 可省略，保留源名称；提供时是完整文件名，后缀从最后一个点拆分。目录名不拆后缀，隐藏文件 `.env` 保持完整主体。
- `recursive` 默认 `false`，复制任何目录（包括空目录）均须显式 `true`。
- `conflictPolicy` 默认 `error`，只支持 `error`、`auto_rename`，不覆盖已有节点。
- `?dryRun=true` 或 `?dry_run=true` 使用同一校验和数据库创建链路，最终回滚；不上传、删除对象，不创建异步任务。只读 `StatObject` 检查全部源文件可达性、大小和配置 bucket。

沿用 `code/message/data/request_id` 外壳，成功 `data`：

```json
{"node":{"id":20,"name":"report-copy","ext":"xlsx","type":"file","parentId":10,"libraryId":3},"copiedCount":1,"affectedParentIds":[10],"dryRun":false}
```

`node` 为常规节点对象；`copiedCount` 包括根副本和全部后代；`affectedParentIds` 为现存目标父目录，不包含未变化的源父目录。预演返回计划数量、解析后的名称，根副本 `id=0` 且无 `storageKey`，不得拿预演结果继续操作。数据库序列可能因预演产生间隙。

## 约束与失败语义

- 同库、活跃节点；缺失或跨库 ID 返回 `404`。沿用资料库写权限校验，拒绝时 `403`。
- 禁止复制资料库根、复制到自身或自身后代；参数和资源超限返回 `400`。
- 同名默认 `409`，`auto_rename` 沿用同级锁和编号规则。
- 每次最多 1000 个节点（含目录）、总文件内容最多 1 GiB、服务端最多 120 秒。目录按层限量枚举，不先无限加载整棵树。
- 无对象绑定的元数据文件、失配 bucket、不可达 provider、缺失对象明确失败，不伪造空文件成功。源对象大小变化返回 `409`。
- 复制保留名称、类型、文件字节、MIME 和存储位置；本轮不复制标签、内置浏览类型、归档模式、视图配置、历史、回收站内容。新节点使用普通创建的默认展示配置。
- 源文件沿用原 provider 和 bucket，在该存储中上传独立随机 key；不尝试自动迁移到其他存储。

## 事务和对象生命周期

先锁定源节点和目标目录，枚举并校验全部对象，再通过现有创建链路在单事务内创建副本树。只有全部数据库创建校验成功后才开始流式复制；dry-run 不进入对象写入阶段。源节点共享锁与已有内容替换的更新锁互斥。

对象写入失败时事务回滚，清理本次随机 key，使用独立且有限的清理超时。提交返回错误且事务回调已完成时，提交结果不明：保留全部新对象，错误含候选根 `nodeId`，禁止自动重试或误删可能已经引用的对象。网络响应丢失也应先检查目标目录。当前无幂等 operation ID 或自动提交对账接口；不明结果可能留下待后续核对的孤立对象。

成功及预演写 `node.copy` 审计，日志区分 `dry_run`；提交不明和清理失败分别记录事件。该同步复制适合有界基础整理，不提供全资料库的一致性快照或超大复制后台任务。

## CLI 与验证

```bash
of fs cp --library-id 3 --node-id 9 --parent-id 10 --name copy.txt --dry-run --json
of fs cp --library-id 3 --node-id 12 --parent-id 10 --recursive --conflict-policy auto_rename --json
```

CLI 仅接受显式 ID，无隐式位置参数；可以先用 `fs path resolve` 定位。复制请求超时 130 秒，不修改其他命令的超时，不自动重试写请求。

测试使用 SQL mock、fake storage 和 HTTP mock，不连接真实资料库。覆盖独立对象字节、事务成功/回滚/提交不明、完整预演、冲突、递归、根保护、子树保护、资源枚举上限、存储错误、权限及 HTTP/CLI 契约。

维护时必须同步路由、CLI 参数、资源限制、失败语义和本文档。
