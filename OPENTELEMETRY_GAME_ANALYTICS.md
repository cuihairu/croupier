## OpenTelemetry 游戏分析架构设计

### 一、自定义 Semantic Conventions

#### 1.1 游戏业务指标扩展

```go
// 游戏业务 Semantic Conventions
package gameconv

const (
    // 游戏基础属性
    GameIDKey        = attribute.Key("game.id")
    GameVersionKey   = attribute.Key("game.version")
    GameEnvKey       = attribute.Key("game.environment")
    GameModeKey      = attribute.Key("game.mode")        // PVP/PVE/Tutorial

    // 用户属性
    UserIDKey        = attribute.Key("user.id")
    UserLevelKey     = attribute.Key("user.level")
    UserVIPKey       = attribute.Key("user.vip_level")
    UserRegionKey    = attribute.Key("user.region")

    // 会话属性
    SessionIDKey     = attribute.Key("session.id")
    SessionTypeKey   = attribute.Key("session.type")    // normal/background/inactive

    // 游戏内容属性
    ContentIDKey     = attribute.Key("content.id")       // 关卡ID、副本ID
    ContentTypeKey   = attribute.Key("content.type")     // level/dungeon/pvp
    ContentDifficultyKey = attribute.Key("content.difficulty")

    // 经济属性
    CurrencyTypeKey  = attribute.Key("economy.currency_type")  // gold/diamond/premium
    ItemIDKey        = attribute.Key("economy.item_id")
    TransactionIDKey = attribute.Key("economy.transaction_id")

    // 社交属性
    GuildIDKey       = attribute.Key("social.guild_id")
    TeamIDKey        = attribute.Key("social.team_id")
    FriendCountKey   = attribute.Key("social.friend_count")
)

// 游戏事件类型
const (
    // 用户生命周期
    EventUserRegister    = "user.register"
    EventUserLogin       = "user.login"
    EventUserLogout      = "user.logout"
    EventSessionStart    = "session.start"
    EventSessionEnd      = "session.end"

    // 游戏进度
    EventLevelStart      = "gameplay.level.start"
    EventLevelComplete   = "gameplay.level.complete"
    EventLevelFail       = "gameplay.level.fail"
    EventQuestComplete   = "gameplay.quest.complete"
    EventAchievementUnlock = "gameplay.achievement.unlock"

    // 经济行为
    EventPurchaseStart   = "economy.purchase.start"
    EventPurchaseComplete = "economy.purchase.complete"
    EventPurchaseFail    = "economy.purchase.fail"
    EventCurrencyEarn    = "economy.currency.earn"
    EventCurrencySpend   = "economy.currency.spend"
    EventItemObtain      = "economy.item.obtain"
    EventItemConsume     = "economy.item.consume"

    // 社交行为
    EventFriendAdd       = "social.friend.add"
    EventGuildJoin       = "social.guild.join"
    EventChatSend        = "social.chat.send"
    EventGiftSend        = "social.gift.send"

    // 竞技行为
    EventBattleStart     = "combat.battle.start"
    EventBattleEnd       = "combat.battle.end"
    EventSkillUse        = "combat.skill.use"
    EventPVPMatch        = "combat.pvp.match"
)
```

#### 1.2 游戏指标定义

