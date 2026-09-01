---
title: 游戏与环境作用域 API 传输设计
icon: plug
order: 3
category:
  - 系统架构
tag:
  - 作用域
  - API
  - Header
---

# 游戏与环境作用域 API 传输设计

> **状态**：Proposal — 设计建议，待评审落地。

## 1. 目标

统一 Croupier 中 `game_id` 与 `env` 在前后端之间的传递方式，解决当前以下问题：

- 前端 scope 来源不统一（`localStorage` 与内存 store 混用）；
- 后端部分接口从 context 读 scope，部分从 query/body 读，部分不读；
- `gameId` 被误用为 `function_id` 的错位问题；
- SSE 无法携带自定义 header，导致 scope 传递链路断裂；
- 部分页面/API 缺少 scope 过滤或过滤不一致。

本设计不改变 `game_id + env` 的业务模型，只规范 **API 传输层** 与 **后端解析层**。

## 2. 设计原则

1. **业务模型不变**：底层仍是 `game_id + env` 两个正交字段，不合并、不打包。
2. **Header 为默认传输方式**：Dashboard / 内部 API 默认通过 `X-Game-ID` / `X-Env` 传递当前作用域。
3. **Query / Body 为显式参数**：SSE、下载、跨 scope 管理操作允许显式传递。
4. **Context 为后端统一入口**：后端所有业务服务统一从 context 取 scope，不直接读 header/query。
5. **兼容渐进**：老接口逐步迁移，新接口直接遵守本规范。

## 3. Scope 模型

```text
scope = game_id + env

game_id : string, 业务游戏标识，如 "tower", "rpg"
env     : string, 逻辑环境，如 "dev", "test", "staging", "prod"
```

- `game_id` 与 `env` 均为 ASCII 字符串；
- 服务端需要校验 `(game_id, env)` 组合已注册；
- 非法或缺失 scope 的请求，返回 400 或 403。

## 4. 传输方式

### 4.1 Header（默认）

用于 Dashboard 发起的普通 API 请求、内部服务调用。

| Header      | 含义         | 示例    |
| ----------- | ------------ | ------- |
| `X-Game-ID` | 当前游戏标识 | `tower` |
| `X-Env`     | 当前逻辑环境 | `prod`  |

规则：

- 前端统一从 `getScope()` 获取，注入到所有 API 请求；
- 后端 middleware 解析 header 并注入 context；
- 如果业务需要显式指定其他 scope（如管理员查询其他游戏），以 query/body 参数为准，header 仅作默认。

### 4.2 Query 参数（SSE / 下载 / 深链接）

用于无法携带自定义 header 的场景：

| 场景              | 参数                     | 示例                                 |
| ----------------- | ------------------------ | ------------------------------------ |
| SSE / EventSource | `?gameId=tower&env=prod` | Analytics 实时数据（若未来恢复 SSE） |
| 文件下载 / 导出   | `?gameId=tower&env=prod` | 审计日志导出                         |
| 页面深链接        | `?gameId=tower&env=prod` | 分享特定 scope 的 Dashboard 页面     |

规则：

- 后端 middleware 支持从 query 回退解析 scope；
- query 参数优先级低于 header，但高于缺省。

### 4.3 Body / 显式参数（跨 scope 操作）

用于管理员跨游戏、跨环境操作：

```json
{
  "gameId": "tower",
  "env": "prod",
  "payload": { ... }
}
```

规则：

- 当请求显式携带 `gameId` / `env` 字段时，以显式值为准；
- 仅管理员或具备跨 scope 权限的账户可使用；
- 记录到审计日志。

### 4.4 三种传输方式对比与选型依据

没有哪一种方式全胜，本设计采用“header 为主、query 回退、body 显式”的原因是三种方式各自扬长避短。

#### 对比表

