<template><div><h1 id="故障排查指南" tabindex="-1"><a class="header-anchor" href="#故障排查指南"><span>故障排查指南</span></a></h1>
<p>本文档介绍 Croupier 常见问题的诊断和解决方法。</p>
<h2 id="目录" tabindex="-1"><a class="header-anchor" href="#目录"><span>目录</span></a></h2>
<nav class="table-of-contents"><ul><li><router-link to="#目录">目录</router-link></li><li><router-link to="#快速诊断流程">快速诊断流程</router-link></li><li><router-link to="#服务启动问题">服务启动问题</router-link><ul><li><router-link to="#server-无法启动">Server 无法启动</router-link></li><li><router-link to="#agent-无法连接到-server">Agent 无法连接到 Server</router-link></li></ul></li><li><router-link to="#函数调用问题">函数调用问题</router-link><ul><li><router-link to="#调用超时">调用超时</router-link></li><li><router-link to="#权限被拒绝">权限被拒绝</router-link></li><li><router-link to="#函数未找到">函数未找到</router-link></li></ul></li><li><router-link to="#性能问题">性能问题</router-link><ul><li><router-link to="#高内存使用">高内存使用</router-link></li><li><router-link to="#高-cpu-使用">高 CPU 使用</router-link></li><li><router-link to="#数据库连接池耗尽">数据库连接池耗尽</router-link></li></ul></li><li><router-link to="#审批流程问题">审批流程问题</router-link><ul><li><router-link to="#审批超时">审批超时</router-link></li></ul></li><li><router-link to="#数据一致性问题">数据一致性问题</router-link><ul><li><router-link to="#agent-离线后函数调用失败">Agent 离线后函数调用失败</router-link></li><li><router-link to="#作业状态不同步">作业状态不同步</router-link></li></ul></li><li><router-link to="#日志分析">日志分析</router-link><ul><li><router-link to="#查看错误日志">查看错误日志</router-link></li><li><router-link to="#查看特定函数调用日志">查看特定函数调用日志</router-link></li><li><router-link to="#json-日志查询">JSON 日志查询</router-link></li></ul></li><li><router-link to="#常见错误码">常见错误码</router-link></li><li><router-link to="#获取帮助">获取帮助</router-link><ul><li><router-link to="#收集诊断信息">收集诊断信息</router-link></li><li><router-link to="#联系支持">联系支持</router-link></li></ul></li><li><router-link to="#相关文档">相关文档</router-link></li></ul></nav>
<h2 id="快速诊断流程" tabindex="-1"><a class="header-anchor" href="#快速诊断流程"><span>快速诊断流程</span></a></h2>
<Mermaid code="eJxNkM1qwkAUhfd9inkBX6FQjdoH6C646Kpdlm5NIZJKIVabtpZUSG2rBgUh47+gDHkZ752ZtzDOjJBZXTjn+w7M3ePtwz25sS5I9q5sGSZy8A0vO96Z1UihcEmKdYza4P9zf4Nu40n1iqfEwfAPl18QUPAnDinZOHTxN5bNNmcJfLxiGEMa1vKAEsF2cWasut7j8z30W9ptqapI+9iJdeqQsnFz9s73UebmUwpvQ+02wMzj3QmM5mIVO6RiAPzxZC/IAJGMBG3kAXTHwmPnhaoBxPoZd8GBpZlMt0vqF67t03zUgub2wD5ltyco1XlZ5+qu5O6quY8Jva4q"></Mermaid><h2 id="服务启动问题" tabindex="-1"><a class="header-anchor" href="#服务启动问题"><span>服务启动问题</span></a></h2>
<h3 id="server-无法启动" tabindex="-1"><a class="header-anchor" href="#server-无法启动"><span>Server 无法启动</span></a></h3>
<h4 id="症状" tabindex="-1"><a class="header-anchor" href="#症状"><span>症状</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">$ ./croupier-server <span class="token parameter variable">--config</span> configs/server.yaml</span>
<span class="line">Error: failed to start server</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="诊断步骤" tabindex="-1"><a class="header-anchor" href="#诊断步骤"><span>诊断步骤</span></a></h4>
<ol>
<li><strong>检查配置文件语法</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 验证 YAML 语法</span></span>
<span class="line">python3 <span class="token parameter variable">-c</span> <span class="token string">"import yaml; yaml.safe_load(open('configs/server.yaml'))"</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或使用 croupier 自带的配置验证</span></span>
<span class="line">./croupier config <span class="token builtin class-name">test</span> <span class="token parameter variable">--config</span> configs/server.yaml</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>检查端口占用</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查 gRPC 端口</span></span>
<span class="line"><span class="token function">lsof</span> <span class="token parameter variable">-i</span> :8443</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查 HTTP 端口</span></span>
<span class="line"><span class="token function">lsof</span> <span class="token parameter variable">-i</span> :8080</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 如果被占用，查看占用进程</span></span>
<span class="line"><span class="token function">netstat</span> <span class="token parameter variable">-tlnp</span> <span class="token operator">|</span> <span class="token function">grep</span> <span class="token number">8443</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li><strong>检查 TLS 证书</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 验证证书文件存在</span></span>
<span class="line"><span class="token function">ls</span> <span class="token parameter variable">-la</span> data/server.crt data/server.key data/ca.crt</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证证书有效</span></span>
<span class="line">openssl x509 <span class="token parameter variable">-in</span> data/server.crt <span class="token parameter variable">-text</span> <span class="token parameter variable">-noout</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证证书与私钥匹配</span></span>
<span class="line">openssl x509 <span class="token parameter variable">-noout</span> <span class="token parameter variable">-modulus</span> <span class="token parameter variable">-in</span> server.crt <span class="token operator">|</span> openssl md5</span>
<span class="line">openssl rsa <span class="token parameter variable">-noout</span> <span class="token parameter variable">-modulus</span> <span class="token parameter variable">-in</span> server.key <span class="token operator">|</span> openssl md5</span>
<span class="line"><span class="token comment"># 两个 MD5 值应该相同</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="常见原因" tabindex="-1"><a class="header-anchor" href="#常见原因"><span>常见原因</span></a></h4>
<table>
<thead>
<tr>
<th>错误信息</th>
<th>原因</th>
<th>解决方法</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>address already in use</code></td>
<td>端口被占用</td>
<td>关闭占用进程或修改配置端口</td>
</tr>
<tr>
<td><code v-pre>no such file or directory</code></td>
<td>证书文件不存在</td>
<td>生成或放置正确的证书文件</td>
</tr>
<tr>
<td><code v-pre>permission denied</code></td>
<td>没有文件访问权限</td>
<td>修改文件权限</td>
</tr>
<tr>
<td><code v-pre>invalid configuration</code></td>
<td>YAML 配置错误</td>
<td>修复配置文件语法</td>
</tr>
</tbody>
</table>
<h3 id="agent-无法连接到-server" tabindex="-1"><a class="header-anchor" href="#agent-无法连接到-server"><span>Agent 无法连接到 Server</span></a></h3>
<h4 id="症状-1" tabindex="-1"><a class="header-anchor" href="#症状-1"><span>症状</span></a></h4>
<div class="language-log line-numbers-mode" data-highlighter="prismjs" data-ext="log"><pre v-pre><code class="language-log"><span class="line">ERRO<span class="token punctuation">[</span><span class="token number">0001</span><span class="token punctuation">]</span> <span class="token property">failed to connect to server:</span> connection refused</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h4 id="诊断步骤-1" tabindex="-1"><a class="header-anchor" href="#诊断步骤-1"><span>诊断步骤</span></a></h4>
<ol>
<li><strong>检查 Server 是否运行</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查 Server 进程</span></span>
<span class="line"><span class="token function">ps</span> aux <span class="token operator">|</span> <span class="token function">grep</span> croupier-server</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查 Server 健康状态</span></span>
<span class="line"><span class="token function">curl</span> http://server:8080/healthz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>检查网络连通性</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 测试网络连接</span></span>
<span class="line">telnet server <span class="token number">8443</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或使用 nc</span></span>
<span class="line"><span class="token function">nc</span> <span class="token parameter variable">-zv</span> server <span class="token number">8443</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li><strong>检查 TLS 证书</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 从 Agent 测试 Server TLS</span></span>
<span class="line">openssl s_client <span class="token parameter variable">-connect</span> server:8443 <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-cert</span> data/agent.crt <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-key</span> data/agent.key <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-CAfile</span> data/ca.crt</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="4">
<li><strong>检查 Agent 配置</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token comment"># agent.yaml</span></span>
<span class="line"><span class="token key atrule">agent</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">server_addr</span><span class="token punctuation">:</span> <span class="token string">"server:8443"</span>  <span class="token comment"># 确保地址正确</span></span>
<span class="line">  <span class="token key atrule">tls</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">ca_file</span><span class="token punctuation">:</span> <span class="token string">"data/ca.crt"</span></span>
<span class="line">    <span class="token key atrule">cert_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.crt"</span></span>
<span class="line">    <span class="token key atrule">key_file</span><span class="token punctuation">:</span> <span class="token string">"data/agent.key"</span></span>
<span class="line">    <span class="token key atrule">server_name</span><span class="token punctuation">:</span> <span class="token string">"server"</span>  <span class="token comment"># 必须与证书 CN 匹配</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="函数调用问题" tabindex="-1"><a class="header-anchor" href="#函数调用问题"><span>函数调用问题</span></a></h2>
<h3 id="调用超时" tabindex="-1"><a class="header-anchor" href="#调用超时"><span>调用超时</span></a></h3>
<h4 id="症状-2" tabindex="-1"><a class="header-anchor" href="#症状-2"><span>症状</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"code"</span><span class="token operator">:</span> <span class="token string">"DEADLINE_EXCEEDED"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"context deadline exceeded"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="解决方法" tabindex="-1"><a class="header-anchor" href="#解决方法"><span>解决方法</span></a></h4>
<ol>
<li><strong>增加超时时间</strong></li>
</ol>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"function_id"</span><span class="token operator">:</span> <span class="token string">"data.export"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"options"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"timeout"</span><span class="token operator">:</span> <span class="token number">300</span>  <span class="token comment">// 增加到 5 分钟</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>检查游戏服务器响应</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 在 Agent 所在服务器检查</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:19090/healthz</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li><strong>使用异步调用</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 改为创建作业</span></span>
<span class="line">POST /api/jobs</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"function_id"</span><span class="token builtin class-name">:</span> <span class="token string">"data.export"</span>,</span>
<span class="line">  <span class="token string">"payload"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span><span class="token punctuation">..</span>.<span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="权限被拒绝" tabindex="-1"><a class="header-anchor" href="#权限被拒绝"><span>权限被拒绝</span></a></h3>
<h4 id="症状-3" tabindex="-1"><a class="header-anchor" href="#症状-3"><span>症状</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"code"</span><span class="token operator">:</span> <span class="token string">"PERMISSION_DENIED"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"没有权限执行该操作"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="解决方法-1" tabindex="-1"><a class="header-anchor" href="#解决方法-1"><span>解决方法</span></a></h4>
<ol>
<li><strong>检查用户权限</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 获取当前用户信息</span></span>
<span class="line">GET /api/auth/me</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 响应包含用户权限</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"permissions"</span><span class="token builtin class-name">:</span> <span class="token punctuation">[</span><span class="token string">"player.view"</span>, <span class="token string">"item.view"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>检查函数权限要求</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 获取函数详情</span></span>
<span class="line">GET /api/functions/player.ban</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 响应包含所需权限</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"auth"</span><span class="token builtin class-name">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token string">"permission"</span><span class="token builtin class-name">:</span> <span class="token string">"player.ban"</span>,</span>
<span class="line">    <span class="token string">"two_person_rule"</span><span class="token builtin class-name">:</span> <span class="token boolean">true</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li><strong>添加权限或角色</strong></li>
</ol>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token comment">// 通过用户管理界面或 API 添加权限</span></span>
<span class="line">POST /api/users/<span class="token punctuation">{</span>user_id<span class="token punctuation">}</span>/roles</span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"role_id"</span><span class="token operator">:</span> <span class="token string">"gm"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="函数未找到" tabindex="-1"><a class="header-anchor" href="#函数未找到"><span>函数未找到</span></a></h3>
<h4 id="症状-4" tabindex="-1"><a class="header-anchor" href="#症状-4"><span>症状</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token property">"code"</span><span class="token operator">:</span> <span class="token string">"NOT_FOUND"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token property">"message"</span><span class="token operator">:</span> <span class="token string">"函数未注册"</span></span>
<span class="line">  <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="解决方法-2" tabindex="-1"><a class="header-anchor" href="#解决方法-2"><span>解决方法</span></a></h4>
<ol>
<li><strong>检查函数是否注册</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 列出所有函数</span></span>
<span class="line">GET /api/functions?game_id<span class="token operator">=</span>my-game<span class="token operator">&amp;</span><span class="token assign-left variable">env</span><span class="token operator">=</span>prod</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>检查 Agent 状态</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 列出所有 Agent</span></span>
<span class="line">GET /api/agents?game_id<span class="token operator">=</span>my-game<span class="token operator">&amp;</span><span class="token assign-left variable">env</span><span class="token operator">=</span>prod</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查 Agent 是否在线</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token string">"status"</span><span class="token builtin class-name">:</span> <span class="token string">"online"</span>,</span>
<span class="line">  <span class="token string">"functions"</span><span class="token builtin class-name">:</span> <span class="token punctuation">[</span><span class="token string">"player.ban"</span>, <span class="token string">"player.kick"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="3">
<li><strong>重新注册函数</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 在游戏服务器端重启 SDK</span></span>
<span class="line"><span class="token comment"># 或触发 Agent 重新注册</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="性能问题" tabindex="-1"><a class="header-anchor" href="#性能问题"><span>性能问题</span></a></h2>
<h3 id="高内存使用" tabindex="-1"><a class="header-anchor" href="#高内存使用"><span>高内存使用</span></a></h3>
<h4 id="诊断" tabindex="-1"><a class="header-anchor" href="#诊断"><span>诊断</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查进程内存</span></span>
<span class="line"><span class="token function">top</span> <span class="token parameter variable">-p</span> <span class="token variable"><span class="token variable">$(</span>pgrep croupier-server<span class="token variable">)</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 或使用</span></span>
<span class="line"><span class="token function">ps</span> aux <span class="token operator">|</span> <span class="token function">grep</span> croupier-server</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="解决方法-3" tabindex="-1"><a class="header-anchor" href="#解决方法-3"><span>解决方法</span></a></h4>
<ol>
<li><strong>调整日志配置</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">max_size</span><span class="token punctuation">:</span> <span class="token number">50</span>     <span class="token comment"># 减小日志文件大小</span></span>
<span class="line">    <span class="token key atrule">max_backups</span><span class="token punctuation">:</span> <span class="token number">2</span>   <span class="token comment"># 减少备份数量</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>配置内存限制</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 ulimit</span></span>
<span class="line"><span class="token builtin class-name">ulimit</span> <span class="token parameter variable">-v</span> <span class="token number">4194304</span>  <span class="token comment"># 限制虚拟内存 4GB</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># Docker 运行时限制</span></span>
<span class="line"><span class="token function">docker</span> run <span class="token parameter variable">-m</span> 2g croupier-server</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="高-cpu-使用" tabindex="-1"><a class="header-anchor" href="#高-cpu-使用"><span>高 CPU 使用</span></a></h3>
<h4 id="诊断-1" tabindex="-1"><a class="header-anchor" href="#诊断-1"><span>诊断</span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 检查 CPU 使用</span></span>
<span class="line"><span class="token function">top</span> <span class="token parameter variable">-p</span> <span class="token variable"><span class="token variable">$(</span>pgrep croupier-server<span class="token variable">)</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 生成 CPU profile</span></span>
<span class="line"><span class="token function">curl</span> http://localhost:8080/debug/pprof/profile <span class="token operator">></span> cpu.prof</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="解决方法-4" tabindex="-1"><a class="header-anchor" href="#解决方法-4"><span>解决方法</span></a></h4>
<ol>
<li><strong>检查日志级别</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">log</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">level</span><span class="token punctuation">:</span> warn  <span class="token comment"># 生产环境使用 warn 或 error</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>启用缓存</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">cache</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">enabled</span><span class="token punctuation">:</span> <span class="token boolean important">true</span></span>
<span class="line">    <span class="token key atrule">backend</span><span class="token punctuation">:</span> redis</span>
<span class="line">    <span class="token key atrule">redis_addr</span><span class="token punctuation">:</span> <span class="token string">"localhost:6379"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="数据库连接池耗尽" tabindex="-1"><a class="header-anchor" href="#数据库连接池耗尽"><span>数据库连接池耗尽</span></a></h3>
<h4 id="症状-5" tabindex="-1"><a class="header-anchor" href="#症状-5"><span>症状</span></a></h4>
<div class="language-log line-numbers-mode" data-highlighter="prismjs" data-ext="log"><pre v-pre><code class="language-log"><span class="line">ERRO<span class="token punctuation">[</span><span class="token number">0001</span><span class="token punctuation">]</span> <span class="token property">failed to acquire database connection:</span> connection pool exhausted</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h4 id="解决方法-5" tabindex="-1"><a class="header-anchor" href="#解决方法-5"><span>解决方法</span></a></h4>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">db</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">max_open_conns</span><span class="token punctuation">:</span> <span class="token number">100</span></span>
<span class="line">    <span class="token key atrule">max_idle_conns</span><span class="token punctuation">:</span> <span class="token number">10</span></span>
<span class="line">    <span class="token key atrule">conn_max_lifetime</span><span class="token punctuation">:</span> <span class="token string">"1h"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="审批流程问题" tabindex="-1"><a class="header-anchor" href="#审批流程问题"><span>审批流程问题</span></a></h2>
<h3 id="审批超时" tabindex="-1"><a class="header-anchor" href="#审批超时"><span>审批超时</span></a></h3>
<h4 id="症状-6" tabindex="-1"><a class="header-anchor" href="#症状-6"><span>症状</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"pending"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"error"</span><span class="token operator">:</span> <span class="token string">"approval timeout"</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="解决方法-6" tabindex="-1"><a class="header-anchor" href="#解决方法-6"><span>解决方法</span></a></h4>
<ol>
<li><strong>检查审批配置</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">audit</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">approval_timeout</span><span class="token punctuation">:</span> <span class="token string">"48h"</span>  <span class="token comment"># 增加超时时间</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>查看待审批列表</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">GET /api/approvals?state<span class="token operator">=</span>pending</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><ol start="3">
<li><strong>催办审批</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 通过系统通知相关人员</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="数据一致性问题" tabindex="-1"><a class="header-anchor" href="#数据一致性问题"><span>数据一致性问题</span></a></h2>
<h3 id="agent-离线后函数调用失败" tabindex="-1"><a class="header-anchor" href="#agent-离线后函数调用失败"><span>Agent 离线后函数调用失败</span></a></h3>
<h4 id="解决方法-7" tabindex="-1"><a class="header-anchor" href="#解决方法-7"><span>解决方法</span></a></h4>
<ol>
<li><strong>配置负载均衡</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">loadbalancer</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">strategy</span><span class="token punctuation">:</span> <span class="token string">"least_conn"</span>  <span class="token comment"># 使用最少连接策略</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>配置健康检查</strong></li>
</ol>
<div class="language-yaml line-numbers-mode" data-highlighter="prismjs" data-ext="yml"><pre v-pre><code class="language-yaml"><span class="line"><span class="token key atrule">server</span><span class="token punctuation">:</span></span>
<span class="line">  <span class="token key atrule">agent</span><span class="token punctuation">:</span></span>
<span class="line">    <span class="token key atrule">heartbeat_interval</span><span class="token punctuation">:</span> <span class="token string">"30s"</span></span>
<span class="line">    <span class="token key atrule">heartbeat_timeout</span><span class="token punctuation">:</span> <span class="token string">"2m"</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="作业状态不同步" tabindex="-1"><a class="header-anchor" href="#作业状态不同步"><span>作业状态不同步</span></a></h3>
<h4 id="症状-7" tabindex="-1"><a class="header-anchor" href="#症状-7"><span>症状</span></a></h4>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"job_id"</span><span class="token operator">:</span> <span class="token string">"job_123"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"status"</span><span class="token operator">:</span> <span class="token string">"running"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"progress"</span><span class="token operator">:</span> <span class="token number">0.5</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"><span class="token comment">// 实际作业已完成，但状态未更新</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="解决方法-8" tabindex="-1"><a class="header-anchor" href="#解决方法-8"><span>解决方法</span></a></h4>
<ol>
<li><strong>检查事件流</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 手动获取事件</span></span>
<span class="line">GET /api/jobs/job_123/events</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2">
<li><strong>重启相关 Agent</strong></li>
</ol>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line">systemctl restart croupier-agent</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h2 id="日志分析" tabindex="-1"><a class="header-anchor" href="#日志分析"><span>日志分析</span></a></h2>
<h3 id="查看错误日志" tabindex="-1"><a class="header-anchor" href="#查看错误日志"><span>查看错误日志</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># Server 错误日志</span></span>
<span class="line"><span class="token function">grep</span> <span class="token string">"ERRO"</span> /var/log/croupier/server.log</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Agent 错误日志</span></span>
<span class="line"><span class="token function">grep</span> <span class="token string">"ERRO"</span> /var/log/croupier/agent.log</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="查看特定函数调用日志" tabindex="-1"><a class="header-anchor" href="#查看特定函数调用日志"><span>查看特定函数调用日志</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 查找特定函数的调用</span></span>
<span class="line"><span class="token function">grep</span> <span class="token string">"player.ban"</span> /var/log/croupier/server.log</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 查看特定时间范围</span></span>
<span class="line"><span class="token function">awk</span> <span class="token string">'/2024-12-01T10:00/,/2024-12-01T11:00/'</span> /var/log/croupier/server.log</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="json-日志查询" tabindex="-1"><a class="header-anchor" href="#json-日志查询"><span>JSON 日志查询</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 使用 jq 查询 JSON 日志</span></span>
<span class="line"><span class="token function">cat</span> /var/log/croupier/server.log <span class="token operator">|</span> jq <span class="token string">'select(.level=="error")'</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 查找特定用户操作</span></span>
<span class="line"><span class="token function">cat</span> /var/log/croupier/server.log <span class="token operator">|</span> jq <span class="token string">'select(.user_id=="user_123")'</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="常见错误码" tabindex="-1"><a class="header-anchor" href="#常见错误码"><span>常见错误码</span></a></h2>
<table>
<thead>
<tr>
<th>错误码</th>
<th>说明</th>
<th>解决方法</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>INVALID_ARGUMENT</code></td>
<td>参数错误</td>
<td>检查请求参数格式</td>
</tr>
<tr>
<td><code v-pre>UNAUTHENTICATED</code></td>
<td>未认证</td>
<td>检查 Token 是否有效</td>
</tr>
<tr>
<td><code v-pre>PERMISSION_DENIED</code></td>
<td>权限不足</td>
<td>联系管理员添加权限</td>
</tr>
<tr>
<td><code v-pre>NOT_FOUND</code></td>
<td>资源不存在</td>
<td>检查函数/Agent 是否注册</td>
</tr>
<tr>
<td><code v-pre>ALREADY_EXISTS</code></td>
<td>资源已存在</td>
<td>使用更新而非创建</td>
</tr>
<tr>
<td><code v-pre>RESOURCE_EXHAUSTED</code></td>
<td>请求过多</td>
<td>等待后重试或增加限流</td>
</tr>
<tr>
<td><code v-pre>INTERNAL</code></td>
<td>内部错误</td>
<td>查看服务器日志</td>
</tr>
<tr>
<td><code v-pre>UNAVAILABLE</code></td>
<td>服务不可用</td>
<td>检查服务状态</td>
</tr>
<tr>
<td><code v-pre>DEADLINE_EXCEEDED</code></td>
<td>超时</td>
<td>增加超时时间</td>
</tr>
</tbody>
</table>
<h2 id="获取帮助" tabindex="-1"><a class="header-anchor" href="#获取帮助"><span>获取帮助</span></a></h2>
<h3 id="收集诊断信息" tabindex="-1"><a class="header-anchor" href="#收集诊断信息"><span>收集诊断信息</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token shebang important">#!/bin/bash</span></span>
<span class="line"><span class="token comment"># diagnostic.sh</span></span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== Croupier 诊断信息 ==="</span></span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">""</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== 版本信息 ==="</span></span>
<span class="line">./croupier-server <span class="token parameter variable">--version</span></span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">""</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== 配置文件 ==="</span></span>
<span class="line"><span class="token function">cat</span> configs/server.yaml</span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">""</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== 进程状态 ==="</span></span>
<span class="line"><span class="token function">ps</span> aux <span class="token operator">|</span> <span class="token function">grep</span> croupier</span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">""</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== 网络连接 ==="</span></span>
<span class="line"><span class="token function">netstat</span> <span class="token parameter variable">-tlnp</span> <span class="token operator">|</span> <span class="token function">grep</span> croupier</span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">""</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== 最近错误日志 ==="</span></span>
<span class="line"><span class="token function">tail</span> <span class="token parameter variable">-n</span> <span class="token number">50</span> /var/log/croupier/server.log <span class="token operator">|</span> <span class="token function">grep</span> ERRO</span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">""</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== 磁盘使用 ==="</span></span>
<span class="line"><span class="token function">df</span> <span class="token parameter variable">-h</span></span>
<span class="line"></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">""</span></span>
<span class="line"><span class="token builtin class-name">echo</span> <span class="token string">"=== 内存使用 ==="</span></span>
<span class="line"><span class="token function">free</span> <span class="token parameter variable">-h</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="联系支持" tabindex="-1"><a class="header-anchor" href="#联系支持"><span>联系支持</span></a></h3>
<p>收集以下信息后可以更快获得帮助：</p>
<ol>
<li>Croupier 版本</li>
<li>操作系统版本</li>
<li>错误日志（去除敏感信息）</li>
<li>配置文件（去除敏感信息）</li>
<li>复现步骤</li>
</ol>
<h2 id="相关文档" tabindex="-1"><a class="header-anchor" href="#相关文档"><span>相关文档</span></a></h2>
<ul>
<li><RouteLink to="/guide/operations/monitoring.html">监控指南</RouteLink></li>
<li><RouteLink to="/guide/operations/security.html">安全配置</RouteLink></li>
<li><RouteLink to="/guide/configuration.html">配置管理</RouteLink></li>
</ul>
</div></template>


