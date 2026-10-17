---
title: 服务器状态对接——可插拔维护状态 Provider（设计简档，已批开工）
---

# 服务器状态对接（维护窗口 gate，不绑死 atlas）

## 状态

- 状态: **已批开工（2026-10-10 需求令落档）**——文档先落盘，落盘即开工。
- 需求令：agent 监控到服务异常时，查目标服务器的维护状态——**在维护=静默不报，不在维护=报警通知**；状态源抽成可插拔 Provider（atlas 只是第一个实现，用户自有项目），未来自建 Zabbix/Prometheus/云监控按同接口接入。
- 判定裁决（用户令原文）：**agent 探活异常 ∧ 状态源不在维护窗 → 报警；状态源不可达=按未知处理，照报警并标 `source_unknown`，不许静默**。
- 机制归位：本设计与[插件机制](plugin-mechanism.md)同一套 Provider 模式——**状态源 Provider、告警出口 Provider、供应商 Provider 三者同构**（决策①/⑦），不搞三套模式；**默认 no-op 第一原则**全适用（零配置=零维护信息，零外发）。

## 1. 定位与判定规则

### 1.1 场景链路

```
agent 探活异常（probe.unavailable / supervisor.breaker_tripped 等事件）
  → 维护状态 gate：StatusProvider.GetServerStatus(目标服务器)
      ├─ InMaintenance = true   → 抑制通知投递（全出口链，含站内）——记录抑制日志+计数，不静默丢失
      ├─ InMaintenance = false  → 照常走出口链投递（站内默认必有 → 外部 Provider 链，见插件机制 §5.1）
      └─ 查询失败/超时/未配置映射 → 按「未知」处理：照常报警 + 事件 metadata 标 source_unknown=true
```

- **gate 抑制面 = 通知投递**（出口链整体，站内也静默——维护窗口内的探活失败是预期内噪音，用户口径「在维护=静默不报」）；告警/事件**落库不变**（告警页/supervisor 事件环仍可见，排查需要），只不外发不打扰。
- **fail-open 到报警**：状态源不可达绝不等于「不在维护」，必须照报并标注——宁误报不漏报（用户令原文「不许静默」）。

### 1.2 gate 范围

- v1 gate 挂在**出口链入口**（Outlet Manager 投递前），作用于带服务器定位的事件（probe 窗口 / supervisor 熔断重启 / capture 告警带 agentId 的）。
- 事件无法解析出服务器定位（无 agentId/host 映射）→ **不过 gate 照常投递**（gate 只对能定位服务器的事件生效，宁多报不漏报）。

## 2. Provider 接口契约（与出口/供应商 Provider 同构）

```go
// ServerStatusProvider 服务器维护状态源。默认 no-op（无维护信息），
// atlas 是第一个真实选项，Zabbix/Prometheus/云监控按同接口实现。
type ServerStatusProvider interface {
    Name() string // 注册表键（"noop" / "atlas" / "http" / …）
    // GetServerStatus 查询目标服务器维护状态；错误=状态未知（gate 按 fail-open 处理）。
    GetServerStatus(ctx context.Context, ref ServerRef) (ServerStatus, error)
    // ListServers 状态源已知的服务器清单（设置页映射校验/面板用）。
    ListServers(ctx context.Context) ([]ServerSummary, error)
}

// ServerRef 目标服务器定位（gate 侧从事件解析后传入）。
type ServerRef struct {
    AgentID string // croupier agentId（首选匹配键）
    Host    string // 主机名/IP（agent 元数据或事件携带）
    GameID  string
    Env     string
}

// ServerStatus 统一契约（用户令原文 getServerStatus(server) -> {healthy, in_maintenance, window, source}）。
type ServerStatus struct {
    Healthy        bool               // 状态源自视角的服务器健康
    InMaintenance  bool               // 是否处于维护窗口（gate 唯一裁决字段）
    Window         *MaintenanceWindow // 当前/最近维护窗口（可空）
    Source         string             // provider 名（"atlas"…；noop="noop"）
}
```

