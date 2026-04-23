<template><div><h1 id="croupier-虚拟对象-virtual-object-架构完整分析" tabindex="-1"><a class="header-anchor" href="#croupier-虚拟对象-virtual-object-架构完整分析"><span>Croupier 虚拟对象(Virtual Object)架构完整分析</span></a></h1>
<h2 id="📚-执行摘要" tabindex="-1"><a class="header-anchor" href="#📚-执行摘要"><span>📚 执行摘要</span></a></h2>
<p>Croupier 项目中的&quot;虚拟对象&quot;概念是一套完整的<strong>对象驱动的组件化管理系统</strong>，基于 JSON Schema 实现低代码/无代码管理界面生成。虚拟对象在系统中被称为 <strong>Entity(实体)</strong>，通过 <strong>Resource(资源)</strong> 进行 UI 层级的函数组合，最终打包为 <strong>Component(组件)</strong> 进行模块化管理。</p>
<hr>
<h2 id="_1️⃣-虚拟对象的核心定义" tabindex="-1"><a class="header-anchor" href="#_1️⃣-虚拟对象的核心定义"><span>1️⃣ 虚拟对象的核心定义</span></a></h2>
<h3 id="_1-1-概念模型" tabindex="-1"><a class="header-anchor" href="#_1-1-概念模型"><span>1.1 概念模型</span></a></h3>
<p>在 Croupier 中，&quot;虚拟对象&quot;对应以下三个层级的概念：</p>
<table>
<thead>
<tr>
<th>层级</th>
<th>中文名</th>
<th>文件格式</th>
<th>作用</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Entity</strong></td>
<td>实体/虚拟对象</td>
<td><code v-pre>*.entity.json</code></td>
<td>定义业务对象的完整描述，包括数据结构、UI配置、操作映射</td>
</tr>
<tr>
<td><strong>Function</strong></td>
<td>函数</td>
<td><code v-pre>*.json</code></td>
<td>具体的业务操作实现，包含输入输出Schema、权限、语义</td>
</tr>
<tr>
<td><strong>Resource</strong></td>
<td>资源</td>
<td><code v-pre>*.resource.json</code></td>
<td>UI层面的操作集合，将多个函数组合成完整的管理界面</td>
</tr>
<tr>
<td><strong>Component</strong></td>
<td>组件</td>
<td><code v-pre>manifest.json</code></td>
<td>功能模块的打包单位，包含entities、functions、resources</td>
</tr>
</tbody>
</table>
<h3 id="_1-2-official-definition" tabindex="-1"><a class="header-anchor" href="#_1-2-official-definition"><span>1.2 Official Definition</span></a></h3>
<p>根据 <code v-pre>docs/providers-manifest.md</code> 的定义：</p>
<blockquote>
<p><strong>entity（实体/虚拟对象）</strong>：业务对象类型（可&quot;虚拟&quot;，仅有上下文/生命周期），含对象 schema 及一组操作（create/get/update/delete/custom…）。每个操作独立声明参数、权限、目标定位方式（如何找到某个对象实例）。</p>
</blockquote>
<p>关键特征：</p>
<ul>
<li><strong>虚拟化</strong>：可以是纯粹的上下文对象，不一定对应数据库表</li>
<li><strong>多操作</strong>：支持标准CRUD和自定义操作</li>
<li><strong>目标定位</strong>：通过 <code v-pre>target</code> 字段指定如何找到对象实例</li>
<li><strong>权限隔离</strong>：每个操作独立的权限声明</li>
</ul>
<hr>
<h2 id="_2️⃣-虚拟对象的设计文档" tabindex="-1"><a class="header-anchor" href="#_2️⃣-虚拟对象的设计文档"><span>2️⃣ 虚拟对象的设计文档</span></a></h2>
<h3 id="_2-1-official-documents" tabindex="-1"><a class="header-anchor" href="#_2-1-official-documents"><span>2.1 Official Documents</span></a></h3>
<h4 id="📄-docs-providers-manifest-md" tabindex="-1"><a class="header-anchor" href="#📄-docs-providers-manifest-md"><span>📄 <code v-pre>docs/providers-manifest.md</code></span></a></h4>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">路径: /Users/cui/Workspaces/croupier/docs/providers-manifest.md</span>
<span class="line">大小: 111 行</span>
<span class="line">内容: Provider Manifest 设计说明</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>核心内容：</p>
<ul>
<li><strong>Manifest 文件结构</strong>：JSON格式，包含provider元信息、functions数组、entities数组</li>
<li><strong>参数定义与校验</strong>：首选JSON-Schema，支持x-ui扩展、x-mask敏感字段标记</li>
<li><strong>虚拟对象操作定义</strong>：
<ul>
<li><code v-pre>op</code>：操作类型(create/get/update/delete/custom)</li>
<li><code v-pre>target</code>：定位方式(field或jsonpath)</li>
<li><code v-pre>request/response</code>：Schema或Proto FQN</li>
<li><code v-pre>auth.require</code>：权限要求</li>
<li><code v-pre>semantics</code>：限流、并发、幂等性等</li>
</ul>
</li>
</ul>
<h4 id="📄-docs-providers-manifest-schema-json" tabindex="-1"><a class="header-anchor" href="#📄-docs-providers-manifest-schema-json"><span>📄 <code v-pre>docs/providers-manifest.schema.json</code></span></a></h4>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">路径: /Users/cui/Workspaces/croupier/docs/providers-manifest.schema.json</span>
<span class="line">大小: 167 行</span>
<span class="line">内容: Manifest JSON Schema 验证规范</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>核心schema定义：</p>
<ul>
<li><code v-pre>entity</code>：id, title, color, schema, operations[]</li>
<li><code v-pre>operation</code>：op, target, request, response, auth, semantics, transport, routing, ui</li>
</ul>
<h4 id="📄-architecture-md" tabindex="-1"><a class="header-anchor" href="#📄-architecture-md"><span>📄 <code v-pre>ARCHITECTURE.md</code></span></a></h4>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">路径: /Users/cui/Workspaces/croupier/ARCHITECTURE.md</span>
<span class="line">大小: 323 行</span>
<span class="line">内容: 对象驱动系统的完整架构文档</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>三层架构：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────┐</span>
<span class="line">│   UI Resource Layer         │  ← 资源配置层</span>
<span class="line">│  player.resource, ...       │</span>
<span class="line">└─────────────────────────────┘</span>
<span class="line">           ↓</span>
<span class="line">┌─────────────────────────────┐</span>
<span class="line">│   Entity Definition Layer    │  ← 实体定义层</span>
<span class="line">│  player.entity, ...         │</span>
<span class="line">└─────────────────────────────┘</span>
<span class="line">           ↓</span>
<span class="line">┌─────────────────────────────┐</span>
<span class="line">│   Function Layer            │  ← 函数实现层</span>
<span class="line">│  player.register, ...       │</span>
<span class="line">└─────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="_3️⃣-虚拟对象如何组合多个函数" tabindex="-1"><a class="header-anchor" href="#_3️⃣-虚拟对象如何组合多个函数"><span>3️⃣ 虚拟对象如何组合多个函数</span></a></h2>
<h3 id="_3-1-函数绑定机制" tabindex="-1"><a class="header-anchor" href="#_3-1-函数绑定机制"><span>3.1 函数绑定机制</span></a></h3>
<p>每个函数通过 <code v-pre>entity</code> 字段绑定到虚拟对象：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"operation"</span><span class="token operator">:</span> <span class="token string">"create"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* 输入 Schema */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* 输出 Schema */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* 权限控制 */</span> <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-2-entity-definition-中的操作映射" tabindex="-1"><a class="header-anchor" href="#_3-2-entity-definition-中的操作映射"><span>3.2 Entity Definition 中的操作映射</span></a></h3>
<p>Entity定义了哪些函数操作该对象：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* JSON Schema */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"create"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.register"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.profile.get"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"update"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.profile.update"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"delete"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.ban"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"list"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.list"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"unban"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.unban"</span><span class="token punctuation">]</span>  <span class="token comment">// 自定义操作</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"display_field"</span><span class="token operator">:</span> <span class="token string">"username"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"title_template"</span><span class="token operator">:</span> <span class="token string">"{username} ({nickname})"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"avatar_field"</span><span class="token operator">:</span> <span class="token string">"avatar_url"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"status_field"</span><span class="token operator">:</span> <span class="token string">"status"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>关键点</strong>：</p>
<ul>
<li>每个操作可以绑定<strong>单个函数</strong>或<strong>函数数组</strong></li>
<li>支持标准CRUD和自定义操作</li>
<li>UI配置指定显示方式</li>
</ul>
<h3 id="_3-3-resource-definition-中的函数组合" tabindex="-1"><a class="header-anchor" href="#_3-3-resource-definition-中的函数组合"><span>3.3 Resource Definition 中的函数组合</span></a></h3>
<p>Resource 在UI层面组合函数成完整的管理界面：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"玩家"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"primary_key"</span><span class="token operator">:</span> <span class="token string">"player_id"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"display_field"</span><span class="token operator">:</span> <span class="token string">"username"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"create"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"注册玩家"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"icon"</span><span class="token operator">:</span> <span class="token string">"UserAddOutlined"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.profile.get"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"查看详情"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"icon"</span><span class="token operator">:</span> <span class="token string">"EyeOutlined"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"update"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.profile.update"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"编辑资料"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"icon"</span><span class="token operator">:</span> <span class="token string">"EditOutlined"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"delete"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"封禁玩家"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"icon"</span><span class="token operator">:</span> <span class="token string">"StopOutlined"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"danger"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"list"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.list"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"玩家列表"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"icon"</span><span class="token operator">:</span> <span class="token string">"TableOutlined"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"pro-table"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"layout"</span><span class="token operator">:</span> <span class="token string">"table"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"columns"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">      <span class="token comment">/* ProTable列定义 */</span></span>
<span class="line">    <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"toolbar"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"create"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"row"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">,</span> <span class="token string">"update"</span><span class="token punctuation">,</span> <span class="token string">"delete"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"features"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"searchable"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"pagination"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"sortable"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"filterable"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"exportable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player:manage"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"has_role('gm') || has_role('admin')"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"cacheable"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"cache_ttl"</span><span class="token operator">:</span> <span class="token string">"5m"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="_4️⃣-现有的虚拟对象实现代码" tabindex="-1"><a class="header-anchor" href="#_4️⃣-现有的虚拟对象实现代码"><span>4️⃣ 现有的虚拟对象实现代码</span></a></h2>
<h3 id="_4-1-后端实现-go" tabindex="-1"><a class="header-anchor" href="#_4-1-后端实现-go"><span>4.1 后端实现（Go）</span></a></h3>
<h4 id="📝-entity-验证-internal-validation-entity-go" tabindex="-1"><a class="header-anchor" href="#📝-entity-验证-internal-validation-entity-go"><span>📝 Entity 验证（<code v-pre>internal/validation/entity.go</code>）</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> validation</span>
<span class="line"></span>
<span class="line"><span class="token comment">// ValidateEntityDefinition validates an entity definition structure</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">ValidateEntityDefinition</span><span class="token punctuation">(</span>entity <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any<span class="token punctuation">)</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 检查必需字段：id, type, schema</span></span>
<span class="line">    <span class="token comment">// 验证JSON Schema结构</span></span>
<span class="line">    <span class="token comment">// 验证operations映射</span></span>
<span class="line">    <span class="token comment">// 验证UI配置</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// validateJSONSchema 验证JSON Schema本身</span></span>
<span class="line"><span class="token comment">// validateSchemaProperties 验证各属性定义</span></span>
<span class="line"><span class="token comment">// validateOperations 验证操作映射</span></span>
<span class="line"><span class="token comment">// validateUIConfig 验证UI配置</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>功能：</p>
<ul>
<li>验证entity的id和type字段</li>
<li>验证schema是否为有效的JSON Schema</li>
<li>验证operations映射的操作名称和函数ID</li>
<li>验证UI配置的字段</li>
</ul>
<h4 id="📝-descriptor-loader-internal-function-descriptor-loader-go" tabindex="-1"><a class="header-anchor" href="#📝-descriptor-loader-internal-function-descriptor-loader-go"><span>📝 Descriptor Loader（<code v-pre>internal/function/descriptor/loader.go</code>）</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> descriptor</span>
<span class="line"></span>
<span class="line"><span class="token comment">// Descriptor is a simplified function descriptor model for UI/validation</span></span>
<span class="line"><span class="token keyword">type</span> Descriptor <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    ID        <span class="token builtin">string</span>         <span class="token string">`json:"id"`</span></span>
<span class="line">    Version   <span class="token builtin">string</span>         <span class="token string">`json:"version"`</span></span>
<span class="line">    Category  <span class="token builtin">string</span>         <span class="token string">`json:"category"`</span></span>
<span class="line">    Risk      <span class="token builtin">string</span>         <span class="token string">`json:"risk"`</span></span>
<span class="line">    Auth      <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any <span class="token string">`json:"auth"`</span></span>
<span class="line">    Params    <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any <span class="token string">`json:"params"`</span></span>
<span class="line">    Semantics <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any <span class="token string">`json:"semantics"`</span></span>
<span class="line">    Transport <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any <span class="token string">`json:"transport"`</span></span>
<span class="line">    Outputs   <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any <span class="token string">`json:"outputs"`</span></span>
<span class="line">    UI        <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span>any <span class="token string">`json:"ui"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// LoadAll 从目录递归加载所有描述符</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">LoadAll</span><span class="token punctuation">(</span>dir <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token operator">*</span>Descriptor<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>特点：</p>
<ul>
<li>递归扫描目录加载所有.json文件</li>
<li>跳过ui子目录和无id字段的文件</li>
<li>构建统一的Descriptor结构供UI和校验使用</li>
</ul>
<h4 id="📝-component-manager-internal-pack-manager-go" tabindex="-1"><a class="header-anchor" href="#📝-component-manager-internal-pack-manager-go"><span>📝 Component Manager（<code v-pre>internal/pack/manager.go</code>）</span></a></h4>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> pack</span>
<span class="line"></span>
<span class="line"><span class="token comment">// ComponentManager manages function components</span></span>
<span class="line"><span class="token keyword">type</span> ComponentManager <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    dataDir      <span class="token builtin">string</span></span>
<span class="line">    installedDir <span class="token builtin">string</span></span>
<span class="line">    disabledDir  <span class="token builtin">string</span></span>
<span class="line">    registry     <span class="token operator">*</span>ComponentRegistry</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">type</span> ComponentManifest <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    ID           <span class="token builtin">string</span>              <span class="token string">`json:"id"`</span></span>
<span class="line">    Name         <span class="token builtin">string</span>              <span class="token string">`json:"name"`</span></span>
<span class="line">    Version      <span class="token builtin">string</span>              <span class="token string">`json:"version"`</span></span>
<span class="line">    Description  <span class="token builtin">string</span>              <span class="token string">`json:"description"`</span></span>
<span class="line">    Category     <span class="token builtin">string</span>              <span class="token string">`json:"category"`</span> <span class="token comment">// player, item, economy, social, etc.</span></span>
<span class="line">    Dependencies <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span>            <span class="token string">`json:"dependencies,omitempty"`</span></span>
<span class="line">    Functions    <span class="token punctuation">[</span><span class="token punctuation">]</span>ComponentFunction <span class="token string">`json:"functions"`</span></span>
<span class="line">    Author       <span class="token builtin">string</span>              <span class="token string">`json:"author,omitempty"`</span></span>
<span class="line">    License      <span class="token builtin">string</span>              <span class="token string">`json:"license,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 主要方法：</span></span>
<span class="line"><span class="token comment">// InstallComponent(componentPath) - 安装组件</span></span>
<span class="line"><span class="token comment">// UninstallComponent(componentID) - 卸载组件</span></span>
<span class="line"><span class="token comment">// EnableComponent(componentID) - 启用组件</span></span>
<span class="line"><span class="token comment">// DisableComponent(componentID) - 禁用组件</span></span>
<span class="line"><span class="token comment">// LoadRegistry() - 加载组件注册表</span></span>
<span class="line"><span class="token comment">// SaveRegistry() - 保存组件注册表</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-2-http-api-实现" tabindex="-1"><a class="header-anchor" href="#_4-2-http-api-实现"><span>4.2 HTTP API 实现</span></a></h3>
<h4 id="📝-entity-管理-api-go-zero-services-server-internal-handler-entity" tabindex="-1"><a class="header-anchor" href="#📝-entity-管理-api-go-zero-services-server-internal-handler-entity"><span>📝 Entity 管理 API（go-zero <code v-pre>services/server/internal/handler/entity/*</code>）</span></a></h4>
<table>
<thead>
<tr>
<th>端点</th>
<th>方法</th>
<th>权限</th>
<th>功能</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>/api/v1/entities</code></td>
<td>GET</td>
<td><code v-pre>entities:read</code></td>
<td>获取所有 entity 定义</td>
</tr>
<tr>
<td><code v-pre>/api/v1/entities</code></td>
<td>POST</td>
<td><code v-pre>entities:create</code></td>
<td>创建新 entity</td>
</tr>
<tr>
<td><code v-pre>/api/v1/entities/:id</code></td>
<td>GET</td>
<td><code v-pre>entities:read</code></td>
<td>获取特定 entity</td>
</tr>
<tr>
<td><code v-pre>/api/v1/entities/:id</code></td>
<td>PUT</td>
<td><code v-pre>entities:update</code></td>
<td>更新 entity 定义</td>
</tr>
<tr>
<td><code v-pre>/api/v1/entities/:id</code></td>
<td>DELETE</td>
<td><code v-pre>entities:delete</code></td>
<td>删除 entity</td>
</tr>
<tr>
<td><code v-pre>/api/v1/entities/validate</code></td>
<td>POST</td>
<td><code v-pre>entities:read</code></td>
<td>验证 entity</td>
</tr>
<tr>
<td><code v-pre>/api/v1/entities/:id/preview</code></td>
<td>GET</td>
<td><code v-pre>entities:read</code></td>
<td>预览 entity UI</td>
</tr>
</tbody>
</table>
<p>实现细节：</p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// GET /api/v1/entities 扫描所有components目录</span></span>
<span class="line"><span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> entry <span class="token operator">:=</span> <span class="token keyword">range</span> entries <span class="token punctuation">{</span>  <span class="token comment">// 遍历components</span></span>
<span class="line">    descriptorsDir <span class="token operator">:=</span> filepath<span class="token punctuation">.</span><span class="token function">Join</span><span class="token punctuation">(</span>componentsDir<span class="token punctuation">,</span> entry<span class="token punctuation">.</span><span class="token function">Name</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token string">"descriptors"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> file <span class="token operator">:=</span> <span class="token keyword">range</span> descriptorFiles <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token operator">!</span>strings<span class="token punctuation">.</span><span class="token function">HasSuffix</span><span class="token punctuation">(</span>file<span class="token punctuation">.</span><span class="token function">Name</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token string">".entity.json"</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">continue</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token comment">// 读取并解析entity定义</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// POST /api/v1/entities 保存到指定component</span></span>
<span class="line">componentDir <span class="token operator">:=</span> filepath<span class="token punctuation">.</span><span class="token function">Join</span><span class="token punctuation">(</span><span class="token string">"components"</span><span class="token punctuation">,</span> component<span class="token punctuation">)</span></span>
<span class="line">descriptorsDir <span class="token operator">:=</span> filepath<span class="token punctuation">.</span><span class="token function">Join</span><span class="token punctuation">(</span>componentDir<span class="token punctuation">,</span> <span class="token string">"descriptors"</span><span class="token punctuation">)</span></span>
<span class="line">entityFile <span class="token operator">:=</span> filepath<span class="token punctuation">.</span><span class="token function">Join</span><span class="token punctuation">(</span>descriptorsDir<span class="token punctuation">,</span> id<span class="token operator">+</span><span class="token string">".entity.json"</span><span class="token punctuation">)</span></span>
<span class="line">os<span class="token punctuation">.</span><span class="token function">WriteFile</span><span class="token punctuation">(</span>entityFile<span class="token punctuation">,</span> entityData<span class="token punctuation">,</span> <span class="token number">0644</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="📝-descriptor-api-go-zero-rest-示例" tabindex="-1"><a class="header-anchor" href="#📝-descriptor-api-go-zero-rest-示例"><span>📝 Descriptor API（go-zero rest 示例）</span></a></h4>
<blockquote>
<p>说明：当前服务基于 go-zero（<code v-pre>rest</code> + <code v-pre>httpx</code>）。以下为示例写法与现有路由风格一致。</p>
</blockquote>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// GET /api/v1/functions/descriptors</span></span>
<span class="line">server<span class="token punctuation">.</span><span class="token function">AddRoutes</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>rest<span class="token punctuation">.</span>Route<span class="token punctuation">{</span></span>
<span class="line">	<span class="token punctuation">{</span>Method<span class="token punctuation">:</span> http<span class="token punctuation">.</span>MethodGet<span class="token punctuation">,</span> Path<span class="token punctuation">:</span> <span class="token string">"/descriptors"</span><span class="token punctuation">,</span> Handler<span class="token punctuation">:</span> function<span class="token punctuation">.</span><span class="token function">DescriptorsHandler</span><span class="token punctuation">(</span>serverCtx<span class="token punctuation">)</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">,</span> rest<span class="token punctuation">.</span><span class="token function">WithPrefix</span><span class="token punctuation">(</span><span class="token string">"/api/v1/functions"</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// GET /api/v1/providers/:id/entities（id 可为 "*" 聚合全部）</span></span>
<span class="line">server<span class="token punctuation">.</span><span class="token function">AddRoutes</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span>rest<span class="token punctuation">.</span>Route<span class="token punctuation">{</span></span>
<span class="line">	<span class="token punctuation">{</span>Method<span class="token punctuation">:</span> http<span class="token punctuation">.</span>MethodGet<span class="token punctuation">,</span> Path<span class="token punctuation">:</span> <span class="token string">"/:id/entities"</span><span class="token punctuation">,</span> Handler<span class="token punctuation">:</span> provider<span class="token punctuation">.</span><span class="token function">ProvidersEntitiesHandler</span><span class="token punctuation">(</span>serverCtx<span class="token punctuation">)</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">,</span> rest<span class="token punctuation">.</span><span class="token function">WithPrefix</span><span class="token punctuation">(</span><span class="token string">"/api/v1/providers"</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="_5️⃣-虚拟对象的配置和管理机制" tabindex="-1"><a class="header-anchor" href="#_5️⃣-虚拟对象的配置和管理机制"><span>5️⃣ 虚拟对象的配置和管理机制</span></a></h2>
<h3 id="_5-1-文件组织结构" tabindex="-1"><a class="header-anchor" href="#_5-1-文件组织结构"><span>5.1 文件组织结构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">components/</span>
<span class="line">├── player-management/          # 组件目录</span>
<span class="line">│   ├── manifest.json           # 组件清单</span>
<span class="line">│   └── descriptors/</span>
<span class="line">│       ├── player.entity.json  # Entity定义</span>
<span class="line">│       ├── player.resource.json # Resource定义</span>
<span class="line">│       ├── player.register.json # Function定义</span>
<span class="line">│       ├── player.profile.get.json</span>
<span class="line">│       ├── player.profile.update.json</span>
<span class="line">│       ├── player.ban.json</span>
<span class="line">│       ├── player.unban.json</span>
<span class="line">│       └── player.list.json</span>
<span class="line">│</span>
<span class="line">├── item-management/</span>
<span class="line">│   ├── manifest.json</span>
<span class="line">│   └── descriptors/</span>
<span class="line">│       ├── item.entity.json</span>
<span class="line">│       ├── item.resource.json</span>
<span class="line">│       ├── item.create.json</span>
<span class="line">│       ├── item.get.json</span>
<span class="line">│       ├── item.list.json</span>
<span class="line">│       ├── item.update.json</span>
<span class="line">│       └── item.delete.json</span>
<span class="line">│</span>
<span class="line">├── economy-system/</span>
<span class="line">│   ├── manifest.json</span>
<span class="line">│   └── descriptors/</span>
<span class="line">│       ├── currency.entity.json</span>
<span class="line">│       ├── currency.resource.json</span>
<span class="line">│       ├── wallet.entity.json</span>
<span class="line">│       ├── wallet.resource.json</span>
<span class="line">│       └── ...</span>
<span class="line">│</span>
<span class="line">└── entity-management/          # 虚拟对象管理系统本身</span>
<span class="line">    ├── manifest.json</span>
<span class="line">    └── descriptors/</span>
<span class="line">        ├── entity.resource.json     # Entity的资源配置</span>
<span class="line">        ├── entity.create.json       # 创建entity函数</span>
<span class="line">        ├── entity.update.json</span>
<span class="line">        ├── entity.preview.json</span>
<span class="line">        └── schema.validate.json</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-2-manifest-配置格式" tabindex="-1"><a class="header-anchor" href="#_5-2-manifest-配置格式"><span>5.2 Manifest 配置格式</span></a></h3>
<h4 id="🔧-component-manifest-manifest-json" tabindex="-1"><a class="header-anchor" href="#🔧-component-manifest-manifest-json"><span>🔧 Component Manifest (<code v-pre>manifest.json</code>)</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player-management"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player Management System"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Core player operations..."</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"dependencies"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Player business object"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Register a new player"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.profile.get"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Get player profile"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"author"</span><span class="token operator">:</span> <span class="token string">"Croupier Team"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"license"</span><span class="token operator">:</span> <span class="token string">"MIT"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="🔧-entity-definition-entity-json" tabindex="-1"><a class="header-anchor" href="#🔧-entity-definition-entity-json"><span>🔧 Entity Definition (<code v-pre>*.entity.json</code>)</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player Entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Player business object definition"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Unique player identifier"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"primary_key"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"username"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Player username"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"unique"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"searchable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"status"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"enum"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"active"</span><span class="token punctuation">,</span> <span class="token string">"banned"</span><span class="token punctuation">,</span> <span class="token string">"suspended"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"filterable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"required"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">,</span> <span class="token string">"username"</span><span class="token punctuation">,</span> <span class="token string">"email"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"create"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.register"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.profile.get"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"update"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.profile.update"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"delete"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.ban"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"list"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.list"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"unban"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.unban"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"display_field"</span><span class="token operator">:</span> <span class="token string">"username"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"title_template"</span><span class="token operator">:</span> <span class="token string">"{username} ({nickname})"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"avatar_field"</span><span class="token operator">:</span> <span class="token string">"avatar_url"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"status_field"</span><span class="token operator">:</span> <span class="token string">"status"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="🔧-resource-definition-resource-json" tabindex="-1"><a class="header-anchor" href="#🔧-resource-definition-resource-json"><span>🔧 Resource Definition (<code v-pre>*.resource.json</code>)</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player Resource Management"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"玩家"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"primary_key"</span><span class="token operator">:</span> <span class="token string">"player_id"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"create"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"注册玩家"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"icon"</span><span class="token operator">:</span> <span class="token string">"UserAddOutlined"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"function"</span><span class="token operator">:</span> <span class="token string">"player.profile.get"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"查看详情"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"icon"</span><span class="token operator">:</span> <span class="token string">"EyeOutlined"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"pro-table"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"columns"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"player_id"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"ID"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"width"</span><span class="token operator">:</span> <span class="token number">100</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"fixed"</span><span class="token operator">:</span> <span class="token string">"left"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"username"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"用户名"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"width"</span><span class="token operator">:</span> <span class="token number">120</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"searchable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"actions"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"toolbar"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"create"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"row"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">,</span> <span class="token string">"update"</span><span class="token punctuation">,</span> <span class="token string">"delete"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player:manage"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"has_role('gm') || has_role('admin')"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"cacheable"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"cache_ttl"</span><span class="token operator">:</span> <span class="token string">"5m"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="🔧-function-definition-json" tabindex="-1"><a class="header-anchor" href="#🔧-function-definition-json"><span>🔧 Function Definition (<code v-pre>*.json</code>)</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player Registration"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"operation"</span><span class="token operator">:</span> <span class="token string">"create"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"username"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"pattern"</span><span class="token operator">:</span> <span class="token string">"^[a-zA-Z0-9_]{3,16}$"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"email"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"format"</span><span class="token operator">:</span> <span class="token string">"email"</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"required"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"username"</span><span class="token punctuation">,</span> <span class="token string">"email"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"status"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player:register"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"has_role('gm') || has_role('admin')"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"idempotent"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"rate_limit"</span><span class="token operator">:</span> <span class="token string">"10rps"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"timeout"</span><span class="token operator">:</span> <span class="token string">"30s"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-3-provider-manifest-格式-新的统一标准" tabindex="-1"><a class="header-anchor" href="#_5-3-provider-manifest-格式-新的统一标准"><span>5.3 Provider Manifest 格式（新的统一标准）</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"provider"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.2.0"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"lang"</span><span class="token operator">:</span> <span class="token string">"python"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"sdk"</span><span class="token operator">:</span> <span class="token string">"croupier-py@0.3.0"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/ban_request.json"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"response"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/ban_response.json"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"require"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player:ban"</span><span class="token punctuation">]</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"idempotent"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span> <span class="token property">"rate_limit"</span><span class="token operator">:</span> <span class="token string">"100/s"</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"session"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"Session"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"color"</span><span class="token operator">:</span> <span class="token string">"#1677ff"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/session.json"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"op"</span><span class="token operator">:</span> <span class="token string">"create"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/create_session_request.json"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"response"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/session.json"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"require"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"session:create"</span><span class="token punctuation">]</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"op"</span><span class="token operator">:</span> <span class="token string">"close"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"target"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"field"</span><span class="token operator">:</span> <span class="token string">"session_id"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/close_request.json"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"response"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/empty.json"</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="_6️⃣-虚拟对象在函数注册表中的表示" tabindex="-1"><a class="header-anchor" href="#_6️⃣-虚拟对象在函数注册表中的表示"><span>6️⃣ 虚拟对象在函数注册表中的表示</span></a></h2>
<h3 id="_6-1-provider-capabilities-注册" tabindex="-1"><a class="header-anchor" href="#_6-1-provider-capabilities-注册"><span>6.1 Provider Capabilities 注册</span></a></h3>
<h4 id="http-api-注册流程" tabindex="-1"><a class="header-anchor" href="#http-api-注册流程"><span>HTTP API 注册流程</span></a></h4>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Provider Process</span>
<span class="line">    ↓</span>
<span class="line">GET /api/v1/providers/capabilities</span>
<span class="line">    ↓</span>
<span class="line">POST /api/v1/providers/:id/reload（按需刷新）</span>
<span class="line">    ↓</span>
<span class="line">Server Registry</span>
<span class="line">    ├── 验证manifest JSON</span>
<span class="line">    ├── 保存到registry</span>
<span class="line">    ├── 合并provider functions到descriptors</span>
<span class="line">    ├── 暴露 /api/v1/functions/descriptors</span>
<span class="line">    └── 暴露 /api/v1/providers/descriptors</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="实现说明-go-zero-现状" tabindex="-1"><a class="header-anchor" href="#实现说明-go-zero-现状"><span>实现说明（go-zero 现状）</span></a></h4>
<ul>
<li>当前对外能力以查询为主：<code v-pre>GET /api/v1/providers/capabilities</code> 返回 Registry 中已加载的 Provider 能力列表</li>
<li>Provider 刷新：<code v-pre>POST /api/v1/providers/:id/reload</code>（触发重新加载/重建 Registry）</li>
</ul>
<h3 id="_6-2-unified-descriptors-构建" tabindex="-1"><a class="header-anchor" href="#_6-2-unified-descriptors-构建"><span>6.2 Unified Descriptors 构建</span></a></h3>
<h4 id="api-端点-go-zero-现状" tabindex="-1"><a class="header-anchor" href="#api-端点-go-zero-现状"><span>API 端点（go-zero 现状）</span></a></h4>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">GET /api/v1/functions/descriptors      # 函数描述符模板列表（用于 UI 选择/渲染）</span>
<span class="line">GET /api/v1/providers/capabilities     # Provider 能力列表</span>
<span class="line">GET /api/v1/providers/descriptors      # Provider manifest 聚合后的 descriptors</span>
<span class="line">GET /api/v1/providers/:id/entities     # Provider entities（id="*" 聚合全部）</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_6-3-entity-与-operation-的关系" tabindex="-1"><a class="header-anchor" href="#_6-3-entity-与-operation-的关系"><span>6.3 Entity 与 Operation 的关系</span></a></h3>
<p>在函数注册表中，Entity 和 Operation 的关系：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Entity (player.entity)</span>
<span class="line">├── schema: { JSON Schema }</span>
<span class="line">├── operations:</span>
<span class="line">│   ├── create → "player.register" (Function)</span>
<span class="line">│   ├── read → "player.profile.get" (Function)</span>
<span class="line">│   ├── update → "player.profile.update" (Function)</span>
<span class="line">│   ├── delete → "player.ban" (Function)</span>
<span class="line">│   ├── list → "player.list" (Function)</span>
<span class="line">│   └── unban → "player.unban" (Function)</span>
<span class="line">└── ui:</span>
<span class="line">    ├── display_field</span>
<span class="line">    ├── title_template</span>
<span class="line">    └── ...</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>跨实体操作</strong>（如wallet.transfer）：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Transfer currency between wallets (cross-entity operation)"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"economy"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"from_player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"to_player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"currency_code"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"amount"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span>...<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"wallet:transfer"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"has_role('admin') || has_role('economy_manager')"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"risk"</span><span class="token operator">:</span> <span class="token string">"high"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"two_person_rule"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"atomic"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"idempotent"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="_7️⃣-agent-和-server-中的虚拟对象处理" tabindex="-1"><a class="header-anchor" href="#_7️⃣-agent-和-server-中的虚拟对象处理"><span>7️⃣ Agent 和 Server 中的虚拟对象处理</span></a></h2>
<h3 id="_7-1-server-端处理-http-层" tabindex="-1"><a class="header-anchor" href="#_7-1-server-端处理-http-层"><span>7.1 Server 端处理（HTTP 层）</span></a></h3>
<ol>
<li>
<p><strong>Descriptor 加载和缓存</strong></p>
<ul>
<li>启动时加载 <code v-pre>components/*/descriptors/*.json</code></li>
<li>构建 <code v-pre>s.descs</code> 和 <code v-pre>s.descIndex</code> 映射</li>
<li>支持动态加载新的provider manifest</li>
</ul>
</li>
<li>
<p><strong>Entity API 处理</strong></p>
<ul>
<li>扫描所有components目录查找entity定义</li>
<li>支持CRUD操作：create/read/update/delete</li>
<li>验证entity定义的有效性</li>
</ul>
</li>
<li>
<p><strong>Function Invocation</strong></p>
<ul>
<li>基于function_id查找descriptor</li>
<li>使用descriptor中的params验证请求</li>
<li>路由到相应的agent执行</li>
</ul>
</li>
</ol>
<h3 id="_7-2-前端集成-react-umi" tabindex="-1"><a class="header-anchor" href="#_7-2-前端集成-react-umi"><span>7.2 前端集成（React/Umi）</span></a></h3>
<h4 id="ui-自动生成流程" tabindex="-1"><a class="header-anchor" href="#ui-自动生成流程"><span>UI 自动生成流程</span></a></h4>
<div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre v-pre><code class="language-typescript"><span class="line"><span class="token comment">// 1. 从 /api/v1/providers/descriptors 获取 entity/resource 定义（聚合后的 provider manifests）</span></span>
<span class="line"><span class="token comment">//    具体取值按 resourceId/entityId 在返回的 descriptors 中查找</span></span>
<span class="line"><span class="token keyword">const</span> descriptors <span class="token operator">=</span> <span class="token keyword">await</span> <span class="token function">fetch</span><span class="token punctuation">(</span><span class="token string">'/api/v1/providers/descriptors'</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">then</span><span class="token punctuation">(</span>r <span class="token operator">=></span> r<span class="token punctuation">.</span><span class="token function">json</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 2. 基于 Resource Definition 生成 ProTable</span></span>
<span class="line"><span class="token operator">&lt;</span>ResourceTable</span>
<span class="line">  resourceId<span class="token operator">=</span><span class="token string">"player.resource"</span></span>
<span class="line">  <span class="token comment">// 自动读取操作、列定义、UI配置</span></span>
<span class="line"><span class="token operator">/</span><span class="token operator">></span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 3. 基于 Entity Definition 生成表单</span></span>
<span class="line"><span class="token operator">&lt;</span>EntityForm</span>
<span class="line">  entityId<span class="token operator">=</span><span class="token string">"player.entity"</span></span>
<span class="line">  operation<span class="token operator">=</span><span class="token string">"create"</span></span>
<span class="line">  <span class="token comment">// 基于entity的schema生成表单字段</span></span>
<span class="line"><span class="token operator">/</span><span class="token operator">></span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="前端组件-web-src-components-xentityform-tsx" tabindex="-1"><a class="header-anchor" href="#前端组件-web-src-components-xentityform-tsx"><span>前端组件（<code v-pre>web/src/components/XEntityForm.tsx</code>）</span></a></h4>
<p>存在前端组件用于渲染entity表单，自动生成基于schema的表单界面。</p>
<hr>
<h2 id="_8️⃣-与函数包-packs-系统的关系" tabindex="-1"><a class="header-anchor" href="#_8️⃣-与函数包-packs-系统的关系"><span>8️⃣ 与函数包(Packs)系统的关系</span></a></h2>
<h3 id="_8-1-pack-与-component-的关系" tabindex="-1"><a class="header-anchor" href="#_8-1-pack-与-component-的关系"><span>8.1 Pack 与 Component 的关系</span></a></h3>
<table>
<thead>
<tr>
<th>概念</th>
<th>定义</th>
<th>文件格式</th>
<th>作用</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Pack</strong></td>
<td>函数打包单位</td>
<td><code v-pre>.tgz</code></td>
<td>包含manifest、descriptors、schemas、UI资源</td>
</tr>
<tr>
<td><strong>Component</strong></td>
<td>组件打包单位</td>
<td><code v-pre>manifest.json</code></td>
<td>包含entities、functions、resources的模块</td>
</tr>
</tbody>
</table>
<h3 id="_8-2-pack-结构" tabindex="-1"><a class="header-anchor" href="#_8-2-pack-结构"><span>8.2 Pack 结构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">provider.tgz (或目录)</span>
<span class="line">├── manifest.json           # Provider 清单</span>
<span class="line">├── schema/                 # JSON Schemas</span>
<span class="line">│   ├── ban_request.json</span>
<span class="line">│   ├── ban_response.json</span>
<span class="line">│   └── ...</span>
<span class="line">├── ui/                     # UI 附加资源</span>
<span class="line">│   ├── custom_component.ts</span>
<span class="line">│   └── ...</span>
<span class="line">└── descriptors.fds         # 可选：FileDescriptorSet</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_8-3-pack-import-export-api" tabindex="-1"><a class="header-anchor" href="#_8-3-pack-import-export-api"><span>8.3 Pack Import/Export API</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">POST /api/v1/packs/import  # 导入 pack</span>
<span class="line">GET  /api/v1/packs/export  # 导出 pack（包含 descriptors/schemas/configs 等）</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="_9️⃣-实现现状" tabindex="-1"><a class="header-anchor" href="#_9️⃣-实现现状"><span>9️⃣ 实现现状</span></a></h2>
<h3 id="_9-1-已完成的功能" tabindex="-1"><a class="header-anchor" href="#_9-1-已完成的功能"><span>9.1 已完成的功能</span></a></h3>
<p>✅ <strong>核心架构</strong></p>
<ul>
<li>Entity、Function、Resource、Component 四层模型</li>
<li>JSON Schema 驱动的定义和验证</li>
<li>函数绑定到虚拟对象的机制</li>
</ul>
<p>✅ <strong>现有实现的组件</strong></p>
<ol>
<li>
<p><strong>player-management</strong></p>
<ul>
<li>player.entity.json：玩家实体定义</li>
<li>player.resource.json：玩家资源配置</li>
<li>player.register/get/update/ban/unban 等函数</li>
</ul>
</li>
<li>
<p><strong>item-management</strong></p>
<ul>
<li>item.entity.json：物品实体定义</li>
<li>item.resource.json：物品资源配置</li>
<li>item.create/get/list/update/delete 等函数</li>
</ul>
</li>
<li>
<p><strong>economy-system</strong></p>
<ul>
<li>currency.entity.json：货币实体定义</li>
<li>wallet.entity.json：钱包实体定义</li>
<li>货币和钱包相关操作</li>
<li><strong>跨实体操作</strong>：wallet.transfer（涉及player和currency两个entity）</li>
</ul>
</li>
<li>
<p><strong>entity-management</strong>（虚拟对象管理系统本身）</p>
<ul>
<li>entity.resource.json：Entity定义管理界面</li>
<li>entity.create/update/delete/preview 函数</li>
<li>schema.validate 函数</li>
</ul>
</li>
</ol>
<p>✅ <strong>后端 API</strong></p>
<ul>
<li><code v-pre>/api/v1/entities</code> - Entity CRUD</li>
<li><code v-pre>/api/v1/functions/descriptors</code> - 函数描述符模板列表</li>
<li><code v-pre>/api/v1/providers/capabilities</code> - Provider 能力列表（来自 Registry）</li>
<li><code v-pre>/api/v1/providers/descriptors</code> - Provider manifests 聚合 descriptors</li>
<li><code v-pre>/api/v1/providers/:id/entities</code> - Provider entities（id=&quot;*&quot; 聚合全部）</li>
</ul>
<p>✅ <strong>验证机制</strong></p>
<ul>
<li>Entity Definition 验证 (internal/validation/entity.go)</li>
<li>Manifest JSON Schema 验证</li>
<li>Function Parameter 验证</li>
</ul>
<p>✅ <strong>组件管理</strong></p>
<ul>
<li>ComponentManager - 组件安装/卸载/启用/禁用</li>
<li>Component Registry - 组件注册表管理</li>
<li>Dependency Resolution - 依赖关系检查</li>
</ul>
<h3 id="_9-2-进行中的功能" tabindex="-1"><a class="header-anchor" href="#_9-2-进行中的功能"><span>9.2 进行中的功能</span></a></h3>
<p>🔄 <strong>Provider Manifest 系统</strong></p>
<ul>
<li>Server 端接收和合并 provider manifest</li>
<li>统一 descriptors 暴露 API</li>
<li>多语言 SDK (Python/Node 等) 支持</li>
</ul>
<p>🔄 <strong>Proto-First 生成</strong></p>
<ul>
<li>扩展 <code v-pre>tools/protoc-gen-croupier</code> 支持 manifest 生成</li>
<li>从 .proto 文件生成 manifest.json 和 schema</li>
</ul>
<h3 id="_9-3-待实现的功能" tabindex="-1"><a class="header-anchor" href="#_9-3-待实现的功能"><span>9.3 待实现的功能</span></a></h3>
<p>⏳ <strong>Entity 管理界面</strong></p>
<ul>
<li>可视化 entity 创建和编辑</li>
<li>JSON Schema 编辑器</li>
<li>UI 配置工具</li>
<li>预览功能</li>
</ul>
<p>⏳ <strong>进阶特性</strong></p>
<ul>
<li>Composite Entity（组合实体）</li>
<li>Entity Relationship（实体关系）</li>
<li>Workflow Orchestration（工作流编排）</li>
<li>Dynamic Entity 生成</li>
</ul>
<p>⏳ <strong>多租户支持</strong></p>
<ul>
<li>租户级别的 entity 隔离</li>
<li>数据隔离</li>
<li>权限隔离</li>
</ul>
<hr>
<h2 id="🔟-架构模式与最佳实践" tabindex="-1"><a class="header-anchor" href="#🔟-架构模式与最佳实践"><span>🔟 架构模式与最佳实践</span></a></h2>
<h3 id="_10-1-分层模式" tabindex="-1"><a class="header-anchor" href="#_10-1-分层模式"><span>10.1 分层模式</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Presentation Layer (UI)</span>
<span class="line">├── ProTable Component    ← 基于Resource渲染</span>
<span class="line">├── ProForm Component     ← 基于Entity+Function渲染</span>
<span class="line">└── UI Schema</span>
<span class="line"></span>
<span class="line">Domain Layer</span>
<span class="line">├── Entity Definition     ← 业务对象的完整描述</span>
<span class="line">├── Operation Definition  ← 对象支持的操作</span>
<span class="line">└── Relationship          ← 对象间的关系</span>
<span class="line"></span>
<span class="line">Service Layer</span>
<span class="line">├── Function Invocation   ← 执行具体操作</span>
<span class="line">├── Parameter Validation  ← 基于Schema验证</span>
<span class="line">└── Auth &amp; Permission     ← 权限检查</span>
<span class="line"></span>
<span class="line">Data Layer</span>
<span class="line">├── Repository Pattern    ← 数据访问</span>
<span class="line">├── Transaction           ← 事务支持（跨entity操作）</span>
<span class="line">└── Cache                 ← 缓存策略</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_10-2-函数组合的两种模式" tabindex="-1"><a class="header-anchor" href="#_10-2-函数组合的两种模式"><span>10.2 函数组合的两种模式</span></a></h3>
<h4 id="模式1-entity-operation-映射" tabindex="-1"><a class="header-anchor" href="#模式1-entity-operation-映射"><span>模式1：Entity Operation 映射</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line">Entity.operations.create → <span class="token punctuation">[</span>Function1<span class="token punctuation">,</span> Function2<span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><strong>用途</strong>：同一操作可以由多个函数串联完成
<strong>例子</strong>：用户注册可能涉及验证→创建→发送邮件</p>
<h4 id="模式2-resource-operation-映射" tabindex="-1"><a class="header-anchor" href="#模式2-resource-operation-映射"><span>模式2：Resource Operation 映射</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line">Resource.operations.create → <span class="token punctuation">{</span></span>
<span class="line">  function<span class="token operator">:</span> <span class="token string">"entity.create"</span><span class="token punctuation">,</span></span>
<span class="line">  ui<span class="token operator">:</span> <span class="token punctuation">{</span> ... <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>用途</strong>：Resource 在 Entity 基础上添加 UI 定义和语义
<strong>例子</strong>：ProTable 列定义、操作按钮、批量操作</p>
<h3 id="_10-3-跨实体操作的设计" tabindex="-1"><a class="header-anchor" href="#_10-3-跨实体操作的设计"><span>10.3 跨实体操作的设计</span></a></h3>
<p>对于涉及多个 Entity 的操作（如 wallet.transfer），设计方案：</p>
<ol>
<li>
<p><strong>操作定义在主要 Entity</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"from_player_id"</span><span class="token operator">:</span> <span class="token string">"..."</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"to_player_id"</span><span class="token operator">:</span> <span class="token string">"..."</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"currency_code"</span><span class="token operator">:</span> <span class="token string">"..."</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"amount"</span><span class="token operator">:</span> <span class="token string">"..."</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p><strong>权限和语义配置</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"risk"</span><span class="token operator">:</span> <span class="token string">"high"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"two_person_rule"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"atomic"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"idempotent"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p><strong>关系定义</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"relationships"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"currency"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"many-to-one"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token string">"currency"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"foreign_key"</span><span class="token operator">:</span> <span class="token string">"currency_id"</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"player"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"many-to-one"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"foreign_key"</span><span class="token operator">:</span> <span class="token string">"player_id"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
</ol>
<hr>
<h2 id="_1️⃣1️⃣-文件路径总结" tabindex="-1"><a class="header-anchor" href="#_1️⃣1️⃣-文件路径总结"><span>1️⃣1️⃣ 文件路径总结</span></a></h2>
<h3 id="核心文档" tabindex="-1"><a class="header-anchor" href="#核心文档"><span>核心文档</span></a></h3>
<ul>
<li>📄 <code v-pre>/Users/cui/Workspaces/croupier/ARCHITECTURE.md</code> - 完整的对象驱动系统架构文档</li>
<li>📄 <code v-pre>/Users/cui/Workspaces/croupier/docs/providers-manifest.md</code> - Provider Manifest 设计说明</li>
<li>📄 <code v-pre>/Users/cui/Workspaces/croupier/docs/providers-manifest.schema.json</code> - Manifest JSON Schema</li>
</ul>
<h3 id="实现代码" tabindex="-1"><a class="header-anchor" href="#实现代码"><span>实现代码</span></a></h3>
<ul>
<li>📝 <code v-pre>/Users/cui/Workspaces/croupier/internal/validation/entity.go</code> - Entity 验证</li>
<li>📝 <code v-pre>/Users/cui/Workspaces/croupier/internal/function/descriptor/loader.go</code> - Descriptor 加载</li>
<li>📝 <code v-pre>/Users/cui/Workspaces/croupier/internal/pack/manager.go</code> - Component 管理</li>
<li>📝 <code v-pre>/Users/cui/Workspaces/croupier/internal/app/server/http/server.go</code> (第4060-4400行) - Entity API 实现</li>
</ul>
<h3 id="示例定义" tabindex="-1"><a class="header-anchor" href="#示例定义"><span>示例定义</span></a></h3>
<ul>
<li>
<p><code v-pre>components/player-management/descriptors/</code></p>
<ul>
<li>📋 <code v-pre>player.entity.json</code></li>
<li>📋 <code v-pre>player.resource.json</code></li>
<li>📋 <code v-pre>player.register.json</code> 等</li>
</ul>
</li>
<li>
<p><code v-pre>components/item-management/descriptors/</code></p>
<ul>
<li>📋 <code v-pre>item.entity.json</code></li>
<li>📋 <code v-pre>item.resource.json</code></li>
</ul>
</li>
<li>
<p><code v-pre>components/economy-system/descriptors/</code></p>
<ul>
<li>📋 <code v-pre>currency.entity.json</code></li>
<li>📋 <code v-pre>wallet.entity.json</code></li>
<li>📋 <code v-pre>wallet.transfer.json</code> (跨实体操作)</li>
</ul>
</li>
<li>
<p><code v-pre>components/entity-management/descriptors/</code></p>
<ul>
<li>📋 <code v-pre>entity.resource.json</code></li>
<li>📋 <code v-pre>entity.create.json</code></li>
</ul>
</li>
</ul>
<hr>
<h2 id="_1️⃣2️⃣-总结与建议" tabindex="-1"><a class="header-anchor" href="#_1️⃣2️⃣-总结与建议"><span>1️⃣2️⃣ 总结与建议</span></a></h2>
<h3 id="核心要点" tabindex="-1"><a class="header-anchor" href="#核心要点"><span>核心要点</span></a></h3>
<ol>
<li>
<p><strong>虚拟对象 = Entity</strong>：业务对象的完整定义，包括数据结构、操作和UI配置</p>
</li>
<li>
<p><strong>四层架构</strong>：</p>
<ul>
<li>Function Layer：具体操作实现</li>
<li>Entity Layer：对象定义和操作映射</li>
<li>Resource Layer：UI操作编排</li>
<li>Component Layer：模块打包</li>
</ul>
</li>
<li>
<p><strong>函数组合机制</strong>：</p>
<ul>
<li>Entity.operations 映射函数ID</li>
<li>Resource 在 Entity 基础上添加 UI 定义</li>
<li>支持多函数组合和跨实体操作</li>
</ul>
</li>
<li>
<p><strong>Provider Manifest 标准</strong>：统一的、语言无关的能力声明标准，支持多语言 SDK</p>
</li>
</ol>
<h3 id="建议的下一步" tabindex="-1"><a class="header-anchor" href="#建议的下一步"><span>建议的下一步</span></a></h3>
<ol>
<li>
<p><strong>完善 Entity 管理界面</strong></p>
<ul>
<li>实现可视化的 entity 创建/编辑</li>
<li>JSON Schema 编辑器集成</li>
<li>UI 预览功能</li>
</ul>
</li>
<li>
<p><strong>实现 Proto-First 生成</strong></p>
<ul>
<li>扩展 protoc-gen-croupier 支持 manifest 生成</li>
<li>支持自定义注解声明权限、语义等</li>
</ul>
</li>
<li>
<p><strong>多语言 SDK 支持</strong></p>
<ul>
<li>Python/Node SDK 实现</li>
<li>Out-of-proc provider 模式</li>
</ul>
</li>
<li>
<p><strong>高级特性</strong></p>
<ul>
<li>Entity Composition（组合实体）</li>
<li>Workflow Orchestration（工作流）</li>
<li>Dynamic Entity 生成</li>
</ul>
</li>
</ol>
</div></template>


