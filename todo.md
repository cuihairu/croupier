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
> 已知边界：token 撤销最长 30s 生效延迟（缓存 TTL）；`docs/security.md` 已记录。
> MFA 前端二次输入 UI（原 T3 边界）已于 2026-09-02 补齐：登录页拦截 `401 + error=mfa_required`，展示动态验证码输入并以 `totpCode` 重试（`web/src/pages/User/Login/`）。

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

> **状态：F1-F15 全部完成（2026-09-02）**——F-A/F-B/F-C 前端呈现层与调用 UX、F-D 契约演进保护、SDK 便捷层与 provider 侧入站校验均已落地；已知边界见各任务小节。

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

## F8. visibleWhen 隐藏策略改进：不丢值 ✅

**目标**：条件隐藏切换时保留用户已填数据，避免来回切换丢输入。

**改动点**：

- [x] `deriveRuntimeSchema`：隐藏字段不再从 schema.properties 删除，改 `ui:widget: 'hidden'`（rjsf 从 uiSchema uiOptions 读 hidden）+ 从 required 移除（校验豁免）；返回 `hiddenKeys`
- [x] 提交 payload 剔除隐藏字段（`handleSubmitEvent` 按 hiddenKeysRef 过滤）——表单内保值可恢复，但不可见的值不提交
- [x] 单测：保值/required 摘除、可见性往返恢复、提交剔除、隐藏必填不阻断（4 例）

**验收**：tsc 0 错误 + web test 全绿（176 passed）+ guard PASSED。

## F9. 远程选项源（x-options-source）✅

**目标**：选择玩家/角色等高频输入从「手输 id」变为「下拉搜索」。

**改动点**：

- [x] `FormFieldSpec` 扩展 `remoteOptions`（types/dashboard.ts `RemoteOptionsSpec`）；`asRemoteOptions` 提取进 F2 推导器（缺 functionId 忽略）
- [x] 新增 hook `useRemoteOptions`：会话级缓存（functionId+search），失败静默降级为空选项（widget 退化为普通输入），`selectByPointer` 支持 `*` 通配数组段
- [x] Select（`RemoteSelectWidget` 覆盖内置 `select`，非远程委托内置）/TreeSelect（`RemoteTreeSelectBody`）接入；searchParam 声明时下拉搜索重新调用
- [x] 权限天然走 RBAC（无权限调用失败 → 降级手输）
- [x] 单测 6 例：通配取值、label/value 映射兜底、缓存命中、失败降级、searchParam 重调、Select 集成（走真实 hints 推导链）+ F2 测试同步为新语义

> 已知边界：valuePath 缺省复用 labelPath、选项 label 缺省复用 value；远程选项在 TreeSelect 中为平铺列表（无层级）。

**验收**：tsc + web test 全绿（182 passed）+ guard PASSED + demo `player.ban` 配置 x-options-source 端到端示例 + docs build 通过。

## F-C 阶段：结果渲染与调用 UX（P1）

## F10. outputSchema 驱动结果渲染 ✅

**目标**：调用结果不再是裸 JSON，优先结构化展示。

**改动点**：

- [x] 新增 `web/src/utils/resultSpec.ts`：`deriveResultSpec(outputSchema)`——object+properties → 字段卡片（title 缺失人性化兜底，LocalizedText 形态）；array+items.properties → 表格列；标量/解析失败 → undefined（JSON 兜底）
- [x] `InvocationResponse.tsx`：结构化 Tab 优先（对象 → Descriptions、对象数组 → antd Table，复用 PageRenderer 的 `renderJSONValueSummary`），JSON/原始 Tab 保留兜底
- [x] 错误响应 `details` 结构化渲染（对接 `extractErrorDetails`，字段级错误列表）
- [x] Invoke 页接线：`resolveOutputSchema`（string/object 双形态解析）、errorDetails 状态维护
- [x] 单测 8 例：规格推导三分支、isArrayOfObjects、结构化卡片/表格/兜底、错误明细

**验收**：tsc 0 错误 + web test 全绿（190 passed）+ guard PASSED。

## F11. 异步任务进度内嵌 ✅

**目标**：async 调用后在工作台内直接看任务进度，不再只弹 taskId。

**改动点**：

