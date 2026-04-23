<template><div><h1 id="grpc-api" tabindex="-1"><a class="header-anchor" href="#grpc-api"><span>gRPC API</span></a></h1>
<p>Croupier 使用 gRPC 作为服务间通信协议，提供高性能、类型安全的 API。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#服务列表">服务列表</router-link></li><li><router-link to="#连接配置">连接配置</router-link><ul><li><router-link to="#mtls-配置">mTLS 配置</router-link></li><li><router-link to="#客户端配置">客户端配置</router-link></li></ul></li><li><router-link to="#controlservice">ControlService</router-link><ul><li><router-link to="#registeragent">RegisterAgent</router-link></li><li><router-link to="#heartbeat">Heartbeat</router-link></li><li><router-link to="#getassignments">GetAssignments</router-link></li></ul></li><li><router-link to="#functionservice">FunctionService</router-link><ul><li><router-link to="#invokefunction">InvokeFunction</router-link></li><li><router-link to="#invokejob">InvokeJob</router-link></li><li><router-link to="#streamjobevents">StreamJobEvents</router-link></li><li><router-link to="#canceljob">CancelJob</router-link></li><li><router-link to="#registerfunction">RegisterFunction</router-link></li></ul></li><li><router-link to="#registryservice">RegistryService</router-link><ul><li><router-link to="#getregistrations">GetRegistrations</router-link></li><li><router-link to="#watchregistrations">WatchRegistrations</router-link></li></ul></li><li><router-link to="#错误处理">错误处理</router-link><ul><li><router-link to="#grpc-状态码">gRPC 状态码</router-link></li><li><router-link to="#错误详情">错误详情</router-link></li></ul></li><li><router-link to="#客户端示例">客户端示例</router-link><ul><li><router-link to="#go-客户端">Go 客户端</router-link></li><li><router-link to="#c-客户端">C++ 客户端</router-link></li></ul></li><li><router-link to="#拦截器">拦截器</router-link><ul><li><router-link to="#日志拦截器">日志拦截器</router-link></li><li><router-link to="#重试拦截器">重试拦截器</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="服务列表" tabindex="-1"><a class="header-anchor" href="#服务列表"><span>服务列表</span></a></h2>
<table>
<thead>
<tr>
<th>服务</th>
<th>端口</th>
<th>用途</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>ControlService</code></td>
<td>8443</td>
<td>Agent 注册与连接管理</td>
</tr>
<tr>
<td><code v-pre>FunctionService</code></td>
<td>8443</td>
<td>函数调用与作业管理</td>
</tr>
<tr>
<td><code v-pre>RegistryService</code></td>
<td>8443</td>
<td>函数注册与发现</td>
</tr>
</tbody>
</table>
<h2 id="连接配置" tabindex="-1"><a class="header-anchor" href="#连接配置"><span>连接配置</span></a></h2>
<h3 id="mtls-配置" tabindex="-1"><a class="header-anchor" href="#mtls-配置"><span>mTLS 配置</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line">creds<span class="token punctuation">,</span> err <span class="token operator">:=</span> credentials<span class="token punctuation">.</span><span class="token function">NewClientTLSFromFile</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"ca.crt"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"server.example.com"</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Fatal</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">conn<span class="token punctuation">,</span> err <span class="token operator">:=</span> grpc<span class="token punctuation">.</span><span class="token function">Dial</span><span class="token punctuation">(</span><span class="token string">"server.example.com:8443"</span><span class="token punctuation">,</span></span>
<span class="line">    grpc<span class="token punctuation">.</span><span class="token function">WithTransportCredentials</span><span class="token punctuation">(</span>creds<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="客户端配置" tabindex="-1"><a class="header-anchor" href="#客户端配置"><span>客户端配置</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"google.golang.org/grpc"</span></span>
<span class="line">    <span class="token string">"google.golang.org/grpc/credentials"</span></span>
<span class="line">    <span class="token string">"time"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 创建连接</span></span>
<span class="line">conn<span class="token punctuation">,</span> err <span class="token operator">:=</span> grpc<span class="token punctuation">.</span><span class="token function">Dial</span><span class="token punctuation">(</span><span class="token string">"server:8443"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// mTLS 凭证</span></span>
<span class="line">    grpc<span class="token punctuation">.</span><span class="token function">WithTransportCredentials</span><span class="token punctuation">(</span>credentials<span class="token punctuation">.</span><span class="token function">NewTLS</span><span class="token punctuation">(</span>tlsConfig<span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// 保持连接</span></span>
<span class="line">    grpc<span class="token punctuation">.</span><span class="token function">WithKeepaliveParams</span><span class="token punctuation">(</span>keepalive<span class="token punctuation">.</span>ClientParameters<span class="token punctuation">{</span></span>
<span class="line">        Time<span class="token punctuation">:</span>                <span class="token number">10</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Second<span class="token punctuation">,</span></span>
<span class="line">        Timeout<span class="token punctuation">:</span>             time<span class="token punctuation">.</span>Second<span class="token punctuation">,</span></span>
<span class="line">        PermitWithoutStream<span class="token punctuation">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// 消息大小限制</span></span>
<span class="line">    grpc<span class="token punctuation">.</span><span class="token function">WithDefaultCallOptions</span><span class="token punctuation">(</span></span>
<span class="line">        grpc<span class="token punctuation">.</span><span class="token function">MaxCallRecvMsgSize</span><span class="token punctuation">(</span><span class="token number">10</span> <span class="token operator">*</span> <span class="token number">1024</span> <span class="token operator">*</span> <span class="token number">1024</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        grpc<span class="token punctuation">.</span><span class="token function">MaxCallSendMsgSize</span><span class="token punctuation">(</span><span class="token number">10</span> <span class="token operator">*</span> <span class="token number">1024</span> <span class="token operator">*</span> <span class="token number">1024</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="controlservice" tabindex="-1"><a class="header-anchor" href="#controlservice"><span>ControlService</span></a></h2>
<p>Agent 注册与连接管理。</p>
<h3 id="registeragent" tabindex="-1"><a class="header-anchor" href="#registeragent"><span>RegisterAgent</span></a></h3>
<p>注册新 Agent 到 Server。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">RegisterAgent</span><span class="token punctuation">(</span><span class="token class-name">RegisterAgentRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">RegisterAgentResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterAgentRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>       <span class="token comment">// Agent 唯一标识</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>        <span class="token comment">// 游戏 ID</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>            <span class="token comment">// 环境</span></span>
<span class="line">  <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>        <span class="token comment">// Agent 版本</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token builtin">string</span> functions <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>  <span class="token comment">// 支持的函数列表</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> labels <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>  <span class="token comment">// 标签</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterAgentResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">bool</span> success <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">int64</span> heartbeat_interval <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// 心跳间隔（秒）</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="heartbeat" tabindex="-1"><a class="header-anchor" href="#heartbeat"><span>Heartbeat</span></a></h3>
<p>Agent 心跳保活。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">Heartbeat</span><span class="token punctuation">(</span><span class="token class-name">HeartbeatRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">HeartbeatResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">HeartbeatRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">int64</span> timestamp <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> status <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// Agent 状态</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">HeartbeatResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">bool</span> success <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="getassignments" tabindex="-1"><a class="header-anchor" href="#getassignments"><span>GetAssignments</span></a></h3>
<p>获取分配给 Agent 的配置。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">GetAssignments</span><span class="token punctuation">(</span><span class="token class-name">GetAssignmentsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">GetAssignmentsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">GetAssignmentsResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">FunctionAssignment</span> assignments <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">int64</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>  <span class="token comment">// 配置版本</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">FunctionAssignment</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> function_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">bool</span> enabled <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> config <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="functionservice" tabindex="-1"><a class="header-anchor" href="#functionservice"><span>FunctionService</span></a></h2>
<p>函数调用与作业管理。</p>
<h3 id="invokefunction" tabindex="-1"><a class="header-anchor" href="#invokefunction"><span>InvokeFunction</span></a></h3>
<p>同步调用函数。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">InvokeFunction</span><span class="token punctuation">(</span><span class="token class-name">InvokeFunctionRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">InvokeFunctionResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeFunctionRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> function_id <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> payload <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">InvokeOptions</span> options <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeOptions</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> idempotency_key <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// 幂等键</span></span>
<span class="line">  <span class="token builtin">int32</span> timeout <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>           <span class="token comment">// 超时（秒）</span></span>
<span class="line">  <span class="token builtin">string</span> routing_mode <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>     <span class="token comment">// 路由模式</span></span>
<span class="line">  <span class="token builtin">string</span> target_agent <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>     <span class="token comment">// 目标 Agent</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeFunctionResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">bool</span> success <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> result <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> error <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> error_code <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="invokejob" tabindex="-1"><a class="header-anchor" href="#invokejob"><span>InvokeJob</span></a></h3>
<p>异步调用函数（创建作业）。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">InvokeJob</span><span class="token punctuation">(</span><span class="token class-name">InvokeJobRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">InvokeJobResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeJobResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">bool</span> success <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> job_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> status <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// pending, running, completed, failed</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="streamjobevents" tabindex="-1"><a class="header-anchor" href="#streamjobevents"><span>StreamJobEvents</span></a></h3>
<p>流式获取作业事件。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">StreamJobEvents</span><span class="token punctuation">(</span><span class="token class-name">StreamJobEventsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token keyword">stream</span> <span class="token class-name">JobEvent</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>事件</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">JobEvent</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> job_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">EventType</span> type <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> message <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">double</span> progress <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>  <span class="token comment">// 0.0 - 1.0</span></span>
<span class="line">  <span class="token builtin">int64</span> timestamp <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> data <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>  <span class="token comment">// 附加数据</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">enum</span> <span class="token class-name">EventType</span> <span class="token punctuation">{</span></span>
<span class="line">  START <span class="token operator">=</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line">  PROGRESS <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  LOG <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  DONE <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  ERROR <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="canceljob" tabindex="-1"><a class="header-anchor" href="#canceljob"><span>CancelJob</span></a></h3>
<p>取消正在执行的作业。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">CancelJob</span><span class="token punctuation">(</span><span class="token class-name">CancelJobRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">CancelJobResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">CancelJobRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> job_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> reason <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="registerfunction" tabindex="-1"><a class="header-anchor" href="#registerfunction"><span>RegisterFunction</span></a></h3>
<p>注册函数到 Server。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">RegisterFunction</span><span class="token punctuation">(</span><span class="token class-name">RegisterFunctionRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">RegisterFunctionResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterFunctionRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">FunctionDescriptor</span> descriptor <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">FunctionDescriptor</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> name <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> category <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> params_schema <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> result_schema <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">AuthConfig</span> auth <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">Semantics</span> semantics <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">UIConfig</span> ui <span class="token operator">=</span> <span class="token number">8</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="registryservice" tabindex="-1"><a class="header-anchor" href="#registryservice"><span>RegistryService</span></a></h2>
<p>函数注册与发现。</p>
<h3 id="getregistrations" tabindex="-1"><a class="header-anchor" href="#getregistrations"><span>GetRegistrations</span></a></h3>
<p>获取函数注册信息。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">GetRegistrations</span><span class="token punctuation">(</span><span class="token class-name">GetRegistrationsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">GetRegistrationsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">GetRegistrationsRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> function_id <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// 可选，查询特定函数</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">GetRegistrationsResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">FunctionRegistration</span> registrations <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">FunctionRegistration</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> function_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">AgentInfo</span> agents <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>  <span class="token comment">// 提供该函数的 Agent</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">AgentInfo</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> addr <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">int64</span> last_heartbeat <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> labels <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="watchregistrations" tabindex="-1"><a class="header-anchor" href="#watchregistrations"><span>WatchRegistrations</span></a></h3>
<p>监听注册变化。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">rpc</span> <span class="token function">WatchRegistrations</span><span class="token punctuation">(</span><span class="token class-name">WatchRegistrationsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token keyword">stream</span> <span class="token class-name">RegistrationEvent</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>事件</strong>：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">RegistrationEvent</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token positional-class-name class-name">EventType</span> type <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// ADDED, UPDATED, REMOVED</span></span>
<span class="line">  <span class="token positional-class-name class-name">FunctionRegistration</span> registration <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="错误处理" tabindex="-1"><a class="header-anchor" href="#错误处理"><span>错误处理</span></a></h2>
<h3 id="grpc-状态码" tabindex="-1"><a class="header-anchor" href="#grpc-状态码"><span>gRPC 状态码</span></a></h3>
<table>
<thead>
<tr>
<th>状态码</th>
<th>说明</th>
<th>HTTP 映射</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>OK</code></td>
<td>成功</td>
<td>200</td>
</tr>
<tr>
<td><code v-pre>INVALID_ARGUMENT</code></td>
<td>参数错误</td>
<td>400</td>
</tr>
<tr>
<td><code v-pre>UNAUTHENTICATED</code></td>
<td>未认证</td>
<td>401</td>
</tr>
<tr>
<td><code v-pre>PERMISSION_DENIED</code></td>
<td>权限不足</td>
<td>403</td>
</tr>
<tr>
<td><code v-pre>NOT_FOUND</code></td>
<td>资源不存在</td>
<td>404</td>
</tr>
<tr>
<td><code v-pre>ALREADY_EXISTS</code></td>
<td>资源已存在</td>
<td>409</td>
</tr>
<tr>
<td><code v-pre>RESOURCE_EXHAUSTED</code></td>
<td>限流</td>
<td>429</td>
</tr>
<tr>
<td><code v-pre>INTERNAL</code></td>
<td>内部错误</td>
<td>500</td>
</tr>
<tr>
<td><code v-pre>UNAVAILABLE</code></td>
<td>服务不可用</td>
<td>503</td>
</tr>
</tbody>
</table>
<h3 id="错误详情" tabindex="-1"><a class="header-anchor" href="#错误详情"><span>错误详情</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">ErrorInfo</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> code <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>       <span class="token comment">// 错误码</span></span>
<span class="line">  <span class="token builtin">string</span> message <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>    <span class="token comment">// 错误消息</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> details <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// 错误详情</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="客户端示例" tabindex="-1"><a class="header-anchor" href="#客户端示例"><span>客户端示例</span></a></h2>
<h3 id="go-客户端" tabindex="-1"><a class="header-anchor" href="#go-客户端"><span>Go 客户端</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"log"</span></span>
<span class="line">    <span class="token string">"time"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"google.golang.org/grpc"</span></span>
<span class="line">    <span class="token string">"google.golang.org/grpc/credentials"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 创建连接</span></span>
<span class="line">    conn<span class="token punctuation">,</span> err <span class="token operator">:=</span> grpc<span class="token punctuation">.</span><span class="token function">Dial</span><span class="token punctuation">(</span><span class="token string">"server:8443"</span><span class="token punctuation">,</span></span>
<span class="line">        grpc<span class="token punctuation">.</span><span class="token function">WithTransportCredentials</span><span class="token punctuation">(</span>credentials<span class="token punctuation">.</span><span class="token function">NewTLS</span><span class="token punctuation">(</span>tlsConfig<span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Fatal</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">defer</span> conn<span class="token punctuation">.</span><span class="token function">Close</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 创建客户端</span></span>
<span class="line">    client <span class="token operator">:=</span> <span class="token function">NewFunctionServiceClient</span><span class="token punctuation">(</span>conn<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 调用函数</span></span>
<span class="line">    ctx<span class="token punctuation">,</span> cancel <span class="token operator">:=</span> context<span class="token punctuation">.</span><span class="token function">WithTimeout</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token number">30</span><span class="token operator">*</span>time<span class="token punctuation">.</span>Second<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> <span class="token function">cancel</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    resp<span class="token punctuation">,</span> err <span class="token operator">:=</span> client<span class="token punctuation">.</span><span class="token function">InvokeFunction</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>InvokeFunctionRequest<span class="token punctuation">{</span></span>
<span class="line">        GameId<span class="token punctuation">:</span>     <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">        Env<span class="token punctuation">:</span>        <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">        FunctionId<span class="token punctuation">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">        Payload<span class="token punctuation">:</span>    structpb<span class="token punctuation">.</span><span class="token function">NewStructValue</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        Options<span class="token punctuation">:</span> <span class="token operator">&amp;</span>InvokeOptions<span class="token punctuation">{</span></span>
<span class="line">            IdempotencyKey<span class="token punctuation">:</span> <span class="token string">"unique-key-123"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"调用失败: %v"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"调用成功: %+v"</span><span class="token punctuation">,</span> resp<span class="token punctuation">.</span>Result<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="c-客户端" tabindex="-1"><a class="header-anchor" href="#c-客户端"><span>C++ 客户端</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;croupier/sdk/client.h></span></span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;grpc/grpc.h></span></span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;grpc++/channel.h></span></span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 创建通道</span></span>
<span class="line">    <span class="token keyword">auto</span> tls_credentials <span class="token operator">=</span> grpc<span class="token double-colon punctuation">::</span><span class="token function">SslCredentials</span><span class="token punctuation">(</span></span>
<span class="line">        grpc<span class="token double-colon punctuation">::</span>SslCredentialsOptions<span class="token punctuation">{</span></span>
<span class="line">            <span class="token punctuation">.</span>pem_root_certs <span class="token operator">=</span> <span class="token function">ReadFile</span><span class="token punctuation">(</span><span class="token string">"ca.crt"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token punctuation">.</span>pem_cert_chain <span class="token operator">=</span> <span class="token function">ReadFile</span><span class="token punctuation">(</span><span class="token string">"client.crt"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token punctuation">.</span>pem_private_key <span class="token operator">=</span> <span class="token function">ReadFile</span><span class="token punctuation">(</span><span class="token string">"client.key"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">auto</span> channel <span class="token operator">=</span> grpc<span class="token double-colon punctuation">::</span><span class="token function">CreateChannel</span><span class="token punctuation">(</span></span>
<span class="line">        <span class="token string">"server:8443"</span><span class="token punctuation">,</span></span>
<span class="line">        tls_credentials</span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 创建客户端</span></span>
<span class="line">    croupier<span class="token double-colon punctuation">::</span>FunctionServiceClient <span class="token function">client</span><span class="token punctuation">(</span>channel<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 调用函数</span></span>
<span class="line">    grpc<span class="token double-colon punctuation">::</span>ClientContext context<span class="token punctuation">;</span></span>
<span class="line">    context<span class="token punctuation">.</span><span class="token function">set_deadline</span><span class="token punctuation">(</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>chrono<span class="token double-colon punctuation">::</span>system_clock<span class="token double-colon punctuation">::</span><span class="token function">now</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">+</span> std<span class="token double-colon punctuation">::</span>chrono<span class="token double-colon punctuation">::</span><span class="token function">seconds</span><span class="token punctuation">(</span><span class="token number">30</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    croupier<span class="token double-colon punctuation">::</span>InvokeFunctionRequest request<span class="token punctuation">;</span></span>
<span class="line">    request<span class="token punctuation">.</span><span class="token function">set_game_id</span><span class="token punctuation">(</span><span class="token string">"my-game"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    request<span class="token punctuation">.</span><span class="token function">set_env</span><span class="token punctuation">(</span><span class="token string">"prod"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    request<span class="token punctuation">.</span><span class="token function">set_function_id</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    request<span class="token punctuation">.</span><span class="token function">mutable_payload</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token operator">-></span><span class="token function">CopyFrom</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">auto</span> response <span class="token operator">=</span> client<span class="token punctuation">.</span><span class="token function">InvokeFunction</span><span class="token punctuation">(</span><span class="token operator">&amp;</span>context<span class="token punctuation">,</span> request<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>response<span class="token punctuation">.</span><span class="token function">ok</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"调用成功"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">else</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"调用失败: "</span> <span class="token operator">&lt;&lt;</span> response<span class="token punctuation">.</span><span class="token function">error_message</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="拦截器" tabindex="-1"><a class="header-anchor" href="#拦截器"><span>拦截器</span></a></h2>
<h3 id="日志拦截器" tabindex="-1"><a class="header-anchor" href="#日志拦截器"><span>日志拦截器</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">func</span> <span class="token function">loggingInterceptor</span><span class="token punctuation">(</span></span>
<span class="line">    ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span></span>
<span class="line">    method <span class="token builtin">string</span><span class="token punctuation">,</span></span>
<span class="line">    req<span class="token punctuation">,</span> reply <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    cc <span class="token operator">*</span>grpc<span class="token punctuation">.</span>ClientConn<span class="token punctuation">,</span></span>
<span class="line">    invoker grpc<span class="token punctuation">.</span>UnaryInvoker<span class="token punctuation">,</span></span>
<span class="line">    opts <span class="token operator">...</span>grpc<span class="token punctuation">.</span>CallOption<span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    start <span class="token operator">:=</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    err <span class="token operator">:=</span> <span class="token function">invoker</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> method<span class="token punctuation">,</span> req<span class="token punctuation">,</span> reply<span class="token punctuation">,</span> cc<span class="token punctuation">,</span> opts<span class="token operator">...</span><span class="token punctuation">)</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"调用 %s 耗时 %v"</span><span class="token punctuation">,</span> method<span class="token punctuation">,</span> time<span class="token punctuation">.</span><span class="token function">Since</span><span class="token punctuation">(</span>start<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> err</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="重试拦截器" tabindex="-1"><a class="header-anchor" href="#重试拦截器"><span>重试拦截器</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">func</span> <span class="token function">retryInterceptor</span><span class="token punctuation">(</span>maxRetries <span class="token builtin">int</span><span class="token punctuation">)</span> grpc<span class="token punctuation">.</span>UnaryClientInterceptor <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">func</span><span class="token punctuation">(</span></span>
<span class="line">        ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span></span>
<span class="line">        method <span class="token builtin">string</span><span class="token punctuation">,</span></span>
<span class="line">        req<span class="token punctuation">,</span> reply <span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        cc <span class="token operator">*</span>grpc<span class="token punctuation">.</span>ClientConn<span class="token punctuation">,</span></span>
<span class="line">        invoker grpc<span class="token punctuation">.</span>UnaryInvoker<span class="token punctuation">,</span></span>
<span class="line">        opts <span class="token operator">...</span>grpc<span class="token punctuation">.</span>CallOption<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">var</span> lastErr <span class="token builtin">error</span></span>
<span class="line">        <span class="token keyword">for</span> i <span class="token operator">:=</span> <span class="token number">0</span><span class="token punctuation">;</span> i <span class="token operator">&lt;</span> maxRetries<span class="token punctuation">;</span> i<span class="token operator">++</span> <span class="token punctuation">{</span></span>
<span class="line">            lastErr <span class="token operator">=</span> <span class="token function">invoker</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> method<span class="token punctuation">,</span> req<span class="token punctuation">,</span> reply<span class="token punctuation">,</span> cc<span class="token punctuation">,</span> opts<span class="token operator">...</span><span class="token punctuation">)</span></span>
<span class="line">            <span class="token keyword">if</span> lastErr <span class="token operator">==</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">                <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">            time<span class="token punctuation">.</span><span class="token function">Sleep</span><span class="token punctuation">(</span>time<span class="token punctuation">.</span>Second <span class="token operator">*</span> time<span class="token punctuation">.</span><span class="token function">Duration</span><span class="token punctuation">(</span>i<span class="token operator">+</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">return</span> lastErr</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/api/rest.html">REST API</RouteLink></li>
<li><RouteLink to="/api/proto-options.html">Proto 选项</RouteLink></li>
<li><RouteLink to="/api/">API 概览</RouteLink></li>
</ul>
</div></template>


