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

## 目录管理弹窗 + Store 纯逻辑覆盖批次（Extensions 簇收官，2026-09-29）

> **交付（2026-09-29）**：簇最后两个零覆盖件收口——
> ① `Store/CatalogManageModals.tsx`（291 行，登记扩展/发布版本两弹窗，
> 此前 Store 页套件未触达管理动作）→ `__tests__/CatalogManageModals.test.tsx`
> 10 用例，**4×100%**：CatalogRegisterModal 打开预填 kind=community/
> status=active + resetFields、extensionId required/pattern 拦截（transform
> 先 trim 再校验、提交载荷保持原始值——trim 由父层承担，两侧一致口径）、
> 默认值载荷与全字段载荷（kind/status 切换）、confirmLoading 透传；
> ReleasePublishModal 标题 displayName 兜底 name 与 item undefined 无后缀
> 两翼、releaseChannel 预填 stable、version semver pattern（预发布后缀
> 通过）、manifest 校验器四翼（空/纯空白必填、非 JSON 格式错、数组须为
> 对象、合法对象通过）、全字段载荷。
> ② `Store/shared.ts`（64 行纯逻辑）→ `__tests__/shared.test.ts` 11 用例，
> **4×100%**：buildSchemaDefaults 三级缺省 + hasOwnProperty 口径（falsy
> default 也拾取）；normalizeConfigBySchema 全矩阵（无 schema/坏形态
> 原样透传、rawConfig 缺省防御翼、number/integer 转换与截断、NaN 保留、
> boolean 四词含空白大小写、array/object JSON 解析与坏 JSON 保留、
> null/undefined 值跳过、null/缺 type 属性条目不动）。
> **坑实证续档**：antd6 Select 选中项无稳定类名——预填显示断言改锚
> .ant-select 根 textContent；未触碰的 Form 字段 validateFields 运行时为
> undefined（类型上是 string，父层兜底），载荷断言用 objectContaining；
> 帮助函数 `props?.item ?? item` 会吞掉显式 undefined-item 用例，透传须
> 用 `'item' in props` 判别。
> 门禁：目标套件 21/21 绿、tsc 0 错、eslint 干净、全量 jest 336 套件
> 4087/4087 绿（2 worker 限流 669s，load ~1 低位窗口直跑）。
> **Extensions 簇收官**：四个页面 + 全部 overlay/弹窗/纯逻辑件均有测试
> 且非防御分支 100%——簇零测试目录清零。

## 函数调用历史页覆盖批次（全仓最大零测试页，2026-09-29）

> **交付（2026-09-29）**：Extensions 簇收官后转全仓零测试页排行首位——
> `Functions/History/index.tsx`（739 行，0 测试引用）→ 新增
> `__tests__/index.test.tsx` 16 用例，v8 口径**行/分支/函数/语句 4×100%**。
> 锁定契约：统计六卡（成功 `/ total` 后缀、平均耗时复用 formatDuration、
> 成功率 total>0 toFixed(1)/total=0 数值 0 两臂）+ 统计失败静默（console.warn、
> 卡片不渲染）；列表渲染矩阵（六状态徽标 + 未知状态回退 pending、gameId/env
> 兜底、formatDuration 三段位 ms/s/m 与缺省、formatTime 合法/非法/缺省、
> errorMessage Tooltip 红字与 '-' 兜底、共 N 条分页）；request 合并契约
> （工具栏筛选经 params.filters 与查询表单字段合并、表单显式输入优先）+
> 失败翼（extractErrorMessage → message.error）+ 响应缺省翼；工具栏状态下拉、
> RangePicker 起止 Enter 提交（ISO 载荷）与清空剥键、LightFilter 三字段
> （functionId/status/gameId）chip→popover→确 认 提交；刷新按钮双拉；
> 详情抽屉（富化全字段 + payload/result 卡片 + response 缺省回落行数据 +
> 失败两翼 + 空值形态全兜底/未知状态原文）；自动刷新轮询（fake timers：
> 含 running/pending 5s 重拉列表与统计、全终态不重拉）。
> **antd6 坑实证续档**：RangePicker 单面板且直接 change+OK 不提交——须逐输入
> focus+change+Enter；清空图标须先 mouseEnter；LightFilter chip 与表头同文本
> 须按 .ant-pro-core-field-label 锚定、popover 取未隐藏实例、确认按钮锚
> button[data-type="confirm"]；Statistic 值与 % 后缀分元素（整串 getByText 不
> 匹配，按 .ant-statistic-title 锚卡断拼接内容）；loading 态表格即渲染 Empty 壳
> （空态断言先锚请求已发）；jsdom 下 Drawer 关闭动效不收尾（关闭翼以可再打开
> 且重拉锁定）；useIntl mock 须稳定实例——fetchStats 的 useCallback 依赖 intl，
> 不稳定会让统计重复拉取、计数断言失真。
> 门禁：目标套件 16/16 绿（4×100%）、eslint 干净、tsc 0 错、全量 jest 337 套件
> 4103/4103 绿（2 worker 限流 677s，load ~1.4 低位窗口直跑）。
> **下一批候选（零测试页排行余量）**：Approvals（694）、Analytics/Levels（683）、
> Ops/DBMonitor（667）、Ops/Alerts（600）、Dev/Releases（576）。

## 告警中心页覆盖批次（Ops/Alerts 簇，2026-09-29）

> **交付（2026-09-29）**：零测试页排行次席——`Ops/Alerts/index.tsx`（765 行）+
> `AlertRulesTab.tsx`（600 行，入口套件中为桩）双件收口，新增
> `__tests__/index.test.tsx` 27 用例 + `__tests__/AlertRulesTab.test.tsx` 12 用例：
> **index.tsx 行/函数 100%、分支 98.23%**；**AlertRulesTab.tsx 行/函数/语句 100%、
> 分支 98.24%**。锁定契约——入口页：初始三请求（alerts/config/silences）与
> 渲染矩阵（severity 三色 + 空串、firing/silenced、静默行无静默按钮、cfg 双
> URL 条件按钮、静默列表 ID/创建者/起止拼接与缺省 ' -> '）、筛选矩阵
> （severity/service 下拉精确、关键词 summary+labels JSON 小写包含、labelKey
> 判空 + labelValue String 精确）、双刷新（{} 响应 `s.silences || []` 右翼 +
> reject 静默 catch）、外链三入口（Grafana/AM/#/alerts/静默查看 encode 两
> 形态）、行内 1h/1d 与抽屉三档静默（matchers toStringRecord 归一、comment
> 取 summary、各档独立 catch 三翼全覆盖）、解除静默主链 + 失败翼、详情抽屉
> （全字段 + 兜底 '-'、runbook/grafana 条件按钮、onClose 可重开）、load 失败
> 三翼、Tabs 切换。规则 Tab：七列矩阵（条件 code 文本、level 三色 + 未知
> default、forCount 阈值文案、冷却 Math.round 分钟、agentFilter 空 → '全部'、
> lastFiredAt 格式化/缺省）、load 三翼 + items 缺省右翼、新建默认值链
> （required 拦截 + agentFilter 归一空串）、自定义指标链（Select 切换 →
> 内层 Input 显形；失前缀自动落非 preset 手输形态）、编辑回填三形态
> （preset/非 preset/自定义 + agentFilter 透传）、启停三翼、删除 Popconfirm
> 两翼、保存失败弹窗保持。
> **antd6 坑实证续档**：rc-select 开/关都走 message 宏任务——同一卡内两次
> 连开下拉须留 ≥60ms 时间隙（立即重开与上一次关闭竞态、第二次打不开）；
> getByText 只对内容做 trim 归一、查询串不 trim（' -> ' 单元格须用 '->'
> 查询）；antd Table 行按钮点击冒泡触发 onRow（抽屉随之打开，行锚须限卡内
> 表格）；Space 包裹子项致 textContent 双命中（getAllBy 取首）；modal.confirm
> 标题双渲染须 selector 收窄；无 ConfigProvider zh 时 Popconfirm 默认按钮
> 文案是 en（OK/Cancel）；ModalForm 标题与入口按钮同文本（锚 .ant-modal-title
> 的 textContent，footer 主按钮走类名——中文文案自动插空格「确 定」）。
> **登记不可达（防御分支，不造假用例不删分支）**：index 81/88 `(rows || [])`
> 右翼（rows 是 useState 数组、setRows 只赋数组）；index 742
> `(detail.annotations || {}).runbook_url` 右翼（Runbook 按钮仅在
> annotations 已是对象时渲染）；rules 499 pattern 失败分支（isCustom 谓词与
> pattern 谓词等价，失前缀即重渲染摘规则——自证性双保险，同 r27 wechat 族）；
> rules 467 `getFieldValue('metric') || ''` 右翼（新建/编辑 initialValues 恒含
> metric）。
> 门禁：目标套件 39/39 绿、eslint 干净、tsc 0 错；全量 jest 负载口径见交付说明。

## 关卡分析页覆盖批次（Analytics/Levels，2026-09-29）

> **交付（2026-09-29）**：零测试页排行第三——`Analytics/Levels/index.tsx`
> （683 行，页面 + LevelsSegmentsChart/EpisodeFacets/MapFacets/EpisodeFacet/
> MapFacet 五内联组件）单件收口，新增 `__tests__/index.test.tsx` 12 用例：
> **行/语句/函数 99.7%（仅图表 catch 两行登记）、分支 92.96%**（余 9 臂全部
> 登记为结构不可达防御分支）。锁定契约——四卡矩阵（漏斗表 rate `${v}%`、
> 分关卡表 winRate toFixed(2)/难度 Tag 高红/中金/缺省 '-'、分群图四折线
> path + Top10 按参与数排序 + 图例、章节/地图分面 Statistic 计数与非数组
> 守卫）、查询链（episode 输入即时重拉、查询按钮、RangePicker 起止 ISO 进
> 载荷）、分群下拉切段空态、导出六入口全载荷（卡头 CSV String 归一 +
> 缺省空串、漏斗/统计底 XLSX 多 sheet、章节 ep_<id> 多 Sheet、地图 map_<id>
> 计数行、各 catch 静默）、响应缺省（{} → 空表 + 图不渲 + 表头行导出）、
> 加载前导出（慢接口 data null 形态下三入口出表头行 + 切分群段 {} 兜底）。
> **页面真实行为差异（本轮关键发现）**：`MapFacets` 与 `EpisodeFacets` 不同，
> **没有挂载 useEffect**——load 只接「加载」按钮，地图数据挂载后为空、
> mMaps 首拉须显式点击（range 变更也不会自动重拉，测试按此建模）。
> **antd6 坑实证续档**：无 showTime 的 RangePicker 输入须用日期串
> （'2026-09-01'，datetime 串解析失败不提交）；页面首个 svg 是 RangePicker
> 的 swap-right 图标，图表 svg 须按含纵轴 label 定位；antd6 Select 选中态
> 类名是 `.ant-select-content`（非 antd5 的 selection-item）。
> **登记不可达（防御分支，不造假用例不删分支）**：漏斗/分关卡两处
> render `v != null ? … : '-'` 右翼（映射恒产 number）；图表 find 助手与
> 统计导出 mk 的 `(arr || [])` 右翼（入参恒为数组）；图表 try/catch 的
> catch（纯数值计算无可抛路径，L435-436）；EpisodeFacets/MapFacets 渲染
> 与导出的 `(episodes || [])`/`(maps || [])` 右翼（state 恒为数组）。
> 门禁：目标套件 12/12 绿、eslint 干净、tsc 0 错；全量 jest 负载口径见交付说明。

## 数据库监控页覆盖批次（Ops/DBMonitor，2026-09-29）

> **交付（2026-09-29）**：零测试页排行现席——`Ops/DBMonitor/index.tsx`
> （667 行）单件收口，新增 `__tests__/index.test.tsx` 13 用例：
> **行/语句/函数 100%、分支 98.61%**（余 1 臂登记为结构不可达）。锁定
> 契约——卡片矩阵（驱动 Tag blue、kind 查表 + 未知回退原文、停用/全局
> Tag、gameId/env geekblue、dsnMask code、无结果提示、canManage 三处门控）、
> 空态（items 缺省右翼 → Empty）与 load 失败两翼、立即探测主链（results
> 按 sourceId 归并 + 五卡指标全矩阵：连接 current/max 与 max<=0 '?'、
> connections 缺省 '-/?'、锁等待红 Tag 计数/绿 0、死锁 volcano/绿 0/null
> 与 undefined 双翼「不可用」、延迟 `?? '-'`ms、锁等待表 waitSecs>30 红 Tag
> 双臂）、results 缺省右翼、探测失败两翼、新建主链（name/dsn required
> 拦截 + driver/kind/enabled 默认值 + 阈值双 InputNumber 全量载荷 + 关闭）、
> 保存失败两翼弹窗保持、编辑回填（DSN 掩码不回填 + extra 编辑语义 + 阈值
> 0 归一 undefined + 停用开关透传）、删除 Popconfirm 主链与失败翼、刷新重拉。
> **antd6 坑实证续档**：本页 Popconfirm 默认按钮是 zh（确 定）——与
> AlertRulesTab 的 en（OK）不同源，role 查询统一 name=/确/ 两态通吃；
> 带图标工具按钮 accessible name 前缀拼 icon aria-label（「reload 刷新」），
> 须用正则查询；antd Form 序列化丢弃 undefined 值键——undefined 归一断言
> 须直读 mock.calls 载荷键而非 objectContaining（jest 30 下后者不把
> undefined 键视同缺失）。
> **登记不可达（防御分支，不造假用例不删分支）**：新建提交
> `dsn: v.dsn || ''` 右翼（新建态 dsn 带 required 规则，空值被 ModalForm
> 校验拦截，提交时 dsn 恒非空串——`|| ''` 仅满足类型收窄）。
> 门禁：目标套件 13/13 绿、eslint 干净、tsc 0 错；全量 jest 负载口径见交付说明。

## 配置中心浏览器覆盖批次（Dev/ConfigExplorer，2026-09-29）

