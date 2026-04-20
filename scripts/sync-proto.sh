#!/bin/bash
# Sync proto files from remote

set -e

echo "Syncing proto/ with latest..."

cd "$(dirname "$0")/.."
cd proto

# Fetch latest
git fetch origin

# Get current branch or use main
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")

# Check if we're in detached HEAD state
if git rev-parse --abbrev-ref HEAD > /dev/null 2>&1; then
    # On a branch, pull
    git pull origin "$CURRENT_BRANCH"
else
    # Detached HEAD, checkout main and pull
    git checkout main
    git pull origin main
fi

echo "✅ Proto files synced to latest version"
