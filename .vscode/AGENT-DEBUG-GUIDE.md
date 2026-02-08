# VS Code 调试配置 - Agent OpenAPI 注册

## 📋 可用的启动配置

在 VS Code 中按 **F5** 或点击调试面板，可以选择以下配置：

### 1. Agent (多文件示例) ⭐

**加载内容**：
- ✅ `openapi.example.yaml` (13 个示例函数)
- ✅ 所有 Packs (6 个函数)

**适用场景**：开发测试，快速验证

**配置文件**：`services/agent/etc/providers.yaml`

```json
{
  "name": "Agent (多文件示例)",
  "type": "go",
  "request": "launch",
  "mode": "auto",
  "program": "${workspaceFolder}/services/agent",
  "args": ["-f", "etc/agent.yaml"],
  "env": {
    "PROVIDER_CONFIG": "etc/providers.yaml"
  },
  "cwd": "${workspaceFolder}/services/agent",
  "console": "integratedTerminal"
}
```

**注册的函数**：
```
examples.player.create
examples.player.get
examples.player.delete
examples.inventory.get
...
packs.http.generic_invoke
packs.prom.query
packs.player.ban
...
```

---

### 2. Agent (加载所有 Packs)

**加载内容**：
- ✅ 所有 Packs 的 OpenAPI 文件

**适用场景**：Pack 功能测试

**环境变量**：
```json
{
  "GAME_ID": "dev-game",
  "ENV": "dev"
}
```

**注册的函数**：
```
packs.http.generic_invoke
packs.prom.query
packs.prom.query_range
packs.player.ban
packs.grafana.search_dashboards
packs.alertmanager.list_alerts
```

---

### 3. Agent (调试模式)

**特点**：
- ✅ Debug 模式（可设置断点）
- ✅ 详细的日志输出
- ✅ 开发环境标识

**配置**：
```json
{
  "name": "Agent (调试模式)",
  "mode": "debug",
  "args": ["-f", "etc/agent.yaml", "-log.level", "debug"],
  "env": {
    "CROUPIER_DEV": "1"
  },
  "showLog": true
}
```

---

### 4. Agent (微服务架构) 🎯

**特点**：
- ✅ 配置多个服务（不同端口）
- ✅ 每个服务独立认证
- ✅ 模拟微服务架构

**配置文件**：`services/agent/etc/providers.multi-service.example.yaml`

**服务端口**：
- Player Service: `8081`
- Inventory Service: `8082`
- Chat Service: `8083`
- Prometheus: `9090`
- Alertmanager: `9093`

**环境变量**：
```json
{
  "PLAYER_SERVICE_TOKEN": "dev-token-player",
  "INVENTORY_SERVICE_TOKEN": "dev-token-inventory",
  "CHAT_SERVICE_TOKEN": "dev-token-chat",
  "MATCH_SERVICE_TOKEN": "dev-token-match",
  "PROM_USER": "prometheus",
  "PROM_PASSWORD": "prometheus"
}
```

**注册的函数**：
```
player_service.player.create      → localhost:8081
player_service.player.get         → localhost:8081
inventory_service.http.generic_invoke  → localhost:8082
chat_service.player.ban           → localhost:8083
monitoring_service.prom.query    → localhost:9090
alerting_service.alertmanager.list_alerts  → localhost:9093
```

---

## 🚀 快速开始

### 方式 1: 使用 VS Code 调试（推荐）

1. **打开调试面板**
   - 按 `Cmd+Shift+D` (Mac)
   - 或 `Ctrl+Shift+D` (Windows/Linux)
   - 或点击侧边栏的调试图标

2. **选择配置**
   - 从顶部下拉菜单选择 "Agent (多文件示例)"

3. **启动调试**
   - 按 `F5` 或点击绿色播放按钮

4. **查看输出**
   - 在 "DEBUG CONSOLE" 中查看日志
   - 或在集成终端中查看

### 方式 2: 使用命令行

```bash
# 进入 agent 目录
cd services/agent

# 启动 agent（使用默认配置）
../../bin/croupier-agent -f etc/agent.yaml

# 或使用微服务配置
PROVIDER_CONFIG=etc/providers.multi-service.example.yaml \
  PLAYER_SERVICE_TOKEN=dev-token-player \
  INVENTORY_SERVICE_TOKEN=dev-token-inventory \
  CHAT_SERVICE_TOKEN=dev-token-chat \
  ../../bin/croupier-agent -f etc/agent.yaml
```

---

## 🔍 验证函数注册

### 1. 查看 Agent 日志

启动后应该看到类似的日志：

```
INFO  loading provider config from providers.yaml
INFO  provider loaded name=examples methods=13
INFO  registering provider method function=examples.player.create
INFO  registering provider method function=examples.player.get
INFO  registering provider method function=examples.player.delete
...
INFO  provider loaded name=packs methods=6
INFO  registering provider method function=packs.http.generic_invoke
...
```

### 2. 查询已注册函数

```bash
# 查询所有函数
curl http://localhost:18888/api/v1/functions/list

# 查询特定平台的函数
curl http://localhost:18888/api/v1/functions/list | \
  jq '.[] | select(.function_id | startswith("examples"))'
```

