# XRender API

### 1. "获取XRender组件"

1. route definition

- Url: /api/v1/xrender/components
- Method: GET
- Request: `XRenderComponentsRequest`
- Response: `XRenderComponentsResponse`

2. request definition



```golang
type XRenderComponentsRequest struct {
}
```


3. response definition



```golang
type XRenderComponentsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "生成XRender模式"

1. route definition

- Url: /api/v1/xrender/generate
- Method: POST
- Request: `XRenderGenerateRequest`
- Response: `XRenderGenerateSchemaResponse`

2. request definition



```golang
type XRenderGenerateRequest struct {
	Schema interface{} `json:"schema"`
}
```


3. response definition



```golang
type XRenderGenerateSchemaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "预览XRender模式"

1. route definition

- Url: /api/v1/xrender/preview
- Method: POST
- Request: `XRenderPreviewRequest`
- Response: `XRenderPreviewSchemaResponse`

2. request definition



```golang
type XRenderPreviewRequest struct {
	Schema interface{} `json:"schema"`
}
```


3. response definition



```golang
type XRenderPreviewSchemaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "获取XRender模板"

1. route definition

- Url: /api/v1/xrender/templates
- Method: GET
- Request: `XRenderTemplatesRequest`
- Response: `XRenderTemplatesResponse`

2. request definition



```golang
type XRenderTemplatesRequest struct {
}
```


3. response definition



```golang
type XRenderTemplatesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 5. "获取UI模式"

1. route definition

- Url: /api/v1/xrender/schema/ui-schema
- Method: GET
- Request: `UISchemaRequest`
- Response: `UISchemaResponse`

2. request definition



```golang
type UISchemaRequest struct {
	Type string `form:"type"`
}
```


3. response definition



```golang
type UISchemaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

