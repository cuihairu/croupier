---
title: 上传即成页：契约与绑定正交化设计
icon: rocket
order: 10
category:
  - 系统架构
tag:
  - OpenAPI
  - Page Studio
  - 设计
---

# 上传即成页：契约与绑定正交化设计

> **状态**：Proposed -- 设计已评审待落地；原子任务拆分见根目录 `todo.md`。
> 前置阅读：[ProComponents 页面生成与运行时](./ui-generation.md)、[Dashboard Resource/Page 模型](./dashboard-page-model.md)、[OpenAPI / SDK Descriptor v2](./openapi-sdk-descriptor-v2.md)。

## 1. 问题陈述：为什么现在"传了文档却生成不了页面"

现有链路把**「接口长什么样」（契约）**与**「接口能不能执行」（运行时绑定）**耦合为前置条件：

```text
现状（耦合）：
  OpenAPI 上传 → operations
    → Binding 要求 functionId 已运行时注册，否则 400    ← 断点①
    → FunctionContract → 页面提案
  组件模板只能手动 POST /component-templates/regenerate  ← 断点②
  组合页强制 ≥2 函数区块，单组件页面被禁止                ← 断点③
  全部保存都必须走 提案→接受→发布 双重门禁               ← 断点④
```

后果：上传一份 OpenAPI 文档，一个组件、一个页面都不会产生；用户必须在 OpenAPISources / ComponentTemplates / PageStudio 收件箱 / CompositeEditor 四个页面之间手工接力，任何一步不知道要做就卡住。

根因不是实现缺失，而是模型错位：

- **绑定是契约的属性，不是契约的前提**。接口的 UI 形态（表单字段、表格列、动作）只依赖 schema，与"当前有没有 agent 在线"无关。
- **审核门禁应该按风险分级，而不是对所有保存一刀切**。提案→审核→发布是为"契约自动生成的页面"设计的治理手段，被无差别套在人工拖拽页面上。
- **≥2 区块约束**来源于"组合页 = Composite"的命名洁癖，而"一个表格就是一页"恰是运营后台最常见的形态。

## 2. 核心决策（不再变更）

- **D1 一切皆物料**：无论契约来自 SDK 注册、OpenAPI 上传还是手写，落库即生成组件物料（组件模板 + 函数面板条目）。UI 生成永不依赖执行就绪。
- **D2 绑定状态正交化**：FunctionContract 增加执行状态维度 `executionState: bound | unbound`。unbound 组件可拖入画布、可发布页面；运行时执行动作时服务端拒绝并返回结构化诊断（`executor_unbound`）。
- **D3 自动绑定**：运行时 agent/SDK 注册的函数与 unbound 契约按 `(game_id, env, function_id)` 匹配即自动置 bound，无需人工干预。
- **D4 上传即生成管线**：OpenAPI 上传是一个完整的生成动作，单请求内完成 解析 → 契约 → 组件模板 → 页面提案，响应携带生成摘要与编辑器直达入口。
- **D5 发布分级**：是否走提案审核由 env 级策略决定；dev 默认保存即发布（快照与版本历史保留），prod 保留提案→审核→发布。
- **D6 协议零改动**：PageSpec wire 契约、渲染器、权限/审计体系不变。本设计只改契约准入条件与流程编排。
- **D7 文案 i18n：默认名称必填、翻译可选**：菜单/页面标题等 LocalizedText 字段只要求**默认名称**（系统默认语言 `systemDefaultLocale`）非空；其余语言（含 en-US）一律是可选翻译。发布校验不再强制 zh-CN+en-US 双写（废除 `hasDefaultLocale` 的双 key 判定与 `ensureBilingual` 强制补写）；渲染端回退链不变（当前界面语言 → 系统默认语言 → 任一非空值）。编辑入口统一为 `LocalizedTextEditor` 组件（已是仓库唯一实现）：语言选项来自全局支持列表，用户按需录入翻译，缺失任何非默认语言不警告、不阻断。
  - **落地状态**：T12 已落地——后端三处 `hasDefaultLocale` 统一为「任一 locale 非空即过」（全空白仍拒），生成器改名 `ensureDefaultLocale` 并废除 en-US 强制补写，前端 `REQUIRED_LOCALES` 降级为 `['zh-CN']`（编辑器警告随之只对默认语言缺失生效）。T13 待做：组件 `missingRequired` 警告、下拉 ⚠ 标记与 `contractHint` 双必填文案的彻底移除。`normalizeLocalizedText` 归一层「不再强制输出双 key」属后续演进，尚未实施。