- [x] 新增 `TaskProgressPanel.tsx`：taskId → 轮询 `GET /api/v1/tasks/:id`（2s 间隔，终态停止），展示状态机标签（queued/dispatching/running/succeeded/failed/cancelled/timed_out 中文化）
- [x] succeeded 时 `onCompleted(payload)` 交回工作台，结果进入 F10 结构化渲染；failed/timed_out 展示错误详情
- [x] 取消按钮（对接 `cancelTask`）+ 手动刷新；终态后收起操作
- [x] 单测 5 例：轮询状态机与终态停摆、失败展示、取消调用、终态收起、单次轮询失败自恢复

**验收**：tsc 0 错误 + web test 全绿（195 passed）+ guard PASSED。

## F-D 阶段：契约演进保护（P2）

## F12. 注册时 schema 兼容性 diff ✅

**目标**：schema 破坏性变更不被静默覆盖。

**改动点**：

- [x] 新增 `internal/function/schemadiff`：JSON Schema 语义 diff——新增 required、删除已有 properties、schema type/结构类型变更、enum 扩张 = breaking；新增可选字段/描述标题增改 = compatible；首注（旧空）/非法 JSON 不产差异
- [x] `ContractService.RebuildContractFromFunctionMeta`：upsert 前对比库中现有契约 input/output schema，breaking 追加 `schema_breaking_change` Diagnostics（SDK/OpenAPI 来源统一覆盖）
- [x] `ControlService.handleRegisterRequest`：与会话中上次注册 schema 对比，breaking 写入注册警告（code=`schema_breaking_change`）+ `RegisterResponse.warnings` 返回 agent；在 UpsertAgent 覆盖会话前执行
- [x] 开关 `descriptors.schemaDiffWarn`（默认开，`*bool` 三态），`cmd/server/root.go` 接线
- [x] 单测：diff 八类判定（required/删除/类型/enum/兼容/嵌套/首注/source 标签）、service Diagnostics 合并、handler 警告透传与开关抑制

**验收**：`go build ./...` + `go vet` 通过；`go test ./internal/function/... ./internal/service/ ./internal/server/` 全绿。

## F13. 契约变更可视化（依赖 F12）✅

**目标**：运营在 Functions 页能看到契约近期变更与告警。

**改动点**：

- [x] Functions 详情页（Detail.tsx + useFunctionDetailPage）展示 Diagnostics 告警条（code/message/field）；后端 `FunctionSpecFromContract` 投影 `contract.Diagnostics` → descriptors API 自动携带
- [x] contract 变更写入审计链：`function.contract_updated` 事件（ContractService `WithAuditService`，含 breaking diff 摘要/来源/scope），router 装配（SQL store，失败降级无审计）
- [x] 单测：Go 投影透传 + 审计事件断言（首注不写/更新写入/breaking 摘要）；web 告警条渲染

**验收**：`go build ./...` + `go vet` 通过；`go test ./internal/service/ ./internal/server/ ./internal/audit/` 全绿；web tsc 0 错误 + 198 passed + guard PASSED。

## F14.（可选）SDK 便捷层注入 hints ✅

**目标**：游戏方在 SDK 侧低成本声明呈现意图。

**改动点**：

- [x] Go SDK `function/builder.go`：`SetFieldHint(field, hint, value)` / `SetFieldWidget(field, widget)`——向 InputSchema properties[field] 合并 x-* hint（空 schema 建骨架、重复覆盖、x_ 归一 x-、非法键入 builder errors）
- [x] JS SDK `setFieldHint`/`setFieldWidget`；Python SDK `set_field_hint`/`set_field_widget`（不可变风格返回 descriptor）
- [x] `sdks/SDK_FEATURE_MATRIX.md` L2 新增行（Go/Python/JS ✅，Java/C++/C# ❌）；`docs/sdks/{go,python,js}/index.md` 同步示例
- [x] 单测：Go 8 例（合并/骨架/覆盖/归一/三类拒绝/非法 schema）、JS 5 例、Python 6 例

**验收**：`go test ./sdks/go/function/` + `js jest`（357 passed）+ `python pytest` 全绿；docs build 通过。

## F15. JSON Schema 入站 payload 校验（provider 侧）✅

**目标**：补齐 SDK 矩阵缺口——SDK 收到 InvokeRequest 后、派发用户 handler 前，按函数声明的 input schema 校验 payload（服务端仍是权威校验方）。

**改动点**：

