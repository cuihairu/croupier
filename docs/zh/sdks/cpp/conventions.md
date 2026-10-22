# Croupier C++ SDK 约定

## 命名约定

函数 ID 采用 `[namespace.]resource.operation`：

```cpp
"player.get"
"player.ban"
"inventory.item.add"
```

## 注册约定

- 在 `Connect()` 之前完成注册
- 函数 ID 在同一服务实例内唯一
- 描述符至少包含 `id` 与 `version`

## 描述符建议

填写基础描述字段：

- `summary`
- `description`
- `tags`
- `input_schema`（C++ 成员名，对应契约键 `inputSchema`）
- `output_schema`（C++ 成员名，对应契约键 `outputSchema`）

填写业务与治理字段：

- `resource`
- `operation`
- `capability`
- `risk`
- `execution`（ExecutionSemantics：timeout/retry/concurrency 语义）
- `approvalRequired` / `approvalPolicyKey`（ApprovalPolicy）
- `enabled`
- `permission`

语义约束：

- `operation` 是业务操作键，例如 `ban`、`grant`、`send`。
- SDK 描述符不提供页面 schema、组件树、页面映射、菜单、分类显示名、页面标题、按钮文案或页面摆放。
- 动态分类、页面标题、按钮文案与页面摆放只在 PageSpec / Page Studio 决定，不写入 SDK 描述符，也不写入前端静态 locale 文件。
- `capability` 只允许受控的资源语义：`collection_query/item_query/create/update/delete/action/task/report`。它不是页面类型，也不是按钮摆放。
- 默认 PageProposal 由 Server 基于 FunctionContract、JSON Schema、CapabilitySemantics 与诊断信息生成；SDK 不提供列、映射、菜单或 UI。

完整契约见 [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)。

## 实践建议

- 函数参数按 ID 引用业务对象，不要传大对象
- 高危操作显式设置 `risk`
