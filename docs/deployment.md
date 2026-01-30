---
title: 部署指南
icon: rocket
order: 5
category:
  - 入门指南
tag:
  - 部署
  - 运维
---

# 部署指南

本指南介绍 Croupier 各组件在生产环境中的部署方法。

## 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         公网 / DMZ                              │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Croupier Server                       │  │
│  │  gRPC: 8443 (mTLS)  │  HTTP: 8080 (REST API)           │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────┬───────────────────────────────────┘
                              │ mTLS (Agent 主动连接)
                              │
┌─────────────────────────────▼───────────────────────────────────┐
│                      游戏内网 (私有网络)                         │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Croupier Agent                        │  │
│  │  本地监听: 19090 (gRPC)  │  上行连接: Server:8443       │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                        │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │                   游戏服务器                              │  │
│  │  通过 SDK 向 Agent 注册函数，接收 GM 操作调用             │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**组件说明：**

| 组件 | 部署位置 | 网络方向 | 说明 |
|------|----------|----------|------|
| **Server** | DMZ/公网 | 被动监听 | 中央控制面，提供 API 和 Agent 注册 |
| **Agent** | 游戏内网 | 主动出站 | 本地代理，连接 Server 并注册游戏函数 |
| **Edge** (可选) | DMZ/中转区 | 双向代理 | 中继 Server 与 Agent 之间的连接 |

## 目录

[[toc]]

## 快速开始

### 前置准备

