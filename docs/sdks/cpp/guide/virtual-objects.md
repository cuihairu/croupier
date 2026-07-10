# 虚拟对象

本文是 C++ SDK 虚拟对象注册机制的 canonical 入口。虚拟对象用于把相关 CRUD 和业务操作组织成实体管理单元，适合钱包、背包、订单、玩家资料等复杂业务域。

## 四层模型

```text
Function Level    单个原子操作，例如 wallet.transfer
Entity Level      业务对象模型，例如 wallet.entity
Resource Level    UI 资源组织，例如钱包管理面板
Component Level   可分发模块，例如 economy-system
```

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

## 描述符要素

| 字段 | 说明 |
| --- | --- |
| `id` | 虚拟对象唯一标识，例如 `wallet.entity` |
| `version` | 描述符版本 |
| `name` | 展示名称 |
| `description` | 业务说明 |
| `schema` | JSON Schema 对象结构 |
| `operations` | 操作到函数 ID 的映射 |
| `relationships` | 与其他实体的关系 |

## 建议

- CRUD 操作尽量完整，避免 Dashboard 只能生成半成品页面。
- 关系定义保持清晰，避免 UI 和权限侧猜测对象关系。
- 使用 ID 引用，不传递笨重对象实例。
- 函数保持无状态，通过 Repository 或业务服务查找对象。
- 描述符 ID 和 operation key 应保持稳定。

## 相关页面

- [虚拟对象 API](/sdks/cpp/api/virtual-objects)
- [虚拟对象示例](/sdks/cpp/examples/virtual-object)
