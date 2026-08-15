#!/usr/bin/env bash

set -euo pipefail

fixture_dir=$(mktemp -d)
cleanup() {
  rm -rf -- "${fixture_dir}"
}
trap cleanup EXIT

fixture_file="${fixture_dir}/guard-fixture.spec.ts"

write_fixture() {
  printf '%s\n' "$1" >"${fixture_file}"
}

expect_rejection() {
  local label="$1"
  local source="$2"

  write_fixture "${source}"
  if bash "scripts/dashboard_e2e_guard.sh" "${fixture_dir}" >/dev/null 2>&1; then
    echo "ERROR: guard accepted ${label}" >&2
    exit 1
  fi
}

write_fixture "import { test, expect } from '@playwright/test'; test('ok', async ({ page }) => { await expect(page.getByText('业务记录')).toBeVisible(); });"
bash "scripts/dashboard_e2e_guard.sh" "${fixture_dir}" >/dev/null

expect_rejection "test.skip" "import { test } from '@playwright/test'; test.skip('skip', async () => {});"
expect_rejection "isVisible fallback" "const visible = await button.isVisible().catch(() => false);"
expect_rejection "boolean OR assertion" "expect(hasForm || hasCard).toBeTruthy();"
expect_rejection "zero-row assertion" "expect(rows).toBeGreaterThanOrEqual(0);"
expect_rejection "button-missing return" "if (!button) return;"
expect_rejection "main-container assertion" "await expect(page.locator('main, .container')).toBeVisible();"

echo "Dashboard E2E guard rejection tests passed."
