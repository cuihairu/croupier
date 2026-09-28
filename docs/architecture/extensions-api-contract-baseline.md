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

> **状态**：Decision — 扩展域 API 契约基线，约束前后端请求/响应与错误结构。
> 本文 2026-09-28 版按平台 API 响应契约与实现现状收口（#46）：旧版「统一包装
> `code/message/data`」的表述作废——实现（`internal/common/response`）一直是
> 成功直返业务 JSON，旧文档属于漂移而非超前。

更新时间：2026-09-28

## 1. 目的

该文档作为前后端与 Agent 联调的稳定基线，约束扩展域 API 的请求/响应与错误结构。

## 2. 通用约定

- **成功响应直返业务 JSON**（`internal/common/response.Success` 即 `c.JSON(200, data)`），
  不带 `code`/`message`/`data` 包装。业务响应结构体（如列表返回 `{total, items}`）
  只含业务字段。
- **错误响应**统一 `{ "error": "<snake_case 稳定码>", "message": "<人类可读>",
"details"?: {...} }`，HTTP 状态码表达错误类别（400/403/404/409/500，见平台
  API 响应契约）。
- **契约字段一律 lowerCamelCase**（`installationId`、`scopeType`、`releaseVersion`）。
  旧版文档中的 snake_case 字段名（`installation_id` 等）是文档笔误，wire 上从来
  不是这个形态。
- 列表接口统一分页参数：`page`（默认 1）、`pageSize`（默认 20）；列表响应统一
  `{ total, items }`。
- 扩展域路由挂 `FlagExtensions` soft-flag 组（未启用时整域 404），**不在 scoped
  组**（无 `GameDBMiddleware`）：扩展表是 meta 库模型，`scopeType/scopeId` 是
  行级业务标记而非分库路由。

## 3. 接口基线

canonical 路由形态如下（与 `internal/handler/routes.go registerExtensionRoutes`
一致）。**兼容路由组 `/api/v1/extensions/:id/*` 已废弃**：它是历史 URL 形态的
别名，唯一前端消费方（`listExtensionPages`）本就契约错位（见 §6），修复后该组
零消费方，按「禁止兼容旧键」纪律随实施批次删除。

### 3.1 Catalog

1. `GET /api/v1/extensions/catalog`
2. `GET /api/v1/extensions/catalog/:id`
3. `GET /api/v1/extensions/catalog/:id/releases`

关键字段（`ExtensionCatalogItem`）：

- `id`（= extensionId，如 `official.notification`）
- `name` / `displayName` / `vendor` / `kind`（`official|community|private`）
- `summary` / `iconUrl` / `status`（`active|hidden|deprecated`）
- `latestVersion` / `installed`（该 scope 是否有活跃安装）/ `defaultInstall` / `tags[]`

detail 额外返回：`releases[]`、`manifest`（快照）、`capabilities[]`。
releases 关键字段（`ExtensionReleaseItem`）：`version` / `releaseChannel`
（`stable|beta|experimental`）/ `minCoreVersion` / `publishedAt` / `changelog`。

> **已知缺口（V2 批次）**：catalog/release 目前**只读**——repo 层无写方法、
> 无 admin CRUD、无 pack 导入登记。目录行如何产生是安装模型收口的最大缺口，
> 批次链见 §7。

### 3.2 Installation

1. `GET /api/v1/extensions/installations`
2. `GET /api/v1/extensions/installations/:id`
3. `POST /api/v1/extensions/install`
4. `GET|PUT /api/v1/extensions/installations/:id/config`
5. `GET /api/v1/extensions/installations/:id/config-schema`
6. `POST /api/v1/extensions/installations/:id/test-connection`
7. `POST /api/v1/extensions/installations/:id/enable`
8. `POST /api/v1/extensions/installations/:id/disable`
9. `POST /api/v1/extensions/installations/:id/upgrade`
10. `POST /api/v1/extensions/installations/:id/reconcile`
11. `DELETE /api/v1/extensions/installations/:id`（卸载）
12. `GET /api/v1/extensions/installations/:id/events`

安装项关键字段（`ExtensionInstallationItem`）：

- `id` / `installationKey` / `extensionId` / `displayName`
- `releaseVersion` / `status` / `desiredState` / `enabled`
- `scopeType`（`global|game|env|node-group|node`）/ `scopeId`
- `targetType`（`server|agent|hybrid`）/ `targetId`
- `healthStatus`（**V1 恒 `unknown`**：无健康探测落库，`extension_health` 表按
  简化决策不建，见安装模型文档；health-check 端点的**响应**才是推导值——
  enabled→healthy、否则 disabled、卸载→uninstalled）
- `displayName`（V1 实为 `extensionId` 原值：列表组装未 join catalog，列为批次
  链待修项）
- `lastError` / `updatedAt`

detail 额外返回：`installation`、`configSchema`、`config`、`secretRefs`、
`bindings[]`（`bindingType/bindingKey/targetRef/status/lastError`）、`events[]`。

动作响应统一 `[{status: "<enabled|disabled|upgraded|uninstalled>"}]` 直返；
reconcile 额外 `applied` / `failed` 计数；health-check 返回 `{status, checkedAt}`。

升级语义：`upgrade {releaseVersion}` 要求目标版本存在于 release 目录、依赖图
校验通过、现有 config 对新版本 schema 校验通过；**回滚 = upgrade 到旧版本**
（同版本重复升级返回 409 conflict），不设独立 rollback 端点。

