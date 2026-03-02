# 🔥 Croupier SDK 热更新最佳实践指南

## 📋 概述

本指南提供了在不同开发环境和生产环境中使用Croupier SDK热更新功能的最佳实践和建议。

## 🎯 核心理念

### **分离关注点**
- **SDK负责连接管理**：自动重连、函数注册、状态监控
- **工具负责代码热更新**：Air、Nodemon、PM2等负责代码变更检测
- **开发者负责业务逻辑**：专注游戏功能实现，无需关心底层重载机制

### **渐进式集成**
```
基础连接 → 自动重连 → 文件监听 → 函数热重载 → 生产部署
```

## 🔧 开发环境最佳实践

### **Go开发环境**

#### 推荐配置
```yaml
# croupier-hotreload.yaml
hotreload:
  enabled: true
  auto_reconnect: true
  reconnect_delay: 3s
  max_retry_attempts: 5
  health_check_interval: 10s

  tools:
    air: true
    plugin: false  # 开发环境关闭复杂特性
```

#### 开发工作流
```bash
# 1. 启动Agent
./croupier-agent --config configs/agent.example.yaml

# 2. 启动Air热重载开发
cd examples/go-hotreload
air

# 3. 修改代码 -> Air自动重编译 -> SDK自动重连 -> 继续开发
```

#### Air最佳配置
```toml
# .air.toml
[build]
cmd = "go build -o ./tmp/main ."
delay = 1000              # 1秒防抖，避免频繁触发
exclude_regex = ["_test.go"]  # 排除测试文件

[log]
main_only = true          # 只显示主程序日志，减少噪音

[misc]
clean_on_exit = true      # 自动清理临时文件
```

### **Node.js开发环境**

#### 推荐配置
```json
{
  "hotReload": {
    "enabled": true,
    "autoReconnect": true,
    "reconnectDelay": 3000,
    "tools": {
      "nodemon": true,
      "moduleReload": true,
      "pm2": false
    },
    "fileWatching": {
      "enabled": true,
      "watchDir": "./functions",
      "patterns": ["*.js", "*.json"]
    }
  }
}
```

#### 开发工作流
```bash
# 1. 启动Agent
./croupier-agent --config configs/agent.example.yaml

# 2. 启动Nodemon热重载
cd examples/js-hotreload
npm run dev

# 3. 修改代码 -> Nodemon重启进程 -> SDK自动重连 -> 继续开发
```

#### Nodemon最佳配置
```json
{
  "watch": ["src/", "functions/", "config/"],
  "ext": "js,json,yaml",
  "delay": "2000",          # 2秒防抖
  "env": {
    "NODE_ENV": "development",
    "LOG_LEVEL": "debug"
  },
  "ignore": ["tmp/", "logs/", "*.tmp"]
}
```

## 🚀 生产环境最佳实践

### **Go生产环境**

#### 配置策略
```yaml
# 生产环境配置
hotreload:
  enabled: true
  auto_reconnect: true
  reconnect_delay: 10s        # 生产环境延长间隔
  max_retry_attempts: 20      # 增加重试次数
  health_check_interval: 60s  # 降低检查频率
  graceful_shutdown_timeout: 60s  # 更长的关闭时间

  tools:
    air: false              # 关闭开发工具
    plugin: true            # 启用Go Plugin热更新
```

#### 部署策略
1. **蓝绿部署** - 推荐用于大版本更新
2. **滚动更新** - 用于小版本补丁
3. **插件热更新** - 用于紧急修复

```bash
# 插件热更新部署
go build -buildmode=plugin -o player_ban.so ./plugins/player_ban
# 通过管理界面上传插件
# SDK自动加载新插件
```

### **Node.js生产环境**

#### PM2集群配置
```json
{
  "apps": [{
    "name": "croupier-game",
    "script": "main.js",
    "instances": "max",           # 使用所有CPU核心
    "exec_mode": "cluster",       # 集群模式
    "watch": false,               # 生产环境关闭文件监听
    "autorestart": true,
    "max_restarts": 10,
    "min_uptime": "10s",
    "env": {
      "NODE_ENV": "production",
      "CROUPIER_AUTO_RECONNECT": "true",
      "CROUPIER_RECONNECT_DELAY": "15000"
    }
  }]
}
```

#### 零停机部署
```bash
# 1. 验证新代码
npm test

# 2. 零停机重载
pm2 reload croupier-game

# 3. 验证服务状态
pm2 status
curl http://localhost:18780/health
```

## 📊 监控和观测最佳实践

