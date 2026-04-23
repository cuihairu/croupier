<template><div><h1 id="新手教程-10-分钟上手-croupier" tabindex="-1"><a class="header-anchor" href="#新手教程-10-分钟上手-croupier"><span>新手教程：10 分钟上手 Croupier</span></a></h1>
<p>本教程将带你从零开始，逐步搭建一个完整的 Croupier 环境，并创建你的第一个函数。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#准备工作">准备工作</router-link><ul><li><router-link to="#你将学到">你将学到</router-link></li><li><router-link to="#环境要求">环境要求</router-link></li></ul></li><li><router-link to="#第一步-获取代码">第一步：获取代码</router-link></li><li><router-link to="#第二步-构建项目">第二步：构建项目</router-link></li><li><router-link to="#第三步-生成证书">第三步：生成证书</router-link></li><li><router-link to="#第四步-启动-server">第四步：启动 Server</router-link></li><li><router-link to="#第五步-启动-agent">第五步：启动 Agent</router-link></li><li><router-link to="#第六步-创建测试函数">第六步：创建测试函数</router-link></li><li><router-link to="#第七步-调用函数">第七步：调用函数</router-link><ul><li><router-link to="#方式一-使用-rest-api">方式一：使用 REST API</router-link></li><li><router-link to="#方式二-使用-dashboard">方式二：使用 Dashboard</router-link></li></ul></li><li><router-link to="#第八步-配置权限">第八步：配置权限</router-link><ul><li><router-link to="#创建角色">创建角色</router-link></li><li><router-link to="#为函数添加审批">为函数添加审批</router-link></li></ul></li><li><router-link to="#第九步-查看审计日志">第九步：查看审计日志</router-link></li><li><router-link to="#架构图理解">架构图理解</router-link></li><li><router-link to="#下一步">下一步</router-link></li><li><router-link to="#常见问题">常见问题</router-link><ul><li><router-link to="#server-启动失败">Server 启动失败？</router-link></li><li><router-link to="#agent-连接失败">Agent 连接失败？</router-link></li><li><router-link to="#函数调用失败">函数调用失败？</router-link></li></ul></li><li><router-link to="#清理环境">清理环境</router-link></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="准备工作" tabindex="-1"><a class="header-anchor" href="#准备工作"><span>准备工作</span></a></h2>
<h3 id="你将学到" tabindex="-1"><a class="header-anchor" href="#你将学到"><span>你将学到</span></a></h3>
<p>通过本教程，你将学会：</p>
<ol>
<li>✅ 启动 Croupier Server 和 Agent</li>
<li>✅ 创建第一个函数</li>
<li>✅ 通过 Web 界面调用函数</li>
<li>✅ 配置权限和审批流程</li>
</ol>
<h3 id="环境要求" tabindex="-1"><a class="header-anchor" href="#环境要求"><span>环境要求</span></a></h3>
<p>确保已安装：</p>
<ul>
<li><strong>Go 1.25+</strong> - <a href="https://go.dev/dl/" target="_blank" rel="noopener noreferrer">下载地址</a></li>
<li><strong>Git</strong> - 用于克隆代码</li>
</ul>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查版本</span></span>
<span class="line">go version</span>
<span class="line"><span class="token function">git</span> <span class="token parameter variable">--version</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第一步-获取代码" tabindex="-1"><a class="header-anchor" href="#第一步-获取代码"><span>第一步：获取代码</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 克隆仓库（包含子模块）</span></span>
<span class="line"><span class="token function">git</span> clone <span class="token parameter variable">--recursive</span> https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 检查子模块</span></span>
<span class="line"><span class="token function">git</span> submodule status</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><blockquote>
<p><strong>提示：</strong> 如果子模块为空，执行：</p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> submodule update <span class="token parameter variable">--init</span> <span class="token parameter variable">--recursive</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div></blockquote>
<h2 id="第二步-构建项目" tabindex="-1"><a class="header-anchor" href="#第二步-构建项目"><span>第二步：构建项目</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 安装依赖</span></span>
<span class="line">go mod download</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 安装 buf 工具</span></span>
<span class="line">go <span class="token function">install</span> github.com/bufbuild/buf/cmd/buf@latest</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 生成协议代码</span></span>
<span class="line"><span class="token function">make</span> proto</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 编译二进制文件</span></span>
<span class="line"><span class="token function">make</span> build</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 5. 验证构建产物</span></span>
<span class="line"><span class="token function">ls</span> <span class="token parameter variable">-la</span> bin/</span>
<span class="line"><span class="token comment"># 应该看到：croupier-server、croupier-agent、croupier-edge</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第三步-生成证书" tabindex="-1"><a class="header-anchor" href="#第三步-生成证书"><span>第三步：生成证书</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 运行开发证书生成脚本</span></span>
<span class="line">./scripts/dev-certs.sh</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证证书生成</span></span>
<span class="line"><span class="token function">ls</span> <span class="token parameter variable">-la</span> data/dev-certs/</span>
<span class="line"><span class="token comment"># 应该看到：ca.crt、server.crt、server.key、agent.crt、agent.key</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第四步-启动-server" tabindex="-1"><a class="header-anchor" href="#第四步-启动-server"><span>第四步：启动 Server</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 复制配置文件</span></span>
<span class="line"><span class="token function">cp</span> configs/server.example.yaml configs/server.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 启动 Server（新终端窗口）</span></span>
<span class="line">./bin/croupier-server <span class="token parameter variable">--config</span> configs/server.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 验证 Server 运行</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/healthz</span>
<span class="line"><span class="token comment"># 预期输出：{"status":"ok"}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>Server 启动成功后，你会看到：</strong></p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">INFO[0000] Croupier Server starting...</span>
<span class="line">INFO[0000] gRPC server listening on :8443</span>
<span class="line">INFO[0000] HTTP server listening on :8080</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第五步-启动-agent" tabindex="-1"><a class="header-anchor" href="#第五步-启动-agent"><span>第五步：启动 Agent</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 新开一个终端窗口</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 复制配置文件</span></span>
<span class="line"><span class="token function">cp</span> configs/agent.example.yaml configs/agent.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 启动 Agent</span></span>
<span class="line">./bin/croupier-agent <span class="token parameter variable">--config</span> configs/agent.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Agent 启动成功后，你会看到：</span></span>
<span class="line"><span class="token comment"># INFO[0000] Croupier Agent starting...</span></span>
<span class="line"><span class="token comment"># INFO[0000] Connected to server at :8443</span></span>
<span class="line"><span class="token comment"># INFO[0000] Registered successfully</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第六步-创建测试函数" tabindex="-1"><a class="header-anchor" href="#第六步-创建测试函数"><span>第六步：创建测试函数</span></a></h2>
<p>创建一个简单的测试文件 <code v-pre>test_function.go</code>：</p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line"><span class="token keyword">package</span> main</span>
<span class="line"></span>
<span class="line"><span class="token keyword">import</span> <span class="token punctuation">(</span></span>
<span class="line">    <span class="token string">"context"</span></span>
<span class="line">    <span class="token string">"fmt"</span></span>
<span class="line">    <span class="token string">"log"</span></span>
<span class="line">    <span class="token string">"net"</span></span>
<span class="line"></span>
<span class="line">    <span class="token string">"google.golang.org/grpc"</span></span>
<span class="line">    <span class="token string">"google.golang.org/grpc/credentials/insecure"</span></span>
<span class="line">    <span class="token string">"google.golang.org/protobuf/encoding/protojson"</span></span>
<span class="line"></span>
<span class="line">    proto <span class="token string">"croupier/proto/proto"</span></span>
<span class="line"><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 测试函数：获取服务器时间</span></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">GetServerTime</span><span class="token punctuation">(</span>ctx context<span class="token punctuation">.</span>Context<span class="token punctuation">,</span> payload <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span> <span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token builtin">error</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">        <span class="token string">"current_time"</span><span class="token punctuation">:</span> fmt<span class="token punctuation">.</span><span class="token function">Sprintf</span><span class="token punctuation">(</span><span class="token string">"%v"</span><span class="token punctuation">,</span> ctx<span class="token punctuation">.</span><span class="token function">Value</span><span class="token punctuation">(</span><span class="token string">"timestamp"</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token string">"message"</span><span class="token punctuation">:</span>      <span class="token string">"Hello from Croupier!"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token boolean">nil</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 连接到本地 Agent</span></span>
<span class="line">    conn<span class="token punctuation">,</span> err <span class="token operator">:=</span> grpc<span class="token punctuation">.</span><span class="token function">Dial</span><span class="token punctuation">(</span><span class="token string">"localhost:19090"</span><span class="token punctuation">,</span></span>
<span class="line">        grpc<span class="token punctuation">.</span><span class="token function">WithTransportCredentials</span><span class="token punctuation">(</span>insecure<span class="token punctuation">.</span><span class="token function">NewCredentials</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Fatal</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">defer</span> conn<span class="token punctuation">.</span><span class="token function">Close</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    client <span class="token operator">:=</span> proto<span class="token punctuation">.</span><span class="token function">NewFunctionServiceClient</span><span class="token punctuation">(</span>conn<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册函数</span></span>
<span class="line">    descriptor <span class="token operator">:=</span> <span class="token operator">&amp;</span>proto<span class="token punctuation">.</span>FunctionDescriptor<span class="token punctuation">{</span></span>
<span class="line">        Id<span class="token punctuation">:</span>   <span class="token string">"test.get_time"</span><span class="token punctuation">,</span></span>
<span class="line">        Name<span class="token punctuation">:</span> <span class="token string">"获取服务器时间"</span><span class="token punctuation">,</span></span>
<span class="line">        ParamsSchema<span class="token punctuation">:</span> <span class="token function">toProtoStruct</span><span class="token punctuation">(</span><span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">            <span class="token string">"type"</span><span class="token punctuation">:</span> <span class="token string">"object"</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token string">"properties"</span><span class="token punctuation">:</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">                <span class="token string">"timezone"</span><span class="token punctuation">:</span> <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">{</span></span>
<span class="line">                    <span class="token string">"type"</span><span class="token punctuation">:</span>        <span class="token string">"string"</span><span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"title"</span><span class="token punctuation">:</span>       <span class="token string">"时区"</span><span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"default"</span><span class="token punctuation">:</span>     <span class="token string">"UTC"</span><span class="token punctuation">,</span></span>
<span class="line">                    <span class="token string">"description"</span><span class="token punctuation">:</span> <span class="token string">"例如：Asia/Shanghai"</span><span class="token punctuation">,</span></span>
<span class="line">                <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">            <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token punctuation">}</span><span class="token punctuation">)</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token boolean">_</span><span class="token punctuation">,</span> err <span class="token operator">=</span> client<span class="token punctuation">.</span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>context<span class="token punctuation">.</span><span class="token function">Background</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">,</span> <span class="token operator">&amp;</span>proto<span class="token punctuation">.</span>RegisterFunctionRequest<span class="token punctuation">{</span></span>
<span class="line">        GameId<span class="token punctuation">:</span>     <span class="token string">"test-game"</span><span class="token punctuation">,</span></span>
<span class="line">        Env<span class="token punctuation">:</span>        <span class="token string">"dev"</span><span class="token punctuation">,</span></span>
<span class="line">        Descriptor<span class="token punctuation">:</span> descriptor<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> err <span class="token operator">!=</span> <span class="token boolean">nil</span> <span class="token punctuation">{</span></span>
<span class="line">        log<span class="token punctuation">.</span><span class="token function">Fatal</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    log<span class="token punctuation">.</span><span class="token function">Println</span><span class="token punctuation">(</span><span class="token string">"函数注册成功: test.get_time"</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 保持运行</span></span>
<span class="line">    <span class="token keyword">select</span> <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">func</span> <span class="token function">toProtoStruct</span><span class="token punctuation">(</span>m <span class="token keyword">map</span><span class="token punctuation">[</span><span class="token builtin">string</span><span class="token punctuation">]</span><span class="token keyword">interface</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">)</span> <span class="token operator">*</span>proto<span class="token punctuation">.</span>Struct <span class="token punctuation">{</span></span>
<span class="line">    b<span class="token punctuation">,</span> <span class="token boolean">_</span> <span class="token operator">:=</span> protojson<span class="token punctuation">.</span><span class="token function">Marshal</span><span class="token punctuation">(</span>m<span class="token punctuation">)</span></span>
<span class="line">    s <span class="token operator">:=</span> <span class="token operator">&amp;</span>proto<span class="token punctuation">.</span>Struct<span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">    s<span class="token punctuation">.</span><span class="token function">UnmarshalJSON</span><span class="token punctuation">(</span>b<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> s</span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第七步-调用函数" tabindex="-1"><a class="header-anchor" href="#第七步-调用函数"><span>第七步：调用函数</span></a></h2>
<h3 id="方式一-使用-rest-api" tabindex="-1"><a class="header-anchor" href="#方式一-使用-rest-api"><span>方式一：使用 REST API</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 创建测试用户（首次使用）</span></span>
<span class="line"><span class="token comment"># 通过 Server 默认创建的管理员账号</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 获取 Token</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/auth/login <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{"username":"admin","password":"admin123"}'</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 调用函数</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/invoke <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Game-ID: test-game"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"X-Env: dev"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{</span>
<span class="line">    "function_id": "test.get_time",</span>
<span class="line">    "payload": {</span>
<span class="line">      "timezone": "Asia/Shanghai"</span>
<span class="line">    }</span>
<span class="line">  }'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="方式二-使用-dashboard" tabindex="-1"><a class="header-anchor" href="#方式二-使用-dashboard"><span>方式二：使用 Dashboard</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 启动 Dashboard（可选）</span></span>
<span class="line"><span class="token builtin class-name">cd</span> dashboard</span>
<span class="line"><span class="token function">pnpm</span> <span class="token function">install</span></span>
<span class="line"><span class="token function">pnpm</span> dev</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 访问 http://localhost:8000</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 登录后，在函数列表中找到 "test.get_time"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 点击调用，填写参数，提交</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第八步-配置权限" tabindex="-1"><a class="header-anchor" href="#第八步-配置权限"><span>第八步：配置权限</span></a></h2>
<h3 id="创建角色" tabindex="-1"><a class="header-anchor" href="#创建角色"><span>创建角色</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/roles <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Authorization: Bearer <span class="token variable">$TOKEN</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{</span>
<span class="line">    "role_id": "tester",</span>
<span class="line">    "name": "测试人员",</span>
<span class="line">    "permissions": ["test.*"]</span>
<span class="line">  }'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="为函数添加审批" tabindex="-1"><a class="header-anchor" href="#为函数添加审批"><span>为函数添加审批</span></a></h3>
<p>修改函数描述，添加双人审批：</p>
<div class="language-go line-numbers-mode" data-highlighter="prismjs" data-ext="go"><pre v-pre><code class="language-go"><span class="line">descriptor <span class="token operator">:=</span> <span class="token operator">&amp;</span>proto<span class="token punctuation">.</span>FunctionDescriptor<span class="token punctuation">{</span></span>
<span class="line">    Id<span class="token punctuation">:</span>   <span class="token string">"test.get_time"</span><span class="token punctuation">,</span></span>
<span class="line">    Name<span class="token punctuation">:</span> <span class="token string">"获取服务器时间"</span><span class="token punctuation">,</span></span>
<span class="line">    Auth<span class="token punctuation">:</span> <span class="token operator">&amp;</span>proto<span class="token punctuation">.</span>AuthConfig<span class="token punctuation">{</span></span>
<span class="line">        Permission<span class="token punctuation">:</span>      <span class="token string">"test.get_time"</span><span class="token punctuation">,</span></span>
<span class="line">        TwoPersonRule<span class="token punctuation">:</span>   <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">        ApprovalEnabled<span class="token punctuation">:</span> <span class="token boolean">true</span><span class="token punctuation">,</span></span>
<span class="line">        ApprovalThreshold<span class="token punctuation">:</span> <span class="token number">2</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// ... 其他字段</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="第九步-查看审计日志" tabindex="-1"><a class="header-anchor" href="#第九步-查看审计日志"><span>第九步：查看审计日志</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 查询最近的操作日志</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> POST http://localhost:8080/api/audit/query <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Content-Type: application/json"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-H</span> <span class="token string">"Authorization: Bearer <span class="token variable">$TOKEN</span>"</span> <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-d</span> <span class="token string">'{</span>
<span class="line">    "game_id": "test-game",</span>
<span class="line">    "env": "dev",</span>
<span class="line">    "start_time": "2024-01-01T00:00:00Z",</span>
<span class="line">    "page": 1,</span>
<span class="line">    "size": 10</span>
<span class="line">  }'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="架构图理解" tabindex="-1"><a class="header-anchor" href="#架构图理解"><span>架构图理解</span></a></h2>
<p>你刚刚部署的系统架构如下：</p>
<Mermaid code="eJxLL0osyFAIceJSAILi0iQIX+nJ3gXPZ7U8m9z7ZO8cJbAcCIR6RoenJgEpBX0FxwBPhRcbmp9PWRELlk/NS+FCM8S5KL+0IDO1SCE4tagstQhhDoQfDaEUrCwMLAz0rSxMTIwhRoGAY2lJRvSzuc0vZ054tmDhy1U9yFIpmSXRT9ctfLFu4bPpS5/un07YBY7pqXklCAeAudFgUsHK0NLA0gCnEZCQeLJ78fMFjQgD3ErzkqPdU0sgXgjJzE1VeNq+99nUDWjmAINKV9cO6mGwCNTTIFGQH8FiIAZUBOg1DGUgZ0LUgR0MEgPZzwUA5EGP+w=="></Mermaid><h2 id="下一步" tabindex="-1"><a class="header-anchor" href="#下一步"><span>下一步</span></a></h2>
<p>恭喜你完成了第一个 Croupier 函数！接下来可以：</p>
<ol>
<li>📖 阅读核心概念，了解架构设计</li>
<li>🔧 查看配置指南，学习更多配置选项</li>
<li>🚀 阅读部署指南，准备生产环境部署</li>
<li>💻 查看示例代码，学习更多函数模式</li>
</ol>
<h2 id="常见问题" tabindex="-1"><a class="header-anchor" href="#常见问题"><span>常见问题</span></a></h2>
<h3 id="server-启动失败" tabindex="-1"><a class="header-anchor" href="#server-启动失败"><span>Server 启动失败？</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查端口占用</span></span>
<span class="line"><span class="token function">lsof</span> <span class="token parameter variable">-i</span> :8443</span>
<span class="line"><span class="token function">lsof</span> <span class="token parameter variable">-i</span> :8080</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 如果被占用，修改配置文件中的端口</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="agent-连接失败" tabindex="-1"><a class="header-anchor" href="#agent-连接失败"><span>Agent 连接失败？</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 确认 Server 正在运行</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/healthz</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 检查配置文件中的 server_addr</span></span>
<span class="line"><span class="token comment"># agent.yaml</span></span>
<span class="line">agent:</span>
<span class="line">  server_addr: <span class="token string">"localhost:8443"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="函数调用失败" tabindex="-1"><a class="header-anchor" href="#函数调用失败"><span>函数调用失败？</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 查看日志</span></span>
<span class="line"><span class="token function">tail</span> <span class="token parameter variable">-f</span> logs/server.log</span>
<span class="line"><span class="token function">tail</span> <span class="token parameter variable">-f</span> logs/agent.log</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查函数是否注册成功</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/api/functions?game_id<span class="token operator">=</span>test-game<span class="token operator">&amp;</span><span class="token assign-left variable">env</span><span class="token operator">=</span>dev</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="清理环境" tabindex="-1"><a class="header-anchor" href="#清理环境"><span>清理环境</span></a></h2>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 停止所有进程</span></span>
<span class="line"><span class="token function">pkill</span> croupier-server</span>
<span class="line"><span class="token function">pkill</span> croupier-agent</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 删除测试数据</span></span>
<span class="line"><span class="token function">rm</span> <span class="token parameter variable">-rf</span> data/</span>
<span class="line"><span class="token function">rm</span> configs/server.yaml</span>
<span class="line"><span class="token function">rm</span> configs/agent.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/guide/quick-start.html">快速开始</RouteLink></li>
<li><RouteLink to="/guide/concepts/overview.html">核心概念</RouteLink></li>
<li><RouteLink to="/guide/configuration.html">配置管理</RouteLink></li>
<li><RouteLink to="/guide/faq.html">常见问题</RouteLink></li>
</ul>
</div></template>


