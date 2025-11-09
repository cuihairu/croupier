# Croupier 游戏分析系统 - 缺失关键指标深度分析报告

## 执行摘要

基于对Croupier系统架构的深入分析，当前系统已实现基础的分析能力（用户、付费、事件、漏斗等），但在以下**10个战略性维度**仍存在重大缺口。本报告对每个维度的**缺失指标、实现方案和商业价值**进行系统分析。

---

## 1. 技术运营指标（DevOps & System Reliability）

### 当前状态
✅ 已有：基础的在线人数、DAU/WAU/MAU、支付数据

❌ 缺失：系统性能、稳定性、质量指标

### 缺失关键指标

#### 1.1 可用性和性能指标
```
指标名称                    定义                         告警阈值      现状
─────────────────────────────────────────────────────────────────────
API响应时间(P50/P95/P99)   API调用延迟百分位数          P99>500ms     ❌ 无
错误率(5xx/4xx)            HTTP错误占比                 >1%           ❌ 无
吞吐量(RPS)                每秒请求数                   <预期80%      ❌ 无
热点接口TOP 10             负载最高的API列表            -             ❌ 无
```

#### 1.2 系统健康指标
```
指标名称                    定义                         告警阈值      现状
─────────────────────────────────────────────────────────────────────
数据库连接池使用率         活跃连接/总连接数            >80%          ❌ 无
缓存命中率                 缓存命中次数/(命中+未命中)   <85%          ⚠️ Redis HLL有
队列堆积深度               Analytics MQ待处理消息数    >10000         ❌ 无
ClickHouse查询延迟         分析查询P95延迟              >2s           ❌ 无
```

#### 1.3 数据质量指标
```
指标名称                    定义                         告警阈值      现状
─────────────────────────────────────────────────────────────────────
事件缺失率                  入库失败/总上报数            >0.1%         ❌ 无
数据延迟                    事件上报到查询可见的延迟     >5分钟        ❌ 无
重复事件率                  相同事件ID重复出现的比例     >0.01%        ❌ 无
字段完整度                  记录中非空字段占比           <95%          ❌ 无
```

### 为什么重要
- **业务影响**：系统性能问题直接影响用户体验和承载能力
- **成本优化**：及早发现瓶颈可降低基础设施成本20-30%
- **可靠性**：数据质量问题导致决策偏差，影响ROI计算

### 实现方案

#### 方案A：Prometheus + Grafana（推荐）
```go
// 在analytics_routes.go中增加Prometheus指标
import "github.com/prometheus/client_golang/prometheus"

var (
    httpDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Buckets: []float64{.001, .01, .05, .1, .5, 1},
        },
        []string{"handler", "method", "status"},
    )
    
    eventIngestErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "event_ingest_errors_total",
        },
        []string{"reason"},
    )
)

// 中间件：记录API性能
func promMiddleware(c *gin.Context) {
    start := time.Now()
    c.Next()
    duration := time.Since(start).Seconds()
    httpDuration.WithLabelValues(
        c.Request.URL.Path,
        c.Request.Method,
        strconv.Itoa(c.Writer.Status()),
    ).Observe(duration)
}
```

#### 数据表设计
```sql
-- 性能日志表（ClickHouse）
CREATE TABLE IF NOT EXISTS analytics.api_metrics (
    timestamp DateTime,
    game_id String,
    env String,
    handler String,
    method String,
    status_code UInt16,
    response_time_ms Float32,
    request_size UInt32,
    response_size UInt32
) ENGINE = MergeTree()
ORDER BY (timestamp, game_id, handler)
TTL timestamp + INTERVAL 90 DAY;

-- 数据质量表
CREATE TABLE IF NOT EXISTS analytics.data_quality (
    check_time DateTime,
    game_id String,
    metric_name String,
    metric_value Float32,
    threshold Float32,
    status String -- 'pass', 'warning', 'alert'
) ENGINE = MergeTree()
ORDER BY (check_time, game_id, metric_name);
```

#### 优先级和成本
```
投入成本：    中等（2-3周）
实现难度：    低（基于成熟方案）
ROI周期：    2周（快速发现问题）
优先级：     🔴 高（基础设施级）
```

---

## 2. 产品迭代指标（Product Analytics）

### 当前状态
✅ 已有：事件采集、基础漏斗分析

❌ 缺失：版本效果对比、功能采纳率、A/B测试支持

### 缺失关键指标

#### 2.1 版本迭代效果
```
指标名称                      定义                           目标        现状
────────────────────────────────────────────────────────────────────────
版本升级率                   新版本用户/总用户              >70%        ❌ 无
新版本vs旧版本对比
  - DAU变化                  新版本DAU vs 旧版本DAU         +5-10%      ❌ 无
  - ARPU变化                 新版本ARPU vs 旧版本ARPU       +3-8%       ❌ 无
  - 留存率变化               D7留存对比                     +2-5%       ❌ 无
  - 崩溃率                   版本级别的crash count          <1%         ❌ 无
功能采纳率                   使用新功能的用户/总用户        >40%        ❌ 无
```

#### 2.2 A/B测试框架
```
指标名称                      定义                           覆盖范围    现状
────────────────────────────────────────────────────────────────────────
实验管理平台                  可视化创建、运行、分析实验     全类型      ❌ 无
流量分配                      分配给对照组/实验组的流量      1%-100%     ❌ 无
统计显著性检验                验证结果可信度                 p<0.05      ❌ 无
样本量计算                    基于MDE计算所需样本             -           ❌ 无
```

#### 2.3 功能影响分析
```
指标名称                      定义                           示例        现状
────────────────────────────────────────────────────────────────────────
功能热度排行                  功能使用频率排序               TOP 20      ❌ 无
功能用户分布                  使用该功能的用户特征           新手/老玩家 ❌ 无
功能留存贡献度                功能使用与留存的相关性        r系数       ❌ 无
功能ARPU贡献度                功能使用与付费的相关性        提升幅度    ❌ 无
```

### 为什么重要
- **产品决策**：数据驱动的功能优先级排序，降低试错成本50%
- **A/B测试**：避免主观决策，提高新功能成功率到75%以上
- **迭代加速**：版本对比数据可快速识别回归问题

### 实现方案

#### 方案B：实验平台架构
```go
// 内部数据模型
type Experiment struct {
    ID              string    // exp_001
    Name            string    // "新手引导优化"
    Type            string    // "feature" | "ab_test" | "rollout"
    StartTime       time.Time
    EndTime         time.Time
    TargetAudience  Filter    // 用户筛选条件
    Groups          []Group   // 对照/实验组
    Metrics         []string  // 追踪指标ID
    Status          string    // "planning" | "running" | "finished"
    Result          map[string]interface{} // 统计结果
}

type Group struct {
    ID              string   // "control" | "variant_a" | "variant_b"
    AllocationRate  float32  // 0.5 (50%)
    UserCount       int64
    Properties      map[string]string // 投放的功能开关
}

// API：创建实验
POST /api/experiments
{
    "name": "新手引导V2测试",
    "type": "ab_test",
    "start_time": "2025-01-20T00:00:00Z",
    "end_time": "2025-02-20T00:00:00Z",
    "target_audience": {
        "created_at": ">2025-01-01",
        "server": "prod",
        "game_id": "game_001"
    },
    "groups": [
        {
            "id": "control",
            "allocation_rate": 0.5,
            "properties": {"tutorial_version": "v1"}
        },
        {
            "id": "variant",
            "allocation_rate": 0.5,
            "properties": {"tutorial_version": "v2"}
        }
    ],
    "metrics": ["d1_retention", "tutorial_completion_rate", "new_user_arpu"]
}

// API：获取实验结果
GET /api/experiments/{id}/results
{
    "experiment_id": "exp_001",
    "status": "running",
    "started_at": "2025-01-20T00:00:00Z",
    "results": {
        "tutorial_completion_rate": {
            "control": 0.65,
            "variant": 0.72,
            "p_value": 0.031,
            "significant": true,
            "winner": "variant"
        },
        "d1_retention": {
            "control": 0.43,
            "variant": 0.45,
            "p_value": 0.15,
            "significant": false
        }
    },
    "sample_sizes": {
        "control": 45000,
        "variant": 45000
    }
}
```

#### 数据表设计
```sql
-- 实验定义表
CREATE TABLE IF NOT EXISTS product.experiments (
    id String,
    name String,
    type String,
    game_id String,
    env String,
    start_time DateTime,
    end_time DateTime,
    status String,
    config JSON,
    created_at DateTime,
    created_by String
) ENGINE = MergeTree()
ORDER BY (game_id, start_time);

-- 实验分配表
CREATE TABLE IF NOT EXISTS product.experiment_assignments (
    timestamp DateTime,
    user_id String,
    experiment_id String,
    group_id String,
    game_id String,
    env String
) ENGINE = MergeTree()
ORDER BY (timestamp, experiment_id, user_id)
PARTITION BY toYYYYMM(timestamp);

-- 实验指标快照
CREATE TABLE IF NOT EXISTS product.experiment_metrics (
    snapshot_time DateTime,
    experiment_id String,
    group_id String,
    metric_name String,
    metric_value Float32,
    sample_size UInt64
) ENGINE = MergeTree()
ORDER BY (snapshot_time, experiment_id, metric_name);

-- 功能采纳表
CREATE TABLE IF NOT EXISTS product.feature_adoption (
    date Date,
    game_id String,
    env String,
    feature_id String,
    user_count UInt64,
    event_count UInt64,
    dau_adopters UInt64
) ENGINE = MergeTree()
ORDER BY (date, game_id, feature_id);
```