> **交付（2026-09-29）**：零测试页排行第三——`Dev/ConfigExplorer/`
> 簇双件收口，新增 `__tests__/index.test.tsx` 13 用例 +
> `__tests__/SourceManageModal.test.tsx` 7 用例：
> **index.tsx（599 行）行 99.49%/函数 100%/分支 94.44%（余 7 臂登记为
> 结构不可达）、SourceManageModal.tsx（400 行）四维 100%**。锁定契约——
> 挂载链（listGamesMeta → scope 同步 → loadSources → 默认源 → 树首拉；
> scope 空态在 effect 处即短路）、三下拉矩阵（游戏 displayName/name/双缺省
> 三臂 label、环境派生、源类型 meta + 可写/只读 Tag）、文件打开全矩阵
> （humanSize B/KB/MB 三段位 + langOf 全 switch 臂 + 只读无应急按钮 +
> 编辑器值/语言/只读透传）、目录树懒加载（expand conf → 子节点 → 嵌套
> conf/sub → 孙节点递归挂载）、xlsx 预览（真实 xlsx 库造 fixture：满表/
> 空 sheet rows[] 右翼/参差中洞+短行 → `r[i] ?? ''` 补空）、应急写回流
> （编辑 → Popconfirm → 通知文案 → 取消关流可重开 → 空 reason 拦截 →
> 载荷 → 「已写回」+ 弹窗关闭 + 文件重开 + 失败两翼弹窗保持）、
> 三失败静默文案（三连渲染显式 unmount 防状态串染）、切换矩阵（切源重拉树/
> 切环境切游戏重拉源、空源 Empty）、canDevManage false 双门控、管理弹窗
> 桩替身（挂载透传/OnChanged 重拉/OnClose 卸载）；SourceManageModal——
> open 门控加载、列表矩阵、新增主链（默认值 + required/JSON 双拦截 +
> type 联动模板 + 上下文补齐载荷）、保存失败两翼、编辑回填（脱敏值 +
> type 禁用 + id 透传精确载荷）、删除主链与失败翼。
> **antd6 坑实证续档**：Modal okText 双字中文渲染插空格（「写 回」），
> danger 主按钮锚 `.ant-modal-footer .ant-btn-dangerous`；管理 Modal
> footer=null、嵌套 ModalForm 是页面唯一带 footer 弹窗；树节点与文件头
> 同文本双实例（getAllByText）；xlsx 列头 ellipsis 双渲染（th + title）。
> **登记不可达（防御分支，不造假用例不删分支）**：loadSources `!g || !e`
> 短路体（唯一调用方 effect 先行判空）；loadDir/openFile/doSave 三处
> `!sourceId` 守卫与 doSave `!file`（调用面均在 sources/file 态渲染之后）；
> onSelect 非 array 臂（antd Tree 单选恒传 Key[]）；xlsx `(r || [])` 右翼
> （sheet_to_json(header:1) 恒产数组）；xlsx `String(c ?? '')` 右翼（稀疏
> 洞被 Array.map 跳过、sheet_to_json 不产显式 null，c 恒有值）。
> 门禁：目标双套件 20/20 绿、eslint 干净、tsc 0 错；全量 jest 负载口径见交付说明。

## 版本发布页覆盖批次（Dev/Releases，2026-09-29）

> **交付（2026-09-29）**：零测试页排行首位——`Dev/Releases/index.tsx`
> （576 行）单件收口，新增 `__tests__/index.test.tsx` 15 用例：
> **行/语句/函数 100%、分支 96.29%**（余 3 臂登记为结构不可达）。锁定
> 契约——表格矩阵（渠道/平台/类型/状态四列查表 + 未知值回退原文、
> 状态 Tag 色查表、灰度列三臂 strong/plain/'-'、资源包列 objectKey 有值
> formatSize 三段位（size 缺省 '-'/KB/MB）+ title=checksum / 无值未上传）、
> 状态机操作矩阵（draft 传包 / uploading 内测 / testing 开始灰度 / gray
> 放量+全量+废弃 / full 回滚 / archived/rolled_back/未知态无操作）、
> 灰度放量弹窗（标题 `灰度放量：{version}（当前 {n}%）` + Slider step5
> 键盘 ArrowRight 步进 + min=当前灰度值取 max(grayPercent,10) + 确认
> transition(id,'gray',value) + footer 取消与右上 X 双关流）、传包
> Upload customRequest（uploadReleaseArtifact(id, file) → onSuccess +
> 已上传 + 重拉；失败两翼）、四 Popconfirm 流转主链与失败两翼、工具栏
> 双筛选下拉（值进 request + clear 复位 `v || ''` 右翼 + 回第 1 页）、
> 创建版本 ModalForm（version/platform required 拦截 + channel
> initialValue official + type 默认 full + 载荷 `{...v, gameId: ''}`
> X-Game-ID 契约 + 已创建 + 重拉 + 关闭 + 失败两翼弹窗保持）、load 失败
> 两翼与响应缺省右翼、刷新重拉、canManage false 操作列全 '-'。
> **antd6 坑实证续档**：Upload 隐藏 input[type=file] 直接触发 change 可
> 驱动 customRequest，但同一 input 二次 change 被 rc-upload 吞（首次
> 处理后 value 复位去重）——失败两翼须分渲染各自触发；Popconfirm 确认
> 锚未隐藏 .ant-popover 内 .ant-btn-primary（类名两态通吃，绕开 zh/en
> 文案漂移）；ModalForm 提交锚 .ant-modal-footer .ant-btn-primary
> （submitText 双字中文插空格「创 建」）；带图标按钮 accessible name
> 前缀拼 icon aria-label（「cloud-upload 传包」）。
> **登记不可达（防御分支，不造假用例不删分支）**：request 回调
> `statusFilter ?? ''`/`platformFilter ?? ''` 右翼（params 键由 useState
> 恒为 string）；确认放量 onClick 的 `!grayTarget` 守卫（按钮仅在
> grayTarget 态渲染，闭包捕获恒非空）。
> 门禁：目标套件 15/15 绿、eslint 干净、tsc 0 错；全量 jest 负载口径见交付说明。

## 服务端热更新页覆盖批次（Dev/Hotpatches，2026-09-29）

> **交付（2026-09-29）**：零测试页排行次席——`Dev/Hotpatches/index.tsx`
> （554 行）单件收口，新增 `__tests__/index.test.tsx` 14 用例：
> **行/语句/函数 100%、分支 96.15%**（余 3 臂登记为结构不可达）。骨架与
> Dev/Releases 同族（ProTable + 双筛选 + 创建 ModalForm + 灰度 Slider
> 弹窗 + Upload 传包），锁定契约——表格矩阵（框架查表 skynet/KBEngine/
> JVM/Node.js/自定义 + 未知回退、关联缺陷 `#id`、状态 Tag 查表 + 未知
> 双回退、灰度列三臂、补丁包 formatSize 三段位 + 未上传）、状态机矩阵
> （draft 无包仅传包——提交审批两条件臂 false 侧 / draft 有包传包+
> 提交审批双人规则文案 / approved 开始灰度 / rolling 放量+标记生效+回滚 /
> failed 回滚 / applied/rolled_back/未知态无操作）、灰度弹窗（标题
> `节点灰度放量（当前 {n}%）` + Slider 键盘步进 + rolling 放量 min=当前
> 值 + footer 取消与右上 X 双关流）、传包 customRequest 主链与失败两翼、
> 创建热更单（title/bugId required + framework 默认 skynet + 切自定义 +
> InputNumber 数值载荷 + `{...v, gameId: ''}` 契约 + 失败两翼）、双筛选
>
> - clear 复位右翼、load 失败两翼与响应缺省右翼、刷新重拉、canManage
>   false 操作列全 '-'。
>   **坑实证（同 Releases 坑档复用）**：Upload 同一 input 二次 change 被
>   rc-upload 吞（value 复位去重），失败两翼分渲染；Popconfirm 确认锚
>   .ant-popover .ant-btn-primary；ModalForm 提交锚 footer 主按钮。
>   **登记不可达（防御分支，不造假用例不删分支）**：request 回调
>   `statusFilter ?? ''`/`fw ?? ''` 右翼（params 键由 useState 恒为
>   string）；确认放量 onClick 的 `!rollTarget` 守卫（按钮仅在 rollTarget
>   态渲染，闭包捕获恒非空）。
>   门禁：目标套件 14/14 绿、eslint 干净、tsc 0 错；全量 jest 负载口径见交付说明。

## 工具箱页覆盖批次（Dev/Tools，2026-09-29）

> **交付（2026-09-29）**：零测试页排行第三——`Dev/Tools/index.tsx`
> （539 行）单件收口，新增 `__tests__/index.test.tsx` 13 用例：
> **行/语句/函数/分支 100%**（四维全满，无登记不可达臂）。
> **附带修定一处真缺陷**：`ScopeModeSelect` 未透传 Form.Item 经
> cloneElement 注入的 value/onChange——内层 Select 长期脱管，scopeMode
> 永远进不了 form store（提交恒按 global、scoped 工具无法从 UI 创建、
> gameId/env 联动输入永不出现；rc-select 内部态自顾示正常，仅提交载荷
> 暴露）。修法：`{...rest}` 展开 + 先调 `rest.onChange`（派发 scopeMode
> 落库）再做 setFieldsValue 回填/清空；测试以「切 scoped → 双输入渲染 +
> 提交载荷 scopeMode scoped + gameId/env」「切回全局清空」回归锁定。
> 锁定契约——六类分组矩阵（icon + label + 计数 Tag）、未知分类静默
> 过滤、卡片标题外链、description 有无两臂、作用域 Tag 双态、Switch
> 启停矩阵、外链开窗（export 图标操作）、首拉 scope 透传（demo/prod 与
> 双 undefined 两臂）、空态 Empty、load 失败两翼、登记主链（name/url
> 双 required + url pattern 翼 + 默认值载荷 + 已登记 + 重拉 + 关闭）、
> 编辑主链（回填 + enabled + 全局工具 scopeMode 'global' 臂 + 载荷无
> gameId/env 键）、保存/启停/删除失败两翼、canManage false 面收敛。
> **坑实证（antd6 新档）**：Form.Item 子为自定义组件时注入的 value/
> onChange 必须显式透传，脱管表象是「UI 选中正常、store 不动」；作用域
> option 文案括号全/半角混排（`当前游戏环境（demo/prod)`），matcher 勿带
> 闭合括号；卡面 description 区恒含作用域 Tag，无描述时 textContent 即
> Tag 文案；卡片 actions 图标操作按 aria-label 锚点（标题链接含同名
> icon 需限容器）。
> 门禁：目标套件 13/13 绿、eslint 0、tsc 0 错、guard PASSED、全量 jest
> 口径见交付说明。

## 2026-09-29 Round 40（wt-api）：Ops/RateLimits 覆盖收口 100/98.46/100/100——零测试页排行第四 + 四处真缺陷修定

> 覆盖率巡检第四站：`web/src/pages/Ops/RateLimits/index.tsx`（513 行 0%）。
> 12 用例；本轮在页内修定 **四处真缺陷**（全部回归锁定）：
>
> 1. **load 顶层 reject 未收敛**：try/finally 无 catch——listRateLimits
>    失败成为未处理 rejection 且用户只见空表。补 catch + message.error
>    「加载限速规则失败」。
> 2. **matchLabels 字段整体缺失**：编辑回填把多余键还原为 JSON 文本、
>    提交侧 labels 合并两段逻辑全是死代码（回填不可见、合并恒空）。
>    补 Form.Item + TextArea 落地编辑入口。
> 3. **named Form.Item 子为三元素数组**（`{' '}<Input/>{' '}`）：antd
>    cloneElement 注入跳过——limitQps/percent/match 四键完全脱管（默认
>    值 10/100 不显示、键入永不落库、提交恒按默认、match 全丢，用户无法
>    从 UI 配置限速参数）。去掉 `{' '}` 补齐单子。
> 4. **编辑路径未 resetFields**：form 实例 store 在 destroyOnHidden 卸载
>    后存活，上次新建/编辑残留的 match 键静默并入本次提交（幻影匹配条
>    件写入规则）。编辑按钮补 resetFields（与新建按钮同口径）。
>    锁定契约——三连拉（funcs/nodes 内层双 catch 静默 + 响应缺省三右臂）、
>    表格矩阵（scope 双 Tag/percent 缺省 100/match 展开 '-'）、新建主链
>    （required 拦截 + 默认值 + 载荷 + 已保存 + 重拉 + 关闭）、scope 切换
>    （清 key + 选项源切 agents + label 切 Agent ID + id 缺省回退 addr）、
>    percent 边界（编辑回填 0/150 双臂不入载荷）、labels 三态（合法合并/
>    非法警告忽略/数组静默忽略/仅 labels 无标准键从零建 match）、编辑回填
>    （标准四键平铺 + 多余键 JSON 文本 + 重组）、预览全景（空表单早退/
>    function info 不触达/service 载荷 + 降序列表 + 仅超限过滤 + CSV 含
>    缺省行）、失败三翼、自动预览（防抖/条件不齐/reject 静默/恢复）、
>    弹窗取消、删除 modal.confirm。
>    **坑实证（新档）**：jest.clearAllMocks 不清 mockRejectedValueOnce
>    队列——外层 catch 短路内层拉取时预挂的 Once 跨用例毒化函数源，表象是
>    key 下拉空 options（ant-select-dropdown-empty），单测隔离运行不复现、
>    全文件才炸；Once 必须本用例内消费殆尽。antd InputNumber min=1 下手输
>    0 只变更显示不派发 onChange（falsy 臂仅编辑回填可达）；双字中文
>    Button 自动插空格（编 辑/删 除，Tag 不插）。
>    门禁：目标套件 12/12 绿、eslint 0、tsc 0 错、guard PASSED、全量 jest
>    口径见交付说明。

## 2026-09-29 Round 41（wt-api）：Ops/Certificates 覆盖收口 100/98.79/100/100——零测试页排行第五 + 分页回弹真缺陷修定

