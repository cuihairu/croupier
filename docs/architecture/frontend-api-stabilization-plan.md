---
title: 前后端 API 稳定化与前端重整计划
icon: object-group
order: 31
category:
  - 系统架构
tag:
  - frontend
  - api
  - extension
---

# 前后端 API 稳定化与前端重整计划

更新时间：2026-03-15

## 1. 目标

1. 先冻结后端 API 契约，再推进前端页面重整。
2. 把前端改造成“域适配层驱动”，而不是“页面直连接口字段”。
3. 所有扩展相关页面统一收敛为 extension-first 语义。

## 2. 范围

优先覆盖以下域：

- extension catalog/installations/runtime/events
- platform（external-platform）入口与调用体验
- ops 中已扩展化模块（alerts/approval/notification/backup）

不在本阶段：

- 全站视觉重设计
- 非扩展域页面大规模改版

## 3. 契约冻结清单

以下接口作为第一批冻结对象：

1. `GET /api/v1/extensions/catalog`
2. `GET /api/v1/extensions/catalog/:id`
3. `GET /api/v1/extensions/catalog/:id/releases`
4. `GET /api/v1/extensions/installations`
5. `GET /api/v1/extensions/installations/:id`
6. `POST /api/v1/extensions/install`
7. `POST /api/v1/extensions/:id/enable`
8. `POST /api/v1/extensions/:id/disable`
9. `POST /api/v1/extensions/:id/upgrade`
10. `DELETE /api/v1/extensions/:id/uninstall`
11. `GET /api/v1/extensions/:id/events`
12. `POST /api/v1/extensions/:id/health-check`
13. `POST /api/v1/extensions/:id/reconcile`

冻结内容：

- 请求/响应字段
- 分页参数与默认值
- 错误码与 `details` 结构
- 权限键要求

## 4. 前端分层模板

建议统一三层：

1. `api client`：纯 HTTP 与 DTO 类型。
2. `domain adapter`：把 DTO 映射为前端稳定 ViewModel。
3. `ui pages`：只消费 ViewModel，不直接读后端细节字段。

约束：

- 页面禁止直接访问 `response.data.raw` 异构字段。
- 结构化错误统一经过 `errorMapper`。

## 5. 错误语义标准化

最低支持以下错误码映射：

- `extension_already_installed`
- `dependency_blocked`
- `missing_dependency`
- `version_mismatch`
- `dependency_cycle`
- `forbidden`
- `not_found`

前端统一处理：

- toast 简短错误
- 详情弹窗展示 `details.blockers` / `details.missing` / `details.expected`

## 6. 页面重整顺序

Phase A（先稳契约）

1. 输出 OpenAPI/TS 类型基线（生成文件入库）。
2. 建立 `services/api` + `services/adapters` 双层结构。

Phase B（核心页面切流）

1. 商店页与安装列表页改为 adapter 驱动。
2. 安装详情页（config/events/health）统一字段来源。

Phase C（业务页切流）

1. platform 页面去 legacy 文案与状态逻辑。
2. alerts/approval/notification/backup 入口切到扩展页面容器。

Phase D（门禁与回归）

1. 增加联调 smoke：安装、启停、升级、健康检查、事件分页。
2. 增加 UI 回归截图对比。

## 7. 交付物模板

- `docs/contracts/extensions-openapi-v1.yaml`（当前已落地）
- `dashboard/src/services/api/extensions.ts`（API 层）
- `dashboard/src/services/adapters/extensions.ts`（适配层）
- `dashboard/src/constants/error-codes.ts`
- `dashboard/src/tests/smoke/extensions/*.spec.ts`

## 8. 验收标准

1. 后端新增非破坏字段不导致前端改动。
2. 后端兼容更新时，前端仅改 adapter，不改页面业务逻辑。
3. 扩展域关键链路（安装/升级/启停/健康检查）可稳定回归。
