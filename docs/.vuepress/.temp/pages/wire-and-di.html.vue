<template><div><h1 id="wire-di-providers" tabindex="-1"><a class="header-anchor" href="#wire-di-providers"><span>Wire DI &amp; Providers</span></a></h1>
<p>This document summarizes the dependency injection setup using Google Wire and the available providers.</p>
<h2 id="entry-points" tabindex="-1"><a class="header-anchor" href="#entry-points"><span>Entry Points</span></a></h2>
<ul>
<li><code v-pre>InitServerApp(...)</code>: compose a Server from explicit dependencies (audit writer, RBAC policy, JWT manager, db, repos, services, etc.)</li>
<li><code v-pre>InitServerAppAuto(...)</code>: auto-construct audit/RBAC/JWT/DB/Repos/Services from environment variables</li>
</ul>
<h2 id="providers" tabindex="-1"><a class="header-anchor" href="#providers"><span>Providers</span></a></h2>
<ul>
<li>DB: <code v-pre>ProvideGormDBFromEnv()</code>
<ul>
<li><code v-pre>DB_DRIVER</code>: <code v-pre>postgres|mysql|sqlite|mssql|sqlserver|auto</code> (default <code v-pre>auto</code>)</li>
<li><code v-pre>DATABASE_URL</code>: connection string/DSN</li>
<li>Auto fallback to SQLite: <code v-pre>file:data/croupier.db</code></li>
</ul>
</li>
<li>Games defaults: <code v-pre>ProvideGamesDefaults()</code> → reads <code v-pre>configs/games.json</code> (<code v-pre>default_envs</code>)</li>
<li>RBAC policy:
<ul>
<li><code v-pre>ProvideRBACPolicyAuto()</code>
<ul>
<li>If both <code v-pre>RBAC_MODEL</code> and <code v-pre>RBAC_POLICY</code> are set, use Casbin with these paths</li>
<li>Else use <code v-pre>RBAC_CONFIG</code> (JSON or Casbin directory); JSON falls back to legacy policy</li>
</ul>
</li>
</ul>
</li>
<li>JWT manager: <code v-pre>ProvideJWTManagerFromEnv()</code>
<ul>
<li><code v-pre>JWT_SECRET</code>: HS256 secret (default <code v-pre>dev-secret</code>)</li>
</ul>
</li>
<li>Certificate store: <code v-pre>ProvideCertStore(db)</code> (GORM-backed)</li>
<li>Object store: <code v-pre>ProvideObjectStoreFromEnv()</code>
<ul>
<li><code v-pre>STORAGE_DRIVER</code>: <code v-pre>s3|file|oss|cos</code></li>
<li><code v-pre>STORAGE_BUCKET</code>, <code v-pre>STORAGE_REGION</code>, <code v-pre>STORAGE_ENDPOINT</code>, <code v-pre>STORAGE_ACCESS_KEY</code>, <code v-pre>STORAGE_SECRET_KEY</code>, <code v-pre>STORAGE_FORCE_PATH_STYLE</code></li>
<li><code v-pre>file</code> mode defaults to <code v-pre>data/uploads</code> when base dir is empty</li>
</ul>
</li>
<li>ClickHouse: <code v-pre>ProvideClickHouseFromEnv()</code>
<ul>
<li><code v-pre>CLICKHOUSE_DSN</code>: e.g., <code v-pre>clickhouse://host:port/...</code> (optional)</li>
</ul>
</li>
</ul>
<h2 id="local-development" tabindex="-1"><a class="header-anchor" href="#local-development"><span>Local Development</span></a></h2>
<ul>
<li>Install wire: <code v-pre>go install github.com/google/wire/cmd/wire@latest</code></li>
<li>Generate code: <code v-pre>make wire</code> (runs <code v-pre>wire</code> in <code v-pre>internal/app/server/http</code>)</li>
<li>Commit generated file: <code v-pre>internal/app/server/http/wire_gen.go</code> (CI validates no diff)</li>
</ul>
<h2 id="notes" tabindex="-1"><a class="header-anchor" href="#notes"><span>Notes</span></a></h2>
<ul>
<li>Handlers should be thin: decode → authorize → call service → encode</li>
<li>Services should depend only on ports, not GORM</li>
<li>Repositories (GORM) own DB models and perform persistence only</li>
</ul>
</div></template>


