---
title: 备份与恢复
icon: box
order: 30
category:
  - 运维手册
tag:
  - 备份
  - 恢复
---

# 备份与恢复

## 平台内置备份（Dashboard/API）

「运营中心 → 数据备份」页面与 `/api/v1/backups` 提供**数据库逻辑备份**：

- 支持驱动：`mysql`（`mysqldump`）、`postgres`（`pg_dump`）、`sqlite`（文件拷贝）
- 备份由 Server 执行；Server 容器/宿主机内必须存在对应工具（`mysqldump`/`pg_dump`）

```bash
# Trigger a full backup
curl -X POST https://<server>:18780/api/v1/backups \
  -H "Authorization: Bearer ***" \
  -d '{"name":"weekly-meta","type":"full"}'

# List / download / delete
curl -H "Authorization: Bearer ***" https://<server>:18780/api/v1/backups
curl -O -H "Authorization: Bearer ***" https://<server>:18780/api/v1/backups/{id}/download
curl -X DELETE -H "Authorization: Bearer ***" https://<server>:18780/api/v1/backups/{id}
```

调度：crontab 直接调 API（或用平台自身的定时调度功能托管）。

## 备份范围清单

| 对象                                                         | 方式                | 备注                                                     |
| ------------------------------------------------------------ | ------------------- | -------------------------------------------------------- |
| **meta 库**（用户/角色/审计/成员表/游戏注册表）              | 内置备份或外部 dump | 升级/回滚的生命线                                        |
| **游戏库**（`multiGame: true` 时每 `(game_id, env)` 一个库） | 逐库 dump           | 内置备份只覆盖 Server 所连的库；多库环境建议外部遍历脚本 |
| `data/` 目录                                                 | 文件备份            | assignments、上传文件（配置版本工件等）                  |
| 配置文件                                                     | Git 管理            | `server.yaml` 等纳管版本，不要依赖备份                   |
| Redis                                                        | 通常不需要          | 缓存/成员表都是可重建的运行时状态                        |

## 恢复步骤

**MySQL**：

```bash
mysql -h <host> -u root -p croupier_meta < weekly-meta.sql
```

**PostgreSQL**：

```bash
psql -h <host> -U postgres -d croupier_meta -f weekly-meta.sql
```

**SQLite**：停 Server → 替换 `data/croupier.db`（或对应 dataSource 路径）→ 启动。

恢复后首次启动的注意事项：

- 丢失恢复点之后新建的审计/任务记录属预期；Agent 会重新注册，成员表租约自动收敛

## 建议节奏

| 对象         | 频率              | 保留           |
| ------------ | ----------------- | -------------- |
| meta 库      | 每天 + 升级前手动 | 30 天          |
| 游戏库       | 每天              | 按游戏合规要求 |
| `data/` 目录 | 每天 rsync/快照   | 14 天          |

## 验证备份可用

没验证过的备份等于没有备份——每月跑一次恢复演练：

1. 恢复到隔离实例（不同端口/库名）
2. 启动同版本 Server 指向恢复库，`/healthz` 通过
3. 登 Dashboard 抽查：管理员能登录、函数目录完整、审计链可查
