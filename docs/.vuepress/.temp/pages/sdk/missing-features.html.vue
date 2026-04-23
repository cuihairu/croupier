<template><div><h1 id="croupier-sdk-缺失功能清单" tabindex="-1"><a class="header-anchor" href="#croupier-sdk-缺失功能清单"><span>Croupier SDK 缺失功能清单</span></a></h1>
<p>本文档汇总了各个 SDK 相对于规范和最佳实践的缺失功能，以及需要改进的地方。</p>
<p><strong>最后更新:</strong> 2025-01-06 - 重试机制已完成</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<ul>
<li><a href="#%E5%8A%9F%E8%83%BD%E5%AF%B9%E6%AF%94%E7%9F%A9%E9%98%B5">功能对比矩阵</a></li>
<li><a href="#go-sdk">Go SDK</a></li>
<li><a href="#javascripttypescript-sdk">JavaScript/TypeScript SDK</a></li>
<li><a href="#python-sdk">Python SDK</a></li>
<li><a href="#java-sdk">Java SDK</a></li>
<li><a href="#c-sdk">C++ SDK</a></li>
<li><a href="#%E8%B7%A8-sdk-%E7%BB%9F%E4%B8%80%E6%94%B9%E8%BF%9B%E9%A1%B9">跨 SDK 统一改进项</a></li>
</ul>
<hr>
<h2 id="功能对比矩阵" tabindex="-1"><a class="header-anchor" href="#功能对比矩阵"><span>功能对比矩阵</span></a></h2>
<table>
<thead>
<tr>
<th>功能类别</th>
<th style="text-align:center">Go SDK</th>
<th style="text-align:center">JS SDK</th>
<th style="text-align:center">Python SDK</th>
<th style="text-align:center">Java SDK</th>
<th style="text-align:center">C++ SDK</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>核心接口</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>Client 接口</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>Invoker 接口</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>函数注册</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>函数描述符</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>版本管理</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>元数据支持</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>JSON Schema 验证</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>连接管理</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>连接建立</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>自动重连</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>指数退避</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>会话管理</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>生命周期</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>Start/Stop</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>优雅关闭</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>配置选项</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>ClientConfig</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>InvokerConfig</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>TLS/mTLS</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>错误处理</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>错误类型</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>重试机制</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>特殊功能</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>心跳机制</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>幂等性支持</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>作业管理</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>流式事件</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>高级功能</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>文件传输</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
</tr>
<tr>
<td>虚拟对象系统</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>组件系统</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td><strong>代码质量</strong></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
<td style="text-align:center"></td>
</tr>
<tr>
<td>单元测试</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">⚠️</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>集成测试</td>
<td style="text-align:center">⚠️</td>
<td style="text-align:center">❌</td>
<td style="text-align:center">⚠️</td>
<td style="text-align:center">⚠️</td>
<td style="text-align:center">✅</td>
</tr>
<tr>
<td>文档完整性</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
<td style="text-align:center">✅</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="go-sdk" tabindex="-1"><a class="header-anchor" href="#go-sdk"><span>Go SDK</span></a></h2>
<p><strong>位置:</strong> <code v-pre>sdks/go/</code></p>
<blockquote>
<p>✅ <strong>更新:</strong> 自动重连已于 2025-01-06 完成实现</p>
</blockquote>
<h3 id="已实现功能" tabindex="-1"><a class="header-anchor" href="#已实现功能"><span>已实现功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th style="text-align:center">状态</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Invoker 接口</strong></td>
<td style="text-align:center">✅</td>
<td>完整实现</td>
</tr>
<tr>
<td><strong>幂等性支持</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>InvokeOptions</code> 包含 <code v-pre>IdempotencyKey</code></td>
</tr>
<tr>
<td><strong>流式事件</strong></td>
<td style="text-align:center">✅</td>
<td>使用 channel 流式获取作业事件</td>
</tr>
<tr>
<td><strong>作业取消</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>CancelJob()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 设置</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>SetSchema()</code> 方法</td>
</tr>
<tr>
<td><strong>JSON Schema 验证</strong></td>
<td style="text-align:center">✅</td>
<td>使用 <code v-pre>gojsonschema</code> 完整实现</td>
</tr>
<tr>
<td><strong>自动重连</strong></td>
<td style="text-align:center">✅</td>
<td>指数退避 + 抖动重连策略</td>
</tr>
</tbody>
</table>
<h3 id="缺失功能" tabindex="-1"><a class="header-anchor" href="#缺失功能"><span>缺失功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>优先级</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>文件传输功能</strong></td>
<td>低</td>
<td><code v-pre>uploadFile</code> 接口未实现</td>
</tr>
</tbody>
</table>
<h3 id="重试机制说明" tabindex="-1"><a class="header-anchor" href="#重试机制说明"><span>重试机制说明</span></a></h3>
<blockquote>
<p>✅ <strong>更新:</strong> 重试机制已于 2025-01-06 完成实现</p>
</blockquote>
<p>Go SDK 的重试机制包含以下特性：</p>
<table>
<thead>
<tr>
<th>特性</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>指数退避</strong></td>
<td>每次重试延迟按倍数增长</td>
</tr>
<tr>
<td><strong>抖动</strong></td>
<td>添加随机性防止惊群效应</td>
</tr>
<tr>
<td><strong>可重试状态码</strong></td>
<td>UNAVAILABLE, INTERNAL, UNKNOWN, ABORTED, DEADLINE_EXCEEDED</td>
</tr>
<tr>
<td><strong>可配置</strong></td>
<td>通过 <code v-pre>RetryConfig</code> 配置所有参数</td>
</tr>
<tr>
<td><strong>调用级覆盖</strong></td>
<td><code v-pre>InvokeOptions.retry</code> 可覆盖默认配置</td>
</tr>
</tbody>
</table>
<h3 id="代码示例-重试机制" tabindex="-1"><a class="header-anchor" href="#代码示例-重试机制"><span>代码示例 - 重试机制</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">import</span> <span class="token string">"github.com/cuihairu/croupier/sdks/go/pkg/croupier"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 创建 Invoker（带重试配置）</span></span>
<span class="line">config <span class="token operator">:=</span> <span class="token operator">&amp;</span>croupier<span class="token punctuation">.</span>InvokerConfig<span class="token punctuation">{</span></span>
<span class="line">    Address<span class="token punctuation">:</span> <span class="token string">"127.0.0.1:8080"</span><span class="token punctuation">,</span></span>
<span class="line">    Retry<span class="token punctuation">:</span> <span class="token operator">&amp;</span>croupier<span class="token punctuation">.</span>RetryConfig<span class="token punctuation">{</span></span>
<span class="line">        Enabled<span class="token punctuation">:</span>           <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">        MaxAttempts<span class="token punctuation">:</span>       <span class="token number">3</span><span class="token punctuation">,</span>              <span class="token comment">// 最多重试 3 次</span></span>
<span class="line">        InitialDelayMs<span class="token punctuation">:</span>    <span class="token number">100</span><span class="token punctuation">,</span>            <span class="token comment">// 初始延迟 100ms</span></span>
<span class="line">        MaxDelayMs<span class="token punctuation">:</span>        <span class="token number">5000</span><span class="token punctuation">,</span>           <span class="token comment">// 最大延迟 5 秒</span></span>
<span class="line">        BackoffMultiplier<span class="token punctuation">:</span> <span class="token number">2.0</span><span class="token punctuation">,</span>            <span class="token comment">// 指数退避倍数</span></span>
<span class="line">        JitterFactor<span class="token punctuation">:</span>      <span class="token number">0.1</span><span class="token punctuation">,</span>            <span class="token comment">// 抖动因子</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">invoker <span class="token operator">:=</span> croupier<span class="token punctuation">.</span><span class="token function">NewInvoker</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span></span>
<span class="line">err <span class="token operator">:=</span> invoker<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span>ctx<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Invoke 会在失败时自动重试</span></span>
<span class="line">result<span class="token punctuation">,</span> err <span class="token operator">:=</span> invoker<span class="token punctuation">.</span><span class="token function">Invoke</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="需要改进" tabindex="-1"><a class="header-anchor" href="#需要改进"><span>需要改进</span></a></h3>
<table>
<thead>
<tr>
<th>项目</th>
<th>当前状态</th>
<th>建议改进</th>
</tr>
</thead>
<tbody>
<tr>
<td>集成测试</td>
<td>较少</td>
<td>增加端到端测试</td>
</tr>
</tbody>
</table>
<h3 id="代码示例-自动重连配置" tabindex="-1"><a class="header-anchor" href="#代码示例-自动重连配置"><span>代码示例 - 自动重连配置</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 自动重连已实现</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"github.com/cuihairu/croupier/sdks/go/pkg/croupier"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 创建 Invoker（带自动重连配置）</span></span>
<span class="line">config <span class="token operator">:=</span> <span class="token operator">&amp;</span>croupier<span class="token punctuation">.</span>InvokerConfig<span class="token punctuation">{</span></span>
<span class="line">    Address<span class="token punctuation">:</span>        <span class="token string">"127.0.0.1:8080"</span><span class="token punctuation">,</span></span>
<span class="line">    TimeoutSeconds<span class="token punctuation">:</span> <span class="token number">30</span><span class="token punctuation">,</span></span>
<span class="line">    Insecure<span class="token punctuation">:</span>       <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    Reconnect<span class="token punctuation">:</span> <span class="token operator">&amp;</span>croupier<span class="token punctuation">.</span>ReconnectConfig<span class="token punctuation">{</span></span>
<span class="line">        Enabled<span class="token punctuation">:</span>           <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">        MaxAttempts<span class="token punctuation">:</span>       <span class="token number">0</span><span class="token punctuation">,</span>        <span class="token comment">// 0 = 无限重试</span></span>
<span class="line">        InitialDelayMs<span class="token punctuation">:</span>    <span class="token number">1000</span><span class="token punctuation">,</span>     <span class="token comment">// 初始延迟 1 秒</span></span>
<span class="line">        MaxDelayMs<span class="token punctuation">:</span>        <span class="token number">30000</span><span class="token punctuation">,</span>    <span class="token comment">// 最大延迟 30 秒</span></span>
<span class="line">        BackoffMultiplier<span class="token punctuation">:</span> <span class="token number">2.0</span><span class="token punctuation">,</span>      <span class="token comment">// 指数退避倍数</span></span>
<span class="line">        JitterFactor<span class="token punctuation">:</span>      <span class="token number">0.2</span><span class="token punctuation">,</span>      <span class="token comment">// 抖动因子</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">invoker <span class="token operator">:=</span> croupier<span class="token punctuation">.</span><span class="token function">NewInvoker</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span></span>
<span class="line">err <span class="token operator">:=</span> invoker<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span>ctx<span class="token punctuation">)</span></span>
<span class="line"><span class="token comment">// 连接断开会自动重连</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 或使用默认配置</span></span>
<span class="line">invoker <span class="token operator">:=</span> croupier<span class="token punctuation">.</span><span class="token function">NewInvoker</span><span class="token punctuation">(</span><span class="token boolean">nil</span><span class="token punctuation">)</span>  <span class="token comment">// 使用默认配置（包含自动重连）</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="javascript-typescript-sdk" tabindex="-1"><a class="header-anchor" href="#javascript-typescript-sdk"><span>JavaScript/TypeScript SDK</span></a></h2>
<p><strong>位置:</strong> <code v-pre>sdks/js/</code></p>
<blockquote>
<p>✅ <strong>更新:</strong> 自动重连和重试机制已于 2025-01-06 完成实现</p>
</blockquote>
<h3 id="已实现功能-1" tabindex="-1"><a class="header-anchor" href="#已实现功能-1"><span>已实现功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th style="text-align:center">状态</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Invoker 接口</strong></td>
<td style="text-align:center">✅</td>
<td>完整实现</td>
</tr>
<tr>
<td><strong>幂等性支持</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>InvokeOptions</code> 包含 <code v-pre>idempotencyKey</code></td>
</tr>
<tr>
<td><strong>流式事件</strong></td>
<td style="text-align:center">✅</td>
<td>使用 AsyncGenerator 流式获取作业事件</td>
</tr>
<tr>
<td><strong>作业取消</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>cancelJob()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 设置</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>setSchema()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 验证</strong></td>
<td style="text-align:center">✅</td>
<td>完整的 JSON Schema 验证</td>
</tr>
<tr>
<td><strong>自动重连</strong></td>
<td style="text-align:center">✅</td>
<td>指数退避 + 抖动重连策略</td>
</tr>
<tr>
<td><strong>重试机制</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>executeWithRetry</code> + 指数退避 + 抖动</td>
</tr>
</tbody>
</table>
<h3 id="缺失功能-1" tabindex="-1"><a class="header-anchor" href="#缺失功能-1"><span>缺失功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>优先级</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>文件传输功能</strong></td>
<td>低</td>
<td>未实现</td>
</tr>
</tbody>
</table>
<h3 id="代码示例-重试机制-1" tabindex="-1"><a class="header-anchor" href="#代码示例-重试机制-1"><span>代码示例 - 重试机制</span></a></h3>
<div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre v-pre><code class="language-typescript"><span class="line"><span class="token comment">// 重试机制已实现</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">{</span> Invoker<span class="token punctuation">,</span> InvokerConfig<span class="token punctuation">,</span> RetryConfig <span class="token punctuation">}</span> <span class="token keyword">from</span> <span class="token string">'@croupier/sdk'</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 配置重试策略</span></span>
<span class="line"><span class="token keyword">const</span> invoker <span class="token operator">=</span> <span class="token keyword">new</span> <span class="token class-name">Invoker</span><span class="token punctuation">(</span><span class="token punctuation">{</span></span>
<span class="line">  address<span class="token operator">:</span> <span class="token string">'127.0.0.1:8080'</span><span class="token punctuation">,</span></span>
<span class="line">  retry<span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    enabled<span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    maxAttempts<span class="token operator">:</span> <span class="token number">3</span><span class="token punctuation">,</span>           <span class="token comment">// 最多重试 3 次</span></span>
<span class="line">    initialDelayMs<span class="token operator">:</span> <span class="token number">100</span><span class="token punctuation">,</span>      <span class="token comment">// 初始延迟 100ms</span></span>
<span class="line">    maxDelayMs<span class="token operator">:</span> <span class="token number">5000</span><span class="token punctuation">,</span>         <span class="token comment">// 最大延迟 5 秒</span></span>
<span class="line">    backoffMultiplier<span class="token operator">:</span> <span class="token number">2</span><span class="token punctuation">,</span>     <span class="token comment">// 指数退避倍数</span></span>
<span class="line">    jitterFactor<span class="token operator">:</span> <span class="token number">0.1</span><span class="token punctuation">,</span>        <span class="token comment">// 抖动因子</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Invoke 会在失败时自动重试</span></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">'player.ban'</span><span class="token punctuation">,</span> payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 单次调用可覆盖重试配置</span></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">'player.ban'</span><span class="token punctuation">,</span> payload<span class="token punctuation">,</span> <span class="token punctuation">{</span></span>
<span class="line">  retry<span class="token operator">:</span> <span class="token punctuation">{</span> enabled<span class="token operator">:</span> <span class="token boolean">false</span> <span class="token punctuation">}</span><span class="token punctuation">,</span>  <span class="token comment">// 本次调用不重试</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="需要改进-1" tabindex="-1"><a class="header-anchor" href="#需要改进-1"><span>需要改进</span></a></h3>
<table>
<thead>
<tr>
<th>项目</th>
<th>当前状态</th>
<th>建议改进</th>
</tr>
</thead>
<tbody>
<tr>
<td>测试覆盖</td>
<td>基础测试</td>
<td>需要增加更多单元测试</td>
</tr>
<tr>
<td>错误处理</td>
<td>基础</td>
<td>需要更详细的错误分类</td>
</tr>
<tr>
<td>文档示例</td>
<td>较少</td>
<td>需要更多使用场景示例</td>
</tr>
</tbody>
</table>
<h3 id="代码示例-schema-验证" tabindex="-1"><a class="header-anchor" href="#代码示例-schema-验证"><span>代码示例 - Schema 验证</span></a></h3>
<div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre v-pre><code class="language-typescript"><span class="line"><span class="token comment">// Schema 验证已实现</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">{</span> Invoker <span class="token punctuation">}</span> <span class="token keyword">from</span> <span class="token string">'@croupier/sdk'</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">const</span> invoker <span class="token operator">=</span> <span class="token keyword">new</span> <span class="token class-name">Invoker</span><span class="token punctuation">(</span><span class="token punctuation">{</span> address<span class="token operator">:</span> <span class="token string">'127.0.0.1:8080'</span> <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 设置 Schema</span></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span><span class="token function">setSchema</span><span class="token punctuation">(</span><span class="token string">'player.ban'</span><span class="token punctuation">,</span> <span class="token punctuation">{</span></span>
<span class="line">  type<span class="token operator">:</span> <span class="token string">'object'</span><span class="token punctuation">,</span></span>
<span class="line">  properties<span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    player_id<span class="token operator">:</span> <span class="token punctuation">{</span> type<span class="token operator">:</span> <span class="token string">'string'</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    reason<span class="token operator">:</span> <span class="token punctuation">{</span> type<span class="token operator">:</span> <span class="token string">'string'</span> <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  required<span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">'player_id'</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Invoke 会自动验证 payload</span></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">'player.ban'</span><span class="token punctuation">,</span> <span class="token string">'{"player_id": "123"}'</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token comment">// 如果 payload 缺少 player_id，会抛出验证错误</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="代码示例-自动重连配置-1" tabindex="-1"><a class="header-anchor" href="#代码示例-自动重连配置-1"><span>代码示例 - 自动重连配置</span></a></h3>
<div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre v-pre><code class="language-typescript"><span class="line"><span class="token comment">// 自动重连已实现</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">{</span> Invoker<span class="token punctuation">,</span> InvokerConfig <span class="token punctuation">}</span> <span class="token keyword">from</span> <span class="token string">'@croupier/sdk'</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 配置重连策略</span></span>
<span class="line"><span class="token keyword">const</span> invoker <span class="token operator">=</span> <span class="token keyword">new</span> <span class="token class-name">Invoker</span><span class="token punctuation">(</span><span class="token punctuation">{</span></span>
<span class="line">  address<span class="token operator">:</span> <span class="token string">'127.0.0.1:8080'</span><span class="token punctuation">,</span></span>
<span class="line">  reconnect<span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    enabled<span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    maxAttempts<span class="token operator">:</span> <span class="token number">0</span><span class="token punctuation">,</span>        <span class="token comment">// 0 = 无限重试</span></span>
<span class="line">    initialDelayMs<span class="token operator">:</span> <span class="token number">1000</span><span class="token punctuation">,</span>  <span class="token comment">// 初始延迟 1 秒</span></span>
<span class="line">    maxDelayMs<span class="token operator">:</span> <span class="token number">30000</span><span class="token punctuation">,</span>     <span class="token comment">// 最大延迟 30 秒</span></span>
<span class="line">    backoffMultiplier<span class="token operator">:</span> <span class="token number">2</span><span class="token punctuation">,</span>  <span class="token comment">// 指数退避倍数</span></span>
<span class="line">    jitterFactor<span class="token operator">:</span> <span class="token number">0.2</span><span class="token punctuation">,</span>     <span class="token comment">// 抖动因子</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token comment">// 连接断开时会自动重连</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="python-sdk" tabindex="-1"><a class="header-anchor" href="#python-sdk"><span>Python SDK</span></a></h2>
<p><strong>位置:</strong> <code v-pre>sdks/python/</code></p>
<blockquote>
<p>✅ <strong>更新:</strong> Schema 验证和重试机制已于 2025-01-06 完成实现</p>
</blockquote>
<h3 id="已实现功能-2" tabindex="-1"><a class="header-anchor" href="#已实现功能-2"><span>已实现功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th style="text-align:center">状态</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Invoker 接口</strong></td>
<td style="text-align:center">✅</td>
<td>完整实现，支持同步和异步调用</td>
</tr>
<tr>
<td><strong>幂等性支持</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>InvokeOptions</code> 包含 <code v-pre>idempotency_key</code></td>
</tr>
<tr>
<td><strong>流式事件</strong></td>
<td style="text-align:center">✅</td>
<td>使用 <code v-pre>async for</code> 流式获取作业事件</td>
</tr>
<tr>
<td><strong>作业取消</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>cancel_job()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 设置</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>set_schema()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 验证</strong></td>
<td style="text-align:center">✅</td>
<td>完整的 JSON Schema 验证</td>
</tr>
<tr>
<td><strong>自动重连</strong></td>
<td style="text-align:center">✅</td>
<td>指数退避 + 抖动重连策略</td>
</tr>
<tr>
<td><strong>重试机制</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>_execute_with_retry</code> + 指数退避 + 抖动</td>
</tr>
</tbody>
</table>
<h3 id="缺失功能-2" tabindex="-1"><a class="header-anchor" href="#缺失功能-2"><span>缺失功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>优先级</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>文件传输功能</strong></td>
<td>低</td>
<td>未实现</td>
</tr>
</tbody>
</table>
<h3 id="代码示例-重试机制-2" tabindex="-1"><a class="header-anchor" href="#代码示例-重试机制-2"><span>代码示例 - 重试机制</span></a></h3>
<div class="language-python line-numbers-mode" data-highlighter="prismjs" data-ext="py"><pre v-pre><code class="language-python"><span class="line"><span class="token comment"># 重试机制已实现</span></span>
<span class="line"><span class="token keyword">from</span> croupier <span class="token keyword">import</span> Invoker<span class="token punctuation">,</span> InvokerConfig<span class="token punctuation">,</span> RetryConfig</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 创建 Invoker（带重试配置）</span></span>
<span class="line">invoker <span class="token operator">=</span> Invoker<span class="token punctuation">(</span>InvokerConfig<span class="token punctuation">(</span></span>
<span class="line">    address<span class="token operator">=</span><span class="token string">"127.0.0.1:8080"</span><span class="token punctuation">,</span></span>
<span class="line">    retry<span class="token operator">=</span>RetryConfig<span class="token punctuation">(</span></span>
<span class="line">        enabled<span class="token operator">=</span><span class="token boolean">True</span><span class="token punctuation">,</span></span>
<span class="line">        max_attempts<span class="token operator">=</span><span class="token number">3</span><span class="token punctuation">,</span>          <span class="token comment"># 最多重试 3 次</span></span>
<span class="line">        initial_delay_ms<span class="token operator">=</span><span class="token number">100</span><span class="token punctuation">,</span>    <span class="token comment"># 初始延迟 100ms</span></span>
<span class="line">        max_delay_ms<span class="token operator">=</span><span class="token number">5000</span><span class="token punctuation">,</span>       <span class="token comment"># 最大延迟 5 秒</span></span>
<span class="line">        backoff_multiplier<span class="token operator">=</span><span class="token number">2.0</span><span class="token punctuation">,</span>  <span class="token comment"># 指数退避倍数</span></span>
<span class="line">        jitter_factor<span class="token operator">=</span><span class="token number">0.1</span><span class="token punctuation">,</span>       <span class="token comment"># 抖动因子</span></span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span>connect<span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Invoke 会在失败时自动重试</span></span>
<span class="line"><span class="token keyword">await</span> invoker<span class="token punctuation">.</span>invoke<span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="java-sdk" tabindex="-1"><a class="header-anchor" href="#java-sdk"><span>Java SDK</span></a></h2>
<p><strong>位置:</strong> <code v-pre>sdks/java/</code></p>
<blockquote>
<p>✅ <strong>更新:</strong> Schema 验证和重试机制已于 2025-01-06 完成实现</p>
</blockquote>
<h3 id="已实现功能-3" tabindex="-1"><a class="header-anchor" href="#已实现功能-3"><span>已实现功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th style="text-align:center">状态</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Invoker 接口</strong></td>
<td style="text-align:center">✅</td>
<td>完整实现，支持同步调用</td>
</tr>
<tr>
<td><strong>幂等性支持</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>InvokeOptions</code> 包含 <code v-pre>idempotencyKey</code></td>
</tr>
<tr>
<td><strong>流式作业事件</strong></td>
<td style="text-align:center">✅</td>
<td>使用 Reactive Streams Publisher</td>
</tr>
<tr>
<td><strong>作业取消</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>cancelJob()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 设置</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>setSchema()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 验证</strong></td>
<td style="text-align:center">✅</td>
<td>完整的 JSON Schema 验证</td>
</tr>
<tr>
<td><strong>异常处理</strong></td>
<td style="text-align:center">✅</td>
<td>完整的 <code v-pre>InvokerException</code> 和 <code v-pre>ErrorCode</code></td>
</tr>
<tr>
<td><strong>自动重连</strong></td>
<td style="text-align:center">✅</td>
<td>指数退避 + 抖动重连策略</td>
</tr>
<tr>
<td><strong>重试机制</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>executeWithRetry</code> + 指数退避 + 抖动</td>
</tr>
</tbody>
</table>
<h3 id="缺失功能-3" tabindex="-1"><a class="header-anchor" href="#缺失功能-3"><span>缺失功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>优先级</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>文件传输功能</strong></td>
<td>低</td>
<td>未实现</td>
</tr>
</tbody>
</table>
<h3 id="代码示例-重试机制-3" tabindex="-1"><a class="header-anchor" href="#代码示例-重试机制-3"><span>代码示例 - 重试机制</span></a></h3>
<div class="language-java line-numbers-mode" data-highlighter="prismjs" data-ext="java"><pre v-pre><code class="language-java"><span class="line"><span class="token comment">// 重试机制已实现</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token import"><span class="token namespace">io<span class="token punctuation">.</span>github<span class="token punctuation">.</span>cuihairu<span class="token punctuation">.</span>croupier<span class="token punctuation">.</span>sdk<span class="token punctuation">.</span></span><span class="token operator">*</span></span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token import"><span class="token namespace">io<span class="token punctuation">.</span>github<span class="token punctuation">.</span>cuihairu<span class="token punctuation">.</span>croupier<span class="token punctuation">.</span>sdk<span class="token punctuation">.</span>invoker<span class="token punctuation">.</span></span><span class="token operator">*</span></span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 创建 Invoker（带重试配置）</span></span>
<span class="line"><span class="token class-name">Invoker</span> invoker <span class="token operator">=</span> <span class="token class-name">CroupierSDK</span><span class="token punctuation">.</span><span class="token function">createInvoker</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token class-name">InvokerConfig</span><span class="token punctuation">.</span><span class="token function">builder</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">address</span><span class="token punctuation">(</span><span class="token string">"127.0.0.1:8080"</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">insecure</span><span class="token punctuation">(</span><span class="token boolean">true</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">retry</span><span class="token punctuation">(</span><span class="token class-name">RetryConfig</span><span class="token punctuation">.</span><span class="token function">builder</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">enabled</span><span class="token punctuation">(</span><span class="token boolean">true</span><span class="token punctuation">)</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">maxAttempts</span><span class="token punctuation">(</span><span class="token number">3</span><span class="token punctuation">)</span>          <span class="token comment">// 最多重试 3 次</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">initialDelayMs</span><span class="token punctuation">(</span><span class="token number">100</span><span class="token punctuation">)</span>      <span class="token comment">// 初始延迟 100ms</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">maxDelayMs</span><span class="token punctuation">(</span><span class="token number">5000</span><span class="token punctuation">)</span>         <span class="token comment">// 最大延迟 5 秒</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">backoffMultiplier</span><span class="token punctuation">(</span><span class="token number">2.0</span><span class="token punctuation">)</span>   <span class="token comment">// 指数退避倍数</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">jitterFactor</span><span class="token punctuation">(</span><span class="token number">0.1</span><span class="token punctuation">)</span>        <span class="token comment">// 抖动因子</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">build</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">build</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">invoker<span class="token punctuation">.</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Invoke 会在失败时自动重试</span></span>
<span class="line"><span class="token class-name">String</span> result <span class="token operator">=</span> invoker<span class="token punctuation">.</span><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="c-sdk" tabindex="-1"><a class="header-anchor" href="#c-sdk"><span>C++ SDK</span></a></h2>
<p><strong>位置:</strong> <code v-pre>sdks/cpp/</code></p>
<blockquote>
<p>✅ <strong>更新:</strong> 重试机制已于 2025-01-06 完成实现</p>
</blockquote>
<h3 id="已实现功能-4" tabindex="-1"><a class="header-anchor" href="#已实现功能-4"><span>已实现功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th style="text-align:center">状态</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Invoker 接口</strong></td>
<td style="text-align:center">✅</td>
<td>完整实现</td>
</tr>
<tr>
<td><strong>幂等性支持</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>InvokeOptions</code> 包含 <code v-pre>idempotency_key</code></td>
</tr>
<tr>
<td><strong>流式作业事件</strong></td>
<td style="text-align:center">✅</td>
<td>使用 <code v-pre>std::future</code> 和 <code v-pre>std::vector&lt;JobEvent&gt;</code></td>
</tr>
<tr>
<td><strong>作业取消</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>CancelJob()</code> 方法</td>
</tr>
<tr>
<td><strong>Schema 设置</strong></td>
<td style="text-align:center">✅</td>
<td>支持 <code v-pre>SetSchema()</code> 方法</td>
</tr>
<tr>
<td><strong>自动重连</strong></td>
<td style="text-align:center">✅</td>
<td>指数退避 + 抖动重连策略</td>
</tr>
<tr>
<td><strong>重试机制</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>RetryConfig</code> + <code v-pre>SetRetryConfig()</code> + 指数退避</td>
</tr>
<tr>
<td><strong>可配置日志</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>disable_logging</code>/<code v-pre>debug_logging</code> 配置</td>
</tr>
<tr>
<td><strong>Token 脱敏</strong></td>
<td style="text-align:center">✅</td>
<td><code v-pre>MaskSensitive()</code> + <code v-pre>LogMasked()</code></td>
</tr>
</tbody>
</table>
<h3 id="缺失功能-4" tabindex="-1"><a class="header-anchor" href="#缺失功能-4"><span>缺失功能</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>优先级</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>文件传输功能</strong></td>
<td>低</td>
<td>虽有基础设施，但未实现（所有 SDK 都未实现）</td>
</tr>
</tbody>
</table>
<h3 id="需要改进-2" tabindex="-1"><a class="header-anchor" href="#需要改进-2"><span>需要改进</span></a></h3>
<table>
<thead>
<tr>
<th>项目</th>
<th>当前状态</th>
<th>建议改进</th>
</tr>
</thead>
<tbody>
<tr>
<td>文档</td>
<td>详细</td>
<td>可读性强，保持</td>
</tr>
<tr>
<td>构建系统</td>
<td>CMake</td>
<td>已完善</td>
</tr>
<tr>
<td>示例代码</td>
<td>丰富</td>
<td>已完善</td>
</tr>
</tbody>
</table>
<h3 id="代码示例-重试机制-4" tabindex="-1"><a class="header-anchor" href="#代码示例-重试机制-4"><span>代码示例 - 重试机制</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 重试机制已实现</span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 创建 Invoker</span></span>
<span class="line">CroupierInvoker <span class="token function">invoker</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 配置重试策略</span></span>
<span class="line">RetryConfig retry_config<span class="token punctuation">;</span></span>
<span class="line">retry_config<span class="token punctuation">.</span>enabled <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line">retry_config<span class="token punctuation">.</span>max_attempts <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>           <span class="token comment">// 最多重试 3 次</span></span>
<span class="line">retry_config<span class="token punctuation">.</span>initial_delay_ms <span class="token operator">=</span> <span class="token number">100</span><span class="token punctuation">;</span>     <span class="token comment">// 初始延迟 100ms</span></span>
<span class="line">retry_config<span class="token punctuation">.</span>max_delay_ms <span class="token operator">=</span> <span class="token number">5000</span><span class="token punctuation">;</span>        <span class="token comment">// 最大延迟 5 秒</span></span>
<span class="line">retry_config<span class="token punctuation">.</span>backoff_multiplier <span class="token operator">=</span> <span class="token number">2.0</span><span class="token punctuation">;</span>   <span class="token comment">// 指数退避</span></span>
<span class="line">retry_config<span class="token punctuation">.</span>jitter_factor <span class="token operator">=</span> <span class="token number">0.1</span><span class="token punctuation">;</span>        <span class="token comment">// 抖动因子</span></span>
<span class="line"></span>
<span class="line">invoker<span class="token punctuation">.</span><span class="token function">SetRetryConfig</span><span class="token punctuation">(</span>retry_config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Invoke 会在失败时自动重试</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> invoker<span class="token punctuation">.</span><span class="token function">Invoke</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="代码示例-可配置日志" tabindex="-1"><a class="header-anchor" href="#代码示例-可配置日志"><span>代码示例 - 可配置日志</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 可配置日志已实现</span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>agent_addr <span class="token operator">=</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 禁用日志</span></span>
<span class="line">config<span class="token punctuation">.</span>disable_logging <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 或启用调试日志</span></span>
<span class="line">config<span class="token punctuation">.</span>debug_logging <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="优势功能" tabindex="-1"><a class="header-anchor" href="#优势功能"><span>优势功能</span></a></h3>
<p>C++ SDK 独有的高级功能（其他 SDK 可参考）：</p>
<table>
<thead>
<tr>
<th>功能</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>虚拟对象系统</strong></td>
<td><code v-pre>VirtualObjectDescriptor</code> 支持对象关系</td>
</tr>
<tr>
<td><strong>组件系统</strong></td>
<td><code v-pre>ComponentDescriptor</code> 支持组件依赖管理</td>
</tr>
<tr>
<td><strong>模板生成</strong></td>
<td>支持从模板生成描述符</td>
</tr>
<tr>
<td><strong>工具函数库</strong></td>
<td><code v-pre>utils</code> 命名空间提供丰富的辅助函数</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="跨-sdk-统一改进项" tabindex="-1"><a class="header-anchor" href="#跨-sdk-统一改进项"><span>跨 SDK 统一改进项</span></a></h2>
<h3 id="_1-高优先级-必须实现" tabindex="-1"><a class="header-anchor" href="#_1-高优先级-必须实现"><span>1. 高优先级（必须实现）</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>说明</th>
<th>影响 SDK</th>
</tr>
</thead>
<tbody>
<tr>
<td><s><strong>Invoker 接口</strong></s></td>
<td><s>Python 和 Java SDK 必须实现</s></td>
<td>✅ 已完成</td>
</tr>
<tr>
<td><s><strong>自动重连</strong></s></td>
<td><s>连接断开后自动重连（指数退避）</s></td>
<td>✅ 所有 SDK 已完成</td>
</tr>
</tbody>
</table>
<h3 id="_2-中优先级-建议实现" tabindex="-1"><a class="header-anchor" href="#_2-中优先级-建议实现"><span>2. 中优先级（建议实现）</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>说明</th>
<th>影响 SDK</th>
</tr>
</thead>
<tbody>
<tr>
<td><s><strong>JSON Schema 验证</strong></s></td>
<td><s>Invoker 端验证请求 payload</s></td>
<td>✅ 所有 SDK 已完成</td>
</tr>
<tr>
<td><s><strong>重试机制</strong></s></td>
<td><s>可配置的重试策略</s></td>
<td>✅ 所有 SDK 已完成</td>
</tr>
<tr>
<td><strong>集成测试</strong></td>
<td>端到端测试覆盖</td>
<td>JS, Python, Java</td>
</tr>
</tbody>
</table>
<h3 id="_3-低优先级-可选实现" tabindex="-1"><a class="header-anchor" href="#_3-低优先级-可选实现"><span>3. 低优先级（可选实现）</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>说明</th>
<th>影响 SDK</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>文件传输</strong></td>
<td>支持上传文件到服务器</td>
<td>所有 SDK</td>
</tr>
<tr>
<td><strong>高级重试</strong></td>
<td>断路器、舱壁模式</td>
<td>所有 SDK</td>
</tr>
<tr>
<td><strong>指标收集</strong></td>
<td>内置 Prometheus 指标</td>
<td>所有 SDK</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="实现优先级路线图" tabindex="-1"><a class="header-anchor" href="#实现优先级路线图"><span>实现优先级路线图</span></a></h2>
<h3 id="phase-1-核心功能补全-必须-✅-已完成" tabindex="-1"><a class="header-anchor" href="#phase-1-核心功能补全-必须-✅-已完成"><span>Phase 1: 核心功能补全（必须）✅ 已完成</span></a></h3>
<ol>
<li><s><strong>Python SDK</strong> - 实现 Invoker 接口</s> ✅ 已完成</li>
<li><s><strong>Java SDK</strong> - 实现 Invoker 接口</s> ✅ 已完成</li>
<li><s><strong>所有 SDK</strong> - 添加幂等性支持</s> ✅ 已完成</li>
<li><s><strong>JS SDK</strong> - 实现自动重连（指数退避）</s> ✅ 已完成</li>
<li><s><strong>Go SDK</strong> - JSON Schema 验证</s> ✅ 已确认实现</li>
<li><s><strong>Python SDK</strong> - 实现自动重连</s> ✅ 已完成</li>
<li><s><strong>Java SDK</strong> - 实现自动重连</s> ✅ 已完成</li>
<li><s><strong>Go SDK</strong> - 实现自动重连</s> ✅ 已完成</li>
<li><s><strong>C++ SDK</strong> - 实现自动重连</s> ✅ 已完成</li>
<li><s><strong>所有 SDK</strong> - JSON Schema 验证</s> ✅ 已完成</li>
</ol>
<h3 id="phase-2-功能增强-建议" tabindex="-1"><a class="header-anchor" href="#phase-2-功能增强-建议"><span>Phase 2: 功能增强（建议）</span></a></h3>
<ol>
<li><s><strong>Go SDK</strong> - JSON Schema 验证</s> ✅ 已完成</li>
<li><s><strong>所有 SDK</strong> - JSON Schema 验证</s> ✅ 已完成</li>
<li><s><strong>所有 SDK</strong> - 重试机制（调用失败重试）</s> ✅ 已完成</li>
<li><s><strong>Python/Java SDK</strong> - 实现自动重连</s> ✅ 已完成</li>
<li><s><strong>Go SDK</strong> - 实现自动重连</s> ✅ 已完成</li>
<li><s><strong>C++ SDK</strong> - 实现自动重连</s> ✅ 已完成</li>
<li><strong>JS SDK</strong> - 增加测试覆盖率</li>
<li><strong>集成测试</strong> - 端到端测试覆盖</li>
</ol>
<h3 id="phase-3-高级特性-可选" tabindex="-1"><a class="header-anchor" href="#phase-3-高级特性-可选"><span>Phase 3: 高级特性（可选）</span></a></h3>
<ol>
<li><strong>所有 SDK</strong> - 文件传输功能</li>
<li><strong>所有 SDK</strong> - 高级重试策略</li>
<li><strong>C++ 以外的 SDK</strong> - 虚拟对象系统</li>
<li><strong>所有 SDK</strong> - 内置指标收集</li>
</ol>
<hr>
<h2 id="功能完成度评分" tabindex="-1"><a class="header-anchor" href="#功能完成度评分"><span>功能完成度评分</span></a></h2>
<table>
<thead>
<tr>
<th>SDK</th>
<th style="text-align:center">核心功能</th>
<th style="text-align:center">高级功能</th>
<th style="text-align:center">代码质量</th>
<th style="text-align:center">总体评分</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>C++</strong></td>
<td style="text-align:center">100%</td>
<td style="text-align:center">98%</td>
<td style="text-align:center">95%</td>
<td style="text-align:center"><strong>98%</strong></td>
</tr>
<tr>
<td><strong>Go</strong></td>
<td style="text-align:center">100%</td>
<td style="text-align:center">95%</td>
<td style="text-align:center">90%</td>
<td style="text-align:center"><strong>95%</strong></td>
</tr>
<tr>
<td><strong>Java</strong></td>
<td style="text-align:center">98%</td>
<td style="text-align:center">92%</td>
<td style="text-align:center">85%</td>
<td style="text-align:center"><strong>92%</strong> ⬆️</td>
</tr>
<tr>
<td><strong>Python</strong></td>
<td style="text-align:center">95%</td>
<td style="text-align:center">85%</td>
<td style="text-align:center">80%</td>
<td style="text-align:center"><strong>87%</strong> ⬆️</td>
</tr>
<tr>
<td><strong>JS</strong></td>
<td style="text-align:center">98%</td>
<td style="text-align:center">85%</td>
<td style="text-align:center">75%</td>
<td style="text-align:center"><strong>86%</strong> ⬆️</td>
</tr>
</tbody>
</table>
<blockquote>
<p>⬆️ 表示评分因功能完成而提升（Java, Python, JS: 重试机制）</p>
</blockquote>
<h3 id="评分标准" tabindex="-1"><a class="header-anchor" href="#评分标准"><span>评分标准</span></a></h3>
<ul>
<li><strong>核心功能</strong>: Client/Invoker 接口、函数注册、连接管理</li>
<li><strong>高级功能</strong>: 作业管理、幂等性、重试、TLS、自动重连</li>
<li><strong>代码质量</strong>: 测试覆盖、文档完整性、示例代码</li>
</ul>
<hr>
<h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/sdk/specification.html">SDK 行为规范</RouteLink> - 所有 SDK 必须遵守的行为规范</li>
<li><RouteLink to="/sdks/SDK_REGISTRATION_FLOW.html">函数注册流程</RouteLink> - 函数注册的详细流程</li>
<li><RouteLink to="/sdks/go/examples/">Go SDK 示例</RouteLink> - Go SDK 使用示例</li>
<li><RouteLink to="/sdks/js/examples/">JS SDK 示例</RouteLink> - JS SDK 使用示例</li>
</ul>
</div></template>


