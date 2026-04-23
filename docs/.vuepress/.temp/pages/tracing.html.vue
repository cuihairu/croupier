<template><div><h1 id="tracing-trace-id-propagation" tabindex="-1"><a class="header-anchor" href="#tracing-trace-id-propagation"><span>Tracing &amp; Trace ID Propagation</span></a></h1>
<p>Croupier propagates a lightweight <code v-pre>trace_id</code> across the Server → Agent → Adapter chain. This helps correlate UI/API calls, audit events, and downstream requests.</p>
<p>Flow</p>
<ul>
<li>Server (HTTP): generates a random <code v-pre>trace_id</code> per request and records it in audit; forwards it via <code v-pre>InvokeRequest.metadata[&quot;trace_id&quot;]</code>.</li>
<li>Agent/Server routing: preserves request metadata (trace_id/game_id/env) on RPC to function handlers.</li>
<li>Adapters (HTTP/Prom): add <code v-pre>X-Trace-Id</code>, <code v-pre>X-Game-Id</code>, <code v-pre>X-Env</code> headers to outbound HTTP requests when not already present.</li>
</ul>
<p>Headers</p>
<ul>
<li><code v-pre>X-Trace-Id</code>: correlates downstream REST requests with the originating GM action.</li>
<li><code v-pre>X-Game-Id</code>, <code v-pre>X-Env</code>: scope hints for multi-game environments (optional).</li>
</ul>
<p>UI</p>
<ul>
<li><code v-pre>/gm/audit</code>（或相关页面）中可查看 <code v-pre>trace_id</code> 字段；服务端日志也会打印 <code v-pre>trace_id</code>。</li>
</ul>
<p>Future Work</p>
<ul>
<li>OTLP exporter for distributed tracing (Jaeger/Tempo/etc.): planned. The existing <code v-pre>trace_id</code> can be embedded into spans once enabled.</li>
</ul>
</div></template>


