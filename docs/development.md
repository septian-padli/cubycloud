# Development Guide

This guide covers running CubyCloud from source. The repository's Docker image currently has a Go toolchain mismatch; see the container build note in the [README](../README.md#container-build-status).

## Requirements

- Go **1.25.0 or newer** (`apps/backend/go.mod`)
- Bun **1.x** (`apps/frontend/package.json` and `bun.lock`)
- Air only when using the `make dev` convenience script
- Docker Compose only for the optional MinIO S3 integration service

## Run the full development stack

Install Air if it is not already available, install frontend dependencies, then start the development script:

```bash
go install github.com/air-verse/air@latest
cd apps/frontend && bun install
cd ../..
make dev
```

The script starts the backend on `http://localhost:8383` and the Vite frontend on `http://localhost:5188`. It uses `/tmp/cubycloud-dev` for development data and creates `apps/backend/.env` if that file does not exist. Treat that directory and its development settings as disposable; do not use them for production data or secrets.

Open `http://localhost:5188/setup` for first-time setup. The setup wizard creates the initial Super Admin account, configures storage, and issues a recovery key. Save the recovery key securely.

## Run the services separately

The following commands run the backend and frontend in separate terminals without Air.

**Terminal 1 — backend:**

```bash
cd apps/backend
PORT=8383 \
DATA_DIR_PATH="$HOME/cubycloud-dev-data" \
JWT_SECRET="$(openssl rand -hex 32)" \
go run ./cmd/server/main.go
```

**Terminal 2 — frontend:**

```bash
cd apps/frontend
bun install
VITE_API_PROXY_TARGET=http://localhost:8383 \
bun run dev --host 127.0.0.1 --port 5188
```

The selected `DATA_DIR_PATH` persists the SQLite database and file data between local runs. Keep it outside version control and back it up if it contains data you need.

## Optional S3-compatible development service

The checked-in Compose file provides a local MinIO service for development and S3 integration tests:

```bash
docker compose -f compose.minio.yaml up -d
```

- S3 API: `http://localhost:19000`
- MinIO Console: `http://localhost:19001`
- Development bucket: `cubycloud-dev`

The Compose file contains local-only development credentials and provisions a bucket-scoped application account. Do not reuse those values outside a local test environment. Stopping the stack with `docker compose -f compose.minio.yaml down` preserves its named data volume; avoid `down -v` unless intentionally deleting that local data.

See [Testing](testing.md) for unit, frontend, lint, and Playwright commands.
