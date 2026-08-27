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

「运维中心 → 证书监控」纳管平台相关 TLS 证书的到期与状态——**手动登记为主**（平台无法自动发现你内部 CA 签的证书），配合到期告警规则与[通知渠道](./notifications)联动。

## 登记证书

两种方式：

1. **表单登记**：域名 + 端口（默认 443）+ 可选 PEM 证书（+ 私钥，仅用于到期解析，不代管流量）
2. **TLS 拨号辅助**（`GET /api/v1/certificates/domain-info?domain=xxx`）：对任意域名现场 TLS 握手（10s 超时）读取证书信息，回填表单——适合先探测再登记

> 平台内部链路证书（transport/dispatch）的配置见 [TLS 与证书](./tls-certificates)；本页是**到期台账**，不负责下发。

## 状态机与复检

| 状态       | 判定                       |
| ---------- | -------------------------- |
| `active`   | 距到期 > 阈值              |
| `expiring` | 进入阈值窗口（默认 30 天） |
| `expired`  | 已过期                     |
| `unknown`  | 从未成功解析               |

- 单张复检 `POST /api/v1/certificates/:id/check`（解析存储的 PEM 更新到期/状态）
- 全量复检 `POST /api/v1/certificates/check-all`——建议定时触发（crontab 调 API，或平台定时调度托管）
- `GET /api/v1/certificates/expiring` 直接取「即将到期」清单
- `GET /api/v1/certificates/stats` 总览计数（active/expiring/expired）

## 到期告警

告警规则（`certificate_alerts` 表，页面表单或 `POST /api/v1/certificates/alerts`）：

| 字段              | 说明                    |
| ----------------- | ----------------------- |
| `domain`          | 证书域名                |
| `thresholdDays`   | 提前告警天数（默认 30） |
| `active`          | 规则开关                |
| `lastTriggeredAt` | 上次触发（防重复轰炸）  |

触发 `certificate_expiring` / `certificate_expired` 事件 → 按 ops 通知路由分发（如「到期事件只进钉钉值班群」）。

## 运维建议

- **纳管清单**：对外域名证书 + transport CA/服务证书 + 依赖的上游（数据库/Redis 若走 TLS）
- **节奏**：`check-all` 每日一次足够；告警阈值按签发周期设（Let's Encrypt 类 90 天证书用 14 天阈值，年签证书用 30/14/7 三段）
- 到期前的更换操作本身走平台审批流（证书更换属低危，双人复核即可）
