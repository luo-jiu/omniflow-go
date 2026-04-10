# 全接口进度报告（Go）

更新时间：2026-04-10（P1-B11 已完成）  
基线分支：`main`  
对应日志底座提交：`a6e2a57`

## 总览

| 指标 | 数量 | 说明 |
|---|---:|---|
| `/api/v1` 接口总数 | 45 | 以当前路由注册为准 |
| 功能已实现 | 45 | 全量可用（含兼容 no-op 1 个） |
| 规范改造已完成 | 45 | 包含边界收口、依赖显式错误语义、`lo` 最小必要改造 |
| 不建议改造 | 1 | `PATCH /api/v1/nodes`（兼容保留 no-op） |
| 日志底座（P0） | 已完成 | JSON/文本、console/file、滚动切割、严格配置校验 |
| 日志分层接入（P1） | 已完成（45/45） | B01-B11 已完成 45 个接口 |

## 接口明细

说明：
- `功能状态`：是否可调用。
- `规范状态`：是否完成当前重构规范（职责边界、错误语义、最小必要 `lo`）。
- `日志P1`：是否完成分层日志接入（按批次推进）。

| 模块 | 方法 | 路径 | 功能状态 | 规范状态 | 日志P1 | 备注 |
|---|---|---|---|---|---|---|
| Auth | POST | `/api/v1/auth/login` | 已实现 | 已完成 | 已完成（B01） | 请求日志+业务日志+审计日志 |
| Auth | GET | `/api/v1/auth/status` | 已实现 | 已完成 | 已完成（B01） | 请求日志+业务日志（debug） |
| Auth | DELETE | `/api/v1/auth/logout` | 已实现 | 已完成 | 已完成（B01） | 请求日志+业务日志+审计日志 |
| User | GET | `/api/v1/user/me` | 已实现 | 已完成 | 已完成（B02） | 请求日志+业务日志（debug）+审计日志 |
| User | PUT | `/api/v1/user/me` | 已实现 | 已完成 | 已完成（B02） | 请求日志+业务日志+审计日志 |
| User | PUT | `/api/v1/user/me/password` | 已实现 | 已完成 | 已完成（B02） | 请求日志+业务日志+审计日志 |
| User | POST | `/api/v1/user/me/avatar` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志+审计日志 |
| User | GET | `/api/v1/user/exists` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志（debug） |
| User | GET | `/api/v1/user/:username` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志（debug） |
| User | POST | `/api/v1/user` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志+审计日志 |
| User | PUT | `/api/v1/user/:id` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志+审计日志 |
| Library | GET | `/api/v1/libraries/scroll` | 已实现 | 已完成 | 已完成（B03） | 请求日志+业务日志（debug） |
| Library | POST | `/api/v1/libraries` | 已实现 | 已完成 | 已完成（B03） | 请求日志+业务日志+审计日志 |
| Library | PUT | `/api/v1/libraries/:id` | 已实现 | 已完成 | 已完成（B03） | 请求日志+业务日志+审计日志 |
| Library | DELETE | `/api/v1/libraries/:id` | 已实现 | 已完成 | 已完成（B03） | 请求日志+业务日志+审计日志 |
| Node | POST | `/api/v1/nodes` | 已实现 | 已完成 | 已完成（B07） | 请求日志+业务日志+审计日志 |
| Node | POST | `/api/v1/nodes/search` | 已实现 | 已完成 | 已完成（B07） | 请求日志+业务日志（debug） |
| Node | GET | `/api/v1/nodes/library/:libraryId/root` | 已实现 | 已完成 | 已完成（B06） | 请求日志+业务日志（debug） |
| Node | GET | `/api/v1/nodes/:nodeId` | 已实现 | 已完成 | 已完成（B05） | 请求日志+业务日志（debug） |
| Node | GET | `/api/v1/nodes/:nodeId/descendants` | 已实现 | 已完成 | 已完成（B06） | 请求日志+业务日志（debug） |
| Node | GET | `/api/v1/nodes/:nodeId/children` | 已实现 | 已完成 | 已完成（B05） | 请求日志+业务日志（debug） |
| Node | GET | `/api/v1/nodes/:nodeId/archive/cards` | 已实现 | 已完成 | 已完成（B06） | 请求日志+业务日志（debug） |
| Node | GET | `/api/v1/nodes/:nodeId/ancestors` | 已实现 | 已完成 | 已完成（B07） | 请求日志+业务日志（debug） |
| Node | GET | `/api/v1/nodes/:nodeId/path` | 已实现 | 已完成 | 已完成（B05） | 请求日志+业务日志（debug） |
| Node | PUT | `/api/v1/nodes/:nodeId` | 已实现 | 已完成 | 已完成（B08） | 请求日志+业务日志+审计日志 |
| Node | PATCH | `/api/v1/nodes/:nodeId/rename` | 已实现 | 已完成 | 已完成（B08） | 请求日志+业务日志+审计日志 |
| Node | PATCH | `/api/v1/nodes` | 已实现 | 不建议改造 | 已完成（B10） | 兼容保留 no-op（请求日志+兼容日志） |
| Node | PATCH | `/api/v1/nodes/:nodeId/move` | 已实现 | 已完成 | 已完成（B08） | 请求日志+业务日志+审计日志 |
| Node | PATCH | `/api/v1/nodes/:nodeId/comic/sort-by-name` | 已实现 | 已完成 | 已完成（B10） | 请求日志+业务日志+审计日志 |
| Node | PATCH | `/api/v1/nodes/:nodeId/archive/built-in-type/batch-set` | 已实现 | 已完成 | 已完成（B10） | 请求日志+业务日志+审计日志 |
| Node | DELETE | `/api/v1/nodes/:nodeId/library/:libraryId` | 已实现 | 已完成 | 已完成（B09） | 请求日志+业务日志+审计日志 |
| Node | GET | `/api/v1/nodes/recycle/library/:libraryId` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志（debug） |
| Node | PATCH | `/api/v1/nodes/:nodeId/library/:libraryId/restore` | 已实现 | 已完成 | 已完成（B09） | 请求日志+业务日志+审计日志 |
| Node | DELETE | `/api/v1/nodes/:nodeId/library/:libraryId/hard` | 已实现 | 已完成 | 已完成（B09） | 请求日志+业务日志+审计日志 |
| Directory | POST | `/api/v1/directory/upload` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志+审计日志 |
| Directory | GET | `/api/v1/directory/link` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志（debug） |
| Directory | POST | `/api/v1/directory/links/batch` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志（debug） |
| File | POST | `/api/v1/files/upload` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志 |
| File | GET | `/api/v1/files/link` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志（debug） |
| Tag | GET | `/api/v1/tags/search-types` | 已实现 | 已完成 | 已完成（B04） | 请求日志+业务日志（debug） |
| Tag | GET | `/api/v1/tags` | 已实现 | 已完成 | 已完成（B04） | 请求日志+业务日志（debug） |
| Tag | POST | `/api/v1/tags` | 已实现 | 已完成 | 已完成（B04） | 请求日志+业务日志 |
| Tag | PUT | `/api/v1/tags/:tagId` | 已实现 | 已完成 | 已完成（B04） | 请求日志+业务日志 |
| Tag | DELETE | `/api/v1/tags/:tagId` | 已实现 | 已完成 | 已完成（B04） | 请求日志+业务日志 |
| Health | GET | `/api/v1/health` | 已实现 | 已完成 | 已完成（B11） | 请求日志+业务日志（debug） |

