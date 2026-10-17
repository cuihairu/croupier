---
title: 外部服务探测说明
---

# 外部服务探测说明

本仓库的“外部服务”分两类：运行期依赖（身份源、SMTP、webhook、更新源、维护状态源）和跨网络的游戏/基础设施目标（Agent 探针）。现有探活能力以手动测试为主，只有 `core/healthprobe` 配置目标才周期执行；平台没有统一的后台第三方健康巡检、连续失败阈值、告警或降级编排。

## 外部服务清单

### 文件配置清单

`GET /api/v1/site/external-services` 返回只读清单，当前覆盖：

- **herald**（出站告警）：启用、`baseUrl`、token 环境变量三项齐全才 Active。
- **alertInbound.alertmanager / alertInbound.generic**（入站 webhook）：来源配置且 secret 环境变量存在才 Active。
- **serverStatus.\***（出站维护状态源）：启用、provider 已注册、非 noop 且 `baseUrl` 与可选 token 齐全才 Active。

证据：`internal/api/sitesettings/external_services.go:57` 挂载端点，`internal/api/sitesettings/external_services.go:63` 返回四类清单；herald 判定在 `internal/api/sitesettings/external_services.go:75`，入站 webhook 在 `internal/api/sitesettings/external_services.go:98`，状态源在 `internal/api/sitesettings/external_services.go:121`。响应只报环境变量名与是否设置，不回显值（`internal/api/sitesettings/external_services.go:5`）。

该接口只检查配置完整性，不做网络探测。

### 身份源

| 依赖                           | 正常使用时探测        | 手动测试                                                     | 失败处理                     |
| ------------------------------ | --------------------- | ------------------------------------------------------------ | ---------------------------- |
| LDAP                           | 登录时建立会话和 bind | TCP 拨号，5 秒                                               | 配置测试返回失败；无周期巡检 |
| OIDC                           | 授权码交换和 UserInfo | GET `/.well-known/openid-configuration`，5 秒，必须 HTTP 200 | 配置测试返回失败；无周期巡检 |
| GitHub / WeChat / 自定义 OAuth | 授权码交换和用户信息  | 当前未实现配置连通测试                                       | 登录请求失败；无周期巡检     |

LDAP 5 秒拨号在 `internal/api/sitesettings/auth_test_conn.go:69`；OIDC 5 秒发现端点检查在 `internal/api/sitesettings/auth_test_conn.go:90`，状态判定在 `internal/api/sitesettings/auth_test_conn.go:100`。LDAP 会话实现见 `internal/security/identity/ldap.go:43`；自定义 OAuth token/userinfo 流程见 `internal/security/identity/genericoauth.go:98`，其中 `internal/security/identity/genericoauth.go:133` 使用默认 `http.Client{}`，未设显式超时。

**当前未实现**：OAuth/GitHub/WeChat 连通性测试、周期身份源巡检、失败计数和自动禁用。**计划**：统一身份源 Probe 契约，LDAP 做 bind、OIDC 做发现、OAuth 做只读端点或 head 请求；失败进入告警但登录自动降级须显式配置。

### SMTP

两类操作：

- 无副作用探活：TCP 拨号、SMTP greeting、EHLO、NOOP、QUIT，不认证、不发信；默认 10 秒。`internal/security/secguard/retry_probe.go:104` 定义流程，`internal/security/secguard/retry_probe.go:113` 设拨号超时，SSRF 开启时 `internal/security/secguard/retry_probe.go:114` 挂拨号校验。
- 真实测试邮件：按 L3 SMTP 配置发信，拨号默认 10 秒；`internal/api/sitesettings/testemail.go:31` 提供入口，`internal/api/sitesettings/testemail.go:47` 在 host 未配置时拒绝假成功，实际发送在 `internal/platform/approvals/notification.go:432`，10 秒拨号在 `internal/platform/approvals/notification.go:449`。

手动探活入口是 `POST /api/v1/ops/probes/smtp`（`internal/api/ops/probe.go:24`）。SMTP 传输/认证失败只返回当前调用失败；无周期探活、无重试、无降级。

### 通知 webhook 与更新源

手动探针支持钉钉、企业微信、飞书、通用 webhook、更新源和 SMTP 六个渠道。`internal/api/ops/probe.go:14` 定义渠道，HTTP 目标使用 GET，状态码低于 400 即健康；`internal/security/secguard/retry_probe.go:70` 设最多 3 跳重定向并读 1KB 后断开，`internal/security/secguard/retry_probe.go:80` 使用 `secguard` 超时或缺省 10 秒。未配置返回 `configured=false`（`internal/api/ops/probe.go:25`），非法渠道 404。

真实外呼不同：

- **通知 webhook**：POST JSON，HMAC 签名（`internal/platform/approvals/notification.go:731`）；整体预算默认 10 秒或 `net.requestTimeoutMs`，通过 `secguard.DoWithRetry` 按 5xx/网络错误指数退避重试（`internal/platform/approvals/notification.go:756`）。4xx 不重试（`internal/security/secguard/retry_probe.go:43`）。
- **更新源**：GET 幂等可重试；默认 5 秒或 `net.requestTimeoutMs`，网络错误/5xx 重试（`internal/api/ops/systeminfo.go:116`）。未配置不外呼（`internal/api/ops/systeminfo.go:89`），失败只返回失败注记，不阻塞系统运行。

