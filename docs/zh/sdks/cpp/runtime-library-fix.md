# 修复 Windows RuntimeLibrary 不匹配

Windows 常见问题是 runtime library 不匹配，例如 `/MD` 与 `/MT` 混用。

## 根因

- vcpkg triplet 与 SDK 编译选项不一致
- 依赖库与目标库使用了不同的 CRT 策略

## 建议

- `x64-windows` 对应动态运行时
- `x64-windows-static` 对应静态运行时
- 保持 CMake、CI 与本地 presets 一致
