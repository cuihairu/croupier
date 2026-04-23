# Croupier Windows x64 triplet with static library linkage and dynamic CRT (MD)
# This is the most common configuration for Windows applications
set(VCPKG_TARGET_ARCHITECTURE x64)
set(VCPKG_CRT_LINKAGE dynamic)
set(VCPKG_LIBRARY_LINKAGE static)
set(VCPKG_CMAKE_SYSTEM_NAME Windows)

# 强制 C++17 标准
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# 使用动态 CRT (/MD)
set(VCPKG_CRT_FLAGS "/MD")
