# CubyCloud — Product Requirements Document (PRD)

> **Product:** CubyCloud, self-hosted modular cloud storage · **Scope:** v1.0 product contract  
> **Status:** Draft / In Review · **Original author:** Septian Padli · **Created:** 7 August 2026  
> **Historical target launch:** 1 October 2026 (a planning date, **not** proof of release)  
> **Audience:** Core engineering, design, QA, contributors

## Reading guide for OMP and developers

**This document is the product/behavior source of truth, not a checklist that every agent must read cover to cover.** Start with the module index, read the relevant module and any applicable cross-cutting requirements (§3, §5), and expand only for dependencies or contract changes. Scoped `AGENTS.md` govern agent execution; `docs/architecture.md` governs system implementation; `apps/frontend/SITEMAP.md` governs routes and UI interactions; `apps/frontend/DESIGN.md` governs visual tokens. For a full PRD revision or cross-cutting architectural decision, review all affected modules and linked contracts.

| Change area | Read first | Also inspect when applicable |
| --- | --- | --- |
| Setup/recovery | §4.1, §5.2 | §4.9, architecture, SITEMAP |
| Storage nodes/health | §4.2, §3.2 | §4.10, §5.3–5.4 |
| Home and explorer | §4.3–4.4 | §4.8, §4.10, §5.1 |
| Transfer/upload/download | §4.4 | §4.2, §4.10, §5.3 |
| Search | §4.4 (search) | §4.8, §5.2, SITEMAP |
| Trash/starred/stats | §4.5–4.7 | §4.10 |
| Sharing/Organization | §4.8, §4.11 | §4.4, §4.10, §5.2 |
| Admin/users/quota | §4.2, §4.9–4.10 | §5.2 |
| Archive/workers | §4.12 | §4.4, §5.3 |
| Profile/auth/session | §4.13, §5.2 | §4.1, §4.9 |
| Infrastructure/release | §3.2, §5, §6 | architecture, Dockerfile, Makefile |

**Contract discipline:** An explicit, newer approved user decision overrides an older requirement for the active task; update this PRD when the product contract changes. Distinguish requirements/targets from implemented, tested, or released features. Do not infer delivery status from this document. See the **Open alignment checks** section for known discrepancies requiring deliberate resolution.

---

## 1. Product vision, audience, and success criteria

### 1.1 Purpose and positioning

CubyCloud is a **lightweight, modern, modular self-hosted alternative to Nextcloud and commercial cloud drives**. It integrates several local disks and S3-compatible buckets behind one virtual directory, with a simple web-based setup and a Google Drive-like interface. The primary value is owning your data and managing heterogeneous storage on low-resource hardware without maintaining multiple runtime services.

The product must remain **single-container / single-executable**, with a Go backend and embedded SvelteKit SPA, SQLite, local and S3-compatible storage, and zero-CLI initial configuration from the web. Privacy means storage under operator control; the application does **not** promise end-to-end encryption or automatic encryption of physical file bytes.

### 1.2 Target users

| Persona | Environment and needs | Key outcomes |
| --- | --- | --- |
| Homelab enthusiast / minimal sysadmin | Legacy Celeron server, 2 GB RAM, mixed HDD/SSD + optional R2/S3 | One small deployable, unified storage pool, low operations burden |
| Privacy-conscious researcher / creator | PDF/documents and sensitive archives without third-party dependency | Secure shares, previews, Organization collaboration; AI Workspace only later |
| Nontechnical family/team member | Desktop/mobile browsers, simple storage tasks | Intuitive upload, download, sharing, preview, responsive interface |

Major pain points: high idle footprint and complex self-hosted maintenance; vendor lock-in and recurring cloud-storage costs; difficulty pooling local and cloud capacity. No claim about competitors' actual runtime usage is a verified benchmark for CubyCloud.

### 1.3 Targets (not measured outcomes)

