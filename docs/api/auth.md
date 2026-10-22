# Authentication API

### 1. "User login"

1. route definition

- Url: /api/v1/auth/login
- Method: POST
- Request: `LoginRequest`
- Response: `LoginResponse`

2. request definition

```go
type LoginRequest struct {
	Username string `json:"username"` // username
	Password string `json:"password"` // password
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

### 2. "User logout"

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

### 3. "Permission check"

1. route definition

- Url: /api/v1/auth/check
- Method: POST
- Auth: Bearer Token
- Request: `CheckRequest`
- Response: `CheckResponse`

2. request definition

```go
type CheckRequest struct {
	Resource string `json:"resource"` // e.g. roles, games, functions
	Action   string `json:"action"`   // e.g. read, write, manage
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

### 4. "Batch permission check"

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

### Notes

- Checks are based on the logged-in user's roles and permissions in the database, not on frontend caches.
- The `gameId` and `env` fields are currently kept for compatibility with the frontend request shape; permission decisions are currently made by resource and action.

---

## External Identity Source Endpoints

The following endpoints become available when `auth.providers` is enabled; when disabled, `providers` returns all `local` and the OIDC endpoints return 400.

### Query Enabled Login Methods

- Url: /api/v1/auth/providers
- Method: GET
- Auth: none

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

### Start an OIDC Login

- Url: /api/v1/auth/oidc/login
- Method: GET
- Auth: none
- Behavior: 302 redirect to the identity source's authorization page; state carries an HMAC signature and a 10-minute validity

### OIDC Callback

- Url: /api/v1/auth/oidc/callback?code=xxx&state=xxx
- Method: GET
- Auth: none
- Response: `LoginResponse` (same as password login); when `loginSuccessUrl` is configured, instead a 302 carrying `?token=xxx` redirects to the frontend

### Self-Service Registration and Email Verification (both anonymous)

- `POST /api/v1/auth/register` — self-service registration (the `auth.providers.register` switch defaults to off)
- `GET /api/v1/auth/verify-email` — email verification callback (the token is the credential)
- `POST /api/v1/auth/resend-verification` — resend the verification email by username+email (anti-enumeration silence at the service layer)

### OAuth Logins (all anonymous, same semantics family)

| Provider      | Start                            | Callback                            |
| ------------- | -------------------------------- | ----------------------------------- |
| OIDC          | `GET /api/v1/auth/oidc/login`    | `GET /api/v1/auth/oidc/callback`    |
| GitHub        | `GET /api/v1/auth/github/login`  | `GET /api/v1/auth/github/callback`  |
| WeChat QR     | `GET /api/v1/auth/wechat/login`  | `GET /api/v1/auth/wechat/callback`  |
| Custom OAuth2 | `GET /api/v1/auth/generic/login` | `GET /api/v1/auth/generic/callback` |

### MFA (TOTP; authentication required)

The server enforces two steps: before confirmation, login returns `403 mfa_required`.

- `GET /api/v1/auth/mfa/status` — whether MFA is enabled
- `POST /api/v1/auth/mfa/setup` — generate a TOTP secret and otpauth URL
- `POST /api/v1/auth/mfa/confirm` — confirm the first code, activating MFA (returns backup recovery codes)
- `POST /api/v1/auth/mfa/disable` — disable MFA (requires a code)

Backup recovery codes live in the `admin_otp_recovery_codes` table (migration 0032).
