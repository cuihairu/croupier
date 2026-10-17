---
title: 运维健康监控说明
---

# 运维健康监控说明

本文按“接口与探针 → 指标 → 告警阈值 → 日志与接入”核对实现。当前是本地和中心化观测系统混合架构：server 暴露公开健康接口和可选 Prometheus 端点；Agent 采集系统指标并执行业务探针；中心规则、告警生命周期和外部分发落在 server。

## 健康检查接口

| 入口                                                                              | 鉴权     | 用途                                                         |
| --------------------------------------------------------------------------------- | -------- | ------------------------------------------------------------ |
| `GET /healthz` 或 `GET /api/v1/monitoring/healthz`                                | 公开     | 检查数据库、Agent 注册表和运维状态三组件，任一失败整体不健康 |
| `GET /api/v1/monitoring/metrics`、`/status`                                       | 受保护   | JSON 快照，包含数据库延迟、Agent 健康、函数数和运维摘要      |
| `GET /metrics/prometheus`                                                         | 公开抓取 | 可选 Prometheus exposition，默认关闭                         |
| `GET /api/v1/ops/health`、`POST /api/v1/ops/health/run`、`PUT /api/v1/ops/health` | 需认证   | 运维页健康检查清单；当前只存配置和合成状态                   |
| `POST /api/v1/ops/probe/diagnose`                                                 | 需认证   | 手动执行一个已配置的 server 探针                             |

- 路由：`internal/handler/routes.go:101` 挂公开监控组，`internal/handler/routes.go:104` 有条件挂 Prometheus exposition；`internal/handler/routes.go:325` 注册 `/healthz`，`internal/handler/routes.go:332` 注册受保护 `metrics/status`。
- 健康聚合：`internal/api/monitoring/service.go:23` 检查三组件；`internal/api/monitoring/service.go:97` 执行 `SELECT 1` 并记录延迟；`internal/api/monitoring/service.go:121` 统计注册 Agent 总数、未过期 Agent 数和启用函数；`internal/api/monitoring/service.go:156` 汇总运维状态。`internal/api/monitoring/service.go:179` 用组件 `ok` 布尔值合成整体健康。
- Prometheus 端点：`internal/config/config.go:306` 定义 `telemetry.prometheus.enabled/path`，`internal/config/config.go:315` 默认路径是 `/metrics/prometheus`。采集器在 `internal/api/monitoring/prometheus.go:88` 注册 Go、进程和平台采集器，并设 5 秒数据库收集超时（`internal/api/monitoring/prometheus.go:53`）。
- **运维页健康检查当前未实现真实探测**：`internal/api/ops/helpers.go:935` 对所有 check 直接设 `ok=true`，`internal/api/ops/helpers.go:942` 注明 tcp/http/https 只记录合成成功，不执行网络连接，也不使用 `TimeoutMs`。它不是生产健康探针。
- **计划**：把 `OpsHealthCheck` 执行器收敛到 `core/healthprobe`，执行配置的 timeout、expected 断言和间隔；后台调度沿用 Agent 探针状态机，失败窗口转 Alert。

## Agent 健康探针

Agent 侧配置 `probe.targets[]`，类型支持 `http`、`tcp`、`db`、`protocol`。

- 目标契约：`core/healthprobe/targets.go:25` 定义目标字段；`core/healthprobe/targets.go:51` 应用显式间隔，否则默认 30 秒。
- 默认阈值：`core/healthprobe/probe.go:95` 设置连续失败 3 次开窗、连续成功 1 次闭窗；30 秒间隔下最坏约 90 秒开窗。
- 默认超时：HTTP 5 秒（`core/healthprobe/targets.go:69`、`core/healthprobe/healthprobe.go:48`，状态码 2xx/3xx 通过见 `core/healthprobe/healthprobe.go:67`）；TCP 2 秒（`core/healthprobe/targets.go:78`）；DB 默认 5 秒并执行 `SELECT 1` 或自定义只读查询（`core/healthprobe/db.go:11`、`core/healthprobe/db.go:19`、`core/healthprobe/db.go:42`）；协议单步 5 秒、建连 2 秒（`core/healthprobe/protocol.go:24`）。
- 执行位置：`internal/app/agent/probe_manager.go:19` 装配 `probe.targets[] → Manager → ProbeUplink → MetricsReport`；`internal/app/agent/probe_manager.go:25` 明确探针只做观测，不触发进程恢复。`configs/agent.yaml:78` 是目标示例。
- 采样策略：`core/healthprobe/policy.go:29` 定下异常提频下限 5 秒；`core/healthprobe/policy.go:192` 根据目标与故障窗状态计算实际间隔。`internal/app/agent/probe_manager.go:62` 持有策略快照，故障窗口启用提频。

