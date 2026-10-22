# 资源与操作 API

本页说明 C++ SDK 在 descriptor v2 下的资源页生成 API 约定。

当前目标模型没有定义独立的资源运行时 API。SDK 只上报 FunctionContract；Server 将其归一化为 ResourceCapability / CapabilitySemantics，再生成 PageProposal。页面分类、动态标签、页面类型与摆放只在 Page Studio / PageSpec 决定。

## 必填字段

需提供以下字段：

- `id`
- `version`
- `summary`
- `description`
- `resource`
- `operation`
- `capability`
- `execution`（`sync` / `task`）
- `risk`
- `input_schema`（C++ 成员名；契约键 `inputSchema`）
- `output_schema`（C++ 成员名；契约键 `outputSchema`）
- 高危操作另需：`approval_required`（契约键 `approvalRequired`）与 `approval_policy_key`（契约键 `approvalPolicyKey`）

SDK 描述符不提供页面 schema、组件树、页面映射、菜单、分类显示名、页面标题、按钮文案或页面摆放。

## 继续阅读
