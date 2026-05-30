@echo off
REM Install git hooks for Python SDK (Windows)
REM Run this script from the repository root: scripts\install-hooks.bat

setlocal enabledelayedexpansion

echo Installing git hooks...

set HOOKS_DIR=.git\hooks

REM Create pre-commit hook
(
echo #!/bin/bash
echo #
echo # Pre-commit hook for Python SDK
echo # Runs formatting, linting, and tests before committing
echo #
echo.
echo set -e
echo.
echo echo "🔍 Running pre-commit checks..."
echo.
echo # Get the directory where the hook is located
echo REPO_DIR=$(git rev-parse --show-toplevel^)
echo cd "$REPO_DIR"
echo.
echo # 1. Run black formatting check
echo echo "📦 Checking code formatting (black^)..."
echo uv run black --check croupier tests
echo.
echo # 2. Run flake8 linting
echo echo "🔍 Running linting (flake8^)..."
echo uv run flake8 croupier tests --max-line-length=100 --ignore=E203,W503
echo.
echo # 3. Run tests
echo echo "🧪 Running tests (pytest^)..."
echo uv run pytest -q --tb=short
echo.
echo echo "✅ All pre-commit checks passed!"
) > %HOOKS_DIR%\pre-commit

REM Create pre-push hook
(
echo #!/bin/bash
echo #
echo # Pre-push hook for Python SDK
echo # Runs full test suite with coverage before pushing
echo #
echo.
echo set -e
echo.
echo echo "🔍 Running pre-push checks..."
echo.
echo REPO_DIR=$(git rev-parse --show-toplevel^)
echo cd "$REPO_DIR"
echo.
echo # Run tests with coverage (minimum 70%%^)
echo echo "🧪 Running tests with coverage..."
echo uv run pytest -q --cov=croupier --cov-report=term --cov-fail-under=70
echo.
echo echo "✅ All pre-push checks passed!"
) > %HOOKS_DIR%\pre-push

echo ✅ Git hooks installed successfully!
echo.
echo Installed hooks:
echo   - pre-commit: Runs black, flake8, and pytest
echo   - pre-push: Runs pytest with coverage (^>^=70%%)
echo.
echo To skip hooks temporarily, use:
echo   git commit --no-verify
echo   git push --no-verify
