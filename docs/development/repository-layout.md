---
title: 仓库结构
---

# 仓库结构

当前仓库的有效分层如下：

```text
cmd/                可执行入口，包含 server、agent、analytics-worker、ingest 等
internal/           主要业务实现与共享内部包
pkg/                少量对外可复用包
configs/            运行配置、RBAC、种子数据
docker/             Dockerfile 与 compose 配置
docs/               VuePress 文档
descriptors/        示例描述文件
schemas/            UI schema 与相关配置
proto/              protobuf 定义
scripts/            构建、安装、同步脚本
tools/              辅助工具与适配器
examples/           示例程序
sdks/               多语言 SDK 仓库
dashboard/          前端控制台仓库
```

几个重要约定：

- 所有新二进制入口放在 `cmd/`，不要再引入 `services/*` 风格目录。
- Server/Agent 的共享业务逻辑继续放在 `internal/`，按功能域组织。
- 配置统一从 `configs/` 读取，脚本、Docker、文档都应引用这里。
- 文档只保留当前结构对应的说明，迁移过程文档不再作为长期文档维护。
