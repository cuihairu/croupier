---
title: OpenAPI Import
---

# OpenAPI Import

The Python SDK provides an import helper aligned with the Go SDK's `RegisterFromOpenAPI`. It converts each operation of an OpenAPI 3 spec into a `FunctionDescriptor` and registers it:

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

Handlers are looked up by operationId (or a path-derived fallback ID); you can also pass a `handlers={"player_ban": handler}` mapping instead of `handler_resolver`.

## ImportOptions

- `resource_prefix`: prefixes non-empty `x-resource` values, e.g. `game.player`
- `tag_prefix`: prefixes tags
- `default_timeout_ms`: kept for alignment with the Go ImportOptions; the Python descriptor has no timeout field yet
- `continue_on_error`: skip an operation whose conversion or registration fails instead of raising

## Field Conversion Rules

| OpenAPI source                                 | FunctionDescriptor field                               |
| ---------------------------------------------- | ------------------------------------------------------ |
| `operationId` (path-derived `a.b.c` if absent) | `id`                                                   |
| `summary` (titleCase(operationId) if absent)   | `summary`                                              |
| requestBody `application/json` schema          | `input_schema`                                         |
| 200 response `application/json` schema         | `output_schema`                                        |
| `x-resource` / `x-operation` / `x-permission`  | `resource` / `operation` / `permission`                |
| `x-capability`                                 | `capability` (controlled enum, invalid values dropped) |
| `x-execution`                                  | `execution` (`sync` \| `task`, invalid values dropped) |
| `x-approval`                                   | `approval_required` / `approval_policy_key`            |
| `x-risk`                                       | `risk`                                                 |

`x-approval` looks like `{"required": true, "policyKey": "gm.player.ban"}` and is orthogonal to execution.

`x-risk` output is normalized to the canonical vocabulary `safe|warning|high|danger`; `low`/`medium`/`moderate` are deprecated aliases, normalized to `safe`/`warning` respectively at import time.

## Full Example

See `sdks/python/examples/game_demo.py` and `sdks/python/tests/test_openapi.py`.

For the full contract, see [OpenAPI / SDK Descriptor v2](../../../zh/architecture/openapi-sdk-descriptor-v2.md).
