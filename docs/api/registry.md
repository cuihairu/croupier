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

