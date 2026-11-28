# C-005 鉴权门：JWT localStorage → httpOnly Cookie 迁移评审

日期：2026-11-28。状态：**提案，待负责人拍板**。C-005 交付物（P2 安全项，
2026-11-03 因「须设计确认方向」延期）。**本文档不含任何已实现代码。**
提案把延期中的设计决策收敛为一次签字；以下代码事实均为今日复核——
延期时点的若干前提已变化（C-004 已修复、AUD-044 移除了 `?token=` 查询
通道、refresh-token 机制已落地）。

## 1. 事实依据（代码现状，2026-11-28）

| 事实                                                                                                                           | 来源                                                                               |
| ------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- |
| 登录响应携带 `token`（+ `refreshToken`）；web 只存 access token（`localStorage.setItem('token', …)`）                          | `web/src/pages/User/Login/index.tsx:173`；`web/src` 尚无任何 `refreshToken` 消费方 |
| 中间件只认 `Authorization: Bearer`——`?token=` 查询通道在 C-004 后移除（SSE 改 fetch-stream + header），明确让 token 不再进 URL | `internal/svc/service_context_auth.go:162-173`（AUD-044 注释）                     |
| OAuth 回调 302 跳转前端时 **token 仍在跳转 URL 里**（`q.Set("token", resp.Token)`），四条同构路径（OIDC/GitHub/微信/自定义）   | `internal/api/auth/handler.go:353-372`（`oauthCallback`）                          |
| Logout 调 `token_version` 自增——该账号所有 token 全网 ≤30s 内失效（中间件缓存）                                                | `internal/api/auth/auth_check.go:13-25`                                            |
| MFA / mustChangePassword 是**响应门状态**（401 + 稳定错误码），不是独立中间 token                                              | `internal/api/auth/dto.go:38-44`、`service.go:22-28`                               |
| web 的 401 处理与启动路径在 7 处读/删 `localStorage.token`                                                                     | `web/src/requestErrorConfig.ts:214,232,318,325,338,401`；`web/src/app.tsx:118,150` |
| 同源部署形态：dev 代理 + 生产 nginx L7——浏览器→API 同源，fetch 默认携带 cookie（`credentials: 'same-origin'`）                 | 部署拓扑（deploy-inventory §2）                                                    |
| 非浏览器客户端（Flutter 移动端、SDK、CLI）走同一 JWT 的 Bearer                                                                 | wire 契约                                                                          |

解读：AUD-044 移除 URL 面之后，浏览器链路上 **localStorage 是最后一个
XSS 可读的 token 面**。本迁移是该方向的收尾，不是新开方向。

## 2. D1 —— 传输终态

| 选项                                                          | 形态                                                                                                                                             | 评估                                                                                          |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------- |
| (a) **浏览器走 httpOnly cookie，非浏览器保留 Bearer**（推荐） | 登录/回调在 body token 之外 `Set-Cookie`（httpOnly）；中间件先读 Bearer、cookie 作为浏览器请求的兜底；web 彻底停写停读 localStorage、停发 Bearer | 浏览器完全读不到 token（XSS 拿不到持久凭据）；非浏览器客户端零影响；与 AUD-044 单一面方向一致 |
| (b) web 仅内存持有 token + Bearer                             | token 存 JS 变量，刷新页面经 refresh 重发                                                                                                        | XSS 依然能在每次页面加载时读到；refresh-on-reload 引入不透明的启动依赖；严格劣于 (a)          |
| (c) 服务端 session 存储 + 不透明 cookie                       | 浏览器侧抛弃无状态 JWT                                                                                                                           | 更大的重构（session 存储、失效广播），违背「不新增未经设计的恢复服务」；修本项发现不需要它    |

## 3. D2 —— cookie 属性

推荐两条 cookie：

- `croupier_session` = access JWT：`HttpOnly; SameSite=Strict; Path=/; Max-Age=<access TTL>`，`Secure` **按配置门控**（生产 https 开、`server.mode: dev` 明文 http 关）；
- `croupier_refresh` = refresh JWT：同旗标，另加 `Path=/api/v1/auth/refresh`，使其不随其他任何请求外发。

命名/旗标为单包常量（杜绝按 handler 漂移）；不设 `Domain`（仅宿主域）。
dev 明文 http 下无 `Secure` 属文档化的 dev 专有边界。

## 4. D3 —— CSRF 姿态

