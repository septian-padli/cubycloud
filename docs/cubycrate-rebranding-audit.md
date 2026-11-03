# CubyCrate Rebranding — Phase 1: Application Repository Audit Report

**Repository:** `septian-padli/cubycloud`  
**Base Branch:** `main`  
**Target Brand:** CubyCrate (`cubycrate`)  
**Production Domain:** `https://cubycrate.com`  
**Tagline:** *Your files. Your server. Your rules.*  
**Positioning:** *Lightweight, open-source, self-hosted file drive*  
**Audit Mode:** Read-Only Audit (Phase 1)

---

## 1. Current State

The repository is a monolithic single-container application consisting of:
- **Backend:** Go 1.25.6 HTTP service (`cmd/server/main.go`, Echo framework, Bun ORM)
- **Frontend:** SvelteKit SPA (`adapter-static`, Tailwind CSS tokens, embedded via Go `embed.FS` into `web/dist`)
- **Database:** Embedded SQLite in WAL mode (`db/cubycloud.db`)
- **Packaging:** Multi-stage Docker container deployed via GHCR (`ghcr.io/septian-padli/cubycloud`) and `compose.yaml`

The name **CubyCloud** (`cubycloud`, `CUBYCLOUD`) is deeply embedded across:
1. User-facing branding (titles, copy, error messages, placeholders, SVGs)
2. Go module namespace (`github.com/cubycloud/backend`)
3. Storage blob directory namespace (`cubycloud/{instance_id}/{node_id}/blobs/{blob_id}`)
4. Primary database filename (`db/cubycloud.db`)
5. Session authentication cookies (`cuby_token`)
6. Browser storage (localStorage keys, IndexedDB database, BroadcastChannels, Web Locks)
7. Docker volumes (`cubycloud_data`) and Compose service networking (`cubycloud:8383`)
8. Environment variables (`CUBYCLOUD_*`)

---

## 2. Affected Files and Categories