卸载保护：存在活跃依赖方（`ensureNoActiveDependents`）时拒绝，错误码
`dependency_blocked` + `details.blockers`。

### 3.3 Runtime 与可观测

1. `GET /api/v1/extensions/installations/:id/capabilities`
2. `POST /api/v1/extensions/installations/:id/health-check`
3. `GET /api/v1/extensions/installations/:id/pages`

capabilities 返回 `{capabilities[], details[]}`；details 项含 `type/key/capability/
provider/operations[]/permissions/configKeys/source`。

pages 返回 `{pages[]}`（**不是 `items[]`**），页面项（`ExtensionPageItem`）：
`type`（binding|manifest）/ `key` / `title` / `route` / `icon` / `group` / `order` /
`requiredPermission` / `source` / `schema`。数据来源优先 runtime `page` 绑定，
回退 release manifest 的 `ui.pages`（此时仅 route，title=route）。

事件查询参数：`level` / `keyword` / `page` / `pageSize`；事件项
`{eventType, level, message, payload(string), createdBy, createdAt}`。

### 3.4 Agent 同步（server ↔ agent wire）

1. `GET /api/v1/agents/:agentId/extensions` —— Agent 轮询拉取（HTTP，默认 30s，
   `ExtensionSyncPuller`）；响应为 `{payload}` 包装，`payload` 即
   `AgentSyncPayload`（installations + bindings 的 agent 运行时视图，字段
   lowerCamelCase）。**该结构是 agent wire 契约**：修改需同步
   `internal/app/agent/extension_sync_puller.go` 的解析端。
2. `GET /api/v1/extensions/agents/:agentId/sync-payload` —— 同一 payload 的
   管理端预览（前端 AgentSync 页使用）。

> **已知边界**：Agent 同步走 HTTP 轮询而非自研 TCP 隧道推送。V1 保留该形态
> （实现完整、间隔可配），隧道推送列为演进项不承诺批次。

## 4. 错误码基线

必须稳定支持（实现位于 `internal/api/extension` 的 `mapServiceError` /
`errorx`）：

- `extension_already_installed`
- `dependency_blocked`
- `missing_dependency`
- `version_mismatch`
- `dependency_cycle`
- `forbidden`
- `not_found`
- `conflict`（升级目标同版本等）

## 5. 结构化错误 details 基线

`dependency_blocked`：

- `details.code = dependency_blocked`
- `details.blockers = ["extension@version", ...]`

`extension_already_installed`：

- `details.code = extension_already_installed`
- `details.installationId`
- `details.scopeType/scopeId`
- `details.targetType/targetId`

`missing_dependency` / `version_mismatch`：

- `details.missing` / `details.expected` / `details.actual`

## 6. 前端消费现状审计（2026-09-28，#46）

| 页面                                        | 消费端点                                                            | 结论                                                                                                                                                                       |
| ------------------------------------------- | ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Store（`Extensions/Store`）                 | catalog 列表/详情/releases、install                                 | ✅ 一致；adapter 层（`services/adapters/extensions.ts`）薄归一，`EXTENSION_ERROR_CODES` 按 §4 分支已接                                                                     |
| Installations（`Extensions/Installations`） | installations 列表/详情、enable/disable/reconcile/uninstall、events | ✅ 一致；详情/事件/升级三 overlay 受控组合                                                                                                                                 |
| AgentSync（`Extensions/AgentSync`）         | `sync-payload` 预览                                                 | ✅ 一致（只读 JSON 预览）                                                                                                                                                  |
| DomainEntry（`Extensions/DomainEntry`）     | installations 列表 → `listExtensionPages`                           | ✅ 已修（#46 批次 1）：canonical 端点 `installations/:id/pages`，先取首个安装实例再拉页面绑定；`.catch` 静默已移除，加载失败走页面统一错误提示。页面级真实渲染用例归批次 4 |

前端 `services/api/extensions.ts` 的类型/归一**不消费** `code`/`message`
（normalize 直取业务字段）——后端响应 DTO 去掉内嵌 `code/message` 对前端零影响；
唯一消费方是 agent puller 的 `payload` 包装（§3.4，随批次同步）。

## 7. 已知缺口与实施批次链（#46 收口路径）

1. ✅ **契约收口批次（2026-09-28 已落地）**：16 个响应 DTO 去内嵌 `code/message`
   字段；agent puller 解析端同步（wrapper 仅存 `payload`）；删除 compat 路由组
   （13 条）与 `resolveCompatInstallationID` 及其 18 个测试函数；
   `listExtensionPages` 切 canonical 端点、DomainEntry 静默吞错移除。
   已知边界：Extensions 四页面（Store/Installations/AgentSync/DomainEntry）
   此前零测试文件，本批仅 API 层 `extensions.test.ts` 回归（16 用例），页面级
   真实渲染用例归批次 4。
2. **列表组装修正**：`displayName` join catalog 真名；`healthStatus` 从
   status/enabled 推导（或引 runtime binding 状态），替换恒 `unknown`。
3. **catalog 写路径批次（V2 主缺口）**：admin catalog/release CRUD（登记/
   上下架/发布版本）+ 官方扩展 seed（`official.notification/alerting/approval/
backup-advanced`，对齐 `official-extension-unified-pattern.md`）；pack
   （`.tgz`，protoc-gen-croupier 产物）导入自动登记列为后续。
4. **DomainEntry 恢复实测**：批次 1 后页面入口区块真实渲染 binding/manifest
   pages，补真实渲染用例（替换恒 Empty 的现状）。
