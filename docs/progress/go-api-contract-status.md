# Go API 契约状态摘要

更新时间：2026-05-12
状态：Go API 当前契约已收口，持续维护

## 1. 当前结论

后续新增或修改接口时，必须以当前 Go API 契约为准。

Go 后端当前 `/api/v1` 接口功能已覆盖核心业务，并包含 Go 侧扩展能力。

| 指标 | 数量 | 说明 |
|---|---:|---|
| `/api/v1` 接口总数 | 56 | 以当前路由注册为准 |
| 功能实现 | 56/56 | 含兼容 no-op 1 个 |
| 日志 P1 接入 | 56/56 | 详见 API 与日志归档摘要 |
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
| Tag | 类型、列表、创建、更新、删除；标签定义支持 `scope / dimension / resourceKind` 多维字段 |
| Browser | 文件映射、书签树、匹配、导入、创建、更新、移动、删除 |
| Storage Migration | 入队 / 列表 / 详情 / 取消 / 子项 / 存储分布 |
| Resource Monitor | 当前用户资料库范围的物理存储分布快照和只读资源探针 |
| Health | 服务健康检查 |

## 4. Go 扩展能力

Go 当前能力包含以下扩展能力，后续应按 Go 自身契约维护：

- `GET /api/v1/health`
- `GET /api/v1/resource-monitor/snapshot`
  - 返回当前 actor 拥有资料库范围内的资源分布快照；支持可选 `libraryId` 查询参数收敛到指定资料库。
  - `summary` 包含 `providerCount / bucketCount / objectCount / fileRefCount / physicalBytes / visible* / recycle* / orphan* / unmatchedCount / legacyProviderCount`。
  - `storage` 按 provider / bucket 聚合，包含 `provider / sourceProvider / providerType / providerLabel / endpoint / bucket / isDefault / isLegacyProvider / objectCount / fileRefCount / physicalBytes / visible* / recycle* / orphan* / percent / matchedConfig`。
  - `distributionError` 表示资源分布统计失败时的脱敏错误摘要；失败时接口仍返回 partial snapshot 和探针结果。
  - `probeSummary` 和 `probes` 返回对象存储、Postgres、Redis 的只读探针状态、耗时和错误摘要。
  - 当前接口只读；对象存储探针只检查 bucket 可访问性，不创建对象、不清理数据。
- `GET /api/v1/resource-monitor/distribution`
  - 返回资源分布字段；支持可选 `libraryId`，范围语义与 snapshot 一致。
- `GET /api/v1/resource-monitor/breakdown`
  - 返回资源细分仪表盘字段；支持可选 `libraryId`，范围语义与 distribution 一致。
  - `summary / libraries / categories / statuses / anomalies` 分别表示总览、资料库排行、归档分类、资源状态和只读诊断摘要。
  - 同时区分 `physicalBytes` 物理去重容量和 `referencedBytes` 引用展开容量。
- `GET /api/v1/resource-monitor/dashboard`
  - 返回资源监测 V2 仪表盘字段；支持可选 `libraryId`，范围语义与 breakdown 一致。
  - `fileTypes` 表示基础文件类型分布，`collections` 表示业务集合类型分布，`collectionFileTypeMatrix` 表示业务集合 x 基础文件类型交叉统计。
  - 当前作为并行 V2 接口提供，不替换旧 `/breakdown`；它不执行探针，探针仍由 `/probes` 独立返回。
- `GET /api/v1/resource-monitor/probes`
  - 返回对象存储、Postgres、Redis 的只读探针字段；供前端和分布统计并行加载。
- `POST /api/v1/resource-monitor/samples`
  - 显式写入一条资源监测历史采样；支持可选 `libraryId` 查询参数，范围语义与 snapshot 一致。
  - 带 `libraryId` 时会先校验资料库归属；不属于当前 actor 时返回 not found 且不写入空样本。
  - 支持 `dryRun=true` 返回样本预览但不持久化。
  - 样本 summary 字段用于后续趋势查询，完整快照保存到 `resource_monitor_samples.snapshot_json`。
  - 当前不启动后台定时采样，不自动告警。
