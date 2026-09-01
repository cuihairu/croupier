# Croupier Go SDK 约定规范

## 命名约定

函数 ID 使用 `[namespace.]resource.operation`：

```go
"player.get"
"player.ban"
"inventory.item.add"
```

## 注册约定

- 在客户端启动前完成函数注册
- 同一服务实例内函数 ID 保持唯一
- 高风险操作明确填写 `Risk`

## 描述符建议

最小可注册字段：

- `id`
- `version`

建议补齐基础说明：

- `summary`
- `description`
- `tags`
- `inputSchema`
- `outputSchema`

建议补齐业务与治理信息：

- `resource`
- `operation`
- `capability`
- `risk`
- `enabled`
- `permission`
- `execution`（ExecutionSemantics：超时/重试/并发语义）
- `approvalRequired`（是否需审批）
- `approvalPolicyKey`（审批策略键；两字段合称 ApprovalPolicy）

语义约束：

- `operation` 是业务操作 key，例如 `ban`、`grant`、`send`。
- SDK descriptor 不提供页面 schema、组件树、页面 mapping、菜单、分类显示名、页面标题、按钮文案或页面位置。
- 动态分类、页面标题、按钮文案和页面位置只在 PageSpec / Page Studio 中确定，不写入 SDK descriptor，也不写入前端静态 locale 文件。
- `capability` 只允许受控资源语义：`collection_query/item_query/create/update/delete/action/task/report`；它不是页面类型或按钮位置。
- 默认 PageProposal 由 Server 根据 FunctionContract、JSON Schema、CapabilitySemantics 和 diagnostics 生成；SDK 不提供列、mapping、菜单或 UI。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
