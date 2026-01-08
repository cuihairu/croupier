# 活动系统设计文档

## 概述

活动系统是 Croupier 的事件驱动的营销/运营工具，与数据分析系统共用事件源，实现：
- 实时活动触发（玩家登录、充值、任务完成等）
- 灵活的条件判断（玩家等级、VIP、历史行为等）
- 可配置的动作执行（发奖励、发通知、修改状态等）
- 支持多种活动类型（签到、累充、首充、限时活动等）

## 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                            活动系统架构                                                         │
├─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                                 │
│   Event Bus (Redis Streams)                                                                                                    │
│        │                                                                                                                         │
│        ▼                                                                                                                         │
│   ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │
│   │                                                       Campaign Worker                                                      │   │
│   │                                                                                                                          │   │
│   │  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐ │   │
│   │  │                                                    Trigger Matcher (触发器匹配器)                                    │ │   │
│   │  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │ │   │
│   │  │  │ EventType Match │  │  Time Window   │  │  Audience Rules │  │  Cooldown Check │  │  Frequency Cap  │             │ │   │
│   │  │  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘             │ │   │
│   │  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ │   │
│   │                                                                     │                                                   │   │
│   │                                                                     ▼                                                   │   │
│   │  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐ │   │
│   │  │                                                 Condition Evaluator (条件评估器)                                  │ │   │
│   │  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │ │   │
│   │  │  │ Player Level    │  │  VIP Level      │  │  Recharge Amount│  │  Activity Progress│ │ Custom Expression│             │ │   │
│   │  │  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘             │ │   │
│   │  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ │   │
│   │                                                                     │                                                   │   │
│   │                                                                     ▼                                                   │   │
│   │  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐ │   │
│   │  │                                                   Action Executor (动作执行器)                                   │ │   │
│   │  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │ │   │
│   │  │  │  Grant Reward  │  │  Send Mail      │  │  Send Notification│ │  Update Progress │  │  Custom RPC     │             │ │   │
│   │  │  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘             │ │   │
│   │  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ │   │
│   │                                                                                                                          │   │
│   └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## 1. 数据模型定义

### 1.1 活动模板 (Campaign Template)

```go
// internal/campaign/types/template.go

package types

import "time"

// CampaignTemplate 活动模板
type CampaignTemplate struct {
    // 基础信息
    ID          string   `json:"id" db:"id"`
    Name        string   `json:"name" db:"name"`
    Description string   `json:"description" db:"description"`
    Category    string   `json:"category" db:"category"`    // login/recharge/quest/social/limit
    Version     string   `json:"version" db:"version"`

    // 模板参数定义 (用于生成具体活动实例)
    ParameterDefinitions []ParameterDef `json:"parameter_definitions" db:"-"`

    // 触发器配置
    TriggerConfig TriggerConfig `json:"trigger_config" db:"trigger_config"`

    // 条件组配置
    ConditionGroups []ConditionGroup `json:"condition_groups" db:"condition_groups"`

    // 动作配置
    Actions []Action `json:"actions" db:"actions"`

    // 默认值
    DefaultPriority int32  `json:"default_priority" db:"default_priority"`
    DefaultEnabled  bool   `json:"default_enabled" db:"default_enabled"`

    // 元数据
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
    CreatedBy string   `json:"created_by" db:"created_by"`
}

// ParameterDef 参数定义
type ParameterDef struct {
    Name         string      `json:"name"`
    Type         string      `json:"type"`         // int/string/bool/json/item_list
    Label        string      `json:"label"`
    Description  string      `json:"description"`
    Required     bool        `json:"required"`
    DefaultValue interface{} `json:"default_value"`
    Constraints  interface{} `json:"constraints"`   // min/max/options
}

// TriggerConfig 触发器配置
type TriggerConfig struct {
    // 事件类型匹配
    EventTypes []string `json:"event_types"`

    // 事件属性过滤 (JSONPath 表达式)
    EventFilter string `json:"event_filter"`

    // 时间窗口
    TimeWindow TimeWindow `json:"time_window"`

    // 受众规则
    AudienceRules *AudienceRules `json:"audience_rules,omitempty"`

    // 触发限制
    FrequencyCap *FrequencyCap `json:"frequency_cap,omitempty"`
}

// TimeWindow 时间窗口
type TimeWindow struct {
    Type       string    `json:"type"`        // absolute/rolling/daily/weekly/monthly/cron
    StartTime  time.Time `json:"start_time"`
    EndTime    time.Time `json:"end_time"`
    CronExpr   string    `json:"cron_expr"`   // 当 type=cron 时使用
    Timezone   string    `json:"timezone"`    // 时区，默认 Local
    WeekDays   []int     `json:"week_days"`   // 0-6, 周一到周日
    DayStart   string    `json:"day_start"`   // 每天开始时间 "00:00"
    DayEnd     string    `json:"day_end"`     // 每天结束时间 "23:59"
}

// AudienceRules 受众规则
type AudienceRules struct {
    // 白名单
    Whitelist []string `json:"whitelist,omitempty"`  // player_ids

    // 黑名单
    Blacklist []string `json:"blacklist,omitempty"`

    // 平台限制
    Platforms []string `json:"platforms,omitempty"`  // ios/android/web/pc

    // 渠道限制
    Channels []string `json:"channels,omitempty"`

    // 服务器限制
    ServerIds []string `json:"server_ids,omitempty"`

    // 注册时间范围
    RegisterAfter  *time.Time `json:"register_after,omitempty"`
    RegisterBefore *time.Time `json:"register_before,omitempty"`
}

// FrequencyCap 触发频率限制
type FrequencyCap struct {
    Scope    string `json:"scope"`    // global/player/server/campaign
    MaxCount int    `json:"max_count"` // 最大触发次数，-1 表示无限制
    Window   string `json:"window"`   // once/daily/weekly/monthly/activity/seconds:N
}

// ConditionGroup 条件组
type ConditionGroup struct {
    ID             string      `json:"id"`
    LogicOperator  string      `json:"logic_operator"` // AND/OR
    Conditions     []Condition `json:"conditions"`
    RequireAll     bool        `json:"require_all"`     // true=AND, false=OR
}

// Condition 条件
type Condition struct {
    ID       string      `json:"id"`
    Type     string      `json:"type"`     // player_level/vip_level/recharge_amount/etc.
    Operator string      `json:"operator"` // >/>=/==/!=/<=/</in/not_in
    Value    interface{} `json:"value"`
    // 逻辑组合
    IsAnd    bool        `json:"is_and"`   // 与前一个条件的逻辑关系
}

// Action 动作
type Action struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`  // grant_item/send_mail/etc.
    Params      map[string]interface{} `json:"params"`
    DelayMs     int32                  `json:"delay_ms"`      // 延迟执行
    Dependency  string                 `json:"dependency"`    // 依赖的前置动作ID
    RetryConfig *RetryConfig           `json:"retry_config,omitempty"`
}

