### 1. "获取功能包列表"

1. route definition

- Url: /api/v1/packs
- Method: GET
- Request: `PacksListRequest`
- Response: `PacksListResponse`

2. request definition



```golang
type PacksListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```


3. response definition



```golang
type PacksListResponse struct {
	Manifest interface{} `json:"manifest,omitempty"`
	Packs interface{} `json:"packs,omitempty"`
	Counts interface{} `json:"counts,omitempty"`
	ETag string `json:"etag,omitempty"`
	ExportAuthRequired bool `json:"exportAuthRequired,omitempty"`
}
```

### 2. "导出功能包"

1. route definition

- Url: /api/v1/packs/export
- Method: GET
- Request: `PacksExportRequest`
- Response: `PacksExportResponse`

2. request definition



```golang
type PacksExportRequest struct {
	ID string `form:"id"`
}
```


3. response definition



```golang
type PacksExportResponse struct {
	Filename string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Content []byte `json:"content,omitempty"`
}
```

### 3. "导入功能包"

1. route definition

- Url: /api/v1/packs/import
- Method: POST
- Request: `PacksImportRequest`
- Response: `PacksImportResponse`

2. request definition



```golang
type PacksImportRequest struct {
	Archive string `json:"archive"`
}
```


3. response definition



```golang
type PacksImportResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "重新加载功能包"

1. route definition

- Url: /api/v1/packs/reload
- Method: POST
- Request: `PacksReloadRequest`
- Response: `PacksReloadResponse`

2. request definition



```golang
type PacksReloadRequest struct {
}
```


3. response definition



```golang
type PacksReloadResponse struct {
	OK bool `json:"ok"`
	UpdatedAt string `json:"updatedAt,optional"`
}
```

