---
title: 云资源申请与开通设计（申请-审批-执行-回收闭环）
---

# 云资源申请与开通设计

## 状态

- **设计单（用户 2026-10-15，先落档不写码）**。本轮只交付本设计文档，实现未动；实现随立项走编号迁移 + RBAC 权限点 + 面板三页的独立批次。
- 用户原话：「运维可以控制各个云服务商创建服务器，开发需要发起申请，许可之后运维去执行」「可以申请的资源也需要列出来」。
- 与既有能力的衔接以本仓实况 file:line 标注（§7），不另起一套资源管理。

## 0. 三类角色与硬边界

三类角色与职责（用户口径，不改）：

| 角色       | 职责                                  |
| ---------- | ------------------------------------- |
| **开发**   | 提出资源申请（选类型/规格/用途/时长） |
| **审批人** | 审批申请（通过/驳回 + 理由）          |
| **运维**   | 云服务商侧实际执行创建/回收           |

**设计红线（本功能核心安全约束）：开发与运维的权限必须分离。**

1. 开发**只能发起申请**，**不能直接调用云 API 创建资源**——云侧创建/回收动作的唯一触发方是运维。
2. 这条红线同时约束三层：**RBAC 权限点正交**（`cloud.request.create` 不含 `cloud.request.execute`，见 §6）、**前端入口分离**（开发只见申请与「我的申请」，不见执行台）、**API 层拦截**（无 execute 权限调用云执行端点一律 403，服务端强校验不信任前端隐藏）。
3. **凭据隔离**：云服务商密钥只存在运维侧，开发与审批链路不得接触任何云凭据（§5）。

## 1. 资源目录（可配置，运维维护）

可申请资源做成**数据化的资源目录**，不硬编码：目录是表（运维 CRUD），申请界面从目录 API 拉取，目录为空 = 开发端无可申请项（诚实空态，不伪装可选）。

### 1.1 类别闭集

`compute | database | storage | network | other`（闭集枚举，新类别=改枚举+迁移，不自由文本）：

| 类别     | 项目（目录条目示例，非代码）                       |
| -------- | -------------------------------------------------- |
| compute  | 云服务器 ECS/CVM/EC2（规格族、CPU/内存档位、镜像） |
| database | MySQL / PostgreSQL / Redis / MongoDB 托管实例      |
| storage  | 对象存储桶、云盘                                   |
| network  | 公网 IP/带宽、VPC、子网、安全组                    |
| other    | CDN、消息队列、日志服务（按需增删）                |

### 1.2 目录条目字段

| 字段             | 说明                                                                                                                  |
| ---------------- | --------------------------------------------------------------------------------------------------------------------- |
| name             | 目录项名称（唯一）                                                                                                    |
| category         | 类别闭集（上表）                                                                                                      |
| provider         | 云服务商（aliyun/tencent/aws/…，运维配置时给定）                                                                      |
| specOptions      | 规格范围：预设档位列表 + 边界说明（如 2c4g/4c8g/8c16g），申请人在范围内选                                             |
| defaultDuration  | 默认申请时长（临时资源用；永久资源忽略）                                                                              |
| approvalRequired | 是否需审批（低风险条目可免审——免审也留痕，见 §4 审计）                                                                |
| costEstimate     | 计费预估口径（静态月成本值或简单说明文案；预估仅供审批参考，以云侧账单为准——设计单未定公式化计费，记开放拍板项 §8.5） |
| project          | 归属项目                                                                                                              |
| lifecycle        | **temporary / permanent** 标记                                                                                        |

### 1.3 临时/永久与 atlas 对齐

`lifecycle` 标记与 atlas 注册生命周期概念对齐（atlas 是用户自有服务器状态/维护窗管理，本仓经 serverstatus Provider 消费，`internal/platform/serverstatus/atlas.go:45`）：