Server 也有本机探针面，用于只有 server 可见的依赖或第三方 API。`internal/svc/server_probe.go:3` 说明该视角；`cmd/server/root.go:461` 装配，`cmd/server/root.go:473` 启动。目标契约和默认阈值与 Agent 一致（`configs/server.yaml:206`）；空目标集不装配（`internal/svc/server_probe.go:31`），非法目标启动失败（`internal/svc/server_probe.go:53`）。

## 指标

### Server 平台指标

| 指标                            | 类型     | 含义                  |
| ------------------------------- | -------- | --------------------- |
| `croupier_db_up`                | Gauge    | 数据库连通性，1/0     |
| `croupier_db_latency_ms`        | Gauge    | 数据库 ping 延迟毫秒  |
| `croupier_agents_total`         | Gauge    | 已注册 Agent 数       |
| `croupier_agents_healthy`       | Gauge    | 会话未过期的 Agent 数 |
| `croupier_functions_registered` | Gauge    | 当前启用函数数        |
| `go_*`、`process_*`             | 标准指标 | Go runtime 与进程指标 |

证据：`internal/api/monitoring/prometheus.go:30` 注册指标描述，`internal/api/monitoring/prometheus.go:53` 在抓取时收集数据库指标，`internal/api/monitoring/prometheus.go:69` 发送注册表指标，`internal/api/monitoring/prometheus.go:88` 附带 Go/process 标准指标。

### Agent 系统与业务指标

- `core/metrics/collector.go:88` 采集 CPU、内存、磁盘、网络、受管进程、探针统计和 Go runtime 自定义指标。
- 默认每 30 秒上报：`internal/app/agent/upstream.go:586` 是循环，`internal/app/agent/upstream.go:588` 零值回退 30 秒；配置入口是 `cmd/sidecar-agent/root.go:648`。
- 指标入站：`internal/server/control_handler.go:1010` 接收 `MetricsReport`；`internal/platform/registry/metrics.go:143` 写内存环并异步落库，`internal/platform/registry/metrics.go:171` 异步触发告警规则。默认保留 10 份/Agent、2000 份总量、7 天历史、1 小时清理（`internal/platform/registry/metrics.go:115`）。
- 存活判定不是心跳独立通道，而是注册表会话过期时间：`internal/api/monitoring/service.go:142` 只把 `ExpireAt` 未到的 Agent 计为健康。心跳循环由 `core/register/loops.go:76` 驱动，缺省 3 秒。
- 受管进程资源阈值只打标记，不杀进程；自动重启退避默认 1 秒起、60 秒封顶，连续失败 5 次熔断需要人工复位（`configs/agent.yaml:55`）。

## 告警规则与阈值

### 阈值规则

告警规则存储在数据库，指标路径支持 CPU、内存、磁盘、自定义和探针聚合指标。模型与评估语义在 `internal/model/alert_rule.go:10`。评估挂在每次 Agent 指标上报后：`internal/svc/service_context.go:592` 构造评估器并接入 `MetricsStore.SetOnReport`；`internal/platform/alertrule/evaluator.go:49` 遍历启用规则，`internal/platform/alertrule/evaluator.go:63` 提取当前值并比较。

- 持续触发：`internal/platform/alertrule/evaluator.go:74` 把 `ForCount` 小于 1 修正为 1，连续命中达到阈值才触发；未命中在 `internal/platform/alertrule/evaluator.go:83` 重置。
- 默认冷却：`internal/platform/alertrule/evaluator.go:93` 使用 `CooldownSeconds`，非正值回退 5 分钟。
- 可配置阈值/级别/范围：规则 CRUD 在 `internal/api/alert/rules.go:141`、`internal/api/alert/rules.go:176`，路由在 `internal/handler/routes.go:744`。`internal/platform/alertrule/evaluator.go:7` 列出支持路径；探针四字段在 `internal/platform/alertrule/evaluator.go:15`。
- **平台没有内置告警阈值**：阈值、比较符、`ForCount`、冷却和 scope 都是规则行字段。`docs/operations/monitoring.md:199` 的 Prometheus 表达式只是部署侧示例，不是仓库自动加载的告警配置；其中 `croupier_server_requests_total` 未实现。

### 探针窗口告警

