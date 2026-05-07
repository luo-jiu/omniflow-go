# 直传 MinIO 上传链路（终态）

更新时间：2026-05-07
状态：已落地（前端 + 后端 + CLI）

## 1. 背景与决策

OmniFlow 的上传链路从 proxy 模式（client → backend → MinIO）切换为客户端直传 MinIO。原因：
- proxy 模式下后端 SDK 内部 buffer + 4 路并发让 ProgressReader 字节计数“领先于”真实网络出口字节，对远端 MinIO 的进度不准；
- 多客户端、CLI 也要走同一套上传链路，proxy 模式下后端进程要把所有字节都过一遍，资源浪费且不可扩展；
- 直传配 S3 multipart + presigned URL 是行业标准，跨厂商可移植，未来切其他 S3 兼容存储零成本。

终态决策（已锁定）：
- **协议**：S3 Multipart Upload + Presigned PUT URL；不做秒传（私有空间无去重收益）。
- **会话存储**：Postgres `upload_sessions` 表（多实例就绪、崩溃可恢复）。
- **续约模型**：双层 TTL：会话 lease（DB `expires_at`，24h 可续）+ presigned URL 签名（1h 不可改，按需重发）。
- **客户端覆盖**：Electron 主进程 + CLI 共用同一套后端 7 端点。
- **proxy 旧链路**：上传相关的 `UploadAndCreateNode` / multipart proxy / 进度轮询基础设施 整套删除。
- **存储 key 命名**：`libraries/{libraryId}/{uuid}.{ext}` 由后端 `init` 阶段生成，客户端不可写——避免越权。

## 2. API 契约

| Method | Path | 说明 |
|---|---|---|
| POST | `/api/v1/upload/init` | 创建会话，返回 `uploadId / storageKey / mode / partSize / totalParts / expiresAt` |
| POST | `/api/v1/upload/parts/sign` | 颁发分片预签名 URL（默认 1h），隐式续约 lease |
| GET | `/api/v1/upload/parts?uploadId=…` | 透传 MinIO ListParts，断点续传支持 |
| POST | `/api/v1/upload/:uploadId/renew` | 心跳续约，仅刷 lease 不签 URL |
| POST | `/api/v1/upload/complete` | 提交分片清单（multipart）/ 校验对象（single），创建 node |
| DELETE | `/api/v1/upload/:uploadId` | MinIO Abort + 删 session 行 |

**模式分流**：`fileSize ≤ 16 MiB` → `single`（1 个 PUT，无 InitiateMultipart）；否则 `multipart`，`partSize=16 MiB`。

**conflictPolicy** 在 `complete` 阶段透传给 `NodeUseCase.Create`，支持 `error / auto_rename / replace`。Session 表不存策略，避免 schema 漂移。

**鉴权语义**：
- 所有端点校验 actor 与 `upload_sessions.actor_id` 一致；
- 不一致 / session 不存在统一返回 `404`（防 uploadId 枚举）；
- lease 过期返回 `410 Gone`（客户端需要重新 init）。

## 3. 双层 TTL 模型

```
init  ──→ lease 24h, no URL
sign  ──→ URL 1h, lease 顺手刷新到 now+24h
PUT  (URL: 1h) ──→ MinIO 直接验签
renew ──→ lease 顺手刷新（心跳兜底，不签 URL）
complete / abort ──→ 删 session 行
```

- **lease（DB 字段）**：粗粒度的“会话还活着”信号。前端心跳每 8h 一次（TTL/3）显式 renew；任何 sign / list-parts 请求都会顺手刷。
- **URL 签名**：细粒度的“这个 URL 还能用”信号。无状态，不可改。过期或 5xx 时客户端重新 sign，不需重新 init。

后端不需要持久化签名状态，所有 URL 校验由 MinIO 自己完成。

## 4. 客户端覆盖

