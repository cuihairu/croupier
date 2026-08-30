---
title: SDK 包发布
icon: send
order: 8
category:
  - 运维手册
tag:
  - 发布
  - SDK
---

# SDK 包发布

六语言 SDK 发布到各语言官方包仓库。**每语言一个独立手动工作流**（`.github/workflows/release-sdk-<lang>.yml`，仅 `workflow_dispatch`，不在 push 时自动触发）——失败重发互不影响，单独重跑即可。

```text
Actions → Release SDK — Python / JS / Go / Java / C# / C++ → Run workflow
```

## 1. 工作流输入（六个工作流相同）

| 输入      | 类型 | 说明                                                                                                             |
| --------- | ---- | ---------------------------------------------------------------------------------------------------------------- |
| `version` | 文本 | 发布版本。**留空 = 最新 release tag**（如 `v0.1.1`）；可显式指定，`v0.2.0` 与 `0.2.0` 两种写法等价（自动归一化） |

| 工作流               | 目标                                  | 所需 Secrets                               |
| -------------------- | ------------------------------------- | ------------------------------------------ |
| Release SDK — Python | PyPI                                  | `PYPI_API_TOKEN`                           |
| Release SDK — JS     | npm                                   | `NPM_TOKEN`                                |
| Release SDK — Go     | Go module tag（`sdks/go/vX.Y.Z`）     | 无                                         |
| Release SDK — Java   | GitHub Packages（可选 Maven Central） | 无（Central 需 `MAVEN_CENTRAL_*`+`GPG_*`） |
| Release SDK — C#     | NuGet                                 | `NUGET_API_KEY`                            |
| Release SDK — C++    | GitHub Release 源码包                 | 无                                         |

## 2. 发布目标与所需 Secrets

版本号只注入构建产物，不回写仓库（各语言清单文件保持不变）。

