---
title: 二进制部署（systemd）
icon: server
order: 4
category:
  - 运维手册
tag:
  - 部署
---

# 二进制部署（systemd）

适合无容器约束的裸机/VM 环境。产物来自 [Release](https://github.com/cuihairu/croupier/releases) 或本地构建（`make build` → `bin/croupier-server`、`bin/croupier-agent`）。

## 前置依赖

- Linux x86_64，Go 1.26+（仅本地构建需要）
- MySQL 8.x / PostgreSQL 16（生产）——SQLite 仅适合单机试点
- Redis 7+（多实例必须；单实例可退化为 `cache.type: memory`）
- 备份执行器依赖命令行工具：`mysqldump`（MySQL）/ `pg_dump`（PostgreSQL）——见[备份与恢复](./backup-restore)

## 目录规划

```
/opt/croupier/
├── bin/            # croupier-server, croupier-agent
├── etc/            # server.yaml, agent.yaml, rbac.json, ...
├── data/           # assignments.json、上传文件等运行时数据
└── logs/           # （可选）文件日志；默认 stdout 交给 journald
```

## 配置

从仓库模板起步并按[Server 配置全解](./config-server)调整：

```bash
cp configs/server.yaml /opt/croupier/etc/server.yaml
# 生产必改：auth.jwtSecret、database.dataSource、cache 密码、server.mode: prod
```

配置校验（不启动服务）：

```bash
/opt/croupier/bin/croupier-server --config /opt/croupier/etc/server.yaml validate
```

## systemd 单元

`/etc/systemd/system/croupier-server.service`：

```ini
[Unit]
Description=Croupier Server
After=network-online.target mysql.service redis.service
Wants=network-online.target

[Service]
Type=simple
User=croupier
Group=croupier
WorkingDirectory=/opt/croupier
ExecStart=/opt/croupier/bin/croupier-server --config /opt/croupier/etc/server.yaml
Restart=on-failure
RestartSec=5
# 敏感值建议：YAML 里写 $(VAR) 占位，加载时按环境变量展开
# （config_loader 的 os.ExpandEnv），秘钥不落盘：
#   auth:
#     jwtSecret: "$(CROUPIER_JWT_SECRET)"
# Environment=CROUPIER_JWT_SECRET=xxx
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/croupier/data

[Install]
WantedBy=multi-user.target
```

`croupier-agent.service` 同构，差异点：

```ini
[Unit]
After=network-online.target
[Service]
ExecStart=/opt/croupier/bin/croupier-agent --config /opt/croupier/etc/agent.yaml
# Agent 无本地状态目录要求；上游地址指向 Server 入口（LB 或直连）
```

启动与自启：

```bash
systemctl daemon-reload
systemctl enable --now croupier-server
systemctl enable --now croupier-agent
```

## 验证

```bash
systemctl status croupier-server
curl -fsS http://127.0.0.1:18780/healthz          # {"status":"ok"}
curl -fsS http://127.0.0.1:18780/api/v1           # 版本 + features 元信息
journalctl -u croupier-server -f                  # 启动期迁移日志（goose catch-up）
```

Agent 侧验证：Server 日志出现 session 注册即通；Dashboard「节点维护」页可见 agent 在线。

## 多实例（裸机 HA）

1. 每个 Server 实例一份 `server.yaml`，`cluster.enabled: true` 且 `advertiseAddr` 填**对端可达地址**
2. 前置 HAProxy/nginx 分流（配置模板见[负载均衡](./load-balancing)）
3. Agent `server.addr` 指向 LB 入口，不指具体实例

## 常见坑

- **首次启动慢**：版本化迁移在启动时 catch-up（新库跑 baseline AutoMigrate），多表时需几十秒，不是卡死
- **`data/` 权限**：`ProtectSystem=strict` 下必须显式 `ReadWritePaths`，否则 assignments/上传写入失败
- **时区**：MySQL DSN 需带 `loc=Local`，否则审计/任务时间偏移
