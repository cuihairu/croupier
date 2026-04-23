<template><div><h1 id="croupier-虚拟对象-virtual-object-快速参考指南" tabindex="-1"><a class="header-anchor" href="#croupier-虚拟对象-virtual-object-快速参考指南"><span>Croupier 虚拟对象(Virtual Object) - 快速参考指南</span></a></h1>
<h2 id="🎯-核心概念速览" tabindex="-1"><a class="header-anchor" href="#🎯-核心概念速览"><span>🎯 核心概念速览</span></a></h2>
<h3 id="四层架构" tabindex="-1"><a class="header-anchor" href="#四层架构"><span>四层架构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Function (函数)</span>
<span class="line">    ↓ 绑定到</span>
<span class="line">Entity (虚拟对象)</span>
<span class="line">    ↓ 组织成</span>
<span class="line">Resource (资源/UI)</span>
<span class="line">    ↓ 打包为</span>
<span class="line">Component (组件)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><table>
<thead>
<tr>
<th>层级</th>
<th>文件</th>
<th>定义</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Function</strong></td>
<td><code v-pre>*.json</code></td>
<td>单个操作实现</td>
</tr>
<tr>
<td><strong>Entity</strong></td>
<td><code v-pre>*.entity.json</code></td>
<td>业务对象定义 + 操作映射</td>
</tr>
<tr>
<td><strong>Resource</strong></td>
<td><code v-pre>*.resource.json</code></td>
<td>UI展现层 + 函数组合</td>
</tr>
<tr>
<td><strong>Component</strong></td>
<td><code v-pre>manifest.json</code></td>
<td>模块打包单位</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="📝-关键文件清单" tabindex="-1"><a class="header-anchor" href="#📝-关键文件清单"><span>📝 关键文件清单</span></a></h2>
<h3 id="核心文档-必读" tabindex="-1"><a class="header-anchor" href="#核心文档-必读"><span>核心文档 (必读)</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">📄 ARCHITECTURE.md                          # 完整架构文档</span>
<span class="line">📄 docs/providers-manifest.md               # Provider标准说明</span>
<span class="line">📄 docs/providers-manifest.schema.json      # Manifest验证规范</span>
<span class="line">📄 docs/VIRTUAL_OBJECT_DESIGN.md            # 本项目的详细分析</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="实现代码" tabindex="-1"><a class="header-anchor" href="#实现代码"><span>实现代码</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">📝 internal/validation/entity.go            # Entity验证逻辑</span>
<span class="line">📝 internal/function/descriptor/loader.go   # Descriptor加载</span>
<span class="line">📝 internal/pack/manager.go                 # 组件管理器</span>
<span class="line">📝 internal/app/server/http/server.go       # HTTP API实现(L4060+)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="实例定义" tabindex="-1"><a class="header-anchor" href="#实例定义"><span>实例定义</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">components/</span>
<span class="line">├── player-management/</span>
<span class="line">│   ├── manifest.json</span>
<span class="line">│   └── descriptors/</span>
<span class="line">│       ├── player.entity.json              # Entity示例</span>
<span class="line">│       ├── player.resource.json            # Resource示例</span>
<span class="line">│       └── player.register.json            # Function示例</span>
<span class="line">├── item-management/                        # 物品管理</span>
<span class="line">├── economy-system/                         # 经济系统(跨实体)</span>
<span class="line">└── entity-management/                      # 虚拟对象管理系统本身</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="🏗️-虚拟对象设计模板" tabindex="-1"><a class="header-anchor" href="#🏗️-虚拟对象设计模板"><span>🏗️ 虚拟对象设计模板</span></a></h2>
<h3 id="_1️⃣-entity-definition-entity-json" tabindex="-1"><a class="header-anchor" href="#_1️⃣-entity-definition-entity-json"><span>1️⃣ Entity Definition (<code v-pre>*.entity.json</code>)</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player Entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span> <span class="token property">"primary_key"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"username"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span> <span class="token property">"unique"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span> <span class="token property">"searchable"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"status"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span> <span class="token property">"enum"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"active"</span><span class="token punctuation">,</span> <span class="token string">"banned"</span><span class="token punctuation">]</span><span class="token punctuation">,</span> <span class="token property">"filterable"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"required"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">,</span> <span class="token string">"username"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"create"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.register"</span><span class="token punctuation">]</span><span class="token punctuation">,</span>           <span class="token comment">// 可以是单函数或函数数组</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.profile.get"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"update"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.profile.update"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"delete"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.ban"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"list"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.list"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"custom"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.unban"</span><span class="token punctuation">]</span>               <span class="token comment">// 自定义操作</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"display_field"</span><span class="token operator">:</span> <span class="token string">"username"</span><span class="token punctuation">,</span>             <span class="token comment">// 显示字段</span></span>
<span class="line">    <span class="token property">"title_template"</span><span class="token operator">:</span> <span class="token string">"{username} ({id})"</span><span class="token punctuation">,</span>   <span class="token comment">// 标题模板</span></span>
<span class="line">    <span class="token property">"avatar_field"</span><span class="token operator">:</span> <span class="token string">"avatar_url"</span><span class="token punctuation">,</span>            <span class="token comment">// 头像字段</span></span>
<span class="line">    <span class="token property">"status_field"</span><span class="token operator">:</span> <span class="token string">"status"</span>                 <span class="token comment">// 状态字段</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>关键点</strong>：</p>
<ul>
<li><code v-pre>operations</code> 映射标准CRUD和自定义操作到函数</li>
<li>每个操作可绑定单个或多个函数</li>
<li><code v-pre>ui</code> 定义显示配置</li>
</ul>
<h3 id="_2️⃣-function-definition-json" tabindex="-1"><a class="header-anchor" href="#_2️⃣-function-definition-json"><span>2️⃣ Function Definition (<code v-pre>*.json</code>)</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"operation"</span><span class="token operator">:</span> <span class="token string">"create"</span>                    <span class="token comment">// 关联到Entity的操作</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"username"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span> <span class="token property">"pattern"</span><span class="token operator">:</span> <span class="token string">"^[a-zA-Z0-9_]{3,16}$"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"email"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span> <span class="token property">"format"</span><span class="token operator">:</span> <span class="token string">"email"</span><span class="token punctuation">}</span></span>
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
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>关键点</strong>：</p>
<ul>
<li><code v-pre>entity</code> 字段指定关联的对象和操作</li>
<li><code v-pre>params</code> 和 <code v-pre>result</code> 定义输入输出Schema</li>
<li><code v-pre>auth</code> 定义权限和条件</li>
<li><code v-pre>semantics</code> 定义限流、并发等</li>
</ul>
<h3 id="_3️⃣-resource-definition-resource-json" tabindex="-1"><a class="header-anchor" href="#_3️⃣-resource-definition-resource-json"><span>3️⃣ Resource Definition (<code v-pre>*.resource.json</code>)</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"resource"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
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
<span class="line">      <span class="token property">"label"</span><span class="token operator">:</span> <span class="token string">"玩家列表"</span></span>
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
<span class="line">        <span class="token property">"searchable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"dataIndex"</span><span class="token operator">:</span> <span class="token string">"status"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"状态"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"filterable"</span><span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
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
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>关键点</strong>：</p>
<ul>
<li><code v-pre>operations</code> 中每个操作指定具体函数和UI标签</li>
<li><code v-pre>ui</code> 定义ProTable的列定义和行为</li>
<li><code v-pre>actions</code> 定义工具栏和行操作按钮</li>
<li><code v-pre>features</code> 定义表格功能</li>
</ul>
<h3 id="_4️⃣-component-manifest-manifest-json" tabindex="-1"><a class="header-anchor" href="#_4️⃣-component-manifest-manifest-json"><span>4️⃣ Component Manifest (<code v-pre>manifest.json</code>)</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player-management"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player Management System"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"dependencies"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span> <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"Player"</span><span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"Register a new player"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="🔄-函数组合模式" tabindex="-1"><a class="header-anchor" href="#🔄-函数组合模式"><span>🔄 函数组合模式</span></a></h2>
<h3 id="模式1-单操作单函数" tabindex="-1"><a class="header-anchor" href="#模式1-单操作单函数"><span>模式1: 单操作单函数</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"create"</span><span class="token operator">:</span> <span class="token string">"player.register"</span>               <span class="token comment">// 直接指定函数ID</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="模式2-单操作多函数" tabindex="-1"><a class="header-anchor" href="#模式2-单操作多函数"><span>模式2: 单操作多函数</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"create"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player.validate"</span><span class="token punctuation">,</span> <span class="token string">"player.register"</span><span class="token punctuation">,</span> <span class="token string">"player.notify"</span><span class="token punctuation">]</span>  <span class="token comment">// 按顺序执行</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="模式3-跨实体操作" tabindex="-1"><a class="header-anchor" href="#模式3-跨实体操作"><span>模式3: 跨实体操作</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"from_player_id"</span><span class="token operator">:</span> <span class="token string">"..."</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"to_player_id"</span><span class="token operator">:</span> <span class="token string">"..."</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"currency_code"</span><span class="token operator">:</span> <span class="token string">"..."</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"amount"</span><span class="token operator">:</span> <span class="token string">"..."</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
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
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="🔌-http-api-端点" tabindex="-1"><a class="header-anchor" href="#🔌-http-api-端点"><span>🔌 HTTP API 端点</span></a></h2>
<h3 id="entity-管理" tabindex="-1"><a class="header-anchor" href="#entity-管理"><span>Entity 管理</span></a></h3>
<table>
<thead>
<tr>
<th>端点</th>
<th>方法</th>
<th>权限</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>/api/entities</code></td>
<td>GET</td>
<td><code v-pre>entities:read</code></td>
<td>列出所有entity</td>
</tr>
<tr>
<td><code v-pre>/api/entities</code></td>
<td>POST</td>
<td><code v-pre>entities:create</code></td>
<td>创建新entity</td>
</tr>
<tr>
<td><code v-pre>/api/entities/:id</code></td>
<td>GET</td>
<td><code v-pre>entities:read</code></td>
<td>获取entity详情</td>
</tr>
<tr>
<td><code v-pre>/api/entities/:id</code></td>
<td>PUT</td>
<td><code v-pre>entities:update</code></td>
<td>更新entity</td>
</tr>
<tr>
<td><code v-pre>/api/entities/:id</code></td>
<td>DELETE</td>
<td><code v-pre>entities:delete</code></td>
<td>删除entity</td>
</tr>
</tbody>
</table>
<h3 id="descriptor-与-provider" tabindex="-1"><a class="header-anchor" href="#descriptor-与-provider"><span>Descriptor 与 Provider</span></a></h3>
<table>
<thead>
<tr>
<th>端点</th>
<th>方法</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>/api/descriptors</code></td>
<td>GET</td>
<td>获取所有descriptor</td>
</tr>
<tr>
<td><code v-pre>/api/descriptors?detailed=true</code></td>
<td>GET</td>
<td>获取详细descriptor + provider manifest</td>
</tr>
<tr>
<td><code v-pre>/api/providers/capabilities</code></td>
<td>POST</td>
<td>注册provider能力</td>
</tr>
<tr>
<td><code v-pre>/api/providers/descriptors</code></td>
<td>GET</td>
<td>获取所有provider的descriptors</td>
</tr>
<tr>
<td><code v-pre>/api/providers/entities</code></td>
<td>GET</td>
<td>聚合所有provider的entities</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="✅-设计检查清单" tabindex="-1"><a class="header-anchor" href="#✅-设计检查清单"><span>✅ 设计检查清单</span></a></h2>
<p>创建虚拟对象时，确保：</p>
<ul>
<li>
<p>[ ] <strong>Entity定义</strong></p>
<ul>
<li>[ ] ID遵循 <code v-pre>entity.name</code> 命名</li>
<li>[ ] type为 <code v-pre>entity</code></li>
<li>[ ] schema是有效的JSON Schema</li>
<li>[ ] operations映射了必要的CRUD操作</li>
<li>[ ] ui配置了display_field和title_template</li>
</ul>
</li>
<li>
<p>[ ] <strong>Function定义</strong></p>
<ul>
<li>[ ] ID遵循 <code v-pre>entity.operation</code> 命名</li>
<li>[ ] 有entity字段指定关联对象</li>
<li>[ ] params是有效的JSON Schema</li>
<li>[ ] result定义了输出格式</li>
<li>[ ] auth声明了权限要求</li>
<li>[ ] semantics定义了限流和超时</li>
</ul>
</li>
<li>
<p>[ ] <strong>Resource定义</strong></p>
<ul>
<li>[ ] operations的function字段指向存在的函数</li>
<li>[ ] ui.columns定义了所有必要的列</li>
<li>[ ] actions定义了toolbar和row操作</li>
<li>[ ] auth权限与entity操作一致</li>
</ul>
</li>
<li>
<p>[ ] <strong>Component清单</strong></p>
<ul>
<li>[ ] 所有function都在manifest中声明</li>
<li>[ ] dependencies指定了依赖的其他component</li>
<li>[ ] entities声明了所有定义的entity</li>
</ul>
</li>
</ul>
<hr>
<h2 id="🚀-常见任务" tabindex="-1"><a class="header-anchor" href="#🚀-常见任务"><span>🚀 常见任务</span></a></h2>
<h3 id="创建新的虚拟对象" tabindex="-1"><a class="header-anchor" href="#创建新的虚拟对象"><span>创建新的虚拟对象</span></a></h3>
<ol>
<li>
<p>在 <code v-pre>components/{component}/descriptors/</code> 中创建文件：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">entity.entity.json     - Entity定义</span>
<span class="line">entity.resource.json   - Resource定义</span>
<span class="line">entity.create.json     - 创建函数</span>
<span class="line">entity.get.json        - 读取函数</span>
<span class="line">entity.update.json     - 更新函数</span>
<span class="line">entity.delete.json     - 删除函数</span>
<span class="line">entity.list.json       - 列表函数</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p>在 <code v-pre>components/{component}/manifest.json</code> 中声明函数</p>
</li>
<li>
<p>POST to <code v-pre>/api/entities</code> 创建entity（或直接编辑JSON）</p>
</li>
</ol>
<h3 id="添加自定义操作" tabindex="-1"><a class="header-anchor" href="#添加自定义操作"><span>添加自定义操作</span></a></h3>
<ol>
<li>创建函数定义 <code v-pre>entity.custom_op.json</code></li>
<li>在entity的operations中添加：<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token property">"custom_op"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"entity.custom_op"</span><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div></li>
<li>如需UI，在resource中添加到actions</li>
</ol>
<h3 id="实现跨实体操作" tabindex="-1"><a class="header-anchor" href="#实现跨实体操作"><span>实现跨实体操作</span></a></h3>
<ol>
<li>创建函数，params中包含多个entity的ID</li>
<li>在auth中标记 <code v-pre>&quot;risk&quot;: &quot;high&quot;</code> 和 <code v-pre>&quot;two_person_rule&quot;: true</code></li>
<li>在semantics中标记 <code v-pre>&quot;atomic&quot;: true</code></li>
<li>在function params中定义完整的关系和验证规则</li>
</ol>
<hr>
<h2 id="📊-现有实现参考" tabindex="-1"><a class="header-anchor" href="#📊-现有实现参考"><span>📊 现有实现参考</span></a></h2>
<h3 id="player-management-完整示例" tabindex="-1"><a class="header-anchor" href="#player-management-完整示例"><span>Player Management (完整示例)</span></a></h3>
<ul>
<li>路径: <code v-pre>components/player-management/</code></li>
<li>包含: player.entity + player.resource + 所有CRUD函数</li>
<li>特点: 标准CRUD + 自定义操作(unban)</li>
</ul>
<h3 id="economy-system-跨实体示例" tabindex="-1"><a class="header-anchor" href="#economy-system-跨实体示例"><span>Economy System (跨实体示例)</span></a></h3>
<ul>
<li>路径: <code v-pre>components/economy-system/</code></li>
<li>包含: currency + wallet + wallet.transfer(跨实体)</li>
<li>特点: 关系定义 + 原子操作</li>
</ul>
<h3 id="entity-management-系统本身" tabindex="-1"><a class="header-anchor" href="#entity-management-系统本身"><span>Entity Management (系统本身)</span></a></h3>
<ul>
<li>路径: <code v-pre>components/entity-management/</code></li>
<li>特点: entity.resource用于管理entity定义本身</li>
</ul>
<hr>
<h2 id="🔍-调试技巧" tabindex="-1"><a class="header-anchor" href="#🔍-调试技巧"><span>🔍 调试技巧</span></a></h2>
<h3 id="验证entity定义" tabindex="-1"><a class="header-anchor" href="#验证entity定义"><span>验证Entity定义</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Entity验证</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/entities/:id/validate</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 获取所有entity</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/api/entities</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查descriptor</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/api/descriptors?id<span class="token operator">=</span>player.entity</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="查看实现源码" tabindex="-1"><a class="header-anchor" href="#查看实现源码"><span>查看实现源码</span></a></h3>
<ul>
<li>Entity验证: <code v-pre>internal/validation/entity.go</code></li>
<li>HTTP处理: <code v-pre>internal/app/server/http/server.go:4060</code></li>
<li>Descriptor加载: <code v-pre>internal/function/descriptor/loader.go</code></li>
</ul>
<hr>
<h2 id="📚-相关文档" tabindex="-1"><a class="header-anchor" href="#📚-相关文档"><span>📚 相关文档</span></a></h2>
<ul>
<li><strong>完整分析</strong>: <code v-pre>docs/VIRTUAL_OBJECT_DESIGN.md</code></li>
<li><strong>Manifest标准</strong>: <code v-pre>docs/providers-manifest.md</code></li>
<li><strong>架构文档</strong>: <code v-pre>ARCHITECTURE.md</code></li>
<li><strong>TODO任务</strong>: <code v-pre>todo.md</code> (函数管理与 Provider Manifest 部分)</li>
</ul>
</div></template>


