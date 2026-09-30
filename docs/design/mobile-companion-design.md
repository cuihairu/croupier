# Flutter 移动伴侣端设计（Mobile Companion）

> 状态：设计稿（本批仅设计文档，不含实现代码）
> 日期：2026-09-29 · 目标仓库路径：`mobile/` · CI：`.github/workflows/ci-mobile.yml`
> 原则：契约零新增——移动端直连既有 `/api/*`；服务端改动收敛为两处：§2.3 设备视图最小补集（只读、字段级、零新业务逻辑）与 §6.4/§6.5 两个事件发布钩子（后端后续项），除此之外**零改动**。

## 1. 背景与定位

值班与外出场景下，运营/管理动作经常卡在「不在电脑前」：

- **两人规则的第二审批人**不在位，高危操作（封号、大额发放、开服开关）整链停滞——这是本设计的 **P0 场景**，优先级最高；
- 告警炸了，需要先在手机上看一眼大盘确认影响面；
- 出问题时需要随手查一条审计记录。

移动端定位为**伴侣端（companion）而非替代端**：审批、看数、查询、简单调用四域做深；一切复杂编辑与治理动作仍归 Web 端。

### 范围（四域）

| 域       | 内容                                                               |
| -------- | ------------------------------------------------------------------ |
| 审批中心 | 待批列表 / 详情 / 批准 / 拒绝 / 两人规则与高风险确认               |
| 监控大盘 | 性能快照 / 设备（Agent）在线状态 / LB 统计（只读）+ 告警列表与静默 |
| 审计查询 | 审计事件检索（筛选 + 分页，只读）                                  |
| 函数调用 | 描述符驱动动态表单的**简单参数**调用（含异步任务跟进）             |

### 明确不做（负面范围）

- **复合编辑器**（PageStudio/Composite Editor 域）——编辑归 Web；
- **pack 管理**（extensions 域：catalog 登记/发布/导入）——治理归 Web；
- 监控与设备**写操作**（`PUT /ops/health`、`PUT /ops/maintenance`、`POST /ops/nodes/:nodeId/drain|undrain|restart` 等）——设备页只读，drain/重启等治理动作归 Web；
- 站内信/公告/工单等其余域（不消费 `/messages`、`/announcements`、`/tickets`）；
- 消息中心与富通知模板（通知只做「有待批 + 深链」级别）。

## 2. 场景与页面清单

### 2.1 审批中心（P0，M1 交付）

**待批列表页**（首页底部 Tab 之一，默认着陆 Tab）

- 数据：`GET /api/v1/approvals/?status=pending&page=&pageSize=`；
- 行信息：`functionId`、发起人 `actor`、`gameId/env` Tag、`mode`（invoke / start_job）、创建时间；
- 行标签：从函数列表构建 `{functionId → risk, approvalRequired}` 描述符映射（对齐 Web Approvals 页 descMap 方案），`risk=high` 行标「高危」、`approvalRequired` 行标「两人复核」；
- 下拉刷新 + 分页加载更多；
- 顶部 scope 切换器（§3.3）：切游戏/环境后列表重查（approvals 是 scoped 路径）。

**审批详情页**

- 数据：`GET /api/v1/approvals/:id`；
- 展示：`payloadPreview`（JSON 折叠视图）、完整元数据（actor/时间/mode/idempotencyKey/targetServiceId）、当前 `state`；
- **两人规则确认**：`actor == 当前用户` 时页面只读并显示「两人规则：申请人不能审批自己的申请」横幅（对齐 Web「我发起的」只读语义），批准/拒绝按钮禁用；
- 动作：
  - **批准**：`POST /api/v1/approvals/:id/approve`，body `{otp?}`；
  - **拒绝**：`POST /api/v1/approvals/:id/reject`，body `{reason}`（reason 必填，移动端表单校验前置）；
- 状态竞态：`409`（已被他人处理）→ 本地化冲突提示 + 自动刷新详情。

**高风险批准确认流（step-up，§4.3）**：`risk=high` 时批准按钮触发三段确认——① 生物识别再认证 → ② 函数 ID 文本输入确认（输入内容须逐字等于 `functionId`，对齐 Web 的 window.prompt 确认语义）→ ③ TOTP 动态码输入（随 `otp` 字段提交）。中低风险：直接批准（服务端若要求 OTP 会以错误透出，表单回退补输）。

### 2.2 监控大盘与告警（M2 交付）

**大盘页**

