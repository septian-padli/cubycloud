# CubyCloud

Self-hosted modular cloud storage — lightweight alternative to Nextcloud and Google Drive, built for homelab and small teams.

> **Vision:** Google Drive you own. Ultra-light on legacy hardware, zero-CLI setup, unified local + S3/R2 storage.

**Target launch:** 1 October 2026 (v1.0)

---

## Status

| Area | Status |
|------|--------|
| Product spec | [`docs/prd.md`](docs/prd.md) — v1.0 scope locked |
| Frontend | Coming soon landing page at [`apps/frontend`](apps/frontend) |
| Backend (Go) | Not started |

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
├── Makefile          # Monorepo build, dev, and test orchestration
├── Dockerfile        # Production multi-stage single container image
├── apps/
│   ├── backend/      # Go 1.22+ backend (Echo, SQLite WAL, Clean Architecture)
│   └── frontend/     # SvelteKit SPA (adapter-static build → go:embed)
├── docs/
│   ├── prd.md        # Product requirements & business rules
│   └── architecture.md # Architecture & system design
├── scripts/          # Helper scripts (dev runner, setup reset)
└── README.md
```

---

## Quick start (Development)

**Requirements:**
- [Go](https://go.dev) 1.22+
- [Bun](https://bun.sh) 1.x

### Run Full Stack (Backend + Frontend)

```bash
make dev
```
Or run directly via script: `./scripts/dev.sh`.
- Frontend: [http://localhost:5173](http://localhost:5173)
- Backend API: [http://localhost:8080](http://localhost:8080)

### Run Backend Only (Go)

```bash
# From repository root
make dev-backend

# Or from apps/backend directory
cd apps/backend
go run ./cmd/server/main.go
```
The backend automatically creates `./data` for SQLite database and storage nodes with sensible development defaults.

### Run Frontend Only (SvelteKit)

```bash
# From repository root
make dev-frontend

# Or from apps/frontend directory
cd apps/frontend
bun install
bun run dev
```

---

## Building Single Executable (Production)

```bash
make build
```
This builds the SvelteKit frontend, synchronizes the static build to `apps/backend/web/dist/`, and compiles a single self-contained Go binary with embedded frontend at `apps/backend/bin/cubycloud`.

```bash
./apps/backend/bin/cubycloud
```

---

## Testing

```bash
make test          # Run all backend + frontend tests
make test-backend  # Run Go backend tests
make test-frontend # Run SvelteKit type check
```

---

## Tech stack (planned / in use)

| Layer | Technology |
|-------|------------|
| Backend | Go 1.22+, Echo, ent, SQLite (WAL) |
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
