# 虚拟对象

本文说明 C++ SDK 如何通过 descriptor v2 表达资源页面生成语义。当前目标模型是 `ResourceSpec + OperationSpec + PageSpec`。

“虚拟对象”在当前文档中只表示一组函数围绕同一业务资源组织出的页面候选能力，例如钱包、背包、订单、玩家资料。实现上不需要单独的虚拟对象运行时协议，直接在函数 descriptor 中提供 `entity`、`operation_kind`、`placement` 和动态 labels。

## ID 引用模式

虚拟对象不应通过参数传递大对象实例，而应通过稳定 ID 引用业务对象：

```cpp
invoke("wallet.transfer", {
  {"from_player_id", "player123"},
  {"to_player_id", "player456"},
  {"currency_code", "gold"},
  {"amount", "100.0"}
});
```

这种方式能保持函数无状态，便于水平扩展、权限审计和 Dashboard 自动生成。

## Descriptor v2 要素

| 字段 | 说明 |
| --- | --- |
| `id` | 函数唯一标识，例如 `wallet.transfer` |
| `version` | 函数版本 |
| `summary` / `description` | 函数简介和详细说明 |
| `input_schema` / `output_schema` | JSON payload 输入输出契约 |
| `entity` / `entity_display` | 资源 key 和动态多语言标题 |
| `operation` / `operation_display` | 业务操作 key 和动态多语言标题 |
| `operation_kind` | 页面生成语义，例如 `list`、`get`、`action`、`task`、`report` |
| `placement` | 页面放置位置，例如 `tableData`、`rowAction`、`standalone` |
| `category` / `category_display` | 动态菜单分类 key 和多语言标题 |

## 建议

- 默认页面生成必须依赖明确的 `operation_kind` 和 `placement`，不能让前端根据函数名猜测。
- 动态分类、资源和操作标题必须随 descriptor 或 PageSpec 提供，不写入前端静态 locale 文件。
- 使用 ID 引用，不传递笨重对象实例。
- 函数保持无状态，通过 Repository 或业务服务查找对象。
- 描述符 ID 和 operation key 应保持稳定。

## 相关页面

- [虚拟对象与 Resource/Page 模型](/guide/concepts/virtual-objects)
- [OpenAPI / SDK Descriptor v2](/architecture/openapi-sdk-descriptor-v2)
- [Dashboard Resource/Page 模型](/architecture/dashboard-page-model)
