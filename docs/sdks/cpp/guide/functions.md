# 函数注册

## 基本流程

1. 定义 `FunctionDescriptor`
2. 绑定处理器
3. 在 `Connect()` 之前完成注册

## Descriptor v2 执行契约

每个 descriptor 应声明完整的 v2 执行契约：

- `capability`：`collection_query` / `item_query` / `create` / `update` / `delete` / `action` / `task` / `report`
- `execution`：`sync` / `task`（批量扇出等长耗时操作用 `task`）
- 高危操作（如 `risk = "danger"` / `"high"`）可声明 `approval_required = true` 并附带 `approval_policy_key`

语义分配示例：列表 → `collection_query` + `sync`；单查 → `item_query` + `sync`；创建 → `create` + `sync`；批量 → `action` + `task`。完整示例见 `examples/game_demo.cpp`。

## OpenAPI 导入

已有 OpenAPI 3 文档时可用 `RegisterFromOpenAPI`（`croupier/sdk/openapi_importer.h`）批量导入：

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

转换规则与 Go SDK 一致：`operationId`（缺失时按 path 转 `a.b.c`）作为函数 ID；`summary` 作为名称；requestBody / 200 响应的 `application/json` schema 转为输入输出契约；`x-resource` / `x-operation` / `x-permission` / `x-risk` / `x-capability` / `x-execution` 映射到对应字段；`x-approval`（`required` / `policyKey`）映射审批字段。

## 建议

- 函数 ID 采用 `[namespace.]resource.operation`
- 高风险操作设置 `risk`，必要时声明审批字段
- 处理器保持无状态或显式管理状态
