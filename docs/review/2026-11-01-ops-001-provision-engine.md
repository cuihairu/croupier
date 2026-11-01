# OPS-001 云资源供给引擎（Terraform/OpenTofu 驱动，2026-11-01）

日期：2026-11-01。交付：`internal/ops/provision` 云资源供给引擎——用 hashicorp terraform-exec 驱动 tofu/terraform，provider 模板内嵌（阿里云/腾讯云/AWS/Hetzner 起步），一键扩容闭环 = 选厂商/地域/规格 → create → cloud-init 注入 croupier agent（bootstrap token + hub 地址）→ 自注册 → 网络审批策略（个人直通 / 团队工单）→ 纳管；HTTP 面 `/ops/provision*`。

## 1. 交付前现状与缺口

既有（OPS-001 前）：无任何云资源供给代码——无供给引擎、无 `/ops/provision` 端点、无运行记录表。agent 纳管只覆盖「手动装好 agent 拨入」的路径，云上新建主机如何装上 agent 并自注册入作用域没有闭环。

本轮补的缺口：

| 缺口                | 补法                                                                                                                            |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| 无供给引擎          | `internal/ops/provision`：编排（validate → render → apply → waitRegister → adopt）+ 四 provider 模板 + cloud-init               |
| 无运行记录          | 迁移 0052 建 `provision_runs`；模型 `ProvisionRunModel` CRUD；状态机 pending/applying/waiting_register/adopted/failed/destroyed |
| 无执行抽象          | `Runner` 接口（生产 `TfexecRunner` 走 terraform-exec，测试 `FakeRunner` 同一代码路径）                                          |
| 无 agent 自注册关联 | 等待注册按供给标签（`croupier-provisioned: <runID>`）匹配在线 agent 会话（`AgentRegistry` 适配 reg.Store）                      |
| 无审批策略接入      | `ApprovalGate` 适配 approvals.Store：ticket 模式建审批记录（关联 runID），批准后 Resume 执行                                    |
| 无 HTTP 面          | `/ops/provision` 提交/列表/详情/续行/销毁五端点；未配置时如实报 503 `provision_not_configured`                                  |
| 无敏感字段隔离      | bootstrap token 不入库（runToModel 只落非敏感快照）、不落工作目录 main.tf、`json:"-"` 不出现在 API 响应                         |

## 2. 实现要点

- **编排**（`orchestrator.go`）：`Submit`（direct 同步全链 / ticket 建审批记录返 pending）、`Create`（全闭环）、`Resume`/`ResumeAsync`（pending+批准双检查 → 状态推进 applying → apply）、`Destroy`（terraform destroy 原语，无等注册段）、`SubmitAsync`（HTTP 入口不阻塞：persist applying → 后台 goroutine 全链）。`applyMu` 串行化 apply 段并配合 pending→applying 状态闸防并发双跑；异步 apply 用独立 context + 30m 封顶（`asyncApplyBudget`）防 terraform 悬挂泄漏 goroutine。
- **模板**（`template.go`）：四 provider 的 main.tf 内嵌（变量块统一 `var.region/instance_type/image/disk_size/user_data/count/name_prefix`），resource 各按 provider 命名（`alicloud_instance`/`tencentcloud_instance`/`aws_instance`/`hcloud_server`），输出 `instance_id`/`public_ip`；凭据全部走 provider 环境变量，不入模板。
- **cloud-init**（`renderCloudInit`）：写 `/etc/croupier/agent.yaml`（hubAddr + bootstrapToken + 供给标签），下载安装 agent（`AgentInstallURL`，空则只写配置），runcmd 支持 `CloudInitExtra` 追加；经 `base64encode` 注入 user_data，内容不落 terraform state 与运行表。
- **自注册关联**：agent 的 AgentID 由 agent 自身决定、不与 terraform `instance_id` 等同，故以供给运行专属标签（`croupier-provisioned: runID`）关联——等待注册按标签匹配，而非按 instance_id 猜测。
- **HTTP 面**（`internal/api/ops/provision.go`）：提交/续行异步（202 + run）；列表/详情同步；销毁同步（terraform destroy 原语）。契约键 lowerCamelCase；错误映射：引擎未装配 503 `provision_not_configured`、token/hub 缺失 503（server 侧装配不完整，非客户端输入错误）、校验失败 422 `validation_failed`、运行不存在 404、未批准/非 pending 续行 409 `provision_state_conflict`。`requestedBy` 取认证上下文 username；bootstrap token 提交时经 `provision.bootstrapTokenEnv` 环境变量读取（可轮换，不入配置明文）。
- **装配**（`service_context.go`）：`provision.enabled=false` 不装配（端点如实报 503）；enabled 时构造 Orchestrator（DBStore + ApprovalGate + AgentRegistry 适配器，`provision_wiring.go`），`RegisterTimeout`/`WorkDir`/`InstallURL`/`BinaryPath` 来自配置。
- **迁移**（0052）：新表无存量约束名漂移（0023 教训），HasTable 检查后 CreateTable，幂等、缺表跳过；敏感字段无列（表只落非敏感快照）。

