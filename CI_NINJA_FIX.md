# CI构建修复：解决ninja-build命令缺失问题

## 问题描述

在GitHub Actions的SDK nightly构建过程中，CI脚本在验证构建工具时失败，错误信息显示：
```
ninja-build: command not found
Error: Process completed with exit code 127.
```

## 根本原因分析

问题出现在`.github/workflows/sdk-nightly.yml`文件的第167行，原有的验证逻辑：

```bash
which ninja || which ninja-build && (which ninja && ninja --version || which ninja-build && ninja-build --version)
```

这个命令在逻辑上有缺陷：
1. `&&`操作符的优先级导致即使`ninja-build`不存在，脚本仍会尝试执行`ninja-build --version`
2. 某些Linux环境中ninja的可执行文件名只是`ninja`，而不是`ninja-build`
3. 缺乏明确的错误处理机制

## 修复方案

### 1. 改进构建工具验证逻辑

**原有代码：**
```bash
which ninja || which ninja-build && (which ninja && ninja --version || which ninja-build && ninja-build --version)
```

**修复后：**
```bash
if which ninja >/dev/null 2>&1; then
  which ninja && ninja --version
elif which ninja-build >/dev/null 2>&1; then
  which ninja-build && ninja-build --version
else
  echo "ERROR: Neither ninja nor ninja-build found in PATH"
  exit 1
fi
```

### 2. 改进CMake配置步骤

**原有问题：**
```bash
-DCMAKE_MAKE_PROGRAM=$(which ninja || which ninja-build) \
```

**修复后：**
```bash
# Determine ninja command and generator
if which ninja >/dev/null 2>&1; then
  NINJA_CMD=$(which ninja)
  CMAKE_GENERATOR="Ninja"
elif which ninja-build >/dev/null 2>&1; then
  NINJA_CMD=$(which ninja-build)
  CMAKE_GENERATOR="Ninja"
else
  echo "ERROR: Neither ninja nor ninja-build found in PATH"
  exit 1
fi

cmake -S sdks/cpp -B build \
  -G "$CMAKE_GENERATOR" \
  -DCMAKE_MAKE_PROGRAM="$NINJA_CMD" \
  ...
```

## 修复的改进点

1. **明确的错误处理**：使用条件判断替代逻辑运算符，确保在找不到构建工具时明确报错
2. **更好的调试信息**：添加了详细的日志输出，便于问题诊断
3. **健壮的路径检测**：使用`>/dev/null 2>&1`重定向避免不必要的输出
4. **一致性保证**：确保验证步骤和配置步骤使用相同的检测逻辑

## 预期效果

修复后的CI脚本将能够：
1. 正确识别系统中可用的ninja构建工具（无论是`ninja`还是`ninja-build`）
2. 在构建工具缺失时提供清晰的错误信息
3. 确保CMake配置使用正确的构建工具路径
4. 提高CI构建的可靠性和可调试性

## 验证方法

修复可以通过以下方式验证：
1. 在本地环境模拟CI构建过程
2. 检查GitHub Actions的构建输出
3. 确认ninja工具检测和版本信息显示正常
4. 验证CMake配置步骤成功执行

## 相关文件

- `.github/workflows/sdk-nightly.yml`: 主要修复的CI配置文件
- 涉及步骤：`Verify build tools (Unix)` 和 `Configure (Release, static)`

修复完成后，CI构建应该能够正常通过ninja工具验证和CMake配置步骤。