---
home: true
title: 首页
heroImage: /logo.png
heroText: Croupier
tagline: 游戏运营控制面与 Agent 协同平台
actions:
  - text: 快速开始 →
    link: /guide/quick-start.html
    type: primary
  - text: 开发约定
    link: /development/
    type: secondary
features:
  - title: 控制面与 Agent 协同
    details: Server 负责权限、审批、审计、配置和函数路由，Agent 负责接入游戏服与节点能力。
  - title: 当前入口清晰
    details: 所有可执行程序统一位于 cmd/，配置文件统一位于 configs/，Docker 镜像统一位于 docker/。
  - title: API 与描述符驱动
    details: OpenAPI、JSON Schema、函数 UI 配置与扩展安装绑定共同驱动管理界面与调用流程。
  - title: 遥测链路解耦
    details: Ingest、analytics-worker、Redis 与 ClickHouse 组成独立分析链路，不与控制面强耦合。
footer: Apache-2.0 License | Copyright © 2024-present Croupier
---

## 文档分层

- `guide/`: 上手、配置、部署
- `architecture/`: 架构分层与数据流
- `api/`: REST/API 能力说明
- `analytics/`: 采集、处理、指标与分析链路
- `development/`: 仓库结构、开发命令、约定

## 快速开始

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
go mod download
make build
./bin/croupier-server --config configs/server.yaml
./bin/croupier-agent --config configs/agent.yaml
```

更多内容从这里进入：

- [使用指南](/guide/)
- [架构文档](/architecture/)
- [API 参考](/api/)
- [分析系统](/analytics/)
- [开发文档](/development/)
