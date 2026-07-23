# P0 重构复审意见

审核日期：2026-07-23

结论：P0 仍未整体通过，但 2026-07-23 复修已经解决一批原阻断项。当前代码已经补了 `spec/normalizer/generator/Page API/Console API/FormilyPageRenderer` 的骨架，并修复了 Page/Console 状态一致性、动态菜单 locale、Function UI 旧协议回流等问题；剩余重点是 SDK registry v2 采集闭环、Page/Console 端到端测试、旧 Workspace/Entity 主链路清理和 scope 设计。

## 本次复修结果

- `internal/router/router.go` 与 `internal/svc/service_context.go` 已初始化 `PageSpecModel`、`PublishedPageSpecModel`、`PageVersionModel`，路由启动路径不再因 Page/Console model nil 崩溃。
- `internal/model/migration.go` 已将 `PageSpec`、`PublishedPageSpec`、`PageVersion` 接入 `MetaModels()`；但 PageSpec 暂无 `game_id/env` 字段，scope 隔离设计仍未完成。
- `internal/api/page/service.go` 已通过 `pageSpecFromModel/applyPageSpecToModel` 保存完整 PageSpec，保存草稿、发布、取消发布、回滚改为事务化；发布前会校验 bindings、函数存在、operationKind、placement、labels。
- `PublishedPageSpec` 已有 `active/unpublished_at` 语义，取消发布会 deactivate 运行快照，`Console API` 只读取 active 最新快照。
- `web/src/services/console.ts` 已 unwrap `{items}` 和 `{page}`；`web/src/app.tsx` 不再使用 `any` 构造动态菜单，动态项显式 `locale:false`，并把当前 `getLocale()` 传给 `/console/menu`。
- `internal/api/console/service.go` 运行菜单不再推断 category；缺少 PageSpec category 的发布快照会被跳过，分类必须在保存/发布阶段确定。
- `web/src/components/FormilyPageRenderer/index.tsx` 已具备 QueryForm/DataTable/ActionButton/TaskTimeline/ChartPanel 的最小执行能力，不再是纯 TODO 空壳。
- Function UI 主链路已收敛为 Formily `schema`：API response/history/rollback 不再暴露 `Layout/Components`，更新接口提交 `ui/layout/components` 直接 400，logic 层 versioning 只保存/恢复 `schema` 并清理旧 metadata。

已验证：

```bash
GOCACHE=/tmp/croupier-go-cache go test ./internal/dashboard/... ./internal/api/resource/... ./internal/api/console/... ./internal/api/page/... ./internal/api/function/... ./internal/logic/function/... ./internal/router/...
pnpm --dir "web" exec eslint "src/app.tsx" "src/services/console.ts" "src/pages/Console" "src/components/FormilyPageRenderer" "src/types/dashboard.ts"
```

## 剩余阻断

- `internal/api/function/dto.go` 仍有历史 `interface{}` DTO，Function UI 主链路已修，但 Function API 整体强类型收敛未完成。
- SDK registry runtime `sess.Functions` 的 v2 metadata 采集仍需端到端验收；Resource API 已复用 collector/normalizer，但 builder -> provider proto -> server registry -> Resource API 缺测试。
- `internal/logic/function/descriptors_logic.go` 仍保留旧 descriptor 推断逻辑，需要进一步把旧推断降级为 diagnostics，避免成为可发布事实。
- PageSpec 目前属于 meta scope，缺少 `game_id/env` 或明确的全局唯一策略；多游戏/多环境下同 `pageKey` 冲突仍需设计。
- 旧 Workspace/Entity 页面、服务、路由仍大量存在；运行控制台主路径已断开，但系统菜单仍有 `/system/functions/workspaces/:objectKey` 等旧入口，P1/P2 必须删除或重写。
- 缺少 Page publish/unpublish/rollback、Console menu、Function UI 旧字段 400、Resource API v2 采集的单测/集成测试。

## 当前已推进的部分