// RetryConfig 重试配置
type RetryConfig struct {
    MaxRetries int    `json:"max_retries"`
    IntervalMs int    `json:"interval_ms"`
    BackoffRate float64 `json:"backoff_rate"`
}
```

### 1.2 活动实例 (Campaign Instance)

```go
// internal/campaign/types/instance.go

package types

import "time"

// CampaignInstance 活动实例
type CampaignInstance struct {
    // 基础信息
    ID          string `json:"id" db:"id"`
    TemplateID  string `json:"template_id" db:"template_id"`
    Name        string `json:"name" db:"name"`
    GameID      string `json:"game_id" db:"game_id"`
    Env         string `json:"env" db:"env"`

    // 活动时间
    StartTime   time.Time `json:"start_time" db:"start_time"`
    EndTime     time.Time `json:"end_time" db:"end_time"`

    // 活动状态
    Status      string `json:"status" db:"status"` // draft/active/paused/archived

    // 配置 (从模板继承 + 覆盖)
    Priority    int32        `json:"priority" db:"priority"`
    Enabled     bool         `json:"enabled" db:"enabled"`
    TriggerConfig   TriggerConfig   `json:"trigger_config" db:"trigger_config"`
    ConditionGroups []ConditionGroup `json:"condition_groups" db:"condition_groups"`
    Actions     []Action     `json:"actions" db:"actions"`

    // 活动参数 (模板参数的实例化值)
    Parameters  map[string]interface{} `json:"parameters" db:"parameters"`

    // 统计
    Stats       CampaignStats `json:"stats" db:"stats"`

    // 元数据
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
    CreatedBy   string    `json:"created_by" db:"created_by"`
}

// CampaignStats 活动统计
type CampaignStats struct {
    TotalTriggers   int64 `json:"total_triggers" db:"total_triggers"`
    UniquePlayers   int64 `json:"unique_players" db:"unique_players"`
    SuccessCount    int64 `json:"success_count" db:"success_count"`
    FailureCount    int64 `json:"failure_count" db:"failure_count"`
    LastTriggerTime time.Time `json:"last_trigger_time" db:"last_trigger_time"`
}
```

### 1.3 玩家活动进度 (Player Progress)

```go
// internal/campaign/types/progress.go

package types

import "time"

// PlayerProgress 玩家活动进度
type PlayerProgress struct {
    // 主键
    PlayerID    string `json:"player_id" db:"player_id"`
    CampaignID  string `json:"campaign_id" db:"campaign_id"`
    GameID      string `json:"game_id" db:"game_id"`
    Env         string `json:"env" db:"env"`

    // 进度数据
    Progress    map[string]interface{} `json:"progress" db:"progress"`    // 活动特定进度数据
    Stage       int32  `json:"stage" db:"stage"`                        // 当前阶段
    Completed   bool   `json:"completed" db:"completed"`                // 是否完成

    // 触发记录
    TriggerCount int    `json:"trigger_count" db:"trigger_count"`       // 触发次数
    FirstTrigger time.Time `json:"first_trigger" db:"first_trigger"`    // 首次触发时间
    LastTrigger  time.Time `json:"last_trigger" db:"last_trigger"`      // 最后触发时间

    // 奖励记录
    ClaimedRewards []string `json:"claimed_rewards" db:"claimed_rewards"` // 已领取的奖励ID

    // 时间戳
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ProgressKey 进度键
type ProgressKey struct {
    PlayerID   string
    CampaignID string
}
```

## 2. 触发器引擎 (Trigger Engine)

```go
// internal/campaign/engine/trigger.go

package engine

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/repository"
    "github.com/cuihairu/croupier/internal/campaign/cache"
)

// TriggerMatcher 触发器匹配器
type TriggerMatcher struct {
    repo    repository.CampaignRepository
    cache   cache.CampaignCache
}

// Match 匹配事件到活动
func (tm *TriggerMatcher) Match(ctx context.Context, event Event) ([]MatchedCampaign, error) {
    // 1. 从缓存获取所有活动活动实例
    campaigns, err := tm.cache.GetActiveCampaigns(ctx, event.GameID, event.Env)
    if err != nil {
        return nil, fmt.Errorf("get campaigns: %w", err)
    }

    var matched []MatchedCampaign

    for _, campaign := range campaigns {
        if !campaign.Enabled {
            continue
        }

        // 2. 检查事件类型匹配
        if !tm.matchEventType(campaign.TriggerConfig, event) {
            continue
        }

        // 3. 检查事件属性过滤
        if !tm.matchEventFilter(campaign.TriggerConfig, event) {
            continue
        }

        // 4. 检查时间窗口
        if !tm.matchTimeWindow(campaign.TriggerConfig.TimeWindow, event.EventTime) {
            continue
        }

        // 5. 检查受众规则
        if !tm.matchAudience(campaign.TriggerConfig.AudienceRules, event) {
            continue
        }

        // 6. 检查频率限制
        if !tm.checkFrequencyCap(ctx, campaign, event) {
            slog.Debug("frequency cap exceeded",
                "campaign", campaign.ID,
                "player", event.PlayerID)
            continue
        }

        matched = append(matched, MatchedCampaign{
            Campaign:  campaign,
            Event:     event,
            MatchedAt: time.Now(),
        })
    }

    return matched, nil
}

// MatchedCampaign 匹配的活动
type MatchedCampaign struct {
    Campaign  *types.CampaignInstance
    Event     Event
    MatchedAt time.Time
}

// matchEventType 检查事件类型匹配
func (tm *TriggerMatcher) matchEventType(config types.TriggerConfig, event Event) bool {
    if len(config.EventTypes) == 0 {
        return true
    }
    for _, et := range config.EventTypes {
        if et == event.EventType {
            return true
        }
    }
    return false
}

// matchEventFilter 检查事件属性过滤 (JSONPath)
func (tm *TriggerMatcher) matchEventFilter(config types.TriggerConfig, event Event) bool {
    if config.EventFilter == "" {
        return true
    }
    // TODO: 实现 JSONPath 表达式求值
    // 例如: "$.props.level > 10" 或 "$.props.item_id == 'sword_001'"
    return true
}

