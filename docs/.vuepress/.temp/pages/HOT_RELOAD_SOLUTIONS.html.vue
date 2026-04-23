<template><div><h1 id="🔥-游戏开发热更新方案总览" tabindex="-1"><a class="header-anchor" href="#🔥-游戏开发热更新方案总览"><span>🔥 游戏开发热更新方案总览</span></a></h1>
<h2 id="📊-各语言热更新技术对比" tabindex="-1"><a class="header-anchor" href="#📊-各语言热更新技术对比"><span>📊 各语言热更新技术对比</span></a></h2>
<h3 id="go语言热更新" tabindex="-1"><a class="header-anchor" href="#go语言热更新"><span><strong>Go语言热更新</strong></span></a></h3>
<table>
<thead>
<tr>
<th>方案</th>
<th>类型</th>
<th>性能</th>
<th>易用性</th>
<th>生产适用</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Air</strong></td>
<td>进程重启</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>配置简单，开发常用</td>
</tr>
<tr>
<td><strong>Realize</strong></td>
<td>进程重启</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>功能丰富，支持多项目</td>
</tr>
<tr>
<td><strong>Plugin热加载</strong></td>
<td>动态库</td>
<td>高</td>
<td>低</td>
<td>是</td>
<td>真正热更新，但复杂</td>
</tr>
<tr>
<td><strong>配置热载</strong></td>
<td>配置刷新</td>
<td>高</td>
<td>高</td>
<td>是</td>
<td>仅配置，不涉及代码</td>
</tr>
</tbody>
</table>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// Air 配置示例</span></span>
<span class="line"><span class="token comment">// .air.toml</span></span>
<span class="line"><span class="token punctuation">[</span>build<span class="token punctuation">]</span></span>
<span class="line">cmd <span class="token operator">=</span> <span class="token string">"go build -o ./bin/game-server ./cmd/server"</span></span>
<span class="line">bin <span class="token operator">=</span> <span class="token string">"bin/game-server"</span></span>
<span class="line">include_ext <span class="token operator">=</span> <span class="token punctuation">[</span><span class="token string">"go"</span><span class="token punctuation">,</span> <span class="token string">"yaml"</span><span class="token punctuation">,</span> <span class="token string">"json"</span><span class="token punctuation">]</span></span>
<span class="line">exclude_dir <span class="token operator">=</span> <span class="token punctuation">[</span><span class="token string">"bin"</span><span class="token punctuation">,</span> <span class="token string">"vendor"</span><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="javascript-node-js热更新" tabindex="-1"><a class="header-anchor" href="#javascript-node-js热更新"><span><strong>JavaScript/Node.js热更新</strong></span></a></h3>
<table>
<thead>
<tr>
<th>方案</th>
<th>类型</th>
<th>性能</th>
<th>易用性</th>
<th>生产适用</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Nodemon</strong></td>
<td>进程重启</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>经典方案，开发必备</td>
</tr>
<tr>
<td><strong>PM2 Reload</strong></td>
<td>无缝重启</td>
<td>高</td>
<td>中</td>
<td>是</td>
<td>零停机时间重启</td>
</tr>
<tr>
<td><strong>Module热替换</strong></td>
<td>模块替换</td>
<td>高</td>
<td>低</td>
<td>是</td>
<td>真正的热更新</td>
</tr>
<tr>
<td><strong>Webpack HMR</strong></td>
<td>模块热替换</td>
<td>高</td>
<td>中</td>
<td>否</td>
<td>前端开发利器</td>
</tr>
</tbody>
</table>
<div class="language-javascript line-numbers-mode" data-highlighter="prismjs" data-ext="js"><pre v-pre><code class="language-javascript"><span class="line"><span class="token comment">// Nodemon 配置</span></span>
<span class="line"><span class="token comment">// nodemon.json</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string-property property">"watch"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"src"</span><span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string-property property">"ext"</span><span class="token operator">:</span> <span class="token string">"js,json"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string-property property">"exec"</span><span class="token operator">:</span> <span class="token string">"node server.js"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string-property property">"env"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token string-property property">"NODE_ENV"</span><span class="token operator">:</span> <span class="token string">"development"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="python热更新" tabindex="-1"><a class="header-anchor" href="#python热更新"><span><strong>Python热更新</strong></span></a></h3>
<table>
<thead>
<tr>
<th>方案</th>
<th>类型</th>
<th>性能</th>
<th>易用性</th>
<th>生产适用</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Watchdog</strong></td>
<td>文件监听</td>
<td>中</td>
<td>中</td>
<td>否</td>
<td>灵活的文件系统监听</td>
</tr>
<tr>
<td><strong>Uvicorn --reload</strong></td>
<td>进程重启</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>ASGI服务器自动重载</td>
</tr>
<tr>
<td><strong>importlib.reload</strong></td>
<td>模块重载</td>
<td>高</td>
<td>低</td>
<td>是</td>
<td>运行时模块重新加载</td>
</tr>
<tr>
<td><strong>Django Auto-reload</strong></td>
<td>进程重启</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>Django开发服务器</td>
</tr>
</tbody>
</table>
<div class="language-python line-numbers-mode" data-highlighter="prismjs" data-ext="py"><pre v-pre><code class="language-python"><span class="line"><span class="token comment"># Watchdog 示例</span></span>
<span class="line"><span class="token keyword">from</span> watchdog<span class="token punctuation">.</span>observers <span class="token keyword">import</span> Observer</span>
<span class="line"><span class="token keyword">from</span> watchdog<span class="token punctuation">.</span>events <span class="token keyword">import</span> FileSystemEventHandler</span>
<span class="line"></span>
<span class="line"><span class="token keyword">class</span> <span class="token class-name">ReloadHandler</span><span class="token punctuation">(</span>FileSystemEventHandler<span class="token punctuation">)</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token keyword">def</span> <span class="token function">on_modified</span><span class="token punctuation">(</span>self<span class="token punctuation">,</span> event<span class="token punctuation">)</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">if</span> event<span class="token punctuation">.</span>src_path<span class="token punctuation">.</span>endswith<span class="token punctuation">(</span><span class="token string">'.py'</span><span class="token punctuation">)</span><span class="token punctuation">:</span></span>
<span class="line">            reload_module<span class="token punctuation">(</span>event<span class="token punctuation">.</span>src_path<span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="java热更新" tabindex="-1"><a class="header-anchor" href="#java热更新"><span><strong>Java热更新</strong></span></a></h3>
<table>
<thead>
<tr>
<th>方案</th>
<th>类型</th>
<th>性能</th>
<th>易用性</th>
<th>生产适用</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>JRebel</strong></td>
<td>字节码替换</td>
<td>高</td>
<td>高</td>
<td>是</td>
<td>商业产品，功能最强</td>
</tr>
<tr>
<td><strong>Spring DevTools</strong></td>
<td>类重载</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>Spring生态首选</td>
</tr>
<tr>
<td><strong>HotSwapAgent</strong></td>
<td>JVM增强</td>
<td>高</td>
<td>中</td>
<td>是</td>
<td>开源JRebel替代</td>
</tr>
<tr>
<td><strong>DCEVM</strong></td>
<td>JVM修改</td>
<td>高</td>
<td>低</td>
<td>是</td>
<td>需要特殊JVM</td>
</tr>
</tbody>
</table>
<div class="language-java line-numbers-mode" data-highlighter="prismjs" data-ext="java"><pre v-pre><code class="language-java"><span class="line"><span class="token comment">// Spring DevTools 配置</span></span>
<span class="line"><span class="token comment">// application.yml</span></span>
<span class="line">spring<span class="token operator">:</span></span>
<span class="line">  devtools<span class="token operator">:</span></span>
<span class="line">    restart<span class="token operator">:</span></span>
<span class="line">      enabled<span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line">      additional<span class="token operator">-</span>paths<span class="token operator">:</span> src<span class="token operator">/</span>main<span class="token operator">/</span>java</span>
<span class="line">    livereload<span class="token operator">:</span></span>
<span class="line">      enabled<span class="token operator">:</span> <span class="token boolean">true</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="c-net热更新" tabindex="-1"><a class="header-anchor" href="#c-net热更新"><span><strong>C#/.NET热更新</strong></span></a></h3>
<table>
<thead>
<tr>
<th>方案</th>
<th>类型</th>
<th>性能</th>
<th>易用性</th>
<th>生产适用</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>.NET Hot Reload</strong></td>
<td>运行时编译</td>
<td>高</td>
<td>高</td>
<td>是</td>
<td>.NET 6+原生支持</td>
</tr>
<tr>
<td><strong>dotnet watch</strong></td>
<td>进程重启</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>开发时自动重启</td>
</tr>
<tr>
<td><strong>Assembly热替换</strong></td>
<td>程序集替换</td>
<td>高</td>
<td>低</td>
<td>是</td>
<td>高级技术，需要架构支持</td>
</tr>
</tbody>
</table>
<div class="language-csharp line-numbers-mode" data-highlighter="prismjs" data-ext="cs"><pre v-pre><code class="language-csharp"><span class="line"><span class="token comment">// dotnet watch 使用</span></span>
<span class="line"><span class="token comment">// 命令行</span></span>
<span class="line">dotnet watch run <span class="token operator">--</span>project GameServer<span class="token punctuation">.</span>csproj</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="unity游戏引擎" tabindex="-1"><a class="header-anchor" href="#unity游戏引擎"><span><strong>Unity游戏引擎</strong></span></a></h3>
<table>
<thead>
<tr>
<th>方案</th>
<th>类型</th>
<th>性能</th>
<th>易用性</th>
<th>生产适用</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Unity域重载</strong></td>
<td>域重载</td>
<td>中</td>
<td>高</td>
<td>否</td>
<td>Unity Editor内置</td>
</tr>
<tr>
<td><strong>AssetBundle</strong></td>
<td>资源热更</td>
<td>高</td>
<td>中</td>
<td>是</td>
<td>资源和代码热更新</td>
</tr>
<tr>
<td><strong>Lua热更新</strong></td>
<td>脚本热更</td>
<td>高</td>
<td>中</td>
<td>是</td>
<td>XLua/ToLua方案</td>
</tr>
<tr>
<td><strong>C#反射热更</strong></td>
<td>反射调用</td>
<td>低</td>
<td>低</td>
<td>是</td>
<td>HybridCLR方案</td>
</tr>
</tbody>
</table>
<h3 id="unreal-engine" tabindex="-1"><a class="header-anchor" href="#unreal-engine"><span><strong>Unreal Engine</strong></span></a></h3>
<table>
<thead>
<tr>
<th>方案</th>
<th>类型</th>
<th>性能</th>
<th>易用性</th>
<th>生产适用</th>
<th>特点</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Live Coding</strong></td>
<td>增量编译</td>
<td>高</td>
<td>高</td>
<td>否</td>
<td>UE4/5内置开发功能</td>
</tr>
<tr>
<td><strong>Blueprint热编译</strong></td>
<td>可视化脚本</td>
<td>高</td>
<td>高</td>
<td>是</td>
<td>蓝图系统天然支持</td>
</tr>
<tr>
<td><strong>Plugin热加载</strong></td>
<td>插件系统</td>
<td>高</td>
<td>中</td>
<td>是</td>
<td>模块化热替换</td>
</tr>
</tbody>
</table>
<h2 id="🎯-游戏开发热更新需求分析" tabindex="-1"><a class="header-anchor" href="#🎯-游戏开发热更新需求分析"><span>🎯 游戏开发热更新需求分析</span></a></h2>
<h3 id="开发期热更新" tabindex="-1"><a class="header-anchor" href="#开发期热更新"><span><strong>开发期热更新</strong></span></a></h3>
<ul>
<li>✅ <strong>代码逻辑</strong> - 游戏规则、AI逻辑</li>
<li>✅ <strong>配置参数</strong> - 数值平衡、关卡配置</li>
<li>✅ <strong>脚本文件</strong> - Lua/Python脚本</li>
<li>✅ <strong>资源文件</strong> - 图片、音频、模型</li>
</ul>
<h3 id="生产环境热更新" tabindex="-1"><a class="header-anchor" href="#生产环境热更新"><span><strong>生产环境热更新</strong></span></a></h3>
<ul>
<li>✅ <strong>配置热更</strong> - 不停服调整参数</li>
<li>✅ <strong>脚本热更</strong> - 活动逻辑、运营脚本</li>
<li>✅ <strong>内容热更</strong> - 新关卡、新功能</li>
<li>⚠️ <strong>核心热更</strong> - 核心逻辑（高风险）</li>
</ul>
<h2 id="🏗️-croupier热更新架构设计" tabindex="-1"><a class="header-anchor" href="#🏗️-croupier热更新架构设计"><span>🏗️ Croupier热更新架构设计</span></a></h2>
<Mermaid code="eJyVVF1PE0EUfedXNPK6iG2gfMSQCCV8qA1JFR4aH7bdWbph6Ta7W0zfECwCKZaaUCMfSkFio1AQjEVL5c8ws9t/4exMd3a7FBM3aXZn7jn33Dv3TGdVPpXwPRvu8OFHS8fo+h68WoT5grmYhdUjmK3C70v3CMJ6JkKjUfxD+Xc3tZ8PY2r30HTEN6IIoHsS6MMqLyW1Fww88mQiOqIq6ZQEVGtB8LBQv6l9Nks5mt1Bz/B6PBFFxTc4s7FTgJvH8EOZUMxqGf66gPkzuk0pICl0eAqfATGjUjI2V4ytXGPvwKk6xGuJmMKrQtRYPkE7P1DxDMfR3jWtKH9qflmBuSIlOxWNCpKuqFG4WzZ+XxtXRfNPwS6pkd0w6pVu8/U22j2mIYc3LmmYl4kaa6s4Ct+uwPw5Vdr5iGrbbpk2XbAS0e4GXC85XYwregSoC0B1uqAQuyiq5+1impclgbcauakdGvuv0H6p8TXHzvb0BF1socNF9OnI4YRASlYyWKmxXDbq5zYYVtZgtozNAS+X76w/EnrcYpkxBe9ExxQffpEsU3J6VkriHuD6vlmvO6qTEQs5GWHIp4qQlgHp9hptHDjIqYyeUJIWmn4xhjSfUlRdlmL3VSArvOBKzi/wJD1+Mzg8eW+uL+FDaaPxPCnpGYtBPhjlkaZhp6eTggzuPAJ0eYlW82w4rrPg5wGdoT/qAfnpCO3JwsoBWq0a306dghxywEsO/Ac57CWH/0lm7eFr7+vqGqL3lOzgK+3ZIV9kj5mVktkN9ASda0YCzKskwFZtSGxFYrZfm2LNFQkR+93eJl67vc2M1YZBDXQ7YBuFihM9quwMm0SIZNsIU/VEA5RHhdvGbG1PMExricu8poWA6BPAgk+UZHmwEwTFPlHkNF1V5sBgp79/4IEotoJfglgTLAZFEQgM3BuI9/j5VrCG9aQ4sAmi2AeCjCDy/XF/0EMQ5mzwgIi1GbgvEAACPY+W7PiUWfIA6Hcl7+3xx12NWg7lsCc56kLcsyvGDMhRu3HNP2mrWxeM2Ypj5uPYsJu9uuBk2BwZLMeGyDUHxrHh4J7dJGf6nGueru9ws/GOv7yh5qI="></Mermaid><h2 id="🔧-热更新类型设计" tabindex="-1"><a class="header-anchor" href="#🔧-热更新类型设计"><span>🔧 热更新类型设计</span></a></h2>
<h3 id="_1-配置热更新-最安全" tabindex="-1"><a class="header-anchor" href="#_1-配置热更新-最安全"><span><strong>1. 配置热更新（最安全）</strong></span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># 游戏配置</span></span>
<span class="line"><span class="token key atrule">game_balance</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">player_hp</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line">  <span class="token key atrule">enemy_damage</span><span class="token punctuation">:</span> <span class="token number">25</span></span>
<span class="line">  <span class="token key atrule">level_rewards</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level_1</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line">    <span class="token key atrule">level_2</span><span class="token punctuation">:</span> <span class="token number">200</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-脚本热更新-常用" tabindex="-1"><a class="header-anchor" href="#_2-脚本热更新-常用"><span><strong>2. 脚本热更新（常用）</strong></span></a></h3>
<div class="language-lua line-numbers-mode" data-highlighter="prismjs" data-ext="lua"><pre v-pre><code class="language-lua"><span class="line"><span class="token comment">-- 游戏逻辑脚本</span></span>
<span class="line"><span class="token keyword">function</span> <span class="token function">calculate_damage</span><span class="token punctuation">(</span>base_damage<span class="token punctuation">,</span> level<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> base_damage <span class="token operator">*</span> <span class="token punctuation">(</span><span class="token number">1</span> <span class="token operator">+</span> level <span class="token operator">*</span> <span class="token number">0.1</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-函数热更新-高级" tabindex="-1"><a class="header-anchor" href="#_3-函数热更新-高级"><span><strong>3. 函数热更新（高级）</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// Go Plugin热更新</span></span>
<span class="line"><span class="token comment">//go:build plugin</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">CalculateReward</span><span class="token punctuation">(</span>level <span class="token builtin">int</span><span class="token punctuation">,</span> score <span class="token builtin">int</span><span class="token punctuation">)</span> <span class="token builtin">int</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> score <span class="token operator">*</span> level <span class="token operator">*</span> <span class="token number">10</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-资源热更新" tabindex="-1"><a class="header-anchor" href="#_4-资源热更新"><span><strong>4. 资源热更新</strong></span></a></h3>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.1"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"assets"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"textures/ui_button.png"</span><span class="token operator">:</span> <span class="token string">"hash123"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"sounds/click.wav"</span><span class="token operator">:</span> <span class="token string">"hash456"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🚀-sdk集成策略" tabindex="-1"><a class="header-anchor" href="#🚀-sdk集成策略"><span>🚀 SDK集成策略</span></a></h2>
<h3 id="通用热更新接口" tabindex="-1"><a class="header-anchor" href="#通用热更新接口"><span><strong>通用热更新接口</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// 统一的热更新接口</span></span>
<span class="line"><span class="token keyword">type</span> HotReloader <span class="token keyword">interface</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 注册热更新回调</span></span>
<span class="line">    <span class="token function">RegisterHandler</span><span class="token punctuation">(</span>pattern <span class="token builtin">string</span><span class="token punctuation">,</span> handler ReloadHandler<span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 启动热更新监听</span></span>
<span class="line">    <span class="token function">StartWatching</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 手动触发重载</span></span>
<span class="line">    <span class="token function">Reload</span><span class="token punctuation">(</span>path <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 停止监听</span></span>
<span class="line">    <span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token builtin">error</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="语言特定实现" tabindex="-1"><a class="header-anchor" href="#语言特定实现"><span><strong>语言特定实现</strong></span></a></h3>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token comment">// Go实现 - Plugin机制</span></span>
<span class="line"><span class="token keyword">type</span> GoHotReloader <span class="token keyword">struct</span> <span class="token punctuation">{</span></span>
<span class="line">    plugins <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token operator">*</span>plugin<span class="token punctuation">.</span>Plugin</span>
<span class="line">    watchers <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token operator">*</span>fsnotify<span class="token punctuation">.</span>Watcher</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token punctuation">(</span>r <span class="token operator">*</span>GoHotReloader<span class="token punctuation">)</span> <span class="token function">Reload</span><span class="token punctuation">(</span>path <span class="token builtin">string</span><span class="token punctuation">)</span> <span class="token builtin">error</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 重新加载Go插件</span></span>
<span class="line">    <span class="token keyword">return</span> r<span class="token punctuation">.</span><span class="token function">reloadPlugin</span><span class="token punctuation">(</span>path<span class="token punctuation">)</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-javascript line-numbers-mode" data-highlighter="prismjs" data-ext="js"><pre v-pre><code class="language-javascript"><span class="line"><span class="token comment">// JavaScript实现 - require缓存清除</span></span>
<span class="line"><span class="token keyword">class</span> <span class="token class-name">JSHotReloader</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">reload</span><span class="token punctuation">(</span><span class="token parameter">path</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">delete</span> require<span class="token punctuation">.</span>cache<span class="token punctuation">[</span>require<span class="token punctuation">.</span><span class="token function">resolve</span><span class="token punctuation">(</span>path<span class="token punctuation">)</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">require</span><span class="token punctuation">(</span>path<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-python line-numbers-mode" data-highlighter="prismjs" data-ext="py"><pre v-pre><code class="language-python"><span class="line"><span class="token comment"># Python实现 - importlib</span></span>
<span class="line"><span class="token keyword">import</span> importlib</span>
<span class="line"><span class="token keyword">import</span> sys</span>
<span class="line"></span>
<span class="line"><span class="token keyword">class</span> <span class="token class-name">PythonHotReloader</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token keyword">def</span> <span class="token function">reload</span><span class="token punctuation">(</span>self<span class="token punctuation">,</span> module_name<span class="token punctuation">)</span><span class="token punctuation">:</span></span>
<span class="line">        <span class="token keyword">if</span> module_name <span class="token keyword">in</span> sys<span class="token punctuation">.</span>modules<span class="token punctuation">:</span></span>
<span class="line">            <span class="token keyword">return</span> importlib<span class="token punctuation">.</span><span class="token builtin">reload</span><span class="token punctuation">(</span>sys<span class="token punctuation">.</span>modules<span class="token punctuation">[</span>module_name<span class="token punctuation">]</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🎮-游戏引擎特殊支持" tabindex="-1"><a class="header-anchor" href="#🎮-游戏引擎特殊支持"><span>🎮 游戏引擎特殊支持</span></a></h2>
<h3 id="unity集成" tabindex="-1"><a class="header-anchor" href="#unity集成"><span><strong>Unity集成</strong></span></a></h3>
<div class="language-csharp line-numbers-mode" data-highlighter="prismjs" data-ext="cs"><pre v-pre><code class="language-csharp"><span class="line"><span class="token comment">// Unity热更新管理器</span></span>
<span class="line"><span class="token keyword">public</span> <span class="token keyword">class</span> <span class="token class-name">CroupierHotReloader</span> <span class="token punctuation">:</span> <span class="token type-list"><span class="token class-name">MonoBehaviour</span></span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">public</span> <span class="token keyword">async</span> <span class="token return-type class-name"><span class="token keyword">void</span></span> <span class="token function">ReloadAssetBundle</span><span class="token punctuation">(</span><span class="token class-name"><span class="token keyword">string</span></span> bundleName<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">await</span> AssetBundle<span class="token punctuation">.</span><span class="token function">LoadFromFileAsync</span><span class="token punctuation">(</span>bundleName<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token comment">// 通知相关系统重新加载</span></span>
<span class="line">        GameEventSystem<span class="token punctuation">.</span><span class="token function">Trigger</span><span class="token punctuation">(</span><span class="token string">"AssetReloaded"</span><span class="token punctuation">,</span> bundleName<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="unreal集成" tabindex="-1"><a class="header-anchor" href="#unreal集成"><span><strong>Unreal集成</strong></span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Unreal热更新系统</span></span>
<span class="line"><span class="token keyword">class</span> <span class="token class-name">GAME_API</span> UCroupierHotReloader <span class="token operator">:</span> <span class="token keyword">public</span> USubsystem</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line"><span class="token keyword">public</span><span class="token operator">:</span></span>
<span class="line">    <span class="token function">UFUNCTION</span><span class="token punctuation">(</span>BlueprintCallable<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">ReloadBlueprint</span><span class="token punctuation">(</span><span class="token keyword">const</span> FString<span class="token operator">&amp;</span> BlueprintPath<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token function">UFUNCTION</span><span class="token punctuation">(</span>BlueprintCallable<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">ReloadGameplayScript</span><span class="token punctuation">(</span><span class="token keyword">const</span> FString<span class="token operator">&amp;</span> ScriptPath<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<p><em>接下来我将详细实现各语言的SDK和Web管理界面。这个热更新系统将大大提升游戏开发效率！</em></p>
</div></template>


