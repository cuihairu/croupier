<template><div><h1 id="监控指南" tabindex="-1"><a class="header-anchor" href="#监控指南"><span>监控指南</span></a></h1>
<p>本文档介绍 Croupier 的监控、日志和告警配置。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#监控架构">监控架构</router-link></li><li><router-link to="#prometheus-指标">Prometheus 指标</router-link><ul><li><router-link to="#server-指标">Server 指标</router-link></li><li><router-link to="#agent-指标">Agent 指标</router-link></li><li><router-link to="#配置指标采集">配置指标采集</router-link></li></ul></li><li><router-link to="#grafana-面板">Grafana 面板</router-link><ul><li><router-link to="#server-面板">Server 面板</router-link></li></ul></li><li><router-link to="#日志配置">日志配置</router-link><ul><li><router-link to="#日志格式">日志格式</router-link></li><li><router-link to="#日志配置-1">日志配置</router-link></li><li><router-link to="#日志收集-loki">日志收集 (Loki)</router-link></li></ul></li><li><router-link to="#告警规则">告警规则</router-link><ul><li><router-link to="#prometheus-告警规则">Prometheus 告警规则</router-link></li><li><router-link to="#alertmanager-配置">AlertManager 配置</router-link></li></ul></li><li><router-link to="#分布式追踪">分布式追踪</router-link><ul><li><router-link to="#opentelemetry-集成">OpenTelemetry 集成</router-link></li><li><router-link to="#追踪调用">追踪调用</router-link></li></ul></li><li><router-link to="#健康检查">健康检查</router-link><ul><li><router-link to="#http-健康检查">HTTP 健康检查</router-link></li><li><router-link to="#响应示例">响应示例</router-link></li></ul></li><li><router-link to="#性能监控">性能监控</router-link><ul><li><router-link to="#数据库性能">数据库性能</router-link></li><li><router-link to="#redis-性能">Redis 性能</router-link></li></ul></li><li><router-link to="#最佳实践">最佳实践</router-link><ul><li><router-link to="#_1-指标命名规范">1. 指标命名规范</router-link></li><li><router-link to="#_2-标签使用">2. 标签使用</router-link></li><li><router-link to="#_3-日志级别">3. 日志级别</router-link></li><li><router-link to="#_4-敏感信息脱敏">4. 敏感信息脱敏</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="监控架构" tabindex="-1"><a class="header-anchor" href="#监控架构"><span>监控架构</span></a></h2>
<Mermaid code="eJxLL0osyFAIceJSAILi0iQIX8m5KL+0IDO1SOHZnN6nXQuVwNIgEJxaVJZaFA2hbJKK9O2sLA0sDRT0c1NLijKTi2PhKh3TU/NKosEkXJ0hmrrUvBQuNJufz574rG/5y/b2l7PbENYGFOUDNWaklhZHI5g4zXjav/7F8ranPdMQBrgXJaYl5iVGQ2ncWid2vVi7DKHPMSe1qCQ3vSgazPAFak1PLULTDQkMBV1dOyR3gmXAvscmgeCCZaGuwiYFcwAXAE1FiMI="></Mermaid><h2 id="prometheus-指标" tabindex="-1"><a class="header-anchor" href="#prometheus-指标"><span>Prometheus 指标</span></a></h2>
<h3 id="server-指标" tabindex="-1"><a class="header-anchor" href="#server-指标"><span>Server 指标</span></a></h3>
<table>
<thead>
<tr>
<th>指标名称</th>
<th>类型</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>croupier_server_requests_total</code></td>
<td>Counter</td>
<td>请求总数</td>
</tr>
<tr>
<td><code v-pre>croupier_server_request_duration</code></td>
<td>Histogram</td>
<td>请求延迟</td>
</tr>
<tr>
<td><code v-pre>croupier_server_functions_invoked_total</code></td>
<td>Counter</td>
<td>函数调用总数</td>
</tr>
<tr>
<td><code v-pre>croupier_server_agents_connected</code></td>
<td>Gauge</td>
<td>已连接 Agent 数</td>
</tr>
<tr>
<td><code v-pre>croupier_server_jobs_active</code></td>
<td>Gauge</td>
<td>活跃作业数</td>
</tr>
<tr>
<td><code v-pre>croupier_server_approvals_pending</code></td>
<td>Gauge</td>
<td>待审批数</td>
</tr>
</tbody>
</table>
<h3 id="agent-指标" tabindex="-1"><a class="header-anchor" href="#agent-指标"><span>Agent 指标</span></a></h3>
<table>
<thead>
<tr>
<th>指标名称</th>
<th>类型</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>croupier_agent_connected</code></td>
<td>Gauge</td>
<td>Agent 连接状态</td>
</tr>
<tr>
<td><code v-pre>croupier_agent_functions_registered</code></td>
<td>Gauge</td>
<td>已注册函数数</td>
</tr>
<tr>
<td><code v-pre>croupier_agent_jobs_executed_total</code></td>
<td>Counter</td>
<td>执行作业总数</td>
</tr>
<tr>
<td><code v-pre>croupier_agent_jobs_duration</code></td>
<td>Histogram</td>
<td>作业执行时长</td>
</tr>
</tbody>
</table>
<h3 id="配置指标采集" tabindex="-1"><a class="header-anchor" href="#配置指标采集"><span>配置指标采集</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># prometheus.yml</span></span>
<span class="line"><span class="token key atrule">scrape_configs</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">job_name</span><span class="token punctuation">:</span> <span class="token string">'croupier-server'</span></span>
<span class="line">    <span class="token key atrule">static_configs</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">targets</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">'server1:9090'</span><span class="token punctuation">,</span> <span class="token string">'server2:9090'</span><span class="token punctuation">,</span> <span class="token string">'server3:9090'</span><span class="token punctuation">]</span></span>
<span class="line"></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">job_name</span><span class="token punctuation">:</span> <span class="token string">'croupier-agent'</span></span>
<span class="line">    <span class="token key atrule">static_configs</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">targets</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">'agent1:9091'</span><span class="token punctuation">,</span> <span class="token string">'agent2:9091'</span><span class="token punctuation">,</span> <span class="token string">'agent3:9091'</span><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="grafana-面板" tabindex="-1"><a class="header-anchor" href="#grafana-面板"><span>Grafana 面板</span></a></h2>
<h3 id="server-面板" tabindex="-1"><a class="header-anchor" href="#server-面板"><span>Server 面板</span></a></h3>
<p>导入 JSON 配置创建仪表盘：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"dashboard"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"Croupier Server"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"panels"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"请求速率"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"targets"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">          <span class="token punctuation">{</span></span>
<span class="line">            <span class="token property">"expr"</span><span class="token operator">:</span> <span class="token string">"rate(croupier_server_requests_total[5m])"</span></span>
<span class="line">          <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"请求延迟 (P99)"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"targets"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">          <span class="token punctuation">{</span></span>
<span class="line">            <span class="token property">"expr"</span><span class="token operator">:</span> <span class="token string">"histogram_quantile(0.99, rate(croupier_server_request_duration_bucket[5m]))"</span></span>
<span class="line">          <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"title"</span><span class="token operator">:</span> <span class="token string">"函数调用 Top 10"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"targets"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">          <span class="token punctuation">{</span></span>
<span class="line">            <span class="token property">"expr"</span><span class="token operator">:</span> <span class="token string">"topk(10, sum by (function_id) (croupier_server_functions_invoked_total))"</span></span>
<span class="line">          <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">]</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="日志配置" tabindex="-1"><a class="header-anchor" href="#日志配置"><span>日志配置</span></a></h2>
<h3 id="日志格式" tabindex="-1"><a class="header-anchor" href="#日志格式"><span>日志格式</span></a></h3>
<p>Croupier 使用结构化 JSON 日志：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"timestamp"</span><span class="token operator">:</span> <span class="token string">"2024-12-01T10:30:00Z"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"level"</span><span class="token operator">:</span> <span class="token string">"info"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"component"</span><span class="token operator">:</span> <span class="token string">"server"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"msg"</span><span class="token operator">:</span> <span class="token string">"Function invoked"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"player.ban"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"user_id"</span><span class="token operator">:</span> <span class="token string">"user_123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"duration_ms"</span><span class="token operator">:</span> <span class="token number">123</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="日志配置-1" tabindex="-1"><a class="header-anchor" href="#日志配置-1"><span>日志配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> info      <span class="token comment"># debug | info | warn | error</span></span>
<span class="line">    <span class="token key atrule">format</span><span class="token punctuation">:</span> json     <span class="token comment"># console | json</span></span>
<span class="line">    <span class="token key atrule">file</span><span class="token punctuation">:</span> logs/server.log</span>
<span class="line">    <span class="token key atrule">max_size</span><span class="token punctuation">:</span> <span class="token number">100</span>    <span class="token comment"># MB</span></span>
<span class="line">    <span class="token key atrule">max_backups</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">    <span class="token key atrule">max_age</span><span class="token punctuation">:</span> <span class="token number">7</span>       <span class="token comment"># days</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="日志收集-loki" tabindex="-1"><a class="header-anchor" href="#日志收集-loki"><span>日志收集 (Loki)</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># promtail-config.yml</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">http_listen_port</span><span class="token punctuation">:</span> <span class="token number">9080</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">positions</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">filename</span><span class="token punctuation">:</span> /tmp/positions.yaml</span>
<span class="line"></span>
<span class="line"><span class="token key atrule">clients</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">url</span><span class="token punctuation">:</span> http<span class="token punctuation">:</span>//loki<span class="token punctuation">:</span>3100/loki/api/v1/push</span>
<span class="line"></span>
<span class="line"><span class="token key atrule">scrape_configs</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">job_name</span><span class="token punctuation">:</span> croupier</span>
<span class="line">    <span class="token key atrule">static_configs</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">targets</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token punctuation">-</span> localhost</span>
<span class="line">        <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">job</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>server</span>
<span class="line">          <span class="token key atrule">__path__</span><span class="token punctuation">:</span> /var/log/croupier/<span class="token important">*.log</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="告警规则" tabindex="-1"><a class="header-anchor" href="#告警规则"><span>告警规则</span></a></h2>
<h3 id="prometheus-告警规则" tabindex="-1"><a class="header-anchor" href="#prometheus-告警规则"><span>Prometheus 告警规则</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># alerts.yml</span></span>
<span class="line"><span class="token key atrule">groups</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier</span>
<span class="line">    <span class="token key atrule">interval</span><span class="token punctuation">:</span> 30s</span>
<span class="line">    <span class="token key atrule">rules</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token comment"># 服务可用性</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">alert</span><span class="token punctuation">:</span> CroupierServerDown</span>
<span class="line">        <span class="token key atrule">expr</span><span class="token punctuation">:</span> up<span class="token punctuation">{</span>job="croupier<span class="token punctuation">-</span>server"<span class="token punctuation">}</span> == 0</span>
<span class="line">        <span class="token key atrule">for</span><span class="token punctuation">:</span> 1m</span>
<span class="line">        <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">severity</span><span class="token punctuation">:</span> critical</span>
<span class="line">        <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">summary</span><span class="token punctuation">:</span> <span class="token string">"Croupier Server 宕机"</span></span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"{{ $labels.instance }} 已宕机超过 1 分钟"</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># Agent 离线</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">alert</span><span class="token punctuation">:</span> CroupierAgentDisconnected</span>
<span class="line">        <span class="token key atrule">expr</span><span class="token punctuation">:</span> croupier_agent_connected == 0</span>
<span class="line">        <span class="token key atrule">for</span><span class="token punctuation">:</span> 5m</span>
<span class="line">        <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">severity</span><span class="token punctuation">:</span> warning</span>
<span class="line">        <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">summary</span><span class="token punctuation">:</span> <span class="token string">"Croupier Agent 离线"</span></span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"{{ $labels.instance }} 已离线超过 5 分钟"</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># 高错误率</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">alert</span><span class="token punctuation">:</span> HighErrorRate</span>
<span class="line">        <span class="token key atrule">expr</span><span class="token punctuation">:</span> <span class="token punctuation">|</span><span class="token scalar string"></span>
<span class="line">          rate(croupier_server_requests_total{status="error"}[5m])</span>
<span class="line">          /</span>
<span class="line">          rate(croupier_server_requests_total[5m]) > 0.05</span></span>
<span class="line">        <span class="token key atrule">for</span><span class="token punctuation">:</span> 5m</span>
<span class="line">        <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">severity</span><span class="token punctuation">:</span> warning</span>
<span class="line">        <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">summary</span><span class="token punctuation">:</span> <span class="token string">"错误率过高"</span></span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"错误率超过 5%"</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># 高延迟</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">alert</span><span class="token punctuation">:</span> HighLatency</span>
<span class="line">        <span class="token key atrule">expr</span><span class="token punctuation">:</span> <span class="token punctuation">|</span><span class="token scalar string"></span>
<span class="line">          histogram_quantile(0.99,</span>
<span class="line">            rate(croupier_server_request_duration_bucket[5m])</span>
<span class="line">          ) > 1</span></span>
<span class="line">        <span class="token key atrule">for</span><span class="token punctuation">:</span> 5m</span>
<span class="line">        <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">severity</span><span class="token punctuation">:</span> warning</span>
<span class="line">        <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">summary</span><span class="token punctuation">:</span> <span class="token string">"请求延迟过高"</span></span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"P99 延迟超过 1 秒"</span></span>
<span class="line"></span>
<span class="line">      <span class="token comment"># 待审批积压</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">alert</span><span class="token punctuation">:</span> ApprovalBacklog</span>
<span class="line">        <span class="token key atrule">expr</span><span class="token punctuation">:</span> croupier_server_approvals_pending <span class="token punctuation">></span> 50</span>
<span class="line">        <span class="token key atrule">for</span><span class="token punctuation">:</span> 1h</span>
<span class="line">        <span class="token key atrule">labels</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">severity</span><span class="token punctuation">:</span> warning</span>
<span class="line">        <span class="token key atrule">annotations</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">summary</span><span class="token punctuation">:</span> <span class="token string">"审批积压"</span></span>
<span class="line">          <span class="token key atrule">description</span><span class="token punctuation">:</span> <span class="token string">"待审批数量超过 50"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="alertmanager-配置" tabindex="-1"><a class="header-anchor" href="#alertmanager-配置"><span>AlertManager 配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># alertmanager.yml</span></span>
<span class="line"><span class="token key atrule">global</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">resolve_timeout</span><span class="token punctuation">:</span> 5m</span>
<span class="line"></span>
<span class="line"><span class="token key atrule">route</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">group_by</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">'alertname'</span><span class="token punctuation">,</span> <span class="token string">'cluster'</span><span class="token punctuation">,</span> <span class="token string">'service'</span><span class="token punctuation">]</span></span>
<span class="line">  <span class="token key atrule">group_wait</span><span class="token punctuation">:</span> 10s</span>
<span class="line">  <span class="token key atrule">group_interval</span><span class="token punctuation">:</span> 10s</span>
<span class="line">  <span class="token key atrule">repeat_interval</span><span class="token punctuation">:</span> 12h</span>
<span class="line">  <span class="token key atrule">receiver</span><span class="token punctuation">:</span> <span class="token string">'default'</span></span>
<span class="line"></span>
<span class="line">  <span class="token key atrule">routes</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token punctuation">-</span> <span class="token key atrule">match</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">severity</span><span class="token punctuation">:</span> critical</span>
<span class="line">      <span class="token key atrule">receiver</span><span class="token punctuation">:</span> <span class="token string">'pagerduty'</span></span>
<span class="line"></span>
<span class="line">    <span class="token punctuation">-</span> <span class="token key atrule">match</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token key atrule">severity</span><span class="token punctuation">:</span> warning</span>
<span class="line">      <span class="token key atrule">receiver</span><span class="token punctuation">:</span> <span class="token string">'slack'</span></span>
<span class="line"></span>
<span class="line"><span class="token key atrule">receivers</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> <span class="token string">'default'</span></span>
<span class="line">    <span class="token key atrule">webhook_configs</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">url</span><span class="token punctuation">:</span> <span class="token string">'http://webhook:8080/webhook'</span></span>
<span class="line"></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> <span class="token string">'pagerduty'</span></span>
<span class="line">    <span class="token key atrule">pagerduty_configs</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">service_key</span><span class="token punctuation">:</span> <span class="token string">'&lt;PAGERDUTY_SERVICE_KEY>'</span></span>
<span class="line"></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> <span class="token string">'slack'</span></span>
<span class="line">    <span class="token key atrule">slack_configs</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">api_url</span><span class="token punctuation">:</span> <span class="token string">'&lt;SLACK_WEBHOOK_URL>'</span></span>
<span class="line">        <span class="token key atrule">channel</span><span class="token punctuation">:</span> <span class="token string">'#alerts'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="分布式追踪" tabindex="-1"><a class="header-anchor" href="#分布式追踪"><span>分布式追踪</span></a></h2>
<h3 id="opentelemetry-集成" tabindex="-1"><a class="header-anchor" href="#opentelemetry-集成"><span>OpenTelemetry 集成</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"go.opentelemetry.io/otel"</span></span>
<span class="line">    <span class="token string">"go.opentelemetry.io/otel/exporters/jaeger"</span></span>
<span class="line">    <span class="token string">"go.opentelemetry.io/otel/sdk/trace"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">initTracer</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    exporter<span class="token punctuation">,</span> err <span class="token operator">:=</span> jaeger<span class="token punctuation">.</span><span class="token function">New</span><span class="token punctuation">(</span>jaeger<span class="token punctuation">.</span><span class="token function">WithCollectorEndpoint</span><span class="token punctuation">(</span></span>
<span class="line">        jaeger<span class="token punctuation">.</span><span class="token function">WithEndpoint</span><span class="token punctuation">(</span><span class="token string">"http://jaeger:14268/api/traces"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> err</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    tp <span class="token operator">:=</span> trace<span class="token punctuation">.</span><span class="token function">NewTracerProvider</span><span class="token punctuation">(</span></span>
<span class="line">        trace<span class="token punctuation">.</span><span class="token function">WithBatcher</span><span class="token punctuation">(</span>exporter<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        trace<span class="token punctuation">.</span><span class="token function">WithResource</span><span class="token punctuation">(</span>resources<span class="token punctuation">.</span><span class="token function">NewWithAttributes</span><span class="token punctuation">(</span></span>
<span class="line">            semconv<span class="token punctuation">.</span>SchemaURL<span class="token punctuation">,</span></span>
<span class="line">            semconv<span class="token punctuation">.</span><span class="token function">ServiceName</span><span class="token punctuation">(</span><span class="token string">"croupier-server"</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    otel<span class="token punctuation">.</span><span class="token function">SetTracerProvider</span><span class="token punctuation">(</span>tp<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="追踪调用" tabindex="-1"><a class="header-anchor" href="#追踪调用"><span>追踪调用</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"go.opentelemetry.io/otel"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>s <span class="token operator">*</span>Server<span class="token punctuation">)</span> <span class="token function">InvokeFunction</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> req <span class="token operator">*</span>Request<span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token operator">*</span>Response<span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    tracer <span class="token operator">:=</span> otel<span class="token punctuation">.</span><span class="token function">Tracer</span><span class="token punctuation">(</span><span class="token string">"server"</span><span class="token punctuation">)</span></span>
<span class="line">    ctx<span class="token punctuation">,</span> span <span class="token operator">:=</span> tracer<span class="token punctuation">.</span><span class="token function">Start</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> <span class="token string">"InvokeFunction"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">defer</span> span<span class="token punctuation">.</span><span class="token function">End</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    span<span class="token punctuation">.</span><span class="token function">SetAttributes</span><span class="token punctuation">(</span></span>
<span class="line">        attribute<span class="token punctuation">.</span><span class="token function">String</span><span class="token punctuation">(</span><span class="token string">"function_id"</span><span class="token punctuation">,</span> req<span class="token punctuation">.</span>FunctionId<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        attribute<span class="token punctuation">.</span><span class="token function">String</span><span class="token punctuation">(</span><span class="token string">"game_id"</span><span class="token punctuation">,</span> req<span class="token punctuation">.</span>GameId<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 调用 Agent</span></span>
<span class="line">    resp<span class="token punctuation">,</span> err <span class="token operator">:=</span> s<span class="token punctuation">.</span>agent<span class="token punctuation">.</span><span class="token function">InvokeFunction</span><span class="token punctuation">(</span>ctx<span class="token punctuation">,</span> req<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        span<span class="token punctuation">.</span><span class="token function">RecordError</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token boolean">nil</span><span class="token punctuation">,</span> err</span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> resp<span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="健康检查" tabindex="-1"><a class="header-anchor" href="#健康检查"><span>健康检查</span></a></h2>
<h3 id="http-健康检查" tabindex="-1"><a class="header-anchor" href="#http-健康检查"><span>HTTP 健康检查</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 健康检查</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/healthz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 就绪检查</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/readyz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="响应示例" tabindex="-1"><a class="header-anchor" href="#响应示例"><span>响应示例</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"ok"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"checks"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"database"</span><span class="token operator">:</span> <span class="token string">"ok"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"redis"</span><span class="token operator">:</span> <span class="token string">"ok"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"agents"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"total"</span><span class="token operator">:</span> <span class="token number">5</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"online"</span><span class="token operator">:</span> <span class="token number">5</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="性能监控" tabindex="-1"><a class="header-anchor" href="#性能监控"><span>性能监控</span></a></h2>
<h3 id="数据库性能" tabindex="-1"><a class="header-anchor" href="#数据库性能"><span>数据库性能</span></a></h3>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token comment">-- 慢查询</span></span>
<span class="line"><span class="token keyword">SELECT</span> query<span class="token punctuation">,</span> mean_exec_time<span class="token punctuation">,</span> calls</span>
<span class="line"><span class="token keyword">FROM</span> pg_stat_statements</span>
<span class="line"><span class="token keyword">WHERE</span> mean_exec_time <span class="token operator">></span> <span class="token number">100</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> mean_exec_time <span class="token keyword">DESC</span></span>
<span class="line"><span class="token keyword">LIMIT</span> <span class="token number">10</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">-- 连接数</span></span>
<span class="line"><span class="token keyword">SELECT</span> <span class="token function">count</span><span class="token punctuation">(</span><span class="token operator">*</span><span class="token punctuation">)</span> <span class="token keyword">FROM</span> pg_stat_activity<span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="redis-性能" tabindex="-1"><a class="header-anchor" href="#redis-性能"><span>Redis 性能</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Redis 信息</span></span>
<span class="line">redis-cli INFO stats</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 慢日志</span></span>
<span class="line">redis-cli SLOWLOG GET <span class="token number">10</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="最佳实践" tabindex="-1"><a class="header-anchor" href="#最佳实践"><span>最佳实践</span></a></h2>
<h3 id="_1-指标命名规范" tabindex="-1"><a class="header-anchor" href="#_1-指标命名规范"><span>1. 指标命名规范</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">croupier_&lt;component>_&lt;metric>_&lt;unit></span>
<span class="line"></span>
<span class="line">示例：</span>
<span class="line">- croupier_server_requests_total</span>
<span class="line">- croupier_agent_functions_registered</span>
<span class="line">- croupier_job_duration_seconds</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-标签使用" tabindex="-1"><a class="header-anchor" href="#_2-标签使用"><span>2. 标签使用</span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 添加标签</span></span>
<span class="line">counter<span class="token punctuation">.</span><span class="token function">WithLabelValues</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"player.ban"</span><span class="token punctuation">,</span>  <span class="token comment">// function_id</span></span>
<span class="line">    <span class="token string">"my-game"</span><span class="token punctuation">,</span>     <span class="token comment">// game_id</span></span>
<span class="line">    <span class="token string">"prod"</span><span class="token punctuation">,</span>        <span class="token comment">// env</span></span>
<span class="line">    <span class="token string">"success"</span><span class="token punctuation">,</span>     <span class="token comment">// status</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">Inc</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-日志级别" tabindex="-1"><a class="header-anchor" href="#_3-日志级别"><span>3. 日志级别</span></a></h3>
<table>
<thead>
<tr>
<th>级别</th>
<th>用途</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>debug</code></td>
<td>开发调试信息</td>
</tr>
<tr>
<td><code v-pre>info</code></td>
<td>正常操作日志</td>
</tr>
<tr>
<td><code v-pre>warn</code></td>
<td>警告信息</td>
</tr>
<tr>
<td><code v-pre>error</code></td>
<td>错误信息</td>
</tr>
</tbody>
</table>
<h3 id="_4-敏感信息脱敏" tabindex="-1"><a class="header-anchor" href="#_4-敏感信息脱敏"><span>4. 敏感信息脱敏</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">sensitive_fields</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"password"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"token"</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token string">"secret"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/deployment.html">部署指南</RouteLink></li>
<li><RouteLink to="/guide/operations/security.html">安全配置</RouteLink></li>
<li><RouteLink to="/guide/operations/troubleshooting.html">故障排查</RouteLink></li>
</ul>
</div></template>


