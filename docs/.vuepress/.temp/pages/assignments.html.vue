<template><div><h1 id="assignments-per-game-env-function-sets" tabindex="-1"><a class="header-anchor" href="#assignments-per-game-env-function-sets"><span>Assignments (Per Game/Env Function Sets)</span></a></h1>
<p>This document describes a minimal control-plane for function assignments per game/env. It is an early building block for M5 (pack downlink &amp; hot update).</p>
<p>Concept</p>
<ul>
<li>Assignments is a mapping from <code v-pre>game_id|env</code> to an array of <code v-pre>function_id</code>s.</li>
<li>Server persists this mapping to <code v-pre>&lt;packDir&gt;/assignments.json</code> and exposes HTTP APIs to get/set it.</li>
<li>Agents can later poll this mapping to decide which adapters/packs to activate (future work).</li>
</ul>
<p>Server APIs</p>
<ul>
<li>GET <code v-pre>/api/assignments?game_id=&amp;env=</code>
<ul>
<li>Returns <code v-pre>{ assignments: { &quot;&lt;game&gt;|&lt;env&gt;&quot;: [&quot;fn1&quot;,&quot;fn2&quot;,...] } }</code>.</li>
<li>Filters are optional; when omitted, returns all entries.</li>
</ul>
</li>
<li>POST <code v-pre>/api/assignments</code>
<ul>
<li>Body: <code v-pre>{ &quot;game_id&quot;: &quot;&lt;game&gt;&quot;, &quot;env&quot;: &quot;&lt;env&gt;&quot;, &quot;functions&quot;: [&quot;fn1&quot;,&quot;fn2&quot;] }</code></li>
<li>Overwrites the mapping for the given key; persists to <code v-pre>assignments.json</code>.</li>
<li>Response: <code v-pre>{ ok: true, unknown: [&quot;fnX&quot;, ...] }</code> where <code v-pre>unknown</code> lists function ids that are not present in the current descriptors and were ignored.</li>
</ul>
</li>
</ul>
<p>CLI (Current: use HTTP API or edit JSON directly)</p>
<ul>
<li>List assignments via HTTP API:</li>
</ul>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">curl http://localhost:8080/api/v1/assignments?game_id=mygame&amp;env=prod</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><ul>
<li>Set assignments via HTTP API:</li>
</ul>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">curl -X POST http://localhost:8080/api/v1/assignments \</span>
<span class="line">  -H "Content-Type: application/json" \</span>
<span class="line">  -d '{"game_id": "mygame", "env": "prod", "functions": ["prom.query", "prom.query_range"]}'</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>Alternative: Edit <code v-pre>configs/assignments.json</code> directly (server will reload on change)</li>
</ul>
<p>Note: A unified <code v-pre>croupier</code> CLI tool is planned for future releases to simplify these operations.</p>
<p>Web UI</p>
<ul>
<li>Configure via GM → Assignments: choose game/env and select function ids (empty means allow all). Save to persist on the server.</li>
<li>GM → Functions will auto-filter the function list by assignments when game/env is selected.</li>
</ul>
<p>Notes</p>
<ul>
<li>This is a minimal, file-backed control-plane intended to unblock end-to-end demos.</li>
<li>Future work (M5): Agent-side polling &amp; hot (un)load of adapters/packs, Server-side validation &amp; drift detection.</li>
</ul>
</div></template>