| 选项                                      | 评估                                                                                                                                                       |
| ----------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| (a) **仅 SameSite=Strict**（推荐）        | 跨站请求（fetch 或顶层导航 POST）一律不带 cookie → 无需 CSRF token 即封死 CSRF 写路径；同源 fetch 不受影响。最简，无轮换机制负担                           |
| (b) Lax + CSRF double-submit token        | 仅当存在必须携带会话的顶层跨站导航时才需要——本应用不存在（SPA 启动是同源 fetch）                                                                           |
| (c) Strict + 自定义请求头校验（纵深防御） | 可选加固：cookie 兜底另要求一个跨站 fetch 设不了的非 CORS 安全清单头（如 `X-Requested-With: croupier`）。中间件一行；作为廉价的第二把锁推荐随 D3(a) 一并定 |

## 5. D4 —— OAuth 回调 token 交付

把 `?token=` 跳转查询改为在 **302 响应本身** `Set-Cookie`（回调发生在
API 源站，cookie 直接设上）；跳转 URL 不再携带 token。四条同构回调路径
（OIDC/GitHub/微信/自定义）在共享的 `oauthCallback` 出口一次性收口。
这同时消灭了 token 残留在 Referer/浏览器历史的问题——AUD-044 注释里
正是这一点导致查询通道被移除。

## 6. D5 —— refresh token 位置

- (a) **第二条限定路径的 httpOnly cookie**（推荐）：浏览器 refresh 调用
  自动携带；不会随其他请求外发（Path 限定）。
- (b) 仅 body 携带 refresh token：web 一旦接入 refresh（`web/src` 目前
  尚无消费方），就为浏览器重新制造同一个 XSS 可读长效凭据问题——
  现在定 (a) 免去日后返工。

## 7. D6 —— 登出

`Logout` 已有服务端吊销（`token_version` 自增，≤30s 全网生效）。补齐：
登出响应同时清除两条 cookie（`Max-Age=0`、旗标同设置）；web 删掉 7 处
localStorage 触点（`requestErrorConfig.ts`、`app.tsx`、`Login/index.tsx`）。

## 8. D7 —— 上线形态

| 选项                               | 评估                                                                                                                                     |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| (a) **单次发布，两端同上**（推荐） | web 与 server 同仓同栈交付；body token 保留（非浏览器客户端不受影响）；web 同一发布切到仅 cookie。一次迁移，中间件不留事后要拆的双模分支 |
| (b) 两次发布（先双写后裁剪）       | 仅当外部部署会把 web 与 server 版本钉开才有价值——compose 部署两者是同一栈                                                                |

body token 为非浏览器客户端保留在响应里；web 代码只是不再持久化、
不再发送。单元测试断言登录/回调/登出全流程 `localStorage` 零写入
（验收 A2）。

## 9. 验收映射

| #   | 场景                                                                         | 对应        |
| --- | ---------------------------------------------------------------------------- | ----------- |
| A1  | 登录响应按 D2 旗标设置 `croupier_session`（+`croupier_refresh`）             | D1(a)/D2/D5 |
| A2  | 登录 / OAuth 回调 / 登出后 `localStorage` 保持原样（新增单元测试）           | D7          |
| A3  | 无 Bearer 的浏览器请求经 cookie 兜底通过鉴权；非浏览器客户端 Bearer 路径不变 | D1(a)       |
| A4  | 跨站 POST：cookie 不附加（Strict），请求 401                                 | D3(a)       |
| A5  | OAuth 回调 302：URL 无 token；cookie 设在 302 上；前端落地干净               | D4          |
| A6  | 仅凭限定路径的 refresh cookie 即可完成 refresh 调用                          | D5(a)       |
| A7  | 登出：服务端吊销不变（≤30s）+ 两条 cookie 清除                               | D6          |
| A8  | 既有鉴权套件（登录状态机、MFA 门、回调测试）加 cookie 断言后全绿             | 全部        |

## 10. 待负责人决策

- [ ] D1：认可 (a) 浏览器 httpOnly cookie / 非浏览器 Bearer。
- [ ] D2：认可双 cookie 命名/旗标与 dev 明文 http 的 `Secure` 门控。
- [ ] D3：认可仅 Strict 的 CSRF 姿态（可选 + 自定义请求头校验）。
- [ ] D4：认可 OAuth 302 Set-Cookie（四条路径全部移除 URL token）。
- [ ] D5：认可限定路径的 refresh cookie。
- [ ] D6/D7：认可登出清除 + 单次发布上线形态。

## 11. 边界

- 不改 token 格式、TTL、加密；不引入服务端 session 存储——无状态 JWT
  契约原样不动；移动的只是浏览器传输面。
- 移动端/SDK/CLI 继续 Bearer + body token，零变更。
- 延期时点「中间件读 Authorization/?token=」的前提已过时（AUD-044）；
  本评审以当前单头中间件重新落地事实。