#### 优先级和成本
```
投入成本：    高（6-8周）
实现难度：    高（需要统计学知识）
ROI周期：    1个月（快速优化功能）
优先级：     🔴 高（产品决策核心）
```

---

## 3. 内容消费指标（Content Lifecycle）

### 当前状态
✅ 已有：关卡流量、win_rate统计

❌ 缺失：内容生命周期、UGC质量、消费深度

### 缺失关键指标

#### 3.1 内容生命周期指标
```
指标名称                      定义                        衡量周期    现状
────────────────────────────────────────────────────────────────────
内容热度曲线                  内容发布后的参与度变化       日/周/月    ❌ 无
内容平均生命周期              从上线到不活跃的天数         天         ❌ 无
内容衰减速率                  热度环比下降速度             周         ❌ 无
常青内容占比                  保持热度的内容比例           %          ❌ 无
内容更新频率要求              保持热度需要多久更新一次     天         ❌ 无
```

#### 3.2 UGC相关指标
```
指标名称                      定义                        告警值      现状
────────────────────────────────────────────────────────────────────
UGC创建率                     创建UGC的用户占比            <2%         ❌ 无
UGC质量评分                   用户评分/审核评分            <3.5星      ❌ 无
UGC被消费率                   有播放的UGC占比              <30%        ❌ 无
高质量创作者占比              高产/优质创作者比例          %          ❌ 无
审核通过率                    通过审核的UGC占比            <70%        ❌ 无
审核平均耗时                  从提交到审核结果的时间       >2小时      ❌ 无
UGC平均生命周期               UGC活跃时间                  天         ❌ 无
```

#### 3.3 消费深度指标
```
指标名称                      定义                        现状
────────────────────────────────────────────────────────────────────
内容消费序列                  用户的内容消费轨迹          ❌ 无
跳出率                        开始消费但未完成的比例      ❌ 无
重复消费率                    消费相同内容多次的用户占比  ❌ 无
内容推荐命中率                推荐内容被消费的概率        ❌ 无
内容相似度图谱                关联内容网络                ❌ 无
```

### 为什么重要
- **运营策略**：精准把握内容生命周期，延长热度期平均15-25%
- **创作激励**：UGC数据驱动的激励机制提高产出质量
- **推荐优化**：了解消费深度改进推荐算法的准确性

### 实现方案

#### 方案C：内容热度追踪系统
```go
// 内容热度的时序数据结构
type ContentMetrics struct {
    Date              time.Time
    ContentID         string
    ContentType       string      // "ugc" | "official" | "event"
    Views             uint64
    Engagement        uint64      // 互动次数（赞、评论、分享）
    AverageEngageRate float32     // 互动/浏览
    UniqUsers         uint64
    ReturnVisitors    uint64      // 回头用户
    HeatScore         float32     // 0-100 热度评分
}

// 热度评分算法（可定制）
func calculateHeatScore(m ContentMetrics) float32 {
    // 公式：基于view/engagement/return_visitor的加权平均
    baseScore := math.Log10(float64(m.Views + 1)) * 20  // 对数衰减
    engagement := float64(m.EngagementRate) * 100 * 30  // 互动占比
    retention := float64(m.ReturnVisitors) / 
                 math.Max(1, float64(m.UniqUsers)) * 50  // 回头用户
    return float32(baseScore + engagement + retention)
}

// API：获取内容生命周期
GET /api/analytics/content/lifecycle?content_id=ugc_12345&days=90
{
    "content_id": "ugc_12345",
    "content_type": "ugc",
    "creator_id": "user_999",
    "created_at": "2024-11-01T10:00:00Z",
    "lifecycle_stage": "decline",
    "total_views": 1250000,
    "total_engagement": 45000,
    "avg_engagement_rate": 0.036,
    "peak_heat_score": 92.5,
    "current_heat_score": 12.3,
    "heat_decline_rate": 0.18,  // 周衰减18%
    "expected_inactive_date": "2024-12-15T00:00:00Z",
    "metrics_by_date": [
        {
            "date": "2024-11-01",
            "views": 50000,
            "engagement": 2500,
            "engagement_rate": 0.05,
            "heat_score": 85.2
        }
        // ... 更多天数
    ]
}

// UGC质量评估
type UGCQualityAssessment struct {
    UGCID            string
    UserRating       float32    // 1-5
    AutomQAScore     float32    // 自动质量评分（基于内容长度、格式等）
    AuditStatus      string     // "pending" | "approved" | "rejected"
    AuditFeedback    string
    AuditTime        time.Time
    QualityTier      string     // "excellent" | "good" | "fair" | "poor"
}
```

#### 数据表设计
```sql
-- 内容热度时序表
CREATE TABLE IF NOT EXISTS content.heat_metrics (
    date Date,
    content_id String,
    content_type String,
    game_id String,
    views UInt64,
    engagement UInt32,
    uniq_users UInt32,
    return_visitors UInt32,
    heat_score Float32,
    lifecycle_stage String
) ENGINE = MergeTree()
ORDER BY (date, content_id)
PARTITION BY toYYYYMM(date);

-- UGC内容表
CREATE TABLE IF NOT EXISTS content.ugc (
    id String,
    creator_id String,
    game_id String,
    content_type String,
    created_at DateTime,
    user_rating Float32,
    qa_score Float32,
    audit_status String,
    quality_tier String,
    total_views UInt64,
    total_likes UInt32,
    total_comments UInt32
) ENGINE = MergeTree()
ORDER BY (created_at, game_id, creator_id);

-- 内容推荐效果表
CREATE TABLE IF NOT EXISTS content.recommendation_metrics (
    date Date,
    game_id String,
    recommender_type String,  -- "ml_based" | "rule_based" | "trending"
    content_id String,
    impression_count UInt32,
    click_count UInt32,
    completion_count UInt32,
    ctr Float32,
    completion_rate Float32
) ENGINE = MergeTree()
ORDER BY (date, recommender_type, content_id);
```

#### 优先级和成本
```
投入成本：    中等（3-4周）
实现难度：    中等（涉及算法）
ROI周期：    3周（优化内容策略）
优先级：     🟡 中等（依赖游戏内容特性）
```

---

## 4. 风控安全指标（Risk & Security）

### 当前状态
✅ 已有：基础的支付成功率

❌ 缺失：作弊检测、账号风控、交易防控

### 缺失关键指标

#### 4.1 作弊检测指标
```
指标名称                      定义                        告警阈值    现状
────────────────────────────────────────────────────────────────────
异常登录                      陌生地区/设备登录频率       5次/天      ❌ 无
概率性作弊                    (得分 / 游戏时长)异常高    >2σ         ❌ 无
机器人账号比例                自动化操作账户占比          >0.5%       ❌ 无
升级速度异常                  等级提升速度异常快          >1σ         ❌ 无
装备刷新频率异常              获得装备的频率异常          >2σ         ❌ 无
多账号关联度                  关联账户数                  >5个        ❌ 无
```

#### 4.2 账号风控指标
```
指标名称                      定义                        告警阈值    现状
────────────────────────────────────────────────────────────────────
账户被封禁比例                被封禁的用户占比            >1%         ❌ 无
申诉通过率                    用户申诉成功占比            <50%        ⚠️ 人工处理
处理时长                      从举报到处置的时间          >24小时     ❌ 无
复审准确率                    人工复审的正确率            <95%        ❌ 无
误封率                        错误封禁的比例              >0.1%       ❌ 无
```

#### 4.3 交易风控指标
```
指标名称                      定义                        告警阈值    现状
────────────────────────────────────────────────────────────────────
重复支付频率                  同一用户/订单重复支付       1次         ❌ 无
退款风险                      退款申请中欺诈申请比例      >5%         ❌ 无
高风险地区交易占比            特定地区的支付异常占比      >20%        ❌ 无
支付卡测试                    短时间多个小额支付          3次/分钟    ❌ 无
黑卡识别率                    使用黑卡支付的检出率        <95%        ❌ 无
频率限制违规                  超限流量触发/所有请求       >1%         ❌ 无
```

### 为什么重要
- **收入保护**：防控作弊和欺诈可保护5-15%的营收
- **用户信任**：公平的风控机制提高玩家留存率10-20%
- **合规性**：满足国际支付规范（PCI-DSS）要求

### 实现方案

