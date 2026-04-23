<template><div><h1 id="活动系统设计文档" tabindex="-1"><a class="header-anchor" href="#活动系统设计文档"><span>活动系统设计文档</span></a></h1>
<h2 id="概述" tabindex="-1"><a class="header-anchor" href="#概述"><span>概述</span></a></h2>
<p>活动系统是 Croupier 的事件驱动的营销/运营工具，与数据分析系统共用事件源，实现：</p>
<ul>
<li>实时活动触发（玩家登录、充值、任务完成等）</li>
<li>灵活的条件判断（玩家等级、VIP、历史行为等）</li>
<li>可配置的动作执行（发奖励、发通知、修改状态等）</li>
<li>支持多种活动类型（签到、累充、首充、限时活动等）</li>
</ul>
<h2 id="系统架构" tabindex="-1"><a class="header-anchor" href="#系统架构"><span>系统架构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐</span>
<span class="line">│                                                            活动系统架构                                                         │</span>
<span class="line">├─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤</span>
<span class="line">│                                                                                                                                 │</span>
<span class="line">│   Event Bus (Redis Streams)                                                                                                    │</span>
<span class="line">│        │                                                                                                                         │</span>
<span class="line">│        ▼                                                                                                                         │</span>
<span class="line">│   ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │</span>
<span class="line">│   │                                                       Campaign Worker                                                      │   │</span>
<span class="line">│   │                                                                                                                          │   │</span>
<span class="line">│   │  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐ │   │</span>
<span class="line">│   │  │                                                    Trigger Matcher (触发器匹配器)                                    │ │   │</span>
<span class="line">│   │  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │ │   │</span>
<span class="line">│   │  │  │ EventType Match │  │  Time Window   │  │  Audience Rules │  │  Cooldown Check │  │  Frequency Cap  │             │ │   │</span>
<span class="line">│   │  │  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘             │ │   │</span>
<span class="line">│   │  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ │   │</span>
<span class="line">│   │                                                                     │                                                   │   │</span>
<span class="line">│   │                                                                     ▼                                                   │   │</span>
<span class="line">│   │  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐ │   │</span>
<span class="line">│   │  │                                                 Condition Evaluator (条件评估器)                                  │ │   │</span>
<span class="line">│   │  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │ │   │</span>
<span class="line">│   │  │  │ Player Level    │  │  VIP Level      │  │  Recharge Amount│  │  Activity Progress│ │ Custom Expression│             │ │   │</span>
<span class="line">│   │  │  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘             │ │   │</span>
<span class="line">│   │  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ │   │</span>
<span class="line">│   │                                                                     │                                                   │   │</span>
<span class="line">│   │                                                                     ▼                                                   │   │</span>
<span class="line">│   │  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐ │   │</span>
<span class="line">│   │  │                                                   Action Executor (动作执行器)                                   │ │   │</span>
<span class="line">│   │  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │ │   │</span>
<span class="line">│   │  │  │  Grant Reward  │  │  Send Mail      │  │  Send Notification│ │  Update Progress │  │  Custom RPC     │             │ │   │</span>
<span class="line">│   │  │  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────────────┘             │ │   │</span>
<span class="line">│   │  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ │   │</span>
<span class="line">│   │                                                                                                                          │   │</span>
<span class="line">│   └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │</span>
<span class="line">│                                                                                                                                 │</span>
<span class="line">└─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_1-数据模型定义" tabindex="-1"><a class="header-anchor" href="#_1-数据模型定义"><span>1. 数据模型定义</span></a></h2>
<h3 id="_1-1-活动模板-campaign-template" tabindex="-1"><a class="header-anchor" href="#_1-1-活动模板-campaign-template"><span>1.1 活动模板 (Campaign Template)</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/types/template.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> types</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"time"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// CampaignTemplate 活动模板</span></span>
<span class="line"><span class="token keyword">type</span> CampaignTemplate <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 基础信息</span></span>
<span class="line">    ID          <span class="token builtin">string</span>   <span class="token string">`json:"id" db:"id"`</span></span>
<span class="line">    Name        <span class="token builtin">string</span>   <span class="token string">`json:"name" db:"name"`</span></span>
<span class="line">    Description <span class="token builtin">string</span>   <span class="token string">`json:"description" db:"description"`</span></span>
<span class="line">    Category    <span class="token builtin">string</span>   <span class="token string">`json:"category" db:"category"`</span>    <span class="token comment">// login/recharge/quest/social/limit</span></span>
<span class="line">    Version     <span class="token builtin">string</span>   <span class="token string">`json:"version" db:"version"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 模板参数定义 (用于生成具体活动实例)</span></span>
<span class="line">    ParameterDefinitions <span class="token punctuation">[</span><span class="token punctuation">]</span>ParameterDef <span class="token string">`json:"parameter_definitions" db:"-"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 触发器配置</span></span>
<span class="line">    TriggerConfig TriggerConfig <span class="token string">`json:"trigger_config" db:"trigger_config"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 条件组配置</span></span>
<span class="line">    ConditionGroups <span class="token punctuation">[</span><span class="token punctuation">]</span>ConditionGroup <span class="token string">`json:"condition_groups" db:"condition_groups"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 动作配置</span></span>
<span class="line">    Actions <span class="token punctuation">[</span><span class="token punctuation">]</span>Action <span class="token string">`json:"actions" db:"actions"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 默认值</span></span>
<span class="line">    DefaultPriority <span class="token builtin">int32</span>  <span class="token string">`json:"default_priority" db:"default_priority"`</span></span>
<span class="line">    DefaultEnabled  <span class="token builtin">bool</span>   <span class="token string">`json:"default_enabled" db:"default_enabled"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 元数据</span></span>
<span class="line">    CreatedAt time<span class="token punctuation">.</span>Time <span class="token string">`json:"created_at" db:"created_at"`</span></span>
<span class="line">    UpdatedAt time<span class="token punctuation">.</span>Time <span class="token string">`json:"updated_at" db:"updated_at"`</span></span>
<span class="line">    CreatedBy <span class="token builtin">string</span>   <span class="token string">`json:"created_by" db:"created_by"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ParameterDef 参数定义</span></span>
<span class="line"><span class="token keyword">type</span> ParameterDef <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Name         <span class="token builtin">string</span>      <span class="token string">`json:"name"`</span></span>
<span class="line">    Type         <span class="token builtin">string</span>      <span class="token string">`json:"type"`</span>         <span class="token comment">// int/string/bool/json/item_list</span></span>
<span class="line">    Label        <span class="token builtin">string</span>      <span class="token string">`json:"label"`</span></span>
<span class="line">    Description  <span class="token builtin">string</span>      <span class="token string">`json:"description"`</span></span>
<span class="line">    Required     <span class="token builtin">bool</span>        <span class="token string">`json:"required"`</span></span>
<span class="line">    DefaultValue <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"default_value"`</span></span>
<span class="line">    Constraints  <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"constraints"`</span>   <span class="token comment">// min/max/options</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// TriggerConfig 触发器配置</span></span>
<span class="line"><span class="token keyword">type</span> TriggerConfig <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 事件类型匹配</span></span>
<span class="line">    EventTypes <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token string">`json:"event_types"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 事件属性过滤 (JSONPath 表达式)</span></span>
<span class="line">    EventFilter <span class="token builtin">string</span> <span class="token string">`json:"event_filter"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 时间窗口</span></span>
<span class="line">    TimeWindow TimeWindow <span class="token string">`json:"time_window"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 受众规则</span></span>
<span class="line">    AudienceRules <span class="token operator">*</span>AudienceRules <span class="token string">`json:"audience_rules,omitempty"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 触发限制</span></span>
<span class="line">    FrequencyCap <span class="token operator">*</span>FrequencyCap <span class="token string">`json:"frequency_cap,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// TimeWindow 时间窗口</span></span>
<span class="line"><span class="token keyword">type</span> TimeWindow <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Type       <span class="token builtin">string</span>    <span class="token string">`json:"type"`</span>        <span class="token comment">// absolute/rolling/daily/weekly/monthly/cron</span></span>
<span class="line">    StartTime  time<span class="token punctuation">.</span>Time <span class="token string">`json:"start_time"`</span></span>
<span class="line">    EndTime    time<span class="token punctuation">.</span>Time <span class="token string">`json:"end_time"`</span></span>
<span class="line">    CronExpr   <span class="token builtin">string</span>    <span class="token string">`json:"cron_expr"`</span>   <span class="token comment">// 当 type=cron 时使用</span></span>
<span class="line">    Timezone   <span class="token builtin">string</span>    <span class="token string">`json:"timezone"`</span>    <span class="token comment">// 时区，默认 Local</span></span>
<span class="line">    WeekDays   <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">int</span>     <span class="token string">`json:"week_days"`</span>   <span class="token comment">// 0-6, 周一到周日</span></span>
<span class="line">    DayStart   <span class="token builtin">string</span>    <span class="token string">`json:"day_start"`</span>   <span class="token comment">// 每天开始时间 "00:00"</span></span>
<span class="line">    DayEnd     <span class="token builtin">string</span>    <span class="token string">`json:"day_end"`</span>     <span class="token comment">// 每天结束时间 "23:59"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// AudienceRules 受众规则</span></span>
<span class="line"><span class="token keyword">type</span> AudienceRules <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 白名单</span></span>
<span class="line">    Whitelist <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token string">`json:"whitelist,omitempty"`</span>  <span class="token comment">// player_ids</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 黑名单</span></span>
<span class="line">    Blacklist <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token string">`json:"blacklist,omitempty"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 平台限制</span></span>
<span class="line">    Platforms <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token string">`json:"platforms,omitempty"`</span>  <span class="token comment">// ios/android/web/pc</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 渠道限制</span></span>
<span class="line">    Channels <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token string">`json:"channels,omitempty"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 服务器限制</span></span>
<span class="line">    ServerIds <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token string">`json:"server_ids,omitempty"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册时间范围</span></span>
<span class="line">    RegisterAfter  <span class="token operator">*</span>time<span class="token punctuation">.</span>Time <span class="token string">`json:"register_after,omitempty"`</span></span>
<span class="line">    RegisterBefore <span class="token operator">*</span>time<span class="token punctuation">.</span>Time <span class="token string">`json:"register_before,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// FrequencyCap 触发频率限制</span></span>
<span class="line"><span class="token keyword">type</span> FrequencyCap <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Scope    <span class="token builtin">string</span> <span class="token string">`json:"scope"`</span>    <span class="token comment">// global/player/server/campaign</span></span>
<span class="line">    MaxCount <span class="token builtin">int</span>    <span class="token string">`json:"max_count"`</span> <span class="token comment">// 最大触发次数，-1 表示无限制</span></span>
<span class="line">    Window   <span class="token builtin">string</span> <span class="token string">`json:"window"`</span>   <span class="token comment">// once/daily/weekly/monthly/activity/seconds:N</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ConditionGroup 条件组</span></span>
<span class="line"><span class="token keyword">type</span> ConditionGroup <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    ID             <span class="token builtin">string</span>      <span class="token string">`json:"id"`</span></span>
<span class="line">    LogicOperator  <span class="token builtin">string</span>      <span class="token string">`json:"logic_operator"`</span> <span class="token comment">// AND/OR</span></span>
<span class="line">    Conditions     <span class="token punctuation">[</span><span class="token punctuation">]</span>Condition <span class="token string">`json:"conditions"`</span></span>
<span class="line">    RequireAll     <span class="token builtin">bool</span>        <span class="token string">`json:"require_all"`</span>     <span class="token comment">// true=AND, false=OR</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Condition 条件</span></span>
<span class="line"><span class="token keyword">type</span> Condition <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    ID       <span class="token builtin">string</span>      <span class="token string">`json:"id"`</span></span>
<span class="line">    Type     <span class="token builtin">string</span>      <span class="token string">`json:"type"`</span>     <span class="token comment">// player_level/vip_level/recharge_amount/etc.</span></span>
<span class="line">    Operator <span class="token builtin">string</span>      <span class="token string">`json:"operator"`</span> <span class="token comment">// >/>=/==/!=/&lt;=/&lt;/in/not_in</span></span>
<span class="line">    Value    <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"value"`</span></span>
<span class="line">    <span class="token comment">// 逻辑组合</span></span>
<span class="line">    IsAnd    <span class="token builtin">bool</span>        <span class="token string">`json:"is_and"`</span>   <span class="token comment">// 与前一个条件的逻辑关系</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Action 动作</span></span>
<span class="line"><span class="token keyword">type</span> Action <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    ID          <span class="token builtin">string</span>                 <span class="token string">`json:"id"`</span></span>
<span class="line">    Type        <span class="token builtin">string</span>                 <span class="token string">`json:"type"`</span>  <span class="token comment">// grant_item/send_mail/etc.</span></span>
<span class="line">    Params      <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"params"`</span></span>
<span class="line">    DelayMs     <span class="token builtin">int32</span>                  <span class="token string">`json:"delay_ms"`</span>      <span class="token comment">// 延迟执行</span></span>
<span class="line">    Dependency  <span class="token builtin">string</span>                 <span class="token string">`json:"dependency"`</span>    <span class="token comment">// 依赖的前置动作ID</span></span>
<span class="line">    RetryConfig <span class="token operator">*</span>RetryConfig           <span class="token string">`json:"retry_config,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// RetryConfig 重试配置</span></span>
<span class="line"><span class="token keyword">type</span> RetryConfig <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    MaxRetries <span class="token builtin">int</span>    <span class="token string">`json:"max_retries"`</span></span>
<span class="line">    IntervalMs <span class="token builtin">int</span>    <span class="token string">`json:"interval_ms"`</span></span>
<span class="line">    BackoffRate <span class="token builtin">float64</span> <span class="token string">`json:"backoff_rate"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_1-2-活动实例-campaign-instance" tabindex="-1"><a class="header-anchor" href="#_1-2-活动实例-campaign-instance"><span>1.2 活动实例 (Campaign Instance)</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/types/instance.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> types</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"time"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// CampaignInstance 活动实例</span></span>
<span class="line"><span class="token keyword">type</span> CampaignInstance <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 基础信息</span></span>
<span class="line">    ID          <span class="token builtin">string</span> <span class="token string">`json:"id" db:"id"`</span></span>
<span class="line">    TemplateID  <span class="token builtin">string</span> <span class="token string">`json:"template_id" db:"template_id"`</span></span>
<span class="line">    Name        <span class="token builtin">string</span> <span class="token string">`json:"name" db:"name"`</span></span>
<span class="line">    GameID      <span class="token builtin">string</span> <span class="token string">`json:"game_id" db:"game_id"`</span></span>
<span class="line">    Env         <span class="token builtin">string</span> <span class="token string">`json:"env" db:"env"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 活动时间</span></span>
<span class="line">    StartTime   time<span class="token punctuation">.</span>Time <span class="token string">`json:"start_time" db:"start_time"`</span></span>
<span class="line">    EndTime     time<span class="token punctuation">.</span>Time <span class="token string">`json:"end_time" db:"end_time"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 活动状态</span></span>
<span class="line">    Status      <span class="token builtin">string</span> <span class="token string">`json:"status" db:"status"`</span> <span class="token comment">// draft/active/paused/archived</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 配置 (从模板继承 + 覆盖)</span></span>
<span class="line">    Priority    <span class="token builtin">int32</span>        <span class="token string">`json:"priority" db:"priority"`</span></span>
<span class="line">    Enabled     <span class="token builtin">bool</span>         <span class="token string">`json:"enabled" db:"enabled"`</span></span>
<span class="line">    TriggerConfig   TriggerConfig   <span class="token string">`json:"trigger_config" db:"trigger_config"`</span></span>
<span class="line">    ConditionGroups <span class="token punctuation">[</span><span class="token punctuation">]</span>ConditionGroup <span class="token string">`json:"condition_groups" db:"condition_groups"`</span></span>
<span class="line">    Actions     <span class="token punctuation">[</span><span class="token punctuation">]</span>Action     <span class="token string">`json:"actions" db:"actions"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 活动参数 (模板参数的实例化值)</span></span>
<span class="line">    Parameters  <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"parameters" db:"parameters"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 统计</span></span>
<span class="line">    Stats       CampaignStats <span class="token string">`json:"stats" db:"stats"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 元数据</span></span>
<span class="line">    CreatedAt   time<span class="token punctuation">.</span>Time <span class="token string">`json:"created_at" db:"created_at"`</span></span>
<span class="line">    UpdatedAt   time<span class="token punctuation">.</span>Time <span class="token string">`json:"updated_at" db:"updated_at"`</span></span>
<span class="line">    CreatedBy   <span class="token builtin">string</span>    <span class="token string">`json:"created_by" db:"created_by"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// CampaignStats 活动统计</span></span>
<span class="line"><span class="token keyword">type</span> CampaignStats <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    TotalTriggers   <span class="token builtin">int64</span> <span class="token string">`json:"total_triggers" db:"total_triggers"`</span></span>
<span class="line">    UniquePlayers   <span class="token builtin">int64</span> <span class="token string">`json:"unique_players" db:"unique_players"`</span></span>
<span class="line">    SuccessCount    <span class="token builtin">int64</span> <span class="token string">`json:"success_count" db:"success_count"`</span></span>
<span class="line">    FailureCount    <span class="token builtin">int64</span> <span class="token string">`json:"failure_count" db:"failure_count"`</span></span>
<span class="line">    LastTriggerTime time<span class="token punctuation">.</span>Time <span class="token string">`json:"last_trigger_time" db:"last_trigger_time"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_1-3-玩家活动进度-player-progress" tabindex="-1"><a class="header-anchor" href="#_1-3-玩家活动进度-player-progress"><span>1.3 玩家活动进度 (Player Progress)</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/types/progress.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> types</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"time"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// PlayerProgress 玩家活动进度</span></span>
<span class="line"><span class="token keyword">type</span> PlayerProgress <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 主键</span></span>
<span class="line">    PlayerID    <span class="token builtin">string</span> <span class="token string">`json:"player_id" db:"player_id"`</span></span>
<span class="line">    CampaignID  <span class="token builtin">string</span> <span class="token string">`json:"campaign_id" db:"campaign_id"`</span></span>
<span class="line">    GameID      <span class="token builtin">string</span> <span class="token string">`json:"game_id" db:"game_id"`</span></span>
<span class="line">    Env         <span class="token builtin">string</span> <span class="token string">`json:"env" db:"env"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 进度数据</span></span>
<span class="line">    Progress    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"progress" db:"progress"`</span>    <span class="token comment">// 活动特定进度数据</span></span>
<span class="line">    Stage       <span class="token builtin">int32</span>  <span class="token string">`json:"stage" db:"stage"`</span>                        <span class="token comment">// 当前阶段</span></span>
<span class="line">    Completed   <span class="token builtin">bool</span>   <span class="token string">`json:"completed" db:"completed"`</span>                <span class="token comment">// 是否完成</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 触发记录</span></span>
<span class="line">    TriggerCount <span class="token builtin">int</span>    <span class="token string">`json:"trigger_count" db:"trigger_count"`</span>       <span class="token comment">// 触发次数</span></span>
<span class="line">    FirstTrigger time<span class="token punctuation">.</span>Time <span class="token string">`json:"first_trigger" db:"first_trigger"`</span>    <span class="token comment">// 首次触发时间</span></span>
<span class="line">    LastTrigger  time<span class="token punctuation">.</span>Time <span class="token string">`json:"last_trigger" db:"last_trigger"`</span>      <span class="token comment">// 最后触发时间</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 奖励记录</span></span>
<span class="line">    ClaimedRewards <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token string">`json:"claimed_rewards" db:"claimed_rewards"`</span> <span class="token comment">// 已领取的奖励ID</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 时间戳</span></span>
<span class="line">    CreatedAt   time<span class="token punctuation">.</span>Time <span class="token string">`json:"created_at" db:"created_at"`</span></span>
<span class="line">    UpdatedAt   time<span class="token punctuation">.</span>Time <span class="token string">`json:"updated_at" db:"updated_at"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ProgressKey 进度键</span></span>
<span class="line"><span class="token keyword">type</span> ProgressKey <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    PlayerID   <span class="token builtin">string</span></span>
<span class="line">    CampaignID <span class="token builtin">string</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_2-触发器引擎-trigger-engine" tabindex="-1"><a class="header-anchor" href="#_2-触发器引擎-trigger-engine"><span>2. 触发器引擎 (Trigger Engine)</span></a></h2>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/engine/trigger.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> engine</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"encoding/json"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line">    <span class="token string">"log/slog"</span></span>
<span class="line">    <span class="token string">"time"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/types"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/repository"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/cache"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// TriggerMatcher 触发器匹配器</span></span>
<span class="line"><span class="token keyword">type</span> TriggerMatcher <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    repo    repository<span class="token punctuation">.</span>CampaignRepository</span>
<span class="line">    cache   cache<span class="token punctuation">.</span>CampaignCache</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Match 匹配事件到活动</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>tm <span class="token operator">*</span>TriggerMatcher<span class="token punctuation">)</span> <span class="token function">Match</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>MatchedCampaign<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 1. 从缓存获取所有活动活动实例</span></span>
<span class="line">    campaigns<span class="token punctuation">,</span> err <span class="token operator">:=</span> tm<span class="token punctuation">.</span>cache<span class="token punctuation">.</span><span class="token function">GetActiveCampaigns</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> event<span class="token punctuation">.</span>GameID<span class="token punctuation">,</span> event<span class="token punctuation">.</span>Env<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">nil</span><span class="token punctuation">,</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"get campaigns: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">var</span> matched <span class="token punctuation">[</span><span class="token punctuation">]</span>MatchedCampaign</span>
<span class="line"></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> campaign <span class="token operator">:=</span> <span class="token keyword">range</span> campaigns <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>campaign<span class="token punctuation">.</span>Enabled <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 2. 检查事件类型匹配</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>tm<span class="token punctuation">.</span><span class="token function">matchEventType</span><span class="token punctuation">(</span>campaign<span class="token punctuation">.</span>TriggerConfig<span class="token punctuation">,</span> event<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 3. 检查事件属性过滤</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>tm<span class="token punctuation">.</span><span class="token function">matchEventFilter</span><span class="token punctuation">(</span>campaign<span class="token punctuation">.</span>TriggerConfig<span class="token punctuation">,</span> event<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 4. 检查时间窗口</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>tm<span class="token punctuation">.</span><span class="token function">matchTimeWindow</span><span class="token punctuation">(</span>campaign<span class="token punctuation">.</span>TriggerConfig<span class="token punctuation">.</span>TimeWindow<span class="token punctuation">,</span> event<span class="token punctuation">.</span>EventTime<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 5. 检查受众规则</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>tm<span class="token punctuation">.</span><span class="token function">matchAudience</span><span class="token punctuation">(</span>campaign<span class="token punctuation">.</span>TriggerConfig<span class="token punctuation">.</span>AudienceRules<span class="token punctuation">,</span> event<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 6. 检查频率限制</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>tm<span class="token punctuation">.</span><span class="token function">checkFrequencyCap</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> campaign<span class="token punctuation">,</span> event<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            slog<span class="token punctuation">.</span><span class="token function">Debug</span><span class="token punctuation">(</span><span class="token string">"frequency cap exceeded"</span><span class="token punctuation">,</span></span>
<span class="line">                <span class="token string">"campaign"</span><span class="token punctuation">,</span> campaign<span class="token punctuation">.</span>ID<span class="token punctuation">,</span></span>
<span class="line">                <span class="token string">"player"</span><span class="token punctuation">,</span> event<span class="token punctuation">.</span>PlayerID<span class="token punctuation">)</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        matched <span class="token operator">=</span> <span class="token function">append</span><span class="token punctuation">(</span>matched<span class="token punctuation">,</span> MatchedCampaign<span class="token punctuation">{</span></span>
<span class="line">            Campaign<span class="token punctuation">:</span>  campaign<span class="token punctuation">,</span></span>
<span class="line">            Event<span class="token punctuation">:</span>     event<span class="token punctuation">,</span></span>
<span class="line">            MatchedAt<span class="token punctuation">:</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> matched<span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// MatchedCampaign 匹配的活动</span></span>
<span class="line"><span class="token keyword">type</span> MatchedCampaign <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Campaign  <span class="token operator">*</span>types<span class="token punctuation">.</span>CampaignInstance</span>
<span class="line">    Event     Event</span>
<span class="line">    MatchedAt time<span class="token punctuation">.</span>Time</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// matchEventType 检查事件类型匹配</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>tm <span class="token operator">*</span>TriggerMatcher<span class="token punctuation">)</span> <span class="token function">matchEventType</span><span class="token punctuation">(</span>config types<span class="token punctuation">.</span>TriggerConfig<span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token builtin">bool</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>config<span class="token punctuation">.</span>EventTypes<span class="token punctuation">)</span> <span class="token operator">==</span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> et <span class="token operator">:=</span> <span class="token keyword">range</span> config<span class="token punctuation">.</span>EventTypes <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> et <span class="token operator">==</span> event<span class="token punctuation">.</span>EventType <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// matchEventFilter 检查事件属性过滤 (JSONPath)</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>tm <span class="token operator">*</span>TriggerMatcher<span class="token punctuation">)</span> <span class="token function">matchEventFilter</span><span class="token punctuation">(</span>config types<span class="token punctuation">.</span>TriggerConfig<span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token builtin">bool</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> config<span class="token punctuation">.</span>EventFilter <span class="token operator">==</span> <span class="token string">""</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token comment">// TODO: 实现 JSONPath 表达式求值</span></span>
<span class="line">    <span class="token comment">// 例如: "$.props.level > 10" 或 "$.props.item_id == 'sword_001'"</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// matchTimeWindow 检查时间窗口</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>tm <span class="token operator">*</span>TriggerMatcher<span class="token punctuation">)</span> <span class="token function">matchTimeWindow</span><span class="token punctuation">(</span>window types<span class="token punctuation">.</span>TimeWindow<span class="token punctuation">,</span> eventTime time<span class="token punctuation">.</span>Time<span class="token punctuation">)</span> <span class="token builtin">bool</span> <span class="token punctuation">{</span></span>
<span class="line">    now <span class="token operator">:=</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">switch</span> window<span class="token punctuation">.</span>Type <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"absolute"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 绝对时间窗口</span></span>
<span class="line">        <span class="token keyword">return</span> eventTime<span class="token punctuation">.</span><span class="token function">Between</span><span class="token punctuation">(</span>window<span class="token punctuation">.</span>StartTime<span class="token punctuation">,</span> window<span class="token punctuation">.</span>EndTime<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"rolling"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 滚动窗口 (从活动开始算起)</span></span>
<span class="line">        <span class="token keyword">return</span> now<span class="token punctuation">.</span><span class="token function">Before</span><span class="token punctuation">(</span>window<span class="token punctuation">.</span>EndTime<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"daily"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 每日窗口</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>now<span class="token punctuation">.</span><span class="token function">Between</span><span class="token punctuation">(</span>window<span class="token punctuation">.</span>StartTime<span class="token punctuation">,</span> window<span class="token punctuation">.</span>EndTime<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">if</span> window<span class="token punctuation">.</span>DayStart <span class="token operator">!=</span> <span class="token string">""</span> <span class="token punctuation">{</span></span>
<span class="line">            start <span class="token operator">:=</span> <span class="token function">parseTimeOfDay</span><span class="token punctuation">(</span>now<span class="token punctuation">,</span> window<span class="token punctuation">.</span>DayStart<span class="token punctuation">)</span></span>
<span class="line">            end <span class="token operator">:=</span> <span class="token function">parseTimeOfDay</span><span class="token punctuation">(</span>now<span class="token punctuation">,</span> window<span class="token punctuation">.</span>DayEnd<span class="token punctuation">)</span></span>
<span class="line">            <span class="token keyword">return</span> now<span class="token punctuation">.</span><span class="token function">Between</span><span class="token punctuation">(</span>start<span class="token punctuation">,</span> end<span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"weekly"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 每周窗口</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>window<span class="token punctuation">.</span>WeekDays<span class="token punctuation">)</span> <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">            weekday <span class="token operator">:=</span> <span class="token function">int</span><span class="token punctuation">(</span>now<span class="token punctuation">.</span><span class="token function">Weekday</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">            <span class="token keyword">if</span> <span class="token operator">!</span><span class="token function">contains</span><span class="token punctuation">(</span>window<span class="token punctuation">.</span>WeekDays<span class="token punctuation">,</span> weekday<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">                <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"cron"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// Cron 表达式</span></span>
<span class="line">        <span class="token comment">// TODO: 实现 cron 匹配</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">default</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// matchAudience 检查受众规则</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>tm <span class="token operator">*</span>TriggerMatcher<span class="token punctuation">)</span> <span class="token function">matchAudience</span><span class="token punctuation">(</span>rules <span class="token operator">*</span>types<span class="token punctuation">.</span>AudienceRules<span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token builtin">bool</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> rules <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 黑名单检查</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Blacklist<span class="token punctuation">)</span> <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token function">contains</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Blacklist<span class="token punctuation">,</span> event<span class="token punctuation">.</span>PlayerID<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 白名单检查</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Whitelist<span class="token punctuation">)</span> <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span><span class="token function">contains</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Whitelist<span class="token punctuation">,</span> event<span class="token punctuation">.</span>PlayerID<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 平台检查</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Platforms<span class="token punctuation">)</span> <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span><span class="token function">contains</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Platforms<span class="token punctuation">,</span> event<span class="token punctuation">.</span>Platform<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 渠道检查</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Channels<span class="token punctuation">)</span> <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span><span class="token function">contains</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>Channels<span class="token punctuation">,</span> event<span class="token punctuation">.</span>Channel<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 服务器检查</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>ServerIds<span class="token punctuation">)</span> <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span><span class="token function">contains</span><span class="token punctuation">(</span>rules<span class="token punctuation">.</span>ServerIds<span class="token punctuation">,</span> event<span class="token punctuation">.</span>ServerID<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册时间检查</span></span>
<span class="line">    <span class="token keyword">if</span> rules<span class="token punctuation">.</span>RegisterAfter <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token operator">&amp;&amp;</span> event<span class="token punctuation">.</span>RegisterTime<span class="token punctuation">.</span><span class="token function">Before</span><span class="token punctuation">(</span><span class="token operator">*</span>rules<span class="token punctuation">.</span>RegisterAfter<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">if</span> rules<span class="token punctuation">.</span>RegisterBefore <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token operator">&amp;&amp;</span> event<span class="token punctuation">.</span>RegisterTime<span class="token punctuation">.</span><span class="token function">After</span><span class="token punctuation">(</span><span class="token operator">*</span>rules<span class="token punctuation">.</span>RegisterBefore<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// checkFrequencyCap 检查频率限制</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>tm <span class="token operator">*</span>TriggerMatcher<span class="token punctuation">)</span> <span class="token function">checkFrequencyCap</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> campaign <span class="token operator">*</span>types<span class="token punctuation">.</span>CampaignInstance<span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token builtin">bool</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">cap</span> <span class="token operator">:=</span> campaign<span class="token punctuation">.</span>TriggerConfig<span class="token punctuation">.</span>FrequencyCap</span>
<span class="line">    <span class="token keyword">if</span> <span class="token builtin">cap</span> <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token operator">||</span> <span class="token builtin">cap</span><span class="token punctuation">.</span>MaxCount <span class="token operator">&lt;</span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">switch</span> <span class="token builtin">cap</span><span class="token punctuation">.</span>Scope <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"global"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 全局限制</span></span>
<span class="line">        <span class="token keyword">if</span> campaign<span class="token punctuation">.</span>Stats<span class="token punctuation">.</span>TotalTriggers <span class="token operator">>=</span> <span class="token function">int64</span><span class="token punctuation">(</span><span class="token builtin">cap</span><span class="token punctuation">.</span>MaxCount<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"player"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 玩家限制</span></span>
<span class="line">        progress<span class="token punctuation">,</span> err <span class="token operator">:=</span> tm<span class="token punctuation">.</span>repo<span class="token punctuation">.</span><span class="token function">GetPlayerProgress</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> event<span class="token punctuation">.</span>PlayerID<span class="token punctuation">,</span> campaign<span class="token punctuation">.</span>ID<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> err <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token operator">&amp;&amp;</span> progress<span class="token punctuation">.</span>TriggerCount <span class="token operator">>=</span> <span class="token builtin">cap</span><span class="token punctuation">.</span>MaxCount <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"server"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 服务器限制</span></span>
<span class="line">        <span class="token comment">// TODO: 实现服务器级别计数</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"campaign"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 活动实例限制 (每个玩家每个活动只能触发 N 次)</span></span>
<span class="line">        progress<span class="token punctuation">,</span> err <span class="token operator">:=</span> tm<span class="token punctuation">.</span>repo<span class="token punctuation">.</span><span class="token function">GetPlayerProgress</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> event<span class="token punctuation">.</span>PlayerID<span class="token punctuation">,</span> campaign<span class="token punctuation">.</span>ID<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> err <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">switch</span> <span class="token builtin">cap</span><span class="token punctuation">.</span>Window <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">case</span> <span class="token string">"once"</span><span class="token punctuation">:</span></span>
<span class="line">                <span class="token keyword">return</span> progress<span class="token punctuation">.</span>TriggerCount <span class="token operator">==</span> <span class="token number">0</span></span>
<span class="line">            <span class="token keyword">case</span> <span class="token string">"daily"</span><span class="token punctuation">:</span></span>
<span class="line">                <span class="token comment">// 检查今天是否已触发</span></span>
<span class="line">                today <span class="token operator">:=</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Truncate</span><span class="token punctuation">(</span><span class="token number">24</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Hour<span class="token punctuation">)</span></span>
<span class="line">                <span class="token keyword">return</span> progress<span class="token punctuation">.</span>LastTrigger<span class="token punctuation">.</span><span class="token function">Before</span><span class="token punctuation">(</span>today<span class="token punctuation">)</span> <span class="token operator">||</span> progress<span class="token punctuation">.</span>TriggerCount <span class="token operator">==</span> <span class="token number">0</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_3-条件评估器-condition-evaluator" tabindex="-1"><a class="header-anchor" href="#_3-条件评估器-condition-evaluator"><span>3. 条件评估器 (Condition Evaluator)</span></a></h2>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/engine/condition.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> engine</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line">    <span class="token string">"log/slog"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/types"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/repository"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/evaluator"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ConditionEvaluator 条件评估器</span></span>
<span class="line"><span class="token keyword">type</span> ConditionEvaluator <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    repo          repository<span class="token punctuation">.</span>CampaignRepository</span>
<span class="line">    playerService PlayerService</span>
<span class="line">    evaluators    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>evaluator<span class="token punctuation">.</span>ConditionEvaluator</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// PlayerService 玩家服务接口</span></span>
<span class="line"><span class="token keyword">type</span> PlayerService <span class="token keyword">interface</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">GetPlayer</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> playerID <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token operator">*</span>Player<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token function">GetPlayerRecharge</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> playerID <span class="token builtin">string</span><span class="token punctuation">,</span> startTime<span class="token punctuation">,</span> endTime <span class="token builtin">int64</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token operator">*</span>RechargeInfo<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token function">GetPlayerHistory</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> playerID <span class="token builtin">string</span><span class="token punctuation">,</span> eventTypes <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span><span class="token punctuation">,</span> limit <span class="token builtin">int</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>Event<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Player 玩家信息</span></span>
<span class="line"><span class="token keyword">type</span> Player <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    PlayerID    <span class="token builtin">string</span></span>
<span class="line">    Level       <span class="token builtin">int32</span></span>
<span class="line">    VIPLevel    <span class="token builtin">int32</span></span>
<span class="line">    RegisterAt  <span class="token builtin">int64</span></span>
<span class="line">    TotalRecharge <span class="token builtin">int64</span></span>
<span class="line">    <span class="token comment">// 其他属性...</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Evaluate 评估条件组</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>ce <span class="token operator">*</span>ConditionEvaluator<span class="token punctuation">)</span> <span class="token function">Evaluate</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> groups <span class="token punctuation">[</span><span class="token punctuation">]</span>types<span class="token punctuation">.</span>ConditionGroup<span class="token punctuation">,</span> matched MatchedCampaign<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>groups<span class="token punctuation">)</span> <span class="token operator">==</span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">,</span> <span class="token boolean">nil</span> <span class="token comment">// 无条件则通过</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 构建评估上下文</span></span>
<span class="line">    evalCtx <span class="token operator">:=</span> ce<span class="token punctuation">.</span><span class="token function">buildContext</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> matched<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 组之间是 OR 关系 (任一组通过即可)</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> group <span class="token operator">:=</span> <span class="token keyword">range</span> groups <span class="token punctuation">{</span></span>
<span class="line">        passed<span class="token punctuation">,</span> err <span class="token operator">:=</span> ce<span class="token punctuation">.</span><span class="token function">evaluateGroup</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> group<span class="token punctuation">,</span> evalCtx<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">            slog<span class="token punctuation">.</span><span class="token function">Warn</span><span class="token punctuation">(</span><span class="token string">"condition group evaluation error"</span><span class="token punctuation">,</span></span>
<span class="line">                <span class="token string">"group"</span><span class="token punctuation">,</span> group<span class="token punctuation">.</span>ID<span class="token punctuation">,</span></span>
<span class="line">                <span class="token string">"error"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">if</span> passed <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// EvaluationContext 评估上下文</span></span>
<span class="line"><span class="token keyword">type</span> EvaluationContext <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Event       Event</span>
<span class="line">    Campaign    <span class="token operator">*</span>types<span class="token punctuation">.</span>CampaignInstance</span>
<span class="line">    Player      <span class="token operator">*</span>Player</span>
<span class="line">    Progress    <span class="token operator">*</span>types<span class="token punctuation">.</span>PlayerProgress</span>
<span class="line">    Variables   <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// evaluateGroup 评估单个条件组</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>ce <span class="token operator">*</span>ConditionEvaluator<span class="token punctuation">)</span> <span class="token function">evaluateGroup</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> group types<span class="token punctuation">.</span>ConditionGroup<span class="token punctuation">,</span> evalCtx <span class="token operator">*</span>EvaluationContext<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>group<span class="token punctuation">.</span>Conditions<span class="token punctuation">)</span> <span class="token operator">==</span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 组内条件根据 RequireAll 决定是 AND 还是 OR</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> cond <span class="token operator">:=</span> <span class="token keyword">range</span> group<span class="token punctuation">.</span>Conditions <span class="token punctuation">{</span></span>
<span class="line">        evaluator<span class="token punctuation">,</span> ok <span class="token operator">:=</span> ce<span class="token punctuation">.</span>evaluators<span class="token punctuation">[</span>cond<span class="token punctuation">.</span>Type<span class="token punctuation">]</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>ok <span class="token punctuation">{</span></span>
<span class="line">            slog<span class="token punctuation">.</span><span class="token function">Warn</span><span class="token punctuation">(</span><span class="token string">"unknown condition type"</span><span class="token punctuation">,</span> <span class="token string">"type"</span><span class="token punctuation">,</span> cond<span class="token punctuation">.</span>Type<span class="token punctuation">)</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"unknown condition type: %s"</span><span class="token punctuation">,</span> cond<span class="token punctuation">.</span>Type<span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        passed<span class="token punctuation">,</span> err <span class="token operator">:=</span> evaluator<span class="token punctuation">.</span><span class="token function">Evaluate</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> cond<span class="token punctuation">,</span> evalCtx<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> err</span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token keyword">if</span> group<span class="token punctuation">.</span>RequireAll <span class="token punctuation">{</span></span>
<span class="line">            <span class="token comment">// AND: 任何一个失败则整个组失败</span></span>
<span class="line">            <span class="token keyword">if</span> <span class="token operator">!</span>passed <span class="token punctuation">{</span></span>
<span class="line">                <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span> <span class="token keyword">else</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token comment">// OR: 任何一个成功则整个组成功</span></span>
<span class="line">            <span class="token keyword">if</span> passed <span class="token punctuation">{</span></span>
<span class="line">                <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 如果是 AND 且全部通过，返回 true</span></span>
<span class="line">    <span class="token comment">// 如果是 OR 且没有通过，返回 false</span></span>
<span class="line">    <span class="token keyword">return</span> group<span class="token punctuation">.</span>RequireAll<span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// buildContext 构建评估上下文</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>ce <span class="token operator">*</span>ConditionEvaluator<span class="token punctuation">)</span> <span class="token function">buildContext</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> matched MatchedCampaign<span class="token punctuation">)</span> <span class="token operator">*</span>EvaluationContext <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 获取玩家信息</span></span>
<span class="line">    player<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> ce<span class="token punctuation">.</span>playerService<span class="token punctuation">.</span><span class="token function">GetPlayer</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> matched<span class="token punctuation">.</span>Event<span class="token punctuation">.</span>PlayerID<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取玩家进度</span></span>
<span class="line">    progress<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> ce<span class="token punctuation">.</span>repo<span class="token punctuation">.</span><span class="token function">GetPlayerProgress</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> matched<span class="token punctuation">.</span>Event<span class="token punctuation">.</span>PlayerID<span class="token punctuation">,</span> matched<span class="token punctuation">.</span>Campaign<span class="token punctuation">.</span>ID<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token operator">&amp;</span>EvaluationContext<span class="token punctuation">{</span></span>
<span class="line">        Event<span class="token punctuation">:</span>     matched<span class="token punctuation">.</span>Event<span class="token punctuation">,</span></span>
<span class="line">        Campaign<span class="token punctuation">:</span>  matched<span class="token punctuation">.</span>Campaign<span class="token punctuation">,</span></span>
<span class="line">        Player<span class="token punctuation">:</span>    player<span class="token punctuation">,</span></span>
<span class="line">        Progress<span class="token punctuation">:</span>  progress<span class="token punctuation">,</span></span>
<span class="line">        Variables<span class="token punctuation">:</span> <span class="token function">make</span><span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-1-内置条件评估器" tabindex="-1"><a class="header-anchor" href="#_3-1-内置条件评估器"><span>3.1 内置条件评估器</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/evaluator/player_level.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> evaluator</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"strconv"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// PlayerLevelEvaluator 玩家等级评估器</span></span>
<span class="line"><span class="token keyword">type</span> PlayerLevelEvaluator <span class="token keyword">struct</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>e <span class="token operator">*</span>PlayerLevelEvaluator<span class="token punctuation">)</span> <span class="token function">Evaluate</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> cond types<span class="token punctuation">.</span>Condition<span class="token punctuation">,</span> evalCtx <span class="token operator">*</span>EvaluationContext<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> evalCtx<span class="token punctuation">.</span>Player <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    level <span class="token operator">:=</span> evalCtx<span class="token punctuation">.</span>Player<span class="token punctuation">.</span>Level</span>
<span class="line">    targetLevel <span class="token operator">:=</span> <span class="token function">parseInt</span><span class="token punctuation">(</span>cond<span class="token punctuation">.</span>Value<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token function">compareNumbers</span><span class="token punctuation">(</span><span class="token function">int64</span><span class="token punctuation">(</span>level<span class="token punctuation">)</span><span class="token punctuation">,</span> cond<span class="token punctuation">.</span>Operator<span class="token punctuation">,</span> targetLevel<span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// parseInt 解析整数</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">parseInt</span><span class="token punctuation">(</span>v <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span> <span class="token builtin">int64</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">switch</span> val <span class="token operator">:=</span> v<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token keyword">type</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">int</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">int64</span><span class="token punctuation">(</span>val<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">int32</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">int64</span><span class="token punctuation">(</span>val<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">int64</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> val</span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">float64</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">int64</span><span class="token punctuation">(</span>val<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">string</span><span class="token punctuation">:</span></span>
<span class="line">        i<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> strconv<span class="token punctuation">.</span><span class="token function">ParseInt</span><span class="token punctuation">(</span>val<span class="token punctuation">,</span> <span class="token number">10</span><span class="token punctuation">,</span> <span class="token number">64</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span> i</span>
<span class="line">    <span class="token keyword">default</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">0</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// compareNumbers 比较数字</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">compareNumbers</span><span class="token punctuation">(</span>a <span class="token builtin">int64</span><span class="token punctuation">,</span> op <span class="token builtin">string</span><span class="token punctuation">,</span> b <span class="token builtin">int64</span><span class="token punctuation">)</span> <span class="token builtin">bool</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">switch</span> op <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">">"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> a <span class="token operator">></span> b</span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">">="</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> a <span class="token operator">>=</span> b</span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"=="</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> a <span class="token operator">==</span> b</span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"!="</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> a <span class="token operator">!=</span> b</span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"&lt;"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> a <span class="token operator">&lt;</span> b</span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"&lt;="</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> a <span class="token operator">&lt;=</span> b</span>
<span class="line">    <span class="token keyword">default</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/evaluator/recharge_amount.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> evaluator</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"time"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// RechargeAmountEvaluator 累充金额评估器</span></span>
<span class="line"><span class="token keyword">type</span> RechargeAmountEvaluator <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    playerService PlayerService</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>e <span class="token operator">*</span>RechargeAmountEvaluator<span class="token punctuation">)</span> <span class="token function">Evaluate</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> cond types<span class="token punctuation">.</span>Condition<span class="token punctuation">,</span> evalCtx <span class="token operator">*</span>EvaluationContext<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 确定时间范围</span></span>
<span class="line">    <span class="token keyword">var</span> startTime<span class="token punctuation">,</span> endTime <span class="token builtin">int64</span></span>
<span class="line">    <span class="token keyword">if</span> window<span class="token punctuation">,</span> ok <span class="token operator">:=</span> cond<span class="token punctuation">.</span>Value<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span> ok <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// 从条件中获取时间窗口</span></span>
<span class="line">        <span class="token keyword">if</span> start<span class="token punctuation">,</span> ok <span class="token operator">:=</span> window<span class="token punctuation">[</span><span class="token string">"start_time"</span><span class="token punctuation">]</span><span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">int64</span><span class="token punctuation">)</span><span class="token punctuation">;</span> ok <span class="token punctuation">{</span></span>
<span class="line">            startTime <span class="token operator">=</span> start</span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">if</span> end<span class="token punctuation">,</span> ok <span class="token operator">:=</span> window<span class="token punctuation">[</span><span class="token string">"end_time"</span><span class="token punctuation">]</span><span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">int64</span><span class="token punctuation">)</span><span class="token punctuation">;</span> ok <span class="token punctuation">{</span></span>
<span class="line">            endTime <span class="token operator">=</span> end</span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">else</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// 默认: 从活动开始到现在</span></span>
<span class="line">        startTime <span class="token operator">=</span> evalCtx<span class="token punctuation">.</span>Campaign<span class="token punctuation">.</span>StartTime<span class="token punctuation">.</span><span class="token function">Unix</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">        endTime <span class="token operator">=</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Unix</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取充值信息</span></span>
<span class="line">    recharge<span class="token punctuation">,</span> err <span class="token operator">:=</span> e<span class="token punctuation">.</span>playerService<span class="token punctuation">.</span><span class="token function">GetPlayerRecharge</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> evalCtx<span class="token punctuation">.</span>Player<span class="token punctuation">.</span>PlayerID<span class="token punctuation">,</span> startTime<span class="token punctuation">,</span> endTime<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> err</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取目标金额</span></span>
<span class="line">    targetAmount <span class="token operator">:=</span> <span class="token function">parseInt</span><span class="token punctuation">(</span>cond<span class="token punctuation">.</span>Value<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token function">compareNumbers</span><span class="token punctuation">(</span>recharge<span class="token punctuation">.</span>TotalAmount<span class="token punctuation">,</span> cond<span class="token punctuation">.</span>Operator<span class="token punctuation">,</span> targetAmount<span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/evaluator/activity_progress.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> evaluator</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"encoding/json"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ActivityProgressEvaluator 活动进度评估器</span></span>
<span class="line"><span class="token keyword">type</span> ActivityProgressEvaluator <span class="token keyword">struct</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>e <span class="token operator">*</span>ActivityProgressEvaluator<span class="token punctuation">)</span> <span class="token function">Evaluate</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> cond types<span class="token punctuation">.</span>Condition<span class="token punctuation">,</span> evalCtx <span class="token operator">*</span>EvaluationContext<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> evalCtx<span class="token punctuation">.</span>Progress <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 从进度中获取指定字段的值</span></span>
<span class="line">    fieldName <span class="token operator">:=</span> cond<span class="token punctuation">.</span>Value<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">string</span><span class="token punctuation">)</span></span>
<span class="line">    value<span class="token punctuation">,</span> exists <span class="token operator">:=</span> evalCtx<span class="token punctuation">.</span>Progress<span class="token punctuation">.</span>Progress<span class="token punctuation">[</span>fieldName<span class="token punctuation">]</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token operator">!</span>exists <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 根据字段类型进行比较</span></span>
<span class="line">    <span class="token keyword">switch</span> v <span class="token operator">:=</span> value<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token keyword">type</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">float64</span><span class="token punctuation">:</span></span>
<span class="line">        target <span class="token operator">:=</span> <span class="token function">parseInt</span><span class="token punctuation">(</span>cond<span class="token punctuation">.</span>Value<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">compareNumbers</span><span class="token punctuation">(</span><span class="token function">int64</span><span class="token punctuation">(</span>v<span class="token punctuation">)</span><span class="token punctuation">,</span> cond<span class="token punctuation">.</span>Operator<span class="token punctuation">,</span> target<span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">int</span><span class="token punctuation">:</span></span>
<span class="line">        target <span class="token operator">:=</span> <span class="token function">parseInt</span><span class="token punctuation">(</span>cond<span class="token punctuation">.</span>Value<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">compareNumbers</span><span class="token punctuation">(</span><span class="token function">int64</span><span class="token punctuation">(</span>v<span class="token punctuation">)</span><span class="token punctuation">,</span> cond<span class="token punctuation">.</span>Operator<span class="token punctuation">,</span> target<span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token builtin">bool</span><span class="token punctuation">:</span></span>
<span class="line">        target<span class="token punctuation">,</span> ok <span class="token operator">:=</span> cond<span class="token punctuation">.</span>Value<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>ok <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">return</span> v <span class="token operator">==</span> target<span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token keyword">default</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token comment">// 字符串比较</span></span>
<span class="line">        targetStr<span class="token punctuation">,</span> ok <span class="token operator">:=</span> cond<span class="token punctuation">.</span>Value<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">string</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>ok <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">compareStrings</span><span class="token punctuation">(</span><span class="token function">jsonToString</span><span class="token punctuation">(</span>v<span class="token punctuation">)</span><span class="token punctuation">,</span> cond<span class="token punctuation">.</span>Operator<span class="token punctuation">,</span> targetStr<span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/evaluator/expression.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> evaluator</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/Knetic/govaluate"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ExpressionEvaluator 自定义表达式评估器</span></span>
<span class="line"><span class="token keyword">type</span> ExpressionEvaluator <span class="token keyword">struct</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>e <span class="token operator">*</span>ExpressionEvaluator<span class="token punctuation">)</span> <span class="token function">Evaluate</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> cond types<span class="token punctuation">.</span>Condition<span class="token punctuation">,</span> evalCtx <span class="token operator">*</span>EvaluationContext<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    exprStr<span class="token punctuation">,</span> ok <span class="token operator">:=</span> cond<span class="token punctuation">.</span>Value<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">string</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token operator">!</span>ok <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"expression must be string"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 创建表达式</span></span>
<span class="line">    expr<span class="token punctuation">,</span> err <span class="token operator">:=</span> govaluate<span class="token punctuation">.</span><span class="token function">NewEvaluableExpression</span><span class="token punctuation">(</span>exprStr<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"parse expression: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 构建参数</span></span>
<span class="line">    parameters <span class="token operator">:=</span> <span class="token function">make</span><span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 玩家属性</span></span>
<span class="line">    <span class="token keyword">if</span> evalCtx<span class="token punctuation">.</span>Player <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        parameters<span class="token punctuation">[</span><span class="token string">"player_level"</span><span class="token punctuation">]</span> <span class="token operator">=</span> evalCtx<span class="token punctuation">.</span>Player<span class="token punctuation">.</span>Level</span>
<span class="line">        parameters<span class="token punctuation">[</span><span class="token string">"vip_level"</span><span class="token punctuation">]</span> <span class="token operator">=</span> evalCtx<span class="token punctuation">.</span>Player<span class="token punctuation">.</span>VIPLevel</span>
<span class="line">        parameters<span class="token punctuation">[</span><span class="token string">"total_recharge"</span><span class="token punctuation">]</span> <span class="token operator">=</span> evalCtx<span class="token punctuation">.</span>Player<span class="token punctuation">.</span>TotalRecharge</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 活动进度</span></span>
<span class="line">    <span class="token keyword">if</span> evalCtx<span class="token punctuation">.</span>Progress <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">for</span> k<span class="token punctuation">,</span> v <span class="token operator">:=</span> <span class="token keyword">range</span> evalCtx<span class="token punctuation">.</span>Progress<span class="token punctuation">.</span>Progress <span class="token punctuation">{</span></span>
<span class="line">            parameters<span class="token punctuation">[</span>fmt<span class="token punctuation">.</span><span class="token function">Sprintf</span><span class="token punctuation">(</span><span class="token string">"progress_%s"</span><span class="token punctuation">,</span> k<span class="token punctuation">)</span><span class="token punctuation">]</span> <span class="token operator">=</span> v</span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 事件属性</span></span>
<span class="line">    parameters<span class="token punctuation">[</span><span class="token string">"event_type"</span><span class="token punctuation">]</span> <span class="token operator">=</span> evalCtx<span class="token punctuation">.</span>Event<span class="token punctuation">.</span>EventType</span>
<span class="line">    <span class="token keyword">for</span> k<span class="token punctuation">,</span> v <span class="token operator">:=</span> <span class="token keyword">range</span> evalCtx<span class="token punctuation">.</span>Event<span class="token punctuation">.</span>Properties <span class="token punctuation">{</span></span>
<span class="line">        parameters<span class="token punctuation">[</span>fmt<span class="token punctuation">.</span><span class="token function">Sprintf</span><span class="token punctuation">(</span><span class="token string">"event_%s"</span><span class="token punctuation">,</span> k<span class="token punctuation">)</span><span class="token punctuation">]</span> <span class="token operator">=</span> v</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 评估表达式</span></span>
<span class="line">    result<span class="token punctuation">,</span> err <span class="token operator">:=</span> expr<span class="token punctuation">.</span><span class="token function">Evaluate</span><span class="token punctuation">(</span>parameters<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">,</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"evaluate expression: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    boolResult<span class="token punctuation">,</span> ok <span class="token operator">:=</span> result<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">bool</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> boolResult<span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_4-动作执行器-action-executor" tabindex="-1"><a class="header-anchor" href="#_4-动作执行器-action-executor"><span>4. 动作执行器 (Action Executor)</span></a></h2>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/engine/action.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> engine</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"log/slog"</span></span>
<span class="line">    <span class="token string">"sync"</span></span>
<span class="line">    <span class="token string">"time"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/types"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ActionExecutor 动作执行器</span></span>
<span class="line"><span class="token keyword">type</span> ActionExecutor <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    executors    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>ActionHandler</span>
<span class="line">    gameClient   GameServiceClient</span>
<span class="line">    notification NotificationService</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ActionHandler 动作处理器接口</span></span>
<span class="line"><span class="token keyword">type</span> ActionHandler <span class="token keyword">interface</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">Execute</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> action types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line">    <span class="token function">Validate</span><span class="token punctuation">(</span>action types<span class="token punctuation">.</span>Action<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ExecutionContext 执行上下文</span></span>
<span class="line"><span class="token keyword">type</span> ExecutionContext <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Event        Event</span>
<span class="line">    Campaign     <span class="token operator">*</span>types<span class="token punctuation">.</span>CampaignInstance</span>
<span class="line">    PlayerID     <span class="token builtin">string</span></span>
<span class="line">    Progress     <span class="token operator">*</span>types<span class="token punctuation">.</span>PlayerProgress</span>
<span class="line">    Results      <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span>  <span class="token comment">// 动作执行结果</span></span>
<span class="line">    Variables    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span>  <span class="token comment">// 可在动作间传递的变量</span></span>
<span class="line">    StartTime    time<span class="token punctuation">.</span>Time</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ExecuteResult 执行结果</span></span>
<span class="line"><span class="token keyword">type</span> ExecuteResult <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    ActionID     <span class="token builtin">string</span></span>
<span class="line">    Success      <span class="token builtin">bool</span></span>
<span class="line">    Error        <span class="token builtin">error</span></span>
<span class="line">    Result       <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">    DurationMs   <span class="token builtin">int64</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Execute 执行动作列表</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>ae <span class="token operator">*</span>ActionExecutor<span class="token punctuation">)</span> <span class="token function">Execute</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> actions <span class="token punctuation">[</span><span class="token punctuation">]</span>types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>ExecuteResult<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    results <span class="token operator">:=</span> <span class="token function">make</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>ExecuteResult<span class="token punctuation">,</span> <span class="token number">0</span><span class="token punctuation">,</span> <span class="token function">len</span><span class="token punctuation">(</span>actions<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 构建动作依赖图</span></span>
<span class="line">    dag <span class="token operator">:=</span> ae<span class="token punctuation">.</span><span class="token function">buildDAG</span><span class="token punctuation">(</span>actions<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 按依赖顺序执行</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> layer <span class="token operator">:=</span> <span class="token keyword">range</span> dag<span class="token punctuation">.</span><span class="token function">TopologicalSort</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        layerResults <span class="token operator">:=</span> ae<span class="token punctuation">.</span><span class="token function">executeLayer</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> layer<span class="token punctuation">,</span> execCtx<span class="token punctuation">)</span></span>
<span class="line">        results <span class="token operator">=</span> <span class="token function">append</span><span class="token punctuation">(</span>results<span class="token punctuation">,</span> layerResults<span class="token operator">...</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 检查是否有失败</span></span>
<span class="line">        <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> r <span class="token operator">:=</span> <span class="token keyword">range</span> layerResults <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">if</span> <span class="token operator">!</span>r<span class="token punctuation">.</span>Success <span class="token punctuation">{</span></span>
<span class="line">                slog<span class="token punctuation">.</span><span class="token function">Error</span><span class="token punctuation">(</span><span class="token string">"action execution failed"</span><span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"action_id"</span><span class="token punctuation">,</span> r<span class="token punctuation">.</span>ActionID<span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"error"</span><span class="token punctuation">,</span> r<span class="token punctuation">.</span>Error<span class="token punctuation">)</span></span>
<span class="line">                <span class="token comment">// 可以选择继续或中止</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> results<span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// executeLayer 执行一层动作（并行执行）</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>ae <span class="token operator">*</span>ActionExecutor<span class="token punctuation">)</span> <span class="token function">executeLayer</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> layer <span class="token punctuation">[</span><span class="token punctuation">]</span>types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token punctuation">[</span><span class="token punctuation">]</span>ExecuteResult <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">var</span> wg sync<span class="token punctuation">.</span>WaitGroup</span>
<span class="line">    results <span class="token operator">:=</span> <span class="token function">make</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>ExecuteResult<span class="token punctuation">,</span> <span class="token function">len</span><span class="token punctuation">(</span>layer<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">    resultChan <span class="token operator">:=</span> <span class="token function">make</span><span class="token punctuation">(</span><span class="token keyword">chan</span> ExecuteResult<span class="token punctuation">,</span> <span class="token function">len</span><span class="token punctuation">(</span>layer<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">for</span> i<span class="token punctuation">,</span> action <span class="token operator">:=</span> <span class="token keyword">range</span> layer <span class="token punctuation">{</span></span>
<span class="line">        wg<span class="token punctuation">.</span><span class="token function">Add</span><span class="token punctuation">(</span><span class="token number">1</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">go</span> <span class="token keyword">func</span><span class="token punctuation">(</span>idx <span class="token builtin">int</span><span class="token punctuation">,</span> a types<span class="token punctuation">.</span>Action<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">defer</span> wg<span class="token punctuation">.</span><span class="token function">Done</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">            <span class="token comment">// 延迟执行</span></span>
<span class="line">            <span class="token keyword">if</span> a<span class="token punctuation">.</span>DelayMs <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">                time<span class="token punctuation">.</span><span class="token function">Sleep</span><span class="token punctuation">(</span>time<span class="token punctuation">.</span><span class="token function">Duration</span><span class="token punctuation">(</span>a<span class="token punctuation">.</span>DelayMs<span class="token punctuation">)</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Millisecond<span class="token punctuation">)</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">            start <span class="token operator">:=</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">            result <span class="token operator">:=</span> ExecuteResult<span class="token punctuation">{</span></span>
<span class="line">                ActionID<span class="token punctuation">:</span> a<span class="token punctuation">.</span>ID<span class="token punctuation">,</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">            <span class="token comment">// 查找处理器</span></span>
<span class="line">            handler<span class="token punctuation">,</span> ok <span class="token operator">:=</span> ae<span class="token punctuation">.</span>executors<span class="token punctuation">[</span>a<span class="token punctuation">.</span>Type<span class="token punctuation">]</span></span>
<span class="line">            <span class="token keyword">if</span> <span class="token operator">!</span>ok <span class="token punctuation">{</span></span>
<span class="line">                result<span class="token punctuation">.</span>Success <span class="token operator">=</span> <span class="token boolean">false</span></span>
<span class="line">                result<span class="token punctuation">.</span>Error <span class="token operator">=</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"unknown action type: %s"</span><span class="token punctuation">,</span> a<span class="token punctuation">.</span>Type<span class="token punctuation">)</span></span>
<span class="line">                resultChan <span class="token operator">&lt;-</span> result</span>
<span class="line">                <span class="token keyword">return</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">            <span class="token comment">// 执行动作（带重试）</span></span>
<span class="line">            err <span class="token operator">:=</span> ae<span class="token punctuation">.</span><span class="token function">executeWithRetry</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> a<span class="token punctuation">,</span> handler<span class="token punctuation">,</span> execCtx<span class="token punctuation">)</span></span>
<span class="line">            result<span class="token punctuation">.</span>Success <span class="token operator">=</span> <span class="token punctuation">(</span>err <span class="token operator">==</span> <span class="token boolean">nil</span><span class="token punctuation">)</span></span>
<span class="line">            result<span class="token punctuation">.</span>Error <span class="token operator">=</span> err</span>
<span class="line">            result<span class="token punctuation">.</span>DurationMs <span class="token operator">=</span> time<span class="token punctuation">.</span><span class="token function">Since</span><span class="token punctuation">(</span>start<span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Milliseconds</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">            resultChan <span class="token operator">&lt;-</span> result</span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">(</span>i<span class="token punctuation">,</span> action<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">go</span> <span class="token keyword">func</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        wg<span class="token punctuation">.</span><span class="token function">Wait</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token function">close</span><span class="token punctuation">(</span>resultChan<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    i <span class="token operator">:=</span> <span class="token number">0</span></span>
<span class="line">    <span class="token keyword">for</span> result <span class="token operator">:=</span> <span class="token keyword">range</span> resultChan <span class="token punctuation">{</span></span>
<span class="line">        results<span class="token punctuation">[</span>i<span class="token punctuation">]</span> <span class="token operator">=</span> result</span>
<span class="line">        i<span class="token operator">++</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> results</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// executeWithRetry 带重试的执行</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>ae <span class="token operator">*</span>ActionExecutor<span class="token punctuation">)</span> <span class="token function">executeWithRetry</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> action types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> handler ActionHandler<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">var</span> err <span class="token builtin">error</span></span>
<span class="line">    maxRetries <span class="token operator">:=</span> <span class="token number">0</span></span>
<span class="line">    intervalMs <span class="token operator">:=</span> <span class="token number">0</span></span>
<span class="line">    backoffRate <span class="token operator">:=</span> <span class="token number">1.0</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> action<span class="token punctuation">.</span>RetryConfig <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        maxRetries <span class="token operator">=</span> action<span class="token punctuation">.</span>RetryConfig<span class="token punctuation">.</span>MaxRetries</span>
<span class="line">        intervalMs <span class="token operator">=</span> action<span class="token punctuation">.</span>RetryConfig<span class="token punctuation">.</span>IntervalMs</span>
<span class="line">        backoffRate <span class="token operator">=</span> action<span class="token punctuation">.</span>RetryConfig<span class="token punctuation">.</span>BackoffRate</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">for</span> attempt <span class="token operator">:=</span> <span class="token number">0</span><span class="token punctuation">;</span> attempt <span class="token operator">&lt;=</span> maxRetries<span class="token punctuation">;</span> attempt<span class="token operator">++</span> <span class="token punctuation">{</span></span>
<span class="line">        err <span class="token operator">=</span> handler<span class="token punctuation">.</span><span class="token function">Execute</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> action<span class="token punctuation">,</span> execCtx<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> err <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token keyword">if</span> attempt <span class="token operator">&lt;</span> maxRetries <span class="token punctuation">{</span></span>
<span class="line">            sleepMs <span class="token operator">:=</span> <span class="token function">int</span><span class="token punctuation">(</span><span class="token function">float64</span><span class="token punctuation">(</span>intervalMs<span class="token punctuation">)</span> <span class="token operator">*</span> <span class="token function">pow</span><span class="token punctuation">(</span>backoffRate<span class="token punctuation">,</span> <span class="token function">float64</span><span class="token punctuation">(</span>attempt<span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">            time<span class="token punctuation">.</span><span class="token function">Sleep</span><span class="token punctuation">(</span>time<span class="token punctuation">.</span><span class="token function">Duration</span><span class="token punctuation">(</span>sleepMs<span class="token punctuation">)</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Millisecond<span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> err</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-1-内置动作处理器" tabindex="-1"><a class="header-anchor" href="#_4-1-内置动作处理器"><span>4.1 内置动作处理器</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/actions/grant_reward.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> actions</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"encoding/json"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/types"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// GrantRewardHandler 奖励发放处理器</span></span>
<span class="line"><span class="token keyword">type</span> GrantRewardHandler <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    gameClient GameServiceClient</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> GrantRewardParams <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Rewards <span class="token punctuation">[</span><span class="token punctuation">]</span>RewardItem <span class="token string">`json:"rewards"`</span></span>
<span class="line">    Mail    <span class="token operator">*</span>MailReward  <span class="token string">`json:"mail,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> RewardItem <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Type     <span class="token builtin">string</span> <span class="token string">`json:"type"`</span>     <span class="token comment">// item/currency/title/exp</span></span>
<span class="line">    ID       <span class="token builtin">string</span> <span class="token string">`json:"id"`</span>       <span class="token comment">// 物品ID / 货币类型</span></span>
<span class="line">    Count    <span class="token builtin">int64</span>  <span class="token string">`json:"count"`</span>    <span class="token comment">// 数量</span></span>
<span class="line">    Quality  <span class="token builtin">string</span> <span class="token string">`json:"quality"`</span>  <span class="token comment">// 品质</span></span>
<span class="line">    Bind     <span class="token builtin">bool</span>   <span class="token string">`json:"bind"`</span>     <span class="token comment">// 是否绑定</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> MailReward <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Title    <span class="token builtin">string</span> <span class="token string">`json:"title"`</span></span>
<span class="line">    Content  <span class="token builtin">string</span> <span class="token string">`json:"content"`</span></span>
<span class="line">    Sender   <span class="token builtin">string</span> <span class="token string">`json:"sender"`</span></span>
<span class="line">    ExpireDays <span class="token builtin">int32</span> <span class="token string">`json:"expire_days"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>h <span class="token operator">*</span>GrantRewardHandler<span class="token punctuation">)</span> <span class="token function">Execute</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> action types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">var</span> params GrantRewardParams</span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> json<span class="token punctuation">.</span><span class="token function">Unmarshal</span><span class="token punctuation">(</span>action<span class="token punctuation">.</span>Params<span class="token punctuation">,</span> <span class="token operator">&amp;</span>params<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"parse params: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 调用游戏服务发放奖励</span></span>
<span class="line">    request <span class="token operator">:=</span> <span class="token operator">&amp;</span>GrantRewardRequest<span class="token punctuation">{</span></span>
<span class="line">        PlayerID<span class="token punctuation">:</span>  execCtx<span class="token punctuation">.</span>PlayerID<span class="token punctuation">,</span></span>
<span class="line">        Rewards<span class="token punctuation">:</span>   params<span class="token punctuation">.</span>Rewards<span class="token punctuation">,</span></span>
<span class="line">        CampaignID<span class="token punctuation">:</span> execCtx<span class="token punctuation">.</span>Campaign<span class="token punctuation">.</span>ID<span class="token punctuation">,</span></span>
<span class="line">        ActionID<span class="token punctuation">:</span>  action<span class="token punctuation">.</span>ID<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 如果配置了邮件，则通过邮件发放</span></span>
<span class="line">    <span class="token keyword">if</span> params<span class="token punctuation">.</span>Mail <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        request<span class="token punctuation">.</span>MailTitle <span class="token operator">=</span> params<span class="token punctuation">.</span>Mail<span class="token punctuation">.</span>Title</span>
<span class="line">        request<span class="token punctuation">.</span>MailContent <span class="token operator">=</span> params<span class="token punctuation">.</span>Mail<span class="token punctuation">.</span>Content</span>
<span class="line">        request<span class="token punctuation">.</span>MailSender <span class="token operator">=</span> params<span class="token punctuation">.</span>Mail<span class="token punctuation">.</span>Sender</span>
<span class="line">        request<span class="token punctuation">.</span>MailExpireDays <span class="token operator">=</span> params<span class="token punctuation">.</span>Mail<span class="token punctuation">.</span>ExpireDays</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> h<span class="token punctuation">.</span>gameClient<span class="token punctuation">.</span><span class="token function">GrantReward</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> request<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/actions/send_notification.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> actions</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"encoding/json"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/types"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// SendNotificationHandler 通知发送处理器</span></span>
<span class="line"><span class="token keyword">type</span> SendNotificationHandler <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    notificationService NotificationService</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> NotificationParams <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Type     <span class="token builtin">string</span>                 <span class="token string">`json:"type"`</span>     <span class="token comment">// mail/popup/red_dot/toast</span></span>
<span class="line">    Title    <span class="token builtin">string</span>                 <span class="token string">`json:"title"`</span></span>
<span class="line">    Content  <span class="token builtin">string</span>                 <span class="token string">`json:"content"`</span></span>
<span class="line">    Icon     <span class="token builtin">string</span>                 <span class="token string">`json:"icon"`</span></span>
<span class="line">    Action   <span class="token operator">*</span>NotificationAction    <span class="token string">`json:"action,omitempty"`</span></span>
<span class="line">    Extra    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"extra,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> NotificationAction <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Type <span class="token builtin">string</span> <span class="token string">`json:"type"`</span> <span class="token comment">// navigate/open_panel/quest</span></span>
<span class="line">    Data <span class="token builtin">string</span> <span class="token string">`json:"data"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>h <span class="token operator">*</span>SendNotificationHandler<span class="token punctuation">)</span> <span class="token function">Execute</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> action types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">var</span> params NotificationParams</span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> json<span class="token punctuation">.</span><span class="token function">Unmarshal</span><span class="token punctuation">(</span>action<span class="token punctuation">.</span>Params<span class="token punctuation">,</span> <span class="token operator">&amp;</span>params<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"parse params: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 替换模板变量</span></span>
<span class="line">    content <span class="token operator">:=</span> h<span class="token punctuation">.</span><span class="token function">replaceVariables</span><span class="token punctuation">(</span>params<span class="token punctuation">.</span>Content<span class="token punctuation">,</span> execCtx<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    request <span class="token operator">:=</span> <span class="token operator">&amp;</span>SendNotificationRequest<span class="token punctuation">{</span></span>
<span class="line">        PlayerID<span class="token punctuation">:</span> execCtx<span class="token punctuation">.</span>PlayerID<span class="token punctuation">,</span></span>
<span class="line">        Type<span class="token punctuation">:</span>     params<span class="token punctuation">.</span>Type<span class="token punctuation">,</span></span>
<span class="line">        Title<span class="token punctuation">:</span>    params<span class="token punctuation">.</span>Title<span class="token punctuation">,</span></span>
<span class="line">        Content<span class="token punctuation">:</span>  content<span class="token punctuation">,</span></span>
<span class="line">        Icon<span class="token punctuation">:</span>     params<span class="token punctuation">.</span>Icon<span class="token punctuation">,</span></span>
<span class="line">        Action<span class="token punctuation">:</span>   params<span class="token punctuation">.</span>Action<span class="token punctuation">,</span></span>
<span class="line">        Extra<span class="token punctuation">:</span>    params<span class="token punctuation">.</span>Extra<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> h<span class="token punctuation">.</span>notificationService<span class="token punctuation">.</span><span class="token function">Send</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> request<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/actions/update_progress.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> actions</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"encoding/json"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/types"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/repository"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// UpdateProgressHandler 进度更新处理器</span></span>
<span class="line"><span class="token keyword">type</span> UpdateProgressHandler <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    repo repository<span class="token punctuation">.</span>CampaignRepository</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> UpdateProgressParams <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Operations <span class="token punctuation">[</span><span class="token punctuation">]</span>ProgressOperation <span class="token string">`json:"operations"`</span></span>
<span class="line">    Stage      <span class="token operator">*</span>StageOperation     <span class="token string">`json:"stage,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> ProgressOperation <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Op    <span class="token builtin">string</span>      <span class="token string">`json:"op"`</span>    <span class="token comment">// set/add/multiply/max</span></span>
<span class="line">    Field <span class="token builtin">string</span>      <span class="token string">`json:"field"`</span></span>
<span class="line">    Value <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"value"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> StageOperation <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Op    <span class="token builtin">string</span> <span class="token string">`json:"op"`</span>    <span class="token comment">// set/increment</span></span>
<span class="line">    Value <span class="token builtin">int32</span>  <span class="token string">`json:"value"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>h <span class="token operator">*</span>UpdateProgressHandler<span class="token punctuation">)</span> <span class="token function">Execute</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> action types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">var</span> params UpdateProgressParams</span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> json<span class="token punctuation">.</span><span class="token function">Unmarshal</span><span class="token punctuation">(</span>action<span class="token punctuation">.</span>Params<span class="token punctuation">,</span> <span class="token operator">&amp;</span>params<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"parse params: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    progress <span class="token operator">:=</span> execCtx<span class="token punctuation">.</span>Progress</span>
<span class="line">    <span class="token keyword">if</span> progress <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        progress <span class="token operator">=</span> <span class="token operator">&amp;</span>types<span class="token punctuation">.</span>PlayerProgress<span class="token punctuation">{</span></span>
<span class="line">            PlayerID<span class="token punctuation">:</span>   execCtx<span class="token punctuation">.</span>PlayerID<span class="token punctuation">,</span></span>
<span class="line">            CampaignID<span class="token punctuation">:</span> execCtx<span class="token punctuation">.</span>Campaign<span class="token punctuation">.</span>ID<span class="token punctuation">,</span></span>
<span class="line">            Progress<span class="token punctuation">:</span>   <span class="token function">make</span><span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 执行字段操作</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> op <span class="token operator">:=</span> <span class="token keyword">range</span> params<span class="token punctuation">.</span>Operations <span class="token punctuation">{</span></span>
<span class="line">        h<span class="token punctuation">.</span><span class="token function">applyOperation</span><span class="token punctuation">(</span>progress<span class="token punctuation">.</span>Progress<span class="token punctuation">,</span> op<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 执行阶段操作</span></span>
<span class="line">    <span class="token keyword">if</span> params<span class="token punctuation">.</span>Stage <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">switch</span> params<span class="token punctuation">.</span>Stage<span class="token punctuation">.</span>Op <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">case</span> <span class="token string">"set"</span><span class="token punctuation">:</span></span>
<span class="line">            progress<span class="token punctuation">.</span>Stage <span class="token operator">=</span> params<span class="token punctuation">.</span>Stage<span class="token punctuation">.</span>Value</span>
<span class="line">        <span class="token keyword">case</span> <span class="token string">"increment"</span><span class="token punctuation">:</span></span>
<span class="line">            progress<span class="token punctuation">.</span>Stage <span class="token operator">+=</span> params<span class="token punctuation">.</span>Stage<span class="token punctuation">.</span>Value</span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 保存进度</span></span>
<span class="line">    progress<span class="token punctuation">.</span>UpdatedAt <span class="token operator">=</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> h<span class="token punctuation">.</span>repo<span class="token punctuation">.</span><span class="token function">SavePlayerProgress</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> progress<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>h <span class="token operator">*</span>UpdateProgressHandler<span class="token punctuation">)</span> <span class="token function">applyOperation</span><span class="token punctuation">(</span>progress <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">,</span> op ProgressOperation<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    current <span class="token operator">:=</span> progress<span class="token punctuation">[</span>op<span class="token punctuation">.</span>Field<span class="token punctuation">]</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">switch</span> op<span class="token punctuation">.</span>Op <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"set"</span><span class="token punctuation">:</span></span>
<span class="line">        progress<span class="token punctuation">[</span>op<span class="token punctuation">.</span>Field<span class="token punctuation">]</span> <span class="token operator">=</span> op<span class="token punctuation">.</span>Value</span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"add"</span><span class="token punctuation">:</span></span>
<span class="line">        progress<span class="token punctuation">[</span>op<span class="token punctuation">.</span>Field<span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token function">toFloat64</span><span class="token punctuation">(</span>current<span class="token punctuation">)</span> <span class="token operator">+</span> <span class="token function">toFloat64</span><span class="token punctuation">(</span>op<span class="token punctuation">.</span>Value<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"multiply"</span><span class="token punctuation">:</span></span>
<span class="line">        progress<span class="token punctuation">[</span>op<span class="token punctuation">.</span>Field<span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token function">toFloat64</span><span class="token punctuation">(</span>current<span class="token punctuation">)</span> <span class="token operator">*</span> <span class="token function">toFloat64</span><span class="token punctuation">(</span>op<span class="token punctuation">.</span>Value<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">case</span> <span class="token string">"max"</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token function">toFloat64</span><span class="token punctuation">(</span>op<span class="token punctuation">.</span>Value<span class="token punctuation">)</span> <span class="token operator">></span> <span class="token function">toFloat64</span><span class="token punctuation">(</span>current<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            progress<span class="token punctuation">[</span>op<span class="token punctuation">.</span>Field<span class="token punctuation">]</span> <span class="token operator">=</span> op<span class="token punctuation">.</span>Value</span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/campaign/actions/custom_rpc.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> actions</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"encoding/json"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/campaign/types"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// CustomRPCHandler 自定义 RPC 调用处理器</span></span>
<span class="line"><span class="token keyword">type</span> CustomRPCHandler <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    functionInvoker FunctionInvoker</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> CustomRPCParams <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    FunctionID <span class="token builtin">string</span>                 <span class="token string">`json:"function_id"`</span></span>
<span class="line">    Payload    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`json:"payload"`</span></span>
<span class="line">    TimeoutMs  <span class="token builtin">int32</span>                  <span class="token string">`json:"timeout_ms"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>h <span class="token operator">*</span>CustomRPCHandler<span class="token punctuation">)</span> <span class="token function">Execute</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> action types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">var</span> params CustomRPCParams</span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> json<span class="token punctuation">.</span><span class="token function">Unmarshal</span><span class="token punctuation">(</span>action<span class="token punctuation">.</span>Params<span class="token punctuation">,</span> <span class="token operator">&amp;</span>params<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"parse params: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 构建调用上下文</span></span>
<span class="line">    payload <span class="token operator">:=</span> <span class="token function">make</span><span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">for</span> k<span class="token punctuation">,</span> v <span class="token operator">:=</span> <span class="token keyword">range</span> params<span class="token punctuation">.</span>Payload <span class="token punctuation">{</span></span>
<span class="line">        payload<span class="token punctuation">[</span>k<span class="token punctuation">]</span> <span class="token operator">=</span> h<span class="token punctuation">.</span><span class="token function">replaceVariables</span><span class="token punctuation">(</span>v<span class="token punctuation">,</span> execCtx<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 添加活动上下文</span></span>
<span class="line">    payload<span class="token punctuation">[</span><span class="token string">"campaign_id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> execCtx<span class="token punctuation">.</span>Campaign<span class="token punctuation">.</span>ID</span>
<span class="line">    payload<span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> execCtx<span class="token punctuation">.</span>PlayerID</span>
<span class="line">    payload<span class="token punctuation">[</span><span class="token string">"event_type"</span><span class="token punctuation">]</span> <span class="token operator">=</span> execCtx<span class="token punctuation">.</span>Event<span class="token punctuation">.</span>EventType</span>
<span class="line">    payload<span class="token punctuation">[</span><span class="token string">"event_properties"</span><span class="token punctuation">]</span> <span class="token operator">=</span> execCtx<span class="token punctuation">.</span>Event<span class="token punctuation">.</span>Properties</span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 调用函数</span></span>
<span class="line">    request <span class="token operator">:=</span> <span class="token operator">&amp;</span>InvokeRequest<span class="token punctuation">{</span></span>
<span class="line">        FunctionID<span class="token punctuation">:</span> params<span class="token punctuation">.</span>FunctionID<span class="token punctuation">,</span></span>
<span class="line">        Payload<span class="token punctuation">:</span>    payload<span class="token punctuation">,</span></span>
<span class="line">        GameID<span class="token punctuation">:</span>     execCtx<span class="token punctuation">.</span>Campaign<span class="token punctuation">.</span>GameID<span class="token punctuation">,</span></span>
<span class="line">        Env<span class="token punctuation">:</span>        execCtx<span class="token punctuation">.</span>Campaign<span class="token punctuation">.</span>Env<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> params<span class="token punctuation">.</span>TimeoutMs <span class="token operator">></span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">var</span> cancel context<span class="token punctuation">.</span>CancelFunc</span>
<span class="line">        ctx<span class="token punctuation">,</span> cancel <span class="token operator">=</span> context<span class="token punctuation">.</span><span class="token function">WithTimeout</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> time<span class="token punctuation">.</span><span class="token function">Duration</span><span class="token punctuation">(</span>params<span class="token punctuation">.</span>TimeoutMs<span class="token punctuation">)</span><span class="token operator">*</span>time<span class="token punctuation">.</span>Millisecond<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">defer</span> <span class="token function">cancel</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    response<span class="token punctuation">,</span> err <span class="token operator">:=</span> h<span class="token punctuation">.</span>functionInvoker<span class="token punctuation">.</span><span class="token function">Invoke</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> request<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"invoke function: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 保存结果到执行上下文</span></span>
<span class="line">    execCtx<span class="token punctuation">.</span>Results<span class="token punctuation">[</span>action<span class="token punctuation">.</span>ID<span class="token punctuation">]</span> <span class="token operator">=</span> response<span class="token punctuation">.</span>Result</span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_5-常见活动类型模板" tabindex="-1"><a class="header-anchor" href="#_5-常见活动类型模板"><span>5. 常见活动类型模板</span></a></h2>
<h3 id="_5-1-签到活动" tabindex="-1"><a class="header-anchor" href="#_5-1-签到活动"><span>5.1 签到活动</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"daily_check_in"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"每日签到"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"login"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"trigger_config"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"event_types"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.login"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"frequency_cap"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"scope"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"max_count"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"window"</span><span class="token operator">:</span> <span class="token string">"daily"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"condition_groups"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"grant_reward"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"grant_reward"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"rewards"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">          <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"currency"</span><span class="token punctuation">,</span> <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"gold"</span><span class="token punctuation">,</span> <span class="token property">"count"</span><span class="token operator">:</span> <span class="token number">100</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-2-累充活动" tabindex="-1"><a class="header-anchor" href="#_5-2-累充活动"><span>5.2 累充活动</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"recharge_milestone"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"累充活动"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"recharge"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"trigger_config"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"event_types"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"payment.success"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"frequency_cap"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"scope"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"max_count"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"window"</span><span class="token operator">:</span> <span class="token string">"activity"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"condition_groups"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"check_recharge_amount"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"logic_operator"</span><span class="token operator">:</span> <span class="token string">"AND"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"conditions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"total_recharge"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"recharge_amount"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"operator"</span><span class="token operator">:</span> <span class="token string">">="</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"value"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"min_amount"</span><span class="token operator">:</span> <span class="token number">10000</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"grant_milestone_reward"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"grant_reward"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"rewards"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">          <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"item"</span><span class="token punctuation">,</span> <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"rare_weapon"</span><span class="token punctuation">,</span> <span class="token property">"count"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"mail"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"累充奖励"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"content"</span><span class="token operator">:</span> <span class="token string">"恭喜达成累充目标！"</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-3-任务活动" tabindex="-1"><a class="header-anchor" href="#_5-3-任务活动"><span>5.3 任务活动</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"daily_task"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"每日任务"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"quest"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"trigger_config"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"event_types"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"quest.complete"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"event_filter"</span><span class="token operator">:</span> <span class="token string">"$.props.quest_id == 'daily_boss_kill'"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"condition_groups"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"check_player_level"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"conditions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"min_level"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"player_level"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"operator"</span><span class="token operator">:</span> <span class="token string">">="</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"value"</span><span class="token operator">:</span> <span class="token number">20</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"update_progress"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"update_progress"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">          <span class="token punctuation">{</span><span class="token property">"op"</span><span class="token operator">:</span> <span class="token string">"add"</span><span class="token punctuation">,</span> <span class="token property">"field"</span><span class="token operator">:</span> <span class="token string">"daily_task_count"</span><span class="token punctuation">,</span> <span class="token property">"value"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"check_complete"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"update_progress"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"dependency"</span><span class="token operator">:</span> <span class="token string">"update_progress"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"stage"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"op"</span><span class="token operator">:</span> <span class="token string">"increment"</span><span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"condition"</span><span class="token operator">:</span> <span class="token string">"$.progress.daily_task_count >= 3"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-4-首充活动" tabindex="-1"><a class="header-anchor" href="#_5-4-首充活动"><span>5.4 首充活动</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"first_recharge"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"首充大礼包"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"recharge"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"trigger_config"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"event_types"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"payment.success"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"frequency_cap"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"scope"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"max_count"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"window"</span><span class="token operator">:</span> <span class="token string">"once"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"condition_groups"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"check_first_recharge"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"conditions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"recharge_count"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"recharge_count"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"operator"</span><span class="token operator">:</span> <span class="token string">"=="</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"value"</span><span class="token operator">:</span> <span class="token number">1</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"grant_first_reward"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"grant_reward"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"rewards"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">          <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"currency"</span><span class="token punctuation">,</span> <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"diamond"</span><span class="token punctuation">,</span> <span class="token property">"count"</span><span class="token operator">:</span> <span class="token number">648</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"item"</span><span class="token punctuation">,</span> <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"first_recharge_pack"</span><span class="token punctuation">,</span> <span class="token property">"count"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_6-api-接口定义" tabindex="-1"><a class="header-anchor" href="#_6-api-接口定义"><span>6. API 接口定义</span></a></h2>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// proto/campaign/v1/campaign.proto</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">syntax</span> <span class="token operator">=</span> <span class="token string">"proto3"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> campaign<span class="token punctuation">.</span>v1<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"google/protobuf/timestamp.proto"</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"google/protobuf/struct.proto"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 活动管理服务</span></span>
<span class="line"><span class="token keyword">service</span> <span class="token class-name">CampaignService</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 活动模板管理</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">CreateTemplate</span><span class="token punctuation">(</span><span class="token class-name">CreateTemplateRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Template</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">GetTemplate</span><span class="token punctuation">(</span><span class="token class-name">GetTemplateRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Template</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">ListTemplates</span><span class="token punctuation">(</span><span class="token class-name">ListTemplatesRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">ListTemplatesResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">UpdateTemplate</span><span class="token punctuation">(</span><span class="token class-name">UpdateTemplateRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Template</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">DeleteTemplate</span><span class="token punctuation">(</span><span class="token class-name">DeleteTemplateRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">DeleteResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 活动实例管理</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">CreateInstance</span><span class="token punctuation">(</span><span class="token class-name">CreateInstanceRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Instance</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">GetInstance</span><span class="token punctuation">(</span><span class="token class-name">GetInstanceRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Instance</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">ListInstances</span><span class="token punctuation">(</span><span class="token class-name">ListInstancesRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">ListInstancesResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">UpdateInstance</span><span class="token punctuation">(</span><span class="token class-name">UpdateInstanceRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Instance</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">DeleteInstance</span><span class="token punctuation">(</span><span class="token class-name">DeleteInstanceRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">DeleteResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">PauseInstance</span><span class="token punctuation">(</span><span class="token class-name">PauseInstanceRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Instance</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">ResumeInstance</span><span class="token punctuation">(</span><span class="token class-name">ResumeInstanceRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">Instance</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 玩家进度查询</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">GetPlayerProgress</span><span class="token punctuation">(</span><span class="token class-name">GetPlayerProgressRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">PlayerProgress</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">ListPlayerProgress</span><span class="token punctuation">(</span><span class="token class-name">ListPlayerProgressRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">ListPlayerProgressResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">ClaimReward</span><span class="token punctuation">(</span><span class="token class-name">ClaimRewardRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">ClaimRewardResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 统计</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">GetCampaignStats</span><span class="token punctuation">(</span><span class="token class-name">GetCampaignStatsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">CampaignStats</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">Template</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> name <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> description <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> category <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">ParameterDef</span> parameter_definitions <span class="token operator">=</span> <span class="token number">10</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">TriggerConfig</span> trigger_config <span class="token operator">=</span> <span class="token number">11</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">ConditionGroup</span> condition_groups <span class="token operator">=</span> <span class="token number">12</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">Action</span> actions <span class="token operator">=</span> <span class="token number">13</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token builtin">int32</span> default_priority <span class="token operator">=</span> <span class="token number">20</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">bool</span> default_enabled <span class="token operator">=</span> <span class="token number">21</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> created_at <span class="token operator">=</span> <span class="token number">30</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> updated_at <span class="token operator">=</span> <span class="token number">31</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> created_by <span class="token operator">=</span> <span class="token number">32</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">Instance</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> template_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> name <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> start_time <span class="token operator">=</span> <span class="token number">10</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> end_time <span class="token operator">=</span> <span class="token number">11</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> status <span class="token operator">=</span> <span class="token number">12</span><span class="token punctuation">;</span>  <span class="token comment">// draft/active/paused/archived</span></span>
<span class="line"></span>
<span class="line">    <span class="token builtin">int32</span> priority <span class="token operator">=</span> <span class="token number">20</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">bool</span> enabled <span class="token operator">=</span> <span class="token number">21</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">TriggerConfig</span> trigger_config <span class="token operator">=</span> <span class="token number">22</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">ConditionGroup</span> condition_groups <span class="token operator">=</span> <span class="token number">23</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">Action</span> actions <span class="token operator">=</span> <span class="token number">24</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> parameters <span class="token operator">=</span> <span class="token number">30</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token positional-class-name class-name">CampaignStats</span> stats <span class="token operator">=</span> <span class="token number">40</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> created_at <span class="token operator">=</span> <span class="token number">50</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> updated_at <span class="token operator">=</span> <span class="token number">51</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> created_by <span class="token operator">=</span> <span class="token number">52</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">TriggerConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> event_types <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> event_filter <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">TimeWindow</span> time_window <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">AudienceRules</span> audience_rules <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">FrequencyCap</span> frequency_cap <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">TimeWindow</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> type <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// absolute/rolling/daily/weekly/cron</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> start_time <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> end_time <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> cron_expr <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> timezone <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">int32</span> week_days <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> day_start <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> day_end <span class="token operator">=</span> <span class="token number">8</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">AudienceRules</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> whitelist <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> blacklist <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> platforms <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> channels <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> server_ids <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> register_after <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> register_before <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">FrequencyCap</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> scope <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>     <span class="token comment">// global/player/server/campaign</span></span>
<span class="line">    <span class="token builtin">int32</span> max_count <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> window <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>    <span class="token comment">// once/daily/weekly/activity</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ConditionGroup</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> logic_operator <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>  <span class="token comment">// AND/OR</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">Condition</span> conditions <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">bool</span> require_all <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">Condition</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> type <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> operator <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Value</span> value <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">Action</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> type <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> params <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">int32</span> delay_ms <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> dependency <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">RetryConfig</span> retry_config <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RetryConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">int32</span> max_retries <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">int32</span> interval_ms <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">double</span> backoff_rate <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ParameterDef</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> name <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> type <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> label <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> description <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">bool</span> required <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Value</span> default_value <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Value</span> constraints <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">PlayerProgress</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> player_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> campaign_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> progress <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">int32</span> stage <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">bool</span> completed <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token builtin">int32</span> trigger_count <span class="token operator">=</span> <span class="token number">10</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> first_trigger <span class="token operator">=</span> <span class="token number">11</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> last_trigger <span class="token operator">=</span> <span class="token number">12</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> claimed_rewards <span class="token operator">=</span> <span class="token number">20</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> created_at <span class="token operator">=</span> <span class="token number">30</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> updated_at <span class="token operator">=</span> <span class="token number">31</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">CampaignStats</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">int64</span> total_triggers <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">int64</span> unique_players <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">int64</span> success_count <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">int64</span> failure_count <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> last_trigger_time <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ========== Request/Response Messages ==========</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">CreateTemplateRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> name <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> description <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> category <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">TriggerConfig</span> trigger_config <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">ConditionGroup</span> condition_groups <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">Action</span> actions <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">ParameterDef</span> parameter_definitions <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">CreateInstanceRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> template_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> name <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> start_time <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> end_time <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> parameters <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">GetPlayerProgressRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> player_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> campaign_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ClaimRewardRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> player_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> campaign_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> reward_id <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ClaimRewardResponse</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">bool</span> success <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">Reward</span> granted_rewards <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">Reward</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> type <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">int64</span> count <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_7-目录结构" tabindex="-1"><a class="header-anchor" href="#_7-目录结构"><span>7. 目录结构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">server/</span>
<span class="line">├── cmd/</span>
<span class="line">│   ├── event-gateway/          # 事件网关服务</span>
<span class="line">│   ├── analytics-worker/       # 数据分析 Worker</span>
<span class="line">│   └── campaign-worker/        # 活动 Worker (新增)</span>
<span class="line">│       └── main.go</span>
<span class="line">│</span>
<span class="line">├── internal/</span>
<span class="line">│   └── campaign/</span>
<span class="line">│       ├── types/              # 数据类型定义</span>
<span class="line">│       │   ├── template.go</span>
<span class="line">│       │   ├── instance.go</span>
<span class="line">│       │   ├── progress.go</span>
<span class="line">│       │   └── event.go</span>
<span class="line">│       │</span>
<span class="line">│       ├── engine/             # 核心引擎</span>
<span class="line">│       │   ├── trigger.go      # 触发器匹配器</span>
<span class="line">│       │   ├── condition.go    # 条件评估器</span>
<span class="line">│       │   └── action.go       # 动作执行器</span>
<span class="line">│       │</span>
<span class="line">│       ├── evaluator/          # 条件评估器实现</span>
<span class="line">│       │   ├── evaluator.go    # 接口定义</span>
<span class="line">│       │   ├── player_level.go</span>
<span class="line">│       │   ├── vip_level.go</span>
<span class="line">│       │   ├── recharge_amount.go</span>
<span class="line">│       │   ├── activity_progress.go</span>
<span class="line">│       │   └── expression.go</span>
<span class="line">│       │</span>
<span class="line">│       ├── actions/            # 动作处理器实现</span>
<span class="line">│       │   ├── grant_reward.go</span>
<span class="line">│       │   ├── send_notification.go</span>
<span class="line">│       │   ├── update_progress.go</span>
<span class="line">│       │   └── custom_rpc.go</span>
<span class="line">│       │</span>
<span class="line">│       ├── repository/         # 数据访问层</span>
<span class="line">│       │   ├── repository.go</span>
<span class="line">│       │   ├── template.go</span>
<span class="line">│       │   ├── instance.go</span>
<span class="line">│       │   └── progress.go</span>
<span class="line">│       │</span>
<span class="line">│       ├── cache/              # 缓存层</span>
<span class="line">│       │   └── cache.go</span>
<span class="line">│       │</span>
<span class="line">│       └── service/            # 服务层</span>
<span class="line">│           ├── template_service.go</span>
<span class="line">│           ├── instance_service.go</span>
<span class="line">│           └── progress_service.go</span>
<span class="line">│</span>
<span class="line">├── proto/</span>
<span class="line">│   └── campaign/</span>
<span class="line">│       └── v1/</span>
<span class="line">│           └── campaign.proto</span>
<span class="line">│</span>
<span class="line">└── docs/</span>
<span class="line">    ├── event-driven-architecture.md</span>
<span class="line">    └── campaign-system.md      # 本文档</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_8-数据库表设计" tabindex="-1"><a class="header-anchor" href="#_8-数据库表设计"><span>8. 数据库表设计</span></a></h2>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token comment">-- 活动模板表</span></span>
<span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> campaign_templates <span class="token punctuation">(</span></span>
<span class="line">    id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token keyword">PRIMARY</span> <span class="token keyword">KEY</span><span class="token punctuation">,</span></span>
<span class="line">    name <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">255</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    description <span class="token keyword">TEXT</span><span class="token punctuation">,</span></span>
<span class="line">    category <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">50</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    version <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">20</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span> <span class="token keyword">DEFAULT</span> <span class="token string">'1.0.0'</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    trigger_config JSON <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    condition_groups JSON <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    actions JSON <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    default_priority <span class="token keyword">INT</span> <span class="token keyword">DEFAULT</span> <span class="token number">0</span><span class="token punctuation">,</span></span>
<span class="line">    default_enabled <span class="token keyword">BOOLEAN</span> <span class="token keyword">DEFAULT</span> <span class="token boolean">TRUE</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    created_at <span class="token keyword">TIMESTAMP</span> <span class="token keyword">DEFAULT</span> <span class="token keyword">CURRENT_TIMESTAMP</span><span class="token punctuation">,</span></span>
<span class="line">    updated_at <span class="token keyword">TIMESTAMP</span> <span class="token keyword">DEFAULT</span> <span class="token keyword">CURRENT_TIMESTAMP</span> <span class="token keyword">ON</span> <span class="token keyword">UPDATE</span> <span class="token keyword">CURRENT_TIMESTAMP</span><span class="token punctuation">,</span></span>
<span class="line">    created_by <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_category <span class="token punctuation">(</span>category<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">-- 活动实例表</span></span>
<span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> campaign_instances <span class="token punctuation">(</span></span>
<span class="line">    id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token keyword">PRIMARY</span> <span class="token keyword">KEY</span><span class="token punctuation">,</span></span>
<span class="line">    template_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    name <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">255</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    game_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    env <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">20</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    start_time <span class="token keyword">TIMESTAMP</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    end_time <span class="token keyword">TIMESTAMP</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">status</span> <span class="token keyword">ENUM</span><span class="token punctuation">(</span><span class="token string">'draft'</span><span class="token punctuation">,</span> <span class="token string">'active'</span><span class="token punctuation">,</span> <span class="token string">'paused'</span><span class="token punctuation">,</span> <span class="token string">'archived'</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span> <span class="token keyword">DEFAULT</span> <span class="token string">'draft'</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    priority <span class="token keyword">INT</span> <span class="token keyword">DEFAULT</span> <span class="token number">0</span><span class="token punctuation">,</span></span>
<span class="line">    enabled <span class="token keyword">BOOLEAN</span> <span class="token keyword">DEFAULT</span> <span class="token boolean">TRUE</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    trigger_config JSON <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    condition_groups JSON <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    actions JSON <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    parameters JSON<span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    stats JSON<span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    created_at <span class="token keyword">TIMESTAMP</span> <span class="token keyword">DEFAULT</span> <span class="token keyword">CURRENT_TIMESTAMP</span><span class="token punctuation">,</span></span>
<span class="line">    updated_at <span class="token keyword">TIMESTAMP</span> <span class="token keyword">DEFAULT</span> <span class="token keyword">CURRENT_TIMESTAMP</span> <span class="token keyword">ON</span> <span class="token keyword">UPDATE</span> <span class="token keyword">CURRENT_TIMESTAMP</span><span class="token punctuation">,</span></span>
<span class="line">    created_by <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_game_env <span class="token punctuation">(</span>game_id<span class="token punctuation">,</span> env<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_status_time <span class="token punctuation">(</span><span class="token keyword">status</span><span class="token punctuation">,</span> start_time<span class="token punctuation">,</span> end_time<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_template <span class="token punctuation">(</span>template_id<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">-- 玩家活动进度表</span></span>
<span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> campaign_player_progress <span class="token punctuation">(</span></span>
<span class="line">    player_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    campaign_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    game_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    env <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">20</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    progress JSON <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    stage <span class="token keyword">INT</span> <span class="token keyword">DEFAULT</span> <span class="token number">0</span><span class="token punctuation">,</span></span>
<span class="line">    completed <span class="token keyword">BOOLEAN</span> <span class="token keyword">DEFAULT</span> <span class="token boolean">FALSE</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    trigger_count <span class="token keyword">INT</span> <span class="token keyword">DEFAULT</span> <span class="token number">0</span><span class="token punctuation">,</span></span>
<span class="line">    first_trigger <span class="token keyword">TIMESTAMP</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    last_trigger <span class="token keyword">TIMESTAMP</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    claimed_rewards JSON<span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    created_at <span class="token keyword">TIMESTAMP</span> <span class="token keyword">DEFAULT</span> <span class="token keyword">CURRENT_TIMESTAMP</span><span class="token punctuation">,</span></span>
<span class="line">    updated_at <span class="token keyword">TIMESTAMP</span> <span class="token keyword">DEFAULT</span> <span class="token keyword">CURRENT_TIMESTAMP</span> <span class="token keyword">ON</span> <span class="token keyword">UPDATE</span> <span class="token keyword">CURRENT_TIMESTAMP</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">PRIMARY</span> <span class="token keyword">KEY</span> <span class="token punctuation">(</span>player_id<span class="token punctuation">,</span> campaign_id<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_campaign <span class="token punctuation">(</span>campaign_id<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_player <span class="token punctuation">(</span>player_id<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">-- 动作执行记录表 (审计)</span></span>
<span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> campaign_action_logs <span class="token punctuation">(</span></span>
<span class="line">    id <span class="token keyword">BIGINT</span> <span class="token keyword">AUTO_INCREMENT</span> <span class="token keyword">PRIMARY</span> <span class="token keyword">KEY</span><span class="token punctuation">,</span></span>
<span class="line">    campaign_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    player_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    event_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    action_id <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">64</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    action_type <span class="token keyword">VARCHAR</span><span class="token punctuation">(</span><span class="token number">50</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">status</span> <span class="token keyword">ENUM</span><span class="token punctuation">(</span><span class="token string">'pending'</span><span class="token punctuation">,</span> <span class="token string">'success'</span><span class="token punctuation">,</span> <span class="token string">'failed'</span><span class="token punctuation">)</span> <span class="token operator">NOT</span> <span class="token boolean">NULL</span><span class="token punctuation">,</span></span>
<span class="line">    error_message <span class="token keyword">TEXT</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    input_params JSON<span class="token punctuation">,</span></span>
<span class="line">    output_result JSON<span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    duration_ms <span class="token keyword">INT</span><span class="token punctuation">,</span></span>
<span class="line">    executed_at <span class="token keyword">TIMESTAMP</span> <span class="token keyword">DEFAULT</span> <span class="token keyword">CURRENT_TIMESTAMP</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_campaign_player <span class="token punctuation">(</span>campaign_id<span class="token punctuation">,</span> player_id<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_event <span class="token punctuation">(</span>event_id<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">INDEX</span> idx_status_time <span class="token punctuation">(</span><span class="token keyword">status</span><span class="token punctuation">,</span> executed_at<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="_9-性能优化" tabindex="-1"><a class="header-anchor" href="#_9-性能优化"><span>9. 性能优化</span></a></h2>
<h3 id="_9-1-缓存策略" tabindex="-1"><a class="header-anchor" href="#_9-1-缓存策略"><span>9.1 缓存策略</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">+-------------------+     +-------------------+     +-------------------+</span>
<span class="line">|   Redis Cache     |     |   Local Cache     |     |   Database        |</span>
<span class="line">|                   |     |   (sync.Map)      |     |                   |</span>
<span class="line">| Active Campaigns  |&lt;--->| Template Config   |&lt;--->| Persistent Data   |</span>
<span class="line">| Player Progress   |     | Condition Eval    |     |                   |</span>
<span class="line">+-------------------+     +-------------------+     +-------------------+</span>
<span class="line">        ^                         ^                         ^</span>
<span class="line">        | TTL: 5min               | TTL: 1min               |</span>
<span class="line">        |                         |                         |</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_9-2-批处理优化" tabindex="-1"><a class="header-anchor" href="#_9-2-批处理优化"><span>9.2 批处理优化</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 批量获取玩家进度</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>ProgressRepository<span class="token punctuation">)</span> <span class="token function">GetPlayerProgressBatch</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> playerIDs <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span><span class="token punctuation">,</span> campaignID <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token operator">*</span>types<span class="token punctuation">.</span>PlayerProgress<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 使用 IN 查询</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 批量更新进度</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>ProgressRepository<span class="token punctuation">)</span> <span class="token function">SavePlayerProgressBatch</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> progresses <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token operator">*</span>types<span class="token punctuation">.</span>PlayerProgress<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 使用批量 INSERT ON DUPLICATE KEY UPDATE</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_9-3-异步执行" tabindex="-1"><a class="header-anchor" href="#_9-3-异步执行"><span>9.3 异步执行</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 非关键路径异步执行</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>ae <span class="token operator">*</span>ActionExecutor<span class="token punctuation">)</span> <span class="token function">ExecuteAsync</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> actions <span class="token punctuation">[</span><span class="token punctuation">]</span>types<span class="token punctuation">.</span>Action<span class="token punctuation">,</span> execCtx <span class="token operator">*</span>ExecutionContext<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">go</span> <span class="token keyword">func</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// 后台执行，不阻塞主流程</span></span>
<span class="line">        ae<span class="token punctuation">.</span><span class="token function">Execute</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> actions<span class="token punctuation">,</span> execCtx<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></div></template>


