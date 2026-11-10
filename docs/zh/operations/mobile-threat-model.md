---
title: 移动端威胁模型
icon: shield-alert
order: 60
category:
  - 安全
tag:
  - 移动端
  - 威胁模型
  - 伴侣端
---

# 移动端威胁模型

移动伴侣端（`mobile/`，Flutter）的威胁模型。它是**伴侣客户端**：与
Web 控制台调用同一套控制面 REST API，不承担游戏侧 agent/SDK 角色。
作用域与资产边界见[移动伴侣设计稿](../../archive/legacy-en/design/mobile-companion-design.md)（仅英文版）。

模型按资产做 STRIDE 风格枚举。资产按影响分级（高/中/低）。每条记录
**当前缓解**（代码已做的防护）与**接受的风险**（明知不防、附理由）。

## 资产

| 资产                          | 位置                                                             | 影响 |
| ----------------------------- | ---------------------------------------------------------------- | ---- |
| 会话令牌（Bearer JWT）        | `core/storage/session_store.dart`（经 `flutter_secure_storage`） | 高   |
| 服务器地址（端点身份）        | 同 store（登出后保留）                                           | 中   |
| 作用域选择（`game_id`/`env`） | 同 store；以 `X-Game-ID`/`X-Env` 发送                            | 中   |
| API 响应（审计、审批、告警）  | 仅内存，按页 controller                                          | 中   |
| 设备身份 / 探活结果           | `core/server/server_probe.dart`                                  | 低   |

## 威胁表

| #   | 威胁                        | 资产       | STRIDE   | 影响 | 当前缓解                                                                                                                                                 | 接受的风险                                                                                                            |
| --- | --------------------------- | ---------- | -------- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| M-1 | 令牌从应用存储被窃取        | 会话令牌   | 信息泄露 | 高   | 令牌存 `flutter_secure_storage`（keystore / Keychain），从不落 `SharedPreferences` 或明文文件；`SessionData` 明示「不落密码、TOTP 种子、生物特征数据」。 | 在 root / jailbreak 设备上系统 keystore 较弱；接受——伴侣端面向运营人员手机，非对抗硬件。                              |
| M-2 | 令牌对另一端点重放          | 会话令牌   | 欺骗     | 高   | 令牌由服务端限定（audience/expiry），应用只发给配置的 `serverUrl`；`ApiClient` 每请求注入 `Authorization: Bearer`。                                      | 捕获的令牌可在过期前重放，前提是服务端 TLS 被降级；那是服务端边界（见 `docs/operations/security.md`），非移动端控制。 |
| M-3 | 经请求选项注入作用域头      | 作用域选择 | 篡改     | 中   | `ApiClient` 注入前先剥除调用方传入的 `X-Game-ID`/`X-Env`（对齐 web 拦截器语义）。                                                                        | 应用层无；防御在服务端（原子校验作用域对）。                                                                          |
| M-4 | 401 清会话后留下半配置应用  | 会话令牌   | 拒绝服务 | 低   | `onUnauthorized` 清会话但**保留 `serverUrl`**，登录页据此预填（地址属配置非凭据）。                                                                      | 运营人员需重输凭据；无安全影响。                                                                                      |
| M-5 | 敏感数据落盘                | API 响应   | 信息泄露 | 中   | 响应仅存 Riverpod controller 状态（内存），从不把审计/审批载荷写存储。                                                                                   | 截图/系统快照可捕获已渲染数据；对短会话的运营工具接受。                                                               |
| M-6 | 经深链/二维码伪装服务器地址 | 服务器地址 | 欺骗     | 中   | `ServerProbe` 先探活端点、返回探活结果后才允许登录；地址由运营人员输入。                                                                                 | 移动端无 pinning（标准 `dio` TLS 校验）；设计将控制台主机视为可信基础设施。                                           |
| M-7 | 日志泄露令牌                | 会话令牌   | 信息泄露 | 中   | `ApiClient`/`SessionStore` 无令牌 `debugPrint`；错误归一走 `ApiError`（状态+消息），不回声请求头。                                                       | 未发现；未来改动若打印请求选项必须脱敏 `Authorization`。                                                              |

## 验证

- `mobile/test/core/storage/session_store_test.dart` 与
  `session_store_secure_test.dart` 覆盖 secure-storage 契约与「serverUrl
  登出后保留」（M-4）。
- `mobile/test/core/api/api_client_test.dart` 与 `api_error_test.dart`
  覆盖令牌注入、作用域头剥除（M-3）与 401 → 清会话 → 跳转（M-4）。
- `.github/workflows/ci-mobile.yml` 覆盖率门禁（80% 下限）持续覆盖安全
  相关路径。

## 已知边界

- 本模型仅覆盖**伴侣端**。服务端、agent、SDK 与 wire 协议信任边界见
  `docs/operations/security.md` 与 `docs/architecture/transport-glossary.md`。
- 伴侣端无生物识别/PIN 应用锁（设计稿 M2 范围）；已解锁且未锁屏的设备
  暴露活跃会话直至服务端令牌过期。作为面向运营工具的产品决策接受。
