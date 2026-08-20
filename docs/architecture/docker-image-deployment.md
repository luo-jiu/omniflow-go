# Docker 镜像构建与运行说明

更新时间：2026-08-13

适用范围：`omniflow-go` HTTP 服务镜像构建、本地 Docker Desktop 试运行、单机 Docker Compose 生产部署、运行时配置挂载与后续版本替换流程。

## 1. 概述

`omniflow-go` 可以通过项目根目录的 `Dockerfile` 构建为本地 Docker 镜像。镜像内包含编译后的 HTTP 服务二进制与默认 `configs/` 目录，容器启动时默认执行 `./server -config configs/config.yaml`。

当前本地验证结论：

- 镜像名：`omniflow-go:local`
- 镜像位置：Docker Desktop 当前 context 的本地 image 存储，不会生成到项目目录。
- 当前 context：`desktop-linux`
- 服务端口：容器内 `8850`，本地测试映射到宿主机 `8850`
- 健康检查：`GET /healthz` 已验证返回 `200 OK`

## 2. 背景

容器化的目标是让后端服务发布流程变成：

1. 修改代码或配置模板。
2. 重新构建 Docker image。
3. 用新 image 重建容器。
4. 运行时通过挂载配置连接现有 PostgreSQL、Redis、MinIO 等中间件。

当前中间件不要求纳入同一个 Docker Compose。只要容器内配置能访问既有中间件，`omniflow-go` 可以单独以 Docker 容器运行。

## 3. 核心概念

### 3.1 Dockerfile

`Dockerfile` 是镜像构建说明书，负责描述：

- 使用 Go 构建环境编译 `./cmd/server`。
- 将编译后的 Linux 二进制复制到运行镜像。
- 复制默认 `configs/` 目录。
- 暴露 `8850` 端口。
- 定义默认启动命令。

### 3.2 镜像与容器

- image：`docker build` 生成的只读模板，存放在 Docker 本地 image 存储。
- container：基于 image 启动出来的运行实例。
- 重新 build image 不会自动更新正在运行的 container，必须停止并重建 container。

### 3.3 配置文件

`cmd/server/main.go` 支持通过 `-config` 指定主配置文件：

```bash
./server -config configs/config.yaml
```

不指定时默认读取 `configs/config.yaml`。

存储配置还有一个独立规则：服务启动时会优先读取 `configs/storage.yaml`。因此 Docker 运行时如果需要切换对象存储地址，必须同时挂载：

- `configs/config.docker.yaml` 到容器内 `/app/configs/config.yaml`
- `configs/storage.docker.yaml` 到容器内 `/app/configs/storage.yaml`

## 4. 构建流程

在 `omniflow-go` 项目根目录执行：

```bash
docker build -t omniflow-go:local .
```

查看镜像：

```bash
docker images omniflow-go:local
```

已验证镜像信息：

```text
IMAGE               ID             DISK USAGE
omniflow-go:local   7e0a8dd688f6   41.6MB
```

后续发版本时可以使用更明确的 tag：

```bash
docker build -t omniflow-go:0.1.1 -t omniflow-go:latest .
```

如果复用同一个 tag，例如 `omniflow-go:local`，新构建会让该 tag 指向新 image。旧 image 如果没有 tag 或容器引用，后续可由 Docker 清理。

## 5. 运行流程

本地 Docker Desktop 环境下，容器访问宿主机上的中间件应使用 `host.docker.internal`，不能使用 `127.0.0.1` 或 `localhost`。

测试运行命令：

```bash
docker run -d --name omniflow-go-test \
  -p 8850:8850 \
  -v /Users/loyce/personal/omniflow/omniflow-go/configs/config.docker.yaml:/app/configs/config.yaml:ro \
  -v /Users/loyce/personal/omniflow/omniflow-go/configs/storage.docker.yaml:/app/configs/storage.yaml:ro \
  omniflow-go:local
```

查看状态：

```bash
docker ps --filter name=omniflow-go-test
```

查看日志：

```bash
docker logs omniflow-go-test
```

