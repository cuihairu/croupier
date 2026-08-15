---
title: Dashboard 术语表
icon: book-open
order: 9
category:
  - 系统架构
tag:
  - Dashboard
  - 术语
---

# Dashboard 术语表

> **状态**：Current -- 本文只定义当前模型。任何未出现在本文的历史页面模型都不作为当前术语使用。

| 术语                       | 定义                                                                                                                                                                                 | 不是什么                              |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------- |
| FunctionContract           | 一个可执行函数的版本、输入/输出 JSON Schema、治理和有限能力语义                                                                                                                      | 页面或菜单配置                        |
| ResourceCapability         | 围绕同一业务资源聚合的函数能力集合                                                                                                                                                   | 数据库表或直接业务数据 API            |
| CapabilitySemantics        | 资源 identity、collection、CRUD、action/task/report 的可验证业务语义                                                                                                                 | 列、按钮位置、页面布局或 mapping JSON |
| Resource Catalog           | 管理和审核 CapabilitySemantics 的平台入口                                                                                                                                            | Page Studio 或业务 CRUD API           |
| PageProposal               | Server 根据能力和语义生成的可追溯页面建议                                                                                                                                            | 用户草稿或运行页面                    |
| ProposalKey                | 生成器的幂等身份：`resource:<resourceKey>` 或 `<kind>:<functionId>`                                                                                                                  | 菜单标题或随机页面 ID                 |
| PageIdentity               | `game_id + env + pageKey` 的发布页面身份                                                                                                                                             | 函数 ID 或 URL 中可覆盖的 scope       |
| PageDraft                  | 用户接受 Proposal 后可编辑的页面版本                                                                                                                                                 | 自动覆盖的生成缓存                    |
| PublishedPageSpec          | 校验通过且冻结契约摘要的不可变运行页面快照                                                                                                                                           | 最新函数的实时投影                    |
| BindingFreshnessDiagnostic | 发布快照与最新 FunctionSpec 的只读漂移报告（input/output/governance/execution/function missing）；同步仍需显式合并与重新发布                                                         | 自动失效或静默切换信号                |
| BindingFreshnessStatus     | `fresh` / `contract_missing` / `function_missing` / `input_schema_stale` / `output_schema_stale` / `selector_stale` / `governance_stale` / `approval_stale` / `execution_mode_stale` | 页面 quality 枚举                     |
| ThreeWayMerge              | base proposal 快照、当前 draft、最新 proposal 的三方合并：展示类字段自动合并，执行类字段进入人工冲突集                                                                               | 全量接受最新合同的自动同步            |
| BlockedProposalIssue       | 不可安全物化页面时的诊断和修复指引                                                                                                                                                   | 带 `blocked` quality 的 Proposal      |
| SemanticProvenance         | 每个有效或冲突语义字段的来源、摘要和置信度                                                                                                                                           | 单一 source 字段                      |
| ApprovalPolicy             | 与同步/异步执行正交的审批要求和策略引用                                                                                                                                              | 第五种页面类型或 execution 枚举值     |
| ResourcePage               | 围绕一个资源的列表、详情、CRUD 和资源动作页面                                                                                                                                        | 所有函数的容器                        |
| OperationPage              | 独立同步命令的表单、确认和结果页面                                                                                                                                                   | 低配 CRUD 页面                        |
| TaskPage                   | 启动和跟踪异步任务的页面                                                                                                                                                             | 只展示 taskId 的结果框                |
| ReportPage                 | 查询、数据集、指标、维度和图表/表格的页面                                                                                                                                            | 任意 JSON 输出                        |
| PageBinding                | 页面中受控执行一个函数的身份、输入/输出 selector 和治理约束                                                                                                                          | 浏览器可任意调用的 functionId         |
| Typed Selector             | 受限 AST，引用 form/row/selection/detail/page state/literal                                                                                                                          | 任意 JSONPath 或整行盲传              |
| FormPresentationSpec       | JSON Schema 表单的字段顺序、分组、widget hint 和可见性配置                                                                                                                           | FunctionContract 或页面布局树         |
| ConsoleMenuSpec            | 由 active PublishedPageSpec 生成的 Console 动态菜单                                                                                                                                  | 函数目录、字典或静态 locale           |
| Scope                      | 全局 `game_id + env` 上下文                                                                                                                                                          | 页面 URL 或 payload 中可覆盖的参数    |

基础 DTO（`Scope`、`FunctionRef`、`SourceDigest`、`Diagnostic`、`LocalizedText`、`JsonPointer` 和 JSON 值类型）唯一以 [Dashboard Resource/Page 模型](./dashboard-page-model.md) 的定义为准；前端共享类型与 Go DTO 必须逐项对应，不得在页面或组件内重复定义。

## 常见边界

```text
FunctionContract answers: "这个能力怎样调用？"
CapabilitySemantics answers: "它在资源生命周期中做什么？"
PageSpec answers: "运营人员如何完成业务任务？"
ProComponents renderer answers: "如何在 React 中显示这个页面？"
```

若一个字段同时试图回答两层以上的问题，应拆分模型而不是增加通用 metadata。
