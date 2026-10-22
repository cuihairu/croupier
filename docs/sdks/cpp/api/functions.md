# Function Registration API

Function registration is the most fundamental capability of the C++ SDK.

## Key Concepts

- `FunctionDescriptor`: function metadata
- `FunctionHandler`: business handler
- `RegisterFunction(...)`: registers a function

## OpenAPI Import API

Header: `croupier/sdk/openapi_importer.h` (namespace `croupier::sdk::openapi`).

```cpp
std::vector<std::string> RegisterFromOpenAPI(
    const RegistrationSink& sink, const std::string& spec,
    const ImportOptions& options, const HandlerResolver& resolver);

std::vector<std::string> RegisterFromOpenAPI(
    CroupierClient& client, const std::string& spec,
    const ImportOptions& options, const HandlerResolver& resolver);

std::vector<std::string> RegisterFromOpenAPIWithHandlers(
    CroupierClient& client, const std::string& spec,
    const ImportOptions& options,
    const std::map<std::string, FunctionHandler>& handlers);
```

- `ImportOptions`: `resource_prefix`, `tag_prefix`, `default_timeout_ms` (kept for Go contract alignment; the current C++ provider wire contract has no timeout field, so it is not written into the descriptor), `continue_on_error`
- Returns the list of successfully registered function IDs; throws `std::runtime_error` on an invalid spec or a missing handler (with `continue_on_error = true`, failures are skipped)
- Extension mappings: `x-capability` → `capability`, `x-execution` → `execution`, `x-approval` (`required` / `policyKey`) → `approval_required` / `approval_policy_key`

## Recommendations

- Keep function IDs stable
- Descriptors must include at least `id` and `version`
- Use JSON structures for inputs and outputs, attaching schemas as needed
