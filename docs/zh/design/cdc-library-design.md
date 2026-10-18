---
title: 通用 CDC 核心库与事件回放设计
---

# 通用 CDC 核心库与事件回放

## 状态与结论

- 状态：L1/L2 核心、Croupier 可选 journal 接线与通用恢复协调契约已实现；MySQL/Mongo 不再误报完整事务边界，恢复门禁拒绝缺失能力；具体源快照/schema history 与目标数据库适配仍待实施（2026-10-19）。
- 结论：方向成熟、技术上可行。先在本仓建立业务无关的 Go package 边界，等契约有第二个真实消费者且版本策略稳定后，再拆成独立 Go module / 仓库。
- 目标不是采集所有表。每个数据源必须显式声明感兴趣的库表和操作；核心库负责通用选择、源端可用时下推，并在事件进入下游前再次过滤。
- “可回放”不等于“任意数据库都能完整 PITR”。只记录筛选后的事件时，只能恢复已采集范围；完整时间点恢复还需要一致性快照、连续位点、事务边界、DDL/schema 历史和目标库应用器。

## 代码现状与缺口

现有多数据库 Connector 已覆盖 MySQL/MariaDB、PostgreSQL 和 MongoDB，统一事件/位点/能力契约位于 `agents/capture/connector`。数据源通过 `capture.options.tables` 做表白名单；`agents/capture/filter` 中另有 Croupier 策略头、库表授权匹配和仅按操作类型过滤的 EventFilter。capture runner 将事件送入 reconcile/CEP/lineage 三闸，命中告警有持久 spool，采集位点也可恢复。

现有边界足以支持 Croupier 的检测用途，但还不是可抽取的通用库：Connector 契约与捕获 Agent 同目录；策略头要求 `gameId/env`，规则类型含 detection/delivery；表过滤、操作过滤分散在源 Connector 和业务 runner。runner 不保留全量事件，所以当前位点恢复只表示继续采集，不表示可重放已经处理过的操作。

## 分层边界

```text
数据库协议适配器（MySQL / PostgreSQL / MongoDB / 后续实现）
            ↓ SourceConnector + capability 声明
通用 CDC 库（事件模型、过滤编译/执行、位点、可选事件日志与回放）
            ↓ ChangeEvent 流
Croupier adapter（game/env scope、配置下发、检测闸、告警与面板）
```

通用 CDC 库可以知道 source ID、数据库、表、操作、行像和源端位点；不能依赖 Croupier 的游戏、玩家、风控规则、告警、权限和面板类型。Croupier 继续把 `gameId/env` 绑定到 source 配置和授权，不把该 scope 写进通用事件模型。

首阶段将通用契约放在 `pkg/cdc`，过滤原语放在 `pkg/cdc/filter`。现有 `agents/capture/connector` 保留兼容别名，先避免一次性改写所有驱动。包只用 Go 标准库；不得依赖 `agents/`、`internal/`、数据库驱动或 Croupier protobuf。以后拆 module 时，迁移应是模块路径和发布流程工作，而不是重新设计 API。

## 事件选择与过滤

过滤规则使用明确的 include/exclude 选择器：source ID/type、database、table、operation。空 include 默认拒绝，exclude 始终优先；`*` 只表示一个字段的通配，不引入正则或可执行表达式。未配置过滤器与“显式允许所有”需要由调用层区分，避免空配置意外采全库。

过滤分两处执行：

1. **Source pushdown**：连接器能力允许时，把精确库表 include 下推，减少读取和传输；不支持通配/下推的 Connector 明确回报能力或保守退回。
2. **通用事件过滤**：Connector 解码后、事件进入业务消费者前再按同一规则过滤。下推是优化，不能成为正确性唯一保障；规则变更也不能让已过滤表的数据绕过 runtime 过滤。

首阶段只支持来源、库、表和 DML 操作过滤。字段投影、字段脱敏、行值谓词、DDL 过滤放到后续版本：这些能力需要统一缺失列/null/类型语义，并且可能破坏还原所需的主键、前像或依赖字段。任意 SQL/脚本谓词不进入首版。

## 日志与回放

需要回放时，通用日志只追加过滤器最终放行的事件，并保存稳定事件 ID、源位点、事务 ID/序号/事务结束标记、前后像和 schema 版本。事件必须先持久化，再确认源位点；回放按日志序号或显式区间顺序消费，并由目标适配器以事务方式应用。重复投递采用 at-least-once + 目标幂等，不宣称跨源、日志和任意目标数据库的分布式 exactly-once。

