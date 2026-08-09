# CubyCloud — Global AI Agent Guidance & Core Constraints

## 0. AI Agent Toolchain (wajib)

Tiga tool ini **selalu aktif** untuk agent yang mengerjakan repo ini:

| Tool | Fungsi | Setup proyek |
| ---- | ------ | ------------- |
| **Graphify** | Knowledge graph codebase + docs; query sebelum explore | `.cursor/rules/graphify.mdc`, `.cursor/mcp.json`, panduan `GRAPHIFY.md`, output `graphify-out/` (gitignored) |
| **RTK** (Rust Token Killer) | Kompres output perintah shell (~60–90% token) | Hook global Cursor: `rtk init -g --agent cursor --auto-patch`. Aturan: `.cursor/rules/rtk.mdc`, referensi: `RTK.md` |
| **Ponytail** | Diff kecil, hindari over-engineering; stack CubyCloud tetap wajib | `.cursor/rules/ponytail.mdc` — mode **full, relaxed** |

### Graphify (wajib untuk eksplorasi)

Sebelum `Read` / `Grep` / `Glob` / `Shell` untuk memahami arsitektur atau alur fitur:

```bash
graphify query "<pertanyaan>"
graphify path "<konsep A>" "<konsep B>"
graphify explain "<konsep>"
```

Setelah mengubah **file kode**, jalankan `graphify update .` (AST-only, tanpa biaya API). Setelah mengubah **dokumen produk**, jalankan full rebuild:

```bash
./scripts/graphify-extract.sh   # butuh .env.graphify.local (OpenAI-compatible gateway)
```

**MCP:** `.cursor/mcp.json` — server `graphify` (`graphify-mcp`). Wajib: `uv tool install "graphifyy[mcp,openai]"` (extra `[mcp]` untuk Cursor MCP).

**Prioritas:** Aturan produk CubyCloud (§1–§3) menang. Ponytail fokus **diff minimal & tanpa abstraksi extra** — bukan melawan stack shadcn/Tailwind/Go yang sudah ditetapkan PRD.

## 1. System Identity & Mission

CubyCloud adalah Self-Hosted Modular Cloud Storage berbasis **Single-Container** (Go backend + embedded SvelteKit SPA).

- **Target hardware:** Intel Celeron 2-Core, **2 GB RAM** (minimum)
- **Filosofi produk:** Zero-CLI onboarding, Flat Earthy Design, modular storage (local + S3/R2/S3-compatible)
- **Target image:** Single Docker image **< 150 MB**

## 2. Source of Truth (patuhi sebelum menulis kode)

| Dokumen | Path | Isi |
| ------- | ---- | --- |
| PRD | `docs/prd.md` | Scope v1.0, modul fitur, aturan bisnis (quota, share, org) |
| Architecture | `docs/architecture.md` | Blueprint backend: modul Go, SQLite WAL, VFS, queue, security |
| Design | `apps/frontend/DESIGN.md` | Token Flat Earthy, Tailwind v4, komponen UI |
| Sitemap | `apps/frontend/SITEMAP.md` | Route SvelteKit, sidebar, kontrak UI (quota, badge) |

Detail stack per area ada di `apps/frontend/AGENTS.md` dan `apps/backend/AGENTS.md`.

## 3. Strict Global Rules

### Resource & deployment

- **Idle RAM container:** **< 200 MB** (bukan target backend saja)
- **CGO-free:** Seluruh kode Go wajib tanpa CGO (`glebarez/go-sqlite`, dll.)
- **Single binary:** Frontend di-build dari `apps/frontend/` lalu di-embed ke biner Go

### Dependencies (v1.0)

- **DILARANG** dependensi runtime berat eksternal: Redis, MinIO, RabbitMQ, broker message queue terpisah
- **DILARANG** implementasi **AI Workspace / RAG / Ollama** di v1.0 (roadmap Fase 2/3 — lihat PRD)

### Security baseline

- Session: **JWT** disimpan di **HTTP-Only Cookie** (`SameSite=Lax`, `Secure` di HTTPS)
- Password (user & share link): **bcrypt** cost = 10

### User quota (My Storage)

- Scope: file dengan `owner_id = user` **di luar** Organization tree; Trash dihitung sampai purge
- Organization writes: **skip** user-quota check (tetap cek kapasitas node fisik)
- Error **wajib beda:** `USER_QUOTA_EXCEEDED` vs pool/node penuh — lihat PRD §4.10 & SITEMAP §5

### Frontend styling

- **Zero arbitrary values** di Tailwind: patuhi token di `apps/frontend/DESIGN.md` (`bg-app`, `bg-surface`, `text-main`, `rounded-xl`, dll.)
- Flat surfaces: **zero shadow** di seluruh UI (`shadow-none`); pemisahan visual via `border-subtle` dan surface layering — bukan elevasi bayangan
- Theme light/dark: `$lib/theme.svelte.ts` + `ThemeToggle` — detail di `apps/frontend/AGENTS.md`