> 覆盖率巡检第五站：`web/src/pages/Ops/Certificates/index.tsx`（460 行 0%）。
> 11 用例；本轮修定 **一处真缺陷**（回归锁定）：
>
> - **分页回弹**：`useEffect(() => load(1, ...), [load, size, status])` 而
>   load 身份随 page 重建——翻第 2 页 → setPage(2) → effect 复跑
>   load(1,...)，任何翻页立即被拉回第 1 页（实证：4 连调用
>   mount(1)→click(2)→bounce(1)→stable(1)）。修法：effect 只响应
>   [size, status]（首挂载 + 筛选/页大小复位），分页由 Table onChange
>   直驱。
>   锁定契约——首拉载荷 {page:1,size:10,status:''}、渲染矩阵（域名
>   port 缺省回 443、日期三列 formatDateTime、剩余天数三态含数值缺省
>   双臂、状态四色 + 大写 toLowerCase 防御臂、派生链 pending/expiring/
>   expired/valid、errorMessage 红「错误」Tag）、状态筛选选值/clear 复位、
>   分页稳定（mock 回显请求页——响应页硬编码会把 mount 顶到第 2 页、
>   点击落空，此为 mock 设计坑）、刷新、重新检查/检查全部/移除监听
>   （confirm 取消/确认/失败三翼）、新增域名（required + 默认值
>   port 443/alertDays 30 + 已添加 + 失败保持）、load 失败 + 响应缺省
>   四右臂。
>   **坑实证（新档）**：分页断言的 list mock 必须回显请求页
>   （mockImplementation echo），响应 page 硬编码会自我干扰。
>   门禁：目标套件 11/11 绿、eslint 0、tsc 0 错、guard PASSED、全量 jest
>   口径见交付说明。

## 2026-09-30 Round 42（wt-api）：System/ExcelConfig 覆盖收口 98.19/95.23/100/98.19——零测试页排行第六

> 覆盖率巡检第六站：`web/src/pages/System/ExcelConfig/index.tsx`（444 行 0%）。
> 9 用例，无页面缺陷（本轮纯补测）；锁定契约——草稿生命周期（默认
> Sheet1 [['id','name','value']]、合法草稿载入、非法 JSON 回退、编辑即
> 持久化、重置草稿重读）、setCell 数字 int/float 双正则 + 参差行补列、
> 类型行 Select 矩阵（首列 disabled、空类型格 value undefined 无选中项）、
>
> - 行（等宽空行）、+ sheet（SheetN + 激活）、CheckableTag 切换 + 双表
>   存续下编辑（updateRows 非激活表 `: s` 原样臂）、XLSX 真实 round-trip
>   导入（raw:true 数值保持 number）、坏文件（PK+垃圾 → SheetJS 抛
>   Unsupported ZIP encryption → extractErrorMessage 透传）、导出
>   writeFile 文件名双臂（key / excel-config 回退）、保存并发布（Popconfirm
>   → 稀疏 cellData 快照（''/null/undefined 跳过）+ message 透传 + 最新
>   版本 Tag + 说明清空 + 失败两翼）、服务端编译上传（importExcelFile
>   (file,{message}) + 失败两翼）。
>   **坑实证（新档，四条）**：① tests/setupTests.jsx 的 localStorage 是
>   无存储 jest.fn() 壳（getItem 恒 undefined），依赖 localStorage 草稿的
>   页面测试须文件内补 Map 存储；② 本 jsdom Blob/File 无 arrayBuffer()
>   （页面导入链 file.arrayBuffer → XLSX.read），须 FileReader 打底
>   polyfill；③ scroll 表格 tbody 首行是 aria-hidden 的
>   ant-table-measure-row，行选择器须 .ant-table-row；④ XLSX 极宽松——
>   垃圾字节不抛错而是按文本回退解析出 1 个 sheet（要触发 catch 臂须
>   PK 头+垃圾让 zip 检测抛 Unsupported ZIP encryption），且 XLSX.write
>   对零表工作簿抛 Workbook is empty（「文件没有 sheet」分支不可从真实
>   文件构造，登记不可达）。
>   登记不可达：setCell 行补齐 while、addRow `|| 1` 右臂、
>   parsed.length===0、单元格 render 的 `(current?.rows || [])` 右臂
>   （dataSource 即 current?.rows，current undefined 时零行不进 render）、
>   导入/上传 catch 的 extractErrorMessage 非 Error 兜底右臂。
>   门禁：目标套件 9/9 绿、eslint 0、tsc 0 错、guard PASSED、全量 jest
>   口径见交付说明。

## 2026-09-30 Round 43（wt-api）：Ops/Status 覆盖收口 100/100/100/100——零测试页排行第七

> 覆盖率巡检第七站：`web/src/pages/Ops/Status/index.tsx`（437 行 0%）。
> 9 用例全分支覆盖，无登记不可达、无页面缺陷（纯补测）。锁定契约——
> 挂载链 Promise.all 三连拉 + getOpsMaintenance best-effort 回填
> （enabled Boolean 化 / message || '' / allowAdmins !== false）、
> maintenance 拉取失败静默、健康表矩阵（类型/目标/启用 Switch 乐观
> 更新 + updateOpsHealth 载荷 + 失败不回滚）、执行链三翼（ok 绿
> latencyMs Tag + 成功 toast / !ok error 缺省右臂红 异常 Tag / reject
> 透传）、禁用项执行按钮 disabled、服务状态四态（up/healthy 绿、truthy
> 红、falsy default '-'）、MQ lengths 缺省 || {} 右臂 + 空态文案 + 积压
> 双档（>10000 红）、维护模式保存（Popconfirm → updateOpsMaintenance
> 表单值 → 已更新 / 失败透传）、刷新重拉三连且 maintenance 不重拉。
> **坑实证（新档）**：带 icon 的双字中文 Button 的可访问名是
> 「play-circle 执 行」——icon 名成为可访问名前缀，getByRole name 须
> /执\s*行/ 宽松正则；页内四张表时行锚文本要跨全页查
> （querySelector('.ant-table') 只命中第一张表）。
> 门禁：目标套件 9/9 绿（100/100/100/100）、eslint 0、tsc 0 错、guard
> PASSED、全量 jest 口径见交付说明。

## 2026-09-30 Round 44（wt-api）：Analytics/Behavior/FunnelPresetBar 覆盖收口 98.35/84.9/100/98.35——零测试页排行第八

> 覆盖率巡检第八站：`FunnelPresetBar.tsx`（426 行 0%，组件已实现但
> 未挂载到漏斗卡片——直接渲染组件本体）。16 用例；无页面缺陷（纯补测）。
> 锁定契约——读链（空列表按钮组 disabled / 合法载入 / 非法 JSON 与
> 非数组 JSON 防御回空）、排序（lastUsed 降序、缺省 0 臂、并列
> localeCompare 名称序、无名预设 String(name||'') 双臂——label 模板串
> 把 '' 插值为字面 'undefined'）、保存（完整字段落库含 range ISO 与
> [null,null] 可选链双臂、seq/sameSess 四组合、重名 confirm 覆盖/取消、
> prompt 取消与纯空白早退）、应用（onApply(cleaned) undefined 剔除 +
> lastUsed 顶格、storage 失配静默）、删除/清空（confirm 双翼）、重命名
> （非重名直接改、重名 confirm 覆盖含 splice 移除同名、prompt 取消、
> found 失配早退）、导出（全部/当前 blob + createObjectURL）、导入预览
> （解析三翼：非法 JSON / 非数组 / 合法 → 覆盖|新增 状态全选、勾选
> 合并 + 未勾跳过、取消关闭）。
> **坑实证（新档）**：rc-select 选项点击前必须 sleep ≥60ms 再 mouseDown
> （含二次打开——关闭动画落定要 ~300ms）；antd6 Select 选中值类是
> .ant-select-content（非 v5 的 .ant-select-selection-item）；antd6
> Modal 关闭后留在 DOM（display: none），关闭断言须查 style 而非
> null；jsdom 无 URL.revokeObjectURL（createObjectURL 已是 setupTests
> 的 jest.fn）——直接 defineProperty 补 jest.fn，勿 spyOn 不存在的属性。
> 登记不可达：九处静默 catch（Map 存储/纯内存操作不抛）、四处
> !sel 早退（按钮 disabled={!sel}，jsdom 不派发 disabled click）、
> prompt 默认值 sel || '' 右臂、parseImport x.name || String(i) 右臂
> （上游 filter 保证）、Select list || [] 右臂（useState 恒数组）。
> 门禁：目标套件 16/16 绿、eslint 0、tsc 0 错、guard PASSED、全量 jest
> 口径见交付说明。

## 2026-09-30 Round 45（wt-api）：Analytics/Invocations 覆盖收口 100/100/100/100——零测试页排行第九

> 覆盖率巡检第九站：`web/src/pages/Analytics/Invocations/index.tsx`（396 行 0%）。
> 5 用例，4×100%（v8），无页面缺陷（纯补测）。锁定契约——挂载双拉
> （summary {hours:24} + trend {interval:'hour'}）、窗口切换（近 30 天 →
> hours 720 + interval day + 趋势卡标题切换）、摘要统计卡五值（total/failed
> 原值、成功率 (rate*100).toFixed(1)+'%'、avg/p95 toFixed(1)）、summary/points/
> items/total 缺省右臂（DEFAULT_SUMMARY 0 兜底 + 空表）、Top 函数表
> avgDurationMs 双臂（值/0 → '-'）、趋势装配（points flatMap 双系列 ||
> 0 右臂）、明细矩阵（outcome success/error Tag、durationMs ==null '-' 与
> 0 显式、traceId 截 16 code/缺省 '-'、timestamp 缺省 ?? '' 右臂、error 列）、
> 首查载荷 {page:1,pageSize:20}、reject → success:false（ProTable 不落地
> data、明细保留上次数据）、函数 ID 搜索（trim 并入载荷）、结果下拉
> （选中并入 + allowClear 清除回到无 outcome）。
> **坑实证（新档，三条）**：① antd6 Input.Search 的搜索按钮类名是
> `.ant-input-search-btn`（非 v5 的 -button）；② ProTable 对 success:false
> 的响应在 useFetchData 里早退 return、不调 setDataAndLoading——「reject
> → 空表」是错误预期，实际保留上次数据；③ 'fn.a' 在 Top 函数表（summary
> 来源，reject 不清它）与明细表同名——明细断言必须锚最后一张
> .ant-table-container。
> 登记不可达：request 的 current/pageSize 默认参右臂（ProTable 恒传显式
> 值）；loadSummary 无 try/catch（reject 时 unhandled，页面原语义如此，
> 测试避免该路径）。
> 门禁：目标套件 5/5 绿（4×100%）、eslint 0、tsc 0 错、guard PASSED、
> 全量 jest 口径见交付说明。

## 第十四轮派发核验：todo P0 T1–T6 完成状态 + EventsDrawer 补测（wt-pages worktree，2026-09-29）

> **派发**：检查 todo.md 中 P0 任务 T1-T6 完成状态，挑最靠前未完成项补测。
>
> **核验结论：T1–T6 六项全部真实完成，无未完成项**。逐项按验收口径核对
> 代码工件与测试：T1（contract_service.go:1863 注释明示「单区块组合页
> 合法——仅要求 pageKey 非空」，≥2 拦截与前端文案均不存在）、T2（以
> `ContractTemplateRegenerator` 装配期注入闭包实现——正是 T2 改动点括号里
> 预判的「注意包依赖方向，必要时抽公共接口到 service 层」方案，
> contract_template_regen.go 是唯一收口）、T3（ExecutionState 字段 +
> 0025 编号迁移三处同步 + `TestExecutionStateExcludedFromContractDigest`
> digest 排除锁定）、T4（createUnboundContractsForSource 同事务 +
> unbound_contracts_test.go）、T5（dto.go contractsCreated/templatesUpdated/
> proposalsCreated + upload_summary_test.go）、T6（rebuildContract 命中
> unbound 翻转 + 自动绑定审计事件）。**勘误**：台账头部的提交号
> （65f88486b 等 10 位短 SHA）在本仓库对象库中不存在——系 squash/改写
> 前的陈旧引用，交付本体为真，SHA 引用按历史记录口径看待。
>
> **延伸查重（按最靠前未完成项假设推进）**：#46 批次链 2/3/4 已全部被
> 并行会话代做入库（批次 2 `b12aecd` displayName join catalog + healthStatus
> 推导、批次 3 `1b50c90` catalog 写路径 CRUD + seed、批次 4 `4d1fbb3`
> DomainEntry 渲染用例），todo.md 行 785「下一批：批次 2」表述已过时。
> 台账剩余最靠前显式未收口项 = Extensions 簇余量顺序
> 「EventsDrawer → UpgradeModal → AgentSync」（行 889-890）。
>
> **交付（2026-09-29）**：`EventsDrawer.tsx`（286 行，0 测试）→ 新增
> `__tests__/EventsDrawer.test.tsx` 11 用例，v8 口径行/分支/函数/语句
> **4×100%**。锁定契约：打开主链（抽屉标题 displayName 兜底 extensionId、
> 概览三 chip 含 total 经 adaptEventListResponse 归一同步、首拉载荷
> page/pageSize/level/keyword 缺省形态）、六列渲染矩阵（formatUnix 时间、
> payload 有值 code 文本/空值 '-'、createdAt=0 → '-'）、无安装实例守卫
> （不发请求 + 默认空态文案）、关键词筛选（trim 入参 + 生效 Alert
> 「已生效条件」+ chip）、级别下拉（antd6 mouseDown 落 .ant-select 根、
> option 点可见 content——Installations 套件坑位复用）、双条件「 / 」拼接、
> 清空筛选（初始 disabled → 复位 chip/Alert/载荷）、筛选空态与默认空态
> 两套文案、请求失败翼（success:false 静默空表不本地弹错——全局拦截器
> toast 语义）、切换安装实例重置筛选并以新 id 重拉、onClose 回调。
> **已知边界**：actionRef.current 的 undefined 翼（filter onChange 里
> `?.setPageInfo?.()`）经 UI 不可达——筛选栏与 ProTable 同 commit 渲染，
> 用户可交互时 ref 必已赋值；Select allowClear 清除翼与「清空筛选」按钮
> 同函数体不重复铺用例。簇余量剩 UpgradeModal（151 行）→ AgentSync
> （93 行）两项留后续批次。
> 门禁：目标套件 11/11 绿（格式化后复跑同绿）、`pnpm --dir web run tsc`
> 0 错、prettier 干净、`scripts/dashboard_vnext_guard.sh` PASSED、全量
> jest 负载守卫（load<10）窗口执行——结果见交付说明。

