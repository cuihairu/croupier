# 上传即成页：契约与绑定正交化 TODO

设计依据：`docs/architecture/ui-generation-upload-pipeline.md`（D1–D7 决策不再变更）。

每个任务原子性：独立完成、独立迁移文件、独立测试、独立可提交。仅以下落地顺序依赖：
T3（execution_state 字段）→ T4/T6/T8；T2 → T5；T12（后端校验放宽）→ T13；其余互不依赖。

> 上一版「平台安全与可观测性补强 TODO」（T1 登录锁定 / T2 tokenVersion / T3 MFA / T4 Prometheus / T5 清理）已于 2026-09-02 全部完成，历史内容见 git 记录。

## P0 — 最小可用

### T1. 放开组合页 ≥2 区块约束

**目标**：单组件（单函数区块）页面可保存、发布、渲染、执行。

**改动点**：

- `internal/service/contract_service.go` `CreateCompositeProposal`：删除 `len(sections) < 2` 校验（保留 `pageKey == ""` 校验）
- `web/src/pages/PageStudio/CompositeEditor/index.tsx`：删除「组合页至少需要 2 个函数区块」前置拦截与对应 locale 文案（zh-CN/en-US 同步）
- `TemplateQuickStart`：移除「单节点模板不出现」过滤逻辑与注释
- 受影响的单测同步更新（compiler/integration 用例中依赖 ≥2 断言的部分）

**验收**：

- 新增单测：单区块 composite 提案可创建、可发布
- `go test ./internal/service/...` 与 `pnpm --dir web test` 通过
- 编辑器拖入 1 个函数组件即可保存并走完发布链，页面可执行

### T2. 契约重建后自动重建组件模板

**目标**：FunctionContract 落库/变更后自动触发 `RegenerateFromContracts`，手动「从契约重新生成」退化为兜底按钮。

**改动点**：

- `internal/service/contract_service.go`（或 contractsvc）契约重建完成处：异步/同事务调用 `component.Handler.RegenerateFromContracts`（注意包依赖方向，必要时抽公共接口到 service 层）
- 失败不阻塞契约重建主流程，记 warn 日志 + 审计字段
- ComponentTemplates 页「从契约重新生成」按钮保留，文案改为兜底语义

**验收**：

- 新增单测：契约 upsert 后 builtin 模板自动更新、regenerate 失败不阻塞主流程
- `go test ./internal/...` 通过

## P1 — 体验跃迁

### T3. FunctionContract 增加 execution_state 字段

**目标**：契约携带执行状态（D2），存量行默认 bound，行为与现状完全一致。

**改动点**：

- `internal/db/migrate/migrations/` 新增迁移：`function_contracts` 加 `execution_state VARCHAR(16) NOT NULL DEFAULT 'bound'`
- `internal/model/function_contract.go`：模型加 `ExecutionState` 字段（json tag `executionState`）
- digest/stale 判定**不**纳入该字段（显式注释 + 测试锁定）
- 所有契约 DTO 透传该字段

**验收**：

- 迁移测试：存量行升级后为 bound；新插入默认 bound
- digest 一致性测试：仅改 execution_state 不触发 stale
- `go test ./internal/model/... ./internal/service/...` 通过

### T4. 上传管线生成 unbound 契约（依赖 T3）

**目标**：OpenAPI 上传后，无运行时函数的 operation 直接生成 unbound FunctionContract，不再要求前置注册（D1/D4 前半）。

**改动点**：

- `internal/api/openapi/service.go`：新增 `createUnboundContractsForSource`——对 source 的全部 operation 走 `RebuildContractFromFunctionMeta` 等价入口（source=openapi，executionState=unbound）；operationId→functionId 用既有确定性映射
- `CreateSource`/`UpdateSource` 成功后同事务调用
- `CreateBinding` 的「显式绑定到运行时函数」路径与校验保持不变
- 已存在 bound 契约的 operation 不降级（幂等：仅不存在时创建 unbound）

**验收**：

- 新增单测：上传文档（无 agent）后契约落库且 executionState=unbound；重复上传幂等；已有 bound 契约不被覆盖
- `go test ./internal/api/openapi/...` 通过

