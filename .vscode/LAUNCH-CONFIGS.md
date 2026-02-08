# 🎯 VS Code Agent 启动配置完整指南

## 📋 快速参考

| 配置名称 | 用途 | 配置文件 | 端口 |
|---------|------|---------|------|
| **Agent (多文件示例)** ⭐ | 加载示例 + Packs | `providers.yaml` | 8080 |
| **Agent (加载所有 Packs)** | 仅加载 Packs | `providers.yaml` | 8080 |
| **Agent (调试模式)** | Debug 模式 | `providers.yaml` | - |
| **Agent (微服务架构)** 🎯 | 多服务配置 | `providers.multi-service.example.yaml` | 多个 |

---

## 🚀 使用方法

### 步骤 1: 打开调试面板

按 **F5** 或点击侧边栏的调试图标

### 步骤 2: 选择配置

从顶部下拉菜单选择配置

### 步骤 3: 启动调试

点击绿色播放按钮或按 F5

---

## 📖 详细配置说明

### 1️⃣ Agent (多文件示例) ⭐

**最佳选择**：适合大多数开发场景

**加载内容**：
```yaml
platforms:
  examples:
    openapi_spec: ./etc/openapi.example.yaml  # 13 个示例函数
    
  packs:
    openapi_specs:
      - ../packs/http/openapi.yaml           # HTTP 调用
      - ../packs/prom/openapi.yaml           # Prometheus 查询
      - ../packs/player/openapi.yaml         # 玩家管理
      - ../packs/grafana/openapi.yaml        # Grafana 集成
      - ../packs/alertmanager/openapi.yaml   # 告警管理
```

**注册函数数**：19 个（13 示例 + 6 Packs）

**使用场景**：
- ✅ 快速验证 OpenAPI 功能
- ✅ 测试 Pack 集成
- ✅ 开发新功能

**预期日志**：
```
INFO platform loaded name=examples methods=13
INFO platform loaded name=packs methods=6
INFO registering platform method function=examples.player.create
...
```

---

### 2️⃣ Agent (加载所有 Packs)

**适用场景**：专注于 Pack 功能测试

**加载内容**：
```yaml
platforms:
  packs:
    openapi_specs:
      - ../packs/http/openapi.yaml
      - ../packs/prom/openapi.yaml
      - ../packs/player/openapi.yaml
      - ../packs/grafana/openapi.yaml
      - ../packs/alertmanager/openapi.yaml
```

**环境变量**：
```json
{
  "GAME_ID": "dev-game",
  "ENV": "dev"
}
```

**注册函数数**：6 个 Pack 函数

**使用场景**：
- ✅ Pack 功能验证
- ✅ 集成测试
- ✅ 性能测试

---

### 3️⃣ Agent (调试模式)

**适用场景**：需要断点调试

**配置特点**：
```json
{
  "mode": "debug",
  "args": ["-f", "etc/agent.yaml", "-log.level", "debug"],
  "showLog": true
}
```

**功能**：
- ✅ 可在代码中设置断点
- ✅ 详细的 DEBUG 日志
- ✅ 单步执行代码

**推荐断点位置**：
- `services/agent/platform.go:80` - 平台初始化
- `internal/platform/openapi/provider.go:345` - OpenAPI 解析
- `internal/app/agent/upstream.go:417` - 函数同步

---

### 4️⃣ Agent (微服务架构) 🎯

**适用场景**：模拟生产环境微服务

**配置文件**：`providers.multi-service.example.yaml`

**服务架构**：
```
┌─────────────────────────────────────┐
│          Agent (统一入口)            │
└─────────────────────────────────────┘
              │
    ┌─────────┼─────────┬──────────┐
    │         │         │          │
┌───▼───┐ ┌──▼───┐ ┌──▼───┐ ┌────▼────┐
│ 8081  │ │ 8082 │ │ 8083 │ │  9090   │
│Player │ │Inv.  │ │Chat  │ │Prom     │
│Service│ │Service│ │Service│ │         │
└───────┘ └──────┘ └──────┘ └─────────┘
```

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

**注册函数**：
```
player_service.player.create       → localhost:8081
inventory_service.http.invoke     → localhost:8082
chat_service.player.ban           → localhost:8083
monitoring_service.prom.query     → localhost:9090
```

**使用场景**：
- ✅ 微服务架构测试
- ✅ 服务网格验证
- ✅ 多服务调试

---

## 🔍 验证配置

### 方法 1: 查看日志

启动后在 **DEBUG CONSOLE** 查看：

```log
INFO  loading provider config from providers.yaml
INFO  provider loaded name=examples methods=13
INFO  registering platform method function=examples.player.create
INFO  registering platform method function=examples.player.get
...
```

