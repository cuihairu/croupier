---
title: 发布约定
---

# 发布约定

当前仓库已经是 monorepo，发布标签必须区分 Server / Agent 与各语言 SDK。

## 标签规则

### Server / Agent

- 使用 `v*`
- 示例：`v0.2.0`

对应 workflow：

- `.github/workflows/release.yml`
- `.github/workflows/docker.yml`

### SDK

- JavaScript: `sdk-js-v*`
- Python: `sdk-python-v*`
- Go: `sdk-go-v*`
- Java: `sdk-java-v*`
- C++: `sdk-cpp-v*`

示例：

- `sdk-js-v0.1.0`
- `sdk-go-v0.1.0`

对应 workflow：

- `.github/workflows/release-sdk-js.yml`
- `.github/workflows/release-sdk-python.yml`
- `.github/workflows/release-sdk-go.yml`
- `.github/workflows/release-sdk-java.yml`
- `.github/workflows/release-sdk-cpp.yml`

## 原则

- 不要再用单一 `v*` 标签发布某个 SDK
- SDK release workflow 必须在各自 `sdks/<lang>` 目录内运行
- SDK 文档统一发布到根 `docs/` 站点，不再单独维护每个 SDK 的独立文档站

## 操作建议

### 发布 Server / Agent

```bash
git tag v0.2.0
git push origin v0.2.0
```

### 发布 JavaScript SDK

```bash
git tag sdk-js-v0.1.0
git push origin sdk-js-v0.1.0
```

### 发布 Go SDK

```bash
git tag sdk-go-v0.1.0
git push origin sdk-go-v0.1.0
```

## 注意事项

- `docker.yml` 和 `release.yml` 只会响应 `v*`，不会响应 `sdk-*-v*`
- SDK release tag 不会触发 Server / Agent 发布链路
- 如需手动发布，优先使用对应 workflow 的 `workflow_dispatch`
