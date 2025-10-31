#!/usr/bin/env bash
set -euo pipefail

# One-click script to update all git submodules to the latest remote commit of their default branch.
# - Detects each submodule's default branch (origin/HEAD if available, otherwise main/master fallback)
# - Fast-forwards clean submodules; if diverged, skips unless --force is provided
# - Stages updated submodule pointers in the superproject; optional --commit and --push
#
# Usage:
#   ./update-submodules.sh [--force] [--commit [<msg>]] [--push [<remote>]]
#
# Flags:
#   --force            Reset diverged submodules to origin/<default-branch> (data loss for local commits)
#   --commit [msg]     Create a commit in the superproject; optional custom message
#   --push [remote]    Push the superproject commit; default remote is 'origin'
#
# Notes:
# - Requires network access to fetch submodule remotes.
# - Skips submodules with uncommitted changes to avoid losing work.

force_reset=false
make_commit=false
commit_msg=""
push=false
push_remote="origin"

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --force)
      force_reset=true
      shift
      ;;
    --commit)
      make_commit=true
      shift
      if [[ ${1-} && ${1:0:2} != "--" ]]; then
        commit_msg="$1"
        shift || true
      fi
      ;;
    --push)
      push=true
      shift
      if [[ ${1-} && ${1:0:2} != "--" ]]; then
        push_remote="$1"
        shift || true
      fi
      ;;
    -h|--help)
      sed -n '1,60p' "$0"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# Ensure we're in a git repository root
repo_root=$(git rev-parse --show-toplevel 2>/dev/null || true)
if [[ -z "$repo_root" ]]; then
  echo "Error: not inside a git repository" >&2
  exit 1
fi
cd "$repo_root"

if [[ ! -f .gitmodules ]]; then
  echo "No .gitmodules found; nothing to update."
  exit 0
fi

# Initialize submodules so directories exist
echo "==> Initializing submodules (if needed)"
git submodule update --init --recursive

# Track submodule paths for staging (portable; parse .gitmodules directly)
# Example lines: "\tpath = web"
sub_paths=$(awk -F'= ' '/^[ \t]*path = /{print $2}' .gitmodules | sed -E 's/^[[:space:]]+//;s/[[:space:]]+$//' || true)
if [[ -z "$sub_paths" ]]; then
  echo "No submodules configured; done."
  exit 0
fi

updated_any=false
skipped_any=false

detect_default_branch() {
  # $1 = submodule path
  local spath="$1"
  local def_br=""

  # Ensure we have fresh refs and remote HEAD, if advertised
  git -C "$spath" fetch --tags --prune origin >/dev/null 2>&1 || true

  # Prefer origin/HEAD symbolic ref if present
  local origin_head
  origin_head=$(git -C "$spath" symbolic-ref -q --short refs/remotes/origin/HEAD || true)
  if [[ -n "$origin_head" ]]; then
    def_br="${origin_head#origin/}"
  fi

  # Fallback to main or master if origin/HEAD is missing
  if [[ -z "$def_br" ]]; then
    if git -C "$spath" show-ref --verify --quiet refs/remotes/origin/main; then
      def_br="main"
    elif git -C "$spath" show-ref --verify --quiet refs/remotes/origin/master; then
      def_br="master"
    fi
  fi

  # Last-resort: pick the first remote branch
  if [[ -z "$def_br" ]]; then
    def_br=$(git -C "$spath" for-each-ref --format='%(refname:short)' 'refs/remotes/origin/*' | grep -v '^origin/HEAD$' | head -n1 || true)
    def_br="${def_br#origin/}"
  fi

  echo "$def_br"
}

can_ff_to_origin() {
  # $1 = submodule path, $2 = branch
  local spath="$1"; local br="$2"
  if ! git -C "$spath" rev-parse --verify -q "origin/$br" >/dev/null; then
    return 1
  fi
  # If no HEAD (detached or unborn), allow reset/checkout path
  if ! git -C "$spath" rev-parse --verify -q HEAD >/dev/null; then
    return 0
  fi
  # Check if HEAD is ancestor of origin/br
  git -C "$spath" merge-base --is-ancestor HEAD "origin/$br"
}

while IFS= read -r spath; do
  if [[ ! -d "$spath/.git" && ! -f "$spath/.git" ]]; then
    echo "-- Skipping $spath (not initialized)"
    skipped_any=true
    continue
  fi

  # Skip if dirty
  if [[ -n $(git -C "$spath" status --porcelain) ]]; then
    echo "-- Skipping $spath (has uncommitted changes)"
    skipped_any=true
    continue
  fi

  def_br=$(detect_default_branch "$spath")
  if [[ -z "$def_br" ]]; then
    echo "-- Skipping $spath (cannot determine default branch)"
    skipped_any=true
    continue
  fi

  echo "==> Updating $spath -> origin/$def_br"
  # Ensure a local branch exists tracking origin/def_br
  if git -C "$spath" show-ref --verify --quiet "refs/heads/$def_br"; then
    # Switch if needed
    current_br=$(git -C "$spath" symbolic-ref -q --short HEAD || true)
    if [[ "$current_br" != "$def_br" ]]; then
      git -C "$spath" checkout "$def_br" >/dev/null 2>&1 || true
    fi
  else
    git -C "$spath" checkout -B "$def_br" "origin/$def_br" >/dev/null 2>&1 || true
  fi

  # Try fast-forward; if diverged, reset only with --force
  if can_ff_to_origin "$spath" "$def_br"; then
    git -C "$spath" merge --ff-only "origin/$def_br" >/dev/null 2>&1 || \
      git -C "$spath" reset --hard "origin/$def_br" >/dev/null 2>&1
  else
    if [[ "$force_reset" == true ]]; then
      echo "   Diverged; forcing reset to origin/$def_br"
      git -C "$spath" reset --hard "origin/$def_br" >/dev/null 2>&1
    else
      echo "   Diverged; skip (use --force to reset)"
      skipped_any=true
      continue
    fi
  fi

  # Stage updated gitlink in superproject
  git add "$spath"
  updated_any=true

done <<< "$sub_paths"

if [[ "$updated_any" == true ]]; then
  if [[ "$make_commit" == true ]]; then
    if [[ -z "$commit_msg" ]]; then
      commit_msg="chore: update submodules to latest"
    fi
    echo "==> Committing superproject changes"
    git commit -m "$commit_msg" || true
  else
    echo "==> Submodule pointers updated; remember to commit in the superproject"
  fi

  if [[ "$push" == true ]]; then
    echo "==> Pushing superproject to $push_remote"
    git push "$push_remote" HEAD
  fi
else
  echo "No submodule updates applied."
fi

if [[ "$skipped_any" == true ]]; then
  echo "Note: Some submodules were skipped (dirty, diverged without --force, or no default branch)."
fi

