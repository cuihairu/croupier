# ⚡ 5分钟快速集成指南

## 🎯 三种集成方式对比

| 集成方式 | 集成时间 | 复杂度 | 适用场景 | 功能完整度 |
|---------|----------|--------|----------|------------|
| **SimpleAnalytics** | 5分钟 | 低 | 快速验证、小型游戏 | 基础分析 |
| **OTel标准集成** | 30分钟 | 中 | 中大型游戏 | 完整可观测性 |
| **企业级部署** | 1-2天 | 高 | 大型游戏、生产环境 | 企业级特性 |

## 🚀 方案一：极简集成（推荐新手）

### 前置条件
```bash
# 确保已安装
go version    # Go 1.21+
redis-server --version
```

### 步骤 1：启动基础服务
```bash
# 1. 启动Redis
redis-server

# 2. 启动Croupier Server
cd /path/to/croupier
export ANALYTICS_MQ_TYPE=redis
export REDIS_URL=redis://localhost:6379/0
./croupier server --config configs/server.example.yaml

# 3. 启动Analytics Worker
./analytics-worker
```

### 步骤 2：游戏服务器集成
```go
// main.go
package main

import (
    "github.com/cuihairu/croupier/examples/otel-integration/internal/telemetry"
    "time"
)

func main() {
    // 1. 初始化（5行代码搞定）
    telemetry.Init(telemetry.SimpleConfig{
        GameID:    "my-awesome-game",
        ServerURL: "http://localhost:8080",
    })
    defer telemetry.Shutdown()

    // 2. 发送事件（随时调用）
    userID := "player_123"
    sessionID := "session_" + time.Now().Format("20060102150405")

    // 用户登录
    telemetry.Login(userID, "ios", "cn-north")

    // 关卡开始
    telemetry.StartLevel(userID, sessionID, "level-1", "tutorial")

    // 关卡完成
    time.Sleep(2 * time.Second) // 模拟游戏时间
    telemetry.CompleteLevel(userID, sessionID, "level-1", 120, 1, 1500)

    // 内购
    telemetry.Buy(userID, "order_123", "coin_pack_small", 0.99, "USD", true)

    println("游戏事件已发送，查看 http://localhost:8080/api/analytics/realtime")
}
```

### 步骤 3：验证数据
```bash
# 查看实时数据
curl "http://localhost:8080/api/analytics/realtime"

# 查看事件列表
curl "http://localhost:8080/api/analytics/behavior/events"
```

**🎉 完成！你的游戏已接入分析系统！**

## 🔧 方案二：OTel完整集成

### 一键启动
```bash
cd examples/otel-integration
make start      # 启动完整环境
make demo      # 运行演示
```

### 游戏服务器集成
```go
// 使用标准OTel SDK
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

func main() {
    // 初始化OTel
    exporter, _ := otlptracehttp.New(context.Background(),
        otlptracehttp.WithEndpoint("http://localhost:4318"),
    )

    // 使用Croupier的游戏语义约定
    tracer := otel.Tracer("game-server")

    // 追踪用户会话
    ctx, span := tracer.Start(context.Background(), "user.session",
        trace.WithAttributes(
            attribute.String("game.user_id", "player_123"),
            attribute.String("game.platform", "ios"),
        ),
    )
    defer span.End()

    // 业务逻辑...
}
```

### 访问监控界面
- **Grafana**: http://localhost:3000 (admin/admin)
- **Jaeger**: http://localhost:16686
- **Prometheus**: http://localhost:9090

## 🏢 方案三：企业级部署

### Kubernetes部署
```bash
# 使用Helm Chart
helm install croupier-analytics ./charts/croupier-analytics \
  --set redis.cluster.enabled=true \
  --set clickhouse.cluster.enabled=true \
  --set analytics.workers.replicas=5
```

### Docker Compose部署
```bash
# 生产级配置
docker-compose -f docker-compose.prod.yml up -d
```

## 📚 更多集成选项

### Unity游戏集成
```csharp
// Unity C# SDK（即将推出）
CroupierAnalytics.Init("my-game", "http://server:8080");
CroupierAnalytics.TrackEvent("level_start", new {
    level = "1-1",
    episode = "tutorial"
});
```

### Unreal Engine集成
```cpp
// Unreal C++ SDK（即将推出）
FCroupierAnalytics::Init(TEXT("my-game"), TEXT("http://server:8080"));
FCroupierAnalytics::TrackEvent(TEXT("level_start"), LevelData);
```

### JavaScript/HTML5集成
```javascript
// Web/H5游戏集成
import { CroupierAnalytics } from 'croupier-analytics-js';

CroupierAnalytics.init({
  gameId: 'my-h5-game',
  serverUrl: 'http://server:8080'
});

CroupierAnalytics.track('level_start', {
  level: '1-1',
  episode: 'tutorial'
});
```

## 🚨 常见问题

### Q: 事件没有显示在面板中？
A: 检查以下几点：
1. Redis是否正常运行：`redis-cli ping`
2. Worker是否处理消息：`redis-cli XLEN analytics:events`
3. 服务器是否健康：`curl http://localhost:8080/health`

### Q: 如何自定义事件？
A: 使用通用Track方法：
```go
telemetry.Track("custom_event", "user123", map[string]interface{}{
    "action": "boss_defeated",
    "boss_name": "fire_dragon",
    "damage_dealt": 9999,
})
```

### Q: 如何查看更多分析维度？
A: 访问完整的Analytics API：
- 实时数据：`GET /api/analytics/realtime`
- 概览数据：`GET /api/analytics/overview`
- 用户行为：`GET /api/analytics/behavior/funnel`
- 留存分析：`GET /api/analytics/retention`

## 🎯 下一步

1. **扩展监控**：添加更多游戏特定指标
2. **性能优化**：根据数据量调整批处理大小
3. **告警设置**：配置关键指标的阈值告警
4. **多环境部署**：设置开发/测试/生产环境

---

*🎮 开始你的游戏分析之旅吧！*