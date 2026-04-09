# OmniFlow CLI 最小版快速使用

该 CLI 放在同仓库，目标是最小可用：

- 支持登录态管理（本地配置）
- 支持基础健康检查
- 支持资料库列表
- 支持文件树查询（children/search）

## 1. 构建

```bash
cd /Users/loyce/personal/omniflow/omniflow-go
GOCACHE=/tmp/go-build go build -o ./bin/of ./cmd/cli
```

## 2. 可用命令

```bash
./bin/of --help
./bin/of config show
./bin/of health
./bin/of auth login --username <username> --password <password>
./bin/of auth status
./bin/of auth whoami
./bin/of auth logout
./bin/of lib ls --size 20
./bin/of fs ls --library-id <id> --node-id <id>
./bin/of fs search --library-id <id> --keyword <kw> --limit 20
```

支持 `--json` 的命令会输出结构化结果，便于脚本和 AI 调用。

## 3. 本地会话文件

CLI 会把登录会话写到：

`~/.omniflow/cli.json`

字段：

- `baseUrl`
- `username`
- `token`

可通过环境变量覆盖：

- `OMNIFLOW_BASE_URL`
- `OMNIFLOW_USERNAME`
- `OMNIFLOW_TOKEN`