- 连续失败达 `failThreshold` 开窗，Agent 即时上报窗口事件；server 用 `alertID=probe:<agentId>:<target>` 幂等落库。`internal/server/control_handler.go:917` 分派窗口事件，`internal/server/probe_lifecycle.go:90` 处理 firing；`internal/server/probe_lifecycle.go:105` 处理恢复，重放不重复。
- 探针告警固定 warning 级别（`internal/server/probe_lifecycle.go:120`）。
- 异常明细即时上报，每故障窗口每目标默认最多 10 条（`internal/app/agent/probe_manager.go:42`）；server 保留最近 5 条明细进告警 details（`internal/server/probe_lifecycle.go:29`）。
- 指标侧另有 `probe.<target>.successRate/failed/checked/latencyP95Ms`，由 30 秒上报窗口聚合；`internal/platform/alertrule/evaluator.go:233` 按目标取值。

### 通知分发

- `internal/platform/alertrule/evaluator.go:110` 创建 firing Alert；`internal/platform/alertrule/evaluator.go:140` 通知为 nil 时仅落库。生产装配在 `internal/svc/service_context.go:594` 当前传 nil，随后 `internal/svc/service_context.go:597` 注册出口链。
- 告警出口、herald 与维护窗抑制不在本文展开；平台没有恢复时自动 resolve 指标阈值告警的通用机制。

## 日志与监控接入

### 日志

- 配置结构：`core/logx/logging.go:17` 支持 `debug/info/warn/error`、`console/json`、stdout/stderr 和轮转文件字段。
- server 默认：`cmd/server/root.go:153` 把空级别设为 `info`，`cmd/server/root.go:157` 把空格式设为 `console`，`cmd/server/root.go:160` 初始化日志。
- JSON 与轮转：`core/logx/logging.go:71` 设置级别，`core/logx/logging.go:72` 选择 JSON handler，`core/logx/logging.go:51` 使用 lumberjack 文件轮转，`core/logx/logging.go:54` 同步写 console 和文件。
- **当前未实现日志自动转发到 Loki/OTLP**：没有内置 collector 客户端。接入方式是容器 stdout 收集或挂载轮转文件，再由外部 Promtail/Fluent Bit/Vector 采集。计划是增加可选 OTLP logs exporter，并保持本地轮转为兜底。

### Prometheus / Grafana

1. 在 server 配置中开启：

```yaml
telemetry:
  prometheus:
    enabled: true
    path: /metrics/prometheus
```

2. Prometheus 抓取该路径。仓库自带本机 telemetry 栈配置抓取 OTel Collector 的 8889 exporter，15 秒全局抓取间隔见 `configs/prometheus.yml:3`，Collector 目标见 `configs/prometheus.yml:11`。
3. Grafana 需自行导入面板；仓库没有随代码交付的自动配置面板。

### OpenTelemetry

OTLP 使用 HTTP，可同时开启 tracing 和 metrics。配置映射在 `internal/telemetry/provider.go:28`；`internal/telemetry/provider.go:83` 初始化 tracing，`internal/telemetry/provider.go:92` 初始化 metrics。默认 collector 是 `http://localhost:4318`（`internal/telemetry/provider.go:247`），可通过 `OTEL_EXPORTER_OTLP_ENDPOINT` 覆盖；tracing 5 秒批量、采样率可配（`internal/telemetry/provider.go:150`），metrics 每 30 秒推送（`internal/telemetry/provider.go:190`）。

`configs/telemetry.example.yaml:5` 提供服务标识、collector、采样率与 Prometheus 开关示例；`configs/otel-collector-config.yaml:8` 暴露 OTLP gRPC/HTTP，`configs/otel-collector-config.yaml:19` 分别导出 Jaeger 和 Prometheus。

**当前未实现**：server 请求总数、请求时长直方图和 HTTP 中间件自动指标；探针/Agent 指标不会自动转成 Prometheus 指标。计划是在请求中间件和 dispatch 层添加 OTel HTTP metric，并把平台 collector 扩展到探针窗口、告警计数与状态源裁决计数。

## 当前缺口汇总

| 缺口                | 现状                                                 | 计划                                            |
| ------------------- | ---------------------------------------------------- | ----------------------------------------------- |
| 运维页健康检查      | 手动 run 恒成功（`internal/api/ops/helpers.go:942`） | 迁到 `core/healthprobe` 并接失败窗口            |
| HTTP 服务指标       | 只有 Go/process 与五个平台 Gauge                     | 增加请求 counter、持续时间和错误率              |
| 日志转发            | 无内置 Loki/OTLP logs 出口                           | 增加可选 exporter，保留 stdout/file 兜底        |
| 指标告警自动恢复    | 规则不再命中不更新旧 Alert 状态                      | 增加 AlertID 绑定、恢复判定和 resolved 生命周期 |
| 第三方监控 SLO 汇总 | 无 SLO/错误预算对象                                  | 先导出探针窗口和告警计数，再建 SLO 计算         |
