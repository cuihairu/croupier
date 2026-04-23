<template><div><h1 id="分层设计" tabindex="-1"><a class="header-anchor" href="#分层设计"><span>分层设计</span></a></h1>
<p>Croupier 采用<strong>五层分布式架构</strong>，实现权限控制、函数路由、边缘代理和 UI 展示的完全分离。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#五层架构概览">五层架构概览</router-link><ul><li><router-link to="#部署场景">部署场景</router-link></li></ul></li><li><router-link to="#第一层-展示层-display-layer">第一层：展示层 (Display Layer)</router-link><ul><li><router-link to="#职责">职责</router-link></li><li><router-link to="#组件">组件</router-link></li><li><router-link to="#描述符驱动-ui">描述符驱动 UI</router-link></li></ul></li><li><router-link to="#第二层-控制层-control-layer">第二层：控制层 (Control Layer)</router-link><ul><li><router-link to="#职责-1">职责</router-link></li><li><router-link to="#核心模块">核心模块</router-link></li><li><router-link to="#server-组件">Server 组件</router-link></li></ul></li><li><router-link to="#第三层-边缘层-edge-layer">第三层：边缘层 (Edge Layer)</router-link><ul><li><router-link to="#edge-proxy">Edge Proxy</router-link></li><li><router-link to="#edge-职责">Edge 职责</router-link></li></ul></li><li><router-link to="#第四层-代理层-agent-layer">第四层：代理层 (Agent Layer)</router-link><ul><li><router-link to="#agent-架构">Agent 架构</router-link></li><li><router-link to="#agent-特性">Agent 特性</router-link></li></ul></li><li><router-link to="#第五层-业务层-service-layer">第五层：业务层 (Service Layer)</router-link><ul><li><router-link to="#game-server-sdk">Game Server + SDK</router-link></li><li><router-link to="#sdk-职责">SDK 职责</router-link></li></ul></li><li><router-link to="#层间通信">层间通信</router-link><ul><li><router-link to="#通信协议">通信协议</router-link></li><li><router-link to="#数据格式">数据格式</router-link></li></ul></li><li><router-link to="#分层优势">分层优势</router-link><ul><li><router-link to="#_1-关注点分离">1. 关注点分离</router-link></li><li><router-link to="#_2-独立部署">2. 独立部署</router-link></li><li><router-link to="#_3-安全隔离">3. 安全隔离</router-link></li><li><router-link to="#_4-技术栈灵活">4. 技术栈灵活</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="五层架构概览" tabindex="-1"><a class="header-anchor" href="#五层架构概览"><span>五层架构概览</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                   第一层：展示层 (Display Layer)             │</span>
<span class="line">│  ┌──────────────────────────────────────────────────────┐ │</span>
<span class="line">│  │  Web Dashboard (React + Ant Design + ProComponents)│ │</span>
<span class="line">│  │  - 描述符驱动 UI 自动生成                            │ │</span>
<span class="line">│  │  - 实时进度与日志流式展示                            │ │</span>
<span class="line">│  │  - 审批流程可视化                                    │ │</span>
<span class="line">│  └──────────────────────────────────────────────────────┘ │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              ▼ HTTPS/TLS (8080)</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                   第二层：控制层 (Control Layer)             │</span>
<span class="line">│  ┌──────────────────────────────────────────────────────┐ │</span>
<span class="line">│  │  Server (HTTP + gRPC)                                │ │</span>
<span class="line">│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │ │</span>
<span class="line">│  │  │ RBAC/ABAC   │  │ 函数路由    │  │ 审计日志    │ │ │</span>
<span class="line">│  │  │ 权限引擎    │  │ 负载均衡    │  │ 审批流程    │ │ │</span>
<span class="line">│  │  └─────────────┘  └─────────────┘  └─────────────┘ │ │</span>
<span class="line">│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │ │</span>
<span class="line">│  │  │ 描述符管理  │  │ Provider    │  │ Pack 管理   │ │ │</span>
<span class="line">│  │  │              │  │ Manifest    │  │             │ │ │</span>
<span class="line">│  │  └─────────────┘  └─────────────┘  └─────────────┘ │ │</span>
<span class="line">│  └──────────────────────────────────────────────────────┘ │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              ▼ mTLS (8443)</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                   第三层：边缘层 (Edge Layer)                │</span>
<span class="line">│  ┌──────────────────────────────────────────────────────┐ │</span>
<span class="line">│  │  Edge Proxy (可选，DMZ 部署)                         │ │</span>
<span class="line">│  │  - 双向隧道转发                                      │ │</span>
<span class="line">│  │  - 隧道复用 (多路复用)                               │ │</span>
<span class="line">│  │  - 流量转发 (Server ↔ Agent)                        │ │</span>
<span class="line">│  │  - 连接管理与故障恢复                                │ │</span>
<span class="line">│  └──────────────────────────────────────────────────────┘ │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              ▼ mTLS (内部)</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                   第四层：代理层 (Agent Layer)               │</span>
<span class="line">│  ┌──────────────────────────────────────────────────────┐ │</span>
<span class="line">│  │  Agent (游戏内网部署)                                 │ │</span>
<span class="line">│  │  ┌────────────────────────────────────────────────┐ │ │</span>
<span class="line">│  │  │ Function Registry                              │ │ │</span>
<span class="line">│  │  │ - 函数注册/注销                                │ │ │</span>
<span class="line">│  │  │ - 心跳保活                                     │ │ │</span>
<span class="line">│  │  │ - 进程管理 (ProcessSession)                    │ │ │</span>
<span class="line">│  │  └────────────────────────────────────────────────┘ │ │</span>
<span class="line">│  │  ┌────────────────────────────────────────────────┐ │ │</span>
<span class="line">│  │  │ Job Executor                                  │ │ │</span>
<span class="line">│  │  │ - 同步调用 (Invoke)                           │ │ │</span>
<span class="line">│  │  │ - 异步作业 (StartJob)                         │ │ │</span>
<span class="line">│  │  │ - 流式事件 (StreamJob)                        │ │ │</span>
<span class="line">│  │  │ - 作业取消 (CancelJob)                         │ │ │</span>
<span class="line">│  │  └────────────────────────────────────────────────┘ │ │</span>
<span class="line">│  │  ┌────────────────────────────────────────────────┐ │ │</span>
<span class="line">│  │  │ Local Control Service                          │ │ │</span>
<span class="line">│  │  │ - SDK 注册 (RegisterService)                   │ │ │</span>
<span class="line">│  │  │ - 本地服务发现                                 │ │ │</span>
<span class="line">│  │  └────────────────────────────────────────────────┘ │ │</span>
<span class="line">│  └──────────────────────────────────────────────────────┘ │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              ▼ gRPC (19090)</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│                  第五层：业务层 (Service Layer)              │</span>
<span class="line">│  ┌──────────────────────────────────────────────────────┐ │</span>
<span class="line">│  │  Game Server + SDK                                   │ │</span>
<span class="line">│  │  - 函数实现 (业务逻辑)                               │ │</span>
<span class="line">│  │  - 函数定义 (Proto / 描述符)                         │ │</span>
<span class="line">│  │  - gRPC 服务端                                      │ │</span>
<span class="line">│  └──────────────────────────────────────────────────────┘ │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="部署场景" tabindex="-1"><a class="header-anchor" href="#部署场景"><span>部署场景</span></a></h3>
<h4 id="场景-1-简化部署-无-edge" tabindex="-1"><a class="header-anchor" href="#场景-1-简化部署-无-edge"><span>场景 1: 简化部署 (无 Edge)</span></a></h4>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Dashboard ──HTTPS──> Server ──mTLS──> Agent ──gRPC──> Game Server</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h4 id="场景-2-标准部署-有-edge" tabindex="-1"><a class="header-anchor" href="#场景-2-标准部署-有-edge"><span>场景 2: 标准部署 (有 Edge)</span></a></h4>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Dashboard ──HTTPS──> Server ──mTLS──> Edge ──mTLS──> Agent ──gRPC──> Game Server</span>
<span class="line">                         ↑                        ↑</span>
<span class="line">                    (内网)                 (DMZ/公网)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第一层-展示层-display-layer" tabindex="-1"><a class="header-anchor" href="#第一层-展示层-display-layer"><span>第一层：展示层 (Display Layer)</span></a></h2>
<h3 id="职责" tabindex="-1"><a class="header-anchor" href="#职责"><span>职责</span></a></h3>
<ul>
<li>用户界面呈现</li>
<li>表单自动生成</li>
<li>实时数据展示</li>
<li>审批流程交互</li>
</ul>
<h3 id="组件" tabindex="-1"><a class="header-anchor" href="#组件"><span>组件</span></a></h3>
<table>
<thead>
<tr>
<th>组件</th>
<th>技术栈</th>
<th>职责</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Dashboard</strong></td>
<td>React + Ant Design</td>
<td>Web 管理界面</td>
</tr>
<tr>
<td><strong>X-Render</strong></td>
<td>FormRender</td>
<td>表单自动生成</td>
</tr>
<tr>
<td><strong>ProTable</strong></td>
<td>Ant Design Pro</td>
<td>列表自动生成</td>
</tr>
</tbody>
</table>
<h3 id="描述符驱动-ui" tabindex="-1"><a class="header-anchor" href="#描述符驱动-ui"><span>描述符驱动 UI</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token comment">// 函数描述符 → 自动生成 UI</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span> <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"玩家ID"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span> <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"封禁时长"</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>自动生成的 UI：</p>
<ul>
<li>表单字段</li>
<li>验证规则</li>
<li>风险提示</li>
<li>提交确认</li>
</ul>
<h2 id="第二层-控制层-control-layer" tabindex="-1"><a class="header-anchor" href="#第二层-控制层-control-layer"><span>第二层：控制层 (Control Layer)</span></a></h2>
<h3 id="职责-1" tabindex="-1"><a class="header-anchor" href="#职责-1"><span>职责</span></a></h3>
<ul>
<li>权限控制 (RBAC/ABAC)</li>
<li>函数路由与负载均衡</li>
<li>审批工作流</li>
<li>审计日志记录</li>
</ul>
<h3 id="核心模块" tabindex="-1"><a class="header-anchor" href="#核心模块"><span>核心模块</span></a></h3>
<h4 id="_1-权限引擎" tabindex="-1"><a class="header-anchor" href="#_1-权限引擎"><span>1. 权限引擎</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 权限检查流程</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>e <span class="token operator">*</span>Engine<span class="token punctuation">)</span> <span class="token function">Check</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> req <span class="token operator">*</span>CheckRequest<span class="token punctuation">)</span> <span class="token operator">*</span>CheckResponse <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 1. RBAC 检查</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token operator">!</span>e<span class="token punctuation">.</span>rbac<span class="token punctuation">.</span><span class="token function">HasPermission</span><span class="token punctuation">(</span>ctx<span class="token punctuation">.</span>User<span class="token punctuation">,</span> req<span class="token punctuation">.</span>Permission<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">Denied</span><span class="token punctuation">(</span><span class="token string">"missing permission"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 2. ABAC 检查</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token operator">!</span>e<span class="token punctuation">.</span>abac<span class="token punctuation">.</span><span class="token function">Evaluate</span><span class="token punctuation">(</span>ctx<span class="token punctuation">.</span>User<span class="token punctuation">,</span> req<span class="token punctuation">.</span>Attributes<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">Denied</span><span class="token punctuation">(</span><span class="token string">"abac denied"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 3. 审批检查</span></span>
<span class="line">    <span class="token keyword">if</span> e<span class="token punctuation">.</span>approval<span class="token punctuation">.</span><span class="token function">Required</span><span class="token punctuation">(</span>req<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">PendingApproval</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token function">Approved</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_2-函数路由" tabindex="-1"><a class="header-anchor" href="#_2-函数路由"><span>2. 函数路由</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 路由策略</span></span>
<span class="line"><span class="token keyword">type</span> Router <span class="token keyword">interface</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">SelectAgent</span><span class="token punctuation">(</span>functionID <span class="token builtin">string</span><span class="token punctuation">,</span> agents <span class="token punctuation">[</span><span class="token punctuation">]</span>Agent<span class="token punctuation">)</span> <span class="token punctuation">(</span>Agent<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> RoundRobinRouter <span class="token keyword">struct</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token keyword">type</span> ConsistentHashRouter <span class="token keyword">struct</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token keyword">type</span> LeastConnectionRouter <span class="token keyword">struct</span><span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_3-审计日志" tabindex="-1"><a class="header-anchor" href="#_3-审计日志"><span>3. 审计日志</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">type</span> AuditLog <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    AuditID   <span class="token builtin">string</span></span>
<span class="line">    Timestamp time<span class="token punctuation">.</span>Time</span>
<span class="line">    User      UserInfo</span>
<span class="line">    Action    <span class="token builtin">string</span></span>
<span class="line">    Target    <span class="token builtin">string</span></span>
<span class="line">    Result    <span class="token builtin">string</span></span>
<span class="line">    Hash      <span class="token builtin">string</span>  <span class="token comment">// 防篡改</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="server-组件" tabindex="-1"><a class="header-anchor" href="#server-组件"><span>Server 组件</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Server</span>
<span class="line">├── HTTP API Server    :8080</span>
<span class="line">│   ├── REST API</span>
<span class="line">│   ├── WebSocket</span>
<span class="line">│   └── SSE Events</span>
<span class="line">│</span>
<span class="line">├── gRPC Server        :8443</span>
<span class="line">│   ├── ControlService  (Agent 管理)</span>
<span class="line">│   ├── FunctionService (函数调用)</span>
<span class="line">│   └── RegistryService (注册中心)</span>
<span class="line">│</span>
<span class="line">├── Auth Engine</span>
<span class="line">│   ├── RBAC</span>
<span class="line">│   ├── ABAC</span>
<span class="line">│   └── JWT/OIDC</span>
<span class="line">│</span>
<span class="line">├── Approval Engine</span>
<span class="line">│   ├── Two-person rule</span>
<span class="line">│   └── Workflow</span>
<span class="line">│</span>
<span class="line">└── Audit Log</span>
<span class="line">    ├── Events</span>
<span class="line">    ├── Chain</span>
<span class="line">    └── Sensitive masking</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第三层-边缘层-edge-layer" tabindex="-1"><a class="header-anchor" href="#第三层-边缘层-edge-layer"><span>第三层：边缘层 (Edge Layer)</span></a></h2>
<h3 id="edge-proxy" tabindex="-1"><a class="header-anchor" href="#edge-proxy"><span>Edge Proxy</span></a></h3>
<p>当 Server 在内网，需要通过 Edge 暴露到公网时使用。</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">公网 ──TLS──> Edge ──mTLS──> Server ──mTLS──> Agent</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="edge-职责" tabindex="-1"><a class="header-anchor" href="#edge-职责"><span>Edge 职责</span></a></h3>
<table>
<thead>
<tr>
<th>功能</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>隧道复用</strong></td>
<td>多个连接共享隧道</td>
</tr>
<tr>
<td><strong>流量转发</strong></td>
<td>请求/响应转发</td>
</tr>
<tr>
<td><strong>连接管理</strong></td>
<td>保持长连接池</td>
</tr>
<tr>
<td><strong>安全隔离</strong></td>
<td>DMZ 部署</td>
</tr>
</tbody>
</table>
<h2 id="第四层-代理层-agent-layer" tabindex="-1"><a class="header-anchor" href="#第四层-代理层-agent-layer"><span>第四层：代理层 (Agent Layer)</span></a></h2>
<h3 id="agent-架构" tabindex="-1"><a class="header-anchor" href="#agent-架构"><span>Agent 架构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Agent</span>
<span class="line">├── gRPC Client (出站)</span>
<span class="line">│   └── 连接 Server (mTLS)</span>
<span class="line">│</span>
<span class="line">├── gRPC Server (本地)</span>
<span class="line">│   └── :19090 (LocalControlService)</span>
<span class="line">│</span>
<span class="line">├── Function Registry</span>
<span class="line">│   ├── 注册函数</span>
<span class="line">│   ├── 心跳保活</span>
<span class="line">│   ├── 更新通知</span>
<span class="line">│   └── 进程管理 (ProcessSession)</span>
<span class="line">│</span>
<span class="line">├── Job Executor</span>
<span class="line">│   ├── 同步调用 (Invoke)</span>
<span class="line">│   ├── 异步作业 (StartJob)</span>
<span class="line">│   ├── 流式事件 (StreamJob)</span>
<span class="line">│   └── 作业取消 (CancelJob)</span>
<span class="line">│</span>
<span class="line">└── Downloader</span>
<span class="line">    └── 函数包下载</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="agent-特性" tabindex="-1"><a class="header-anchor" href="#agent-特性"><span>Agent 特性</span></a></h3>
<table>
<thead>
<tr>
<th>特性</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>出站连接</strong></td>
<td>主动连接 Server，穿透内网</td>
</tr>
<tr>
<td><strong>函数注册</strong></td>
<td>向 Server 注册本地函数</td>
</tr>
<tr>
<td><strong>调用转发</strong></td>
<td>转发 Server 调用到 Game Server</td>
</tr>
<tr>
<td><strong>作业执行</strong></td>
<td>支持长时间运行的异步作业</td>
</tr>
<tr>
<td><strong>热重载</strong></td>
<td>函数更新无需重启</td>
</tr>
</tbody>
</table>
<h2 id="第五层-业务层-service-layer" tabindex="-1"><a class="header-anchor" href="#第五层-业务层-service-layer"><span>第五层：业务层 (Service Layer)</span></a></h2>
<h3 id="game-server-sdk" tabindex="-1"><a class="header-anchor" href="#game-server-sdk"><span>Game Server + SDK</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Game Server</span>
<span class="line">└── Croupier SDK</span>
<span class="line">    ├── 函数定义</span>
<span class="line">    ├── 函数实现</span>
<span class="line">    ├── gRPC 客户端</span>
<span class="line">    └── 类型安全</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="sdk-职责" tabindex="-1"><a class="header-anchor" href="#sdk-职责"><span>SDK 职责</span></a></h3>
<ol>
<li><strong>函数定义</strong>：定义函数接口</li>
<li><strong>函数实现</strong>：实现业务逻辑</li>
<li><strong>类型安全</strong>：编译时类型检查</li>
<li><strong>自动重连</strong>：连接断开自动重连</li>
</ol>
<h2 id="层间通信" tabindex="-1"><a class="header-anchor" href="#层间通信"><span>层间通信</span></a></h2>
<h3 id="通信协议" tabindex="-1"><a class="header-anchor" href="#通信协议"><span>通信协议</span></a></h3>
<table>
<thead>
<tr>
<th>层级</th>
<th>协议</th>
<th>端口</th>
<th>安全</th>
</tr>
</thead>
<tbody>
<tr>
<td>Dashboard → Server</td>
<td>HTTPS (REST)</td>
<td>8080</td>
<td>TLS</td>
</tr>
<tr>
<td>Server ↔ Agent</td>
<td>gRPC</td>
<td>8443</td>
<td>mTLS</td>
</tr>
<tr>
<td>Server ↔ Edge</td>
<td>gRPC</td>
<td>8443</td>
<td>mTLS</td>
</tr>
<tr>
<td>Edge ↔ Agent</td>
<td>gRPC (Tunnel)</td>
<td>动态</td>
<td>mTLS</td>
</tr>
<tr>
<td>Agent → Game Server</td>
<td>gRPC (Local)</td>
<td>19090</td>
<td>可选 mTLS</td>
</tr>
<tr>
<td>SDK → Agent</td>
<td>gRPC (Local)</td>
<td>19090</td>
<td>可选 mTLS</td>
</tr>
</tbody>
</table>
<h3 id="数据格式" tabindex="-1"><a class="header-anchor" href="#数据格式"><span>数据格式</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token comment">// 函数调用请求</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeFunctionRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> game_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> env <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> function_id <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> payload <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">InvokeOptions</span> options <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 函数调用响应</span></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">InvokeFunctionResponse</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">bool</span> success <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">google<span class="token punctuation">.</span>protobuf<span class="token punctuation">.</span>Struct</span> result <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> error <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="分层优势" tabindex="-1"><a class="header-anchor" href="#分层优势"><span>分层优势</span></a></h2>
<h3 id="_1-关注点分离" tabindex="-1"><a class="header-anchor" href="#_1-关注点分离"><span>1. 关注点分离</span></a></h3>
<ul>
<li>展示层专注 UI</li>
<li>控制层专注安全和路由</li>
<li>业务层专注游戏逻辑</li>
</ul>
<h3 id="_2-独立部署" tabindex="-1"><a class="header-anchor" href="#_2-独立部署"><span>2. 独立部署</span></a></h3>
<ul>
<li>Dashboard 可独立部署</li>
<li>Server 可水平扩展</li>
<li>Agent 随游戏服务器部署</li>
</ul>
<h3 id="_3-安全隔离" tabindex="-1"><a class="header-anchor" href="#_3-安全隔离"><span>3. 安全隔离</span></a></h3>
<ul>
<li>层间强制认证</li>
<li>最小权限原则</li>
<li>审计完整追溯</li>
</ul>
<h3 id="_4-技术栈灵活" tabindex="-1"><a class="header-anchor" href="#_4-技术栈灵活"><span>4. 技术栈灵活</span></a></h3>
<ul>
<li>各层可使用不同技术</li>
<li>接口通过 Protobuf 定义</li>
<li>语言无关的集成</li>
</ul>
<h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/architecture/components.html">组件说明</RouteLink></li>
<li><RouteLink to="/architecture/data-flow.html">数据流</RouteLink></li>
<li><RouteLink to="/architecture/design-patterns.html">设计模式</RouteLink></li>
</ul>
</div></template>