| KPI | Target / assumption |
| --- | --- |
| Baseline server | Intel Celeron 2-core, 2 GB RAM, ~10 GB OS storage |
| Idle memory | **< 200 MB** |
| Peak active transfer memory | **< 600 MB** |
| Idle CPU | **< 10%** on baseline device |
| LAN metadata API latency | **< 50 ms** target |
| LAN page navigation | **< 1.0 second** |
| First Contentful Paint | **< 800 ms** target |
| Production image | **< 150 MB** target |
| First-run web onboarding | **< 3 minutes** target |
| Runtime distribution | One container or one executable, no required external services |

Use real profiling on the target hardware before claiming these targets are achieved. Cloud presigned transfer should keep object payload off the Go server data path where supported; “zero CPU/bandwidth” is an architectural goal, not a literal promise that all control-plane work costs nothing.

---

## 2. Product scope and non-goals

### 2.1 In scope for v1.0

- Initial web setup: Super Admin, local node scan, node capacity cap, placement strategy, optional cloud credentials, Emergency Recovery Key.
- Virtual filesystem combining local SSD/HDD and S3/R2; backend storage driver, physical capacity rules, direct cloud transfers where available.
- My Storage personal explorer, fixed shared Organization tree, Shared With Me, Home, Starred, Trash, Stats, and Super Admin pages.
- File/folder CRUD, upload, direct download, previews (image/video/audio/text/code/PDF), recursive folder operations, conflict-safe naming, share links and internal Viewer/Editor permissions.
- Persistent/recoverable upload queue for supported transports, HTTP Range on supported regular file downloads, direct S3 downloads, streaming ZIP.
- Per-user My Storage quota, separate node-capacity enforcement; soft-delete/restore/purge; in-app toasts/status/notifications.
- Server health and offline-file metadata availability, account-scoped English/Indonesian UI, global filename/folder search, profile updates and password/session security.
- Core archive/download ZIP without a separately installed add-on; SQLite-backed background jobs.

### 2.2 Explicitly deferred

AI Workspace, MCP/RAG/vector database/Ollama integration; file versioning/history; PWA/offline app cache; native desktop/mobile background sync; live office collaboration (OnlyOffice/Collabora); SMTP/email notifications and verification; server-side heavyweight video transcoding; SaaS registration/billing/payment gateways. Do not silently include these in MVP tasks.

**Historical discrepancy to resolve:** The original v1.0 PRD explicitly deferred **Grid View** and specified List View only, while the latest frontend SITEMAP/DESIGN describe an existing List/Grid toggle. Preserve the existing UI without removing it during unrelated work. A product decision is needed to reconcile the out-of-scope line and the established frontend contract; see **Open alignment checks**.

### 2.3 Global boundaries

- All pages and APIs enforce backend authorization; a hidden UI element is not an access control.
- **Super Admin:** global administration; file physical-node badges/identifiers in authorized contexts; always unlimited personal quota.
- **Regular User:** personal files only unless Organization or explicit sharing grants access; never expose physical storage node identities.
- **Organization:** fixed shared tree, all authenticated users have full access; no user quota (physical capacity still applies).
- **Shared:** Viewer/Editor permissions on granted files and descendants; owner pays storage quota.
- File moves and copies crossing owners/scopes must enforce destination permissions and accounting.

---

## 3. Cross-cutting product contracts

### 3.1 Deployment and runtime

Single image/binary; Go serves embedded static SvelteKit SPA, HTTP API, and background workers. SQLite WAL and internal job queue eliminate required Redis, RabbitMQ, Kafka, PostgreSQL, Node.js runtime, or MinIO. Optional S3-compatible providers are external storage **options**, never required local services.

### 3.2 Storage placement, availability, and privacy

- Pools multiple local nodes and S3-compatible cloud nodes behind a virtual file tree; available placement strategies: **`FILL_FIRST`**, **`ROUND_ROBIN`**, **`MOST_FREE_SPACE`**.
- Operators configure physical node capacity caps separately from logical per-user quotas.
- Periodically inspect local free capacity (e.g. `unix.Statfs`). Nodes with **< 5% physical capacity remaining** are marked `READ_ONLY`; new writes select another suitable node where possible. When no suitable node exists, report **physical storage full**, never user quota full.
- Public unauthenticated **`GET /health`** probes every configured node, returns HTTP **200** only if all are accessible, and **503** if any node is inaccessible. Public payload must not expose node paths, names, identifiers, or credentials.
- My Storage and Organization file records expose boolean `storage_available`. An offline physical node does **not** make file metadata disappear; render file/folder metadata and leave folders navigable. Disable operations requiring inaccessible bytes and explain via localized hover/click popover without leaking node identity. Physical node badges are for Super Admin only.

