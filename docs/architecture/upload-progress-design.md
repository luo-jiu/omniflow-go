# Upload Progress 设计（Proxy 模式服务端真进度）

更新时间：2026-05-06
关联代码：
- `internal/uploadprogress/tracker.go`（端口）
- `internal/repository/progress/memory.go`（内存实现）
- `internal/usecase/upload_progress_reader.go`（字节流插入点）
- `internal/usecase/directory.go`（整传写链路）
- `internal/usecase/multipart_upload.go`（分片写链路）
- `internal/transport/http/handler/upload_progress.go`（查询端点）

## 1. 问题背景

OmniFlow 当前上传链路是 Electron 客户端 → Go 后端 → MinIO 的 proxy 模式。客户端只能感知 client→backend 这一段字节流；当 IPC 写完后，UI 进度立刻封顶到 100%，但 backend→MinIO 的对象写入还在继续。结果用户看到的"完成"和真实落库之间存在不可见的等待，体感是"卡在 100%"。

要解决这个问题，必须让客户端能感知 backend→MinIO 这段的真实进度。

## 2. 方案选型与决定

| 维度 | 选项 | 决定 | 理由 |
|---|---|---|---|
| 覆盖范围 | 仅整传 / 仅分片 / 全覆盖 | 全覆盖 | 大文件分片是最容易出现"卡在 100%"的场景 |
| 存储 | 单实例内存 / Redis | 单实例内存 | 后端单实例部署假设；不引入新的 ops 依赖；切到直传后整段可下线 |
| 传输 | 轮询 / SSE / WebSocket | 轮询 | 未来直传后整段会下线，要"零成本可删"；不引入长连接基础设施 |
| 客户端 uploadId | 复用 IPC taskId / 独立 UUID | 独立 UUID | 不把客户端内部状态泄漏到 wire 协议 |
| 完成契约 | 修改 / 不变 | 不变 | 直传迁移时 `parts:[{partNumber, etag}]` 仍可复用 |
| UI 绑定 | 绑定服务端事件 / 解耦 | 解耦 | UI 只消费 `onProgress(bytes, speed?)`，未来可切到 XHR `progress` 事件 |
| CLI 能力面 | 接入 / 豁免 | 豁免 | 进度查询是内部辅助接口，非用户能力面，按 `omniflow-go/AGENTS.md` 精神可豁免 |

## 3. 架构概览

```
                              proxy 写链路
   Electron Client ──── upload_id ───► Backend ──── ProgressReader ───► MinIO
        │                              │   │
        │                              │   └── tracker.Add(uploadID, n)
        │                              ▼
        │                       Memory Tracker (single instance)
        │                              ▲
        └─── poll GET /upload/:id ─────┘
                  (500ms)
```

关键不变量：

- `upload_id` 由客户端生成（UUID），随 form / multipart `Initiate` 一起送达。
- 后端通过 `wrapProgressReader` 在 `io.Reader` 层拦截累加，**不感知具体存储后端**。
- 查询端点按 actor 校验：uploadID 不存在与 actor 不匹配返回相同 404，避免 uploadID 枚举。

## 4. 模块责任

### 4.1 `internal/uploadprogress`（port）

- 定义 `Tracker` 接口（`Register / Add / Get / Done`）和 `Progress / State` 类型。
- 不持有实现细节，便于切到 Redis、no-op 或 mock。
- 哨兵错误：`ErrNotFound`。"actor 不匹配"也映射为 `ErrNotFound`，由实现内部完成。

### 4.2 `internal/repository/progress.MemoryTracker`

- `sync.RWMutex` + `map[string]*entry`。参考点 `internal/usecase/multipart_upload.go:99` 的 `sessions` map 写法。
- TTL 设计：
  - `doneTTL = 30s`：`Done` 后保留，让前端最后一次轮询能拿到 `state==='done'`。
  - `doneTTL * 4 = 2min`：长时间没收到 `Add` 也没收到 `Done` 的 running 会话视作僵尸清理。
