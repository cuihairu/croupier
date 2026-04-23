#!/bin/bash

# Croupier C++ SDK macOS Build Script (with Homebrew conflict workaround)
#
# This script temporarily disables Homebrew's abseil headers during vcpkg build
# to avoid conflicts with vcpkg's version.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🍎 macOS Build Script for Croupier C++ SDK"
echo "=========================================="
echo ""

# Check if Homebrew abseil exists
HOMEBREW_ABSL="/usr/local/include/absl"
if [ -L "$HOMEBREW_ABSL" ] || [ -d "$HOMEBREW_ABSL" ]; then
    echo "⚠️  Detected Homebrew abseil at $HOMEBREW_ABSL"
    echo "📦 Temporarily disabling to avoid conflicts..."

    # Save the original
    if [ -L "$HOMEBREW_ABSL" ]; then
        BACKUP_LINK=$(readlink "$HOMEBREW_ABSL")
        echo "$BACKUP_LINK" > /tmp/absl_homebrew_backup.txt
    fi

    # Temporarily move it
    sudo mv "$HOMEBREW_ABSL" "$HOMEBREW_ABSL.disabled" 2>/dev/null || {
        echo "❌ Failed to disable Homebrew abseil. Please run:"
        echo "   sudo mv /usr/local/include/absl /usr/local/include/absl.disabled"
        echo "   ./scripts/build.sh"
        echo "   sudo mv /usr/local/include/absl.disabled /usr/local/include/absl"
        exit 1
    }

    echo "✅ Homebrew abseil disabled"
    echo ""
fi

# Trap to restore on exit
trap '{
    if [ -f /tmp/absl_homebrew_backup.txt ]; then
        echo ""
        echo "📦 Restoring Homebrew abseil..."
        sudo mv "/usr/local/include/absl.disabled" "/usr/local/include/absl" 2>/dev/null || \
            echo "⚠️  Failed to restore (you may need to manually restore with: sudo mv /usr/local/include/absl.disabled /usr/local/include/absl)"
        rm -f /tmp/absl_homebrew_backup.txt
        echo "✅ Restored"
    fi
}' EXIT

# Build the project
cd "$SDK_ROOT"
echo "🔨 Starting build..."
echo ""

./scripts/build.sh --clean "$@"

echo ""
echo "✅ Build completed successfully!"
