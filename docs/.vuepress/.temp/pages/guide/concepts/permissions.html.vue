<template><div><h1 id="权限控制" tabindex="-1"><a class="header-anchor" href="#权限控制"><span>权限控制</span></a></h1>
<p>Croupier 实现<strong>零信任安全</strong>模型，提供 RBAC/ABAC 混合权限控制、操作审批和完整审计链。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#权限模型">权限模型</router-link><ul><li><router-link to="#整体架构">整体架构</router-link></li><li><router-link to="#rbac-模型">RBAC 模型</router-link></li><li><router-link to="#abac-模型">ABAC 模型</router-link></li></ul></li><li><router-link to="#用户和角色">用户和角色</router-link><ul><li><router-link to="#用户定义">用户定义</router-link></li><li><router-link to="#角色定义">角色定义</router-link></li></ul></li><li><router-link to="#权限定义">权限定义</router-link><ul><li><router-link to="#权限格式">权限格式</router-link></li><li><router-link to="#游戏作用域">游戏作用域</router-link></li></ul></li><li><router-link to="#函数权限配置">函数权限配置</router-link><ul><li><router-link to="#基础权限">基础权限</router-link></li><li><router-link to="#abac-表达式">ABAC 表达式</router-link></li><li><router-link to="#可用表达式">可用表达式</router-link></li><li><router-link to="#可用变量">可用变量</router-link></li></ul></li><li><router-link to="#审批流程">审批流程</router-link><ul><li><router-link to="#双人规则">双人规则</router-link></li><li><router-link to="#审批流程-1">审批流程</router-link></li><li><router-link to="#审批-api">审批 API</router-link></li></ul></li><li><router-link to="#审计日志">审计日志</router-link><ul><li><router-link to="#审计事件">审计事件</router-link></li><li><router-link to="#敏感字段脱敏">敏感字段脱敏</router-link></li><li><router-link to="#审计链防篡改">审计链防篡改</router-link></li></ul></li><li><router-link to="#风险等级">风险等级</router-link><ul><li><router-link to="#风险分级">风险分级</router-link></li><li><router-link to="#风险配置">风险配置</router-link></li></ul></li><li><router-link to="#限流保护">限流保护</router-link><ul><li><router-link to="#函数级限流">函数级限流</router-link></li><li><router-link to="#用户级限流">用户级限流</router-link></li></ul></li><li><router-link to="#最佳实践">最佳实践</router-link><ul><li><router-link to="#_1-最小权限原则">1. 最小权限原则</router-link></li><li><router-link to="#_2-环境隔离">2. 环境隔离</router-link></li><li><router-link to="#_3-审批配置">3. 审批配置</router-link></li><li><router-link to="#_4-审计保留">4. 审计保留</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="权限模型" tabindex="-1"><a class="header-anchor" href="#权限模型"><span>权限模型</span></a></h2>
<h3 id="整体架构" tabindex="-1"><a class="header-anchor" href="#整体架构"><span>整体架构</span></a></h3>
<Mermaid code="eJxLL0osyFAIceJSAILQ4tSi6OdTVjzr2B4LFgjKz0mNfrF80ovOTRCBgNSi3Mzi4sz8vOhnc5tfzpwAEXYrzUsuAQk+bd/7bOqGWC64cQq6unY1z7qXPpvTWQM2Dm4uWOZpR9vL1t4aJHPRrAGrerJ/7rOuJTVwayDGOzo5OkeDCIXna6c9n7oU1SkQ47tWPGtofLa44dn8pTVgDagOe7px3rOG5U97doIdAZYHAKXGa0w="></Mermaid><h3 id="rbac-模型" tabindex="-1"><a class="header-anchor" href="#rbac-模型"><span>RBAC 模型</span></a></h3>
<p>RBAC (Role-Based Access Control) 基于角色的访问控制。</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">用户 (User)</span>
<span class="line">  ↓ 拥有</span>
<span class="line">角色 (Role)</span>
<span class="line">  ↓ 分配</span>
<span class="line">权限 (Permission)</span>
<span class="line">  ↓ 保护</span>
<span class="line">函数 (Function)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="abac-模型" tabindex="-1"><a class="header-anchor" href="#abac-模型"><span>ABAC 模型</span></a></h3>
<p>ABAC (Attribute-Based Access Control) 基于属性的访问控制。</p>
<div class="language-javascript line-numbers-mode" data-highlighter="prismjs" data-ext="js"><pre v-pre><code class="language-javascript"><span class="line"><span class="token comment">// 评估表达式</span></span>
<span class="line">user<span class="token punctuation">.</span>roles<span class="token punctuation">.</span><span class="token function">includes</span><span class="token punctuation">(</span><span class="token string">'admin'</span><span class="token punctuation">)</span> <span class="token operator">||</span> <span class="token punctuation">(</span>game_id <span class="token operator">===</span> <span class="token string">'my-game'</span> <span class="token operator">&amp;&amp;</span> env <span class="token operator">===</span> <span class="token string">'dev'</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="用户和角色" tabindex="-1"><a class="header-anchor" href="#用户和角色"><span>用户和角色</span></a></h2>
<h3 id="用户定义" tabindex="-1"><a class="header-anchor" href="#用户定义"><span>用户定义</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"email"</span><span class="token operator">:</span> <span class="token string">"admin@example.com"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"roles"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">,</span> <span class="token string">"operator"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"attributes"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"department"</span><span class="token operator">:</span> <span class="token string">"operations"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"level"</span><span class="token operator">:</span> <span class="token number">5</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="角色定义" tabindex="-1"><a class="header-anchor" href="#角色定义"><span>角色定义</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"role_id"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"管理员"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token string">"*.*"</span>  <span class="token comment">// 所有权限</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"role_id"</span><span class="token operator">:</span> <span class="token string">"gm"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"游戏管理员"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token string">"player.*"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"item.*"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"guild.*"</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"role_id"</span><span class="token operator">:</span> <span class="token string">"viewer"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"查看者"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token string">"player.view"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"item.view"</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="权限定义" tabindex="-1"><a class="header-anchor" href="#权限定义"><span>权限定义</span></a></h2>
<h3 id="权限格式" tabindex="-1"><a class="header-anchor" href="#权限格式"><span>权限格式</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">{entity}.{operation}</span>
<span class="line"></span>
<span class="line">示例：</span>
<span class="line">- player.ban      # 封禁玩家</span>
<span class="line">- player.view     # 查看玩家</span>
<span class="line">- item.create     # 创建物品</span>
<span class="line">- item.delete     # 删除物品</span>
<span class="line">- *.*             # 所有权限（谨慎使用）</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="游戏作用域" tabindex="-1"><a class="header-anchor" href="#游戏作用域"><span>游戏作用域</span></a></h3>
<p>权限可以限定到特定游戏：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">game:{game_id}:{permission}</span>
<span class="line"></span>
<span class="line">示例：</span>
<span class="line">- game:my-game:player.ban    # 仅 my-game 的封禁权限</span>
<span class="line">- game:test-game:*.*         # test-game 的所有权限</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数权限配置" tabindex="-1"><a class="header-anchor" href="#函数权限配置"><span>函数权限配置</span></a></h2>
<h3 id="基础权限" tabindex="-1"><a class="header-anchor" href="#基础权限"><span>基础权限</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="abac-表达式" tabindex="-1"><a class="header-anchor" href="#abac-表达式"><span>ABAC 表达式</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"has_role('admin') || has_role('senior_gm')"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="可用表达式" tabindex="-1"><a class="header-anchor" href="#可用表达式"><span>可用表达式</span></a></h3>
<table>
<thead>
<tr>
<th>函数</th>
<th>说明</th>
<th>示例</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>has_role(role)</code></td>
<td>检查角色</td>
<td><code v-pre>has_role('admin')</code></td>
</tr>
<tr>
<td><code v-pre>has_permission(perm)</code></td>
<td>检查权限</td>
<td><code v-pre>has_permission('player.view')</code></td>
</tr>
<tr>
<td><code v-pre>==</code></td>
<td>等于</td>
<td><code v-pre>env == 'prod'</code></td>
</tr>
<tr>
<td><code v-pre>!=</code></td>
<td>不等于</td>
<td><code v-pre>env != 'dev'</code></td>
</tr>
<tr>
<td><code v-pre>&amp;&amp;</code></td>
<td>逻辑与</td>
<td><code v-pre>has_role('admin') &amp;&amp; env == 'prod'</code></td>
</tr>
<tr>
<td><code v-pre>||</code></td>
<td>逻辑或</td>
<td><code v-pre>has_role('admin') || has_role('gm')</code></td>
</tr>
<tr>
<td><code v-pre>()</code></td>
<td>分组</td>
<td><code v-pre>(a || b) &amp;&amp; c</code></td>
</tr>
</tbody>
</table>
<h3 id="可用变量" tabindex="-1"><a class="header-anchor" href="#可用变量"><span>可用变量</span></a></h3>
<table>
<thead>
<tr>
<th>变量</th>
<th>类型</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>user</code></td>
<td>object</td>
<td>当前用户信息</td>
</tr>
<tr>
<td><code v-pre>user.roles</code></td>
<td>array</td>
<td>用户角色列表</td>
</tr>
<tr>
<td><code v-pre>game_id</code></td>
<td>string</td>
<td>目标游戏 ID</td>
</tr>
<tr>
<td><code v-pre>env</code></td>
<td>string</td>
<td>环境 (dev/staging/prod)</td>
</tr>
<tr>
<td><code v-pre>function_id</code></td>
<td>string</td>
<td>被调用的函数 ID</td>
</tr>
</tbody>
</table>
<h2 id="审批流程" tabindex="-1"><a class="header-anchor" href="#审批流程"><span>审批流程</span></a></h2>
<h3 id="双人规则" tabindex="-1"><a class="header-anchor" href="#双人规则"><span>双人规则</span></a></h3>
<p>高风险操作需要双人审批：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"two_person_rule"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"approval"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"threshold"</span><span class="token operator">:</span> <span class="token number">2</span>  <span class="token comment">// 需要两人审批</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审批流程-1" tabindex="-1"><a class="header-anchor" href="#审批流程-1"><span>审批流程</span></a></h3>
<Mermaid code="eJwrLkksSXXJTEwvSszVLTPiUgCCaK1YBV1dO4Wn+1qfrlv4rHOnlcKz/glPdi15saH5+ZQVYDVwOYjK7ZteNsx6sb/dSuHFts1Pl8yHyD2bugG74mfdk57vngs0FkxjV/O0f9qzbR0gA1ufTd8GUQOzBqzmWefyFwt7nuxYa6XwdE/D0+XdEAGwSrgc3LR1Pc86JgBtBEsAmU+75uNQuWTjiy1LYSohPJjtEFPAKoGBBBcFq0EXhfgNQy3YV3BRAFQ3wBM="></Mermaid><h3 id="审批-api" tabindex="-1"><a class="header-anchor" href="#审批-api"><span>审批 API</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 创建审批请求</span></span>
<span class="line">POST /api/approvals</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"function_id"</span><span class="token builtin class-name">:</span> <span class="token string">"player.ban"</span>,</span>
<span class="line">  <span class="token string">"payload"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span><span class="token punctuation">..</span>.<span class="token punctuation">}</span>,</span>
<span class="line">  <span class="token string">"reason"</span><span class="token builtin class-name">:</span> <span class="token string">"玩家使用外挂"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 获取待审批列表</span></span>
<span class="line">GET /api/approvals?state<span class="token operator">=</span>pending</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 审批通过</span></span>
<span class="line">POST /api/approvals/<span class="token punctuation">{</span>id<span class="token punctuation">}</span>/approve</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 审批拒绝</span></span>
<span class="line">POST /api/approvals/<span class="token punctuation">{</span>id<span class="token punctuation">}</span>/reject</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"reason"</span><span class="token builtin class-name">:</span> <span class="token string">"证据不足"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="审计日志" tabindex="-1"><a class="header-anchor" href="#审计日志"><span>审计日志</span></a></h2>
<h3 id="审计事件" tabindex="-1"><a class="header-anchor" href="#审计事件"><span>审计事件</span></a></h3>
<p>所有操作都会记录审计日志：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"audit_id"</span><span class="token operator">:</span> <span class="token string">"audit_20241201_001"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"timestamp"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:30:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"user"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"action"</span><span class="token operator">:</span> <span class="token string">"function.invoke"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload_preview"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token string">"***"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token number">24</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token string">"success"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"approval_id"</span><span class="token operator">:</span> <span class="token string">"approval_123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ip"</span><span class="token operator">:</span> <span class="token string">"192.168.1.100"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ip_region"</span><span class="token operator">:</span> <span class="token string">"中国 上海"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="敏感字段脱敏" tabindex="-1"><a class="header-anchor" href="#敏感字段脱敏"><span>敏感字段脱敏</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># 配置脱敏字段</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">sensitive_fields</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"password"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"token"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"secret"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"api_key"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审计链防篡改" tabindex="-1"><a class="header-anchor" href="#审计链防篡改"><span>审计链防篡改</span></a></h3>
<p>每条审计记录包含哈希值：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"audit_id"</span><span class="token operator">:</span> <span class="token string">"audit_001"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"hash"</span><span class="token operator">:</span> <span class="token string">"sha256(prev_hash + content)"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"prev_hash"</span><span class="token operator">:</span> <span class="token string">"sha256(...)"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="风险等级" tabindex="-1"><a class="header-anchor" href="#风险等级"><span>风险等级</span></a></h2>
<h3 id="风险分级" tabindex="-1"><a class="header-anchor" href="#风险分级"><span>风险分级</span></a></h3>
<table>
<thead>
<tr>
<th>等级</th>
<th>说明</th>
<th>审批要求</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>low</code></td>
<td>低风险</td>
<td>无需审批</td>
</tr>
<tr>
<td><code v-pre>medium</code></td>
<td>中风险</td>
<td>可选审批</td>
</tr>
<tr>
<td><code v-pre>high</code></td>
<td>高风险</td>
<td>强制双人审批</td>
</tr>
</tbody>
</table>
<h3 id="风险配置" tabindex="-1"><a class="header-anchor" href="#风险配置"><span>风险配置</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"risk_level"</span><span class="token operator">:</span> <span class="token string">"high"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"risk_warning"</span><span class="token operator">:</span> <span class="token string">"高风险操作，封禁后玩家无法登录"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"confirm_message"</span><span class="token operator">:</span> <span class="token string">"确认封禁玩家？"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="限流保护" tabindex="-1"><a class="header-anchor" href="#限流保护"><span>限流保护</span></a></h2>
<h3 id="函数级限流" tabindex="-1"><a class="header-anchor" href="#函数级限流"><span>函数级限流</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"rate_limit"</span><span class="token operator">:</span> <span class="token string">"10rps"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"concurrency"</span><span class="token operator">:</span> <span class="token number">5</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="用户级限流" tabindex="-1"><a class="header-anchor" href="#用户级限流"><span>用户级限流</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"user_rate_limit"</span><span class="token operator">:</span> <span class="token string">"5rps"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"user_burst"</span><span class="token operator">:</span> <span class="token number">10</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="最佳实践" tabindex="-1"><a class="header-anchor" href="#最佳实践"><span>最佳实践</span></a></h2>
<h3 id="_1-最小权限原则" tabindex="-1"><a class="header-anchor" href="#_1-最小权限原则"><span>1. 最小权限原则</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token comment">// ❌ 不推荐：过度授权</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"roles"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">]</span>  <span class="token comment">// 拥有所有权限</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ✅ 推荐：按需授权</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"roles"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"gm"</span><span class="token punctuation">]</span><span class="token punctuation">,</span>  <span class="token comment">// 仅游戏管理权限</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> <span class="token string">"player.view"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-环境隔离" tabindex="-1"><a class="header-anchor" href="#_2-环境隔离"><span>2. 环境隔离</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"env == 'dev' || has_role('admin')"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-审批配置" tabindex="-1"><a class="header-anchor" href="#_3-审批配置"><span>3. 审批配置</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"approval"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"threshold"</span><span class="token operator">:</span> <span class="token string">"has_role('admin') ? 1 : 2"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"approvers"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">,</span> <span class="token string">"senior_gm"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"timeout"</span><span class="token operator">:</span> <span class="token string">"24h"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-审计保留" tabindex="-1"><a class="header-anchor" href="#_4-审计保留"><span>4. 审计保留</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">retention_days</span><span class="token punctuation">:</span> <span class="token number">365</span></span>
<span class="line">    <span class="token key atrule">backup_enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/guide/concepts/function-management.html">函数管理</RouteLink></li>
<li><RouteLink to="/guide/configuration.html">配置管理</RouteLink></li>
<li><RouteLink to="/guide/operations/security.html">安全配置</RouteLink></li>
</ul>
</div></template>