### **关键指标监控**

```go
// Go SDK监控指标
type HotReloadMetrics struct {
    // 连接健康
    ConnectionStatus     string    `json:"connection_status"`
    LastReconnectTime   time.Time `json:"last_reconnect_time"`
    ReconnectCount      int64     `json:"reconnect_count"`

    // 重载性能
    FunctionReloads     int64     `json:"function_reloads"`
    AvgReloadTime       time.Duration `json:"avg_reload_time"`

    // 错误追踪
    FailedReloads       int64     `json:"failed_reloads"`
    LastError           string    `json:"last_error"`
}
```

### **告警规则**
```yaml
# Prometheus告警规则
- alert: CroupierReconnectHigh
  expr: increase(croupier_reconnect_count[5m]) > 3
  for: 2m
  annotations:
    summary: "Croupier客户端频繁重连"

- alert: CroupierReloadFailed
  expr: increase(croupier_reload_failed_count[5m]) > 1
  for: 1m
  annotations:
    summary: "Croupier热重载失败"
```

### **日志最佳实践**

```json
{
  "level": "info",
  "timestamp": "2024-01-15T10:30:00Z",
  "component": "hotreload",
  "event": "function_reloaded",
  "function_id": "player.ban",
  "old_version": "1.0.0",
  "new_version": "1.1.0",
  "reload_duration_ms": 150,
  "trace_id": "abc123"
}
```

## 🔐 安全最佳实践

### **访问控制**
```yaml
# RBAC配置示例
permissions:
  hotreload.function.reload:
    description: "允许重载函数"
    scopes: ["dev", "staging"]  # 仅开发和测试环境

  hotreload.config.reload:
    description: "允许重载配置"
    requires_approval: true     # 需要审批
    scopes: ["production"]      # 生产环境
```

### **代码验证**
```go
// 函数重载前的安全检查
func validateFunctionReload(functionID string, newCode []byte) error {
    // 1. 语法检查
    if err := validateSyntax(newCode); err != nil {
        return fmt.Errorf("syntax error: %w", err)
    }

    // 2. 安全扫描
    if err := scanForVulnerabilities(newCode); err != nil {
        return fmt.Errorf("security issue: %w", err)
    }

    // 3. 签名验证
    if err := verifyCodeSignature(newCode); err != nil {
        return fmt.Errorf("invalid signature: %w", err)
    }

    return nil
}
```

### **审计日志**
```json
{
  "event": "hotreload_request",
  "actor": "admin@company.com",
  "function_id": "player.ban",
  "change_type": "function_update",
  "approval_status": "approved",
  "approver": "manager@company.com",
  "risk_level": "medium",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## 🎯 性能优化最佳实践

### **重连优化**

```go
// 智能重连策略
type ReconnectStrategy struct {
    baseDelay    time.Duration
    maxDelay     time.Duration
    multiplier   float64
    jitter       bool
}

func (s *ReconnectStrategy) nextDelay(attempt int) time.Duration {
    delay := float64(s.baseDelay) * math.Pow(s.multiplier, float64(attempt))

    if delay > float64(s.maxDelay) {
        delay = float64(s.maxDelay)
    }

    if s.jitter {
        // 添加±25%的随机偏移，避免惊群效应
        jitterRange := delay * 0.25
        delay += (rand.Float64()*2 - 1) * jitterRange
    }

    return time.Duration(delay)
}
```

### **批量操作优化**

```javascript
// Node.js批量重载优化
class BatchReloader {
  constructor(client) {
    this.client = client;
    this.pendingReloads = new Map();
    this.batchTimeout = 5000; // 5秒批量窗口
  }

  async reloadFunction(functionId, descriptor, handler) {
    // 收集批量操作
    this.pendingReloads.set(functionId, { descriptor, handler });

    // 延迟执行，允许更多操作加入批次
    clearTimeout(this.batchTimer);
    this.batchTimer = setTimeout(() => {
      this.executeBatchReload();
    }, this.batchTimeout);
  }

