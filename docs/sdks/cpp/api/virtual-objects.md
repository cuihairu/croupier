# 虚拟对象 API

本文说明 C++ SDK 在 descriptor v2 下的资源页面生成 API 约定。

当前目标模型不定义独立的虚拟对象运行时 API。SDK 只上报函数能力契约，由 Server 归一化为 `FunctionSpec / ResourceSpec / OperationSpec`，再生成 PageSpec 候选。页面分类、动态 labels、页面类型和位置只在 Page Studio / PageSpec 中确定。

## 必要字段

建议提供：

- `id`
- `version`
- `summary`
- `description`
- `resource`
- `operation`
- `risk`
- `input_schema`
- `output_schema`

不得在 SDK descriptor 中提供 `entity_display`、`operation_display`、`category_display`、`operation_kind`、`placement`、`page_hint` 或任何 Page UI 配置。

## 继续阅读

- [虚拟对象指南](/sdks/cpp/guide/virtual-objects)
- [OpenAPI / SDK Descriptor v2](/architecture/openapi-sdk-descriptor-v2)
- [Dashboard Resource/Page 模型](/architecture/dashboard-page-model)
