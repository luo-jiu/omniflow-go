# 资源监测控制台后端契约

更新时间：2026-05-12

适用范围：`resource-monitor` 后端 API、资源占用统计、后续存储 / 数据库探针扩展。

## 1. 概述

资源监测控制台后端提供只读资源快照，服务于前端仓库页 / 资料库页 system workspace。它不承担存储配置修改，不触发迁移，不清理对象，也不执行有副作用的连通性测试。

当前实现全局资源分布快照、细分仪表盘和只读资源探针：

```text
GET /api/v1/resource-monitor/snapshot
GET /api/v1/resource-monitor/snapshot?libraryId=123
GET /api/v1/resource-monitor/distribution
GET /api/v1/resource-monitor/distribution?libraryId=123
GET /api/v1/resource-monitor/breakdown
GET /api/v1/resource-monitor/breakdown?libraryId=123
GET /api/v1/resource-monitor/dashboard
GET /api/v1/resource-monitor/dashboard?libraryId=123
GET /api/v1/resource-monitor/probes
POST /api/v1/resource-monitor/samples
POST /api/v1/resource-monitor/samples?libraryId=123
POST /api/v1/resource-monitor/samples?libraryId=123&dryRun=true
```

## 2. 分层边界

当前链路：

```text
transport/http/handler/resource_monitor.go
  -> usecase/resource_monitor.go
    -> domain/resourcemonitor.Repository
      <- repository/postgres/impl/resourcemonitor
```

约束：

- handler 只做 actor 注入、调用 usecase 和响应封装。
- usecase 负责 actor 校验、汇总、排序和 provider 配置补全。
- domain 放快照模型和 repository 端口。
- repository 只做 PostgreSQL 聚合查询。
- 不在 `StorageConfigHandler` 中扩展监测逻辑；配置和监控分开。

## 3. API 契约

### 3.1 快照

```text
GET /api/v1/resource-monitor/snapshot
```

`/snapshot` 是兼容聚合接口，会同时返回分布统计和探针结果。前端资源监测面板默认使用下面几个拆分接口并行加载，避免慢探针、慢统计或慢细分阻塞其它 UI：

```text
GET /api/v1/resource-monitor/distribution
GET /api/v1/resource-monitor/distribution?libraryId=123
GET /api/v1/resource-monitor/breakdown
GET /api/v1/resource-monitor/breakdown?libraryId=123
GET /api/v1/resource-monitor/dashboard
GET /api/v1/resource-monitor/dashboard?libraryId=123
GET /api/v1/resource-monitor/probes
```

鉴权：

- 需要登录态。
- 不带 `libraryId` 时统计 actor 自己拥有的全部资料库范围。
- 带 `libraryId` 时只统计 actor 自己拥有的指定资料库；不拥有该资料库时结果为空，不泄露跨用户资源。
- `/distribution` 的范围语义与 `/snapshot` 一致，只返回 `summary / storage / distributionError` 等分布字段。
- `/breakdown` 的范围语义与 `/distribution` 一致，只返回资料库、归档分类、资源状态和诊断摘要等细分字段。
- `/dashboard` 的范围语义与 `/breakdown` 一致，返回 V2 仪表盘字段；它不执行探针，探针仍由 `/probes` 独立返回。
- `/probes` 只返回 `probeSummary / probes`，探针是当前后端基础设施和 provider 配置级别的只读检查，不按资料库过滤。

响应 `data`：

```json
{
  "generatedAt": "2026-05-11T00:00:00Z",
  "summary": {
    "providerCount": 1,
    "bucketCount": 1,
    "objectCount": 10,
    "fileRefCount": 10,
    "physicalBytes": 1024,
    "visibleObjectCount": 8,
    "visibleFileRefCount": 8,
    "visibleBytes": 768,
    "recycleObjectCount": 2,
    "recycleFileRefCount": 2,
    "recycleBytes": 256,
    "orphanObjectCount": 0,
    "orphanBytes": 0,
    "unmatchedCount": 0,
    "legacyProviderCount": 0
  },
  "storage": [
    {
      "provider": "local-minio",
      "sourceProvider": "MINIO",
      "providerType": "MINIO",
      "providerLabel": "Local MinIO",
      "endpoint": "localhost:9000",
      "bucket": "default",
      "isDefault": true,
      "isLegacyProvider": true,
      "objectCount": 10,
      "fileRefCount": 10,
      "physicalBytes": 1024,
      "visibleObjectCount": 8,
      "visibleFileRefCount": 8,
      "visibleBytes": 768,
      "recycleObjectCount": 2,
      "recycleFileRefCount": 2,
      "recycleBytes": 256,
      "orphanObjectCount": 0,
      "orphanBytes": 0,
      "percent": 100,
      "matchedConfig": true
    }
  ],
  "distributionError": "",
  "probeSummary": {
    "total": 3,
    "ok": 3,
    "error": 0,
    "unknown": 0
  },
  "probes": [
    {
      "key": "object-storage:local-minio",
      "kind": "object_storage",
      "label": "Local MinIO",
      "provider": "local-minio",
      "providerType": "MINIO",
      "endpoint": "localhost:9000",
      "bucket": "default",
      "isDefault": true,
      "status": "ok",
      "latencyMs": 4,
      "checkedAt": "2026-05-11T00:00:00Z"
    },
    {
      "key": "postgres:primary",
      "kind": "postgres",
      "label": "PostgreSQL",
      "status": "ok",
      "latencyMs": 1,
      "checkedAt": "2026-05-11T00:00:00Z"
    },
    {
      "key": "redis:primary",
      "kind": "redis",
      "label": "Redis",
      "status": "ok",
      "latencyMs": 1,
      "checkedAt": "2026-05-11T00:00:00Z"
    }
  ]
}
```

