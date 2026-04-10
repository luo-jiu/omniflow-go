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
| Auth | GET | /api/v1/auth/status | 低 | 依赖缺失语义收口（显式错误），会话一致性校验保持黑盒行为 | 已完成 | B07 |
| Auth | DELETE | /api/v1/auth/logout | 低 | 依赖缺失语义收口（显式错误），dry-run 语义保持不变 | 已完成 | B07 |
| User | GET | /api/v1/user/{username} | 低 | 依赖缺失语义收口（显式错误） | 已完成 | B06 |
| User | GET | /api/v1/user/me | 低 | 以规范收口为主（依赖缺失统一错误语义） | 已完成 | B02 |
| User | GET | /api/v1/user/exists | 低 | 依赖缺失语义收口（显式错误） | 已完成 | B06 |
| User | POST | /api/v1/user | 低 | 注册链路昵称 fallback 改为 `lo.Ternary`；依赖缺失语义收口 | 已完成 | B06 |
| User | PUT | /api/v1/user/{id} | 低 | 依赖缺失语义收口（显式错误） | 已完成 | B06 |
| User | PUT | /api/v1/user/me | 低 | 以规范收口为主（依赖缺失统一错误语义） | 已完成 | B02 |
| User | PUT | /api/v1/user/me/password | 低 | 以规范收口为主（依赖缺失统一错误语义） | 已完成 | B02 |
| User | POST | /api/v1/user/me/avatar | 中 | 头像上传归一化引入 lo 候选查找，减少分支样板 | 已完成 | B02 |
| Library | GET | /api/v1/libraries/scroll | 中 | 列表映射与依赖缺失语义收口；映射样板改为 lo.Map | 已完成 | B05 |
| Library | POST | /api/v1/libraries | 低 | 依赖缺失语义收口（显式错误） | 已完成 | B05 |
| Library | PUT | /api/v1/libraries/{id} | 低 | 小集合校验改为 lo.Contains；依赖缺失语义收口 | 已完成 | B05 |
| Library | DELETE | /api/v1/libraries/{id} | 低 | 依赖缺失语义收口（显式错误） | 已完成 | B05 |
| Node | POST | /api/v1/nodes | 高 | 依赖缺失语义收口（显式错误），创建参数归一化保持最小语义变更 | 已完成 | B12 |
| Node | GET | /api/v1/nodes/{nodeId} | 低 | 依赖缺失语义收口（显式错误） | 已完成 | B08 |
| Node | GET | /api/v1/nodes/{nodeId}/descendants | 高 | 依赖缺失语义收口（显式错误），子树查询职责保持在 usecase/repository | 已完成 | B08 |
| Node | GET | /api/v1/nodes/{nodeId}/children | 中 | 依赖缺失语义收口（显式错误），子节点查询职责保持在 usecase/repository | 已完成 | B08 |
| Node | GET | /api/v1/nodes/library/{libraryId}/root | 低 | 依赖缺失语义收口（显式错误），根节点自修复行为保持不变 | 已完成 | B09 |
| Node | POST | /api/v1/nodes/search | 高 | 依赖缺失语义收口（显式错误），tagIDs 归一化改为 lo.Filter+Uniq | 已完成 | B09 |
| Node | GET | /api/v1/nodes/{nodeId}/ancestors | 中 | 祖先链转换改为 lo.Map，依赖缺失语义收口（显式错误） | 已完成 | B08 |
| Node | GET | /api/v1/nodes/{nodeId}/path | 中 | 路径拼接改为 lo.Map + Join，依赖缺失语义收口（显式错误） | 已完成 | B08 |
| Node | PUT | /api/v1/nodes/{nodeId} | 中 | 依赖缺失语义收口（显式错误），archiveMode 校验改为 lo.Contains | 已完成 | B11 |
| Node | PATCH | /api/v1/nodes/{nodeId}/rename | 低 | 依赖缺失语义收口（显式错误），最小改造保持重命名语义 | 已完成 | B11 |
| Node | PATCH | /api/v1/nodes | 中 | 当前为 Java 兼容 no-op 占位接口，无实际集合处理逻辑 | 不建议改造 | B14 |
| Node | PATCH | /api/v1/nodes/{nodeId}/move | 高 | 依赖缺失语义收口（显式错误），移动语义与 dry-run 保持不变 | 已完成 | B11 |
| Node | PATCH | /api/v1/nodes/{nodeId}/comic/sort-by-name | 高 | 依赖缺失语义收口（显式错误），排序语义保持不变 | 已完成 | B12 |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId} | 中 | 依赖缺失语义收口（显式错误），子树删除语义保持不变 | 已完成 | B12 |
| Node | GET | /api/v1/nodes/recycle/library/{libraryId} | 高 | 回收站顶层筛选改为 lo.Filter，依赖缺失语义收口（显式错误） | 已完成 | B13 |
| Node | PATCH | /api/v1/nodes/{ancestorId}/library/{libraryId}/restore | 高 | 依赖缺失语义收口（显式错误），恢复语义与 dry-run 保持不变 | 已完成 | B13 |
| Node | DELETE | /api/v1/nodes/{ancestorId}/library/{libraryId}/hard | 高 | 依赖缺失语义收口（显式错误），彻底删除语义与 dry-run 保持不变 | 已完成 | B13 |
| File | POST | /api/v1/files/upload | 中 | 存储依赖配置检查收口，默认值归一化保持朴素 if 语义 | 已完成 | B10 |
| File | GET | /api/v1/files/link | 低 | 存储依赖配置检查收口，过期时间默认值保持朴素 if 语义 | 已完成 | B10 |
| Directory | POST | /api/v1/directory/upload | 中 | 接口降级行为收口，依赖缺失统一显式报错 | 已完成 | B04 |
| Directory | GET | /api/v1/directory/link | 中 | 接口降级行为收口，依赖缺失统一显式报错 | 已完成 | B04 |
| Tag | GET | /api/v1/tags/search-types | 中 | 依赖缺失语义收口（显式错误），返回契约保持不变 | 已完成 | B07 |
| Tag | GET | /api/v1/tags | 中 | 依赖缺失语义收口（显式错误），列表查询职责留在 usecase/repository | 已完成 | B07 |
| Tag | POST | /api/v1/tags | 中 | 类型集合与唯一键 scope 处理改为 lo 工具，减少手写去重样板 | 已完成 | B03 |
| Tag | PUT | /api/v1/tags/{tagId} | 中 | 类型集合与唯一键 scope 处理改为 lo 工具，减少手写去重样板 | 已完成 | B03 |
| Tag | DELETE | /api/v1/tags/{tagId} | 低 | 写接口 dry-run 头与其他模块保持一致 | 已完成 | B03 |

