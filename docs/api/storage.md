# 存储 API

### 1. "获取签名URL"

1. route definition

- Url: /api/v1/storage/signed-url
- Method: GET
- Request: `SignedUrlRequest`
- Response: `SignedUrlResponse`

2. request definition



```golang
type SignedUrlRequest struct {
	Path string `form:"path"`
	Expire int `form:"expire,optional"`
}
```


3. response definition



```golang
type SignedUrlResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

