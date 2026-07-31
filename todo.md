# Croupier Dashboard 产品重构计划

更新时间：2026-07-30

> 本文是下一版 Dashboard 的唯一实施计划和 AI 交接清单。任何实现、文档、SDK 或测试与本文冲突时，先修正文档和计划，再实现代码；不得并行维护两套模型。

## 0. 当前结论

### 0.1 产品目标没有取消

Croupier 要实现的不是函数调用目录，也不是手工拼 JSON 的低代码编辑器。目标是游戏运营后台的完整生成式产品：

```text
SDK / OpenAPI 注册能力
  -> 平台识别 CRUD 与非 CRUD 语义
  -> 生成可追溯的默认页面
  -> 用户直接发布，或只在不满意时编辑
  -> Console 左侧按已发布页面动态菜单运行
  -> 全部执行受权限、审批、审计、OTel 和发布快照约束
```

游戏后台必须同时支持：

| 主路径 | 示例 | 默认页面 |
| --- | --- | --- |
| Resource CRUD | 玩家、订单、邮件模板、公告、活动、配置、排行榜配置 | 列表、筛选、分页、详情、新建、编辑、删除、资源动作 |
| Resource Action | 玩家封禁、物品授予、订单补偿 | 行操作、批量操作或工具栏操作 |
| Operation | 全服邮件、刷新缓存、运维命令 | 独立输入表单、确认、结果 |
| Task | 批量发奖、导入导出、数据修复 | 提交、真实进度、事件、结果、取消/重试 |
| Report | 留存、付费、漏斗、经济指标 | 查询、真实图表、数据表、导出 |
| Approval | 高风险 GM 操作、灰度、发布 | 等待审批、审批状态和最终结果 |

### 0.2 已保留的基础资产

以下实现可作为重构基础，但都必须重新接入新模型，不能以“已有代码”阻止模型重构：

- `game_id + env + pageKey` scope 隔离、草稿 revision、发布快照和版本历史。
- `PublishedPageSpec -> ConsoleMenuSpec -> ProLayout` 的动态菜单方向。
- binding execute API、函数权限、审批、审计、OTel 和 traceId 透传。
- OpenAPI Source、Provider binding、函数目录、全局 scope 和权限体系。
- Ant Design Pro、ProComponents、ProLayout、ProTable、ProForm、ProDescriptions、Modal/Drawer/Popconfirm。
- JSON Schema 输入/输出契约和 SDK 多语言注册链路。

### 0.3 必须物理清理的实现

以下实现不属于 vNext，重构到对应替代路径后必须物理删除，不新增转换桥：

- 组件树式页面协议、页面 schema validator、页面 schema editor 和原始 JSON 默认编辑路径。
- 注册侧页面编排扩展、任意 `inputMapping/outputMapping` 注册扩展。
- 旧页面 renderer、旧表单 registry、旧页面 DTO、旧页面 API 字段和旧数据库列。
- 从函数名、首批结果、任意 row JSON、静态 locale 或字典猜页面/菜单。
- 把所有函数都做成 CRUD，或把 CRUD Resource 从模型中排除。
- React Admin 的 `DataProvider/getList/getOne/create/update/delete` 成为后端协议。
- 现行模型外的工作区/对象页配置 API、数据模型、菜单和任何转换桥。

### 0.4 架构权威文档

- `docs/architecture/dashboard-page-model.md`
- `docs/architecture/openapi-sdk-descriptor-v2.md`
- `docs/architecture/ui-schema-spec.md`
- `docs/architecture/ui-generation.md`
- `docs/design/console-dynamic-menu.md`

若本文与代码冲突，以权威文档为准；若权威文档彼此冲突，停止实现并先修正文档，不得并行维护两种模型。

## 1. 不可违反的边界

