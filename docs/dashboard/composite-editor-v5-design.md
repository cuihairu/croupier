---
title: 组合页编辑器 V5 — 变量名与表达式绑定（组件组装数据层）
---

# 组合页编辑器 V5 — 变量名与表达式绑定

> 状态：**设计评审中**（尚未实现）
> 前置：V3/V3.1（[组件化编辑器](./composite-editor-v3.md)）、V4（[组件模板三层组合](./composite-editor-v4-design.md)）已上线
> 参考产品：Appsmith（<code v-pre>{{Table1.selectedRow.uid}}</code>）、amis-editor（插值选择器）、Retool（组件命名引用）
> 本文回答两个悬而未决的问题：**拖放出来的组件如何定义变量名**、**组件组装时数据怎么绑定**。

## 1. 问题定义：为什么现在"组装不起来"

当前编辑器可以把组件拖到画布上并连线（事件/动作链），但**数据层没有名字体系**，组装时的四个核心问题没有答案：

| 问题                      | 现状                                                                       | 后果                                               |
| ------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------- |
| 拖入的组件叫什么？        | 只有内部 `nanoid`（`n-xxx`），用户不可见                                   | 绑定参数时只能面对"区块 key"下拉，无法确认指向谁   |
| 参数怎么引用别的组件？    | `resolveStepParams` 只支持 `区块key.字段` **一层点路径**                   | 嵌套取值（`selectedRow.uid`）写不出；没有语法定义  |
| 表格选中的行叫什么？      | 运行时表格**不保存 selectedRow 状态**（仅行操作按钮用固定映射 `row.字段`） | "选中一行 → 表单预填该行"这类最常见组装做不到      |
| 改名/删组件后引用怎么办？ | 无变量名概念，引用就是 key 字符串                                          | 删节点后绑定静默失效（仅保存时警告），无系统化诊断 |

V5 的目标：给每个拖入的组件一个**可读变量名**，给参数/动作绑定一套**受限表达式语法**，并让表达式**编译回现有发布 spec**——服务端、发布链、PageSpec 协议零改动。

## 2. 核心决策（不再变更）

- **D1 变量名 == 发布 spec 的区块 key**：变量名不引入新概念，就是 `CompositeSection.key`。key 的页面内唯一性约束已存在，天然满足变量唯一。
- **D2 表达式只存在于编辑器层**：保存（编译）时把 <code v-pre>{{expr}}</code> 编译为现有 wire 字段（`inputAssignments` / 事件 `params` / `chain` 参数）。**服务端校验、提案、发布、PageSpec 协议全部不变**——没有新 wire 字段。
- **D3 受限路径表达式，不做 JS eval**：只允许变量引用 + 属性路径 + 数组下标。不做函数调用、运算符、计算属性（V5 边界，见 §9）。
- **D4 交互 = 表达式输入 + 自动补全**：属性面板的参数映射从"下拉点选"升级为"表达式输入框 + 光标处插入变量 + 路径自动补全"；原下拉保留为快捷入口（点选自动生成表达式）。
- **D5 运行时状态按需解析**：每区块的运行时状态扩展为 `{ data, selectedRow, selectedRows, values }`；选中行/表单值的变化**不触发** refreshOn 自动重跑（避免选中即风暴），只在动作执行、提交、参数求值时取值。

## 3. 变量名模型

### 3.1 生成规则（拖入时自动命名）

约束：`[a-z][a-zA-Z0-9]*`（camelCase，ASCII；变量名要进表达式，禁止中文/点号/空格）。

```text
函数组件：camelCase(资源名) + Camel(操作名) + 组件类型后缀
  player.list  表格 → playerListTable
  player.list  表格 → playerListTable2      （同函数再拖一次，数字后缀去重）
  mail.send    表单 → mailSendForm
  player.get   字段卡 → playerGetFields
基础组件：
  弹窗 → sendMailModal（取标题 camelize，无标题则 modal2）
  按钮 → grantButton；容器 → container2；文本 → text3
静态表单（staticForm）：camelCase(标题) + Form，无标题则 filterForm / staticForm2
```

去重：以页面变量集合为准，冲突追加最小可用数字后缀（`2`、`3`…）。

### 3.2 改名与引用同步

