---
title: OpenAPI 导入
---

# OpenAPI 导入

Python SDK 提供与 Go SDK `RegisterFromOpenAPI` 对齐的导入 helper。它把 OpenAPI 3 spec 的每个 operation 转换为一个 `FunctionDescriptor` 并注册：

```python
from croupier import CroupierClient, ImportOptions, register_from_openapi

client = CroupierClient(config)
registered = register_from_openapi(
    client,
    spec_data,  # JSON string/bytes or an already-parsed dict
    ImportOptions(resource_prefix="game", tag_prefix="svc-"),
    handler_resolver=lambda function_id: handlers.get(function_id),
)
```

handler 通过 operationId（或从 path 派生的兜底 ID）查找；也可以传 `handlers={"player_ban": handler}` 映射来代替 `handler_resolver`。

## ImportOptions

- `resource_prefix`：为非空的 `x-resource` 值加前缀，如 `game.player`
- `tag_prefix`：为 tags 加前缀
- `default_timeout_ms`：为与 Go 的 ImportOptions 对齐而保留；Python 描述符尚无 timeout 字段
- `continue_on_error`：转换或注册失败时跳过该 operation，而不是抛错

## 字段转换规则

| OpenAPI 来源                                  | FunctionDescriptor 字段                     |
| --------------------------------------------- | ------------------------------------------- |
| `operationId`（缺失时从 path 派生 `a.b.c`）   | `id`                                        |
| `summary`（缺失时 titleCase(operationId)）    | `summary`                                   |
| requestBody `application/json` schema         | `input_schema`                              |
| 200 响应 `application/json` schema            | `output_schema`                             |
| `x-resource` / `x-operation` / `x-permission` | `resource` / `operation` / `permission`     |
| `x-capability`                                | `capability`（受控枚举，非法值丢弃）        |
| `x-execution`                                 | `execution`（`sync` \| `task`，非法值丢弃） |
| `x-approval`                                  | `approval_required` / `approval_policy_key` |
| `x-risk`                                      | `risk`                                      |

`x-approval` 形如 `{"required": true, "policyKey": "gm.player.ban"}`，与 execution 正交。

`x-risk` 输出会归一化为规范词表 `safe|warning|high|danger`；`low`/`medium`/`moderate` 是已废弃别名，导入时分别归一化为 `safe`/`warning`。

## 完整示例

见 `sdks/python/examples/game_demo.py` 与 `sdks/python/tests/test_openapi.py`。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
