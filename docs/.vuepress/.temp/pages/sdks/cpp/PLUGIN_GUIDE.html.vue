<template><div><h1 id="croupier-c-sdk-dynamic-plugin-system" tabindex="-1"><a class="header-anchor" href="#croupier-c-sdk-dynamic-plugin-system"><span>Croupier C++ SDK - Dynamic Plugin System</span></a></h1>
<p>The Croupier C++ SDK includes a comprehensive dynamic plugin system that allows you to extend functionality at runtime through dynamically loaded libraries (.dll/.so/.dylib).</p>
<h2 id="🚀-quick-start" tabindex="-1"><a class="header-anchor" href="#🚀-quick-start"><span>🚀 Quick Start</span></a></h2>
<h3 id="_1-build-the-sdk-with-plugin-support" tabindex="-1"><a class="header-anchor" href="#_1-build-the-sdk-with-plugin-support"><span>1. Build the SDK with Plugin Support</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">cmake <span class="token parameter variable">-B</span> build <span class="token parameter variable">-DBUILD_EXAMPLES</span><span class="token operator">=</span>ON <span class="token punctuation">\</span></span>
<span class="line">    <span class="token parameter variable">-DCMAKE_TOOLCHAIN_FILE</span><span class="token operator">=</span><span class="token punctuation">[</span>vcpkg-root<span class="token punctuation">]</span>/scripts/buildsystems/vcpkg.cmake</span>
<span class="line">cmake <span class="token parameter variable">--build</span> build <span class="token parameter variable">--config</span> Release</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-run-the-plugin-demo" tabindex="-1"><a class="header-anchor" href="#_2-run-the-plugin-demo"><span>2. Run the Plugin Demo</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># This will demonstrate plugin loading and function calling</span></span>
<span class="line">./build/bin/croupier-plugin-demo</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-test-with-the-example-plugin" tabindex="-1"><a class="header-anchor" href="#_3-test-with-the-example-plugin"><span>3. Test with the Example Plugin</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># The example plugin will be built automatically to:</span></span>
<span class="line"><span class="token comment"># - Windows: build/plugins/example_plugin.dll</span></span>
<span class="line"><span class="token comment"># - macOS:   build/plugins/libexample_plugin.dylib</span></span>
<span class="line"><span class="token comment"># - Linux:   build/plugins/libexample_plugin.so</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📚-plugin-development-guide" tabindex="-1"><a class="header-anchor" href="#📚-plugin-development-guide"><span>📚 Plugin Development Guide</span></a></h2>
<h3 id="plugin-interface-requirements" tabindex="-1"><a class="header-anchor" href="#plugin-interface-requirements"><span>Plugin Interface Requirements</span></a></h3>
<p>Every plugin must export these standard C functions:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">extern</span> <span class="token string">"C"</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// Initialize plugin (return 0 for success)</span></span>
<span class="line">    <span class="token keyword">int</span> <span class="token function">croupier_plugin_init</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Return plugin metadata</span></span>
<span class="line">    PluginInfo<span class="token operator">*</span> <span class="token function">croupier_plugin_info</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Cleanup plugin resources</span></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">croupier_plugin_cleanup</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="plugin-function-signature" tabindex="-1"><a class="header-anchor" href="#plugin-function-signature"><span>Plugin Function Signature</span></a></h3>
<p>All plugin functions must use this signature:</p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">extern</span> <span class="token string">"C"</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> <span class="token function">your_function_name</span><span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> context<span class="token punctuation">,</span> <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ul>
<li><strong>context</strong>: Execution context (usually JSON string)</li>
<li><strong>payload</strong>: Input data (JSON string)</li>
<li><strong>return</strong>: Result data (JSON string, must be persistent)</li>
</ul>
<h3 id="example-plugin-implementation" tabindex="-1"><a class="header-anchor" href="#example-plugin-implementation"><span>Example Plugin Implementation</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/plugin/dynamic_loader.h"</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Plugin metadata</span></span>
<span class="line"><span class="token keyword">static</span> PluginInfo plugin_info <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token string">"my_plugin"</span><span class="token punctuation">,</span>                     <span class="token comment">// name</span></span>
<span class="line">    <span class="token string">"1.0.0"</span><span class="token punctuation">,</span>                        <span class="token comment">// version</span></span>
<span class="line">    <span class="token string">"Your Name"</span><span class="token punctuation">,</span>                    <span class="token comment">// author</span></span>
<span class="line">    <span class="token string">"Plugin description"</span><span class="token punctuation">,</span>           <span class="token comment">// description</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"function1"</span><span class="token punctuation">,</span> <span class="token string">"function2"</span><span class="token punctuation">}</span><span class="token punctuation">,</span>     <span class="token comment">// provided_functions</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token punctuation">{</span><span class="token string">"license"</span><span class="token punctuation">,</span> <span class="token string">"MIT"</span><span class="token punctuation">}</span><span class="token punctuation">}</span>           <span class="token comment">// metadata</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">extern</span> <span class="token string">"C"</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">int</span> <span class="token function">croupier_plugin_init</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// Initialize resources</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span> <span class="token comment">// Success</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    PluginInfo<span class="token operator">*</span> <span class="token function">croupier_plugin_info</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token operator">&amp;</span>plugin_info<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">croupier_plugin_cleanup</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// Cleanup resources</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// Your custom functions</span></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> <span class="token function">function1</span><span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> context<span class="token punctuation">,</span> <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">static</span> std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> <span class="token raw-string string">R"({"result": "Hello from plugin!"})"</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> result<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="cmakelists-txt-for-plugin" tabindex="-1"><a class="header-anchor" href="#cmakelists-txt-for-plugin"><span>CMakeLists.txt for Plugin</span></a></h3>
<div class="language-cmake line-numbers-mode" data-highlighter="prismjs" data-ext="cmake"><pre v-pre><code class="language-cmake"><span class="line"><span class="token comment"># Create plugin shared library</span></span>
<span class="line"><span class="token keyword">add_library</span><span class="token punctuation">(</span>my_plugin <span class="token namespace">SHARED</span> my_plugin.cpp<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Include SDK headers</span></span>
<span class="line"><span class="token keyword">target_include_directories</span><span class="token punctuation">(</span>my_plugin <span class="token namespace">PRIVATE</span></span>
<span class="line">    path/to/croupier-sdk/include</span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Platform-specific settings</span></span>
<span class="line"><span class="token keyword">if</span><span class="token punctuation">(</span><span class="token variable">WIN32</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">set_target_properties</span><span class="token punctuation">(</span>my_plugin <span class="token namespace">PROPERTIES</span> <span class="token property">SUFFIX</span> <span class="token string">".dll"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">elseif</span><span class="token punctuation">(</span><span class="token variable">APPLE</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">set_target_properties</span><span class="token punctuation">(</span>my_plugin <span class="token namespace">PROPERTIES</span> <span class="token property">SUFFIX</span> <span class="token string">".dylib"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">else</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">set_target_properties</span><span class="token punctuation">(</span>my_plugin <span class="token namespace">PROPERTIES</span> <span class="token property">SUFFIX</span> <span class="token string">".so"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">endif</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔧-using-the-plugin-system" tabindex="-1"><a class="header-anchor" href="#🔧-using-the-plugin-system"><span>🔧 Using the Plugin System</span></a></h2>
<h3 id="loading-plugins" tabindex="-1"><a class="header-anchor" href="#loading-plugins"><span>Loading Plugins</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/plugin/dynamic_loader.h"</span></span></span>
<span class="line"></span>
<span class="line">PluginManager plugin_manager<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Load single plugin</span></span>
<span class="line"><span class="token keyword">bool</span> success <span class="token operator">=</span> plugin_manager<span class="token punctuation">.</span><span class="token function">LoadPlugin</span><span class="token punctuation">(</span><span class="token string">"./plugins/my_plugin.so"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Scan directory for plugins</span></span>
<span class="line"><span class="token keyword">auto</span> found_plugins <span class="token operator">=</span> plugin_manager<span class="token punctuation">.</span><span class="token function">ScanPlugins</span><span class="token punctuation">(</span><span class="token string">"./plugins"</span><span class="token punctuation">,</span> <span class="token boolean">true</span><span class="token punctuation">)</span><span class="token punctuation">;</span> <span class="token comment">// auto-load</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Enable auto-loading</span></span>
<span class="line">plugin_manager<span class="token punctuation">.</span><span class="token function">SetAutoLoading</span><span class="token punctuation">(</span><span class="token boolean">true</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="calling-plugin-functions" tabindex="-1"><a class="header-anchor" href="#calling-plugin-functions"><span>Calling Plugin Functions</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Get function handler</span></span>
<span class="line"><span class="token keyword">auto</span> handler <span class="token operator">=</span> plugin_manager<span class="token punctuation">.</span><span class="token function">GetPluginFunction</span><span class="token punctuation">(</span><span class="token string">"my_plugin.function1"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">if</span> <span class="token punctuation">(</span>handler<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> <span class="token function">handler</span><span class="token punctuation">(</span><span class="token string">"context"</span><span class="token punctuation">,</span> <span class="token raw-string string">R"({"input": "data"})"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Result: "</span> <span class="token operator">&lt;&lt;</span> result <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="integration-with-croupierclient" tabindex="-1"><a class="header-anchor" href="#integration-with-croupierclient"><span>Integration with CroupierClient</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Register all plugin functions with client</span></span>
<span class="line">plugin_manager<span class="token punctuation">.</span><span class="token function">RegisterPluginFunctions</span><span class="token punctuation">(</span>client<span class="token punctuation">,</span> <span class="token string">"my_plugin"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Now functions are available through the normal client interface</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="plugin-information" tabindex="-1"><a class="header-anchor" href="#plugin-information"><span>Plugin Information</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Get plugin metadata</span></span>
<span class="line">PluginInfo info <span class="token operator">=</span> plugin_manager<span class="token punctuation">.</span><span class="token function">GetPluginInfo</span><span class="token punctuation">(</span><span class="token string">"my_plugin"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Plugin: "</span> <span class="token operator">&lt;&lt;</span> info<span class="token punctuation">.</span>name <span class="token operator">&lt;&lt;</span> <span class="token string">" v"</span> <span class="token operator">&lt;&lt;</span> info<span class="token punctuation">.</span>version <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// List plugin functions</span></span>
<span class="line"><span class="token keyword">auto</span> functions <span class="token operator">=</span> plugin_manager<span class="token punctuation">.</span><span class="token function">GetPluginFunctions</span><span class="token punctuation">(</span><span class="token string">"my_plugin"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">for</span> <span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">auto</span><span class="token operator">&amp;</span> func <span class="token operator">:</span> functions<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Function: "</span> <span class="token operator">&lt;&lt;</span> func <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🛡️-security-considerations" tabindex="-1"><a class="header-anchor" href="#🛡️-security-considerations"><span>🛡️ Security Considerations</span></a></h2>
<h3 id="safe-loading-practices" tabindex="-1"><a class="header-anchor" href="#safe-loading-practices"><span>Safe Loading Practices</span></a></h3>
<ol>
<li><strong>Validate Plugin Sources</strong>: Only load plugins from trusted sources</li>
<li><strong>Sandbox Plugins</strong>: Consider running plugins in isolated processes</li>
<li><strong>Version Checking</strong>: Verify plugin compatibility before loading</li>
<li><strong>Error Isolation</strong>: Handle plugin errors gracefully without crashing</li>
</ol>
<h3 id="example-security-wrapper" tabindex="-1"><a class="header-anchor" href="#example-security-wrapper"><span>Example Security Wrapper</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">class</span> <span class="token class-name">SecurePluginManager</span> <span class="token punctuation">{</span></span>
<span class="line"><span class="token keyword">public</span><span class="token operator">:</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">LoadPluginSafely</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> plugin_path<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// 1. Verify file signature/checksum</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span><span class="token function">VerifyPluginIntegrity</span><span class="token punctuation">(</span>plugin_path<span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 2. Check plugin metadata</span></span>
<span class="line">        PluginManager temp_manager<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span>temp_manager<span class="token punctuation">.</span><span class="token function">LoadPlugin</span><span class="token punctuation">(</span>plugin_path<span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token keyword">auto</span> info <span class="token operator">=</span> temp_manager<span class="token punctuation">.</span><span class="token function">GetPluginInfo</span><span class="token punctuation">(</span><span class="token function">ExtractPluginName</span><span class="token punctuation">(</span>plugin_path<span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span><span class="token function">ValidatePluginSafety</span><span class="token punctuation">(</span>info<span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">            temp_manager<span class="token punctuation">.</span><span class="token function">UnloadPlugin</span><span class="token punctuation">(</span>info<span class="token punctuation">.</span>name<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 3. Load into main manager</span></span>
<span class="line">        <span class="token keyword">return</span> main_manager_<span class="token punctuation">.</span><span class="token function">LoadPlugin</span><span class="token punctuation">(</span>plugin_path<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">private</span><span class="token operator">:</span></span>
<span class="line">    PluginManager main_manager_<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🎯-example-functions" tabindex="-1"><a class="header-anchor" href="#🎯-example-functions"><span>🎯 Example Functions</span></a></h2>
<p>The included example plugin provides these functions:</p>
<h3 id="hello" tabindex="-1"><a class="header-anchor" href="#hello"><span>hello</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">Input:  <span class="token punctuation">{</span><span class="token string">"name"</span><span class="token builtin class-name">:</span> <span class="token string">"World"</span><span class="token punctuation">}</span></span>
<span class="line">Output: <span class="token punctuation">{</span><span class="token string">"message"</span><span class="token builtin class-name">:</span> <span class="token string">"Hello, World! Greetings from the example plugin."</span><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="calculate" tabindex="-1"><a class="header-anchor" href="#calculate"><span>calculate</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">Input:  <span class="token punctuation">{</span><span class="token string">"operation"</span><span class="token builtin class-name">:</span> <span class="token string">"add"</span>, <span class="token string">"a"</span><span class="token builtin class-name">:</span> <span class="token number">15</span>, <span class="token string">"b"</span><span class="token builtin class-name">:</span> <span class="token number">25</span><span class="token punctuation">}</span></span>
<span class="line">Output: <span class="token punctuation">{</span><span class="token string">"result"</span><span class="token builtin class-name">:</span> <span class="token number">40.0</span>, <span class="token string">"operation"</span><span class="token builtin class-name">:</span> <span class="token string">"add"</span>, <span class="token string">"operands"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span><span class="token string">"a"</span><span class="token builtin class-name">:</span> <span class="token number">15</span>, <span class="token string">"b"</span><span class="token builtin class-name">:</span> <span class="token number">25</span><span class="token punctuation">}</span><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="time" tabindex="-1"><a class="header-anchor" href="#time"><span>time</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">Input:  <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">Output: <span class="token punctuation">{</span><span class="token string">"timestamp"</span><span class="token builtin class-name">:</span> <span class="token number">1637123456</span>, <span class="token string">"formatted_time"</span><span class="token builtin class-name">:</span> <span class="token string">"Wed Nov 17 10:30:56 2021"</span><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="echo" tabindex="-1"><a class="header-anchor" href="#echo"><span>echo</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">Input:  <span class="token punctuation">{</span><span class="token string">"test"</span><span class="token builtin class-name">:</span> <span class="token string">"data"</span><span class="token punctuation">}</span></span>
<span class="line">Output: <span class="token punctuation">{</span><span class="token string">"echo"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span><span class="token string">"context"</span><span class="token builtin class-name">:</span> <span class="token string">"..."</span>, <span class="token string">"payload"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span><span class="token string">"test"</span><span class="token builtin class-name">:</span> <span class="token string">"data"</span><span class="token punctuation">}</span><span class="token punctuation">}</span><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔍-debugging-plugins" tabindex="-1"><a class="header-anchor" href="#🔍-debugging-plugins"><span>🔍 Debugging Plugins</span></a></h2>
<h3 id="common-issues" tabindex="-1"><a class="header-anchor" href="#common-issues"><span>Common Issues</span></a></h3>
<p><strong>Plugin fails to load</strong></p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Check error message</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string error <span class="token operator">=</span> library_manager<span class="token punctuation">.</span><span class="token function">GetLastError</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Load error: "</span> <span class="token operator">&lt;&lt;</span> error <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>Function not found</strong></p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Verify exported functions</span></span>
<span class="line"><span class="token keyword">auto</span> functions <span class="token operator">=</span> plugin_manager<span class="token punctuation">.</span><span class="token function">GetPluginFunctions</span><span class="token punctuation">(</span><span class="token string">"my_plugin"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token keyword">for</span> <span class="token punctuation">(</span><span class="token keyword">const</span> <span class="token keyword">auto</span><span class="token operator">&amp;</span> func <span class="token operator">:</span> functions<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"Available: "</span> <span class="token operator">&lt;&lt;</span> func <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>Runtime errors</strong></p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Set error callback</span></span>
<span class="line">plugin_manager<span class="token punctuation">.</span><span class="token function">SetErrorCallback</span><span class="token punctuation">(</span><span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> error<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">"Plugin error: "</span> <span class="token operator">&lt;&lt;</span> error <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="plugin-development-tips" tabindex="-1"><a class="header-anchor" href="#plugin-development-tips"><span>Plugin Development Tips</span></a></h3>
<ol>
<li><strong>Use Static Storage</strong>: Return values must persist after function returns</li>
<li><strong>Handle Exceptions</strong>: Wrap plugin code in try-catch blocks</li>
<li><strong>Memory Management</strong>: Be careful with dynamic allocations</li>
<li><strong>JSON Validation</strong>: Validate input payload format</li>
<li><strong>Thread Safety</strong>: Ensure functions are thread-safe if needed</li>
</ol>
<h2 id="📦-distribution" tabindex="-1"><a class="header-anchor" href="#📦-distribution"><span>📦 Distribution</span></a></h2>
<h3 id="plugin-packaging" tabindex="-1"><a class="header-anchor" href="#plugin-packaging"><span>Plugin Packaging</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Create plugin package</span></span>
<span class="line"><span class="token function">mkdir</span> my_plugin_v1.0.0/</span>
<span class="line"><span class="token function">cp</span> my_plugin.so my_plugin_v1.0.0/</span>
<span class="line"><span class="token function">cp</span> plugin_info.json my_plugin_v1.0.0/</span>
<span class="line"><span class="token function">cp</span> README.md my_plugin_v1.0.0/</span>
<span class="line"><span class="token function">tar</span> <span class="token parameter variable">-czf</span> my_plugin_v1.0.0.tar.gz my_plugin_v1.0.0/</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="installation" tabindex="-1"><a class="header-anchor" href="#installation"><span>Installation</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Install plugin</span></span>
<span class="line"><span class="token function">tar</span> <span class="token parameter variable">-xzf</span> my_plugin_v1.0.0.tar.gz</span>
<span class="line"><span class="token function">cp</span> my_plugin_v1.0.0/my_plugin.so /usr/local/lib/croupier/plugins/</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<p>🎮 <strong>Build powerful, extensible game backends with the Croupier Plugin System!</strong></p>
</div></template>