```go
// 游戏核心指标注册
func RegisterGameMetrics(meter metric.Meter) *GameMetrics {
    return &GameMetrics{
        // 用户活跃指标
        DAU: meter.Int64ObservableGauge("game.users.daily_active",
            metric.WithDescription("Daily Active Users"),
            metric.WithUnit("{users}"),
        ),

        MAU: meter.Int64ObservableGauge("game.users.monthly_active",
            metric.WithDescription("Monthly Active Users"),
            metric.WithUnit("{users}"),
        ),

        NewUsers: meter.Int64Counter("game.users.new_registrations",
            metric.WithDescription("New user registrations"),
            metric.WithUnit("{users}"),
        ),

        // 会话指标
        SessionDuration: meter.Float64Histogram("game.session.duration",
            metric.WithDescription("Game session duration"),
            metric.WithUnit("s"),
            metric.WithBuckets([]float64{30, 60, 300, 600, 1800, 3600, 7200}...),
        ),

        SessionCount: meter.Int64Counter("game.session.count",
            metric.WithDescription("Number of game sessions"),
            metric.WithUnit("{sessions}"),
        ),

        // 经济指标
        Revenue: meter.Float64Counter("game.economy.revenue",
            metric.WithDescription("Game revenue"),
            metric.WithUnit("USD"),
        ),

        ARPU: meter.Float64ObservableGauge("game.economy.arpu",
            metric.WithDescription("Average Revenue Per User"),
            metric.WithUnit("USD"),
        ),

        ConversionRate: meter.Float64ObservableGauge("game.economy.conversion_rate",
            metric.WithDescription("Payment conversion rate"),
            metric.WithUnit("1"),
        ),

        // 内容指标
        LevelCompletion: meter.Float64Histogram("game.content.level.completion_rate",
            metric.WithDescription("Level completion rate"),
            metric.WithUnit("1"),
            metric.WithBuckets([]float64{0.1, 0.2, 0.3, 0.5, 0.7, 0.8, 0.9, 0.95, 0.99, 1.0}...),
        ),

        LevelDuration: meter.Float64Histogram("game.content.level.duration",
            metric.WithDescription("Time to complete level"),
            metric.WithUnit("s"),
        ),

        // 技术指标
        ClientFPS: meter.Float64Histogram("game.client.fps",
            metric.WithDescription("Client frame rate"),
            metric.WithUnit("fps"),
            metric.WithBuckets([]float64{15, 30, 45, 60, 75, 90, 120, 144}...),
        ),

        LoadTime: meter.Float64Histogram("game.client.load_time",
            metric.WithDescription("Game loading time"),
            metric.WithUnit("ms"),
        ),

        NetworkLatency: meter.Float64Histogram("game.network.latency",
            metric.WithDescription("Network round-trip latency"),
            metric.WithUnit("ms"),
        ),
    }
}
```

### 二、多层级数据收集架构

#### 2.1 客户端集成 (Unity/Unreal SDK)

```go
// Unity C# SDK 示例
using OpenTelemetry.Api;
using OpenTelemetry.Instrumentation;

public class GameTelemetrySDK {
    private readonly Tracer tracer;
    private readonly Meter meter;

    public GameTelemetrySDK() {
        tracer = TracerProvider.Default.GetTracer("game.client");
        meter = MeterProvider.Default.GetMeter("game.client");
    }

    // 用户行为追踪
    public void TrackUserAction(string action, Dictionary<string, object> properties) {
        using var activity = tracer.StartActivity($"user.{action}");
        activity?.SetTag(GameConv.UserIDKey, GameManager.CurrentUserID);
        activity?.SetTag(GameConv.SessionIDKey, SessionManager.CurrentSessionID);

        foreach (var prop in properties) {
            activity?.SetTag(prop.Key, prop.Value);
        }

        // 记录业务指标
        var counter = meter.CreateCounter<long>($"game.user.{action}.count");
        counter.Add(1, new TagList {
            { GameConv.UserIDKey, GameManager.CurrentUserID },
            { GameConv.GameModeKey, GameManager.CurrentMode }
        });
    }

    // 关卡性能追踪
    public IDisposable StartLevelTrace(string levelID) {
        var activity = tracer.StartActivity("gameplay.level");
        activity?.SetTag(GameConv.ContentIDKey, levelID);
        activity?.SetTag(GameConv.ContentTypeKey, "level");

        return new LevelTraceScope(activity, levelID, meter);
    }

    // 支付流程追踪
    public void TrackPurchase(string productID, decimal amount, string currency) {
        using var activity = tracer.StartActivity("economy.purchase");
        activity?.SetTag("product.id", productID);
        activity?.SetTag("purchase.amount", amount);
        activity?.SetTag("purchase.currency", currency);

        var revenueCounter = meter.CreateCounter<double>("game.economy.revenue");
        revenueCounter.Add((double)amount, new TagList {
            { "currency", currency },
            { "product.id", productID }
        });
    }

    // 性能监控
    public void RecordPerformance() {
        var fpsGauge = meter.CreateObservableGauge<float>("game.client.fps");
        var memoryGauge = meter.CreateObservableGauge<long>("game.client.memory_usage");

        meter.RegisterCallback(() => {
            fpsGauge.Observe(Application.targetFrameRate);
            memoryGauge.Observe(GC.GetTotalMemory(false));
        });
    }
}

// 关卡追踪作用域
public class LevelTraceScope : IDisposable {
    private readonly Activity activity;
    private readonly Histogram<double> durationHistogram;
    private readonly DateTime startTime;

    public LevelTraceScope(Activity activity, string levelID, Meter meter) {
        this.activity = activity;
        this.startTime = DateTime.UtcNow;
        this.durationHistogram = meter.CreateHistogram<double>("game.content.level.duration");
    }

    public void Dispose() {
        var duration = (DateTime.UtcNow - startTime).TotalSeconds;
        durationHistogram.Record(duration, new TagList {
            { GameConv.ContentIDKey, activity?.GetTagItem(GameConv.ContentIDKey) }
        });
        activity?.Dispose();
    }
}
```

