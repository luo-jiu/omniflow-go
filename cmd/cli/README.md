# OmniFlow CLI 开发规范

本目录存放 CLI 入口。当前 CLI 采用“薄入口 + 传输层命令实现”的结构：

- 入口：`cmd/cli/main.go`
- 命令实现：`internal/transport/cli/*.go`
- 网络访问：`internal/transport/cli/client.go`
- 本地会话：`internal/transport/cli/config.go`
- Agent 作战手册：`docs/architecture/cli-agent-development-playbook.md`

## 1. 设计目标

- 长期可维护：命令按子域拆文件，不在单文件堆积所有逻辑。
- 行为一致：CLI 调用同一套 HTTP API，复用后端 `code/message/data/request_id` 契约。
- 易于自动化：所有命令支持非交互；关键命令支持 `--json`。

## 2. 目录与职责

- `cmd/cli/main.go`
  - 仅做进程入口，不放业务逻辑。
- `internal/transport/cli/app.go`
  - 命令树、路由分发、帮助信息。
- `internal/transport/cli/command_*.go`
  - 各子命令实现（auth/lib/fs/config/health）。
- `internal/transport/cli/helpers.go`
  - 通用输出、flag 解析、会话检查与通用解析工具。
- `internal/transport/cli/client.go`
  - HTTP 请求封装、鉴权头注入、响应壳解析、错误映射。
- `internal/transport/cli/config.go`
  - `~/.omniflow/cli.json` 读写与环境变量覆盖。

## 3. 命令设计约定

- 子命令优先：`of <domain> <action>`，例如 `of auth login`。
- 参数命名统一使用 kebab-case：`--library-id`、`--base-url`。
- 对脚本友好的输出：
  - 人类可读模式：简洁单行文本。
  - 机器可读模式：`--json` 输出结构化 JSON。
- 鉴权命令必须复用本地会话逻辑，不重复读写 token 文件。
- 帮助信息采用渐进式披露：
  - `of help <domain>` 看命令域列表。
  - `of help <domain> <command>` 看 usage + flags。
  - `of help <domain> <command> --examples` 看完整示例。
- 文件系统命令分层组织：
  - 节点操作：`mkdir` / `rename` / `mv` / `rm`
  - 归档操作：`archive batch-set-built-in-type`
  - 回收站操作：`recycle ls|restore|hard`
  - 路径工具：`path resolve`
- 路径演进策略：写命令保持 `id` 参数兼容，同时逐步增加 `path` 参数入口。

## 4. 错误与退出码约定

- 成功退出码：`0`
- 失败退出码：`1`
- 错误输出统一写入 stderr，格式：`error: <message>`
- API 错误保留后端 message/code，避免 CLI 私自改写语义。
- 命令不接受未声明的位置参数，出现多余参数应直接失败。
- `of auth status` 在未登录态会返回非零退出码，便于脚本判断。

## 5. 配置与安全约定

- 默认地址：`http://127.0.0.1:8850`
- 本地会话文件：`~/.omniflow/cli.json`（权限 0600）
- 配置文件损坏时 CLI 会回落到默认配置，允许用户通过重新登录自恢复。
- 支持环境变量覆盖：
  - `OMNIFLOW_BASE_URL`
  - `OMNIFLOW_USERNAME`
  - `OMNIFLOW_TOKEN`
- 新增敏感字段时必须评估存储安全，不得明文打印到 stdout。

## 6. 新增命令 Checklist

1. 在 `internal/transport/cli/command_<domain>.go` 增加实现函数。
2. 在 `internal/transport/cli/app.go` 注册到命令树并补 Usage/Summary。
3. 补 `--json` 输出（如命令结果可结构化）。
4. 复用 `resolveClient` / `ensureSession`，避免重复鉴权逻辑。
5. 执行 `go test ./...`，确保回归通过。
6. 更新：
   - `docs/architecture/cli-minimal-quickstart.md`
   - 本文档（如新增了通用约定）。

## 7. 构建与验证

```bash
cd /Users/loyce/personal/omniflow/omniflow-go
GOCACHE=/tmp/go-build go build -o ./bin/of ./cmd/cli
./bin/of --help
```
