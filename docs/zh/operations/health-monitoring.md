---
title: 运维健康监控说明
---

# 运维健康监控说明

本文按「端点与探针 → 指标 → 告警阈值 → 日志与集成」的主线核对实现。现状是本地与集中式观测并存的混合架构：Server 暴露公开健康端点和可选的 Prometheus 端点；Agent 采集系统指标并执行业务探测；集中规则、告警生命周期与外部派发在 Server 侧。

## 健康检查端点

| 端点                                                                              | 认证     | 用途                                                             |
| --------------------------------------------------------------------------------- | -------- | ---------------------------------------------------------------- |
| `GET /healthz` 或 `GET /api/v1/monitoring/healthz`                                | 公开     | 检查数据库、Agent 注册表、运维状态三个组件；任一失败整体即不健康 |
| `GET /api/v1/monitoring/metrics`、`/status`                                       | 需认证   | JSON 快照，含数据库延迟、Agent 健康、函数计数与运维摘要          |
| `GET /metrics/prometheus`                                                         | 公开抓取 | 可选 Prometheus 暴露端点，默认关闭                               |
| `GET /api/v1/ops/health`、`POST /api/v1/ops/health/run`、`PUT /api/v1/ops/health` | 需认证   | 运维页健康检查清单；当前只存配置与合成状态                       |
| `POST /api/v1/ops/probe/diagnose`                                                 | 需认证   | 手动执行一条已配置的 server 探针                                 |

- 路由：`internal/handler/routes.go:101` 挂载公开监控组，`internal/handler/routes.go:104` 按条件挂载 Prometheus 暴露端点；`internal/handler/routes.go:325` 注册 `/healthz`，`internal/handler/routes.go:332` 注册受保护的 `metrics/status`。
- 健康聚合：`internal/api/monitoring/service.go:23` 检查三个组件；`internal/api/monitoring/service.go:97` 执行 `SELECT 1` 并记录延迟；`internal/api/monitoring/service.go:121` 统计已注册 Agent 总数、未过期 Agent 数与启用函数数；`internal/api/monitoring/service.go:156` 聚合运维状态。`internal/api/monitoring/service.go:179` 把各组件的 `ok` 布尔合并为整体健康。
- Prometheus 端点：`internal/config/config.go:306` 定义 `telemetry.prometheus.enabled/path`，`internal/config/config.go:315` 默认路径为 `/metrics/prometheus`。采集器在 `internal/api/monitoring/prometheus.go:88` 注册 Go、process 与平台采集器，数据库采集超时 5 秒（`internal/api/monitoring/prometheus.go:53`）。
- **运维页健康检查当前不做真实探测**：`internal/api/ops/helpers.go:935` 对所有检查直接置 `ok=true`，`internal/api/ops/helpers.go:942` 注明 tcp/http/https 只记录合成成功，不发网络连接、不使用 `TimeoutMs`。它不是生产健康探针。
- **计划**：把 `OpsHealthCheck` 执行器收敛到 `core/healthprobe`，遵循配置的超时、期望断言与间隔；后台调度复用 Agent 探测状态机，失败开窗转 Alerts。

## Agent 健康探测

Agent 侧配置 `probe.targets[]` 支持 `http`、`tcp`、`db`、`protocol` 四类。

- 目标契约：`core/healthprobe/targets.go:25` 定义目标字段；`core/healthprobe/targets.go:51` 应用显式间隔，缺省 30 秒。
- 默认阈值：`core/healthprobe/probe.go:95` 连续失败 3 次开窗、连续成功 1 次闭窗；30 秒间隔下最坏约 90 秒窗口。
- 默认超时：HTTP 5 秒（`core/healthprobe/targets.go:69`、`core/healthprobe/healthprobe.go:48`；2xx/3xx 状态码通过，见 `core/healthprobe/healthprobe.go:67`）；TCP 2 秒（`core/healthprobe/targets.go:78`）；DB 默认 5 秒，执行 `SELECT 1` 或自定义只读查询（`core/healthprobe/db.go:11`、`core/healthprobe/db.go:19`、`core/healthprobe/db.go:42`）；protocol 单步 5 秒、建连 2 秒（`core/healthprobe/protocol.go:24`）。
- 执行位置：`internal/app/agent/probe_manager.go:19` 组装 `probe.targets[] → Manager → ProbeUplink → MetricsReport`；`internal/app/agent/probe_manager.go:25` 明确探测只观察、绝不触发进程恢复。`configs/agent.yaml:78` 是目标示例。
- 采样策略：`core/healthprobe/policy.go:29` 把异常上报下限设为 5 秒；`core/healthprobe/policy.go:192` 按目标与失败窗状态计算实际间隔。`internal/app/agent/probe_manager.go:62` 持有策略快照并在失败窗期间启用加速。

