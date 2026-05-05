# 节点元数据契约

更新时间：2026-05-05

适用范围：`nodes.view_meta`、归档卡片接口、漫画 / 视频 / ASMR 等 viewer 读写节点状态和文件固有元数据的后端实现。

## 1. 概述

OmniFlow 当前还没有独立的 `node_metadata` 表，节点扩展数据统一暂存在 `nodes.view_meta` JSONB 中。为了避免不同 viewer 各自发明 key，后续必须按“文件固有元数据”和“视图状态”分层管理。

当前约定：

- 文件固有元数据写入 `__omniflowNodeMetadataV1`
- viewer 状态继续写入 `__omniflowViewerStateV1`
- 业务标签等旧顶层字段暂时保持兼容，不在本轮迁移

## 2. 背景

视频时长、音频时长、图片尺寸、页数、编码信息等数据不应该由前端页面反复读取媒体文件得到。它们通常在入库、扫描、归档卡片 warmup 或后台任务中由后端提取，随后作为稳定字段提供给前端。

漫画阅读位置、视频播放位置、归档滚动位置这类数据会随用户操作频繁变化，属于 viewer 状态，不属于文件固有元数据。

## 3. 核心概念

### 3.1 文件固有元数据

文件固有元数据描述文件本身，一般不随用户观看行为变化。当前稳定命名空间：

```json
{
  "__omniflowNodeMetadataV1": {
    "media": {
      "durationSeconds": 123.456,
      "source": "ffprobe",
      "probedAt": "2026-05-05T12:00:00Z"
    }
  }
}
```

规则：

- `durationSeconds` 使用秒，允许小数
- `source` 记录提取来源，当前视频归档 warmup 使用 `ffprobe`
- `probedAt` 使用 UTC RFC3339 / RFC3339Nano
- 后续图片尺寸、音视频 codec、页数等都应先进入该命名空间，再考虑是否拆到专表

### 3.2 视图状态

视图状态描述用户如何看这个文件，可能频繁更新。当前稳定命名空间：

```json
{
  "__omniflowViewerStateV1": {
    "comicReader": {},
    "videoPlayer": {},
    "comicArchiveReader": {},
    "asmrArchiveReader": {}
  }
}
```

规则：

- 阅读位置、播放位置、归档滚动位置写在 viewer state 下
- 不能把文件时长、图片宽高、页数这类固有信息写进 viewer state
- 清理 legacy key 前必须先确认历史数据迁移

## 4. 契约

### 4.1 `GET /api/v1/nodes/:nodeId/archive/cards`

`builtInType=VIDEO` 卡片会返回：

- `mediaNodeId`：真正的视频文件节点
- `coverNodeId`：封面文件节点
- `subtitleCount`：伴随字幕数量
- `durationSeconds`：从媒体节点 `__omniflowNodeMetadataV1.media.durationSeconds` 读取的视频时长

如果时长缺失，后端会在归档卡片返回后调度 best-effort warmup，不阻塞本次列表响应：

- 每次请求最多处理 6 个缺失项
- 需要可用的 `ffprobe`，可通过 `OMNIFLOW_FFPROBE_PATH` 指定
- 单个探测超时为 4 秒
- 探测成功后写回媒体节点 `view_meta`
- 探测失败不影响卡片列表返回；首次响应只返回已经存在的 `durationSeconds`

### 4.2 `PUT /api/v1/nodes/:nodeId`

该接口仍允许前端整体更新 `viewMeta`，但前端写入时必须保留未知 key，不能覆盖 `__omniflowNodeMetadataV1` 或其它 viewer 状态。

## 5. 实现约束

- `transport/http` 不解析元数据 JSON，只负责透传 usecase 结果。
- `usecase` 负责命名空间解析、兼容旧 key、best-effort warmup 和审计/日志。
- `repository` 只负责读写 `view_meta`、节点和对象存储定位，不理解 viewer 语义。
- 读接口里的 warmup 只能作为异步补偿机制，不能阻塞列表主流程，也不能让主流程依赖它成功。
- 后续如果元数据规模变大，应新增专表或后台任务，不继续把复杂扫描逻辑堆在同步列表接口里。

## 6. 验证方式

- 单元测试覆盖 `__omniflowNodeMetadataV1.media.durationSeconds` 的解析与写回。
- 视频归档卡片接口验证已有 duration 时直接返回。
- 本地有 `ffprobe` 时，可手工验证缺失 duration 的视频卡片在首次读取后写回 `view_meta`，再次读取直接返回 `durationSeconds`。
- 本地无 `ffprobe` 时，接口仍必须正常返回，只是缺少 `durationSeconds`。

## 7. 后续维护

出现以下任一变化时必须更新本文档：

- 新增节点元数据 key 或 viewer state key
- 新增元数据提取来源或后台任务
- 将 `view_meta` 中的元数据迁移到专表
- 修改 `archive/cards` 返回的元数据字段