- **匹配键**：默认 `agentId`，可配 `host`/`ip`（agent_sessions 有 AgentID/GameID/Env/Addr，join 可做；映射不到=未知→fail-open 报警标 source_unknown）。
- **缓存**：状态查询结果按 ServerRef 缓存 TTL（默认 60s，可配），防告警风暴时打爆状态源；缓存内命中不回源；**超时/错误不缓存**（下次再查）。

## 3. Provider 注册表与配置

### 3.1 注册表（编译期工厂，与 Outlet 同构）

| 键       | 实现                                         | 默认                                    |
| -------- | -------------------------------------------- | --------------------------------------- |
| `noop`   | 真实 no-op：恒返回「无维护信息」             | 显式配 `provider: noop` 时（gate 直通） |
| `atlas`  | atlas REST 适配器（§4）                      | **缺省**（`provider` 留空时，批 S4 起） |
| `http`   | 通用 HTTP 状态源（任意返回状态 JSON 的服务） | 选项（批 S4 注册）                      |
| （未来） | zabbix / prometheus / 云监控                 | 按需注册                                |

- **默认 no-op 第一原则**（插件机制决策①）在 S4 的落点：`enabled=false`（缺省）= gate 整体旁路；`enabled=true` 且 `provider` 留空 = 缺省选 **atlas**（选源零配置，只需 `providers.atlas.baseUrl`）；显式配 `noop` = 无维护信息 gate 直通。真实源（atlas/http）缺 `baseUrl` 视为「从未配置」——装配旁路不挂 gate 并启动告警，**不进 fail-open**（fail-open 只针对「源挂了」，不针对「从未配置」）。切换=换注册项，gate 业务代码零改动。

### 3.2 配置段

```yaml
serverStatus:
  enabled: false # 缺省关（herald 同款缺省关先例）；false=gate 整体旁路
  provider: atlas # 留空=atlas；noop | atlas | http（注册表键）
  matchBy: agentId # agentId | host | ip
  cacheTtlSeconds: 60
  providers:
    atlas:
      baseUrl: http://atlas.internal:8080 # secguard 出站白名单内
      tokenEnv: ATLAS_API_TOKEN # 凭据环境变量引用，不落明文
      timeoutMs: 3000
    http:
      baseUrl: http://ops-status.internal:9100/status # baseUrl 即端点
      timeoutMs: 3000
```

- 接入路径判定（[外部服务接入模板](../templates/external-service-integration-template.md)）：**路径 B 平台接口位**（单实例语义、编译期 Provider + 配置段，`herald:`/`alertInbound:` 同款）；步骤 3/4/6 不适用；secrets=tokenEnv；拨测=单测（httptest stub）+ env 门控冒烟；面板=v1 状态标记进事件与设置页；下线=`enabled: false`。

### 3.3 provider 切换与 http 源逐字段（批 S4）

**切换=只改 `serverStatus.provider`（留空=atlas）+ 对应 `providers.<key>` 段**，gate 业务代码零改动；改完重启 server 生效。设置页「已启用外部服务」卡片（`serverStatus.<provider>` 行）随配置呈现三态，缺项点名可操作的配置键。

`providers.<key>` 逐字段（REST 源通用，atlas/http 共用同一组键）：

| 字段                  | atlas 缺省                           | http 缺省            | 说明                                                                                                                                                       |
| --------------------- | ------------------------------------ | -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `baseUrl`             | **必填**（缺=装配旁路不挂 gate）     | **必填**（同左）     | 状态源服务地址；http 源缺省它本身就是端点                                                                                                                  |
| `tokenEnv`            | 不填=不带鉴权                        | 不填=不带鉴权        | 持 API token 的环境变量名，请求带 `Authorization: Bearer <值>`；凭证不落配置文件                                                                           |
| `timeoutMs`           | 3000                                 | 3000                 | 单次查询超时；查询失败=fail-open 照报标 `source_unknown`（错误不缓存）                                                                                     |
| `statusPath`          | `/v1/discovery/servers/{id}`         | 空（baseUrl 即端点） | `{id}` 占位符=路径参数替换（PathEscape）；否则拼 `?matchKey=<值>`（baseUrl 已带 query 用 `&` 续接）                                                        |
| `matchKey`            | `agentId`                            | `agentId`            | 定位参数名（契约键），可换 `host`/`ip`                                                                                                                     |
| `fieldMapping`        | `agentId→id`、`inMaintenance→status` | 空=契约键名直读      | 契约字段名→状态源 JSON 键；契约字段闭集 `healthy`/`inMaintenance`/`windowStart`/`windowEnd`/`windowNote`；部分覆盖（只写要改的键，atlas 侧与默认映射合并） |
| `inMaintenanceStatus` | `maintenance`                        | `"true"`             | inMaintenance 字段是 bool 时直取不看此值；是字符串时按此值判等                                                                                             |

