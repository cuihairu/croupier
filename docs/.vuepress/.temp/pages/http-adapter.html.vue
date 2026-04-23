<template><div><h1 id="http-adapter-generic" tabindex="-1"><a class="header-anchor" href="#http-adapter-generic"><span>HTTP Adapter (Generic)</span></a></h1>
<p>The <code v-pre>http-adapter</code> exposes one function <code v-pre>http.generic_invoke</code> that forwards a generic HTTP request and returns the response body (best-effort JSON passthrough).</p>
<p>Usage</p>
<ul>
<li>Function: <code v-pre>http.generic_invoke</code></li>
<li>Request schema: <code v-pre>{ method, url, headers, body }</code></li>
<li>Output views (pack example):
<ul>
<li><code v-pre>json.view</code> to preview raw response</li>
<li><code v-pre>table.basic</code> to render array responses as a table</li>
</ul>
</li>
</ul>
<p>Examples</p>
<ul>
<li>List JSON array
<ul>
<li>URL: https://example.com/api/items</li>
<li>Response: <code v-pre>[ { &quot;id&quot;: 1, &quot;name&quot;: &quot;foo&quot; }, { &quot;id&quot;: 2, &quot;name&quot;: &quot;bar&quot; } ]</code></li>
<li>The built-in <code v-pre>table.basic</code> view works directly with <code v-pre>transform.expr: '$'</code>.</li>
</ul>
</li>
<li>Nested data array
<ul>
<li>Response: <code v-pre>{ &quot;data&quot;: { &quot;items&quot;: [ { &quot;id&quot;: 1 }, { &quot;id&quot;: 2 } ] } }</code></li>
<li>Update the view transform to <code v-pre>expr: '$.data.items'</code> or use <code v-pre>template</code>:</li>
</ul>
</li>
</ul>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">{</span>
<span class="line">  "id": "table",</span>
<span class="line">  "renderer": "table.basic",</span>
<span class="line">  "transform": { "expr": "$.data.items" }</span>
<span class="line">}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>Timeseries
<ul>
<li>Response: <code v-pre>{ &quot;series&quot;: [ { &quot;name&quot;: &quot;cpu&quot;, &quot;data&quot;: [[1719916800000, 0.5], ...] } ] }</code></li>
<li>Add a chart view:</li>
</ul>
</li>
</ul>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">{</span>
<span class="line">  "id": "chart",</span>
<span class="line">  "renderer": "echarts.line",</span>
<span class="line">  "transform": { "expr": "$.series" }</span>
<span class="line">}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Notes</p>
<ul>
<li>The adapter does not transform JSON by itself. Packs (descriptors) can shape data for views via <code v-pre>outputs.views[].transform</code> (see <code v-pre>docs/ui-and-views.md</code>).</li>
<li>For authenticated APIs, set headers in the request (e.g., <code v-pre>Authorization</code>).</li>
<li>For large payloads, prefer <code v-pre>GET</code> or compressed responses; current timeout defaults to 15s.</li>
</ul>
<p>Built-in function (example)</p>
<ul>
<li>The HTTP adapter also exposes a convenience function:
<ul>
<li><code v-pre>alertmanager.list_alerts</code>: map simple params <code v-pre>{ base_url, silenced?, inhibited?, active? }</code> to Alertmanager <code v-pre>GET /api/v2/alerts</code>.</li>
<li>A sample pack <code v-pre>packs/alertmanager</code> renders alerts as a table (columns: name, severity, status, startsAt, summary).</li>
<li>Usage flow: import <code v-pre>alertmanager.pack.tgz</code> → select <code v-pre>alertmanager.list_alerts</code> → fill base URL and filters → view table/json.</li>
</ul>
</li>
</ul>
</div></template>


