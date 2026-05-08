# 存储迁移设计

## 目的与边界

存储迁移把节点子树（文件夹或单个文件）下的所有 `storage_objects` 物理对象从一个 provider 搬到另一个 provider，
节点元数据保持不变，仅 `node_files.storage_object_id` 引用切换到新对象。

入队侧：HTTP / CLI 两种入口；执行侧：bootstrap 启动的 worker pool；进度反馈：HTTP 轮询（5 s）。

不在 v1 范围内：
- 检疫期（`keep_source_days`）
- WebSocket / SSE 实时推送
- 跨 provider SHA256 校验（v1 仅校验 size）
- 处理任务（转码 / 归档）的真实 executor
- 暂停 / 续作（v1 取消即软取消，已完成项不回滚）

## 表结构

详见 `omniflow-go/docs/schema/2026-05-09-migration-tasks.sql`，要点：

- `migration_tasks(id text PK, actor_id, library_id, root_node_id, target_provider, status, total/completed/failed/skipped objects, total/transferred bytes, current_object_key, error_message, timestamps)`
- `migration_task_items(id bigserial, task_id, storage_object_id, source_provider/bucket/key, target_storage_object_id, target_key, file_size, status, error_message, timestamps)`
- worker 抢任务索引：`(status, id) WHERE status='pending'`
- ID 类型：`migration_tasks.id` 是 text（UUID 或 `mtk_xxx` 等业务 ID），子项是 bigserial

## 状态机

任务级：`pending → running → (completed | failed | canceled)`
子项级：`pending → running → (done | failed | skipped)`

转换规则：
- 任务创建时 status=pending；至少一个子项进入 running 时整体 running（隐式聚合）
- 所有子项进入终态后聚合到任务终态：全部 done → completed；任意 failed → failed；混合 skipped 不计为失败
- 取消：用户主动 → status=canceled；已 running 的子项不抢回，剩余 pending 子项被 worker 抢到时立刻置 skipped
- v1 不支持暂停 / 续作

## 安全切换流程

每个子项的处理是事务原子的，避免节点指向不存在的对象：

1. `source.GetObject(key)` → 流式 reader（defer Close）
2. `target.Upload(target_key, reader, size, contentType)` → 流式直传
3. `target.StatObject(target_key)` → 校验 size 一致；不一致则 `target.Delete(target_key)` + 子项 failed
4. **事务内**：
   - INSERT 新 `storage_objects` 行（target provider/bucket/object_key）
   - UPDATE `node_files SET storage_object_id = new_id WHERE storage_object_id = old_id`（同一 storage_object 的所有引用一起切，绝不创建副本）
   - 软删（GORM Delete）旧 `storage_objects` 行
   - UPDATE `migration_task_items SET status='done', target_storage_object_id=new_id, finished_at=now()`
   - INCREMENT `migration_tasks.completed_objects/transferred_bytes/current_object_key`
5. 事务提交后：`source.Delete(source_key)` —— best effort，失败仅 warn 不报错（孤儿由 v1.1 sweeper 清理）
6. 全部子项进入终态后，`AggregateTaskProgressIfFinal` 聚合任务终态

## Worker pool

- bootstrap 启动 N 个 worker（默认 2，可配置），共享一个 `MigrationUseCase`
- 每个 worker 主循环：`ClaimNextItem`（`UPDATE ... FOR UPDATE SKIP LOCKED LIMIT 1`） → `processItem` → 立即继续
- 抢不到子项时 sleep 5 s 再 poll
- bootstrap cleanup 关闭 stopCh，等待 wg.Wait()

`ClaimNextItem` 是仓储层中**唯一**的 raw SQL 例外（gorm/gen 不直接支持 SKIP LOCKED 并发控制），符合 AGENTS.md 豁免条款。

## HTTP 端点

```
POST   /api/v1/migration/tasks                       入队（支持 ?dryRun=true）
GET    /api/v1/migration/tasks?libraryId=N           列举任务
GET    /api/v1/migration/tasks/:id                   单任务详情
POST   /api/v1/migration/tasks/:id/cancel            取消任务（支持 ?dryRun=true）
GET    /api/v1/migration/tasks/:id/items             子项列表（调试用）
GET    /api/v1/libraries/:libraryId/storage-distribution?nodeId=N  按 provider 分布
```

错误语义：404 actor 不一致 / 任务不存在（统一防 task_id 枚举）；409 已终态不能取消；400 参数错误。

`?dryRun=true` 行为：
- 入队 dry-run：跑完所有校验和枚举，返回 `{plannedObjects, plannedBytes, targetProvider, targetBucket, storageObjectIds}`，不落库
- 取消 dry-run：仅校验任务可取消（actor + 非终态），不真正落库

## 共享对象处理

同一 `storage_object_id` 被多 node_files 引用时，迁移其中一个文件夹会自动把所有引用一起切：
- `EnumerateStorageObjectsUnderNode` 返回 distinct 的 `storage_object_id`
- `SwapStorageObject` 的 UPDATE node_files 不限定 node_file_id，按 `(library_id, storage_object_id=old)` 全量切换
- 这种语义保证不重复迁移，也不创建副本

## 客户端覆盖

CLI（`internal/transport/cli/command_migration.go`）：

```
of storage migrate --library-id N --node-id N --target-provider X [--dry-run] [--json]
of storage migration ls [--library-id N] [--status running,pending] [--limit N] [--json]
of storage migration status --task-id T [--items] [--json]
of storage migration cancel --task-id T [--dry-run] [--json]
of storage distribution --library-id N --node-id N [--json]
```

Web UI：
- 文件树右键菜单 → "迁移到其他存储..." → MigrationDialog（拉 storage-distribution + provider 列表 → 选目标 → 入队）
- 入队成功后跳转 `#/transfer-center?tab=migration` 进入迁移 tab，5 s 轮询任务列表
- 任务行显示进度条 / 计数 / 字节 / 当前 key / 状态 tag / 取消按钮

CLI 与 Web 共用同一套 6 个 HTTP 端点，后端动一处两端受益。

## 不变约束

1. `UploadTaskInput` / `UploadManagerEvent` 形状不变 —— 嗅探 / 下载 / 上传链路 0 改动
2. `storage_objects` 唯一索引 `(bucket, object_key, provider)` 在迁移后仍成立（new_id 用新 key + 新 provider）
3. 单文件迁移失败不能损坏 node：事务保证 storage_object 切换原子化
4. 同一 storage_object 被多 node 引用时，所有引用一起切，绝不创建副本
5. `migration_task_items.status='done'` 一旦持久化，回滚仅由 v1.1 sweeper 处理（v1 不支持）

## v1.1 / v2 候补

- `keep_source_days` 检疫期（延迟删源）
- WebSocket / SSE 实时进度（替换 5 s 轮询）
- 跨 provider 校验：流式 SHA256
- 暂停 / 续作（不只是软取消）
- 库级"存储分布仪表盘"`/library/:id/storage`
- 处理任务真实 executor
