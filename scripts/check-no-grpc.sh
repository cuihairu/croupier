#!/usr/bin/env bash
# gRPC 禁入门禁（docs/grpc-investigation.md）。
#
# 本仓库已废除 gRPC（docs/architecture/transport-no-grpc.md），传输层为自研
# TCP（长度前缀分帧 + protobuf）。本脚本在 CI 阻断 gRPC 经四条路径再次潜入：
#   1) go.mod 出现 gRPC 直接依赖
#   2) Go 源码 import gRPC 包
#   3) proto 出现 service/rpc 定义（gRPC 代码生成的前提）
#   4) 生成链挂 protoc-gen-go-grpc / buf grpc 插件
#
# 已知豁免（fail-open 项，仅提示不阻断）：
#   google.golang.org/grpc / grpc-gateway 以 // indirect 存在于 go.mod，
#   由 otel otlp http exporter 的上游共享配置代码拉入（croupier 运行时走
#   纯 HTTP，无任何 gRPC 通信）。断链方案见调查文档，落地后收紧为完全禁令。
set -uo pipefail
cd "$(dirname "$0")/.."

fail=0

# 1) go.mod：禁止直接依赖（require 行无 // indirect 标记）
direct=$(grep -E '^[[:space:]]+(google\.golang\.org/grpc|github\.com/grpc-ecosystem/grpc-gateway/v2)[[:space:]]' go.mod | grep -v '// indirect' || true)
if [ -n "$direct" ]; then
  echo "FAIL: go.mod 出现 gRPC 直接依赖（禁止引入 gRPC，见 docs/grpc-investigation.md）:"
  echo "$direct"
  fail=1
fi

# 2) Go 源码：禁止 import grpc / grpc-gateway
grpc_imports=$(grep -rlE '"(google\.golang\.org/grpc|github\.com/grpc-ecosystem/grpc-gateway)' \
  --include='*.go' internal pkg cmd sdks examples 2>/dev/null || true)
if [ -n "$grpc_imports" ]; then
  echo "FAIL: Go 源码 import gRPC（禁止引入 gRPC）:"
  echo "$grpc_imports"
  fail=1
fi

# 3) proto：禁止 service/rpc 定义（纯 message 用 protoc-gen-go 即可）
svc=$(grep -rnE '^[[:space:]]*(service|rpc)[[:space:]]' proto --include='*.proto' || true)
if [ -n "$svc" ]; then
  echo "FAIL: proto 出现 service/rpc 定义（禁止 gRPC 服务定义）:"
  echo "$svc"
  fail=1
fi

# 4) 生成链：禁止 gRPC 代码生成插件
gen=$(grep -rniE 'gen-go-grpc|protoc-gen-go-grpc|grpc.*plugin|plugin.*grpc' Makefile scripts/gen-proto.sh proto/buf.yaml 2>/dev/null || true)
if [ -n "$gen" ]; then
  echo "FAIL: 生成链出现 gRPC 插件:"
  echo "$gen"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "PASSED: no-grpc guard passed（无 gRPC 直接依赖 / import / service 定义 / 生成插件）"
  grep -E '^[[:space:]]+(google\.golang\.org/grpc|github\.com/grpc-ecosystem/grpc-gateway/v2)[[:space:]].*// indirect' go.mod \
    | sed 's/^/  note(indirect, otel 上游拉入，运行时纯 HTTP): /'
fi
exit "$fail"
