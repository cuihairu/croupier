### 1. "获取FAQ列表"

1. route definition

- Url: /api/v1/faqs
- Method: GET
- Request: `FAQListRequest`
- Response: `FAQListResponse`

2. request definition



```golang
type FAQListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Category string `form:"category,optional"`
	Keyword string `form:"keyword,optional"`
	Visible *bool `form:"visible,optional"`
}
```


3. response definition



```golang
type FAQListResponse struct {
	Items []FAQ `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "创建FAQ"

1. route definition

- Url: /api/v1/faqs
- Method: POST
- Request: `FAQCreateRequest`
- Response: `FAQDetailResponse`

2. request definition



```golang
type FAQCreateRequest struct {
	Question string `json:"question"`
	Answer string `json:"answer"`
	Category string `json:"category"`
	Tags []string `json:"tags,optional"`
	Visible bool `json:"visible,optional,default=true"`
	Sort int `json:"sort,optional,default=0"`
}
```


3. response definition



```golang
type FAQDetailResponse struct {
	Id int64 `json:"id"`
	Question string `json:"question"`
	Answer string `json:"answer"`
	Category string `json:"category"`
	Tags []string `json:"tags"`
	Visible bool `json:"visible"`
	Sort int `json:"sort"`
	Views int `json:"views"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type FAQ struct {
	Id int64 `json:"id"`
	Question string `json:"question"`
	Answer string `json:"answer"`
	Category string `json:"category"`
	Tags []string `json:"tags"`
	Visible bool `json:"visible"`
	Sort int `json:"sort"`
	Views int `json:"views"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 3. "更新FAQ"

1. route definition

- Url: /api/v1/faqs/:id
- Method: PUT
- Request: `FAQUpdateRequest`
- Response: `FAQDetailResponse`

2. request definition



```golang
type FAQUpdateRequest struct {
	ID string `path:"id"`
	Question string `json:"question,optional"`
	Answer string `json:"answer,optional"`
	Category string `json:"category,optional"`
	Tags []string `json:"tags,optional"`
	Visible *bool `json:"visible,optional"`
	Sort *int `json:"sort,optional"`
}
```


3. response definition



```golang
type FAQDetailResponse struct {
	Id int64 `json:"id"`
	Question string `json:"question"`
	Answer string `json:"answer"`
	Category string `json:"category"`
	Tags []string `json:"tags"`
	Visible bool `json:"visible"`
	Sort int `json:"sort"`
	Views int `json:"views"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type FAQ struct {
	Id int64 `json:"id"`
	Question string `json:"question"`
	Answer string `json:"answer"`
	Category string `json:"category"`
	Tags []string `json:"tags"`
	Visible bool `json:"visible"`
	Sort int `json:"sort"`
	Views int `json:"views"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 4. "删除FAQ"

1. route definition

- Url: /api/v1/faqs/:id
- Method: DELETE
- Request: `FAQDeleteRequest`
- Response: `-`

2. request definition



```golang
type FAQDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 5. "获取FAQ分类"

1. route definition

- Url: /api/v1/faqs/categories
- Method: GET
- Request: `FAQCategoriesRequest`
- Response: `FAQCategoriesResponse`

2. request definition



```golang
type FAQCategoriesRequest struct {
}
```


3. response definition



```golang
type FAQCategoriesResponse struct {
	Items []FAQCategory `json:"items"`
}
```