### 3.3 User interface language and global behavior

- Support **Indonesian** and **English**. Browser locales beginning with `id` default to Indonesian; all other browser locales default to English.
- Manual language choice is stored **in browser `localStorage` keyed by authenticated account ID**, not persisted server-side. UI titles, labels, errors, placeholders, accessible names, confirmations, and notifications follow the selected language. Do not translate identifiers, technical terms, or user content inappropriately.
- The global language control uses a Globe icon and shadcn-svelte Select displaying `id`/`en`.
- Header search and upload are available throughout authenticated drive pages; see §4.4 for behaviors and exceptions.

---

## 4. Functional requirements by module

### 4.1 Onboarding and emergency recovery (`/setup`, `/recovery`)

**Precondition:** When `is_setup_completed` is false, direct the app to the first-boot setup; completed setup proceeds to `/drive/home`. Avoid creating a second Super Admin through repeated onboarding.

1. Create initial Super Admin with username, unique email, password (**minimum 8 characters**).
2. Discover local nodes under `/app/data/storage/` by default; optionally discover Linux devices for mounting/registration. Show total/free capacity; let admin choose enabled nodes and each node capacity cap.
3. Choose placement strategy `FILL_FIRST`, `ROUND_ROBIN`, or `MOST_FREE_SPACE`.
4. Optionally register Cloudflare R2, AWS S3, or S3-compatible storage using endpoint, bucket, access key, and secret key; never expose stored secrets to regular users.
5. Generate in the browser a cryptographically secure **30-character uppercase alphanumeric Emergency Recovery Key**, formatted **six groups of five** (`XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX`). Offer copy/download and explicit acknowledgment. Store only SHA-256 of the canonical value without separators, not plaintext.
6. Emergency reset is **single-use and atomic**: verify current key, reset Super Admin password, consume old key and return replacement `new_recovery_key`. Require saving the replacement and login again.
7. Migration-only backend compatibility accepts a legacy 64-hex-character key solely for an account with a legacy bcrypt recovery hash. Successful use rotates to the current key/hash scheme. The frontend UI does **not** generate or accept the legacy format.

### 4.2 Virtual storage and Super Admin settings (`/admin/settings`, `/admin/storage`)

- Storage driver and VFS abstract physical nodes from regular users. Super Admin can inspect nodes, node names, capacity and actual usage, with local/cloud storage badges as specified by UI contracts.
- If local free capacity drops below **5%**, mark node read-only and fail over new uploads to a healthy writable node where possible. If no node can accept the write, return a distinct **physical pool full** message.
- Allow **Rescan Storage Nodes** without rerunning onboarding. Handle `Detach Node` by moving it to read-only and safely migrating its physical files to another node through background jobs.
- Admin configures **Default User Quota** in GiB (binary 1024^3) or Unlimited, applying only to **new users**; existing users retain their effective settings unless explicitly updated.
- **Audit-log retention:** default **90 days**, selectable **7/14/30/60/90** days, cleaned with `AUDIT_CLEANUP_JOB`.
- **Background worker concurrency:** configurable, default **2**. Queued heavy operations remain `PENDING` until worker capacity is available.
- `/admin/storage` shows node count/cumulative declared capacity, used physical space and available capacity (calculated from configured node capacity limit minus usage, with physical free disk acting as a safety tripwire), and users ranked by logical usage with pagination. All capacity and size inputs and displays across the system use binary GiB/KiB/MiB/TiB (1024^n). The backend is authoritative for all numbers.
- Provide **Download DB Backup** action for a consistent `cubycloud.db` SQLite snapshot. This is **database-only**, not a verified full-file backup; full disaster recovery also requires storage blobs and relevant secure configuration/credentials. Do not label DB backup as complete system backup.

### 4.3 Drive Home (`/drive/home`)

