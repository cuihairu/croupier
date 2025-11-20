cmake_minimum_required(VERSION 3.20)

project(croupier-cpp-sdk
    VERSION 0.1.0
    DESCRIPTION "Croupier C++ SDK for Game Function Registration"
    LANGUAGES CXX
)

# ========== Project Configuration ==========
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Enable position independent code for shared libraries
set(CMAKE_POSITION_INDEPENDENT_CODE ON)

# ========== Build Options ==========
option(BUILD_SHARED_LIBS "Build shared libraries" ON)
option(BUILD_STATIC_LIBS "Build static libraries" ON)
option(BUILD_EXAMPLES "Build example programs" ON)
option(BUILD_TESTS "Build unit tests" OFF)
option(ENABLE_VCPKG "Enable vcpkg package management" ON)
option(CROUPIER_CI_BUILD "Enable CI build with proto generation" OFF)

# ========== Standalone Build Options ==========
option(CROUPIER_STANDALONE_BUILD "Enable standalone build mode" OFF)
option(CROUPIER_PREBUILT_PROTO "Use prebuilt proto files" OFF)
option(CROUPIER_ONLINE_BUILD "Enable online proto download" OFF)
option(USE_SYSTEM_PACKAGES "Use system packages instead of vcpkg" OFF)
set(CROUPIER_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/downloaded_proto" CACHE STRING "Proto files directory for online build")

# ========== Proto Generation Module ==========
include(cmake/ProtoGeneration.cmake)

# Setup build type with enhanced logic for standalone builds
if(CROUPIER_STANDALONE_BUILD)
    setup_standalone_build()
else()
    setup_ci_build()
endif()

# Set gRPC enable flag based on CI build result
if(CROUPIER_SDK_ENABLE_GRPC)
    message(STATUS "Building with real gRPC implementation")
    set(ENABLE_GRPC ON)
else()
    message(STATUS "Building with mock gRPC implementation")
    set(ENABLE_GRPC OFF)
endif()

# ========== vcpkg Integration ==========
if(ENABLE_VCPKG)
    # Set vcpkg toolchain file if not already set
    if(NOT CMAKE_TOOLCHAIN_FILE AND DEFINED ENV{VCPKG_ROOT})
        set(CMAKE_TOOLCHAIN_FILE "$ENV{VCPKG_ROOT}/scripts/buildsystems/vcpkg.cmake"
            CACHE STRING "Vcpkg toolchain file")
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

# ========== Dependencies ==========
find_package(Threads REQUIRED)

# gRPC and Protobuf for real implementation
if(ENABLE_GRPC)
    find_package(gRPC CONFIG REQUIRED)
    find_package(Protobuf CONFIG REQUIRED)

    message(STATUS "Found gRPC: ${gRPC_VERSION}")
    message(STATUS "Found Protobuf: ${Protobuf_VERSION}")

    set(GRPC_LIBRARIES
        gRPC::grpc++
        gRPC::grpc++_reflection
        protobuf::libprotobuf
    )
endif()

# JSON library for better JSON support (via vcpkg)
find_package(nlohmann_json CONFIG)
if(nlohmann_json_FOUND)
    message(STATUS "Found nlohmann_json: ${nlohmann_json_VERSION}")
    set(JSON_LIBRARY nlohmann_json::nlohmann_json)
else()
    message(STATUS "nlohmann_json not found, using built-in JSON utilities")
    set(JSON_LIBRARY "")
endif()

# ========== Compiler Settings ==========
if(MSVC)
    # MSVC specific settings
    add_compile_options(/W4 /WX)
    add_compile_definitions(_CRT_SECURE_NO_WARNINGS)
else()
    # GCC/Clang settings
    add_compile_options(-Wall -Wextra -Werror)

    # Enable debug info in release builds for better profiling
    if(CMAKE_BUILD_TYPE STREQUAL "Release")
        add_compile_options(-g)
    endif()
endif()

# ========== Generated Proto Files ==========
set(GENERATED_PROTO_SOURCES "")
set(GENERATED_PROTO_HEADERS "")

if(ENABLE_GRPC)
    # Monorepo mode: Use main project's generated files
    set(MAIN_PROJECT_GEN_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../gen")

    message(STATUS "🏠 Monorepo build detected")
    message(STATUS "Using generated files from: ${MAIN_PROJECT_GEN_DIR}")

    # Check if main project generated files exist
    if(EXISTS ${MAIN_PROJECT_GEN_DIR})
        # Collect all generated C++ proto files from main project
        file(GLOB_RECURSE GENERATED_PROTO_SOURCES "${MAIN_PROJECT_GEN_DIR}/**/*.cc")
        file(GLOB_RECURSE GENERATED_PROTO_HEADERS "${MAIN_PROJECT_GEN_DIR}/**/*.h")

        if(GENERATED_PROTO_SOURCES)
            list(LENGTH GENERATED_PROTO_SOURCES file_count)
            message(STATUS "✅ Found ${file_count} generated proto source files")
            set(PROTO_GENERATED_DIR ${MAIN_PROJECT_GEN_DIR})
        else()
            message(STATUS "⚠️ Generated files directory exists but no .cc/.h files found")
            message(STATUS "Please run 'make proto' from project root")
        endif()
    else()
        message(STATUS "⚠️ Main project generated files not found: ${MAIN_PROJECT_GEN_DIR}")
        message(STATUS "Please run 'make proto' from project root to generate proto files")
    endif()

    # Fallback: Check standalone build options
    if(NOT GENERATED_PROTO_SOURCES)
        if(CROUPIER_STANDALONE_BUILD)
            setup_standalone_build()
        elseif(CROUPIER_CI_BUILD)
            setup_ci_build()
        else()
            message(STATUS "🔧 No proto files found, using mock gRPC implementation")
            set(ENABLE_GRPC OFF)
        endif()
    endif()
endif()

# ========== Source Files ==========
# SDK source and header files
set(SDK_SOURCES
    src/croupier_client.cpp
    src/grpc_service.cpp
    src/config_driven_loader.cpp
    src/utils/json_utils.cpp
    src/utils/file_utils.cpp
    src/config/client_config_loader.cpp
    src/plugin/dynamic_loader.cpp
)

set(SDK_HEADERS
    include/croupier/sdk/croupier_client.h
    include/croupier/sdk/grpc_service.h
    include/croupier/sdk/config_driven_loader.h
    include/croupier/sdk/utils/json_utils.h
    include/croupier/sdk/utils/file_utils.h
    include/croupier/sdk/config/client_config_loader.h
    include/croupier/sdk/plugin/dynamic_loader.h
)

# Add generated proto files if gRPC is enabled AND proto files exist
if(ENABLE_GRPC AND GENERATED_PROTO_SOURCES)
    list(APPEND SDK_SOURCES ${GENERATED_PROTO_SOURCES})
    list(APPEND SDK_HEADERS ${GENERATED_PROTO_HEADERS})
    message(STATUS "Added generated proto files to build")
else()
    message(STATUS "Building without generated proto files (using mock implementation)")
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

    target_include_directories(croupier-sdk-shared
        PUBLIC
            $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
            $<INSTALL_INTERFACE:include>
        PRIVATE
            $<BUILD_INTERFACE:${PROTO_GENERATED_DIR}>
    )

    target_link_libraries(croupier-sdk-shared
        PUBLIC
            Threads::Threads
    )

    if(ENABLE_GRPC)
        target_link_libraries(croupier-sdk-shared
            PUBLIC
                ${GRPC_LIBRARIES}
        )
        target_compile_definitions(croupier-sdk-shared
            PUBLIC
                CROUPIER_SDK_ENABLE_GRPC
        )
    endif()

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
        EXPORT_NAME "croupier-sdk-static"
    )

    target_include_directories(croupier-sdk-static
        PUBLIC
            $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
            $<INSTALL_INTERFACE:include>
        PRIVATE
            $<BUILD_INTERFACE:${PROTO_GENERATED_DIR}>
    )

    target_link_libraries(croupier-sdk-static
        PUBLIC
            Threads::Threads
    )

    if(ENABLE_GRPC)
        target_link_libraries(croupier-sdk-static
            PUBLIC
                ${GRPC_LIBRARIES}
        )
        target_compile_definitions(croupier-sdk-static
            PUBLIC
                CROUPIER_SDK_ENABLE_GRPC
        )
    endif()

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

    target_compile_definitions(croupier-sdk-static
        PUBLIC
            CROUPIER_SDK_STATIC
    )

    # Alias for easier usage
    add_library(croupier::sdk-static ALIAS croupier-sdk-static)
endif()

# Default library target (shared if available, otherwise static)
if(BUILD_SHARED_LIBS)
    add_library(croupier::sdk ALIAS croupier-sdk-shared)
elseif(BUILD_STATIC_LIBS)
    add_library(croupier::sdk ALIAS croupier-sdk-static)
endif()

# ========== Example Programs ==========
if(BUILD_EXAMPLES)
    # Basic example
    add_executable(croupier-example examples/example.cpp)
    target_link_libraries(croupier-example
        PRIVATE
            croupier::sdk
    )

    # Virtual Object Demo
    add_executable(croupier-virtual-object-demo examples/virtual_object_demo.cpp)
    target_link_libraries(croupier-virtual-object-demo
        PRIVATE
            croupier::sdk
    )

    # Complete Example with gRPC
    add_executable(croupier-complete-example examples/complete_example.cpp)
    target_link_libraries(croupier-complete-example
        PRIVATE
            croupier::sdk
    )

    # Advanced Configuration Example
    add_executable(croupier-config-example examples/config_example.cpp)
    target_link_libraries(croupier-config-example
        PRIVATE
            croupier::sdk
    )

    # Plugin System Demo
    add_executable(croupier-plugin-demo examples/plugin_demo.cpp)
    target_link_libraries(croupier-plugin-demo
        PRIVATE
            croupier::sdk
    )

    # Comprehensive Demo - Shows ALL SDK interfaces
    add_executable(croupier-comprehensive-demo examples/comprehensive_demo.cpp)
    target_link_libraries(croupier-comprehensive-demo
        PRIVATE
            croupier::sdk
    )

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
    set_target_properties(croupier-example croupier-virtual-object-demo croupier-complete-example croupier-config-example croupier-plugin-demo croupier-comprehensive-demo PROPERTIES
        RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/bin
    )
endif()

# ========== Unit Tests ==========
if(BUILD_TESTS)
    enable_testing()

    find_package(GTest CONFIG)

    if(GTest_FOUND)
        add_executable(croupier-sdk-tests
            tests/test_virtual_objects.cpp
            tests/test_utils.cpp
            tests/test_integration.cpp
        )

        target_link_libraries(croupier-sdk-tests
            PRIVATE
                croupier::sdk
                GTest::gtest_main
        )

        add_test(NAME CroupierSDKTests COMMAND croupier-sdk-tests)
    endif()
endif()

# ========== Package Information ==========
set(CPACK_PACKAGE_NAME "croupier-cpp-sdk")
set(CPACK_PACKAGE_VERSION ${PROJECT_VERSION})
set(CPACK_PACKAGE_DESCRIPTION_SUMMARY "Croupier C++ SDK for Virtual Object Registration")
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
message(STATUS "  ENABLE_GRPC:       ${ENABLE_GRPC}")
if(ENABLE_VCPKG)
message(STATUS "  VCPKG_TRIPLET:     ${VCPKG_TARGET_TRIPLET}")
endif()
message(STATUS "")
message(STATUS "Dependencies:")
if(ENABLE_GRPC)
message(STATUS "  gRPC:            Found")
message(STATUS "  Protobuf:        Found")
endif()
if(nlohmann_json_FOUND)
message(STATUS "  nlohmann_json:   Found")
endif()
message(STATUS "=================================================")
message(STATUS "")
