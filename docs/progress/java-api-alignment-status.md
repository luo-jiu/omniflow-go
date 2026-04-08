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
| 已实现并已校对 | 41 |
| 已实现待校对 | 0 |
| 未实现 | 0 |

## 全量清单

| 模块 | 方法 | Java 接口 | 当前状态 | 备注 |
|---|---|---|---|---|
| Auth | POST | /api/v1/auth/login | 已实现并已校对 | 登录返回结构已按 Java 契约对齐。 |
| Auth | GET | /api/v1/auth/status | 已实现并已校对 | 状态校验链路已对齐。 |
| Auth | DELETE | /api/v1/auth/logout | 已实现并已校对 | 已支持注销。 |
| User | GET | /api/v1/user/{username} | 已实现并已校对 | 已按 Java 语义返回不脱敏用户资料。 |
| User | GET | /api/v1/user/me | 已实现并已校对 | 已按当前登录用户语义返回不脱敏资料。 |
| User | GET | /api/v1/user/exists | 已实现并已校对 | 已对齐 Java 语义：返回“用户名是否可用（未占用）”。 |
| User | POST | /api/v1/user | 已实现并已校对 | 注册入参与返回语义已对齐。 |
| User | PUT | /api/v1/user/{id} | 已实现并已校对 | 更新语义已对齐。 |
| User | PUT | /api/v1/user/me | 已实现并已校对 | 已按“仅本人更新”与更新后回包语义对齐。 |
| User | PUT | /api/v1/user/me/password | 已实现并已校对 | 已对齐旧密码校验与“新旧密码不能相同”语义。 |
| User | POST | /api/v1/user/me/avatar | 已实现并已校对 | 已对齐图片上传校验、对象键格式与头像 URL 过期策略。 |
| Library | GET | /api/v1/libraries/scroll | 已实现并已校对 | 滚动分页语义已对齐。 |
| Library | POST | /api/v1/libraries | 已实现并已校对 | 创建流程已对齐。 |
| Library | PUT | /api/v1/libraries/{id} | 已实现并已校对 | name/starred 更新语义已对齐。 |
| Library | DELETE | /api/v1/libraries/{id} | 已实现并已校对 | 删除流程已对齐。 |
| Node | POST | /api/v1/nodes | 已实现并已校对 | 已对齐 parent 缺失/失效回退 root 的 Java 语义。 |
| Node | GET | /api/v1/nodes/{nodeId} | 已实现并已校对 | 节点详情链路已补齐并对齐。 |
| Node | GET | /api/v1/nodes/{nodeId}/descendants | 已实现并已校对 | 已补读权限校验并保持子树返回语义。 |
| Node | GET | /api/v1/nodes/{nodeId}/children | 已实现并已校对 | 已补读权限校验并保持 sortOrder/id 顺序。 |
| Node | GET | /api/v1/nodes/library/{libraryId}/root | 已实现并已校对 | 已补 root 自修复（坏父引用归根）与读权限校验。 |
| Node | POST | /api/v1/nodes/search | 已实现并已校对 | 已按 keyword/tagIds/tagMatchMode/limit 语义补齐。 |
| Node | GET | /api/v1/nodes/{nodeId}/ancestors | 已实现并已校对 | 已补读权限校验并保持 root->self 深度链语义。 |
| Node | GET | /api/v1/nodes/{nodeId}/path | 已实现并已校对 | 已补读权限校验并保持完整路径拼接语义。 |
| Node | PUT | /api/v1/nodes/{nodeId} | 已实现并已校对 | builtInType/archiveMode/viewMeta 已对齐。 |
| Node | PATCH | /api/v1/nodes/{nodeId}/rename | 已实现并已校对 | 已支持 rename + 文件 ext 处理。 |
| Node | PATCH | /api/v1/nodes | 已实现并已校对 | 已对齐为兼容保留空实现（与 Java 一致）。 |
| Node | PATCH | /api/v1/nodes/{nodeId}/move | 已实现并已校对 | 已对齐并发锁域、beforeNode 自指 no-op 与间隔排序重排策略。 |
| Node | PATCH | /api/v1/nodes/{nodeId}/comic/sort-by-name | 已实现并已校对 | 已按 COMIC 目录排序语义补齐。 |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId} | 已实现并已校对 | Go 路由参数名使用 nodeId，行为等价。 |
| Node | GET | /api/v1/nodes/recycle/library/{libraryId} | 已实现并已校对 | 回收站顶层列表已补齐。 |
| Node | PATCH | /api/v1/nodes/{ancestorId}/library/{libraryId}/restore | 已实现并已校对 | 恢复链路已补齐。 |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId}/hard | 已实现并已校对 | 彻底删除链路已补齐。 |
| File | POST | /api/v1/files/upload | 已实现并已校对 | 已对齐 path 默认值、上传后直接回预签名链接语义。 |
| File | GET | /api/v1/files/link | 已实现并已校对 | 已对齐 file_name/path/expiry 参数语义与默认值。 |
| Directory | POST | /api/v1/directory/upload | 已实现并已校对 | 已对齐上传建节点流程与失败补偿语义。 |
| Directory | GET | /api/v1/directory/link | 已实现并已校对 | 已对齐 node_id/library_id/expiry 参数与默认时长语义。 |
| Tag | GET | /api/v1/tags/search-types | 已实现并已校对 | 已对齐 Java 黑盒返回语义（MySQL）。 |
| Tag | GET | /api/v1/tags | 已实现并已校对 | 已按 Java 逻辑支持 owner+global 查询与 type 过滤。 |
| Tag | POST | /api/v1/tags | 已实现并已校对 | 已补齐 type/targetKey/color/enabled 等参数校验与唯一性检查。 |
| Tag | PUT | /api/v1/tags/{tagId} | 已实现并已校对 | 已补齐 owner 约束更新、冲突检查与字段归一化。 |
| Tag | DELETE | /api/v1/tags/{tagId} | 已实现并已校对 | 已补齐 owner 约束软删除语义。 |

## 下一步建议（按优先级）

1. 补一组 `nodes/search` 边界联调用例（空 keyword、ANY/ALL、limit 截断）。
2. 跟前端做一轮全量冒烟联调（Auth/User/Library/Node/File/Directory/Tag）。
3. 把联调发现的行为差异回灌到该表，维持持续可追踪。