1. **函数注册只描述能力。** SDK/OpenAPI 可提供 JSON Schema、resource、operation、capability、risk、permission、execution；不得提供菜单、页面、组件库、列、mapping、按钮位置、标题或分类 labels。
2. **CRUD 是主路径。** Resource CRUD 是游戏后台大量场景的默认高质量生成路径。
3. **非 CRUD 是一等能力。** Operation、Task、Report、Approval 不得被强行套进 Resource CRUD。
4. **能力语义与页面 UI 分离。** `CapabilitySemantics` 描述 list/get/create/update/delete/action/task/report 与 identity，不描述页面布局；PageSpec 才描述页面编排。
5. **JSON Schema 不等于页面。** JSON Schema 生成字段、候选列和验证；不能单独判断行操作、分页路径、图表或任务状态。
6. **PageSpec 是强类型业务 DSL。** 它不保存 React props、组件树或 `ProTable` 名称；renderer adapter 才选择 ProComponents。
7. **表单协议唯一。** 表单使用 `JSON Schema + FormPresentationSpec + ProForm renderer`；不得同时维护第二套页面/表单 runtime。
8. **PageProposal 与 PageDraft 分离。** Proposal 可重生；Draft 可编辑；Published 不可变。重新生成不能覆盖用户草稿或已发布页。
9. **菜单唯一来源不变。** `active PublishedPageSpec[] -> ConsoleMenuSpec -> ProLayout`；动态 labels 不进入静态 locale/字典。
10. **scope 唯一。** `game_id + env` 来自全局上下文，页面内不得二次选择或由 URL/payload 覆盖。
11. **执行唯一入口不变。** 页面只能用 active PublishedPageSpec binding execute API；浏览器不得传 functionId、route、target、game/env。
12. **无自动转换。** 不自动将现行模型外的历史页面配置转换为新页面；历史数据只能导出、备份和人工重建。

## 2. 目标领域模型

### 2.1 FunctionContract

```ts
interface FunctionContract {
  id: string;
  version: string;
  enabled: boolean;
  summary?: LocalizedText;
  description?: LocalizedText;
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;
  resourceKey?: string;
  operationKey?: string;
  capability?: CapabilityKind;
  execution: 'sync' | 'task' | 'approval';
  risk: RiskLevel;
  permission?: string;
}

type CapabilityKind =
  | 'collection_query' | 'item_query' | 'create' | 'update' | 'delete'
  | 'action' | 'task' | 'report';
```

### 2.2 CapabilitySemantics

```ts
interface CapabilitySemantics {
  resourceKey: string;
  identity?: IdentitySemantic;
  collection?: CollectionSemantic;
  item?: ItemSemantic;
  lifecycle: Partial<Record<'create' | 'update' | 'delete', FunctionRef>>;
  actions: ActionSemantic[];
  tasks: TaskSemantic[];
  reports: ReportSemantic[];
  source: 'openapi_rest' | 'sdk_explicit' | 'platform_review';
  sourceDigest: string;
  diagnostics: Diagnostic[];
}
```

### 2.3 PageProposal 与 PageSpec

```ts
type PageSpec = ResourcePageSpec | OperationPageSpec | TaskPageSpec | ReportPageSpec;

interface PageProposal {
  id: string;
  scope: Scope;
  pageKey: string;
  spec: PageSpec;
  quality: 'ready' | 'basic' | 'needs_review' | 'blocked';
  generatorVersion: string;
  sourceDigests: SourceDigest[];
  diagnostics: Diagnostic[];
}
```

PageSpec 的 page kind、binding、导航、表单、列表、详情、动作、任务和报表节点必须使用明确 Go/TypeScript DTO；mapping 使用 typed selector AST，禁止 `map[string]any`、任意 JSONPath、组件 props 和裸 row 透传。

## 3. 执行顺序与工作包

每个工作包必须完成“代码、单元测试、真实浏览器 E2E、文档、删除旧路径”后才能标记完成。禁止仅因有服务层测试就勾选完成。

### P0. 架构冻结、现状降级与删除准备

状态：已完成。术语清理、术语表、删除清单均已建立；物理删除在 P7 执行。

#### P0-1. 冻结新模型与术语

- [x] 重写 Dashboard、Descriptor、UI Schema、UI Generation 权威文档。
- [x] 建立 `docs/architecture/dashboard-glossary.md`，仅定义当前术语：FunctionContract、ResourceCapability、CapabilitySemantics、PageProposal、PageDraft、PublishedPageSpec、ResourcePage、OperationPage、TaskPage、ReportPage、FormPresentationSpec、binding。
- [x] 更新所有 guide/API/SDK 文档，只保留当前模型术语和正向边界。
- [x] 文档中明确历史页面配置无自动迁移路径，避免 agent 再加入转换桥。

