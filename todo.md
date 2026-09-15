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
