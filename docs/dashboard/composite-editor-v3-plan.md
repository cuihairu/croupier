---
title: 组合页编辑器 V3（组件化重构）— 原子任务清单
---

# 组合页编辑器 V3 — 原子任务清单

状态：待执行（已评审方向：方案 B——自研，抄 amis-editor/Appsmith 机制，复用 antd/rjsf 资产）
参考样品：amis-editor（http://192.168.5.5:8001）、Appsmith（http://192.168.5.5:8002）
拆分原则：每任务独立可开发、可验证、可单独提交；标注依赖与验收。

## 核心决策（不再变更）

1. **一切皆组件**：Button/Modal/Container 是独立组件（Appsmith 模型），不再把按钮藏在表格属性里。
2. **抄机制不抄代码**：scaffold/属性面板 schema 声明（amis panelControls）、事件动作下拉编排（Appsmith onClick）。
3. **属性面板用 rjsf 渲染**：每个组件声明 propSchema（JSON Schema），面板 = 现有 SchemaFormRenderer，零手写表单。
4. **编辑器内部是组件树，保存时编译为现有平铺 CompositePageSpec**——发布链（提案/校验/版本/菜单/CompositeRenderer）零改动。
5. **V1 边界（明确不做）**：无限嵌套（Container 只一层）、Modal 内多组件（只装一个 fnForm）、撤销重做、多选/右键菜单、动作链。列入 V1.1。

## 页面树模型（编辑视图）

```ts
type PageNode = {
  id: string; // nanoid
  type:
    | "fnTable"
    | "fnForm"
    | "fnFields"
    | "button"
    | "modal"
    | "container"
    | "text";
  props: Record<string, unknown>; // 含事件绑定 props.onClick/onSuccess
  children?: PageNode[]; // 仅 container/modal 可有
};
type ActionSpec =
  | { kind: "openModal"; target: string } // modal 节点 id
  | { kind: "runBinding"; target: string } // fnForm/fnTable 节点 id
  | { kind: "refreshNode"; target: string };
```

## 批次总览

| 批次 | 目标                                    | 任务数    | 依赖 |
| ---- | --------------------------------------- | --------- | ---- |
| P0   | 地基：树模型/注册表/布局骨架/scope 引导 | T0.1–T0.5 | 无   |
| P1   | 组件面板 + 属性面板（7 个组件定义）     | T1.1–T1.9 | P0   |
| P2   | 画布：拖入/选中/移动/嵌套/大纲          | T2.1–T2.7 | P1   |
| P3   | 事件动作系统 + 预览                     | T3.1–T3.5 | P2   |
| P4   | 编译保存 + 发布对齐 + 验收门            | T4.1–T4.5 | P3   |
| P5   | 收尾：文档/清理/回归                    | T5.1–T5.3 | P4   |

关键路径：T0.2→T0.3→T1.2→T2.1→T2.2→T3.2→T3.3→T4.1→T4.5

---

## 批次 P0 — 地基

### ✅ T0.1 编辑器空态 scope 引导

- 文件：`web/src/pages/PageStudio/CompositeEditor/FunctionPanel.tsx`
- 改动：descriptors 为空时显示当前 scope（`getScope()`）+ 调 `getMyGames()` 列出其他 scope 的函数计数，提供一键 `setScope` 切换按钮（修已实测的"死页面"：函数全在 demo_game/development，用户停在 default/prod 时 0 函数无提示）
- 验收：default/prod 下打开编辑器，空态显示"当前 scope 无函数契约，demo_game/development 有 22 个 → [切换]"，点击后左栏出函数
- 依赖：无 ｜ 预估：0.25d

### T0.2 PageNode 树模型与类型

- 文件：`web/src/pages/PageStudio/CompositeEditor/model.ts`（新）
- 改动：`PageNode`/`ActionSpec`/`ComponentType` 类型；树工具函数（insert/remove/move/find/updateProps/duplicate，全部纯函数）
- 验收：`pnpm --dir web test` 新增 model.test.ts 用例（插/删/移/查/复制含子树）全绿
- 依赖：无 ｜ 预估：0.25d

### T0.3 ComponentRegistry 接口与骨架

- 文件：`web/src/pages/PageStudio/CompositeEditor/registry.tsx`（新）
- 改动：组件定义接口 `ComponentDef { type; name; icon; category; allowedParents?; allowedChildren?; propSchema: JSONSchema; scaffold(fn?): PageNode['props']; Preview: React.FC }` + 空注册表 `registerComponent`/`getComponent`
- 验收：类型检查通过；registry 单测（注册/获取/重复注册报错）
- 依赖：T0.2 ｜ 预估：0.25d

### T0.4 编辑器四区布局骨架

