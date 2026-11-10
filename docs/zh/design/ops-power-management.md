---
title: 运维电源管理设计
---

# 运维电源管理设计

日期：2026-11-11。状态：**按本文档实现**（OPS-COMPLETE-001，本地 mock 验证；真实 BMC/WOL/云硬件验证待环境）。本页设计运维面的远程机器电源生命周期能力：开机、关机、重启与电源状态查询，全部接入现有审批链。

## 1. 目标与范围

运维面已有 provision（terraform-exec 云开通/销毁）、batchops、runbooks、supervisor 与带 OTP step-up 的审批服务。缺的是**物理机/远程机器电源生命周期**：把机器开机、关机、重启，以及查询电源状态。核心场景是自动恢复——supervisor 发现服务宕机，原因是机器被关了；平台必须能开机、等就绪、再让 supervisor 把服务拉起。

平台不自建第二套审批体系。电源操作完全复用 approvals store、OTP step-up、两人规则与哈希链审计，与 provision ticket 模式同构。

## 2. 本设计依托的现状事实

- **Provision Run 模式**（`internal/ops/provision`）：`Run{ID, Request, State, ApprovalID, Error, CreatedAt, UpdatedAt}`，状态 `pending → applying → waiting_register → adopted / failed`，外加销毁子状态机 `destroy_pending → destroying → destroyed / destroy_failed`。编排器先持久化 run，再建审批记录并回填 `run.ApprovalID`；续行前重查批准态（`ErrRunNotApproved` → 409）。
- **审批门抽象**（`internal/svc/provision_wiring.go`）：子系统经 `approvals.Store.Create(&Approval{FunctionID, GameID, Env, Actor, Mode, Route, Metadata})` 建审批、经 `State == "approved"` 读结果。provision 用 `FunctionID: "ops.provision"`、`Mode: "provision"`、`Route: "/ops/provision"`、`Metadata: {provisionRunId}`。powerops 同构：`FunctionID: "ops.power.*"`、`Mode: "powerops"`、`Metadata: {powerRequestId, targets, action}`。
- **OTP step-up**（`internal/api/approval/stepup_otp.go`）：批准时风险按审批记录的 `FunctionID` 经 `function.ResolvePolicyRisk` 解析；解析故障 fail-closed 到 high。high/danger 审批在 `security.approvalStepUpOtp` 开启（默认开）时必须 TOTP；稳定错误码 `otp_required` / `otp_invalid` / `otp_not_enrolled`。
- **两人规则**：审批人不能批准自己的申请（`self_approval_forbidden`），显式逃生门 `approval.allowSelfApprove`；拦截与放行均落哈希链审计。
- **Agent 执行通道**：`internal/logic/ops.GetAgentOpsClient().GetClient(ctx, agentID)` 提供 `ExecuteCommand` / `PushFile`（opsv1 wire），batchops 已在用。`POST /ops/nodes/:nodeId/restart` 经控制通道重启的是 **agent 进程**（`MsgRestartProcessRequest`）——不是主机重启，也无审批门。电源操作两者都要：带内 agent 通道做优雅主机重启，其余场景（agent 已关机）走带外路径。
- **机器清单**：`nodes` 表（`NodeID`、`Meta datatypes.JSONMap`）是现成机器模型。powerops 把每台机器的电源配置挂在 `nodes.meta.power` 下，不新增清单表（清单本身无需新编号迁移；电源**请求**有自己的表，见 §6）。
- **批量语义**（`internal/ops/batchop` + `internal/api/ops/batchops.go`）：registry 选目标、dry-run 默认、逐目标结果聚合、`context.WithoutCancel` 后台执行（客户端断开不得把批次卡在 Running，CR-FIX-22）。powerops 复用同一聚合与后台执行纪律。

## 3. 能力矩阵

机器类型 × 电源动作，含协议与凭据来源：

| 机器类别                         | 开机                           | 优雅关机                                | 强制关机                            | 重启                                    | 状态查询                          | 协议 / 凭据                                                                                                      |
| -------------------------------- | ------------------------------ | --------------------------------------- | ----------------------------------- | --------------------------------------- | --------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| 带 IPMI/BMC 的物理机             | ✅ `ipmitool chassis power on` | ✅ `chassis power soft`                 | ✅ `chassis power off`              | ✅ `chassis power cycle` / `reset`      | ✅ `chassis power status`         | ipmitool CLI 走 BMC 网络；BMC 凭据来自 server 配置（`powerops.ipmi.*`），配置了密码箱时用户名/密码从凭据存储解析 |
| 局域网 Wake-on-LAN 机器          | ✅ 魔术包（UDP 广播 9 端口）   | ❌                                      | ❌                                  | ❌                                      | ❌（间接：唤醒后 agent 会话上线） | WOL 单向；唤醒确认 = agent 重连                                                                                  |
| 云实例（provision 托管或已登记） | ✅ provider start              | ✅ provider stop（provider 支持时优雅） | ⚠️ provider 强停语义不一            | ✅ provider reboot                      | ✅ provider status                | 经配置声明的命令模板（`powerops.cloud.*`）调 provider CLI/API；云凭据只存 server 配置，绝不进 API 入参           |
| Agent 所在机（agent 会话在线）   | 不适用（已在运行）             | ✅ agent 通道优雅关机                   | ❌（关机中途 agent 即死；改走 BMC） | ✅ agent 通道优雅重启 + 验证 agent 重连 | ✅ agent 会话 + system-info 存活  | agent-ops `ExecuteCommand`（如 `systemctl reboot` / `shutdown -h now`），受 agent 侧命令策略约束                 |