- 属性面板顶部新增「变量名」输入框（校验格式 + 唯一性，非法即红标不写入）。
- 改名成功时**同步重写**编辑器树内全部表达式引用（按词边界替换 `oldName` → `newName`，含链参数、行操作映射、onSuccessRefresh、refreshOn）。
- 改名后画布 hover、大纲、下拉补全均显示新名。
- 注意边界：**已发布页面改名后再次保存**，发布 spec 中区块 key 变化（`refreshOn`/`chain`/动作目标同步重写，编译器负责）；页面 key（菜单/路由标识）与区块变量名无关，不受影响。

### 3.3 回读兼容（旧页面没有变量名）

- 已有发布 spec 的 key 可能是 `player.list`（含点）等**不合法变量名**。
- 回读时保留原 key 不动（不强行改名，避免破坏既有绑定）；表达式解析采用**「页面变量集合最长前缀匹配」**：<code v-pre>{{player.list.data}}</code> 中 `player.list` 命中既有 key，可继续被引用。
- 仅当用户主动改名时才写入合法 camelCase 名。新拖入组件一律走 §3.1 生成器。

## 4. 表达式语法（V5 子集）

### 4.1 形式

绑定值只能是以下三种之一（编辑器负责区分）：

```text
{{变量名.路径.到.字段}}     # 单表达式：参数绑定、行操作映射、动作参数
{{row.字段}}               # 行上下文：仅行操作 / 行点击 / 行选中事件内可用
字面量                      # 不以 {{ 开头的原样值（沿用现状 kind=literal）
```

### 4.2 路径文法

```ebnf
expression = variable , { segment } ;
variable   = <页面变量集合最长前缀匹配> | "row" ;
segment    = "." , identifier | "[" , integer , "]" ;
identifier = [a-zA-Z_][a-zA-Z0-9_]* ;
```

示例：

```text
{{playerListTable.selectedRow.uid}}      # 表格选中行的 uid
{{playerListTable.selectedRows}}         # 多选行数组
{{filterForm.values.keyword}}            # 静态表单当前输入值
{{playerListTable.data.total}}           # 表格函数输出对象的 total 字段
{{playerListTable.data.items[0].uid}}    # 数组下标
{{row.playerId}}                         # 行操作内的当前行
```

### 4.3 变量空间（每类组件暴露什么）

| 组件                              | 暴露属性                       | 含义                                              |
| --------------------------------- | ------------------------------ | ------------------------------------------------- |
| fnTable / fnFields                | `data`                         | 函数最近一次输出对象（顶层即 outputSchema 字段）  |
| fnTable                           | `selectedRow` / `selectedRows` | 当前选中行（单选/多选；未选中为 `undefined`）     |
| fnForm / staticForm               | `values`                       | 表单当前值（实时，防抖并入运行时状态）            |
| fnForm                            | `data`                         | 最近一次提交成功的输出                            |
| button / modal / container / text | —（不暴露数据，可作动作目标）  | —                                                 |
| `row`                             | 任意行字段                     | 仅行操作/行事件上下文，编译为现有 `row.字段` 语义 |

`game_id`/`env` 等 scope 字段维持服务端注入，不进表达式命名空间（与现状一致）。

## 5. 编辑器交互

### 5.1 ExpressionInput 组件（新）

```text
┌─────────────────────────────────────────────┐
│ playerId  ←  {{playerListTable.selectedRow.▌│
└─────────────────────────────────────────────┘
        ↓ 键入 {{ 或点击输入框右侧 ⛁ 按钮
┌─────────────────────────────────────────────┐
│ ◇ playerListTable   玩家列表（表格）          │
│ ◇ playerGetFields   玩家详情（字段卡）        │
│ ◇ mailSendForm      发邮件（表单）            │
│ ◇ row               当前行（行操作上下文）    │
└─────────────────────────────────────────────┘
        ↓ 选中变量后继续补全路径（来自 descriptor schema）
   .data  /  .selectedRow.uid / .selectedRows / .values.keyword …
```

行为要点：

