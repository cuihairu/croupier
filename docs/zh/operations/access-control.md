---
title: 权限控制说明（RBAC、鉴权、敏感操作与堡垒机现状）
---

# 权限控制说明

面向安全与运维角色，盘点本仓权限控制的真实实现路径：角色与权限模型、鉴权链路、敏感操作管控，以及堡垒机/跳板接入能力现状。逐项给出 file:line 证据，未实现或仅骨架的部分如实标注。口径核对日期 2026-10-20，行号以后续演进可能漂移，以核对日为准。

> 口径边界：`docs/zh/guide/concepts/permissions.md` 与 `docs/zh/operations/security.md` 是面向使用者的概念与配置文档；本文以代码实现为口径，两者不一致处以本文标注的差异为准（见文末「已知口径差异」）。本文不涉及 CI/CD 面的内容，那在DevOps 说明。

## 角色与权限模型（RBAC 生产路径）

### 判定入口

- **生产判定函数只有一条路径**：`internal/security/rbac/logical_permissions.go:40` 的 `EnforceAnyPermission(subject, granted, required...)`——Casbin「相等模型」（`logicalPermissionModel`，`:10`）驱动，支持 `resource:action` 拆分与 `*` 通配（`:96` 起）。服务侧以变量注入使用：`internal/service/permission/service.go:35`（`enforceAnyPermission = rbac.EnforceAnyPermission`）。
- **CheckPermission 流程**：`internal/service/permission/service.go:37` 起——加载管理员角色（`super_admin` 直接放行），由 `permissionCandidates(resource, action)`（`:215`）生成候选权限串（含 `*` 与 `resource:*` 形态），任一命中即放行。
- **角色与权限数据模型**：`roles`（`internal/model/role.go:6`，表 `roles`）、`permissions`（`internal/model/permission.go:10`，表 `permissions`）、`role_permissions` 关联、`admin_roles` 关联。**角色是全局的，不带 game 维度**——game/env 隔离放在管理员级作用域表：`admin_game_scopes` 与 `admin_game_env_scopes`（`internal/model/admin_scope.go:6`/`:18`）。
- **game/env 作用域判定**：`CheckGameScope`（`service.go:82`）与 `CheckGameEnvScope`（`:99`）查上述两张作用域表；HTTP 侧 `X-Game-ID`/`X-Env` 头解析为作用域上下文后逐请求校验（`internal/service/permission/middleware.go`）。即：**多游戏隔离靠管理员作用域表，不靠角色复制**。

### 种子数据与外部身份

- **种子角色/权限**：`configs/roles.json` 41 个角色、`configs/permissions.json` 39 条权限（2026-10-20 实测 JSON 条目数），启动时由 `seedBootstrapRoles`（`internal/svc/service_context.go:1100`，调用点 `:576`）落库。
- **身份提供方抽象**：`internal/security/identity/provider.go:1-9` 写明边界——Provider 只负责「证明你是谁」（`KindLocal`/`KindLDAP`/`KindOIDC`/`KindGitHub` 常量 `:19-22`），「你能做什么」始终由本地 RBAC（admins/roles 表）裁决；外部身份登录按需 JIT 落库为本地影子账号（`internal/api/auth/service.go:506` 的 `assignDefaultRoles`，`:513` 实现）。
- **缺失项（模型未接线）**：`internal/security/rbac/` 下还存在 `rbac.go`（文件策略）、`casbin.go`（Casbin enforcer 封装）、`unified.go`（UnifiedPolicyEngine）、`enhanced.go`（ABAC 表达式引擎）——这些引擎**没有生产调用方**：文件策略引擎唯一的非测试入口是 CLI 配置校验 `internal/cli/common/validate.go:86-89`（校验 `rbac_config` 文件可读），运行时鉴权不经过它们。文档若出现「ABAC 策略引擎」相关能力描述，均为未接线状态。

## 鉴权链路

### 令牌签发与校验

