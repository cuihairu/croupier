<template><div><h1 id="croupier-c-sdk-深度分析" tabindex="-1"><a class="header-anchor" href="#croupier-c-sdk-深度分析"><span>Croupier C++ SDK 深度分析</span></a></h1>
<p><strong>创建日期</strong>: 2025-11-13<br>
<strong>SDK版本</strong>: 1.0.0<br>
<strong>C++ 标准</strong>: C++17</p>
<hr>
<h2 id="📌-概述" tabindex="-1"><a class="header-anchor" href="#📌-概述"><span>📌 概述</span></a></h2>
<p>Croupier C++ SDK 是一个高性能的游戏后端虚拟对象注册系统，提供：</p>
<ul>
<li><strong>虚拟对象管理</strong> - 四层架构 (Function → Entity → Resource → Component)</li>
<li><strong>与后台Agent交互</strong> - 通过 gRPC LocalControlService 注册和通信</li>
<li><strong>多游戏环境隔离</strong> - 通过 game_id + env 实现租户隔离</li>
<li><strong>权限和控制</strong> - RBAC 和描述符验证机制</li>
</ul>
<hr>
<h2 id="_1️⃣-spi-service-provider-interface-实现方式" tabindex="-1"><a class="header-anchor" href="#_1️⃣-spi-service-provider-interface-实现方式"><span>1️⃣ SPI (Service Provider Interface) 实现方式</span></a></h2>
<h3 id="_1-1-核心-spi-设计" tabindex="-1"><a class="header-anchor" href="#_1-1-核心-spi-设计"><span>1.1 核心 SPI 设计</span></a></h3>
<p><strong>Handler 回调模式</strong> (Service Provider Interface):</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 类型定义：函数处理器</span></span>
<span class="line"><span class="token keyword">using</span> FunctionHandler <span class="token operator">=</span> std<span class="token double-colon punctuation">::</span>function<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span><span class="token function">string</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> context<span class="token punctuation">,</span>  <span class="token comment">// 请求上下文</span></span>
<span class="line">    <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload   <span class="token comment">// JSON 序列化的参数</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token operator">></span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h</code> (第 18 行)</p>
<h3 id="_1-2-spi-注册机制" tabindex="-1"><a class="header-anchor" href="#_1-2-spi-注册机制"><span>1.2 SPI 注册机制</span></a></h3>
<h4 id="方式1-基础函数注册-向后兼容" tabindex="-1"><a class="header-anchor" href="#方式1-基础函数注册-向后兼容"><span><strong>方式1：基础函数注册 (向后兼容)</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">bool</span> <span class="token function">RegisterFunction</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token keyword">const</span> FunctionDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">,</span></span>
<span class="line">    FunctionHandler handler</span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li><strong>说明</strong>: 注册单个原子操作函数</li>
<li><strong>参数</strong>: 函数描述符 + 处理器回调</li>
<li><strong>用途</strong>: 简单的函数导出</li>
</ul>
<h4 id="方式2-虚拟对象注册-推荐" tabindex="-1"><a class="header-anchor" href="#方式2-虚拟对象注册-推荐"><span><strong>方式2：虚拟对象注册 (推荐)</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">bool</span> <span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span><span class="token operator">&amp;</span> handlers</span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li><strong>说明</strong>: 将相关函数组织为业务对象</li>
<li><strong>参数</strong>: 对象描述符 + 操作函数映射</li>
<li><strong>优势</strong>: 关系明确，易于管理</li>
</ul>
<h4 id="方式3-组件级注册-生产推荐" tabindex="-1"><a class="header-anchor" href="#方式3-组件级注册-生产推荐"><span><strong>方式3：组件级注册 (生产推荐)</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">bool</span> <span class="token function">RegisterComponent</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span> comp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> config_file<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li><strong>说明</strong>: 整个子系统一次性注册</li>
<li><strong>参数</strong>: 组件描述符或配置文件路径</li>
<li><strong>特点</strong>: 支持声明式配置驱动</li>
</ul>
<h3 id="_1-3-handler-签名规范" tabindex="-1"><a class="header-anchor" href="#_1-3-handler-签名规范"><span>1.3 Handler 签名规范</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 实现示例</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">WalletTransferHandler</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> context<span class="token punctuation">,</span>  <span class="token comment">// 调用上下文</span></span>
<span class="line">    <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload   <span class="token comment">// JSON: {"from_player_id":"p1", "to_player_id":"p2", "amount":"100"}</span></span>
<span class="line"><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 1. 解析 payload</span></span>
<span class="line">    <span class="token keyword">auto</span> data <span class="token operator">=</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ParseJSON</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string from_player <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"from_player_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string to_player <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"to_player_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string amount <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"amount"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 2. 执行业务逻辑</span></span>
<span class="line">    TransferResult result <span class="token operator">=</span> <span class="token class-name">WalletService</span><span class="token double-colon punctuation">::</span><span class="token function">Transfer</span><span class="token punctuation">(</span>from_player<span class="token punctuation">,</span> to_player<span class="token punctuation">,</span> amount<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 3. 返回 JSON 响应</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> response<span class="token punctuation">;</span></span>
<span class="line">    response<span class="token punctuation">[</span><span class="token string">"transfer_id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> result<span class="token punctuation">.</span>transfer_id<span class="token punctuation">;</span></span>
<span class="line">    response<span class="token punctuation">[</span><span class="token string">"status"</span><span class="token punctuation">]</span> <span class="token operator">=</span> result<span class="token punctuation">.</span>status<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ToJSON</span><span class="token punctuation">(</span>response<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>调用时机</strong>：</p>
<ul>
<li>后台 Agent 通过 gRPC 调用 → 转发到本地 Server → 查找对应 Handler → 同步执行</li>
<li>支持幂等性 (idempotency_key)</li>
</ul>
<p><strong>文件位置</strong>:</p>
<ul>
<li>实现: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp</code> (第 102-407 行)</li>
<li>示例: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/examples/virtual_object_demo.cpp</code> (第 8-93 行)</li>
</ul>
<hr>
<h2 id="_2️⃣-game-id-和-env-相关代码" tabindex="-1"><a class="header-anchor" href="#_2️⃣-game-id-和-env-相关代码"><span>2️⃣ game_id 和 env 相关代码</span></a></h2>
<h3 id="_2-1-配置结构体" tabindex="-1"><a class="header-anchor" href="#_2-1-配置结构体"><span>2.1 配置结构体</span></a></h3>
<p><strong>ClientConfig</strong> (客户端配置):</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">struct</span> <span class="token class-name">ClientConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// ========== Game Environment Configuration ==========</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string game_id <span class="token operator">=</span> <span class="token string">""</span><span class="token punctuation">;</span>              <span class="token comment">// 🎮 必需：游戏标识符</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string env <span class="token operator">=</span> <span class="token string">"development"</span><span class="token punctuation">;</span>       <span class="token comment">// 🔧 必需：环境隔离</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 其他配置项</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string agent_addr <span class="token operator">=</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string local_listen <span class="token operator">=</span> <span class="token string">"127.0.0.1:0"</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string service_id <span class="token operator">=</span> <span class="token string">"cpp-service"</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// ... 认证、TLS 等</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>InvokerConfig</strong> (调用者配置):</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">struct</span> <span class="token class-name">InvokerConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string address<span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string game_id<span class="token punctuation">;</span>                   <span class="token comment">// 🎮 必需</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string env <span class="token operator">=</span> <span class="token string">"development"</span><span class="token punctuation">;</span>       <span class="token comment">// 🔧 必需</span></span>
<span class="line">    <span class="token comment">// ... 其他配置</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h</code> (第 57-106 行)</p>
<h3 id="_2-2-game-id-env-的验证和使用" tabindex="-1"><a class="header-anchor" href="#_2-2-game-id-env-的验证和使用"><span>2.2 game_id/env 的验证和使用</span></a></h3>
<p><strong>初始化时验证</strong>:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">explicit</span> <span class="token function">Impl</span><span class="token punctuation">(</span><span class="token keyword">const</span> ClientConfig<span class="token operator">&amp;</span> config<span class="token punctuation">)</span> <span class="token operator">:</span> <span class="token function">config_</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// Validate required configuration</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>config_<span class="token punctuation">.</span>game_id<span class="token punctuation">.</span><span class="token function">empty</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Warning: game_id is required for proper backend separation"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// Validate environment</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>config_<span class="token punctuation">.</span>env <span class="token operator">!=</span> <span class="token string">"development"</span> <span class="token operator">&amp;&amp;</span> config_<span class="token punctuation">.</span>env <span class="token operator">!=</span> <span class="token string">"staging"</span> <span class="token operator">&amp;&amp;</span> config_<span class="token punctuation">.</span>env <span class="token operator">!=</span> <span class="token string">"production"</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Warning: Unknown environment '"</span> <span class="token operator">&lt;&lt;</span> config_<span class="token punctuation">.</span>env</span>
<span class="line">                  <span class="token operator">&lt;&lt;</span> <span class="token string">"'. Valid values: development, staging, production"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    </span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Initialized CroupierClient for game '"</span> <span class="token operator">&lt;&lt;</span> config_<span class="token punctuation">.</span>game_id</span>
<span class="line">              <span class="token operator">&lt;&lt;</span> <span class="token string">"' in '"</span> <span class="token operator">&lt;&lt;</span> config_<span class="token punctuation">.</span>env <span class="token operator">&lt;&lt;</span> <span class="token string">"' environment"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp</code> (第 117-131 行)</p>
<h3 id="_2-3-后台交互中的传递" tabindex="-1"><a class="header-anchor" href="#_2-3-后台交互中的传递"><span>2.3 后台交互中的传递</span></a></h3>
<p><strong>在 Proto 中的定义</strong> (<code v-pre>control.proto</code>):</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">FunctionDescriptor</span> functions <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> rpc_addr <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>           <span class="token comment">// ← 关键字段</span></span>
<span class="line">    <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>               <span class="token comment">// ← 关键字段</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/proto/croupier/control/v1/control.proto</code> (第 17-24 行)</p>
<h3 id="_2-4-环境隔离策略" tabindex="-1"><a class="header-anchor" href="#_2-4-环境隔离策略"><span>2.4 环境隔离策略</span></a></h3>
<table>
<thead>
<tr>
<th>环境</th>
<th>用途</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>development</strong></td>
<td>本地开发</td>
<td>允许不安全连接 (insecure=true)</td>
</tr>
<tr>
<td><strong>staging</strong></td>
<td>预发布测试</td>
<td>需要 TLS 但可能使用自签名证书</td>
</tr>
<tr>
<td><strong>production</strong></td>
<td>生产环境</td>
<td>强制 TLS + 证书验证 + 认证 Token</td>
</tr>
</tbody>
</table>
<p><strong>租户隔离机制</strong>:</p>
<ul>
<li>Backend 按 (game_id, env) 元组索引所有资源</li>
<li>不同游戏的函数注册表完全隔离</li>
<li>调用时必须传递 game_id，后台验证租户权限</li>
</ul>
<p><strong>示例配置</strong>:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 游戏A开发环境</span></span>
<span class="line">ClientConfig config_a<span class="token punctuation">;</span></span>
<span class="line">config_a<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"game-a"</span><span class="token punctuation">;</span></span>
<span class="line">config_a<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"development"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 游戏B生产环境</span></span>
<span class="line">ClientConfig config_b<span class="token punctuation">;</span></span>
<span class="line">config_b<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"game-b"</span><span class="token punctuation">;</span></span>
<span class="line">config_b<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"production"</span><span class="token punctuation">;</span></span>
<span class="line">config_b<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">config_b<span class="token punctuation">.</span>cert_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/client.crt"</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h</code> (第 58-83 行)</p>
<hr>
<h2 id="_3️⃣-与后台-agent-的注册交互机制" tabindex="-1"><a class="header-anchor" href="#_3️⃣-与后台-agent-的注册交互机制"><span>3️⃣ 与后台 Agent 的注册交互机制</span></a></h2>
<h3 id="_3-1-整体交互流程" tabindex="-1"><a class="header-anchor" href="#_3-1-整体交互流程"><span>3.1 整体交互流程</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────────────────────┐</span>
<span class="line">│  游戏服务器 (C++ SDK)                                      │</span>
<span class="line">└──────────────────────┬──────────────────────────────────┘</span>
<span class="line">                       │</span>
<span class="line">                       │ 1. LocalControlService::RegisterLocal()</span>
<span class="line">                       │    (service_id, rpc_addr, functions)</span>
<span class="line">                       ↓</span>
<span class="line">┌──────────────────────────────────────────────────────────┐</span>
<span class="line">│  Agent (19090 LocalControlService)                       │</span>
<span class="line">├──────────────────────────────────────────────────────────┤</span>
<span class="line">│  • 接收函数注册                                             │</span>
<span class="line">│  • 返回 session_id                                         │</span>
<span class="line">│  • 建立反向 Tunnel                                        │</span>
<span class="line">└──────────────────────┬──────────────────────────────────┘</span>
<span class="line">                       │</span>
<span class="line">                       │ 2. 定期 Heartbeat 保持活跃</span>
<span class="line">                       │    (service_id, session_id)</span>
<span class="line">                       │</span>
<span class="line">                       │ 3. Agent 负载均衡向后台 Server 转发</span>
<span class="line">                       │    ControlService::Register()</span>
<span class="line">                       ↓</span>
<span class="line">┌──────────────────────────────────────────────────────────┐</span>
<span class="line">│  Server (8443 ControlService)                           │</span>
<span class="line">├──────────────────────────────────────────────────────────┤</span>
<span class="line">│  • game_id + env 隔离维护                                  │</span>
<span class="line">│  • RBAC 权限检查                                           │</span>
<span class="line">│  • 函数注册表管理                                           │</span>
<span class="line">└──────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-2-注册流程详解" tabindex="-1"><a class="header-anchor" href="#_3-2-注册流程详解"><span>3.2 注册流程详解</span></a></h3>
<h4 id="步骤1-连接到-agent" tabindex="-1"><a class="header-anchor" href="#步骤1-连接到-agent"><span><strong>步骤1：连接到 Agent</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">bool</span> <span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>connected_<span class="token punctuation">)</span> <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Connecting to agent at: "</span> <span class="token operator">&lt;&lt;</span> config_<span class="token punctuation">.</span>agent_addr <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// NOTE: 当前为模拟实现，真实 gRPC 连接待实现</span></span>
<span class="line">    <span class="token comment">// 预期实现：</span></span>
<span class="line">    <span class="token comment">// 1. 建立 gRPC stub 到 LocalControlService</span></span>
<span class="line">    <span class="token comment">// 2. 调用 RegisterLocal RPC</span></span>
<span class="line">    <span class="token comment">// 3. 接收 session_id</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Start local gRPC server</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span><span class="token function">StartLocalServer</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Failed to start local server"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// NOTE: Agent 注册功能待实现</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Registered "</span> <span class="token operator">&lt;&lt;</span> handlers_<span class="token punctuation">.</span><span class="token function">size</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">&lt;&lt;</span> <span class="token string">" functions with agent"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    connected_ <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp</code> (第 317-337 行)</p>
<h4 id="步骤2-本地服务启动" tabindex="-1"><a class="header-anchor" href="#步骤2-本地服务启动"><span><strong>步骤2：本地服务启动</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">bool</span> <span class="token function">StartLocalServer</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// Parse listen address</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string host<span class="token punctuation">,</span> port_str<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">auto</span> colon_pos <span class="token operator">=</span> config_<span class="token punctuation">.</span>local_listen<span class="token punctuation">.</span><span class="token function">find</span><span class="token punctuation">(</span><span class="token char">':'</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>colon_pos <span class="token operator">!=</span> std<span class="token double-colon punctuation">::</span>string<span class="token double-colon punctuation">::</span>npos<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        host <span class="token operator">=</span> config_<span class="token punctuation">.</span>local_listen<span class="token punctuation">.</span><span class="token function">substr</span><span class="token punctuation">(</span><span class="token number">0</span><span class="token punctuation">,</span> colon_pos<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        port_str <span class="token operator">=</span> config_<span class="token punctuation">.</span>local_listen<span class="token punctuation">.</span><span class="token function">substr</span><span class="token punctuation">(</span>colon_pos <span class="token operator">+</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">else</span> <span class="token punctuation">{</span></span>
<span class="line">        host <span class="token operator">=</span> config_<span class="token punctuation">.</span>local_listen<span class="token punctuation">;</span></span>
<span class="line">        port_str <span class="token operator">=</span> <span class="token string">"0"</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// Simulate port allocation</span></span>
<span class="line">    <span class="token keyword">int</span> port <span class="token operator">=</span> std<span class="token double-colon punctuation">::</span><span class="token function">stoi</span><span class="token punctuation">(</span>port_str<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>port <span class="token operator">==</span> <span class="token number">0</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// Allocate random port</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>random_device rd<span class="token punctuation">;</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>mt19937 <span class="token function">gen</span><span class="token punctuation">(</span><span class="token function">rd</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>uniform_int_distribution<span class="token operator">&lt;</span><span class="token operator">></span> <span class="token function">dis</span><span class="token punctuation">(</span><span class="token number">20000</span><span class="token punctuation">,</span> <span class="token number">30000</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        port <span class="token operator">=</span> <span class="token function">dis</span><span class="token punctuation">(</span>gen<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    </span>
<span class="line">    local_address_ <span class="token operator">=</span> host <span class="token operator">+</span> <span class="token string">":"</span> <span class="token operator">+</span> std<span class="token double-colon punctuation">::</span><span class="token function">to_string</span><span class="token punctuation">(</span>port<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Local server listening on: "</span> <span class="token operator">&lt;&lt;</span> local_address_ <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp</code> (第 377-406 行)</p>
<h4 id="步骤3-grpc-proto-消息定义" tabindex="-1"><a class="header-anchor" href="#步骤3-grpc-proto-消息定义"><span><strong>步骤3：gRPC Proto 消息定义</strong></span></a></h4>
<p><strong>LocalControlService</strong> (agent/local/v1/local.proto):</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// 客户端 → Agent 注册请求</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterLocalRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> service_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>                        <span class="token comment">// e.g. "game-server-1"</span></span>
<span class="line">    <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>                           <span class="token comment">// e.g. "1.0.0"</span></span>
<span class="line">    <span class="token builtin">string</span> rpc_addr <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>                          <span class="token comment">// e.g. "127.0.0.1:20001"</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">LocalFunctionDescriptor</span> functions <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>  <span class="token comment">// 函数列表</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Agent 返回 session_id</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterLocalResponse</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> session_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// 后续用于识别连接</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 定期心跳</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">HeartbeatRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> service_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> session_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 获取本地函数列表（用于调试）</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ListLocalRequest</span> <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">ListLocalResponse</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">LocalFunction</span> functions <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// 已注册函数</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/proto/croupier/agent/local/v1/local.proto</code></p>
<h3 id="_3-3-注册消息结构" tabindex="-1"><a class="header-anchor" href="#_3-3-注册消息结构"><span>3.3 注册消息结构</span></a></h3>
<p><strong>完整注册流程消息</strong>:</p>
<ol>
<li><strong>初始化阶段</strong>：</li>
</ol>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">C++ SDK                          Agent (19090)</span>
<span class="line">   |                              |</span>
<span class="line">   | RegisterLocal(                |</span>
<span class="line">   |   service_id="game-1",       |</span>
<span class="line">   |   version="1.0.0",           |</span>
<span class="line">   |   rpc_addr="127.0.0.1:20001",|</span>
<span class="line">   |   functions=[                |</span>
<span class="line">   |     {id:"wallet.transfer"},  |</span>
<span class="line">   |     {id:"wallet.get"}        |</span>
<span class="line">   |   ]                          |</span>
<span class="line">   | )                            |</span>
<span class="line">   |----------------------------->|</span>
<span class="line">   |                              | 存储注册信息</span>
<span class="line">   |        RegisterLocalResponse  | 转发到 Server</span>
<span class="line">   |        {session_id:"sess_abc"}|</span>
<span class="line">   |&lt;-----------------------------|</span>
<span class="line">   |                              |</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>心跳阶段</strong>（定期，如 60 秒一次）：</li>
</ol>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">C++ SDK                          Agent (19090)</span>
<span class="line">   |                              |</span>
<span class="line">   | Heartbeat(                   |</span>
<span class="line">   |   service_id="game-1",       |</span>
<span class="line">   |   session_id="sess_abc"      |</span>
<span class="line">   | )                            |</span>
<span class="line">   |----------------------------->|</span>
<span class="line">   |        HeartbeatResponse     |</span>
<span class="line">   |&lt;-----------------------------|</span>
<span class="line">   |                              |</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li><strong>调用阶段</strong>（来自后台）：</li>
</ol>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Server                           Agent                    C++ SDK</span>
<span class="line">  |                               |                         |</span>
<span class="line">  | FunctionService::Invoke()     |                         |</span>
<span class="line">  | (wallet.transfer, game="x")   |                         |</span>
<span class="line">  |------------------------------>|                         |</span>
<span class="line">  |                               | 根据 game_id             |</span>
<span class="line">  |                               | 查找 service_id="game-1" |</span>
<span class="line">  |                               |                         |</span>
<span class="line">  |                               | 转发 RPC 到本地服务      |</span>
<span class="line">  |                               | (或反向隧道)            |</span>
<span class="line">  |                               |----------------------->|</span>
<span class="line">  |                               |                         | 执行 Handler</span>
<span class="line">  |                               |                         | 返回结果</span>
<span class="line">  |                               |&lt;------------------------|</span>
<span class="line">  |                    结果         |                         |</span>
<span class="line">  |&lt;------------------------------|                         |</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-4-实现关键要点" tabindex="-1"><a class="header-anchor" href="#_3-4-实现关键要点"><span>3.4 实现关键要点</span></a></h3>
<h4 id="重点-建立本地-grpc-服务器" tabindex="-1"><a class="header-anchor" href="#重点-建立本地-grpc-服务器"><span><strong>重点：建立本地 gRPC 服务器</strong></span></a></h4>
<p>C++ SDK 需要实现一个本地 gRPC 服务器来接收来自 Agent 的函数调用。这涉及：</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 伪代码：实现思路</span></span>
<span class="line"><span class="token keyword">class</span> <span class="token class-name">LocalGameServer</span> <span class="token operator">:</span> <span class="token base-clause"><span class="token keyword">public</span> croupier<span class="token double-colon punctuation">::</span>agent<span class="token double-colon punctuation">::</span>local<span class="token double-colon punctuation">::</span>v1<span class="token double-colon punctuation">::</span>LocalControlService<span class="token double-colon punctuation">::</span><span class="token class-name">Service</span></span> <span class="token punctuation">{</span></span>
<span class="line"><span class="token keyword">public</span><span class="token operator">:</span></span>
<span class="line">    <span class="token double-colon punctuation">::</span>grpc<span class="token double-colon punctuation">::</span>Status <span class="token function">InvokeFunction</span><span class="token punctuation">(</span></span>
<span class="line">        <span class="token double-colon punctuation">::</span>grpc<span class="token double-colon punctuation">::</span>ServerContext<span class="token operator">*</span> context<span class="token punctuation">,</span></span>
<span class="line">        <span class="token keyword">const</span> croupier<span class="token double-colon punctuation">::</span>function<span class="token double-colon punctuation">::</span>v1<span class="token double-colon punctuation">::</span>InvokeRequest<span class="token operator">*</span> request<span class="token punctuation">,</span></span>
<span class="line">        croupier<span class="token double-colon punctuation">::</span>function<span class="token double-colon punctuation">::</span>v1<span class="token double-colon punctuation">::</span>InvokeResponse<span class="token operator">*</span> response</span>
<span class="line">    <span class="token punctuation">)</span> <span class="token keyword">override</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// 1. 查找 function_id 对应的 handler</span></span>
<span class="line">        <span class="token keyword">auto</span> handler <span class="token operator">=</span> handlers_<span class="token punctuation">[</span>request<span class="token operator">-></span><span class="token function">function_id</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">        </span>
<span class="line">        <span class="token comment">// 2. 执行 handler，获得 response payload</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> <span class="token function">handler</span><span class="token punctuation">(</span><span class="token string">""</span><span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span><span class="token function">string</span><span class="token punctuation">(</span>request<span class="token operator">-></span><span class="token function">payload</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">begin</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> request<span class="token operator">-></span><span class="token function">payload</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">end</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        </span>
<span class="line">        <span class="token comment">// 3. 返回结果</span></span>
<span class="line">        response<span class="token operator">-></span><span class="token function">set_payload</span><span class="token punctuation">(</span>result<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token double-colon punctuation">::</span>grpc<span class="token double-colon punctuation">::</span>Status<span class="token double-colon punctuation">::</span>OK<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="重点-函数表维护" tabindex="-1"><a class="header-anchor" href="#重点-函数表维护"><span><strong>重点：函数表维护</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">private</span><span class="token operator">:</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span> handlers_<span class="token punctuation">;</span>      <span class="token comment">// function_id → handler</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionDescriptor<span class="token operator">></span> descriptors_<span class="token punctuation">;</span> <span class="token comment">// 元数据</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> VirtualObjectDescriptor<span class="token operator">></span> objects_<span class="token punctuation">;</span> <span class="token comment">// 对象描述</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> ComponentDescriptor<span class="token operator">></span> components_<span class="token punctuation">;</span>  <span class="token comment">// 组件描述</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp</code> (第 102-115 行)</p>
<h3 id="_3-5-连接参数" tabindex="-1"><a class="header-anchor" href="#_3-5-连接参数"><span>3.5 连接参数</span></a></h3>
<table>
<thead>
<tr>
<th>参数</th>
<th>默认值</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>agent_addr</code></td>
<td><code v-pre>127.0.0.1:19090</code></td>
<td>Agent 本地服务地址</td>
</tr>
<tr>
<td><code v-pre>local_listen</code></td>
<td><code v-pre>127.0.0.1:0</code></td>
<td>本地服务监听地址（0=自动分配端口）</td>
</tr>
<tr>
<td><code v-pre>service_id</code></td>
<td><code v-pre>cpp-service</code></td>
<td>服务标识，用于 Agent 识别</td>
</tr>
<tr>
<td><code v-pre>timeout_seconds</code></td>
<td><code v-pre>30</code></td>
<td>连接超时（秒）</td>
</tr>
<tr>
<td><code v-pre>heartbeat_interval</code></td>
<td><code v-pre>60</code></td>
<td>心跳间隔（秒）</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="_4️⃣-权限相关的接口设计" tabindex="-1"><a class="header-anchor" href="#_4️⃣-权限相关的接口设计"><span>4️⃣ 权限相关的接口设计</span></a></h2>
<h3 id="_4-1-权限模型概览" tabindex="-1"><a class="header-anchor" href="#_4-1-权限模型概览"><span>4.1 权限模型概览</span></a></h3>
<p><strong>多层权限架构</strong>:</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────┐</span>
<span class="line">│  Backend RBAC/ABAC (Server 层)       │</span>
<span class="line">├─────────────────────────────────────┤</span>
<span class="line">│  • 角色权限 (Role-Based)              │</span>
<span class="line">│  • 属性权限 (Attribute-Based)         │</span>
<span class="line">│  • 二人规则 (Two-Person Rule)        │</span>
<span class="line">│  • 审计链 (Audit Chain)              │</span>
<span class="line">└─────────────────┬───────────────────┘</span>
<span class="line">                  │ 验证</span>
<span class="line">┌─────────────────────────────────────┐</span>
<span class="line">│  Agent 权限验证层                     │</span>
<span class="line">├─────────────────────────────────────┤</span>
<span class="line">│  • game_id 租户隔离                    │</span>
<span class="line">│  • env 环境隔离                        │</span>
<span class="line">│  • session 会话管理                   │</span>
<span class="line">└─────────────────┬───────────────────┘</span>
<span class="line">                  │ 授权</span>
<span class="line">┌─────────────────────────────────────┐</span>
<span class="line">│  C++ SDK（应用层）                    │</span>
<span class="line">├─────────────────────────────────────┤</span>
<span class="line">│  • Handler 执行                       │</span>
<span class="line">│  • 本地业务逻辑                        │</span>
<span class="line">│  • 结果返回                            │</span>
<span class="line">└─────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-2-sdk-端权限接口" tabindex="-1"><a class="header-anchor" href="#_4-2-sdk-端权限接口"><span>4.2 SDK 端权限接口</span></a></h3>
<h4 id="_1-认证配置" tabindex="-1"><a class="header-anchor" href="#_1-认证配置"><span><strong>1. 认证配置</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">struct</span> <span class="token class-name">ClientConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// ========== Authentication ==========</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string auth_token<span class="token punctuation">;</span>                    <span class="token comment">// Bearer token</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> headers<span class="token punctuation">;</span> <span class="token comment">// 自定义 HTTP 头</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">InvokerConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// ========== Authentication &amp; Headers ==========</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string auth_token<span class="token punctuation">;</span>                    <span class="token comment">// Bearer token</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> headers<span class="token punctuation">;</span> <span class="token comment">// 额外的请求头</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>使用示例</strong>:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"my-game"</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"production"</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>auth_token <span class="token operator">=</span> <span class="token string">"Bearer eyJhbGc..."</span><span class="token punctuation">;</span>  <span class="token comment">// JWT Token</span></span>
<span class="line">config<span class="token punctuation">.</span>headers<span class="token punctuation">[</span><span class="token string">"X-Custom-Header"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"value"</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h</code> (第 76-78, 100-102 行)</p>
<h4 id="_2-tls-mtls-配置" tabindex="-1"><a class="header-anchor" href="#_2-tls-mtls-配置"><span><strong>2. TLS/mTLS 配置</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">struct</span> <span class="token class-name">ClientConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// ========== Optional TLS Configuration ==========</span></span>
<span class="line">    <span class="token keyword">bool</span> insecure <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span>              <span class="token comment">// 开发：true，生产：false</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string cert_file<span class="token punctuation">;</span>             <span class="token comment">// 客户端证书</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string key_file<span class="token punctuation">;</span>              <span class="token comment">// 私钥</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string ca_file<span class="token punctuation">;</span>               <span class="token comment">// CA 证书</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string server_name<span class="token punctuation">;</span>           <span class="token comment">// SNI 验证</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>生产配置示例</strong>:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">ClientConfig production_config<span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"my-production-game"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"production"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>cert_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/client.crt"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>key_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/client.key"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>ca_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/ca.crt"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>server_name <span class="token operator">=</span> <span class="token string">"croupier.internal"</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h</code> (第 70-74 行)</p>
<h3 id="_4-3-后台权限协议" tabindex="-1"><a class="header-anchor" href="#_4-3-后台权限协议"><span>4.3 后台权限协议</span></a></h3>
<h4 id="proto-定义" tabindex="-1"><a class="header-anchor" href="#proto-定义"><span><strong>Proto 定义</strong>：</span></a></h4>
<p><strong>control.proto</strong> - 权限在后台处理：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">FunctionDescriptor</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>        <span class="token comment">// "player.ban"</span></span>
<span class="line">    <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> category <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// 权限分类：e.g. "player_management"</span></span>
<span class="line">    <span class="token builtin">string</span> risk <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>      <span class="token comment">// 风险等级："low" | "medium" | "high"</span></span>
<span class="line">    <span class="token builtin">string</span> entity <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>    <span class="token comment">// e.g. "player"</span></span>
<span class="line">    <span class="token builtin">string</span> operation <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span> <span class="token comment">// "create" | "read" | "update" | "delete"</span></span>
<span class="line">    <span class="token builtin">bool</span> enabled <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span>     <span class="token comment">// 是否启用（权限控制）</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">FunctionDescriptor</span> functions <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> rpc_addr <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>          <span class="token comment">// ← 租户隔离</span></span>
<span class="line">    <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>              <span class="token comment">// ← 环境隔离</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/proto/croupier/control/v1/control.proto</code> (第 7-24 行)</p>
<h3 id="_4-4-sdk-调用时的权限选项" tabindex="-1"><a class="header-anchor" href="#_4-4-sdk-调用时的权限选项"><span>4.4 SDK 调用时的权限选项</span></a></h3>
<h4 id="invokeoptions-中的权限相关字段" tabindex="-1"><a class="header-anchor" href="#invokeoptions-中的权限相关字段"><span><strong>InvokeOptions 中的权限相关字段</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">struct</span> <span class="token class-name">InvokeOptions</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string idempotency_key<span class="token punctuation">;</span>        <span class="token comment">// 幂等性（防重复）</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string route<span class="token punctuation">;</span>                  <span class="token comment">// 路由策略</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string target_service_id<span class="token punctuation">;</span>      <span class="token comment">// 目标服务（权限受限）</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string hash_key<span class="token punctuation">;</span>               <span class="token comment">// 一致性哈希</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string trace_id<span class="token punctuation">;</span>               <span class="token comment">// 追踪 ID（审计）</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> metadata<span class="token punctuation">;</span> <span class="token comment">// 请求元数据（可用于权限信息）</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>权限应用场景</strong>:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">InvokeOptions options<span class="token punctuation">;</span></span>
<span class="line">options<span class="token punctuation">.</span>idempotency_key <span class="token operator">=</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token double-colon punctuation">::</span>utils<span class="token double-colon punctuation">::</span><span class="token function">NewIdempotencyKey</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">options<span class="token punctuation">.</span>trace_id <span class="token operator">=</span> <span class="token string">"trace_123456"</span><span class="token punctuation">;</span>  <span class="token comment">// 用于审计日志追踪</span></span>
<span class="line">options<span class="token punctuation">.</span>metadata<span class="token punctuation">[</span><span class="token string">"user_id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"admin_user_1"</span><span class="token punctuation">;</span>  <span class="token comment">// 可在后台进行权限检查</span></span>
<span class="line">options<span class="token punctuation">.</span>metadata<span class="token punctuation">[</span><span class="token string">"approval_id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"approval_xyz"</span><span class="token punctuation">;</span>  <span class="token comment">// 审批流水号</span></span>
<span class="line"></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> invoker<span class="token punctuation">.</span><span class="token function">Invoke</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> payload<span class="token punctuation">,</span> options<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h</code> (第 108-116 行)</p>
<h3 id="_4-5-权限验证流程" tabindex="-1"><a class="header-anchor" href="#_4-5-权限验证流程"><span>4.5 权限验证流程</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">C++ SDK 调用</span>
<span class="line">    ↓</span>
<span class="line">┌─────────────────────────────────────┐</span>
<span class="line">│ 1. 客户端验证 (SDK)                  │</span>
<span class="line">│  • 检查认证 token                    │</span>
<span class="line">│  • 验证 TLS 证书                     │</span>
<span class="line">└──────────────┬──────────────────────┘</span>
<span class="line">               ↓</span>
<span class="line">┌─────────────────────────────────────┐</span>
<span class="line">│ 2. Agent 层验证                      │</span>
<span class="line">│  • 检查 session 有效性               │</span>
<span class="line">│  • 验证 game_id 权限                 │</span>
<span class="line">│  • 验证 env 访问权限                 │</span>
<span class="line">└──────────────┬──────────────────────┘</span>
<span class="line">               ↓</span>
<span class="line">┌─────────────────────────────────────┐</span>
<span class="line">│ 3. Server 层验证 (RBAC/ABAC)        │</span>
<span class="line">│  • 检查用户角色                      │</span>
<span class="line">│  • 检查函数访问权限                  │</span>
<span class="line">│  • 检查属性权限 (ABAC)              │</span>
<span class="line">│  • 触发审批流 (如果需要)             │</span>
<span class="line">└──────────────┬──────────────────────┘</span>
<span class="line">               ↓</span>
<span class="line">┌─────────────────────────────────────┐</span>
<span class="line">│ 4. 执行函数                          │</span>
<span class="line">│  • 调用本地 handler                 │</span>
<span class="line">│  • 生成审计日志                      │</span>
<span class="line">└─────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-6-权限相关的数据结构" tabindex="-1"><a class="header-anchor" href="#_4-6-权限相关的数据结构"><span>4.6 权限相关的数据结构</span></a></h3>
<h4 id="函数描述符中的权限信息" tabindex="-1"><a class="header-anchor" href="#函数描述符中的权限信息"><span><strong>函数描述符中的权限信息</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">struct</span> <span class="token class-name">FunctionDescriptor</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string id<span class="token punctuation">;</span>         <span class="token comment">// "player.ban"</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string version<span class="token punctuation">;</span>    <span class="token comment">// "1.0.0"</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> schema<span class="token punctuation">;</span>  <span class="token comment">// 参数 schema（可包含权限需求）</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 扩展提案（后续版本）：</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">FunctionDescriptorExtended</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string category<span class="token punctuation">;</span>       <span class="token comment">// "player_management"</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string risk_level<span class="token punctuation">;</span>     <span class="token comment">// "high" - 需要更严格审批</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string required_role<span class="token punctuation">;</span>  <span class="token comment">// "admin" - 所需角色</span></span>
<span class="line">    <span class="token keyword">bool</span> requires_approval<span class="token punctuation">;</span>     <span class="token comment">// true - 需要二人规则</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h</code> (第 20-25 行)</p>
<h3 id="_4-7-审计和追踪" tabindex="-1"><a class="header-anchor" href="#_4-7-审计和追踪"><span>4.7 审计和追踪</span></a></h3>
<h4 id="追踪-id-机制" tabindex="-1"><a class="header-anchor" href="#追踪-id-机制"><span><strong>追踪 ID 机制</strong></span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 生成唯一追踪 ID</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string trace_id <span class="token operator">=</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token double-colon punctuation">::</span>utils<span class="token double-colon punctuation">::</span><span class="token function">NewIdempotencyKey</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">InvokeOptions options<span class="token punctuation">;</span></span>
<span class="line">options<span class="token punctuation">.</span>trace_id <span class="token operator">=</span> trace_id<span class="token punctuation">;</span></span>
<span class="line">options<span class="token punctuation">.</span>idempotency_key <span class="token operator">=</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token double-colon punctuation">::</span>utils<span class="token double-colon punctuation">::</span><span class="token function">NewIdempotencyKey</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 后台会在审计日志中记录：</span></span>
<span class="line"><span class="token comment">// {</span></span>
<span class="line"><span class="token comment">//   "trace_id": "abc123...",</span></span>
<span class="line"><span class="token comment">//   "idempotency_key": "def456...",</span></span>
<span class="line"><span class="token comment">//   "function_id": "player.ban",</span></span>
<span class="line"><span class="token comment">//   "game_id": "game_x",</span></span>
<span class="line"><span class="token comment">//   "timestamp": "2025-11-13T10:30:00Z",</span></span>
<span class="line"><span class="token comment">//   "user": "admin_1",</span></span>
<span class="line"><span class="token comment">//   "result": "success"</span></span>
<span class="line"><span class="token comment">// }</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>文件位置</strong>: <code v-pre>/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp</code> (第 17-27 行)</p>
<hr>
<h2 id="📂-目录结构详解" tabindex="-1"><a class="header-anchor" href="#📂-目录结构详解"><span>📂 目录结构详解</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">/Users/cui/Workspaces/croupier/sdks/cpp/</span>
<span class="line">├── CMakeLists.txt                    # 构建系统配置</span>
<span class="line">│                                    # - gRPC/Protobuf 集成</span>
<span class="line">│                                    # - 多平台支持 (Windows/Linux/macOS)</span>
<span class="line">│                                    # - vcpkg 依赖管理</span>
<span class="line">│</span>
<span class="line">├── include/croupier/sdk/</span>
<span class="line">│   └── croupier_client.h            # 【核心公开接口】</span>
<span class="line">│                                    # - CroupierClient (SPI 实现)</span>
<span class="line">│                                    # - CroupierInvoker (调用者)</span>
<span class="line">│                                    # - ClientConfig/InvokerConfig (game_id/env)</span>
<span class="line">│                                    # - 虚拟对象相关数据结构</span>
<span class="line">│</span>
<span class="line">├── src/</span>
<span class="line">│   └── croupier_client.cpp          # 【核心实现】</span>
<span class="line">│                                    # - Impl class (PImpl 模式)</span>
<span class="line">│                                    # - 本地 gRPC 服务器启动</span>
<span class="line">│                                    # - Handler 映射和调用</span>
<span class="line">│                                    # - game_id/env 验证逻辑</span>
<span class="line">│</span>
<span class="line">├── examples/</span>
<span class="line">│   └── virtual_object_demo.cpp      # 【使用示例】</span>
<span class="line">│                                    # - 6 个演示场景</span>
<span class="line">│                                    # - 虚拟对象注册流程</span>
<span class="line">│                                    # - 完整的 handler 实现</span>
<span class="line">│</span>
<span class="line">├── .github/workflows/</span>
<span class="line">│   └── cpp-sdk-build.yml            # 【CI/CD 自动化】</span>
<span class="line">│                                    # - 每日构建 (nightly)</span>
<span class="line">│                                    # - 多平台矩阵编译</span>
<span class="line">│                                    # - 自动发布 releases</span>
<span class="line">│</span>
<span class="line">├── vcpkg.json                       # 【依赖描述】</span>
<span class="line">│                                    # - gRPC, Protobuf, nlohmann-json</span>
<span class="line">│</span>
<span class="line">├── README.md                        # 【用户文档】</span>
<span class="line">│                                    # - 快速开始指南</span>
<span class="line">│                                    # - API 参考</span>
<span class="line">│                                    # - 部署说明</span>
<span class="line">│</span>
<span class="line">└── VIRTUAL_OBJECT_REGISTRATION.md  # 【架构文档】</span>
<span class="line">                                    # - 四层设计</span>
<span class="line">                                    # - ID 引用模式</span>
<span class="line">                                    # - 实现指南</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="关键文件功能对应" tabindex="-1"><a class="header-anchor" href="#关键文件功能对应"><span>关键文件功能对应</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>主要文件</th>
<th>行号范围</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>SPI 定义</strong></td>
<td>croupier_client.h</td>
<td>10-220</td>
</tr>
<tr>
<td><strong>game_id/env</strong></td>
<td>croupier_client.h</td>
<td>57-106</td>
</tr>
<tr>
<td><strong>虚拟对象结构</strong></td>
<td>croupier_client.h</td>
<td>20-55</td>
</tr>
<tr>
<td><strong>权限配置</strong></td>
<td>croupier_client.h</td>
<td>70-102</td>
</tr>
<tr>
<td><strong>Handler 实现</strong></td>
<td>croupier_client.cpp</td>
<td>102-407</td>
</tr>
<tr>
<td><strong>本地服务器</strong></td>
<td>croupier_client.cpp</td>
<td>317-406</td>
</tr>
<tr>
<td><strong>示例代码</strong></td>
<td>virtual_object_demo.cpp</td>
<td>1-334</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="🔌-集成示例" tabindex="-1"><a class="header-anchor" href="#🔌-集成示例"><span>🔌 集成示例</span></a></h2>
<h3 id="完整的游戏经济系统集成" tabindex="-1"><a class="header-anchor" href="#完整的游戏经济系统集成"><span>完整的游戏经济系统集成</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 1. 定义钱包实体的操作处理器</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">WalletGetHandler</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> ctx<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span> data <span class="token operator">=</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ParseJSON</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string wallet_id <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"wallet_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token comment">// 业务逻辑：从数据库获取钱包信息</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token string">"{\"wallet_id\":\""</span> <span class="token operator">+</span> wallet_id <span class="token operator">+</span> <span class="token string">"\",\"balance\":\"1000\"}"</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">WalletTransferHandler</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> ctx<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 业务逻辑：转账操作</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token string">"{\"status\":\"success\"}"</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 2. 配置客户端</span></span>
<span class="line">    ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"mmorpg-game"</span><span class="token punctuation">;</span>        <span class="token comment">// 🎮 游戏标识</span></span>
<span class="line">    config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"production"</span><span class="token punctuation">;</span>              <span class="token comment">// 🔧 环境隔离</span></span>
<span class="line">    config<span class="token punctuation">.</span>service_id <span class="token operator">=</span> <span class="token string">"economy-service"</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>agent_addr <span class="token operator">=</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>cert_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/client.crt"</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 3. 定义虚拟对象</span></span>
<span class="line">    VirtualObjectDescriptor wallet<span class="token punctuation">;</span></span>
<span class="line">    wallet<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"wallet.entity"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet<span class="token punctuation">.</span>name <span class="token operator">=</span> <span class="token string">"玩家钱包"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.get"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    RelationshipDef currency_rel<span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>type <span class="token operator">=</span> <span class="token string">"many-to-one"</span><span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>entity <span class="token operator">=</span> <span class="token string">"currency"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet<span class="token punctuation">.</span>relationships<span class="token punctuation">[</span><span class="token string">"currency"</span><span class="token punctuation">]</span> <span class="token operator">=</span> currency_rel<span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 4. 关联处理器</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span> handlers<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.get"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletGetHandler<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletTransferHandler<span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 5. 注册虚拟对象</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span>client<span class="token punctuation">.</span><span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span>wallet<span class="token punctuation">,</span> handlers<span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Failed to register wallet"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 6. 连接并服务</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span>client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Failed to connect to agent"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 7. 启动阻塞服务</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span>  <span class="token comment">// 接收来自后台的函数调用</span></span>
<span class="line">    </span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="📚-总结" tabindex="-1"><a class="header-anchor" href="#📚-总结"><span>📚 总结</span></a></h2>
<table>
<thead>
<tr>
<th>方面</th>
<th>关键设计</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>SPI</strong></td>
<td>Handler 回调 + 描述符驱动</td>
</tr>
<tr>
<td><strong>game_id/env</strong></td>
<td>客户端配置必需字段，实现租户隔离</td>
</tr>
<tr>
<td><strong>Agent 交互</strong></td>
<td>LocalControlService gRPC，注册+心跳模式</td>
</tr>
<tr>
<td><strong>权限</strong></td>
<td>分层验证：认证 → Agent 授权 → Server RBAC/ABAC</td>
</tr>
<tr>
<td><strong>架构</strong></td>
<td>四层：Function → Entity → Resource → Component</td>
</tr>
</tbody>
</table>
<p><strong>核心优势</strong>：</p>
<ul>
<li>✅ 高性能（ID 引用模式，无重对象序列化）</li>
<li>✅ 易扩展（声明式配置，模块化组件）</li>
<li>✅ 安全（多层权限验证，审计追踪）</li>
<li>✅ 多环境（game_id + env 隔离）</li>
</ul>
</div></template>