#### 2.2 服务器端集成 (Go Backend)

```go
// 游戏服务器 OpenTelemetry 集成
package analytics

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/trace"
)

type GameAnalyticsService struct {
    tracer trace.Tracer
    meter  metric.Meter

    // 核心指标
    userLoginCounter    metric.Int64Counter
    revenueCounter     metric.Float64Counter
    levelCompletionHist metric.Float64Histogram
    sessionDurationHist metric.Float64Histogram
}

func NewGameAnalyticsService() *GameAnalyticsService {
    tracer := otel.Tracer("game.server")
    meter := otel.Meter("game.server")

    return &GameAnalyticsService{
        tracer: tracer,
        meter:  meter,
        userLoginCounter: meter.Int64Counter("game.user.login.count"),
        revenueCounter: meter.Float64Counter("game.economy.revenue"),
        levelCompletionHist: meter.Float64Histogram("game.content.level.completion_rate"),
        sessionDurationHist: meter.Float64Histogram("game.session.duration"),
    }
}

// 用户登录事件
func (s *GameAnalyticsService) TrackUserLogin(ctx context.Context, userID, gameID, env string) {
    ctx, span := s.tracer.Start(ctx, "user.login")
    defer span.End()

    span.SetAttributes(
        attribute.String("game.id", gameID),
        attribute.String("game.env", env),
        attribute.String("user.id", userID),
    )

    s.userLoginCounter.Add(ctx, 1, metric.WithAttributes(
        attribute.String("game.id", gameID),
        attribute.String("game.env", env),
    ))
}

// 关卡完成事件
func (s *GameAnalyticsService) TrackLevelComplete(ctx context.Context, req LevelCompleteRequest) {
    ctx, span := s.tracer.Start(ctx, "gameplay.level.complete")
    defer span.End()

    span.SetAttributes(
        attribute.String("user.id", req.UserID),
        attribute.String("level.id", req.LevelID),
        attribute.Int("level.attempts", req.Attempts),
        attribute.Float64("level.duration", req.Duration.Seconds()),
        attribute.Bool("level.completed", req.Success),
    )

    // 记录完成率
    completionRate := 0.0
    if req.Success {
        completionRate = 1.0
    }

    s.levelCompletionHist.Record(ctx, completionRate, metric.WithAttributes(
        attribute.String("level.id", req.LevelID),
        attribute.String("level.difficulty", req.Difficulty),
    ))
}

// 支付事件追踪
func (s *GameAnalyticsService) TrackPurchase(ctx context.Context, purchase PurchaseEvent) {
    ctx, span := s.tracer.Start(ctx, "economy.purchase.complete")
    defer span.End()

    span.SetAttributes(
        attribute.String("user.id", purchase.UserID),
        attribute.String("product.id", purchase.ProductID),
        attribute.Float64("purchase.amount", purchase.Amount),
        attribute.String("purchase.currency", purchase.Currency),
        attribute.String("payment.method", purchase.PaymentMethod),
    )

    s.revenueCounter.Add(ctx, purchase.Amount, metric.WithAttributes(
        attribute.String("product.id", purchase.ProductID),
        attribute.String("currency", purchase.Currency),
        attribute.String("payment.method", purchase.PaymentMethod),
    ))
}
```