- 文件：`web/src/pages/PageStudio/CompositeEditor/index.tsx`（重构）
- 改动：页面骨架 = 顶栏（标题/pageKey/预览切换/保存）+ 左侧 Tabs（组件面板/大纲）+ 画布 + 右侧属性面板；页面状态从 sections 平铺改为 `tree: PageNode[]` + `selectedId`（本次只搭骨架，画布渲染 P2 接管）
- 验收：路由可开、四区可见（空态）、tsc 通过
- 依赖：T0.2 ｜ 预估：0.25d

### T0.5 移除旧编辑器残留

- 文件：删除 `SectionCard.tsx`/`SectionPreview.tsx`/`TryRunPanel.tsx`/旧 `Inspector.tsx`、`types.ts` 中平铺模型；ProposalInbox 入口保持跳转新编辑器
- 验收：`rg -n "SectionDraft" web/src` 无结果；jest/tsc 全绿
- 依赖：T0.4 ｜ 预估：0.1d

---

## 批次 P1 — 组件面板与属性面板

### T1.1 属性面板渲染器（rjsf 驱动）

- 文件：`web/src/pages/PageStudio/CompositeEditor/PropsPanel.tsx`（新）
- 改动：读选中节点 `ComponentDef.propSchema` → `SchemaFormRenderer` 渲染；onChange 写回 `node.props`；无选中显示空态。propSchema 内约定字段（`title` 文案、`span` 数字、事件字段用 ui:widget 标记，事件编辑器 T3.2 替换实现）
- 验收：临时给 fnTable 注册含 title/span 的 propSchema，选中后可改标题并回显画布
- 依赖：T0.3 T0.4 ｜ 预估：0.5d

### T1.2 fnTable 组件定义

- 文件：`web/src/pages/PageStudio/CompositeEditor/components/fnTable.tsx`（新目录）
- 改动：propSchema（functionId 只读、标题、列多选 enum 来自 outputSchema、行操作配置、autoRun、span）；scaffold(fn) 默认列=输出 schema 全选；Preview = antd Table（列头真实、数据空态）
- 验收：注册后面板可见；scaffold 用 inventory.list 契约生成正确列
- 依赖：T0.3 T1.1 ｜ 预估：0.5d

### T1.3 fnForm 组件定义

- 文件：`.../components/fnForm.tsx`
- 改动：propSchema（functionId、标题、展示方式 inline/dialog、成功后刷新）；scaffold(fn) 字段=输入 schema；Preview = rjsf 表单（编辑态禁用交互）
- 验收：inventory.grant scaffold 出 playerId/templateId 必填表单
- 依赖：T0.3 T1.1 ｜ 预估：0.5d

### T1.4 fnFields 组件定义

- 文件：`.../components/fnFields.tsx`
- 改动：propSchema 同 fnTable 简化；Preview = Descriptions（键=输出 schema）
- 验收：注册+预览正确
- 依赖：T1.2（复用列提取工具）｜ 预估：0.25d

### T1.5 button 组件定义

- 文件：`.../components/button.tsx`
- 改动：propSchema（文案、样式 primary/danger、onClick 动作占位字段）；Preview = antd Button
- 验收：注册+画布预览
- 依赖：T0.3 T1.1 ｜ 预估：0.25d

### T1.6 modal 容器定义

- 文件：`.../components/modal.tsx`
- 改动：allowedChildren=['fnForm']；propSchema（标题、宽度）；Preview = 画布收纳卡片（弹窗形态不占栅格，amis 悬浮预览 V1.1）
- 验收：面板可拖入/点击加入；children 约束生效（加入非 fnForm 被拒）
- 依赖：T0.3 ｜ 预估：0.25d

### T1.7 container 与 text 定义

- 文件：`.../components/container.tsx`、`.../components/text.tsx`
- 改动：container allowedChildren=全部（一层）；text propSchema（内容、层级 h2/h3/p）
- 验收：注册+预览
- 依赖：T0.3 ｜ 预估：0.25d

### T1.8 组件面板 UI

- 文件：`.../ComponentPanel.tsx`（新）
- 改动：分两组——「函数组件」：拉 listDescriptors 按资源分组列出（每函数旁标注将生成的组件类型，点击=用契约 scaffold 加入）；「基础组件」：button/modal/container/text 网格（点击加入，amis 式 scaffold）
- 验收：demo_game scope 下 22 个函数分组可见；点击 inventory.list 画布出现 fnTable
- 依赖：T1.2–T1.7 ｜ 预估：0.5d

### T1.9 scaffold/propSchema 单测固化

- 文件：`.../components/__tests__/scaffold.test.tsx`
- 改动：用 mock 契约（list/get/form 三形态）固化每个组件 scaffold 输出形状
- 验收：jest 全绿；后续改 scaffold 必须改测试（防回归）
- 依赖：T1.2–T1.7 ｜ 预估：0.25d

