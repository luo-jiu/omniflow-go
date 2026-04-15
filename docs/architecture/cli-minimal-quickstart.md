# OmniFlow CLI 最小版快速使用

该 CLI 放在同仓库，目标是最小可用：

- 支持登录态管理（本地配置）
- 支持基础健康检查
- 支持资料库列表
- 支持文件树查询（children/search）
- 支持文件树基础写操作（mkdir/rename/mv/rm）与回收站管理
- 支持浏览器文件映射管理（list/resolve/create/update/delete）

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
./bin/of help fs mkdir --examples
./bin/of fs mkdir --library-id <id> --name <name> [--parent-id <id>|--parent-path </a/b>]
./bin/of fs rename --node-id <id> --name <new_name>
./bin/of fs mv --library-id <id> (--node-id <id>|--node-path </a/b>) (--new-parent-id <id>|--new-parent-path </a/b>) [--before-node-id <id>] [--name <new_name>]
./bin/of fs rm --library-id <id> (--node-id <id>|--path </a/b>)
./bin/of fs ls --library-id <id> --node-id <id>
./bin/of fs search --library-id <id> --keyword <kw> --limit 20
./bin/of help fs archive batch-set-built-in-type --examples
./bin/of fs archive batch-set-built-in-type --node-id <id> [--dry-run] [--json]
./bin/of fs recycle ls --library-id <id>
./bin/of fs recycle clear --library-id <id> [--dry-run] [--json]
./bin/of fs recycle restore --library-id <id> --node-id <id>
./bin/of fs recycle hard --library-id <id> --node-id <id>
./bin/of fs path resolve --library-id <id> --path </docs/ch1>
./bin/of browser-map ls
./bin/of browser-map resolve --ext <ext>
./bin/of browser-map create --ext <ext> --url <url> [--dry-run] [--json]
./bin/of browser-map update --id <id> --ext <ext> --url <url> [--dry-run] [--json]
./bin/of browser-map rm --id <id> [--dry-run] [--json]
./bin/of browser-bookmark tree [--json]
./bin/of browser-bookmark match --url <url> [--json]
./bin/of browser-bookmark import --file <path> [--source <label>] [--dry-run] [--json]
./bin/of browser-bookmark create --title <title> [--kind <url|folder>] [--url <url>] [--dry-run] [--json]
./bin/of browser-bookmark update --id <id> [--title <title>] [--url <url>] [--icon-url <url>] [--clear-icon] [--dry-run] [--json]
./bin/of browser-bookmark move --id <id> [--parent-id <id>] [--before-id <id>|--after-id <id>] [--dry-run] [--json]
./bin/of browser-bookmark rm --id <id> [--dry-run] [--json]
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
