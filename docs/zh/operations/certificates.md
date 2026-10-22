---
title: 证书监控
icon: certificate
order: 41
category:
  - 运维手册
tag:
  - 安全
---

# 证书监控

「运营中心 → 证书监控」跟踪平台相关 TLS 证书的到期与状态——以**人工登记为主**（平台无法自动发现你内部 CA 签发的证书），配合到期告警规则与通知渠道集成。

## 登记证书

两种方式：

1. **表单登记**：域名 + 端口（默认 443）+ 可选 PEM 证书（+ 私钥，仅用于到期解析，绝不参与流量处理）
2. **TLS 拨测助手**（`GET /api/v1/certificates/domain-info?domain=xxx`）：对任意域名做一次真实 TLS 握手（10s 超时）读取证书信息并回填表单——适合登记前先探测

> 平台内部链路证书（transport/dispatch）的配置见 TLS 与证书；本页是**到期台账**，不负责签发。

## 状态机与复检

| 状态       | 判定标准                     |
| ---------- | ---------------------------- |
| `active`   | 距到期日超过阈值             |
| `expiring` | 已进入阈值窗口（默认 30 天） |
| `expired`  | 已过期                       |
| `unknown`  | 从未解析成功                 |

- 单证书复检 `POST /api/v1/certificates/:id/check`（解析已存 PEM 并更新到期时间/状态）
- 全量复检 `POST /api/v1/certificates/check-all`——需定期调度（crontab 调 API，或交给平台定时调度器托管）
- `GET /api/v1/certificates/expiring` 直接拉取「即将到期」列表
- `GET /api/v1/certificates/stats` 给出总览计数（active/expiring/expired）

## 到期告警

告警规则（`certificate_alerts` 表，页面表单或 `POST /api/v1/certificates/alerts`）：

| 字段              | 说明                         |
| ----------------- | ---------------------------- |
| `domain`          | 证书域名                     |
| `thresholdDays`   | 到期前多少天告警（默认 30）  |
| `active`          | 规则开关                     |
| `lastTriggeredAt` | 上次触发时间（防止重复轰炸） |

触发 `certificate_expiring` / `certificate_expired` 事件 → 按运营通知路由分发（例如「到期事件只进钉钉值班群」）。

## 运营建议

- **管住清单**：公网域名证书 + transport CA/服务端证书 + 依赖的上游（数据库/Redis 若启用 TLS）
- **节奏**：每天一次 `check-all` 足够；告警阈值按签发周期设置（Let's Encrypt 这类 90 天证书用 14 天阈值；年检证书用 30/14/7 三档阶梯）
- 到期前的更换动作走平台审批流（证书更换属低风险，两人复审即可）