| 维度                  | Header                                        | Query 参数                         | Body 参数                                       |
| --------------------- | --------------------------------------------- | ---------------------------------- | ----------------------------------------------- |
| **接口签名整洁度**    | 干净，不污染 path/query/body                  | 污染 URL，每个 GET 都要带          | 污染请求体，POST/PUT 要加字段                   |
| **REST 语义**         | 符合“传输上下文”定位（与 Authorization 同类） | 语义上是“过滤条件”，不是作用域     | 语义上是“请求数据”，跨 scope 操作尚属合理       |
| **调试可见性**        | DevTools 需切 Headers 面板                    | URL 一眼可见，最易排查             | 需看 Payload 面板                               |
| **日志检索**          | 需 access log 记录 header 才能查              | URL 自带，最易 grep                | 一般不记 body，排查难                           |
| **深链接 / 分享**     | 无法嵌入 URL                                  | 天然支持                           | 不支持（GET 无 body）                           |
| **SSE / EventSource** | **不支持**，浏览器 API 硬限制                 | 支持                               | 不支持                                          |
| **文件下载**          | 浏览器直接触发时带不了                        | 支持                               | 不支持                                          |
| **缓存键**            | 不参与 URL 缓存键，CDN 可能串 scope           | 天然参与缓存键，隔离正确           | 不涉及                                          |
| **CORS**              | 自定义 header 触发 preflight，需配置放行      | 无额外成本                         | 无额外成本                                      |
| **中间代理**          | 部分网关/CDN 可能丢未知 header                | 稳定                               | 稳定                                            |
| **误用风险**          | 低，前端框架统一注入                          | 高，业务方容易当普通过滤参数随意改 | 中，POST 需手动加                               |
| **GET 请求适配**      | 适合                                          | 适合                               | **不适合**（GET body 语义有争议，很多框架忽略） |
| **一致性维护成本**    | 前端一处拦截器搞定                            | 每个调用点都要拼                   | 每个调用点都要拼                                |

#### 关键差异分析

**Header 的核心优势：职责分离。**

`X-Game-ID` / `X-Env` 与 `Authorization` 是同类东西——“这个请求代表谁在什么上下文下操作”，不是业务参数。放 header 里，接口签名只表达业务：

```http
GET /api/v1/functions?page=1
Authorization: Bearer xxx
X-Game-ID: tower
X-Env: prod
```

对比 query 版本：

```http
GET /api/v1/functions?page=1&gameId=tower&env=prod
```

后者的问题是 scope 和业务过滤参数混在一起：分页、搜索、状态过滤、`gameId`、`env` 全在一个平面上，接口语义变模糊，前端每个调用点都要记得拼。

**Header 的核心劣势：三个传输死角。**

1. **SSE**：`EventSource` 不能自定义 header，浏览器硬限制；
2. **浏览器直接下载**：`window.open(url)` 触发不了带 header 的请求；
3. **深链接**：header 进不了 URL，无法“分享一个 prod 环境的链接给同事”。

这三个场景只能走 query。

**Query 的核心优势：可定位、可分享、可缓存。**

URL 是资源的唯一标识，scope 放 query 里，同一个接口不同环境就是不同 URL，CDN/浏览器缓存天然隔离，不会把 dev 数据缓存到 prod 页面。

**Body 的适用面最窄。**

只适合 POST/PUT，且只在“scope 本身是这次操作的业务对象”时合理，例如：

```json
POST /api/v1/functions/fn-1/copy
{ "targetGameId": "rpg", "targetEnv": "staging" }
```

这里目标 scope 是操作的参数，不是当前上下文，放 body 最贴切。

#### 选型结论

| 场景                              | 选择                | 理由                                                 |
| --------------------------------- | ------------------- | ---------------------------------------------------- |
| Dashboard 常规请求（约 90% 流量） | **Header**          | 职责分离，接口干净，一处拦截器统一注入，维护成本最低 |
| SSE / 下载 / 深链接               | **Query**           | header 物理上传不过去，只能 query                    |
| 跨 scope 管理操作                 | **Body / 显式参数** | scope 是业务参数而非上下文                           |

如果硬要“只用一种”：选 query 会牺牲接口整洁度和职责分离；选 header 会丢掉 SSE/下载/深链接——两头都会踩坑。

## 5. 前端实现

### 5.1 Scope Store

保持 `web/src/stores/scope.ts` 作为唯一状态源：

- `gameId` / `env` 持久化到 `localStorage`；
- 提供 `getScope()` / `setScope()` / `subscribeScope()`；
- `GameSelector` 组件负责选择与持久化。

### 5.2 统一请求工具

新增两个工具函数，避免各 service 自行拼装：

```ts
// web/src/utils/scope.ts
export function getScopeHeaders(): Record<string, string> {
  const { gameId, env } = getScope();
  const headers: Record<string, string> = {};
  if (gameId) headers["X-Game-ID"] = gameId;
  if (env) headers["X-Env"] = env;
  return headers;
}

export function getScopeParams(): Record<string, string> {
  const { gameId, env } = getScope();
  const params: Record<string, string> = {};
  if (gameId) params.gameId = gameId;
  if (env) params.env = env;
  return params;
}
```

### 5.3 请求拦截器