- 性能快照：`GET /api/v1/ops/performance` → `runtime`（goroutines/heap/GC/uptime）、`host`（CPU/内存/磁盘）、`overload` 三块布尔——各一张卡片，`overload.*=true` 的卡片描红；
- 设备（Agent）在线状态：`GET /api/v1/ops/nodes`（scoped），独立设备页承载——在线/离线徽标、最近心跳、版本、能力标签，详见 §2.3；
- LB 统计：`GET /api/v1/ops/cluster/lb-stats`（按后端返回形态列表格）；
- 全部**只读**，不提供健康开关与维护模式操作。

**告警列表页**

- `GET /api/v1/alerts?page=&pageSize=&level=&status=`（level/status 筛选芯片）；
- 行：`type`、`level`（色阶 Tag）、`message`、`source`、`createdAt`、`status`；
- 静默：`POST /api/v1/alerts/:id/silence`，body `{duration, reason}`（duration 分钟数，reason 必填）；静默规则列表 `GET /api/v1/alerts/silences` 只读展示。

### 2.3 设备 / Agent 在线状态（M2 交付）

**数据面盘点（既有端点，全部现成）**

- `GET /api/v1/ops/nodes`（ops 域、scoped）：agent 会话聚合视图，Web Ops 页同款数据源。单条 Node DTO 字段：`id`（agentId）/ `hostname` / `addr` / `gameId` / `env` / `status` / `labels` / `lastSeen`（最近心跳）/ `sdkLanguage` / `sdkVersion` / `sdkName` / `functions`（已注册函数数）/ `expiresInSec` / `cpu` / `memory` / `disks`（有系统信息上报时）；
- `status` 值域（实现批核实修正）：`active`（本实例持有且活跃）/ `online`（集群对端实例持有）/ `drained`（运维排空）/ `stale`（心跳超时或会话过期）/ `offline`（数据库静态节点未注册）——移动端徽标归并为四档：在线（active/online 绿）/ drained（灰）/ 异常（stale 橙）/ 离线（offline 红）；
- `GET /api/v1/registry`：函数覆盖视角（`agents[]{agentId, gameId, env, addr, functions, healthy, expiresInSec}` + coverage 汇总），设备页不直接消费，排障时可对照；
- 勘误：`GET /api/v1/nodes` 是静态节点注册表（nodes 表：id/name/type/status/ip/port），**不是** agent 会话数据面——v1 稿曾误引为设备数据源，本版修正，移动端不消费该端点。

**设备列表页**

- 数据：`GET /api/v1/ops/nodes`；
- 行：`hostname`（次行 `id`）、`gameId/env` Tag、状态徽标（按上述值域归并四档）、`lastSeen` 相对时间（「3 分钟前」）、版本（`version` 优先展示，回退 `sdkVersion`）、`functions` 数；
- 能力标签：`labels`（通用 k/v，可承载能力/分组标注）渲染为芯片，支持按 label 值客户端侧过滤（agent 规模通常 <100，整表返回后本地过滤足够）；
- 下拉刷新 + 前台 60s 定时刷新（离线推送未落地前的发现手段，见下）；无分页（端点整表返回）。

**设备详情页**

- 基础信息卡：id / hostname / addr / gameId / env / status / lastSeen / expiresInSec / 版本（agent 版本 + sdkLanguage / sdkVersion / sdkName）；
- 资源卡：`cpu` / `memory` / `disks`（无系统信息上报时显示「暂无上报」）；
- 函数卡：`functions` 计数（端点只回数量；函数清单不展开，需要时引导回 Web）；
- **历史在线状态：不承诺**。agent 生命周期钩子（注册/心跳/断开）当前不落任何事件表，无历史数据可查；如后续要做，须后端新增在线事件落库——超出本设计后端零改动边界，列 M4 可选并标注该依赖。

**后端最小补充端点集（只读、零新业务逻辑；两项均已落地 2026-09-29）**

| 补充                            | 形态                                                                                                                                                                             | 动机                                                                       |
| ------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| Node DTO 暴露 `version` 字段    | `internal/api/ops` 的 Node DTO 增加 `version string`；`agent_sessions` 表已有 `Version` 列（agent 二进制版本），listNodes 组装处一行映射即可                                     | 列表/详情页「版本」当前只能显示 SDK 版本；agent 自身版本已有落库数据未出参 |
| `GET /api/v1/ops/nodes/:nodeId` | 单设备详情：复用 listNodes 同源数据（RegistryStore 快照 + agent_sessions + 集群归属判定）按 id 过滤返回单个 Node；现有 `/ops/nodes/:nodeId/meta` 只回 `{labels}`，承载不了详情页 | 详情页现只能整表拉取后客户端过滤；设备多时省流量，也让单设备语义完整       |

两项均为既有数据的出参整理：不加表、不加迁移、不加写路径、不新写业务逻辑。已落地：Node DTO `version` 字段随列表出参（`registry.AgentSession.Version` 一行映射），单设备详情挂 `GET /nodes/:nodeId`（与列表同源同 scope 过滤，未命中 404；路径参数兜底 `c.Param`，因 GET 绑定走 BindQueryCompat 不绑 uri tag）。

