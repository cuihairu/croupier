<template><div><h1 id="api-概览" tabindex="-1"><a class="header-anchor" href="#api-概览"><span>API 概览</span></a></h1>
<p>Croupier 提供两套 API：<strong>gRPC API</strong>（服务间通信）和 <strong>REST API</strong>（Web 调用）。</p>
<h2 id="api-类型" tabindex="-1"><a class="header-anchor" href="#api-类型"><span>API 类型</span></a></h2>
<table>
<thead>
<tr>
<th>类型</th>
<th>协议</th>
<th>端口</th>
<th>用途</th>
</tr>
</thead>
<tbody>
<tr>
<td>gRPC</td>
<td>HTTP/2 + mTLS</td>
<td>8443</td>
<td>Server-Agent、Server-SDK 通信</td>
</tr>
<tr>
<td>REST</td>
<td>HTTP/HTTPS</td>
<td>8080</td>
<td>Dashboard、外部调用</td>
</tr>
</tbody>
</table>
<h2 id="grpc-api" tabindex="-1"><a class="header-anchor" href="#grpc-api"><span>gRPC API</span></a></h2>
<h3 id="基础配置" tabindex="-1"><a class="header-anchor" href="#基础配置"><span>基础配置</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// 连接选项</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"address"</span><span class="token punctuation">:</span> <span class="token string">"server.example.com:8443"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"tls"</span><span class="token punctuation">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token string">"ca_cert"</span><span class="token punctuation">:</span> <span class="token string">"path/to/ca.crt"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"client_cert"</span><span class="token punctuation">:</span> <span class="token string">"path/to/client.crt"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"client_key"</span><span class="token punctuation">:</span> <span class="token string">"path/to/client.key"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"server_name"</span><span class="token punctuation">:</span> <span class="token string">"server.example.com"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="服务列表" tabindex="-1"><a class="header-anchor" href="#服务列表"><span>服务列表</span></a></h3>
<h4 id="control-service-控制服务" tabindex="-1"><a class="header-anchor" href="#control-service-控制服务"><span>Control Service (控制服务)</span></a></h4>
<p>Agent 注册与连接管理。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">service</span> <span class="token class-name">ControlService</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token comment">// 注册 Agent</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">RegisterAgent</span><span class="token punctuation">(</span><span class="token class-name">RegisterAgentRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">RegisterAgentResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 心跳保活</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">Heartbeat</span><span class="token punctuation">(</span><span class="token class-name">HeartbeatRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">HeartbeatResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 获取分配配置</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">GetAssignments</span><span class="token punctuation">(</span><span class="token class-name">GetAssignmentsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">GetAssignmentsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 导出函数包</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">ExportPack</span><span class="token punctuation">(</span><span class="token class-name">ExportPackRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token keyword">stream</span> <span class="token class-name">ExportPackChunk</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="function-service-函数服务" tabindex="-1"><a class="header-anchor" href="#function-service-函数服务"><span>Function Service (函数服务)</span></a></h4>
<p>函数调用与作业管理。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">service</span> <span class="token class-name">FunctionService</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token comment">// 调用函数（同步）</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">InvokeFunction</span><span class="token punctuation">(</span><span class="token class-name">InvokeFunctionRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">InvokeFunctionResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 调用函数（异步作业）</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">InvokeJob</span><span class="token punctuation">(</span><span class="token class-name">InvokeJobRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">InvokeJobResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 流式获取作业事件</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">StreamJobEvents</span><span class="token punctuation">(</span><span class="token class-name">StreamJobEventsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token keyword">stream</span> <span class="token class-name">JobEvent</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 取消作业</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">CancelJob</span><span class="token punctuation">(</span><span class="token class-name">CancelJobRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">CancelJobResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 注册函数</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">RegisterFunction</span><span class="token punctuation">(</span><span class="token class-name">RegisterFunctionRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">RegisterFunctionResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 批量注册函数</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">RegisterFunctions</span><span class="token punctuation">(</span><span class="token class-name">RegisterFunctionsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">RegisterFunctionsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 注销函数</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">UnregisterFunction</span><span class="token punctuation">(</span><span class="token class-name">UnregisterFunctionRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">UnregisterFunctionResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 获取函数列表</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">ListFunctions</span><span class="token punctuation">(</span><span class="token class-name">ListFunctionsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">ListFunctionsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 获取函数详情</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">GetFunction</span><span class="token punctuation">(</span><span class="token class-name">GetFunctionRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">GetFunctionResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="registry-service-注册中心" tabindex="-1"><a class="header-anchor" href="#registry-service-注册中心"><span>Registry Service (注册中心)</span></a></h4>
<p>函数注册与发现。</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">service</span> <span class="token class-name">RegistryService</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token comment">// 获取函数注册信息</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">GetRegistrations</span><span class="token punctuation">(</span><span class="token class-name">GetRegistrationsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">GetRegistrationsResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// 监听注册变化</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">WatchRegistrations</span><span class="token punctuation">(</span><span class="token class-name">WatchRegistrationsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token keyword">stream</span> <span class="token class-name">RegistrationEvent</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="调用示例" tabindex="-1"><a class="header-anchor" href="#调用示例"><span>调用示例</span></a></h3>
<h4 id="go" tabindex="-1"><a class="header-anchor" href="#go"><span>Go</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"google.golang.org/grpc"</span></span>
<span class="line">    <span class="token string">"google.golang.org/grpc/credentials"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">conn<span class="token punctuation">,</span> err <span class="token operator">:=</span> grpc<span class="token punctuation">.</span><span class="token function">Dial</span><span class="token punctuation">(</span><span class="token string">"server:8443"</span><span class="token punctuation">,</span></span>
<span class="line">    grpc<span class="token punctuation">.</span><span class="token function">WithTransportCredentials</span><span class="token punctuation">(</span>credentials<span class="token punctuation">.</span><span class="token function">NewTLS</span><span class="token punctuation">(</span>tlsConfig<span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">client <span class="token operator">:=</span> <span class="token function">NewFunctionServiceClient</span><span class="token punctuation">(</span>conn<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">resp<span class="token punctuation">,</span> err <span class="token operator">:=</span> client<span class="token punctuation">.</span><span class="token function">InvokeFunction</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>InvokeFunctionRequest<span class="token punctuation">{</span></span>
<span class="line">    GameId<span class="token punctuation">:</span>    <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">    Env<span class="token punctuation">:</span>       <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">    FunctionId<span class="token punctuation">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    Payload<span class="token punctuation">:</span>   structpb<span class="token punctuation">.</span><span class="token function">NewStructValue</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    Options<span class="token punctuation">:</span> <span class="token operator">&amp;</span>InvokeOptions<span class="token punctuation">{</span></span>
<span class="line">        IdempotencyKey<span class="token punctuation">:</span> <span class="token string">"unique-key-123"</span><span class="token punctuation">,</span></span>
<span class="line">        Timeout<span class="token punctuation">:</span>        durationpb<span class="token punctuation">.</span><span class="token function">New</span><span class="token punctuation">(</span><span class="token number">30</span> <span class="token operator">*</span> time<span class="token punctuation">.</span>Second<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="c" tabindex="-1"><a class="header-anchor" href="#c"><span>C++</span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;croupier/sdk/client.h></span></span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;grpc/grpc.h></span></span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">auto</span> tls_credentials <span class="token operator">=</span> grpc<span class="token double-colon punctuation">::</span><span class="token function">SslCredentials</span><span class="token punctuation">(</span></span>
<span class="line">    grpc<span class="token double-colon punctuation">::</span>SslCredentialsOptions<span class="token punctuation">{</span></span>
<span class="line">        <span class="token punctuation">.</span>pem_root_certs <span class="token operator">=</span> <span class="token function">ReadFile</span><span class="token punctuation">(</span><span class="token string">"ca.crt"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">.</span>pem_cert_chain <span class="token operator">=</span> <span class="token function">ReadFile</span><span class="token punctuation">(</span><span class="token string">"client.crt"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">.</span>pem_private_key <span class="token operator">=</span> <span class="token function">ReadFile</span><span class="token punctuation">(</span><span class="token string">"client.key"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">auto</span> channel <span class="token operator">=</span> grpc<span class="token double-colon punctuation">::</span><span class="token function">CreateChannel</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"server:8443"</span><span class="token punctuation">,</span></span>
<span class="line">    tls_credentials</span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">croupier<span class="token double-colon punctuation">::</span>FunctionServiceClient <span class="token function">client</span><span class="token punctuation">(</span>channel<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">auto</span> response <span class="token operator">=</span> client<span class="token punctuation">.</span><span class="token function">InvokeFunction</span><span class="token punctuation">(</span></span>
<span class="line">    croupier<span class="token double-colon punctuation">::</span>InvokeFunctionRequest<span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">set_game_id</span><span class="token punctuation">(</span><span class="token string">"my-game"</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">set_env</span><span class="token punctuation">(</span><span class="token string">"prod"</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">set_function_id</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">set_payload</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="rest-api" tabindex="-1"><a class="header-anchor" href="#rest-api"><span>REST API</span></a></h2>
<h3 id="基础信息" tabindex="-1"><a class="header-anchor" href="#基础信息"><span>基础信息</span></a></h3>
<ul>
<li><strong>Base URL</strong>: <code v-pre>http://server:8080</code> 或 <code v-pre>https://server:8080</code></li>
<li><strong>Content-Type</strong>: <code v-pre>application/json</code></li>
<li><strong>认证</strong>: Bearer Token</li>
</ul>
<h3 id="通用请求头" tabindex="-1"><a class="header-anchor" href="#通用请求头"><span>通用请求头</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"><span class="token header"><span class="token header-name keyword">Authorization</span><span class="token punctuation">:</span> <span class="token header-value">Bearer {token}</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">X-Game-ID</span><span class="token punctuation">:</span> <span class="token header-value">{game_id}</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">X-Env</span><span class="token punctuation">:</span> <span class="token header-value">{env}</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">Content-Type</span><span class="token punctuation">:</span> <span class="token header-value">application/json</span></span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="端点列表" tabindex="-1"><a class="header-anchor" href="#端点列表"><span>端点列表</span></a></h3>
<h4 id="函数调用" tabindex="-1"><a class="header-anchor" href="#函数调用"><span>函数调用</span></a></h4>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"># 调用函数</span>
<span class="line">POST /api/invoke</span>
<span class="line">{</span>
<span class="line">  "function_id": "player.ban",</span>
<span class="line">  "payload": { ... },</span>
<span class="line">  "idempotency_key": "optional-key"</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line"># 调用函数（异步）</span>
<span class="line">POST /api/jobs</span>
<span class="line">{</span>
<span class="line">  "function_id": "player.ban",</span>
<span class="line">  "payload": { ... }</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line"># 获取作业状态</span>
<span class="line">GET /api/jobs/{job_id}</span>
<span class="line"></span>
<span class="line"># 取消作业</span>
<span class="line">DELETE /api/jobs/{job_id}</span>
<span class="line"></span>
<span class="line"># 流式获取作业事件</span>
<span class="line">GET /api/jobs/{job_id}/events</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="函数管理" tabindex="-1"><a class="header-anchor" href="#函数管理"><span>函数管理</span></a></h4>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"># 获取函数列表</span>
<span class="line">GET /api/functions?game_id={game_id}&amp;env={env}</span>
<span class="line"></span>
<span class="line"># 获取函数详情</span>
<span class="line">GET /api/functions/{function_id}?game_id={game_id}&amp;env={env}</span>
<span class="line"></span>
<span class="line"># 注册函数</span>
<span class="line">POST /api/functions</span>
<span class="line">{</span>
<span class="line">  "game_id": "my-game",</span>
<span class="line">  "env": "prod",</span>
<span class="line">  "descriptor": { ... }</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line"># 注销函数</span>
<span class="line">DELETE /api/functions/{function_id}?game_id={game_id}&amp;env={env}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="agent-管理" tabindex="-1"><a class="header-anchor" href="#agent-管理"><span>Agent 管理</span></a></h4>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"># 获取 Agent 列表</span>
<span class="line">GET /api/agents?game_id={game_id}&amp;env={env}</span>
<span class="line"></span>
<span class="line"># 获取 Agent 详情</span>
<span class="line">GET /api/agents/{agent_id}</span>
<span class="line"></span>
<span class="line"># 获取 Agent 分配配置</span>
<span class="line">GET /api/agents/{agent_id}/assignments</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="审批流程" tabindex="-1"><a class="header-anchor" href="#审批流程"><span>审批流程</span></a></h4>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"># 创建审批请求</span>
<span class="line">POST /api/approvals</span>
<span class="line">{</span>
<span class="line">  "function_id": "player.ban",</span>
<span class="line">  "payload": { ... },</span>
<span class="line">  "reason": "违规玩家处理"</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line"># 获取审批列表</span>
<span class="line">GET /api/approvals?status=pending</span>
<span class="line"></span>
<span class="line"># 审批通过</span>
<span class="line">POST /api/approvals/{approval_id}/approve</span>
<span class="line"></span>
<span class="line"># 审批拒绝</span>
<span class="line">POST /api/approvals/{approval_id}/reject</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="审计日志" tabindex="-1"><a class="header-anchor" href="#审计日志"><span>审计日志</span></a></h4>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"># 查询审计日志</span>
<span class="line">POST /api/audit/query</span>
<span class="line">{</span>
<span class="line">  "game_id": "my-game",</span>
<span class="line">  "env": "prod",</span>
<span class="line">  "start_time": "2024-01-01T00:00:00Z",</span>
<span class="line">  "end_time": "2024-01-02T00:00:00Z",</span>
<span class="line">  "filters": { ... }</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line"># 获取审计详情</span>
<span class="line">GET /api/audit/{audit_id}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="错误响应" tabindex="-1"><a class="header-anchor" href="#错误响应"><span>错误响应</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"code"</span><span class="token operator">:</span> <span class="token string">"PERMISSION_DENIED"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"没有权限执行该操作"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"details"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"required_permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"user_permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.view"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="错误码" tabindex="-1"><a class="header-anchor" href="#错误码"><span>错误码</span></a></h3>
<table>
<thead>
<tr>
<th>错误码</th>
<th>HTTP 状态</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>OK</code></td>
<td>200</td>
<td>成功</td>
</tr>
<tr>
<td><code v-pre>INVALID_ARGUMENT</code></td>
<td>400</td>
<td>参数错误</td>
</tr>
<tr>
<td><code v-pre>UNAUTHENTICATED</code></td>
<td>401</td>
<td>未认证</td>
</tr>
<tr>
<td><code v-pre>PERMISSION_DENIED</code></td>
<td>403</td>
<td>权限不足</td>
</tr>
<tr>
<td><code v-pre>NOT_FOUND</code></td>
<td>404</td>
<td>资源不存在</td>
</tr>
<tr>
<td><code v-pre>ALREADY_EXISTS</code></td>
<td>409</td>
<td>资源已存在</td>
</tr>
<tr>
<td><code v-pre>RESOURCE_EXHAUSTED</code></td>
<td>429</td>
<td>请求过于频繁</td>
</tr>
<tr>
<td><code v-pre>INTERNAL</code></td>
<td>500</td>
<td>内部错误</td>
</tr>
<tr>
<td><code v-pre>NOT_IMPLEMENTED</code></td>
<td>501</td>
<td>未实现</td>
</tr>
<tr>
<td><code v-pre>UNAVAILABLE</code></td>
<td>503</td>
<td>服务不可用</td>
</tr>
</tbody>
</table>
<h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/api/grpc.html">gRPC API 详情</RouteLink></li>
<li><RouteLink to="/api/rest.html">REST API 详情</RouteLink></li>
<li><RouteLink to="/api/proto-options.html">Proto 选项指南</RouteLink></li>
</ul>
</div></template>