图例：✅ 一等支持；⚠️ 支持但有明确保留；❌ 协议表达不了该动作。

矩阵蕴含的要点：

- WOL 与 IPMI 是**带外**通道：目标机 agent 已关机时依然可用——这正是自动开机场景的价值。
- Agent 通道优雅重启是**带内**通道：随机器一起死，验证步骤是「N 秒内 agent 会话重连」，且只用于预期会回来的计划内维护。
- 云强制关机语义因 provider 而异；powerops 如实上报 provider 自身的结果，不伪装统一语义。

## 4. 需求场景

1. **自动开机恢复**（主场景）：supervisor/healthprobe 观察到服务宕机；诊断表明机器关机（agent 会话缺席 + BMC 状态 off）。运维（或 runbook 步骤）提交 `POST /ops/power`、动作 `power_on`；批准后 BMC/WOL provider 执行；验证轮询 BMC 状态 / agent 重连；done 态记录谁/何时/如何。
2. **维护窗口批量关机**：运维选 N 台机器、动作 `power_off`（优雅）。批量语义：一个请求、逐机结果行、部分失败不掩盖成功；agent 在线的走带内优雅关机，无 agent 的走 BMC soft-off。
3. **异常重启**：挂死的机器执行 `reboot`；按 §5 分级审批；验证 = 机器在配置窗口内回来（BMC 可达 / agent 重连），否则请求落 `failed` 并带最后观测状态——机器没回来绝不谎报 done。

## 5. 审批策略矩阵

动作映射风险级；审批 step-up 解析的正是请求的 `FunctionID`（`ops.power.power_on` / `ops.power.power_off` / `ops.power.force_off` / `ops.power.reboot` / `ops.power.status`）：

| 动作                                | 风险                                       | 审批流                                                                                                 | OTP                                  |
| ----------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------ | ------------------------------------ |
| `power_on`                          | low                                        | 单审批、默认工作流                                                                                     | step-up 不强制（仅审计）             |
| `reboot`（优雅）                    | medium                                     | 单审批                                                                                                 | 按 step-up 策略（medium 非强制 OTP） |
| `power_off`（优雅）                 | medium                                     | 单审批                                                                                                 | 按 step-up 策略                      |
| `force_off`                         | high                                       | 单审批                                                                                                 | **强制 OTP**（high ⇒ step-up）       |
| 批量 `reboot`/`power_off`（多目标） | 取成员动作最高风险；批量在审批元数据中声明 | 一张审批覆盖声明的目标清单（目标在请求时快照进审批元数据——批准范围不可被静默偷换）；逐机结果落电源请求 | 任一成员为 high 即 OTP               |

谁发起/谁批：就是现有审批模型——发起人是已认证 admin（记录为 `Actor`），审批人是申请人之外的任意 admin（两人规则），同样的 `approval.allowSelfApprove` 逃生门与审计留痕。`power.status` 是查询：需认证 + agent/机器作用域，但不需要审批。

fail-closed 对齐：策略管理器解析不出 `ops.power.*` 的风险时，审批 step-up 按现有行为视作 high——未配置的部署不会丢掉危险动作的 OTP 门。

## 6. 请求状态机

一个电源请求覆盖一个动作 + 目标机器列表。状态沿用 provision Run 模式：

```text
pending ──(approved)──▶ executing ──▶ verifying ──▶ done
   │                      │             │
   └──(rejected)──▶ rejected          ├──▶ failed        （任一机器验证失败）
                                      └──▶ partial      （部分 done 部分失败）
```

- `pending`：请求落库（请求表行，含目标快照 + 动作 + 原因 + 发起人）；建审批记录并回填（`ApprovalID`）。provision 先例：先持久化、再建审批、再回填。
- `approved → executing`：审批通过；开始执行。逐机经 provider 尝试；每台机器行记录尝试时间、原始结果、错误。
- `executing → verifying`：对有可观测效果的动作（power_on、reboot、power_off）轮询确认——BMC `power status`、云 provider 状态、或 agent 会话重连——直到截止（`powerops.verifyTimeout`，默认 120s，逐次退避）。
- `done` / `partial` / `failed`：终态带逐机结果；`partial` 存在的理由：批量电源是唯一必须区分「部分机器回来了」与「全部回来了」的地方。
- 拒绝路径：审批拒绝 ⇒ 请求 `rejected`，什么都不执行。
- 执行跑在 `context.WithoutCancel` 派生的后台 context（CR-FIX-22 纪律）：HTTP 客户端断开不得把请求卡在 `executing`。

