# E2E Testing Makefile
# Usage: make -f scripts/e2e/Makefile [target]

.PHONY: all health-check performance ui-config help

# Default target
all: run-all

# Run all E2E tests
run-all:
	@echo "Running all E2E tests..."
	@bash scripts/e2e/run-all.sh

# Health check only
health-check:
	@echo "Running health check..."
	@bash scripts/e2e/health-check.sh

# Performance tests
performance:
	@echo "Running performance tests..."
	@bash scripts/e2e/test-performance.sh

# UI configuration tests
ui-config:
	@echo "Running UI configuration tests..."
	@bash scripts/e2e/test-ui-configuration.sh

# Setup (add execute permissions to scripts)
setup:
	@echo "Adding execute permissions to E2E scripts..."
	@chmod +x scripts/e2e/*.sh
	@echo "Done! Scripts are now executable."

# Help
help:
	@echo "E2E Testing Targets:"
	@echo "  make -f scripts/e2e/Makefile all          Run all E2E tests"
	@echo "  make -f scripts/e2e/Makefile health-check  Run health check only"
	@echo "  make -f scripts/e2e/Makefile performance   Run performance tests"
	@echo "  make -f scripts/e2e/Makefile ui-config     Run UI configuration tests"
	@echo "  make -f scripts/e2e/Makefile setup         Add execute permissions"
	@echo ""
	@echo "Environment Variables:"
	@echo "  SERVER_URL    Server URL (default: http://localhost:18780)"
	@echo "  DASHBOARD_URL Dashboard URL (default: http://localhost:8000)"
	@echo ""
	@echo "Example:"
	@echo "  make -f scripts/e2e/Makefile all"
	@echo "  SERVER_URL=http://localhost:18780 make -f scripts/e2e/Makefile all"