#### 方案D：多层次风控系统
```go
// 风控规则引擎
type RiskRule struct {
    ID              string
    Name            string
    RuleType        string        // "login" | "payment" | "gameplay"
    Condition       RiskCondition // 规则条件
    Risk            string        // "low" | "medium" | "high"
    Action          string        // "allow" | "challenge" | "block"
    Challenge       *Challenge    // 二次验证配置
}

type RiskCondition struct {
    Operator   string      // "AND" | "OR"
    Conditions []Condition
}

type Condition struct {
    Field      string      // "login_country" | "payment_amount" | "level_up_speed"
    Operator   string      // "equals" | "gt" | "lt" | "in" | "regex"
    Value      interface{}
}

type Challenge struct {
    Type      string      // "sms" | "email" | "captcha"
    MaxTries  int
    Timeout   time.Duration
}

// 示例规则配置
var rules = []RiskRule{
    {
        ID:   "login_异常地点",
        Name: "从陌生地点登录",
        Condition: RiskCondition{
            Operator: "AND",
            Conditions: []Condition{
                {Field: "login_country", Operator: "!=", Value: "last_login_country"},
                {Field: "days_since_last_login", Operator: "lt", Value: 7},
            },
        },
        Risk:    "medium",
        Action:  "challenge",
        Challenge: &Challenge{Type: "sms", MaxTries: 3},
    },
    {
        ID:   "payment_概率性欺诈",
        Name: "支付金额异常高",
        Condition: RiskCondition{
            Conditions: []Condition{
                {Field: "payment_amount_vs_avg", Operator: "gt", Value: 5.0}, // 平均值的5倍
                {Field: "account_age_days", Operator: "lt", Value: 30},
            },
        },
        Risk:   "high",
        Action: "block",
    },
}

// 风控评分接口
POST /api/risk/evaluate
{
    "event_type": "payment",
    "user_id": "user_123",
    "payload": {
        "amount": 9999,
        "currency": "USD",
        "payment_method": "credit_card",
        "country": "US",
        "ip_country": "CN"
    }
}

Response:
{
    "user_id": "user_123",
    "risk_score": 0.87,
    "risk_level": "high",
    "triggered_rules": [
        {
            "rule_id": "payment_概率性欺诈",
            "risk": "high",
            "action": "block",
            "reason": "Payment amount 5x user average"
        }
    ],
    "recommended_action": "block",
    "challenge": null
}

// 异常检测（基于统计）
func detectAnomaly(userID string, field string, value float32, 
                    threshold float32) (bool, float32) {
    // 获取用户历史统计
    stats := getUserStats(userID, field)
    
    // 计算标准差
    zscore := (value - stats.Mean) / stats.StdDev
    
    return zscore > threshold, zscore
}
```

#### 数据表设计
```sql
-- 风控规则表
CREATE TABLE IF NOT EXISTS risk.rules (
    rule_id String,
    name String,
    rule_type String,
    enabled Boolean,
    risk_level String,
    action String,
    config JSON,
    created_at DateTime,
    updated_at DateTime
) ENGINE = MergeTree()
ORDER BY (rule_type, risk_level);

-- 风控评分日志
CREATE TABLE IF NOT EXISTS risk.evaluations (
    timestamp DateTime,
    user_id String,
    event_type String,
    risk_score Float32,
    risk_level String,
    triggered_rules Array(String),
    action_taken String,
    game_id String
) ENGINE = MergeTree()
ORDER BY (timestamp, user_id)
PARTITION BY toYYYYMM(timestamp);

-- 用户统计特征表（用于异常检测）
CREATE TABLE IF NOT EXISTS risk.user_features (
    user_id String,
    feature_date Date,
    field_name String,
    mean_value Float32,
    std_dev Float32,
    min_value Float32,
    max_value Float32,
    sample_count UInt32
) ENGINE = MergeTree()
ORDER BY (user_id, field_name, feature_date);

-- 风险事件日志
CREATE TABLE IF NOT EXISTS risk.risk_events (
    timestamp DateTime,
    user_id String,
    event_type String,
    description String,
    evidence JSON,
    status String,  -- "detected" | "confirmed" | "false_positive"
    investigation_note String,
    game_id String
) ENGINE = MergeTree()
ORDER BY (timestamp, event_type)
PARTITION BY toYYYYMM(timestamp);
```

#### 优先级和成本
```
投入成本：    高（8-10周）
实现难度：    高（涉及ML和统计）
ROI周期：    1-2周（快速保护收入）
优先级：     🔴 高（保护核心收入）
```

---

## 5. 国际化运营指标（Global Operations）

### 当前状态
✅ 已有：地理位置数据（country/region/city）

❌ 缺失：地域差异分析、本地化效果、跨区对比

### 缺失关键指标

#### 5.1 地域差异指标
```
指标名称                      定义                        分析维度    现状
────────────────────────────────────────────────────────────────────
地域DAU分布                   各地区DAU占比               国家/省市   ✅ 有基础
地域收入分布                  各地区收入占比               国家/省市   ✅ 有基础
地域ARPU排行                  各地区ARPU排序               国家排序    ❌ 无
地域ARPPU排行                 各地区付费用户ARPU           国家排序    ❌ 无
地域付费率差异                各地区付费率对比             国家/省市   ⚠️ 有但无对比
地域人均会话时长              各地区平均游戏时长           国家        ❌ 无
地域新用户质量                各地区新用户留存对比         国家        ❌ 无
```

#### 5.2 本地化效果指标
```
指标名称                      定义                        评估方法    现状
────────────────────────────────────────────────────────────────────
语言本地化接受度              本地化区域vs英文区域DAU对比   对照组分析  ❌ 无
文化适配度                    本地化内容消费热度            热度排名    ❌ 无
本地支付方式使用率            本地支付方式的占比            地区支付    ⚠️ 有但无分析
时区覆盖质量                  各时区活跃度均衡性            活跃时段    ❌ 无
本地活动参与率                地区限定活动的参与度          参与率      ❌ 无
```

#### 5.3 跨区域对比指标
```
指标名称                      定义                        基线        现状
────────────────────────────────────────────────────────────────────
版本首发地区选择              新版本灰度地区选择            最优地区    ❌ 无
功能偏好差异                  不同地区的功能使用差异        特征分析    ❌ 无
支付行为特征                  地区级支付习惯差异            特征分析    ❌ 无
游戏时间偏好                  地区玩家的高峰时段差异        时段分析    ❌ 无
转化率对比                    地区级新手转化率差异          对标管理    ❌ 无
```

### 为什么重要
- **收入优化**：精准识别高价值地区可提高ARPU 20-40%
- **本地化投资**：数据驱动的本地化决策提高ROI
- **全球扩张**：跨区对比数据指导新市场进入策略

### 实现方案

#### 方案E：地域分析平台
```go
// 地域指标聚合
type GeoMetrics struct {
    Country         string
    Region          string
    City            string
    Date            time.Time
    DAU             uint64
    WAU             uint64
    MAU             uint64
    Revenue         uint64      // 美元分（cents）
    PayerCount      uint64
    ARPU            float32
    ARPPU           float32
    PayRate         float32
    NewUsers        uint64
    D1Retention     float32
    D7Retention     float32
    D30Retention    float32
    AvgSessionTime  float32     // 分钟
    LocalizedLang   string      // 玩家使用的语言
}

// API：地域对比分析
GET /api/analytics/geo/comparison?dimensions=country,region&metrics=arpu,pay_rate&start=2025-01-01&end=2025-01-31
{
    "analysis_period": "2025-01-01 to 2025-01-31",
    "dimensions": ["country", "region"],
    "metrics": {
        "arpu": {
            "baseline": 2.45,  // 全球平均
            "regions": [
                {
                    "country": "US",
                    "region": null,
                    "arpu": 4.32,
                    "index": 176,      // vs baseline
                    "dau": 250000,
                    "revenue": 1080000
                },
                {
                    "country": "CN",
                    "region": "Beijing",
                    "arpu": 3.12,
                    "index": 127,
                    "dau": 180000,
                    "revenue": 561600
                },
                {
                    "country": "JP",
                    "region": "Tokyo",
                    "arpu": 5.67,
                    "index": 232,
                    "dau": 95000,
                    "revenue": 538650
                }
            ]
        },
        "pay_rate": {
            "baseline": 0.032,
            "regions": [
                {
                    "country": "US",
                    "pay_rate": 0.045,
                    "index": 141,
                    "payer_count": 11250
                }
                // ...
            ]
        }
    }
}

// API：本地化效果评估
GET /api/analytics/localization/impact?languages=zh_CN,en_US,ja_JP
{
    "comparison": {
        "metric": "dau_7d_growth",
        "by_language": [
            {
                "language": "zh_CN",
                "native_region": "CN",
                "dau": 450000,
                "growth_rate": 0.12,
                "quality_score": 0.92
            },
            {
                "language": "en_US",
                "native_region": "US",
                "dau": 350000,
                "growth_rate": 0.08,
                "quality_score": 0.85
            }
        ]
    },
    "localization_recommendations": [
        {
            "language": "pt_BR",
            "estimated_potential_dau": 150000,
            "priority": "high",
            "reason": "Large untapped market with high ARPU potential"
        }
    ]
}

// 时区活跃度热力图
GET /api/analytics/geo/timezone_heatmap?date=2025-01-15
{
    "date": "2025-01-15",
    "timezones": [
        {
            "timezone": "UTC-8",  // 太平洋
            "regions": ["US_West", "CA"],
            "hour_distribution": [
                {"hour": 0, "active_users": 5000, "peak": false},
                {"hour": 1, "active_users": 3000, "peak": false},
                {"hour": 8, "active_users": 45000, "peak": true},
                {"hour": 12, "active_users": 60000, "peak": true},
                {"hour": 20, "active_users": 50000, "peak": true}
                // ... 24小时数据
            ],
            "peak_hours": [8, 12, 20]
        }
        // ... 其他时区
    ]
}
```

