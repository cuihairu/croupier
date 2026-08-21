cmake_minimum_required(VERSION 3.20)

# Set vcpkg toolchain file before project() so it's processed during compiler detection
if(NOT CMAKE_TOOLCHAIN_FILE AND DEFINED ENV{VCPKG_ROOT})
    set(CMAKE_TOOLCHAIN_FILE "$ENV{VCPKG_ROOT}/scripts/buildsystems/vcpkg.cmake"
        CACHE STRING "Vcpkg toolchain file")
endif()

# Load version configuration
include(VERSION.cmake)

project(croupier-cpp-sdk
    VERSION ${CROUPIER_SDK_VERSION}
    DESCRIPTION "${CROUPIER_SDK_DESCRIPTION}"
    HOMEPAGE_URL "${CROUPIER_SDK_HOMEPAGE_URL}"
    LANGUAGES CXX
)

# Verify build tools are available after project setup
# Note: Changed to WARNING to support vcpkg toolchain initialization
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    if(NOT CMAKE_MAKE_PROGRAM)
        message(WARNING "CMAKE_MAKE_PROGRAM is not set. Build may fail if build tools are not installed.")
    else()
        message(STATUS "Make program: ${CMAKE_MAKE_PROGRAM}")
    endif()
    if(NOT CMAKE_CXX_COMPILER)
        message(WARNING "C++ compiler is not available. Build may fail if g++ is not installed.")
    else()
        message(STATUS "C++ compiler: ${CMAKE_CXX_COMPILER}")
    endif()
endif()

# ========== Project Configuration ==========
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Enable position independent code for shared libraries
set(CMAKE_POSITION_INDEPENDENT_CODE ON)

# Windows DLL symbol export: automatically export all symbols from shared library
# This ensures the import library (.lib) is generated when building DLLs
if(WIN32)
    set(CMAKE_WINDOWS_EXPORT_ALL_SYMBOLS ON)
endif()

# ========== Build Options ==========
option(BUILD_SHARED_LIBS "Build shared libraries" OFF)
option(BUILD_STATIC_LIBS "Build static libraries" ON)
option(BUILD_EXAMPLES "Build example programs" ON)
option(BUILD_TESTS "Build unit tests" OFF)
option(ENABLE_VCPKG "Enable vcpkg package management" ON)
option(CROUPIER_CI_BUILD "Enable CI build with proto generation" OFF)
option(ENABLE_LUA_BINDING "Enable Lua language binding (requires Lua 5.4+)" OFF)

