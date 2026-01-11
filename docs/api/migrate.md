# 迁移 API

### 1. "执行向下迁移"

1. route definition

- Url: /api/v1/migrate/down
- Method: POST
- Request: `MigrateDownRequest`
- Response: `MigrateDownResponse`

2. request definition



```golang
type MigrateDownRequest struct {
	Target string `json:"target,optional"` // specific version to migrate to
	Force bool `json:"force,optional"` // force run even if already applied
}
```


3. response definition



```golang
type MigrateDownResponse struct {
	Success bool `json:"success"`
	Message string `json:"message"`
	Results []MigrationResult `json:"results,omitempty"`
}
```

### 2. "获取迁移历史"

1. route definition

- Url: /api/v1/migrate/history
- Method: GET
- Request: `MigrationHistoryRequest`
- Response: `MigrationHistoryResponse`

2. request definition



```golang
type MigrationHistoryRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Status string `form:"status,optional"`
}
```


3. response definition



```golang
type MigrationHistoryResponse struct {
	Items []MigrationResult `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 3. "获取迁移状态"

1. route definition

- Url: /api/v1/migrate/status
- Method: GET
- Request: `MigrationStatusRequest`
- Response: `MigrationStatusResponse`

2. request definition



```golang
type MigrationStatusRequest struct {
}
```


3. response definition



```golang
type MigrationStatusResponse struct {
	LatestVersion string `json:"latestVersion"`
	PendingCount int `json:"pendingCount"`
	HistoryItems []MigrationResult `json:"historyItems,omitempty"`
}
```

### 4. "执行向上迁移"

1. route definition

- Url: /api/v1/migrate/up
- Method: POST
- Request: `MigrateUpRequest`
- Response: `MigrateUpResponse`

2. request definition



```golang
type MigrateUpRequest struct {
	Force bool `json:"force,optional"` // force run even if already applied
	Step int `json:"step,optional"` // specific step to run (-1 for all)
}
```


3. response definition



```golang
type MigrateUpResponse struct {
	Success bool `json:"success"`
	Message string `json:"message"`
	Results []MigrationResult `json:"results,omitempty"`
}
```