Server 还有本地探测面，面向只有 Server 可见的依赖或第三方 API：`internal/svc/server_probe.go:3` 说明该视角，`cmd/server/root.go:461` 装配，`cmd/server/root.go:473` 启动。目标契约与默认阈值与 Agent 一致（`configs/server.yaml:206`）；空目标集不装配（`internal/svc/server_probe.go:31`），非法目标启动失败（`internal/svc/server_probe.go:53`）。

## 指标

### Server 平台指标

| 指标                            | 类型     | 含义                     |
| ------------------------------- | -------- | ------------------------ |
| `croupier_db_up`                | Gauge    | 数据库连通性，1/0        |
| `croupier_db_latency_ms`        | Gauge    | 数据库 ping 延迟（毫秒） |
| `croupier_agents_total`         | Gauge    | 已注册 Agent 数          |
| `croupier_agents_healthy`       | Gauge    | 会话未过期的 Agent 数    |
| `croupier_functions_registered` | Gauge    | 当前启用函数数           |
| `go_*`、`process_*`             | 标准指标 | Go 运行时与进程指标      |

证据：`internal/api/monitoring/prometheus.go:30` 注册指标描述符，`internal/api/monitoring/prometheus.go:53` 在抓取时采集数据库指标，`internal/api/monitoring/prometheus.go:69` 送出注册表指标，`internal/api/monitoring/prometheus.go:88` 附加 Go/process 标准指标。

### Agent 系统与业务指标

- `core/metrics/collector.go:88` 采集 CPU、内存、磁盘、网络、托管进程、探测统计与 Go 运行时自定义指标。
- 默认每 30 秒上报：`internal/app/agent/upstream.go:586` 是循环，`internal/app/agent/upstream.go:588` 在零值时回落 30 秒；配置入口在 `cmd/sidecar-agent/root.go:648`。
- 指标入库：`internal/server/control_handler.go:1010` 接收 `MetricsReport`；`internal/platform/registry/metrics.go:143` 写内存环并异步持久化，`internal/platform/registry/metrics.go:171` 异步触发告警规则。默认容量：每 Agent 10 份、总量 2000 份、历史 7 天、1 小时清理（`internal/platform/registry/metrics.go:115`）。
- 存活判定不设独立心跳通道，而以注册表会话过期时间为准：`internal/api/monitoring/service.go:142` 只统计 `ExpireAt` 未到的 Agent。心跳循环由 `core/register/loops.go:76` 驱动，默认 3 秒。
- 托管进程资源阈值只标记、不杀进程；自动重启退避默认首次 1 秒、上限 60 秒，连续 5 次失败触发熔断、须人工复位（`configs/agent.yaml:55`）。

## 告警规则与阈值

### 阈值规则

告警规则存数据库；指标路径支持 CPU、内存、磁盘、custom 与探测聚合指标。模型与求值语义见 `internal/model/alert_rule.go:10`。求值挂在每次 Agent 指标上报之后：`internal/svc/service_context.go:592` 构造求值器并接入 `MetricsStore.SetOnReport`；`internal/platform/alertrule/evaluator.go:49` 遍历启用规则，`internal/platform/alertrule/evaluator.go:63` 取当前值并比较。

- 持续触发：`internal/platform/alertrule/evaluator.go:74` 把低于 1 的 `ForCount` 收敛到 1；连续命中达到阈值才触发，未命中在 `internal/platform/alertrule/evaluator.go:83` 复位。
- 默认冷却：`internal/platform/alertrule/evaluator.go:93` 使用 `CooldownSeconds`，非正值回落 5 分钟。
- 阈值/级别/scope 可配：规则 CRUD 在 `internal/api/alert/rules.go:141`、`internal/api/alert/rules.go:176`，路由在 `internal/handler/routes.go:744`。`internal/platform/alertrule/evaluator.go:7` 列出支持的路径，四个探测字段在 `internal/platform/alertrule/evaluator.go:15`。
- **平台没有内置告警阈值**：阈值、比较符、`ForCount`、冷却与 scope 都是规则行上的字段。`docs/operations/monitoring.md:199` 中的 Prometheus 表达式只是部署侧示例，不是仓库自动加载的告警配置；其中 `croupier_server_requests_total` 未实现。

### 探测开窗告警

