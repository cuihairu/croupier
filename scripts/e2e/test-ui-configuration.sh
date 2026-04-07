#!/bin/bash
# scripts/e2e/test-ui-configuration.sh
# Test UI Configuration Flow (Read, Update, History, Rollback)

set -euo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:18780}"
FUNCTION_ID="${FUNCTION_ID:-player.ban}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"

TOKEN=$(curl -s -X POST "$SERVER_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r '.token // empty')

if [ -z "$TOKEN" ]; then
    echo "Error: Failed to obtain auth token"
    exit 1
fi

echo "=========================================="
echo "  UI Configuration Flow Test"
echo "=========================================="
echo "Function ID: $FUNCTION_ID"
echo "Server:     $SERVER_URL"
echo ""

# Check if function exists
echo ">>> Checking if function exists..."
code=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" \
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
current_ui=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui" 2>/dev/null || echo '{}')
echo "$current_ui" | jq '.' 2>/dev/null || echo "$current_ui"

current_cols=$(echo "$current_ui" | jq -r 'if (.layout | type) == "object" then .layout.cols // empty else empty end')
echo "Current layout cols: ${current_cols:-unknown}"

# Step 2: Update UI configuration
echo ""
echo ">>> Step 2: Update UI configuration"

# Prepare test UI config
new_ui_config='{
  "schema": {
    "type": "object",
    "properties": {
      "playerId": { "type": "string", "title": "Player ID" },
      "reason": { "type": "string", "title": "Reason" },
      "operator": { "type": "string", "title": "Operator" }
    },
    "required": ["playerId", "reason"]
  },
  "layout": {
    "type": "grid",
    "cols": 3
  },
  "components": {
    "reason": {
      "widget": "textarea"
    }
  }
}'

update_result=$(curl -s -X PUT "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$new_ui_config" 2>/dev/null || echo '{}')

echo "$update_result" | jq '.' 2>/dev/null || echo "$update_result"

echo "Update request completed"

# Step 3: Verify configuration updated
echo ""
echo ">>> Step 3: Verify configuration updated"
updated_ui=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui" 2>/dev/null || echo '{}')

# Check if layout was updated
layout_cols=$(echo "$updated_ui" | jq -r 'if (.layout | type) == "object" then .layout.cols // empty else empty end')
if [ "$layout_cols" = "3" ]; then
    echo -e "\e[32m✓\e[0m Layout successfully updated to cols=3"
else
    echo -e "\e[31m✗\e[0m Layout update verification failed"
    exit 1
fi

# Step 4: View UI history
echo ""
echo ">>> Step 4: View UI configuration history"
history=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui/history" 2>/dev/null || echo '{}')
echo "$history" | jq '.' 2>/dev/null || echo "$history"

history_count=$(echo "$history" | jq '.items | length' 2>/dev/null || echo '0')
echo "History entries: $history_count"

# Step 5: Rollback (if previous version exists)
rollback_version=$(echo "$history" | jq -r '.items[1].version // empty' 2>/dev/null || echo '')
if [ -n "$rollback_version" ] && [ "$rollback_version" != "null" ]; then
    echo ""
    echo ">>> Step 5: Rollback to version $rollback_version"

    rollback_result=$(curl -s -X POST \
        "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui/rollback" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"version\": $rollback_version}" 2>/dev/null || echo '{}')

    echo "$rollback_result" | jq '.' 2>/dev/null || echo "$rollback_result"

    applied_version=$(echo "$rollback_result" | jq -r '.appliedVersion // empty')
    if [ "$applied_version" = "$rollback_version" ]; then
        echo -e "\e[32m✓\e[0m Rolled back to version $rollback_version"
    else
        echo -e "\e[31m✗\e[0m Rollback verification failed"
        exit 1
    fi
else
    echo ""
    echo ">>> Step 5: Skip rollback (no previous version or initial version)"
fi

echo ""
echo "=========================================="
echo "  UI Configuration Flow Test Complete"
echo "=========================================="
