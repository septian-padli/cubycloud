# CubyCloud

Self-hosted modular cloud storage — lightweight alternative to Nextcloud and Google Drive, built for homelab and small teams.

> **Vision:** Google Drive you own. Ultra-light on legacy hardware, zero-CLI setup, unified local + S3/R2 storage.

**Target launch:** 1 October 2026 (v1.0)

---

## Status

| Area | Status |
|------|--------|
| Product spec | [`docs/prd.md`](docs/prd.md) — v1.0 scope locked |
| Frontend | SvelteKit SPA (UI mock) at [`apps/frontend`](apps/frontend) |
| Backend (Go) | Scaffolded — Echo + `GET /health` (no features yet) |

---

## What is CubyCloud?

CubyCloud is a **single-container** app: Go backend + embedded SvelteKit SPA. v1.0 focuses on:

- Zero-CLI web setup wizard
- Multi-node virtual storage (local disks + S3 / Cloudflare R2)
- Drive workspace (home, my-storage, organization, shared, starred, trash, stats)
- Per-user My Storage quotas (admin default + per-user override)
- Resumable uploads, sharing, trash lifecycle, bundled add-ons (ZIP, image convert, optional PDF preview)

AI Workspace & MCP RAG (Ollama) are **roadmap** items, not v1.0.

See the full PRD for architecture, NFRs, and stack decisions: [`docs/prd.md`](docs/prd.md).

---

## Repository structure

```text
cubycloud/
├── apps/
│   ├── frontend/     # SvelteKit SPA (static build → go:embed)
│   └── backend/      # Go module (Echo)
├── docs/
│   └── prd.md        # Product requirements
└── README.md
```

---

## Quick start (backend)

**Requirements:** [Go](https://go.dev/dl/) 1.25+

```bash
cp .env.example .env   # optional; defaults are PORT=8080, DATA_DIR_PATH=./data
make run               # http://localhost:8080/health
```

Live reload (install [Air](https://github.com/air-verse/air) once): `go install github.com/air-verse/air@latest` then `make dev`.

```bash
make build             # apps/backend/bin/server
```

---

## Quick start (frontend)

**Requirements:** [Bun](https://bun.sh) 1.x

```bash
cd apps/frontend
bun install
bun run dev
```

Open [http://localhost:5173](http://localhost:5173) for the coming soon page.

```bash
bun run build    # output: apps/frontend/build/
bun run check
bun run lint
```

Frontend details: [`apps/frontend/README.md`](apps/frontend/README.md)

---

## Tech stack (planned / in use)

| Layer | Technology |
|-------|------------|
| Backend | Go 1.22+, Echo, Bun ORM (`uptrace/bun`), SQLite (WAL) |
| Frontend | SvelteKit 2, Svelte 5, Tailwind CSS v4, shadcn-svelte |
| Deploy | Single Docker image (< 150 MB target) |
| Cloud I/O | AWS SDK v2 presigned URLs (S3 / R2) |

---

## Performance targets (v1.0)

| Metric | Target |
|--------|--------|
| Idle RAM | < 200 MB |
| Active RAM (transfer peak) | < 600 MB |
| Page navigation (LAN) | < 1.0 s |
| Minimum hardware | Intel Celeron 2-core, 2 GB RAM |

---

## Design

UI design system (Flat Earthy, Montserrat, Tailwind v4 tokens) is defined in [`apps/frontend/DESIGN.md`](apps/frontend/DESIGN.md) and referenced by the PRD. The coming soon page implements this spec.

---

## License

TBD.