- Default authenticated landing after successful login/setup.
- **Quick Access:** last **5** opened files/folders by current account, sorted newest-first, persisted per account across devices as specified in the product contract.
- **Recent Activities:** latest **5 distinct** items uploaded or changed by current account (including rename/move) in My Storage or Organization, deduplicated by item.
- My Storage quota summary matches the sidebar; quick upload drawer writes to My Storage and enforces logical user quota and physical capacity.

### 4.4 Explorer, search, transfer, preview, and global upload

**My Storage route:** `/drive/my-storage` with `?folder=...&preview=...`. Organization and Shared use their own page/permission contracts. The existing frontend list/grid UI must not be removed without resolving the historical scope contradiction in §2.2.

**File operations and conflicts**

- File/folder picker uploads, nested folder uploads, drag/drop, downloads, rename, move, zero-byte-reference duplicate, star/unstar, share, recursive delete-to-Trash; handle partially failed folder uploads per item. No arbitrary application-wide file-size ceiling: practical limits come from available node capacity and supported transport capabilities.
- A name conflicts only with a sibling at the destination path. Upload and duplicate automatically choose the next available numbered suffix (`Report (1).pdf`). Rename/move offer a suffix or cancel, **never overwrite** or silently merge folders. Different folders may contain identical names.
- Moving or copying items into a different owner/scope must recheck permissions and quota; recursive operations account for descendant files.

**Global filename/folder search**

- Server-side, case-insensitive search across **own My Storage**, **Organization**, and authorized **Shared With Me subtrees**, including nested shared folders. Search **names**, not file contents or OCR. Exclude Trash and unrelated public share links.
- Header uses debounced preview (up to **5** results), keyboard navigation, and a **View all results** action; Enter navigates to `/drive/search?q=...&page=...` with paginated results (**25 per page**).
- Results show name, file/folder type, origin, accessible location, size and modified time. Search and result navigation enforce backend permissions; never expose private ancestors above a shared root or leak inaccessible metadata when permissions are revoked.
- Quick category filters cover image (PNG/JPG/WebP/GIF/SVG), video (MP4/WebM), audio (MP3/WAV/OGG), document (PDF/TXT/MD/CSV/DOCX), and archive (ZIP/RAR/7Z) types.
- Empty/invalid query, special wildcard characters, duplicates at different paths, deleted/moved files, stale queries, and pagination must produce safe, understandable behavior.

**Persistent uploads and resume**

- My Storage and Organization use the supported resumable upload paths. **Local:** TUS (including small-file path as implemented). **S3/R2:** direct presigned upload, single PUT for files **under 100 MiB**, multipart for files **from 100 MiB**, default **16 MiB parts**, at most **3 parts concurrently**.
- Backend is authoritative for TUS offsets and multipart part reconciliation (`ListParts`); presigned part URLs expire and must be regenerated as needed.
- Persistent transfer metadata/queue belongs in **IndexedDB**. Do not persist raw file bytes or cloud secrets. Server resumable sessions expire after **24 hours**; cleanup must not delete valid committed data.
- Following page reload, browser close, or server restart, allow explicit recovery using **the same reselected file**, verified with streaming/bounded-memory SHA-256. Incompatible, missing, or expired session/file requires clear user action to restart; no silent duplicate upload.
- Queue states: `Uploading`, `Retrying`, `Paused`, `Waiting for File`, `Verifying`, `Committing`, `Completed`, `Failed`, `Expired`. Support pause/resume/cancel, reconnect retry and cross-tab single-owner coordination; keep progress honest until server commit.
- Show an expandable upload progress surface near the upper-right header, retained until dismissed; show completion toast in the normal location. Support item-level failure reporting and quota/capacity errors.

**Global header upload destination**

- Normally upload to **My Storage root**, regardless of current My Storage/Organization folder or other drive page. Use the unified transfer infrastructure rather than a separate direct-upload path.
- **Exception: `/drive/shared` while viewing an actual folder with effective `Editor` permission.** Upload to that active shared folder, subject to backend ownership, quota, and permission enforcement.
- On Shared root/list or insufficient permission (including `Viewer`), display a clear **denial dialog**; do **not** silently fall back to My Storage. If the shared upload transport is not resumable, do not claim resume is supported.

