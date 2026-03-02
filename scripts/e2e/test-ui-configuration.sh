#!/bin/bash
# scripts/e2e/test-ui-configuration.sh
# Test UI Configuration Flow (Read, Update, History, Rollback)

set -euo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:18780}"
FUNCTION_ID="${FUNCTION_ID:-test.player.addCurrency}"

echo "=========================================="
echo "  UI Configuration Flow Test"
echo "=========================================="
echo "Function ID: $FUNCTION_ID"
echo "Server:     $SERVER_URL"
echo ""

# Check if function exists
echo ">>> Checking if function exists..."
code=$(curl -s -o /dev/null -w "%{http_code}" \
    "$SERVER_URL/api/v1/functions/$FUNCTION_ID" 2>/dev/null || echo "000")

if [ "$code" != "200" ] && [ "$code" != "404" ]; then
    echo "Error: Cannot connect to server (HTTP $code)"
    exit 1
fi

if [ "$code" = "404" ]; then
    echo "Warning: Function '$FUNCTION_ID' not found"
    echo "Using test function instead..."
    FUNCTION_ID="test.e2e.function.$(date +%s)"
fi

# Step 1: Read current UI configuration
echo ""
echo ">>> Step 1: Read current UI configuration"
current_ui=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui" 2>/dev/null || echo '{}')
echo "$current_ui" | jq '.' 2>/dev/null || echo "$current_ui"

current_version=$(echo "$current_ui" | jq -r '.version // 1')
echo "Current version: $current_version"

# Step 2: Update UI configuration
echo ""
echo ">>> Step 2: Update UI configuration"

# Prepare test UI config
new_ui_config='{
  "layout": "horizontal",
  "labelWidth": 160,
  "size": "large",
  "colon": true
}'

update_result=$(curl -s -X PUT "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui" \
    -H "Content-Type: application/json" \
    -d "$new_ui_config" 2>/dev/null || echo '{}')

echo "$update_result" | jq '.' 2>/dev/null || echo "$update_result"

new_version=$(echo "$update_result" | jq -r '.version // empty')
if [ -n "$new_version" ] && [ "$new_version" != "null" ] && [ "$new_version" != "empty" ]; then
    echo "New version: $new_version"
else
    echo "Warning: Could not determine new version"
fi

# Step 3: Verify configuration updated
echo ""
echo ">>> Step 3: Verify configuration updated"
updated_ui=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui" 2>/dev/null || echo '{}')

# Check if layout was updated
layout=$(echo "$updated_ui" | jq -r '.config.layout // empty')
if [ "$layout" = "horizontal" ]; then
    echo -e "\e[32m✓\e[0m Layout successfully updated to 'horizontal'"
else
    echo -e "\e[31m✗\e[0m Layout update verification failed"
fi

# Step 4: View UI history
echo ""
echo ">>> Step 4: View UI configuration history"
history=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui/history" 2>/dev/null || '[]')
echo "$history" | jq '.' 2>/dev/null || echo "$history"

history_count=$(echo "$history" | jq '.history | length' 2>/dev/null || echo '0')
echo "History entries: $history_count"

# Step 5: Rollback (if previous version exists)
if [ -n "$current_version" ] && [ "$current_version" != "null" ] && [ "$current_version" != "1" ]; then
    echo ""
    echo ">>> Step 5: Rollback to version $current_version"

    rollback_result=$(curl -s -X POST \
        "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui/rollback" \
        -H "Content-Type: application/json" \
        -d "{\"version\": $current_version}" 2>/dev/null || echo '{}')

    echo "$rollback_result" | jq '.' 2>/dev/null || echo "$rollback_result"

    rollback_version=$(echo "$rollback_result" | jq -r '.version // empty')
    if [ -n "$rollback_version" ] && [ "$rollback_version" != "null" ] && [ "$rollback_version" != "empty" ]; then
        echo -e "\e[32m✓\e[0m Rolled back to version $current_version, new version: $rollback_version"
    fi
else
    echo ""
    echo ">>> Step 5: Skip rollback (no previous version or initial version)"
fi

echo ""
echo "=========================================="
echo "  UI Configuration Flow Test Complete"
echo "=========================================="
