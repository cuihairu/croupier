### 1. "获取限流规则列表"

1. route definition

- Url: /api/v1/rate-limits
- Method: GET
- Request: `RateLimitsListRequest`
- Response: `RateLimitsListResponse`

2. request definition



```golang
type RateLimitsListRequest struct {
	Resource string `form:"resource,optional"`
}
```


3. response definition



```golang
type RateLimitsListResponse struct {
	Items []RateLimit `json:"items"`
}
```

### 2. "创建/更新限流规则"

1. route definition

- Url: /api/v1/rate-limits
- Method: PUT
- Request: `RateLimitUpsertRequest`
- Response: `RateLimitResponse`

2. request definition



```golang
type RateLimitUpsertRequest struct {
	Name string `json:"name"`
	Resource string `json:"resource"`
	Limit int `json:"limit"`
	Window int `json:"window"`
	Action string `json:"action"`
	Rules interface{} `json:"rules,optional"`
}
```


3. response definition



```golang
type RateLimitResponse struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Resource string `json:"resource"` // function, api, user
	Limit int `json:"limit"` // 每秒请求数
	Window int `json:"window"` // 时间窗口（秒）
	Action string `json:"action"` // reject, throttle
	Rules interface{} `json:"rules"`
	Status int `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

type RateLimit struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Resource string `json:"resource"` // function, api, user
	Limit int `json:"limit"` // 每秒请求数
	Window int `json:"window"` // 时间窗口（秒）
	Action string `json:"action"` // reject, throttle
	Rules interface{} `json:"rules"`
	Status int `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 3. "获取限流规则"

1. route definition

- Url: /api/v1/rate-limits/:id
- Method: GET
- Request: `RateLimitGetRequest`
- Response: `RateLimitResponse`

2. request definition



```golang
type RateLimitGetRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type RateLimitResponse struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Resource string `json:"resource"` // function, api, user
	Limit int `json:"limit"` // 每秒请求数
	Window int `json:"window"` // 时间窗口（秒）
	Action string `json:"action"` // reject, throttle
	Rules interface{} `json:"rules"`
	Status int `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

type RateLimit struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Resource string `json:"resource"` // function, api, user
	Limit int `json:"limit"` // 每秒请求数
	Window int `json:"window"` // 时间窗口（秒）
	Action string `json:"action"` // reject, throttle
	Rules interface{} `json:"rules"`
	Status int `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 4. "删除限流规则"

1. route definition

- Url: /api/v1/rate-limits/:id
- Method: DELETE
- Request: `RateLimitDeleteRequest`
- Response: `-`

2. request definition



```golang
type RateLimitDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 5. "预览限流规则"

1. route definition

- Url: /api/v1/rate-limits/preview
- Method: POST
- Request: `RateLimitPreviewRequest`
- Response: `RateLimitPreviewResponse`

2. request definition



```golang
type RateLimitPreviewRequest struct {
	Rules interface{} `json:"rules"`
}
```


3. response definition



```golang
type RateLimitPreviewResponse struct {
	Matches interface{} `json:"matches"`
	Impact interface{} `json:"impact"`
}
```

