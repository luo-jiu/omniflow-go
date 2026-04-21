# Go API 契约状态摘要

更新时间：2026-04-15  
状态：Go API 当前契约已收口，持续维护

## 1. 当前结论

后续新增或修改接口时，必须以当前 Go API 契约为准。

Go 后端当前 `/api/v1` 接口功能已覆盖核心业务，并包含 Go 侧扩展能力。

| 指标 | 数量 | 说明 |
|---|---:|---|
| `/api/v1` 接口总数 | 45 | 以当前路由注册为准 |
| 功能实现 | 45/45 | 含兼容 no-op 1 个 |
| 日志 P1 接入 | 45/45 | 详见 API 与日志归档摘要 |
| CLI 主要写链路 | 已覆盖 | 详见 CLI 进度台账 |

## 2. 保留契约

后续修改已存在接口时，必须保持：

- `Method + Path` 不随意变化。
- query/body 字段名、大小写、默认行为保持一致。
- 响应外壳保持 `code/message/data/request_id`。
- 关键错误语义保持一致，尤其是 `401/403/404/409`。
- 前端可感知的分页、排序、权限、状态流转语义不漂移。
- 写接口的 `dry-run` 语义不漂移。

内部实现可以继续演进，包括数据模型、SQL 写法、事务实现和存储 provider，但不能破坏对外契约。

## 3. 当前覆盖范围

| 模块 | 当前能力 |
|---|---|
| Auth | login/status/logout |
| User | me、公开用户、注册、更新、密码、头像、用户名可用性 |
| Library | scroll/create/update/delete |
| Node | 创建、查询、树关系、路径、移动、重命名、回收站、归档批量能力 |
| File/Directory | 上传、链接、批量链接 |
| Tag | 类型、列表、创建、更新、删除 |
| Browser | 文件映射、书签树、匹配、导入、创建、更新、移动、删除 |
| Health | 服务健康检查 |

## 4. Go 扩展能力

Go 当前能力包含以下扩展能力，后续应按 Go 自身契约维护：

- `GET /api/v1/health`
- `POST /api/v1/directory/links/batch`
- `GET /api/v1/nodes/:nodeId/archive/cards`
  - 当前支持 `COMIC` / `ASMR` / `VIDEO` 归档卡片查询
  - `VIDEO` 当前返回归档目录下的直属视频媒体文件，不要求子文件额外设置 `built_in_type=VIDEO`
- `PATCH /api/v1/nodes/:nodeId/archive/built-in-type/batch-set`
- Browser file mapping 与 browser bookmark 相关接口
- CLI `of` 命令域及其 `--json`、`--dry-run` 契约
- 节点创建与目录上传支持可选 `conflictPolicy`：
  - 默认或 `error`：同一目录下重名时返回 `409`，message 为“同一目录下已存在同名节点”。
  - `auto_rename`：系统插入场景可让后端自动追加序号，规则为 `name`、`name (1)`、`name (2)`；文件扩展名单独保存，序号只追加到文件名主体。
  - 手动重命名和移动仍保持重名即 `409`，不自动改名。

## 5. 建议持续回归

以下场景更容易出现边界差异，后续改动时优先补回归：

1. `nodes/search`：空 keyword、ANY/ALL、limit 截断。
2. Node 移动与排序：beforeNode 自指 no-op、间隔排序重排。
3. Node root 自修复：坏父引用回根。
4. User avatar：MIME/扩展名校验、预签名链接时效。
5. Tag owner/global 查询与唯一性冲突。
6. Browser bookmark import：结构化导入、排序、dry-run 输出。

## 6. 维护规则

出现以下情况必须更新本文档：

- `/api/v1` 接口数量发生变化。
- 对外请求或响应契约发生变化。
- CLI 新增可操作能力，需要同步说明 API 覆盖关系。
- 新增跨模块回归风险，需要纳入持续回归清单。
