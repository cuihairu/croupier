<template><div><h1 id="croupier-c-sdk-虚拟对象注册系统" tabindex="-1"><a class="header-anchor" href="#croupier-c-sdk-虚拟对象注册系统"><span>Croupier C++ SDK：虚拟对象注册系统</span></a></h1>
<p><a href="https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/cpp-sdk-build.yml" target="_blank" rel="noopener noreferrer"><img src="https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/cpp-sdk-build.yml/badge.svg" alt="Build Status"></a>
<a href="https://github.com/cuihairu/croupier-sdk-cpp/blob/main/LICENSE" target="_blank" rel="noopener noreferrer"><img src="https://img.shields.io/badge/License-Apache 2.0-blue.svg" alt="License: Apache-2.0"></a>
<a href="https://en.cppreference.com/w/cpp/17" target="_blank" rel="noopener noreferrer"><img src="https://img.shields.io/badge/C%2B%2B-17-blue.svg" alt="C++17"></a></p>
<p>高性能的C++ SDK，支持虚拟对象注册、gRPC通信、多平台构建。采用<strong>ID引用模式</strong>解决对象参数传递性能问题，通过<strong>四层组件化架构</strong>实现优雅的函数和对象管理。</p>
<h2 id="🎯-核心特性" tabindex="-1"><a class="header-anchor" href="#🎯-核心特性"><span>🎯 核心特性</span></a></h2>
<h3 id="✅-已实现功能" tabindex="-1"><a class="header-anchor" href="#✅-已实现功能"><span>✅ <strong>已实现功能</strong></span></a></h3>
<ul>
<li>🏗️ <strong>虚拟对象注册系统</strong> - 完整的对象和函数组注册机制</li>
<li>📡 <strong>gRPC集成</strong> - 真实的protobuf+gRPC通信（通过vcpkg）</li>
<li>🔧 <strong>多平台支持</strong> - Windows、Linux、macOS（x64/ARM64）</li>
<li>📦 <strong>vcpkg包管理</strong> - 自动依赖管理和跨平台构建</li>
<li>🚀 <strong>高性能设计</strong> - ID引用模式，无状态函数，轻量参数</li>
<li>🛡️ <strong>类型安全</strong> - JSON Schema验证，编译时类型检查</li>
<li>🔄 <strong>自动构建</strong> - GitHub Actions每日构建发布</li>
</ul>
<h3 id="🔧-架构亮点" tabindex="-1"><a class="header-anchor" href="#🔧-架构亮点"><span>🔧 <strong>架构亮点</strong></span></a></h3>
<ul>
<li><strong>四层抽象模型</strong>：Function → Entity → Resource → Component</li>
<li><strong>ID引用模式</strong>：只传递ID字符串，避免对象实例序列化</li>
<li><strong>声明式配置</strong>：JSON驱动，配置与代码分离</li>
<li><strong>多环境支持</strong>：game_id + env 后台环境隔离</li>
</ul>
<h2 id="📋-快速开始" tabindex="-1"><a class="header-anchor" href="#📋-快速开始"><span>📋 快速开始</span></a></h2>
<h3 id="系统要求" tabindex="-1"><a class="header-anchor" href="#系统要求"><span>系统要求</span></a></h3>
<p><strong>基础要求：</strong></p>
<ul>
<li><strong>C++17</strong> 编译器（GCC 8+, Clang 10+, MSVC 2019+）</li>
<li><strong>CMake 3.20+</strong></li>
<li><strong>vcpkg</strong>（推荐，用于依赖管理）</li>
</ul>
<p><strong>依赖库（自动安装）：</strong></p>
<ul>
<li>gRPC (通过vcpkg)</li>
<li>Protobuf (通过vcpkg)</li>
<li>nlohmann/json (通过vcpkg)</li>
</ul>
<h3 id="🚀-一键构建" tabindex="-1"><a class="header-anchor" href="#🚀-一键构建"><span>🚀 一键构建</span></a></h3>
<h4 id="方式1-使用构建脚本-推荐" tabindex="-1"><a class="header-anchor" href="#方式1-使用构建脚本-推荐"><span><strong>方式1：使用构建脚本（推荐）</strong></span></a></h4>
<p><strong>Linux/macOS:</strong></p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 基础构建</span></span>
<span class="line">./scripts/build.sh</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 清理重构建</span></span>
<span class="line">./scripts/build.sh <span class="token parameter variable">--clean</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启用测试</span></span>
<span class="line">./scripts/build.sh <span class="token parameter variable">--tests</span> ON</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 交叉编译 ARM64（Linux）</span></span>
<span class="line">./scripts/build.sh <span class="token parameter variable">--arch</span> arm64</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 自定义 vcpkg 路径</span></span>
<span class="line">./scripts/build.sh --vcpkg-root /path/to/vcpkg</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>Windows:</strong></p>
<div class="language-powershell line-numbers-mode" data-highlighter="prismjs" data-ext="powershell"><pre v-pre><code class="language-powershell"><span class="line"><span class="token comment"># 基础构建</span></span>
<span class="line"><span class="token punctuation">.</span>\scripts\build<span class="token punctuation">.</span>ps1</span>
<span class="line"></span>
<span class="line"><span class="token comment"># x86 构建</span></span>
<span class="line"><span class="token punctuation">.</span>\scripts\build<span class="token punctuation">.</span>ps1 <span class="token operator">-</span>Platform x86</span>
<span class="line"></span>
<span class="line"><span class="token comment"># Debug 构建</span></span>
<span class="line"><span class="token punctuation">.</span>\scripts\build<span class="token punctuation">.</span>ps1 <span class="token operator">-</span>BuildType Debug</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启用测试</span></span>
<span class="line"><span class="token punctuation">.</span>\scripts\build<span class="token punctuation">.</span>ps1 <span class="token operator">-</span>Tests ON</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="方式2-手动cmake构建" tabindex="-1"><a class="header-anchor" href="#方式2-手动cmake构建"><span><strong>方式2：手动CMake构建</strong></span></a></h4>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 设置 vcpkg（如果未安装）</span></span>
<span class="line"><span class="token function">git</span> clone https://github.com/Microsoft/vcpkg.git</span>
<span class="line"><span class="token builtin class-name">cd</span> <span class="token function">vcpkg</span></span>
<span class="line">./bootstrap-vcpkg.sh  <span class="token comment"># Windows: .\bootstrap-vcpkg.bat</span></span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">VCPKG_ROOT</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">pwd</span><span class="token variable">)</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 配置构建</span></span>
<span class="line">cmake <span class="token parameter variable">-B</span> build <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-DCMAKE_TOOLCHAIN_FILE</span><span class="token operator">=</span><span class="token variable">$VCPKG_ROOT</span>/scripts/buildsystems/vcpkg.cmake <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-DCMAKE_BUILD_TYPE</span><span class="token operator">=</span>Release <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-DENABLE_GRPC</span><span class="token operator">=</span>ON <span class="token punctuation">\</span></span>
<span class="line">  <span class="token parameter variable">-DBUILD_EXAMPLES</span><span class="token operator">=</span>ON</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 构建</span></span>
<span class="line">cmake <span class="token parameter variable">--build</span> build <span class="token parameter variable">--parallel</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 运行示例</span></span>
<span class="line">./build/bin/virtual-object-demo</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="💡-使用示例" tabindex="-1"><a class="header-anchor" href="#💡-使用示例"><span>💡 使用示例</span></a></h2>
<h3 id="示例1-基础函数注册" tabindex="-1"><a class="header-anchor" href="#示例1-基础函数注册"><span>示例1：基础函数注册</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 简单函数处理器</span></span>
<span class="line">std<span class="token double-colon punctuation">::</span>string <span class="token function">TransferHandler</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> context<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span> data <span class="token operator">=</span> utils<span class="token double-colon punctuation">::</span><span class="token function">ParseJSON</span><span class="token punctuation">(</span>payload<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 通过ID获取对象（避免传递重对象）</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string from_player <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"from_player_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string to_player <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"to_player_id"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string amount <span class="token operator">=</span> data<span class="token punctuation">[</span><span class="token string">"amount"</span><span class="token punctuation">]</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 执行业务逻辑</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token function">ExecuteTransfer</span><span class="token punctuation">(</span>from_player<span class="token punctuation">,</span> to_player<span class="token punctuation">,</span> amount<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"mmorpg-demo"</span><span class="token punctuation">;</span>      <span class="token comment">// 🎮 游戏标识</span></span>
<span class="line">    config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"development"</span><span class="token punctuation">;</span>          <span class="token comment">// 🔧 环境隔离</span></span>
<span class="line">    config<span class="token punctuation">.</span>agent_addr <span class="token operator">=</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册函数</span></span>
<span class="line">    FunctionDescriptor desc<span class="token punctuation">{</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span> <span class="token string">"1.0.0"</span><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>desc<span class="token punctuation">,</span> TransferHandler<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span>  <span class="token comment">// 阻塞服务</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="示例2-虚拟对象注册-推荐" tabindex="-1"><a class="header-anchor" href="#示例2-虚拟对象注册-推荐"><span>示例2：虚拟对象注册（推荐）</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"><span class="token keyword">using</span> <span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"economy-system"</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"production"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 定义钱包实体 ==========</span></span>
<span class="line">    VirtualObjectDescriptor wallet_entity<span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"wallet.entity"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>name <span class="token operator">=</span> <span class="token string">"钱包实体"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>description <span class="token operator">=</span> <span class="token string">"玩家钱包管理虚拟对象"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 定义 Schema</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>schema<span class="token punctuation">[</span><span class="token string">"type"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"object"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>schema<span class="token punctuation">[</span><span class="token string">"properties"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token raw-string string">R"({</span>
<span class="line">        "wallet_id": {"type": "string"},</span>
<span class="line">        "player_id": {"type": "string"},</span>
<span class="line">        "balance": {"type": "string", "pattern": "^[0-9]+\\.?[0-9]*$"}</span>
<span class="line">    })"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 定义操作映射（核心设计！）</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"read"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.get"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.transfer"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>operations<span class="token punctuation">[</span><span class="token string">"deposit"</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token string">"wallet.deposit"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 定义关系</span></span>
<span class="line">    RelationshipDef currency_rel<span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>type <span class="token operator">=</span> <span class="token string">"many-to-one"</span><span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>entity <span class="token operator">=</span> <span class="token string">"currency"</span><span class="token punctuation">;</span></span>
<span class="line">    currency_rel<span class="token punctuation">.</span>foreign_key <span class="token operator">=</span> <span class="token string">"currency_id"</span><span class="token punctuation">;</span></span>
<span class="line">    wallet_entity<span class="token punctuation">.</span>relationships<span class="token punctuation">[</span><span class="token string">"currency"</span><span class="token punctuation">]</span> <span class="token operator">=</span> currency_rel<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 准备函数处理器 ==========</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span> handlers<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.get"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletGetHandler<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletTransferHandler<span class="token punctuation">;</span></span>
<span class="line">    handlers<span class="token punctuation">[</span><span class="token string">"wallet.deposit"</span><span class="token punctuation">]</span> <span class="token operator">=</span> WalletDepositHandler<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 一键注册整个对象 ==========</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>client<span class="token punctuation">.</span><span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span>wallet_entity<span class="token punctuation">,</span> handlers<span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>cout <span class="token operator">&lt;&lt;</span> <span class="token string">"✅ 成功注册虚拟对象: "</span> <span class="token operator">&lt;&lt;</span> wallet_entity<span class="token punctuation">.</span>id <span class="token operator">&lt;&lt;</span> std<span class="token double-colon punctuation">::</span>endl<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="示例3-组件级注册-最优雅" tabindex="-1"><a class="header-anchor" href="#示例3-组件级注册-最优雅"><span>示例3：组件级注册（最优雅）</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// ========== 经济系统组件注册 ==========</span></span>
<span class="line">ComponentDescriptor economy_system<span class="token punctuation">;</span></span>
<span class="line">economy_system<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token string">"economy-system"</span><span class="token punctuation">;</span></span>
<span class="line">economy_system<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">;</span></span>
<span class="line">economy_system<span class="token punctuation">.</span>name <span class="token operator">=</span> <span class="token string">"完整经济系统"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 添加多个实体</span></span>
<span class="line">economy_system<span class="token punctuation">.</span>entities<span class="token punctuation">.</span><span class="token function">push_back</span><span class="token punctuation">(</span>wallet_entity<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">economy_system<span class="token punctuation">.</span>entities<span class="token punctuation">.</span><span class="token function">push_back</span><span class="token punctuation">(</span>currency_entity<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">economy_system<span class="token punctuation">.</span>entities<span class="token punctuation">.</span><span class="token function">push_back</span><span class="token punctuation">(</span>market_entity<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 添加跨实体函数</span></span>
<span class="line">FunctionDescriptor market_trade<span class="token punctuation">{</span><span class="token string">"market.trade"</span><span class="token punctuation">,</span> <span class="token string">"1.0.0"</span><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line">economy_system<span class="token punctuation">.</span>functions<span class="token punctuation">.</span><span class="token function">push_back</span><span class="token punctuation">(</span>market_trade<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ========== 一键注册整个组件 ==========</span></span>
<span class="line">client<span class="token punctuation">.</span><span class="token function">RegisterComponent</span><span class="token punctuation">(</span>economy_system<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="示例4-配置文件驱动-生产推荐" tabindex="-1"><a class="header-anchor" href="#示例4-配置文件驱动-生产推荐"><span>示例4：配置文件驱动（生产推荐）</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">int</span> <span class="token function">main</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 🎯 一行代码注册整个系统！</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token string">"economy-system.json"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>economy-system.json:</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"economy-system"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"version"</span><span class="token operator">:</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"name"</span><span class="token operator">:</span> <span class="token string">"经济系统组件"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"entities"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      <span class="token property">"id"</span><span class="token operator">:</span> <span class="token string">"wallet.entity"</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"operations"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"read"</span><span class="token operator">:</span> <span class="token string">"wallet.get"</span><span class="token punctuation">,</span></span>
<span class="line">        <span class="token property">"transfer"</span><span class="token operator">:</span> <span class="token string">"wallet.transfer"</span></span>
<span class="line">      <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">      <span class="token property">"relationships"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token property">"currency"</span><span class="token operator">:</span> <span class="token punctuation">{</span></span>
<span class="line">          <span class="token property">"type"</span><span class="token operator">:</span> <span class="token string">"many-to-one"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"entity"</span><span class="token operator">:</span> <span class="token string">"currency"</span><span class="token punctuation">,</span></span>
<span class="line">          <span class="token property">"foreign_key"</span><span class="token operator">:</span> <span class="token string">"currency_id"</span></span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">      <span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">  <span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🏗️-架构设计" tabindex="-1"><a class="header-anchor" href="#🏗️-架构设计"><span>🏗️ 架构设计</span></a></h2>
<h3 id="四层组件化架构" tabindex="-1"><a class="header-anchor" href="#四层组件化架构"><span>四层组件化架构</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">Function Level    ← wallet.transfer (具体函数实现)</span>
<span class="line">     ↓</span>
<span class="line">Entity Level      ← wallet.entity (业务对象模型)</span>
<span class="line">     ↓</span>
<span class="line">Resource Level    ← 钱包管理面板 (UI资源组织)</span>
<span class="line">     ↓</span>
<span class="line">Component Level   ← economy-system (可分发模块)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="id引用模式的优势" tabindex="-1"><a class="header-anchor" href="#id引用模式的优势"><span>ID引用模式的优势</span></a></h3>
<p><strong>❌ 避免笨重的对象参数：</strong></p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 传统方式：传递整个对象实例</span></span>
<span class="line"><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span> <span class="token punctuation">{</span>object<span class="token operator">:</span> wallet_instance<span class="token punctuation">,</span> params<span class="token operator">:</span> <span class="token punctuation">{</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">.</span><span class="token punctuation">}</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>✅ 优雅的ID引用设计：</strong></p>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// Croupier方式：只传递ID，后端通过Repository获取对象</span></span>
<span class="line"><span class="token function">invoke</span><span class="token punctuation">(</span><span class="token string">"wallet.transfer"</span><span class="token punctuation">,</span> <span class="token punctuation">{</span></span>
<span class="line">  from_player_id<span class="token operator">:</span> <span class="token string">"player123"</span><span class="token punctuation">,</span>  <span class="token comment">// 轻量级ID引用</span></span>
<span class="line">  to_player_id<span class="token operator">:</span> <span class="token string">"player456"</span><span class="token punctuation">,</span></span>
<span class="line">  currency_code<span class="token operator">:</span> <span class="token string">"gold"</span><span class="token punctuation">,</span></span>
<span class="line">  amount<span class="token operator">:</span> <span class="token string">"100.0"</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>核心优势：</strong></p>
<ul>
<li>🚀 <strong>极致性能</strong>：网络传输只有几十字节</li>
<li>🛡️ <strong>线程安全</strong>：无状态函数，天然支持并发</li>
<li>🔄 <strong>水平扩展</strong>：函数可以部署到任意节点</li>
<li>🧩 <strong>松耦合</strong>：对象管理与业务逻辑完全分离</li>
</ul>
<h2 id="🔧-api参考" tabindex="-1"><a class="header-anchor" href="#🔧-api参考"><span>🔧 API参考</span></a></h2>
<h3 id="核心数据结构" tabindex="-1"><a class="header-anchor" href="#核心数据结构"><span>核心数据结构</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 虚拟对象描述符</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">VirtualObjectDescriptor</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string id<span class="token punctuation">;</span>                              <span class="token comment">// 对象标识</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string version<span class="token punctuation">;</span>                         <span class="token comment">// 版本号</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string name<span class="token punctuation">;</span>                            <span class="token comment">// 显示名称</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string description<span class="token punctuation">;</span>                     <span class="token comment">// 描述</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> schema<span class="token punctuation">;</span>   <span class="token comment">// JSON Schema</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">></span> operations<span class="token punctuation">;</span> <span class="token comment">// 操作映射</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> RelationshipDef<span class="token operator">></span> relationships<span class="token punctuation">;</span> <span class="token comment">// 关系定义</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 客户端配置</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">ClientConfig</span> <span class="token punctuation">{</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string agent_addr <span class="token operator">=</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string game_id<span class="token punctuation">;</span>           <span class="token comment">// 🎮 必需：游戏标识符</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string env <span class="token operator">=</span> <span class="token string">"development"</span><span class="token punctuation">;</span> <span class="token comment">// 🔧 环境：development/staging/production</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string service_id <span class="token operator">=</span> <span class="token string">"cpp-service"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// TLS配置</span></span>
<span class="line">    <span class="token keyword">bool</span> insecure <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span>          <span class="token comment">// 开发环境：true，生产环境：false</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string cert_file<span class="token punctuation">;</span>         <span class="token comment">// 客户端证书</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string key_file<span class="token punctuation">;</span>          <span class="token comment">// 私钥</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string ca_file<span class="token punctuation">;</span>           <span class="token comment">// CA证书</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="主要api" tabindex="-1"><a class="header-anchor" href="#主要api"><span>主要API</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">class</span> <span class="token class-name">CroupierClient</span> <span class="token punctuation">{</span></span>
<span class="line"><span class="token keyword">public</span><span class="token operator">:</span></span>
<span class="line">    <span class="token comment">// ========== 基础函数注册 ==========</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">RegisterFunction</span><span class="token punctuation">(</span><span class="token keyword">const</span> FunctionDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">,</span> FunctionHandler handler<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 虚拟对象注册（推荐） ==========</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span></span>
<span class="line">        <span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">,</span></span>
<span class="line">        <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>map<span class="token operator">&lt;</span>std<span class="token double-colon punctuation">::</span>string<span class="token punctuation">,</span> FunctionHandler<span class="token operator">></span><span class="token operator">&amp;</span> handlers</span>
<span class="line">    <span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 组件级注册（最优雅） ==========</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">RegisterComponent</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span> comp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">LoadComponentFromFile</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> config_file<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 管理接口 ==========</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>VirtualObjectDescriptor<span class="token operator">></span> <span class="token function">GetRegisteredObjects</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">const</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>vector<span class="token operator">&lt;</span>ComponentDescriptor<span class="token operator">></span> <span class="token function">GetRegisteredComponents</span><span class="token punctuation">(</span><span class="token punctuation">)</span> <span class="token keyword">const</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">UnregisterVirtualObject</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> object_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">UnregisterComponent</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> component_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// ========== 核心操作 ==========</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span>    <span class="token comment">// 阻塞服务</span></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">void</span> <span class="token function">Close</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="工具函数" tabindex="-1"><a class="header-anchor" href="#工具函数"><span>工具函数</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">namespace</span> croupier<span class="token double-colon punctuation">::</span>sdk<span class="token double-colon punctuation">::</span>utils <span class="token punctuation">{</span></span>
<span class="line">    <span class="token comment">// 验证</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">ValidateObjectDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">bool</span> <span class="token function">ValidateComponentDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span> comp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 配置文件</span></span>
<span class="line">    VirtualObjectDescriptor <span class="token function">LoadObjectDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> file_path<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    ComponentDescriptor <span class="token function">LoadComponentDescriptor</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> file_path<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 模板生成</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string <span class="token function">GenerateObjectTemplate</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> object_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string <span class="token function">GenerateComponentTemplate</span><span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> component_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 序列化</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string <span class="token function">ObjectDescriptorToJSON</span><span class="token punctuation">(</span><span class="token keyword">const</span> VirtualObjectDescriptor<span class="token operator">&amp;</span> desc<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string <span class="token function">ComponentDescriptorToJSON</span><span class="token punctuation">(</span><span class="token keyword">const</span> ComponentDescriptor<span class="token operator">&amp;</span> comp<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🚀-部署和分发" tabindex="-1"><a class="header-anchor" href="#🚀-部署和分发"><span>🚀 部署和分发</span></a></h2>
<h3 id="github-actions自动构建" tabindex="-1"><a class="header-anchor" href="#github-actions自动构建"><span>GitHub Actions自动构建</span></a></h3>
<p>每日自动构建，支持多平台：</p>
<ul>
<li>✅ <strong>Windows</strong> (x64, x86)</li>
<li>✅ <strong>Linux</strong> (x64, ARM64)</li>
<li>✅ <strong>macOS</strong> (x64, ARM64 Apple Silicon)</li>
</ul>
<p><strong>构建产物：</strong></p>
<ul>
<li>静态库 (.a/.lib)</li>
<li>动态库 (.so/.dylib/.dll)</li>
<li>头文件包</li>
<li>示例程序</li>
<li>CMake配置文件</li>
</ul>
<h3 id="手动触发构建" tabindex="-1"><a class="header-anchor" href="#手动触发构建"><span>手动触发构建</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 在GitHub仓库页面</span></span>
<span class="line">Actions → C++ SDK Build <span class="token operator">&amp;</span> Release → Run workflow</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 选择构建类型</span></span>
<span class="line">- nightly: 每日构建</span>
<span class="line">- release: 正式发布</span>
<span class="line">- patch: 补丁版本</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="下载预构建包" tabindex="-1"><a class="header-anchor" href="#下载预构建包"><span>下载预构建包</span></a></h3>
<p>访问 <a href="https://github.com/cuihairu/croupier/releases" target="_blank" rel="noopener noreferrer">Releases页面</a> 下载最新构建：</p>
<ul>
<li><code v-pre>croupier-cpp-sdk-{version}-windows-x64.zip</code></li>
<li><code v-pre>croupier-cpp-sdk-{version}-linux-x64.tar.gz</code></li>
<li><code v-pre>croupier-cpp-sdk-{version}-macos-arm64.tar.gz</code></li>
</ul>
<h2 id="📦-在项目中使用" tabindex="-1"><a class="header-anchor" href="#📦-在项目中使用"><span>📦 在项目中使用</span></a></h2>
<h3 id="方式1-cmake-find-package" tabindex="-1"><a class="header-anchor" href="#方式1-cmake-find-package"><span>方式1：CMake find_package</span></a></h3>
<div class="language-cmake line-numbers-mode" data-highlighter="prismjs" data-ext="cmake"><pre v-pre><code class="language-cmake"><span class="line"><span class="token keyword">find_package</span><span class="token punctuation">(</span>croupier-sdk REQUIRED<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">add_executable</span><span class="token punctuation">(</span>my_game src/main.cpp<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">target_link_libraries</span><span class="token punctuation">(</span>my_game <span class="token inserted class-name">croupier::sdk</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="方式2-vcpkg集成" tabindex="-1"><a class="header-anchor" href="#方式2-vcpkg集成"><span>方式2：vcpkg集成</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 安装 SDK（未来支持）</span></span>
<span class="line"><span class="token function">vcpkg</span> <span class="token function">install</span> croupier-cpp-sdk</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-cmake line-numbers-mode" data-highlighter="prismjs" data-ext="cmake"><pre v-pre><code class="language-cmake"><span class="line"><span class="token keyword">find_package</span><span class="token punctuation">(</span>croupier-cpp-sdk CONFIG REQUIRED<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">target_link_libraries</span><span class="token punctuation">(</span>my_game <span class="token inserted class-name">croupier::sdk</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="方式3-git子模块" tabindex="-1"><a class="header-anchor" href="#方式3-git子模块"><span>方式3：Git子模块</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token function">git</span> submodule <span class="token function">add</span> https://github.com/cuihairu/croupier.git</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><div class="language-cmake line-numbers-mode" data-highlighter="prismjs" data-ext="cmake"><pre v-pre><code class="language-cmake"><span class="line"><span class="token keyword">add_subdirectory</span><span class="token punctuation">(</span>croupier/sdks/cpp<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">target_link_libraries</span><span class="token punctuation">(</span>my_game <span class="token inserted class-name">croupier::sdk</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔧-开发环境搭建" tabindex="-1"><a class="header-anchor" href="#🔧-开发环境搭建"><span>🔧 开发环境搭建</span></a></h2>
<h3 id="本地开发" tabindex="-1"><a class="header-anchor" href="#本地开发"><span>本地开发</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 克隆仓库</span></span>
<span class="line"><span class="token function">git</span> clone <span class="token parameter variable">--recursive</span> https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier/sdks/cpp</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 安装 vcpkg（如果没有）</span></span>
<span class="line"><span class="token function">git</span> clone https://github.com/Microsoft/vcpkg.git</span>
<span class="line"><span class="token builtin class-name">cd</span> <span class="token function">vcpkg</span> <span class="token operator">&amp;&amp;</span> ./bootstrap-vcpkg.sh</span>
<span class="line"><span class="token builtin class-name">export</span> <span class="token assign-left variable">VCPKG_ROOT</span><span class="token operator">=</span><span class="token variable"><span class="token variable">$(</span><span class="token builtin class-name">pwd</span><span class="token variable">)</span></span></span>
<span class="line"><span class="token builtin class-name">cd</span> <span class="token punctuation">..</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 构建</span></span>
<span class="line">./scripts/build.sh <span class="token parameter variable">--examples</span> ON <span class="token parameter variable">--tests</span> ON</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 运行示例</span></span>
<span class="line">./build/bin/virtual-object-demo</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="ide集成" tabindex="-1"><a class="header-anchor" href="#ide集成"><span>IDE集成</span></a></h3>
<p><strong>Visual Studio Code:</strong></p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">{</span></span>
<span class="line">  <span class="token property">"cmake.configureArgs"</span><span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token string">"-DCMAKE_TOOLCHAIN_FILE=${env:VCPKG_ROOT}/scripts/buildsystems/vcpkg.cmake"</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token property">"cmake.buildArgs"</span><span class="token operator">:</span> <span class="token punctuation">[</span><span class="token string">"--parallel"</span><span class="token punctuation">]</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>CLion:</strong></p>
<ul>
<li>Settings → Build → CMake</li>
<li>CMake options: <code v-pre>-DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake</code></li>
</ul>
<h2 id="📚-进阶主题" tabindex="-1"><a class="header-anchor" href="#📚-进阶主题"><span>📚 进阶主题</span></a></h2>
<h3 id="生产环境配置" tabindex="-1"><a class="header-anchor" href="#生产环境配置"><span>生产环境配置</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line">ClientConfig production_config<span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token string">"my-production-game"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token string">"production"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>agent_addr <span class="token operator">=</span> <span class="token string">"croupier-agent.internal:19090"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 🛡️ 启用安全配置</span></span>
<span class="line">production_config<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>cert_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/client.crt"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>key_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/client.key"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>ca_file <span class="token operator">=</span> <span class="token string">"/etc/croupier/ca.crt"</span><span class="token punctuation">;</span></span>
<span class="line">production_config<span class="token punctuation">.</span>auth_token <span class="token operator">=</span> <span class="token string">"your-jwt-token"</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">CroupierClient <span class="token function">client</span><span class="token punctuation">(</span>production_config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="性能优化" tabindex="-1"><a class="header-anchor" href="#性能优化"><span>性能优化</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// 1. 连接池配置</span></span>
<span class="line">config<span class="token punctuation">.</span>timeout_seconds <span class="token operator">=</span> <span class="token number">30</span><span class="token punctuation">;</span></span>
<span class="line">config<span class="token punctuation">.</span>heartbeat_interval <span class="token operator">=</span> <span class="token number">60</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 2. 并发处理</span></span>
<span class="line"><span class="token comment">// SDK内部线程安全，支持多线程并发注册</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 3. 内存优化</span></span>
<span class="line"><span class="token comment">// 使用move语义避免拷贝</span></span>
<span class="line"><span class="token keyword">auto</span> handlers <span class="token operator">=</span> std<span class="token double-colon punctuation">::</span><span class="token function">move</span><span class="token punctuation">(</span>handler_map<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">client<span class="token punctuation">.</span><span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span>std<span class="token double-colon punctuation">::</span><span class="token function">move</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">)</span><span class="token punctuation">,</span> handlers<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="错误处理" tabindex="-1"><a class="header-anchor" href="#错误处理"><span>错误处理</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">RegisterVirtualObject</span><span class="token punctuation">(</span>desc<span class="token punctuation">,</span> handlers<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    client<span class="token punctuation">.</span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    spdlog<span class="token double-colon punctuation">::</span><span class="token function">error</span><span class="token punctuation">(</span><span class="token string">"SDK error: {}"</span><span class="token punctuation">,</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 实现重试逻辑</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>this_thread<span class="token double-colon punctuation">::</span><span class="token function">sleep_for</span><span class="token punctuation">(</span>std<span class="token double-colon punctuation">::</span>chrono<span class="token double-colon punctuation">::</span><span class="token function">seconds</span><span class="token punctuation">(</span><span class="token number">5</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token comment">// 重新连接...</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🧰-troubleshooting-faq" tabindex="-1"><a class="header-anchor" href="#🧰-troubleshooting-faq"><span>🧰 Troubleshooting / FAQ</span></a></h2>
<h3 id="常见构建问题" tabindex="-1"><a class="header-anchor" href="#常见构建问题"><span>常见构建问题</span></a></h3>
<h4 id="_1-vcpkg-依赖安装失败" tabindex="-1"><a class="header-anchor" href="#_1-vcpkg-依赖安装失败"><span>1) vcpkg / 依赖安装失败</span></a></h4>
<ul>
<li>确认子模块完整：<code v-pre>git submodule update --init --recursive</code></li>
<li>清理并重试（尤其是切换分支/升级依赖后）：删除 <code v-pre>build/</code> 目录重新配置</li>
<li>确认 vcpkg 可用：<code v-pre>vcpkg --version</code>（或先运行 bootstrap 脚本）</li>
</ul>
<h4 id="_2-cmake-找不到-grpc-protobuf-could-not-find" tabindex="-1"><a class="header-anchor" href="#_2-cmake-找不到-grpc-protobuf-could-not-find"><span>2) CMake 找不到 gRPC/Protobuf（<code v-pre>Could NOT find ...</code>）</span></a></h4>
<ul>
<li>典型原因是没有正确启用 vcpkg toolchain：
<ul>
<li><code v-pre>-DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake</code></li>
<li>确认 <code v-pre>VCPKG_ROOT</code> 指向你实际的 vcpkg 路径</li>
</ul>
</li>
</ul>
<h4 id="_3-windows-上链接错误-triplet-不一致" tabindex="-1"><a class="header-anchor" href="#_3-windows-上链接错误-triplet-不一致"><span>3) Windows 上链接错误 / triplet 不一致</span></a></h4>
<ul>
<li>确认 triplet 与编译器/运行库一致（例如 <code v-pre>x64-windows</code> vs <code v-pre>x64-windows-static</code>）</li>
<li>建议固定 triplet 并清理构建缓存后重试（删除 <code v-pre>build/</code>）</li>
</ul>
<h4 id="_4-macos-上-ld-library-not-found" tabindex="-1"><a class="header-anchor" href="#_4-macos-上-ld-library-not-found"><span>4) macOS 上 <code v-pre>ld: library not found ...</code></span></a></h4>
<ul>
<li>常见于依赖未安装完成或架构不匹配（x64/arm64 混用）</li>
<li>建议确认使用同一套工具链（同一架构的 CMake/Clang/vcpkg），然后清理 <code v-pre>build/</code> 重配</li>
</ul>
<h3 id="常见问题-faq" tabindex="-1"><a class="header-anchor" href="#常见问题-faq"><span>常见问题（FAQ）</span></a></h3>
<h4 id="q-我只想跑最小可用示例-应该从哪里开始" tabindex="-1"><a class="header-anchor" href="#q-我只想跑最小可用示例-应该从哪里开始"><span>Q: 我只想跑最小可用示例，应该从哪里开始？</span></a></h4>
<ul>
<li>先跑基础示例（<code v-pre>example.cpp</code> / <code v-pre>complete_example.cpp</code>），再切换到配置驱动示例（<code v-pre>config_example.cpp</code>）。</li>
</ul>
<h4 id="q-grpc-连接失败-连接被拒绝-握手失败-怎么排查" tabindex="-1"><a class="header-anchor" href="#q-grpc-连接失败-连接被拒绝-握手失败-怎么排查"><span>Q: gRPC 连接失败（连接被拒绝/握手失败）怎么排查？</span></a></h4>
<ul>
<li>确认目标地址/端口正确、Server/Agent 已启动且可达</li>
<li>如果启用了 TLS/mTLS，确认 CA/证书/私钥路径与 ServerName 配置一致</li>
<li>打开更详细日志（如果你的示例/集成支持日志级别配置）</li>
</ul>
<h2 id="🤝-贡献指南" tabindex="-1"><a class="header-anchor" href="#🤝-贡献指南"><span>🤝 贡献指南</span></a></h2>
<ol>
<li><strong>Fork</strong> 项目</li>
<li>创建特性分支：<code v-pre>git checkout -b feature/amazing-feature</code></li>
<li>提交更改：<code v-pre>git commit -m 'Add amazing feature'</code></li>
<li>推送分支：<code v-pre>git push origin feature/amazing-feature</code></li>
<li>创建 <strong>Pull Request</strong></li>
</ol>
<h3 id="开发规范" tabindex="-1"><a class="header-anchor" href="#开发规范"><span>开发规范</span></a></h3>
<ul>
<li>遵循 <strong>C++17</strong> 标准</li>
<li>使用 <strong>clang-format</strong> 格式化代码</li>
<li>编写单元测试</li>
<li>更新相关文档</li>
</ul>
<h2 id="📄-license" tabindex="-1"><a class="header-anchor" href="#📄-license"><span>📄 License</span></a></h2>
<p>本项目采用 <a href="https://github.com/cuihairu/croupier-sdk-cpp/blob/main/LICENSE" target="_blank" rel="noopener noreferrer">Apache License 2.0</a> 开源协议。</p>
<h2 id="🔗-相关链接" tabindex="-1"><a class="header-anchor" href="#🔗-相关链接"><span>🔗 相关链接</span></a></h2>
<ul>
<li>📖 <strong>主项目</strong>: <a href="https://github.com/cuihairu/croupier" target="_blank" rel="noopener noreferrer">cuihairu/croupier</a></li>
<li>🏗️ <strong>架构文档</strong>: <RouteLink to="/sdks/cpp/VIRTUAL_OBJECT_REGISTRATION.html">虚拟对象设计</RouteLink></li>
<li>🐛 <strong>问题报告</strong>: <a href="https://github.com/cuihairu/croupier/issues" target="_blank" rel="noopener noreferrer">GitHub Issues</a></li>
<li>💬 <strong>讨论区</strong>: <a href="https://github.com/cuihairu/croupier/discussions" target="_blank" rel="noopener noreferrer">GitHub Discussions</a></li>
</ul>
<hr>
<p><strong>通过这套架构，您可以优雅地管理复杂的游戏业务对象，同时保持高性能和良好的开发体验！</strong> 🎮✨</p>
</div></template>


