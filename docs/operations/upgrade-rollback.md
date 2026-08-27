---
title: 版本升级与回滚
icon: refresh
order: 6
category:
  - 运维手册
tag:
  - 部署
  - 升级
---

# 版本升级与回滚

## 迁移机制回顾（升级前必读）

- Schema 变更走**版本化迁移**（goose 编号迁移，0001–0014+，随二进制嵌入）：启动时自动 catch-up，无需手工执行 SQL（详见[数据库迁移策略](../architecture/database-migration-strategy.md)）
- 迁移全程持**跨进程锁**（MySQL `GET_LOCK` / Postgres `pg_advisory_lock` / SQL Server `sp_getapplock`）——多实例同时滚动重启是安全的，迁移只会被一个进程执行
- 每个二进制有 `MinimumRequiredVersion`：库上已应用版本**低于**要求时启动自动补齐；补齐后版本高于旧二进制要求时，**旧二进制会拒绝启动**（防旧代码写新库）

## 升级前置

1. 读 Release Notes：关注 `MinimumRequiredVersion` 提升与破坏性配置变更
2. 备份（见[备份与恢复](./backup-restore)）——尤其 meta 库（用户/角色/审计/成员表）
3. 确认磁盘与 DB 配额（迁移可能新增表/列）

## 单实例升级

```bash
# Compose
docker compose pull && docker compose up -d

# 二进制
systemctl stop croupier-server
cp bin/croupier-server /opt/croupier/bin/croupier-server.new && mv /opt/croupier/bin/croupier-server.new /opt/croupier/bin/croupier-server
systemctl start croupier-server
journalctl -u croupier-server -f | grep -i migrate   # 观察 catch-up 完成
```

验证：`/healthz` 200、`/api/v1` 的 version 与预期一致、Dashboard 首页可登录。

## HA 滚动升级（零断流）

逐台执行，任何一台验证不过立即停下排查：

```
摘流量（LB set state drain / 缩副本）→ 替换二进制 → 启动（自动迁移或空跑）
→ /healthz 通过 → 回流量 → 下一台
```

- Agent 侧无需操作：断连重连 + 重新注册自动迁移归属（owner 随连接更新）
- 第一台启动时持锁执行迁移，后续实例启动时迁移已到位、直接跳过（幂等 catch-up）
- K8s：`maxUnavailable: 0` + readinessProbe 保证滚动期间始终有存活实例

## 回滚语义（重要）

**迁移只向前，没有 down 路径。** 生产回滚二进制版本时：

| 场景                                                                 | 处理                                                                                      |
| -------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| 新版二进制**未引入**新迁移（版本号未提升 `MinimumRequiredVersion`）  | 直接回滚二进制/镜像即可                                                                   |
| 新版**已应用**新迁移，旧二进制 `MinimumRequiredVersion` ≤ 已应用版本 | 直接回滚（版本检查只拒绝「库太旧」，不拒绝「库更新」）；但需确认旧代码不写新列            |
| 旧二进制因版本检查拒绝启动                                           | 两个选择：**前滚修复**（优先，发布修复版）或 **恢复 DB 备份**（接受备份点之后的数据丢失） |

因此升级窗口的纪律是：**先备份、后升级；验证不过、先摘流量再决策**。

## 数据面组件

`agent`、`analytics-worker`、`ingest` 与 Server 版本解耦升级（session 协议兼容窗口以 Release Notes 为准）。Agent 滚动升级即断连重连，无状态可随时重启。
