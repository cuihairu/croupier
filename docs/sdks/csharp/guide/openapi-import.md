# OpenAPI Import

`OpenAPIImporter` provides a purely local OpenAPI 3 import aligned with the Go SDK's `RegisterFromOpenAPI`: it parses the JSON spec, converts each operation into a `FunctionDescriptor`, and registers it with its handler on the `CroupierClient` without issuing any network requests.

## Basic Usage

```csharp
using Croupier.Sdk;

var registered = OpenAPIImporter.RegisterFromOpenAPI(
    client,
    specJson,
    new OpenAPIImportOptions
    {
        ResourcePrefix = "demo",
        TagPrefix = "v2:",
        DefaultTimeoutMs = 30000,
        ContinueOnError = true,
    },
    operationId => handlers.TryGetValue(operationId, out var handler) ? handler : null);
```

You can also pass an explicit handler mapping (equivalent to Go's `RegisterFromOpenAPIWithHandlers`):

```csharp
var registered = OpenAPIImporter.RegisterFromOpenAPI(
    client, specJson, options, handlersDictionary);
```

The return value is the list of registered function IDs (in spec order).

## ImportOptions

| Property           | Description                                                                               |
| ------------------ | ----------------------------------------------------------------------------------------- |
| `ResourcePrefix`   | Prefixes `x-resource`, e.g. `"demo"` → `demo.player`                                      |
| `TagPrefix`        | Prefixes every tag                                                                        |
| `DefaultTimeoutMs` | Default timeout; the C# descriptor contract has no timeout field yet, option kept only    |
| `ContinueOnError`  | Skip a function whose handler is missing or whose registration fails, instead of throwing |

Without `ContinueOnError`, a missing handler throws `InvalidOperationException` and an invalid spec throws `ArgumentException`.

## Conversion Rules (Descriptor v2)

| Descriptor field    | Source                                                                                                     |
| ------------------- | ---------------------------------------------------------------------------------------------------------- |
| `Id`                | `operationId`; when missing, generated from the path as `a.b.c` (`/api/players/{id}` → `api.players.{id}`) |
| `Summary`           | `summary`; when missing, title-cased from the operationId                                                  |
| `Description`       | `description`                                                                                              |
| `Tags`              | `tags`                                                                                                     |
| `InputSchema`       | `requestBody.content["application/json"].schema` (shallow JSON Schema)                                     |
| `OutputSchema`      | `responses["200"].content["application/json"].schema`                                                      |
| `Resource`          | `x-resource`                                                                                               |
| `Operation`         | `x-operation`                                                                                              |
| `Capability`        | `x-capability` (`collection_query\|item_query\|create\|update\|delete\|action\|task\|report`)              |
| `Execution`         | `x-execution` (`sync\|task`)                                                                               |
| `Permission`        | `x-permission`                                                                                             |
| `Risk`              | `x-risk` (`safe\|warning\|high\|danger`; legacy aliases like `low/medium/critical` are normalized)         |
| `Enabled`           | `x-enabled` (default `true`)                                                                               |
| `ApprovalRequired`  | `x-approval.required`                                                                                      |
| `ApprovalPolicyKey` | `x-approval.policyKey`                                                                                     |

`x-approval` is orthogonal to `execution`: `execution: "task"` + `approval.required: true` means the task is created only after approval passes; approval is not a third `execution` enum value.

For the full contract, see [OpenAPI / SDK Descriptor v2](../../../zh/architecture/openapi-sdk-descriptor-v2.md).