### T5. 上传管线自动模板 + 提案 + 摘要响应（依赖 T2、T4）

**目标**：上传单请求完成全链生成，响应携带摘要与直达入口（D4 后半）。

**改动点**：

- `CreateSource`/`UpdateSource` 在契约落库后：调用 `RegenerateFromContracts`（T2 已联动则此处复用其结果计数）+ 触发现有 proposal rebuild
- 响应 DTO 扩展：`{ operations, contractsCreated, templatesUpdated, proposalsCreated, diagnostics }`（lowerCamelCase，遵守契约命名规范）
- 超大文档超时风险记入交付说明已知边界（>500 operations 另议异步化）

**验收**：

- 新增单测：摘要各计数与实际操作数一致；诊断透传
- `go test ./internal/api/openapi/...` 通过

### T6. 运行时注册自动绑定 unbound 契约（依赖 T3）

**目标**：agent/SDK 注册函数时自动将同 scope 同 functionId 的 unbound 契约置 bound（D3）。

**改动点**：

- `RebuildContractFromFunctionMeta`（source=sdk/runtime 路径）：命中既有 unbound 契约时更新 executionState=bound 并触发模板/提案 freshness 重算
- 审计事件 `openapi_source.binding_auto`（或复用现有契约事件加字段）

**验收**：

- 新增单测：unbound → 注册同名函数 → 自动 bound；functionId 不匹配时不误绑
- `go test ./internal/service/... ./internal/api/openapi/...` 通过

### T7. 前端：上传页摘要 + 编辑器直达（依赖 T5）

**目标**：上传 OpenAPI 后用户看到生成结果并可一键进入编辑器/收件箱。

**改动点**：

- `web/src/pages/OpenAPISources/index.tsx`：上传成功展示摘要 Modal（生成组件数/页面提案数/诊断）+「打开编辑器」「查看提案」CTA
- `web/src/services/api/openapi.ts`：响应类型同步扩展（禁止 any）
- 组件面板/模板库 unbound 物料加「未绑定」Tag（不置灰）

**验收**：

- 新增前端用例：摘要渲染、CTA 跳转、unbound 标记展示
- `pnpm --dir web run tsc` 0 错误、`pnpm --dir web test` 通过

### T8. 执行边界 executor_unbound（依赖 T3）

**目标**：unbound 函数的执行请求返回结构化错误，前端渲染空态，禁止静默失败。

**改动点**：

- binding execute 服务端路径（`internal/api/console/handler.go`）：契约 executionState=unbound → `409 { error: "executor_unbound" }`（遵守 API 响应契约）
- PageRenderer / SchemaFormRenderer 提交路径：识别该错误码渲染「未绑定执行器」空态 + 去绑定入口（入口在 T9 前可跳 OpenAPISources 页）
- 发布页禁止 mock 数据兜底

**验收**：

- 新增单测：unbound 执行返回 409 + 错误码；bound 不受影响
- 前端用例：错误态渲染
- `go test ./internal/api/console/...` 与 `pnpm --dir web test` 通过

## P1.5 — 菜单与文案 i18n 放宽（D7）

### T12. 发布校验放宽：默认名称必填、翻译可选

**目标**：`title`/`category.labels` 不再强制 zh-CN+en-US 双写；仅要求默认名称（系统默认语言）非空，en-US 与其他语言一律可选。

**改动点**：

- `internal/service/proposal_service.go`：`hasDefaultLocale` 双 key 判定改为「系统默认语言 key 非空，或任一非空值（向后兼容存量仅有 en-US 的页面）」；`title must include zh-CN and en-US locales` 错误文案同步更新
- `internal/dashboard/generator/`：`ensureBilingual` 强制双写改为单写系统默认语言（已有双 key 的不删除）
- `internal/dashboard/spec/page_publish_validation.go` 中同类 locale 校验同步放宽
- 前端 `web/src/locales/supported.ts`：`REQUIRED_LOCALES` 双必填常量降级为单一默认语言常量（读取方同步）

**验收**：

