# 资源监测控制台后端契约

更新时间：2026-05-11

适用范围：`resource-monitor` 后端 API、资源占用统计、后续存储 / 数据库探针扩展。

## 1. 概述

资源监测控制台后端提供只读资源快照，服务于前端仓库页 / 资料库页 system workspace。它不承担存储配置修改，不触发迁移，不清理对象，也不执行有副作用的连通性测试。

当前第一版只实现全局资源分布快照：

```text
GET /api/v1/resource-monitor/snapshot
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
- 当前只统计 actor 自己拥有的资料库范围。

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
    "unmatchedCount": 0
  },
  "storage": [
    {
      "provider": "local-minio",
      "providerType": "MINIO",
      "providerLabel": "Local MinIO",
      "endpoint": "localhost:9000",
      "bucket": "default",
      "isDefault": true,
      "objectCount": 10,
      "fileRefCount": 10,
      "physicalBytes": 1024,
      "percent": 100,
      "matchedConfig": true
    }
  ]
}
```

## 4. 统计口径

- `physicalBytes`：按 distinct `storage_objects` 聚合 `content_length`，代表真实对象占用。
- `objectCount`：当前用户资料库下未软删的 `storage_objects` 数量。
- `fileRefCount`：引用这些对象的 `node_files` 行数。
- `providerCount`：结果中不同 provider 数量。
- `bucketCount`：结果中不同 provider / bucket 组合数量。
- `unmatchedCount`：结果中无法通过当前 `StorageRegistry` 匹配到 provider 配置的行数。

第一版暂不区分：

- 可见文件占用
- 回收站占用
- 孤儿对象占用
- 历史 provider 类型值修复建议

## 5. 后续扩展

后续探针能力必须遵守：

- 对象存储 probe 只能做只读检查，例如 bucket exists / head bucket。
- 不允许为了探测连通性创建 bucket、写测试对象或删除对象。
- Postgres / Redis / MySQL 探针必须收敛在对应 repository 实现，usecase 不直连技术客户端。
- 探针错误应返回稳定的状态、耗时和脱敏错误摘要，不返回密钥。

## 6. 验证方式

当前最低验证：

```bash
GOCACHE=/tmp/go-build go test ./...
```

重点回归：

- actor id 缺失时返回参数错误。
- provider 配置匹配时补齐 label / type / endpoint / default。
- provider 配置缺失时保留原 provider 并标记 `matchedConfig=false`。