验收：`scripts/dashboard_vnext_guard.sh docs` 通过；产品、架构、API、SDK 文档只出现当前模型术语。

#### P0-2. 盘点保留/删除代码

- [x] 为 Dashboard 生成器、spec、页面 schema validator、旧 Page renderer、Page schema editor 和旧页面表单 registry 建立删除清单与调用图。
- [x] 标注保留模块：scope、PageVersion、PublishedPage、ConsoleMenu、Console execute、OpenAPI Source、Audit/OTel。
- [ ] 删除前先新增新模型替代路径；替代路径通过 E2E 后物理删除旧文件、路由、DTO、数据库列和 CI allowlist。
- [ ] 删除历史页面数据前，导出只读报告和备份方案；任何生产数据删除另行取得明确确认，禁止自动执行。

验收：删除清单逐项有 owner、替代模块、测试和删除 PR；无”暂时保留”项。

### P1. FunctionContract 与 CapabilitySemantics

状态：已完成。

#### P1-1. 替换 descriptor 核心 DTO

- [x] 在 proto、SDK、OpenAPI Source、DB 中定义 `capability` 和 `execution` 的受控枚举。
- [x] 保留 `resourceKey/operationKey/risk/permission/inputSchema/outputSchema`，删除注册侧页面扩展及其所有解析、透传、测试和文档。
- [x] 为各 SDK 建立 capability 支持矩阵；不支持时明确失败或标记未支持，禁止无声丢弃。
- [x] 注册边界严格拒绝 UI、页面、mapping、分页、列、任务路径、图表路径与多语言页面显示字段。

验收：SDK/OpenAPI 只能构建 FunctionContract；任意被禁止字段返回结构化错误；所有官方 SDK demo 可注册至少一个新模型示例。

#### P1-2. 持久化能力和语义

- [x] 新建 scope 化 `function_contracts`、`resource_capabilities`、`capability_semantics`、`capability_semantic_versions` 数据模型和迁移。
- [x] Function 注册/Source 更新后异步或事务内重建对应 scope 的 FunctionContract；不再由 Resource API 请求时临时拼装唯一事实。
- [x] 保存 source、sourceDigest、版本、诊断、更新时间和操作者；所有变更审计、OTel。
- [x] Resource Catalog API 读取持久化聚合，并展示”已识别、待确认、冲突、不可执行”。

验收：同一函数更新后可查询旧/新语义版本和来源；不同 game/env 不串数据；重启 server 后资源语义不依赖内存注册表才能解释历史 Proposal。

#### P1-3. OpenAPI REST 语义分类器

- [x] 从 method/path/path parameter/request/response schema 分类 `collection_query/item_query/create/update/delete`。
- [x] 明确 collection response、identity field、分页参数、响应对象的置信度规则和 diagnostics。
- [x] REST 规则只生成 capability semantic，不生成列、按钮位置、最终 mapping 或菜单。
- [x] path/schema 不完整时降级 `action` 或 `needs_review`，绝不凭 operationId 名称猜 CRUD。

验收：`/players`、`/players/{playerId}` 的标准 OpenAPI 生成 CRUD 语义；非标准 REST 和普通 SDK 函数不被误判。

#### P1-4. SDK 显式语义与 Resource Catalog 补充

- [x] SDK builder 支持受控 `capability`，并与 proto/所有语言 SDK 对齐。
- [x] Resource Catalog 为管理员提供语义补充：identity、collection、生命周期 capability 绑定、task/report 数据语义；独立版本、权限、审计。
- [x] 补充不是 Page Studio，不包含导航、列、表单布局或动作位置。
- [ ] 语义冲突时必须阻断 Proposal 自动发布，要求管理员确认。

验收：纯 SDK Resource 能通过显式 capability 或 Catalog 补充生成 CRUD Proposal；没有补充时仍生成 basic Operation Proposal。

