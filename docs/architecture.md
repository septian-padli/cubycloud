# ARCHITECTURE.md — CubyCloud Backend Architecture & System Blueprint

**Document Control**

- **Project Name:** CubyCloud (v1.0.0)
- **Author:** Septian Padli
- **Target Environment:** Single Docker Container / Executable Binary (Intel Celeron 2-Core, 2 GB RAM, SQLite WAL Mode)
- **Design Pattern:** Domain/Module-Driven Clean Architecture
- **Product contracts (source of truth for UX & business rules):**
  - [`docs/prd.md`](./prd.md) — scope, modules, quota rules
  - [`apps/frontend/SITEMAP.md`](../apps/frontend/SITEMAP.md) — routes & UI contracts
  - [`apps/frontend/DESIGN.md`](../apps/frontend/DESIGN.md) — Flat Earthy Design System
- **Agent guidance (implementation hints, not product spec):**
  - [`AGENTS.md`](../AGENTS.md) — global constraints
  - [`apps/backend/AGENTS.md`](../apps/backend/AGENTS.md) — Go module conventions
  - [`apps/frontend/AGENTS.md`](../apps/frontend/AGENTS.md) — SvelteKit & UI conventions

---

## 1. System Architecture Principles

CubyCloud dikembangkan dengan prinsip utama: **High Performance, Zero External Runtime Dependencies, Ultra-Low Resource Usage (Idle RAM < 200 MB), dan Single Executable Distribution.**

```text
+-------------------------------------------------------------------------------+
|                             CUBYCLOUD CONTAINER                               |
|                                                                               |
|  +-------------------------------------------------------------------------+  |
|  |  apps/frontend (SvelteKit build/) ──sync──► apps/backend (go:embed)     |  |
|  +-------------------------------------------------------------------------+  |
|                                     | HTTP / REST API & Server-Sent Events    |
|  +-------------------------------------------------------------------------+  |
|  |     apps/backend — Echo v4.12+ + In-Memory Rate Limiter + Middleware    |  |
|  +-------------------------------------------------------------------------+  |
|                                     |                                         |
|  +-------------------------------------------------------------------------+  |
|  |       apps/backend/internal/modules — Domain / Module Application       |  |
|  |  [Auth]      [VFS / Drive]      [Share]      [Admin]      [Jobs/Queue]  |  |
|  +-------------------------------------------------------------------------+  |
|          |                            |                        |              |
|  +---------------+            +------------------+     +-------------------+  |
|  | StorageDriver |            |  Ent ORM Engine  |     | Goroutine Workers |  |
|  |  (Interface)  |            +------------------+     +-------------------+  |
|  +---------------+                     |                        |             |
|    |           |               +-----------------------------------+          |
|  [Disk]    [S3/R2]             |   SQLite Database Engine (WAL)    |          |
| (Local)   (Presigned)          |   (MaxOpenConns=1, timeout=5000)  |          |
|                                +-----------------------------------+          |
+-------------------------------------------------------------------------------+

```

### Module mapping (product → backend)

| Product surface (PRD / SITEMAP)                                                                            | Backend home                                        |
| ---------------------------------------------------------------------------------------------------------- | --------------------------------------------------- |
| `/drive/home`, `/drive/my-storage`, `/drive/organization`, `/drive/starred`, `/drive/storage-stats`, Trash | **VFS** query/views + write paths                   |
| Per-user My Storage quota                                                                                  | **VFS** write pre-check + **Admin** policy/override |
| `/admin/settings`, `/admin/users`, nodes, audit                                                            | **Admin**                                           |
| Share links `/s/:token`, user-to-user share                                                                | **Share**                                           |
| Login, recovery, session                                                                                   | **Auth**                                            |
| ZIP, cleanup, deferred work                                                                                | **Jobs**                                            |

UI tokens and layout are **not** specified here — see `DESIGN.md`. Business rules for quota scope, Organization exclusion, and error copy live in **PRD §4.9–4.10** and **SITEMAP §5**.

### Core Architectural Guarantees:

1. **Domain/Module-Driven Clean Architecture:** Setiap domain/modul berdiri mandiri mengkapsulasi logika handler, usecase, dan repository-nya.
2. **Virtual File System (VFS) Abstraction:** Pengguna hanya berinteraksi dengan struktur direktori virtual (Adjacency List). Drive views (Home, Org, Starred, Stats) adalah _query layer_ di atas VFS — bukan modul Go terpisah.
3. **Pure Go & CGO-Free Database:** Memakai driver `glebarez/go-sqlite` (tanpa dependensi C/GCC) untuk kompilasi lintas platform (_cross-compilation_).
4. **Single-Binary Bundle:** Build SvelteKit ke `apps/frontend/build/`, disinkronkan ke paket embed backend, lalu di-bundle ke biner Go via `go:embed` (orchestrasi oleh root `Makefile`).
5. **No Redis/Message Brokers:** Semua kebutuhan _queue_, pub/sub (SSE trigger), dan persistensi ditangani sepenuhnya oleh Go Memory & SQLite.