- **Electron 主进程**：`electron/ipc/http.ts::http:upload:presigned-put` IPC handler。文件用 `fs.createReadStream({start, end})` 切片 + `https.request({method:'PUT', headers:{'Content-Length': ...}})` 流式 PUT。进度通过 80 ms throttle 的 `http:upload:progress` 事件回报，渲染进程按 partNumber 合并。
- **渲染进程**：`src/modules/upload-center/services/upload-direct.ts::runDirectUpload` 是共享流程：init → 4 并发 sign+PUT → complete，心跳 + abort 都在这里收口。`UploadManager.defaultExecutor` 和 `uploadLocalPathAndCreateNode` 共用这条链路。
- **CLI**：`of upload file --library-id N --file PATH [--parent-id N] [--storage-provider X] [--conflict-policy P]`。命令在 `internal/transport/cli/command_upload.go`，client 方法在 `internal/transport/cli/client.go::Upload*`。捕 SIGINT 触发 `UploadAbort`。

`UploadTaskInput` 形状不动 → 嗅探 / 浏览器下载 / 自动导入链路（`useResourceImportToLibrary`、`useEmbeddedBrowserDownloadImport`、`auto-import/runner`、`useDirectoryUpload`）零改动。

## 5. Janitor

`UploadSessionUseCase.NewWithJanitor` 启动 5 min ticker：
- 扫 `repo.ListExpiredBefore(now)`；
- multipart session 调 MinIO `AbortMultipartUpload` 回收已上传分片；
- `repo.Delete(uploadId)` 删 session 行。

模式直抄 `multipart_upload.go` janitor pattern，bootstrap 注册 stop 钩子。

## 6. 数据访问

- `upload_sessions` 表的所有 CRUD 走 gorm/gen 生成的 query 方法（`q.UploadSession.WithContext(ctx)…`）；
- 唯一例外是迁移文件 `docs/schema/2026-05-07-upload-sessions.sql`；
- 事务边界放在 usecase 层：`Complete` 内的 “MinIO complete + node 创建 + session 删除” 不原子化（节点 audit 已经容错），单点失败 janitor 兜底。

## 7. 不变约束

1. `UploadTaskInput` / `UploadManagerEvent` 形状不变 → UI / 嗅探 / 下载链路 0 改动。
2. 客户端持有 `uploadId`、`storageKey` 后端拥有 → 切其他 S3 厂商只换 MinIO 实现。
3. `parts:[{partNumber, etag}]` complete 协议 → AWS S3 / Aliyun OSS / Tencent COS / Cloudflare R2 全兼容。
4. `ObjectStorage` 是 port → 未来加 STS 临时凭证只换 sign 内部，对外契约不变。
5. 续约抽象到 lease（DB）+ URL（无状态）两层 → 切 Redis lease 只动 repo。
6. CLI 与 Electron 共用同一套后端流程 → 后端动一处，所有客户端受益。

## 8. 已删除的旧链路

| 删除项 | 替代 |
|---|---|
| `internal/uploadprogress/` | DB lease + 直传字节进度（前端聚合） |
| `internal/repository/progress/` | 同上 |
| `internal/usecase/upload_progress_reader.go` | 同上 |
| `internal/usecase/multipart_upload.go` | `internal/usecase/upload_session.go` |
| `internal/transport/http/handler/upload_progress.go` | 直传 7 端点 |
| `internal/transport/http/handler/multipart_upload.go` | 同上 |
| `internal/transport/http/router/routes_multipart_upload.go` | `routes_upload.go` |
| `directory.go::UploadAndCreateNode` | `upload_session.go::Complete` |
| 前端 `upload-progress.api.ts` | `upload-session.api.ts` + `upload-direct.ts` |
| 前端 `uploadAndCreateNode` | `runDirectUpload` |
| `electron/ipc/http.ts::http:upload`（chunked proxy） | `http:upload:presigned-put` |

`http:upload:formdata` 保留：仅服务于头像这类小文件、走后端 `POST /api/v1/files/upload` 代理保存的旧链路。
