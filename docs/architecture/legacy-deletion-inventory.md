---
title: 旧模型删除清单
icon: trash-can
order: 11
category:
  - 系统架构
tag:
  - Dashboard
  - 重构
  - 清理
---

# 旧模型删除清单

> **状态**：Complete -- `todo.md` 中 `A-002` 与 `H-001`~`H-005` 已全部完成并验收，本清单不再是待执行任务依据，转为**历史记录 + 防回流索引**。发现旧模型回流时，先在本清单登记条目，再引用 guard 项创建最小修复任务。

## 使用规则

- 「已删除」表示物理删除 + guard 防回流同时成立。
- 新增发现的旧模型残留，必须先登记进本清单再删除，不得绕过清单直接删。
- 生产部署执行 `CleanupAllLegacy` 前，须按 `H-005` 约定取得单独明确确认并完成备份校验（见 `internal/model/migration_legacy_cleanup.go` 的 `LegacyCleanupReport` dry-run）。

## 状态约定

- **已删除**：代码库中已物理移除，且有 guard 条目防回流。
- **待复核**：代码库中已无定义，但生产环境/历史数据中可能仍存在，需 H-005 阶段核查。

## 防回流证据来源

- `scripts/dashboard_vnext_guard.sh`（下称 **vNext guard**）：CI lint job 调用，守护 PageSpec 唯一模型。
- `.github/scripts/dashboard_guard.sh`（下称 **model guard**）：`ci-dashboard.yml` 调用，`removed_paths` 数组与符号扫描防止旧路径回流。

---

## H-001 Formily 与 form-render 清理

> 删除任务：`H-001`；Depends: `A-001`, `E-001`。整体 E2E 前置：`E-001` SchemaFormRenderer 表单链路浏览器 E2E 通过，web build 通过。

| #       | 清单项                                                    | 类型   | 状态   | 替代任务 | Owner | 删除前置条件（E2E）                     | 删除任务 | 防回流证据                                                |
| ------- | --------------------------------------------------------- | ------ | ------ | -------- | ----- | --------------------------------------- | -------- | --------------------------------------------------------- | ----------- | ------------------ | ------------------- | --------------- | --------------- | ------------------------ |
| H-001-1 | `web/package.json` 中 `@formily/*` 依赖                   | 依赖   | 已删除 | `E-001`  | H-001 | SchemaFormRenderer(@rjsf) 表单 E2E 通过 | `H-001`  | vNext guard「Formily runtime dependency absent」          |
| H-001-2 | `web/package.json` 中 `form-render` 依赖                  | 依赖   | 已删除 | `E-001`  | H-001 | 同上                                    | `H-001`  | vNext guard「legacy Formily/FunctionForm runtime absent」 |
| H-001-3 | `web/pnpm-lock.yaml` 中 formily/form-render lockfile 条目 | 依赖   | 已删除 | `E-001`  | H-001 | 同上                                    | `H-001`  | lockfile 零命中（A-002 勘察记录）                         |
| H-001-4 | `web/src/components/formily/`                             | 源文件 | 已删除 | `E-001`  | H-001 | 同上                                    | `H-001`  | vNext guard `assert_file_absent`                          |
| H-001-5 | `web/src/components/FormilyPageRenderer/`                 | 源文件 | 已删除 | `E-001`  | H-001 | 同上                                    | `H-001`  | vNext guard `assert_file_absent`                          |
| H-001-6 | `web/src/components/FunctionFormManager/`                 | 源文件 | 已删除 | `E-001`  | H-001 | 同上                                    | `H-001`  | vNext guard `assert_file_absent`                          |
| H-001-7 | `web/src` 内 Formily/form-render 符号与文案               | 引用   | 已删除 | `E-001`  | H-001 | 同上                                    | `H-001`  | vNext guard rg 扫描 `@formily/                            | form-render | components/formily | FunctionFormManager | generateFormily | validateFormily | BuildFallbackFormSchema` |

## H-002 旧 Page renderer 与旧运行 registry

> 删除任务：`H-002`；Depends: `A-002`, `E-002`~`E-007`。整体 E2E 前置：全量浏览器 E2E（`I-*` 验收矩阵）通过，且 vNext guard 通过；Console 仅使用 vNext PageRenderer，无 fallback renderer 或 feature flag 双路径。

