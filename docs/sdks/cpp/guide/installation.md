# 安装

## 依赖

- CMake
- Ninja 或平台原生构建工具
- vcpkg

## 基本流程

```bash
cmake -B build -G Ninja \
  -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
```

## 建议

- 优先使用仓库内 CI 已验证的 triplet
- Windows 上显式确认运行时库与 triplet 一致
