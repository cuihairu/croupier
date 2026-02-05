# Git Hooks

This directory contains Git hooks that are version-controlled.

## Setup

After cloning the repository, run:

```bash
git config core.hooksPath .githooks
```

This tells Git to use the hooks in this directory instead of `.git/hooks/`.

## Available Hooks

- **pre-commit**: Runs `gofmt` and `goimports` before committing