### 3. 测试函数调用

```bash
# 调用示例函数
curl -X POST http://localhost:18888/api/v1/functions/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "examples.player.create",
    "payload": {
      "username": "testplayer",
      "email": "test@example.com",
      "region": "us-east"
    }
  }'
```

---

## 🛠️ 调试技巧

### 1. 设置断点

在代码中设置断点：
- `services/agent/platform.go` - 平台加载逻辑
- `internal/platform/openapi/provider.go` - OpenAPI 解析逻辑

启动时选择 "Agent (调试模式)" 配置。

### 2. 查看日志

```json
{
  "args": ["-f", "etc/agent.yaml", "-log.level", "debug"],
  "showLog": true
}
```

日志会显示在 DEBUG CONSOLE 中。

### 3. 环境变量调试

在 launch.json 中添加环境变量：

```json
{
  "env": {
    "CROUPIER_DEV": "1",
    "LOG_LEVEL": "debug",
    "GAME_ID": "dev-game"
  }
}
```

### 4. 热重载

修改配置文件后：
1. 在调试控制台点击停止按钮
2. 重新按 F5 启动

---

## 📊 配置文件说明

### providers.yaml

**路径**：`services/agent/etc/providers.yaml`

**用途**：默认配置，加载示例和 packs

```yaml
platforms:
  examples:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./etc/openapi.example.yaml
      base_url: http://localhost:8080

  packs:
    enabled: true
    type: openapi
    config:
      openapi_specs:
        - ../packs/http/openapi.yaml
        - ../packs/prom/openapi.yaml
        - ../packs/player/openapi.yaml
      base_url: http://localhost:8080
```

### providers.multi-service.example.yaml

**路径**：`services/agent/etc/providers.multi-service.example.yaml`

**用途**：微服务架构，多个服务不同端口

```yaml
platforms:
  player_service:
    enabled: true
    config:
      openapi_specs: [./etc/openapi.example.yaml]
      base_url: http://localhost:8081  # 不同端口

  inventory_service:
    enabled: true
    config:
      openapi_specs: [../packs/http/openapi.yaml]
      base_url: http://localhost:8082  # 不同端口

  # ... 更多服务
```

---

## 🎯 常见场景

### 场景 1: 开发新 OpenAPI 函数

1. 创建 OpenAPI 文件：`services/agent/etc/my-functions.yaml`
2. 添加到 `providers.yaml`：
   ```yaml
   my_functions:
     enabled: true
     type: openapi
     config:
       openapi_spec: ./etc/my-functions.yaml
       base_url: http://localhost:8080
   ```
3. 按 F5 启动 "Agent (多文件示例)"
4. 查看日志确认加载

### 场景 2: 测试微服务架构

1. 确保各个服务在不同端口运行
2. 按 F5 启动 "Agent (微服务架构)"
3. 查看日志确认所有平台加载
4. 测试函数调用验证路由

### 场景 3: 调试 OpenAPI 解析

1. 在 `internal/platform/openapi/provider.go` 设置断点
2. 按 F5 启动 "Agent (调试模式)"
3. 在断点处检查 OpenAPI 解析过程

---

## 🐛 故障排查

### 问题 1: Agent 启动失败

**检查**：
- Server 是否运行：`ps aux | grep croupier-server`
- 端口是否占用：`lsof -i :18888`
- 配置文件是否存在：`ls services/agent/etc/agent.yaml`

### 问题 2: 函数未注册

**检查**：
- providers.yaml 语法：`python3 -c "import yaml; yaml.safe_load(open('services/agent/etc/providers.yaml'))"`
- OpenAPI 文件路径：`ls services/agent/etc/openapi.example.yaml`
- Agent 日志：查看是否有 "provider loaded" 日志

### 问题 3: 函数调用失败

**检查**：
- 函数 ID 是否正确：检查函数 ID 前缀
- 后端服务是否运行：`lsof -i :8080`
- base_url 配置是否正确

---

## 📚 相关文档

- [快速参考](../services/agent/etc/QUICKSTART.md)
- [完整指南](../services/agent/etc/README-OPENAPI.md)
- [多服务配置](../services/agent/etc/MULTI-SERVICE-GUIDE.md)
- [OpenAPI 示例](../services/agent/etc/openapi.example.yaml)

---

## 💡 最佳实践

### 1. 使用不同的配置文件

- 开发：`providers.yaml`
- 测试：`providers.multi-service.example.yaml`
- 生产：单独的配置文件

### 2. 环境变量管理

在 VS Code 设置中配置：
```json
{
  "terminal.integrated.env.linux": {
    "PLAYER_SERVICE_TOKEN": "${env:PLAYER_SERVICE_TOKEN}"
  }
}
```

### 3. 复合启动

先启动 Server，再启动 Agent：

1. 选择 "Server (dev sqlite)" 启动
2. 选择 "Agent (多文件示例)" 启动

---

**最后更新**: 2024-02-07
**VS Code 版本**: 1.80+
**Go 扩展版本**: 0.40+