**推送联动（设备离线 → ntfy，复用既有告警通道）**

- 通道面已存在：`GET/PUT /api/v1/ops/notifications`（`channels[]{id, type, url, secret}` + `rules[]{event, channels, thresholdDays}`），channel 即 URL 型推送配置（webhook/ntfy 类）——离线告警复用这套通道，移动端只订阅对应 topic；
- 缺口：`agent_offline` 事件的**生成端**未实现（registry 生命周期钩子未接 notifications 派发，当前 rules 只服务备份到期类事件）。该生成端为后端后续项，与 §6.4 审批发布钩子同族跟踪；落地前移动端的离线发现降级为轮询（设备列表页前台 60s 刷新）。

### 2.4 审计查询（M2 交付）

**审计页**（只读检索）

- `GET /api/v1/audit?actor=&kind=&kinds=&env=&ip=&start=&end=&gameId=&page=&pageSize=`；
- 筛选区：actor（文本）、kind（常用闭集芯片 + 自定义）、env、时间区间（date range picker）、ip；
- 注意：audit **不是 scoped 路径**（无 `X-Game-ID` 头语义），游戏过滤走 `gameId` query 参数——与 Web `listAudit` 参数口径一致；
- 行：`createdAt`、`action`、`userId`、`target`、`result`、`gameId/env`；点行展开 `metadata` JSON 折叠视图 + 审计链 `hash/prevHash`（有值才显示）；
- 分页：`page/pageSize`（默认 20，与 Web 同）。

### 2.5 函数调用（M3 交付）

**函数选择页**：`GET /api/v1/functions`（响应形态兜底：裸数组 / `{functions}` / `{items}` 三态，对齐 Web `getFunctionSummary`）→ 搜索 + 描述符映射（risk/approvalRequired 标签同审批中心）。

**调用页（描述符驱动动态表单）**

- `GET /api/v1/functions/:id` 取 descriptor：`inputSchema`（JSON Schema）+ `risk` + `approvalRequired`；
- `inputSchema` → Flutter 表单：映射表见 §2.6；
- 提交：`POST /api/v1/functions/:id/invoke`，body `{payload, route?, targetServiceId?, hashKey?, mode?}`；移动端 `route` 固定省略（服务端默认 lb），`targetServiceId/hashKey` 折叠在「高级」区，`mode=async` 由「异步任务」开关控制；
- `approvalRequired` 的函数：调用后提示「已提交审批，等待第二审批人」（移动端仅发起侧，审批在审批中心完成）；
- 异步任务：响应带 taskId 时进入任务跟进页——`GET /api/v1/tasks/:id` 轮询（5s 间隔，页面可见时）、`POST /api/v1/tasks/:id/cancel`。

### 2.6 JSON Schema → Flutter 控件映射表

收敛原则：**简单参数表单化，复杂结构只读化**。顶层任一参数为复杂形态（嵌套 object / object 数组）时，该参数只读展示，不进入表单编辑区。

| JSON Schema 形态                                         | Flutter 控件                                      | 细节                                                                                                            |
| -------------------------------------------------------- | ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `string`（无 enum）                                      | `TextFormField`                                   | description 作 helper text                                                                                      |
| `string` + `enum`（≤4 项）                               | `SegmentedButton<String>`                         | 横向排布                                                                                                        |
| `string` + `enum`（>4 项）                               | `DropdownButtonFormField`                         | 默认值预选                                                                                                      |
| `number` / `integer`                                     | `TextFormField`（`TextInputType.number`）         | validator `num.tryParse`；integer 追加取整校验；提交时 string→number 归一（对齐 Web `normalizeConfigBySchema`） |
| `boolean`                                                | `SwitchListTile`                                  |                                                                                                                 |
| `array of string/number`                                 | 自绘 TagInput（chips + 底部输入框回车追加）       | 标量数组可编辑                                                                                                  |
| `array of object`                                        | 只读 JSON 视图（pretty print + 复制按钮）         | 收敛：不提供逐项编辑                                                                                            |
| `object` 且全部属性为简单类型（string/number/bool/enum） | `ExpansionTile` 内递归一层渲染简单控件            | 仅递归一层                                                                                                      |
| `object` 含嵌套 object/array                             | 只读 JSON 视图 + 「编辑原始 JSON」折叠兜底        | 收敛核心                                                                                                        |
| 无 schema / schema 无 `properties`                       | 原始 JSON 编辑器（TextField + `jsonDecode` 校验） | 对齐 Web RequestBodyEditor 兜底行为                                                                             |
| `required`                                               | 控件必填 validator                                |                                                                                                                 |
| `default`                                                | 预填初值                                          | 对齐 Web `buildSchemaDefaults`                                                                                  |
| `minLength/maxLength/minimum/maximum/pattern`            | validator 映射                                    | `pattern` 编译失败（坏正则）降级忽略，不阻塞表单                                                                |

