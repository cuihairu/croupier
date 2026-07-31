# P0-2 Dashboard 旧代码盘点与删除清单

更新时间：2026-07-31

> 本清单是 P0-2 的执行产物，记录旧模型代码的位置、调用图、替代模块和删除计划。

## 1. 删除清单

### 1.1 Formily 相关代码

| 文件 | 用途 | 调用者 | 替代模块 | 删除优先级 |
| --- | --- | --- | --- | --- |
| `internal/logic/function/formily_schema.go` | `supportedFormilyComponents` map | 无运行依赖 | JSON Schema + FormPresentationSpec | P4-4 |
| `internal/dashboard/spec/types.go:28-30` | `FormilySchema` 类型定义 | PageSpec.Schema | JSONSchema + FormPresentationSpec | P3-1 |
| `internal/model/page_spec.go` | FormilySchema 字段 | PageSpec DTO | 强类型 PageSpec | P3-1 |

### 1.2 旧模型结构

| 文件 | 类型/结构 | 替代 | 删除阶段 |
| --- | --- | --- | --- |
| `internal/dashboard/spec/types.go:205-230` | `FunctionSpec` | `FunctionContract` | P1-1 |
| `internal/dashboard/spec/types.go:239-256` | `ResourceSpec` / `ResourceCategorySpec` | `ResourceCapability` + `CapabilitySemantics` | P1-2 |
| `internal/dashboard/spec/types.go:264-275` | `OperationSpec` | `FunctionContract` + capability | P1-2 |
| `internal/dashboard/spec/types.go:284-304` | `PageSpec` (含 FormilySchema) | 强类型 PageSpec vNext | P3-1 |
| `internal/dashboard/spec/types.go:429-435` | `GeneratedPageSpec` | `PageProposal` | P2-1 |

### 1.3 旧页面 Schema Validator

| 文件 | 内容 | 替代 |
| --- | --- | --- |
| `internal/api/page/schema_validator.go` | `pageComponentContracts` (ConsolePage, QueryForm, DataTable, etc.) | 强类型 PageSpec 校验 |

### 1.4 旧 Generator / Normalizer

| 文件 | 用途 | 替代 |
| --- | --- | --- |
| `internal/dashboard/generator/generator.go` | 当前页面生成逻辑 | P2 PageProposal 生成器 |
| `internal/dashboard/normalizer/normalizer.go` | 描述符规范化 | P1 CapabilitySemantics 构建 |
| `internal/dashboard/descriptors/collector.go` | 描述符收集 | P1 FunctionContract 收集 |

### 1.5 旧 Function Form 逻辑

| 文件 | 用途 | 替代 |
| --- | --- | --- |
| `internal/logic/function/function_form_logic.go` | 函数表单逻辑 | JSON Schema + ProForm |
| `internal/logic/function/function_form_update_logic.go` | 函数表单更新 | JSON Schema + ProForm |
| `internal/logic/function/function_form_rollback_logic.go` | 函数表单回滚 | PageSpec 版本管理 |
| `internal/logic/function/function_form_resolver.go` | 函数表单解析 | JSON Schema resolver |

## 2. 保留模块

| 模块 | 路径 | 保留原因 |
| --- | --- | --- |
| Scope 隔离 | `internal/model/` | `game_id + env` 全局上下文 |
| PageVersion | `internal/model/page_version.go` | 草稿 revision 管理 |
| PublishedPage | `internal/model/published_page.go` | 发布快照 |
| ConsoleMenu | `internal/console/` | 动态菜单 |
| Console execute | `internal/api/console/` | binding execute API |
| OpenAPI Source | `internal/openapi/` | Provider binding |
| Audit/OTel | `internal/audit/`, `internal/otel/` | 审计链 |

## 3. 删除顺序

1. **P1**：替换 FunctionSpec → FunctionContract，删除 ResourceSpec/OperationSpec
2. **P2**：替换 GeneratedPageSpec → PageProposal
3. **P3**：替换 PageSpec 为强类型 vNext，删除 FormilySchema
4. **P4**：删除旧 Page renderer 和表单 runtime
5. **P7**：物理清理所有旧代码

## 4. 验收条件

- [ ] 每个删除项有替代模块和测试
- [ ] 无"暂时保留"项
- [ ] 删除前替代路径通过 E2E