**http 源示例**——任意能返回状态 JSON 的简单服务（哪怕 `python3 -m http.server` 挂一个 JSON 文件）都能接：

```bash
# 源侧：GET http://ops-status.internal:9100/status?agentId=agent-1 返回
{"frozen": true, "ok": false, "since": "2026-10-12T00:00:00Z", "until": "2026-10-13T00:00:00Z", "why": "发版"}
```

```yaml
serverStatus:
  enabled: true
  provider: http # ← 只改这一行即从 atlas 切到 http
  providers:
    http:
      baseUrl: http://ops-status.internal:9100/status
      matchKey: agentId # 拼成 GET …?agentId=agent-1
      fieldMapping:
        inMaintenance: frozen
        healthy: ok
        windowStart: since
        windowEnd: until
        windowNote: why
```

- **响应形态**：数组（按匹配字段过滤取首条）或单对象都吃；http 源单对象**不含**匹配键字段时整体接受（URL 即标识——单服务器状态端点无需回显自身 id），携带但值不符才算无映射。
- **无映射/不可达**：错误不缓存、下次再查；gate fail-open 照报 + `source_unknown` 标注（宁误报不漏报）。

## 4. atlas 首个实现

- atlas 是用户自有项目（服务器状态/维护窗口管理）；croupier 侧先定死**本契约**（ServerStatusProvider + ServerStatus 字段闭集），atlas 适配器按其 API 实现映射。
- **边界（诚实）**：atlas 具体端点/字段/鉴权形态**待 atlas 侧对齐**（仓不在本仓；开工时若可访问 atlas 仓则按其实际 API 回填本节并补映射表；否则适配器以可配置路径模板 + 响应字段映射落地，见下）。
- 适配器骨架：`GET {baseURL}/api/servers?match=<ref>` → 响应字段映射到 ServerStatus（路径/字段映射可配，映射不到的字段按零值+Healthy 未知处理）；`test-connection` 拨测端点复用 cicd integration test 先例。

## 5. 接线与实现落点

| 落点                 | 位置                                                                                     | 说明                                                             |
| -------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Provider 接口+注册表 | `internal/platform/serverstatus/`（新包，仿 `internal/platform/outlet`）                 | 接口 + noop/atlas 实现 + 缓存 + ServiceContext 注册              |
| gate                 | `internal/svc`（Outlet 投递前钩子，supervisorEventToOutlet 同层）                        | 事件→ServerRef 解析（agent_sessions join）→ provider 查询→三分支 |
| 配置段               | `internal/config` `ServerStatus ServerStatusConfig`（lowerCamelCase tags）               | `serverStatus:` 段                                               |
| 事件标注             | AlertEvent.Metadata / 信封扩展位：`source_unknown: true`、`maintenance_suppressed: true` | 未知照报标注；抑制事件计数（日志+指标，禁静默丢）                |
| 抑制可见性           | server 日志 + 失败/抑制计数（Manager 既有口径）；面板专项视图 P2                         | 抑制不是丢弃：日志留痕可查                                       |

## 6. 分批交付计划（每批测试绿 → commit → push）

