# 资源与操作

本文说明 C++ SDK 如何通过 descriptor v2 表达资源页面生成语义。当前目标模型是 `ResourceSpec + OperationSpec + PageSpec`。

资源不是数据库表，也不是 SDK 侧的运行时对象。它只是一组函数围绕同一业务能力域组织出的稳定 key，例如钱包、背包、订单、玩家资料。SDK descriptor 只提供函数能力契约；页面分类、动态 labels、页面类型和位置必须在 Page Studio / PageSpec 中确定。

## ID 引用模式

函数不应通过参数传递大对象实例，而应通过稳定 ID 引用业务对象：

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
| `resource` | 业务资源或能力域 key |
| `operation` | 业务操作 key |
| `capability` | 受控资源语义，例如 `collection_query`、`update`、`action` |
| `risk` | 治理风险等级 |

## 建议

- 默认页面生成基于输入/输出契约和 CapabilitySemantics；SDK 可提供受控 `capability`，不能让前端根据函数名猜测。
- 动态分类、资源标题、操作标题和按钮位置必须随 PageSpec 提供，不写入 SDK descriptor 或前端静态 locale 文件。
- 使用 ID 引用，不传递笨重对象实例。
- 函数保持无状态，通过 Repository 或业务服务查找对象。
- 描述符 ID 和 operation key 应保持稳定。

## 相关页面

- [OpenAPI / SDK Descriptor v2](/architecture/openapi-sdk-descriptor-v2)
- [Dashboard Resource/Page 模型](/architecture/dashboard-page-model)
