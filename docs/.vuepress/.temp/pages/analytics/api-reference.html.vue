<template><div><h1 id="ingestion-api" tabindex="-1"><a class="header-anchor" href="#ingestion-api"><span>Ingestion API</span></a></h1>
<p>鉴权</p>
<ul>
<li>头部：<code v-pre>X-Timestamp</code>（秒）、<code v-pre>X-Nonce</code>（随机串）、<code v-pre>X-Signature</code>（Base64(HMAC-SHA256(secret, <code v-pre>${ts}\n${nonce}\n${sha256(body)}</code>))）</li>
<li>异常码：401/403（签名）、429（限流）、400（格式错误）</li>
</ul>
<p>POST /api/ingest/events</p>
<ul>
<li>请求体：事件数组，每条至少包含 <code v-pre>event</code>、<code v-pre>ts</code>，推荐加 <code v-pre>uid</code>、<code v-pre>game_id</code>、<code v-pre>env</code></li>
</ul>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">[</span></span>
<span class="line">  <span class="token punctuation">{</span><span class="token property">"event"</span><span class="token operator">:</span><span class="token string">"session.start"</span><span class="token punctuation">,</span><span class="token property">"ts"</span><span class="token operator">:</span><span class="token number">1731700000000</span><span class="token punctuation">,</span><span class="token property">"attrs"</span><span class="token operator">:</span><span class="token punctuation">{</span><span class="token property">"uid"</span><span class="token operator">:</span><span class="token string">"u1"</span><span class="token punctuation">,</span><span class="token property">"game_id"</span><span class="token operator">:</span><span class="token string">"demo"</span><span class="token punctuation">,</span><span class="token property">"env"</span><span class="token operator">:</span><span class="token string">"dev"</span><span class="token punctuation">}</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>返回：<code v-pre>{&quot;ok&quot;:true}</code> 或错误详情</li>
</ul>
<p>POST /api/ingest/payments</p>
<ul>
<li>请求体：支付事件数组（字段同上，业务字段根据需要扩展）</li>
</ul>
<h1 id="otel-collector-服务端" tabindex="-1"><a class="header-anchor" href="#otel-collector-服务端"><span>OTel Collector（服务端）</span></a></h1>
<ul>
<li>推荐直接接入 OTLP（HTTP/gRPC），采集 traces/metrics/logs</li>
<li>参考: ./opentelemetry-integration.md</li>
</ul>
</div></template>


