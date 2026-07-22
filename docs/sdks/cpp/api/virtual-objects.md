# 虚拟对象 API

本文说明 C++ SDK 在 descriptor v2 下的资源页面生成 API 约定。

当前目标模型不定义独立的虚拟对象运行时 API。SDK 通过函数 descriptor v2 上报 `entity`、`operation_kind`、`placement` 和动态 labels，由 Server 归一化为 `ResourceSpec / OperationSpec / PageSpec`。

## 必要字段

需要参与默认页面生成的函数至少应提供：

- `id`
- `version`
- `summary`
- `description`
- `entity`
- `entity_display`
- `operation`
- `operation_display`
- `operation_kind`
- `placement`
- `category`
- `category_display`
- `input_schema`
- `output_schema`

## 继续阅读

- [虚拟对象指南](/sdks/cpp/guide/virtual-objects)
- [OpenAPI / SDK Descriptor v2](/architecture/openapi-sdk-descriptor-v2)
- [Dashboard Resource/Page 模型](/architecture/dashboard-page-model)
