# 构建

## 目标

- 本地开发构建可重复
- CI/发布环境尽量贴近本地配置

## 建议

- 使用 CMake presets 或显式 triplet
- Windows 上先确认 runtime library 匹配
- 依赖管理交给 vcpkg，不要混用多个依赖来源
