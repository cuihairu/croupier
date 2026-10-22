# Go SDK: OpenAPI Import

The Go SDK is the reference implementation of the six-language OpenAPI helper. It registers functions in bulk from an OpenAPI (3.x) document; Descriptor v2 extension fields (`x-capability`/`x-execution`/
`x-approval`/`x-risk`) are parsed into the contract semantics.

## Basic Usage

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

`function.Handler` contract: `func(ctx context.Context, input []byte) ([]byte, error)`.

## ImportOptions

```go
opts := &function.ImportOptions{
    ResourcePrefix:  "game1.",  // resource prefix
    TagPrefix:       "sdk:",    // tag prefix
    DefaultTimeoutMs: 5000,     // default when no x-execution timeout is annotated
    ContinueOnError:  true,     // a partial function failure does not abort the batch
}
```

## Extension Field Semantics

| OpenAPI extension | Contract target     | Description                                                 |
| ----------------- | ------------------- | ----------------------------------------------------------- |
| `x-capability`    | CapabilitySemantics | Resource/operation semantics                                |
| `x-execution`     | ExecutionSemantics  | Timeout/retry semantics                                     |
| `x-approval`      | ApprovalPolicy      | Approval requirements (required, policyKey)                 |
| `x-risk`          | risk                | `safe/warning/high/danger` (normalized by `parseRiskLevel`) |

Without annotated extensions, defaults apply (GET→query, everything else→operation;
see the [Descriptor v2 specification](../../../zh/architecture/openapi-sdk-descriptor-v2.md) for details).

## Reference Implementation

- Source: `sdks/go/function/openapi.go` (`Registry.RegisterFromOpenAPI` /
  `RegisterFromOpenAPIWithHandlers`), `builder.go` (`ImportOptions`)
- Tests: `sdks/go/function/openapi_test.go`
