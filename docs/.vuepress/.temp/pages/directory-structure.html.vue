<template><div><h1 id="directory-structure-go-zero-multi-process" tabindex="-1"><a class="header-anchor" href="#directory-structure-go-zero-multi-process"><span>Directory Structure (Go-Zero Multi-Process)</span></a></h1>
<p>The repository now standardizes on <a href="https://go-zero.dev/" target="_blank" rel="noopener noreferrer">go-zero</a> across all backend services. Each process (server, agent, edge, analytics-worker, …) manages its own generated code, GORM models, and migrations under <code v-pre>services/&lt;name&gt;/</code>. This document captures the conventions to keep those services consistent.</p>
<h2 id="top-level-layout" tabindex="-1"><a class="header-anchor" href="#top-level-layout"><span>Top-Level Layout</span></a></h2>
<ul>
<li><code v-pre>cmd/</code>            Legacy entrypoints and helper binaries.</li>
<li><code v-pre>services/</code>       Go-zero applications (<code v-pre>server</code>, <code v-pre>agent</code>, <code v-pre>edge</code>, <code v-pre>ingest</code>, …).</li>
<li><code v-pre>internal/</code>       Shared libraries (auth, db helpers, schedulers, etc.).</li>
<li><code v-pre>pkg/</code>            Exported helper packages (rare; only for stable APIs).</li>
<li><code v-pre>configs/</code>        Global YAML, RBAC, bootstrap data.</li>
<li><code v-pre>proto/</code> &amp; <code v-pre>gen/</code> Protobuf definitions and generated stubs.</li>
<li><code v-pre>web/</code>            Frontend (Umi Max + Ant Design 5).</li>
<li><code v-pre>scripts/</code>, <code v-pre>tools/</code>, <code v-pre>docs/</code>, <code v-pre>packs/</code>, <code v-pre>data/</code> remain unchanged.</li>
</ul>
<h2 id="go-zero-service-layout" tabindex="-1"><a class="header-anchor" href="#go-zero-service-layout"><span>Go-Zero Service Layout</span></a></h2>
<p>Each service inside <code v-pre>services/&lt;name&gt;</code> follows the go-zero scaffold:</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">services/&lt;name>/</span>
<span class="line">  server.go                # main()</span>
<span class="line">  etc/                     # config yaml files</span>
<span class="line">  cmd/                     # optional process-specific commands (migrate, seed…)</span>
<span class="line">  internal/</span>
<span class="line">    config/                # goctl generated config structs</span>
<span class="line">    handler/               # HTTP/gRPC handlers (wire request ↔ logic)</span>
<span class="line">    logic/                 # business logic (generated skeletons + custom code)</span>
<span class="line">    model/                 # GORM models + data access helpers</span>
<span class="line">    svc/                   # ServiceContext (wires config, db, models, services)</span>
<span class="line">    middleware/, runtime/, common/ … (service-scoped helpers)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Key layering inside a service:</p>
<ul>
<li><code v-pre>handler → logic → svc → model</code>.</li>
<li>Handlers perform decoding/encoding + auth guard only.</li>
<li>Logic packages contain the use-cases and should rely on interfaces exposed through <code v-pre>svc.ServiceContext</code>.</li>
<li><code v-pre>svc</code> wires configs, DB clients, models, and auxiliary services.</li>
<li><code v-pre>internal/model</code> owns the GORM structs, migrations, and persistence helpers for <strong>that process only</strong>.</li>
</ul>
<h2 id="model-migration-guidelines" tabindex="-1"><a class="header-anchor" href="#model-migration-guidelines"><span>Model &amp; Migration Guidelines</span></a></h2>
<ul>
<li>Every go-zero service keeps its database models under <code v-pre>services/&lt;name&gt;/internal/model</code>.</li>
<li>Models use GORM and expose helper structs (e.g. <code v-pre>AdminModel</code>) instead of the old <code v-pre>internal/repo/gorm</code> adapters.</li>
<li><code v-pre>svc/service_context.go</code> must invoke <code v-pre>&lt;model&gt;.AutoMigrate(db)</code> so each process migrates only the tables it owns.</li>
<li>Cross-process data sharing happens through APIs; do <strong>not</strong> import another service's <code v-pre>internal/model</code>.</li>
<li>Legacy <code v-pre>internal/repo/gorm/*</code> packages are considered deprecated. Do not add new dependencies to them; migrate features into the corresponding service's <code v-pre>internal/model</code> as you touch them.</li>
</ul>
<h2 id="shared-internal-packages" tabindex="-1"><a class="header-anchor" href="#shared-internal-packages"><span>Shared Internal Packages</span></a></h2>
<p>The <code v-pre>internal/</code> directory (outside <code v-pre>services/</code>) now only hosts code that is safe to share across processes, such as:</p>
<ul>
<li><code v-pre>internal/auth/*</code> – token helpers, permission checks (may depend on service models via interfaces).</li>
<li><code v-pre>internal/database/*</code> – helpers for opening and configuring GORM/SQL connections.</li>
<li><code v-pre>internal/platform/*</code> – integrations (object storage, TLS, packaging).</li>
<li><code v-pre>internal/security/*</code> – RBAC loaders, JWT tooling.</li>
</ul>
<p>When a shared package needs to inspect service-specific tables, inject the required model interface from the service instead of importing the model package directly. This keeps the dependency direction from service → shared helper.</p>
<h2 id="dependency-flow" tabindex="-1"><a class="header-anchor" href="#dependency-flow"><span>Dependency Flow</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">handler  →  logic  →  svc.ServiceContext  →  internal/model</span>
<span class="line">                    ↘ shared internal helpers (auth, db, platform)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li>Handlers never import <code v-pre>internal/model</code>.</li>
<li>Logic only touches persistence via the interfaces exposed on <code v-pre>svc.ServiceContext</code>.</li>
<li>Shared internal helpers must remain infrastructure-only; no business logic there.</li>
</ul>
<h2 id="testing-guidance" tabindex="-1"><a class="header-anchor" href="#testing-guidance"><span>Testing Guidance</span></a></h2>
<ul>
<li>Logic: use go-zero generated mocks or hand-rolled stubs for the interfaces exposed via <code v-pre>svc</code>.</li>
<li>Model: prefer sqlite-in-memory or dedicated test schemas; call <code v-pre>model.AutoMigrate</code> inside tests.</li>
<li>Handler: thin HTTP tests verifying routing/middleware.</li>
<li>Each service owns its own test data/migrations; do not rely on other services' fixtures.</li>
</ul>
<h2 id="migration-notes" tabindex="-1"><a class="header-anchor" href="#migration-notes"><span>Migration Notes</span></a></h2>
<ul>
<li>When moving legacy code from <code v-pre>internal/repo/gorm</code>, port the structs into the relevant <code v-pre>services/&lt;name&gt;/internal/model</code> package and wire them through that service's <code v-pre>svc</code>.</li>
<li>Update docs and examples as soon as a service completes migration to avoid confusion between old Ports/Adapters notes and the go-zero layout.</li>
</ul>
</div></template>


