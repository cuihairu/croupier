# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 交付完成定义（Delivery Definition of Done）— 强制

用户审核前，交付必须同时满足以下全部条件，缺任何一项视为未完成、禁止宣称"已完成"：

1. **编译**：`pnpm --dir web run tsc` 0 错误；涉及 Go 时 `go build ./...` 通过
2. **测试**：`pnpm --dir web test` 与 `go test ./internal/...` 全绿（新功能必须带新用例）
3. **Guard**：`bash scripts/dashboard_vnext_guard.sh`（必须从仓库根执行）PASSED
4. **发布链闭环**：功能若涉及 PageSpec/渲染，创建提案→accept-and-publish 线上验证 spec 字段落库（禁止只验证编辑器/预览侧）
5. **文档同步**：功能/字段变更同步 `docs/architecture/dashboard-page-model.md`（模型）、`docs/architecture/pagespec-protocol.md`（wire）、`docs/dashboard/composite-editor-v3.md`（使用）三层；`cd docs && pnpm build` 通过
6. **部署**：Docker 构建成功 + deploy-self-hosted 完成 + CI Core 全绿后才可宣称上线
7. **边界诚实**：未实现/仅预览可用的行为必须在交付说明与文档「已知边界」中列出，禁止静默丢弃

## Essential Development Commands

**Build System (Makefile-driven):**

```bash
make proto        # Generate protobuf code (make dev 不存在)
make build        # Build all binaries (server, agent) to /bin
make proto        # Generate protobuf code via local protoc (scripts/gen-proto.sh)
make pack         # Generate pack artifacts via protoc-gen-croupier
make test         # Run unit tests with race detection
make clean        # Remove build artifacts and generated code
```

**Local Development Setup:**

```bash
git clone --recursive https://github.com/cuihairu/croupier.git
go mod download
./scripts/dev-certs.sh    # Generate self-signed TLS certs
buf lint proto && make proto  # Lint via buf, generate via local protoc
make build               # Build binaries
```

**Testing:**

```bash
make test                 # All tests with race detection
go test ./internal/...    # Subset testing
./bin/croupier-server --config configs/server.yaml validate      # Config validation
```

**Code Style:**

```bash
gofmt -w .                # Format all Go files
gofmt -l .                # List files that need formatting
```

**TypeScript Type Safety (Mandatory):**

**Localized Text Contract (Mandatory):**

后端唯一契约为 `spec.LocalizedText = map[BCP47-locale]string`，即 key 必须是 `"zh-CN"` / `"en-US"`。

- 前端 `web/src/types/dashboard.ts` 的 `LocalizedText` 是唯一类型定义；禁止任何模块再声明自有本地化类型或自造短 key（`zh` / `en` / `zh_cn`）。
- Service 边界必须经 `normalizeLocalizedText`（`web/src/services/api/functions-enhanced.ts`）归一：任何输入形态（BCP47、遗留短 key、裸字符串）统一输出 `{ "zh-CN", "en-US" }`。
- 渲染必须走 `web/src/utils/localizedText.ts` 的 `localizedText(value, locale, fallback)`，禁止在组件内内联 `value['zh-CN'] || ...` 之类的取值链。
- 新增 API/DTO 的本地化字段 review 时按上述三条检查；出现第二份 LocalizedText 定义或组件内取值链视为 review failure。

**NEVER use `any` type in TypeScript/React code.** This is strictly prohibited.

Instead:

- Use proper interface/type definitions from API services
- Use `unknown` for truly unknown types, then narrow with type guards
- Use generics like `Record<string, JSONValue>` for dynamic objects
- Import specific types from libraries: `import type { UploadProps } from 'antd'`
- Use `as` type assertions only when you know the exact type
- Define proper interfaces for component props and state

Examples:

```typescript
// ❌ WRONG - never do this
const handleUpload = async (options: any) => { ... }
const data: any = response;

// ✅ CORRECT - use proper types
const handleUpload: UploadProps['customRequest'] = async (options) => { ... }
const data: Record<string, JSONValue> = response;
```

If you encounter a type error, fix it by:

1. Checking the actual type from the library/API
2. Defining a proper interface if one doesn't exist
3. Using type narrowing or assertions with known types

**SDK Conformance:**

```bash
./scripts/check-sdk-matrix.sh   # Verify each SDK exposes L1 APIs from sdks/SDK_FEATURE_MATRIX.md
```

**IMPORTANT: Before committing any changes, ALWAYS run `gofmt -w .` to ensure all Go files are properly formatted.**

## Architecture Overview

Croupier implements a **three-tier distributed GM backend system**:

