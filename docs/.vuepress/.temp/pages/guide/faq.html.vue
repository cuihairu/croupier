<template><div><h1 id="常见问题-faq" tabindex="-1"><a class="header-anchor" href="#常见问题-faq"><span>常见问题 (FAQ)</span></a></h1>
<p>本文档收集了新手最常遇到的问题和解答。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#基础概念">基础概念</router-link><ul><li><router-link to="#croupier-是什么">Croupier 是什么？</router-link></li><li><router-link to="#我需要-croupier-吗">我需要 Croupier 吗？</router-link></li><li><router-link to="#croupier-和传统-gm-系统有什么区别">Croupier 和传统 GM 系统有什么区别？</router-link></li></ul></li><li><router-link to="#快速开始">快速开始</router-link><ul><li><router-link to="#最快的上手方式是什么">最快的上手方式是什么？</router-link></li><li><router-link to="#必须使用子模块吗">必须使用子模块吗？</router-link></li><li><router-link to="#开发环境需要什么">开发环境需要什么？</router-link></li></ul></li><li><router-link to="#配置问题">配置问题</router-link><ul><li><router-link to="#默认端口是什么">默认端口是什么？</router-link></li><li><router-link to="#端口被占用怎么办">端口被占用怎么办？</router-link></li><li><router-link to="#必须配置-tls-证书吗">必须配置 TLS 证书吗？</router-link></li><li><router-link to="#如何启用-debug-日志">如何启用 debug 日志？</router-link></li></ul></li><li><router-link to="#函数开发">函数开发</router-link><ul><li><router-link to="#如何创建第一个函数">如何创建第一个函数？</router-link></li><li><router-link to="#函数定义在哪里">函数定义在哪里？</router-link></li><li><router-link to="#如何定义函数参数">如何定义函数参数？</router-link></li></ul></li><li><router-link to="#权限控制">权限控制</router-link><ul><li><router-link to="#如何创建角色">如何创建角色？</router-link></li><li><router-link to="#什么是双人审批">什么是双人审批？</router-link></li><li><router-link to="#如何配置权限">如何配置权限？</router-link></li></ul></li><li><router-link to="#运维问题">运维问题</router-link><ul><li><router-link to="#如何优雅关闭">如何优雅关闭？</router-link></li><li><router-link to="#如何备份数据">如何备份数据？</router-link></li><li><router-link to="#如何升级版本">如何升级版本？</router-link></li></ul></li><li><router-link to="#故障排查">故障排查</router-link><ul><li><router-link to="#server-启动失败怎么办">Server 启动失败怎么办？</router-link></li><li><router-link to="#agent-连不上-server">Agent 连不上 Server？</router-link></li><li><router-link to="#函数调用超时怎么办">函数调用超时怎么办？</router-link></li></ul></li><li><router-link to="#性能优化">性能优化</router-link><ul><li><router-link to="#支持多少并发">支持多少并发？</router-link></li><li><router-link to="#如何减少内存占用">如何减少内存占用？</router-link></li></ul></li><li><router-link to="#集成问题">集成问题</router-link><ul><li><router-link to="#如何与现有系统集成">如何与现有系统集成？</router-link></li><li><router-link to="#如何对接现有权限系统">如何对接现有权限系统？</router-link></li></ul></li><li><router-link to="#更多帮助">更多帮助</router-link></li></ul></nav>
<h2 id="基础概念" tabindex="-1"><a class="header-anchor" href="#基础概念"><span>基础概念</span></a></h2>
<h3 id="croupier-是什么" tabindex="-1"><a class="header-anchor" href="#croupier-是什么"><span>Croupier 是什么？</span></a></h3>
<p>Croupier 是一个<strong>分布式游戏管理系统</strong>，专为游戏运营设计。它提供统一的控制面，让你通过 Web 界面管理游戏服务器，而不是直接登录游戏服务器。</p>
<p><strong>核心价值：</strong></p>
<ul>
<li>统一管理多台游戏服务器</li>
<li>安全的权限控制和审批流程</li>
<li>可视化的操作界面</li>
<li>完整的审计日志</li>
</ul>
<h3 id="我需要-croupier-吗" tabindex="-1"><a class="header-anchor" href="#我需要-croupier-吗"><span>我需要 Croupier 吗？</span></a></h3>
<p>如果你的项目符合以下情况，Croupier 适合你：</p>
<ul>
<li>✅ 有多台游戏服务器需要管理</li>
<li>✅ 需要精细的权限控制（GM、运营、客服等不同角色）</li>
<li>✅ 需要记录操作日志，用于审计</li>
<li>✅ 团队多人协作，需要审批流程</li>
<li>✅ 需要通过 Web 界面执行游戏管理操作</li>
</ul>
<h3 id="croupier-和传统-gm-系统有什么区别" tabindex="-1"><a class="header-anchor" href="#croupier-和传统-gm-系统有什么区别"><span>Croupier 和传统 GM 系统有什么区别？</span></a></h3>
<table>
<thead>
<tr>
<th>特性</th>
<th>传统 GM 系统</th>
<th>Croupier</th>
</tr>
</thead>
<tbody>
<tr>
<td>部署方式</td>
<td>内嵌游戏服务器</td>
<td>独立部署</td>
</tr>
<tr>
<td>访问方式</td>
<td>登录服务器命令行</td>
<td>Web 界面</td>
</tr>
<tr>
<td>权限控制</td>
<td>简单的账号密码</td>
<td>RBAC + ABAC + 双人审批</td>
</tr>
<tr>
<td>审计日志</td>
<td>简单文本记录</td>
<td>结构化 + 防篡改链</td>
</tr>
<tr>
<td>扩展性</td>
<td>受限于游戏服务器</td>
<td>独立扩展</td>
</tr>
</tbody>
</table>
<h2 id="快速开始" tabindex="-1"><a class="header-anchor" href="#快速开始"><span>快速开始</span></a></h2>
<h3 id="最快的上手方式是什么" tabindex="-1"><a class="header-anchor" href="#最快的上手方式是什么"><span>最快的上手方式是什么？</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 克隆仓库</span></span>
<span class="line"><span class="token function">git</span> clone <span class="token parameter variable">--recursive</span> https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 一键启动（开发模式）</span></span>
<span class="line"><span class="token function">make</span> dev <span class="token operator">&amp;&amp;</span> ./bin/croupier-server <span class="token parameter variable">--config</span> configs/server.example.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 访问管理界面</span></span>
<span class="line"><span class="token comment"># 浏览器打开 http://localhost:8080</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="必须使用子模块吗" tabindex="-1"><a class="header-anchor" href="#必须使用子模块吗"><span>必须使用子模块吗？</span></a></h3>
<p>是的。Croupier 使用 Git 子模块管理以下组件：</p>
<ul>
<li><code v-pre>proto/</code> - gRPC 协议定义</li>
<li><code v-pre>sdks/</code> - 多语言 SDK</li>
<li><code v-pre>dashboard/</code> - Web 管理界面</li>
</ul>
<p>如果子模块目录为空：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="开发环境需要什么" tabindex="-1"><a class="header-anchor" href="#开发环境需要什么"><span>开发环境需要什么？</span></a></h3>
<p><strong>最低要求：</strong></p>
<ul>
<li>Go 1.25+</li>
<li>buf（Protocol Buffers 工具）</li>
</ul>
<p><strong>完整开发（含 Dashboard）：</strong></p>
<ul>
<li>Node.js 22+</li>
<li>pnpm 10+</li>
</ul>
<h2 id="配置问题" tabindex="-1"><a class="header-anchor" href="#配置问题"><span>配置问题</span></a></h2>
<h3 id="默认端口是什么" tabindex="-1"><a class="header-anchor" href="#默认端口是什么"><span>默认端口是什么？</span></a></h3>
<table>
<thead>
<tr>
<th>组件</th>
<th>端口</th>
<th>协议</th>
</tr>
</thead>
<tbody>
<tr>
<td>Server gRPC</td>
<td>8443</td>
<td>gRPC + mTLS</td>
</tr>
<tr>
<td>Server HTTP</td>
<td>8080</td>
<td>REST API</td>
</tr>
<tr>
<td>Server Metrics</td>
<td>9090</td>
<td>Prometheus</td>
</tr>
<tr>
<td>Agent Local</td>
<td>19090</td>
<td>gRPC</td>
</tr>
<tr>
<td>Agent Metrics</td>
<td>9091</td>
<td>Prometheus</td>
</tr>
</tbody>
</table>
<h3 id="端口被占用怎么办" tabindex="-1"><a class="header-anchor" href="#端口被占用怎么办"><span>端口被占用怎么办？</span></a></h3>
<p>修改配置文件：</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># server.yaml</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":9443"</span>        <span class="token comment"># 修改 gRPC 端口</span></span>
<span class="line">  <span class="token key atrule">http_addr</span><span class="token punctuation">:</span> <span class="token string">":9080"</span>   <span class="token comment"># 修改 HTTP 端口</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="必须配置-tls-证书吗" tabindex="-1"><a class="header-anchor" href="#必须配置-tls-证书吗"><span>必须配置 TLS 证书吗？</span></a></h3>
<p><strong>生产环境：</strong> 必须使用 mTLS</p>
<p><strong>开发环境：</strong> 可以使用项目提供的自签名证书生成脚本：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">./scripts/dev-certs.sh</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>生成的证书位于 <code v-pre>data/dev-certs/</code> 目录。</p>
<h3 id="如何启用-debug-日志" tabindex="-1"><a class="header-anchor" href="#如何启用-debug-日志"><span>如何启用 debug 日志？</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># 配置文件</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> debug  <span class="token comment"># debug | info | warn | error</span></span>
<span class="line">    <span class="token key atrule">format</span><span class="token punctuation">:</span> console  <span class="token comment"># console | json</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数开发" tabindex="-1"><a class="header-anchor" href="#函数开发"><span>函数开发</span></a></h2>
<h3 id="如何创建第一个函数" tabindex="-1"><a class="header-anchor" href="#如何创建第一个函数"><span>如何创建第一个函数？</span></a></h3>
<p>**最简单的方式：**使用示例模板</p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 在你的游戏服务器中</span></span>
<span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier-sdk-go/sdk"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">BanPlayer</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> req BanPlayerRequest<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token operator">*</span>BanPlayerResponse<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 1. 参数验证</span></span>
<span class="line">    <span class="token keyword">if</span> req<span class="token punctuation">.</span>PlayerId <span class="token operator">==</span> <span class="token string">""</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">nil</span><span class="token punctuation">,</span> errors<span class="token punctuation">.</span><span class="token function">New</span><span class="token punctuation">(</span><span class="token string">"player_id is required"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 2. 执行业务逻辑</span></span>
<span class="line">    err <span class="token operator">:=</span> gameServer<span class="token punctuation">.</span><span class="token function">BanPlayer</span><span class="token punctuation">(</span>req<span class="token punctuation">.</span>PlayerId<span class="token punctuation">,</span> req<span class="token punctuation">.</span>Duration<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">nil</span><span class="token punctuation">,</span> err</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 3. 返回结果</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token operator">&amp;</span>BanPlayerResponse<span class="token punctuation">{</span></span>
<span class="line">        BanId<span class="token punctuation">:</span> <span class="token function">generateBanId</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        ExpiresAt<span class="token punctuation">:</span> time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Add</span><span class="token punctuation">(</span>req<span class="token punctuation">.</span>Duration<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 注册函数</span></span>
<span class="line">    sdk<span class="token punctuation">.</span><span class="token function">Register</span><span class="token punctuation">(</span><span class="token string">"player.ban"</span><span class="token punctuation">,</span> BanPlayer<span class="token punctuation">)</span></span>
<span class="line">    sdk<span class="token punctuation">.</span><span class="token function">Start</span><span class="token punctuation">(</span><span class="token string">":19090"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="函数定义在哪里" tabindex="-1"><a class="header-anchor" href="#函数定义在哪里"><span>函数定义在哪里？</span></a></h3>
<p>函数定义在你的<strong>游戏服务器代码</strong>中，通过 SDK 注册到 Croupier Agent。</p>
<p><strong>流程：</strong></p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">游戏服务器代码 → SDK 注册 → Agent → Server → Web UI</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="如何定义函数参数" tabindex="-1"><a class="header-anchor" href="#如何定义函数参数"><span>如何定义函数参数？</span></a></h3>
<p>使用 JSON Schema 定义参数结构：</p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">var</span> PlayerBanSchema <span class="token operator">=</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">    <span class="token string">"type"</span><span class="token punctuation">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"properties"</span><span class="token punctuation">:</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"player_id"</span><span class="token punctuation">:</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">            <span class="token string">"type"</span><span class="token punctuation">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token string">"title"</span><span class="token punctuation">:</span> <span class="token string">"玩家ID"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"duration"</span><span class="token punctuation">:</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">            <span class="token string">"type"</span><span class="token punctuation">:</span> <span class="token string">"integer"</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token string">"title"</span><span class="token punctuation">:</span> <span class="token string">"封禁时长（小时）"</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token string">"minimum"</span><span class="token punctuation">:</span> <span class="token number">1</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token string">"maximum"</span><span class="token punctuation">:</span> <span class="token number">720</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"reason"</span><span class="token punctuation">:</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">            <span class="token string">"type"</span><span class="token punctuation">:</span> <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token string">"title"</span><span class="token punctuation">:</span> <span class="token string">"封禁原因"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"required"</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span><span class="token punctuation">{</span><span class="token string">"player_id"</span><span class="token punctuation">,</span> <span class="token string">"duration"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="权限控制" tabindex="-1"><a class="header-anchor" href="#权限控制"><span>权限控制</span></a></h2>
<h3 id="如何创建角色" tabindex="-1"><a class="header-anchor" href="#如何创建角色"><span>如何创建角色？</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 通过 API 创建角色</span></span>
<span class="line">POST /api/roles</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"role_id"</span><span class="token builtin class-name">:</span> <span class="token string">"gm_level1"</span>,</span>
<span class="line">  <span class="token string">"name"</span><span class="token builtin class-name">:</span> <span class="token string">"初级GM"</span>,</span>
<span class="line">  <span class="token string">"permissions"</span><span class="token builtin class-name">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token string">"player.view"</span>,</span>
<span class="line">    <span class="token string">"player.kick"</span>,</span>
<span class="line">    <span class="token string">"item.view"</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="什么是双人审批" tabindex="-1"><a class="header-anchor" href="#什么是双人审批"><span>什么是双人审批？</span></a></h3>
<p>高风险操作（如永久封禁）需要<strong>两人审批</strong>才能执行：</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># 函数定义中配置</span></span>
<span class="line"><span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">two_person_rule</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">approval</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">threshold</span><span class="token punctuation">:</span> <span class="token number">2</span>  <span class="token comment"># 需要两人审批</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="如何配置权限" tabindex="-1"><a class="header-anchor" href="#如何配置权限"><span>如何配置权限？</span></a></h3>
<p>权限格式为 <code v-pre>&lt;资源&gt;.&lt;操作&gt;</code>：</p>
<ul>
<li><code v-pre>player.view</code> - 查看玩家信息</li>
<li><code v-pre>player.ban</code> - 封禁玩家</li>
<li><code v-pre>item.*</code> - 所有道具操作</li>
<li><code v-pre>*.*</code> - 超级管理员</li>
</ul>
<h2 id="运维问题" tabindex="-1"><a class="header-anchor" href="#运维问题"><span>运维问题</span></a></h2>
<h3 id="如何优雅关闭" tabindex="-1"><a class="header-anchor" href="#如何优雅关闭"><span>如何优雅关闭？</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 发送 SIGTERM 信号</span></span>
<span class="line"><span class="token function">kill</span> <span class="token parameter variable">-TERM</span> <span class="token variable"><span class="token variable">$(</span>pgrep croupier-server<span class="token variable">)</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Server 会：</span></span>
<span class="line"><span class="token comment"># 1. 停止接受新请求</span></span>
<span class="line"><span class="token comment"># 2. 等待现有请求完成（最多 30 秒）</span></span>
<span class="line"><span class="token comment"># 3. 关闭数据库连接</span></span>
<span class="line"><span class="token comment"># 4. 退出</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="如何备份数据" tabindex="-1"><a class="header-anchor" href="#如何备份数据"><span>如何备份数据？</span></a></h3>
<p><strong>数据库备份：</strong></p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># PostgreSQL</span></span>
<span class="line">pg_dump <span class="token parameter variable">-U</span> croupier croupier <span class="token operator">></span> backup_<span class="token variable"><span class="token variable">$(</span><span class="token function">date</span> +%Y%m%d<span class="token variable">)</span></span>.sql</span>
<span class="line"></span>
<span class="line"><span class="token comment"># SQLite</span></span>
<span class="line"><span class="token function">cp</span> data/croupier.db backup/croupier_<span class="token variable"><span class="token variable">$(</span><span class="token function">date</span> +%Y%m%d<span class="token variable">)</span></span>.db</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>配置备份：</strong></p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">tar</span> <span class="token parameter variable">-czf</span> config_backup_<span class="token variable"><span class="token variable">$(</span><span class="token function">date</span> +%Y%m%d<span class="token variable">)</span></span>.tar.gz configs/</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="如何升级版本" tabindex="-1"><a class="header-anchor" href="#如何升级版本"><span>如何升级版本？</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 备份数据</span></span>
<span class="line">./scripts/backup.sh</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 停止服务</span></span>
<span class="line">systemctl stop croupier-server</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 更新代码</span></span>
<span class="line"><span class="token function">git</span> pull origin main</span>
<span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 重新构建</span></span>
<span class="line"><span class="token function">make</span> build</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 5. 启动服务</span></span>
<span class="line">systemctl start croupier-server</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="故障排查" tabindex="-1"><a class="header-anchor" href="#故障排查"><span>故障排查</span></a></h2>
<h3 id="server-启动失败怎么办" tabindex="-1"><a class="header-anchor" href="#server-启动失败怎么办"><span>Server 启动失败怎么办？</span></a></h3>
<ol>
<li><strong>检查配置文件语法</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">./croupier config <span class="token builtin class-name">test</span> <span class="token parameter variable">--config</span> configs/server.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><ol start="2">
<li><strong>检查端口占用</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">lsof</span> <span class="token parameter variable">-i</span> :8443</span>
<span class="line"><span class="token function">lsof</span> <span class="token parameter variable">-i</span> :8080</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li><strong>查看错误日志</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">tail</span> <span class="token parameter variable">-f</span> logs/server.log</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="agent-连不上-server" tabindex="-1"><a class="header-anchor" href="#agent-连不上-server"><span>Agent 连不上 Server？</span></a></h3>
<p><strong>检查清单：</strong></p>
<ul>
<li>[ ] Server 正在运行</li>
<li>[ ] 网络连通（<code v-pre>telnet server 8443</code>）</li>
<li>[ ] TLS 证书正确</li>
<li>[ ] Agent 配置中的 server_addr 正确</li>
</ul>
<h3 id="函数调用超时怎么办" tabindex="-1"><a class="header-anchor" href="#函数调用超时怎么办"><span>函数调用超时怎么办？</span></a></h3>
<p><strong>可能原因：</strong></p>
<ol>
<li>游戏服务器处理慢</li>
<li>网络延迟高</li>
<li>Agent 与游戏服务器连接问题</li>
</ol>
<p><strong>解决方法：</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"data.export"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"options"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"timeout"</span><span class="token operator">:</span> <span class="token number">300</span>  <span class="token comment">// 增加超时到 5 分钟</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>或使用异步调用：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">POST /api/jobs</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="性能优化" tabindex="-1"><a class="header-anchor" href="#性能优化"><span>性能优化</span></a></h2>
<h3 id="支持多少并发" tabindex="-1"><a class="header-anchor" href="#支持多少并发"><span>支持多少并发？</span></a></h3>
<p><strong>默认配置：</strong></p>
<ul>
<li>Server: 1000 并发连接</li>
<li>每连接: 100 并发请求</li>
</ul>
<p><strong>优化方法：</strong></p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">grpc</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">max_concurrent_streams</span><span class="token punctuation">:</span> <span class="token number">1000</span></span>
<span class="line">  <span class="token key atrule">http</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">max_connections</span><span class="token punctuation">:</span> <span class="token number">5000</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="如何减少内存占用" tabindex="-1"><a class="header-anchor" href="#如何减少内存占用"><span>如何减少内存占用？</span></a></h3>
<ol>
<li><strong>调整日志级别</strong>：生产环境使用 <code v-pre>warn</code> 或 <code v-pre>error</code></li>
<li><strong>限制日志文件大小</strong></li>
<li><strong>启用日志压缩</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> warn</span>
<span class="line">    <span class="token key atrule">max_size</span><span class="token punctuation">:</span> <span class="token number">100</span>  <span class="token comment"># MB</span></span>
<span class="line">    <span class="token key atrule">max_backups</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">    <span class="token key atrule">compress</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="集成问题" tabindex="-1"><a class="header-anchor" href="#集成问题"><span>集成问题</span></a></h2>
<h3 id="如何与现有系统集成" tabindex="-1"><a class="header-anchor" href="#如何与现有系统集成"><span>如何与现有系统集成？</span></a></h3>
<p><strong>REST API 集成：</strong></p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 调用函数</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://server:8080/api/invoke <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Authorization: Bearer <span class="token variable">$TOKEN</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Game-ID: my-game"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Env: prod"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{"function_id":"player.ban","payload":{...}}'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="如何对接现有权限系统" tabindex="-1"><a class="header-anchor" href="#如何对接现有权限系统"><span>如何对接现有权限系统？</span></a></h3>
<p>通过 <strong>Webhook</strong> 扩展：</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">webhook</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">url</span><span class="token punctuation">:</span> <span class="token string">"https://your-auth-system.com/validate"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="更多帮助" tabindex="-1"><a class="header-anchor" href="#更多帮助"><span>更多帮助</span></a></h2>
<ul>
<li>📖 <a href="/" target="_blank" rel="noopener noreferrer">完整文档</a></li>
<li>💬 <a href="https://github.com/cuihairu/croupier/discussions" target="_blank" rel="noopener noreferrer">GitHub Discussions</a></li>
<li>🐛 <a href="https://github.com/cuihairu/croupier/issues" target="_blank" rel="noopener noreferrer">报告问题</a></li>
</ul>
</div></template>


