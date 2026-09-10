# 节点标签增量契约

更新时间：2026-09-10

## 权威与边界

`node_tag_rel` 为正式直接标签关系：节点搜索和 2026-05-06 标签迁移以此为准。
`nodes.view_meta.tagIds` 为旧 viewer 的兼容输入/投影，不用于推导本接口的完整关系。
本次不改表结构，不需要迁移或重生成 model/query；使用现有生成模型与查询。

## HTTP

- `GET /api/v1/nodes/:nodeId/tags?libraryId=3`：当前资料库授权后，在节点共享锁事务内读取正式关系。
- `PATCH /api/v1/nodes/:nodeId/tags`：body 为 `{libraryId, addTagIds?, removeTagIds?}`。
- 每组最多 100 个正整数 ID，两组至少一个 ID，不接受重复或交集。跨库或已删除节点返回 404；鉴权沿用现有 library read/write authorizer。
- 新增标签必须属于当前用户或公共标签、已启用且未删除，并通过现有 target policy 校验。
- 移除标签必须属于当前用户或公共标签；允许清理已停用或软删除的标签。其他 owner 的指定标签拒绝，未指定关系不改动。
- 结果为 `{nodeId, libraryId, parentId, tagIds, affectedParentIds, dryRun}`；tagIds 是正式关系的完整 ID 快照，不包含其他 owner 的标签定义。
- `?dryRun=true` 运行同一校验及 SQL 链路后回滚，沿用响应 `data:{dryRun:true,result:...}` 与 dry-run header。正常成功 data 直接为结果。

## 事务

usecase 开启事务并强制要求事务管理器；repository 先锁节点 `FOR UPDATE`，再按 ID 顺序共享锁指定标签。
新增使用 `ON CONFLICT DO NOTHING`，删除严格限定 library/node/明确 tag ID；不会全量替换或重新验证未指定标签。
随后读取正式关系，用 RawMessage 保留其他 metadata 字段和大整数精度，仅同步 tagIds，与关系变更同事务提交。
并发 delta 请求由节点锁串行化。dry-run 使用既有 rollback marker，不触碰 MinIO、Redis 或文件。
日志/审计包含 mode；事务失败不给出已提交结果，客户端遇到网络或 commit 异常须读取核对，不自动重试。

旧 `PUT /nodes/:id` 显式提交整个 viewMeta 的替换契约没有改变：旧调用方之后提交陈旧全量数据仍可能覆盖标签。
增量接口自身不会根据前端旧快照覆盖标签；未来迁移旧 viewer 可消除此旧接口残余风险。

## CLI

```bash
of fs tags --library-id 3 --node-id 9 --json
of fs tags-update --library-id 3 --node-id 9 --add-tag-ids 2,3 --remove-tag-ids 4 --dry-run --json
```

仅接受 node-id/library-id，不支持隐式位置参数；本轮不新增 path 模式。继承认证、base-url 和原始 API 响应 data 语义。

## 验证

专项覆盖节点锁与事务提交/回滚、未知关系和大整数 metadata 保留、增删参数、owner 拒绝、绑定策略失败、只读及缺失节点、HTTP 绑定/dry-run header、CLI 鉴权与 body/query。
使用 sqlmock 与 HTTP fake，不将测试结果当作真实数据库并发压测或已运行服务验收；按本轮协作要求不跑全量、不重启服务。