---

## 2. Directory & Project Structure

CubyCloud adalah **monorepo**: frontend (`apps/frontend/`), backend Go (`apps/backend/`), dan dokumen produk (`docs/`) hidup dalam satu repository. Build & deploy diorchestrasi dari **root** (`Makefile`, `Dockerfile`).

### 2.1. Monorepo layout (ringkas)

```text
cubycloud/
├── AGENTS.md                       # Global AI agent constraints
├── Makefile                        # Orchestrasi: FE build → sync embed → go build
├── Dockerfile                      # Multi-stage image (< 150 MB target)
├── docs/
│   ├── prd.md
│   └── architecture.md             # Dokumen ini
├── apps/
│   ├── frontend/                   # SvelteKit SPA — DESIGN.md, SITEMAP.md, AGENTS.md
│   │   └── build/                  # adapter-static output (sumber artifact embed)
│   └── backend/                    # Go server — AGENTS.md
│       ├── cmd/server/main.go
│       ├── ent/schema/
│       ├── migrations/
│       ├── internal/               # config, database, middleware, storage, sse, modules
│       └── web/                    # go:embed package (dist/ diisi saat build)
└── README.md
```

### 2.2. Backend detail (`apps/backend/`)

Backend Go memakai pendekatan **Domain/Module-Driven**. Detail konvensi implementasi: [`apps/backend/AGENTS.md`](../apps/backend/AGENTS.md).

```text
apps/backend/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point, DI, system boot
├── ent/
│   └── schema/                     # Ent ORM schema definitions
│       ├── user.go, virtualnode.go, physicalblob.go, storagenode.go
│       ├── sharelink.go, job.go, setting.go, auditlog.go
├── migrations/                     # Versioned SQL migrations
├── internal/
│   ├── config/                     # Hybrid config (env + DB settings loader)
│   ├── database/                   # SQLite WAL connection & PRAGMA setup
│   ├── middleware/                 # JWT cookie guard, rate limit, setup guard
│   ├── storage/                    # StorageDriver (LocalDisk, S3Presigner)
│   ├── sse/                        # SSE connection manager
│   └── modules/                    # Core business domains
│       ├── auth/                   # Login, recovery, session invalidation
│       ├── vfs/                    # Explorer, copy, move, trash, TUS, quota pre-check
│       ├── share/                  # Public links, password challenge
│       ├── admin/                  # Nodes, settings, users & quotas, audit logs
│       └── jobs/                   # Transactional queue, workers, cron cleanup
└── web/
    ├── dist/                       # Gitignored — disinkronkan dari apps/frontend/build/
    └── embed.go                    # go:embed dist/*
```

### 2.3. Frontend embed pipeline

Artifact SPA **canonical** ada di `apps/frontend/build/` (output `adapter-static`). Go `go:embed` tidak boleh mengandalkan path di luar modul backend saat compile; root **`Makefile`** menjalankan langkah berikut sebelum `go build`:

1. `cd apps/frontend && bun run build` → menghasilkan `apps/frontend/build/`
2. Sinkronkan `apps/frontend/build/` → `apps/backend/web/dist/` (copy/rsync; folder `dist/` gitignored)
3. `cd apps/backend && go build ./cmd/server` → biner tunggal dengan SPA embedded

Dockerfile multi-stage di root mengikuti urutan yang sama. Sumber UI tetap `apps/frontend/`; `apps/backend/web/dist/` hanya staging compile-time, bukan source of truth.

### 2.4. Runtime data paths (container)

Data persisten di container menggunakan prefix **`/app/data/`** (volume mount):

| Path | Isi |
| ---- | --- |
| `/app/data/db/` | SQLite database (WAL) |
| `/app/data/storage/` | Blob fisik node lokal |
| `/app/data/temp/tus/` | Chunk TUS sementara |

Boot env `DATA_DIR_PATH` dapat override root data dir; default produksi tetap `/app/data/`.

---

## 3. Configuration & Database Migrations

### 3.1. Hybrid Configuration Strategy

CubyCloud menggunakan pendekatan hibrida untuk menjaga aplikasi tetap _Zero-CLI_ namun aman:

- **System-Level (.env / OS Env):** Hanya untuk nilai _booting_ statis: `PORT`, `DATA_DIR_PATH`, dan `JWT_SECRET`.
- **Application-Level (SQLite `settings` table):** Disetel via Web UI Admin, mencakup: `worker_concurrency_limit`, `is_setup_completed`, `jwt_secret_version`, `audit_log_retention_days`, `default_user_quota` (GB atau Unlimited untuk user baru), dan status _Add-ons_.

