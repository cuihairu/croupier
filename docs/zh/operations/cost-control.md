---
title: 成本控制
icon: dollar
order: 60
category:
  - 运维手册
tag:
  - 成本
  - 运维
---

# 成本控制（OPS-007）

成本控制提供创建前预估、节点成本打标、空闲回收与月度预算告警，控制台入口在 `/ops/cost`。

## 创建前预估

`POST /api/v1/ops/cost/estimate` 接收 provider、实例类型、数量与预期运行小时数，返回预估 USD 成本与 `knownRate` 标志。

价格目录是**内置参考表**（`internal/ops/costcontrol/costcontrol.go` 的 `HourlyRates`），跨 provider 归一。未知实例类型以 `knownRate: false` 返回且**不**作价（引擎拒绝编造成本）。生产计价须接入真实账单/价格 API；预估准确性是部署后对账项。

## 节点成本与汇总

`GET /api/v1/ops/cost/summary`（按 `X-Game-ID` / `X-Env` 定域）聚合已采纳的 provision run 的成本（run 规格 + 运行时长），返回 per-provider 合计与带空闲标志的逐节点行。

空闲评估（`GET /api/v1/ops/cost/idle`）依赖 agent 资源指标（OPS-010）：近期的全部 CPU 采样连续 `idleHours` 低于配置阈值，节点才判空闲。无近期采样或无指标源一律按**未知**处理（绝不误判为空闲）。

## 预算

`GET` / `PUT /api/v1/ops/cost/budget` 管理按域的月度预算。花费越过阈值（默认 80%）时，经配置的通知渠道（`notification.*`）派发通知，复用审批与告警的同一派发链路。

## 空闲回收策略

策略默认安全：**只提醒**。自动销毁需要 `IdlePolicy` 显式 `enabled: true` 加 `autoDestroy: true`。自动销毁是部署后事项——需要真实云/多节点栈来验证。

## 面板

控制台页面（`web/src/pages/Ops/Cost`）展示预估表单、成本汇总、per-provider 合计、节点表格与预算编辑器。需要 `canOpsManage` 权限。
