# 节点元数据查询契约

更新时间：2026-09-08

## 概述

为 Agent 和 CLI 提供不依赖 UI 展开状态的资料库浏览与搜索。查询只访问数据库，不访问 MinIO，不创建或修复资料库根节点；不变更表结构。

## HTTP

`POST /api/v1/nodes/metadata/query` 沿用认证、资料库读权限与 `code/message/data/request_id` 外壳。

请求字段：

| 字段 | 规则 |
|---|---|
| `libraryId` | 必填，授权后才能读取 |
| `mode` | `root` / `children`（默认）/ `search` / `node` |
| `parentId` | 仅 children，省略或 0 表示根目录 |
| `ancestorId` | 仅 search，限定目录子树 |
| `nodeId` | 仅 node，必填 |
| `keyword` | 字面量名称子串，不区分大小写，最多 512 字节 |
| `names` | 最多两个精确存储名称，用于路径解析的完整文件名与去扩展名候选 |
| `nodeType` | `dir` / `file` |
| `tagIds` / `tagMatchMode` | 最多 50 个标签，ANY（默认）/ ALL |
| `limit` / `cursor` | 默认 50，最大 100；续页使用上次 nextCursor |

root / node 不接受筛选或游标，目录与子树范围必须属于当前资料库。非法参数映射现有参数错误，缺失节点映射 404，权限错误沿用原有读权限语义。

`data` 包含 `root`、`entries`、`hasMore`、可选 `nextCursor`。节点字段为 `id/libraryId/parentId/name/type/path/ext/mimeType/fileSize/storageProvider/storageProviderLabel/updatedAt`，可选字段省略；不返回对象键、endpoint、bucket、viewMeta 或凭据。根路径为 `/`，不包含资料库显示名；文件路径补足扩展名。损坏或缺失祖先链时不提供 path，不虚构位置。

## 分页与实现边界

按节点 ID 升序 keyset 分页，读取 limit+1 判断续页。游标绑定规范化查询的 SHA256，不是授权凭证；每次请求独立验证身份和资料库权限。续页可以调整 limit，不能更改筛选条件。

查询有 10 秒上下文期限。分页不是事务快照，并发新增、移动、删除可能影响结果。常规查询使用生成 model 的 GORM 链式条件，递归 SQL 仅用于子树范围和规范祖先路径，包含资料库、软删除和环路约束。

## CLI

`of fs browse --library-id <id> --mode root|children|search|node --json`。筛选字段使用 kebab-case flags；node 模式使用 `--node-id`。CLI 不解析物理存储、不绕过 HTTP，读取无持久化副作用，不需要 dry-run。

## 验证

单测覆盖权限优先、跨库拒绝、纯根读取、游标范围、分页限额、GORM SQL 范围与转义、元数据安全投影、HTTP 参数及 CLI 请求契约。集成测试不得使用第一个资料库。

2026-09-08 本机最新 API 与 Electron 在非第一个 win 资料库完成真实 PostgreSQL 验收：根目录两页 keyset 查询、目录子树搜索、逐段精确解析、缺失路径失败、单节点查询均通过；内部音频保存后再次读取实际输出节点通过。离线 Windows MinIO 不阻断数据库元数据查询。Go 全量测试通过；本轮未替换云端 API 镜像。
