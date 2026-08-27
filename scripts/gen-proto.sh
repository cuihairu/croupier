#!/usr/bin/env bash
# 本地 protobuf 生成（脱离 buf registry）：
#   - Go:    protoc-gen-go v1.36.11（go install，对齐 protobuf 4.25.x API）
#   - Python/C++: protoc v34.1 内置生成器（python 运行时 6.33.5 / cpp 运行时
#     由 CMake find_package 决定，均向后兼容 gencode）
# 依赖：protoc(34.x)、protoc-gen-go 在 PATH；缺失时给出安装指引并退出。
set -euo pipefail
cd "$(dirname "$0")/.."

command -v protoc >/dev/null || { echo "缺少 protoc：从 https://github.com/protocolbuffers/protobuf/releases 安装 34.x"; exit 1; }
command -v protoc-gen-go >/dev/null || { echo "缺少 protoc-gen-go：go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"; exit 1; }

PROTO_VERSION="$(protoc --version)"
echo "[gen] using ${PROTO_VERSION}"

# proto/ 是 buf module 根（proto/buf.yaml），proto 文件内部引用 croupier/... 前缀
cd proto

find . -name '*.proto' | sort > /tmp/gen_proto_files.txt
echo "[gen] $(wc -l < /tmp/gen_proto_files.txt) proto files"

# Go（主仓库 pkg/pb）
protoc -I. \
  --plugin=protoc-gen-go="$(command -v protoc-gen-go)" \
  --go_out=../pkg/pb --go_opt=paths=source_relative \
  $(cat /tmp/gen_proto_files.txt)

# Python SDK
protoc -I. --python_out=../sdks/python/generated $(cat /tmp/gen_proto_files.txt)

# C++ SDK
protoc -I. --cpp_out=../sdks/cpp/generated $(cat /tmp/gen_proto_files.txt)

echo "[gen] done: pkg/pb, sdks/python/generated, sdks/cpp/generated"
