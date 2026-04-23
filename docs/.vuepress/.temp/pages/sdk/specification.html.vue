<template><div><h1 id="croupier-sdk-行为规范" tabindex="-1"><a class="header-anchor" href="#croupier-sdk-行为规范"><span>Croupier SDK 行为规范</span></a></h1>
<p>本文档定义了所有 Croupier SDK 必须遵守的行为规范，确保跨语言 SDK 的一致性和可预测性。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<ul>
<li><a href="#%E6%A0%B8%E5%BF%83%E6%8E%A5%E5%8F%A3%E8%A7%84%E8%8C%83">核心接口规范</a></li>
<li><a href="#%E9%85%8D%E7%BD%AE%E8%A7%84%E8%8C%83">配置规范</a></li>
<li><a href="#%E9%94%99%E8%AF%AF%E5%A4%84%E7%90%86%E8%A7%84%E8%8C%83">错误处理规范</a></li>
<li><a href="#%E7%94%9F%E5%91%BD%E5%91%A8%E6%9C%9F%E7%AE%A1%E7%90%86%E8%A7%84%E8%8C%83">生命周期管理规范</a></li>
<li><a href="#%E7%BD%91%E7%BB%9C%E8%A1%8C%E4%B8%BA%E8%A7%84%E8%8C%83">网络行为规范</a></li>
<li><a href="#%E5%AE%89%E5%85%A8%E8%A7%84%E8%8C%83">安全规范</a></li>
<li><a href="#%E6%97%A5%E5%BF%97%E8%A7%84%E8%8C%83">日志规范</a></li>
<li><a href="#%E6%B5%8B%E8%AF%95%E8%A7%84%E8%8C%83">测试规范</a></li>
</ul>
<hr>
<h2 id="核心接口规范" tabindex="-1"><a class="header-anchor" href="#核心接口规范"><span>核心接口规范</span></a></h2>
<h3 id="_1-client-接口" tabindex="-1"><a class="header-anchor" href="#_1-client-接口"><span>1. Client 接口</span></a></h3>
<p>所有 SDK <strong>必须</strong>实现 <code v-pre>Client</code> 接口，用于向 Agent 注册函数并接收调用。</p>
<h4 id="必需方法" tabindex="-1"><a class="header-anchor" href="#必需方法"><span>必需方法</span></a></h4>
<table>
<thead>
<tr>
<th>方法名</th>
<th>签名</th>
<th>行为规范</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>registerFunction</code></td>
<td><code v-pre>(descriptor, handler) -&gt; void</code></td>
<td>在连接前注册函数，连接后禁止注册</td>
</tr>
<tr>
<td><code v-pre>connect</code></td>
<td><code v-pre>() -&gt; Promise/void</code></td>
<td>建立与 Agent 的连接，启动本地 gRPC 服务器</td>
</tr>
<tr>
<td><code v-pre>serve</code></td>
<td><code v-pre>() -&gt; Promise/void</code></td>
<td>阻塞当前线程/协程，直到调用 <code v-pre>stop()</code></td>
</tr>
<tr>
<td><code v-pre>stop</code></td>
<td><code v-pre>() -&gt; Promise/void</code></td>
<td>优雅停止服务，清理资源</td>
</tr>
<tr>
<td><code v-pre>close</code></td>
<td><code v-pre>() -&gt; Promise/void</code></td>
<td>完全关闭客户端，释放所有资源</td>
</tr>
<tr>
<td><code v-pre>getLocalAddress</code></td>
<td><code v-pre>() -&gt; string</code></td>
<td>返回本地 gRPC 服务器地址</td>
</tr>
</tbody>
</table>
<h4 id="行为约束" tabindex="-1"><a class="header-anchor" href="#行为约束"><span>行为约束</span></a></h4>
<ol>
<li>
<p><strong>函数注册时机</strong></p>
<ul>
<li>函数只能在 <code v-pre>connect()</code> <strong>之前</strong>注册</li>
<li><code v-pre>connect()</code> 后调用 <code v-pre>registerFunction()</code> <strong>必须</strong>抛出错误</li>
<li>空函数列表（0个函数）调用 <code v-pre>connect()</code> <strong>必须</strong>抛出错误</li>
</ul>
</li>
<li>
<p><strong>连接状态管理</strong></p>
<ul>
<li><code v-pre>isConnected()</code> 必须准确反映连接状态</li>
<li>重复调用 <code v-pre>connect()</code> <strong>应该</strong>幂等（不应报错，应直接返回）</li>
</ul>
</li>
<li>
<p><strong>线程/并发安全</strong></p>
<ul>
<li><code v-pre>registerFunction()</code> 必须线程安全</li>
<li><code v-pre>connect()</code>/<code v-pre>stop()</code> 必须与函数调用并发安全</li>
</ul>
</li>
</ol>
<h3 id="_2-invoker-接口" tabindex="-1"><a class="header-anchor" href="#_2-invoker-接口"><span>2. Invoker 接口</span></a></h3>
<p>所有 SDK <strong>必须</strong>实现 <code v-pre>Invoker</code> 接口，用于调用远程注册的函数。</p>
<h4 id="必需方法-1" tabindex="-1"><a class="header-anchor" href="#必需方法-1"><span>必需方法</span></a></h4>
<table>
<thead>
<tr>
<th>方法名</th>
<th>签名</th>
<th>行为规范</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>connect</code></td>
<td><code v-pre>() -&gt; Promise/void</code></td>
<td>建立与服务器的连接</td>
</tr>
<tr>
<td><code v-pre>invoke</code></td>
<td><code v-pre>(functionId, payload, options) -&gt; string</code></td>
<td>同步调用函数，返回结果</td>
</tr>
<tr>
<td><code v-pre>startJob</code></td>
<td><code v-pre>(functionId, payload, options) -&gt; jobId</code></td>
<td>启动异步任务，返回任务ID</td>
</tr>
<tr>
<td><code v-pre>streamJob</code></td>
<td><code v-pre>(jobId) -&gt; Stream&lt;JobEvent&gt;</code></td>
<td>流式获取任务事件</td>
</tr>
<tr>
<td><code v-pre>cancelJob</code></td>
<td><code v-pre>(jobId) -&gt; void</code></td>
<td>取消正在运行的任务</td>
</tr>
<tr>
<td><code v-pre>close</code></td>
<td><code v-pre>() -&gt; Promise/void</code></td>
<td>关闭连接，释放资源</td>
</tr>
</tbody>
</table>
<h4 id="行为约束-1" tabindex="-1"><a class="header-anchor" href="#行为约束-1"><span>行为约束</span></a></h4>
<ol>
<li>
<p><strong>自动连接</strong></p>
<ul>
<li><code v-pre>invoke()</code> 和 <code v-pre>startJob()</code> 在未连接时<strong>应该</strong>自动连接</li>
</ul>
</li>
<li>
<p><strong>幂等性</strong></p>
<ul>
<li>使用相同 <code v-pre>idempotencyKey</code> 的请求<strong>必须</strong>返回相同结果</li>
<li>幂等性键的有效期由服务器决定</li>
</ul>
</li>
<li>
<p><strong>流式事件</strong></p>
<ul>
<li><code v-pre>streamJob()</code> 必须在任务完成时结束流</li>
<li>必须支持事件类型：<code v-pre>started</code>, <code v-pre>progress</code>, <code v-pre>completed</code>, <code v-pre>error</code>, <code v-pre>cancelled</code></li>
</ul>
</li>
</ol>
<hr>
<h2 id="配置规范" tabindex="-1"><a class="header-anchor" href="#配置规范"><span>配置规范</span></a></h2>
<h3 id="clientconfig-标准字段" tabindex="-1"><a class="header-anchor" href="#clientconfig-标准字段"><span>ClientConfig 标准字段</span></a></h3>
<p>所有 SDK 的 <code v-pre>ClientConfig</code> <strong>必须</strong>支持以下字段：</p>
<table>
<thead>
<tr>
<th>字段名</th>
<th>类型</th>
<th>必需</th>
<th>默认值</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>agentAddr</code></td>
<td>string</td>
<td>否</td>
<td><code v-pre>&quot;127.0.0.1:19090&quot;</code></td>
<td>Agent gRPC 地址</td>
</tr>
<tr>
<td><code v-pre>controlAddr</code></td>
<td>string</td>
<td>否</td>
<td><code v-pre>&quot;&quot;</code></td>
<td>控制面地址（可选）</td>
</tr>
<tr>
<td><code v-pre>serviceId</code></td>
<td>string</td>
<td>否</td>
<td>自动生成</td>
<td>唯一服务标识符</td>
</tr>
<tr>
<td><code v-pre>serviceVersion</code></td>
<td>string</td>
<td>否</td>
<td><code v-pre>&quot;1.0.0&quot;</code></td>
<td>服务版本</td>
</tr>
<tr>
<td><code v-pre>gameId</code></td>
<td>string</td>
<td>否</td>
<td><code v-pre>&quot;&quot;</code></td>
<td>游戏标识符（多租户）</td>
</tr>
<tr>
<td><code v-pre>env</code></td>
<td>string</td>
<td>否</td>
<td><code v-pre>&quot;development&quot;</code></td>
<td>环境标识</td>
</tr>
<tr>
<td><code v-pre>localListen</code></td>
<td>string</td>
<td>否</td>
<td><code v-pre>&quot;127.0.0.1:0&quot;</code></td>
<td>本地监听地址</td>
</tr>
<tr>
<td><code v-pre>timeout</code></td>
<td>number</td>
<td>否</td>
<td><code v-pre>30000</code></td>
<td>连接超时（毫秒）</td>
</tr>
<tr>
<td><code v-pre>insecure</code></td>
<td>boolean</td>
<td>否</td>
<td><code v-pre>true</code></td>
<td>是否使用不安全连接（开发用）</td>
</tr>
<tr>
<td><code v-pre>heartbeatIntervalSeconds</code></td>
<td>number</td>
<td>否</td>
<td><code v-pre>60</code></td>
<td>心跳间隔（秒）</td>
</tr>
</tbody>
</table>
<h3 id="tls-配置标准字段" tabindex="-1"><a class="header-anchor" href="#tls-配置标准字段"><span>TLS 配置标准字段</span></a></h3>
<table>
<thead>
<tr>
<th>字段名</th>
<th>类型</th>
<th>必需</th>
<th>默认值</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>caFile</code></td>
<td>string</td>
<td>否</td>
<td>系统证书</td>
<td>CA 证书文件路径</td>
</tr>
<tr>
<td><code v-pre>certFile</code></td>
<td>string</td>
<td>否</td>
<td>无</td>
<td>客户端证书文件路径</td>
</tr>
<tr>
<td><code v-pre>keyFile</code></td>
<td>string</td>
<td>否</td>
<td>无</td>
<td>客户端私钥文件路径</td>
</tr>
<tr>
<td><code v-pre>serverName</code></td>
<td>string</td>
<td>否</td>
<td>从地址提取</td>
<td>TLS 服务器名称验证</td>
</tr>
<tr>
<td><code v-pre>insecureSkipVerify</code></td>
<td>boolean</td>
<td>否</td>
<td><code v-pre>false</code></td>
<td>跳过证书验证（不推荐）</td>
</tr>
</tbody>
</table>
<h3 id="文件传输配置标准字段" tabindex="-1"><a class="header-anchor" href="#文件传输配置标准字段"><span>文件传输配置标准字段</span></a></h3>
<table>
<thead>
<tr>
<th>字段名</th>
<th>类型</th>
<th>必需</th>
<th>默认值</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>enableFileTransfer</code></td>
<td>boolean</td>
<td>否</td>
<td><code v-pre>false</code></td>
<td><strong>是否启用文件传输功能（默认关闭）</strong></td>
</tr>
<tr>
<td><code v-pre>maxFileSize</code></td>
<td>number</td>
<td>否</td>
<td><code v-pre>10485760</code> (10MB)</td>
<td>单个文件最大大小（字节）</td>
</tr>
<tr>
<td><code v-pre>allowedExtensions</code></td>
<td>string[]</td>
<td>否</td>
<td><code v-pre>[]</code></td>
<td>允许的文件扩展名（白名单）</td>
</tr>
<tr>
<td><code v-pre>allowedMimeTypes</code></td>
<td>string[]</td>
<td>否</td>
<td><code v-pre>[]</code></td>
<td>允许的 MIME 类型（白名单）</td>
</tr>
<tr>
<td><code v-pre>uploadTimeout</code></td>
<td>number</td>
<td>否</td>
<td><code v-pre>300000</code> (5分钟)</td>
<td>文件上传超时（毫秒）</td>
</tr>
</tbody>
</table>
<p><strong>重要安全要求：</strong></p>
<ul>
<li>文件传输<strong>必须默认关闭</strong></li>
<li>启用时<strong>必须</strong>配置文件大小限制</li>
<li><strong>必须</strong>使用扩展名或 MIME 类型白名单（拒绝所有其他类型）</li>
<li>上传的文件<strong>必须</strong>在服务端进行二次验证</li>
</ul>
<hr>
<h2 id="错误处理规范" tabindex="-1"><a class="header-anchor" href="#错误处理规范"><span>错误处理规范</span></a></h2>
<h3 id="错误类型标准" tabindex="-1"><a class="header-anchor" href="#错误类型标准"><span>错误类型标准</span></a></h3>
<p>所有 SDK <strong>必须</strong>区分以下错误类型：</p>
<table>
<thead>
<tr>
<th>错误类型</th>
<th>HTTP/gRPC 码</th>
<th>触发条件</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>InvalidArgument</code></td>
<td>400/InvalidArgument</td>
<td>参数无效或缺失</td>
</tr>
<tr>
<td><code v-pre>NotFound</code></td>
<td>404/NotFound</td>
<td>函数或任务不存在</td>
</tr>
<tr>
<td><code v-pre>AlreadyExists</code></td>
<td>409/AlreadyExists</td>
<td>重复注册</td>
</tr>
<tr>
<td><code v-pre>Unauthenticated</code></td>
<td>401/Unauthenticated</td>
<td>认证失败</td>
</tr>
<tr>
<td><code v-pre>PermissionDenied</code></td>
<td>403/PermissionDenied</td>
<td>权限不足</td>
</tr>
<tr>
<td><code v-pre>Internal</code></td>
<td>500/Internal</td>
<td>内部错误</td>
</tr>
<tr>
<td><code v-pre>Unavailable</code></td>
<td>503/Unavailable</td>
<td>服务不可用</td>
</tr>
</tbody>
</table>
<h3 id="错误处理行为" tabindex="-1"><a class="header-anchor" href="#错误处理行为"><span>错误处理行为</span></a></h3>
<ol>
<li>
<p><strong>错误传播</strong></p>
<ul>
<li>Handler 内部错误<strong>必须</strong>包装为 <code v-pre>Internal</code> 错误</li>
<li>网络错误<strong>必须</strong>传播给调用者</li>
</ul>
</li>
<li>
<p><strong>错误信息</strong></p>
<ul>
<li>错误消息<strong>必须</strong>包含足够的调试信息</li>
<li>不应泄露敏感信息（密码、密钥等）</li>
</ul>
</li>
</ol>
<hr>
<h2 id="生命周期管理规范" tabindex="-1"><a class="header-anchor" href="#生命周期管理规范"><span>生命周期管理规范</span></a></h2>
<h3 id="启动流程" tabindex="-1"><a class="header-anchor" href="#启动流程"><span>启动流程</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">1. 创建 Client 实例</span>
<span class="line">2. 调用 registerFunction() 注册所有函数</span>
<span class="line">3. 调用 connect() 建立连接</span>
<span class="line">4. 调用 serve() 进入服务循环</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="关闭流程" tabindex="-1"><a class="header-anchor" href="#关闭流程"><span>关闭流程</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">1. 调用 stop() 停止接受新请求</span>
<span class="line">2. 等待进行中的请求完成（超时时间可配置）</span>
<span class="line">3. 关闭本地 gRPC 服务器</span>
<span class="line">4. 停止心跳</span>
<span class="line">5. 关闭与 Agent 的连接</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="状态转换" tabindex="-1"><a class="header-anchor" href="#状态转换"><span>状态转换</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">      ┌─────────┐</span>
<span class="line">      │  New   │</span>
<span class="line">      └────┬────┘</span>
<span class="line">           │ registerFunction()</span>
<span class="line">           ▼</span>
<span class="line">      ┌─────────┐</span>
<span class="line">      │Registered│</span>
<span class="line">      └────┬────┘</span>
<span class="line">           │ connect()</span>
<span class="line">           ▼</span>
<span class="line">      ┌─────────┐  stop()</span>
<span class="line">   ┌──▶│Connected│◀─────┐</span>
<span class="line">   │   └─────────┘      │</span>
<span class="line">   │                    │</span>
<span class="line">   │                    │</span>
<span class="line">   │   ┌─────────┐      │</span>
<span class="line">   └───│  Closed │◀─────┘</span>
<span class="line">       └─────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr>
<h2 id="网络行为规范" tabindex="-1"><a class="header-anchor" href="#网络行为规范"><span>网络行为规范</span></a></h2>
<h3 id="心跳机制" tabindex="-1"><a class="header-anchor" href="#心跳机制"><span>心跳机制</span></a></h3>
<ol>
<li>
<p><strong>默认行为</strong></p>
<ul>
<li>心跳间隔默认 60 秒</li>
<li>心跳失败<strong>应该</strong>记录警告但不中断服务</li>
<li>心跳失败超过阈值（如 3 次）<strong>应该</strong>触发重连</li>
</ul>
</li>
<li>
<p><strong>心跳负载</strong></p>
<ul>
<li>必须包含 <code v-pre>serviceId</code> 和 <code v-pre>sessionId</code></li>
<li>必须使用与注册相同的标识符</li>
</ul>
</li>
</ol>
<h3 id="重连机制" tabindex="-1"><a class="header-anchor" href="#重连机制"><span>重连机制</span></a></h3>
<ol>
<li>
<p><strong>自动重连</strong></p>
<ul>
<li>网络错误后<strong>应该</strong>自动重连</li>
<li>重连间隔<strong>应该</strong>使用指数退避策略</li>
<li>最大重试次数<strong>应该</strong>可配置（默认无限制）</li>
</ul>
</li>
<li>
<p><strong>重连状态</strong></p>
<ul>
<li>重连期间，<code v-pre>isConnected()</code> 返回 <code v-pre>false</code></li>
<li>重连成功后，<strong>必须</strong>重新注册函数</li>
</ul>
</li>
</ol>
<h3 id="重试机制-指数退避" tabindex="-1"><a class="header-anchor" href="#重试机制-指数退避"><span>重试机制（指数退避）</span></a></h3>
<p><strong>所有 SDK 应该实现可配置的重试机制</strong></p>
<ol>
<li>
<p><strong>重试配置</strong></p>
<ul>
<li><code v-pre>enabled</code>: 是否启用重试（默认 true）</li>
<li><code v-pre>maxAttempts</code>: 最大重试次数（默认 3）</li>
<li><code v-pre>initialDelayMs</code>: 初始重试延迟（默认 100ms）</li>
<li><code v-pre>maxDelayMs</code>: 最大重试延迟（默认 5000ms）</li>
<li><code v-pre>backoffMultiplier</code>: 指数退避倍数（默认 2.0）</li>
<li><code v-pre>jitterFactor</code>: 抖动因子，避免雷群效应（默认 0.1）</li>
</ul>
</li>
<li>
<p><strong>重试行为</strong></p>
<ul>
<li>仅对可重试的错误码进行重试（如网络错误、超时）</li>
<li>使用指数退避计算延迟：<code v-pre>delay = min(initialDelay * multiplier^attempt, maxDelay)</code></li>
<li>添加随机抖动：<code v-pre>finalDelay = delay * (1 ± jitterFactor)</code></li>
<li>达到最大重试次数后停止并返回最后错误</li>
</ul>
</li>
<li>
<p><strong>可重试错误示例</strong></p>
<ul>
<li>gRPC: <code v-pre>UNAVAILABLE</code> (14), <code v-pre>INTERNAL</code> (13), <code v-pre>UNKNOWN</code> (2)</li>
<li>HTTP: 503 Service Unavailable, 502 Bad Gateway</li>
</ul>
</li>
</ol>
<h3 id="超时处理" tabindex="-1"><a class="header-anchor" href="#超时处理"><span>超时处理</span></a></h3>
<table>
<thead>
<tr>
<th>操作</th>
<th>默认超时</th>
<th>行为</th>
</tr>
</thead>
<tbody>
<tr>
<td>连接</td>
<td>30 秒</td>
<td>抛出超时错误</td>
</tr>
<tr>
<td>调用</td>
<td>30 秒</td>
<td>抛出超时错误</td>
</tr>
<tr>
<td>心跳</td>
<td>10 秒</td>
<td>记录警告</td>
</tr>
<tr>
<td>关闭</td>
<td>5 秒</td>
<td>强制关闭</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="安全规范" tabindex="-1"><a class="header-anchor" href="#安全规范"><span>安全规范</span></a></h2>
<h3 id="tls-mtls" tabindex="-1"><a class="header-anchor" href="#tls-mtls"><span>TLS/mTLS</span></a></h3>
<ol>
<li>
<p><strong>生产环境</strong></p>
<ul>
<li>生产环境<strong>必须</strong>使用 TLS</li>
<li><strong>禁止</strong>使用 <code v-pre>insecure = true</code></li>
</ul>
</li>
<li>
<p><strong>证书验证</strong></p>
<ul>
<li>默认<strong>必须</strong>验证服务器证书</li>
<li>支持自定义 CA 证书</li>
</ul>
</li>
<li>
<p><strong>mTLS</strong></p>
<ul>
<li>支持双向认证</li>
<li>客户端证书和私钥<strong>必须</strong>安全存储</li>
</ul>
</li>
</ol>
<h3 id="敏感信息处理" tabindex="-1"><a class="header-anchor" href="#敏感信息处理"><span>敏感信息处理</span></a></h3>
<ol>
<li>
<p><strong>日志脱敏</strong></p>
<ul>
<li>日志中<strong>禁止</strong>输出密码、密钥</li>
<li>Token <strong>必须</strong>部分遮蔽（如 <code v-pre>abc...xyz</code>，显示前 3 和后 3 个字符）</li>
<li>API Key <strong>必须</strong>部分遮蔽</li>
<li>敏感头信息（如 <code v-pre>Authorization</code>）<strong>必须</strong>脱敏</li>
</ul>
</li>
<li>
<p><strong>脱敏格式要求</strong></p>
<ul>
<li>推荐格式：<code v-pre>前3位...后3位</code>，如 <code v-pre>eyJ0...iOiJ</code></li>
<li>如果值长度 ≤ 6 位，全部替换为 <code v-pre>*</code>：<code v-pre>******</code></li>
<li>日志输出示例：<code v-pre>Using auth token: eyJ0...iOiJ</code></li>
</ul>
</li>
<li>
<p><strong>SDK 实现要求</strong></p>
<ul>
<li><strong>必须</strong>提供 <code v-pre>MaskSensitive(value)</code> 工具函数</li>
<li><strong>必须</strong>在所有日志输出中自动应用脱敏</li>
<li><strong>应该</strong>提供 <code v-pre>MaskJsonSensitive()</code> 用于 JSON 负载脱敏</li>
</ul>
</li>
<li>
<p><strong>错误信息</strong></p>
<ul>
<li>错误响应<strong>禁止</strong>包含内部路径或堆栈</li>
<li><strong>禁止</strong>在错误消息中泄露敏感配置</li>
</ul>
</li>
</ol>
<h3 id="文件传输安全" tabindex="-1"><a class="header-anchor" href="#文件传输安全"><span>文件传输安全</span></a></h3>
<ol>
<li>
<p><strong>默认关闭原则</strong></p>
<ul>
<li>文件传输功能<strong>必须默认关闭</strong>（<code v-pre>enableFileTransfer = false</code>）</li>
<li><strong>必须</strong>通过显式配置才能启用</li>
<li>SDK 启动时如果检测到文件传输被启用，<strong>应该</strong>记录警告日志</li>
</ul>
</li>
<li>
<p><strong>权限验证</strong></p>
<ul>
<li><strong>必须</strong>在调用任何文件传输方法前检查 <code v-pre>enableFileTransfer</code> 标志</li>
<li>如果标志为 <code v-pre>false</code>，<strong>必须</strong>拒绝操作并返回 <code v-pre>PermissionDenied</code> 错误</li>
<li>服务端<strong>必须</strong>二次验证客户端的文件传输权限</li>
</ul>
</li>
<li>
<p><strong>文件类型白名单</strong></p>
<ul>
<li><strong>必须</strong>使用扩展名白名单（<code v-pre>.png</code>, <code v-pre>.jpg</code>, <code v-pre>.pdf</code> 等）</li>
<li><strong>必须</strong>使用 MIME 类型白名单（<code v-pre>image/png</code>, <code v-pre>application/pdf</code> 等）</li>
<li><strong>必须</strong>拒绝不在白名单中的文件类型</li>
<li>白名单为空时<strong>必须</strong>拒绝所有文件上传</li>
</ul>
</li>
<li>
<p><strong>文件大小限制</strong></p>
<ul>
<li><strong>必须</strong>限制单个文件大小（默认 10MB）</li>
<li><strong>必须</strong>在传输前验证文件大小</li>
<li>超过限制<strong>必须</strong>拒绝上传并返回错误</li>
</ul>
</li>
<li>
<p><strong>安全扫描要求</strong></p>
<ul>
<li><strong>应该</strong>在上传后进行病毒扫描</li>
<li><strong>应该</strong>验证文件内容类型与扩展名匹配</li>
<li><strong>禁止</strong>执行上传的文件</li>
</ul>
</li>
</ol>
<hr>
<h2 id="日志规范" tabindex="-1"><a class="header-anchor" href="#日志规范"><span>日志规范</span></a></h2>
<h3 id="日志级别" tabindex="-1"><a class="header-anchor" href="#日志级别"><span>日志级别</span></a></h3>
<table>
<thead>
<tr>
<th>级别</th>
<th>用途</th>
<th>示例</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>DEBUG</code></td>
<td>详细调试信息</td>
<td>函数调用详情</td>
</tr>
<tr>
<td><code v-pre>INFO</code></td>
<td>正常操作信息</td>
<td>函数注册成功</td>
</tr>
<tr>
<td><code v-pre>WARN</code></td>
<td>警告信息</td>
<td>心跳失败</td>
</tr>
<tr>
<td><code v-pre>ERROR</code></td>
<td>错误信息</td>
<td>连接失败</td>
</tr>
</tbody>
</table>
<h3 id="日志格式" tabindex="-1"><a class="header-anchor" href="#日志格式"><span>日志格式</span></a></h3>
<p>推荐格式：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">[timestamp] [level] [component] message</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><p>示例：</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">2024-01-06T10:30:00Z [INFO] [croupier] Registered function: player.ban</span>
<span class="line">2024-01-06T10:30:05Z [ERROR] [croupier] Connection failed: dial tcp 127.0.0.1:19090: connection refused</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="可配置性" tabindex="-1"><a class="header-anchor" href="#可配置性"><span>可配置性</span></a></h3>
<ul>
<li><strong>必须</strong>支持禁用日志（<code v-pre>disableLogging</code>）</li>
<li><strong>必须</strong>支持调试模式（<code v-pre>debugLogging</code>）</li>
<li><strong>应该</strong>支持自定义日志输出</li>
</ul>
<hr>
<h2 id="测试规范" tabindex="-1"><a class="header-anchor" href="#测试规范"><span>测试规范</span></a></h2>
<h3 id="必需测试" tabindex="-1"><a class="header-anchor" href="#必需测试"><span>必需测试</span></a></h3>
<p>每个 SDK <strong>必须</strong>包含以下测试：</p>
<ol>
<li>
<p><strong>单元测试</strong></p>
<ul>
<li>函数注册测试</li>
<li>连接管理测试</li>
<li>错误处理测试</li>
</ul>
</li>
<li>
<p><strong>集成测试</strong></p>
<ul>
<li>与真实 Agent 的连接测试</li>
<li>函数调用端到端测试</li>
<li>异步任务测试</li>
</ul>
</li>
</ol>
<h3 id="测试覆盖率" tabindex="-1"><a class="header-anchor" href="#测试覆盖率"><span>测试覆盖率</span></a></h3>
<ul>
<li>核心路径覆盖率 <strong>应≥ 80%</strong></li>
<li>错误处理路径覆盖率 <strong>应≥ 60%</strong></li>
</ul>
<hr>
<h2 id="兼容性规范" tabindex="-1"><a class="header-anchor" href="#兼容性规范"><span>兼容性规范</span></a></h2>
<h3 id="版本兼容性" tabindex="-1"><a class="header-anchor" href="#版本兼容性"><span>版本兼容性</span></a></h3>
<ol>
<li>
<p><strong>SDK 版本</strong></p>
<ul>
<li>遵循语义化版本（SemVer）</li>
<li>主版本变更可能包含破坏性更新</li>
</ul>
</li>
<li>
<p><strong>协议兼容性</strong></p>
<ul>
<li>SDK <strong>必须</strong>与同版本或更高版本的 Agent 兼容</li>
<li>跨小版本（如 v1.x）<strong>应该</strong>兼容</li>
</ul>
</li>
</ol>
<h3 id="平台支持" tabindex="-1"><a class="header-anchor" href="#平台支持"><span>平台支持</span></a></h3>
<table>
<thead>
<tr>
<th>SDK</th>
<th>支持平台</th>
<th>最低版本</th>
</tr>
</thead>
<tbody>
<tr>
<td>Go</td>
<td>Linux, macOS, Windows</td>
<td>Go 1.21</td>
</tr>
<tr>
<td>JS</td>
<td>Node.js</td>
<td>Node.js 20</td>
</tr>
<tr>
<td>Python</td>
<td>Linux, macOS, Windows</td>
<td>Python 3.10</td>
</tr>
<tr>
<td>Java</td>
<td>Linux, macOS, Windows</td>
<td>Java 17</td>
</tr>
<tr>
<td>C++</td>
<td>Linux, macOS, Windows</td>
<td>C++17</td>
</tr>
</tbody>
</table>
<hr>
<h2 id="示例代码模板" tabindex="-1"><a class="header-anchor" href="#示例代码模板"><span>示例代码模板</span></a></h2>
<h3 id="基础使用-所有-sdk-应遵循" tabindex="-1"><a class="header-anchor" href="#基础使用-所有-sdk-应遵循"><span>基础使用（所有 SDK 应遵循）</span></a></h3>
<div class="language-pseudo line-numbers-mode" data-highlighter="prismjs" data-ext="pseudo"><pre v-pre><code class="language-pseudo"><span class="line">// 1. 创建配置</span>
<span class="line">config = ClientConfig{</span>
<span class="line">    agentAddr = &quot;127.0.0.1:19090&quot;,</span>
<span class="line">    serviceId = &quot;my-service&quot;,</span>
<span class="line">    serviceVersion = &quot;1.0.0&quot;</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line">// 2. 创建客户端</span>
<span class="line">client = createClient(config)</span>
<span class="line"></span>
<span class="line">// 3. 注册函数</span>
<span class="line">client.registerFunction(</span>
<span class="line">    FunctionDescriptor{id = &quot;func1&quot;, version = &quot;1.0.0&quot;},</span>
<span class="line">    handler</span>
<span class="line">)</span>
<span class="line"></span>
<span class="line">// 4. 连接并服务</span>
<span class="line">client.connect()</span>
<span class="line">client.serve()  // 阻塞直到停止</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="调用函数-所有-sdk-应遵循" tabindex="-1"><a class="header-anchor" href="#调用函数-所有-sdk-应遵循"><span>调用函数（所有 SDK 应遵循）</span></a></h3>
<div class="language-pseudo line-numbers-mode" data-highlighter="prismjs" data-ext="pseudo"><pre v-pre><code class="language-pseudo"><span class="line">// 1. 创建 Invoker</span>
<span class="line">invoker = createInvoker(InvokerConfig{address = &quot;127.0.0.1:8080&quot;})</span>
<span class="line"></span>
<span class="line">// 2. 调用函数</span>
<span class="line">result = invoker.invoke(</span>
<span class="line">    &quot;func1&quot;,</span>
<span class="line">    JSON.stringify({data: &quot;value&quot;}),</span>
<span class="line">    InvokeOptions{idempotencyKey = &quot;key-123&quot;}</span>
<span class="line">)</span>
<span class="line"></span>
<span class="line">// 3. 异步任务</span>
<span class="line">jobId = invoker.startJob(&quot;func1&quot;, payload)</span>
<span class="line"></span>
<span class="line">// 4. 流式事件</span>
<span class="line">for event in invoker.streamJob(jobId):</span>
<span class="line">    print(f&quot;Event: {event.type}&quot;)</span>
<span class="line">    if event.done:</span>
<span class="line">        break</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></div></template>


