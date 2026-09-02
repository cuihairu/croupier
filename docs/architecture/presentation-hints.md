---
title: 呈现 Hints 契约（x-ui-*）
icon: highlight
order: 7
category:
  - 系统架构
tag:
  - JSON Schema
  - FormPresentationSpec
  - 契约
---

# 呈现 Hints 契约（x-ui-*）

> **状态**：Current — 本文档是 JSON Schema 呈现 hints（`x-ui-*` 扩展字段）的唯一规范出处。权威消费实现是 `web/src/utils/schemaHints.ts`（推导器）；本文与实现不一致时以代码为准并修正本文。

## 背景与问题

`FunctionContract.input_schema`（JSON Schema）是表单校验的事实源，但不含任何呈现语义。
后果：

- Invoke 工作台只能把裸 schema 直接交给 `SchemaFormRenderer`，字段 label 退化为英文 key；
- widget/分组/联动等能力只在 PageSpec 体系（Page Studio 人工配置）中存在，与 descriptor 注册链路脱节；
- 游戏方最了解自己参数的呈现意图（"这个字段是玩家选择器"、"这两个参数是一组"），却没有任何表达通道。

本契约定义 **JSON Schema `x-ui-*` 扩展字段**作为呈现意图的载体：游戏方在 schema 内声明，
前端推导为 `FormPresentationSpec`。wire 层零改动（`input_schema` 本就是字符串透传）。

## 分层与原则

```text
input_schema（事实源）
  └─ 字段 schema 上的 x-ui-* hints        ← SDK/游戏方声明（本文档）
       → derivePresentationSpec() 推导     ← 前端纯函数
         → FormPresentationSpec            ← 表单呈现层（pagespec-protocol.md）
           → SchemaFormRenderer 渲染
```

1. **页面编排不在此列**：hints 只表达*字段级*表单呈现意图，不表达页面类型、列表、
   导航、路由。页面编排仍归 PageSpec / Page Studio（见 pagespec-protocol.md）。
2. **schema 永远是校验事实源**：hints 不得改变 schema 的类型、required、enum 取值域。
   `x-enum-options` 只补充选项*标签*，取值域仍以 schema `enum`/`enumNames` 为准。
3. **hints 是意图，不是协议持久化**：hints 随 `input_schema` 整体持久化于 contract
   （`SourceDigest` 天然覆盖），前端派生的 uiSchema 仍只在内存中（遵守 ui-generation.md
   「表单渲染」约束）。

## 字段清单与映射

hints 放置在 **字段 schema 对象**上（支持嵌套，推导为点路径 key）；布局/分组声明放在
**根 schema** 上。

### 字段级 hints

| hint               | 类型                        | 映射到 `FormFieldSpec`    | 说明                                                                                                                             |
| ------------------ | --------------------------- | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `x-widget`         | `FormWidget` 枚举           | `widget`                  | 受控枚举，取值同 pagespec-protocol.md「FormPresentationSpec」（`Input`/`Select`/`TreeSelect`/…）；未知值忽略并按 schema 默认推导 |
| `x-label`          | `LocalizedText` 或 `string` | `label`                   | 覆盖 schema `title`；string 归一为 `{ "zh-CN": s }`                                                                              |
| `x-placeholder`    | `LocalizedText` 或 `string` | `placeholder`             |                                                                                                                                  |
| `x-description`    | `LocalizedText` 或 `string` | `description`             | 帮助文本；schema `description` 缺失时的兜底来源之一                                                                              |
| `x-width`          | `1..12` int                 | `width`                   | grid 布局栅格宽度                                                                                                                |
| `x-order`          | number                      | `order`                   | 排序，小者在前；未声明时按 schema properties 顺序                                                                                |
| `x-disabled`       | bool                        | `disabled`                |                                                                                                                                  |
| `x-visible-when`   | `ConditionSpec`             | `visibleWhen`             | 受限表达式，结构同 pagespec-protocol.md（`equals`/`notEquals`/`exists`/`all`/`any`，path 为 JSON Pointer）                       |
| `x-enum-options`   | `[{ value, label }]`        | `enumOptions`             | 只补充标签展示；`label` 为 `LocalizedText` 或 `string`                                                                           |
| `x-widget-props`   | object                      | `widgetProps`             | 透传给 widget 的额外 props（如 `treeData`、`accept`、`maxCount`）                                                                |
| `x-group`          | string                      | 归入 `groups[].fields`    | 分组 key；组标题在根级 `x-ui-groups` 声明                                                                                        |
| `x-options-source` | 见下文                      | `remoteOptions`（规划中） | 远程选项源，**当前为保留字段**，状态见下文                                                                                       |

### 根级 hints

| hint          | 类型                                          | 映射到 `FormPresentationSpec` | 说明                                                                                                         |
| ------------- | --------------------------------------------- | ----------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `x-ui-layout` | `vertical`/`horizontal`/`inline`/`grid`       | `layout`                      | 缺省 `vertical`                                                                                              |
| `x-ui-groups` | `[{ key, title?, collapsible?, collapsed? }]` | `groups`                      | 组成员由字段 `x-group` 归属；未在 `x-ui-groups` 声明的分组 key 按字段出现顺序自动补组（title 取 key 人性化） |

### 远程选项源（`x-options-source`，保留）

