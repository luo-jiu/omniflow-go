# 用户偏好同步契约

更新时间：2026-08-19

适用范围：`user_preferences` 表、`/api/v1/user/me/preferences` HTTP 接口及桌面端跨设备个人布局同步。

## 1. 概述

`user_preferences` 保存当前用户需要跨 macOS、Windows 和后续客户端同步的轻量偏好。账号资料继续属于 `users`；窗口坐标、本地目录、缓存路径等设备事实不得写入本表。

偏好按 `user_id + namespace` 隔离，每个命名空间保存一个 JSON 对象。工具定义、页面组件和插件代码仍由客户端版本决定，数据库只保存稳定 ID、顺序、宽度和开关等用户选择。

## 2. 数据模型

迁移文件：`docs/schema/2026-08-19-user-preferences.sql`。

| 字段 | 含义 |
| --- | --- |
| `user_id` | 当前用户 ID，删除用户时级联删除偏好 |
| `namespace` | 小写命名空间，例如 `tool-workspace` |
| `data` | 对象类型 JSONB，单次 API 写入最大 `64 KiB` |
| `schema_version` | 当前命名空间的数据结构版本 |
| `revision` | 乐观并发版本，首次写入为 `1`，后续每次成功更新加 `1` |
| `created_at / updated_at` | 创建和最近更新时间 |

主键为 `(user_id, namespace)`，不增加无业务含义的自增 ID。当前不对 `data` 建 GIN 索引，因为服务不会按布局 JSON 内部字段检索用户集合。

## 3. HTTP 契约

当前接口：

- `GET /api/v1/user/me/preferences`：返回当前用户全部偏好；没有记录时返回空数组。
- `GET /api/v1/user/me/preferences/:namespace`：返回单个命名空间；不存在时返回 `404`。
- `PUT /api/v1/user/me/preferences/:namespace`：完整替换单个命名空间的数据。

`PUT` 请求体：

```json
{
  "schemaVersion": 1,
  "expectedRevision": 3,
  "preferences": {
    "navWidth": 224,
    "toolOrder": ["subtitle-translation", "media-file-processing", "media-processing"]
  }
}
```

`expectedRevision=0` 只允许首次创建；更新已有记录必须携带当前 revision。版本不一致返回 `409`，不得静默覆盖其他设备的新数据。写接口支持 `?dryRun=true`，执行同一套 JSON、命名空间和 revision 校验，但事务最终回滚。

所有接口只从认证 actor 取得用户 ID，不接受客户端传入任意 `userId`。审计和结构化日志只记录用户 ID、namespace、schema version、revision 和执行模式，不记录具体偏好 JSON。

## 4. 前端边界

前端登录后由应用级偏好 provider 拉取一次列表，并按 namespace 向业务 feature 投影。数据库是跨设备偏好的事实来源；本地 optimistic state 只负责即时交互，不得成为第二份长期权威数据。

工具工作区第一版数据：

```json
{
  "navWidth": 208,
  "toolOrder": ["subtitle-translation", "media-file-processing", "media-processing"]
}
```

客户端读取工具顺序时必须丢弃未知 ID、去重，并把当前版本新增的工具追加到末尾。宽度按客户端支持范围钳制，小窗口临时压缩不得反向覆盖数据库中的用户选择。

现有 `users.ext` 暂时继续承载历史偏好，不在本次一次性迁移。后续迁移时必须区分：

- 跨设备：主题、语言、工具顺序、界面宽度、插件开关。
- 设备本地：监听目录、窗口坐标、缓存目录、本地服务地址。

## 5. 分层与验证

- HTTP handler 只绑定请求和响应。
- usecase 校验 actor、namespace、JSON、revision、事务、dry-run 和审计。
- PostgreSQL repository 使用生成 model/query 和 GORM 完成 CRUD，不把 SQL 放进 handler/usecase。
- 该接口只服务客户端偏好同步，不新增 CLI 命令；CLI 不需要读取或修改桌面布局。

最低验证：

- 首次创建 revision 为 `1`。
- 正确 revision 更新后加 `1`。
- 旧 revision 返回 `409`。
- 非对象 JSON、超限 JSON 和非法 namespace 返回 `400`。
- dry-run 返回预期结果但数据库不新增或更新记录。
- 两个平台使用同一账号时能恢复同一工具顺序和合法宽度。

## 6. 部署顺序

该变更采用 expand-first：

1. 备份 PostgreSQL。
2. 执行 `2026-08-19-user-preferences.sql` 并验证表、外键和约束。
3. 部署包含新接口的 Go API。
4. 最后发布调用新接口的桌面客户端。

当前后端发布脚本不会自动执行数据库迁移，远程迁移必须作为独立受控步骤执行。

## 7. 维护规则

新增命名空间、修改 payload 结构、并发策略、大小限制、同步范围或部署顺序时，必须更新本文。需要查询或统计 JSON 内字段时，应重新评估是否拆成正式业务表，不能直接无边界扩充 `data`。
