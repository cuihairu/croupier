# vcpkg 多配置优化方案
#
# 问题：Debug 和 Release 各自安装依赖，浪费时间和存储空间
# 解决：使用预构建二进制 + 缓存 + 多配置triplet

# ========== 方案 1: 多配置 triplet ==========

# 创建自定义 triplet 文件
# vcpkg/triplets/community/x64-linux-release-shared.cmake
set(VCPKG_TARGET_ARCHITECTURE x64)
set(VCPKG_CRT_LINKAGE dynamic)
set(VCPKG_LIBRARY_LINKAGE shared)
set(VCPKG_BUILD_TYPE release)  # 只构建 Release，Debug 使用相同二进制

# 在 CMake 中使用
set(VCPKG_TARGET_TRIPLET "x64-linux-release-shared")

# 好处：
# - Debug 和 Release 共享同一套二进制文件
# - 减少 50% 的构建时间
# - Release 优化的库性能更好

# ========== 方案 2: 容器化预构建 ==========

# Dockerfile.vcpkg-deps
FROM mcr.microsoft.com/vcpkg:latest as vcpkg-builder

# 预安装所有依赖
COPY vcpkg.json .
RUN vcpkg install --triplet=x64-linux

# 多阶段构建，只复制安装结果
FROM ubuntu:22.04 as builder
COPY --from=vcpkg-builder /vcpkg/installed /opt/vcpkg/installed

# 在 CI 中直接使用预构建镜像
# 构建时间：从 15 分钟 → 2 分钟

# ========== 方案 3: 缓存优化 ==========

# GitHub Actions 缓存配置
```yaml
- name: Cache vcpkg
  uses: actions/cache@v3
  with:
    path: |
      ${{ github.workspace }}/vcpkg/installed
      ${{ github.workspace }}/vcpkg/buildtrees
    key: ${{ runner.os }}-vcpkg-${{ hashFiles('vcpkg.json') }}-${{ matrix.triplet }}
    restore-keys: |
      ${{ runner.os }}-vcpkg-${{ hashFiles('vcpkg.json') }}-
      ${{ runner.os }}-vcpkg-
```

# 缓存命中率 90%+，大幅减少重复下载编译

# ========== 方案 4: 包管理替代 ==========

# 对于常用库，直接使用系统包管理器
apt-get install -y \
    libgrpc++-dev \
    libprotobuf-dev \
    protobuf-compiler-grpc \
    nlohmann-json3-dev

# 好处：
# - 安装速度快（秒级）
# - 系统级缓存，多项目共享
# - 维护成本低

# 缺点：
# - 版本可能不是最新
# - 跨平台一致性相对较差
