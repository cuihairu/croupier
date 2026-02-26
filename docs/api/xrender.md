# Formily Schema API（替代 XRender）

> 说明：Dashboard 已迁移至 Formily。原 `/api/v1/xrender/*` 系列接口已移除，
> 请使用 Schema 与 UI 配置接口。

## Schema 管理（Formily）

基础路径：

```
/api/v1/schemas
```

### 1. 获取 Schema 列表
- Url: `/api/v1/schemas/`
- Method: GET
- Request: `SchemasListRequest`
- Response: `SchemasListResponse`

### 2. 创建 Schema
- Url: `/api/v1/schemas/`
- Method: POST
- Request: `SchemaCreateRequest`
- Response: `SchemaCreateResponse`

### 3. 获取 Schema 详情
- Url: `/api/v1/schemas/:id`
- Method: GET
- Request: `SchemaDetailRequest`
- Response: `SchemaDetailResponse`

### 4. 更新 Schema
- Url: `/api/v1/schemas/:id`
- Method: PUT
- Request: `SchemaUpdateRequest`
- Response: `SchemaUpdateResponse`

### 5. 删除 Schema
- Url: `/api/v1/schemas/:id`
- Method: DELETE
- Request: `SchemaDeleteRequest`
- Response: `SchemaDeleteResponse`

### 6. 验证 Schema 数据
- Url: `/api/v1/schemas/:id/validate`
- Method: POST
- Request: `SchemaValidateRequest`
- Response: `SchemaValidateResponse`

### 7. 原始 Schema 验证
- Url: `/api/v1/schemas/raw-validate`
- Method: POST
- Request: `SchemaRawValidateRequest`
- Response: `SchemaRawValidateResponse`

### 8. 获取 UI 配置
- Url: `/api/v1/schemas/:id/ui-config`
- Method: GET
- Request: `SchemaUIConfigRequest`
- Response: `SchemaUIConfigResponse`

### 9. 更新 UI 配置
- Url: `/api/v1/schemas/:id/ui-config`
- Method: PUT
- Request: `SchemaUIConfigUpdateRequest`
- Response: `SchemaUIConfigUpdateResponse`

## 函数 UI 配置（Formily）

用于覆盖函数级别的 UI 配置（与 Schema UI 配置并行）。

### 1. 获取函数 UI 配置
- Url: `/api/v1/functions/:id/ui`
- Method: GET
- Request: `FunctionUIRequest`
- Response: `FunctionUIResponse`

### 2. 更新函数 UI 配置
- Url: `/api/v1/functions/:id/ui`
- Method: PUT
- Request: `FunctionUIUpdateRequest`
- Response: `FunctionUIResponse`

## 旧版 XRender 接口（已移除）

旧版 `/api/v1/xrender/*` 接口已从服务端移除。如需兼容历史数据，请迁移到 Schema 与 UI 配置接口。