// matchTimeWindow 检查时间窗口
func (tm *TriggerMatcher) matchTimeWindow(window types.TimeWindow, eventTime time.Time) bool {
    now := time.Now()

    switch window.Type {
    case "absolute":
        // 绝对时间窗口
        return eventTime.Between(window.StartTime, window.EndTime)

    case "rolling":
        // 滚动窗口 (从活动开始算起)
        return now.Before(window.EndTime)

    case "daily":
        // 每日窗口
        if !now.Between(window.StartTime, window.EndTime) {
            return false
        }
        if window.DayStart != "" {
            start := parseTimeOfDay(now, window.DayStart)
            end := parseTimeOfDay(now, window.DayEnd)
            return now.Between(start, end)
        }
        return true

    case "weekly":
        // 每周窗口
        if len(window.WeekDays) > 0 {
            weekday := int(now.Weekday())
            if !contains(window.WeekDays, weekday) {
                return false
            }
        }
        return true

    case "cron":
        // Cron 表达式
        // TODO: 实现 cron 匹配
        return true

    default:
        return true
    }
}

// matchAudience 检查受众规则
func (tm *TriggerMatcher) matchAudience(rules *types.AudienceRules, event Event) bool {
    if rules == nil {
        return true
    }

    // 黑名单检查
    if len(rules.Blacklist) > 0 {
        if contains(rules.Blacklist, event.PlayerID) {
            return false
        }
    }

    // 白名单检查
    if len(rules.Whitelist) > 0 {
        if !contains(rules.Whitelist, event.PlayerID) {
            return false
        }
    }

    // 平台检查
    if len(rules.Platforms) > 0 {
        if !contains(rules.Platforms, event.Platform) {
            return false
        }
    }

    // 渠道检查
    if len(rules.Channels) > 0 {
        if !contains(rules.Channels, event.Channel) {
            return false
        }
    }

    // 服务器检查
    if len(rules.ServerIds) > 0 {
        if !contains(rules.ServerIds, event.ServerID) {
            return false
        }
    }

    // 注册时间检查
    if rules.RegisterAfter != nil && event.RegisterTime.Before(*rules.RegisterAfter) {
        return false
    }
    if rules.RegisterBefore != nil && event.RegisterTime.After(*rules.RegisterBefore) {
        return false
    }

    return true
}

// checkFrequencyCap 检查频率限制
func (tm *TriggerMatcher) checkFrequencyCap(ctx context.Context, campaign *types.CampaignInstance, event Event) bool {
    cap := campaign.TriggerConfig.FrequencyCap
    if cap == nil || cap.MaxCount < 0 {
        return true
    }

    switch cap.Scope {
    case "global":
        // 全局限制
        if campaign.Stats.TotalTriggers >= int64(cap.MaxCount) {
            return false
        }

    case "player":
        // 玩家限制
        progress, err := tm.repo.GetPlayerProgress(ctx, event.PlayerID, campaign.ID)
        if err == nil && progress.TriggerCount >= cap.MaxCount {
            return false
        }

    case "server":
        // 服务器限制
        // TODO: 实现服务器级别计数
        return true

    case "campaign":
        // 活动实例限制 (每个玩家每个活动只能触发 N 次)
        progress, err := tm.repo.GetPlayerProgress(ctx, event.PlayerID, campaign.ID)
        if err == nil {
            switch cap.Window {
            case "once":
                return progress.TriggerCount == 0
            case "daily":
                // 检查今天是否已触发
                today := time.Now().Truncate(24 * time.Hour)
                return progress.LastTrigger.Before(today) || progress.TriggerCount == 0
            }
        }
    }

    return true
}
```

## 3. 条件评估器 (Condition Evaluator)

```go
// internal/campaign/engine/condition.go

package engine

import (
    "context"
    "fmt"
    "log/slog"

    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/repository"
    "github.com/cuihairu/croupier/internal/campaign/evaluator"
)

// ConditionEvaluator 条件评估器
type ConditionEvaluator struct {
    repo          repository.CampaignRepository
    playerService PlayerService
    evaluators    map[string]evaluator.ConditionEvaluator
}

// PlayerService 玩家服务接口
type PlayerService interface {
    GetPlayer(ctx context.Context, playerID string) (*Player, error)
    GetPlayerRecharge(ctx context.Context, playerID string, startTime, endTime int64) (*RechargeInfo, error)
    GetPlayerHistory(ctx context.Context, playerID string, eventTypes []string, limit int) ([]Event, error)
}

// Player 玩家信息
type Player struct {
    PlayerID    string
    Level       int32
    VIPLevel    int32
    RegisterAt  int64
    TotalRecharge int64
    // 其他属性...
}

// Evaluate 评估条件组
func (ce *ConditionEvaluator) Evaluate(ctx context.Context, groups []types.ConditionGroup, matched MatchedCampaign) (bool, error) {
    if len(groups) == 0 {
        return true, nil // 无条件则通过
    }

    // 构建评估上下文
    evalCtx := ce.buildContext(ctx, matched)

    // 组之间是 OR 关系 (任一组通过即可)
    for _, group := range groups {
        passed, err := ce.evaluateGroup(ctx, group, evalCtx)
        if err != nil {
            slog.Warn("condition group evaluation error",
                "group", group.ID,
                "error", err)
            continue
        }
        if passed {
            return true, nil
        }
    }

    return false, nil
}

// EvaluationContext 评估上下文
type EvaluationContext struct {
    Event       Event
    Campaign    *types.CampaignInstance
    Player      *Player
    Progress    *types.PlayerProgress
    Variables   map[string]interface{}
}

// evaluateGroup 评估单个条件组
func (ce *ConditionEvaluator) evaluateGroup(ctx context.Context, group types.ConditionGroup, evalCtx *EvaluationContext) (bool, error) {
    if len(group.Conditions) == 0 {
        return true, nil
    }

    // 组内条件根据 RequireAll 决定是 AND 还是 OR
    for _, cond := range group.Conditions {
        evaluator, ok := ce.evaluators[cond.Type]
        if !ok {
            slog.Warn("unknown condition type", "type", cond.Type)
            return false, fmt.Errorf("unknown condition type: %s", cond.Type)
        }

        passed, err := evaluator.Evaluate(ctx, cond, evalCtx)
        if err != nil {
            return false, err
        }

        if group.RequireAll {
            // AND: 任何一个失败则整个组失败
            if !passed {
                return false, nil
            }
        } else {
            // OR: 任何一个成功则整个组成功
            if passed {
                return true, nil
            }
        }
    }

    // 如果是 AND 且全部通过，返回 true
    // 如果是 OR 且没有通过，返回 false
    return group.RequireAll, nil
}

