#!/bin/bash
# 安装本地protoc插件，避免buf速率限制

set -e

echo "🔧 安装本地protoc插件以避免buf速率限制..."

# 检测操作系统
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    echo "📦 在macOS上安装插件..."

    # 安装基础工具
    if ! command -v protoc &> /dev/null; then
        echo "安装protobuf..."
        brew install protobuf
    fi

    if ! command -v grpc_cpp_plugin &> /dev/null; then
        echo "安装gRPC..."
        brew install grpc
    fi

    # 安装Go插件
    echo "安装Go protoc插件..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

    # 安装Python插件
    echo "安装Python grpc插件..."
    pip3 install grpcio-tools

    # 安装JavaScript插件
    echo "安装JavaScript插件..."
    npm install -g @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es

elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    echo "📦 在Linux上安装插件..."

    # Ubuntu/Debian
    if command -v apt-get &> /dev/null; then
        sudo apt-get update
        sudo apt-get install -y protobuf-compiler libgrpc++-dev

        # 安装Go插件
        go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
        go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

        # 安装Python插件
        pip3 install grpcio-tools

        # 安装JavaScript插件
        npm install -g @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es
    fi
fi

echo "✅ 本地插件安装完成！"
echo "现在可以使用本地protoc而不依赖buf远程插件"