- [x] Go：`ClientConfig.ValidateInputPayloads`（默认关）；`TCPManager.validateInboundPayload`（Draft7 编译，非法 schema/payload JSON 跳过不阻断），invoke 失败回 `{"error":"payload validation failed: …"}`、startTask 返回错误；8 例单测
- [x] JS：`ClientConfig.validateInputPayloads`（默认关）；Ajv 编译缓存，invokeInbound/handleInboundStartTask 接入；4 例单测
- [x] Python：`ClientConfig.validate_input_payloads`（默认关）；jsonschema 校验进 `invoke`/`start_task`；6 例单测
- [x] `sdks/SDK_FEATURE_MATRIX.md` 更新为 Go/Python/JS ✅（Java/C++ ❌）

**验收**：go build/vet/test、js tsc + jest、python pytest 全绿；`check-sdk-matrix.sh` 通过。

## 执行顺序建议（F 系列）

F1（规范先行）→ F2（推导器，后续任务的接线点）→ F5/F6（独立小项，可穿插）→ F3 → F4 → F7/F8 → F9（依赖 F3）→ F10/F11 → F12 → F13（依赖 F12）→ F14（可选）。

F2 是汇聚点：F3-F9 的成果都通过它进入 Invoke 工作台。每个任务独立提交；涉及 web 的按交付 DoD 跑 `pnpm --dir web run tsc` + `pnpm --dir web test` + `bash scripts/dashboard_vnext_guard.sh`，涉及文档的 `cd docs && pnpm build`。

---

# A 阶段：审批可见性（申请人视角，P1）—— ✅ 全部完成（2026-09-04，A1→A5）

背景：高风险执行进入审批后，申请人几乎无法追踪自己的申请——Invoke 页不消费 approvalId、审批中心无「我发起的」视图、审批创建不通知申请人。调研结论（2026-09-04）：

- 存储层已支持按 actor 过滤（`internal/platform/approvals/sql_store.go:133`），仅 API/前端未暴露
- 前端 `/approvals` 发送的过滤参数与后端 `ApprovalsListRequest`（仅 `page/pageSize/status`，`internal/api/approval/dto.go:35`）参数名漂移，状态/操作者过滤实际不生效
- Console 页面执行有「等待审批 + 刷新」入口（`OperationPageRenderer.tsx:265`）；Invoke 页对 `approvalId` 零处理且误弹「调用成功」
- approve/reject 无「审批人 ≠ 申请人」校验（`internal/api/approval/service.go:131`）
- 审批 created 事件只通知 admin（`internal/api/function/helpers.go:634`），完成事件才含申请人（`internal/api/approval/notify.go:29`）

## A1. 审批列表 API 补 actor 过滤 + 修复参数契约漂移 ✅

**目标**：审批列表支持服务端 actor 过滤，前端既有过滤参数真实生效。

**改动点**：

- [x] `internal/api/approval/dto.go` `ApprovalsListRequest` 增加 `actor`、`functionId`、`gameId`、`env` 绑定（与前端既有发送参数对齐）；`state` 别名兼容或前端改发 `status`（二选一，禁止双读并存，按 CLAUDE.md 兼容规则处理）
- [x] `internal/api/approval/service.go` 列表查询透传 Filter（存储层 `Filter.Actor` 已支持，接线即可）
- [x] 新增 `mine=true` 参数：服务端忽略请求中的 actor、强制 `actor = 当前登录用户`（防止越权枚举他人申请）
- [x] 单测：actor 过滤命中/不命中、mine=true 强制覆盖 actor、参数漂移修复后前端契约回归

**验收**：`go test ./internal/api/approval/...` 全绿；curl 带各过滤参数返回过滤后结果。

## A2. 两人规则：禁止申请人自批 ✅

**目标**：消除申请人自行通过自己申请的合规缺口。

**改动点**：

- [x] `internal/api/approval/service.go` `Approve`/`Reject` 校验操作者 ≠ `record.Actor`，违反返回 403 `self_approval_forbidden`
- [x] 逃生门配置 `approval.allowSelfApprove`（默认 false；单管理员环境可显式打开），配置示例更新
- [x] 自批拦截写审计 `approval.self_rejected`（复用哈希链审计）
- [x] 单测：自批 403、开启逃生门后放行、他人正常审批不受影响

**验收**：`go test ./internal/api/approval/...` 全绿；审计事件可查。

## A3. 审批中心「我发起的」视图 + 过滤修复（依赖 A1）✅

**目标**：申请人一站式看到自己的申请（审批中/已通过/已拒绝）。

**改动点**：

