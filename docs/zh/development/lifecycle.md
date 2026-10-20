---
title: 开发周期与发布说明
---

# 开发周期与发布说明

本文盘点本仓开发周期的四个面向（分支模型、发布节奏、版本策略、测试与发布门禁），逐项给出 file:line 证据；仓库内没有对应机制的条目如实标注为缺失。口径核对日期 2026-10-20，行号以后续演进可能漂移，以核对日为准。

## 分支模型

- **单 main 主干开发**：全部工作流触发面只指向 main —— `ci.yml:5`/`:14`（push 与 pull_request 均 `branches: [main]`），`ci-analytics.yml:4`/`:13`、`ci-mobile.yml:12`/`:17`、六个 `ci-sdk-*.yml` 同口径；仓库内无 develop/release 分支。
- **PR 为辅、直推为主**：`git log --merges` 实测 main 共 88 个 merge，其中仅 4 个 PR merge（`#72`–`#76`，含 dependabot），其余为 worktree 临时分支的 merge；当前日常形态是直接 push main（CI 在 push 时全量把关）。
- **合并纪律**：PR 需说明改了什么、为什么、如何验证，合并前 `make test` 通过 —— `docs/zh/development/repository-guidelines.md:93-96`。
- **缺失项（分支保护）**：`.github/` 下无 `CODEOWNERS`、无分支保护相关配置文件；分支保护属 GitHub 仓库设置，仓库文件内无可引用证据，如实标注为「仓库内无机制」。

## 发布节奏

- **nightly 每日构建**：`schedule` 触发 `cron: "0 2 * * *"`（每日 02:00 UTC，`.github/workflows/nightly.yml:5`），支持 `workflow_dispatch` 手动触发（`:6`）；`concurrency.group: nightly-release` 且不取消在途（`:14-16`）。各 build job 固定 checkout main（`:53` 等），release job 汇总产物后滚动更新 `nightly` tag（`:814` force push）并发布 prerelease（`:817-829`，`tag_name: nightly`、`prerelease: true`）。
- **缺失项（变更检测）**：nightly 是无条件每日构建，仓库内没有「48 小时有更新才发」之类的变更检测逻辑（`nightly.yml` 全文与全历史 grep 均无命中）；该口径属流程约定。
- **稳定版发布**：唯一有效入口是人工 push `v*` tag —— `release.yml:4-6`（`push: tags: ["v*"]`）；`workflow_dispatch` 存在（`:7`）但三个 job 均有 `if: startsWith(github.ref, 'refs/tags/v')`（`:22`/`:151`/`:201`），手动运行等于空跑。`docker.yml:4-6` 同样响应 `v*` tag（另加 push main）。
- **SDK 发布**：每语言一个独立手动 workflow，仅 `workflow_dispatch` 绝不自动触发（`release-sdk-*.yml` 各文件 `:3` 注释；`release-sdk-go.yml:5-11` 的 `version` 输入留空则取最新 release tag，逻辑在 `:24-34`）。
- **安全扫描节奏**：CodeQL 每周一 03:00 UTC（`codeql.yml:24`）。

## 版本策略

- **根 `VERSION` 文件是唯一版本真源**（当前内容 `0.1.6`）：`Makefile:2` `VERSION := $(shell cat VERSION ...)`；`:10` 组装 `LDFLAGS := -X main.Version=$(FULL_VERSION) ...`。
- **注入点**：server 走 `cmd/server/root.go:34` 的 `Version` 变量与 `internal/svc/version.go:6` 的 `ServerVersion`（`Makefile:89` 完整导入路径注入）；devops/capture/synthetics 注入 `agents/*/runner.Version`（`Makefile:117`/`:122`/`:128`）。
- **发布时点重注**：VERSION 文件值在发布切 tag 时可能滞后（`release.yml:58-60` 注释记录 v0.1.5 实证恒 0.1.4），发布面统一以 `git describe` 重注（`release.yml:61`）；nightly 面统一以 `nightly-YYYYMMDD` 重注（`nightly.yml:81`，二进制 `--version` 必含 `nightly-` 的校验在 `:190`）。
- **SDK 版本同步**：根 VERSION bump 后运行 `make version-sync`（`scripts/sync-sdk-versions.sh:2-16`）。
- **版本漂移现场（如实记录）**：根 `VERSION`（0.1.6）> 最新 tag（`v0.1.5`，`git ls-remote --tags` 实测）> SDK 清单（`sdks/go/version.go:5`、`sdks/js/package.json:3`、`sdks/python/pyproject.toml:7` 均 0.1.4）三层并存，是「VERSION 先行、发布滞后」的直接证据。
- **tag 命名**：Server/Agent 用 `v*`，SDK 用子模块 tag（`docs/operations/sdk-release.md:96`：`sdks/go/vX.Y.Z`；`release-sdk-go.yml:40-46` 实际执行）。`repository-guidelines.md:104-110` 的 `sdk-go-v*` 旧口径与现行实现已漂移，以子模块 tag 为准。
- **约定边界（非机制）**：「版本号只由用户给定并手动触发」目前是流程约定——机制上成立（发版唯一入口是人工 push `v*` tag），但仓库内没有任何工作流或代码强制校验该约定，任何有 push 权限者理论上可直接打 tag 触发发布。如实标注为约定而非仓库内强制机制。

