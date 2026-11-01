# OPS-010 节点资源指标与进程清单采集（2026-11-01）

日期：2026-11-01。交付：agent 周期采集节点指标（CPU/内存/磁盘/网络收发速率）+ 进程清单（名称/pid/内存/监听端口）+ 端口列表；服务端入库（原始 7 天 + 小时聚合 90 天）；节点详情面板出资源曲线（CPU/内存/磁盘原有 + 网络速率新增）与进程/端口清单。

## 1. 交付前现状与缺口

既有（OPS-010 前）：CPU/内存/磁盘采集、心跳独立批量上报、`agent_metrics_history` 原始表（7 天保留）、面板 CPU/内存/磁盘曲线、采集开关（`MetricsEnabled`）、间隔可配（默认 30s）。

本轮补的缺口：

| 缺口                 | 补法                                                                                                             |
| -------------------- | ---------------------------------------------------------------------------------------------------------------- |
| 进程清单未采集       | `core/metrics/processes.go`：top-N(50) 按 RSS 排序，含名称/pid/内存/线程/状态/启动时间/监听端口                  |
| 端口列表未采集       | 同文件 `scanListeners`：tcp/tcp6/udp/udp6 一次扫描派生两用（进程内嵌端口 + 全节点端口表）                        |
| 网络只有累计字节     | `core/metrics/disknet.go`：新增 bytes/packets 收发速率（两次扫描差/间隔），首报为 0                              |
| 原始表无三面列       | 迁移 0051 加 `networks_json`/`processes_json`/`ports_json`（0025 加列模式）                                      |
| 无 90 天聚合         | 迁移 0051 建 `agent_metrics_hourly`；`RollupHourly` 每小时聚合上一整小时，`PruneHourly` 90 天                    |
| 默认间隔 30s         | `internal/app/agent/ops_config.go`：默认 15s（最小 3s、最大 24h，可配）                                          |
| history API 无三面   | `agentMetricsHistory` 补 networks/processes/ports 转换；新增 `window=raw\|hourly`（raw 默认 7 天，hourly 90 天） |
| 面板无网络/进程/端口 | `NodeDetailDrawer` 补网络速率曲线 + 进程/端口清单表（取最新一报快照）                                            |

## 2. 实现要点

- **采集库**：`core/metrics`（gopsutil v4，跨平台无外部依赖），agent-core K3 独立于函数通道，capture/devops/sidecar/synthetics 后续按需接入同一库（`NewCollector` + `WithSampler`/`WithProbeSampler` 已提供扩展点）。
- **上报**：`MetricsReport` 经现有 `SendMetricEvent` 通道随心跳或独立批量（默认 15s）；`MetricsEnabled=false` 时 `app.go:618` 不启动上报循环 → 零上报流量（关闭采集验收面）。
- **持久化**：`reg.MetricsStore.Add` 异步落 `agent_metrics_history`；三面空切片不落列（NULL 而非 `"[]"`），读侧按列非空重建。`StartCleanupRoutine` 每周期先 `RollupHourly` + `PruneHourly(90d)` 再 `Prune(7d)`。
- **聚合**：`RollupHourly` 取上一整小时（`[now-1h 截断到小时, +1h)`）原始行，按 agent 聚合成一行：CPU/内存使用率均值、各挂载点使用率均值（JSON）、各接口收发速率均值（JSON）、进程/端口计数均值；`(agent, hour)` 幂等 upsert（find-or-create，跨 sqlite/pg/mysql 无方言 ON CONFLICT）。
- **面板**：网络速率曲线按 `接口 TX/RX` 双序列；进程/端口清单取最新一报（`metricsHistory[0]`）渲染 antd Table。

## 3. 字段语义与 OBS-AGENT-001 对齐

指标字段名遵循 proto 契约（`proto/croupier/ops/v1/ops.proto`），与 OBS-AGENT-001 的 OTel 语义对齐约定：`usage_percent`/`used_bytes`/`total_bytes`/`bytes_sent_rate`/`bytes_recv_rate` 均以「使用率百分比（0-100）+ 字节数（uint64）+ 字节每秒（double）」命名，不在两套系统造第二套字段名。进程 `cpu_percent` 为 **gopsutil 语义的进程启动以来平均 CPU%**（CPU 总时间/存活时长），非瞬时速率（瞬时需每进程两样本跟踪，本项不引入）。

