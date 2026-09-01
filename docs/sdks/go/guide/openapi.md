# Go SDK：OpenAPI 导入

Go SDK 是六语言 OpenAPI helper 的基准实现。从 OpenAPI（3.x）文档批量
注册函数，Descriptor v2 扩展字段（`x-capability`/`x-execution`/
`x-approval`/`x-risk`）会被解析进契约语义。

## 基本用法

```go
import (
    "github.com/cuihairu/croupier/sdks/go/function"
)

registry := function.NewRegistry(client)

// 按 operationId 生成 handler 桩（未映射的操作返回 not-implemented）
err := registry.RegisterFromOpenAPI(specData, nil, func(operationID string) function.Handler {
    return dispatchByID(operationID) // 你的路由
})

// 或显式 handler 映射
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
    ResourcePrefix:  "game1.",  // 资源前缀
    TagPrefix:       "sdk:",    // tag 前缀
    DefaultTimeoutMs: 5000,     // 未标注 x-execution 超时时的缺省
    ContinueOnError:  true,     // 部分函数失败不中断整批
}
```

## 扩展字段语义

| OpenAPI 扩展   | 契约落点            | 说明                                                |
| -------------- | ------------------- | --------------------------------------------------- |
| `x-capability` | CapabilitySemantics | 资源/操作语义                                       |
| `x-execution`  | ExecutionSemantics  | 超时/重试语义                                       |
| `x-approval`   | ApprovalPolicy      | 审批要求（required、policyKey）                     |
| `x-risk`       | risk                | `safe/warning/high/danger`（`parseRiskLevel` 归一） |

未标注扩展时按默认值归一（GET→查询、其余→操作；
详见 [Descriptor v2 规范](../../../architecture/openapi-sdk-descriptor-v2.md)）。

## 参考实现

- 源码：`sdks/go/function/openapi.go`（`Registry.RegisterFromOpenAPI` /
  `RegisterFromOpenAPIWithHandlers`）、`builder.go`（`ImportOptions`）
- 测试：`sdks/go/function/openapi_test.go`
- 其余五语言等价 helper 见 [SDK 对齐矩阵](/sdks/sdk-parity-matrix)
