---
title: 第三方平台集成
icon: link
order: 4
category:
  - 集成指南
tag:
  - 集成
  - 第三方平台
  - OpenAPI
---

# 第三方平台集成

第三方平台集成负责把外部能力接入 Croupier 的 FunctionContract、受控执行与页面生成体系；它不定义页面、菜单、列或按钮位置。

```text
Third-party OpenAPI / Provider
  -> FunctionContract
  -> CapabilitySemantics
  -> PageProposal
  -> PublishedPageSpec
  -> Console binding execute
  -> Agent / Provider / Third-party API
```

## 集成方式

| 方式 | 适用场景 | 执行边界 |
| --- | --- | --- |
| OpenAPI Source 上传 + Provider binding | 已有 OpenAPI，业务 handler 已在 Provider 中 | Server 通过受控 binding 调用 Provider |
| OpenAPI Source + controlled HTTP connector | 平台审核过的外部 HTTP API | allowlist、SecretRef、超时、重试、审计后才可启用 |
| 专用 Provider | 签名、限流、协议转换复杂 | Provider 封装 HTTP 细节，只暴露 FunctionContract |

OpenAPI Source 未绑定执行器时，只是契约/语义/Proposal 目录，不得发布可执行页面。

## Dashboard 边界

- OpenAPI REST 可帮助识别 CRUD capability；非标准接口通过受控 `capability` 或 Resource Catalog 补充。
- Provider 只返回输入/输出 schema、resource、capability、execution、risk、permission 等能力事实。
- PageProposal 由 Server 产生；动态导航、列、动作位置和表单展示由 PageSpec/Page Studio 管理。
- Console 只调用 published binding execute API。浏览器不提供第三方 URL、header、secret、target 或 scope。

## QuickSDK 等专用 Provider

专用 Provider 适合签名固定、限流复杂或响应需要归一化的平台。它可以注册：

| 函数 | resource | capability |
| --- | --- | --- |
| `quicksdk.channel.list` | `quicksdk.channel` | `collection_query` |
| `quicksdk.role.get` | `quicksdk.role` | `item_query` |
| `quicksdk.message.push` | `quicksdk.message` | `action` |
| `quicksdk.report.day` | `quicksdk.report` | `report` |

Provider 不提交页面 schema、组件树、页面编排或菜单。平台按相同流程生成 Proposal；无 CRUD 语义时自动得到安全 OperationPage，而不是空白页面。

## 安全约束

- Agent 主动连接 Server；Server 不回拨 Agent 的 `rpc_addr`。
- HTTP connector 必须由平台配置 allowlist、SecretRef、请求策略、超时、重试、限流和审计；不允许用户在页面或 OpenAPI 中填写任意 URL/header。
- Page binding 执行要经过 scope、权限、审批、风险、审计与 OTel。
- 函数/语义变化使受影响 PublishedPageSpec stale，直到用户合并并重新发布。