提交组装：表单值 + 只读参数原值合并为 `payload`；「编辑原始 JSON」修改过的只读参数以编辑值为准（用户显式改写优先）。

### 2.7 明确页面外的系统界面

- 登录页（双步，§3.2）、scope 选择页（§3.3）、设置页（服务器地址 / ntfy 配置 / 主题 / 语言 / 生物门禁开关）、生物门禁锁屏（§4.1）。

## 3. 契约复用（直连既有 `/api/*`）

### 3.1 移动端消费端点全集

| 域    | 端点                            | 方法 | 用途                                   | scoped（注入 X-Game-ID/X-Env） |
| ----- | ------------------------------- | ---- | -------------------------------------- | ------------------------------ |
| 登录  | `/api/v1/auth/login`            | POST | 双步登录（§3.2）                       | ✗                              |
| scope | `/api/v1/profile/games`         | GET  | 游戏/环境列表（选择器数据源）          | ✗                              |
| scope | `/api/v1/profile/scope`         | PUT  | 持久化所选 scope（服务端兜底口径）     | ✗                              |
| 审批  | `/api/v1/approvals/`            | GET  | 列表（status/page/pageSize）           | ✓                              |
| 审批  | `/api/v1/approvals/:id`         | GET  | 详情                                   | ✓                              |
| 审批  | `/api/v1/approvals/:id/approve` | POST | `{otp?}`                               | ✓                              |
| 审批  | `/api/v1/approvals/:id/reject`  | POST | `{reason}`                             | ✓                              |
| 监控  | `/api/v1/ops/performance`       | GET  | 性能快照                               | ✓                              |
| 监控  | `/api/v1/ops/cluster/lb-stats`  | GET  | LB 统计                                | ✓                              |
| 设备  | `/api/v1/ops/nodes`             | GET  | 设备（Agent）在线状态列表（§2.3）      | ✓                              |
| 设备  | `/api/v1/ops/nodes/:nodeId`     | GET  | 单设备详情（§2.3 最小补集）            | ✓                              |
| 告警  | `/api/v1/alerts`                | GET  | 告警列表（level/status/page/pageSize） | ✗                              |
| 告警  | `/api/v1/alerts/:id/silence`    | POST | `{duration, reason}`                   | ✗                              |
| 告警  | `/api/v1/alerts/silences`       | GET  | 静默规则                               | ✗                              |
| 审计  | `/api/v1/audit`                 | GET  | 检索（gameId 走 query）                | ✗                              |
| 函数  | `/api/v1/functions`             | GET  | 函数列表                               | ✓                              |
| 函数  | `/api/v1/functions/:id`         | GET  | descriptor                             | ✓                              |
| 函数  | `/api/v1/functions/:id/invoke`  | POST | 调用                                   | ✓                              |
| 任务  | `/api/v1/tasks/:id`             | GET  | 异步结果轮询                           | ✓                              |
| 任务  | `/api/v1/tasks/:id/cancel`      | POST | 取消                                   | ✓                              |

scoped 前缀清单从 Web `services/core/scope.ts` 的 `SCOPED_API_PREFIXES` 移植（移动端只需其中 7 个消费域：approvals / functions / function-calls（暂不消费，预留） / ops / tasks）。

### 3.2 认证：JWT + TOTP 双步

1. `POST /api/v1/auth/login`，body `{username, password}`；
2. 响应 `401` 且 `error == "mfa_required"` → 表单补动态码输入，携带 `{username, password, totpCode}` 重试（凭据保留在内存表单 state，不落盘）；
3. 成功响应 `{token, user, lastGameId, lastEnv, mustChangePassword?, mfaSetupRequired?}`：
   - `mustChangePassword=true` → 移动端弹「请回 Web 端完成改密」并终止登录（改密流程移动端不做）；
   - `mfaSetupRequired=true` → 同上引导回 Web 绑定 TOTP；
   - 正常 → `token` 入 secure storage（§4.2），`lastGameId/lastEnv` 预选 scope；
4. 令牌使用：`Authorization: Bearer <token>`（dio 拦截器统一注入）；
5. 过期：任意 API `401` → 清 session 回登录页（登录接口自身的 401 分支除外）。

### 3.3 Scope 头：X-Game-ID / X-Env