- `proto/croupier/sdk/v1/provider.proto` 已新增 descriptor v2 字段，包括 display labels、`operation_kind`、`placement`、`page_hint`、`extensions`。
- `internal/dashboard/spec` 已新增强类型模型，覆盖 Function/Resource/Operation/Page/PublishedPage/Menu。
- `internal/dashboard/normalizer` 已建立，placement camelCase 归一化问题看起来已修复。
- `internal/dashboard/generator` 已新增，`internal/api/resource/service.go` 已调用 `generator.GenerateForResource`。
- `internal/api/page` 不再是空实现，已有草稿、发布、版本、回滚雏形。
- `internal/api/console` 不再是空实现，已尝试从 `published_page_specs` 读取菜单和页面。
- 前端运行路由已改为 `/console/:categoryKey/:pageKey`，`Console/Page.tsx` 已接 `FormilyPageRenderer`，旧 `Console/Workspace.tsx` 已删除。

## 阻断项

### P0-6/P0-7：路由启动路径下 Page/Console 会空指针

证据：

- `internal/router/router.go` 手工构造 `svc.ServiceContext` 时没有初始化 `PageSpecModel`、`PublishedPageSpecModel`、`PageVersionModel`。
- 同一文件已经注册 `registerResourceRoutes`、`registerConsoleRoutes`、`registerPageRoutes`。
- `internal/svc/service_context.go` 的 `NewServiceContext` 分支确实初始化了这 3 个 model，但 `router.RegisterRoutes` 没走该构造函数。

影响：

- 走 `router.RegisterRoutes` 启动时，访问 `/api/v1/workspaces/pages`、`/api/v1/console/menu`、`/api/v1/console/pages/:pageKey` 会因为 nil model 崩溃。
- 即使新增 API 编译通过，部署后也不能稳定运行。

必须修复：

- 统一 `ServiceContext` 构造路径，禁止在 `router.go` 里遗漏新 model。
- 最小修复是在 `router.RegisterRoutes` 的手工构造处补 `PageSpecModel/PublishedPageSpecModel/PageVersionModel`。
- 更好的修复是收敛为单一构造函数，避免以后新增依赖继续漏注入。

### P0-6/P1-4：新 PageSpec 表没有接入 migration

证据：

- `internal/model/migration.go` 的 `MetaModels()` 没有 `&PageSpec{}`、`&PublishedPageSpec{}`、`&PageVersion{}`。
- `autoMigrateAllModels()` 只组合 `MetaModels()` 和 `GameModels()`，因此新表不会自动创建。

影响：

- Page 工作台和 Console API 依赖的 `page_specs`、`published_page_specs`、`page_versions` 表不存在。
- 首次部署或新环境会直接运行失败。

必须修复：

- 明确 PageSpec 属于 meta scope 还是 game/env scope；按当前模型没有 `game_id/env` 字段，只能先放 `MetaModels()`。
- 如果目标是按全局 scope 隔离，必须在 model 增加 scope 字段并补唯一索引，否则同一个 `pageKey` 会跨游戏/环境冲突。

### P0-6：Page 版本和回滚语义错误

证据：

- `internal/api/page/service.go` 的 `SaveDraft` 写版本时 `SpecJSON: string(req.Schema)`，只保存了 Formily schema，不是完整 PageSpec。
- `PageVersion.SpecJSON` 注释写的是 Full PageSpec JSON，代码和模型语义冲突。
- `Rollback` 直接 `p.SchemaJSON = targetVersion.SpecJSON`，如果未来版本保存完整 PageSpec，会把完整 PageSpec 塞进 schema；如果继续只存 schema，又无法恢复 title/category/bindings/metadata。
- `buildPageSpecJSON` 没带 `Description` 和 `Metadata`，发布快照会丢字段。
- `Preview` 也没有返回 `Description` 和 `Metadata`。

影响：

