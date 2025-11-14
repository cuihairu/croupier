# OpenTelemetry 集成示例

这个示例展示了如何在 Croupier 游戏后端系统中完整集成 OpenTelemetry，实现全面的观测性（traces、metrics、logs）。

## 🎯 功能特性

### 📊 完整的观测性栈
- **链路追踪 (Traces)**: 完整的请求追踪，从前端到游戏服务器
- **指标收集 (Metrics)**: 业务指标和系统指标，符合游戏行业标准
- **日志聚合 (Logs)**: 结构化日志，支持分布式日志关联

### 🎮 游戏业务语义
- 基于游戏行业的 Semantic Conventions
- 支持会话、关卡、经济、对战等游戏核心概念
- 自动数据脱敏和隐私保护

### 🚀 生产就绪
- 完整的配置管理（环境变量、YAML、CLI）
- 性能优化（采样、批处理、内存限制）
- 容器化部署（Docker Compose）

## 📁 目录结构

```
examples/otel-integration/
├── README.md                 # 本文档
├── go.mod                    # Go 模块
├── cmd/
│   ├── server/               # 示例服务器
│   ├── client/               # 示例客户端
│   └── game-simulator/       # 游戏事件模拟器
├── internal/
│   ├── telemetry/           # OTel 集成核心
│   └── game/                # 游戏业务逻辑
├── configs/
│   ├── otel-collector.yaml  # OTel Collector 配置
│   ├── prometheus.yml       # Prometheus 配置
│   └── jaeger.yml           # Jaeger 配置
├── docker/
│   ├── docker-compose.yml   # 完整的观测性栈
│   └── Dockerfile.*         # 各组件镜像
└── scripts/
    ├── start.sh             # 启动脚本
    ├── load-test.sh         # 压力测试
    └── demo.sh              # 演示脚本
```

## 🚀 快速开始

### 1. 启动观测性基础设施

```bash
cd examples/otel-integration
docker-compose up -d
```

这将启动：
- **Jaeger**: http://localhost:16686 (链路追踪 UI)
- **Prometheus**: http://localhost:9090 (指标查询)
- **Grafana**: http://localhost:3000 (可视化，admin/admin)
- **OTel Collector**: localhost:4317 (gRPC), localhost:4318 (HTTP)

### 2. 构建并运行示例

```bash
# 构建示例程序
go build -o bin/server cmd/server/main.go
go build -o bin/client cmd/client/main.go
go build -o bin/game-simulator cmd/game-simulator/main.go

# 运行服务器
./bin/server

# 在另一个终端运行客户端
./bin/client

# 运行游戏事件模拟器
./bin/game-simulator
```

### 3. 查看观测数据

- **链路追踪**: 访问 Jaeger UI，查看完整的请求链路
- **指标监控**: 访问 Grafana，查看游戏业务仪表板
- **日志查询**: 查看服务器输出的结构化日志

## 🎮 游戏业务指标

### 用户活跃度指标
- `game.users.daily_active`: 日活跃用户
- `game.users.weekly_active`: 周活跃用户
- `game.session.duration`: 会话时长分布
- `game.retention.d1/d7/d30`: 留存率

### 游戏玩法指标
- `game.level.start/complete/fail`: 关卡开始/完成/失败
- `game.match.start/end`: 对战开始/结束
- `game.economy.earn/spend`: 货币获得/消费

### 技术指标
- `game.client.fps`: 客户端帧率
- `game.network.latency`: 网络延迟
- `game.client.crash.rate`: 崩溃率

### 变现指标
- `game.monetization.revenue`: 收入
- `game.monetization.arpu`: 每用户收入
- `game.ad.revenue`: 广告收入

## 📊 Semantic Conventions

本示例遵循游戏行业的 Semantic Conventions，包括：

### 基础属性
```
game.id: 游戏ID
game.user_id: 用户ID（已脱敏）
game.session_id: 会话ID
game.platform: 平台（ios/android/web）
game.region: 地区
game.version: 游戏版本
```

### 会话属性
```
session.entry_point: 入口点
session.duration_ms: 会话时长
session.cause_end: 结束原因
```

### 经济属性
```
economy.currency: 货币类型
economy.amount: 数量
economy.source: 来源
economy.sink: 消费去向
```

