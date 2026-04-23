# 入门指南

欢迎来到 Croupier Go SDK！本指南将帮助你快速上手。

## 系统要求

- **Go 1.20+**
- **Protocol Buffers 编译器** (protoc)
- **Go protoc 插件**:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

## 安装

```bash
go get github.com/cuihairu/croupier/sdks/go
```

## 构建模式

SDK 支持两种构建模式：

### 1. Mock 模式（默认）

无需依赖，适合本地开发：

```bash
go build ./...
go run examples/basic/main.go
```

### 2. 真实 gRPC 模式

需要生成 proto 代码：

```bash
# 生成 gRPC 代码
./generate_proto.sh

# 或使用 Makefile
make build-with-grpc
```

详见 [PROTO_GENERATION.md](https://github.com/cuihairu/croupier-sdk-go/blob/main/PROTO_GENERATION.md)。

## 目录结构

```
croupier-sdk-go/
├── pkg/croupier/      # SDK 核心包
├── examples/          # 示例程序
├── scripts/           # 构建脚本
└── go.mod             # Go 模块定义
```

## 下一步

- [主线程调度器](./threading.md) - gRPC 回调线程安全处理
