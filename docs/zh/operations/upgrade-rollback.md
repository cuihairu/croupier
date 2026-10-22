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

- 全部迁移在**跨进程锁**下执行（MySQL `GET_LOCK` / Postgres `pg_advisory_lock` / SQL Server `sp_getapplock`）——多实例同时滚动重启是安全的，迁移只由一个进程执行
- 每个二进制带 `MinimumRequiredVersion`：数据库已应用版本**低于**要求时启动自动追平；追平后数据库版本高于旧二进制要求时，**旧二进制拒绝启动**（防止旧代码写新 schema）

## 升级前检查清单

1. 读 Release Notes：关注 `MinimumRequiredVersion` 提升与破坏性配置变更
2. 确认磁盘与 DB 配额（迁移可能加表/加列）

## 单实例升级

```bash
# Compose
docker compose pull && docker compose up -d

# Binary
systemctl stop croupier-server
cp bin/croupier-server /opt/croupier/bin/croupier-server.new && mv /opt/croupier/bin/croupier-server.new /opt/croupier/bin/croupier-server
systemctl start croupier-server
journalctl -u croupier-server -f | grep -i migrate   # Watch for catch-up completion
```

验证：`/healthz` 返回 200、`/api/v1` 报告预期版本、Dashboard 首页可登录。

## HA 滚动升级（零断流）

逐节点执行；任一节点验证失败立即停手排查：

```
Drain traffic (LB set state drain / scale down replicas) → Replace binary → Start (automatic migration or dry run)
→ /healthz passes → Restore traffic → Next node
```

- Agent 侧无需操作：断线重连 + 重新注册自动迁移归属（owner 随连接走）
- 第一个启动的节点持锁跑迁移；后续实例启动时迁移已就位直接跳过（幂等追平）
- K8s：`maxUnavailable: 0` + readinessProbe 保证全程有存活实例

## 回滚语义（重要）

**迁移只向前，没有 down 路径。**生产回滚二进制版本时：

| 场景                                                                     | 处置                                                                                     |
| ------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- |
| 新二进制**没有引入**新迁移（`MinimumRequiredVersion` 未提升）            | 直接回滚二进制/镜像即可                                                                  |
| 新版本**已应用**新迁移，且旧二进制 `MinimumRequiredVersion` ≤ 已应用版本 | 可直接回滚（版本检查只拒「库太旧」，不拒「库更新」）；但需确认旧代码不写新列             |
| 旧二进制因版本检查拒绝启动                                               | 两条路：**向前修复**（首选，发 patched release）或**恢复 DB 备份**（接受备份点后丢数据） |

因此升级窗口的纪律：**先备份后升级；验证不过先 drain 再决策**。

## 数据面组件

`agent`、`analytics-worker`、`ingest` 与 Server 版本解耦（会话协议兼容窗口以 Release Notes 为准）。Agent 滚动升级就是断线重连，无状态、随时可重启。

## 托管程序更新与指纹审计（2026-10-14 定稿，随 R2-R4 交付）

> 本节针对 agent 节点上受管进程的二进制更新（游戏辅助程序 / 外部探测适配器 / **游戏进程本身**）；上文讲的是 Croupier 自身组件升级。设计与权威源见 Agent Supervisor 设计（仅中文小节）§10.4。

- **每次更新必须留指纹**（binary sha256）：字段包括目标标识、执行位置（agentID）、旧指纹 → 新指纹、时间戳、操作者/来源、校验结果（hash/签名/兼容性）、结果（成功/失败/**rolled_back**——回滚也审计）。
- **查询模型**：按「目标 + 时间 + 指纹」检索执行审计（execlog）；一条日志带指纹 + 版本串，出问题可回溯到引入它的那次更新。告警/探测时间线与更新记录按（目标, 时间）对齐，定位「哪次更新之后开始告警」。
- **游戏进程自身的额外前置**：维护窗口或在线人数低于阈值 + 暂停调度与新连接接收（**探测不能停**，否则更新失败不可见）+ 失败回滚。
- Agent 自升级同样留指纹（经注册/心跳版本信息暴露）；排查 agent 缺失时，服务端最后已知版本 + 指纹是锚点。
- 现状：**未实现**——今天受管面只有监控/自动重启/事件记录（S1-S3），更新能力与指纹随 §10 落地。