- [x] `web/src/pages/Approvals/index.tsx` 增加 Tabs：待我审批 / 我发起的 / 全部；「我发起的」带 `mine=true` 且隐藏通过/拒绝操作列
- [x] 前端过滤参数与 A1 新契约对齐（状态/函数/游戏/环境过滤真实生效，删除仅前端补过滤的 `filtered` 逻辑）
- [x] Drawer 详情支持 deep-link：`/approvals?approvalId=xxx` 打开对应申请
- [x] web tsc + 既有 Approvals 测试更新 + guard

**验收**：`pnpm --dir web run tsc` + `pnpm --dir web test` 全绿；`bash scripts/dashboard_vnext_guard.sh` PASSED。

## A4. Invoke 页审批中状态（消费 approvalId）✅

**目标**：Invoke 调试页触发审批后，申请人原地看到状态并可跟进。

**改动点**：

- [x] `web/src/pages/Functions/Invoke/index.tsx`：`approvalRequired=true` 时展示「审批中」Alert（含 approvalId），修正误弹「调用成功」的 toast
- [x] 轮询 `GET /api/v1/approvals/:id`（复用 `web/src/services/console.ts:148` `queryApprovalStatus` 模式，10s 间隔可停）
- [x] 审批通过后提示并提供「重新调用」（携原参数一键重发）；拒绝后展示 reason
- [x] 单测：approvalRequired 渲染、轮询状态机（pending→approved/rejected 停止）

**验收**：web tsc + test 全绿。

## A5. 审批事件通知申请人 ✅

**目标**：审批创建/完成申请人全程有感知。

**改动点**：

- [x] `internal/api/function/helpers.go` 与 `internal/api/console/service.go` 的 `approval.created` 通知接收人加入 `record.Actor`（与完成事件 `notify.go:29` 对齐）
- [x] 站内信 Data 携带 `approvalId`；`web/src/components/MessagesBell.tsx` / Profile 收件箱点击跳转 `/approvals?approvalId=xxx`（依赖 A3 deep-link）
- [x] 单测：created 通知含申请人、完成通知不回归

**验收**：`go test ./internal/api/approval/... ./internal/api/function/... ./internal/api/console/...` 相关用例全绿。

# R 阶段：执行留痕与保留期（P1）—— ✅ 全部完成（2026-09-04，R1→R5）

背景：同步执行的请求体/响应体完全不落库（审计仅元数据，且 `function.invoke` 受 `require_audit` 开关默认关）；`task_runs.input_payload/result_payload` 反而完整落库却永久累积无清理；`audit_records` 的 `DeleteBefore`/`Archive` 是死代码。目标：payload 级留痕可查、保留期可配，审计链永久保留不参与清理。

## R1. execution_logs 表 + REST invoke 写入 ✅

**目标**：同步函数调用的请求/响应落库，payload 级事后可查。

**改动点**：

- [x] 迁移：`execution_logs` 表——`id/game_id/env/source/function_id/page_key/binding_id/actor/route/status/duration_ms/trace_id/request_payload/response_body/truncated/created_at`，索引 `(game_id, env, created_at)` + `(actor, created_at)` + `(function_id, created_at)`
- [x] 写入点：`internal/api/function/helpers.go` invoke 成功/失败后**异步**写入（带缓冲 channel，丢写只告警不阻断主路径）；响应体只存 JSON 类型结果
- [x] 脱敏：复用 `internal/audit` `maskSensitiveData` 清单；单条 payload 上限（默认 64KB，超出截断置 `truncated=true`）
- [x] 开关与上限配置 `executionLog.enabled`（默认 true）/ `maxPayloadBytes`
- [x] 单测：成功/失败写入、脱敏命中、截断、开关关闭不写、异步失败不影响调用

**验收**：`go test ./internal/api/function/...` 全绿；invoke 后表内有记录且敏感字段已掩码。

## R2. 页面绑定执行写入 execution_logs ✅

**目标**：Console 页面执行与 REST 调用同一留痕（`source=page`）。

**改动点**：

- [x] `internal/api/console/service.go` execute 路径接入同一异步写入器（复用 R1 writer），记录 page_key/binding_id/publish_version
- [x] 单测：页面执行写入、审批类执行（kind=approval）不写（审批走 approvals 表已有 Payload）

**验收**：`go test ./internal/api/console/...` 全绿。

## R3. 保留期配置 + 清理循环（含 task_runs 纳管） ✅

