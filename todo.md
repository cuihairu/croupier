# 上传即成页：契约与绑定正交化 TODO

> **T1–T13 已于 2026-09-18 全部完成**（T1 `65f88486b`、T2 `3b17cfc41`、T3 `f61011249`、
> T4 `a911471e0`、T5 `f4bd57640`、T6 `0a6a4703e`、T7 `16aea5947`、T8 `057f17045`、
> T9 `3937b71ff`、T10 `f5b397f9c`、T11 `7564698d1`/`3970ce87d`、T12 `6dd616154`、
> T13 `545477a5e`），下方任务清单保留为验收口径存档。

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

> **存量库迁移补漏（2026-09-18，0027）**：T-M1/T-M4 落地时只改了模型（MenuItem 表 + PageSpec.MenuID 列），未配编号迁移——线上 postgres `menu_items` 表缺失（menus API 500）、`page_specs.menu_id` 缺列（页面保存/发布链 SQLSTATE 42703 整体中断）；sqlite/dev 环境走 AutoMigrateGame 建全列，CI 拦不住。0027 补齐（缺表 CreateTable + 缺列 AddColumn，幂等），`MinimumRequiredVersion` 26→27。与 0021/0023 同族「模型改了迁移漏配」事故。

### T-M10. 默认菜单种子（scope 化惰性导入）（已完成 2026-09-18）

**动机**：页面能自动生成、auto env 保存即发布，但菜单树开箱为空——控制台导航「最后一公里」断在手动建菜单。

**设计要点**：

- 对齐 AdminManager 默认管理员/角色先例：种子文件放 `configs/`（如 `default-menus.json`）
- **scope 化惰性导入**：某 `(gameId, env)` 首次访问菜单服务且该 scope 菜单为空时导入（非仅进程首启）——每个新游戏/环境接入自动拿到骨架
- **只在空表导入、永不覆盖**：用户改名/删菜单后重启不受影响（幂等种子，非 sync）
- 骨架与 capability 常见分类对齐：player 玩家 / operation 运营 / payment 支付订单 / announcement 公告 / audit 审计
- 边界：默认菜单只是空组骨架，三要素不变（页面仍需已发布+挂载才上控制台）；空组渲染为指向 `/console/<menuKey>` 的单链接

**验收**：

- 全新 scope 首次访问菜单 → 种子导入且只导一次
- 已有菜单的 scope → 不导入不覆盖
- 种子导入的菜单与手工创建行为一致（可改/可删/可挂载）

> **交付（2026-09-18）**：`internal/svc/menu_seeder.go`（MenuSeeder 惰性种子 + LoadSeedMenus 校验）、`configs/default-menus.json`（player/operation/payment/announcement/audit 五组双语骨架）、读路径接线 `menu.Service.List` 与 `menu.AccessibleTree`（console 导航共用）；零新配置（文件存在即启用，缺失即禁用）；`MenuItemModel.CountByScope` 查询方法（无 schema 变更，无需迁移）。单测 `menu_seeder_test.go` 12 例 + e2e 实证（fixture 日志 `menu seed: default menus imported created=5`，menu/operation/openapi-crud 三 spec 全绿含 openapi-crud:344 排他断言）。

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
5. ~~**composite 保存弹窗前端暂未消费 `published`/`publishError`**~~（已收口 2026-09-18：保存弹窗按响应三态提示——`published=true` → 「页面已发布」；带 `publishError` → 「提案已创建，自动发布失败」+ 原因与人工重试指引；默认 → 进收件箱文案）。
6. **M3 门控仅限 builtin 行**：custom 占 key 行维持覆盖路径且 Builtin 标记不翻转；JSON 列解析失败回退字节比较（宁误写不误跳过）。

明确不做（当期范围外）：freshness 评估、Console 菜单聚合、unbound 翻转、`registration_warnings` DB 表接线。

## 覆盖率收尾批次（主树清单 #28/#31/#41 与转正提交闭环后，2026-09-27）

> **交付（2026-09-27）**：主树最大零覆盖模块 `System/SiteSettings`（2118 行五文件 0 测试）入口页补齐——
> `index.tsx` 0% → 行覆盖 100%（分支 90.9%/函数 100%），7 用例锁定三层配置契约
> （database 覆盖+恢复按钮 / config 跟随 / default 徽标；保存 trim+空值守卫+成功重拉；
> 恢复走 clearSiteSetting；加载/保存/恢复失败路径不白屏不重拉）。
> 全量 jest 3688/3688 绿（空载窗口）、tsc 0 错、go test ./internal/... 全绿（99.8% 总量、
> 0 文件低于 60%）、guard PASSED。
> **已知边界**：四个子 Tab（AuthTab 789 / NotificationTab 450 / FeatureFlagsTab 298 /
> ObservabilityTab 223 行）桩替换未覆盖，留下一批次（AuthTab 已于 2026-09-28 批次闭环，
> 见下节；其余三个属 croupier-ui 工作面）。

## 覆盖率巡检批次·Go 侧（wt-api worktree，2026-09-28）

> **交付（2026-09-28）**：全量 profile（99.82%，61051/61160 语句）后按文件粒度
> 取最低可离线测文件补齐——`internal/model/ticket_model.go` **79.2% → 100%**
> （internal/ 全树最低文件）：缺口为 #21 服务端下拉聚合
> `ListCategories`/`ListAssignees`/`listOptionStats`（落地时无模型层测试），
> 补 3 例锁定契约：distinct 非空聚合 + COUNT + name ASC、空串不成行、
> (game_id, env) 双带/单带/不带三口径、缺表报错不 panic。
> 此前批次已封顶（均在 main）：api/provider 99.2%、registry 99.7%、
> api/profile 99.5%、api/node 100%、common/errorx 100%。
> 门禁：gofmt 干净、go vet 干净、go test ./internal/... 全绿、guard PASSED。
> **已知边界**：auth/mfa.go(90.4%) 与 security/otp 缺口属 d9fdc05 会话
> OTP 工作面，回避不碰；assignment/gate.go(89.5%) 属 BUG-035 会话工作面
> 同理；共享内存库（cache=shared）跨用例数据共享，补测用唯一 scope 隔离。

## 覆盖率巡检批次·Go 侧第二轮（wt-api worktree，2026-09-28）

> **交付（2026-09-28）**：全量 profile 重排（本轮全树 109 未覆盖语句）后按
> 文件粒度取最低可离线测文件——`internal/platform/objstore/avatar.go`
> **91.1% → 98.2%**（56 语句中仅剩 1 条已登记不可达）：补 2 例——
> ① 相对输入形态的 query/fragment 裁剪（绝对 URL 经 url.Parse 后 u.Path
> 已不含 ?/#，原测试永远触达不了 105/108 两个 Index 分支，须用相对 key）；
> ② `LocalBaseDir` 的 filepath.Abs 错误分支与 `mustGetwd` 失败分支
> （均需 os.Getwd() 失败，chdir 进已删除目录构造），objstore 包
> 99.2% → 99.8%。门禁：gofmt 干净、go vet 干净、go test ./internal/...
> 全绿、guard PASSED（机器负载 99→50 回落窗口执行，如实注明）。
> **已知边界**：NormalizeAvatarKey 的 `key == AvatarPrefix`（key 不能是
> 目录）防御分支不可达——sanitizeKey 基于 filepath.Clean，Clean 恒去除
> 尾斜杠（根 "/" 例外，TrimPrefix 后为空串、过不了 HasPrefix 前置校验），
> 归一结果不可能等于 "avatars/"；不造假用例、不删防御分支。
> 下一轮候选（本轮快照）：api/resourcecatalog/handler.go 93.1%
> （9 语句，新落地面，待确认无归属冲突）、api/function/
> version_history_handler.go 91.4%（#26 旧域，单错误分支）。

## 登录方式 Tab 覆盖批次（SiteSettings 子 Tab 之一，2026-09-28）

> **交付（2026-09-28）**：`System/SiteSettings/AuthTab.tsx`（789 行）0 测试 →
> 行覆盖 100%（789/789）、函数 100%（12/12）、分支 92.0%（104/113），12 用例锁定
> LDAP/OIDC 双卡片契约：快照回填（SourceTag 矩阵 database→UI / yaml·config→配置文件 /
> default→默认 / 未知与缺失来源不渲染；secretSet 脱敏徽标 +「留空保持不变」占位；
> startTls 'true' 解析）、保存 saveKeys（trim 落库、空串 clearSiteSetting 回落配置文件、
> secret 留空既不清也不提、布尔透传、成功重拉）、保存并测试三态（ok=true message 透传 /
> ok=false modal.warning / 抛错 extractErrorMessage 兜底，test 路径不弹「已保存」toast）、
> required 校验失败静默早退（不提交不弹错）、保存失败不重拉、enabled 开关载荷翻转、
> 加载失败不白屏。
> 门禁：套件 12/12 绿、tsc 0 错、go test ./internal/... 全绿、全量 jest 空载窗口绿。
> **已知边界**：saveKeys 的 value===undefined/null 分支臂经 UI 不可达（表单值只会产生
> string/boolean，trim ?? '' 兜底空串），计 9 个未覆盖分支；NotificationTab /
> FeatureFlagsTab / ObservabilityTab 三个子 Tab 属 croupier-ui 工作面，本会话不碰。

## 通知设置 Tab 覆盖批次（SiteSettings 子 Tab 之二，2026-09-28）

> **交付（2026-09-28）**：`System/SiteSettings/NotificationTab.tsx`（450 行）0 测试 →
> 语句/分支/函数/行 4×100%（v8），11 用例锁定通知配置读写/校验/失败路径：
> 加载成功（非密文回填、三个 secret 字段永不回显、开关两态、SMTP 区块随
> emailEnabled 条件渲染、密文徽标配置/未配置两态）、加载失败（extractErrorMessage
> 兜底 + null settings 渲染不白屏）、saveKey 三分支（文本 trim 落库 / 空值=清除
> 走 clearSiteSetting / 端口 InputNumber 数字直提不经 trim）+ 保存失败（透出后端
> message、不重拉、按钮退出 loading）、toggleBool 开「已开启」/关「已关闭」 +
> 失败「操作失败」兜底不重拉、placeholderMsg 经 intl 解析、Card loading→finally
> 骨架收尾。开关无 accessible name 按 DOM 序索引定位；字段保存按钮按所属
> Form.Item 定位（全局 [0] 在 emailEnabled=true 时会错点 SMTP 字段）。
> 门禁：目标套件 11/11 绿、tsc 0 错；全量 jest 3853 用例在 load 27-64 限
> 2 worker 完成（非空载，如实注明）——8 失败中 7 个他域套件纯 timeout 形态、
> 隔离复跑全绿定责负载型，1 个为并行会话未跟踪 WIP 套件（Operations/Configs
> **tests** 未入库），均非已交付代码回归。
> **已知边界（诚实清单）**：
>
> 1. secretState 的 ternary 链缺 feishuSecret 特例：飞书密钥徽标实际读
>    webhookSecretSet/Masked，NotificationSettings.feishuSecretSet 存在但徽标
>    不消费——现状行为断言（「未配置」仅钉钉 1 个、无尾号「已配置 」2 个），
>    用例登记翻转条件，组件修正时同步。
> 2. FieldDef 上 smtpPort 的 `kind: 'int'` 元数据无消费方（字段被 filter 剔除后
>    走 smtpPortField 专用渲染），无对应行为可断言。
> 3. FeatureFlagsTab / ObservabilityTab 两个子 Tab 仍留待后续批次。

## 覆盖率巡检批次·Go 侧第三轮 + main 红测修复（wt-api worktree，2026-09-28）

> **交付（2026-09-28）**：派发两项核对结果——① api/function/
> version_history_handler.go 的 91.4% 是排序管线吃到陈旧测试缓存条目的
> 假缺口（全量 profile 中该包为 (cached) 旧条目），`-count=1` 实跑 function
> 包已 100%（version_history_handler_extra_test.go 三例确定性覆盖
> 错误/排序/去重分支），无需补测；② api/resourcecatalog/handler.go 归属
> 冲突核实成立——wt-support 分支持有未进 main 的 c79e0e9（BUG-031，
> resourcecatalog 目录），按派发跳过。同窗发现并修复 main tip 红测：
> internal/model/bug_error_paths_test.go 两个 sanity check 用例漏建
> bug_ticket_links 表，与自身「正常表结构下不应报错」断言矛盾，
> fresh run 必挂（此前被测试缓存掩盖带病落库）；拆出 newBugErrNormalDB
> 补齐完整表结构（test-only，业务源码零改动），model 包 fresh 全绿
> 99.9%，bug.go 仅剩两处 Scan 错误防御分支按既有口径登记不可达。
> 门禁：触及文件 gofmt 干净、go vet ./internal/... 干净、
> go test ./internal/... 全绿（fresh 实跑）、guard PASSED
> （负载 ~30-50 非空载窗口执行，如实注明）。
> **已知边界**：并行会话正在本共享目录在途编辑
> coverage_contract_version_test.go（ListDistinctVersions 补测，未提交），
> 按「只 add 自己文件」铁律未纳入本提交，function_contract_version.go
> 缺口视为已被认领；全树 gofmt -l 现存该在途文件的格式化挂起（非本会话
> 文件，不代改）。下一轮候选：registry/store_metadata.go 97.3%（4 语句，
> 本会话旧域有上游新增）、api/provider/handler.go 97.8%（2 语句，同上）；
> 回避面不变（auth/mfa.go、security/otp、assignment/gate.go、
> resourcecatalog、他会话在途文件）。

## 功能开关 + 观测配置 Tab 覆盖批次（SiteSettings 子 Tab 收尾，2026-09-28）

> **交付（2026-09-28）**：SiteSettings 四个子 Tab 最后两个补齐——
> `FeatureFlagsTab.tsx`（298 行）语句/分支/函数/行 4×100%（v8）+ `ObservabilityTab.tsx`
> （223 行）语句/函数/行 100%、分支 91.66%，合计 15 用例：
> FeatureFlags 锁定五域合成值回显、来源三态徽标（跟随部署配置/数据库覆盖/
> 部署已裁剪）、缺省域兜底（snapshot 缺 key 默认开启）、裁剪禁用两翼
> （trimmed && !enabled → disabled / trimmed 但已开仍可关）、toggle 开/关文案、
> 成功链三步（重拉快照 + fetchServerFeatures + setInitialState 全局 features
> 缓存刷新，失败不触达全局同步）、Popconfirm 确认恢复 clearSiteSetting；
> Observability 锁定三入口回填、徽标三态（数据库覆盖/环境变量/未配置）、
> 恢复按钮仅 database 来源行可见、trim 落库、空值=清除覆盖、保存失败透出
> 后端 message 不重拉、Card loading 骨架收尾。
> 门禁：目标套件 15/15 绿（28s）、tsc 0 错；全量 jest 3901 用例限 2 worker
> 883s 完成，唯一失败为并行会话未跟踪 WIP 套件（Operations/Configs
> **tests** 未入库、git 历史无记录），非已交付代码回归，其余 312 套件全绿。
> **已知边界**：ObservabilityTab saveField 的 `if (!meta) return;` 守卫 true 翼
> 构造性不可达（按钮 onClick 闭包只传 FIELDS 自有键），分支 91.66% 余量即此，
> 不造假用例。SiteSettings 五文件（入口 + 四子 Tab）至此全部收口。