健康检查：

```bash
curl -i http://127.0.0.1:8850/healthz
```

已验证响应：

```text
HTTP/1.1 200 OK
```

## 6. 配置约束

配置优先级为：代码默认值 < YAML < `OMNIFLOW_*` 环境变量。生产部署通过服务器 `.env` 注入秘密和少量环境差异；`.env` 不提交 Git，也不复制进 image。

当前支持的环境变量：

- `OMNIFLOW_APP_ENV`、`OMNIFLOW_APP_VERSION`
- `OMNIFLOW_SERVER_HOST`、`OMNIFLOW_SERVER_PORT`、`OMNIFLOW_SERVER_MODE`
- `OMNIFLOW_LOG_LEVEL`、`OMNIFLOW_LOG_FORMAT`、`OMNIFLOW_LOG_ADD_SOURCE`
- `OMNIFLOW_DATABASE_DSN`，或结构化连接变量 `OMNIFLOW_DATABASE_HOST`、`OMNIFLOW_DATABASE_PORT`、`OMNIFLOW_DATABASE_USER`、`OMNIFLOW_DATABASE_PASSWORD`、`OMNIFLOW_DATABASE_NAME`、`OMNIFLOW_DATABASE_SSLMODE`
- `OMNIFLOW_DATABASE_MAX_OPEN_CONNS`、`OMNIFLOW_DATABASE_MAX_IDLE_CONNS`
- `OMNIFLOW_DATABASE_LOG_LEVEL`、`OMNIFLOW_DATABASE_DEBUG_SQL`
- `OMNIFLOW_REDIS_ADDR`、`OMNIFLOW_REDIS_PASSWORD`、`OMNIFLOW_REDIS_DB`

空字符串不覆盖 YAML；整数或布尔值格式错误时服务启动失败。生产配置结构模板见 `configs/config.production.yaml.example`。

生产 Compose 优先使用结构化数据库变量，并由程序生成正确转义的 DSN，避免随机密码中的 `@`、`:`、`/` 等 URL 保留字符破坏连接串。显式设置 `OMNIFLOW_DATABASE_DSN` 时，其优先级高于结构化变量。

`configs/config.docker.yaml` 面向 Docker 本地运行，关键差异是将中间件地址切换为容器可达地址。MinIO 需要同时配置 `public_endpoint`，用于生成前端 / Electron 可直接访问的预签名 URL：

```yaml
database:
  dsn: postgres://postgres:***@host.docker.internal:5432/omniflow?sslmode=disable

redis:
  addr: host.docker.internal:6379

minio:
  endpoint: host.docker.internal:9000
  public_endpoint: localhost:9000
  access_key: ***
  secret_key: ***
```

`configs/storage.docker.yaml` 也必须同步配置内部连接地址和外部签名地址：

```yaml
providers:
  local-minio:
    type: minio
    endpoint: host.docker.internal:9000
    public_endpoint: localhost:9000
    access_key: ***
    secret_key: ***
    bucket: my-bucket
default_provider: local-minio
```

`endpoint` 面向容器内后端：用于 bucket 检查、上传完成、分片列表、迁移复制等服务端直连 MinIO 的操作。

`public_endpoint` 面向客户端：用于 `GET /directory/link`、批量文件链接、直传上传分片签名等返回给前端或 CLI 的预签名 URL。本地 Docker Desktop 下不要把 `host.docker.internal` 签进返回给前端的 URL；宿主机访问该 host 时可能出现空响应，导致图片、图集缩略图和 HEIC 预览下载失败。

配置文件是在服务启动时读取的。修改挂载的配置文件后，需要重启容器才能生效：

```bash
docker restart omniflow-go-test
```

## 7. 版本替换

代码变更后发布新版：

```bash
docker build -t omniflow-go:local .
docker stop omniflow-go-test
docker rm omniflow-go-test
docker run -d --name omniflow-go-test \
  -p 8850:8850 \
  -v /Users/loyce/personal/omniflow/omniflow-go/configs/config.docker.yaml:/app/configs/config.yaml:ro \
  -v /Users/loyce/personal/omniflow/omniflow-go/configs/storage.docker.yaml:/app/configs/storage.yaml:ro \
  omniflow-go:local
```

