# Proto Generation for CI Build
#
# This module handles proto file downloading and code generation for CI builds

# Function to download proto files from main repository
function(download_proto_files PROTO_SOURCE_DIR PROTO_DEST_DIR)
    message(STATUS "Downloading proto files for CI build...")

    # Create destination directory
    file(MAKE_DIRECTORY ${PROTO_DEST_DIR})

    if(NOT CROUPIER_PROTO_BRANCH)
        set(CROUPIER_PROTO_BRANCH "main")
    endif()

    # Download proto files from main repository
    set(PROTO_BASE_URL "https://raw.githubusercontent.com/cuihairu/croupier/${CROUPIER_PROTO_BRANCH}/proto")

    # List of proto files to download
    set(PROTO_FILES
        "croupier/agent/local/v1/local.proto"
        "croupier/control/v1/control.proto"
        "croupier/function/v1/function.proto"
        "croupier/edge/job/v1/job.proto"
        "croupier/tunnel/v1/tunnel.proto"
        "croupier/options/ui.proto"
        "croupier/options/function.proto"
    )

    foreach(PROTO_FILE ${PROTO_FILES})
        set(PROTO_URL "${PROTO_BASE_URL}/${PROTO_FILE}")
        set(PROTO_LOCAL_PATH "${PROTO_DEST_DIR}/${PROTO_FILE}")

        # Create subdirectories
        get_filename_component(PROTO_DIR ${PROTO_LOCAL_PATH} DIRECTORY)
        file(MAKE_DIRECTORY ${PROTO_DIR})

        message(STATUS "Downloading: ${PROTO_URL}")

        # Download file
        file(DOWNLOAD
            ${PROTO_URL}
            ${PROTO_LOCAL_PATH}
            STATUS DOWNLOAD_STATUS
            LOG DOWNLOAD_LOG
        )

        # Check download status
        list(GET DOWNLOAD_STATUS 0 STATUS_CODE)
        if(NOT STATUS_CODE EQUAL 0)
            message(FATAL_ERROR "Failed to download ${PROTO_FILE}: ${DOWNLOAD_LOG}")
        endif()

        message(STATUS "Downloaded: ${PROTO_FILE}")
    endforeach()

    message(STATUS "Proto files downloaded successfully to ${PROTO_DEST_DIR}")
endfunction()

# Function to generate gRPC code from proto files
function(generate_grpc_code PROTO_SOURCE_DIR GENERATED_DIR)
    message(STATUS "Generating gRPC code from proto files...")

    # Find protobuf and gRPC
    find_package(Protobuf REQUIRED)
    find_package(gRPC REQUIRED)

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
        set(GRPC_SRCS "${GENERATED_DIR}/${PROTO_DIR}/${PROTO_NAME}.grpc.pb.cc")
        set(GRPC_HDRS "${GENERATED_DIR}/${PROTO_DIR}/${PROTO_NAME}.grpc.pb.h")

        # Create output directory
        get_filename_component(OUTPUT_DIR ${PROTO_SRCS} DIRECTORY)
        file(MAKE_DIRECTORY ${OUTPUT_DIR})

        # Add custom command to generate code
        add_custom_command(
            OUTPUT ${PROTO_SRCS} ${PROTO_HDRS} ${GRPC_SRCS} ${GRPC_HDRS}
            COMMAND protobuf::protoc
                --proto_path=${PROTO_SOURCE_DIR}
                --cpp_out=${GENERATED_DIR}
                --grpc_out=${GENERATED_DIR}
                --plugin=protoc-gen-grpc=$<TARGET_FILE:gRPC::grpc_cpp_plugin>
                ${PROTO_FILE}
            DEPENDS ${PROTO_FILE}
            COMMENT "Generating gRPC code for ${PROTO_REL_PATH}"
            VERBATIM
        )

        list(APPEND GENERATED_SOURCES ${PROTO_SRCS} ${GRPC_SRCS})
        list(APPEND GENERATED_HEADERS ${PROTO_HDRS} ${GRPC_HDRS})
    endforeach()

    # Set parent scope variables
    set(GENERATED_PROTO_SOURCES ${GENERATED_SOURCES} PARENT_SCOPE)
    set(GENERATED_PROTO_HEADERS ${GENERATED_HEADERS} PARENT_SCOPE)

    message(STATUS "gRPC code generation configured for ${GENERATED_DIR}")
endfunction()

