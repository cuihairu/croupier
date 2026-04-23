<template><div><h1 id="表结构-ddl" tabindex="-1"><a class="header-anchor" href="#表结构-ddl"><span>表结构（DDL）</span></a></h1>
<p>事件明细（events）</p>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> <span class="token keyword">IF</span> <span class="token operator">NOT</span> <span class="token keyword">EXISTS</span> analytics<span class="token punctuation">.</span>events <span class="token punctuation">(</span></span>
<span class="line">  event_time <span class="token keyword">DateTime</span> <span class="token keyword">DEFAULT</span> <span class="token function">now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  game_id LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  env LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  user_id String<span class="token punctuation">,</span></span>
<span class="line">  session_id String<span class="token punctuation">,</span></span>
<span class="line">  event LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  channel LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  platform LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  country FixedString<span class="token punctuation">(</span><span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  app_version String<span class="token punctuation">,</span></span>
<span class="line">  event_id UUID<span class="token punctuation">,</span></span>
<span class="line">  props_json String</span>
<span class="line"><span class="token punctuation">)</span> <span class="token keyword">ENGINE</span> <span class="token operator">=</span> MergeTree</span>
<span class="line"><span class="token keyword">PARTITION</span> <span class="token keyword">BY</span> toYYYYMM<span class="token punctuation">(</span>event_time<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> <span class="token punctuation">(</span>game_id<span class="token punctuation">,</span> env<span class="token punctuation">,</span> event<span class="token punctuation">,</span> user_id<span class="token punctuation">,</span> event_time<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>支付明细（payments）</p>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> <span class="token keyword">IF</span> <span class="token operator">NOT</span> <span class="token keyword">EXISTS</span> analytics<span class="token punctuation">.</span>payments <span class="token punctuation">(</span></span>
<span class="line">  <span class="token keyword">time</span> <span class="token keyword">DateTime</span> <span class="token keyword">DEFAULT</span> <span class="token function">now</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  game_id LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  env LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  user_id String<span class="token punctuation">,</span></span>
<span class="line">  order_id String<span class="token punctuation">,</span></span>
<span class="line">  amount_cents UInt64<span class="token punctuation">,</span></span>
<span class="line">  currency FixedString<span class="token punctuation">(</span><span class="token number">3</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token keyword">status</span> LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  channel LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  platform LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  country FixedString<span class="token punctuation">(</span><span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  region LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  city String<span class="token punctuation">,</span></span>
<span class="line">  product_id LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  reason String</span>
<span class="line"><span class="token punctuation">)</span> <span class="token keyword">ENGINE</span> <span class="token operator">=</span> MergeTree</span>
<span class="line"><span class="token keyword">PARTITION</span> <span class="token keyword">BY</span> toYYYYMM<span class="token punctuation">(</span><span class="token keyword">time</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> <span class="token punctuation">(</span>game_id<span class="token punctuation">,</span> env<span class="token punctuation">,</span> <span class="token keyword">time</span><span class="token punctuation">,</span> order_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>分钟在线（minute_online）</p>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> <span class="token keyword">IF</span> <span class="token operator">NOT</span> <span class="token keyword">EXISTS</span> analytics<span class="token punctuation">.</span>minute_online <span class="token punctuation">(</span></span>
<span class="line">  m <span class="token keyword">DateTime</span><span class="token punctuation">,</span></span>
<span class="line">  game_id LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  env LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  online UInt32</span>
<span class="line"><span class="token punctuation">)</span> <span class="token keyword">ENGINE</span> <span class="token operator">=</span> MergeTree</span>
<span class="line"><span class="token keyword">PARTITION</span> <span class="token keyword">BY</span> toYYYYMM<span class="token punctuation">(</span>m<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> <span class="token punctuation">(</span>game_id<span class="token punctuation">,</span> env<span class="token punctuation">,</span> m<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>日活/新增（daily_users，ReplacingMergeTree）</p>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> <span class="token keyword">IF</span> <span class="token operator">NOT</span> <span class="token keyword">EXISTS</span> analytics<span class="token punctuation">.</span>daily_users <span class="token punctuation">(</span></span>
<span class="line">  d <span class="token keyword">Date</span><span class="token punctuation">,</span></span>
<span class="line">  game_id LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  env LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  dau UInt64<span class="token punctuation">,</span></span>
<span class="line">  new_users UInt64<span class="token punctuation">,</span></span>
<span class="line">  version UInt64</span>
<span class="line"><span class="token punctuation">)</span> <span class="token keyword">ENGINE</span> <span class="token operator">=</span> ReplacingMergeTree<span class="token punctuation">(</span>version<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">PARTITION</span> <span class="token keyword">BY</span> toYYYYMM<span class="token punctuation">(</span>d<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> <span class="token punctuation">(</span>game_id<span class="token punctuation">,</span> env<span class="token punctuation">,</span> d<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>日收入（daily_revenue，ReplacingMergeTree）</p>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> <span class="token keyword">IF</span> <span class="token operator">NOT</span> <span class="token keyword">EXISTS</span> analytics<span class="token punctuation">.</span>daily_revenue <span class="token punctuation">(</span></span>
<span class="line">  d <span class="token keyword">Date</span><span class="token punctuation">,</span></span>
<span class="line">  game_id LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  env LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  revenue_cents UInt64<span class="token punctuation">,</span></span>
<span class="line">  refunds_cents UInt64<span class="token punctuation">,</span></span>
<span class="line">  failed UInt64<span class="token punctuation">,</span></span>
<span class="line">  version UInt64</span>
<span class="line"><span class="token punctuation">)</span> <span class="token keyword">ENGINE</span> <span class="token operator">=</span> ReplacingMergeTree<span class="token punctuation">(</span>version<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">PARTITION</span> <span class="token keyword">BY</span> toYYYYMM<span class="token punctuation">(</span>d<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> <span class="token punctuation">(</span>game_id<span class="token punctuation">,</span> env<span class="token punctuation">,</span> d<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>日峰值在线（物化视图）</p>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">CREATE</span> <span class="token keyword">TABLE</span> <span class="token keyword">IF</span> <span class="token operator">NOT</span> <span class="token keyword">EXISTS</span> analytics<span class="token punctuation">.</span>daily_online_peak <span class="token punctuation">(</span></span>
<span class="line">  d <span class="token keyword">Date</span><span class="token punctuation">,</span></span>
<span class="line">  game_id LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  env LowCardinality<span class="token punctuation">(</span>String<span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">  peak_online AggregateFunction<span class="token punctuation">(</span>max<span class="token punctuation">,</span> UInt32<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">)</span> <span class="token keyword">ENGINE</span> <span class="token operator">=</span> AggregatingMergeTree</span>
<span class="line"><span class="token keyword">PARTITION</span> <span class="token keyword">BY</span> toYYYYMM<span class="token punctuation">(</span>d<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> <span class="token punctuation">(</span>game_id<span class="token punctuation">,</span> env<span class="token punctuation">,</span> d<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">CREATE</span> MATERIALIZED <span class="token keyword">VIEW</span> <span class="token keyword">IF</span> <span class="token operator">NOT</span> <span class="token keyword">EXISTS</span> analytics<span class="token punctuation">.</span>daily_online_peak_mv</span>
<span class="line"><span class="token keyword">TO</span> analytics<span class="token punctuation">.</span>daily_online_peak <span class="token keyword">AS</span></span>
<span class="line"><span class="token keyword">SELECT</span> toDate<span class="token punctuation">(</span>m<span class="token punctuation">)</span> <span class="token keyword">AS</span> d<span class="token punctuation">,</span> game_id<span class="token punctuation">,</span> env<span class="token punctuation">,</span> maxState<span class="token punctuation">(</span>online<span class="token punctuation">)</span> <span class="token keyword">AS</span> peak_online</span>
<span class="line"><span class="token keyword">FROM</span> analytics<span class="token punctuation">.</span>minute_online</span>
<span class="line"><span class="token keyword">GROUP</span> <span class="token keyword">BY</span> d<span class="token punctuation">,</span> game_id<span class="token punctuation">,</span> env<span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h1 id="ingestion-字段映射" tabindex="-1"><a class="header-anchor" href="#ingestion-字段映射"><span>Ingestion 字段映射</span></a></h1>
<ul>
<li>事件写入（analytics.events）
<ul>
<li>从上报 JSON 中映射：<code v-pre>ts -&gt; event_time (RFC3339)</code>、<code v-pre>game_id</code>、<code v-pre>env</code>、<code v-pre>user_id</code>、<code v-pre>session_id</code>、<code v-pre>event</code>、<code v-pre>channel</code>、<code v-pre>platform</code>、<code v-pre>country</code>、<code v-pre>app_version</code>、<code v-pre>event_id</code>；其余作为 <code v-pre>props_json</code></li>
</ul>
</li>
<li>支付写入（analytics.payments）
<ul>
<li><code v-pre>ts -&gt; time</code>、<code v-pre>game_id</code>、<code v-pre>env</code>、<code v-pre>user_id</code>、<code v-pre>order_id</code>、<code v-pre>amount_cents</code>、<code v-pre>currency</code>、<code v-pre>status</code>、<code v-pre>channel</code>、<code v-pre>platform</code>、<code v-pre>country</code>、<code v-pre>region</code>、<code v-pre>city</code>、<code v-pre>product_id</code>、<code v-pre>reason</code></li>
</ul>
</li>
<li>分钟在线与日活/新增
<ul>
<li>Worker 使用 Redis HyperLogLog 统计分钟在线（heartbeat/session_start）和 DAU/新增（login/register/first_active），周期性落入 ClickHouse</li>
</ul>
</li>
</ul>
<h1 id="示例查询" tabindex="-1"><a class="header-anchor" href="#示例查询"><span>示例查询</span></a></h1>
<ul>
<li>最近 7 天 DAU/New</li>
</ul>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">SELECT</span> d<span class="token punctuation">,</span> dau<span class="token punctuation">,</span> new_users</span>
<span class="line"><span class="token keyword">FROM</span> analytics<span class="token punctuation">.</span>daily_users</span>
<span class="line"><span class="token keyword">WHERE</span> game_id <span class="token operator">=</span> <span class="token string">'demo'</span> <span class="token operator">AND</span> env <span class="token operator">=</span> <span class="token string">'prod'</span> <span class="token operator">AND</span> d <span class="token operator">>=</span> today<span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">-</span> <span class="token number">7</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> d<span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>最近 7 天收入（元）</li>
</ul>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">SELECT</span> d<span class="token punctuation">,</span> revenue_cents<span class="token operator">/</span><span class="token number">100.0</span> <span class="token keyword">AS</span> revenue<span class="token punctuation">,</span> refunds_cents<span class="token operator">/</span><span class="token number">100.0</span> <span class="token keyword">AS</span> refunds</span>
<span class="line"><span class="token keyword">FROM</span> analytics<span class="token punctuation">.</span>daily_revenue</span>
<span class="line"><span class="token keyword">WHERE</span> game_id <span class="token operator">=</span> <span class="token string">'demo'</span> <span class="token operator">AND</span> env <span class="token operator">=</span> <span class="token string">'prod'</span> <span class="token operator">AND</span> d <span class="token operator">>=</span> today<span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">-</span> <span class="token number">7</span></span>
<span class="line"><span class="token keyword">ORDER</span> <span class="token keyword">BY</span> d<span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>峰值在线（聚合状态求值）</li>
</ul>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">SELECT</span> d<span class="token punctuation">,</span> maxMerge<span class="token punctuation">(</span>peak_online<span class="token punctuation">)</span> <span class="token keyword">AS</span> peak_online</span>
<span class="line"><span class="token keyword">FROM</span> analytics<span class="token punctuation">.</span>daily_online_peak</span>
<span class="line"><span class="token keyword">WHERE</span> game_id <span class="token operator">=</span> <span class="token string">'demo'</span> <span class="token operator">AND</span> env <span class="token operator">=</span> <span class="token string">'prod'</span> <span class="token operator">AND</span> d <span class="token operator">>=</span> today<span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">-</span> <span class="token number">7</span></span>
<span class="line"><span class="token keyword">GROUP</span> <span class="token keyword">BY</span> d <span class="token keyword">ORDER</span> <span class="token keyword">BY</span> d<span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>事件漏斗示例（进入-&gt;完成）</li>
</ul>
<div class="language-sql line-numbers-mode" data-highlighter="prismjs" data-ext="sql"><pre v-pre><code class="language-sql"><span class="line"><span class="token keyword">WITH</span></span>
<span class="line">  <span class="token punctuation">(</span><span class="token keyword">SELECT</span> <span class="token function">count</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">FROM</span> analytics<span class="token punctuation">.</span>events</span>
<span class="line">   <span class="token keyword">WHERE</span> game_id<span class="token operator">=</span><span class="token string">'demo'</span> <span class="token operator">AND</span> env<span class="token operator">=</span><span class="token string">'prod'</span></span>
<span class="line">     <span class="token operator">AND</span> event_time <span class="token operator">>=</span> <span class="token function">now</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">-</span> <span class="token keyword">INTERVAL</span> <span class="token number">7</span> <span class="token keyword">DAY</span></span>
<span class="line">     <span class="token operator">AND</span> event<span class="token operator">=</span><span class="token string">'level.start'</span><span class="token punctuation">)</span> <span class="token keyword">AS</span> starts<span class="token punctuation">,</span></span>
<span class="line">  <span class="token punctuation">(</span><span class="token keyword">SELECT</span> <span class="token function">count</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">FROM</span> analytics<span class="token punctuation">.</span>events</span>
<span class="line">   <span class="token keyword">WHERE</span> game_id<span class="token operator">=</span><span class="token string">'demo'</span> <span class="token operator">AND</span> env<span class="token operator">=</span><span class="token string">'prod'</span></span>
<span class="line">     <span class="token operator">AND</span> event_time <span class="token operator">>=</span> <span class="token function">now</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">-</span> <span class="token keyword">INTERVAL</span> <span class="token number">7</span> <span class="token keyword">DAY</span></span>
<span class="line">     <span class="token operator">AND</span> event<span class="token operator">=</span><span class="token string">'level.complete'</span><span class="token punctuation">)</span> <span class="token keyword">AS</span> completes</span>
<span class="line"><span class="token keyword">SELECT</span> starts<span class="token punctuation">,</span> completes<span class="token punctuation">,</span> completes<span class="token operator">/</span>starts <span class="token keyword">AS</span> cr<span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h1 id="性能与治理建议" tabindex="-1"><a class="header-anchor" href="#性能与治理建议"><span>性能与治理建议</span></a></h1>
<ul>
<li>低基数字段使用 LowCardinality（已在 DDL 使用）</li>
<li>按月分区、合理 ORDER BY（已在 DDL 使用）</li>
<li>ReplacingMergeTree + version 字段用于“幂等/更新”写入</li>
<li>高基数字段优先放入 props_json，避免维度爆炸；对分析常用字段正式列化</li>
</ul>
</div></template>