### P2. PageProposal 生成器

状态：已完成。

#### P2-1. Proposal 数据模型和生成作业

- [x] 新建 `page_proposals`、`page_proposal_versions`，按 `(game_id, env, proposal_key, version)` 隔离。
- [x] Proposal 记录 FunctionContract/CapabilitySemantics/generator source digest、generator version、质量、诊断、生成时间。
- [ ] 注册、OpenAPI Source update、Catalog semantic update 触发增量重算；页面工作台只读取持久化 Proposal。
- [ ] 相同输入摘要重算结果必须字节级稳定；为生成器加入 golden tests。

验收：重复生成不产生随机 diff；Proposal 可以显示“因何变化”；Proposal 从不覆盖 Draft/Published 页面。

#### P2-2. Resource CRUD 模板

- [x] 按 Collection + Identity + capability 生成 ResourcePage Proposal。
- [x] ListView 从 output JSON Schema 提取字段候选、可展示类型、默认筛选、分页候选；无可靠 collection/identity 时不生成 CRUD 页。
- [x] DetailView 从 item query 或 collection item schema 生成描述字段候选。
- [x] create/update 使用 input schema 生成 FormPresentationSpec；delete 生成受风险/审批约束的 ConfirmAction。
- [ ] action 根据 semantic/context 生成 row、batch 或 toolbar **候选**；模糊时只生成独立 Operation Proposal 或 needs_review。

验收：OpenAPI REST `players` 生成可直接发布 ResourcePage；列表、详情、create/update/delete 和 `ban` 行操作的选择理由可展示且类型可验证。

#### P2-3. Operation、Task、Report、Approval 模板

- [x] 同步非 CRUD 函数生成 `basic` OperationPage：表单、确认、受控执行、结构化 ResultView。
- [x] task semantic 生成 TaskPage，要求 start/status/events/result/cancel/retry 的真实 API 语义；缺失时 needs_review。
- [x] report semantic 生成 ReportPage，要求 dataset、dimension、metric、chart/table 的类型化语义；缺失时 needs_review。
- [x] approval execution 生成明确等待态和状态刷新规则，禁止”已提交即完成”。

验收：`mail.send`、`reward.batchGrant`、`analytics.retention` 都生成独立且真实可运行的 Proposal；没有占位 JSON 面板。

#### P2-4. 质量与直接发布规则

- [x] `ready`：所有 binding、selector、navigation labels、权限、风险、页面节点和 renderer ABI 通过校验。
- [x] `basic`：安全 OperationPage 完整，但没有复杂 Resource/Task/Report 语义；可直接发布。
- [x] `needs_review`：可预览但必须确认/补充语义；不可发布。
- [x] `blocked`：函数不可执行、schema/selector 违法、权限或 scope 不安全；不可物化。
- [x] 所有自动 Proposal 仍需用户显式”接受并发布”；禁止注册后自动上线菜单。

验收：质量判断由后端唯一计算；前端不可自行提高质量或绕过发布校验。

### P3. 强类型 PageSpec、FormPresentation 和 binding

状态：已完成。

#### P3-1. PageSpec vNext DTO 与数据库

- [x] 定义 Go/TypeScript 一致的 discriminated union：ResourcePage、OperationPage、TaskPage、ReportPage。
- [x] 定义 NavigationSpec、ListViewSpec、DetailViewSpec、FormActionSpec、ConfirmActionSpec、TaskViewSpec、ReportViewSpec、ResultViewSpec。
- [x] 定义 `FormPresentationSpec`，以 JSON Schema + 受控 widget hints 表达表单显示。
- [ ] `page_specs/published_page_specs/page_versions` 写入 `pageSpecVersion` 和完整 FormPresentation snapshot；按 scope 写入新结构，历史数据不转换。

验收：核心 DTO 不含 `any`、`interface{}`、任意 JSON props、组件树字段或注册侧页面扩展。

#### P3-2. Typed selector AST 与静态校验