#### 数据表设计
```sql
-- 地域指标汇总表
CREATE TABLE IF NOT EXISTS geo.daily_metrics (
    date Date,
    country String,
    region String,
    city String,
    game_id String,
    env String,
    dau UInt32,
    wau UInt32,
    mau UInt32,
    revenue_cents UInt64,
    payer_count UInt32,
    new_users UInt32,
    d1_retention Float32,
    d7_retention Float32,
    d30_retention Float32,
    avg_session_minutes Float32
) ENGINE = MergeTree()
ORDER BY (date, country, region)
PARTITION BY toYYYYMM(date);

-- 地域新用户质量表
CREATE TABLE IF NOT EXISTS geo.user_quality_by_region (
    cohort_date Date,
    country String,
    region String,
    new_user_count UInt32,
    d1_users UInt32,
    d7_users UInt32,
    d30_users UInt32,
    converted_to_payer UInt32,
    ltv_estimate Float32
) ENGINE = MergeTree()
ORDER BY (cohort_date, country, region);

-- 本地化效果评估表
CREATE TABLE IF NOT EXISTS geo.localization_metrics (
    date Date,
    game_id String,
    language String,
    country String,
    dau UInt32,
    engagement_rate Float32,
    content_completion_rate Float32,
    user_satisfaction Float32,
    quality_score Float32
) ENGINE = MergeTree()
ORDER BY (date, language, country);
```

#### 优先级和成本
```
投入成本：    中等（3-4周）
实现难度：    中等（数据聚合）
ROI周期：    2周（国际化决策优化）
优先级：     🟡 中等（全球业务必需）
```

---

## 6. AI/ML驱动的高级指标（Advanced Analytics）

### 当前状态
✅ 已有：基础事件采集、漏斗分析

❌ 缺失：推荐效果、个性化、智能运营

### 缺失关键指标

#### 6.1 推荐系统指标
```
指标名称                      定义                        基线        现状
────────────────────────────────────────────────────────────────────
推荐点击率(CTR)              用户点击推荐的概率          3-5%        ❌ 无
推荐转化率                   推荐成功的比例              10-15%      ❌ 无
推荐多样性                   推荐内容的丰富度(0-1)       >0.6        ❌ 无
新物品推荐比例               冷启动物品的推荐占比        5-10%       ❌ 无
推荐覆盖度                   能被推荐的物品占比          >50%        ❌ 无
长尾物品销售额               长尾物品通过推荐的收入      20-30%      ❌ 无
推荐新颖性                   推荐未见过的物品比例        >30%        ❌ 无
```

#### 6.2 个性化指标
```
指标名称                      定义                        目标        现状
────────────────────────────────────────────────────────────────────
个性化收入提升                vs非个性化的收入对比        +15-25%     ❌ 无
用户分群准确性               聚类模型的稳定性            >85%        ❌ 无
预测准确性                   用户行为预测的准确率        >75%        ❌ 无
流失预测召回率               能识别的流失用户比例        >70%        ❌ 无
LTV预测误差                  预测LTV vs实际LTV的偏差     <20%        ❌ 无
```

#### 6.3 智能运营指标
```
指标名称                      定义                        应用场景    现状
────────────────────────────────────────────────────────────────────
最优发送时间命中率           在用户活跃时发送的比例      push通知    ❌ 无
最优定价推荐                 动态定价的收入提升          付费商品    ❌ 无
客服智能分类准确率           AI自动分类工单的准确度      客服工单    ❌ 无
内容审核准确率               自动审核的正确率            UGC审核     ❌ 无
异常检测漏报率               逃脱检测的异常占比          风控检测    ❌ 无
```

### 为什么重要
- **收入增长**：个性化推荐可提高转化率15-30%
- **成本优化**：智能运营降低人工成本30-50%
- **用户体验**：AI驱动的个性化提高留存率10-20%

### 实现方案

#### 方案F：推荐与个性化平台
```go
// 推荐系统接口
type RecommendationRequest struct {
    UserID          string
    Context         string          // "store" | "lobby" | "reward"
    ItemCount       int
    ExcludeOwned    bool
    PersonalizationLevel string     // "none" | "low" | "high"
}

type RecommendationResponse struct {
    UserID          string
    RecommendedItems []RecommendedItem
    Explanation     string
}

type RecommendedItem struct {
    ItemID          string
    Score           float32         // 推荐分数 0-1
    Rank            int
    Algorithm       string          // "cf" | "cb" | "hybrid"
    Confidence      float32
    CTRExpectation  float32
}

// API：获取推荐
POST /api/recommendations
{
    "user_id": "user_123",
    "context": "store",
    "item_count": 10,
    "personalization_level": "high"
}

Response:
{
    "user_id": "user_123",
    "timestamp": "2025-01-20T10:30:00Z",
    "recommended_items": [
        {
            "item_id": "sword_001",
            "rank": 1,
            "score": 0.92,
            "algorithm": "hybrid",
            "confidence": 0.88,
            "ctr_expectation": 0.12,
            "reason": "Similar items purchased; popular in your region"
        },
        {
            "item_id": "armor_042",
            "rank": 2,
            "score": 0.87,
            "algorithm": "cf",
            "confidence": 0.85,
            "ctr_expectation": 0.09,
            "reason": "Purchased by similar players"
        }
    ],
    "diversity_score": 0.74,
    "cold_start_items": 1,
    "long_tail_items": 2
}

// 推荐效果评估
type RecommendationMetrics struct {
    Date              time.Time
    RecommenderID     string        // 推荐器算法ID
    ItemID            string
    Impressions       uint64        // 展示次数
    Clicks            uint64        // 点击次数
    Conversions       uint64        // 转化次数
    CTR               float32
    ConversionRate    float32
    Revenue           uint64
    RevenuePerClick   float32
}

// API：推荐系统性能
GET /api/analytics/recommendation/performance?recommender_id=hybrid_v2&start=2025-01-01&end=2025-01-31
{
    "recommender_id": "hybrid_v2",
    "period": "2025-01-01 to 2025-01-31",
    "aggregate_metrics": {
        "total_impressions": 1000000,
        "total_clicks": 45000,
        "total_conversions": 12000,
        "ctr": 0.045,
        "conversion_rate": 0.267,
        "total_revenue": 450000,
        "revenue_per_click": 10.0,
        "diversity_score": 0.68,
        "coverage": 0.72,
        "novelty_rate": 0.35
    },
    "comparison_with_baseline": {
        "ctr_lift": 0.15,        // +15%
        "conversion_lift": 0.22,  // +22%
        "revenue_lift": 0.28      // +28%
    },
    "top_recommended_items": [
        {
            "item_id": "sword_001",
            "total_impressions": 50000,
            "clicks": 2250,
            "conversions": 900,
            "ctr": 0.045,
            "conversion_rate": 0.4,
            "revenue": 45000
        }
    ],
    "user_segment_performance": [
        {
            "segment": "new_players",
            "ctr": 0.032,
            "conversion_rate": 0.15,
            "revenue_per_user": 5.2
        }
    ]
}

// 用户流失预测
type ChurnPrediction struct {
    UserID              string
    ChurnRisk           float32         // 0-1，风险分数
    ChurnRiskLevel      string          // "low" | "medium" | "high" | "critical"
    ExpectedChurnDate   time.Time
    RiskFactors         []string        // 流失主要因素
    RetentionActions    []Action        // 推荐的留存行动
}

type Action struct {
    Type                string          // "offer" | "notification" | "event"
    Description         string
    ExpectedEfficiency  float32
    Priority            string          // "high" | "medium" | "low"
}

// 个性化动态定价
type DynamicPricingRecommendation struct {
    UserID              string
    ProductID           string
    BasePrice           float32
    RecommendedPrice    float32
    PriceElasticity     float32         // 价格敏感度
    ConversionLift      float32         // 预期转化提升
    RevenueOptimal      bool            // 是否为收入最优价格
}
```

#### 数据表设计
```sql
-- 推荐记录表
CREATE TABLE IF NOT EXISTS ml.recommendation_logs (
    timestamp DateTime,
    user_id String,
    recommender_id String,
    context String,
    recommended_items Array(String),
    user_segment String,
    game_id String
) ENGINE = MergeTree()
ORDER BY (timestamp, user_id)
PARTITION BY toYYYYMM(timestamp);

-- 推荐效果评估表
CREATE TABLE IF NOT EXISTS ml.recommendation_metrics (
    date Date,
    recommender_id String,
    item_id String,
    impressions UInt32,
    clicks UInt32,
    conversions UInt32,
    revenue_cents UInt64,
    ctr Float32,
    conversion_rate Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (date, recommender_id)
PARTITION BY toYYYYMM(date);

-- 用户分群表
CREATE TABLE IF NOT EXISTS ml.user_segments (
    date Date,
    user_id String,
    segment_id String,
    segment_name String,
    characteristics JSON,
    game_id String
) ENGINE = MergeTree()
ORDER BY (date, user_id);

-- 流失预测结果
CREATE TABLE IF NOT EXISTS ml.churn_predictions (
    prediction_date Date,
    user_id String,
    churn_risk Float32,
    risk_factors Array(String),
    expected_churn_date Date,
    action_taken String,
    game_id String
) ENGINE = MergeTree()
ORDER BY (prediction_date, user_id)
PARTITION BY toYYYYMM(prediction_date);

-- 动态定价表
CREATE TABLE IF NOT EXISTS ml.dynamic_pricing (
    date Date,
    user_id String,
    product_id String,
    base_price Float32,
    recommended_price Float32,
    price_elasticity Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (date, product_id);
```

