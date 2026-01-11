# 管理员 API

### 1. "获取管理员列表"

1. route definition

- Url: /api/v1/admin
- Method: GET
- Request: `AdminsListRequest`
- Response: `AdminsListResponse`

2. request definition



```golang
type AdminsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Search string `form:"search,optional"`
	Role string `form:"role,optional"`
	Status int `form:"status,optional"`
}
```


3. response definition



```golang
type AdminsListResponse struct {
	Items []Admin `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "创建管理员"

1. route definition

- Url: /api/v1/admin
- Method: POST
- Request: `AdminCreateRequest`
- Response: `AdminCreateResponse`

2. request definition



```golang
type AdminCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname,optional"`
	Email string `json:"email,optional"`
	Phone string `json:"phone,optional"`
	Roles []string `json:"roles"`
}
```


3. response definition



```golang
type AdminCreateResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Status int `json:"status"` // 1:active 0:disabled
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Admin struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Status int `json:"status"` // 1:active 0:disabled
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 3. "获取管理员详情"

1. route definition

- Url: /api/v1/admin/:id
- Method: GET
- Request: `AdminDetailRequest`
- Response: `AdminDetailResponse`

2. request definition



```golang
type AdminDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type AdminDetailResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Status int `json:"status"` // 1:active 0:disabled
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Admin struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Status int `json:"status"` // 1:active 0:disabled
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 4. "更新管理员"

1. route definition

- Url: /api/v1/admin/:id
- Method: PUT
- Request: `AdminUpdateRequest`
- Response: `AdminUpdateResponse`

2. request definition



```golang
type AdminUpdateRequest struct {
	ID string `path:"id"`
	Nickname string `json:"nickname,optional"`
	Email string `json:"email,optional"`
	Phone string `json:"phone,optional"`
	Roles []string `json:"roles,optional"`
	Status int `json:"status,optional"`
}
```


3. response definition



```golang
type AdminUpdateResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Status int `json:"status"` // 1:active 0:disabled
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Admin struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Status int `json:"status"` // 1:active 0:disabled
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 5. "删除管理员"

1. route definition

- Url: /api/v1/admin/:id
- Method: DELETE
- Request: `AdminDeleteRequest`
- Response: `-`

2. request definition



```golang
type AdminDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 6. "重置管理员密码"

1. route definition

- Url: /api/v1/admin/:id/password-reset
- Method: POST
- Request: `AdminPasswordResetRequest`
- Response: `-`

2. request definition



```golang
type AdminPasswordResetRequest struct {
	ID string `path:"id"`
	NewPassword string `json:"newPassword"`
}
```


3. response definition


### 7. "获取权限列表"

1. route definition

- Url: /api/v1/permissions
- Method: GET
- Request: `PermissionsListRequest`
- Response: `PermissionsListResponse`

2. request definition



```golang
type PermissionsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Category string `form:"category,optional"`
	Resource string `form:"resource,optional"`
}
```


3. response definition



```golang
type PermissionsListResponse struct {
	Items []Permission `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 8. "获取权限详情"

1. route definition

- Url: /api/v1/permissions/:id
- Method: GET
- Request: `PermissionDetailRequest`
- Response: `PermissionDetailResponse`

2. request definition



```golang
type PermissionDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type PermissionDetailResponse struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Resource string `json:"resource"`
	Action string `json:"action"`
	Category string `json:"category"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Permission struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Resource string `json:"resource"`
	Action string `json:"action"`
	Category string `json:"category"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 9. "获取角色列表"

1. route definition

- Url: /api/v1/roles
- Method: GET
- Request: `RolesListRequest`
- Response: `RolesListResponse`

2. request definition



```golang
type RolesListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Category string `form:"category,optional"`
	Search string `form:"search,optional"`
}
```


3. response definition



```golang
type RolesListResponse struct {
	Items []Role `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 10. "创建角色"

1. route definition

- Url: /api/v1/roles
- Method: POST
- Request: `RoleCreateRequest`
- Response: `RoleCreateResponse`

2. request definition



```golang
type RoleCreateRequest struct {
	Name string `json:"name"`
	Description string `json:"description,optional"`
	Category string `json:"category,optional"`
	Permissions []string `json:"permissions,optional"`
}
```


3. response definition



```golang
type RoleCreateResponse struct {
	Id int64 `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Role struct {
	Id int64 `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 11. "获取角色详情"

1. route definition

- Url: /api/v1/roles/:id
- Method: GET
- Request: `RoleDetailRequest`
- Response: `RoleDetailResponse`

2. request definition



```golang
type RoleDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type RoleDetailResponse struct {
	Id int64 `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Role struct {
	Id int64 `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 12. "更新角色"

1. route definition

- Url: /api/v1/roles/:id
- Method: PUT
- Request: `RoleUpdateRequest`
- Response: `RoleUpdateResponse`

2. request definition



```golang
type RoleUpdateRequest struct {
	ID string `path:"id"`
	Name string `json:"name,optional"`
	Description string `json:"description,optional"`
	Category string `json:"category,optional"`
	Permissions []string `json:"permissions,optional"`
}
```


3. response definition



```golang
type RoleUpdateResponse struct {
	Id int64 `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Role struct {
	Id int64 `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 13. "删除角色"

1. route definition

- Url: /api/v1/roles/:id
- Method: DELETE
- Request: `RoleDeleteRequest`
- Response: `-`

2. request definition



```golang
type RoleDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


