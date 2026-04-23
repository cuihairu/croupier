<template><div><h1 id="protoc-gen-croupier-skeleton" tabindex="-1"><a class="header-anchor" href="#protoc-gen-croupier-skeleton"><span>protoc-gen-croupier (skeleton)</span></a></h1>
<p>This plugin turns your .proto into Croupier &quot;packs&quot;: descriptors, UI schema, a manifest and an fds.pb. It can also bundle them into <code v-pre>pack.tgz</code>.</p>
<p>Status: initial skeleton. It derives defaults when no custom options are present.
Update: custom options are supported via typed protobuf extensions (with a fallback parser for legacy descriptors).</p>
<h2 id="install-build" tabindex="-1"><a class="header-anchor" href="#install-build"><span>Install/Build</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">make croupier-plugin</span>
<span class="line"># binary at bin/protoc-gen-croupier</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="generate-with-protoc" tabindex="-1"><a class="header-anchor" href="#generate-with-protoc"><span>Generate with protoc</span></a></h2>
<p>Requires <code v-pre>protoc</code> on PATH.</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">PATH="$PWD/bin:$PATH" \</span>
<span class="line">protoc -I proto \</span>
<span class="line">  --croupier_out=emit_pack=true:gen/croupier \</span>
<span class="line">  proto/your/package/*.proto</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Artifacts go to <code v-pre>gen/croupier/</code>:</p>
<ul>
<li><code v-pre>manifest.json</code>: function list</li>
<li><code v-pre>descriptors/*.json</code>: function descriptors (transport/auth/semantics)</li>
<li><code v-pre>ui/*.schema.json</code> and <code v-pre>ui/*.uischema.json</code>: JSON Schema and UI Schema for requests</li>
<li><code v-pre>fds.pb</code>: FileDescriptorSet (types)</li>
<li><code v-pre>pack.tgz</code>: all the above bundled (if <code v-pre>emit_pack=true</code>)</li>
</ul>
<h2 id="provider-manifest-emit-manifest" tabindex="-1"><a class="header-anchor" href="#provider-manifest-emit-manifest"><span>Provider manifest (emit_manifest)</span></a></h2>
<p>When <code v-pre>emit_manifest=true</code>, the plugin additionally generates:</p>
<ul>
<li><code v-pre>manifest.json</code> with a top-level <code v-pre>provider</code> block + richer <code v-pre>functions[]</code> entries (request/response schema refs)</li>
<li><code v-pre>schema/*.json</code>: JSON Schema files for request/response messages when resolvable</li>
<li><code v-pre>descriptors.fds</code>: FileDescriptorSet in <code v-pre>.fds</code> form (same content as <code v-pre>fds.pb</code>)</li>
</ul>
<p>Suggested params:
<code v-pre>provider_id</code>, <code v-pre>provider_version</code>, <code v-pre>provider_lang</code>, <code v-pre>provider_sdk</code>, <code v-pre>provider_description</code></p>
<p>Example:</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">PATH="$PWD/bin:$PATH" \</span>
<span class="line">protoc -I proto \</span>
<span class="line">  --croupier_out=emit_pack=true,emit_manifest=true,provider_id=player,provider_version=1.0.0:gen/croupier \</span>
<span class="line">  proto/examples/games/player/v1/player.proto</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="inspect-validate-packs" tabindex="-1"><a class="header-anchor" href="#inspect-validate-packs"><span>Inspect &amp; Validate packs</span></a></h2>
<p>Use the unified CLI to inspect or validate a generated pack:</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line"># show manifest and list files</span>
<span class="line">./bin/croupier packs inspect gen/croupier/pack.tgz</span>
<span class="line"></span>
<span class="line"># validate presence of manifest/fds/descriptors for each function</span>
<span class="line">./bin/croupier packs validate gen/croupier/pack.tgz</span>
<span class="line"># or validate an extracted directory</span>
<span class="line">./bin/croupier packs validate gen/croupier</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>If you prefer a one-liner with checks, use:</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">./scripts/generate-pack.sh</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>This script builds <code v-pre>protoc-gen-croupier</code> if needed and invokes <code v-pre>protoc</code> against all files under <code v-pre>proto/</code>.</p>
<h2 id="generate-with-buf-optional" tabindex="-1"><a class="header-anchor" href="#generate-with-buf-optional"><span>Generate with buf (optional)</span></a></h2>
<p>Buf will look for <code v-pre>protoc-gen-croupier</code> on PATH.</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">PATH="$PWD/bin:$PATH" buf generate</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>Note: remote plugins in <code v-pre>buf.gen.yaml</code> may require network. You can remove them if offline.</p>
<h2 id="defaults" tabindex="-1"><a class="header-anchor" href="#defaults"><span>Defaults</span></a></h2>
<ul>
<li>function_id: <code v-pre>&lt;package&gt;.&lt;Service&gt;.&lt;Method&gt;</code> lowercased</li>
<li>version: <code v-pre>1.0.0</code></li>
<li>category: second-to-last segment of package (e.g., <code v-pre>games.player.v1</code> → <code v-pre>player</code>)</li>
<li>transport: protobuf (pb-json UI, Server encodes to pb-bin)</li>
<li>semantics: mode=query, route=lb, timeout=30s, idempotency_key=false</li>
<li>auth: permission=function_id, two_person_rule=false</li>
<li>placement: agent</li>
<li>outputs: a default <code v-pre>json.view</code></li>
</ul>
<h2 id="next-steps" tabindex="-1"><a class="header-anchor" href="#next-steps"><span>Next steps</span></a></h2>
<ul>
<li>Parse map-style options (labels/enum_map) – basic support added; improve nested parsing</li>
<li>UI annotations enrich generated UI Schema – widget/label/placeholder/sensitive/show_if/required_if supported</li>
<li>Enum detection in JSON Schema – supported (string names + enum list)</li>
<li>Map fields in JSON Schema – supported (additionalProperties)</li>
<li>Per-method route/approval/placement/timeout – supported</li>
<li>Pack signature and validation</li>
</ul>
<h2 id="supported-custom-options-current" tabindex="-1"><a class="header-anchor" href="#supported-custom-options-current"><span>Supported custom options (current)</span></a></h2>
<ul>
<li>Method option <code v-pre>(croupier.options.v1.function)</code>:
<ul>
<li><code v-pre>function_id</code>, <code v-pre>version</code>, <code v-pre>category</code>, <code v-pre>risk</code>, <code v-pre>route</code>, <code v-pre>timeout</code>, <code v-pre>two_person_rule</code>, <code v-pre>placement</code>, <code v-pre>mode</code>, <code v-pre>idempotency_key</code></li>
</ul>
</li>
<li>Field option <code v-pre>(croupier.options.v1.ui)</code>:
<ul>
<li><code v-pre>widget</code>, <code v-pre>label</code>, <code v-pre>placeholder</code>, <code v-pre>sensitive</code>, <code v-pre>show_if</code>, <code v-pre>required_if</code>, <code v-pre>enum_map</code></li>
</ul>
</li>
</ul>
<p>Example:</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">rpc Ban(BanRequest) returns (BanResponse) {</span>
<span class="line">  option (croupier.options.v1.function) = {</span>
<span class="line">    function_id: "player.ban" version: "1.2.0" risk: "high"</span>
<span class="line">    route: "lb" timeout: "30s" two_person_rule: true placement: "agent"</span>
<span class="line">    mode: "command" idempotency_key: true</span>
<span class="line">  };</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line">message BanRequest {</span>
<span class="line">  string player_id = 1 [(croupier.options.v1.ui) = { label: "玩家ID", widget: "input" }];</span>
<span class="line">  string reason    = 2 [(croupier.options.v1.ui) = { widget: "textarea", placeholder: "原因" }];</span>
<span class="line">}</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>Pack signature and validation</li>
</ul>
</div></template>


