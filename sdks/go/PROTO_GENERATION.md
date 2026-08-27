# Proto Generation Guide

Croupier 内部 RPC 不使用 gRPC（[传输层决策](../../docs/architecture/transport-no-grpc.md)）：Server ↔ Agent ↔ SDK 走自研 TCP transport（length-prefix framing）+ protobuf 消息。本文档说明 Go SDK 的 protobuf 代码生成流程。

## 生成方式（buf，与主仓库一致）

统一使用 buf 远程插件生成，**只生成 protobuf 消息代码，不生成 gRPC 代码**：

```bash
# 在仓库根目录
make proto          # 主仓库 pkg/pb（transport/agent/SDK 契约消息）
make pack           # pack artifacts（protoc-gen-croupier）

# Go SDK 单独生成（sdks/go/pkg/pb）
cd sdks/go && make proto
```

`sdks/go/buf.gen.yaml` 固定插件版本（与主仓库 CI 一致）：

- `buf.build/protocolbuffers/go:v1.36.11` —— protobuf 消息代码

## 约束

- **不要引入 `protoc-gen-go-grpc` / `buf.build/grpc/*` 插件**——历史上配置过但从未产生被使用的产物，已移除。
- 本地 `protoc` 版本与 buf 远程插件不匹配会生成不兼容代码，一律走 buf。
- 生成的代码在 `sdks/go/pkg/pb/`（git 跟踪），只有 proto 契约变更时才需要重新生成。

## 历史说明

早期 SDK 文档描述过 "Mock gRPC mode / Real gRPC mode" 双模式与 `generate_proto.sh` 脚本——该模型已随 gRPC 移除而废弃，脚本已删除。