**目标**：payload 类数据默认 7 天过期，可配置；审计链永久保留不动。

**改动点**：

- [x] 配置 `executionLog.retentionDays`（默认 7，0=永久）；`taskLog.retentionDays`（默认 7，0=永久）——`internal/config/config.go` + `configs/server.yaml` 示例
- [x] `internal/model` 增加 `ExecutionLogModel.DeleteBefore` / `TaskRunModel.DeleteBefore`（含 task_events 级联）；`audit_records` 明确排除在清理外（哈希链完整性，代码注释 + 文档声明）
- [x] 清理循环：复用 `internal/server/control_handler.go:195` 指标清理的每小时模式，删除分批（防长事务）
- [x] 单测：过期删除、0 天不删、audit 表不受影响、分批边界

**验收**：`go test ./internal/model/... ./internal/server/...` 全绿。

## R4. 「我的调用记录」查询 API ✅

**目标**：给前端提供带权限边界的执行留痕查询。

**改动点**：

- [x] `GET /api/v1/execution-logs`：分页 + 过滤（function_id/status/时间范围）+ `mine=true` 强制 actor=当前用户；非 mine 需 `audit:read` 权限（与审批中心同级）
- [x] 响应脱敏沿用存储时掩码（不二次处理）；`GET /:id` 详情（mine 或有权限）
- [x] 单测：权限边界（普通用户仅 mine）、scope 隔离（game/env）、分页

**验收**：`go test ./internal/api/...` 新增模块用例全绿。

## R5. 前端「我的调用记录」（依赖 A3/R4） ✅

**目标**：申请人在 UI 里查自己的执行历史与请求/响应，替代仅本地的 localStorage 历史。

**改动点**：

- [x] Invoke 页历史区改造：本地 50 条之外提供「服务端记录」入口（mine 视图，展示请求/响应 Drawer）
- [x] 审批中心「我发起的」Tab 行内可跳转对应调用记录（approvalId 关联 trace 关联的留痕，可按 trace_id 串联）
- [x] web tsc + test + guard

**验收**：交付 DoD 全项（tsc/test/guard）；涉及页面发布的按 DoD 走一次 accept-and-publish 线上验证。

## 执行顺序建议（A/R 系列）

A1（API 契约先行）→ A2（独立可并行）→ A3（依赖 A1）→ A4（独立，可穿插）→ A5（依赖 A3 deep-link）；R1（表+writer）→ R2（复用 writer）→ R3（清理循环）→ R4（查询 API）→ R5（依赖 A3/R4）。

A 系列与 R 系列互相独立可并行。每个任务独立提交；涉及 web 的按交付 DoD 跑 `pnpm --dir web run tsc` + `pnpm --dir web test` + `bash scripts/dashboard_vnext_guard.sh`，涉及文档的 `cd docs && pnpm build`。

---

# U 阶段：UI 生成规则与组件模型补强（2026-09-06 全链路分析）

背景（2026-09-06 UI 生成规则走查结论，覆盖 descriptor→generator→PageSpec→组件模板→组合页编辑器全链路）：

- **契约违规**：`generateRepairHint`（`internal/dashboard/generator/generator.go:1380`）以变量 locale 为 key 构造单语言 `LocalizedText`，违反「key 必须为 `zh-CN`/`en-US` 字面量」契约；同时 `dashboard-page-model.md` 定义 `repairHint: string` 与代码 `spec.LocalizedText`（`types.go:803`）类型漂移
- **文档滞后**：composition-model.md 仍标「显式参数映射 UI 未暴露」，但 `ParamMappingEditor.tsx` + `compiler.ts:273` 已落地；ComponentLibrary 空态提示「右键保存」，实际唯一入口是顶栏按钮
- **双端能力不对称**：`x-options-source` 前端已消费（`useRemoteOptions.ts`），Go spec 侧无 `RemoteOptionsSpec`，发布页退化为普通控件——违反 presentation-hints.md「调试页与发布页表现一致」自身约束
- **组件模型缺口**（composition-model.md 已承认）：区块实例命名空间（P0）、模板级参数化（P0）、跨模板联动悬空无提示；transform 仅 `pick`、失败策略未定义
- 已核实不是问题：builtin 模板 stale 徽标闭环完整（`computeStaleKeys` + 前端展示），不列入

每个任务原子：独立实现、独立测试、独立提交，互相之间无代码依赖。

## U1. generateRepairHint LocalizedText 契约修复 + 文档类型对齐 ✅

