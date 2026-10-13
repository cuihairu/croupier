# 认证 API

### 1. "用户登录"

1. route definition

- Url: /api/v1/auth/login
- Method: POST
- Request: `LoginRequest`
- Response: `LoginResponse`

2. request definition

```go
type LoginRequest struct {
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
}
```

3. response definition

```go
type LoginResponse struct {
	Token string `json:"token"`
	User UserInfo `json:"user"`
}

type UserInfo struct {
	Username string `json:"username"`
	Roles []string `json:"roles"`
	Nickname string `json:"nickname,omitempty"`
}
```

### 2. "用户登出"

1. route definition

- Url: /api/v1/auth/logout
- Method: POST
- Request: `LogoutRequest`
- Response: `LogoutResponse`

2. request definition

```go
type LogoutRequest struct {
}
```

3. response definition

```go
type LogoutResponse struct {
}
```

### 3. "权限校验"

1. route definition

- Url: /api/v1/auth/check
- Method: POST
- Auth: Bearer Token
- Request: `CheckRequest`
- Response: `CheckResponse`

2. request definition

```go
type CheckRequest struct {
	Resource string `json:"resource"` // 例如 roles、games、functions
	Action   string `json:"action"`   // 例如 read、write、manage
	GameID   string `json:"gameId,omitempty"`
	Env      string `json:"env,omitempty"`
}
```

3. response definition

```go
type CheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}
```

### 4. "批量权限校验"

1. route definition

- Url: /api/v1/auth/check/batch
- Method: POST
- Auth: Bearer Token
- Request: `BatchCheckRequest`
- Response: `BatchCheckResponse`

2. request definition

```go
type BatchCheckRequest struct {
	Checks []CheckRequest `json:"checks"`
}
```

3. response definition

```go
type BatchCheckResponse struct {
	Results []CheckResponse `json:"results"`
}
```

### 说明

- 校验基于当前登录用户在数据库中的角色和权限，不依赖前端缓存。
- `gameId` 与 `env` 字段当前保留用于兼容前端请求结构，现阶段权限判断主要按资源和动作执行。

---

## 外部身份源端点

以下端点在启用 `auth.providers` 后可用，未启用时 `providers` 返回全 `local`，OIDC 端点返回 400。

### 查询已启用登录方式

- Url: /api/v1/auth/providers
- Method: GET
- Auth: 无

```json
{
  "local": true,
  "ldap": true,
  "oidc": false,
  "github": false,
  "wechat": false,
  "genericoauth": false,
  "register": true
}
```

### 发起 OIDC 登录

- Url: /api/v1/auth/oidc/login
- Method: GET
- Auth: 无
- 行为: 302 重定向到身份源授权页，state 含 HMAC 签名与 10 分钟有效期

### OIDC 回调

- Url: /api/v1/auth/oidc/callback?code=xxx&state=xxx
- Method: GET
- Auth: 无
- Response: `LoginResponse`（与密码登录一致）；配置 `loginSuccessUrl` 时改为 302 携带 `?token=xxx` 跳转前端

### 自助注册与邮箱验证（均匿名）

- `POST /api/v1/auth/register` — 自助注册（`auth.providers.register` 开关默认关闭）
- `GET /api/v1/auth/verify-email` — 邮箱验证回调（令牌即凭据）
- `POST /api/v1/auth/resend-verification` — 用户名+邮箱重发验证邮件（service 层防枚举静默）

### OAuth 登录（均匿名，语义同族）

| 提供方        | 发起                             | 回调                                |
| ------------- | -------------------------------- | ----------------------------------- |
| OIDC          | `GET /api/v1/auth/oidc/login`    | `GET /api/v1/auth/oidc/callback`    |
| GitHub        | `GET /api/v1/auth/github/login`  | `GET /api/v1/auth/github/callback`  |
| 微信扫码      | `GET /api/v1/auth/wechat/login`  | `GET /api/v1/auth/wechat/callback`  |
| 自定义 OAuth2 | `GET /api/v1/auth/generic/login` | `GET /api/v1/auth/generic/callback` |

### MFA（TOTP；需认证）

服务端强制两步：未确认前登录返回 `403 mfa_required`。

- `GET /api/v1/auth/mfa/status` — MFA 启用状态
- `POST /api/v1/auth/mfa/setup` — 生成 TOTP secret 与 otpauth URL
- `POST /api/v1/auth/mfa/confirm` — 首次验证码确认，激活 MFA（返回备用恢复码）
- `POST /api/v1/auth/mfa/disable` — 关闭 MFA（需验证码）

备用恢复码存 `admin_otp_recovery_codes` 表（迁移 0032）。