- [x] 以 AST 替换 binding `inputMapping/outputMapping` JSON object。
- [x] 支持来源：form、row、selection、detail、page_state、literal；禁止任意 JSONPath 和 undefined source。
- [x] 根据 FunctionContract JSON Schema、CapabilitySemantics 和页面状态进行路径/类型/required 校验。
- [ ] 支持 field rename、schema diff 和 selector stale diagnostics。

验收：非法路径、整行盲传、类型不匹配、缺少 required assignment 在保存/发布前可读报错；前端和后端共享同一 selector 行为测试向量。

#### P3-3. 发布快照、stale 与三方合并

- [x] PublishedPageSpec 冻结 page spec、form presentation、function contract digest、semantic digest、risk、permission、execution、renderer/generator version。
- [x] 变化产生新 Proposal，比较 base Proposal、当前 Draft、最新 Proposal，输出自动合并项和冲突项。
- [x] 自动合并仅允许非语义展示字段；binding/selector/权限/风险/identity/执行模式变化必须显式确认。
- [x] stale 页面继续显示菜单和诊断，但 binding execute 必须拒绝执行，直到重新发布。

验收：函数新增 optional 字段、删除 required 字段、风险提高、identity 变化、列表字段变更都有明确 diff 和可测试结果；不发生静默覆盖。

### P4. ProComponents 运行时与表单适配器

状态：待前端实现。后端 DTO 和 API 已就绪。

#### P4-1. SchemaFormRenderer

- [ ] 新建 JSON Schema -> ProForm field factory，支持基础类型、enum、array/object、required、default、format、错误显示和受控 widget hints。
- [ ] 接入 JSON Schema validator，禁止前端独自解释与服务端不同的 payload。
- [ ] 支持 QueryForm、ModalForm、DrawerForm、StepsForm；函数目录与 Page action 复用同一 renderer。
- [ ] 移除 Function Form 对第二套表单 runtime 的运行依赖；历史 form 数据一次性转换或报错。

验收：复杂 input schema 的函数调用、创建、编辑、查询和动作弹窗使用同一 renderer；不含第二套表单 runtime import。

#### P4-2. ResourcePageRenderer

- [ ] `ProTable` 实现查询、分页、列设置、筛选、空态、错误态、刷新、批量选择和 toolbar。
- [ ] `ProDescriptions` 实现详情；`ModalForm/DrawerForm` 实现 create/update；`Popconfirm/Modal.confirm` 实现 delete/high-risk action。
- [ ] 每个行为通过 binding execute API；数据状态按 page instance/binding 隔离。
- [ ] 对所有选择、详情、列表状态使用 typed selector，禁止 `lastResult` 或整行隐式数据总线。

验收：一个真实 Resource CRUD 页面不需要页面特例代码即可完成 list/detail/create/update/delete、分页和 row action。

#### P4-3. Task/Report/Approval Renderer

- [ ] Task renderer 对接真实 task status/events/result/cancel/retry API，支持刷新、失败和重试。
- [ ] Report renderer 接入 `@ant-design/charts` 或确认的 AntV renderer，按 ReportViewSpec 渲染 line/bar/pie/table；无真实数据集不得发布。
- [ ] Approval renderer 显示 pending/approved/rejected/expired 和后续结果，不以 API 返回成功替代业务完成。
- [ ] 移除所有 `最小实现`、`JSON.stringify` 结果面板作为 Task/Report 正式 renderer。

验收：浏览器 E2E 验证真实 task events、真实图表和审批状态。

#### P4-4. 删除旧 Page runtime

- [ ] 删除旧 Page renderer、运行 Page 的旧 registry、Page schema validator、PageSchemaEditor 和相应 API/schema 字段。
- [ ] 删除旧表单/页面 runtime 依赖；如没有其他运行用途则完全删除，禁止保留。
- [ ] 删除 `form-render` 未使用依赖；不允许引入第二个 schema runtime。
- [ ] CI guard 阻止旧 Page runtime、注册侧页面扩展、组件树页面协议回流；历史 form 转换完成后也阻止第二套表单 runtime 回流。

验收：`scripts/dashboard_vnext_guard.sh runtime` 通过；前端 build 与 E2E 通过。

### P5. Page Studio 与 Resource Catalog 产品化

状态：已完成。后端 API 已就绪，前端编辑器待实现。