#### 2.3 OpenTelemetry Collector 配置

```yaml
# otel-collector-config.yaml
receivers:
  # 接收游戏客户端数据
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
        cors:
          allowed_origins:
            - "*"

  # 接收服务器日志
  filelog:
    include: [ "/var/log/game/*.log" ]
    operators:
      - type: json_parser
        parse_from: body

processors:
  # 游戏特有的数据处理
  transform:
    trace_statements:
      - context: span
        statements:
          # 添加游戏环境标签
          - set(attributes["deployment.environment"], "production")
          # 用户ID脱敏（保留前3位+后2位）
          - replace_pattern(attributes["user.id"], "(?P<prefix>.{3}).*(?P<suffix>.{2})", "$${prefix}***$${suffix}")

    metric_statements:
      - context: metric
        statements:
          # 转换单位：毫秒 -> 秒
          - set(unit, "s") where name == "game.client.load_time"
          - set(value, value / 1000) where name == "game.client.load_time"

  # 批处理优化
  batch:
    timeout: 1s
    send_batch_size: 1024
    send_batch_max_size: 2048

  # 内存限制
  memory_limiter:
    limit_mib: 512

exporters:
  # 导出到 ClickHouse
  clickhouse:
    endpoint: "http://clickhouse:8123"
    database: "analytics"
    ttl: 720h  # 30天
    create_schema: true

    # 表映射配置
    table_mappings:
      traces:
        table_name: "otel_traces"
        partition_by: "toYYYYMM(timestamp)"
        order_by: ["game_id", "user_id", "timestamp"]

      metrics:
        table_name: "otel_metrics"
        partition_by: "toYYYYMM(timestamp)"
        order_by: ["metric_name", "game_id", "timestamp"]

      logs:
        table_name: "otel_logs"
        partition_by: "toYYYYMM(timestamp)"
        order_by: ["game_id", "log_level", "timestamp"]

  # 导出到 Redis (实时指标)
  redis:
    endpoint: "redis:6379"
    key_prefix: "otel:game:"
    ttl: 3600  # 1小时

  # 导出到 Prometheus (监控告警)
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: "game"
    const_labels:
      service: "game-analytics"

service:
  pipelines:
    # 链路追踪管道
    traces:
      receivers: [otlp, filelog]
      processors: [transform, memory_limiter, batch]
      exporters: [clickhouse]

    # 指标管道
    metrics:
      receivers: [otlp]
      processors: [transform, memory_limiter, batch]
      exporters: [clickhouse, redis, prometheus]

    # 日志管道
    logs:
      receivers: [otlp, filelog]
      processors: [transform, memory_limiter, batch]
      exporters: [clickhouse]

  extensions: [health_check, pprof, zpages]
```

### 三、游戏特有的查询和分析

#### 3.1 Trace 查询示例 (用户行为路径)

```sql
-- 查询用户完整游戏会话轨迹
SELECT
    user_id,
    span_name,
    start_time,
    duration,
    attributes['level.id'] as level_id,
    attributes['level.difficulty'] as difficulty,
    parent_span_id
FROM otel_traces
WHERE trace_id = '123e4567-e89b-12d3-a456-426614174000'
  AND game_id = 'my_game'
ORDER BY start_time;

-- 分析关卡完成路径
WITH level_sessions AS (
  SELECT
    user_id,
    trace_id,
    span_name,
    attributes['level.id'] as level_id,
    CASE
      WHEN span_name = 'gameplay.level.complete' THEN 'completed'
      WHEN span_name = 'gameplay.level.fail' THEN 'failed'
      ELSE 'started'
    END as level_status,
    start_time
  FROM otel_traces
  WHERE span_name LIKE 'gameplay.level.%'
    AND date >= today() - 7
)
SELECT
  level_id,
  countIf(level_status = 'started') as attempts,
  countIf(level_status = 'completed') as completions,
  completions / attempts as completion_rate
FROM level_sessions
GROUP BY level_id
ORDER BY completion_rate DESC;
```