1. **Permission Control Layer** - RBAC/ABAC system independent of game logic
2. **Game Control Layer** - Function registration-driven game operations
3. **Observable Display Layer** - Descriptor-driven UI generation

### Core Components

**Server** (`internal/server/`)

- Central control plane with HTTP REST (18780) + self-built TCP transport (19090, agent/SDK connection entry); **gRPC is not used**, see docs/architecture/transport-no-grpc.md
- Two main services: `ControlService` (agent registration) and `FunctionService` (invocation routing)
- Features: load balancing, RBAC, audit chain, approval workflows, multi-game scoping, HA multi-instance (cluster member table + owner forwarding)

**Agent** (`internal/agent/`)

- Distributed proxy in game networks, outbound TCP connection (self-built transport/tcp, length-prefix framing + protobuf) to Server
- Local TCP listener (19091) for game server function registration
- Bidirectional tunnel support for request/response multiplexing
- Job execution with async streaming, idempotency, cancellation

### Data Flow Pattern

```
Web UI → Server (HTTP) → Load Balancer → Agent → Game Server
```

## Key Development Patterns

**Protocol-First Development:**

- All APIs defined in `proto/` using Buf toolchain
- Custom protoc plugin (`protoc-gen-croupier`) generates pack artifacts
- Generated code in `gen/` (ignored in git)

**CRITICAL: Protobuf Code Generation**

- **ALWAYS** use `make proto` (scripts/gen-proto.sh) to generate protobuf code
- Generation is fully **local**: protoc 34.x + protoc-gen-go v1.36.11 (no buf registry / remote plugins)
- Buf is used only for `buf lint proto` (built-in WKT, no network)
- protoc-gen-go version is pinned to match the protobuf 4.25.x API used by pkg/pb

**Descriptor-Driven Architecture:**

- Functions defined via protobuf + JSON Schema descriptors
- UI auto-generates forms, validation, and permission checks from single source
- Function packs (`.tgz`) bundle descriptors, schemas, and UI plugins

**Configuration Management:**

- Multi-layer: YAML → includes → profiles → env vars → CLI flags
- Environment prefixes: `CROUPIER_SERVER_*`, `CROUPIER_AGENT_*`
- Config validation: `./croupier config test`

**Idempotency & Task Model:**

- All operations support `idempotency-key` to prevent duplicate side effects
- Async tasks with event streaming (progress/logs/done/error)
- Task cancellation via `CancelTask` RPC

**Build Tags for Features:**

- `pg` tag: PostgreSQL support for approvals
- `sqlite` tag: SQLite approvals store
- Enables flexible deployment options

## Project Structure Essentials

```
cmd/                      # Binary entry points (server, agent, unified CLI)
proto/                    # Protobuf definitions (Buf workspace)
internal/server/          # Server business logic (control, function, http, registry)
internal/agent/           # Agent logic (tunnel, local server, jobs)
internal/auth/            # RBAC, JWT, TOTP, user management
internal/function/        # Descriptor loading and validation
internal/jobs/            # Job state machine and execution
internal/loadbalancer/    # Load balancing strategies (RR, consistent hash, least conn)
sdks/                     # Multi-language SDKs (go, python, java, js, cpp, csharp)
                          # Feature matrix: sdks/SDK_FEATURE_MATRIX.md (single source of truth)
                          # Wire protocol: docs/architecture/sdk-wire-protocol.md
web/                      # Frontend submodule (Umi Max + Ant Design)
configs/                  # Configuration templates and examples
examples/                 # Demo game servers and invokers
```

## Important Implementation Details

**Security Architecture:**

- TLS for inter-service communication (self-built TCP transport, TLS optional per config; prod should enable)
- Field-level masking for sensitive data in audit logs
- Two-person rule enforcement for high-risk operations
- Audit chain with hash-based integrity

**Single-Company Scope Model:**

- Croupier is not a SaaS multi-tenant platform; it assumes one game company with multiple games
- All operations are scoped by `game_id`/`env` for game/environment isolation
- Registry indexed by `(game_id, function_id)` for function routing
- HTTP headers `X-Game-ID`/`X-Env` propagated through call chain

**Database-per-Game Architecture:**

