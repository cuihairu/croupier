---
home: true
title: Croupier C++ SDK
titleTemplate: false
heroImage: /logo.png
heroText: Croupier C++ SDK
tagline: 高性能 C++ SDK，用于 Croupier 游戏函数注册与虚拟对象管理
actions:
  - text: 快速开始
    link: /guide/quick-start.html
    type: primary
  - text: 安装指南
    link: /guide/installation.html
    type: secondary
features:
  - title: 🏗️ 虚拟对象注册
    details: 完整的对象和函数组注册机制，支持四层组件化架构
  - title: 📡 gRPC 集成
    details: 真实的 Protobuf + gRPC 通信，通过 vcpkg 自动依赖管理
  - title: 🔧 多平台支持
    details: 支持 Windows、Linux、macOS（x64/ARM64）全平台构建
  - title: 🚀 高性能设计
    details: ID 引用模式，无状态函数，轻量参数传递
  - title: 🛡️ 类型安全
    details: JSON Schema 验证，编译时类型检查
  - title: 🔄 自动构建
    details: GitHub Actions 每日自动构建发布

footer: Apache License 2.0 | Copyright © 2024 Croupier
---

## 📋 简介

Croupier C++ SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 C++ 客户端实现。它提供了高性能的虚拟对象注册、gRPC 通信和多平台构建支持。

## 🚀 快速开始

### 安装依赖

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier-sdk-cpp.git
cd croupier-sdk-cpp

# 使用脚本构建（推荐）
./scripts/build.sh
```

### 基础使用

```cpp
#include "croupier/sdk/croupier_client.h"

int main() {
    croupier::sdk::ClientConfig config;
    config.game_id = "my-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    croupier::sdk::CroupierClient client(config);
    client.Connect();
    client.Serve();
}
```

## 🔗 相关链接

- [主项目](https://github.com/cuihairu/croupier)
- [Go SDK](https://github.com/cuihairu/croupier-sdk-go)
- [Java SDK](https://github.com/cuihairu/croupier-sdk-java)
- [JavaScript SDK](https://github.com/cuihairu/croupier-sdk-js)
- [Python SDK](https://github.com/cuihairu/croupier-sdk-python)