**目标**：修复契约违规，消除 BlockedProposalIssue.repairHint 的文档/代码类型漂移。

**改动点**：

- [x] `internal/dashboard/generator/generator.go` `generateRepairHint`：改为返回双语 `spec.LocalizedText{"zh-CN": ..., "en-US": ...}` 字面量 key（中文/英文按现有文案补齐），删除 `{locale: ...}` 变量 key 写法与 locale 参数（`CreateBlockedProposalIssue` 签名同步，唯一调用方 `contract_service.go` 更新）
- [x] `docs/architecture/dashboard-page-model.md` `BlockedProposalIssue.repairHint: string` → `repairHint: LocalizedText`（以代码 `spec/types.go` 为准）
- [x] `generator_test.go` 断言更新：返回值同时含 `zh-CN`/`en-US` 且恰好 2 key
- [x] `cd docs && pnpm build` 通过

**验收**：`go test ./internal/dashboard/... ./internal/service/` 全绿。
已核实：生成器其余 31 处 `spec.LocalizedText{locale: ...}` 为「系统默认语言单条目」模式（`DefaultLocale` 恒为 `"zh-CN"` 合法 BCP47，ui-generation.md「生成器只保证系统默认语言」明文承认），**不是违规**，不在本任务范围。

## U2. composition-model.md 同步显式参数映射现状 ✅

**目标**：文档反映 ParamMappingEditor 已落地的事实，并补入悬空联动缺口（为 U7 留锚点）。

**改动点**：

- [x] React 原语映射表「props 传入」行：`⚠️ 半自动：显式参数映射 UI 未暴露` → `✅ 同名隐式合并 + 显式参数映射（ParamMappingEditor）`
- [x] 缺口表 P0 行「显式参数映射 UI（暴露 SelectorAST）」移除，替换为「模板级参数化（U6）」；补 P1「跨模板联动断链提示（U7）」
- [x] 组合四轴「数据」轴描述同步（显式映射已落地，P0 仅剩区块实例命名空间）
- [x] `cd docs && pnpm build` 通过

**验收**：文档表格与 `ParamMappingEditor.tsx`/`compiler.ts:273` 实现一致，无「未暴露」表述残留。

## U3. ComponentLibrary 空态提示文案修正 ✅

**目标**：消除「右键保存为组件」的错误引导（实际唯一入口是顶栏「保存为组件(N)」按钮）。

**改动点**：

- [x] `web/src/pages/PageStudio/CompositeEditor/ComponentLibrary.tsx` 空态文案改为「选中画布多个节点 → 顶栏「保存为组件」可创建」（与 composite-editor-v4-design.md §3.3 一致）
- [x] 全仓无其他「右键保存」误导文案（Canvas 右键菜单为上移/下移，语义正确）

**验收**：`pnpm --dir web run tsc` + `pnpm --dir web test` 全绿 + guard PASSED。

## U4. x-options-source 服务端派生补齐（RemoteOptionsSpec）

**目标**：发布页表单与调试页一致地消费远程选项源，消除「调试页有下拉、发布页手输」漂移。

**改动点**：

- [ ] `internal/dashboard/spec/form_presentation.go`：`FormFieldSpec` 增加 `remoteOptions *RemoteOptionsSpec`（functionId/labelPath/valuePath/searchParam，lowerCamelCase 契约键）
- [ ] `internal/dashboard/generator/form_hints.go`：解析 `x-options-source`（functionId 缺失静默忽略，与前端 `asRemoteOptions` 一致）；`buildFormFields` 下发
- [ ] `web/src/types/dashboard.ts` 同步类型；`SchemaFormRenderer` 消费 spec 侧 `remoteOptions`（复用 `useRemoteOptions` 既有逻辑，与 hints 推导路径合流）
- [ ] 单测：Go 侧 hint 解析/忽略分支 + golden；web 侧 spec 驱动的远程选项渲染
- [ ] `docs/architecture/presentation-hints.md` 删除「暂不参与服务端派生」边界说明；`pagespec-protocol.md` FormFieldSpec 表补 `remoteOptions`
- [ ] 按 DoD 走一次 accept-and-publish 线上验证（发布页含远程选项 spec 字段落库）

**验收**：同一 `x-options-source` 函数在 Invoke 页与发布 OperationPage 表现一致；三层文档同步 + docs build 通过。

## U5. 区块实例命名空间（P0） ✅