#### 优先级和成本
```
投入成本：    高（10-12周，包括ML模型）
实现难度：    高（需要ML/DS团队）
ROI周期：    1-2个月（显著收入提升）
优先级：     🔴 高（长期核心竞争力）
```

---

## 7. 商业智能指标（Business Intelligence）

### 当前状态
✅ 已有：基础KPI（DAU、收入、ARPU）

❌ 缺失：预测分析、趋势洞察、竞品对标

### 缺失关键指标

#### 7.1 预测分析指标
```
指标名称                      定义                        用途        现状
────────────────────────────────────────────────────────────────────
收入预测                      未来7/30/90天的收入预测      财务规划    ❌ 无
DAU趋势预测                   未来DAU走势预测              容量规划    ❌ 无
用户生命周期价值(LTV)预测     用户全生命周期预期收入      获客成本ROI ❌ 无
版本发布影响预测              新版本对主要指标的影响      版本规划    ❌ 无
留存率预测                    未来留存率变化趋势          用户健康度  ❌ 无
```

#### 7.2 趋势洞察指标
```
指标名称                      定义                        分析周期    现状
────────────────────────────────────────────────────────────────────
周期性识别                    数据的周期性规律            日/周/月    ❌ 无
异常检测                      数据异常变化告警            实时        ❌ 无
关键驱动因素分析              影响指标的主要因素          周度        ❌ 无
指标相关性分析                指标间的因果关系            -           ❌ 无
转折点识别                    运营策略生效的时间点        事件关联    ❌ 无
```

#### 7.3 竞品对标指标
```
指标名称                      定义                        对标对象    现状
────────────────────────────────────────────────────────────────────
App Store排名追踪             本游戏在排行榜的位置        日度        ❌ 无
市场份额估计                  在品类中的估计收入占比      月度        ❌ 无
主要竞品对比                  与TOP竞品的指标差异         竞品        ❌ 无
用户迁移分析                  竞品的用户向本游戏迁移      用户流动    ❌ 无
功能对标                      竞品新功能分析              新功能      ❌ 无
```

### 为什么重要
- **战略决策**：数据驱动的财务规划提高预算效率
- **竞争优势**：趋势洞察早期发现市场机会
- **风险管理**：异常检测及时发现问题

### 实现方案

#### 方案G：预测与BI平台
```go
// 时间序列预测
type TimeSeriesForecast struct {
    Metric          string
    CurrentValue    float32
    ForecastDays    int
    ForecastValues  []ForecastPoint
    Confidence      float32
    Model           string          // "arima" | "prophet" | "lstm"
    Accuracy        float32
}

type ForecastPoint struct {
    Date            time.Time
    PredictedValue  float32
    ConfidenceHigh  float32
    ConfidenceLow   float32
}

// API：收入预测
GET /api/analytics/forecast/revenue?days=30&game_id=game_001
{
    "metric": "revenue",
    "current_value": 450000,  // 今天收入（美元分）
    "forecast_days": 30,
    "forecast_values": [
        {
            "date": "2025-01-21",
            "predicted_value": 455000,
            "confidence_high": 480000,
            "confidence_low": 430000
        },
        {
            "date": "2025-01-22",
            "predicted_value": 460000,
            "confidence_high": 490000,
            "confidence_low": 430000
        }
        // ... 30天的数据
    ],
    "confidence": 0.85,
    "model": "prophet",
    "total_revenue_forecast_30d": 13500000,
    "growth_rate": 0.12
}

// LTV预测
type LTVForecast struct {
    CohortDate      time.Time
    NewUsersCount   uint64
    EstimatedLTV    float32         // 平均每用户生命周期价值
    LTVRange        [2]float32      // [低, 高]
    PayoutLTV       float32         // 已实现LTV
    FutureExpected  float32         // 预期未来LTV
    Confidence      float32
}

// API：LTV预测
GET /api/analytics/ltv/forecast?cohort_date=2024-12-01
{
    "cohort_date": "2024-12-01",
    "new_users_count": 50000,
    "estimated_ltv": 4.50,        // 平均每用户生命周期价值
    "ltv_range": [3.20, 6.50],
    "payout_ltv": 2.30,           // 已实现
    "future_expected": 2.20,       // 预期未来
    "confidence": 0.78,
    "ltv_by_segment": [
        {
            "segment": "whale",
            "estimated_ltv": 45.0,
            "count": 500,
            "contribution": 0.45
        },
        {
            "segment": "regular_payer",
            "estimated_ltv": 8.5,
            "count": 5000,
            "contribution": 0.35
        }
    ]
}

// 异常检测
type AnomalyDetection struct {
    Metric          string
    CurrentValue    float32
    ExpectedValue   float32
    Deviation       float32         // 百分比偏差
    AnomalyScore    float32         // 0-1，异常程度
    IsAnomaly       bool
    PossibleCauses  []string
    RecommendedAction string
}

// API：异常告警
GET /api/analytics/anomalies?severity=high
{
    "detection_time": "2025-01-20T14:30:00Z",
    "anomalies": [
        {
            "metric": "payment_success_rate",
            "current_value": 0.75,
            "expected_value": 0.95,
            "deviation": -0.21,
            "anomaly_score": 0.92,
            "is_anomaly": true,
            "severity": "critical",
            "possible_causes": [
                "Payment gateway timeout",
                "Traffic spike",
                "API rate limit exceeded"
            ],
            "recommended_action": "Check payment gateway status and error logs"
        }
    ]
}

// 关键驱动因素分析
type KeyDriverAnalysis struct {
    TargetMetric    string
    AnalysisDate    time.Time
    Drivers         []Driver
}

type Driver struct {
    FactorName      string
    CorrelationWith float32         // 与目标指标的相关系数
    Elasticity      float32         // 单位变化的影响
    ContributionPct float32         // 对目标变化的贡献度
    ConfidenceLevel float32
}

// API：驱动因素分析
GET /api/analytics/drivers/analysis?metric=revenue&date=2025-01-20
{
    "target_metric": "revenue",
    "analysis_date": "2025-01-20",
    "drivers": [
        {
            "factor_name": "user_acquisition",
            "correlation": 0.87,
            "elasticity": 0.45,      // 新增用户每增加1%，收入增加0.45%
            "contribution_pct": 0.35,
            "confidence_level": 0.92
        },
        {
            "factor_name": "payment_success_rate",
            "correlation": 0.81,
            "elasticity": 0.62,
            "contribution_pct": 0.30,
            "confidence_level": 0.89
        },
        {
            "factor_name": "arpu",
            "correlation": 0.95,
            "elasticity": 0.98,
            "contribution_pct": 0.25,
            "confidence_level": 0.94
        }
    ]
}
```

#### 数据表设计
```sql
-- 预测结果表
CREATE TABLE IF NOT EXISTS bi.forecasts (
    forecast_date DateTime,
    metric_name String,
    forecast_horizon Int32,  -- 天数
    predicted_value Float32,
    confidence_high Float32,
    confidence_low Float32,
    model_name String,
    accuracy Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (forecast_date, metric_name)
PARTITION BY toYYYYMM(forecast_date);

-- LTV预测表
CREATE TABLE IF NOT EXISTS bi.ltv_cohorts (
    cohort_date Date,
    segment String,
    new_user_count UInt64,
    estimated_ltv Float32,
    payout_ltv Float32,
    future_expected Float32,
    confidence Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (cohort_date, segment);

-- 异常检测日志
CREATE TABLE IF NOT EXISTS bi.anomalies (
    detection_time DateTime,
    metric_name String,
    current_value Float32,
    expected_value Float32,
    anomaly_score Float32,
    severity String,  -- "info" | "warning" | "critical"
    possible_causes Array(String),
    game_id String
) ENGINE = MergeTree()
ORDER BY (detection_time, metric_name)
PARTITION BY toYYYYMM(detection_time);

-- 关键驱动因素
CREATE TABLE IF NOT EXISTS bi.key_drivers (
    analysis_date Date,
    target_metric String,
    driver_name String,
    correlation Float32,
    elasticity Float32,
    contribution_pct Float32,
    confidence Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (analysis_date, target_metric, contribution_pct DESC);
```

#### 优先级和成本
```
投入成本：    高（8-10周）
实现难度：    高（需要统计学/ML）
ROI周期：    1个月（战略决策优化）
优先级：     🔴 高（管理决策核心）
```

---

## 8. 用户体验指标（User Experience）

### 当前状态
✅ 已有：基础留存数据

❌ 缺失：UI/UX效果、操作流畅度、细分满意度

### 缺失关键指标