- dio 拦截器：URL 命中 scoped 前缀 → 注入 `X-Game-ID` / `X-Env`，并**剥除任何外部传入的同名头**（对齐 Web 拦截器的防注入语义）；
- scope 来源：登录响应 `lastGameId/lastEnv` 预选 → `/api/v1/profile/games` 列表选择 → `PUT /api/v1/profile/scope` 持久化；
- 顶栏 scope 切换器全局生效（Riverpod ScopeController，切后各页自行失效重查）。

### 3.4 统一错误对象的移动端处理

后端错误体 `{error, message, details?}` + 标准 HTTP status（本仓库 API 响应契约）。dio 层归一为 `ApiError{status, code, message, details}`：

| HTTP status    | 移动端行为                                                                                                |
| -------------- | --------------------------------------------------------------------------------------------------------- |
| `400`          | 表单类错误：`details`（对象：字段→文案）映射回 SchemaForm/筛选表单对应字段；无 details 时 toast `message` |
| `401`          | session 失效：清 storage 回登录（`error=mfa_required` 仅登录接口出现，走 §3.2 分支）                      |
| `403`          | `error=mfa_required` → 「请先在 Web 端绑定 TOTP」；其余 → toast `message`（无权限）                       |
| `404`          | 页面级「资源不存在」态                                                                                    |
| `409`          | 竞态文案（如审批已被他人处理）+ 刷新当前数据                                                              |
| `422`          | 同 400 的 details 处理                                                                                    |
| `5xx` / 网络层 | 「服务不可用/网络错误」+ 重试按钮                                                                         |

文案直接用服务端 `message`（服务端已本地化）；客户端逻辑分支一律按 `error` code，不解析 message 文本。SSE / 二进制端点不在移动端消费范围内（`/messages/stream` 不消费，见 §6.3）。

## 4. 安全分层

### 4.1 三层模型

```
┌─ 设备层：local_auth 生物识别（指纹/Face ID/设备凭据）——只解锁本机 App
├─ 会话层：JWT Bearer（服务端身份）+ 登录双步 TOTP
└─ 操作层：高危审批 step-up = 生物再认证 + 函数 ID 文本确认 + TOTP 重输
```

**设备解锁（local_auth）**：

- 保护对象：secure storage 中的 session（token + 用户信息 + scope）；
- 触发时机：冷启动、退后台超过 60s 再回前台（阈值可配置，设置页）；
- 实现：`local_auth` 包（iOS Face ID/Touch ID，Android 生物识别/设备 PIN 图案兜底——无生物硬件设备自动落到设备凭据，两者都没有则跳过门禁并在设置页提示风险）；
- 关闭开关：设置页可关（默认开）；关闭后仅依赖系统级应用锁（若有）。

### 4.2 Session 存储

- `flutter_secure_storage`（iOS Keychain / Android EncryptedSharedPreferences/Keystore）存 `{token, user, scope, serverUrl}`；
- 登出 / 401 时整块清除；
- **不落**：密码、TOTP 种子、生物特征数据（见 §4.4）。

### 4.3 高危审批 step-up 流

`risk=high`（来自函数描述符映射）的批准动作：

1. **生物再认证**：local_auth 弹窗（设备在手证明，防远端诱导）；
2. **函数 ID 文本确认**：对话框输入，逐字匹配 `functionId` 才放行（对齐 Web 确认语义，移动端换 TextField 对话框实现）；
3. **TOTP 重输**：动态码输入，随 `POST /approve` 的 `otp` 字段提交，服务端校验（服务端身份再证明）。

三段全过才发请求；任一取消则中止。中低风险审批：直接批准，服务端要求 OTP 时按错误回退补输。

### 4.4 生物数据边界

| 数据                | 去向                                                                         |
| ------------------- | ---------------------------------------------------------------------------- |
| 指纹/Face ID 模板   | 仅设备安全芯片（iOS Secure Enclave / Android TEE），App 与服务端**均不可得** |
| local_auth 认证结果 | 仅本机布尔值（通过/取消），不产生任何网络请求字段                            |
| 服务端生物字段      | **无**——服务端零新增字段，认证身份仍是 JWT+TOTP                              |

即：生物识别只回答「拿手机的人是不是机主」，不参与、不增强服务端身份判定。

### 4.5 传输与证书

- HTTPS 强制；自签 TLS 场景（本仓库 dev-certs 先例）：首次连接弹指纹确认对话框（显示 SHA-256 指纹，用户确认一次后 pin 到 secure storage），指纹变更时重新确认——防中间人；
- 证书固定（certificate pinning）列 M4 可选增强。

## 5. 技术栈与工程结构

### 5.1 选型

