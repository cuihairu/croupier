---
title: 文档治理
---

# 文档治理

本文定义 Croupier 文档的权威来源、生命周期和迁移规则。目标是减少重复入口，避免历史设计继续影响当前实现。

## 信息架构

| 目录 | 定位 | 维护规则 |
| --- | --- | --- |
| `docs/guide/` | 使用者指南 | 只写当前可操作流程，不放设计草案 |
| `docs/concepts/` | 业务概念 | 规划目录；现阶段概念文档仍位于 `docs/guide/concepts/` |
| `docs/architecture/` | 架构与协议 | 当前规范优先；提案必须标明状态 |
| `docs/api/` | REST API 与兼容 API | 每个业务域一个 canonical 页面 |
| `docs/analytics/` | 分析系统 | ingest、worker、ClickHouse、指标和 Playbook |
| `docs/sdks/` | SDK 使用者文档 | 跨语言契约和语言接入指南 |
| `docs/development/` | 开发者文档 | 仓库、发布、生成、文档治理 |
| `docs/archive/` | 历史资料 | 规划目录；归档前需要确认批量移动 |
| `sdks/<lang>/README.md` | SDK 源码入口 | 构建、测试、发布、包管理说明 |
| `web/docs/` | Dashboard 开发文档 | 只保留 Web 子系统开发资料 |

## 生命周期

文档必须处于以下状态之一：

| 状态 | 含义 | 要求 |
| --- | --- | --- |
| Current | 当前实现或当前规范 | 可作为实现依据 |
| Compatibility | 为兼容历史 API 或历史部署保留 | 必须说明替代入口 |
| Proposal | 已接受或待实现的目标设计 | 不得写成当前事实 |
| Reference | 调研、模板、背景材料 | 不得作为规范入口 |
| Archived | 发布材料、阶段报告、过期方案 | 默认不进导航 |

## Canonical 规则

- 每个主题只能有一个 canonical 页面。
- 重复页面必须改成 Compatibility、Reference 或 Archived。
- 新文档必须先判断是否能合并到现有 canonical 页面。
- API 文档以 `docs/api/index.md` 中的分类为准。
- SDK 使用者文档以 `docs/sdks/<lang>/` 为准，源码构建和发布说明以 `sdks/<lang>/README.md` 为准。
- 架构文档必须说明是 Current、Proposal、Decision 还是 Reference。

## 当前迁移优先级

1. 修复首页、导航和总入口，清理明显过时的默认表述。
2. 将 Architecture 按当前规范、决策边界、提案、参考资料分组。
3. 合并 API 重复入口，明确 `function_call`、`ops_core`、`ops-simple` 的兼容定位。
4. 收敛 C++ SDK 双份文档，消除 `CONFIG_GUIDE`、`PLUGIN_GUIDE`、`VIRTUAL_OBJECT_REGISTRATION` 的重复来源。
5. 将 `web/` 下 v0.1.6 发布资料和阶段报告归档到 `docs/archive/`。
6. 增加文档 CI：VitePress build、内部链接检查、过时术语检查。

## 过时术语处理

以下术语只允许出现在兼容说明、历史背景或提案迁移章节中：

- `gRPC` 作为 SDK 或 Agent 默认主链路
- `LocalControlService` 作为新语义入口
- `rpc_addr` 作为长期运行时依赖
- SDK 本地 server 回调模型
- 历史 `REQ/REP` 作为主链路模型

如果新文档必须提及这些术语，需要同时写明当前替代方案：统一 TCP session、Agent 本地 gateway、provider session 或 agent session。

## 批量迁移规则

以下操作必须单独确认后执行：

- 删除 Markdown 文件
- 批量移动 Markdown 文件
- 将 `web/` 发布资料迁移到 `docs/archive/`
- 将 SDK 源目录文档改写为跳转页或归档页

批量迁移前必须先列出影响范围、目标路径和回滚方式。
