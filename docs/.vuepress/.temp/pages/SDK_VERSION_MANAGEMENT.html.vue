<template><div><h1 id="sdk-版本管理" tabindex="-1"><a class="header-anchor" href="#sdk-版本管理"><span>SDK 版本管理</span></a></h1>
<p>本文档说明如何统一管理所有 SDK 的版本号。</p>
<h2 id="当前版本" tabindex="-1"><a class="header-anchor" href="#当前版本"><span>当前版本</span></a></h2>
<p>项目使用 <strong>统一版本号</strong> 管理所有 SDK，当前版本：<strong>0.1.0</strong></p>
<h2 id="版本文件位置" tabindex="-1"><a class="header-anchor" href="#版本文件位置"><span>版本文件位置</span></a></h2>
<ul>
<li><strong>主版本文件</strong>: <code v-pre>/VERSION</code> - 单一来源的真相</li>
<li><strong>SDK 特定配置</strong>:
<ul>
<li>JS: <code v-pre>sdks/js/package.json</code> → <code v-pre>&quot;version&quot;: &quot;0.1.0&quot;</code></li>
<li>Python: <code v-pre>sdks/python/setup.py</code> → <code v-pre>version=&quot;0.1.0&quot;</code></li>
<li>Java: <code v-pre>sdks/java/build.gradle</code> → <code v-pre>version = '0.1.0'</code></li>
<li>C++: <code v-pre>sdks/cpp/CMakeLists.txt</code> → <code v-pre>VERSION 0.1.0</code></li>
<li>Go: <code v-pre>sdks/go/version.go</code> → <code v-pre>const Version = &quot;0.1.0&quot;</code></li>
</ul>
</li>
</ul>
<h2 id="查看当前版本" tabindex="-1"><a class="header-anchor" href="#查看当前版本"><span>查看当前版本</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">make</span> version</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>输出示例：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Current SDK Version: 0.1.0</span>
<span class="line"></span>
<span class="line">SDK Versions:</span>
<span class="line">  JS:     0.1.0</span>
<span class="line">  Python: 0.1.0</span>
<span class="line">  Java:   0.1.0</span>
<span class="line">  C++:    0.1.0</span>
<span class="line">  Go:     0.1.0</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="更新版本" tabindex="-1"><a class="header-anchor" href="#更新版本"><span>更新版本</span></a></h2>
<h3 id="方法-1-使用-make-命令-推荐" tabindex="-1"><a class="header-anchor" href="#方法-1-使用-make-命令-推荐"><span>方法 1: 使用 make 命令（推荐）</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 编辑 VERSION 文件，修改为新版本号，例如 0.2.0</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"0.2.0"</span> <span class="token operator">></span> VERSION</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 同步到所有 SDK</span></span>
<span class="line"><span class="token function">make</span> version-sync</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="方法-2-直接使用脚本" tabindex="-1"><a class="header-anchor" href="#方法-2-直接使用脚本"><span>方法 2: 直接使用脚本</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 同步当前 VERSION 文件到所有 SDK</span></span>
<span class="line">./scripts/sync-sdk-versions.sh</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或者直接指定新版本</span></span>
<span class="line">./scripts/sync-sdk-versions.sh <span class="token number">0.2</span>.0</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="版本同步流程" tabindex="-1"><a class="header-anchor" href="#版本同步流程"><span>版本同步流程</span></a></h2>
<p><code v-pre>make version-sync</code> 或 <code v-pre>sync-sdk-versions.sh</code> 会执行以下操作：</p>
<ol>
<li>✅ 验证版本号格式（必须符合 semver 规范）</li>
<li>✅ 更新 <code v-pre>VERSION</code> 文件</li>
<li>✅ 同步到所有 SDK 配置文件：
<ul>
<li><code v-pre>sdks/js/package.json</code></li>
<li><code v-pre>sdks/python/setup.py</code></li>
<li><code v-pre>sdks/java/build.gradle</code></li>
<li><code v-pre>sdks/cpp/CMakeLists.txt</code></li>
<li><code v-pre>sdks/go/version.go</code>（自动创建）</li>
</ul>
</li>
<li>✅ 更新 <code v-pre>sdks/js/pnpm-lock.yaml</code></li>
<li>✅ 提示提交变更</li>
</ol>
<h2 id="发布流程" tabindex="-1"><a class="header-anchor" href="#发布流程"><span>发布流程</span></a></h2>
<h3 id="_1-准备发布" tabindex="-1"><a class="header-anchor" href="#_1-准备发布"><span>1. 准备发布</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 更新版本号</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"0.2.0"</span> <span class="token operator">></span> VERSION</span>
<span class="line"><span class="token function">make</span> version-sync</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 验证版本</span></span>
<span class="line"><span class="token function">make</span> version</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 测试构建</span></span>
<span class="line"><span class="token function">make</span> build-sdks</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-提交变更" tabindex="-1"><a class="header-anchor" href="#_2-提交变更"><span>2. 提交变更</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> <span class="token function">add</span> VERSION <span class="token punctuation">\</span></span>
<span class="line">  sdks/js/package.json sdks/js/pnpm-lock.yaml <span class="token punctuation">\</span></span>
<span class="line">  sdks/python/setup.py <span class="token punctuation">\</span></span>
<span class="line">  sdks/java/build.gradle <span class="token punctuation">\</span></span>
<span class="line">  sdks/cpp/CMakeLists.txt <span class="token punctuation">\</span></span>
<span class="line">  sdks/go/version.go</span>
<span class="line"></span>
<span class="line"><span class="token function">git</span> commit <span class="token parameter variable">-m</span> <span class="token string">"chore: bump SDK versions to 0.2.0"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-创建-git-标签" tabindex="-1"><a class="header-anchor" href="#_3-创建-git-标签"><span>3. 创建 Git 标签</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> tag <span class="token parameter variable">-a</span> v0.2.0 <span class="token parameter variable">-m</span> <span class="token string">"Release version 0.2.0"</span></span>
<span class="line"><span class="token function">git</span> push origin main <span class="token parameter variable">--tags</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-触发-nightly-release" tabindex="-1"><a class="header-anchor" href="#_4-触发-nightly-release"><span>4. 触发 Nightly Release</span></a></h3>
<p>GitHub Actions 会自动：</p>
<ul>
<li>构建所有 SDK</li>
<li>生成正确的包名：
<ul>
<li><code v-pre>croupier-js-sdk-0.2.0.tgz</code></li>
<li><code v-pre>croupier-python-sdk-0.2.0.whl</code></li>
<li><code v-pre>croupier-java-sdk-0.2.0.jar</code></li>
<li><code v-pre>croupier-cpp-sdk-*-0.2.0.tar.gz</code></li>
<li><code v-pre>croupier-go-sdk.tar.gz</code></li>
</ul>
</li>
</ul>
<h2 id="包命名规范" tabindex="-1"><a class="header-anchor" href="#包命名规范"><span>包命名规范</span></a></h2>
<p>所有 SDK 包遵循统一命名规范：</p>
<ul>
<li><strong>JS</strong>: <code v-pre>croupier-js-sdk-{version}.tgz</code></li>
<li><strong>Python</strong>: <code v-pre>croupier-python-sdk-{version}.whl</code></li>
<li><strong>Java</strong>: <code v-pre>croupier-java-sdk-{version}.jar</code></li>
<li><strong>C++</strong>: <code v-pre>croupier-cpp-sdk-{os}-{arch}-static-{version}.tar.gz</code></li>
<li><strong>Go</strong>: <code v-pre>croupier-go-sdk.tar.gz</code> (源码包，无版本后缀)</li>
</ul>
<h2 id="版本号规范" tabindex="-1"><a class="header-anchor" href="#版本号规范"><span>版本号规范</span></a></h2>
<p>遵循 <a href="https://semver.org/" target="_blank" rel="noopener noreferrer">Semantic Versioning 2.0.0</a>：</p>
<ul>
<li><code v-pre>0.y.z</code> - 初始开发阶段（当前）</li>
<li><code v-pre>1.0.0</code> - 首个稳定版本</li>
<li><code v-pre>x.y.z-alpha.N</code> - Alpha 预发布版本</li>
<li><code v-pre>x.y.z-beta.N</code> - Beta 预发布版本</li>
<li><code v-pre>x.y.z-rc.N</code> - Release Candidate</li>
</ul>
<p>示例：</p>
<ul>
<li><code v-pre>0.1.0</code> - 初始版本</li>
<li><code v-pre>0.2.0</code> - 新增功能</li>
<li><code v-pre>0.2.1</code> - Bug 修复</li>
<li><code v-pre>1.0.0-beta.1</code> - 首个 beta 版本</li>
<li><code v-pre>1.0.0</code> - 正式版本</li>
</ul>
<h2 id="常见问题" tabindex="-1"><a class="header-anchor" href="#常见问题"><span>常见问题</span></a></h2>
<h3 id="q-为什么需要统一版本号" tabindex="-1"><a class="header-anchor" href="#q-为什么需要统一版本号"><span>Q: 为什么需要统一版本号？</span></a></h3>
<p>A: 统一版本号确保：</p>
<ul>
<li>所有 SDK 功能对齐</li>
<li>文档和发布说明一致</li>
<li>用户更容易理解兼容性</li>
</ul>
<h3 id="q-如果只更新一个-sdk-怎么办" tabindex="-1"><a class="header-anchor" href="#q-如果只更新一个-sdk-怎么办"><span>Q: 如果只更新一个 SDK 怎么办？</span></a></h3>
<p>A: 仍然建议统一升级版本号（至少更新补丁版本），并在 CHANGELOG 中注明具体变更的 SDK。</p>
<h3 id="q-如何回滚版本" tabindex="-1"><a class="header-anchor" href="#q-如何回滚版本"><span>Q: 如何回滚版本？</span></a></h3>
<p>A:</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 恢复到指定版本</span></span>
<span class="line">./scripts/sync-sdk-versions.sh <span class="token number">0.1</span>.0</span>
<span class="line"><span class="token function">git</span> <span class="token function">add</span> <span class="token parameter variable">-A</span></span>
<span class="line"><span class="token function">git</span> commit <span class="token parameter variable">-m</span> <span class="token string">"chore: revert SDK versions to 0.1.0"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文件" tabindex="-1"><a class="header-anchor" href="#相关文件"><span>相关文件</span></a></h2>
<ul>
<li><code v-pre>/VERSION</code> - 主版本文件</li>
<li><code v-pre>/scripts/sync-sdk-versions.sh</code> - 版本同步脚本</li>
<li><code v-pre>/Makefile</code> - 版本管理命令</li>
<li><code v-pre>/.github/workflows/sdk-nightly.yml</code> - CI/CD 配置</li>
</ul>
</div></template>


