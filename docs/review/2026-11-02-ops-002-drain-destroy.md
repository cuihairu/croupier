# OPS-002 缩容与排水（drain → destroy），2026-11-02

日期：2026-11-02。交付：供给运行的缩容排水闭环——销毁前置三闸（数据盘护栏 / 排空门 / 无在途任务）+ 护栏端点 + 同一 Terraform 状态销毁（防漂移）。todo 验收三条「无负载才允许销毁（保守版）/ 数据盘护栏（持久化标记禁止自动销毁）/ 销毁走同一 Terraform 状态防漂移」逐条对应 §2。

## 1. 交付前现状与缺口

OPS-001 交付了 `/ops/provision` 五端点，其中 `DELETE /ops/provision/:runId` 是裸销毁：不检查目标机是否仍有在途任务、不检查是否已排水、没有数据盘护栏——一条 DELETE 就能把承载负载的机器连盘销毁。节点的 drain 门（`POST /ops/nodes/:nodeId/drain`，停新路由等在途）已存在，但与供给销毁没有衔接。

| 缺口                 | 补法                                                                                                          |
| -------------------- | ------------------------------------------------------------------------------------------------------------- |
| 裸销毁（无负载检查） | 销毁前置三闸（`provisionDestroyGuards`）：护栏 → 排空 → 在途任务归零                                          |
| 无数据盘护栏         | `provision_runs.destroy_protected` 持久化列（迁移 0053）+ `PUT /ops/provision/:runId/protection` 端点         |
| drain 门与销毁无衔接 | 排空门复用 `AgentSession.Draining`（drain 停新路由的注册表路由门），销毁前要求运行名下在线 agent 全部已 drain |
| 销毁防漂移           | 已有（OPS-001 `Orchestrator.Destroy` 复用 WorkDir/main.tf 同一 terraform 状态）；本轮补守卫后语义完整         |

## 2. 实现要点

- **销毁前置三闸**（`internal/api/ops/provision.go` `provisionDestroyGuards`，顺序即运维处置顺序）：
  1. **数据盘护栏**：`run.DestroyProtected=true` → 409 `provision_destroy_protected`（持久化标记，禁止自动销毁）。
  2. **排空门**：运行名下所有**在线** agent 会话（`RegistryStore`）必须 `Draining=true`，否则 409 `provision_agents_not_drained`。先于在途检查——未排空的机新任务仍在进来，等在途归零无意义；排空（drain 停新路由）+ 在途归零合并消除销毁竞态。离线会话无路由、不计。
  3. **无负载才允许销毁**（保守版 drain）：`TaskRunModel.CountActiveByAgents`（新查询，按 agent 集合 × 活跃状态集 `[queued, dispatching, running, cancel_requested]` 计数）> 0 → 409 `provision_tasks_inflight`。计数经 `GameScopeContext`（`ServiceContext` 新导出包装，含 per-game DB 覆盖）解析到运行作用域的 game 库。数据源不可用（DB 缺失/查询失败）时 fail-closed 500 `provision_load_unverifiable`，绝不放行。
- **护栏端点**：`PUT /ops/provision/:runId/protection`，body `{"destroyProtected": bool}`；仅 `adopted` 状态可改（其他状态 409 `provision_state_conflict`）；成功落 audit `provision.protect` / `provision.unprotect`。
- **审计**：销毁成功落 `provision.destroy`（provider/agents/protected 快照）；新事件常量 `provision.destroy`/`provision.protect`/`provision.unprotect`（`internal/audit/audit.go`）。
- **护栏映射**：`provision.Run.DestroyProtected` ↔ `model.ProvisionRun.DestroyProtected`（`provision_wiring.go` 双向映射）；API 响应契约键 `destroyProtected`（lowerCamelCase）。
- **迁移 0053**：`provision_runs.destroy_protected` 列，`HasColumn`+`AddColumn` 补列模式（0025/0043/0044/0049/0051 先例），幂等、缺表跳过（fanout 重放到无该表的库不报错、不建空壳）；存量行默认 `false`=未护栏，行为与现状一致。三处同步：`internal/svc/migrations.go`（注册 + 函数 + 头清单）、`internal/db/migrate/migrate.go` `MinimumRequiredVersion=53`、`internal/db/migrate/migrate_test.go` 合成清单。
- **销毁防漂移（既有，语义补全）**：`Orchestrator.Destroy` 沿用 OPS-001 设计——从 `WorkDir/<runID>/main.tf` 同一 terraform 状态执行 destroy 原语，云侧资源只经该状态回收，无第二套删除路径。