# ========== Standalone Build Options ==========
option(CROUPIER_STANDALONE_BUILD "Enable standalone build mode" OFF)
option(CROUPIER_PREBUILT_PROTO "Use prebuilt proto files" OFF)
option(CROUPIER_ONLINE_BUILD "Enable online proto download" OFF)
option(USE_SYSTEM_PACKAGES "Use system packages instead of vcpkg" OFF)
set(CROUPIER_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/downloaded_proto" CACHE STRING "Proto files directory for online build")

# ========== vcpkg Integration (must be before proto generation) ==========
if(ENABLE_VCPKG)
    # Set vcpkg overlay ports for custom versions
    get_filename_component(PROJECT_ROOT "${CMAKE_CURRENT_LIST_DIR}" ABSOLUTE)
    if(EXISTS "${PROJECT_ROOT}/vcpkg-overlays/ports")
        set(ENV{VCPKG_OVERLAY_PORTS} "${PROJECT_ROOT}/vcpkg-overlays/ports")
        message(STATUS "Using vcpkg overlay ports: ${PROJECT_ROOT}/vcpkg-overlays/ports")
    endif()

    # vcpkg triplet configuration
    if(NOT VCPKG_TARGET_TRIPLET)
        if(CMAKE_SYSTEM_NAME STREQUAL "Windows")
            if(CMAKE_GENERATOR_PLATFORM STREQUAL "x64" OR CMAKE_SIZEOF_VOID_P EQUAL 8)
                set(VCPKG_TARGET_TRIPLET "x64-windows")
            else()
                set(VCPKG_TARGET_TRIPLET "x86-windows")
            endif()
        elseif(CMAKE_SYSTEM_NAME STREQUAL "Darwin")
            if(CMAKE_SYSTEM_PROCESSOR STREQUAL "arm64")
                set(VCPKG_TARGET_TRIPLET "arm64-osx")
            else()
                set(VCPKG_TARGET_TRIPLET "x64-osx")
            endif()
        elseif(CMAKE_SYSTEM_NAME STREQUAL "Linux")
            if(CMAKE_SYSTEM_PROCESSOR STREQUAL "aarch64")
                set(VCPKG_TARGET_TRIPLET "arm64-linux")
            else()
                set(VCPKG_TARGET_TRIPLET "x64-linux")
            endif()
        endif()
    endif()

    message(STATUS "Using vcpkg triplet: ${VCPKG_TARGET_TRIPLET}")
endif()

# ========== Proto Generation Module ==========
include(cmake/ProtoGeneration.cmake)

# Setup build type with enhanced logic for standalone builds
if(CROUPIER_STANDALONE_BUILD)
    setup_standalone_build()
else()
    setup_ci_build()
endif()

# gRPC support has been removed - using TCP transport
message(STATUS "Building with TCP transport (gRPC removed)")

# ========== Dependencies ==========
find_package(Threads REQUIRED)
find_package(CURL REQUIRED)

# TCP transport is enabled by default (gRPC removed)
set(tcp_ENABLED ON)

# Protobuf for message serialization
find_package(Protobuf CONFIG)
if(NOT Protobuf_FOUND)
    find_package(Protobuf REQUIRED)
endif()
message(STATUS "Found protobuf: ${Protobuf_VERSION}")

# The Server HTTP invoker sends and receives structured JSON; a placeholder
# parser would be unsafe because it could fabricate a successful L3 result.
find_package(nlohmann_json CONFIG REQUIRED)
message(STATUS "Found nlohmann_json: ${nlohmann_json_VERSION}")
set(JSON_LIBRARY nlohmann_json::nlohmann_json)

# In current HTTP/JSON mode we do not regenerate proto sources in CI, so fall
# back to the checked-in generated invocation bindings when they were not
# populated by ProtoGeneration.cmake.
if(tcp_ENABLED AND (NOT DEFINED GENERATED_PROTO_SOURCES OR NOT GENERATED_PROTO_SOURCES))
    set(_sdk_generated_proto_dir "${CMAKE_CURRENT_SOURCE_DIR}/generated")
    set(_sdk_generated_invocation_cc "${_sdk_generated_proto_dir}/croupier/sdk/v1/invocation.pb.cc")
    set(_sdk_generated_invocation_h "${_sdk_generated_proto_dir}/croupier/sdk/v1/invocation.pb.h")
    set(_sdk_generated_provider_cc "${_sdk_generated_proto_dir}/croupier/sdk/v1/provider.pb.cc")
    set(_sdk_generated_provider_h "${_sdk_generated_proto_dir}/croupier/sdk/v1/provider.pb.h")
    if(EXISTS "${_sdk_generated_invocation_cc}" AND EXISTS "${_sdk_generated_invocation_h}" AND
       EXISTS "${_sdk_generated_provider_cc}" AND EXISTS "${_sdk_generated_provider_h}")
        set(PROTO_GENERATED_DIR "${_sdk_generated_proto_dir}")
        set(GENERATED_PROTO_SOURCES
            "${_sdk_generated_invocation_cc}"
            "${_sdk_generated_provider_cc}")
        set(GENERATED_PROTO_HEADERS
            "${_sdk_generated_invocation_h}"
            "${_sdk_generated_provider_h}")
        message(STATUS "Using checked-in generated invocation/provider protobuf sources")
    endif()
endif()

# ========== Lua Binding ==========
if(ENABLE_LUA_BINDING)
    # Set up search paths for vcpkg-installed packages when using manifest mode
    if(DEFINED ENV{VCPKG_ROOT} AND NOT LUA_FOUND)
        list(APPEND CMAKE_PREFIX_PATH "$ENV{VCPKG_ROOT}/installed/x64-windows" "$ENV{VCPKG_ROOT}/installed/x64-windows-static")
    endif()

    # Try vcpkg's unofficial-lua first (vcpkg uses this name)
    find_package(unofficial-lua CONFIG QUIET)
    if(unofficial-lua_FOUND)
        set(LUA_FOUND TRUE)
        set(LUA_INCLUDE_DIR "$ENV{VCPKG_ROOT}/installed/x64-windows/include" "$ENV{VCPKG_ROOT}/installed/x64-windows-static/include")
        set(LUA_LIBRARIES unofficial::lua::lua)
        set(LUA_VERSION_STRING "5.5.0")
    endif()

    # Try standard Lua package names
    if(NOT LUA_FOUND)
        find_package(Lua 5.5 QUIET)
        if(NOT Lua_FOUND)
            find_package(Lua 5.4 QUIET)
        endif()
    endif()
    if(NOT LUA_FOUND)
        find_package(PkgConfig)
        if(PKG_CONFIG_FOUND)
            pkg_check_modules(LUA lua5.5 lua5.4 lua)
        endif()
    endif()

    # Find sol2 or use FetchContent
    # First try vcpkg-installed sol2 (may be in global vcpkg if not in manifest)
    if(DEFINED ENV{VCPKG_ROOT} AND NOT sol2_FOUND)
        set(CMAKE_PREFIX_PATH "$ENV{VCPKG_ROOT}/installed/x64-windows;$ENV{VCPKG_ROOT}/installed/x64-windows-static;${CMAKE_PREFIX_PATH}")
    endif()
    find_package(sol2 CONFIG QUIET)
    if(sol2_FOUND)
        message(STATUS "Found sol2 via find_package (vcpkg or system)")
        set(SOL2_INCLUDE_DIR "$ENV{VCPKG_ROOT}/installed/x64-windows/include")
        if(NOT EXISTS "${SOL2_INCLUDE_DIR}/sol2/sol.hpp")
            set(SOL2_INCLUDE_DIR "$ENV{VCPKG_ROOT}/installed/x64-windows-static/include")
        endif()
    else()
        message(STATUS "sol2 not found via find_package, using FetchContent...")
        include(FetchContent)
        FetchContent_Declare(
            sol2
            GIT_REPOSITORY https://github.com/ThePhD/sol2.git
            GIT_TAG v3.3.1
            GIT_SHALLOW TRUE
        )
        # sol2 is header-only, just need to fetch and include
        FetchContent_MakeAvailable(sol2)
        set(SOL2_INCLUDE_DIR "${sol2_SOURCE_DIR}")
        message(STATUS "sol2 fetched via FetchContent: ${SOL2_INCLUDE_DIR}")
    endif()

    if(NOT (Lua_FOUND OR LUA_FOUND))
        message(FATAL_ERROR "ENABLE_LUA_BINDING=ON but Lua 5.4+ was not found. Install Lua-devel/lua-dev")
    endif()

    message(STATUS "Lua binding enabled (sol2)")
    if(Lua_FOUND)
        message(STATUS "Found Lua: ${LUA_VERSION_STRING}")
        set(LUA_INCLUDE_DIRS ${LUA_INCLUDE_DIR})
        set(LUA_LIBRARIES ${LUA_LIBRARIES})
    else()
        message(STATUS "Found Lua: ${LUA_VERSION}")
    endif()
endif()

# ========== MSVC Runtime Library Settings ==========
if(MSVC)
    # Enable CMP0091 policy to allow setting MSVC runtime library
    if(POLICY CMP0091)
        cmake_policy(SET CMP0091 NEW)
    endif()

    # Option to choose between static (MT/MTd) and dynamic (MD/MDd) runtime
    # Default to dynamic runtime (MD/MDd) to match vcpkg's standard x64-windows triplet
    # Use static runtime only when explicitly specified or using x64-windows-static triplet
    if(VCPKG_TARGET_TRIPLET MATCHES "static$")
        # Static triplet (x64-windows-static) defaults to static runtime
        option(CROUPIER_USE_STATIC_RUNTIME "Use static runtime library (MT/MTd) instead of dynamic (MD/MDd)" ON)
    else()
        # Standard triplet (x64-windows) defaults to dynamic runtime
        option(CROUPIER_USE_STATIC_RUNTIME "Use static runtime library (MT/MTd) instead of dynamic (MD/MDd)" OFF)
    endif()

    if(CROUPIER_USE_STATIC_RUNTIME)
        # Use static runtime: MT for Release, MTd for Debug
        set(_CROUPIER_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
        message(STATUS "Croupier SDK: Using static runtime library (MT/MTd)")
    else()
        # Use dynamic runtime: MD for Release, MDd for Debug
        set(_CROUPIER_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>DLL")
        message(STATUS "Croupier SDK: Using dynamic runtime library (MD/MDd)")
    endif()

    set(CMAKE_MSVC_RUNTIME_LIBRARY "${_CROUPIER_MSVC_RUNTIME_LIBRARY}")
endif()

# ========== Compiler Settings ==========
if(MSVC)
    # MSVC specific settings
    # Use /W4 for SDK code but disable warnings-as-errors for specific warnings
    # from protobuf generated code
    add_compile_options(/W4 /utf-8)
    add_compile_definitions(_CRT_SECURE_NO_WARNINGS)
    # Disable C4100 (unreferenced parameter) and C4267 (size_t to int conversion)
    # for generated protobuf code - these are common in generated code
    add_compile_options(/wd4100 /wd4267)
    # Keep WX for most code, but these warnings are unavoidable in generated code
    # Note: We rely on CI to catch real errors, generated code warnings are acceptable
else()
    # GCC/Clang settings
    add_compile_options(-Wall -Wextra -Werror)

    # Suppress warnings from third-party libraries (protobuf, gRPC)
    add_compile_options(
        -Wno-deprecated-declarations    # Suppress protobuf deprecated API warnings
        -Wno-unused-parameter           # Suppress gRPC unused parameter warnings
        -Wno-sign-compare               # Suppress sign comparison warnings in generated code
    )

    # Enable debug info in release builds for better profiling
    if(CMAKE_BUILD_TYPE STREQUAL "Release")
    add_compile_options(-g)
    endif()
endif()

# ========== Generated Proto Files ==========
# `cmake/ProtoGeneration.cmake` populates:
# ========== Source Files ==========
# SDK source and header files
set(SDK_SOURCES
    src/croupier_client.cpp
    src/http_transport.cpp
    src/tcp_transport.cpp
    src/utils/json_utils.cpp
    src/utils/file_utils.cpp
    src/config/client_config_loader.cpp
    src/plugin/dynamic_loader.cpp
)

set(SDK_HEADERS
    include/croupier/sdk/croupier_client.h
    include/croupier/sdk/http_transport.h
    include/croupier/sdk/logger.h
    include/croupier/sdk/tcp_transport.h
    include/croupier/sdk/protocol.h
    include/croupier/sdk/utils/json_utils.h
    include/croupier/sdk/utils/file_utils.h
    include/croupier/sdk/config/client_config_loader.h
    include/croupier/sdk/plugin/dynamic_loader.h
)

# Add Lua binding source files if enabled (using sol2)
if(ENABLE_LUA_BINDING)
    list(APPEND SDK_SOURCES src/bindings/lua_binding_sol2.cpp)
    list(APPEND SDK_HEADERS include/croupier/sdk/bindings/lua_binding_sol2.h)
    message(STATUS "Added Lua binding source files (sol2)")
endif()

if(DEFINED GENERATED_PROTO_SOURCES AND GENERATED_PROTO_SOURCES)
    list(APPEND SDK_SOURCES ${GENERATED_PROTO_SOURCES})
endif()

if(DEFINED GENERATED_PROTO_HEADERS AND GENERATED_PROTO_HEADERS)
    list(APPEND SDK_HEADERS ${GENERATED_PROTO_HEADERS})
endif()


# ========== Library Targets ==========

# Shared Library
if(BUILD_SHARED_LIBS)
    add_library(croupier-sdk-shared SHARED ${SDK_SOURCES} ${SDK_HEADERS})
    set_target_properties(croupier-sdk-shared PROPERTIES
        OUTPUT_NAME "croupier-sdk"
        VERSION ${PROJECT_VERSION}
        SOVERSION ${PROJECT_VERSION_MAJOR}
        EXPORT_NAME "croupier-sdk"
    )

    # Set output directories for the shared library (vcpkg-compatible layout)
    # Release: RUNTIME → bin/, LIBRARY → lib/, ARCHIVE → lib/
    # Debug: RUNTIME → debug/bin/, LIBRARY → debug/lib/, ARCHIVE → debug/lib/
    set_target_properties(croupier-sdk-shared PROPERTIES
        RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/bin
        RUNTIME_OUTPUT_DIRECTORY_DEBUG ${CMAKE_BINARY_DIR}/debug/bin
        RUNTIME_OUTPUT_DIRECTORY_RELEASE ${CMAKE_BINARY_DIR}/bin
        RUNTIME_OUTPUT_DIRECTORY_RELWITHDEBINFO ${CMAKE_BINARY_DIR}/bin
        RUNTIME_OUTPUT_DIRECTORY_MINSIZEREL ${CMAKE_BINARY_DIR}/bin
        LIBRARY_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/lib
        LIBRARY_OUTPUT_DIRECTORY_DEBUG ${CMAKE_BINARY_DIR}/debug/lib
        LIBRARY_OUTPUT_DIRECTORY_RELEASE ${CMAKE_BINARY_DIR}/lib
        LIBRARY_OUTPUT_DIRECTORY_RELWITHDEBINFO ${CMAKE_BINARY_DIR}/lib
        LIBRARY_OUTPUT_DIRECTORY_MINSIZEREL ${CMAKE_BINARY_DIR}/lib
        ARCHIVE_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/lib
        ARCHIVE_OUTPUT_DIRECTORY_DEBUG ${CMAKE_BINARY_DIR}/debug/lib
        ARCHIVE_OUTPUT_DIRECTORY_RELEASE ${CMAKE_BINARY_DIR}/lib
        ARCHIVE_OUTPUT_DIRECTORY_RELWITHDEBINFO ${CMAKE_BINARY_DIR}/lib
        ARCHIVE_OUTPUT_DIRECTORY_MINSIZEREL ${CMAKE_BINARY_DIR}/lib
    )

    if(MSVC)
        set_property(TARGET croupier-sdk-shared PROPERTY
            MSVC_RUNTIME_LIBRARY "${_CROUPIER_MSVC_RUNTIME_LIBRARY}")
    endif()


    target_include_directories(croupier-sdk-shared
        PUBLIC
            $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
            $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}>
            $<INSTALL_INTERFACE:include>
    )
    if(DEFINED PROTO_GENERATED_DIR AND PROTO_GENERATED_DIR)
        target_include_directories(croupier-sdk-shared PUBLIC ${PROTO_GENERATED_DIR})
    else()
        # Fallback: use checked-in generated proto files
        set(_sdk_generated_proto_dir "${CMAKE_CURRENT_SOURCE_DIR}/generated")
        if(EXISTS "${_sdk_generated_proto_dir}/croupier/sdk/v1/invocation.pb.h")
            target_include_directories(croupier-sdk-shared PUBLIC ${_sdk_generated_proto_dir})
            message(STATUS "Adding checked-in proto directory to include path: ${_sdk_generated_proto_dir}")
        endif()
    endif()

    # Link dependencies
    target_link_libraries(croupier-sdk-shared
        PRIVATE
            Threads::Threads
            CURL::libcurl
    )

    if(tcp_ENABLED)
        target_link_libraries(croupier-sdk-shared
            PUBLIC
                
        )
        target_compile_definitions(croupier-sdk-shared PUBLIC CROUPIER_SDK_HAS_TCP)
    endif()

    # Link Protobuf for message serialization
    target_link_libraries(croupier-sdk-shared
        PUBLIC
            protobuf::libprotobuf
    )

    if(JSON_LIBRARY)
        target_link_libraries(croupier-sdk-shared
            PUBLIC
                ${JSON_LIBRARY}
        )
        target_compile_definitions(croupier-sdk-shared
            PUBLIC
                CROUPIER_SDK_ENABLE_JSON
        )
    endif()

    if(ENABLE_LUA_BINDING)
        target_include_directories(croupier-sdk-shared PRIVATE ${LUA_INCLUDE_DIRS})
        target_link_libraries(croupier-sdk-shared PRIVATE ${LUA_LIBRARIES})
        if(sol2_FOUND)
            # vcpkg or system-installed sol2
            target_include_directories(croupier-sdk-shared PRIVATE ${SOL2_INCLUDE_DIR})
            target_link_libraries(croupier-sdk-shared PRIVATE sol2::sol2)
        elseif(TARGET sol2)
            # sol2 was fetched via FetchContent - add include dir explicitly
            target_include_directories(croupier-sdk-shared PRIVATE ${SOL2_INCLUDE_DIR})
            target_link_libraries(croupier-sdk-shared PRIVATE sol2)
        endif()
        target_compile_definitions(croupier-sdk-shared PUBLIC CROUPIER_SDK_ENABLE_LUA)
        # Define version macro for Lua binding
        target_compile_definitions(croupier-sdk-shared PRIVATE "CROUPIER_SDK_VERSION=\"${PROJECT_VERSION}\"")
    endif()


    target_compile_definitions(croupier-sdk-shared
        PRIVATE
            CROUPIER_SDK_BUILDING_DLL
        PUBLIC
            CROUPIER_SDK_SHARED
    )

    # Alias for easier usage
    add_library(croupier::sdk-shared ALIAS croupier-sdk-shared)
endif()

# Static Library
if(BUILD_STATIC_LIBS)
    add_library(croupier-sdk-static STATIC ${SDK_SOURCES} ${SDK_HEADERS})
    set_target_properties(croupier-sdk-static PROPERTIES
        OUTPUT_NAME "croupier-sdk-static"
        EXPORT_NAME "croupier-sdk"
        # vcpkg-style: Release in lib/, Debug in debug/lib/
        ARCHIVE_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/lib
        ARCHIVE_OUTPUT_DIRECTORY_DEBUG ${CMAKE_BINARY_DIR}/debug/lib
        ARCHIVE_OUTPUT_DIRECTORY_RELEASE ${CMAKE_BINARY_DIR}/lib
        ARCHIVE_OUTPUT_DIRECTORY_RELWITHDEBINFO ${CMAKE_BINARY_DIR}/lib
        ARCHIVE_OUTPUT_DIRECTORY_MINSIZEREL ${CMAKE_BINARY_DIR}/lib
    )

    target_include_directories(croupier-sdk-static
        PUBLIC
            $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
            $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}>
            $<INSTALL_INTERFACE:include>
    )
    if(DEFINED PROTO_GENERATED_DIR AND PROTO_GENERATED_DIR)
        target_include_directories(croupier-sdk-static PUBLIC ${PROTO_GENERATED_DIR})
    else()
        # Fallback: use checked-in generated proto files
        set(_sdk_generated_proto_dir "${CMAKE_CURRENT_SOURCE_DIR}/generated")
        if(EXISTS "${_sdk_generated_proto_dir}/croupier/sdk/v1/invocation.pb.h")
            target_include_directories(croupier-sdk-static PUBLIC ${_sdk_generated_proto_dir})
            message(STATUS "Adding checked-in proto directory to include path (static): ${_sdk_generated_proto_dir}")
        endif()
    endif()

    # For static library, we still link dependencies publicly for user projects
    target_link_libraries(croupier-sdk-static
        PUBLIC
            Threads::Threads
            CURL::libcurl
    )

    if(tcp_ENABLED)
        target_link_libraries(croupier-sdk-static
            PUBLIC
                
        )
        target_compile_definitions(croupier-sdk-static PUBLIC CROUPIER_SDK_HAS_TCP)
    endif()

    # Link Protobuf for message serialization
    target_link_libraries(croupier-sdk-static
        PUBLIC
            protobuf::libprotobuf
    )

    if(JSON_LIBRARY)
        target_link_libraries(croupier-sdk-static
            PUBLIC
                ${JSON_LIBRARY}
        )
        target_compile_definitions(croupier-sdk-static
            PUBLIC
                CROUPIER_SDK_ENABLE_JSON
        )
    endif()

    if(ENABLE_LUA_BINDING)
        target_include_directories(croupier-sdk-static PRIVATE ${LUA_INCLUDE_DIRS})
        target_link_libraries(croupier-sdk-static PRIVATE ${LUA_LIBRARIES})
        if(sol2_FOUND)
            # vcpkg or system-installed sol2
            target_include_directories(croupier-sdk-static PRIVATE ${SOL2_INCLUDE_DIR})
            target_link_libraries(croupier-sdk-static PRIVATE sol2::sol2)
        elseif(TARGET sol2)
            # sol2 was fetched via FetchContent - add include dir explicitly
            target_include_directories(croupier-sdk-static PRIVATE ${SOL2_INCLUDE_DIR})
            target_link_libraries(croupier-sdk-static PRIVATE sol2)
        endif()
        target_compile_definitions(croupier-sdk-static PUBLIC CROUPIER_SDK_ENABLE_LUA)
        # Define version macro for Lua binding
        target_compile_definitions(croupier-sdk-static PRIVATE "CROUPIER_SDK_VERSION=\"${PROJECT_VERSION}\"")
    endif()


    target_compile_definitions(croupier-sdk-static
        PUBLIC
            CROUPIER_SDK_STATIC
    )

    # Apply MSVC runtime library setting to static library target
    if(MSVC)
        set_property(TARGET croupier-sdk-static PROPERTY
            MSVC_RUNTIME_LIBRARY "${_CROUPIER_MSVC_RUNTIME_LIBRARY}")
    endif()

    # On Windows, we do NOT merge static libraries
    # Users should link against the required gRPC dependencies explicitly
    # This keeps library sizes reasonable and avoids version conflicts
    # See docs/INTEGRATION.md for required dependencies list

    # Alias for easier usage
    add_library(croupier::sdk-static ALIAS croupier-sdk-static)
endif()

# Default library target (shared if available, otherwise static)
if(BUILD_SHARED_LIBS)
    add_library(croupier::sdk ALIAS croupier-sdk-shared)
elseif(BUILD_STATIC_LIBS)
    add_library(croupier::sdk ALIAS croupier-sdk-static)
endif()

# ========== Determine SDK Library Target for Linking ==========
# Determine which library target to use for linking (examples and tests)
# This ensures direct target linking instead of alias resolution issues
if(BUILD_SHARED_LIBS)
    set(_CROUPIER_SDK_TARGET croupier-sdk-shared)
elseif(BUILD_STATIC_LIBS)
    set(_CROUPIER_SDK_TARGET croupier-sdk-static)
else()
    message(FATAL_ERROR "At least one of BUILD_SHARED_LIBS or BUILD_STATIC_LIBS must be ON")
endif()

# ========== Example Programs ==========
if(BUILD_EXAMPLES)
    # Add library search path for Windows import libraries
    link_directories(${CMAKE_BINARY_DIR}/lib)

    # Basic example
    add_executable(croupier-example examples/example.cpp)
    target_link_libraries(croupier-example
        PRIVATE
            ${_CROUPIER_SDK_TARGET}
    )
    add_dependencies(croupier-example ${_CROUPIER_SDK_TARGET})

    # Advanced Configuration Example
    add_executable(croupier-config-example examples/config_example.cpp)
    target_link_libraries(croupier-config-example
        PRIVATE
            ${_CROUPIER_SDK_TARGET}
    )
    add_dependencies(croupier-config-example ${_CROUPIER_SDK_TARGET})

    # Plugin System Demo
    add_executable(croupier-plugin-demo examples/plugin_demo.cpp)
    target_link_libraries(croupier-plugin-demo
        PRIVATE
            ${_CROUPIER_SDK_TARGET}
    )
    add_dependencies(croupier-plugin-demo ${_CROUPIER_SDK_TARGET})

    # Game Demo (19 functions, matching Go SDK demo)
    add_executable(croupier-game-demo examples/game_demo.cpp)
    target_link_libraries(croupier-game-demo
        PRIVATE
            ${_CROUPIER_SDK_TARGET}
    )
    add_dependencies(croupier-game-demo ${_CROUPIER_SDK_TARGET})

    # Example Plugin (shared library)
    add_library(example_plugin SHARED examples/plugins/example_plugin.cpp)
    target_include_directories(example_plugin
        PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
    )

    # Platform-specific settings for plugin
    if(WIN32)
        # Windows DLL export settings
        target_compile_definitions(example_plugin PRIVATE BUILDING_PLUGIN_DLL)
        set_target_properties(example_plugin PROPERTIES
            PREFIX ""
            SUFFIX ".dll"
        )
    elseif(APPLE)
        # macOS dylib settings
        set_target_properties(example_plugin PROPERTIES
            PREFIX "lib"
            SUFFIX ".dylib"
        )
    else()
        # Linux shared library settings
        set_target_properties(example_plugin PROPERTIES
            PREFIX "lib"
            SUFFIX ".so"
        )
    endif()

    # Set output directory for plugins
    set_target_properties(example_plugin PROPERTIES
        LIBRARY_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/plugins
        RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/plugins
    )

    # Set output directory for examples
    set_target_properties(croupier-example croupier-config-example croupier-plugin-demo croupier-game-demo PROPERTIES
        RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/bin
    )
endif()

# ========== Unit Tests ==========
if(BUILD_TESTS)
    enable_testing()

    find_package(GTest CONFIG QUIET)
    if(NOT GTest_FOUND)
        include(FetchContent)

        # Keep this minimal: only used when GTest isn't provided by vcpkg/system packages.
        # Pin to a stable tag to avoid test flakiness from upstream changes.
        FetchContent_Declare(
            googletest
            GIT_REPOSITORY https://github.com/google/googletest.git
            GIT_TAG v1.15.2
        )

        set(INSTALL_GTEST OFF CACHE BOOL "" FORCE)
        set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
        FetchContent_MakeAvailable(googletest)
    endif()

    # Targets are available either from `find_package(GTest)` (vcpkg/system) or FetchContent.
    if(TARGET GTest::gtest_main OR TARGET gtest_main)
        set(CROUPIER_TEST_SOURCES
            tests/test_utils.cpp
            tests/test_json_utils.cpp
            tests/test_file_utils.cpp
            tests/test_main_thread_dispatcher.cpp
            tests/test_config_loading.cpp
            tests/test_config_network.cpp
            tests/test_config_environment.cpp
            tests/test_config_security.cpp
            tests/test_config_merge.cpp
            tests/test_config_profiles.cpp
            tests/test_config_defaults.cpp
            tests/test_client_initialization.cpp
            tests/test_client_function_registration.cpp
            tests/test_client_connection.cpp
            tests/test_client_lifecycle.cpp
            tests/test_invoker_fallback.cpp
            tests/test_invoker.cpp
            tests/test_plugin_registry.cpp
            tests/test_tcp_transport_errors.cpp
            tests/test_plugin_manager.cpp
            tests/test_protocol.cpp
            tests/test_logger.cpp
            tests/test_http_transport.cpp
            tests/test_client_provider.cpp
            tests/test_dynamic_loader_lifecycle.cpp
            tests/test_json_schema.cpp
            tests/test_config_generation.cpp
            tests/test_invoker_retry.cpp
        )

        # Real shared-object plugins used by the dynamic loader tests.
        if(UNIX)
            add_library(croupier-sample-plugin SHARED tests/plugins/sample_plugin.cpp)
            target_include_directories(croupier-sample-plugin PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/include)
            set_target_properties(croupier-sample-plugin PROPERTIES
                LIBRARY_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/test-plugins
                PREFIX "lib"
            )
            add_library(croupier-failing-init-plugin SHARED tests/plugins/failing_init_plugin.cpp)
            target_include_directories(croupier-failing-init-plugin PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/include)
            set_target_properties(croupier-failing-init-plugin PROPERTIES
                LIBRARY_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/test-plugins
                PREFIX "lib"
            )
        endif()

        add_executable(croupier-sdk-tests
            ${CROUPIER_TEST_SOURCES}
        )

        # Ensure TCP tests are compiled when TCP is enabled
        if(tcp_ENABLED)
            target_compile_definitions(croupier-sdk-tests PRIVATE CROUPIER_SDK_HAS_TCP)
        endif()

        if(UNIX)
            add_dependencies(croupier-sdk-tests croupier-sample-plugin croupier-failing-init-plugin)
            target_compile_definitions(croupier-sdk-tests PRIVATE
                CROUPIER_TEST_PLUGIN_DIR="${CMAKE_BINARY_DIR}/test-plugins")
        endif()

        if(TARGET GTest::gtest_main)
            set(_croupier_gtest_main GTest::gtest_main)
        else()
            set(_croupier_gtest_main gtest_main)
        endif()

        # On Windows, always use static linking for tests to avoid DLL-related crashes
        # This fixes SEGFAULT issues in CI when gRPC is built as a DLL
        if(WIN32 AND TARGET croupier-sdk-static)
            target_link_libraries(croupier-sdk-tests
                PRIVATE
                    croupier-sdk-static
                    ${_croupier_gtest_main}
            )
            add_dependencies(croupier-sdk-tests croupier-sdk-static)
        else()
            target_link_libraries(croupier-sdk-tests
                PRIVATE
                    ${_CROUPIER_SDK_TARGET}
                    ${_croupier_gtest_main}
            )
            add_dependencies(croupier-sdk-tests ${_CROUPIER_SDK_TARGET})
        endif()

        add_test(NAME CroupierSDKTests COMMAND croupier-sdk-tests)
    else()
        message(WARNING "BUILD_TESTS=ON but GTest is unavailable; skipping croupier-sdk-tests target")
    endif()
endif()

# ========== Install Rules ==========
include(GNUInstallDirs)

# Public SDK headers
install(DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}/include/"
    DESTINATION "${CMAKE_INSTALL_INCLUDEDIR}"
)


