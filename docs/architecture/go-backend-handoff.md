# OmniFlow Go 后端交接说明

更新时间：2026-04-15

适用对象：继续维护 `omniflow-go` 后端、CLI、Agent 集成和前端联调的开发者或 Coding Agent。

## 1. 当前结论

`omniflow-go` 是 OmniFlow 的 Go 后端，所有开发都以当前 Go 对外契约为准。

Go 后端已经覆盖并扩展核心业务能力，后续工作重点是：

- 保持前端、CLI、脚本调用方对外契约稳定。
- 持续收紧分层、事务、`dry-run`、日志、审计和测试。
- 新增能力时同步更新 CLI 与文档，避免知识只留在代码里。

## 2. 对外契约

后续修改 HTTP API 或 CLI 时，必须优先保持当前 Go 契约稳定：

- `Method + Path` 不随意变化。
- query/body 字段名、大小写、默认行为保持稳定。
- 响应外壳保持 `code/message/data/request_id`。
- 关键错误语义保持一致，尤其是 `401/403/404/409`。
- 前端可感知的分页、排序、权限、状态流转语义不漂移。
- CLI 面向脚本消费的 `--json` 输出字段不随意破坏。

确实需要变更契约时，必须同步更新 handler、CLI、测试和文档，并在变更说明中写明兼容性影响。

## 3. 当前核心能力

| 模块 | 当前能力 |
|---|---|
| Auth/User | 登录、会话、当前用户、公开用户、注册、更新、密码、头像 |
| Library | 资料库滚动列表、创建、更新、删除 |
| Node | 创建、查询、树关系、路径、移动、重命名、回收站、归档批量能力 |
| File/Directory | 文件上传、目录上传、链接生成、批量链接 |
| Tag | 类型、列表、创建、更新、删除 |
| Browser | 浏览器文件映射、浏览器书签树、匹配、导入和编辑 |
| CLI | `of` 命令覆盖 health/auth/lib/fs/browser/config 等域 |

当前 API 契约状态见：`docs/progress/go-api-contract-status.md`。

## 4. 分层基线

当前采用轻量分层，依赖方向是：

```text
transport -> usecase -> domain(port) <- repository(impl)
```

职责边界：

- `transport`：只做协议绑定、响应映射、错误码映射，不写业务规则。
- `usecase`：业务编排、权限、审计、事务边界、`dry-run` 控制。
- `domain`：稳定业务对象、值对象、领域错误和端口抽象。
- `repository`：具体存储实现，封装 SQL、缓存、对象存储和错误映射。

更细目录规范见：`docs/architecture/layered-structure-spec.md`。

## 5. 数据与存储

- 当前主数据库路线是 PostgreSQL。
- 常规 CRUD 优先使用 GORM 链式查询或 gorm/gen query。
- 复杂场景（递归、复杂聚合）允许 Raw SQL，但必须集中收口并参数化。
- 事务边界在 usecase 层定义。
- repository 通过 `context.Context` 感知并复用事务。
- 对象存储默认使用 MinIO；新增 provider 时必须显式实现，不允许静默回退。
- Redis 会话能力必须通过 repository 端口封装，usecase 不直连 Redis client。

## 6. 写链路与 dry-run

所有持久化写链路必须支持或显式评估 `dry-run`：

- `dry-run` 与真实执行共用同一套业务校验链路。
- 差异只能出现在提交事务、外部副作用执行或最终落库阶段。
- MinIO、Redis、异步任务等外部副作用在 dry-run 下不得真实执行。
- 日志和审计必须能区分 `mode=dry-run` 与 `mode=execute`。
- CLI 写命令默认支持 `--dry-run` 并透传给后端。

详细契约见：`docs/progress/cli-dry-run-master-plan.md`。

## 7. 持续回归重点

后续改动时优先补回归：

1. `nodes/search`：空 keyword、ANY/ALL、limit 截断。
2. Node 移动与排序：beforeNode 自指 no-op、间隔排序重排。
3. Node root 自修复：坏父引用回根。
4. User avatar：MIME/扩展名校验、预签名链接时效。
5. Tag owner/global 查询与唯一性冲突。
6. Browser bookmark import：结构化导入、排序、dry-run 输出。

## 8. Agent 工作方式

推荐流程：

1. 先看本交接文档、API 契约状态和相关模块文档。
2. 判断影响范围：HTTP、CLI、usecase、repository、docs、tests。
3. 优先保持对外契约稳定，再做内部实现优化。
4. 写链路必须确认事务、`dry-run`、日志、审计和外部副作用。
5. 改完代码后执行必要测试，并根据 `docs/architecture/documentation-standard.md` 更新文档。

一句话总结：代码、测试和文档必须一起维护当前 Go 契约。