## 3. 测试证据

- `internal/model/task_model_test.go` `TestTaskRunModel_CountActiveByAgents`：跨 agent 累计活跃（terminal/unknown 不计）、单 agent、无活跃 agent→0、空 agentIDs no-op（不触库）、空状态集 no-op。
- `internal/svc/migrations_provision_test.go`：0053 补列 + 幂等 + **缺表跳过**（无 provision_runs 的库不建空壳）+ **存量形态复刻**（0052 建表后 DROP 该列模拟 0052 时代老表，再跑 0053 补列——迁移契约要求的存量形态，非 AutoMigrate 新表）+ 存量行默认 false + true round-trip；注册表含 0053（`TestRegisterSvcMigrations_ContainsProvisionDestroyProtection`）。
- `internal/api/ops/provision_test.go` 新增：`TestProvisionDestroy_Protected_409`（护栏拒绝）、`TestProvisionDestroy_InflightTasks_409`（在途任务拒绝，sqlite 种子 task_runs）、`TestProvisionDestroy_NotDrained_409`（在线未 drain 拒绝，真实 reg.Store 会话）、`TestProvisionDestroy_DrainedClean_200`（已 drain + 无在途 → destroyed）、`TestProvisionProtection_Toggle`（开/关护栏 + 持久化读回）、`TestProvisionProtection_NonAdopted_409`（destroyed 状态拒绝改护栏）。既有 `TestProvisionDestroy_Sync_Transitions`（无 AgentIDs 的运行，三闸全过）回归不变。
- 迁移三处同步核验：`go test ./internal/db/migrate/ -count=1` 全绿（`MinimumRequiredVersion=53` + 合成清单 0053 probe）。
- 全量：`go build ./...`；`go vet ./internal/api/ops/... ./internal/model/... ./internal/svc/... ./internal/ops/provision/... ./internal/audit/...` 过；`go test ./internal/api/ops/ ./internal/model/ ./internal/svc/ ./internal/ops/provision/ ./internal/handler/ ./internal/audit/ -count=1` 全绿；新守卫用例含 `-race` 复跑通过。

## 4. 已知边界（诚实清单）

- **「无负载」的口径是在途任务（task_runs 活跃状态）而非账号负载**：todo 验收原文「drain=把目标机账号负载迁移走（依赖调度编排，先行用『无负载才允许销毁』保守版）」——账号分布/调度编排（账号负载视图）属后续调度编排项；本轮交付其保守版：在途任务归零 + drain 门停新路由。账号级负载迁移不做（无账号分布数据源，诚实缺位，不假装覆盖）。
- **护栏是运行级而非磁盘级**：`destroy_protected` 标记在供给运行上（一行=一次 count N 的扩容），保护整组实例；未建 per-磁盘/per-实例粒度（terraform 状态按 run 管理，per-实例 target 级销毁属后续）。
- **排空门只覆盖在线会话**：目标机 agent 离线（会话过期）时排空门不拦截（无路由可停）；此时在途任务闸仍生效——session-loss reaper 把断连 agent 的在途任务落 `unknown`（活跃状态集外）后才可销毁。**注意**：`unknown` 是「执行状态未知」，销毁一台任务状态 unknown 的机器等于放弃对账——护栏/审计留痕是唯一补偿，操作者应自行确认。
- **destroy 同步阻塞 + 失败即 500**：terraform destroy 原语在 HTTP 请求内执行（无等注册段）；失败返回原始错误（500 `internal_error`），运行状态不推进（可重试销毁）。
- **多实例不互斥**：守卫读取的 task 计数与 registry 会话是本实例视图；供给引擎仅应在单一 server 实例启用（OPS-001 已知边界延续）。

## 5. 部署说明

- 迁移 0053 在 server 启动时自动跑（goose）：`provision_runs` 补 `destroy_protected` 列，存量行默认 false（行为不变）。
- 运维处置序列（销毁一台供给机器）：`POST /ops/nodes/:agentId/drain`（逐台，停新路由）→ 等在途任务完成（`GET /api/v1/tasks?...` 观察）→ `DELETE /ops/provision/:runId`（三闸放行，同一 terraform 状态回收）。数据盘要保活的运行先 `PUT /ops/provision/:runId/protection {"destroyProtected":true}`。