| Category | Key Files / Paths | Description / Occurrences |
| :--- | :--- | :--- |
| **1. Public-Facing Brand Identity** | `apps/frontend/src/routes/**/*.svelte`<br>`apps/frontend/src/lib/components/drive/drive-sidebar.svelte`<br>`apps/frontend/src/lib/components/auth/auth-split-shell.svelte`<br>`apps/frontend/src/lib/components/share/share-public-shell.svelte` | Brand headers, copyright notices ("Copyright (C) 2026 CubyCloud contributors"), page titles, and hero branding. |
| **2. Documentation & Marketing** | `README.md`<br>`docs/*.md` (architecture, backup-restore, release-guide, etc.)<br>`CONTRIBUTING.md`<br>`SECURITY.md`<br>`SOUL.md`<br>`.cursor/WORKFLOW_RULES.md`<br>`AGENTS.md` | Project descriptions, setup guides, architectural diagrams, runbooks, and contributing guidelines. |
| **3. Frontend UI & Assets** | `apps/frontend/src/app.html`<br>`apps/frontend/src/lib/assets/auth-hero-placeholder.svg`<br>`apps/frontend/src/lib/mocks/file-tree.ts`<br>`apps/frontend/src/lib/mocks/starred-files.ts` | HTML `<title>`, SVG placeholder graphic texts, mock sample files (`Logo CubyCloud.svg`), and UI tooltips. |
| **4. API & Application Config** | `apps/backend/cmd/server/main.go`<br>`apps/backend/internal/config/config.go`<br>`apps/backend/internal/modules/docs/handler.go`<br>`docs/openapi.yaml`<br>`docs/openapi/**/*.yaml` | Log messages ("Starting CubyCloud"), Swagger UI HTML title, and OpenAPI 3.1 specification titles/descriptions. |
| **5. Go Module Paths & Imports** | `apps/backend/go.mod`<br>73 Go source files across `apps/backend/` | Go module identifier `github.com/cubycloud/backend` and all internal package imports. |
| **6. Docker & GHCR Packaging** | `Dockerfile`<br>`compose.yaml`<br>`compose.minio.yaml`<br>`Makefile` | Binary name `/app/cubycloud`, OCI image labels, Compose service name `cubycloud`, container name, and MinIO development buckets. |
| **7. Database Paths & Volumes** | `apps/backend/internal/config/config.go:95`<br>`apps/backend/internal/database/db.go:22`<br>`apps/backend/internal/storage/namespace.go:40,45`<br>`compose.yaml:10,24` | SQLite file `db/cubycloud.db`, Docker volume `cubycloud_data`, and storage blob key prefix `cubycloud/...`. |
| **8. Environment Variables** | `apps/backend/internal/config/config.go:100-106`<br>`.env.example`<br>`compose.yaml:4,8,13,25` | `CUBYCLOUD_MASTER_KEY`, `CUBYCLOUD_MASTER_KEY_FILE`, `CUBYCLOUD_STORAGE_CLEANUP_*`, `CUBYCLOUD_PORT`, `CUBYCLOUD_VERSION`, etc. |
| **9. Browser Storage & Cookies** | `apps/backend/internal/middleware/jwt_auth.go:19`<br>`apps/backend/internal/modules/auth/handler.go:74`<br>`apps/frontend/src/lib/theme.svelte.ts:3`<br>`apps/frontend/src/lib/drive-view.svelte.ts:3`<br>`apps/frontend/src/lib/transfer-queue.svelte.ts:3,9,513`<br>`apps/frontend/src/lib/locale.svelte.ts:10` | Cookie `cuby_token`, localStorage keys (`cubycloud-theme`, `cubycloud-drive-view`, `cubycloud-language:*`, `cubycloud_setup_draft`), IndexedDB `cubycloud-transfer-queue`, Web Locks `cubycloud-transfer:*`. |
| **10. Release & CI Automation** | `.github/workflows/ci.yml`<br>`.github/workflows/release.yml` | Image verification tags `cubycloud:ci-verify`, GitHub Release title generation, and release note compose templates. |
| **11. Tests & Fixtures** | `apps/backend/**/*_test.go`<br>`apps/frontend/e2e/*.spec.ts`<br>`scripts/test/*.sh` | Integration test mock buckets (`cubycloud-dev`), temporary directories (`/tmp/cubycloud-dev`), and test email fixtures (`@cubycloud.local`). |

---

## 3. Safe Rename Targets

These items can be renamed immediately in Phase 1 without breaking existing databases, storage blobs, or Docker volumes:

1. **User Interface Text & Page Titles:**
   - `<title>` tags across all routes (`Masuk — CubyCloud` → `Sign in — CubyCrate`, etc.).
   - Brand titles in `drive-sidebar.svelte`, `auth-split-shell.svelte`, `share-public-shell.svelte`, and `error-page.svelte`.
   - Setup wizard step copy and primary CTA button (`Selesaikan Setup & Masuk ke CubyCrate`).
   - Placeholder copy, input labels, and alert dialogs.
2. **Landing Page & Metadata (`apps/frontend/src/routes/+page.svelte`):**
   - Headline and positioning: "CubyCrate — Lightweight, open-source, self-hosted file drive".
   - Tagline: "Your files. Your server. Your rules.".
   - Domain links updated to `https://cubycrate.com`.
3. **Hero & Graphic Assets:**
   - `apps/frontend/src/lib/assets/auth-hero-placeholder.svg`: Update title and text node to `CubyCrate`.
   - Mock demo files in `file-tree.ts` (`Logo CubyCrate.svg`).