**Preview and download**

- Preview authorized image (PNG/JPG/WebP/GIF/SVG), video (MP4/WebM with Vidstack), audio (MP3/WAV/OGG), text/code/Markdown/JSON/CSV/log (as text, PrismJS where supported), and PDF (PDF.js), in My Storage, Organization, Shared, and public/restricted share according to access rights.
- Page-level `?preview=file_id` opens URL-driven lightbox; download endpoint `?preview=1` opts into safe inline MIME and `Content-Disposition: inline`; standard downloads remain attachment.
- Regular local downloads in My Storage and Organization support **HTTP Range and HEAD**: proper `200`, `206`, `416`, `Content-Range`, `Content-Length`, `Accept-Ranges`, `ETag`, `If-Range`; keep browser-native downloads. S3/R2 regular downloads use direct presigned GET. ZIP remains streaming and **is not resumable**.

### 4.5 Trash and lifecycle (`/drive/trash`)

- Delete marks item/folder subtree `is_trashed=true`, retaining physical bytes and personal quota until permanent purge.
- Automatic purge after **30 days** using a daily background worker (original schedule: **00:00**); reduce usage and reclaim physical references safely.
- Manual restore or permanent purge are supported. If original parent was permanently removed, require confirmation before restoring to **My Storage root**.
- Prevent restore conflicts, incorrect quota release, and orphaned physical blobs; return clear results on partial failures.

### 4.6 Starred (`/drive/starred`)

- Star/unstar is **per-user**, aggregates My Storage, Organization and Shared where accessible. Show starred items with preview/share actions subject to the source's permissions; unstar is immediately available.

### 4.7 Storage Stats (`/drive/storage-stats`)

- Show **Used / Limit GB** for limited accounts; only **Used: X GB** for Unlimited, without a fictional limit or progress cap. Super Admin always Unlimited.
- Category usage chart: Video, Gambar, Dokumen, Arsip, Trash. List **10 largest** owned files (My Storage + Trash; exclude Organization) and offer cleanup advisor and `/drive/trash` shortcut.

### 4.8 Internal sharing and Organization

- User-to-user internal share grants **Viewer** (read/preview/download) or **Editor** (read/download/add/edit/delete within shared tree). Permissions propagate to accessible descendants and must be enforced on API and navigation, not just hidden UI.
- Shared storage consumption remains with **owner**; recipients do not accrue extra personal usage by accessing shared files. Write and move operations require effective permission and correct quota accounting.
- **Organization** is a **fixed, distinct shared virtual tree**, not a My Storage folder that an Admin marks as an Organization folder. All authenticated users have full Read/Write/Download/Rename/Delete access; data does **not** count toward any personal quota. Physical capacity remains enforced.
- Moving Organization → My Storage must enforce the destination user's personal quota before completing.

### 4.9 User management (`/admin/users`)

- **Super Admin only.** View username, email, role, effective quota, usage and status. Create regular users with default quota or explicitly specified override; user creation sets default password `12345678` without password input in the form.
- Edit personal quota to a GB value or Unlimited and support **Reset to Default** as product-defined. Super Admin's personal quota remains Unlimited.
- Reset user password to default `12345678` via confirmation modal dialog. Super Admin cannot reset their own password (enforced on UI and API).
- Separate user administration from self-service Account Profile, and enforce ownership/roles server-side.

### 4.10 Logical quota engine

- Personal usage includes owned non-Organization items **including Trash until purge**. Owner-preserving zero-byte duplicate does not increase physical bytes; copying across owners increases destination owner's **logical** usage as applicable.
- Effective quota: **Super Admin Unlimited**; otherwise a configured personal override (GB or Unlimited), else system default quota.
- For My Storage uploads/imports/move-ins/copy-to-new-owner: at **≥80%** of limit show a warning; if proposed usage exceeds quota reject with **`USER_QUOTA_EXCEEDED`** and a localized message equivalent to *“Kuota My Storage penuh. Hubungi admin.”*
- Organization writes ignore personal quota but enforce node capacity. Distinguish physical-pool-full errors from logical-quota errors, including in UI and HTTP/API responses.
- Do not allow concurrent writes or unfinished uploads to bypass quota enforcement.

