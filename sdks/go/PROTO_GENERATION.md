# Proto Generation Guide

Croupier 内部 RPC 不使用 gRPC（[传输层决策](../../docs/architecture/transport-no-grpc.md)）：Server ↔ Agent ↔ SDK 走自研 TCP transport（length-prefix framing）+ protobuf 消息。本文档说明 Go SDK 的 protobuf 代码生成流程。

## 生成方式（唯一入口：仓库根 `make proto`）

Go SDK 是**独立 Go module**，它的 `pkg/pb/` 与主仓库 `pkg/pb/` 是同一批 proto 的两份生成产物（`go_package` 前缀不同）。两份都由仓库根的 `scripts/gen-proto.sh` 一次生成：

```bash
# 在仓库根目录（不要 cd sdks/go）
make proto            # = scripts/gen-proto.sh
make pack             # pack artifacts（protoc-gen-croupier）
```

产物：

| 目标 | 路径 | go_package |
| --- | --- | --- |
| 主仓库 | `pkg/pb/` | `github.com/cuihairu/croupier/pkg/pb/...`（proto 里的原值） |
| Go SDK | `sdks/go/pkg/pb/` | `github.com/cuihairu/croupier/sdks/go/pkg/pb/...`（脚本改写前缀） |
| Python SDK | `sdks/python/generated/` | — |
| C++ SDK | `sdks/cpp/generated/` | — |

Go SDK 那份的做法是「复制 proto 树 → 改写副本里的 `option go_package` 前缀 → 对副本跑 protoc」：protoc 的 `M<file>=<path>` flag 只影响跨文件 import，**不会**改写嵌入 descriptor 的 `go_package`，所以必须改 option 本身。脚本在任一 proto 缺少/不匹配该前缀时直接退出。

依赖与版本：本地 `protoc` 必须是 34.x（Python/C++ gencode 内嵌运行时版本，跨大版本降级会让 CI 编译/运行失败），`protoc-gen-go` 与 protobuf 4.25.x API 对齐（v1.36.11+）。生成文件头记录了实际工具版本，比对时以头注释为准。

## 约束

- **不要引入 `protoc-gen-go-grpc` / `buf.build/grpc/*` 插件**——历史上配置过但从未产生被使用的产物，已移除。
- **不要只改 proto 而漏掉 `sdks/go/pkg/pb/`**：Go SDK 是独立 module，CI「Go SDK」直接编译它，缺字段会以 `unknown field ... in struct literal` 形式失败。
- 生成的代码在 `sdks/go/pkg/pb/`（git 跟踪），只有 proto 契约变更时才需要重新生成。

## 历史说明

早期 SDK 文档描述过 "Mock gRPC mode / Real gRPC mode" 双模式与 `generate_proto.sh` 脚本——该模型已随 gRPC 移除而废弃，脚本已删除。

`sdks/go/buf.gen.yaml` + `cd sdks/go && make proto`（buf 托管模式 + 远程插件）曾用于生成 `sdks/go/pkg/pb/`：它需要网络访问 buf registry，且 `out: sdks/go/pkg/pb` 相对 `cd sdks/go` 的路径是错的。生成入口已统一到仓库根 `make proto`，该 buf 配置仅作历史参考，不再作为生成路径。
