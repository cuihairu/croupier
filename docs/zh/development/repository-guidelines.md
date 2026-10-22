---
title: 仓库规范
---

# Repository Guidelines

## Project Structure & Module Organization

- Go monorepo: binaries in `cmd/`, core implementation in `internal/`, stable exported helpers in `pkg/`.
- Frontend UI: `web/`.
- Configs & assets: `configs/`, `descriptors/`, `scripts/`, `docs/`, runtime data in `data/`.
- Protocol/IDL: `proto/`, generated stubs in `pkg/pb`.
- SDKs: `sdks/<lang>` for code, `docs/sdks/<lang>` for formal docs.

## Build, Test, and Development Commands

- Build all server-side binaries: `make build`
- Build a specific binary: `make server`, `make agent`, `make worker`, `make ingest`
- Generate protobuf code: `make proto`
- Run tests: `make test`
- Build docs: `cd docs && pnpm install && pnpm run build`
- Build dashboard: `cd web && pnpm install && pnpm build`

## Coding Style & Naming Conventions

- Go: `gofmt`/`goimports`; packages lowercase; exported ids `CamelCase`; use `context.Context` first; structured logs.
- New entrypoints go under `cmd/`; do not reintroduce `services/*` layout or stale document references.
- TypeScript/React: Prettier + ESLint; 2-space indent; components `PascalCase`; hooks `useX`.
- Commits: Conventional Commits (`feat(scope): ...`, `fix`, `chore`, `docs`).

## Enumeration Design (Mandatory)

平台状态字段与用户契约枚举采用两套完全不同的机制，禁止混用：

### 1) 平台状态机 [Go int 枚举 + DB int 列]

平台自有状态列（词表由平台定义、编译期收敛）一律使用 `internal/dbenum` 的 int 底座枚举：

- 覆盖范围：`function_contracts.capability`/`risk`、`page_proposals.status`、`tickets.status`、`messages.status`、`feedback.status`、`extension_installation.status`、`certificate.status`。
- Go 层：`type TicketStatus int` + `iota` 常量 + `Parse*/String()`；比较与分支编译期安全。
- DB 层：列存 small int（比较快、索引便宜）；迁移脚本负责把存量字符串回填为 int（空 capability 按函数名推断，如 `player.list → collection_query`）。
- JSON/wire 层：枚举实现 `json.Marshaler/Unmarshaler`，REST 输入输出仍是可读字符串（`"open"`、`"accepted"`），对外契约不变。
- 写入边界统一走 `Parse*`（非法值返回 422），禁止裸字符串直接落库；`Scan` 兼容历史 string 行以便迁移期平滑读取。
- proto/SDK wire 层保持 string（协议不受 DB 枚举化影响）。

### 2) 用户契约枚举 [JSON Schema `enum` 数组透传]

游戏方在 function spec / OpenAPI 里定义的枚举（哪怕字段也叫 status）是**用户数据**，不是平台状态：

- 平台只做三件事：透传（schema 原文存 `inputSchema`/`outputSchema`，无损）、渲染（property 带 `enum` → 表单生成 Select）、校验（运行时按 schema 拒绝非法值）。
- 词表随用户 spec 版本漂移，平台代码不得预知、不得转换为 Go 枚举、不得在 DB 层加 CHECK 约束。
- 用户改 enum 不需要平台发版。

### 3) 判定规则

新增字段时按 owner 判定：词表归平台（状态机/治理语义）→ dbenum int 枚举；词表归游戏方（业务词汇）→ 留在 schema JSON 里。禁止对用户词表新建 Go 枚举，也禁止平台状态继续新增裸字符串列。

豁免记录（保留 string，但必须有词表校验）：

- `extension_installation.status`：状态集含 `uninstalled` 且与 `desired_state` 联动、比较用 EqualFold，涉及扩展生命周期语义重构，暂保留 string + 词表校验。
- `certificate.status`：由到期时间派生（active/expiring/expired），是监控快照而非状态机，保留 string。

## Testing Guidelines

- Go unit tests co-locate as `*_test.go`; prefer table-driven tests.
- Frontend: `cd web && pnpm test` or `pnpm test:coverage`.
- Add tests when touching RBAC, APIs, routing, analytics processing, or descriptor resolution.

## Selective Testing (Mandatory)

本地开发按改动范围选校验范围，禁止「改一行文档也全量编译测试」。CI 门禁保持全量不变（CI 是守门，本地开发用增量）。工具：`make test-changed`（即 `scripts/test-changed.sh`），默认对比 `@{u}`..工作区（含未提交改动），`make test-changed ARG=--committed` 对比 base..HEAD。

分类规则：

