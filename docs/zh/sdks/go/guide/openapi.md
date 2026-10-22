# Go SDK：OpenAPI 导入

Go SDK 是六语言 OpenAPI helper 的参考实现。它从 OpenAPI（3.x）文档批量注册函数；Descriptor v2 扩展字段（`x-capability`/`x-execution`/
`x-approval`/`x-risk`）会被解析为契约语义。

## 基本用法

```go
import (
    "github.com/cuihairu/croupier/sdks/go/function"
)

registry := function.NewRegistry(client)

// Generate handler stubs by operationId (unmapped operations return not-implemented)
err := registry.RegisterFromOpenAPI(specData, nil, func(operationID string) function.Handler {
    return dispatchByID(operationID) // your routing
})

// Or an explicit handler mapping
err = registry.RegisterFromOpenAPIWithHandlers(specData, nil, map[string]function.Handler{
    "player.list": func(ctx context.Context, input []byte) ([]byte, error) {
        return []byte(`{"items":[]}`), nil
    },
})
```

`function.Handler` 契约：`func(ctx context.Context, input []byte) ([]byte, error)`。

## ImportOptions

```go
opts := &function.ImportOptions{
    ResourcePrefix:  "game1.",  // resource prefix
    TagPrefix:       "sdk:",    // tag prefix
    DefaultTimeoutMs: 5000,     // default when no x-execution timeout is annotated
    ContinueOnError:  true,     // a partial function failure does not abort the batch
}
```

## 扩展字段语义

| OpenAPI 扩展   | 契约目标            | 说明                                                     |
| -------------- | ------------------- | -------------------------------------------------------- |
| `x-capability` | CapabilitySemantics | 资源/操作语义                                            |
| `x-execution`  | ExecutionSemantics  | timeout/retry 语义                                       |
| `x-approval`   | ApprovalPolicy      | 审批要求（required、policyKey）                          |
| `x-risk`       | risk                | `safe/warning/high/danger`（由 `parseRiskLevel` 归一化） |

未标注扩展字段时使用默认值（GET→query，其余→operation；
详见 [Descriptor v2 规范](../../../architecture/openapi-sdk-descriptor-v2.md)）。

## 参考实现

- 源码：`sdks/go/function/openapi.go`（`Registry.RegisterFromOpenAPI` /
  `RegisterFromOpenAPIWithHandlers`）、`builder.go`（`ImportOptions`）
- 测试：`sdks/go/function/openapi_test.go`
