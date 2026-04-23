<template><div><h1 id="配置管理" tabindex="-1"><a class="header-anchor" href="#配置管理"><span>配置管理</span></a></h1>
<p>Croupier 使用 YAML 配置文件管理系统行为。本文档详细说明配置选项、优先级和最佳实践。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#配置优先级">配置优先级</router-link><ul><li><router-link to="#环境变量语法">环境变量语法</router-link></li></ul></li><li><router-link to="#server-配置">Server 配置</router-link><ul><li><router-link to="#完整配置示例">完整配置示例</router-link></li><li><router-link to="#环境变量覆盖">环境变量覆盖</router-link></li></ul></li><li><router-link to="#agent-配置">Agent 配置</router-link><ul><li><router-link to="#完整配置示例-1">完整配置示例</router-link></li></ul></li><li><router-link to="#edge-配置">Edge 配置</router-link><ul><li><router-link to="#完整配置示例-2">完整配置示例</router-link></li></ul></li><li><router-link to="#配置验证">配置验证</router-link><ul><li><router-link to="#验证配置文件">验证配置文件</router-link></li><li><router-link to="#常见配置错误">常见配置错误</router-link></li></ul></li><li><router-link to="#敏感信息处理">敏感信息处理</router-link><ul><li><router-link to="#使用环境变量">使用环境变量</router-link></li><li><router-link to="#环境变量展开">环境变量展开</router-link></li></ul></li><li><router-link to="#profiles-使用">Profiles 使用</router-link><ul><li><router-link to="#激活-profile">激活 Profile</router-link></li><li><router-link to="#profile-配置示例">Profile 配置示例</router-link></li></ul></li><li><router-link to="#对象存储配置">对象存储配置</router-link><ul><li><router-link to="#s3-兼容存储">S3 兼容存储</router-link></li><li><router-link to="#minio">MinIO</router-link></li><li><router-link to="#腾讯云-cos">腾讯云 COS</router-link></li><li><router-link to="#本地文件存储">本地文件存储</router-link></li></ul></li><li><router-link to="#最佳实践">最佳实践</router-link><ul><li><router-link to="#_1-分离环境配置">1. 分离环境配置</router-link></li><li><router-link to="#_2-使用环境变量管理敏感信息">2. 使用环境变量管理敏感信息</router-link></li><li><router-link to="#_3-配置文件模板">3. 配置文件模板</router-link></li><li><router-link to="#_4-配置验证">4. 配置验证</router-link></li></ul></li><li><router-link to="#下一步">下一步</router-link></li></ul></nav>
<h2 id="配置优先级" tabindex="-1"><a class="header-anchor" href="#配置优先级"><span>配置优先级</span></a></h2>
<p>配置加载顺序（低 → 高）：</p>
<ol>
<li><strong>YAML 文件</strong> - 基础配置</li>
<li><strong>YAML includes</strong> - 包含的配置文件</li>
<li><strong>YAML profiles</strong> - 环境配置</li>
<li><strong>环境变量</strong> - 运行时覆盖</li>
<li><strong>命令行参数</strong> - 最高优先级</li>
</ol>
<h3 id="环境变量语法" tabindex="-1"><a class="header-anchor" href="#环境变量语法"><span>环境变量语法</span></a></h3>
<ul>
<li>环境变量前缀：<code v-pre>CROUPIER_SERVER_*</code>、<code v-pre>CROUPIER_AGENT_*</code>、<code v-pre>CROUPIER_EDGE_*</code></li>
<li>点号和连字符转换为下划线</li>
<li>示例：<code v-pre>CROUPIER_SERVER_ADDR</code>、<code v-pre>CROUPIER_SERVER_HTTP_ADDR</code></li>
</ul>
<h2 id="server-配置" tabindex="-1"><a class="header-anchor" href="#server-配置"><span>Server 配置</span></a></h2>
<h3 id="完整配置示例" tabindex="-1"><a class="header-anchor" href="#完整配置示例"><span>完整配置示例</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># server.yaml</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token comment"># gRPC 监听地址</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":8443"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># HTTP REST API 监听地址</span></span>
<span class="line">  <span class="token key atrule">http_addr</span><span class="token punctuation">:</span> <span class="token string">":8080"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># TLS 配置</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.key"</span></span>
<span class="line">    <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span>  <span class="token comment"># 用于验证客户端证书</span></span>
<span class="line">    <span class="token key atrule">min_version</span><span class="token punctuation">:</span> <span class="token string">"TLS1.2"</span></span>
<span class="line">    <span class="token key atrule">max_version</span><span class="token punctuation">:</span> <span class="token string">"TLS1.3"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 数据库配置</span></span>
<span class="line">  <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> auto  <span class="token comment"># auto | postgres | mysql | sqlite</span></span>
<span class="line">    <span class="token key atrule">datasource</span><span class="token punctuation">:</span> <span class="token string">""</span>  <span class="token comment"># DSN/URL</span></span>
<span class="line">    <span class="token comment"># Postgres: postgres://user:pass@host:5432/croupier?sslmode=disable</span></span>
<span class="line">    <span class="token comment"># MySQL: mysql://user:pass@host:3306/croupier?charset=utf8mb4</span></span>
<span class="line">    <span class="token comment"># SQLite: file:data/croupier.db</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 对象存储配置</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> s3  <span class="token comment"># s3 | cos | oss | file</span></span>
<span class="line">    <span class="token key atrule">bucket</span><span class="token punctuation">:</span> <span class="token string">"my-bucket"</span></span>
<span class="line">    <span class="token key atrule">region</span><span class="token punctuation">:</span> <span class="token string">"ap-shanghai"</span></span>
<span class="line">    <span class="token key atrule">endpoint</span><span class="token punctuation">:</span> <span class="token string">"https://cos.ap-shanghai.myqcloud.com"</span></span>
<span class="line">    <span class="token key atrule">access_key</span><span class="token punctuation">:</span> <span class="token string">"${STORAGE_AK}"</span></span>
<span class="line">    <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> <span class="token string">"${STORAGE_SK}"</span></span>
<span class="line">    <span class="token key atrule">force_path_style</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">signed_url_ttl</span><span class="token punctuation">:</span> <span class="token string">"15m"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 日志配置</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"info"</span>  <span class="token comment"># debug | info | warn | error</span></span>
<span class="line">    <span class="token key atrule">format</span><span class="token punctuation">:</span> <span class="token string">"console"</span>  <span class="token comment"># console | json</span></span>
<span class="line">    <span class="token key atrule">file</span><span class="token punctuation">:</span> <span class="token string">""</span>  <span class="token comment"># 日志文件路径</span></span>
<span class="line">    <span class="token key atrule">max_size</span><span class="token punctuation">:</span> <span class="token number">100</span>  <span class="token comment"># MB</span></span>
<span class="line">    <span class="token key atrule">max_backups</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">    <span class="token key atrule">max_age</span><span class="token punctuation">:</span> <span class="token number">7</span>  <span class="token comment"># days</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 指标配置</span></span>
<span class="line">  <span class="token key atrule">metrics</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">per_function</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">per_game_denies</span><span class="token punctuation">:</span> <span class="token boolean important">false</span></span>
<span class="line">    <span class="token key atrule">enable_prometheus</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">prometheus_addr</span><span class="token punctuation">:</span> <span class="token string">":9090"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 审计配置</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">sensitive_fields</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"password"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"token"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"secret"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 认证配置</span></span>
<span class="line">  <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">jwt_secret</span><span class="token punctuation">:</span> <span class="token string">"${JWT_SECRET}"</span></span>
<span class="line">    <span class="token key atrule">jwt_expiry</span><span class="token punctuation">:</span> <span class="token string">"24h"</span></span>
<span class="line">    <span class="token key atrule">oidc</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">false</span></span>
<span class="line">      <span class="token key atrule">issuer</span><span class="token punctuation">:</span> <span class="token string">"https://accounts.example.com"</span></span>
<span class="line">      <span class="token key atrule">client_id</span><span class="token punctuation">:</span> <span class="token string">"${OIDC_CLIENT_ID}"</span></span>
<span class="line">      <span class="token key atrule">client_secret</span><span class="token punctuation">:</span> <span class="token string">"${OIDC_CLIENT_SECRET}"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 环境配置（profiles）</span></span>
<span class="line">  <span class="token key atrule">profiles</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">dev</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"debug"</span></span>
<span class="line">      <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">driver</span><span class="token punctuation">:</span> <span class="token string">"sqlite"</span></span>
<span class="line">        <span class="token key atrule">datasource</span><span class="token punctuation">:</span> <span class="token string">"file:data/dev.db"</span></span>
<span class="line">    <span class="token key atrule">prod</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"info"</span></span>
<span class="line">        <span class="token key atrule">format</span><span class="token punctuation">:</span> <span class="token string">"json"</span></span>
<span class="line">        <span class="token key atrule">file</span><span class="token punctuation">:</span> <span class="token string">"logs/server.log"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="环境变量覆盖" tabindex="-1"><a class="header-anchor" href="#环境变量覆盖"><span>环境变量覆盖</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 覆盖数据库配置</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">DB_DRIVER</span><span class="token operator">=</span>postgres</span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">DATABASE_URL</span><span class="token operator">=</span><span class="token string">"postgres://user:pass@localhost:5432/croupier?sslmode=disable"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 覆盖监听地址</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_SERVER_ADDR</span><span class="token operator">=</span><span class="token string">":9443"</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_SERVER_HTTP_ADDR</span><span class="token operator">=</span><span class="token string">":9080"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 覆盖日志级别</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_SERVER_LOG_LEVEL</span><span class="token operator">=</span><span class="token string">"debug"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="agent-配置" tabindex="-1"><a class="header-anchor" href="#agent-配置"><span>Agent 配置</span></a></h2>
<h3 id="完整配置示例-1" tabindex="-1"><a class="header-anchor" href="#完整配置示例-1"><span>完整配置示例</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># agent.yaml</span></span>
<span class="line"><span class="token key atrule">agent</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token comment"># Server 连接配置</span></span>
<span class="line">  <span class="token key atrule">server_addr</span><span class="token punctuation">:</span> <span class="token string">"localhost:8443"</span></span>
<span class="line">  <span class="token key atrule">server_name</span><span class="token punctuation">:</span> <span class="token string">"croupier.server"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 本地监听地址</span></span>
<span class="line">  <span class="token key atrule">local_addr</span><span class="token punctuation">:</span> <span class="token string">":19090"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 游戏标识</span></span>
<span class="line">  <span class="token key atrule">game_id</span><span class="token punctuation">:</span> <span class="token string">"my-game"</span></span>
<span class="line">  <span class="token key atrule">env</span><span class="token punctuation">:</span> <span class="token string">"dev"</span>  <span class="token comment"># dev | staging | prod</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># TLS 配置</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.key"</span></span>
<span class="line">    <span class="token key atrule">server_name</span><span class="token punctuation">:</span> <span class="token string">"croupier.server"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 心跳配置</span></span>
<span class="line">  <span class="token key atrule">heartbeat_interval</span><span class="token punctuation">:</span> <span class="token string">"30s"</span></span>
<span class="line">  <span class="token key atrule">heartbeat_timeout</span><span class="token punctuation">:</span> <span class="token string">"5m"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 分配配置</span></span>
<span class="line">  <span class="token key atrule">assignments_api</span><span class="token punctuation">:</span> <span class="token string">"http://localhost:8080"</span></span>
<span class="line">  <span class="token key atrule">assignments_poll_sec</span><span class="token punctuation">:</span> <span class="token number">30</span></span>
<span class="line">  <span class="token key atrule">downlink_dir</span><span class="token punctuation">:</span> <span class="token string">"./packs/downlink"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 适配器配置（开发用）</span></span>
<span class="line">  <span class="token key atrule">adapter_prom_cmd</span><span class="token punctuation">:</span> <span class="token string">"go run ./tools/adapters/prom"</span></span>
<span class="line">  <span class="token key atrule">adapter_http_cmd</span><span class="token punctuation">:</span> <span class="token string">"go run ./tools/adapters/http"</span></span>
<span class="line">  <span class="token key atrule">adapter_prom_health_url</span><span class="token punctuation">:</span> <span class="token string">"http://localhost:9091/-/healthy"</span></span>
<span class="line">  <span class="token key atrule">adapter_http_health_url</span><span class="token punctuation">:</span> <span class="token string">"http://localhost:9092/-/healthy"</span></span>
<span class="line">  <span class="token key atrule">adapter_health_interval_sec</span><span class="token punctuation">:</span> <span class="token number">30</span></span>
<span class="line">  <span class="token key atrule">adapter_log_dir</span><span class="token punctuation">:</span> <span class="token string">"logs"</span></span>
<span class="line">  <span class="token key atrule">adapter_log_max_mb</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line">  <span class="token key atrule">adapter_log_backups</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 日志配置</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"info"</span></span>
<span class="line">    <span class="token key atrule">format</span><span class="token punctuation">:</span> <span class="token string">"console"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="edge-配置" tabindex="-1"><a class="header-anchor" href="#edge-配置"><span>Edge 配置</span></a></h2>
<h3 id="完整配置示例-2" tabindex="-1"><a class="header-anchor" href="#完整配置示例-2"><span>完整配置示例</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># edge.yaml</span></span>
<span class="line"><span class="token key atrule">edge</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token comment"># 监听地址</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":8443"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># Server 连接配置</span></span>
<span class="line">  <span class="token key atrule">server_addr</span><span class="token punctuation">:</span> <span class="token string">"internal.server:8443"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># TLS 配置</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/edge.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/edge.key"</span></span>
<span class="line">    <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span></span>
<span class="line">    <span class="token key atrule">server_name</span><span class="token punctuation">:</span> <span class="token string">"croupier.server"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 隧道配置</span></span>
<span class="line">  <span class="token key atrule">tunnel</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">max_connections</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line">    <span class="token key atrule">idle_timeout</span><span class="token punctuation">:</span> <span class="token string">"5m"</span></span>
<span class="line">    <span class="token key atrule">keepalive_interval</span><span class="token punctuation">:</span> <span class="token string">"30s"</span></span>
<span class="line"></span>
<span class="line">  <span class="token comment"># 日志配置</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"info"</span></span>
<span class="line">    <span class="token key atrule">format</span><span class="token punctuation">:</span> <span class="token string">"console"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="配置验证" tabindex="-1"><a class="header-anchor" href="#配置验证"><span>配置验证</span></a></h2>
<h3 id="验证配置文件" tabindex="-1"><a class="header-anchor" href="#验证配置文件"><span>验证配置文件</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 CLI 验证</span></span>
<span class="line">./bin/croupier-server config <span class="token builtin class-name">test</span> <span class="token parameter variable">--config</span> configs/server.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 输出示例</span></span>
<span class="line"><span class="token comment"># ✓ Configuration is valid</span></span>
<span class="line"><span class="token comment"># - server.addr: :8443</span></span>
<span class="line"><span class="token comment"># - server.http_addr: :8080</span></span>
<span class="line"><span class="token comment"># - server.db.driver: postgres</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="常见配置错误" tabindex="-1"><a class="header-anchor" href="#常见配置错误"><span>常见配置错误</span></a></h3>
<table>
<thead>
<tr>
<th>错误</th>
<th>原因</th>
<th>解决方法</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>invalid address</code></td>
<td>端口格式错误</td>
<td>使用 <code v-pre>:port</code> 或 <code v-pre>host:port</code> 格式</td>
</tr>
<tr>
<td><code v-pre>certificate not found</code></td>
<td>证书文件路径错误</td>
<td>检查证书文件是否存在</td>
</tr>
<tr>
<td><code v-pre>database connection failed</code></td>
<td>DSN 格式错误</td>
<td>检查数据库连接字符串格式</td>
</tr>
<tr>
<td><code v-pre>permission denied</code></td>
<td>文件权限不足</td>
<td>检查证书和密钥文件权限</td>
</tr>
</tbody>
</table>
<h2 id="敏感信息处理" tabindex="-1"><a class="header-anchor" href="#敏感信息处理"><span>敏感信息处理</span></a></h2>
<h3 id="使用环境变量" tabindex="-1"><a class="header-anchor" href="#使用环境变量"><span>使用环境变量</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># 不推荐：直接写入配置</span></span>
<span class="line"><span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">access_key</span><span class="token punctuation">:</span> <span class="token string">"AKIDxxxxxxxx"</span></span>
<span class="line">  <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> <span class="token string">"xxxxxxxxxxxx"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 推荐：使用环境变量</span></span>
<span class="line"><span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">access_key</span><span class="token punctuation">:</span> <span class="token string">"${STORAGE_AK}"</span></span>
<span class="line">  <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> <span class="token string">"${STORAGE_SK}"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="环境变量展开" tabindex="-1"><a class="header-anchor" href="#环境变量展开"><span>环境变量展开</span></a></h3>
<p>支持以下展开语法：</p>
<ul>
<li><code v-pre>${VAR}</code> - 简单展开</li>
<li><code v-pre>${VAR:-default}</code> - 带默认值</li>
<li><code v-pre>${VAR:+replacement}</code> - 如果设置了则替换</li>
</ul>
<h2 id="profiles-使用" tabindex="-1"><a class="header-anchor" href="#profiles-使用"><span>Profiles 使用</span></a></h2>
<h3 id="激活-profile" tabindex="-1"><a class="header-anchor" href="#激活-profile"><span>激活 Profile</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 --profile 参数</span></span>
<span class="line">./bin/croupier-server <span class="token parameter variable">--config</span> configs/server.yaml <span class="token parameter variable">--profile</span> prod</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或使用环境变量</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_SERVER_PROFILE</span><span class="token operator">=</span>prod</span>
<span class="line">./bin/croupier-server <span class="token parameter variable">--config</span> configs/server.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="profile-配置示例" tabindex="-1"><a class="header-anchor" href="#profile-配置示例"><span>Profile 配置示例</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"info"</span></span>
<span class="line">  <span class="token key atrule">profiles</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">dev</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"debug"</span></span>
<span class="line">      <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">driver</span><span class="token punctuation">:</span> <span class="token string">"sqlite"</span></span>
<span class="line">    <span class="token key atrule">staging</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"info"</span></span>
<span class="line">      <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">driver</span><span class="token punctuation">:</span> <span class="token string">"postgres"</span></span>
<span class="line">    <span class="token key atrule">prod</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">level</span><span class="token punctuation">:</span> <span class="token string">"warn"</span></span>
<span class="line">        <span class="token key atrule">format</span><span class="token punctuation">:</span> <span class="token string">"json"</span></span>
<span class="line">      <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">driver</span><span class="token punctuation">:</span> <span class="token string">"postgres"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="对象存储配置" tabindex="-1"><a class="header-anchor" href="#对象存储配置"><span>对象存储配置</span></a></h2>
<h3 id="s3-兼容存储" tabindex="-1"><a class="header-anchor" href="#s3-兼容存储"><span>S3 兼容存储</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> s3</span>
<span class="line">    <span class="token key atrule">bucket</span><span class="token punctuation">:</span> <span class="token string">"my-bucket"</span></span>
<span class="line">    <span class="token key atrule">region</span><span class="token punctuation">:</span> <span class="token string">"us-east-1"</span></span>
<span class="line">    <span class="token key atrule">endpoint</span><span class="token punctuation">:</span> <span class="token string">"https://s3.amazonaws.com"</span></span>
<span class="line">    <span class="token key atrule">access_key</span><span class="token punctuation">:</span> <span class="token string">"${AWS_ACCESS_KEY_ID}"</span></span>
<span class="line">    <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> <span class="token string">"${AWS_SECRET_ACCESS_KEY}"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="minio" tabindex="-1"><a class="header-anchor" href="#minio"><span>MinIO</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> s3</span>
<span class="line">    <span class="token key atrule">bucket</span><span class="token punctuation">:</span> <span class="token string">"croupier"</span></span>
<span class="line">    <span class="token key atrule">endpoint</span><span class="token punctuation">:</span> <span class="token string">"http://minio:9000"</span></span>
<span class="line">    <span class="token key atrule">access_key</span><span class="token punctuation">:</span> <span class="token string">"${MINIO_ROOT_USER}"</span></span>
<span class="line">    <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> <span class="token string">"${MINIO_ROOT_PASSWORD}"</span></span>
<span class="line">    <span class="token key atrule">force_path_style</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="腾讯云-cos" tabindex="-1"><a class="header-anchor" href="#腾讯云-cos"><span>腾讯云 COS</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> s3  <span class="token comment"># 或 cos</span></span>
<span class="line">    <span class="token key atrule">bucket</span><span class="token punctuation">:</span> <span class="token string">"bucket-APPID"</span></span>
<span class="line">    <span class="token key atrule">region</span><span class="token punctuation">:</span> <span class="token string">"ap-shanghai"</span></span>
<span class="line">    <span class="token key atrule">endpoint</span><span class="token punctuation">:</span> <span class="token string">"https://cos.ap-shanghai.myqcloud.com"</span></span>
<span class="line">    <span class="token key atrule">access_key</span><span class="token punctuation">:</span> <span class="token string">"${TENCENT_SECRET_ID}"</span></span>
<span class="line">    <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> <span class="token string">"${TENCENT_SECRET_KEY}"</span></span>
<span class="line">    <span class="token key atrule">force_path_style</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="本地文件存储" tabindex="-1"><a class="header-anchor" href="#本地文件存储"><span>本地文件存储</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> file</span>
<span class="line">    <span class="token key atrule">base_dir</span><span class="token punctuation">:</span> <span class="token string">"data/uploads"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="最佳实践" tabindex="-1"><a class="header-anchor" href="#最佳实践"><span>最佳实践</span></a></h2>
<h3 id="_1-分离环境配置" tabindex="-1"><a class="header-anchor" href="#_1-分离环境配置"><span>1. 分离环境配置</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">configs/</span>
<span class="line">├── base.yaml          # 基础配置</span>
<span class="line">├── dev.yaml           # 开发环境</span>
<span class="line">├── staging.yaml       # 预发布环境</span>
<span class="line">└── prod.yaml          # 生产环境</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-使用环境变量管理敏感信息" tabindex="-1"><a class="header-anchor" href="#_2-使用环境变量管理敏感信息"><span>2. 使用环境变量管理敏感信息</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># .env.example</span></span>
<span class="line"><span class="token assign-left variable">JWT_SECRET</span><span class="token operator">=</span>your-jwt-secret-here</span>
<span class="line"><span class="token assign-left variable">DATABASE_URL</span><span class="token operator">=</span>postgres://<span class="token punctuation">..</span>.</span>
<span class="line"><span class="token assign-left variable">STORAGE_AK</span><span class="token operator">=</span>your-access-key</span>
<span class="line"><span class="token assign-left variable">STORAGE_SK</span><span class="token operator">=</span>your-secret-key</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-配置文件模板" tabindex="-1"><a class="header-anchor" href="#_3-配置文件模板"><span>3. 配置文件模板</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># server.example.yaml</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":8443"</span></span>
<span class="line">  <span class="token key atrule">http_addr</span><span class="token punctuation">:</span> <span class="token string">":8080"</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.key"</span></span>
<span class="line">  <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> <span class="token string">"postgres"</span></span>
<span class="line">    <span class="token key atrule">datasource</span><span class="token punctuation">:</span> <span class="token string">"${DATABASE_URL}"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-配置验证" tabindex="-1"><a class="header-anchor" href="#_4-配置验证"><span>4. 配置验证</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># CI/CD 中验证配置</span></span>
<span class="line">./bin/croupier-server config <span class="token builtin class-name">test</span> <span class="token parameter variable">--config</span> configs/server.prod.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="下一步" tabindex="-1"><a class="header-anchor" href="#下一步"><span>下一步</span></a></h2>
<ul>
<li><RouteLink to="/guide/deployment.html">部署指南</RouteLink> - 生产环境部署</li>
<li><RouteLink to="/guide/operations/security.html">安全配置</RouteLink> - 安全相关配置</li>
<li><RouteLink to="/guide/operations/monitoring.html">运维指南</RouteLink> - 监控和日志配置</li>
</ul>
</div></template>


