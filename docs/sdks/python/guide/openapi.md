---
title: OpenAPI 导入
---

# OpenAPI 导入

Python SDK 提供与 Go SDK `RegisterFromOpenAPI` 对齐的导入 helper，可把 OpenAPI 3 spec 的每个 operation 转换为 `FunctionDescriptor` 并注册：

```python
from croupier import CroupierClient, ImportOptions, register_from_openapi

client = CroupierClient(config)
registered = register_from_openapi(
    client,
    spec_data,  # JSON 字符串/bytes 或已解析的 dict
    ImportOptions(resource_prefix="game", tag_prefix="svc-"),
    handler_resolver=lambda function_id: handlers.get(function_id),
)
```

handler 按 operationId（或 path 回退派生的 ID）查找；也可以用 `handlers={"player_ban": handler}` 映射代替 `handler_resolver`。

## ImportOptions

- `resource_prefix`：为非空 `x-resource` 追加前缀，如 `game.player`
- `tag_prefix`：为 tags 追加前缀
- `default_timeout_ms`：为与 Go ImportOptions 对齐而保留；Python descriptor 暂无 timeout 字段
- `continue_on_error`：单个 operation 转换或注册失败时跳过而不是抛错

## 字段转换规则

| OpenAPI 来源                                  | FunctionDescriptor 字段                     |
| --------------------------------------------- | ------------------------------------------- |
| `operationId`（无则 path 转 `a.b.c`）         | `id`                                        |
| `summary`（无则 titleCase(operationId))       | `summary`                                   |
| requestBody `application/json` schema         | `input_schema`                              |
| 200 响应 `application/json` schema            | `output_schema`                             |
| `x-resource` / `x-operation` / `x-permission` | `resource` / `operation` / `permission`     |
| `x-capability`                                | `capability`（受控枚举，非法值丢弃）        |
| `x-execution`                                 | `execution`（`sync` \| `task`，非法值丢弃） |
| `x-approval`                                  | `approval_required` / `approval_policy_key` |
| `x-risk`                                      | `risk`                                      |

`x-approval` 形如 `{"required": true, "policyKey": "gm.player.ban"}`，与 execution 正交。

`x-risk` 输出统一为 canonical 词表 `safe|warning|high|danger`；`low`/`medium`/`moderate` 是废弃别名，导入时分别归一为 `safe`/`warning`。

## 完整示例

参见 `sdks/python/examples/game_demo.py` 与 `sdks/python/tests/test_openapi.py`。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
