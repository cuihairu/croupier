#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

git -C "${repo_root}" config core.hooksPath scripts/git-hooks

echo "Configured git hooks path to ${repo_root}/scripts/git-hooks"
