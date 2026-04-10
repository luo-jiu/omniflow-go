# Go 接口 `lo` 改造台账（唯一维护版本）

更新时间：2026-04-10

## 维护决策

- 从 2026-04-10 起，仅维护 Go 项目接口与实现。
- Java 项目不再作为迁移目标或验收基线，仅作历史参考。
- 本文档后续只跟踪 Go `/api/v1` 接口的 `lo` 改造进度。

## 目标

在不改变当前 Go 对外接口契约的前提下，按接口逐批改造 Go 代码中“集合处理/映射转换/去重分组”场景，优先用 `github.com/samber/lo` 提升可读性与一致性。

## 接口基线（单独调研结果）

| 维度 | 数量 | 说明 |
|---|---:|---|
| Go `/api/v1` 接口 | 45 | 以 `omniflow-go` 路由注册为准（当前唯一维护集合） |
| 历史 Java Controller 接口 | 41 | 冻结，仅历史参考，不再维护 |
| Go 历史增量接口 | 4 | 健康检查 1 个、业务扩展 3 个 |

## 状态定义

- `待开始`：已调研，尚未进入改造批次。
- `改造中`：已开始改造，尚未完成自检。
- `已完成`：已改造并通过当前批次自检。
- `不建议改造`：该接口链路场景不适合引入 `lo`（收益低或可读性更差）。

## 优先级定义

- `高`：存在明显的切片/映射处理、去重、过滤、分组或转换逻辑。
- `中`：存在少量集合处理，可改可不改。
- `低`：以单对象流程为主，引入 `lo` 收益有限。

## Go 核心接口清单（历史来源于原 41 个迁移接口）

| 模块 | 方法 | 接口 | `lo` 适配优先级 | 推荐改造点（调研结论） | 当前状态 | 批次 |
|---|---|---|---|---|---|---|
| Auth | POST | /api/v1/auth/login | 低 | 登录链路共享鉴权白名单集合构建，适合统一做 trim/去空/去重/set 化 | 已完成 | B01 |
| Auth | GET | /api/v1/auth/status | 低 | 状态读取为主，集合处理少 | 待开始 | - |
| Auth | DELETE | /api/v1/auth/logout | 低 | 清理流程为主，集合处理少 | 待开始 | - |
| User | GET | /api/v1/user/{username} | 低 | 单对象查询与返回 | 待开始 | - |
| User | GET | /api/v1/user/me | 低 | 以规范收口为主（依赖缺失统一错误语义） | 已完成 | B02 |
| User | GET | /api/v1/user/exists | 低 | 校验逻辑为主 | 待开始 | - |
| User | POST | /api/v1/user | 低 | 注册流程为主 | 待开始 | - |
| User | PUT | /api/v1/user/{id} | 低 | 单对象更新为主 | 待开始 | - |
| User | PUT | /api/v1/user/me | 低 | 以规范收口为主（依赖缺失统一错误语义） | 已完成 | B02 |
| User | PUT | /api/v1/user/me/password | 低 | 以规范收口为主（依赖缺失统一错误语义） | 已完成 | B02 |
| User | POST | /api/v1/user/me/avatar | 中 | 头像上传归一化引入 lo 候选查找，减少分支样板 | 已完成 | B02 |
| Library | GET | /api/v1/libraries/scroll | 中 | 列表过滤、映射与分页边界处理 | 待开始 | - |
| Library | POST | /api/v1/libraries | 低 | 创建流程为主 | 待开始 | - |
| Library | PUT | /api/v1/libraries/{id} | 低 | 更新流程为主 | 待开始 | - |
| Library | DELETE | /api/v1/libraries/{id} | 低 | 删除流程为主 | 待开始 | - |
| Node | POST | /api/v1/nodes | 高 | 请求归一化、ID 过滤去重、批量关联处理 | 待开始 | - |
| Node | GET | /api/v1/nodes/{nodeId} | 低 | 单节点查询为主 | 待开始 | - |
| Node | GET | /api/v1/nodes/{nodeId}/descendants | 高 | 子树结果聚合与映射处理 | 待开始 | - |
| Node | GET | /api/v1/nodes/{nodeId}/children | 中 | 子节点结果映射与过滤 | 待开始 | - |
| Node | GET | /api/v1/nodes/library/{libraryId}/root | 低 | 单节点定位与自修复为主 | 待开始 | - |
| Node | POST | /api/v1/nodes/search | 高 | 条件过滤、结果去重、标签匹配聚合 | 待开始 | - |
| Node | GET | /api/v1/nodes/{nodeId}/ancestors | 中 | 路径链条转换与排序 | 待开始 | - |
| Node | GET | /api/v1/nodes/{nodeId}/path | 中 | 路径结果拼接与映射 | 待开始 | - |
| Node | PUT | /api/v1/nodes/{nodeId} | 中 | viewMeta/builtInType 归一化可用集合工具增强可读性 | 待开始 | - |
| Node | PATCH | /api/v1/nodes/{nodeId}/rename | 低 | 单节点改名为主 | 待开始 | - |
| Node | PATCH | /api/v1/nodes | 中 | 批量重排入参处理与归一化 | 待开始 | - |
| Node | PATCH | /api/v1/nodes/{nodeId}/move | 高 | 同级/跨级移动时列表重排和去重映射 | 待开始 | - |
| Node | PATCH | /api/v1/nodes/{nodeId}/comic/sort-by-name | 高 | 排序前后集合处理与稳定性处理 | 待开始 | - |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId} | 中 | 子树收集与删除候选集处理 | 待开始 | - |
| Node | GET | /api/v1/nodes/recycle/library/{libraryId} | 高 | 回收站聚合结果映射与去重处理 | 待开始 | - |
| Node | PATCH | /api/v1/nodes/{ancestorId}/library/{libraryId}/restore | 高 | 恢复链路中的批量节点收集与关联处理 | 待开始 | - |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId}/hard | 高 | 彻底删除前的子集推导与去重 | 待开始 | - |
| File | POST | /api/v1/files/upload | 中 | 批量参数归一化与白名单处理可优化 | 待开始 | - |
| File | GET | /api/v1/files/link | 低 | 单链接生成为主 | 待开始 | - |
| Directory | POST | /api/v1/directory/upload | 中 | 接口降级行为收口，依赖缺失统一显式报错 | 已完成 | B04 |
| Directory | GET | /api/v1/directory/link | 中 | 接口降级行为收口，依赖缺失统一显式报错 | 已完成 | B04 |
| Tag | GET | /api/v1/tags/search-types | 中 | 固定类型列表转换/去重 | 待开始 | - |
| Tag | GET | /api/v1/tags | 中 | 结果过滤与映射 | 待开始 | - |
| Tag | POST | /api/v1/tags | 中 | 类型集合与唯一键 scope 处理改为 lo 工具，减少手写去重样板 | 已完成 | B03 |
| Tag | PUT | /api/v1/tags/{tagId} | 中 | 类型集合与唯一键 scope 处理改为 lo 工具，减少手写去重样板 | 已完成 | B03 |
| Tag | DELETE | /api/v1/tags/{tagId} | 低 | 写接口 dry-run 头与其他模块保持一致 | 已完成 | B03 |