## 测试与发布门禁

### CI - Core（`ci.yml`）

- 六个 job：`lint`（`:27`）、`security`（`:119`）、`test`（`:161`）三轨并行；`e2e`（`:225`）、`migrate-matrix`（`:321`）、`capture-matrix`（`:418`）`needs: [test]`（`:232`/`:331`/`:432`）。
- **lint 守卫链**：架构守卫 `ci.yml:45`、contracts `:48`、API 响应契约 `:51`、dashboard vnext 守卫及自测 `:54`/`:57`、SDK 矩阵 `:60`、demo 契约基线 `:63`、`buf lint proto` `:89`、gofmt `:106`、`go vet` `:114`、传输依赖边界（`transport-no-grpc` 守卫）`:117`。
- **security**：govulncheck（豁免清单包装，`ci.yml:159`）。
- **test**：全量 `go test ./internal/... ./core/... ./agents/...` 无 `-race`（`ci.yml:208`）+ 定向 `-race`（`:212`，register/devops/capture/sidecar）+ Codecov 上传（`:214-223`）。
- **e2e**：SQLite 起服跑 happy-path 与 task 生命周期脚本（`:311-312`，SQLite 配置见 `:282`）。
- **migrate-matrix**：MySQL 8.4 / Postgres 16 / MSSQL 2022 容器跑迁移矩阵（`:332-367`）+ `atlas migrate validate`（`:416`）。
- **capture-matrix**：MySQL/MariaDB/Postgres/MongoDB 四源容器集成测试（`:460-528`）。

### CI - Dashboard（`ci-dashboard.yml`）

- 四 job 并行：质量门禁（`:41`，含 dashboard 守卫 `:94`、本地化文本守卫 `:99`、`tsc --noEmit` `:122`、jest 分片 `:136`）、mock E2E（`:223`）、真实 E2E（`:245`）、Docker 镜像与 nginx smoke（`:355`，`:402-417`）。
- **注意**：`:16-18` 使 v* tag 推送时 dashboard 全套质量门禁自动运行，但它与 `release.yml` 并行、不构成发版阻塞。

### 其余

- Analytics E2E（`ci-analytics.yml:39`）、Mobile（`ci-mobile.yml:24`，release APK 仅手动 `:48-49`）、文档检查（`docs-check.yml:44`/`:48` 链接与术语守卫）。
- **本地门禁**：pre-commit 钩子做 BOM 与 gofmt 检查（`scripts/git-hooks/pre-commit:29-43`，BOM 循环与 gofmt 各一段）；`make test-changed` 是本地选择性测试工具，CI 门禁保持全量（`repository-guidelines.md:71`）。

### 缺失项（发布前置门禁）

- `release.yml` 与 `nightly.yml` 均无任何测试/门禁 job（`release.yml` 的 `build-go` `:20`、`build-sdk-sources` `:150`、`release` `:189` 只构建打包）；六个 SDK 发布 workflow 也无测试直接发布。「发布前必须过门禁」目前依赖人工纪律（打 tag 前确认 CI 全绿），仓库内无强制机制——如实标注。

## 已知缺失汇总

| 面向             | 状态       | 说明                                                                                   |
| ---------------- | ---------- | -------------------------------------------------------------------------------------- |
| 分支保护         | 仓库内无   | 属 GitHub 仓库设置，文件内无可引用证据                                                 |
| nightly 变更检测 | 无         | 无条件每日构建，「有更新才发」是流程约定                                               |
| 发布前置门禁     | 无强制     | release/nightly/SDK workflow 均无测试 job；靠人工确认 CI 绿                            |
| 版本号人工给定   | 约定非机制 | 机制上唯一入口是人工打 `v*` tag；无仓库内强制校验                                      |
| SDK tag 口径     | 漂移       | 现行子模块 tag（`sdks/go/vX.Y.Z`），`repository-guidelines.md` 的 `sdk-go-v*` 为旧口径 |
