# Function Registration

## Basic Workflow

1. Define a `FunctionDescriptor`
2. Bind a handler
3. Complete registration before `Connect()`

## Descriptor v2 Execution Contract

Each descriptor should declare the full v2 execution contract:

- `capability`: `collection_query` / `item_query` / `create` / `update` / `delete` / `action` / `task` / `report`
- `execution`: `sync` / `task` (use `task` for long-running operations such as batch fan-out)
- High-risk operations (e.g. `risk = "danger"` / `"high"`) may declare `approval_required = true` along with an `approval_policy_key`

Example semantic assignments: list → `collection_query` + `sync`; single-item query → `item_query` + `sync`; create → `create` + `sync`; batch → `action` + `task`. For a full example, see `sdks/cpp/examples/game_demo.cpp`.

## OpenAPI Import

If you already have an OpenAPI 3 document, you can bulk-import it with `RegisterFromOpenAPI` (`croupier/sdk/openapi_importer.h`):

```cpp
using croupier::sdk::openapi::ImportOptions;
using croupier::sdk::openapi::RegisterFromOpenAPIWithHandlers;

ImportOptions options;
options.resource_prefix = "game";
options.tag_prefix = "svc-";
options.default_timeout_ms = 60000;
options.continue_on_error = true;

std::map<std::string, croupier::sdk::FunctionHandler> handlers;
handlers["player_ban"] = [](const std::string&, const std::string& payload) {
    return std::string("{\"status\":\"ok\"}");
};
RegisterFromOpenAPIWithHandlers(client, spec_json, options, handlers);
```

The conversion rules match the Go SDK: `operationId` (falling back to a path-derived `a.b.c` when missing) becomes the function ID; `summary` becomes the name; the `application/json` schemas of the requestBody / 200 response become the input/output contracts; `x-resource` / `x-operation` / `x-permission` / `x-risk` / `x-capability` / `x-execution` map to their corresponding fields; `x-approval` (`required` / `policyKey`) maps to the approval fields.

## Recommendations

- Use `[namespace.]resource.operation` for function IDs
- Set `risk` on high-risk operations and declare approval fields when necessary
- Keep handlers stateless or manage state explicitly
