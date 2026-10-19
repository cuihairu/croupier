#!/usr/bin/env bash
# gRPC 依赖边界守卫（2026-10-17 更新后的裁决，
# docs/archive/legacy-en/architecture/transport-no-grpc.md）。
#
# gRPC 不再全局禁用：插件运行时（todo:36 探测、todo:35 sidecar 函数适配）
# 允许插件宿主与其插件使用 HashiCorp go-plugin 的本地 gRPC IPC。本脚本
# 从旧「全局禁用」四连检改为「依赖边界」守卫，只检查一件事实：
# gRPC import 是否逃出插件宿主边界。
#
#   1) 全仓 Go 源码：grpc 只允许出现在插件宿主子树
#      （agents/synthetics/pluginhost/、agents/sidecar/funcplugin/ 与
#      共享宿主引擎 core/pluginhost/，含各自生成代码与示例插件）；
#      游戏 SDK（pkg/sdks/core/internal/cmd）、其他 agent、服务端一律禁止。
#   2) 顶层 proto/：Agent↔Server↔SDK 的 TCP 自研传输契约仍禁止
#      service/rpc 定义（该契约与插件 IPC 契约分轨版本化；
#      插件 proto 位于各插件宿主的 protocol/ 目录）。
#   3) 生成链：Makefile / scripts/gen-proto.sh / proto/buf.yaml 禁止
#     gRPC 生成插件（插件契约的生成独立于主 proto 生成链）。
#
# go.mod 中 google.golang.org/grpc 的直接依赖是宿主边界的必然结果，
# 不再视为违规（fail-open 提示）。
set -uo pipefail
cd "$(dirname "$0")/.."

fail=0
PLUGINHOST_BOUNDARY='^(agents/synthetics/pluginhost/|agents/sidecar/funcplugin/|core/pluginhost/)'

# 1) 依赖边界：grpc / grpc-gateway import 只允许在插件宿主子树
hits=$(grep -rlE '"(google\.golang\.org/grpc|github\.com/grpc-ecosystem/grpc-gateway)' \
  --include='*.go' agents cmd core internal pkg server tools sdks examples mobile 2>/dev/null \
  | grep -Ev "$PLUGINHOST_BOUNDARY" || true)
if [ -n "$hits" ]; then
  echo "FAIL: gRPC import 逃出插件宿主边界（仅 synthetics pluginhost、sidecar funcplugin 和 core/pluginhost 子树允许，见 transport-no-grpc.md）:"
  echo "$hits"
  fail=1
fi

# 2) 顶层 proto/：传输契约禁止 service/rpc 定义（纯 message 即可）
svc=$(grep -rnE '^[[:space:]]*(service|rpc)[[:space:]]' proto --include='*.proto' || true)
if [ -n "$svc" ]; then
  echo "FAIL: 主 proto 出现 service/rpc 定义（Agent↔Server↔SDK 传输契约禁止 gRPC 服务定义）:"
  echo "$svc"
  fail=1
fi

# 3) 主生成链：禁止 gRPC 代码生成插件
gen=$(grep -rniE 'gen-go-grpc|protoc-gen-go-grpc' Makefile scripts/gen-proto.sh proto/buf.yaml 2>/dev/null || true)
if [ -n "$gen" ]; then
  echo "FAIL: 主 proto 生成链出现 gRPC 插件:"
  echo "$gen"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "PASSED: no-grpc boundary guard passed（gRPC 仅存在于插件宿主边界内）"
  grep -E '^[[:space:]]+(google\.golang\.org/grpc)[[:space:]]' go.mod \
    | sed 's/^/  note(grpc direct, synthetics 插件宿主边界内的必然依赖): /'
fi
exit "$fail"
