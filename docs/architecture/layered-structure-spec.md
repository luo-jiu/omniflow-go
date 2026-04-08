# OmniFlow Go 分层与目录规范（v2）

关联文档：
- `docs/architecture/java-to-go-migration-handoff.md`（给 Java 视角同学/Agent 的迁移交接说明）
- `docs/progress/java-api-alignment-status.md`（接口对齐进度台账）

## 1. 目标与共识

本规范用于防止重构期间目录和职责失控，核心原则是：

- 对前端保持 Java 接口黑盒契约一致，做到“后端切换无感”。
- 当前阶段目录组织以**仓储类型优先**（postgres/redis/minio），业务域作为次级维度。
- `usecase` 只做业务编排，不直接操作 SQL/Redis/MinIO 客户端。

## 2. 设计原则（先仓储，再业务）

### 2.1 一级按仓储类型拆分

优先按 `postgres/redis/minio` 拆分顶层目录，先完成底层能力清晰化。

### 2.2 二级按业务域拆分

在 `postgres/redis/minio` 内部按 `user/library/node/auth` 等业务域分包，避免“同一业务实现堆在一个超大文件夹”。

### 2.3 文件数量控制

单个 package 推荐控制在 `3-7` 个核心文件：

- 小于 3：通常说明职责过重或过于集中。
- 大于 7：优先考虑按职责继续拆包（如 query/command）。

## 3. 目标目录结构（推荐）

```text
internal/
  domain/
    auth/
      session.go
    user/
      user.go
    library/
      library.go
    node/
      node.go

  usecase/
    auth.go
    user.go
    library.go
    node.go
    directory.go
    file.go

  repository/
    postgres/
      gen/
        main.go
      model/
        *.gen.go
      query/
        *.gen.go
      impl/
        txctx/
          txctx.go
        user/
          repository.go
        library/
          repository.go
        node/
          node_base.go
          node_query.go
          node_write.go
          ...
    redis/
      session/
        repository.go
    object/
      minio/
        store.go
    repoerr/
      errors.go
    user.go
    library.go
    node.go
    session.go
    object_storage.go
```

## 4. 分层职责边界

### 4.1 domain

- 放实体、值对象、领域错误、端口接口（port）。
- 不允许出现 `gorm`、`redis`、`minio` 等技术依赖。

### 4.2 usecase

- 放业务流程编排、权限校验、审计调用。
- 仅依赖 `domain` 端口接口。
- 不允许直接写 SQL 或 Redis 命令。

### 4.3 repository

- 放端口接口的具体实现。
- 事务、SQL、缓存 key 规则、TTL 等都收敛在这里。
- 同一存储类型下按业务域收敛，不跨目录散放。

### 4.4 transport

- 负责 HTTP 请求绑定、响应封装、错误码映射。
- 不写业务规则，不拼装数据库逻辑。

## 5. 依赖方向（硬约束）

必须保持单向依赖：

`transport -> usecase -> domain(port) <- repository(impl)`

禁止反向依赖：

- `domain` 依赖 `repository`
- `usecase` 直接依赖 `gorm.DB/redis.Client/minio.Client`

## 6. Go 代码组织细则（防爆炸）

### 6.1 repo 文件命名建议

- `repo.go`：构造与对外方法集合。
- `表名_entity.go`：DB 表实体定义（例如 `users_entity.go`、`nodes_entity.go`）。
- `*_filter.go`：查询过滤条件对象（避免函数参数爆炸）。
- `*_view.go`：聚合查询/投影结构（不直接映射单表）。
- `mapper.go`：model 与 domain 映射。
- `query*.go`：只读逻辑。
- `command*.go`：写入逻辑。

### 6.2 何时拆包

满足任一条件即考虑拆包：

- 单文件超过 `400` 行。
- 单 package 超过 `8` 个文件且职责分散。
- 评审时“找逻辑入口困难”。

### 6.3 何时不拆

