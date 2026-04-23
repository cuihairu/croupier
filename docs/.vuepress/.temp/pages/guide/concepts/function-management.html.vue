<template><div><h1 id="函数管理" tabindex="-1"><a class="header-anchor" href="#函数管理"><span>函数管理</span></a></h1>
<p>Croupier 采用<strong>函数注册驱动</strong>的架构，游戏服务器通过 Agent 注册函数，控制面统一管理、路由和调用。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#什么是函数">什么是函数</router-link><ul><li><router-link to="#函数特性">函数特性</router-link></li></ul></li><li><router-link to="#函数生命周期">函数生命周期</router-link></li><li><router-link to="#函数描述符">函数描述符</router-link><ul><li><router-link to="#完整描述符示例">完整描述符示例</router-link></li></ul></li><li><router-link to="#函数注册">函数注册</router-link><ul><li><router-link to="#注册流程">注册流程</router-link></li><li><router-link to="#go-sdk-注册示例">Go SDK 注册示例</router-link></li><li><router-link to="#c-sdk-注册示例">C++ SDK 注册示例</router-link></li></ul></li><li><router-link to="#函数调用">函数调用</router-link><ul><li><router-link to="#同步调用">同步调用</router-link></li><li><router-link to="#异步调用-作业">异步调用（作业）</router-link></li><li><router-link to="#作业事件流">作业事件流</router-link></li></ul></li><li><router-link to="#函数路由">函数路由</router-link><ul><li><router-link to="#负载均衡策略">负载均衡策略</router-link></li><li><router-link to="#路由模式">路由模式</router-link></li></ul></li><li><router-link to="#函数控制">函数控制</router-link><ul><li><router-link to="#权限控制">权限控制</router-link></li><li><router-link to="#审批流程">审批流程</router-link></li><li><router-link to="#限流">限流</router-link></li></ul></li><li><router-link to="#函数包">函数包</router-link><ul><li><router-link to="#包结构">包结构</router-link></li><li><router-link to="#导入-导出">导入/导出</router-link></li></ul></li><li><router-link to="#最佳实践">最佳实践</router-link><ul><li><router-link to="#_1-函数命名">1. 函数命名</router-link></li><li><router-link to="#_2-参数验证">2. 参数验证</router-link></li><li><router-link to="#_3-错误处理">3. 错误处理</router-link></li><li><router-link to="#_4-幂等性">4. 幂等性</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="什么是函数" tabindex="-1"><a class="header-anchor" href="#什么是函数"><span>什么是函数</span></a></h2>
<p>在 Croupier 中，<strong>函数 (Function)</strong> 是最小的可执行单元，代表一个具体的业务操作。</p>
<h3 id="函数特性" tabindex="-1"><a class="header-anchor" href="#函数特性"><span>函数特性</span></a></h3>
<table>
<thead>
<tr>
<th>特性</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>自描述</strong></td>
<td>通过 JSON Schema 描述输入输出</td>
</tr>
<tr>
<td><strong>可发现</strong></td>
<td>注册到控制面，可被查询和调用</td>
</tr>
<tr>
<td><strong>可控制</strong></td>
<td>支持权限、审批、限流等控制</td>
</tr>
<tr>
<td><strong>可观测</strong></td>
<td>调用记录审计日志</td>
</tr>
</tbody>
</table>
<h2 id="函数生命周期" tabindex="-1"><a class="header-anchor" href="#函数生命周期"><span>函数生命周期</span></a></h2>
<Mermaid code="eJwrLkksSXXJTEwvSszVLTPiUgCCaK1YBV1dO4Vnc1Y927ziaVsPWBTOA8s93b4JwrNScExPzSvRC0pNzywuSS1yK81LLsnMz9PQBOuCq4OYuGX3i+3NVgoeqYlFJUmpiSUKT/bPBQpCLABLopv+sr332bQNyO5AV/ZySgPMEaF5RdidgaTp+bLdz3ftt1J4sa312fRtQF893d/8YvtmsDKIHIpTIfa/2D/vWd9SJB8BLQUrA4YVFwBNzJi4"></Mermaid><h2 id="函数描述符" tabindex="-1"><a class="header-anchor" href="#函数描述符"><span>函数描述符</span></a></h2>
<p>函数通过 <strong>描述符 (Descriptor)</strong> 进行定义，包含完整的元数据。</p>
<h3 id="完整描述符示例" tabindex="-1"><a class="header-anchor" href="#完整描述符示例"><span>完整描述符示例</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"封禁玩家"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"封禁指定玩家账号"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"category"</span><span class="token operator">:</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"玩家ID"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"description"</span><span class="token operator">:</span> <span class="token string">"要封禁的玩家唯一标识"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"封禁时长（小时）"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"minimum"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"maximum"</span><span class="token operator">:</span> <span class="token number">8760</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"default"</span><span class="token operator">:</span> <span class="token number">24</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"reason"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"封禁原因"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"minLength"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"maxLength"</span><span class="token operator">:</span> <span class="token number">500</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"required"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">,</span> <span class="token string">"duration"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">  <span class="token property">"result"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"success"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"boolean"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"ban_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"expires_at"</span><span class="token operator">:</span> <span class="token punctuation">{</span><span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span> <span class="token property">"format"</span><span class="token operator">:</span> <span class="token string">"date-time"</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"approval"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"threshold"</span><span class="token operator">:</span> <span class="token number">2</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"idempotent"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"rate_limit"</span><span class="token operator">:</span> <span class="token string">"10rps"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"timeout"</span><span class="token operator">:</span> <span class="token string">"30s"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"></span>
<span class="line">  <span class="token property">"ui"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"risk_level"</span><span class="token operator">:</span> <span class="token string">"high"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"risk_warning"</span><span class="token operator">:</span> <span class="token string">"高风险操作，封禁后玩家无法登录"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"confirm_message"</span><span class="token operator">:</span> <span class="token string">"确认封禁玩家 {player_id}？"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数注册" tabindex="-1"><a class="header-anchor" href="#函数注册"><span>函数注册</span></a></h2>
<h3 id="注册流程" tabindex="-1"><a class="header-anchor" href="#注册流程"><span>注册流程</span></a></h3>
<Mermaid code="eJwrTi0sTc1LTnXJTEwvSszlUgCCgsSikszkzILEvBIF98TcVIXEYggdnFpUllqEocYxPRVIAhU5F+WXFmSmFkFEMNRBtKMoxGFiUGp6ZnFJUSVIrVtpXnJJZn4eXJALrBzkIF07O7BNVgrpQQHOVlAVqUUwLRopqcXJRZkFJflFmmBNYNVAXRBridYGUQ7UB3OClcLTtTOeNq142r732dQNz/r7X+zfAFYJU6CLZMmzzSuetvU865jwtGs+snEI12OogLgTqADkSzR5AFAwoyE="></Mermaid><h3 id="go-sdk-注册示例" tabindex="-1"><a class="header-anchor" href="#go-sdk-注册示例"><span>Go SDK 注册示例</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier-sdk-go/client"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 创建 Agent 客户端</span></span>
<span class="line">    agent <span class="token operator">:=</span> client<span class="token punctuation">.</span><span class="token function">NewAgent</span><span class="token punctuation">(</span>client<span class="token punctuation">.</span>Config<span class="token punctuation">{</span></span>
<span class="line">        ServerAddr<span class="token punctuation">:</span> <span class="token string">"localhost:19090"</span><span class="token punctuation">,</span></span>
<span class="line">        GameID<span class="token punctuation">:</span>     <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">        Env<span class="token punctuation">:</span>        <span class="token string">"dev"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册函数</span></span>
<span class="line">    err <span class="token operator">:=</span> agent<span class="token punctuation">.</span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token operator">&amp;</span>client<span class="token punctuation">.</span>FunctionDescriptor<span class="token punctuation">{</span></span>
<span class="line">        ID<span class="token punctuation">:</span>          <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">        Name<span class="token punctuation">:</span>        <span class="token string">"封禁玩家"</span><span class="token punctuation">,</span></span>
<span class="line">        Category<span class="token punctuation">:</span>    <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">        Handler<span class="token punctuation">:</span>     BanPlayer<span class="token punctuation">,</span></span>
<span class="line">        ParamsSchema<span class="token punctuation">:</span> paramsSchema<span class="token punctuation">,</span></span>
<span class="line">        ResultSchema<span class="token punctuation">:</span> resultSchema<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token function">panic</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">BanPlayer</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> input <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 函数实现</span></span>
<span class="line">    playerID <span class="token operator">:=</span> input<span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">]</span><span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">string</span><span class="token punctuation">)</span></span>
<span class="line">    duration <span class="token operator">:=</span> <span class="token function">int</span><span class="token punctuation">(</span>input<span class="token punctuation">[</span><span class="token string">"duration"</span><span class="token punctuation">]</span><span class="token punctuation">.</span><span class="token punctuation">(</span><span class="token builtin">float64</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 执行封禁逻辑...</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"success"</span><span class="token punctuation">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"ban_id"</span><span class="token punctuation">:</span>  <span class="token string">"ban_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="c-sdk-注册示例" tabindex="-1"><a class="header-anchor" href="#c-sdk-注册示例"><span>C++ SDK 注册示例</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;croupier/sdk/client.h></span></span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">class</span> <span class="token class-name">BanPlayerFunction</span> <span class="token operator">:</span> <span class="token base-clause"><span class="token keyword">public</span> <span class="token class-name">Function</span></span> <span class="token punctuation">{</span></span>
<span class="line"><span class="token keyword">public</span><span class="token operator">:</span></span>
<span class="line">    Descriptor <span class="token function">GetDescriptor</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">const</span> <span class="token keyword">override</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> Descriptor<span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">set_id</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">)</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">set_name</span><span class="token punctuation">(</span><span class="token string">"封禁玩家"</span><span class="token punctuation">)</span></span>
<span class="line">            <span class="token punctuation">.</span><span class="token function">set_category</span><span class="token punctuation">(</span><span class="token string">"player"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    Result <span class="token function">Call</span><span class="token punctuation">(</span><span class="token keyword">const</span> Context<span class="token operator">&amp;</span> ctx<span class="token punctuation">,</span> <span class="token keyword">const</span> nlohmann<span class="token double-colon punctuation">::</span>json<span class="token operator">&amp;</span> params<span class="token punctuation">)</span> <span class="token keyword">override</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>string player_id <span class="token operator">=</span> params<span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">int</span> duration <span class="token operator">=</span> params<span class="token punctuation">[</span><span class="token string">"duration"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 执行封禁逻辑...</span></span>
<span class="line"></span>
<span class="line">        <span class="token keyword">return</span> Result<span class="token punctuation">{</span></span>
<span class="line">            <span class="token punctuation">{</span><span class="token string">"success"</span><span class="token punctuation">,</span> <span class="token boolean">true</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token punctuation">{</span><span class="token string">"ban_id"</span><span class="token punctuation">,</span> <span class="token string">"ban_123"</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    AgentClient <span class="token function">agent</span><span class="token punctuation">(</span>AgentConfig<span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">set_server_addr</span><span class="token punctuation">(</span><span class="token string">"localhost:19090"</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">set_game_id</span><span class="token punctuation">(</span><span class="token string">"my-game"</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">.</span><span class="token function">set_env</span><span class="token punctuation">(</span><span class="token string">"dev"</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    agent<span class="token punctuation">.</span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>std<span class="token double-colon punctuation">::</span><span class="token generic-function"><span class="token function">make_unique</span><span class="token generic class-name"><span class="token operator">&lt;</span>BanPlayerFunction<span class="token operator">></span></span></span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    agent<span class="token punctuation">.</span><span class="token function">Run</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数调用" tabindex="-1"><a class="header-anchor" href="#函数调用"><span>函数调用</span></a></h2>
<h3 id="同步调用" tabindex="-1"><a class="header-anchor" href="#同步调用"><span>同步调用</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/invoke <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Authorization: Bearer <span class="token variable">$TOKEN</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Game-ID: my-game"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Env: dev"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{</span>
<span class="line">    "function_id": "player.ban",</span>
<span class="line">    "payload": {</span>
<span class="line">      "player_id": "player_123",</span>
<span class="line">      "duration": 24,</span>
<span class="line">      "reason": "作弊"</span>
<span class="line">    }</span>
<span class="line">  }'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="异步调用-作业" tabindex="-1"><a class="header-anchor" href="#异步调用-作业"><span>异步调用（作业）</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/jobs <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Authorization: Bearer <span class="token variable">$TOKEN</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{</span>
<span class="line">    "function_id": "player.ban",</span>
<span class="line">    "payload": {...}</span>
<span class="line">  }'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="作业事件流" tabindex="-1"><a class="header-anchor" href="#作业事件流"><span>作业事件流</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 获取作业状态</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/api/jobs/<span class="token punctuation">{</span>job_id<span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 流式获取事件</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/api/jobs/<span class="token punctuation">{</span>job_id<span class="token punctuation">}</span>/events</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数路由" tabindex="-1"><a class="header-anchor" href="#函数路由"><span>函数路由</span></a></h2>
<h3 id="负载均衡策略" tabindex="-1"><a class="header-anchor" href="#负载均衡策略"><span>负载均衡策略</span></a></h3>
<p>Server 支持多种负载均衡策略：</p>
<table>
<thead>
<tr>
<th>策略</th>
<th>说明</th>
<th>适用场景</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Round Robin</strong></td>
<td>轮询</td>
<td>请求均匀分布</td>
</tr>
<tr>
<td><strong>Consistent Hash</strong></td>
<td>一致性哈希</td>
<td>需要会话亲和</td>
</tr>
<tr>
<td><strong>Least Connection</strong></td>
<td>最少连接</td>
<td>动态负载</td>
</tr>
<tr>
<td><strong>Targeted</strong></td>
<td>指定 Agent</td>
<td>调试和测试</td>
</tr>
</tbody>
</table>
<h3 id="路由模式" tabindex="-1"><a class="header-anchor" href="#路由模式"><span>路由模式</span></a></h3>
<Mermaid code="eJxLL0osyFDwCeJSAALHaLfSvOSSzPw8BefEnJxYBV1dOwWn6hfb1z+fsvHZioVP9/TXghU6gWRqfPITUxScEnMS85JTaxSco58umfVkxyoFx/TUvJJYJHVORUCFyYnFJTUKLtHPOhuezenEVBSSWJSeWpKaUqPgGv2sp/3pulkwNQA1dDyy"></Mermaid><h2 id="函数控制" tabindex="-1"><a class="header-anchor" href="#函数控制"><span>函数控制</span></a></h2>
<h3 id="权限控制" tabindex="-1"><a class="header-anchor" href="#权限控制"><span>权限控制</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"has_role('admin') || has_role('gm')"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审批流程" tabindex="-1"><a class="header-anchor" href="#审批流程"><span>审批流程</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"approval"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"threshold"</span><span class="token operator">:</span> <span class="token number">2</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"approvers"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">,</span> <span class="token string">"senior_gm"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="限流" tabindex="-1"><a class="header-anchor" href="#限流"><span>限流</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"rate_limit"</span><span class="token operator">:</span> <span class="token string">"10rps"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"concurrency"</span><span class="token operator">:</span> <span class="token number">5</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数包" tabindex="-1"><a class="header-anchor" href="#函数包"><span>函数包</span></a></h2>
<p>函数可以打包成 <code v-pre>.tgz</code> 文件进行分发。</p>
<h3 id="包结构" tabindex="-1"><a class="header-anchor" href="#包结构"><span>包结构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">player-management-1.0.0.tgz</span>
<span class="line">├── manifest.json</span>
<span class="line">└── descriptors/</span>
<span class="line">    ├── player.entity.json</span>
<span class="line">    ├── player.resource.json</span>
<span class="line">    ├── player.register.json</span>
<span class="line">    ├── player.get.json</span>
<span class="line">    ├── player.ban.json</span>
<span class="line">    └── player.list.json</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="导入-导出" tabindex="-1"><a class="header-anchor" href="#导入-导出"><span>导入/导出</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 导出函数包</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/packs/export <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{"functions": ["player.*"]}'</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 导入函数包</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/packs/import <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-F</span> <span class="token string">"pack=@player-management-1.0.0.tgz"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="最佳实践" tabindex="-1"><a class="header-anchor" href="#最佳实践"><span>最佳实践</span></a></h2>
<h3 id="_1-函数命名" tabindex="-1"><a class="header-anchor" href="#_1-函数命名"><span>1. 函数命名</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">{entity}.{operation}</span>
<span class="line"></span>
<span class="line">示例：</span>
<span class="line">- player.register</span>
<span class="line">- player.ban</span>
<span class="line">- item.create</span>
<span class="line">- item.delete</span>
<span class="line">- guild.disband</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-参数验证" tabindex="-1"><a class="header-anchor" href="#_2-参数验证"><span>2. 参数验证</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"params"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"properties"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"player_id"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"pattern"</span><span class="token operator">:</span> <span class="token string">"^[a-zA-Z0-9_]{3,32}$"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"duration"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"minimum"</span><span class="token operator">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"maximum"</span><span class="token operator">:</span> <span class="token number">8760</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"required"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"player_id"</span><span class="token punctuation">,</span> <span class="token string">"duration"</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-错误处理" tabindex="-1"><a class="header-anchor" href="#_3-错误处理"><span>3. 错误处理</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">return</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">    <span class="token string">"success"</span><span class="token punctuation">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"error"</span><span class="token punctuation">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"code"</span><span class="token punctuation">:</span> <span class="token string">"PLAYER_NOT_FOUND"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"message"</span><span class="token punctuation">:</span> <span class="token string">"玩家不存在"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"details"</span><span class="token punctuation">:</span> <span class="token punctuation">{</span><span class="token string">"player_id"</span><span class="token punctuation">:</span> playerID<span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-幂等性" tabindex="-1"><a class="header-anchor" href="#_4-幂等性"><span>4. 幂等性</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"semantics"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"idempotent"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"idempotency_key"</span><span class="token operator">:</span> <span class="token string">"player_id"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/guide/concepts/virtual-objects.html">虚拟对象系统</RouteLink></li>
<li><RouteLink to="/guide/concepts/permissions.html">权限控制</RouteLink></li>
<li><RouteLink to="/api/grpc.html">gRPC API</RouteLink></li>
<li><RouteLink to="/api/rest.html">REST API</RouteLink></li>
</ul>
</div></template>