# Function to setup standalone build
function(setup_standalone_build)
    message(STATUS "Standalone build mode detected")

    if(CROUPIER_PREBUILT_PROTO)
        # Mode 1: Use prebuilt proto files
        message(STATUS "🎯 Using prebuilt proto files")

        set(PROTO_GENERATED_DIR "${CMAKE_CURRENT_SOURCE_DIR}/generated")

        if(EXISTS ${PROTO_GENERATED_DIR})
            # Collect existing generated files
            file(GLOB_RECURSE GENERATED_PROTO_SOURCES "${PROTO_GENERATED_DIR}/**/*.cc")
            file(GLOB_RECURSE GENERATED_PROTO_HEADERS "${PROTO_GENERATED_DIR}/**/*.h")

            if(GENERATED_PROTO_SOURCES)
                set(CROUPIER_SDK_ENABLE_GRPC ON PARENT_SCOPE)
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
        # Mode 2: Online download and generation
        message(STATUS "🌐 Online build mode - downloading proto files")

        if(EXISTS ${CROUPIER_PROTO_DIR})
            set(PROTO_GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")

            # Generate gRPC code from downloaded files
            generate_grpc_code(${CROUPIER_PROTO_DIR} ${PROTO_GENERATED_DIR})

            # Enable gRPC and set paths
            set(CROUPIER_SDK_ENABLE_GRPC ON PARENT_SCOPE)
            set(PROTO_GENERATED_DIR ${PROTO_GENERATED_DIR} PARENT_SCOPE)
            set(GENERATED_PROTO_SOURCES ${GENERATED_PROTO_SOURCES} PARENT_SCOPE)
            set(GENERATED_PROTO_HEADERS ${GENERATED_PROTO_HEADERS} PARENT_SCOPE)

            message(STATUS "✅ Online proto generation configured")
            return()
        else()
            message(STATUS "⚠️ Proto directory not found: ${CROUPIER_PROTO_DIR}")
        endif()
    endif()

    # Mode 3: Fallback to mock implementation
    message(STATUS "🔧 Using mock gRPC implementation for standalone build")
    set(CROUPIER_SDK_ENABLE_GRPC OFF PARENT_SCOPE)
endfunction()

# Function to setup CI build with proto generation
function(setup_ci_build)
    # Check if we're in CI environment or CI build is explicitly enabled
    if(DEFINED ENV{CI} OR CROUPIER_CI_BUILD)
        message(STATUS "CI build detected, setting up proto generation...")

        # First check if we have pre-copied proto files (from CI)
        set(PROTO_SOURCE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/generated/croupier")
        set(PROTO_GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")

        if(EXISTS ${PROTO_SOURCE_DIR})
            message(STATUS "✅ Using pre-copied proto files from CI")
            # Create generated directory
            file(MAKE_DIRECTORY ${PROTO_GENERATED_DIR})

            # Generate gRPC code from pre-copied files
            generate_grpc_code("${CMAKE_CURRENT_SOURCE_DIR}/generated" ${PROTO_GENERATED_DIR})
        else()
            # Fallback: download from main project
            message(STATUS "⬇️  Proto files not pre-copied, downloading from main project...")
            set(PROTO_DOWNLOAD_DIR "${CMAKE_CURRENT_BINARY_DIR}/downloaded_proto")

            # Download proto files
            download_proto_files("${CMAKE_CURRENT_SOURCE_DIR}/../../proto" ${PROTO_DOWNLOAD_DIR})

            # Generate gRPC code
            generate_grpc_code(${PROTO_DOWNLOAD_DIR} ${PROTO_GENERATED_DIR})
        endif()

        # Enable gRPC and set paths
        set(CROUPIER_SDK_ENABLE_GRPC ON PARENT_SCOPE)
        set(PROTO_GENERATED_DIR ${PROTO_GENERATED_DIR} PARENT_SCOPE)
        set(GENERATED_PROTO_SOURCES ${GENERATED_PROTO_SOURCES} PARENT_SCOPE)
        set(GENERATED_PROTO_HEADERS ${GENERATED_PROTO_HEADERS} PARENT_SCOPE)

        message(STATUS "✅ CI build setup completed with proto generation")
    else()
        # Check if we have local generated files
        set(LOCAL_GENERATED_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../gen/croupier")
        if(EXISTS ${LOCAL_GENERATED_DIR})
            message(STATUS "🏠 Local build detected, using existing generated files from main project")
            # Use the main project's generated files
            set(PROTO_GENERATED_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../gen")

            # Collect existing generated files
            file(GLOB_RECURSE GENERATED_PROTO_SOURCES "${PROTO_GENERATED_DIR}/**/*.cc")
            file(GLOB_RECURSE GENERATED_PROTO_HEADERS "${PROTO_GENERATED_DIR}/**/*.h")

            if(GENERATED_PROTO_SOURCES)
                set(CROUPIER_SDK_ENABLE_GRPC ON PARENT_SCOPE)
                set(PROTO_GENERATED_DIR ${PROTO_GENERATED_DIR} PARENT_SCOPE)
                set(GENERATED_PROTO_SOURCES ${GENERATED_PROTO_SOURCES} PARENT_SCOPE)
                set(GENERATED_PROTO_HEADERS ${GENERATED_PROTO_HEADERS} PARENT_SCOPE)
                message(STATUS "✅ Found ${list(LENGTH GENERATED_PROTO_SOURCES)} generated files")
            else()
                message(STATUS "⚠️  Generated files directory exists but no .cc/.h files found")
                set(CROUPIER_SDK_ENABLE_GRPC OFF PARENT_SCOPE)
            endif()
        else()
            message(STATUS "🔧 Local build detected, using mock gRPC implementation (no generated files)")
            set(CROUPIER_SDK_ENABLE_GRPC OFF PARENT_SCOPE)
        endif()
    endif()
endfunction()
