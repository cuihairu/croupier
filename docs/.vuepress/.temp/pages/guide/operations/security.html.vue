<template><div><h1 id="安全配置" tabindex="-1"><a class="header-anchor" href="#安全配置"><span>安全配置</span></a></h1>
<p>本文档介绍 Croupier 的安全配置和最佳实践。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#安全架构">安全架构</router-link></li><li><router-link to="#tls-mtls-配置">TLS/mTLS 配置</router-link><ul><li><router-link to="#证书结构">证书结构</router-link></li><li><router-link to="#生成-ca">生成 CA</router-link></li><li><router-link to="#生成服务器证书">生成服务器证书</router-link></li><li><router-link to="#生成-agent-证书">生成 Agent 证书</router-link></li><li><router-link to="#server-配置">Server 配置</router-link></li><li><router-link to="#agent-配置">Agent 配置</router-link></li></ul></li><li><router-link to="#认证配置">认证配置</router-link><ul><li><router-link to="#jwt-配置">JWT 配置</router-link></li><li><router-link to="#jwt-token-示例">JWT Token 示例</router-link></li><li><router-link to="#oidc-配置">OIDC 配置</router-link></li><li><router-link to="#totp-双因素认证">TOTP 双因素认证</router-link></li></ul></li><li><router-link to="#权限配置">权限配置</router-link><ul><li><router-link to="#rbac-角色">RBAC 角色</router-link></li><li><router-link to="#abac-策略">ABAC 策略</router-link></li></ul></li><li><router-link to="#审批配置">审批配置</router-link><ul><li><router-link to="#双人规则">双人规则</router-link></li><li><router-link to="#审批存储">审批存储</router-link></li></ul></li><li><router-link to="#审计日志">审计日志</router-link><ul><li><router-link to="#审计配置">审计配置</router-link></li><li><router-link to="#审计链防篡改">审计链防篡改</router-link></li></ul></li><li><router-link to="#网络安全">网络安全</router-link><ul><li><router-link to="#防火墙配置">防火墙配置</router-link></li><li><router-link to="#ddos-防护">DDoS 防护</router-link></li></ul></li><li><router-link to="#数据加密">数据加密</router-link><ul><li><router-link to="#数据库加密">数据库加密</router-link></li><li><router-link to="#敏感字段加密">敏感字段加密</router-link></li></ul></li><li><router-link to="#安全检查清单">安全检查清单</router-link><ul><li><router-link to="#部署前检查">部署前检查</router-link></li><li><router-link to="#定期检查">定期检查</router-link></li></ul></li><li><router-link to="#故障排查">故障排查</router-link><ul><li><router-link to="#tls-握手失败">TLS 握手失败</router-link></li><li><router-link to="#认证失败">认证失败</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="安全架构" tabindex="-1"><a class="header-anchor" href="#安全架构"><span>安全架构</span></a></h2>
<Mermaid code="eJxLL0osyFAIceJSAILi0iQIX+npukXPOrY/X71eCSwBAqGe0S6JxRlJ+YlFKTZJRfp2HiEhAcGxYPnUvBQuNBOCU4vKUosQ2kN8gqNzgYTCy1U9L9Y3QvSBgGNpSUb0i3VLgILP+jqezW1GlkrJLIl+um7hi3ULn01f+nT/dJzWvWyY9WT/wqcbmxA2gmwzjIa4Q+FR2xQFx/TUvBKw00FSCGtAPCNkha4p6alo6rBY+Wzqhme961CsdM1LLqosKInWgMp1LXi6vk0TzYRQTwVdXbsacPDVgMIFLAoKGqAwODjAAiAGVAQYCihqwD7DEDHiAgDc7Zc+"></Mermaid><h2 id="tls-mtls-配置" tabindex="-1"><a class="header-anchor" href="#tls-mtls-配置"><span>TLS/mTLS 配置</span></a></h2>
<h3 id="证书结构" tabindex="-1"><a class="header-anchor" href="#证书结构"><span>证书结构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">                            Root CA (ca.crt)</span>
<span class="line">                                 |</span>
<span class="line">                +----------------+----------------+</span>
<span class="line">                |                                 |</span>
<span class="line">        Server CA                         Agent CA</span>
<span class="line">                |                                 |</span>
<span class="line">        +-------+-------+                 +-------+-------+</span>
<span class="line">        |               |                 |               |</span>
<span class="line">   Server.crt     Edge.crt         Agent1.crt     Agent2.crt</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="生成-ca" tabindex="-1"><a class="header-anchor" href="#生成-ca"><span>生成 CA</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 生成根 CA</span></span>
<span class="line">openssl genrsa <span class="token parameter variable">-out</span> ca.key <span class="token number">4096</span></span>
<span class="line">openssl req <span class="token parameter variable">-new</span> <span class="token parameter variable">-x509</span> <span class="token parameter variable">-days</span> <span class="token number">3650</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-key</span> ca.key <span class="token parameter variable">-out</span> ca.crt <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-subj</span> <span class="token string">"/CN=Croupier Root CA/O=Croupier/C=CN"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="生成服务器证书" tabindex="-1"><a class="header-anchor" href="#生成服务器证书"><span>生成服务器证书</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 生成私钥</span></span>
<span class="line">openssl genrsa <span class="token parameter variable">-out</span> server.key <span class="token number">4096</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 生成 CSR</span></span>
<span class="line">openssl req <span class="token parameter variable">-new</span> <span class="token parameter variable">-key</span> server.key <span class="token parameter variable">-out</span> server.csr <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-subj</span> <span class="token string">"/CN=server.example.com/O=Croupier/C=CN"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 签发证书</span></span>
<span class="line">openssl x509 <span class="token parameter variable">-req</span> <span class="token parameter variable">-days</span> <span class="token number">365</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-in</span> server.csr <span class="token parameter variable">-CA</span> ca.crt <span class="token parameter variable">-CAkey</span> ca.key <span class="token parameter variable">-CAcreateserial</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-out</span> server.crt <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-extfile</span> <span class="token operator">&lt;</span><span class="token punctuation">(</span><span class="token builtin class-name">echo</span> <span class="token string">"subjectAltName=DNS:server.example.com,DNS:*.server.example.com"</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="生成-agent-证书" tabindex="-1"><a class="header-anchor" href="#生成-agent-证书"><span>生成 Agent 证书</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 生成私钥</span></span>
<span class="line">openssl genrsa <span class="token parameter variable">-out</span> agent.key <span class="token number">4096</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 生成 CSR</span></span>
<span class="line">openssl req <span class="token parameter variable">-new</span> <span class="token parameter variable">-key</span> agent.key <span class="token parameter variable">-out</span> agent.csr <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-subj</span> <span class="token string">"/CN=agent-1/O=Croupier/C=CN"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 签发证书</span></span>
<span class="line">openssl x509 <span class="token parameter variable">-req</span> <span class="token parameter variable">-days</span> <span class="token number">365</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-in</span> agent.csr <span class="token parameter variable">-CA</span> ca.crt <span class="token parameter variable">-CAkey</span> ca.key <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-out</span> agent.crt</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="server-配置" tabindex="-1"><a class="header-anchor" href="#server-配置"><span>Server 配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.key"</span></span>
<span class="line">    <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span>  <span class="token comment"># 用于验证客户端证书</span></span>
<span class="line">    <span class="token key atrule">min_version</span><span class="token punctuation">:</span> <span class="token string">"TLS1.2"</span></span>
<span class="line">    <span class="token key atrule">max_version</span><span class="token punctuation">:</span> <span class="token string">"TLS1.3"</span></span>
<span class="line">    <span class="token key atrule">cipher_suites</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"</span></span>
<span class="line">    <span class="token key atrule">client_auth</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">mode</span><span class="token punctuation">:</span> <span class="token string">"require_and_verify"</span>  <span class="token comment"># 要求并验证客户端证书</span></span>
<span class="line">      <span class="token key atrule">ca_files</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token string">"data/agent-ca.crt"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="agent-配置" tabindex="-1"><a class="header-anchor" href="#agent-配置"><span>Agent 配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">agent</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.key"</span></span>
<span class="line">    <span class="token key atrule">server_name</span><span class="token punctuation">:</span> <span class="token string">"server.example.com"</span></span>
<span class="line">    <span class="token key atrule">min_version</span><span class="token punctuation">:</span> <span class="token string">"TLS1.2"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="认证配置" tabindex="-1"><a class="header-anchor" href="#认证配置"><span>认证配置</span></a></h2>
<h3 id="jwt-配置" tabindex="-1"><a class="header-anchor" href="#jwt-配置"><span>JWT 配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">jwt_secret</span><span class="token punctuation">:</span> <span class="token string">"${JWT_SECRET}"</span>  <span class="token comment"># 至少 32 字符</span></span>
<span class="line">    <span class="token key atrule">jwt_expiry</span><span class="token punctuation">:</span> <span class="token string">"24h"</span></span>
<span class="line">    <span class="token key atrule">jwt_refresh_expiry</span><span class="token punctuation">:</span> <span class="token string">"168h"</span>  <span class="token comment"># 7 天</span></span>
<span class="line">    <span class="token key atrule">issuer</span><span class="token punctuation">:</span> <span class="token string">"croupier"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="jwt-token-示例" tabindex="-1"><a class="header-anchor" href="#jwt-token-示例"><span>JWT Token 示例</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"header"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"alg"</span><span class="token operator">:</span> <span class="token string">"HS256"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"typ"</span><span class="token operator">:</span> <span class="token string">"JWT"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"payload"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"username"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"roles"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"exp"</span><span class="token operator">:</span> <span class="token number">1733140800</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"iat"</span><span class="token operator">:</span> <span class="token number">1733054400</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"iss"</span><span class="token operator">:</span> <span class="token string">"croupier"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="oidc-配置" tabindex="-1"><a class="header-anchor" href="#oidc-配置"><span>OIDC 配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">oidc</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">issuer</span><span class="token punctuation">:</span> <span class="token string">"https://accounts.example.com"</span></span>
<span class="line">      <span class="token key atrule">client_id</span><span class="token punctuation">:</span> <span class="token string">"${OIDC_CLIENT_ID}"</span></span>
<span class="line">      <span class="token key atrule">client_secret</span><span class="token punctuation">:</span> <span class="token string">"${OIDC_CLIENT_SECRET}"</span></span>
<span class="line">      <span class="token key atrule">redirect_url</span><span class="token punctuation">:</span> <span class="token string">"https://croupier.example.com/auth/callback"</span></span>
<span class="line">      <span class="token key atrule">scopes</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token string">"openid"</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token string">"profile"</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token string">"email"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="totp-双因素认证" tabindex="-1"><a class="header-anchor" href="#totp-双因素认证"><span>TOTP 双因素认证</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">auth</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">totp</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">issuer</span><span class="token punctuation">:</span> <span class="token string">"Croupier"</span></span>
<span class="line">      <span class="token key atrule">period</span><span class="token punctuation">:</span> <span class="token number">30</span></span>
<span class="line">      <span class="token key atrule">digits</span><span class="token punctuation">:</span> <span class="token number">6</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="权限配置" tabindex="-1"><a class="header-anchor" href="#权限配置"><span>权限配置</span></a></h2>
<h3 id="rbac-角色" tabindex="-1"><a class="header-anchor" href="#rbac-角色"><span>RBAC 角色</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"role_id"</span><span class="token operator">:</span> <span class="token string">"admin"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"管理员"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"*.*"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"role_id"</span><span class="token operator">:</span> <span class="token string">"gm"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"游戏管理员"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token string">"player.*"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"item.*"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"guild.*"</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"role_id"</span><span class="token operator">:</span> <span class="token string">"viewer"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"查看者"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"permissions"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token string">"player.view"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"item.view"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"guild.view"</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="abac-策略" tabindex="-1"><a class="header-anchor" href="#abac-策略"><span>ABAC 策略</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"permission"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"allow_if"</span><span class="token operator">:</span> <span class="token string">"has_role('admin') || (has_role('gm') &amp;&amp; env == 'dev')"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="审批配置" tabindex="-1"><a class="header-anchor" href="#审批配置"><span>审批配置</span></a></h2>
<h3 id="双人规则" tabindex="-1"><a class="header-anchor" href="#双人规则"><span>双人规则</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"two_person_rule"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"approval"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"enabled"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"threshold"</span><span class="token operator">:</span> <span class="token number">2</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"approvers"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"admin"</span><span class="token punctuation">,</span> <span class="token string">"senior_gm"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"timeout"</span><span class="token operator">:</span> <span class="token string">"24h"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审批存储" tabindex="-1"><a class="header-anchor" href="#审批存储"><span>审批存储</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">approval_storage</span><span class="token punctuation">:</span> <span class="token string">"postgres"</span>  <span class="token comment"># memory | postgres | sqlite</span></span>
<span class="line">    <span class="token key atrule">approval_db</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">dsn</span><span class="token punctuation">:</span> <span class="token string">"postgres://user:pass@localhost:5432/croupier"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="审计日志" tabindex="-1"><a class="header-anchor" href="#审计日志"><span>审计日志</span></a></h2>
<h3 id="审计配置" tabindex="-1"><a class="header-anchor" href="#审计配置"><span>审计配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token comment"># 敏感字段脱敏</span></span>
<span class="line">    <span class="token key atrule">sensitive_fields</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"password"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"token"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"secret"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"api_key"</span></span>
<span class="line">    <span class="token comment"># 审计保留天数</span></span>
<span class="line">    <span class="token key atrule">retention_days</span><span class="token punctuation">:</span> <span class="token number">365</span></span>
<span class="line">    <span class="token comment"># 备份配置</span></span>
<span class="line">    <span class="token key atrule">backup_enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">backup_location</span><span class="token punctuation">:</span> <span class="token string">"s3://audit-logs/"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="审计链防篡改" tabindex="-1"><a class="header-anchor" href="#审计链防篡改"><span>审计链防篡改</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">type</span> AuditLog <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    AuditID  <span class="token builtin">string</span></span>
<span class="line">    Previous <span class="token builtin">string</span>  <span class="token comment">// 前一条记录的哈希</span></span>
<span class="line">    Hash     <span class="token builtin">string</span>  <span class="token comment">// 本条记录的哈希</span></span>
<span class="line">    Content  <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token builtin">byte</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>a <span class="token operator">*</span>AuditLog<span class="token punctuation">)</span> <span class="token function">ComputeHash</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">string</span> <span class="token punctuation">{</span></span>
<span class="line">    h <span class="token operator">:=</span> sha256<span class="token punctuation">.</span><span class="token function">New</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    h<span class="token punctuation">.</span><span class="token function">Write</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token function">byte</span><span class="token punctuation">(</span>a<span class="token punctuation">.</span>Previous<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">    h<span class="token punctuation">.</span><span class="token function">Write</span><span class="token punctuation">(</span>a<span class="token punctuation">.</span>Content<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> hex<span class="token punctuation">.</span><span class="token function">EncodeToString</span><span class="token punctuation">(</span>h<span class="token punctuation">.</span><span class="token function">Sum</span><span class="token punctuation">(</span><span class="token boolean">nil</span><span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="网络安全" tabindex="-1"><a class="header-anchor" href="#网络安全"><span>网络安全</span></a></h2>
<h3 id="防火墙配置" tabindex="-1"><a class="header-anchor" href="#防火墙配置"><span>防火墙配置</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Server</span></span>
<span class="line">ufw default deny incoming</span>
<span class="line">ufw default allow outgoing</span>
<span class="line">ufw allow <span class="token number">22</span>/tcp      <span class="token comment"># SSH</span></span>
<span class="line">ufw allow <span class="token number">443</span>/tcp     <span class="token comment"># HTTPS</span></span>
<span class="line">ufw allow <span class="token number">8443</span>/tcp    <span class="token comment"># gRPC</span></span>
<span class="line">ufw allow <span class="token number">8080</span>/tcp    <span class="token comment"># HTTP</span></span>
<span class="line">ufw <span class="token builtin class-name">enable</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="ddos-防护" tabindex="-1"><a class="header-anchor" href="#ddos-防护"><span>DDoS 防护</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">http</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">rate_limit</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">requests_per_second</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line">      <span class="token key atrule">burst</span><span class="token punctuation">:</span> <span class="token number">200</span></span>
<span class="line">    <span class="token key atrule">ip_whitelist</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"10.0.0.0/8"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"192.168.0.0/16"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="数据加密" tabindex="-1"><a class="header-anchor" href="#数据加密"><span>数据加密</span></a></h2>
<h3 id="数据库加密" tabindex="-1"><a class="header-anchor" href="#数据库加密"><span>数据库加密</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">dsn</span><span class="token punctuation">:</span> <span class="token string">"postgres://user:pass@localhost:5432/croupier?sslmode=require"</span></span>
<span class="line">    <span class="token key atrule">ssl</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/client.crt"</span></span>
<span class="line">      <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/client.key"</span></span>
<span class="line">      <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="敏感字段加密" tabindex="-1"><a class="header-anchor" href="#敏感字段加密"><span>敏感字段加密</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">type</span> User <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    UserID   <span class="token builtin">string</span></span>
<span class="line">    Username <span class="token builtin">string</span></span>
<span class="line">    Password <span class="token builtin">string</span> <span class="token string">`encrypt:"true"`</span></span>
<span class="line">    APIKey   <span class="token builtin">string</span> <span class="token string">`encrypt:"true"`</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="安全检查清单" tabindex="-1"><a class="header-anchor" href="#安全检查清单"><span>安全检查清单</span></a></h2>
<h3 id="部署前检查" tabindex="-1"><a class="header-anchor" href="#部署前检查"><span>部署前检查</span></a></h3>
<ul>
<li>[ ] 所有组件使用 mTLS</li>
<li>[ ] JWT Secret 足够复杂</li>
<li>[ ] 启用了双因素认证</li>
<li>[ ] 配置了双人规则</li>
<li>[ ] 审计日志已启用</li>
<li>[ ] 敏感字段已脱敏</li>
<li>[ ] 数据库连接加密</li>
<li>[ ] 防火墙已配置</li>
<li>[ ] 限流已启用</li>
</ul>
<h3 id="定期检查" tabindex="-1"><a class="header-anchor" href="#定期检查"><span>定期检查</span></a></h3>
<ul>
<li>[ ] 证书有效期检查</li>
<li>[ ] 审计日志完整性检查</li>
<li>[ ] 权限审查</li>
<li>[ ] 安全漏洞扫描</li>
</ul>
<h2 id="故障排查" tabindex="-1"><a class="header-anchor" href="#故障排查"><span>故障排查</span></a></h2>
<h3 id="tls-握手失败" tabindex="-1"><a class="header-anchor" href="#tls-握手失败"><span>TLS 握手失败</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 测试 TLS 连接</span></span>
<span class="line">openssl s_client <span class="token parameter variable">-connect</span> server:8443 <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-cert</span> agent.crt <span class="token parameter variable">-key</span> agent.key <span class="token parameter variable">-CAfile</span> ca.crt</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查证书</span></span>
<span class="line">openssl x509 <span class="token parameter variable">-in</span> server.crt <span class="token parameter variable">-text</span> <span class="token parameter variable">-noout</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="认证失败" tabindex="-1"><a class="header-anchor" href="#认证失败"><span>认证失败</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 解码 JWT</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"eyJhbGci..."</span> <span class="token operator">|</span> jq <span class="token parameter variable">-R</span> <span class="token string">'split(".") | .[1] | @base64d | fromjson'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/guide/concepts/permissions.html">权限控制</RouteLink></li>
<li><RouteLink to="/guide/configuration.html">配置管理</RouteLink></li>
<li><RouteLink to="/guide/operations/monitoring.html">监控指南</RouteLink></li>
</ul>
</div></template>