- **JWT**：`internal/security/jwtutil/token.go:32` `Sign(...)`，HS256（`:50`），claims 含 `tokenVersion`（`:19`）与 `adminID`；TTL 24 小时（`:23`）。
- **secret 解析**：`internal/security/jwtutil/secret.go:37` `ResolveSecret`——非开发模式未配置 `CROUPIER_AUTH_JWT_SECRET` / `auth.jwtSecret` 时直接报错拒绝启动（`:44`）；dev 模式回退 `croupier-dev-secret`（`:12`/`:42`）。生产部署必须显式配置。
- **中间件装配**：`internal/svc/service_context.go:62` 声明 `Authority gin.HandlerFunc`，`:626` 装配；HTTP 路由在 `internal/handler/routes.go:119-120` 把 `protected` 组整体挂上 `serverCtx.Authority`。

### 请求侧校验

- **取令牌**：`Authorization: Bearer` 头，SSE 等 `<img>`/EventSource 场景回退 `?token=` query 参数（`internal/svc/service_context.go:1629-1641`）。
- **authenticate**：`:1736` 起——解析 claims 后强制比对 `admins.token_version` 当前值（`:1747-1751`，`checkTokenVersion` `:1758`）：改密码/禁用/登出会使库中版本递增，旧令牌立即失效。带短 TTL 缓存降低查库频率。
- **mfaGate**：`:1686-1707`——站点安全策略 `MFARequired` 开启时，未绑定 TOTP 的账号除白名单前缀外一律 `403 mfa_required`（`:1699-1703`）。
- **公开端点白名单**：`routes.go:108-116` 注册的公开路由（registry 只读、OpenAPI 只读、更新检查、CI webhook、告警推入、玩家客服等）不经过 Authority；webhook/告警端点的令牌与 HMAC 校验在各 handler 内部（`:110-112` 注释）。Prometheus 抓取端点开启时加入免认证白名单（`service_context.go:1615`）。
- **外部令牌（API token）**：`internal/model/external_token.go:18-21` `HashExternalToken`——明文只在创建时返回一次，库存 sha256 十六进制（`crypto/sha256`），校验时哈希比对。
- **前端守卫**：`web/src/app.tsx` 的 `onPageChange` 未登录重定向登录页（`:296-299`）；`web/src/access.ts` 由角色映射前端可见性。**前端只是体验层**，后端 Authority 才是强制点。

### 已知弱点（如实标注）

- **`X-Admin-ID` 头兜底**：`internal/service/permission/middleware.go:161-171`，取不到会话上下文时直接信任 `X-Admin-ID` 头声明身份。该路径面向内网部署形态，**外网暴露部署须确认该中间件未被绕过链路触达**。

## 敏感操作管控

### 风险分级与审批策略

- **风险四级**：`internal/dbenum/p1_enums.go:226-229`——`safe | warning | high | danger`（`String()` 与解析在 `:233-247`）。函数契约的 `Risk`/`ApprovalRequired` 字段经 `function.ResolvePolicyRisk` 归一（契约显式声明 > 默认策略两级优先）。
- **默认策略（硬编码兜底）**：`internal/policy/manager.go:109-132`——`high`：强制审批（`single_admin` 工作流）+ 审计；`danger`：强制审批（**`two_person` 工作流**）+ 审计 + 仅 `super_admin` 可执行；`configs/default-policies.yaml` 存在时覆盖硬编码默认。
- **审批拦截**：命中强制审批的函数调用会转审批工单而非直接执行；审批相关路由在 `routes.go:229` 起。

### 两人规则与 step-up TOTP

- **两人规则的真实位置**：不在 rbac 包，而在审批服务——`internal/api/approval/service.go:117-136` `rejectSelfApproval`：申请人不能审批自己的申请，approve（`:217`）与 reject（`:295`）双路径强制；`approval.allowSelfApprove=true` 时放行（单管理员部署的显式选择），拦截与放行均写哈希链审计事件（`approval.self_approval_blocked`）供追溯。
- **高危审批二次验证**：`internal/api/approval/stepup_otp.go:16-26`——治理风险 `high`/`danger` 的审批必须 step-up TOTP：已绑定账号 otp 必填且校验通过，未绑定账号直接拒绝（`otp_not_enrolled`），禁止静默放行。

