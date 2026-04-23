<template><div><h1 id="protobuf-自定义-options-使用指南" tabindex="-1"><a class="header-anchor" href="#protobuf-自定义-options-使用指南"><span>Protobuf 自定义 Options 使用指南</span></a></h1>
<h2 id="概述" tabindex="-1"><a class="header-anchor" href="#概述"><span>概述</span></a></h2>
<p>protoc-gen-croupier 插件支持通过 protobuf 自定义 options 来控制生成后的函数描述符（descriptors）和 UI 配置。这些 options 允许您在不修改生成代码的情况下，自定义函数的行为、权限、展示方式等。</p>
<h2 id="支持的-options" tabindex="-1"><a class="header-anchor" href="#支持的-options"><span>支持的 Options</span></a></h2>
<h3 id="_1-函数级-options-croupier-options-v1-functionoptions" tabindex="-1"><a class="header-anchor" href="#_1-函数级-options-croupier-options-v1-functionoptions"><span>1. 函数级 Options (<code v-pre>croupier.options.v1.FunctionOptions</code>)</span></a></h3>
<p>通过在 RPC 方法上添加 <code v-pre>(croupier.options.v1.function)</code> 选项来配置函数：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">import</span> <span class="token string">"croupier/options/v1/function.proto"</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">import</span> <span class="token string">"croupier/common/v1/ui.proto"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">service</span> <span class="token class-name">PlayerService</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">rpc</span> <span class="token function">GetPlayer</span><span class="token punctuation">(</span><span class="token class-name">GetPlayerRequest</span><span class="token punctuation">)</span> <span class="token keyword">returns</span> <span class="token punctuation">(</span><span class="token class-name">GetPlayerResponse</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">option</span> <span class="token punctuation">(</span>croupier<span class="token punctuation">.</span>options<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>function<span class="token punctuation">)</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">      function_id<span class="token punctuation">:</span> <span class="token string">"player.get_info"</span><span class="token punctuation">;</span>      <span class="token comment">// 全局唯一函数ID</span></span>
<span class="line">      version<span class="token punctuation">:</span> <span class="token string">"1.2.0"</span><span class="token punctuation">;</span>                    <span class="token comment">// 版本号</span></span>
<span class="line">      category<span class="token punctuation">:</span> <span class="token string">"player"</span><span class="token punctuation">;</span>                   <span class="token comment">// 分类</span></span>
<span class="line">      risk<span class="token punctuation">:</span> <span class="token string">"low"</span><span class="token punctuation">;</span>                          <span class="token comment">// 风险级别：low/medium/high</span></span>
<span class="line">      mode<span class="token punctuation">:</span> <span class="token string">"query"</span><span class="token punctuation">;</span>                        <span class="token comment">// 调用模式：query/command</span></span>
<span class="line">      timeout<span class="token punctuation">:</span> <span class="token string">"10s"</span><span class="token punctuation">;</span>                       <span class="token comment">// 超时时间</span></span>
<span class="line">      route<span class="token punctuation">:</span> <span class="token string">"lb"</span><span class="token punctuation">;</span>                          <span class="token comment">// 路由策略：lb/broadcast/targeted/hash</span></span>
<span class="line">      placement<span class="token punctuation">:</span> <span class="token string">"agent"</span><span class="token punctuation">;</span>                   <span class="token comment">// 部署位置：core/agent</span></span>
<span class="line">      idempotency_key<span class="token punctuation">:</span> <span class="token boolean">true</span><span class="token punctuation">;</span>               <span class="token comment">// 是否支持幂等键</span></span>
<span class="line">      two_person_rule<span class="token punctuation">:</span> <span class="token boolean">true</span><span class="token punctuation">;</span>               <span class="token comment">// 是否需要双人审核</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment">// 展示相关</span></span>
<span class="line">      display_name<span class="token punctuation">:</span> <span class="token punctuation">{</span></span>
<span class="line">        zh<span class="token punctuation">:</span> <span class="token string">"获取玩家信息"</span><span class="token punctuation">,</span></span>
<span class="line">        en<span class="token punctuation">:</span> <span class="token string">"Get Player Info"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">      summary<span class="token punctuation">:</span> <span class="token punctuation">{</span></span>
<span class="line">        zh<span class="token punctuation">:</span> <span class="token string">"根据玩家ID获取玩家信息"</span><span class="token punctuation">,</span></span>
<span class="line">        en<span class="token punctuation">:</span> <span class="token string">"Retrieve player information by ID"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">      tags<span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"player"</span><span class="token punctuation">,</span> <span class="token string">"info"</span><span class="token punctuation">]</span><span class="token punctuation">;</span>            <span class="token comment">// 标签列表</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment">// 权限配置</span></span>
<span class="line">      permissions<span class="token punctuation">:</span> <span class="token punctuation">{</span></span>
<span class="line">        verbs<span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">]</span><span class="token punctuation">;</span>                   <span class="token comment">// 所需操作权限</span></span>
<span class="line">        scopes<span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"game"</span><span class="token punctuation">,</span> <span class="token string">"env"</span><span class="token punctuation">,</span> <span class="token string">"function_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span> <span class="token comment">// 支持的 ABAC scopes（示例）</span></span>
<span class="line">        defaults<span class="token punctuation">:</span> <span class="token punctuation">[</span>                        <span class="token comment">// 默认角色 → verbs</span></span>
<span class="line">          <span class="token punctuation">{</span> role<span class="token punctuation">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span> verbs<span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">]</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token punctuation">{</span> role<span class="token punctuation">:</span> <span class="token string">"operator"</span><span class="token punctuation">,</span> verbs<span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">]</span> <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-字段级-ui-options-croupier-options-v1-ui" tabindex="-1"><a class="header-anchor" href="#_2-字段级-ui-options-croupier-options-v1-ui"><span>2. 字段级 UI Options (<code v-pre>croupier.options.v1.ui</code>)</span></a></h3>
<p>通过在消息字段上添加 <code v-pre>(croupier.options.v1.ui)</code> 选项来配置 UI 展示：</p>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">GetPlayerRequest</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token builtin">uint64</span> player_id <span class="token operator">=</span> <span class="token number">1</span> <span class="token punctuation">[</span><span class="token punctuation">(</span>croupier<span class="token punctuation">.</span>options<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>ui<span class="token punctuation">)</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">    label<span class="token punctuation">:</span> <span class="token string">"玩家ID"</span><span class="token punctuation">,</span></span>
<span class="line">    widget<span class="token punctuation">:</span> <span class="token string">"input"</span><span class="token punctuation">,</span></span>
<span class="line">    placeholder<span class="token punctuation">:</span> <span class="token string">"请输入玩家ID"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token builtin">string</span> status <span class="token operator">=</span> <span class="token number">2</span> <span class="token punctuation">[</span><span class="token punctuation">(</span>croupier<span class="token punctuation">.</span>options<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>ui<span class="token punctuation">)</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">    label<span class="token punctuation">:</span> <span class="token string">"状态"</span><span class="token punctuation">,</span></span>
<span class="line">    widget<span class="token punctuation">:</span> <span class="token string">"select"</span><span class="token punctuation">,</span></span>
<span class="line">    enum_map<span class="token punctuation">:</span> <span class="token punctuation">{</span> key<span class="token punctuation">:</span> <span class="token string">"ACTIVE"</span><span class="token punctuation">,</span> value<span class="token punctuation">:</span> <span class="token string">"正常"</span> <span class="token punctuation">}</span></span>
<span class="line">    enum_map<span class="token punctuation">:</span> <span class="token punctuation">{</span> key<span class="token punctuation">:</span> <span class="token string">"BANNED"</span><span class="token punctuation">,</span> value<span class="token punctuation">:</span> <span class="token string">"封禁"</span> <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token builtin">string</span> reason <span class="token operator">=</span> <span class="token number">3</span> <span class="token punctuation">[</span><span class="token punctuation">(</span>croupier<span class="token punctuation">.</span>options<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>ui<span class="token punctuation">)</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">    label<span class="token punctuation">:</span> <span class="token string">"原因"</span><span class="token punctuation">,</span></span>
<span class="line">    widget<span class="token punctuation">:</span> <span class="token string">"textarea"</span><span class="token punctuation">,</span></span>
<span class="line">    placeholder<span class="token punctuation">:</span> <span class="token string">"请输入详细原因"</span><span class="token punctuation">,</span></span>
<span class="line">    sensitive<span class="token punctuation">:</span> <span class="token boolean">true</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-支持的-ui-组件类型" tabindex="-1"><a class="header-anchor" href="#_3-支持的-ui-组件类型"><span>3. 支持的 UI 组件类型</span></a></h3>
<ul>
<li><strong>input</strong>: 文本输入框</li>
<li><strong>number</strong>: 数字输入框</li>
<li><strong>textarea</strong>: 多行文本</li>
<li><strong>select</strong>: 下拉选择（单选）</li>
<li><strong>multiselect</strong>: 下拉选择（多选）</li>
<li><strong>checkbox</strong>: 复选框</li>
<li><strong>radio</strong>: 单选按钮组</li>
<li><strong>date</strong>: 日期选择</li>
<li><strong>datetime</strong>: 日期时间选择</li>
<li><strong>time</strong>: 时间选择</li>
<li><strong>upload</strong>: 文件上传</li>
<li><strong>switch</strong>: 开关</li>
<li><strong>slider</strong>: 滑块</li>
<li><strong>rate</strong>: 评分</li>
<li><strong>color</strong>: 颜色选择</li>
<li><strong>password</strong>: 密码输入</li>
<li><strong>email</strong>: 邮箱输入</li>
<li><strong>url</strong>: URL输入</li>
<li><strong>phone</strong>: 电话号码输入</li>
</ul>
<h3 id="_4-高级配置" tabindex="-1"><a class="header-anchor" href="#_4-高级配置"><span>4. 高级配置</span></a></h3>
<h4 id="路由选项" tabindex="-1"><a class="header-anchor" href="#路由选项"><span>路由选项</span></a></h4>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">option</span> <span class="token punctuation">(</span>croupier<span class="token punctuation">.</span>options<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>function<span class="token punctuation">)</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">  route<span class="token punctuation">:</span> <span class="token string">"hash"</span><span class="token punctuation">;</span>                         <span class="token comment">// 基于哈希的路由</span></span>
<span class="line">  routing<span class="token punctuation">:</span> <span class="token punctuation">{</span>                              <span class="token comment">// 路由配置</span></span>
<span class="line">    hash_key<span class="token punctuation">:</span> <span class="token string">"player_id"</span><span class="token punctuation">,</span>              <span class="token comment">// 哈希键字段</span></span>
<span class="line">    jsonpath<span class="token punctuation">:</span> <span class="token string">"$.request.player_id"</span>     <span class="token comment">// JSONPath 表达式</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="速率限制-todo-待实现" tabindex="-1"><a class="header-anchor" href="#速率限制-todo-待实现"><span>速率限制（TODO：待实现）</span></a></h4>
<div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre v-pre><code class="language-protobuf"><span class="line"><span class="token keyword">option</span> <span class="token punctuation">(</span>croupier<span class="token punctuation">.</span>options<span class="token punctuation">.</span>v1<span class="token punctuation">.</span>function<span class="token punctuation">)</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token comment">// 速率限制配置（需在 schema 和生成器中完善支持）</span></span>
<span class="line">  rate_limit<span class="token punctuation">:</span> <span class="token punctuation">{</span></span>
<span class="line">    value<span class="token punctuation">:</span> <span class="token number">100</span><span class="token punctuation">,</span>                          <span class="token comment">// 请求数</span></span>
<span class="line">    window<span class="token punctuation">:</span> <span class="token string">"1m"</span>                        <span class="token comment">// 时间窗口</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">  concurrency<span class="token punctuation">:</span> <span class="token number">10</span><span class="token punctuation">;</span>                       <span class="token comment">// 最大并发数</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="生成的输出" tabindex="-1"><a class="header-anchor" href="#生成的输出"><span>生成的输出</span></a></h2>
<p>当运行 protoc-gen-croupier 时，这些 options 会被转换为：</p>
<ol>
<li><strong>manifest.json</strong>: 包含函数的元数据</li>
<li><strong>descriptors/*.json</strong>: 函数描述符，包含 transport、auth、semantics 等信息</li>
<li><strong>ui/*.schema.json</strong>: JSON Schema 定义</li>
<li><strong>ui/*.uischema.json</strong>: UI 配置，用于表单渲染</li>
</ol>
<h2 id="最佳实践" tabindex="-1"><a class="header-anchor" href="#最佳实践"><span>最佳实践</span></a></h2>
<ol>
<li>
<p><strong>命名规范</strong></p>
<ul>
<li><code v-pre>function_id</code>: 使用全小写，用点分隔命名空间，如 <code v-pre>player.get_info</code></li>
<li><code v-pre>category</code>: 使用短名称，如 <code v-pre>player</code>, <code v-pre>inventory</code>, <code v-pre>payment</code></li>
</ul>
</li>
<li>
<p><strong>安全配置</strong></p>
<ul>
<li>对敏感操作使用 <code v-pre>two_person_rule: true</code></li>
<li>根据风险级别设置 <code v-pre>risk</code> 字段</li>
<li>精细配置 <code v-pre>permissions</code></li>
</ul>
</li>
<li>
<p><strong>用户体验</strong></p>
<ul>
<li>使用 <code v-pre>display_name</code> 提供多语言支持</li>
<li>通过 <code v-pre>summary</code> 清晰描述功能</li>
<li>使用 <code v-pre>tags</code> 便于分类和搜索</li>
</ul>
</li>
<li>
<p><strong>UI 配置</strong></p>
<ul>
<li>为所有输入字段提供清晰的 <code v-pre>label</code></li>
<li>使用适当的 <code v-pre>widget</code> 类型</li>
<li>设置合理的 <code v-pre>required</code> 和验证规则</li>
</ul>
</li>
</ol>
<h2 id="示例项目" tabindex="-1"><a class="header-anchor" href="#示例项目"><span>示例项目</span></a></h2>
<p>完整示例请参考：<code v-pre>docs/proto-options-example.proto</code></p>
<h2 id="注意事项" tabindex="-1"><a class="header-anchor" href="#注意事项"><span>注意事项</span></a></h2>
<ol>
<li>确保 import 相关的 proto 文件</li>
<li>options 的值需要是字符串类型（即使在 proto 中定义为其他类型）</li>
<li>UI options 中的 <code v-pre>default</code> 值需要是字符串格式</li>
<li>生成的 pack 可以使用 <code v-pre>./bin/croupier packs validate</code> 验证</li>
</ol>
</div></template>


