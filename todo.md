# 平台安全与可观测性补强 TODO

方案共识：MFA 按 provider 维度接线（local 默认建议开启，LDAP/OIDC 默认信任 IdP）；Prometheus 端点提供但默认关闭；登录锁定与 token 撤销按 tokenVersion 方案；清理文档债。

每个任务原子性：独立完成、独立迁移文件、独立测试、独立可提交，互相之间无代码依赖（仅迁移序号需按落地顺序递增）。

> **状态：全部完成（T5→T1→T2→T3→T4 顺序落地，2026-09-02）**
>
> - T1: 迁移 0017（admins.failed_attempts/locked_until/token_version）+ `LoginLockoutConfig`（默认 5 次/15 分钟，仅 local 计数）+ `auth.login_locked` 审计；测试 `internal/api/auth/lockout_test.go`
> - T2: JWT claims 增 `tokenVersion`（旧 token 解析为 0 平滑兼容）；中间件比对 + 30s 进程内缓存；登出/改密（自助+重置）/禁用账号递增；测试 `internal/svc/token_revoke_test.go`
> - T3: 迁移 0018（admins.otp_enabled）+ MFA API（setup/confirm/disable）+ 登录按 provider 分支（local 校验 totpCode，外部身份源跳过）+ `mfa_required` 错误码；测试 `internal/api/auth/mfa_test.go`
> - T4: `telemetry.prometheus.enabled`（默认 false）+ exposition 端点（默认 `/metrics/prometheus`，免认证白名单动态注册）+ go/process + 平台指标（与 JSON /metrics 同口径）；测试 `internal/api/monitoring/prometheus_test.go`；文档 `docs/operations/monitoring.md` 已按实际实现修正
> - T5: 删除 `internal/api/user` 空壳；CLAUDE.md 路径漂移修正（`internal/auth/` → `internal/security/`）
>
> 已知边界：token 撤销最长 30s 生效延迟（缓存 TTL）；MFA 前端二次输入 UI 未做（后端契约就绪，`401 + error=mfa_required`）；`docs/security.md` 已记录。

## T1. 登录失败锁定 ✅

**目标**：密码连续失败 N 次后锁定账号 M 分钟，防止在线爆破。

**改动点**：

- `internal/db/migrate/migrations/` 新增迁移：`admins` 表加 `failed_attempts INT NOT NULL DEFAULT 0`、`locked_until TIMESTAMP NULL`
- `internal/model` Admin 模型加对应字段
- `internal/api/auth/service.go` `Login()`：
  - 认证前检查 `locked_until > now` → 拒绝并审计 `auth.login_locked`
  - 失败时 `failed_attempts+1`，达到阈值（默认 5 次）写 `locked_until = now + 15min`
  - 成功后清零 `failed_attempts`
- 阈值/窗口走配置（`security.loginLockout.threshold` / `windowMinutes`），带默认值，配置文件示例更新
- OIDC/LDAP 登录失败不计数（失败在 IdP 侧）；仅 local provider 失败计数

**验收**：

- 新增单测：连续 5 次失败锁定、锁定期内正确密码也拒绝、窗口过后自动解锁、成功登录清零计数
- `go test ./internal/api/auth/...` 通过
- 锁定事件写入哈希链审计可查

## T2. tokenVersion 撤销机制 ✅

**目标**：改密码、禁用账号、登出后旧 token 立即失效，消除 24h 不可吊销窗口。

**改动点**：

- 新增迁移：`admins` 表加 `token_version INT NOT NULL DEFAULT 0`
- `internal/security/jwtutil/token.go`：claims 加 `tokenVersion`，`Sign` 接受版本参数
- `internal/middleware/auth.go`：验签后比对 claims 版本与库中当前版本，不一致返回 401（查询走带短 TTL 的进程内缓存，避免每请求打库；缓存与 T1 的锁定检查可复用同一查询）
- `internal/api/auth/service.go`：`Logout` 递增 `token_version`（全局登出）；`issueLogin` 签发时带上当前版本
- 改密码（admin PasswordReset / profile 修改密码）、禁用账号处递增 `token_version`
- 审计事件 `auth.token_revoked`