## 操作日志页全局 scope 联动（OPEN-ISSUES #43，2026-09-28）

> **交付（2026-09-28）**：`Admin/OperationLogs.tsx` 游戏/环境过滤接入全局 scope
> store（#32/#35/#38/#39 同族「选了没刷新」收口）：初值取当前 scope，顶栏切
> 游戏/环境后 useEffect 同步覆盖本地过滤输入（用户可见、可手改，直至下一次
> scope 变化再覆盖），gameId/env 在 ProTable params 中，params 变化自动重发
> request。3 用例锁定：挂载 scope 预填进首拉参数、切 scope 覆盖手输值并带新
> 值重查、scope 清空回全量（请求不带 gameId/env）。store 用真实单例 +
> setScope 驱动（merge 语义下显式 undefined 才能清空）。门禁：套件 16/16 绿、
> tsc 0 错、全量 jest 3818/3818（2 worker 限流——并行会话曾把机器打到
> load 94，回落窗口完成）、guard PASSED。

## SDK 分布注册时间列（OPEN-ISSUES #44，2026-09-28）

> **交付（2026-09-28）**：`/api/v1/providers/sdk-stats` 实例明细透出
> `firstSeenUnix`（#27② 同模式服务端归一：零值回退 `lastSeenUnix`；dto 字段 +
> service 映射 + Go 回归 1 例：显式观测值原样透传/无观测实例归一非零），前端
> `SdkInstanceItem.firstSeenUnix` 类型 + 「注册时间」列（Tooltip 绝对时间，
> 与「最后活跃」对照区分「实例活了多久 vs 只是无响应」，zh/en locale 同步）+
> 页面用例 1 例（回退值 firstSeen===lastSeen 同刻渲染两列 → 同文本双元素，
> _AllBy_ 断言恰为 2）。wire 文档同步 `sdk-wire-protocol.md` 实例元数据小节。
> 门禁：go build ./... + go test ./internal/... 全绿、tsc 0 错、全量 jest
> 3818/3818、guard PASSED。
> **已知边界（诚实清单）**：FirstSeenUnix 为进程窗口语义——registry 会话是
> 内存态，会话过期或 server 重启后从零重新累计（列注释与 wire 文档均注明）；
> 持久化历史注册时间需 registry 落库演进，本批不做。

## 覆盖率巡检批次·Go 侧第四轮（wt-api worktree，2026-09-28）

> **交付（2026-09-28）**：上轮候选两文件补齐——① `registry/store_metadata.go`
> **97.3% → 100%**（registry 包 99.7% → **100%**）：4 块全闭——增量判定
> providerMetadataDelta 的「集合等长但 ID 不同/键数不同」翼（漏判会让重复
> 注册跳过 DB 写）、staleServiceIDs 的 prev 重复去重翼、DB-less 内存聚合
> aggregateMetaOptionsFromMemory 的 env 过滤翼、groupMetaOptions 的实例数
> 降序比较翼；4 例同包直测（未导出字段/类型，不 import internal/model
> 遵守包边界）。② api/provider/handler.go 的 ShouldBindQuery 错误分支
> **登记防御性不可达**（SdkStatsRequest 全 optional string 字段，gin form
> 绑定无失败路径），新增证明性用例锁定「任意 query 绑定永不失败」前提，
> 文件维持 97.8%（90 语句中 2 条登记）；字段未来引入 required/强类型时
> 分支转可达，届时补真实错误路径用例。
> 门禁：触及文件 gofmt 干净、go vet ./internal/... 干净、
> go test ./internal/... 全绿（fresh）、guard PASSED。
> **下一轮候选**：svc/migrations.go 97.9%（8 语句，迁移敏感面需谨慎评估）、
> rbac/logical_permissions.go 97.5%（1 语句）；回避面不变。

## 覆盖率巡检批次·Go 侧第五轮（wt-api worktree，2026-09-28）

