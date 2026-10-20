# Testing Guide

## Prerequisites

- Go **1.25.0 or newer**
- Bun **1.x**
- Playwright's Chromium browser for end-to-end tests
- The optional MinIO Compose service for the S3 integration test

Install frontend packages from the repository root:

```bash
cd apps/frontend && bun install
```

## Standard checks

Run the checks included in the root Makefile:

```bash
make test
```

`make test` runs `go test ./...` in `apps/backend` and `bun run check` in `apps/frontend`. The frontend check runs SvelteKit sync and `svelte-check`; it does not run Playwright or ESLint.

Run either part separately:

```bash
make test-backend
make test-frontend
```

The frontend package also defines a formatter/linter check:

```bash
cd apps/frontend && bun run lint
```

## Playwright end-to-end tests

The Playwright configuration starts a temporary backend and Vite server, uses an isolated temporary data directory, and runs the browser tests serially. It uses ports `18383` (backend) and `15188` (frontend) by default; override `MY_STORAGE_E2E_BACKEND_PORT` or `MY_STORAGE_E2E_FRONTEND_PORT` if those ports are occupied.

Install Chromium if it is not already available, then run the suite:

```bash
cd apps/frontend
bunx playwright install chromium
bun run test:e2e
```

The full suite includes the MinIO-backed S3 storage scenario. Start the optional service before running the full suite:

```bash
docker compose -f ../../compose.minio.yaml up -d
```

The E2E tests cover setup provisioning, authentication, file management and resumable uploads, previews, sharing, Organization access, storage administration, quotas, and S3 transfers. They use synthetic accounts and temporary data; do not add real credentials or user files to fixtures.

## Test scope notes

- Backend Go tests exercise service, handler, middleware, storage, migration, and concurrency behavior.
- Playwright tests exercise browser-visible flows against the local backend and frontend.
- `make test` does not include the Playwright suite.
- The development MinIO service is test infrastructure only; it is not required for ordinary local-storage development.
