<template><div><h1 id="安装指南" tabindex="-1"><a class="header-anchor" href="#安装指南"><span>安装指南</span></a></h1>
<p>本指南详细介绍 Croupier 各组件的安装和配置方法。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#系统要求">系统要求</router-link><ul><li><router-link to="#最低要求">最低要求</router-link></li><li><router-link to="#硬件要求">硬件要求</router-link></li></ul></li><li><router-link to="#开发环境安装">开发环境安装</router-link><ul><li><router-link to="#_1-安装-go">1. 安装 Go</router-link></li><li><router-link to="#_2-安装-node-js-和-pnpm">2. 安装 Node.js 和 pnpm</router-link></li><li><router-link to="#_3-安装-buf">3. 安装 buf</router-link></li><li><router-link to="#_4-克隆项目">4. 克隆项目</router-link></li></ul></li><li><router-link to="#生产环境安装">生产环境安装</router-link><ul><li><router-link to="#docker-部署">Docker 部署</router-link></li><li><router-link to="#二进制部署">二进制部署</router-link></li></ul></li><li><router-link to="#sdk-安装">SDK 安装</router-link><ul><li><router-link to="#go-sdk">Go SDK</router-link></li><li><router-link to="#c-sdk">C++ SDK</router-link></li><li><router-link to="#java-sdk">Java SDK</router-link></li><li><router-link to="#javascript-typescript-sdk">JavaScript/TypeScript SDK</router-link></li><li><router-link to="#python-sdk">Python SDK</router-link></li></ul></li><li><router-link to="#配置文件">配置文件</router-link><ul><li><router-link to="#server-配置">Server 配置</router-link></li><li><router-link to="#agent-配置">Agent 配置</router-link></li></ul></li><li><router-link to="#验证安装">验证安装</router-link><ul><li><router-link to="#健康检查">健康检查</router-link></li><li><router-link to="#功能测试">功能测试</router-link></li></ul></li><li><router-link to="#故障排查">故障排查</router-link><ul><li><router-link to="#常见问题">常见问题</router-link></li></ul></li><li><router-link to="#下一步">下一步</router-link></li></ul></nav>
<h2 id="系统要求" tabindex="-1"><a class="header-anchor" href="#系统要求"><span>系统要求</span></a></h2>
<h3 id="最低要求" tabindex="-1"><a class="header-anchor" href="#最低要求"><span>最低要求</span></a></h3>
<table>
<thead>
<tr>
<th>组件</th>
<th>最低版本</th>
<th>推荐版本</th>
</tr>
</thead>
<tbody>
<tr>
<td>Go</td>
<td>1.25</td>
<td>1.25+</td>
</tr>
<tr>
<td>Node.js</td>
<td>22</td>
<td>22+</td>
</tr>
<tr>
<td>pnpm</td>
<td>10</td>
<td>10.22+</td>
</tr>
<tr>
<td>buf</td>
<td>-</td>
<td>latest</td>
</tr>
<tr>
<td>操作系统</td>
<td>Linux/macOS/Windows</td>
<td>Linux/macOS</td>
</tr>
</tbody>
</table>
<h3 id="硬件要求" tabindex="-1"><a class="header-anchor" href="#硬件要求"><span>硬件要求</span></a></h3>
<table>
<thead>
<tr>
<th>场景</th>
<th>CPU</th>
<th>内存</th>
<th>磁盘</th>
</tr>
</thead>
<tbody>
<tr>
<td>开发</td>
<td>2 核</td>
<td>4 GB</td>
<td>10 GB</td>
</tr>
<tr>
<td>生产（小规模）</td>
<td>4 核</td>
<td>8 GB</td>
<td>50 GB</td>
</tr>
<tr>
<td>生产（大规模）</td>
<td>8 核+</td>
<td>16 GB+</td>
<td>100 GB+</td>
</tr>
</tbody>
</table>
<h2 id="开发环境安装" tabindex="-1"><a class="header-anchor" href="#开发环境安装"><span>开发环境安装</span></a></h2>
<h3 id="_1-安装-go" tabindex="-1"><a class="header-anchor" href="#_1-安装-go"><span>1. 安装 Go</span></a></h3>
<h4 id="macos" tabindex="-1"><a class="header-anchor" href="#macos"><span>macOS</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 Homebrew</span></span>
<span class="line">brew <span class="token function">install</span> go</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证安装</span></span>
<span class="line">go version</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="linux-ubuntu-debian" tabindex="-1"><a class="header-anchor" href="#linux-ubuntu-debian"><span>Linux (Ubuntu/Debian)</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 下载并安装</span></span>
<span class="line"><span class="token function">wget</span> https://go.dev/dl/go1.25.0.linux-amd64.tar.gz</span>
<span class="line"><span class="token function">sudo</span> <span class="token function">tar</span> <span class="token parameter variable">-C</span> /usr/local <span class="token parameter variable">-xzf</span> go1.25.0.linux-amd64.tar.gz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 配置 PATH</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">'export PATH=$PATH:/usr/local/go/bin'</span> <span class="token operator">>></span> ~/.bashrc</span>
<span class="line"><span class="token builtin class-name">source</span> ~/.bashrc</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="windows" tabindex="-1"><a class="header-anchor" href="#windows"><span>Windows</span></a></h4>
<p>从 <a href="https://go.dev/dl/" target="_blank" rel="noopener noreferrer">Go 官网</a> 下载安装程序并运行。</p>
<h3 id="_2-安装-node-js-和-pnpm" tabindex="-1"><a class="header-anchor" href="#_2-安装-node-js-和-pnpm"><span>2. 安装 Node.js 和 pnpm</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 fnm 安装 Node.js（推荐）</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-fsSL</span> https://fnm.vercel.app/install <span class="token operator">|</span> <span class="token function">bash</span></span>
<span class="line">fnm <span class="token function">install</span> <span class="token number">22</span></span>
<span class="line">fnm use <span class="token number">22</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 安装 pnpm</span></span>
<span class="line"><span class="token function">npm</span> <span class="token function">install</span> <span class="token parameter variable">-g</span> <span class="token function">pnpm</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证安装</span></span>
<span class="line"><span class="token function">node</span> <span class="token parameter variable">-v</span></span>
<span class="line"><span class="token function">pnpm</span> <span class="token parameter variable">-v</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-安装-buf" tabindex="-1"><a class="header-anchor" href="#_3-安装-buf"><span>3. 安装 buf</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 go install</span></span>
<span class="line">go <span class="token function">install</span> github.com/bufbuild/buf/cmd/buf@latest</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或使用 Homebrew（macOS）</span></span>
<span class="line">brew <span class="token function">install</span> bufbuild/buf/buf</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证安装</span></span>
<span class="line">buf <span class="token parameter variable">--version</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-克隆项目" tabindex="-1"><a class="header-anchor" href="#_4-克隆项目"><span>4. 克隆项目</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 克隆主仓库</span></span>
<span class="line"><span class="token function">git</span> clone https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 初始化子模块</span></span>
<span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="生产环境安装" tabindex="-1"><a class="header-anchor" href="#生产环境安装"><span>生产环境安装</span></a></h2>
<h3 id="docker-部署" tabindex="-1"><a class="header-anchor" href="#docker-部署"><span>Docker 部署</span></a></h3>
<h4 id="使用-docker-compose" tabindex="-1"><a class="header-anchor" href="#使用-docker-compose"><span>使用 Docker Compose</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 构建镜像</span></span>
<span class="line"><span class="token function">docker-compose</span> build</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启动服务</span></span>
<span class="line"><span class="token function">docker-compose</span> up <span class="token parameter variable">-d</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 查看日志</span></span>
<span class="line"><span class="token function">docker-compose</span> logs <span class="token parameter variable">-f</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="单独部署" tabindex="-1"><a class="header-anchor" href="#单独部署"><span>单独部署</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 构建 Server 镜像</span></span>
<span class="line"><span class="token function">docker</span> build <span class="token parameter variable">-t</span> croupier-server:latest <span class="token parameter variable">-f</span> docker/Dockerfile.server <span class="token builtin class-name">.</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 运行 Server</span></span>
<span class="line"><span class="token function">docker</span> run <span class="token parameter variable">-d</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">--name</span> croupier-server <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-p</span> <span class="token number">8443</span>:8443 <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-p</span> <span class="token number">8080</span>:8080 <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-v</span> <span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">pwd</span><span class="token variable">)</span></span>/data:/app/data <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-v</span> <span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">pwd</span><span class="token variable">)</span></span>/configs:/app/configs <span class="token punctuation">\</span></span>
<span class="line">  croupier-server:latest</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="二进制部署" tabindex="-1"><a class="header-anchor" href="#二进制部署"><span>二进制部署</span></a></h3>
<h4 id="下载预构建版本" tabindex="-1"><a class="header-anchor" href="#下载预构建版本"><span>下载预构建版本</span></a></h4>
<p>从 <a href="https://github.com/cuihairu/croupier/releases" target="_blank" rel="noopener noreferrer">Release 页面</a> 下载对应平台的二进制文件。</p>
<h4 id="从源码构建" tabindex="-1"><a class="header-anchor" href="#从源码构建"><span>从源码构建</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 克隆仓库</span></span>
<span class="line"><span class="token function">git</span> clone https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 构建发布版本</span></span>
<span class="line"><span class="token function">make</span> build-release</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 二进制文件位于 bin/ 目录</span></span>
<span class="line"><span class="token function">ls</span> <span class="token parameter variable">-la</span> bin/</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="sdk-安装" tabindex="-1"><a class="header-anchor" href="#sdk-安装"><span>SDK 安装</span></a></h2>
<h3 id="go-sdk" tabindex="-1"><a class="header-anchor" href="#go-sdk"><span>Go SDK</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">go get github.com/cuihairu/croupier-sdk-go@latest</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="c-sdk" tabindex="-1"><a class="header-anchor" href="#c-sdk"><span>C++ SDK</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 vcpkg</span></span>
<span class="line"><span class="token function">vcpkg</span> <span class="token function">install</span> croupier-sdk-cpp</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或从源码构建</span></span>
<span class="line"><span class="token builtin class-name">cd</span> sdks/cpp</span>
<span class="line"><span class="token function">mkdir</span> build <span class="token operator">&amp;&amp;</span> <span class="token builtin class-name">cd</span> build</span>
<span class="line">cmake <span class="token punctuation">..</span> <span class="token parameter variable">-DCMAKE_BUILD_TYPE</span><span class="token operator">=</span>Release</span>
<span class="line"><span class="token function">make</span> <span class="token function">install</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="java-sdk" tabindex="-1"><a class="header-anchor" href="#java-sdk"><span>Java SDK</span></a></h3>
<div class="language-xml line-numbers-mode" data-highlighter="prismjs" data-ext="xml"><pre v-pre><code class="language-xml"><span class="line"><span class="token tag"><span class="token tag"><span class="token punctuation">&lt;</span>dependency</span><span class="token punctuation">></span></span></span>
<span class="line">    <span class="token tag"><span class="token tag"><span class="token punctuation">&lt;</span>groupId</span><span class="token punctuation">></span></span>com.github.cuihairu<span class="token tag"><span class="token tag"><span class="token punctuation">&lt;/</span>groupId</span><span class="token punctuation">></span></span></span>
<span class="line">    <span class="token tag"><span class="token tag"><span class="token punctuation">&lt;</span>artifactId</span><span class="token punctuation">></span></span>croupier-sdk-java<span class="token tag"><span class="token tag"><span class="token punctuation">&lt;/</span>artifactId</span><span class="token punctuation">></span></span></span>
<span class="line">    <span class="token tag"><span class="token tag"><span class="token punctuation">&lt;</span>version</span><span class="token punctuation">></span></span>LATEST<span class="token tag"><span class="token tag"><span class="token punctuation">&lt;/</span>version</span><span class="token punctuation">></span></span></span>
<span class="line"><span class="token tag"><span class="token tag"><span class="token punctuation">&lt;/</span>dependency</span><span class="token punctuation">></span></span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="javascript-typescript-sdk" tabindex="-1"><a class="header-anchor" href="#javascript-typescript-sdk"><span>JavaScript/TypeScript SDK</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># npm</span></span>
<span class="line"><span class="token function">npm</span> <span class="token function">install</span> @croupier/sdk-js</span>
<span class="line"></span>
<span class="line"><span class="token comment"># pnpm</span></span>
<span class="line"><span class="token function">pnpm</span> <span class="token function">add</span> @croupier/sdk-js</span>
<span class="line"></span>
<span class="line"><span class="token comment"># yarn</span></span>
<span class="line"><span class="token function">yarn</span> <span class="token function">add</span> @croupier/sdk-js</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="python-sdk" tabindex="-1"><a class="header-anchor" href="#python-sdk"><span>Python SDK</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">pip <span class="token function">install</span> croupier-sdk-python</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="配置文件" tabindex="-1"><a class="header-anchor" href="#配置文件"><span>配置文件</span></a></h2>
<h3 id="server-配置" tabindex="-1"><a class="header-anchor" href="#server-配置"><span>Server 配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":8443"</span>              <span class="token comment"># gRPC 监听地址</span></span>
<span class="line">  <span class="token key atrule">http_addr</span><span class="token punctuation">:</span> <span class="token string">":8080"</span>         <span class="token comment"># HTTP 监听地址</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/server.key"</span></span>
<span class="line">  <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">driver</span><span class="token punctuation">:</span> postgres</span>
<span class="line">    <span class="token key atrule">datasource</span><span class="token punctuation">:</span> <span class="token string">"postgres://user:pass@localhost:5432/croupier"</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> info</span>
<span class="line">    <span class="token key atrule">format</span><span class="token punctuation">:</span> json</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="agent-配置" tabindex="-1"><a class="header-anchor" href="#agent-配置"><span>Agent 配置</span></a></h3>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">agent</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">server_addr</span><span class="token punctuation">:</span> <span class="token string">"localhost:8443"</span></span>
<span class="line">  <span class="token key atrule">local_addr</span><span class="token punctuation">:</span> <span class="token string">":19090"</span></span>
<span class="line">  <span class="token key atrule">game_id</span><span class="token punctuation">:</span> <span class="token string">"my-game"</span></span>
<span class="line">  <span class="token key atrule">env</span><span class="token punctuation">:</span> <span class="token string">"dev"</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.key"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="验证安装" tabindex="-1"><a class="header-anchor" href="#验证安装"><span>验证安装</span></a></h2>
<h3 id="健康检查" tabindex="-1"><a class="header-anchor" href="#健康检查"><span>健康检查</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Server 健康检查</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/healthz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 预期输出</span></span>
<span class="line"><span class="token punctuation">{</span><span class="token string">"status"</span><span class="token builtin class-name">:</span><span class="token string">"ok"</span><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="功能测试" tabindex="-1"><a class="header-anchor" href="#功能测试"><span>功能测试</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 运行单元测试</span></span>
<span class="line"><span class="token function">make</span> <span class="token builtin class-name">test</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 运行集成测试</span></span>
<span class="line"><span class="token function">make</span> test-integration</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="故障排查" tabindex="-1"><a class="header-anchor" href="#故障排查"><span>故障排查</span></a></h2>
<h3 id="常见问题" tabindex="-1"><a class="header-anchor" href="#常见问题"><span>常见问题</span></a></h3>
<details>
<summary>Go 版本不兼容</summary>
<p>确保 Go 版本为 1.25 或更高：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">go version</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>如果不满足，请升级 Go 版本。</p>
</details>
<details>
<summary>子模块为空</summary>
<p>重新初始化子模块：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> submodule deinit <span class="token parameter variable">--all</span></span>
<span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div></details>
<details>
<summary>buf 命令未找到</summary>
<p>确保 <code v-pre>$GOPATH/bin</code> 在 <code v-pre>PATH</code> 中：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable"><span class="token environment constant">PATH</span></span><span class="token operator">=</span><span class="token environment constant">$PATH</span><span class="token builtin class-name">:</span><span class="token variable"><span class="token variable">$(</span>go <span class="token function">env</span> GOPATH<span class="token variable">)</span></span>/bin</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div></details>
<details>
<summary>端口已被占用</summary>
<p>修改配置文件中的端口：</p>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">addr</span><span class="token punctuation">:</span> <span class="token string">":9443"</span>      <span class="token comment"># 修改为其他端口</span></span>
<span class="line">  <span class="token key atrule">http_addr</span><span class="token punctuation">:</span> <span class="token string">":9080"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></details>
<h2 id="下一步" tabindex="-1"><a class="header-anchor" href="#下一步"><span>下一步</span></a></h2>
<p>安装完成后，请参考：</p>
<ul>
<li><RouteLink to="/guide/configuration.html">配置指南</RouteLink> - 详细配置选项</li>
<li><RouteLink to="/guide/deployment.html">部署指南</RouteLink> - 生产环境部署</li>
<li><RouteLink to="/guide/concepts/overview.html">核心概念</RouteLink> - 了解系统设计</li>
</ul>
</div></template>


