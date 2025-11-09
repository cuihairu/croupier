# TODO

## C-Architecture Migration (Ports/Adapters + Wire)

Goal: Clear app/service/ports/repo layering, Wire DI, single-source models, no infra leakage.

### 0) Agree On Structure (done)
- Docs: `docs/directory-structure.md` describing target layout
- Naming: `app/`, `service/`, `ports/`, `repo/gorm/`, `platform/`, `security/`

### 1) Repo Unification (low risk)
- [x] Move GORM repos to `internal/repo/gorm/*`
  - [x] `internal/infra/persistence/gorm/users`     → `internal/repo/gorm/users`
  - [x] `internal/infra/persistence/gorm/messages`  → `internal/repo/gorm/messages`
  - [x] `internal/infra/persistence/gorm/support`   → `internal/repo/gorm/support`
  - [x] `internal/infra/persistence/gorm/assignments` → `internal/repo/gorm/assignments`
- [x] Update imports across codebase
- [x] `go mod tidy` + build + smoke test (users/messages/support/assignments) — build OK (server/edge/agent)

### 2) Games Domain Split
- [x] Define ports for games
  - [x] `internal/ports/games.go`: `GamesRepository`, `UnitOfWork`, DTOs
- [x] Create repo/gorm/games (implement ports)
  - [x] Adapter wraps existing `internal/server/games.Repo` for now
  - [x] Move GORM models from `internal/server/games` into `internal/repo/gorm/games` (store remains under server for edge)
  - [x] Methods: Get/Create/Update/ListEnvs/ListEnvDefs/EnsureEnvDef
- [x] Create service/games (use-cases)
  - [x] AddEnv/UpdateEnv/RemoveEnv: rules (normalize/unique/case-insensitive)
  - [x] Apply defaults from `configs/games.json`
- [x] Wire
  - [x] `internal/app/server/http/wire.go`: RepoSet + ServiceSet + InitServerApp
  - [x] Generate `wire_gen.go` (checked-in; manual generation for now)
  - [x] Switch cmd/server and servercmd to use InitServerApp
  - [x] Later: run `wire` tool in CI/dev to verify
- [ ] HTTP handlers refactor
  - [x] Replace direct GORM calls with service invocations for envs POST/PUT/DELETE/GET
  - [x] Create uses service defaults
  - [x] Move other game handlers to service (list/detail/update/delete)
  - [x] /api/me/games moved to service where available
  - [x] importLegacyGamesIfEmpty uses service (when available) for consistency
  - [x] Responses keep `envs` + `game_envs[{env,description,color}]`
- [x] Build + e2e test (create game → default envs + colors → CRUD envs)

### 3) App Layer Placement
- [x] Move `internal/server/http` → `internal/app/server/http`
- [x] Fix imports; keep route paths unchanged
- [x] Verify middlewares (RBAC, ScopeGuard, CORS, Logger)

### 4) Security/Platform Renames (optional)
- [x] `internal/auth/*` → `internal/security/*` (rbac, token)
- [x] `internal/objstore` → `internal/platform/objstore`
- [x] `internal/tlsutil` → `internal/platform/tlsutil`
- [x] Update imports + build

### 5) Adapters Examples
- [x] Move `adapters/http` → `tools/adapters/http` (sample)
- [x] README note: examples are non-critical
 - [x] Remove empty legacy `adapters/` root

### 6) Permissions Package Decision
- [x] Remove legacy permissions package (unused)
- Notes: roles/permissions currently managed via usersgorm + Casbin; revisit unification later if needed

### 7) CI/Docs
- [x] Update README (architecture overview + DB drivers + Wire usage note)
- [x] Ensure `wire` installed in dev tooling; document `wire` generation (see `docs/wire-and-di.md`)
- [x] Add CI step: `wire` check (or commit `wire_gen.go`)

Notes:
- Already removed legacy `internal/infra/persistence/gorm/gamemeta/models.go` to avoid env table conflicts.
- Removed `internal/infra/persistence/gorm` and `internal/infra/persistence` (legacy paths) after moving repos.
- Frontend: removed legacy GamesMeta page/service; switched to `/api/games` and `/api/me/games` via `web/src/services/croupier/games.ts`.
- Games envs now: global env PK (varchar(50), description, color), per-game `envs` JSON list.