- When `database.multiGame: true` is set, each `(game_id, env)` pair gets its own physical database (e.g. `game_demo_prod`)
- The configured `database.dataSource` is the **meta database** (`croupier_meta`) holding users, roles, games registry, `game_envs` bindings, audit, extensions, etc.
- `internal/db/router.Router` lazily opens and caches per-game `*gorm.DB` connections, auto-creating and migrating each game database on first use
- `internal/db/dbctx` carries the per-request DB override via `context.Context`; game-scoped models call `dbctx.Resolve(ctx, m.db)` to pick the right connection
- `svc.GameDBMiddleware` resolves the game DB from `X-Game-ID`/`X-Env` headers and injects it into the request context
- Game-scoped models (Player, Function, Ticket, Analytics, Task, ConfigVersion, etc.) live in per-game databases; meta models (Admin, Game, Role, Audit, Extension) stay in the meta DB
- The `game_envs` table (`model.GameEnvBinding`) stores `(game_id, env, database_name)` routing records
- When `multiGame: false` (default for dev/CI), all tables coexist in a single database with `game_id` columns for row-level isolation

**Load Balancing Abstraction:**

- Strategy interface with multiple implementations
- Health checking integrated with agent selection
- Supports routing modes: lb, broadcast, targeted, hash

## Testing Approach

Unit tests focus on:

- RBAC policy grant/deny logic (`internal/auth/rbac/`)
- Job executor state transitions and idempotency (`internal/agent/jobs/`)
- Sensitive field masking (`internal/server/http/`)
- Pack import/export workflows
- Registry agent session management

Integration examples in `examples/` demonstrate end-to-end flows from function registration through UI invocation.

## API Response Contract (Mandatory)

All HTTP APIs must follow one response contract. Do not mix envelope and non-envelope styles.

### 1) Success Response

- Use standard HTTP status code (`200/201/204`).
- Return business payload directly as JSON object/array.
- Do **not** wrap success payload in `{ "code": ..., "message": ..., "data": ... }`.

Examples:

- `200 OK` + `{ "id": 1, "name": "admin" }`
- `200 OK` + `{ "items": [...], "total": 10 }`
- `204 No Content` with empty body

### 2) Error Response

- Use standard HTTP error status (`400/401/403/404/409/422/500`).
- Body uses unified error object:
  - `error`: stable machine-readable code (snake_case)
  - `message`: human-readable message
  - `details` (optional): structured validation/business detail

Example:

- `401 Unauthorized` + `{ "error": "unauthorized", "message": "未授权" }`

### 3) Frontend Error Handling Contract

HTTP status and response body have different responsibilities. Do not duplicate them.

- HTTP status expresses the error class:
  - `400`: invalid request / validation error
  - `401`: unauthenticated / token invalid
  - `403`: authenticated but forbidden
  - `404`: resource not found
  - `409`: conflict
  - `422`: semantically invalid but well-formed request
  - `500`: internal server error
- Response body expresses the readable and structured error detail:
  - `error`: stable code for frontend branching
  - `message`: user-facing error text
  - `details`: optional structured payload for form fields or extra diagnostics

Recommended examples:

- `400 Bad Request`

```json
{
  "error": "validation_failed",
  "message": "请求参数无效",
  "details": {
    "gameId": "不能为空"
  }
}
```

- `401 Unauthorized`

```json
{
  "error": "unauthorized",
  "message": "未授权"
}
```

- `409 Conflict`

```json
{
  "error": "conflict",
  "message": "资源状态冲突"
}
```

Frontend rules:

- Route and global handling should primarily branch on HTTP status.
- UI text should come from `message`.
- Stable client logic should branch on `error`, not on localized `message`.
- Form-level rendering should read `details` when present.
- Do not require frontend to read a business `code` field when HTTP status already exists.

### 4) Explicit Exceptions

- SSE endpoints must return `text/event-stream` (not JSON envelope).
- Health/readiness probes may return minimal payload (for example `{ "status": "ok" }`).
- File/binary download endpoints follow content-type requirements instead of JSON.

### 5) Implementation Rules

- Prefer `internal/common/response` for API handlers.
- Do not introduce new handlers based on `internal/pkg2/response` envelope style.
- Middleware auth failures must keep the same JSON error shape as above.
- New APIs must keep response shape consistent with existing REST handlers in `internal/api`.

### 6) CI/Review Checklist

Before merge, scan for direct ad-hoc response writes:

```bash
rg -n "\bc\.(JSON|IndentedJSON|String|Data|PureJSON|XML|YAML|ProtoBuf)\(" internal --glob "!**/*_test.go"
```

Any new direct write must be justified as an exception (SSE/health/binary), otherwise refactor to unified response helpers.

## Configuration Contract (Mandatory)

Configuration naming must be standardized. Do not invent new casing per file or per module.

This repository previously drifted into mixed styles such as `Server` vs `storage`, `Driver` vs `driver`, `Dir` vs `dir`, `Enabled` vs `enabled`. That is invalid going forward.

