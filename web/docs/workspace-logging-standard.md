# Workspace 日志标准（V1）

目标：统一 Workspace 相关接口与任务日志格式，便于检索、告警、审计串联。

## 1. 结构化日志字段

所有 Workspace 业务日志统一使用 JSON 结构，建议字段如下：

- `ts`：ISO8601 时间戳。
- `level`：`debug/info/warn/error`。
- `service`：服务名（例如 `server`）。
- `module`：固定为 `workspace`。
- `action`：动作名（`save/publish/unpublish/rollback/delete/list/detail`）。
- `object_key`：对象标识。
- `version`：目标版本号（可选）。
- `request_id`：请求链路 ID。
- `user_id`：操作者标识。
- `result`：`success/failed`。
- `latency_ms`：耗时。
- `error_code`：失败时业务错误码。
- `error_message`：失败时摘要信息（避免泄露敏感内容）。

## 2. 错误日志规范

- 错误日志必须包含：`request_id/object_key/action/error_code`。
- 错误日志必须区分：
  - 业务错误：`level=warn`，带业务 `error_code`。
  - 系统错误：`level=error`，带异常栈 `stack`。
- 错误日志禁止输出：
  - Token、密码、完整用户隐私数据。
  - 过长原始配置全文（建议输出 `config_hash` 或摘要）。

## 3. 推荐日志示例

```json
{
  "ts": "2026-03-08T10:23:00.123Z",
  "level": "info",
  "service": "server",
  "module": "workspace",
  "action": "publish",
  "object_key": "player",
  "version": 12,
  "request_id": "req_abc123",
  "user_id": "admin",
  "result": "success",
  "latency_ms": 42
}
```

```json
{
  "ts": "2026-03-08T10:24:01.002Z",
  "level": "warn",
  "service": "server",
  "module": "workspace",
  "action": "rollback",
  "object_key": "player",
  "request_id": "req_def456",
  "user_id": "admin",
  "result": "failed",
  "error_code": "workspace_version_not_found",
  "error_message": "target version not found",
  "latency_ms": 31
}
```

## 4. 与审计日志协同

- 审计日志保留“谁在什么时候做了什么”。
- 业务运行日志补充“执行细节与失败原因”。
- 通过 `request_id + object_key + action` 关联两者，形成完整排障链路。