### 4.11 Share links (`/s/:token`)

- Generate unique unguessable share-link tokens. **Public** links can be opened by holders; **Restricted** links require an authenticated registered user.
- Support optional **bcrypt-hashed password protection** and **expiration date**; expired/invalid links must deny access. Password challenge precedes file disclosure.
- No link download-count limit in v1.0. Authorized link viewers can preview/download and, for permitted folders/items, stream ZIP; backend validates tokens and restrictions on every sensitive action.

### 4.12 Core archive and background jobs

- **Download as ZIP** streams selected files/folders as one `.zip`; **Compress to ZIP** stores selected content as an archive. Both are **core**, not an opt-in add-on. `/admin/addons` is removed.
- Background operations, including archive compression, image conversion where supported, and maintenance tasks, use SQLite transactional jobs and a bounded Go worker pool (default concurrency **2**). States include `PENDING`, `PROCESSING`, `COMPLETED`, `FAILED`.
- Job failures (e.g. disk full, corrupt input) must not leave partial committed output; surface user-facing error/notification and appropriate Admin audit diagnostics; support safe recovery/retry.

### 4.13 Account Profile and session security (`/drive/profile/:userId`)

- Show **display name, email, username, role, effective quota, actual usage** from the backend. Username, role, and quota are read-only here.
- Users may **read and edit only their own** display name/email. **Only Super Admin** may read another user's profile, and that view is **read-only**. Regular users may not view or modify another profile.
- Email changes require **current password**, valid normalized address and **case-insensitive uniqueness**. Reject invalid or duplicate emails atomically; conflicting concurrent edits must not overwrite newer data and should require refresh/retry.
- Password change requires current password, valid new password (**at least 8 characters**) and matching confirmation. Change revokes **all existing sessions**, including current, and forces re-login, using account token/session version invalidation or equivalent server-enforced mechanism.
- Do not implement avatar uploading, username changes, account deletion, or SMTP validation as part of this MVP contract.

---

## 5. Non-functional and security requirements

### 5.1 Performance and browser support

The thresholds in §1.3 are **benchmarks to test**, not assertions of measured performance. Minimize memory pressure, CPU, I/O and concurrent heavy tasks on Celeron/2 GB systems.

Target responsive compatibility: current Chrome/Firefox/Edge on Windows, Linux, macOS; Safari macOS 14+/Safari 17+; WebKit browsers on iOS/iPadOS; Chrome/Firefox mobile on Android (roughly the latest two-year browser window stated in the original requirements). Use semantic HTML5, standard web APIs, CSS Grid/Flexbox and avoid large vendor-specific polyfills.

### 5.2 Authentication, secrets, and authorization

- JWT tokens signed with **HS256** in **HttpOnly cookie** (`SameSite=Lax`; `Secure` whenever deployed through HTTPS, including TLS-terminating proxies). No separate refresh-token service is required in v1.0.
- **Original PRD specified a 24-hour token lifetime**; the backend at the preceding code audit used a seven-day cookie/JWT. This discrepancy is explicitly unresolved; do not silently change session lifetime as part of unrelated documentation work. All password changes must still revoke existing sessions regardless of token expiry.
- Passwords for users and protected share links use **bcrypt (cost 10)** per original requirement; ensure safe verification, throttling, and error handling.
- Emergency 6×5 Recovery Key uses **SHA-256 of canonical value**, single-use rotation; legacy bcrypt-hashed 64-hex key compatibility is backend migration-only, never UI-generated.
- No automatic application-level encryption of physical uploaded file contents; rely on OS/container volume permissions and operator controls. Storage credentials and other application secrets require secure handling and must not be exposed via API, log, or public health response.
- Apply backend authorization to profile routes, private file identifiers, search results, nested shares, moves, downloads, metadata and cross-user actions. Consider CSRF, IDOR, rate limiting, path traversal, leaked tokens, and race conditions.

### 5.3 Durability and failure recovery

