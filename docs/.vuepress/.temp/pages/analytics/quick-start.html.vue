<template><div><h1 id="_5-分钟上手" tabindex="-1"><a class="header-anchor" href="#_5-分钟上手"><span>5 分钟上手</span></a></h1>
<p>本节帮助你快速启动基础依赖、启用采集入口，并通过 HTTP 上报一条事件。</p>
<p>前置要求</p>
<ul>
<li>Docker 或本地 ClickHouse/Redis</li>
<li>curl 或任意 HTTP 客户端</li>
</ul>
<ol>
<li>启动基础依赖</li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">docker</span> compose up <span class="token parameter variable">-d</span> clickhouse redis</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><ol start="2">
<li>启动 Ingestion 与 Worker（示例）</li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 构建二进制</span></span>
<span class="line"><span class="token function">make</span> dev   <span class="token comment"># 生成到 bin/</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启动服务（示意参数，按需调整）</span></span>
<span class="line">bin/server <span class="token parameter variable">--http_addr</span> :8080</span>
<span class="line">bin/ingest <span class="token parameter variable">--http_addr</span> :18080 <span class="token parameter variable">--redis_url</span> redis://localhost:6379/0 <span class="token parameter variable">--secret</span> your-secret</span>
<span class="line">bin/analytics-worker <span class="token parameter variable">--redis_url</span> redis://localhost:6379/0 <span class="token parameter variable">--clickhouse_url</span> http://localhost:8123</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li>客户端上报一条事件</li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token assign-left variable">BODY</span><span class="token operator">=</span><span class="token string">'[{"event":"session.start","ts":'</span><span class="token variable"><span class="token variable">$(</span><span class="token function">date</span> +%s<span class="token variable">)</span></span><span class="token string">'000,"attrs":{"uid":"u1","game_id":"demo"}}]'</span></span>
<span class="line"><span class="token assign-left variable">TS</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span><span class="token function">date</span> +%s<span class="token variable">)</span></span></span>
<span class="line"><span class="token assign-left variable">NONCE</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span>openssl rand <span class="token parameter variable">-hex</span> <span class="token number">8</span><span class="token variable">)</span></span></span>
<span class="line"><span class="token assign-left variable">SIG</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">printf</span> <span class="token string">"%s<span class="token entity" title="\n">\n</span>%s<span class="token entity" title="\n">\n</span>%s"</span> <span class="token string">"<span class="token variable">$TS</span>"</span> <span class="token string">"<span class="token variable">$NONCE</span>"</span> <span class="token string">"<span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">printf</span> <span class="token string">"%s"</span> <span class="token string">"<span class="token variable">$BODY</span>"</span> <span class="token operator">|</span> shasum <span class="token parameter variable">-a</span> <span class="token number">256</span> <span class="token operator">|</span> <span class="token function">awk</span> <span class="token string">'{print $1}'</span><span class="token variable">)</span></span>"</span> <span class="token operator">|</span> <span class="token punctuation">\</span></span>
<span class="line">  openssl dgst <span class="token parameter variable">-sha256</span> <span class="token parameter variable">-hmac</span> <span class="token string">"your-secret"</span> <span class="token parameter variable">-binary</span> <span class="token operator">|</span> base64<span class="token variable">)</span></span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-sS</span> <span class="token parameter variable">-X</span> POST <span class="token string">"http://localhost:18080/api/ingest/events"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Timestamp: <span class="token variable">$TS</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Nonce: <span class="token variable">$NONCE</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Signature: <span class="token variable">$SIG</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">--data</span> <span class="token string">"<span class="token variable">$BODY</span>"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="4">
<li>在 ClickHouse 中查看</li>
</ol>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token comment">-- 连接 CH 后执行：</span></span>
<span class="line"><span class="token keyword">SELECT</span> event<span class="token punctuation">,</span> ts<span class="token punctuation">,</span> attrs <span class="token keyword">FROM</span> analytics<span class="token punctuation">.</span>events <span class="token keyword">ORDER</span> <span class="token keyword">BY</span> ts <span class="token keyword">DESC</span> <span class="token keyword">LIMIT</span> <span class="token number">10</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><ol start="5">
<li>接入 Grafana（可选）</li>
</ol>
<ul>
<li>添加 ClickHouse/Prometheus 数据源</li>
<li>导入内置看板（在 packs/analytics/*.json 中提供示例）</li>
</ul>
<p>小结</p>
<ul>
<li>客户端事件 → Ingestion → Redis Stream → Worker → ClickHouse</li>
<li>服务端 Traces/Metrics 建议直接走 OTel Collector → ClickHouse/Prometheus</li>
</ul>
<p>参考</p>
<ul>
<li>指标全景图: ./game-metrics-overview.md</li>
<li>采集架构: ./data-collection-architecture.md</li>
</ul>
</div></template>


