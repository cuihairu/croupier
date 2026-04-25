# 构建

## 目标

- 本地开发可重复构建
- CI/发布环境与本地配置尽量一致

## 建议

- 使用 CMake presets 或显式 triplet
- 在 Windows 上优先先确认运行时库匹配
- 把依赖管理交给 vcpkg，不要混用多套依赖来源
