# Go API 契约状态摘要

更新时间：2026-05-05
状态：Go API 当前契约已收口，持续维护

## 1. 当前结论

后续新增或修改接口时，必须以当前 Go API 契约为准。

Go 后端当前 `/api/v1` 接口功能已覆盖核心业务，并包含 Go 侧扩展能力。

| 指标 | 数量 | 说明 |
|---|---:|---|
| `/api/v1` 接口总数 | 46 | 以当前路由注册为准 |
| 功能实现 | 46/46 | 含兼容 no-op 1 个 |
| 日志 P1 接入 | 46/46 | 详见 API 与日志归档摘要 |
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
- `PUT /api/v1/nodes/:nodeId/content`
  - 按节点 ID 原地替换文件内容，请求体使用 `libraryId`、`content`、可选 `contentType`。
  - 后端生成新对象并更新 `storage_objects` / `node_files`，保留节点 ID、目录位置和文件名不变。
  - 若文件节点来自右键新建、尚未绑定 `node_files` / `storage_objects`，首次写入会初始化空文件对象，之后可正常获取预签名链接。
  - 支持 `dryRun` 查询参数，dry-run 只做校验，不写对象存储和数据库。
- `PATCH /api/v1/nodes/move/batch`
  - 归档目录允许作为移动目标；移动接口只保留跨库、移动到自身 / 子节点、同目录可见名称冲突等通用安全校验。
- `GET /api/v1/nodes/:nodeId/archive/cards`
  - 当前支持 `COMIC` / `ASMR` / `VIDEO` / `AUDIO` 归档卡片查询
  - `VIDEO` 当前返回归档目录下的第一代视频单元：优先支持直属 `built_in_type=VIDEO` 的目录，目录内第一个视频文件作为 `mediaNodeId`，第一个图片文件作为 `coverNodeId`，字幕文件通过 `subtitleCount` 计数，媒体时长通过 `durationSeconds` 返回；历史直属视频媒体文件仍兼容返回
  - `AUDIO` 当前返回归档目录下的直属音频媒体文件，不要求子文件额外设置 `built_in_type=AUDIO`
- `PATCH /api/v1/nodes/:nodeId/archive/built-in-type/batch-set`
- Browser file mapping 与 browser bookmark 相关接口
- CLI `of` 命令域及其 `--json`、`--dry-run` 契约
- 节点创建与目录上传支持可选 `conflictPolicy`：
  - “同名”按用户可见名称判断：目录为 `name`，文件为 `name.ext`（无后缀文件仍为 `name`）。因此同一目录允许 `demo.txt` 与 `demo.md` 共存，但不允许两个 `demo.txt`。
  - 默认或 `error`：同一目录下可见名称重复时返回 `409`，message 为“同一目录下已存在同名节点”。
  - `auto_rename`：系统插入场景可让后端自动追加序号，规则为 `name`、`name (1)`、`name (2)`；文件扩展名单独保存，序号只追加到文件名主体，并只针对同一可见文件名冲突生效。
  - `replace`：同名同后缀文件已存在时，替换其存储内容（更新 `storage_objects` 和 `node_files`），保留原节点 ID 不变；若未找到同名同后缀文件则回退为新建。用于兼容旧上传替换链路；文档编辑器保存优先使用 `PUT /api/v1/nodes/:nodeId/content`。
  - 手动重命名和移动仍保持可见名称重复即 `409`，不自动改名。
  - 数据库唯一索引也必须按同一可见名称语义维护，不能只用 `name` 判断文件节点冲突；迁移脚本见 `docs/schema/2026-05-03-node-visible-name-and-storage-provider.sql`。
- `GET /api/v1/nodes/:nodeId`
  - 文件节点详情会返回物理存储位置：`storageProvider`、`storageProviderType`、`storageProviderLabel`、`storageEndpoint`、`storageBucket`、`storageKey`。
  - `storageProvider` 持久化 provider 别名（例如 `local-minio`、`win-minio`），用于区分同为 MinIO 的不同机器或不同桶，也用于后续 S3 / OSS 等多存储位置的无感切换；历史类型值（如 `MINIO`）仅保留兼容读取能力。
  - `storageEndpoint` 来自当前 `configs/storage.yaml` 快照，不在数据库中重复持久化；密钥不通过节点详情接口返回。
  - 若历史 `storage_objects.provider` 存的是标准类型值，必须先人工确认该类型只对应一个真实 provider 后再显式迁移到 alias；服务启动过程不会自动把历史类型值改写为默认 alias，避免误指向错误对象存储。

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
