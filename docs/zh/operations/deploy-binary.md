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

适合没有容器约束的裸金属/虚拟机环境。产物来自 [Releases](https://github.com/cuihairu/croupier/releases) 或本地构建（`make build` → `bin/croupier-server`、`bin/croupier-agent`）。

> 如果只需要 agent，用一键安装脚本（自动探测架构、匿名下载、幂等升级、可选注册服务）；本页覆盖完整的手工部署（目录布局、Server 配置、生产级 systemd unit）。

## 前置条件

- Linux x86_64，Go 1.27.3+（仅本地构建需要）
- MySQL 8.x / PostgreSQL 16（生产）——SQLite 只适合单机试点
- Redis 7+（多实例必需；单实例可降级为 `cache.type: memory`）

## 目录布局

```
/opt/croupier/
├── bin/            # croupier-server, croupier-agent
├── etc/            # server.yaml, agent.yaml, rbac.json, ...
├── data/           # assignments.json, uploaded files, and other runtime data
└── logs/           # (optional) file logs; by default stdout goes to journald
```

## 配置

从仓库模板起步，按 Server 配置全量参考调整：

```bash
cp configs/server.yaml /opt/croupier/etc/server.yaml
# Must change for production: auth.jwtSecret, database.dataSource, cache password, server.mode: prod
```

配置校验（不启动服务）：

```bash
/opt/croupier/bin/croupier-server --config /opt/croupier/etc/server.yaml validate
```

## systemd unit

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
# Recommended for sensitive values: write a $(VAR) placeholder in YAML, expanded from environment variables at load time
# (config_loader's os.ExpandEnv), so secrets never touch disk:
#   auth:
#     jwtSecret: "$(CROUPIER_JWT_SECRET)"
# Environment=CROUPIER_JWT_SECRET=xxx
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/croupier/data

[Install]
WantedBy=multi-user.target
```

`croupier-agent.service` 形态相同，差异如下：

```ini
[Unit]
After=network-online.target
[Service]
ExecStart=/opt/croupier/bin/croupier-agent --config /opt/croupier/etc/agent.yaml
# The Agent has no local state directory requirements; the upstream address points at the Server entry point (LB or direct)
```

启动并设开机自启：

```bash
systemctl daemon-reload
systemctl enable --now croupier-server
systemctl enable --now croupier-agent
```

## 验证

```bash
systemctl status croupier-server
curl -fsS http://127.0.0.1:18780/healthz          # {"status":"ok"}
curl -fsS http://127.0.0.1:18780/api/v1           # version + features metadata
journalctl -u croupier-server -f                  # startup migration logs (goose catch-up)
```

Agent 侧验证：Server 日志出现会话注册即通；Dashboard「节点维护」页能看到 agent 在线。

## 多实例（裸金属 HA）

1. 每个 Server 实例一份 `server.yaml`，`cluster.enabled: true`，`advertiseAddr` 设为**对端可达地址**
2. Agent 的 `server.addr` 指向 LB 入口，而不是具体实例

## 常见坑

- **首次启动慢**：版本化迁移在启动时追平（全新库会跑基线 AutoMigrate），表多时可能几十秒——不是卡死
- **`data/` 权限**：`ProtectSystem=strict` 下必须显式 `ReadWritePaths`，否则 assignments/上传写不进
- **时区**：MySQL DSN 需要 `loc=Local`，否则审计/任务时间戳偏移