#### 8.1 UI/UX效果指标
```
指标名称                      定义                        目标        现状
────────────────────────────────────────────────────────────────────
界面元素点击率                特定UI元素的交互率          >20%        ❌ 无
页面加载时间                  从点击到页面可交互的时间    <1s         ❌ 无
页面跳出率                    进入即离开的用户占比         <10%        ❌ 无
操作完成率                    用户完成特定操作的比例      >80%        ❌ 无
错误点击率                    无意义点击/总点击           <5%         ❌ 无
手势识别准确率                移动端手势识别的准确度      >95%        ❌ 无
```

#### 8.2 操作流畅度指标
```
指标名称                      定义                        目标        现状
────────────────────────────────────────────────────────────────────
帧率(FPS)分布                 游戏运行FPS统计             >30FPS avg   ❌ 无
卡顿事件频率                  每小时卡顿次数              <1次        ❌ 无
内存使用情况                  游戏进程内存占用            <500MB      ❌ 无
GPU使用率                     显卡利用率分布              <80%        ❌ 无
网络延迟(Ping)                往返延迟                    <100ms      ❌ 无
丢包率                        网络丢包占比                <0.5%       ❌ 无
连接稳定性                    连接断开重连频率            <1次/小时   ❌ 无
```

#### 8.3 用户满意度细分指标
```
指标名称                      定义                        评分方法    现状
────────────────────────────────────────────────────────────────────
游戏乐趣评分                  用户对游戏乐趣的评价        1-5星       ❌ 无
难度评价                      用户觉得游戏难度如何        1-5星       ❌ 无
画面质量评价                  用户对画质的评价            1-5星       ❌ 无
音效满意度                    用户对音乐音效的评价        1-5星       ❌ 无
创意度评价                    用户觉得游戏创意程度        1-5星       ❌ 无
性价比评价                    用户对付费内容价格的评价    1-5星       ❌ 无
推荐意愿                      用户推荐给他人的意愿        0-10分NPS   ❌ 无
```

### 为什么重要
- **留存提升**：优化UX可提高D7留存10-20%
- **口碑传播**：高满意度用户的自然传播效果显著
- **减少流失**：识别卡顿等问题及时修复

### 实现方案

#### 方案H：UX监控与反馈平台
```go
// 用户体验事件追踪
type UXEvent struct {
    EventID         string
    UserID          string
    EventType       string      // "ui_click" | "page_load" | "gesture" | "error"
    UIElement       string      // 被交互的UI元素ID
    Timestamp       time.Time
    Duration        int32       // 毫秒
    Success         bool
    ErrorCode       string      // 如果失败
}

// 性能监控数据
type PerformanceMetrics struct {
    Timestamp       time.Time
    UserID          string
    DeviceType      string      // "mobile" | "tablet" | "pc"
    OSVersion       string
    FPS             float32
    FrameDropRate   float32     // 0-1
    MemoryUsageMB   float32
    GPUUsagePercent float32
    NetworkLatency  uint32      // 毫秒
    PacketLoss      float32     // 0-1
    BatteryPercent  float32     // 移动设备
}

// 用户反馈
type UserFeedback struct {
    FeedbackID      string
    UserID          string
    FeedbackType    string      // "bug_report" | "feature_request" | "satisfaction"
    Category        string      // "gameplay" | "ui" | "graphics" | "network"
    Rating          int32       // 1-5
    Message         string
    Attachment      []byte
    Timestamp       time.Time
    Status          string      // "open" | "acknowledged" | "resolved"
}

// API：收集UX事件
POST /api/analytics/ux/events
{
    "events": [
        {
            "event_id": "evt_001",
            "event_type": "ui_click",
            "ui_element": "shop_buy_button",
            "timestamp": "2025-01-20T10:30:00Z",
            "duration": 150,
            "success": true
        },
        {
            "event_id": "evt_002",
            "event_type": "page_load",
            "ui_element": "battle_screen",
            "timestamp": "2025-01-20T10:31:00Z",
            "duration": 2500,
            "success": true
        }
    ]
}

// API：收集性能指标
POST /api/analytics/performance/metrics
{
    "metrics": {
        "timestamp": "2025-01-20T10:30:00Z",
        "fps": 58.5,
        "frame_drop_rate": 0.02,
        "memory_usage_mb": 245.3,
        "gpu_usage_percent": 65.0,
        "network_latency": 45,
        "packet_loss": 0.001,
        "battery_percent": 72.0
    }
}

// API：获取UX健康度报告
GET /api/analytics/ux/health?days=7
{
    "period": "Last 7 days",
    "overall_score": 8.2,  // 0-10
    "metrics": {
        "page_load_time_avg": 1.2,  // 秒
        "page_load_time_p95": 3.5,
        "bounce_rate": 0.08,
        "operation_completion_rate": 0.92,
        "fps_avg": 55.0,
        "fps_p10": 35.0,
        "frame_drop_rate": 0.03,
        "network_latency_avg": 52,
        "packet_loss_avg": 0.002,
        "crash_rate_per_session": 0.001
    },
    "ui_elements_performance": [
        {
            "element_id": "shop_buy_button",
            "click_count": 50000,
            "success_rate": 0.98,
            "avg_load_time": 0.5
        }
    ],
    "device_breakdown": [
        {
            "device_type": "mobile",
            "fps_avg": 45.0,
            "crash_rate": 0.002,
            "satisfaction": 7.8
        }
    ],
    "critical_issues": [
        {
            "issue": "High frame drops on iOS 13",
            "affected_users_pct": 0.05,
            "severity": "high",
            "recommendation": "Optimize graphics pipeline"
        }
    ]
}

// API：用户反馈收集
POST /api/feedback
{
    "feedback_type": "bug_report",
    "category": "gameplay",
    "rating": 2,
    "message": "游戏在BOSS战时经常卡顿",
    "attachments": ["screenshot.png", "video.mp4"]
}

Response:
{
    "feedback_id": "fb_12345",
    "status": "acknowledged",
    "message": "感谢您的反馈，我们已将其转发给技术团队"
}
```

#### 数据表设计
```sql
-- UX事件表
CREATE TABLE IF NOT EXISTS ux.events (
    timestamp DateTime,
    user_id String,
    event_id String,
    event_type String,
    ui_element String,
    duration_ms UInt32,
    success Boolean,
    error_code String,
    game_id String
) ENGINE = MergeTree()
ORDER BY (timestamp, user_id)
PARTITION BY toYYYYMM(timestamp);

-- 性能指标表
CREATE TABLE IF NOT EXISTS ux.performance_metrics (
    timestamp DateTime,
    user_id String,
    device_type String,
    os_version String,
    fps Float32,
    frame_drop_rate Float32,
    memory_usage_mb Float32,
    gpu_usage_percent Float32,
    network_latency_ms UInt32,
    packet_loss Float32,
    battery_percent Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (timestamp, user_id, device_type)
PARTITION BY toYYYYMM(timestamp);

-- UI元素性能表
CREATE TABLE IF NOT EXISTS ux.ui_element_performance (
    date Date,
    ui_element String,
    click_count UInt32,
    success_count UInt32,
    avg_load_time_ms Float32,
    p95_load_time_ms Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (date, ui_element);

-- 用户反馈表
CREATE TABLE IF NOT EXISTS ux.user_feedback (
    feedback_id String,
    user_id String,
    feedback_type String,
    category String,
    rating Int32,
    message String,
    timestamp DateTime,
    status String,
    game_id String
) ENGINE = MergeTree()
ORDER BY (timestamp, category)
PARTITION BY toYYYYMM(timestamp);
```

#### 优先级和成本
```
投入成本：    中等（4-5周）
实现难度：    低至中等（相对直接）
ROI周期：    2周（快速识别问题）
优先级：     🟡 中等（用户留存关键）
```

---

## 9. 生态运营指标（Ecosystem Operations）

### 当前状态
❌ 缺失：合作伙伴数据、渠道效果、生态健康度

### 缺失关键指标

#### 9.1 合作伙伴效果指标
```
指标名称                      定义                        评估周期    现状
────────────────────────────────────────────────────────────────────
合作伙伴付费转化率            通过渠道获得的付费用户占比  周度        ❌ 无
合作伙伴用户留存对比          各渠道用户的D7/D30对比      日度        ❌ 无
合作伙伴用户LTV对比           各渠道用户的生命周期价值    月度        ❌ 无
合作伙伴成本效率(CPI)         单个用户获取成本            -           ❌ 无
合作伙伴ROI                   投入回报比                  月度        ❌ 无
合作伙伴活跃度指数            合作伙伴的推广活跃程度      周度        ❌ 无
```

#### 9.2 渠道效果指标
```
指标名称                      定义                        示例        现状
────────────────────────────────────────────────────────────────────
渠道用户获取量                各渠道的新用户数            日度统计    ⚠️ 有但无分析
渠道转化漏斗                  各渠道的转化步骤完成率      -           ❌ 无
渠道用户质量评分              渠道用户与平台平均的对比    评分体系    ❌ 无
渠道竞争度                    相同渠道的竞品数量           动态        ❌ 无
渠道市场饱和度                该渠道的供给过剩程度         评估        ❌ 无
```

