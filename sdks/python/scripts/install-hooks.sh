#!/bin/bash
# Install git hooks for Python SDK
# Run this script from the repository root: ./scripts/install-hooks.sh

set -e

HOOKS_DIR=".git/hooks"

echo "Installing git hooks..."

# Create pre-commit hook
cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash
#
# Pre-commit hook for Python SDK
# Runs formatting, linting, and tests before committing
#

set -e

echo "🔍 Running pre-commit checks..."

# Get the directory where the hook is located
REPO_DIR=$(git rev-parse --show-toplevel)
cd "$REPO_DIR"

# 1. Run black formatting check
echo "📦 Checking code formatting (black)..."
uv run black --check croupier tests

# 2. Run flake8 linting
echo "🔍 Running linting (flake8)..."
uv run flake8 croupier tests --max-line-length=100 --ignore=E203,W503

# 3. Run tests
echo "🧪 Running tests (pytest)..."
uv run pytest -q --tb=short

echo "✅ All pre-commit checks passed!"
EOF

# Create pre-push hook
cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash
#
# Pre-push hook for Python SDK
# Runs full test suite with coverage before pushing
#

set -e

echo "🔍 Running pre-push checks..."

REPO_DIR=$(git rev-parse --show-toplevel)
cd "$REPO_DIR"

# Run tests with coverage (minimum 70%)
echo "🧪 Running tests with coverage..."
uv run pytest -q --cov=croupier --cov-report=term --cov-fail-under=70

echo "✅ All pre-push checks passed!"
EOF

# Make hooks executable
chmod +x "$HOOKS_DIR/pre-commit"
chmod +x "$HOOKS_DIR/pre-push"

echo "✅ Git hooks installed successfully!"
echo ""
echo "Installed hooks:"
echo "  - pre-commit: Runs black, flake8, and pytest"
echo "  - pre-push: Runs pytest with coverage (>=70%)"
echo ""
echo "To skip hooks temporarily, use:"
echo "  git commit --no-verify"
echo "  git push --no-verify"
