---
title: 安装指南
icon: download
order: 2
category:
  - 入门指南
tag:
  - 安装
  - 环境配置
---

# 安装指南

本页说明各组件的安装路径。开发环境从源码构建；生产环境按团队规范选择二进制或容器，SDK 则按语言使用源码构建或既有发布包。

## 系统要求

| 组件     | 最低版本                                      |
| -------- | --------------------------------------------- |
| Go       | `1.27.2`                                      |
| Node.js  | `24+`                                         |
| pnpm     | `11+`                                         |
| buf      | 最新稳定版                                    |
| 操作系统 | Linux、macOS、Windows；Linux/macOS 为主要目标 |

开发机建议至少 2 核、4 GB 内存、10 GB 磁盘。生产规模取决于 Agent 数量、函数调用频率、数据库和 ClickHouse 工作负载，不能只用一个固定规格衡量。

## 安装开发依赖

```bash
brew install go buf          # macOS
brew install pnpm
```

或按发行版安装 Go 与构建工具，并确认构建命令可用：

```bash
go version
node -v
pnpm -v
buf --version
```

## 源码构建

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
go mod download
make proto
make build
ls -la bin/
```

`make build` 覆盖 Server、Agent、devops-agent、capture-agent、analytics worker、ingest 和工具类产物。

## 二进制安装

从项目 Release 页下载与操作系统、CPU 架构匹配的产物，校验发布清单后安装到独立目录：

```bash
install -m 0755 croupier-server /usr/local/bin/croupier-server
install -m 0755 croupier-agent /usr/local/bin/croupier-agent
```

不要把示例配置直接当作生产配置；生产参数见[Server 配置](/operations/config-server)与[Agent 配置](/operations/agent-config)。

## Docker 部署

仓库提供 `docker/docker-compose.yml` 作为源码开发栈入口：

```bash
docker compose build
docker compose up -d
docker compose logs -f
```

单镜像构建示例：

```bash
docker build -t croupier-server:local -f docker/Dockerfile.server .
```

生产部署、卷、依赖服务、升级与回滚见 [Docker 部署](/zh/operations/deploy-docker)。

## SDK 安装

| SDK                     | 推荐入口                                                                |
| ----------------------- | ----------------------------------------------------------------------- |
| Go                      | `sdks/go` 源码或 `go get github.com/cuihairu/croupier/sdks/go@latest`。 |
| C++                     | 使用 `sdks/cpp/vcpkg.json` 和 CMake preset 构建。                       |
| Java                    | 使用 Gradle 发布管线或 `publishToMavenLocal`。                          |
| JavaScript / TypeScript | 源码目录 `sdks/js`，使用 pnpm 构建包。                                  |
| Python                  | 源码目录 `sdks/python`，可用 `pip install .`。                          |
| C#                      | 见 `sdks/csharp` 源码目录和对应指南。                                   |

语言级细节以 [SDK 文档](/zh/sdks/)为准。

## 安装后验证

```bash
curl http://localhost:18780/healthz
```

返回 `ok: true` 后，再启动 Agent 并检查注册列表。单元与集成验证可分别使用 `make test` 和 `make test-integration`；接入 SDK 前，先确认 `19090` 与 `19091` 的边界没有混淆。

## 故障排查

| 现象               | 检查点                                                         |
| ------------------ | -------------------------------------------------------------- |
| Go 构建失败        | `go version` 是否不低于 `1.27.2`，`go mod download` 是否完成。 |
| proto 产物缺失     | 是否运行 `make proto`，`buf` 是否在 `PATH`。                   |
| 端口冲突           | 调整配置中的 `18780`、`19090` 或 `19091`，同步更新访问端。     |
| Agent 无法注册     | Server 地址、TLS、`gameId/env`、Agent 凭证和 Server 日志。     |
| Dashboard 无法打开 | `web` 依赖是否安装，Server `18780` 是否健康。                  |