if(BUILD_SHARED_LIBS)
    install(TARGETS croupier-sdk-shared
        RUNTIME DESTINATION "${CMAKE_INSTALL_BINDIR}"
        LIBRARY DESTINATION "${CMAKE_INSTALL_LIBDIR}"
        ARCHIVE DESTINATION "${CMAKE_INSTALL_LIBDIR}"
    )
endif()

if(BUILD_STATIC_LIBS)
    install(TARGETS croupier-sdk-static
        ARCHIVE DESTINATION "${CMAKE_INSTALL_LIBDIR}"
    )
endif()

# ========== Package Information ==========
set(CPACK_PACKAGE_NAME "croupier-cpp-sdk")
set(CPACK_PACKAGE_VERSION ${PROJECT_VERSION})
set(CPACK_PACKAGE_DESCRIPTION_SUMMARY "Croupier C++ SDK for Function Provider and Invoker")
set(CPACK_PACKAGE_VENDOR "Croupier Project")
set(CPACK_PACKAGE_CONTACT "support@croupier.dev")

# Platform-specific package settings
if(WIN32)
    set(CPACK_GENERATOR "ZIP;NSIS")
elseif(APPLE)
    set(CPACK_GENERATOR "TGZ;DragNDrop")
else()
    set(CPACK_GENERATOR "TGZ;DEB;RPM")
    set(CPACK_DEBIAN_PACKAGE_MAINTAINER "Croupier Project <support@croupier.dev>")
    set(CPACK_RPM_PACKAGE_LICENSE "MIT")