- 补全数据源：变量列表来自页面树（变量名 + 标题 + 类型图标）；路径候选来自该变量绑定函数的 `outputSchema`（data/selectedRow 分支）或 `inputSchema`（values 分支）。
- 即时校验：未知变量 → 红标 error（保存阻断级诊断）；路径字段不在 schema → 黄标 warning（schema 可能不完整，不阻断）。
- 原「来源区块 + 字段」双下拉保留为「快捷选择」按钮，点选后**生成表达式**填入输入框（同一数据模型，两种录入方式等价）。
- 纯文本/字面量输入：不以 <code v-pre>{{</code> 开头即字面量，与现状一致。

### 5.2 接入点（属性面板内全部替换）

| 现有交互                                | V5 升级                                                            |
| --------------------------------------- | ------------------------------------------------------------------ |
| ParamMappingEditor（参数映射双下拉）    | 每参数一行 ExpressionInput                                         |
| RowActionsEditor（行操作参数映射）      | 值侧 ExpressionInput（<code v-pre>{{row.字段}}</code> 为主要补全） |
| 动作链步骤参数（`参数=节点.字段` 输入） | ExpressionInput                                                    |
| 成功后刷新 / 动作目标选择               | 下拉选项 label 显示变量名（值为 key，不变）                        |

### 5.3 画布与大纲

- 画布卡片标题行右侧以 `⌗varName` 徽标显示变量名（hover 卡片同步显示）。
- 大纲树节点显示「标题（varName）」。

## 6. 编译规则（表达式 → 现有 wire）

保存时编译器对每个绑定表达式求「静态形态」并落到现有字段：

| 表达式                                      | 编译结果                                                                                                |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| <code v-pre>{{node.data.total}}</code>      | `inputAssignments[]: { target: "/total参数路径", kind: "page_state", key: "var", path: "/data/total" }` |
| <code v-pre>{{node.selectedRow.uid}}</code> | 同上，`path: "/selectedRow/uid"`（运行时从变量状态取，见 §7）                                           |
| <code v-pre>{{node.values.keyword}}</code>  | 同上，`path: "/values/keyword"`                                                                         |
| <code v-pre>{{row.playerId}}</code>         | 事件/行操作 `params: { playerId: "row.playerId" }`（沿用现状语义）                                      |
| 字面量                                      | `kind: "literal", value: 原样值`                                                                        |

路径段转 JSON Pointer（`a[0].b` → `/a/0/b`），与 `internal/dashboard/spec` 的 `ValueSource.Path` 约定一致。

**round-trip 可逆**：回读时 `{kind: page_state, key, path}` → <code v-pre>{{key + path转点路径}}</code>；`literal` → 原样回填。`row.x` → <code v-pre>{{row.x}}</code>。旧页面（无表达式）回读后在下拉里看到与今天完全相同的值，只是显示为表达式文本。

## 7. 运行时求值（PageRenderer / PreviewRuntime 共用）

### 7.1 运行时状态结构

```ts
// 现状：results: Record<key, { data }>
// V5：每区块状态对象扩展（键不变，向后兼容）
type SectionRuntimeState = {
  data?: Record<string, unknown>; // 函数输出（现状）
  selectedRow?: Record<string, unknown>; // fnTable 单选（V5 新增）
  selectedRows?: Record<string, unknown>[]; // fnTable 多选（V5 新增）
  values?: Record<string, unknown>; // fnForm/staticForm 当前值（V5 新增，防抖）
};
```

### 7.2 求值器

- 新增纯函数 `resolveExpression(expr, state, ctx)`（`PageRenderer/runtime.ts`，编辑器/预览/发布渲染三方共用同一实现）：
  1. 最长前缀匹配变量名 → 取 `state[var]`
  2. 逐段取值（对象属性 / 数组下标），中途 `undefined` → `undefined`（不抛错）
  3. `row` 从动作上下文 `ctx` 取
- `resolveStepParams` 重构为求值器的薄封装（`区块key.字段` 旧串 → 内部按表达式解析，行为兼容）。
- 现有「上游输出同名字段自动合并下游输入」逻辑**保持不变**（仍只读 `data` 顶层）。

### 7.3 状态写入时机

- `data`：函数执行成功（现状）。
- `values`：表单 onChange 防抖 300ms 写入（与 staticForm 现状一致，扩到 fnForm）。
- `selectedRow/selectedRows`：表格行选择变化时写入；**不触发** refreshOn 自动重跑（refreshOn 判定仍只看 `data` 更新），避免"点选即连锁执行"。

## 8. 校验与诊断

- **编辑器保存前**（编译期）：解析全部表达式——未知变量、路径语法非法 → error 级诊断，沿用提案 `needs_review` 通道（与 V3.1 统一校验同一入口），不静默丢失。
- **删除组件**：树内存在引用其变量名的表达式 → 保存时 warning 列出"表达式引用已删除的变量 X"（现有「动作目标已删」同款通道）；编辑期大纲诊断面板黄点提示。
- **服务端**：零改动。wire 不新增字段，accept-and-publish 与现有校验链原样工作。

## 9. V5 明确边界（不做）

| 不做                                                                                      | 原因 / 去向                                                                                 |
| ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| JS eval / 函数调用 / 运算符（<code v-pre>{{a + 1}}</code>、<code v-pre>{{fn(x)}}</code>） | 安全与可校验性；后续如需走受控 transform（selector_ast 已有 `TransformSpec` 扩展位）        |
| 文本模板混排（<code v-pre>"玩家 {{row.uid}} 已处理"</code>）                              | showMessage/navigate 文案插值 → V5.1（需运行时插值渲染器，小改）                            |
| selectedRow/values 变化触发 refreshOn 自动重跑                                            | 防连锁风暴；用户通过按钮/动作显式触发                                                       |
| 表达式循环引用静态检测                                                                    | V5 绑定只在动作时求值，无自动传播环；refreshOn 沿用现有规则                                 |
| 组件模板（V4）内部表达式在实例化时的变量重绑定                                            | 模板内变量实例化时按 §3.1 重新去重命名，内部引用随树整体重写；模板对外的数据面参数化 → V5.2 |
| 服务端存储/校验表达式原文                                                                 | wire 只存编译产物（D2）                                                                     |

## 10. 原子任务拆分

| 批次 | 任务                                                                                                               | 验收                                             |
| ---- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------ |
| T5.1 | 变量模型：PageNode 增加变量名字段；命名生成器（§3.1）；改名引用同步重写（纯函数）                                  | model 单测：生成/去重/改名同步全绿               |
| T5.2 | 表达式解析 + 求值器（§4 文法、最长前缀变量解析、JSON Pointer 互转）                                                | runtime 纯函数测试：合法/非法/嵌套/下标/row 全绿 |
| T5.3 | 运行时状态扩展：PageRenderer 写入 selectedRow/values；resolveStepParams 重构为求值器封装                           | PageRenderer 测试：选中行→动作参数取值正确       |
| T5.4 | ExpressionInput 组件（输入 + 两级补全 + 即时校验）                                                                 | 组件测试：补全候选来自 schema、未知变量红标      |
| T5.5 | 属性面板四接入点替换（§5.2）+ 变量名改名输入框 + 画布/大纲徽标                                                     | 编辑器测试：点选生成表达式、改名同步引用         |
| T5.6 | 编译/回读 round-trip（§6）+ 删除组件引用诊断                                                                       | compiler 测试：表达式⇄wire 等价、旧页面回读无损  |
| T5.7 | 端到端验收（编辑→提案→发布页真实联动）+ 三层文档同步（本文件、dashboard-page-model、composite-editor-v3 使用指南） | 线上验证：选中行→表单预填→提交→刷新全链通过      |

关键路径：T5.1 → T5.2 → T5.3 → T5.4 → T5.5 → T5.6 → T5.7（T5.3/T5.4 可并行）。

## 11. 一个完整组装示例（验收场景）

```text
① 拖入 player.list（表格）→ 变量名 playerListTable，autoRun
② 拖入静态表单 → 改名 filterForm；字段 keyword
③ 选中表格 → 参数映射 keyword = {{filterForm.values.keyword}}
④ 选中表格 → 行操作「发邮件」→ 目标弹窗 mailSendModal
   映射 playerId = {{row.uid}}
⑤ 弹窗内 mail.send 表单 → 参数 title = {{playerListTable.selectedRow.nickname}}（V5 能力）
⑥ 发布页验证：输入关键词→表格按 keyword 刷新；选中一行→行尾「发邮件」
   →弹窗 playerId 预填、title 带出昵称；提交→关窗→表格刷新
```