---

## 批次 P2 — 画布

### T2.1 画布树渲染与选中

- 文件：`.../Canvas.tsx`（新）
- 改动：递归渲染 PageNode（container→Row/Col 嵌套一层；modal 收纳区）；选中高亮/点选；Preview 按 registry 渲染
- 验收：含 container+fnTable+button+modal 的树正确渲染；点选联动属性面板
- 依赖：T0.4 T1.8 ｜ 预估：0.5d

### T2.2 面板→画布拖入（落点指示）

- 文件：`.../Canvas.tsx`、`.../ComponentPanel.tsx`
- 改动：dnd-kit 单一 DndContext；面板项 draggable（携带组件类型/函数 id）；画布 droppable 显示插入位置指示线（closestCenter 计算同级 index）
- 验收：拖 fnTable 到 container 内指定位置生效；拖 modal 显示到收纳区
- 依赖：T2.1 ｜ 预估：0.5d

### T2.3 画布内移动排序

- 改动：节点拖拽手柄；同级重排（复用 SortableList externalDnd 模式或直接 dnd-kit）
- 验收：拖动 fnTable 越过 button 顺序变化且树结构正确
- 依赖：T2.2 ｜ 预估：0.25d

### T2.4 边缘拖拽调宽

- 改动：选中节点右缘 col-resize 手柄（px→24 栅格）；写回 props.span
- 验收：拖动宽度即时变化，最小 4 最大 24
- 依赖：T2.1 ｜ 预估：0.25d

### T2.5 大纲树面板

- 文件：`.../OutlinePanel.tsx`（新）
- 改动：左侧 Tabs 第二页；antd Tree 映射 PageNode 树；点击定位选中、拖拽排序联动画布
- 验收：大纲点击/画布点选双向同步；modal 子节点可见
- 依赖：T2.1 ｜ 预估：0.25d

### T2.6 删除/复制节点

- 改动：选中节点悬浮操作条（删除/复制，含子树；事件引用目标被删时属性面板标红）
- 验收：删除 modal 后引用它的 button.onClick 显示"目标已删除"警示
- 依赖：T2.1 ｜ 预估：0.25d

### T2.7 画布批次验收（门）

- 手工：搭"标题+container(表格+按钮)+modal(表单)"结构全部交互可用
- 验收：录屏/截图存 `docs/dashboard/editor-v3-acceptance/`；tsc/jest 绿
- 依赖：T2.1–T2.6 ｜ 预估：0.25d

---

## 批次 P3 — 事件动作 + 预览

### T3.1 ActionSpec 与动作注册表

- 文件：`.../actions.ts`（新）
- 改动：ActionSpec 类型 + `ACTIONS: { kind; label; 目标选择器过滤 }` 注册表（openModal→modal 节点、runBinding→fn* 节点、refreshNode→fn* 节点）
- 验收：类型测试
- 依赖：T0.2 ｜ 预估：0.25d

### T3.2 属性面板动作编辑器

- 文件：`.../PropsPanel.tsx` 扩展
- 改动：propSchema 中 `format: 'action'` 字段渲染为动作卡片（动作类型下拉 + 目标节点下拉——目标按注册表过滤）；支持单动作（V1 不做链）
- 验收：button.onClick 配"打开弹窗[modal-1]"；fnForm.onSuccess 配"刷新[fnTable-1]"
- 依赖：T3.1 T1.1 ｜ 预估：0.5d

### T3.3 动作执行器（预览态）

- 文件：`.../previewRuntime.tsx`（新）
- 改动：预览模式执行树——autoRun fnTable 自动 invokeFunction；openModal 打开 Modal 渲染其 fnForm（rjsf 真实提交）；runBinding 执行；refreshNode 重跑；执行结果注入节点（表格显示真实行）
- 验收：预览态表格出数、按钮开弹窗、提交成功刷新表格
- 依赖：T3.2 T2.1 ｜ 预估：0.5d

### T3.4 预览模式接线

- 改动：顶栏"预览"切到 previewRuntime 渲染整树（隐藏编辑装饰与左右面板）
- 验收：与 T3.3 同场景在整页预览走通
- 依赖：T3.3 ｜ 预估：0.25d

### T3.5 事件批次验收（门）

- 手工：完整用户旅程——玩家表格(自动执行) + 行尾按钮(封禁) + 顶部按钮(发邮件→弹窗) + 提交后刷新表格，全程预览态无 JS 报错
- 验收：录屏存 acceptance 目录
- 依赖：T3.2–T3.4 ｜ 预估：0.25d

---

## 批次 P4 — 编译保存与发布对齐

### T4.1 树→CompositeSection 编译器