endif()

include(CPack)

# ========== Summary ==========
message(STATUS "")
message(STATUS "========== Build Configuration Summary ==========")
message(STATUS "Project Name:    ${PROJECT_NAME}")
message(STATUS "Version:         ${PROJECT_VERSION}")
message(STATUS "Build Type:      ${CMAKE_BUILD_TYPE}")
message(STATUS "C++ Standard:    ${CMAKE_CXX_STANDARD}")
message(STATUS "")
message(STATUS "Options:")
message(STATUS "  BUILD_SHARED_LIBS: ${BUILD_SHARED_LIBS}")
message(STATUS "  BUILD_STATIC_LIBS: ${BUILD_STATIC_LIBS}")
message(STATUS "  BUILD_EXAMPLES:    ${BUILD_EXAMPLES}")
message(STATUS "  BUILD_TESTS:       ${BUILD_TESTS}")
message(STATUS "  ENABLE_VCPKG:      ${ENABLE_VCPKG}")
message(STATUS "  gRPC support:     Removed (HTTP/JSON only)")
message(STATUS "  ENABLE_LUA_BINDING: ${ENABLE_LUA_BINDING}")
if(ENABLE_VCPKG)
message(STATUS "  VCPKG_TRIPLET:     ${VCPKG_TARGET_TRIPLET}")
endif()
message(STATUS "")
message(STATUS "Dependencies:")
if(nlohmann_json_FOUND)
message(STATUS "  nlohmann_json:   Found")
endif()
if(ENABLE_LUA_BINDING)
    message(STATUS "  Lua:             Enabled (${LUA_VERSION_STRING})")
endif()
message(STATUS "=================================================")
message(STATUS "")
