---
title: 常见问题
icon: circle-question
order: 5
category:
  - 入门指南
tag:
  - FAQ
  - 常见问题
---

# 常见问题 (FAQ)

本文档收集了新手最常遇到的问题和解答。

## 目录

[[toc]]

## 基础概念

### Croupier 是什么？

Croupier 是一个**分布式游戏管理系统**，专为游戏运营设计。它提供统一的控制面，让你通过 Web 界面管理游戏服务器，而不是直接登录游戏服务器。

**核心价值：**
- 统一管理多台游戏服务器
- 安全的权限控制和审批流程
- 可视化的操作界面
- 完整的审计日志

### 我需要 Croupier 吗？

如果你的项目符合以下情况，Croupier 适合你：

- ✅ 有多台游戏服务器需要管理
- ✅ 需要精细的权限控制（GM、运营、客服等不同角色）
- ✅ 需要记录操作日志，用于审计
- ✅ 团队多人协作，需要审批流程
- ✅ 需要通过 Web 界面执行游戏管理操作

### Croupier 和传统 GM 系统有什么区别？

| 特性 | 传统 GM 系统 | Croupier |
|------|-------------|----------|
| 部署方式 | 内嵌游戏服务器 | 独立部署 |
| 访问方式 | 登录服务器命令行 | Web 界面 |
| 权限控制 | 简单的账号密码 | RBAC + ABAC + 双人审批 |
| 审计日志 | 简单文本记录 | 结构化 + 防篡改链 |
| 扩展性 | 受限于游戏服务器 | 独立扩展 |

## 快速开始

### 最快的上手方式是什么？

```bash
# 1. 克隆仓库
git clone --recursive https://github.com/cuihairu/croupier.git
cd croupier

# 2. 一键启动（开发模式）
make dev && ./bin/croupier-server --config services/server/etc/server.yaml

# 3. 访问管理界面
# 浏览器打开 http://localhost:18780
```

### 必须使用子模块吗？

是的。Croupier 使用 Git 子模块管理以下组件：

- `proto/` - gRPC 协议定义
- `sdks/` - 多语言 SDK
- `dashboard/` - Web 管理界面

如果子模块目录为空：

```bash
git submodule update --init --recursive
```

### 开发环境需要什么？

**最低要求：**
- Go 1.25+
- buf（Protocol Buffers 工具）

**完整开发（含 Dashboard）：**
- Node.js 22+
- pnpm 10+

## 配置问题

### 默认端口是什么？

| 组件 | 端口 | 协议 |
|------|------|------|
| Server gRPC | 8443 | gRPC + mTLS |
| Server HTTP | 18780 | REST API |
| Server Metrics | 9090 | Prometheus |
| Agent Local | 19090 | gRPC |
| Agent Metrics | 9091 | Prometheus |

### 端口被占用怎么办？

修改配置文件：

```yaml
# server.yaml
Control:
  Addr: ":19091"  # 修改 Control 端口
Port: 19080       # 修改 HTTP 端口
```

### 必须配置 TLS 证书吗？

**生产环境：** 必须使用 mTLS

**开发环境：** 可以使用项目提供的自签名证书生成脚本：

```bash
./scripts/dev-certs.sh
```

生成的证书位于 `data/dev-certs/` 目录。

### 如何启用 debug 日志？

```yaml
# 配置文件
server:
  log:
    level: debug  # debug | info | warn | error
    format: console  # console | json
```

## 函数开发

### 如何创建第一个函数？

**最简单的方式：**使用示例模板

```go
// 在你的游戏服务器中
package main

import (
    "context"
    "github.com/cuihairu/croupier-sdk-go/sdk"
)

func BanPlayer(ctx context.Context, req BanPlayerRequest) (*BanPlayerResponse, error) {
    // 1. 参数验证
    if req.PlayerId == "" {
        return nil, errors.New("player_id is required")
    }

    // 2. 执行业务逻辑
    err := gameServer.BanPlayer(req.PlayerId, req.Duration)
    if err != nil {
        return nil, err
    }

    // 3. 返回结果
    return &BanPlayerResponse{
        BanId: generateBanId(),
        ExpiresAt: time.Now().Add(req.Duration),
    }, nil
}

func main() {
    // 注册函数
    sdk.Register("player.ban", BanPlayer)
    sdk.Start(":19090")
}
```

### 函数定义在哪里？

函数定义在你的**游戏服务器代码**中，通过 SDK 注册到 Croupier Agent。

