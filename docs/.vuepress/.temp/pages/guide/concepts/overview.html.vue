<template><div><h1 id="系统概览" tabindex="-1"><a class="header-anchor" href="#系统概览"><span>系统概览</span></a></h1>
<p>Croupier 是一个现代化的<strong>三层分布式 GM 后台系统</strong>，专为游戏运营和管理而设计。</p>
<h2 id="设计理念" tabindex="-1"><a class="header-anchor" href="#设计理念"><span>设计理念</span></a></h2>
<h3 id="三层架构" tabindex="-1"><a class="header-anchor" href="#三层架构"><span>三层架构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                    可观测展示层                            │</span>
<span class="line">│  描述符驱动 UI · 风控提示 · 敏感字段脱敏 · 进度追踪        │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              │</span>
<span class="line">                              ▼</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                    函数控制层                              │</span>
<span class="line">│  函数注册 · 调用路由 · 幂等处理 · 负载均衡                │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              │</span>
<span class="line">                              ▼</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                    权限控制层                              │</span>
<span class="line">│  RBAC/ABAC · 操作审批 · 审计日志 · 风控策略                │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="核心特性" tabindex="-1"><a class="header-anchor" href="#核心特性"><span>核心特性</span></a></h3>
<table>
<thead>
<tr>
<th>特性</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>零信任安全</strong></td>
<td>gRPC+mTLS、细粒度 RBAC/ABAC、操作审批与审计日志</td>
</tr>
<tr>
<td><strong>函数注册驱动</strong></td>
<td>游戏服务器通过 Agent 注册函数，控制面统一管理</td>
</tr>
<tr>
<td><strong>Schema 驱动 UI</strong></td>
<td>X-Render + JSON Schema 自动生成表单和界面</td>
</tr>
<tr>
<td><strong>可观测性解耦</strong></td>
<td>控制面与遥测面分离，支持实时事件处理</td>
</tr>
<tr>
<td><strong>多语言 SDK</strong></td>
<td>Go / C++ / Java / JS / Python 全覆盖</td>
</tr>
<tr>
<td><strong>协议优先</strong></td>
<td>所有 API 通过 Protocol Buffers 定义</td>
</tr>
</tbody>
</table>
<h2 id="系统组件" tabindex="-1"><a class="header-anchor" href="#系统组件"><span>系统组件</span></a></h2>
<h3 id="server-控制平面" tabindex="-1"><a class="header-anchor" href="#server-控制平面"><span>Server（控制平面）</span></a></h3>
<p>中央控制平面，负责权限控制、函数路由和协调。</p>
<ul>
<li><strong>端口</strong>: gRPC 8443 (mTLS), HTTP 8080 (REST)</li>
<li><strong>职责</strong>:
<ul>
<li>Agent 注册与连接管理</li>
<li>函数调用路由与负载均衡</li>
<li>RBAC/ABAC 权限校验</li>
<li>审计日志记录</li>
<li>操作审批工作流</li>
</ul>
</li>
</ul>
<h3 id="agent-分布式代理" tabindex="-1"><a class="header-anchor" href="#agent-分布式代理"><span>Agent（分布式代理）</span></a></h3>
<p>部署在游戏内网的代理进程，负责游戏服务器与控制平面的通信。</p>
<ul>
<li><strong>端口</strong>: gRPC 19090 (本地监听)</li>
<li><strong>职责</strong>:
<ul>
<li>连接 Server 并保持长连接</li>
<li>注册游戏服务器函数</li>
<li>转发函数调用请求</li>
<li>执行异步作业</li>
<li>支持双向隧道</li>
</ul>
</li>
</ul>
<h3 id="edge-边缘代理" tabindex="-1"><a class="header-anchor" href="#edge-边缘代理"><span>Edge（边缘代理）</span></a></h3>
<p>可选的 DMZ/边缘组件，用于公网场景。</p>
<ul>
<li><strong>职责</strong>:
<ul>
<li>桥接内网 Server 和公网 Agent</li>
<li>隧道切换与连接复用</li>
<li>流量转发与负载均衡</li>
</ul>
</li>
</ul>
<h3 id="dashboard-管理界面" tabindex="-1"><a class="header-anchor" href="#dashboard-管理界面"><span>Dashboard（管理界面）</span></a></h3>
<p>基于 React + Ant Design 的 Web 管理界面。</p>
<ul>
<li><strong>技术栈</strong>: Umi Max + Ant Design + X-Render</li>
<li><strong>职责</strong>:
<ul>
<li>函数调用可视化</li>
<li>审批流程管理</li>
<li>实时日志查看</li>
<li>权限配置界面</li>
</ul>
</li>
</ul>
<h2 id="数据流模式" tabindex="-1"><a class="header-anchor" href="#数据流模式"><span>数据流模式</span></a></h2>
<h3 id="标准调用流程" tabindex="-1"><a class="header-anchor" href="#标准调用流程"><span>标准调用流程</span></a></h3>
<Mermaid code="eJwrTi0sTc1LTnXJTEwvSszlUlAoSCwqyUzOLEjMK1EI9VRILFYIT00CstCkglOLylKLQNLORfmlBZlANkQITZ1jeiqQRFYGFkFT5R4MUuKemJsKMwWoINRT184OwrVSCPAPDlHQTyzI1M/MK8vPTgXKQ6SQ1AQ5OTorvOzc8mxuMzbpp+sWPuvc+Wxxw7P5S5HlwQ6yUkgPCnBW8IQZDhYESroHWyk8m7Pm6ZwNEAUvNjQ/n7ICqMA9WBeh9+nk3qe7psC1IVsKk8HmnBcgtOHp3qkIeaCCUE8rhee7Jz+bN4cLAHcRm5M="></Mermaid><h3 id="隧道模式-经-edge" tabindex="-1"><a class="header-anchor" href="#隧道模式-经-edge"><span>隧道模式（经 Edge）</span></a></h3>
<Mermaid code="eJxdzr0KwjAQAODdp7gXEPcMgqCUTgqxOEc9ShBrrT+7OotInaSImw6COgn+PY2t9S28tFqlS3K5L/fTw+4ArQYWpTAd0c4A2MLpy4a0hdUHQwfRgxrWKUoRR2eIjuI4SnGpaaJCdaeoYCKdZFGQQo0r0UQbv33pg6Fn8/n4yaBS5lXICVvmpDXstFT3mOiPmsYgvO382Tzcn4LjmFQlyaJpDF7LzWvkPq7r8O4SRllSjTMIvJ3vHcLD5LnYEmk8+yvz3al/XiQFyawkH035WzOBz3JEhs7geXGDlZd5A/p/jQg="></Mermaid><h2 id="虚拟对象系统" tabindex="-1"><a class="header-anchor" href="#虚拟对象系统"><span>虚拟对象系统</span></a></h2>
<p>Croupier 采用<strong>四层虚拟对象模型</strong>：</p>
<h3 id="_1-function-函数" tabindex="-1"><a class="header-anchor" href="#_1-function-函数"><span>1. Function（函数）</span></a></h3>
<p>具体的业务操作实现。</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"封禁玩家"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-entity-实体" tabindex="-1"><a class="header-anchor" href="#_2-entity-实体"><span>2. Entity（实体）</span></a></h3>
<p>业务对象的完整描述。</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"玩家"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* JSON Schema */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"create"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token string">"player.get"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"update"</span><span class="token operator">:</span> <span class="token string">"player.update"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"delete"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-resource-资源" tabindex="-1"><a class="header-anchor" href="#_3-resource-资源"><span>3. Resource（资源）</span></a></h3>
<p>UI 层面的操作集合。</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"pro-table"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"columns"</span><span class="token operator">:</span> <span class="token punctuation">[</span> <span class="token comment">/* 列定义 */</span> <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"create"</span><span class="token punctuation">,</span> <span class="token string">"edit"</span><span class="token punctuation">,</span> <span class="token string">"delete"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-component-组件" tabindex="-1"><a class="header-anchor" href="#_4-component-组件"><span>4. Component（组件）</span></a></h3>
<p>功能模块的打包单位。</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player-management"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.*"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"resources"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.resource"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="安全模型" tabindex="-1"><a class="header-anchor" href="#安全模型"><span>安全模型</span></a></h2>
<h3 id="rbac-abac-权限控制" tabindex="-1"><a class="header-anchor" href="#rbac-abac-权限控制"><span>RBAC/ABAC 权限控制</span></a></h3>
<ul>
<li><strong>RBAC</strong>: 基于角色的访问控制</li>
<li><strong>ABAC</strong>: 基于属性的访问控制</li>
<li><strong>审批工作流</strong>: 高风险操作需要双人审批</li>
<li><strong>审计链</strong>: 所有操作可追溯、防篡改</li>
</ul>
<h3 id="mtls-通信" tabindex="-1"><a class="header-anchor" href="#mtls-通信"><span>mTLS 通信</span></a></h3>
<ul>
<li>服务间强制使用 mTLS</li>
<li>支持自定义 CA 签名</li>
<li>证书自动轮换（可选）</li>
</ul>
<h2 id="关键概念" tabindex="-1"><a class="header-anchor" href="#关键概念"><span>关键概念</span></a></h2>
<table>
<thead>
<tr>
<th>概念</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Game ID</strong></td>
<td>游戏标识，用于租户隔离</td>
</tr>
<tr>
<td><strong>Env</strong></td>
<td>环境标识（dev/staging/prod）</td>
</tr>
<tr>
<td><strong>Function ID</strong></td>
<td>函数唯一标识</td>
</tr>
<tr>
<td><strong>Idempotency Key</strong></td>
<td>幂等键，防止重复执行</td>
</tr>
<tr>
<td><strong>Job ID</strong></td>
<td>异步作业标识</td>
</tr>
<tr>
<td><strong>Pack</strong></td>
<td>函数打包文件 (.tgz)</td>
</tr>
</tbody>
</table>
<h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/guide/concepts/virtual-objects.html">虚拟对象设计</RouteLink></li>
<li><RouteLink to="/guide/concepts/function-management.html">函数管理</RouteLink></li>
<li><RouteLink to="/guide/concepts/permissions.html">权限控制</RouteLink></li>
<li><RouteLink to="/architecture/">系统架构</RouteLink></li>
</ul>
</div></template>


