<template><div><h1 id="croupier-配置管理系统使用指南" tabindex="-1"><a class="header-anchor" href="#croupier-配置管理系统使用指南"><span>Croupier 配置管理系统使用指南</span></a></h1>
<h2 id="🎯-概述" tabindex="-1"><a class="header-anchor" href="#🎯-概述"><span>🎯 概述</span></a></h2>
<p>Croupier配置管理系统是一个企业级的、高度可扩展的配置管理解决方案，支持多源配置、动态加载、热重载和全面验证。</p>
<h2 id="🚀-快速开始" tabindex="-1"><a class="header-anchor" href="#🚀-快速开始"><span>🚀 快速开始</span></a></h2>
<h3 id="_1-基本使用" tabindex="-1"><a class="header-anchor" href="#_1-基本使用"><span>1. 基本使用</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"log"</span></span>
<span class="line">    <span class="token string">"github.com/cuihairu/croupier/internal/config"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ctx <span class="token operator">:=</span> context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 创建配置管理器</span></span>
<span class="line">    manager<span class="token punctuation">,</span> err <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">NewManager</span><span class="token punctuation">(</span>ctx<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Fatal</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">defer</span> manager<span class="token punctuation">.</span><span class="token function">Close</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 从文件加载配置</span></span>
<span class="line">    err <span class="token operator">=</span> manager<span class="token punctuation">.</span><span class="token function">LoadFromFile</span><span class="token punctuation">(</span><span class="token string">"configs/app.yaml"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Fatal</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 获取配置</span></span>
<span class="line">    appConfig <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">GetAppConfig</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"应用: %s v%s"</span><span class="token punctuation">,</span> appConfig<span class="token punctuation">.</span>Name<span class="token punctuation">,</span> appConfig<span class="token punctuation">.</span>Version<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-多源配置" tabindex="-1"><a class="header-anchor" href="#_2-多源配置"><span>2. 多源配置</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 从多个源加载配置</span></span>
<span class="line">sources <span class="token operator">:=</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token operator">*</span>config<span class="token punctuation">.</span>ConfigSource<span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 基础配置文件（必需）</span></span>
<span class="line">    config<span class="token punctuation">.</span><span class="token function">NewConfigSource</span><span class="token punctuation">(</span>config<span class="token punctuation">.</span>SourceTypeFile<span class="token punctuation">,</span> <span class="token string">"/etc/croupier/base.yaml"</span><span class="token punctuation">,</span> <span class="token boolean">true</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// 环境特定配置文件</span></span>
<span class="line">    config<span class="token punctuation">.</span><span class="token function">NewConfigSource</span><span class="token punctuation">(</span>config<span class="token punctuation">.</span>SourceTypeFile<span class="token punctuation">,</span> <span class="token string">"/etc/croupier/prod.yaml"</span><span class="token punctuation">,</span> <span class="token boolean">false</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// 环境变量覆盖</span></span>
<span class="line">    config<span class="token punctuation">.</span><span class="token function">NewEnvConfigSource</span><span class="token punctuation">(</span><span class="token string">"CROUPIER_"</span><span class="token punctuation">,</span> <span class="token boolean">false</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// 远程配置中心</span></span>
<span class="line">    config<span class="token punctuation">.</span><span class="token function">NewRemoteConfigSource</span><span class="token punctuation">(</span></span>
<span class="line">        <span class="token string">"https://config.example.com/api/v1/config"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token builtin">string</span><span class="token punctuation">{</span><span class="token string">"Authorization"</span><span class="token punctuation">:</span> <span class="token string">"Bearer token"</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">err <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">LoadFromMultiple</span><span class="token punctuation">(</span>sources<span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📁-配置结构" tabindex="-1"><a class="header-anchor" href="#📁-配置结构"><span>📁 配置结构</span></a></h2>
<h3 id="主配置层次" tabindex="-1"><a class="header-anchor" href="#主配置层次"><span>主配置层次</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Config</span>
<span class="line">├── App              # 应用配置</span>
<span class="line">├── Network          # 网络配置</span>
<span class="line">│   └── Server      # 服务器配置</span>
<span class="line">├── Database         # 数据库配置</span>
<span class="line">│   ├── Primary     # 主数据库</span>
<span class="line">│   └── ReadOnly    # 只读副本</span>
<span class="line">├── Security         # 安全配置</span>
<span class="line">├── Observability    # 可观测性配置</span>
<span class="line">├── Business         # 业务配置</span>
<span class="line">└── Storage          # 存储配置</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="配置文件示例-yaml" tabindex="-1"><a class="header-anchor" href="#配置文件示例-yaml"><span>配置文件示例 (YAML)</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># configs/app.yaml</span></span>
<span class="line"><span class="token key atrule">app</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> <span class="token string">"croupier"</span></span>
<span class="line">  <span class="token key atrule">version</span><span class="token punctuation">:</span> <span class="token string">"1.0.0"</span></span>
<span class="line">  <span class="token key atrule">env</span><span class="token punctuation">:</span> <span class="token string">"production"</span></span>
<span class="line">  <span class="token key atrule">debug</span><span class="token punctuation">:</span> <span class="token boolean important">false</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">network</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">host</span><span class="token punctuation">:</span> <span class="token string">"0.0.0.0"</span></span>
<span class="line">    <span class="token key atrule">http_port</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">    <span class="token key atrule">grpc_port</span><span class="token punctuation">:</span> <span class="token number">9090</span></span>
<span class="line">    <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"/etc/certs/server.crt"</span></span>
<span class="line">      <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"/etc/certs/server.key"</span></span>
<span class="line">    <span class="token key atrule">cors</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">allowed_origins</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"https://example.com"</span><span class="token punctuation">]</span></span>
<span class="line">      <span class="token key atrule">allowed_methods</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">"GET"</span><span class="token punctuation">,</span> <span class="token string">"POST"</span><span class="token punctuation">,</span> <span class="token string">"PUT"</span><span class="token punctuation">,</span> <span class="token string">"DELETE"</span><span class="token punctuation">]</span></span>
<span class="line">    <span class="token key atrule">rate_limit</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">requests</span><span class="token punctuation">:</span> <span class="token number">1000</span></span>
<span class="line">      <span class="token key atrule">window</span><span class="token punctuation">:</span> <span class="token string">"1m"</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">database</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">primary</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">host</span><span class="token punctuation">:</span> <span class="token string">"db.example.com"</span></span>
<span class="line">    <span class="token key atrule">port</span><span class="token punctuation">:</span> <span class="token number">5432</span></span>
<span class="line">    <span class="token key atrule">database</span><span class="token punctuation">:</span> <span class="token string">"croupier"</span></span>
<span class="line">    <span class="token key atrule">username</span><span class="token punctuation">:</span> <span class="token string">"croupier_user"</span></span>
<span class="line">    <span class="token key atrule">password</span><span class="token punctuation">:</span> <span class="token string">"${DB_PASSWORD}"</span></span>
<span class="line">    <span class="token key atrule">ssl_mode</span><span class="token punctuation">:</span> <span class="token string">"require"</span></span>
<span class="line">  <span class="token key atrule">connection_pool</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">max_open_conns</span><span class="token punctuation">:</span> <span class="token number">25</span></span>
<span class="line">    <span class="token key atrule">max_idle_conns</span><span class="token punctuation">:</span> <span class="token number">5</span></span>
<span class="line">    <span class="token key atrule">conn_max_lifetime</span><span class="token punctuation">:</span> <span class="token string">"5m"</span></span>
<span class="line">  <span class="token key atrule">migration</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">path</span><span class="token punctuation">:</span> <span class="token string">"./migrations"</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">security</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">jwt</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">secret</span><span class="token punctuation">:</span> <span class="token string">"${JWT_SECRET}"</span></span>
<span class="line">    <span class="token key atrule">expiry</span><span class="token punctuation">:</span> <span class="token string">"1h"</span></span>
<span class="line">    <span class="token key atrule">refresh_expiry</span><span class="token punctuation">:</span> <span class="token string">"24h"</span></span>
<span class="line">  <span class="token key atrule">password_policy</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">min_length</span><span class="token punctuation">:</span> <span class="token number">12</span></span>
<span class="line">    <span class="token key atrule">require_uppercase</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">require_lowercase</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">require_numbers</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">require_symbols</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">retention</span><span class="token punctuation">:</span> <span class="token string">"90d"</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">observability</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">logging</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"info"</span></span>
<span class="line">    <span class="token key atrule">format</span><span class="token punctuation">:</span> <span class="token string">"json"</span></span>
<span class="line">  <span class="token key atrule">metrics</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">port</span><span class="token punctuation">:</span> <span class="token number">9090</span></span>
<span class="line">    <span class="token key atrule">path</span><span class="token punctuation">:</span> <span class="token string">"/metrics"</span></span>
<span class="line">  <span class="token key atrule">tracing</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">false</span></span>
<span class="line">    <span class="token key atrule">jaeger</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">false</span></span>
<span class="line">      <span class="token key atrule">endpoint</span><span class="token punctuation">:</span> <span class="token string">""</span></span>
<span class="line">  <span class="token key atrule">health_check</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">port</span><span class="token punctuation">:</span> <span class="token number">8081</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">business</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">games</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">max_concurrent_games</span><span class="token punctuation">:</span> <span class="token number">1000</span></span>
<span class="line">    <span class="token key atrule">max_players_per_game</span><span class="token punctuation">:</span> <span class="token number">10</span></span>
<span class="line">    <span class="token key atrule">default_game_timeout</span><span class="token punctuation">:</span> <span class="token string">"1h"</span></span>
<span class="line">  <span class="token key atrule">functions</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">registry</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">max_size</span><span class="token punctuation">:</span> <span class="token number">10000</span></span>
<span class="line">    <span class="token key atrule">execution</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">default_timeout</span><span class="token punctuation">:</span> <span class="token string">"30s"</span></span>
<span class="line">      <span class="token key atrule">max_timeout</span><span class="token punctuation">:</span> <span class="token string">"5m"</span></span>
<span class="line">  <span class="token key atrule">jobs</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">queue</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">max_size</span><span class="token punctuation">:</span> <span class="token number">5000</span></span>
<span class="line">    <span class="token key atrule">retry</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">max_attempts</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">      <span class="token key atrule">initial_delay</span><span class="token punctuation">:</span> <span class="token string">"1s"</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">files</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">base_path</span><span class="token punctuation">:</span> <span class="token string">"./data/files"</span></span>
<span class="line">  <span class="token key atrule">objects</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">provider</span><span class="token punctuation">:</span> <span class="token string">"s3"</span></span>
<span class="line">    <span class="token key atrule">s3</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">bucket</span><span class="token punctuation">:</span> <span class="token string">"croupier-files"</span></span>
<span class="line">      <span class="token key atrule">region</span><span class="token punctuation">:</span> <span class="token string">"us-east-1"</span></span>
<span class="line">      <span class="token key atrule">access_key</span><span class="token punctuation">:</span> <span class="token string">"${AWS_ACCESS_KEY}"</span></span>
<span class="line">      <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> <span class="token string">"${AWS_SECRET_KEY}"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔧-环境变量管理" tabindex="-1"><a class="header-anchor" href="#🔧-环境变量管理"><span>🔧 环境变量管理</span></a></h2>
<h3 id="环境变量命名规范" tabindex="-1"><a class="header-anchor" href="#环境变量命名规范"><span>环境变量命名规范</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用统一的命名空间 CROUPIER_</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_APP_NAME</span><span class="token operator">=</span>my-app</span>
<span class="line"><span class="token assign-left variable">CROUPIER_APP_VERSION</span><span class="token operator">=</span><span class="token number">1.0</span>.0</span>
<span class="line"><span class="token assign-left variable">CROUPIER_APP_ENV</span><span class="token operator">=</span>production</span>
<span class="line"><span class="token assign-left variable">CROUPIER_APP_DEBUG</span><span class="token operator">=</span>false</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 网络配置</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_NETWORK_SERVER_HOST</span><span class="token operator">=</span><span class="token number">0.0</span>.0.0</span>
<span class="line"><span class="token assign-left variable">CROUPIER_NETWORK_SERVER_HTTP_PORT</span><span class="token operator">=</span><span class="token number">8080</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_NETWORK_SERVER_GRPC_PORT</span><span class="token operator">=</span><span class="token number">9090</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_NETWORK_SERVER_TLS_ENABLED</span><span class="token operator">=</span>true</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 数据库配置</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_DATABASE_ENABLED</span><span class="token operator">=</span>true</span>
<span class="line"><span class="token assign-left variable">CROUPIER_DATABASE_PRIMARY_HOST</span><span class="token operator">=</span>db.example.com</span>
<span class="line"><span class="token assign-left variable">CROUPIER_DATABASE_PRIMARY_PORT</span><span class="token operator">=</span><span class="token number">5432</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_DATABASE_PRIMARY_DATABASE</span><span class="token operator">=</span>croupier</span>
<span class="line"><span class="token assign-left variable">CROUPIER_DATABASE_PRIMARY_USERNAME</span><span class="token operator">=</span>croupier</span>
<span class="line"><span class="token assign-left variable">CROUPIER_DATABASE_PRIMARY_PASSWORD</span><span class="token operator">=</span>your-secure-password</span>
<span class="line"><span class="token assign-left variable">CROUPIER_DATABASE_PRIMARY_SSL_MODE</span><span class="token operator">=</span>require</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 安全配置</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_SECURITY_JWT_ENABLED</span><span class="token operator">=</span>true</span>
<span class="line"><span class="token assign-left variable">CROUPIER_SECURITY_JWT_SECRET</span><span class="token operator">=</span>your-very-long-jwt-secret-key-here</span>
<span class="line"><span class="token assign-left variable">CROUPIER_SECURITY_JWT_EXPIRY</span><span class="token operator">=</span>1h</span>
<span class="line"><span class="token assign-left variable">CROUPIER_SECURITY_JWT_REFRESH_EXPIRY</span><span class="token operator">=</span>24h</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="程序化使用" tabindex="-1"><a class="header-anchor" href="#程序化使用"><span>程序化使用</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">type</span> AppConfig <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Name     <span class="token builtin">string</span>        <span class="token string">`env:"APP_NAME"`</span></span>
<span class="line">    Version  <span class="token builtin">string</span>        <span class="token string">`env:"APP_VERSION"`</span></span>
<span class="line">    Env      <span class="token builtin">string</span>        <span class="token string">`env:"APP_ENV"`</span></span>
<span class="line">    Debug    <span class="token builtin">bool</span>          <span class="token string">`env:"APP_DEBUG"`</span></span>
<span class="line">    Port     <span class="token builtin">int</span>           <span class="token string">`env:"APP_PORT"`</span></span>
<span class="line">    Timeout  time<span class="token punctuation">.</span>Duration <span class="token string">`env:"APP_TIMEOUT"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 创建环境变量管理器</span></span>
<span class="line">envManager <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">NewEnvManager</span><span class="token punctuation">(</span><span class="token string">"CROUPIER_"</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 添加转换器</span></span>
<span class="line">envManager<span class="token punctuation">.</span><span class="token function">AddTransformer</span><span class="token punctuation">(</span><span class="token operator">&amp;</span>config<span class="token punctuation">.</span>URLTransformer<span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line">envManager<span class="token punctuation">.</span><span class="token function">AddTransformer</span><span class="token punctuation">(</span><span class="token operator">&amp;</span>config<span class="token punctuation">.</span>LowerCaseTransformer<span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 加载配置</span></span>
<span class="line"><span class="token keyword">var</span> config AppConfig</span>
<span class="line">err <span class="token operator">:=</span> envManager<span class="token punctuation">.</span><span class="token function">LoadFromEnv</span><span class="token punctuation">(</span><span class="token operator">&amp;</span>config<span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="✅-配置验证" tabindex="-1"><a class="header-anchor" href="#✅-配置验证"><span>✅ 配置验证</span></a></h2>
<h3 id="内置验证规则" tabindex="-1"><a class="header-anchor" href="#内置验证规则"><span>内置验证规则</span></a></h3>
<ol>
<li><strong>应用验证</strong> - 名称、版本、环境</li>
<li><strong>网络验证</strong> - 端口范围、TLS配置</li>
<li><strong>数据库验证</strong> - 连接参数、连接池</li>
<li><strong>安全验证</strong> - JWT密钥长度、密码策略</li>
<li><strong>可观测性验证</strong> - 日志级别、指标配置</li>
<li><strong>业务验证</strong> - 游戏配置、函数配置</li>
<li><strong>存储验证</strong> - 存储提供商配置</li>
</ol>
<h3 id="使用验证" tabindex="-1"><a class="header-anchor" href="#使用验证"><span>使用验证</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 创建验证器</span></span>
<span class="line">validator <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">NewDefaultValidator</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 添加自定义验证规则</span></span>
<span class="line">validator<span class="token punctuation">.</span><span class="token function">AddRule</span><span class="token punctuation">(</span>config<span class="token punctuation">.</span><span class="token function">NewCustomValidationRule</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"PortRangeValidation"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"验证端口范围"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token keyword">func</span><span class="token punctuation">(</span>config <span class="token operator">*</span>config<span class="token punctuation">.</span>Config<span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">if</span> config<span class="token punctuation">.</span>Network<span class="token punctuation">.</span>Server<span class="token punctuation">.</span>HTTPPort <span class="token operator">==</span> config<span class="token punctuation">.</span>Network<span class="token punctuation">.</span>Server<span class="token punctuation">.</span>GRPCPort <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> fmt<span class="token punctuation">.</span><span class="token function">Errorf</span><span class="token punctuation">(</span><span class="token string">"HTTP和gRPC端口不能相同"</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 验证配置</span></span>
<span class="line">err <span class="token operator">:=</span> validator<span class="token punctuation">.</span><span class="token function">Validate</span><span class="token punctuation">(</span>configStruct<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Fatal</span><span class="token punctuation">(</span><span class="token string">"配置验证失败:"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="便捷验证函数" tabindex="-1"><a class="header-anchor" href="#便捷验证函数"><span>便捷验证函数</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 端口验证</span></span>
<span class="line">err <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">ValidatePort</span><span class="token punctuation">(</span><span class="token number">8080</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// URL验证</span></span>
<span class="line">err <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">ValidateURL</span><span class="token punctuation">(</span><span class="token string">"https://example.com"</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 邮箱验证</span></span>
<span class="line">err <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">ValidateEmail</span><span class="token punctuation">(</span><span class="token string">"user@example.com"</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 数值范围验证</span></span>
<span class="line">err <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">ValidateRange</span><span class="token punctuation">(</span>port<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token number">65535</span><span class="token punctuation">,</span> <span class="token string">"端口"</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 时间段验证</span></span>
<span class="line">err <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">ValidateDuration</span><span class="token punctuation">(</span>time<span class="token punctuation">.</span>Hour<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 字符串枚举验证</span></span>
<span class="line">err <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">ValidateStringEnum</span><span class="token punctuation">(</span>env<span class="token punctuation">,</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span><span class="token punctuation">{</span><span class="token string">"dev"</span><span class="token punctuation">,</span> <span class="token string">"test"</span><span class="token punctuation">,</span> <span class="token string">"prod"</span><span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token string">"环境"</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔄-热重载和监听" tabindex="-1"><a class="header-anchor" href="#🔄-热重载和监听"><span>🔄 热重载和监听</span></a></h2>
<h3 id="配置监听" tabindex="-1"><a class="header-anchor" href="#配置监听"><span>配置监听</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 监听配置变更</span></span>
<span class="line">manager<span class="token punctuation">.</span><span class="token function">WatchConfig</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token keyword">func</span><span class="token punctuation">(</span>config <span class="token operator">*</span>config<span class="token punctuation">.</span>Config<span class="token punctuation">,</span> err <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"配置监听错误: %v"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"配置已更新:"</span><span class="token punctuation">)</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"  HTTP端口: %d"</span><span class="token punctuation">,</span> config<span class="token punctuation">.</span>Network<span class="token punctuation">.</span>Server<span class="token punctuation">.</span>HTTPPort<span class="token punctuation">)</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"  数据库: %s:%d/%s"</span><span class="token punctuation">,</span></span>
<span class="line">        config<span class="token punctuation">.</span>Database<span class="token punctuation">.</span>Primary<span class="token punctuation">.</span>Host<span class="token punctuation">,</span></span>
<span class="line">        config<span class="token punctuation">.</span>Database<span class="token punctuation">.</span>Primary<span class="token punctuation">.</span>Port<span class="token punctuation">,</span></span>
<span class="line">        config<span class="token punctuation">.</span>Database<span class="token punctuation">.</span>Primary<span class="token punctuation">.</span>Database<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="重启信号处理" tabindex="-1"><a class="header-anchor" href="#重启信号处理"><span>重启信号处理</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 监听配置重启信号</span></span>
<span class="line">restartChan <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">RestartChan</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">go</span> <span class="token keyword">func</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token keyword">range</span> restartChan <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Println</span><span class="token punctuation">(</span><span class="token string">"收到配置重启信号，准备重启服务..."</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token comment">// 优雅关闭现有服务</span></span>
<span class="line">        <span class="token comment">// 重新初始化服务组件</span></span>
<span class="line">        <span class="token comment">// 启动新服务实例</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="手动重新加载" tabindex="-1"><a class="header-anchor" href="#手动重新加载"><span>手动重新加载</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 重新加载所有配置源</span></span>
<span class="line">err <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">Reload</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"重新加载配置失败: %v"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span> <span class="token keyword">else</span> <span class="token punctuation">{</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Println</span><span class="token punctuation">(</span><span class="token string">"配置重新加载成功"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔐-安全最佳实践" tabindex="-1"><a class="header-anchor" href="#🔐-安全最佳实践"><span>🔐 安全最佳实践</span></a></h2>
<h3 id="敏感信息管理" tabindex="-1"><a class="header-anchor" href="#敏感信息管理"><span>敏感信息管理</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># ✅ 好的做法 - 使用环境变量</span></span>
<span class="line"><span class="token key atrule">security</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">jwt</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">secret</span><span class="token punctuation">:</span> <span class="token string">"${JWT_SECRET}"</span></span>
<span class="line"><span class="token key atrule">database</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">primary</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">password</span><span class="token punctuation">:</span> <span class="token string">"${DB_PASSWORD}"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># ❌ 坏的做法 - 直接在配置文件中存储</span></span>
<span class="line"><span class="token key atrule">security</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">jwt</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">secret</span><span class="token punctuation">:</span> <span class="token string">"hardcoded-secret"</span></span>
<span class="line"><span class="token key atrule">database</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">primary</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">password</span><span class="token punctuation">:</span> <span class="token string">"hardcoded-password"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="环境变量文件-env" tabindex="-1"><a class="header-anchor" href="#环境变量文件-env"><span>环境变量文件 (.env)</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># .env.example - 模板文件</span></span>
<span class="line"><span class="token assign-left variable">APP_NAME</span><span class="token operator">=</span>croupier</span>
<span class="line"><span class="token assign-left variable">APP_VERSION</span><span class="token operator">=</span><span class="token number">1.0</span>.0</span>
<span class="line"><span class="token assign-left variable">APP_ENV</span><span class="token operator">=</span>development</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 敏感信息 - 实际部署时需要设置真实值</span></span>
<span class="line"><span class="token assign-left variable">JWT_SECRET</span><span class="token operator">=</span></span>
<span class="line"><span class="token assign-left variable">DB_PASSWORD</span><span class="token operator">=</span></span>
<span class="line"><span class="token assign-left variable">AWS_ACCESS_KEY</span><span class="token operator">=</span></span>
<span class="line"><span class="token assign-left variable">AWS_SECRET_KEY</span><span class="token operator">=</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># .env - 本地开发文件（不要提交到版本控制）</span></span>
<span class="line"><span class="token assign-left variable">JWT_SECRET</span><span class="token operator">=</span>your-local-jwt-secret-for-development</span>
<span class="line"><span class="token assign-left variable">DB_PASSWORD</span><span class="token operator">=</span>local-dev-password</span>
<span class="line"><span class="token assign-left variable">AWS_ACCESS_KEY</span><span class="token operator">=</span>your-local-aws-key</span>
<span class="line"><span class="token assign-left variable">AWS_SECRET_KEY</span><span class="token operator">=</span>your-local-aws-secret</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="配置权限控制" tabindex="-1"><a class="header-anchor" href="#配置权限控制"><span>配置权限控制</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 设置配置文件权限</span></span>
<span class="line"><span class="token function">chmod</span> <span class="token number">600</span> configs/app.yaml</span>
<span class="line"><span class="token function">chmod</span> <span class="token number">600</span> .env</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 确保配置目录权限正确</span></span>
<span class="line"><span class="token function">chmod</span> <span class="token number">700</span> configs/</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📊-监控和可观测性" tabindex="-1"><a class="header-anchor" href="#📊-监控和可观测性"><span>📊 监控和可观测性</span></a></h2>
<h3 id="配置监控" tabindex="-1"><a class="header-anchor" href="#配置监控"><span>配置监控</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 获取配置源信息</span></span>
<span class="line">sources <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">GetConfigSources</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">for</span> <span class="token boolean">_</span><span class="token punctuation">,</span> source <span class="token operator">:=</span> <span class="token keyword">range</span> sources <span class="token punctuation">{</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"配置源: %s (%s) 加载时间: %v"</span><span class="token punctuation">,</span></span>
<span class="line">        source<span class="token punctuation">.</span>Path<span class="token punctuation">,</span> source<span class="token punctuation">.</span>Type<span class="token punctuation">,</span> source<span class="token punctuation">.</span>Loaded<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 配置健康检查</span></span>
<span class="line"><span class="token keyword">type</span> ConfigHealth <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    Loaded       <span class="token builtin">bool</span>      <span class="token string">`json:"loaded"`</span></span>
<span class="line">    SourcesCount <span class="token builtin">int</span>       <span class="token string">`json:"sources_count"`</span></span>
<span class="line">    LastReload   time<span class="token punctuation">.</span>Time <span class="token string">`json:"last_reload"`</span></span>
<span class="line">    Errors       <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">string</span>  <span class="token string">`json:"errors,omitempty"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">health <span class="token operator">:=</span> ConfigHealth<span class="token punctuation">{</span></span>
<span class="line">    Loaded<span class="token punctuation">:</span>       <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    SourcesCount<span class="token punctuation">:</span> <span class="token function">len</span><span class="token punctuation">(</span>sources<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    LastReload<span class="token punctuation">:</span>   time<span class="token punctuation">.</span><span class="token function">Now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 导出健康检查信息（go-zero handler）</span></span>
<span class="line">httpx<span class="token punctuation">.</span><span class="token function">OkJson</span><span class="token punctuation">(</span>w<span class="token punctuation">,</span> health<span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="配置指标" tabindex="-1"><a class="header-anchor" href="#配置指标"><span>配置指标</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 配置加载次数</span></span>
<span class="line"><span class="token keyword">var</span> configLoadCount <span class="token operator">=</span> prometheus<span class="token punctuation">.</span><span class="token function">NewCounter</span><span class="token punctuation">(</span></span>
<span class="line">    prometheus<span class="token punctuation">.</span>CounterOpts<span class="token punctuation">{</span></span>
<span class="line">        Name<span class="token punctuation">:</span> <span class="token string">"config_load_total"</span><span class="token punctuation">,</span></span>
<span class="line">        Help<span class="token punctuation">:</span> <span class="token string">"Total number of configuration loads"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 配置加载时间</span></span>
<span class="line"><span class="token keyword">var</span> configLoadDuration <span class="token operator">=</span> prometheus<span class="token punctuation">.</span><span class="token function">NewHistogram</span><span class="token punctuation">(</span></span>
<span class="line">    prometheus<span class="token punctuation">.</span>HistogramOpts<span class="token punctuation">{</span></span>
<span class="line">        Name<span class="token punctuation">:</span> <span class="token string">"config_load_duration_seconds"</span><span class="token punctuation">,</span></span>
<span class="line">        Help<span class="token punctuation">:</span> <span class="token string">"Time spent loading configuration"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 配置验证失败次数</span></span>
<span class="line"><span class="token keyword">var</span> configValidationFailures <span class="token operator">=</span> prometheus<span class="token punctuation">.</span><span class="token function">NewCounter</span><span class="token punctuation">(</span></span>
<span class="line">    prometheus<span class="token punctuation">.</span>CounterOpts<span class="token punctuation">{</span></span>
<span class="line">        Name<span class="token punctuation">:</span> <span class="token string">"config_validation_failures_total"</span><span class="token punctuation">,</span></span>
<span class="line">        Help<span class="token punctuation">:</span> <span class="token string">"Total number of configuration validation failures"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🚀-部署配置" tabindex="-1"><a class="header-anchor" href="#🚀-部署配置"><span>🚀 部署配置</span></a></h2>
<h3 id="docker-部署" tabindex="-1"><a class="header-anchor" href="#docker-部署"><span>Docker 部署</span></a></h3>
<div class="language-docker line-numbers-mode" data-highlighter="prismjs" data-ext="docker"><pre v-pre><code class="language-docker"><span class="line"><span class="token comment"># Dockerfile</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">FROM</span> golang:1.21-alpine <span class="token keyword">AS</span> builder</span></span>
<span class="line"></span>
<span class="line"><span class="token instruction"><span class="token keyword">WORKDIR</span> /app</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">COPY</span> go.mod go.sum ./</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">RUN</span> go mod download</span></span>
<span class="line"></span>
<span class="line"><span class="token instruction"><span class="token keyword">COPY</span> . .</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">RUN</span> CGO_ENABLED=0 GOOS=linux go build -o croupier ./cmd/server</span></span>
<span class="line"></span>
<span class="line"><span class="token instruction"><span class="token keyword">FROM</span> alpine:latest</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">RUN</span> apk --no-cache add ca-certificates</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">WORKDIR</span> /root/</span></span>
<span class="line"></span>
<span class="line"><span class="token instruction"><span class="token keyword">COPY</span> <span class="token options"><span class="token property">--from</span><span class="token punctuation">=</span><span class="token string">builder</span></span> /app/croupier .</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">COPY</span> <span class="token options"><span class="token property">--from</span><span class="token punctuation">=</span><span class="token string">builder</span></span> /app/configs ./configs</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 设置环境变量</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">ENV</span> CROUPIER_APP_ENV=production</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">ENV</span> CROUPIER_APP_NAME=croupier</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">ENV</span> CROUPIER_APP_VERSION=1.0.0</span></span>
<span class="line"></span>
<span class="line"><span class="token instruction"><span class="token keyword">EXPOSE</span> 8080 9090</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">CMD</span> [<span class="token string">"./croupier"</span>]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># docker-compose.yml</span></span>
<span class="line"><span class="token key atrule">version</span><span class="token punctuation">:</span> <span class="token string">'3.8'</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">services</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">croupier</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">build</span><span class="token punctuation">:</span> .</span>
<span class="line">    <span class="token key atrule">ports</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"8080:8080"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"9090:9090"</span></span>
<span class="line">    <span class="token key atrule">environment</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_APP_ENV=production</span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_DATABASE_PRIMARY_HOST=db</span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_DATABASE_PRIMARY_PORT=5432</span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_DATABASE_PRIMARY_DATABASE=croupier</span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_DATABASE_PRIMARY_USERNAME=croupier</span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_DATABASE_PRIMARY_PASSWORD=$<span class="token punctuation">{</span>DB_PASSWORD<span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_SECURITY_JWT_SECRET=$<span class="token punctuation">{</span>JWT_SECRET<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> ./configs<span class="token punctuation">:</span>/app/configs</span>
<span class="line">      <span class="token punctuation">-</span> ./data<span class="token punctuation">:</span>/app/data</span>
<span class="line">    <span class="token key atrule">depends_on</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> db</span>
<span class="line">      <span class="token punctuation">-</span> redis</span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">image</span><span class="token punctuation">:</span> postgres<span class="token punctuation">:</span><span class="token number">15</span></span>
<span class="line">    <span class="token key atrule">environment</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">POSTGRES_DB</span><span class="token punctuation">:</span> croupier</span>
<span class="line">      <span class="token key atrule">POSTGRES_USER</span><span class="token punctuation">:</span> croupier</span>
<span class="line">      <span class="token key atrule">POSTGRES_PASSWORD</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>DB_PASSWORD<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> postgres_data<span class="token punctuation">:</span>/var/lib/postgresql/data</span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">redis</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">image</span><span class="token punctuation">:</span> redis<span class="token punctuation">:</span>7<span class="token punctuation">-</span>alpine</span>
<span class="line">    <span class="token key atrule">command</span><span class="token punctuation">:</span> redis<span class="token punctuation">-</span>server <span class="token punctuation">-</span><span class="token punctuation">-</span>appendonly yes</span>
<span class="line">    <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> redis_data<span class="token punctuation">:</span>/data</span>
<span class="line"></span>
<span class="line"><span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">postgres_data</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">redis_data</span><span class="token punctuation">:</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="kubernetes-部署" tabindex="-1"><a class="header-anchor" href="#kubernetes-部署"><span>Kubernetes 部署</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># k8s/configmap.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> ConfigMap</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>config</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> production</span>
<span class="line"><span class="token key atrule">data</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">app.yaml</span><span class="token punctuation">:</span> <span class="token punctuation">|</span><span class="token scalar string"></span>
<span class="line">    app:</span>
<span class="line">      name: "croupier"</span>
<span class="line">      version: "1.0.0"</span>
<span class="line">      env: "production"</span>
<span class="line">      debug: false</span>
<span class="line">    network:</span>
<span class="line">      server:</span>
<span class="line">        http_port: 8080</span>
<span class="line">        grpc_port: 9090</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">---</span></span>
<span class="line"><span class="token comment"># k8s/secret.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Secret</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>secrets</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> production</span>
<span class="line"><span class="token key atrule">type</span><span class="token punctuation">:</span> Opaque</span>
<span class="line"><span class="token key atrule">data</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">jwt-secret</span><span class="token punctuation">:</span> &lt;base64<span class="token punctuation">-</span>encoded<span class="token punctuation">-</span>secret<span class="token punctuation">></span></span>
<span class="line">  <span class="token key atrule">db-password</span><span class="token punctuation">:</span> &lt;base64<span class="token punctuation">-</span>encoded<span class="token punctuation">-</span>password<span class="token punctuation">></span></span>
<span class="line">  <span class="token key atrule">aws-access-key</span><span class="token punctuation">:</span> &lt;base64<span class="token punctuation">-</span>encoded<span class="token punctuation">-</span>key<span class="token punctuation">></span></span>
<span class="line">  <span class="token key atrule">aws-secret-key</span><span class="token punctuation">:</span> &lt;base64<span class="token punctuation">-</span>encoded<span class="token punctuation">-</span>key<span class="token punctuation">></span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">---</span></span>
<span class="line"><span class="token comment"># k8s/deployment.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> apps/v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Deployment</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> production</span>
<span class="line"><span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">replicas</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">  <span class="token key atrule">selector</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">matchLabels</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">app</span><span class="token punctuation">:</span> croupier</span>
<span class="line">  <span class="token key atrule">template</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">app</span><span class="token punctuation">:</span> croupier</span>
<span class="line">    <span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">containers</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier</span>
<span class="line">        <span class="token key atrule">image</span><span class="token punctuation">:</span> croupier<span class="token punctuation">:</span>latest</span>
<span class="line">        <span class="token key atrule">ports</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">containerPort</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">containerPort</span><span class="token punctuation">:</span> <span class="token number">9090</span></span>
<span class="line">        <span class="token key atrule">env</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> CROUPIER_APP_ENV</span>
<span class="line">          <span class="token key atrule">value</span><span class="token punctuation">:</span> <span class="token string">"production"</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> CROUPIER_SECURITY_JWT_SECRET</span>
<span class="line">          <span class="token key atrule">valueFrom</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">secretKeyRef</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>secrets</span>
<span class="line">              <span class="token key atrule">key</span><span class="token punctuation">:</span> jwt<span class="token punctuation">-</span>secret</span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> CROUPIER_DATABASE_PRIMARY_PASSWORD</span>
<span class="line">          <span class="token key atrule">valueFrom</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">secretKeyRef</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>secrets</span>
<span class="line">              <span class="token key atrule">key</span><span class="token punctuation">:</span> db<span class="token punctuation">-</span>password</span>
<span class="line">        <span class="token key atrule">volumeMounts</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> config<span class="token punctuation">-</span>volume</span>
<span class="line">          <span class="token key atrule">mountPath</span><span class="token punctuation">:</span> /app/configs</span>
<span class="line">      <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> config<span class="token punctuation">-</span>volume</span>
<span class="line">        <span class="token key atrule">configMap</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>config</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🛠️-开发工具" tabindex="-1"><a class="header-anchor" href="#🛠️-开发工具"><span>🛠️ 开发工具</span></a></h2>
<h3 id="配置验证cli" tabindex="-1"><a class="header-anchor" href="#配置验证cli"><span>配置验证CLI</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 验证配置文件</span></span>
<span class="line">./croupier config validate <span class="token parameter variable">--config</span> configs/app.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 测试数据库连接</span></span>
<span class="line">./croupier config test-db <span class="token parameter variable">--config</span> configs/app.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 导出配置到JSON</span></span>
<span class="line">./croupier config <span class="token builtin class-name">export</span> <span class="token parameter variable">--format</span> json <span class="token parameter variable">--config</span> configs/app.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 显示配置信息</span></span>
<span class="line">./croupier config info <span class="token parameter variable">--config</span> configs/app.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="配置生成器" tabindex="-1"><a class="header-anchor" href="#配置生成器"><span>配置生成器</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 生成默认配置模板</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">GenerateDefaultConfig</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">*</span>config<span class="token punctuation">.</span>Config <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token operator">&amp;</span>config<span class="token punctuation">.</span>Config<span class="token punctuation">{</span></span>
<span class="line">        App<span class="token punctuation">:</span> config<span class="token punctuation">.</span>AppConfig<span class="token punctuation">{</span></span>
<span class="line">            Name<span class="token punctuation">:</span>    <span class="token string">"croupier"</span><span class="token punctuation">,</span></span>
<span class="line">            Version<span class="token punctuation">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">            Env<span class="token punctuation">:</span>     <span class="token string">"development"</span><span class="token punctuation">,</span></span>
<span class="line">            Debug<span class="token punctuation">:</span>   <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        Network<span class="token punctuation">:</span> config<span class="token punctuation">.</span>NetworkConfig<span class="token punctuation">{</span></span>
<span class="line">            Server<span class="token punctuation">:</span> config<span class="token punctuation">.</span>ServerConfig<span class="token punctuation">{</span></span>
<span class="line">                Host<span class="token punctuation">:</span>     <span class="token string">"localhost"</span><span class="token punctuation">,</span></span>
<span class="line">                HTTPPort<span class="token punctuation">:</span> <span class="token number">8080</span><span class="token punctuation">,</span></span>
<span class="line">                GRPCPort<span class="token punctuation">:</span> <span class="token number">9090</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token comment">// ... 其他默认配置</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔍-故障排除" tabindex="-1"><a class="header-anchor" href="#🔍-故障排除"><span>🔍 故障排除</span></a></h2>
<h3 id="常见问题" tabindex="-1"><a class="header-anchor" href="#常见问题"><span>常见问题</span></a></h3>
<ol>
<li>
<p><strong>配置文件解析失败</strong></p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查YAML语法</span></span>
<span class="line">yamllint configs/app.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证JSON格式</span></span>
<span class="line">python <span class="token parameter variable">-m</span> json.tool configs/app.json</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p><strong>环境变量未生效</strong></p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 检查环境变量前缀</span></span>
<span class="line">envManager <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">NewEnvManager</span><span class="token punctuation">(</span><span class="token string">"CROUPIER_"</span><span class="token punctuation">)</span></span>
<span class="line">envInfo <span class="token operator">:=</span> envManager<span class="token punctuation">.</span><span class="token function">GetEnvInfo</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">for</span> key<span class="token punctuation">,</span> info <span class="token operator">:=</span> <span class="token keyword">range</span> envInfo <span class="token punctuation">{</span></span>
<span class="line">    fmt<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"%s = %v\n"</span><span class="token punctuation">,</span> key<span class="token punctuation">,</span> info<span class="token punctuation">.</span>Value<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p><strong>配置验证失败</strong></p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 详细的验证错误信息</span></span>
<span class="line">validator <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">NewDefaultValidator</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">err <span class="token operator">:=</span> validator<span class="token punctuation">.</span><span class="token function">Validate</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">    fmt<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"验证失败详情: %+v\n"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p><strong>热重载不工作</strong></p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 检查配置监听器是否正确设置</span></span>
<span class="line">manager<span class="token punctuation">.</span><span class="token function">WatchConfig</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token keyword">func</span><span class="token punctuation">(</span>config <span class="token operator">*</span>config<span class="token punctuation">.</span>Config<span class="token punctuation">,</span> err <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"配置监听器被调用: err=%v"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
</ol>
<h3 id="调试工具" tabindex="-1"><a class="header-anchor" href="#调试工具"><span>调试工具</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 配置调试器</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">DebugConfig</span><span class="token punctuation">(</span>manager <span class="token operator">*</span>config<span class="token punctuation">.</span>Manager<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 获取当前配置</span></span>
<span class="line">    config <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">GetConfig</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 导出配置用于调试</span></span>
<span class="line">    jsonConfig<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">Export</span><span class="token punctuation">(</span><span class="token string">"json"</span><span class="token punctuation">)</span></span>
<span class="line">    fmt<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"当前配置:\n%s\n"</span><span class="token punctuation">,</span> <span class="token function">string</span><span class="token punctuation">(</span>jsonConfig<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 检查配置源</span></span>
<span class="line">    sources <span class="token operator">:=</span> manager<span class="token punctuation">.</span><span class="token function">GetConfigSources</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    fmt<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"配置源: %+v\n"</span><span class="token punctuation">,</span> sources<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 验证配置</span></span>
<span class="line">    validator <span class="token operator">:=</span> config<span class="token punctuation">.</span><span class="token function">NewDefaultValidator</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">:=</span> validator<span class="token punctuation">.</span><span class="token function">Validate</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        fmt<span class="token punctuation">.</span><span class="token function">Printf</span><span class="token punctuation">(</span><span class="token string">"配置验证错误: %v\n"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">else</span> <span class="token punctuation">{</span></span>
<span class="line">        fmt<span class="token punctuation">.</span><span class="token function">Println</span><span class="token punctuation">(</span><span class="token string">"配置验证通过"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📚-更多资源" tabindex="-1"><a class="header-anchor" href="#📚-更多资源"><span>📚 更多资源</span></a></h2>
<ul>
<li><RouteLink to="/api.html">API文档</RouteLink></li>
<li><a href="https://github.com/cuihairu/croupier/tree/main/configs" target="_blank" rel="noopener noreferrer">配置模板</a></li>
<li><RouteLink to="/e2e-example.html">示例代码</RouteLink></li>
<li><a href="#%E5%AE%89%E5%85%A8%E6%9C%80%E4%BD%B3%E5%AE%9E%E8%B7%B5">安全最佳实践</a></li>
</ul>
<p>通过这个配置管理系统，您可以轻松管理从开发到生产的所有配置需求，确保系统的稳定性和安全性。</p>
</div></template>