- 连续失败达到 `failThreshold` 开窗，Agent 立即上报窗口事件；服务端以 `alertID=probe:<agentId>:<target>` 幂等落库。`internal/server/control_handler.go:917` 分派窗口事件，`internal/server/probe_lifecycle.go:90` 处理 firing；`internal/server/probe_lifecycle.go:105` 处理恢复，重放去重。
- 探测告警固定 warning 级（`internal/server/probe_lifecycle.go:120`）。
- 异常明细即时上报，默认每失败窗每目标最多 10 条（`internal/app/agent/probe_manager.go:42`）；服务端在告警明细里保留最近 5 条（`internal/server/probe_lifecycle.go:29`）。
- 指标侧另有 `probe.<target>.successRate/failed/checked/latencyP95Ms`，按 30 秒上报窗聚合；`internal/platform/alertrule/evaluator.go:233` 按 target 取值。

### 通知派发

- `internal/platform/alertrule/evaluator.go:110` 创建 firing Alert；`internal/platform/alertrule/evaluator.go:140` 在 notifier 为 nil 时只落库。生产装配在 `internal/svc/service_context.go:594`，当前传 nil，随后 `internal/svc/service_context.go:597` 注册出站链。
- 告警出站通道、herald 与维护窗抑制不在本篇展开；平台没有在恢复时自动 resolve 指标阈值告警的通用机制。

## 日志与监控集成

### 日志

- 配置结构：`core/logx/logging.go:17` 支持 `debug/info/warn/error`、`console/json`、stdout/stderr 与轮转文件字段。
- Server 默认：`cmd/server/root.go:153` 把空 level 设为 `info`，`cmd/server/root.go:157` 把空 format 设为 `console`，`cmd/server/root.go:160` 初始化日志。
- JSON 与轮转：`core/logx/logging.go:71` 设级别，`core/logx/logging.go:72` 选 JSON handler，`core/logx/logging.go:51` 用 lumberjack 轮转文件，`core/logx/logging.go:54` 同步写控制台与文件。
- **日志自动转发到 Loki/OTLP 当前未实现**：没有内置 collector 客户端。集成方式是容器 stdout 采集或挂载轮转文件，再用 Promtail/Fluent Bit/Vector 外部采集。计划是加可选的 OTLP logs exporter，本地轮转保留为兜底。

### Prometheus / Grafana

1. 在 server 配置中启用：

```yaml
telemetry:
  prometheus:
    enabled: true
    path: /metrics/prometheus
```

2. Prometheus 抓取该路径。仓库附带本地 telemetry 栈配置，抓取 OTel Collector 的 8889 exporter；15 秒全局抓取间隔在 `configs/prometheus.yml:3`，Collector 目标在 `configs/prometheus.yml:11`。
3. Grafana 面板需手动导入；仓库不附带自动预置面板。

### OpenTelemetry

OTLP 走 HTTP，可同时启用 tracing 与 metrics。配置映射在 `internal/telemetry/provider.go:28`；`internal/telemetry/provider.go:83` 初始化 tracing，`internal/telemetry/provider.go:92` 初始化 metrics。默认 collector 为 `http://localhost:4318`（`internal/telemetry/provider.go:247`），可用 `OTEL_EXPORTER_OTLP_ENDPOINT` 覆盖；tracing 每 5 秒批量、采样率可配（`internal/telemetry/provider.go:150`），metrics 每 30 秒推送（`internal/telemetry/provider.go:190`）。

`configs/telemetry.example.yaml:5` 提供服务身份、collector、采样率与 Prometheus 开关示例；`configs/otel-collector-config.yaml:8` 暴露 OTLP gRPC/HTTP，`configs/otel-collector-config.yaml:19` 分别导出到 Jaeger 与 Prometheus。

**当前未实现**：server 请求总数、请求耗时直方图与自动 HTTP 中间件指标；探测/Agent 指标不会自动转成 Prometheus 指标。计划是在请求中间件与派发层加 OTel HTTP 指标，并扩展平台采集器到探测开窗、告警计数与状态源裁决计数。

## 当前缺口汇总

| 缺口                | 现状                                                | 计划                                          |
| ------------------- | --------------------------------------------------- | --------------------------------------------- |
| 运维页健康检查      | 手动执行恒成功（`internal/api/ops/helpers.go:942`） | 迁到 `core/healthprobe` 并接入失败开窗        |
| HTTP 服务指标       | 只有 Go/process 与五个平台 Gauge                    | 加请求计数、耗时与错误率                      |
| 日志转发            | 无内置 Loki/OTLP logs 出口                          | 加可选 exporter，stdout/文件保留为兜底        |
| 指标告警自动恢复    | 规则不再命中不更新旧 Alert 状态                     | 加 AlertID 绑定、恢复检测与 resolved 生命周期 |
| 第三方监控 SLO 汇总 | 无 SLO/错误预算对象                                 | 先导出探测开窗与告警计数，再建 SLO 计算       |
