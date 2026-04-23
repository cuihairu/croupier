<template><div><h1 id="第三方运营平台接入架构设计" tabindex="-1"><a class="header-anchor" href="#第三方运营平台接入架构设计"><span>第三方运营平台接入架构设计</span></a></h1>
<h2 id="概述" tabindex="-1"><a class="header-anchor" href="#概述"><span>概述</span></a></h2>
<p>本文档描述 Croupier 中可扩展的第三方运营平台接入架构，支持动态配置和启用多个运营平台。</p>
<h3 id="架构演进" tabindex="-1"><a class="header-anchor" href="#架构演进"><span>架构演进</span></a></h3>
<p><strong>重要架构变更</strong>：OpenAPI Provider 现已支持 <strong>Agent 侧部署</strong>，以访问内网游戏服务器 API。</p>
<table>
<thead>
<tr>
<th>Provider</th>
<th>类型</th>
<th>部署位置</th>
<th>说明</th>
<th>状态</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>QuickSDK</strong></td>
<td>专用</td>
<td>Server</td>
<td>游戏运营数据平台，20+ API</td>
<td>✅ 已实现</td>
</tr>
<tr>
<td><strong>OpenAPI</strong></td>
<td>通用</td>
<td>Agent</td>
<td>任意 HTTP API，配置驱动</td>
<td>✅ 已实现</td>
</tr>
</tbody>
</table>
<h3 id="快速选择" tabindex="-1"><a class="header-anchor" href="#快速选择"><span>快速选择</span></a></h3>
<ul>
<li><strong>有 SDK 的第三方平台</strong> → 编写专用 Provider（如 QuickSDK）</li>
<li><strong>只有 HTTP API 的内网游戏服务器</strong> → 使用 OpenAPI Provider 部署在 Agent 侧</li>
<li><strong>只有 HTTP API 的公网服务</strong> → 可以使用 OpenAPI Provider 部署在 Agent 侧（通过 Agent 出网）</li>
</ul>
<h2 id="架构设计" tabindex="-1"><a class="header-anchor" href="#架构设计"><span>架构设计</span></a></h2>
<h3 id="_1-目录结构" tabindex="-1"><a class="header-anchor" href="#_1-目录结构"><span>1. 目录结构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">server/</span>
<span class="line">├── internal/</span>
<span class="line">│   ├── platform/</span>
<span class="line">│   │   ├── provider/           # Provider 接口定义</span>
<span class="line">│   │   │   ├── provider.go     # Provider 接口</span>
<span class="line">│   │   │   └── registry.go     # Provider 注册表（Server 侧）</span>
<span class="line">│   │   ├── quicksdk/           # QuickSDK 实现（Server 侧）</span>
<span class="line">│   │   │   ├── client.go       # HTTP 客户端</span>
<span class="line">│   │   │   ├── sign.go         # 签名算法</span>
<span class="line">│   │   │   ├── api.go          # 20个 API 实现</span>
<span class="line">│   │   │   └── provider.go     # QuickSDK Provider 实现</span>
<span class="line">│   │   ├── openapi/            # OpenAPI 通用 Provider</span>
<span class="line">│   │   │   └── provider.go     # OpenAPI Provider 实现（Agent 侧使用）</span>
<span class="line">│   │   ├── ratelimit/          # 速率限制工具</span>
<span class="line">│   │   │   └── tokenbucket.go  # 令牌桶实现</span>
<span class="line">│   │   ├── loader.go           # Server 侧配置加载器</span>
<span class="line">│   │   └── server.go           # Platform gRPC 服务</span>
<span class="line">│   └── app/agent/</span>
<span class="line">│       ├── platform.go         # Agent 侧 PlatformManager</span>
<span class="line">│       ├── app.go              # 集成 PlatformManager</span>
<span class="line">│       └── function_server.go  # 支持平台调用</span>
<span class="line">├── configs/</span>
<span class="line">│   └── platforms.yaml          # Agent 侧平台配置文件</span>
<span class="line">└── pkg/pb/platform/v1/         # gRPC 生成代码</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_1-1-agent-侧-platform-架构" tabindex="-1"><a class="header-anchor" href="#_1-1-agent-侧-platform-架构"><span>1.1 Agent 侧 Platform 架构</span></a></h3>
<p>Agent 侧的 Platform 架构复用了现有的 SDK 注册机制：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────────────────────────────┐</span>
<span class="line">│                         Croupier Agent                          │</span>
<span class="line">├─────────────────────────────────────────────────────────────────┤</span>
<span class="line">│  ┌──────────────────┐    ┌─────────────────────────────────┐   │</span>
<span class="line">│  │ PlatformManager  │    │       LocalStore                │   │</span>
<span class="line">│  │                  │    │  (function_id -> Instance[])    │   │</span>
<span class="line">│  │ - Load YAML      │    │                                 │   │</span>
<span class="line">│  │ - Init Providers │◄───┤  game_server.get_role           │   │</span>
<span class="line">│  │ - Register()     │    │  game_server.ban_user           │   │</span>
<span class="line">│  └────────┬─────────┘    └──────────────┬──────────────────┘   │</span>
<span class="line">│           │                             │                         │</span>
<span class="line">│           ▼                             ▼                         │</span>
<span class="line">│  ┌─────────────────────────────────────────────────────────┐    │</span>
<span class="line">│  │              FunctionServer.Invoke()                     │    │</span>
<span class="line">│  │  ┌─────────────────┐    ┌─────────────────────────┐    │    │</span>
<span class="line">│  │  │ Platform Call?  │ NO │  gRPC Forward           │    │    │</span>
<span class="line">│  │  │ (prefix check)  │───►│  to game SDK            │    │    │</span>
<span class="line">│  │  └────┬─────▲──────┘    └──────────────────────────┘    │    │</span>
<span class="line">│  │       │     │YES                                              │</span>
<span class="line">│  │       │     └──► Provider.Call() → HTTP to Game Server       │</span>
<span class="line">│  └─────────────────────────────────────────────────────────┘    │</span>
<span class="line">└─────────────────────────────────────────────────────────────────┘</span>
<span class="line">           │                                       │</span>
<span class="line">           ▼                                       ▼</span>
<span class="line">    ┌─────────────┐                        ┌──────────────┐</span>
<span class="line">    │ Upstream    │                        │ Game Server  │</span>
<span class="line">    │ Register    │                        │ HTTP API     │</span>
<span class="line">    └─────────────┘                        └──────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>关键设计点：</strong></p>
<ol>
<li><strong>复用注册机制</strong>：Platform 方法注册为 Function，与 SDK 完全一致</li>
<li><strong>ServiceID 标识</strong>：Platform 函数使用 <code v-pre>platform:&lt;name&gt;</code> 作为 ServiceID</li>
<li><strong>请求拦截</strong>：FunctionServer.Invoke 检测平台函数，直接调用 Provider</li>
<li><strong>代码复用</strong>：<code v-pre>internal/platform/openapi/provider.go</code> 在 Agent 和 Server 共享</li>
</ol>
<h3 id="_2-核心-api-设计" tabindex="-1"><a class="header-anchor" href="#_2-核心-api-设计"><span>2. 核心 API 设计</span></a></h3>
<h4 id="_2-1-provider-接口" tabindex="-1"><a class="header-anchor" href="#_2-1-provider-接口"><span>2.1 Provider 接口</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> provider</span>
<span class="line"></span>
<span class="line"><span class="token comment">// Provider 定义第三方运营平台接口</span></span>
<span class="line"><span class="token keyword">type</span> Provider <span class="token keyword">interface</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// Name 返回平台名称</span></span>
<span class="line">    <span class="token function">Name</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">string</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Init 初始化 Provider</span></span>
<span class="line">    <span class="token function">Init</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> config ProviderConfig<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// IsEnabled 检查是否启用</span></span>
<span class="line">    <span class="token function">IsEnabled</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">bool</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// SupportedMethods 返回支持的方法列表</span></span>
<span class="line">    <span class="token function">SupportedMethods</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Call 调用平台 API</span></span>
<span class="line">    <span class="token function">Call</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> method <span class="token builtin">string</span><span class="token punctuation">,</span> request <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">byte</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">byte</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Close 关闭 Provider</span></span>
<span class="line">    <span class="token function">Close</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ProviderConfig 平台配置</span></span>
<span class="line"><span class="token keyword">type</span> ProviderConfig <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Enabled   <span class="token builtin">bool</span>                   <span class="token string">`yaml:"enabled" json:"enabled"`</span></span>
<span class="line">    Type      <span class="token builtin">string</span>                 <span class="token string">`yaml:"type" json:"type"`</span>       <span class="token comment">// "quicksdk", "thinkingdata", etc.</span></span>
<span class="line">    Config    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span> <span class="token string">`yaml:"config" json:"config"`</span>   <span class="token comment">// 平台特定配置</span></span>
<span class="line">    RateLimit <span class="token operator">*</span>RateLimitConfig       <span class="token string">`yaml:"rate_limit" json:"rate_limit"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// RateLimitConfig 速率限制配置</span></span>
<span class="line"><span class="token keyword">type</span> RateLimitConfig <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    RequestsPerMinute <span class="token builtin">int</span> <span class="token string">`yaml:"requests_per_minute" json:"requests_per_minute"`</span></span>
<span class="line">    BurstSize         <span class="token builtin">int</span> <span class="token string">`yaml:"burst_size" json:"burst_size"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_2-2-registry-注册表" tabindex="-1"><a class="header-anchor" href="#_2-2-registry-注册表"><span>2.2 Registry 注册表</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// Registry 管理所有 Provider</span></span>
<span class="line"><span class="token keyword">type</span> Registry <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    mu        sync<span class="token punctuation">.</span>RWMutex</span>
<span class="line">    providers <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>Provider  <span class="token comment">// key: platform name</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Register 注册新的 Provider</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>Registry<span class="token punctuation">)</span> <span class="token function">Register</span><span class="token punctuation">(</span>p Provider<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Unregister 注销 Provider</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>Registry<span class="token punctuation">)</span> <span class="token function">Unregister</span><span class="token punctuation">(</span>name <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Get 获取 Provider</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>Registry<span class="token punctuation">)</span> <span class="token function">Get</span><span class="token punctuation">(</span>name <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token punctuation">(</span>Provider<span class="token punctuation">,</span> <span class="token builtin">bool</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// List 列出所有 Provider</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>Registry<span class="token punctuation">)</span> <span class="token function">List</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">[</span><span class="token punctuation">]</span>Provider</span>
<span class="line"></span>
<span class="line"><span class="token comment">// Call 调用指定平台的方法</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>Registry<span class="token punctuation">)</span> <span class="token function">Call</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> platform<span class="token punctuation">,</span> method <span class="token builtin">string</span><span class="token punctuation">,</span> request<span class="token punctuation">,</span> response <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-quicksdk-provider-设计" tabindex="-1"><a class="header-anchor" href="#_3-quicksdk-provider-设计"><span>3. QuickSDK Provider 设计</span></a></h3>
<h4 id="_3-1-配置" tabindex="-1"><a class="header-anchor" href="#_3-1-配置"><span>3.1 配置</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">quicksdk</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">type</span><span class="token punctuation">:</span> quicksdk</span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">open_id</span><span class="token punctuation">:</span> <span class="token string">"${QUICKSDK_OPEN_ID}"</span></span>
<span class="line">      <span class="token key atrule">open_key</span><span class="token punctuation">:</span> <span class="token string">"${QUICKSDK_OPEN_KEY}"</span></span>
<span class="line">      <span class="token key atrule">api_base_url</span><span class="token punctuation">:</span> <span class="token string">"https://www.quicksdk.com"</span></span>
<span class="line">      <span class="token key atrule">timeout</span><span class="token punctuation">:</span> 30s</span>
<span class="line">      <span class="token key atrule">retry_count</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">      <span class="token key atrule">enable_cache</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">cache_duration</span><span class="token punctuation">:</span> 300s</span>
<span class="line">    <span class="token key atrule">rate_limit</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">requests_per_minute</span><span class="token punctuation">:</span> <span class="token number">1000</span></span>
<span class="line">      <span class="token key atrule">burst_size</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_3-2-支持的方法" tabindex="-1"><a class="header-anchor" href="#_3-2-支持的方法"><span>3.2 支持的方法</span></a></h4>
<table>
<thead>
<tr>
<th>方法</th>
<th>QuickSDK 接口</th>
<th>描述</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>channel_list</code></td>
<td>open/channelList</td>
<td>获取渠道列表</td>
</tr>
<tr>
<td><code v-pre>server_list</code></td>
<td>open/serverList</td>
<td>获取区服列表</td>
</tr>
<tr>
<td><code v-pre>product_list</code></td>
<td>open/productList</td>
<td>获取产品列表</td>
</tr>
<tr>
<td><code v-pre>role_info</code></td>
<td>open/roleInfo</td>
<td>获取角色信息</td>
</tr>
<tr>
<td><code v-pre>order_list</code></td>
<td>open/orderList</td>
<td>获取订单列表</td>
</tr>
<tr>
<td><code v-pre>day_report</code></td>
<td>open/dayReport</td>
<td>单日报表</td>
</tr>
<tr>
<td><code v-pre>day_hour_report</code></td>
<td>open/dayHourReport</td>
<td>每小时报表</td>
</tr>
<tr>
<td><code v-pre>user_live</code></td>
<td>open/userLive</td>
<td>玩家留存</td>
</tr>
<tr>
<td><code v-pre>channel_days_report</code></td>
<td>open/channelDaysReport</td>
<td>渠道报表</td>
</tr>
<tr>
<td><code v-pre>channel_report</code></td>
<td>open/channelReport</td>
<td>渠道日报</td>
</tr>
<tr>
<td><code v-pre>ad_report</code></td>
<td>open/adReport</td>
<td>广告效果报表</td>
</tr>
<tr>
<td><code v-pre>media_app_list</code></td>
<td>open/getMediaApp</td>
<td>广告主列表</td>
</tr>
<tr>
<td><code v-pre>ad_plan_group_list</code></td>
<td>open/getAdPlanGroup</td>
<td>广告分组列表</td>
</tr>
<tr>
<td><code v-pre>package_version_list</code></td>
<td>open/getPackageVersion</td>
<td>分包列表</td>
</tr>
<tr>
<td><code v-pre>ad_pages_list</code></td>
<td>open/getAdPages</td>
<td>落地页列表</td>
</tr>
<tr>
<td><code v-pre>create_ad_plan</code></td>
<td>open/createAdPlan</td>
<td>创建广告计划</td>
</tr>
<tr>
<td><code v-pre>update_ad_plan</code></td>
<td>open/updateAdPlan</td>
<td>更新广告计划</td>
</tr>
<tr>
<td><code v-pre>ad_plan_list</code></td>
<td>open/getAdPlan</td>
<td>广告计划列表</td>
</tr>
<tr>
<td><code v-pre>user_lost_list</code></td>
<td>open/uwlLost</td>
<td>流失预警</td>
</tr>
<tr>
<td><code v-pre>push_message</code></td>
<td>open/pushMessage</td>
<td>消息推送</td>
</tr>
</tbody>
</table>
<h3 id="_4-grpc-api-定义" tabindex="-1"><a class="header-anchor" href="#_4-grpc-api-定义"><span>4. gRPC API 定义</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">syntax</span> <span class="token operator">=</span> <span class="token string">"proto3"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> croupier<span class="token punctuation">.</span>platform<span class="token punctuation">.</span>v1<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">option</span> go_package <span class="token operator">=</span> <span class="token string">"github.com/cuihairu/croupier/gen/go/croupier/platform/v1"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Platform 服务</span></span>
<span class="line"><span class="token keyword">service</span> <span class="token class-name">PlatformService</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 调用第三方平台 API</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">CallPlatform</span><span class="token punctuation">(</span><span class="token class-name">CallPlatformRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">CallPlatformResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取支持的平台列表</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">ListPlatforms</span><span class="token punctuation">(</span><span class="token class-name">ListPlatformsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">ListPlatformsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取平台支持的方法列表</span></span>
<span class="line">    <span class="token keyword">rpc</span> <span class="token function">ListPlatformMethods</span><span class="token punctuation">(</span><span class="token class-name">ListPlatformMethodsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">ListPlatformMethodsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">CallPlatformRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> platform <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>    <span class="token comment">// 平台名称: "quicksdk"</span></span>
<span class="line">    <span class="token builtin">string</span> method <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>      <span class="token comment">// 方法名: "day_report"</span></span>
<span class="line">    <span class="token builtin">bytes</span> request <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>      <span class="token comment">// 请求参数 (JSON)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">CallPlatformResponse</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">bytes</span> response <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>     <span class="token comment">// 响应数据 (JSON)</span></span>
<span class="line">    <span class="token builtin">string</span> error <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>       <span class="token comment">// 错误信息</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ListPlatformsRequest</span> <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">PlatformInfo</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> name <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">bool</span> enabled <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> methods <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ListPlatformsResponse</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">PlatformInfo</span> platforms <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ListPlatformMethodsRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> platform <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ListPlatformMethodsResponse</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token builtin">string</span> methods <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-使用示例" tabindex="-1"><a class="header-anchor" href="#_5-使用示例"><span>5. 使用示例</span></a></h3>
<h4 id="_5-1-代码调用" tabindex="-1"><a class="header-anchor" href="#_5-1-代码调用"><span>5.1 代码调用</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 通过 Registry 调用</span></span>
<span class="line">registry <span class="token operator">:=</span> platform<span class="token punctuation">.</span><span class="token function">NewRegistry</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">err <span class="token operator">:=</span> registry<span class="token punctuation">.</span><span class="token function">Call</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token string">"quicksdk"</span><span class="token punctuation">,</span> <span class="token string">"day_report"</span><span class="token punctuation">,</span> request<span class="token punctuation">,</span> <span class="token operator">&amp;</span>response<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 直接通过 Service 调用</span></span>
<span class="line">client <span class="token operator">:=</span> platformv1<span class="token punctuation">.</span><span class="token function">NewPlatformServiceClient</span><span class="token punctuation">(</span>conn<span class="token punctuation">)</span></span>
<span class="line">resp<span class="token punctuation">,</span> err <span class="token operator">:=</span> client<span class="token punctuation">.</span><span class="token function">CallPlatform</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>platformv1<span class="token punctuation">.</span>CallPlatformRequest<span class="token punctuation">{</span></span>
<span class="line">    Platform<span class="token punctuation">:</span> <span class="token string">"quicksdk"</span><span class="token punctuation">,</span></span>
<span class="line">    Method<span class="token punctuation">:</span>   <span class="token string">"day_report"</span><span class="token punctuation">,</span></span>
<span class="line">    Request<span class="token punctuation">:</span>  jsonRequest<span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_5-2-http-api" tabindex="-1"><a class="header-anchor" href="#_5-2-http-api"><span>5.2 HTTP API</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">POST /api/v1/platform/call</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">    <span class="token string">"platform"</span><span class="token builtin class-name">:</span> <span class="token string">"quicksdk"</span>,</span>
<span class="line">    <span class="token string">"method"</span><span class="token builtin class-name">:</span> <span class="token string">"day_report"</span>,</span>
<span class="line">    <span class="token string">"request"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"productCode"</span><span class="token builtin class-name">:</span> <span class="token string">"xxx"</span>,</span>
<span class="line">        <span class="token string">"bTime"</span><span class="token builtin class-name">:</span> <span class="token number">1704067200</span>,</span>
<span class="line">        <span class="token string">"eTime"</span><span class="token builtin class-name">:</span> <span class="token number">1704153600</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="实现计划" tabindex="-1"><a class="header-anchor" href="#实现计划"><span>实现计划</span></a></h2>
<h3 id="phase-1-基础框架-2-3-人日" tabindex="-1"><a class="header-anchor" href="#phase-1-基础框架-2-3-人日"><span>Phase 1: 基础框架 (2-3 人日)</span></a></h3>
<ul>
<li>[x] Provider 接口定义</li>
<li>[x] Registry 实现</li>
<li>[x] 配置加载</li>
<li>[x] gRPC Proto 定义</li>
</ul>
<h3 id="phase-2-quicksdk-实现-4-5-人日" tabindex="-1"><a class="header-anchor" href="#phase-2-quicksdk-实现-4-5-人日"><span>Phase 2: QuickSDK 实现 (4-5 人日)</span></a></h3>
<ul>
<li>[x] HTTP 客户端 + 签名</li>
<li>[x] 20 个 API 实现</li>
<li>[x] 缓存支持</li>
<li>[x] 速率限制</li>
</ul>
<h3 id="phase-3-集成-2-3-人日" tabindex="-1"><a class="header-anchor" href="#phase-3-集成-2-3-人日"><span>Phase 3: 集成 (2-3 人日)</span></a></h3>
<ul>
<li>[x] 集成到 Server</li>
<li>[x] HTTP API 端点</li>
<li>[x] 前端 UI 支持</li>
</ul>
<h3 id="phase-4-agent-侧-openapi-支持-1-2-人日" tabindex="-1"><a class="header-anchor" href="#phase-4-agent-侧-openapi-支持-1-2-人日"><span>Phase 4: Agent 侧 OpenAPI 支持 (1-2 人日)</span></a></h3>
<ul>
<li>[x] Agent PlatformManager 实现</li>
<li>[x] FunctionServer 平台调用拦截</li>
<li>[x] Agent 侧 YAML 配置加载</li>
<li>[x] 复用 openapi.Provider 代码</li>
</ul>
<h3 id="phase-5-openapi-通用-provider-完善-1-2-人日" tabindex="-1"><a class="header-anchor" href="#phase-5-openapi-通用-provider-完善-1-2-人日"><span>Phase 5: OpenAPI 通用 Provider 完善 (1-2 人日)</span></a></h3>
<ul>
<li>[x] OpenAPI Provider 实现</li>
<li>[x] 配置示例</li>
<li>[x] 设计文档更新</li>
<li>[ ] OpenAPI 规范自动发现完善</li>
</ul>
<h3 id="phase-6-测试与文档-1-2-人日" tabindex="-1"><a class="header-anchor" href="#phase-6-测试与文档-1-2-人日"><span>Phase 6: 测试与文档 (1-2 人日)</span></a></h3>
<ul>
<li>[ ] 单元测试</li>
<li>[ ] 集成测试</li>
<li>[ ] 使用文档</li>
</ul>
<p><strong>已完成: Phase 1, Phase 2, Phase 3, Phase 4</strong></p>
<h2 id="已实现的-provider" tabindex="-1"><a class="header-anchor" href="#已实现的-provider"><span>已实现的 Provider</span></a></h2>
<h3 id="_1-quicksdk-provider" tabindex="-1"><a class="header-anchor" href="#_1-quicksdk-provider"><span>1. QuickSDK Provider</span></a></h3>
<p>专用 Provider，对接 QuickSDK 游戏运营数据平台。</p>
<p><strong>配置：</strong></p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">quicksdk</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">type</span><span class="token punctuation">:</span> quicksdk</span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">open_id</span><span class="token punctuation">:</span> <span class="token string">"${QUICKSDK_OPEN_ID}"</span></span>
<span class="line">      <span class="token key atrule">open_key</span><span class="token punctuation">:</span> <span class="token string">"${QUICKSDK_OPEN_KEY}"</span></span>
<span class="line">      <span class="token key atrule">api_base_url</span><span class="token punctuation">:</span> <span class="token string">"https://www.quicksdk.com"</span></span>
<span class="line">      <span class="token key atrule">timeout</span><span class="token punctuation">:</span> 30s</span>
<span class="line">      <span class="token key atrule">retry_count</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">      <span class="token key atrule">enable_cache</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">cache_duration</span><span class="token punctuation">:</span> 300s</span>
<span class="line">    <span class="token key atrule">rate_limit</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">requests_per_minute</span><span class="token punctuation">:</span> <span class="token number">1000</span></span>
<span class="line">      <span class="token key atrule">burst_size</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>支持的方法（20+）：</strong></p>
<table>
<thead>
<tr>
<th>分类</th>
<th>方法</th>
</tr>
</thead>
<tbody>
<tr>
<td>基础数据</td>
<td><code v-pre>channel_list</code>, <code v-pre>server_list</code>, <code v-pre>product_list</code>, <code v-pre>role_info</code>, <code v-pre>order_list</code></td>
</tr>
<tr>
<td>运营报表</td>
<td><code v-pre>day_report</code>, <code v-pre>day_hour_report</code>, <code v-pre>user_live</code>, <code v-pre>channel_days_report</code>, <code v-pre>channel_report</code></td>
</tr>
<tr>
<td>广告管理</td>
<td><code v-pre>ad_report</code>, <code v-pre>media_app_list</code>, <code v-pre>ad_plan_group_list</code>, <code v-pre>create_ad_plan</code>, <code v-pre>update_ad_plan</code></td>
</tr>
<tr>
<td>其他</td>
<td><code v-pre>user_lost_list</code>, <code v-pre>push_message</code></td>
</tr>
</tbody>
</table>
<h3 id="_2-openapi-provider" tabindex="-1"><a class="header-anchor" href="#_2-openapi-provider"><span>2. OpenAPI Provider</span></a></h3>
<p>通用 Provider，支持任意 HTTP API，无需编写代码。</p>
<p><strong>特点：</strong></p>
<ul>
<li>✅ 配置驱动，无需编码</li>
<li>✅ 多种认证方式</li>
<li>✅ 灵活参数映射</li>
<li>✅ 自动发现 OpenAPI/Swagger 接口</li>
<li>✅ 请求/响应转换</li>
<li>✅ 内置重试和限流</li>
</ul>
<p>详见下文「OpenAPI 通用 Provider」章节。</p>
<h2 id="未来扩展" tabindex="-1"><a class="header-anchor" href="#未来扩展"><span>未来扩展</span></a></h2>
<h3 id="添加新的专用-provider" tabindex="-1"><a class="header-anchor" href="#添加新的专用-provider"><span>添加新的专用 Provider</span></a></h3>
<p>需要编写代码的场景：</p>
<ol>
<li>实现 <code v-pre>Provider</code> 接口</li>
<li>在 <code v-pre>loader.go</code> 中注册类型</li>
<li>在配置文件中添加配置</li>
</ol>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 示例：添加 ThinkingData</span></span>
<span class="line"><span class="token keyword">type</span> ThinkingDataProvider <span class="token keyword">struct</span> <span class="token punctuation">{</span> <span class="token operator">...</span> <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>t <span class="token operator">*</span>ThinkingDataProvider<span class="token punctuation">)</span> <span class="token function">Name</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">string</span> <span class="token punctuation">{</span> <span class="token keyword">return</span> <span class="token string">"thinkingdata"</span> <span class="token punctuation">}</span></span>
<span class="line"><span class="token comment">// ... 实现其他接口方法</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">thinkingdata</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">type</span><span class="token punctuation">:</span> thinkingdata</span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">app_id</span><span class="token punctuation">:</span> <span class="token string">"xxx"</span></span>
<span class="line">      <span class="token key atrule">app_key</span><span class="token punctuation">:</span> <span class="token string">"xxx"</span></span>
<span class="line">      <span class="token key atrule">server_url</span><span class="token punctuation">:</span> <span class="token string">"https://xxx.thinkingdata.cn"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="openapi-通用-provider-推荐" tabindex="-1"><a class="header-anchor" href="#openapi-通用-provider-推荐"><span>OpenAPI 通用 Provider（推荐）</span></a></h3>
<p>对于有 HTTP API 但无 SDK 的服务器，使用 OpenAPI Provider 无需编写代码。</p>
<hr>
<h2 id="openapi-通用-provider-详解" tabindex="-1"><a class="header-anchor" href="#openapi-通用-provider-详解"><span>OpenAPI 通用 Provider 详解</span></a></h2>
<h3 id="概述-1" tabindex="-1"><a class="header-anchor" href="#概述-1"><span>概述</span></a></h3>
<p>OpenAPI Provider 是一个通用 HTTP API 客户端，通过 YAML 配置即可接入任意 HTTP 服务。</p>
<p><strong>适用场景：</strong></p>
<ul>
<li>游戏服务器管理 API</li>
<li>内部管理后台接口</li>
<li>第三方 OpenAPI/Swagger 服务</li>
<li>快速原型验证</li>
</ul>
<p><strong>核心优势：</strong></p>
<ul>
<li>✅ <strong>零代码</strong> - 纯配置驱动</li>
<li>✅ <strong>灵活认证</strong> - 支持 Bearer、Basic、API Key、自定义 Header</li>
<li>✅ <strong>参数映射</strong> - Path/Query/Header 参数灵活配置</li>
<li>✅ <strong>请求转换</strong> - 字段映射或 Go 模板</li>
<li>✅ <strong>响应转换</strong> - 提取指定字段、包装响应</li>
<li>✅ <strong>自动发现</strong> - 从 OpenAPI/Swagger 文档自动发现接口</li>
</ul>
<h3 id="快速开始" tabindex="-1"><a class="header-anchor" href="#快速开始"><span>快速开始</span></a></h3>
<h4 id="_1-最简配置" tabindex="-1"><a class="header-anchor" href="#_1-最简配置"><span>1. 最简配置</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">my_game_server</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">type</span><span class="token punctuation">:</span> openapi</span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">base_url</span><span class="token punctuation">:</span> <span class="token string">"http://my-game-server:8081"</span></span>
<span class="line">      <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">type</span><span class="token punctuation">:</span> bearer</span>
<span class="line">        <span class="token key atrule">token</span><span class="token punctuation">:</span> <span class="token string">"my-secret-token"</span></span>
<span class="line">      <span class="token key atrule">methods</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> get_player</span>
<span class="line">          <span class="token key atrule">path</span><span class="token punctuation">:</span> <span class="token string">"/api/player/get"</span></span>
<span class="line">          <span class="token key atrule">method</span><span class="token punctuation">:</span> POST</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_2-调用示例" tabindex="-1"><a class="header-anchor" href="#_2-调用示例"><span>2. 调用示例</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># HTTP API 调用</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://croupier-server:8080/api/v1/platform/call <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{</span>
<span class="line">    "platform": "my_game_server",</span>
<span class="line">    "method": "get_player",</span>
<span class="line">    "request": {"player_id": "12345"}</span>
<span class="line">  }'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="配置参考" tabindex="-1"><a class="header-anchor" href="#配置参考"><span>配置参考</span></a></h3>
<h4 id="完整配置结构" tabindex="-1"><a class="header-anchor" href="#完整配置结构"><span>完整配置结构</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">&lt;platform_name></span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">type</span><span class="token punctuation">:</span> openapi</span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token comment"># 基础配置</span></span>
<span class="line">      <span class="token key atrule">base_url</span><span class="token punctuation">:</span> <span class="token string">"http://api.example.com"</span>          <span class="token comment"># 必填：API 基础 URL</span></span>
<span class="line">      <span class="token key atrule">timeout</span><span class="token punctuation">:</span> 30s                                 <span class="token comment"># 可选：请求超时</span></span>
<span class="line">      <span class="token key atrule">retry_count</span><span class="token punctuation">:</span> <span class="token number">3</span>                               <span class="token comment"># 可选：重试次数</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># OpenAPI 规范（可选，用于自动发现）</span></span>
<span class="line">      <span class="token key atrule">openapi_spec</span><span class="token punctuation">:</span> <span class="token string">"http://api.example.com/openapi.json"</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># 认证配置</span></span>
<span class="line">      <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">type</span><span class="token punctuation">:</span> bearer                               <span class="token comment"># none, bearer, basic, api_key, custom</span></span>
<span class="line">        <span class="token key atrule">token</span><span class="token punctuation">:</span> <span class="token string">"${API_TOKEN}"</span>                      <span class="token comment"># bearer 类型</span></span>
<span class="line">        <span class="token comment"># username: "user"                         # basic 类型</span></span>
<span class="line">        <span class="token comment"># password: "${PASSWORD}"</span></span>
<span class="line">        <span class="token comment"># api_key:                                 # api_key 类型</span></span>
<span class="line">        <span class="token comment">#   name: "X-API-Key"</span></span>
<span class="line">        <span class="token comment">#   value: "${KEY}"</span></span>
<span class="line">        <span class="token comment">#   in: "header"                           # header 或 query</span></span>
<span class="line">        <span class="token comment"># custom_headers:                          # custom 类型</span></span>
<span class="line">        <span class="token comment">#   X-Custom-Header: "value"</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># 默认请求头</span></span>
<span class="line">      <span class="token key atrule">headers</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">User-Agent</span><span class="token punctuation">:</span> <span class="token string">"MyApp/1.0"</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># 响应转换（全局）</span></span>
<span class="line">      <span class="token key atrule">transform</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">success_field</span><span class="token punctuation">:</span> <span class="token string">"code"</span>                      <span class="token comment"># 成功字段</span></span>
<span class="line">        <span class="token key atrule">success_value</span><span class="token punctuation">:</span> <span class="token number">0</span>                           <span class="token comment"># 成功值</span></span>
<span class="line">        <span class="token key atrule">data_field</span><span class="token punctuation">:</span> <span class="token string">"data"</span>                         <span class="token comment"># 数据字段</span></span>
<span class="line">        <span class="token key atrule">error_field</span><span class="token punctuation">:</span> <span class="token string">"message"</span>                     <span class="token comment"># 错误字段</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># 方法定义</span></span>
<span class="line">      <span class="token key atrule">methods</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> &lt;method_name<span class="token punctuation">></span>                      <span class="token comment"># 方法名</span></span>
<span class="line">          <span class="token key atrule">path</span><span class="token punctuation">:</span> <span class="token string">"/api/endpoint"</span>                    <span class="token comment"># API 路径</span></span>
<span class="line">          <span class="token key atrule">method</span><span class="token punctuation">:</span> POST                             <span class="token comment"># HTTP 方法</span></span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"Method description"</span>        <span class="token comment"># 描述</span></span>
<span class="line"></span>
<span class="line">          <span class="token comment"># 参数映射</span></span>
<span class="line">          <span class="token key atrule">parameters</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> param_name                     <span class="token comment"># API 参数名</span></span>
<span class="line">              <span class="token key atrule">in</span><span class="token punctuation">:</span> path                             <span class="token comment"># 位置: path, query, header</span></span>
<span class="line">              <span class="token key atrule">from</span><span class="token punctuation">:</span> input_field                    <span class="token comment"># 输入字段名（默认同 name）</span></span>
<span class="line">              <span class="token key atrule">required</span><span class="token punctuation">:</span> <span class="token boolean important">true</span>                       <span class="token comment"># 是否必填</span></span>
<span class="line">              <span class="token key atrule">default</span><span class="token punctuation">:</span> <span class="token string">"default_value"</span>             <span class="token comment"># 默认值</span></span>
<span class="line"></span>
<span class="line">          <span class="token comment"># 请求体配置</span></span>
<span class="line">          <span class="token key atrule">request_body</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">type</span><span class="token punctuation">:</span> json                             <span class="token comment"># 类型: json, form, text</span></span>
<span class="line">            <span class="token comment"># 方式1：字段映射</span></span>
<span class="line">            <span class="token key atrule">fields</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">dest_field</span><span class="token punctuation">:</span> src_field</span>
<span class="line">            <span class="token comment"># 方式2：Go 模板</span></span>
<span class="line">            <span class="token key atrule">template</span><span class="token punctuation">:</span> <span class="token string">'{"field": "{{ .input }}"}'</span></span>
<span class="line"></span>
<span class="line">          <span class="token comment"># 响应转换</span></span>
<span class="line">          <span class="token key atrule">response_mapping</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">extract_path</span><span class="token punctuation">:</span> <span class="token string">"data.items"</span>             <span class="token comment"># JSON 路径提取</span></span>
<span class="line">            <span class="token key atrule">wrap</span><span class="token punctuation">:</span> <span class="token boolean important">true</span>                             <span class="token comment"># 包装响应</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 速率限制</span></span>
<span class="line">    <span class="token key atrule">rate_limit</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">requests_per_minute</span><span class="token punctuation">:</span> <span class="token number">60</span></span>
<span class="line">      <span class="token key atrule">burst_size</span><span class="token punctuation">:</span> <span class="token number">10</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="认证方式详解" tabindex="-1"><a class="header-anchor" href="#认证方式详解"><span>认证方式详解</span></a></h4>
<table>
<thead>
<tr>
<th>类型</th>
<th>说明</th>
<th>配置</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>none</code></td>
<td>无认证</td>
<td><code v-pre>type: none</code></td>
</tr>
<tr>
<td><code v-pre>bearer</code></td>
<td>Bearer Token</td>
<td><code v-pre>type: bearer; token: &quot;xxx&quot;</code></td>
</tr>
<tr>
<td><code v-pre>basic</code></td>
<td>HTTP Basic Auth</td>
<td><code v-pre>type: basic; username: &quot;xxx&quot;; password: &quot;xxx&quot;</code></td>
</tr>
<tr>
<td><code v-pre>api_key</code></td>
<td>API Key</td>
<td><code v-pre>type: api_key; api_key: {name: &quot;X-Key&quot;, value: &quot;xxx&quot;, in: &quot;header&quot;}</code></td>
</tr>
<tr>
<td><code v-pre>custom</code></td>
<td>自定义 Header</td>
<td><code v-pre>type: custom; custom_headers: {X-Token: &quot;xxx&quot;}</code></td>
</tr>
</tbody>
</table>
<h4 id="参数位置详解" tabindex="-1"><a class="header-anchor" href="#参数位置详解"><span>参数位置详解</span></a></h4>
<table>
<thead>
<tr>
<th>位置</th>
<th>说明</th>
<th>示例</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>path</code></td>
<td>URL 路径参数</td>
<td><code v-pre>/api/users/{id}</code> 中的 <code v-pre>id</code></td>
</tr>
<tr>
<td><code v-pre>query</code></td>
<td>URL 查询参数</td>
<td><code v-pre>/api/users?page=1</code> 中的 <code v-pre>page</code></td>
</tr>
<tr>
<td><code v-pre>header</code></td>
<td>请求头</td>
<td><code v-pre>X-Request-ID: xxx</code></td>
</tr>
</tbody>
</table>
<h4 id="请求体配置详解" tabindex="-1"><a class="header-anchor" href="#请求体配置详解"><span>请求体配置详解</span></a></h4>
<p><strong>字段映射方式：</strong></p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">request_body</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">type</span><span class="token punctuation">:</span> json</span>
<span class="line">  <span class="token key atrule">fields</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">apiField</span><span class="token punctuation">:</span> inputField    <span class="token comment"># API 字段名: 输入字段名</span></span>
<span class="line">    <span class="token key atrule">user_id</span><span class="token punctuation">:</span> player_id</span>
<span class="line">    <span class="token key atrule">server</span><span class="token punctuation">:</span> server_id</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>Go 模板方式：</strong></p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">request_body</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">type</span><span class="token punctuation">:</span> json</span>
<span class="line">  <span class="token key atrule">template</span><span class="token punctuation">:</span> <span class="token string">'{"user_id": "{{ .player_id }}", "action": "{{ .action }}"}'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="响应转换详解" tabindex="-1"><a class="header-anchor" href="#响应转换详解"><span>响应转换详解</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">response_mapping</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token comment"># 提取嵌套字段</span></span>
<span class="line">  <span class="token key atrule">extract_path</span><span class="token punctuation">:</span> <span class="token string">"data.items"</span>     <span class="token comment"># 从 {"data": {"items": [...]}} 提取 items</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 包装响应</span></span>
<span class="line">  <span class="token key atrule">wrap</span><span class="token punctuation">:</span> <span class="token boolean important">true</span>                     <span class="token comment"># 将响应包装为 {"data": &lt;原始响应>}</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 成功判断（配合全局 transform）</span></span>
<span class="line">  <span class="token key atrule">success_field</span><span class="token punctuation">:</span> <span class="token string">"code"</span>          <span class="token comment"># 检查 code 字段</span></span>
<span class="line">  <span class="token key atrule">success_value</span><span class="token punctuation">:</span> <span class="token number">0</span>               <span class="token comment"># code == 0 表示成功</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="实际案例" tabindex="-1"><a class="header-anchor" href="#实际案例"><span>实际案例</span></a></h3>
<h4 id="案例-1-游戏服玩家查询" tabindex="-1"><a class="header-anchor" href="#案例-1-游戏服玩家查询"><span>案例 1：游戏服玩家查询</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">game_server</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">type</span><span class="token punctuation">:</span> openapi</span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">base_url</span><span class="token punctuation">:</span> <span class="token string">"http://game-server:8081"</span></span>
<span class="line">      <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">type</span><span class="token punctuation">:</span> bearer</span>
<span class="line">        <span class="token key atrule">token</span><span class="token punctuation">:</span> <span class="token string">"${GAME_ADMIN_TOKEN}"</span></span>
<span class="line">      <span class="token key atrule">methods</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> get_player</span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"获取玩家信息"</span></span>
<span class="line">          <span class="token key atrule">path</span><span class="token punctuation">:</span> <span class="token string">"/api/player/info"</span></span>
<span class="line">          <span class="token key atrule">method</span><span class="token punctuation">:</span> POST</span>
<span class="line">          <span class="token key atrule">request_body</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">type</span><span class="token punctuation">:</span> json</span>
<span class="line">            <span class="token key atrule">fields</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">player_id</span><span class="token punctuation">:</span> player_id</span>
<span class="line">              <span class="token key atrule">server_id</span><span class="token punctuation">:</span> server_id</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>调用：</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"platform"</span><span class="token operator">:</span> <span class="token string">"game_server"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"method"</span><span class="token operator">:</span> <span class="token string">"get_player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"12345"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"server_id"</span><span class="token operator">:</span> <span class="token string">"s1"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="案例-2-游戏服封禁玩家" tabindex="-1"><a class="header-anchor" href="#案例-2-游戏服封禁玩家"><span>案例 2：游戏服封禁玩家</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">game_server</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">methods</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> ban_player</span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"封禁玩家"</span></span>
<span class="line">          <span class="token key atrule">path</span><span class="token punctuation">:</span> <span class="token string">"/api/player/ban"</span></span>
<span class="line">          <span class="token key atrule">method</span><span class="token punctuation">:</span> POST</span>
<span class="line">          <span class="token key atrule">request_body</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">type</span><span class="token punctuation">:</span> json</span>
<span class="line">            <span class="token key atrule">template</span><span class="token punctuation">:</span> <span class="token string">'{"player_id": "{{ .player_id }}", "reason": "{{ .reason }}", "duration": {{ .duration }}}'</span></span>
<span class="line">          <span class="token key atrule">response_mapping</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">extract_path</span><span class="token punctuation">:</span> <span class="token string">"data"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>调用：</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"platform"</span><span class="token operator">:</span> <span class="token string">"game_server"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"method"</span><span class="token operator">:</span> <span class="token string">"ban_player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"12345"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token string">"作弊"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token number">86400</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="案例-3-带路径参数的-api" tabindex="-1"><a class="header-anchor" href="#案例-3-带路径参数的-api"><span>案例 3：带路径参数的 API</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">game_server</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">methods</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> get_guild</span>
<span class="line">          <span class="token key atrule">path</span><span class="token punctuation">:</span> <span class="token string">"/api/guild/{guild_id}/members"</span></span>
<span class="line">          <span class="token key atrule">method</span><span class="token punctuation">:</span> GET</span>
<span class="line">          <span class="token key atrule">parameters</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> guild_id</span>
<span class="line">              <span class="token key atrule">in</span><span class="token punctuation">:</span> path</span>
<span class="line">              <span class="token key atrule">from</span><span class="token punctuation">:</span> guild_id</span>
<span class="line">              <span class="token key atrule">required</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">            <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> page</span>
<span class="line">              <span class="token key atrule">in</span><span class="token punctuation">:</span> query</span>
<span class="line">              <span class="token key atrule">from</span><span class="token punctuation">:</span> page</span>
<span class="line">              <span class="token key atrule">default</span><span class="token punctuation">:</span> <span class="token string">"1"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>调用：</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"platform"</span><span class="token operator">:</span> <span class="token string">"game_server"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"method"</span><span class="token operator">:</span> <span class="token string">"get_guild"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"guild_id"</span><span class="token operator">:</span> <span class="token string">"guild_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"page"</span><span class="token operator">:</span> <span class="token string">"2"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="案例-4-自动发现-openapi-接口" tabindex="-1"><a class="header-anchor" href="#案例-4-自动发现-openapi-接口"><span>案例 4：自动发现 OpenAPI 接口</span></a></h4>
<p>如果服务提供 OpenAPI/Swagger 文档，可以自动发现接口：</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">platforms</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">game_server</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">type</span><span class="token punctuation">:</span> openapi</span>
<span class="line">    <span class="token key atrule">config</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">base_url</span><span class="token punctuation">:</span> <span class="token string">"http://game-server:8081"</span></span>
<span class="line">      <span class="token key atrule">openapi_spec</span><span class="token punctuation">:</span> <span class="token string">"http://game-server:8081/openapi.json"</span></span>
<span class="line">      <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">type</span><span class="token punctuation">:</span> api_key</span>
<span class="line">        <span class="token key atrule">api_key</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">name</span><span class="token punctuation">:</span> <span class="token string">"X-API-Key"</span></span>
<span class="line">          <span class="token key atrule">value</span><span class="token punctuation">:</span> <span class="token string">"${API_KEY}"</span></span>
<span class="line">          <span class="token key atrule">in</span><span class="token punctuation">:</span> <span class="token string">"header"</span></span>
<span class="line">      <span class="token comment"># methods 留空，自动从 openapi_spec 发现</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="目录结构" tabindex="-1"><a class="header-anchor" href="#目录结构"><span>目录结构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">internal/platform/openapi/</span>
<span class="line">└── provider.go       # OpenAPI Provider 核心实现</span>
<span class="line"></span>
<span class="line">internal/platform/ratelimit/</span>
<span class="line">└── tokenbucket.go    # 令牌桶速率限制器</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="限制与注意事项" tabindex="-1"><a class="header-anchor" href="#限制与注意事项"><span>限制与注意事项</span></a></h3>
<ol>
<li><strong>安全性</strong>：敏感信息（如 Token）应使用环境变量 <code v-pre>${VAR_NAME}</code></li>
<li><strong>超时</strong>：默认 30 秒，可根据需要调整</li>
<li><strong>重试</strong>：仅对 5xx 错误自动重试</li>
<li><strong>速率限制</strong>：建议根据服务端限流配置合适的 <code v-pre>requests_per_minute</code></li>
</ol>
</div></template>


