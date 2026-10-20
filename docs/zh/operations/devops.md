---
title: DevOps 说明（CI/CD、构建、部署与回滚）
---

# DevOps 说明

面向运维与发布角色，盘点本仓 CI/CD 流程、构建产物、部署方式与回滚手段，逐项给出 file:line 证据，缺失项如实标注。口径核对日期 2026-10-20。

> 路径口径说明：本仓 `docs/operations/` 下部分部署/回滚文档（deploy-overview、deploy-binary、deploy-kubernetes、upgrade-rollback）路径在英文侧但内容为中文，且被站点构建排除（`docs/.vitepress/config.mjs:13-25` 的 `srcExclude` 含 `operations/**`）——这些文件在仓库里可查、在文档站上看不到。本文把其中与 DevOps 直接相关的事实收进 zh 侧可访问页面；原文件仍是细节出处。
>
> 术语消歧：本文的「DevOps」指 CI/CD 与交付面；`agents/devops`（devops-agent，内网远程访问代理）是另一个东西，其堡垒机/跳板能力现状见[权限控制说明](/zh/operations/access-control)。

## CI/CD 流程

`.github/workflows/` 共 26 个 workflow，按职责分组：

| 分组    | 文件                                                                                                               | 触发                                                                |
| ------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------- |
| 核心 CI | `ci.yml`                                                                                                           | push main + PR（paths-ignore 排除纯文档，`:4-25`）                  |
| 专项 CI | `ci-dashboard.yml`、`ci-analytics.yml`、`ci-mobile.yml`、六个 `ci-sdk-*.yml`                                       | push main + PR（按路径过滤）；dashboard 另响应 `v*` tag（`:16-18`） |
| 安全    | `codeql.yml`（push + 每周一 cron `:24`）                                                                           | SAST                                                                |
| 镜像    | `docker.yml`（push main + `v*` tag + 手动，`:4-26`）、`docker-go-sdk-examples.yml`（push main）                    | GHCR 镜像构建                                                       |
| 发布    | `nightly.yml`（cron + 手动，`:4-6`）、`release.yml`（`v*` tag + 手动，`:4-7`）、六个 `release-sdk-*.yml`（仅手动） | 发布面                                                              |
| 部署    | `deploy-self-hosted.yml`（**仅手动**，`:4`）                                                                       | 自托管环境部署                                                      |
| 文档    | `docs-check.yml`（链接/术语守卫）、`docs-pages.yml`（Pages 发布）                                                  | push/PR/手动                                                        |

**CI - Core 的 job 拓扑**：`lint`/`security`/`test` 三轨并行，`e2e`、`migrate-matrix`、`capture-matrix` 均只依赖 `test`（`ci.yml:232`/`:331`/`:432`）。各 job 内容与守卫脚本调用点见[开发周期与发布说明](/zh/development/lifecycle)。

**自托管部署工作流**（`deploy-self-hosted.yml`）：手动触发后由 `self-hosted` runner 执行（`:51-52`）——复制 compose 编排与遥测栈（`:66-67`）、重建 `configs/`（`:74-110`，含 agent 地址收敛到 `haproxy:19090`）、按 inputs 写 `.env` 镜像 tag（`:125-132`，默认 `main`）、GHCR 登录与全量 `docker compose pull`（`:155-188`）、compose 归属校验防误操作（`:213`）、`docker compose up -d --wait --remove-orphans`（`:240`）+ 核心容器 force-recreate（`:249`），并采集 server panic、agent 注册与 dashboard 失败证据（`:220`/`:265`/`:268`）。

## 构建产物

- **二进制**：`make build` 产出 8 个二进制（`Makefile:132` 聚合 `server agents worker ingest tools`）——croupier-server（`:89`）、croupier-agent（`:109`）、devops-agent（`:117`）、capture-agent（`:122`）、synthetics-agent（`:128`，已进 `make agents` 聚合 `:130`）、schema-validator（`:144`）、analytics-worker（`:150`）、ingest（`:157`），全部落在 `bin/`（`Makefile:1`）。
- **归档包**：nightly 每平台总包 `croupier-bin-<os>-<arch>.tar.gz`（`nightly.yml:156`；Windows 为 `.zip` `:167`；ARM 归一为 armv7 `:152`，该面仅 sidecar/devops）+ 单 agent 包 `scripts/package-agents.py`（含二进制、示例配置、AGENTS.md/INSTALL.md、`agents-<os>-<arch>.json` manifest 与 `.sha256`，扩展名规则 `:66`）。
- **上传与发布**：build 产物先上传 artifact 保留 7 天（`nightly.yml:197-202`），release job 汇总 binaries/SDK/dashboard 到 `release-assets/`（`:692-719`）并生成 release notes（`:724` 起，`## Contents` 清单在 `:734`），最后滚动 `nightly` tag（`:814` force push）+ prerelease（`:819-829`）。稳定版由 `release.yml:201-205` 建 GitHub Release（`generate_release_notes: true`）。
- **Docker 镜像**（ghcr.io）：server/agent/ingest/analytics-worker/openapi-demo 五镜像由 `docker.yml` 构建（`:33`/`:129`/`:212`/`:268`/`:322`，Dockerfile 在 `docker/`，tag 触发时 `VERSION="${GITHUB_REF_NAME#v}"` 重注 `:47`），并做 cosign 签名与 SARIF 上报（`supply-chain` job `:393-421`）；dashboard 镜像在 `ci-dashboard.yml:355-417`（显式 `:main` tag `:409`）；六个 SDK 示例镜像在 `docker-go-sdk-examples.yml:31-48`。

