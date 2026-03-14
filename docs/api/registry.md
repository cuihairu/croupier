# 注册表 API

### 1. "获取注册表信息"

1. route definition

- Url: /api/v1/registry
- Method: GET
- Request: `RegistryRequest`
- Response: `RegistryResponse`

2. request definition



```golang
type RegistryRequest struct {
}
```


3. response definition



```golang
type RegistryResponse struct {
	Agents []RegistryAgent `json:"agents"`
	Functions []RegistryFunction `json:"functions"`
	Assignments map[string][]string `json:"assignments"`
	Coverage []RegistryCoverage `json:"coverage"`
}
```

### 2. "获取注册服务列表"

1. route definition

- Url: /api/v1/registry/services
- Method: GET
- Auth: Bearer Token
- Response: `OpsServicesResponse`

2. response definition

```golang
type OpsServicesResponse struct {
	Services []OpsServiceItem `json:"services"`
	Total    int              `json:"total"`
}
```

### 说明

- `/api/v1/registry/services` 是给 Dashboard 使用的兼容快捷路由，当前复用 `ops.Services` 的实现。
- 若需要完整注册表视图，仍优先使用 `/api/v1/registry`。