// buildContext 构建评估上下文
func (ce *ConditionEvaluator) buildContext(ctx context.Context, matched MatchedCampaign) *EvaluationContext {
    // 获取玩家信息
    player, _ := ce.playerService.GetPlayer(ctx, matched.Event.PlayerID)

    // 获取玩家进度
    progress, _ := ce.repo.GetPlayerProgress(ctx, matched.Event.PlayerID, matched.Campaign.ID)

    return &EvaluationContext{
        Event:     matched.Event,
        Campaign:  matched.Campaign,
        Player:    player,
        Progress:  progress,
        Variables: make(map[string]interface{}),
    }
}
```

### 3.1 内置条件评估器

```go
// internal/campaign/evaluator/player_level.go

package evaluator

import (
    "context"
    "strconv"
)

// PlayerLevelEvaluator 玩家等级评估器
type PlayerLevelEvaluator struct{}

func (e *PlayerLevelEvaluator) Evaluate(ctx context.Context, cond types.Condition, evalCtx *EvaluationContext) (bool, error) {
    if evalCtx.Player == nil {
        return false, nil
    }

    level := evalCtx.Player.Level
    targetLevel := parseInt(cond.Value)

    return compareNumbers(int64(level), cond.Operator, targetLevel), nil
}

// parseInt 解析整数
func parseInt(v interface{}) int64 {
    switch val := v.(type) {
    case int:
        return int64(val)
    case int32:
        return int64(val)
    case int64:
        return val
    case float64:
        return int64(val)
    case string:
        i, _ := strconv.ParseInt(val, 10, 64)
        return i
    default:
        return 0
    }
}

