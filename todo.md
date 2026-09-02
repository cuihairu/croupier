# 平台安全与可观测性补强 TODO

方案共识：MFA 按 provider 维度接线（local 默认建议开启，LDAP/OIDC 默认信任 IdP）；Prometheus 端点提供但默认关闭；登录锁定与 token 撤销按 tokenVersion 方案；清理文档债。

每个任务原子性：独立完成、独立迁移文件、独立测试、独立可提交，互相之间无代码依赖（仅迁移序号需按落地顺序递增）。

> **状态：全部完成（T5→T1→T2→T3→T4 顺序落地，2026-09-02）**
>
> - T1: 迁移 0017（admins.failed_attempts/locked_until/token_version）+ `LoginLockoutConfig`（默认 5 次/15 分钟，仅 local 计数）+ `auth.login_locked` 审计；测试 `internal/api/auth/lockout_test.go`
> - T2: JWT claims 增 `tokenVersion`（旧 token 解析为 0 平滑兼容）；中间件比对 + 30s 进程内缓存；登出/改密（自助+重置）/禁用账号递增；测试 `internal/svc/token_revoke_test.go`
> - T3: 迁移 0018（admins.otp_enabled）+ MFA API（setup/confirm/disable）+ 登录按 provider 分支（local 校验 totpCode，外部身份源跳过）+ `mfa_required` 错误码；测试 `internal/api/auth/mfa_test.go`
> - T4: `telemetry.prometheus.enabled`（默认 false）+ exposition 端点（默认 `/metrics/prometheus`，免认证白名单动态注册）+ go/process + 平台指标（与 JSON /metrics 同口径）；测试 `internal/api/monitoring/prometheus_test.go`；文档 `docs/operations/monitoring.md` 已按实际实现修正
> - T5: 删除 `internal/api/user` 空壳；CLAUDE.md 路径漂移修正（`internal/auth/` → `internal/security/`）
>
> 已知边界：token 撤销最长 30s 生效延迟（缓存 TTL）；MFA 前端二次输入 UI 未做（后端契约就绪，`401 + error=mfa_required`）；`docs/security.md` 已记录。

## T1. 登录失败锁定 ✅

**目标**：密码连续失败 N 次后锁定账号 M 分钟，防止在线爆破。

**改动点**：

- `internal/db/migrate/migrations/` 新增迁移：`admins` 表加 `failed_attempts INT NOT NULL DEFAULT 0`、`locked_until TIMESTAMP NULL`
- `internal/model` Admin 模型加对应字段
- `internal/api/auth/service.go` `Login()`：
  - 认证前检查 `locked_until > now` → 拒绝并审计 `auth.login_locked`
  - 失败时 `failed_attempts+1`，达到阈值（默认 5 次）写 `locked_until = now + 15min`
  - 成功后清零 `failed_attempts`
- 阈值/窗口走配置（`security.loginLockout.threshold` / `windowMinutes`），带默认值，配置文件示例更新
- OIDC/LDAP 登录失败不计数（失败在 IdP 侧）；仅 local provider 失败计数

**验收**：

- 新增单测：连续 5 次失败锁定、锁定期内正确密码也拒绝、窗口过后自动解锁、成功登录清零计数
- `go test ./internal/api/auth/...` 通过
- 锁定事件写入哈希链审计可查

## T2. tokenVersion 撤销机制 ✅

**目标**：改密码、禁用账号、登出后旧 token 立即失效，消除 24h 不可吊销窗口。

**改动点**：

- 新增迁移：`admins` 表加 `token_version INT NOT NULL DEFAULT 0`
- `internal/security/jwtutil/token.go`：claims 加 `tokenVersion`，`Sign` 接受版本参数
- `internal/middleware/auth.go`：验签后比对 claims 版本与库中当前版本，不一致返回 401（查询走带短 TTL 的进程内缓存，避免每请求打库；缓存与 T1 的锁定检查可复用同一查询）
- `internal/api/auth/service.go`：`Logout` 递增 `token_version`（全局登出）；`issueLogin` 签发时带上当前版本
- 改密码（admin PasswordReset / profile 修改密码）、禁用账号处递增 `token_version`
- 审计事件 `auth.token_revoked`

**验收**：

- 新增单测：登出后旧 token 401、改密码后旧 token 401、正常 token 不受影响、缓存失效后版本同步
- `go test ./internal/api/auth/... ./internal/middleware/...` 通过

## T3. MFA（TOTP）按 provider 接线 ✅

**目标**：`VerifyTOTP` 接入登录流程；local provider 账号可启用 TOTP，LDAP/OIDC 登录跳过平台 MFA（信任 IdP）。