- 文件：`.../compiler.ts`（新）+ 单测
- 改动：modal+fnForm → `display=dialog` section；button.onClick.openModal 且按钮在表格行 → `rowActions`（参数映射列→弹窗表单参数）；按钮独立于表格 → 表格 `toolbarActions`（V1 约束：独立按钮编译为最近表格的顶部按钮，无表格则提示）；fnForm.onSuccess → `onSuccessRefresh`；container/text → span 组合（text 编译为区块标题行——V1 降级为"忽略并警告"亦可，按实现定）；pageKey 推导沿用
- 验收：compiler.test.ts 固化三种典型树→spec 快照
- 依赖：T3.2 ｜ 预估：0.5d

### T4.2 保存创建提案

- 改动：保存按钮 → 编译 → POST `/api/v1/versioning/pages/composite`（现有端点，payload 已支持 display/rowActions/toolbarActions/onSuccessRefresh）；错误回显
- 验收：保存后提案收件箱出现提案，spec 内容与编译快照一致
- 依赖：T4.1 ｜ 预估：0.25d

### T4.3 发布渲染器独立按钮支持（如需）

- 文件：`web/src/components/PageRenderer/index.tsx`、后端 spec（若引入独立按钮行）
- 改动：仅当 T4.1 选择"独立按钮不降级"时：spec 增 `ButtonBar`（页面级按钮组区块），渲染器+generator 透传；否则跳过本任务
- 验收：发布页按钮位置与预览一致
- 依赖：T4.1（按其结论）｜ 预估：0.5d｜可跳过

### T4.4 回读编辑（V1.1 预留）

- 改动：已发布/提案 spec → 反编译为树（编辑已有页面）
- 验收：——本版本**不做**，占位记录
- 依赖：— ｜ 预估：0（V1.1）

### T4.5 端到端验收门（发布链路）

- 手工：编辑器搭玩家管理页 → 保存 → 提案收件箱接受发布 → 左侧菜单出现 → 发布页验证：表格自动执行、行操作弹窗、提交刷新、审批提示（危险函数）
- 验收：线上（192.168.5.5）全流程录屏；数据落 ClickHouse（bridge 归流可见 function.call）
- 依赖：T4.2 T4.3 ｜ 预估：0.25d

---

## 批次 P5 — 收尾

### T5.1 使用与扩展文档

- 文件：`docs/dashboard/composite-editor-v3.md`（新）：使用指南（含参考样品对照说明）+ 新组件开发指南（registry 约定）
- 验收：docs-link 检查通过
- 依赖：T4.5 ｜ 预估：0.25d

### T5.2 旧路由与死代码清理

- 改动：旧编辑器路由重定向到新版；`rg` 确认无残留组件/类型
- 验收：`rg -n "CompositeBuilder|SectionDraft" web/src` 无结果
- 依赖：T5.1 ｜ 预估：0.1d

### T5.3 全量回归与部署

- 验收：`pnpm --dir web test`、`pnpm --dir web run tsc`、`go test ./internal/...` 全绿；Docker 构建 + deploy-self-hosted 部署；线上冒烟（编辑器可开、保存链路通）
- 依赖：T5.2 ｜ 预估：0.25d

---

## 汇总

- 任务数：24（含 3 个验收门；T4.3 条件任务、T4.4 V1.1 占位不计）
- 工作量：P0 ≈ 1.1d ｜ P1 ≈ 3d ｜ P2 ≈ 2.25d ｜ P3 ≈ 1.75d ｜ P4 ≈ 1.5d ｜ P5 ≈ 0.6d，合计 ≈ 10.2d
- 关键路径：T0.2→T0.3→T1.1→T1.2→T2.1→T2.2→T3.2→T3.3→T4.1→T4.2→T4.5
- 风险项：rjsf 对动态 enum（列选择）的 ui-schema 适配（T1.1 预留半天）；编译器降级规则需在 T4.1 实现时定稿

## 附：样品对照索引（实现时随时回看）

| 机制                     | amis-editor 体现                    | Appsmith 体现                  | 对应任务  |
| ------------------------ | ----------------------------------- | ------------------------------ | --------- |
| 拖入即实例化（scaffold） | 组件面板拖入生成骨架 JSON           | widget 拖入画布                | T1.8/T2.2 |
| 属性面板 schema 声明     | panelControls（amis schema 自描述） | Property pane 分属性/样式/事件 | T1.1      |
| 事件动作下拉编排         | 动作链（dialog/refresh/...）        | onClick=open modal/run query   | T3.2      |
| Modal 一等组件           | dialog 容器                         | ModalWidget                    | T1.6      |
| 大纲树                   | 大纲面板                            | Widget 树                      | T2.5      |
| 预览=发布                | preview 模式同引擎                  | 编辑画布即真实 widget          | T3.3/T3.4 |