## 4. 测试证据

- `core/metrics/processes_test.go`：`TestCounterRate`（6 分支，含计数器回退/零间隔）、`TestCollector_CollectProcessAndPortFaces`（两面对非 nil、端口协议合法、pid 合法；无枚举权限时两面对空不造假）、`TestCollector_NetworkRateNonNegative`（两扫速率非负）。
- `internal/platform/registry/metrics_hourly_test.go`：`TestMetricsStore_RollupHourly_AggregatesCompletedHour`（2 行均值 CPU40/内存15/进程4/端口3/网络150；旧小时行排除；重roll幂等）、`TestMetricsStore_PruneHourly_RemovesBeyondRetention`。
- `internal/platform/registry/metrics_enrichment_test.go`：persistToDB 落三面列、getFromDB 重建、空面 NULL。
- `internal/svc/migrations_metrics_enrichment_test.go`：存量形态（手工 CREATE TABLE 无三面列）→ AddColumn + 建聚合表；存量行保持 NULL；缺表跳过（fanout 不建空壳）；模型列集 round-trip。
- `internal/api/ops/metrics_history_window_test.go`：raw 面重建三 DTO；`window=hourly` 返回聚合；越权 agent fail-closed。
- 迁移三处同步：`migrations.go`（注册 + 0051 函数 + 头清单注释）、`migrate.go` `MinimumRequiredVersion=51`、`migrate_test.go` 合成清单 + 版本断言。
- 全量：`go build ./...`、`go test ./core/metrics/... ./internal/platform/registry/... ./internal/api/ops/... ./internal/svc/... ./internal/db/migrate/... ./internal/app/agent/... -count=1` 全绿；`pnpm --dir web run tsc` 0 错误。

## 5. 已知边界（诚实清单）

- **真机抽样验收（todo.md 验收：Linux+Windows 各一台、与 top/任务管理器对账 <10%）不在本环境可运行**——需要真实部署机与人工抽样对账。本环境以单元测试 + 真实采集冒烟（collect 断言非负/字段合法）覆盖采集正确性，数值精度验收登记为部署后的人工对账项。
- **Windows 采样未实测**：`core/metrics` 走 gopsutil 跨平台路径，Windows 侧未在本环境运行；`net.Connections` 状态串在 Windows 上为平台映射，`LISTEN` 判定按 `strings.EqualFold` 容错。
- **UDP「监听」语义**：无 LISTEN 状态，定义为「绑定本地端口且无对端」（`Raddr.Port==0`）；不保证与 netstat 逐行一致（UDP 无连接语义，按绑定面归并）。
- **每进程瞬时 CPU% 未采集**：见 §3（gopsutil 启动以来平均语义）；OPS-010 验收只要求进程清单名称/pid/内存/监听端口。
- **进程/端口清单只显示最新一报**（面板不绘制历史进程曲线——进程集合随时间变化，历史曲线语义未定义，登记为后续大屏（OPS-011）候选）。
- **聚合面仅投影 CPU/内存/磁盘/网络速率均值**：进程/端口计数均值落 `agent_metrics_hourly` 但 history API 聚合面只重建资源曲线所需字段（进程/端口列表不随聚合面返回——90 天进程历史无意义）。

## 6. 部署说明

- 迁移 0051 在 server 启动时自动跑（goose）：`agent_metrics_history` 加三列、建 `agent_metrics_hourly`。存量行三面保持 NULL（历史窗口无该数据，不回填）。
- 老版本 agent 上报无三面字段：server 端 `persistToDB` 判空不落列，读侧重建为空——前后向兼容，无需 agent 与 server 同时升级。
- 默认间隔 30s→15s 只影响新部署；已部署 agent 保留其配置值（YAML `ops.metricsInterval` 或 `metrics_interval` 兼容键）。