- 新增单测：仅 zh-CN 的 title/category.labels 可发布；仅 en-US 存量页面不被误拒；全空仍拒
- `go test ./internal/service/... ./internal/dashboard/...` 通过

### T13. LocalizedTextEditor 移除双必填警告（依赖 T12）

**目标**：唯一编辑组件回归「默认名称 + 可选翻译」语义：非默认语言缺失不 ⚠、不提示「无法发布」。

**改动点**：

- `web/src/components/LocalizedTextEditor/index.tsx`：移除 `missingRequired` 警告、下拉 ⚠ 标记、`contractHint` 中「zh-CN 与 en-US 为必填，缺失发布被拒」表述；仅默认语言缺失时提示
- `web/src/locales/{zh-CN,en-US}/component.ts` 相关文案同步
- 菜单/标题编辑入口（PageEditor 各页、PageStudio studio 导航面板）确认统一走该组件，无第二处手写 locale 输入
- 相关前端用例更新

**验收**：

- 新增/更新前端用例：仅默认语言有值时无警告；默认语言缺失时给出提示
- `pnpm --dir web run tsc` 0 错误、`pnpm --dir web test` 通过

## P2 — 闭环与治理

### T9. 编辑器内绑定抽屉（依赖 T3、T8）

**目标**：unbound 组件在画布属性面板内就地完成绑定，OpenAPISources 的 BindingModal 下沉复用。

**改动点**：

- CompositeEditor 属性面板：unbound 组件显示「执行器：未绑定 [去绑定]」→ 抽屉列出当前 scope 运行时函数 → 调现有 CreateBinding API
- 绑定成功即刷新契约状态与画布标记
- OpenAPISources 页保留为上传入口 + 诊断/绑定状态总览

**验收**：

- 新增前端用例：抽屉打开/绑定/状态刷新
- `pnpm --dir web test` 通过

### T10. 发布分级策略（D5）

**目标**：保存是否走提案审核由 env 级策略控制；dev 默认保存即发布，prod 保留审核。

**改动点**：

- 配置新增 `pages.publishReview: auto | required`（lowerCamelCase，含 env 级覆盖；`X-Env` 缺失按 required）
- composite 保存路径：auto 时跳过人工接受直接落 published_page_specs（快照/版本历史不变；error 级诊断仍拒绝发布）
- 配置示例与 `docs/architecture/config-layering.md` 同步

**验收**：

- 新增单测：auto/required 两路径；缺 env 从严
- `go test ./internal/...` 通过

### T11. E2E 与文档收尾（依赖 T1–T10、T12、T13）

**目标**：设计文档 §6 验收标准全部由真实浏览器 E2E 覆盖，三层文档同步。

**改动点**：

- `web/e2e/` real-dashboard 新增用例：无 agent 上传出组件/提案、单区块页发布执行、unbound 空态、自动绑定、dev 保存即发布
- 同步 `docs/architecture/dashboard-page-model.md`（executionState）、`docs/architecture/pagespec-protocol.md`（如 wire 有变化）、`docs/dashboard/composite-editor-v3.md`（≥2 约束移除、绑定抽屉）、`docs/architecture/ui-generation.md`（管线入口更新）
- **旧模型表述清除**（防止误导，与代码同 PR 落地）：
  - `openapi-sdk-descriptor-v2.md`「OpenAPI Source 与执行绑定」节重写为 unbound 契约模型，删除「上传只产生候选、绑定为前提」表述与 ⚠️ 横幅
  - `dashboard-page-model.md` 重写契约准入相关段落与「本地化必填契约」（改为默认名称必填、翻译可选），删除两处 ⚠️ 横幅
  - `localized-text-contract.md` 第 2 条归一输出改为「按输入归一 BCP47 key 原样透传」，删除 ⚠️ 横幅
  - `ui-generation.md` 重写「② 提案生成」「③ 审核发布」「Page Studio」节，删除 ⚠️ 横幅
  - `composite-editor-v3.md` 删除「≥2 区块」全部表述与 ⚠️ 横幅
  - 全库 grep 确认无残留：`rg -n '即将变更|绑定.*前提|≥2 区块|至少 2 个函数区块|must include zh-CN and en-US|zh-CN 与 en-US 为必填|双必填' docs`
