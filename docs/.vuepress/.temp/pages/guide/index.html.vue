<template><div><h2 id="快速开始" tabindex="-1"><a class="header-anchor" href="#快速开始"><span>快速开始</span></a></h2>
<div class="hint-container tip">
<p class="hint-container-title">环境要求</p>
<ul>
<li>Go 1.25+</li>
<li>Node.js 22+</li>
<li>Docker (可选)</li>
<li>CMake 3.20+ (C++ SDK)</li>
</ul>
</div>
<h3 id="_1-克隆仓库" tabindex="-1"><a class="header-anchor" href="#_1-克隆仓库"><span>1. 克隆仓库</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> clone https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-安装依赖并构建" tabindex="-1"><a class="header-anchor" href="#_2-安装依赖并构建"><span>2. 安装依赖并构建</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 安装依赖</span></span>
<span class="line">go mod download</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 生成协议代码</span></span>
<span class="line"><span class="token function">make</span> proto</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 构建所有组件</span></span>
<span class="line"><span class="token function">make</span> build</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-运行服务" tabindex="-1"><a class="header-anchor" href="#_3-运行服务"><span>3. 运行服务</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 启动 Server</span></span>
<span class="line">./bin/croupier-server <span class="token parameter variable">--config</span> configs/server.example.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启动 Agent</span></span>
<span class="line">./bin/croupier-agent <span class="token parameter variable">--config</span> configs/agent.example.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="项目结构" tabindex="-1"><a class="header-anchor" href="#项目结构"><span>项目结构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">croupier/</span>
<span class="line">├── cmd/                 # 程序入口</span>
<span class="line">├── proto/               # Protobuf 定义</span>
<span class="line">├── internal/            # 内部实现</span>
<span class="line">│   ├── server/          # 服务端核心逻辑</span>
<span class="line">│   ├── agent/           # 代理实现</span>
<span class="line">│   └── auth/            # 认证授权</span>
<span class="line">├── sdks/                # 多语言 SDK</span>
<span class="line">│   ├── go/</span>
<span class="line">│   ├── cpp/</span>
<span class="line">│   ├── java/</span>
<span class="line">│   ├── js/</span>
<span class="line">│   └── python/</span>
<span class="line">├── dashboard/           # Web 管理界面</span>
<span class="line">├── configs/             # 配置文件</span>
<span class="line">├── examples/            # 示例代码</span>
<span class="line">└── docs/                # 项目文档</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="系统架构" tabindex="-1"><a class="header-anchor" href="#系统架构"><span>系统架构</span></a></h2>
<Mermaid code="eJxLL0osyFAIceJSUCguTYLwlJ6uW/SsY/vz1euVgMIKCs45mal5JdHPdux41tEPl4sFyqXmpXCh6Hy+buHzCW3P+pY/7dj2dGMTRH+oZ3R4apICRO751J6XcxeBNCsoBKcWlaUWRTsX5ZcWZKYWQflYDX7a0fZ0R/PTPf1Pdi8GmgI329Ew2jEd6DoFQ4iRjkZQvhFWYyB+eDan92nXQrgZ7sGG0e6JualQ+xUcIUa5BxuhCDshmRjqqaCra1fjERISoBDkGhxSA1UElIKqBkmnBwU4K+SG+ATXAN2JW8oIKOVoCBIGOQXEMYJyQDKQ0EeSRREw4gIA4F2exw=="></Mermaid><h2 id="核心组件" tabindex="-1"><a class="header-anchor" href="#核心组件"><span>核心组件</span></a></h2>
<table>
<thead>
<tr>
<th>组件</th>
<th>说明</th>
<th>仓库</th>
</tr>
</thead>
<tbody>
<tr>
<td>Server</td>
<td>中央控制平面（HTTP + gRPC）</td>
<td><a href="https://github.com/cuihairu/croupier" target="_blank" rel="noopener noreferrer">cuihairu/croupier</a></td>
</tr>
<tr>
<td>Agent</td>
<td>分布式代理进程</td>
<td><a href="https://github.com/cuihairu/croupier" target="_blank" rel="noopener noreferrer">cuihairu/croupier</a></td>
</tr>
<tr>
<td>Edge</td>
<td>DMZ/边缘代理</td>
<td><a href="https://github.com/cuihairu/croupier" target="_blank" rel="noopener noreferrer">cuihairu/croupier</a></td>
</tr>
<tr>
<td>Dashboard</td>
<td>Web 管理界面</td>
<td><a href="https://github.com/cuihairu/croupier-dashboard" target="_blank" rel="noopener noreferrer">cuihairu/croupier-dashboard</a></td>
</tr>
</tbody>
</table>
<h2 id="相关链接" tabindex="-1"><a class="header-anchor" href="#相关链接"><span>相关链接</span></a></h2>
<ul>
<li><a href="https://github.com/cuihairu/croupier" target="_blank" rel="noopener noreferrer">GitHub 仓库</a></li>
<li><a href="https://github.com/cuihairu/croupier/issues" target="_blank" rel="noopener noreferrer">问题跟踪</a></li>
<li><a href="https://github.com/cuihairu/croupier/releases" target="_blank" rel="noopener noreferrer">更新日志</a></li>
</ul>
</div></template>