### 1) Canonical Naming Rules

- YAML keys: `lowerCamelCase`
- JSON keys: `lowerCamelCase`
- Go struct fields: `PascalCase`
- Environment variables: `UPPER_SNAKE_CASE`
- CLI flags: `kebab-case`

Examples:

```yaml
server:
  host: 0.0.0.0
  port: 18780
  timeout: 600000
  mode: dev

database:
  driver: mysql
  dataSource: xxx

storage:
  driver: file
  baseDir: data/uploads

cache:
  enabled: true
  type: redis
```

### 2) Implementation Rules

- New config structs must use `yaml:"lowerCamelCase"` and `json:"lowerCamelCase"`.
- Do not add new config tags using `Server`, `Driver`, `Dir`, `Enabled`, `BaseDir` style keys.
- Do not mix uppercase and lowercase siblings in the same config object.
- Config examples under `configs/*.yaml` must use canonical keys only.
- If old configs already exist, backward compatibility must be handled in code, not by continuing to write new examples in legacy style.

### 3) Backward Compatibility Rules

- Legacy config keys may be tolerated only in compatibility parsing code.
- Compatibility exists only to avoid breaking old deployments.
- Compatibility code must not become the new documentation standard.
- All docs, examples, generated templates, and new tests must use canonical `lowerCamelCase`.

### 4) Review Checklist

Before merge, check for mixed or legacy config naming:

```bash
rg -n 'yaml:"[A-Z]' internal/config
rg -n '^\s+[A-Z][A-Za-z0-9]*:' configs/*.yaml configs/**/*.yaml
```

If a change introduces new uppercase YAML keys, it is a review failure unless the file is explicitly a legacy compatibility fixture.

## Contract Field Naming (Mandatory)

FunctionContract 及所有对外暴露的 JSON/SDK 契约字段名必须统一 `lowerCamelCase`。此仓库此前因 proto 字段名（snake_case）被直接透传为 SDK/JSON 键名而产生漂移（如 `input_schema`），该做法已废弃。

规范来源：`docs/architecture/openapi-sdk-descriptor-v2.md` 的「命名约定」小节。

### 1) Canonical Rules

- 契约字段（`inputSchema`、`outputSchema`、`approvalRequired`、`policyKey` 等）在所有文档、示例、REST payload、SDK 表层一律 `lowerCamelCase`。
- proto 字段名的 snake_case 仅允许出现在 `.proto` 文件内部（protobuf 官方风格）；其生成 `json_name` 必须为 `lowerCamelCase`，文档引用时禁止把 proto 字段名当作契约键名（允许显式的「对应 proto 字段名 X」对照注释）。
- 序列化边界必须输出 lowerCamelCase 契约键：REST payload、SDK 产出的 JSON/manifest、proto `json_name`。语言原生标识符遵循各自惯例（Go 导出字段 PascalCase 配 lowerCamelCase json tag；Python dataclass/kwargs 按 PEP8 snake_case；C++ 成员 snake_case；TS/JS 与 Java/C# 属性即契约键本身，用 lowerCamelCase），不得把语言的内部命名直接透传为契约键。
- 业务 payload 内部（JSON Schema properties、游戏业务数据）不属于平台契约，命名由游戏方自行决定。
- 枚举字符串值（如 `input_schema_stale`）是机器标识符，不属于本规则范围。

### 2) Implementation Rules

- 新增 REST DTO、SDK descriptor 字段时禁止 `json:"snake_case"` 形式的契约键。
- **禁止兼容旧键**：发现漂移时直接改名为 canonical 键，并在同一变更内更新全部调用方（含 web 子模块、SDK 示例与测试）；不得为旧键保留兼容解析、双读或别名映射。兼容层只会让错误命名永久化。
- 更新 SDK 字段名时，同 PR 内必须同步更新对应 `docs/sdks/<lang>/` 指南，保持文档与代码一致。

### 3) Review Checklist

Before merge, scan for snake_case contract keys leaking to JSON/SDK surfaces or docs:

```bash
rg -n 'json:"[a-z]+_[a-zA-Z]+"' internal sdks --glob '!**/*_test.go' --glob '!**/generated/**' --glob '!**/_pb2*' --glob '!gen/**' --glob '!**/pkg/pb/**'
rg -n '`input_schema`|`output_schema`|`approval_required`|`approval_policy_key`' docs --glob '!docs/archive/**' --glob '!docs/sdks/**'
```

New occurrences are review failures unless they are explicit legacy compatibility code or annotated proto-name references.