| 层   | 选型                                 | 说明                                                                                                                                      |
| ---- | ------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------- |
| 框架 | Flutter 3.35.1（stable）+ Material 3 | 本机 `~/.local/flutter`，`ANDROID_HOME=~/android-sdk`（已配）；v1 稿写 3.38 系为笔误，按实际钉版修正（2026-09-30，与 ci-mobile.yml 一致） |
| 状态 | Riverpod 2                           | 对齐 cockpit mobile 先例（见 §9 假设 1）；feature 级 NotifierProvider + core 级单例 Provider                                              |
| 网络 | dio                                  | 拦截器链：token 注入 → scope 头 → 错误归一（ApiError）                                                                                    |
| 路由 | go_router                            | 底部 Tab + 详情页 + 深链（`croupier://`）                                                                                                 |
| 存储 | flutter_secure_storage               | §4.2                                                                                                                                      |
| 生物 | local_auth                           | §4.1                                                                                                                                      |
| 推送 | ntfy（自托管，HTTP subscribe）       | §6，不接 FCM/APNs                                                                                                                         |
| 深链 | app_links                            | ntfy 通知点击直达审批详情                                                                                                                 |
| DTO  | 手写 `fromJson` + 归一层             | 对齐 Web services/api 的 normalize 先例（响应三态兜底等），不引 codegen                                                                   |

### 5.2 目录结构（目标态）

```
mobile/
  lib/
    main.dart
    app/                      # router、theme、locale
    core/
      api/                    # dio_client、interceptors/、api_error.dart
      auth/                   # session_controller、biometric_gate、login_service
      scope/                  # scope_controller、scoped_prefixes.dart
      storage/                # session_store
      schema_form/            # §2.6 映射实现：schema_form.dart、field builders、json_view.dart
    features/
      approvals/              # M1：list/ detail/ controllers/ widgets/
      login/                  # M1
      settings/               # M1：服务器地址、ntfy、门禁开关
      monitoring/             # M2：dashboard/ alerts/ devices/
      audit/                  # M2
      invoke/                 # M3：functions/ call/ task/
      push/                   # M3：ntfy 订阅、深链分发
    l10n/                     # M4（M1 起中文硬编码，键位预留 zh/en 双文件结构）
  test/                       # 单测：schema_form 映射矩阵、拦截器、ApiError 归一、各 controller
  android/ ios/
```

## 6. 推送与通知（ntfy 复用）

### 6.1 为什么 ntfy、不接 FCM/APNs

- 自托管数据不出域（GM 后台审计链敏感）；
- 免 FCM 谷歌服务依赖（国内 Android 生态不可达）与 APNs 证书/审核摩擦；
- 分发渠道本就是侧载 APK（内部工具，不上架），不需要厂商推送通道兼容。

### 6.2 Topic 与订阅设计

- topic 规范：`croupier/<env>/approvals/<usernameHash>`（usernameHash = SHA-256(username + serverUrl) 前 16 位——避免 topic 直接暴露用户名被枚举遍历）；
- 订阅鉴权：ntfy 访问 token（ntfy `accessTokens`，设置页粘贴或扫码录入），topic 侧配 read-allowlist；
- 订阅方式：ntfy HTTP stream（`GET {ntfyBase}/{topic}/json` 长连接）+ `app_links` 深链 `croupier://approvals/<id>`；
- 通知内容只含「有新的待批：functionId @ gameId/env」级别，**不含 payload**（最小化泄露面）；
- 运维告警 topic：`croupier/<env>/ops`（设备离线等事件，由服务端 ops notifications 通道发布，见 §6.5）——订阅为设置页可选项，值班角色才建议开。

### 6.3 前台提醒（既有能力复用）

App 前台时：审批列表页 60s 定时轮询 + 下拉刷新（不依赖 ntfy）。既有 `GET /api/v1/messages/stream` SSE 是站内信域（移动端不消费，见 §1 负面范围），审批事件流服务端暂无独立 SSE 端点——不为此新增服务端端点，前台靠轮询。

### 6.4 服务端发布钩子（后端后续项，明确不在本设计范围）

审批创建 → ntfy publish 的服务端钩子需要后端改动（settings 键 `notifications.ntfy.baseurl` / `notifications.ntfy.topicPrefix` + approval 创建路径挂钩）。**本设计只定义客户端订阅与深链**；钩子落地前 ntfy 推送整体降级为轮询（功能不缺失，及时性降）。列为依赖项跟踪，不阻塞 M1-M2。

### 6.5 设备离线推送（复用告警通道，后端后续项）

设备（agent）离线事件复用 ops notifications 既有通道面（§2.3 推送联动）：channel URL 指向 ntfy topic `croupier/<env>/ops`，`agent_offline` 规则把事件路由到该通道。移动端设置页追加订阅该 topic，收到离线通知深链直达设备详情页。`agent_offline` 事件生成端未落地前此推送不可用（设备页降级轮询发现），不阻塞 M1-M2。