#### 9.3 生态健康度指标
```
指标名称                      定义                        告警值      现状
────────────────────────────────────────────────────────────────────
活跃合作伙伴数                正在进行推广的合作伙伴数     <3个        ❌ 无
平均合作伙伴在线时长          合作伙伴平均服务时长        <10小时     ❌ 无
投诉解决率                    合作伙伴投诉的解决占比      <90%        ❌ 无
续约率                        与合作伙伴继续合作的比例    <80%        ❌ 无
合作伙伴满意度                对合作体验的评价            <3.5星      ❌ 无
生态多元化指数                渠道的多元程度(0-1)         <0.6        ❌ 无
```

### 为什么重要
- **获客成本控制**：精细的渠道对标控制CPI
- **生态稳定性**：多元化的渠道降低风险
- **长期增长**：生态伙伴是可持续增长引擎

### 实现方案

#### 方案I：生态运营管理平台
```go
// 合作伙伴数据
type Partner struct {
    ID              string
    Name            string
    Type            string      // "publisher" | "ad_network" | "influencer"
    Country         string
    Contact         string
    ContractStart   time.Time
    ContractEnd     time.Time
    Status          string      // "active" | "inactive" | "paused"
}

// 渠道性能数据
type ChannelPerformance struct {
    Date            time.Time
    ChannelID       string
    PartnerID       string
    Installs        uint64      // 新增安装
    NewUsers        uint64      // 注册用户
    D1Retention     float32
    D7Retention     float32
    D30Retention    float32
    PayingUsers     uint64
    Revenue         uint64
    CPI             float32     // Cost Per Install
    LTVCPI          float32     // LTV/CPI
    PartnerCost     uint64      // 合作伙伴成本
}

// API：合作伙伴管理
GET /api/partners/{partner_id}/performance?start=2025-01-01&end=2025-01-31
{
    "partner_id": "partner_001",
    "partner_name": "TopPublisher",
    "contract_period": "2025-01-01 to 2025-03-31",
    "performance_summary": {
        "total_installs": 500000,
        "total_new_users": 450000,
        "avg_d7_retention": 0.38,
        "avg_d30_retention": 0.15,
        "total_payers": 15000,
        "total_revenue": 1350000,
        "partner_cost": 250000,
        "roi": 4.4,
        "cpi": 0.56
    },
    "daily_performance": [
        {
            "date": "2025-01-01",
            "installs": 10000,
            "new_users": 8500,
            "payers": 200,
            "revenue": 15000,
            "cpi": 0.58
        }
    ],
    "health_metrics": {
        "partnership_score": 8.5,
        "activity_level": "high",
        "communication_score": 9.0,
        "issue_resolution_score": 8.0
    },
    "recommendations": [
        "Increase budget allocation by 20% due to strong performance",
        "Consider extending partnership for next quarter"
    ]
}

// 渠道对标分析
type ChannelBenchmarking struct {
    Metric          string
    Channels        []ChannelMetric
    BestPerformer   string
    AvgPerformance  float32
    YourPerformance float32
    Index           float32  // vs平均的指数
}

// API：渠道对标
GET /api/analytics/channels/benchmarking?metric=d7_retention
{
    "metric": "d7_retention",
    "period": "Last 30 days",
    "channels": [
        {
            "channel_id": "organic",
            "channel_name": "App Store自然流量",
            "value": 0.42,
            "sample_size": 100000
        },
        {
            "channel_id": "partner_001",
            "channel_name": "TopPublisher",
            "value": 0.38,
            "sample_size": 15000
        },
        {
            "channel_id": "facebook",
            "channel_name": "Facebook广告",
            "value": 0.35,
            "sample_size": 50000
        }
    ],
    "best_performer": "organic",
    "avg_performance": 0.38,
    "your_performance": 0.38,
    "index": 100,
    "insights": [
        "自然流量仍是最优质渠道，需加强品牌声誉管理",
        "付费渠道与平均水平持平，建议优化创意和定位"
    ]
}

// 生态健康度评估
type EcosystemHealth struct {
    HealthScore     float32  // 0-100
    ActivePartners  int32
    DiversityIndex  float32  // 0-1
    PartnerSatisfaction float32  // 0-5
    RenewalRate     float32
    IssueResolution float32
}

// API：生态健康度
GET /api/analytics/ecosystem/health
{
    "health_score": 76.5,
    "status": "good",
    "metrics": {
        "active_partners": 12,
        "avg_partner_satisfaction": 4.2,
        "renewal_rate": 0.92,
        "diversity_index": 0.73,  // 多元化程度，>0.7为优
        "issue_resolution_rate": 0.94,
        "avg_response_time_hours": 4.2
    },
    "top_partners": [
        {
            "partner_id": "partner_001",
            "name": "TopPublisher",
            "contribution_pct": 0.25,
            "satisfaction": 4.5,
            "status": "excellent"
        }
    ],
    "at_risk_partners": [
        {
            "partner_id": "partner_005",
            "name": "InactivePublisher",
            "issue": "No campaigns in last 30 days",
            "recommended_action": "Schedule meeting to re-engage"
        }
    ],
    "recommendations": [
        "Diversify partnerships to reduce concentration risk",
        "Establish SLA with all partners to ensure accountability"
    ]
}
```

#### 数据表设计
```sql
-- 合作伙伴表
CREATE TABLE IF NOT EXISTS ecosystem.partners (
    partner_id String,
    name String,
    type String,
    country String,
    contract_start Date,
    contract_end Date,
    status String,
    contact_info JSON,
    created_at DateTime
) ENGINE = MergeTree()
ORDER BY (partner_id, contract_start);

-- 渠道性能表
CREATE TABLE IF NOT EXISTS ecosystem.channel_performance (
    date Date,
    channel_id String,
    partner_id String,
    game_id String,
    installs UInt32,
    new_users UInt32,
    d1_retention Float32,
    d7_retention Float32,
    d30_retention Float32,
    paying_users UInt32,
    revenue_cents UInt64,
    partner_cost_cents UInt64,
    cpi Float32,
    roi Float32
) ENGINE = MergeTree()
ORDER BY (date, channel_id)
PARTITION BY toYYYYMM(date);

-- 合作伙伴评估表
CREATE TABLE IF NOT EXISTS ecosystem.partner_assessments (
    assessment_date Date,
    partner_id String,
    overall_score Float32,
    communication_score Float32,
    activity_level String,
    issue_count Int32,
    resolved_issues Int32,
    satisfaction_score Float32,
    notes String,
    game_id String
) ENGINE = MergeTree()
ORDER BY (assessment_date, partner_id);
```

#### 优先级和成本
```
投入成本：    中等（3-4周）
实现难度：    中等（数据聚合）
ROI周期：    2-3周（优化渠道策略）
优先级：     🟡 中等（取决于业务模式）
```

---

## 10. 可持续发展指标（Sustainability & Innovation）

### 当前状态
❌ 缺失：长期价值、品牌健康、创新指数

### 缺失关键指标

#### 10.1 长期价值指标
```
指标名称                      定义                        评估周期    现状
────────────────────────────────────────────────────────────────────
单位经济学(Unit Economics)    ARPU vs CAC vs LTV           月度        ❌ 无
生命周期利润率                LTV - CAC的利润率            月度        ❌ 无
回本周期                      收回获客成本的天数           获客后      ❌ 无
新用户价值分布                按价值分层的用户占比         分层分析    ❌ 无
用户价值集中度                少数用户贡献的收入占比       帕累托分析  ❌ 无
LTV增长率                     用户生命周期价值的增长       月度        ❌ 无
```

#### 10.2 品牌健康指标
```
指标名称                      定义                        数据来源    现状
────────────────────────────────────────────────────────────────────
品牌认知度                    了解该品牌的人占比           舆情分析    ❌ 无
品牌好感度                    正面评价的占比              评论分析    ❌ 无
净推荐值(NPS)                 用户推荐意愿(0-10)          用户调研    ❌ 无
自然提及度                    社交媒体中品牌自然提及      舆情数据    ❌ 无
负面舆情监控                  负面评论和投诉的比例        舆情系统    ❌ 无
口碑传播指数                  用户自然推荐的力度          行为数据    ❌ 无
```

#### 10.3 创新指数指标
```
指标名称                      定义                        评估方法    现状
────────────────────────────────────────────────────────────────────
创新活跃度                    新功能发布频率和质量         月度统计    ❌ 无
玩法多样性                    游戏机制类别的丰富程度       特征分析    ❌ 无
内容更新频率                  新内容的发布速度            内容日志    ❌ 无
技术栈现代化程度              采用新技术的广度             架构分析    ❌ 无
社区参与度                    玩家参与创意和反馈的程度    社区数据    ❌ 无
```

### 为什么重要
- **业务可持续**：理解单位经济学确保商业模式健康
- **品牌价值**：品牌健康度决定用户获取成本
- **竞争优势**：持续创新是长期领先的保证

### 实现方案

