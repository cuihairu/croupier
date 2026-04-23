<template><div><h1 id="metrics-observability" tabindex="-1"><a class="header-anchor" href="#metrics-observability"><span>Metrics &amp; Observability</span></a></h1>
<p>This doc lists the built-in metrics endpoints and exported series for Server/Agent/Edge.</p>
<p>Endpoints</p>
<ul>
<li>JSON metrics
<ul>
<li>Server: GET /metrics</li>
<li>Agent:  GET /metrics</li>
<li>Edge:   GET /metrics</li>
</ul>
</li>
<li>Prometheus text format
<ul>
<li>Server: GET /metrics.prom</li>
<li>Agent:  GET /metrics.prom</li>
<li>Edge:   GET /metrics.prom</li>
</ul>
</li>
</ul>
<p>Server JSON (/metrics)</p>
<ul>
<li>uptime_sec</li>
<li>invocations_total / invocations_error_total</li>
<li>jobs_started_total / jobs_error_total</li>
<li>rbac_denied_total / audit_errors_total</li>
<li>logs: <code v-pre>{ debug, info, warn, error, total }</code></li>
<li>lb_stats, conn_pool (when available)</li>
<li>functions: per-function snapshot
<ul>
<li>invocations_total / errors_total / rbac_denied_total</li>
<li>latency_seconds: <code v-pre>{ buckets[], counts[], sum, count }</code></li>
</ul>
</li>
</ul>
<p>Server Prometheus (/metrics.prom)</p>
<ul>
<li>croupier_invocations_total</li>
<li>croupier_invocations_error_total</li>
<li>croupier_jobs_started_total</li>
<li>croupier_jobs_error_total</li>
<li>croupier_rbac_denied_total</li>
<li>croupier_audit_errors_total</li>
<li><code v-pre>croupier_logs_total{level=&quot;debug|info|warn|error&quot;}</code></li>
<li>Per-function series
<ul>
<li><code v-pre>croupier_invocations_total{function_id=&quot;...&quot;}</code></li>
<li><code v-pre>croupier_invocations_error_total{function_id=&quot;...&quot;}</code></li>
<li><code v-pre>croupier_rbac_denied_total{function_id=&quot;...&quot;}</code></li>
<li><code v-pre>croupier_invoke_latency_seconds_bucket{function_id=&quot;...&quot;,le=&quot;...&quot;}</code></li>
<li><code v-pre>croupier_invoke_latency_seconds_sum{function_id=&quot;...&quot;}</code></li>
<li><code v-pre>croupier_invoke_latency_seconds_count{function_id=&quot;...&quot;}</code></li>
</ul>
</li>
</ul>
<p>Agent JSON (/metrics)</p>
<ul>
<li>functions, instances, tunnel_reconnects</li>
<li>logs</li>
</ul>
<p>Agent Prometheus (/metrics.prom)</p>
<ul>
<li><code v-pre>croupier_agent_instances</code></li>
<li><code v-pre>croupier_tunnel_reconnects</code></li>
<li><code v-pre>croupier_logs_total{level}</code></li>
</ul>
<p>Edge JSON (/metrics)</p>
<ul>
<li>tunnel metrics map + logs</li>
</ul>
<p>Edge Prometheus (/metrics.prom)</p>
<ul>
<li><code v-pre>croupier_logs_total{level}</code></li>
</ul>
<p>Notes</p>
<ul>
<li>Histogram buckets follow Prometheus defaults (0.005 .. 10 seconds). Values are best-effort for HTTP path and meant for dashboards/alerts.</li>
<li>Series cardinality: per-function metrics may increase cardinality; keep function ids bounded.</li>
<li>Toggles: you can disable per-function metrics via <code v-pre>--metrics.per_function=false</code> and enable per-game RBAC denied counters via <code v-pre>--metrics.per_game_denies=true</code>.</li>
</ul>
<p>Prometheus scrape example</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">scrape_configs</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">job_name</span><span class="token punctuation">:</span> <span class="token string">'croupier-server'</span></span>
<span class="line">    <span class="token key atrule">metrics_path</span><span class="token punctuation">:</span> /metrics.prom</span>
<span class="line">    <span class="token key atrule">static_configs</span><span class="token punctuation">:</span> <span class="token punctuation">[</span> <span class="token punctuation">{</span> <span class="token key atrule">targets</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">'localhost:8080'</span><span class="token punctuation">]</span> <span class="token punctuation">}</span> <span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">job_name</span><span class="token punctuation">:</span> <span class="token string">'croupier-agent'</span></span>
<span class="line">    <span class="token key atrule">metrics_path</span><span class="token punctuation">:</span> /metrics.prom</span>
<span class="line">    <span class="token key atrule">static_configs</span><span class="token punctuation">:</span> <span class="token punctuation">[</span> <span class="token punctuation">{</span> <span class="token key atrule">targets</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">'localhost:19091'</span><span class="token punctuation">]</span> <span class="token punctuation">}</span> <span class="token punctuation">]</span></span>
<span class="line">  <span class="token punctuation">-</span> <span class="token key atrule">job_name</span><span class="token punctuation">:</span> <span class="token string">'croupier-edge'</span></span>
<span class="line">    <span class="token key atrule">metrics_path</span><span class="token punctuation">:</span> /metrics.prom</span>
<span class="line">    <span class="token key atrule">static_configs</span><span class="token punctuation">:</span> <span class="token punctuation">[</span> <span class="token punctuation">{</span> <span class="token key atrule">targets</span><span class="token punctuation">:</span> <span class="token punctuation">[</span><span class="token string">'localhost:9080'</span><span class="token punctuation">]</span> <span class="token punctuation">}</span> <span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Grafana quick panel ideas</p>
<ul>
<li>Query rate by function: <code v-pre>increase(croupier_invocations_total{function_id=&quot;$fid&quot;}[5m])</code></li>
<li>Error ratio: <code v-pre>increase(croupier_invocations_error_total{function_id=&quot;$fid&quot;}[5m]) / increase(croupier_invocations_total{function_id=&quot;$fid&quot;}[5m])</code></li>
<li>P95 latency: <code v-pre>histogram_quantile(0.95, sum by (le,function_id) (rate(croupier_invoke_latency_seconds_bucket[5m])))</code></li>
</ul>
</div></template>