- 版本不可审计，不可完整恢复。
- 回滚后页面结构可能损坏。
- 发布快照不是完整 PageSpec，运行控制台拿到的数据会缺字段。

必须修复：

- 增加 `pageSpecFromModel(*model.PageSpec) spec.PageSpec` 和 `applyPageSpecToModel(*model.PageSpec, spec.PageSpec)` 两个单一转换入口。
- `PageVersion.SpecJSON` 和 `PublishedPageSpec.SpecJSON` 都保存完整 PageSpec。
- `Rollback` 反序列化完整 PageSpec 后恢复 title/category/schema/bindings/metadata/description。
- 保存版本失败不能静默忽略，必须返回错误或事务回滚。

### P0-6/P0-7：取消发布没有让运行菜单消失

证据：

- `Unpublish` 只把 `page_specs.status` 改为 `draft`。
- `PublishedPageSpecModel.ListLatest` 直接从 `published_page_specs` 取每个 page 的最新快照，没有检查 draft 状态或 active/unpublished 标记。

影响：

- 用户取消发布后，`GET /api/v1/console/menu` 仍会返回该页面。
- `todo.md` 的“取消发布 Page 后刷新页面，菜单消失”验收不成立。

必须修复：

- 给发布快照增加 active/unpublished 语义，或让 Console API join `page_specs.status = published`。
- 更推荐增加显式 `UnpublishedAt/Active` 或 publish state 表，避免仅靠草稿状态推导运行态。

### P0-7：前端 Console service 与后端响应结构不匹配

证据：

- 后端 `ConsolePagesResponse` 返回 `{items: [...]}`，`ConsolePageResponse` 返回 `{page: {...}}`。
- `web/src/services/console.ts` 的 `listPublishedPages` 按 `PublishedPageSpec[]` 接收。
- `web/src/services/console.ts` 的 `getPublishedPage` 按 `PublishedPageSpec` 接收。

影响：

- `Console/Page.tsx` 实际拿到的是 `{page: ...}` 包装对象，传给 `FormilyPageRenderer` 后 `page.schema` 为空。
- 已发布页面会显示“PageSpec schema 缺失或格式错误”，运行控制台不可用。

必须修复：

- 前端 service unwrap `{items}` 和 `{page}`。
- 加基础单测或至少类型约束，避免后端 DTO 包装结构变更时静默错用。

### P0-7/P0-1：动态菜单仍有类型和 locale 问题

证据：

- `web/src/app.tsx` 的 `buildMenuFromConsoleSpec(defaultMenuData: any[], ...): any[]` 和 `(child: any)` 仍使用 `any`。
- 动态菜单项没有显式设置 `locale: false`，只是在后端 `ConsoleMenuItem` 中有 `Locale: false`，前端转换成 ProLayout menu 时丢了该字段。
- `buildMenuFromConsoleSpec` 写死优先 `zh-CN/en-US`，没有用当前用户语言或服务端 `lang` 参数。

影响：

- 违反 `todo.md` 中 TS 禁止核心链路 `any` 的硬约束。
- ProLayout 可能继续按静态 locale key 解析动态 name，动态分类国际化不稳定。
- 用户切换语言时动态菜单不一定符合预期。

必须修复：

- 定义明确的 `RuntimeMenuItem` 类型，删除本函数内 `any`。
- 动态 category/page menu item 显式 `locale: false`。
- `getConsoleMenu` 传当前语言，或前端按当前语言解析 `LocalizedText`。

### P0-8：FormilyPageRenderer 仍是占位，不能执行函数

证据：

- `web/src/components/FormilyPageRenderer/index.tsx` 的 `DataTable` 只有 TODO div，不查询、不分页。
- `DetailPanel`、`ActionButton`、`TaskTimeline`、`ChartPanel` 都是 TODO div。
- `FormilyPageRendererProps` 定义了 `onQuery/onAction/onTaskStart`，但组件内部没有使用这些 callback。
- `web/src/pages/Console/Page.tsx` 的 callback 只是 `console.log` 和 TODO。

