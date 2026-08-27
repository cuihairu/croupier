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

「运维中心 → 数据备份」页面与 `/api/v1/backups` 提供**数据库逻辑备份**：

- 支持驱动：`mysql`（`mysqldump`）、`postgres`（`pg_dump`）、`sqlite`（文件复制）
- SQL Server 尚不支持内置备份（驱动支持进行中，见[迁移策略](../architecture/database-migration-strategy.md)），用 `sqlcmd BACKUP DATABASE` 外部备份
- 备份由 Server 执行，要求 Server 容器/宿主内存在对应工具（`mysqldump`/`pg_dump`）

```bash
# 触发全量备份
curl -X POST https://<server>:18780/api/v1/backups \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"weekly-meta","type":"full"}'

# 列表 / 下载 / 删除
curl -H "Authorization: Bearer $TOKEN" https://<server>:18780/api/v1/backups
curl -O -H "Authorization: Bearer $TOKEN" https://<server>:18780/api/v1/backups/{id}/download
curl -X DELETE -H "Authorization: Bearer $TOKEN" https://<server>:18780/api/v1/backups/{id}
```

定时化：crontab 直接调用 API（或用平台的定时调度功能托管自身）。

## 备份范围清单

| 对象                                                        | 方式                | 说明                                                   |
| ----------------------------------------------------------- | ------------------- | ------------------------------------------------------ |
| **meta 库**（用户/角色/审计/成员表/游戏注册表）             | 内置备份或外部 dump | 升级/回滚的生命线                                      |
| **game 库**（`multiGame: true` 时每 `(game_id, env)` 一库） | 逐库 dump           | 内置备份仅覆盖 Server 所连库，多库环境建议外部脚本遍历 |
| `data/` 目录                                                | 文件备份            | assignments、上传文件（配置版本制品等）                |
| 配置文件                                                    | git 管理            | `server.yaml` 等纳入版本控制，不依赖备份               |
| Redis                                                       | 一般无需            | 缓存/成员表均为可重建的运行时状态                      |

## 恢复步骤

**MySQL**：

```bash
mysql -h <host> -u root -p croupier_meta < weekly-meta.sql
```

**PostgreSQL**：

```bash
psql -h <host> -U postgres -d croupier_meta -f weekly-meta.sql
```

**SQLite**：停止 Server → 替换 `data/croupier.db`（或对应 dataSource 路径）→ 启动。

恢复后首次启动注意：

- 恢复的库带着旧迁移版本号，若当前二进制 `MinimumRequiredVersion` 更高，启动会自动补齐迁移（见[升级与回滚](./upgrade-rollback)）
- 恢复点之后的审计/任务记录丢失是预期行为；Agent 会重新注册，成员表租约自动收敛

## 建议节奏

| 对象         | 频率              | 保留           |
| ------------ | ----------------- | -------------- |
| meta 库      | 每日 + 升级前手动 | 30 天          |
| game 库      | 每日              | 按游戏合规要求 |
| `data/` 目录 | 每日 rsync/快照   | 14 天          |

## 验证备份可用性

备份没有验证等于没有备份——每月抽一次恢复演练：

1. 恢复到隔离实例（不同端口/库名）
2. 启动同版本 Server 指向恢复库，`/healthz` 通过
3. 登录 Dashboard 抽查：管理员可登录、函数目录完整、审计链可查
