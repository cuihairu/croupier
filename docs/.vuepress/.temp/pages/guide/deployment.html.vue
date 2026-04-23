<template><div><h1 id="部署指南" tabindex="-1"><a class="header-anchor" href="#部署指南"><span>部署指南</span></a></h1>
<p>本文档介绍 Croupier 在生产环境中的部署方案和最佳实践。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#部署架构">部署架构</router-link><ul><li><router-link to="#推荐架构">推荐架构</router-link></li></ul></li><li><router-link to="#部署模式">部署模式</router-link><ul><li><router-link to="#单机房部署">单机房部署</router-link></li><li><router-link to="#多机房部署">多机房部署</router-link></li></ul></li><li><router-link to="#docker-部署">Docker 部署</router-link><ul><li><router-link to="#server-部署">Server 部署</router-link></li><li><router-link to="#agent-部署">Agent 部署</router-link></li></ul></li><li><router-link to="#kubernetes-部署">Kubernetes 部署</router-link><ul><li><router-link to="#namespace">Namespace</router-link></li><li><router-link to="#configmap">ConfigMap</router-link></li><li><router-link to="#secret">Secret</router-link></li><li><router-link to="#server-deployment">Server Deployment</router-link></li><li><router-link to="#service">Service</router-link></li><li><router-link to="#ingress">Ingress</router-link></li></ul></li><li><router-link to="#二进制部署">二进制部署</router-link><ul><li><router-link to="#系统服务配置">系统服务配置</router-link></li></ul></li><li><router-link to="#高可用配置">高可用配置</router-link><ul><li><router-link to="#数据库高可用">数据库高可用</router-link></li><li><router-link to="#redis-高可用">Redis 高可用</router-link></li></ul></li><li><router-link to="#负载均衡">负载均衡</router-link><ul><li><router-link to="#nginx-配置">Nginx 配置</router-link></li></ul></li><li><router-link to="#监控配置">监控配置</router-link><ul><li><router-link to="#prometheus">Prometheus</router-link></li><li><router-link to="#grafana-dashboard">Grafana Dashboard</router-link></li></ul></li><li><router-link to="#备份策略">备份策略</router-link><ul><li><router-link to="#数据库备份">数据库备份</router-link></li><li><router-link to="#配置备份">配置备份</router-link></li></ul></li><li><router-link to="#安全加固">安全加固</router-link><ul><li><router-link to="#tls-证书">TLS 证书</router-link></li><li><router-link to="#防火墙">防火墙</router-link></li></ul></li><li><router-link to="#健康检查">健康检查</router-link></li><li><router-link to="#故障排查">故障排查</router-link><ul><li><router-link to="#常见问题">常见问题</router-link></li><li><router-link to="#日志查看">日志查看</router-link></li></ul></li><li><router-link to="#下一步">下一步</router-link></li></ul></nav>
<h2 id="部署架构" tabindex="-1"><a class="header-anchor" href="#部署架构"><span>部署架构</span></a></h2>
<h3 id="推荐架构" tabindex="-1"><a class="header-anchor" href="#推荐架构"><span>推荐架构</span></a></h3>
<Mermaid code="eJxLL0osyFAIceJSAILi0iQIX8nFN0pBX+Fp65rneyc+7dmlBJYGAR+n6Bdb5r/Yu/fp3PYXCxc+nbnCJqlI384vPTOvQt/RxykWrtI1JT01GkQoBBTlV1SClT3tX/+yoROiJjUvhQvN1qdtrUD7FHQVnq9b+HxCG4rFwalFZalFHo7RzkX5pQWZqUVQEbC5L1fPABr9fMqKl7Pbnu9bgnCEi1O0RkB+cUl6UWpwoA9Y7ZMdu5/s7tNEqAlKTcksjtYAU2AVz/dMfrp2hv7LGfOfdkyHKsTn2mc7djzr6EdxrWN6al6JYTSYUjAEmwpR9WxO79MuULApOCJcAFZmBFVthFU1UsCClRlDVRtjVe2M5mgfJwVdXTtwlID54GgBicBCFVkVihiMA5ZxgSQTFDFwsGEKQwIAh7gRDnFjLgBy4ulx"></Mermaid><h2 id="部署模式" tabindex="-1"><a class="header-anchor" href="#部署模式"><span>部署模式</span></a></h2>
<h3 id="单机房部署" tabindex="-1"><a class="header-anchor" href="#单机房部署"><span>单机房部署</span></a></h3>
<p>适用于小规模或单一区域部署。</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────┐</span>
<span class="line">│           数据中心                       │</span>
<span class="line">│  ┌─────────────────────────────────┐   │</span>
<span class="line">│  │  管理网段 (内网)                │   │</span>
<span class="line">│  │  Server HA (3 节点)            │   │</span>
<span class="line">│  │         │                       │   │</span>
<span class="line">│  │    PostgreSQL + Redis          │   │</span>
<span class="line">│  └─────────────────────────────────┘   │</span>
<span class="line">│  ┌─────────────────────────────────┐   │</span>
<span class="line">│  │  游戏网段 (内网)                │   │</span>
<span class="line">│  │  Agent + Game Server (多节点)  │   │</span>
<span class="line">│  └─────────────────────────────────┘   │</span>
<span class="line">└─────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="多机房部署" tabindex="-1"><a class="header-anchor" href="#多机房部署"><span>多机房部署</span></a></h3>
<p>适用于跨区域部署，需要 Edge 组件。</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌──────────────┐       ┌──────────────┐</span>
<span class="line">│  机房 A       │       │  机房 B       │</span>
<span class="line">│  Server HA   │◄─────►│  Server HA   │</span>
<span class="line">│  │           │       │  │           │</span>
<span class="line">│  Agent       │       │  Agent       │</span>
<span class="line">│  │           │       │  │           │</span>
<span class="line">│  Game Server │       │  Game Server │</span>
<span class="line">└──────────────┘       └──────────────┘</span>
<span class="line">       │                       │</span>
<span class="line">       └───────────┬───────────┘</span>
<span class="line">                   │</span>
<span class="line">              ┌─────────┐</span>
<span class="line">              │  Edge   │</span>
<span class="line">              │  (DMZ)  │</span>
<span class="line">              └─────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="docker-部署" tabindex="-1"><a class="header-anchor" href="#docker-部署"><span>Docker 部署</span></a></h2>
<h3 id="server-部署" tabindex="-1"><a class="header-anchor" href="#server-部署"><span>Server 部署</span></a></h3>
<h4 id="docker-compose" tabindex="-1"><a class="header-anchor" href="#docker-compose"><span>Docker Compose</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># docker-compose.yml</span></span>
<span class="line"><span class="token key atrule">version</span><span class="token punctuation">:</span> <span class="token string">'3.8'</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">services</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">postgres</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">image</span><span class="token punctuation">:</span> postgres<span class="token punctuation">:</span><span class="token number">16</span></span>
<span class="line">    <span class="token key atrule">environment</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">POSTGRES_DB</span><span class="token punctuation">:</span> croupier</span>
<span class="line">      <span class="token key atrule">POSTGRES_USER</span><span class="token punctuation">:</span> croupier</span>
<span class="line">      <span class="token key atrule">POSTGRES_PASSWORD</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>POSTGRES_PASSWORD<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> postgres_data<span class="token punctuation">:</span>/var/lib/postgresql/data</span>
<span class="line">    <span class="token key atrule">ports</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"5432:5432"</span></span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">redis</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">image</span><span class="token punctuation">:</span> redis<span class="token punctuation">:</span>7<span class="token punctuation">-</span>alpine</span>
<span class="line">    <span class="token key atrule">ports</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"6379:6379"</span></span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">image</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server<span class="token punctuation">:</span>latest</span>
<span class="line">    <span class="token key atrule">ports</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"8443:8443"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"8080:8080"</span></span>
<span class="line">    <span class="token key atrule">environment</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> DATABASE_URL=postgres<span class="token punctuation">:</span>//croupier<span class="token punctuation">:</span>$<span class="token punctuation">{</span>POSTGRES_PASSWORD<span class="token punctuation">}</span>@postgres<span class="token punctuation">:</span>5432/croupier<span class="token punctuation">?</span>sslmode=disable</span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_SERVER_ADDR=<span class="token punctuation">:</span><span class="token number">8443</span></span>
<span class="line">      <span class="token punctuation">-</span> CROUPIER_SERVER_HTTP_ADDR=<span class="token punctuation">:</span><span class="token number">8080</span></span>
<span class="line">    <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> ./configs<span class="token punctuation">:</span>/app/configs</span>
<span class="line">      <span class="token punctuation">-</span> ./data<span class="token punctuation">:</span>/app/data</span>
<span class="line">    <span class="token key atrule">depends_on</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> postgres</span>
<span class="line">      <span class="token punctuation">-</span> redis</span>
<span class="line"></span>
<span class="line"><span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">postgres_data</span><span class="token punctuation">:</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="单独部署" tabindex="-1"><a class="header-anchor" href="#单独部署"><span>单独部署</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 构建镜像</span></span>
<span class="line"><span class="token function">docker</span> build <span class="token parameter variable">-t</span> croupier-server:latest <span class="token parameter variable">-f</span> docker/Dockerfile.server <span class="token builtin class-name">.</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 运行容器</span></span>
<span class="line"><span class="token function">docker</span> run <span class="token parameter variable">-d</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">--name</span> croupier-server <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-p</span> <span class="token number">8443</span>:8443 <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-p</span> <span class="token number">8080</span>:8080 <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-v</span> <span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">pwd</span><span class="token variable">)</span></span>/data:/app/data <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-v</span> <span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">pwd</span><span class="token variable">)</span></span>/configs:/app/configs <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-e</span> <span class="token assign-left variable">DATABASE_URL</span><span class="token operator">=</span><span class="token string">"postgres://croupier:password@postgres:5432/croupier"</span> <span class="token punctuation">\</span></span>
<span class="line">  croupier-server:latest</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="agent-部署" tabindex="-1"><a class="header-anchor" href="#agent-部署"><span>Agent 部署</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 运行 Agent 容器</span></span>
<span class="line"><span class="token function">docker</span> run <span class="token parameter variable">-d</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">--name</span> croupier-agent <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">--network</span> <span class="token function">host</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-v</span> <span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">pwd</span><span class="token variable">)</span></span>/configs:/app/configs <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-e</span> <span class="token assign-left variable">CROUPIER_AGENT_SERVER_ADDR</span><span class="token operator">=</span><span class="token string">"server:8443"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-e</span> <span class="token assign-left variable">CROUPIER_AGENT_GAME_ID</span><span class="token operator">=</span><span class="token string">"my-game"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-e</span> <span class="token assign-left variable">CROUPIER_AGENT_ENV</span><span class="token operator">=</span><span class="token string">"prod"</span> <span class="token punctuation">\</span></span>
<span class="line">  croupier-agent:latest</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="kubernetes-部署" tabindex="-1"><a class="header-anchor" href="#kubernetes-部署"><span>Kubernetes 部署</span></a></h2>
<h3 id="namespace" tabindex="-1"><a class="header-anchor" href="#namespace"><span>Namespace</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># namespace.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Namespace</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="configmap" tabindex="-1"><a class="header-anchor" href="#configmap"><span>ConfigMap</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># configmap.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> ConfigMap</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>config</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> croupier</span>
<span class="line"><span class="token key atrule">data</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">server.yaml</span><span class="token punctuation">:</span> <span class="token punctuation">|</span><span class="token scalar string"></span>
<span class="line">    server:</span>
<span class="line">      addr: ":8443"</span>
<span class="line">      http_addr: ":8080"</span>
<span class="line">      db:</span>
<span class="line">        driver: postgres</span>
<span class="line">      log:</span>
<span class="line">        level: info</span>
<span class="line">        format: json</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="secret" tabindex="-1"><a class="header-anchor" href="#secret"><span>Secret</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># secret.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Secret</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>secret</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> croupier</span>
<span class="line"><span class="token key atrule">type</span><span class="token punctuation">:</span> Opaque</span>
<span class="line"><span class="token key atrule">stringData</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">database-url</span><span class="token punctuation">:</span> <span class="token string">"postgres://croupier:password@postgres:5432/croupier"</span></span>
<span class="line">  <span class="token key atrule">jwt-secret</span><span class="token punctuation">:</span> <span class="token string">"your-jwt-secret"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="server-deployment" tabindex="-1"><a class="header-anchor" href="#server-deployment"><span>Server Deployment</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># server-deployment.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> apps/v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Deployment</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> croupier</span>
<span class="line"><span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">replicas</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">  <span class="token key atrule">selector</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">matchLabels</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">app</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server</span>
<span class="line">  <span class="token key atrule">template</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">app</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server</span>
<span class="line">    <span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">containers</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> server</span>
<span class="line">        <span class="token key atrule">image</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server<span class="token punctuation">:</span>latest</span>
<span class="line">        <span class="token key atrule">ports</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">containerPort</span><span class="token punctuation">:</span> <span class="token number">8443</span></span>
<span class="line">          <span class="token key atrule">name</span><span class="token punctuation">:</span> grpc</span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">containerPort</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">          <span class="token key atrule">name</span><span class="token punctuation">:</span> http</span>
<span class="line">        <span class="token key atrule">env</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> DATABASE_URL</span>
<span class="line">          <span class="token key atrule">valueFrom</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">secretKeyRef</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>secret</span>
<span class="line">              <span class="token key atrule">key</span><span class="token punctuation">:</span> database<span class="token punctuation">-</span>url</span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> JWT_SECRET</span>
<span class="line">          <span class="token key atrule">valueFrom</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">secretKeyRef</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>secret</span>
<span class="line">              <span class="token key atrule">key</span><span class="token punctuation">:</span> jwt<span class="token punctuation">-</span>secret</span>
<span class="line">        <span class="token key atrule">volumeMounts</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> config</span>
<span class="line">          <span class="token key atrule">mountPath</span><span class="token punctuation">:</span> /app/configs</span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> data</span>
<span class="line">          <span class="token key atrule">mountPath</span><span class="token punctuation">:</span> /app/data</span>
<span class="line">        <span class="token key atrule">resources</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">requests</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">memory</span><span class="token punctuation">:</span> <span class="token string">"256Mi"</span></span>
<span class="line">            <span class="token key atrule">cpu</span><span class="token punctuation">:</span> <span class="token string">"500m"</span></span>
<span class="line">          <span class="token key atrule">limits</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">memory</span><span class="token punctuation">:</span> <span class="token string">"1Gi"</span></span>
<span class="line">            <span class="token key atrule">cpu</span><span class="token punctuation">:</span> <span class="token string">"2000m"</span></span>
<span class="line">        <span class="token key atrule">livenessProbe</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">httpGet</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">path</span><span class="token punctuation">:</span> /healthz</span>
<span class="line">            <span class="token key atrule">port</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">          <span class="token key atrule">initialDelaySeconds</span><span class="token punctuation">:</span> <span class="token number">10</span></span>
<span class="line">          <span class="token key atrule">periodSeconds</span><span class="token punctuation">:</span> <span class="token number">30</span></span>
<span class="line">        <span class="token key atrule">readinessProbe</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">httpGet</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">path</span><span class="token punctuation">:</span> /readyz</span>
<span class="line">            <span class="token key atrule">port</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">          <span class="token key atrule">initialDelaySeconds</span><span class="token punctuation">:</span> <span class="token number">5</span></span>
<span class="line">          <span class="token key atrule">periodSeconds</span><span class="token punctuation">:</span> <span class="token number">10</span></span>
<span class="line">      <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> config</span>
<span class="line">        <span class="token key atrule">configMap</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>config</span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> data</span>
<span class="line">        <span class="token key atrule">persistentVolumeClaim</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">claimName</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>data</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="service" tabindex="-1"><a class="header-anchor" href="#service"><span>Service</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># service.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Service</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> croupier</span>
<span class="line"><span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">selector</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">app</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server</span>
<span class="line">  <span class="token key atrule">ports</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> grpc</span>
<span class="line">    <span class="token key atrule">port</span><span class="token punctuation">:</span> <span class="token number">8443</span></span>
<span class="line">    <span class="token key atrule">targetPort</span><span class="token punctuation">:</span> <span class="token number">8443</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> http</span>
<span class="line">    <span class="token key atrule">port</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">    <span class="token key atrule">targetPort</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line">  <span class="token key atrule">type</span><span class="token punctuation">:</span> ClusterIP</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="ingress" tabindex="-1"><a class="header-anchor" href="#ingress"><span>Ingress</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># ingress.yaml</span></span>
<span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> networking.k8s.io/v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Ingress</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>ingress</span>
<span class="line">  <span class="token key atrule">namespace</span><span class="token punctuation">:</span> croupier</span>
<span class="line">  <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">nginx.ingress.kubernetes.io/grpc-backend</span><span class="token punctuation">:</span> <span class="token string">"true"</span></span>
<span class="line">    <span class="token key atrule">nginx.ingress.kubernetes.io/ssl-redirect</span><span class="token punctuation">:</span> <span class="token string">"true"</span></span>
<span class="line">    <span class="token key atrule">cert-manager.io/cluster-issuer</span><span class="token punctuation">:</span> <span class="token string">"letsencrypt-prod"</span></span>
<span class="line"><span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">ingressClassName</span><span class="token punctuation">:</span> nginx</span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">hosts</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token punctuation">-</span> croupier.example.com</span>
<span class="line">    <span class="token key atrule">secretName</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>tls</span>
<span class="line">  <span class="token key atrule">rules</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">host</span><span class="token punctuation">:</span> croupier.example.com</span>
<span class="line">    <span class="token key atrule">http</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">paths</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">path</span><span class="token punctuation">:</span> /</span>
<span class="line">        <span class="token key atrule">pathType</span><span class="token punctuation">:</span> Prefix</span>
<span class="line">        <span class="token key atrule">backend</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">service</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server</span>
<span class="line">            <span class="token key atrule">port</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">number</span><span class="token punctuation">:</span> <span class="token number">8080</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="二进制部署" tabindex="-1"><a class="header-anchor" href="#二进制部署"><span>二进制部署</span></a></h2>
<h3 id="系统服务配置" tabindex="-1"><a class="header-anchor" href="#系统服务配置"><span>系统服务配置</span></a></h3>
<h4 id="systemd-linux" tabindex="-1"><a class="header-anchor" href="#systemd-linux"><span>systemd (Linux)</span></a></h4>
<div class="language-ini line-numbers-mode" data-highlighter="prismjs" data-ext="ini"><pre v-pre><code class="language-ini"><span class="line"><span class="token comment"># /etc/systemd/system/croupier-server.service</span></span>
<span class="line"><span class="token section"><span class="token punctuation">[</span><span class="token section-name selector">Unit</span><span class="token punctuation">]</span></span></span>
<span class="line"><span class="token key attr-name">Description</span><span class="token punctuation">=</span><span class="token value attr-value">Croupier Server</span></span>
<span class="line"><span class="token key attr-name">After</span><span class="token punctuation">=</span><span class="token value attr-value">network.target postgresql.service</span></span>
<span class="line"></span>
<span class="line"><span class="token section"><span class="token punctuation">[</span><span class="token section-name selector">Service</span><span class="token punctuation">]</span></span></span>
<span class="line"><span class="token key attr-name">Type</span><span class="token punctuation">=</span><span class="token value attr-value">simple</span></span>
<span class="line"><span class="token key attr-name">User</span><span class="token punctuation">=</span><span class="token value attr-value">croupier</span></span>
<span class="line"><span class="token key attr-name">Group</span><span class="token punctuation">=</span><span class="token value attr-value">croupier</span></span>
<span class="line"><span class="token key attr-name">WorkingDirectory</span><span class="token punctuation">=</span><span class="token value attr-value">/opt/croupier</span></span>
<span class="line"><span class="token key attr-name">ExecStart</span><span class="token punctuation">=</span><span class="token value attr-value">/opt/croupier/bin/croupier-server --config /etc/croupier/server.yaml</span></span>
<span class="line"><span class="token key attr-name">Restart</span><span class="token punctuation">=</span><span class="token value attr-value">always</span></span>
<span class="line"><span class="token key attr-name">RestartSec</span><span class="token punctuation">=</span><span class="token value attr-value">5</span></span>
<span class="line"></span>
<span class="line"><span class="token key attr-name">Environment</span><span class="token punctuation">=</span><span class="token value attr-value">"<span class="token inner-value">DATABASE_URL=postgres://croupier:password@localhost:5432/croupier</span>"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 安全加固</span></span>
<span class="line"><span class="token key attr-name">NoNewPrivileges</span><span class="token punctuation">=</span><span class="token value attr-value">true</span></span>
<span class="line"><span class="token key attr-name">PrivateTmp</span><span class="token punctuation">=</span><span class="token value attr-value">true</span></span>
<span class="line"><span class="token key attr-name">ProtectSystem</span><span class="token punctuation">=</span><span class="token value attr-value">strict</span></span>
<span class="line"><span class="token key attr-name">ProtectHome</span><span class="token punctuation">=</span><span class="token value attr-value">true</span></span>
<span class="line"><span class="token key attr-name">ReadWritePaths</span><span class="token punctuation">=</span><span class="token value attr-value">/var/log/croupier /var/lib/croupier</span></span>
<span class="line"></span>
<span class="line"><span class="token section"><span class="token punctuation">[</span><span class="token section-name selector">Install</span><span class="token punctuation">]</span></span></span>
<span class="line"><span class="token key attr-name">WantedBy</span><span class="token punctuation">=</span><span class="token value attr-value">multi-user.target</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>启动服务：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 重载配置</span></span>
<span class="line"><span class="token function">sudo</span> systemctl daemon-reload</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启用并启动服务</span></span>
<span class="line"><span class="token function">sudo</span> systemctl <span class="token builtin class-name">enable</span> <span class="token parameter variable">--now</span> croupier-server</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 查看状态</span></span>
<span class="line"><span class="token function">sudo</span> systemctl status croupier-server</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 查看日志</span></span>
<span class="line"><span class="token function">sudo</span> journalctl <span class="token parameter variable">-u</span> croupier-server <span class="token parameter variable">-f</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="高可用配置" tabindex="-1"><a class="header-anchor" href="#高可用配置"><span>高可用配置</span></a></h2>
<h3 id="数据库高可用" tabindex="-1"><a class="header-anchor" href="#数据库高可用"><span>数据库高可用</span></a></h3>
<h4 id="postgresql-主从复制" tabindex="-1"><a class="header-anchor" href="#postgresql-主从复制"><span>PostgreSQL 主从复制</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 主库配置 (postgresql.conf)</span></span>
<span class="line">wal_level <span class="token operator">=</span> replica</span>
<span class="line">max_wal_senders <span class="token operator">=</span> <span class="token number">5</span></span>
<span class="line">wal_keep_size <span class="token operator">=</span> 1GB</span>
<span class="line"></span>
<span class="line"><span class="token comment"># pg_hba.conf</span></span>
<span class="line"><span class="token function">host</span>    replication     replicator      <span class="token number">0.0</span>.0.0/0      md5</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="连接池-pgbouncer" tabindex="-1"><a class="header-anchor" href="#连接池-pgbouncer"><span>连接池 (PgBouncer)</span></a></h4>
<div class="language-ini line-numbers-mode" data-highlighter="prismjs" data-ext="ini"><pre v-pre><code class="language-ini"><span class="line"><span class="token section"><span class="token punctuation">[</span><span class="token section-name selector">databases</span><span class="token punctuation">]</span></span></span>
<span class="line"><span class="token key attr-name">croupier</span> <span class="token punctuation">=</span> <span class="token value attr-value">host=localhost port=5432 dbname=croupier</span></span>
<span class="line"></span>
<span class="line"><span class="token section"><span class="token punctuation">[</span><span class="token section-name selector">pgbouncer</span><span class="token punctuation">]</span></span></span>
<span class="line"><span class="token key attr-name">listen_addr</span> <span class="token punctuation">=</span> <span class="token value attr-value">0.0.0.0</span></span>
<span class="line"><span class="token key attr-name">listen_port</span> <span class="token punctuation">=</span> <span class="token value attr-value">6432</span></span>
<span class="line"><span class="token key attr-name">auth_type</span> <span class="token punctuation">=</span> <span class="token value attr-value">md5</span></span>
<span class="line"><span class="token key attr-name">auth_file</span> <span class="token punctuation">=</span> <span class="token value attr-value">/etc/pgbouncer/userlist.txt</span></span>
<span class="line"><span class="token key attr-name">pool_mode</span> <span class="token punctuation">=</span> <span class="token value attr-value">transaction</span></span>
<span class="line"><span class="token key attr-name">max_client_conn</span> <span class="token punctuation">=</span> <span class="token value attr-value">1000</span></span>
<span class="line"><span class="token key attr-name">default_pool_size</span> <span class="token punctuation">=</span> <span class="token value attr-value">50</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="redis-高可用" tabindex="-1"><a class="header-anchor" href="#redis-高可用"><span>Redis 高可用</span></a></h3>
<h4 id="redis-sentinel" tabindex="-1"><a class="header-anchor" href="#redis-sentinel"><span>Redis Sentinel</span></a></h4>
<div class="language-conf line-numbers-mode" data-highlighter="prismjs" data-ext="conf"><pre v-pre><code class="language-conf"><span class="line"># sentinel.conf</span>
<span class="line">port 26379</span>
<span class="line">sentinel monitor mymaster 127.0.0.1 6379 2</span>
<span class="line">sentinel down-after-milliseconds mymaster 5000</span>
<span class="line">sentinel parallel-syncs mymaster 1</span>
<span class="line">sentinel failover-timeout mymaster 10000</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="负载均衡" tabindex="-1"><a class="header-anchor" href="#负载均衡"><span>负载均衡</span></a></h2>
<h3 id="nginx-配置" tabindex="-1"><a class="header-anchor" href="#nginx-配置"><span>Nginx 配置</span></a></h3>
<div class="language-nginx line-numbers-mode" data-highlighter="prismjs" data-ext="nginx"><pre v-pre><code class="language-nginx"><span class="line"><span class="token directive"><span class="token keyword">upstream</span> croupier_grpc</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">least_conn</span></span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server</span> server1:8443 max_fails=3 fail_timeout=30s</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server</span> server2:8443 max_fails=3 fail_timeout=30s</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server</span> server3:8443 max_fails=3 fail_timeout=30s</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token directive"><span class="token keyword">upstream</span> croupier_http</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">least_conn</span></span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server</span> server1:8080 max_fails=3 fail_timeout=30s</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server</span> server2:8080 max_fails=3 fail_timeout=30s</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server</span> server3:8080 max_fails=3 fail_timeout=30s</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># gRPC 代理</span></span>
<span class="line"><span class="token directive"><span class="token keyword">server</span></span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">listen</span> <span class="token number">8443</span> ssl http2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server_name</span> croupier.example.com</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token directive"><span class="token keyword">ssl_certificate</span> /etc/nginx/ssl/server.crt</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">ssl_certificate_key</span> /etc/nginx/ssl/server.key</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token directive"><span class="token keyword">location</span> /</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token directive"><span class="token keyword">grpc_pass</span> grpc://croupier_grpc</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token directive"><span class="token keyword">grpc_set_header</span> X-Real-IP <span class="token variable">$remote_addr</span></span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># HTTP 代理</span></span>
<span class="line"><span class="token directive"><span class="token keyword">server</span></span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">listen</span> <span class="token number">8080</span> ssl</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">server_name</span> croupier.example.com</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token directive"><span class="token keyword">ssl_certificate</span> /etc/nginx/ssl/server.crt</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token directive"><span class="token keyword">ssl_certificate_key</span> /etc/nginx/ssl/server.key</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token directive"><span class="token keyword">location</span> /</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token directive"><span class="token keyword">proxy_pass</span> http://croupier_http</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token directive"><span class="token keyword">proxy_set_header</span> Host <span class="token variable">$host</span></span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token directive"><span class="token keyword">proxy_set_header</span> X-Real-IP <span class="token variable">$remote_addr</span></span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token directive"><span class="token keyword">proxy_set_header</span> X-Forwarded-For <span class="token variable">$proxy_add_x_forwarded_for</span></span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="监控配置" tabindex="-1"><a class="header-anchor" href="#监控配置"><span>监控配置</span></a></h2>
<h3 id="prometheus" tabindex="-1"><a class="header-anchor" href="#prometheus"><span>Prometheus</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># prometheus.yml</span></span>
<span class="line"><span class="token key atrule">scrape_configs</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">job_name</span><span class="token punctuation">:</span> <span class="token string">'croupier-server'</span></span>
<span class="line">    <span class="token key atrule">static_configs</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">targets</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">'server1:9090'</span><span class="token punctuation">,</span> <span class="token string">'server2:9090'</span><span class="token punctuation">,</span> <span class="token string">'server3:9090'</span><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="grafana-dashboard" tabindex="-1"><a class="header-anchor" href="#grafana-dashboard"><span>Grafana Dashboard</span></a></h3>
<p>导入 Croupier 提供的 Grafana 面板配置：</p>
<ul>
<li>Server 指标面板</li>
<li>Agent 连接面板</li>
<li>函数调用面板</li>
<li>错误率面板</li>
</ul>
<h2 id="备份策略" tabindex="-1"><a class="header-anchor" href="#备份策略"><span>备份策略</span></a></h2>
<h3 id="数据库备份" tabindex="-1"><a class="header-anchor" href="#数据库备份"><span>数据库备份</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 每日全量备份</span></span>
<span class="line"><span class="token number">0</span> <span class="token number">2</span> * * * pg_dump <span class="token parameter variable">-U</span> croupier croupier <span class="token operator">|</span> <span class="token function">gzip</span> <span class="token operator">></span> /backup/croupier_<span class="token variable"><span class="token variable">$(</span><span class="token function">date</span> +<span class="token punctuation">\</span>%Y<span class="token punctuation">\</span>%m<span class="token punctuation">\</span>%d<span class="token variable">)</span></span>.sql.gz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 保留 30 天</span></span>
<span class="line"><span class="token function">find</span> /backup <span class="token parameter variable">-name</span> <span class="token string">"croupier_*.sql.gz"</span> <span class="token parameter variable">-mtime</span> +30 <span class="token parameter variable">-delete</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="配置备份" tabindex="-1"><a class="header-anchor" href="#配置备份"><span>配置备份</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 备份配置文件</span></span>
<span class="line"><span class="token function">rsync</span> <span class="token parameter variable">-av</span> /etc/croupier/ /backup/configs/</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="安全加固" tabindex="-1"><a class="header-anchor" href="#安全加固"><span>安全加固</span></a></h2>
<h3 id="tls-证书" tabindex="-1"><a class="header-anchor" href="#tls-证书"><span>TLS 证书</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 生成 CA</span></span>
<span class="line">openssl genrsa <span class="token parameter variable">-out</span> ca.key <span class="token number">4096</span></span>
<span class="line">openssl req <span class="token parameter variable">-new</span> <span class="token parameter variable">-x509</span> <span class="token parameter variable">-days</span> <span class="token number">3650</span> <span class="token parameter variable">-key</span> ca.key <span class="token parameter variable">-out</span> ca.crt</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 生成服务器证书</span></span>
<span class="line">openssl genrsa <span class="token parameter variable">-out</span> server.key <span class="token number">4096</span></span>
<span class="line">openssl req <span class="token parameter variable">-new</span> <span class="token parameter variable">-key</span> server.key <span class="token parameter variable">-out</span> server.csr</span>
<span class="line">openssl x509 <span class="token parameter variable">-req</span> <span class="token parameter variable">-days</span> <span class="token number">365</span> <span class="token parameter variable">-in</span> server.csr <span class="token parameter variable">-CA</span> ca.crt <span class="token parameter variable">-CAkey</span> ca.key <span class="token parameter variable">-CAcreateserial</span> <span class="token parameter variable">-out</span> server.crt</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 生成 Agent 证书</span></span>
<span class="line">openssl genrsa <span class="token parameter variable">-out</span> agent.key <span class="token number">4096</span></span>
<span class="line">openssl req <span class="token parameter variable">-new</span> <span class="token parameter variable">-key</span> agent.key <span class="token parameter variable">-out</span> agent.csr</span>
<span class="line">openssl x509 <span class="token parameter variable">-req</span> <span class="token parameter variable">-days</span> <span class="token number">365</span> <span class="token parameter variable">-in</span> agent.csr <span class="token parameter variable">-CA</span> ca.crt <span class="token parameter variable">-CAkey</span> ca.key <span class="token parameter variable">-out</span> agent.crt</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="防火墙" tabindex="-1"><a class="header-anchor" href="#防火墙"><span>防火墙</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Server</span></span>
<span class="line">ufw allow <span class="token number">22</span>/tcp      <span class="token comment"># SSH</span></span>
<span class="line">ufw allow <span class="token number">8080</span>/tcp    <span class="token comment"># HTTP</span></span>
<span class="line">ufw allow <span class="token number">8443</span>/tcp    <span class="token comment"># gRPC</span></span>
<span class="line">ufw <span class="token builtin class-name">enable</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Agent (出站连接即可)</span></span>
<span class="line">ufw allow out <span class="token number">8443</span>/tcp</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="健康检查" tabindex="-1"><a class="header-anchor" href="#健康检查"><span>健康检查</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># HTTP 健康检查</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/healthz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 预期输出</span></span>
<span class="line"><span class="token comment"># {"status":"ok"}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 就绪检查</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/readyz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 版本信息</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/version</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="故障排查" tabindex="-1"><a class="header-anchor" href="#故障排查"><span>故障排查</span></a></h2>
<h3 id="常见问题" tabindex="-1"><a class="header-anchor" href="#常见问题"><span>常见问题</span></a></h3>
<table>
<thead>
<tr>
<th>问题</th>
<th>可能原因</th>
<th>解决方法</th>
</tr>
</thead>
<tbody>
<tr>
<td>连接拒绝</td>
<td>端口未监听</td>
<td>检查配置和防火墙</td>
</tr>
<tr>
<td>TLS 握手失败</td>
<td>证书过期/不匹配</td>
<td>更新证书</td>
</tr>
<tr>
<td>数据库连接失败</td>
<td>网络或认证问题</td>
<td>检查 DATABASE_URL</td>
</tr>
<tr>
<td>Agent 无法注册</td>
<td>Server 地址错误</td>
<td>检查 server_addr</td>
</tr>
</tbody>
</table>
<h3 id="日志查看" tabindex="-1"><a class="header-anchor" href="#日志查看"><span>日志查看</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Server 日志</span></span>
<span class="line">journalctl <span class="token parameter variable">-u</span> croupier-server <span class="token parameter variable">-n</span> <span class="token number">100</span> <span class="token parameter variable">-f</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Agent 日志</span></span>
<span class="line">journalctl <span class="token parameter variable">-u</span> croupier-agent <span class="token parameter variable">-n</span> <span class="token number">100</span> <span class="token parameter variable">-f</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Docker 日志</span></span>
<span class="line"><span class="token function">docker</span> logs <span class="token parameter variable">-f</span> croupier-server</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Kubernetes 日志</span></span>
<span class="line">kubectl logs <span class="token parameter variable">-f</span> deployment/croupier-server <span class="token parameter variable">-n</span> croupier</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="下一步" tabindex="-1"><a class="header-anchor" href="#下一步"><span>下一步</span></a></h2>
<ul>
<li><RouteLink to="/guide/operations/monitoring.html">监控指南</RouteLink> - 监控和告警配置</li>
<li><RouteLink to="/guide/operations/security.html">安全配置</RouteLink> - 安全加固指南</li>
<li><RouteLink to="/guide/operations/troubleshooting.html">故障排查</RouteLink> - 常见问题解决</li>
</ul>
</div></template>


