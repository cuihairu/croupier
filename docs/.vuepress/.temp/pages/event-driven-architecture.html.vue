<template><div><h1 id="事件驱动架构设计" tabindex="-1"><a class="header-anchor" href="#事件驱动架构设计"><span>事件驱动架构设计</span></a></h1>
<h2 id="概述" tabindex="-1"><a class="header-anchor" href="#概述"><span>概述</span></a></h2>
<p>本文档描述 Croupier 系统的事件驱动架构，该架构支持数据分析、活动系统和未来扩展的事件订阅者。</p>
<h2 id="设计目标" tabindex="-1"><a class="header-anchor" href="#设计目标"><span>设计目标</span></a></h2>
<ol>
<li><strong>统一事件采集</strong> - 客户端/游戏服务器只需发送一次事件</li>
<li><strong>解耦订阅者</strong> - 数据分析、活动系统等独立订阅，互不影响</li>
<li><strong>可扩展性</strong> - 支持新增订阅者无需修改现有代码</li>
<li><strong>高可用性</strong> - 支持事件重试、死信队列、故障恢复</li>
<li><strong>性能优化</strong> - 批量处理、异步处理、检查点机制</li>
</ol>
<h2 id="架构图" tabindex="-1"><a class="header-anchor" href="#架构图"><span>架构图</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐</span>
<span class="line">│                                                                  事件驱动架构                                                       │</span>
<span class="line">├─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤</span>
<span class="line">│                                                                                                                             │</span>
<span class="line">│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                                                                │</span>
<span class="line">│  │ Game Client  │     │ Game Server  │     │  Admin API   │                                                                │</span>
<span class="line">│  │   (SDK)      │     │   (Backend)  │     │   (手动触发)   │                                                                │</span>
<span class="line">│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘                                                                │</span>
<span class="line">│         │                    │                     │                                                                       │</span>
<span class="line">│         └────────────────────┼─────────────────────┘                                                                       │</span>
<span class="line">│                              │                                                                                              │</span>
<span class="line">│                              ▼                                                                                              │</span>
<span class="line">│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │</span>
<span class="line">│  │                                                       Event Gateway Service                                          │   │</span>
<span class="line">│  │                                                   (事件网关服务 - 独立进程)                                            │   │</span>
<span class="line">│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐              │   │</span>
<span class="line">│  │  │  HTTP Collector │  │  gRPC Collector│  │  Event Validator│  │  Event Enricher │  │  Rate Limiter   │              │   │</span>
<span class="line">│  │  │   /api/events   │  │   EventService │  │   (Schema)      │  │  (Context)      │  │                 │              │   │</span>
<span class="line">│  │  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘  └─────────────────┘              │   │</span>
<span class="line">│  └───────────┼────────────────────┼───────────────────┼───────────────────┼────────────────────────────────────────────────┘   │</span>
<span class="line">│              │                    │                   │                   │                                                   │</span>
<span class="line">│  ┌───────────┴────────────────────┴───────────────────┴───────────────────┴────────────────────────────────────────────────┐   │</span>
<span class="line">│  │                                                      Event Bus / Message Queue                                          │   │</span>
<span class="line">│  │                                                                                                                     Redis   │</span>
<span class="line">│  │  ┌───────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │   │</span>
<span class="line">│  │  │                                          events (主事件流)                                                       │   │   │</span>
<span class="line">│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐                 │   │   │</span>
<span class="line">│  │  │  │  event  │ │  event  │ │  event  │ │  event  │ │  event  │ │  event  │ │  event  │ │  event  │  ...          │   │   │</span>
<span class="line">│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘                 │   │   │</span>
<span class="line">│  │  └───────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │   │</span>
<span class="line">│  │                                                                                                                      │   │</span>
<span class="line">│  │  ┌───────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │   │</span>
<span class="line">│  │  │                                       events:high_priority (高优先级/实时)                                        │   │   │</span>
<span class="line">│  │  └───────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │   │</span>
<span class="line">│  │                                                                                                                      │   │</span>
<span class="line">│  │  ┌───────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │   │</span>
<span class="line">│  │  │                                        events:dlq (死信队列)                                                     │   │   │</span>
<span class="line">│  │  └───────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │   │</span>
<span class="line">│  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │</span>
<span class="line">│                                                                                                                             │</span>
<span class="line">│                              │                    │                    │                                                   │</span>
<span class="line">│              ┌───────────────┴────────────────────┴────────────────────┴───────────────────────────────────────┐         │</span>
<span class="line">│              │                                     │                                      │                    │         │</span>
<span class="line">│              ▼                                     ▼                                      ▼                    ▼         │</span>
<span class="line">│  ┌───────────────────────┐           ┌───────────────────────┐           ┌───────────────────────┐   ┌──────────────────┐  │</span>
<span class="line">│  │  Analytics Worker     │           │   Campaign Worker     │           │   (Future) Worker     │   │   Other Services  │  │</span>
<span class="line">│  │                       │           │                       │           │                       │   │                  │  │</span>
<span class="line">│  │  - ClickHouse 写入    │           │  - 触发器匹配         │           │  - 实时通知           │   │  - Webhook       │  │</span>
<span class="line">│  │  - 聚合计算 (DAU/MAU) │           │  - 条件评估           │           │  - 风控检测           │   │  - 第三方集成     │  │</span>
<span class="line">│  │  - 指标刷新           │           │  - 动作执行           │           │  - 日志分析           │   │                  │  │</span>
<span class="line">│  │                       │           │  - 奖励发放           │           │                       │   │                  │  │</span>
<span class="line">│  └───────────────────────┘           └───────────────────────┘           └───────────────────────┘   └──────────────────┘  │</span>
<span class="line">│                                                                                                                             │</span>
<span class="line">└─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="核心组件" tabindex="-1"><a class="header-anchor" href="#核心组件"><span>核心组件</span></a></h2>
<h3 id="_1-event-gateway-service-事件网关服务" tabindex="-1"><a class="header-anchor" href="#_1-event-gateway-service-事件网关服务"><span>1. Event Gateway Service (事件网关服务)</span></a></h3>
<p><strong>职责</strong>: 统一的事件采集入口，接收所有客户端/服务器上报的事件。</p>
<p><strong>功能</strong>:</p>
<ul>
<li>接收 HTTP/gRPC 事件上报</li>
<li>事件验证 (Schema Validation)</li>
<li>事件丰富化 (Enrichment - 添加 IP、时间、地理位置等)</li>
<li>限流保护 (Rate Limiting)</li>
<li>发布到消息队列</li>
</ul>
<p><strong>接口定义</strong>:</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// proto/event/v1/event_gateway.proto</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">syntax</span> <span class="token operator">=</span> <span class="token string">"proto3"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> event<span class="token punctuation">.</span>v1<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"google/protobuf/timestamp.proto"</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"google/protobuf/struct.proto"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 事件网关服务</span></span>
<span class="line"><span class="token keyword">service</span> <span class="token class-name">EventGateway</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token comment">// 上报单个事件</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">PublishEvent</span><span class="token punctuation">(</span><span class="token class-name">Event</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">PublishResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 批量上报事件</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">PublishEvents</span><span class="token punctuation">(</span><span class="token class-name">EventBatch</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">PublishBatchResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 获取事件 Schema</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">GetEventSchema</span><span class="token punctuation">(</span><span class="token class-name">GetSchemaRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">EventSchema</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 获取上报状态</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">GetPublishStatus</span><span class="token punctuation">(</span><span class="token class-name">GetStatusRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">PublishStatus</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 事件定义</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">Event</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token comment">// 必填字段</span></span>
<span class="line">  <span class="token builtin">string</span> event_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>           <span class="token comment">// 事件唯一 ID (可选，系统自动生成)</span></span>
<span class="line">  <span class="token builtin">string</span> event_type <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>         <span class="token comment">// 事件类型 (见 EventType 枚举)</span></span>
<span class="line">  <span class="token builtin">string</span> player_id <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>          <span class="token comment">// 玩家 ID</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>            <span class="token comment">// 游戏 ID</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>                <span class="token comment">// 环境 (dev/staging/prod)</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 时间信息</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> event_time <span class="token operator">=</span> <span class="token number">10</span><span class="token punctuation">;</span>     <span class="token comment">// 事件发生时间</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Timestamp</span> receive_time <span class="token operator">=</span> <span class="token number">11</span><span class="token punctuation">;</span>   <span class="token comment">// 接收时间 (服务端填充)</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 事件属性 (动态结构)</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> properties <span class="token operator">=</span> <span class="token number">20</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 上下文信息 (客户端填充)</span></span>
<span class="line">  <span class="token positional-class-name class-name">EventContext</span> context <span class="token operator">=</span> <span class="token number">30</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 事件上下文</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">EventContext</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> session_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>         <span class="token comment">// 会话 ID</span></span>
<span class="line">  <span class="token builtin">string</span> server_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>          <span class="token comment">// 服务器 ID</span></span>
<span class="line">  <span class="token builtin">string</span> channel <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>            <span class="token comment">// 渠道</span></span>
<span class="line">  <span class="token builtin">string</span> platform <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>           <span class="token comment">// 平台 (ios/android/web/pc)</span></span>
<span class="line">  <span class="token builtin">string</span> app_version <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>        <span class="token comment">// 应用版本</span></span>
<span class="line">  <span class="token builtin">string</span> device_id <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>          <span class="token comment">// 设备 ID</span></span>
<span class="line">  <span class="token builtin">string</span> ip <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span>                 <span class="token comment">// IP 地址</span></span>
<span class="line">  <span class="token builtin">string</span> country <span class="token operator">=</span> <span class="token number">8</span><span class="token punctuation">;</span>            <span class="token comment">// 国家</span></span>
<span class="line">  <span class="token builtin">string</span> region <span class="token operator">=</span> <span class="token number">9</span><span class="token punctuation">;</span>             <span class="token comment">// 地区</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> custom <span class="token operator">=</span> <span class="token number">100</span><span class="token punctuation">;</span>  <span class="token comment">// 自定义字段</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 批量事件</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">EventBatch</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">Event</span> events <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">bool</span> require_all <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>          <span class="token comment">// 是否要求全部成功</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 发布响应</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">PublishResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">bool</span> success <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> event_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> error_message <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> trace_id <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>           <span class="token comment">// 追踪 ID</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 批量发布响应</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">PublishBatchResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">int32</span> success_count <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">int32</span> failure_count <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">PublishResponse</span> results <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Schema 请求</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">GetSchemaRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> event_type <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 事件 Schema</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">EventSchema</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> event_type <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> description <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> properties_schema <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>  <span class="token comment">// JSON Schema 格式</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token builtin">string</span> required_fields <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 状态查询</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">GetStatusRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> trace_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> event_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 发布状态</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">PublishStatus</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> status <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>             <span class="token comment">// pending/processed/failed</span></span>
<span class="line">  <span class="token builtin">string</span> error_message <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token builtin">string</span> subscribers <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// 已处理的订阅者</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-event-bus-事件总线" tabindex="-1"><a class="header-anchor" href="#_2-event-bus-事件总线"><span>2. Event Bus (事件总线)</span></a></h3>
<p><strong>实现</strong>: Redis Streams / Kafka (可配置切换)</p>
<p><strong>Stream 定义</strong>:</p>
<table>
<thead>
<tr>
<th>Stream 名称</th>
<th>用途</th>
<th>优先级</th>
<th>TTL</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>events</code></td>
<td>主事件流</td>
<td>普通</td>
<td>7天</td>
</tr>
<tr>
<td><code v-pre>events:high_priority</code></td>
<td>高优先级事件 (如支付)</td>
<td>高</td>
<td>30天</td>
</tr>
<tr>
<td><code v-pre>events:dlq</code></td>
<td>死信队列</td>
<td>-</td>
<td>90天</td>
</tr>
</tbody>
</table>
<h3 id="_3-事件类型定义" tabindex="-1"><a class="header-anchor" href="#_3-事件类型定义"><span>3. 事件类型定义</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/event/types/event_type.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> types</span>
<span class="line"></span>
<span class="line"><span class="token comment">// EventType 事件类型枚举</span></span>
<span class="line"><span class="token keyword">type</span> EventType <span class="token builtin">string</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">const</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token comment">// ========== 账户事件 ==========</span></span>
<span class="line">    EventTypePlayerRegister   EventType <span class="token operator">=</span> <span class="token string">"player.register"</span>   <span class="token comment">// 玩家注册</span></span>
<span class="line">    EventTypePlayerLogin      EventType <span class="token operator">=</span> <span class="token string">"player.login"</span>      <span class="token comment">// 玩家登录</span></span>
<span class="line">    EventTypePlayerLogout     EventType <span class="token operator">=</span> <span class="token string">"player.logout"</span>     <span class="token comment">// 玩家登出</span></span>
<span class="line">    EventTypePlayerSessionEnd EventType <span class="token operator">=</span> <span class="token string">"player.session_end"</span> <span class="token comment">// 会话结束</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 经济事件 ==========</span></span>
<span class="line">    EventTypePaymentStart     EventType <span class="token operator">=</span> <span class="token string">"payment.start"</span>     <span class="token comment">// 支付开始</span></span>
<span class="line">    EventTypePaymentSuccess   EventType <span class="token operator">=</span> <span class="token string">"payment.success"</span>   <span class="token comment">// 支付成功</span></span>
<span class="line">    EventTypePaymentFail      EventType <span class="token operator">=</span> <span class="token string">"payment.fail"</span>      <span class="token comment">// 支付失败</span></span>
<span class="line">    EventTypePaymentRefund    EventType <span class="token operator">=</span> <span class="token string">"payment.refund"</span>    <span class="token comment">// 退款</span></span>
<span class="line">    EventTypeCurrencyConsume  EventType <span class="token operator">=</span> <span class="token string">"currency.consume"</span>  <span class="token comment">// 货币消耗</span></span>
<span class="line">    EventTypeCurrencyEarn     EventType <span class="token operator">=</span> <span class="token string">"currency.earn"</span>     <span class="token comment">// 货币获得</span></span>
<span class="line">    EventTypeItemConsume      EventType <span class="token operator">=</span> <span class="token string">"item.consume"</span>      <span class="token comment">// 物品消耗</span></span>
<span class="line">    EventTypeItemEarn         EventType <span class="token operator">=</span> <span class="token string">"item.earn"</span>         <span class="token comment">// 物品获得</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 游戏行为 ==========</span></span>
<span class="line">    EventTypeQuestAccept      EventType <span class="token operator">=</span> <span class="token string">"quest.accept"</span>      <span class="token comment">// 接取任务</span></span>
<span class="line">    EventTypeQuestComplete    EventType <span class="token operator">=</span> <span class="token string">"quest.complete"</span>    <span class="token comment">// 完成任务</span></span>
<span class="line">    EventTypeQuestFail        EventType <span class="token operator">=</span> <span class="token string">"quest.fail"</span>        <span class="token comment">// 任务失败</span></span>
<span class="line">    EventTypeLevelUp          EventType <span class="token operator">=</span> <span class="token string">"player.level_up"</span>   <span class="token comment">// 升级</span></span>
<span class="line">    EventTypeAchievementUnlock EventType <span class="token operator">=</span> <span class="token string">"achievement.unlock"</span> <span class="token comment">// 成就解锁</span></span>
<span class="line">    EventTypeBossKill         EventType <span class="token operator">=</span> <span class="token string">"boss.kill"</span>         <span class="token comment">// 击杀 BOSS</span></span>
<span class="line">    EventTypePVPBattle        EventType <span class="token operator">=</span> <span class="token string">"pvp.battle"</span>        <span class="token comment">// PVP 战斗</span></span>
<span class="line">    EventTypePVPWin           EventType <span class="token operator">=</span> <span class="token string">"pvp.win"</span>           <span class="token comment">// PVP 胜利</span></span>
<span class="line">    EventTypePVPLoss          EventType <span class="token operator">=</span> <span class="token string">"pvp.loss"</span>          <span class="token comment">// PVP 失败</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 社交事件 ==========</span></span>
<span class="line">    EventTypeGuildJoin        EventType <span class="token operator">=</span> <span class="token string">"guild.join"</span>        <span class="token comment">// 加入公会</span></span>
<span class="line">    EventTypeGuildLeave       EventType <span class="token operator">=</span> <span class="token string">"guild.leave"</span>       <span class="token comment">// 离开公会</span></span>
<span class="line">    EventTypeFriendAdd        EventType <span class="token operator">=</span> <span class="token string">"friend.add"</span>        <span class="token comment">// 添加好友</span></span>
<span class="line">    EventTypeChatSend         EventType <span class="token operator">=</span> <span class="token string">"chat.send"</span>         <span class="token comment">// 发送聊天</span></span>
<span class="line">    EventTypeGiftSend         EventType <span class="token operator">=</span> <span class="token string">"gift.send"</span>         <span class="token comment">// 发送礼物</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 系统事件 ==========</span></span>
<span class="line">    EventTypeSystemMaintenance EventType <span class="token operator">=</span> <span class="token string">"system.maintenance"</span> <span class="token comment">// 系统维护</span></span>
<span class="line">    EventTypeSystemAnnouncement EventType <span class="token operator">=</span> <span class="token string">"system.announcement"</span> <span class="token comment">// 系统公告</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// EventPriority 事件优先级</span></span>
<span class="line"><span class="token keyword">type</span> EventPriority <span class="token builtin">string</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">const</span> <span class="token punctuation">(</span></span>
<span class="line">    PriorityLow      EventPriority <span class="token operator">=</span> <span class="token string">"low"</span></span>
<span class="line">    PriorityNormal   EventPriority <span class="token operator">=</span> <span class="token string">"normal"</span></span>
<span class="line">    PriorityHigh     EventPriority <span class="token operator">=</span> <span class="token string">"high"</span></span>
<span class="line">    PriorityCritical EventPriority <span class="token operator">=</span> <span class="token string">"critical"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// GetPriority 获取事件优先级</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>et EventType<span class="token punctuation">)</span> <span class="token function">GetPriority</span><span class="token punctuation">(</span><span class="token punctuation">)</span> EventPriority <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">switch</span> et <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">case</span> EventTypePaymentSuccess<span class="token punctuation">,</span> EventTypePaymentRefund<span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> PriorityCritical</span>
<span class="line">    <span class="token keyword">case</span> EventTypePaymentStart<span class="token punctuation">,</span> EventTypePaymentFail<span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> PriorityHigh</span>
<span class="line">    <span class="token keyword">case</span> EventTypePlayerLogin<span class="token punctuation">,</span> EventTypePlayerRegister<span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> PriorityHigh</span>
<span class="line">    <span class="token keyword">default</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">return</span> PriorityNormal</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-worker-订阅者基类" tabindex="-1"><a class="header-anchor" href="#_4-worker-订阅者基类"><span>4. Worker 订阅者基类</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// internal/event/worker/worker.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> worker</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"encoding/json"</span></span>
<span class="line">    <span class="token string">"log/slog"</span></span>
<span class="line">    <span class="token string">"time"</span></span>
<span class="line"></span>
<span class="line">    redis <span class="token string">"github.com/redis/go-redis/v9"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Event 事件结构</span></span>
<span class="line"><span class="token keyword">type</span> Event <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any</span>
<span class="line"></span>
<span class="line"><span class="token comment">// WorkerConfig Worker 配置</span></span>
<span class="line"><span class="token keyword">type</span> WorkerConfig <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// Redis 配置</span></span>
<span class="line">    RedisURL        <span class="token builtin">string</span></span>
<span class="line">    StreamEvents    <span class="token builtin">string</span></span>
<span class="line">    StreamHighPrio  <span class="token builtin">string</span></span>
<span class="line">    StreamDLQ       <span class="token builtin">string</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 消费者组配置</span></span>
<span class="line">    ConsumerGroup   <span class="token builtin">string</span></span>
<span class="line">    ConsumerName    <span class="token builtin">string</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 批处理配置</span></span>
<span class="line">    BatchSize       <span class="token builtin">int</span></span>
<span class="line">    BlockTime       time<span class="token punctuation">.</span>Duration</span>
<span class="line">    FlushInterval   time<span class="token punctuation">.</span>Duration</span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 重试配置</span></span>
<span class="line">    MaxRetries      <span class="token builtin">int</span></span>
<span class="line">    RetryInterval   time<span class="token punctuation">.</span>Duration</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// EventHandler 事件处理器接口</span></span>
<span class="line"><span class="token keyword">type</span> EventHandler <span class="token keyword">interface</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// GetEventTypes 返回要处理的事件类型，空表示处理所有</span></span>
<span class="line">    <span class="token function">GetEventTypes</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Handle 处理单个事件</span></span>
<span class="line">    <span class="token function">Handle</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// HandleBatch 批量处理事件 (可选，优化性能)</span></span>
<span class="line">    <span class="token function">HandleBatch</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> events <span class="token punctuation">[</span><span class="token punctuation">]</span>Event<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// GetName 获取处理器名称</span></span>
<span class="line">    <span class="token function">GetName</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">string</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Worker 事件处理 Worker</span></span>
<span class="line"><span class="token keyword">type</span> Worker <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    config   WorkerConfig</span>
<span class="line">    client   <span class="token operator">*</span>redis<span class="token punctuation">.</span>Client</span>
<span class="line">    handlers <span class="token punctuation">[</span><span class="token punctuation">]</span>EventHandler</span>
<span class="line">    ctx      context<span class="token punctuation">.</span>Context</span>
<span class="line">    cancel   context<span class="token punctuation">.</span>CancelFunc</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// NewWorker 创建新 Worker</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">NewWorker</span><span class="token punctuation">(</span>config WorkerConfig<span class="token punctuation">)</span> <span class="token operator">*</span>Worker <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token operator">&amp;</span>Worker<span class="token punctuation">{</span></span>
<span class="line">        config<span class="token punctuation">:</span>   config<span class="token punctuation">,</span></span>
<span class="line">        handlers<span class="token punctuation">:</span> <span class="token function">make</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>EventHandler<span class="token punctuation">,</span> <span class="token number">0</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// RegisterHandler 注册事件处理器</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">RegisterHandler</span><span class="token punctuation">(</span>handler EventHandler<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    w<span class="token punctuation">.</span>handlers <span class="token operator">=</span> <span class="token function">append</span><span class="token punctuation">(</span>w<span class="token punctuation">.</span>handlers<span class="token punctuation">,</span> handler<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Start 启动 Worker</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">Start</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    w<span class="token punctuation">.</span>ctx<span class="token punctuation">,</span> w<span class="token punctuation">.</span>cancel <span class="token operator">=</span> context<span class="token punctuation">.</span><span class="token function">WithCancel</span><span class="token punctuation">(</span>ctx<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 初始化 Redis</span></span>
<span class="line">    opt<span class="token punctuation">,</span> err <span class="token operator">:=</span> redis<span class="token punctuation">.</span><span class="token function">ParseURL</span><span class="token punctuation">(</span>w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>RedisURL<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> err</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    w<span class="token punctuation">.</span>client <span class="token operator">=</span> redis<span class="token punctuation">.</span><span class="token function">NewClient</span><span class="token punctuation">(</span>opt<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 确保消费组存在</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">ensureConsumerGroup</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 启动处理循环</span></span>
<span class="line">    <span class="token keyword">go</span> w<span class="token punctuation">.</span><span class="token function">processLoop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">go</span> w<span class="token punctuation">.</span><span class="token function">flushLoop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 启动死信处理</span></span>
<span class="line">    <span class="token keyword">go</span> w<span class="token punctuation">.</span><span class="token function">processDLQ</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Stop 停止 Worker</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> w<span class="token punctuation">.</span>cancel <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        w<span class="token punctuation">.</span><span class="token function">cancel</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">if</span> w<span class="token punctuation">.</span>client <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        w<span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">Close</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ensureConsumerGroup 确保消费组存在</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">ensureConsumerGroup</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ctx<span class="token punctuation">,</span> cancel <span class="token operator">:=</span> context<span class="token punctuation">.</span><span class="token function">WithTimeout</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token number">5</span><span class="token operator">*</span>time<span class="token punctuation">.</span>Second<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> <span class="token function">cancel</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 创建主消费组</span></span>
<span class="line">    w<span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">XGroupCreateMkStream</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>StreamEvents<span class="token punctuation">,</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>ConsumerGroup<span class="token punctuation">,</span> <span class="token string">"$"</span><span class="token punctuation">)</span></span>
<span class="line">    w<span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">XGroupCreateMkStream</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>StreamHighPrio<span class="token punctuation">,</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>ConsumerGroup<span class="token punctuation">,</span> <span class="token string">"$"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// processLoop 主处理循环</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">processLoop</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">select</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">case</span> <span class="token operator">&lt;-</span>w<span class="token punctuation">.</span>ctx<span class="token punctuation">.</span><span class="token function">Done</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token keyword">return</span></span>
<span class="line">        <span class="token keyword">default</span><span class="token punctuation">:</span></span>
<span class="line">            w<span class="token punctuation">.</span><span class="token function">processMessages</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// processMessages 处理消息</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">processMessages</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 优先处理高优先级队列</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">processStream</span><span class="token punctuation">(</span>w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>StreamHighPrio<span class="token punctuation">)</span></span>
<span class="line">    <span class="token comment">// 然后处理普通队列</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">processStream</span><span class="token punctuation">(</span>w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>StreamEvents<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// processStream 处理单个 Stream</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">processStream</span><span class="token punctuation">(</span>stream <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ctx<span class="token punctuation">,</span> cancel <span class="token operator">:=</span> context<span class="token punctuation">.</span><span class="token function">WithTimeout</span><span class="token punctuation">(</span>w<span class="token punctuation">.</span>ctx<span class="token punctuation">,</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>BlockTime<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> <span class="token function">cancel</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 读取消息</span></span>
<span class="line">    result<span class="token punctuation">,</span> err <span class="token operator">:=</span> w<span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">XReadGroup</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>redis<span class="token punctuation">.</span>XReadGroupArgs<span class="token punctuation">{</span></span>
<span class="line">        Group<span class="token punctuation">:</span>    w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>ConsumerGroup<span class="token punctuation">,</span></span>
<span class="line">        Consumer<span class="token punctuation">:</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>ConsumerName<span class="token punctuation">,</span></span>
<span class="line">        Streams<span class="token punctuation">:</span>  <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span><span class="token punctuation">{</span>stream<span class="token punctuation">,</span> <span class="token string">">"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        Count<span class="token punctuation">:</span>    <span class="token function">int64</span><span class="token punctuation">(</span>w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>BatchSize<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Result</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token operator">||</span> <span class="token function">len</span><span class="token punctuation">(</span>result<span class="token punctuation">)</span> <span class="token operator">==</span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> stream <span class="token operator">:=</span> <span class="token keyword">range</span> result <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> msg <span class="token operator">:=</span> <span class="token keyword">range</span> stream<span class="token punctuation">.</span>Messages <span class="token punctuation">{</span></span>
<span class="line">            w<span class="token punctuation">.</span><span class="token function">handleMessage</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> stream<span class="token punctuation">.</span>Stream<span class="token punctuation">,</span> msg<span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// handleMessage 处理单条消息</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">handleMessage</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> stream <span class="token builtin">string</span><span class="token punctuation">,</span> msg redis<span class="token punctuation">.</span>XMessage<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 解析事件</span></span>
<span class="line">    <span class="token keyword">var</span> event Event</span>
<span class="line">    data<span class="token punctuation">,</span> ok <span class="token operator">:=</span> msg<span class="token punctuation">.</span>Values<span class="token punctuation">[</span><span class="token string">"data"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token operator">!</span>ok <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> w<span class="token punctuation">.</span><span class="token function">ack</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> stream<span class="token punctuation">,</span> msg<span class="token punctuation">.</span>ID<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> json<span class="token punctuation">.</span><span class="token function">Unmarshal</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token function">byte</span><span class="token punctuation">(</span>data<span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">string</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token operator">&amp;</span>event<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        w<span class="token punctuation">.</span><span class="token function">sendToDLQ</span><span class="token punctuation">(</span>stream<span class="token punctuation">,</span> msg<span class="token punctuation">,</span> <span class="token string">"invalid_json"</span><span class="token punctuation">,</span> err<span class="token punctuation">.</span><span class="token function">Error</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span> w<span class="token punctuation">.</span><span class="token function">ack</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> stream<span class="token punctuation">,</span> msg<span class="token punctuation">.</span>ID<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 分发给处理器</span></span>
<span class="line">    <span class="token keyword">var</span> handlerErr <span class="token builtin">error</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> handler <span class="token operator">:=</span> <span class="token keyword">range</span> w<span class="token punctuation">.</span>handlers <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> w<span class="token punctuation">.</span><span class="token function">shouldHandle</span><span class="token punctuation">(</span>handler<span class="token punctuation">,</span> event<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">if</span> err <span class="token operator">:=</span> handler<span class="token punctuation">.</span><span class="token function">Handle</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> event<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">                slog<span class="token punctuation">.</span><span class="token function">Warn</span><span class="token punctuation">(</span><span class="token string">"handler failed"</span><span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"handler"</span><span class="token punctuation">,</span> handler<span class="token punctuation">.</span><span class="token function">GetName</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"event_id"</span><span class="token punctuation">,</span> event<span class="token punctuation">[</span><span class="token string">"event_id"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"error"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">                handlerErr <span class="token operator">=</span> err</span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 处理失败，重试</span></span>
<span class="line">    <span class="token keyword">if</span> handlerErr <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        retryCount<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> event<span class="token punctuation">[</span><span class="token string">"retry_count"</span><span class="token punctuation">]</span><span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">float64</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token function">int</span><span class="token punctuation">(</span>retryCount<span class="token punctuation">)</span> <span class="token operator">>=</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>MaxRetries <span class="token punctuation">{</span></span>
<span class="line">            w<span class="token punctuation">.</span><span class="token function">sendToDLQ</span><span class="token punctuation">(</span>stream<span class="token punctuation">,</span> msg<span class="token punctuation">,</span> <span class="token string">"max_retries"</span><span class="token punctuation">,</span> handlerErr<span class="token punctuation">.</span><span class="token function">Error</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span> <span class="token keyword">else</span> <span class="token punctuation">{</span></span>
<span class="line">            event<span class="token punctuation">[</span><span class="token string">"retry_count"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token function">int</span><span class="token punctuation">(</span>retryCount<span class="token punctuation">)</span> <span class="token operator">+</span> <span class="token number">1</span></span>
<span class="line">            w<span class="token punctuation">.</span><span class="token function">requeue</span><span class="token punctuation">(</span>stream<span class="token punctuation">,</span> event<span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> w<span class="token punctuation">.</span><span class="token function">ack</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> stream<span class="token punctuation">,</span> msg<span class="token punctuation">.</span>ID<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// shouldHandle 判断处理器是否应该处理此事件</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">shouldHandle</span><span class="token punctuation">(</span>handler EventHandler<span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token builtin">bool</span> <span class="token punctuation">{</span></span>
<span class="line">    eventTypes <span class="token operator">:=</span> handler<span class="token punctuation">.</span><span class="token function">GetEventTypes</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>eventTypes<span class="token punctuation">)</span> <span class="token operator">==</span> <span class="token number">0</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">true</span> <span class="token comment">// 处理所有事件</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    eventType<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> event<span class="token punctuation">[</span><span class="token string">"event_type"</span><span class="token punctuation">]</span><span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">string</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> et <span class="token operator">:=</span> <span class="token keyword">range</span> eventTypes <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> et <span class="token operator">==</span> eventType <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">true</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">false</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ack 确认消息</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">ack</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> stream<span class="token punctuation">,</span> id <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> w<span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">XAck</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> stream<span class="token punctuation">,</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>ConsumerGroup<span class="token punctuation">,</span> id<span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Err</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// requeue 重新入队</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">requeue</span><span class="token punctuation">(</span>stream <span class="token builtin">string</span><span class="token punctuation">,</span> event Event<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    data<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> json<span class="token punctuation">.</span><span class="token function">Marshal</span><span class="token punctuation">(</span>event<span class="token punctuation">)</span></span>
<span class="line">    w<span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">XAdd</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token operator">&amp;</span>redis<span class="token punctuation">.</span>XAddArgs<span class="token punctuation">{</span></span>
<span class="line">        Stream<span class="token punctuation">:</span> stream<span class="token punctuation">,</span></span>
<span class="line">        Values<span class="token punctuation">:</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span><span class="token string">"data"</span><span class="token punctuation">:</span> <span class="token function">string</span><span class="token punctuation">(</span>data<span class="token punctuation">)</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// sendToDLQ 发送到死信队列</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">sendToDLQ</span><span class="token punctuation">(</span>stream <span class="token builtin">string</span><span class="token punctuation">,</span> msg redis<span class="token punctuation">.</span>XMessage<span class="token punctuation">,</span> reason<span class="token punctuation">,</span> details <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    deadEntry <span class="token operator">:=</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"original_stream"</span><span class="token punctuation">:</span> stream<span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"original_id"</span><span class="token punctuation">:</span>     msg<span class="token punctuation">.</span>ID<span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"reason"</span><span class="token punctuation">:</span>          reason<span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"details"</span><span class="token punctuation">:</span>         details<span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"failed_at"</span><span class="token punctuation">:</span>       time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Unix</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"original_data"</span><span class="token punctuation">:</span>   msg<span class="token punctuation">.</span>Values<span class="token punctuation">[</span><span class="token string">"data"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    w<span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">XAdd</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token operator">&amp;</span>redis<span class="token punctuation">.</span>XAddArgs<span class="token punctuation">{</span></span>
<span class="line">        Stream<span class="token punctuation">:</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>StreamDLQ<span class="token punctuation">,</span></span>
<span class="line">        Values<span class="token punctuation">:</span> deadEntry<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// processDLQ 处理死信队列 (后台任务)</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">processDLQ</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ticker <span class="token operator">:=</span> time<span class="token punctuation">.</span><span class="token function">NewTicker</span><span class="token punctuation">(</span><span class="token number">5</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Minute<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> ticker<span class="token punctuation">.</span><span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">for</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">select</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">case</span> <span class="token operator">&lt;-</span>w<span class="token punctuation">.</span>ctx<span class="token punctuation">.</span><span class="token function">Done</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token keyword">return</span></span>
<span class="line">        <span class="token keyword">case</span> <span class="token operator">&lt;-</span>ticker<span class="token punctuation">.</span>C<span class="token punctuation">:</span></span>
<span class="line">            <span class="token comment">// 扫描死信队列，可以进行告警或人工处理</span></span>
<span class="line">            slog<span class="token punctuation">.</span><span class="token function">Info</span><span class="token punctuation">(</span><span class="token string">"checking DLQ"</span><span class="token punctuation">,</span> <span class="token string">"stream"</span><span class="token punctuation">,</span> w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>StreamDLQ<span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// flushLoop 定期刷新 (用于批处理优化)</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>w <span class="token operator">*</span>Worker<span class="token punctuation">)</span> <span class="token function">flushLoop</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ticker <span class="token operator">:=</span> time<span class="token punctuation">.</span><span class="token function">NewTicker</span><span class="token punctuation">(</span>w<span class="token punctuation">.</span>config<span class="token punctuation">.</span>FlushInterval<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> ticker<span class="token punctuation">.</span><span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">for</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">select</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">case</span> <span class="token operator">&lt;-</span>w<span class="token punctuation">.</span>ctx<span class="token punctuation">.</span><span class="token function">Done</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token keyword">return</span></span>
<span class="line">        <span class="token keyword">case</span> <span class="token operator">&lt;-</span>ticker<span class="token punctuation">.</span>C<span class="token punctuation">:</span></span>
<span class="line">            <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> handler <span class="token operator">:=</span> <span class="token keyword">range</span> w<span class="token punctuation">.</span>handlers <span class="token punctuation">{</span></span>
<span class="line">                <span class="token keyword">if</span> flusher<span class="token punctuation">,</span> ok <span class="token operator">:=</span> handler<span class="token punctuation">.</span><span class="token punctuation">(</span>Flusher<span class="token punctuation">)</span><span class="token punctuation">;</span> ok <span class="token punctuation">{</span></span>
<span class="line">                    flusher<span class="token punctuation">.</span><span class="token function">Flush</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">                <span class="token punctuation">}</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Flusher 刷新器接口 (可选)</span></span>
<span class="line"><span class="token keyword">type</span> Flusher <span class="token keyword">interface</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">Flush</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-analytics-worker-数据分析订阅者" tabindex="-1"><a class="header-anchor" href="#_5-analytics-worker-数据分析订阅者"><span>5. Analytics Worker (数据分析订阅者)</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// cmd/analytics-worker/main.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"os"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/event/worker"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/event/handlers/analytics"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    config <span class="token operator">:=</span> worker<span class="token punctuation">.</span>WorkerConfig<span class="token punctuation">{</span></span>
<span class="line">        RedisURL<span class="token punctuation">:</span>        <span class="token function">getEnv</span><span class="token punctuation">(</span><span class="token string">"REDIS_URL"</span><span class="token punctuation">,</span> <span class="token string">"redis://localhost:6379/0"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        StreamEvents<span class="token punctuation">:</span>    <span class="token string">"events"</span><span class="token punctuation">,</span></span>
<span class="line">        StreamHighPrio<span class="token punctuation">:</span>  <span class="token string">"events:high_priority"</span><span class="token punctuation">,</span></span>
<span class="line">        StreamDLQ<span class="token punctuation">:</span>       <span class="token string">"events:dlq"</span><span class="token punctuation">,</span></span>
<span class="line">        ConsumerGroup<span class="token punctuation">:</span>   <span class="token string">"analytics-group"</span><span class="token punctuation">,</span></span>
<span class="line">        ConsumerName<span class="token punctuation">:</span>    <span class="token string">"analytics-worker"</span><span class="token punctuation">,</span></span>
<span class="line">        BatchSize<span class="token punctuation">:</span>       <span class="token number">100</span><span class="token punctuation">,</span></span>
<span class="line">        BlockTime<span class="token punctuation">:</span>       <span class="token number">2</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Second<span class="token punctuation">,</span></span>
<span class="line">        FlushInterval<span class="token punctuation">:</span>   <span class="token number">15</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Second<span class="token punctuation">,</span></span>
<span class="line">        MaxRetries<span class="token punctuation">:</span>      <span class="token number">3</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    w <span class="token operator">:=</span> worker<span class="token punctuation">.</span><span class="token function">NewWorker</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册分析处理器</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">RegisterHandler</span><span class="token punctuation">(</span>analytics<span class="token punctuation">.</span><span class="token function">NewClickHouseHandler</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">RegisterHandler</span><span class="token punctuation">(</span>analytics<span class="token punctuation">.</span><span class="token function">NewAggregationHandler</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    ctx<span class="token punctuation">,</span> cancel <span class="token operator">:=</span> signal<span class="token punctuation">.</span><span class="token function">NotifyContext</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> os<span class="token punctuation">.</span>Interrupt<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> <span class="token function">cancel</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> w<span class="token punctuation">.</span><span class="token function">Start</span><span class="token punctuation">(</span>ctx<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        slog<span class="token punctuation">.</span><span class="token function">Error</span><span class="token punctuation">(</span><span class="token string">"start worker"</span><span class="token punctuation">,</span> <span class="token string">"err"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">        os<span class="token punctuation">.</span><span class="token function">Exit</span><span class="token punctuation">(</span><span class="token number">1</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token operator">&lt;-</span>ctx<span class="token punctuation">.</span><span class="token function">Done</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_6-campaign-worker-活动系统订阅者" tabindex="-1"><a class="header-anchor" href="#_6-campaign-worker-活动系统订阅者"><span>6. Campaign Worker (活动系统订阅者)</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// cmd/campaign-worker/main.go</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"os"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/event/worker"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/event/handlers/campaign"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    config <span class="token operator">:=</span> worker<span class="token punctuation">.</span>WorkerConfig<span class="token punctuation">{</span></span>
<span class="line">        RedisURL<span class="token punctuation">:</span>        <span class="token function">getEnv</span><span class="token punctuation">(</span><span class="token string">"REDIS_URL"</span><span class="token punctuation">,</span> <span class="token string">"redis://localhost:6379/0"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        StreamEvents<span class="token punctuation">:</span>    <span class="token string">"events"</span><span class="token punctuation">,</span></span>
<span class="line">        StreamHighPrio<span class="token punctuation">:</span>  <span class="token string">"events:high_priority"</span><span class="token punctuation">,</span></span>
<span class="line">        StreamDLQ<span class="token punctuation">:</span>       <span class="token string">"events:dlq"</span><span class="token punctuation">,</span></span>
<span class="line">        ConsumerGroup<span class="token punctuation">:</span>   <span class="token string">"campaign-group"</span><span class="token punctuation">,</span>  <span class="token comment">// 不同的消费组</span></span>
<span class="line">        ConsumerName<span class="token punctuation">:</span>    <span class="token string">"campaign-worker"</span><span class="token punctuation">,</span></span>
<span class="line">        BatchSize<span class="token punctuation">:</span>       <span class="token number">50</span><span class="token punctuation">,</span></span>
<span class="line">        BlockTime<span class="token punctuation">:</span>       <span class="token number">1</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Second<span class="token punctuation">,</span></span>
<span class="line">        FlushInterval<span class="token punctuation">:</span>   <span class="token number">5</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Second<span class="token punctuation">,</span></span>
<span class="line">        MaxRetries<span class="token punctuation">:</span>      <span class="token number">5</span><span class="token punctuation">,</span>  <span class="token comment">// 活动系统更多重试</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    w <span class="token operator">:=</span> worker<span class="token punctuation">.</span><span class="token function">NewWorker</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册活动处理器</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">RegisterHandler</span><span class="token punctuation">(</span>campaign<span class="token punctuation">.</span><span class="token function">NewTriggerMatcher</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span>      <span class="token comment">// 触发器匹配</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">RegisterHandler</span><span class="token punctuation">(</span>campaign<span class="token punctuation">.</span><span class="token function">NewConditionEvaluator</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span>  <span class="token comment">// 条件评估</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">RegisterHandler</span><span class="token punctuation">(</span>campaign<span class="token punctuation">.</span><span class="token function">NewActionExecutor</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span>      <span class="token comment">// 动作执行</span></span>
<span class="line"></span>
<span class="line">    ctx<span class="token punctuation">,</span> cancel <span class="token operator">:=</span> signal<span class="token punctuation">.</span><span class="token function">NotifyContext</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> os<span class="token punctuation">.</span>Interrupt<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> <span class="token function">cancel</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> w<span class="token punctuation">.</span><span class="token function">Start</span><span class="token punctuation">(</span>ctx<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        slog<span class="token punctuation">.</span><span class="token function">Error</span><span class="token punctuation">(</span><span class="token string">"start worker"</span><span class="token punctuation">,</span> <span class="token string">"err"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">        os<span class="token punctuation">.</span><span class="token function">Exit</span><span class="token punctuation">(</span><span class="token number">1</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token operator">&lt;-</span>ctx<span class="token punctuation">.</span><span class="token function">Done</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    w<span class="token punctuation">.</span><span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="目录结构" tabindex="-1"><a class="header-anchor" href="#目录结构"><span>目录结构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">server/</span>
<span class="line">├── cmd/</span>
<span class="line">│   ├── event-gateway/          # 事件网关服务 (新增)</span>
<span class="line">│   │   └── main.go</span>
<span class="line">│   ├── analytics-worker/       # 数据分析 Worker (已有)</span>
<span class="line">│   ├── campaign-worker/        # 活动 Worker (新增)</span>
<span class="line">│   └── server/                 # 主服务器</span>
<span class="line">│</span>
<span class="line">├── internal/</span>
<span class="line">│   └── event/</span>
<span class="line">│       ├── gateway/            # 事件网关实现</span>
<span class="line">│       │   ├── collector.go    # HTTP/gRPC 收集器</span>
<span class="line">│       │   ├── validator.go    # Schema 验证</span>
<span class="line">│       │   ├── enricher.go     # 事件丰富化</span>
<span class="line">│       │   └── publisher.go    # 发布到 MQ</span>
<span class="line">│       │</span>
<span class="line">│       ├── types/              # 事件类型定义</span>
<span class="line">│       │   ├── event_type.go</span>
<span class="line">│       │   └── schema.go</span>
<span class="line">│       │</span>
<span class="line">│       ├── worker/             # Worker 基础框架</span>
<span class="line">│       │   └── worker.go</span>
<span class="line">│       │</span>
<span class="line">│       └── handlers/           # 事件处理器</span>
<span class="line">│           ├── analytics/      # 数据分析处理器</span>
<span class="line">│           │   ├── clickhouse.go</span>
<span class="line">│           │   └── aggregation.go</span>
<span class="line">│           │</span>
<span class="line">│           └── campaign/       # 活动处理器</span>
<span class="line">│               ├── trigger.go</span>
<span class="line">│               ├── condition.go</span>
<span class="line">│               └── action.go</span>
<span class="line">│</span>
<span class="line">├── proto/</span>
<span class="line">│   └── event/</span>
<span class="line">│       └── v1/</span>
<span class="line">│           └── event_gateway.proto</span>
<span class="line">│</span>
<span class="line">└── docs/</span>
<span class="line">    ├── event-driven-architecture.md    # 本文档</span>
<span class="line">    ├── event-types.md                  # 事件类型文档</span>
<span class="line">    └── campaign-system.md              # 活动系统文档</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="部署架构" tabindex="-1"><a class="header-anchor" href="#部署架构"><span>部署架构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────────────────────────────────────────────┐</span>
<span class="line">│                                  部署视图                                        │</span>
<span class="line">├─────────────────────────────────────────────────────────────────────────────────┤</span>
<span class="line">│                                                                                 │</span>
<span class="line">│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐            │</span>
<span class="line">│  │   Game Server   │    │   Game Server   │    │   Game Server   │            │</span>
<span class="line">│  │   (Region: CN)  │    │   (Region: US)  │    │   (Region: EU)  │            │</span>
<span class="line">│  └────────┬────────┘    └────────┬────────┘    └────────┬────────┘            │</span>
<span class="line">│           │                      │                      │                       │</span>
<span class="line">│           └──────────────────────┼──────────────────────┘                       │</span>
<span class="line">│                                  │                                               │</span>
<span class="line">│                    ┌─────────────▼──────────────┐                               │</span>
<span class="line">│                    │   Event Gateway (LB)       │                               │</span>
<span class="line">│                    │   3 instances               │                               │</span>
<span class="line">│                    └─────────────┬──────────────┘                               │</span>
<span class="line">│                                  │                                               │</span>
<span class="line">│                    ┌─────────────▼──────────────┐                               │</span>
<span class="line">│                    │   Redis Cluster            │                               │</span>
<span class="line">│                    │   (Event Bus)              │                               │</span>
<span class="line">│                    └─────────────┬──────────────┘                               │</span>
<span class="line">│                                  │                                               │</span>
<span class="line">│           ┌──────────────────────┼──────────────────────┐                      │</span>
<span class="line">│           │                      │                      │                       │</span>
<span class="line">│  ┌────────▼─────────┐  ┌────────▼─────────┐  ┌────────▼─────────┐             │</span>
<span class="line">│  │ Analytics Worker │  │ Campaign Worker  │  │ (Future) Worker  │             │</span>
<span class="line">│  │ 3 instances      │  │ 2 instances      │  │                  │             │</span>
<span class="line">│  └──────────────────┘  └──────────────────┘  └──────────────────┘             │</span>
<span class="line">│           │                      │                                              │</span>
<span class="line">│           ▼                      ▼                                              │</span>
<span class="line">│  ┌──────────────────┐  ┌──────────────────┐                                     │</span>
<span class="line">│  │ ClickHouse       │  │ Game DB          │                                     │</span>
<span class="line">│  └──────────────────┘  └──────────────────┘                                     │</span>
<span class="line">│                                                                                 │</span>
<span class="line">└─────────────────────────────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="迁移计划" tabindex="-1"><a class="header-anchor" href="#迁移计划"><span>迁移计划</span></a></h2>
<h3 id="阶段-1-创建-event-gateway-不影响现有" tabindex="-1"><a class="header-anchor" href="#阶段-1-创建-event-gateway-不影响现有"><span>阶段 1: 创建 Event Gateway (不影响现有)</span></a></h3>
<ol>
<li>实现 <code v-pre>cmd/event-gateway</code></li>
<li>定义事件 Schema</li>
<li>部署独立服务</li>
</ol>
<h3 id="阶段-2-重构-analytics-worker" tabindex="-1"><a class="header-anchor" href="#阶段-2-重构-analytics-worker"><span>阶段 2: 重构 Analytics Worker</span></a></h3>
<ol>
<li>重构现有 <code v-pre>analytics-worker</code> 使用新 Worker 基类</li>
<li>验证功能不变</li>
</ol>
<h3 id="阶段-3-实现-campaign-worker" tabindex="-1"><a class="header-anchor" href="#阶段-3-实现-campaign-worker"><span>阶段 3: 实现 Campaign Worker</span></a></h3>
<ol>
<li>创建 <code v-pre>cmd/campaign-worker</code></li>
<li>实现触发器、条件、动作处理器</li>
<li>联调测试</li>
</ol>
<h3 id="阶段-4-客户端迁移-可选" tabindex="-1"><a class="header-anchor" href="#阶段-4-客户端迁移-可选"><span>阶段 4: 客户端迁移 (可选)</span></a></h3>
<ol>
<li>SDK 更新，支持直接上报到 Event Gateway</li>
<li>游戏服务器继续通过 MQ 上报 (向后兼容)</li>
</ol>
<h2 id="配置示例" tabindex="-1"><a class="header-anchor" href="#配置示例"><span>配置示例</span></a></h2>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># configs/event-gateway.yaml</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">http_port</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">  <span class="token key atrule">grpc_port</span><span class="token punctuation">:</span> <span class="token number">9090</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">redis</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">url</span><span class="token punctuation">:</span> <span class="token string">"redis://localhost:6379/0"</span></span>
<span class="line">  <span class="token key atrule">stream_events</span><span class="token punctuation">:</span> <span class="token string">"events"</span></span>
<span class="line">  <span class="token key atrule">stream_high_priority</span><span class="token punctuation">:</span> <span class="token string">"events:high_priority"</span></span>
<span class="line">  <span class="token key atrule">stream_dlq</span><span class="token punctuation">:</span> <span class="token string">"events:dlq"</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">validation</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">strict_mode</span><span class="token punctuation">:</span> <span class="token boolean important">false</span>  <span class="token comment"># false=忽略未知字段, true=拒绝</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">enrichment</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">geo_ip_enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">user_agent_enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">rate_limit</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">requests_per_second</span><span class="token punctuation">:</span> <span class="token number">10000</span></span>
<span class="line">  <span class="token key atrule">burst</span><span class="token punctuation">:</span> <span class="token number">20000</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="监控指标" tabindex="-1"><a class="header-anchor" href="#监控指标"><span>监控指标</span></a></h2>
<table>
<thead>
<tr>
<th>指标</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>event_gateway_events_total</code></td>
<td>接收事件总数</td>
</tr>
<tr>
<td><code v-pre>event_gateway_events_by_type</code></td>
<td>按类型统计</td>
</tr>
<tr>
<td><code v-pre>event_gateway_publish_duration</code></td>
<td>发布耗时</td>
</tr>
<tr>
<td><code v-pre>worker_processed_total</code></td>
<td>Worker 处理总数</td>
</tr>
<tr>
<td><code v-pre>worker_failed_total</code></td>
<td>处理失败数</td>
</tr>
<tr>
<td><code v-pre>worker_dlq_size</code></td>
<td>死信队列大小</td>
</tr>
</tbody>
</table>
</div></template>