- `Done` 把 `uploaded` 拉齐到 `total`，避免最后一帧因尾部累加未到达而显示 99%。
- `NewMemoryTracker` 返回 cleanup 函数；调用方在程序退出时关闭 janitor goroutine。

### 4.3 `internal/usecase/upload_progress_reader.go`

- `wrapProgressReader(r, tracker, uploadID)`：为空或 nil 时透传，确保旧客户端、单测、no-op 模式都能正常工作。
- 在 `Read` 上累加，单次系统调用粒度，开销可忽略。

### 4.4 写链路插入点

| 链路 | 文件 | 位置 |
|---|---|---|
| 整传 | `internal/usecase/directory.go` | `UploadAndCreateNode` 内 `wrapProgressReader(contentReader, ...)` 后传给 `store.Upload` |
| 分片 | `internal/usecase/multipart_upload.go` | `Initiate` 时一次性 `Register`；`UploadPart` 在 `store.UploadPart` 前 wrap reader；`Complete` / `Abort` / 过期清理时 `Done` |

整段进度共用同一个 uploadID；分片在 `Initiate` 注册一次，`UploadPart` 累加，`Complete/Abort/sweepExpiredSessions` 收尾。

### 4.5 查询端点

`GET /api/v1/upload/:uploadId/progress`

- 不挂在 `directory/upload/multipart` 子分组下，整传 / 分片共用。
- 不需要 `extendUploadTimeout`，进度查询是短请求。
- 返回字段：`uploadId / totalBytes / uploadedBytes / percentage / state`。
- 错误语义：缺 uploadId → 400；不存在或 actor 不匹配 → 404；其他 → 500。

## 5. 鉴权与安全

- uploadID 是 UUID，碰撞概率忽略；不可枚举。
- `Tracker.Get` 内部对比 actor，越权访问统一返回 `ErrNotFound`。
- 路由经标准 Auth 中间件，未登录请求在到达 handler 前已被 401 拦截。
- 不做审计：进度查询是高频读，写审计会形成无意义的洪流。

## 6. CLI 豁免说明

`omniflow-go/AGENTS.md` 要求新接口要有 CLI 落地。本接口豁免，理由：

- 进度查询是高频内部辅助接口，不属于"用户能力面"。
- CLI 没有"轮询观察某个上传"的现实场景；交互式上传场景已被 `of upload` 体系覆盖。
- 直传迁移后整段下线，CLI 命令也会同步删除，预付投入会被立即作废。

如果未来出现"运维需要观察长时间上传"等场景，再补 `of upload progress :id` 即可，本豁免不阻塞。

## 7. 已知限制与边界

- **单实例假设**：内存 map 不跨实例同步。后端要扩成多实例时本包必须废弃或切到 Redis。
- **不持久化**：进程重启后所有进度丢失；客户端 404 后会兜底回 IPC 字节，UX 退化但不报错。
- **不写审计**：进度查询不会留下任何审计记录。
- **不做断点续传感知**：当前接口只反映"本次会话累计写入字节"，不追踪历史会话。

## 8. 验证矩阵

| 场景 | 预期 |
|---|---|
| 整传 ≥100MB | UI 进度越过 IPC 100% 时点继续平滑推进 |
| 分片多 part | 跨 part 累加，无回退 |
| 网络中断重试 | 后端 `Done` → 30s TTL → 客户端最后一帧能看到 done |
| actor 不匹配 | 返回 404 而非 403 |
| uploadId 缺失 | 返回 400 |
| 服务端 5xx | 客户端 monotonic 保护，不回退；恢复后继续 |

## 9. 相关文档

- 未来直传迁移路径：`docs/architecture/upload-direct-upload-migration.md`
- 前端轮询接入：`omniflow-app/docs/upload-progress-architecture.md`
- API 契约登记：`docs/progress/go-api-contract-status.md`