// compareNumbers 比较数字
func compareNumbers(a int64, op string, b int64) bool {
    switch op {
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
```

```go
// internal/campaign/evaluator/recharge_amount.go

package evaluator

import (
    "context"
    "time"
)

// RechargeAmountEvaluator 累充金额评估器
type RechargeAmountEvaluator struct {
    playerService PlayerService
}

func (e *RechargeAmountEvaluator) Evaluate(ctx context.Context, cond types.Condition, evalCtx *EvaluationContext) (bool, error) {
    // 确定时间范围
    var startTime, endTime int64
    if window, ok := cond.Value.(map[string]interface{}); ok {
        // 从条件中获取时间窗口
        if start, ok := window["start_time"].(int64); ok {
            startTime = start
        }
        if end, ok := window["end_time"].(int64); ok {
            endTime = end
        }
    } else {
        // 默认: 从活动开始到现在
        startTime = evalCtx.Campaign.StartTime.Unix()
        endTime = time.Now().Unix()
    }

    // 获取充值信息
    recharge, err := e.playerService.GetPlayerRecharge(ctx, evalCtx.Player.PlayerID, startTime, endTime)
    if err != nil {
        return false, err
    }

    // 获取目标金额
    targetAmount := parseInt(cond.Value)

    return compareNumbers(recharge.TotalAmount, cond.Operator, targetAmount), nil
}
```

```go
// internal/campaign/evaluator/activity_progress.go

package evaluator

import (
    "context"
    "encoding/json"
)

// ActivityProgressEvaluator 活动进度评估器
type ActivityProgressEvaluator struct{}

func (e *ActivityProgressEvaluator) Evaluate(ctx context.Context, cond types.Condition, evalCtx *EvaluationContext) (bool, error) {
    if evalCtx.Progress == nil {
        return false, nil
    }

    // 从进度中获取指定字段的值
    fieldName := cond.Value.(string)
    value, exists := evalCtx.Progress.Progress[fieldName]
    if !exists {
        return false, nil
    }

    // 根据字段类型进行比较
    switch v := value.(type) {
    case float64:
        target := parseInt(cond.Value)
        return compareNumbers(int64(v), cond.Operator, target), nil
    case int:
        target := parseInt(cond.Value)
        return compareNumbers(int64(v), cond.Operator, target), nil
    case bool:
        target, ok := cond.Value.(bool)
        if !ok {
            return false, nil
        }
        return v == target, nil
    default:
        // 字符串比较
        targetStr, ok := cond.Value.(string)
        if !ok {
            return false, nil
        }
        return compareStrings(jsonToString(v), cond.Operator, targetStr), nil
    }
}
```

```go
// internal/campaign/evaluator/expression.go

package evaluator

import (
    "context"
    "fmt"

    "github.com/Knetic/govaluate"
)

// ExpressionEvaluator 自定义表达式评估器
type ExpressionEvaluator struct{}

func (e *ExpressionEvaluator) Evaluate(ctx context.Context, cond types.Condition, evalCtx *EvaluationContext) (bool, error) {
    exprStr, ok := cond.Value.(string)
    if !ok {
        return false, fmt.Errorf("expression must be string")
    }

    // 创建表达式
    expr, err := govaluate.NewEvaluableExpression(exprStr)
    if err != nil {
        return false, fmt.Errorf("parse expression: %w", err)
    }

    // 构建参数
    parameters := make(map[string]interface{})

    // 玩家属性
    if evalCtx.Player != nil {
        parameters["player_level"] = evalCtx.Player.Level
        parameters["vip_level"] = evalCtx.Player.VIPLevel
        parameters["total_recharge"] = evalCtx.Player.TotalRecharge
    }

    // 活动进度
    if evalCtx.Progress != nil {
        for k, v := range evalCtx.Progress.Progress {
            parameters[fmt.Sprintf("progress_%s", k)] = v
        }
    }

    // 事件属性
    parameters["event_type"] = evalCtx.Event.EventType
    for k, v := range evalCtx.Event.Properties {
        parameters[fmt.Sprintf("event_%s", k)] = v
    }

    // 评估表达式
    result, err := expr.Evaluate(parameters)
    if err != nil {
        return false, fmt.Errorf("evaluate expression: %w", err)
    }

    boolResult, ok := result.(bool)
    return boolResult, nil
}
```

## 4. 动作执行器 (Action Executor)

```go
// internal/campaign/engine/action.go

package engine

import (
    "context"
    "log/slog"
    "sync"
    "time"

    "github.com/cuihairu/croupier/internal/campaign/types"
)

// ActionExecutor 动作执行器
type ActionExecutor struct {
    executors    map[string]ActionHandler
    gameClient   GameServiceClient
    notification NotificationService
}

// ActionHandler 动作处理器接口
type ActionHandler interface {
    Execute(ctx context.Context, action types.Action, execCtx *ExecutionContext) error
    Validate(action types.Action) error
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
    Event        Event
    Campaign     *types.CampaignInstance
    PlayerID     string
    Progress     *types.PlayerProgress
    Results      map[string]interface{}  // 动作执行结果
    Variables    map[string]interface{}  // 可在动作间传递的变量
    StartTime    time.Time
}

// ExecuteResult 执行结果
type ExecuteResult struct {
    ActionID     string
    Success      bool
    Error        error
    Result       interface{}
    DurationMs   int64
}

// Execute 执行动作列表
func (ae *ActionExecutor) Execute(ctx context.Context, actions []types.Action, execCtx *ExecutionContext) ([]ExecuteResult, error) {
    results := make([]ExecuteResult, 0, len(actions))

    // 构建动作依赖图
    dag := ae.buildDAG(actions)

    // 按依赖顺序执行
    for _, layer := range dag.TopologicalSort() {
        layerResults := ae.executeLayer(ctx, layer, execCtx)
        results = append(results, layerResults...)

        // 检查是否有失败
        for _, r := range layerResults {
            if !r.Success {
                slog.Error("action execution failed",
                    "action_id", r.ActionID,
                    "error", r.Error)
                // 可以选择继续或中止
            }
        }
    }

    return results, nil
}

// executeLayer 执行一层动作（并行执行）
func (ae *ActionExecutor) executeLayer(ctx context.Context, layer []types.Action, execCtx *ExecutionContext) []ExecuteResult {
    var wg sync.WaitGroup
    results := make([]ExecuteResult, len(layer))
    resultChan := make(chan ExecuteResult, len(layer))

    for i, action := range layer {
        wg.Add(1)
        go func(idx int, a types.Action) {
            defer wg.Done()

            // 延迟执行
            if a.DelayMs > 0 {
                time.Sleep(time.Duration(a.DelayMs) * time.Millisecond)
            }

            start := time.Now()
            result := ExecuteResult{
                ActionID: a.ID,
            }

            // 查找处理器
            handler, ok := ae.executors[a.Type]
            if !ok {
                result.Success = false
                result.Error = fmt.Errorf("unknown action type: %s", a.Type)
                resultChan <- result
                return
            }

            // 执行动作（带重试）
            err := ae.executeWithRetry(ctx, a, handler, execCtx)
            result.Success = (err == nil)
            result.Error = err
            result.DurationMs = time.Since(start).Milliseconds()

            resultChan <- result
        }(i, action)
    }

    go func() {
        wg.Wait()
        close(resultChan)
    }()

    i := 0
    for result := range resultChan {
        results[i] = result
        i++
    }

    return results
}

// executeWithRetry 带重试的执行
func (ae *ActionExecutor) executeWithRetry(ctx context.Context, action types.Action, handler ActionHandler, execCtx *ExecutionContext) error {
    var err error
    maxRetries := 0
    intervalMs := 0
    backoffRate := 1.0

    if action.RetryConfig != nil {
        maxRetries = action.RetryConfig.MaxRetries
        intervalMs = action.RetryConfig.IntervalMs
        backoffRate = action.RetryConfig.BackoffRate
    }

    for attempt := 0; attempt <= maxRetries; attempt++ {
        err = handler.Execute(ctx, action, execCtx)
        if err == nil {
            return nil
        }

        if attempt < maxRetries {
            sleepMs := int(float64(intervalMs) * pow(backoffRate, float64(attempt)))
            time.Sleep(time.Duration(sleepMs) * time.Millisecond)
        }
    }

    return err
}
```

### 4.1 内置动作处理器

```go
// internal/campaign/actions/grant_reward.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/types"
)

// GrantRewardHandler 奖励发放处理器
type GrantRewardHandler struct {
    gameClient GameServiceClient
}

type GrantRewardParams struct {
    Rewards []RewardItem `json:"rewards"`
    Mail    *MailReward  `json:"mail,omitempty"`
}

type RewardItem struct {
    Type     string `json:"type"`     // item/currency/title/exp
    ID       string `json:"id"`       // 物品ID / 货币类型
    Count    int64  `json:"count"`    // 数量
    Quality  string `json:"quality"`  // 品质
    Bind     bool   `json:"bind"`     // 是否绑定
}

type MailReward struct {
    Title    string `json:"title"`
    Content  string `json:"content"`
    Sender   string `json:"sender"`
    ExpireDays int32 `json:"expire_days"`
}

func (h *GrantRewardHandler) Execute(ctx context.Context, action types.Action, execCtx *ExecutionContext) error {
    var params GrantRewardParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return fmt.Errorf("parse params: %w", err)
    }

    // 调用游戏服务发放奖励
    request := &GrantRewardRequest{
        PlayerID:  execCtx.PlayerID,
        Rewards:   params.Rewards,
        CampaignID: execCtx.Campaign.ID,
        ActionID:  action.ID,
    }

    // 如果配置了邮件，则通过邮件发放
    if params.Mail != nil {
        request.MailTitle = params.Mail.Title
        request.MailContent = params.Mail.Content
        request.MailSender = params.Mail.Sender
        request.MailExpireDays = params.Mail.ExpireDays
    }

    return h.gameClient.GrantReward(ctx, request)
}
```

```go
// internal/campaign/actions/send_notification.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/types"
)

// SendNotificationHandler 通知发送处理器
type SendNotificationHandler struct {
    notificationService NotificationService
}

type NotificationParams struct {
    Type     string                 `json:"type"`     // mail/popup/red_dot/toast
    Title    string                 `json:"title"`
    Content  string                 `json:"content"`
    Icon     string                 `json:"icon"`
    Action   *NotificationAction    `json:"action,omitempty"`
    Extra    map[string]interface{} `json:"extra,omitempty"`
}

type NotificationAction struct {
    Type string `json:"type"` // navigate/open_panel/quest
    Data string `json:"data"`
}

func (h *SendNotificationHandler) Execute(ctx context.Context, action types.Action, execCtx *ExecutionContext) error {
    var params NotificationParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return fmt.Errorf("parse params: %w", err)
    }

    // 替换模板变量
    content := h.replaceVariables(params.Content, execCtx)

    request := &SendNotificationRequest{
        PlayerID: execCtx.PlayerID,
        Type:     params.Type,
        Title:    params.Title,
        Content:  content,
        Icon:     params.Icon,
        Action:   params.Action,
        Extra:    params.Extra,
    }

    return h.notificationService.Send(ctx, request)
}
```

```go
// internal/campaign/actions/update_progress.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/types"
    "github.com/cuihairu/croupier/internal/campaign/repository"
)

