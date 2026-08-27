# 函数注册 API

函数注册是 C++ SDK 最基础的能力。

## 关键概念

- `FunctionDescriptor`: 函数元数据
- `FunctionHandler`: 业务处理器
- `RegisterFunction(...)`: 注册函数

## OpenAPI 导入 API

头文件：`croupier/sdk/openapi_importer.h`（命名空间 `croupier::sdk::openapi`）。

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

- `ImportOptions`：`resource_prefix`、`tag_prefix`、`default_timeout_ms`（Go 契约对齐；当前 C++ provider wire 契约无 timeout 字段，不写入 descriptor）、`continue_on_error`
- 返回成功注册的函数 ID 列表；spec 非法或缺少 handler 时抛 `std::runtime_error`（`continue_on_error = true` 时跳过失败项）
- 扩展映射：`x-capability` → `capability`、`x-execution` → `execution`、`x-approval`（`required` / `policyKey`）→ `approval_required` / `approval_policy_key`

## 建议

- 保持函数 ID 稳定
- 描述符至少包含 `id` 和 `version`
- 对输入输出使用 JSON 结构并按需附带 schema