选择性采集有明确代价：若过滤掉某张表或某种操作，就不能声称可将整个数据库还原到指定时点。完整 PITR 必须使用显式全范围采集策略，包含相关表的快照与 DDL/schema 变更；否则功能名称和 UI 必须写成“筛选事件回放/局部恢复”。

`pkg/cdc/journal` 已提供单写者 JSONL append-only 日志、稳定事件 ID、崩溃残尾截断、顺序范围读取、重复追加幂等以及事务 sink 回放。`Capture` 会先对事件运行通用过滤；命中事件先落盘并 fsync，再确认 source 位点，未命中事件则明确丢弃后确认位点。重复投递可安全重试。日志故意不依赖 Croupier runner、告警 spool 或任何数据库驱动。

Croupier adapter 已提供可选 `capture.journal` 配置：filter include 从现有 `capture.options.tables` / legacy `capture.mysql.tables` 编译，source ID/type 和 operations 加入通用选择器，exclude 可进一步缩小范围；不会读取 game/env 做事件筛选。runner 与原检测流共用一个 Connector stream：先持久化选中事件，再运行本地检测，最后确认 source 位点。日志有必填硬容量上限；达到上限时不确认当前事件，消费失败并由现有重连退避暂停，扩容/归档后可续采。它不会自动轮转或删除历史日志。

回放严格要求事务边界元数据：有事务 ID 的事件须从序号 1 连续到 `End=true`；无事务 ID 的独立提交也须显式 `End=true`。PG 解码器已标记事务末事件；MySQL 尚未产出事务结束标记，MongoDB change stream 当前没有可靠的事务结束事件。因此这两个来源可以按筛选策略记录事件，但当前回放会拒绝其缺少结束边界的事务，不得把单行事件误当作完整事务恢复。真实数据库快照一致性、MySQL/Mongo 事务边界、目标库写入和恢复 UI 仍须按 Connector 能力分批实现。缺失 before image / snapshot 的 Connector 必须拒绝不满足能力要求的恢复计划，不能用空值伪造。

恢复协调契约位于 `pkg/cdc/recovery`，不含游戏/env 或某个数据库的 SQL。`TargetConfig` 只接收 adapter ID、非密选项和 DSN 环境变量名，并拒绝常见明文凭据选项；`Plan` 要求明确 principal、source、target、日志区间、源位点、精确表清单和一致快照覆盖；`Authorizer` 由产品 adapter 映射租户/游戏/环境权限，`SnapshotVerifier` 负责验证快照位置早于恢复起点及 schema 版本，`Applier` 声明能力、提供只读校验以及具备原子回滚和幂等键的事务应用，`Auditor` 必须持久记录请求、授权、预检、每个事务和最终结果。dry-run 只执行校验；apply 只有在预检和审计成功后才调 applier。审计仅存元数据、哈希事件 ID 和错误类型，不写 row image、SQL 错误原文或凭据。目标调用返回错误时，applier 必须保证事务回滚；结果未知时按 request ID + transaction ID 重试。能力声明增加 `schemaHistory`，源适配器必须只在确实能提供连续 DDL/schema 历史时才置为 true。

## 阶段与验收

| 阶段 | 范围 | 验收 |
| --- | --- | --- |
| L1 | 独立 CDC 契约与通用 include/exclude 过滤 | 无 Croupier import；空 include 默认拒绝；exclude 优先；旧 connector import 兼容；源过滤和 runtime 过滤一致 |
| L2 | 可选 append-only journal 与顺序回放 API（已实现） | 崩溃残尾处理、校验损坏、幂等 append、范围回放、事务不完整拒绝、sink 失败不推进读取游标；journal 持久化后再 ack source 位点 |
| L3 | Croupier 接线与运维配置（已实现可选本地接线） | game/env 留在 adapter；表白名单/操作/exclude 编译为通用 filter；同一个 source stream 在 ack 前写日志；容量满时 fail closed |
| L4 | 通用恢复协调契约（已实现） | 精确 scope、授权、快照覆盖校验、source/target 能力门禁、dry-run、事务原子回滚与幂等契约、持久审计 |
| L5 | 具体数据库快照与目标库 adapter（未实现） | 每种 source/target 验证 snapshot/before/transaction/schema 能力；真实隔离库 dry-run、事务应用、失败回滚和审计集成；无能力的来源保持拒绝 |

L1/L2 是可独立复用的库能力；L3 是产品接入；L4 是数据库无关的恢复协调边界；只有 L5 完成后，具体数据库才可被称为“可恢复”。
