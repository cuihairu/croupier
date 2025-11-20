# SDK 版本管理

本文档说明如何统一管理所有 SDK 的版本号。

## 当前版本

项目使用 **统一版本号** 管理所有 SDK，当前版本：**0.1.0**

## 版本文件位置

- **主版本文件**: `/VERSION` - 单一来源的真相
- **SDK 特定配置**:
  - JS: `sdks/js/package.json` → `"version": "0.1.0"`
  - Python: `sdks/python/setup.py` → `version="0.1.0"`
  - Java: `sdks/java/build.gradle` → `version = '0.1.0'`
  - C++: `sdks/cpp/CMakeLists.txt` → `VERSION 0.1.0`
  - Go: `sdks/go/version.go` → `const Version = "0.1.0"`

## 查看当前版本

```bash
make version
```

输出示例：
```
Current SDK Version: 0.1.0

SDK Versions:
  JS:     0.1.0
  Python: 0.1.0
  Java:   0.1.0
  C++:    0.1.0
  Go:     0.1.0
```

## 更新版本

### 方法 1: 使用 make 命令（推荐）

```bash
# 编辑 VERSION 文件，修改为新版本号，例如 0.2.0
echo "0.2.0" > VERSION

# 同步到所有 SDK
make version-sync
```

### 方法 2: 直接使用脚本

```bash
# 同步当前 VERSION 文件到所有 SDK
./scripts/sync-sdk-versions.sh

# 或者直接指定新版本
./scripts/sync-sdk-versions.sh 0.2.0
```

## 版本同步流程

`make version-sync` 或 `sync-sdk-versions.sh` 会执行以下操作：

1. ✅ 验证版本号格式（必须符合 semver 规范）
2. ✅ 更新 `VERSION` 文件
3. ✅ 同步到所有 SDK 配置文件：
   - `sdks/js/package.json`
   - `sdks/python/setup.py`
   - `sdks/java/build.gradle`
   - `sdks/cpp/CMakeLists.txt`
   - `sdks/go/version.go`（自动创建）
4. ✅ 更新 `sdks/js/pnpm-lock.yaml`
5. ✅ 提示提交变更

## 发布流程

### 1. 准备发布

```bash
# 1. 更新版本号
echo "0.2.0" > VERSION
make version-sync

# 2. 验证版本
make version

# 3. 测试构建
make build-sdks
```

### 2. 提交变更

```bash
git add VERSION \
  sdks/js/package.json sdks/js/pnpm-lock.yaml \
  sdks/python/setup.py \
  sdks/java/build.gradle \
  sdks/cpp/CMakeLists.txt \
  sdks/go/version.go

git commit -m "chore: bump SDK versions to 0.2.0"
```

### 3. 创建 Git 标签

```bash
git tag -a v0.2.0 -m "Release version 0.2.0"
git push origin main --tags
```

### 4. 触发 Nightly Release

GitHub Actions 会自动：
- 构建所有 SDK
- 生成正确的包名：
  - `croupier-js-sdk-0.2.0.tgz`
  - `croupier-python-sdk-0.2.0.whl`
  - `croupier-java-sdk-0.2.0.jar`
  - `croupier-cpp-sdk-*-0.2.0.tar.gz`
  - `croupier-go-sdk.tar.gz`

## 包命名规范

所有 SDK 包遵循统一命名规范：

- **JS**: `croupier-js-sdk-{version}.tgz`
- **Python**: `croupier-python-sdk-{version}.whl`
- **Java**: `croupier-java-sdk-{version}.jar`
- **C++**: `croupier-cpp-sdk-{os}-{arch}-static-{version}.tar.gz`
- **Go**: `croupier-go-sdk.tar.gz` (源码包，无版本后缀)

## 版本号规范

遵循 [Semantic Versioning 2.0.0](https://semver.org/)：

- `0.y.z` - 初始开发阶段（当前）
- `1.0.0` - 首个稳定版本
- `x.y.z-alpha.N` - Alpha 预发布版本
- `x.y.z-beta.N` - Beta 预发布版本
- `x.y.z-rc.N` - Release Candidate

示例：
- `0.1.0` - 初始版本
- `0.2.0` - 新增功能
- `0.2.1` - Bug 修复
- `1.0.0-beta.1` - 首个 beta 版本
- `1.0.0` - 正式版本

## 常见问题

### Q: 为什么需要统一版本号？

A: 统一版本号确保：
- 所有 SDK 功能对齐
- 文档和发布说明一致
- 用户更容易理解兼容性

### Q: 如果只更新一个 SDK 怎么办？

A: 仍然建议统一升级版本号（至少更新补丁版本），并在 CHANGELOG 中注明具体变更的 SDK。

### Q: 如何回滚版本？

A:
```bash
# 恢复到指定版本
./scripts/sync-sdk-versions.sh 0.1.0
git add -A
git commit -m "chore: revert SDK versions to 0.1.0"
```

## 相关文件

- `/VERSION` - 主版本文件
- `/scripts/sync-sdk-versions.sh` - 版本同步脚本
- `/Makefile` - 版本管理命令
- `/.github/workflows/sdk-nightly.yml` - CI/CD 配置