**验收**：

- 新增单测：登出后旧 token 401、改密码后旧 token 401、正常 token 不受影响、缓存失效后版本同步
- `go test ./internal/api/auth/... ./internal/middleware/...` 通过

## T3. MFA（TOTP）按 provider 接线 ✅

**目标**：`VerifyTOTP` 接入登录流程；local provider 账号可启用 TOTP，LDAP/OIDC 登录跳过平台 MFA（信任 IdP）。

**改动点**：

- 新增迁移：`admins` 表加 `totp_secret VARCHAR NULL`、`totp_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `internal/api/auth/`（或新子模块）新增 MFA 管理 API：
  - `POST /api/v1/auth/mfa/setup`：生成 secret 返回 otpauth URL
  - `POST /api/v1/auth/mfa/confirm`：校验首个 code 后置 `totp_enabled=true`
  - `POST /api/v1/auth/mfa/disable`：校验 code 后关闭（需登录态 + 密码确认）
- `internal/api/auth/service.go` `Login()`：密码通过后，若 `provider == local && totp_enabled`：
  - 请求未带 `totpCode` → 返回特定错误码（如 `mfa_required`），前端据此展示二次输入
  - 带 `totpCode` → `VerifyTOTP` 校验，失败审计 `auth.mfa_failed`
  - LDAP/OIDC 身份直接跳过该分支
- 复用已定义的审计事件 `auth.mfa_enabled/disabled`
- `LoginRequest` DTO 加 `totpCode` 可选字段

**验收**：

- 新增单测：local+MFA 无 code 返回 `mfa_required`、错误 code 拒绝、正确 code 放行、LDAP/OIDC 登录不触发 MFA、setup/confirm/disable 状态机
- `go test ./internal/api/auth/...` 通过
- 前端二次输入 UI 不在本任务范围（仅后端契约 + 错误码），在交付说明中列为已知边界

## T4. Prometheus /metrics 端点（默认关闭） ✅

**目标**：提供标准 Prometheus exposition，不强制部署 OTel Collector。

**改动点**：

- 配置新增 `telemetry.prometheus.enabled`（默认 `false`）、`telemetry.prometheus.path`（默认 `/metrics/prometheus`，避开现有 JSON `/metrics`）
- 引入 `prometheus/client_golang`，`internal/telemetry/` 新建 prometheus provider：注册 DB 延迟、agent 在线数、函数调用计数等已有 OTLP 指标的对应 prometheus collector（只暴露已在 OTLP 侧存在的指标，不新增口径）
- `internal/handler/routes.go`：开关开启时挂载 `promhttp.Handler()`
- `configs/telemetry.example.yaml` 补示例

**验收**：

- 新增单测：开关关闭时路由 404、开启时返回 exposition 文本且含预期指标名
- 默认配置下行为与现状一致（回归）
- `go test ./internal/telemetry/... ./internal/handler/...` 通过

## T5. 文档债清理 ✅

**目标**：删除空壳模块、修正 CLAUDE.md 路径漂移。

**改动点**：

- 删除 `internal/api/user/`（仅 dto.go + dto_test.go，无 handler/service，功能由 `internal/api/admin` 承担）；确认全仓库无 import 引用
- CLAUDE.md：
  - `internal/auth/  # RBAC, JWT, TOTP, user management` → 修正为实际路径 `internal/security/` + `internal/api/auth`
  - `internal/auth/rbac/` → `internal/security/rbac/`
- 顺带核对 CLAUDE.md 中其他 `internal/server/http` 等路径引用与实际 `internal/api/` 的一致性

**验收**：

- `go build ./...` 通过、`go test ./internal/...` 无回归
- `rg "internal/auth/" CLAUDE.md` 无残留

## 执行顺序建议

T5（无风险热身）→ T1 → T2 → T3 → T4。前四个每个含独立 goose 迁移，按落地顺序编迁移序号；每个任务单独提交。