4. **Documentation & Guides:**
   - `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `SOUL.md`, `RTK.md`.
   - All architecture and operational guides under `docs/`.
   - GitHub Issue templates (`.github/ISSUE_TEMPLATE/*.md`).
5. **OpenAPI & API Documentation:**
   - Swagger UI page title in `apps/backend/internal/modules/docs/handler.go` (`CubyCrate API Documentation`).
   - OpenAPI title, description, and server summaries in `docs/openapi.yaml` and `docs/openapi/root.yaml`.
6. **Downloaded File Defaults (Non-persistent):**
   - Emergency recovery key export: `cubycrate-recovery-key.txt` and `cubycrate-recovery-key-${Date.now()}.txt`.
   - Batch ZIP download: `cubycrate-archive.zip` in `mystorage/handler.go` and `organization/handler.go`.
7. **CI Pipeline Image Tags:**
   - `cubycloud:ci-verify` → `cubycrate:ci-verify` in `.github/workflows/ci.yml`.

---

## 4. Protected Legacy Identifiers

The following identifiers **MUST NOT** be modified via blanket search-and-replace. They require strict preservation or backward-compatible fallback mechanisms:

### Critical Tier (Data Loss / Access Loss Risk)

1. **Storage Blob Namespace (`apps/backend/internal/storage/namespace.go` & `orphan.go`)**
   - **Current Implementation:** `BuildBlobKey` returns `"cubycloud/" + instanceID + "/" + nodeID + "/blobs/" + blobID`.
   - **Protection Requirement:** Every file already uploaded on local disks and S3 object storage uses this physical path and is stored in `my_storage_items.disk_key` and `organization_items.disk_key`.
   - **Rule:** Do NOT rename existing blob paths. If new keys use `cubycrate/`, `ParseBlobKey` and `orphan.go` MUST support both prefixes; otherwise, existing files will become unreadable or get purged as orphans.

2. **Docker Named Volume (`compose.yaml:24`)**
   - **Current Implementation:** `cubycloud_data:/app/data` (mapped via `${CUBYCLOUD_VOLUME_NAME:-cubycloud_data}`).
   - **Protection Requirement:** If `compose.yaml` changes the default volume name to `cubycrate_data`, Docker Compose will create a fresh empty volume upon upgrade. Existing databases, master encryption keys, and files will be detached.
   - **Rule:** Default volume name MUST remain `cubycloud_data` for existing setups, or Compose must provide explicit upgrade instructions.

3. **SQLite Database Path (`apps/backend/internal/database/db.go:22` & `config/config.go:95`)**
   - **Current Implementation:** `filepath.Join(dataDir, "db", "cubycloud.db")`.
   - **Protection Requirement:** Changing the hardcoded path to `cubycrate.db` causes the application to create a new, empty database on existing volumes, triggering the onboarding wizard again.
   - **Rule:** Retain `cubycloud.db` as the default on-disk filename, OR implement fallback resolution:
     ```go
     // If cubycrate.db exists, use it; else if cubycloud.db exists, use it; else create cubycrate.db
     ```

### High Tier (Security & Session Disruption Risk)

4. **Master Key Environment Variables (`apps/backend/internal/config/config.go:100-101`)**
   - **Current Implementation:** `os.Getenv("CUBYCLOUD_MASTER_KEY")` and `os.Getenv("CUBYCLOUD_MASTER_KEY_FILE")`.
   - **Protection Requirement:** S3 node credentials in `storage_nodes.access_key_encrypted` and `secret_key_encrypted` are sealed with this key.
   - **Rule:** Implement dual-resolution in `config.go`:
     ```go
     masterKey := getEnvFallback("CUBYCRATE_MASTER_KEY", "CUBYCLOUD_MASTER_KEY")
     masterKeyFile := getEnvFallback("CUBYCRATE_MASTER_KEY_FILE", "CUBYCLOUD_MASTER_KEY_FILE")
     ```

5. **Session Authentication Cookie (`apps/backend/internal/middleware/jwt_auth.go:19`)**
   - **Current Implementation:** `c.Cookie("cuby_token")`.
   - **Protection Requirement:** If the cookie name is switched to `cubycrate_token`, all currently active user sessions will be immediately invalidated upon deployment.
   - **Rule:** JWT middleware must check `cubycrate_token` first, falling back to `cuby_token`.

6. **Frontend State & Theme LocalStorage (`apps/frontend/src/app.html:8` & `theme.svelte.ts:3`)**
   - **Current Implementation:** `localStorage.getItem('cubycloud-theme')`.
   - **Protection Requirement:** Blanket renaming will cause dark-mode users to experience a white flash (FOUT) and reset their view settings.
   - **Rule:** Read `cubycrate-theme` with fallback to `cubycloud-theme`.

---

## 5. Compatibility Risks

```
[Existing Deployment]
      │
      ├── Upgrade to CubyCrate Binary ──► Check db/cubycrate.db? ──► NOT FOUND ──► [Empty DB / Data Lost] (CRITICAL)
      │                                       │ (Must fallback to db/cubycloud.db)
      │
      ├── Compose Volume cubycrate_data? ────────────────────────► [Empty Volume Created] (CRITICAL)
      │                                       │ (Must keep cubycloud_data as default)
      │
      ├── Orphan Scanner: scan prefix? ─► "cubycrate/..." only ──► Ignores "cubycloud/..." ──► [Orphan Audit Failure] (CRITICAL)
      │                                       │ (Must scan both or keep prefix)
      │
      └── Reverse Proxy (Caddy/Nginx) ──► Proxy to cubycloud:8383 ─► Container renamed? ──► [502 Bad Gateway] (MEDIUM)
```

1. **Volume Disconnection:** Docker Compose named volumes do not auto-migrate. Changing `volumes.cubycloud_data` in `compose.yaml` detaches user data.
2. **SQLite WAL Desynchronization:** If renaming `cubycloud.db` to `cubycrate.db` on disk while SQLite WAL files (`cubycloud.db-wal`, `cubycloud.db-shm`) exist, database corruption can occur if not cleanly checkpointed.
3. **Master Key Desynchronization:** If an operator set `CUBYCLOUD_MASTER_KEY` in their container environment or Docker secrets, omitting fallback will cause startup failure or permanent inability to read S3 nodes.
4. **Reverse Proxy Outage:** Existing templates in `deploy/caddy/Caddyfile` and `deploy/nginx/cubycloud.conf` reference upstream `cubycloud:8383`. Renaming the Compose service breaks reverse proxy routing unless aliased.

---

## 6. Recommended Implementation Tasks

### Wave 1: Presentation & Documentation (Zero Risk)
- [ ] Update frontend UI text, page titles, and meta descriptions across Svelte routes.
- [ ] Update landing page (`+page.svelte`) with official tagline: *"Your files. Your server. Your rules."* and positioning.
- [ ] Update placeholder SVG branding (`auth-hero-placeholder.svg`).
- [ ] Update documentation (`README.md`, `docs/*.md`, `CONTRIBUTING.md`, `SECURITY.md`).
- [ ] Update Swagger UI header and OpenAPI specifications (`CubyCrate API`).

### Wave 2: Browser Storage & Client Transition (Low Risk)
- [ ] Implement dual-key reading in `theme.svelte.ts` and `app.html` (`cubycrate-theme` -> `cubycloud-theme`).
- [ ] Implement dual-key reading in `drive-view.svelte.ts` (`cubycrate-drive-view` -> `cubycloud-drive-view`).
- [ ] Implement dual-key reading in `locale.svelte.ts` (`cubycrate-language:*` -> `cubycloud-language:*`).
- [ ] Migrate IndexedDB database name in `transfer-queue.svelte.ts` with transparent version check.

### Wave 3: Backend Configuration & Backward Compatibility (Medium Risk)
- [ ] Update `config.go` to support `CUBYCRATE_*` env vars with full fallback to `CUBYCLOUD_*`.
- [ ] Update `jwt_auth.go` to accept `cubycrate_token` with fallback to `cuby_token`.
- [ ] Add runtime database path fallback in `db.go` (`cubycrate.db` if present, else `cubycloud.db`).
- [ ] Maintain `storage.BuildBlobKey` prefix or implement dual-prefix parser in `namespace.go` and `orphan.go`.

### Wave 4: Packaging & Orchestration (Medium Risk)
- [ ] Update `Dockerfile` metadata labels and internal binary destination (`/app/cubycrate`).
- [ ] Update `compose.yaml` container and service definitions, ensuring volume fallback to `cubycloud_data`.
- [ ] Add network alias in `compose.yaml` so `cubycloud:8383` and `cubycrate:8383` both resolve.
- [ ] Update `Makefile` targets.

### Wave 5: Go Module Path (Atomic Refactor - Optional in Phase 1)
- [ ] If approved, rename `github.com/cubycloud/backend` to `github.com/cubycrate/backend` across `go.mod` and all 73 source files in a single atomic commit.

---

## 7. Testing and Regression Plan

| Test Scope | Target / Scenario | Expected Verification Result |
| :--- | :--- | :--- |
| **Existing Installation Upgrade** | Run current `main` image with data, stop container, start new CubyCrate image on the exact same data directory. | Database opens without triggering `/setup`; users and files remain intact. |
| **S3 Credential Decryption** | Existing S3 node credentials encrypted with `CUBYCLOUD_MASTER_KEY`. | New binary starts, reads `CUBYCLOUD_MASTER_KEY` fallback, decrypts access keys, and downloads file successfully. |
| **Storage Blob Access** | File previously uploaded with key `cubycloud/<inst>/<node>/blobs/<id>`. | `ReadLocal` and presigned S3 URLs resolve and stream file without 404 or path traversal errors. |
| **Orphan Reconciliation Audit** | Run startup orphan reconciliation audit on storage nodes with legacy keys. | Legacy `cubycloud/...` objects are recognized as valid referenced blobs; not flagged as orphans. |
| **Session Persistence** | User logged in with `cuby_token` cookie before upgrade. | Requests with legacy `cuby_token` succeed without forced logout. |
| **Theme / Language Persistence** | User with dark mode and custom language preference in browser localStorage. | Page reloads without flash of light theme or language reset. |
| **Full Regression Suite** | Run `make test`, `bun run check`, `bun run lint`, and Playwright E2E. | All unit, integration, and E2E browser tests pass cleanly. |

---

## 8. Dependencies on Marketing Website & Docs Repository

1. **Domain Cutover:** DNS records and SSL provisioning for `https://cubycrate.com`.
2. **Marketing Site / Repository:**
   - Landing page assets, feature highlights, and screenshots matching the new brand name.
   - Update external links from the app's coming-soon page to `https://cubycrate.com`.
3. **Official Brand Logo Assets:**
   - Vector logo (SVG) and raster icons (PNG / WebP) for dark and light modes.
   - `favicon.svg`, `favicon-32x32.png`, and `favicon-16x16.png`.
   - OpenGraph preview image (`og-image.png`).

---

## 9. Open Decisions Requiring Owner Approval

1. **Go Module Path Rename:**
   - *Option A (Recommended):* Keep `github.com/cubycloud/backend` during Phase 1 while the GitHub repository name remains `septian-padli/cubycloud`.
   - *Option B:* Atomically rename to `github.com/cubycrate/backend` now.
2. **Storage Namespace Prefix Strategy:**
   - *Option A (Safest):* Keep physical blob key prefix as `cubycloud/...` internally for all existing and new files.
   - *Option B:* Update new uploads to `cubycrate/...`, but update `namespace.go` and `orphan.go` to dual-read both `cubycrate/` and `cubycloud/`.
3. **Database Filename Strategy:**
   - *Option A (Safest):* Keep default database filename as `cubycloud.db`.
   - *Option B:* Implement runtime migration check (if `cubycloud.db` exists, continue using it; only new installations use `cubycrate.db`).
4. **Docker Named Volume Strategy:**
   - *Option A (Safest):* Keep `volumes.cubycloud_data` as the default in `compose.yaml`.
   - *Option B:* Introduce `cubycrate_data` with fallback or explicit upgrade migration documentation.
5. **Session Cookie Name:**
   - *Option A:* Keep `cuby_token` (brand-neutral prefix).
   - *Option B:* Switch to `cubycrate_token` with dual-read fallback for `cuby_token`.
