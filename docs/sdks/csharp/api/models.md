# Models API Reference

## Core Models

- `FunctionDescriptor`
- `ClientConfig`
- `InvokeOptions`
- `InvokeResult`
- `FunctionContext`
- `JobStatus`
- `BatchInvokeRequest`

## FunctionDescriptor v2 Fields

| Field               | Description                                                                                   |
| ------------------- | --------------------------------------------------------------------------------------------- |
| `Capability`        | Capability type: `collection_query\|item_query\|create\|update\|delete\|action\|task\|report` |
| `Execution`         | Execution mode: `sync\|task`                                                                  |
| `ApprovalRequired`  | Whether approval must complete before execution                                               |
| `ApprovalPolicyKey` | Optional approval workflow identifier                                                         |
| `Risk`              | Risk level: `safe\|warning\|high\|danger`                                                     |
| `Resource`          | Business resource or capability-domain identifier                                             |
| `Operation`         | Business action identifier                                                                    |
| `Permission`        | Permission identifier                                                                         |

Approval and execution mode are orthogonal: a batch operation that requires approval can set both `Execution = "task"` and `ApprovalRequired = true`. For the corresponding OpenAPI import extension fields (`x-capability`/`x-execution`/`x-approval`, etc.), see the OpenAPI import guide.