**目标**：同一组件/函数拖入多次，实例间 page_state 输出与联动引用不互相覆盖、不随增删排序漂移。

**实现说明**：实际方案优于原计划的「实例前缀」——采用 **`sectionKey` 声明固化**（节点携带稳定 key：编译声明优先、反编译写回、实例化清除）。走查中发现并修复三个连带缺陷：`inputAssignments` 回读恒等映射（显式参数映射 round-trip 丢失）、`staticForm.refreshOn` 编译不透传、`instantiateTemplate` 前向引用不重映射 + 嵌套对象污染模板缓存。

**改动点**：

- [x] `compiler.ts`：`compileTree` 两遍 key 分配（声明 `sectionKey` 优先，非法/重复回退 `fid`/`fid-N` + warning）；`emitStaticSection` 透传 `refreshOn`；导出 `SECTION_KEY_RE`
- [x] `compiler.ts`：`decompileToTree` 把 `section.key` 写回 `props.sectionKey`（fn*/staticForm，round-trip 固化）；`inputAssignments` 反查移到第二遍（完整 keyToNodeId → 真实上游节点 id，悬空来源 warning）
- [x] `ComponentLibrary.tsx`：`instantiateTemplate` 重写为两遍克隆（预分配全部新 id——修复前向引用；嵌套 action/rowActions 拷贝后重映射——修复模板缓存污染）；`sectionKey` 不随实例复制
- [x] `PropsPanel.tsx`：fn*/staticForm 增加「区块 key」编辑框（非法字符红框提示，留空自动分配）；`OutlinePanel.tsx` 声明 key 时显示「标题 (key)」
- [x] 单测 10 例新增：声明优先/冲突回退/非法忽略/删除首实例不漂移/round-trip key 逐项不变/inputAssignments 反查与再编译保留/staticForm key+refreshOn round-trip/悬空来源警告/实例化引用重映射/sectionKey 不复制（`compiler.test.ts`、`decompile.test.ts`、新建 `componentLibrary.test.ts`）
- [x] 文档：`composite-editor-v4-design.md` §3.5「区块 key 与多实例命名空间」；`composition-model.md` 缺口表该 P0 关闭；`dashboard-page-model.md`/`pagespec-protocol.md` key 行同步

**验收**：web tsc 0 错误 + 253 tests 全绿 + guard PASSED + docs build 通过。
已知边界：未重命名已发布页面的既有自动分配 key（回读一次即固化）；按 DoD 的 accept-and-publish 线上验证待下次部署窗口执行。

## U6. 组件模板参数化（props 默认值，P0） ✅

**目标**：组件实例化后不必逐节点手改，支持模板级批量配置。

**改动点**：

- [x] `model/component_template.go`：`Params JSON` 字段（JSON 列，无需迁移）；`internal/api/component/handler.go`：`TemplateParam`（key/label/nodeId/prop/default，lowerCamelCase）+ `validateTemplateParams`（key 非空唯一、nodeId 存在于 tree 含子树、prop ∈ 白名单 title/span/autoRun——执行类配置拒绝）+ Create/Update 校验 + DTO `params` 投影
- [x] `ComponentLibrary.tsx`：DTO `params` + `instantiateTemplate(tpl, paramValues?)` 参数应用（值覆盖白名单 prop，未填回退 default；与 U5 引用重映射共存）；带参数模板点击/拖入抛给父级弹窗（`onInsert([], tpl)`）
- [x] `index.tsx`：保存弹窗「参数化」Checkbox（`scanParamCandidates` 扫描选中子树候选，勾选生成定义，default=当前值）；拖入/点击带参数模板弹「配置组件参数」（title=Input/span=InputNumber/autoRun=Switch，default 预填）
- [x] `types.ts`：`ParamCandidate` + `scanParamCandidates` 纯函数（title 恒列出，span/autoRun 存在才列，容器/文本跳过，子树递归）
- [x] 单测：Go `params_test.go` 6 组（合法/空规范化/key 空与重复/nodeId 悬空含子树命中/白名单外拒绝/节点 id 收集）；web `componentLibrary.test.ts` 3 例（参数覆盖+default 回退+子树参数、sectionKey 不复制、无参模板不变）+ 候选扫描
- [x] 文档：`composite-editor-v4-design.md` §3.6「模板参数化」；`composition-model.md` 缺口表该 P0 关闭

