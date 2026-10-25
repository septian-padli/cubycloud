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

The application stores the SQLite database at `DATA_DIR_PATH/db/cubycloud.db`; local storage nodes and temporary TUS upload data also live under this data root. TUS chunks and metadata default to `DATA_DIR_PATH/temp/tus` (overridable with `TEMP_TUS_PATH` or `TUS_TEMP_PATH`) so unfinished transfers survive a container replacement when the same volume is retained. Mount a persistent, writable directory or volume and back it up as application data.

Boot-time settings are read from process environment variables (and supported `.env` files):

| Variable          | Purpose                                                                              |
| ----------------- | ------------------------------------------------------------------------------------ |
| `PORT`            | HTTP listen port; defaults to `8383`                                                 |
| `DATA_DIR_PATH`   | Root directory for the database and storage data                                     |
| `JWT_SECRET`      | Secret used to sign sessions; required when `ENV=production` or `APP_ENV=production` |
| `ENV` / `APP_ENV` | Selects production safety behavior                                                   |

Application settings such as default quota and audit retention are managed through the admin interface and persisted in SQLite. The audit log subsystem captures all mutating HTTP operations (`POST`, `PUT`, `PATCH`, `DELETE`) with sensitive credential redaction, preserving non-mutating `GET` requests from log bloat. A background worker (`AUDIT_CLEANUP_JOB`) purges records older than the configured retention period (7, 14, 30, 60, or 90 days).

## Storage and capacity

The storage driver supports local filesystem nodes and configured S3-compatible nodes. S3/R2 credentials are encrypted before they are persisted. Browser-to-S3 transfers use presigned requests where supported; local uploads use the backend and resumable TUS protocol.

Physical node capacity is enforced independently from per-user My Storage quota. Organization data is not charged to an individual user's quota, but it still consumes shared node capacity. Capacity checks include active upload reservations to prevent concurrent uploads from over-allocating a node.

For browser-to-S3 uploads, a presigned PUT URL lasts 15 minutes, while its pending upload intent lasts 24 hours. Finalization verifies that S3 reports exactly the requested object size before recording metadata. A failed metadata commit preserves the object and intent for retry; cancellation or expiry deletes a cloud object only when no My Storage or Organization item still references it.
On server startup, an orphan-file reconciliation audit scans registered local filesystem and S3 nodes, removing unreferenced storage blobs while safeguarding active files, trash items, unexpired upload intents, and recent writes within a safety grace period.
Batch ZIP downloads use the existing authenticated POST endpoint. The browser submits selected IDs as form fields and handles the ZIP attachment as a native download, without collecting the response in JavaScript memory. Native downloads do not expose byte progress or reliable completion events to the page; the UI reports that the download has started, not that it has finished.

## Background job queue, Compress to ZIP, and Node Detach Migration

Asynchronous server-side operations are coordinated using a SQLite-backed transactional queue (`jobs` table) and a bounded Go worker pool. Concurrency defaults to `2` (governed by `settings.worker_concurrency_limit`).

- **Atomic Job Claims:** Workers acquire pending tasks using atomic `UPDATE jobs SET status = 'PROCESSING' WHERE id = (SELECT id FROM jobs WHERE status = 'PENDING' ORDER BY created_at ASC LIMIT 1) RETURNING *`.
- **Compress to ZIP Workflow:** Invoked via `POST /api/v1/drive/my-storage/batch/compress-zip` and `POST /api/v1/drive/organization/batch/compress-zip`, returning `202 Accepted` with a `job_id`. Selected files/folders are recursively packaged into a temporary archive, verified against quota (for personal storage) and storage capacity, placed on a local or S3 node via placement strategy, and registered as a `.zip` file in the folder where the user triggered the action (`parent_id`). Sibling naming conflicts resolve with numbered suffixes (`Archive (1).zip`).
- **Rollback & Restart Recovery:** Job errors or cancellations immediately unlink temporary archive files and do not commit VFS metadata. Upon server boot, lingering `PROCESSING` jobs automatically transition to `FAILED` (`Interrupted by server restart`), preventing hung states.
- **Real-Time Events:** Real-time updates stream through SSE at `GET /api/v1/events` (authenticated), while direct job polling remains available via `GET /api/v1/jobs/:id`.
- **Node Detach & Physical Migration Workflow:** Triggered via `POST /api/v1/admin/storage-nodes/:id/detach`, returning `safe_to_disconnect: true` immediately if the node has zero used bytes, or enqueuing a `node_migration` job when files are present. The background worker disables the source node (`enabled = false`), streams each unique blob across drivers (`LOCAL ↔ S3`), verifies the destination with SHA-256 and byte length checks, and updates VFS pointers atomically. Under the zero-deletion invariant, source blobs are never deleted, ensuring that any migration error leaves source files completely readable. A node is declared safe to disconnect only when all files have been migrated (`used_bytes = 0`).

## Health, storage availability, and interface-language contract

- `GET /health` is unauthenticated. It returns HTTP `200` only if every currently configured storage node is accessible, and HTTP `503` if any one is inaccessible. The health check probes all registered nodes. Its public response does not expose storage names, physical paths, or credentials.
- My Storage and Organization file-list items expose the boolean JSON field `storage_available` for each file, based on whether its associated storage node is accessible. Listings retain offline file metadata rather than converting unavailable storage into a not-found result. Folders remain navigable and available when a node is offline.
- Regular users do not receive physical node identifiers or names. The availability signal does not identify the node.
- The UI supports English and Indonesian. Initial language selection uses the browser language: `id*` selects Indonesian; otherwise English. An explicit choice is stored only in browser `localStorage`, keyed by authenticated account ID, allowing separate preferences per account in the same browser. The preference is never persisted server-side.

## Security boundaries

- User passwords are bcrypt-hashed.
- Authenticated sessions are carried in HTTP-only `SameSite=Lax` cookies with `Secure` enforced when `COOKIE_SECURE=true`, direct TLS is active, or reverse proxies forward `X-Forwarded-Proto: https`.
- Login and public share-unlock endpoints have an in-memory rate limiter; its counters reset when the server restarts.
- Storage credentials are encrypted; this does not imply that uploaded file contents are end-to-end encrypted.
- In production mode (`ENV=production` or `APP_ENV=production`), the server enforces an explicit `JWT_SECRET`, `COOKIE_SECURE=true`, and explicit `ALLOWED_ORIGINS` (rejecting wildcards `*`), refusing startup if any are missing.

See [SECURITY.md](../SECURITY.md) for disclosure guidance. No CPU, memory, latency, or image-size benchmark is published in this document.

## Build and deployment status

The Dockerfile defines a multi-stage frontend/backend build and declares `/app/data` as a volume.

**Current limitation:** `apps/backend/go.mod` requires Go 1.25.0, but the Dockerfile's backend build stage uses `golang:1.24-alpine`. A Docker build currently fails at `go mod download` because the builder's toolchain is too old. The Dockerfile is intentionally unchanged by this documentation task; do not treat its image as a verified installation path until the mismatch is fixed.
