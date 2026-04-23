<template><div><h1 id="数据流" tabindex="-1"><a class="header-anchor" href="#数据流"><span>数据流</span></a></h1>
<p>本文档详细说明 Croupier 系统中的调用流、数据流转和事件处理。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#标准调用流程">标准调用流程</router-link><ul><li><router-link to="#端到端流程">端到端流程</router-link></li><li><router-link to="#详细步骤">详细步骤</router-link></li></ul></li><li><router-link to="#同步调用流">同步调用流</router-link><ul><li><router-link to="#请求格式">请求格式</router-link></li><li><router-link to="#响应格式">响应格式</router-link></li></ul></li><li><router-link to="#异步调用流-作业">异步调用流（作业）</router-link><ul><li><router-link to="#异步调用流程">异步调用流程</router-link></li><li><router-link to="#作业事件类型">作业事件类型</router-link></li><li><router-link to="#作业管理-api">作业管理 API</router-link></li></ul></li><li><router-link to="#审批流程">审批流程</router-link><ul><li><router-link to="#审批数据流">审批数据流</router-link></li><li><router-link to="#审批请求格式">审批请求格式</router-link></li><li><router-link to="#审批操作">审批操作</router-link></li></ul></li><li><router-link to="#隧道模式-经-edge">隧道模式（经 Edge）</router-link><ul><li><router-link to="#隧道调用流程">隧道调用流程</router-link></li><li><router-link to="#隧道复用">隧道复用</router-link></li></ul></li><li><router-link to="#广播模式">广播模式</router-link><ul><li><router-link to="#广播调用流程">广播调用流程</router-link></li><li><router-link to="#广播调用示例">广播调用示例</router-link></li></ul></li><li><router-link to="#审计数据流">审计数据流</router-link><ul><li><router-link to="#审计记录流程">审计记录流程</router-link></li><li><router-link to="#审计事件结构">审计事件结构</router-link></li></ul></li><li><router-link to="#实时数据流">实时数据流</router-link><ul><li><router-link to="#websocket-sse">WebSocket / SSE</router-link></li><li><router-link to="#事件类型">事件类型</router-link></li></ul></li><li><router-link to="#错误处理流">错误处理流</router-link><ul><li><router-link to="#错误传播">错误传播</router-link></li><li><router-link to="#错误响应格式">错误响应格式</router-link></li></ul></li><li><router-link to="#数据格式转换">数据格式转换</router-link><ul><li><router-link to="#json-↔-protobuf">JSON ↔ Protobuf</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="标准调用流程" tabindex="-1"><a class="header-anchor" href="#标准调用流程"><span>标准调用流程</span></a></h2>
<h3 id="端到端流程" tabindex="-1"><a class="header-anchor" href="#端到端流程"><span>端到端流程</span></a></h3>
<Mermaid code="eJx9kctOAjEUhvc8RcPesJ8FCUI040bCJa4rNjgxzIwzwBpkAahREjFoQNAoRowZZ6VBojxN2+Et7MULpIQu2ub8X//8PcdFhyVk5lDCgHkHFkKALRs6RSNn2NAsgqyLHABdQNtPpPGuyjoXE9Dd37Wgs6foaeSUpUHcsUq2we6ypJCxPGL7PCgqCrcJC4hj4vzxEhBPuhaNZnUN0KMxrk/C2K/Sxyo9G2HvLSwZnRHykQaS2+kMiEDbiBhm2TpAgpDiHDV7Pg1eq2BrJwMyDDKXU6n1WDwSYxsgN7XZdYvcV8hguJzF3h1pjlcRs0qTnIxo1yO39bk2/HGipIF8KhkHusi+UTJzRcOS6YTMMN4iDZDeC+75Eg78GhukgLj4bxVM27jbp5ML0u8tePxGUgAldOD5+POSfS5g/+sM8bSzyPHBKC5iIHxyLOfVF334II0WPh6Q8xa7h74BA8oJZg=="></Mermaid><h3 id="详细步骤" tabindex="-1"><a class="header-anchor" href="#详细步骤"><span>详细步骤</span></a></h3>
<table>
<thead>
<tr>
<th>步骤</th>
<th>操作</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td>1</td>
<td>用户操作</td>
<td>在 Dashboard 点击操作按钮</td>
</tr>
<tr>
<td>2</td>
<td>HTTP 请求</td>
<td>发送 POST /api/invoke</td>
</tr>
<tr>
<td>3</td>
<td>身份验证</td>
<td>验证 JWT Token</td>
</tr>
<tr>
<td>4</td>
<td>权限检查</td>
<td>RBAC/ABAC 验证</td>
</tr>
<tr>
<td>5</td>
<td>审批检查</td>
<td>检查是否需要审批</td>
</tr>
<tr>
<td>6</td>
<td>路由选择</td>
<td>选择目标 Agent</td>
</tr>
<tr>
<td>7</td>
<td>gRPC 调用</td>
<td>调用 Agent 的 InvokeFunction</td>
</tr>
<tr>
<td>8</td>
<td>业务执行</td>
<td>Game Server 执行业务逻辑</td>
</tr>
<tr>
<td>9</td>
<td>结果返回</td>
<td>逐层返回结果</td>
</tr>
<tr>
<td>10</td>
<td>审计记录</td>
<td>记录操作审计日志</td>
</tr>
</tbody>
</table>
<h2 id="同步调用流" tabindex="-1"><a class="header-anchor" href="#同步调用流"><span>同步调用流</span></a></h2>
<h3 id="请求格式" tabindex="-1"><a class="header-anchor" href="#请求格式"><span>请求格式</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line">POST /api/invoke</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"player_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token number">24</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token string">"作弊"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"options"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"idempotency_key"</span><span class="token operator">:</span> <span class="token string">"unique-key-123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"timeout"</span><span class="token operator">:</span> <span class="token string">"30s"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="响应格式" tabindex="-1"><a class="header-anchor" href="#响应格式"><span>响应格式</span></a></h3>
<p><strong>成功响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"ban_id"</span><span class="token operator">:</span> <span class="token string">"ban_456"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"expires_at"</span><span class="token operator">:</span> <span class="token string">"2024-12-02T10:30:00Z"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>需要审批</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"pending_approval"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"approval_id"</span><span class="token operator">:</span> <span class="token string">"approval_789"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"操作需要双人审批"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>权限拒绝</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"code"</span><span class="token operator">:</span> <span class="token string">"PERMISSION_DENIED"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"没有权限执行该操作"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"required_permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="异步调用流-作业" tabindex="-1"><a class="header-anchor" href="#异步调用流-作业"><span>异步调用流（作业）</span></a></h2>
<h3 id="异步调用流程" tabindex="-1"><a class="header-anchor" href="#异步调用流程"><span>异步调用流程</span></a></h3>
<Mermaid code="eJx1jz1Pg0Acxnc/xY06GPYOTYyQBgdpiszmKJd6ajmElsWYWKcaG+OgDmJMHHxZbDqoMdSmXwYu+C28F4JU9Aa4/J/f/Z/nCdBBH7ltpGLY8WF3CbDjQb+H29iDbg9YOoABUGGwYxPoOxXdRH6IfM6s+6TvYXaXowq51kHsWwbFpMI1YBdxTPzzXQKy9NV6XQ5qoGmYW0CBHlZ2iR0IXUolJh1G6TROZrfJx002nqSzqzLHQEuvgWx+mUZ3gG3ZxrLego9paj82yiF2jhQUstiBzFR4ii41oLsh2UMbxBaqGDKRV2FxPk/oywM9fcruR/L1PiEekPmS+CyZvtO3gRD44Y94yHxzNo/S+JFGr/R6UjDSoBSXOWs83nKzZTRammmuFOxibd6Lnj9/HQ+ks8CQ68hgv7xl5nQ8osOLUrG/fFVjU5Oe//nla74B8jDlqQ=="></Mermaid><h3 id="作业事件类型" tabindex="-1"><a class="header-anchor" href="#作业事件类型"><span>作业事件类型</span></a></h3>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">enum</span> <span class="token class-name">EventType</span> <span class="token punctuation">{</span></span>
<span class="line">  START    <span class="token operator">=</span> <span class="token number">0</span><span class="token punctuation">;</span>  <span class="token comment">// 作业开始</span></span>
<span class="line">  PROGRESS <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// 进度更新</span></span>
<span class="line">  LOG      <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span>  <span class="token comment">// 日志输出</span></span>
<span class="line">  DONE     <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>  <span class="token comment">// 作业完成</span></span>
<span class="line">  ERROR    <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>  <span class="token comment">// 作业错误</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">JobEvent</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">string</span> job_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token positional-class-name class-name">EventType</span> type <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">string</span> message <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token builtin">double</span> progress <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span>  <span class="token comment">// 0.0 - 1.0</span></span>
<span class="line">  <span class="token builtin">int64</span> timestamp <span class="token operator">=</span> <span class="token number">5</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="作业管理-api" tabindex="-1"><a class="header-anchor" href="#作业管理-api"><span>作业管理 API</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 创建异步作业</span></span>
<span class="line">POST /api/jobs</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"function_id"</span><span class="token builtin class-name">:</span> <span class="token string">"data.export"</span>,</span>
<span class="line">  <span class="token string">"payload"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span><span class="token punctuation">..</span>.<span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"><span class="token comment"># 返回: {"job_id": "job_123"}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 获取作业状态</span></span>
<span class="line">GET /api/jobs/job_123</span>
<span class="line"><span class="token comment"># 返回: {"status": "running", "progress": 0.5}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 流式获取事件</span></span>
<span class="line">GET /api/jobs/job_123/events</span>
<span class="line"><span class="token comment"># SSE 流式事件</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 取消作业</span></span>
<span class="line">DELETE /api/jobs/job_123</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="审批流程" tabindex="-1"><a class="header-anchor" href="#审批流程"><span>审批流程</span></a></h2>
<h3 id="审批数据流" tabindex="-1"><a class="header-anchor" href="#审批数据流"><span>审批数据流</span></a></h3>
<Mermaid code="eJwrLkksSXXJTEwvSszVLTPiUgCCaK1YBV1dO4Vn/ROe7FpipfB8yopnHdshvBcbmoFcsDKIAETl4oZn85daKQSnFpWlFkG5L1t7n+9dB1EKFgArfbqv9em6hc86d1opvJzT8GJZ49P+nie7dkHE0BU/n73lWd/SZ53LXyzssVJ4Nn0BUAtUJVgp3DCI0ds3vWyY9WJ/O9DJa9Y82YVmLobiZ92Tnu+eCzQXTGNX87R/2rNtHVYKL7a1Ppu+DWorzB6I36Gue7qn4enybggPrAzZ8bhVQryMUPR0Xc+zjglAR4GFgMynXfMx1CzZ+GILMLghQhAe1Glg3WBFwFiECIGlUYVgfkcXhfgWLgoAoU8Okg=="></Mermaid><h3 id="审批请求格式" tabindex="-1"><a class="header-anchor" href="#审批请求格式"><span>审批请求格式</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line">POST /api/approvals</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token string">"玩家使用外挂"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"requested_by"</span><span class="token operator">:</span> <span class="token string">"user_123"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审批操作" tabindex="-1"><a class="header-anchor" href="#审批操作"><span>审批操作</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 审批通过</span></span>
<span class="line">POST /api/approvals/<span class="token punctuation">{</span>id<span class="token punctuation">}</span>/approve</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"approved_by"</span><span class="token builtin class-name">:</span> <span class="token string">"user_456"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 审批拒绝</span></span>
<span class="line">POST /api/approvals/<span class="token punctuation">{</span>id<span class="token punctuation">}</span>/reject</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"rejected_by"</span><span class="token builtin class-name">:</span> <span class="token string">"user_456"</span>,</span>
<span class="line">  <span class="token string">"reason"</span><span class="token builtin class-name">:</span> <span class="token string">"证据不足"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="隧道模式-经-edge" tabindex="-1"><a class="header-anchor" href="#隧道模式-经-edge"><span>隧道模式（经 Edge）</span></a></h2>
<h3 id="隧道调用流程" tabindex="-1"><a class="header-anchor" href="#隧道调用流程"><span>隧道调用流程</span></a></h3>
<Mermaid code="eJwrTi0sTc1LTnXJTEwvSszlUgCCgsSikszkzILEvBKFUE+FxGIFl8TijKT8xKIUDPng1KKy1CKQGihL42lb6/O9EzUxVLqmpKeC1IFpDRffKEwljumpQBKoBsLQeLZjx7OOflwGuifmgg0E0xDbucCKQj117ewgAlYKHiEhAcEKAf7BIQr6iQWZ+pl5ZfnZqWB1ECVIal9sX/98ysanbZufr52moPFyTsOLZY3Pd/eDnQyxH64FJGSlkB4U4Kzgll9UDgwaT4TBIEmgGrAvrBRezlr+snHykz0LXuybrABR5Vaal1ySmZ8HVg1WBlQO8oeVwrM5a57O2fBiQ/PzKSvA0iBhXYRpz3dPfjZvDpJGuGOQ7UFSBXYMsif3T3k6ex6SAqifgEpCPdGkAZO50hU="></Mermaid><h3 id="隧道复用" tabindex="-1"><a class="header-anchor" href="#隧道复用"><span>隧道复用</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">单一隧道连接复用多个请求：</span>
<span class="line"></span>
<span class="line">Server &lt;---> Edge</span>
<span class="line">    |</span>
<span class="line">    +-- Tunnel 1 --> Agent 1 --> Game Server A</span>
<span class="line">    |                       +--> Game Server B</span>
<span class="line">    |</span>
<span class="line">    +-- Tunnel 2 --> Agent 2 --> Game Server C</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="广播模式" tabindex="-1"><a class="header-anchor" href="#广播模式"><span>广播模式</span></a></h2>
<h3 id="广播调用流程" tabindex="-1"><a class="header-anchor" href="#广播调用流程"><span>广播调用流程</span></a></h3>
<Mermaid code="eJxLL0osyFDwCeJSAALH6ODUorLUolgFXV27Gs+8svzs1BoFJ8Nox/TUvBIFw1iIKlRZI6isEVZZY6iscSwXWNrJECSv4G4Y7Z6Ymwoz0skIImoEEYUa5WQMETWGiMKMANugEBT9onHW0wkdz3dPfjZvTiwXAAwjOPU="></Mermaid><h3 id="广播调用示例" tabindex="-1"><a class="header-anchor" href="#广播调用示例"><span>广播调用示例</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">POST /api/invoke</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"function_id"</span><span class="token builtin class-name">:</span> <span class="token string">"config.reload"</span>,</span>
<span class="line">  <span class="token string">"routing"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token string">"mode"</span><span class="token builtin class-name">:</span> <span class="token string">"broadcast"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="审计数据流" tabindex="-1"><a class="header-anchor" href="#审计数据流"><span>审计数据流</span></a></h2>
<h3 id="审计记录流程" tabindex="-1"><a class="header-anchor" href="#审计记录流程"><span>审计记录流程</span></a></h3>
<Mermaid code="eJwrTi0sTc1LTnXJTEwvSszlUgCCgsSikszkzILEvBKF0OLUIoXEYoXnU1Y869iOIR2cWlQGUQBhYShwLE3JLAHJP1238MW6hc/m9D7tWohpTEl+UWJ6Kljd2hlPm1ZwgZWALNe1s4MYbaXwYkMz0BlP2/c+m7oBLA+RAKoA2wJUsG7D071TITY92dX9ZPc2BY3gEMegEE2wcrAqkHkQ26wUnvU0PtnZ+rRnGsQ6uHEwC591Ln+xsOfJjllAN79s2P1i30SirXX29w3wcQ1xJc5mmDzcwIXP101/Ornj6Y6el5P3cQEAZLeyQQ=="></Mermaid><h3 id="审计事件结构" tabindex="-1"><a class="header-anchor" href="#审计事件结构"><span>审计事件结构</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"audit_id"</span><span class="token operator">:</span> <span class="token string">"audit_20241201_001"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"timestamp"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:30:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"action"</span><span class="token operator">:</span> <span class="token string">"function.invoke"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload_preview"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"***"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token number">24</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token string">"success"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ip"</span><span class="token operator">:</span> <span class="token string">"192.168.1.100"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ip_region"</span><span class="token operator">:</span> <span class="token string">"中国 上海"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"hash"</span><span class="token operator">:</span> <span class="token string">"sha256(...)"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"prev_hash"</span><span class="token operator">:</span> <span class="token string">"sha256(...)"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="实时数据流" tabindex="-1"><a class="header-anchor" href="#实时数据流"><span>实时数据流</span></a></h2>
<h3 id="websocket-sse" tabindex="-1"><a class="header-anchor" href="#websocket-sse"><span>WebSocket / SSE</span></a></h3>
<Mermaid code="eJwrTi0sTc1LTnXJTEwvSszlUgCCgsSikszkzILEvBIF55zMVCCVWKzwtLP3+er1GAqCU4vKUotACiAsLrAKiDZdOzuIoJVCcLCrgrtriIJ+YkGmfmoZULIYrBAirwtUCdFipfB0967nq7tf7J/3rG8pxLCc/PwChSe7up/s3vasb8XLhkawKJJuhDXPFjc829r9tGMDRDm6QiRrUhJLEq0UqvX09GrBqlLzUnA4/Wnr5pfT10IdBAC+sXJ5"></Mermaid><h3 id="事件类型" tabindex="-1"><a class="header-anchor" href="#事件类型"><span>事件类型</span></a></h3>
<table>
<thead>
<tr>
<th>事件类型</th>
<th>说明</th>
<th>示例</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>function.called</code></td>
<td>函数被调用</td>
<td><code v-pre>{function_id, user, result}</code></td>
</tr>
<tr>
<td><code v-pre>job.progress</code></td>
<td>作业进度更新</td>
<td><code v-pre>{job_id, progress, message}</code></td>
</tr>
<tr>
<td><code v-pre>approval.pending</code></td>
<td>待审批请求</td>
<td><code v-pre>{approval_id, function_id}</code></td>
</tr>
<tr>
<td><code v-pre>agent.connected</code></td>
<td>Agent 上线</td>
<td><code v-pre>{agent_id, game_id}</code></td>
</tr>
<tr>
<td><code v-pre>agent.disconnected</code></td>
<td>Agent 下线</td>
<td><code v-pre>{agent_id, reason}</code></td>
</tr>
</tbody>
</table>
<h2 id="错误处理流" tabindex="-1"><a class="header-anchor" href="#错误处理流"><span>错误处理流</span></a></h2>
<h3 id="错误传播" tabindex="-1"><a class="header-anchor" href="#错误传播"><span>错误传播</span></a></h3>
<Mermaid code="eJxLL0osyFDwCeJSAALHaPfE3FSF4NSistQiBdeiovyiWAVdXTsFp2jH9NS8kliwKiewkHM0RBlEzBks5hLtklickZSfWJQSywUWLy6pzElVcFRIy8zJsVJOS0sGAiQJJ1wSzrgkXFAlAKdPOAk="></Mermaid><h3 id="错误响应格式" tabindex="-1"><a class="header-anchor" href="#错误响应格式"><span>错误响应格式</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"code"</span><span class="token operator">:</span> <span class="token string">"PLAYER_NOT_FOUND"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"玩家不存在"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"details"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"player_123"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"trace_id"</span><span class="token operator">:</span> <span class="token string">"trace_abc123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"timestamp"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:30:00Z"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="数据格式转换" tabindex="-1"><a class="header-anchor" href="#数据格式转换"><span>数据格式转换</span></a></h2>
<h3 id="json-↔-protobuf" tabindex="-1"><a class="header-anchor" href="#json-↔-protobuf"><span>JSON ↔ Protobuf</span></a></h3>
<div class="language-javascript line-numbers-mode" data-highlighter="prismjs" data-ext="js"><pre v-pre><code class="language-javascript"><span class="line"><span class="token comment">// HTTP JSON 请求</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string-property property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string-property property">"payload"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token string-property property">"player_id"</span><span class="token operator">:</span> <span class="token string">"123"</span><span class="token punctuation">,</span> <span class="token string-property property">"duration"</span><span class="token operator">:</span> <span class="token number">24</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 转换为 gRPC Protobuf</span></span>
<span class="line">InvokeFunctionRequest <span class="token punctuation">{</span></span>
<span class="line">  <span class="token literal-property property">function_id</span><span class="token operator">:</span> <span class="token string">"player.ban"</span></span>
<span class="line">  payload <span class="token punctuation">{</span></span>
<span class="line">    fields <span class="token punctuation">{</span></span>
<span class="line">      <span class="token literal-property property">key</span><span class="token operator">:</span> <span class="token string">"player_id"</span></span>
<span class="line">      value <span class="token punctuation">{</span> <span class="token literal-property property">string_value</span><span class="token operator">:</span> <span class="token string">"123"</span> <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    fields <span class="token punctuation">{</span></span>
<span class="line">      <span class="token literal-property property">key</span><span class="token operator">:</span> <span class="token string">"duration"</span></span>
<span class="line">      value <span class="token punctuation">{</span> <span class="token literal-property property">number_value</span><span class="token operator">:</span> <span class="token number">24</span> <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/architecture/layers.html">分层设计</RouteLink></li>
<li><RouteLink to="/architecture/components.html">组件说明</RouteLink></li>
<li><RouteLink to="/api/">API 参考</RouteLink></li>
</ul>
</div></template>