## 3. 目标链路

```text
契约来源（SDK / OpenAPI 上传 / 手写）──┐
                                      ▼
                            FunctionContract（统一物料）
                            executionState: bound | unbound
                                      │
              ┌───────────────────────┼───────────────────────┐
              ▼                       ▼                       ▼
        组件模板（自动重建）     页面提案（自动生成）      编辑器拖拽物料
              └───────────────────────┴───────────────────────┘
                                      ▼
                          PageSpec → 发布（分级）
```

### 3.1 上传管线（D4）

```text
POST /api/v1/openapi/sources（上传）
  → 解析 spec → operations + diagnostics（现状保留）
  → 每个 operation：
      有运行时函数 → 现状 provider 绑定路径（不变）
      无运行时函数 → 建 unbound FunctionContract（不再 400）
  → 自动 RegenerateFromContracts（组件模板，不再需要手动按钮触发）
  → 自动产出页面提案（现有 generator 复用）
  → 响应摘要：
      { operations: 24, contractsCreated: 20, templatesUpdated: 24,
        proposalsCreated: 6, diagnostics: [...] }
  → 前端展示摘要 + [打开编辑器] / [查看提案] CTA
```

### 3.2 编辑器就地闭环

- 组件面板/模板库中 unbound 物料带「未绑定」标记，**不禁用**拖拽。
- 画布上选中 unbound 组件 → 属性面板显示「执行器：未绑定 [去绑定]」→ 抽屉内选择当前 scope 下已注册的运行时函数完成绑定（复用现有 Binding 模型，`kind=provider`）。
- 抽屉打开即溯源：前端复刻服务端确定性映射（`DeriveFunctionID` + `unboundFunctionID` 归一，见 `web/src/pages/PageStudio/CompositeEditor/unboundTrace.ts`），把组件引用的 unbound functionId 反查回 (source, operationId) 并预填；零命中时降级为手动选择。
- 函数候选 = bound 描述符 ∪ 运行时 provider 独有函数，与 `CreateBinding` 的 `registeredFunctionMetaInScope` 校验源一致；保存即重建 bound 契约并刷新编辑器契约视图。
- **同名绑定**（所选函数 id == unbound functionId）：走 T6 原地翻转语义，unbound 契约原地变 bound，刷新后「未绑定」标记自动消失，组件无需改动。
- **不同名绑定**：bound 契约建在运行时函数名下，组件仍引用 unbound 物料 → 抽屉保存后弹确认引导切换组件函数引用（`patchProps({functionId})` 换绑 scaffold，列/字段/联动按新函数重建）。
- OpenAPISources 页的 BindingModal 下沉为编辑器抽屉；源管理页退化为上传入口 + 诊断/绑定状态总览。
- 组合页取消 ≥2 区块限制：单区块页面合法，保存编译、发布、渲染全链放行。

### 3.3 运行时执行边界

- 执行 unbound 函数的 binding → 服务端返回 `409 { "error": "executor_unbound", "message": "该函数尚未绑定执行器" }`，前端渲染为「未绑定执行器」空态 + 去绑定入口，**禁止**静默失败或伪造数据。
- agent/SDK 注册函数时（`RebuildContractFromFunctionMeta` 路径），若同 scope 存在同 functionId 的 unbound 契约 → 自动置 bound 并触发模板/提案 freshness 重算。

### 3.4 发布分级（D5）

| 场景                          | 流程                                                                  |
| ----------------------------- | --------------------------------------------------------------------- |
| dev env（默认）/ 人工拖拽页   | 保存 = 直接发布（published_page_specs 快照 + page_versions 照常记录） |
| prod env 或含高风险函数的页面 | 提案 → 审核 → 发布（现状链路不变）                                    |

策略配置：`pages.publishReview: auto | required`（env 级，默认 dev=auto / prod=required）。审核通过的提案链路、质量门槛（error 级诊断拒绝发布）完全复用。

