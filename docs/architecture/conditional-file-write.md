# 文件内容条件写入

## 契约

`PUT /api/v1/nodes/:nodeId/content/conditional` 接受现有内容保存参数 `libraryId`、`content`、`contentType`、`storageProvider`，并要求 `expectedStorageKey`。条件来自节点详情；空字符串表示尚未绑定存储的空文件，缺失或 null 返回 400。版本不符返回 409。节点 ID、名称、父目录保持不变。

保留原有 `/content` 接口的无条件写入兼容性，也支持可选条件。Agent 必须使用独立条件端点；旧服务返回 404 时不得回退到无条件保存。

`dryRun=true` 和执行共用身份、权限、类型、版本、Provider 校验；dry-run 不上传对象、不开启写事务。执行先上传新对象，再由 NodeUseCase 开启事务，仓储锁定 Node 行、再次检查存储条件并原子替换存储元数据。明确失败清理新对象；事务提交结果未知时保留新对象，避免删除可能已提交的内容。提交成功后才清理旧对象。

## CLI

`of fs write --library-id 3 --node-id 9 --file ./notes.txt [--expected-storage-key <key>] [--dry-run] [--json]`

CLI 读取本机 UTF-8 文本，最多 8 MiB，通过 HTTP 修改既有节点。显式 `--expected-storage-key ''` 会传空条件，不等同于省略。新文件仍走上传链路。不要把私有存储 key 放进 Agent 的模型结果；前端对模型提供的是正文 SHA-256 revision。

## 验证

sqlmock 覆盖条件匹配、旧版本拒绝、存储绑定缺失及中间更新失败回滚；usecase 覆盖 dry-run 无上传及执行上传失败；handler 覆盖必填条件，CLI 覆盖条件端点、空条件、认证、正文和 dry-run 透传。测试不写真实资料库。
