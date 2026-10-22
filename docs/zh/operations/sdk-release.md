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

六语言 SDK 发布到各语言官方包仓库。**每种语言一条独立的手动 workflow**（`.github/workflows/release-sdk-<lang>.yml`，仅 `workflow_dispatch`，不随 push 自动触发）——某条失败可单独重发、互不影响；单独重跑那一条即可。

```text
Actions → Release SDK — Python / JS / Go / Java / C# / C++ → Run workflow
```

## 1. Workflow 输入（六条 workflow 一致）

| 输入      | 类型 | 说明                                                                                                       |
| --------- | ---- | ---------------------------------------------------------------------------------------------------------- |
| `version` | 文本 | 发布版本。**留空 = 最新 release tag**（如 `v0.1.1`）；也可显式指定，`v0.2.0` 与 `0.2.0` 等价（自动归一化） |

## 2. 发布目标与所需 Secrets

版本号只注入构建产物，从不回写仓库（各语言 manifest 文件保持不变）。

| Workflow             | 目标                                  | 所需 Secrets                               |
| -------------------- | ------------------------------------- | ------------------------------------------ |
| Release SDK — Python | PyPI                                  | `PYPI_API_TOKEN`                           |
| Release SDK — JS     | npm                                   | `NPM_TOKEN`                                |
| Release SDK — Go     | Go module tag（`sdks/go/vX.Y.Z`）     | 无                                         |
| Release SDK — Java   | GitHub Packages（可选 Maven Central） | 无（Central 需 `MAVEN_CENTRAL_*`+`GPG_*`） |
| Release SDK — C#     | NuGet                                 | `NUGET_API_KEY`                            |
| Release SDK — C++    | GitHub Release 源码包                 | 无                                         |

