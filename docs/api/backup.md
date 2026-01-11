# 备份 API

### 1. "获取备份列表"

1. route definition

- Url: /api/v1/backups
- Method: GET
- Request: `BackupsListRequest`
- Response: `BackupsListResponse`

2. request definition



```golang
type BackupsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Type string `form:"type,optional"`
}
```


3. response definition



```golang
type BackupsListResponse struct {
	Items []Backup `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "创建备份"

1. route definition

- Url: /api/v1/backups
- Method: POST
- Request: `BackupCreateRequest`
- Response: `BackupDetailResponse`

2. request definition



```golang
type BackupCreateRequest struct {
	Name string `json:"name,optional"`
	Type string `json:"type,optional"` // full, incremental
}
```


3. response definition



```golang
type BackupDetailResponse struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Size int64 `json:"size"`
	Type string `json:"type"`
	Status string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type Backup struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Size int64 `json:"size"`
	Type string `json:"type"`
	Status string `json:"status"`
	CreatedAt string `json:"createdAt"`
}
```

### 3. "删除备份"

1. route definition

- Url: /api/v1/backups/:id
- Method: DELETE
- Request: `BackupDeleteRequest`
- Response: `-`

2. request definition



```golang
type BackupDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 4. "下载备份"

1. route definition

- Url: /api/v1/backups/:id/download
- Method: GET
- Request: `BackupDownloadRequest`
- Response: `-`

2. request definition



```golang
type BackupDownloadRequest struct {
	ID string `path:"id"`
}
```


3. response definition


