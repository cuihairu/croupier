# Plugins

本页是 C++ SDK 动态插件能力的权威入口。Plugin 在运行时加载业务扩展，不改变 SDK 与 Agent 之间的统一 session 基线。

## 使用场景

- 把业务函数打包成独立动态库。
- 不重启宿主应用即可加载扩展能力。
- 把示例、测试或游戏域专用能力与主 SDK 解耦。

## 构建与运行

```bash
cmake -B build -DBUILD_EXAMPLES=ON \
  -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
./build/bin/croupier-plugin-demo
```

示例插件产物通常位于：

- Windows：`build/plugins/example_plugin.dll`
- macOS：`build/plugins/libexample_plugin.dylib`
- Linux：`build/plugins/libexample_plugin.so`

## Plugin 导出接口

Plugin 应导出稳定的 C ABI，避免 C++ ABI 差异影响运行时加载：

```cpp
extern "C" {
    int croupier_plugin_init();
    PluginInfo* croupier_plugin_info();
    void croupier_plugin_cleanup();
}
```

业务函数建议使用 JSON 输入/输出：

```cpp
extern "C" {
    const char* your_function_name(const char* context, const char* payload);
}
```

| 参数      | 说明                                 |
| --------- | ------------------------------------ |
| `context` | 执行上下文，通常是 JSON 字符串       |
| `payload` | 输入数据，通常是 JSON 字符串         |
| 返回值    | 输出数据；调用方读取期间必须保持有效 |

## 实现要求

- `croupier_plugin_init()` 返回 `0` 表示初始化成功。
- `croupier_plugin_info()` 返回插件元数据与函数清单。
- `croupier_plugin_cleanup()` 释放插件级资源。
- Plugin 函数不得依赖 SDK local-listener 或 Agent callback 模型。
- Plugin 内注册的函数 ID 应保持稳定，便于 Dashboard、权限与审计引用。

## 相关页面