| 语言         | 目标仓库                       | 包名/坐标                              | 所需 Secrets                                                                                                    | 未配置时的行为                     |
| ------------ | ------------------------------ | -------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| Python       | [PyPI](https://pypi.org)       | `croupier`（pyproject）                | `PYPI_API_TOKEN`                                                                                                | Job 显式失败，指名缺失的凭据       |
| JS           | [npm](https://www.npmjs.com)   | `croupier-js-sdk`                      | `NPM_TOKEN`                                                                                                     | 同上                               |
| Go           | Go module proxy                | `github.com/cuihairu/croupier/sdks/go` | 无（打 tag `sdks/go/vX.Y.Z`）                                                                                   | 无需凭据；可直接试跑               |
| Java         | GitHub Packages（主仓库）      | `io.github.cuihairu.croupier:*`        | 无（内置 `GITHUB_TOKEN`）                                                                                       | 无需额外凭据                       |
| Java（可选） | Maven Central（OSSRH）         | 同上                                   | `MAVEN_CENTRAL_USERNAME`、`MAVEN_CENTRAL_PASSWORD`、`GPG_KEY_ID`、`GPG_PRIVATE_KEY`、`GPG_PRIVATE_KEY_PASSWORD` | 缺失时只发 GitHub Packages，不报错 |
| C#           | [NuGet](https://www.nuget.org) | `Croupier.Sdk`                         | `NUGET_API_KEY`                                                                                                 | Job 显式失败，指名缺失的凭据       |
| C++          | GitHub Release 源码附件        | `croupier-cpp-sdk-<version>.tar.gz`    | 无（内置 `GITHUB_TOKEN`）                                                                                       | 无需凭据；可直接试跑               |

## 3. Secrets 配置步骤

仓库路径：**Settings → Secrets and variables → Actions → New repository secret**。

### PYPI_API_TOKEN（Python）

1. 注册/登录 PyPI 账号；首次发布目标包名前，需先在 PyPI 确认其可用性；
2. 账户设置 → API tokens → Add API token；
3. 范围选 Entire account（首次）或限定 `croupier` 包；token 形如 `«redacted:pypi-…»...`；
4. Name 设为 `PYPI_API_TOKEN`。

### NPM_TOKEN（JS）

1. 注册/登录 npmjs.com（发布 `croupier-js-sdk` 需拥有该包名，或选 scoped 名 `@<scope>/croupier-js-sdk`）；
2. Access Tokens → Generate New Token → **Granular Access Token**（勾选 Read and write）；
3. Name 设为 `NPM_TOKEN`。workflow 内以 `NODE_AUTH_TOKEN` 生效。

### NUGET_API_KEY（C#）

1. 注册/登录 nuget.org（Microsoft 账号）；
2. 账户 → API Keys → Create；Package scope 填 `Croupier.Sdk`，Expiration 按政策；
3. Name 设为 `NUGET_API_KEY`。

### Maven Central（Java，可选）

命名空间 `io.github.cuihairu` 对应 GitHub 命名空间，走 Sonatype Central Portal：

1. 用 GitHub 账号登录 Central Portal（central.sonatype.com）并验证 `io.github.cuihairu` 命名空间；
2. 生成 Portal 用户 token（Account → Generate User Token）→ 分别配置 `MAVEN_CENTRAL_USERNAME` / `MAVEN_CENTRAL_PASSWORD`；
3. GPG 签名密钥：`gpg --full-generate-key`（RSA 4096）后导出——
   - Key ID（8 位或完整 40 位十六进制）→ `GPG_KEY_ID`
   - `gpg --export-secret-keys --armor <KEYID>` 的完整输出 → `GPG_PRIVATE_KEY`
   - 密钥口令 → `GPG_PRIVATE_KEY_PASSWORD`
4. 五个 secrets 齐备后 workflow 自动追加 `-PforceRelease` 一并发布到 Central；缺任一则只发 GitHub Packages。

## 4. 各语言发布机制

### Python（PyPI）

`pyproject.toml` 的 `version` 经 `sed` 注入 → `python -m build` 产出 sdist/wheel → `twine upload`（token 认证）。

### JS（npm）

`npm version <v> --no-git-tag-version` 设版本 → `npm ci` → `npm run build`（如定义）→ `npm publish --access public`。

### Go（module tag）

Go 没有中心仓库，发布约定是**子模块 tag**：`sdks/go/vX.Y.Z`（module path `github.com/cuihairu/croupier/sdks/go`）。tag 已存在则跳过不报错；proxy.golang.org 按 tag 拉取。

### Java（GitHub Packages / Maven Central）

`sdks/java/build.gradle` 已适配三个构建属性（外加环境变量）：

| 属性 / 环境变量                                           | 用途                                                                                                  |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `-PoverrideVersion=<v>`                                   | 覆盖硬编码的 `baseVersion`（workflow 注入发布版本）                                                   |
| `-PforceRelease`                                          | 手动发布不打 tag 也走 release 通道（签名 + OSSRH 仓库）                                               |
| `GITHUB_PACKAGES_URL`                                     | 覆盖 GitHub Packages 上传 URL（workflow 指回主仓库 `croupier`；默认值是独立仓库 `croupier-sdk-java`） |
| `MAVEN_CENTRAL_USERNAME/PASSWORD`                         | OSSRH 凭据                                                                                            |
| `GPG_KEY_ID / GPG_PRIVATE_KEY / GPG_PRIVATE_KEY_PASSWORD` | 签名（缺密钥自动跳过签名；仅本地构建场景）                                                            |

### C#（NuGet）

`dotnet pack -c Release -p:Version=<v>`（csproj 里的 `<Version>` 不回写）→ `dotnet nuget push`（`--skip-duplicate` 允许重试）。

### C++（Release 附件）

C++ 没有中心包仓库：把 `sdks/cpp` 打成 tarball（排除 build 目录）上传到该版本的 GitHub Release；Release 不存在时自动建 draft。

## 5. 首发清单

1. （按需）配置第 3 节的 secrets；
2. 建议先试跑 **Go + C++**（均零外部凭据）验证 workflow 与版本解析；
3. 确认目标包名未被占用（PyPI `croupier` / npm `croupier-js-sdk` / NuGet `Croupier.Sdk`）；
4. Java 同步 Central 前确认命名空间验证与 GPG secrets 全集；
5. 发布后在各仓库检索版本：`pip index versions croupier`、`npm view croupier-js-sdk versions`、NuGet 页面、`go list -m github.com/cuihairu/croupier/sdks/go@vX.Y.Z`。

## 6. 常见问题

| 症状                                | 处置                                                                   |
| ----------------------------------- | ---------------------------------------------------------------------- |
| Job 立即失败报 `missing secret ...` | 按提示配置对应凭据后重跑                                               |
| PyPI 403 target package unavailable | 包名被占用或 token scope 不含该包；首次需 Entire account scope         |
| NuGet 409                           | 版本已发布；`--skip-duplicate` 已容忍——确认是否忘了升版本号            |
| Java 发到了旧的独立仓库             | 未注入 `GITHUB_PACKAGES_URL`；确认跑的是本 workflow 而非旧 release.yml |
| Go tag 已存在被跳过                 | 同版本重发属预期幂等行为；新版本请指定新的 version 输入                |