1. **准备二进制文件**：从 [Release 页面](https://github.com/cuihairu/croupier/releases) 下载对应平台的二进制文件，或自行编译：
   ```bash
   make build
   # 二进制文件位于 bin/ 目录
   ls bin/  # croupier-server, croupier-agent, croupier-edge
   ```

2. **准备配置文件**：参考 [安装指南](./guide/installation.md) 中的配置说明

3. **生成 TLS 证书**（生产环境必须）：
   ```bash
   ./scripts/dev-certs.sh
   # 或使用已有 CA 证书签发服务端和客户端证书
   ```

---

## Windows 部署

### 使用服务安装脚本

Windows 提供了 PowerShell 脚本，可将 Server/Agent 注册为系统服务，实现开机自启和自动重启。

#### Server 部署

```powershell
# 以管理员身份打开 PowerShell，进入项目目录
cd E:\croupier\server

# 安装服务
.\scripts\install-windows-service.ps1 Install

# 查看服务状态
.\scripts\install-windows-service.ps1 Status

# 启动服务
.\scripts\install-windows-service.ps1 Start
```

**默认安装位置：**

| 类型 | 路径 |
|------|------|
| 安装目录 | `C:\Program Files\Croupier` |
| 配置目录 | `C:\ProgramData\Croupier\config` |
| 数据目录 | `C:\ProgramData\Croupier` |
| 日志目录 | `C:\ProgramData\Croupier\logs` |

#### Agent 部署

```powershell
# 设置环境变量指定 Agent 配置
$env:CROUPIER_SERVICE_NAME = "CroupierAgent"
$env:CROUPIER_DISPLAY_NAME = "Croupier Agent - Game Server Proxy"
$env:CROUPIER_BIN_PATH = "E:\croupier\bin\croupier-agent.exe"

# 安装服务
.\scripts\install-windows-service.ps1 Install
```

#### 服务管理命令

```powershell
# 查看状态
.\scripts\install-windows-service.ps1 Status

# 启动/停止/重启
.\scripts\install-windows-service.ps1 Start
.\scripts\install-windows-service.ps1 Stop
.\scripts\install-windows-service.ps1 Restart

# 启用/禁用开机自启
.\scripts\install-windows-service.ps1 Enable
.\scripts\install-windows-service.ps1 Disable

# 卸载服务
.\scripts\install-windows-service.ps1 Uninstall
```

#### 使用 Windows 服务管理器

```powershell
# 使用 PowerShell 服务管理
Get-Service CroupierServer
Start-Service CroupierServer
Stop-Service CroupierServer
Restart-Service CroupierServer

# 或使用 services.msc 图形界面
```

#### 查看日志

```powershell
# 实时查看日志
Get-Content 'C:\ProgramData\Croupier\logs\croupier.log' -Tail 50 -Wait

# 或使用事件查看器 (Event Viewer)
evvntvwr.msc
```

#### 自定义安装参数

```powershell
# 自定义安装目录
$env:CROUPIER_INSTALL_DIR = "D:\Croupier"
$env:CROUPIER_DATA_DIR = "D:\Croupier\data"
$env:CROUPIER_LOG_DIR = "D:\Croupier\logs"
$env:CROUPIER_CONFIG_DIR = "D:\Croupier\config"

.\scripts\install-windows-service.ps1 Install
```

---

## Linux 部署 (systemd)

### 使用服务安装脚本

Linux 提供 systemd 脚本，适用于 Ubuntu、Debian、CentOS、RHEL 等发行版。

#### Server 部署

```bash
# 安装服务
sudo ./scripts/install-systemd.sh install

# 编辑配置文件
sudo vim /etc/croupier/server.yaml

# 启用并启动服务
sudo systemctl enable --now croupier-server

# 查看服务状态
sudo systemctl status croupier-server
```

**默认安装位置：**

| 类型 | 路径 |
|------|------|
| 安装目录 | `/opt/croupier` |
| 二进制目录 | `/opt/croupier/bin` |
| 配置目录 | `/etc/croupier` |
| 数据目录 | `/var/lib/croupier` |
| 日志目录 | `/var/log/croupier` |
| 运行用户 | `croupier` |

#### Agent 部署

```bash
# 指定 Agent 二进制文件安装
sudo CROUPIER_BIN_SRC=./bin/croupier-agent \
     CROUPIER_SERVICE_NAME=croupier-agent \
     ./scripts/install-systemd.sh install
```

#### 服务管理命令

```bash
# 查看状态
sudo systemctl status croupier-server

# 启动/停止/重启
sudo systemctl start croupier-server
sudo systemctl stop croupier-server
sudo systemctl restart croupier-server

# 启用/禁用开机自启
sudo systemctl enable croupier-server
sudo systemctl disable croupier-server

# 重新加载配置（修改配置后）
sudo systemctl daemon-reload
sudo systemctl restart croupier-server
```

#### 查看日志

```bash
# 实时查看日志
sudo journalctl -u croupier-server -f

# 查看最近 100 条日志
sudo journalctl -u croupier-server -n 100

# 查看今天日志
sudo journalctl -u croupier-server --since today

# 查看带时间的日志
sudo journalctl -u croupier-server --since "2024-01-01" --until "2024-01-02"
```

#### 卸载服务

```bash
sudo ./scripts/install-systemd.sh uninstall

# 完全删除（包括用户和数据目录）
sudo ./scripts/install-systemd.sh uninstall
sudo userdel croupier
sudo rm -rf /opt/croupier /etc/croupier /var/lib/croupier /var/log/croupier
```

#### 自定义安装参数

```bash
# 自定义安装目录和用户
sudo CROUPIER_USER=myuser \
     CROUPIER_HOME=/opt/my-croupier \
     CROUPIER_CONFIG_DIR=/etc/my-croupier \
     ./scripts/install-systemd.sh install
```

---

## macOS 部署 (launchd)

### 使用服务安装脚本

macOS 使用 launchd 服务管理，通过 LaunchDaemon 实现开机自启。

#### Server 部署

```bash
# 安装服务
sudo ./scripts/install-launchd.sh install

# 编辑配置文件
sudo vim /usr/local/etc/croupier/server.yaml

# 加载并启动服务
sudo ./scripts/install-launchd.sh load
sudo ./scripts/install-launchd.sh start

# 查看服务状态
./scripts/install-launchd.sh status
```

**默认安装位置：**

| 类型 | 路径 |
|------|------|
| 安装目录 | `/usr/local/croupier` |
| 二进制目录 | `/usr/local/croupier/bin` |
| 配置目录 | `/usr/local/etc/croupier` |
| 数据目录 | `/usr/local/var/croupier` |
| 日志目录 | `/usr/local/var/log/croupier` |

#### Agent 部署

```bash
# 指定 Agent 二进制文件安装
sudo CROUPIER_BIN_SRC=./bin/croupier-agent \
     CROUPIER_SERVICE_NAME=com.github.cuihairu.croupier.agent \
     ./scripts/install-launchd.sh install
```

#### 服务管理命令

```bash
# 查看状态
./scripts/install-launchd.sh status

# 启动/停止/重启
sudo ./scripts/install-launchd.sh start
sudo ./scripts/install-launchd.sh stop
sudo ./scripts/install-launchd.sh restart

# 加载/卸载 LaunchDaemon
sudo ./scripts/install-launchd.sh load
sudo ./scripts/install-launchd.sh unload
```

#### 查看日志

```bash
# 实时查看日志
tail -f /usr/local/var/log/croupier/croupier-server.log

# 查看错误日志
tail -f /usr/local/var/log/croupier/croupier-server.err

# 使用 log 命令查看系统日志
log show --predicate 'process == "croupier-server"' --last 1h
```

#### 卸载服务

```bash
sudo ./scripts/install-launchd.sh unload
sudo ./scripts/install-launchd.sh uninstall

# 完全删除
sudo dscl . delete /Users/croupier
sudo rm -rf /usr/local/croupier /usr/local/etc/croupier /usr/local/var/croupier
```

---

## Docker 部署

### 使用 Docker Compose

```bash
# 启动所有服务
docker-compose up -d

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

### 单独部署

```bash
# Server
docker run -d \
  --name croupier-server \
  --restart unless-stopped \
  -p 8443:8443 \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  croupier-server:latest

# Agent
docker run -d \
  --name croupier-agent \
  --restart unless-stopped \
  -v $(pwd)/agent-data:/app/data \
  -e CROUPIER_AGENT_SERVER_ADDR=server:8443 \
  croupier-agent:latest
```

---

## GeoIP / IP2Location（可选）

若希望在日志/审计/审批等页面展示"属地"（国家/省/市），可启用以下任一方案：

### 1. 离线库（推荐）

- 下载 IP2Location LITE DB（免费）：
  - IPv4：IP2LOCATION-LITE-DB3.BIN
  - IPv6：IP2LOCATION-LITE-DB3.IPV6.BIN
- 放置到 Server 工作目录的 `configs/` 下，文件名保持一致；或用环境变量显式指定：
  - `IP2LOCATION_BIN_PATH=/abs/path/IP2LOCATION-LITE-DB3.BIN`
  - `IP2LOCATION_BIN_PATH_V6=/abs/path/IP2LOCATION-LITE-DB3.IPV6.BIN`
- Server 运行时会自动探测并启用；不存在时自动跳过。

### 2. 在线 HTTP 解析

- 配置环境变量：
  - `GEOIP_HTTP_URL`：例如 `https://your-geo.example.com/lookup?ip={{ip}}`
  - `GEOIP_TIMEOUT_MS`：HTTP 调用超时，默认 1500
- 响应 JSON 可包含 `country/country_name`、`region/region_name/province/state`、`city` 中的一种或多种字段。

### 内网地址处理

内网/本地地址不会进行查询：
- `127.0.0.1/::1` → "本地"
- `10/172.16–31/192.168/169.254`、`fc00::/7`、`fe80::/10` → "局域网"

### 验证

- 登录后台后查看"登录日志"的"属地"列，或请求 `/api/audit?kinds=login` 查看 `meta.ip_region`。

---

## 健康检查

### Server 健康检查

```bash
# HTTP 健康检查
curl http://localhost:8080/healthz

# 预期输出
{"status":"ok"}
```

### Agent 健康检查

```bash
# 检查 Agent 本地监听端口
curl http://localhost:19090/healthz
```

---

## 防火墙配置

### Server 端口

| 端口 | 协议 | 说明 |
|------|------|------|
| 8443 | gRPC/TLS | Agent 连接端口（需要 mTLS） |
| 8080 | HTTP | REST API 端口 |

### Agent 端口

| 端口 | 协议 | 说明 |
|------|------|------|
| 19090 | gRPC | 本地游戏服务器连接端口 |

### firewalld (CentOS/RHEL)

```bash
# Server 开放端口
sudo firewall-cmd --permanent --add-port=8443/tcp
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

### ufw (Ubuntu/Debian)

```bash
# Server 开放端口
sudo ufw allow 8443/tcp
sudo ufw allow 8080/tcp
sudo ufw reload
```

---

## 故障排查

### 服务无法启动

1. 检查配置文件语法
2. 检查端口占用
3. 检查 TLS 证书有效性
4. 查看日志获取详细错误信息

详见 [故障排查指南](./guide/operations/troubleshooting.md)。

---

## 相关文档

- [安装指南](./guide/installation.md) - 开发环境安装
- [配置指南](./configuration.md) - 配置选项说明
- [故障排查](./guide/operations/troubleshooting.md) - 问题诊断