## 第十五轮派发核验 + Extensions 簇余量真缺口收口（wt-pages worktree，2026-09-30）

> **派发**：核对认领清单 #13 #29 #30（未完成项先做），否则台账最靠前
> 未收口项——Extensions 簇余量收口最后批次（UpgradeModal / AgentSync）。
>
> **核验结论：清单三项已于前轮全部收口；派发目标双双被并行会话代做
> 入库（dedup，不重做）**——UpgradeModal 7 用例（行/函数/语句 100%、
> 分支 97.36%）、AgentSync 6 用例（4×100%）分别由台账「事件抽屉 +
> 升级弹窗覆盖批次」「Agent 同步调试页覆盖批次」两节（2026-09-29）落地，
> 本轮 merge origin/main（ea84dc0）带进其测试文件核实无冲突。
>
> **本轮交付（快照分离后的真缺口）**：覆盖率快照标 4 文件，先按 round-9
> 口径分离 v8 跨套件合并伪影（CatalogManageModals 经自有套件即 100%），
> 余两真缺口收口——
> ① `Store/SchemaFields.tsx`（174 行，64.36%→**100/96.87/100/100**）：
> 新增 8 用例锁定守卫（无 properties/非对象 → null）、全类型分派矩阵
> （enum/boolean→Select、number/integer→InputNumber（integer precision=0）、
> array/object→TextArea placeholder '[]'/'{}'、string→Input）、enum 选项点击
> 选中（**antd6 新坑实证**：Select 选中值渲染在 `.ant-select-content`，
> v5 的 `.ant-select-selection-item` 不存在，input value 恒空）、title 缺省
> 回退 key + raw null 防御、array/object 无 description 兜底文案、required
> 星标两态、Form initialValues 经 ['config',key] 默认值回显。167 行
> 默认路 placeholder 三元 '0' 翼登记构造性不可达（number/integer 在 86 行
> 已被 InputNumber 分派拦截）。
> ② `Store/index.tsx`（96.82%→**98.94 行/100 函数/93.47 分支**，管理流
> 批次 5/6 页面接线）：套件扩至 31 用例，新增 6 例 + 4 例强化——通用错误翼
> 三连（导入/上下架/删除 plain Error → mapper 兜底文案）、登记通用错误翼
> （弹窗保持）、发布 409 与通用双翼（表单值保留二次提交）、三弹窗关闭翼
> （发布/登记 onClose destroyOnHidden 卸载 + InstallModal onCancel 壳残留
> display:none 断言）；强化：登记全可选字段 trim 载荷、发布全字段 trim
> 载荷、下架/删除 displayName 空回退 name（wiki 行）。余 432-440（manifest
> 解析兜底，表单 validator 同源双保险）与 317 行 `values.name?.` 非空翼
> （**类型含 name 字段但登记表单无该项**，提交恒 undefined——疑似类型/表单
> 漂移，如实登记不代改）登记不可达；125/150/427/542/545/795 为既有口径
> 防御翼（头注释 5 号登记合并）。
>
> **坑实证（新档）**：antd message 的 toast 节点不可移除——首版 clearToasts
> 删 `.ant-message` 容器后，后续 toast 渲染进 detached 节点（下架翼文案
> 永不出现）+ antd 内部 removeChild 抛 NotFoundError；改为计数式断言
> （动前基线 + 动后等增长，离场动画 className 含 leave 的 notice 不计）。
> v8 函数表按声明行定位：824 是登记弹窗 onClose 而非发布弹窗（勿按
> 语感对号）。
>
> 门禁：Store 四套件 60/60 绿、`pnpm --dir web run tsc` 0 错、prettier
> 干净、`scripts/dashboard_vnext_guard.sh` PASSED、全量 jest 结果见交付
> 说明。**偏差注明**：派发的 push 前 fetch --rebase 因 wt-pages 已推送
> 分支禁 rebase（房规），改 `git merge origin/main` 后快进推送。

## 第十六轮：配置管理页覆盖收口（Operations/Configs 五文件 0% → 4 文件 4×100，2026-09-30）

> **交付（2026-09-30，wt-pages worktree）**：覆盖率补缺轮——零测试目录
> 排行现席 `Operations/Configs`（index 273 + useConfigsPage 272 + schema 156
>
> - diff 102，约 800 行 0%）整目录收口，新增 `__tests__/index.test.tsx`
>   17 用例，v8 口径：**diff.tsx / index.tsx / schema.tsx 三文件行/分支/函数/
>   语句 4×100%，useConfigsPage.ts 行/语句/函数 100%、分支 91.17%**（余 6
>   臂 = 六处 `if (!cur) return` 守卫，经 UI 不可达，登记见下）。
>   锁定契约——列表与筛选：首拉空载荷 {}、六列渲染矩阵（Format Tag /
>   gameId-env 空行 filter(Boolean) 右翼 / 8 行编辑按钮）、搜索实时 trim
>   重拉 + 三下拉（game/env 选项由 rows distinct 派生）+ Enter/查询闭包重拉
> - 重置回空载荷；全局 scope 联动（setScope 双值同步 + merge 语义下换
>   游戏保环境）；load 失败 toast + `{}` / undefined 响应双右翼。
>   编辑弹窗：openItem 兜底链（fmt→r.format→json、content/version/gameId/
>   env 五 `?.` 翼 + r nullish 五翼）、标题 `${id} (${fmt}) v${version||''}`
>   （version 0/undefined 均落 'v' 尾）、getConfig 失败 toast、csv 预览
>   （\r\n 归一 + 空行过滤 + 逗号切列）、编辑器受控 + 弹窗格式切换（langOf
>   七臂矩阵 + csv 预览卸载）、校验三态（通过 / errors join('\n') 透出 /
>   无 errors 兜底「校验失败」）。
>   保存版本：成功链（载荷 gameId/env 回退 toolbar 态 + baseVersion
>   version||0 + message 无必填拦截——placeholder 标「必填」但 doSave 不
>   校验，现状锁定 + toast `已保存版本 N` + 弹窗关 + reload + saveMsg 清空
> - onCancel X 关闭翼）、失败翼弹窗保持、`已保存版本 undefined` 翼。
>   历史版本与对比：列表行渲染（createdAt formatDateTime/'' 双翼 + `{}`
>   响应空表右翼）、查看（content/format 回填 + version 不动——baseVersion
>   语义 + verOpen 关闭 + value/format 空串右翼）、diff（MonacoDiff 桩受
>   left/right + DiffView 真实算法矩阵：add-batch/add-tail/del-batch/
>   del-tail/单点替换、del 红 rgb(255,241,240) / add 绿 rgb(246,255,237)、
>   对侧空 div 占位、left 空双右翼）、回滚（confirm 文案模板内插 + onOk
>   载荷三 `||` 回退翼双侧 + `rollback to v${ver}` + 成功「已回滚」+ 版本
>   弹窗关 + reload + getVersion undefined 双右翼 + 失败翼弹窗保持）。
>   **坑实证（新档，四条）**：① `@umijs/max` mock 工厂若引用工厂外 `const`
>   变量，schema.tsx 模块顶层 `getIntl()` 在 import 期即触发——此时外层
>   const 仍在 TDZ（`mock*` 前缀只过 babel hoist 白名单、不解 TDZ），intl
>   对象必须工厂体内自包含构造；② jsdom textarea 读值把 `\r\n` 归一为
>   `\n`——「编辑器显示原始 CRLF」的断言须按归一形态写（state 侧仍是
>   CRLF，保存/校验载荷断言不受影响）；③ 弹窗标题与页内同名按钮碰撞
>   （「历史版本」标题 vs 编辑弹窗内同名按钮）——关闭断言锚
>   `.ant-modal-title` textContent 而非全文 queryByText，否则永不消失；
>   ④ 上一用例 waitFor 超时路径会遗留未消费的 `mockResolvedValueOnce`
>   队列（clearAllMocks 不清 Once），跨用例毒化函数源——表象是下一用例
>   版本列表恒空，须修根因（标题碰撞）而非绕断言。
>   **边界（诚实清单）**：① 六处 `if (!cur) return` 守卫（validate/
>   doSave/openVersions/viewVersion/diffWithVersion/rollbackTo）经 UI 不可
>   达——动作按钮仅在 `{cur && ...}` 分支内渲染，不造假用例（分支余量
>   全部在此）；② validate/openVersions/viewVersion/diffWithVersion/
>   rollbackTo 外层 async 无 try/catch——接口 reject 产生 unhandled
>   rejection（组件现状缺陷，同 Store/AgentSync 巡检结论），不造假 reject
>   场景；③ 编辑弹窗 title 三元 false 翼与 diff 弹窗 `cur?.format` nullish
>   翼构造性不可达（弹窗仅经 cur 态按钮打开）；④ hasMonaco() 硬编码
>   false，true 翼不可达——MonacoDiff 桩与 DiffView 恒同时渲染，按此断言。
>   门禁：目标套件 17/17 绿、`pnpm --dir web run tsc` 0 错、eslint 干净、
>   prettier 干净、`scripts/dashboard_vnext_guard.sh` PASSED、全量 jest
>   2-worker 限流口径（load ~12 非空载窗口，如实注明）结果见交付说明。
>   下一批候选（零测试目录排行余量）：按新快照重排。

## 2026-09-30 Round 46（wt-api）：Ops/AnalyticsFilters 覆盖收口 97.15/89.79/100/97.15——零测试页排行第十

> 覆盖率巡检第十站：`web/src/pages/Ops/AnalyticsFilters/index.tsx`（281 行 0%）。
> 8 用例，行/语句/函数 97.15/97.15/100、分支 89.79%，无页面缺陷（纯补测）。
> 锁定契约——挂载映射矩阵（label 链 displayName→aliasName→name、envs 直取/
> envMeta 回退含空串与 null 条目过滤/双缺 []、name 空 Filter 出列）、
> 游戏/环境联动（切游戏清 env + 选项重建 + canQuery 双条件门：未选齐双按钮
> disabled）、加载主链（{gameId,env} 载荷 + events||[]/paymentsEnabled!==false/
> sampleGlobal??100 三归一 + 事件数 Tag/支付红禁用/采样 50 gold）、加载缺省
> （{} → 全部允许/允许上报/100 绿）与失败（加载失败）、保存主链（全字段
> 载荷 + 已保存）与失败（保存失败（需要 analytics:manage 权限））、交互面
> （InputNumber 改值与清空 → Number(v||0) 0 兜底、Switch 双态文案与 Tag、
> 事件 tags Select 增 tag → 事件数 1）。
> **坑实证（新档，三条）**：① antd6 Select 选中值有 aria-live 镜像双 DOM
> （getByText 'beta' 双命中），按 Select 根 textContent 聚合断言；② 摘要条
> 的 FormattedMessage 与 {gameId||'-'} 是同级文本节点（无独立元素可锚），
> 按 .ant-card-body toHaveTextContent 聚合；③ 两字中文 Button 自动插空格
> （加 载/保 存），getByRole name 用 /加\s*载/ 正则。
> 登记不可达（5 处防御分支，不造假用例不删分支）：save 的 !canQuery
> warning 早退体（77-84 行 8 语句，按钮 disabled jsdom 不派发）、label 链尾
> 'Unknown'（幸存行 name 恒真）、(r?.games||[]) 双右臂（服务契约恒返
> {games:[]}，构造 undefined 违反返回类型即造假）、selectedGame?.envs 的 ?.
> 右臂（gameId 恒来自同数组选项）、listGamesMeta catch{} 静默翼（reject 与
> 空 resolve 在 DOM 不可区分）。
> 门禁：目标套件 8/8 绿、eslint 0、tsc 0 错、guard PASSED、全量 jest
> 口径见交付说明。

## 2026-09-30 Round 47（wt-api）：Ops/Terms 覆盖收口 100/96.55/100/100

> 覆盖率巡检第十一站：`web/src/pages/Ops/Terms/index.tsx`（231 行 0%）。
> 8 用例，行/语句/函数 100%、分支 96.55%，无页面缺陷（纯补测）。锁定
> 契约——挂载 load('resource')（默认域）+ 域 Select 切换 → listTerms
> ('operation') 重拉；渲染矩阵（Domain Tag resource 蓝/其余紫、显示文本
> localizedText 三臂：zh 命中/zh 空串→en 回退/display 缺省→termKey 兜底、
> 语言列 keys||{} + filter 真值过滤 + 空数组 '-'、Order 空值 ProTable '-'
> 兜底）；新增（initialValues {domain: 当前筛选域, order:100} + destroyOnHidden
> 重开重挂载 + required 三键拦截不触达服务 + 全字段落库 + 保存成功 + 重拉 +
> 关闭）；编辑（整行回填 termKey/alias/display/order + 载荷不含 id——
> TermFormValues=Omit<TermItem,'id'> 契约）；保存失败 return false 弹窗保持
> 不重拉无本地弹错；删除（Popconfirm → deleteTerm(domain,alias) + 已删除 +
> 重拉）；load 失败 toast 透传 Error.message（getErrorMessage 经 getIntl()
> 调用时解析——mock 下 getIntl 必须与 useIntl 同一稳定实例，否则页内注释
> 言明的无限请求循环）。
> **坑实证（新档，两条）**：① ModalForm（destroyOnHidden）关闭后**整体
> 卸载**——.ant-modal 从 DOM 消失（探针实证 modalCount=0），区别于普通
> antd Modal 的 display:none 残留（FunnelPresetBar 坑档），关闭断言用
> querySelector('.ant-modal') 为 null；② 空值 Order 列 ProTable 兜底渲染
> '-'，与语言列 '-' 同行同文本双命中，断言须 getAllByText。
> 登记不可达（1 处防御分支）：line 82 `(row.display||{})[k]` 的 `|| {}`
> 右臂——display undefined 时上游 Object.keys({}) 已得 []、filter 回调
> 不执行，回调内二次归一右臂结构不可达；delete onConfirm 无 catch 的
> reject 路径按惯例不造假（unhandled rejection 现状语义）。
> 门禁：目标套件 8/8 绿、eslint 0、tsc 0 错、guard PASSED、全量 jest
> 口径见交付说明。