**流程：**
```
游戏服务器代码 → SDK 注册 → Agent → Server → Web UI
```

### 如何定义函数参数？

使用 JSON Schema 定义参数结构：

```go
var PlayerBanSchema = map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "player_id": map[string]interface{}{
            "type": "string",
            "title": "玩家ID",
        },
        "duration": map[string]interface{}{
            "type": "integer",
            "title": "封禁时长（小时）",
            "minimum": 1,
            "maximum": 720,
        },
        "reason": map[string]interface{}{
            "type": "string",
            "title": "封禁原因",
        },
    },
    "required": []string{"player_id", "duration"},
}
```

## 权限控制

### 如何创建角色？

```bash
# 通过 API 创建角色
POST /api/roles
{
  "role_id": "gm_level1",
  "name": "初级GM",
  "permissions": [
    "player.view",
    "player.kick",
    "item.view"
  ]
}
```

### 什么是双人审批？

高风险操作（如永久封禁）需要**两人审批**才能执行：

```yaml
# 函数定义中配置
auth:
  two_person_rule: true
  approval:
    enabled: true
    threshold: 2  # 需要两人审批
```

### 如何配置权限？

权限格式为 `<资源>.<操作>`：

- `player.view` - 查看玩家信息
- `player.ban` - 封禁玩家
- `item.*` - 所有道具操作
- `*.*` - 超级管理员

## 运维问题

### 如何优雅关闭？

```bash
# 发送 SIGTERM 信号
kill -TERM $(pgrep croupier-server)

# Server 会：
# 1. 停止接受新请求
# 2. 等待现有请求完成（最多 30 秒）
# 3. 关闭数据库连接
# 4. 退出
```

### 如何备份数据？

**数据库备份：**

```bash
# PostgreSQL
pg_dump -U croupier croupier > backup_$(date +%Y%m%d).sql

# SQLite
cp data/croupier.db backup/croupier_$(date +%Y%m%d).db
```

**配置备份：**

```bash
tar -czf config_backup_$(date +%Y%m%d).tar.gz configs/
```

### 如何升级版本？

```bash
# 1. 备份数据
./scripts/backup.sh

# 2. 停止服务
systemctl stop croupier-server

# 3. 更新代码
git pull origin main
git submodule update --init --recursive

# 4. 重新构建
make build

# 5. 启动服务
systemctl start croupier-server
```

## 故障排查

### Server 启动失败怎么办？

1. **检查配置文件语法**

```bash
./croupier config test --config configs/server.yaml
```

2. **检查端口占用**

```bash
lsof -i :8443
lsof -i :8080
```

3. **查看错误日志**

```bash
tail -f logs/server.log
```

### Agent 连不上 Server？

**检查清单：**

- [ ] Server 正在运行
- [ ] 网络连通（`telnet server 8443`）
- [ ] TLS 证书正确
- [ ] Agent 配置中的 server_addr 正确

### 函数调用超时怎么办？

**可能原因：**

1. 游戏服务器处理慢
2. 网络延迟高
3. Agent 与游戏服务器连接问题

**解决方法：**

```json
{
  "function_id": "data.export",
  "options": {
    "timeout": 300  // 增加超时到 5 分钟
  }
}
```

或使用异步调用：

```bash
POST /api/jobs
```

## 性能优化

### 支持多少并发？

**默认配置：**
- Server: 1000 并发连接
- 每连接: 100 并发请求

**优化方法：**

```yaml
server:
  grpc:
    max_concurrent_streams: 1000
  http:
    max_connections: 5000
```

### 如何减少内存占用？

1. **调整日志级别**：生产环境使用 `warn` 或 `error`
2. **限制日志文件大小**
3. **启用日志压缩**

```yaml
server:
  log:
    level: warn
    max_size: 100  # MB
    max_backups: 3
    compress: true
```

## 集成问题

### 如何与现有系统集成？

**REST API 集成：**

```bash
# 调用函数
curl -X POST http://server:8080/api/invoke \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Game-ID: my-game" \
  -H "X-Env: prod" \
  -d '{"function_id":"player.ban","payload":{...}}'
```

### 如何对接现有权限系统？

通过 **Webhook** 扩展：

```yaml
server:
  auth:
    webhook:
      enabled: true
      url: "https://your-auth-system.com/validate"
```

## 更多帮助

- 📖 [完整文档](/)
- 💬 [GitHub Discussions](https://github.com/cuihairu/croupier/discussions)
- 🐛 [报告问题](https://github.com/cuihairu/croupier/issues)