Per-user quota override disimpan di entitas `users` (bukan hanya `settings`) — lihat PRD §4.9.

### 3.2. Versioned Database Migration

Skema database dikelola menggunakan **Versioned SQL Migrations** (Tabel `schema_migrations`). Saat server _booting_, Go membaca folder `migrations/` dan mengeksekusi file `.sql` yang belum dijalankan. Hal ini menjamin integritas data dan kemudahan _rollback_ jika terjadi pembaruan versi CubyCloud (v1.0 → v1.1).

---

## 4. Database Schema & Data Models (SQLite WAL)

**Kunci Konfigurasi SQLite (Anti-Locking):**
Mengingat SQLite WAL Mode memiliki limitasi penulisan (_single writer_), koneksi Go `database/sql` wajib di-set agar tidak terjadi _race condition_:

```go
db.SetMaxOpenConns(1) // Menjamin 1 penulisan pada satu waktu
db.Exec("PRAGMA busy_timeout = 5000;") // Menunggu antrean tulis maks 5 detik

```

### Primary Key Strategy:

- **Database PK (`id`):** Menggunakan **ULID** (26-karakter, _Time-Sorted_) agar pohon indeks (B-Tree) SQLite tidak mengalami fragmentasi (_Page Splitting_).
- **Public Tokens:** Menggunakan **NanoID** (8–12 karakter acak) khusus untuk URL `share_links.token`.

### Entitas Inti:

1. **`virtual_nodes` (Adjacency List):** Tabel hierarki folder/file. Memiliki `parent_id` (merujuk ke folder induk) dan `physical_blob_id` (merujuk ke wujud fisik berkas). Drive views (starred, org tree, trash) adalah filter/flag di atas model ini.
2. **`physical_blobs` (Zero-Byte Copy Core):** Memiliki kolom `ref_count` (Integer). Saat file diduplikasi secara virtual, `ref_count` bertambah +1. Berkas fisik di SSD/S3 HANYA boleh dihapus jika `ref_count == 0`.
3. **`users`:** Identitas + role; menyimpan override kuota My Storage (GB / Unlimited / inherit default). Agregasi usage dihitung dari file milik user di luar Organization tree (termasuk Trash sampai purge) — aturan lengkap di PRD §4.10.
4. **`audit_logs`:** Mencatat seluruh aksi modifikasi (`action`: CREATE, DELETE, SHARE, LOGIN) dengan kolom `ip_address`, `user_id`, dan `details` (JSON).

DDL kolom kuota dan flag Organization/starred tidak diulang di sini; kontrak produk di PRD, kontrak endpoint di `API_CONTRACT.md` (berikutnya).

---

## 5. Storage Engine & Resumable Upload

Seluruh I/O fisik disk diabstraksikan melalui antarmuka `StorageDriver`:

### Upload Workflow:

- **Local Disk (TUS Protocol):** Frontend menggunakan TUS (chunked, resumable). Potongan file (_chunks_) disimpan di `/app/data/temp/tus/`. Jika transfer selesai, _chunk_ digabung dan dipindah ke `storage_nodes` menggunakan strategi:
- **FILL_FIRST:** Memenuhi node-1 hingga sisa kapasitas < 5% (Safety Margin), lalu pindah ke node-2.
- **MOST_FREE_SPACE:** Memilih node yang kapasitas sisa byte-nya paling besar.
- **ROUND_ROBIN:** Membagi alokasi file secara bergiliran ke seluruh disk aktif.

- **Cloud (Zero-Egress Direct Transfer):** Frontend meminta _Presigned URL_ ke Backend Go → File di-PUT langsung dari browser ke R2/S3 → Frontend memanggil `/api/v1/drive/sync-cloud` untuk menyimpan metadata ke SQLite.

### Capacity & quota gates (write path)

Setiap write ke My Storage (upload, copy, move-in) melewati dua gerbang berurutan di **VFS**:

1. **User quota pre-check** (My Storage only; Organization di-skip) → gagal: `USER_QUOTA_EXCEEDED`.
2. **Node / pool capacity** (5% safety + placement strategy) → gagal: pesan pool penuh (_bukan_ kode user-quota).

Detail pesan UI dan widget: SITEMAP §5. Policy default & override: PRD §4.9–4.10.

---

## 6. Real-Time Sync & Background Jobs

### 6.1. Transactional Job Queue (SQLite)

Alih-alih Redis, CubyCloud menggunakan **SQLite-as-a-Queue** dengan mekanisme _Lock-Free Atomic Fetching_ (`UPDATE ... RETURNING`):

```sql
UPDATE jobs SET status = 'PROCESSING', updated_at = CURRENT_TIMESTAMP
WHERE id = (SELECT id FROM jobs WHERE status = 'PENDING' ORDER BY created_at ASC LIMIT 1)
RETURNING *;

```