频率与告警：**当前未实现**周期探针、失败阈值、恢复窗口和告警。**计划**：把六渠道登记为外部探测目标，复用 healthprobe 窗口状态机与告警生命周期，提供逐渠道间隔、失败阈值和恢复阈值；webhook 周期探测仍须使用无副作用只读端点，不能 POST 业务通知。

### 出站安全与调用策略

- `sec.*` 解析：`internal/security/secguard/secguard.go:34` 持有允许端口/IP/域名、SSRF 开关和 `net.*` 三键；`internal/security/secguard/secguard.go:48` 从分层设置解析。
- 超时：`net.requestTimeoutMs` 大于 0 时覆盖调用方缺省（`internal/security/secguard/secguard.go:184`）。
- 重试：`net.maxRetries` 范围 0-10（`internal/security/secguard/secguard.go:192`）；`net.retryBackoffMs` 缺省 500 毫秒（`internal/security/secguard/secguard.go:207`）。指数退避在 `internal/security/secguard/retry_probe.go:54`，共享调用方 context 预算。
- 注意：探针和更新源是幂等 GET 可重试；通知 POST 是 at-least-once，对端必须自行去重。

### 维护状态源

状态源用于告警出口的维护窗抑制，不作为周期健康探针。

超时：`serverStatus.providers.<provider>.timeoutMs`，缺省 3 秒（`internal/config/config.go:146`、`internal/platform/serverstatus/rest.go:34`）。缓存只缓存成功结果，TTL 缺省 60 秒，错误不缓存（`internal/platform/serverstatus/cache.go:8`）；配置缺省见 `internal/config/config.go:133`。查询失败、超时或无映射时 gate fail-open：继续告警并标 `source_unknown`，不静默吞告警（`internal/platform/serverstatus/gate.go:49`）；装配配置见 `internal/svc/serverstatus_gate.go:63`。

失败降级就是照常告警加来源未知标注；没有状态源故障自动摘除或维护窗扩展。

## 游戏/基础设施目标探测

`probe.targets[]` 是唯一通用周期探测模型，由 Agent 或 server 执行：

- 间隔缺省 30 秒（`core/healthprobe/targets.go:10`）；连续失败 3 次开窗，连续成功 1 次闭窗（`core/healthprobe/probe.go:95`）。
- HTTP 默认 5 秒，接受 2xx/3xx（`core/healthprobe/healthprobe.go:42`）；TCP 默认 2 秒（`core/healthprobe/targets.go:74`）；DB 默认 5 秒，执行 Ping 加只读查询（`core/healthprobe/db.go:17`）；协议单步 5 秒、建连 2 秒（`core/healthprobe/protocol.go:24`）。
- 异常态采样下限 5 秒（`core/healthprobe/policy.go:29`），策略按目标、时段和窗口状态计算间隔（`core/healthprobe/policy.go:150`、`core/healthprobe/policy.go:192`）。
- 窗口事件即时上行；失败明细每窗口默认上限 10 条（`internal/app/agent/probe_manager.go:42`）。正常统计经 30 秒 `MetricsReport` 批量上报（`internal/app/agent/upstream.go:586`）。

失败动作只有状态机开窗、告警落库和通知出口；探针不重启进程、不切流、不修改配置。该边界由 `internal/app/agent/probe_manager.go:25` 明确约束。

## 缺口与计划

| 缺口               | 当前状态                                        | 计划                                         |
| ------------------ | ----------------------------------------------- | -------------------------------------------- |
| 第三方服务周期探测 | 只有人工 `/ops/probes/:channel`                 | 目标类别化后接入 healthprobe 与告警生命周期  |
| 身份源健康         | 只有 LDAP/OIDC 手动测试                         | 统一 Probe 契约、失败告警与可选降级开关      |
| OAuth 出站超时     | 自定义 OAuth userinfo 无显式超时                | 收敛到受守卫 HTTP 客户端与统一 timeout       |
| 服务目录/状态页    | 文件配置清单不含 Redis、对象存储、herald 连通性 | 扩展清单项并绑定探测目标                     |
| 告警/降级编排      | 第三方失败只返回调用结果                        | 连续失败开窗、告警、自动降级必须审批与可观测 |

## 最小运行建议

1. 生产启用 `sec.ssrfProtection`，显式配置 `sec.allowDomains`/`sec.allowIPs`/`sec.allowPorts`，避免守卫全关直通。
2. 给 `net.requestTimeoutMs` 设业务上限；`net.maxRetries` 保持小值，尤其不能对非幂等 webhook 端点盲目放大。
3. webhook 周期探测接入前，先在接收方提供 `/health` 或专用只读探针，不把业务 POST 端点当健康检查。
4. 状态源部署时确认 `timeoutMs`、`cacheTtlSeconds` 和 token 环境变量；fail-open 语义意味着状态源故障会增加误报而不是抑制告警。
5. 手动探针每次调用都应记录失败原因；当前响应不含历史窗口，不能替代告警。
