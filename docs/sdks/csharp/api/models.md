# 模型 API 参考

## 核心模型

- `FunctionDescriptor`
- `ClientConfig`
- `InvokeOptions`
- `InvokeResult`
- `FunctionContext`
- `JobStatus`
- `BatchInvokeRequest`

## FunctionDescriptor v2 字段

| 字段                | 说明                                                                                   |
| ------------------- | -------------------------------------------------------------------------------------- |
| `Capability`        | 能力类型：`collection_query\|item_query\|create\|update\|delete\|action\|task\|report` |
| `Execution`         | 执行方式：`sync\|task`                                                                 |
| `ApprovalRequired`  | 是否必须在执行前完成审批                                                               |
| `ApprovalPolicyKey` | 可选审批流程标识                                                                       |
| `Risk`              | 风险级别：`safe\|warning\|high\|danger`                                                |
| `Resource`          | 业务资源或能力域标识                                                                   |
| `Operation`         | 业务动作标识                                                                           |
| `Permission`        | 权限标识                                                                               |

审批与执行方式正交：需要审批的批量操作可同时设置 `Execution = "task"` 与 `ApprovalRequired = true`。OpenAPI 导入的对应扩展字段（`x-capability`/`x-execution`/`x-approval` 等）见 [OpenAPI 导入指南](../guide/openapi-import)。