// UpdateProgressHandler 进度更新处理器
type UpdateProgressHandler struct {
    repo repository.CampaignRepository
}

type UpdateProgressParams struct {
    Operations []ProgressOperation `json:"operations"`
    Stage      *StageOperation     `json:"stage,omitempty"`
}

type ProgressOperation struct {
    Op    string      `json:"op"`    // set/add/multiply/max
    Field string      `json:"field"`
    Value interface{} `json:"value"`
}

type StageOperation struct {
    Op    string `json:"op"`    // set/increment
    Value int32  `json:"value"`
}

func (h *UpdateProgressHandler) Execute(ctx context.Context, action types.Action, execCtx *ExecutionContext) error {
    var params UpdateProgressParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return fmt.Errorf("parse params: %w", err)
    }

    progress := execCtx.Progress
    if progress == nil {
        progress = &types.PlayerProgress{
            PlayerID:   execCtx.PlayerID,
            CampaignID: execCtx.Campaign.ID,
            Progress:   make(map[string]interface{}),
        }
    }

    // 执行字段操作
    for _, op := range params.Operations {
        h.applyOperation(progress.Progress, op)
    }

    // 执行阶段操作
    if params.Stage != nil {
        switch params.Stage.Op {
        case "set":
            progress.Stage = params.Stage.Value
        case "increment":
            progress.Stage += params.Stage.Value
        }
    }

    // 保存进度
    progress.UpdatedAt = time.Now()
    return h.repo.SavePlayerProgress(ctx, progress)
}

func (h *UpdateProgressHandler) applyOperation(progress map[string]interface{}, op ProgressOperation) {
    current := progress[op.Field]

    switch op.Op {
    case "set":
        progress[op.Field] = op.Value
    case "add":
        progress[op.Field] = toFloat64(current) + toFloat64(op.Value)
    case "multiply":
        progress[op.Field] = toFloat64(current) * toFloat64(op.Value)
    case "max":
        if toFloat64(op.Value) > toFloat64(current) {
            progress[op.Field] = op.Value
        }
    }
}
```

```go
// internal/campaign/actions/custom_rpc.go

package actions

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/cuihairu/croupier/internal/campaign/types"
)

// CustomRPCHandler 自定义 RPC 调用处理器
type CustomRPCHandler struct {
    functionInvoker FunctionInvoker
}

type CustomRPCParams struct {
    FunctionID string                 `json:"function_id"`
    Payload    map[string]interface{} `json:"payload"`
    TimeoutMs  int32                  `json:"timeout_ms"`
}

func (h *CustomRPCHandler) Execute(ctx context.Context, action types.Action, execCtx *ExecutionContext) error {
    var params CustomRPCParams
    if err := json.Unmarshal(action.Params, &params); err != nil {
        return fmt.Errorf("parse params: %w", err)
    }

    // 构建调用上下文
    payload := make(map[string]interface{})
    for k, v := range params.Payload {
        payload[k] = h.replaceVariables(v, execCtx)
    }

    // 添加活动上下文
    payload["campaign_id"] = execCtx.Campaign.ID
    payload["player_id"] = execCtx.PlayerID
    payload["event_type"] = execCtx.Event.EventType
    payload["event_properties"] = execCtx.Event.Properties

    // 调用函数
    request := &InvokeRequest{
        FunctionID: params.FunctionID,
        Payload:    payload,
        GameID:     execCtx.Campaign.GameID,
        Env:        execCtx.Campaign.Env,
    }

    if params.TimeoutMs > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, time.Duration(params.TimeoutMs)*time.Millisecond)
        defer cancel()
    }

    response, err := h.functionInvoker.Invoke(ctx, request)
    if err != nil {
        return fmt.Errorf("invoke function: %w", err)
    }

    // 保存结果到执行上下文
    execCtx.Results[action.ID] = response.Result
    return nil
}
```

## 5. 常见活动类型模板

### 5.1 签到活动

```json
{
  "id": "daily_check_in",
  "name": "每日签到",
  "category": "login",
  "trigger_config": {
    "event_types": ["player.login"],
    "frequency_cap": {
      "scope": "player",
      "max_count": 1,
      "window": "daily"
    }
  },
  "condition_groups": [],
  "actions": [
    {
      "id": "grant_reward",
      "type": "grant_reward",
      "params": {
        "rewards": [
          {"type": "currency", "id": "gold", "count": 100}
        ]
      }
    }
  ]
}
```

### 5.2 累充活动

```json
{
  "id": "recharge_milestone",
  "name": "累充活动",
  "category": "recharge",
  "trigger_config": {
    "event_types": ["payment.success"],
    "frequency_cap": {
      "scope": "player",
      "max_count": 1,
      "window": "activity"
    }
  },
  "condition_groups": [
    {
      "id": "check_recharge_amount",
      "logic_operator": "AND",
      "conditions": [
        {
          "id": "total_recharge",
          "type": "recharge_amount",
          "operator": ">=",
          "value": {"min_amount": 10000}
        }
      ]
    }
  ],
  "actions": [
    {
      "id": "grant_milestone_reward",
      "type": "grant_reward",
      "params": {
        "rewards": [
          {"type": "item", "id": "rare_weapon", "count": 1}
        ],
        "mail": {
          "title": "累充奖励",
          "content": "恭喜达成累充目标！"
        }
      }
    }
  ]
}
```

### 5.3 任务活动

```json
{
  "id": "daily_task",
  "name": "每日任务",
  "category": "quest",
  "trigger_config": {
    "event_types": ["quest.complete"],
    "event_filter": "$.props.quest_id == 'daily_boss_kill'"
  },
  "condition_groups": [
    {
      "id": "check_player_level",
      "conditions": [
        {
          "id": "min_level",
          "type": "player_level",
          "operator": ">=",
          "value": 20
        }
      ]
    }
  ],
  "actions": [
    {
      "id": "update_progress",
      "type": "update_progress",
      "params": {
        "operations": [
          {"op": "add", "field": "daily_task_count", "value": 1}
        ]
      }
    },
    {
      "id": "check_complete",
      "type": "update_progress",
      "dependency": "update_progress",
      "params": {
        "stage": {"op": "increment"}
      },
      "condition": "$.progress.daily_task_count >= 3"
    }
  ]
}
```

### 5.4 首充活动

```json
{
  "id": "first_recharge",
  "name": "首充大礼包",
  "category": "recharge",
  "trigger_config": {
    "event_types": ["payment.success"],
    "frequency_cap": {
      "scope": "player",
      "max_count": 1,
      "window": "once"
    }
  },
  "condition_groups": [
    {
      "id": "check_first_recharge",
      "conditions": [
        {
          "id": "recharge_count",
          "type": "recharge_count",
          "operator": "==",
          "value": 1
        }
      ]
    }
  ],
  "actions": [
    {
      "id": "grant_first_reward",
      "type": "grant_reward",
      "params": {
        "rewards": [
          {"type": "currency", "id": "diamond", "count": 648},
          {"type": "item", "id": "first_recharge_pack", "count": 1}
        ]
      }
    }
  ]
}
```

## 6. API 接口定义

```protobuf
// proto/campaign/v1/campaign.proto