### 审计链与诚实边界

- **审计记录与链**：`internal/audit/audit.go:151-166` `AuditRecord`（含 `ChainInfo`）；敏感字段脱敏 `maskSensitiveData`（`:549`）+ 统一敏感字段清单（`:564`）。
- **诚实边界 1（链哈希强度）**：链哈希 `hashBytes`（`:807-815`）实际是 DJB2 32 位（函数内注释自称「Using SHA256」但实现是 uint32 迭代）——**完整性提示用途尚可，防篡改证明强度不足**，属于已知待加固项。
- **诚实边界 2（验证无生产入口）**：`ValidateChain`（`:667-668`）存在但无生产调用方、无暴露端点（全仓 grep 仅定义与测试命中）。
- **诚实边界 3（审批存储默认内存）**：审批存储默认 `approvals.NewMemStore()`（`internal/svc/service_context.go:342`）——**重启丢审批工单**；SQL 存储实现存在但该装配点未切换。
- **诚实边界 4（签名器为空）**：生产装配 `audit.NewAuditService(auditStore, nil)`（`:350`）——审计外发签名器为 nil，只落库不签名。

## 堡垒机/跳板接入能力（未实现，骨架）

`agents/devops`（devops-agent）的远程会话通道**尚未实现**，当前为骨架批次：

- **判定证据**：`agents/devops/session/session.go:64-73` `noopTarget`——骨架占位 Provider，`Open` 一律返回 `session channel %s not implemented yet (skeleton)`（`:72`）；装配点 `agents/devops/runner/runner.go:92` 就是 `session.NewNoopTarget()`。全目录无 `x/crypto/ssh`、无 guacamole 依赖；`config.go:64-66` 的 `guacdAddr` 是**预留字段，注释明示骨架批次不接**。
- **已落地部分**（通道到位即可复用）：
  - **白名单**：`agents/devops/session/allowlist.go:31` `Check(host, port, protocol)`——host/port/protocol 三元组不在白名单即拒绝；配置校验要求 `devops.enabled=true` 时 allowlist 非空、host/ports 必填（`agents/devops/config/config.go:98-118`）。
  - **Broker 编排**：`agents/devops/session/broker.go:63` `OpenSession`——白名单先决，审计先行（失败也留痕），再进 Provider。
  - **元数据级录制**：`agents/devops/session/recorder.go:16-27` `SessionMeta`——会话起止/协议/目标/操作者以 JSONL 落盘（不含屏幕内容）。
- **计划指针**：真实会话通道（guacd 主通道 + 本地 SSH 兜底）属 `todo.md` 的 devops 真实会话条目；设计背景见 `docs/zh/design/agent-core-design.md`。**在那之前，本仓不存在可用的堡垒机/跳板能力，任何「通过 Croupier 跳转内网主机」的表述均为不实。**

## 已知口径差异（文档 vs 代码）

| 口径                                      | 文档表述                                                     | 代码实况                                                                                                              |
| ----------------------------------------- | ------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------- |
| `game:{gameId}:{permission}` 权限格式     | `docs/zh/guide/concepts/permissions.md:132` 列为权限命名形态 | 代码无任何 `game:` 前缀权限串实现（全仓 grep 仅命中无关的遥测 topic 前缀）；游戏隔离实际走 `admin_game_scopes` 表判定 |
| ABAC / UnifiedPolicyEngine / 风险策略引擎 | 概念文档有相关能力描述                                       | 引擎代码在 `internal/security/rbac/` 但无生产调用方（见上「模型未接线」）                                             |
| 两人规则归属                              | 常被理解为 RBAC 能力                                         | 实现在审批服务 `rejectSelfApproval`（`internal/api/approval/service.go:117`），RBAC 层只有角色/权限/作用域判定        |
| 堡垒机远程会话                            | 部分设计文档描述了 guacd/SSH 方案                            | 仅骨架（`noopTarget` 恒拒），方案未落地                                                                               |

## 相关页面