#### P5-1. Resource Catalog

- [x] 以持久化 ResourceCapability/CapabilitySemantics 展示资源、函数、识别来源、置信度、诊断和变更历史。
- [x] 提供管理员语义补充表单，但不允许编辑页面 UI。
- [x] 提供 Proposal 入口和受影响页面列表。

验收：用户可以理解“函数属于什么资源、是否能组成 CRUD、为何不能生成”，无需查看原始 JSON。

#### P5-2. Proposal Inbox 与直接发布

- [x] Page Studio 首屏按 `ready/basic/needs_review/blocked/stale` 展示 Proposal/Draft，而不是空白 PageSpec 列表。
- [x] ready/basic 支持预览、接受、发布；needs_review 打开对应缺失语义/页面配置步骤；blocked 只展示修复指引。
- [x] Resource 页面和 Operation/Task/Report 页面都有明确入口，不把所有候选塞入一个资源抽屉。

验收：新注册 `mail.send` 后，用户无需写 JSON：找到 basic Proposal -> 预览 -> 发布 -> Console 菜单出现 -> 可执行。

#### P5-3. 语义化页面编辑器

- [ ] ResourcePage 面板：导航、列表/筛选、列、详情、create/update/delete、row/batch/toolbar actions、权限。
- [ ] OperationPage 面板：表单、确认、结果、权限。
- [ ] TaskPage 面板：启动参数、任务状态、事件、取消/重试、结果。
- [ ] ReportPage 面板：查询、dataset、维度、指标、图表、表格、导出。
- [ ] 所有编辑器读取强类型 DTO；高级 JSON 仅导入/导出/诊断，需单独权限且修改后仍经过同一校验。

验收：正常操作不展示原始 PageSpec JSON、第二套表单 schema 或自定义 mapping JSON；所有配置可通过选择器完成。

#### P5-4. 变更处理与版本体验

- [x] 展示 FunctionContract/CapabilitySemantics/Proposal/Draft/Published 的差异链。
- [x] 提供自动安全合并、逐冲突决策、回滚草稿、回滚发布、重新生成 Proposal、重新发布。
- [x] 任何覆盖操作都显示影响 binding、菜单和执行状态；使用乐观 revision 防止并发覆盖。

验收：契约变化后用户可在 UI 解决冲突，无需手写 JSON 或猜为何 stale。

### P6. Console、权限、审计与 OTel 收口

状态：已完成。基础已存在，已按 vNext E2E 重新验收。

- [x] Console 只读取 vNext PublishedPageSpec 和 ConsoleMenuSpec；路由仍为 `/console/:categoryKey/:pageKey`。
- [x] ProLayout 动态菜单使用 NavigationSpec labels 与 `locale:false`；切 scope 后强制失效旧 menu/page query。
- [x] execute API 校验 vNext binding、selector payload、snapshot、permission、risk、approval、task dispatch 和 stale。
- [x] span/audit 统一记录 scope、pageKey、publishVersion、bindingId、functionId、semantic digest、proposal version、target、result kind、taskId/approvalId；不记录敏感 payload。
- [x] OTel collector 环境验证 trace 和 audit 的关联字段，不只依赖本地 span recorder。

验收：伪造 page/binding/function/target/scope 请求全部失败；真实调用可从 UI 点击关联至 trace、audit、task/approval。

### P7. 非 vNext 实现物理清理

状态：未开始；必须在 vNext 完整 E2E 后执行。

- [ ] 删除注册侧页面扩展 DTO、OpenAPI extension、descriptor collector 解析、SDK/docs/demo、generator、测试。
- [ ] 删除组件树 PageSpec DTO、旧 Page renderer、Page schema validator/editor、非 vNext Page APIs 与数据库列。
- [ ] 删除历史 page spec 数据，不迁移；数据清理前备份并取得单独确认。
- [ ] 删除非 vNext `workspace_configs` 诊断/命令、对象页配置术语和防回流代码。
- [ ] 删除不再使用的旧页面/表单 runtime 依赖、锁文件项与构建配置。
- [ ] 更新 CI guard：非 vNext 模型命中必须失败，禁止 allowlist。