- 只有 1-2 个简单查询/写入，且总行数不高时，不强制拆。
- 拆分后反而增加跨文件跳转负担时，保持当前结构。

### 6.4 事务与上下文规范

- 事务边界统一放在 `usecase`，不在普通仓储函数内隐式开启事务。
- 仓储函数统一通过 `dbWithContext(ctx)` 获取执行器：
  - `ctx` 中存在事务时自动复用事务；
  - 不存在事务时回落到默认 `db`。
- 事务上下文通过 `repository/postgres/impl/txctx` 传递。
- 避免在业务代码中大量显式 `WithTx(tx)` 串联调用，优先使用上下文感知方式。

### 6.5 SQL 编写规范

- 优先使用 GORM 链式查询处理常规 CRUD。
- 仅在递归查询、复杂聚合等场景使用 Raw SQL。
- Raw SQL 必须集中在 `*_sql.go` 常量中，避免散落在业务方法体内。
- Raw SQL 统一使用参数占位符，禁止字符串拼接构造条件。
- 仓储方法内调用统一走封装函数（如 `scanRaw`），减少重复模板代码。

### 6.6 代码生成规范（gorm/gen）

- 生成器入口放在 `internal/repository/postgres/gen`。
- 生成脚本放在 `tools/gen_postgres.sh`。
- 生成结果统一输出到：
  - `internal/repository/postgres/query`（query 代码）
  - `internal/repository/postgres/model`（model 代码）
- 业务仓储优先复用生成的 query/model；递归 CTE 等复杂 SQL 允许保留手写实现。

## 7. 登录链路规范（示例）

目标链路：

1. `handler/auth` 解析请求。
2. `usecase/auth` 编排登录流程。
3. `repository/postgres/impl/user` 校验用户凭证。
4. `repository/redis/session` 维护会话 token。
5. handler 返回统一响应结构。

关键要求：

- Redis 会话不在 usecase 直接操作客户端。
- key 命名、TTL、序列化统一封装在 session store 中。

## 8. 接口黑盒契约检查清单

每迁移一个接口，必须对齐以下内容：

1. Method + Path 不变。
2. Query/Body 字段名与大小写不变。
3. 响应外壳不变（`code/message/data/request_id`）。
4. `data` 字段键名不变（如 `parentId/libraryId/mimeType/fileSize`）。
5. 错误状态和语义不变（401/403/404/409）。

## 9. 渐进迁移策略

采用“小步快跑，每步可回滚”：

- 一次只改一个接口或一个子域。
- 每次改动后执行最小测试并提交 checkpoint。
- 未完成契约核对，不扩大改动范围。

推荐顺序：

1. `auth/session`：先把 Redis 会话抽成 domain port + repository impl。
2. `object/minio`：对象存储实现沉到底层仓储目录，并保留 provider 切换能力。
3. `node`：按 `repository/postgres/impl/node` 持续细化 query/command。
4. `library/user`：完成同样重组。
5. 清理历史遗留目录与过渡实现。

## 10. 当前仓库执行约束

从本规范生效后：

- 新增代码优先放入 `repository/postgres|redis|object`，不再向 `internal/repository` 根目录继续堆业务实现。
- 涉及 PG/Redis/MinIO 的实现，必须落在对应仓储类型目录下的业务子目录。
- 规范先行：结构调整前先更新此文档，再落地代码。

## 11. Java 重构阶段补充约束

为避免“接口已对齐但实现风格再次发散”，补充以下约束：

- 行为优先级：先保证 Java 黑盒契约，再做实现优化。
- 实现优先级：先补基建阻塞（端口/仓储/事务/配置），再补业务细节。
- 风格优先级：常规查询统一走 gen/ORM，Raw SQL 仅用于必要复杂场景并集中管理。
- 代码提交：坚持小步提交；一次提交聚焦一个行为点，便于回滚和追踪。
- 文档同步：接口行为调整后，必须同步更新进度台账与交接说明。

---

该文档是当前阶段重构的执行基线；若后续团队有新共识，先修订文档版本再实施代码变更。
