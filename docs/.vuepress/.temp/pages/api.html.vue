<template><div><h1 id="croupier-api-文档" tabindex="-1"><a class="header-anchor" href="#croupier-api-文档"><span>Croupier API 文档</span></a></h1>
<p>本文档描述 Croupier 系统暴露的 gRPC 和 HTTP REST API。</p>
<blockquote>
<p><strong>注意</strong>: 权威 API 定义在 <code v-pre>proto/</code> 目录中。本文档提供概览和使用指南。</p>
</blockquote>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<ul>
<li><a href="#%E6%A0%B8%E5%BF%83%E6%9C%8D%E5%8A%A1">核心服务</a>
<ul>
<li><a href="#controlservice">ControlService</a> - Agent 注册与管理</li>
<li><a href="#functionservice">FunctionService</a> - 函数调用</li>
<li><a href="#edgeservice">EdgeService</a> - Edge 代理</li>
<li><a href="#localcontrolservice">LocalControlService</a> - 本地控制</li>
</ul>
</li>
<li><a href="#http-rest-api">HTTP REST API</a></li>
<li><a href="#%E6%95%B0%E6%8D%AE%E6%A8%A1%E5%9E%8B">数据模型</a></li>
<li><a href="#%E9%94%99%E8%AF%AF%E7%A0%81">错误码</a></li>
</ul>
<hr>
<h2 id="核心服务" tabindex="-1"><a class="header-anchor" href="#核心服务"><span>核心服务</span></a></h2>
<h3 id="controlservice" tabindex="-1"><a class="header-anchor" href="#controlservice"><span>ControlService</span></a></h3>
<p><strong>包</strong>: <code v-pre>croupier.server.v1</code></p>
<p><strong>功能</strong>: Agent 注册、心跳、能力声明和函数目录查询</p>
<h4 id="方法" tabindex="-1"><a class="header-anchor" href="#方法"><span>方法</span></a></h4>
<table>
<thead>
<tr>
<th>方法</th>
<th>请求</th>
<th>响应</th>
<th>描述</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>Register</code></td>
<td><code v-pre>RegisterRequest</code></td>
<td><code v-pre>RegisterResponse</code></td>
<td>Agent 注册到 Server</td>
</tr>
<tr>
<td><code v-pre>Heartbeat</code></td>
<td><code v-pre>HeartbeatRequest</code></td>
<td><code v-pre>HeartbeatResponse</code></td>
<td>Agent 心跳保活</td>
</tr>
<tr>
<td><code v-pre>RegisterCapabilities</code></td>
<td><code v-pre>RegisterCapabilitiesRequest</code></td>
<td><code v-pre>RegisterCapabilitiesResponse</code></td>
<td>注册 Provider 能力清单</td>
</tr>
<tr>
<td><code v-pre>ListFunctionsSummary</code></td>
<td><code v-pre>google.protobuf.Empty</code></td>
<td><code v-pre>ListFunctionsSummaryResponse</code></td>
<td>获取函数目录摘要</td>
</tr>
</tbody>
</table>
<h4 id="消息定义" tabindex="-1"><a class="header-anchor" href="#消息定义"><span>消息定义</span></a></h4>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// Agent 注册请求</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>              <span class="token comment">// Agent 唯一 ID</span></span>
<span class="line">  <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>               <span class="token comment">// Agent 版本</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">FunctionDescriptor</span> functions <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// 函数列表</span></span>
<span class="line">  <span class="token builtin">string</span> rpc_addr <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>              <span class="token comment">// Agent 可达 gRPC 地址</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>               <span class="token comment">// 游戏 ID (多租户必需)</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>                   <span class="token comment">// 环境 (prod/stage/test)</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">AgentProcess</span> processes <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span>  <span class="token comment">// 注册的进程</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Agent 注册响应</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> session_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>            <span class="token comment">// 会话 ID</span></span>
<span class="line">  <span class="token builtin">int64</span> expire_at <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>              <span class="token comment">// 过期时间 (Unix 秒)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 心跳请求</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">HeartbeatRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> agent_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> session_id <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Provider 能力注册</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterCapabilitiesRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token positional-class-name class-name">ProviderMeta</span> provider <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>        <span class="token comment">// Provider 元信息</span></span>
<span class="line">  <span class="token builtin">bytes</span> manifest_json_gz <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>       <span class="token comment">// Gzip 压缩的 manifest.json</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 函数描述符（包含 UI/RBAC 元数据）</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">FunctionDescriptor</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>                   <span class="token comment">// 函数 ID，如 "player.ban"</span></span>
<span class="line">  <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>              <span class="token comment">// SemVer，如 "1.2.0"</span></span>
<span class="line">  <span class="token builtin">string</span> category <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>             <span class="token comment">// 分组类别</span></span>
<span class="line">  <span class="token builtin">string</span> risk <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>                 <span class="token comment">// 风险级别: low/medium/high</span></span>
<span class="line">  <span class="token builtin">string</span> entity <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>               <span class="token comment">// 实体类型，如 "player"</span></span>
<span class="line">  <span class="token builtin">string</span> operation <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>            <span class="token comment">// 操作类型: create/read/update/delete</span></span>
<span class="line">  <span class="token builtin">bool</span> enabled <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span>                <span class="token comment">// 是否启用</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment">// UI/i18n/权限元数据</span></span>
<span class="line">  <span class="token positional-class-name class-name">croupier<span class="token punctuation">.</span>common<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>I18nText</span> display_name <span class="token operator">=</span> <span class="token number">20</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">croupier<span class="token punctuation">.</span>common<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>I18nText</span> summary <span class="token operator">=</span> <span class="token number">21</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token builtin">string</span> tags <span class="token operator">=</span> <span class="token number">22</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">croupier<span class="token punctuation">.</span>common<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>Menu</span> menu <span class="token operator">=</span> <span class="token number">23</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">croupier<span class="token punctuation">.</span>common<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>PermissionSpec</span> permissions <span class="token operator">=</span> <span class="token number">24</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="使用示例-go" tabindex="-1"><a class="header-anchor" href="#使用示例-go"><span>使用示例 (Go)</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">import</span> serverv1 <span class="token string">"github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Agent 注册</span></span>
<span class="line">resp<span class="token punctuation">,</span> err <span class="token operator">:=</span> controlClient<span class="token punctuation">.</span><span class="token function">Register</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>serverv1<span class="token punctuation">.</span>RegisterRequest<span class="token punctuation">{</span></span>
<span class="line">    AgentId<span class="token punctuation">:</span> <span class="token string">"agent-001"</span><span class="token punctuation">,</span></span>
<span class="line">    Version<span class="token punctuation">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">    GameId<span class="token punctuation">:</span>  <span class="token string">"mygame"</span><span class="token punctuation">,</span></span>
<span class="line">    Env<span class="token punctuation">:</span>     <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">    Functions<span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token operator">*</span>serverv1<span class="token punctuation">.</span>FunctionDescriptor<span class="token punctuation">{</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">            Id<span class="token punctuation">:</span>      <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">            Version<span class="token punctuation">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">            Enabled<span class="token punctuation">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h3 id="functionservice" tabindex="-1"><a class="header-anchor" href="#functionservice"><span>FunctionService</span></a></h3>
<p><strong>包</strong>: <code v-pre>croupier.function.v1</code></p>
<p><strong>功能</strong>: 函数调用、异步作业、流式事件</p>
<h4 id="方法-1" tabindex="-1"><a class="header-anchor" href="#方法-1"><span>方法</span></a></h4>
<table>
<thead>
<tr>
<th>方法</th>
<th>请求</th>
<th>响应</th>
<th>描述</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>Invoke</code></td>
<td><code v-pre>InvokeRequest</code></td>
<td><code v-pre>InvokeResponse</code></td>
<td>同步函数调用</td>
</tr>
<tr>
<td><code v-pre>StartJob</code></td>
<td><code v-pre>InvokeRequest</code></td>
<td><code v-pre>StartJobResponse</code></td>
<td>启动异步作业</td>
</tr>
<tr>
<td><code v-pre>StreamJob</code></td>
<td><code v-pre>JobStreamRequest</code></td>
<td><code v-pre>stream JobEvent</code></td>
<td>订阅作业事件流</td>
</tr>
<tr>
<td><code v-pre>CancelJob</code></td>
<td><code v-pre>CancelJobRequest</code></td>
<td><code v-pre>StartJobResponse</code></td>
<td>取消作业</td>
</tr>
</tbody>
</table>
<h4 id="消息定义-1" tabindex="-1"><a class="header-anchor" href="#消息定义-1"><span>消息定义</span></a></h4>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// 函数调用请求</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> function_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>           <span class="token comment">// 函数 ID</span></span>
<span class="line">  <span class="token builtin">string</span> idempotency_key <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>       <span class="token comment">// 幂等键（可选）</span></span>
<span class="line">  <span class="token builtin">bytes</span> payload <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>                <span class="token comment">// 请求载荷 (JSON/Proto)</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> metadata <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span> <span class="token comment">// 元数据</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 函数调用响应</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">bytes</span> payload <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>                <span class="token comment">// 响应载荷</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 作业事件</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">JobEvent</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> type <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>                  <span class="token comment">// 事件类型: progress/log/done/error</span></span>
<span class="line">  <span class="token builtin">string</span> message <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>               <span class="token comment">// 消息内容</span></span>
<span class="line">  <span class="token builtin">int32</span> progress <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>              <span class="token comment">// 进度 0-100 (type=progress 时)</span></span>
<span class="line">  <span class="token builtin">bytes</span> payload <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>               <span class="token comment">// 最终结果 (type=done 时)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="使用示例-go-1" tabindex="-1"><a class="header-anchor" href="#使用示例-go-1"><span>使用示例 (Go)</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 同步调用</span></span>
<span class="line">resp<span class="token punctuation">,</span> err <span class="token operator">:=</span> functionClient<span class="token punctuation">.</span><span class="token function">Invoke</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>functionv1<span class="token punctuation">.</span>InvokeRequest<span class="token punctuation">{</span></span>
<span class="line">    FunctionId<span class="token punctuation">:</span>     <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    IdempotencyKey<span class="token punctuation">:</span> <span class="token string">"req-12345"</span><span class="token punctuation">,</span></span>
<span class="line">    Payload<span class="token punctuation">:</span>       jsonPayload<span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 启动异步作业</span></span>
<span class="line">jobResp<span class="token punctuation">,</span> err <span class="token operator">:=</span> functionClient<span class="token punctuation">.</span><span class="token function">StartJob</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>functionv1<span class="token punctuation">.</span>InvokeRequest<span class="token punctuation">{</span></span>
<span class="line">    FunctionId<span class="token punctuation">:</span> <span class="token string">"player.mass_ban"</span><span class="token punctuation">,</span></span>
<span class="line">    Payload<span class="token punctuation">:</span>    payload<span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 订阅作业事件流</span></span>
<span class="line">stream<span class="token punctuation">,</span> err <span class="token operator">:=</span> functionClient<span class="token punctuation">.</span><span class="token function">StreamJob</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token operator">&amp;</span>functionv1<span class="token punctuation">.</span>JobStreamRequest<span class="token punctuation">{</span></span>
<span class="line">    JobId<span class="token punctuation">:</span> jobResp<span class="token punctuation">.</span>JobId<span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">for</span> <span class="token punctuation">{</span></span>
<span class="line">    event<span class="token punctuation">,</span> err <span class="token operator">:=</span> stream<span class="token punctuation">.</span><span class="token function">Recv</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">==</span> io<span class="token punctuation">.</span>EOF <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">break</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token comment">// 处理 event...</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h3 id="edgeservice" tabindex="-1"><a class="header-anchor" href="#edgeservice"><span>EdgeService</span></a></h3>
<p><strong>包</strong>: <code v-pre>croupier.server.v1</code></p>
<p><strong>功能</strong>: Edge 代理作业查询</p>
<h4 id="方法-2" tabindex="-1"><a class="header-anchor" href="#方法-2"><span>方法</span></a></h4>
<table>
<thead>
<tr>
<th>方法</th>
<th>请求</th>
<th>响应</th>
<th>描述</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>GetJobResult</code></td>
<td><code v-pre>GetJobResultRequest</code></td>
<td><code v-pre>GetJobResultResponse</code></td>
<td>查询作业结果</td>
</tr>
</tbody>
</table>
<h4 id="消息定义-2" tabindex="-1"><a class="header-anchor" href="#消息定义-2"><span>消息定义</span></a></h4>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">GetJobResultRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> job_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">GetJobResultResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> state <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>      <span class="token comment">// 作业状态</span></span>
<span class="line">  <span class="token builtin">bytes</span> payload <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>     <span class="token comment">// 结果载荷</span></span>
<span class="line">  <span class="token builtin">string</span> error <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>      <span class="token comment">// 错误信息</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h3 id="localcontrolservice" tabindex="-1"><a class="header-anchor" href="#localcontrolservice"><span>LocalControlService</span></a></h3>
<p><strong>包</strong>: <code v-pre>croupier.agent.local.v1</code></p>
<p><strong>功能</strong>: Agent 本地控制面（SDK 到 Agent 的本地注册）</p>
<h4 id="方法-3" tabindex="-1"><a class="header-anchor" href="#方法-3"><span>方法</span></a></h4>
<table>
<thead>
<tr>
<th>方法</th>
<th>描述</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>RegisterService</code></td>
<td>SDK 注册本地服务到 Agent</td>
</tr>
<tr>
<td><code v-pre>UnregisterService</code></td>
<td>SDK 注销服务</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="http-rest-api" tabindex="-1"><a class="header-anchor" href="#http-rest-api"><span>HTTP REST API</span></a></h2>
<p>Server 同时暴露 HTTP REST API (端口 8080)，用于 Dashboard 和 Web 客户端。</p>
<h3 id="基础路径" tabindex="-1"><a class="header-anchor" href="#基础路径"><span>基础路径</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">http://server:8080/api/v1</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="公共端点" tabindex="-1"><a class="header-anchor" href="#公共端点"><span>公共端点</span></a></h3>
<table>
<thead>
<tr>
<th>方法</th>
<th>路径</th>
<th>描述</th>
</tr>
</thead>
<tbody>
<tr>
<td>GET</td>
<td><code v-pre>/functions/descriptors</code></td>
<td>获取所有函数描述符</td>
</tr>
<tr>
<td>GET</td>
<td><code v-pre>/functions/descriptors/:id</code></td>
<td>获取单个函数描述符</td>
</tr>
<tr>
<td>GET</td>
<td><code v-pre>/functions/instances</code></td>
<td>获取函数实例列表</td>
</tr>
<tr>
<td>POST</td>
<td><code v-pre>/functions/validate</code></td>
<td>验证函数调用请求</td>
</tr>
<tr>
<td>GET</td>
<td><code v-pre>/providers/capabilities</code></td>
<td>获取 Provider 能力</td>
</tr>
<tr>
<td>GET</td>
<td><code v-pre>/providers/descriptors</code></td>
<td>获取 Provider 描述符</td>
</tr>
<tr>
<td>GET</td>
<td><code v-pre>/packs</code></td>
<td>获取函数包列表</td>
</tr>
<tr>
<td>POST</td>
<td><code v-pre>/packs/reload</code></td>
<td>重新加载函数包</td>
</tr>
<tr>
<td>GET</td>
<td><code v-pre>/packs/export</code></td>
<td>导出函数包</td>
</tr>
</tbody>
</table>
<h3 id="请求头" tabindex="-1"><a class="header-anchor" href="#请求头"><span>请求头</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">X-Game-ID: mygame       # 游戏 ID (多租户必需)</span>
<span class="line">X-Env: prod             # 环境 (可选)</span>
<span class="line">Authorization: Bearer &lt;token>  # 认证令牌</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="响应格式" tabindex="-1"><a class="header-anchor" href="#响应格式"><span>响应格式</span></a></h3>
<p>成功响应:</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"code"</span><span class="token operator">:</span> <span class="token number">0</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"success"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"data"</span><span class="token operator">:</span> <span class="token punctuation">{</span> ... <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>错误响应:</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"code"</span><span class="token operator">:</span> <span class="token number">10001</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"function not found"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"details"</span><span class="token operator">:</span> <span class="token punctuation">{</span> ... <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="数据模型" tabindex="-1"><a class="header-anchor" href="#数据模型"><span>数据模型</span></a></h2>
<h3 id="i18ntext-国际化文本" tabindex="-1"><a class="header-anchor" href="#i18ntext-国际化文本"><span>I18nText (国际化文本)</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">I18nText</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> en <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// 英文</span></span>
<span class="line">  <span class="token builtin">string</span> zh <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span> <span class="token comment">// 中文</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="menu-菜单元数据" tabindex="-1"><a class="header-anchor" href="#menu-菜单元数据"><span>Menu (菜单元数据)</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">Menu</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> section <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>   <span class="token comment">// 菜单分区</span></span>
<span class="line">  <span class="token builtin">string</span> group <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>     <span class="token comment">// 菜单分组</span></span>
<span class="line">  <span class="token builtin">string</span> path <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>      <span class="token comment">// 路由路径</span></span>
<span class="line">  <span class="token builtin">int32</span> order <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>      <span class="token comment">// 排序</span></span>
<span class="line">  <span class="token builtin">string</span> icon <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span>      <span class="token comment">// 图标</span></span>
<span class="line">  <span class="token builtin">string</span> badge <span class="token operator">=</span> <span class="token number">6</span><span class="token punctuation">;</span>     <span class="token comment">// 徽章</span></span>
<span class="line">  <span class="token builtin">bool</span> hidden <span class="token operator">=</span> <span class="token number">7</span><span class="token punctuation">;</span>      <span class="token comment">// 是否隐藏</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="permissionspec-权限规范" tabindex="-1"><a class="header-anchor" href="#permissionspec-权限规范"><span>PermissionSpec (权限规范)</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">PermissionSpec</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token builtin">string</span> verbs <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>           <span class="token comment">// 权限动词: read/invoke/write</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token builtin">string</span> scopes <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>          <span class="token comment">// 权限范围</span></span>
<span class="line">  <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">RoleBinding</span> defaults <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>   <span class="token comment">// 默认角色绑定</span></span>
<span class="line">  <span class="token map class-name">map<span class="token punctuation">&lt;</span><span class="token builtin">string</span><span class="token punctuation">,</span> <span class="token builtin">string</span><span class="token punctuation">></span></span> i18n_zh <span class="token operator">=</span> <span class="token number">10</span><span class="token punctuation">;</span>    <span class="token comment">// 中文国际化</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="错误码" tabindex="-1"><a class="header-anchor" href="#错误码"><span>错误码</span></a></h2>
<table>
<thead>
<tr>
<th>gRPC 状态</th>
<th>HTTP 状态</th>
<th>描述</th>
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
<td>请求参数无效</td>
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
<td><code v-pre>UNAVAILABLE</code></td>
<td>503</td>
<td>服务不可用</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="认证与安全" tabindex="-1"><a class="header-anchor" href="#认证与安全"><span>认证与安全</span></a></h2>
<h3 id="mtls" tabindex="-1"><a class="header-anchor" href="#mtls"><span>mTLS</span></a></h3>
<p>所有服务间通信 (Server ↔ Agent ↔ Edge) 强制使用 mTLS。</p>
<p>配置示例:</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">ca_file</span><span class="token punctuation">:</span>   /etc/croupier/ca.crt</span>
<span class="line">  <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> /etc/croupier/server.crt</span>
<span class="line">  <span class="token key atrule">key_file</span><span class="token punctuation">:</span>  /etc/croupier/server.key</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="jwt-token" tabindex="-1"><a class="header-anchor" href="#jwt-token"><span>JWT Token</span></a></h3>
<p>HTTP REST API 使用 JWT 认证。</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 登录获取 token</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://server:8080/api/v1/auth/login <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{"username": "admin", "password": "..."}'</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 使用 token 调用 API</span></span>
<span class="line"><span class="token function">curl</span> http://server:8080/api/v1/functions/descriptors <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Authorization: Bearer &lt;token>"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="参考资料" tabindex="-1"><a class="header-anchor" href="#参考资料"><span>参考资料</span></a></h2>
<ul>
<li><strong>Proto 定义</strong>: <code v-pre>proto/</code> 目录</li>
<li><strong>服务实现</strong>: <code v-pre>services/server/</code>, <code v-pre>services/agent/</code></li>
<li><strong>HTTP 路由</strong>: <code v-pre>services/server/internal/handler/routes.go</code></li>
<li><strong>客户端 SDK</strong>: <code v-pre>sdks/go/</code>, <code v-pre>sdks/cpp/</code>, <code v-pre>sdks/java/</code></li>
</ul>
</div></template>


