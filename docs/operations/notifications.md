---
title: 通知渠道
icon: notification
order: 23
category:
  - 运维手册
tag:
  - 可观测性
  - 通知
---

# 通知渠道

平台事件的统一出口：**审批事件、告警触发、函数调用事件**。四个渠道独立开关，配置热生效（L3 覆盖层，见[配置分层](../architecture/config-layering)）。

## 渠道与配置键

| 渠道           | 配置键（`PUT /api/v1/site/<key>`）                                                               | 说明                                        |
| -------------- | ------------------------------------------------------------------------------------------------ | ------------------------------------------- |
| 站内信         | `notification.inAppEnabled`（默认开）                                                            | 写 `messages` 表 + SSE 实时推送，零外部依赖 |
| 钉钉机器人     | `notification.dingtalkUrl` / `dingtalkSecret`                                                    | 机器人 Webhook + 加签                       |
| 自定义 webhook | `notification.webhookUrl` / `webhookSecret`                                                      | POST JSON 事件体，可对接企微/飞书自建应用   |
| SMTP 邮件      | `notification.emailEnabled` + `smtpHost` / `smtpPort` / `smtpUser` / `smtpPassword` / `smtpFrom` | 密码写后脱敏（只回显「已设置 + 尾 4 位」）  |

配置方式（Dashboard「系统管理 → 网站配置」，或 API 直改）：

```bash
# 设置钉钉机器人
curl -X PUT https://<server>:18780/api/v1/site/notification.dingtalkUrl \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"value":"https://oapi.dingtalk.com/robot/send?access_token=xxx"}'

# 查看渠道快照（含脱敏与来源标注）
curl -H "Authorization: Bearer $TOKEN" https://<server>:18780/api/v1/site/notification

# 清除覆盖 = 回落跟随配置文件
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  https://<server>:18780/api/v1/site/notification.dingtalkUrl
```

键值语义：value 是 JSON 值（字符串/布尔）；删除覆盖即回退 YAML 里的默认——**改错了不重启就能回滚**。

## 事件与绑定

| 事件                                                       | 触发源                     |
| ---------------------------------------------------------- | -------------------------- |
| 审批流转（待审批/通过/驳回）                               | 审批中心                   |
| 告警触发（`alert.fired`）                                  | [告警规则引擎](./alerts)   |
| 证书到期（`certificate_expiring` / `certificate_expired`） | [证书监控](./certificates) |
| 函数调用事件                                               | 调用链路                   |

Ops 侧另有事件→渠道路由（`GET/PUT /api/v1/ops/notifications`，存 `ops_state.json`）：`{channels:[{id,type,url,secret}], rules:[{event, channels, thresholdDays}]}`——可按事件挑选哪些渠道接收（如证书到期只进钉钉不进邮件）。

## 可靠性语义

- 分发失败**只记日志、不阻塞业务**——通知是尽力而为，告警落库才是事实源
- webhook 接收方要求幂等（可能收到重复推送）
- 密钥类（SMTP 密码/加签 secret）读取永远脱敏，审计日志同步脱敏

## 排障

| 症状               | 检查                                                                           |
| ------------------ | ------------------------------------------------------------------------------ |
| 站内信有、钉钉没有 | `GET /api/v1/site/notification` 确认 url 已设置且渠道未被 ops 路由排除         |
| webhook 收不到     | Server 出口到目标地址的连通性；Server 日志 `notify` 关键字                     |
| 邮件全部失败       | SMTP 账号/端口（465 SSL 与 587 STARTTLS 按服务商）；密码是否被当脱敏占位符写回 |
| 改了配置没生效     | L3 覆盖是即时的；确认改的是 `notification.*` 键而非 YAML（YAML 需重启）        |