Batas konkurensi (Default: 2 Goroutine Workers) dikontrol lewat tabel `settings`.

### 6.2. Cron-Like Cleanup Jobs

Worker pool juga mengeksekusi _maintenance_ otomatis secara periodik:

1. **`TUS_CLEANUP_JOB`:** Menghapus sisa _chunk_ unggahan TUS di folder `temp/` yang terabaikan lebih dari 24 jam.
2. **`AUDIT_CLEANUP_JOB`:** Memindai tabel `audit_logs` setiap malam dan melakukan `DELETE` pada log yang lebih tua dari batas retensi (Default: 90 hari. Konfigurasi Admin: 7, 14, 30, 60, 90 hari).
3. **`TRASH_PURGE_JOB`:** Purge permanen item Trash > 30 hari (mengurangi usage kuota user setelah blob `ref_count` turun ke 0).

### 6.3. Server-Sent Events (SSE)

Untuk menghindari lonjakan RAM dari _WebSockets_ dan polusi CPU dari _Short-Polling_, backend Go menyediakan endpoint `GET /api/v1/events` (SSE). Saat Worker menyelesaikan tugas (contoh: _Compress to ZIP_ selesai), Go memancarkan _event trigger_ ke klien SvelteKit yang terhubung agar antarmuka ter-_update_ secara _real-time_. Event yang sama dapat dipakai untuk refresh widget kuota setelah write/purge berhasil.

---

## 7. Security & Protection Layers

CubyCloud dilengkapi sistem perlindungan bawaan tanpa modul eksternal:

### 1. In-Memory Token Bucket (Rate Limiter)

Mencegah serangan _Brute-force_ pada endpoint `/login` dan `/s/:token`.

- Diimplementasikan murni di Go RAM menggunakan `sync.Map`.
- **Trade-off Diterima:** Jika server _restart_, riwayat hitungan _rate limit_ akan di-reset. Ini ditoleransi demi menjaga I/O SQLite agar tidak terkuras oleh _spam login_.

### 2. Session (JWT in HTTP-Only Cookie)

- Setelah login sukses, JWT disimpan di **HTTP-Only Cookie** dengan `SameSite=Lax` dan atribut `Secure` saat HTTPS.
- Middleware Auth memvalidasi cookie pada request API terlindungi.

### 3. Session Invalidation (JWT Versioning)

Jika sistem diretas atau _Super Admin_ lupa kata sandi dan menggunakan _Emergency Recovery Key 64-char_:

- Setelah _recovery_ sukses, nilai `jwt_secret_version` di tabel SQLite `settings` bertambah 1 (+1).
- _Middleware Auth_ akan langsung menolak token JWT lama (karena versinya tidak cocok), memaksa **logout instan** di seluruh sesi/perangkat di dunia.

### 4. Password Hashing (bcrypt)

- Kata sandi pengguna (Super Admin & Regular Users) dan kata sandi proteksi share link wajib di-hash dengan **bcrypt** (_cost factor_ = 10) via `golang.org/x/crypto`.

### 5. Setup Guard Cache (Middleware)

Agar status `is_setup_completed` tidak terus-menerus menembak _query_ ke SQLite pada setiap _request_ API:

- Status ini dimuat 1 kali ke variabel _Atomic Boolean_ (`sync/atomic.Bool`) saat server menyala.
- _Middleware_ membaca langsung dari RAM (~0 ns _latency_) untuk memutuskan apakah pengguna harus di-_redirect_ ke rute Onboarding `/setup`.
- Setelah setup selesai, SPA mengarahkan ke **`/drive/home`** (bukan My Storage) — lihat SITEMAP.

### 6. Quota vs pool error codes

| Kondisi                                         | Kode / sinyal API                      | Pesan user-facing (PRD/SITEMAP)        |
| ----------------------------------------------- | -------------------------------------- | -------------------------------------- |
| My Storage user quota penuh                     | `USER_QUOTA_EXCEEDED`                  | Kuota My Storage penuh. Hubungi admin. |
| Semua node fisik penuh / di bawah safety margin | Pool capacity error (bukan user-quota) | Penyimpanan server penuh.              |

Super Admin selalu Unlimited pada user-quota; node capacity tetap berlaku untuk seluruh sistem.

---

Dokumen **`ARCHITECTURE.md`** ini mendefinisikan fondasi backend (performa, modul, storage, jobs, security) dan **menyilang** ke PRD / SITEMAP / DESIGN untuk aturan produk dan UI — tanpa menduplikasi spesifikasi fitur secara penuh.

**Langkah Selanjutnya:** Susun **`API_CONTRACT.md`** untuk endpoint RESTful (URL, metode, payload request/response) sebagai jembatan Backend Go ↔ SvelteKit Frontend, termasuk gate kuota dan event SSE.
`)
