<template><div><h1 id="croupier-c-sdk-快速参考" tabindex="-1"><a class="header-anchor" href="#croupier-c-sdk-快速参考"><span>Croupier C++ SDK 快速参考</span></a></h1>
<h2 id="🎯-四种使用模式速查表" tabindex="-1"><a class="header-anchor" href="#🎯-四种使用模式速查表"><span>🎯 四种使用模式速查表</span></a></h2>
<h3 id="模式1-基础函数注册" tabindex="-1"><a class="header-anchor" href="#模式1-基础函数注册"><span>模式1：基础函数注册</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">FunctionDescriptor desc<span class="token punctuation">{</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span> <span class="token string">"1.0.0"</span><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">client<span class="token punctuation">.</span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>desc<span class="token punctuation">,</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> ctx<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 处理函数逻辑</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token string">"{\"status\":\"ok\"}"</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="模式2-虚拟对象注册" tabindex="-1"><a class="header-anchor" href="#模式2-虚拟对象注册"><span>模式2：虚拟对象注册</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">VirtualObjectDescriptor wallet<span class="token punctuation">;</span></span>
<span class="line">wallet<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"wallet.entity"</span><span class="token punctuation">;</span></span>
<span class="line">wallet<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span> handlers<span class="token punctuation">;</span></span>
<span class="line">handlers<span class="token punctuation">[</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">(</span><span class="token keyword">auto</span> ctx<span class="token punctuation">,</span> <span class="token keyword">auto</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span> <span class="token comment">/* ... */</span> <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">client<span class="token punctuation">.</span><span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span>wallet<span class="token punctuation">,</span> handlers<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="模式3-组件注册" tabindex="-1"><a class="header-anchor" href="#模式3-组件注册"><span>模式3：组件注册</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">ComponentDescriptor comp<span class="token punctuation">;</span></span>
<span class="line">comp<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"economy-system"</span><span class="token punctuation">;</span></span>
<span class="line">comp<span class="token punctuation">.</span>entities <span class="token operator">=</span> <span class="token punctuation">{</span>wallet<span class="token punctuation">,</span> currency<span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">comp<span class="token punctuation">.</span>functions <span class="token operator">=</span> <span class="token punctuation">{</span>market_trade<span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">client<span class="token punctuation">.</span><span class="token function">RegisterComponent</span><span class="token punctuation">(</span>comp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="模式4-配置文件驱动" tabindex="-1"><a class="header-anchor" href="#模式4-配置文件驱动"><span>模式4：配置文件驱动</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">client<span class="token punctuation">.</span><span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token string">"economy-system.json"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><hr>
<h2 id="🎮-game-id-env-配置表" tabindex="-1"><a class="header-anchor" href="#🎮-game-id-env-配置表"><span>🎮 game_id + env 配置表</span></a></h2>
<table>
<thead>
<tr>
<th>game_id 示例</th>
<th>env</th>
<th>insecure</th>
<th>用途</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>game-dev</code></td>
<td>development</td>
<td>true</td>
<td>本地开发</td>
</tr>
<tr>
<td><code v-pre>game-staging</code></td>
<td>staging</td>
<td>true/false</td>
<td>预发布测试</td>
</tr>
<tr>
<td><code v-pre>game-prod</code></td>
<td>production</td>
<td>false</td>
<td>生产环境</td>
</tr>
</tbody>
</table>
<p><strong>关键代码</strong>：</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"my-game"</span><span class="token punctuation">;</span>              <span class="token comment">// 🎮 必需</span></span>
<span class="line">config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"production"</span><span class="token punctuation">;</span>               <span class="token comment">// 🔧 必需</span></span>
<span class="line">config<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span>                 <span class="token comment">// 🔐 生产需要关闭</span></span>
<span class="line">config<span class="token punctuation">.</span>auth_token <span class="token operator">=</span> <span class="token string">"Bearer token..."</span><span class="token punctuation">;</span>   <span class="token comment">// 🔑 认证</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="📡-与-agent-交互流程" tabindex="-1"><a class="header-anchor" href="#📡-与-agent-交互流程"><span>📡 与 Agent 交互流程</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">SDK 启动</span>
<span class="line">  ↓</span>
<span class="line">RegisterFunction/RegisterVirtualObject/RegisterComponent</span>
<span class="line">  ↓</span>
<span class="line">Connect()  ← 连接到 Agent (127.0.0.1:19090)</span>
<span class="line">  ↓</span>
<span class="line">Serve()    ← 启动本地 gRPC 服务器，等待调用</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>Agent 消息协议</strong>：</p>
<ul>
<li><strong>RegisterLocal</strong>: 向 Agent 注册服务</li>
<li><strong>Heartbeat</strong>: 定期保活 (60秒)</li>
<li><strong>FunctionService</strong>: 接收来自 Server 的调用</li>
</ul>
<hr>
<h2 id="🔐-权限和认证" tabindex="-1"><a class="header-anchor" href="#🔐-权限和认证"><span>🔐 权限和认证</span></a></h2>
<h3 id="开发环境" tabindex="-1"><a class="header-anchor" href="#开发环境"><span>开发环境</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span>  <span class="token comment">// 允许不安全连接</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="生产环境" tabindex="-1"><a class="header-anchor" href="#生产环境"><span>生产环境</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>cert_file <span class="token operator">=</span> <span class="token string">"/etc/certs/client.crt"</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>key_file <span class="token operator">=</span> <span class="token string">"/etc/certs/client.key"</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>ca_file <span class="token operator">=</span> <span class="token string">"/etc/certs/ca.crt"</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>auth_token <span class="token operator">=</span> <span class="token string">"Bearer &lt;JWT>"</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>headers<span class="token punctuation">[</span><span class="token string">"X-Request-Id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"trace_123"</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="调用时的权限" tabindex="-1"><a class="header-anchor" href="#调用时的权限"><span>调用时的权限</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">InvokeOptions opts<span class="token punctuation">;</span></span>
<span class="line">opts<span class="token punctuation">.</span>idempotency_key <span class="token operator">=</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token double-colon punctuation">::</span>utils<span class="token double-colon punctuation">::</span><span class="token function">NewIdempotencyKey</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">opts<span class="token punctuation">.</span>trace_id <span class="token operator">=</span> <span class="token string">"trace_xyz"</span><span class="token punctuation">;</span>  <span class="token comment">// 审计</span></span>
<span class="line">opts<span class="token punctuation">.</span>metadata<span class="token punctuation">[</span><span class="token string">"user_id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"admin_1"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">invoker<span class="token punctuation">.</span><span class="token function">Invoke</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> payload<span class="token punctuation">,</span> opts<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="📂-目录导航" tabindex="-1"><a class="header-anchor" href="#📂-目录导航"><span>📂 目录导航</span></a></h2>
<table>
<thead>
<tr>
<th>文件</th>
<th>用途</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>include/croupier/sdk/croupier_client.h</code></td>
<td>核心公开接口（SPI 定义）</td>
</tr>
<tr>
<td><code v-pre>src/croupier_client.cpp</code></td>
<td>实现细节（Handler 存储、验证）</td>
</tr>
<tr>
<td><code v-pre>examples/virtual_object_demo.cpp</code></td>
<td>6 个完整示例</td>
</tr>
<tr>
<td><code v-pre>proto/croupier/control/v1/control.proto</code></td>
<td>后台消息协议</td>
</tr>
<tr>
<td><code v-pre>proto/croupier/agent/local/v1/local.proto</code></td>
<td>Agent 协议</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="⚙️-handler-签名" tabindex="-1"><a class="header-anchor" href="#⚙️-handler-签名"><span>⚙️ Handler 签名</span></a></h2>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 标准签名</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">HandlerFunction</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> context<span class="token punctuation">,</span>   <span class="token comment">// 调用上下文（目前未用）</span></span>
<span class="line">    <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload    <span class="token comment">// JSON 字符串</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 实现模板</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">MyHandler</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> ctx<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 1. 解析</span></span>
<span class="line">    <span class="token keyword">auto</span> data <span class="token operator">=</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ParseJSON</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string param <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"key"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 2. 处理</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> <span class="token function">DoSomething</span><span class="token punctuation">(</span>param<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 3. 返回</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> resp<span class="token punctuation">;</span></span>
<span class="line">    resp<span class="token punctuation">[</span><span class="token string">"result"</span><span class="token punctuation">]</span> <span class="token operator">=</span> result<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ToJSON</span><span class="token punctuation">(</span>resp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="🔍-调试和诊断" tabindex="-1"><a class="header-anchor" href="#🔍-调试和诊断"><span>🔍 调试和诊断</span></a></h2>
<h3 id="获取注册信息" tabindex="-1"><a class="header-anchor" href="#获取注册信息"><span>获取注册信息</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">auto</span> objects <span class="token operator">=</span> client<span class="token punctuation">.</span><span class="token function">GetRegisteredObjects</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">auto</span> components <span class="token operator">=</span> client<span class="token punctuation">.</span><span class="token function">GetRegisteredComponents</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">for</span> <span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">auto</span><span class="token operator">&amp;</span> obj <span class="token operator">:</span> objects<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Object: "</span> <span class="token operator">&lt;&lt;</span> obj<span class="token punctuation">.</span>id <span class="token operator">&lt;&lt;</span> <span class="token string">" with "</span> </span>
<span class="line">              <span class="token operator">&lt;&lt;</span> obj<span class="token punctuation">.</span>operations<span class="token punctuation">.</span><span class="token function">size</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">&lt;&lt;</span> <span class="token string">" operations"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="日志和错误处理" tabindex="-1"><a class="header-anchor" href="#日志和错误处理"><span>日志和错误处理</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span>desc<span class="token punctuation">,</span> handlers<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Error: "</span> <span class="token operator">&lt;&lt;</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token comment">// 实现重连逻辑</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="📊-性能考虑" tabindex="-1"><a class="header-anchor" href="#📊-性能考虑"><span>📊 性能考虑</span></a></h2>
<h3 id="✅-最优实践" tabindex="-1"><a class="header-anchor" href="#✅-最优实践"><span>✅ 最优实践</span></a></h3>
<ul>
<li>使用 ID 引用模式（只传递 ID 字符串）</li>
<li>Handler 保持无状态</li>
<li>使用对象缓存而非重复序列化</li>
<li>定期检查心跳状态</li>
</ul>
<h3 id="❌-避免的做法" tabindex="-1"><a class="header-anchor" href="#❌-避免的做法"><span>❌ 避免的做法</span></a></h3>
<ul>
<li>传递序列化的大对象</li>
<li>Handler 中阻塞操作（或异步化）</li>
<li>频繁重新连接</li>
<li>忽视幂等性检查</li>
</ul>
<hr>
<h2 id="🚀-构建和依赖" tabindex="-1"><a class="header-anchor" href="#🚀-构建和依赖"><span>🚀 构建和依赖</span></a></h2>
<h3 id="vcpkg-依赖" tabindex="-1"><a class="header-anchor" href="#vcpkg-依赖"><span>vcpkg 依赖</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">✓ gRPC      (gRPC 通信)</span>
<span class="line">✓ Protobuf  (消息编码)</span>
<span class="line">✓ nlohmann/json (JSON 处理)</span>
<span class="line">✓ gtest     (可选测试)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="构建命令" tabindex="-1"><a class="header-anchor" href="#构建命令"><span>构建命令</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 快速构建</span></span>
<span class="line">./scripts/build.sh</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启用测试和示例</span></span>
<span class="line">./scripts/build.sh <span class="token parameter variable">--tests</span> ON <span class="token parameter variable">--examples</span> ON</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 清理重建</span></span>
<span class="line">./scripts/build.sh <span class="token parameter variable">--clean</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="📋-关键类和方法速查" tabindex="-1"><a class="header-anchor" href="#📋-关键类和方法速查"><span>📋 关键类和方法速查</span></a></h2>
<h3 id="croupierclient" tabindex="-1"><a class="header-anchor" href="#croupierclient"><span>CroupierClient</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">bool</span> <span class="token function">RegisterFunction</span><span class="token punctuation">(</span><span class="token keyword">const</span> FunctionDescriptor<span class="token operator">&amp;</span><span class="token punctuation">,</span> FunctionHandler<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span><span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span><span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token operator">></span><span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">RegisterComponent</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">void</span> <span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">void</span> <span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>VirtualObjectDescriptor<span class="token operator">></span> <span class="token function">GetRegisteredObjects</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">const</span><span class="token punctuation">;</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>ComponentDescriptor<span class="token operator">></span> <span class="token function">GetRegisteredComponents</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">const</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="croupierinvoker" tabindex="-1"><a class="header-anchor" href="#croupierinvoker"><span>CroupierInvoker</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">bool</span> <span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">Invoke</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> func_id<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">,</span> <span class="token keyword">const</span> InvokeOptions<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">StartJob</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> func_id<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">,</span> <span class="token keyword">const</span> InvokeOptions<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>future<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>JobEvent<span class="token operator">>></span> <span class="token function">StreamJob</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> job_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">CancelJob</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> job_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="工具函数-utils" tabindex="-1"><a class="header-anchor" href="#工具函数-utils"><span>工具函数 (utils)</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">NewIdempotencyKey</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span>                                    <span class="token comment">// 生成唯一 ID</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">ValidateJSON</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span><span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token operator">></span><span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span>       <span class="token comment">// 验证 JSON</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> <span class="token function">ParseJSON</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span>  <span class="token comment">// 解析 JSON</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">ToJSON</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token operator">></span><span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span>                          <span class="token comment">// 转换为 JSON</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">ValidateObjectDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span>     <span class="token comment">// 验证对象</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">ValidateComponentDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token punctuation">;</span>      <span class="token comment">// 验证组件</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">GenerateObjectTemplate</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> id<span class="token punctuation">)</span><span class="token punctuation">;</span>         <span class="token comment">// 生成模板</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="🔗-相关资源" tabindex="-1"><a class="header-anchor" href="#🔗-相关资源"><span>🔗 相关资源</span></a></h2>
<ul>
<li><strong>完整分析</strong>: <code v-pre>docs/CPP_SDK_DEEP_ANALYSIS.md</code></li>
<li><strong>README</strong>: <code v-pre>sdks/cpp/README.md</code></li>
<li><strong>架构文档</strong>: <code v-pre>sdks/cpp/VIRTUAL_OBJECT_REGISTRATION.md</code></li>
<li><strong>示例代码</strong>: <code v-pre>sdks/cpp/examples/virtual_object_demo.cpp</code></li>
<li><strong>Proto 定义</strong>:
<ul>
<li><code v-pre>proto/croupier/control/v1/control.proto</code></li>
<li><code v-pre>proto/croupier/agent/local/v1/local.proto</code></li>
<li><code v-pre>proto/croupier/function/v1/function.proto</code></li>
</ul>
</li>
</ul>
</div></template>