## Go 扩展接口清单（4）

| 模块 | 方法 | 接口 | 与 Java 关系 | `lo` 适配优先级 | 推荐改造点（调研结论） | 当前状态 | 批次 |
|---|---|---|---|---|---|---|---|
| Health | GET | /api/v1/health | Go 新增 | 低 | 健康检查，无集合逻辑 | 待开始 | - |
| Directory | POST | /api/v1/directory/links/batch | Go 新增 | 高 | 批量 nodeID 去重、过滤、结果映射改为 lo 写法 | 已完成 | B04 |
| Node | GET | /api/v1/nodes/{nodeId}/archive/cards | Go 新增 | 高 | 档案卡片聚合、父子关系映射、缺失项补齐 | 待开始 | - |
| Node | PATCH | /api/v1/nodes/{nodeId}/archive/built-in-type/batch-set | Go 新增 | 高 | 批量子节点筛选、分组与写入前归一化 | 待开始 | - |

## 批次推进规则

1. 每次只做一批（建议 2-5 个接口）。
2. 仅在“集合处理”层面引入 `lo`，不改接口契约。
3. 每批完成后，同步更新本文件的“当前状态/批次/备注”。
4. 若某接口验证后收益低，标记为 `不建议改造`，避免过度工程化。

## 批次验收门禁（固定执行）

每一批接口改造都按以下顺序执行，避免“改了代码但方向跑偏”：

1. 层次边界检查：`handler -> usecase -> repository` 依赖方向是否干净，禁止反向依赖。
2. 职责清晰检查：协议解析留在 handler；业务规则留在 usecase；数据访问留在 repository。
3. 跨层泄漏检查：避免在 handler/usecase 直接出现底层 SQL/存储实现细节。
4. 关键注释检查：关键分支、兼容语义、边界约束必须有轻量注释。
5. `lo` 替换检查：优先替换集合处理样板代码，避免为了使用 `lo` 强行改造。
6. 改造分寸检查：坚持最小必要改造，不做与当前接口无关的重构。
7. 风险顺带检查：改造过程中顺带识别明显风险并修复，不额外扩大战场。

## 批次记录

| 批次 | 涉及接口 | 改造摘要 | 结果 |
|---|---|---|---|
| B01 | /api/v1/auth/login | 在鉴权中间件引入 `lo`，统一白名单路径的 `trim + 去空 + 去重 + set` 构建，保持登录行为不变。 | 已完成 |
| B02 | /api/v1/user/me, /api/v1/user/me(password/avatar) | `user/me` 相关 4 接口做全链路规范收口；头像上传归一化引入 `lo.SomeBy + lo.Find` 处理 MIME/扩展候选。 | 已完成 |
| B03 | /api/v1/tags (POST/PUT/DELETE) | `Tag` 写接口链路引入 `lo`（类型集合、锁 scope 去重过滤、列表映射），并统一 dry-run 响应头行为。 | 已完成 |
| B04 | /api/v1/directory/upload, /api/v1/directory/link, /api/v1/directory/links/batch | Directory 三接口全链路收口：handler 依赖缺失统一显式错误；批量 nodeID 与存储键映射改为 `lo` 去重/过滤/映射。 | 已完成 |
