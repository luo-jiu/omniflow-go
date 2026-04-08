# Java 接口对齐状态表

更新时间：2026-04-08

状态定义：
- `已实现并已校对`：Go 已实现，且已做过针对 Java 黑盒行为的对齐改造。
- `已实现待校对`：Go 路由/处理已存在，但还没做逐项 Java 行为对齐。
- `未实现`：Java 有该接口，Go 还没有对应实现。

## 汇总

| 指标 | 数量 |
|---|---:|
| Java 接口总数 | 41 |
| 已实现并已校对 | 24 |
| 已实现待校对 | 17 |
| 未实现 | 0 |

## 全量清单

| 模块 | 方法 | Java 接口 | 当前状态 | 备注 |
|---|---|---|---|---|
| Auth | POST | /api/v1/auth/login | 已实现并已校对 | 登录返回结构已按 Java 契约对齐。 |
| Auth | GET | /api/v1/auth/status | 已实现并已校对 | 状态校验链路已对齐。 |
| Auth | DELETE | /api/v1/auth/logout | 已实现并已校对 | 已支持注销。 |
| User | GET | /api/v1/user/{username} | 已实现待校对 | 已有实现，待联调确认字段细节。 |
| User | GET | /api/v1/user/me | 已实现待校对 | 已有实现，待联调确认响应一致性。 |
| User | GET | /api/v1/user/exists | 已实现待校对 | 已有实现。 |
| User | POST | /api/v1/user | 已实现并已校对 | 注册入参与返回语义已对齐。 |
| User | PUT | /api/v1/user/{id} | 已实现并已校对 | 更新语义已对齐。 |
| User | PUT | /api/v1/user/me | 已实现待校对 | 已有实现，待确认边界行为。 |
| User | PUT | /api/v1/user/me/password | 已实现待校对 | 已有实现，待确认错误码/提示细节。 |
| User | POST | /api/v1/user/me/avatar | 已实现待校对 | 已有实现，待确认上传与响应细节。 |
| Library | GET | /api/v1/libraries/scroll | 已实现并已校对 | 滚动分页语义已对齐。 |
| Library | POST | /api/v1/libraries | 已实现并已校对 | 创建流程已对齐。 |
| Library | PUT | /api/v1/libraries/{id} | 已实现并已校对 | name/starred 更新语义已对齐。 |
| Library | DELETE | /api/v1/libraries/{id} | 已实现并已校对 | 删除流程已对齐。 |
| Node | POST | /api/v1/nodes | 已实现并已校对 | 已对齐 parent 缺失/失效回退 root 的 Java 语义。 |
| Node | GET | /api/v1/nodes/{nodeId} | 已实现并已校对 | 节点详情链路已补齐并对齐。 |
| Node | GET | /api/v1/nodes/{nodeId}/descendants | 已实现待校对 | 已有实现，待确认排序/字段细节。 |
| Node | GET | /api/v1/nodes/{nodeId}/children | 已实现待校对 | 已有实现，待确认排序/字段细节。 |
| Node | GET | /api/v1/nodes/library/{libraryId}/root | 已实现待校对 | 已有实现，待确认修复策略一致性。 |
| Node | POST | /api/v1/nodes/search | 已实现并已校对 | 已按 keyword/tagIds/tagMatchMode/limit 语义补齐。 |
| Node | GET | /api/v1/nodes/{nodeId}/ancestors | 已实现待校对 | 已有实现，待确认路径深度语义。 |
| Node | GET | /api/v1/nodes/{nodeId}/path | 已实现待校对 | 已有实现，待确认路径字符串规则。 |
| Node | PUT | /api/v1/nodes/{nodeId} | 已实现并已校对 | builtInType/archiveMode/viewMeta 已对齐。 |
| Node | PATCH | /api/v1/nodes/{nodeId}/rename | 已实现并已校对 | 已支持 rename + 文件 ext 处理。 |
| Node | PATCH | /api/v1/nodes | 已实现并已校对 | 已对齐为兼容保留空实现（与 Java 一致）。 |
| Node | PATCH | /api/v1/nodes/{nodeId}/move | 已实现待校对 | 已补并发锁域和 beforeNode==nodeId 的 no-op，待继续校对排序细节。 |
| Node | PATCH | /api/v1/nodes/{nodeId}/comic/sort-by-name | 已实现并已校对 | 已按 COMIC 目录排序语义补齐。 |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId} | 已实现并已校对 | Go 路由参数名使用 nodeId，行为等价。 |
| Node | GET | /api/v1/nodes/recycle/library/{libraryId} | 已实现并已校对 | 回收站顶层列表已补齐。 |
| Node | PATCH | /api/v1/nodes/{ancestorId}/library/{libraryId}/restore | 已实现并已校对 | 恢复链路已补齐。 |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId}/hard | 已实现并已校对 | 彻底删除链路已补齐。 |
| File | POST | /api/v1/files/upload | 已实现待校对 | 已有实现，待确认回包/错误语义。 |
| File | GET | /api/v1/files/link | 已实现待校对 | 已有实现，待确认参数兼容。 |
| Directory | POST | /api/v1/directory/upload | 已实现待校对 | 已有实现，待确认上传细节。 |
| Directory | GET | /api/v1/directory/link | 已实现待校对 | 已有实现，待确认参数兼容。 |
| Tag | GET | /api/v1/tags/search-types | 已实现待校对 | 已有实现，待确认返回值与 Java 一致。 |
| Tag | GET | /api/v1/tags | 已实现并已校对 | 已按 Java 逻辑支持 owner+global 查询与 type 过滤。 |
| Tag | POST | /api/v1/tags | 已实现并已校对 | 已补齐 type/targetKey/color/enabled 等参数校验与唯一性检查。 |
| Tag | PUT | /api/v1/tags/{tagId} | 已实现并已校对 | 已补齐 owner 约束更新、冲突检查与字段归一化。 |
| Tag | DELETE | /api/v1/tags/{tagId} | 已实现并已校对 | 已补齐 owner 约束软删除语义。 |

## 下一步建议（按优先级）

1. 对 `已实现待校对` 的 `node move` 做逐项黑盒对齐，防止联调阶段出现“看似可用但行为偏差”。
2. 补一组 `nodes/search` 边界联调用例（空 keyword、ANY/ALL、limit 截断）。
3. 跟前端联调 `tags`，重点确认错误码与消息文案是否需要完全贴合 Java。
