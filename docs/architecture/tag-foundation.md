# 多维标签基座

更新时间：2026-05-07

适用范围：`tags`、`tag_target_kinds`、`tag_bind_policies`、`node_resource_targets`、`tag_aliases`、`node_tag_rel`、标签 HTTP API、节点搜索与 `nodes.view_meta.tagIds` 兼容链路。

## 1. 概述

OmniFlow 的标签从“按 `type` 区分的扁平标签”演进为“可描述资源、维度和展示用途的多维标签基座”。当前结论是：

- `tags` 是标签定义表，继续保留 `type` 兼容旧调用方。
- `tag_target_kinds` 是“可绑定对象”字典，例如普通文件、ASMR 作品、视频合集。
- `tag_bind_policies` 声明每个标签允许绑定到哪些对象。
- `node_resource_targets` 是节点当前资源语义的投影表，由数据库触发器维护，供后端校验和未来搜索过滤使用。
- `node_tag_rel` 是节点与标签的正式关系表，后续搜索过滤以它为准。
- `nodes.view_meta.tagIds` 只作为兼容输入，后端会在节点更新时同步到 `node_tag_rel`。
- `tag_aliases` 预留给别名、同义词和归一化搜索，不在本轮暴露写 API。

## 2. 背景

漫画、ASMR、音频、视频、普通文件和文件夹都会使用标签，但这些标签不只描述内容风格，也可能描述作者、角色、系列、来源、语言、技术属性或处理状态。继续只靠 `type=ASMR/COMIC/GENERAL` 会让后续搜索、筛选和管理页难以扩展。

本轮先完成底座，不强行一次性做完整标签管理体验。前端现有 ASMR 标签、顶部文件标签和未来文件 / 文件夹标签都应复用同一套标签定义与关系表。

## 3. 核心概念

### 3.1 `type`

历史兼容字段，仍然保留并返回给前端：

- 已有值：`ASMR`、`COMIC`、`GENERAL`、`FILE_TAB`
- 新增可用值：`AUDIO`、`VIDEO`、`FILE`、`FOLDER`
- 数据库约束改为大写枚举形态字符串：`^[A-Z0-9_-]{1,64}$`

后续新增资源类型时，不需要再改数据库枚举约束，但仍应在前端管理页和文档中登记。

### 3.2 `scope`

标签用途范围：

| 值 | 含义 |
|---|---|
| `resource` | 资源标签，用于漫画、ASMR、音频、视频、文件、文件夹等内容对象 |
| `ui` | UI 标签，例如顶部文件标签 `FILE_TAB` |

`FILE_TAB` 创建和更新时会被后端强制归为 `scope=ui`。

### 3.3 `dimension`

标签描述维度：

| 值 | 含义 |
|---|---|
| `genre` | 内容 / 风格 |
| `creator` | 作者 / 创作者 |
| `character` | 角色 |
| `series` | 系列 / 作品 |
| `source` | 来源 |
| `language` | 语言 |
| `region` | 地区 |
| `technical` | 技术属性 |
| `status` | 状态 |
| `custom` | 自定义 |

默认值是 `custom`。如果未来要把作者升级为独立实体，应在实体表与标签之间建立映射，而不是删除 `creator` 维度。

### 3.4 `resource_kind`

标签适用的资源类型，小写字符串，例如：

- `asmr`
- `comic`
- `audio`
- `video`
- `file`
- `folder`
- `general`

`resource_kind` 可以为空；`FILE_TAB` 会保持为空。后端会根据旧 `type` 推导常见资源类型，保证旧请求不必立即修改。

### 3.5 `target_kind`

`target_kind` 表示标签真正能贴到的资源单位，不等同于标签业务类型：

| 值 | 含义 |
|---|---|
| `file` | 普通文件 |
| `folder` | 普通文件夹 |
| `archive_root` | 开启归档模式的根目录 |
| `asmr_work` | ASMR 作品文件夹 |
| `comic_work` | 漫画作品文件夹 |
| `audio_track` | 音频文件 |
| `audio_album` | 音频专辑 / 音频归档项 |
| `video_file` | 视频文件 |
| `video_collection` | 视频合集 / 视频归档项 |

标签定义通过 `tag_bind_policies` 绑定多个 `target_kind`。节点通过 `node_resource_targets` 投影自己当前具备的资源语义。写入 `node_tag_rel` 前必须保证标签策略和节点语义存在交集。

默认策略：

| 标签类型 | 默认可贴对象 |
|---|---|
| `GENERAL` | `file / folder / archive_root` |
| `ASMR` | `asmr_work / archive_root / folder` |
| `COMIC` | `comic_work / archive_root / folder` |
| `AUDIO` | `audio_track / audio_album / folder` |
| `VIDEO` | `video_file / video_collection / folder` |
| `FILE` | `file` |
| `FOLDER` | `folder` |
| `FILE_TAB` | 无资源绑定策略 |