## 7. CI 接线

新增 `.github/workflows/ci-mobile.yml`（对齐既有 `ci-dashboard.yml` 风格：paths 触发 + ubuntu-latest）：

```yaml
name: CI - Mobile
on:
  pull_request:
    branches: [main]
    paths: ["mobile/**", ".github/workflows/ci-mobile.yml"]
  push:
    branches: [main]
    paths: ["mobile/**", ".github/workflows/ci-mobile.yml"]
  workflow_dispatch:

jobs:
  flutter:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: mobile
    steps:
      - uses: actions/checkout@v7
      - uses: subosito/flutter-action@v2
        with:
          channel: stable
          flutter-version: 3.35.1 # 与本机工具链对齐（v1 稿 3.38 系为笔误）
      - run: flutter pub get
      - run: dart format --set-exit-if-changed --output=none .
      - run: flutter analyze # 0 warning 纪律（对齐 web tsc 0 错）
      - run: flutter test
```

- `flutter analyze` / `dart format` 零容忍，对齐仓库「tsc 0 错」门禁纪律；
- Android 构建烟测（`flutter build apk --debug`）M2 起追加（M1 无 android/ 完整工程时可跳过）；
- 本机工具链（`~/.local/flutter` 3.35.1 + `~/android-sdk`，`ANDROID_HOME` 已配）用于本地真机调试与 APK 侧载产物，CI 不依赖本机。

## 8. 分批实施计划

| 批次                               | 内容                                                                                                                                                                                                       | 验收标准                                                                                                                                                      |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **M1（P0）骨架 + 登录 + 审批中心** | 工程脚手架、dio 拦截器链（token/scope/ApiError）、登录双步（mfa_required 分支）、scope 选择、审批列表/详情/批准/拒绝（含 409 竞态）、高危 step-up 三段确认、local_auth 门禁、secure storage、ci-mobile.yml | 真机对自签 server 完成：登录（TOTP 双步）→ 切 scope → 批准一条两人规则高危审批（生物+函数ID+OTP 全链）；`flutter analyze`/`test` 全绿；CI - Mobile 工作流上线 |
| **M2 监控 + 审计**                 | 性能/设备（Agent）在线状态/LB 只读大盘、设备列表+详情（§2.3，含后端最小补集两项落地）、告警列表+静默、审计筛选查询、APK debug 构建烟测入 CI                                                                | 三域页面真机可用；监控写操作确认不存在（负面范围回归）                                                                                                        |
| **M3 函数调用 + 推送**             | descriptor 拉取、SchemaForm 映射表全实现（§2.6）、invoke 同步/异步（任务轮询+取消）、ntfy 订阅+深链（服务端钩子未落地则轮询降级）                                                                          | 全映射矩阵单测覆盖（每种 schema 形态一例）；复杂 object 只读边界有测试锁定；ntfy 深链直达审批详情                                                             |
| **M4 打磨**                        | i18n（zh-CN/en-US 双语，键位结构对齐 web locales 习惯）、平板/横屏布局、证书固定（可选）、离线最近数据缓存、APK 产物流水线（workflow_dispatch 手动触发）                                                   | 双语切换；横屏可用；APK 产物可下载                                                                                                                            |

批次节奏：M1 单独一批先上（P0 场景尽早闭环），M2/M3 可并行，M4 收尾。

### 8.1 M1 落地事实（2026-09-30）

M1 已分五切片全部合入 main（de4dfd3 工程脚手架 + core 层 → da3816f 登录双步 → 8609a8e scope 选择器 → dd062e3 审批中心 + Tab 主壳 → 20feae7 设置页）：

- **core 层**：`mobile/lib/core/`——ApiClient 拦截器链（token 注入 → scope 头成对校验且先剥外部同名头 → 401 清会话回登录）、ApiError 响应形态归一（非契约体退化 `http_<status>`，防 SPA 兜底当空列表）、SessionData secure storage（单 key JSON 整块）、LoginService 双步；
- **登录双步**：401+`mfa_required` → TOTP 输入框按需出现（错误文案透传服务端 message）；`mustChangePassword` / `mfaSetupRequired` → 弹窗引导回 Web 且**不写会话**（防冷启动带残留 token 直进 App）；
- **scope 选择器**：`GET /api/v1/profile/games` 拉取 + `PUT /api/v1/profile/scope`（先 PUT 成功再改本地，失败不动现状）；顶栏切换器 + 底部 Sheet 两段（game→env）选择，预选命中 lastGameId/lastEnv 否则落第一个可用项；审批列表监听 scope 变更自动刷新；
- **审批中心**：列表（pending/approved/rejected 三段筛选、下拉刷新、加载更多，pageSize 20）+ 详情（payloadPreview 折叠、reject reason 必填、409 竞态→「该审批已被处理」并自动刷新、两人规则发起人=自己时按钮隐藏 + 只读横幅）+ 批准/拒绝；
- **Tab 主壳**：审批（默认着陆）/监控占位（M2）/设置；AppBar 挂 scope 切换器与登出；
- **设置页**：会话信息（用户/scope/服务器地址）+ 更换地址（清会话回登录 + 新地址预填登录表单）+ ntfy / 生物门禁禁用占位（M3/M2 边界明示）；
- **质量门禁**：`flutter analyze` 0 告警、`dart format` 零 diff、`flutter test` 72 用例全绿；`ci-mobile.yml` 上线（钉版 3.35.1）。

