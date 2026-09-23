# gRPC 残留调查与禁入门禁

状态：**调查处置完成（遗留 mock 已清、门禁已挂），indirect 依赖链处置待拍板**。

铁律：**禁止引入 gRPC**——今后任何代码、依赖、设计不得新增 gRPC。仓库传输层为自研 TCP（长度前缀分帧 + protobuf），废除 gRPC 的决策与历史见 `docs/architecture/transport-no-grpc.md`。

## 结论摘要

| 检查面                     | 结论                                                                                                                                             | 处置               |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------ |
| 运行时通信                 | **零 gRPC**。server/agent/SDK 全部走自研 TCP + protobuf，无任何 gRPC 连接                                                                        | 无需处置           |
| Go 源码 import             | **零直接 import**（internal/pkg/cmd/sdks/examples 全查）                                                                                         | 无需处置           |
| `internal/mocks` 遗留 mock | gRPC 时代的 `MockGRPCClient` 及 12 个自测用例，无任何业务引用方                                                                                  | **本次已删**       |
| proto 生成链               | 干净：`gen-proto.sh` 只挂 `protoc-gen-go`，`buf.gen.yaml` 只有 go 插件，proto 无 service/rpc 定义                                                | 无需处置           |
| buf lint 配置              | `buf.yaml` 曾为 RPC 规则设 except（历史残留）                                                                                                    | **本次已清**       |
| go.mod 依赖                | `google.golang.org/grpc v1.83.2`、`grpc-gateway/v2 v2.30.0` 均 `// indirect`，由 otel OTLP HTTP exporter 上游拉入                                | **待拍板**（见下） |
| 六语言 SDK 源码            | 零 gRPC（`sdks/cpp/build/` 下命中为 vcpkg 安装的 absl 第三方头文件注释，非本项目代码）                                                           | 无需处置           |
| 文档                       | 全部为历史/否定/外部组件语境，受 `check-docs-terms.sh` baseline 管控；能力矩阵中的 gRPC 引用是「替换候选不可行」的否定对比，**无建议引入的结论** | 无需处置           |

## 依赖链：grpc 为何还在 go.mod

```
go mod why -m google.golang.org/grpc
github.com/cuihairu/croupier/internal/telemetry
  └─ go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
       └─ .../otlpmetrichttp/internal/oconf        ← otel 上游共享配置代码
            └─ google.golang.org/grpc              ← 传递依赖

go mod why -m github.com/grpc-ecosystem/grpc-gateway/v2
internal/telemetry → otlpmetrichttp → otel 的 otlp proto 生成物 → grpc-gateway/runtime
```

要点：

- 引入者是 **otel 的 OTLP HTTP exporter**（`internal/telemetry/provider.go` 用 `otlpmetrichttp.New` / `otlptracehttp.New`，两个都是纯 HTTP 客户端）。grpc 包只被 otel 上游的共享配置代码（`internal/oconf`）和 otlp proto 的 gateway 类型引用——**运行时数据面走 net/http，不建立任何 gRPC 连接**。
- 这是 otel 社区的已知上游问题（HTTP 形态 exporter 仍传递依赖 grpc）。croupier 侧无法在不放弃 OTLP 功能的前提下断开。
- OTLP 推送是**可选功能且默认关闭**（`TelemetryConfig.Enabled` 零值 false，需显式配置或 `OTEL_ENABLED`/`OTEL_EXPORTER_OTLP_ENDPOINT` 环境变量）；metrics 的 Prometheus pull 出口（`telemetry.prometheus`，`TelemetryPrometheusConfig`）已作为不依赖 Collector 的替代路径存在，两者互不影响。

## 使用点清单（全量核查记录)

1. **`internal/mocks/grpc_client.go`（已删）**：gRPC 时代的 `MockGRPCClient`，mock 的接口形状是旧 pb client（Invoke/StartTask/StreamEvents/CancelTask）。全仓 grep 确认除 mocks 包自测外零引用——传输层换自研 TCP 后它 mock 的对象已不存在。随删：`grpc_client_cancel_error_test.go` 及 `mocks_test.go`（4 例）、`mocks_extra_test.go`（7 例）中的 mock 自测用例，共 **-12 用例**（均为「测遗留 mock 本身」，无业务回归价值；`MockFunctionStore`/`MockServiceContext` 及其用例保留）。
2. **`internal/platform/tlsutil/tlsutil.go`（注释，保留）**：交代「原 grpc/credentials helpers 是 gRPC 时代的死代码，已随废除清除」——历史说明，正是防复发的记忆点。
3. **`internal/telemetry/provider.go`（保留，待拍板）**：OTLP HTTP exporter 初始化，grpc indirect 依赖的唯一来源。
4. **能力矩阵 `docs/agent-capability-library-matrix.md`（保留）**：gRPC 出现在「50 行分帧是否该换轮子」的论证里，语境是「任何第三方分帧（gRPC framing 等）都是协议变更，成本全 SDK 重写，收益为负」——否定引入，非建议。
5. **`docs/analytics/opentelemetry-integration.md`（保留）**：描述**外部 OTel Collector** 同时支持 OTLP/gRPC（4317）与 HTTP（4318）接收端口；croupier 侧文档与推荐配置只用 HTTP（4318）。
6. **`proto/buf.yaml`（已清）**：删除 RPC_* 三条 lint except；proto 只定义 message，STANDARD 集合全量启用，`buf lint` 通过。

## 防再引入门禁

`scripts/check-no-grpc.sh`，已挂 CI - Core（`Check no gRPC` step）。四条阻断路径：

1. go.mod 出现 gRPC 直接依赖（require 行无 `// indirect`）
2. Go 源码 import `google.golang.org/grpc` / `grpc-gateway`
3. proto 出现 `service`/`rpc` 定义（gRPC 代码生成的前提）
4. 生成链出现 `protoc-gen-go-grpc` 等插件（Makefile / gen-proto.sh / buf.yaml）

负例自证已过：注入临时 import 与临时 proto service 定义均 FAIL，移除后恢复 PASS。当前对两条 indirect 仅提示不阻断；断链落地后收紧为完全禁令（删除 fail-open 提示即可）。文档术语面由 `check-docs-terms.sh` + baseline 继续管控，本文件自身按调查报告语境登记 baseline。

## 待拍板：indirect 依赖链是否断

| 选项      | 内容                                                                                          | 代价                                                                                                                    | 收益                                                                                                           |
| --------- | --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| 1（建议） | 维持现状：OTLP 保留为可选功能，门禁禁直接依赖 + indirect 仅提示                               | 零                                                                                                                      | go.mod 里的 grpc 是死传递依赖（不建立连接、不参与编译产物行为），攻击面/维护面为零；等 otel 上游拆分后自动消失 |
| 2         | 移除 OTLP 推送：metrics 走既有 Prometheus pull；trace 导出改 zipkin exporter（纯 HTTP）或砍掉 | `provider.go` 重写 + config 语义变更 + 文档同步 + 回归，约 1 天；OTLP 是可观测行业标准，接自建 Collector 的部署方受影响 | go.mod 完全无 grpc/grpc-gateway，门禁可收紧为完全禁令                                                          |

若选 2，同 PR 完成门禁收紧与本文档更新。