- `ui-generation-upload-pipeline.md` 状态从 Proposed 改为 Current，「已知边界」按实际交付更新

**验收**：

- `pnpm --dir web run tsc`、`pnpm --dir web test`、`go test ./internal/...` 全绿
- `bash scripts/dashboard_vnext_guard.sh` PASSED
- `cd docs && pnpm build` 通过
- E2E 用例全绿

---

## 菜单管理系统（Menu Management）

设计依据：`docs/design/menu-management.md`
原子任务：独立完成、独立测试、独立提交。无顺序依赖，可并行开发。

### T-M1. 数据库迁移：创建 menu_items 表

**目标**：创建菜单管理的基础数据结构。

**改动点**：

- `internal/model/migration.go`：新增 `menu_items` 表自动迁移
- `internal/model/menu.go`：新建 `MenuItem` 模型（GORM）

**验收**：

- `go test ./internal/model/...` 通过
- 数据库启动后自动创建 `menu_items` 表
- 表结构包含：id, parent_id, menu_key, labels, icon, sort_order, permission, is_visible

### T-M2. 菜单 CRUD API

**目标**：实现菜单的增删改查接口。

**改动点**：

- `internal/api/menu/handler.go`：新建菜单 handler
- `internal/api/menu/service.go`：新建菜单 service
- `internal/handler/routes.go`：注册菜单路由

**API**：

- `GET /api/v1/menus` — 获取菜单树
- `POST /api/v1/menus` — 创建菜单
- `PUT /api/v1/menus/:id` — 更新菜单
- `DELETE /api/v1/menus/:id` — 删除菜单
- `PUT /api/v1/menus/:id/sort` — 更新排序

**验收**：

- `go test ./internal/api/menu/...` 通过
- CRUD 接口可正常调用
- 菜单支持多级嵌套

### T-M3. 菜单权限过滤 API

**目标**：实现用户可访问菜单的过滤接口。

**改动点**：

- `internal/api/menu/handler.go`：新增 `GET /api/v1/menus/accessible`
- `internal/api/menu/service.go`：实现权限继承过滤逻辑

**验收**：

- `go test ./internal/api/menu/...` 通过
- 无权限用户看不到受限菜单
- 子菜单继承父菜单权限

### T-M4. 页面关联菜单 API

**目标**：实现页面与菜单的关联。

**改动点**：

- `internal/api/page/handler.go`：新增 `PUT /api/v1/pages/:key/menu`
- `internal/api/page/service.go`：实现菜单关联逻辑
- `internal/model/migration.go`：pages 表新增 `menu_id` 字段

**验收**：

- `go test ./internal/api/page/...` 通过
- 页面可设置所属菜单
- 页面可查看所属菜单

### T-M5. 数据迁移脚本

**目标**：将现有 category 数据迁移到 menu_items 表。

**改动点**：

- `scripts/migrate-categories-to-menus.sql`：迁移脚本
- 从 pages 表提取 category_key/category_labels 创建 menu_items
- 更新 pages 表的 menu_id

**验收**：

- 迁移脚本可执行
- 迁移后数据一致性验证通过
- 页面正确关联到菜单

### T-M6. 前端菜单管理页面

**目标**：实现菜单管理的 UI 界面。

**改动点**：

- `web/src/pages/MenuManagement/index.tsx`：菜单管理页面
- `web/src/pages/MenuManagement/MenuForm.tsx`：菜单编辑弹窗
- `web/src/pages/MenuManagement/MenuTree.tsx`：菜单树组件
- `web/config/routes.ts`：注册菜单管理路由

**验收**：

- `pnpm --dir web test` 通过
- 可查看菜单树
- 可创建/编辑/删除菜单
- 可拖拽排序

### T-M7. 前端登录时拉取菜单

**目标**：登录后获取用户可访问的菜单树。

**改动点**：

- `web/src/services/api/menu.ts`：菜单 API 封装
- `web/src/store/modules/menu.ts`：菜单状态管理
- `web/src/layouts/BasicLayout/index.tsx`：侧边栏使用菜单数据