syntax = "proto3";

package campaign.v1;

import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

// 活动管理服务
service CampaignService {
    // 活动模板管理
    rpc CreateTemplate(CreateTemplateRequest) returns (Template);
    rpc GetTemplate(GetTemplateRequest) returns (Template);
    rpc ListTemplates(ListTemplatesRequest) returns (ListTemplatesResponse);
    rpc UpdateTemplate(UpdateTemplateRequest) returns (Template);
    rpc DeleteTemplate(DeleteTemplateRequest) returns (DeleteResponse);

    // 活动实例管理
    rpc CreateInstance(CreateInstanceRequest) returns (Instance);
    rpc GetInstance(GetInstanceRequest) returns (Instance);
    rpc ListInstances(ListInstancesRequest) returns (ListInstancesResponse);
    rpc UpdateInstance(UpdateInstanceRequest) returns (Instance);
    rpc DeleteInstance(DeleteInstanceRequest) returns (DeleteResponse);
    rpc PauseInstance(PauseInstanceRequest) returns (Instance);
    rpc ResumeInstance(ResumeInstanceRequest) returns (Instance);

    // 玩家进度查询
    rpc GetPlayerProgress(GetPlayerProgressRequest) returns (PlayerProgress);
    rpc ListPlayerProgress(ListPlayerProgressRequest) returns (ListPlayerProgressResponse);
    rpc ClaimReward(ClaimRewardRequest) returns (ClaimRewardResponse);

    // 统计
    rpc GetCampaignStats(GetCampaignStatsRequest) returns (CampaignStats);
}

message Template {
    string id = 1;
    string name = 2;
    string description = 3;
    string category = 4;
    string version = 5;

    repeated ParameterDef parameter_definitions = 10;
    TriggerConfig trigger_config = 11;
    repeated ConditionGroup condition_groups = 12;
    repeated Action actions = 13;

    int32 default_priority = 20;
    bool default_enabled = 21;

    google.protobuf.Timestamp created_at = 30;
    google.protobuf.Timestamp updated_at = 31;
    string created_by = 32;
}

message Instance {
    string id = 1;
    string template_id = 2;
    string name = 3;
    string game_id = 4;
    string env = 5;

    google.protobuf.Timestamp start_time = 10;
    google.protobuf.Timestamp end_time = 11;
    string status = 12;  // draft/active/paused/archived

    int32 priority = 20;
    bool enabled = 21;
    TriggerConfig trigger_config = 22;
    repeated ConditionGroup condition_groups = 23;
    repeated Action actions = 24;

    google.protobuf.Struct parameters = 30;

    CampaignStats stats = 40;

    google.protobuf.Timestamp created_at = 50;
    google.protobuf.Timestamp updated_at = 51;
    string created_by = 52;
}

message TriggerConfig {
    repeated string event_types = 1;
    string event_filter = 2;
    TimeWindow time_window = 3;
    AudienceRules audience_rules = 4;
    FrequencyCap frequency_cap = 5;
}

message TimeWindow {
    string type = 1;  // absolute/rolling/daily/weekly/cron
    google.protobuf.Timestamp start_time = 2;
    google.protobuf.Timestamp end_time = 3;
    string cron_expr = 4;
    string timezone = 5;
    repeated int32 week_days = 6;
    string day_start = 7;
    string day_end = 8;
}

message AudienceRules {
    repeated string whitelist = 1;
    repeated string blacklist = 2;
    repeated string platforms = 3;
    repeated string channels = 4;
    repeated string server_ids = 5;
    google.protobuf.Timestamp register_after = 6;
    google.protobuf.Timestamp register_before = 7;
}

message FrequencyCap {
    string scope = 1;     // global/player/server/campaign
    int32 max_count = 2;
    string window = 3;    // once/daily/weekly/activity
}

message ConditionGroup {
    string id = 1;
    string logic_operator = 2;  // AND/OR
    repeated Condition conditions = 3;
    bool require_all = 4;
}

message Condition {
    string id = 1;
    string type = 2;
    string operator = 3;
    google.protobuf.Value value = 4;
}

message Action {
    string id = 1;
    string type = 2;
    google.protobuf.Struct params = 3;
    int32 delay_ms = 4;
    string dependency = 5;
    RetryConfig retry_config = 6;
}

message RetryConfig {
    int32 max_retries = 1;
    int32 interval_ms = 2;
    double backoff_rate = 3;
}

message ParameterDef {
    string name = 1;
    string type = 2;
    string label = 3;
    string description = 4;
    bool required = 5;
    google.protobuf.Value default_value = 6;
    google.protobuf.Value constraints = 7;
}

message PlayerProgress {
    string player_id = 1;
    string campaign_id = 2;
    google.protobuf.Struct progress = 3;
    int32 stage = 4;
    bool completed = 5;

    int32 trigger_count = 10;
    google.protobuf.Timestamp first_trigger = 11;
    google.protobuf.Timestamp last_trigger = 12;

    repeated string claimed_rewards = 20;

    google.protobuf.Timestamp created_at = 30;
    google.protobuf.Timestamp updated_at = 31;
}

message CampaignStats {
    int64 total_triggers = 1;
    int64 unique_players = 2;
    int64 success_count = 3;
    int64 failure_count = 4;
    google.protobuf.Timestamp last_trigger_time = 5;
}

// ========== Request/Response Messages ==========

message CreateTemplateRequest {
    string name = 1;
    string description = 2;
    string category = 3;
    TriggerConfig trigger_config = 4;
    repeated ConditionGroup condition_groups = 5;
    repeated Action actions = 6;
    repeated ParameterDef parameter_definitions = 7;
}

