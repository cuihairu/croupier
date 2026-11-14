---
title: 版本与依赖策略
---

# 版本与依赖策略

平台版本与依赖升级在 Go module、pnpm workspace、六个 SDK manifest 上的管理
方式。事实核验于 2026-11-10；本文引用的 `scripts/check-version-sync.sh` 与
`Makefile` 目标是本页策略的可执行形态。

## 版本来源

仓库根 `VERSION` 文件是平台版本的唯一真相源（当前 `0.1.6`）。六个 SDK
manifest 必须与它严格相等：

| 面         | 文件                                               | 形式                |
| ---------- | -------------------------------------------------- | ------------------- |
| 平台       | `VERSION`                                          | `MAJOR.MINOR.PATCH` |
| Go SDK     | `sdks/go/version.go` + `sdks/go` module            | `const Version`     |
| JS/TS SDK  | `sdks/js/package.json` + `sdks/js/src/version.ts`  | `version` 字段      |
| Python SDK | `sdks/python/pyproject.toml`                       | `project.version`   |
| Java SDK   | `sdks/java/build.gradle` + `SdkInfo.java`          | `baseVersion`       |
| C# SDK     | `sdks/csharp/src/Croupier.Sdk/Croupier.Sdk.csproj` | `<Version>`         |
| C++ SDK    | `sdks/cpp/VERSION.cmake` + `croupier_client.cpp`   | CMake 版本三元组    |

- **守卫**：`scripts/check-version-sync.sh` 是只读一致性检查，接入 CI
  （"Version guard" job）。默认 CI 模式要求 `VERSION` **大于等于**最新
  `vX.Y.Z` tag（领先 tag 是两次发版之间的正常状态）；`--require-tag-match`
  （发布管线）要求严格相等。该脚本从不修改任何文件。
- **同步**：改完 `VERSION` 后跑 `make version-sync`
  （`scripts/sync-sdk-versions.sh`）——禁止手工改六处 manifest；守卫失败
  信息同样如此提示。
- **谁改版本**：版本号与发版 tag 由维护者手动给出。每日构建只产出唯一一条滚动 `nightly` prerelease
  （仅保留最新一次构建），永远不产生带版本号的正式发布
  （发布节奏见[开发周期与发布说明](./lifecycle)）。
- **Java snapshot 行为**：main 上 Java SDK 以 `X.Y.Z-SNAPSHOT` 构建；精确
  版本仅在 `v*` tag 构建时使用。

## Go Modules

两个 Go module，刻意分离：

- 根 module `github.com/cuihairu/croupier` —— server、agent、CLI、共享包
  （`internal/`、`core/`、`agents/`、`pkg/`、`cmd/`）。
- SDK module `github.com/cuihairu/croupier/sdks/go` —— 发布的 SDK，独立
  `go.mod`/`go.sum`，其依赖面与 server 侧解耦。

依赖规则：

- **Dependabot**（`gomod`，每周）提 minor/patch 升级；major 升级被配置忽略，
  需要时按依赖逐个刻意进行。
- **漏洞门禁**：Security Scan job 跑 `govulncheck`。扫描红通过升级受影响
  依赖（stdlib 命中则抬 `go` 指令）修复——不豁免检查。注意 `go get` 可能
  按本机 `GOTOOLCHAIN` 静默抬升 `go` 指令，依赖变更后必须核对 diff。
- 两个 module 当前使用相同 `go` 指令（`1.27.3`）；升级时保持两侧对齐。

## pnpm Workspaces

`web/`（控制台）与 `docs/`（本站）是两个独立 pnpm workspace，各有自己的
`pnpm-workspace.yaml` 与 lockfile。pnpm 分别解析，版本不跨 workspace 提升。

以下规则来自真实事故（`web/pnpm-workspace.yaml` 每条 override 都带日期
注释）：

- **overrides 只认 `pnpm-workspace.yaml`**：pnpm 11 只从 `pnpm-workspace.yaml`
  读 `overrides` 块；`package.json` 顶层的 `overrides` 键会被静默忽略。
- **安全驱动的最低版本钉定**：override 的存在是为了把传递依赖强制到已修复
  版本（每条注释引用 Dependabot/GHSA）。值形态有两条守则：
  - 范围钉定要圈在一条 major 线内（`>=x.y.z <下一 major`），除非刻意跨
    major。曾发生过开放下限让钉定的 `axios` 线重新解析到 1.x，现已钉为
    `>=0.30.0 <1.0.0`。
  - 整线钉死曾两次造成破坏：`path-to-regexp` 全站崩溃、`semver` 7.x→6.x
    打断下游 SDK CI。确需钉一条漂移线时，必须重验每个消费方包。
- **直接依赖的 specifier 必须等于 override 解析值**：override 钉定某个
  直接依赖时，`package.json` 的 specifier 必须与之一致。不一致时本地安装
  正常（override 获胜），CI `--frozen-lockfile` 直接失败。
- **`minimumReleaseAgeExclude`**：新发布版本受发布冷静期策略约束；豁免
  清单留给等不起冷静期的紧急安全修复（如 `react-router` 6.30.6 修复线）。
- **`allowBuilds`**：postinstall 脚本默认拒绝，仅白名单构建工具放行
  （`esbuild`、`unrs-resolver`）；其余（`core-js`、`@scarf/scarf` 等）
  一律拒绝。

## 相关页面

- [开发周期与发布说明](./lifecycle) —— 分支模型、发布节奏、nightly 机制。
- [SDK 能力矩阵](https://github.com/cuihairu/croupier/blob/main/sdks/SDK_FEATURE_MATRIX.md)
  —— 各语言能力等级（SDK 功能唯一真相源；版本号本身遵循上文表格）。