## 部署方式

| 形态                      | 文档出处                                                                                                             | 要点                                                                                                           |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Docker Compose（单机/HA） | `docs/zh/operations/deploy-docker.md:34-56`（快速体验五容器 healthy 校验）、`:149`（HA 双实例）                      | 编排在 `docker/docker-compose.yml` 与 `docker-compose.deploy.yml`；镜像 tag 经 `.env` 的 `CROUPIER_*_TAG` 覆盖 |
| 二进制 + systemd          | `docs/operations/deploy-binary.md:24-71`（`/opt/croupier` 目录规划、配置 validate、systemd 单元 `:63-71`）           | 脚本 `scripts/install-systemd.sh`、`install-launchd.sh`、`install-windows-service.ps1`                         |
| Kubernetes                | `docs/operations/deploy-kubernetes.md:60-70`（rolling `maxUnavailable: 0`）、`:142-168`（Agent Deployment 出站 TCP） | 无 helm/kustomize 清单，YAML 内嵌文档（缺失项）                                                                |
| 一键脚本                  | `scripts/install.sh:11-33`（curl 用法与 `--version latest\|nightly\|vX.Y.Z`）                                        | 从 GitHub Release 下载总包（`:440-448`），装到 `/usr/local/bin` 或用户目录（`:452-483`）                       |
| CI 自托管                 | `deploy-self-hosted.yml`（仅手动）                                                                                   | 部署机 compose 全栈重建，见上节                                                                                |

Agent 一律主动出连 Server（控制入口 `:19090`），Server 不反向拨号——见[运维手册](/zh/operations/)端口速查。

## 回滚手段

- **唯一回滚文档**：`docs/operations/upgrade-rollback.md`（中文内容，注意其站点不可见，见顶部路径口径说明）——单实例升级（`:26-37`）、HA 滚动升级（`:41-52`）、回滚语义（`:54-63`）。
- **核心纪律：数据库迁移只向前，没有 down 路径**（`upgrade-rollback.md:56`）。代码侧印证：`internal/db/migrate/migrate.go` 仅导出 `EnsureUpToDate`（`:184`）与 `CurrentVersion`（`:259`），无任何 Down/Rollback 入口；`MinimumRequiredVersion`（`:48-54`，当前 45）校验失败即拒绝启动（`:251-252`，旧二进制遇更新库直接拒绝启动，`:18`）。
- **镜像回滚**：改 `.env` 的 `CROUPIER_*_TAG` 指向旧 tag 后 `docker compose pull && up -d`（`upgrade-rollback.md:29-31`；tag 面来自 `docker.yml:68` 的 metadata-action main+sha 语义与 tag 触发 semver 重注 `:47`）。
- **二进制回滚**：替换 `/opt/croupier/bin/croupier-server` 固定路径文件（`deploy-binary.md:63-64`；`install.sh:473-483` 覆盖安装不自动备份旧版本）。
- **数据面兜底**：平台内置备份/恢复（`docs/operations/backup-restore.md:46` 恢复步骤、`:75` 备份可用性验证）。
- **回滚三场景**（`upgrade-rollback.md:57-61`）：① 新版未引入新迁移→直接回滚二进制/镜像；② 旧二进制版本检查通过但需人工确认不写新列；③ 旧二进制被拒启动→前滚修复（优先）或恢复 DB 备份（接受备份点后数据丢失）。

### 缺失项（回滚）

- `docs/zh/operations/` 下没有任何升级/回滚页面（`find docs/zh -iname "*rollback*" -o -iname "*upgrade*"` 零命中）——本文即为此补的 zh 侧入口。
- 无 helm/kustomize 清单，K8s 回滚仅靠文档内嵌 Deployment 的滚动语义，无仓库内自动回滚机制。
- `zh/operations/index.md:21-22` 内容地图声称覆盖「升级回滚」等条目，但对应文件不在站点构建内，导航与实际可见页面不符（已知口径问题，本文不代修该索引以外的指向）。

## 相关页面

- 发布节奏、版本策略与门禁：[开发周期与发布说明](/zh/development/lifecycle)
- 部署形态细节：[Docker 部署](/zh/operations/deploy-docker)
- 安全与风控：[安全基线](/zh/operations/security)、[风控](/zh/operations/risk-control)
