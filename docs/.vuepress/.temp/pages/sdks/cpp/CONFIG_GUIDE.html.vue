<template><div><h1 id="croupier-c-sdk-advanced-configuration-system" tabindex="-1"><a class="header-anchor" href="#croupier-c-sdk-advanced-configuration-system"><span>Croupier C++ SDK - Advanced Configuration System</span></a></h1>
<p>The Croupier C++ SDK now features a comprehensive, modular configuration system that supports multiple environments, validation, and flexible deployment scenarios.</p>
<h2 id="🏗️-architecture-overview" tabindex="-1"><a class="header-anchor" href="#🏗️-architecture-overview"><span>🏗️ Architecture Overview</span></a></h2>
<p>The configuration system is built with the following modular components:</p>
<h3 id="core-modules" tabindex="-1"><a class="header-anchor" href="#core-modules"><span>Core Modules</span></a></h3>
<ul>
<li><strong><code v-pre>utils/JsonUtils</code></strong> - Cross-platform JSON processing with nlohmann/json support</li>
<li><strong><code v-pre>utils/FileSystemUtils</code></strong> - File and directory operations across platforms</li>
<li><strong><code v-pre>config/ClientConfigLoader</code></strong> - Client configuration loading and validation</li>
<li><strong><code v-pre>ConfigDrivenLoader</code></strong> - Component and virtual object loading from configuration</li>
</ul>
<h3 id="benefits" tabindex="-1"><a class="header-anchor" href="#benefits"><span>Benefits</span></a></h3>
<p>✅ <strong>Modular Design</strong> - Each utility is in its own file for maintainability
✅ <strong>Environment Support</strong> - Development, staging, production configurations
✅ <strong>Validation</strong> - Comprehensive configuration validation with detailed errors
✅ <strong>Override Support</strong> - Environment variables can override file settings
✅ <strong>Cross-Platform</strong> - Works on Windows, Linux, and macOS
✅ <strong>Schema Support</strong> - Virtual object schemas with field validation</p>
<h2 id="🚀-quick-start" tabindex="-1"><a class="header-anchor" href="#🚀-quick-start"><span>🚀 Quick Start</span></a></h2>
<h3 id="_1-basic-configuration-loading" tabindex="-1"><a class="header-anchor" href="#_1-basic-configuration-loading"><span>1. Basic Configuration Loading</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/config/client_config_loader.h"</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Load configuration</span></span>
<span class="line">ClientConfigLoader loader<span class="token punctuation">;</span></span>
<span class="line">ClientConfig config <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">LoadFromFile</span><span class="token punctuation">(</span><span class="token string">"./configs/development.json"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Validate configuration</span></span>
<span class="line"><span class="token keyword">auto</span> errors <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">ValidateConfig</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span>errors<span class="token punctuation">.</span><span class="token function">empty</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">for</span> <span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">auto</span><span class="token operator">&amp;</span> error <span class="token operator">:</span> errors<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Config error: "</span> <span class="token operator">&lt;&lt;</span> error <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Create client</span></span>
<span class="line">CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-environment-based-configuration" tabindex="-1"><a class="header-anchor" href="#_2-environment-based-configuration"><span>2. Environment-Based Configuration</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Load configuration with environment variable overrides</span></span>
<span class="line">ClientConfig config <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">LoadWithEnvironmentOverrides</span><span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"./configs/production.json"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"CROUPIER_"</span>  <span class="token comment">// Environment variable prefix</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Environment variables like CROUPIER_GAME_ID will override config file values</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-profile-based-loading" tabindex="-1"><a class="header-anchor" href="#_3-profile-based-loading"><span>3. Profile-Based Loading</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Load configuration profile (base + environment-specific)</span></span>
<span class="line">ClientConfig config <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">LoadProfile</span><span class="token punctuation">(</span><span class="token string">"./configs"</span><span class="token punctuation">,</span> <span class="token string">"production"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// This loads base.json and production.json, merging them together</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📁-configuration-file-structure" tabindex="-1"><a class="header-anchor" href="#📁-configuration-file-structure"><span>📁 Configuration File Structure</span></a></h2>
<h3 id="environment-specific-configurations" tabindex="-1"><a class="header-anchor" href="#environment-specific-configurations"><span>Environment-Specific Configurations</span></a></h3>
<p>Create separate configuration files for each environment:</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">configs/</span>
<span class="line">├── development.json    # Development environment</span>
<span class="line">├── staging.json       # Staging environment</span>
<span class="line">├── production.json    # Production environment</span>
<span class="line">└── base.json          # Common base configuration</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="development-configuration-example" tabindex="-1"><a class="header-anchor" href="#development-configuration-example"><span>Development Configuration Example</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game-dev"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"development"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"service_id"</span><span class="token operator">:</span> <span class="token string">"backend-dev"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"agent_addr"</span><span class="token operator">:</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"local_listen"</span><span class="token operator">:</span> <span class="token string">"0.0.0.0:0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"insecure"</span><span class="token operator">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"timeout_seconds"</span><span class="token operator">:</span> <span class="token number">30</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"headers"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"X-Game-Version"</span><span class="token operator">:</span> <span class="token string">"1.0.0-dev"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"X-Environment"</span><span class="token operator">:</span> <span class="token string">"development"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="production-configuration-example" tabindex="-1"><a class="header-anchor" href="#production-configuration-example"><span>Production Configuration Example</span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"game_id"</span><span class="token operator">:</span> <span class="token string">"my-game-prod"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"env"</span><span class="token operator">:</span> <span class="token string">"production"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"service_id"</span><span class="token operator">:</span> <span class="token string">"backend-prod-01"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"agent_addr"</span><span class="token operator">:</span> <span class="token string">"croupier-agent.internal:19090"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"local_listen"</span><span class="token operator">:</span> <span class="token string">"0.0.0.0:0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"insecure"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"timeout_seconds"</span><span class="token operator">:</span> <span class="token number">60</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"security"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"cert_file"</span><span class="token operator">:</span> <span class="token string">"/etc/tls/client.crt"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"key_file"</span><span class="token operator">:</span> <span class="token string">"/etc/tls/client.key"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"ca_file"</span><span class="token operator">:</span> <span class="token string">"/etc/tls/ca.crt"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"server_name"</span><span class="token operator">:</span> <span class="token string">"croupier.internal"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"token"</span><span class="token operator">:</span> <span class="token string">"Bearer ${JWT_TOKEN}"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"headers"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"X-Game-Version"</span><span class="token operator">:</span> <span class="token string">"2.1.0"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"X-Service-Region"</span><span class="token operator">:</span> <span class="token string">"us-west-2"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔧-environment-variable-overrides" tabindex="-1"><a class="header-anchor" href="#🔧-environment-variable-overrides"><span>🔧 Environment Variable Overrides</span></a></h2>
<p>You can override any configuration value using environment variables with the <code v-pre>CROUPIER_</code> prefix:</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Override basic settings</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_GAME_ID</span><span class="token operator">=</span><span class="token string">"my-override-game"</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_ENV</span><span class="token operator">=</span><span class="token string">"staging"</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_AGENT_ADDR</span><span class="token operator">=</span><span class="token string">"staging-agent:19090"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Override security settings</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_INSECURE</span><span class="token operator">=</span><span class="token string">"false"</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_CERT_FILE</span><span class="token operator">=</span><span class="token string">"/path/to/cert.pem"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Override authentication</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_AUTH_TOKEN</span><span class="token operator">=</span><span class="token string">"Bearer abc123..."</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Run your application</span></span>
<span class="line">./your-app</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🛠️-building-and-running-examples" tabindex="-1"><a class="header-anchor" href="#🛠️-building-and-running-examples"><span>🛠️ Building and Running Examples</span></a></h2>
<h3 id="build-the-sdk-with-examples" tabindex="-1"><a class="header-anchor" href="#build-the-sdk-with-examples"><span>Build the SDK with Examples</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Configure build</span></span>
<span class="line">cmake <span class="token parameter variable">-B</span> build <span class="token parameter variable">-DBUILD_EXAMPLES</span><span class="token operator">=</span>ON <span class="token punctuation">\</span></span>
<span class="line">    <span class="token parameter variable">-DCMAKE_TOOLCHAIN_FILE</span><span class="token operator">=</span><span class="token punctuation">[</span>vcpkg-root<span class="token punctuation">]</span>/scripts/buildsystems/vcpkg.cmake</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Build all targets</span></span>
<span class="line">cmake <span class="token parameter variable">--build</span> build <span class="token parameter variable">--config</span> Release</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Examples will be in build/bin/</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="run-configuration-example" tabindex="-1"><a class="header-anchor" href="#run-configuration-example"><span>Run Configuration Example</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Run with default (development) configuration</span></span>
<span class="line">./build/bin/croupier-config-example</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Run with specific environment</span></span>
<span class="line">./build/bin/croupier-config-example production</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Use environment variable</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">CROUPIER_ENV</span><span class="token operator">=</span>staging</span>
<span class="line">./build/bin/croupier-config-example</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📊-configuration-validation" tabindex="-1"><a class="header-anchor" href="#📊-configuration-validation"><span>📊 Configuration Validation</span></a></h2>
<p>The system provides comprehensive validation with detailed error messages:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">auto</span> errors <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">ValidateConfig</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">for</span> <span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">auto</span><span class="token operator">&amp;</span> error <span class="token operator">:</span> errors<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"❌ "</span> <span class="token operator">&lt;&lt;</span> error <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>Common validation checks:</p>
<ul>
<li>✅ Required fields (game_id, agent_addr)</li>
<li>✅ Network address formats (host:port)</li>
<li>✅ File path existence (TLS certificates)</li>
<li>✅ Environment values (development, staging, production)</li>
<li>✅ Security configuration consistency</li>
<li>✅ Authentication token formats</li>
</ul>
<h2 id="🔒-security-configuration" tabindex="-1"><a class="header-anchor" href="#🔒-security-configuration"><span>🔒 Security Configuration</span></a></h2>
<p>For production deployments, the SDK supports comprehensive TLS configuration:</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"insecure"</span><span class="token operator">:</span> <span class="token boolean">false</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"security"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"cert_file"</span><span class="token operator">:</span> <span class="token string">"/etc/tls/croupier/client.crt"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"key_file"</span><span class="token operator">:</span> <span class="token string">"/etc/tls/croupier/client.key"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"ca_file"</span><span class="token operator">:</span> <span class="token string">"/etc/tls/croupier/ca.crt"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"server_name"</span><span class="token operator">:</span> <span class="token string">"croupier.internal"</span></span>
<span class="line">  <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"auth"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"token"</span><span class="token operator">:</span> <span class="token string">"Bearer eyJhbGciOiJIUzI1NiIs..."</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"headers"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"X-Client-ID"</span><span class="token operator">:</span> <span class="token string">"backend-service-01"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"X-Service-Region"</span><span class="token operator">:</span> <span class="token string">"us-west-2"</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🐳-docker-deployment" tabindex="-1"><a class="header-anchor" href="#🐳-docker-deployment"><span>🐳 Docker Deployment</span></a></h2>
<h3 id="dockerfile-example" tabindex="-1"><a class="header-anchor" href="#dockerfile-example"><span>Dockerfile Example</span></a></h3>
<div class="language-docker line-numbers-mode" data-highlighter="prismjs" data-ext="docker"><pre v-pre><code class="language-docker"><span class="line"><span class="token instruction"><span class="token keyword">FROM</span> ubuntu:22.04</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Install runtime dependencies</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">RUN</span> apt-get update &amp;&amp; apt-get install -y <span class="token operator">\</span></span>
<span class="line">    libgrpc++1.54 <span class="token operator">\</span></span>
<span class="line">    libprotobuf32 <span class="token operator">\</span></span>
<span class="line">    &amp;&amp; rm -rf /var/lib/apt/lists/*</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Copy application and configs</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">COPY</span> build/bin/your-app /usr/local/bin/</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">COPY</span> configs/ /etc/croupier/configs/</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Set environment</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">ENV</span> CROUPIER_ENV=production</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">ENV</span> CROUPIER_GAME_ID=your-production-game</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Run application</span></span>
<span class="line"><span class="token instruction"><span class="token keyword">CMD</span> [<span class="token string">"/usr/local/bin/your-app"</span>]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="kubernetes-deployment" tabindex="-1"><a class="header-anchor" href="#kubernetes-deployment"><span>Kubernetes Deployment</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">apiVersion</span><span class="token punctuation">:</span> apps/v1</span>
<span class="line"><span class="token key atrule">kind</span><span class="token punctuation">:</span> Deployment</span>
<span class="line"><span class="token key atrule">metadata</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">name</span><span class="token punctuation">:</span> game<span class="token punctuation">-</span>backend</span>
<span class="line"><span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">replicas</span><span class="token punctuation">:</span> <span class="token number">3</span></span>
<span class="line">  <span class="token key atrule">template</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">spec</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token key atrule">containers</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> backend</span>
<span class="line">        <span class="token key atrule">image</span><span class="token punctuation">:</span> your<span class="token punctuation">-</span>registry/game<span class="token punctuation">-</span>backend<span class="token punctuation">:</span>latest</span>
<span class="line">        <span class="token key atrule">env</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> CROUPIER_ENV</span>
<span class="line">          <span class="token key atrule">value</span><span class="token punctuation">:</span> <span class="token string">"production"</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> CROUPIER_GAME_ID</span>
<span class="line">          <span class="token key atrule">valueFrom</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">configMapKeyRef</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">name</span><span class="token punctuation">:</span> game<span class="token punctuation">-</span>config</span>
<span class="line">              <span class="token key atrule">key</span><span class="token punctuation">:</span> game<span class="token punctuation">-</span>id</span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> CROUPIER_AUTH_TOKEN</span>
<span class="line">          <span class="token key atrule">valueFrom</span><span class="token punctuation">:</span></span>
<span class="line">            <span class="token key atrule">secretKeyRef</span><span class="token punctuation">:</span></span>
<span class="line">              <span class="token key atrule">name</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>auth</span>
<span class="line">              <span class="token key atrule">key</span><span class="token punctuation">:</span> jwt<span class="token punctuation">-</span>token</span>
<span class="line">        <span class="token key atrule">volumeMounts</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> tls<span class="token punctuation">-</span>certs</span>
<span class="line">          <span class="token key atrule">mountPath</span><span class="token punctuation">:</span> /etc/tls</span>
<span class="line">          <span class="token key atrule">readOnly</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">      <span class="token key atrule">volumes</span><span class="token punctuation">:</span></span>
<span class="line">      <span class="token punctuation">-</span> <span class="token key atrule">name</span><span class="token punctuation">:</span> tls<span class="token punctuation">-</span>certs</span>
<span class="line">        <span class="token key atrule">secret</span><span class="token punctuation">:</span></span>
<span class="line">          <span class="token key atrule">secretName</span><span class="token punctuation">:</span> croupier<span class="token punctuation">-</span>tls</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🧪-testing-your-configuration" tabindex="-1"><a class="header-anchor" href="#🧪-testing-your-configuration"><span>🧪 Testing Your Configuration</span></a></h2>
<p>Use the provided example to test your configuration:</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Test development configuration</span></span>
<span class="line">./build/bin/croupier-config-example development</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Test production configuration</span></span>
<span class="line">./build/bin/croupier-config-example production</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Test with environment overrides</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_GAME_ID</span><span class="token operator">=</span><span class="token string">"test-game"</span> <span class="token punctuation">\</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_TIMEOUT_SECONDS</span><span class="token operator">=</span><span class="token string">"60"</span> <span class="token punctuation">\</span></span>
<span class="line">./build/bin/croupier-config-example</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🚨-troubleshooting" tabindex="-1"><a class="header-anchor" href="#🚨-troubleshooting"><span>🚨 Troubleshooting</span></a></h2>
<h3 id="common-issues" tabindex="-1"><a class="header-anchor" href="#common-issues"><span>Common Issues</span></a></h3>
<p><strong>Configuration file not found</strong></p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">⚠️ Configuration file not found: ./configs/production.json</span>
<span class="line">📄 Generating example configuration...</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><p><em>Solution: The SDK will generate an example configuration file for you to edit.</em></p>
<p><strong>TLS certificate not found</strong></p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">❌ cert_file does not exist: /etc/tls/client.crt</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><em>Solution: Ensure TLS certificates exist or set <code v-pre>insecure: true</code> for development.</em></p>
<p><strong>Invalid network address</strong></p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">❌ agent_addr format is invalid (should be host:port)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p><em>Solution: Use proper host:port format like <code v-pre>127.0.0.1:19090</code> or <code v-pre>agent.internal:19090</code></em></p>
<p><strong>Environment variable override not working</strong></p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Make sure to use the LoadWithEnvironmentOverrides method</span></span>
<span class="line">ClientConfig config <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">LoadWithEnvironmentOverrides</span><span class="token punctuation">(</span></span>
<span class="line">    config_file<span class="token punctuation">,</span></span>
<span class="line">    <span class="token string">"CROUPIER_"</span>  <span class="token comment">// This prefix is important</span></span>
<span class="line"><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="debug-configuration-loading" tabindex="-1"><a class="header-anchor" href="#debug-configuration-loading"><span>Debug Configuration Loading</span></a></h3>
<p>Enable verbose output in your application:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Add debug logging</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Loading config from: "</span> <span class="token operator">&lt;&lt;</span> config_file <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">ClientConfig config <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">LoadFromFile</span><span class="token punctuation">(</span>config_file<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Validate and show errors</span></span>
<span class="line"><span class="token keyword">auto</span> errors <span class="token operator">=</span> loader<span class="token punctuation">.</span><span class="token function">ValidateConfig</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">for</span> <span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">auto</span><span class="token operator">&amp;</span> error <span class="token operator">:</span> errors<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Validation error: "</span> <span class="token operator">&lt;&lt;</span> error <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📖-api-reference" tabindex="-1"><a class="header-anchor" href="#📖-api-reference"><span>📖 API Reference</span></a></h2>
<h3 id="clientconfigloader-methods" tabindex="-1"><a class="header-anchor" href="#clientconfigloader-methods"><span>ClientConfigLoader Methods</span></a></h3>
<ul>
<li><code v-pre>LoadFromFile(file_path)</code> - Load configuration from JSON file</li>
<li><code v-pre>LoadFromJson(json_content)</code> - Load configuration from JSON string</li>
<li><code v-pre>LoadWithEnvironmentOverrides(file_path, prefix)</code> - Load with env var overrides</li>
<li><code v-pre>LoadProfile(config_dir, profile)</code> - Load configuration profile</li>
<li><code v-pre>ValidateConfig(config)</code> - Validate configuration and return errors</li>
<li><code v-pre>GenerateExampleConfig(environment)</code> - Generate example configuration</li>
<li><code v-pre>CreateDefaultConfig()</code> - Create configuration with defaults</li>
<li><code v-pre>MergeConfigs(base, overlay)</code> - Merge two configurations</li>
</ul>
<h3 id="utility-classes" tabindex="-1"><a class="header-anchor" href="#utility-classes"><span>Utility Classes</span></a></h3>
<ul>
<li><code v-pre>utils::JsonUtils</code> - JSON processing utilities</li>
<li><code v-pre>utils::FileSystemUtils</code> - Cross-platform file operations</li>
</ul>
<hr>
<p>🎮 <strong>Ready to build next-generation game backends with Croupier C++ SDK!</strong></p>
</div></template>