| #        | 清单项                                                        | 类型     | 状态   | 替代任务        | Owner | 删除前置条件（E2E） | 删除任务 | 防回流证据                                                                                                    |
| -------- | ------------------------------------------------------------- | -------- | ------ | --------------- | ----- | ------------------- | -------- | ------------------------------------------------------------------------------------------------------------- |
| H-002-1  | `web/src/components/PageGenerator/`                           | 源文件   | 已删除 | `E-002`~`E-007` | H-002 | 全量浏览器 E2E 通过 | `H-002`  | model guard `removed_paths` + 符号扫描                                                                        |
| H-002-2  | `web/src/components/WorkspaceRenderer/`                       | 源文件   | 已删除 | `E-002`~`E-007` | H-002 | 同上                | `H-002`  | model guard `removed_paths` + 符号扫描                                                                        |
| H-002-3  | `web/src/components/XUISchema.tsx`                            | 源文件   | 已删除 | `E-002`~`E-007` | H-002 | 同上                | `H-002`  | model guard `removed_paths`                                                                                   |
| H-002-4  | `web/src/components/FunctionUIManager/`                       | 源文件   | 已删除 | `E-005`         | H-002 | 同上                | `H-002`  | model guard `removed_paths` + 符号扫描                                                                        |
| H-002-5  | `web/src/components/FunctionFormRenderer/`                    | 源文件   | 已删除 | `E-001`         | H-002 | 同上                | `H-002`  | model guard `removed_paths` + 符号扫描                                                                        |
| H-002-6  | `web/src/pages/WorkspaceEditor/`                              | 源文件   | 已删除 | `D-002`         | H-002 | 同上                | `H-002`  | model guard `removed_paths` + web/docs 扫描                                                                   |
| H-002-7  | `web/src/pages/Workspaces/`                                   | 源文件   | 已删除 | `E-002`         | H-002 | 同上                | `H-002`  | model guard `removed_paths`                                                                                   |
| H-002-8  | `web/src/pages/Entities/`                                     | 源文件   | 已删除 | `E-002`         | H-002 | 同上                | `H-002`  | model guard `removed_paths`                                                                                   |
| H-002-9  | `web/src/services/workspaceConfig.ts`                         | 源文件   | 已删除 | `D-001`         | H-002 | 同上                | `H-002`  | model guard `removed_paths` + 符号扫描                                                                        |
| H-002-10 | `web/src/types/workspace.ts`                                  | 源文件   | 已删除 | `D-001`         | H-002 | 同上                | `H-002`  | model guard `removed_paths`                                                                                   |
| H-002-11 | `web/tests/workspace/`                                        | 测试     | 已删除 | `I-*`           | H-002 | 同上                | `H-002`  | model guard `removed_paths`                                                                                   |
| H-002-12 | 旧前端路由（Workspace/Entities 等旧工作台入口）               | 路由     | 已删除 | `D-003`         | H-002 | 同上                | `H-002`  | `web/config/routes.ts` 仅注册 `/console/:categoryKey/:pageKey` 等 vNext 路由（vNext guard `assert_contains`） |
| H-002-13 | `objectKey` 运行时引用                                        | 引用     | 已删除 | `D-003`         | H-002 | 同上                | `H-002`  | model guard「objectKey must not be used in Page/Console runtime」                                             |
| H-002-14 | Console 直接 `/functions/:id/invoke` 调用                     | API 调用 | 已删除 | `E-005`         | H-002 | 同上                | `H-002`  | model guard「console runtime must execute page bindings」                                                     |
| H-002-15 | `web/docs/` 下 8 个旧 workspace 文档（DEMO/WORKSPACE\_\* 等） | 文档     | 已删除 | `G-*`           | H-002 | 同上                | `H-002`  | model guard `removed_paths` + web/docs 符号扫描                                                               |

## H-003 旧 Page schema validator/editor

> 删除任务：`H-003`；Depends: `A-002`, `D-002`, `F-004`。整体 E2E 前置：浏览器 E2E 编辑/发布链路（PageStudio 强类型 DTO + 语义面板）通过；`rg` 无旧 editor/validator 引用。

