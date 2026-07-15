# 节点 API

### 1. "获取节点列表"

1. route definition

- Url: /api/v1/nodes
- Method: GET
- Request: `NodesListRequest`
- Response: `NodesListResponse`

2. request definition



```go
type NodesListRequest struct {
	Type string `form:"type,optional"`
	Status string `form:"status,optional"`
}
```


3. response definition



```go
type NodesListResponse struct {
	Items []Node `json:"items"`
}

// Node 描述一个接入的 Agent 节点。SDKLanguage / SDKVersion 来自该 Agent 上 provider 的元数据
// （Instance.Metadata → AgentProcess → ProviderSession 端到端透传）。
type Node struct {
	Id          string            `json:"id"`
	Hostname    string            `json:"hostname"`
	Addr        string            `json:"addr"`
	GameId      string            `json:"gameId"`                // 作用域：游戏
	Env         string            `json:"env"`                   // 作用域：环境
	Status      string            `json:"status"`                // active / inactive
	Labels      map[string]string `json:"labels"`
	LastSeen    string            `json:"lastSeen"`              // RFC3339
	SDKLanguage string            `json:"sdkLanguage,omitempty"` // go/java/python/cpp/csharp/node/custom
	SDKVersion  string            `json:"sdkVersion,omitempty"`
}
```

### 2. "排空节点"

1. route definition

- Url: /api/v1/nodes/:id/drain
- Method: POST
- Request: `NodeDrainRequest`
- Response: `-`

2. request definition



```go
type NodeDrainRequest struct {
	ID string `path:"id"`
	Timeout int `json:"timeout,optional"` // 秒
}
```


3. response definition


### 3. "获取节点元数据"

1. route definition

- Url: /api/v1/nodes/:id/meta
- Method: GET
- Request: `NodeMetaRequest`
- Response: `NodeMetaResponse`

2. request definition



```go
type NodeMetaRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```go
type NodeMetaResponse struct {
	Meta interface{} `json:"meta"`
}
```

### 4. "更新节点元数据"

1. route definition

- Url: /api/v1/nodes/:id/meta
- Method: PUT
- Request: `NodeMetaUpdateRequest`
- Response: `NodeMetaResponse`

2. request definition



```go
type NodeMetaUpdateRequest struct {
	ID string `path:"id"`
	Meta interface{} `json:"meta"`
}
```


3. response definition



```go
type NodeMetaResponse struct {
	Meta interface{} `json:"meta"`
}
```

### 5. "重启节点"

1. route definition

- Url: /api/v1/nodes/:id/restart
- Method: POST
- Request: `NodeActionRequest`
- Response: `-`

2. request definition



```go
type NodeActionRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 6. "取消排空节点"

1. route definition

- Url: /api/v1/nodes/:id/undrain
- Method: POST
- Request: `NodeActionRequest`
- Response: `-`

2. request definition



```go
type NodeActionRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 7. "获取节点命令"

1. route definition

- Url: /api/v1/nodes/commands
- Method: GET
- Request: `NodeCommandsRequest`
- Response: `NodeCommandsResponse`

2. request definition



```go
type NodeCommandsRequest struct {
}
```


3. response definition



```go
type NodeCommandsResponse struct {
	Items []NodeCommand `json:"items"`
}
```