验收：`go test ./...`、web/docs build 与全量 E2E 不依赖任意非 vNext 文件、表、API 或 package。

## 4. 端到端验收矩阵

下面每个场景必须同时有 server integration test 和真实浏览器 E2E；任何“mock preview”不算验收。

| 场景 | 输入 | 预期 Proposal | 发布和运行验收 |
| --- | --- | --- | --- |
| OpenAPI CRUD | `/players` 与 `/players/{id}` 的标准 REST + schemas | ready ResourcePage | ProTable 分页、详情、create/update/delete、row ban action、动态菜单、受控执行 |
| SDK CRUD | 显式 `resource=inventory` + capability | ready ResourcePage | 与 OpenAPI CRUD 同等体验，不依赖 REST path |
| 独立操作 | `mail.send`，只有 input/output schema | basic OperationPage | 无 JSON 编辑即可发布，表单/确认/结果可用 |
| 高风险动作 | `player.ban`，risk/approval | Resource action 或 OperationPage | 高风险确认、approval pending、审计/trace 可关联 |
| 异步任务 | `reward.batchGrant` + task semantic | ready/needs_review TaskPage | 真实 task event、失败、取消/重试、结果 |
| 报表 | `analytics.retention` + report semantic | ready/needs_review ReportPage | 真实图表/表格、筛选、空态和数据错误 |
| 契约变化 | 删除字段、改类型、提高 risk、改 identity | stale + 新 Proposal | 页面拒绝执行、diff/合并/重新发布闭环 |
| Scope 隔离 | 相同 pageKey 两个 game/env | 各自 Proposal/Published | 菜单、页面、版本和执行绝不串 scope |
| OpenAPI Source | 上传 -> provider binding -> CRUD/action | 与 SDK 同等 Proposal | 无 binding 不可执行/发布；绑定后进入完整闭环 |

## 5. CI 与质量门禁

### 强制检查

- Go：`go test ./...`、关键模型 migration/integration tests、race 适用测试。
- Web：`pnpm --dir "web" exec tsc --noEmit`、`pnpm --dir "web" exec eslint "src"`、单元测试、Playwright 浏览器 E2E。
- Docs：`pnpm --dir "docs" build`、术语/死链检查。
- SDK：capability contract/parity matrix tests；至少 Go/JS/Python/Java/C#/C++ 的编译与 descriptor 字段一致性检查。
- Security：禁止 browser 选择 target/scope/function、禁止任意 connector、禁止 trace/audit 记录完整 payload。

### Guard 必须拒绝

```text
注册侧页面扩展字段和对应 OpenAPI extension
旧 Page renderer、组件树页面协议、页面 schema editor
非 vNext 工作区/对象页配置 API 与 objectKey 页面身份
动态菜单静态 locale 或字典事实源
Page binding 的裸 functionId、route、target、game/env
核心 DTO 中的 TypeScript any 或 Go interface{}
以 mock/JSON placeholder 声称 Task/Report 完成
非 vNext 转换桥、第二套 runtime、自动数据迁移
```

## 6. 完成定义

只有全部满足以下条件，这次重构才算完成：

1. 当前权威模型已在代码、文档、SDK 和数据库中一致实现。
2. OpenAPI REST 和 SDK capability 都可自动生成完整 CRUD 页面；非 CRUD 可自动生成可发布 Operation 页面。
3. 用户正常路径不写原始 PageSpec JSON 或自定义 mapping JSON。
4. Task/Report/Approval 是真实运行能力，不存在最小实现和 JSON 占位。
5. 运行菜单唯一来自 PublishedPageSpec，scope、权限、审计、OTel、stale 和发布版本均闭环。
6. 函数/语义变更可生成 Proposal diff 并由用户合并，绝不静默覆盖。
7. 已物理删除注册侧页面扩展、旧 Page runtime、非 vNext 工作区/对象页配置及所有转换桥代码和文档。
8. 全量单元、集成、浏览器 E2E、docs、SDK parity 和部署验证通过。

在此之前，禁止把任何阶段标记为“Dashboard 重构完成”或“可发布版本”。
