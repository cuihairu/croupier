<template><div><h1 id="croupier-lua-sdk-设计文档-集成到-croupier-sdk-cpp" tabindex="-1"><a class="header-anchor" href="#croupier-lua-sdk-设计文档-集成到-croupier-sdk-cpp"><span>Croupier Lua SDK 设计文档（集成到 croupier-sdk-cpp）</span></a></h1>
<h2 id="概述" tabindex="-1"><a class="header-anchor" href="#概述"><span>概述</span></a></h2>
<p>Croupier Lua SDK 通过在 <code v-pre>croupier-sdk-cpp</code> 中添加 <strong>Lua 绑定层</strong> 实现，复用 C++ SDK 的核心功能。Lua 绑定作为可选构建选项，启用后生成包含 Lua API 的共享库。</p>
<h2 id="架构设计" tabindex="-1"><a class="header-anchor" href="#架构设计"><span>架构设计</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">┌─────────────────────────────────────────────────────────────────────────────────┐</span>
<span class="line">│                              Skynet 服务节点                                     │</span>
<span class="line">├─────────────────────────────────────────────────────────────────────────────────┤</span>
<span class="line">│                                                                                 │</span>
<span class="line">│   ┌─────────────┐                                                               │</span>
<span class="line">│   │ Lua Service │  业务逻辑服务                                                  │</span>
<span class="line">│   └──────┬──────┘                                                               │</span>
<span class="line">│          │                                                                       │</span>
<span class="line">│          │ Lua API 调用                                                          │</span>
<span class="line">│          ▼                                                                       │</span>
<span class="line">│   ┌──────────────────────────────────────────────────────────────────────────┐  │</span>
<span class="line">│   │                       Lua SDK (croupier.lua)                               │  │</span>
<span class="line">│   │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐                 │  │</span>
<span class="line">│   │  │ Client Module │  │ Invoker Module│  │ Logger Module │                 │  │</span>
<span class="line">│   │  └───────────────┘  └───────────────┘  └───────────────┘                 │  │</span>
<span class="line">│   └──────────────────────────────────────────────────────────────────────────┘  │</span>
<span class="line">│                            │                                                     │</span>
<span class="line">│                            │ Lua C API                                          │</span>
<span class="line">│                            ▼                                                     │</span>
<span class="line">│   ┌──────────────────────────────────────────────────────────────────────────┐  │</span>
<span class="line">│   │               croupier-sdk-cpp (WITH LUA BINDING)                        │  │</span>
<span class="line">│   │  ┌────────────────────────────────────────────────────────────────────┐   │  │</span>
<span class="line">│   │  │                    Lua C API Binding                               │   │  │</span>
<span class="line">│   │  │  (src/bindings/lua_binding.cpp)                                   │   │  │</span>
<span class="line">│   │  └────────────────────────────────────────────────────────────────────┘   │  │</span>
<span class="line">│   │                              │                                          │   │  │</span>
<span class="line">│   │  ┌────────────────────────────────────────────────────────────────────┐   │  │</span>
<span class="line">│   │  │                    C++ SDK Core                                  │   │  │</span>
<span class="line">│   │  │  - CroupierClient                                              │   │  │</span>
<span class="line">│   │  │  - GrpcManager                                                 │   │  │</span>
<span class="line">│   │  │  - FunctionHandler                                             │   │  │</span>
<span class="line">│   │  └────────────────────────────────────────────────────────────────────┘   │  │</span>
<span class="line">│   │                              │                                          │   │  │</span>
<span class="line">│   │  ┌────────────────────────────────────────────────────────────────────┐   │  │</span>
<span class="line">│   │  │                    gRPC C++ Library                               │   │  │</span>
<span class="line">│   │  └────────────────────────────────────────────────────────────────────┘   │  │</span>
<span class="line">│   └──────────────────────────────────────────────────────────────────────────┘  │</span>
<span class="line">│                            │                                                     │</span>
<span class="line">│                            │ gRPC                                                │</span>
<span class="line">│                            ▼                                                     │</span>
<span class="line">│                    ┌─────────────┐                                              │</span>
<span class="line">│                    │ Croupier    │                                              │</span>
<span class="line">│                    │   Agent     │                                              │</span>
<span class="line">│                    └─────────────┘                                              │</span>
<span class="line">│                                                                                 │</span>
<span class="line">└─────────────────────────────────────────────────────────────────────────────────┘</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="设计优势" tabindex="-1"><a class="header-anchor" href="#设计优势"><span>设计优势</span></a></h2>
<table>
<thead>
<tr>
<th>方案</th>
<th>优势</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>集成到 croupier-sdk-cpp</strong></td>
<td>✅ 复用 C++ 核心，无重复代码</td>
</tr>
<tr>
<td><strong>CMake 可选构建</strong></td>
<td>✅ <code v-pre>ENABLE_LUA_BINDING=ON</code> 才编译 Lua 绑定</td>
</tr>
<tr>
<td><strong>统一版本管理</strong></td>
<td>✅ C++ SDK 更新时 Lua 同步更新</td>
</tr>
<tr>
<td><strong>共享库输出</strong></td>
<td>✅ <code v-pre>libcroupier-sdk-lua.so</code> 包含所有功能</td>
</tr>
<tr>
<td><strong>Skynet 开箱即用</strong></td>
<td>✅ 编译后直接复制到 Skynet 使用</td>
</tr>
</tbody>
</table>
<h2 id="croupier-sdk-cpp-集成方案" tabindex="-1"><a class="header-anchor" href="#croupier-sdk-cpp-集成方案"><span>croupier-sdk-cpp 集成方案</span></a></h2>
<h3 id="_1-目录结构-在-croupier-sdk-cpp-中" tabindex="-1"><a class="header-anchor" href="#_1-目录结构-在-croupier-sdk-cpp-中"><span>1. 目录结构（在 croupier-sdk-cpp 中）</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">croupier-sdk-cpp/</span>
<span class="line">├── CMakeLists.txt                    # 添加 ENABLE_LUA_BINDING 选项</span>
<span class="line">├── include/</span>
<span class="line">│   └── croupier/</span>
<span class="line">│       └── sdk/</span>
<span class="line">│           └── *.h                   # 现有的 C++ 头文件</span>
<span class="line">│</span>
<span class="line">├── src/</span>
<span class="line">│   ├── croupier_client.cpp           # 现有 C++ 实现</span>
<span class="line">│   ├── grpc_service.cpp</span>
<span class="line">│   └── bindings/                     # 新增：语言绑定</span>
<span class="line">│       ├── lua_binding.cpp           # Lua C API 绑定</span>
<span class="line">│       └── lua_binding.h</span>
<span class="line">│</span>
<span class="line">├── lua/                              # 新增：Lua SDK 模块</span>
<span class="line">│   ├── croupier/</span>
<span class="line">│   │   ├── init.lua</span>
<span class="line">│   │   ├── client.lua</span>
<span class="line">│   │   ├── invoker.lua</span>
<span class="line">│   │   └── utils.lua</span>
<span class="line">│   └── examples/</span>
<span class="line">│       └── skynet_service.lua</span>
<span class="line">│</span>
<span class="line">└── skynet/                           # 新增：Skynet 集成</span>
<span class="line">    ├── service/</span>
<span class="line">    │   └── croupier_service.lua</span>
<span class="line">    └── examples/</span>
<span class="line">        └── config/</span>
<span class="line">            └── croupier.conf</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_2-cmake-配置" tabindex="-1"><a class="header-anchor" href="#_2-cmake-配置"><span>2. CMake 配置</span></a></h3>
<div class="language-cmake line-numbers-mode" data-highlighter="prismjs" data-ext="cmake"><pre v-pre><code class="language-cmake"><span class="line"><span class="token comment"># croupier-sdk-cpp/CMakeLists.txt（添加部分）</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># ========== Lua Binding Option ==========</span></span>
<span class="line"><span class="token keyword">option</span><span class="token punctuation">(</span>ENABLE_LUA_BINDING <span class="token string">"Enable Lua language binding"</span> <span class="token boolean">OFF</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">if</span><span class="token punctuation">(</span>ENABLE_LUA_BINDING<span class="token punctuation">)</span></span>
<span class="line">    <span class="token comment"># 查找 Lua</span></span>
<span class="line">    <span class="token keyword">find_package</span><span class="token punctuation">(</span>LUA REQUIRED<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 检查 Lua 版本 (5.3+)</span></span>
<span class="line">    <span class="token keyword">if</span><span class="token punctuation">(</span><span class="token operator">NOT</span> <span class="token variable">LUA_VERSION_MAJOR</span> <span class="token operator">EQUAL</span> <span class="token number">5</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">message</span><span class="token punctuation">(</span>WARNING <span class="token string">"Lua 5.3+ is recommended, found <span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_VERSION_MAJOR</span><span class="token punctuation">}</span></span>.<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_VERSION_MINOR</span><span class="token punctuation">}</span></span>"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">endif</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">message</span><span class="token punctuation">(</span>STATUS <span class="token string">"Lua binding enabled"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">message</span><span class="token punctuation">(</span>STATUS <span class="token string">"  LUA_VERSION: <span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_VERSION_MAJOR</span><span class="token punctuation">}</span></span>.<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_VERSION_MINOR</span><span class="token punctuation">}</span></span>"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">message</span><span class="token punctuation">(</span>STATUS <span class="token string">"  LUA_INCLUDE_DIR: <span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_INCLUDE_DIR</span><span class="token punctuation">}</span></span>"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">message</span><span class="token punctuation">(</span>STATUS <span class="token string">"  LUA_LIBRARIES: <span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_LIBRARIES</span><span class="token punctuation">}</span></span>"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">endif</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_3-lua-绑定目标" tabindex="-1"><a class="header-anchor" href="#_3-lua-绑定目标"><span>3. Lua 绑定目标</span></a></h3>
<div class="language-cmake line-numbers-mode" data-highlighter="prismjs" data-ext="cmake"><pre v-pre><code class="language-cmake"><span class="line"><span class="token comment"># croupier-sdk-cpp/CMakeLists.txt（添加目标）</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># ========== Lua Binding Target ==========</span></span>
<span class="line"><span class="token keyword">if</span><span class="token punctuation">(</span>ENABLE_LUA_BINDING<span class="token punctuation">)</span></span>
<span class="line">    <span class="token comment"># Lua 绑定源文件</span></span>
<span class="line">    <span class="token keyword">set</span><span class="token punctuation">(</span>LUA_BINDING_SOURCES</span>
<span class="line">        src/bindings/lua_binding.cpp</span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 创建带 Lua 绑定的共享库</span></span>
<span class="line">    <span class="token keyword">add_library</span><span class="token punctuation">(</span>croupier-sdk-lua <span class="token namespace">SHARED</span></span>
<span class="line">        <span class="token punctuation">${</span>SDK_SOURCES<span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">${</span>LUA_BINDING_SOURCES<span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">${</span>GENERATED_PROTO_SOURCES<span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 设置输出名称</span></span>
<span class="line">    <span class="token keyword">set_target_properties</span><span class="token punctuation">(</span>croupier-sdk-lua <span class="token namespace">PROPERTIES</span></span>
<span class="line">        <span class="token property">OUTPUT_NAME</span> <span class="token string">"croupier"</span></span>
<span class="line">        <span class="token property">VERSION</span> <span class="token punctuation">${</span><span class="token variable">PROJECT_VERSION</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token property">SOVERSION</span> <span class="token punctuation">${</span><span class="token variable">PROJECT_VERSION_MAJOR</span><span class="token punctuation">}</span></span>
<span class="line">        <span class="token property">PREFIX</span> <span class="token string">"lib"</span>                    <span class="token comment"># Linux/macOS: libcroupier.so</span></span>
<span class="line">                                    <span class="token comment"># Windows: croupier.dll (由 PREFIX 处理)</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 包含目录</span></span>
<span class="line">    <span class="token keyword">target_include_directories</span><span class="token punctuation">(</span>croupier-sdk-lua <span class="token namespace">PRIVATE</span></span>
<span class="line">        <span class="token punctuation">${</span><span class="token variable">CMAKE_CURRENT_SOURCE_DIR</span><span class="token punctuation">}</span>/include</span>
<span class="line">        <span class="token punctuation">${</span>PROTO_GENERATED_DIR<span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">${</span>LUA_INCLUDE_DIR<span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 链接库</span></span>
<span class="line">    <span class="token keyword">target_link_libraries</span><span class="token punctuation">(</span>croupier-sdk-lua <span class="token namespace">PRIVATE</span></span>
<span class="line">        <span class="token inserted class-name">Threads::Threads</span></span>
<span class="line">        <span class="token punctuation">${</span>LUA_LIBRARIES<span class="token punctuation">}</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span><span class="token punctuation">(</span>ENABLE_GRPC<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">target_link_libraries</span><span class="token punctuation">(</span>croupier-sdk-lua <span class="token namespace">PRIVATE</span></span>
<span class="line">            <span class="token punctuation">${</span>GRPC_LIBRARIES<span class="token punctuation">}</span></span>
<span class="line">            <span class="token inserted class-name">ZLIB::ZLIB</span></span>
<span class="line">        <span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">target_compile_definitions</span><span class="token punctuation">(</span>croupier-sdk-lua</span>
<span class="line">            <span class="token namespace">PRIVATE</span></span>
<span class="line">                CROUPIER_SDK_ENABLE_GRPC</span>
<span class="line">        <span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">endif</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 导出符号（Lua 需要动态链接）</span></span>
<span class="line">    <span class="token keyword">if</span><span class="token punctuation">(</span><span class="token variable">WIN32</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token comment"># Windows: 导出所有符号</span></span>
<span class="line">        <span class="token keyword">set_target_properties</span><span class="token punctuation">(</span>croupier-sdk-lua <span class="token namespace">PROPERTIES</span></span>
<span class="line">            <span class="token property">WINDOWS_EXPORT_ALL_SYMBOLS</span> <span class="token boolean">ON</span></span>
<span class="line">        <span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">else</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token comment"># Linux/macOS: 导出符号</span></span>
<span class="line">        <span class="token keyword">target_link_options</span><span class="token punctuation">(</span>croupier-sdk-lua <span class="token namespace">PRIVATE</span></span>
<span class="line">            -Wl,--export-dynamic</span>
<span class="line">        <span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">endif</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 安装 Lua 模块</span></span>
<span class="line">    <span class="token keyword">install</span><span class="token punctuation">(</span>DIRECTORY <span class="token string">"<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">CMAKE_CURRENT_SOURCE_DIR</span><span class="token punctuation">}</span></span>/lua/"</span></span>
<span class="line">        DESTINATION <span class="token string">"<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">CMAKE_INSTALL_DATAROOTDIR</span><span class="token punctuation">}</span></span>/lua/<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_VERSION_MAJOR</span><span class="token punctuation">}</span></span>.<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">LUA_VERSION_MINOR</span><span class="token punctuation">}</span></span>"</span></span>
<span class="line">        FILES_MATCHING PATTERN <span class="token string">"*.lua"</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment"># 安装共享库</span></span>
<span class="line">    <span class="token keyword">install</span><span class="token punctuation">(</span>TARGETS croupier-sdk-lua</span>
<span class="line">        RUNTIME DESTINATION <span class="token string">"<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">CMAKE_INSTALL_BINDIR</span><span class="token punctuation">}</span></span>"</span></span>
<span class="line">        LIBRARY DESTINATION <span class="token string">"<span class="token interpolation"><span class="token punctuation">${</span><span class="token variable">CMAKE_INSTALL_LIBDIR</span><span class="token punctuation">}</span></span>"</span></span>
<span class="line">    <span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">message</span><span class="token punctuation">(</span>STATUS <span class="token string">"Lua binding target configured: croupier-sdk-lua"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">endif</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_4-lua-c-api-绑定实现" tabindex="-1"><a class="header-anchor" href="#_4-lua-c-api-绑定实现"><span>4. Lua C API 绑定实现</span></a></h3>
<div class="language-cpp line-numbers-mode" data-highlighter="prismjs" data-ext="cpp"><pre v-pre><code class="language-cpp"><span class="line"><span class="token comment">// croupier-sdk-cpp/src/bindings/lua_binding.cpp</span></span>
<span class="line"></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">"croupier/sdk/croupier_client.h"</span></span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;lua.hpp></span></span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;memory></span></span></span>
<span class="line"><span class="token macro property"><span class="token directive-hash">#</span><span class="token directive keyword">include</span> <span class="token string">&lt;string></span></span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">namespace</span> croupier <span class="token punctuation">{</span></span>
<span class="line"><span class="token keyword">namespace</span> lua <span class="token punctuation">{</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// Lua 用户数据结构</span></span>
<span class="line"><span class="token keyword">struct</span> <span class="token class-name">LuaClient</span> <span class="token punctuation">{</span></span>
<span class="line">    CroupierClient<span class="token operator">*</span> client<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">int</span> callback_ref<span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">bool</span> serving<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ===== 辅助函数 =====</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 获取 LuaClient 从栈</span></span>
<span class="line"><span class="token keyword">static</span> LuaClient<span class="token operator">*</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">void</span><span class="token operator">*</span> ud <span class="token operator">=</span> <span class="token function">luaL_checkudata</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"CroupierClient"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token function">luaL_argcheck</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> ud <span class="token operator">!=</span> <span class="token keyword">nullptr</span><span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"'CroupierClient' expected"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token generic-function"><span class="token function">static_cast</span><span class="token generic class-name"><span class="token operator">&lt;</span>LuaClient<span class="token operator">*</span><span class="token operator">></span></span></span><span class="token punctuation">(</span>ud<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 从表获取字符串字段</span></span>
<span class="line"><span class="token keyword">static</span> std<span class="token double-colon punctuation">::</span>string <span class="token function">get_table_string</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">,</span> <span class="token keyword">int</span> index<span class="token punctuation">,</span> <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> key<span class="token punctuation">,</span> <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> default_val <span class="token operator">=</span> <span class="token string">""</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">lua_getfield</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> index<span class="token punctuation">,</span> key<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token function">lua_isnil</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> default_val<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> value <span class="token operator">=</span> <span class="token function">lua_tostring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> value <span class="token operator">?</span> value <span class="token operator">:</span> <span class="token string">""</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> result<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 从表获取整数字段</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">get_table_int</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">,</span> <span class="token keyword">int</span> index<span class="token punctuation">,</span> <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> key<span class="token punctuation">,</span> <span class="token keyword">int</span> default_val <span class="token operator">=</span> <span class="token number">0</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">lua_getfield</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> index<span class="token punctuation">,</span> key<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token function">lua_isnil</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> default_val<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">int</span> value <span class="token operator">=</span> <span class="token punctuation">(</span><span class="token keyword">int</span><span class="token punctuation">)</span><span class="token function">lua_tointeger</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> value<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 从表获取布尔字段</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">bool</span> <span class="token function">get_table_bool</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">,</span> <span class="token keyword">int</span> index<span class="token punctuation">,</span> <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> key<span class="token punctuation">,</span> <span class="token keyword">bool</span> default_val <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">lua_getfield</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> index<span class="token punctuation">,</span> key<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token function">lua_isnil</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> default_val<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">bool</span> value <span class="token operator">=</span> <span class="token function">lua_toboolean</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span> <span class="token operator">!=</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> value<span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 推送结果到 Lua</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">push_result</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> result<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> result<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 推送错误到 Lua</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">push_error</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> error<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">lua_pushnil</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> error<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">2</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ===== C API 函数 =====</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 创建客户端</span></span>
<span class="line"><span class="token comment">// croupier.create_client(config_table)</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">create_client</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token function">luaL_checktype</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> LUA_TTABLE<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 从配置表读取参数</span></span>
<span class="line">    ClientConfig config<span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>agent_addr <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"agent_addr"</span><span class="token punctuation">,</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>service_id <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"service_id"</span><span class="token punctuation">,</span> <span class="token string">"lua-service"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>service_version <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"service_version"</span><span class="token punctuation">,</span> <span class="token string">"1.0.0"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"game_id"</span><span class="token punctuation">,</span> <span class="token string">"default-game"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"env"</span><span class="token punctuation">,</span> <span class="token string">"dev"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>local_addr <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"local_addr"</span><span class="token punctuation">,</span> <span class="token string">"0.0.0.0:0"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>insecure <span class="token operator">=</span> <span class="token function">get_table_bool</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"insecure"</span><span class="token punctuation">,</span> <span class="token boolean">false</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>cert_file <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"cert_file"</span><span class="token punctuation">,</span> <span class="token string">""</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>key_file <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"key_file"</span><span class="token punctuation">,</span> <span class="token string">""</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>ca_file <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"ca_file"</span><span class="token punctuation">,</span> <span class="token string">""</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>server_name <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"server_name"</span><span class="token punctuation">,</span> <span class="token string">""</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>timeout_seconds <span class="token operator">=</span> <span class="token function">get_table_int</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"timeout"</span><span class="token punctuation">,</span> <span class="token number">30</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>heartbeat_interval <span class="token operator">=</span> <span class="token function">get_table_int</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"heartbeat_interval"</span><span class="token punctuation">,</span> <span class="token number">30</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>auto_reconnect <span class="token operator">=</span> <span class="token function">get_table_bool</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"auto_reconnect"</span><span class="token punctuation">,</span> <span class="token boolean">true</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>reconnect_interval_seconds <span class="token operator">=</span> <span class="token function">get_table_int</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"reconnect_interval"</span><span class="token punctuation">,</span> <span class="token number">5</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    config<span class="token punctuation">.</span>reconnect_max_attempts <span class="token operator">=</span> <span class="token function">get_table_int</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">,</span> <span class="token string">"reconnect_max_attempts"</span><span class="token punctuation">,</span> <span class="token number">0</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 创建 C++ 客户端</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> client <span class="token operator">=</span> <span class="token keyword">new</span> <span class="token function">CroupierClient</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 创建 Lua 用户数据</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token generic-function"><span class="token function">static_cast</span><span class="token generic class-name"><span class="token operator">&lt;</span>LuaClient<span class="token operator">*</span><span class="token operator">></span></span></span><span class="token punctuation">(</span><span class="token function">lua_newuserdata</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token keyword">sizeof</span><span class="token punctuation">(</span>LuaClient<span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    lc<span class="token operator">-></span>client <span class="token operator">=</span> client<span class="token punctuation">;</span></span>
<span class="line">    lc<span class="token operator">-></span>callback_ref <span class="token operator">=</span> LUA_NOREF<span class="token punctuation">;</span></span>
<span class="line">    lc<span class="token operator">-></span>serving <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 设置元表</span></span>
<span class="line">    <span class="token function">luaL_getmetatable</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token string">"CroupierClient"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token function">lua_setmetatable</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 注册函数</span></span>
<span class="line"><span class="token comment">// client:register_function(descriptor_table)</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">register_function</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token function">luaL_checktype</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> LUA_TTABLE<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 从描述符表读取参数</span></span>
<span class="line">    FunctionDescriptor descriptor<span class="token punctuation">;</span></span>
<span class="line">    descriptor<span class="token punctuation">.</span>id <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"id"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    descriptor<span class="token punctuation">.</span>version <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"version"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    descriptor<span class="token punctuation">.</span>category <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"category"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    descriptor<span class="token punctuation">.</span>risk <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"risk"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    descriptor<span class="token punctuation">.</span>entity <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"entity"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    descriptor<span class="token punctuation">.</span>operation <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"operation"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    descriptor<span class="token punctuation">.</span>enabled <span class="token operator">=</span> <span class="token function">get_table_bool</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"enabled"</span><span class="token punctuation">,</span> <span class="token boolean">true</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 验证描述符</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>descriptor<span class="token punctuation">.</span>id<span class="token punctuation">.</span><span class="token function">empty</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token string">"function id is required"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>descriptor<span class="token punctuation">.</span>version<span class="token punctuation">.</span><span class="token function">empty</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token string">"function version is required"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>descriptor<span class="token punctuation">.</span>category<span class="token punctuation">.</span><span class="token function">empty</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token string">"function category is required"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 注册到 C++ 客户端</span></span>
<span class="line">    <span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">        lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">RegisterFunction</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">,</span></span>
<span class="line">            <span class="token punctuation">[</span>L<span class="token punctuation">,</span> lc<span class="token punctuation">]</span><span class="token punctuation">(</span><span class="token keyword">const</span> FunctionContext<span class="token operator">&amp;</span> ctx<span class="token punctuation">,</span> <span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>string<span class="token operator">&amp;</span> payload<span class="token punctuation">)</span> <span class="token operator">-></span> std<span class="token double-colon punctuation">::</span>string <span class="token punctuation">{</span></span>
<span class="line">                <span class="token comment">// 调用 Lua 回调</span></span>
<span class="line">                <span class="token function">lua_rawgeti</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> LUA_REGISTRYINDEX<span class="token punctuation">,</span> lc<span class="token operator">-></span>callback_ref<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">                <span class="token comment">// 推送参数</span></span>
<span class="line">                <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> ctx<span class="token punctuation">.</span>function_id<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> ctx<span class="token punctuation">.</span>call_id<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> payload<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">                <span class="token comment">// 调用 Lua 处理器</span></span>
<span class="line">                <span class="token keyword">int</span> result <span class="token operator">=</span> <span class="token function">lua_pcall</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">3</span><span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> <span class="token number">0</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">                <span class="token keyword">if</span> <span class="token punctuation">(</span>result <span class="token operator">!=</span> LUA_OK<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">                    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> err <span class="token operator">=</span> <span class="token function">lua_tostring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                    <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                    <span class="token keyword">throw</span> std<span class="token double-colon punctuation">::</span><span class="token function">runtime_error</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">                <span class="token comment">// 获取结果</span></span>
<span class="line">                <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token function">lua_isnil</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">                    <span class="token comment">// 第二个值是错误</span></span>
<span class="line">                    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> err <span class="token operator">=</span> <span class="token function">lua_tostring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                    <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                    <span class="token keyword">throw</span> std<span class="token double-colon punctuation">::</span><span class="token function">runtime_error</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">                <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> ret <span class="token operator">=</span> <span class="token function">lua_tostring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> ret <span class="token operator">?</span> ret <span class="token operator">:</span> <span class="token string">""</span><span class="token punctuation">;</span></span>
<span class="line">                <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">                <span class="token keyword">return</span> result<span class="token punctuation">;</span></span>
<span class="line">            <span class="token punctuation">}</span></span>
<span class="line">        <span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">        <span class="token function">lua_pushboolean</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token boolean">true</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 连接</span></span>
<span class="line"><span class="token comment">// client:connect() -> ok, err</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">connect</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">        lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">Connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pushboolean</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token boolean">true</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 断开连接</span></span>
<span class="line"><span class="token comment">// client:disconnect()</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">disconnect</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">Disconnect</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 开始服务</span></span>
<span class="line"><span class="token comment">// client:serve(callback_function)</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">serve</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token function">luaL_checktype</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">,</span> LUA_TFUNCTION<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 保存回调引用</span></span>
<span class="line">    <span class="token function">lua_pushvalue</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">int</span> callback_ref <span class="token operator">=</span> <span class="token function">luaL_ref</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> LUA_REGISTRYINDEX<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    lc<span class="token operator">-></span>callback_ref <span class="token operator">=</span> callback_ref<span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    lc<span class="token operator">-></span>serving <span class="token operator">=</span> <span class="token boolean">true</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 启动服务（阻塞）</span></span>
<span class="line">    <span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">        lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">Serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        lc<span class="token operator">-></span>serving <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        lc<span class="token operator">-></span>serving <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">luaL_unref</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> LUA_REGISTRYINDEX<span class="token punctuation">,</span> lc<span class="token operator">-></span>callback_ref<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        lc<span class="token operator">-></span>callback_ref <span class="token operator">=</span> LUA_NOREF<span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 停止服务</span></span>
<span class="line"><span class="token comment">// client:stop()</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">stop</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">Stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    lc<span class="token operator">-></span>serving <span class="token operator">=</span> <span class="token boolean">false</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">// 清理回调引用</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>lc<span class="token operator">-></span>callback_ref <span class="token operator">!=</span> LUA_NOREF<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token function">luaL_unref</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> LUA_REGISTRYINDEX<span class="token punctuation">,</span> lc<span class="token operator">-></span>callback_ref<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        lc<span class="token operator">-></span>callback_ref <span class="token operator">=</span> LUA_NOREF<span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 同步调用</span></span>
<span class="line"><span class="token comment">// client:invoke(function_id, payload, options_table) -> result, err</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">invoke</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> function_id <span class="token operator">=</span> <span class="token function">luaL_checkstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> payload <span class="token operator">=</span> <span class="token function">lua_tostring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">3</span><span class="token punctuation">)</span><span class="token punctuation">;</span>  <span class="token comment">// 可选</span></span>
<span class="line"></span>
<span class="line">    InvokeOptions options<span class="token punctuation">;</span></span>
<span class="line">    options<span class="token punctuation">.</span>game_id <span class="token operator">=</span> _config<span class="token punctuation">.</span>game_id<span class="token punctuation">;</span></span>
<span class="line">    options<span class="token punctuation">.</span>env <span class="token operator">=</span> _config<span class="token punctuation">.</span>env<span class="token punctuation">;</span></span>
<span class="line">    options<span class="token punctuation">.</span>timeout_seconds <span class="token operator">=</span> <span class="token number">30</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token function">lua_istable</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        options<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"game_id"</span><span class="token punctuation">,</span> options<span class="token punctuation">.</span>game_id<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        options<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"env"</span><span class="token punctuation">,</span> options<span class="token punctuation">.</span>env<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        options<span class="token punctuation">.</span>timeout_seconds <span class="token operator">=</span> <span class="token function">get_table_int</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"timeout"</span><span class="token punctuation">,</span> <span class="token number">30</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        options<span class="token punctuation">.</span>idempotency_key <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"idempotency_key"</span><span class="token punctuation">,</span> <span class="token string">""</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>string result <span class="token operator">=</span> lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">Invoke</span><span class="token punctuation">(</span>function_id<span class="token punctuation">,</span> payload <span class="token operator">?</span> payload <span class="token operator">:</span> <span class="token string">""</span><span class="token punctuation">,</span> options<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> result<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 启动任务</span></span>
<span class="line"><span class="token comment">// client:start_job(function_id, payload, options_table) -> job_id, err</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">start_job</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> function_id <span class="token operator">=</span> <span class="token function">luaL_checkstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> payload <span class="token operator">=</span> <span class="token function">lua_tostring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">3</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    InvokeOptions options<span class="token punctuation">;</span></span>
<span class="line">    options<span class="token punctuation">.</span>game_id <span class="token operator">=</span> _config<span class="token punctuation">.</span>game_id<span class="token punctuation">;</span></span>
<span class="line">    options<span class="token punctuation">.</span>env <span class="token operator">=</span> _config<span class="token punctuation">.</span>env<span class="token punctuation">;</span></span>
<span class="line">    options<span class="token punctuation">.</span>timeout_seconds <span class="token operator">=</span> <span class="token number">30</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span><span class="token function">lua_istable</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">)</span><span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        options<span class="token punctuation">.</span>game_id <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"game_id"</span><span class="token punctuation">,</span> options<span class="token punctuation">.</span>game_id<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        options<span class="token punctuation">.</span>env <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"env"</span><span class="token punctuation">,</span> options<span class="token punctuation">.</span>env<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        options<span class="token punctuation">.</span>timeout_seconds <span class="token operator">=</span> <span class="token function">get_table_int</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"timeout"</span><span class="token punctuation">,</span> <span class="token number">30</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        options<span class="token punctuation">.</span>idempotency_key <span class="token operator">=</span> <span class="token function">get_table_string</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">4</span><span class="token punctuation">,</span> <span class="token string">"idempotency_key"</span><span class="token punctuation">,</span> <span class="token string">""</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">        std<span class="token double-colon punctuation">::</span>string job_id <span class="token operator">=</span> lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">StartJob</span><span class="token punctuation">(</span>function_id<span class="token punctuation">,</span> payload <span class="token operator">?</span> payload <span class="token operator">:</span> <span class="token string">""</span><span class="token punctuation">,</span> options<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> job_id<span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 取消任务</span></span>
<span class="line"><span class="token comment">// client:cancel_job(job_id) -> ok, err</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">cancel_job</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">const</span> <span class="token keyword">char</span><span class="token operator">*</span> job_id <span class="token operator">=</span> <span class="token function">luaL_checkstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">2</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">try</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">bool</span> cancelled <span class="token operator">=</span> lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">CancelJob</span><span class="token punctuation">(</span>job_id<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pushboolean</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> cancelled<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span> <span class="token keyword">catch</span> <span class="token punctuation">(</span><span class="token keyword">const</span> std<span class="token double-colon punctuation">::</span>exception<span class="token operator">&amp;</span> e<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">push_error</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> e<span class="token punctuation">.</span><span class="token function">what</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// GC 清理</span></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">int</span> <span class="token function">gc</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>lc<span class="token operator">-></span>callback_ref <span class="token operator">!=</span> LUA_NOREF<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token function">luaL_unref</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> LUA_REGISTRYINDEX<span class="token punctuation">,</span> lc<span class="token operator">-></span>callback_ref<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> <span class="token punctuation">(</span>lc<span class="token operator">-></span>client<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">delete</span> lc<span class="token operator">-></span>client<span class="token punctuation">;</span></span>
<span class="line">        lc<span class="token operator">-></span>client <span class="token operator">=</span> <span class="token keyword">nullptr</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token number">0</span><span class="token punctuation">;</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// ===== 库初始化 =====</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">const</span> <span class="token keyword">struct</span> <span class="token class-name">luaL_Reg</span> croupier_lib<span class="token punctuation">[</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"create_client"</span><span class="token punctuation">,</span> create_client<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token constant">NULL</span><span class="token punctuation">,</span> <span class="token constant">NULL</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">const</span> <span class="token keyword">struct</span> <span class="token class-name">luaL_Reg</span> client_methods<span class="token punctuation">[</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"register_function"</span><span class="token punctuation">,</span> register_function<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"connect"</span><span class="token punctuation">,</span> connect<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"disconnect"</span><span class="token punctuation">,</span> disconnect<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"serve"</span><span class="token punctuation">,</span> serve<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"stop"</span><span class="token punctuation">,</span> stop<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"invoke"</span><span class="token punctuation">,</span> invoke<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"start_job"</span><span class="token punctuation">,</span> start_job<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"cancel_job"</span><span class="token punctuation">,</span> cancel_job<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"is_connected"</span><span class="token punctuation">,</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pushboolean</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">IsConnected</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"get_local_address"</span><span class="token punctuation">,</span> <span class="token punctuation">[</span><span class="token punctuation">]</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token keyword">auto</span><span class="token operator">*</span> lc <span class="token operator">=</span> <span class="token function">check_lua_client</span><span class="token punctuation">(</span>L<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pushstring</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> lc<span class="token operator">-></span>client<span class="token operator">-></span><span class="token function">GetLocalAddress</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">.</span><span class="token function">c_str</span><span class="token punctuation">(</span><span class="token punctuation">)</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token constant">NULL</span><span class="token punctuation">,</span> <span class="token constant">NULL</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">static</span> <span class="token keyword">const</span> <span class="token keyword">struct</span> <span class="token class-name">luaL_Reg</span> client_metamethods<span class="token punctuation">[</span><span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"__gc"</span><span class="token punctuation">,</span> gc<span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token string">"__close"</span><span class="token punctuation">,</span> stop<span class="token punctuation">}</span><span class="token punctuation">,</span>  <span class="token comment">// 支持 to-be-closed 协议</span></span>
<span class="line">    <span class="token punctuation">{</span><span class="token constant">NULL</span><span class="token punctuation">,</span> <span class="token constant">NULL</span><span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">// 注册元方法</span></span>
<span class="line"><span class="token keyword">extern</span> <span class="token string">"C"</span> <span class="token punctuation">{</span></span>
<span class="line">    <span class="token keyword">int</span> <span class="token function">luaopen_croupier_core</span><span class="token punctuation">(</span>lua_State<span class="token operator">*</span> L<span class="token punctuation">)</span> <span class="token punctuation">{</span></span>
<span class="line">        <span class="token comment">// 创建客户端元表</span></span>
<span class="line">        <span class="token function">luaL_newmetatable</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token string">"CroupierClient"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pushvalue</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_setfield</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token operator">-</span><span class="token number">2</span><span class="token punctuation">,</span> <span class="token string">"__index"</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">luaL_setfuncs</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> client_methods<span class="token punctuation">,</span> <span class="token number">0</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">luaL_setfuncs</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> client_metamethods<span class="token punctuation">,</span> <span class="token number">0</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token function">lua_pop</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> <span class="token number">1</span><span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">// 创建库</span></span>
<span class="line">        <span class="token function">luaL_newlib</span><span class="token punctuation">(</span>L<span class="token punctuation">,</span> croupier_lib<span class="token punctuation">)</span><span class="token punctuation">;</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token number">1</span><span class="token punctuation">;</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token punctuation">}</span> <span class="token comment">// namespace lua</span></span>
<span class="line"><span class="token punctuation">}</span> <span class="token comment">// namespace croupier</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_5-lua-sdk-模块" tabindex="-1"><a class="header-anchor" href="#_5-lua-sdk-模块"><span>5. Lua SDK 模块</span></a></h3>
<div class="language-lua line-numbers-mode" data-highlighter="prismjs" data-ext="lua"><pre v-pre><code class="language-lua"><span class="line"><span class="token comment">-- croupier-sdk-cpp/lua/croupier/init.lua</span></span>
<span class="line"><span class="token keyword">local</span> croupier <span class="token operator">=</span> <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">-- 加载 C 核心库</span></span>
<span class="line"><span class="token keyword">local</span> ok<span class="token punctuation">,</span> core <span class="token operator">=</span> <span class="token function">pcall</span><span class="token punctuation">(</span>require<span class="token punctuation">,</span> <span class="token string">"croupier.core"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">if</span> <span class="token keyword">not</span> ok <span class="token keyword">then</span></span>
<span class="line">    <span class="token function">error</span><span class="token punctuation">(</span><span class="token string">"Failed to load croupier.core: "</span> <span class="token operator">..</span> core<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">croupier<span class="token punctuation">.</span>_core <span class="token operator">=</span> core</span>
<span class="line"></span>
<span class="line"><span class="token comment">-- 导出模块</span></span>
<span class="line">croupier<span class="token punctuation">.</span>Client <span class="token operator">=</span> require <span class="token string">"croupier.client"</span></span>
<span class="line">croupier<span class="token punctuation">.</span>Invoker <span class="token operator">=</span> require <span class="token string">"croupier.invoker"</span></span>
<span class="line">croupier<span class="token punctuation">.</span>types <span class="token operator">=</span> require <span class="token string">"croupier.types"</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">return</span> croupier</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><div class="language-lua line-numbers-mode" data-highlighter="prismjs" data-ext="lua"><pre v-pre><code class="language-lua"><span class="line"><span class="token comment">-- croupier-sdk-cpp/lua/croupier/client.lua</span></span>
<span class="line"><span class="token keyword">local</span> core <span class="token operator">=</span> require <span class="token string">"croupier.core"</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">local</span> Client <span class="token operator">=</span> <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">Client<span class="token punctuation">.</span>__index <span class="token operator">=</span> Client</span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> Client<span class="token punctuation">.</span><span class="token function">new</span><span class="token punctuation">(</span>config<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">local</span> self <span class="token operator">=</span> <span class="token function">setmetatable</span><span class="token punctuation">(</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">,</span> Client<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 创建核心客户端</span></span>
<span class="line">    self<span class="token punctuation">.</span>_handle <span class="token operator">=</span> core<span class="token punctuation">.</span><span class="token function">create_client</span><span class="token punctuation">(</span><span class="token punctuation">{</span></span>
<span class="line">        agent_addr <span class="token operator">=</span> config<span class="token punctuation">.</span>agent_addr <span class="token keyword">or</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">,</span></span>
<span class="line">        service_id <span class="token operator">=</span> config<span class="token punctuation">.</span>service_id <span class="token keyword">or</span> <span class="token string">"lua-service"</span><span class="token punctuation">,</span></span>
<span class="line">        service_version <span class="token operator">=</span> config<span class="token punctuation">.</span>service_version <span class="token keyword">or</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">        game_id <span class="token operator">=</span> config<span class="token punctuation">.</span>game_id <span class="token keyword">or</span> <span class="token string">"default-game"</span><span class="token punctuation">,</span></span>
<span class="line">        env <span class="token operator">=</span> config<span class="token punctuation">.</span>env <span class="token keyword">or</span> <span class="token string">"dev"</span><span class="token punctuation">,</span></span>
<span class="line">        local_addr <span class="token operator">=</span> config<span class="token punctuation">.</span>local_addr <span class="token keyword">or</span> <span class="token string">"0.0.0.0:0"</span><span class="token punctuation">,</span></span>
<span class="line">        insecure <span class="token operator">=</span> config<span class="token punctuation">.</span>insecure <span class="token keyword">or</span> <span class="token keyword">false</span><span class="token punctuation">,</span></span>
<span class="line">        cert_file <span class="token operator">=</span> config<span class="token punctuation">.</span>cert_file<span class="token punctuation">,</span></span>
<span class="line">        key_file <span class="token operator">=</span> config<span class="token punctuation">.</span>key_file<span class="token punctuation">,</span></span>
<span class="line">        ca_file <span class="token operator">=</span> config<span class="token punctuation">.</span>ca_file<span class="token punctuation">,</span></span>
<span class="line">        timeout <span class="token operator">=</span> config<span class="token punctuation">.</span>timeout <span class="token keyword">or</span> <span class="token number">30</span><span class="token punctuation">,</span></span>
<span class="line">        heartbeat_interval <span class="token operator">=</span> config<span class="token punctuation">.</span>heartbeat_interval <span class="token keyword">or</span> <span class="token number">30</span><span class="token punctuation">,</span></span>
<span class="line">        auto_reconnect <span class="token operator">=</span> config<span class="token punctuation">.</span>auto_reconnect<span class="token punctuation">,</span></span>
<span class="line">        reconnect_interval <span class="token operator">=</span> config<span class="token punctuation">.</span>reconnect_interval<span class="token punctuation">,</span></span>
<span class="line">        reconnect_max_attempts <span class="token operator">=</span> config<span class="token punctuation">.</span>reconnect_max_attempts<span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    self<span class="token punctuation">.</span>_functions <span class="token operator">=</span> <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">    self<span class="token punctuation">.</span>_connected <span class="token operator">=</span> <span class="token keyword">false</span></span>
<span class="line">    self<span class="token punctuation">.</span>_serving <span class="token operator">=</span> <span class="token keyword">false</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> self</span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> Client<span class="token punctuation">:</span><span class="token function">register_function</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">,</span> handler<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token function">type</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">)</span> <span class="token operator">~=</span> <span class="token string">"table"</span> <span class="token keyword">then</span></span>
<span class="line">        <span class="token function">error</span><span class="token punctuation">(</span><span class="token string">"descriptor must be a table"</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 验证必需字段</span></span>
<span class="line">    <span class="token keyword">local</span> required <span class="token operator">=</span> <span class="token punctuation">{</span><span class="token string">"id"</span><span class="token punctuation">,</span> <span class="token string">"version"</span><span class="token punctuation">,</span> <span class="token string">"category"</span><span class="token punctuation">,</span> <span class="token string">"risk"</span><span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">for</span> _<span class="token punctuation">,</span> field <span class="token keyword">in</span> <span class="token function">ipairs</span><span class="token punctuation">(</span>required<span class="token punctuation">)</span> <span class="token keyword">do</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token keyword">not</span> descriptor<span class="token punctuation">[</span>field<span class="token punctuation">]</span> <span class="token keyword">then</span></span>
<span class="line">            <span class="token function">error</span><span class="token punctuation">(</span>string<span class="token punctuation">.</span><span class="token function">format</span><span class="token punctuation">(</span><span class="token string">"descriptor.%s is required"</span><span class="token punctuation">,</span> field<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">end</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 存储处理器</span></span>
<span class="line">    self<span class="token punctuation">.</span>_functions<span class="token punctuation">[</span>descriptor<span class="token punctuation">.</span>id<span class="token punctuation">]</span> <span class="token operator">=</span> handler</span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 通过 C API 注册</span></span>
<span class="line">    <span class="token keyword">local</span> ok<span class="token punctuation">,</span> err <span class="token operator">=</span> self<span class="token punctuation">.</span>_handle<span class="token punctuation">:</span><span class="token function">register_function</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token keyword">not</span> ok <span class="token keyword">then</span></span>
<span class="line">        self<span class="token punctuation">.</span>_functions<span class="token punctuation">[</span>descriptor<span class="token punctuation">.</span>id<span class="token punctuation">]</span> <span class="token operator">=</span> <span class="token keyword">nil</span></span>
<span class="line">        <span class="token function">error</span><span class="token punctuation">(</span>string<span class="token punctuation">.</span><span class="token function">format</span><span class="token punctuation">(</span><span class="token string">"register function failed: %s"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 设置回调</span></span>
<span class="line">    self<span class="token punctuation">.</span>_handle<span class="token punctuation">:</span><span class="token function">set_function_handler</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">.</span>id<span class="token punctuation">,</span> <span class="token keyword">function</span><span class="token punctuation">(</span>context<span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line">        <span class="token comment">-- 解析 payload</span></span>
<span class="line">        <span class="token keyword">local</span> payload_obj</span>
<span class="line">        <span class="token keyword">if</span> payload <span class="token keyword">and</span> payload <span class="token operator">~=</span> <span class="token string">""</span> <span class="token keyword">then</span></span>
<span class="line">            <span class="token keyword">local</span> ok<span class="token punctuation">,</span> decoded <span class="token operator">=</span> <span class="token function">pcall</span><span class="token punctuation">(</span>cjson<span class="token punctuation">.</span>decode<span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line">            <span class="token keyword">if</span> ok <span class="token keyword">then</span></span>
<span class="line">                payload_obj <span class="token operator">=</span> decoded</span>
<span class="line">            <span class="token keyword">else</span></span>
<span class="line">                payload_obj <span class="token operator">=</span> payload</span>
<span class="line">            <span class="token keyword">end</span></span>
<span class="line">        <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">-- 调用处理器</span></span>
<span class="line">        <span class="token keyword">local</span> ok<span class="token punctuation">,</span> result <span class="token operator">=</span> <span class="token function">pcall</span><span class="token punctuation">(</span>handler<span class="token punctuation">,</span> context<span class="token punctuation">,</span> payload_obj<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">        <span class="token keyword">if</span> <span class="token keyword">not</span> ok <span class="token keyword">then</span></span>
<span class="line">            <span class="token keyword">return</span> <span class="token keyword">nil</span><span class="token punctuation">,</span> result  <span class="token comment">-- 错误信息</span></span>
<span class="line">        <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">-- 序列化结果</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token function">type</span><span class="token punctuation">(</span>result<span class="token punctuation">)</span> <span class="token operator">==</span> <span class="token string">"table"</span> <span class="token keyword">then</span></span>
<span class="line">            <span class="token keyword">return</span> cjson<span class="token punctuation">.</span><span class="token function">encode</span><span class="token punctuation">(</span>result<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">tostring</span><span class="token punctuation">(</span>result<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">end</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">true</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> Client<span class="token punctuation">:</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> self<span class="token punctuation">.</span>_connected <span class="token keyword">then</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token keyword">true</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">local</span> ok<span class="token punctuation">,</span> err <span class="token operator">=</span> self<span class="token punctuation">.</span>_handle<span class="token punctuation">:</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token keyword">not</span> ok <span class="token keyword">then</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token keyword">false</span><span class="token punctuation">,</span> err</span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    self<span class="token punctuation">.</span>_connected <span class="token operator">=</span> <span class="token keyword">true</span></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">true</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> Client<span class="token punctuation">:</span><span class="token function">serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token keyword">not</span> self<span class="token punctuation">.</span>_connected <span class="token keyword">then</span></span>
<span class="line">        <span class="token keyword">local</span> ok<span class="token punctuation">,</span> err <span class="token operator">=</span> self<span class="token punctuation">:</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">if</span> <span class="token keyword">not</span> ok <span class="token keyword">then</span></span>
<span class="line">            <span class="token function">error</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">end</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    self<span class="token punctuation">.</span>_serving <span class="token operator">=</span> <span class="token keyword">true</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 启动服务（阻塞）</span></span>
<span class="line">    <span class="token keyword">local</span> ok<span class="token punctuation">,</span> err <span class="token operator">=</span> self<span class="token punctuation">.</span>_handle<span class="token punctuation">:</span><span class="token function">serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    self<span class="token punctuation">.</span>_serving <span class="token operator">=</span> <span class="token keyword">false</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">if</span> <span class="token keyword">not</span> ok <span class="token keyword">then</span></span>
<span class="line">        <span class="token function">error</span><span class="token punctuation">(</span>err<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> Client<span class="token punctuation">:</span><span class="token function">stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token keyword">not</span> self<span class="token punctuation">.</span>_serving <span class="token keyword">then</span></span>
<span class="line">        <span class="token keyword">return</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    self<span class="token punctuation">.</span>_serving <span class="token operator">=</span> <span class="token keyword">false</span></span>
<span class="line">    self<span class="token punctuation">.</span>_handle<span class="token punctuation">:</span><span class="token function">stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> Client<span class="token punctuation">:</span><span class="token function">disconnect</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token keyword">not</span> self<span class="token punctuation">.</span>_connected <span class="token keyword">then</span></span>
<span class="line">        <span class="token keyword">return</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    self<span class="token punctuation">:</span><span class="token function">stop</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    self<span class="token punctuation">.</span>_handle<span class="token punctuation">:</span><span class="token function">disconnect</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    self<span class="token punctuation">.</span>_connected <span class="token operator">=</span> <span class="token keyword">false</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> Client<span class="token punctuation">:</span><span class="token function">is_connected</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> self<span class="token punctuation">.</span>_connected</span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">return</span> Client</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_6-skynet-服务示例" tabindex="-1"><a class="header-anchor" href="#_6-skynet-服务示例"><span>6. Skynet 服务示例</span></a></h3>
<div class="language-lua line-numbers-mode" data-highlighter="prismjs" data-ext="lua"><pre v-pre><code class="language-lua"><span class="line"><span class="token comment">-- croupier-sdk-cpp/skynet/service/croupier_service.lua</span></span>
<span class="line"><span class="token keyword">local</span> skynet <span class="token operator">=</span> require <span class="token string">"skynet"</span></span>
<span class="line"><span class="token keyword">local</span> croupier <span class="token operator">=</span> require <span class="token string">"croupier"</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">local</span> M <span class="token operator">=</span> <span class="token punctuation">{</span><span class="token punctuation">}</span></span>
<span class="line">M<span class="token punctuation">.</span>__index <span class="token operator">=</span> M</span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> M<span class="token punctuation">.</span><span class="token function">new</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">local</span> self <span class="token operator">=</span> <span class="token function">setmetatable</span><span class="token punctuation">(</span><span class="token punctuation">{</span><span class="token punctuation">}</span><span class="token punctuation">,</span> M<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 从环境变量读取配置</span></span>
<span class="line">    self<span class="token punctuation">.</span>config <span class="token operator">=</span> <span class="token punctuation">{</span></span>
<span class="line">        agent_addr <span class="token operator">=</span> skynet<span class="token punctuation">.</span><span class="token function">getenv</span><span class="token punctuation">(</span><span class="token string">"CROUPIER_AGENT_ADDR"</span><span class="token punctuation">)</span> <span class="token keyword">or</span> <span class="token string">"127.0.0.1:19090"</span><span class="token punctuation">,</span></span>
<span class="line">        service_id <span class="token operator">=</span> skynet<span class="token punctuation">.</span><span class="token function">getenv</span><span class="token punctuation">(</span><span class="token string">"SERVICE_ID"</span><span class="token punctuation">)</span> <span class="token keyword">or</span> <span class="token string">"skynet-service"</span><span class="token punctuation">,</span></span>
<span class="line">        game_id <span class="token operator">=</span> skynet<span class="token punctuation">.</span><span class="token function">getenv</span><span class="token punctuation">(</span><span class="token string">"GAME_ID"</span><span class="token punctuation">)</span> <span class="token keyword">or</span> <span class="token string">"default-game"</span><span class="token punctuation">,</span></span>
<span class="line">        env <span class="token operator">=</span> skynet<span class="token punctuation">.</span><span class="token function">getenv</span><span class="token punctuation">(</span><span class="token string">"ENV"</span><span class="token punctuation">)</span> <span class="token keyword">or</span> <span class="token string">"dev"</span><span class="token punctuation">,</span></span>
<span class="line">        insecure <span class="token operator">=</span> skynet<span class="token punctuation">.</span><span class="token function">getenv</span><span class="token punctuation">(</span><span class="token string">"CROUPIER_INSECURE"</span><span class="token punctuation">)</span> <span class="token operator">==</span> <span class="token string">"true"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 创建客户端</span></span>
<span class="line">    self<span class="token punctuation">.</span>client <span class="token operator">=</span> croupier<span class="token punctuation">.</span>Client<span class="token punctuation">.</span><span class="token function">new</span><span class="token punctuation">(</span>self<span class="token punctuation">.</span>config<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> self</span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> M<span class="token punctuation">:</span><span class="token function">register</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">,</span> handler<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> self<span class="token punctuation">.</span>client<span class="token punctuation">:</span><span class="token function">register_function</span><span class="token punctuation">(</span>descriptor<span class="token punctuation">,</span> <span class="token keyword">function</span><span class="token punctuation">(</span>context<span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line">        <span class="token comment">-- 在 Skynet 上下文中执行</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token function">handler</span><span class="token punctuation">(</span>context<span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">end</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> M<span class="token punctuation">:</span><span class="token function">start</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">local</span> ok<span class="token punctuation">,</span> err <span class="token operator">=</span> self<span class="token punctuation">.</span>client<span class="token punctuation">:</span><span class="token function">connect</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">if</span> <span class="token keyword">not</span> ok <span class="token keyword">then</span></span>
<span class="line">        skynet<span class="token punctuation">.</span><span class="token function">error</span><span class="token punctuation">(</span><span class="token string">"croupier connect failed:"</span><span class="token punctuation">,</span> err<span class="token punctuation">)</span></span>
<span class="line">        <span class="token keyword">return</span> <span class="token keyword">false</span></span>
<span class="line">    <span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line">    skynet<span class="token punctuation">.</span><span class="token function">error</span><span class="token punctuation">(</span><span class="token string">"croupier connected"</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 启动服务（在独立协程中）</span></span>
<span class="line">    skynet<span class="token punctuation">.</span><span class="token function">fork</span><span class="token punctuation">(</span><span class="token keyword">function</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">        self<span class="token punctuation">.</span>client<span class="token punctuation">:</span><span class="token function">serve</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">end</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token keyword">return</span> <span class="token keyword">true</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">function</span> M<span class="token punctuation">:</span><span class="token function">invoke</span><span class="token punctuation">(</span>function_id<span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">return</span> self<span class="token punctuation">.</span>client<span class="token punctuation">:</span><span class="token function">invoke</span><span class="token punctuation">(</span>function_id<span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">end</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">-- 启动服务</span></span>
<span class="line">skynet<span class="token punctuation">.</span><span class="token function">start</span><span class="token punctuation">(</span><span class="token keyword">function</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line">    <span class="token keyword">local</span> service <span class="token operator">=</span> M<span class="token punctuation">.</span><span class="token function">new</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 注册示例函数</span></span>
<span class="line">    service<span class="token punctuation">:</span><span class="token function">register</span><span class="token punctuation">(</span><span class="token punctuation">{</span></span>
<span class="line">        id <span class="token operator">=</span> <span class="token string">"player.get"</span><span class="token punctuation">,</span></span>
<span class="line">        version <span class="token operator">=</span> <span class="token string">"1.0.0"</span><span class="token punctuation">,</span></span>
<span class="line">        category <span class="token operator">=</span> <span class="token string">"player"</span><span class="token punctuation">,</span></span>
<span class="line">        risk <span class="token operator">=</span> <span class="token string">"low"</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span> <span class="token keyword">function</span><span class="token punctuation">(</span>context<span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line">        skynet<span class="token punctuation">.</span><span class="token function">error</span><span class="token punctuation">(</span><span class="token string">"player.get called:"</span><span class="token punctuation">,</span> payload<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">        <span class="token comment">-- 调用其他 Skynet 服务</span></span>
<span class="line">        <span class="token keyword">local</span> player <span class="token operator">=</span> skynet<span class="token punctuation">.</span><span class="token function">call</span><span class="token punctuation">(</span><span class="token string">".player_mgr"</span><span class="token punctuation">,</span> <span class="token string">"lua"</span><span class="token punctuation">,</span> <span class="token string">"get_player"</span><span class="token punctuation">,</span> payload<span class="token punctuation">.</span>player_id<span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">        <span class="token keyword">return</span> <span class="token punctuation">{</span></span>
<span class="line">            status <span class="token operator">=</span> <span class="token string">"success"</span><span class="token punctuation">,</span></span>
<span class="line">            player <span class="token operator">=</span> player</span>
<span class="line">        <span class="token punctuation">}</span></span>
<span class="line">    <span class="token keyword">end</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    <span class="token comment">-- 启动服务</span></span>
<span class="line">    service<span class="token punctuation">:</span><span class="token function">start</span><span class="token punctuation">(</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line">    skynet<span class="token punctuation">.</span><span class="token function">info</span><span class="token punctuation">(</span><span class="token string">"croupier service started"</span><span class="token punctuation">)</span></span>
<span class="line"><span class="token keyword">end</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token keyword">return</span> M</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="_7-编译和使用" tabindex="-1"><a class="header-anchor" href="#_7-编译和使用"><span>7. 编译和使用</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 编译（启用 Lua 绑定）</span></span>
<span class="line"><span class="token builtin class-name">cd</span> croupier-sdk-cpp</span>
<span class="line"><span class="token function">mkdir</span> build <span class="token operator">&amp;&amp;</span> <span class="token builtin class-name">cd</span> build</span>
<span class="line">cmake <span class="token punctuation">..</span> <span class="token parameter variable">-DENABLE_LUA_BINDING</span><span class="token operator">=</span>ON <span class="token parameter variable">-DCMAKE_BUILD_TYPE</span><span class="token operator">=</span>Release</span>
<span class="line"><span class="token function">make</span> -j<span class="token variable"><span class="token variable">$(</span>nproc<span class="token variable">)</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 输出文件：</span></span>
<span class="line"><span class="token comment"># Linux: libcroupier.so</span></span>
<span class="line"><span class="token comment"># Windows: croupier.dll</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 安装到 Skynet</span></span>
<span class="line"><span class="token function">cp</span> libcroupier.so /path/to/skynet/cservice/</span>
<span class="line"><span class="token function">cp</span> <span class="token parameter variable">-r</span> lua/croupier /path/to/skynet/lua/</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="配置选项对比" tabindex="-1"><a class="header-anchor" href="#配置选项对比"><span>配置选项对比</span></a></h2>
<table>
<thead>
<tr>
<th>选项</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>ENABLE_LUA_BINDING=ON</code></td>
<td>启用 Lua 绑定，生成 <code v-pre>libcroupier.so</code></td>
</tr>
<tr>
<td><code v-pre>LUA_INCLUDE_DIR</code></td>
<td>Lua 头文件路径（自动查找）</td>
</tr>
<tr>
<td><code v-pre>LUA_LIBRARIES</code></td>
<td>Lua 库名（自动查找）</td>
</tr>
<tr>
<td><code v-pre>LUA_VERSION_MAJOR</code></td>
<td>Lua 主版本号（需要 5.3+）</td>
</tr>
</tbody>
</table>
<h2 id="优势总结" tabindex="-1"><a class="header-anchor" href="#优势总结"><span>优势总结</span></a></h2>
<p>与独立 <code v-pre>croupier-sdk-lua</code> 仓库相比，集成方案优势明显：</p>
<table>
<thead>
<tr>
<th>对比项</th>
<th>独立仓库</th>
<th>集成到 croupier-sdk-cpp</th>
</tr>
</thead>
<tbody>
<tr>
<td>代码复用</td>
<td>❌ 需要复制 C++ 代码</td>
<td>✅ 直接复用</td>
</tr>
<tr>
<td>维护成本</td>
<td>❌ 需要同步更新</td>
<td>✅ 统一维护</td>
</tr>
<tr>
<td>构建复杂度</td>
<td>❌ 需要 C++ + Lua 两套构建系统</td>
<td>✅ 一个 CMake 系统</td>
</tr>
<tr>
<td>版本一致性</td>
<td>❌ 容易出现版本不同步</td>
<td>✅ 自动保持一致</td>
</tr>
<tr>
<td>发布流程</td>
<td>❌ 需要分别发布</td>
<td>✅ 一次发布，多语言可用</td>
</tr>
</tbody>
</table>
<h2 id="实现状态" tabindex="-1"><a class="header-anchor" href="#实现状态"><span>实现状态</span></a></h2>
<h3 id="已实现文件" tabindex="-1"><a class="header-anchor" href="#已实现文件"><span>已实现文件</span></a></h3>
<table>
<thead>
<tr>
<th>文件路径</th>
<th>状态</th>
<th>说明</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>croupier-sdk-cpp/CMakeLists.txt</code></td>
<td>✅ 已修改</td>
<td>添加 <code v-pre>ENABLE_LUA_BINDING</code> 选项</td>
</tr>
<tr>
<td><code v-pre>croupier-sdk-cpp/src/bindings/lua_binding.h</code></td>
<td>✅ 已创建</td>
<td>Lua 绑定头文件</td>
</tr>
<tr>
<td><code v-pre>croupier-sdk-cpp/src/bindings/lua_binding.cpp</code></td>
<td>✅ 已创建</td>
<td>Lua C API 绑定实现</td>
</tr>
<tr>
<td><code v-pre>croupier-sdk-cpp/lua/croupier/init.lua</code></td>
<td>✅ 已创建</td>
<td>Lua 模块入口</td>
</tr>
<tr>
<td><code v-pre>croupier-sdk-cpp/skynet/service/croupier_service.lua</code></td>
<td>✅ 已创建</td>
<td>Skynet 服务封装</td>
</tr>
<tr>
<td><code v-pre>croupier-sdk-cpp/skynet/examples/config.lua</code></td>
<td>✅ 已创建</td>
<td>Skynet 配置示例</td>
</tr>
<tr>
<td><code v-pre>croupier-sdk-cpp/skynet/examples/main.lua</code></td>
<td>✅ 已创建</td>
<td>Skynet 主服务示例</td>
</tr>
<tr>
<td><code v-pre>croupier-sdk-cpp/lua/examples/standalone_example.lua</code></td>
<td>✅ 已创建</td>
<td>独立 Lua 示例</td>
</tr>
</tbody>
</table>
<h3 id="编译命令" tabindex="-1"><a class="header-anchor" href="#编译命令"><span>编译命令</span></a></h3>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 启用 Lua 绑定编译</span></span>
<span class="line"><span class="token builtin class-name">cd</span> croupier-sdk-cpp</span>
<span class="line"><span class="token function">mkdir</span> build <span class="token operator">&amp;&amp;</span> <span class="token builtin class-name">cd</span> build</span>
<span class="line">cmake <span class="token punctuation">..</span> <span class="token parameter variable">-DENABLE_LUA_BINDING</span><span class="token operator">=</span>ON <span class="token parameter variable">-DBUILD_SHARED_LIBS</span><span class="token operator">=</span>ON</span>
<span class="line"><span class="token function">make</span> -j<span class="token variable"><span class="token variable">$(</span>nproc<span class="token variable">)</span></span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 输出: bin/libcroupier-sdk.so (包含 Lua API)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="使用示例" tabindex="-1"><a class="header-anchor" href="#使用示例"><span>使用示例</span></a></h3>
<div class="language-lua line-numbers-mode" data-highlighter="prismjs" data-ext="lua"><pre v-pre><code class="language-lua"><span class="line"><span class="token comment">-- 独立 Lua 使用</span></span>
<span class="line"><span class="token keyword">local</span> croupier <span class="token operator">=</span> require <span class="token string">"croupier"</span></span>
<span class="line"><span class="token keyword">local</span> client <span class="token operator">=</span> croupier<span class="token punctuation">.</span>Client<span class="token punctuation">.</span><span class="token function">new</span><span class="token punctuation">(</span><span class="token string">"localhost:50051"</span><span class="token punctuation">)</span></span>
<span class="line">client<span class="token punctuation">:</span><span class="token function">register_virtual_object</span><span class="token punctuation">(</span><span class="token string">"player:1001"</span><span class="token punctuation">,</span> <span class="token string">"player"</span><span class="token punctuation">,</span> <span class="token punctuation">{</span>level <span class="token operator">=</span> <span class="token number">50</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span>
<span class="line"><span class="token comment">-- Skynet 中使用</span></span>
<span class="line"><span class="token keyword">local</span> croupier_service <span class="token operator">=</span> skynet<span class="token punctuation">.</span><span class="token function">call</span><span class="token punctuation">(</span><span class="token string">".croupier"</span><span class="token punctuation">,</span> <span class="token string">"lua"</span><span class="token punctuation">,</span> <span class="token string">"start"</span><span class="token punctuation">,</span> <span class="token string">"localhost:50051"</span><span class="token punctuation">)</span></span>
<span class="line">skynet<span class="token punctuation">.</span><span class="token function">call</span><span class="token punctuation">(</span><span class="token string">".croupier"</span><span class="token punctuation">,</span> <span class="token string">"lua"</span><span class="token punctuation">,</span> <span class="token string">"register_vo"</span><span class="token punctuation">,</span> <span class="token string">"player:1001"</span><span class="token punctuation">,</span> <span class="token string">"player"</span><span class="token punctuation">,</span> <span class="token punctuation">{</span>level <span class="token operator">=</span> <span class="token number">50</span><span class="token punctuation">}</span><span class="token punctuation">)</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></div></template>