#### 方案J：可持续发展分析系统
```go
// 单位经济学模型
type UnitEconomics struct {
    CohortDate      time.Time
    CohortSize      uint64
    TotalCAC        float32     // 单用户获客成本
    AverageLTV      float32
    Profitability   float32     // (LTV - CAC) / CAC
    PaybackDays     int32       // 回本周期
    MonthlyMetrics  []MonthlyUE
}

type MonthlyUE struct {
    Month           int32       // 注册后第N个月
    CumulativeLTV   float32
    MarginalProfit  float32
}

// API：单位经济学分析
GET /api/analytics/unit_economics/cohort?cohort_date=2024-12-01
{
    "cohort_date": "2024-12-01",
    "cohort_size": 50000,
    "total_cac": 1.20,
    "average_ltv": 5.40,
    "ltv_cac_ratio": 4.5,
    "profitability_pct": 350.0,
    "payback_days": 12,
    "monthly_progression": [
        {
            "month": 1,
            "cumulative_ltv": 0.80,
            "marginal_profit": 0.80,
            "roi": 66.7
        },
        {
            "month": 2,
            "cumulative_ltv": 2.15,
            "marginal_profit": 1.35,
            "roi": 179.2
        },
        {
            "month": 3,
            "cumulative_ltv": 3.60,
            "marginal_profit": 1.45,
            "roi": 300.0
        }
    ],
    "sustainability_score": 8.5,  // 基于LTV/CAC比率
    "recommendations": [
        "Current cohort has excellent unit economics",
        "Consider increasing CAC to acquire more volume"
    ]
}

// 品牌健康评分卡
type BrandHealthCard struct {
    Date            time.Time
    OverallScore    float32     // 0-100
    Awareness       float32     // 0-100
    Sentiment       float32     // -100 to 100
    NPS             float32     // 0-100
    TrendingScore   float32     // 话题热度
}

// API：品牌健康度
GET /api/analytics/brand/health
{
    "period": "Last 30 days",
    "overall_health_score": 76.5,
    "metrics": {
        "awareness": 72.0,
        "sentiment": 65.3,  // -100到100，正数为正面
        "nps": 42.0,
        "trending_score": 78.5,
        "negative_mention_ratio": 0.12
    },
    "sentiment_breakdown": {
        "positive": 0.68,
        "neutral": 0.20,
        "negative": 0.12
    },
    "top_positive_mentions": [
        "Amazing gameplay",
        "Great graphics",
        "Fun community"
    ],
    "top_negative_mentions": [
        "Frequent crashes on older devices",
        "Pay-to-win mechanics",
        "Slow customer support"
    ],
    "trend": "stable",
    "recommendations": [
        "Address crash issues mentioned in 150+ reviews",
        "Promote positive community initiatives to boost sentiment"
    ]
}

// 创新指数
type InnovationIndex struct {
    Date            time.Time
    OverallScore    float32
    FeatureVelocity float32     // 功能发布速度
    PlayMechanics   float32     // 玩法多样性
    ContentFreshness float32    // 内容新鲜度
    TechAdoption    float32     // 技术采纳度
}

// API：创新指数
GET /api/analytics/innovation/index?months=6
{
    "analysis_period": "Last 6 months",
    "overall_innovation_score": 72.5,
    "trend": "upward",
    "components": {
        "feature_velocity": {
            "score": 75.0,
            "monthly_avg": 8.5,  // 每月发布的新功能数
            "major_features": 3,
            "trend": "stable"
        },
        "play_mechanics_diversity": {
            "score": 70.0,
            "unique_mechanics": 25,
            "new_mechanics_this_period": 5,
            "player_exploration_score": 0.68
        },
        "content_freshness": {
            "score": 73.0,
            "content_update_frequency": 3,  // 周/月
            "repeat_content_ratio": 0.15,
            "player_engagement_with_new_content": 0.45
        },
        "technology_adoption": {
            "score": 72.0,
            "new_tech_implementation": 4,
            "tech_modernization_progress": 0.65
        }
    },
    "community_contribution": {
        "ugc_creation_rate": 0.08,
        "ugc_quality_avg": 3.8,
        "player_suggestion_adoption_rate": 0.35
    },
    "competitive_analysis": {
        "innovation_rank": 2,  // 品类排名
        "vs_top_competitor": -5,  // 相对领先者的差距
        "recommendation": "Increase focus on novel mechanics to close gap with #1"
    }
}
```

#### 数据表设计
```sql
-- 单位经济学表
CREATE TABLE IF NOT EXISTS sustainability.unit_economics (
    cohort_date Date,
    cohort_size UInt64,
    total_cac Float32,
    average_ltv Float32,
    payback_days Int32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (cohort_date);

-- 品牌健康评分表
CREATE TABLE IF NOT EXISTS sustainability.brand_health (
    date Date,
    overall_score Float32,
    awareness Float32,
    sentiment Float32,
    nps Float32,
    trending_score Float32,
    positive_mentions UInt32,
    negative_mentions UInt32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (date)
PARTITION BY toYYYYMM(date);

-- 创新指数表
CREATE TABLE IF NOT EXISTS sustainability.innovation_index (
    date Date,
    overall_score Float32,
    feature_velocity Float32,
    play_mechanics_score Float32,
    content_freshness Float32,
    tech_adoption Float32,
    new_features_count UInt32,
    ugc_creation_rate Float32,
    game_id String
) ENGINE = MergeTree()
ORDER BY (date)
PARTITION BY toYYYYMM(date);
```

#### 优先级和成本
```
投入成本：    高（6-8周）
实现难度：    高（涉及舆情/社区数据）
ROI周期：    1-2个月（长期战略指导）
优先级：     🟡 中等（长期战略级）
```

---

## 总结：优先级矩阵与实现路线图

### 优先级矩阵（按商业价值 × 实现难度）

```
高价值
  ┌─────────────────────────────────────┐
  │  高优先（快赢）      高优先（长期）│
  │  • 技术运营(1)      • 风控安全(4)   │
  │  • 产品迭代(2)      • AI/ML驱动(6)  │
  │  • UX体验(8)        • BI分析(7)     │
  │                     • 可持续(10)    │
  │                                      │
  │  中优先（选择性）    低优先(不推荐)│
  │  • 内容消费(3)      • 国际化(5)*    │
  │  • 生态运营(9)      (*按市场需求)  │
低价值
  │低                  高
  └────────────────────→ 难度
```

### 12个月实现路线图

#### 第1-2月（基础设施阶段）
优先完成：
1. **技术运营指标** (1) - Prometheus监控集成
2. **UX体验基础** (8) - 性能监控、错误追踪

成本：2-3名工程师，完成度：50%

#### 第3-4月（核心分析）
优先完成：
3. **产品迭代指标** (2) - A/B测试框架
4. **内容消费分析** (3) - 内容热度系统

成本：3名工程师 + 1名数据分析师，完成度：75%

#### 第5-6月（风控与国际化）
优先完成：
5. **风控安全体系** (4) - 多层风控系统
6. **国际化分析** (5) - 地域对比平台（如适用）

成本：3名工程师 + 1名数据科学家，完成度：70%

#### 第7-9月（AI/ML能力）
优先完成：
7. **AI/ML驱动** (6) - 推荐与个性化系统
8. **BI分析** (7) - 预测与分析平台

成本：2名机器学习工程师 + 1名数据科学家，完成度：80%

#### 第10-12月（生态与可持续）
优先完成：
9. **生态运营** (9) - 合作伙伴管理平台
10. **可持续发展** (10) - 长期价值分析系统

成本：2名工程师 + 1名数据分析师，完成度：75%

### 总体投入估算
- **人员：** 约12-15人月分配（可并行）
- **时间：** 12个月分阶段实现
- **预期收益：** 20-40%的商业指标提升

---

## 实现框架：推荐技术栈

### 数据基础设施
```
数据收集    →  数据处理   →  数据存储   →  分析&可视化
─────────────────────────────────────────────────────
SDK/事件    Kafka/MQ   ClickHouse   Metabase/Grafana
埋点系统    流处理      实时分析     自定义仪表板
```

### 推荐服务选型
- **消息队列**：Kafka（高吞吐）或 Redis Pub/Sub（低延迟）
- **OLAP数据库**：ClickHouse（已有）
- **可视化**：Grafana（运营）+ Metabase（分析）
- **ML框架**：TensorFlow 或 PyTorch（推荐系统）
- **实验平台**：自建或 LaunchDarkly

### 现有系统改进点
基于 Croupier 的现状优化：
1. 扩展 `analytics.events` 表的 schema，支持更多维度
2. 在 `analytics_routes.go` 中添加新的API端点
3. 开发专用的分析工作台（AnalyticsWorker 扩展）
4. 建立元数据管理系统（指标字典、数据血缘）

---

## 风险与缓解措施

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|---------|
| 数据质量问题 | 高 | 决策失效 | 建立数据QA流程，定期审核 |
| 系统性能瓶颈 | 中 | 查询缓慢 | 进行容量规划，预留扩展空间 |
| 隐私合规风险 | 中 | 法律风险 | GDPR合规审计，数据脱敏 |
| 团队技能不足 | 中 | 延期交付 | 进行培训，引入外部专家 |

---

## 成功关键指标(KPI)

系统建成后的评估指标：
- 决策周期缩短：从手工分析的3-5天降至自动化的1小时内
- 数据准确性：分析数据与实际业务指标的偏差<5%
- 用户采用率：80%以上的运营/产品团队日常使用
- 商业影响：在实现后的3个月内带来可量化的收益