> **交付（2026-09-28）**：上轮候选两文件收口——① `svc/migrations.go`
> 可达性评估结论：**8 块全部可达，零排除项**（与 30+ 同构兄弟迁移同模式：
> 全查询失败连接注入触达 wrapGorm err 透传翼，PRAGMA query_only 拒写注入
> 触达 CreateTable/AddColumn 失败翼，夹具复用 C 批 coverage_c_migrations_test.go），
> 新增 coverage_d_migrations_test.go 5 例闭齐 0032（admin_otp_recovery_codes
> 建表）/0033（admins 密码策略加列）/0034（provider_metadata 建表，本会话
> #11 域）/0035（bug_ticket_links 建表）四迁移错误分支至 **100%**；
> 0032-0033/0035 为 NewGoMigration 内联闭包，经导出字段 UpFnNoTxContext
> 直调（goose 对 RunDB 的映射），业务源码零改动；0032 虽邻 OTP 面，测试放
> 本会话独立新文件、零编辑他会话在途的 admin_otp_recovery_test.go。
> ② `rbac/logical_permissions.go` **97.5% → 100%**（rbac 包 99.8% → 100%）：
> 导出包装 SplitLogicalPermission 此前 0%——8 形态表驱动锁定透传语义与
> 通配约定（""/*/admin:all → 通配、无冒号/空 action/all → *、归一）。
> 门禁：触及文件 gofmt 干净、go vet ./internal/... 干净、
> go test ./internal/... 全绿（fresh）、guard PASSED。
> **下一轮候选**：巡检主干已近枯竭（99.8%+ 文件余 1-2 语句防御分支为主），
> 转监控回补：任何新落地 Go 文件 48h 内补齐主函数与错误路径。

## 公告绑定多游戏与按游戏过滤（OPEN-ISSUES #45，2026-09-28）

> **交付（2026-09-28）**：语义拍板「一公告可绑定多个游戏，未绑定任何游戏=全服可见」。
> `announcement_games` M2M 表（0036 编号迁移三处同步齐：migrations.go 注册 +
> MinimumRequiredVersion=36 + migrate_test probe 清单；HasTable→CreateTable 幂等，
> 0019 先例）；Create/Update 全量替换语义（`gameIds` 缺省 nil=绑定不动、空数组=
> 清空→全服可见、非空=全量替换，归一 trim/去重/丢空保序）；管理列表 `?gameId=`
> 过滤 + 「适用游戏」列（绑定 Tag / 全服可见 Tag）+ 表单多选（选项来自 games
> 列表，与顶栏 scope gameId 同口径）；用户侧 `/announcements/active` 按
> `X-Game-ID` 头过滤（scope.ts SCOPED_API_PREFIXES 注入，无游戏上下文仅见未绑定
> 公告=严格语义）；Delete 级联删绑定；批量加载绑定防 N+1。
> 门禁：announcement/svc/migrate/model 包 Go 测试绿（新建 8 用例：绑定生命周期/
> 过滤矩阵/三态可见性/级联删除/迁移幂等）+ 页面套件 17/17（新 6 用例：列渲染/
> scope 预填与清空/切 scope 覆盖手选/默认空提交/编辑回填清空/追加携带）+
> tsc 0 错 + go build ./... + 全量 jest 3839/3839（2 worker 限流——load 5min 峰值
> 32，非空载直跑）+ guard PASSED；rebase origin/main 后受影响包复验绿。
> **已知边界（诚实清单）**：① 管理列表过滤为服务端全量后过滤（公告量级小，
> 不分页下推）；② 游戏列表加载失败时表单多选下拉仅剩手输、过滤下拉为空，
> 不阻塞公告管理；③ 用户侧可见性未与「玩家归属游戏」联动（无玩家-游戏注册
> 关系，X-Game-ID 即用户顶栏选择）；④ web 端 `/announcements/active` 消费方
> （AnnouncementPopup/NotificationsTab）取值链未按绑定渲染游戏名，属纯过滤
> 透传，不改变展示文案。

## Dev/Bugs 缺陷页覆盖批次（全仓最大零测试页收口，2026-09-28）

> **交付（2026-09-28）**：`Dev/Bugs/index.tsx`（1113 行，此前全仓最大零测试
> 引用页）0 测试 → 新增 `__tests__/index.test.tsx` 17 用例，v8 口径行/函数/
> 语句 3×100%、分支 93.75%。锁定契约：列表渲染矩阵（链接图标 5 类内联 +
> '+N' 溢出徽标、status/severity/priority 未知枚举原文兜底、platform 大写、
> source 三态、canManage 操作列两态）；工具栏（关键词/四下拉筛选/修复版本/
> 刷新各自重拉、筛选清空 `|| ''` 翼、回车仅回第一页不重复拉取）；真实
> ModalForm（required 拦截、GitHub 链接自动标题 o/r#42、空 url 守卫、创建/
> 编辑载荷与 source:internal、创建/编辑失败兜底两翼、删除 Popconfirm）；详情
> 弹窗（tag 矩阵、meta 拼接三翼、未知 priority 兜底、外链按钮、关联工单
> 列表翼矩阵 + 跳转、添加/解除关联成功失败双翼、tickets 加载失败静默、关闭
> 可重开）；?bugId= 深链三翼与 listAdmins 兜底两翼；canManage=false 只读形态。
> 门禁：目标套件 17/17 绿、tsc 0 错；全量 jest 限 2 worker 落盘后台
> （负载口径，结果见交付说明）。
> **已知边界（诚实清单）**：
>
> 1. 守卫与防御性分支经 UI 不可达（分支余量 12 处全部在此）：addDetailTicket
>    `if (!detail || !ticketDraft) return`（按钮 disabled）、removeDetailTicket
>    `if (!detail) return`（按钮仅详情开启时存在）、request 包装层参数 `?? ''`
>    右翼（ProTable 恒传 params 键）、链接 `l.title || l.url` 右翼
>    （deriveBugLinkTitle 对解析失败 url 也回退原文）、InputNumber
>    `typeof v === 'number' ? v : null` 右翼（antd 6 jsdom 清空不回调
>    onChange(null)）。
> 2. 行内链接图标断言锚定标题文本与溢出徽标（antd Icon aria-label 无文本
>    节点）；详情 `<Empty />` 兜底在 detail=null 隐藏态渲染（v8 计入覆盖），
>    用例断言可观测的 onCancel 后果（隐藏 → 可再打开）。
> 3. ProTable 挂载首拉与快速输入变更会被 20ms 防抖合并（用例先等首拉落定
>    再驱动筛选，保证计数断言确定性）。

## 插件域设计收口（OPEN-ISSUES #46，2026-09-28）

> **交付（2026-09-28）**：#46 设计层收口完成（用户指令「继续设计插件」），
> 全库审计后两份核心文档定稿：
> ① `extension-installation-model.md` 草案→**已收口**——对齐实际落地的 5 表
> （catalog/release/installation/runtime_binding/event，复数表名+unix 时间戳+
> JSON 列）；生命周期从草案 12 态收口为实态 4 状态同步流转（install/enable/
> disable/upgrade/uninstall，中间态无落库消费方）；升级=校验链（release 存在
> →依赖图→config 兼容）+版本切换，**回滚=upgrade 到旧版本**（不设独立端点）；
> Agent 运行时副本=内存 RuntimeSnapshot+HTTP 轮询 30s（非 TCP 隧道推送，演进
> 项）；scope=meta 库行级标记（与 database-per-game 不冲突的论证入档）；
> capability/health/secret_binding 三表暂缓决策+回补路径入档；如实记录已知
> 边界：禁用态升级后 `status=enabled && enabled=false` 字段不同步（实施批次修）。
> ② `extensions-api-contract-baseline.md` 按实现重写——「统一包装 code/message/
> data」旧表述作废（实现恒为 common/response 直返，纯文档漂移）；字段名
> snake_case 笔误全部改 lowerCamelCase；canonical 路由=installations/:id 主组，
> compat 路由组（13 条）判废（唯一前端消费方本就契约错位）；agent wire 契约
> （/agents/:id/extensions 的 payload 包装）单列防误伤。
> ③ 四页面审计（结论入基线 §6）：Store/Installations/AgentSync ✅ 一致（adapter
> 层+错误码分支已接）；**DomainEntry ❌ 实锤**——`listExtensionPages` 期待
> `{items:[{path}]}` 后端实返 `{pages:[{route}]}` 且 `.catch` 静默，「扩展页面
> 入口」区块生产环境恒 Empty。
> ④ 审计副产物：响应 DTO 内嵌 code/message 对前端零影响（normalize 直取业务
> 字段已核实）；`healthStatus` 恒 "unknown"、`displayName`=extensionId 直填
> （列表组装未 join catalog）——两处失真入批次链；**catalog/release 无任何写
> 路径**（repo 只读、无 admin CRUD、无 pack 导入、无 seed）——「扩展从哪来」
> 是安装模型最大缺口，立为批次链主项。
> 门禁：`cd docs && pnpm build` 通过（60.93s）。
> **实施批次链（后续独立批次，设计已定）**：1) 契约收口（24 个响应 DTO 去内
> 嵌 code/message + agent puller 解析端同步 + compat 组删除 + pages 契约修复，
> 禁兼容旧键）；2) 列表组装修正（displayName join catalog / healthStatus 推
> 导）；3) catalog 写路径（admin CRUD + official.* 四扩展 seed，pack 导入后续）；4) DomainEntry 真实渲染恢复+用例。

## 覆盖率巡检批次·Go 侧第六轮·监控回补（wt-api worktree，2026-09-28）

> **交付（2026-09-28）**：派发「排查最近 48h 新落地 Go 文件，为主函数与错误
> 路径补齐测试」。排查结论：近期新文件中 `registry/agent/provider_metadata.go`
> 已 100%、`model/provider_metadata.go` 纯数据型（读写路径经伴生测试覆盖）、
> `api/function/version_history_handler.go` 归他会话在途（untracked 补测文件
> 已在其目录，回避）、`server/assignment/gate.go` 归 wt-support BUG-031 域
> （回避）、`registry/store_metadata.go` 上轮已 100%——唯一真实缺口在
> `examples/cmd/dev-seed`（包 82%：main 0%、runMain 35%、全部 seeder 错误翼
> 空）。补齐 `seed_error_injection_test.go`（271 行，15 用例/30 子测试）→
> **dev-seed 包 100.0%**（main/runMain/seed.go/seed_edges.go 全文件 100%）：
> ① main 体经 exit 注入点 + os.Args 覆盖（-h → 0）；runMain 五分支全通：
> 死端口 postgres dsn → 打开失败（1）、games 表预置 NOT NULL 无默认列 →
> AutoMigrate 容忍但 INSERT 违约 → seedAll 失败（1）、空 dsn + t.Chdir 临时
> 目录 → 成功（0）、**只读库文件**（合法空库 chmod 0400，Open 只读回落成功、
> 首个 CREATE TABLE 即拒）→ 建表失败（1）。
> ② seeder 错误翼双注入（同 svc C 批口径）：读翼（哨兵 Count/Pluck）
> DropTable 缺表即错；写翼全量预铺 + Unscoped 硬删第 k 组行 + PRAGMA
> query_only 拒写——前组读命中、第 k 组读未命中写入被拒，k 遍历各组即盖全。
> ③ 三个注入拦截点如实修正（教训入档）：a) 写在哨兵早退之后的翼（工单评论）
> 拒写到不了 → 缺表注入（清空工单行放行哨兵 + drop ticket_comments）；
> b) seedAccounts 的 role_permissions OnConflict Create 无存在性检查，拒写下
> 必先在 links 翼报错、admins/admin_roles 两翼被拦截 → 缺表注入（links 重放
> DO NOTHING no-op 放行）；c) 垃圾文件注入实测落「打开失败」分支（glebarez
> Open 期即校验文件头，与死端口 dsn 同翼），建表失败分支改只读文件注入，
> 两分支互补不重叠。
> ④ seedHeavy 万级只铺一次底（与既有 BulkVolume 同价），缺表/拒写变体盖
> 其余三翼；空库拒写直接盖 players CreateInBatches 翼免铺万级。
> 门禁：触及文件 gofmt 干净、go vet ./internal/... 干净、go test
> ./internal/... 全绿（fresh）、dev-seed 包 fresh 全绿（100.0%）、
> scripts/dashboard_vnext_guard.sh PASSED。
> **已知边界**：门禁在负载高位窗口执行（并行会话持续占机，dev-seed 包
> 222s、audit 包 148s 属环境性慢，非回归）；全量 jest 未单跑（本轮零 web
> 触碰，guard 已覆盖 PageSpec 侧校验）。

## 插件域批次 1：契约收口（OPEN-ISSUES #46，2026-09-28）

> **交付（2026-09-28）**：按设计收口定的批次链 1 落地（wire 变更，禁兼容旧键）：
> ① 16 个响应 DTO 去内嵌 `code`/`message`（32 字段行）——成功响应回到平台契约
> 直返语义（`response.Success` 直返业务 JSON，前端 normalize 零消费已核实）；
> 修正设计文档笔误（草案写「24 个 DTO」，实际 16 Response 类型）；事件项的
> 业务 `Message` 字段（`json:"message"`）不受影响（正则误删后已加回，构成
> 回归用例覆盖）。
> ② compat 路由组全删：routes.go 13 条 `/:id/*` 注册 + handler 13 个 Compat*
> 方法 + `resolveCompatInstallationID`；`registerAgentExtensionCompatRoutes`
> 更名 `registerAgentExtensionRoutes`（agent wire 端点保留——puller 消费方，
> 只是命名去误导）。
> ③ agent puller 同步：`extensionSyncAPIResponse` 仅存 `payload` 包装（与基线
> §3.4 一致）；mock 响应里遗留 code/message 键由 JSON decode 宽容容忍（该测试
> 保留兼作多余字段容忍回归）。
> ④ `listExtensionPages` 切 canonical `GET /installations/:id/pages`（响应
> `{pages:[...]}`，字段 route/key/title/order/icon/source），DomainEntry 改
> 「先取首个安装实例再拉页面绑定」并移除 `.catch` 静默（契约错位+吞错=页面
> 入口恒 Empty 的根因）；渲染 path→route。
> 门禁：go build ./... + go test ./internal/... 154 包全绿（extension/handler/
> agent 三包重点复验）、tsc 0 错、extensions 套件 16/16、guard PASSED、全量
> jest 2-worker 限流（load 17+）；gofmt 全仓净。
> **已知边界（诚实清单）**：① Extensions 四页面（Store/Installations/
> AgentSync/DomainEntry）此前零测试文件，本批仅 API 层回归，页面级真实渲染
> 用例归批次 4；② DomainEntry 在无安装实例时不再请求 pages（行为变化：此前
> 恒请求恒空），错误提示语义不变；③ agent 同步仍为 HTTP 轮询（演进项不在
> 批次链内）。
> 下一批：批次 2（displayName join catalog / healthStatus 推导）。

## 扩展安装详情抽屉覆盖批次（Extensions 簇缺口首发，2026-09-28）

> **交付（2026-09-28）**：覆盖率快照定位 Extensions 簇为 web 侧最大零测试目录
> （约 2858 行 0%，无任何测试引用），首批收口簇内最大单文件
> `Extensions/Installations/InstallationDetailDrawer.tsx`（522 行）——新增
> `__tests__/InstallationDetailDrawer.test.tsx` 17 用例，v8 口径行/分支/函数/
> 语句 **4×100%**。锁定契约：打开加载链（detail → 真实 adapter 兜底 →
> schema/config 各自失败静默、卸载 cancelled 竞态）、概览四项（启用态/
> 健康/版本/绑定数含缺省兜底）、基本信息五行（displayName 空回退
> extensionId）、Schema 预览（title 兜底 key、type Tag、required 标、
> 字段值 null 兜底、无 required 键、无 schema 空态）、绑定表行渲染与空态、
> 工具栏四动作（健康检查 ok/unknown 兜底、测试连接、运行能力 Modal 两态、
> 保存配置 JSON 双解析校验/空串 `|| '{}'` 双右翼/载荷/onSaved 链）、加载
> 未就绪点击守卫（`if (!target) return` 四翼经 pending 期点击真实触达）、
> canExtensionsManage=false 只读形态、onClose 回调与 open/row 守卫。
> 门禁：目标套件 17/17 绿、tsc 0 错、eslint 干净；全量 jest 门禁与负载口径
> 见交付说明。
> **已知边界（诚实清单）**：
>
> 1. 组件加载链 try/finally 无 catch：detail 接口 reject 时产生 unhandled
>    rejection（现状行为，不改组件）；守卫翼改经「加载 pending 期点击」
>    真实触达，不造假 reject 场景。
> 2. getByDisplayValue 对 node.value 折叠空白但期望串不折叠——multiline JSON
>    回显断言一律用正则（首次踩坑记录）。
> 3. Extensions 簇余量（Store/index 512、Installations/index 431、
>    EventsDrawer 286、columns 231、UpgradeModal 151 等）留待后续批次。

## 游戏/环境授权防线 + 新增游戏入口（OPEN-ISSUES #47，2026-09-28）

> **背景（用户双质询）**：① `/system/environments` 为何有独立游戏选择器而不用全局的——「授权游戏就可以编辑其他游戏这不对的」；② 为何看不到新增游戏的入口。
> **诊断**：① 页面 `loadGames` 优先全量 `listGamesMeta()`（GET /games）并自带可自由切换的 Select，仅单向订阅 scope；后端 games 域 envs 端点唯一防线是功能权限点 `games:manage`（RBAC 不按游戏切分），游戏/环境授权表 `admin_game_env_scopes` 只作用于 `/profile/games` 可见性过滤——质询属实（越权编辑面 + 未授权游戏可见）。② `POST /games` 后端 + `upsertGame` 前端封装都在但零 UI 消费（BUG-021 同款）。
> **交付（2026-09-28）**：
> 前端：环境页删独立 Select 与全量列表消费，数据源改 `listMyGames()` 授权视图（与全局选择器同源）；目标游戏恒等于 `scope.gameId`（业务 game_id），未选/未授权两空态引导；新增「新增游戏」入口（`useAccess().canGamesManage` 门控 ModalForm：name/aliasName/description），成功后广播 `games:changed`（GameSelector 监听即时重拉）并刷新授权列表。`envs.ts` 四函数 gameId 参数放宽 `number | string`。
> 后端：`internal/api/game/helpers.go` 新增 `resolveGameID`（数字主键或业务 game_id 双形态寻址——授权视图无数字主键）、`gameEnvScopes`/`authorizeGameEnv`（admin 直过；非 admin 须命中 `admin_game_env_scopes`）、`filterEnvItemsByScopes`；envs 四端点接线：EnvsList 按授权过滤（无授权=空）、EnvAdd 校验游戏维度（新增环境尚不存在）、EnvUpdate 新旧环境都须授权（改名即扩权防线）、EnvDelete 校验目标环境。
> 门禁：go build + `go test ./internal/...` 全绿（game 包新增 `service_envscope_test.go` 6 用例：无授权 403/有授权过/Update 双环境授权/Delete 单环境/List 过滤/业务串寻址 404）、tsc 0 错、GamesEnvs 21/21、全量 jest 落盘日志核绿。
> **已知边界**：① 游戏本体 CRUD（List/Detail/Update/Delete/Create）仍只受 `games:read`/`games:manage` 功能权限点保护，不做游戏维度过滤——游戏元数据（名称/别名）视为平台级信息，环境数据才是租户隔离面；② `GET /games` 全量列表端点保留（GamesEnvs 已不消费；Ops/AnalyticsFilters、Dev/ConfigExplorer、Admin/Announcements、Permissions/UsersV2 仍用），其余页面后续按需收敛；③ 前端「新增环境/编辑/删除」按钮无按钮级权限隐藏（后端授权防线兜底 403），仅「新增游戏」按 canGamesManage 门控。
> 下一步：#48 账号安全策略（site settings 板块，默认全关）。

## 账号安全策略板块（OPEN-ISSUES #48，2026-09-28）

> 用户原话：「设置中为啥没有账号安全呢？比如强制所有人开启二次验证，密码必须多少位，有没有大写字母 特殊符号之类的，密码有效期之类的，默认不开启」。交付=五键策略（默认全关）+ 密码校验接线 + MFA 强制三件套 + SiteSettings「账号安全」Tab。
> 后端：`internal/platform/settings/layered.go` 新增 `security.*` 五键（mfaRequired/passwordMinLength/passwordRequireUppercase/passwordRequireSpecial/passwordMaxAgeDays）注册 ValidKeys/boolKeys/intKeys + `SecurityPolicySnapshot`/`Layered.SecurityPolicy()`；`GET /api/v1/site/security` 快照端点。密码链：`utils.ValidatePassword` 尾接 `applySecurityPolicy`（策略只收紧不放宽，min<8 不生效），三入口（admin.Create/PasswordReset/profile.ChangePassword——后者原为新密码零校验缺口，本批补上）统一生效；`PasswordExpiresAtFromPolicy`（days>0 → UTC now+days）接 `adminNeedsPasswordChange` 既有 mustChangePassword 链（过期登录强制走改密流程）。
> MFA 强制：`mfaSetupRequired(provider, otpEnabled)`（仅 local + 未绑定 + 策略开启；外部身份源由 IdP 负责不强制）→ 登录响应 `mfaSetupRequired`；`AuthMiddleware.mfaGate`（策略开启时未绑定账号除白名单外一律 403 `{"error":"mfa_required"}`；白名单 `/api/v1/auth/mfa` + `/api/v1/profile` 保留绑定与改密恢复通道；adminID=0/存储故障 fail-open 不锁人；OTPEnabled 30s 进程内缓存）；前端 Login 收到标记 → warning + 引导 `/profile?tab=security`。
> 前端：SiteSettings 新增「账号安全」Tab（`SecurityTab.tsx`：3 Switch + 2 InputNumber 逐键保存，空值/关/0 = clearSiteSetting 回默认；locales security.* 双语 15 键）；`sites.ts` 增 `SecuritySettings`/`fetchSecuritySettings`。**收口时发现并修复 saveKey 键路 bug**：Form.Item name 用 `item.field`（如 `mfaRequired`）而取值读 `item.key`（`security.mfaRequired`）——路径不匹配恒 undefined、保存永远走清除分支；修为 `saveKey(key, field)` 双参。
> 测试：`layered_security_test.go`（默认全关/L3 覆盖热生效/Clear 回默认）、`password_policy_test.go`（基线不放宽/收紧三态/有效期矩阵/Current 透读）、`mfa_setup_required_test.go`（判定矩阵：local+未绑定+策略开才强制）、`mfa_gate_test.go`（403 拦真实未绑定账号/白名单放行/adminID=0 fail-open/已绑定放行——初版用库中不存在的 adminID=42 撞 fail-open 分支语义错位，改为 fixture 建真实 OTPEnabled=false 账号）、profile 旧用例翻转向（原断言「空密码允许」正是本批修的缺口）；SecurityTab 9 用例（回填/缺省回零/加载失败/开关 set-clear/数字 0 清除/失败两态）。
> 门禁：go build + `go test ./internal/...` 全绿、tsc 0 错、SecurityTab 9/9、全量 jest 落盘核绿、guard PASSED。
> **已知边界**：① OTPEnabled 查询失败 fail-open（存储故障宁可放行不锁全部管理员），缓存 30s 内策略关闭不即时生效于已拦请求；② mfaGate 只拦 HTTP API，agent/SDK TCP 通道不受影响；③ 密码有效期只在新改密/建号时写入 expiresAt，存量账号不回溯（下次改密起算）；④ 策略收紧不放宽：minLength 策略值小于内置 8 时按内置执行。
> 下一步：需求清单 #49-57 立项排队（系统信息/系统公告/身份验证+OAuth/系统维护/性能参数/日志维护/SMTP 归运维/安全与限制/第三方探针）。

## 系统信息板块（OPEN-ISSUES #49，2026-09-28）

> 用户原话：「有些网站设置是属于系统信息，应该包含系统名称，也就是网站名称，服务器地址，一般是域名 徽标url 异步任务对外地址，文档链接 也叫 关于，首页内容 用户协议 隐私政策等」。既有「站点信息」Tab 已纳管 name/logo/favicon/description/footer 六键，本批补齐缺的六键并整体更名「系统信息」。
> 后端：`layered.go` 新增 `site.{serverUrl,taskPublicUrl,docsUrl,homeContent,userAgreement,privacyPolicy}` 六字符串键（ValidKeys 白名单）+ SiteSnapshot 六字段（lowerCamelCase omitempty）+ builder 逐键 provenance；公开快照 `GET /api/v1/public/site` 即时下发（协议全文本就是给访客看的，无脱敏面）。
> 前端：`sites.ts` SiteConfig 六字段；SiteSettings「站点信息」→「系统信息」（12 键全矩阵：逐键保存/来源徽标/恢复覆盖机制复用 fieldWithActions）；登录页消费面（均按配置存在才渲染）：homeContent 欢迎区段落 + docsUrl 新窗口外链 + userAgreement/privacyPolicy 弹窗全文（pre-wrap + 60vh 滚动，同一弹窗复用按入口切换）。
> 测试：`layered_notify_test.go` 快照用例扩六键（L3 透传 + provenance）；SiteSettings index 8 用例（保存钮 6→12、扩展字段回填/保存/空值守卫）；新增 `siteInfoFooter.test.tsx` 3 用例（齐全渲染/协议切换/Portal 挂 body）。**测试坑**：本地覆写 @umijs/max 时 useModel 必须给 `initialState.siteConfig` 嵌套形状（平铺无效）；antd Modal 关闭动效 jsdom 不走完，断言 Portal 可达而非 DOM 卸载。
> 门禁：go build + `go test ./internal/...` 全绿、tsc 0 错、全量 jest 落盘核绿、guard PASSED。
> **已知边界**：① serverUrl/taskPublicUrl 本批仅存储+快照暴露，真实消费（通知外链拼装/任务地址展示）归 #52/#53；② docsUrl 登录后侧入口（头像菜单/页脚）归 #52 系统维护展示；③ 协议展示暂限登录页，#51 自助注册落地时同步接入；④ 首页内容消费面=匿名首页（登录页），登录后无独立首页路由，若后续做工作台首页（#52 系统维护）再接。
> 下一步：#50 系统公告核对 → #51 身份验证+OAuth → #52 系统维护 → #53 性能参数 → #54 日志维护 → #55 SMTP 归运维 → #56 安全与限制 → #57 第三方探针。

## 扩展商店页 + 安装列表页覆盖批次（Extensions 簇缺口第二批，2026-09-28）

> **交付（2026-09-28）**：按上轮台账继续收口 Extensions 簇两个最大页面文件——
> `Store/index.tsx`（512 行）与 `Installations/index.tsx`（431 行，含连带
> `columns.tsx` 231 行真实渲染）。新增
> `Store/__tests__/index.test.tsx` 17 用例 + `Installations/__tests__/index.test.tsx`
> 17 用例，v8 口径：**Installations/index.tsx 与 columns.tsx 行/分支/函数/语句
> 4×100%**；Store/index.tsx 行/语句 100%、函数 19/20、分支 92.4%。
> 锁定契约——安装列表页：概览五项统计（items 派生含作用域去重）、7 列渲染
> 矩阵（displayName/status/healthStatus/targetId/updatedAt 空值兜底、健康
> error 红翼）、筛选（扩展 ID 输入/状态下拉 → request 载荷、生效 Alert + chips、
> 清空筛选复位）、刷新、withReload 成功/失败两翼（extractErrorMessage 兜底）、
> 启停按 row.enabled 分派 enable/disable、重建绑定、卸载 confirm 流（成功/
> dependency_blocked blockers warning/普通错误/空 blockers/非 HTTP unknown 兜底）、
> 三 overlay 受控开合与 onUpgraded/onSaved → reload、canExtensionsManage=false
> 菜单 aria-disabled 门控——三个 overlay（EventsDrawer/UpgradeModal/详情抽屉）
> mock 为桩聚焦页面契约；商店页：草稿/提交双态筛选（草稿不触发拉取、查询
> 提交三态、params 未变 reload 兜底、重置清双态）、7 列矩阵（displayName 兜底
> name、kind/版本空兜底、标签金标+slice(0,3)+全空 '-'、安装按钮 canManage+
> installed 双门控）、详情弹窗加载链（adapter fallbackItem 兜底）、安装弹窗
> 链（latestVersion 兜底链 detail→releases[0]→item、manifest.configSchema 双
> typeof 提取、setFieldsValue 五字段预填、SchemaFields 默认值回显、成功载荷含
> schema 默认 config）、configJson 非法拦截/合并覆盖、四错误码分支 + details
> 缺省 `-`/unknown 兜底、displayName 空成功 message 兜底 name、定位 Alert
> history.push——InstallModal/CatalogDetailModal/SchemaFields 真实渲染（Form
> 实例页面持有，mock 掉会使 validateFields 永远空值）。
> 门禁：目标套件 34/34 绿、tsc 0 错、eslint 干净；全量 jest 门禁与负载口径
> 见交付说明。
> **已知边界（诚实清单）**：
>
> 1. Store/index.tsx 分支 92.4% 余 4 处均不可达：latestVersion 兜底链 `|| ''`
>    尾翼（item.latestVersion 空时预填空串被 releaseVersion required 规则拦截，
>    实测「请选择版本」）；`if (!installItem) return` 守卫（OK 按钮仅在弹窗
>    打开且 installItem 已设时可见）；标签列 `row.tags || []` 右翼（tags 类型
>    必填，undefined 仅运行时防御）；request `kw ?? ''` 右翼（params 恒传
>    keyword 字符串）。
> 2. Store 函数 19/20：InstallModal `onCancel` 关闭翼未断言——antd6 Modal 未开
>    destroyOnHidden 关闭后壳残留标题，关闭态文本断言不可靠（坑 8 同源），
>    不造假断言。
> 3. openDetail 的 try/finally 无 catch（详情接口 reject 产生 unhandled
>    rejection，现状行为与上轮详情抽屉巡检结论一致）：reject 场景不造假，
>    adapter fallbackItem 兜底翼改用 resolve `{}` 覆盖。
> 4. EventsDrawer（286 行）/UpgradeModal（151 行）在本页套件中为桩组件，本体
>    覆盖留 Extensions 簇后续批次（簇余量收口顺序：EventsDrawer →
>    UpgradeModal → AgentSync/DomainEntry）。
> 5. antd6 交互坑新增实证：Select 无 `.ant-select-selector`（mouseDown 直接落
>    `.ant-select` 根）；可见 option 行无 role（点击须落
>    `.ant-select-item-option-content`，role=option 只在 a11y 隐藏 listbox）；
>    menuitem 禁用形态是 aria-disabled（jest-dom toBeDisabled 不识别）；
>    modal.confirm 标题双渲染（`.ant-modal-title` + `.ant-modal-confirm-title`，
>    断言须 selector 收窄）。

## 覆盖率巡检批次·Go 侧第七轮·CI/CD 域收口（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：派发「清单 #11/#10/#2/#26/#27 全部 MERGED，转覆盖率
> 补强，找缺口最大的包补单测」。先收尾工作区 6 份弃置在途测试（mtime 停在
> 09-28 05–07 点、近 22h 无人动，与 main 无重叠）→ 提交 `4d8c913`，api/function
> 与 api/resourcecatalog 双包随之 100.0%（version_history_handler 排序双不可
> 字典序翼、空白变体去重翼、ListDistinctVersions 缺表 500；categories handler
> 聚合/scope 隔离/空数组非 null/缺表 500）。收尾时发现两处需修正：①
> `coverage_contract_version_test.go` 未格式化（gofmt 补齐）；②
> `handler_coverage_test.go` 注释以行号锚定生产路由（已随上游漂移到
> 1042→1069），改为按函数名锚定。
>
> 随后按缺口排序开工：全仓 32 个包 <100%，最大可动缺口为
> **internal/api/cicd 50.2%**（OPEN-ISSUES #58 落地时只补了 service 层，
> handler.go 九个端点整段 0%）。回避面（已核实为在途/他人域）：identity 88.2%
> 与 auth 90.1%（他会话 05:03–05:24 持续落新测试文件）、announcement 96.8%、
> ops 98.3%（同上）、auth/mfa·security/otp·assignment/gate·extension 簇、
> model 95.0%·sitesettings 95.6%（缺口零散且与在途文件同包）。
>
> 补齐 `handler_coverage_test.go`（310 行 9 用例）+ `service_paths_test.go`
> （506 行 17 用例）→ **api/cicd 50.2% → 97.9%**，handler.go 十方法
> 100%（List 除外，见登记），service/webhook 残余 5 块全部定性：
> ① CRUD 主链（建/列/改/测连通/删，断言掩码不回传明文、空 token 不覆盖、
> 删后 items 非 null）；② 路径 id 非法 10 例（abc/0 × 五端点，400 且不
> 触达 service）；③ 绑定失败 7 例（bool/map 收错类型、截断 JSON、query 整型
> 收非数字）；④ service 错误透传（记录不存在 404 / 缺表 500 / 建表被拦）；
> ⑤ 创建业务校验三例（未知 kind、非 http endpoint、provider 必填键缺失）；
> ⑥ 触发全链（假 CI 服务器 201+Location → queued 落库 → 刷新 running →
> 停用后 400）；⑦ 连通性三翼（4xx 算可达、传输失败 ok:false、畸形 endpoint
> 构造期失败）；⑧ DB 关闭态两读端点 500。
>
> service/webhook 注入口径（沿用本仓既有批次）：
>
> - 读翼「缺表」：DropTable 后 gorm 立即报错且无副作用；
> - 写翼「触发器拦写」：BEFORE UPDATE TRIGGER + RAISE(ABORT)，覆盖「校验
>   全过、SQL 真执行才炸」这一类（Update 落库、删除级联、构建状态回写三处）；
> - 记录缺失走真实 404；provider 出错用 httptest 假 CI 服务器按例返回
>   4xx/5xx；provider 构造失败用「库里直写校验层造不出的形态」（未知 kind、
>   畸形 endpoint）绕过 Create 校验。
>
> **一处已发现缺陷（本批未改生产代码，如实登记）**：
> `POST /cicd/integrations` 显式传 `enabled:false` 时**落库仍为 true**。
> 根因：model.CicdIntegration.Enabled 带 `gorm:"default:true"`，GORM 对「有
> default 标签且值为零值」的字段在 INSERT 中省略该列、回退 DB 默认值；服务层
> `row.Enabled = *req.Enabled` 赋值正确但被驱动层丢弃。实测 `Select("*")` 与
> 逐字段 Select 均无法绕过（本仓 GORM 版本无条件回退）。影响面仅「创建即
> 停用」一条路径——Update 走 Save 写全字段，实测正常。修法须把模型字段改
> `*bool`（连带 service 读侧，并触发仓库「模型改动须配编号迁移」契约），
> 属独立修复批次，已写跳过用例锁定契约（修好后该用例转绿，删除 t.Skip）。
>
> **四处不可达分支登记（房规：不造假用例、不删防御分支）**：
>
> 1. `handler.go List` 的 ShouldBindQuery 错误分支：IntegrationListRequest
>    仅两个 `form` string 字段，gin form 绑定无失败路径；
> 2. `service.go normalizeExtra` 的 `case float64`：入参只有两个来源——
>    JSON 列读出（datatypes 启用 decoder.UseNumber()，数字是 json.Number，
>    见 datatypes@v1.2.7 json_map.go:48）与进程内刚构造的行（值来自
>    map[string]string，只可能是 string），都不产出 float64，数字实际走
>    default 分支（Marshal 同样得 "42"）；
> 3. `service.go Trigger` 的 `if build.ExternalID == ""`：上一行已被
>    `firstNonEmpty(ref.ExternalID, fmt.Sprintf("trigger-%d", UnixNano))`
>    兜底，两参不可能同时为空——实测 provider 返回空 Location 时确实落到
>    本地生成的 `trigger-*`（登记用例锁定了这一形态）；
> 4. `webhook.go IngestWebhook` 尾部 GetByID 错误分支：上一行
>    UpsertByExternalKey 已成功返回 id，要让「写成功→紧接着读失败」成立需
>    在同一次请求两次存储调用之间注入存储故障，生产路径不存在此形态。
>
> 门禁：触及文件 gofmt 干净、go vet ./internal/... 干净、go test
> ./internal/... 全绿（fresh）、api/cicd fresh 全绿 97.9%。
> **已知边界**：门禁在负载高位窗口执行（并行会话持续占机，全量 internal
> 属环境性慢，非回归）；本批零 web 触碰故未单跑 jest/tsc（guard 覆盖
> PageSpec 侧校验）。

> **提交链路核验（并行会话代提交后的复核）**：本轮两个提交
> （`4d8c913` 收尾 6 份弃置在途测试、`be6bab4` cicd 域 50.2%→97.9%）由共享
> 目录内的并行会话代为 commit（我的两轮测试文件当时已被它在提交信息中
> 完整描述）。复核结论：① 两提交当时**尚未进 main**，需本会话推送；
> ② 其间的折叠提交 `ebd6193`（`-s ours`）曾使 `docs/design/
mobile-companion-design.md` 停留在旧版（较 main 少 67 行），已由本次
> merge 同步修正（取上游新版，无冲突）；
> ③ 逐文件核实 6 份收尾测试 + 2 份 cicd 测试在 HEAD 中**全部在位**，
> 无内容丢弃。
>
> **门禁补充**：`go test ./internal/...` 全量中 `internal/api/page` 一次
> 失败——panic 于 `stale_heal.go:66` healScopes 拿 nil `*gorm.DB`（后台
> stale-heal goroutine 撞上被测 DB 已关闭），发生在高负载窗口（1 分钟负载
> 126）。**单跑复核 300s 全绿**，且该域零改动（`git diff HEAD -- internal/
api/page/` 为空，源文件最近改动是 09-25 的 main 提交），判定为负载性偶发
> 而非回归；我涉及的 4 包在全量中均 `ok`。合并后复跑 cicd 97.9%、
> resourcecatalog 100.0%，gofmt/vet 干净，guard PASSED。

## 第八轮：cicd/providers 域覆盖收口 73.5%→99.8% + cicd 抽象 95.0%→100.0%

> **本轮两提交已推送**：`4d8c913`（6 份弃置在途测试收尾）、`be6bab4`
> （api/cicd 50.2%→97.9%）、`1d41707`（ops/systeminfo 100%）、
> `4a6127b`（model 99.8%，并行会话所提）、`f33b9f5`（台账）经
> `git push origin HEAD:main` 纯快进落地（`8a5337d..f33b9f5`，16 提交）。
> 合并后复跑 `api/function`、`model`、`platform/approvals` 三包全绿
> （EXIT=0）——即已进 main 的内容在合并态下复验通过。
>
> **本轮目标**：`internal/cicd/providers` 包（四个外部系统适配面，provider
> 是与 Jenkins/GitLab/GitHub/generic 通信的唯一边界，错误分支回归价值最高），
> 73.5% 是当时可动缺口里的最大者（`api/cicd` 上一轮已收至 97.9%）。
> 回避在途域：identity 88.2% / auth 90.1% / announcement / ops /
> auth/mfa / security/otp / assignment/gate / extension 簇。
>
> **新增 `internal/cicd/providers/edge_paths_test.go`（20 顶层用例，含矩阵
> 子用例共 77 例）**，收口四类系统性缺口：
> ① **四 provider 的 `Kind()` 全为 0%**（外部系统类型标识面，此前无人碰）；
> ② **构造默认兜底**：Config.HTTP/Now 缺省回落 `http.DefaultClient`/
> `time.Now`、jenkins 裸 token（无 `user:` 前缀）形态；
> ③ **出站错误域四族**：传输失败（恒失败 RoundTripper 一次打穿四个
> provider 的全部 `client.Do` 错误分支）、URL 解析失败（`http://exa mple.com`
> 能过「必须 http(s) 前缀」校验但 `http.NewRequest` 解析炸）、非 2xx、
> 畸形 JSON（`{"id":` / `[` / `nope` / `{"status":` 等十处）；
> ④ **状态映射全矩阵**：generic 24 词（大小写/空白不敏感、未识别→unknown）
> / gh 12 组（status × conclusion，含 conclusion 缺失与 neutral 等未知结论）/
> gitlab 10 词，外加 RFC3339 时间字段四形态（null / 空串 / 非法串 / 合法串）。
>
> **顺带收口**（同域，`internal/cicd` 95.0%→**100.0%**）：`Register` 对
> 空 kind / nil 工厂的 fail-fast panic 契约此前无用例（既有 `TestRegister_
DuplicatePanics` 只覆盖「同名重复注册」那一处 panic）——新增
> `internal/cicd/register_guard_test.go`，并断言失败的注册尝试不污染
> 注册表 `Kinds()`。
>
> **不可达登记（一处，不造假用例不删防御分支）**：
> `gitlabci.go:75` `isAllDigits` 的 `s == ""` → `return false` 分支。
> 该函数唯一调用点 `projectEsc()` 的入参 `p.project` 来自
> `init()` 的 `strings.TrimSpace(cfg.Extra["project"])`，为空时构造期已被
> `gitlab-ci: 缺少 project` 拦下；构造成功后该分支无法到达。保留为防御
> 分支（未来若新增非工厂构造路径即生效），登记备查。
>
> 门禁：gofmt 干净、`go vet ./internal/cicd/...` 干净、
> `go test ./internal/cicd/... -count=1` 全绿（cicd 100.0% /
> providers 99.8%）、`scripts/localized_text_guard.sh` PASSED。
> **已知边界**：门禁只覆盖本轮触及的 cicd 域（零 web 触碰故未跑 jest/tsc），
> 未重跑全量 `go test ./internal/...`（该命令在并行会话占机的高负载窗口
> 需 30+ 分钟，环境性慢）；上一轮全量的唯一失败 `api/page` 已单跑复验为
> 负载性偶发。

## 覆盖率巡检批次·Go 侧第九轮·在途收尾 + secguard 域收口（wt-api worktree，2026-09-29）

**在途收尾（上一轮半途产物）**：`internal/api/ops/systeminfo_fetch_test.go`
（138 行 4 用例）——`systeminfo.go` 残余 8 块收口至 **100%**（六函数全绿）：
更新源 settings 真实读取体三翼（未初始化 / L3 未配置 / 已配置去空白）、出站
三翼（sec.allowPorts 白名单静态拦截 / 传输层连接失败 / 响应体中途断连
unexpected EOF）、清单无可用版本字段的错误分支（缺失 / 非字符串 / 纯空白）。
ops 包整体 98.6%（残余在 logs.go / performance.go / probe.go，属 #53/#54/#57
他会话刚落地域，本轮未触碰）。已推 wt-api：`ebd6193..1d41707`（纯 FF）。

**本轮新增**：

1. `internal/security/secguard/secguard_coverage_test.go`——secguard
   **74.3% → 100.0%**（secguard.go 28 块 + retry_probe.go 9 块全收）：
   七键解析（Resolve 非 nil 路径 + 未配置零值 + 缓存路径幂等）、端口 scheme
   缺省（含 strconv.Atoi 溢出回落）、HTTPClient 派生副本不污染共享
   `http.DefaultClient`（#56 契约）、超时/重试/退避钳制矩阵（负值→0、
   999→10）、拨号钩子非法地址/非法 IP、ipAllowed 跳过非法 CIDR 且不中断后续
   条目、DoWithRetry（退避缺省 / 坏请求 / 5xx 重试并逐轮重建 body+header）、
   Probe 重定向跳数（3 跳内放行 + 超限报错）、ProbeSMTP 脚本化假 SMTP 服务
   7 态（成功 / SSRF 放行 / SSRF 拦截 / 问候缺失 / EHLO 拒 / NOOP 拒 / 拨号失败）。
2. `internal/model/email_verification_test.go`——email_verification.go
   **2.2% → 100%**（3 用例，增量口径见下）：TableName 契约；
   `CreateInvalidatingActive` 事务**第二段**（`tx.Create`）错误翼（sqlite
   `BEFORE INSERT` trigger 拦写——缺表注入只能打中第一段 UPDATE）并锁定
   回滚不变量（旧令牌不得被误作废、新令牌不得残留）；`Consume` 事务**首段**
   （used_at UPDATE）的 DB 错误回传。

**主动收敛的重复投入（如实登记）**：`email_verification.go` 的主干用例
（签发即作废 / 四态查找 / 消费幂等 / 重发频控时间源 / 缺表错误翼）已由并行
会话 `internal/model/cicd_email_models_test.go` 同批覆盖。本文件初版写了
13 条重叠用例，发现后收敛为"只补其未覆盖的三处"，避免双份维护与符号撞车
（已核双方 helper/用例名零重叠）。**并集证据**：全包 fresh 覆盖率下该文件
七个函数全 100%。

**缺口排查结论（内部 API/模型层排行，fresh 覆盖率）**：最大缺口集中在
刚落地的 CI/CD 域（`api/cicd` 4.7% / `model/cicd_build.go` 1.3% /
`model/cicd_integration.go` 2.0% / `cicd/providers/*` 67-77%），已由并行会话
连做两轮收口（见第七、八轮台账）；`api/extension`（handler 84.4% /
service 96.7%）属插件域批次链在途；`api/auth/*` 与 `api/announcement/*`
有他会话在途测试文件（`oauth_register_handler_gap_test.go`、
`register_verify_gap_test.go`、`announcement_db_error_test.go`），一律回避。
本轮取无冲突的 secguard 域（#56/#57）作为落点。

**本机环境注记（影响用例构造，已按实测调整）**：① 本机解析器把任意短主机名
劫持到 198.18.0.0/15（基准段，非 Go 私有段故不触发 SSRF 判定），`.invalid`
反而解析成功——DNS 解析失败用例改用 >253 字符超长主机名触发本地解析器直接
失败，不依赖外网 DNS；② `http.Redirect` 对伪造的空 `*http.Request` 不写
Location 头（客户端收到 EOF），改直写响应头构造重定向链；③ 端口片段
`notaport` 在 `url.Parse` 阶段即被拒（不可达），Atoi 溢出回落改用 20 位数字端口。

**门禁**：gofmt 干净、`go vet ./internal/...` 干净、`go test ./internal/...`
fresh 全绿（158 包零 FAIL）、guard PASSED。**已知边界**：门禁在负载 30-87
高位窗口执行（model 包单跑 64s~447s 波动，一次整包跑在 447s 时报 FAIL、
复跑即绿，判为环境性慢非回归）；本轮零 web 触碰，故未单跑 jest/tsc
（guard 已覆盖 PageSpec 侧校验）。

## 第九轮：extension catalog/release 写路径覆盖收口（repo 80.3%→100.0%，core service 38.2%→100.0%）

> **落地**：`30042bb`（本轮）+ 并行会话的 `97d6f8c`（secguard 74.3%→100%、
> email_verification 2.2%→100%）经 `git merge origin/main` 同步上游 7 个
> 提交（`f6cc599` PR #73 / wt-pages 折叠 tip / 文档死链修复）后，纯快进
> 推送到 main（`f6cc599..2da24e0`）。合并无冲突——上游改动落在 docs/ 与
> web 测试，与本轮 Go 写路径测试零重叠；合并后复跑本轮三域
> （`repo/gorm/extension`、`core/extension/catalog`、`cicd/...`）全绿。
>
> **本轮目标**：队列里的次大缺口。`internal/core/extension/catalog` 38.2% 与
> `internal/repo/gorm/extension` 80.3% 的缺口**是同一组方法**——catalog/
> release 写路径在两层都整段 0%（service 的七个委托方法 + repo 的七个
> 仓储方法），故合并为一批做完。扩展域的 `api/extension` 侧由并行会话在途
> （`catalog_writes_paths_test.go`），本轮只碰 core/repo 两层，目录不相交。
>
> **repo 层（`internal/repo/gorm/extension/catalog_release_writes_test.go`）**
>
> - 主链：批量读（缺席 id 直接不返回而非报错）、按列更新、版本命中、
>   级联清版本只清目标扩展；
> - 语义锁定三条：`DeleteByExtensionID` 走 `Unscoped` 物理删除才释放
>   `extension_id` 唯一索引（软删行会继续占用 → 重登记冲突）；Update/Delete
>   行不存在返回 `gorm.ErrRecordNotFound`；`ReleaseRepo.DeleteByExtensionID`
>   无匹配行**不**报错（级联语义）；`GetByExtensionIDs` 空入参短路不打 DB；
> - 错误域：缺表 + `CREATE TRIGGER ... BEFORE UPDATE/DELETE ... RAISE(ABORT)`
>   打穿 `res.Error` 分支，并断言拦截错误**不退化**为 `ErrRecordNotFound`
>   （否则写翼注入故障会被误读成「行不存在」）；
> - **契约发现并锁定**：`idx_extension_release_version` 是普通复合索引而**非**
>   唯一索引——repo 层写重复 `(extension_id, version)` 不报错，唯一性校验确由
>   调用方承担（与 `PublishRelease` 的服务层注释一致）。已写成契约用例：若日后
>   收紧为唯一索引，该用例会失败并提示同步收紧调用方校验。
>
> **core service 层（`internal/core/extension/catalog/service_writes_test.go`）**
> 七个委托方法（ListByExtensionIDs / Create / UpdateFields / Remove /
> PublishRelease / ReleaseByVersion / RemoveReleases）三形态覆盖：
> ① **nil 接收者**（读路径 ListByExtensionIDs 回 `nil,nil`，写路径回
> `gorm.ErrInvalidDB`——方法内先判 `s == nil` 再判仓储）；
> ② **仓储缺失守卫**：catalog 侧与 release 侧分别以 `NewService(repo, nil)`
> 与 `NewService(nil, repo)` 构造，验证各方法只依赖自己那半边仓储；
> ③ **真实库主链 + 仓储错误透传**（缺表、`ErrRecordNotFound`、唯一键冲突）。
>
> 门禁：gofmt 干净、`go vet` 两包干净、`go test` 两包 `-count=1` 全绿、
> `scripts/localized_text_guard.sh` PASSED。**已知边界**：本批零 web 触碰，
> 未单跑 jest/tsc（上游同批带进 web 测试改动，属上游提交的范围）。

## 覆盖率巡检批次·Go 侧第十轮·api/ops 收口 98.6%→100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29，提交 `ba27e79`）**：api/ops 残余 26 块全部收口——
> probe.go 9 块（wecom/feishu 渠道经 httptest 假目标、SMTP 已配置探活主链）、
> performance.go 5 块（绑定错误双翼含 jsonNumber 非数值解错、L3 写拒 500、
> svcCtx.StartTime 可达分支）、logs.go 12 块（nil 守卫×2、写失败、绑定×2、
> PurgeBefore 缺表与直清三表逐表错误翼、nil 快照早退、轮转目录不可读降级）。
> **关键技法入档**：ProbeSMTP 的 Hello 阶段阻塞等 220 问候（连接无读超时），
> 假目标必须是裸 `net.Listener` 真 SMTP 形态（220 问候 + 逐行 250 应答）——
> HTTP httptest 服务器一句话不发会把用例挂死（首轮 600s 包级超时实证）。
> sitesettings 有他会话 v10 在途文件（06:14 落盘）已按认领窗口协议回避。
> 门禁：gofmt 干净、go vet 干净、api/ops 包 fresh 全绿 100.0%；
> 全量 `go test ./internal/...` 未在高负载窗口重跑（test-only 单包改动，
> 158 包基线 06:12 全绿）。

## 覆盖率巡检批次·Go 侧第十一轮·svc 包收口 99.3%→100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：svc 包残余 19 块全部收口
> （`internal/svc/coverage_e_svc_wings_test.go`，11 用例）：
> migrations.go 8 块——0036/0037/0038 三迁移的 wrapGorm 探针失败翼
> （复用 C/D 批 `probeFailingDBC`）+ CreateTable/AddColumn 拒写翼
> （复用 `writeRefusingSQLiteDBC`；0037 的 AddColumn 翼须手工
> `CREATE TABLE admins (id integer primary key)` 最小表——AutoMigrate
> 带出全列会让 HasColumn 恒真到不了目标分支；0038 的第二表翼先以可写
> 连接预建 cicd_integrations 再开只读，使首个 CreateTable 通过、
> cicd_builds 缺表触达失败）；
> service_context.go 11 块——ResolveDays 闭包两翼（settings 单例空窗短路
> (0,0) / 就绪后读取 L3 RetentionDays，两阶段 Sweep；闭包在
> NewServiceContext 构造期定义、352 行另有后台 ticker 循环但间隔不可依赖，
> 须显式调 Sweep）、引导管理员 phone 档案回填主链（users.json phone 键 +
> 存量行 Status:1 跳过状态同步）与回填被拦降级翼（BEFORE UPDATE 触发器
> RAISE(ABORT)，错误只 warn 不上抛、档案保持原样）、AuthMiddleware.Handle
> 完整链触达 mfaGate 403（ResetGlobalSecretForTesting 即时换键 + Sign 版本
> 对齐 + newMFAGateFixture 真实未绑定账号）、cachedOTPEnabled 三翼
> （缓存命中 version 0/1 直答不回源 / TTL 过期回源并刷新缓存——回源后
> 库值翻转不再影响结果即为刷新证据 / admins 缺表 FindOne 出错 fail-open）。
> **发现并登记的 profile 陷阱实例**：闭包未覆盖块的行号语义——初版测试
> 只盖了 nil 翼，profile 显示 346-347（非 nil 翼）仍 0 覆盖；闭包内分支
> 各自独立成块，逐块核对而非按区间推断。
> 回避他会话在途域不变：api/announcement、api/auth×2、api/extension、
> api/sitesettings、security/identity。
> 门禁：gofmt 干净、go vet 干净、svc 包 fresh 全绿 **100.0%**
> （全包零未覆盖块）、全量 `go test ./internal/...` fresh 复跑
> （低载窗口 load~10 执行）。

## 覆盖率巡检批次·Go 侧第十二轮·platform/settings 收口至 100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：svc 收口后全仓 fresh profile 重排（157 包 ok；
> 唯一 FAIL 为他会话 sitesettings 在途 WIP 用例，符号核实只存在于其
> 未跟踪 handler_gaps_v10_test.go），排除在途域后最大无冲突缺口为
> `platform/settings/layered.go` 7 块，新增
> `layered_withsource_wings_test.go` 2 用例收口至包 **100.0%**：
> getBoolWithSource——L3 raw 非 bool 解析失败翼（字符串形态经
> store.Set 直写 `\"yes\"` + Reload）与 L2 命中翼
> （ConfigInput.FeatureFlags → resolveL2 真实产出触达）；
> getIntWithSource——非法键翼、raw 既非数字也非数字串的解析失败翼
> （布尔形态：int64 与 string 两段 Unmarshal 均败）、L2 命中翼。
> **口径登记**：resolveL2 目前只产出字符串与五域布尔键、整型键无生产方，
> getIntWithSource 的 L2 翼以白盒直构 `&Layered{l2Values:…}` 锁层契约
> （未来 L2 产出整型键时行为已定），注释注明生产不可达原因。
> 门禁：gofmt 干净、go vet 干净、settings 包 fresh 全绿 100.0%
> （13.7s，零未覆盖块）。本批单包 test-only，未重跑全量
> （20 分钟前全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第十三轮·api/page 收口至 100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：第十二轮 profile 重排后 api/page/service.go
> 5 块 + handler.go Resources 错误翼 1 块收口，新增
> `coverage_e_resources_wings_test.go` 3 用例，包 **99.9% → 100.0%**
> （零未覆盖块）：Resources 双守卫翼——无 pages:read 族权限直接拒绝；
> username 在库但请求上下文缺 game scope（fixture 自带 scope 值，须手动
> 构造 `context.WithValue(bg, \"username\", …)` 绕开——直接传
> context.Background() 会先撞 requirePageRead 的 LoadCurrentAdmin 失败，
> 走错翼）；page_specs 缺表存储错误透传；handler 层错误翼 400。
> functionResourceIndex——nil Service 与无 DB 连线双早退翼、
> function_contracts 缺表按空索引降级（不 panic 不报错）。
> 门禁：gofmt 干净、go vet 干净、api/page 包 fresh 全绿 100.0%（67s）。
> 本批单包 test-only，未重跑全量（同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第十四轮·internal/handler 收口至 100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：routes.go registerUploadStaticRoute 残余
> 5 块收口，新增 `routes_upload_static_test.go`，包 **100.0%**
> （零未覆盖块）：挂载成功主链（GET /uploads/avatars/* 路由注册 +
> avatars 目录未建自动创建）；MkdirAll 失败翼（avatars 路径被同名
> 文件占位 → warn 后不挂载）；LocalAvatarDir 解析失败翼（相对 baseDir
> 须 filepath.Abs→Getwd——chdir 进已删目录构造 Getwd ENOENT；本包
> 核实无 t.Parallel 用例，顺序执行下进程级 cwd 操纵安全，结束恢复）。
> 门禁：gofmt 干净、go vet 干净、handler 包 fresh 全绿 100.0%（1.5s）。
> 本批单包 test-only，未重跑全量（同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第十五轮·api/game 收口至 100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：helpers.go 3 块 + service.go EnvsList 透传翼
> 1 块收口，新增 `helpers_envscope_wings_test.go`，包 **100.0%**：
> gameEnvScopes 双错误翼（上下文无身份 LoadCurrentAdmin 失败 /
> admin_game_env_scopes 缺表包装 CodeError 不裸传 SQL 错误）+
> authorizeGameEnv 与 EnvsList 对底层错误的透传翼（权限与游戏寻址
> 都过后撞存储故障）。
> **坑实证（共享库毒化）**：api/game 的 setupTestDB 是
> `file::memory:?cache=shared` 进程级单例——对其 DropTable 会毒化
> 全部后续用例（首轮实证 5 例 envscope 用例连环 500/缺表报错），
> 缺表注入必须独立命名内存库（`file:<unique>?mode=memory&cache=shared`）。
> 门禁：gofmt 干净、go vet 干净、api/game 包 fresh 全绿 100.0%（34s）。
> 本批单包 test-only，未重跑全量（同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第十六轮·executionlog + service 尾翼（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：两包尾翼收口——① `platform/executionlog`
> PurgeBefore 双错误翼（execution_logs 缺表直传 / task_events 缺表：
> runs 已删、events 报错仍上抛），包 **100.0%**；② `internal/service`
> canonicalJSONBytes 非法 JSON 原样透传翼（提案摘要核对面对存量坏行
> 不炸不吞），包 99.9%——Marshal 失败翼登记不可达：入参 v 来自
> json.Unmarshal 合法输出（map/slice/string/float64/bool/nil），
> json.Marshal 对这些类型无失败路径，不造假用例。
> 门禁：gofmt 干净、go vet 两包干净、两包 fresh 全绿（1.0s/7.1s）。
> 本批 test-only，未重跑全量（同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第十七轮·api/provider + api/openapi 尾翼（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：两包残余可达翼收口，各 1 新测试文件：
> ① `api/provider`（`sdkstats_wings_test.go`，2 用例）98.9% → 99.4%——
> SdkStats 的 `firstSeen <= 0` 回退翼（217-218）：注册链
> carryProviderSessionHistory 只把 `first == 0` 补 now，**负值穿透**，
> `FirstSeenUnix: -1` 是唯一能触达服务端回退翼的注入形态（断言回退
> lastSeenUnix）；MetaOptions 的 items nil → 空切片翼（254-255）：**首版
> 踩坑**——内存聚合路径 `groupMetaOptions` 恒 `make([]…, 0, n)` 返回
> 非 nil 空切片，空 store 测不到该翼；真实形态是 DB 聚合失败
> （provider_metadata 缺表）返回 nil 的 **fail-soft 契约**（聚合故障只
> 损失下拉选项、不报错不透出 null），改 `NewStoreWithDB` + 独立命名
> 内存库缺表触达。
> ② `api/openapi`（`runtime_firstseen_wing_test.go`，1 用例）
> 99.8% → 99.9%——RuntimeSources 同款 `firstSeen <= 0` 回退翼
> （330-331），负 FirstSeenUnix 注入，断言回退 lastSeenUnix（#27②）。
> **两包剩余 1 块均为既有登记不可达**：provider handler.go SdkStats 的
> ShouldBindQuery 错误分支（第四轮 sdkstats_bind_registration_test.go
> 证明性登记）、openapi service.go:710 `"fn-"` 前缀分支
> （coverage_f_test.go 证明性登记：builder 字符集 [a-z0-9._-] + Trim
> 剥首尾 .-_ ⇒ 非空结果首字符恒字母数字）。
> 门禁：gofmt 干净、go vet 两包干净、两包 fresh 全绿（0.1s/38s，
> openapi 属包体量大非环境慢）。本批 test-only，未重跑全量
> （同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第十八轮·requestbind + config 收口双 100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：全量 profile（11:16 快照）重排后，排除已收口
> 包（第 10-17 轮）与回避域（identity 90.8%/otp 98.3% 属他会话在途，
> auth/announcement/extension/sitesettings 同前），剩余可动缺口仅两处，
> 各 1 新测试文件收口：
> ① `common/requestbind` 93.8% → **100.0%**（`lcfirst_wing_test.go`）：
> query.go lcFirst 空串早退翼（107-108）——空 tag 名不进 rune 切片；
> 顺带锁定「仅首字符折叠、其余保留」契约。
> ② `internal/config` 97.5% → **100.0%**（`local_provider_wing_test.go`）：
> LocalEnabled 归一化方法整段 0%（964e40b 05:00 新增、消费方在
> api/auth providers.go，本包内零覆盖）——nil → true（默认启用，停用
> 须显式 false，防 YAML 省略键误停本地登录锁死）、显式 true/false 透传。
> 门禁：gofmt/vet 干净，两包 fresh 全绿 100.0%（0.02s/0.02s，零未覆盖
> 块）。本批 test-only，未重跑全量（同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第十九轮·admin + approval 双 100.0%，profile/model 尾翼（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：全量 profile 剩余非回避块逐一定性后，
> 可达翼 4 处收口（4 新测试文件）+ 不可达翼 2 处登记：
> ① `api/admin` **100.0%**（99.83% 起）：PasswordReset 善后 Update 错误翼
> （354-355）——map 目标列 UPDATE 拦截回调（本包自建，参照 profile 包
> registerFailUpdateCallback 形态），UpdatePassword 的列名写法不受影响；
> 断言密码本体已换（UpdatePassword 先行成功）且 must_change_password
> 标记保持原样（善后失败不假装干净）。
> ② `api/approval` **100.0%**（99.61% 起）：recordApprovalAudit 的
> resultKind/taskId 富化两翼（331/334）——同包直调 + 内存审计断言
> details 落库（operation-logs 审批动作过滤的结构化上下文数据源）。
> ③ `api/profile` 99.8%（99.74% 起）：ChangePassword 善后 Update 错误翼
> （400-401），复用本包 registerFailUpdateCallback 只拦
> must_change_password 列。剩余 179.6 登记不可达：seenGrant 去重
> continue 翼要求同名角色并存，Role.Name 带 uniqueIndex、任何 DB 路径
> 造不出重名。
> ④ `model` 99.9%（99.78% 起）：FindEmailsByDomain 整函数 0% 收口
> （#51c 新增）——LIKE 域后缀主链 + Unscoped 语义锁定（软删行邮箱
> 仍参与查重，防删号后原邮箱被别名重复注册漏报）。剩余三处均登记
> 不可达：bug.go 374/401（第三轮既有登记，Scan 错误防御）、
> function_contract_model.go:458（StableContentDigest 的 Marshal 失败翼：
> payload 为全字符串结构体 + normalizeJSONContent 输出（nil/string/
> Unmarshal 基础类型），json.Marshal 无失败路径——与第十六轮
> canonicalJSONBytes 同构证明）。
> **menu/service.go:381 登记不可达（零改动）**：构建循环里的父级
> 二次 check 恒真——check(item) 递归覆盖父链且 accessible 记忆化一致，
> check(item)=true 时 check(parent) 必为 true（防环语义下先落 false 再
> 递归，可见性契约已由 coverage_f_test.go 脏数据用例锁定）。
> 门禁：触及包 gofmt/vet 干净、四包 fresh 全绿（26s/3s/14s/55s，
> model 属包体量大非环境慢）。本批 test-only，未重跑全量
> （同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第二十轮·approvals + app/agent 双 100.0%（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：全量 profile 剩余最后两处非回避可达翼收口：
> ① `platform/approvals` **100.0%**（99.96% 起）：
> defaultPostJSONWithHeaders 的出站安全守卫拦截翼（760-761）——
> settings 单例铺 L3（sec.allowPorts="80"）后非白名单端口（:9999）在
> CheckURL 即被拒、不发起真实连接；注入键选 allowPorts 因端口判定在
> DNS 解析前，规避本机解析器劫持短主机名的坑（第九轮台账）。本包无
> t.Parallel（已核实），settings 单例操纵安全。
> ② `app/agent` **100.0%**：updateLoop 退出 defer 的 timer.Stop 非 nil 翼
> （364-365）——debounce 10s + 单条消息创建 timer 后立即取消 ctx，
> select 仅 ctx.Done 就绪 → 退出路径必经 defer Stop（既有用例到期后
> nil 再取消，只盖 nil 翼）；exited channel 断言 loop 随取消退出且
> 无 sync 出站。
> 门禁：gofmt/vet 干净、两包 fresh 全绿 100.0%（1.5s/28s，零未覆盖
> 块）。本批 test-only，未重跑全量（同日全量基线 157 ok）。

## 覆盖率巡检批次·Go 侧第二十一轮·全量收官核验 + ops 绑定翼登记（wt-api worktree，2026-09-29）

> **全量 fresh profile 收官核验**：第 10-20 轮战役后全树
> **99.8% → 99.9%**（全量重跑 12:14，157 包 ok + 1 FAIL——
> sitesettings 的 TestSendTestEmailRealSendPath panic，符号核实仅存在
> 于他会话未跟踪 handler_gaps_v10_test.go，非已交付代码，维持既有
> 分类不碰）。非回避域剩余 19 块全部有归属：12 块为既有登记不可达
> （cicd×4 / gitlabci:75 / provider:134 / openapi:710 / menu:381 /
> profile:179 / fn_contract:458 / bug.go×2 / service:1752 /
> objstore:128 / certificates:202），identity/otp/auth/announcement/
> extension/sitesettings/assignment-gate 为他会话域回避。
> **本轮新增**：上游合并 f63acca（ops 单设备详情端点）带进的
> handler.go:352-354 绑定错误翼登记不可达——OpsNodeDetailRequest 仅
> 一个可选 string 字段（无 binding 约束），GET 的 query 兼容绑定无
> 失败路径（第四轮 provider SdkStats 同构证明）；新增
> `node_detail_bind_wing_test.go` 证明性用例锁定「任意 query 绑定
> 永不失败」前提（%zz 畸形转义/脚本串/重复键四形态）。
> **非回避域巡检至此收官**：所有剩余块均已收口或登记，新缺口只能
> 来自后续新落地面（48h 回补口径）。
> 门禁：gofmt/vet 干净、api/ops 包 fresh 全绿 99.9%（唯一剩余块即
> 本轮登记项）。本批 test-only，未重跑全量（本轮核验本身就是全量）。

## 覆盖率巡检批次·Go 侧第二十二轮·cmd/server 小文件 CLI 翼（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：internal/ 非回避域收官后转战 cmd/ 树——
> cmd/server 73.0% 为最大真实余量（pkg/pb 生成代码按惯例不进覆盖率
> 口径）。本批收小文件簇（认领窗口核实无未跟踪撞车），新增
> `cli_wings_r22_test.go`（8 用例），包 **73.0% → 76.0%**：
> ① healthCmd.RunE 三形态——sqlite 全配置（DB/JWT 已配置双 ✓ 翼 +
> bootstrapDataDir 覆盖翼）、空 DataSource 双警告翼（driver=auto 回落
> sqlite 默认 data/croupier.db，t.Chdir 隔离防污染包目录；NewServiceContext
> 全量启动在测试内完成，与 svc 包用例同先例）、配置缺失错误翼；
> ② validateCmd.RunE——multiGame 启用打印翼 + 长密钥掩码尾 4 位 +
> maskIfSet 三翼直测（空/≤8/长）+ 配置缺失错误翼；
> ③ versionCmd/PrintVersionInfo——ldflags 双形态（GitCommit/BuildTime
> 有值打印 / unknown+空串不打印行）；
> ④ completionCmd 四 shell 生成（os.Stdout 捕获防刷屏）；
> ⑤ meshForwarder 的 !OK 返回翼——self-owner 解析器触达 mesh 防御性
> `OK:false("owner is self")`，断言包装为 "remote invoke failed"；
> callerFromContext nil-ctx 早退翼（不读身份键）；
> ⑥ activeAgentIDDirectory.ActiveAgentIDs 的 ListAliveOwners 错误翼
> （嵌入 OwnerStore 只覆写单方法）；
> ⑦ dbFanoutCmd.RunE 四路径——dry-run 报告表格（(meta) 行 + total=1）、
> 配置缺失错误、DSN=目录（open meta database 错误上抛、空表仍打印）、
> **迁移拒写 → ErrFanoutFailures**：DSN 带 `_pragma=query_only(1)`
> 使每条连接只读（Open/ping 成功、首个 CREATE TABLE 被拒）——错误经
> 报告通道（status=error）而非 RunMigrationFanout 返回值，正是命令层
> for-loop 汇总翼；sqliteFileDSN 的 `_pragma=` 幂等守卫保证注入 DSN
> 原样透传（不追加 WAL pragma，规避只读库上 journal_mode 的不确定面）。
> RunE 直调经 `&cobra.Command{}+SetContext`（裸 cmd 的 Context() 为
> nil，database/sql 对 nil ctx 会 panic）。
> **登记不可达（三处，不造假用例不删防御分支）**：
>
> 1. completion.go:59 default 臂——Args=cobra.OnlyValidArgs 在 RunE 前
>    拦截四词之外一切参数，证明性断言锁定前提；
> 2. mesh_forwarder.go:47-48 NotOwner 翼——res.NotOwner 仅由真实远端
>    peer 在 stale-epoch fencing 路径设置（ServeForwardHandler），单机
>    构造不可达，cluster 层 interconnect e2e 覆盖同语义；
> 3. mesh_forwarder.go:53 成功载荷返回——需活 peer 环路（transport 拨号
>    - 应答），同上。
>      门禁：gofmt/vet 干净、go build ./... 通过、cmd/server 包 fresh 全绿
>      8.5s（含两次 NewServiceContext 全量启动）。本批 test-only 单包改动，
>      未重跑全量（同日全量基线 157 ok + 1 他会话 WIP FAIL）。
>      **cmd/server 余量（后续轮次）**：root.go 110 块、dashboard_fixture.go
>      70、service.go 56、dashboard_fixture_cmd.go 9、cluster.go 5、
>      dashboard_fixture_provider.go 2；随后 cmd/agent 89.6%、
>      cmd/analytics-export 87.7%、cmd/ingest 99.5%。

## 覆盖率巡检批次·Go 侧第二十三轮·cmd/server cluster + fixture 命令翼（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：接第二十二轮继续收 cmd/server 小文件余量，
> 新增 `cluster_ddl_wings_test.go`（3 用例）+ `fixture_cmd_wings_test.go`
> （4 用例），包 **76.0% → 77.9%**：
> ① **owner 表 EnsureTable 失败翼（cluster.go:115）**——两连接构造：
> 可写连接预建 cluster_instances 后以 `mode=ro` 重开，成员表
> EnsureTable 对已存在同构表纯读比对幂等通过、owner 表 CREATE 被拒
> → standalone 降级。**推翻 coverage-exemptions.md cmd-5 的旧论证**
> （「单连接无法构造」——两连接即可分叉两表 DDL 命运），豁免项收窄为
> 仅 NormalizeConfig，三处 stale 注释同步（cluster.go 源内、
> cluster_extra_test.go、豁免文档，文档附收窄记录）；
> ② **reconcile 循环 Touch 失败翼（170）**——BEFORE UPDATE 触发器
> 只拦 cluster_agent_owners：成员表 Register/Renew（INSERT/UPDATE）
> 不受影响、集群正常启动，归属行 last_seen_at 保持陈旧即续期失败的
> 可观测后果（断言锁定）；reconcileTickerInterval 注入点 20ms 驱动；
> ③ **互联 Serve 错误翼（221）**——cancel 而不 Close：Accept 的 1s
> deadline 到期后走 ctx.Done → 返回 Canceled（非 nil）→ goroutine
> 记 warn（既有用例先 srv.Close 走 closing 通道返回 nil，结构性到不了）；
> ④ **fixtureCmd.RunE 全链 + 启动失败翼**——错误翼：BaseDir 被文件
> 占位 → MkdirAll 失败上抛；全链：真实 server+agent+SDK+provider
> 起全栈，**就绪锚定 FIXTURE_READY 行**（stdout 管道探测 + 持续排空
> 防子进程写满管道）——首版锚 healthz 实证竞态：HTTP 监听先起、命令
> 内 signal.Notify 尚未注册，SIGINT 只被测试侧预注册通道吃掉，RunE
> 60s 不退出；修正后幂等重发 SIGINT 覆盖注册窗口，RunE 干净走完
> 清理与 Close；
> ⑤ provider 两翼——PUT name-only（192，既有 CRUD 只动 level）+
> readBody nil Body（237，http.Server 恒非 nil，零值 Request 直测
> helper 契约）。
> **登记不可达（三处）**：cluster.go:68 NormalizeConfig err（恒 nil
> error，cmd-5 保留）、cluster.go:439 nil 会话 continue
> （LoadActiveSessions 值扫描不产出 nil 元素）、
> dashboard_fixture_cmd.go:58 CleanupScope 失败 Fprintf（需运行中
> 破坏 fixture 库写路径，无确定性注入面）。
> 门禁：gofmt/vet 干净、go build ./... 通过、cmd/server 包 fresh
> 全绿 13.5s、`cd docs && pnpm build` 通过（豁免文档变更联动）。
> 本批 test-only + 注释/文档同步，未重跑全量（同日基线）。
> **cmd/server 余量**：root.go 110、dashboard_fixture.go 69（boot
> 步骤错误翼群）、service.go 56；随后 cmd/agent 89.6%、
> cmd/analytics-export 87.7%。

## 覆盖率巡检批次·Go 侧第二十四轮·cmd/server root.go 装配面（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：接第二十三轮收 cmd/server 最大余量文件
> root.go（110 块），新增 `root_wings_r24_test.go`（8 用例），包
> **77.9% → 91.3%**，root.go 余 14 块全部登记归属：
> ① **runServer 全链 ×3 boot 矩阵**——mode(prod/test/default) ×
> debug × logLevel × gin 三模式一次排满（boot A test+logLevel=debug →
> 131+146-150+gin TestMode；boot B prod → 129+ReleaseMode；boot C
> default+debug → 133+137-143+DebugMode），boot C 另经全局 port/host
> 覆盖翼（119-124，预选空闲端口防 os.Exit 竞态）；同 boot 内集群 db
> 协调面（SetHeartbeatOwnerLookup/ActiveAgentDirectory/
> SetRemoteForwarder/SetRemoteAgentSource/SetTaskAgentLookup 五注入翼）
> 与遥测启用（199-208 装配 + defer Shutdown 主链 + wrapHTTPHandler
> 中间件翼）一并触达；就绪锚定 "Starting Croupier Server at " 同步
> 打印行 + SIGINT 幂等重发（第二十三轮 fixture 同款竞态消除法），
> 优雅停机全序列（TCP listener→HTTP drain→rootCancel→ControlService
> →Router.Close）真实走完；
> ② 廉价翼群——runServer 配置缺失翼与 rootCmd.RunE / server 别名
> 闭包（坏 cfgFile 三入口共享）、startControlServer 地址归一两翼
> （""→:19090→0.0.0.0、':'前缀拼接；19090 被占时落到 TLS 同款错误
> 翼，断言容忍两态）+ TLS 证书缺失失败翼（确定性构造，不赌端口
> 冲突）、startRegistryCleanup nil store 翼、三个 env 覆盖函数
> （storage/cluster/auth-secrets）nil 配置翼 + 全部注入体、
> validateAndAdjustTimeout 矩阵（自定义/缺省间隔 × 调整/通过两翼）；
> ③ wrapHTTPHandler 遥测翼以**行为证明**锁定（otelhttp.NewHandler
> 自身也返回 http.HandlerFunc，类型断言与 testify 函数值比较均不可
> 用）。
> **两枚新教训（入档）**：
>
> 1. **全局 slog 指向启动管道的 closed-file panic**——runServer 的
>    SetupLoggerWithFile 替换进程级 slog 默认 logger 并捕获当时
>    os.Stdout（测试管道 w）；用例收尾 Close(w) 后，后续任意用例的
>    slog.Info → coloredTextHandler → isTerminal → 对 closed
>    *os.File Stat 得 nil FileInfo → 解引用 SIGSEGV（实测炸在
>    AddrAndTLS 用例的 StartBackgroundTasks）。处置：管道**留开不
>    Close**（排空 goroutine 持续消费）+ 矩阵结束 slog.SetDefault
>    恢复原 logger。internal/cli/common/logging.go:108 的
>    `fileInfo.Mode()` 无 nil 防护是潜在生产隐患（closed-file 场景
>    任何进程内 slog 都会炸），如实登记备查，本批不改。
> 2. **TelemetryConfig yaml tag 是遗留 snake_case**（enable_tracing/
>    collector_url）——键按 lowerCamelCase 写会静默零值；首版遥测
>    失败翼用例因此没触发、卡在 runServer 的 <-quit（挂死而非失败）。
>    修正法：任何「期待 runServer 早退」的用例必须带就绪行兜底分支
>    （就绪即 SIGINT 收尾 + 报构造失效），杜绝挂死。
>    **遥测 init 失败翼证伪登记（198-199）**：internal/telemetry 侧
>    临时探针六形态扫描（坏 OTEL_RESOURCE_ATTRIBUTES ×2 / 畸形
>    collector URL ×2 / metrics 畸形 URL / headers 坏 JSON）——
>    NewGameTelemetryService 全部 EAGER 返回 nil error：OTLP HTTP 客户端
>    不预解析 endpoint（首次上传才失败）、resource 解析宽松。构造路径
>    无可注入失败面，登记不可达（探针用例已移除，不留测试垃圾）。
>    **登记不可达（14 块全数归属）**：Execute/main ×3 块（cmd-1 进程
>    边界）；HTTP ListenAndServe 错误翼 348-350（分支体 os.Exit(1)，
>    进程内构造即杀测试二进制，cmd-1 同族）；遥测 init 198-199（上段
>    证伪）+ Shutdown 错误翼 205-206（需失败后端）；优雅停机内四个
>    错误子翼 371-372/377-378/391-393（listener Close/HTTP Shutdown/
>    Router Close 成功路径无注入面）+ 超时翼 401（需挂死组件 + 30s
>    等待）；prune ticker 体 486-489（30s 硬编码 + 5min 陈旧阈值，无
>    注入点）；tcpListener.Serve 非 Canceled 翼 497-498（Close→nil、
>    cancel→Canceled 被过滤，无第三形态）。
>    门禁：gofmt/vet 干净、go build ./... 通过、cmd/server 包 fresh
>    全绿 14.3s（含 4 次完整 NewServiceContext 启动）。
>    **cmd/server 余量**：dashboard_fixture.go 69（boot 步骤错误翼群，
>    E2E 基础设施口径既有登记）、service.go 56（cmd-2/6 域）；随后
>    cmd/agent 89.6%、cmd/analytics-export 87.7%、cmd/ingest 99.5%。

## 覆盖率巡检批次·Go 侧第二十五轮·service 变更命令五体收口（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：接第二十四轮收 cmd/server service.go（56 块）+
> cmd/agent service 域（57 块）——两包各新增 `service_wings_r25_test.go`
> （agent 30 子用例 / server 33 子用例），经 `newKardianosService` 包级
> 接缝注入可控 fake（status/statusErr + per-method installErr/
> uninstallErr/startErr/stopErr + 调用计数），**变更命令五体分支矩阵
> 全数直测**（不碰真实 systemd）：
> ① install——创建失败/已存在早退（计数断言 Install 未触达）/Install
> 错误/成功（agent 打 `Platform()[:1]`、server 打完整平台串，两包契约
> 各自锁定）；
> ② uninstall——创建失败/状态查询失败/不存在/运行中 Stop 错误/Stopped
> 形态卸载错误/运行中成功（Stop→2s 等待→Uninstall，sleep 翼随成功路径
> 覆盖，全轮仅 2 处 2s 臂）；
> ③ start——状态查询失败/不存在提示 install/已在运行早退（Start 计数
> 为 0）/Start 错误/成功；
> ④ stop——状态查询失败/不存在/已停止早退（无 ✅ 成功标 + Stop 计数
> 为 0，与成功翼以标点+计数双锚区分）/Stop 错误/成功；
> ⑤ restart——状态查询失败/不存在/运行中 Stop 错误/运行中 Stop 干净
> 后 Start 错误（2s 等待翼）/stopped 直启（Stop 计数为 0，status !=
> Running 分支）。
> cmd/server 侧另补两块：**Start 后台 goroutine 的 panic 恢复翼**
> （154-156：runServerFunc 替身 panic → recover → svc.Stop，既有用例
> 只盖错误返回与成功两翼；cmd/agent 侧同位翼已被既有用例覆盖）与
> **wd() 的 Getwd 失败翼**（595：chdir 进已删除目录构造 ENOENT——
> 全包无 t.Parallel，顺序执行下进程级 cwd 操纵安全，结束恢复）。
> 包口径：**cmd/agent 89.6% → 98.9%、cmd/server 91.3% → 95.1%**
> （fresh 全包）。cmd-2 豁免随之**收窄归零**：接缝之下五体的每个
> 错误/早退/成功分支均可构造，「真实系统级变更」只对被替换掉的未注入
> 路径成立——coverage-exemptions.md cmd-2 改写为收窄记录（无豁免块
> 保留），读数段同步（server ≈95 / agent ≈99 / analytics-export
> 87.7 / ingest 99.5）。analytics-export 的 3 块经核实为 main() 体
> （flag 解析 + panic），cmd-1 既有枚举已覆盖登记，无需新条目。
> **service.go 剩余 6 块全部归 cmd-6 既有登记**：os.Executable 守卫
> （agent 255-256 同位）、Abs 内层回退翼、windows StartType 分支、
> defaultConfigDir 系 windows/darwin case、exePath 失败回落 "unknown"
> （cmd-6 位置清单本轮补显式枚举）。
> 门禁：触及文件 gofmt 干净、go vet 两包干净、go build ./... 通过、
> 两包 fresh 全绿（4.7s/19.1s，含 4 次 NewServiceContext 完整启动 +
> 8 次 2s sleep 臂）。本批 test-only + 注释/文档同步，未重跑全量
> （同日基线 157 ok；并行会话 cargo test 占机持续，属环境性慢）。
> **cmd/ 余量**：cmd/server 95.1%（dashboard_fixture.go E2E 基础设施
> 口径 + root.go/cluster/mesh 既有登记翼）、cmd/agent 98.9%（全部为
> cmd-1/cmd-6 登记块）、cmd/ingest/cmd 99.5%、analytics-export 87.7%
> （main-only，cmd-1）——cmd/ 树非豁免余量至此收官。

## 覆盖率巡检批次·Go 侧第二十六轮·dashboard_fixture.go 装配翼收口（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：接第二十五轮收 cmd/server 最后一个大量块文件
> dashboard_fixture.go（69 块），新增 `fixture_wings_r26_test.go`，
> 包 **95.1% → 98.1%**，文件余 14 块全部登记不可达（测试头注释与
> 豁免文档同源）：
> ① **纯 helper 矩阵**——fixtureAddrWithPort 六形态（空串/裁剪 ":0"/
> 0.0.0.0 归一/localhost 保留/非数字端口契约放行/无冒号报错）、
> defaultFixtureBootstrapDir 与 fixtureSDKDir 的 Getwd 失败翼
> （chdir 进已删除目录）与候选缺失回落；
> ② **StartDashboardFixture 前置错误翼**——五个 addr 解析失败
> （startServer 之前拦截）、MkdirTemp 失败（TMPDIR 指向文件）、
> 空 BaseDir 自有目录分支与 cleanupOnError 删除链；
> ③ **五步失败翼 + listen 翼**——Sscanf（cleanupOnError 首次真实执行）、
> server listen 被占（顺带覆盖 CROUPIER_E2E_PUBLISH_REVIEW 环境翼）、
> provider/fixture API 端口被占、agent 目录被占、SDK cmd.Start 失败
> （各一次真实 startServer boot 后在目标步骤失败）；
> ④ **start\* 手工构造错误翼**——provider/fixture API listen、agent
> MkdirAll、providers.yaml 被目录占位、SDK 建目录与 go build 失败
> （GOOS=invalidos）；
> ⑤ **ensureUIScope 阶梯**——nil 模型守卫、缺表查询错、FindEnvBinding
> 缺表、AddEnvBinding 触发器拒写（含 Router 非 nil 翼——databaseName
> 改走 NameForGame）、FindByUsername 缺表、UpdateLastScope 触发器拒写；
> ⑥ **fixture API 全方法臂**——health 双态（经 UpsertAgent 注入带函数
> 契约会话后 agentConnected=true）、sdk/functions 三态（405/坏 JSON/
> 合法+BIN 目录 500）、sdk/calls 五臂、audit 三态（404/垃圾 ActorJSON
> 500 解码失败/合法 200）与查询失败翼（缺表 500）、provider 两端点；
> ⑦ **ready() 三翼**——nil svcCtx、空 store、函数缺一（Functions 少
> 一项 sdkFn）、契约缺表（函数齐备+DB 非 nil 但 function_contracts
> 缺表）；
> ⑧ **CleanupScope 六级错误阶梯**——子查询缺表×2（semantic versions/
> proposal versions）、scoped 循环缺 page_specs、admin scope 恢复被
> BEFORE UPDATE 触发器拒、env binding 删除被 BEFORE DELETE 拒、game
> 删除被 BEFORE DELETE 拒（后三处先铺真实行使 UPDATE/DELETE 命中行——
> 零行更新不报错、触发器无从引爆，首轮实证三翼全漏）；
> ⑨ **Close 运行时句柄全走**——control + 双 HTTP 服务 + telemetry +
> Router（空缓存）+ 自有 BaseDir 删除链 + 幂等。
> **登记不可达（14 块）**：fixtureFreePort Listen 错误翼及两透传翼
> （fd 耗尽不可构造）、候选循环内 Abs 失败翼 ×2、telemetry init 翼
> （二十四轮探针证伪）、startServer 内 ensureUIScope 透传翼（boot
> 自有 DB 全新迁移无注入缝，深层阶梯已直测）、四个 Serve goroutine
> 非 ErrServerClosed 日志翼、SetEnvs/startSDKLocked 的 Marshal 翼、
> WaitReady 60s 超时翼。
> 门禁：触及文件 gofmt 干净、go vet 干净、cmd/server 包 fresh 全绿
> 19.9s（98.1%）。本批 test-only + 注释/文档同步，未重跑全量
> （同日基线 157 ok；并行会话持续占机属环境性慢）。
> **cmd/ 收官状态**：cmd/server 98.1%（余量为六文件登记翼——
> root 14/service 6/mesh 2/cluster 2/fixture_cmd 1/completion 1/
> dashboard_fixture 14 全部有归属）、cmd/agent 98.9%（cmd-1/cmd-6）、
> ingest/cmd 99.5%、analytics-export 87.7%（main-only，cmd-1）——
> cmd/ 树非豁免余量枯竭，后续轮次转监控回补（新落地文件 48h 口径）
> 或 web 侧工作面。

## 覆盖率巡检批次·Go 侧第二十七轮·弃置在途收养 + identity 收口（wt-api worktree，2026-09-29）

> **在途收养（第七轮同款口径）**：工作区 6 份未跟踪测试（mtime 停在
> 09-29 05:07–06:14，近 12h 无人动；`git log --all` 证实无任何分支持有
> ——仅存在于共享工作区；其间 main 流过 15+ 提交而文件零更新）经
> vet + 全量包测试核验后收养：announcement_db_error_test /
> oauth_register_handler_gap_test / register_verify_gap_test /
> catalog_writes_paths_test / github_error_paths_test 五份原样绿色收养；
> handler_gaps_v10_test 一处 seam 误用修正——原稿 `sendTestEmailFn = nil`
> 意图「复原 default」，实则把缝隙变量置 nil（default 只存在于声明初始
> 化器），调用即 nil 函数 panic（r21 台账记录的 TestSendTestEmailRealSendPath
> panic 根因即此）；修为 NotNil 前提断言 + 直接使用当前值（tracked 注入
> 用例均经 t.Cleanup 复原，本文件按字母序先于 testemail_test.go 执行，
> 起点必为 default）。非生产缺陷，testemail.go 本体无恙。
> 收养战果：**announcement 96.8%→100.0%、auth 90.1%→99.0%、
> extension→99.9%、sitesettings 95.6%→100.0%**。
>
> **本轮新增**：`identity/wechat_generic_wings_r27_test.go`——964e40b
> 落地的 WeChat/自定义 OAuth2 两 provider 残余错误翼，identity
> **88.2%→99.2%**：两 Kind 标识（0%）、WeChat 构造缺省 base 回落
> （官方双域名 + AuthCodeURL 前缀）、generic Exchange 双层失败（token
> 端点传输失败 / token 成功后 userinfo 500 透传）、fetchUserInfo 直测
> 四翼（非法 URL NewRequest / 传输失败 / **声明 Content-Length 后断连的
> 体读取错误**——原生 listener + Hijack 形态，ReadAll unexpected EOF 的
> 唯一确定性注入 / 坏 JSON 解码）、wechat Exchange 缺 access_token/
> openid 守卫 + userinfo 500 透传、apiGet 直测同四翼。
> **新增豁免条目 #4**：wechat.go Exchange 尾部 openid 回退（134）与双
> 缺失兜底（137）——上方 122 守卫已按 TrimSpace 拒绝空 openid，通过后
> TrimSpace(token.OpenID) 必非空，两处 `if openID == ""` 恒假（自证性
> 双保险，同 "fn-" 前缀构造）；pin = TestWeChat_ExchangeWings 首臂
> （缺字段形态先于回退触发守卫）。
> 门禁：触及文件 gofmt 干净、go vet 干净、五包 fresh 全绿（37.9s 最重
> 的 auth 属包体量）。本批 test-only + 文档同步，未重跑全量（同日基线）。
> **下一轮候选**：auth 残余 9 块（email_verification 78/87/103/163、
> mfa 95、providers 114/134/162 init 失败日志翼、service 738 continue）
>
> - extension service.go:513（manifest 非 JSON 对象 400）。

## 覆盖率巡检批次·Go 侧第二十八轮·auth + extension 残余收口（wt-api worktree，2026-09-29）

> **交付（2026-09-29）**：接第二十七轮收养后的两包残余——
> ① `api/auth/providers_wings_r28_test.go`：**auth 99.0% → 99.3%**。
> 可达翼四处：BuildIdentityProviders 三处「guard 查非空、ctor 查
> TrimSpace 非空」缝——空白串凭据（ClientID/AppID=" "）过 guard、构造
> 失败 → 失效降级日志翼（github/wechat/generic 三臂各自断言 provider
> 置空且整体不报错，本地与其他登录源不受影响）；VerifyEmailToken 未知
> 令牌翼（155 row==nil → 统一「无效或已过期」，防令牌探测语义）。
> 登记不可达六处（头注释同源）：siteServerURL Current()==nil（settings
> 单例由服务装配初始化，包外无复位缝）；newVerificationToken rand.Read
> 翼及其透传翼（getrandom(2) 引导后无失败路径）；mfa.go:95 恢复码生成
> 失败翼（同 rand 族）；service.go:738 Cut 失败 continue（LIKE
> '%@'+domain 入列行必含 @，Cut 恒成功——register_verify_gap_test 脏行
> 用例已锁 SQL 层前提）；email_verification.go:163 的 !ok 并发消费翼
> （FindValidByTokenHash 过滤 used_at IS NULL，行到 Consume 前单线程
> 无变化窗口，仅并发竞态可达——收养文件既有登记）。
> ② `api/extension/manifest_object_wing_r28_test.go`：PackImport 的
> manifest 非 JSON 对象 Unmarshal 翼（内层 "manifest" 为数组/null——
> extensionPackManifest.Manifest 是 RawMessage 原样透传到落库前校验）
> 两形态收口；extension 维持 99.9%，余 1 块即收养文件登记的
> Marshal 再序列化翼。
> 门禁：触及文件 gofmt 干净、go vet 两包干净、两包 fresh 全绿
> （36.5s/19.1s，包体量非环境慢）。本批 test-only，未重跑全量
> （同日基线 157 ok）。
> **巡检状态**：收养后原回避域全数打开且已收口——identity 99.2%
> （2 块豁免 #4）、auth 99.3%（6 块登记）、extension 99.9%（1 块
> 登记）、announcement/sitesettings 100%、cicd 97.9%（4 块登记）。
> 全仓非登记缺口枯竭，转监控回补口径（新落地 48h）。

## 事件抽屉 + 升级弹窗覆盖批次（Extensions 簇缺口第三批，2026-09-29）

> **交付（2026-09-29）**：接簇余量收口顺序收 Installations 目录剩余两个
> overlay 本体——`EventsDrawer.tsx`（286 行）与 `UpgradeModal.tsx`（151 行）
> 此前在页面套件中为桩组件，本体 0 测试。新增
> `__tests__/EventsDrawer.test.tsx` 12 用例 + `__tests__/UpgradeModal.test.tsx`
> 7 用例（v8 口径）：**EventsDrawer 行/分支/函数/语句 4×100%**；
> **UpgradeModal 行/函数/语句 100%、分支 97.36%**（余 1 处为 handleOk 的
> `if (!row) return` 守卫——OK 按钮仅在 open 且 effect 已按 row 拉取后可点，
> 经 UI 不可达，不造假用例）。
> 锁定契约——事件抽屉：标题（displayName 兜底 extensionId）、概览三项
> （总数副本在 request 成功后同步 + 未筛选态 chips）、六列矩阵（formatUnix
> 秒/毫秒自适应、payload 空 '-'）、无安装实例守卫（不发请求 + 默认空态）、
> 关键词/级别筛选（trim 载荷、Alert 已生效条件单/组合 ' / ' 拼接、筛选态
> 空态文案切换、清空双态复位 + 按钮禁用门）、切换安装实例重置筛选并按新
> id 重拉、request 失败静默翼（success:false 不弹错）、关闭态不挂载不拉取。
> 升级弹窗：打开拉目录版本（当前版本已在列不重复前置 / 不在列前置补齐 /
> releaseVersion 空不补三翼）、空版本提交拦截（warning 不触达写服务）、
> 成功链（upgrade → message → onClose → onUpgraded）、失败四分支结构化
> 文案（missing_dependency/version_mismatch/dependency_cycle 各带 details
> 字段缺省 unknown/'-' 兜底臂 + 非 HTTP unknown 兜底 message）、三分支
> 失败保持弹窗开启。
> **antd6 交互坑新增实证**：Select 的 placeholder 无稳定形态（span/input
> 因版本而异），锚定改为「容器内首个 .ant-select」（DOM 序上工具栏先于
> 表格分页 size changer）；option 双份 DOM（a11y role=option + 可见
> .ant-select-item-option-content），计数断言必须带 selector 收窄。
> 筛选断言口径：打开时 effect 的 reload 与首挂载请求会被 ProTable 内部
> abort 合并，计数不具确定性，一律锚「最后一次调用」。
> 门禁：目标套件 19/19 绿、tsc 0 错、eslint 干净、全量 jest 333 套件
> 4060/4060 绿（2 worker 限流 697s，load ~2.5 低位窗口直跑）。
> **Extensions 簇余量**：AgentSync/index（无测试文件）与 Store/
> CatalogManageModals、两处 shared.ts 留后续批次；DomainEntry 已由
> 4d1fbb3（#46 批次 4）收口。

## Agent 同步调试页覆盖批次（Extensions 簇缺口第四批，2026-09-29）

> **交付（2026-09-29）**：`Extensions/AgentSync/index.tsx`（93 行，簇内
> 唯一零测试页面）→ 新增 `__tests__/index.test.tsx` 6 用例，v8 口径
> **行/分支/函数/语句 4×100%**。锁定契约：页头与初始空态（暂无数据）、
> 空输入/纯空白拦截（warning + 不触达服务）、查询主链（trim 归一入参 +
> 载荷 JSON.stringify(null,2) 落只读 TextArea）、payload undefined →
> '{}' 兜底、onPressEnter 等价查询、清空双态复位（输入 + 载荷回空态）。
> **坑实证补档**：getByDisplayValue 对 value 也做默认空白归一（连续空白
> 折叠为单空格）——多行 pretty JSON 须以「单空格折叠形态」字符串断言，
> 字面换行的正则恒不匹配（此前只记了「用正则」，本例证伪并修正口径）。
> 边界（诚实）：runSyncQuery 是 try/finally 无 catch——查询 reject 产生
> unhandled rejection（现状行为，与 InstallationDetailDrawer 巡检结论
> 同族，不改组件），不造假 reject 场景；`resp?.payload || {}` 右翼经
> resolve undefined 形态覆盖。
> 门禁：目标套件 6/6 绿、tsc 0 错、eslint 干净、全量 jest 334 套件
> 4066/4066 绿（2 worker 限流 658s，load 回落 0.6 窗口直跑）。
> **Extensions 簇余量**：Store/CatalogManageModals 与两处 shared.ts 留
> 后续批次；簇内四个页面（Store/Installations/AgentSync/DomainEntry）
> 与五个 overlay 本体全部有测试。