## Go 扩展接口清单（4）

| 模块 | 方法 | 接口 | 与 Java 关系 | `lo` 适配优先级 | 推荐改造点（调研结论） | 当前状态 | 批次 |
|---|---|---|---|---|---|---|---|
| Health | GET | /api/v1/health | Go 新增 | 低 | 依赖缺失语义收口（显式错误） | 已完成 | B10 |
| Directory | POST | /api/v1/directory/links/batch | Go 新增 | 高 | 批量 nodeID 去重、过滤、结果映射改为 lo 写法 | 已完成 | B04 |
| Node | GET | /api/v1/nodes/{nodeId}/archive/cards | Go 新增 | 高 | 依赖缺失语义收口（显式错误），档案卡片聚合逻辑保持不变 | 已完成 | B09 |
| Node | PATCH | /api/v1/nodes/{nodeId}/archive/built-in-type/batch-set | Go 新增 | 高 | 子目录筛选与映射改为 lo.Filter/Map，依赖缺失语义收口（显式错误） | 已完成 | B12 |

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
| B05 | /api/v1/libraries/scroll, /api/v1/libraries (POST/PUT/DELETE) | Library 四接口全链路收口：handler/usecase 依赖缺失统一显式错误；列表映射和小集合校验替换为 `lo`。 | 已完成 |
| B06 | /api/v1/user/{username}, /api/v1/user/exists, /api/v1/user, /api/v1/user/{id} | 公开用户接口四项收口：handler/usecase 依赖缺失统一显式错误；注册昵称回退替换为 `lo.Ternary`。 | 已完成 |
| B07 | /api/v1/auth/status, /api/v1/auth/logout, /api/v1/tags/search-types, /api/v1/tags | Auth/Tag 读接口四项收口：去除“依赖缺失时假成功”降级，统一显式错误；TagUseCase 增加配置校验函数，保持层次边界清晰。 | 已完成 |
| B08 | /api/v1/nodes/{nodeId}, /api/v1/nodes/{nodeId}/descendants, /api/v1/nodes/{nodeId}/children, /api/v1/nodes/{nodeId}/ancestors, /api/v1/nodes/{nodeId}/path | Node 读接口五项收口：去除“依赖缺失时假成功”降级并统一显式错误；usecase 增加 `nodes` 配置检查；祖先/路径组装替换为 `lo`。 | 已完成 |
| B09 | /api/v1/nodes/library/{libraryId}/root, /api/v1/nodes/search, /api/v1/nodes/{nodeId}/archive/cards | Node 查询接口三项收口：去除“依赖缺失时假成功”降级并统一显式错误；usecase 增加 `nodes` 配置检查；搜索 tagIDs 归一化替换为 `lo`。 | 已完成 |
| B10 | /api/v1/files/upload, /api/v1/files/link, /api/v1/health | File/Health 三接口收口：补齐依赖缺失显式错误；FileUseCase 统一存储依赖检查；默认值归一化保持朴素 `if`。 | 已完成 |
| B11 | /api/v1/nodes/{nodeId} (PUT), /api/v1/nodes/{nodeId}/rename, /api/v1/nodes/{nodeId}/move | Node 变更接口三项收口：去除“依赖缺失时假成功”降级并统一显式错误；usecase 增加 `nodes` 配置检查；archiveMode 校验改为 `lo.Contains`。 | 已完成 |
| B12 | /api/v1/nodes (POST), /api/v1/nodes/{nodeId}/comic/sort-by-name, /api/v1/nodes/{nodeId}/archive/built-in-type/batch-set, /api/v1/nodes/{nodeId}/library/{libraryId} (DELETE) | Node 变更接口四项收口：去除“依赖缺失时假成功”降级并统一显式错误；usecase 增加 `nodes` 配置检查；子目录筛选与映射替换为 `lo.Filter/Map`。 | 已完成 |
| B13 | /api/v1/nodes/recycle/library/{libraryId}, /api/v1/nodes/{nodeId}/library/{libraryId}/restore, /api/v1/nodes/{nodeId}/library/{libraryId}/hard | Node 回收站三接口收口：去除“依赖缺失时假成功”降级并统一显式错误；usecase 增加 `nodes` 配置检查；回收站顶层筛选替换为 `lo.Filter`。 | 已完成 |
| B14 | /api/v1/nodes (PATCH) | 该接口当前为兼容保留 no-op，占位语义明确，无集合处理逻辑，标记为 `不建议改造` 以避免过度设计。 | 已完成 |