### 3.2 细分仪表盘

```text
GET /api/v1/resource-monitor/breakdown
GET /api/v1/resource-monitor/breakdown?libraryId=123
```

响应 `data` 包含：

- `summary`：资料库数、归档目录数、物理去重容量、引用展开容量、对象数、文件引用数、visible / recycle / orphan、多引用对象等总览。
- `libraries`：按资料库聚合的容量、对象、引用、归档目录、状态容量、最大 provider / bucket 和占比，默认按容量降序。
- `categories`：按 `builtInType` 归类的普通资源、漫画、ASMR、视频、音频、图集、未知类型和未归类对象。
- `statuses`：可见资源、回收站、孤儿对象三类对象级主归属。
- `anomalies`：只读诊断摘要，例如回收站占用集中、孤儿对象、多引用对象。
- `breakdownError`：细分统计失败时的脱敏错误摘要；失败不影响 `/distribution` 和 `/probes`。

当前第一版不新增表结构，不写入历史，不做清理动作。

### 3.3 V2 仪表盘

```text
GET /api/v1/resource-monitor/dashboard
GET /api/v1/resource-monitor/dashboard?libraryId=123
```

响应 `data` 包含：

- `summary`：沿用细分仪表盘总览，区分物理去重容量和引用展开容量。
- `fileTypes`：基础文件类型分布，例如视频、图片、音频、文本、压缩包、未知类型。
- `collections`：业务集合类型分布，例如普通资源、漫画、ASMR、视频、音频、图集、未知类型、未归类对象。
- `collectionFileTypeMatrix`：业务集合 x 基础文件类型交叉统计，用于解释 ASMR / 漫画等集合内部由哪些基础文件组成。
- `libraries / statuses / anomalies`：沿用细分仪表盘中的资料库排行、资源状态和只读诊断摘要。
- `dashboardError`：V2 统计失败时的脱敏错误摘要；该接口不影响 `/distribution`、`/breakdown` 和 `/probes`。

`/dashboard` 是 V2 并行接口，当前不替换 `/breakdown`；前端完成 V2 页面验证后再决定旧接口的兼容周期。

### 3.4 显式采样

```text
POST /api/v1/resource-monitor/samples
POST /api/v1/resource-monitor/samples?libraryId=123
```

鉴权和范围：

- 需要登录态。
- 不带 `libraryId` 时写入 actor 全部资料库范围的全局样本。
- 带 `libraryId` 时写入 actor 指定资料库范围的样本；不拥有该资料库时返回 not found，不写入空样本。
- 当前只支持用户显式触发，不启动后台定时器。
- 支持 `dryRun=true`，返回将要写入的样本预览，但不持久化。

响应 `data`：

```json
{
  "id": 1,
  "dryRun": false,
  "actorId": "42",
  "scope": "library",
  "libraryId": 123,
  "generatedAt": "2026-05-11T00:00:00Z",
  "providerCount": 1,
  "bucketCount": 1,
  "objectCount": 10,
  "fileRefCount": 10,
  "physicalBytes": 1024,
  "visibleObjectCount": 8,
  "visibleFileRefCount": 8,
  "visibleBytes": 768,
  "recycleObjectCount": 2,
  "recycleFileRefCount": 2,
  "recycleBytes": 256,
  "orphanObjectCount": 0,
  "orphanBytes": 0,
  "unmatchedCount": 0,
  "legacyProviderCount": 0,
  "probeTotal": 3,
  "probeOk": 3,
  "probeError": 0,
  "probeUnknown": 0,
  "createdAt": "2026-05-11T00:00:00Z"
}
```