## ⚙️ 配置选项

### 环境变量配置

```bash
# 基础配置
OTEL_SERVICE_NAME=game-server
OTEL_SERVICE_VERSION=1.0.0
OTEL_ENVIRONMENT=production
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# 采样配置
OTEL_SAMPLING_RATIO=0.1  # 10% 采样率
OTEL_ENABLE_TRACING=true
OTEL_ENABLE_METRICS=true

# 游戏特定配置
GAME_ID=my-awesome-game
ANALYTICS_BRIDGE_ENABLED=true
ANALYTICS_REDIS_ADDR=localhost:6379
```

### YAML 配置文件

```yaml
telemetry:
  service_name: "game-server"
  service_version: "1.0.0"
  environment: "production"
  collector_url: "http://localhost:4318"
  game_id: "my-awesome-game"
  enable_tracing: true
  enable_metrics: true
  sampling_ratio: 0.1

  analytics:
    enabled: true
    redis_addr: "localhost:6379"
    topic_prefix: "game:events"
    retention_hours: 168  # 7天
```

## 🔧 高级功能

### 1. 自定义指标

```go
// 创建自定义指标
playerCounter, err := meter.Int64Counter("game.player.count",
    metric.WithDescription("当前在线玩家数"),
    metric.WithUnit("{players}"),
)

// 记录指标
playerCounter.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("game.id", "my-game"),
        attribute.String("game.region", "cn-east"),
    ),
)
```

### 2. 分布式追踪

```go
// 创建 Span
ctx, span := tracer.Start(ctx, "game.level.start",
    trace.WithAttributes(
        attribute.String("game.id", gameID),
        attribute.String("level.id", levelID),
        attribute.String("player.id", playerID),
    ),
)
defer span.End()

// 添加事件
span.AddEvent("level.loading.start")
// ... 游戏逻辑
span.AddEvent("level.loading.complete")
```

### 3. 结构化日志关联

```go
// 从 span 获取 trace 信息并记录到日志
spanCtx := span.SpanContext()
logger.InfoContext(ctx, "关卡开始",
    slog.String("trace_id", spanCtx.TraceID().String()),
    slog.String("span_id", spanCtx.SpanID().String()),
    slog.String("game.id", gameID),
    slog.String("level.id", levelID),
)
```

## 🚦 性能考虑

### 采样策略
- **开发环境**: 100% 采样 (`sampling_ratio: 1.0`)
- **测试环境**: 10% 采样 (`sampling_ratio: 0.1`)
- **生产环境**: 1-5% 采样 (`sampling_ratio: 0.01-0.05`)

### 批处理优化
- 链路数据批次大小: 512
- 指标推送间隔: 30秒
- 内存限制: 512MB

### 网络优化
- 使用 gRPC (4317) 而非 HTTP (4318) 以获得更好性能
- 启用 gzip 压缩
- 配置连接池

## 🛡️ 隐私与安全

### 数据脱敏
- 用户ID自动哈希化
- 敏感字段过滤
- IP地理位置聚合

### 安全传输
- TLS 加密（生产环境）
- API 密钥认证
- 网络隔离

## 📈 监控仪表板

### Grafana 仪表板
- **游戏概览**: DAU, MAU, 收入概览
- **性能监控**: 延迟, 错误率, 吞吐量
- **玩法分析**: 关卡完成率, 对战胜率
- **变现分析**: ARPU, ARPPU, 付费转化

### 告警规则
- 崩溃率 > 1%
- 平均延迟 > 500ms
- 错误率 > 5%
- DAU 下降 > 20%

## 🧪 测试与验证

### 负载测试
```bash
# 运行负载测试
./scripts/load-test.sh --users 1000 --duration 60s
```

### 集成测试
```bash
# 验证观测性数据
go test ./test/integration/... -tags=integration
```

## 📚 参考文档

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [游戏行业 Semantic Conventions](./docs/semantic-conventions.md)
- [性能优化指南](./docs/performance-tuning.md)
- [故障排除](./docs/troubleshooting.md)

## 🤝 贡献

欢迎提交 Issue 和 Pull Request 来改进这个示例！

## 📄 许可证

本示例遵循与 Croupier 项目相同的许可证。