**验收**：go test（component/model 全绿）+ web tsc 0 错误 + 256 tests 全绿 + guard PASSED。
已知边界：参数仅覆盖节点展示 props（不含函数输入参数默认值）；带参数模板拖拽落点语义退化为根级追加；accept-and-publish 线上验证待部署窗口。

## U7. 组件实例化跨模板联动悬空提示

**目标**：拖入组件后，与画布已有区块的联动断链可见、可接线，不再静默丢失。

**改动点**：

- [ ] `instantiateTemplate` 检出跨模板边界的悬空引用（`idMap.get(nid) ?? nid` 保留旧 id 的分支），返回悬空清单
- [ ] 编辑器拖入后 message/Modal 提示「N 处联动指向模板外区块，已断开」，并提供快捷重连（悬空项 → 画布区块下拉选择）
- [ ] composition-model.md 缺口表补「跨模板联动断链无提示」条目并在本任务关闭
- [ ] 单测：悬空检出清单、重连后引用更新

**验收**：拖入含外部联动的模板（手工构造）有明确提示；web tsc/test/guard 全绿。

## U8. transform 白名单扩展：rename / default（P1）

**目标**：字段名不一致的上下游不必逐参数手写显式映射。

**改动点**：

- [ ] `internal/dashboard/spec/selector_ast.go`：transform 增加 `rename`（映射表）与 `default`（缺省值）两种受控类型 + 校验器
- [ ] `web/src/types/dashboard.ts` 同步；ParamMappingEditor 增加映射方式选择（直接取值/改名/缺省）
- [ ] `compiler.ts` 编译/反编译覆盖新 transform
- [ ] 单测：Go 校验器 + web 编译 round-trip
- [ ] `docs/architecture/pagespec-protocol.md` Selector AST 节更新（当前仅 `pick`）

**验收**：上游 `uid` 可经 rename 直连下游 `player_id`；三层文档同步 + docs build。

## U9. refreshOn 级联失败策略（P1）

**目标**：上游区块执行失败时下游行为确定，不再未定义。

**改动点**：

- [ ] 定义三种策略：`clear`（清空下游数据）/ `keep`（保留上次结果）/ `pause`（停止本次级联，默认），写入 `CompositeSection.cascadePolicy`
- [ ] 运行时 `CompositeRenderer` 按策略执行；编辑器 DataPanel 提供策略选择
- [ ] 单测：三策略行为 + 缺省回退 pause
- [ ] `docs/architecture/dashboard-page-model.md` CompositeSection 字段表 + `pagespec-protocol.md` 补 `cascadePolicy`

**验收**：上游查询失败时下游表格按策略处理而非残留旧数据无提示；guard + docs build。

## U10. 区块级条件显示（P2，可选）

**目标**：支持按 page_state 条件显隐整个区块（当前仅表单内 visibleWhen）。

**改动点**：

- [ ] `CompositeSection` 增加 `visibleWhen: ConditionSpec`（复用受限表达式，仅读 page_state）
- [ ] 编辑器属性面板 + `CompositeRenderer` 渲染分支
- [ ] 单测 + `pagespec-protocol.md`/`dashboard-page-model.md` 字段表同步

**验收**：mode=批量 时隐藏单发区块；发布链 DoD。

## U11. 组件模板更新提醒（P2，可选）

**目标**：缓解复制语义的「快照漂移」——模板改进后，已实例化页面无从得知。

**改动点**：

- [ ] 保存组件时记录内容 digest；页面提案保存时记录所用模板快照 digest
- [ ] 模板更新后，编辑器打开旧页面时提示「以下模板有新版本，可对比/重新拖入」（只提示，不自动同步——保持复制语义）
- [ ] 单测：digest 比对 + 提示触发

**验收**：更新「资源管理」模板后，旧页面编辑时有更新提示；不改变已发布页面行为。

## 执行顺序建议（U 系列）

U1（契约红线，最小）→ U2/U3（文档/文案，可并行热身）→ U5（P0，命名空间先于参数化避免返工）→ U6（P0）→ U7 → U4（独立，可与 U5/U6 并行）→ U8/U9（P1）→ U10/U11（P2 可选）。

每个任务独立提交；涉及 web 的按交付 DoD 跑 `pnpm --dir web run tsc` + `pnpm --dir web test` + `bash scripts/dashboard_vnext_guard.sh`；涉及发布链的（U4/U5/U6/U8/U9/U10）须走 accept-and-publish 线上验证；涉及文档的 `cd docs && pnpm build`。
