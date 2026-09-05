---
title: 组合模型与表达力边界
icon: blocks
order: 9
category:
  - 系统架构
tag:
  - 组合页
  - PageSpec
  - 设计总纲
---

# 组合模型与表达力边界

> **状态**：Current —— 本文是组合页/页面生成的**设计总纲**：定义心智模型、组合原语、
> 表达力边界与扩展方法论。规则细节以 pagespec-protocol.md（wire）、
> dashboard-page-model.md（模型）、presentation-hints.md（hints）为准，本文不重复。
> 新手入门（为什么有这套设计、全流程怎么走）见
> [界面是怎么生成的：核心思路与全链路](./descriptor-driven-ui.md)。

## 定位：为什么这是产品的关键设计

组合页的搭建方式是「图形化的声明式 React」：用户拖放积木、连线数据、配置动作，
**不写代码就得到一个治理完备的管理页面**。它与市面低代码产品的本质区别不在积木数量，
而在组合原语的设计取舍：

- **治理优先的白名单原语**：每个组合能力都是 spec 字段 + 渲染器能力 + 校验器——
  不是自由画布加任意代码。权限、审批、审计挂在函数绑定上，页面无论怎么组合都
  不会绕过治理（浏览器只提交 `page_state`，functionId/target/scope 由服务端解析）。
- **声明式数据级组合**：页面是 JSON 树而非代码——可校验、可快照、可 diff、可回滚、
  可跨语言渲染（rjsf 消费 schema，PageRenderer 消费 spec）。
- **三类生成来源**：契约自动生成（A 类）、配置导入（B 类，如常量 Excel/JSON）、
  用户创作（画布多选保存为组件）。组合页面 = 这三类的拖放组合。

## 心智模型：图形化的声明式 React

React 的本质是 `UI = f(data)`：声明界面长什么样，框架负责更新。本体系把 `f`
从代码变成数据——受约束的声明式 React 子集。

### 两层分工

| 层         | 职责                         | 载体        | 消费方                               |
| ---------- | ---------------------------- | ----------- | ------------------------------------ |
| **控件层** | 字段/类型/enum/校验/控件类型 | JSON Schema | `@rjsf/antd`（SchemaFormRenderer）   |
| **编排层** | 布局/数据流/动作/生命周期    | PageSpec    | PageRenderer（CompositeRenderer 等） |

JSON Schema 只管「控件与数据形状」（rjsf 已是 "Schema → React 表单" 的成熟
图形化等价物）；布局、联动、动作、生命周期归 PageSpec。两层合起来才是完整的
图形化 React。

### React 组合原语 ↔ PageSpec 映射

| React 原语                  | PageSpec 对应物                                                                                        | 现状                                                |
| --------------------------- | ------------------------------------------------------------------------------------------------------ | --------------------------------------------------- |
| 嵌套 `children`             | container/modal 的 children 树                                                                         | ✅                                                  |
| **props 传入**（父→子数据） | SelectorAST 输入赋值（form/row/detail/page_state/literal + JsonPointer）+ `refreshOn` 同名字段隐式合并 | ⚠️ 半自动：同名合并已通，**显式参数映射 UI 未暴露** |
| **回调传出**（子→父行为）   | events → 动作链（runBinding/refreshNode/openModal/closeModal/navigate/showMessage）                    | ✅                                                  |
| context 共享状态            | page_state（output 赋值 stateKey）                                                                     | ✅                                                  |
| 派生值（computed props）    | transform 白名单：仅 `pick`                                                                            | ❌ 缺 rename/default/format                         |
| 条件渲染                    | 表单内 `visibleWhen`（只读 form/page_state）                                                           | ⚠️ 缺区块级                                         |
| 列表 `map`（每行执行）      | fnTable 内置渲染；跨区块批量无                                                                         | ❌ 缺（selection 语义未闭环）                       |
| 生命周期                    | autoRun / refreshOn 级联 / events                                                                      | ✅（缺失败策略）                                    |
| 组件参数化（props 默认值）  | 模板 scaffold + 属性面板                                                                               | ⚠️ 缺模板级批量配置                                 |

### 组合四轴与缺口

组合 = **空间 + 数据 + 行为 + 条件** 四个轴：

1. **空间**（放哪、多大）：栅格 span / 容器 / 弹窗分组——✅ 基本完整；
   已知边界：容器嵌套两层内完整交互，孙层预览简化。
2. **数据**（值从哪来、到哪去）：refreshOn 级联 + 同名合并 + SelectorAST——
   **P0 缺口**：显式参数映射 UI（下游参数 ← 来源区块.字段 下拉选择）；
   区块实例命名空间（变量名前缀，通用化——常量表单已先行）。
3. **行为**（触发什么）：events + 动作链——✅ 可用；
   P1 缺口：条件动作、失败策略（上游失败时下游清空/保留/提示，未定义）。
4. **条件**（何时显示/执行）：表单内 visibleWhen ✅；区块级条件显示 ❌。

## 表达力边界：刻意取舍

React 的表达力是**任意代码**；PageSpec 的表达力是**白名单原语**。每一条限制
都是治理的代价换来的：

| 不允许                                      | 为什么                                               |
| ------------------------------------------- | ---------------------------------------------------- |
| 页面 spec 出现代码/任意表达式/任意 JSONPath | 保存可校验、发布可快照、运行不逃逸                   |
| 浏览器传 functionId/target/scope            | 治理（权限/审批/审计）只在服务端绑定执行上           |
| 前端私加 widget/变换/动作                   | 原语受控：扩展必须走 spec 包 + 校验器 + 文档全链路   |
| 无绑定函数的执行区块                        | 常量表单（static）只产生页面状态；执行必须挂治理函数 |

## 扩展方法论：加一个原语的五步链路

任何新组合能力（新 widget/新变换/新区块类型/新动作）必须走完整链路，
缺一步即视为未完成：

1. **spec 字段**（internal/dashboard/spec + JSON Schema 校验器）
2. **前端类型**（web/src/types/dashboard.ts）
3. **编辑器**（组件注册/属性面板/编译与回读 round-trip）
4. **渲染器**（PageRenderer/SchemaFormRenderer 消费）
5. **文档 + 测试**（三层文档同步 + 单测/快照）

## 缺口与路线

| 优先级 | 缺口                                                            | 说明                                                 |
| ------ | --------------------------------------------------------------- | ---------------------------------------------------- |
| P0     | 显式参数映射 UI（暴露 SelectorAST）                             | 从「同名巧合」升级为「显式契约」；后端已就绪，纯前端 |
| P0     | 区块实例命名空间（变量名前缀/重命名）                           | 通用化常量表单的先行实践；解决同模板多实例同名覆盖   |
| P1     | 失败策略 + transform 扩展（rename/default/format）              | 数据流健壮性                                         |
| P2     | 区块级条件显示、批量（map）组合                                 | 批量依赖 selection 语义闭环                          |
| P3     | 新积木：任务监控组合（taskStatus 节点）、报表图表（chart 节点） | 每项 = 新节点类型 + 渲染器，属组件模型扩展           |

## 与其他文档的关系

- wire 契约细节：[pagespec-protocol.md](./pagespec-protocol.md)
- 领域模型（契约/语义/提案/发布）：[dashboard-page-model.md](./dashboard-page-model.md)
- 呈现 hints（x-ui-\*）：[presentation-hints.md](./presentation-hints.md)
- 生成与运行时：[ui-generation.md](./ui-generation.md)
- 编辑器使用：[组合页编辑器 V3](../dashboard/composite-editor-v3.md)
