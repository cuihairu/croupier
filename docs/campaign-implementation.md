# 活动系统核心实现详解

本文档详细描述活动系统三大核心组件的具体实现。

## 目录

1. [触发器引擎 (Trigger Engine)](#1-触发器引擎-trigger-engine)
2. [条件评估器 (Condition Evaluator)](#2-条件评估器-condition-evaluator)
3. [动作执行器 (Action Executor)](#3-动作执行器-action-executor)

---

## 1. 触发器引擎 (Trigger Engine)

### 1.1 核心接口定义

```go
// internal/campaign/engine/matcher.go

package engine

import (
    "context"
    "time"
)

// Event 统一事件结构
type Event struct {
    // 基础字段
    EventID    string                 `json:"event_id"`
    EventType  string                 `json:"event_type"`
    PlayerID   string                 `json:"player_id"`
    GameID     string                 `json:"game_id"`
    Env        string                 `json:"env"`
    ServerID   string                 `json:"server_id"`
    SessionID  string                 `json:"session_id"`

    // 时间
    EventTime  time.Time              `json:"event_time"`
    ReceiveTime time.Time             `json:"receive_time"`

    // 上下文
    Platform   string                 `json:"platform"`    // ios/android/web/pc
    Channel    string                 `json:"channel"`
    Country    string                 `json:"country"`
    Region     string                 `json:"region"`
    DeviceID   string                 `json:"device_id"`
    AppVersion string                 `json:"app_version"`
    IP         string                 `json:"ip"`

    // 事件属性 (动态)
    Properties map[string]interface{} `json:"properties"`

    // 扩展
    Context    map[string]interface{} `json:"context"`
}

// Matcher 匹配器接口
type Matcher interface {
    // Match 匹配事件到活动
    Match(ctx context.Context, event Event) ([]MatchResult, error)

    // MatchByID 匹配特定活动
    MatchByID(ctx context.Context, campaignID string, event Event) (*MatchResult, error)
}

// MatchResult 匹配结果
type MatchResult struct {
    CampaignID   string
    TemplateID   string
    Priority     int32
    MatchReason  []string  // 匹配原因（用于调试）
    MatchedAt    time.Time

    // 缓存的匹配数据，供后续使用
    PlayerInfo   *PlayerSnapshot
    ContextData  map[string]interface{}
}

// PlayerSnapshot 玩家快照
type PlayerSnapshot struct {
    PlayerID       string
    Level          int32
    VIPLevel       int32
    RegisterTime   time.Time
    TotalRecharge  int64
    LastLoginTime  time.Time
    OnlineDuration int64  // 秒

    // 扩展属性
    Attributes     map[string]interface{}
}
```

### 1.2 触发器匹配器实现

```go
// internal/campaign/engine/trigger_matcher.go

package engine

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/cache"
    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/service/playersvc"
)

// TriggerMatcher 触发器匹配器
type TriggerMatcher struct {
    cache        cache.CampaignCache
    playerSvc    playersvc.PlayerService
    freqLimiter  FrequencyLimiter
    logger       *slog.Logger

    // 配置
    maxConcurrent int
}

// FrequencyLimiter 频率限制器接口
type FrequencyLimiter interface {
    Check(ctx context.Context, scope string, key string, limit int, window string) (bool, error)
    Record(ctx context.Context, scope string, key string, window string) error
}

// NewTriggerMatcher 创建触发器匹配器
func NewTriggerMatcher(
    cache cache.CampaignCache,
    playerSvc playersvc.PlayerService,
    freqLimiter FrequencyLimiter,
) *TriggerMatcher {
    return &TriggerMatcher{
        cache:         cache,
        playerSvc:     playerSvc,
        freqLimiter:   freqLimiter,
        logger:        slog.Default(),
        maxConcurrent: 100,
    }
}

// Match 匹配事件到所有活动
func (tm *TriggerMatcher) Match(ctx context.Context, event Event) ([]MatchResult, error) {
    startTime := time.Now()
    defer func() {
        tm.logger.Debug("trigger_match_completed",
            "event_type", event.EventType,
            "player_id", event.PlayerID,
            "duration_ms", time.Since(startTime).Milliseconds())
    }()

    // 1. 获取所有活动活动实例
    campaigns, err := tm.cache.GetActiveCampaigns(ctx, event.GameID, event.Env)
    if err != nil {
        return nil, fmt.Errorf("get active campaigns: %w", err)
    }

    if len(campaigns) == 0 {
        return nil, nil
    }

    // 2. 并发匹配（控制并发数）
    results := make([]MatchResult, 0, len(campaigns))
    resultChan := make(chan *MatchResult, len(campaigns))
    errorChan := make(chan error, len(campaigns))

    sem := make(chan struct{}, tm.maxConcurrent)

    for _, campaign := range campaigns {
        sem <- struct{}{}
        go func(c *types.CampaignInstance) {
            defer func() { <-sem }()

            result, err := tm.matchCampaign(ctx, c, event)
            if err != nil {
                errorChan <- err
                return
            }
            if result != nil {
                resultChan <- result
            }
        }(campaign)
    }

    // 等待所有 goroutine 完成
    go func() {
        for i := 0; i < len(campaigns); i++ {
            <-sem
        }
        close(resultChan)
        close(errorChan)
    }()

    // 收集结果
    for result := range resultChan {
        results = append(results, *result)
    }

    // 处理错误
    for err := range errorChan {
        tm.logger.Warn("match_campaign_error", "error", err)
    }

    // 按优先级排序
    tm.sortResults(results)

    return results, nil
}

// matchCampaign 匹配单个活动
func (tm *TriggerMatcher) matchCampaign(ctx context.Context, campaign *types.CampaignInstance, event Event) (*MatchResult, error) {
    config := campaign.TriggerConfig
    reasons := make([]string, 0, 5)

    // 1. 检查活动状态
    if !campaign.Enabled {
        return nil, nil
    }

    // 2. 检查事件类型
    if !tm.matchEventTypes(config.EventTypes, event.EventType) {
        return nil, nil
    }
    reasons = append(reasons, "event_type_matched")

    // 3. 检查事件属性过滤
    if !tm.matchEventFilter(config.EventFilter, event) {
        return nil, nil
    }
    reasons = append(reasons, "event_filter_passed")

    // 4. 检查时间窗口
    if !tm.matchTimeWindow(ctx, config.TimeWindow, event) {
        return nil, nil
    }
    reasons = append(reasons, "time_window_matched")

    // 5. 检查受众规则
    if !tm.matchAudienceRules(ctx, config.AudienceRules, event) {
        return nil, nil
    }
    reasons = append(reasons, "audience_matched")

    // 6. 检查频率限制
    if !tm.checkFrequencyLimit(ctx, campaign, event) {
        tm.logger.Debug("frequency_limit_exceeded",
            "campaign_id", campaign.ID,
            "player_id", event.PlayerID)
        return nil, nil
    }
    reasons = append(reasons, "frequency_ok")

    // 7. 获取玩家快照
    playerInfo, err := tm.getPlayerSnapshot(ctx, event.PlayerID)
    if err != nil {
        return nil, fmt.Errorf("get player snapshot: %w", err)
    }

    return &MatchResult{
        CampaignID:  campaign.ID,
        TemplateID:  campaign.TemplateID,
        Priority:    campaign.Priority,
        MatchReason: reasons,
        MatchedAt:   time.Now(),
        PlayerInfo:  playerInfo,
        ContextData: make(map[string]interface{}),
    }, nil
}

// matchEventTypes 检查事件类型匹配
func (tm *TriggerMatcher) matchEventTypes(eventTypes []string, eventType string) bool {
    if len(eventTypes) == 0 {
        return true // 空列表表示匹配所有
    }

    for _, et := range eventTypes {
        if et == eventType {
            return true
        }
        // 支持通配符匹配
        if tm.matchWildcard(et, eventType) {
            return true
        }
    }
    return false
}

// matchWildcard 通配符匹配
func (tm *TriggerMatcher) matchWildcard(pattern, s string) bool {
    // 简单实现：支持 * 通配符
    // 例如: "payment.*" 匹配 "payment.success"
    if len(pattern) == 0 {
        return false
    }

    // TODO: 实现完整的通配符匹配
    return false
}

// matchEventFilter 检查事件属性过滤
func (tm *TriggerMatcher) matchEventFilter(filter string, event Event) bool {
    if filter == "" {
        return true
    }

    // 支持 JSONPath 表达式
    // 例如: "$.props.level > 10"
    // 例如: "$.props.item_id == 'sword_001'"
    // 例如: "$.context.platform in ['ios', 'android']"

    result, err := tm.evaluateExpression(filter, event)
    if err != nil {
        tm.logger.Warn("event_filter_eval_error", "filter", filter, "error", err)
        return false
    }

    boolResult, ok := result.(bool)
    return ok && boolResult
}

// evaluateExpression 求值表达式
func (tm *TriggerMatcher) evaluateExpression(expr string, event Event) (interface{}, error) {
    // 使用 govaluate 库
    // 需要将事件属性转换为表达式变量

    // TODO: 实现完整的表达式求值
    return true, nil
}

// matchTimeWindow 检查时间窗口
func (tm *TriggerMatcher) matchTimeWindow(ctx context.Context, window types.TimeWindow, event Event) bool {
    now := time.Now()

    switch window.Type {
    case "absolute":
        // 绝对时间窗口：活动在固定时间段内有效
        return now.After(window.StartTime) && now.Before(window.EndTime)

    case "rolling":
        // 滚动窗口：从玩家首次触发开始计算
        // 需要查询玩家首次触发时间
        firstTrigger, err := tm.getFirstTriggerTime(ctx, event.PlayerID, window.StartTime)
        if err != nil {
            return false
        }
        duration := now.Sub(firstTrigger)
        return duration < window.EndTime.Sub(window.StartTime)

    case "daily":
        // 每日窗口：每天特定时间段
        if !tm.isInActivePeriod(now, window.StartTime, window.EndTime) {
            return false
        }
        if window.DayStart != "" || window.DayEnd != "" {
            dayStart := tm.parseTimeOfDay(window.DayStart)
            dayEnd := tm.parseTimeOfDay(window.DayEnd)
            current := tm.timeOfDay(now)
            return current >= dayStart && current <= dayEnd
        }
        return true

    case "weekly":
        // 每周窗口：特定星期几
        if len(window.WeekDays) > 0 {
            weekday := int(now.Weekday())
            // 转换：周日=0 -> 周一=0
            if weekday == 0 {
                weekday = 6
            } else {
                weekday -= 1
            }
            if !tm.containsInt(window.WeekDays, weekday) {
                return false
            }
        }
        return tm.isInActivePeriod(now, window.StartTime, window.EndTime)

    case "monthly":
        // 每月窗口：每月特定日期
        dayOfMonth := now.Day()
        if len(window.DaysOfMonth) > 0 {
            if !tm.containsInt(window.DaysOfMonth, dayOfMonth) {
                return false
            }
        }
        return tm.isInActivePeriod(now, window.StartTime, window.EndTime)

    case "cron":
        // Cron 表达式
        return tm.matchCron(window.CronExpr, now, window.Timezone)

    default:
        return true
    }
}

// isInActivePeriod 检查是否在活动周期内
func (tm *TriggerMatcher) isInActivePeriod(now, start, end time.Time) bool {
    if start.IsZero() && end.IsZero() {
        return true
    }
    return (now.Equal(start) || now.After(start)) && (now.Equal(end) || now.Before(end))
}

// parseTimeOfDay 解析一天中的时间
func (tm *TriggerMatcher) parseTimeOfDay(s string) int {
    // 解析 "HH:MM" 格式
    var hour, min int
    fmt.Sscanf(s, "%d:%d", &hour, &min)
    return hour*60 + min
}

// timeOfDay 获取一天中的分钟数
func (tm *TriggerMatcher) timeOfDay(t time.Time) int {
    return t.Hour()*60 + t.Minute()
}

// containsInt 检查整数是否在切片中
func (tm *TriggerMatcher) containsInt(slice []int, v int) bool {
    for _, item := range slice {
        if item == v {
            return true
        }
    }
    return false
}

// matchCron Cron 表达式匹配
func (tm *TriggerMatcher) matchCron(cronExpr string, t time.Time, timezone string) bool {
    // TODO: 实现 cron 匹配
    // 使用 github.com/robfig/cron 库
    return true
}

// matchAudienceRules 检查受众规则
func (tm *TriggerMatcher) matchAudienceRules(ctx context.Context, rules *types.AudienceRules, event Event) bool {
    if rules == nil {
        return true
    }

    // 黑名单优先
    if len(rules.Blacklist) > 0 {
        if tm.containsString(rules.Blacklist, event.PlayerID) {
            return false
        }
    }

    // 白名单
    if len(rules.Whitelist) > 0 {
        if !tm.containsString(rules.Whitelist, event.PlayerID) {
            return false
        }
    }

    // 平台限制
    if len(rules.Platforms) > 0 {
        if !tm.containsString(rules.Platforms, event.Platform) {
            return false
        }
    }

    // 渠道限制
    if len(rules.Channels) > 0 {
        if !tm.containsString(rules.Channels, event.Channel) {
            return false
        }
    }

    // 服务器限制
    if len(rules.ServerIds) > 0 {
        if !tm.containsString(rules.ServerIds, event.ServerID) {
            return false
        }
    }

    // 注册时间限制
    if !tm.checkRegisterTime(event.EventTime, rules.RegisterAfter, rules.RegisterBefore) {
        return false
    }

    return true
}

// containsString 检查字符串是否在切片中
func (tm *TriggerMatcher) containsString(slice []string, v string) bool {
    for _, item := range slice {
        if item == v {
            return true
        }
    }
    return false
}

// checkRegisterTime 检查注册时间
func (tm *TriggerMatcher) checkRegisterTime(registerTime time.Time, after, before *time.Time) bool {
    if after != nil && registerTime.Before(*after) {
        return false
    }
    if before != nil && registerTime.After(*before) {
        return false
    }
    return true
}

// checkFrequencyLimit 检查频率限制
func (tm *TriggerMatcher) checkFrequencyLimit(ctx context.Context, campaign *types.CampaignInstance, event Event) bool {
    cap := campaign.TriggerConfig.FrequencyCap
    if cap == nil || cap.MaxCount < 0 {
        return true // 无限制
    }

    // 构建限制键
    var key string
    switch cap.Scope {
    case "global":
        key = fmt.Sprintf("global:%s", campaign.ID)
    case "player":
        key = fmt.Sprintf("player:%s:%s", campaign.ID, event.PlayerID)
    case "server":
        key = fmt.Sprintf("server:%s:%s", campaign.ID, event.ServerID)
    case "campaign":
        key = fmt.Sprintf("campaign:%s:%s", campaign.ID, event.PlayerID)
    default:
        return true
    }

    // 检查限制
    allowed, err := tm.freqLimiter.Check(ctx, cap.Scope, key, cap.MaxCount, cap.Window)
    if err != nil {
        tm.logger.Warn("frequency_limit_check_error", "error", err)
        return true // 出错时允许通过
    }

    return allowed
}

// getFirstTriggerTime 获取首次触发时间
func (tm *TriggerMatcher) getFirstTriggerTime(ctx context.Context, playerID string, campaignStart time.Time) (time.Time, error) {
    // TODO: 从缓存或数据库查询
    return time.Now(), nil
}

// getPlayerSnapshot 获取玩家快照
func (tm *TriggerMatcher) getPlayerSnapshot(ctx context.Context, playerID string) (*PlayerSnapshot, error) {
    player, err := tm.playerSvc.GetPlayer(ctx, playerID)
    if err != nil {
        return nil, err
    }

    return &PlayerSnapshot{
        PlayerID:       player.PlayerID,
        Level:          player.Level,
        VIPLevel:       player.VIPLevel,
        RegisterTime:   player.RegisterTime,
        TotalRecharge:  player.TotalRecharge,
        LastLoginTime:  player.LastLoginTime,
        OnlineDuration: player.OnlineDuration,
        Attributes:     player.Attributes,
    }, nil
}

// sortResults 按优先级排序结果
func (tm *TriggerMatcher) sortResults(results []MatchResult) {
    // 按优先级降序排序
    for i := 0; i < len(results)-1; i++ {
        for j := i + 1; j < len(results); j++ {
            if results[i].Priority < results[j].Priority {
                results[i], results[j] = results[j], results[i]
            }
        }
    }
}
```

### 1.3 频率限制器实现

```go
// internal/campaign/engine/frequency_limiter.go

package engine

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// RedisFrequencyLimiter 基于 Redis 的频率限制器
type RedisFrequencyLimiter struct {
    client *redis.Client
    prefix string
}

// NewRedisFrequencyLimiter 创建频率限制器
func NewRedisFrequencyLimiter(client *redis.Client) *RedisFrequencyLimiter {
    return &RedisFrequencyLimiter{
        client: client,
        prefix: "campaign:freq",
    }
}

// Check 检查是否允许
func (fl *RedisFrequencyLimiter) Check(ctx context.Context, scope, key string, limit int, window string) (bool, error) {
    redisKey := fl.buildKey(scope, key, window)

    switch window {
    case "once":
        // 只检查是否存在
        exists, err := fl.client.Exists(ctx, redisKey).Result()
        return exists == 0, err

    case "daily":
        // 检查当天的计数
        count, err := fl.client.Get(ctx, redisKey).Int()
        if err == redis.Nil {
            return true, nil
        }
        return count < limit, err

    case "weekly", "monthly":
        // 检查计数
        count, err := fl.client.Get(ctx, redisKey).Int()
        if err == redis.Nil {
            return true, nil
        }
        return count < limit, err

    case "activity":
        // 活动期间只触发一次
        exists, err := fl.client.Exists(ctx, redisKey).Result()
        return exists == 0, err

    default:
        // 自定义时间窗口，格式: "seconds:N"
        var seconds int
        fmt.Sscanf(window, "seconds:%d", &seconds)
        if seconds > 0 {
            count, err := fl.client.Get(ctx, redisKey).Int()
            if err == redis.Nil {
                return true, nil
            }
            return count < limit, err
        }
        return true, nil
    }
}

// Record 记录触发
func (fl *RedisFrequencyLimiter) Record(ctx context.Context, scope, key, window string) error {
    redisKey := fl.buildKey(scope, key, window)

    switch window {
    case "once", "activity":
        // 永久设置（或到活动结束）
        return fl.client.Set(ctx, redisKey, 1, 0).Err()

    case "daily":
        // 设置过期时间为当天结束
        now := time.Now()
        endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
        ttl := time.Until(endOfDay)
        return fl.client.Incr(ctx, redisKey).Err()
        // 需要同时设置过期时间（首次）
        // TODO: 使用 Set NX 或 Lua 脚本

    case "weekly":
        // 设置过期时间为本周结束
        now := time.Now()
        endOfWeek := now.Add(time.Duration(7-int(now.Weekday())) * 24 * time.Hour)
        ttl := time.Until(endOfWeek)
        return fl.client.Incr(ctx, redisKey).Err()

    case "monthly":
        // 设置过期时间为本月结束
        now := time.Now()
        endOfMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)
        ttl := time.Until(endOfMonth)
        return fl.client.Incr(ctx, redisKey).Err()

    default:
        // 自定义时间窗口
        var seconds int
        fmt.Sscanf(window, "seconds:%d", &seconds)
        if seconds > 0 {
            pipe := fl.client.Pipeline()
            pipe.Incr(ctx, redisKey)
            pipe.Expire(ctx, redisKey, time.Duration(seconds)*time.Second)
            _, err := pipe.Exec(ctx)
            return err
        }
    }

    return nil
}

// buildKey 构建 Redis 键
func (fl *RedisFrequencyLimiter) buildKey(scope, key, window string) string {
    dateSuffix := ""
    switch window {
    case "daily":
        dateSuffix = ":" + time.Now().Format("2006-01-02")
    case "weekly":
        year, week := time.Now().ISOWeek()
        dateSuffix = fmt.Sprintf(":%d-W%02d", year, week)
    case "monthly":
        dateSuffix = ":" + time.Now().Format("2006-01")
    }
    return fmt.Sprintf("%s:%s:%s%s", fl.prefix, scope, key, dateSuffix)
}
```

---

## 2. 条件评估器 (Condition Evaluator)

### 2.1 核心接口定义

```go
// internal/campaign/engine/evaluator.go

package engine

import (
    "context"

    "github.com/cuihairu/croupier/internal/campaign/types"
)

// Evaluator 条件评估器接口
type Evaluator interface {
    // Evaluate 评估条件组
    Evaluate(ctx context.Context, groups []types.ConditionGroup, input *EvaluationInput) (bool, error)

    // EvaluateGroup 评估单个条件组
    EvaluateGroup(ctx context.Context, group types.ConditionGroup, input *EvaluationInput) (bool, error)
}

// EvaluationInput 评估输入
type EvaluationInput struct {
    Event     Event
    Player    *PlayerSnapshot
    Progress  *types.PlayerProgress
    Campaign  *types.CampaignInstance
    Variables map[string]interface{}
}

// EvaluationOutput 评估输出
type EvaluationOutput struct {
    Passed     bool
    Details    []ConditionDetail
    Variables  map[string]interface{} // 可传递给后续使用
}

// ConditionDetail 条件评估详情
type ConditionDetail struct {
    ConditionID string
    Type        string
    Passed      bool
    Reason      string
    Value       interface{}    // 实际值
    Expected    interface{}    // 期望值
}
```

### 2.2 条件评估器实现

```go
// internal/campaign/engine/condition_evaluator.go

package engine

import (
    "context"
    "fmt"
    "log/slog"
    "reflect"
    "strconv"
    "strings"
    "time"

    "github.com/Knetic/govaluate"
    "github.com/cuihairu/croupier/internal/campaign/types"
)

// ConditionEvaluator 条件评估器
type ConditionEvaluator struct {
    evaluators map[string]ConditionHandler
    logger     *slog.Logger
}

// ConditionHandler 条件处理器接口
type ConditionHandler interface {
    // Evaluate 评估条件
    Evaluate(ctx context.Context, cond types.Condition, input *EvaluationInput) (bool, *ConditionDetail, error)

    // Validate 验证条件配置
    Validate(cond types.Condition) error
}

// NewConditionEvaluator 创建条件评估器
func NewConditionEvaluator() *ConditionEvaluator {
    ce := &ConditionEvaluator{
        evaluators: make(map[string]ConditionHandler),
        logger:     slog.Default(),
    }

    // 注册内置处理器
    ce.RegisterHandler("player_level", &PlayerLevelHandler{})
    ce.RegisterHandler("vip_level", &VIPLevelHandler{})
    ce.RegisterHandler("recharge_amount", &RechargeAmountHandler{})
    ce.RegisterHandler("recharge_count", &RechargeCountHandler{})
    ce.RegisterHandler("online_duration", &OnlineDurationHandler{})
    ce.RegisterHandler("register_days", &RegisterDaysHandler{})
    ce.RegisterHandler("activity_progress", &ActivityProgressHandler{})
    ce.RegisterHandler("event_count", &EventCountHandler{})
    ce.RegisterHandler("expression", &ExpressionHandler{})

    return ce
}

// RegisterHandler 注册条件处理器
func (ce *ConditionEvaluator) RegisterHandler(condType string, handler ConditionHandler) {
    ce.evaluators[condType] = handler
}

// Evaluate 评估条件组
func (ce *ConditionEvaluator) Evaluate(ctx context.Context, groups []types.ConditionGroup, input *EvaluationInput) (bool, error) {
    if len(groups) == 0 {
        return true, nil // 无条件则通过
    }

    // 组之间是 OR 关系（任一组通过即可）
    for _, group := range groups {
        passed, err := ce.EvaluateGroup(ctx, group, input)
        if err != nil {
            ce.logger.Warn("condition_group_error",
                "group_id", group.ID,
                "error", err)
            continue
        }
        if passed {
            return true, nil
        }
    }

    return false, nil
}

// EvaluateGroup 评估单个条件组
func (ce *ConditionEvaluator) EvaluateGroup(ctx context.Context, group types.ConditionGroup, input *EvaluationInput) (bool, error) {
    if len(group.Conditions) == 0 {
        return true, nil
    }

    passed := true
    details := make([]ConditionDetail, 0, len(group.Conditions))

    for _, cond := range group.Conditions {
        handler, ok := ce.evaluators[cond.Type]
        if !ok {
            return false, fmt.Errorf("unknown condition type: %s", cond.Type)
        }

        condPassed, detail, err := handler.Evaluate(ctx, cond, input)
        if err != nil {
            return false, fmt.Errorf("evaluate condition %s: %w", cond.ID, err)
        }

        details = append(details, *detail)

        // 根据 RequireAll 决定逻辑
        if group.RequireAll {
            // AND: 任一失败则整体失败
            if !condPassed {
                return false, nil
            }
        } else {
            // OR: 任一成功则整体成功
            if condPassed {
                return true, nil
            }
            passed = false
        }
    }

    // AND 全部通过 / OR 全部失败
    return passed, nil
}
```

### 2.3 内置条件处理器

```go
// internal/campaign/engine/conditions/player_level.go

package conditions

import (
    "context"
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
)

// PlayerLevelHandler 玩家等级条件处理器
type PlayerLevelHandler struct{}

func (h *PlayerLevelHandler) Evaluate(ctx context.Context, cond types.Condition, input *engine.EvaluationInput) (bool, *engine.ConditionDetail, error) {
    if input.Player == nil {
        return false, &engine.ConditionDetail{
            ConditionID: cond.ID,
            Type:        cond.Type,
            Passed:      false,
            Reason:      "player not found",
        }, nil
    }

    actualLevel := int(input.Player.Level)
    expectedLevel := parseInt(cond.Value)

    passed := compareNumbers(actualLevel, cond.Operator, expectedLevel)

    return passed, &engine.ConditionDetail{
        ConditionID: cond.ID,
        Type:        cond.Type,
        Passed:      passed,
        Reason:      fmt.Sprintf("player level %d %s %d", actualLevel, cond.Operator, expectedLevel),
        Value:       actualLevel,
        Expected:    expectedLevel,
    }, nil
}

func (h *PlayerLevelHandler) Validate(cond types.Condition) error {
    if cond.Operator == "in" || cond.Operator == "not_in" {
        // 验证 value 是数组
        return validateArray(cond.Value)
    }
    return validateNumber(cond.Value)
}
```

```go
// internal/campaign/engine/conditions/recharge_amount.go

package conditions

import (
    "context"
    "fmt"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/service/rechargesvc"
)

// RechargeAmountHandler 累充金额条件处理器
type RechargeAmountHandler struct {
    rechargeSvc rechargesvc.RechargeService
}

func (h *RechargeAmountHandler) Evaluate(ctx context.Context, cond types.Condition, input *engine.EvaluationInput) (bool, *engine.ConditionDetail, error) {
    // 解析时间范围
    var startTime, endTime time.Time
    if timeWindow, ok := cond.Value.(map[string]interface{}); ok {
        if s, ok := timeWindow["start_time"].(string); ok {
            startTime, _ = time.Parse(time.RFC3339, s)
        }
        if e, ok := timeWindow["end_time"].(string); ok {
            endTime, _ = time.Parse(time.RFC3339, e)
        }
    } else {
        // 默认: 活动开始到现在
        startTime = input.Campaign.StartTime
        endTime = time.Now()
    }

    // 获取充值总额
    amount, err := h.rechargeSvc.GetTotalRecharge(ctx, input.Player.PlayerID, startTime, endTime)
    if err != nil {
        return false, nil, fmt.Errorf("get recharge amount: %w", err)
    }

    // 获取目标金额
    targetAmount := parseInt64(cond.Value)
    passed := compareNumbers(int(amount), cond.Operator, int(targetAmount))

    return passed, &engine.ConditionDetail{
        ConditionID: cond.ID,
        Type:        cond.Type,
        Passed:      passed,
        Reason:      fmt.Sprintf("recharge amount %d %s %d", amount, cond.Operator, targetAmount),
        Value:       amount,
        Expected:    targetAmount,
    }, nil
}
```

```go
// internal/campaign/engine/conditions/event_count.go

package conditions

import (
    "context"
    "fmt"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/service/eventsvc"
)

// EventCountHandler 事件计数条件处理器
type EventCountHandler struct {
    eventSvc eventsvc.EventService
}

func (h *EventCountHandler) Evaluate(ctx context.Context, cond types.Condition, input *engine.EvaluationInput) (bool, *engine.ConditionDetail, error) {
    // 解析参数
    params, ok := cond.Value.(map[string]interface{})
    if !ok {
        return false, nil, fmt.Errorf("invalid event_count params")
    }

    eventTypes, _ := params["event_types"].([]string)
    if len(eventTypes) == 0 {
        eventTypes = []string{input.Event.EventType}
    }

    // 解析时间范围
    var startTime, endTime time.Time
    if s, ok := params["start_time"].(string); ok {
        startTime, _ = time.Parse(time.RFC3339, s)
    } else {
        startTime = input.Campaign.StartTime
    }
    if e, ok := params["end_time"].(string); ok {
        endTime, _ = time.Parse(time.RFC3339, e)
    } else {
        endTime = time.Now()
    }

    // 获取事件计数
    count, err := h.eventSvc.CountEvents(ctx, input.Player.PlayerID, eventTypes, startTime, endTime)
    if err != nil {
        return false, nil, fmt.Errorf("count events: %w", err)
    }

    // 获取目标值
    targetCount := int(params["target_count"].(float64))
    passed := compareNumbers(count, cond.Operator, targetCount)

    return passed, &engine.ConditionDetail{
        ConditionID: cond.ID,
        Type:        cond.Type,
        Passed:      passed,
        Reason:      fmt.Sprintf("event count %d %s %d", count, cond.Operator, targetCount),
        Value:       count,
        Expected:    targetCount,
    }, nil
}
```

```go
// internal/campaign/engine/conditions/expression.go

package conditions

import (
    "context"
    "fmt"
    "strings"

    "github.com/Knetic/govaluate"
    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
)

// ExpressionHandler 自定义表达式条件处理器
type ExpressionHandler struct{}

func (h *ExpressionHandler) Evaluate(ctx context.Context, cond types.Condition, input *engine.EvaluationInput) (bool, *engine.ConditionDetail, error) {
    exprStr, ok := cond.Value.(string)
    if !ok {
        return false, nil, fmt.Errorf("expression must be string")
    }

    // 创建表达式
    expr, err := govaluate.NewEvaluableExpression(exprStr)
    if err != nil {
        return false, nil, fmt.Errorf("parse expression: %w", err)
    }

    // 构建参数
    parameters := h.buildParameters(input)

    // 评估表达式
    result, err := expr.Evaluate(parameters)
    if err != nil {
        return false, nil, fmt.Errorf("evaluate expression: %w", err)
    }

    boolResult, ok := result.(bool)
    if !ok {
        // 如果表达式不返回布尔值，尝试转换为布尔
        boolResult = toBool(result)
    }

    return boolResult, &engine.ConditionDetail{
        ConditionID: cond.ID,
        Type:        cond.Type,
        Passed:      boolResult,
        Reason:      fmt.Sprintf("expression '%s' = %v", exprStr, boolResult),
        Value:       result,
    }, nil
}

// buildParameters 构建表达式参数
func (h *ExpressionHandler) buildParameters(input *engine.EvaluationInput) map[string]interface{} {
    params := make(map[string]interface{})

    // 玩家属性
    if input.Player != nil {
        params["player_level"] = int(input.Player.Level)
        params["vip_level"] = int(input.Player.VIPLevel)
        params["total_recharge"] = input.Player.TotalRecharge
        params["player_id"] = input.Player.PlayerID
    }

    // 活动进度
    if input.Progress != nil {
        params["stage"] = int(input.Progress.Stage)
        params["trigger_count"] = input.Progress.TriggerCount
        params["completed"] = input.Progress.Completed

        // 动态进度字段
        for k, v := range input.Progress.Progress {
            // 转换为 snake_case 命名
            key := fmt.Sprintf("progress_%s", toSnakeCase(k))
            params[key] = v
        }
    }

    // 事件属性
    params["event_type"] = input.Event.EventType
    params["event_time"] = input.Event.EventTime.Unix()
    for k, v := range input.Event.Properties {
        key := fmt.Sprintf("event_%s", toSnakeCase(k))
        params[key] = v
    }

    // 上下文变量
    for k, v := range input.Variables {
        params[k] = v
    }

    return params
}

// toSnakeCase 转换为 snake_case
func toSnakeCase(s string) string {
    var result []rune
    for i, r := range s {
        if i > 0 && r >= 'A' && r <= 'Z' {
            result = append(result, '_')
        }
        result = append(result, r)
    }
    return strings.ToLower(string(result))
}
```

### 2.4 辅助函数

```go
// internal/campaign/engine/conditions/utils.go

package conditions

import (
    "reflect"
    "strconv"
)

// compareNumbers 比较数字
func compareNumbers(a int, operator string, b int) bool {
    switch operator {
    case ">":
        return a > b
    case ">=":
        return a >= b
    case "==":
        return a == b
    case "=":
        return a == b
    case "!=":
        return a != b
    case "<>":
        return a != b
    case "<":
        return a < b
    case "<=":
        return a <= b
    case "in":
        // b 应该是数组
        return isInArray(a, b)
    case "not_in":
        return !isInArray(a, b)
    default:
        return false
    }
}

// compareFloats 比较浮点数
func compareFloats(a float64, operator string, b float64) bool {
    switch operator {
    case ">":
        return a > b
    case ">=":
        return a >= b
    case "==":
        return a == b
    case "!=":
        return a != b
    case "<":
        return a < b
    case "<=":
        return a <= b
    default:
        return false
    }
}

// compareStrings 比较字符串
func compareStrings(a, operator, b string) bool {
    switch operator {
    case "==":
        return a == b
    case "!=":
        return a != b
    case "in":
        return isInArrayString(a, b)
    case "not_in":
        return !isInArrayString(a, b)
    case "contains":
        return strings.Contains(a, b)
    case "starts_with":
        return strings.HasPrefix(a, b)
    case "ends_with":
        return strings.HasSuffix(a, b)
    default:
        return false
    }
}

// isInArray 检查数字是否在数组中
func isInArray(a int, b interface{}) bool {
    val := reflect.ValueOf(b)
    if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
        return false
    }

    for i := 0; i < val.Len(); i++ {
        elem := val.Index(i).Interface()
        if int(parseFloat(elem)) == a {
            return true
        }
    }
    return false
}

// isInArrayString 检查字符串是否在数组中
func isInArrayString(a, b string) bool {
    parts := strings.Split(b, ",")
    for _, part := range parts {
        if strings.TrimSpace(part) == a {
            return true
        }
    }
    return false
}

// parseInt 解析整数
func parseInt(v interface{}) int {
    return int(parseInt64(v))
}

// parseInt64 解析 int64
func parseInt64(v interface{}) int64 {
    switch val := v.(type) {
    case int:
        return int64(val)
    case int32:
        return int64(val)
    case int64:
        return val
    case float64:
        return int64(val)
    case float32:
        return int64(val)
    case string:
        i, _ := strconv.ParseInt(val, 10, 64)
        return i
    case bool:
        if val {
            return 1
        }
        return 0
    default:
        return 0
    }
}

// parseFloat 解析浮点数
func parseFloat(v interface{}) float64 {
    switch val := v.(type) {
    case int:
        return float64(val)
    case int32:
        return float64(val)
    case int64:
        return float64(val)
    case float64:
        return val
    case float32:
        return float64(val)
    case string:
        f, _ := strconv.ParseFloat(val, 64)
        return f
    default:
        return 0
    }
}

// parseString 解析字符串
func parseString(v interface{}) string {
    switch val := v.(type) {
    case string:
        return val
    case int, int32, int64, float32, float64:
        return fmt.Sprintf("%v", val)
    case bool:
        return strconv.FormatBool(val)
    default:
        return ""
    }
}

// toBool 转换为布尔值
func toBool(v interface{}) bool {
    switch val := v.(type) {
    case bool:
        return val
    case int, int32, int64:
        return parseInt64(v) != 0
    case float32, float64:
        return parseFloat(v) != 0
    case string:
        return val != "" && val != "0" && val != "false"
    default:
        return false
    }
}

// validateNumber 验证数字配置
func validateNumber(v interface{}) error {
    switch v.(type) {
    case int, int32, int64, float32, float64:
        return nil
    case string:
        _, err := strconv.ParseFloat(v.(string), 64)
        return err
    default:
        return fmt.Errorf("invalid number type: %T", v)
    }
}

// validateArray 验证数组配置
func validateArray(v interface{}) error {
    val := reflect.ValueOf(v)
    if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
        return fmt.Errorf("not an array: %T", v)
    }
    return nil
}
```

---

## 3. 动作执行器 (Action Executor)

### 3.1 核心接口定义

```go
// internal/campaign/engine/executor.go

package engine

import (
    "context"
    "time"
)

// Executor 动作执行器接口
type Executor interface {
    // Execute 执行动作列表
    Execute(ctx context.Context, actions []types.Action, input *ExecutionInput) (*ExecutionOutput, error)

    // ExecuteAction 执行单个动作
    ExecuteAction(ctx context.Context, action types.Action, input *ExecutionInput) (*ActionResult, error)
}

// ExecutionInput 执行输入
type ExecutionInput struct {
    Event       Event
    Campaign    *types.CampaignInstance
    PlayerID    string
    Player      *PlayerSnapshot
    Progress    *types.PlayerProgress
    MatchResult *MatchResult

    // 上下文数据（可在动作间传递）
    ContextData map[string]interface{}
}

// ExecutionOutput 执行输出
type ExecutionOutput struct {
    Success     bool
    Results     []*ActionResult
    Error       error
    DurationMs  int64

    // 更新的数据
    UpdatedProgress *types.PlayerProgress
    ContextData     map[string]interface{}
}

// ActionResult 单个动作执行结果
type ActionResult struct {
    ActionID    string
    ActionType  string
    Success     bool
    Error       error
    Result      interface{}
    DurationMs  int64
    RetryCount  int
    Skipped     bool
    SkipReason  string
}
```

### 3.2 动作执行器实现

```go
// internal/campaign/engine/action_executor.go

package engine

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/types"
)

// ActionExecutor 动作执行器
type ActionExecutor struct {
    handlers   map[string]ActionHandler
    repository ProgressRepository
    logger     *slog.Logger

    // 配置
    maxConcurrent int
    defaultRetry  *RetryConfig
}

// ActionHandler 动作处理器接口
type ActionHandler interface {
    // Execute 执行动作
    Execute(ctx context.Context, action types.Action, input *ExecutionInput) (interface{}, error)

    // Validate 验证动作配置
    Validate(action types.Action) error

    // ShouldRetry 判断是否应该重试
    ShouldRetry(err error) bool
}

// RetryConfig 重试配置
type RetryConfig struct {
    MaxRetries  int
    IntervalMs  int
    BackoffRate float64
}

// NewActionExecutor 创建动作执行器
func NewActionExecutor(repo ProgressRepository) *ActionExecutor {
    ae := &ActionExecutor{
        handlers:      make(map[string]ActionHandler),
        repository:    repo,
        logger:        slog.Default(),
        maxConcurrent: 50,
        defaultRetry: &RetryConfig{
            MaxRetries:  3,
            IntervalMs:  1000,
            BackoffRate: 2.0,
        },
    }

    // 注册内置处理器
    ae.RegisterHandler("grant_reward", &GrantRewardHandler{})
    ae.RegisterHandler("send_notification", &SendNotificationHandler{})
    ae.RegisterHandler("update_progress", &UpdateProgressHandler{})
    ae.RegisterHandler("set_stage", &SetStageHandler{})
    ae.RegisterHandler("complete_activity", &CompleteActivityHandler{})
    ae.RegisterHandler("custom_rpc", &CustomRPCHandler{})
    ae.RegisterHandler("send_mail", &SendMailHandler{})
    ae.RegisterHandler("add_red_dot", &AddRedDotHandler{})
    ae.RegisterHandler("remove_red_dot", &RemoveRedDotHandler{})

    return ae
}

// RegisterHandler 注册动作处理器
func (ae *ActionExecutor) RegisterHandler(actionType string, handler ActionHandler) {
    ae.handlers[actionType] = handler
}

// Execute 执行动作列表
func (ae *ActionExecutor) Execute(ctx context.Context, actions []types.Action, input *ExecutionInput) (*ExecutionOutput, error) {
    startTime := time.Now()

    output := &ExecutionOutput{
        Success:     true,
        Results:     make([]*ActionResult, 0, len(actions)),
        ContextData: make(map[string]interface{}),
    }

    if len(actions) == 0 {
        output.DurationMs = time.Since(startTime).Milliseconds()
        return output, nil
    }

    // 构建动作 DAG（依赖关系）
    dag := ae.buildActionDAG(actions)

    // 按层执行（每层内并行，层间串行）
    for layerNum, layer := range dag.Layers {
        layerResults := ae.executeLayer(ctx, layer, input)
        output.Results = append(output.Results, layerResults...)

        // 检查是否有失败
        for _, result := range layerResults {
            if !result.Success && !result.Skipped {
                // 关键动作失败，是否继续？
                // 取决于动作配置，这里默认继续
                ae.logger.Warn("action_failed",
                    "action_id", result.ActionID,
                    "error", result.Error)
            }
        }

        ae.logger.Debug("action_layer_completed",
            "layer", layerNum,
            "actions", len(layer),
            "duration_ms", time.Since(startTime).Milliseconds())
    }

    output.Success = ae.allActionsSuccess(output.Results)
    output.DurationMs = time.Since(startTime).Milliseconds()

    return output, nil
}

// executeLayer 执行一层动作
func (ae *ActionExecutor) executeLayer(ctx context.Context, layer []types.Action, input *ExecutionInput) []*ActionResult {
    if len(layer) == 0 {
        return nil
    }

    results := make([]*ActionResult, len(layer))
    resultChan := make(chan *ActionResult, len(layer))

    // 控制并发
    sem := make(chan struct{}, ae.maxConcurrent)
    var wg sync.WaitGroup

    for i, action := range layer {
        wg.Add(1)
        sem <- struct{}{}

        go func(idx int, a types.Action) {
            defer wg.Done()
            defer func() { <-sem }()

            // 检查依赖的前置动作是否成功
            skipReason := ae.shouldSkipAction(a, input)
            if skipReason != "" {
                resultChan <- &ActionResult{
                    ActionID:   a.ID,
                    ActionType: a.Type,
                    Skipped:    true,
                    SkipReason: skipReason,
                }
                return
            }

            // 延迟执行
            if a.DelayMs > 0 {
                select {
                case <-time.After(time.Duration(a.DelayMs) * time.Millisecond):
                case <-ctx.Done():
                    resultChan <- &ActionResult{
                        ActionID:   a.ID,
                        ActionType: a.Type,
                        Success:    false,
                        Error:      ctx.Err(),
                    }
                    return
                }
            }

            // 执行动作（带重试）
            result := ae.executeActionWithRetry(ctx, a, input)
            resultChan <- result

            // 更新上下文数据
            if result.Success && result.Result != nil {
                ae.updateContextData(input, a, result.Result)
            }

        }(i, action)
    }

    // 等待完成
    go func() {
        wg.Wait()
        close(resultChan)
    }()

    // 收集结果
    i := 0
    for result := range resultChan {
        results[i] = result
        i++
    }

    return results
}

// executeActionWithRetry 带重试的执行
func (ae *ActionExecutor) executeActionWithRetry(ctx context.Context, action types.Action, input *ExecutionInput) *ActionResult {
    startTime := time.Now()

    result := &ActionResult{
        ActionID:   action.ID,
        ActionType: action.Type,
        Success:    false,
    }

    // 获取重试配置
    retryConfig := ae.defaultRetry
    if action.RetryConfig != nil {
        retryConfig = &RetryConfig{
            MaxRetries:  action.RetryConfig.MaxRetries,
            IntervalMs:  action.RetryConfig.IntervalMs,
            BackoffRate: action.RetryConfig.BackoffRate,
        }
    }

    handler, ok := ae.handlers[action.Type]
    if !ok {
        result.Error = fmt.Errorf("unknown action type: %s", action.Type)
        return result
    }

    // 执行（带重试）
    var lastErr error
    for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
        result.RetryCount = attempt

        output, err := handler.Execute(ctx, action, input)
        lastErr = err

        if err == nil {
            result.Success = true
            result.Result = output
            result.DurationMs = time.Since(startTime).Milliseconds()

            ae.logger.Debug("action_success",
                "action_id", action.ID,
                "action_type", action.Type,
                "duration_ms", result.DurationMs,
                "attempt", attempt)
            return result
        }

        // 检查是否应该重试
        if !handler.ShouldRetry(err) {
            break
        }

        if attempt < retryConfig.MaxRetries {
            // 计算退避时间
            sleepMs := int(float64(retryConfig.IntervalMs) * pow(retryConfig.BackoffRate, float64(attempt)))

            ae.logger.Debug("action_retry",
                "action_id", action.ID,
                "attempt", attempt,
                "next_retry_ms", sleepMs,
                "error", err)

            select {
            case <-time.After(time.Duration(sleepMs) * time.Millisecond):
            case <-ctx.Done():
                result.Error = ctx.Err()
                return result
            }
        }
    }

    result.Error = lastErr
    result.DurationMs = time.Since(startTime).Milliseconds()

    ae.logger.Warn("action_failed_after_retries",
        "action_id", action.ID,
        "attempts", result.RetryCount,
        "error", lastErr)

    return result
}

// shouldSkipAction 检查是否应该跳过动作
func (ae *ActionExecutor) shouldSkipAction(action types.Action, input *ExecutionInput) string {
    if action.Dependency == "" {
        return ""
    }

    // 检查依赖的前置动作结果
    // TODO: 从上下文中获取前置动作结果
    return ""
}

// updateContextData 更新上下文数据
func (ae *ActionExecutor) updateContextData(input *ExecutionInput, action types.Action, result interface{}) {
    if input.ContextData == nil {
        input.ContextData = make(map[string]interface{})
    }
    input.ContextData["action_"+action.ID+"_result"] = result
}

// allActionsSuccess 检查所有动作是否成功
func (ae *ActionExecutor) allActionsSuccess(results []*ActionResult) bool {
    for _, r := range results {
        if !r.Success && !r.Skipped {
            return false
        }
    }
    return true
}

// pow 计算幂
func pow(base, exp float64) float64 {
    result := 1.0
    for i := 0; i < int(exp); i++ {
        result *= base
    }
    return result
}
```

### 3.3 动作 DAG 实现

```go
// internal/campaign/engine/action_dag.go

package engine

import (
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/types"
)

// ActionDAG 动作依赖图
type ActionDAG struct {
    Actions []types.Action
    Layers  [][]types.Action // 按依赖关系分层的动作
}

// buildActionDAG 构建动作 DAG
func (ae *ActionExecutor) buildActionDAG(actions []types.Action) *ActionDAG {
    dag := &ActionDAG{
        Actions: actions,
    }

    if len(actions) == 0 {
        return dag
    }

    // 构建依赖图
    actionMap := make(map[string]*types.Action)
    inDegree := make(map[string]int)
    dependents := make(map[string][]string)

    for i := range actions {
        actionMap[actions[i].ID] = &actions[i]
        inDegree[actions[i].ID] = 0
    }

    for _, action := range actions {
        if action.Dependency != "" {
            if _, exists := actionMap[action.Dependency]; exists {
                inDegree[action.ID]++
                dependents[action.Dependency] = append(dependents[action.Dependency], action.ID)
            }
        }
    }

    // 拓扑排序，分层执行
    queue := make([]string, 0)
    for actionID, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, actionID)
        }
    }

    layers := make([][]types.Action, 0)

    for len(queue) > 0 {
        layerSize := len(queue)
        layer := make([]types.Action, 0, layerSize)

        for i := 0; i < layerSize; i++ {
            actionID := queue[0]
            queue = queue[1:]

            action := *actionMap[actionID]
            layer = append(layer, action)

            // 处理依赖此动作的其他动作
            for _, dependentID := range dependents[actionID] {
                inDegree[dependentID]--
                if inDegree[dependentID] == 0 {
                    queue = append(queue, dependentID)
                }
            }
        }

        layers = append(layers, layer)
    }

    dag.Layers = layers
    return dag
}
```

### 3.4 内置动作处理器

```go
// internal/campaign/engine/actions/grant_reward.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/service/gamesvc"
)

// GrantRewardHandler 奖励发放处理器
type GrantRewardHandler struct {
    gameClient gamesvc.GameServiceClient
}

type GrantRewardParams struct {
    Rewards []RewardItem `json:"rewards"`
    Mail    *MailInfo    `json:"mail,omitempty"`
    Notify  bool         `json:"notify"`
}

type RewardItem struct {
    Type    string `json:"type"`    // item/currency/title/exp/honor
    ID      string `json:"id"`      // 物品ID / 货币类型
    Count   int64  `json:"count"`   // 数量
    Quality string `json:"quality"` // 品质: common/rare/epic/legendary
    Bind    bool   `json:"bind"`    // 是否绑定
    Expire  int32  `json:"expire"`  // 过期时间（天），0表示永久
}

type MailInfo struct {
    Title       string `json:"title"`
    Content     string `json:"content"`
    Sender      string `json:"sender"`
    ExpireDays  int32  `json:"expire_days"`
    Attachments []RewardItem `json:"attachments"`
}

func (h *GrantRewardHandler) Execute(ctx context.Context, action types.Action, input *engine.ExecutionInput) (interface{}, error) {
    var params GrantRewardParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return nil, fmt.Errorf("parse params: %w", err)
    }

    // 构建请求
    request := &gamesvc.GrantRewardRequest{
        PlayerID:   input.PlayerID,
        CampaignID: input.Campaign.ID,
        ActionID:   action.ID,
        Rewards:    make([]gamesvc.RewardItem, 0, len(params.Rewards)),
    }

    // 转换奖励项
    for _, r := range params.Rewards {
        request.Rewards = append(request.Rewards, gamesvc.RewardItem{
            Type:    r.Type,
            ID:      r.ID,
            Count:   r.Count,
            Quality: r.Quality,
            Bind:    r.Bind,
            Expire:  r.Expire,
        })
    }

    // 如果配置了邮件，通过邮件发送
    if params.Mail != nil {
        request.MailTitle = params.Mail.Title
        request.MailContent = params.Mail.Content
        request.MailSender = params.Mail.Sender
        request.MailExpireDays = params.Mail.ExpireDays
    }

    // 调用游戏服务
    response, err := h.gameClient.GrantReward(ctx, request)
    if err != nil {
        return nil, fmt.Errorf("grant reward: %w", err)
    }

    // 返回结果
    return &GrantRewardResult{
        Success:     response.Success,
        Granted:     response.GrantedItems,
        MailID:      response.MailID,
        Balance:     response.NewBalance,
    }, nil
}

func (h *GrantRewardHandler) Validate(action types.Action) error {
    var params GrantRewardParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return err
    }

    if len(params.Rewards) == 0 {
        return fmt.Errorf("at least one reward required")
    }

    for _, r := range params.Rewards {
        if r.Type == "" {
            return fmt.Errorf("reward type is required")
        }
        if r.Count <= 0 {
            return fmt.Errorf("reward count must be positive")
        }
    }

    return nil
}

func (h *GrantRewardHandler) ShouldRetry(err error) bool {
    // 网络错误可以重试
    return IsNetworkError(err)
}

type GrantRewardResult struct {
    Success bool           `json:"success"`
    Granted []RewardItem   `json:"granted"`
    MailID  string         `json:"mail_id,omitempty"`
    Balance map[string]int64 `json:"balance,omitempty"`
}
```

```go
// internal/campaign/engine/actions/send_notification.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/service/notificationsvc"
)

// SendNotificationHandler 通知发送处理器
type SendNotificationHandler struct {
    notificationSvc notificationsvc.NotificationService
}

type NotificationParams struct {
    Type    string                 `json:"type"`    // mail/popup/red_dot/toast/center/banner
    Title   string                 `json:"title"`
    Content string                 `json:"content"`
    Icon    string                 `json:"icon"`
    Action  *NotificationAction    `json:"action,omitempty"`
    Extra   map[string]interface{} `json:"extra,omitempty"`
}

type NotificationAction struct {
    Type string `json:"type"` // navigate/open_panel/jump_quest
    Data string `json:"data"`
}

func (h *SendNotificationHandler) Execute(ctx context.Context, action types.Action, input *engine.ExecutionInput) (interface{}, error) {
    var params NotificationParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return nil, fmt.Errorf("parse params: %w", err)
    }

    // 替换模板变量
    content := h.replaceTemplateVariables(params.Content, input)

    request := &notificationsvc.SendNotificationRequest{
        PlayerID: input.PlayerID,
        Type:     params.Type,
        Title:    params.Title,
        Content:  content,
        Icon:     params.Icon,
        Action:   params.Action,
        Extra:    params.Extra,
    }

    response, err := h.notificationSvc.Send(ctx, request)
    if err != nil {
        return nil, fmt.Errorf("send notification: %w", err)
    }

    return &NotificationResult{
        Success:    response.Success,
        NotificationID: response.ID,
    }, nil
}

// replaceTemplateVariables 替换模板变量
func (h *SendNotificationHandler) replaceTemplateVariables(content string, input *engine.ExecutionInput) string {
    // 支持的变量:
    // {player_id}, {player_name}, {level}, {vip_level}
    // {campaign_name}, {reward_amount} 等
    // TODO: 实现完整的模板变量替换
    return content
}

func (h *SendNotificationHandler) Validate(action types.Action) error {
    var params NotificationParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return err
    }

    validTypes := map[string]bool{
        "mail": true, "popup": true, "red_dot": true,
        "toast": true, "center": true, "banner": true,
    }

    if !validTypes[params.Type] {
        return fmt.Errorf("invalid notification type: %s", params.Type)
    }

    if params.Title == "" && params.Content == "" {
        return fmt.Errorf("title or content is required")
    }

    return nil
}

func (h *SendNotificationHandler) ShouldRetry(err error) bool {
    return IsNetworkError(err)
}

type NotificationResult struct {
    Success          bool   `json:"success"`
    NotificationID   string `json:"notification_id"`
}
```

```go
// internal/campaign/engine/actions/update_progress.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/repository"
)

// UpdateProgressHandler 进度更新处理器
type UpdateProgressHandler struct {
    repo repository.ProgressRepository
}

type UpdateProgressParams struct {
    Operations []ProgressOperation `json:"operations"`
    Stage      *StageOperation     `json:"stage,omitempty"`
    Complete   *CompleteOperation  `json:"complete,omitempty"`
}

type ProgressOperation struct {
    Op    string      `json:"op"`    // set/add/multiply/max/min
    Field string      `json:"field"` // 进度字段名
    Value interface{} `json:"value"`
}

type StageOperation struct {
    Op    string `json:"op"`    // set/increment/decrement
    Value int32  `json:"value"`
}

type CompleteOperation struct {
    SetComplete bool   `json:"set_complete"`
    Stage       *int32 `json:"require_stage,omitempty"`
}

func (h *UpdateProgressHandler) Execute(ctx context.Context, action types.Action, input *engine.ExecutionInput) (interface{}, error) {
    var params UpdateProgressParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return nil, fmt.Errorf("parse params: %w", err)
    }

    // 获取或创建进度
    progress := input.Progress
    if progress == nil {
        progress = &types.PlayerProgress{
            PlayerID:   input.PlayerID,
            CampaignID: input.Campaign.ID,
            GameID:     input.Campaign.GameID,
            Env:        input.Campaign.Env,
            Progress:   make(map[string]interface{}),
            CreatedAt:  time.Now(),
        }
    }

    // 执行字段操作
    changes := make(map[string]interface{})
    for _, op := range params.Operations {
        oldValue := progress.Progress[op.Field]
        newValue := h.applyOperation(oldValue, op)
        progress.Progress[op.Field] = newValue
        changes[op.Field] = newValue
    }

    // 执行阶段操作
    oldStage := progress.Stage
    if params.Stage != nil {
        switch params.Stage.Op {
        case "set":
            progress.Stage = params.Stage.Value
        case "increment":
            progress.Stage += params.Stage.Value
        case "decrement":
            progress.Stage -= params.Stage.Value
        }
    }

    // 执行完成操作
    if params.Complete != nil {
        if params.Complete.SetComplete {
            if params.Complete.Stage == nil || progress.Stage >= *params.Complete.Stage {
                progress.Completed = true
            }
        }
    }

    // 更新触发信息
    progress.TriggerCount++
    now := time.Now()
    if progress.FirstTrigger.IsZero() {
        progress.FirstTrigger = now
    }
    progress.LastTrigger = now
    progress.UpdatedAt = now

    // 保存进度
    if err := h.repo.Save(ctx, progress); err != nil {
        return nil, fmt.Errorf("save progress: %w", err)
    }

    return &UpdateProgressResult{
        Success:       true,
        OldStage:      oldStage,
        NewStage:      progress.Stage,
        Completed:     progress.Completed,
        FieldChanges:  changes,
    }, nil
}

// applyOperation 应用操作
func (h *UpdateProgressHandler) applyOperation(oldValue interface{}, op ProgressOperation) interface{} {
    old := toFloat64(oldValue)
    val := toFloat64(op.Value)

    switch op.Op {
    case "set":
        return op.Value
    case "add":
        return old + val
    case "subtract":
        return old - val
    case "multiply":
        return old * val
    case "divide":
        if val != 0 {
            return old / val
        }
        return old
    case "max":
        if val > old {
            return op.Value
        }
        return oldValue
    case "min":
        if val < old {
            return op.Value
        }
        return oldValue
    default:
        return oldValue
    }
}

func (h *UpdateProgressHandler) Validate(action types.Action) error {
    var params UpdateProgressParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return err
    }

    validOps := map[string]bool{
        "set": true, "add": true, "subtract": true,
        "multiply": true, "divide": true, "max": true, "min": true,
    }

    for _, op := range params.Operations {
        if !validOps[op.Op] {
            return fmt.Errorf("invalid operation: %s", op.Op)
        }
        if op.Field == "" {
            return fmt.Errorf("field name is required")
        }
    }

    return nil
}

func (h *UpdateProgressHandler) ShouldRetry(err error) bool {
    // 进度更新一般不应该重试
    return false
}

type UpdateProgressResult struct {
    Success      bool                   `json:"success"`
    OldStage     int32                  `json:"old_stage"`
    NewStage     int32                  `json:"new_stage"`
    Completed    bool                   `json:"completed"`
    FieldChanges map[string]interface{} `json:"field_changes"`
}
```

```go
// internal/campaign/engine/actions/custom_rpc.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/service/functionsvc"
)

// CustomRPCHandler 自定义 RPC 调用处理器
type CustomRPCHandler struct {
    functionInvoker functionsvc.FunctionInvoker
}

type CustomRPCParams struct {
    FunctionID string                 `json:"function_id"`
    Payload    map[string]interface{} `json:"payload"`
    TimeoutMs  int32                  `json:"timeout_ms"`
    Async      bool                   `json:"async"`
}

func (h *CustomRPCHandler) Execute(ctx context.Context, action types.Action, input *engine.ExecutionInput) (interface{}, error) {
    var params CustomRPCParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return nil, fmt.Errorf("parse params: %w", err)
    }

    // 构建调用上下文
    payload := make(map[string]interface{})
    for k, v := range params.Payload {
        payload[k] = h.replaceVariables(v, input)
    }

    // 添加标准上下文
    payload["__campaign_id"] = input.Campaign.ID
    payload["__campaign_name"] = input.Campaign.Name
    payload["__player_id"] = input.PlayerID
    payload["__event_type"] = input.Event.EventType
    payload["__event_properties"] = input.Event.Properties
    payload["__action_id"] = action.ID

    // 设置超时
    execCtx := ctx
    if params.TimeoutMs > 0 {
        var cancel context.CancelFunc
        execCtx, cancel = context.WithTimeout(ctx, time.Duration(params.TimeoutMs)*time.Millisecond)
        defer cancel()
    }

    // 调用函数
    request := &functionsvc.InvokeRequest{
        FunctionID: params.FunctionID,
        Payload:    payload,
        GameID:     input.Campaign.GameID,
        Env:        input.Campaign.Env,
        PlayerID:   input.PlayerID,
    }

    response, err := h.functionInvoker.Invoke(execCtx, request)
    if err != nil {
        return nil, fmt.Errorf("invoke function: %w", err)
    }

    return &CustomRPCResult{
        Success: response.Success,
        Result:  response.Result,
        Error:   response.Error,
    }, nil
}

// replaceVariables 替换变量
func (h *CustomRPCHandler) replaceVariables(v interface{}, input *engine.ExecutionInput) interface{} {
    // TODO: 实现完整的变量替换
    // 支持引用: {player.level}, {progress.score}, {event.props.item_id} 等
    return v
}

func (h *CustomRPCHandler) Validate(action types.Action) error {
    var params CustomRPCParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return err
    }

    if params.FunctionID == "" {
        return fmt.Errorf("function_id is required")
    }

    return nil
}

func (h *CustomRPCHandler) ShouldRetry(err error) bool {
    // 函数调用超时可以重试
    return IsTimeoutError(err)
}

type CustomRPCResult struct {
    Success bool                   `json:"success"`
    Result  map[string]interface{} `json:"result"`
    Error   string                 `json:"error,omitempty"`
}
```

### 3.5 辅助函数

```go
// internal/campaign/engine/actions/utils.go

package actions

import (
    "errors"
    "net"
    "syscall"
)

// IsNetworkError 判断是否为网络错误
func IsNetworkError(err error) bool {
    if err == nil {
        return false
    }

    // 检查是否为网络相关错误
    var netErr net.Error
    if errors.As(err, &netErr) {
        return true
    }

    // 检查系统调用错误
    var syscallErr syscall.Errno
    if errors.As(err, &syscallErr) {
        switch syscallErr {
        case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ETIMEDOUT:
            return true
        }
    }

    // 检查常见网络错误字符串
    errStr := err.Error()
    networkErrors := []string{
        "connection refused",
        "connection reset",
        "timeout",
        "network is unreachable",
        "no such host",
        "connection lost",
    }

    for _, msg := range networkErrors {
        if contains(errStr, msg) {
            return true
        }
    }

    return false
}

// IsTimeoutError 判断是否为超时错误
func IsTimeoutError(err error) bool {
    if err == nil {
        return false
    }

    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }

    errStr := err.Error()
    return contains(errStr, "timeout") || contains(errStr, "deadline")
}

// contains 字符串包含检查
func contains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (
        s[:len(substr)] == substr ||
        s[len(s)-len(substr):] == substr ||
        indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return i
        }
    }
    return -1
}
```

---

## 4. 集成：Campaign Worker

```go
// cmd/campaign-worker/main.go

package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/cache"
    "github.com/cuihairu/croupier/internal/campaign/engine"
    "github.com/cuihairu/croupier/internal/campaign/repository"
    "github.com/cuihairu/croupier/internal/campaign/service"
    "github.com/cuihairu/croupier/internal/event/worker"
    "github.com/redis/go-redis/v9"
)

func main() {
    // 初始化
    logger := slog.Default()

    // Redis 客户端
    redisClient := redis.NewClient(&redis.Options{
        Addr: getEnv("REDIS_ADDR", "localhost:6379"),
    })

    // 依赖注入
    campaignRepo := repository.NewPostgresRepository(getEnv("DATABASE_URL", ""))
    campaignCache := cache.NewRedisCache(redisClient)
    playerSvc := service.NewPlayerService(campaignRepo)
    gameClient := service.NewGameServiceClient()
    notificationSvc := service.NewNotificationService()
    progressRepo := repository.NewProgressRepository(getEnv("DATABASE_URL", ""))

    // 频率限制器
    freqLimiter := engine.NewRedisFrequencyLimiter(redisClient)

    // 创建核心组件
    triggerMatcher := engine.NewTriggerMatcher(campaignCache, playerSvc, freqLimiter)
    conditionEvaluator := engine.NewConditionEvaluator()
    actionExecutor := engine.NewActionExecutor(progressRepo)

    // 注册外部服务处理器
    actionExecutor.RegisterHandler("grant_reward", &actions.GrantRewardHandler{gameClient})
    actionExecutor.RegisterHandler("send_notification", &actions.SendNotificationHandler{notificationSvc})

    // 创建活动处理器
    campaignHandler := NewCampaignHandler(
        triggerMatcher,
        conditionEvaluator,
        actionExecutor,
        campaignRepo,
        progressRepo,
        logger,
    )

    // 创建事件 Worker
    eventWorkerConfig := worker.WorkerConfig{
        RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379/0"),
        StreamEvents:   "events",
        StreamHighPrio: "events:high_priority",
        StreamDLQ:      "events:dlq",
        ConsumerGroup:  "campaign-group",
        ConsumerName:   "campaign-worker-" + generateID(),
        BatchSize:      50,
        BlockTime:      1 * time.Second,
        FlushInterval:  5 * time.Second,
        MaxRetries:     5,
    }

    eventWorker := worker.NewWorker(eventWorkerConfig)
    eventWorker.RegisterHandler(campaignHandler)

    // 启动
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    logger.Info("campaign-worker starting")

    if err := eventWorker.Start(ctx); err != nil {
        logger.Error("worker start failed", "error", err)
        os.Exit(1)
    }

    <-ctx.Done()

    logger.Info("campaign-worker shutting down")
    eventWorker.Stop()
    logger.Info("campaign-worker stopped")
}

// CampaignHandler 活动事件处理器
type CampaignHandler struct {
    matcher   *engine.TriggerMatcher
    evaluator *engine.ConditionEvaluator
    executor  *engine.ActionExecutor
    repo      repository.CampaignRepository
    progress  repository.ProgressRepository
    logger    *slog.Logger
}

func NewCampaignHandler(
    matcher *engine.TriggerMatcher,
    evaluator *engine.ConditionEvaluator,
    executor *engine.ActionExecutor,
    repo repository.CampaignRepository,
    progress repository.ProgressRepository,
    logger *slog.Logger,
) *CampaignHandler {
    return &CampaignHandler{
        matcher:   matcher,
        evaluator: evaluator,
        executor:  executor,
        repo:      repo,
        progress:  progress,
        logger:    logger,
    }
}

func (h *CampaignHandler) GetEventTypes() []string {
    // 返回所有感兴趣的事件类型
    return []string{
        "player.login",
        "player.register",
        "payment.success",
        "quest.complete",
        "level.up",
        // ...
    }
}

func (h *CampaignHandler) Handle(ctx context.Context, event worker.Event) error {
    startTime := time.Now()
    logger := h.logger.With("event_type", event["event_type"], "player_id", event["player_id"])

    // 转换事件
    campaignEvent := h.convertEvent(event)

    // 1. 触发器匹配
    matches, err := h.matcher.Match(ctx, campaignEvent)
    if err != nil {
        logger.Error("trigger_match_failed", "error", err)
        return err
    }

    if len(matches) == 0 {
        logger.Debug("no_campaigns_matched")
        return nil
    }

    logger.Info("campaigns_matched", "count", len(matches))

    // 2. 处理每个匹配的活动
    for _, match := range matches {
        if err := h.processMatchedCampaign(ctx, match, campaignEvent); err != nil {
            logger.Error("process_campaign_failed",
                "campaign_id", match.CampaignID,
                "error", err)
            // 继续处理其他活动
        }
    }

    logger.Debug("event_processed",
        "duration_ms", time.Since(startTime).Milliseconds())

    return nil
}

func (h *CampaignHandler) processMatchedCampaign(ctx context.Context, match *engine.MatchResult, event engine.Event) error {
    logger := h.logger.With("campaign_id", match.CampaignID)

    // 获取活动实例
    campaign, err := h.repo.Get(ctx, match.CampaignID)
    if err != nil {
        return fmt.Errorf("get campaign: %w", err)
    }

    // 获取玩家进度
    progress, _ := h.progress.Get(ctx, event.PlayerID, campaign.ID)

    // 构建评估输入
    evalInput := &engine.EvaluationInput{
        Event:    event,
        Player:   match.PlayerInfo,
        Progress: progress,
        Campaign: campaign,
    }

    // 2. 条件评估
    passed, err := h.evaluator.Evaluate(ctx, campaign.ConditionGroups, evalInput)
    if err != nil {
        return fmt.Errorf("condition evaluation: %w", err)
    }

    if !passed {
        logger.Debug("conditions_not_met")
        return nil
    }

    logger.Info("conditions_passed")

    // 构建执行输入
    execInput := &engine.ExecutionInput{
        Event:       event,
        Campaign:    campaign,
        PlayerID:    event.PlayerID,
        Player:      match.PlayerInfo,
        Progress:    progress,
        MatchResult: match,
        ContextData: make(map[string]interface{}),
    }

    // 3. 动作执行
    output, err := h.executor.Execute(ctx, campaign.Actions, execInput)
    if err != nil {
        return fmt.Errorf("action execution: %w", err)
    }

    logger.Info("actions_executed",
        "success", output.Success,
        "duration_ms", output.DurationMs)

    // 4. 更新统计
    if err := h.updateStats(ctx, campaign, event, output); err != nil {
        logger.Warn("update_stats_failed", "error", err)
    }

    return nil
}

func (h *CampaignHandler) convertEvent(workerEvent worker.Event) engine.Event {
    // 转换 Worker 事件到 Campaign 事件格式
    event := engine.Event{
        EventID:    getString(workerEvent, "event_id"),
        EventType:  getString(workerEvent, "event_type"),
        PlayerID:   getString(workerEvent, "player_id"),
        GameID:     getString(workerEvent, "game_id"),
        Env:        getString(workerEvent, "env"),
        ServerID:   getString(workerEvent, "server_id"),
        SessionID:  getString(workerEvent, "session_id"),
        Platform:   getString(workerEvent, "platform"),
        Channel:    getString(workerEvent, "channel"),
        Country:    getString(workerEvent, "country"),
        DeviceID:   getString(workerEvent, "device_id"),
        AppVersion: getString(workerEvent, "app_version"),
        Properties: make(map[string]interface{}),
    }

    if props, ok := workerEvent["properties"].(map[string]interface{}); ok {
        event.Properties = props
    }

    if ts, ok := workerEvent["event_time"].(string); ok {
        event.EventTime, _ = time.Parse(time.RFC3339, ts)
    }

    return event
}

func (h *CampaignHandler) updateStats(ctx context.Context, campaign *types.CampaignInstance, event engine.Event, output *engine.ExecutionOutput) error {
    campaign.Stats.TotalTriggers++
    campaign.Stats.LastTriggerTime = time.Now()

    if output.Success {
        campaign.Stats.SuccessCount++
    } else {
        campaign.Stats.FailureCount++
    }

    return h.repo.UpdateStats(ctx, campaign.ID, campaign.Stats)
}
```

---

## 5. 性能优化

### 5.1 批处理优化

```go
// BatchProcessor 批处理器
type BatchProcessor struct {
    buffer    []Event
    maxSize   int
    flushInterval time.Duration
    processor func(ctx context.Context, events []Event) error
}

func (bp *BatchProcessor) Add(ctx context.Context, event Event) error {
    bp.buffer = append(bp.buffer, event)

    if len(bp.buffer) >= bp.maxSize {
        return bp.flush(ctx)
    }

    return nil
}

func (bp *BatchProcessor) flush(ctx context.Context) error {
    if len(bp.buffer) == 0 {
        return nil
    }

    events := bp.buffer
    bp.buffer = make([]Event, 0, bp.maxSize)

    return bp.processor(ctx, events)
}
```

### 5.2 缓存策略

```go
// CachedMatcher 带缓存的匹配器
type CachedMatcher struct {
    inner   engine.Matcher
    cache   *lru.Cache
    ttl     time.Duration
}

func (cm *CachedMatcher) Match(ctx context.Context, event engine.Event) ([]engine.MatchResult, error) {
    // 生成缓存键
    key := fmt.Sprintf("%s:%s:%s", event.GameID, event.Env, event.EventType)

    // 尝试从缓存获取
    if cached, ok := cm.cache.Get(key); ok {
        campaigns := cached.([]*types.CampaignInstance)
        // 过滤并返回匹配结果
        return cm.filterMatched(ctx, campaigns, event), nil
    }

    // 从源获取
    results, err := cm.inner.Match(ctx, event)
    if err != nil {
        return nil, err
    }

    // 缓存结果
    cm.cache.Set(key, results, cm.ttl)

    return results, nil
}
```

### 5.3 指标收集

```go
// MetricsCollector 指标收集器
type MetricsCollector struct {
    matcherCounter   prometheus.Counter
    evaluatorCounter prometheus.Counter
    executorCounter  prometheus.Counter
    executorDuration prometheus.Histogram
    executorErrors   prometheus.Counter
}

func (mc *MetricsCollector) RecordExecution(output *engine.ExecutionOutput) {
    mc.executorCounter.Inc()
    mc.executorDuration.Observe(float64(output.DurationMs))

    if !output.Success {
        mc.executorErrors.Inc()
    }

    for _, result := range output.Results {
        if result.Skipped {
            skippedCounter.WithLabelValues(result.ActionType).Inc()
        } else if !result.Success {
            errorCounter.WithLabelValues(result.ActionType).Inc()
        }
    }
}
```

---

## 6. 测试示例

```go
// internal/campaign/engine/trigger_matcher_test.go

package engine

import (
    "context"
    "testing"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestTriggerMatcher_Match(t *testing.T) {
    // Mock 依赖
    mockCache := new(MockCampaignCache)
    mockPlayerSvc := new(MockPlayerService)
    mockFreqLimiter := new(MockFrequencyLimiter)

    matcher := NewTriggerMatcher(mockCache, mockPlayerSvc, mockFreqLimiter)

    // 准备测试数据
    event := Event{
        EventType:  "player.login",
        PlayerID:   "player123",
        GameID:     "game1",
        Env:        "prod",
        EventTime:  time.Now(),
        Platform:   "ios",
        Properties: make(map[string]interface{}),
    }

    campaign := &types.CampaignInstance{
        ID:       "campaign1",
        Enabled:  true,
        Priority: 10,
        TriggerConfig: types.TriggerConfig{
            EventTypes: []string{"player.login"},
            TimeWindow: types.TimeWindow{
                Type:      "absolute",
                StartTime: time.Now().Add(-time.Hour),
                EndTime:   time.Now().Add(time.Hour),
            },
        },
    }

    // 设置 Mock 期望
    mockCache.On("GetActiveCampaigns", mock.Anything, "game1", "prod").Return([]*types.CampaignInstance{campaign}, nil)
    mockPlayerSvc.On("GetPlayer", mock.Anything, "player123").Return(&PlayerSnapshot{
        PlayerID: "player123",
        Level:    10,
    }, nil)
    mockFreqLimiter.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

    // 执行
    results, err := matcher.Match(context.Background(), event)

    // 验证
    assert.NoError(t, err)
    assert.Len(t, results, 1)
    assert.Equal(t, "campaign1", results[0].CampaignID)

    // 验证 Mock 调用
    mockCache.AssertExpectations(t)
    mockPlayerSvc.AssertExpectations(t)
}
```

---

## 7. 监控指标

```go
// internal/campaign/metrics/metrics.go

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    // 触发器指标
    TriggerMatchAttempts = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "campaign_trigger_match_attempts_total",
            Help: "Total number of trigger match attempts",
        },
        []string{"game_id", "env", "event_type"},
    )

    TriggerMatches = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "campaign_trigger_matches_total",
            Help: "Total number of successful trigger matches",
        },
        []string{"game_id", "env", "campaign_id"},
    )

    // 条件指标
    ConditionEvaluations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "campaign_condition_evaluations_total",
            Help: "Total number of condition evaluations",
        },
        []string{"game_id", "env", "condition_type"},
    )

    ConditionPasses = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "campaign_condition_passes_total",
            Help: "Total number of passed condition evaluations",
        },
        []string{"game_id", "env", "condition_type"},
    )

    // 动作指标
    ActionExecutions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "campaign_action_executions_total",
            Help: "Total number of action executions",
        },
        []string{"game_id", "env", "action_type"},
    )

    ActionSuccesses = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "campaign_action_successes_total",
            Help: "Total number of successful action executions",
        },
        []string{"game_id", "env", "action_type"},
    )

    ActionFailures = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "campaign_action_failures_total",
            Help: "Total number of failed action executions",
        },
        []string{"game_id", "env", "action_type", "error_type"},
    )

    ActionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "campaign_action_duration_milliseconds",
            Help:    "Action execution duration in milliseconds",
            Buckets: []float64{10, 50, 100, 500, 1000, 5000, 10000},
        },
        []string{"game_id", "env", "action_type"},
    )

    // 活动指标
    CampaignProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "campaign_processing_duration_milliseconds",
            Help:    "Total campaign processing duration in milliseconds",
            Buckets: []float64{10, 50, 100, 500, 1000, 5000, 10000},
        },
        []string{"game_id", "env", "campaign_id"},
    )
)

func init() {
    prometheus.MustRegister(
        TriggerMatchAttempts,
        TriggerMatches,
        ConditionEvaluations,
        ConditionPasses,
        ActionExecutions,
        ActionSuccesses,
        ActionFailures,
        ActionDuration,
        CampaignProcessingDuration,
    )
}
```
