cmake_minimum_required(VERSION 3.20)

# Wrapper around vcpkg's macOS toolchain that:
# - forces C++17 (required by modern Abseil/Protobuf)
# - makes the vcpkg include directory win over AppleClang's implicit `/usr/local/include`

include("${CMAKE_CURRENT_LIST_DIR}/../vcpkg/scripts/toolchains/osx.cmake")

# Ensure a consistent baseline across all vcpkg ports.
set(CMAKE_CXX_STANDARD 17 CACHE STRING "C++ standard" FORCE)
set(CMAKE_CXX_STANDARD_REQUIRED ON CACHE BOOL "Require C++ standard" FORCE)
set(CMAKE_CXX_EXTENSIONS OFF CACHE BOOL "Disable compiler extensions" FORCE)

if(NOT DEFINED VCPKG_INSTALLED_DIR AND DEFINED _VCPKG_INSTALLED_DIR)
  set(VCPKG_INSTALLED_DIR "${_VCPKG_INSTALLED_DIR}")
endif()

if(DEFINED VCPKG_INSTALLED_DIR AND DEFINED VCPKG_TARGET_TRIPLET)
  set(_CROUPIER_VCPKG_INCLUDE_DIR "${VCPKG_INSTALLED_DIR}/${VCPKG_TARGET_TRIPLET}/include")
  if(EXISTS "${_CROUPIER_VCPKG_INCLUDE_DIR}")
    foreach(_lang IN ITEMS C CXX OBJC OBJCXX)
      set(_flags_var "CMAKE_${_lang}_FLAGS_INIT")
      if(DEFINED ${_flags_var} AND NOT "${${_flags_var}}" STREQUAL "")
        set(${_flags_var} "-I${_CROUPIER_VCPKG_INCLUDE_DIR} ${${_flags_var}}")
      else()
        set(${_flags_var} "-I${_CROUPIER_VCPKG_INCLUDE_DIR}")
      endif()
    endforeach()
  endif()
endif()