| 语言         | 目标仓库                       | 包名 / 坐标                            | 所需 Secrets                                                                                                    | 未配置时行为                         |
| ------------ | ------------------------------ | -------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ------------------------------------ |
| Python       | [PyPI](https://pypi.org)       | `croupier`（pyproject）                | `PYPI_API_TOKEN`                                                                                                | job 显式报错并提示凭据名             |
| JS           | [npm](https://www.npmjs.com)   | `croupier-js-sdk`                      | `NPM_TOKEN`                                                                                                     | 同上                                 |
| Go           | Go module proxy                | `github.com/cuihairu/croupier/sdks/go` | 无（打 tag `sdks/go/vX.Y.Z`）                                                                                   | 无需凭据，可直接试跑                 |
| Java         | GitHub Packages（主仓库）      | `io.github.cuihairu.croupier:*`        | 无（内置 `GITHUB_TOKEN`）                                                                                       | 无需额外凭据                         |
| Java（可选） | Maven Central（OSSRH）         | 同上                                   | `MAVEN_CENTRAL_USERNAME`、`MAVEN_CENTRAL_PASSWORD`、`GPG_KEY_ID`、`GPG_PRIVATE_KEY`、`GPG_PRIVATE_KEY_PASSWORD` | 未配置时仅发 GitHub Packages，不报错 |
| C#           | [NuGet](https://www.nuget.org) | `Croupier.Sdk`                         | `NUGET_API_KEY`                                                                                                 | job 显式报错并提示凭据名             |
| C++          | GitHub Release 附源码包        | `croupier-cpp-sdk-<version>.tar.gz`    | 无（内置 `GITHUB_TOKEN`）                                                                                       | 无需凭据，可直接试跑                 |

## 3. Secrets 配置步骤

仓库路径：**Settings → Secrets and variables → Actions → New repository secret**。

### PYPI_API_TOKEN（Python）

1. 注册/登录 PyPI 账号，目标包名首次发布前需在 PyPI 确认可用；
2. Account settings → API tokens → Add API token；
3. Scope 选 Entire account（首次）或限定 `croupier` 包；Token 形如 `pypi-AgEIcHlwaS5vcmc...`；
4. Name 填 `PYPI_API_TOKEN`。

### NPM_TOKEN（JS）

1. 注册/登录 npmjs.com（发布 `croupier-js-sdk` 需拥有该包名，或选择 scoped 包名 `@<scope>/croupier-js-sdk`）；
2. Access Tokens → Generate New Token → **Granular Access Token**（勾选 Read and write）；
3. Name 填 `NPM_TOKEN`。工作流内以 `NODE_AUTH_TOKEN` 生效。

### NUGET_API_KEY（C#）

1. 注册/登录 nuget.org（微软账号）；
2. 账号 → API Keys → Create；Package scope 选 `Croupier.Sdk`，Expiration 按策略；
3. Name 填 `NUGET_API_KEY`。

### Maven Central（Java，可选）

命名空间 `io.github.cuihairu` 对应 GitHub 命名空间，走 Sonatype Central Portal：

1. Central Portal（central.sonatype.com）用 GitHub 账号登录并验证 `io.github.cuihairu` 命名空间；
2. 生成 Portal 用户 token（Account → Generate User Token）→ 分别配 `MAVEN_CENTRAL_USERNAME` / `MAVEN_CENTRAL_PASSWORD`；
3. GPG 签名密钥：`gpg --full-generate-key`（RSA 4096）后导出——
   - Key ID（8 位或完整 40 位十六进制）→ `GPG_KEY_ID`
   - `gpg --export-secret-keys --armor <KEYID>` 输出全文 → `GPG_PRIVATE_KEY`
   - 密钥口令 → `GPG_PRIVATE_KEY_PASSWORD`
4. 五个 secret 齐备后，工作流自动附加 `-PforceRelease` 同步发布 Central；任一缺失则只发 GitHub Packages。

## 4. 各语言发布机制

### Python（PyPI）

`pyproject.toml` 的 `version` 由 `sed` 注入 → `python -m build` 产出 sdist/wheel → `twine upload`（token 认证）。

### JS（npm）

`npm version <v> --no-git-tag-version` 置版 → `npm ci` → `npm run build`（如定义）→ `npm publish --access public`。

### Go（module tag）

Go 无中心仓库，发布约定为**子模块 tag**：`sdks/go/vX.Y.Z`（module path 为 `github.com/cuihairu/croupier/sdks/go`）。tag 已存在时跳过不报错；proxy.golang.org 按 tag 拉取。

### Java（GitHub Packages / Maven Central）

`sdks/java/build.gradle` 已适配三个构建属性（与环境变量）：

| 属性 / 环境变量                                           | 作用                                                                                             |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `-PoverrideVersion=<v>`                                   | 覆盖写死的 `baseVersion`（工作流注入发布版本）                                                   |
| `-PforceRelease`                                          | 无 tag 的手动发布也走 release 通道（signing + OSSHR 仓库）                                       |
| `GITHUB_PACKAGES_URL`                                     | 覆盖 GitHub Packages 上传地址（工作流指回主仓库 `croupier`；缺省为独立仓库 `croupier-sdk-java`） |
| `MAVEN_CENTRAL_USERNAME/PASSWORD`                         | OSSRH 凭据                                                                                       |
| `GPG_KEY_ID / GPG_PRIVATE_KEY / GPG_PRIVATE_KEY_PASSWORD` | 签名（key 缺失时 signing 自动跳过，仅本地构建场景）                                              |

### C#（NuGet）

`dotnet pack -c Release -p:Version=<v>`（csproj 中 `<Version>` 不回写）→ `dotnet nuget push`（`--skip-duplicate` 允许重试）。

### C++（Release 附件）

C++ 无中心包仓库：`sdks/cpp` 打 tarball（排除 build 目录）上传到版本对应的 GitHub Release；Release 不存在时自动创建 draft。

## 5. 首次发布检查清单

1. （按需）配置第 3 节 Secrets；
2. 建议先用 **Go + C++** 试跑（两者零外部凭据）验证工作流与版本解析；
3. 确认目标包名未被占用（PyPI `croupier` / npm `croupier-js-sdk` / NuGet `Croupier.Sdk`）；
4. Java 同步 Central 前确认命名空间验证与 GPG 全套 secret；
5. 发布后在对应仓库检索版本：`pip index versions croupier`、`npm view croupier-js-sdk versions`、NuGet 页面、`go list -m github.com/cuihairu/croupier/sdks/go@vX.Y.Z`。

## 6. 常见问题

| 现象                                 | 处理                                                                 |
| ------------------------------------ | -------------------------------------------------------------------- |
| job 立即失败并提示 `缺少 secret ...` | 按提示补配对应凭据后重跑                                             |
| PyPI 403 目标包不可用                | 包名被占用或 token scope 不含该包；首次需 Entire account scope       |
| NuGet 409                            | 版本已发布；`--skip-duplicate` 已容错，确认版本号是否递增            |
| Java 发布到旧独立仓库                | `GITHUB_PACKAGES_URL` 未注入；确认通过本工作流运行而非旧 release.yml |
| Go tag 已存在跳过                    | 同版本重复发布属预期幂等行为；新版本请指定新 version 输入            |
