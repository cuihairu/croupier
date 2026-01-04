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

| 组件 | 最低版本 | 推荐版本 |
|------|----------|----------|
| Go | 1.25 | 1.25+ |
| Node.js | 22 | 22+ |
| pnpm | 10 | 10.22+ |
| buf | - | latest |
| 操作系统 | Linux/macOS/Windows | Linux/macOS |

### 硬件要求

| 场景 | CPU | 内存 | 磁盘 |
|------|-----|------|------|
| 开发 | 2 核 | 4 GB | 10 GB |
| 生产（小规模） | 4 核 | 8 GB | 50 GB |
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
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz

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
docker-compose build

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

#### 单独部署

```bash
# 构建 Server 镜像
docker build -t croupier-server:latest -f docker/Dockerfile.server .

# 运行 Server
docker run -d \
  --name croupier-server \
  -p 8443:8443 \
  -p 8080:8080 \
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

# 构建发布版本
make build-release

# 二进制文件位于 bin/ 目录
ls -la bin/
```

## SDK 安装

### Go SDK

```bash
go get github.com/cuihairu/croupier-sdk-go@latest
```

### C++ SDK

```bash
# 使用 vcpkg
vcpkg install croupier-sdk-cpp

# 或从源码构建
cd sdks/cpp
mkdir build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release
make install
```

### Java SDK

```xml
<dependency>
    <groupId>com.github.cuihairu</groupId>
    <artifactId>croupier-sdk-java</artifactId>
    <version>LATEST</version>
</dependency>
```

### JavaScript/TypeScript SDK

```bash
# npm
npm install @croupier/sdk-js

# pnpm
pnpm add @croupier/sdk-js

# yarn
yarn add @croupier/sdk-js
```

### Python SDK

```bash
pip install croupier-sdk-python
```

## 配置文件

### Server 配置

```yaml
server:
  addr: ":8443"              # gRPC 监听地址
  http_addr: ":8080"         # HTTP 监听地址
  tls:
    cert_file: "data/server.crt"
    key_file: "data/server.key"
  db:
    driver: postgres
    datasource: "postgres://user:pass@localhost:5432/croupier"
  log:
    level: info
    format: json
```

### Agent 配置

```yaml
agent:
  server_addr: "localhost:8443"
  local_addr: ":19090"
  game_id: "my-game"
  env: "dev"
  tls:
    ca_file: "data/ca.crt"
    cert_file: "data/agent.crt"
    key_file: "data/agent.key"
```

## 验证安装

### 健康检查

```bash
# Server 健康检查
curl http://localhost:8080/healthz

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

确保 Go 版本为 1.25 或更高：
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
server:
  addr: ":9443"      # 修改为其他端口
  http_addr: ":9080"
```
</details>

## 下一步

安装完成后，请参考：

- [配置指南](./configuration.md) - 详细配置选项
- [部署指南](./deployment.md) - 生产环境部署
- [核心概念](./concepts/overview.md) - 了解系统设计
