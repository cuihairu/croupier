# 支付分析 API

### 1. "获取支付分析"

1. route definition

- Url: /api/v1/analytics/payments
- Method: GET
- Request: `PaymentsRequest`
- Response: `PaymentsResponse`

2. request definition



```go
type PaymentsRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type PaymentsResponse struct {
	Metrics PaymentsMetrics `json:"metrics"`
	Trends interface{} `json:"trends"`
}

type PaymentsMetrics struct {
	Revenue float64 `json:"revenue"`
	Transactions int `json:"transactions"`
	PayingUsers int `json:"payingUsers"`
	ARPU float64 `json:"arpu"`
	ARPPU float64 `json:"arppu"`
	ConversionRate float64 `json:"conversionRate"`
}
```

### 2. "采集支付数据"

1. route definition

- Url: /api/v1/analytics/payments/ingest
- Method: POST
- Request: `PaymentsIngestRequest`
- Response: `PaymentsIngestResponse`

2. request definition



```go
type PaymentsIngestRequest struct {
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Transactions interface{} `json:"transactions"`
	Timestamp string `json:"timestamp,optional"`
}
```


3. response definition



```go
type PaymentsIngestResponse struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
	BatchId string `json:"batchId"`
}
```

### 3. "获取产品趋势"

1. route definition

- Url: /api/v1/analytics/payments/product-trend
- Method: GET
- Request: `PaymentsProductTrendRequest`
- Response: `PaymentsProductTrendResponse`

2. request definition



```go
type PaymentsProductTrendRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
	Limit int `form:"limit,optional,default=10"`
}
```


3. response definition



```go
type PaymentsProductTrendResponse struct {
	Items []ProductTrend `json:"items"`
}
```

### 4. "获取支付摘要"

1. route definition

- Url: /api/v1/analytics/payments/summary
- Method: GET
- Request: `PaymentsSummaryRequest`
- Response: `PaymentsSummaryResponse`

2. request definition



```go
type PaymentsSummaryRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
	GroupBy string `form:"groupBy,optional,default=day"` // day, week, month
}
```


3. response definition



```go
type PaymentsSummaryResponse struct {
	Items []PaymentsSummary `json:"items"`
}
```

### 5. "获取支付交易列表"

1. route definition

- Url: /api/v1/analytics/payments/transactions
- Method: GET
- Request: `PaymentsTransactionsRequest`
- Response: `PaymentsTransactionsResponse`

2. request definition



```go
type PaymentsTransactionsRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Status string `form:"status,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type PaymentsTransactionsResponse struct {
	Items []PaymentTransaction `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