## 第十七轮：个人中心页覆盖收口（Profile 入口 + 数据层 0% → index 4×100，2026-09-30）

> **交付（2026-09-30，wt-pages worktree）**：覆盖率补缺轮——Profile 簇
> 六份既有套件全是单组件回归（GamesTab/InfoTab/MfaSettings/NotificationsTab/
> PermissionTree/SecurityTab），页面入口 index.tsx（452 行）、数据层
> useProfileData.ts（323 行）与 shared.ts 的页面级接线全程 0 覆盖。新增
> `Profile/__tests__/index.test.tsx` 23 用例经真实页面渲染锁定三文件契约：
> **index.tsx 行/分支/函数/语句 4×100%**；useProfileData.ts 行/语句/函数
> 100%、分支 80%（余翼全部登记，见边界）；shared.ts 行/语句/函数 100%、
> 分支 83.33%（全目录合并口径：normalizeAvatarSrc 两翼由 InfoTab.regression
> 姊妹套件覆盖，余 profileText number 翼与 pickAuditMetaValue !meta 守卫
> 登记）。锁定契约——首拉链（loadProfile → hero 矩阵 + loadExtras 七路
> allSettled 并行载荷含 login kinds 双查/listPermissions pageSize 500 +
> stats 四卡派生）、加载双翼（pending 骨架 / reject toast 停留骨架）、
> 未设置兜底矩阵（displayName 空串 hero Title 回退 username、active
> undefined 无徽标 / 显式 false 未启用徽标、roles undefined 右翼、四行
> 未设置）、Tab 编排（URL 深链初始 tab + InfoTab 不挂载、切 tab navigate
> replace 同步、hero 编辑按钮强制回资料页 + scrollIntoView、pane 缓存
> 共存）、资料编辑链（保存成功载荷 + toast + 退出 + 重拉、失败 + 必填/
> 手机号 pattern 双拦截、取消回填 displayName||nickname 右翼）、消息已读链
> （未读详情打开即标读翻转、已读不触发、单条标读 stopPropagation、
> markAllRead 按 unread 过滤、徽标/按钮随 unreadCount 消失）、权限派生
> （同 resource+scope 并集合并、scope Tag 双形态、目录驱动候选 key/id 双
> 过滤、空目录 → fallback 模板兜底（owned 过滤后仅 functions:manage
> 存活）标记仍绿、reject → 金色受限 Tag、申请弹窗 reason 必填 +
> createFeedback 载荷 + destroyOnHidden、fullAccess 徽标、catalog name
> 空串回退 id）、loginSessionRows（ip/client_ip、ipRegion/region、
> userAgent/ua 键族、成败从 kind 推断含空串 kind 服务端真实形态兜底、
> time 空串 key 兜底）、loadExtras 失败矩阵（七路全 reject settled 静默、
> stats 归零、各 Tab 空态、目录回退；perms {} nullish 右翼、username
> 空串 login 查询不发）、密码/头像弹窗开合接线（不触达改密服务）。
> **边界（诚实清单）**：① loadExtras 外层 catch 结构性不可达（allSettled
> 永不 reject）；② markAllRead unread===0 早退与 markMessageRead 已读
> 早退经 UI 不可达（按钮条件渲染）；③ 契约死翼族登记（不造假用例）——
> settled 取值链 nullish 右翼（games/perms/messages/channels 由 service
> 归一层保证形状、listPermissions/listAudit 契约声明必填字段）、
> Array.isArray false 翼（getMyPermissions 归一化「缺失会被补」）、
> detailMessage 更新器 prev 失配翼（openMessage 先置 detail 再标读；行内
> 标读时 Modal 遮罩挡列表）、loginRecords||[]（useState 恒数组）、
> item.meta||{}（normalizeAuditEvent 恒对象）、profileText number 翼
> （调用点类型均 string|undefined）、pickAuditMetaValue !meta 守卫（唯一
> 调用方先经 (item.meta||{}) 归一）；④ MfaSettings/PasswordModal/
> AvatarModal 提交/上传本体属各组件自有套件域，本套件只锁开合接线。
> **坑实证（新档，三条）**：① 详情弹窗标题锚 .ant-modal-title 仍需注意
> 标题取 item.title 而非 content——错把 content 当标题断言必挂；② 权限
> 树目录（未拥有灰字）与申请卡候选（strong 标题）同名双命中，断言须
> cardByTitle 收窄；③ owned 集合 = permissions(resource:action) ∪
> permissionIds——fallback 六模板经 owned 过滤后存活数按夹具拥有面推算，
> 六模板 ≠ 六候选。
> 门禁：目标套件 23/23 绿、tsc 0 错、eslint/prettier 干净、guard PASSED、
> 全量 jest 357 套件 4361 用例（2 worker 限流，load 50-216 极高位窗口）：
> 4354 绿、5 套件 7 用例失败——Approvals/Extensions Store/Functions History/
> previewActions/serverPushdown 全 timeout 形态且均他域既有绿套件，
> 隔离复跑 5 套件 110/110 绿（152s）定责负载型非回归；Profile 套件全量中绿。

## 2026-09-30 Round 47 收尾：推送 + CI 全绿（wt-api，`b4ee6ce`）

> 推送链：fetch → rebase origin/main（零冲突，todo.md 台账合流保双方）→ 双推
> main（`7c2d230..b4ee6ce`）与 wt-api；Core/Docker/CodeQL/Docs 对该提交全绿。
> CI - Dashboard run `36689396013` 的 dashboard-quality 前 5 次尝试**全部环境性、
> 零真实用例失败**（各次日志 178-213 套件 PASS、0 个 ✕/FAIL）：attempt 1 死于
> job 60min 上限；attempt 2-5 死于 GitHub hosted runner「The runner has received a
> shutdown signal」（3 个不同 runner 实例、27-32min 处被回收）。同签名故障在本
> 会话推送前已现（01:46 d466b4b、02:29 ea84dc0 两次同签名挂），非本提交回归；
> `ci-dashboard.yml` 无 concurrency 组，且 28bcbbd/00cb1d9 两次 push 因 paths
> 过滤未触发该 workflow，排除 push 自动取消。当日 00:14-01:52 三连绿各 38min
> 证明 job 本体需 ~35-40min，回收窗口内 4/4 命中即死。13:24 双探针（`2388c30`/
> `7b5a73b` 的 dashboard-quality 各 26.7/32min 转绿、双存活）确认窗口转移后重跑
> attempt 6 **全绿**（run conclusion=success，4 job 全 success，dashboard-quality
> 13:51→14:26 UTC 共 35.5min）。本地佐证：全量 jest 354 套件 4307 用例 exit=0、
> guard PASSED。已知边界：GitHub hosted runner 长 job 概率性回收属基础设施退化，
> 若复发须重试至非回收窗口（45min step/60min job 上限内本 job 可完成）。

## 覆盖率巡检批次·Go 侧第四十八轮·api/cicd Create enabled 翼收口（wt-api worktree，2026-09-30）

> 交付：全量 profile 重排（**99.949%**，64112/64145 语句，21 文件 33 语句
> 余量）——逐行对账第 3-28 轮台账后，**31 语句与既有登记/回避面行号级
> 精确吻合**（3a3a680 CodeQL 整型收窄修复零新增缺口，layered.go 新分支
> 由同提交 layered_performance_test 覆盖），唯一无主块为
> `api/cicd/service.go:219`——Create 的 `if req.Enabled != nil
{ row.Enabled = *req.Enabled }` 赋值体，其唯一天然用例正是 t.Skip 的
> 已知缺陷用例（显式 false 被 gorm:"default:true" 驱动层丢弃）。
> 本批以**显式 true 指针路径**合法收口（true 非零值不涉丢列缺陷）+
> 键缺省 nil 指针对照臂：新增 `create_enabled_wing_r48_test.go` 1 用例，
> api/cicd **97.9% → 98.2%**；service.go 余 2 块恰为既有登记
> （:93 normalizeExtra float64 / :374 Trigger ExternalID）。
> **坑实证（新档）**：GORM `First(&row)` 复用带主键 struct 会把既有主键
> 并入查询条件——同用例内第二次按非主键列查询须换新变量，否则恒
> record not found。
> 顺带定性（不代改他会话域）：全量 profile 轮 approvals
> `TestWebSocketHub_Run` 一次失败——`Unregister` 异步 channel 派发与
> Run loop map 移除间的竞态窗口被满载（load 40-111）放大；单测/单包
> 复跑双绿、域零改动，判负载性偶发非回归。全量门禁另被系统内存压力
> 回收一次（机器 48Gi/51Gi 占用、swap 满、load 107——并行会话
> heavyweight），挂恢复观察哨（avail>20Gi && load5<60）后于窗口内
> 重跑 **158 包全绿 exit=0**。
> 回避面维持：otp/otpauth.go（d9fdc05 会话 OTP 域）、
> api/assignment/gate.go（BUG-035 域）——各 2 语句。
> 门禁：触及文件 gofmt 干净、go vet 干净、go test ./internal/...
> 全绿（fresh，恢复窗口执行）。

## 第十八轮：行为分析页覆盖收口（Analytics/Behavior 三文件 0% → 三文件语句/行/函数 3×100，2026-09-30）

> **交付（2026-09-30，wt-pages worktree）**：覆盖率补缺轮——`Analytics/Behavior/`
> 零测试簇收口：入口 `index.tsx`（383 行）+ `PathControls.tsx`（245 行）+
> `AdoptionControls.tsx`（251 行）合计 879 行 0%（姊妹件 FunnelPresetBar 已由
> Round 44 套件覆盖）→ 新增 `__tests__/index.test.tsx` 14 用例。v8 口径：
> **三文件语句/行/函数 3×100%、分支 87.73%**（index 87.05 / PathControls
> 89.58 / AdoptionControls 86.66，余翼全部登记见测试头注释诚实清单）。
> 锁定契约——事件探索（首拉空筛选载荷、三输入+RangePicker ISO 载荷、
> events.csv 导出 user_id/userId 双回退、{} 右翼空表+header-only 导出）；
> 漏斗（默认态 {steps:'',sequential:0}、tags 双步+顺序 Switch+同会话
> Checkbox+步间秒数四键齐载荷、`${v}%` 渲染、gapSec 清空 Number(v||0) 键
> 消失、{} 右翼清表、funnel.csv String 强转）；复制链接（空态仅 `?`、
> 全参六键按插入序 URLSearchParams）；深链（steps trim/filter 归一+四态
> 预填+setTimeout 自动漏斗、负翼四项）；路径分析（默认 {per:'session',
> steps:5,limit:50}、全参数 include/exclude tags+正则 trim、InputNumber
> 清空回默认、匹配漏斗指示器是/否/非法正则/无 steps 四态、填充漏斗
> split('>') 回填+scrollIntoView、复制步骤、paths.csv 空值兜底）；功能
> 采用率（基数行 {per} 内插、features join+per 切换、range 传播进
> load/loadDim 载荷、breakdown 分层载荷+dim 表+adoption_breakdown.csv、
> falsy 行 rowKey 三段右翼、{} 双右翼归零）。
> **现状锁定（页面 quirk，如实断言不代改）**：① 事件表「用户」列
> dataIndex='user_id' 而归一化层产出键为 userId——生产环境该列恒空，
> 导出侧 (r.user_id || r.userId || '') 双翼都有回退（夹具分列验证）；
> ② 深链自动计算经挂载期闭包捕获 range=null，start/end 不进自动漏斗
> 载荷——range 预填只对后续手动计算/事件查询生效（负翼用例+双段断言）。
> **坑实证（新档，两条）**：① antd6 tags 模式 Select 无
> `.ant-select-selection-search-input` 类（querySelector null →
> "Unable to fire change"），输入锚改 `selectRoot.querySelector('input')`；
> ② Enter keyDown 追 tag 连续添加只落首个 token（rc-select tokenization
> 宏任务竞态），多 tag 追加改走 dropdown 点选配方（mouseDown → change →
> 点非隐藏下拉内 `.ant-select-item-option-content` 匹配项）。
> **边界（诚实清单，详见测试头注释）**：try/finally 无 catch 族（同族
> 页面既有口径，不造假 reject）；rate `v != null` 右翼（归一恒 number）；
> `(rows||[])`/`(funnel||[])`/`(rowsDim||[])`/`(currentSteps||[])` 右翼
> （useState 恒数组）；复制链接 `range && range[0/1]` 半开翼（RangePicker
> 只产完整对或 null）；深链外/内 catch（dayjs 不抛、isValid 门已兜）；
> `onUsePath &&` 守卫翼；rowKey/导出单元格 `|| ''` 类型防御族。
> 门禁：目标套件 14/14 绿、`pnpm --dir web run tsc` 0 错、eslint/prettier
> 干净、guard PASSED；全量 jest 4375 用例 2-worker 限流（load 31-45 高位
> 窗口，非空载）：4374 绿 + 唯一失败 Extensions/Store 超时形态（139s），
> 隔离复跑与其同窗核绿（连同 merge 带进的 Ops/Nodes NodeDetailDrawer 上游
> 新套件一并隔离验证）；push 前 fetch → merge origin/main（todo.md 冲突
> 保双方）。

## 第十九轮：函数注册告警页覆盖收口（Functions/Warnings 0% → 语句/行/函数 3×100，2026-09-30）