## 3. 测试证据

- `internal/ops/provision/orchestrator_test.go`：`TestSubmit_Direct_RunsFullLoopAndAdopts`（全链 + apply vars 注入 name_prefix/disk_size/user_data + 供给标签一致 + main.tf 无 token）、`TestSubmit_Ticket_PendingThenResumeAdopts`（未批准 Resume 拒、批准后全链）、`TestCreate_ApplyFailure_StateFailed`、`TestCreate_NoRegister_StateFailed`、`TestDestroy_Transitions`、`TestSubmitAsync_Direct_ReturnsApplyingThenAdopts`、`TestSubmitAsync_Ticket_ReturnsPendingNoApply`、`TestResumeAsync_NotApproved_Rejected`、`TestResumeAsync_DoubleResume_SecondRejected`（状态闸防双跑）、`TestValidate_Matrix`（9 分支）。
- `internal/ops/provision/template_test.go`：四 provider 模板渲染真值面（provider/resource/user_data/instance_id/var.name_prefix）、cloud-init 含 token+hub、无 InstallURL 跳过下载、阿里云 base64encode、未知 provider 报错。
- `internal/svc/migrations_provision_test.go`：0052 建表 + 幂等 + 模型列集 round-trip；注册表含 0052（`TestRegisterSvcMigrations_ContainsProvisionRuns`，防 registerSvcMigrations 漏配）。
- `internal/api/ops/provision_test.go`：未配置五端点 503 `provision_not_configured`；direct 提交 202 + applying + 后台终态 failed（无注册）；校验失败 422；token 缺失 503；未知运行 404；列表 200；未批准续行 409；销毁同步推进 destroyed。
- 迁移三处同步：`migrations.go`（注册 + 0052 函数 + 头清单注释）、`migrate.go` `MinimumRequiredVersion=52`、`migrate_test.go` 合成清单 + 版本断言。
- 全量：`go build ./...`；`go test ./internal/ops/provision/... ./internal/api/ops/... ./internal/svc/... ./internal/model/... ./internal/db/migrate/... ./internal/handler/... -count=1` 全绿（含 `-race` 于 provision 包）；`go vet` 通过。

## 4. 已知边界（诚实清单）

- **真实 provider 集成（apply 落地）不在本环境可运行**——需要云凭据（`ALICLOUD_ACCESS_KEY_ID`/`TENCENTCLOUD_SECRET_ID`/`AWS_ACCESS_KEY_ID`/`HCLOUD_TOKEN`）+ tofu/terraform 二进制。本环境以模板渲染真值面 + FakeRunner 全链覆盖编排正确性；真实 apply 登记为部署后人工验收项（todo.md 交付「集成测试用 mock cloud + 真实 provider 各一」拆为此项 + mock 面由 FakeRunner 覆盖）。
- **cloud-resource-requests.md 的三角色目录/申请单流未交付**：该设计纸（2026-10-15「document first, no code」）的 `cloud_resource_catalog`/`cloud_resource_requests` 表、`cloud.request.*` 四权限点、执行回执（receipt）模型与 expiry→recycle 长尾**不在本轮**——本轮交付的是 OPS-001 todo 的「引擎 + 一键扩容闭环」；设计纸的目录/申请单面保持未实现状态，其 §8.7 的「v1 manual receipts、automation 另项目」与本引擎的关系在设计纸 §「Implementation status」标注。
- **异步 apply 后台 goroutine 与优雅停机**：`SubmitAsync`/`ResumeAsync` 在独立 context 跑 apply 全链，server 关闭时不等待在飞 apply（terraform 继续至其 30m budget 或完成）；run 终态由后台落库，跨进程续查见最后持久化状态。
- **多实例不互斥**：`applyMu` 仅在单 server 进程内有效；供给引擎应仅在单一 server 实例启用（多实例各自跑 apply 引用同一工作目录会互相干扰）。
- **bootstrap token 单值**：一个环境变量供所有供给请求使用，无每 run 独立 token（agent 注册鉴权的细粒度隔离登记为后续）。
- **destroy 同步阻塞**：terraform destroy 原语在 HTTP 请求内执行（无等注册段、典型时长分钟级内）；超长 destroy 受 server 写超时约束。
- **Windows/桌面供给**：供给目标是云上 Linux 主机（cloud-init），Windows 主机供给不在范围。

## 5. 部署说明

- 迁移 0052 在 server 启动时自动跑（goose）：建 `provision_runs`。新表无存量数据兼容问题。
- 配置段 `provision`（`configs/*.yaml` 示例）：`enabled`（缺省 false=不装配，端点 503）、`binaryPath`（默认 tofu）、`workDir`（默认 `data/provision`）、`hubAddr`（必填，agent 拨入地址）、`bootstrapTokenEnv`（默认 `CROUPIER_PROVISION_TOKEN`）、`agentInstallUrl`、`registerTimeoutSec`（默认 300）。
- 云 provider 凭据不落配置：走各自环境变量（见 §4）。
