import{_ as s,c as a,a as e,o as l}from"./app-C_dHcy8Q.js";const i={};function t(p,n){return l(),a("div",null,[...n[0]||(n[0]=[e(`<h1 id="croupier-c-sdk-目录索引与文件映射" tabindex="-1"><a class="header-anchor" href="#croupier-c-sdk-目录索引与文件映射"><span>Croupier C++ SDK 目录索引与文件映射</span></a></h1><h2 id="📦-完整目录结构" tabindex="-1"><a class="header-anchor" href="#📦-完整目录结构"><span>📦 完整目录结构</span></a></h2><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">sdks/cpp/</span>
<span class="line">│</span>
<span class="line">├── 📄 CMakeLists.txt (355 行)</span>
<span class="line">│   ├─ 项目配置：C++17, vcpkg 集成</span>
<span class="line">│   ├─ 多平台支持：Windows/Linux/macOS (x64/x86/arm64)</span>
<span class="line">│   ├─ 依赖配置：gRPC, Protobuf, nlohmann-json</span>
<span class="line">│   ├─ 库目标：shared + static 并行构建</span>
<span class="line">│   ├─ 示例程序：croupier-example, virtual-object-demo</span>
<span class="line">│   └─ 安装配置：CMake config + CPack 打包</span>
<span class="line">│</span>
<span class="line">├── 📂 include/croupier/sdk/</span>
<span class="line">│   └─ croupier_client.h (270 行) ⭐ 核心公开接口</span>
<span class="line">│       ├─ FunctionHandler 类型定义 (L:18)</span>
<span class="line">│       ├─ FunctionDescriptor 结构 (L:21-25)</span>
<span class="line">│       ├─ VirtualObjectDescriptor 结构 (L:35-43)</span>
<span class="line">│       ├─ RelationshipDef 结构 (L:28-32)</span>
<span class="line">│       ├─ ComponentDescriptor 结构 (L:46-55)</span>
<span class="line">│       ├─ ClientConfig 结构 (L:57-83) 🎮 game_id/env</span>
<span class="line">│       ├─ InvokerConfig 结构 (L:86-106) 🎮 game_id/env</span>
<span class="line">│       ├─ InvokeOptions 结构 (L:109-116)</span>
<span class="line">│       ├─ JobEvent 结构 (L:119-125)</span>
<span class="line">│       ├─ CroupierClient 类 (L:128-186)</span>
<span class="line">│       │  ├─ RegisterFunction() (L:136)</span>
<span class="line">│       │  ├─ RegisterVirtualObject() (L:141-144) ⭐</span>
<span class="line">│       │  ├─ RegisterComponent() (L:147)</span>
<span class="line">│       │  ├─ LoadComponentFromFile() (L:150)</span>
<span class="line">│       │  ├─ GetRegisteredObjects() (L:155)</span>
<span class="line">│       │  ├─ GetRegisteredComponents() (L:158)</span>
<span class="line">│       │  ├─ UnregisterVirtualObject() (L:161)</span>
<span class="line">│       │  ├─ UnregisterComponent() (L:164)</span>
<span class="line">│       │  ├─ Connect() (L:169)</span>
<span class="line">│       │  ├─ Serve() (L:172)</span>
<span class="line">│       │  ├─ Stop() (L:175)</span>
<span class="line">│       │  ├─ Close() (L:178)</span>
<span class="line">│       │  └─ GetLocalAddress() (L:181)</span>
<span class="line">│       ├─ CroupierInvoker 类 (L:189-220)</span>
<span class="line">│       │  ├─ Connect() (L:195)</span>
<span class="line">│       │  ├─ Invoke() (L:198-199)</span>
<span class="line">│       │  ├─ StartJob() (L:202-203)</span>
<span class="line">│       │  ├─ StreamJob() (L:206)</span>
<span class="line">│       │  ├─ CancelJob() (L:209)</span>
<span class="line">│       │  ├─ SetSchema() (L:212)</span>
<span class="line">│       │  └─ Close() (L:215)</span>
<span class="line">│       └─ utils 命名空间 (L:223-267) 🛠️ 工具函数</span>
<span class="line">│          ├─ NewIdempotencyKey()</span>
<span class="line">│          ├─ ValidateJSON()</span>
<span class="line">│          ├─ ParseJSON()</span>
<span class="line">│          ├─ ToJSON()</span>
<span class="line">│          ├─ LoadObjectDescriptor() ⭐</span>
<span class="line">│          ├─ LoadComponentDescriptor() ⭐</span>
<span class="line">│          ├─ ValidateObjectDescriptor() ⭐</span>
<span class="line">│          ├─ ValidateComponentDescriptor() ⭐</span>
<span class="line">│          ├─ GenerateObjectTemplate()</span>
<span class="line">│          ├─ GenerateComponentTemplate()</span>
<span class="line">│          ├─ ObjectDescriptorToJSON()</span>
<span class="line">│          └─ ComponentDescriptorToJSON()</span>
<span class="line">│</span>
<span class="line">├── 📂 src/</span>
<span class="line">│   └─ croupier_client.cpp (898 行) ⭐ 核心实现</span>
<span class="line">│       ├─ Utils 工具函数实现 (L:15-99)</span>
<span class="line">│       │  ├─ NewIdempotencyKey() 生成 UUID (L:17-27)</span>
<span class="line">│       │  ├─ ValidateJSON() JSON 语法验证 (L:29-65)</span>
<span class="line">│       │  ├─ ParseJSON() JSON 解析 (L:67-85)</span>
<span class="line">│       │  └─ ToJSON() JSON 序列化 (L:87-98)</span>
<span class="line">│       │</span>
<span class="line">│       ├─ CroupierClient::Impl 类实现 (L:102-407) ⭐</span>
<span class="line">│       │  ├─ 成员变量 (L:104-115)</span>
<span class="line">│       │  │  ├─ config_ 配置存储</span>
<span class="line">│       │  │  ├─ handlers_ 函数映射表</span>
<span class="line">│       │  │  ├─ descriptors_ 元数据</span>
<span class="line">│       │  │  ├─ objects_ 虚拟对象表</span>
<span class="line">│       │  │  └─ components_ 组件表</span>
<span class="line">│       │  │</span>
<span class="line">│       │  ├─ Impl() 构造函数 (L:117-131) 🎮 game_id/env 验证</span>
<span class="line">│       │  │  ├─ game_id 空检查 (L:119-121)</span>
<span class="line">│       │  │  ├─ env 有效性验证 (L:123-127)</span>
<span class="line">│       │  │  └─ 日志记录 (L:129-130)</span>
<span class="line">│       │  │</span>
<span class="line">│       │  ├─ RegisterFunction() (L:137-148)</span>
<span class="line">│       │  ├─ RegisterVirtualObject() (L:151-193) ⭐</span>
<span class="line">│       │  ├─ RegisterComponent() (L:196-234) ⭐</span>
<span class="line">│       │  ├─ LoadComponentFromFile() (L:237-246)</span>
<span class="line">│       │  ├─ GetRegisteredObjects() (L:249-255)</span>
<span class="line">│       │  ├─ GetRegisteredComponents() (L:258-264)</span>
<span class="line">│       │  ├─ UnregisterVirtualObject() (L:267-287)</span>
<span class="line">│       │  ├─ UnregisterComponent() (L:290-315)</span>
<span class="line">│       │  ├─ Connect() (L:317-337) 📡 Agent 连接</span>
<span class="line">│       │  ├─ Serve() (L:339-353) 🔄 主服务循环</span>
<span class="line">│       │  ├─ Stop() (L:355-364)</span>
<span class="line">│       │  ├─ Close() (L:366-370)</span>
<span class="line">│       │  ├─ GetLocalAddress() (L:372-374)</span>
<span class="line">│       │  └─ StartLocalServer() (L:377-406) 🖧 本地 gRPC</span>
<span class="line">│       │</span>
<span class="line">│       ├─ CroupierInvoker::Impl 类实现 (L:410-543)</span>
<span class="line">│       │  ├─ Connect() (L:418-429) 📡 连接</span>
<span class="line">│       │  ├─ Invoke() (L:431-455) 📨 同步调用</span>
<span class="line">│       │  ├─ StartJob() (L:457-472) 🚀 异步任务</span>
<span class="line">│       │  ├─ StreamJob() (L:474-517) 📊 流式传输</span>
<span class="line">│       │  ├─ CancelJob() (L:519-531) ⏹️ 取消任务</span>
<span class="line">│       │  ├─ SetSchema() (L:533-536)</span>
<span class="line">│       │  └─ Close() (L:538-542)</span>
<span class="line">│       │</span>
<span class="line">│       ├─ CroupierClient 公开接口转发 (L:546-609)</span>
<span class="line">│       ├─ CroupierInvoker 公开接口转发 (L:611-645)</span>
<span class="line">│       └─ Utils 工具函数实现 (L:648-896)</span>
<span class="line">│          ├─ LoadObjectDescriptor() (L:651-661)</span>
<span class="line">│          ├─ LoadComponentDescriptor() (L:664-674)</span>
<span class="line">│          ├─ ValidateObjectDescriptor() (L:677-718) ⭐ 验证逻辑</span>
<span class="line">│          ├─ ValidateComponentDescriptor() (L:721-750) ⭐</span>
<span class="line">│          ├─ GenerateObjectTemplate() (L:753-777)</span>
<span class="line">│          ├─ GenerateComponentTemplate() (L:780-793)</span>
<span class="line">│          ├─ ParseObjectDescriptor() (L:796-804)</span>
<span class="line">│          ├─ ParseComponentDescriptor() (L:807-815)</span>
<span class="line">│          ├─ ObjectDescriptorToJSON() (L:818-859)</span>
<span class="line">│          └─ ComponentDescriptorToJSON() (L:862-893)</span>
<span class="line">│</span>
<span class="line">├── 📂 examples/</span>
<span class="line">│   └─ virtual_object_demo.cpp (334 行) 📚 完整示例</span>
<span class="line">│       ├─ Wallet 处理器 (L:8-60)</span>
<span class="line">│       │  ├─ WalletGetHandler() (L:10-24)</span>
<span class="line">│       │  ├─ WalletTransferHandler() (L:26-45)</span>
<span class="line">│       │  └─ WalletDepositHandler() (L:47-60)</span>
<span class="line">│       ├─ Currency 处理器 (L:62-93)</span>
<span class="line">│       │  ├─ CurrencyGetHandler() (L:64-78)</span>
<span class="line">│       │  └─ CurrencyCreateHandler() (L:80-92)</span>
<span class="line">│       ├─ Demo 1: 单函数注册 (L:96-119)</span>
<span class="line">│       ├─ Demo 2: 虚拟对象 (L:121-175) ⭐</span>
<span class="line">│       ├─ Demo 3: 组件注册 (L:177-244) ⭐</span>
<span class="line">│       ├─ Demo 4: 模板生成 (L:246-260)</span>
<span class="line">│       ├─ Demo 5: 序列化 (L:262-284)</span>
<span class="line">│       ├─ Demo 6: 验证 (L:286-306)</span>
<span class="line">│       └─ main() 启动 (L:308-334)</span>
<span class="line">│</span>
<span class="line">├── 📂 .github/workflows/</span>
<span class="line">│   └─ cpp-sdk-build.yml (483 行) 🤖 CI/CD 自动化</span>
<span class="line">│       ├─ 版本管理任务 (L:44-118)</span>
<span class="line">│       ├─ 多平台构建矩阵 (L:120-165)</span>
<span class="line">│       ├─ 构建步骤 (L:168-237)</span>
<span class="line">│       ├─ 测试执行 (L:231-236)</span>
<span class="line">│       ├��� 分离打包 (L:239-284)</span>
<span class="line">│       ├─ 发布流程 (L:303-462)</span>
<span class="line">│       └─ 通知系统 (L:465-483)</span>
<span class="line">│</span>
<span class="line">├── 📄 vcpkg.json (40 行) 📦 依赖声明</span>
<span class="line">│   ├─ grpc (含 codegen)</span>
<span class="line">│   ├─ protobuf (含 zlib)</span>
<span class="line">│   ├─ nlohmann-json</span>
<span class="line">│   └─ gtest (可选)</span>
<span class="line">│</span>
<span class="line">├── 📄 README.md (569 行) 📖 用户文档</span>
<span class="line">│   ├─ 核心特性说明</span>
<span class="line">│   ├─ 快速开始指南</span>
<span class="line">│   ├─ 使用示例 (4 个)</span>
<span class="line">│   ├─ 架构设计说明</span>
<span class="line">│   ├─ API 参考</span>
<span class="line">│   ├─ 部署和分发</span>
<span class="line">│   ├─ 开发环境搭建</span>
<span class="line">│   ├─ 进阶主题</span>
<span class="line">│   └─ 贡献指南</span>
<span class="line">│</span>
<span class="line">└─ VIRTUAL_OBJECT_REGISTRATION.md (441 行) 🏗️ 架构深度文档</span>
<span class="line">    ├─ 四层抽象模型</span>
<span class="line">    ├─ 设计理念说明</span>
<span class="line">    ├─ C++ SDK 扩展方案</span>
<span class="line">    ├─ 4 个使用示例</span>
<span class="line">    ├─ 实现指南</span>
<span class="line">    ├─ 架构优势分析</span>
<span class="line">    └─ 后续规划</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr><h2 id="🎯-按功能查找代码" tabindex="-1"><a class="header-anchor" href="#🎯-按功能查找代码"><span>🎯 按功能查找代码</span></a></h2><h3 id="🔴-spi-service-provider-interface" tabindex="-1"><a class="header-anchor" href="#🔴-spi-service-provider-interface"><span>🔴 SPI (Service Provider Interface)</span></a></h3><table><thead><tr><th>功能</th><th>文件</th><th>行号</th></tr></thead><tbody><tr><td>FunctionHandler 类型</td><td>croupier_client.h</td><td>18</td></tr><tr><td>RegisterFunction()</td><td>croupier_client.h</td><td>136</td></tr><tr><td>RegisterVirtualObject()</td><td>croupier_client.h</td><td>141-144</td></tr><tr><td>RegisterComponent()</td><td>croupier_client.h</td><td>147</td></tr><tr><td>Handler 执行</td><td>croupier_client.cpp</td><td>431-455</td></tr><tr><td>Handler 存储</td><td>croupier_client.cpp</td><td>104-106</td></tr></tbody></table><p><strong>关键代码片段</strong>：</p><div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre><code class="language-cpp"><span class="line"><span class="token comment">// 定义 (line 18)</span></span>
<span class="line"><span class="token keyword">using</span> FunctionHandler <span class="token operator">=</span> std<span class="token double-colon punctuation">::</span>function<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span><span class="token function">string</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span><span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span><span class="token punctuation">)</span><span class="token operator">&gt;</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 注册 (line 136)</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">RegisterFunction</span><span class="token punctuation">(</span><span class="token keyword">const</span> FunctionDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">,</span> FunctionHandler handler<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 调用 (line 431-455)</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">Invoke</span><span class="token punctuation">(</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span> handler <span class="token operator">=</span> schemas_<span class="token punctuation">[</span>function_id<span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token function">handler</span><span class="token punctuation">(</span>context<span class="token punctuation">,</span> payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr><h3 id="🟢-game-id-和-env" tabindex="-1"><a class="header-anchor" href="#🟢-game-id-和-env"><span>🟢 game_id 和 env</span></a></h3><table><thead><tr><th>功能</th><th>文件</th><th>行号</th></tr></thead><tbody><tr><td>ClientConfig 定义</td><td>croupier_client.h</td><td>57-83</td></tr><tr><td>game_id/env 字段</td><td>croupier_client.h</td><td>65-66, 90-91</td></tr><tr><td>初始化验证</td><td>croupier_client.cpp</td><td>117-131</td></tr><tr><td>日志记录</td><td>croupier_client.cpp</td><td>129-130</td></tr><tr><td>Proto 定义</td><td>control.proto</td><td>22-23</td></tr></tbody></table><p><strong>关键代码片段</strong>：</p><div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre><code class="language-cpp"><span class="line"><span class="token comment">// 配置 (lines 65-66, 90-91)</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">ClientConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string game_id<span class="token punctuation">;</span>           <span class="token comment">// Required</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string env <span class="token operator">=</span> <span class="token string">&quot;development&quot;</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 验证 (lines 117-131)</span></span>
<span class="line"><span class="token keyword">if</span> <span class="token punctuation">(</span>config_<span class="token punctuation">.</span>game_id<span class="token punctuation">.</span><span class="token function">empty</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">&quot;Warning: game_id is required for proper backend separation&quot;</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"><span class="token keyword">if</span> <span class="token punctuation">(</span>config_<span class="token punctuation">.</span>env <span class="token operator">!=</span> <span class="token string">&quot;development&quot;</span> <span class="token operator">&amp;&amp;</span> config_<span class="token punctuation">.</span>env <span class="token operator">!=</span> <span class="token string">&quot;staging&quot;</span> <span class="token operator">&amp;&amp;</span> config_<span class="token punctuation">.</span>env <span class="token operator">!=</span> <span class="token string">&quot;production&quot;</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>cerr <span class="token operator">&lt;&lt;</span> <span class="token string">&quot;Warning: Unknown environment &#39;&quot;</span> <span class="token operator">&lt;&lt;</span> config_<span class="token punctuation">.</span>env <span class="token operator">&lt;&lt;</span> <span class="token string">&quot;&#39;&quot;</span> <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr><h3 id="🔵-与-agent-交互" tabindex="-1"><a class="header-anchor" href="#🔵-与-agent-交互"><span>🔵 与 Agent 交互</span></a></h3><table><thead><tr><th>功能</th><th>文件</th><th>行号</th></tr></thead><tbody><tr><td>Connect()</td><td>croupier_client.h</td><td>169</td></tr><tr><td>Connect() 实现</td><td>croupier_client.cpp</td><td>317-337</td></tr><tr><td>StartLocalServer()</td><td>croupier_client.cpp</td><td>377-406</td></tr><tr><td>Serve()</td><td>croupier_client.h</td><td>172</td></tr><tr><td>Heartbeat</td><td>croupier_client.cpp</td><td>（待实现）</td></tr><tr><td>Proto 协议</td><td>local.proto</td><td>1-40</td></tr></tbody></table><p><strong>关键代码片段</strong>：</p><div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre><code class="language-cpp"><span class="line"><span class="token comment">// 步骤1：连接 Agent (line 317)</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 连接到 agent_addr (127.0.0.1:19090)</span></span>
<span class="line">    <span class="token comment">// 调用 LocalControlService::RegisterLocal()</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 步骤2：启动本地服务器</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token operator">!</span><span class="token function">StartLocalServer</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span> <span class="token comment">/* ... */</span> <span class="token punctuation">}</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// 步骤3：注册会话</span></span>
<span class="line">    <span class="token comment">// TODO: Register with agent via gRPC</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 步骤2：本地服务器 (line 377)</span></span>
<span class="line"><span class="token keyword">bool</span> <span class="token function">StartLocalServer</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 解析 local_listen 配置</span></span>
<span class="line">    <span class="token comment">// 分配端口（port=0 时自动分配）</span></span>
<span class="line">    <span class="token comment">// 保存 local_address_</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>Proto 消息</strong> (local.proto):</p><div class="language-protobuf line-numbers-mode" data-highlighter="prismjs" data-ext="protobuf"><pre><code class="language-protobuf"><span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterLocalRequest</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> service_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> version <span class="token operator">=</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token builtin">string</span> rpc_addr <span class="token operator">=</span> <span class="token number">3</span><span class="token punctuation">;</span>                    <span class="token comment">// 本地服务地址</span></span>
<span class="line">    <span class="token keyword">repeated</span> <span class="token positional-class-name class-name">LocalFunctionDescriptor</span> functions <span class="token operator">=</span> <span class="token number">4</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">message</span> <span class="token class-name">RegisterLocalResponse</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token builtin">string</span> session_id <span class="token operator">=</span> <span class="token number">1</span><span class="token punctuation">;</span>  <span class="token comment">// 后续用于心跳</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr><h3 id="🟣-权限接口" tabindex="-1"><a class="header-anchor" href="#🟣-权限接口"><span>🟣 权限接口</span></a></h3><table><thead><tr><th>功能</th><th>文件</th><th>行号</th></tr></thead><tbody><tr><td>auth_token</td><td>croupier_client.h</td><td>77</td></tr><tr><td>TLS 配置</td><td>croupier_client.h</td><td>70-74</td></tr><tr><td>InvokeOptions</td><td>croupier_client.h</td><td>109-116</td></tr><tr><td>metadata</td><td>croupier_client.h</td><td>115</td></tr><tr><td>认证示例</td><td>README.md</td><td>490-506</td></tr></tbody></table><p><strong>关键代码片段</strong>：</p><div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre><code class="language-cpp"><span class="line"><span class="token comment">// 认证配置 (lines 70-78)</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">ClientConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// ========== Authentication ==========</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string auth_token<span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&gt;</span> headers<span class="token punctuation">;</span></span>
<span class="line">    </span>
<span class="line">    <span class="token comment">// ========== TLS Configuration ==========</span></span>
<span class="line">    <span class="token keyword">bool</span> insecure <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string cert_file<span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string key_file<span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string ca_file<span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string server_name<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 调用时权限 (lines 109-116)</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">InvokeOptions</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string idempotency_key<span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string trace_id<span class="token punctuation">;</span>                   <span class="token comment">// 审计追踪</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&gt;</span> metadata<span class="token punctuation">;</span>  <span class="token comment">// 权限元数据</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr><h2 id="📍-关键类和方法定位" tabindex="-1"><a class="header-anchor" href="#📍-关键类和方法定位"><span>📍 关键类和方法定位</span></a></h2><h3 id="croupierclient-核心方法" tabindex="-1"><a class="header-anchor" href="#croupierclient-核心方法"><span>CroupierClient 核心方法</span></a></h3><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">文件: croupier_client.h/cpp</span>
<span class="line"></span>
<span class="line">CroupierClient</span>
<span class="line">  ├─ 构造函数 (h:130, cpp:546-547)</span>
<span class="line">  ├─ RegisterFunction() (h:136, cpp:552-554)</span>
<span class="line">  ├─ RegisterVirtualObject() (h:141-144, cpp:557-561) ⭐</span>
<span class="line">  ├─ RegisterComponent() (h:147, cpp:563-565) ⭐</span>
<span class="line">  ├─ LoadComponentFromFile() (h:150, cpp:567-569)</span>
<span class="line">  ├─ GetRegisteredObjects() (h:155, cpp:572-574)</span>
<span class="line">  ├─ GetRegisteredComponents() (h:158, cpp:576-578)</span>
<span class="line">  ├─ UnregisterVirtualObject() (h:161, cpp:580-582)</span>
<span class="line">  ├─ UnregisterComponent() (h:164, cpp:584-586)</span>
<span class="line">  ├─ Connect() (h:169, cpp:590-592)</span>
<span class="line">  ├─ Serve() (h:172, cpp:594-596)</span>
<span class="line">  ���─ Stop() (h:175, cpp:598-600)</span>
<span class="line">  ├─ Close() (h:178, cpp:602-604)</span>
<span class="line">  └─ GetLocalAddress() (h:181, cpp:606-608)</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="impl-内部实现" tabindex="-1"><a class="header-anchor" href="#impl-内部实现"><span>Impl 内部实现</span></a></h3><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">文件: croupier_client.cpp</span>
<span class="line"></span>
<span class="line">CroupierClient::Impl (L:102-407)</span>
<span class="line">  ├─ 成员变量 (L:104-115)</span>
<span class="line">  ├─ 构造函数 (L:117-131)</span>
<span class="line">  ├─ 虚拟对象注册 (L:151-193)</span>
<span class="line">  ├─ 组件注册 (L:196-234)</span>
<span class="line">  ├─ 连接和服务 (L:317-406)</span>
<span class="line">  └─ Handler 存储和验证</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><hr><h2 id="🔧-常用代码片段位置" tabindex="-1"><a class="header-anchor" href="#🔧-常用代码片段位置"><span>🔧 常用代码片段位置</span></a></h2><table><thead><tr><th>用途</th><th>文件</th><th>行号</th><th>说明</th></tr></thead><tbody><tr><td>生成幂等性 ID</td><td>croupier_client.cpp</td><td>17-27</td><td>NewIdempotencyKey()</td></tr><tr><td>JSON 验证</td><td>croupier_client.cpp</td><td>29-65</td><td>ValidateJSON()</td></tr><tr><td>JSON 解析</td><td>croupier_client.cpp</td><td>67-85</td><td>ParseJSON()</td></tr><tr><td>JSON 序列化</td><td>croupier_client.cpp</td><td>87-98</td><td>ToJSON()</td></tr><tr><td>对象验证</td><td>croupier_client.cpp</td><td>677-718</td><td>ValidateObjectDescriptor()</td></tr><tr><td>组件验证</td><td>croupier_client.cpp</td><td>721-750</td><td>ValidateComponentDescriptor()</td></tr><tr><td>模板生成</td><td>croupier_client.cpp</td><td>753-793</td><td>GenerateXxxTemplate()</td></tr></tbody></table><hr><h2 id="📚-文档导航" tabindex="-1"><a class="header-anchor" href="#📚-文档导航"><span>📚 文档导航</span></a></h2><h3 id="用户文档" tabindex="-1"><a class="header-anchor" href="#用户文档"><span>用户文档</span></a></h3><ul><li><strong>README.md</strong>: 快速开始、示例、API 参考</li><li><strong>VIRTUAL_OBJECT_REGISTRATION.md</strong>: 架构设计、四层模型、DDD 模式</li></ul><h3 id="技术分析" tabindex="-1"><a class="header-anchor" href="#技术分析"><span>技术分析</span></a></h3><ul><li><strong>CPP_SDK_DEEP_ANALYSIS.md</strong>: 深度分析（新）</li><li><strong>CPP_SDK_QUICK_REFERENCE.md</strong>: 快速参考表（新）</li><li><strong>CPP_SDK_DIRECTORY_INDEX.md</strong>: 本文档（目录索引）</li></ul><h3 id="示例代码" tabindex="-1"><a class="header-anchor" href="#示例代码"><span>示例代码</span></a></h3><ul><li><strong>virtual_object_demo.cpp</strong>: 6 个完整演示</li></ul><h3 id="配置" tabindex="-1"><a class="header-anchor" href="#配置"><span>配置</span></a></h3><ul><li><strong>CMakeLists.txt</strong>: 构建系统</li><li><strong>vcpkg.json</strong>: 依赖管理</li><li><strong>.github/workflows/cpp-sdk-build.yml</strong>: CI/CD 自动化</li></ul><hr><h2 id="🔍-快速查询表" tabindex="-1"><a class="header-anchor" href="#🔍-快速查询表"><span>🔍 快速查询表</span></a></h2><h3 id="我要" tabindex="-1"><a class="header-anchor" href="#我要"><span>&quot;我要...&quot;</span></a></h3><table><thead><tr><th>需求</th><th>查看</th><th>关键行号</th></tr></thead><tbody><tr><td>注册一个函数</td><td>croupier_client.h</td><td>136</td></tr><tr><td>注册虚拟对象</td><td>croupier_client.h</td><td>141-144</td></tr><tr><td>注册完整组件</td><td>croupier_client.h</td><td>147</td></tr><tr><td>配置 game_id</td><td>croupier_client.h</td><td>65-66</td></tr><tr><td>配置生产环境</td><td>croupier_client.h</td><td>70-74</td></tr><tr><td>实现 handler</td><td>virtual_object_demo.cpp</td><td>8-93</td></tr><tr><td>验证对象</td><td>croupier_client.cpp</td><td>677-718</td></tr><tr><td>生成模板</td><td>croupier_client.cpp</td><td>753-793</td></tr><tr><td>连接 Agent</td><td>croupier_client.cpp</td><td>317-337</td></tr><tr><td>启动服务</td><td>croupier_client.cpp</td><td>339-353</td></tr></tbody></table>`,49)])])}const d=s(i,[["render",t]]),r=JSON.parse('{"path":"/CPP_SDK_DIRECTORY_INDEX.html","title":"Croupier C++ SDK 目录索引与文件映射","lang":"zh-CN","frontmatter":{},"git":{"updatedTime":1763084302000,"contributors":[{"name":"cuihairu","username":"cuihairu","email":"chuihairu@gmail.com","commits":1,"url":"https://github.com/cuihairu"}],"changelog":[{"hash":"d015f85af868febe5fc90a28c7fbab7bd4691325","time":1763084302000,"email":"chuihairu@gmail.com","author":"cuihairu","message":"feat: update all SDK submodules to proto-aligned versions and add comprehensive documentation"}]},"filePathRelative":"CPP_SDK_DIRECTORY_INDEX.md"}');export{d as comp,r as data};