- **普通 HTTP / umi request**：自动 merge `getScopeHeaders()`；
- **SSE / EventSource**：自动把 `getScopeParams()` 拼到 URL；
- **文件下载**：通过 `buildDownloadUrl()` 拼接 scope 参数。

所有前端 service 不再直接读 `localStorage.getItem('game_id')`。

### 5.4 页面深链接

URL query 中的 `gameId` / `env` 可以覆盖当前 scope，并同步到 store：

```text
/dashboard/analytics?gameId=tower&env=prod
```

## 6. 后端实现

### 6.1 Middleware：`GameScopeMiddleware`

统一解析 scope 并注入 context。

> **提案目标 vs 当前实现**：header → query 回退（SSE/下载/深链接场景）
> 是本提案的目标形态；当前实现（`internal/svc/game_middleware.go` 的
> `GameDBMiddleware`）**只读 header，有意拒绝 query/body 回退**，避免
> scope 来源歧义。SSE/下载场景需在 URL 中显式携带 header 等价物时，
> 应走专门的公开端点白名单而非通用回退。

```go
// internal/svc/game_middleware.go
func GameDBMiddleware(svcCtx *ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        gameID := strings.TrimSpace(c.GetHeader(GameDBHeader))
        env := strings.TrimSpace(c.GetHeader(EnvHeader))

        // SSE / 下载 / 深链接 fallback
        if gameID == "" {
            gameID = strings.TrimSpace(c.Query("gameId"))
        }
        if env == "" {
            env = strings.TrimSpace(c.Query("env"))
        }

        scope := GameScope{GameID: gameID, Env: env}
        if gameID != "" {
            c.Set(GameScopeKey, scope)
        }
        ctx := context.WithValue(c.Request.Context(), gameScopeCtxKey{}, scope)

        // 数据库路由
        if svcCtx != nil && svcCtx.Router != nil && gameID != "" {
            if err := validateGameScope(ctx, svcCtx, gameID, env); err != nil {
                c.AbortWithStatusJSON(http.StatusForbidden, ...)
                return
            }
            gameDB, err := svcCtx.Router.GameDB(ctx, gameID, env)
            if err != nil {
                c.AbortWithStatusJSON(http.StatusBadRequest, ...)
                return
            }
            ctx = dbctx.WithDB(ctx, gameDB)
        }

        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

### 6.2 Context 统一入口

所有需要 scope 的 Service 层统一调用：

```go
func requireScope(ctx context.Context) (string, string, error) {
    scope := svc.GameScopeFromContext(ctx)
    gameID := strings.TrimSpace(scope.GameID)
    env := strings.TrimSpace(scope.Env)
    if gameID == "" {
        return "", "", errorx.NewBadRequest("X-Game-ID or gameId query param is required")
    }
    if env == "" {
        return "", "", errorx.NewBadRequest("X-Env or env query param is required")
    }
    return gameID, env, nil
}
```

不再允许 Service 层直接从 `c.GetHeader()` 或 `c.Query()` 取 scope。

### 6.3 模型层

保持现状：

```go
db := dbctx.Resolve(ctx, m.db)
```

`dbctx` 从 context 取当前请求的 per-game DB；没有时回退到 meta DB。

### 6.4 显式参数优先级

当 Handler 显式从 body/query 解析出 `gameId` / `env` 时（如跨 scope 管理），可以显式覆盖 context：

```go
if req.GameId != "" {
    scope.GameID = req.GameId
}
if req.Env != "" {
    scope.Env = req.Env
}
```

但必须重新调用 `validateGameScope` 校验权限。

## 7. API 契约规范

### 7.1 默认 scope 接口

| 方法                | 路径         | Scope 来源     | 说明           |
| ------------------- | ------------ | -------------- | -------------- |
| GET/POST/PUT/DELETE | `/api/v1/**` | Header / Query | Dashboard 默认 |

示例：

```http
GET /api/v1/functions?page=1 HTTP/1.1
Authorization: Bearer <token>
X-Game-ID: tower
X-Env: prod
```

### 7.2 SSE / 下载接口

| 方法 | 路径                         | Scope 来源 |
| ---- | ---------------------------- | ---------- |
| GET  | `/api/v1/analytics/realtime` | Query      |
| GET  | `/api/v1/messages/stream`    | Query      |
| GET  | `/api/v1/audit/export`       | Query      |

示例：

```http
GET /api/v1/analytics/realtime?gameId=tower&env=prod HTTP/1.1
Authorization: Bearer <token>
```

### 7.3 跨 scope 管理接口

| 方法 | 路径                         | Scope 来源 | 权限      |
| ---- | ---------------------------- | ---------- | --------- |
| POST | `/api/v1/functions/:id/copy` | Body       | admin:all |
| GET  | `/api/v1/admin/:id/games`    | Query      | admin:all |

示例：

```json
POST /api/v1/functions/fn-1/copy
{
  "gameId": "rpg",
  "env": "staging",
  "newName": "copy-of-fn-1"
}
```

## 8. 需要修复的问题清单

### 8.1 前端

| 文件                                                                                          | 问题                                                               | 修复方式                               |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ | -------------------------------------- |
| `web/src/services/api/functions.ts`                                                           | 直接读 `localStorage`，把 `gameId` 当 query param 传给 descriptors | 改为走统一拦截器；backend 修复字段含义 |
| `web/src/services/api/audit.ts`                                                               | 组件内直接读 `localStorage`                                        | 改为从 `getScope()` 或统一拦截器       |
| `web/src/services/api/analytics.ts`                                                           | 混用 `getScope()` 和手动 params                                    | 统一走拦截器，SSE 走 query 回退        |
| `web/src/services/api/nodes.ts` / `pages.ts` / `resources.ts` / `versioning.ts` / `alerts.ts` | 不传 scope，仅靠 header                                            | 保持 header 即可，确保拦截器统一加     |
| `web/src/services/core/http.ts`                                                               | `createEventSource` 不带 scope                                     | 自动拼接 `getScopeParams()`            |

### 8.2 后端

| 文件/接口                              | 问题                                                   | 修复方式                                      |
| -------------------------------------- | ------------------------------------------------------ | --------------------------------------------- |
| `internal/api/function/helpers.go:591` | `req.GameId` 被当 `function_id` 传入 `ListDescriptors` | 修复 DTO 字段含义或新增 `functionId` 过滤参数 |
| `internal/api/analytics/handler.go`    | SSE 只读 query                                         | 改为 middleware 统一解析                      |
| `internal/api/function/helpers.go`     | 函数告警/分析/历史/待审批返回空 stub                   | 按业务优先级逐个实现                          |
| `internal/api/audit/service.go`        | 从 OpsStateStore 读，非 DB                             | 评估是否迁移到 per-game DB                    |
| `internal/api/function/helpers.go:478` | 函数实例不按 scope 过滤                                | 如需隔离，从 context 取 scope 过滤            |
| `internal/svc/game_middleware.go`      | 只读 header                                            | 增加 query 回退                               |

## 9. 迁移计划

### Phase 1：统一前端 scope 工具（低风险）

1. 新增 `web/src/utils/scope.ts`；
2. 修改 `web/src/services/core/http.ts` 与 `requestErrorConfig.ts`，统一从 `getScope()` 取；
3. 替换所有 `localStorage.getItem('game_id')` / `localStorage.getItem('env')` 直接调用。

### Phase 2：后端 middleware 增强（低风险）

1. `GameDBMiddleware` 增加 query 回退；
2. 所有 Service 层统一用 `requireScope(ctx)`；
3. 修复 `functions/descriptors` 字段错位。

### Phase 3：补齐过滤与实现（按业务优先级）

1. 函数实例 / 节点 / 资源等接口按 scope 过滤；
2. 实现函数告警、分析、历史等 stub；
3. 评估 Audit 是否迁移到 per-game DB。

## 10. 开放问题

1. **是否需要 `X-Scope` 复合 header**？  
   例如 `X-Scope: tower:prod`，替代 `X-Game-ID` + `X-Env`。目前建议保持两个独立 header，更直观。

2. **Analytics 实时大屏是否恢复 SSE**？  
   当前建议保持轮询；若未来需要秒级实时，恢复 SSE 并按本设计走 query 传 scope。

3. **是否引入 `app_key` 映射层**？  
   当前 Dashboard / 内部 API 场景不需要；若未来出游戏客户端 SDK，可新增 `X-App-Key` 映射到 `(game_id, env)`。

4. **CORS 配置**？  
   需要确认 `Access-Control-Allow-Headers` 包含 `X-Game-ID`、`X-Env`、`Authorization`。

## 11. 参考

- [Flagsmith Authentication Docs](https://docs.flagsmith.com/integrating-with-flagsmith/flagsmith-api-overview/flags-api/authentication)
- [LaunchDarkly Environments Docs](https://launchdarkly.com/docs/home/account/environment)
- [Stripe Context Docs](https://docs.stripe.com/context)
- [The x-tenant-id Pattern](https://akshaynikhare.github.io/posts/x-tenant-id-multi-tenant-api/)
- [Unleash API Tokens](https://docs.getunleash.io/api)
