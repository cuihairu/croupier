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
| 添加           | 点击（根末尾）/ 拖拽（**落点指示线**；拖到容器上进 children；拖到弹窗占位卡进弹窗）                                                                         |
| 同函数多实例   | 同一函数可拖 N 个组件（key 自动 `fid`/`fid-2`…），分别配置                                                                                                  |
| 换绑           | 属性面板「函数（可换绑）」全量下拉；换绑后组件重新 scaffold                                                                                                 |
| 画布           | 拖拽排序、右缘调宽（4-24 栅格）、**右键菜单**（上移/下移/选择父容器/复制/删除）、容器子级点选/删除/同级移动                                                 |
| 多选           | 点选累积（高亮+计数）→ 顶栏「删除所选」批量删除                                                                                                             |
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

## 3. 编译规则（编辑树 → CompositeSection）

| 画布                                | 发布 spec                                                            |
| ----------------------------------- | -------------------------------------------------------------------- |
| fnTable / fnFields / fnForm(inline) | 区块（view=table/fields/form）                                       |
| 弹窗容器（modal）                   | 其每个函数子组件 → `display: dialog` + `group: modal-<id>`（同弹窗） |
| 表格属性「行操作」                  | `table.rowActions`（目标=group；行字段→参数映射；链透传）            |
| 独立按钮（置于表格后）              | 该表格 `toolbar.actions`；非弹窗动作（执行/刷新）发布为 `chain`      |
| fnForm「成功后刷新」                | `onSuccessRefresh`（目标=区块 key）                                  |
| 容器                                | 子节点平铺（span 各自保留）                                          |
| 文本                                | 不发布（警告）                                                       |

警告场景（保存时提示，不阻断）：空弹窗、动作目标已删、按钮不在任何表格之后、区块未绑定函数、文本组件。

## 4. 发布页行为（PageRenderer/CompositeRenderer）

- `autoRun` 区块进入页面自动执行；`refreshOn` 上游产出自动重跑（page_state 同名字段合并）
- 行操作列/顶部按钮 → 打开 group 对应弹窗（参数预填，danger 二次确认）→ 提交成功关窗+提示 → `onSuccessRefresh` 级联刷新
- `chain` 步骤在主动作后按序执行（runBinding/refreshNode）
- 同函数多实例按 key 独立执行互不干扰

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
├── registry.tsx      # ComponentDef 注册表（scaffold/propSchema/Preview）
├── components/builtin.tsx   # 七个内置组件定义
├── ComponentPanel.tsx# 组件面板（函数分组+基础组件+scope 引导）
├── Canvas.tsx        # 画布（CanvasNode 装饰+右键菜单 / ModalPlaceholder / RootDropZone）
├── OutlinePanel.tsx  # 大纲树
├── PropsPanel.tsx    # 属性面板（rjsf + columns/rowActions/action 分区渲染）
├── ActionEditor.tsx  # 动作编排（主动作+链）
├── RowActionsEditor.tsx      # 行操作编辑器
├── PreviewRuntime.tsx# 预览运行时（=发布形态）
├── DataPanel.tsx     # 底部数据试跑面板
├── actions.ts        # ActionSpec/动作注册表
├── compiler.ts       # 编译（树→sections）+ 反编译（sections→树，回读）
└── index.tsx         # 编辑器主页（四区布局/拖拽域/撤销重做/多选/保存/回读）
```

发布链：编译产物 `POST /api/v1/versioning/pages/composite`（请求结构含 `key/group/display/rowActions/toolbarActions/onSuccessRefresh/chain`）→ 提案 → 接受发布 → `PageRenderer/CompositeRenderer` 按 spec 渲染。

## 7. 测试

`__tests__/`：model 10（树操作/悬空绑定清理）、registry 3（注册/约束）、scaffold 8（实例化快照/面板声明）、compiler 11（编译快照/多实例/警告）、decompile 5（回读 round-trip/破损引用）、canvas 3（弹窗占位卡交互）、action-editor 6（动作编排）、integration 4（组合流程）。合计 50 用例，`pnpm --dir web test` 全绿。

**自动生成模板清单（2026-09）**：组件模板「从契约重新生成」现产出三类内置模板——

1. `fn--<fid>`：单函数组件（collection_query→表格、item_query→字段卡、其余→表单）
2. `crud--<resource>`：资源管理组合（列表+详情+增改弹窗，onSuccess 自动刷新）
3. `query--<fid>`：查询组合（查询条件表单 + 结果表格，经 refreshOnNode 引用在
   实例化后解析为区块 key，键位漂移安全）——带查询参数的 collection_query 自动生成

非函数类需配置生成：常量表单（staticForm，Excel/JSON 导入常量）、用户自定义组合
（画布多选保存）。暂不可自动生成（需新组件类型，backlog）：任务监控组合
（taskStatus 节点）、报表图表（chart 节点）、批量选择操作。

**常量表单（staticForm，2026-09）**：基础组件新增「常量表单」——不绑定函数，
字段在设计期以 JSON Schema 定义（属性面板支持在线编辑与 JSON/Excel 导入选项，
第 1 列=值、第 2 列=标签）。画布/预览/发布均渲染真实控件（enum→下拉），
值防抖并入页面状态驱动 refreshOn 联动下游。可保存为组件模板复用
（「变量下拉框」场景的标准做法）。发布校验：static 区块禁带 bindingId。

V4 新增文件：`ComponentLibrary.tsx`（组件库面板——模板浏览/实例化/id 重映射/scope 检查）、`types.ts`（共享类型）。

**模板拖放（2026-09）**：组件库 Tab 的模板卡片是 dnd-kit 拖拽源——可拖入画布任意落点
（根级末尾/节点之后链式插入/容器内），落点规则与函数组件一致；拖放与点击插入共用
`instantiateTemplate`（id 重分配 + 内部引用重映射）。落点决策抽为纯函数
`templateDrop.ts#planTemplateDrop`（含 V1 弹窗仅 fnForm 边界），7 个单测覆盖。缺依赖
函数的模板拖入时在落点处警告并放弃（不静默失败）。

## 8. V4 展望：组件模板与三层组合

V3 之上已上线**组件模板层 V4**（组件库面板实例化 + 选中节点保存为组件 + 契约自动生成模板），详见
[组合页编辑器 V4 设计](./composite-editor-v4-design.md)。

## 9. 已知边界

- 容器子级两层内完整交互（孙层为简化预览）
- 文本组件不参与发布（编译警告）；弹窗内 text 组件不进 spec
- 回读依赖提案或 draft 至少其一存在（三者都无则提示）
- container 的 click 事件无独立 section 挂载点（预览可用，发布忽略）
