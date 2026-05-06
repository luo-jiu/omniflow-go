# 客户端直传 MinIO 迁移路径

更新时间：2026-05-06
前置阅读：`docs/architecture/upload-progress-design.md`

本文档说明：当 OmniFlow 决定从 proxy 上传切到客户端直传 MinIO 时，**当前进度方案需要改什么、可以删什么、为什么这么设计**。读这份文档的人可能距离当前实现已经过了很久；目的是让"未来翻这份代码的人"能快速判断哪些抽象是为这次迁移预付的，不会被吓到也不会误删。

## 1. 当前架构是 proxy 模式

```
Client ──► Backend ──► MinIO
       (form/multipart)   (S3 SDK)
```

后端是字节流的中转：

- 单次写入：客户端 → 后端 multipart form → 后端走 S3 PUT。
- 大文件分片：客户端 → 后端 `Initiate/UploadPart/Complete` → 后端在 S3 上做 multipart。

进度跟踪问题来自这个架构本身：客户端只能看到 client→backend 那段。当前方案（`upload-progress-design.md`）通过服务端轮询暴露 backend→MinIO 的真实写入。

## 2. 直传后架构

```
Client ──── presigned URL / S3 SDK ──► MinIO
   │                                      │
   └──── notify ─► Backend ──── (db) ─────┘
                  (only metadata/audit)
```

后端职责变化：

- 只颁发 presigned URL（或 STS token）；不再处理字节流。
- 完成后由客户端通知后端落库（创建/更新 node、写 audit）。
- 进度由客户端 XHR `progress` 事件 / S3 SDK 回调天然提供，**根本不需要服务端进度跟踪**。

## 3. 哪些当前抽象是为这次迁移预付的

设计当前方案时刻意保留的"零成本可删"抽象：

| 抽象 | 目的 | 直传时怎么处理 |
|---|---|---|
| 客户端持有 `uploadId`（UUID） | 不依赖服务端生成 | 保留：直传 multipart 也用同一个 ID 关联 part |
| `complete` 契约 `parts:[{partNumber, etag}]` | 兼容 S3 multipart 标准 | 保留：直传后由客户端把 etag 列表 POST 给后端落库 |
| `UploadProgressTracker` 是 port | 实现可换 | 替换为 no-op 实现，或者从 DI 链路里整体删除 |
| 前端 `UploadManagerEvent.PROGRESS` 不绑定来源 | UI 不感知数据来源 | 把 executor 内部的"轮询服务端"换成"监听 XHR `progress` 事件" |
| `wrapProgressReader` 在 `io.Reader` 层注入 | 不入侵 storage 实现 | 整段删除：后端不再持有字节流 |

## 4. 三步走迁移清单

### 步骤一：颁发 presigned URL

- 在 `internal/usecase` 新增 `UploadPresignUseCase`：
  - 整传：`POST /api/v1/upload/presign-direct`，返回 `{uploadUrl, headers, storageKey, expiry}`。
  - 分片：扩展现有 `Initiate` 返回 `{uploadId, storageKey, parts:[{partNumber, presignedUrl}]}`，让客户端按 part 直接 PUT。
- 后端不再读字节流，但需要保留对 `storageKey` 的 namespace 控制（`libraries/{id}/...` 不可由客户端伪造）。

### 步骤二：客户端落库通知

- `POST /api/v1/upload/complete-direct`：
  - 整传：`{uploadId, libraryId, parentId, fileName, fileSize, contentType, storageKey}` → 后端 HEAD 一次确认对象存在 → 创建 node。
  - 分片：`{uploadId, parts:[{partNumber, etag}]}` → 后端调 S3 `CompleteMultipartUpload`（这一步仍由后端发起，避免客户端持有完整 S3 写权限）→ 创建 node。
- `parts` 契约不变，客户端实现成本最低。

### 步骤三：删除 proxy 进度方案

可以一次性删除：

- `internal/uploadprogress/`（port 整段）
- `internal/repository/progress/`（内存实现）
- `internal/usecase/upload_progress_reader.go`
- `internal/transport/http/handler/upload_progress.go`
- `internal/transport/http/router/routes_upload.go`
- `internal/usecase/directory.go` 内 `tracker.Register/Done` 与 `wrapProgressReader` 调用
- `internal/usecase/multipart_upload.go` 内同上
- `internal/bootstrap/wire*.go` 内 `NewUploadProgressTracker` 与 `NewUploadProgressHandler` 注入
- 前端 `src/modules/upload-center/services/upload-progress.api.ts`
- 前端 `src/utils/uploadManager.ts` 中 `pollUploadProgress` 调用与 monotonic 拼装

或者保留 port + no-op 实现，把"是否启用"放成配置——但通常不值得，"零成本可删"就是为了真删。

## 5. 不变的契约

无论是否切到直传，以下契约必须不变（前后端都依赖）：

- `UploadFileCommand` / `InitiateMultipartUploadCommand` 的可见字段语义。
- `parts:[{partNumber, etag}]` 的 wire 结构。
- 节点最终落库后的 `storageKey` 命名规则 `libraries/{id}/{uuid}.{ext}`。
- `409` / `404` / `403` / `400` 的错误语义。

## 6. 为什么不直接做直传？

当前阶段不直接做直传，是因为：

1. MinIO 部署形态可能多端 / 跨网，presigned URL 的可达性需要逐机器验证。
2. 客户端持有 S3 写权限会改变安全模型，需要审计 / 限速 / 防滥用一整套配套。
3. proxy 模式下"backend 即 trust boundary"的简单性，对当前阶段足够好。

进度方案选择"零成本可删"的轮询路径，就是给未来留路：当前问题先解决，未来切换时不留旧债。

## 7. 相关文档

- 当前进度方案：`docs/architecture/upload-progress-design.md`
- API 契约：`docs/progress/go-api-contract-status.md`
- 前端架构：`omniflow-app/docs/upload-progress-architecture.md`