- `POST /api/v1/directory/links/batch`
- `PUT /api/v1/nodes/:nodeId/content`
  - 按节点 ID 原地替换文件内容，请求体使用 `libraryId`、`content`、可选 `contentType` 和 `storageProvider`。
  - 后端生成新对象并更新 `storage_objects` / `node_files`，保留节点 ID、目录位置和文件名不变。
  - `storageProvider` 只在文件节点尚未绑定对象存储时生效，用于右键新建文件这类首次写入场景；已有存储绑定的文件保存时继续沿用原 provider。
  - 若文件节点来自右键新建、尚未绑定 `node_files` / `storage_objects`，首次写入会初始化空文件对象，之后可正常获取预签名链接。
  - 支持 `dryRun` 查询参数，dry-run 只做校验，不写对象存储和数据库。
- `PATCH /api/v1/nodes/move/batch`
  - 归档目录允许作为移动目标；移动接口只保留跨库、移动到自身 / 子节点、同目录可见名称冲突等通用安全校验。
- `GET /api/v1/nodes/recycle/library/:libraryId`
  - 返回回收站顶层条目，文件条目会包含物理存储摘要：`storageProvider`、`storageProviderType`、`storageProviderLabel`、`storageEndpoint`、`storageBucket`、`storageKey`。
  - 目录条目会聚合子树文件的 `storageLocations`，每个位置包含 provider / bucket / endpoint 和文件数量，用于前端展示资源目录实际占用的物理存储。
- `GET /api/v1/nodes/:nodeId/archive/cards`
  - 当前支持 `COMIC` / `ASMR` / `VIDEO` / `AUDIO` 归档卡片查询
  - `COMIC` 当前返回归档目录下的直属漫画单元：直属 `built_in_type=COMIC` 且 `archive_mode=false` 的目录返回为普通漫画卡片，直属 `built_in_type=COMIC` 且 `archive_mode=true` 的子归档返回为 `cardKind=collection` 合集卡片；该规则只看亲子关系，不递归展开孙级，并且 `offset / limit` 在数据库层分页
  - `VIDEO` 当前返回归档目录下的第一代视频单元：优先支持直属 `built_in_type=VIDEO` 且 `archive_mode=false` 的目录，目录内第一个视频文件作为 `mediaNodeId`，第一个图片文件作为 `coverNodeId`，字幕文件通过 `subtitleCount` 计数，媒体时长通过 `durationSeconds` 返回；历史直属视频媒体文件仍兼容返回；有效视频单元的 `offset / limit` 在数据库层分页
  - `VIDEO` 也会返回直属 `built_in_type=VIDEO` 且 `archive_mode=true` 的子归档目录，`cardKind=collection`，用于前端展示“合集”卡片；该规则只看亲子关系，不递归展开孙级；合集封面同样支持 `coverNodeId` 和第一代图片文件探测
  - `AUDIO` 当前返回归档目录下的歌曲单元：优先支持直属 `built_in_type=AUDIO` 且 `archive_mode=false` 的目录，目录内第一个音频文件作为 `mediaNodeId`，第一个图片文件作为 `coverNodeId`，字幕 / 歌词文件通过 `subtitleCount` 计数；历史直属音频媒体文件仍兼容返回且不要求设置内置类型；直属 `AUDIO + archive_mode=true` 子归档不会作为上级合集卡片返回；有效歌曲单元的 `offset / limit` 在数据库层分页
