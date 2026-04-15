# OmniFlow CLI Agent 开发作战手册

更新时间：2026-04-09  
适用对象：负责实现 OmniFlow CLI 的开发 Agent / 开发同学  
目标：让任意 Agent 在不了解上下文的情况下，也能按统一标准稳定推进 CLI 功能。

## 1. 当前状态快照

### 1.1 已稳定能力

- 基础域：
  - `of health`
  - `of auth login|status|whoami|logout`
  - `of config show`
- 文件域（读）：
  - `of lib ls`
  - `of fs ls`
  - `of fs search`
  - `of fs path resolve`
- 文件域（写）：
  - `of fs mkdir`（支持 `--parent-id`/`--parent-path`）
  - `of fs rename`
  - `of fs mv`（支持 `id/path` 混合输入策略）
  - `of fs rm`（支持 `--node-id`/`--path`）
- 回收站：
  - `of fs recycle ls`
  - `of fs recycle clear`
  - `of fs recycle restore|hard`（当前仅支持 `--node-id`，不支持 `--path`）

### 1.2 已知约束

- 回收站节点属于 deleted 集，不能用 live children 路径解析。
- 在后端未补“deleted 节点路径解析”API 前，禁止给 `recycle restore|hard` 暴露 `--path`。
- 命令层必须与后端语义一致，不能“先承诺、后兜底”。

## 2. 代码落点与职责

- 入口：`cmd/cli/main.go`
- 命令树与帮助：`internal/transport/cli/app.go`
- 命令实现：`internal/transport/cli/command_*.go`
- HTTP 客户端：`internal/transport/cli/client.go`
- 本地会话：`internal/transport/cli/config.go`
- 通用工具：`internal/transport/cli/helpers.go`
- 单测：
  - 路由/help/参数：`internal/transport/cli/app_test.go`
  - 工具函数：`internal/transport/cli/helpers_test.go`
  - 配置：`internal/transport/cli/config_test.go`

## 3. CLI 设计硬规则

### 3.1 命令形态

- 统一用：`of <domain> <action> [flags]`
- 禁止隐式位置参数（除了 `help` 的命令路径）
- 参数命名统一 kebab-case

### 3.2 输出与退出码

- 成功：exit code `0`
- 失败：exit code `1`
- 错误统一 stderr：`error: <message>`
- 支持机器调用的命令必须支持 `--json`

### 3.3 帮助系统（渐进披露）

- `of help <domain>`：看子命令
- `of help <domain> <command>`：看 usage + flags
- `of help <domain> <command> --examples`：看完整示例
- 更新命令时，必须同步 `Usage/Flags/Examples`

### 3.4 参数兼容策略（id/path）

- 写命令优先维持既有 `--*-id`，新增 `--*-path` 作为增强入口。
- 如果支持双模式，必须满足：
  - 二选一必填
  - 同时给两者时报错
  - 错误信息明确指出冲突参数
- 不满足语义一致性时，不要暴露 path 模式。

## 4. Agent 开发流程（强制）

### 4.1 开发前

1. 先读这 3 个文档：
   - `cmd/cli/README.md`
   - `docs/architecture/cli-minimal-quickstart.md`
   - `docs/progress/cli-development-plan-status.md`
2. 查后端路由与 handler，再决定是否可做 path 模式。

### 4.2 开发中

1. 先改 `command_*.go` 逻辑。
2. 再改 `app.go` 的命令树、help 元数据。
3. 按需改 `client.go` 接口封装。
4. 同步补测试：
   - 至少 1 条成功路径或可替代验证
   - 至少 1 条参数失败路径

### 4.3 开发后（门禁）

1. `gofmt` 相关 Go 文件
2. `GOCACHE=/tmp/go-build go test ./...`
3. `GOCACHE=/tmp/go-build go build -o ./bin/of ./cmd/cli`
4. 手工 smoke：
   - `./bin/of --help`
   - 变更命令 `--help --examples`
   - 新增命令最小错误路径
5. 更新文档：
   - `docs/architecture/cli-minimal-quickstart.md`
   - `docs/progress/cli-development-plan-status.md`

## 5. 高标准 Review 清单

每次提交前，按以下顺序 review：

1. 语义一致性：
   - CLI 输入/输出是否与后端真实行为一致
   - 文档是否承诺了实际上不可用的能力
2. 参数行为：
   - 必填、互斥、默认值是否清晰
   - 错误信息是否可直接指导修复
3. 自动化友好性：
   - 是否有 `--json`
   - 非交互路径是否完整
4. 帮助可读性：
   - usage/flags/examples 是否同步
5. 测试可信度：
   - 测试是否模拟了真实业务语义，而不是“人为让它过”

## 6. 未来两个月路线（按主业务优先级）

RAG 相关延后。当前 CLI 主线应跟随文件能力建设。

### 6.1 本月：传输能力月

#### 阶段 A：传输域骨架

- 新增命令域：`of xfer ...`
- 先落统一任务模型（task id / status / retry / error）
- 统一 `--json` 输出和状态码

#### 阶段 B：大文件分片断点续传

- 目标命令（建议）：
  - `of xfer upload init`
  - `of xfer upload part`
  - `of xfer upload complete`
  - `of xfer upload resume`
  - `of xfer upload status`
  - `of xfer upload cancel`
- 能力要求：
  - 失败可恢复
  - 并发可配置
  - 分片校验可观测

#### 阶段 C：监控文件夹自动上传

- 目标命令（建议）：
  - `of watch folder start`
  - `of watch folder stop`
  - `of watch folder status`
  - `of watch folder logs`
- 能力要求：
  - include/exclude 规则
  - 防抖和重试策略
  - 崩溃后可恢复

#### 阶段 D：下载截取/范围下载

- 目标命令（建议）：
  - `of xfer download --node-id|--path --offset --length`
  - 或 `--range bytes=a-b`
- 能力要求：
  - 与后端 Range 语义对齐
  - 输出元数据可追踪

### 6.2 下月：编辑能力月

#### 阶段 E：文件编辑基础流

- 目标命令（建议）：
  - `of file edit checkout`
  - `of file edit save`
  - `of file edit commit`
  - `of file edit discard`

#### 阶段 F：冲突与版本保护

- 引入版本号/ETag 检查
- 明确冲突错误码与 CLI 提示

## 7. DoD（定义完成）

一个 CLI 功能只有在满足以下全部条件时才算完成：

1. 命令逻辑实现完成，help 信息完整。
2. 参数校验路径覆盖，错误语义明确。
3. `go test ./...` 和 `go build` 全部通过。
4. 文档与进度台账已更新。
5. Review 未发现 P1/P2 问题。

## 8. 常见踩坑

- 把“暂不可用能力”写进 usage/examples。
- 测试只测 happy path，不测参数冲突和失败路径。
- 命令实现改了，但忘记同步 `app.go` 的 help 元数据。
- API 错误被 CLI 改写导致排障困难。
- 并行运行命令导致读取过期二进制结果，误判验证状态。

## 9. 推荐提交粒度

- 单个 commit 聚焦一个能力点，避免混合“功能 + 大量结构调整”。
- 推荐格式：
  - `feat(cli): add xxx command`
  - `fix(cli): align xxx semantics`
  - `docs(cli): sync xxx playbook`

---

如果后续由多个 Agent 并行开发，优先按“命令域”分配所有权，避免在同一文件（特别是 `app.go`）发生高频冲突。
