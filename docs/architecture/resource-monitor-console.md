# 资源监测控制台后端契约

更新时间：2026-05-11

适用范围：`resource-monitor` 后端 API、资源占用统计、后续存储 / 数据库探针扩展。

## 1. 概述

资源监测控制台后端提供只读资源快照，服务于前端仓库页 / 资料库页 system workspace。它不承担存储配置修改，不触发迁移，不清理对象，也不执行有副作用的连通性测试。

当前实现全局资源分布快照和只读资源探针：

```text
GET /api/v1/resource-monitor/snapshot
GET /api/v1/resource-monitor/snapshot?libraryId=123
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

鉴权：

- 需要登录态。
- 不带 `libraryId` 时统计 actor 自己拥有的全部资料库范围。
- 带 `libraryId` 时只统计 actor 自己拥有的指定资料库；不拥有该资料库时结果为空，不泄露跨用户资源。

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

## 4. 统计口径

- `physicalBytes`：按 distinct `storage_objects` 聚合 `content_length`，代表真实对象占用。
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

- MySQL / 外部资源探针。
- 历史采样、趋势曲线和阈值提醒。

## 6. 验证方式

当前最低验证：

```bash
GOCACHE=/tmp/go-build go test ./...
```

重点回归：

- actor id 缺失时返回参数错误。
- provider 配置匹配时补齐 label / type / endpoint / default。
- provider 配置缺失时保留原 provider 并标记 `matchedConfig=false`。
