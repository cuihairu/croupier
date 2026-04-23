<template><div><h1 id="ui-schema-views-web" tabindex="-1"><a class="header-anchor" href="#ui-schema-views-web"><span>UI Schema &amp; Views (Web)</span></a></h1>
<p>This document explains how the Web app renders forms (UI Schema) and output views from packs.</p>
<p>UI Schema</p>
<ul>
<li>Source: <code v-pre>ui/&lt;function_id&gt;.schema.json</code> (JSON Schema), <code v-pre>ui/&lt;function_id&gt;.uischema.json</code> (UI Schema)</li>
<li>Layout (optional):
<ul>
<li><code v-pre>ui:layout</code>: <code v-pre>{ &quot;type&quot;: &quot;grid&quot;, &quot;cols&quot;: 1|2|3|4 }</code> (default 1)</li>
<li><code v-pre>ui:groups</code>: <code v-pre>[ { &quot;title&quot;: &quot;Group Name&quot;, &quot;fields&quot;: [&quot;field1&quot;,&quot;field2&quot;,...] }, ... ]</code></li>
</ul>
</li>
<li>Field UI hints (<code v-pre>ui:fields</code> per field):
<ul>
<li><code v-pre>label</code>, <code v-pre>placeholder</code>, <code v-pre>widget</code> (input/textarea/select)</li>
<li><code v-pre>enum</code> and <code v-pre>x-enum-labels</code> available for select rendering</li>
<li><code v-pre>show_if</code> / <code v-pre>required_if</code>: simple expressions using paths and operators <code v-pre>==</code>, <code v-pre>!=</code>, <code v-pre>&amp;&amp;</code>, <code v-pre>||</code>. Example: <code v-pre>$.type == &quot;advanced&quot; &amp;&amp; $.enabled == true</code></li>
</ul>
</li>
<li>Supported types:
<ul>
<li><code v-pre>string</code> / <code v-pre>number</code> / <code v-pre>integer</code> / <code v-pre>boolean</code></li>
<li><code v-pre>array</code> (adds/removes items)</li>
<li><code v-pre>object</code> with <code v-pre>properties</code> (nested group)</li>
<li><code v-pre>object</code> with <code v-pre>additionalProperties</code> (map) – UI as key/value entries, submitted as JSON object</li>
</ul>
</li>
<li>Validation:
<ul>
<li><code v-pre>minLength</code> / <code v-pre>maxLength</code> / <code v-pre>pattern</code> for strings</li>
<li><code v-pre>minimum</code> / <code v-pre>maximum</code> for numbers</li>
</ul>
</li>
</ul>
<p>Views &amp; Transform</p>
<ul>
<li>Descriptor: <code v-pre>outputs.views[]</code> can specify <code v-pre>renderer</code>, optional <code v-pre>transform</code></li>
<li>Transform (frontend, safe subset):
<ul>
<li>When <code v-pre>lang</code> is <code v-pre>cel</code> (or omitted), expression is treated as a JSON path starting at root (e.g., <code v-pre>$.data.series</code>)</li>
<li>The selected value is passed to the renderer component</li>
<li>Path supports simple array indexing via brackets, e.g., <code v-pre>$.value[1]</code>.</li>
</ul>
</li>
<li>Built-in renderers:
<ul>
<li><code v-pre>json.view</code> – pretty-print JSON</li>
<li><code v-pre>table.basic</code> – render array of objects as a table</li>
<li><code v-pre>echarts.line</code> – line chart via ECharts (loaded on demand from CDN). Data accepts Prometheus matrix via <code v-pre>$.data.result</code> or generic <code v-pre>{ series: [...] }</code>/<code v-pre>[{ name, data: [[ms,value],...] }]</code>; options are passed through.</li>
</ul>
</li>
</ul>
<p>Layout</p>
<ul>
<li>Use <code v-pre>outputs.layout</code> to control multiple views arrangement.</li>
<li>Example: <code v-pre>{ &quot;type&quot;: &quot;grid&quot;, &quot;cols&quot;: 2 }</code> renders views in a responsive two-column grid.</li>
</ul>
<p>Transform Templates (JSON-based)</p>
<ul>
<li>Besides <code v-pre>expr</code>, packs can provide <code v-pre>transform.template</code> to declaratively reshape data.</li>
<li>Template strings starting with <code v-pre>$.</code> are resolved as JSON paths against the current context item; strings starting with <code v-pre>$$.</code> resolve against the original root.</li>
<li>Special form: <code v-pre>{ &quot;forEach&quot;: { &quot;path&quot;: &quot;$.items&quot;, &quot;template&quot;: { ... } } }</code> maps arrays.</li>
<li>Optional filtering: <code v-pre>{ &quot;forEach&quot;: { &quot;path&quot;: &quot;$.items&quot;, &quot;where&quot;: { &quot;contains&quot;: [&quot;$.name&quot;, &quot;prod&quot;] }, &quot;template&quot;: { ... } } }</code>.</li>
<li>Aliases &amp; helpers:
<ul>
<li><code v-pre>map</code>: <code v-pre>{ &quot;map&quot;: { &quot;path&quot;: &quot;$.items&quot;, &quot;template&quot;: { ... } } }</code></li>
<li><code v-pre>pluck</code>: <code v-pre>{ &quot;pluck&quot;: { &quot;path&quot;: &quot;$.items&quot;, &quot;value&quot;: &quot;$.field&quot; } }</code></li>
<li>Aggregates: <code v-pre>sum</code> / <code v-pre>avg</code> on arrays of values: <code v-pre>{ &quot;sum&quot;: { &quot;path&quot;: &quot;$.items&quot;, &quot;value&quot;: &quot;$.v&quot; } }</code></li>
<li>Value directives: wrap values to coerce/compute
<ul>
<li><code v-pre>{ &quot;number&quot;: &quot;$.v&quot; }</code> → Number($.v)</li>
<li><code v-pre>{ &quot;msFromSec&quot;: &quot;$.ts&quot; }</code> → seconds to milliseconds</li>
<li><code v-pre>{ &quot;mul&quot;: { &quot;value&quot;: &quot;$.v&quot;, &quot;by&quot;: 100 } }</code>, <code v-pre>{ &quot;div&quot;: { &quot;value&quot;: &quot;$.v&quot;, &quot;by&quot;: 1000 } }</code></li>
<li><code v-pre>{ &quot;add&quot;: { &quot;a&quot;: &quot;$.a&quot;, &quot;b&quot;: 1 } }</code>, <code v-pre>{ &quot;sub&quot;: { &quot;a&quot;: &quot;$.a&quot;, &quot;b&quot;: 1 } }</code></li>
<li><code v-pre>{ &quot;toFixed&quot;: { &quot;value&quot;: &quot;$.v&quot;, &quot;digits&quot;: 2 } }</code> → Number fixed decimals</li>
<li><code v-pre>{ &quot;isoFromMs&quot;: &quot;$.tsMs&quot; }</code>, <code v-pre>{ &quot;isoFromSec&quot;: &quot;$.tsSec&quot; }</code> → ISO datetime string</li>
</ul>
</li>
</ul>
</li>
</ul>
<p>View Conditions</p>
<ul>
<li>Optional <code v-pre>show_if</code> on a view allows hiding the view when the referenced path is falsy or an empty array.</li>
<li>Example: <code v-pre>{ &quot;id&quot;: &quot;table&quot;, &quot;show_if&quot;: &quot;$.data.items&quot;, ... }</code>.</li>
<li>Example (Prometheus matrix → ECharts series):</li>
</ul>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">{</span>
<span class="line">  "views": [</span>
<span class="line">    {</span>
<span class="line">      "id": "chart",</span>
<span class="line">      "renderer": "echarts.line",</span>
<span class="line">      "transform": {</span>
<span class="line">        "template": {</span>
<span class="line">          "forEach": {</span>
<span class="line">            "path": "$.data.result",</span>
<span class="line">            "template": { "name": "$.metric.instance", "data": "$.values" }</span>
<span class="line">          }</span>
<span class="line">        }</span>
<span class="line">      }</span>
<span class="line">    }</span>
<span class="line">  ]</span>
<span class="line">}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Notes</p>
<ul>
<li>This is a minimal baseline intended for quick PoC. Full CEL transforms and richer Schema-Form are planned.</li>
<li>Packs generated by <code v-pre>protoc-gen-croupier</code> include a default UI Schema with a grid layout and inferred groups.</li>
</ul>
<p>Plugins</p>
<ul>
<li>Built-in renderers are registered in <code v-pre>web/src/plugin/registry.ts</code>.</li>
<li>Packs can ship frontend plugins under <code v-pre>web-plugin/*</code> and reference them in <code v-pre>manifest.json</code> via <code v-pre>web_plugins</code>.</li>
<li>The plugin module must export a default function receiving <code v-pre>{ registerRenderer }</code> and may register custom renderers.</li>
<li>Example (Prom pack): <code v-pre>web-plugin/echarts_plugin.js</code> registers <code v-pre>echarts.bar</code> and is listed in <code v-pre>manifest.json</code> → <code v-pre>web_plugins</code>.</li>
</ul>
</div></template>