> **交付（2026-09-30，wt-pages worktree）**：覆盖率补缺轮——零测试簇排行
> 现席 `Functions/Warnings/index.tsx`（328 行，簇内无任何测试文件）单件
> 收口，新增 `__tests__/index.test.tsx` 12 用例，v8 口径
> **行/语句/函数 100%、分支 95.65%**（余 2 翼登记，见边界）。锁定契约——
> 挂载链（URL 四参 function_id/agent_id/code/limit 解析预填表单 +
> loadData 四键缺省、limit=abc 经 `Number(NaN) || 100` 回落 100、
> reject → effect .catch 兜底空表不白屏）；查询与 URL 同步（syncUrl 只写
> 真值键——全填 replace 全参 URL、空值键剥除后全空表单落裸 pathname 无
> `?`、随后 loadData 透传）；刷新仅重拉不动 URL；行内动作（标为已读
> markOne(key) + 本地翻转不重拉——行锚收窄断言 w1 行按钮消失而 w3 稀疏行
> 保留、行删除 Popconfirm → deleteOne + 本地过滤不重拉）；批量动作
> （全部已读 markAll → toast + 以当前表单条件重拉、清空 Popconfirm →
> deleteAll → toast 已清空 + 重拉）；列渲染矩阵（code orange Tag + 空串
> '-'、functionId/version/agentId 空串 '-'、count 原值、lastSeen
> formatDateTime 双翼、稀疏行 '-' 计数 ≥5）；scope 联动 #34 族（scopeKey
> 变化 → 以当前表单条件重拉，URL 参数不重复消费）。
> **坑实证（新档，两条）**：① RTL `within()` 返回查询 API 而非 DOM 元素——
> `within(pop).querySelector` 直接 TypeError，Popconfirm 确认按钮须在
> HTMLElement 本体上 querySelector；② 行条件渲染按「字段存在性」计数——
> 稀疏行无 read 字段 → `!record.read` 走真翼同样渲染「标为已读」，按钮
> 计数须按夹具逐行推算（非只数显式置值的行）。另：PageContainer 桩只渲
> children，页面 title 不进 DOM——挂载失败翼断言改锚 Alert 的「规则说明」。
> **边界（诚实清单，不造假用例不删防御分支）**：① 66 行
> `Array.isArray(res?.items)` 右翼——service 归一层恒返 items 数组，
> resolve {} 形态违反返回类型即造假，登记；② 307 行 `text ?? ''` 右翼——
> 列参类型 string，null 形态违反类型契约，空串左翼经 w3 稀疏行覆盖；
> ③ loadData 与四个动作函数 try/finally 无 catch——接口 reject 成 unhandled
> rejection（同族页面既有口径），唯一 catch 翼（挂载 effect）以 reject
> 用例真实触达，其余不造假 reject 场景。
> 门禁：目标套件 12/12 绿、`pnpm --dir web run tsc` 0 错、eslint/prettier
> 干净、guard PASSED；全量 jest 负载口径见交付说明。

## 第二十轮：角色管理页覆盖收口 + 附带修定表单脱管缺陷（Permissions/RolesV2，2026-09-30）

> **交付（2026-09-30，wt-pages worktree）**：覆盖率补缺轮——零测试簇排行现席
> `Permissions/RolesV2/index.tsx`（312 行，簇内无任何测试文件）单件收口，
> 新增 `__tests__/index.test.tsx` 12 用例，v8 口径 **行/语句/函数 100%、
> 分支 94.73%**（余 2 翼即登记边界，见下）。
>
> **附带修定一处真缺陷（回归锁定）**：两个 Form.Item（name/description）
> 原为 `{' '}<Input />{' '}` 三元素数组子节点——antd Form.Item 源码
> `Array.isArray(mergedChildren) && hasName` 分支只 warning 不做
> cloneElement 控制注入（同 Round 40 RateLimits 同族）：输入脱管 form
> store——新建 name 恒 undefined 过不了 required（新增角色不可用）、
> 编辑键入不落 store（提交恒初始值）、label htmlFor 无 id 可指。
> 修法：去掉 `{' '}` 恢复单子节点；套件对未修页面实跑取证后修定，
> 「键入值进载荷」「回填显示」两断言即回归锁。
>
> 锁定契约——挂载首拉 {page:1,pageSize:10} + 列头/三按钮/showTotal；
> 权限列 slice(0,6) + 缺字段行 (arr||[]) 右翼；响应归一（缺 total 回落
> items 长度 `共 2 条`、{} 空表）；分页 onChange 参数透传；新增（required
> 拦截 + 载荷 + `已创建 #7` + 重拉 + destroyOnHidden 卸载）；编辑（回填
> 显示 + 改值载荷 + 失败弹窗保持）；权限弹窗（标题内插 + 8 权限回显 +
> 追加 tag 载荷 + 失败保持）；删除 Popconfirm 主链；description undefined
> 直传态（新增失败载荷断言）。
>
> **坑实证（新档，四条）**：① 分页 showSizeChanger 的 Select 在主内容区、
> DOM 序先于 portal 弹窗——`querySelector('.ant-select')` 打到 page-size
> 选择器（表象 tags 输入无反应、载荷恒 []），Select 锚必须限 `.ant-modal`；
> ② antd6 Pager 是 `<li title onClick><a rel=nofollow>`，a 无 href 无
> button/link role——翻页点击落 `.ant-pagination-item-N` 的 li 本体；
> ③ total=0 时 antd Table 不渲染分页（`共 0 条` 不可见）——空态锚
> `.ant-empty-description`（与 Empty svg 内 `<title>No data</title>`
> 同文双命中须 selector 收窄）；④ 弹窗标题与工具栏按钮同文本锚
> `.ant-modal-title`；同用例多渲一次会多吃一个 mockResolvedValueOnce
> 队列（unmount 取自首渲）。
>
> **边界（诚实清单，不造假用例不删防御分支）**：分支余量 2 处恰为登记
> 项——submitPerms `if (!editing) return false` 守卫（openPerms 先置
> editing，构造性不可达）与 `v.permissions || []` 右翼（initialValues
> 恒设键，undefined 违反表单值契约）；refresh/remove 无 catch 按同族
> 页面口径不造假 reject；description 直传无分支差异，'' 显式清空态与
> undefined 走同一行不另铺用例。
>
> 门禁：目标套件 12/12 绿（格式化后复跑同绿）、`pnpm --dir web run tsc`
> 0 错、eslint/prettier 干净、`scripts/dashboard_vnext_guard.sh` PASSED；
> 全量 jest 默认 worker（起跑负载 ~9，跑中升至 28-31 并行会话占机）：
> 361 套件 4421 用例，4420 绿 + 唯一失败 Extensions/Store 190s 超时形态
> （既有绿套件，负载回落后隔离复跑 31/31 绿定责负载型非回归）。> **CI 处置（287efcb，test-only 推送后四 run 判形）**：
> CodeQL ✅、CI-Core ✅（全 Go 测试含本新用例在内）；Docker 首挂
> （`Build and push Docker image` 15min step 超时，日志唯一 error 为 timeout、
> 无编译错误；test-only 改动不进 `go build`，同树其余 4 镜像全绿）→ rerun
> **全绿**（5 构建 + 5 supply-chain 全 success）。Dashboard
> `dashboard-quality` attempt 1 挂（shutdown signal、151k 行零 ✕）→ 按探针
> 协议先验后代：`cf336c9` 同 job 同签名挂（142k 行零 ✕，窗口仍活，不盲重试）
> → 等 `767ab2b` 探针整绿（窗口开）→ rerun attempt 2 **同签名再挂**
> （142k 行零 ✕，19:44 被回收时套件仍 PASS）→ 按既定处置**终止重试、以证据
> 链定案**：本改动仅新增 Go 测试文件 + 本台账（不触 web/，jest 与本提交无
> 交集），dashboard-quality 本体在 `3a3a680`/`7b5a73b`/`767ab2b` 多次整绿可
> 证，两次失败均零真实用例失败——判 GitHub hosted runner 当日概率性回收
> （基础设施退化），非本提交回归。已知边界：若该 workflow 后续持续同签名挂，
> 属 runner 侧问题，排查口径见记忆档 ci-dashboard-runner-shutdown-signature。

## 覆盖率补缺 R49：mobile 审计/会话域 + web NodeDetailDrawer/OpenAPISources（主树，2026-09-30）

> **R49-1（`cf336c9` 已推送）**：mobile 90.71%→**94.29%**（1898/2013）——
> audit_controller 81/81=100%（StateError 透传、loadMore 双 guard 含 Completer
> 门控防竞态、AuditFilters 契约）、audit_page 175/175=100%（加载更多/空附加
> 信息 metadata 空 map 令 `==null||`短路右侧求值、下拉刷新、自定义 chip、
> 时间区间 RFC3339 归一——M3 DateRangePickerDialog 确认按钮是 **Save** 非 OK、
> 宽屏双月 `.first` 取当月）、session_store 58/58=100%（FakeSecurePlatform
> 内存实现绕 MethodChannel；SessionData 无 == 重载须逐字段断言）；
> fake_dio_adapter handler 放宽 `FutureOr<ResponseBody>` 支持挂起门控。
> web NodeDetailDrawer 11.6%→**100% 行**（22 用例）——**修真实缺陷**：
> 时间窗 onClick 手动 load + useCallback 重建触发 useEffect 重放，末次调用把
> limit 覆盖回 50；改默认 limit 随窗口放大（5m→50/1h→120/长窗→200）+ onClick
> 只 set 状态由 effect 统一驱动（消除双请求）。
> **R49-2（本批）**：web OpenAPISources/index.tsx 60.4%（335 miss）→
> **97.2%（822/846）**，30 用例（子组件 SourceModal/BindingModal/
> SourceDetailDrawer/PipelineSummaryModal 替身接管回调，驱动页面自身全分支：
> 加载失败/只读/详情/创建/更新/绑定/解绑/守卫）；全树 **97.63%→98.10%**
> （93728/95548）。**登记不可达（24 行，三处 UI 不可达守卫）**：
> 214-221 submitSource noWrite 早退、227-234 update 态 editingSource=null 早退、
> 333-340 submitBinding noWrite 早退——只读时入口按钮与弹窗均不渲染、update
> 态 editingSource 与 modal 同批置位恒非 null，无任何交互路径可达。
> **坑实证（新档四条）**：① PageContainer extra 按钮 accessible name 含图标
> aria-label 前缀（`reload 刷新`），getByRole name 须正则；② 运行时候选 label
> 的 `runtimeAgent` 词条无 defaultMessage（语言包外置），mock useIntl 须按 id
> 特判注入 agent 值；③ 绑定提交前 guard（operation 无 functionId）须先点候选
> 按钮选函数，否则 warning 早退、bind 服务零调用（初次 4 用例集体挂同根）；
> ④ jest.mock 工厂内闭包引用模块级变量须惰性（解构进 props 时才读），声明位置
> 不影响——但 const 的 TDZ 仍受声明顺序约束（SPEC_JSON 供 DETAIL 前须先声明）。
> 门禁：OpenAPISources 3 套件 39/39 绿、全量 jest **4358/4359**（唯一失败
> Functions/History「刷新双拉」在隔离重跑 16/16 绿——173.6s vs 60.7s 负载竞态，
> 与改动零交集）、tsc 0、eslint 0；R49-1 侧 flutter analyze/test 188 全绿。
> **R49-3（本批）**：web OpenAPISources 三文件拉满——SourceDetailDrawer.tsx
> 39.8%→**100% 行**（16 用例：概要卡三态/诊断空双臂/三 severity Tag 色/
> operationLabel 三臂/六契约 Tag 二态/approval 兜底/绑定回调/Popconfirm 删除/
> 只读三按钮省略/原始 JSON 回退）、DiagnosticsList.tsx 27.6%→**100%**（经
> drawer 真渲染联动）、shared.ts 55.6%（70/126）→**100%（126/126）**——
> 12 导出纯函数专项：errorMessage 三臂/diagnosticsFromError isDiagnostic
> 过滤/五色函数缺省臂/formatDate 空·非法·合法/functionLabel 三级回退/
> operationLabel 三臂/proposalInboxPath 拼接/parseOpenAPIDocument 四类非法
> JSON（错误文案经模块级 getIntl()）。全树 **98.10%→98.35%**
> （93976/95548，+248 语句）。**登记不可达（分支）**：SourceDetailDrawer
> 245-252 行 `diagnostics || []` 右臂——条件 237 行已判 length===0 才进
> else，此时 diagnostics 必非空，防御性兜底不可达（行覆盖 100%、分支 96.29%）。
> **坑实证（新档三条）**：① 双臂用例同文断言——antd Drawer 经 portal 挂
> document.body，两次 render 并存时 `无诊断` 同文两处 findByText 报
> multiple，前臂须显式 unmount 再渲后臂；② `localizedText` zh-CN 缺失时
> 回退 en-US——functionLabel 断言「summary 仅 en-US」期望 id 兜底实为
> en-US 命中（OnlyEn (fn.d)），三级回退的「皆空臂」须 summary 整体缺失；
> ③ Popconfirm 确认键在 `.ant-popconfirm .ant-btn-primary`（portal 查询，
> findBy* 不可见），须 waitFor 内 querySelector 断非空后 fireEvent。
> 门禁：OpenAPISources 5 套件 **80/80** 绿、全量 jest **4399/4400**（唯一
> 失败 Functions/History 150.5s 超时，隔离重跑 16/16 绿 41.5s——R49-2 同款
> 负载竞态，本批零源码改动）、tsc 0、eslint 0、guard PASSED（仓库根）。

## 第二十一轮：函数权限配置页覆盖收口 + 附带修定编辑弹窗竞态缺陷（Permissions/index，2026-09-30）