| 批    | 内容                                                                                                                               | 验收                                            |
| ----- | ---------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| 批 S1 | Provider 接口 + 注册表 + noop/atlas 骨架 + 配置段 + 缓存 + 单测                                                                    | gate 三分支单测绿（suppress/pass/unknown+标注） |
| 批 S2 | gate 接线（Outlet 投递前钩子 + ServerRef 解析 + agent_sessions join）+ 事件标注 + 抑制计数                                         | 出口链集成测试 + 出站失败不阻塞既有语义回归     |
| 批 S3 | atlas 适配器实联（atlas 仓可得则真实 API 对齐 + env 门控冒烟；不可得则可配映射 + stub 冒烟）+ 设置页「已启用外部服务」呈现         | 拨测绿 + 配置切换换注册项验证                   |
| 批 S4 | 默认源=atlas（provider 留空缺省）+ 通用 HTTP provider（`http` 注册项）+ atlas↔http↔noop 切换实测（含源挂掉降级）+ 配置文档（§3.3） | 切换与降级实测留证（同一 gate 装配面只改配置）  |

### 批 S1 交付说明（2026-10-10）

- 落点 `internal/platform/serverstatus/`：`provider.go`（Provider 接口 + ServerRef/ServerStatus/ServerSummary 契约 + ErrNoMapping/ErrProviderUnavailable 错误闭集）、`registry.go`（编译期工厂注册表，`Options{BaseURL,Token,Timeout,Atlas}` 构造参数；noop/atlas 内置注册）、`noop.go`（真实 no-op：恒「无维护信息」gate 直通）、`atlas.go`（REST 适配器骨架：`GET {baseURL}{statusPath}?{matchKey}={match}`，可配路径模板 + 响应字段映射（契约字段名→atlas JSON 键，alertInbound labelMapping 同款），数组/单对象响应都吃，Bearer token 头）、`cache.go`（进程内 TTL 缓存，只缓存成功结果，错误不缓存）、`gate.go`（三分支判定内核 + 抑制/未知/回源计数 + 日志留痕）。
- gate 三分支（S1 验收）：InMaintenance → Suppressed（计数+日志，落库不变）；非维护 → 照投；查询失败/超时/无映射/ref 无定位 → 照投 + SourceUnknown（宁误报不漏报）。provider 未布线或 ref 无法定位服务器 → 直通不过 gate。
- 配置段 `serverStatus:`（enabled/provider/matchBy/cacheTtlSeconds + `providers` 子段，key 为注册表键：baseUrl/tokenEnv/timeoutMs/statusPath/matchKey/fieldMapping）已落 `internal/config`，lowerCamelCase tags；svc 装配（config→Options 映射 + Outlet 挂钩）属 S2。
- 已知边界：atlas 默认字段名按小驼峰契约（`healthy`/`inMaintenance`/`windowStart`…），未对齐真实 atlas API 前为骨架口径（S3 回填）；非 bool/非 string 的映射值按零值处理。

### 批 S2 交付说明（2026-10-11）