| #        | 清单项                                                                                                                                        | 类型   | 状态   | 替代任务 | Owner | 删除前置条件（E2E）      | 删除任务 | 防回流证据                                                         |
| -------- | --------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------ | -------- | ----- | ------------------------ | -------- | ------------------------------------------------------------------ |
| H-003-1  | `web/src/pages/PageStudioV2/`                                                                                                                 | 源文件 | 已删除 | `D-002`  | H-003 | 编辑/发布浏览器 E2E 通过 | `H-003`  | vNext guard `assert_file_absent` + 符号扫描 `PageStudioV2`         |
| H-003-2  | `web/src/pages/PageStudio/PageSchemaEditor.tsx`                                                                                               | 源文件 | 已删除 | `D-002`  | H-003 | 同上                     | `H-003`  | vNext guard `assert_file_absent` + 符号扫描 `PageSchemaEditor`     |
| H-003-3  | `web/src/types/dashboard-vnext.ts`                                                                                                            | 源文件 | 已删除 | `D-001`  | H-003 | 同上                     | `H-003`  | vNext guard `assert_file_absent` + 符号扫描 `dashboard-vnext`      |
| H-003-4  | `web/src/services/dashboard-vnext.ts`                                                                                                         | 源文件 | 已删除 | `D-001`  | H-003 | 同上                     | `H-003`  | vNext guard `assert_file_absent`                                   |
| H-003-5  | `internal/dashboard/generator/generator_v2.go`                                                                                                | 源文件 | 已删除 | `C-011`  | H-003 | 同上                     | `H-003`  | vNext guard `assert_file_absent` + 符号扫描 `PageSpecV2`           |
| H-003-6  | `internal/dashboard/descriptors/` 请求时 descriptor 收集器                                                                                    | 源文件 | 已删除 | `C-002`  | H-003 | 同上                     | `H-003`  | vNext guard `assert_dir_has_no_files` + `descriptors.Collect` 扫描 |
| H-003-7  | `internal/validation/entity.go` 及 `entity_test.go`、`entity_extra_test.go`                                                                   | 源文件 | 已删除 | `B-006`  | H-003 | 同上                     | `H-003`  | model guard `removed_paths` + validator 符号扫描                   |
| H-003-8  | `internal/function/converter/pack.go`                                                                                                         | 源文件 | 已删除 | `B-006`  | H-003 | 同上                     | `H-003`  | model guard `removed_paths` + `PackConverter` 符号扫描             |
| H-003-9  | `internal/function/uicontract/`                                                                                                               | 源文件 | 已删除 | `B-001`  | H-003 | 同上                     | `H-003`  | model guard `removed_paths` + `uicontract` 符号扫描                |
| H-003-10 | `internal/logic/function/function_u_i_*`（history/rollback/update/v2/versioning 共 6 个文件及测试）                                           | 源文件 | 已删除 | `D-004`  | H-003 | 同上                     | `H-003`  | model guard `removed_paths`（逐项登记）                            |
| H-003-11 | `internal/logic/function/ui_resolver.go` 及 `ui_resolver_test.go`                                                                             | 源文件 | 已删除 | `C-011`  | H-003 | 同上                     | `H-003`  | model guard `removed_paths`                                        |
| H-003-12 | 旧 validator 符号（`PackConverter/PackManifest/PackEntity/PackEntityOperation/ValidateEntityDefinition/validateUIConfig/validateOperations`） | 符号   | 已删除 | `B-006`  | H-003 | 同上                     | `H-003`  | model guard 符号扫描                                               |

## H-004 旧注册 UI 扩展、页面 API 与 DTO

> 删除任务：`H-004`；Depends: `B-001`, `H-002`, `H-003`。整体 E2E 前置：guard 通过；SDK parity、服务集成与浏览器 E2E 不依赖旧接口；注册与运行路径只剩 FunctionContract、CapabilitySemantics、Proposal、PublishedPageSpec。

