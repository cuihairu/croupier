---
title: 仓库结构
---

# 仓库结构

当前仓库的有效分层如下：

```text
cmd/                可执行入口，包含 server、agent、analytics-worker、ingest 等
core/               Agent 公共运行时：传输、注册、探测、配置、审计等按需组合
agents/             devops/capture 角色业务与共用 controlconn；synthetics 待实现
internal/           主要业务实现与共享内部包
pkg/                少量对外可复用包
configs/            运行配置、RBAC、种子数据
docker/             Dockerfile 与 compose 配置
docs/               VitePress 文档
descriptors/        示例描述文件
proto/              protobuf 定义
scripts/            构建、安装、同步脚本
tools/              辅助工具与适配器
examples/           示例程序
sdks/               多语言 SDK 仓库
web/                前端控制台仓库（monorepo 内）
```

几个重要约定：

- 所有新二进制入口放在 `cmd/`，不要再引入 `services/*` 风格目录。
- Agent 公共能力放 `core/`，独立角色业务放 `agents/<role>`；`core/` 与 `agents/` 的生产代码不得 import `internal/`，Server 可消费公共件。
- sidecar 的现有业务仍在 `internal/app/agent` 与 `internal/agent`，后续按职责迁移到 `agents/sidecar`；目录现状、公共能力消费与目标树见 [Agent 运行时目录边界](agent-runtime-layout.md)。
- 配置统一从 `configs/` 读取，脚本、Docker、文档都应引用这里。
- 文档只保留当前结构对应的说明，迁移过程文档不再作为长期文档维护。
- SDK 代码在 `sdks/<lang>`，正式文档统一放在 `docs/sdks/<lang>`。