- **temporary**：申请时必填期望时长，执行回执带 `expiresAt`（§3）；到期进入提醒→回收流程（§4）。对应 atlas 语义里「注册后有下线时点的服务条目」。
- **permanent**：无到期，回收必须走显式回收申请（运维执行），不允许静默消失。对应 atlas 里「常驻服务条目」。
- 联动方向（v2 评估，§8.6）：资源 fulfilled/recycled 回执**单向写 atlas 服务器清单**（注册/下线），本仓不反向消费 atlas 做目录来源——目录的权威在本仓，atlas 只消费结果。

## 2. 申请单模型

新领域单据（不塞进既有 approvals 的 function 语义，理由见 §7.1）：

```
CloudResourceRequest
  requestNo        业务单号（可读、唯一）
  requester        申请人（admin 账号）
  catalogItemId    目录项引用
  catalogItemName  目录项名快照（目录项后续改名不影响历史单据）
  provider         云服务商（目录项快照）
  spec             规格选择（JSON：档位/数量/区域等，范围受目录 specOptions 约束）
  purpose          用途（必填，审批与审计依据）
  duration         期望时长（temporary 必填）
  project          归属项目（目录项快照，可按申请改写需审批确认）
  gameId / env     归属游戏域（资源视图进 game 作用域，见 §8.4 拍板项）
  refTicket        关联工单/项目（可选）
  state            状态机（下表）
  rejectReason     驳回理由（审批人必填）
  createdAt / updatedAt
```

### 2.1 状态机

```
pending ──approve──> approved ──execute(运维回执)──> fulfilled ──(temporary 到期)──> expiring ──recycle──> recycled
   │
   └──reject──> rejected ──(修改后重新提交)──> pending（新提交沿用原单号 + revision+1，驳回链留痕）
```

- `rejected` 允许修改后**重新提交**（同单 revision 递增，不删历史——驳回理由与修改轨迹全留痕）。
- `fulfilled` 后临时资源由到期时间驱动 `expiring`；回收动作完成落 `recycled`（回填回收时间与执行人）。
- `approved` 超时未执行（可配，缺省 7 天）提醒运维，不自动取消（运维执行是线下动作，催办不夺权）。

### 2.2 审批流

- **审批人判定**：RBAC 权限点 `cloud.request.approve`（角色配置，§6）。
- **级数**：v1 单级审批；两级审批留作配置项（`approvalLevel` 目录/全局配置，未拍板前不实现，§8.3）。
- **自批拦截**：申请人=审批人直接拒绝（复用既有 self-approval 拦截先例 `internal/api/approval/self_approval_test.go`）。
- 免审条目（`approvalRequired=false`）直接 `pending → approved`，但状态流仍走同一单据（approved by=system，留痕）。

## 3. 执行与回执（运维执行台）

**v1 执行=人工**：运维在云控制台实际创建，回到本仓执行台**回执**——本仓不代调云 API（云 API 自动化属后续可选批次，见 §8.7；若做，凭据与执行机约束见 §5）。

执行台（仅 `cloud.request.execute` 权限可见）列出 `approved` 待执行单：申请规格、用途、期望时长、项目。执行回执表单（提交即 `fulfilled`）：

| 回执字段             | 说明                                                   |
| -------------------- | ------------------------------------------------------ |
| provider / region    | 实际开通的云与区域（允许与申请有出入，回执为实况真值） |
| instanceId           | 云侧实例/资源 ID                                       |
| privateIp / publicIp | 网络信息（可空）                                       |
| expiresAt            | 到期时间（temporary 必填，驱动 §4 到期链）             |
| costEstimateActual   | 实际月成本预估（可选）                                 |
| operatorNote         | 运维备注（交付说明，如「已入安全组 sg-xxx」）          |

回执落单据闭环：申请（开发）→ 审批（审批人）→ 执行（运维）三段各自留痕，单据上三段操作人/时间齐全。

## 4. 回收与到期

