# Croupier Platform

Croupier 是一套面向游戏运营的后端控制面：统一的 Server、可插拔的 Agent、X-Render 驱动的 Dashboard 以及多语言 SDK。随着组件拆分为独立仓库，这个主仓库主要承载 **Server/Agent/Edge 服务及公共配置**，其余部分以子模块或独立仓库提供。

## 仓库导航
| 组件 | 仓库 | 在本仓库中的位置 | 说明 |
| --- | --- | --- | --- |
| **Server / Agent / Edge** | 本仓库（`cuihairu/croupier`） | 根目录 | 控制面、代理、函数/审批/审计等全部后端服务 |
| **Dashboard（Web 管理后台）** | [cuihairu/croupier-dashboard](https://github.com/cuihairu/croupier-dashboard) | `dashboard/` 子模块 | Umi Max + Ant Design + X-Render 的运营后台；文档见该仓库 README |
| **Proto 定义** | [cuihairu/croupier-proto](https://github.com/cuihairu/croupier-proto) | `proto/` 子模块 | 所有 gRPC/HTTP 接口 IDL，供 Server 与 SDK 共享 |
| **Analytics Worker** | `services/analytics-worker` | 本仓库 | 日志/指标/事件消费与入库 |
| **示例/工具** | `examples/`, `tools/`, `packs/` | 本仓库 | Demo 游戏、Telemetry、脚本等 |

## SDK 一览
| 语言 | 仓库链接 | 子模块路径 | Nightly 构建 |
| --- | --- | --- | --- |
| Go | [cuihairu/croupier-sdk-go](https://github.com/cuihairu/croupier-sdk-go) | `sdks/go` | [![nightly](https://github.com/cuihairu/croupier-sdk-go/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-go/actions/workflows/nightly.yml) |
| C++ | [cuihairu/croupier-sdk-cpp](https://github.com/cuihairu/croupier-sdk-cpp) | `sdks/cpp` | [![nightly](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/nightly.yml) |
| Java | [cuihairu/croupier-sdk-java](https://github.com/cuihairu/croupier-sdk-java) | `sdks/java` | [![nightly](https://github.com/cuihairu/croupier-sdk-java/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-java/actions/workflows/nightly.yml) |
| JavaScript/TypeScript | [cuihairu/croupier-sdk-js](https://github.com/cuihairu/croupier-sdk-js) | `sdks/js` | [![nightly](https://github.com/cuihairu/croupier-sdk-js/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-js/actions/workflows/nightly.yml) |
| Python | [cuihairu/croupier-sdk-python](https://github.com/cuihairu/croupier-sdk-python) | `sdks/python` | [![nightly](https://github.com/cuihairu/croupier-sdk-python/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-python/actions/workflows/nightly.yml) |

> SDK 仓库已独立维护 README、版本发布与 Nightly 构建。服务端与 SDK 协议通过 `croupier-proto` 子模块共享。

## 文档入口
- [AGENTS.md](AGENTS.md)：本仓库开发规范（Go 结构、前后端分层、命名约定、CI 说明）
- [docs/](docs/)：架构说明、配置样例、部署建议
- [configs/](configs/)：示例配置（包括 server/agent/edge、RBAC、审核流程等）
- [proto/](proto/)：IDL 与 `buf` 配置，运行 `buf lint`/`buf generate` 获取所有语言 stub
- [dashboard/README.md](dashboard/README.md)：Dashboard/Web 管理界面文档（开发、部署、X-Render 用法）
- 各 SDK `README`：语言特定的安装与集成指南

## 快速起步
1. **克隆并初始化子模块**
   ```bash
   git clone git@github.com:cuihairu/croupier.git
   cd croupier
   git submodule update --init --recursive
   ```
2. **安装工具链**：参考 [AGENTS.md](AGENTS.md)（Go 1.25+、pnpm、buf、protoc 等）。
3. **生成协议与构建**：`make dev` 会拉取 proto / 生成 go 代码 / 构建二进制。
4. **启动后端**：参阅 `configs/server.example.yaml`、`configs/agent.example.yaml`，执行 `./bin/croupier-server`、`./bin/croupier-agent`。
5. **运行 Dashboard**：进入 `dashboard/`，按照该仓库 README 运行 `pnpm dev` 或 `pnpm build`。
6. **SDK 集成**：挑选对应语言的 SDK 仓库，参考其 README 完成依赖与初始化。

## 社区与贡献
- 提交 PR 前请阅读 [AGENTS.md](AGENTS.md) 内的代码规范与流程。
- Server/Agent 改动请附带 `make test`/`make lint` 结果；SDK 改动在各语言仓库提 PR。
- Dashboard、SDK、Proto 团队使用独立的 issue/PR 列表，欢迎根据组件在对应仓库讨论。

如需更详细的设计背景（架构图、数据流、审批机制等），请查看 `docs/` 目录以及 Dashboard/SDK 的 README。欢迎在 Issues 中交流反馈。