- `PATCH /api/v1/nodes/:nodeId/archive/built-in-type/batch-set`
- Browser file mapping 与 browser bookmark 相关接口
- 直传 MinIO 流程（已替代旧 proxy 整传 / 分片整传 / 进度轮询）：
  - `POST /api/v1/upload/init`：创建会话，返回 `uploadId / storageKey / mode(single|multipart) / partSize / totalParts / expiresAt`。`fileSize ≤ 16 MiB` 走 single；否则 multipart，partSize=16 MiB。`storageKey = libraries/{libraryId}/{uuid}.{ext}` 由后端生成，客户端不可写。
  - `POST /api/v1/upload/parts/sign`：颁发分片预签名 PUT URL（默认 1h），顺手刷新会话 lease 至 `now + 24h`（隐式续约）。
  - `GET /api/v1/upload/parts?uploadId=...`：透传 MinIO ListParts 返回 `partNumber / etag / size`，断点续传支持，顺手刷 lease。
  - `POST /api/v1/upload/:uploadId/renew`：心跳续约，仅刷 lease 不签 URL。
  - `POST /api/v1/upload/complete`：multipart 调 CompleteMultipartUpload，single 校验对象存在；落库由 `NodeUseCase.Create` 兜底，支持 `conflictPolicy=error|auto_rename|replace`。
  - `DELETE /api/v1/upload/:uploadId`：MinIO AbortMultipartUpload + 删 session 行。
  - 鉴权语义：actor 与 session.actor 不一致 / session 不存在统一返回 `404`（防 uploadId 枚举）；lease 过期返回 `410 Gone`。
  - 双层 TTL：DB lease（24h，可续）与 presigned URL 签名（1h，不可改）解耦；URL 过期可重新 sign 而无需重新 init。
  - 客户端覆盖：Electron 主进程 `http:upload:presigned-put` 走 `https.request` 流式 PUT；CLI `of upload file` 走 Go `http.Client` 直传，复用同一套后端流程。
- 存储迁移（migration）：把节点子树下的 `storage_objects` 物理对象从一个 provider 搬到另一个 provider，节点元数据保持不变。
  - `POST /api/v1/migration/tasks`：入队迁移任务，请求体 `{libraryId, rootNodeId, targetProvider}`；支持 `?dryRun=true` 仅返回 `{plannedObjects, plannedBytes, targetProvider, targetBucket, storageObjectIds}` 不落库。
  - `GET /api/v1/migration/tasks?libraryId=&status=&limit=`：列举任务（actor 维度）；`status` 支持逗号分隔。
  - `GET /api/v1/migration/tasks/:id`：单任务详情。
  - `POST /api/v1/migration/tasks/:id/cancel`：取消任务（运行中保留已完成项，pending 子项被 worker 抢到时立刻 skipped）；支持 `?dryRun=true` 仅校验。
  - `GET /api/v1/migration/tasks/:id/items`：子项列表（调试用）。
  - `GET /api/v1/libraries/:libraryId/storage-distribution?nodeId=N`：按 provider 统计当前节点子树文件数 / 字节数，迁移对话框依赖此接口判断"100% 已在该 provider"。
  - 鉴权语义：actor 与任务 actor 不一致 / 任务不存在统一 `404`（防 task_id 枚举）；已终态任务再取消返回 `409`。
  - 数据库迁移脚本：`docs/schema/2026-05-09-migration-tasks.sql`；设计详见 `docs/architecture/storage-migration-design.md`。
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
- Tag 多维基座：
  - `GET /api/v1/tags` 新增可选过滤 `scope / dimension / resourceKind`，原 `type` 过滤保持兼容。
  - `POST /api/v1/tags` 与 `PUT /api/v1/tags/:tagId` 可接收 `scope / dimension / resourceKind / targetKinds`，响应同样返回这些字段。
  - `PUT /api/v1/tags/:tagId` 未携带 `targetKinds` 时保留原绑定策略；携带时才替换可贴对象策略。
  - `FILE_TAB` 会归为 `scope=ui`，资源标签默认 `scope=resource`、`dimension=custom`，常见资源类型会由旧 `type` 推导。
  - `tag_target_kinds / tag_bind_policies / node_resource_targets` 已作为标签可贴对象基座；节点写入 `tagIds` 时会校验标签策略与节点资源语义是否匹配。
  - `POST /api/v1/nodes/search` 的 `tagIds / tagMatchMode` 契约不变，但实现以 `node_tag_rel` 为准；`nodes.view_meta.tagIds` 仅作为兼容输入并在节点更新时同步关系表。
  - 数据库迁移脚本见 `docs/schema/2026-05-06-tag-foundation.sql` 与 `docs/schema/2026-05-07-tag-target-policies.sql`，长期规则见 `docs/architecture/tag-foundation.md`。

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