## 日志 P1 批次记录

| 批次 | 接口 | 完成内容 |
|---|---|---|
| B01 | `/api/v1/auth/login` `/api/v1/auth/status` `/api/v1/auth/logout` | 请求日志补齐 actor/error/dry-run 字段；usecase 新增 auth 业务日志；usecase 错误统一挂载到 gin context 供请求日志输出。 |
| B02 | `/api/v1/user/me` `/api/v1/user/me (PUT)` `/api/v1/user/me/password` | User 链路补齐业务日志：资料读取、资料更新、密码更新（含关键阻断原因）；保持现有审计日志语义不变。 |
| B03 | `/api/v1/libraries/scroll` `/api/v1/libraries (POST/PUT/DELETE)` | Library 链路补齐业务日志：滚动查询、创建、更新、删除；保持审计日志语义与 dry-run 语义不变。 |
| B04 | `/api/v1/tags/search-types` `/api/v1/tags` `/api/v1/tags (POST/PUT/DELETE)` | Tag 链路补齐业务日志：类型读取、列表查询、创建/更新/删除；Tag 当前无独立审计 sink，维持最小侵入实现。 |
| B05 | `/api/v1/nodes/:nodeId` `/api/v1/nodes/:nodeId/children` `/api/v1/nodes/:nodeId/path` | Node 查询链路补齐业务日志：详情读取、子节点读取、路径解析；保持 debug 级别并避免高频 info 噪声。 |
| B06 | `/api/v1/nodes/library/:libraryId/root` `/api/v1/nodes/:nodeId/descendants` `/api/v1/nodes/:nodeId/archive/cards` | Node 查询链路补齐业务日志：根节点解析、后代列表、归档卡片分页；保持 debug 级别并避免高频 info 噪声。 |
| B07 | `/api/v1/nodes/:nodeId/ancestors` `/api/v1/nodes/search` `/api/v1/nodes (POST)` | Node 查询/创建链路补齐业务日志：祖先链查询、条件搜索、节点创建；创建链路保持审计日志与 dry-run 语义一致。 |
| B08 | `/api/v1/nodes/:nodeId (PUT)` `/api/v1/nodes/:nodeId/rename` `/api/v1/nodes/:nodeId/move` | Node 变更链路补齐业务日志：更新、重命名、移动；与现有审计日志并行输出，保持 dry-run 语义一致。 |
| B09 | `/api/v1/nodes/:nodeId/library/:libraryId (DELETE)` `/api/v1/nodes/:nodeId/library/:libraryId/restore` `/api/v1/nodes/:nodeId/library/:libraryId/hard` | Node 回收链路补齐业务日志：删除子树、恢复子树、彻底删除；与现有审计日志并行输出并保留 dry-run 语义。 |
| B10 | `/api/v1/nodes/:nodeId/comic/sort-by-name` `/api/v1/nodes/:nodeId/archive/built-in-type/batch-set` `/api/v1/nodes (PATCH)` | Node 批量能力链路补齐业务日志：漫画子项排序、归档子目录批量内置类型设置；兼容 no-op 重排接口补充兼容日志。 |
| B11 | `/api/v1/user/me/avatar` `/api/v1/user/exists` `/api/v1/user/:username` `/api/v1/user` `/api/v1/user/:id` `/api/v1/nodes/recycle/library/:libraryId` `/api/v1/directory/upload` `/api/v1/directory/link` `/api/v1/directory/links/batch` `/api/v1/files/upload` `/api/v1/files/link` `/api/v1/health` | 收口剩余接口日志分层：补齐 User/Directory/File/Health 与 Node 回收站读链路业务日志，P1 达成 45/45 全覆盖。 |

## 下一步建议（日志 P2）

P1 已完成（45/45），后续可切换到 P2：
1. 统一关键业务字段命名规范（如 `resource_id`、`mode`、`result_count`）并输出日志字典。
2. 增补关键链路日志测试（至少覆盖 auth、node mutation、directory upload）。
3. 规划日志采集接入（stdout/file -> collector），保持业务代码零侵入。
