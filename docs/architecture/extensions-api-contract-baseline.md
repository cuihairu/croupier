---
title: 扩展域 API 契约基线（V1）
icon: file-contract
order: 32
category:
  - 系统架构
tag:
  - extension
  - api
  - contract
---

# 扩展域 API 契约基线（V1）

更新时间：2026-03-15

## 1. 目的

该文档作为前后端联调的稳定基线，约束扩展域 API 的请求/响应与错误结构。

## 2. 通用约定

- 成功响应采用统一包装（`code/message/data`）。
- 列表接口统一分页参数：
  - `page`（默认 1）
  - `page_size`（默认 20）
- 结构化错误通过 `details` 扩展。

## 3. 接口基线

## 3.1 Catalog

1. `GET /api/v1/extensions/catalog`
2. `GET /api/v1/extensions/catalog/:id`
3. `GET /api/v1/extensions/catalog/:id/releases`

关键字段：

- `id`
- `extension_id`
- `display_name`
- `version` / `releases`
- `installed`
- `tags`
- `default_install`

## 3.2 Installation

1. `GET /api/v1/extensions/installations`
2. `GET /api/v1/extensions/installations/:id`
3. `POST /api/v1/extensions/install`
4. `POST /api/v1/extensions/:id/enable`
5. `POST /api/v1/extensions/:id/disable`
6. `POST /api/v1/extensions/:id/upgrade`
7. `DELETE /api/v1/extensions/:id/uninstall`

关键字段：

- `installation_id`
- `installation_key`
- `extension_id`
- `release_version`
- `enabled`
- `status` / `desired_state`
- `scope_type/scope_id`
- `target_type/target_id`

## 3.3 Runtime 与可观测

1. `GET /api/v1/extensions/:id/capabilities`
2. `POST /api/v1/extensions/:id/health-check`
3. `POST /api/v1/extensions/:id/reconcile`
4. `GET /api/v1/extensions/:id/events`

关键字段：

- `capabilities[]`
- `bindings[]`
- `health.status`
- `events[].type`
- `events[].payload`
- `events[].operator`

## 4. 错误码基线

必须稳定支持：

- `extension_already_installed`
- `dependency_blocked`
- `missing_dependency`
- `version_mismatch`
- `dependency_cycle`
- `forbidden`
- `not_found`

## 5. 结构化错误 details 基线

`dependency_blocked`：

- `details.code = dependency_blocked`
- `details.blockers = ["extension@version", ...]`

`extension_already_installed`：

- `details.code = extension_already_installed`
- `details.installation_id`
- `details.scope_type/scope_id`
- `details.target_type/target_id`

`missing_dependency` / `version_mismatch`：

- `details.missing` / `details.expected` / `details.actual`

## 6. 变更规则

允许：

- 新增非必填字段
- 新增错误 details 子字段

禁止（无版本升级前）：

- 删除已发布字段
- 修改字段语义
- 变更已有错误码含义

## 7. 下一步

1. 从本基线生成 OpenAPI 或 TS 类型文件。
2. 在前端建立 adapter 层，只消费基线字段。