## 4. 契约

### 4.1 标签 API

现有路由保持不变：

- `GET /api/v1/tags`
- `POST /api/v1/tags`
- `PUT /api/v1/tags/:tagId`
- `DELETE /api/v1/tags/:tagId`

`GET /api/v1/tags` 新增可选过滤：

| query | 说明 |
|---|---|
| `type` | 历史类型过滤，后端转大写 |
| `scope` | `resource` / `ui` |
| `dimension` | 标签维度 |
| `resourceKind` | 资源类型，小写 |

创建和更新请求新增可选字段：

```json
{
  "scope": "resource",
  "dimension": "creator",
  "resourceKind": "comic",
  "targetKinds": ["comic_work", "archive_root"]
}
```

更新标签时，`targetKinds` 是补丁语义：请求未携带该字段时保留原有绑定策略；携带该字段时才替换 `tag_bind_policies`。`FILE_TAB` 应显式提交空数组，表示不绑定任何资源对象。

响应中的 `Tag` 会返回：

```json
{
  "id": 1,
  "name": "作者名",
  "type": "COMIC",
  "scope": "resource",
  "dimension": "creator",
  "resourceKind": "comic",
  "targetKinds": ["comic_work", "archive_root"]
}
```

旧字段 `targetKey / color / textColor / sortOrder / enabled / description` 语义不变。

### 4.2 节点标签关系

`node_tag_rel` 是正式关系表：

- `node_id`：节点 ID
- `tag_id`：标签 ID
- `library_id`：资料库 ID

`library_id` 由数据库触发器校验，必须与节点所属资料库一致。

当前还没有单独的“节点标签绑定 API”。已有 ASMR 等 viewer 继续写 `nodes.view_meta.tagIds`，后端在 `PUT /api/v1/nodes/:nodeId` 处理 `viewMeta` 时识别该字段并同步关系表。

同步时要求 `tagIds` 全部是当前用户可读、未删除且已启用的标签；只要存在无效或不可读 ID，节点更新会失败并回滚，避免 `viewMeta.tagIds` 与 `node_tag_rel` 分叉。

同步时还会校验绑定策略：

- 后端读取 `node_resource_targets` 判断当前节点可被视作哪些资源单位。
- 后端读取 `tag_bind_policies` 判断标签允许绑定哪些资源单位。
- 两者没有交集时拒绝保存，避免 ASMR 标签误贴到不合适的普通文件上。

### 4.3 节点搜索

`POST /api/v1/nodes/search` 的 `tagIds` 与 `tagMatchMode` 契约不变，但实现改为查询 `node_tag_rel`：

- `ANY`：命中任意一个标签关系即可返回。
- `ALL`：节点必须拥有全部请求标签。

迁移脚本会把历史 `nodes.view_meta.tagIds` 回填到 `node_tag_rel`，避免老数据丢失搜索能力。

## 5. 实现约束

- `transport/http` 只解析新增字段，不做标签分类规则。
- `usecase/tag` 负责 `type / scope / dimension / resourceKind / targetKinds` 的归一化和默认推导。
- `repository/postgres/impl/tag` 收敛标签定义查询与唯一性校验。
- `repository/postgres/impl/node` 收敛 `node_tag_rel` 搜索与同步写入。
- 新增表结构后必须先执行 schema patch，再运行 `tools/gen_postgres.sh` 生成 model/query，业务代码基于生成结果编写。
- 节点更新里同步 `node_tag_rel` 时必须与 `view_meta` 更新同事务提交。
- `node_tag_rel` 只保存直接标签；继承标签、自动标签、来源字段以后再扩展。

## 6. 验证方式

最低验证：

- `GOCACHE=/tmp/go-build go test ./...`
- 创建 / 更新标签时返回新增字段。
- 创建 / 更新标签时能保存并返回 `targetKinds`。
- ASMR 保存 `viewMeta.tagIds` 后，`node_tag_rel` 同步更新。
- 不匹配节点资源语义的标签写入会失败。
- `nodes/search` 的 `ANY / ALL` 在关系表下仍能过滤。

手工验证建议：

- 在标签管理页创建 `COMIC + creator + comic` 标签。
- 在已有 viewer 保存标签后，用节点搜索确认标签过滤生效。
- 顶部标签 `FILE_TAB` 仍能按 `targetKey` 影响 tab 展示。

## 7. 后续维护

出现以下变化时必须更新本文档：

- 新增标签维度、资源类型、scope 或 target kind。
- 新增节点标签绑定 API。
- `tag_aliases` 开始暴露 API 或参与搜索。
- 引入标签继承、自动标签或标签来源字段。
- 将 `nodes.view_meta.tagIds` 兼容字段下线。
