# CubyCloud Architecture and Runtime Model

This document describes the implementation present in the repository. Product behavior should be checked against the running application and tests; this is not a promise of unpublished performance targets.

## System overview

CubyCloud is a Go HTTP service that provides the JSON API and serves the built SvelteKit application. The frontend is compiled to static assets and embedded in the Go server build. SQLite stores application metadata; file contents are stored through the storage driver on local disks or an optional S3-compatible service.

```text
Browser
  ├── SvelteKit static application ── served by the Go server
  └── /api/v1/* ───────────────────── Go HTTP handlers
                                          ├── SQLite (metadata, WAL)
                                          └── Storage driver
                                                ├── Local filesystem
                                                └── S3-compatible service (optional)
```

## Technology

| Area              | Implementation                                                                                     |
| ----------------- | -------------------------------------------------------------------------------------------------- |
| Backend           | Go module requiring Go 1.25.0; Echo HTTP framework                                                 |
| Persistence       | SQLite through `database/sql`, Bun, and `glebarez/go-sqlite`; WAL mode and embedded SQL migrations |
| Frontend          | SvelteKit static build, Svelte 5, TypeScript, Tailwind CSS 4                                       |
| Object storage    | AWS SDK for Go v2; S3-compatible endpoints are configured through the admin interface              |
| Local development | Bun/Vite frontend and Go backend; root `Makefile` provides common commands                         |

Backend modules include setup, authentication, administration, My Storage, Organization, and sharing. The frontend contains the corresponding setup, drive, public-share, and admin surfaces.

## Request and data flow

- The Go server exposes API routes under `/api/v1/` and serves the embedded frontend for browser routes.
- The default backend port is `8383`. `PORT` can override it.
- SQLite migrations are embedded from `apps/backend/internal/database/migrations/` and run when the database is opened.
- SQLite is configured for WAL mode, foreign keys, a busy timeout, and a single open connection.
- Local and S3-compatible storage are accessed through the storage driver. S3 uploads can use presigned browser transfers; My Storage also supports resumable TUS uploads.
- My Storage and Organization share physical node capacity checks. User quota accounting is separate from physical node capacity.

## Runtime data and configuration

`DATA_DIR_PATH` selects the application data root. The default is `/app/data` when that directory is writable, otherwise `./data` relative to the process working directory. In the Dockerfile, `/app/data` is declared as a volume.

The application stores the SQLite database at `DATA_DIR_PATH/db/cubycloud.db`; local storage nodes and temporary TUS upload data also live under this data root. Mount a persistent, writable directory or volume and back it up as application data.

Boot-time settings are read from process environment variables (and supported `.env` files):

| Variable          | Purpose                                                                              |
| ----------------- | ------------------------------------------------------------------------------------ |
| `PORT`            | HTTP listen port; defaults to `8383`                                                 |
| `DATA_DIR_PATH`   | Root directory for the database and storage data                                     |
| `JWT_SECRET`      | Secret used to sign sessions; required when `ENV=production` or `APP_ENV=production` |
| `ENV` / `APP_ENV` | Selects production safety behavior                                                   |

Application settings such as default quota and audit retention are managed through the admin interface and persisted in SQLite.

## Storage and capacity

The storage driver supports local filesystem nodes and configured S3-compatible nodes. S3/R2 credentials are encrypted before they are persisted. Browser-to-S3 transfers use presigned requests where supported; local uploads use the backend and resumable TUS protocol.

Physical node capacity is enforced independently from per-user My Storage quota. Organization data is not charged to an individual user's quota, but it still consumes shared node capacity. Capacity checks include active upload reservations to prevent concurrent uploads from over-allocating a node.

## Security boundaries

- User passwords are bcrypt-hashed.
- Authenticated sessions are carried in HTTP-only `SameSite=Lax` cookies.
- Login and public share-unlock endpoints have an in-memory rate limiter; its counters reset when the server restarts.
- Storage credentials are encrypted; this does not imply that uploaded file contents are end-to-end encrypted.
- The server requires an explicit `JWT_SECRET` when started in production mode.

See [SECURITY.md](../SECURITY.md) for disclosure guidance. No CPU, memory, latency, or image-size benchmark is published in this document.

## Build and deployment status

The Dockerfile defines a multi-stage frontend/backend build and declares `/app/data` as a volume.

**Current limitation:** `apps/backend/go.mod` requires Go 1.25.0, but the Dockerfile's backend build stage uses `golang:1.24-alpine`. A Docker build currently fails at `go mod download` because the builder's toolchain is too old. The Dockerfile is intentionally unchanged by this documentation task; do not treat its image as a verified installation path until the mismatch is fixed.