#### 3.2 Metrics 聚合查询

```sql
-- 实时DAU计算
SELECT
  toHour(timestamp) as hour,
  uniqExact(attributes['user.id']) as dau
FROM otel_metrics
WHERE metric_name = 'game.user.login.count'
  AND date = today()
  AND attributes['game.id'] = 'my_game'
GROUP BY hour
ORDER BY hour;

-- 收入趋势分析
SELECT
  toStartOfDay(timestamp) as date,
  sum(value) as daily_revenue,
  uniqExact(attributes['user.id']) as paying_users,
  daily_revenue / paying_users as arppu
FROM otel_metrics
WHERE metric_name = 'game.economy.revenue'
  AND timestamp >= now() - INTERVAL 30 DAY
GROUP BY date
ORDER BY date;

-- 关卡难度分析
SELECT
  attributes['level.id'] as level_id,
  attributes['level.difficulty'] as difficulty,
  avg(value) as avg_completion_rate,
  count() as total_attempts
FROM otel_metrics
WHERE metric_name = 'game.content.level.completion_rate'
  AND timestamp >= now() - INTERVAL 7 DAY
GROUP BY level_id, difficulty
HAVING total_attempts >= 100  -- 过滤样本量过小的关卡
ORDER BY avg_completion_rate ASC;
```

### 四、实施优势分析

#### 4.1 对比传统方案的优势

| 维度 | 传统自建方案 | OpenTelemetry方案 | 优势 |
|------|-------------|-------------------|------|
| **标准化** | 自定义格式，维护成本高 | 行业标准，互操作性强 | ⭐⭐⭐⭐⭐ |
| **多语言支持** | 需要为每个语言开发SDK | 官方支持主流语言 | ⭐⭐⭐⭐⭐ |
| **生态集成** | 需要自己对接各种后端 | 丰富的exporter生态 | ⭐⭐⭐⭐ |
| **性能优化** | 需要自己优化采样和批处理 | 内置高性能处理器 | ⭐⭐⭐⭐ |
| **可观测性** | 缺乏链路追踪能力 | Trace/Metric/Log统一 | ⭐⭐⭐⭐⭐ |
| **运维复杂度** | 高 | 中等 | ⭐⭐⭐ |

#### 4.2 实施建议

```yaml
实施路线图:
  Phase 1 (1-2月):
    - 服务器端集成 OpenTelemetry Go SDK
    - 部署 OTel Collector
    - 配置 ClickHouse Exporter

  Phase 2 (2-3月):
    - 开发 Unity/Unreal 客户端 SDK
    - 实现游戏业务指标定义
    - 配置实时指标导出到 Redis

  Phase 3 (3-4月):
    - 完善游戏特有的 Semantic Conventions
    - 开发自定义可视化面板
    - 集成告警和异常检测

技术风险评估:
  数据量挑战: ⭐⭐⭐ (需要合理采样策略)
  集成复杂度: ⭐⭐⭐ (客户端集成需要仔细设计)
  性能影响: ⭐⭐ (OTel 性能优化良好)
  学习成本: ⭐⭐ (有标准文档支持)
```

#### 4.3 成本收益分析

```
开发成本:
  - 初期集成: 2-3人月
  - 运维成本: 降低30-50% (标准化工具链)
  - 扩展成本: 降低70% (多语言SDK复用)

业务收益:
  - 数据质量: 提升40% (标准化格式，减少错误)
  - 开发效率: 提升50% (标准SDK，减少重复开发)
  - 问题定位: 提升80% (分布式追踪能力)
  - 运营洞察: 提升60% (链路级别的用户行为分析)
```

总结来说，OpenTelemetry为游戏分析系统提供了一个现代化、标准化的解决方案，虽然有一定的学习成本，但长期来看能显著降低维护成本并提升数据质量。特别适合多平台、多语言的游戏项目。