- **gate 接线（机制在 outlet，策略在 svc）**：`outlet.Manager` 新增注入式 gate 钩子 `SetGate(GateFunc)`（`atomic.Pointer` 挂载，nil 卸载=直通；outlet 包零依赖 serverstatus——跨包环风险与平台包互耦都避开）。`Dispatch`（异步，goroutine 内投递前评估一次）与 `DispatchExternal`（同步，外部链遍历前评估一次）都过 gate——「出口链入口」单点裁决，一次评估覆盖整链（站内+外部）。gate 已有的抑制/未知/回源计数即设计 §5 的抑制可见性面（出口侧抑制不计入投递失败）。
- **gate 生效面**：只对 `Scope.AgentID` 非空（带服务器定位）的事件评估；无定位事件（报表推送 incident-report、生命周期 webhook 等）零调用照投——宁多报不漏报（设计 §1.2）。v1 事件源带 agentId 的现状：supervisor breaker 事件（`supervisorEventToOutlet`）是首个被 gate 的真实流。
- **抑制语义**：suppress=true → 整链静默（含站内），`Dispatch` 直接返回；`DispatchExternal` 返回 `ErrSuppressedByMaintenance`（报表 PushStatus 落库得到可解释原因，v1 报表事件无 agentId 实际不可达，分支为未来带定位的外部事件预留）。
- **事件标注**：`AlertEvent.Metadata map[string]string` 信封扩展位（键闭集 `MetaSourceUnknown`/`MetaMaintenanceSuppressed`，写时复制合入不改调用方信封）；出口渲染面 = 站内正文尾注 + herald 正文尾注 ` [source_unknown]`（键名字典序确定性输出）——herald DispatchRequest 无自由 metadata 位、messages 表无 metadata 列，尾注是标注到达值班人的最小面，不动 schema。抑制事件的标注只出现在 gate 日志（事件不出出口链）。
- **svc 装配**（`internal/svc/serverstatus_gate.go`）：`registerOutlets` 收尾调 `wireServerStatusGate`——enabled=true 且 provider 已注册才挂 gate（`installServerStatusGate` 纯装配可测）；config→`serverstatus.Options` 逐项映射（tokenEnv 环境变量解析、timeoutMs/statusPath/fieldMapping 透传、cacheTtlSeconds→缓存 TTL）；旁路三形态：enabled=false、provider 未注册、OutletManager 未布线（全部直通+零行为变化）。
- **matchBy 诚实回落**：config `matchBy: host|ip` 时启动告警并回落 `agentId`——v1 事件 scope 只带 agentId、`agent_sessions` 表无主机列（AgentID/GameID/Env/Region/Zone/Labels，无 Addr）可 join，host/ip 定位无从解析；atlas 适配器的 MatchKey=host/ip 能力保留（S1），待事件侧携带 host 或 session 表加主机列后放开。
- **测试**：outlet 包 gate 集成 8 用例（抑制整链静默+链入口恰一次评估、fail-open 标注到达双通道正文、无定位不过 gate、DispatchExternal 抑制/绕行、未挂 gate 回归、SetGate(nil) 卸载、尾注确定性）；svc 装配 5 用例（Options 逐项映射+tokenEnv 解析、抑制静默、matchBy 回落、未知标注照报、旁路形态）。`recordingSink`/`fakeStatusProvider` 测试替身补互斥/atomic——Dispatch 异步路径在 -race 下直读共享切片/int 即 DATA RACE。
- 已知边界：herald 外发正文带 ` [source_unknown]` 尾注（非结构化字段，herald 侧如需结构化需 SDK 加 metadata 位）；matchBy=host/ip 为配置陷阱已用启动告警兜底；维护窗内事件仍落 supervisor 事件环/告警页（gate 只静默出口链）。

### 批 S3 交付说明（2026-10-11）

- **atlas 实联**：适配器按 atlas 仓真实 API 对齐——register 要求 server_id/region/endpoint.host/port（201 + status=starting，首次有效心跳在老化窗内自动升 online）；discovery 列表对 offline/starting 服务器隐藏（需 register 重新进入）；`InMaintenance` 新增状态透传（config 同名段 `inMaintenance.status`/`windowStart`）。env 门控冒烟 `CROUPIER_ATLAS_SMOKE_BASE_URL`（可选 `CROUPIER_ATLAS_SMOKE_REGISTRY_BASE_URL` 自助 register+heartbeat 装配，幂等）自验证：online/维护两种形态下 ListServers 成员与 adapter InMaintenance 与 discovery status 枚举一致（re-register 保持维护态）。
- **设置页「已启用外部服务」呈现**：`GET /site/external-services`（admin 组）只读快照 herald / alertInbound（alertmanager/generic 闭集）/ serverStatus 三项 L2 文件侧启用态。三态：active=启用态且必填项全齐（与各服务 svc 装配判定同口径：herald=enabled+baseUrl+token 环境变量值；alertInbound=来源已配置+secretEnv 值；serverStatus=enabled+provider 已注册+非 noop baseUrl）；configured=配置段存在但缺项；未配置。缺项点名契约键/环境变量名（可操作提示，永不回显值）；secretEnv 只报变量名+已设置/未设置。卡片在运维 Tab（`ExternalServicesCard`），纯只读——页面侧配置编辑属「配置整理」单（`docs/design/config-organization.md`，待用户过目），变更需编辑 server.yaml 并重启。
- 已知边界：atlas 无维护窗历史 API，维护态真相在 atlas 侧；serverStatus 缺 baseUrl 的 fail-open 标注属运行时行为，设置页以缺项提示呈现（「配置未生效」而非误报 active）。

