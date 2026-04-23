<template><div><h1 id="rest-api" tabindex="-1"><a class="header-anchor" href="#rest-api"><span>REST API</span></a></h1>
<p>Croupier 提供 HTTP REST API，供 Dashboard 和外部系统调用。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#基础信息">基础信息</router-link><ul><li><router-link to="#base-url">Base URL</router-link></li><li><router-link to="#认证">认证</router-link></li><li><router-link to="#通用请求头">通用请求头</router-link></li><li><router-link to="#通用响应格式">通用响应格式</router-link></li></ul></li><li><router-link to="#函数调用">函数调用</router-link><ul><li><router-link to="#调用函数-同步">调用函数（同步）</router-link></li><li><router-link to="#调用函数-异步">调用函数（异步）</router-link></li><li><router-link to="#获取作业状态">获取作业状态</router-link></li><li><router-link to="#流式获取作业事件">流式获取作业事件</router-link></li><li><router-link to="#取消作业">取消作业</router-link></li></ul></li><li><router-link to="#函数管理">函数管理</router-link><ul><li><router-link to="#获取函数列表">获取函数列表</router-link></li><li><router-link to="#获取函数详情">获取函数详情</router-link></li><li><router-link to="#注册函数">注册函数</router-link></li><li><router-link to="#注销函数">注销函数</router-link></li></ul></li><li><router-link to="#agent-管理">Agent 管理</router-link><ul><li><router-link to="#获取-agent-列表">获取 Agent 列表</router-link></li><li><router-link to="#获取-agent-详情">获取 Agent 详情</router-link></li><li><router-link to="#获取-agent-分配配置">获取 Agent 分配配置</router-link></li></ul></li><li><router-link to="#审批流程">审批流程</router-link><ul><li><router-link to="#创建审批请求">创建审批请求</router-link></li><li><router-link to="#获取审批列表">获取审批列表</router-link></li><li><router-link to="#获取审批详情">获取审批详情</router-link></li><li><router-link to="#审批通过">审批通过</router-link></li><li><router-link to="#审批拒绝">审批拒绝</router-link></li></ul></li><li><router-link to="#审计日志">审计日志</router-link><ul><li><router-link to="#查询审计日志">查询审计日志</router-link></li><li><router-link to="#获取审计详情">获取审计详情</router-link></li></ul></li><li><router-link to="#用户管理">用户管理</router-link><ul><li><router-link to="#登录">登录</router-link></li><li><router-link to="#登出">登出</router-link></li><li><router-link to="#获取当前用户信息">获取当前用户信息</router-link></li></ul></li><li><router-link to="#健康检查">健康检查</router-link><ul><li><router-link to="#健康检查-1">健康检查</router-link></li><li><router-link to="#就绪检查">就绪检查</router-link></li><li><router-link to="#版本信息">版本信息</router-link></li></ul></li><li><router-link to="#错误码">错误码</router-link><ul><li><router-link to="#错误响应示例">错误响应示例</router-link></li></ul></li><li><router-link to="#限流">限流</router-link><ul><li><router-link to="#速率限制">速率限制</router-link></li><li><router-link to="#限流响应">限流响应</router-link></li></ul></li><li><router-link to="#sdk-示例">SDK 示例</router-link><ul><li><router-link to="#curl">cURL</router-link></li><li><router-link to="#javascript">JavaScript</router-link></li><li><router-link to="#python">Python</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="基础信息" tabindex="-1"><a class="header-anchor" href="#基础信息"><span>基础信息</span></a></h2>
<h3 id="base-url" tabindex="-1"><a class="header-anchor" href="#base-url"><span>Base URL</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">开发环境: http://localhost:8080</span>
<span class="line">生产环境: https://croupier.example.com</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="认证" tabindex="-1"><a class="header-anchor" href="#认证"><span>认证</span></a></h3>
<p>使用 Bearer Token 认证：</p>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"><span class="token header"><span class="token header-name keyword">Authorization</span><span class="token punctuation">:</span> <span class="token header-value">Bearer {token}</span></span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="通用请求头" tabindex="-1"><a class="header-anchor" href="#通用请求头"><span>通用请求头</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"><span class="token header"><span class="token header-name keyword">Content-Type</span><span class="token punctuation">:</span> <span class="token header-value">application/json</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">X-Game-ID</span><span class="token punctuation">:</span> <span class="token header-value">{game_id}</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">X-Env</span><span class="token punctuation">:</span> <span class="token header-value">{env}</span></span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="通用响应格式" tabindex="-1"><a class="header-anchor" href="#通用响应格式"><span>通用响应格式</span></a></h3>
<p><strong>成功响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"data"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>错误响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"code"</span><span class="token operator">:</span> <span class="token string">"ERROR_CODE"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"错误描述"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"details"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数调用" tabindex="-1"><a class="header-anchor" href="#函数调用"><span>函数调用</span></a></h2>
<h3 id="调用函数-同步" tabindex="-1"><a class="header-anchor" href="#调用函数-同步"><span>调用函数（同步）</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/invoke</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"player_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token number">24</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token string">"作弊"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"options"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"idempotency_key"</span><span class="token operator">:</span> <span class="token string">"unique-key-123"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"ban_id"</span><span class="token operator">:</span> <span class="token string">"ban_456"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"expires_at"</span><span class="token operator">:</span> <span class="token string">"2024-12-02T10:30:00Z"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="调用函数-异步" tabindex="-1"><a class="header-anchor" href="#调用函数-异步"><span>调用函数（异步）</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/jobs</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"data.export"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"job_id"</span><span class="token operator">:</span> <span class="token string">"job_abc123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"pending"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="获取作业状态" tabindex="-1"><a class="header-anchor" href="#获取作业状态"><span>获取作业状态</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/jobs/{job_id}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"job_id"</span><span class="token operator">:</span> <span class="token string">"job_abc123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"running"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"progress"</span><span class="token operator">:</span> <span class="token number">0.5</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"started_at"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:00:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token null keyword">null</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="流式获取作业事件" tabindex="-1"><a class="header-anchor" href="#流式获取作业事件"><span>流式获取作业事件</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/jobs/{job_id}/events</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>返回 Server-Sent Events (SSE) 流：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">data: {"type":"PROGRESS","progress":0.1,"message":"处理中..."}</span>
<span class="line"></span>
<span class="line">data: {"type":"PROGRESS","progress":0.5,"message":"处理中..."}</span>
<span class="line"></span>
<span class="line">data: {"type":"DONE","progress":1.0,"result":{...}}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="取消作业" tabindex="-1"><a class="header-anchor" href="#取消作业"><span>取消作业</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">DELETE /api/jobs/{job_id}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token string">"用户取消"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数管理" tabindex="-1"><a class="header-anchor" href="#函数管理"><span>函数管理</span></a></h2>
<h3 id="获取函数列表" tabindex="-1"><a class="header-anchor" href="#获取函数列表"><span>获取函数列表</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/functions?game_id={game_id}&amp;env={env}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>查询参数</strong>：</p>
<table>
<thead>
<tr>
<th>参数</th>
<th>类型</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>game_id</code></td>
<td>string</td>
<td>游戏 ID</td>
</tr>
<tr>
<td><code v-pre>env</code></td>
<td>string</td>
<td>环境</td>
</tr>
<tr>
<td><code v-pre>category</code></td>
<td>string</td>
<td>函数分类</td>
</tr>
<tr>
<td><code v-pre>page</code></td>
<td>int</td>
<td>页码</td>
</tr>
<tr>
<td><code v-pre>size</code></td>
<td>int</td>
<td>每页数量</td>
</tr>
</tbody>
</table>
<p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"封禁玩家"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"risk_level"</span><span class="token operator">:</span> <span class="token string">"high"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"total"</span><span class="token operator">:</span> <span class="token number">100</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"page"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"size"</span><span class="token operator">:</span> <span class="token number">20</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="获取函数详情" tabindex="-1"><a class="header-anchor" href="#获取函数详情"><span>获取函数详情</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/functions/{function_id}?game_id={game_id}&amp;env={env}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"封禁玩家"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"封禁指定玩家账号"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params_schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result_schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="注册函数" tabindex="-1"><a class="header-anchor" href="#注册函数"><span>注册函数</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/functions</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"descriptor"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="注销函数" tabindex="-1"><a class="header-anchor" href="#注销函数"><span>注销函数</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">DELETE /api/functions/{function_id}?game_id={game_id}&amp;env={env}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="agent-管理" tabindex="-1"><a class="header-anchor" href="#agent-管理"><span>Agent 管理</span></a></h2>
<h3 id="获取-agent-列表" tabindex="-1"><a class="header-anchor" href="#获取-agent-列表"><span>获取 Agent 列表</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/agents?game_id={game_id}&amp;env={env}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"agents"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"agent_id"</span><span class="token operator">:</span> <span class="token string">"agent_1"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"online"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"last_heartbeat"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:00:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> <span class="token string">"player.kick"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="获取-agent-详情" tabindex="-1"><a class="header-anchor" href="#获取-agent-详情"><span>获取 Agent 详情</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/agents/{agent_id}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="获取-agent-分配配置" tabindex="-1"><a class="header-anchor" href="#获取-agent-分配配置"><span>获取 Agent 分配配置</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/agents/{agent_id}/assignments</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="审批流程" tabindex="-1"><a class="header-anchor" href="#审批流程"><span>审批流程</span></a></h2>
<h3 id="创建审批请求" tabindex="-1"><a class="header-anchor" href="#创建审批请求"><span>创建审批请求</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/approvals</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"player_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token number">24</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token string">"玩家使用外挂"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"approval_id"</span><span class="token operator">:</span> <span class="token string">"approval_123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"pending"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"required_approvals"</span><span class="token operator">:</span> <span class="token number">2</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"current_approvals"</span><span class="token operator">:</span> <span class="token number">0</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="获取审批列表" tabindex="-1"><a class="header-anchor" href="#获取审批列表"><span>获取审批列表</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/approvals?state=pending&amp;game_id={game_id}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>查询参数</strong>：</p>
<table>
<thead>
<tr>
<th>参数</th>
<th>类型</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>state</code></td>
<td>string</td>
<td>状态：pending/approved/rejected</td>
</tr>
<tr>
<td><code v-pre>game_id</code></td>
<td>string</td>
<td>游戏 ID</td>
</tr>
<tr>
<td><code v-pre>function_id</code></td>
<td>string</td>
<td>函数 ID</td>
</tr>
<tr>
<td><code v-pre>page</code></td>
<td>int</td>
<td>页码</td>
</tr>
<tr>
<td><code v-pre>size</code></td>
<td>int</td>
<td>每页数量</td>
</tr>
</tbody>
</table>
<h3 id="获取审批详情" tabindex="-1"><a class="header-anchor" href="#获取审批详情"><span>获取审批详情</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/approvals/{approval_id}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="审批通过" tabindex="-1"><a class="header-anchor" href="#审批通过"><span>审批通过</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/approvals/{approval_id}/approve</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"comment"</span><span class="token operator">:</span> <span class="token string">"确认封禁"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审批拒绝" tabindex="-1"><a class="header-anchor" href="#审批拒绝"><span>审批拒绝</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/approvals/{approval_id}/reject</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token string">"证据不足"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="审计日志" tabindex="-1"><a class="header-anchor" href="#审计日志"><span>审计日志</span></a></h2>
<h3 id="查询审计日志" tabindex="-1"><a class="header-anchor" href="#查询审计日志"><span>查询审计日志</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/audit/query</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"start_time"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T00:00:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"end_time"</span><span class="token operator">:</span> <span class="token string">"2024-12-02T00:00:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"filters"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"action"</span><span class="token operator">:</span> <span class="token string">"function.invoke"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"page"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"size"</span><span class="token operator">:</span> <span class="token number">50</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"audits"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"audit_id"</span><span class="token operator">:</span> <span class="token string">"audit_001"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"timestamp"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:30:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"user"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"action"</span><span class="token operator">:</span> <span class="token string">"function.invoke"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"payload_preview"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"result"</span><span class="token operator">:</span> <span class="token string">"success"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"ip"</span><span class="token operator">:</span> <span class="token string">"192.168.1.100"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"ip_region"</span><span class="token operator">:</span> <span class="token string">"中国 上海"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"total"</span><span class="token operator">:</span> <span class="token number">1000</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"page"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"size"</span><span class="token operator">:</span> <span class="token number">50</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="获取审计详情" tabindex="-1"><a class="header-anchor" href="#获取审计详情"><span>获取审计详情</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/audit/{audit_id}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="用户管理" tabindex="-1"><a class="header-anchor" href="#用户管理"><span>用户管理</span></a></h2>
<h3 id="登录" tabindex="-1"><a class="header-anchor" href="#登录"><span>登录</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/auth/login</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>请求体</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"password"</span><span class="token operator">:</span> <span class="token string">"password"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"totp"</span><span class="token operator">:</span> <span class="token string">"123456"</span>  <span class="token comment">// 可选，双因素认证</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"token"</span><span class="token operator">:</span> <span class="token string">"eyJhbGciOiJIUzI1NiIs..."</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"user"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"roles"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="登出" tabindex="-1"><a class="header-anchor" href="#登出"><span>登出</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">POST /api/auth/logout</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="获取当前用户信息" tabindex="-1"><a class="header-anchor" href="#获取当前用户信息"><span>获取当前用户信息</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /api/auth/me</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"email"</span><span class="token operator">:</span> <span class="token string">"admin@example.com"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"roles"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"*.*"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="健康检查" tabindex="-1"><a class="header-anchor" href="#健康检查"><span>健康检查</span></a></h2>
<h3 id="健康检查-1" tabindex="-1"><a class="header-anchor" href="#健康检查-1"><span>健康检查</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /healthz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"ok"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="就绪检查" tabindex="-1"><a class="header-anchor" href="#就绪检查"><span>就绪检查</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /readyz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"ready"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="版本信息" tabindex="-1"><a class="header-anchor" href="#版本信息"><span>版本信息</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line">GET /version</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>响应</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"commit"</span><span class="token operator">:</span> <span class="token string">"abc123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"build_time"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:00:00Z"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="错误码" tabindex="-1"><a class="header-anchor" href="#错误码"><span>错误码</span></a></h2>
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
<h3 id="错误响应示例" tabindex="-1"><a class="header-anchor" href="#错误响应示例"><span>错误响应示例</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"success"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
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
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="限流" tabindex="-1"><a class="header-anchor" href="#限流"><span>限流</span></a></h2>
<h3 id="速率限制" tabindex="-1"><a class="header-anchor" href="#速率限制"><span>速率限制</span></a></h3>
<p>API 实施速率限制：</p>
<table>
<thead>
<tr>
<th>级别</th>
<th>限制</th>
</tr>
</thead>
<tbody>
<tr>
<td>默认</td>
<td>100 req/min</td>
</tr>
<tr>
<td>认证用户</td>
<td>1000 req/min</td>
</tr>
<tr>
<td>管理员</td>
<td>无限制</td>
</tr>
</tbody>
</table>
<h3 id="限流响应" tabindex="-1"><a class="header-anchor" href="#限流响应"><span>限流响应</span></a></h3>
<div class="language-http line-numbers-mode" data-highlighter="prismjs" data-ext="http"><pre v-pre><code class="language-http"><span class="line"><span class="token response-status"><span class="token http-version property">HTTP/1.1</span> <span class="token status-code number">429</span> <span class="token reason-phrase string">Too Many Requests</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">X-RateLimit-Limit</span><span class="token punctuation">:</span> <span class="token header-value">1000</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">X-RateLimit-Remaining</span><span class="token punctuation">:</span> <span class="token header-value">0</span></span></span>
<span class="line"><span class="token header"><span class="token header-name keyword">X-RateLimit-Reset</span><span class="token punctuation">:</span> <span class="token header-value">1701427200</span></span></span>
<span class="line"></span>
<span class="line">{</span>
<span class="line">  "success": false,</span>
<span class="line">  "error": {</span>
<span class="line">    "code": "RESOURCE_EXHAUSTED",</span>
<span class="line">    "message": "请求过于频繁，请稍后再试"</span>
<span class="line">  }</span>
<span class="line">}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="sdk-示例" tabindex="-1"><a class="header-anchor" href="#sdk-示例"><span>SDK 示例</span></a></h2>
<h3 id="curl" tabindex="-1"><a class="header-anchor" href="#curl"><span>cURL</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/invoke <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Authorization: Bearer <span class="token variable">$TOKEN</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{</span>
<span class="line">    "function_id": "player.ban",</span>
<span class="line">    "payload": {</span>
<span class="line">      "player_id": "player_123",</span>
<span class="line">      "duration": 24</span>
<span class="line">    }</span>
<span class="line">  }'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="javascript" tabindex="-1"><a class="header-anchor" href="#javascript"><span>JavaScript</span></a></h3>
<div class="language-javascript line-numbers-mode" data-highlighter="prismjs" data-ext="js"><pre v-pre><code class="language-javascript"><span class="line"><span class="token keyword">const</span> response <span class="token operator">=</span> <span class="token keyword">await</span> <span class="token function">fetch</span><span class="token punctuation">(</span><span class="token string">'http://localhost:8080/api/invoke'</span><span class="token punctuation">,</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token literal-property property">method</span><span class="token operator">:</span> <span class="token string">'POST'</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token literal-property property">headers</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token string-property property">'Content-Type'</span><span class="token operator">:</span> <span class="token string">'application/json'</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string-property property">'Authorization'</span><span class="token operator">:</span> <span class="token template-string"><span class="token template-punctuation string">`</span><span class="token string">Bearer </span><span class="token interpolation"><span class="token interpolation-punctuation punctuation">${</span>token<span class="token interpolation-punctuation punctuation">}</span></span><span class="token template-punctuation string">`</span></span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token literal-property property">body</span><span class="token operator">:</span> <span class="token constant">JSON</span><span class="token punctuation">.</span><span class="token function">stringify</span><span class="token punctuation">(</span><span class="token punctuation">{</span></span>
<span class="line">    <span class="token literal-property property">function_id</span><span class="token operator">:</span> <span class="token string">'player.ban'</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token literal-property property">payload</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token literal-property property">player_id</span><span class="token operator">:</span> <span class="token string">'player_123'</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token literal-property property">duration</span><span class="token operator">:</span> <span class="token number">24</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">const</span> result <span class="token operator">=</span> <span class="token keyword">await</span> response<span class="token punctuation">.</span><span class="token function">json</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="python" tabindex="-1"><a class="header-anchor" href="#python"><span>Python</span></a></h3>
<div class="language-python line-numbers-mode" data-highlighter="prismjs" data-ext="py"><pre v-pre><code class="language-python"><span class="line"><span class="token keyword">import</span> requests</span>
<span class="line"></span>
<span class="line">response <span class="token operator">=</span> requests<span class="token punctuation">.</span>post<span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">'http://localhost:8080/api/invoke'</span><span class="token punctuation">,</span></span>
<span class="line">    headers<span class="token operator">=</span><span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">'Content-Type'</span><span class="token punctuation">:</span> <span class="token string">'application/json'</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">'Authorization'</span><span class="token punctuation">:</span> <span class="token string-interpolation"><span class="token string">f'Bearer </span><span class="token interpolation"><span class="token punctuation">{</span>token<span class="token punctuation">}</span></span><span class="token string">'</span></span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    json<span class="token operator">=</span><span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">'function_id'</span><span class="token punctuation">:</span> <span class="token string">'player.ban'</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">'payload'</span><span class="token punctuation">:</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token string">'player_id'</span><span class="token punctuation">:</span> <span class="token string">'player_123'</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token string">'duration'</span><span class="token punctuation">:</span> <span class="token number">24</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">result <span class="token operator">=</span> response<span class="token punctuation">.</span>json<span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/api/grpc.html">gRPC API</RouteLink></li>
<li><RouteLink to="/api/proto-options.html">Proto 选项</RouteLink></li>
<li><RouteLink to="/api/">API 概览</RouteLink></li>
</ul>
</div></template>


