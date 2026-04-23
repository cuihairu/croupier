<template><div><p>Provider Manifest（语言无关）— 设计说明</p>
<p>目标</p>
<ul>
<li>用一份语言无关的 JSON 清单宣告 Provider 能力（函数与实体/虚拟对象及其操作）。</li>
<li>基于清单驱动参数校验（JSON Schema）、UI 表单生成（hints）、权限/限流语义、以及传输映射（可选 Proto FQN）。</li>
<li>同时支持 in‑proc（Go 内嵌）与 out‑of‑proc（Python/Node 等独立进程）两种 SDK 形态。</li>
</ul>
<p>核心概念</p>
<ul>
<li>provider：提供者元信息（id、version、language、sdk 版本等）。</li>
<li>function：具名能力（建议命名为 <code v-pre>entity.operation</code>），包含请求/响应（JSON‑Schema 或 Proto FQN）、权限、语义（rate_limit/concurrency/idempotent）、传输映射、UI 提示。</li>
<li>entity（实体/虚拟对象）：业务对象类型（可“虚拟”，仅有上下文/生命周期），含对象 schema 及一组操作（create/get/update/delete/custom…）。每个操作独立声明参数、权限、目标定位方式（如何找到某个对象实例）。</li>
<li>context（隐式）：<code v-pre>game_id</code>、<code v-pre>env</code>、<code v-pre>actor</code>、<code v-pre>trace_id</code>、headers 等由网关注入，不进入 payload schema。</li>
</ul>
<p>可选扩展（用于 UI/RBAC 生成）</p>
<ul>
<li><code v-pre>function.display_name</code>/<code v-pre>function.summary</code>：I18n 文本（en/zh）</li>
<li><code v-pre>function.menu</code>：前端菜单元数据（section/group/path/order/icon/badge/hidden）</li>
<li><code v-pre>function.permissions</code>：权限规范（verbs/scopes/defaults/i18n_zh）</li>
<li><code v-pre>function.labels</code>/<code v-pre>function.tags</code>：自定义标签/检索字段（不影响调用协议）</li>
</ul>
<p>Manifest 文件</p>
<ul>
<li>JSON 文档（建议用 <code v-pre>docs/providers-manifest.schema.json</code> 校验）。</li>
<li>请求/响应可引用 JSON‑Schema（推荐）或 Proto FQN。</li>
<li>与 <code v-pre>schema/*.json</code>（各参数 Schema）以及可选的 FDS（.desc）一起打包分发。</li>
</ul>
<p>注册流程（语言无关 SPI）</p>
<ul>
<li>Provider 进程加载 manifest 后，通过 ControlService 上报能力（可新增 RPC，或给现有 Register 扩展字段）：
<ul>
<li>上报 <code v-pre>provider</code> 元信息、<code v-pre>functions[]</code>、<code v-pre>entities[]</code>，以及内嵌或外链的 JSON‑Schema（也可用内容哈希 + 上传端点）。</li>
</ul>
</li>
<li>Server 接收后合并为统一 descriptors，暴露在 <code v-pre>/api/descriptors</code>，供 UI/RBAC/校验使用。</li>
<li>调用链路使用 FunctionService（gRPC）；载荷默认 JSON；指定 <code v-pre>transport.proto</code> 时，Server/Edge 可用 FDS 做 JSON↔Proto 转换。</li>
</ul>
<p>参数定义与校验</p>
<ul>
<li>首选 JSON‑Schema：
<ul>
<li>约束丰富：<code v-pre>required</code>、<code v-pre>min/max</code>、<code v-pre>enum</code>、<code v-pre>format</code>（email/hostname/ip/uri/date-time/color/json…）、<code v-pre>oneOf/anyOf</code> 等。</li>
<li>UI 提示：使用自定义扩展 <code v-pre>x-ui</code>（widget/options/placeholder/order）与 <code v-pre>x-mask</code>（敏感字段）。</li>
</ul>
</li>
<li>可选 Proto 映射：设置 <code v-pre>transport.proto.request_fqn/response_fqn</code> 并随包提供 <code v-pre>.desc</code>。</li>
<li>参数来源控制：字段级 <code v-pre>x-source: body|query|path|header|meta</code>（meta 从 context 读取）。</li>
</ul>
<p>虚拟对象（实体）</p>
<ul>
<li><code v-pre>entities[]</code> 定义：<code v-pre>id</code>、<code v-pre>title</code>、<code v-pre>color</code>、<code v-pre>schema</code>（JSON‑Schema/Proto）与 <code v-pre>operations[]</code>。</li>
<li>操作字段：
<ul>
<li><code v-pre>op</code>：如 <code v-pre>create</code>、<code v-pre>get</code>、<code v-pre>update</code>、<code v-pre>delete</code>、或 <code v-pre>custom_*</code>。</li>
<li><code v-pre>target</code>：如何定位对象（如 <code v-pre>{ &quot;field&quot;: &quot;session_id&quot; }</code> 或 <code v-pre>{ &quot;jsonpath&quot;: &quot;$.session.id&quot; }</code>），create/list 可不需要。</li>
<li><code v-pre>request/response</code>：Schema 或 Proto FQN。</li>
<li><code v-pre>auth.require</code>：默认权限建议（如 <code v-pre>session:create</code>）。</li>
</ul>
</li>
</ul>
<p>错误模型</p>
<ul>
<li>Provider 处理器需返回带类型的错误：<code v-pre>invalid_argument</code>、<code v-pre>not_found</code>、<code v-pre>already_exists</code>、<code v-pre>precondition_failed</code>、<code v-pre>rate_limited</code>、<code v-pre>deadline_exceeded</code>、<code v-pre>unavailable</code>、<code v-pre>unauthorized</code>、<code v-pre>forbidden</code>、<code v-pre>internal</code>。</li>
<li>Server/Edge 将其映射为 HTTP 状态码与“是否可重试”建议。</li>
</ul>
<p>路由与语义</p>
<ul>
<li><code v-pre>semantics.rate_limit</code>：如 <code v-pre>100/s</code>、<code v-pre>1000/m</code>，或对象 <code v-pre>{ value: 100, window: &quot;1s&quot; }</code>。</li>
<li><code v-pre>semantics.concurrency</code>：并发上限（整数）。</li>
<li><code v-pre>semantics.idempotent</code>：是否幂等。</li>
<li>负载均衡建议（可选）：<code v-pre>routing.hash_key</code>（字段名或 JSONPath），用于一致性哈希。</li>
</ul>
<p>打包</p>
<ul>
<li>provider.tgz（或目录）包含：
<ul>
<li><code v-pre>manifest.json</code>（本文件）</li>
<li><code v-pre>schema/*.json</code>（引用的 JSON‑Schema）</li>
<li>可选：<code v-pre>descriptors.fds</code>（FileDescriptorSet）</li>
<li>可选：<code v-pre>ui/*</code>（UI 附加资源）</li>
</ul>
</li>
</ul>
<p>Manifest 示例（节选）</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"provider"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span> <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.2.0"</span><span class="token punctuation">,</span> <span class="token property">"lang"</span><span class="token operator">:</span> <span class="token string">"python"</span><span class="token punctuation">,</span> <span class="token property">"sdk"</span><span class="token operator">:</span> <span class="token string">"croupier-py@0.3.0"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"functions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/ban_request.json"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"response"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/ban_response.json"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"require"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player:ban"</span><span class="token punctuation">]</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"idempotent"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span> <span class="token property">"rate_limit"</span><span class="token operator">:</span> <span class="token string">"100/s"</span><span class="token punctuation">,</span> <span class="token property">"concurrency"</span><span class="token operator">:</span> <span class="token number">10</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"transport"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"proto"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"request_fqn"</span><span class="token operator">:</span> <span class="token string">"croupier.player.v1.BanRequest"</span><span class="token punctuation">,</span> <span class="token property">"response_fqn"</span><span class="token operator">:</span> <span class="token string">"croupier.player.v1.BanResponse"</span> <span class="token punctuation">}</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span> <span class="token property">"risk"</span><span class="token operator">:</span> <span class="token string">"medium"</span> <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"session"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"Session"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"color"</span><span class="token operator">:</span> <span class="token string">"#1677ff"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"schema"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/session.json"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"op"</span><span class="token operator">:</span> <span class="token string">"create"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/create_session_request.json"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"response"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/session.json"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"require"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"session:create"</span><span class="token punctuation">]</span> <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"op"</span><span class="token operator">:</span> <span class="token string">"close"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"target"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"field"</span><span class="token operator">:</span> <span class="token string">"session_id"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"request"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/close_request.json"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"response"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"json_schema"</span><span class="token operator">:</span> <span class="token string">"schema/empty.json"</span> <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span> <span class="token property">"require"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"session:close"</span><span class="token punctuation">]</span> <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Proto‑First 生成</p>
<ul>
<li>计划扩展 <code v-pre>tools/protoc-gen-croupier</code> 支持 <code v-pre>emit_manifest=true</code>：
<ul>
<li>解析方法/消息及自定义注解，生成 <code v-pre>manifest.json</code> 与 <code v-pre>schema/*.json</code>。</li>
<li>将 RPC 映射为 <code v-pre>functions[]</code>，消息映射为 JSON‑Schema。</li>
<li>允许通过自定义 option 标注 <code v-pre>auth.require</code>、<code v-pre>semantics.*</code>、<code v-pre>entity/op/target</code>、<code v-pre>ui</code>。</li>
</ul>
</li>
</ul>
<p>控制面集成</p>
<ul>
<li>扩展 ControlService（新增 RPC 或字段）以接收 Provider 能力载荷（压缩后的 manifest JSON + 可选嵌入的 schema/fds）。</li>
<li>Server 合并为统一 descriptors 并暴露 <code v-pre>/api/descriptors</code>，供 UI 与校验使用。</li>
</ul>
<p>注意</p>
<ul>
<li>JSON 文件尽量使用 ASCII；颜色按 <code v-pre>#1677ff</code> 六位十六进制。</li>
<li><code v-pre>json_schema</code> 的路径相对 manifest 或包根目录。</li>
</ul>
</div></template>


