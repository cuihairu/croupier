# P0-2 删除清单

更新时间：2026-08-02

> 本文档记录 Dashboard vNext 重构中需要删除的旧模型代码。每项必须有 owner、替代模块、测试和删除 PR。

## 删除清单

### 1. 前端旧组件

| 文件/目录 | 说明 | 使用者 | 替代模块 | 删除条件 | 状态 |
|-----------|------|--------|----------|----------|------|
| `web/src/components/page-schema/*` | 旧组件树 Page schema helper | `Assignments/PageRenderer.tsx` | 新 UI helper（待建） | P7-b 替代路径通过 E2E | ⏳ 待删除 |
| `web/src/pages/PageStudio/index.tsx` | Page Studio（当前已接入新模型） | Console | 保留 | N/A | ✅ 已重写 |

### 2. 后端旧代码

| 文件/目录 | 说明 | 替代模块 | 删除条件 | 状态 |
|-----------|------|----------|----------|------|
| 无 | 后端旧 FunctionForm* DTO 已删除 | N/A | N/A | ✅ 已完成 |
| 无 | PageStudioV2 已删除 | N/A | N/A | ✅ 已完成 |
| 无 | ApprovalPageRenderer 已删除 | N/A | N/A | ✅ 已完成 |

### 3. 旧数据库列/表

| 表/列 | 说明 | 替代 | 删除条件 | 状态 |
|-------|------|------|----------|------|
| 无 | 当前数据库无旧页面配置表 | `page_specs`/`page_versions`/`published_pages` 新表（待建） | P1/P3-a 新表通过 AutoMigrate | ⏳ 待新建 |

> 说明：当前数据库 schema 中无 `page_schema`、`page_config`、`workspace_config` 等旧表。新页面模型表将在 P1/P3-a 阶段通过 GORM AutoMigrate 创建。

### 4. 旧 CI 配置

| 文件 | 说明 | 替代 | 删除条件 | 状态 |
|------|------|------|----------|------|
| 无 | CI 已更新为新模型检查 | N/A | N/A | ✅ 已完成 |

## 保留模块清单

| 模块 | 说明 | Owner |
|------|------|-------|
| `scope` | `game_id + env` 作用域隔离 | 核心 |
| `PageVersion` | 页面版本管理 | Dashboard |
| `PublishedPage` | 已发布页面快照 | Dashboard |
| `ConsoleMenu` | 动态菜单生成 | Dashboard |
| `Console execute` | 页面执行 API | Dashboard |
| `OpenAPI Source` | OpenAPI 导入 | 平台 |
| `Audit/OTel` | 审计与可观测性 | 核心 |

## 删除前检查清单

- [ ] 替代路径已通过 E2E 测试
- [ ] 旧文件无其他模块依赖
- [ ] 已导出历史数据备份方案
- [ ] 已更新 CI allowlist
- [ ] 已创建删除 PR 并获得批准

## 注意事项

1. **禁止自动删除生产数据**：任何生产数据删除必须另行取得明确确认
2. **禁止无登记遗留**：所有需要删除的模块必须登记在本文档
3. **替代路径先行**：删除前必须先新增新模型替代路径并通过测试