存储：新表 `power_requests`（头）+ `power_request_targets`（逐机行），按编号迁移契约落地（`internal/svc/migrations.go` 注册 + `MinimumRequiredVersion` bump + 合成迁移测试同步，与模型同 PR）。机器清单本身留在 `nodes.meta.power`——清单无需迁移。

## 7. 包与 API 面

- `internal/powerops`：`Provider` 接口 `PowerOn / PowerOff / Reboot / Status`（context + machine + options → result），实现 `ipmi`（ipmitool CLI 包装；命令拼装用假二进制覆盖测试）、`wol`（UDP 魔术包）、`cloud`（配置声明命令模板）、`agent`（经 agent-ops 客户端带内优雅重启）。`RequestStore` + 状态机 + 验证循环也在本包，HTTP 层保持 DTO 映射薄壳（provision 分层先例）。
- `internal/api/ops/powerops.go`：`POST /ops/power`（目标 + 动作 + 原因 → pending 请求 + 审批 id）、`POST /ops/power/:id/resume`（审批通过 → 执行，provision-resume 先例）、`GET /ops/power/:id`（状态 + 逐机结果）、`GET /ops/power`（列表）。挂在现有 `/ops` 组下带 feature 开关；provider 未配置 ⇒ 如实 `503 powerops_not_configured`，绝不静默空转。
- 机器登记：`PUT /ops/agents/:agentId/power-config` 写 `nodes.meta.power`（provider 类型、BMC 地址、MAC、云实例引用），agent 作用域校验与其他 per-agent 端点同口径。
- 审计：`powerops.request` / `powerops.execute` / `powerops.verify` 事件经 `recordOpsAudit`，与 provision destroy 同款。

## 8. 依赖

- **BMC 凭据**：`powerops.ipmi.*` server 配置（地址模板、用户名、密码、可选 `passwordEnv` 环境变量引用，沿用 provision bootstrap-token 模式——提交进仓库的配置绝不携带密钥）。仓库今天没有 `vault://` 集成；本设计把 env 引用模式定为凭据来源，vault 后端留作后续工作（诚实边界）。
- **执行通道**：带内动作走现成 agent-ops 客户端（无需新 agent 插件）；带外动作在 **server 宿主机**执行（ipmitool / UDP 广播 / 云 CLI）。本设计不要求 agent 可达 BMC 网络——那会把 agent 部署耦合进管理网。若部署的 server 宿主机到不了某台 BMC，该机即电源不可达，请求如实记录（诚实失败，不是挂死）。
- **ipmitool 二进制**：期望在 server 宿主机配置路径存在；二进制缺失 ⇒ provider 在提交时报 `powerops_ipmi_unavailable`，不在批准后才失败。

## 9. 不做什么与诚实边界

- **CI 内无真实 BMC/WOL/云验证**：没有硬件。provider 行为全部由 mock + 假 ipmitool 二进制（命令拼装）覆盖；**真实 BMC/WOL 验证待有硬件的环境**——如实记录，不含糊。
- **server 无 IPMI 网络可达性 ⇒ 机器不可达**：平台上报 `unreachable`/`failed`；不伪装，也不借任意 agent 跳板转发电源。
- **WOL 只能唤醒**：无优雅关机、无状态查询；确认靠 agent 重连，而这要求机器的 agent 配置自启——属平台外部部署前提。
- **云动作是命令模板驱动**，不是深度 provider SDK 集成：`powerops.cloud.*` 声明逐动作命令行（如 provider CLI 调用）。provision 的 terraform workspace 管创建/销毁，不管实例 start/stop，powerops 不假装复用它；未来 provider SDK 集成会替换模板层。
- **无人值守自动开机循环不做**：平台不自动批准、不自动重试电源动作。supervisor 驱动的场景同样从人工批准的请求进入；自动化可以**起草**请求，绝不**批准**。
- **agent 所在机拒绝 force-off**（通道随宿主一起死）：API 在机器登记了带外 provider 时映射过去；没有则如实失败。

## 10. 验证计划（本地）

- 单测：provider mock（状态流转、验证循环、部分失败聚合）、状态机（审批拒绝 → pending→rejected；续行必须已批准）、ipmitool 命令拼装对假二进制、WOL 包字节。
- Handler：提交 → pending + 审批 id；未批准 resume → 409；拒绝路径零执行；provider 未配置 → 503。
- 真机验收（对真实 BMC 开关机/查状态、WOL 唤醒挂起机器、云 start/stop）——**待环境**，已在上文边界中记录。