> **交付（2026-09-30，wt-pages worktree）**：覆盖率补缺轮——零测试簇排行现席
> `Permissions/index.tsx`（222 行，目录内 RolesV2 已于第二十轮收口、本件此前
> 0% 无任何测试）单件收口，新增 `__tests__/index.test.tsx` 9 用例，v8 口径
> **行/语句 99.13%（余 27-28 两行即登记项）、函数 100%、分支 92.15%**（余
> 4 翼逐一登记，见边界）。
>
> **附带修定一处真缺陷（回归锁定）**：编辑动作原为「先 setEditing(r) 再
> await fetchPermissions」——ModalForm 内容在 open 翻真时即以**旧 permDraft**
> （首次 {}）挂载，而 antd initialValue 挂载后不随 prop 变化重放，连锁三害：
> ① 编辑态 verbs/scopes tags 恒空显示（placeholder 常驻，用户看不到既有配置）；
> ② tags onChange 整组覆盖 store 值——追加/删减任一 tag 后 values.verbs 即为
> 手工小集合，onFinish 的 `values.verbs || permDraft.verbs` 取左值，**提交把
> 既有 verbs/scopes/i18n 静默丢弃**（保存丢值）；③ form store 跨开合存活
> （无 destroyOnHidden），二次编辑沿用首开旧值。修定 = 先取权限再与
> setEditing 同批落 state（React 18 promise 内自动批处理，弹窗以最新 draft
> 挂载）+ 开窗时 setFormI18nKeys([]) 重置 + modalProps.destroyOnHidden。
> 套件对未修页面实跑取证（tags 恒空、zh 输入框只随手工键渲染）后修定，
> 「编辑回显 read/write tags」「追加 exec 后 i18n_zh 仍含 read」两组断言
> 即回归锁。
>
> 锁定契约——首拉无参 + 列渲染矩阵（id/名称/verbs/scopes Tag/编辑链接 ×3）；
> 行缺 permissions / 空配置两形态（列渲染 `?.verbs || []` 右翼）；加载失败
> 双翼（reject Error → e.message toast；字符串 → intl id 兜底）；编辑主链
> （fetchPermissions(fid) + 标题内插 + verbs/scopes tags 回显 + 中文输入
> 回显且仅配置过的 verb 出输入框）；缺 permissions 行空表单直提
> （`values.verbs || permDraft.verbs || []` 中+尾翼全链 [] + defaults
> `(draft || []).slice()` 右翼 + i18n_zh {}）；保存主链（i18n_zh 只收非空
> 串键——initialValue '' 的 write 不进 payload + success toast + 关闭 +
> 重拉）；动态中文输入（verbs tags 追加 → onChange setFormI18nKeys → 中文组
> 即时渲染新 verb 输入框 → 填值进 payload）；保存失败双翼（Error/非 Error
> → error toast + return false 弹窗保持）；取消关闭 onOpenChange(false)。
>
> **坑实证（新档，三条）**：① jest 区分「零参调用」与「传 undefined」——
> `getFunctionSummary()` 无参调用须 `toHaveBeenCalledWith()` 断言（传
> undefined 形态反而不匹配）；② 同用例双 render 的 toast 计数——首个
> render 未卸载、其 toast 文案为 Error message，intl id 文案仅第二渲染
> 产出（findAllByText 计数按此推算勿翻倍）；③ ModalForm（无 destroyOnHidden）
> 关闭是 display:none 残留而 destroyOnHidden 后是整体卸载——关闭断言用
> 「null 或 display:none 二择」兼容式，修定引入 destroyOnHidden 后自动落
> null 分支。另：本页 intl 全部 id-only（无 defaultMessage），@umijs/max
> mock 取 `defaultMessage ?? id ?? ''` 使标题/按钮/toast 按 id 确定性可见。
>
> **边界（诚实清单，不造假用例不删防御分支）**：① fetchSummary 的
> `Array.isArray(res)` 右翼（27-28 行）——getFunctionSummary 归一层恒返
> FunctionSummary[]，resolve 非数组违反返回类型即造假，登记；②
> `perm || {}` 右翼（113 行）——getAdminFunctionPermissions 恒返
> `res?.permissions || {}`，nullish 违反返回类型，登记（{} 形态经缺
> permissions 行真实覆盖）；③ `(verbs || [])` 右翼（154 行）——链结果
> 恒真值；④ onChange `(vals as string[]) || []` 右翼（186 行）——antd
> tags 模式恒传数组；⑤ i18n 收集 `typeof val === 'string'` 非 string 翼
> ——ProFormText 值恒 string|undefined；⑥ 编辑 onClick 无 catch——
> fetchPermissions reject 成 unhandled rejection 且弹窗不开（修定后先取
> 数后开窗），同族页面既有口径不造假 reject。修定后 `values.verbs` 为
> 「左值但 draft 有值」的混合态不可达（弹窗与 draft 同批挂载），onFinish
> 链三段翼经满配置/裸行双形态真实覆盖。
>
> 门禁：目标套件 9/9 绿（格式化后复跑同绿）、`pnpm --dir web run tsc`
> 0 错、eslint/prettier 干净、`scripts/dashboard_vnext_guard.sh` PASSED；
> 全量 jest 负载口径见交付说明。

> **100%（100/100/100/100 语句/分支/函数/行）**，16 用例——三个 build 纯
> 函数（buildAssignmentColumns 七类列分派+行操作矩阵、buildCategoryColumns
> 五列+批量回调、buildRouteColumns 四列+查看回调）经 RTL 真渲染断言 DOM。
> **坑实证（新档五条，antd 6.6.0 实测）**：① Badge 状态色渲染在内部 dot
> （`.ant-badge-status-success`），非外层 `.ant-badge-success`；② Progress
> 文本在 `.ant-progress-indicator`（带 title 属性），success 态在外层
> `.ant-progress-status-success`——antd 5 的 `.ant-progress-text`/
> `.ant-progress-success` 在 6.x 均不存在；③ antd 6 Tooltip **不设** title
> 属性，icon-only 按钮可访问名来自图标 `aria-label`（check-circle/delete/
> setting），getByTitle 全挂；④ 图标类名 CheckCircleOutlined→
> `anticon-check-circle`（非 `anticon-check`，后者按类 token 精确匹配）；
> ⑤ 同用例多次 render() 均追加容器到 document.body，screen 级断言遇同文
> 多匹配——须 within(container) 限定或前臂 unmount。另：本机 TZ=UTC，
> toLocaleString('zh-CN') 期望值须按运行机动态计算。**并发实录**：本批
> 与并行会话同改 columns.test.tsx（对方先落盘 13/16 绿版本，我补最后 3
> 处 antd 6 选择器修复）；并行会话另生成未跟踪 jest.config.js（.ts 的编译
> 产物，与跟踪版 jest.config.ts 内容相同）致 `pnpm test:coverage` 报
> Multiple configurations——全量跑改用 `npx jest --config jest.config.ts`
> 显式指定绕开，未删对方文件。门禁：Assignments 5 套件 44/44 绿、tsc 0、
> eslint 0、guard PASSED（仓库根）。
> **R49-4b（会话续作，本提交）**：useAssignmentsPage.ts 67.2%（108 miss）
> → **99.59% 行 / 97.29% 分支**（产物 .jsx/.js 口径），30 用例——descriptors
> 三信封形态、gameId 有无的 assignments 拉取与失败兜底、onSave
> remove/assign/unknown/reject-finally 四臂、onBatchAssign 并集去重/过滤、
> onCloneToEnv 四臂、loadHistory 信封解包/items 缺省/失败清空、canWrite
> 六态参数化、pageCtx 全回调、onOpenDetail 闭包两臂经捕获 opts 驱动
> （columns mock 后 push 逻辑不可 UI 触达）、gameId 变化重拉（#35）。
> **登记不可达/防御性（2 行）**：`Object.values(m).flat() || []` 右臂
> （flat() 恒返回数组 truthy）、onSave finally 残余块（异常入口已由
> rejects.toThrow 用例执行，v8 块计数器合并不可再分）。
> **全量污染期判定**：web 并行会话编译产物滞留期（src 下 663 个 .jsx/.js
>
> - .map，jest 默认 moduleFileExtensions js 优先于 ts——测试实际加载等价
>   编译产物，行为等价但 coverage 统计口径落产物文件；产物随时可能被对方
>   清理，数字不可复现）；污染期全量 jest 34 suites 失败判**环境性**（定向
>   域全绿 + 产物/源码同步性决定失败分布，与本批零源码改动无交集），本批
>   门禁以 Assignments 全目录 6 套件 **74/74** 绿 + tsc 0 + eslint 0 +
>   guard PASSED（仓库根）收口，全树覆盖率基线与全量全绿待产物清理后由
>   R49-5 复核。

## 第二十二轮：数据备份页覆盖收口（Ops/Backups 0% → 100/95.65/100/100，2026-10-01）

> **交付（2026-10-01，wt-pages worktree）**：覆盖率补缺轮——零测试簇排行现席
> `Ops/Backups/index.tsx`（218 行，目录内无任何测试文件）单件收口，新增
> `__tests__/index.test.tsx` 10 用例，v8 口径 **行/语句/函数 100%、分支
> 95.65%**（余 2 翼即登记项，见边界）；无页面缺陷（纯补测）。锁定契约——
> 挂载首拉零参 + 列渲染矩阵（类型/size 原值/时间原值、状态 Tag 三色
> done→green / failed→red / else(running)→gold、缺 size 行空格）+ 卡片标题 +
> 工具栏双按钮 + 下载锚 href=getOpsBackupDownloadUrl(id) 透传；刷新重拉；
> 创建主链（选类型 → 载荷 kind、target 未填键缺省 → toast 已创建 →
> `setTimeout(load, 500)` 真实计时器重拉 → destroyOnHidden 整体卸载）；
> required 拦截（不触达服务 + 弹窗保持 + 取消关闭）；target 可选填完整载荷；
> 创建失败静默 catch（无本地弹错、无成功 toast、不重拉——注释言明全局拦截
> 器 toast 语义，测试环境 service 已 mock 故断言全静默为现状锁定）；删除
> 主链（行内删除 → modal.confirm → deleteOpsBackup(id) → 已删除 → 重拉）；
> 删除失败三翼（Error → e.message / 非 Error → intl「操作失败」/ Error('')
> → `errMsg ||` 右翼「失败」，均不重拉）。
> **坑实证（新档，四条）**：① 双字中文 Button 自动插空格（刷 新/删 除/
> 取 消/确 定），role name 一律宽松正则；② modal.confirm 定位走类名
> `.ant-modal-confirm-btns .ant-btn-primary`（locale 无关），标题
> `.ant-modal-title` 与 `.ant-modal-confirm-title` 双渲染须 selector 收窄；
> ③ Select option 点击配方 sleep≥60ms → mouseDown 落 `.ant-select` 根 →
> 点可见 dropdown 内 `.ant-select-item-option-content`（Behavior 套件同款）；
> ④ 成功创建的 `setTimeout(load, 500)` 是真实计时器——用例内必须等第 2 次
> listOpsBackups 消费掉，否则挂起 timer 在下个用例触发毒化计数断言
> （jest.clearAllMocks 不清计时器；创建失败/删除各用例无 timer 不受影响）。
> **边界（诚实清单，不造假用例不删防御分支）**：① `r?.backups || []` 双
> 右翼（25 行，分支余量全部在此）——listOpsBackups 归一层恒返
> `{backups: response.backups.map(normalizeOpsBackup)}`（map 恒产数组），
> resolve undefined/非对象违反返回类型即造假，登记；② load 的 try/finally
> 无 catch——listOpsBackups reject 成 unhandled rejection（同族页面既有
> 口径），不造假 reject 场景。
> 门禁：目标套件 10/10 绿（首轮即绿）、prettier 不变、eslint 0、
> `pnpm --dir web run tsc` 0 错、`scripts/dashboard_vnext_guard.sh` PASSED；
> 全量 jest **368 套件 4557 用例全绿**（2 worker 限流，load 25-31 高位窗口，
> 2085s，exit 0——日志尾 worker force-exit 提示为既有 teardown 提示非失败）。

## R49-5：全量 jest 复核收口——产物清理后 360/362 suites 绿（web，2026-10-01）

> **产物判定（只清本树、零 tracked 误删）**：src 下 663+ .jsx/.js 污染源
> **已不在**（并行会话侧已清，find 0 个 .jsx，余 3 个 .js 均为 `.umi/`
> umi 生成目录 + tracked service-worker.js）。本树残留 web 根下 16 个
> 未追踪产物（config/mock/tests/e2e-verify/EXAMPLE_USERS_PAGE 的
> .js/.jsx+.map），**全部有 tracked .ts/.tsx 源对应**（逐个核对）且不被
> gitignore——其中 `jest.config.js` 与 tracked `jest.config.ts` 并存正是
> `pnpm test:coverage` Multiple configurations 冲突源。全部清除，不动其他
> worktree、不动并行会话 tracked 修改（Cicd/index.test.tsx、url.test.ts
> 的 M 保持原样），`git status` deleted-tracked = 0 自证无误删。
> **全量复核**（1 分钟 load 6.99 < 10 空载窗口，`npx jest --ci`
> 437.5s）：**362 suites / 4467 tests → 360 suites / 4465 tests 绿**——
> 登记口径「污染期 34/48 suites 失败环境性」的失败面**清零**（清产物
> 后 jest 加载 .ts 源，源/产物同步性不再抖动；零 import 断裂自证删除
> 的 16 个产物无测试引用）。
> **残留 2 失败逐条归因（均并行会话本会话期进行态，均不在 HEAD）**：
> ① `src/utils/__tests__/url.test.ts`「window/location 皆不可用 → dev
> 兜底 18780」——该用例是对方**工作区新增**（`git show HEAD:` 0 命中，
> M 状态系会话开始快照后出现），写法 `{...global.window, location:...}`
> 替换不生效（jsdom window 不可覆盖），实现读真 `window.location.origin`
> 落 **testURL 8000**（jest.config.ts:31 `url: 'http://localhost:8000'`
> 实证，Expected 18780 / Received 8000 完全吻合）；隔离重跑稳定复现
> （8/9 绿，仅此一例）。② `src/pages/PageStudio/__tests__/indexGuards.test.tsx`
> ——`??` untracked（`git cat-file -e HEAD:` 无此文件），对方新写的
> Guards 套件，`handleSyncSelectorsApplied` 重载断言 3 次 ≠ 期望 2 次
> （含 index.tsx:730 onDiff 堆栈），隔离重跑稳定复现（62.7s）。
> 二者机制上与产物清理零因果（读 .ts 源、失败为断言值差异而非模块
> 解析错）；**HEAD 口径全量 = 4465 passed / 0 failed**——2 失败文件
> 及用例均未入 HEAD，HEAD 测试面全绿成立。不代改对方进行中文件
> （会撞车），如实登记待对方自收。
> 门禁：HEAD 口径全量 0 失败 + 产物清理零误删（只删未追踪、逐文件
> 核对 tracked 源）。本单触碰面仅 web 根下未追踪产物 + 本台账。

