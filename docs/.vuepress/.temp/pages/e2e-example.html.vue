<template><div><h1 id="end-to-end-example-proto-→-pack-→-import-→-ui" tabindex="-1"><a class="header-anchor" href="#end-to-end-example-proto-→-pack-→-import-→-ui"><span>End-to-End Example (Proto → Pack → Import → UI)</span></a></h1>
<p>This walkthrough shows how to go from .proto with Croupier options to a function pack, import it into the Server, and invoke from the Web UI.</p>
<p>Prerequisites</p>
<ul>
<li><code v-pre>protoc</code> installed and on PATH (https://grpc.io/docs/protoc-installation/)</li>
<li>Croupier repo built (<code v-pre>make build</code>)</li>
</ul>
<ol>
<li>Generate a pack from examples</li>
</ol>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line"># Build the protoc plugin if needed and generate pack artifacts for all protos</span>
<span class="line">./scripts/generate-pack.sh</span>
<span class="line"></span>
<span class="line"># Inspect the generated pack (if pack.tgz is emitted by the plugin)</span>
<span class="line">./bin/croupier packs inspect gen/croupier/pack.tgz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li>Start Server and Agent</li>
</ol>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line"># Server (+sqlite approvals support optional)</span>
<span class="line">make server-sqlite</span>
<span class="line">./bin/croupier-server --config configs/server.example.yaml</span>
<span class="line"></span>
<span class="line"># Agent</span>
<span class="line">make agent</span>
<span class="line">./bin/croupier-agent --config configs/agent.example.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li>Import the pack into the Server</li>
</ol>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line"># Either import the generated pack.tgz …</span>
<span class="line">./bin/croupier packs import gen/croupier/pack.tgz</span>
<span class="line"># …or import example packs</span>
<span class="line">make packs-build</span>
<span class="line">./bin/croupier packs import packs/dist/prom.pack.tgz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="4">
<li>Open the Web UI and invoke</li>
</ol>
<ul>
<li>Navigate to GM Functions page</li>
<li>Select <code v-pre>prom.query_range</code></li>
<li>Fill in <code v-pre>expr</code> and optional time range</li>
<li>Submit, and see JSON + line chart (grid layout)</li>
</ul>
<p>Notes</p>
<ul>
<li>The generator uses method-level and field-level options under <code v-pre>proto/croupier/options/*</code>.</li>
<li>If <code v-pre>protoc</code> is not available, you can still use example packs under <code v-pre>packs/*</code> to try the flow.</li>
</ul>
</div></template>