```json
"x-options-source": {
  "functionId": "player.list",
  "labelPath": "/items/*/nickname",
  "valuePath": "/items/*/playerId",
  "searchParam": "keyword"
}
```

指向同 `(game_id, env)` 作用域内已注册的 `collection_query` 函数，运行时调用获取选项。
**状态：契约已预留、前端消费未实现**（todo.md F9）；消费实现落地前，该字段被推导器
忽略。调用走既有 RBAC（无权限时降级为普通输入）。

## 推导规则

`derivePresentationSpec(schema: JSONSchema): FormPresentationSpec`
（`web/src/utils/schemaHints.ts`，纯函数、无副作用）：

1. **key 路径**：推导器只处理顶层 `properties`，每项生成一个 `FormFieldSpec`；嵌套
   object 保留为整体字段（其上的 `x-widget`/`x-label` 等 hints 照常生效），不展开为
   点路径——点路径展开需要渲染器支持 dotted uiSchema，待该能力落地后启用
   （todo.md F 系列）。
   数组/`additionalProperties` 不展开。
2. **优先级**（高→低）：Page Studio 平台覆盖 > `x-ui-*` hints > schema 派生
   （`title`/`format`/`enum`）> 兜底（key 人性化，见 todo.md F5）。
3. **顺序**：`x-order` 升序，其余按 schema properties 定义顺序稳定排序。
4. **容错**：hints 值类型非法（如 `x-widget: "NoSuchWidget"`、`x-width: "6"`）时忽略该
   hint，不影响其余推导，也不产生运行时错误。未知 `x-ui-*` 键一律忽略（向前兼容）。
5. **无 hints 时**：输出与现状等价的 `{ jsonSchema, layout: 'vertical' }`，保证回退行为
   与历史一致。

## 兼容性

- **JSON Schema 校验器**：未知关键字被 AJV 等校验器忽略，hints 不影响 payload 校验。
- **OpenAPI 3.0**：`x-` 前缀是官方 vendor extension 语法，OpenAPI 注册链路（prompt→
  contract 透传）同样携带。
- **proto/wire**：`FunctionDescriptor.input_schema` 是字符串透传，零改动。
- **服务端**：`function_contracts.input_schema` 原样存储；`SourceDigest` 覆盖含 hints 的
  整个 schema——hints 变更即契约变更，天然进入既有 stale/diff 流程（PageSpec 失效检测）。
- **消费端版本差**：旧版前端忽略未知 `x-ui-*`，表现为现状裸 schema 渲染，不报错。

## 治理边界

1. hints **不产生导航/页面级 labels**：页面 title、菜单分类仍只能来自 Page Studio 人工
   编辑（ui-generation.md「生成器职责」的 labels 治理不变）。
2. `x-widget` 只能取受控枚举值；扩展 widget 须修改 `internal/dashboard/spec` 包与前端
   类型后同步本表，不允许前端私加（pagespec-protocol.md 同款约束）。
3. hints 属于函数契约的一部分，随契约变更走既有 review/diff 流程；平台可通过契约
   Diagnostics 对非法 hints 记录告警（todo.md F12 范围）。
4. 本地化遵循 LocalizedText 契约（`localized-text-contract.md`）：`x-label` 等的
   `LocalizedText` 形态 key 必须是 `"zh-CN"`/`"en-US"`。

## 示例

### 1. 基础 widget 与标签

```json
{
  "type": "object",
  "required": ["playerId", "reason"],
  "properties": {
    "playerId": {
      "type": "string",
      "x-widget": "Select",
      "x-label": { "zh-CN": "玩家", "en-US": "Player" },
      "x-placeholder": { "zh-CN": "选择玩家", "en-US": "Select player" }
    },
    "reason": {
      "type": "string",
      "maxLength": 200,
      "x-label": "封禁原因",
      "x-widget": "TextArea"
    },
    "duration": {
      "type": "integer",
      "minimum": 1,
      "x-widget": "InputNumber",
      "x-width": 6
    }
  }
}
```

### 2. 分组与布局

```json
{
  "type": "object",
  "x-ui-layout": "grid",
  "x-ui-groups": [
    { "key": "basic", "title": { "zh-CN": "基本信息", "en-US": "Basic" } },
    { "key": "advanced", "title": { "zh-CN": "高级选项" }, "collapsible": true }
  ],
  "properties": {
    "title": { "type": "string", "x-group": "basic" },
    "level": { "type": "integer", "x-group": "basic", "x-width": 6 },
    "whisper": {
      "type": "boolean",
      "x-widget": "Switch",
      "x-group": "advanced"
    },
    "template": { "type": "string", "x-widget": "Code", "x-group": "advanced" }
  }
}
```

### 3. 联动与远程选项（保留字段示例）

```json
{
  "type": "object",
  "properties": {
    "mode": {
      "type": "string",
      "enum": ["single", "batch"],
      "x-widget": "Radio",
      "x-label": { "zh-CN": "发放模式" }
    },
    "targetPlayer": {
      "type": "string",
      "x-widget": "TreeSelect",
      "x-options-source": {
        "functionId": "player.search",
        "labelPath": "/items/*/name",
        "valuePath": "/items/*/id"
      },
      "x-visible-when": { "kind": "equals", "path": "/mode", "value": "single" }
    },
    "batchFile": {
      "type": "string",
      "x-widget": "FileUpload",
      "x-visible-when": { "kind": "equals", "path": "/mode", "value": "batch" }
    }
  }
}
```