**验收**：

- `pnpm --dir web test` 通过
- 登录后侧边栏显示用户可访问的菜单
- 无权限菜单不显示

### T-M8. 删除旧 category.labels 代码

**目标**：清理所有旧的分类标签相关代码。

**改动点**：

- 删除 `internal/api/page/service.go` 中的 `category.labels` 校验逻辑
- 删除前端中 `category.labels` 相关的代码
- 删除数据库迁移中的旧字段

**验收**：

- `go test ./internal/...` 通过
- `pnpm --dir web test` 通过
- 无残留的 `category.labels` 代码

### T-M9. 端到端验证（已完成 2026-09-17）

**目标**：验证菜单系统完整工作流。

**改动点**：

- `web/e2e/menu-management.spec.ts`：E2E 测试（real-dashboard 项目，三用例：CRUD 完整流程 / 页面按分类 key 挂载菜单 / 权限过滤与继承）

**验收**：

- 创建菜单 → 页面关联菜单 → 用户登录看到菜单 → 权限过滤正确
- 菜单 CRUD 完整流程
- 权限继承正确

---

## UI 生成链路 6 卡点整改（M1–M6，已完成 2026-09-18）

六卡点全部落地（提交 `f28efde48`/`55df216f4`/`50926e89f`/`d5ac9a938`/`5aa785c51`，CI 全绿，
发布链闭环经 dev-fixture 真实 server 验证）：

| 卡点                                  | 交付                                                                                       |
| ------------------------------------- | ------------------------------------------------------------------------------------------ |
| 1. 提案重建失败回滚整个注册           | M2：提案/模板重建移出注册事务，失败降级 registration warning，手动 rebuild/心跳重注册兜底  |
| 2. 模板重建失败仅 slog 静默           | M1：`template_regen_failed` 进告警通道 + 前端 Warnings URL 修复                            |
| 3. auto 发布分级只覆盖 composite 保存 | M5：auto env 下 AcceptProposal 自动接续发布（SetPublishReviewHooks 注入）                  |
| 4. 契约漂移无批量 sync-selectors      | M4：`POST /pages/bulk-sync-selectors`（契约变更队列逐页收口，只写 draft）                  |
| 5. >500 operations 上传同步超时       | M6：`openapi.pipelineOperationGuard` 护栏 warn 级 diagnostic（默认 500、负数禁用、不阻断） |
| 6. 逐函数注册反复全量模板重写         | M3：UpsertBuiltin 内容门控（全字段比较一致跳写，不刷 updated_at）                          |

**已知边界（诚实清单）**：

1. **衍生重建告警内存生命周期**（M1/M2）：`proposal_rebuild_failed`/`template_regen_failed` 与既有注册告警同为内存实现——进程重启即失、重建成功不自动清除既有条目（按 Count 递增）；`registration_warnings` DB 持久化接线留待独立需求。
2. **auto accept 自动发布失败不回滚**（M5）：accept 已成功、draft 保留，`publishError` 带回原因由前端提示走人工链；已存在 draft 的 auto accept 走「提案重建草稿 + 发布」多一跳（行为与 composite 保存链一致）。
3. **批量同步严格只写 draft**（M4）：不自动 publish，上线仍需 `bulk-republish`；governance/version 等不可由 selector 同步修复的漂移整页 `skipped` 并透传诊断（不做半吊子同步）；单页失败/并发冲突计入 `failed` 继续不中断。
4. **大文档护栏只提示不阻断**（M6）：warn 级 `large_document_pipeline` diagnostic，同步管线仍受 HTTP WriteTimeout 约束，异步化另议。
5. **composite 保存弹窗前端暂未消费 `published`/`publishError`**（T10 遗留，M5 只接了收件箱 accept 链）：服务端语义已闭环，保存弹窗提示增强属后续任务。
6. **M3 门控仅限 builtin 行**：custom 占 key 行维持覆盖路径且 Builtin 标记不翻转；JSON 列解析失败回退字节比较（宁误写不误跳过）。

明确不做（当期范围外）：freshness 评估、Console 菜单聚合、unbound 翻转、`registration_warnings` DB 表接线。