## Provider Manifest + 多语言 SDK（语言无关能力声明）

目标：以 Manifest(JSON) + JSON‑Schema 为权威声明，兼容 protobuf；支持 Go in‑proc 与 Python/Node 等 out‑of‑proc。Server 合并 descriptors 到 `/api/descriptors`。

### A) 设计与文档
- [x] 写入 `docs/providers-manifest.md`（中文）说明目标、概念、清单结构、注册流程、参数/校验、实体/操作、错误模型、打包与示例。
- [x] 定义 Manifest 的 JSON Schema：`docs/providers-manifest.schema.json`（约束 provider/functions/entities/operations，扩展 `x-ui`/`x-mask`/`x-source`）。
- [x] 撰写控制面扩展草案：`docs/control-capabilities.md`（RegisterCapabilities RPC、向后兼容、实施步骤）。

### B) 控制面（ControlService）扩展（向后兼容）
- 方案一（推荐）：新增 `RegisterCapabilities`（携带 `provider` 元数据 + 压缩 manifest JSON + 可选内嵌 schemas/fds）。
- 方案二：在现有 `Register` 增加可选字段 `manifest_json_gz`，大于阈值改走分段上传或对象存储。
- [ ] 选型并更新 `proto/croupier/control/v1/control.proto`；`buf generate`。
- [ ] Server 端解析与合并：保存/缓存 manifest；构建统一 descriptors，暴露 `/api/descriptors`。
- [ ] 单测：小 manifest、嵌入 schema、无/有 fds；向后兼容（旧 Agent 仅 functions 列表）。

### C) protoc 插件：从 protobuf 生成 Manifest（可选）
- [ ] 扩展 `tools/protoc-gen-croupier`：`emit_manifest=true`，从 .proto + 自定义 options 产出 `manifest.json` + `schema/*.json`。
- [ ] 自定义注解 options（示例）：`auth.require`、`semantics.rate_limit/concurrency/idempotent`、`entity/op/target`、`ui hints`。
- [ ] 示例 .proto + 生成产物 + 测试。

### D) SDK 骨架（多语言）
- Go（in‑proc）：
  - [ ] Builder 生成 manifest（代码优先），或加载外部 manifest（清单优先）。
  - [ ] 绑定 handler（函数/实体操作）、JSON‑Schema 校验、错误映射、注册到 Control、启动 FunctionService。
- Python/Node（out‑of‑proc）：
  - [ ] 加载 manifest.json，绑定 handler；内置 jsonschema 校验；起 gRPC 服务；注册到 Control。
  - [ ] 示例 Provider（player/session）：函数 + 实体操作，含 target 解析与权限。
- [ ] 统一错误模型与重试建议（InvalidArgument/NotFound/RateLimited/...）。

### E) Server/Edge 集成
- [ ] Server 合并多 Provider 的 descriptors（函数 + 实体/操作），供 `/api/descriptors` 使用。
- [ ] Edge 仅路由 function_id，不关心 manifest 细节；保留 JSON↔Proto 编解码支持。
- [ ] /metrics 暴露 Provider/Forwarder 关键指标（已接入 forwarder 基础指标，可按需扩展）。

### F) 前端与权限
- [ ] 前端从 `/api/descriptors` 读取 Entity/Operation + 字段 UI hints，自动渲染“对象+操作”页面；减少硬编码。
- [ ] RBAC：将 `auth.require` 与角色模板联动（configs/roles.json），提供默认勾选建议。

### G) 测试矩阵
- [ ] Manifest 校验（JSON Schema）、参数校验（必填/枚举/范围/oneOf）。
- [ ] 实体 target 解析（field/jsonpath）与错误处理。
- [ ] Proto FQN 映射（FDS 下 JSON↔Proto 循环测试）。
- [ ] 端到端：Server/Edge/Provider 调用（JSON 与 Proto 两条路径）。

里程碑建议
- M1：文档+Schema+Control 扩展雏形+Server 合并+Go in‑proc 示例（1 周）。
- M2：Python/Node SDK 骨架 + 示例 Provider + e2e（1–2 周）。
- M3：protoc‑gen‑croupier emit_manifest 与前端自动化（1 周）。
