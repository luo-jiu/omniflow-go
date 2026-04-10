# 日志配置规范（omniflow-go）

## 目标

- 本地调试可读性高：支持控制台输出，按需落文件。
- 线上可观测性强：默认 JSON 输出到 stdout，便于平台采集到 ES/Loki/EFK。
- 配置优先：不改代码即可切换日志级别、格式和输出目的地。

## 配置项

```yaml
log:
  format: text            # text | json
  level: debug            # debug | info | warn | error
  add_source: false       # 是否输出源码位置
  console:
    enabled: true
  file:
    enabled: false
    path: ./logs/omniflow-go.log
    max_size_mb: 100
    max_backups: 10
    max_age_days: 30
    compress: true
    local_time: true
```

## 默认策略

- `server.mode=debug`：
  - 默认 `log.format=text`
  - 默认 `log.level=debug`
- `server.mode=release`：
  - 默认 `log.format=json`
  - 默认 `log.level=info`

## 严格校验

以下配置项为严格校验，配置非法会导致启动失败（fail-fast）：

- `server.mode`：`debug | release | test`
- `log.level`：`debug | info | warn | error`
- `log.format`：`text | json`

如果 `log.file.enabled=true` 但 `log.file.path` 为空，系统会跳过文件输出并记录 warning，不会中断服务。

## 推荐实践

- 本地开发：
  - `console.enabled=true`
  - `file.enabled=true`（需要历史日志时）
- 测试/预发/生产：
  - `console.enabled=true`
  - `file.enabled=false`
  - 由容器平台采集 stdout

## 设计说明

- 文件日志使用滚动切割（`lumberjack`），避免单文件无限增长。
- 日志统一附带 `service/env/version` 基础字段。
- 当所有输出都被关闭时，会自动回退到 stdout，避免“静默无日志”。
