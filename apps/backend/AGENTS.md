# CubyCloud — Backend AI Agent Guidance (Golang & SQLite)

Patuhi kontrak global di `AGENTS.md` (root) dan blueprint teknis berikut **sebelum** menulis kode Go:

- `docs/prd.md` — aturan bisnis (quota, share, org, trash)
- `docs/architecture.md` — modul, schema, storage, jobs, security (wajib)

Detail UI/route: `apps/frontend/SITEMAP.md` — jangan duplikasi kontrak UX di backend.

---

## 1. Tech Stack & Standards

- **Language:** Go 1.22+
- **Lokasi kode:** `apps/backend/` (monorepo)
- **Web framework:** Echo v4.12+
- **ORM / DB:** Ent ORM + `glebarez/go-sqlite` (**CGO-free**)
- **Arsitektur:** Domain/Module-Driven Clean Architecture
- **Frontend embed:** Build artifact dari `apps/frontend/build/` → embed via `go:embed`

### Target struktur modul

```text
apps/backend/
├── cmd/server/main.go
├── ent/schema/
├── migrations/
├── internal/
│   ├── config/
│   ├── database/
│   ├── middleware/     # JWT cookie guard, rate limit, setup guard
│   ├── storage/        # StorageDriver interface
│   ├── sse/
│   └── modules/
│       ├── auth/
│       ├── vfs/        # write path + quota pre-check
│       ├── share/
│       ├── admin/      # nodes, settings, users & quota overrides
│       └── jobs/
└── web/embed.go        # static embed bridge
```

Product views (`/drive/home`, org, starred, stats) = **query layer VFS**, bukan modul Go terpisah.

---

## 2. SQLite & Concurrency

- WAL mode; `PRAGMA busy_timeout = 5000`
- **Write safety:** `db.SetMaxOpenConns(1)` — single writer
- **PK internal:** ULID (time-sorted)
- **Public share token:** NanoID (8–12 char)

---

## 3. Storage Engine

- Semua I/O disk lokal/cloud **wajib** melalui interface `StorageDriver`
- **Placement strategy** (configurable): `FILL_FIRST`, `MOST_FREE_SPACE`, `ROUND_ROBIN`
- Safety margin node: block write jika sisa kapasitas **< 5%**
- TUS chunks: `/app/data/temp/tus/`; cloud upload via presigned URL + `/api/v1/drive/sync-cloud`
- **Zero-byte copy:** jangan hapus `physical_blobs` jika `ref_count > 0`

### Write gates (urutan wajut)

1. **User quota pre-check** (My Storage only; Organization di-skip) → `USER_QUOTA_EXCEEDED`
2. **Node/pool capacity** → error pool penuh (**bukan** kode user-quota)

Policy default & override: PRD §4.9–4.10; enforcement di `vfs/` write path, policy di `admin/`.

---

## 4. Background Jobs & Real-Time

- Queue: SQLite atomic fetch (`UPDATE jobs ... RETURNING *`) — **bukan** Redis/RabbitMQ
- Default worker concurrency: **2** (dari `settings`)
- Cron jobs: `TUS_CLEANUP_JOB`, `AUDIT_CLEANUP_JOB`, `TRASH_PURGE_JOB` (> 30 hari trash)
- Real-time UI: **SSE** `GET /api/v1/events` (bukan WebSocket)

---

## 5. Security

- JWT session di **HTTP-Only Cookie** (`SameSite=Lax`, `Secure` on HTTPS)
- Password hashing: **bcrypt** cost = 10 (`golang.org/x/crypto`)
- Rate limit in-memory (token bucket) pada `/login` dan `/s/:token`
- JWT invalidation via `jwt_secret_version` bump setelah emergency recovery
- Setup guard: cache `is_setup_completed` in-memory (`sync/atomic.Bool`)

Super Admin: **unlimited** user-quota; node capacity tetap berlaku.

---

## 6. Larangan & Scope v1.0

- **CGO:** dilarang
- **Broker eksternal:** Redis, MinIO, RabbitMQ — dilarang
- **AI Workspace / RAG / Ollama:** dilarang di v1.0
- Jangan introduce dependency berat tanpa update PRD + ARCHITECTURE

Endpoint REST detail: susun di `docs/API_CONTRACT.md` (belum ada — referensi PRD + ARCHITECTURE sementara).
