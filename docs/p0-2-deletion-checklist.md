# Dashboard 旧模型物理删除清单

更新时间：2026-08-09

本文是 `todo.md` 的 `A-002` 与 `H-001` 至 `H-005` 的执行清单。每一项必须
满足替代路径、验收前置条件和删除责任后才能物理删除；不使用 deprecated 标记、
兼容 wrapper 或数据转换桥。生产数据删除必须另行取得明确确认。

## 执行规则

| 状态                | 含义                                                 |
| ------------------- | ---------------------------------------------------- |
| `已物理删除`        | 路径不存在，guard 负责阻止回流。                     |
| `待删除`            | 仍存在旧模型或旧数据，必须在所列前置条件完成后删除。 |
| `非 Dashboard 范围` | 名称相似但服务于其他业务页面，不得因本清单删除。     |

每次执行删除时，责任 agent 必须在交接中记录：删除的路径、替代任务 ID、执行的
验收命令、数据库备份标识（如涉及数据）。

## 前端旧运行时与路由

| 路径或符号                                      | 状态              | 替代路径                              | 前置条件                                       | 对应任务         |
| ----------------------------------------------- | ----------------- | ------------------------------------- | ---------------------------------------------- | ---------------- |
| `web/src/components/FormilyPageRenderer/`       | 已物理删除        | `SchemaFormRenderer` + `PageRenderer` | guard、前端 build                              | `H-001`、`H-002` |
| `web/src/components/formily/`                   | 已物理删除        | `SchemaFormRenderer`                  | guard、前端 build                              | `H-001`          |
| `web/src/components/FunctionFormManager/`       | 已物理删除        | `SchemaFormRenderer`                  | guard、函数表单 POC                            | `H-001`          |
| `web/src/pages/PageStudio/PageSchemaEditor.tsx` | 已物理删除        | `PageEditor`                          | Page Studio 编辑/发布 E2E                      | `H-003`          |
| `web/src/pages/PageStudioV2/`                   | 已物理删除        | `web/src/pages/PageStudio/`           | Page Studio E2E                                | `H-002`、`H-003` |
| `web/src/types/dashboard-vnext.ts`              | 已物理删除        | `web/src/types/dashboard.ts`          | TypeScript typecheck                           | `H-002`          |
| `web/src/services/dashboard-vnext.ts`           | 已物理删除        | `web/src/services/dashboard.ts`       | TypeScript typecheck                           | `H-002`          |
| `web/src/components/PageGenerator/`             | 已物理删除        | Proposal Inbox + Page Studio          | `I-001`、`I-002`、`I-003`                      | `H-004`          |
| `web/src/components/WorkspaceRenderer/`         | 已物理删除        | Console `PageRenderer`                | 全量 Console E2E                               | `H-002`、`H-004` |
| `web/src/pages/WorkspaceEditor/`                | 已物理删除        | Page Studio                           | Page Studio E2E                                | `H-004`          |
| `web/src/pages/Workspaces/`                     | 已物理删除        | Proposal Inbox / Page Studio          | Console 菜单 E2E                               | `H-004`          |
| `web/src/pages/Entities/`                       | 已物理删除        | Resource Catalog                      | Resource Catalog POC                           | `H-004`          |
| `web/src/components/page-schema/`               | 非 Dashboard 范围 | 不适用                                | 它仅被 Functions Directory 与 Assignments 使用 | 不删除           |

## 后端旧协议、API 与运行路径

