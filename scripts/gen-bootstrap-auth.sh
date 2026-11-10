#!/usr/bin/env bash
# Generate bootstrap admin credentials with a fresh random password (CR-FIX-27).
#
# Writes admins.json / users.json into the config dir with a random password
# per admin, chmod 0600, and prints the generated passwords exactly once.
# Run this instead of shipping the factory default admin123 when preparing a
# new instance's bootstrap config.
#
# Usage:
#   scripts/gen-bootstrap-auth.sh [output-dir] [--username admin] [--roles admin]
#
# Default output: configs/bootstrap-generated
set -euo pipefail

OUT="configs/bootstrap-generated"
USERNAME="admin"
ROLES="admin"

while [ $# -gt 0 ]; do
  case "$1" in
    --username) USERNAME="$2"; shift 2 ;;
    --roles) ROLES="$2"; shift 2 ;;
    -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
    *) OUT="${1}"; shift ;;
  esac
done

mkdir -p "$OUT"
chmod 700 "$OUT"

gen_password() {
  # 32-char base64url (192-bit), same shape as the server-side rotation
  head -c 24 /dev/urandom | base64 | tr '+/' '-_' | tr -d '='
}

now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# roles csv -> JSON array items: ["a","b"]
roles_json="[$(printf '"%s",' $(echo "$ROLES" | tr ',' ' ') | sed 's/,$//')]"

admin_pw="$(gen_password)"
cat > "$OUT/admins.json" <<EOF
[
  {
    "username": "$USERNAME",
    "password": "$admin_pw",
    "roles": $roles_json
  }
]
EOF

user_pw="$(gen_password)"
cat > "$OUT/users.json" <<EOF
[
  {
    "username": "$USERNAME",
    "password": "$user_pw",
    "nickname": "$USERNAME",
    "roles": $roles_json
  }
]
EOF

chmod 600 "$OUT/admins.json" "$OUT/users.json"

echo "Bootstrap credentials generated (shown once - store them now):"
echo "  output: $OUT/admins.json, $OUT/users.json"
echo "  $USERNAME / $admin_pw (admins.json)"
echo "  $USERNAME / $user_pw  (users.json)"
echo "generated at $now" > "$OUT/.generated-at"
chmod 600 "$OUT/.generated-at"