message CreateInstanceRequest {
    string template_id = 1;
    string name = 2;
    string game_id = 3;
    string env = 4;
    google.protobuf.Timestamp start_time = 5;
    google.protobuf.Timestamp end_time = 6;
    google.protobuf.Struct parameters = 7;
}

message GetPlayerProgressRequest {
    string player_id = 1;
    string campaign_id = 2;
    string game_id = 3;
    string env = 4;
}

message ClaimRewardRequest {
    string player_id = 1;
    string campaign_id = 2;
    string reward_id = 3;
    string game_id = 4;
    string env = 5;
}

message ClaimRewardResponse {
    bool success = 1;
    repeated Reward granted_rewards = 2;
}

message Reward {
    string type = 1;
    string id = 2;
    int64 count = 3;
}
```

## 7. 目录结构

```
server/
├── cmd/
│   ├── event-gateway/          # 事件网关服务
│   ├── analytics-worker/       # 数据分析 Worker
│   └── campaign-worker/        # 活动 Worker (新增)
│       └── main.go
│
├── internal/
│   └── campaign/
│       ├── types/              # 数据类型定义
│       │   ├── template.go
│       │   ├── instance.go
│       │   ├── progress.go
│       │   └── event.go
│       │
│       ├── engine/             # 核心引擎
│       │   ├── trigger.go      # 触发器匹配器
│       │   ├── condition.go    # 条件评估器
│       │   └── action.go       # 动作执行器
│       │
│       ├── evaluator/          # 条件评估器实现
│       │   ├── evaluator.go    # 接口定义
│       │   ├── player_level.go
│       │   ├── vip_level.go
│       │   ├── recharge_amount.go
│       │   ├── activity_progress.go
│       │   └── expression.go
│       │
│       ├── actions/            # 动作处理器实现
│       │   ├── grant_reward.go
│       │   ├── send_notification.go
│       │   ├── update_progress.go
│       │   └── custom_rpc.go
│       │
│       ├── repository/         # 数据访问层
│       │   ├── repository.go
│       │   ├── template.go
│       │   ├── instance.go
│       │   └── progress.go
│       │
│       ├── cache/              # 缓存层
│       │   └── cache.go
│       │
│       └── service/            # 服务层
│           ├── template_service.go
│           ├── instance_service.go
│           └── progress_service.go
│
├── proto/
│   └── campaign/
│       └── v1/
│           └── campaign.proto
│
└── docs/
    ├── event-driven-architecture.md
    └── campaign-system.md      # 本文档
```

## 8. 数据库表设计

```sql
-- 活动模板表
CREATE TABLE campaign_templates (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL,
    version VARCHAR(20) NOT NULL DEFAULT '1.0.0',

    trigger_config JSON NOT NULL,
    condition_groups JSON NOT NULL,
    actions JSON NOT NULL,

    default_priority INT DEFAULT 0,
    default_enabled BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    created_by VARCHAR(64) NOT NULL,

    INDEX idx_category (category)
);

-- 活动实例表
CREATE TABLE campaign_instances (
    id VARCHAR(64) PRIMARY KEY,
    template_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    game_id VARCHAR(64) NOT NULL,
    env VARCHAR(20) NOT NULL,

    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    status ENUM('draft', 'active', 'paused', 'archived') NOT NULL DEFAULT 'draft',

    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT TRUE,

    trigger_config JSON NOT NULL,
    condition_groups JSON NOT NULL,
    actions JSON NOT NULL,

    parameters JSON,

    stats JSON,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    created_by VARCHAR(64) NOT NULL,

    INDEX idx_game_env (game_id, env),
    INDEX idx_status_time (status, start_time, end_time),
    INDEX idx_template (template_id)
);

-- 玩家活动进度表
CREATE TABLE campaign_player_progress (
    player_id VARCHAR(64) NOT NULL,
    campaign_id VARCHAR(64) NOT NULL,
    game_id VARCHAR(64) NOT NULL,
    env VARCHAR(20) NOT NULL,

    progress JSON NOT NULL,
    stage INT DEFAULT 0,
    completed BOOLEAN DEFAULT FALSE,

    trigger_count INT DEFAULT 0,
    first_trigger TIMESTAMP NULL,
    last_trigger TIMESTAMP NULL,

    claimed_rewards JSON,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (player_id, campaign_id),
    INDEX idx_campaign (campaign_id),
    INDEX idx_player (player_id)
);

-- 动作执行记录表 (审计)
CREATE TABLE campaign_action_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL,
    player_id VARCHAR(64) NOT NULL,
    event_id VARCHAR(64) NOT NULL,

    action_id VARCHAR(64) NOT NULL,
    action_type VARCHAR(50) NOT NULL,

    status ENUM('pending', 'success', 'failed') NOT NULL,
    error_message TEXT,

    input_params JSON,
    output_result JSON,

    duration_ms INT,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_campaign_player (campaign_id, player_id),
    INDEX idx_event (event_id),
    INDEX idx_status_time (status, executed_at)
);
```

## 9. 性能优化

### 9.1 缓存策略

```
+-------------------+     +-------------------+     +-------------------+
|   Redis Cache     |     |   Local Cache     |     |   Database        |
|                   |     |   (sync.Map)      |     |                   |
| Active Campaigns  |<--->| Template Config   |<--->| Persistent Data   |
| Player Progress   |     | Condition Eval    |     |                   |
+-------------------+     +-------------------+     +-------------------+
        ^                         ^                         ^
        | TTL: 5min               | TTL: 1min               |
        |                         |                         |
```

### 9.2 批处理优化

```go
// 批量获取玩家进度
func (r *ProgressRepository) GetPlayerProgressBatch(ctx context.Context, playerIDs []string, campaignID string) (map[string]*types.PlayerProgress, error) {
    // 使用 IN 查询
}

// 批量更新进度
func (r *ProgressRepository) SavePlayerProgressBatch(ctx context.Context, progresses []*types.PlayerProgress) error {
    // 使用批量 INSERT ON DUPLICATE KEY UPDATE
}
```

### 9.3 异步执行

```go
// 非关键路径异步执行
func (ae *ActionExecutor) ExecuteAsync(ctx context.Context, actions []types.Action, execCtx *ExecutionContext) error {
    go func() {
        // 后台执行，不阻塞主流程
        ae.Execute(context.Background(), actions, execCtx)
    }()
    return nil
}
```