| #       | 清单项                                                                                                                           | 类型           | 状态   | 替代任务 | Owner | 删除前置条件（E2E）                              | 删除任务 | 防回流证据                                                                                   |
| ------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------- | ------ | -------- | ----- | ------------------------------------------------ | -------- | -------------------------------------------------------------------------------------------- |
| H-004-1 | proto/SDK 注册侧 UI 字段（`category_display`、`entity_display`、`operation_display`、`operation_kind`、`page_hint`、`x-labels`） | proto/SDK 字段 | 已删除 | `B-001`  | H-004 | 注册边界目标测试：携带任一 UI 字段返回结构化错误 | `H-004`  | vNext guard + model guard 双重扫描；`internal/function/registrationguard/reject.go` 拒绝逻辑 |
| H-004-2 | functions 旧页面路由（`:id` 下的 `ui`、`ui/history`、`ui/rollback` 子路径）                                                      | API 路由       | 已删除 | `D-004`  | H-004 | SDK parity + 服务集成 + 浏览器 E2E 不依赖旧接口  | `H-004`  | model guard「function form API must use /form routes」                                       |
| H-004-3 | `internal/api/function` 中 `FunctionUI*` DTO/handler                                                                             | DTO            | 已删除 | `D-004`  | H-004 | 同上                                             | `H-004`  | model guard「function API layer must expose FunctionForm types」                             |
| H-004-4 | 旧 workspaces 配置后端路由（v1 API 下 workspaces 前缀）                                                                          | API 路由       | 已删除 | `D-003`  | H-004 | 同上                                             | `H-004`  | model guard「legacy workspace routes must not be referenced」                                |
| H-004-5 | `/metadata/functions/import/openapi` 快捷导入                                                                                    | API 路由       | 已删除 | `C-007`  | H-004 | 同上                                             | `H-004`  | model guard「OpenAPI uploads must use OpenAPI Source APIs」                                  |
| H-004-6 | `web/src/services/api/users.ts`、`web/src/services/api/roles.ts`                                                                 | 前端 service   | 已删除 | `D-005`  | H-004 | 同上                                             | `H-004`  | model guard `removed_paths` + import 扫描                                                    |
| H-004-7 | provider `/entities` 旧 API（`ProvidersEntities/openAPIDocEntities/aggregateEntities/x-entities`）                               | API/DTO        | 已删除 | `C-002`  | H-004 | 同上                                             | `H-004`  | model guard「provider API must expose resources」                                            |
| H-004-8 | C++ SDK VirtualObject/Component 注册 API                                                                                         | SDK API        | 已删除 | `B-003`  | H-004 | 同上                                             | `H-004`  | model guard C++ SDK 符号扫描                                                                 |
| H-004-9 | 旧后端 FunctionForm API（`BuildFallbackFormSchema/generateFormily/validateFormily`）                                             | API            | 已删除 | `E-001`  | H-004 | 同上                                             | `H-004`  | vNext guard「legacy backend FunctionForm APIs absent」                                       |

## H-005 旧页面表/列与历史数据

> 删除任务：`H-005`；Depends: `H-004`。整体前置：备份校验记录、迁移测试、全量 E2E、deployment dry-run，以及**单独明确的生产数据删除确认**。只能使用版本化清理函数通过 `db.Migrator().DropColumn/DropTable` 删除，禁止 AutoMigrate/raw SQL。

A-002 勘察结论：`internal/model` 当前仅保留 vNext 页面表 `page_specs`、`published_page_specs`、`page_versions`（`internal/model/page_spec.go`）；`database/schema.sql` 中无 `function_ui*`、`workspace*`、`page_schema*` 遗留表定义。以下条目为 H-005 阶段的复核对象：

| #       | 清单项                                                     | 类型 | 状态   | 替代任务        | Owner | 删除前置条件（E2E）                                                | 删除任务 | 防回流证据                               |
| ------- | ---------------------------------------------------------- | ---- | ------ | --------------- | ----- | ------------------------------------------------------------------ | -------- | ---------------------------------------- |
| H-005-1 | 生产库中可能残留的历史页面表（如旧 `function_ui*` 版本表） | 表   | 已删除 | `D-001`~`D-006` | H-005 | 备份校验 + 迁移测试 + 全量 E2E + deployment dry-run + 单独明确确认 | `H-005`  | `CleanupLegacyPageTables` 启动时自动清理 |
| H-005-2 | 现有表上的旧列（注册侧 UI 字段对应存储列，若存在）         | 列   | 已删除 | `B-001`         | H-005 | 同上                                                               | `H-005`  | `CleanupLegacyUIColumns` 启动时自动清理  |
| H-005-3 | 旧页面/旧 UI 历史数据行                                    | 数据 | 已删除 | `D-006`         | H-005 | 同上（生产数据删除必须单独确认）                                   | `H-005`  | DropTable 自动清除关联数据               |

## Verify

本清单的 Verify（对应 `todo.md` A-002）：

```bash
# 1. 逐项自查：每项均有替代任务、owner、E2E 前置条件、删除任务（本文表格列）
# 2. guard 通过
bash scripts/dashboard_vnext_guard.sh
```