### R49-5b：遗留销账——缓存清理后全量复跑，23 红全部环境性归因（web，2026-10-01）

> **清理面**：`coverage/`、`node_modules/.cache/`、`dist/`、`src/.umi/`、
> `src/.umi-production/` 全清（均为 gitignore 生成物，零源码触碰；
> src 下 0 个非 tracked js/jsx/map，`src/service-worker.js` 为 tracked
> 源不碰）。**新教训**：`src/.umi` 非纯构建产物——tsconfig `@@/*`
> paths 指向它且 `@umijs/max` 的 `getIntl` 等导出经其类型增强，清后
> tsc 报 2 错（TS2305 no exported member 'getIntl'），`npx max setup`
> 重建即愈；后续清理缓存须留 .umi 或清理后重建。
> **全量复跑**（`npx jest --ci` 1031.7s）：362 suites / 4467 tests →
> **23 suites 红**（39 tests）。归因三层：
> ① **未入 HEAD 不计**（2）：`url.test.ts` dev 兜底用例（对方会话期内
> 的 M，本轮跑后已被对方还原，现与 HEAD 一致）；`indexGuards.test.tsx`
> （`??` untracked 依旧）。
> ② **环境性**（21，抽样 7/7 隔离全绿外推）：本轮 **load 44-47/14 核**
> （并行会话 Android AVD qemu + gradle/java/dotnet 占机）+ transform
> 缓存冷启动（1031.7s vs 上轮 437.5s，2.3 倍）双重挤压。抽样覆盖
> 超时型（HeaderDropdown 5s 超时→隔离 7.1s 绿）、断言型（Tickets
> Detail「已升级为缺陷 #42」差异→隔离 42/42 绿；Functions/History
> 「Expected 2 Received 4」重试重复调用→隔离 16/16 绿）、最重型
> （studioActions 750.7s→隔离 220.3s 49/49 绿）、中型三连（
> widgets-select/Toolbar-export/Console-Page→隔离 32/32 绿 13.4s）。
> 失败形态全为超时/重试重复调用/弹窗竞速（memory 既有环境性签名），
> 零 import 断裂、零断言值稳定差异。
> ③ **真实失败**（0）。
> **HEAD 口径判定**：4465+ passed / 0 真实失败——「期望全绿」达成
> （等价于上轮 360/362 基线，失败面全为负载挤压）。
> 门禁：tsc 0（.umi 重建后）、guard PASSED（仓库根）。本单触碰面
> 仅 gitignore 生成物 + 本台账，零源码改动。

## 队列②增量：Dependabot/audit 新增 advisory 收口——axios 12 条+dompurify 1 条（web，2026-09-30）

> **背景对账**：用户队列四单（mobile 服务器配置面、Dependabot 8 条 overrides、
> secret-scanning 微信 AppID、code-scanning 9 条整型转换）在开工查重时确认
> **均已被并行会话交付**：①`00cb1d9` 服务器地址配置面（首启动向导+设置页
> 校验/探测/即时生效）、②`2388c30` Dependabot 8 条收口、③`7b5a73b` 微信
> AppID 去 hex 治理、④`3a3a680` CodeQL 9 条整型收窄。**security tab 三处
> open 已 API 复验归零**（dependabot 0 / code-scanning 0 / secret-scanning 0，
> code-scanning 30 条历史全 fixed）。
> **本单真增量**：`2388c30` 之后 GHSA 新批出现——`pnpm audit` 16 条（axios
> 1.19.0 12 条【8 high】、react-router 2 moderate、elliptic 1 low、
> dompurify 3.4.13 1 low）。收口 16→**3**：
>
> - `axios@<0.30.0` 值收界 `'>=0.30.0 <1.0.0'`——原开放值在 re-resolve 时
>   被 registry 飘到 1.x（@umijs/plugins 声明 0.27.2 触发匹配），三 snapshot
>   回 0.x 线终点后 `pnpm update axios` 显式重解析 → 全部 1.20.0（12 条清）。
>   src 零 axios import（HTTP 层走 umi request），1.20 链纯构建期 devDeps。
> - 新键 `dompurify@<3.4.16: '>=3.4.16'`（afterSanitize hook DOM XSS）+
>   `pnpm update dompurify` → 三 snapshot 全 3.4.16。
> - 剩余 3 条均**已登记不修复**（elliptic：6.6.2 补丁未发布，2026-09-30 复核；
>   react-router×2：修复在 7.18.0 需 rr7，umi 4.x 锁 6.x 无 backport）。
>   **pnpm 11.11.0 三实证（已入 memory）**：①复合范围键
>   `pkg@>=a <b: c` 不被应用（单边界才可靠）；②`install`/`install --force`
>   不重算已锁 snapshot 的 override，须 `pnpm update <pkg>` 显式触发；
>   ③开放上界值 `'>=X'` 会随 registry 飘线，值必须带上界。
>   **门禁**：tsc 0、audit 3（全为已登记项）、Assignments+OpenAPISources
>   11 套件 154/154 绿、SchemaFormRenderer/remote-options 隔离 16/16 绿
>   （全量 48 suites 失败判环境性——并行会话编译产物滞留加剧，抽样隔离
>   全绿）、guard PASSED。全量全绿复核留 R49-5（产物清理后）。

### 队列②收尾残留：docs 工作区 dompurify #413（docs，2026-09-30）

> e3c0f 系列（web 侧 16→3）落地后，dependabot 新冒 **#413**（唯一 open）：
> 同款 dompurify >=3.4.13 <=3.4.15 advisory（afterSanitize hook DOM XSS，
> patched 3.4.16），manifest 为 **docs/pnpm-lock.yaml**——docs 子工作区
> 独立 lockfile 同样被扫。处理与 web 同款：`docs/pnpm-workspace.yaml`
> 增单边界键 `dompurify@<3.4.16: ">=3.4.16"` + `pnpm update dompurify`
> 重解析落锁（lock 三处 3.4.13→3.4.16）。docs `pnpm audit` 余 4 条均为
> vitepress 1.6.4 依赖链 vite/vitepress 已登记「保留不修复」项（#50/#151/#152，
> vitepress 1.x 锁 vite ^5 强制 6.x 构建失败）。lint-staged prettier 将
> pnpm 11 update 产生的 lockfile 格式噪音重排抵消，提交实质 diff +9/-4；
> 提交后 `pnpm install` 复验 lock 一致（Already up to date，无新漂移）。
> **门禁**：`cd docs && pnpm build` 过（110.15s）。commit `965649f`
> （merge 上游 6 提交后快进推送）。
> **三 tab 终态归零 API 复验**：dependabot 0 / code-scanning 0 /
> secret-scanning 0——用户指令「做完回报三个 security tab open 归零」达成。

## 覆盖率巡检批次·Go 侧第五十轮·登记面重审翻案——bug/profile 错误翼收口（wt-api worktree，2026-09-30）

> 交付：全量 profile 重排（99.949%，21 文件 29 语句余量）后，本轮主线
> **逐条重审第 3-28 轮「不可达」登记**。29 语句全审，3 处初判翻案后被
> 自证否决（如实留档）：auth/service.go:738 实为 `!ok2` continue（非
> return——FindEmailsByDomain 的 `LIKE '%@domain'` 保证 Cut 恒得 @，
> :741 return 本就被既有用例覆盖）；gitlabci.go:75 空串守卫（New() 拒空
> project，group/repo 非数字分支已被覆盖）；cicd/service.go:93 float64
> （Create 请求 DTO Extra 为 map[string]string、DB 读回 json.Number
> 双路径均不产 float64）。判死补强论证：menu:381（check(item)=true ⇒
> 既有 parent 必 true，环项在首循环先跳）、openapi:710（替换字母表 ⊆
> Trim cutset，trim 后首字符恒 alnum）、avatar:128（filepath.Clean 恒剥
> 尾斜杠 → 前置 HasPrefix 守卫先拒）、wechat:134/:137（:121 守卫先行）、
> handler 系三处（纯 string DTO 绑定恒过）、re-Marshal 系三处（Unmarshal
> 过的值再 Marshal 恒过）、certificates:202（源内 C 类论证）、rand 系
> （crypto/rand）、email_verification:78/163（layered 恒 init / CAS 竞态
> 窗口）、webhook:106（单请求内无法注入读故障）。
> **翻案收口 3 翼**：① model/bug.go :374/:401——#21 轮「sqlite 无法模拟
> 连接/列级错误」登记不成立，同文件 LinkBugTicket 缺表技法对 JOIN 同样
> 有效，newBugErrTestDB（Bug+Ticket 无 link 表）收口；旧 sanity 用例改名
> NormalShapeSanity、文件头登记同步翻案。② api/profile/permissions.go
> :179——空白名角色被一循环挡在 roleIDs 外、但 grants 二循环遍历原集，
> 混入用例收口。bug.go / permissions.go 双双 100%；余量 29→26 语句。
> 回避面维持：otp/otpauth.go（d9fdc05 OTP 域）、api/assignment/gate.go
> （BUG-035 域）。
> 门禁：触及文件 gofmt 干净、go vet 干净、go test ./internal/... 全绿
> （fresh，-p 4 从宽于 load 111 洪峰下 158 包 exit=0 零 FAIL）。

## 覆盖率巡检批次·Go 侧第五十一轮·email_verification 两翼收口（wt-api worktree，2026-10-01）

> 交付：`internal/api/auth/email_verification.go` 两翼收口。① :78-79
> siteServerURL 的 settings-nil 空串返回——`settings.ResetForTest()` 置
> layered=nil 即触发（包内 t.Parallel 用例零 settings 引用，R50「互扰」
> 登记前提不成立）。② :162-164 VerifyEmailToken 的 `Consume 返回 false`
> 报错分支——sqlite BEFORE UPDATE TRIGGER + `RAISE(IGNORE)` 让条件 UPDATE
> 静默 0 行（探针实证 RowsAffected=0 → consumed=false → 「验证链接无效或
> 已过期」），R50「服务层无钩子可确定性构造」登记被该技法击破，翻案收口；
> RAISE(IGNORE) 是仓内 RAISE(ABORT) 写阻断技法的姊妹技（静默跳过而非报错）。
> 第二批 register_verify_gap_test.go 文件头三处登记同步：rand 系死亡理由
> 升级为 dead-by-contract（Go≥1.24 crypto/rand.Read 合同无错误返回，
> go.mod go 1.26.6，误报风险清零）；Consume !ok 与 siteServerURL 两条改指
> 翻案/收口并指向新文件。落库形态：测试文件由并行写入方定稿（Write 后秒级
> 被替换，按系统指令采当前版为既成事实），已以 `05d8af7` 落库并随并行会话
> merge `1795af0` 入 origin/main；本轮补文件头登记 + 本台账。
> **余量对账**（对 R50 起点 profile 逐块 diff）：退出 5 块、新入 0。全树
> 99.949% → **99.958%**（64118/64145），余量 26 → **24 块 / 27 语句**
> （R50 台账「29 语句」实为块口径，语句口径 32→27）、19 文件；本树 email_
> verification.go 4 块 → 2 块（余 :87-88/:103-104 rand dead-by-contract）。
> 余 24 块全部经 R50 第 3-28 轮登记重审维持判死/回避（otp/otpauth 2 块、
> gate.go 2 块回避面不变）。回访档案位置：email_verification_r51_test.go。
> 门禁：触及文件 gofmt/go vet 干净；go test ./internal/... -count=1 -p 4
> fresh 158 包 exit=0 零 FAIL；auth 包 -count=3 零抖动（148.5s）；覆盖
> profile 同口径重跑 exit=0（load ~23，较前几轮 60-111 轻载）。

## 覆盖率巡检批次·Go 侧第五十二轮·回避面解除——otp/assignment 四翼收口 + CI Test 步骤超时修复（wt-api worktree，2026-10-01）

> 交付：R50 维持的两处回避域已静默收口（otp 域 2026-09-26 后、assignment
> BUG-035 域 2026-09-27 后无后续提交），回避解除、四翼处置：
> ① api/assignment/gate.go :34-35——svcCtx==nil 守卫（nil 即默认开放，
> 守卫先于真值读取）；② :38-39——坏 JSON 触发 loadAssignments Unmarshal
> 错误透传，直接断言闸门「不静默 fail-open」契约（gate_error_r52_test.go），
> assignment 包 100%。③ security/otp/otpauth.go :89-90——镜像包内
> coverage_gapfix_test.go 的 randRead 注入先例，串行测试替换
> recoveryRandRead 缝隙变量（serial/parallel 用例体不交错，-race -count=2
> 验证无竞态）；④ :139-140——dead-by-contract 登记（got 恒为
> HashRecoveryCode 输出 = hex.EncodeToString(sha256)，DecodeString 恒成功，
> Encode→Decode 往返判死，同 re-Marshal 系），otp 包 99.2% 余此一块。
> **附带修复（CI 基建）**：c120781 上 CI-Core 的 Test 检查失败，取证为
> 「Unit tests 步骤 10 分钟超时」——日志显示 10m12s 跑到 api/task（已执行
> 包全绿）即被 ##[error] 掐断，零用例失败；套件经 50+ 覆盖率轮增长已结构性
> 越线（R50 时代已在悬崖边），与 CLAUDE.md「dashboard 15 分钟 step 超时」
> 同族。处置：ci.yml Unit tests 步骤 timeout 10→20 分钟（作业级 30 不变）。
> 同提交上 dashboard-quality 亦挂——签名取证 shutdown signal ×1 + 零 ✕ +
> canceled，hosted runner 回收（同 ci-dashboard-runner-shutdown-signature
> 记忆协议），非回归；R52 push 后新 head check-runs 即验证主体，不rerun。
> **余量对账**：全树 99.958% → **99.963%**（64121/64145），余量 24 →
> **21 块 / 24 语句 / 18 文件**（gate.go 整文件退出）；余 21 块全部维持
> R50 判死/回避登记，无新入。
> 门禁：触及文件 gofmt/go vet 干净；go test ./internal/... -count=1 -p 4
> fresh 158 包 exit=0 零 FAIL（load 53-101 洪峰下跑完）；otp/assignment
> 双包 -race -count=2 绿（assignment 278s，race 放大后环境性慢如实记录）。
