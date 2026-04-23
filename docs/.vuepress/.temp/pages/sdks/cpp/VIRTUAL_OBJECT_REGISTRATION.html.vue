<template><div><h1 id="croupier-c-sdk-虚拟对象注册机制" tabindex="-1"><a class="header-anchor" href="#croupier-c-sdk-虚拟对象注册机制"><span>Croupier C++ SDK：虚拟对象注册机制</span></a></h1>
<h2 id="🎯-概述" tabindex="-1"><a class="header-anchor" href="#🎯-概述"><span>🎯 概述</span></a></h2>
<p>Croupier采用创新的<strong>四层组件化架构</strong>实现虚拟对象管理，通过<strong>ID引用模式</strong>优雅地解决了对象参数传递的性能问题。本文档详细介绍了虚拟对象的注册机制和C++ SDK的扩展方案。</p>
<h2 id="📋-核心架构" tabindex="-1"><a class="header-anchor" href="#📋-核心架构"><span>📋 核心架构</span></a></h2>
<h3 id="四层抽象模型" tabindex="-1"><a class="header-anchor" href="#四层抽象模型"><span>四层抽象模型</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Function Level    ← 单个原子操作 (wallet.transfer)</span>
<span class="line">     ↓</span>
<span class="line">Entity Level      ← 业务对象模型 (wallet.entity)</span>
<span class="line">     ↓</span>
<span class="line">Resource Level    ← UI资源组织 (钱包管理面板)</span>
<span class="line">     ↓</span>
<span class="line">Component Level   ← 可分发模块 (economy-system)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="设计理念" tabindex="-1"><a class="header-anchor" href="#设计理念"><span>设计理念</span></a></h3>
<h4 id="✅-id引用模式-解决性能问题" tabindex="-1"><a class="header-anchor" href="#✅-id引用模式-解决性能问题"><span>✅ <strong>ID引用模式</strong> - 解决性能问题</span></a></h4>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// ❌ 避免笨重的对象参数传递</span></span>
<span class="line"><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span> <span class="token punctuation">{</span>object<span class="token operator">:</span> wallet_instance<span class="token punctuation">,</span> params<span class="token operator">:</span> <span class="token punctuation">{</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">}</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ✅ 优雅的ID引用设计</span></span>
<span class="line"><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span> <span class="token punctuation">{</span></span>
<span class="line">  from_player_id<span class="token operator">:</span> <span class="token string">"player123"</span><span class="token punctuation">,</span>  <span class="token comment">// 直接使用ID引用</span></span>
<span class="line">  to_player_id<span class="token operator">:</span> <span class="token string">"player456"</span><span class="token punctuation">,</span></span>
<span class="line">  currency_code<span class="token operator">:</span> <span class="token string">"gold"</span><span class="token punctuation">,</span></span>
<span class="line">  amount<span class="token operator">:</span> <span class="token string">"100.0"</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="✅-声明式配置-配置驱动开发" tabindex="-1"><a class="header-anchor" href="#✅-声明式配置-配置驱动开发"><span>✅ <strong>声明式配置</strong> - 配置驱动开发</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token comment">// wallet.entity.json</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"wallet.entity"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* JSON Schema定义对象结构 */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"read"</span><span class="token operator">:</span> <span class="token string">"wallet.get"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"transfer"</span><span class="token operator">:</span> <span class="token string">"wallet.transfer"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"relationships"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"currency"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"many-to-one"</span><span class="token punctuation">,</span> <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token string">"currency"</span><span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="✅-无状态函数-易于扩展" tabindex="-1"><a class="header-anchor" href="#✅-无状态函数-易于扩展"><span>✅ <strong>无状态函数</strong> - 易于扩展</span></a></h4>
<ul>
<li>每个函数是纯函数，通过ID查找对象</li>
<li>支持水平扩展，无状态共享问题</li>
<li>Repository模式管理对象生命周期</li>
</ul>
<h2 id="🏗️-c-sdk扩展方案" tabindex="-1"><a class="header-anchor" href="#🏗️-c-sdk扩展方案"><span>🏗️ C++ SDK扩展方案</span></a></h2>
<h3 id="核心数据结构" tabindex="-1"><a class="header-anchor" href="#核心数据结构"><span>核心数据结构</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk <span class="token punctuation">{</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 虚拟对象描述符</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">VirtualObjectDescriptor</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string id<span class="token punctuation">;</span>                              <span class="token comment">// e.g. "wallet.entity"</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string version<span class="token punctuation">;</span>                         <span class="token comment">// 版本号</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string name<span class="token punctuation">;</span>                            <span class="token comment">// 显示名称</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string description<span class="token punctuation">;</span>                     <span class="token comment">// 描述信息</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> schema<span class="token punctuation">;</span>   <span class="token comment">// JSON Schema定义</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> operations<span class="token punctuation">;</span> <span class="token comment">// 操作映射</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> RelationshipDef<span class="token operator">></span> relationships<span class="token punctuation">;</span> <span class="token comment">// 关系定义</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 关系定义</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">RelationshipDef</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string type<span class="token punctuation">;</span>        <span class="token comment">// "one-to-many", "many-to-one", "many-to-many"</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string entity<span class="token punctuation">;</span>      <span class="token comment">// 关联实体ID</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string foreign_key<span class="token punctuation">;</span> <span class="token comment">// 外键字段名</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 组件描述符（完整模块）</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">ComponentDescriptor</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string id<span class="token punctuation">;</span>                             <span class="token comment">// e.g. "economy-system"</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string version<span class="token punctuation">;</span>                        <span class="token comment">// 组件版本</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string name<span class="token punctuation">;</span>                           <span class="token comment">// 组件名称</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>VirtualObjectDescriptor<span class="token operator">></span> entities<span class="token punctuation">;</span>  <span class="token comment">// 包含的实体</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>FunctionDescriptor<span class="token operator">></span> functions<span class="token punctuation">;</span>      <span class="token comment">// 包含的函数</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> resources<span class="token punctuation">;</span>   <span class="token comment">// UI资源定义</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> config<span class="token punctuation">;</span>      <span class="token comment">// 组件配置</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">}</span> <span class="token comment">// namespace croupier::sdk</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="扩展的croupierclient接口" tabindex="-1"><a class="header-anchor" href="#扩展的croupierclient接口"><span>扩展的CroupierClient接口</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">class</span> <span class="token class-name">CroupierClient</span> <span class="token punctuation">{</span></span>
<span class="line"><span class="token keyword">public</span><span class="token operator">:</span></span>
<span class="line">    <span class="token comment">// ========== 现有接口（保持兼容） ==========</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">RegisterFunction</span><span class="token punctuation">(</span><span class="token keyword">const</span> FunctionDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">,</span> FunctionHandler handler<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">Close</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 新增：虚拟对象注册 ==========</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册单个虚拟对象及其关联函数</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span></span>
<span class="line">        <span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">,</span></span>
<span class="line">        <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span><span class="token operator">&amp;</span> handlers</span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 批量注册组件（推荐方式）</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">RegisterComponent</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span> comp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 从JSON配置文件加载并注册组件</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> config_file<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 新增：管理接口 ==========</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取已注册的虚拟对象列表</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>VirtualObjectDescriptor<span class="token operator">></span> <span class="token function">GetRegisteredObjects</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">const</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取已注册的组件列表</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>ComponentDescriptor<span class="token operator">></span> <span class="token function">GetRegisteredComponents</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">const</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 取消注册虚拟对象</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">UnregisterVirtualObject</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> object_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 取消注册组件</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">UnregisterComponent</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> component_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="工具函数" tabindex="-1"><a class="header-anchor" href="#工具函数"><span>工具函数</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token double-colon punctuation">::</span>utils <span class="token punctuation">{</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 从JSON文件加载虚拟对象描述符</span></span>
<span class="line">VirtualObjectDescriptor <span class="token function">LoadObjectDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> file_path<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 从JSON文件加载组件描述符</span></span>
<span class="line">ComponentDescriptor <span class="token function">LoadComponentDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> file_path<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 验证虚拟对象定义的完整性</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">ValidateObjectDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 验证组件定义的完整性</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">ValidateComponentDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span> comp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 生成默认的对象配置模板</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">GenerateObjectTemplate</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> object_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 生成默认的组件配置模板</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">GenerateComponentTemplate</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> component_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">}</span> <span class="token comment">// namespace croupier::sdk::utils</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="💡-使用示例" tabindex="-1"><a class="header-anchor" href="#💡-使用示例"><span>💡 使用示例</span></a></h2>
<h3 id="示例1-单个函数注册-现有方式" tabindex="-1"><a class="header-anchor" href="#示例1-单个函数注册-现有方式"><span>示例1：单个函数注册（现有方式）</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 函数处理器实现</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">WalletTransferHandler</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> context<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span> data <span class="token operator">=</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ParseJSON</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 通过ID获取源钱包和目标钱包</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string from_player <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"from_player_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string to_player <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"to_player_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string amount <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"amount"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 执行转账业务逻辑</span></span>
<span class="line">    TransferResult result <span class="token operator">=</span> <span class="token class-name">WalletService</span><span class="token double-colon punctuation">::</span><span class="token function">Transfer</span><span class="token punctuation">(</span>from_player<span class="token punctuation">,</span> to_player<span class="token punctuation">,</span> amount<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 返回结果</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> response<span class="token punctuation">;</span></span>
<span class="line">    response<span class="token punctuation">[</span><span class="token string">"transfer_id"</span><span class="token punctuation">]</span> <span class="token operator">=</span> result<span class="token punctuation">.</span>transfer_id<span class="token punctuation">;</span></span>
<span class="line">    response<span class="token punctuation">[</span><span class="token string">"status"</span><span class="token punctuation">]</span> <span class="token operator">=</span> result<span class="token punctuation">.</span>status<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ToJSON</span><span class="token punctuation">(</span>response<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>service_id <span class="token operator">=</span> <span class="token string">"wallet-service"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册单个函数</span></span>
<span class="line">    FunctionDescriptor desc<span class="token punctuation">;</span></span>
<span class="line">    desc<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">;</span></span>
<span class="line">    desc<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>desc<span class="token punctuation">,</span> WalletTransferHandler<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="示例2-虚拟对象注册-推荐方式" tabindex="-1"><a class="header-anchor" href="#示例2-虚拟对象注册-推荐方式"><span>示例2：虚拟对象注册（推荐方式）</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>service_id <span class="token operator">=</span> <span class="token string">"economy-service"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 定义钱包实体</span></span>
<span class="line">    VirtualObjectDescriptor wallet_desc<span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"wallet.entity"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>name <span class="token operator">=</span> <span class="token string">"钱包实体"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>description <span class="token operator">=</span> <span class="token string">"玩家钱包管理实体"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 定义Schema</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>schema<span class="token punctuation">[</span><span class="token string">"type"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"object"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>schema<span class="token punctuation">[</span><span class="token string">"properties"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token raw-string string">R"({</span>
<span class="line">        "wallet_id": {"type": "string"},</span>
<span class="line">        "player_id": {"type": "string"},</span>
<span class="line">        "currency_id": {"type": "string"},</span>
<span class="line">        "balance": {"type": "string", "pattern": "^[0-9]+\\.?[0-9]*$"}</span>
<span class="line">    })"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 定义操作映射</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.get"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"deposit"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.deposit"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"withdraw"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.withdraw"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 定义关系</span></span>
<span class="line">    RelationshipDef currency_rel<span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>type <span class="token operator">=</span> <span class="token string">"many-to-one"</span><span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>entity <span class="token operator">=</span> <span class="token string">"currency"</span><span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>foreign_key <span class="token operator">=</span> <span class="token string">"currency_id"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_desc<span class="token punctuation">.</span>relationships<span class="token punctuation">[</span><span class="token string">"currency"</span><span class="token punctuation">]</span> <span class="token operator">=</span> currency_rel<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 准备函数处理器</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span> handlers<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.get"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletGetHandler<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletTransferHandler<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.deposit"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletDepositHandler<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.withdraw"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletWithdrawHandler<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册虚拟对象</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span>client<span class="token punctuation">.</span><span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span>wallet_desc<span class="token punctuation">,</span> handlers<span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Failed to register wallet entity"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="示例3-组件级注册-最优雅" tabindex="-1"><a class="header-anchor" href="#示例3-组件级注册-最优雅"><span>示例3：组件级注册（最优雅）</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>service_id <span class="token operator">=</span> <span class="token string">"economy-system"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 方式A：从配置文件加载</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span>client<span class="token punctuation">.</span><span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token string">"economy-system.component.json"</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Failed to load economy component"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 方式B：程序化定义组件</span></span>
<span class="line">    ComponentDescriptor economy_comp<span class="token punctuation">;</span></span>
<span class="line">    economy_comp<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"economy-system"</span><span class="token punctuation">;</span></span>
<span class="line">    economy_comp<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">;</span></span>
<span class="line">    economy_comp<span class="token punctuation">.</span>name <span class="token operator">=</span> <span class="token string">"经济系统"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 添加钱包实体</span></span>
<span class="line">    VirtualObjectDescriptor wallet_entity <span class="token operator">=</span> <span class="token function">BuildWalletEntity</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    economy_comp<span class="token punctuation">.</span>entities<span class="token punctuation">.</span><span class="token function">push_back</span><span class="token punctuation">(</span>wallet_entity<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 添加货币实体</span></span>
<span class="line">    VirtualObjectDescriptor currency_entity <span class="token operator">=</span> <span class="token function">BuildCurrencyEntity</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    economy_comp<span class="token punctuation">.</span>entities<span class="token punctuation">.</span><span class="token function">push_back</span><span class="token punctuation">(</span>currency_entity<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 添加跨实体函数</span></span>
<span class="line">    FunctionDescriptor market_trade<span class="token punctuation">;</span></span>
<span class="line">    market_trade<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"market.trade"</span><span class="token punctuation">;</span></span>
<span class="line">    market_trade<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">;</span></span>
<span class="line">    economy_comp<span class="token punctuation">.</span>functions<span class="token punctuation">.</span><span class="token function">push_back</span><span class="token punctuation">(</span>market_trade<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册整个组件</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span>client<span class="token punctuation">.</span><span class="token function">RegisterComponent</span><span class="token punctuation">(</span>economy_comp<span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Failed to register economy component"</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="示例4-配置文件驱动-生产推荐" tabindex="-1"><a class="header-anchor" href="#示例4-配置文件驱动-生产推荐"><span>示例4：配置文件驱动（生产推荐）</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// economy-system.component.json</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"id"</span><span class="token operator">:</span> <span class="token string">"economy-system"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"name"</span><span class="token operator">:</span> <span class="token string">"经济系统组件"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token string">"id"</span><span class="token operator">:</span> <span class="token string">"wallet.entity"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token string">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* ... */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token string">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"read"</span><span class="token operator">:</span> <span class="token string">"wallet.get"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"transfer"</span><span class="token operator">:</span> <span class="token string">"wallet.transfer"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token string">"relationships"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* ... */</span> <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token string">"id"</span><span class="token operator">:</span> <span class="token string">"currency.entity"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token string">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* ... */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token string">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"create"</span><span class="token operator">:</span> <span class="token string">"currency.create"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"read"</span><span class="token operator">:</span> <span class="token string">"currency.get"</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token string">"id"</span><span class="token operator">:</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token string">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* JSON Schema */</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token string">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token comment">/* JSON Schema */</span> <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 简洁的主程序</span></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 一行代码完成整个组件注册</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token string">"economy-system.component.json"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔧-实现指南" tabindex="-1"><a class="header-anchor" href="#🔧-实现指南"><span>🔧 实现指南</span></a></h2>
<h3 id="阶段1-扩展现有sdk" tabindex="-1"><a class="header-anchor" href="#阶段1-扩展现有sdk"><span>阶段1：扩展现有SDK</span></a></h3>
<ol>
<li>
<p><strong>扩展头文件</strong> (<code v-pre>croupier_client.h</code>)</p>
<ul>
<li>添加新的数据结构定义</li>
<li>扩展CroupierClient类接口</li>
<li>保持向后兼容性</li>
</ul>
</li>
<li>
<p><strong>实现核心逻辑</strong> (<code v-pre>croupier_client.cpp</code>)</p>
<ul>
<li>实现虚拟对象注册逻辑</li>
<li>添加配置文件解析功能</li>
<li>扩展现有的注册机制</li>
</ul>
</li>
<li>
<p><strong>添加工具函数</strong> (<code v-pre>utils.cpp</code>)</p>
<ul>
<li>JSON配置解析和验证</li>
<li>模板生成功能</li>
<li>错误处理和日志</li>
</ul>
</li>
</ol>
<h3 id="阶段2-生产化增强" tabindex="-1"><a class="header-anchor" href="#阶段2-生产化增强"><span>阶段2：生产化增强</span></a></h3>
<ol start="4">
<li>
<p><strong>配置验证系统</strong></p>
<ul>
<li>JSON Schema验证</li>
<li>关系一致性检查</li>
<li>循环依赖检测</li>
</ul>
</li>
<li>
<p><strong>开发工具支持</strong></p>
<ul>
<li>配置文件生成器</li>
<li>可视化编辑器集成</li>
<li>调试和诊断工具</li>
</ul>
</li>
<li>
<p><strong>性能优化</strong></p>
<ul>
<li>配置缓存机制</li>
<li>懒加载和热重载</li>
<li>批量操作优化</li>
</ul>
</li>
</ol>
<h2 id="🎯-架构优势" tabindex="-1"><a class="header-anchor" href="#🎯-架构优势"><span>🎯 架构优势</span></a></h2>
<h3 id="性能优势" tabindex="-1"><a class="header-anchor" href="#性能优势"><span>性能优势</span></a></h3>
<ul>
<li>✅ <strong>轻量参数</strong>：只传递ID字符串，网络开销极小</li>
<li>✅ <strong>无状态设计</strong>：函数可水平扩展，无状态共享问题</li>
<li>✅ <strong>缓存友好</strong>：多层级缓存对象数据</li>
</ul>
<h3 id="开发体验" tabindex="-1"><a class="header-anchor" href="#开发体验"><span>开发体验</span></a></h3>
<ul>
<li>✅ <strong>渐进增强</strong>：从简单函数逐步演进到复杂对象</li>
<li>✅ <strong>声明式配置</strong>：JSON驱动，易于理解和维护</li>
<li>✅ <strong>工具友好</strong>：配置可生成UI、文档、测试用例</li>
</ul>
<h3 id="架构设计" tabindex="-1"><a class="header-anchor" href="#架构设计"><span>架构设计</span></a></h3>
<ul>
<li>✅ <strong>职责清晰</strong>：函数专注业务逻辑，Repository管理对象</li>
<li>✅ <strong>类型安全</strong>：JSON Schema确保参数类型正确</li>
<li>✅ <strong>关系明确</strong>：通过Entity定义明确对象间关系</li>
</ul>
<h2 id="📚-参考模式" tabindex="-1"><a class="header-anchor" href="#📚-参考模式"><span>📚 参考模式</span></a></h2>
<p>该设计借鉴了多个成熟的架构模式：</p>
<ul>
<li><strong>DDD (Domain-Driven Design)</strong>：Entity概念映射到业务领域对象</li>
<li><strong>Repository Pattern</strong>：通过ID获取对象，分离业务逻辑和数据访问</li>
<li><strong>Microservice Architecture</strong>：无状态函数，易于分布式部署</li>
<li><strong>GraphQL思想</strong>：声明式查询，类型安全的API设计</li>
</ul>
<h2 id="🚀-后续规划" tabindex="-1"><a class="header-anchor" href="#🚀-后续规划"><span>🚀 后续规划</span></a></h2>
<ol>
<li><strong>立即实施</strong>：扩展C++ SDK，添加虚拟对象注册接口</li>
<li><strong>短期目标</strong>：完善配置验证和开发工具支持</li>
<li><strong>中期目标</strong>：实现代码生成和可视化编辑</li>
<li><strong>长期目标</strong>：性能优化和多语言SDK统一</li>
</ol>
<hr>
<p><strong>通过这套架构，您可以优雅地管理复杂的游戏业务对象，同时保持高性能和良好的开发体验！</strong></p>
</div></template>