**M1 验收标准修订（诚实边界）**：原验收「真机完成生物+函数 ID+OTP 全链」未全额达成——批准实际为**单段确认**：① 生物识别（local_auth）按批次表归 M2 引入；② `POST /api/v1/approvals/{id}/approve` 后端只读 URI 不读请求体，step-up TOTP 的 `otp` 无落点（见 §9 边界，待后端立项）。其余验收项（analyze/test 全绿、CI 工作流上线）达成；「真机全链」验收顺延至 step-up 补齐后，M1 以 CI 门禁 + 模拟器测试替代。

## 9. 假设与已知边界汇总

**假设（缺信息自行判定，实现批如与事实不符在此修订）：**

1. **cockpit mobile 先例不在本仓库**——按用户口径对齐其技术选型（Flutter + Riverpod + dio、feature-first 目录、拦截器分层）；具体目录命名若与 cockpit 实际不一致，以 cockpit 现行规范为准回改。
2. **ntfy 实例**：部署机（192.168.5.5）docker ps 未见 ntfy 容器（2026-09-29 核查）——假设自备/后续部署自托管 ntfy；其 BASE URL 为 App 设置项，不硬编码。
3. **服务端改动收敛**：仅两处——§2.3 后端最小补集（Node DTO `version` 字段 + 单设备详情端点，均为只读出参整理）与 §6.4/§6.5 两个事件发布钩子（后端后续项）；其余功能全部直连既有端点。若实现中发现新缺口，回本设计补「契约变更」节而非直接加端点。
4. docs/design/ 目录现有文档（menu-management.md）未挂 VitePress 侧边栏——本文档沿用该约定，不加侧边栏项（URL 直达）。

**已知边界（诚实清单）：**

- **M1 实测（2026-09-30）**：`POST /api/v1/approvals/{id}/approve` 后端只 `ShouldBindUri` 不读请求体——step-up TOTP 的 `otp` 无落点，高危批准当前降级为单段确认（设计稿 §2.1 三段确认中的 OTP 段待后端补 otp 落点后接线，列后端立项项）；
- **M1 实测（2026-09-30）**：`GET /api/v1/functions/descriptors` 返回项无 `risk` / `approvalRequired` 字段（Web 端 descMap 同样拿不到）——审批行高危/两人复核标签按向前兼容解析实现，后端字段补上即自动生效；标签缺失不阻塞列表（修正上条设计期假设：标签来源是 descriptors 端点而非 `GET /api/v1/functions` 列表）；
- 生物识别（local_auth）与 ntfy 推送分别为 M2 / M3 交付；设置页对应开关以禁用占位明示，不冒充可用；
- 高危判定的描述符映射依赖 `GET /api/v1/functions` 列表数据：函数列表拉取失败时审批行降级为「风险未知」；设计期意图为「标签缺失仍强制走全三段 step-up（宁可多确认）」——M1 实测后该意图受上面 otp 落点边界约束，当前为单段确认（见上方 M1 实测条目）；
- 复杂 object 只读（§2.6）：移动端不提供嵌套结构逐字段编辑，需要编辑时引导回 Web；
- 审计只读无导出；监控只读无开关操作；
- ntfy 推送依赖服务端发布钩子（未落地前仅轮询，及时性降为分钟级）；
- 设备历史在线状态无落库数据（agent 生命周期钩子不产事件），详情页不承诺历史在线曲线；补齐须后端新增事件落库（M4 可选，标注依赖）；
- 设备离线推送依赖 `agent_offline` 事件生成端（未落地前设备页靠前台轮询发现离线）；
- `mustChangePassword` / `mfaSetupRequired` / 邮箱未验证（403 email_not_verified）账号：移动端一律引导回 Web 完成对应流程，App 内不做改密/绑定/验证；
- 生物门禁可被用户在设置中关闭（默认开），关闭后保护仅剩系统级锁；
- 自签 TLS 指纹确认是「用户确认一次」级别，certificate pinning 列 M4 可选；
- 移动端不做消息中心（`/messages` 域整体不消费）。