- **临时资源到期提醒**：`expiresAt` 前 T-7d / T-1d 生成提醒（复用 Alert 生命周期面 `internal/model/alert.go:11`，source=cloud_resource，带 requestNo；不新造提醒系统）。申请人与运维同时可见。
- **回收动作**：仍是运维执行域（与创建同一权限点+同一执行台），回执 recycle 时间与执行人。**到期不自动删**——本仓不掌握云侧删除的爆炸半径，到期只升级提醒（escalated 状态可标人工处理，复用 Alert handleStatus 工作流）。
- **二次确认**：回收执行提交时前端二次确认（Popconfirm 先例）；**永久资源**回收必须走显式回收申请单（运维不可直接标 recycled 而无单据——单据即授权）。
- 回收后单据终态 `recycled`；atlas 侧对应服务器条目下线（v2 单向写，§1.3）。

## 5. 凭据安全

- **v1（人工执行）**：云控制台凭据根本不进本仓——凭据面天然隔离。
- **若未来接云 API 自动化**（可选批次，§8.7）：
  1. 云服务商密钥只存运维侧，走密码箱 `vault://` 引用（`internal/platform/vault/vault.go:44` Provider 接口、`:83` VaultRefPrefix 机器可扫描先例），密钥不落库、不进配置明文；
  2. 执行端点仅 `cloud.request.execute` 权限可达（运维），server 侧只持有 vault 引用不持明文，凭据在**运维指定的执行机**（agent 侧）消费；
  3. 开发与审批角色对云凭据的读写权限点**不存在**（不是禁用，是根本没有这个权限点）。
- 任何形态下，凭据值不出现在申请单/审批单/回执/审计载荷——审计只记「谁执行了」，不记「用什么执行」。

## 6. RBAC 权限点（红线的技术表达）

四个正交权限点，谁也不包含谁：

| 权限点                  | 角色   | 能做什么                     |
| ----------------------- | ------ | ---------------------------- |
| `cloud.catalog.manage`  | 运维   | 资源目录增删改               |
| `cloud.request.create`  | 开发   | 发起申请、重提、查看自己的单 |
| `cloud.request.approve` | 审批人 | 通过/驳回（自批拦截）        |
| `cloud.request.execute` | 运维   | 回执创建、执行回收           |

- 角色装配走既有 RBAC 角色体系（`internal/security/rbac/unified.go:286` 审批角色判定先例）。
- 服务端强校验：execute 端点、catalog 管理端点在 handler 入口查权限，403 返回统一错误契约（`{error, message, details}`，CLAUDE.md API 响应契约）。
- 前端按权限点渲染入口，但**不依赖前端隐藏做安全**（§0 红线第 2 条）。

## 7. 与现有受管资源的衔接（不另起一套）

| 衔接点                       | 位置                                                                                                                       | 关系                                                                                                                                                                        |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 新机器自动纳管（agent 注册） | `internal/server/control_handler.go:1041`（handleRegisterRequest）                                                         | 云上新建机器装 agent 出网注册即上 Ops/Nodes 节点视图；资源回执里的 instanceId/IP 与节点视图按 IP/主机互查（面板关联跳转），不复制节点数据                                   |
| 受管进程                     | `internal/app/agent/ops_config.go:89`（ManagedProcessConfig，kind: program/probe-adapter）                                 | 新机器上的游戏进程/sidecar 用既有 supervisor 生命周期（启停/监管/快照/调度/更新），资源域不重复造进程管理                                                                   |
| 健康探测目标                 | `core/healthprobe/targets.go:24`（TargetConfig）+ `internal/app/agent/probe_manager.go:304`（ApplyTargets 动态下发）       | 新机器/新依赖的拨测目标走既有 probe.targets 通道；server 派生动态目标链路已通，资源回执后追加 probe 目标是配置动作不是新机制                                                |
| 维护窗状态源                 | `internal/platform/serverstatus/provider.go:18`（Provider）+ `internal/platform/serverstatus/atlas.go:45`（AtlasProvider） | 临时资源到期/回收与 atlas 服务器条目生命周期概念对齐（§1.3）；维护窗 gate（`internal/svc/serverstatus_gate.go`）继续按既有判定消费                                          |
| 审批域                       | `internal/platform/approvals/store.go:16`（Approval，two-person record）                                                   | **模式复用、数据不复用**：资源申请是领域单据（目录引用/规格/回执/到期），与 function 调用审批（FunctionID/Route/HashKey 语义）不同域；自批拦截等行为规范照抄 approvals 先例 |
| 审计链                       | `internal/audit/audit.go:20`（ErrInvalidChain 哈希链校验）+ `internal/audit/chain/writer.go`                               | 资源域全链路事件走既有哈希链审计，不新造审计存储                                                                                                                            |
| 到期提醒                     | `internal/model/alert.go:11`（Alert 生命周期 firing/resolved + handleStatus 人工处理面）                                   | 到期提醒/升级复用 Alert 面（§4），不新造提醒系统                                                                                                                            |

