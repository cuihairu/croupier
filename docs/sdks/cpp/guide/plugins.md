# Plugins

This page is the canonical entry point for the C++ SDK's dynamic plugin capability. Plugins load business extensions at runtime without changing the unified session baseline between the SDK and the Agent.

## Use Cases

- Package business functions as standalone dynamic libraries.
- Load extension capabilities without restarting the host application.
- Decouple examples, tests, or game-domain-specific capabilities from the main SDK.

## Build and Run

```bash
cmake -B build -DBUILD_EXAMPLES=ON \
  -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
./build/bin/croupier-plugin-demo
```

The example plugin artifact is usually located at:

- Windows: `build/plugins/example_plugin.dll`
- macOS: `build/plugins/libexample_plugin.dylib`
- Linux: `build/plugins/libexample_plugin.so`

## Plugin Export Interface

Plugins should export a stable C ABI so that C++ ABI differences do not affect runtime loading:

```cpp
extern "C" {
    int croupier_plugin_init();
    PluginInfo* croupier_plugin_info();
    void croupier_plugin_cleanup();
}
```

Business functions are recommended to use JSON input/output:

```cpp
extern "C" {
    const char* your_function_name(const char* context, const char* payload);
}
```

| Parameter | Description                                              |
| --------- | -------------------------------------------------------- |
| `context` | Execution context, usually a JSON string                 |
| `payload` | Input data, usually a JSON string                        |
| return    | Output data; must remain valid while the caller reads it |

## Implementation Requirements

- `croupier_plugin_init()` returning `0` means initialization succeeded.
- `croupier_plugin_info()` returns the plugin metadata and function manifest.
- `croupier_plugin_cleanup()` releases plugin-level resources.
- Plugin functions must not depend on the SDK local-listener or Agent callback model.
- Function IDs registered inside a plugin should be stable so the Dashboard, permissions, and audits can reference them.

## Related Pages
