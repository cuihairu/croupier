# 游戏 API

### 1. "获取游戏列表"

1. route definition

- Url: /api/v1/games
- Method: GET
- Request: `GamesListRequest`
- Response: `GamesListResponse`

2. request definition

```go
type GamesListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
	Status string `form:"status,optional"`
}
```

3. response definition

```go
type GamesListResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Data GamesData `json:"data,omitempty"`
}

type GamesData struct {
	Games []GameInfo `json:"games"`
	Total int `json:"total,optional"`
}
```

### 2. "创建游戏"

1. route definition

- Url: /api/v1/games
- Method: POST
- Request: `GameCreateRequest`
- Response: `GameCreateResponse`

2. request definition

```go
type GameCreateRequest struct {
	Name string `json:"name"`
	Description string `json:"description,optional"`
	Config string `json:"config,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 3. "获取游戏详情"

1. route definition

- Url: /api/v1/games/:id
- Method: GET
- Request: `GameDetailRequest`
- Response: `GameDetailResponse`

2. request definition

```go
type GameDetailRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type GameDetailResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Data GameInfo `json:"data,omitempty"`
}

type GameInfo struct {
	ID uint `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,optional"`
	Description string `json:"description,optional"`
	Enabled bool `json:"enabled"`
	AliasName string `json:"aliasName,optional"`
	Homepage string `json:"homepage,optional"`
	Status string `json:"status"`
	GameType string `json:"gameType,optional"`
	GenreCode string `json:"genreCode,optional"`
	Color string `json:"color,optional"`
	Envs []GameEnvItem `json:"envs,optional"`
	CreatedAt string `json:"createdAt,optional"`
	UpdatedAt string `json:"updatedAt,optional"`
}
```

### 4. "更新游戏"

1. route definition

- Url: /api/v1/games/:id
- Method: PUT
- Request: `GameUpdateRequest`
- Response: `GameUpdateResponse`

2. request definition

```go
type GameUpdateRequest struct {
	ID string `path:"id"`
	Name string `json:"name,optional"`
	Description string `json:"description,optional"`
	Config string `json:"config,optional"`
	Status string `json:"status,optional"`
}
```

3. response definition

```go
type GameUpdateResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Data GameInfo `json:"data,omitempty"`
}

type GameInfo struct {
	ID uint `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,optional"`
	Description string `json:"description,optional"`
	Enabled bool `json:"enabled"`
	AliasName string `json:"aliasName,optional"`
	Homepage string `json:"homepage,optional"`
	Status string `json:"status"`
	GameType string `json:"gameType,optional"`
	GenreCode string `json:"genreCode,optional"`
	Color string `json:"color,optional"`
	Envs []GameEnvItem `json:"envs,optional"`
	CreatedAt string `json:"createdAt,optional"`
	UpdatedAt string `json:"updatedAt,optional"`
}
```

### 5. "删除游戏"

1. route definition

- Url: /api/v1/games/:id
- Method: DELETE
- Request: `GameDeleteRequest`
- Response: `GameDeleteResponse`

2. request definition

```go
type GameDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 6. "获取游戏环境列表"

1. route definition

- Url: /api/v1/games/:id/envs
- Method: GET
- Request: `GameEnvsListRequest`
- Response: `GameEnvsListResponse`

2. request definition

```go
type GameEnvsListRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type GameEnvsListResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Data GameEnvsData `json:"data,omitempty"`
}

type GameEnvsData struct {
	Envs []GameEnvItem `json:"envs"`
}
```

### 7. "添加游戏环境"

1. route definition

- Url: /api/v1/games/:id/envs
- Method: POST
- Request: `GameEnvAddRequest`
- Response: `GameEnvAddResponse`

2. request definition

```go
type GameEnvAddRequest struct {
	ID string `path:"id"`
	Name string `json:"name"`
	Type string `json:"type,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 8. "更新游戏环境"

1. route definition

- Url: /api/v1/games/:id/envs/:envId
- Method: PUT
- Request: `GameEnvUpdateRequest`
- Response: `GameEnvUpdateResponse`

2. request definition

```go
type GameEnvUpdateRequest struct {
	ID string `path:"id"`
	EnvID string `path:"envId"`
	Name string `json:"name,optional"`
	Type string `json:"type,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 9. "删除游戏环境"

1. route definition

- Url: /api/v1/games/:id/envs/:envId
- Method: DELETE
- Request: `GameEnvDeleteRequest`
- Response: `GameEnvDeleteResponse`

2. request definition

```go
type GameEnvDeleteRequest struct {
	ID string `path:"id"`
	EnvID string `path:"envId"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