**改动点**：

- 新增迁移：`admins` 表加 `totp_secret VARCHAR NULL`、`totp_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `internal/api/auth/`（或新子模块）新增 MFA 管理 API：
  - `POST /api/v1/auth/mfa/setup`：生成 secret 返回 otpauth URL
  - `POST /api/v1/auth/mfa/confirm`：校验首个 code 后置 `totp_enabled=true`
  - `POST /api/v1/auth/mfa/disable`：校验 code 后关闭（需登录态 + 密码确认）
- `internal/api/auth/service.go` `Login()`：密码通过后，若 `provider == local && totp_enabled`：
  - 请求未带 `totpCode` → 返回特定错误码（如 `mfa_required`），前端据此展示二次输入
  - 带 `totpCode` → `VerifyTOTP` 校验，失败审计 `auth.mfa_failed`
  - LDAP/OIDC 身份直接跳过该分支
- 复用已定义的审计事件 `auth.mfa_enabled/disabled`
- `LoginRequest` DTO 加 `totpCode` 可选字段

**验收**：

- 新增单测：local+MFA 无 code 返回 `mfa_required`、错误 code 拒绝、正确 code 放行、LDAP/OIDC 登录不触发 MFA、setup/confirm/disable 状态机
- `go test ./internal/api/auth/...` 通过
- 前端二次输入 UI 不在本任务范围（仅后端契约 + 错误码），在交付说明中列为已知边界

## T4. Prometheus /metrics 端点（默认关闭） ✅

**目标**：提供标准 Prometheus exposition，不强制部署 OTel Collector。

**改动点**：

- 配置新增 `telemetry.prometheus.enabled`（默认 `false`）、`telemetry.prometheus.path`（默认 `/metrics/prometheus`，避开现有 JSON `/metrics`）
- 引入 `prometheus/client_golang`，`internal/telemetry/` 新建 prometheus provider：注册 DB 延迟、agent 在线数、函数调用计数等已有 OTLP 指标的对应 prometheus collector（只暴露已在 OTLP 侧存在的指标，不新增口径）
- `internal/handler/routes.go`：开关开启时挂载 `promhttp.Handler()`
- `configs/telemetry.example.yaml` 补示例

**验收**：

- 新增单测：开关关闭时路由 404、开启时返回 exposition 文本且含预期指标名
- 默认配置下行为与现状一致（回归）
- `go test ./internal/telemetry/... ./internal/handler/...` 通过

## T5. 文档债清理 ✅

**目标**：删除空壳模块、修正 CLAUDE.md 路径漂移。

**改动点**：

- 删除 `internal/api/user/`（仅 dto.go + dto_test.go，无 handler/service，功能由 `internal/api/admin` 承担）；确认全仓库无 import 引用
- CLAUDE.md：
  - `internal/auth/  # RBAC, JWT, TOTP, user management` → 修正为实际路径 `internal/security/` + `internal/api/auth`
  - `internal/auth/rbac/` → `internal/security/rbac/`
- 顺带核对 CLAUDE.md 中其他 `internal/server/http` 等路径引用与实际 `internal/api/` 的一致性

**验收**：

- `go build ./...` 通过、`go test ./internal/...` 无回归
- `rg "internal/auth/" CLAUDE.md` 无残留

## 执行顺序建议

T5（无风险热身）→ T1 → T2 → T3 → T4。前四个每个含独立 goose 迁移，按落地顺序编迁移序号；每个任务单独提交。

---

# 函数注册 → UI 生成链路体验补强 TODO（F 系列）

现状诊断（2026-09-02 全链路走查结论）：

- **两套 UI 体系脱节**：Invoke 工作台（`web/src/pages/Functions/Invoke/index.tsx:110`）把 `inputSchema` 直接包成 `{ jsonSchema, layout: 'vertical' }`，`FormPresentationSpec` 的 widget/分组/联动机制完全没接入（descriptor 里没有呈现数据，也没有推导层）；PageSpec/PageStudio 体系能力完整但靠人工配置，与 descriptor 不打通。
- **widget 大面积退化**：`SchemaFormRenderer/index.tsx:133-174` `widgetToRjsf` 中 Select/Cascader/TreeSelect/Upload/RichText/KeyValue/Array/Object 等返回 undefined，退化为默认 input。
- **结果渲染裸 JSON**：`InvocationResponse.tsx` 仅 Monaco viewer，`outputSchema` 未参与渲染；错误 `details` 不结构化。
- **无远程枚举源**：选择玩家等高频操作只能手输 id。
- **契约演进无保护**：注册 schema 直接覆盖，无 diff/breaking 检查；`Diagnostics`/`SourceDigest` 存了但 UI 不消费。
- **其他**：visibleWhen 是删除式隐藏（丢数据）、FormGroupSpec 定义了但 `deriveRuntimeSchema` 未实现、AJV 错误消息英文裸奔、async 调用只弹 taskId 无进度面板。