### 方法 2: 查询函数

```bash
# 查询所有已注册函数
curl http://localhost:18888/api/v1/functions/list | jq .

# 查询特定平台的函数
curl http://localhost:18888/api/v1/functions/list | \
  jq '.[] | select(.function_id | startswith("examples"))'
```

### 方法 3: 测试调用

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

## 🛠️ 配置文件位置

```
croupier/
├── .vscode/
│   ├── launch.json                    # ✨ VS Code 启动配置
│   ├── AGENT-DEBUG-GUIDE.md           # 📖 详细调试指南
│   └── README.md                      # 📋 VS Code 配置说明
│
└── services/agent/etc/
    ├── agent.yaml                     # Agent 主配置
    ├── providers.yaml                 # ⭐ 默认 Provider 配置
    ├── providers.multi-service.example.yaml   # 🎯 微服务配置
    ├── openapi.example.yaml           # 📘 OpenAPI 示例
    ├── QUICKSTART.md                  # 🚀 快速开始
    ├── README-OPENAPI.md              # 📖 完整指南
    └── MULTI-SERVICE-GUIDE.md         # 🔄 多服务指南
```

---

## 💡 使用技巧

### 技巧 1: 快速切换配置

不需要修改代码，只需：
1. 停止当前调试
2. 从下拉菜单选择新配置
3. 重新启动

### 技巧 2: 自定义环境变量

在 `launch.json` 中添加：

```json
{
  "name": "Agent (自定义)",
  "type": "go",
  "env": {
    "MY_VAR": "my-value",
    "GAME_ID": "${env:GAME_ID}"  // 从系统环境变量读取
  }
}
```

### 技巧 3: 复合启动

先启动 Server，再启动 Agent：

1. **终端 1**：启动 Server
   ```bash
   cd services/server
   go run main.go -f etc/server.yaml
   ```

2. **VS Code**：启动 Agent
   - 按 F5 → 选择 "Agent (多文件示例)"

### 技巧 4: 日志过滤

在 DEBUG CONSOLE 顶部搜索框输入：

```
platform
```

只显示包含 "platform" 的日志行。

---

## 🐛 常见问题

### Q1: Agent 启动失败，提示连接 Server

**原因**：Server 未启动

**解决**：
1. 先启动 Server（F5 → "Server (dev sqlite)"）
2. 再启动 Agent

### Q2: 函数未注册

**原因**：
- `providers.yaml` 不存在
- OpenAPI 文件路径错误
- `enabled: false`

**解决**：
```bash
# 检查配置文件
ls services/agent/etc/providers.yaml

# 检查 OpenAPI 文件
ls services/agent/etc/openapi.example.yaml

# 验证 YAML 语法
python3 -c "import yaml; yaml.safe_load(open('services/agent/etc/providers.yaml'))"
```

### Q3: 微服务配置连接失败

**原因**：对应端口的服务未启动

**解决**：
```bash
# 检查端口
lsof -i :8081
lsof -i :8082
lsof -i :8083

# 或使用 curl 测试
curl http://localhost:8081/health
```

---

## 📚 相关文档

| 需求 | 查看文档 |
|------|---------|
| **30 秒上手** | [QUICKSTART.md](../services/agent/etc/QUICKSTART.md) |
| **完整指南** | [README-OPENAPI.md](../services/agent/etc/README-OPENAPI.md) |
| **多服务配置** | [MULTI-SERVICE-GUIDE.md](../services/agent/etc/MULTI-SERVICE-GUIDE.md) |
| **VS Code 调试** | [AGENT-DEBUG-GUIDE.md](./AGENT-DEBUG-GUIDE.md) |
| **OpenAPI 示例** | [openapi.example.yaml](../services/agent/etc/openapi.example.yaml) |

---

## 🎓 最佳实践

### 1. 开发流程

```
1. F5 → "Agent (多文件示例)"
   ↓
2. 编写/修改 OpenAPI 文件
   ↓
3. 停止 Agent
   ↓
4. F5 → 重新启动
   ↓
5. 验证函数注册
```

### 2. 调试流程

```
1. 在代码中设置断点
   ↓
2. F5 → "Agent (调试模式)"
   ↓
3. 触发函数调用
   ↓
4. 在断点处检查变量
   ↓
5. 单步执行调试
```

### 3. 微服务测试

```
1. 启动各个微服务
   ↓
2. F5 → "Agent (微服务架构)"
   ↓
3. 查看日志确认连接
   ↓
4. 测试函数路由
   ↓
5. 验证故障隔离
```

---

**最后更新**: 2024-02-07
**版本**: v1.0