## 4. 统计口径

- `physicalBytes`：按 distinct `storage_objects` 聚合 `content_length`，代表真实对象占用。
- `referencedBytes`：按 `node_files` 引用展开后的容量，可能因为同一对象多引用而大于 `physicalBytes`。
- `libraryId`：可选查询参数；`0` 或缺失表示当前用户全部资料库，正整数表示指定资料库范围。
- `objectCount`：当前用户资料库下未软删的 `storage_objects` 数量。
- `fileRefCount`：引用这些对象的 `node_files` 行数。
- `visible*`：存在未删除节点引用的对象及其文件引用数 / 容量；对象有可见引用时优先归入此类。
- `recycle*`：没有可见引用、但存在已删除节点引用的对象及其文件引用数 / 容量。
- `orphan*`：没有任何 `node_files` 引用的对象及其容量。
- `providerCount`：结果中不同 provider 数量。
- `bucketCount`：结果中不同 provider / bucket 组合数量。
- `unmatchedCount`：结果中无法通过当前 `StorageRegistry` 匹配到 provider 配置的行数。
- `legacyProviderCount`：仍使用历史 provider 类型值、但已兼容映射到唯一 alias 的存储位置数。
- `sourceProvider` / `isLegacyProvider`：展示历史 provider 类型值与当前 alias 的兼容关系。
- `distributionError`：资源分布统计失败时的脱敏错误摘要；此时接口仍返回 partial snapshot 和探针结果。
- `probeSummary`：对象存储、Postgres、Redis 探针状态汇总。
- `probes`：只读探针结果，错误只返回脱敏后的简短摘要。
- `breakdown.categories`：按引用节点向上寻找最外层非 `DEF` 内置类型做分类；内置类型被视为内容集合，集合内部文件都归属该外层内置类型。物理容量按对象主分类去重，引用容量按引用展开；归档模式允许嵌套，分类统计不按 `archive_mode` 递归聚合。
- `dashboard.collections`：沿用最外层非 `DEF` 内置类型作为业务集合归属。
- `dashboard.fileTypes`：优先按 `node_files.mime_type` / `storage_objects.content_type` 归类基础文件类型，再用节点扩展名兜底；`.ts` 等冲突后缀不能只靠后缀决定真实类型。
- `dashboard.collectionFileTypeMatrix`：同一对象的物理容量只进入一个主业务集合和一个主基础文件类型；引用容量按 `node_files` 展开。
- `breakdown.statuses`：沿用 visible / recycle / orphan 对象级主归属。
- `resource_monitor_samples`：历史采样表；summary 列用于趋势查询，`snapshot_json` 保存完整快照用于后续告警、详情回放和重新聚合。
- `dryRun`：采样写链路的标准 dry-run 参数，真实校验和快照生成照常执行，但跳过持久化。

第一版暂不区分：

- 历史 provider 类型值修复动作

## 5. 探针约束

探针能力必须遵守：

- 对象存储 probe 只能做只读检查，例如 bucket exists / head bucket。
- 不允许为了探测连通性创建 bucket、写测试对象或删除对象。
- Postgres / Redis / MySQL 探针必须收敛在对应 repository 实现，usecase 不直连技术客户端。
- 探针错误应返回稳定的状态、耗时和脱敏错误摘要，不返回密钥。
- 资源分布统计失败不应阻断探针返回；前端需要能看见 Postgres / Redis / 对象存储状态。

当前已实现：

- 对象存储：调用 provider 的只读 `Probe`，MinIO/S3 兼容实现使用 `BucketExists`。
- Postgres：resource monitor PostgreSQL repository 调用 `PingContext`。
- Redis：resource monitor Redis repository 调用 `PING`。

后续仍未实现：

- 自动周期采样、趋势曲线和阈值提醒。
- MySQL / 外部资源探针。

## 6. 验证方式

当前最低验证：

```bash
GOCACHE=/tmp/go-build go test ./...
```

重点回归：

- actor id 缺失时返回参数错误。
- provider 配置匹配时补齐 label / type / endpoint / default。
- provider 配置缺失时保留原 provider 并标记 `matchedConfig=false`。
- 显式采样会写入 `resource_monitor_samples`；带 `libraryId` 时必须先通过 actor ownership 校验。
- `dryRun=true` 时返回样本预览且不写入 `resource_monitor_samples`。
- `/breakdown` 返回资料库、分类、状态和诊断摘要，且不阻塞 `/distribution` 和 `/probes`。
- `/dashboard` 返回基础文件类型、业务集合类型和交叉矩阵；它不执行探针，也不阻塞旧 `/breakdown`。