约定：proto `FunctionDescriptor` 保持无 UI 元数据（有意设计），呈现富化通过 **JSON Schema `x-ui-*` 扩展字段**（`input_schema` 本就是字符串透传，wire 契约零改动）+ 平台侧覆盖层实现。每个任务原子：独立实现、独立测试、独立提交。

> **状态：全部待办**

## F-A 阶段：呈现层打通（P0，改动小见效快）

## F1. x-ui 呈现 hints 契约规范（文档先行）✅

**目标**：定义 JSON Schema `x-ui-*` 扩展字段规范，作为 SDK/游戏方声明呈现意图的唯一约定。

**改动点**：

- [x] 新增 `docs/architecture/presentation-hints.md`：字段清单（`x-widget`/`x-label`/`x-placeholder`/`x-group`/`x-col-span`/`x-order`/`x-visible-when`/`x-options-source` 等）、与 `FormPresentationSpec`（`web/src/types/dashboard.ts:692` FormFieldSpec）的映射表、兼容性说明（未知 x-* 对 OpenAPI/AJV 无害透传）
- [x] `docs/architecture/pagespec-protocol.md` 增补「descriptor hints → presentation spec 推导」小节链接
- [x] `cd docs && pnpm build` 通过

**验收**：规范含完整字段表与 3 个以上示例（含远程选项源示例）；文档构建通过。

## F2. 前端推导器：inputSchema x-* hints → FormPresentationSpec ✅

**目标**：Invoke 工作台真正吃到 FormPresentationSpec 能力，取代裸 schema。

**改动点**：

- [x] 新增 `web/src/utils/schemaHints.ts`：`derivePresentationSpec(schema): FormPresentationSpec`——遍历顶层 properties 提取 x-* hints 生成 fields（widget/label/placeholder/visibleWhen/group/order）
- [x] `web/src/pages/Functions/Invoke/index.tsx:110` 接入推导器（有 hints 时生成 fields，无 hints 保持现状 `layout: 'vertical'`）
- [x] JSON 模式与表单模式双向同步不受影响（现状 rawJson 联动保留）
- [x] 单测：`schemaHints.test.ts`（各 hint 提取、无 hints 回退、嵌套 properties 不误伤）

> 已知边界：嵌套 object 暂不展开为点路径字段（渲染器 dotted uiSchema 支持待 F7 后评估），嵌套容器上的 hints（x-widget/x-label 等）照常生效；docs/architecture/presentation-hints.md 推导规则已与实现同步。

**验收**：`pnpm --dir web run tsc` 0 错误；`pnpm --dir web test` 全绿。

## F3. widget 映射补全 · 第一批（Select 系）✅

**目标**：FormWidget 声明的 Select 系 widget 不再退化为 input。

**改动点**：

- [x] 新增 `SchemaFormRenderer/widgets.tsx` 自定义 RJSF widgets：TreeSelect（treeData 经 widgetProps 透传，单选 string/多选 string[]）、Cascader（cascaderOptions，值取最后一级）、Rate（number）；`customWidgets` 注册进 Form `widgets` prop
- [x] `widgetToRjsf`：Select→内置 select、TreeSelect/Cascader/Rate→自定义 widget 名；MultiSelect 补 `ui:options.multiple`（原缺失导致退化为单选）
- [x] 单测：`__tests__/widgets-select.test.tsx`（uiSchema 映射 3 例 + 渲染/选值/多选/级联/Rate 4 例）

**验收**：tsc 0 错误 + web test 全绿（161 passed）+ guard PASSED。

## F4. widget 映射补全 · 第二批（上传/键值对）✅

**目标**：Upload/ImageUpload/FileUpload/KeyValue 可用。

**改动点**：

- [x] Upload 系 widget（`widgets-upload.tsx`）：antd Upload，值为 URL string/string[]（schema.type 驱动），done 文件取 `file.url ?? response?.url`，action/accept/maxCount/listType 经 widgetProps 透传；`uploadValueFromFileList` 导出供测试
- [x] KeyValue：键值对编辑器，值为 object——object 类型 rjsf 走 ObjectField 忽略 ui:widget，故注册为自定义 field（`ui:field: 'keyValue'`）；空 key 行不计入提交
- [x] SchemaFormRenderer 加固：handler useCallback 稳定化（rjsf 任意 props 变化会重置内部 state 为 formData，引用不稳定 + 镜像滞后会回滚用户输入）；getValues 优先读 Form 实时 state
- [x] 单测：上传值归一（done/uploading/error、response 兜底）、KeyValue 增删改行、Upload 移除清值