### 批 S4 交付说明（2026-10-12）

- **默认源=atlas**：`enabled=true` 且 `provider` 留空 → 缺省选 atlas（启动 Info 留痕），选源零配置——只需 `providers.atlas.baseUrl`；`enabled` 仍缺省 false（零配置=gate 整体旁路不变）。非 noop 源缺 `baseUrl` 视为「从未配置」：装配旁路不挂 gate + 启动告警点名缺项键（与设置页外部服务卡片同口径）——不进 fail-open，fail-open 只针对「源挂了」。显式 `provider: atlas` 但缺 baseUrl 的行为随之从「装 gate 后每条事件 fail-open」变为「旁路+告警」，消除逐事件错误噪音。
- **通用 HTTP provider（`http` 注册项）**：`internal/platform/serverstatus/http.go`，与 atlas 共用抽取出的 REST 映射引擎 `restSource`（`rest.go`——atlas.go 同步瘦身为引擎门面，行为零变化，atlas 测试原样全绿）。差异只在缺省口径：StatusPath 空=baseUrl 即端点（query 参数定位，baseUrl 已带 query 用 `&` 续接）、Fields 空=契约键名直读、inMaintenance 字符串缺省按 `"true"` 判等、单对象响应不含匹配键字段=URL 即标识整体接受（atlas 保持必须携带 id）。目标是「任意能返回状态 JSON 的简单服务都能接」。
- **切换实测（同一 `installServerStatusGate` 装配面，只改配置）**：`internal/svc/serverstatus_provider_switch_test.go` 六用例——provider 留空缺省 atlas（atlas 缺省口径零字段映射，只有 baseUrl）在维护=整链静默且源恰被回源一次 / 同源 online=照投零标注 / 切 `http`（自定义字段名+fieldMapping+bool 维护位）在维护=静默且 query 定位参数正确 / http 源挂掉（端口无人监听）=fail-open 照投+`source_unknown` / 切 `noop`（无源）=照投零标注 / 缺省 atlas 但从未配 baseUrl=旁路照投零标注。包内另有 http provider 单测 12 用例（缺省契约键直读、URL 即标识、数组过滤、字符串布尔三形态、子路径+窗口映射、路径占位符、query 续接、四路降级、ListServers 三形态、gate 三分支全链、注册表）。
- **配置文档**：§3.1 注册表/缺省说明、§3.2 配置段（`baseUrl` 键名修正为与实现一致）、§3.3 新增——provider 切换逐字段对照表（atlas/http 缺省差异）+ curl 状态 JSON→映射配置示例；`configs/server.yaml` serverStatus 注释示例同步（键闭集 noop/atlas/http、缺省说明、http 源示例）。
- 已知边界：http 源 `ListServers` 对单服务器状态对象解出空清单（无清单语义，接口允许）；`statusPath` 的 `{id}` 占位符是路径参数唯一形态（其余占位符不支持）；matchBy=host/ip 仍受 S2 边界约束（事件 scope 只带 agentId，配置侧回落并告警）。

## 已知边界（诚实清单）

- atlas API 契约已按 atlas 仓真实 API 对齐（2026-10-11，批 S3）；后续以 atlas 侧契约为真值。
- 非 noop 源缺 baseUrl = 装配旁路不挂 gate（「从未配置」≠「源挂了」）；真实源服务地址是接入的最低要求，缺省 atlas 也绕不开它。
- gate 抑制=不投递不落空：抑制事件有日志+计数；专项面板视图 P2。
- 匹配键解析不到服务器定位的事件不过 gate（照报）——多报不漏报。
- 缓存 TTL 内的维护窗变更最多延迟 TTL 生效（默认 60s）。
- 状态源不可达 fail-open 照报并标 source_unknown：短暂状态源故障会造成误报——这是用户裁决的取舍（宁误报不漏报）。
- v1 无按类别/游戏分级的维护规则（整服务器粒度）；维护窗口历史不做库表（状态源侧是真值）。
