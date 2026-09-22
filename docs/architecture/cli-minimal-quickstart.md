# OmniFlow CLI 最小版快速使用

批量改名：`of fs rename-batch --library-id 3 --operation-id <uuid> --file proposal.json --dry-run --json`；回执核对：`of fs rename-status --library-id 3 --operation-id <uuid> --json`。proposal.json 保存完整旧值和新主体名的 items 数组；详见 [条件批量改名契约](node-conditional-rename.md)。

节点标签：`of fs tags --library-id 3 --node-id 9 --json`；增删预演：`of fs tags-update --library-id 3 --node-id 9 --add-tag-ids 2 --remove-tag-ids 4 --dry-run --json`。见 [节点标签增量契约](node-tag-delta.md)。

## 分页浏览与搜索元数据

下面的 `3` 仅为示例，测试须换成非第一个资料库的实际 ID：

```bash
of fs browse --library-id 3 --mode root --json
of fs browse --library-id 3 --mode children --limit 20 --json
of fs browse --library-id 3 --mode search --keyword 音乐 --node-type dir --json
of fs browse --library-id 3 --mode node --node-id 88 --json
```

子目录传 `--parent-id`；子树搜索传 `--ancestor-id`。响应 `hasMore=true` 时保留筛选条件并传 `--cursor <nextCursor>` 续页。元数据可读不代表对象存储在线；规范路径和安全字段见 [查询契约](node-metadata-query.md)。

该 CLI 放在同仓库，目标是最小可用：

- 支持登录态管理（本地配置）
- 支持基础健康检查
- 支持资料库列表
- 支持文件树查询（children/search）
- 支持文件树基础写操作（mkdir/rename/mv/rm）与回收站管理
- 支持浏览器文件映射管理（list/resolve/create/update/delete）
- 支持资源监测样本采集

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
./bin/of fs mkdir --library-id <id> --name <name> [--parent-id <id>|--parent-path </a/b>] [--conflict-policy <error|auto_rename>]
./bin/of fs rename --node-id <id> --name <new_name>
./bin/of help fs configure --examples
./bin/of fs configure --node-id <id> [--built-in-type <type>] [--archive-mode <0|1>] [--view-meta <json>] [--dry-run] [--json]
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
./bin/of upload file --library-id <id> --file <path> [--parent-id <id>] [--storage-provider <id>] [--conflict-policy <error|auto_rename|replace>] [--content-type <type>] [--json]
./bin/of storage migrate --library-id <id> --node-id <id> --target-provider <alias> [--dry-run] [--json]
./bin/of storage distribution --library-id <id> --node-id <id> [--json]
./bin/of storage migration ls [--library-id <id>] [--status running,pending] [--limit <n>] [--json]
./bin/of storage migration status --task-id <id> [--items] [--json]
./bin/of storage migration cancel --task-id <id> [--dry-run] [--json]
./bin/of resource-monitor sample [--library-id <id>] [--dry-run] [--json]
```

上传 complete 遇到网络错误、限流或服务端 `5xx` 时，CLI 会先用本次稳定 operation ID 查询服务端完成状态。若仍无法确认，CLI 会输出 operation ID 并保留上传会话，不会自动 abort 或再次上传。

支持 `--json` 的命令会输出结构化结果，便于脚本和 AI 调用。

`fs configure` 适合准备可重复的 viewer 测试夹具。三个可配置字段至少提供一个；未提供的字段保持原值，`--view-meta` 必须是 JSON 对象。建议先执行 `--dry-run --json`，确认后再去掉 `--dry-run` 真实写入。

`fs mkdir` 的 `--conflict-policy` 与后端节点创建一致：

- 省略或 `error`：同目录重名时返回 `409`
- `auto_rename`：自动改名为 `name`、`name (1)`、`name (2)`

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

## 文本内容保存补充

内部复制：`of fs cp --library-id 3 --node-id 9 --parent-id 10 --name copy.txt --dry-run --json`。目录加 `--recursive`，重名可选 `--conflict-policy auto_rename`；只支持同库显式 ID，详见 [复制契约](node-copy-contract.md)。

`of fs write --library-id 3 --node-id 9 --file ./notes.txt --dry-run --json` 验证已有文件的文本保存，去掉 `--dry-run` 执行。可选 `--expected-storage-key` 提供版本条件；详见 [条件写入契约](conditional-file-write.md)。
