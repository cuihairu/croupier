<template><div><h1 id="configuration-yaml-includes-profiles" tabindex="-1"><a class="header-anchor" href="#configuration-yaml-includes-profiles"><span>Configuration (YAML, Includes, Profiles)</span></a></h1>
<p>This repo contains multiple Go entrypoints. For go-zero services under <code v-pre>services/*</code> (including <code v-pre>services/server</code>), configuration is loaded via <code v-pre>github.com/zeromicro/go-zero/core/conf</code> from a YAML file passed by <code v-pre>-f/--config</code>.</p>
<p>Best practice in this repo:</p>
<ul>
<li>Put <strong>non-secret defaults</strong> in YAML (ports, feature toggles, relative paths).</li>
<li>Put <strong>secrets / per-environment DSNs</strong> in environment variables and either:
<ul>
<li>set them directly (e.g. <code v-pre>DATABASE_URL=...</code>), or</li>
<li>reference them from YAML using <code v-pre>${VAR}</code> (env expansion is enabled for <code v-pre>services/server</code>).</li>
</ul>
</li>
</ul>
<p>Notes for <code v-pre>services/server</code>:</p>
<ul>
<li>DB config keys are <code v-pre>Server.db.driver</code> and <code v-pre>Server.db.datasource</code> in YAML (not <code v-pre>Server.Database.*</code>).</li>
<li><code v-pre>DB_DRIVER</code> and <code v-pre>DATABASE_URL</code> (if set) override the YAML DB values at runtime.</li>
<li>Relative paths like <code v-pre>data/...</code>, <code v-pre>configs/...</code>, <code v-pre>packs/...</code> are resolved from the process working directory; when developing locally, run with <code v-pre>cwd=server/</code> (see <code v-pre>server/.vscode/launch.json</code>).</li>
</ul>
<p><code v-pre>services/server</code> load order (low → high)</p>
<ul>
<li>YAML file: <code v-pre>-f services/server/etc/server.yaml</code></li>
<li>YAML <code v-pre>${VAR}</code> expansion (env expansion)</li>
<li>Explicit env overrides: <code v-pre>DB_DRIVER</code>, <code v-pre>DATABASE_URL</code></li>
<li>Flags (e.g. <code v-pre>--port</code>, <code v-pre>--host</code>)</li>
</ul>
<p>Legacy CLI precedence (low → high)</p>
<ul>
<li>Base YAML: <code v-pre>--config base.yaml</code></li>
<li>Include YAMLs: <code v-pre>--config-include a.yaml --config-include b.yaml</code> (later overrides earlier)</li>
<li>Section select: <code v-pre>server:/agent:/edge:</code> (subtree of the merged YAML)</li>
<li>Profile overlay: <code v-pre>--profile &lt;name&gt;</code> (applied from section.profiles.<code v-pre>&lt;name&gt;</code>)</li>
<li>Environment: <code v-pre>CROUPIER_SERVER_* / CROUPIER_AGENT_* / CROUPIER_EDGE_*</code> (dots and dashes become underscores)</li>
<li>Flags: highest precedence</li>
</ul>
<p>Examples</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># server.example.yaml</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":8443"</span></span>
<span class="line">  <span class="token key atrule">http_addr</span><span class="token punctuation">:</span> <span class="token string">":8080"</span></span>
<span class="line">  <span class="token comment"># Database (YAML preferred; flags/env can override per-env)</span></span>
<span class="line">  <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> auto      <span class="token comment"># postgres | mysql | sqlite | auto</span></span>
<span class="line">    <span class="token key atrule">datasource</span><span class="token punctuation">:</span> <span class="token string">""</span>   <span class="token comment"># DSN/URL. Examples:</span></span>
<span class="line">    <span class="token comment"># Postgres: postgres://user:pass@host:5432/croupier?sslmode=disable</span></span>
<span class="line">    <span class="token comment"># MySQL (URL): mysql://user:pass@host:3306/croupier?charset=utf8mb4</span></span>
<span class="line">    <span class="token comment"># MySQL (DSN):  user:pass@tcp(host:3306)/croupier?parseTime=true&amp;charset=utf8mb4</span></span>
<span class="line">    <span class="token comment"># SQLite:       file:data/croupier.db  (defaults to data/croupier.db if empty)</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span> <span class="token punctuation">{</span> <span class="token key atrule">level</span><span class="token punctuation">:</span> debug<span class="token punctuation">,</span> <span class="token key atrule">format</span><span class="token punctuation">:</span> console <span class="token punctuation">}</span></span>
<span class="line">  <span class="token key atrule">metrics</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">per_function</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">per_game_denies</span><span class="token punctuation">:</span> <span class="token boolean important">false</span></span>
<span class="line">  <span class="token key atrule">profiles</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">prod</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">log</span><span class="token punctuation">:</span> <span class="token punctuation">{</span> <span class="token key atrule">level</span><span class="token punctuation">:</span> info<span class="token punctuation">,</span> <span class="token key atrule">format</span><span class="token punctuation">:</span> json<span class="token punctuation">,</span> <span class="token key atrule">file</span><span class="token punctuation">:</span> logs/server.log <span class="token punctuation">}</span></span>
<span class="line">      <span class="token key atrule">metrics</span><span class="token punctuation">:</span> <span class="token punctuation">{</span> <span class="token key atrule">per_function</span><span class="token punctuation">:</span> <span class="token boolean important">true</span> <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">Object Storage (uploads)</span>
<span class="line">```yaml</span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> s3     <span class="token comment"># s3 | cos | oss | file</span></span>
<span class="line">    <span class="token key atrule">bucket</span><span class="token punctuation">:</span> my<span class="token punctuation">-</span>bucket</span>
<span class="line">    <span class="token key atrule">region</span><span class="token punctuation">:</span> ap<span class="token punctuation">-</span>shanghai</span>
<span class="line">    <span class="token key atrule">endpoint</span><span class="token punctuation">:</span> https<span class="token punctuation">:</span>//cos.ap<span class="token punctuation">-</span>shanghai.myqcloud.com   <span class="token comment"># s3/minio/cos endpoint (optional)</span></span>
<span class="line">    <span class="token key atrule">access_key</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>STORAGE_AK<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>STORAGE_SK<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">force_path_style</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">signed_url_ttl</span><span class="token punctuation">:</span> 15m</span>
<span class="line">    <span class="token comment"># dev local:</span></span>
<span class="line">    <span class="token comment"># driver: file</span></span>
<span class="line">    <span class="token comment"># base_dir: data/uploads</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Notes:</p>
<ul>
<li>s3 覆盖 AWS/MinIO/腾讯 COS（S3 兼容模式）。COS 建议设置 <code v-pre>force_path_style=true</code>，并指定正确的 <code v-pre>region</code> 与 <code v-pre>endpoint</code>。</li>
<li>腾讯 COS 也提供官方 SDK 驱动（<code v-pre>driver: cos</code>），在 S3 兼容遇到边角不兼容时使用。</li>
<li>阿里云 OSS 使用官方 SDK 驱动（<code v-pre>driver: oss</code>）；Go Cloud 无原生 OSS 驱动。</li>
<li>file 驱动仅用于本地开发，静态路径 <code v-pre>/uploads/</code> 会映射到 <code v-pre>base_dir</code>。</li>
</ul>
<p>Tencent COS（两种方式）</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> s3  <span class="token comment"># 方式一：S3 兼容</span></span>
<span class="line">    <span class="token key atrule">bucket</span><span class="token punctuation">:</span> your<span class="token punctuation">-</span>bucket</span>
<span class="line">    <span class="token key atrule">region</span><span class="token punctuation">:</span> ap<span class="token punctuation">-</span>shanghai</span>
<span class="line">    <span class="token key atrule">endpoint</span><span class="token punctuation">:</span> https<span class="token punctuation">:</span>//cos.ap<span class="token punctuation">-</span>shanghai.myqcloud.com</span>
<span class="line">    <span class="token key atrule">access_key</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>TENCENT_SECRET_ID<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>TENCENT_SECRET_KEY<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">force_path_style</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">signed_url_ttl</span><span class="token punctuation">:</span> 15m</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或者使用官方 SDK 驱动：</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">storage</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> cos  <span class="token comment"># 方式二：官方 SDK</span></span>
<span class="line">    <span class="token key atrule">bucket</span><span class="token punctuation">:</span> your<span class="token punctuation">-</span>bucket<span class="token punctuation">-</span>APPID</span>
<span class="line">    <span class="token key atrule">region</span><span class="token punctuation">:</span> ap<span class="token punctuation">-</span>shanghai</span>
<span class="line">    <span class="token comment"># endpoint 可选： https://cos.ap-shanghai.myqcloud.com</span></span>
<span class="line">    <span class="token key atrule">access_key</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>TENCENT_SECRET_ID<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">secret_key</span><span class="token punctuation">:</span> $<span class="token punctuation">{</span>TENCENT_SECRET_KEY<span class="token punctuation">}</span></span>
<span class="line">    <span class="token key atrule">signed_url_ttl</span><span class="token punctuation">:</span> 15m</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>说明：</p>
<ul>
<li>使用 <code v-pre>force_path_style: true</code> 避免虚拟主机名路由导致的兼容问题。</li>
<li><code v-pre>region</code> 需与 COS 控制台一致，否则签名可能失败。</li>
<li>如果使用 MinIO，请将 <code v-pre>endpoint</code> 指向 MinIO 地址（如 <code v-pre>http://minio:9000</code>），并保留 <code v-pre>force_path_style: true</code>。</li>
</ul>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line"></span>
<span class="line">`services/server` quickstart:</span>
<span class="line">```bash</span>
<span class="line">cd server</span>
<span class="line"></span>
<span class="line"># SQLite (default)</span>
<span class="line">go run ./services/server -f services/server/etc/server.yaml</span>
<span class="line"></span>
<span class="line"># Postgres</span>
<span class="line">DB_DRIVER=postgres DATABASE_URL="postgres://croupier:croupier_dev_password@localhost:5432/croupier?sslmode=disable" \</span>
<span class="line">  go run ./services/server -f services/server/etc/server.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Environment overrides (<code v-pre>services/server</code>)</p>
<ul>
<li><code v-pre>DB_DRIVER</code>: <code v-pre>postgres|mysql|sqlite|sqlserver|auto</code> (default <code v-pre>auto</code>)</li>
<li><code v-pre>DATABASE_URL</code>: DSN/URL, e.g. <code v-pre>postgres://...</code> or <code v-pre>file:data/croupier.db?...</code></li>
</ul>
<p>Environment overrides (legacy CLI)</p>
<ul>
<li>Server: <code v-pre>CROUPIER_SERVER_ADDR</code>, <code v-pre>CROUPIER_SERVER_HTTP_ADDR</code>, <code v-pre>CROUPIER_SERVER_LOG_LEVEL</code>, ...</li>
<li>Agent:  <code v-pre>CROUPIER_AGENT_SERVER_ADDR</code>, <code v-pre>CROUPIER_AGENT_LOCAL_ADDR</code>, ...</li>
</ul>
<p>Metrics env toggles (server)</p>
<ul>
<li>METRICS_PER_FUNCTION=true|false to enable per-function latency histogram and counters.</li>
<li>METRICS_PER_GAME_DENIES=true|false to enable per-game RBAC deny counters.</li>
</ul>
<p>Agent Assignments &amp; Downlink (dev)</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">agent</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">assignments_api</span><span class="token punctuation">:</span> http<span class="token punctuation">:</span>//localhost<span class="token punctuation">:</span><span class="token number">8080</span>   <span class="token comment"># poll assignments and pack export from this server</span></span>
<span class="line">  <span class="token key atrule">assignments_poll_sec</span><span class="token punctuation">:</span> <span class="token number">30</span>                 <span class="token comment"># polling interval seconds</span></span>
<span class="line">  <span class="token key atrule">downlink_dir</span><span class="token punctuation">:</span> ./packs/downlink           <span class="token comment"># save/export current pack here on updates</span></span>
<span class="line">  <span class="token comment"># optional adapter process demo (dev-only)</span></span>
<span class="line">  <span class="token key atrule">adapter_prom_cmd</span><span class="token punctuation">:</span> <span class="token string">"go run ./tools/adapters/prom"</span></span>
<span class="line">  <span class="token key atrule">adapter_http_cmd</span><span class="token punctuation">:</span> <span class="token string">"go run ./tools/adapters/http"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Adapter supervisor (dev)</p>
<ul>
<li>Agent will supervise optional adapters with graceful restart and backoff.</li>
<li>Environment passed to adapter process includes: <code v-pre>CROUPIER_AGENT_ID</code>, <code v-pre>CROUPIER_GAME_ID</code>, <code v-pre>CROUPIER_ENV</code>, and passthrough <code v-pre>PROM_URL</code>/<code v-pre>ASSIGNMENTS_API</code> if present.</li>
<li>Desired adapters are inferred from assignments: <code v-pre>prom.*</code> → prom adapter, <code v-pre>http.*|grafana.*|alertmanager.*</code> → http adapter. Empty assignments means allow all → start both if configured.</li>
<li>After downlink import/reload, Agent polls <code v-pre>/api/packs/list</code> briefly to verify server responds.</li>
</ul>
<p>Adapter health &amp; logs (dev)</p>
<ul>
<li>Health (optional): set <code v-pre>adapter_prom_health_url</code> / <code v-pre>adapter_http_health_url</code> to an HTTP endpoint that returns 2xx when healthy; tune <code v-pre>adapter_health_interval_sec</code>.</li>
<li>Logs: set <code v-pre>adapter_log_dir</code> (default <code v-pre>logs/</code>), <code v-pre>adapter_log_max_mb</code>, and <code v-pre>adapter_log_backups</code> for size-based rotation of stdout/stderr per adapter.</li>
<li>Metrics: <code v-pre>/metrics.prom</code> exposes <code v-pre>croupier_adapter_running{adapter}</code>, <code v-pre>croupier_adapter_restarts_total{adapter}</code>, <code v-pre>croupier_adapter_healthy{adapter}</code>, <code v-pre>croupier_adapter_last_health_ts{adapter}</code>, <code v-pre>croupier_adapter_last_start_ts{adapter}</code>, <code v-pre>croupier_adapter_health_failures_total{adapter}</code>.</li>
<li>Optional auto-restart: set <code v-pre>adapter_health_restart_threshold</code>&gt;0 to restart adapter after N consecutive failed health checks (dev only, default disabled).</li>
</ul>
<p>Packs endpoints &amp; ETag</p>
<ul>
<li>GET <code v-pre>/api/packs/list</code> returns <code v-pre>{ manifest, counts, etag }</code> where <code v-pre>etag</code> is a content hash of the current pack (manifest/descriptors/ui/web-plugin/js/root *.pb).</li>
<li>GET <code v-pre>/api/packs/export</code> streams a tar.gz of the current pack and sets <code v-pre>ETag</code> header to the same value. Set <code v-pre>PACKS_EXPORT_REQUIRE_AUTH=true</code> to require JWT + RBAC (<code v-pre>packs:export</code>) for this endpoint (default open for Agent downlink demo).</li>
<li>POST <code v-pre>/api/packs/import</code> (RBAC: <code v-pre>packs:import</code>) imports a tar.gz and reloads descriptors/FDS.</li>
<li>POST <code v-pre>/api/packs/reload</code> (RBAC: <code v-pre>packs:reload</code>) rescans the pack directory.</li>
<li>Agent uses the <code v-pre>ETag</code> from export to confirm readiness via <code v-pre>/api/packs/list</code>.</li>
</ul>
<p>Registry API RBAC</p>
<ul>
<li>GET <code v-pre>/api/registry</code> requires <code v-pre>registry:read</code> permission; UI 页面会依据角色隐藏或禁用受限操作（后端仍强校验）。</li>
</ul>
<p>Audit API RBAC</p>
<ul>
<li>GET <code v-pre>/api/audit</code> requires <code v-pre>audit:read</code> permission; 支持 <code v-pre>game_id</code>、<code v-pre>env</code>、<code v-pre>actor</code>、<code v-pre>kind</code>、<code v-pre>limit</code> 过滤；可选 <code v-pre>offset</code> 或 <code v-pre>page</code>+<code v-pre>size</code> 分页（最新在前）。UI 支持自动刷新、导出 CSV。</li>
</ul>
<p>Assignments audit</p>
<ul>
<li>POST <code v-pre>/api/assignments</code> 会写入审计事件（kind=<code v-pre>assignments.update</code>，target=<code v-pre>&lt;game&gt;|&lt;env&gt;</code>，meta 包含 <code v-pre>functions</code> 和 <code v-pre>unknown</code>）。可通过 <code v-pre>/api/audit?kind=assignments.update</code> 查看。</li>
</ul>
<p>Effective config snapshot</p>
<ul>
<li>Validate configuration (go-zero services):</li>
</ul>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Validate server config</span></span>
<span class="line">./bin/croupier-server <span class="token parameter variable">-f</span> services/server/etc/server.yaml <span class="token parameter variable">--mode</span> <span class="token builtin class-name">test</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Or use the built-in validate command</span></span>
<span class="line">./bin/croupier-server <span class="token parameter variable">-f</span> services/server/etc/server.yaml validate</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Notes</p>
<ul>
<li>Flags always win; prefer YAML + env for deploy, flags for local dev tweaks.</li>
<li>The server binary reads <code v-pre>server.*</code> section. In CLI mode (<code v-pre>croupier server</code>), the same section applies.</li>
<li>You can keep secrets (JWT, TLS paths) in environment or external secret managers; YAML supports file paths, not secret storage.</li>
</ul>
</div></template>


