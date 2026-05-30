# Croupier Windows x64 triplet with static library linkage and static CRT (MT)
# Use this for standalone applications that don't want to depend on msvcp*.dll
set(VCPKG_TARGET_ARCHITECTURE x64)
set(VCPKG_CRT_LINKAGE static)
set(VCPKG_LIBRARY_LINKAGE static)
set(VCPKG_CMAKE_SYSTEM_NAME Windows)

# 强制 C++17 标准
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# 使用静态 CRT (/MT)
set(VCPKG_CRT_FLAGS "/MT")
