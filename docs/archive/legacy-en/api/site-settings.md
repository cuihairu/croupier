---
title: 网站配置 API
icon: sliders
order: 21
---

# 网站配置 API（Site Settings）

平台配置中心（L3 数据库覆盖层）的读写入口。分层模型与键族归属见
[平台配置分层设计](../architecture/config-layering.md)；路由实现以
`internal/api/sitesettings/handler.go` 为准。

- **认证**：除 `GET /api/v1/public/site` 匿名外，其余均需 JWT。
- **响应**：成功直返业务 JSON（无 envelope）；错误统一 `{error, message, details}`（见 [REST 契约](./rest.md)）。
- **热生效**：写 L3 即保存即生效（各消费方每次读取 L3 快照；`auth.*` 保存即热重建身份源）。

## 端点

| 方法   | 路径                                   | 说明                                                                    |
| ------ | -------------------------------------- | ----------------------------------------------------------------------- |
| GET    | `/api/v1/public/site`                  | 公开站点快照（品牌/logo/页脚/登录页，匿名可读）                         |
| GET    | `/api/v1/site/features`                | 功能开关合成态（L2∧L3，含 L2 裁剪与 L3 覆盖标注）                       |
| GET    | `/api/v1/site/observability`           | `obs.*` 观测集成 URL + 逐键来源                                         |
| GET    | `/api/v1/site/notification`            | `notification.*` 通知渠道快照（secret 只回「已设置 + 掩码」）           |
| GET    | `/api/v1/site/security`                | `security.*` 账号安全策略生效值（默认全关；`approvalStepUpOtp` 默认开） |
| GET    | `/api/v1/site/outbound`                | `sec.*`/`net.*` 出站安全与调用策略生效值                                |
| GET    | `/api/v1/site/auth`                    | 登录方式生效配置（身份源运行时键）                                      |
| PUT    | `/api/v1/site/:key`                    | 写单键 L3 覆盖（白名单键；非法键/非法值 400）                           |
| DELETE | `/api/v1/site/:key`                    | 清除 L3 覆盖 = 恢复跟随配置文件                                         |
| POST   | `/api/v1/site/notification/test-email` | 向 SMTP 收件地址发测试邮件                                              |
| POST   | `/api/v1/site/auth/test`               | 身份源连通性测试                                                        |

## 写入契约

```bash
# 字符串键：value 必须是 JSON 字符串
curl -X PUT https://<server>:18780/api/v1/site/notification.dingtalkUrl \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"value": "https://oapi.dingtalk.com/robot/send?access_token=xxx"}'

# 布尔键：value 必须是 JSON boolean
curl -X PUT .../site/sec.ssrfProtection -d '{"value": true}'
```

- 键白名单与格式校验在服务端（`validateValue`）：端口清单 1-65535、IP/CIDR、
  域名清单禁协议/路径、枚举键（`smtpEncryption` = none|ssl|starttls 等）非法即 400。
- **secret 掩码回存保护**：读端回显 `****+尾4` 掩码；前端原样回存时服务端检测掩码形态
  并沿用已存真值，不会把掩码写成真值。
- 每次写入与清除均落审计。

## 出站快照结构（`GET /site/outbound`）

```go
type OutboundSnapshot struct {
	AllowPorts       string            `json:"allowPorts"`       // 空串 = 不限
	AllowIPs         string            `json:"allowIPs"`         // 单 IP/CIDR 放行清单
	DomainFilter     string            `json:"domainFilter"`     // 域名后缀白名单
	SSRFProtection   bool              `json:"ssrfProtection"`   // false = 不拦截
	RequestTimeoutMs int               `json:"requestTimeoutMs"` // 0 = 沿用调用方缺省
	MaxRetries       int               `json:"maxRetries"`       // 0 = 不重试（服务端钳 0-10）
	RetryBackoffMs   int               `json:"retryBackoffMs"`   // 0 = 500ms 缺省
	Sources          map[string]string `json:"sources"`          // 逐键来源 default|config|yaml|database
}
```

语义实现见 `internal/security/secguard`（`CheckURL` 静态校验 + 拨号 `Control` 钩子 +
`DoWithRetry`）；边界（覆盖面仅 webhook 通知与检查更新两处外呼）见
[安全配置](../operations/security.md)。

## 通知快照结构（`GET /site/notification`，节选）

```go
type NotificationSnapshot struct {
	EmailEnabled           bool   `json:"emailEnabled"`
	SMTPHost               string `json:"smtpHost"`
	SMTPPort               int    `json:"smtpPort"`
	SMTPUser               string `json:"smtpUser"`
	SMTPFrom               string `json:"smtpFrom"`
	SMTPPasswordSet        bool   `json:"smtpPasswordSet"`
	SMTPPasswordMasked     string `json:"smtpPasswordMasked,omitempty"`
	SMTPEncryption         string `json:"smtpEncryption"`     // "" | none | ssl | starttls
	SMTPAuthType           string `json:"smtpAuthType"`       // "" | plain | login
	SMTPInsecureSkipVerify bool   `json:"smtpInsecureSkipVerify"`
	DingtalkURL            string `json:"dingtalkUrl"`
	DingtalkSecretSet      bool   `json:"dingtalkSecretSet"`
	// ...webhook 渠道同构（Url + SecretSet/SecretMasked）
}
```

渠道语义与事件绑定见 [通知渠道](../operations/notifications.md)。