| 路径或符号                                     | 状态       | 替代路径                                                | 前置条件                       | 对应任务 |
| ---------------------------------------------- | ---------- | ------------------------------------------------------- | ------------------------------ | -------- |
| `internal/dashboard/generator/generator_v2.go` | 已物理删除 | `generator.go` + `resource_generator.go`                | generator/service tests        | `H-002`  |
| `internal/dashboard/descriptors/`              | 已物理删除 | 持久化 FunctionContract / CapabilitySemantics           | SDK/OpenAPI 注册链路 E2E       | `H-004`  |
| `internal/function/uicontract/`                | 已物理删除 | FunctionContract + FormPresentationSpec                 | SDK parity tests               | `H-004`  |
| `internal/function/converter/pack.go`          | 已物理删除 | FunctionContract adapter                                | SDK/OpenAPI registration tests | `H-004`  |
| `internal/logic/function/function_u_i_*.go`    | 已物理删除 | Page Studio / versioning service                        | `I-003`                        | `H-004`  |
| `internal/logic/function/ui_resolver*.go`      | 已物理删除 | Proposal generator                                      | `I-001`、`I-002`               | `H-004`  |
| 旧页面配置 API 路径                            | 已物理删除 | `/api/v1/pages/*` + `/api/v1/console/*`                 | SDK parity、Console E2E        | `H-004`  |
| `/api/v1/functions/:id/ui*`                    | 已物理删除 | 函数注册不提供 UI API；页面由 Proposal/Page Studio 管理 | registration boundary tests    | `H-004`  |
| Function UI DTO/handler symbols                | 已物理删除 | FunctionContract DTO + Page API DTO                     | API integration tests          | `H-004`  |

## 旧数据结构与迁移

| 表或列                              | 状态              | 替代结构                                                                  | 删除前置条件                                              | 对应任务 |
| ----------------------------------- | ----------------- | ------------------------------------------------------------------------- | --------------------------------------------------------- | -------- |
| 旧 workspace/object-page 配置表与列 | 待核对生产 schema | `page_specs`、`page_versions`、`published_page_specs`、`page_proposals`   | 完成 `I-001` 至 `I-003`；导出备份并取得生产删除确认       | `H-005`  |
| 旧 Function UI 历史表与列           | 待核对生产 schema | `page_versions`、`page_proposal_versions`、`capability_semantic_versions` | 完成 Page Studio rollback E2E；导出备份并取得生产删除确认 | `H-005`  |
| 旧 entity definition 相关表与列     | 待核对生产 schema | `function_contracts`、`resource_capabilities`、`capability_semantics`     | Resource Catalog E2E；导出备份并取得生产删除确认          | `H-005`  |

数据库删除必须使用显式的版本化迁移，且只能调用 `db.Migrator().DropColumn` 或
`db.Migrator().DropTable`。不得通过 AutoMigrate、raw SQL 或部署脚本隐式删除。

## Guard 与 CI 防回流

| 防回流点                 | 责任文件                                        | 覆盖范围                                                            | 对应任务         |
| ------------------------ | ----------------------------------------------- | ------------------------------------------------------------------- | ---------------- |
| Dashboard PageSpec guard | `scripts/dashboard_vnext_guard.sh`              | Formily/form-render、旧 PageSpec、旧 renderer、旧 DTO/selector 字段 | `A-001`          |
| CI Dashboard guard       | `.github/scripts/dashboard_guard.sh`            | workspace、Function UI、旧 API、注册侧展示字段                      | `A-001`、`H-004` |
| 注册侧字段拒绝           | `internal/function/registrationguard/reject.go` | UI、菜单、page、mapping、layout、pagination、route 字段             | `B-001`          |
| OpenAPI 上传字段拒绝     | `internal/api/openapi/service.go`               | OpenAPI presentation extension 与外部 `$ref`                        | `B-001`          |

## 删除前核对

- [ ] 对应替代任务已经通过其完整验收，包含浏览器 E2E。
- [ ] `bash "scripts/dashboard_vnext_guard.sh"` 返回 0。
- [ ] `bash ".github/scripts/dashboard_guard.sh"` 返回 0。
- [ ] 删除路径没有非 Dashboard 使用者；名称相似的业务组件必须单独确认。
- [ ] 涉及数据库时已有可恢复备份标识，并获得本次生产删除的明确确认。
- [ ] 删除后执行全量相关服务测试、前端 build 和 deployment dry-run。
