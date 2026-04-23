<template><div><h2 id="项目结构" tabindex="-1"><a class="header-anchor" href="#项目结构"><span>项目结构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">croupier/</span>
<span class="line">├── cmd/                 # 程序入口</span>
<span class="line">├── proto/               # Protobuf 定义（子模块）</span>
<span class="line">├── internal/            # 内部实现</span>
<span class="line">│   ├── server/          # 服务端核心逻辑</span>
<span class="line">│   ├── agent/           # 代理实现</span>
<span class="line">│   ├── edge/            # 边缘代理实现</span>
<span class="line">│   ├── auth/            # 认证授权</span>
<span class="line">│   ├── function/        # 函数管理</span>
<span class="line">│   ├── jobs/            # 作业系统</span>
<span class="line">│   └── loadbalancer/    # 负载均衡</span>
<span class="line">├── sdks/                # 多语言 SDK（子模块）</span>
<span class="line">│   ├── go/</span>
<span class="line">│   ├── cpp/</span>
<span class="line">│   ├── java/</span>
<span class="line">│   ├── js/</span>
<span class="line">│   └── python/</span>
<span class="line">├── dashboard/           # Web 管理界面（子模块）</span>
<span class="line">├── configs/             # 配置文件示例</span>
<span class="line">├── examples/            # 示例代码</span>
<span class="line">├── packs/               # 函数包示例</span>
<span class="line">└── docs/                # 项目文档</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="快速开始" tabindex="-1"><a class="header-anchor" href="#快速开始"><span>快速开始</span></a></h2>
<h3 id="_1-克隆仓库" tabindex="-1"><a class="header-anchor" href="#_1-克隆仓库"><span>1. 克隆仓库</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> clone <span class="token parameter variable">--recursive</span> https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-安装依赖并构建" tabindex="-1"><a class="header-anchor" href="#_2-安装依赖并构建"><span>2. 安装依赖并构建</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 安装 Go 依赖</span></span>
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
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="系统架构" tabindex="-1"><a class="header-anchor" href="#系统架构"><span>系统架构</span></a></h2>
<Mermaid code="eJx9UkFLG0EUvudXDHpsZN1F0yhFWBNJpS0Vd6XQpYdNdmY7NNmE2aTQkkOpGNbSNnrooVApppQeKoZeasD4b5xdc/IvOLMzbHY1eEl47/ve9773zbrEbr0G5noOAL9TFdUcPR2EwVn0dzjH2gCU6hh6bSscjcKgn2CPqkRZw88NRfcc0sSO8gJWXzE+9JxcRi06PY4OeuHXPzT4T/99vB4HtLcXXRxej/eF/s6mxWaB4EXfPk+OBrG47rVBGfrY9cADYL5rQaNGcKvNlwBgQPIWEqtEmp0WhkTW8ZxYxVSU8Gh38v1ACX/+vhoOZporP3up0L0TZkd42XBcaPEfbrM/nHzYZzazqlcXJ7R/OFONBj062qXj/uX5L3aLuFbmlr1ZV6fWdZfFC1Rxl67dBrSZq4Rq+OML/XR8z56KoVoVuwFlPkBnURrlJ2JXxdAy4PoUlPt2NsHCwlr3sWluge0Nw+xKKoPkDIfd7a0SaJhPjS67KwuJDLtxrgzhf3dGNAboKm9zu7zQZMER8fWl0ExD4y5rddv3yxCBDgYI1+ur87CIlhHK+23SfANX59XiyiJCaaYvLAo2KiAEnYS9rNWWVDvNtuOHkGSEHsJCQkZ2saYW0mSXJyq5i2glxb0r7DTeT2WrGVnbUZeS4/hDdHBSyYDFEUlXV/Msudhr0mOR5VlKsamkGb8CW527AWUkfNU="></Mermaid><h2 id="核心文档" tabindex="-1"><a class="header-anchor" href="#核心文档"><span>核心文档</span></a></h2>
<h3 id="入门指南" tabindex="-1"><a class="header-anchor" href="#入门指南"><span>入门指南</span></a></h3>
<ul>
<li><a href="/guide/quick-start/" target="_blank" rel="noopener noreferrer">快速开始</a> - 快速搭建开发环境</li>
<li><a href="/guide/installation/" target="_blank" rel="noopener noreferrer">安装指南</a> - 详细的安装说明</li>
<li><a href="/guide/configuration/" target="_blank" rel="noopener noreferrer">配置管理</a> - 系统配置详解</li>
<li><a href="/guide/deployment/" target="_blank" rel="noopener noreferrer">部署指南</a> - 生产环境部署</li>
</ul>
<h3 id="核心概念" tabindex="-1"><a class="header-anchor" href="#核心概念"><span>核心概念</span></a></h3>
<ul>
<li><a href="/guide/concepts/overview/" target="_blank" rel="noopener noreferrer">系统概览</a> - 系统设计理念</li>
<li><a href="/guide/concepts/virtual-objects/" target="_blank" rel="noopener noreferrer">虚拟对象系统</a> - 四层对象模型</li>
<li><a href="/guide/concepts/function-management/" target="_blank" rel="noopener noreferrer">函数管理</a> - 函数注册与调用</li>
<li><a href="/guide/concepts/permissions/" target="_blank" rel="noopener noreferrer">权限控制</a> - RBAC/ABAC 权限模型</li>
</ul>
<h3 id="架构设计" tabindex="-1"><a class="header-anchor" href="#架构设计"><span>架构设计</span></a></h3>
<ul>
<li><a href="/architecture/" target="_blank" rel="noopener noreferrer">系统架构</a> - 整体架构设计</li>
<li><a href="/architecture/layers/" target="_blank" rel="noopener noreferrer">分层设计</a> - 三层架构详解</li>
<li><a href="/architecture/data-flow/" target="_blank" rel="noopener noreferrer">数据流</a> - 调用与数据流</li>
</ul>
<h3 id="api-参考" tabindex="-1"><a class="header-anchor" href="#api-参考"><span>API 参考</span></a></h3>
<ul>
<li><a href="/api/" target="_blank" rel="noopener noreferrer">API 概览</a> - API 总览</li>
<li><a href="/api/grpc/" target="_blank" rel="noopener noreferrer">gRPC API</a> - gRPC 服务定义</li>
<li><a href="/api/rest/" target="_blank" rel="noopener noreferrer">REST API</a> - HTTP REST 接口</li>
</ul>
<h3 id="sdk-文档" tabindex="-1"><a class="header-anchor" href="#sdk-文档"><span>SDK 文档</span></a></h3>
<ul>
<li><a href="https://github.com/cuihairu/croupier-sdk-cpp" target="_blank" rel="noopener noreferrer">C++ SDK</a> - C++ 客户端开发</li>
<li><a href="https://github.com/cuihairu/croupier-sdk-go" target="_blank" rel="noopener noreferrer">Go SDK</a> - Go 客户端开发</li>
<li><a href="https://github.com/cuihairu/croupier-sdk-java" target="_blank" rel="noopener noreferrer">Java SDK</a> - Java 客户端开发</li>
<li><a href="https://github.com/cuihairu/croupier-sdk-js" target="_blank" rel="noopener noreferrer">JavaScript SDK</a> - JS/TS 客户端开发</li>
<li><a href="https://github.com/cuihairu/croupier-sdk-python" target="_blank" rel="noopener noreferrer">Python SDK</a> - Python 客户端开发</li>
</ul>
<h3 id="分析系统" tabindex="-1"><a class="header-anchor" href="#分析系统"><span>分析系统</span></a></h3>
<ul>
<li><a href="/analytics/" target="_blank" rel="noopener noreferrer">分析系统概览</a> - 游戏分析系统</li>
<li><a href="/analytics/quick-start/" target="_blank" rel="noopener noreferrer">快速开始</a> - 分析系统入门</li>
</ul>
<h2 id="核心特性" tabindex="-1"><a class="header-anchor" href="#核心特性"><span>核心特性</span></a></h2>
<table>
<thead>
<tr>
<th>特性</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>零信任安全</strong></td>
<td>gRPC+mTLS、细粒度 RBAC/ABAC、操作审批与审计日志</td>
</tr>
<tr>
<td><strong>函数注册驱动</strong></td>
<td>游戏服务器通过 Agent 注册函数，控制面统一管理</td>
</tr>
<tr>
<td><strong>Schema 驱动 UI</strong></td>
<td>X-Render + JSON Schema 自动生成表单和界面</td>
</tr>
<tr>
<td><strong>可观测性解耦</strong></td>
<td>控制面与遥测面分离，支持实时事件处理</td>
</tr>
<tr>
<td><strong>多语言 SDK</strong></td>
<td>Go / C++ / Java / JS / Python 全覆盖</td>
</tr>
<tr>
<td><strong>协议优先</strong></td>
<td>所有 API 通过 Protocol Buffers 定义</td>
</tr>
</tbody>
</table>
<h2 id="相关仓库" tabindex="-1"><a class="header-anchor" href="#相关仓库"><span>相关仓库</span></a></h2>
<table>
<thead>
<tr>
<th>组件</th>
<th>仓库</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td>Server / Agent / Edge</td>
<td><a href="https://github.com/cuihairu/croupier" target="_blank" rel="noopener noreferrer">cuihairu/croupier</a></td>
<td>主仓库</td>
</tr>
<tr>
<td>Dashboard</td>
<td><a href="https://github.com/cuihairu/croupier-dashboard" target="_blank" rel="noopener noreferrer">cuihairu/croupier-dashboard</a></td>
<td>Web 管理界面</td>
</tr>
<tr>
<td>Proto 定义</td>
<td><a href="https://github.com/cuihairu/croupier-proto" target="_blank" rel="noopener noreferrer">cuihairu/croupier-proto</a></td>
<td>gRPC/HTTP IDL</td>
</tr>
<tr>
<td>Go SDK</td>
<td><a href="https://github.com/cuihairu/croupier-sdk-go" target="_blank" rel="noopener noreferrer">cuihairu/croupier-sdk-go</a></td>
<td>Go 客户端</td>
</tr>
<tr>
<td>C++ SDK</td>
<td><a href="https://github.com/cuihairu/croupier-sdk-cpp" target="_blank" rel="noopener noreferrer">cuihairu/croupier-sdk-cpp</a></td>
<td>C++ 客户端</td>
</tr>
<tr>
<td>Java SDK</td>
<td><a href="https://github.com/cuihairu/croupier-sdk-java" target="_blank" rel="noopener noreferrer">cuihairu/croupier-sdk-java</a></td>
<td>Java 客户端</td>
</tr>
<tr>
<td>JS/TS SDK</td>
<td><a href="https://github.com/cuihairu/croupier-sdk-js" target="_blank" rel="noopener noreferrer">cuihairu/croupier-sdk-js</a></td>
<td>JavaScript/TypeScript</td>
</tr>
<tr>
<td>Python SDK</td>
<td><a href="https://github.com/cuihairu/croupier-sdk-python" target="_blank" rel="noopener noreferrer">cuihairu/croupier-sdk-python</a></td>
<td>Python 客户端</td>
</tr>
</tbody>
</table>
<h2 id="许可证" tabindex="-1"><a class="header-anchor" href="#许可证"><span>许可证</span></a></h2>
<p>Apache License 2.0</p>
<h2 id="链接" tabindex="-1"><a class="header-anchor" href="#链接"><span>链接</span></a></h2>
<ul>
<li><a href="https://github.com/cuihairu/croupier" target="_blank" rel="noopener noreferrer">GitHub 仓库</a></li>
<li><a href="https://github.com/cuihairu/croupier/issues" target="_blank" rel="noopener noreferrer">问题跟踪</a></li>
<li><a href="https://github.com/cuihairu/croupier/releases" target="_blank" rel="noopener noreferrer">更新日志</a></li>
</ul>
</div></template>