只修改运行时配置，不修改代码时，不需要重新 build：

```bash
docker restart omniflow-go-test
```

### 7.1 单机 Compose 生产发布

推荐服务器目录：

```text
/srv/omniflow/
├── .env
├── compose.yaml
├── app/
├── configs/
├── data/
├── backups/
└── scripts/
```

边界规则：

- `.env` 保存 PG / Redis 原始连接字段与密码，权限必须为 `0600`；可从 `deploy/.env.production.example` 创建，禁止提交真实文件。
- `configs/config.yaml` 保存非敏感生产配置，基于示例模板创建。
- `configs/storage.yaml` 保存 provider 结构和存储凭据，权限必须为 `0600`。
- PostgreSQL / Redis 只加入 Compose 私有网络，不映射宿主机端口。
- API 可先绑定 `127.0.0.1:8850`，由 Tailscale 或反向代理决定外部入口。
- 发版只重建 `api`；PG / Redis 容器及宿主机数据目录不随发版替换。

首次部署或排障时，可以手工执行最小发布命令：

```bash
cd /srv/omniflow
docker compose build api
docker compose up -d --no-deps api
curl --fail http://127.0.0.1:8850/healthz
curl --fail "http://127.0.0.1:8850/api/v1/user/exists?username=__omniflow_deploy_probe__"
```

目录初始化完成后，日常发版统一执行：

```bash
cd /srv/omniflow
./scripts/deploy.sh
```

脚本会生成可追溯的 image tag，只重建并替换 API 容器，然后依次验证 HTTP 健康接口和 PostgreSQL 只读查询链路。PostgreSQL、Redis 容器及其数据目录不会被替换。

正式发布应固定 Git commit 或 tag，并给 image 写入对应 tag；不要把无法追溯的 `latest` 当成唯一版本标识。数据库 schema 发生变化时，必须在应用替换前执行备份与迁移。

如果生产服务器无法访问默认 Go / Alpine 源，可通过 Compose `.env` 覆盖构建源：

```dotenv
OMNIFLOW_BUILD_GOPROXY=https://goproxy.cn,direct
OMNIFLOW_BUILD_ALPINE_MIRROR=mirrors.cloud.tencent.com
```

这些变量只影响 image 构建，不进入应用运行时配置；Go module 仍由 `go.sum` 校验完整性。

## 8. 验证方式

本次已完成以下验证：

- `docker version` 可访问 Docker Desktop daemon。
- `docker network ls` 显示默认 `bridge`、`host`、`none` 网络存在。
- `docker build -t omniflow-go:local .` 构建成功。
- `docker run` 使用 Docker 专用配置启动成功。
- 容器内部访问 `http://127.0.0.1:8850/healthz` 成功。
- 宿主机访问 `http://127.0.0.1:8850/healthz` 成功。
- 已确认 MinIO 预签名 URL 应使用 `public_endpoint=localhost:9000` 返回给宿主机前端；`host.docker.internal:9000` 只用于容器内后端访问宿主机 MinIO。

未覆盖风险：

- 生产发布脚本会额外调用用户存在性只读接口，验证 PostgreSQL 查询链路；Redis 与 MinIO 仍需通过对应业务接口单独验证。
- 当前文档覆盖本地 Docker Desktop 单容器运行，不覆盖正式环境镜像仓库推送、CI 构建缓存、生产发布编排。

## 9. 后续维护

以下变更必须同步更新本文档：

- Dockerfile 构建流程、基础镜像、启动命令变化。
- 服务端口、配置路径、`storage.yaml` 加载规则变化。
- Docker 专用配置文件路径或挂载方式变化。
- 发布流程从本地 `docker run` 切换为 Compose、Kubernetes 或 CI/CD。
- `OMNIFLOW_*` 环境变量白名单、生产目录或单机发布流程变化。