影响：

- 即使菜单和 PublishedPageSpec 正确，页面也只能渲染空壳。
- Entity/Operation/Task/Report 四类最小端到端场景均不成立。

必须修复：

- `QueryForm` 至少能提交查询条件并触发 `onQuery`。
- `DataTable` 至少用显式 `itemsField/totalField/pageField/pageSizeField` 调用 query function 并展示表格。
- `ActionButton` 至少能触发函数调用，危险操作按 risk 做确认。
- Task/Report 可以先标 `needs_review`，但不能在 P0 标为已完成。

### P0-2/P0-3/P0-5：Resource API 没有读取 SDK 注册的 v2 语义

证据：

- `internal/api/resource/service.go` 采集 runtime registry 时只设置 `ID/Version/Enabled`。
- 同一路径没有从 `meta` 中提取 `Category/Entity/Operation/OperationKind/Placement/PageHint/Display labels/InputSchema/OutputSchema`。
- OpenAPI 分支只提取少量 extension，没有提取 `x-category-display/x-entity-display/x-operation-display`，也没提取 request/response schema。

影响：

- 真实 SDK 注册的函数进入 Resource API 后会缺 entity/operationKind/placement，无法生成资源和默认页面。
- OpenAPI 导入的函数 labels/schema 不完整，生成的 PageSpec 质量会错误降级。
- `player.list/player.get/player.ban` 这类关键场景在 Resource API 链路上不成立。

必须修复：

- 找到 registry `sess.Functions` 的真实 metadata 类型，把 descriptor v2 字段完整映射进 `normalizer.DescriptorInput`。
- OpenAPI 分支复用 descriptor 解析逻辑，避免 Function descriptors 和 Resource API 两套解析结果不一致。
- 补 builder -> provider proto -> server registry -> Resource API 的测试。

### P0-3/P0-4：Function descriptor/UI 旧链路仍未完全收敛

证据：

- `internal/logic/function/descriptors_logic.go` 仍保留 `inferCategory`，并在 OpenAPI 缺字段时从函数 ID 推 entity/operation。
- `internal/logic/function/formily_schema.go` 仍大量使用 `map[string]interface{}` 做 schema 遍历；这类代码可以作为 JSON 边界存在，但不应继续扩散到核心 DTO。
- `internal/api/function/dto.go` 旧 Function UI DTO 仍需要复核，之前的 `Layout/Components` 风险没有在本轮完全消除证据。

影响：

- Descriptor normalizer 虽然存在，但旧推断逻辑仍可能影响函数目录和后续页面生成。
- “不猜字段/不猜页面类型/不保留多套 UI 协议”的约束还没有形成 guard。

必须修复：

- Descriptor 输出统一走 normalizer，旧推断只能作为 diagnostics，不能作为可发布事实。
- Function UI 保存接口只接受 Formily schema，旧 layout/components/fields 直接 400。
- 增加 CI guard，禁止旧 UI 协议回流。

### P0-1/P0-5/P2：旧 Workspace/Entity 主链路仍大量存在

证据：

- `web/config/routes.ts` 仍有 `/system/functions/workspaces`、`/system/functions/workspaces/:objectKey`、`/system/functions/workspace-editor/:objectKey`。
- `web/src/components/WorkspaceRenderer/*`、`web/src/services/workspaceConfig.ts`、`web/src/types/workspace.ts` 仍存在。
- `web/config/workspaces/*` 和 `web/src/config/workspaces/*` 仍有 `WorkspaceConfig/objectKey` 静态配置。
- `internal/api/workspace/*`、`internal/model/workspace_config.go` 仍存在。

影响：

- P0 运行控制台路径虽然开始迁移，但系统菜单仍会引导用户进入旧“对象工作台/WorkspaceConfig”概念。
- 后续 agent 很容易继续沿旧模型改，造成新旧协议并行。

必须修复：

