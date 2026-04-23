<template><div><h1 id="快速开始" tabindex="-1"><a class="header-anchor" href="#快速开始"><span>快速开始</span></a></h1>
<p>本指南将帮助你快速搭建 Croupier 开发环境并运行第一个示例。</p>
<h2 id="环境要求" tabindex="-1"><a class="header-anchor" href="#环境要求"><span>环境要求</span></a></h2>
<p>在开始之前，请确保你的开发环境满足以下要求：</p>
<ul>
<li><strong>Go</strong>: 1.25 或更高版本</li>
<li><strong>Node.js</strong>: 22 或更高版本</li>
<li><strong>pnpm</strong>: 10.22.0 或更高版本</li>
<li><strong>Docker</strong>: 可选，用于容器化部署</li>
<li><strong>CMake</strong>: 3.20+（仅 C++ SDK 开发需要）</li>
<li><strong>buf</strong>: Protocol Buffers 工具链</li>
</ul>
<h3 id="检查环境" tabindex="-1"><a class="header-anchor" href="#检查环境"><span>检查环境</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查 Go 版本</span></span>
<span class="line">go version</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查 Node.js 版本</span></span>
<span class="line"><span class="token function">node</span> <span class="token parameter variable">-v</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查 pnpm 版本</span></span>
<span class="line"><span class="token function">pnpm</span> <span class="token parameter variable">-v</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查 buf 版本</span></span>
<span class="line">buf <span class="token parameter variable">--version</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="安装依赖" tabindex="-1"><a class="header-anchor" href="#安装依赖"><span>安装依赖</span></a></h2>
<h3 id="_1-克隆仓库" tabindex="-1"><a class="header-anchor" href="#_1-克隆仓库"><span>1. 克隆仓库</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> clone https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 初始化并更新子模块</span></span>
<span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-安装-go-依赖" tabindex="-1"><a class="header-anchor" href="#_2-安装-go-依赖"><span>2. 安装 Go 依赖</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">go mod download</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="_3-安装工具链" tabindex="-1"><a class="header-anchor" href="#_3-安装工具链"><span>3. 安装工具链</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 安装 buf（如果未安装）</span></span>
<span class="line">go <span class="token function">install</span> github.com/bufbuild/buf/cmd/buf@latest</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证安装</span></span>
<span class="line">buf <span class="token parameter variable">--version</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="构建项目" tabindex="-1"><a class="header-anchor" href="#构建项目"><span>构建项目</span></a></h2>
<h3 id="完整构建" tabindex="-1"><a class="header-anchor" href="#完整构建"><span>完整构建</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 清理构建</span></span>
<span class="line"><span class="token function">make</span> clean</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 完整构建（生成协议 + 编译）</span></span>
<span class="line"><span class="token function">make</span> dev</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="单独构建组件" tabindex="-1"><a class="header-anchor" href="#单独构建组件"><span>单独构建组件</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 仅生成协议代码</span></span>
<span class="line"><span class="token function">make</span> proto</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 构建所有二进制文件</span></span>
<span class="line"><span class="token function">make</span> build</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 构建特定组件</span></span>
<span class="line"><span class="token function">make</span> build-server</span>
<span class="line"><span class="token function">make</span> build-agent</span>
<span class="line"><span class="token function">make</span> build-edge</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>构建产物将输出到 <code v-pre>/bin</code> 目录：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">bin/</span>
<span class="line">├── croupier-server</span>
<span class="line">├── croupier-agent</span>
<span class="line">└── croupier-edge</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="运行服务" tabindex="-1"><a class="header-anchor" href="#运行服务"><span>运行服务</span></a></h2>
<h3 id="_1-生成-tls-证书-开发环境" tabindex="-1"><a class="header-anchor" href="#_1-生成-tls-证书-开发环境"><span>1. 生成 TLS 证书（开发环境）</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">./scripts/dev-certs.sh</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>这将生成自签名证书到 <code v-pre>data/dev-certs/</code> 目录。</p>
<h3 id="_2-配置服务" tabindex="-1"><a class="header-anchor" href="#_2-配置服务"><span>2. 配置服务</span></a></h3>
<p>复制示例配置并根据需要修改：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Server 配置</span></span>
<span class="line"><span class="token function">cp</span> configs/server.example.yaml configs/server.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Agent 配置</span></span>
<span class="line"><span class="token function">cp</span> configs/agent.example.yaml configs/agent.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-启动-server" tabindex="-1"><a class="header-anchor" href="#_3-启动-server"><span>3. 启动 Server</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">./bin/croupier-server <span class="token parameter variable">--config</span> configs/server.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>Server 默认监听：</p>
<ul>
<li><strong>gRPC</strong>: <code v-pre>8443</code> (mTLS)</li>
<li><strong>HTTP</strong>: <code v-pre>8080</code> (REST API)</li>
</ul>
<h3 id="_4-启动-agent" tabindex="-1"><a class="header-anchor" href="#_4-启动-agent"><span>4. 启动 Agent</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">./bin/croupier-agent <span class="token parameter variable">--config</span> configs/agent.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>Agent 将连接到 Server 并注册游戏服务器函数。</p>
<h3 id="_5-启动-dashboard-可选" tabindex="-1"><a class="header-anchor" href="#_5-启动-dashboard-可选"><span>5. 启动 Dashboard（可选）</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token builtin class-name">cd</span> dashboard</span>
<span class="line"><span class="token function">pnpm</span> <span class="token function">install</span></span>
<span class="line"><span class="token function">pnpm</span> dev</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>访问 http://localhost:8000 查看管理界面。</p>
<h2 id="验证安装" tabindex="-1"><a class="header-anchor" href="#验证安装"><span>验证安装</span></a></h2>
<h3 id="健康检查" tabindex="-1"><a class="header-anchor" href="#健康检查"><span>健康检查</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查 Server HTTP 端点</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/healthz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 预期输出</span></span>
<span class="line"><span class="token comment"># {"status":"ok"}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="查看日志" tabindex="-1"><a class="header-anchor" href="#查看日志"><span>查看日志</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Server 日志</span></span>
<span class="line"><span class="token function">tail</span> <span class="token parameter variable">-f</span> logs/server.log</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Agent 日志</span></span>
<span class="line"><span class="token function">tail</span> <span class="token parameter variable">-f</span> logs/agent.log</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="下一步" tabindex="-1"><a class="header-anchor" href="#下一步"><span>下一步</span></a></h2>
<p>安装完成后，你可以：</p>
<ul>
<li>阅读核心概念了解系统设计</li>
<li>查看配置指南了解详细配置选项</li>
<li>探索示例代码学习最佳实践</li>
<li>选择合适的 SDK 开始集成</li>
</ul>
<h2 id="常见问题" tabindex="-1"><a class="header-anchor" href="#常见问题"><span>常见问题</span></a></h2>
<h3 id="端口被占用" tabindex="-1"><a class="header-anchor" href="#端口被占用"><span>端口被占用</span></a></h3>
<p>如果默认端口被占用，可以通过配置文件修改：</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># server.yaml</span></span>
<span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":8443"</span>        <span class="token comment"># gRPC 端口</span></span>
<span class="line">  <span class="token key atrule">http_addr</span><span class="token punctuation">:</span> <span class="token string">":8080"</span>   <span class="token comment"># HTTP 端口</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="tls-证书错误" tabindex="-1"><a class="header-anchor" href="#tls-证书错误"><span>TLS 证书错误</span></a></h3>
<p>开发环境使用自签名证书，需要在客户端跳过证书验证：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">curl</span> <span class="token parameter variable">-k</span> https://localhost:8443/healthz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="子模块为空" tabindex="-1"><a class="header-anchor" href="#子模块为空"><span>子模块为空</span></a></h3>
<p>如果子模块目录为空，重新初始化：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> submodule deinit <span class="token parameter variable">--all</span></span>
<span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div></div></template>


