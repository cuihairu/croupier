<template><div><h1 id="analytics-ingest-signing-guide" tabindex="-1"><a class="header-anchor" href="#analytics-ingest-signing-guide"><span>Analytics Ingest Signing Guide</span></a></h1>
<p>The ingest service (<code v-pre>croupier-ingest</code>) rejects every request unless it carries a
valid HMAC signature. Each upload should set the following headers:</p>
<table>
<thead>
<tr>
<th>Header</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>X-Game-Id</code></td>
<td>Game identifier used to select the shared secret (per-tenant mapping).</td>
</tr>
<tr>
<td><code v-pre>X-Timestamp</code></td>
<td>Unix timestamp (seconds). Replay protection fails if it drifts beyond the skew window (<code v-pre>ANALYTICS_INGEST_SKEW</code>, default 300s).</td>
</tr>
<tr>
<td><code v-pre>X-Nonce</code></td>
<td>Random nonce string. The server stores <code v-pre>(game_id, nonce)</code> in Redis for the dedupe TTL to block replays.</td>
</tr>
<tr>
<td><code v-pre>X-Signature</code></td>
<td><code v-pre>base64(HMAC_SHA256(secret, ts + &quot;\n&quot; + nonce + &quot;\n&quot; + sha256(body)))</code></td>
</tr>
</tbody>
</table>
<p>Example (TypeScript) helper:</p>
<div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre v-pre><code class="language-typescript"><span class="line"><span class="token keyword">import</span> crypto <span class="token keyword">from</span> <span class="token string">'crypto'</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">export</span> <span class="token keyword">function</span> <span class="token function">buildIngestHeaders</span><span class="token punctuation">(</span>secret<span class="token operator">:</span> <span class="token builtin">string</span><span class="token punctuation">,</span> gameId<span class="token operator">:</span> <span class="token builtin">string</span><span class="token punctuation">,</span> payload<span class="token operator">:</span> <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">  <span class="token keyword">const</span> ts <span class="token operator">=</span> <span class="token template-string"><span class="token template-punctuation string">`</span><span class="token interpolation"><span class="token interpolation-punctuation punctuation">${</span>Math<span class="token punctuation">.</span><span class="token function">floor</span><span class="token punctuation">(</span>Date<span class="token punctuation">.</span><span class="token function">now</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token operator">/</span> <span class="token number">1000</span><span class="token punctuation">)</span><span class="token interpolation-punctuation punctuation">}</span></span><span class="token template-punctuation string">`</span></span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">const</span> nonce <span class="token operator">=</span> crypto<span class="token punctuation">.</span><span class="token function">randomUUID</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">const</span> bodyHash <span class="token operator">=</span> crypto<span class="token punctuation">.</span><span class="token function">createHash</span><span class="token punctuation">(</span><span class="token string">'sha256'</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">update</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">digest</span><span class="token punctuation">(</span><span class="token string">'hex'</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">const</span> msg <span class="token operator">=</span> <span class="token template-string"><span class="token template-punctuation string">`</span><span class="token interpolation"><span class="token interpolation-punctuation punctuation">${</span>ts<span class="token interpolation-punctuation punctuation">}</span></span><span class="token string">\n</span><span class="token interpolation"><span class="token interpolation-punctuation punctuation">${</span>nonce<span class="token interpolation-punctuation punctuation">}</span></span><span class="token string">\n</span><span class="token interpolation"><span class="token interpolation-punctuation punctuation">${</span>bodyHash<span class="token interpolation-punctuation punctuation">}</span></span><span class="token template-punctuation string">`</span></span><span class="token punctuation">;</span></span>
<span class="line">  <span class="token keyword">const</span> signature <span class="token operator">=</span> crypto<span class="token punctuation">.</span><span class="token function">createHmac</span><span class="token punctuation">(</span><span class="token string">'sha256'</span><span class="token punctuation">,</span> secret<span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">update</span><span class="token punctuation">(</span>msg<span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">digest</span><span class="token punctuation">(</span><span class="token string">'base64'</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">  <span class="token keyword">return</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token string-property property">'X-Game-Id'</span><span class="token operator">:</span> gameId<span class="token punctuation">,</span></span>
<span class="line">    <span class="token string-property property">'X-Timestamp'</span><span class="token operator">:</span> ts<span class="token punctuation">,</span></span>
<span class="line">    <span class="token string-property property">'X-Nonce'</span><span class="token operator">:</span> nonce<span class="token punctuation">,</span></span>
<span class="line">    <span class="token string-property property">'X-Signature'</span><span class="token operator">:</span> signature<span class="token punctuation">,</span></span>
<span class="line">    <span class="token string-property property">'Content-Type'</span><span class="token operator">:</span> <span class="token string">'application/json'</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Secrets are configured via <code v-pre>ANALYTICS_INGEST_SECRETS</code> (JSON map) or the legacy
<code v-pre>--secret</code> flag for single-tenant setups. The ingest service also validates that
each event/payment payload includes <code v-pre>game_id</code>, <code v-pre>env</code>, <code v-pre>ts</code>, and other required
fields before enqueueing them.</p>
</div></template>


