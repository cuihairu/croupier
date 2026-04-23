<template><div><h1 id="🔥-croupier-sdk-热更新最佳实践指南" tabindex="-1"><a class="header-anchor" href="#🔥-croupier-sdk-热更新最佳实践指南"><span>🔥 Croupier SDK 热更新最佳实践指南</span></a></h1>
<h2 id="📋-概述" tabindex="-1"><a class="header-anchor" href="#📋-概述"><span>📋 概述</span></a></h2>
<p>本指南提供了在不同开发环境和生产环境中使用Croupier SDK热更新功能的最佳实践和建议。</p>
<h2 id="🎯-核心理念" tabindex="-1"><a class="header-anchor" href="#🎯-核心理念"><span>🎯 核心理念</span></a></h2>
<h3 id="分离关注点" tabindex="-1"><a class="header-anchor" href="#分离关注点"><span><strong>分离关注点</strong></span></a></h3>
<ul>
<li><strong>SDK负责连接管理</strong>：自动重连、函数注册、状态监控</li>
<li><strong>工具负责代码热更新</strong>：Air、Nodemon、PM2等负责代码变更检测</li>
<li><strong>开发者负责业务逻辑</strong>：专注游戏功能实现，无需关心底层重载机制</li>
</ul>
<h3 id="渐进式集成" tabindex="-1"><a class="header-anchor" href="#渐进式集成"><span><strong>渐进式集成</strong></span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">基础连接 → 自动重连 → 文件监听 → 函数热重载 → 生产部署</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="🔧-开发环境最佳实践" tabindex="-1"><a class="header-anchor" href="#🔧-开发环境最佳实践"><span>🔧 开发环境最佳实践</span></a></h2>
<h3 id="go开发环境" tabindex="-1"><a class="header-anchor" href="#go开发环境"><span><strong>Go开发环境</strong></span></a></h3>
<h4 id="推荐配置" tabindex="-1"><a class="header-anchor" href="#推荐配置"><span>推荐配置</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># croupier-hotreload.yaml</span></span>
<span class="line"><span class="token key atrule">hotreload</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">auto_reconnect</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">reconnect_delay</span><span class="token punctuation">:</span> 3s</span>
<span class="line">  <span class="token key atrule">max_retry_attempts</span><span class="token punctuation">:</span> <span class="token number">5</span></span>
<span class="line">  <span class="token key atrule">health_check_interval</span><span class="token punctuation">:</span> 10s</span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">tools</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">air</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">plugin</span><span class="token punctuation">:</span> <span class="token boolean important">false</span>  <span class="token comment"># 开发环境关闭复杂特性</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="开发工作流" tabindex="-1"><a class="header-anchor" href="#开发工作流"><span>开发工作流</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 启动Agent</span></span>
<span class="line">./croupier-agent <span class="token parameter variable">--config</span> configs/agent.example.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 启动Air热重载开发</span></span>
<span class="line"><span class="token builtin class-name">cd</span> examples/go-hotreload</span>
<span class="line">air</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 修改代码 -> Air自动重编译 -> SDK自动重连 -> 继续开发</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="air最佳配置" tabindex="-1"><a class="header-anchor" href="#air最佳配置"><span>Air最佳配置</span></a></h4>
<div class="language-toml line-numbers-mode" data-highlighter="prismjs" data-ext="toml"><pre v-pre><code class="language-toml"><span class="line"><span class="token comment"># .air.toml</span></span>
<span class="line"><span class="token punctuation">[</span><span class="token table class-name">build</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token key property">cmd</span> <span class="token punctuation">=</span> <span class="token string">"go build -o ./tmp/main ."</span></span>
<span class="line"><span class="token key property">delay</span> <span class="token punctuation">=</span> <span class="token number">1000</span>              <span class="token comment"># 1秒防抖，避免频繁触发</span></span>
<span class="line"><span class="token key property">exclude_regex</span> <span class="token punctuation">=</span> <span class="token punctuation">[</span><span class="token string">"_test.go"</span><span class="token punctuation">]</span>  <span class="token comment"># 排除测试文件</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">[</span><span class="token table class-name">log</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token key property">main_only</span> <span class="token punctuation">=</span> <span class="token boolean">true</span>          <span class="token comment"># 只显示主程序日志，减少噪音</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">[</span><span class="token table class-name">misc</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token key property">clean_on_exit</span> <span class="token punctuation">=</span> <span class="token boolean">true</span>      <span class="token comment"># 自动清理临时文件</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="node-js开发环境" tabindex="-1"><a class="header-anchor" href="#node-js开发环境"><span><strong>Node.js开发环境</strong></span></a></h3>
<h4 id="推荐配置-1" tabindex="-1"><a class="header-anchor" href="#推荐配置-1"><span>推荐配置</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"hotReload"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"autoReconnect"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"reconnectDelay"</span><span class="token operator">:</span> <span class="token number">3000</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"tools"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"nodemon"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"moduleReload"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"pm2"</span><span class="token operator">:</span> <span class="token boolean">false</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"fileWatching"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"watchDir"</span><span class="token operator">:</span> <span class="token string">"./functions"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"patterns"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"*.js"</span><span class="token punctuation">,</span> <span class="token string">"*.json"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="开发工作流-1" tabindex="-1"><a class="header-anchor" href="#开发工作流-1"><span>开发工作流</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 启动Agent</span></span>
<span class="line">./croupier-agent <span class="token parameter variable">--config</span> configs/agent.example.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 启动Nodemon热重载</span></span>
<span class="line"><span class="token builtin class-name">cd</span> examples/js-hotreload</span>
<span class="line"><span class="token function">npm</span> run dev</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 修改代码 -> Nodemon重启进程 -> SDK自动重连 -> 继续开发</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="nodemon最佳配置" tabindex="-1"><a class="header-anchor" href="#nodemon最佳配置"><span>Nodemon最佳配置</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"watch"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"src/"</span><span class="token punctuation">,</span> <span class="token string">"functions/"</span><span class="token punctuation">,</span> <span class="token string">"config/"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ext"</span><span class="token operator">:</span> <span class="token string">"js,json,yaml"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"delay"</span><span class="token operator">:</span> <span class="token string">"2000"</span><span class="token punctuation">,</span>          # <span class="token number">2</span>秒防抖</span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"NODE_ENV"</span><span class="token operator">:</span> <span class="token string">"development"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"LOG_LEVEL"</span><span class="token operator">:</span> <span class="token string">"debug"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"ignore"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"tmp/"</span><span class="token punctuation">,</span> <span class="token string">"logs/"</span><span class="token punctuation">,</span> <span class="token string">"*.tmp"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🚀-生产环境最佳实践" tabindex="-1"><a class="header-anchor" href="#🚀-生产环境最佳实践"><span>🚀 生产环境最佳实践</span></a></h2>
<h3 id="go生产环境" tabindex="-1"><a class="header-anchor" href="#go生产环境"><span><strong>Go生产环境</strong></span></a></h3>
<h4 id="配置策略" tabindex="-1"><a class="header-anchor" href="#配置策略"><span>配置策略</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># 生产环境配置</span></span>
<span class="line"><span class="token key atrule">hotreload</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">auto_reconnect</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">reconnect_delay</span><span class="token punctuation">:</span> 10s        <span class="token comment"># 生产环境延长间隔</span></span>
<span class="line">  <span class="token key atrule">max_retry_attempts</span><span class="token punctuation">:</span> <span class="token number">20</span>      <span class="token comment"># 增加重试次数</span></span>
<span class="line">  <span class="token key atrule">health_check_interval</span><span class="token punctuation">:</span> 60s  <span class="token comment"># 降低检查频率</span></span>
<span class="line">  <span class="token key atrule">graceful_shutdown_timeout</span><span class="token punctuation">:</span> 60s  <span class="token comment"># 更长的关闭时间</span></span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">tools</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">air</span><span class="token punctuation">:</span> <span class="token boolean important">false</span>              <span class="token comment"># 关闭开发工具</span></span>
<span class="line">    <span class="token key atrule">plugin</span><span class="token punctuation">:</span> <span class="token boolean important">true</span>            <span class="token comment"># 启用Go Plugin热更新</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="部署策略" tabindex="-1"><a class="header-anchor" href="#部署策略"><span>部署策略</span></a></h4>
<ol>
<li><strong>蓝绿部署</strong> - 推荐用于大版本更新</li>
<li><strong>滚动更新</strong> - 用于小版本补丁</li>
<li><strong>插件热更新</strong> - 用于紧急修复</li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 插件热更新部署</span></span>
<span class="line">go build <span class="token parameter variable">-buildmode</span><span class="token operator">=</span>plugin <span class="token parameter variable">-o</span> player_ban.so ./plugins/player_ban</span>
<span class="line"><span class="token comment"># 通过管理界面上传插件</span></span>
<span class="line"><span class="token comment"># SDK自动加载新插件</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="node-js生产环境" tabindex="-1"><a class="header-anchor" href="#node-js生产环境"><span><strong>Node.js生产环境</strong></span></a></h3>
<h4 id="pm2集群配置" tabindex="-1"><a class="header-anchor" href="#pm2集群配置"><span>PM2集群配置</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"apps"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"croupier-game"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"script"</span><span class="token operator">:</span> <span class="token string">"main.js"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"instances"</span><span class="token operator">:</span> <span class="token string">"max"</span><span class="token punctuation">,</span>           # 使用所有CPU核心</span>
<span class="line">    <span class="token property">"exec_mode"</span><span class="token operator">:</span> <span class="token string">"cluster"</span><span class="token punctuation">,</span>       # 集群模式</span>
<span class="line">    <span class="token property">"watch"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span>               # 生产环境关闭文件监听</span>
<span class="line">    <span class="token property">"autorestart"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"max_restarts"</span><span class="token operator">:</span> <span class="token number">10</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"min_uptime"</span><span class="token operator">:</span> <span class="token string">"10s"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"env"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"NODE_ENV"</span><span class="token operator">:</span> <span class="token string">"production"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"CROUPIER_AUTO_RECONNECT"</span><span class="token operator">:</span> <span class="token string">"true"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"CROUPIER_RECONNECT_DELAY"</span><span class="token operator">:</span> <span class="token string">"15000"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="零停机部署" tabindex="-1"><a class="header-anchor" href="#零停机部署"><span>零停机部署</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 验证新代码</span></span>
<span class="line"><span class="token function">npm</span> <span class="token builtin class-name">test</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 零停机重载</span></span>
<span class="line">pm2 reload croupier-game</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 验证服务状态</span></span>
<span class="line">pm2 status</span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/health</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📊-监控和观测最佳实践" tabindex="-1"><a class="header-anchor" href="#📊-监控和观测最佳实践"><span>📊 监控和观测最佳实践</span></a></h2>
<h3 id="关键指标监控" tabindex="-1"><a class="header-anchor" href="#关键指标监控"><span><strong>关键指标监控</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// Go SDK监控指标</span></span>
<span class="line"><span class="token keyword">type</span> HotReloadMetrics <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 连接健康</span></span>
<span class="line">    ConnectionStatus     <span class="token builtin">string</span>    <span class="token string">`json:"connection_status"`</span></span>
<span class="line">    LastReconnectTime   time<span class="token punctuation">.</span>Time <span class="token string">`json:"last_reconnect_time"`</span></span>
<span class="line">    ReconnectCount      <span class="token builtin">int64</span>     <span class="token string">`json:"reconnect_count"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 重载性能</span></span>
<span class="line">    FunctionReloads     <span class="token builtin">int64</span>     <span class="token string">`json:"function_reloads"`</span></span>
<span class="line">    AvgReloadTime       time<span class="token punctuation">.</span>Duration <span class="token string">`json:"avg_reload_time"`</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 错误追踪</span></span>
<span class="line">    FailedReloads       <span class="token builtin">int64</span>     <span class="token string">`json:"failed_reloads"`</span></span>
<span class="line">    LastError           <span class="token builtin">string</span>    <span class="token string">`json:"last_error"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="告警规则" tabindex="-1"><a class="header-anchor" href="#告警规则"><span><strong>告警规则</strong></span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># Prometheus告警规则</span></span>
<span class="line"><span class="token punctuation">-</span> <span class="token key atrule">alert</span><span class="token punctuation">:</span> CroupierReconnectHigh</span>
<span class="line">  <span class="token key atrule">expr</span><span class="token punctuation">:</span> increase(croupier_reconnect_count<span class="token punctuation">[</span>5m<span class="token punctuation">]</span>) <span class="token punctuation">></span> 3</span>
<span class="line">  <span class="token key atrule">for</span><span class="token punctuation">:</span> 2m</span>
<span class="line">  <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">summary</span><span class="token punctuation">:</span> <span class="token string">"Croupier客户端频繁重连"</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">-</span> <span class="token key atrule">alert</span><span class="token punctuation">:</span> CroupierReloadFailed</span>
<span class="line">  <span class="token key atrule">expr</span><span class="token punctuation">:</span> increase(croupier_reload_failed_count<span class="token punctuation">[</span>5m<span class="token punctuation">]</span>) <span class="token punctuation">></span> 1</span>
<span class="line">  <span class="token key atrule">for</span><span class="token punctuation">:</span> 1m</span>
<span class="line">  <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">summary</span><span class="token punctuation">:</span> <span class="token string">"Croupier热重载失败"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="日志最佳实践" tabindex="-1"><a class="header-anchor" href="#日志最佳实践"><span><strong>日志最佳实践</strong></span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"level"</span><span class="token operator">:</span> <span class="token string">"info"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"timestamp"</span><span class="token operator">:</span> <span class="token string">"2024-01-15T10:30:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"component"</span><span class="token operator">:</span> <span class="token string">"hotreload"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"event"</span><span class="token operator">:</span> <span class="token string">"function_reloaded"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"old_version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"new_version"</span><span class="token operator">:</span> <span class="token string">"1.1.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"reload_duration_ms"</span><span class="token operator">:</span> <span class="token number">150</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"trace_id"</span><span class="token operator">:</span> <span class="token string">"abc123"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔐-安全最佳实践" tabindex="-1"><a class="header-anchor" href="#🔐-安全最佳实践"><span>🔐 安全最佳实践</span></a></h2>
<h3 id="访问控制" tabindex="-1"><a class="header-anchor" href="#访问控制"><span><strong>访问控制</strong></span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># RBAC配置示例</span></span>
<span class="line"><span class="token key atrule">permissions</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">hotreload.function.reload</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"允许重载函数"</span></span>
<span class="line">    <span class="token key atrule">scopes</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"dev"</span><span class="token punctuation">,</span> <span class="token string">"staging"</span><span class="token punctuation">]</span>  <span class="token comment"># 仅开发和测试环境</span></span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">hotreload.config.reload</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"允许重载配置"</span></span>
<span class="line">    <span class="token key atrule">requires_approval</span><span class="token punctuation">:</span> <span class="token boolean important">true</span>     <span class="token comment"># 需要审批</span></span>
<span class="line">    <span class="token key atrule">scopes</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"production"</span><span class="token punctuation">]</span>      <span class="token comment"># 生产环境</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="代码验证" tabindex="-1"><a class="header-anchor" href="#代码验证"><span><strong>代码验证</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 函数重载前的安全检查</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">validateFunctionReload</span><span class="token punctuation">(</span>functionID <span class="token builtin">string</span><span class="token punctuation">,</span> newCode <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">byte</span><span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 1. 语法检查</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> <span class="token function">validateSyntax</span><span class="token punctuation">(</span>newCode<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"syntax error: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 2. 安全扫描</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> <span class="token function">scanForVulnerabilities</span><span class="token punctuation">(</span>newCode<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"security issue: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 3. 签名验证</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> <span class="token function">verifyCodeSignature</span><span class="token punctuation">(</span>newCode<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"invalid signature: %w"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审计日志" tabindex="-1"><a class="header-anchor" href="#审计日志"><span><strong>审计日志</strong></span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"event"</span><span class="token operator">:</span> <span class="token string">"hotreload_request"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"actor"</span><span class="token operator">:</span> <span class="token string">"admin@company.com"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"change_type"</span><span class="token operator">:</span> <span class="token string">"function_update"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"approval_status"</span><span class="token operator">:</span> <span class="token string">"approved"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"approver"</span><span class="token operator">:</span> <span class="token string">"manager@company.com"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"risk_level"</span><span class="token operator">:</span> <span class="token string">"medium"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"timestamp"</span><span class="token operator">:</span> <span class="token string">"2024-01-15T10:30:00Z"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🎯-性能优化最佳实践" tabindex="-1"><a class="header-anchor" href="#🎯-性能优化最佳实践"><span>🎯 性能优化最佳实践</span></a></h2>
<h3 id="重连优化" tabindex="-1"><a class="header-anchor" href="#重连优化"><span><strong>重连优化</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 智能重连策略</span></span>
<span class="line"><span class="token keyword">type</span> ReconnectStrategy <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    baseDelay    time<span class="token punctuation">.</span>Duration</span>
<span class="line">    maxDelay     time<span class="token punctuation">.</span>Duration</span>
<span class="line">    multiplier   <span class="token builtin">float64</span></span>
<span class="line">    jitter       <span class="token builtin">bool</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>s <span class="token operator">*</span>ReconnectStrategy<span class="token punctuation">)</span> <span class="token function">nextDelay</span><span class="token punctuation">(</span>attempt <span class="token builtin">int</span><span class="token punctuation">)</span> time<span class="token punctuation">.</span>Duration <span class="token punctuation">{</span></span>
<span class="line">    delay <span class="token operator">:=</span> <span class="token function">float64</span><span class="token punctuation">(</span>s<span class="token punctuation">.</span>baseDelay<span class="token punctuation">)</span> <span class="token operator">*</span> math<span class="token punctuation">.</span><span class="token function">Pow</span><span class="token punctuation">(</span>s<span class="token punctuation">.</span>multiplier<span class="token punctuation">,</span> <span class="token function">float64</span><span class="token punctuation">(</span>attempt<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> delay <span class="token operator">></span> <span class="token function">float64</span><span class="token punctuation">(</span>s<span class="token punctuation">.</span>maxDelay<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        delay <span class="token operator">=</span> <span class="token function">float64</span><span class="token punctuation">(</span>s<span class="token punctuation">.</span>maxDelay<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> s<span class="token punctuation">.</span>jitter <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// 添加±25%的随机偏移，避免惊群效应</span></span>
<span class="line">        jitterRange <span class="token operator">:=</span> delay <span class="token operator">*</span> <span class="token number">0.25</span></span>
<span class="line">        delay <span class="token operator">+=</span> <span class="token punctuation">(</span>rand<span class="token punctuation">.</span><span class="token function">Float64</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token operator">*</span><span class="token number">2</span> <span class="token operator">-</span> <span class="token number">1</span><span class="token punctuation">)</span> <span class="token operator">*</span> jitterRange</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> time<span class="token punctuation">.</span><span class="token function">Duration</span><span class="token punctuation">(</span>delay<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="批量操作优化" tabindex="-1"><a class="header-anchor" href="#批量操作优化"><span><strong>批量操作优化</strong></span></a></h3>
<div class="language-javascript line-numbers-mode" data-highlighter="prismjs" data-ext="js"><pre v-pre><code class="language-javascript"><span class="line"><span class="token comment">// Node.js批量重载优化</span></span>
<span class="line"><span class="token keyword">class</span> <span class="token class-name">BatchReloader</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token function">constructor</span><span class="token punctuation">(</span><span class="token parameter">client</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">this</span><span class="token punctuation">.</span>client <span class="token operator">=</span> client<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">this</span><span class="token punctuation">.</span>pendingReloads <span class="token operator">=</span> <span class="token keyword">new</span> <span class="token class-name">Map</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">this</span><span class="token punctuation">.</span>batchTimeout <span class="token operator">=</span> <span class="token number">5000</span><span class="token punctuation">;</span> <span class="token comment">// 5秒批量窗口</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">  <span class="token keyword">async</span> <span class="token function">reloadFunction</span><span class="token punctuation">(</span><span class="token parameter">functionId<span class="token punctuation">,</span> descriptor<span class="token punctuation">,</span> handler</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 收集批量操作</span></span>
<span class="line">    <span class="token keyword">this</span><span class="token punctuation">.</span>pendingReloads<span class="token punctuation">.</span><span class="token function">set</span><span class="token punctuation">(</span>functionId<span class="token punctuation">,</span> <span class="token punctuation">{</span> descriptor<span class="token punctuation">,</span> handler <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 延迟执行，允许更多操作加入批次</span></span>
<span class="line">    <span class="token function">clearTimeout</span><span class="token punctuation">(</span><span class="token keyword">this</span><span class="token punctuation">.</span>batchTimer<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">this</span><span class="token punctuation">.</span>batchTimer <span class="token operator">=</span> <span class="token function">setTimeout</span><span class="token punctuation">(</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">=></span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token keyword">this</span><span class="token punctuation">.</span><span class="token function">executeBatchReload</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token keyword">this</span><span class="token punctuation">.</span>batchTimeout<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">  <span class="token keyword">async</span> <span class="token function">executeBatchReload</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token keyword">this</span><span class="token punctuation">.</span>pendingReloads<span class="token punctuation">.</span>size <span class="token operator">===</span> <span class="token number">0</span><span class="token punctuation">)</span> <span class="token keyword">return</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">const</span> functions <span class="token operator">=</span> Object<span class="token punctuation">.</span><span class="token function">fromEntries</span><span class="token punctuation">(</span><span class="token keyword">this</span><span class="token punctuation">.</span>pendingReloads<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">this</span><span class="token punctuation">.</span>pendingReloads<span class="token punctuation">.</span><span class="token function">clear</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">await</span> <span class="token keyword">this</span><span class="token punctuation">.</span>client<span class="token punctuation">.</span><span class="token function">reloadFunctions</span><span class="token punctuation">(</span>functions<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🧪-测试最佳实践" tabindex="-1"><a class="header-anchor" href="#🧪-测试最佳实践"><span>🧪 测试最佳实践</span></a></h2>
<h3 id="单元测试" tabindex="-1"><a class="header-anchor" href="#单元测试"><span><strong>单元测试</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">func</span> <span class="token function">TestHotReload</span><span class="token punctuation">(</span>t <span class="token operator">*</span>testing<span class="token punctuation">.</span>T<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 创建测试客户端</span></span>
<span class="line">    client <span class="token operator">:=</span> <span class="token function">NewTestClient</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册初始函数</span></span>
<span class="line">    desc1 <span class="token operator">:=</span> FunctionDescriptor<span class="token punctuation">{</span>ID<span class="token punctuation">:</span> <span class="token string">"test.func"</span><span class="token punctuation">,</span> Version<span class="token punctuation">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">}</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>desc1<span class="token punctuation">,</span> handler1<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 测试函数重载</span></span>
<span class="line">    desc2 <span class="token operator">:=</span> FunctionDescriptor<span class="token punctuation">{</span>ID<span class="token punctuation">:</span> <span class="token string">"test.func"</span><span class="token punctuation">,</span> Version<span class="token punctuation">:</span> <span class="token string">"1.1.0"</span><span class="token punctuation">}</span></span>
<span class="line">    err <span class="token operator">:=</span> client<span class="token punctuation">.</span><span class="token function">ReloadFunction</span><span class="token punctuation">(</span><span class="token string">"test.func"</span><span class="token punctuation">,</span> desc2<span class="token punctuation">,</span> handler2<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    assert<span class="token punctuation">.</span><span class="token function">NoError</span><span class="token punctuation">(</span>t<span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    assert<span class="token punctuation">.</span><span class="token function">Equal</span><span class="token punctuation">(</span>t<span class="token punctuation">,</span> <span class="token string">"1.1.0"</span><span class="token punctuation">,</span> client<span class="token punctuation">.</span><span class="token function">GetFunctionVersion</span><span class="token punctuation">(</span><span class="token string">"test.func"</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="集成测试" tabindex="-1"><a class="header-anchor" href="#集成测试"><span><strong>集成测试</strong></span></a></h3>
<div class="language-javascript line-numbers-mode" data-highlighter="prismjs" data-ext="js"><pre v-pre><code class="language-javascript"><span class="line"><span class="token comment">// 端到端热重载测试</span></span>
<span class="line"><span class="token function">describe</span><span class="token punctuation">(</span><span class="token string">'Hot Reload Integration'</span><span class="token punctuation">,</span> <span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">=></span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">let</span> agent<span class="token punctuation">,</span> client<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token function">beforeEach</span><span class="token punctuation">(</span><span class="token keyword">async</span> <span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">=></span> <span class="token punctuation">{</span></span>
<span class="line">    agent <span class="token operator">=</span> <span class="token keyword">await</span> <span class="token function">startTestAgent</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client <span class="token operator">=</span> <span class="token keyword">new</span> <span class="token class-name">HotReloadableClient</span><span class="token punctuation">(</span>testConfig<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">await</span> client<span class="token punctuation">.</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token function">it</span><span class="token punctuation">(</span><span class="token string">'should handle function reload with agent restart'</span><span class="token punctuation">,</span> <span class="token keyword">async</span> <span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">=></span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 注册初始函数</span></span>
<span class="line">    <span class="token keyword">await</span> client<span class="token punctuation">.</span><span class="token function">registerFunction</span><span class="token punctuation">(</span>descriptor1<span class="token punctuation">,</span> handler1<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 模拟Agent重启</span></span>
<span class="line">    <span class="token keyword">await</span> agent<span class="token punctuation">.</span><span class="token function">stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">await</span> agent<span class="token punctuation">.</span><span class="token function">start</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 验证自动重连和函数重新注册</span></span>
<span class="line">    <span class="token keyword">await</span> <span class="token function">waitFor</span><span class="token punctuation">(</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">=></span> client<span class="token punctuation">.</span>isConnected<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token function">expect</span><span class="token punctuation">(</span>client<span class="token punctuation">.</span>functions<span class="token punctuation">.</span>size<span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">toBe</span><span class="token punctuation">(</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🚨-故障处理最佳实践" tabindex="-1"><a class="header-anchor" href="#🚨-故障处理最佳实践"><span>🚨 故障处理最佳实践</span></a></h2>
<h3 id="常见问题诊断" tabindex="-1"><a class="header-anchor" href="#常见问题诊断"><span><strong>常见问题诊断</strong></span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token shebang important">#!/bin/bash</span></span>
<span class="line"><span class="token comment"># 热重载健康检查脚本</span></span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"🔍 Croupier热重载健康检查"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 1. 检查Agent连接</span></span>
<span class="line"><span class="token keyword">if</span> <span class="token function">curl</span> <span class="token parameter variable">-f</span> http://localhost:19091/health<span class="token punctuation">;</span> <span class="token keyword">then</span></span>
<span class="line">    <span class="token builtin class-name">echo</span> <span class="token string">"✅ Agent健康"</span></span>
<span class="line"><span class="token keyword">else</span></span>
<span class="line">    <span class="token builtin class-name">echo</span> <span class="token string">"❌ Agent连接失败"</span></span>
<span class="line">    <span class="token builtin class-name">exit</span> <span class="token number">1</span></span>
<span class="line"><span class="token keyword">fi</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 检查函数注册</span></span>
<span class="line"><span class="token assign-left variable">FUNC_COUNT</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span><span class="token function">curl</span> <span class="token parameter variable">-s</span> http://localhost:19091/functions <span class="token operator">|</span> jq length<span class="token variable">)</span></span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"📋 注册函数数量: <span class="token variable">$FUNC_COUNT</span>"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 检查重载状态</span></span>
<span class="line"><span class="token assign-left variable">RELOAD_STATUS</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span><span class="token function">curl</span> <span class="token parameter variable">-s</span> http://localhost:19091/hotreload/status<span class="token variable">)</span></span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"🔄 重载状态: <span class="token variable">$RELOAD_STATUS</span>"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 检查错误日志</span></span>
<span class="line"><span class="token assign-left variable">ERROR_COUNT</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span><span class="token function">grep</span> <span class="token parameter variable">-c</span> <span class="token string">"ERROR.*hotreload"</span> /var/log/croupier.log <span class="token operator">||</span> <span class="token builtin class-name">echo</span> <span class="token number">0</span><span class="token variable">)</span></span></span>
<span class="line"><span class="token keyword">if</span> <span class="token punctuation">[</span> <span class="token variable">$ERROR_COUNT</span> <span class="token parameter variable">-gt</span> <span class="token number">0</span> <span class="token punctuation">]</span><span class="token punctuation">;</span> <span class="token keyword">then</span></span>
<span class="line">    <span class="token builtin class-name">echo</span> <span class="token string">"⚠️ 发现 <span class="token variable">$ERROR_COUNT</span> 个热重载错误"</span></span>
<span class="line">    <span class="token function">grep</span> <span class="token string">"ERROR.*hotreload"</span> /var/log/croupier.log <span class="token operator">|</span> <span class="token function">tail</span> <span class="token parameter variable">-5</span></span>
<span class="line"><span class="token keyword">fi</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="回滚策略" tabindex="-1"><a class="header-anchor" href="#回滚策略"><span><strong>回滚策略</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 自动回滚机制</span></span>
<span class="line"><span class="token keyword">type</span> RollbackManager <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    versions <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token punctuation">[</span><span class="token punctuation">]</span>FunctionVersion</span>
<span class="line">    maxHistory <span class="token builtin">int</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>rm <span class="token operator">*</span>RollbackManager<span class="token punctuation">)</span> <span class="token function">rollback</span><span class="token punctuation">(</span>functionID <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    versions <span class="token operator">:=</span> rm<span class="token punctuation">.</span>versions<span class="token punctuation">[</span>functionID<span class="token punctuation">]</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">len</span><span class="token punctuation">(</span>versions<span class="token punctuation">)</span> <span class="token operator">&lt;</span> <span class="token number">2</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"no previous version to rollback to"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 回滚到上一个版本</span></span>
<span class="line">    previousVersion <span class="token operator">:=</span> versions<span class="token punctuation">[</span><span class="token function">len</span><span class="token punctuation">(</span>versions<span class="token punctuation">)</span><span class="token operator">-</span><span class="token number">2</span><span class="token punctuation">]</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> rm<span class="token punctuation">.</span><span class="token function">reloadFunction</span><span class="token punctuation">(</span>functionID<span class="token punctuation">,</span> previousVersion<span class="token punctuation">.</span>Descriptor<span class="token punctuation">,</span> previousVersion<span class="token punctuation">.</span>Handler<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📚-工具和资源" tabindex="-1"><a class="header-anchor" href="#📚-工具和资源"><span>📚 工具和资源</span></a></h2>
<h3 id="推荐工具" tabindex="-1"><a class="header-anchor" href="#推荐工具"><span><strong>推荐工具</strong></span></a></h3>
<table>
<thead>
<tr>
<th>工具</th>
<th>用途</th>
<th>支持语言</th>
<th>生产可用</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Air</strong></td>
<td>Go开发热重载</td>
<td>Go</td>
<td>否</td>
</tr>
<tr>
<td><strong>Nodemon</strong></td>
<td>Node.js开发热重载</td>
<td>JavaScript</td>
<td>否</td>
</tr>
<tr>
<td><strong>PM2</strong></td>
<td>Node.js生产部署</td>
<td>JavaScript</td>
<td>是</td>
</tr>
<tr>
<td><strong>JRebel</strong></td>
<td>Java热重载</td>
<td>Java</td>
<td>是</td>
</tr>
<tr>
<td><strong>Spring DevTools</strong></td>
<td>Spring开发热重载</td>
<td>Java</td>
<td>否</td>
</tr>
</tbody>
</table>
<h3 id="配置模板" tabindex="-1"><a class="header-anchor" href="#配置模板"><span><strong>配置模板</strong></span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 快速生成配置</span></span>
<span class="line">./scripts/generate-hotreload-config.sh <span class="token parameter variable">--language</span> go <span class="token parameter variable">--env</span> development</span>
<span class="line">./scripts/generate-hotreload-config.sh <span class="token parameter variable">--language</span> nodejs <span class="token parameter variable">--env</span> production</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="监控仪表板" tabindex="-1"><a class="header-anchor" href="#监控仪表板"><span><strong>监控仪表板</strong></span></a></h3>
<ul>
<li>Grafana仪表板模板：<code v-pre>monitoring/grafana/hotreload-dashboard.json</code></li>
<li>Prometheus规则：<code v-pre>monitoring/prometheus/hotreload-rules.yaml</code></li>
<li>告警配置：<code v-pre>monitoring/alertmanager/hotreload-alerts.yaml</code></li>
</ul>
<h2 id="🎯-总结和建议" tabindex="-1"><a class="header-anchor" href="#🎯-总结和建议"><span>🎯 总结和建议</span></a></h2>
<h3 id="关键原则" tabindex="-1"><a class="header-anchor" href="#关键原则"><span><strong>关键原则</strong></span></a></h3>
<ol>
<li><strong>渐进式采用</strong>：从基础自动重连开始，逐步引入高级特性</li>
<li><strong>环境分离</strong>：开发环境激进，生产环境保守</li>
<li><strong>监控优先</strong>：先建立监控，再启用热重载</li>
<li><strong>安全第一</strong>：所有热重载操作必须经过验证和审计</li>
</ol>
<h3 id="实施路径" tabindex="-1"><a class="header-anchor" href="#实施路径"><span><strong>实施路径</strong></span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">第1周：基础自动重连 + 开发环境热重载</span>
<span class="line">第2周：生产环境重连 + 监控告警</span>
<span class="line">第3周：函数热重载 + 安全审计</span>
<span class="line">第4周：高级特性 + 性能优化</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="成功指标" tabindex="-1"><a class="header-anchor" href="#成功指标"><span><strong>成功指标</strong></span></a></h3>
<ul>
<li>🎯 <strong>开发效率</strong>：代码变更到测试时间 &lt; 10秒</li>
<li>🎯 <strong>服务可用性</strong>：热重载导致的停机时间 &lt; 1%</li>
<li>🎯 <strong>错误率</strong>：热重载操作成功率 &gt; 99%</li>
<li>🎯 <strong>恢复时间</strong>：故障自动恢复时间 &lt; 30秒</li>
</ul>
<hr>
<p><em>🔥 通过遵循这些最佳实践，您可以安全、高效地在游戏开发中使用Croupier热更新功能！</em></p>
</div></template>


