# Proto Generation for Monorepo Build
#
# This module handles proto file code generation using local proto files

get_filename_component(CPP_SDK_DIR "${CMAKE_CURRENT_LIST_DIR}/.." ABSOLUTE)
get_filename_component(PROJECT_ROOT_DIR "${CMAKE_CURRENT_LIST_DIR}/../../.." ABSOLUTE)

# Function to generate protobuf code from proto files (no gRPC)
function(generate_proto_code PROTO_SOURCE_DIR GENERATED_DIR)
    message(STATUS "Generating protobuf code from ${PROTO_SOURCE_DIR}...")

    # Find protobuf
    find_package(Protobuf CONFIG QUIET)
    if(NOT Protobuf_FOUND)
        find_package(Protobuf MODULE REQUIRED)
    endif()

    # Create generated directory
    file(MAKE_DIRECTORY ${GENERATED_DIR})

    # Get all proto files
    file(GLOB_RECURSE PROTO_FILES "${PROTO_SOURCE_DIR}/*.proto")

    set(GENERATED_SOURCES "")
    set(GENERATED_HEADERS "")

    foreach(PROTO_FILE ${PROTO_FILES})
        # Get relative path
        file(RELATIVE_PATH PROTO_REL_PATH ${PROTO_SOURCE_DIR} ${PROTO_FILE})
        get_filename_component(PROTO_NAME ${PROTO_REL_PATH} NAME_WE)
        get_filename_component(PROTO_DIR ${PROTO_REL_PATH} DIRECTORY)

        # Generated file paths
        set(PROTO_SRCS "${GENERATED_DIR}/${PROTO_DIR}/${PROTO_NAME}.pb.cc")
        set(PROTO_HDRS "${GENERATED_DIR}/${PROTO_DIR}/${PROTO_NAME}.pb.h")

        # Create output directory
        get_filename_component(OUTPUT_DIR ${PROTO_SRCS} DIRECTORY)
        file(MAKE_DIRECTORY ${OUTPUT_DIR})

        # Add custom command to generate code (protobuf only, no gRPC)
        add_custom_command(
            OUTPUT ${PROTO_SRCS} ${PROTO_HDRS}
            COMMAND protobuf::protoc
                --proto_path=${PROTO_SOURCE_DIR}
                --cpp_out=${GENERATED_DIR}
                ${PROTO_FILE}
            DEPENDS ${PROTO_FILE}
            COMMENT "Generating protobuf code for ${PROTO_REL_PATH}"
            VERBATIM
        )

        list(APPEND GENERATED_SOURCES ${PROTO_SRCS})
        list(APPEND GENERATED_HEADERS ${PROTO_HDRS})
    endforeach()

    # Set parent scope variables
    set(GENERATED_PROTO_SOURCES ${GENERATED_SOURCES} PARENT_SCOPE)
    set(GENERATED_PROTO_HEADERS ${GENERATED_HEADERS} PARENT_SCOPE)

    message(STATUS "Protobuf code generation configured for ${GENERATED_DIR}")
endfunction()

# Function to setup standalone build
function(setup_standalone_build)
    message(STATUS "Standalone build mode detected")

    if(CROUPIER_PREBUILT_PROTO)
        # Mode 1: Use prebuilt proto files
        message(STATUS "🎯 Using prebuilt proto files")

        set(PROTO_GENERATED_DIR "${CPP_SDK_DIR}/generated")

        if(EXISTS ${PROTO_GENERATED_DIR})
            # Collect existing generated files
            file(GLOB_RECURSE GENERATED_PROTO_SOURCES "${PROTO_GENERATED_DIR}/**/*.cc")
            file(GLOB_RECURSE GENERATED_PROTO_HEADERS "${PROTO_GENERATED_DIR}/**/*.h")

            if(GENERATED_PROTO_SOURCES)
                set(PROTO_GENERATED_DIR ${PROTO_GENERATED_DIR} PARENT_SCOPE)
                set(GENERATED_PROTO_SOURCES ${GENERATED_PROTO_SOURCES} PARENT_SCOPE)
                set(GENERATED_PROTO_HEADERS ${GENERATED_PROTO_HEADERS} PARENT_SCOPE)

                list(LENGTH GENERATED_PROTO_SOURCES file_count)
                message(STATUS "✅ Found ${file_count} generated proto source files")
                return()
            else()
                message(STATUS "⚠️ Prebuilt directory exists but no .cc files found")
            endif()
        else()
            message(STATUS "⚠️ Prebuilt proto directory not found")
        endif()

    elseif(CROUPIER_ONLINE_BUILD)
        # Mode 2: Online mode - use local proto directory
        message(STATUS "🌐 Online build mode - using local proto directory")

        if(EXISTS "${PROJECT_ROOT_DIR}/proto/croupier")
            set(PROTO_GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")

            # Generate protobuf code from local proto files
            generate_proto_code("${PROJECT_ROOT_DIR}/proto" ${PROTO_GENERATED_DIR})

            # Set paths
            set(PROTO_GENERATED_DIR ${PROTO_GENERATED_DIR} PARENT_SCOPE)
            set(GENERATED_PROTO_SOURCES ${GENERATED_PROTO_SOURCES} PARENT_SCOPE)
            set(GENERATED_PROTO_HEADERS ${GENERATED_PROTO_HEADERS} PARENT_SCOPE)

            message(STATUS "✅ Online proto generation configured")
            return()
        else()
            message(STATUS "⚠️ Proto directory not found: ${PROJECT_ROOT_DIR}/proto")
        endif()
    endif()

    # Mode 3: Fallback
    message(STATUS "🔧 No proto files available for standalone build")
endfunction()

# Function to setup CI build with proto generation
function(setup_ci_build)
    # TCP transport uses protobuf for message serialization
    message(STATUS "TCP transport mode - generating protobuf for message serialization")

    # Always generate in CI to guarantee protoc/runtime version alignment
    set(PROTO_GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")

    # Use local proto directory from monorepo
    if(EXISTS "${PROJECT_ROOT_DIR}/proto/croupier")
        message(STATUS "🏠 Using monorepo proto directory at ${PROJECT_ROOT_DIR}/proto")
        generate_proto_code("${PROJECT_ROOT_DIR}/proto" ${PROTO_GENERATED_DIR})
    else()
        message(FATAL_ERROR "Proto directory not found at ${PROJECT_ROOT_DIR}/proto")
    endif()

    # TCP transport uses protobuf for message serialization (no gRPC)
    set(CROUPIER_SDK_ENABLE_GRPC OFF PARENT_SCOPE)
    set(PROTO_GENERATED_DIR ${PROTO_GENERATED_DIR} PARENT_SCOPE)
    set(GENERATED_PROTO_SOURCES ${GENERATED_PROTO_SOURCES} PARENT_SCOPE)
    set(GENERATED_PROTO_HEADERS ${GENERATED_PROTO_HEADERS} PARENT_SCOPE)

    message(STATUS "✅ CI build setup completed with proto generation")
endfunction()
