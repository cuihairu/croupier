<p align="center">
  <h1 align="center">Croupier Node.js SDK</h1>
  <p align="center">
    <strong>TypeScript 优先的 Node.js SDK，用于 Croupier 游戏函数注册与执行系统</strong>
  </p>
</p>

<p align="center">
  <a href="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml">
    <img src="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg" alt="CI">
  </a>
  <a href="https://codecov.io/gh/cuihairu/croupier">
    <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=js-sdk" alt="Coverage">
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://nodejs.org/">
    <img src="https://img.shields.io/badge/Node.js-22+-339933.svg" alt="Node.js Version">
  </a>
  <a href="https://www.typescriptlang.org/">
    <img src="https://img.shields.io/badge/TypeScript-5.0+-3178C6.svg" alt="TypeScript">
  </a>
</p>

<p align="center">
  <a href="#支持平台">
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg" alt="Platform">
  </a>
  <a href="https://github.com/cuihairu/croupier">
    <img src="https://img.shields.io/badge/Main%20Project-Croupier-green.svg" alt="Main Project">
  </a>
</p>

---

## 📋 目录

- [简介](#简介)
- [主项目](#主项目)
- [其他语言 SDK](#其他语言-sdk)
- [支持平台](#支持平台)
- [核心特性](#核心特性)
- [快速开始](#快速开始)
- [使用示例](#使用示例)
- [架构设计](#架构设计)
- [API 参考](#api-参考)
- [开发指南](#开发指南)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

---

## 简介

Croupier Node.js SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 Node.js/TypeScript 客户端实现。SDK 作为 **Provider 端被调用方**，通过 **单条 TCP session**（`sdk-agent subprotocol`）接入 Agent，提供函数注册、心跳、自动重连、TLS 与控制面 manifest 上传能力。

## 正式文档

- 功能矩阵（跨语言一致性的单一事实来源）：[`sdks/SDK_FEATURE_MATRIX.md`](../SDK_FEATURE_MATRIX.md)
- 线协议约定：[`docs/architecture/sdk-wire-protocol.md`](../../docs/architecture/sdk-wire-protocol.md)
- 统一文档站入口：`/docs/sdks/js/`
- 仓库内路径：`docs/sdks/js`

## 主项目

| 项目         | 描述                 | 链接                                                              |
| ------------ | -------------------- | ----------------------------------------------------------------- |
| **Croupier** | 游戏后端平台主项目    | [cuihairu/croupier](https://github.com/cuihairu/croupier)       |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言 | 目录 | CI | Docs |
| --- | --- | --- | --- |
| Go | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml) | [README](sdks/go/README.md) |
| C++ | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml) | [README](sdks/cpp/README.md) |
| Java | [sdks/java/](https://github.com/cuihairu/croupier/tree/main/sdks/java) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml) | [README](sdks/java/README.md) |
| Python | [sdks/python/](https://github.com/cuihairu/croupier/tree/main/sdks/python) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](sdks/python/README.md) |
| C# | [sdks/csharp/](https://github.com/cuihairu/croupier/tree/main/sdks/csharp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](sdks/csharp/README.md) |

## 支持平台

| 平台        | 架构                       | 状态    |
| ----------- | -------------------------- | ------- |
| **Windows** | x64                        | ✅ 支持 |
| **Linux**   | x64, ARM64                 | ✅ 支持 |
| **macOS**   | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

按 [功能矩阵](../SDK_FEATURE_MATRIX.md) 分层：

**L1 Core Provider（必备）**

- 🛰️ **TCP session 客户端** - 单条 `sdk-agent subprotocol` 长连接，不监听本地端口
- 🤝 **握手与心跳** - `ProviderConnectRequest` 协商，可配置心跳间隔
- 🔁 **自动重连** - 指数退避 + jitter
- 📦 **处理器注册** - 强类型描述符，handler 签名 `(context: string, payload: string) => Promise<string> | string`
- 📝 **TypeScript 优先** - 完整类型定义

**L2 Provider 扩展（可选）**

- 🔐 **TLS** - `certFile` / `keyFile` / `caFile` / `serverName`
- 📋 **JSON Schema 元数据** - `input_schema` / `output_schema`
- 📤 **Provider Manifest 上传** - 配置 `controlAddr` 后自动推送

**L3 Invoker**

- ❌ 当前版本未提供独立 Invoker，远程调用请使用平台 HTTP API 或其他语言 SDK

## 快速开始

### 系统要求

- **Node.js** ≥ 22
- **pnpm** ≥ 10（推荐使用 `package.json#packageManager` 指定版本）

### 安装

```bash
pnpm install
pnpm run build
```

### 基础使用

```ts
import { createClient, FunctionDescriptor, FunctionHandler } from "./src";

const config = {
  agentAddr: "127.0.0.1:19090",
  controlAddr: "127.0.0.1:19100", // 可选：上传 provider manifest
  serviceId: "inventory-service",
  serviceVersion: "1.2.3",
};

const client = createClient(config);

const addItem: FunctionHandler = async (_ctx, payload) => {
  const request = JSON.parse(payload);
  // ... 修改状态 ...
  return JSON.stringify({ status: "ok", item_id: request.item_id });
};

const descriptor: FunctionDescriptor = {
  id: "inventory.add_item",
  version: "1.0.0",
  description: "向玩家背包添加物品",
  input_schema: {
    type: "object",
    required: ["player_id", "item_id"],
    properties: {
      player_id: { type: "string" },
      item_id: { type: "string" },
      quantity: { type: "number", default: 1 },
    },
  },
};

await client.registerFunction(descriptor, addItem);
await client.connect();

console.log("✅ inventory.add_item 已注册");
```

## 使用示例

### 运行示例应用

```bash
# 在项目根目录下
pnpm install
pnpm dev
```

示例注册三个处理器（`player.ban`、`wallet.transfer`、`shop.buy`）并记录调用日志。默认连接到 `127.0.0.1:19090` 的 Agent。

### 函数描述符

```ts
const descriptor: FunctionDescriptor = {
  id: "player.ban", // 函数 ID
  version: "1.0.0", // 语义化版本
  description: "封禁玩家", // 描述
  input_schema: {
    // JSON Schema（可选）
    type: "object",
    required: ["player_id", "reason"],
    properties: {
      player_id: { type: "string" },
      reason: { type: "string" },
      duration: { type: "number" },
    },
  },
};
```

### 函数处理器

```ts
const handler: FunctionHandler = async (context, payload) => {
  // context: 执行上下文
  // payload: JSON 字符串载荷
  const data = JSON.parse(payload);

  // 处理业务逻辑...

  return JSON.stringify({ status: "success" });
};
```

## 架构设计

### 数据流

```
Game Server → Node.js SDK (Provider) → Agent → Croupier Server → Web UI
                                       ↑
                          单条 TCP session（sdk-agent subprotocol）
```

SDK 是 `sdk-agent subprotocol` 上的 Provider 端：

1. 拨号到 Agent，发送 `ProviderConnectRequest`，接收 `ProviderConnectResponse`
2. 周期性发送 `ProviderHeartbeatRequest`
3. 接收 `InvokeRequest`，调用 handler 后回填 `InvokeResponse`
4. 可选：通过 `controlAddr` 向控制面推送 manifest

### 项目结构

```
croupier-sdk-js/
├── src/                # SDK 源码（TypeScript；Protobuf 消息经 protobufjs 内联定义）
├── examples/           # 端到端示例
├── dist/               # tsc 输出
└── package.json
```

## API 参考

### ClientConfig

```ts
interface ClientConfig {
  agentAddr: string; // Agent TCP session 地址（sdk-agent subprotocol）
  controlAddr?: string; // 可选控制面地址（用于 manifest 上传）
  serviceId: string; // 服务标识符
  serviceVersion: string; // 服务版本
  gameId?: string; // 游戏标识符
  env?: string; // 环境（dev/staging/prod）
  insecure?: boolean; // 是否跳过 TLS
}
```

### CroupierClient

```ts
interface CroupierClient {
  // 函数注册
  registerFunction(
    descriptor: FunctionDescriptor,
    handler: FunctionHandler,
  ): Promise<void>;

  // 连接管理
  connect(): Promise<void>;

  // 生命周期
  stop(): Promise<void>;
  close(): Promise<void>;

  // 状态
  isConnected(): boolean;
}
```

## 开发指南

### 构建命令

```bash
# 安装依赖
pnpm install

# 构建
pnpm run build

# 运行测试
pnpm test

# 运行示例
pnpm ts-node examples/main.ts

# 运行完整游戏后台 Demo (19个函数)
pnpm ts-node examples/game_demo.ts
```

### 路线图

- Provider manifest 上传（通过 `ControlService.RegisterCapabilities`）
- 丰富的运行时指标 + 健康探针
- 一流的 CommonJS/ESM 双构建

## 贡献指南

1. 确保所有类型与 proto 定义对齐
2. 为新功能添加测试
3. 更新 API 变更的文档
4. 遵循 TypeScript 编码规范

欢迎贡献 - 如有任何问题，请提交 issue 或 PR！🧑‍💻

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源协议。

---

<p align="center">
  <a href="https://github.com/cuihairu/croupier">🏠 主项目</a> •
  <a href="https://github.com/cuihairu/croupier-sdk-js/issues">🐛 问题反馈</a> •
  <a href="https://github.com/cuihairu/croupier/discussions">💬 讨论区</a>
</p>
