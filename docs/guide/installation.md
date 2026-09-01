---
title: 安装指南
icon: download
order: 2
category:
  - 入门指南
tag:
  - 安装
  - 环境配置
---

# 安装指南

本指南详细介绍 Croupier 各组件的安装和配置方法。

## 目录

[[toc]]

## 系统要求

### 最低要求

| 组件     | 最低版本            | 推荐版本    |
| -------- | ------------------- | ----------- |
| Go       | 1.26                | 1.26+       |
| Node.js  | 22                  | 22+         |
| pnpm     | 10                  | 10.22+      |
| buf      | -                   | latest      |
| 操作系统 | Linux/macOS/Windows | Linux/macOS |

### 硬件要求

| 场景           | CPU   | 内存   | 磁盘    |
| -------------- | ----- | ------ | ------- |
| 开发           | 2 核  | 4 GB   | 10 GB   |
| 生产（小规模） | 4 核  | 8 GB   | 50 GB   |
| 生产（大规模） | 8 核+ | 16 GB+ | 100 GB+ |

## 开发环境安装

### 1. 安装 Go

#### macOS

```bash
# 使用 Homebrew
brew install go

# 验证安装
go version
```

#### Linux (Ubuntu/Debian)

```bash
# 下载并安装
wget https://go.dev/dl/go1.26.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.6.linux-amd64.tar.gz

# 配置 PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

#### Windows

从 [Go 官网](https://go.dev/dl/) 下载安装程序并运行。

### 2. 安装 Node.js 和 pnpm

```bash
# 使用 fnm 安装 Node.js（推荐）
curl -fsSL https://fnm.vercel.app/install | bash
fnm install 22
fnm use 22

# 安装 pnpm
npm install -g pnpm

# 验证安装
node -v
pnpm -v
```

### 3. 安装 buf

```bash
# 使用 go install
go install github.com/bufbuild/buf/cmd/buf@latest

# 或使用 Homebrew（macOS）
brew install bufbuild/buf/buf

# 验证安装
buf --version
```

### 4. 克隆项目

```bash
# 克隆主仓库
git clone https://github.com/cuihairu/croupier.git
cd croupier

# 初始化子模块
git submodule update --init --recursive
```

## 生产环境安装

### Docker 部署

#### 使用 Docker Compose

```bash
# 构建镜像
docker compose build

# 启动服务
docker compose up -d

# 查看日志
docker compose logs -f
```

#### 单独部署

```bash
# 构建 Server 镜像
docker build -t croupier-server:latest -f docker/Dockerfile.server .

# 运行 Server
docker run -d \
  --name croupier-server \
  -p 19090:19090 \
  -p 18780:18780 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  croupier-server:latest
```

### 二进制部署

#### 下载预构建版本

从 [Release 页面](https://github.com/cuihairu/croupier/releases) 下载对应平台的二进制文件。

#### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier.git
cd croupier
git submodule update --init --recursive

# 构建二进制（server + agent）
make build

# 二进制文件位于 bin/ 目录
ls -la bin/
```

## SDK 安装

各语言 SDK 均为本仓库 `sdks/<lang>/` 目录下的源码（安装细节以
[docs/sdks/](../sdks/index.md) 各语言指南为准）：

### Go SDK

```bash
go get github.com/cuihairu/croupier/sdks/go@latest
```

### C++ SDK

```bash
# 从本仓库源码构建（vcpkg 依赖清单见 sdks/cpp/vcpkg.json）
cd sdks/cpp
cmake --preset linux-x64-release-vcpkg
cmake --build --preset linux-x64-release
```

### Java SDK

```bash
# 从本仓库源码构建（产物发布规划中，暂未上 Maven Central）
cd sdks/java && ./gradlew publishToMavenLocal
```

### JavaScript/TypeScript SDK

```bash
# 包名 croupier-js-sdk（源码构建）
cd sdks/js && pnpm install && pnpm build
```

### Python SDK

```bash
# 包名 croupier-sdk（源码构建）
cd sdks/python && pip install .
```

## 配置文件

### Server 配置

```yaml
server:
  host: 0.0.0.0
  port: 18780 # HTTP 监听地址
  control:
    addr: ":19090" # Control（agent 接入）监听地址
database:
  # 支持 sqlite、mysql、postgres、sqlserver
  driver: postgres
  dataSource: "postgres://user:pass@localhost:5432/croupier"
  # SQLite 示例：
  # driver: sqlite
  # dataSource: "data/croupier.db"
  # MySQL 示例：
  # driver: mysql
  # dataSource: "user:pass@tcp(localhost:3306)/croupier_meta?charset=utf8mb4&parseTime=True"
log:
  level: info
  format: json
```

### Agent 配置

```yaml
name: croupier-agent
host: 0.0.0.0
port: 19091

server:
  addr: "localhost:19090"
  insecure: false

agent:
  gameId: "my-game"
  env: "dev"
  localAddr: "localhost:19091"

outboundTLS:
  enabled: true
  caFile: "data/ca.crt"
  certFile: "data/agent.crt"
  keyFile: "data/agent.key"
```

## 验证安装

### 健康检查

```bash
# Server 健康检查
curl http://localhost:18780/healthz

# 预期输出
{"status":"ok"}
```

### 功能测试

```bash
# 运行单元测试
make test

# 运行集成测试
make test-integration
```

## 故障排查

### 常见问题

<details>
<summary>Go 版本不兼容</summary>

确保 Go 版本为 1.26 或更高：

```bash
go version
```

如果不满足，请升级 Go 版本。

</details>

<details>
<summary>子模块为空</summary>

重新初始化子模块：

```bash
git submodule deinit --all
git submodule update --init --recursive
```

</details>

<details>
<summary>buf 命令未找到</summary>

确保 `$GOPATH/bin` 在 `PATH` 中：

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

</details>

<details>
<summary>端口已被占用</summary>

修改配置文件中的端口：

```yaml
control:
  addr: ":19092"
port: 19080
```

</details>

## 下一步

安装完成后，请参考：

- [配置指南](../operations/config-server.md) - 详细配置选项
- [部署指南](../operations/deploy-docker.md) - 生产环境部署
- [核心概念](./concepts/overview.md) - 了解系统设计
