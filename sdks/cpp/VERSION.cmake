# Croupier C++ SDK Version Configuration
# This file defines the canonical version for the entire project
#
# Version Format: MAJOR.MINOR.PATCH
# - MAJOR: Incompatible API changes
# - MINOR: Backwards-compatible functionality additions
# - PATCH: Backwards-compatible bug fixes

set(CROUPIER_SDK_VERSION_MAJOR 0)
set(CROUPIER_SDK_VERSION_MINOR 1)
set(CROUPIER_SDK_VERSION_PATCH 2)

# Full version string
set(CROUPIER_SDK_VERSION "${CROUPIER_SDK_VERSION_MAJOR}.${CROUPIER_SDK_VERSION_MINOR}.${CROUPIER_SDK_VERSION_PATCH}")

# Version for display (may include pre-release markers)
set(CROUPIER_SDK_VERSION_FULL "${CROUPIER_SDK_VERSION}")

# ABI version (for soname/shared library versioning)
set(CROUPIER_SDK_ABI_VERSION "${CROUPIER_SDK_VERSION_MAJOR}.${CROUPIER_SDK_VERSION_MINOR}")

# Project metadata
set(CROUPIER_SDK_NAME "Croupier C++ SDK")
set(CROUPIER_SDK_DESCRIPTION "C++ SDK for Croupier Virtual Object Registration and gRPC Integration")
set(CROUPIER_SDK_HOMEPAGE_URL "https://github.com/cuihairu/croupier-sdk-cpp")
set(CROUPIER_SDK_LICENSE "Apache-2.0")

# Display version info
message(STATUS "${CROUPIER_SDK_NAME} version: ${CROUPIER_SDK_VERSION_FULL}")