| 改动范围                                                                                         | 校验动作                                                                                                                                       |
| ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/**`、`*.md`、纯注释/空白的 `.go`                                                           | 不编译：仅文档检查（docs 站点构建链接校验 + prettier 只查改动文件）                                                                            |
| `internal/**`、`cmd/**`、`pkg/**`、`core/**`、`database/**`、`agents/**`、`examples/**` 的 `.go` | `go build` + `go test` 只跑改动包与其传递反向依赖者（用 `go list` 反查，含测试导入边）                                                         |
| 共享层 `internal/{model,config,common,svc,db}`、`pkg/pb`                                         | 升级全量                                                                                                                                       |
| `go.mod`、`go.sum`、`Makefile`、`CMakeLists.txt`、`.github/**`、`proto/**`、`buf.*`              | 全量门禁                                                                                                                                       |
| `web/**`                                                                                         | tsc + 定向 jest（改动测试文件 + 导入改动源文件的测试文件）；`package.json`、jest/umi 配置、`src/services/api/`、`src/types/` 改动升级全量 jest |
| `docker/**`                                                                                      | `docker compose config` 校验 / Dockerfile 构建（docker CLI 存在时）                                                                            |
| `configs/**`                                                                                     | `go test ./internal/config/...`                                                                                                                |
| `scripts/**`、`tools/**`                                                                         | `bash -n`（shellcheck 若可用）                                                                                                                 |
| 其余（`mobile/`、`packs/`、`descriptors/` 等）                                                   | 仅打印提示不静默丢弃，人工核对                                                                                                                 |

全量门禁 = `gofmt` + `go build ./...` + `go test -short ./...` + web tsc + web jest + docs 构建 + `dashboard_vnext_guard` + `pnpm format:check`。

> Go 测试**不带 `-race`**：与 `make test`（`go test -v -short ./...`）和 CI（`go test ./internal/... ./agents/...`）口径一致，本仓现有门禁均不以 `-race` 执行。已知边界（2026-10-14 实测，均为存量问题、与 CI 无关）：`-race` 全量下 `pkg/protocol` 溢出用例触发 `checkptr` fatal、`internal/server` 等测试代码存在 atomic 写/裸读混用、`internal/hotreload` 插件用例未透传 `-race` 导致 `.so` ABI 不匹配；修好这些前 `-race` 全量不可绿，属独立质量工程项。

什么时候全量：`go.mod`/CI/构建系统改动、共享层改动、发布前收尾门禁。什么时候增量：日常功能、文档、单包修复提交。

## Commit & Pull Request Guidelines

- PR should include: what changed, why, and how it was verified.
- When adding APIs or permissions, update the matching files under `configs/`.
- Keep diffs focused; ensure `make test` passes before merge.

## Security & Configuration Tips

- Secrets go through environment variables, not hardcoded YAML.
- Example local run: `./bin/croupier-server --config configs/server.yaml`.

## Release Tagging

- Server / Agent release tags use `v*`, for example `v0.2.0`.
- SDK release tags use namespaced prefixes:
  - `sdk-js-v*`
  - `sdk-python-v*`
  - `sdk-go-v*`
  - `sdk-java-v*`
  - `sdk-cpp-v*`
- Do not use plain `v*` tags for SDK-only releases, otherwise you will target the server/agent release lane.

## 兼容性与遗留治理（无兼容遗留原则）

本阶段不以向后兼容为约束。新协议、新 SDK、新文档**不默认保留旧入口**。

### 硬性规则

1. **新代码使用 canonical 命名**：协议字段、SDK API、文档术语统一使用当前基线命名（例如 `Task` 而非 `Job`、`ProviderConnect` 而非 `RegisterLocal`、TCP session 而非 `rpc_addr` 回拨）。
2. **默认删除，不默认保留**：重命名或迁移时直接删除旧入口，不留 `@Deprecated` 别名。旧的生成代码、旧文档、旧示例一并清理或归档。
3. **暂留必须有门控**：若确实需要暂留兼容字段（例如 DB 列删除需 migration、proto 字段删除需 SDK 重生成），必须同时满足：
   - 在代码处标注**删除条件**（什么事件触发后可删）和**负责人/期限**。
   - 在 `todo.md` 登记为独立待办，附受影响路径。
   - 提供检测脚本（如 `scripts/check-sdk-matrix.sh`）能在彻底删除前持续可见。
4. **历史归档只进 `docs/archive/`**：迁移说明、旧设计文档移入 `docs/archive/`，不留在主路径文档里作为"当前链路"描述。

### 检查工具

- `scripts/check-sdk-matrix.sh`：SDK 缺能力、旧 wire name、旧 README 术语均视为**失败**（exit 1），CI 阻断。allowlist 仅限协议别名模块，且这些模块本身也须为 canonical 命名。
- `rg "StartJob|RegisterLocal|HeartbeatLocal|rpc_addr|LocalControl"` 只允许出现在 `docs/archive/`、`todo.md` 的门控说明、或生成代码重生成脚本中。

### 违反示例

- [新增 `startJob` 作为 `startTask` 的别名"方便迁移"。]
- [在 README 中把 `rpc_addr` / gRPC 回拨描述为当前链路。]
- [删除旧字段时不写删除条件、不登记 todo。]
- [直接把 `startJob` 改名为 `startTask`,更新所有调用点和测试。]

## 传输层决策(不使用 gRPC)

- 内部 RPC(Server ↔ Agent ↔ SDK)**不使用 gRPC**,用自行开发 TCP transport + protobuf。这是从 gRPC 的坑里重构出来的结论,不可逆。
- 硬约束:**不得新增 gRPC 直接用法**;`go.mod` 残留的 `google.golang.org/grpc` 仅为间接依赖,需定期评估移除。
- 新增 RPC 需求走自行开发 TCP+proto;如需统一 error 协议等能力,在自行开发 RPC 层补齐(proto `Status`/`RpcError`),不引入 gRPC。