> 已知边界：Upload 需游戏方在 x-widget-props 配置 action 上传端点，无 action/失败不计值；free-form object 字段建议声明 `additionalProperties: true`，否则 rjsf omitExtraData 会剪除空内容对象。

**验收**：tsc 0 错误 + web test 全绿（166 passed）+ guard PASSED。

## F5. 表单 label 兜底与人性化 ✅

**目标**：字段 title 缺失时不裸奔英文 key。

**改动点**：

- [x] `deriveRuntimeSchema`（SchemaFormRenderer/index.tsx）：title 缺失时 fallback——x-label hint > schema.title > key 人性化（`web/src/utils/humanize.ts`，全量覆盖 spec.fields 之外的字段）
- [x] description 缺失时 x-description hint 兜底（F2 推导器已映射 `FormFieldSpec.description`）
- [x] 单测：三种来源优先级（presentation-adapter.test.tsx）

**验收**：tsc + web test 全绿（149 passed）。

## F6. AJV 校验错误消息中文化 ✅

**目标**：表单校验错误直接可读。

**改动点**：

- [x] `SchemaFormRenderer` 传入 `transformErrors`（`localizeFormErrors`）：AJV 关键字（required/minLength/maxLength/minimum/maximum/pattern/format/type/enum/oneOf/anyOf/const）→ 本地化模板（含字段 title 插值，沿 property 路径解析嵌套 title）
- [x] 中英文案跟随平台 locale（`getLocale()`，umi 运行时外回退 zh-CN）
- [x] 单测：required/格式/enum/嵌套 title 五类断言（error-localization.test.ts）；既有 game-schema 断言同步为本地化文案

**验收**：tsc 0 错误 + web test 全绿（154 passed）+ guard PASSED。

## F-B 阶段：联动与布局（P1）

## F7. FormGroupSpec 接线：分组渲染 ✅

**目标**：`types/dashboard.ts:683` FormGroupSpec 从定义变为实际渲染。

**改动点**：

- [x] 自定义根级 `ObjectFieldTemplate`（`templates.tsx`）：分组渲染 antd Card（LocalizedText 标题），collapsible 分组用 antd Collapse（v6 Card 无 collapsible）；未分组字段置顶；无分组/无宽度且非根级时委托 antd 默认模板
- [x] 每字段 `width`（FormFieldSpec 既有字段，对应 x-width 1-12）→ Col 栅格宽度，优先级 width > formContext.colSpan > antd 默认
- [x] `deriveRuntimeSchema` 向 formContext 注入 `__groups/__fieldGroups/__fieldWidths`
- [x] 单测：元数据注入、Card 归属、Collapse 展开、width 生效、无分组回退

**验收**：tsc 0 错误 + web test 全绿（172 passed）+ guard PASSED + docs build 通过。

## F8. visibleWhen 隐藏策略改进：不丢值 ⬜

**目标**：条件隐藏切换时保留用户已填数据，避免来回切换丢输入。

**改动点**：

- [ ] `deriveRuntimeSchema`（index.tsx:278-287）：隐藏字段不再从 schema.properties 删除，改 `ui:widget: 'hidden'` + 从 required 移除（校验豁免）
- [ ] 确认提交 payload 是否含隐藏字段值：`omitExtraData` 行为验证，必要时提交前按可见性过滤
- [ ] 单测：隐藏→显示值保留、隐藏字段不参与校验

**验收**：tsc + web test 全绿。

## F9. 远程选项源（x-options-source）⬜

**目标**：选择玩家/角色等高频输入从「手输 id」变为「下拉搜索」。

**改动点**：

- [ ] FormFieldSpec 扩展 `remoteOptions: { functionId, labelPath?, valuePath?, searchParam? }`；x-options-source hint 提取进 F2 推导器
- [ ] 新增 hook `useRemoteOptions`：表单字段失焦/打开下拉时 `invokeFunction(functionId, …)` 拉取（labelPath/valuePath 取值），会话级 Map 缓存（key=functionId+params），失败静默降级为手输
- [ ] F3 的 Select/TreeSelect widget 接入该 hook
- [ ] 权限天然走 RBAC（调用需权限，无权限时提示不可用）
- [ ] 单测：mock invokeFunction——选项映射、缓存命中、失败降级

**验收**：tsc + web test 全绿；demo 包（examples）配一个带 x-options-source 的示例函数验证端到端。