### 3.5 菜单与文案国际化（D7）

现状的「zh-CN + en-US 双必填」是过度设计：默认名称本就可由生成器确定（humanize / 系统默认语言），强制用户再录一遍英文属于重复劳动，且编辑器对每个 LocalizedText 字段都打 ⚠ 造成噪音。

目标模型：

```text
编辑：默认名称（必填，系统默认语言） + 翻译（可选，全局支持语言列表任选）
校验：仅默认名称非空；任何翻译缺失不警告、不阻断发布
渲染：当前界面语言 → 系统默认语言 → 任一非空值（现状回退链，不变）
组件：LocalizedTextEditor 为唯一编辑组件，REQUIRED_LOCALES 双必填标记移除
```

- 生成器继续只保证系统默认语言（与 ui-generation.md「生成器只保证系统默认语言」一致），`ensureBilingual` 的 en-US 强制补写废除，改为单写系统默认语言。
- `normalizeLocalizedText` 归一层不再强制输出 `{ zh-CN, en-US }` 双 key，按输入归一 BCP47 key 原样透传（短 key 读取兜底保留）。

## 4. 数据模型变更

### 4.1 FunctionContract 增加执行状态

```text
function_contracts 新增列：execution_state VARCHAR(16) NOT NULL DEFAULT 'bound'
  - bound   ：有运行时执行器（现状全部契约迁移后为此值）
  - unbound ：仅有契约，无执行器（OpenAPI 上传未匹配运行时函数时产生）
```

- 迁移文件独立；存量行默认 `bound`，行为与现状完全一致。
- `execution_state` 只影响执行路径与 UI 标记，**不参与** schema digest / stale 判定。

### 4.2 契约准入条件调整

`internal/api/openapi/service.go` `CreateBinding` 的 `functionId is not registered in current game/env runtime` 校验仅保留在「显式绑定」路径；上传管线生成的 unbound 契约不经过 Binding，直接走 `RebuildContractFromFunctionMeta` 等价入口（source=openapi，executionState=unbound）。

## 5. 与非目标

- 不改 PageSpec 协议、不改渲染器映射表（见 [ui-generation.md](./ui-generation.md)）。
- 不引入第二套表单/页面运行时。
- 不为 unbound 函数提供"模拟执行"（mock 数据仅在编辑器数据试跑面板内可用，发布页禁止）。
- httpConnector 直连执行仍按现状禁用（需 allowlist + SecretRef 策略，另行设计）。

## 6. 验收标准

- 上传一份含 N 个 operation 的 OpenAPI 文档（无任何 agent 在线）：组件库出现 N 个物料，提案收件箱出现对应页面提案，响应摘要数字正确。
- 单区块组合页可保存、发布、渲染、执行（bound 函数）。
- unbound 组件页面发布后：渲染正常，执行动作返回 `executor_unbound` 结构化错误并展示空态。
- agent 注册同名函数后，unbound 契约自动转 bound，页面可执行，无需人工操作。
- 契约变更后组件模板自动重建（无需手动 regenerate），模板 stale 标记正确。
- dev env 保存即发布；prod env 仍走提案审核。
- 菜单/页面标题仅录默认名称（无 en-US）可保存、可发布；翻译可选录入，渲染回退正确。
- 真实浏览器 E2E（`web/e2e/` real-dashboard）覆盖上述链路。

## 7. 已知边界

- unbound 契约的 functionId 来自 OpenAPI operationId 的确定性映射；agent 侧注册的 functionId 命名不一致时无法自动绑定，需人工在编辑器抽屉内绑定。
- 不同名绑定成功后，原 unbound 契约行即时清理（`CreateBinding` 事务内与上传重放的 `removeSupersededUnboundContract` 对称执行）——否则资源语义槽位出现同源双候选，unresolved conflict 会把 resource proposal 降级 needs_review。
- 抽屉内不展示 proposal/模板 freshness 提示——Proposal 队列有独立入口；抽屉只解决「绑定」这一件事。
- 发布分级的 env 判定依赖 scope 传递正确性；`X-Env` 缺失时按最严格（required）处理。
- 上传管线为同步事务，超大文档（>500 operations）的耗时与超时策略在落地时按实测调整（必要时转异步任务）。
