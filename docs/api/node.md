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