- P0 可以允许旧代码暂存，但必须从运行控制台主路径彻底断开，并在 `todo.md` 标为 P1/P2 待清理，不应把 P0-1/P0-5 标完成。
- P1 开始时重构系统菜单为“资源/操作”和“Page 工作台”，删除旧 objectKey 编辑器入口。

## todo.md 状态修正建议

应立即把顶部执行进度改为：

- P0-1：进行中，强类型存在但旧模型未冻结。
- P0-2：进行中，proto 有 v2，但 SDK/registry/server Resource 链路未验收。
- P0-3：进行中，normalizer 存在但旧 descriptor 逻辑未完全收敛。
- P0-4：进行中，Function UI 是否唯一 Formily 未完成验收。
- P0-5：进行中，Resource API/generator 存在但 SDK v2 采集断裂。
- P0-6：未通过，migration、版本、回滚、取消发布阻断。
- P0-7：未通过，Console API/前端 service/menu locale 仍有阻断。
- P0-8：未通过，renderer 仍是空壳。
- P0-9：进行中，generator 存在但四类端到端和 blocked 语义未验收。
- P1/P2：不得标完成，除非对应旧菜单/权限/迁移/guard 已真实落地。

## 建议修复顺序

1. 先修 ServiceContext 构造和 migration，保证新 API 不崩溃且表存在。
2. 修 PageSpec 完整序列化、版本、回滚、取消发布可见性，补事务。
3. 修前端 `console.ts` unwrap、`app.tsx` 类型和动态 `locale:false`。
4. 修 Resource API descriptor 采集，确保 SDK/OpenAPI v2 字段进入 normalizer。
5. 将 `FormilyPageRenderer` 做到最小可执行：query/table/action 三件事先闭环。
6. 补 Page publish 校验：bindings 存在、函数存在、operationKind/placement/labels/Formily schema 缺失时不能发布。
7. 再做系统菜单和旧 Workspace/Entity 清理。
8. 最后加 CI guard 和四个端到端场景测试。

## 复验命令

基础后端：

```bash
GOCACHE=/tmp/croupier-go-cache go test ./internal/dashboard/... ./internal/api/resource/... ./internal/api/console/... ./internal/api/page/... ./internal/router/...
```

前端关键路径：

```bash
pnpm --dir "web" exec eslint "src/app.tsx" "src/services/console.ts" "src/pages/Console" "src/components/FormilyPageRenderer" "src/types/dashboard.ts"
```

旧链路残留检查：

```bash
rg -n "workspaceConfigs|buildConsoleMenuData|menuDataRender|menu\\.ControlConsole\\.category|:objectKey" "web/src/app.tsx" "web/config/routes.ts" "web/src/pages/Console" "web/src/services"
rg -n "WorkspaceRenderer|WorkspaceConfig|WorkspaceLayout|TabLayout|workspaceConfig" "web/src"
rg -n "defaultMenu|menuSource|metadata\\.menu|inferCategory|applyEntityMenuDefaults" "internal/logic/function" "internal/api/resource"
```

新链路可用性检查：

```bash
rg -n "PageSpecModel|PublishedPageSpecModel|PageVersionModel" "internal/router" "internal/svc" "internal/model/migration.go"
rg -n "GenerateForResource|operation_kind|operationKind|placement|category_display|categoryDisplay" "internal/api/resource" "internal/dashboard" "proto" "sdks"
```

最小端到端验收：

- 注册 `player.list/player.get/player.ban` 后，Resource API 返回 `player` 和 3 个 operation。
- `GET /api/v1/resources/player/pages/generated` 返回 `player.manage`，且缺字段时给 diagnostics。
- 保存并发布 `player.manage` 后，`GET /api/v1/console/menu` 返回动态分类和页面。
- 打开 `/console/{categoryKey}/player.manage` 能加载 `PublishedPageSpec`，并至少完成查询/分页/行操作最小链路。
- 取消发布后，菜单和直接访问页面都不可见。
