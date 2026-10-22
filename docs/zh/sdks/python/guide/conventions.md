# Croupier Python SDK 约定

## 命名约定

函数 ID 使用 `resource.operation` 或 `[namespace.]resource.operation`：

```python
"player.get"
"player.ban"
"game.player.ban"
```

## 注册约定

所有函数必须在 `connect()` 之前注册：

```python
client.register_function(descriptor, handler)
client.connect()
```

## 描述符建议

可注册的最小字段：

- `id`
- `version`

填写基础描述字段：

- `summary`
- `description`
- `tags`
- `input_schema`（Python 字段名；对应契约键 `inputSchema`）
- `output_schema`（Python 字段名；对应契约键 `outputSchema`）

填写业务与治理字段：

- `resource`
- `operation`
- `capability`
- `execution`（`sync` 或 `task`）
- `approval_required` / `approval_policy_key`（Python 字段名；对应契约键 `approvalRequired` / `approvalPolicyKey`）
- `risk`（词表 `safe/warning/high/danger`；`low`/`medium` 是已废弃别名）
- `enabled`
- `permission`

语义约束：

- `operation` 是业务操作键，如 `ban`、`grant`、`send`。
- SDK 描述符不提供页面 schema、组件树、页面映射、菜单、分类显示名、页面标题、按钮文案或页面摆放。
- 动态分类、页面标题、按钮文案与页面摆放只在 PageSpec / Page Studio 中决定。它们不写入 SDK 描述符，也不写入前端静态 locale 文件。
- `capability` 只允许受控的资源语义：`collection_query/item_query/create/update/delete/action/task/report`。它不是页面类型，也不是按钮摆放。
- 默认 PageProposal 由 Server 基于 FunctionContract、JSON Schema、CapabilitySemantics 与诊断信息生成；SDK 不提供列、映射、菜单或 UI。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
