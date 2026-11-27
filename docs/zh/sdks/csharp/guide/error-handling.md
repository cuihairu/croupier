# 错误处理

所有函数调用都应有明确的错误结构和日志上下文。

## 建议

- 业务错误与系统错误区分编码
- 对连接失败、超时和不可用状态做统一重试策略——但不要盲目重试写请求：
  `RetryConfig.RetryWrites` 默认 `false`，写请求耗尽重试后返回
  `InvokeResult.ErrorCode == "RESULT_UNKNOWN"`（`StartTaskAsync` 抛 `ResultUnknownException`），
  因为请求可能在失败前已生效。重放前须用自己的幂等键做对账
- 将待审批视为独立的请求结果而非失败：`InvokeAsync` 返回
  `InvokeResult.ApprovalRequired == true` 与 `ApprovalId`，`StartTaskAsync` 因未派发任务
  抛 `ApprovalRequiredException`。保留审批 ID 并跟进审批流，不要重试该写请求
- 记录 `FunctionId`、`CallId`、耗时和错误码