  async executeBatchReload() {
    if (this.pendingReloads.size === 0) return;

    const functions = Object.fromEntries(this.pendingReloads);
    this.pendingReloads.clear();

    await this.client.reloadFunctions(functions);
  }
}
```

## 🧪 测试最佳实践

### **单元测试**

```go
func TestHotReload(t *testing.T) {
    // 创建测试客户端
    client := NewTestClient()

    // 注册初始函数
    desc1 := FunctionDescriptor{ID: "test.func", Version: "1.0.0"}
    client.RegisterFunction(desc1, handler1)

    // 测试函数重载
    desc2 := FunctionDescriptor{ID: "test.func", Version: "1.1.0"}
    err := client.ReloadFunction("test.func", desc2, handler2)

    assert.NoError(t, err)
    assert.Equal(t, "1.1.0", client.GetFunctionVersion("test.func"))
}
```

### **集成测试**

```javascript
// 端到端热重载测试
describe('Hot Reload Integration', () => {
  let agent, client;

  beforeEach(async () => {
    agent = await startTestAgent();
    client = new HotReloadableClient(testConfig);
    await client.connect();
  });

  it('should handle function reload with agent restart', async () => {
    // 注册初始函数
    await client.registerFunction(descriptor1, handler1);

    // 模拟Agent重启
    await agent.stop();
    await agent.start();

    // 验证自动重连和函数重新注册
    await waitFor(() => client.isConnected);
    expect(client.functions.size).toBe(1);
  });
});
```

## 🚨 故障处理最佳实践

### **常见问题诊断**

```bash
#!/bin/bash
# 热重载健康检查脚本

echo "🔍 Croupier热重载健康检查"

# 1. 检查Agent连接
if curl -f http://localhost:19091/health; then
    echo "✅ Agent健康"
else
    echo "❌ Agent连接失败"
    exit 1
fi

# 2. 检查函数注册
FUNC_COUNT=$(curl -s http://localhost:19091/functions | jq length)
echo "📋 注册函数数量: $FUNC_COUNT"

# 3. 检查重载状态
RELOAD_STATUS=$(curl -s http://localhost:19091/hotreload/status)
echo "🔄 重载状态: $RELOAD_STATUS"

# 4. 检查错误日志
ERROR_COUNT=$(grep -c "ERROR.*hotreload" /var/log/croupier.log || echo 0)
if [ $ERROR_COUNT -gt 0 ]; then
    echo "⚠️ 发现 $ERROR_COUNT 个热重载错误"
    grep "ERROR.*hotreload" /var/log/croupier.log | tail -5
fi
```

### **回滚策略**

```go
// 自动回滚机制
type RollbackManager struct {
    versions map[string][]FunctionVersion
    maxHistory int
}

func (rm *RollbackManager) rollback(functionID string) error {
    versions := rm.versions[functionID]
    if len(versions) < 2 {
        return fmt.Errorf("no previous version to rollback to")
    }

    // 回滚到上一个版本
    previousVersion := versions[len(versions)-2]

    return rm.reloadFunction(functionID, previousVersion.Descriptor, previousVersion.Handler)
}
```

## 📚 工具和资源

### **推荐工具**

| 工具 | 用途 | 支持语言 | 生产可用 |
|------|------|----------|----------|
| **Air** | Go开发热重载 | Go | 否 |
| **Nodemon** | Node.js开发热重载 | JavaScript | 否 |
| **PM2** | Node.js生产部署 | JavaScript | 是 |
| **JRebel** | Java热重载 | Java | 是 |
| **Spring DevTools** | Spring开发热重载 | Java | 否 |

### **配置模板**

```bash
# 快速生成配置
./scripts/generate-hotreload-config.sh --language go --env development
./scripts/generate-hotreload-config.sh --language nodejs --env production
```

### **监控仪表板**

- Grafana仪表板模板：`monitoring/grafana/hotreload-dashboard.json`
- Prometheus规则：`monitoring/prometheus/hotreload-rules.yaml`
- 告警配置：`monitoring/alertmanager/hotreload-alerts.yaml`

## 🎯 总结和建议

### **关键原则**

1. **渐进式采用**：从基础自动重连开始，逐步引入高级特性
2. **环境分离**：开发环境激进，生产环境保守
3. **监控优先**：先建立监控，再启用热重载
4. **安全第一**：所有热重载操作必须经过验证和审计

### **实施路径**

```
第1周：基础自动重连 + 开发环境热重载
第2周：生产环境重连 + 监控告警
第3周：函数热重载 + 安全审计
第4周：高级特性 + 性能优化
```

### **成功指标**

- 🎯 **开发效率**：代码变更到测试时间 < 10秒
- 🎯 **服务可用性**：热重载导致的停机时间 < 1%
- 🎯 **错误率**：热重载操作成功率 > 99%
- 🎯 **恢复时间**：故障自动恢复时间 < 30秒

---

*🔥 通过遵循这些最佳实践，您可以安全、高效地在游戏开发中使用Croupier热更新功能！*