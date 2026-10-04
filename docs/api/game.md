# 游戏 API

权限分两条线，互不包含：游戏本体（新增/编辑/删除游戏）走 `games:write`；游戏环境（增删改环境）走 `games:manage`。读路径 `games:read` / `games:manage` / `games:write` 皆可。游戏管理独立页面在 `/system/games`，环境维护在 `/system/environments`。

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
	AliasName string `json:"aliasName"`
	Icon string `json:"icon"`
	Description string `json:"description"`
	Config string `json:"config"`
}
```

- 权限：`admin:all` 或 `games:write`（`games:manage` 不含游戏创建）。
- `name` 仅限字母、数字和 `_ - @`，重名 409；`aliasName`（显示名称）留空时回填 `name`（alias_name 唯一索引不接受多个空串）。
- `icon` 为图片地址，留空 = 渲染默认骰子占位图。

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
	ID string `uri:"id"`
	Name string `json:"name"`
	AliasName string `json:"aliasName"`
	// Icon 指针语义：字段出现即更新（空串 = 清空，回落默认骰子图标），缺省 = 保持。
	Icon *string `json:"icon"`
	Description string `json:"description"`
	Config string `json:"config"`
	Status string `json:"status"`
}
```

- 权限：`admin:all` 或 `games:write`。
- `name` / `aliasName` / `description` / `config` / `status` 提供非空值才更新，全部为空 400「请提供需要更新的字段」；`icon` 是指针，显式提交即生效（含空串清除）。

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

- 权限：`admin:all` 或 `games:write`（`games:manage` 不能删游戏）。
- **删除门禁**：游戏的 `game_envs` 绑定表或 Envs 元数据任一非空即 409 `conflict`「请先删除该游戏的全部环境，再删除游戏」——先到环境页清空全部环境才能删游戏。

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