## F-C 阶段：结果渲染与调用 UX（P1）

## F10. outputSchema 驱动结果渲染 ⬜

**目标**：调用结果不再是裸 JSON，优先结构化展示。

**改动点**：

- [ ] 新增 `deriveResultSpec(outputSchema)`：对象 → 字段卡片（复用 `web/src/components/PageRenderer/ResultViewRenderer.tsx`）；对象数组 → antd Table（列=properties）；其余/解析失败 → 现有 JSON viewer 兜底
- [ ] `InvocationResponse.tsx` 加「结构化 / JSON」Tabs，结构化优先展示
- [ ] 错误响应 `details` 结构化渲染（字段级错误列表，对接后端 error 契约 details）
- [ ] 单测：deriveResultSpec 三分支、details 渲染

**验收**：tsc + web test 全绿。

## F11. 异步任务进度内嵌 ⬜

**目标**：async 调用后在工作台内直接看任务进度，不再只弹 taskId。

**改动点**：

- [ ] `Invoke/index.tsx` asyncMode 成功后：内嵌任务面板（taskId → 轮询或复用既有任务事件流通道，见 `internal/jobs` 与 task API；SSE 通道已有则优先复用）
- [ ] 展示状态机（pending/running/done/error）+ 进度/日志尾部 + 完成后结果展示（对接 F10 渲染）
- [ ] 取消按钮（对接既有 CancelTask）
- [ ] 单测：状态轮询/事件流 mock、完成渲染、取消交互

**验收**：tsc + web test 全绿；demo task 函数端到端可见进度。

## F-D 阶段：契约演进保护（P2）

## F12. 注册时 schema 兼容性 diff ⬜

**目标**：schema 破坏性变更不被静默覆盖。

**改动点**：

- [ ] 新增 `internal/function/schemadiff`（或 internal/api/openapi 旁）：JSON Schema 语义 diff——新增 required、删除已有 properties、类型变更、enum 收窄 = breaking；新增可选字段/描述变更 = compatible
- [ ] `internal/server/control_handler.go` `handleRegisterRequest`（:429）upsert contract 前对比旧 `input_schema`/`output_schema`：breaking 时写入 contract `Diagnostics`（字段已有）+ `RegisterResponse.warnings` 返回 agent
- [ ] 不阻断注册（只告警），策略开关 `functions.schemaDiffWarn`（默认开）
- [ ] 单测：diff 五类判定、upsert 写 Diagnostics、warnings 透传

**验收**：`go test ./internal/function/... ./internal/server/...` 全绿；`go build ./...` 通过。

## F13. 契约变更可视化（依赖 F12）⬜

**目标**：运营在 Functions 页能看到契约近期变更与告警。

**改动点**：

- [ ] Functions 详情页（`web/src/pages/Functions/Detail.tsx`）展示 Diagnostics 告警条 + `SourceDigest`/`UpdatedAt` 变更提示
- [ ] contract 变更写入审计链（upsert 处加审计事件 `function.contract_updated`，含 diff 摘要）
- [ ] 单测：告警条渲染（mock diagnostics）、审计事件断言

**验收**：tsc + web test 全绿；`go test ./internal/server/...` 全绿。

## F14.（可选）SDK 便捷层注入 hints ⬜

**目标**：游戏方在 SDK 侧低成本声明呈现意图。

**改动点**：

- [ ] Go SDK `function/builder.go`：`SetFieldHint(key, hintKey, value)` / `SetFieldWidget(key, widget)` ——向 InputSchema 的 properties[key] 合并 x-* 字段
- [ ] Python/JS SDK 对齐（按 `sdks/SDK_FEATURE_MATRIX.md` 标注为 L2 可选能力）
- [ ] `sdks/SDK_FEATURE_MATRIX.md` 与 `docs/sdks/<lang>/` 同步
- [ ] Go SDK 单测：hints 合并/覆盖/非法 key 拒绝

**验收**：`go test ./sdks/...`（模块内）全绿；examples 示例更新展示 hints 用法。

## 执行顺序建议（F 系列）

F1（规范先行）→ F2（推导器，后续任务的接线点）→ F5/F6（独立小项，可穿插）→ F3 → F4 → F7/F8 → F9（依赖 F3）→ F10/F11 → F12 → F13（依赖 F12）→ F14（可选）。

F2 是汇聚点：F3-F9 的成果都通过它进入 Invoke 工作台。每个任务独立提交；涉及 web 的按交付 DoD 跑 `pnpm --dir web run tsc` + `pnpm --dir web test` + `bash scripts/dashboard_vnext_guard.sh`，涉及文档的 `cd docs && pnpm build`。
