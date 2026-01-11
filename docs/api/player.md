# 玩家 API

### 1. "获取玩家列表"

1. route definition

- Url: /api/v1/players
- Method: GET
- Request: `PlayersListRequest`
- Response: `PlayersListResponse`

2. request definition



```golang
type PlayersListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	GameId string `form:"gameId,optional"`
	Search string `form:"search,optional"`
	Status int `form:"status,optional"`
	Level int `form:"level,optional"`
	Vip int `form:"vip,optional"`
}
```


3. response definition



```golang
type PlayersListResponse struct {
	Items []Player `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "创建玩家"

1. route definition

- Url: /api/v1/players
- Method: POST
- Request: `PlayerCreateRequest`
- Response: `PlayerCreateResponse`

2. request definition



```golang
type PlayerCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname,optional"`
	Email string `json:"email,optional"`
	Phone string `json:"phone,optional"`
	GameId string `json:"gameId"`
}
```


3. response definition



```golang
type PlayerCreateResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Player struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 3. "获取玩家详情"

1. route definition

- Url: /api/v1/players/:id
- Method: GET
- Request: `PlayerDetailRequest`
- Response: `PlayerDetailResponse`

2. request definition



```golang
type PlayerDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type PlayerDetailResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Player struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 4. "更新玩家信息"

1. route definition

- Url: /api/v1/players/:id
- Method: PUT
- Request: `PlayerUpdateRequest`
- Response: `PlayerUpdateResponse`

2. request definition



```golang
type PlayerUpdateRequest struct {
	ID string `path:"id"`
	Nickname string `json:"nickname,optional"`
	Email string `json:"email,optional"`
	Phone string `json:"phone,optional"`
	Status int `json:"status,optional"`
	Level int `json:"level,optional"`
	Vip int `json:"vip,optional"`
}
```


3. response definition



```golang
type PlayerUpdateResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Player struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 5. "删除玩家"

1. route definition

- Url: /api/v1/players/:id
- Method: DELETE
- Request: `PlayerDeleteRequest`
- Response: `-`

2. request definition



```golang
type PlayerDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 6. "调整玩家余额"

1. route definition

- Url: /api/v1/players/:id/balance
- Method: POST
- Request: `PlayerBalanceRequest`
- Response: `PlayerBalanceResponse`

2. request definition



```golang
type PlayerBalanceRequest struct {
	ID string `path:"id"`
	Amount int64 `json:"amount"`
	Reason string `json:"reason"`
}
```


3. response definition



```golang
type PlayerBalanceResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Player struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	GameId string `json:"gameId"`
	Status int `json:"status"` // 1:active 0:banned 2:suspended
	Balance int64 `json:"balance"` // 游戏货币
	Level int `json:"level"`
	Vip int `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