- SQLite **WAL** with appropriate transaction durability and integrity checks; original configuration requirement: `PRAGMA journal_mode=WAL` and `synchronous=NORMAL`. WAL improves crash recovery but does not replace backups or make every failure mode impossible.
- Startup orphan-file audit reconciles `physical_blobs` and storage to remove genuinely abandoned upload debris **without deleting valid resumable sessions or referenced data**.
- Interrupted jobs recover safely; the original design marks unfinished jobs `FAILED` at restart and permits explicit safe retry. Respect atomic claims, idempotency, clean rollback, and consistent VFS metadata/physical bytes.
- Interrupted upload and multipart sessions follow the recovery rules in §4.4; never report successful commit or resume support without backend evidence.

### 5.4 Deployment, backups, and maintainability

- Multi-stage build: Bun builds static frontend, Go builds backend with embedded assets, small final image (Alpine or equivalent), aiming for **<150 MB**.
- No mandatory runtime services beyond the packaged backend/SQLite and selected storage backends. Version/build targets must be verified against the actual `go.mod`, Dockerfile and lockfiles rather than outdated example versions.
- Admin can download a **database snapshot** from the GUI. It is **not** a full disaster-recovery archive: stored files, secret material and deployment configuration must also be backed up and restoration tested before claiming complete recovery.
- Keep operational procedures documented without mixing implementation manuals into this PRD; `docs/architecture.md`, `README.md`, `Makefile`, and deployment files own technical details.

---

## 6. Technology contract and ownership

| Layer | Selected technology / rule |
| --- | --- |
| Backend | Go, Echo v4, SQL-first Bun ORM (`uptrace/bun`), `database/sql`, SQLite pure-Go driver (`glebarez/go-sqlite`) |
| Auth | `golang-jwt/jwt/v5`, `golang.org/x/crypto` bcrypt, HTTP-only cookie |
| Object storage | AWS SDK for Go v2, S3-compatible endpoints including R2, presigned multipart/GET as applicable |
| Data/jobs | SQLite WAL, transactional jobs, limited goroutine workers; no required Redis or message broker |
| Frontend | SvelteKit SPA, `adapter-static`, Svelte 5 Runes, TypeScript, Tailwind CSS v4 |
| Components | shadcn-svelte / bits-ui, `@lucide/svelte`, Montserrat; visual contracts in DESIGN |
| Tooling | Bun for frontend build/package management, Go toolchain for backend, Makefile and Docker multi-stage build |
| Distribution | One Go executable / one Docker container with embedded SPA |

Version numbers in the earlier PRD (such as Go 1.22+, framework minimums, or `latest`) are **historical baseline examples**. The version actually required by `go.mod`, lockfiles, source imports and production build must be verified at implementation time. Updating tooling versions is not permission to change established product behavior.

### Source-of-truth ownership

- This PRD: product requirements, permissions, quotas, UX outcomes, acceptance conditions.
- `docs/architecture.md`: system decomposition, storage/runtime implementation, infrastructure and data flows.
- Frontend `SITEMAP.md`: route tree, per-page states and navigation.
- Frontend `DESIGN.md`: colors, typography, component shape and interaction visuals.
- Root/scoped `AGENTS.md`: AI engineering workflow, code conventions and context-loading rules.

---

## 7. Open alignment checks (do not silently resolve)

These items existed as differences among dated requirements, UI documentation, and implementation at the audit point. They **do not supersede** the active product contract without approval.

1. **List-only vs List/Grid:** Original PRD §3.2/§4.4 deferred grid; recently revised frontend SITEMAP/DESIGN describe a working toggle. Preserve existing behavior; decide and align documents explicitly.
2. **JWT lifetime:** Original PRD says 24 hours, earlier backend code audit reported 7 days. Confirm desired security/session lifetime and update implementation + documentation together.
3. **Scope vs delivered state:** Features and performance numbers listed here are requirements/targets. Do not mark shipping, benchmarks, security hardening, or backup/recovery as verified without real integration tests and measurements.
4. **Historical release schedule:** The original 1 October 2026 date has passed. It is retained for provenance only and does not serve as an active deadline or release-status statement.

When a mismatch is material to an assigned task, identify it, preserve unrelated behavior, and seek a decision before altering the affected product contract. Avoid blocking unrelated safe work.