### 7.1 为什么不把申请单塞进 approvals 表

approvals 平台的单据语义是「函数调用执行的两步授权」（store.go Approval：FunctionID/Route/TargetServiceID/HashKey/Payload），生命周期止于 approved/rejected+执行结果。资源申请单有**目录引用、规格、执行回执（instanceId/IP/expiresAt）、到期→回收长尾状态机**——硬塞会让 approvals 表长出十个可空列且两种状态机互相污染。设计取：**同一审批行为规范（two-person、自批拦截、驳回理由），不同领域表**；后续若出现第三个同类域再评估抽通用审批内核（不预埋）。

## 8. 数据模型与迁移 / 开放拍板项

### 8.1 表（实现批次落地，走编号迁移三处同步）

| 表                        | 内容                                    |
| ------------------------- | --------------------------------------- |
| `cloud_resource_catalog`  | §1.2 目录条目（运维维护）               |
| `cloud_resource_requests` | §2 申请单（含状态机/回执字段/revision） |
| 审计                      | 复用既有 audit 链，不建新审计表         |

- 模型加列/加表必须配 `internal/svc/migrations.go` 编号迁移 + `internal/db/migrate/migrate.go` 版本 bump + 迁移测试三处同步（CLAUDE.md 迁移契约；存量库不跑 AutoMigrate）。
- game 作用域字段（gameId/env）落列与否见 §8.4。

### 8.2 前端（实现批次）

三页：资源目录（运维维护）、我的申请（开发）、审批与执行台（审批人/运维）。列布局与 Ops 现有抽屉风格一致；LocalizedText 契约、禁 `any`、API 响应契约照全仓规则。

### 8.3 开放拍板项（实现前须拍板，不猜实现）

1. **目录维护权限挂点**：独立 `cloud.catalog.manage`（建议）还是并入既有 ops 管理面。
2. **审批人角色来源**：RBAC 新角色（建议）还是固定管理员组。
3. **两级审批**：v1 单级（建议），两级留配置位。
4. **game 作用域**：申请单是否挂 gameId/env（建议挂——资源视图进 game 维度与全仓单公司多游戏模型一致）。
5. **计费预估口径**：目录存静态月成本值（建议 v1）还是公式化（需规格→价格表，工程量大）。
6. **atlas 单向回写**：fulfilled/recycled 是否自动写 atlas 清单（建议 v2 评估，先人工在 atlas 维护）。
7. **云 API 自动化**：v1 人工回执（建议）；自动化另立项，凭据/执行机约束按 §5。

## 9. 验收口径（实现批次引用）

- 开发账号：能申请、能重提、能看自己的单；调 catalog 管理端点与执行端点 403；任何页面/接口看不到云凭据。
- 审批人账号：能批/驳（自批被拦）；不能执行、不能改目录。
- 运维账号：能回执创建/回收（永久资源回收须有单）；不能替开发发起申请（execute 不含 create）。
- 免审条目：直通 approved，留痕 approved by=system。
- 到期链：temporary fulfilled 带 expiresAt → T-7/T-1 提醒 Alert 生成 → 回收落 recycled 全留痕。
- 目录为空：申请页空态，不出现可点但必然失败的表单。
