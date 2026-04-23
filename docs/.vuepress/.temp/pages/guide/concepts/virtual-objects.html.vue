<template><div><h1 id="虚拟对象系统" tabindex="-1"><a class="header-anchor" href="#虚拟对象系统"><span>虚拟对象系统</span></a></h1>
<p>Croupier 采用<strong>四层虚拟对象模型</strong>，实现从业务定义到 UI 生成的完整驱动。</p>
<h2 id="四层架构" tabindex="-1"><a class="header-anchor" href="#四层架构"><span>四层架构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│  Layer 4: Component（组件）                                 │</span>
<span class="line">│  功能模块的打包单位，包含相关的 functions、entities、resources│</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              │</span>
<span class="line">                              ▼</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│  Layer 3: Resource（资源）                                  │</span>
<span class="line">│  UI 层面的操作集合，ProTable 配置、列定义、操作按钮         │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              │</span>
<span class="line">                              ▼</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│  Layer 2: Entity（实体）                                    │</span>
<span class="line">│  业务对象的完整描述，JSON Schema、UI 配置、操作映射         │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line">                              │</span>
<span class="line">                              ▼</span>
<span class="line">┌────────────────────────────────────────────────────────────┐</span>
<span class="line">│  Layer 1: Function（函数）                                  │</span>
<span class="line">│  具体的业务操作实现，输入输出 Schema、权限、语义            │</span>
<span class="line">└────────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="layer-1-function-函数" tabindex="-1"><a class="header-anchor" href="#layer-1-function-函数"><span>Layer 1: Function（函数）</span></a></h2>
<p>函数是系统中最小的可执行单元，代表一个具体的业务操作。</p>
<h3 id="函数定义" tabindex="-1"><a class="header-anchor" href="#函数定义"><span>函数定义</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"封禁玩家"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"封禁指定玩家账号"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"risk_level"</span><span class="token operator">:</span> <span class="token string">"high"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"玩家ID"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"封禁时长（小时）"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"封禁原因"</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"required"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">,</span> <span class="token string">"duration"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"success"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"boolean"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"ban_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"approval"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"threshold"</span><span class="token operator">:</span> <span class="token number">2</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="函数属性" tabindex="-1"><a class="header-anchor" href="#函数属性"><span>函数属性</span></a></h3>
<table>
<thead>
<tr>
<th>属性</th>
<th>类型</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>id</code></td>
<td>string</td>
<td>函数唯一标识</td>
</tr>
<tr>
<td><code v-pre>name</code></td>
<td>string</td>
<td>函数显示名称</td>
</tr>
<tr>
<td><code v-pre>category</code></td>
<td>string</td>
<td>函数分类</td>
</tr>
<tr>
<td><code v-pre>risk_level</code></td>
<td>string</td>
<td>风险等级：low/medium/high</td>
</tr>
<tr>
<td><code v-pre>params</code></td>
<td>Schema</td>
<td>输入参数定义</td>
</tr>
<tr>
<td><code v-pre>result</code></td>
<td>Schema</td>
<td>返回值定义</td>
</tr>
<tr>
<td><code v-pre>auth</code></td>
<td>object</td>
<td>权限配置</td>
</tr>
</tbody>
</table>
<h2 id="layer-2-entity-实体" tabindex="-1"><a class="header-anchor" href="#layer-2-entity-实体"><span>Layer 2: Entity（实体）</span></a></h2>
<p>实体是业务对象的完整描述，包含数据结构、验证规则和操作映射。</p>
<h3 id="实体定义" tabindex="-1"><a class="header-anchor" href="#实体定义"><span>实体定义</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"玩家"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"玩家实体定义"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"玩家ID"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"readonly"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"username"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"用户名"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"minLength"</span><span class="token operator">:</span> <span class="token number">3</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"maxLength"</span><span class="token operator">:</span> <span class="token number">16</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"nickname"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"昵称"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"level"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"等级"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"vip_level"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"VIP等级"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"status"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"状态"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"enum"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"normal"</span><span class="token punctuation">,</span> <span class="token string">"banned"</span><span class="token punctuation">,</span> <span class="token string">"deleted"</span><span class="token punctuation">]</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"create"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"注册玩家"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.get"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"查看详情"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"update"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.update"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"更新信息"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"delete"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"封禁"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"confirm"</span><span class="token operator">:</span> <span class="token string">"确认封禁该玩家？"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"display_field"</span><span class="token operator">:</span> <span class="token string">"username"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"title_template"</span><span class="token operator">:</span> <span class="token string">"{username} ({nickname})"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="实体属性" tabindex="-1"><a class="header-anchor" href="#实体属性"><span>实体属性</span></a></h3>
<table>
<thead>
<tr>
<th>属性</th>
<th>类型</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>schema</code></td>
<td>Schema</td>
<td>JSON Schema 数据定义</td>
</tr>
<tr>
<td><code v-pre>operations</code></td>
<td>object</td>
<td>CRUD 操作映射</td>
</tr>
<tr>
<td><code v-pre>ui.display_field</code></td>
<td>string</td>
<td>主显示字段</td>
</tr>
<tr>
<td><code v-pre>ui.title_template</code></td>
<td>string</td>
<td>标题模板</td>
</tr>
</tbody>
</table>
<h2 id="layer-3-resource-资源" tabindex="-1"><a class="header-anchor" href="#layer-3-resource-资源"><span>Layer 3: Resource（资源）</span></a></h2>
<p>资源是 UI 层面的操作集合，将多个函数组合成完整的管理界面。</p>
<h3 id="资源定义" tabindex="-1"><a class="header-anchor" href="#资源定义"><span>资源定义</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"pro-table"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"玩家管理"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"pro-table"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"columns"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"player_id"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"玩家ID"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"width"</span><span class="token operator">:</span> <span class="token number">120</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"fixed"</span><span class="token operator">:</span> <span class="token string">"left"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"username"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"用户名"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"width"</span><span class="token operator">:</span> <span class="token number">150</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"searchable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"nickname"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"昵称"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"width"</span><span class="token operator">:</span> <span class="token number">150</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"level"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"等级"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"width"</span><span class="token operator">:</span> <span class="token number">80</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"sortable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"status"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"状态"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"width"</span><span class="token operator">:</span> <span class="token number">100</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"valueEnum"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"normal"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"text"</span><span class="token operator">:</span> <span class="token string">"正常"</span><span class="token punctuation">,</span> <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"Success"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"banned"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"text"</span><span class="token operator">:</span> <span class="token string">"封禁"</span><span class="token punctuation">,</span> <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"Error"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"deleted"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"text"</span><span class="token operator">:</span> <span class="token string">"删除"</span><span class="token punctuation">,</span> <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"Default"</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"create"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"operation"</span><span class="token operator">:</span> <span class="token string">"create"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"新建玩家"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"edit"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"operation"</span><span class="token operator">:</span> <span class="token string">"update"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"编辑"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"delete"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"operation"</span><span class="token operator">:</span> <span class="token string">"delete"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"封禁"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"confirm"</span><span class="token operator">:</span> <span class="token string">"确认封禁该玩家？"</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"toolbar"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"export"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"导出"</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="资源类型" tabindex="-1"><a class="header-anchor" href="#资源类型"><span>资源类型</span></a></h3>
<table>
<thead>
<tr>
<th>类型</th>
<th>说明</th>
<th>组件</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>pro-table</code></td>
<td>列表页面</td>
<td>ProTable</td>
</tr>
<tr>
<td><code v-pre>pro-form</code></td>
<td>表单页面</td>
<td>ProForm</td>
</tr>
<tr>
<td><code v-pre>pro-descriptions</code></td>
<td>详情页面</td>
<td>ProDescriptions</td>
</tr>
</tbody>
</table>
<h2 id="layer-4-component-组件" tabindex="-1"><a class="header-anchor" href="#layer-4-component-组件"><span>Layer 4: Component（组件）</span></a></h2>
<p>组件是功能模块的打包单位，包含相关的函数、实体和资源。</p>
<h3 id="组件定义-manifest-json" tabindex="-1"><a class="header-anchor" href="#组件定义-manifest-json"><span>组件定义（manifest.json）</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player-management"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"玩家管理"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"玩家管理功能模块"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"dependencies"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"author"</span><span class="token operator">:</span> <span class="token string">"Croupier Team"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.get"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.update"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.unban"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.list"</span><span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"resources"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.resource"</span><span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="组件打包" tabindex="-1"><a class="header-anchor" href="#组件打包"><span>组件打包</span></a></h3>
<p>组件被打包成 <code v-pre>.tgz</code> 文件：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">player-management-1.0.0.tgz</span>
<span class="line">├── manifest.json</span>
<span class="line">└── descriptors/</span>
<span class="line">    ├── player.entity.json</span>
<span class="line">    ├── player.resource.json</span>
<span class="line">    ├── player.register.json</span>
<span class="line">    ├── player.get.json</span>
<span class="line">    ├── player.update.json</span>
<span class="line">    ├── player.ban.json</span>
<span class="line">    └── player.list.json</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="ui-自动生成" tabindex="-1"><a class="header-anchor" href="#ui-自动生成"><span>UI 自动生成</span></a></h2>
<p>基于定义自动生成：</p>
<h3 id="列表页面" tabindex="-1"><a class="header-anchor" href="#列表页面"><span>列表页面</span></a></h3>
<div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre v-pre><code class="language-typescript"><span class="line"><span class="token comment">// 自动生成 ProTable 配置</span></span>
<span class="line"><span class="token keyword">const</span> tableConfig <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">  columns<span class="token operator">:</span> resource<span class="token punctuation">.</span>ui<span class="token punctuation">.</span>columns<span class="token punctuation">,</span></span>
<span class="line">  <span class="token function-variable function">request</span><span class="token operator">:</span> <span class="token keyword">async</span> <span class="token punctuation">(</span>params<span class="token punctuation">)</span> <span class="token operator">=></span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">await</span> <span class="token function">invokeFunction</span><span class="token punctuation">(</span><span class="token string">'player.list'</span><span class="token punctuation">,</span> params<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token function-variable function">toolBarRender</span><span class="token operator">:</span> <span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">=></span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token operator">&lt;</span>Button type<span class="token operator">=</span><span class="token string">"primary"</span><span class="token operator">></span>新建<span class="token operator">&lt;</span><span class="token operator">/</span>Button<span class="token operator">></span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="表单页面" tabindex="-1"><a class="header-anchor" href="#表单页面"><span>表单页面</span></a></h3>
<div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre v-pre><code class="language-typescript"><span class="line"><span class="token comment">// 基于 Schema 自动生成表单</span></span>
<span class="line"><span class="token keyword">const</span> formConfig <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">  schema<span class="token operator">:</span> entity<span class="token punctuation">.</span>schema<span class="token punctuation">,</span></span>
<span class="line">  <span class="token function-variable function">onFinish</span><span class="token operator">:</span> <span class="token keyword">async</span> <span class="token punctuation">(</span>values<span class="token punctuation">)</span> <span class="token operator">=></span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">await</span> <span class="token function">invokeFunction</span><span class="token punctuation">(</span><span class="token string">'player.update'</span><span class="token punctuation">,</span> values<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="对象绑定机制" tabindex="-1"><a class="header-anchor" href="#对象绑定机制"><span>对象绑定机制</span></a></h2>
<p>函数通过 <code v-pre>entity</code> 字段绑定到业务对象：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"operation"</span><span class="token operator">:</span> <span class="token string">"delete"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>这种绑定使得：</p>
<ol>
<li>UI 可以自动识别函数属于哪个实体</li>
<li>可以批量生成 CRUD 操作</li>
<li>支持通用的权限控制</li>
</ol>
<h2 id="目录结构" tabindex="-1"><a class="header-anchor" href="#目录结构"><span>目录结构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">components/</span>
<span class="line">├── player-management/</span>
<span class="line">│   ├── manifest.json              # 组件清单</span>
<span class="line">│   └── descriptors/</span>
<span class="line">│       ├── player.entity.json     # 玩家实体</span>
<span class="line">│       ├── player.resource.json   # 玩家资源</span>
<span class="line">│       ├── player.register.json   # 注册函数</span>
<span class="line">│       ├── player.get.json        # 获取函数</span>
<span class="line">│       ├── player.update.json     # 更新函数</span>
<span class="line">│       └── player.ban.json        # 封禁函数</span>
<span class="line">├── item-management/</span>
<span class="line">│   ├── manifest.json</span>
<span class="line">│   └── descriptors/</span>
<span class="line">│       ├── item.entity.json</span>
<span class="line">│       └── ...</span>
<span class="line">└── economy-system/</span>
<span class="line">    ├── manifest.json</span>
<span class="line">    └── descriptors/</span>
<span class="line">        ├── currency.resource.json</span>
<span class="line">        └── ...</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/guide/concepts/function-management.html">函数管理</RouteLink></li>
<li><RouteLink to="/guide/concepts/descriptor-driven-ui.html">描述符驱动 UI</RouteLink></li>
<li><RouteLink to="/architecture/">系统架构</RouteLink></li>
</ul>
</div></template>


