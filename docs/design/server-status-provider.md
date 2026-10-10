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
    Name() string // 注册表键（"noop" / "atlas" / …）
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

| 键       | 实现                                        | 默认     |
| -------- | ------------------------------------------- | -------- |
| `noop`   | 真实 no-op：恒返回「无维护信息」            | **缺省** |
| `atlas`  | atlas REST 适配器（§4）                     | 选项     |
| （未来） | zabbix / prometheus / 云监控 / generic-http | 按需注册 |

- **默认 no-op 第一原则**（插件机制决策①）：`serverStatus.provider` 未配置或配 `noop` = 系统无维护信息、gate 直通（全部照报）——**零配置=零行为变化+零外发**；no-op 是真实注册实现，不是 if 散落；切换=换注册项，gate 业务代码零改动。

### 3.2 配置段

```yaml
serverStatus:
  enabled: false # 缺省关（herald 同款缺省关先例）；false=gate 整体旁路
  provider: noop # noop | atlas | …（注册表键）
  matchBy: agentId # agentId | host | ip
  cacheTtlSeconds: 60
  providers:
    atlas:
      baseURL: http://atlas.internal:8080 # secguard 出站白名单内
      tokenEnv: ATLAS_API_TOKEN # 凭据环境变量引用，不落明文
      timeoutMs: 3000
```

- 接入路径判定（[外部服务接入模板](../templates/external-service-integration-template.md)）：**路径 B 平台接口位**（单实例语义、编译期 Provider + 配置段，`herald:`/`alertInbound:` 同款）；步骤 3/4/6 不适用；secrets=tokenEnv；拨测=单测（httptest stub）+ env 门控冒烟；面板=v1 状态标记进事件与设置页；下线=`enabled: false`。

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

| 批    | 内容                                                                                                                       | 验收                                            |
| ----- | -------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| 批 S1 | Provider 接口 + 注册表 + noop/atlas 骨架 + 配置段 + 缓存 + 单测                                                            | gate 三分支单测绿（suppress/pass/unknown+标注） |
| 批 S2 | gate 接线（Outlet 投递前钩子 + ServerRef 解析 + agent_sessions join）+ 事件标注 + 抑制计数                                 | 出口链集成测试 + 出站失败不阻塞既有语义回归     |
| 批 S3 | atlas 适配器实联（atlas 仓可得则真实 API 对齐 + env 门控冒烟；不可得则可配映射 + stub 冒烟）+ 设置页「已启用外部服务」呈现 | 拨测绿 + 配置切换换注册项验证                   |

### 批 S1 交付说明（2026-10-10）

- 落点 `internal/platform/serverstatus/`：`provider.go`（Provider 接口 + ServerRef/ServerStatus/ServerSummary 契约 + ErrNoMapping/ErrProviderUnavailable 错误闭集）、`registry.go`（编译期工厂注册表，`Options{BaseURL,Token,Timeout,Atlas}` 构造参数；noop/atlas 内置注册）、`noop.go`（真实 no-op：恒「无维护信息」gate 直通）、`atlas.go`（REST 适配器骨架：`GET {baseURL}{statusPath}?{matchKey}={match}`，可配路径模板 + 响应字段映射（契约字段名→atlas JSON 键，alertInbound labelMapping 同款），数组/单对象响应都吃，Bearer token 头）、`cache.go`（进程内 TTL 缓存，只缓存成功结果，错误不缓存）、`gate.go`（三分支判定内核 + 抑制/未知/回源计数 + 日志留痕）。
- gate 三分支（S1 验收）：InMaintenance → Suppressed（计数+日志，落库不变）；非维护 → 照投；查询失败/超时/无映射/ref 无定位 → 照投 + SourceUnknown（宁误报不漏报）。provider 未布线或 ref 无法定位服务器 → 直通不过 gate。
- 配置段 `serverStatus:`（enabled/provider/matchBy/cacheTtlSeconds + `providers` 子段，key 为注册表键：baseUrl/tokenEnv/timeoutMs/statusPath/matchKey/fieldMapping）已落 `internal/config`，lowerCamelCase tags；svc 装配（config→Options 映射 + Outlet 挂钩）属 S2。
- 已知边界：atlas 默认字段名按小驼峰契约（`healthy`/`inMaintenance`/`windowStart`…），未对齐真实 atlas API 前为骨架口径（S3 回填）；非 bool/非 string 的映射值按零值处理。

## 已知边界（诚实清单）

- atlas API 契约待 atlas 侧对齐：S3 之前 atlas 适配器为可配置映射骨架，未对齐前不宣称实联。
- gate 抑制=不投递不落空：抑制事件有日志+计数；专项面板视图 P2。
- 匹配键解析不到服务器定位的事件不过 gate（照报）——多报不漏报。
- 缓存 TTL 内的维护窗变更最多延迟 TTL 生效（默认 60s）。
- 状态源不可达 fail-open 照报并标 source_unknown：短暂状态源故障会造成误报——这是用户裁决的取舍（宁误报不漏报）。
- v1 无按类别/游戏分级的维护规则（整服务器粒度）；维护窗口历史不做库表（状态源侧是真值）。
