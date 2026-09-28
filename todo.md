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
