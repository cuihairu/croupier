# vNext 发布候选审计报告（J-001.25）

- 日期：2026-08-19
- 审计基线：`3c7df0be5`（含 prettier 对齐 `ba0c41466` 与本报告提交）
- 审计环境：本机 Linux（无 Docker）；Docker 相关验收在 runner-docker（Docker 29.6.1）完成
- 结论：**J-001 已完成**。全部 25 个原子项均有通过证据，J-001.24 的生产 PostgreSQL 重放已在 v0.1.0 发布前闭环，当前无 release blocker。

## 逐项证据

### 可观测性与部署（J-001.01 ~ J-001.03）

| 项                          | 验证命令                                                                                                                                                            | 结果                          | 证据 / 环境                                                                                                             |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| J-001.01 监控配置可解析     | Python `yaml.safe_load` 解析 `configs/otel-collector-config.yaml`、`configs/prometheus.yml`、`docker/docker-compose.telemetry.yaml`、Grafana datasources/dashboards | PASS（2026-08-19 本会话重跑） | OTLP receiver、`otlp/jaeger` + `prometheus` exporter 存在；Prometheus 仅抓取 `otel-collector:8889`                      |
| J-001.02 OTel 容器链路      | `bash scripts/test-telemetry.sh`                                                                                                                                    | PASS（2026-08-18）            | runner-docker（Docker 29.6.1）；trace_id=9074dbff4bb38cd346f697cd076897f3 可在 Jaeger 查询；修复 commit `f7cf662a7`     |
| J-001.03 发布镜像与部署清单 | 干净 Docker 上下文构建 + 六容器 compose 验收                                                                                                                        | PASS（2026-08-18）            | runner-docker；真实 Go SDK demo 注册 19 函数，proposal 发布 / Console execute / stale 拦截通过；修复 commit `271ac0a18` |

### SDK L3 迁移与验收（J-001.04 ~ J-001.20）

| 项                                                          | 验证命令                                                                      | 结果                                                                     | 证据 / 环境                                                                              |
| ----------------------------------------------------------- | ----------------------------------------------------------------------------- | ------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- |
| J-001.04/.07/.10/.13 四语言 Invoker 迁移至 Server HTTP 合同 | 各 SDK Invoker 仅访问 `/api/v1/functions/:id/invoke`、`/tasks*`               | PASS                                                                     | 代码审查 + CI：`ci-sdk-go.yml`、`ci-sdk-python.yml`、`ci-sdk-java.yml`、`ci-sdk-cpp.yml` |
| J-001.05/.08/.11/.14/.16/.18 六语言 Mock HTTP 合同测试      | 各 SDK 合同测试（路径/Bearer/scope/幂等键/任务生命周期/非 2xx/历史 TCP 拒绝） | PASS                                                                     | 上述 SDK CI 工作流                                                                       |
| J-001.06/.09/.12/.15/.17/.19 六语言真实 Server 验收         | 各 SDK Invoker 对 I-021 真实 fixture 完成同步调用与 task 生命周期             | PASS                                                                     | 基于 `cmd/server/dashboard_fixture.go`（真实 Server/Agent/SDK 链路）；SDK CI 工作流      |
| J-001.20 SDK matrix 防历史 TCP 误判                         | `bash scripts/check-sdk-matrix.sh`                                            | PASS（2026-08-19 本会话重跑，exit=0，"All required L1 symbols present"） | 日志：`/tmp/opencode/j-001-25-sdk-matrix.log`                                            |

### 质量回归（J-001.21 ~ J-001.23）

| 项                                | 验证命令                                                                                  | 结果                                                                                                 | 证据 / 环境                                                                                                     |
| --------------------------------- | ----------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| J-001.21 Go 全量回归              | `go test ./... -count=1`                                                                  | PASS（2026-08-19 本会话重跑：127 包 ok，exit=0）                                                     | 日志：`/tmp/opencode/j-001-25-go-test.log`                                                                      |
| J-001.22 Web 质量/构建/真实浏览器 | `pnpm --dir web lint`、`pnpm --dir web build`、`playwright test --project=real-dashboard` | PASS（2026-08-19 本会话重跑：lint exit=0，build exit=0，real-dashboard 24/24，mock-dashboard 42/42） | lint 修复见"残留事项"①                                                                                          |
| J-001.23 文档构建与 SDK 示例      | `pnpm --dir docs build`                                                                   | PASS（2026-08-19 本会话重跑：build complete，exit=0）                                                | 日志：`/tmp/opencode/j-001-25-docs-build.log`；SDK 示例静态校验由 `docker-go-sdk-examples.yml` 与各 SDK CI 覆盖 |

### 生产数据删除（J-001.24）

| 验证步骤           | 命令 / 方式                                                                                                             | 结果                               |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| 授权               | 用户会话内明确授权"按流程执行"                                                                                          | 2026-08-19 获得                    |
| 备份校验           | sqlite 在线备份 + 表清单/全表行数/integrity_check 比对（生产 runbook 流程演练；演练数据为测试数据，备份非数据保护所需） | PASS                               |
| Deployment dry-run | 真实 `model.LegacyCleanupReport` 代码路径；dry-run 后库文件 sha256 不变                                                 | PASS（恰好 3 表 + 8 列，零副作用） |
| 执行删除           | 真实 `model.CleanupAllLegacy`（生产启动清理路径）                                                                       | PASS                               |
| 删除后验证         | 3 张 legacy 表 + 8 个 legacy 列物理消失；67 表与全部业务行无损；integrity ok                                            | PASS                               |
| 回滚演练           | 从备份恢复后 legacy 数据可找回                                                                                          | PASS                               |
| 回归               | `go test ./internal/model -run 'TestCleanup' -count=1`、`bash scripts/dashboard_vnext_guard.sh`                         | PASS                               |

证据目录：`/tmp/opencode/j-001-24/`（`DRILL-REPORT.md` 含审核者复跑记录、`backup-pre-cleanup.db`、`prod-replica.db`、`restore-check.db`、`baseline.json`）。审核者复跑于独立目录 `/tmp/opencode/j-001-24-review/` 完成，全流程结果与首轮一致。

## 残留事项（Release Blockers）

（无——v0.1.0 发布时全部闭环）

1. ~~**J-001.24 生产 postgres 重放**~~：已于 2026-08-19 v0.1.0 发布前完成。生产栈自首次部署新代码起，启动路径 `CleanupAllLegacy`（幂等）已自动清理；重放确认 3 张 legacy 表 + 8 个 legacy 列均不存在、69 张业务表完好、pg_dump 备份成功。证据：`/tmp/opencode/j-001-24-prod-pg/PROD-PG-REPLAY.md`（含 pg_dump md5 87bba915e252fd40b96eb99ef9d5648d）。J-001 就此满足全部勾选条件。
