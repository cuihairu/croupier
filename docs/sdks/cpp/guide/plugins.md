# 插件

本文是 C++ SDK 动态插件能力的 canonical 入口。插件用于在运行时加载业务扩展，但不改变 SDK 与 Agent 之间的统一 session 基线。

## 使用场景

- 将业务函数打包成独立动态库。
- 在不重启宿主程序的情况下加载扩展能力。
- 将示例、测试或特定游戏域能力与主 SDK 解耦。

## 构建与运行

```bash
cmake -B build -DBUILD_EXAMPLES=ON \
  -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
./build/bin/croupier-plugin-demo
```

示例插件产物通常位于：

- Windows: `build/plugins/example_plugin.dll`
- macOS: `build/plugins/libexample_plugin.dylib`
- Linux: `build/plugins/libexample_plugin.so`

## 插件导出接口

插件应导出稳定的 C ABI，避免 C++ ABI 差异影响运行时加载：

```cpp
extern "C" {
    int croupier_plugin_init();
    PluginInfo* croupier_plugin_info();
    void croupier_plugin_cleanup();
}
```

业务函数推荐使用 JSON 输入输出：

```cpp
extern "C" {
    const char* your_function_name(const char* context, const char* payload);
}
```

| 参数 | 说明 |
| --- | --- |
| `context` | 执行上下文，通常是 JSON 字符串 |
| `payload` | 输入数据，通常是 JSON 字符串 |
| return | 输出数据，必须在调用方读取期间保持有效 |

## 实现要求

- `croupier_plugin_init()` 返回 `0` 表示初始化成功。
- `croupier_plugin_info()` 返回插件元数据和函数清单。
- `croupier_plugin_cleanup()` 负责释放插件级资源。
- 插件函数不得依赖 SDK 本地监听或 Agent 回拨模型。
- 插件内注册的函数 ID 应稳定，便于 Dashboard、权限和审计引用。

## 相关页面

- [函数注册 API](/sdks/cpp/api/functions)
- [插件示例](/sdks/cpp/examples/plugin)
