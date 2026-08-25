# 资源与操作 API

本文说明 C++ SDK 在 descriptor v2 下的资源页面生成 API 约定。

当前目标模型不定义独立的资源运行时 API。SDK 只上报 FunctionContract，由 Server 归一化为 ResourceCapability / CapabilitySemantics，再生成 PageProposal。页面分类、动态 labels、页面类型和位置只在 Page Studio / PageSpec 中确定。

## 必要字段

建议提供：

- `id`
- `version`
- `summary`
- `description`
- `resource`
- `operation`
- `capability`
- `risk`
- `input_schema`（成员名；契约键 `inputSchema`）
- `output_schema`（成员名；契约键 `outputSchema`）

SDK descriptor 不提供页面 schema、组件树、页面 mapping、菜单、分类显示名、页面标题、按钮文案或页面位置。

## 继续阅读

- [资源与操作指南](/sdks/cpp/guide/resources-and-operations)
- [OpenAPI / SDK Descriptor v2](/architecture/openapi-sdk-descriptor-v2)
- [Dashboard Resource/Page 模型](/architecture/dashboard-page-model)
