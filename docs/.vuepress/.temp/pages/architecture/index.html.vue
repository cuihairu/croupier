<template><div><h1 id="系统架构" tabindex="-1"><a class="header-anchor" href="#系统架构"><span>系统架构</span></a></h1>
<p>Croupier 采用<strong>三层分布式架构</strong>，实现权限控制、函数路由和 UI 展示的完全分离。</p>
<h2 id="架构图" tabindex="-1"><a class="header-anchor" href="#架构图"><span>架构图</span></a></h2>
<Mermaid code="eJx9lN9r01AUx9/7V1y2F2Upa9qudkUGaTuriFDagkLwIW3vzYIxKUk7FPbgJhvbdG7iZCjdQ3GlQ2FlD9PpGP4zS7L+F95fSRM7fWiS+72fe+4553uoaimtJVDLxwCwO3W2mnJOP3pHv5zTNRAHzu7werDmnr2ZwggARcVeqpuK1ZQfw/p4dbduzS5IRhsUoa2pBpgBT+IVaDSh9ZQee2TWNR3K3uDC2T72vg0p73a/uod9d3/ovl0lGOZjkTzcdwNn8zvLowqtZWixJKTyA/l+rVYGlcVqjaxouFw2kU2w6wqm0bZMXeZvelhrQIqplXIB5LLpdIqx9zpGo62Zhux//Jeu5KWCTB6zEn6wMg5fjz7tsVwZJHWaWlt2TnrXJz33oO/8PuB6q2WZy4pOttytn86P/tVl1z37V/F9Z73Pil9sqhDcwlaMXm3dZj0gkkz1smW+eEkzGX0ejFY/OEc73v7xjTGvLr54exsspqRCo837ST5Fmb6AyIo6P3c3d52Nde/yPZB4+mQ/ybHkJJa/uQ6KuN0dZ7uHb2Y3lqqiXFKeQ24rkGi0GVAtPmR3larJCJCfBFIRgHkRADyNYEBBPL6wQmZmhcwL3sFPolFD8ZK86JqPTFjyBwNr/kARmXXtL5E4giVqTAAlsRJMV+TopEpYtk013ChfSHJhgkiRSqORyACGS/Anj5ANXbHtIkSgowGk6XpuGmbRHEKCjat4BnPTYnY+gVCYtFmPGY0yCMFmQM8lG2lRCdMKHRAOI3QHZgIYKdmGmAnDKvGQswk0H2InA0PS1SBuPRJXaYrpoLqx7wL758G1BnvYeYFbJvhdE0inBNo2we8Vrzo4Rz0lKYwjURcE7g4tO9jDxgnYK/xL0RpjfwDp39sZ"></Mermaid><h2 id="分层说明" tabindex="-1"><a class="header-anchor" href="#分层说明"><span>分层说明</span></a></h2>
<h3 id="_1-展示层" tabindex="-1"><a class="header-anchor" href="#_1-展示层"><span>1. 展示层</span></a></h3>
<p><strong>职责</strong>: 用户界面、操作可视化、进度展示</p>
<p><strong>组件</strong>:</p>
<ul>
<li><strong>Dashboard</strong>: Web 管理界面，基于 React + Ant Design</li>
<li><strong>未来</strong>: 移动端支持</li>
</ul>
<p><strong>特性</strong>:</p>
<ul>
<li>X-Render 驱动的表单自动生成</li>
<li>实时日志流式展示</li>
<li>审批流程可视化</li>
<li>响应式设计</li>
</ul>
<h3 id="_2-控制层" tabindex="-1"><a class="header-anchor" href="#_2-控制层"><span>2. 控制层</span></a></h3>
<p><strong>职责</strong>: 权限控制、函数路由、审计记录</p>
<p><strong>组件</strong>:</p>
<ul>
<li><strong>HTTP API</strong>: RESTful 接口 (8080)</li>
<li><strong>Control Service</strong>: Agent 注册与连接管理</li>
<li><strong>Function Service</strong>: 函数调用路由</li>
<li><strong>RBAC/ABAC</strong>: 权限控制引擎</li>
<li><strong>审计日志</strong>: 操作记录与追溯</li>
<li><strong>审批工作流</strong>: 高风险操作审批</li>
</ul>
<p><strong>特性</strong>:</p>
<ul>
<li>负载均衡 (轮询/一致性哈希/最少连接)</li>
<li>幂等性保证</li>
<li>双人强制规则</li>
<li>敏感字段脱敏</li>
</ul>
<h3 id="_3-接入层" tabindex="-1"><a class="header-anchor" href="#_3-接入层"><span>3. 接入层</span></a></h3>
<p><strong>职责</strong>: 公网接入、隧道转发</p>
<p><strong>组件</strong>:</p>
<ul>
<li><strong>Edge Proxy</strong>: DMZ 部署的边缘代理</li>
</ul>
<p><strong>特性</strong>:</p>
<ul>
<li>双向隧道支持</li>
<li>连接复用</li>
<li>流量转发</li>
</ul>
<h3 id="_4-代理层" tabindex="-1"><a class="header-anchor" href="#_4-代理层"><span>4. 代理层</span></a></h3>
<p><strong>职责</strong>: 游戏内网代理、函数注册、调用转发</p>
<p><strong>组件</strong>:</p>
<ul>
<li><strong>Agent</strong>: 部署在游戏内网的代理进程</li>
</ul>
<p><strong>特性</strong>:</p>
<ul>
<li>出站 mTLS 连接</li>
<li>本地 gRPC 监听 (19090)</li>
<li>函数自动注册</li>
<li>异步作业执行</li>
<li>作业取消与进度流</li>
</ul>
<h3 id="_5-游戏服务层" tabindex="-1"><a class="header-anchor" href="#_5-游戏服务层"><span>5. 游戏服务层</span></a></h3>
<p><strong>职责</strong>: 业务逻辑实现</p>
<p><strong>组件</strong>:</p>
<ul>
<li><strong>Game Server</strong>: 游戏服务器进程</li>
<li><strong>SDK</strong>: 多语言客户端 SDK</li>
</ul>
<p><strong>特性</strong>:</p>
<ul>
<li>函数实现与注册</li>
<li>类型安全的 API</li>
<li>热重载支持</li>
</ul>
<h2 id="核心设计模式" tabindex="-1"><a class="header-anchor" href="#核心设计模式"><span>核心设计模式</span></a></h2>
<h3 id="_1-协议优先开发" tabindex="-1"><a class="header-anchor" href="#_1-协议优先开发"><span>1. 协议优先开发</span></a></h3>
<p>所有 API 通过 Protocol Buffers 定义：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// proto/croupier/control/v1/service.proto</span></span>
<span class="line"><span class="token keyword">service</span> <span class="token class-name">ControlService</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">RegisterAgent</span><span class="token punctuation">(</span><span class="token class-name">RegisterAgentRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">RegisterAgentResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">Heartbeat</span><span class="token punctuation">(</span><span class="token class-name">HeartbeatRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">HeartbeatResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// proto/croupier/function/v1/service.proto</span></span>
<span class="line"><span class="token keyword">service</span> <span class="token class-name">FunctionService</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">InvokeFunction</span><span class="token punctuation">(</span><span class="token class-name">InvokeFunctionRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">InvokeFunctionResponse</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">StreamJobEvents</span><span class="token punctuation">(</span><span class="token class-name">StreamJobEventsRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token keyword">stream</span> <span class="token class-name">JobEvent</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-描述符驱动-ui" tabindex="-1"><a class="header-anchor" href="#_2-描述符驱动-ui"><span>2. 描述符驱动 UI</span></a></h3>
<p>基于 JSON Schema 自动生成 UI：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span> <span class="token property">"minimum"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"risk_warning"</span><span class="token operator">:</span> <span class="token string">"高风险操作，需要审批"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-作业模型" tabindex="-1"><a class="header-anchor" href="#_3-作业模型"><span>3. 作业模型</span></a></h3>
<p>异步执行长时间任务：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">JobEvent</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> job_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">EventType</span> type <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>  <span class="token comment">// START, PROGRESS, DONE, ERROR</span></span>
<span class="line">  <span class="token builtin">string</span> message <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">double</span> progress <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="安全架构" tabindex="-1"><a class="header-anchor" href="#安全架构"><span>安全架构</span></a></h2>
<h3 id="通信安全" tabindex="-1"><a class="header-anchor" href="#通信安全"><span>通信安全</span></a></h3>
<table>
<thead>
<tr>
<th>连接</th>
<th>协议</th>
<th>安全方式</th>
</tr>
</thead>
<tbody>
<tr>
<td>Dashboard → Server</td>
<td>HTTPS</td>
<td>TLS</td>
</tr>
<tr>
<td>Server → Agent</td>
<td>gRPC</td>
<td>mTLS</td>
</tr>
<tr>
<td>Server → Edge</td>
<td>gRPC</td>
<td>mTLS</td>
</tr>
<tr>
<td>Edge → Agent</td>
<td>gRPC</td>
<td>mTLS</td>
</tr>
<tr>
<td>Agent → Game Server</td>
<td>gRPC</td>
<td>可选 mTLS</td>
</tr>
</tbody>
</table>
<h3 id="权限模型" tabindex="-1"><a class="header-anchor" href="#权限模型"><span>权限模型</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">用户 (User)</span>
<span class="line">  ↓ 拥有</span>
<span class="line">角色 (Role)</span>
<span class="line">  ↓ 分配</span>
<span class="line">权限 (Permission)</span>
<span class="line">  ↓ 保护</span>
<span class="line">函数 (Function)</span>
<span class="line">  ↓ 关联</span>
<span class="line">实体 (Entity)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审计日志" tabindex="-1"><a class="header-anchor" href="#审计日志"><span>审计日志</span></a></h3>
<p>所有操作记录包含：</p>
<ul>
<li>操作时间</li>
<li>操作用户</li>
<li>目标游戏/环境</li>
<li>函数与参数</li>
<li>审批信息</li>
<li>执行结果</li>
<li>哈希防篡改</li>
</ul>
<h2 id="可观测性" tabindex="-1"><a class="header-anchor" href="#可观测性"><span>可观测性</span></a></h2>
<h3 id="指标-metrics" tabindex="-1"><a class="header-anchor" href="#指标-metrics"><span>指标 (Metrics)</span></a></h3>
<ul>
<li>Prometheus 格式输出</li>
<li><code v-pre>/metrics</code> 端点</li>
<li>按函数/游戏统计</li>
</ul>
<h3 id="日志-logging" tabindex="-1"><a class="header-anchor" href="#日志-logging"><span>日志 (Logging)</span></a></h3>
<ul>
<li>结构化日志 (JSON)</li>
<li>日志级别可配置</li>
<li>支持文件轮转</li>
</ul>
<h3 id="追踪-tracing" tabindex="-1"><a class="header-anchor" href="#追踪-tracing"><span>追踪 (Tracing)</span></a></h3>
<ul>
<li>OpenTelemetry 集成</li>
<li>Jaeger 导出</li>
<li>分布式调用链</li>
</ul>
<h2 id="部署架构" tabindex="-1"><a class="header-anchor" href="#部署架构"><span>部署架构</span></a></h2>
<h3 id="单机房部署" tabindex="-1"><a class="header-anchor" href="#单机房部署"><span>单机房部署</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────┐</span>
<span class="line">│           数据中心                       │</span>
<span class="line">│  ┌─────────────────────────────────┐   │</span>
<span class="line">│  │  管理网段 (内网)                │   │</span>
<span class="line">│  │  Server + Dashboard            │   │</span>
<span class="line">│  │         │                       │   │</span>
<span class="line">│  │    Agent (游戏服务器旁)         │   │</span>
<span class="line">│  └─────────────────────────────────┘   │</span>
<span class="line">│  ┌─────────────────────────────────┐   │</span>
<span class="line">│  │  DMZ 网段                        │   │</span>
<span class="line">│  │  Edge (可选)                    │   │</span>
<span class="line">│  └─────────────────────────────────┘   │</span>
<span class="line">└─────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="多机房部署" tabindex="-1"><a class="header-anchor" href="#多机房部署"><span>多机房部署</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌──────────────┐       ┌──────────────┐</span>
<span class="line">│  机房 A       │       │  机房 B       │</span>
<span class="line">│  Server      │◄─────►│  Server      │</span>
<span class="line">│  │           │       │  │           │</span>
<span class="line">│  Agent       │       │  Agent       │</span>
<span class="line">│  │           │       │  │           │</span>
<span class="line">│  Game Server │       │  Game Server │</span>
<span class="line">└──────────────┘       └──────────────┘</span>
<span class="line">       │                       │</span>
<span class="line">       └───────────┬───────────┘</span>
<span class="line">                   │</span>
<span class="line">              ┌─────────┐</span>
<span class="line">              │  Edge   │</span>
<span class="line">              │  (DMZ)  │</span>
<span class="line">              └─────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/architecture/layers.html">分层设计</RouteLink></li>
<li><RouteLink to="/architecture/components.html">组件说明</RouteLink></li>
<li><RouteLink to="/architecture/data-flow.html">数据流</RouteLink></li>
<li><RouteLink to="/architecture/design-patterns.html">设计模式</RouteLink></li>
</ul>
</div></template>


