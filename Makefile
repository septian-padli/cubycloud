.PHONY: help dev dev-backend dev-frontend build build-frontend build-backend sync-embed test test-backend test-frontend test-e2e docs-bundle docs-check clean

SHELL := /bin/bash

# Default target
help:
	@echo "CubyCloud - Development & Build Commands"
	@echo ""
	@echo "Development:"
	@echo "  make dev            Start both backend and frontend development servers"
	@echo "  make dev-backend    Start Go backend development server"
	@echo "  make dev-frontend   Start SvelteKit frontend development server"
	@echo ""
	@echo "Build:"
	@echo "  make build          Full production build (frontend + embed sync + Go binary)"
	@echo "  make build-frontend Build SvelteKit SPA (apps/frontend/build/)"
	@echo "  make sync-embed     Sync frontend build output to backend embed directory"
	@echo "  make build-backend  Build single Go binary (apps/backend/bin/cubycloud)"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  make test           Run all standard tests (backend + frontend)"
	@echo "  make test-backend   Run Go backend tests"
	@echo "  make test-frontend  Run frontend type check & unit tests"
	@echo "  make test-e2e       Run Playwright end-to-end browser tests"
	@echo ""
	@echo "  make docs-bundle    Bundle OpenAPI fragments into docs/openapi.yaml & backend"
	@echo "  make docs-check     Validate OpenAPI specification and route coverage"
	@echo "Reset & Cleanup:"
	@echo "  make reset-setup    Reset SQLite database & processes for fresh onboarding setup"
	@echo "  make clean          Remove build artifacts and temporary test data"

# Run development environment
dev:
	@./scripts/dev.sh

# Run Go backend standalone
dev-backend:
	@cd apps/backend && go run ./cmd/server/main.go

# Run SvelteKit frontend standalone
dev-frontend:
	@cd apps/frontend && bun run dev

# Build frontend SPA
build-frontend:
	@echo "==> Building frontend SPA..."
	@cd apps/frontend && bun install && bun run build

# Sync built frontend into backend embed package
sync-embed:
	@echo "==> Syncing frontend build to backend web/dist/..."
	@mkdir -p apps/backend/web/dist
	@cp -R apps/frontend/build/* apps/backend/web/dist/

# Build backend Go binary
build-backend:
	@echo "==> Building single Go binary..."
	@mkdir -p apps/backend/bin
	@cd apps/backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/cubycloud ./cmd/server/main.go
	@echo "==> Binary created at apps/backend/bin/cubycloud"

# Full production build
build: build-frontend sync-embed build-backend
	@echo "==> Full build complete! Single executable ready at apps/backend/bin/cubycloud"

# Run all tests
test: test-backend test-frontend

# Run backend tests
test-backend:
	@echo "==> Running backend Go tests..."
	@cd apps/backend && go test ./... -v

# Run frontend type checks and Bun regression tests
test-frontend:
	@echo "==> Running frontend type check and unit tests..."
	@cd apps/frontend && bun run check && bun test src

# Run Playwright E2E browser tests
test-e2e:
	@echo "==> Running Playwright end-to-end tests..."
	@cd apps/frontend && bun run test:e2e

# Bundle OpenAPI 3.1 modular specification
docs-bundle:
	@echo "==> Bundling OpenAPI specification..."
	@cd apps/backend && go run ./cmd/openapi-bundler -source ../../docs/openapi -output ../../docs/openapi.yaml -backend-copy internal/modules/docs/openapi.yaml

# Check OpenAPI 3.1 specification freshness and route coverage
docs-check:
	@echo "==> Validating OpenAPI specification and route coverage..."
	@cd apps/backend && go test -v -run "TestOpenAPI|TestValidateIndividualYamlFiles" ./cmd/...
# Reset setup for clean test
reset-setup:
	@./scripts/reset-setup.sh

# Clean artifacts
clean:
	@echo "==> Cleaning build artifacts..."
	@rm -rf apps/backend/bin/ apps/frontend/build/ /tmp/cubycloud-dev /tmp/recovery-key-new-flow
