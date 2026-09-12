---
title: 组合页编辑器 V3 使用与扩展指南
---

# 组合页编辑器 V3（组件化）

> 状态：**已上线全量**（V3 计划 + V3.1 边界清零：动作链/弹窗分组多组件/容器子级交互/回读增强/多选/撤销重做/右键菜单）
> 设计依据：[V3 计划](./composite-editor-v3-plan.md)｜[参考产品对比分析](./editor-reference-analysis.md)｜spec 模型见 [Dashboard 页面模型](../architecture/dashboard-page-model.md) CompositePage 节
> 参考样品：[amis-editor](http://192.168.5.5:8001)、[Appsmith](http://192.168.5.5:8002)

## 1. 功能全景

| 模块           | 能力                                                                                                                                                        |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 入口           | 提案收件箱「创建组合页」/ 页面管理列表 composite 页「编辑」（**回读已有页面**）                                                                             |
| 组件面板（左） | 函数组件（按资源分组、scaffold 按契约实例化）+ 基础组件（按钮/弹窗/容器/文本）；空 scope 引导切换                                                           |
| 大纲（左 Tab） | 组件树导航，点击定位                                                                                                                                        |
| 添加           | 点击（根末尾）/ 拖拽（**落点指示线**；拖到容器上按 allowedChildren 契约进 children（非法子类型回退为容器后插入）；拖到弹窗占位卡进弹窗）                    |
| 同函数多实例   | 同一函数可拖 N 个组件（key 自动 `fid`/`fid-2`…），分别配置                                                                                                  |
| 换绑           | 属性面板「函数（可换绑）」全量下拉；换绑后组件重新 scaffold                                                                                                 |
| 画布           | 拖拽排序、右缘调宽（4-24 栅格）、**右键菜单**（上移/下移/选择父容器/复制/删除）、容器子级点选/删除/同级移动                                                 |
| 多选           | **Shift+点选**累积（高亮+计数），普通点击=单选（清空多选）→ 顶栏「删除所选」批量删除；删除/撤销/重做后按存活节点自动清理悬空选中                            |
| 撤销/重做      | 顶栏 ↩/↪ + **Ctrl+Z / Ctrl+Shift+Z**（50 步快照，覆盖全部树变更）                                                                                           |
| 弹窗           | 栅格占位卡 → **双击/「进入弹窗编辑」切换画布为弹窗内部**（面包屑「页面 / 弹窗名」返回）；内部可放多个函数表单                                               |
| 属性面板（右） | rjsf schema 驱动；**「配置/动作」两 Tab**（Appsmith 式；选中按钮自动切「动作」Tab）；标题/宽度/自动执行/展示方式/列勾选（Checkbox）/成功后刷新              |
| 行操作         | 表格属性面板可视化配置：行尾按钮 → 弹窗，行字段→表单参数映射下拉，危险标记                                                                                  |
| 动作链         | 按钮主动作 + **后续动作列表**（执行/刷新/关弹窗/跳转/提示，按序执行，步骤可带参数来源）                                                                     |
| **通用事件**   | 全组件事件（WinForms 式）：button=点击；**表格=行点击/行选中**（携带行数据上下文）；**表单=成功后/失败时**；文本/字段卡=点击——「动作」Tab 自动出现          |
| **动作类型**   | 6 种：打开弹窗（重复点击=toggle 关闭）/ 关闭弹窗 / 执行 / 刷新 / 跳转链接(url) / 提示消息(文案)；删除目标节点后绑定自动清理（按钮徽标恢复「点击绑定动作」） |
| **执行参数**   | run/refresh 动作参数来源：`参数=节点.字段`（取组件输出）/ `参数=row.字段`（事件行）/ 字面量                                                                 |
| 数据试跑       | 底部数据面板：选中函数组件一键执行（Appsmith Query 面板形态），结果表格/JSON 即席展示                                                                       |
| 预览           | 顶栏切换，**复用发布渲染器**——autoRun 执行/弹窗提交/刷新级联/动作链，所见即发布                                                                             |
| 保存           | 编译树 → `POST /versioning/pages/composite`（提案）→ 收件箱接受发布 → 菜单出现                                                                              |
| 回读           | `?pageKey=` 自动载入（提案 `composite--key` → 裸 key → draft 三数据源 fallback），**顶部按钮还原为独立按钮节点**（round-trip 等价）                         |

## 2. 典型页面搭建（玩家管理页）

```
① 玩家表格（自动执行）
   左栏点击 player.list → 表格组件（列=输出 schema 全选）
② 发邮件弹窗
   基础组件拖「弹窗」→ 画布紫色占位卡
   双击占位卡 → 进入弹窗内部（面包屑出现）
   左栏拖 mail.send 进来 → 表单（字段=输入 schema）
   点选表单 → 属性面板「成功后刷新」= 刷新 player.list
③ 行操作（行尾按钮）
   面包屑回「页面」→ 点选表格 → 属性面板底部「行操作」
   添加：文案=发邮件｜打开弹窗=发邮件｜映射 playerId ← 行.playerId｜危险=否
④ 预览
   顶栏「预览」→ 表格自动执行出数据 → 行尾[发邮件] → 弹窗（playerId 已带入）
   → 提交 → 关窗+提示 → 表格自动刷新
⑤ 保存
   「保存为提案」→ 提案收件箱接受发布 → 左侧菜单出现页面
```

变体：

- **顶部按钮**：拖「按钮」到表格后 → 属性「点击动作」=打开弹窗（编译为表格顶部按钮）；可加**后续动作**（如先执行再刷新）
- **同一数据多视图**：再拖一次 player.list（key 自动 -2 后缀），配置不同列/宽度
- **弹窗多组件**：弹窗内部可继续拖入第二个函数表单（编辑器限制弹窗内仅 fnForm，V1 边界；同 group 渲染进同一弹窗）
- **页签容器（V2）**：拖「页签容器」→ 自带 2 个空页签；点击页签头切换激活页，拖入的表格/字段卡/按钮/文本自动进**当前页签**。属性「页签组名」可选（缺省自动分配）；页签标签在页容器的「标题」配置。发布端同组渲染进同一 Tabs，页内区块照常 autoRun/联动

## 3. 编译规则（编辑树 → CompositeSection）

| 画布                                | 发布 spec                                                                                                                                                                |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| fnTable / fnFields / fnForm(inline) | 区块（view=table/fields/form）                                                                                                                                           |
| 弹窗容器（modal）                   | 其每个函数子组件 → `display: dialog` + `group: modal-<id>`（同弹窗）                                                                                                     |
| 页签容器（tabs）                    | 每页（container）内区块 → `display: tab` + `group: 页签组名`（优先 `sectionKey`，否则 `tabs-<id尾6>` 去重）+ `tab: 页签标签`（缺省「页签 N」）；同 group 渲染进同一 Tabs |
| 页签页内的按钮                      | 同独立按钮：挂最近表格的 `toolbar.actions`                                                                                                                               |
| 页签页内的文本                      | 静默跳过（同弹窗）                                                                                                                                                       |
| 表格属性「行操作」                  | `table.rowActions`（目标=group；行字段→参数映射；链透传）                                                                                                                |
| 独立按钮（置于表格后）              | 该表格 `toolbar.actions`；非弹窗动作（执行/刷新）发布为 `chain`                                                                                                          |
| fnForm「成功后刷新」                | `onSuccessRefresh`（目标=区块 key）                                                                                                                                      |
| 容器                                | 子节点平铺（span 各自保留）                                                                                                                                              |
| 文本                                | 不发布（警告）                                                                                                                                                           |

警告场景（保存时提示，不阻断）：空弹窗、空页签容器、动作目标已删、按钮不在任何表格之后、区块未绑定函数、文本组件。

**保存即发布级校验（2026-09 统一）**：组合页提案创建（保存）与服务端
accept-and-publish 共用**同一 selector 规则源**（`CollectBindingSelectorIssues`：
必填参数映射 / output shape 匹配 / source kind 上下文合法性等）。保存时违规以
error 级诊断写入提案并降级 `needs_review`——提案收件箱「需要处理」队列可见具体
字段与原因；不再出现「保存看似可发布、点发布才 422」。修复映射后重新保存，
质量恢复并由收件箱正常发布。

## 4. 发布页行为（PageRenderer/CompositeRenderer）

- `autoRun` 区块进入页面自动执行；`refreshOn` 上游产出自动重跑（page_state 同名字段合并）；页签（`display=tab`）内区块同样执行——分组只影响布局聚合，不影响数据行为
- `display=tab` 区块按 `group` 聚合渲染进同一 `<Tabs>`（整行），组内按 `tab` 标签聚合页，页内区块整行堆叠；标签缺省兜底「页签 N」
- 行操作列/顶部按钮 → 打开 group 对应弹窗（参数预填，danger 二次确认）→ 提交成功关窗+提示 → `onSuccessRefresh` 级联刷新
- `chain` 步骤在主动作后按序执行（runBinding/refreshNode）
- 同函数多实例按 key 独立执行互不干扰

### 4.1 预览验证闭环（交互规格）

预览 = 发布行为的编辑器内等价物。交互设计遵循三条原则：**状态常驻可见**（数据来源模式在工具条常驻）、**副作用安全**（模拟模式不触发真实操作；真实模式下操作类函数有真实副作用并明确警示）、**就近反馈**（每个动作在触发点附近给出结果反馈）。

| 交互点       | 行为                                                                                           |
| ------------ | ---------------------------------------------------------------------------------------------- |
| 进入预览     | 工具条显示「数据来源」开关 + 状态徽标（模拟中=橙 / 真实调用=默认）；autoRun 区块按当前模式执行 |
| 切换数据来源 | 清空全部状态并按新模式重跑 autoRun；toast 提示当前模式语义                                     |
| 模拟模式     | 按函数 outputSchema 动态生成假数据（@faker-js/faker zh_CN），不调用真实函数                    |
| 真实调用失败 | 错误 toast + 引导开启模拟数据                                                                  |
| 表格行操作   | 行尾按钮（danger 红色 + 二次确认）；点击后行字段映射 → 弹窗表单预填                            |
| 选中行       | 写入 selectedRow/selectedRows 运行时状态；触发行选中事件（动作参数按选中行上下文求值）         |
| 弹窗提交     | 模拟=立即成功（假输出）；成功后关窗 + onSuccessRefresh 级联刷新 + 成功 toast                   |

### 4.1 变量名与表达式绑定（V5，已实现）

组件组装的数据层，详见 [V5 设计](./composite-editor-v5-design.md)。使用要点：

- **变量名**：拖入组件自动生成语义名（`player.list` 表格 → `playerListTable`），即发布 spec 区块 key。
  画布卡片以 `⌗varName` 徽标展示；属性面板「变量名」输入框可改名——保存时**同步重写**树内全部表达式/裸引用。
- **表达式**：参数映射/动作链参数/行操作映射的值可填受限表达式
  <code v-pre>{{变量名.路径.字段}}</code>（如 <code v-pre>{{playerListTable.selectedRow.uid}}</code>、<code v-pre>{{filterForm.values.keyword}}</code>、<code v-pre>{{row.uid}}</code>）。
  输入 <code v-pre>{{</code> 弹出变量补全，选中后继续补全 schema 路径；未知变量红标（阻断保存诊断）、路径不在 schema 黄标。
- **运行时状态**：每区块暴露 `data`（函数输出）/ `selectedRow`/`selectedRows`（表格选中）/ `values`（表单当前值，防抖）。
  选中行/输入变化**不触发** refreshOn 自动重跑，只在动作求值时取值。
- **编译**：保存时 <code v-pre>{{var.path}}</code> 编译为现有 wire（`inputAssignments` page_state 路径 / 事件参数 / `row.字段`），
  服务端与发布链零改动；回读按同规则还原为表达式文本。

## 5. 组件开发指南

新组件 = 在 `components/builtin.tsx` 注册 `ComponentDef`：

```tsx
registerComponent({
  type: "statCard", // ComponentType 联合类型加一项
  name: "统计卡",
  icon: <Tag color="orange">统计</Tag>,
  category: "basic", // function=由契约生成 / basic=直接拖入
  allowedChildren: ["fnFields"], // 容器类才配（可选）
  propSchema: ({ nodes, fnById, allFns, fn }) => ({
    type: "object",
    properties: {
      title: { type: "string", title: "标题" },
      span: { type: "integer", minimum: 4, maximum: 24, default: 24 },
      onClick: { type: "object", title: "点击动作", format: "action" }, // 动作编排
    },
  }),
  scaffold: (fn) => ({ title: fn?.id ?? "统计", span: 12 }), // 拖入即骨架
  Preview: ({ node, fn }) => <StatCardPreview node={node} fn={fn} />, // 画布预览
});
```

propSchema 字段渲染约定：

| 声明                   | 渲染                                                                    |
| ---------------------- | ----------------------------------------------------------------------- |
| 普通字段               | rjsf（SchemaFormRenderer）                                              |
| `format: 'columns'`    | Checkbox.Group 列勾选                                                   |
| `format: 'rowActions'` | 行操作编辑器（目标弹窗+参数映射+危险）                                  |
| `format: 'action'`     | 动作编排（主动作下拉+目标；`actionKinds` 限定可选动作；支持后续动作链） |

新组件参与发布：`compiler.ts` 加编译规则 + 快照用例；参与回读：`decompileToTree` 对应分支。

## 6. 代码结构

```
web/src/pages/PageStudio/CompositeEditor/
├── model.ts          # PageNode 树模型 + 纯函数树操作（insert/remove/move/duplicate/pruneDanglingBindings…10 用例）
├── registry.tsx      # ComponentDef 注册表（scaffold/propSchema/Preview/scaffoldProps）
├── compiler/         # 编译器（模块化，2026-09 拆分）
│   ├── types.ts      #   CompiledSection/CompileResult/SpecSectionLike/SECTION_KEY_RE/VIEW_MAP
│   ├── normalize.ts  #   normalizeRowActionParams（行操作参数归一/警告）
│   ├── compile.ts    #   compileTree（编辑树 → sections）
│   ├── decompile.ts  #   decompileToTree（sections → 编辑树，回读）
│   └── index.ts      #   门面（re-export，测试与调用方导入路径不变）
├── components/       # 内置组件定义（2026-09 由单文件拆分）
│   ├── builtin.tsx   #   registerBuiltinComponents 引导 + viewTypeToComponent（门面）
│   └── Fn*.tsx/Button/Modal/Container/Text/StaticForm.tsx + shared.ts（commonFnSchema）
├── ComponentPanel.tsx# 组件面板（函数分组+基础组件+scope 引导）
├── Canvas.tsx        # 画布（ModalPlaceholder / RootDropZone；CanvasNode 从 CanvasNode.tsx re-export）
├── CanvasNode.tsx    # 画布节点装饰（选中/拖拽/右键菜单/容器子级）
├── OutlinePanel.tsx  # 大纲树
├── PropsPanel.tsx    # 属性面板（rjsf + columns/rowActions/action 分区渲染）
├── ActionEditor.tsx  # 动作编排（主动作+链）
├── RowActionsEditor.tsx      # 行操作编辑器
├── previewShared.ts  # 预览共享层（payloadOf/itemsOf/findIn/JSONRecord/StepLike）
├── PreviewRuntime.tsx# 预览运行时引擎（状态/执行/动作分派，=发布形态）
├── PreviewNode.tsx   # 预览渲染子组件（表格/字段卡/表单/ModalForm/StaticFormLive）
├── DataPanel.tsx     # 底部数据试跑面板
├── actions.ts        # ActionSpec/动作注册表
├── useEditorHistory.ts      # 树历史 hook（撤销/重做 50 步 + 统一 setTree 入口 + 快捷键）
├── useCanvasDnd.ts   # 画布拖拽 hook（面板插入/重排/modal 收纳/模板落点）
├── SaveComponentModal.tsx   # 「保存为组件模板」弹窗（表单+参数化候选）
├── InsertTemplateModal.tsx  # 带参模板快速配置弹窗
└── index.tsx         # 编辑器主页（四区布局编排/回读/保存/多选/属性分发）
```

页面工作台（`web/src/pages/PageStudio/index.tsx`）2026-09 同步拆分：`studio/` 子目录承载
6 个抽屉/弹窗组件（PreviewDrawer/EditorModal/VersionsDrawer/ChangeChainDrawer/DiffDrawer/MergeModal）

- draftColumns 列定义 + shared 工具，主页保留列表编排与全部数据回调。

发布链：编译产物 `POST /api/v1/versioning/pages/composite`（请求结构含 `key/group/display/rowActions/toolbarActions/onSuccessRefresh/chain`）→ 提案 → 接受发布 → `PageRenderer/CompositeRenderer` 按 spec 渲染。

## 7. 测试

`__tests__/`：model 10（树操作/悬空绑定清理）、registry 3（注册/约束）、scaffold 8（实例化快照/面板声明）、compiler 11（编译快照/多实例/警告）、decompile 5（回读 round-trip/破损引用）、canvas 3（弹窗占位卡交互）、action-editor 6（动作编排）、integration 4（组合流程）。合计 50 用例，`pnpm --dir web test` 全绿。

**模板可用性与入口（2026-09）**：

- 空白画布自动显示「从模板开始」——列出多区块组合模板（单节点模板不出现，
  组合页需 ≥2 区块），点击即实例化为页面起点；「从空白开始」切换到空白白板
  （根落区即拖放目标，可「查看组合模板」返回引导，双向切换）——2026-09-12
  修复：Canvas 的空树根落区（droppable canvas-root）与外层渲染条件互斥曾是
  死代码，空态拖拽无落区、亦无空白起步入口
- builtin 模板带 stale 检测：契约能力/结构变化后，组件库与模板页标记
  「已过期」，到组件模板页「从契约重新生成」刷新；常量/自定义模板不参与
- 入口分工：组件模板页=管理（导入常量/重新生成/删除）；编辑器组件库=消费（拖选）
- 「保存为组件」四入口（2026-09-12，发现性改进）：顶栏按钮常驻（未多选时禁用
  +Tooltip 教学多选手势；多选后启用并显示计数）、组件库面板头部「从画布选中创建」
  （无选中先提示多选、有选中直接弹保存弹窗）、节点右键菜单「保存为组件」（多选
  集合含该节点时保存整个集合，否则保存单节点子树；仅 CanvasNode——弹窗占位卡
  ModalPlaceholder 无右键菜单，弹窗节点走顶栏多选保存）、模板卡片结构缩略图
  （TemplateThumb 线框：表格=表头+行线/表单=标签行/按钮=圆角块/弹窗=紫框/容器
  =嵌套，宽度按 span 占比，纯 div 不实例化真实组件；组件库与「从模板开始」共用）
- 保存弹窗「更新已有模板」通道（2026-09-12，V3）：保存方式可选「另存新模板」
  （默认，`custom--<ts>` 新 key）或「更新已有模板」——后者拉取当前 scope 自定义
  模板下拉（builtin 不列：内置模板改版走组件模板页「从契约重新生成」，且后端
  Update 会强置 builtin=false），选中后名称/分类/描述预填，提交走
  `PUT /api/v1/component-templates/:key` 以当前画布选择覆盖结构/参数/依赖函数

**自动生成模板清单（2026-09）**：组件模板「从契约重新生成」现产出三类内置模板——

1. `fn--<fid>`：单函数组件（collection_query→表格、item_query→字段卡、其余→表单）
2. `crud--<resource>`：资源管理组合（列表+详情+增改弹窗，onSuccess 自动刷新）
3. `query--<fid>`：查询组合（查询条件表单 + 结果表格，经 refreshOnNode 引用在
   实例化后解析为区块 key，键位漂移安全）——带查询参数的 collection_query 自动生成

非函数类需配置生成：常量表单（staticForm，Excel/JSON 导入常量）、用户自定义组合
（画布多选保存）。暂不可自动生成（需新组件类型，backlog）：任务监控组合
（taskStatus 节点）、报表图表（chart 节点）、批量选择操作。

**常量表单（staticForm，2026-09）**：基础组件新增「常量表单」——不绑定函数，

> 组合原语与表达力边界的总纲见 [组合模型与表达力边界](../architecture/composition-model.md)。
> 字段在设计期以 JSON Schema 定义（属性面板支持在线编辑与 JSON/Excel 导入选项，
> 第 1 列=值、第 2 列=标签）。画布/预览/发布均渲染真实控件（enum→下拉），
> 值防抖并入页面状态驱动 refreshOn 联动下游。可保存为组件模板复用
> （「变量下拉框」场景的标准做法）。发布校验：static 区块禁带 bindingId。

**常量导入规范（2026-09 修订）**：一种常量 = 一个独立组件模板——导入弹窗不再
填写模板名称，逐常量生成单下拉 staticForm 模板（key `consts--<batch>-<i>`，
按常量名命名），组合页中自由拖选数量与位置；基础组件列表中的「常量表单」入口
已移除，创建统一走组件模板页「导入常量」。组件模板页配套：

- **旧版合并模板检测**：页面自动识别「一个 staticForm 塞多个常量」的历史模板
  （key `consts--<batch>` 时代的数据），提供一键清理，清理后重新导入即可
- **生成示例常量**：一键创建 4 个示例常量组件（封禁原因/会员等级/服务器状态/
  支付渠道，key `consts--demo-*`，重复点击幂等跳过），便于无数据环境体验；
  后端等价接口 `POST /api/v1/component-templates/seed-demo-constants` 可直接
  curl 灌入（无需前端重建）

**预览态交互（2026-09 修订）**：编辑器预览与模板预览弹窗中的 staticForm 是
**可交互的真实控件**（与发布渲染同一 rjsf 运行时，`StaticFormLive`）——下拉/
输入可操作，值防抖（300ms）并入预览页面状态。已知边界：预览内 refreshOn
对 staticForm 值的级联刷新尚未接线（画布设计态预览仍为静态渲染，避免与
拖拽手势冲突）。

V4 新增文件：`ComponentLibrary.tsx`（组件库面板——模板浏览/实例化/id 重映射/scope 检查）、`types.ts`（共享类型）。

**模板拖放（2026-09）**：组件库 Tab 的模板卡片是 dnd-kit 拖拽源——可拖入画布任意落点
（根级末尾/节点之后链式插入/容器内），落点规则与函数组件一致；拖放与点击插入共用
`instantiateTemplate`（id 重分配 + 内部引用重映射）。落点决策抽为纯函数
`templateDrop.ts#planTemplateDrop`（含 V1 弹窗仅 fnForm 边界），7 个单测覆盖。缺依赖
函数的模板拖入时在落点处警告并放弃（不静默失败）。

## 8. V4 展望：组件模板与三层组合

V3 之上已上线**组件模板层 V4**（组件库面板实例化 + 选中节点保存为组件 + 契约自动生成模板），详见
[组合页编辑器 V4 设计](./composite-editor-v4-design.md)。

## 8.5 Selector 一键同步（契约漂移修复，2026-09）

函数契约 schema 变化后页面绑定 stale（发布 422 阻断 / console 409 拒绝执行）时，
除「重生成」（整页替换、定制冲掉）与手动逐 binding 重选外，第三条路径是
**一键同步 Selector**：只修受影响的 assignment，保留全部未受影响定制
（form/row/selection/page_state/literal 来源与 Transform）。

**三个入口**（共用 `SelectorSyncReportModal`，打开即 dry-run 展示计划）：

- 契约变更收件箱（Proposal Inbox → 契约变更队列）行操作「更多 → 一键同步 Selector」
- 运行控制台 stale 提示条上的「同步 Selector」按钮（发布页被 409 阻断时）
- Page Studio 编辑器 stale 警告条上的「同步 Selector」按钮

报告按 binding 分组，逐条标注动作（kept 保留 / renamed 重映射 / removed 摘除 /
added 补齐 / type_changed 类型变化 / shape_updated 形状更新 / manual_required
需人工处理）与置信度（精确匹配=prev schema 命中 / 启发式）。确认后「应用同步
到草稿」生成新版本（revision+1 + 版本记录 + 审计），**不自动发布**——报告底部
提示剩余错误级诊断，处理完 manual_required 项后手动发布。

composite 页边界：新增 required 输入一律 `manual_required`（composite 输入只应
来自 page_state/literal，不自动补 form），需在编辑器里手动加参数映射。策略阶梯、
prev schema 语义与 wire 契约见
[Dashboard Resource/Page 模型](../architecture/dashboard-page-model.md)与
[PageSpec 协议规范](../architecture/pagespec-protocol.md)。

## 9. 已知边界

- **页签容器（V2）**：页签内组件的细粒度画布交互（拖拽排序/调宽/右键菜单）不生效，
  请用左侧大纲面板选中与删除（大纲树已递归全深，页签页内组件可见可选）；页签嵌套
  （tabs 进 tabs/页内再放 tabs）不支持；页内区块发布为整行堆叠（span 不生效）；
  预览里切换页签不回写编辑树（仅编辑态页签头切换记录激活页）；
  staticForm 不能经编辑器拖入页签（wire 支持 `display=tab` 的 static 区块，回读可还原）
- 容器子级两层内完整交互（孙层为简化预览）
- 文本组件不参与发布（编译警告）；弹窗/页签页内 text 组件不进 spec
- 回读依赖提案或 draft 至少其一存在（三者都无则提示）
- container 的 click 事件无独立 section 挂载点（预览可用，发布忽略）
- 预览「模拟数据」开关**默认开启**：按函数 outputSchema 动态生成假数据
  （@faker-js/faker zh_CN，字段名启发式映射 uid/昵称/时间/手机号等），
  不调用真实函数——预览是编辑器内安全环境，默认不对 agent 发起真实调用
  （scope 不匹配时会得到 no live agent 之类的真实路由错误）；
  显式切到「真实数据」才走发布运行时（操作类函数有真实副作用）。
  无 outputSchema 或顶层非 object 结构时**模拟数据为空**（每节点提示一次），
  不伪造数据——与发布端真实调用为空的行为一致
- 编译警告（不再静默）：行操作参数的嵌套行路径
  （<code v-pre>{{row.a.b}}</code>，发布端只支持单段 row.字段）按字面量保存并警告；
  参数映射的未知 kind / 来源节点失效 → 警告并跳过该映射（不再静默归 page_state 或丢弃）
- 常量表单（staticForm）区块按画布顺序落库（2026-09 修订：服务端此前把 static
  统一追加到 sections 末尾，画布首位的常量筛选发布后会漂移到页尾）
- V5 表达式：预览运行时（编辑器内）尚未接入表达式参数求值（发布渲染器完整支持）；
  行操作参数仅支持 <code v-pre>{{row.字段}}</code>（跨变量表达式编译警告、按字面量保留）；
  模板混排文案（<code v-pre>"玩家 {{row.uid}} 已处理"</code>）属 V5.1 未实现
- V5-T5.7 端到端线上验收（提案→发布→真实联动）待执行
- Selector 一键同步（8.5 节）边界：prev schema 只存一版且仅在本功能上线后的下一次
  契约更新才写入，存量漂移页首次同步走启发式（confidence=low）；多跳漂移
  （发布后又改契约）digest 判定不符时同样降级；composite 页新增 required 输入
  一律 manual_required；新 required 字段补 form 要求页面表单有同名字段，且
  resource 语义盲区（新 required 恰为 identity 字段时应来自 row 源）只在 reason
  里提示核对、不自动推断；同步不自动 publish
